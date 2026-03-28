# 05 — Service Layer

> **Decision:** The service layer sits between the HTTP handler (or CLI) and the broker. It validates input, enforces business rules, and orchestrates operations.

**Reference:** `06-software-patterns/03-service-layer.md`

---

## Table of Contents

1. [Why a Service Layer?](#why-a-service-layer) `[CORE]`
2. [The Service Interface](#the-service-interface) `[CORE]`
3. [The Implementation](#the-implementation) `[CORE]`
4. [Validation Rules](#validation-rules) `[CORE]`
5. [Service Methods](#service-methods) `[CORE]`
6. [Subscribe with Auto-Generated ID](#subscribe-with-auto-generated-id) `[CORE]`
7. [Publish with ID Generation](#publish-with-id-generation) `[CORE]`
8. [Error Propagation Chain](#error-propagation-chain) `[PRODUCTION]`
9. [Testing the Service](#testing-the-service) `[PRODUCTION]`

---

## Why a Service Layer?

Without it, the HTTP handler calls the broker directly. That means:
- Validation logic is scattered across handlers
- Business rules (max topic name length, reserved names) are duplicated
- Testing requires starting an HTTP server

```
  WITHOUT SERVICE LAYER (BAD):

  HTTP Handler ──────────────────► Broker
  (validates + calls)              (pure routing)

  • Validation mixed with HTTP parsing
  • Can't test without HTTP server
  • Business rules in wrong layer


  WITH SERVICE LAYER (GOOD):

  HTTP Handler ──► Service ──► Broker
  (decode/encode)  (validate)  (pure routing)

  • Each layer has ONE job
  • Service testable without HTTP
  • Business rules in the right place
```

---

## The Service Interface

```go
// internal/service/mq.go
// TOPIC 7: Interface — handlers depend on this, not the broker
// TOPIC 8: Error wrapping — contextual errors

package service

import (
    "context"
    "fmt"
    "strings"

    "mini-mq/internal/broker"
    "mini-mq/internal/model"
)

type MessageQueue interface {
    // Topic operations
    CreateTopic(ctx context.Context, name string) error
    DeleteTopic(ctx context.Context, name string) error
    ListTopics() []model.TopicConfig

    // Subscription operations
    Subscribe(ctx context.Context, topic string, bufferSize int) (*broker.Subscriber, error)
    Unsubscribe(ctx context.Context, topic string, subID string) error

    // Message operations
    Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) (string, error)
}
```

**Why return `string` from Publish?** The message ID — so the caller can track it. The service generates the ID, not the publisher.

**Why does `Subscribe` return `*broker.Subscriber`?** This leaks the broker type into the service interface. Normally we'd define a `service.Subscriber` interface. But for this learning project, returning the concrete type avoids an extra abstraction layer. In production, you'd define:

```go
// Production alternative — don't leak broker types
type Subscriber interface {
    ID() string
    Start(ctx context.Context, handler func(model.Message) error)
    Close()
    Stats() SubscriberStats
}
```

---

## The Implementation

```go
// TOPIC 5: Struct, constructor with DI
// TOPIC 4: Validation with maps

type messageQueue struct {
    broker broker.Broker  // injected dependency
}

// NewMessageQueue creates the service with an injected broker
// TOPIC 4: Dependency injection (Pattern 04)
func NewMessageQueue(b broker.Broker) MessageQueue {
    return &messageQueue{broker: b}
}
```

---

## Validation Rules

**What:** Every service method validates input before calling the broker — topic name format, payload size, reserved names.

**Why validate in the service, not the broker?** The broker is a pure routing engine. It should be fast. Validation takes time. The service layer acts as a gatekeeper — bad data never reaches the broker. This separation keeps the broker simple and the service responsible for business rules.

**How:** Helper functions like `validateTopicName()` and `validatePayload()` are called at the start of each service method. Errors are wrapped with context using `fmt.Errorf("...: %w", err)`.

```go
// TOPIC 8: Errors — wrap with context
// TOPIC 2: Variables, strings package

var (
    ErrInvalidTopicName = fmt.Errorf("invalid topic name")
    ErrTopicNameTooLong = fmt.Errorf("topic name exceeds 64 characters")
    ErrReservedTopic    = fmt.Errorf("topic name is reserved")
    ErrPayloadTooLarge  = fmt.Errorf("payload exceeds 1MB limit")
    ErrEmptyPayload     = fmt.Errorf("payload cannot be empty")
)

// reserved topics that users can't create
// TOPIC 4: map as a SET — values are bool, we only care about keys
var reservedTopics = map[string]bool{
    "_dlq":    true,
    "_system": true,
    "_admin":  true,
}
```

**Why `map[string]bool` instead of `[]string`?** (Topic 4: Maps) Lookup is O(1) vs O(n). With a slice, we'd need to loop:

```go
// BAD: O(n) lookup
func isReserved(name string) bool {
    for _, r := range reservedTopics { // loop every time
        if r == name { return true }
    }
    return false
}

// GOOD: O(1) lookup
if reservedTopics[name] { // hash lookup
    return ErrReservedTopic
}
```

**Why not `map[string]struct{}`?** `map[string]bool` is more readable. `struct{}` saves 1 byte per entry — irrelevant for a 3-item set.

**Why validate in the service, not the broker?** The broker is a pure routing engine. It should be fast. Validation takes time. The service layer acts as a gatekeeper — bad data never reaches the broker.

---

## Service Methods

```go
// TOPIC 8: Error wrapping with %w
// TOPIC 5: UUID generation for message IDs

func (mq *messageQueue) CreateTopic(ctx context.Context, name string) error {
    if err := validateTopicName(name); err != nil {
        return err
    }

    config := model.DefaultTopicConfig(name)

    if err := mq.broker.CreateTopic(ctx, config); err != nil {
        return fmt.Errorf("create topic %q: %w", name, err)
    }

    return nil
}

func (mq *messageQueue) DeleteTopic(ctx context.Context, name string) error {
    if err := validateTopicName(name); err != nil {
        return err
    }

    if err := mq.broker.DeleteTopic(ctx, name); err != nil {
        return fmt.Errorf("delete topic %q: %w", name, err)
    }

    return nil
}

func (mq *messageQueue) ListTopics() []model.TopicConfig {
    return mq.broker.ListTopics()
}
```

### Why `fmt.Errorf("create topic %q: %w", name, err)`?

The `%w` wraps the original error. A caller can do:
```go
errors.Is(err, model.ErrTopicExists) // true
```

The message becomes: `create topic "orders": topic already exists` — useful for debugging.

---

## Subscribe with Auto-Generated ID

```go
// TOPIC 10: Goroutines — subscriber starts consuming immediately
// TOPIC 5: Factory function

import "github.com/google/uuid" // or use crypto/rand for no dependency

func (mq *messageQueue) Subscribe(ctx context.Context, topic string, bufferSize int) (*broker.Subscriber, error) {
    if err := validateTopicName(topic); err != nil {
        return nil, err
    }

    if !mq.broker.TopicExists(topic) {
        return nil, fmt.Errorf("subscribe: %w", model.ErrTopicNotFound)
    }

    subID := uuid.New().String() // unique subscriber ID
    sub := broker.NewSubscriber(subID, topic, bufferSize, broker.DropOldest)

    if err := mq.broker.Subscribe(ctx, topic, sub); err != nil {
        return nil, fmt.Errorf("subscribe to %q: %w", topic, err)
    }

    return sub, nil
}
```

**Why return the subscriber?** The caller needs the channel to consume messages. The service creates it, the broker registers it, and the caller reads from `sub.Channel()`.

---

## Publish with ID Generation

```go
func (mq *messageQueue) Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) (string, error) {
    if err := validateTopicName(topic); err != nil {
        return "", err
    }

    if err := validatePayload(payload); err != nil {
        return "", err
    }

    msgID := uuid.New().String()

    msg := model.Message{
        ID:        msgID,
        Topic:     topic,
        Payload:   payload,
        Headers:   headers,
        Timestamp: time.Now(),
    }

    if err := mq.broker.Publish(ctx, msg); err != nil {
        return "", fmt.Errorf("publish to %q: %w", topic, err)
    }

    return msgID, nil
}
```

---

## Error Propagation Chain

```
  Publisher calls svc.Publish(ctx, "orders", payload, headers)
       │
       ├──► validateTopicName("orders")     → nil (ok)
       ├──► validatePayload(payload)         → nil (ok)
       ├──► broker.Publish(ctx, msg)
       │         │
       │         ├──► topic exists?          → yes
       │         ├──► fanOut(msg)
       │         │       ├──► sub1.Publish() → nil (ok)
       │         │       └──► sub2.Publish() → ErrChannelFull
       │         └──► DLQ entry created for sub2 failure
       │
       └──► return msgID, nil (publish succeeded even if some subscribers failed)
```

---

## Testing the Service

```go
// With DI, we can test without a real broker

func TestCreateTopic(t *testing.T) {
    mockBroker := &MockBroker{} // implements broker.Broker
    svc := NewMessageQueue(mockBroker)

    err := svc.CreateTopic(context.Background(), "orders")
    if err != nil {
        t.Fatal(err)
    }

    if !mockBroker.CreateTopicCalled {
        t.Error("broker.CreateTopic was not called")
    }
}

func TestCreateTopic_InvalidName(t *testing.T) {
    mockBroker := &MockBroker{}
    svc := NewMessageQueue(mockBroker)

    err := svc.CreateTopic(context.Background(), "")
    if !errors.Is(err, ErrInvalidTopicName) {
        t.Fatalf("expected ErrInvalidTopicName, got: %v", err)
    }

    if mockBroker.CreateTopicCalled {
        t.Error("broker.CreateTopic should NOT be called for invalid input")
    }
}
```

**Reference:** `06-software-patterns/04-dependency-injection.md` — Testing with mocks

---

## Next

We have the model, broker, and service. Now we **wire it all together** in `main.go`. → `06-wiring.md`
