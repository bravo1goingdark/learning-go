# 04 — Broker Core Engine

> **Decision:** The broker is the central coordinator. It manages topics, routes messages to subscribers via fan-out, and handles concurrency with mutexes.

---

## The Broker Interface

Define the interface first — the service layer depends on this, not on the concrete implementation.

```go
// internal/broker/broker.go
// TOPIC 7: Interfaces — accept interfaces, return structs
// TOPIC 8: Error handling

package broker

import (
    "context"
    "mini-mq/internal/model"
)

// Broker is the core message routing engine
type Broker interface {
    // Topic management
    CreateTopic(ctx context.Context, config model.TopicConfig) error
    DeleteTopic(ctx context.Context, name string) error
    ListTopics() []model.TopicConfig
    TopicExists(name string) bool

    // Subscription
    Subscribe(ctx context.Context, topic string, sub *Subscriber) error
    Unsubscribe(ctx context.Context, topic string, subID string) error

    // Publishing
    Publish(ctx context.Context, msg model.Message) error

    // Lifecycle
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
    Stats() BrokerStats
}
```

**Why an interface?** (Topic 7: Interfaces) The service layer calls `broker.Publish()` without knowing if it's in-memory, Redis-backed, or a mock. We can swap implementations for testing.

---

## Internal Topic State

```go
// internal/broker/topic.go
// TOPIC 4: Maps
// TOPIC 15: Mutex (sync.RWMutex)

package broker

import (
    "sync"
    "mini-mq/internal/model"
)

type topic struct {
    config      model.TopicConfig
    subscribers map[string]*Subscriber // subID → subscriber
    mu          sync.RWMutex           // protects subscribers map
    closed      bool
}

func newTopic(config model.TopicConfig) *topic {
    return &topic{
        config:      config,
        subscribers: make(map[string]*Subscriber),
    }
}

// addSubscriber adds a subscriber thread-safely
func (t *topic) addSubscriber(sub *Subscriber) error {
    t.mu.Lock()
    defer t.mu.Unlock()

    if t.closed {
        return model.ErrBrokerClosed
    }

    if _, exists := t.subscribers[sub.ID]; exists {
        return nil // already subscribed, idempotent
    }

    if len(t.subscribers) >= t.config.MaxSubscribers {
        return model.ErrTopicFull
    }

    t.subscribers[sub.ID] = sub
    return nil
}

// removeSubscriber removes a subscriber thread-safely
func (t *topic) removeSubscriber(subID string) error {
    t.mu.Lock()
    defer t.mu.Unlock()

    sub, exists := t.subscribers[subID]
    if !exists {
        return model.ErrSubscriberNotFound
    }

    sub.Close() // drain + wait
    delete(t.subscribers, subID)
    return nil
}

// fanOut sends a message to ALL subscribers
// TOPIC 18: Fan-out pattern
func (t *topic) fanOut(ctx context.Context, msg model.Message) []FanOutError {
    t.mu.RLock()
    defer t.mu.RUnlock()

    var errors []FanOutError

    for _, sub := range t.subscribers {
        if err := sub.Publish(ctx, msg); err != nil {
            errors = append(errors, FanOutError{
                SubscriberID: sub.ID,
                MessageID:    msg.ID,
                Err:          err,
            })
        }
    }

    return errors
}

// close drains all subscribers and closes the topic
func (t *topic) close() {
    t.mu.Lock()
    defer t.mu.Unlock()

    t.closed = true
    for _, sub := range t.subscribers {
        sub.Close()
    }
}

type FanOutError struct {
    SubscriberID string
    MessageID    string
    Err          error
}
```

### Why `sync.RWMutex` Instead of `sync.Mutex`?

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │   OPERATION                    FREQUENCY              LOCK TYPE          │
  ├──────────────────────────────────────────────────────────────────────────┤
  │   Publish (fanOut)             Very high (1000s/sec)  RLock (read)      │
  │   Subscribe/Unsubscribe        Low (occasional)       Lock (write)      │
  │   Delete topic                 Rare                   Lock (write)      │
  └──────────────────────────────────────────────────────────────────────────┘
```

**Reference:** `06-concurrency/16-mutex-vs-channels.md`

Publishing happens constantly. Subscribing happens occasionally. `RWMutex` allows concurrent publishes (multiple readers) while blocking only for subscriber changes (writer). If we used `Mutex`, every publish would serialize.

---

## The Broker Implementation

```go
// internal/broker/broker.go (continued)

package broker

import (
    "context"
    "sync"

    "mini-mq/internal/model"
)

type Config struct {
    DefaultBufferSize  int  // default subscriber channel size
    MaxTopics          int  // max topics the broker can hold
    EnableDeadLetter   bool // send failed messages to DLQ
}

