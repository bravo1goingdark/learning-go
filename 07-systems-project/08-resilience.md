# 08 — Resilience: Dead Letter Queue & Backpressure

> **Decision:** When a subscriber can't process a message, we don't drop it silently. We send it to a Dead Letter Queue (DLQ) for later inspection or retry.

### What Is a Dead Letter Queue?

A Dead Letter Queue (DLQ) is a safety net. In a pub-sub system, messages flow from publisher → broker → subscriber. If a subscriber's handler fails (crash, timeout, bad data), the message would be lost without a DLQ. Instead, failed messages go to a separate queue where you can:
- **Inspect** them to understand what went wrong
- **Retry** them after fixing the underlying issue
- **Alert** on them to detect systemic problems

DLQs are standard in production message systems (RabbitMQ, AWS SQS, Kafka all have them). They separate "message delivery" from "message processing" — the broker guarantees delivery to the subscriber, and the DLQ captures processing failures.

**Reference:** `06-software-patterns/07-retry-circuit-breaker.md` + `06-software-patterns/08-backpressure-strategies.md`

---

## The Problem

A subscriber's handler fails (database down, panic, timeout). Without a DLQ, the message is lost forever.

```
  Publisher ──► Broker ──► Subscriber ──► Handler fails
                                             │
                                             ▼
                                      💥 Message LOST
```

With a DLQ:

```
  Publisher ──► Broker ──► Subscriber ──► Handler fails
                                             │
                                             ▼
                                      DLQ ◄──┘
                                      (stored for retry/inspection)
```

---

## Dead Letter Queue Design

```go
// internal/broker/dlq.go
// TOPIC 5: Struct
// TOPIC 11: Buffered channel
// TOPIC 10: Goroutine for background processing
// TOPIC 15: Mutex for stats

package broker

import (
    "context"
    "log"
    "sync"
    "time"

    "mini-mq/internal/model"
)

type DLQEntry struct {
    Message      model.Message // the failed message
    SubscriberID string        // which subscriber failed
    Error        string        // error message
    Timestamp    time.Time     // when it failed
    Attempt      int           // retry attempt number
}

type DeadLetterQueue struct {
    ch      chan DLQEntry      // bounded channel for DLQ entries
    mu      sync.RWMutex      // protects stats
    stats   DLQStats
    maxSize int
}

type DLQStats struct {
    TotalReceived   int64
    TotalRetried    int64
    TotalDropped    int64 // DLQ itself overflowed
    CurrentSize     int
}

func NewDeadLetterQueue(bufferSize int) *DeadLetterQueue {
    return &DeadLetterQueue{
        ch:      make(chan DLQEntry, bufferSize),
        maxSize: bufferSize,
    }
}
```

### Why Bounded DLQ? Why Not Unbounded?

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                           │
  │   UNBOUNDED DLQ (make(chan DLQEntry))           DANGEROUS                │
  │   • If subscribers keep failing, DLQ grows forever                      │
  │   • Eventually OOM — same problem we're trying to prevent               │
  │   • No backpressure signal to the system                                │
  │                                                                           │
  │   BOUNDED DLQ (make(chan DLQEntry, 1000))       SAFE                    │
  │   • Fixed memory budget: 1000 entries max                               │
  │   • When full, we drop and log — system stays alive                     │
  │   • Monitoring catches sustained failures (TotalDropped counter)        │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

**Principle:** Every queue in the system must be bounded. Unbounded queues are unbounded memory leaks.

---

## Pushing to the DLQ

```go
// TOPIC 12: Select with default (non-blocking)

func (dlq *DeadLetterQueue) Push(entry DLQEntry) {
    dlq.mu.Lock()
    dlq.stats.TotalReceived++
    dlq.mu.Unlock()

    select {
    case dlq.ch <- entry:
        // queued successfully
    default:
        // DLQ is full — we have to drop it
        dlq.mu.Lock()
        dlq.stats.TotalDropped++
        dlq.mu.Unlock()
        log.Printf("DLQ FULL — dropping message %s for subscriber %s",
            entry.Message.ID[:8], entry.SubscriberID[:8])
    }
}
```

**Why `select` with `default`?** If the DLQ itself is full, blocking the publisher would create a cascading failure. Better to log the loss and keep the system running.

---

## Processing the DLQ

```go
// TOPIC 10: Goroutine
// TOPIC 13: Context cancellation

func (dlq *DeadLetterQueue) Process(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second) // ◄── why 30s?
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            // Drain remaining entries before exiting
            dlq.drainRemaining()
            return

        case <-ticker.C:
            // Periodic: log DLQ stats
            // Why a ticker instead of logging on every entry?
            // Logging on every entry floods stdout under high failure rates.
            // A periodic summary (30s) gives visibility without noise.
            dlq.mu.RLock()
            size := len(dlq.ch)
            dlq.mu.RUnlock()
            if size > 0 {
                log.Printf("DLQ: %d messages pending", size)
            }

        case entry := <-dlq.ch:
            dlq.processEntry(entry)
        }
    }
}
```

