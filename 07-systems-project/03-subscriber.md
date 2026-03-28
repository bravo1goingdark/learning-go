# 03 — Subscriber with Backpressure

> **Decision:** Each subscriber gets its own buffered channel. When the buffer fills up, we need a backpressure strategy. This is the hardest design decision in the broker.

---

## Table of Contents

1. [The Problem](#the-problem) `[CORE]`
2. [Backpressure Strategies](#backpressure-strategies) `[CORE]`
3. [The Subscriber Struct](#the-subscriber-struct) `[CORE]`
4. [Creating a Subscriber](#creating-a-subscriber) `[CORE]`
5. [Publishing to a Subscriber](#publishing-to-a-subscriber) `[CORE]`
6. [Consuming Messages](#consuming-messages) `[CORE]`
7. [Closing a Subscriber](#closing-a-subscriber) `[CORE]`
8. [Putting It Together](#putting-it-together) `[CORE]`

---

## The Problem

A publisher sends 1000 messages/sec. A subscriber can only process 10/sec. Without backpressure, the subscriber's buffer grows forever → OOM crash.

```
  Publisher (1000/sec)              Subscriber (10/sec)
         │                                │
         │   ┌──────────────────────┐     │
         └──►│  Channel Buffer      │────►│
             │                      │     │  Can't keep up!
             │  Growing...          │     │
             │  Growing...          │     │
             │  💥 OOM!             │     │
             └──────────────────────┘
```

**Reference:** `06-software-patterns/08-backpressure-strategies.md`

---

## Backpressure Strategies

We support three strategies, configurable per subscriber.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                    BACKPRESSURE STRATEGIES                                │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   Strategy 1: DROP OLDEST                                                │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  Buffer: [m1][m2][m3][m4][m5]...[m100]  ← FULL                  │   │
  │   │  New message arrives: m101                                         │   │
  │   │  Action: Remove m1, shift left, add m101 at end                  │   │
  │   │  Result: [m2][m3][m4][m5]...[m100][m101]                         │   │
  │   │                                                                    │   │
  │   │  USE WHEN: Real-time data, latest value matters                  │   │
  │   │  EXAMPLE: Stock prices, sensor readings, game state              │   │
  │   └──────────────────────────────────────────────────────────────────┘   │
  │                                                                           │
  │   Strategy 2: DROP NEWEST                                                │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  Buffer: [m1][m2][m3][m4][m5]...[m100]  ← FULL                  │   │
  │   │  New message arrives: m101                                         │   │
  │   │  Action: Discard m101 silently                                    │   │
  │   │  Result: [m1][m2][m3][m4][m5]...[m100]  (unchanged)             │   │
  │   │                                                                    │   │
  │   │  USE WHEN: Analytics, metrics (losing a few is OK)               │   │
  │   │  EXAMPLE: Page view counters, log aggregation                    │   │
  │   └──────────────────────────────────────────────────────────────────┘   │
  │                                                                           │
  │   Strategy 3: BLOCK                                                      │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  Buffer: [m1][m2][m3][m4][m5]...[m100]  ← FULL                  │   │
  │   │  New message arrives: m101                                         │   │
  │   │  Action: Publisher blocks until subscriber drains a message      │   │
  │   │                                                                    │   │
  │   │  USE WHEN: Reliability matters, no data loss allowed             │   │
  │   │  EXAMPLE: Order processing, payment events                       │   │
  │   │                                                                    │   │
  │   │  ⚠ RISK: Slow subscriber blocks ALL publishers for this topic   │   │
  │   └──────────────────────────────────────────────────────────────────┘   │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## The Subscriber Struct

```go
// internal/broker/subscriber.go
// TOPIC 5: Structs, methods
// TOPIC 10: Goroutines
// TOPIC 11: Buffered channels
// TOPIC 8: Error handling

package broker

import (
    "context"
    "sync"
    "time"

    "mini-mq/internal/model"
)

type BackpressureStrategy int

const (
    DropOldest  BackpressureStrategy = iota // remove oldest, add new
    DropNewest                              // discard incoming message
    Block                                   // block publisher until space
)

type Subscriber struct {
    ID       string                        // unique subscriber ID (exported — read by service)
    Topic    string                        // topic this subscriber listens to (exported — read by service)
    ch       chan model.Message             // per-subscriber buffered channel (UNEXPORTED)
    strategy BackpressureStrategy           // what to do when buffer is full (UNEXPORTED)
    mu       sync.RWMutex                  // protects stats (UNEXPORTED)
    stats    SubscriberStats               // delivery metrics (UNEXPORTED)
    done     chan struct{}                  // signals subscriber is closed (UNEXPORTED)
}
```

### Why Are `ch`, `strategy`, `mu`, `stats`, `done` Unexported?

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │   FIELD        EXPORTED?    REASON                                      │
  ├──────────────────────────────────────────────────────────────────────────┤
  │   ID           YES          Service needs to identify the subscriber   │
  │   Topic        YES          Service needs to know what it subscribes to │
  │   ch           NO           Only broker writes to it, only consumer    │
  │                               reads from it. Nobody else should.       │
  │   strategy     NO           Backpressure is an internal concern.       │
  │   mu           NO           Concurrency is encapsulated.               │
  │   stats        NO           Use Stats() method with RLock.             │
  │   done         NO           Only broker's shutdown code uses it.       │
  └──────────────────────────────────────────────────────────────────────────┘
```

**Reference:** Topic 5 — Encapsulation. Unexported fields enforce the principle that callers interact through methods, not direct field access.

### Why `sync.RWMutex` for Stats Instead of `atomic`?

We could use `atomic.AddInt64` for counters. But `sync.RWMutex` is chosen because:
1. We have **multiple fields** (Received, Dropped, Errors, LastMsgAt) that must be read atomically together
2. `atomic` only works per-field — reading `Received` then `Dropped` could give inconsistent values
3. `RWMutex` allows concurrent reads (`Stats()` is called from monitoring goroutines)

```go
// BAD: atomic gives per-field consistency, not group consistency
atomic.LoadInt64(&s.stats.Received)  // might see old value
atomic.LoadInt64(&s.stats.Dropped)   // might see new value

// GOOD: RWMutex gives group consistency
s.mu.RLock()
stats := s.stats  // copy entire struct atomically
s.mu.RUnlock()
```

### Panic Recovery in Subscriber Handler

**What if the handler panics?** Without recovery, the panic kills the goroutine, the subscriber's `done` channel never closes, and the broker leaks a goroutine. Add panic recovery:

```go
// TOPIC 8: Error handling — recover from panics

func (s *Subscriber) Start(ctx context.Context, handler func(model.Message) error) {
    go func() {
        defer close(s.done) // signal broker we're done

        for {
            select {
            case <-ctx.Done():
                s.drainRemaining(handler)
                return

            case msg, ok := <-s.ch:
                if !ok {
                    return
                }

                // Panic recovery wrapper
                err := s.safeHandle(handler, msg)

                if err != nil {
                    s.mu.Lock()
                    s.stats.Errors++
                    s.mu.Unlock()
                } else {
                    s.mu.Lock()
                    s.stats.Received++
                    s.stats.LastMsgAt = time.Now()
                    s.mu.Unlock()
                }
            }
        }
    }()
}

func (s *Subscriber) safeHandle(handler func(model.Message) error, msg model.Message) (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("handler panic: %v", r)
            log.Printf("subscriber %s: handler panic on msg %s: %v", s.ID[:8], msg.ID[:8], r)
        }
    }()
    return handler(msg)
}
```

**Why `safeHandle` as a separate function?** The `defer` + `recover()` must be in the same goroutine as the panic. A separate function keeps the recovery logic clean and reusable. If we put `defer recover()` inside the `for` loop, it would need to be in an anonymous function anyway.

```go
type SubscriberStats struct {
    Received  int64     // total messages received
    Dropped   int64     // messages dropped (backpressure)
    Errors    int64     // processing errors
    LastMsgAt time.Time // when the last message was processed
}
```

### Why a Separate `done` Channel?

When the broker shuts down, it needs to know when each subscriber has finished processing. The `done` channel is closed when the subscriber's goroutine exits. The broker can `range` over all subscribers and wait for their `done` channels.

```
  Broker shutdown:
    │
    ├──► close subscriber.ch (no more messages)
    │
    ├──► subscriber goroutine drains remaining messages
    │
    └──► close(subscriber.done)  ◄── broker waits on this
```

---

## Creating a Subscriber

```go
// TOPIC 5: Factory function pattern
// TOPIC 11: make(chan, bufferSize)

func NewSubscriber(id, topic string, bufferSize int, strategy BackpressureStrategy) *Subscriber {
    if bufferSize <= 0 {
        bufferSize = 100 // sensible default
    }

    return &Subscriber{
        ID:       id,
        Topic:    topic,
        ch:       make(chan model.Message, bufferSize), // buffered!
        strategy: strategy,
        done:     make(chan struct{}),
    }
}
```

**Why `make(chan, bufferSize)`?** (Topic 11: Buffered channels) An unbuffered channel would block the publisher on every send. The buffer decouples publisher speed from subscriber speed. `bufferSize` is the tunable knob.

### Why No `Channel()` Accessor?

Some designs expose a `Channel() <-chan model.Message` method. We intentionally don't. The subscriber manages its own consumption loop via `Start()`. Exposing the channel would let callers read from it directly, bypassing stats tracking, panic recovery, and the drain-on-shutdown logic. The `Start()` method is the single entry point — it controls the entire lifecycle.

---

## Publishing to a Subscriber

This is where backpressure logic lives.

```go
// TOPIC 11: Channel send
// TOPIC 12: Select with default

func (s *Subscriber) Publish(ctx context.Context, msg model.Message) error {
    switch s.strategy {

    case DropNewest:
        // Try to send. If full, drop the message.
        select {
        case s.ch <- msg:
            s.mu.Lock()
            s.stats.Received++
            s.mu.Unlock()
            return nil
        default:
            s.mu.Lock()
            s.stats.Dropped++
            s.mu.Unlock()
            return model.ErrChannelFull
        }

    case DropOldest:
        // Try to send. If full, remove oldest first.
        select {
        case s.ch <- msg:
            s.mu.Lock()
            s.stats.Received++
            s.mu.Unlock()
            return nil
        default:
            // Drain one message, then send new one
            select {
            case <-s.ch: // remove oldest
                s.mu.Lock()
                s.stats.Dropped++
                s.mu.Unlock()
            default:
            }
            // Now try again (non-blocking)
            select {
            case s.ch <- msg:
                s.mu.Lock()
                s.stats.Received++
                s.mu.Unlock()
                return nil
            default:
                s.mu.Lock()
                s.stats.Dropped++
                s.mu.Unlock()
                return model.ErrChannelFull
            }
        }

    case Block:
        // Block until subscriber consumes or context is cancelled
        select {
        case s.ch <- msg:
            s.mu.Lock()
            s.stats.Received++
            s.mu.Unlock()
            return nil
        case <-ctx.Done():
            s.mu.Lock()
            s.stats.Dropped++
            s.mu.Unlock()
            return ctx.Err()
        }

    default:
        return model.ErrChannelFull
    }
}
```

### Why Three `select` Statements in DropOldest?

```
  select {                    select {               select {
  case s.ch <- msg:           case <-s.ch:           case s.ch <- msg:
      // success                  // drained one         // retry success
      return nil                 s.stats.Dropped++      return nil
  default:                    default:               default:
      // full — drain            // race: someone       // still full
      // then retry              // else drained it     return ErrFull
  }                           }                      }
```

The inner select handles a race: between our outer `default` and the drain, another goroutine might have already drained a message. The `default` in the drain prevents blocking.

---

## Consuming Messages

```go
// TOPIC 10: Goroutine loop
// TOPIC 13: Context cancellation
// TOPIC 11: Channel range

func (s *Subscriber) Start(ctx context.Context, handler func(model.Message) error) {
    go func() {
        defer close(s.done) // signal broker we're done

        for {
            select {
            case <-ctx.Done():
                // Graceful shutdown — drain remaining messages
                s.drainRemaining(handler)
                return

            case msg, ok := <-s.ch:
                if !ok {
                    // Channel closed by broker
                    return
                }

                if err := handler(msg); err != nil {
                    s.mu.Lock()
                    s.stats.Errors++
                    s.mu.Unlock()
                    // In production: send to DLQ (file 08)
                } else {
                    s.mu.Lock()
                    s.stats.Received++
                    s.stats.LastMsgAt = time.Now()
                    s.mu.Unlock()
                }
            }
        }
    }()
}

func (s *Subscriber) drainRemaining(handler func(model.Message) error) {
    for {
        select {
        case msg, ok := <-s.ch:
            if !ok {
                return
            }
            handler(msg) // best effort during shutdown
        default:
            return // nothing left
        }
    }
}
```

### Why `defer close(s.done)`?

The broker holds a `sync.WaitGroup`. Each subscriber's goroutine calls `wg.Add(1)` when started and `wg.Done()` when it returns. But we also need a way for the broker to wait on *individual* subscribers (not just the group). The `done` channel lets the broker do:

```go
// In broker: wait for specific subscriber
<-sub.done // blocks until subscriber exits
```

---

## Closing a Subscriber

```go
// TOPIC 9: Defer, resource cleanup

func (s *Subscriber) Close() {
    close(s.ch) // no more messages
    <-s.done    // wait for goroutine to finish
}

func (s *Subscriber) Stats() SubscriberStats {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.stats
}
```

**Why `close(s.ch)` then `<-s.done`?** (Topic 9: Defer order matters)
1. `close(s.ch)` — the subscriber goroutine sees `ok == false` and exits
2. `<-s.done` — blocks until the goroutine has fully exited (drained remaining)
3. This ensures no goroutine leak

---

## Putting It Together

```
  Publisher goroutine                Subscriber goroutine
         │                                   │
         │  Publish(ctx, msg)                │
         │                                   │
         │  ┌─ DropNewest? ──► select{} ─┐  │
         │  │  channel full? → drop      │  │
         │  │  channel ok?   → send      │  │
         │  ├─ DropOldest? ─► drain+send─┤  │
         │  └─ Block? ──────► wait ──────┘  │
         │                                   │
         │                                   │  for { select {
         │                                   │    case <-ctx.Done():
         │                                   │      drainRemaining()
         │                                   │      return
         │                                   │    case msg := <-ch:
         │                                   │      handler(msg)
         │                                   │  }}
         │                                   │
         │                                   │  defer close(done)
```

---

## Next

With subscribers defined, we need the **broker** that manages topics and routes messages to the right subscribers. → `04-broker-core.md`