func DefaultConfig() Config {
    return Config{
        DefaultBufferSize: 100,
        MaxTopics:         1000,
        EnableDeadLetter:  true,
    }
}

type inMemoryBroker struct {
    config   Config
    topics   map[string]*topic // topicName → topic
    mu       sync.RWMutex      // protects topics map
    dlq      *DeadLetterQueue  // dead letter queue (file 08)
    closed   bool
    wg       sync.WaitGroup    // tracks active goroutines
}

func New(cfg Config) Broker {
    return &inMemoryBroker{
        config: cfg,
        topics: make(map[string]*topic),
        dlq:    NewDeadLetterQueue(1000), // buffer 1000 dead messages
    }
}
```

### Why `inMemoryBroker` Is Unexported (lowercase)?

The struct is unexported, but the `New()` function returns the exported `Broker` interface. This means:
- Callers can only use the interface — they can't access struct fields directly
- We can swap `inMemoryBroker` for `redisBroker` without changing any caller code
- The concrete type is an implementation detail

```go
// This works:
var b broker.Broker = broker.New(cfg)  // using interface

// This does NOT compile:
var b *broker.inMemoryBroker = broker.New(cfg)  // unexported type
```

**Reference:** Topic 7 — "Accept interfaces, return structs." We return the interface because the caller shouldn't depend on the implementation.

---

## Topic Management

```go
// TOPIC 4: Maps — CRUD operations
// TOPIC 8: Error handling — sentinel errors

func (b *inMemoryBroker) CreateTopic(ctx context.Context, config model.TopicConfig) error {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.closed {
        return model.ErrBrokerClosed
    }

    if _, exists := b.topics[config.Name]; exists {
        return model.ErrTopicExists
    }

    if len(b.topics) >= b.config.MaxTopics {
        return model.ErrTopicFull
    }

    b.topics[config.Name] = newTopic(config)
    return nil
}

func (b *inMemoryBroker) DeleteTopic(ctx context.Context, name string) error {
    b.mu.Lock()
    defer b.mu.Unlock()

    t, exists := b.topics[name]
    if !exists {
        return model.ErrTopicNotFound
    }

    t.close() // drain all subscribers
    delete(b.topics, name)
    return nil
}

func (b *inMemoryBroker) ListTopics() []model.TopicConfig {
    b.mu.RLock()
    defer b.mu.RUnlock()

    // Why copy into a new slice?
    // Returning the internal map's values directly would let callers
    // hold a reference to broker internals. If the broker mutates the
    // map while the caller iterates, we get a data race.
    configs := make([]model.TopicConfig, 0, len(b.topics))
    for _, t := range b.topics {
        configs = append(configs, t.config)
    }
    return configs
}

func (b *inMemoryBroker) TopicExists(name string) bool {
    b.mu.RLock()
    defer b.mu.RUnlock()
    _, exists := b.topics[name]
    return exists
}
```

---

## Subscription

```go
// TOPIC 15: Mutex — write lock for map mutation

func (b *inMemoryBroker) Subscribe(ctx context.Context, topicName string, sub *Subscriber) error {
    // Phase 1: RLock to look up topic (doesn't block other lookups)
    b.mu.RLock()
    t, exists := b.topics[topicName]
    b.mu.RUnlock()

    if !exists {
        return model.ErrTopicNotFound
    }

    // Phase 2: Lock inside topic.addSubscriber (write lock only on this topic)
    return t.addSubscriber(sub)
}

func (b *inMemoryBroker) Unsubscribe(ctx context.Context, topicName string, subID string) error {
    b.mu.RLock()
    t, exists := b.topics[topicName]
    b.mu.RUnlock()

    if !exists {
        return model.ErrTopicNotFound
    }

    return t.removeSubscriber(subID)
}
```

### Why Two-Phase Locking?

```
  Subscribe("orders", sub)
       │
       ├──► broker.mu.RLock()    ◄── lock broker map (read)
       ├──► t = broker.topics["orders"]
       ├──► broker.mu.RUnlock()  ◄── release immediately
       │
       └──► t.addSubscriber(sub) ◄── lock topic (write, per-topic)
```

If we held the broker's `mu.Lock()` for the entire subscribe operation, subscribing to topic A would block publishes to topic B (because the broker lock is shared). By doing a quick RLock for lookup, then a per-topic lock for mutation, publishes to other topics remain unblocked.

### What If Subscribe Is Called Before CreateTopic?

The `RLock` lookup returns `exists == false`, so we return `ErrTopicNotFound`. The service layer (file 05) checks `TopicExists()` before subscribing, so this error path is only hit if someone bypasses the service layer.

---

## Publishing — The Core Path

This is the hot path. It must be fast.

```go
// TOPIC 18: Fan-out — one message, many subscribers
// TOPIC 13: Context — propagate cancellation
// TOPIC 15: RLock — concurrent reads allowed

