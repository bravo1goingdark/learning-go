# 02 — Message & Topic Model

> **Decision:** Define our core domain types. Everything in the broker revolves around messages and topics.

---

## Why Start Here?

The model layer is the innermost dependency. It imports nothing from outer layers. If we get this wrong, everything built on top is wrong.

```
  ┌────────────────────┐
  │  MODEL (innermost) │  ◄── THIS FILE
  │  No imports        │
  └─────────┬──────────┘
            │
            ▼
  ┌────────────────────┐
  │  BROKER            │  ◄── imports model
  └─────────┬──────────┘
            │
            ▼
  ┌────────────────────┐
  │  SERVICE           │  ◄── imports broker + model
  └─────────┬──────────┘
            │
            ▼
  ┌────────────────────┐
  │  CMD (main.go)     │  ◄── imports everything
  └────────────────────┘
```

---

## The Message

A message is the unit of data flowing through the broker.

### Design Decisions

| Field | Type | Why |
|-------|------|-----|
| `ID` | `string` | String (not `int64`) because IDs come from external systems (UUID, snowflake, ULID). Using `string` avoids coupling to one ID scheme. |
| `Topic` | `string` | Which topic this message belongs to. Used as the routing key. |
| `Payload` | `[]byte` | Raw data — the broker doesn't care about format (JSON, protobuf, text) |
| `Headers` | `map[string]string` | Metadata: trace IDs, correlation IDs, content type |
| `Timestamp` | `time.Time` | When the message was published. Used for ordering, TTL, debugging. |
| `RetryCount` | `int` | How many times delivery was attempted. Starts at 0. Incremented by the DLQ on retry. |

**Why `RetryCount` if the basic broker doesn't retry?** It's a forward-looking field. The DLQ (file 08) uses it when retrying messages. Without it, we'd need a schema migration to add retry tracking later. Struct fields are cheap — schema changes are expensive.

### Why `[]byte` for Payload?

The broker is a transport layer. It doesn't parse messages. If we used `any` or `interface{}`, we'd need serialization. `[]byte` keeps it simple — the publisher serializes, the subscriber deserializes.

```go
// internal/model/message.go
// TOPIC 5: Structs, named fields, zero values
// TOPIC 2: Variables, types

package model

import "time"

type Message struct {
    ID        string            // UUID or snowflake ID
    Topic     string            // destination topic name
    Payload   []byte            // raw bytes — broker is format-agnostic
    Headers   map[string]string // metadata: trace-id, correlation-id
    Timestamp time.Time         // when published
    RetryCount int              // delivery attempts (0 = first try)
}
```

### Why Headers as `map[string]string`?

`map[string]string` (Topic 4: Maps) is simple and covers 90% of use cases:
- Trace propagation: `trace-id: abc-123`
- Content type: `content-type: application/json`
- Source: `source: order-service`

No need for a custom struct — the map is flexible enough.

### Message Status Constants

```go
// TOPIC 2: Constants, iota

type DeliveryStatus int

const (
    StatusPending   DeliveryStatus = iota // waiting in subscriber channel
    StatusDelivered                        // subscriber received it
    StatusFailed                           // delivery failed
    StatusDead                             // sent to dead letter queue
)
```

**Why iota?** (Topic 2: Constants) Each status is a sequential integer. The compiler ensures uniqueness. Readable in logs.

**Where is `DeliveryStatus` used?** Not in the basic broker. It's used when you add persistence or an HTTP API that queries message delivery state. The basic broker tracks stats via `SubscriberStats` instead. Having the type ready avoids a refactor later.

---

## The Topic

A topic is a named channel that groups messages.

### Design Decisions

| Field | Type | Why |
|-------|------|-----|
| `Name` | `string` | Topic identifier (e.g., "orders", "events"). Alphanumeric + hyphens + underscores only. |
| `MaxSubscribers` | `int` | Cap on concurrent subscribers. Prevents resource exhaustion from runaway subscriptions. |
| `MaxMessages` | `int` | Max messages in any subscriber's buffer. Passed to `make(chan, N)`. |
| `CreatedAt` | `time.Time` | For monitoring, TTL, admin dashboards. |

**Why is TopicConfig a struct and not just a string?** Because a topic has configuration limits. If it were just a string name, we'd have magic numbers scattered everywhere (`make(chan, 100)` — what does 100 mean?). The config struct makes limits explicit and configurable.

```go
// internal/model/topic.go
// TOPIC 5: Structs, embedding
// TOPIC 4: Maps

package model

import "time"

type TopicConfig struct {
    Name           string        // topic identifier
    MaxSubscribers int           // cap on concurrent subscribers
    MaxMessages    int           // max messages in any subscriber's buffer
    CreatedAt      time.Time     // when the topic was created
}

// DefaultConfig returns sensible defaults
// TOPIC 5: Factory function pattern
func DefaultTopicConfig(name string) TopicConfig {
    return TopicConfig{
        Name:           name,
        MaxSubscribers: 100,    // 100 subscribers per topic
        MaxMessages:    1000,   // 1000 messages buffer per subscriber
        CreatedAt:      time.Now(),
    }
}
```

**Why separate TopicConfig from the broker's internal topic state?** (Pattern 05: Clean Architecture) Config is a pure value — no behavior. The broker's internal `topic` struct (file 04) holds runtime state like the subscriber map and mutex. Separating them keeps the model layer free of concurrency primitives.

---

## Error Types

```go
// TOPIC 8: Error handling, sentinel errors

package model

import "errors"

var (
    ErrTopicNotFound    = errors.New("topic not found")
    ErrTopicExists      = errors.New("topic already exists")
    ErrSubscriberNotFound = errors.New("subscriber not found")
    ErrBrokerClosed     = errors.New("broker is closed")
    ErrTopicFull        = errors.New("topic has reached max subscribers")
    ErrChannelFull      = errors.New("subscriber channel is full")
    ErrPublishTimeout   = errors.New("publish timed out")
)
```

**Why sentinel errors?** (Topic 8: Error handling) Callers can check with `errors.Is(err, model.ErrTopicNotFound)` without type assertions. Simpler than custom error types for this use case.

---

## What This Enables

With `Message` and `TopicConfig` defined, the broker layer can:
- Route messages to the right topic (using `Message.Topic`)
- Apply per-topic limits (using `TopicConfig.MaxSubscribers`)
- Track delivery (using `Message.RetryCount` and `DeliveryStatus`)

The model layer knows nothing about channels, goroutines, or mutexes. That's the broker's job.