### Why 30-Second Ticker Interval?

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │   INTERVAL    PROBLEM                                                    │
  ├──────────────────────────────────────────────────────────────────────────┤
  │   1 second    Too noisy — floods logs during transient failures         │
  │   30 seconds  Balanced — catches sustained failures, tolerates spikes  │
  │   5 minutes   Too slow — might not notice failures until too late       │
  └──────────────────────────────────────────────────────────────────────────┘
```

The ticker is for **monitoring**, not processing. Actual entries are processed in the `case entry := <-dlq.ch` branch. The ticker just provides periodic health snapshots.

### What Happens When DLQ Overflows?

```go
func (dlq *DeadLetterQueue) Push(entry DLQEntry) {
    dlq.mu.Lock()
    dlq.stats.TotalReceived++
    dlq.mu.Unlock()

    select {
    case dlq.ch <- entry:
        // queued successfully
    default:
        // DLQ is full — we have to drop it
        dlq.mu.Lock()
        dlq.stats.TotalDropped++  // ◄── track the loss
        dlq.mu.Unlock()
        log.Printf("DLQ FULL — dropping message %s for subscriber %s",
            entry.Message.ID[:8], entry.SubscriberID[:8])
    }
}
```

**Why log the drop but not panic?** The DLQ is a safety net, not a critical path. If the DLQ itself is full, the system has bigger problems. Panicking would crash the entire broker — losing ALL messages, not just the overflow. Logging + incrementing a counter lets monitoring alert operators.

**Monitoring query:** If `dlq_dropped_total` increases, something is fundamentally wrong — the DLQ is the last resort.

---

## DLQ in the Publish Path

```go
// internal/broker/broker.go
// How Publish uses the DLQ

func (b *inMemoryBroker) Publish(ctx context.Context, msg model.Message) error {
    // ... topic lookup ...

    fanOutErrors := t.fanOut(ctx, msg)

    // Send failed deliveries to DLQ
    for _, fe := range fanOutErrors {
        b.dlq.Push(DLQEntry{
            Message:      msg,
            SubscriberID: fe.SubscriberID,
            Error:        fe.Err.Error(),
            Timestamp:    time.Now(),
        })
    }

    return nil
}
```

---

## Backpressure Strategy Summary

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                    BACKPRESSURE DECISION TREE                            │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   Is subscriber channel full?                                            │
  │       │                                                                   │
  │       ├── YES ──► Which strategy?                                        │
  │       │              │                                                    │
  │       │              ├── DROP OLDEST                                     │
  │       │              │   • Remove oldest from buffer                     │
  │       │              │   • Add new message                               │
  │       │              │   • Increment Dropped counter                     │
  │       │              │   • USE: real-time data, latest matters           │
  │       │              │                                                    │
  │       │              ├── DROP NEWEST                                     │
  │       │              │   • Discard incoming message                      │
  │       │              │   • Increment Dropped counter                     │
  │       │              │   • USE: analytics, metrics                       │
  │       │              │                                                    │
  │       │              └── BLOCK                                           │
  │       │                  • Publisher waits for space                      │
  │       │                  • Or context timeout → error                    │
  │       │                  • USE: critical data, no loss allowed           │
  │       │                  • ⚠ Risk: slow subscriber blocks publisher     │
  │       │                                                                   │
  │       └── NO ──► Send message, return nil                                │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Why Not Retry Immediately?

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │   APPROACH               PROBLEM                       BETTER           │
  ├──────────────────────────────────────────────────────────────────────────┤
  │   Retry immediately      Retry storm, cascading        DLQ + delay      │
  │   Retry 3x in handler    Blocks publisher              DLQ + background │
  │   Silent drop            Data loss                     DLQ for recovery │
  │   Panic                  Crash the broker              DLQ + log        │
  └──────────────────────────────────────────────────────────────────────────┘
```

**Reference:** `06-software-patterns/07-retry-circuit-breaker.md` — Retry storms section

The DLQ acts like a circuit breaker: failures are isolated to the DLQ, the main publish path stays fast.

---

## Monitoring the DLQ

```go
func (dlq *DeadLetterQueue) Stats() DLQStats {
    dlq.mu.RLock()
    defer dlq.mu.RUnlock()
    dlq.stats.CurrentSize = len(dlq.ch)
    return dlq.stats
}
```

In production, expose this via a `/metrics` endpoint:

```
  GET /metrics

  dlq_received_total 1247
  dlq_retried_total  1198
  dlq_dropped_total  49
  dlq_current_size   0
```

---

## Integration with Broker Stats

```go
type BrokerStats struct {
    TotalTopics      int
    TotalSubscribers int
    DLQSize          int
    DLQStats         DLQStats  // nested stats
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
        DLQStats:         b.dlq.Stats(),
        Closed:           b.closed,
    }
}
```

---

## Next

All components are built. Time to put it all together in a **working demo**. → `09-demo.md`