func (b *inMemoryBroker) Publish(ctx context.Context, msg model.Message) error {
    // Check if broker is shutting down
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Look up topic (read lock — concurrent safe)
    b.mu.RLock()
    t, exists := b.topics[msg.Topic]
    b.mu.RUnlock()

    if !exists {
        return model.ErrTopicNotFound
    }

    // Fan-out to all subscribers
    fanOutErrors := t.fanOut(ctx, msg)

    // Handle delivery failures → DLQ
    if b.config.EnableDeadLetter && len(fanOutErrors) > 0 {
        for _, fe := range fanOutErrors {
            b.dlq.Push(DLQEntry{
                Message:       msg,
                SubscriberID:  fe.SubscriberID,
                Error:         fe.Err.Error(),
                Timestamp:     time.Now(),
            })
        }
    }

    return nil
}
```

### Why Check `ctx.Done()` First?

```
  Publish(ctx, msg)
       │
       ├──► select { case <-ctx.Done(): return }  ◄── FAST EXIT
       │
       ├──► mu.RLock → lookup topic
       │
       └──► fanOut → deliver to subscribers
```

If the broker is shutting down, we don't want to look up topics or attempt delivery. The `select` with `default` is non-blocking — it checks cancellation in nanoseconds.

---

## Lifecycle

```go
// TOPIC 13: Context propagation
// TOPIC 14: WaitGroup for goroutine tracking

func (b *inMemoryBroker) Start(ctx context.Context) error {
    // Start the DLQ processor
    b.wg.Add(1)
    go func() {
        defer b.wg.Done()
        b.dlq.Process(ctx) // processes dead messages, logs them
    }()

    return nil
}

func (b *inMemoryBroker) Shutdown(ctx context.Context) error {
    b.mu.Lock()
    b.closed = true
    b.mu.Unlock()

    // Close all topics (drains subscribers)
    for _, t := range b.topics {
        t.close()
    }

    // Wait for all goroutines (DLQ processor, etc.)
    done := make(chan struct{})
    go func() {
        b.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil // clean shutdown
    case <-ctx.Done():
        return ctx.Err() // shutdown timeout exceeded
    }
}
```

### Why the Extra Goroutine for `wg.Wait()`?

```
  Shutdown(ctx)
       │
       ├──► close all topics
       │
       ├──► go func() {
       │        b.wg.Wait()     ◄── could block forever if goroutine leaks
       │        close(done)
       │    }()
       │
       └──► select {
              case <-done:    // clean
              case <-ctx.Done(): // timeout
            }
```

If a goroutine leaks (never returns), `wg.Wait()` blocks forever. The `ctx.Done()` provides a timeout escape hatch. This is production-grade defensive coding.

---

## Stats

```go
type BrokerStats struct {
    TotalTopics      int
    TotalSubscribers int
    DLQSize          int
    Closed           bool
}

func (b *inMemoryBroker) Stats() BrokerStats {
    b.mu.RLock()
    defer b.mu.RUnlock()

    totalSubs := 0
    for _, t := range b.topics {
        t.mu.RLock()
        totalSubs += len(t.subscribers)
        t.mu.RUnlock()
    }

    return BrokerStats{
        TotalTopics:      len(b.topics),
        TotalSubscribers: totalSubs,
        DLQSize:          b.dlq.Size(),
        Closed:           b.closed,
    }
}
```

---

## Full Request Flow

```
  Publisher                  Broker                         Subscriber
     │                        │                                │
     │  Publish(ctx, msg)     │                                │
     │ ──────────────────────►│                                │
     │                        │                                │
     │                        │  1. Check ctx.Done()          │
     │                        │  2. RLock → lookup topic      │
     │                        │  3. fanOut:                   │
     │                        │     for each subscriber:      │
     │                        │       sub.Publish(ctx, msg)   │
     │                        │         │                     │
     │                        │         ├─ DropNewest?        │
     │                        │         ├─ DropOldest?        │
     │                        │         └─ Block?             │
     │                        │              │                │
     │                        │              └────────────────│──► msg on channel
     │                        │                                │
     │                        │  4. Collect errors             │
     │                        │  5. Failed? → DLQ             │
     │                        │                                │
     │  ◄── error / nil ──────│                                │
```

---

## Next

The broker handles the mechanics. The **service layer** adds validation, logging, and business rules. → `05-service-layer.md`
