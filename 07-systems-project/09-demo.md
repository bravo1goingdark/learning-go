# 09 — Working Demo

> **Goal:** See the entire system working end-to-end. This file shows the demo code, what to expect, and how to verify each feature.

---

## File Structure (Final)

After implementing everything from files 01-08, your project should look like:

```
  mini-mq/
  ├── go.mod
  ├── cmd/
  │   └── server/
  │       └── main.go               # Entry point (file 06)
  ├── internal/
  │   ├── model/
  │   │   ├── message.go            # Message struct (file 02)
  │   │   ├── topic.go              # TopicConfig (file 02)
  │   │   └── errors.go             # Sentinel errors (file 02)
  │   ├── broker/
  │   │   ├── broker.go             # Interface + implementation (file 04)
  │   │   ├── topic.go              # Topic with fan-out (file 04)
  │   │   ├── subscriber.go         # Subscriber + backpressure (file 03)
  │   │   └── dlq.go                # Dead letter queue (file 08)
  │   ├── service/
  │   │   └── mq.go                 # Service layer (file 05)
  │   └── config/
  │       └── config.go             # Configuration (file 06)
  └── main_test.go                  # Tests
```

---

## go.mod

```
  module mini-mq

  go 1.21
```

No external dependencies. Pure Go standard library.

---

## The Demo (in main.go)

```go
// cmd/server/main.go
// This is the complete runnable entry point

package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "mini-mq/internal/broker"
    "mini-mq/internal/config"
    "mini-mq/internal/model"
    "mini-mq/internal/service"
)

func main() {
    cfg := config.Default()
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Create components (DI wiring)
    b := broker.New(broker.Config{
        DefaultBufferSize: cfg.Broker.DefaultBufferSize,
        MaxTopics:         cfg.Broker.MaxTopics,
    })
    svc := service.NewMessageQueue(b)

    // Signal handler
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    var wg sync.WaitGroup

    // Start broker
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := b.Start(ctx); err != nil {
            log.Printf("broker: %v", err)
            cancel()
        }
    }()

    // Run demo
    wg.Add(1)
    go func() {
        defer wg.Done()
        runDemo(ctx, svc)
    }()

    // Wait for shutdown
    log.Println("mini-mq running. Ctrl+C to stop.")
    <-sigCh
    log.Println("shutting down...")
    cancel()

    // Graceful shutdown with timeout
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer shutdownCancel()

    if err := b.Shutdown(shutdownCtx); err != nil {
        log.Printf("shutdown error: %v", err)
    }
    wg.Wait()

    stats := b.Stats()
    log.Printf("final: topics=%d subs=%d dlq=%d", stats.TotalTopics, stats.TotalSubscribers, stats.DLQSize)
}
```

---

## Demo Scenario

```go
func runDemo(ctx context.Context, svc service.MessageQueue) {
    // ─── SCENARIO 1: Basic pub-sub ─────────────────────────────
    log.Println("=== Scenario 1: Basic Pub-Sub ===")

    svc.CreateTopic(ctx, "orders")

    // Two subscribers for the same topic
    sub1, _ := svc.Subscribe(ctx, "orders", 10)
    sub2, _ := svc.Subscribe(ctx, "orders", 10)

    sub1.Start(ctx, func(msg model.Message) error {
        log.Printf("[email-svc] %s", string(msg.Payload))
        return nil
    })

    sub2.Start(ctx, func(msg model.Message) error {
        // Simulate slow processing
        time.Sleep(100 * time.Millisecond)
        log.Printf("[analytics] %s", string(msg.Payload))
        return nil
    })

    // Publish 5 messages — both subscribers receive all 5
    for i := 0; i < 5; i++ {
        payload := fmt.Sprintf("order-%d", i)
        svc.Publish(ctx, "orders", []byte(payload), nil)
    }

    time.Sleep(2 * time.Second) // let subscribers process


    // ─── SCENARIO 2: Backpressure (DropOldest) ─────────────────
    log.Println("=== Scenario 2: Backpressure ===")

    svc.CreateTopic(ctx, "events")

    // Slow subscriber with tiny buffer
    slowSub, _ := svc.Subscribe(ctx, "events", 5)
    slowSub.Start(ctx, func(msg model.Message) error {
        time.Sleep(500 * time.Millisecond) // very slow
        return nil
    })

    // Blast 50 messages — buffer fills, oldest dropped
    for i := 0; i < 50; i++ {
        payload := fmt.Sprintf("event-%d", i)
        svc.Publish(ctx, "events", []byte(payload), nil)
    }

    time.Sleep(3 * time.Second)
```

### Why `time.Sleep` Instead of Proper Synchronization?

This is a **demo**, not production code. `time.Sleep` gives the subscriber goroutines time to process. In production, you'd use:
- `sync.WaitGroup` to wait for known goroutines
- `<-sub.done` to wait for a specific subscriber
- A drain function that returns when all buffers are empty

`time.Sleep` is simpler for learning but imprecise — use it in demos and tests, never in production.


    // ─── SCENARIO 3: Multiple Topics ───────────────────────────
    log.Println("=== Scenario 3: Multiple Topics ===")

    svc.CreateTopic(ctx, "payments")
    svc.CreateTopic(ctx, "notifications")

    paySub, _ := svc.Subscribe(ctx, "payments", 10)
    notifSub, _ := svc.Subscribe(ctx, "notifications", 10)

    paySub.Start(ctx, func(msg model.Message) error {
        log.Printf("[payment-processor] %s", string(msg.Payload))
        return nil
    })

    notifSub.Start(ctx, func(msg model.Message) error {
        log.Printf("[notification-svc] %s", string(msg.Payload))
        return nil
    })

    // Messages go to their respective topics
    svc.Publish(ctx, "payments", []byte("charge $50"), nil)
    svc.Publish(ctx, "notifications", []byte("welcome email"), nil)
    svc.Publish(ctx, "payments", []byte("refund $20"), nil)

    time.Sleep(2 * time.Second)


    // ─── SCENARIO 4: Error in handler → DLQ ────────────────────
    log.Println("=== Scenario 4: Errors → DLQ ===")

    svc.CreateTopic(ctx, "critical")

    critSub, _ := svc.Subscribe(ctx, "critical", 10)
    critSub.Start(ctx, func(msg model.Message) error {
        // Fail every other message
        if msg.Payload[len(msg.Payload)-1]%2 == 0 {
            return fmt.Errorf("processing failed")
        }
        log.Printf("[critical-handler] processed: %s", string(msg.Payload))
        return nil
    })

    for i := 0; i < 10; i++ {
        svc.Publish(ctx, "critical", []byte(fmt.Sprintf("msg-%d", i)), nil)
    }

    time.Sleep(2 * time.Second)


    // ─── SCENARIO 5: Graceful Shutdown ─────────────────────────
    log.Println("=== Scenario 5: Waiting for shutdown signal ===")
    log.Println("Press Ctrl+C to trigger graceful shutdown")

    <-ctx.Done() // wait for shutdown signal

    // On shutdown: all subscriber buffers are drained
    // You'll see remaining messages processed before exit
}
```

---

## Expected Output

```
  2026/03/25 12:00:00 mini-mq running. Ctrl+C to stop.
  2026/03/25 12:00:00 === Scenario 1: Basic Pub-Sub ===
  2026/03/25 12:00:00 [email-svc] order-0
  2026/03/25 12:00:00 [email-svc] order-1
  2026/03/25 12:00:00 [email-svc] order-2
  2026/03/25 12:00:00 [email-svc] order-3
  2026/03/25 12:00:00 [email-svc] order-4
  2026/03/25 12:00:00 [analytics] order-0
  2026/03/25 12:00:01 [analytics] order-1
  2026/03/25 12:00:01 [analytics] order-2
  2026/03/25 12:00:01 [analytics] order-3
  2026/03/25 12:00:01 [analytics] order-4
  2026/03/25 12:00:01 === Scenario 2: Backpressure ===
  2026/03/25 12:00:04 === Scenario 3: Multiple Topics ===
  2026/03/25 12:00:04 [payment-processor] charge $50
  2026/03/25 12:00:04 [notification-svc] welcome email
  2026/03/25 12:00:04 [payment-processor] refund $20
  2026/03/25 12:00:04 === Scenario 4: Errors → DLQ ===
  2026/03/25 12:00:04 DLQ: msg=msg-0 topic=critical sub=abc12345 err=processing failed attempt=0
  2026/03/25 12:00:04 [critical-handler] processed: msg-1
  2026/03/25 12:00:04 DLQ: msg=msg-2 topic=critical sub=abc12345 err=processing failed attempt=0
  2026/03/25 12:00:04 [critical-handler] processed: msg-3
  ...
  2026/03/25 12:00:06 === Scenario 5: Waiting for shutdown signal ===
  2026/03/25 12:00:06 Press Ctrl+C to trigger graceful shutdown

  ^C
  2026/03/25 12:00:10 shutting down...
  2026/03/25 12:00:10 broker: stopping new publishes
  2026/03/25 12:00:10 subscriber abc12345: drained 2 messages on shutdown
  2026/03/25 12:00:10 broker: all subscribers drained
  2026/03/25 12:00:10 broker: all goroutines stopped
  2026/03/25 12:00:10 final: topics=4 subs=6 dlq=0
```

---

## Verification Checklist

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │   FEATURE                    HOW TO VERIFY                              │
  ├──────────────────────────────────────────────────────────────────────────┤
  │   ✓ Topic creation           Log shows topic created, no errors        │
  │   ✓ Fan-out                  Both sub1 and sub2 receive all messages   │
  │   ✓ Backpressure             Slow sub doesn't crash, oldest dropped    │
  │   ✓ Multiple topics          Payments go to paySub, notifs to notifSub │
  │   ✓ DLQ                      Failed messages logged in DLQ             │
  │   ✓ Graceful shutdown        Ctrl+C drains remaining messages          │
  │   ✓ No goroutine leaks       "all goroutines stopped" in final logs    │
  │   ✓ Context cancellation     All components respect ctx.Done()         │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Build & Run

```bash
  # Create the project
  mkdir mini-mq && cd mini-mq
  go mod init mini-mq

  # Create directory structure
  mkdir -p cmd/server internal/model internal/broker internal/service internal/config

  # Implement each file (files 02-08)
  # ...

  # Build
  go build -o mini-mq ./cmd/server

  # Run
  ./mini-mq

  # Or run directly
  go run ./cmd/server
```

---

## Testing Structure

Don't just run the demo — write proper tests. Each layer should be testable independently.

### Test the Broker (Unit Tests)

```go
// internal/broker/broker_test.go

func TestPublish_NonexistentTopic(t *testing.T) {
    b := New(DefaultConfig())
    ctx := context.Background()

    msg := model.Message{ID: "1", Topic: "nope", Payload: []byte("hi")}
    err := b.Publish(ctx, msg)

    if !errors.Is(err, model.ErrTopicNotFound) {
        t.Fatalf("expected ErrTopicNotFound, got: %v", err)
    }
}

func TestFanOut_DeliversToAllSubscribers(t *testing.T) {
    b := New(DefaultConfig())
    ctx := context.Background()

    b.CreateTopic(ctx, model.DefaultTopicConfig("test"))

    received1 := make(chan model.Message, 1)
    received2 := make(chan model.Message, 1)

    sub1 := NewSubscriber("s1", "test", 10, DropOldest)
    sub1.Start(ctx, func(msg model.Message) error {
        received1 <- msg
        return nil
    })

    sub2 := NewSubscriber("s2", "test", 10, DropOldest)
    sub2.Start(ctx, func(msg model.Message) error {
        received2 <- msg
        return nil
    })

    b.Subscribe(ctx, "test", sub1)
    b.Subscribe(ctx, "test", sub2)

    msg := model.Message{ID: "1", Topic: "test", Payload: []byte("hi")}
    b.Publish(ctx, msg)

    // Both subscribers receive the message
    select {
    case m := <-received1:
        if m.ID != "1" { t.Errorf("sub1 got wrong msg") }
    case <-time.After(time.Second):
        t.Fatal("sub1 didn't receive")
    }

    select {
    case m := <-received2:
        if m.ID != "1" { t.Errorf("sub2 got wrong msg") }
    case <-time.After(time.Second):
        t.Fatal("sub2 didn't receive")
    }
}
```

### Test the Service (With Mock Broker)

```go
// internal/service/mq_test.go

type mockBroker struct {
    createTopicCalled bool
    lastTopicName     string
}

func (m *mockBroker) CreateTopic(_ context.Context, cfg model.TopicConfig) error {
    m.createTopicCalled = true
    m.lastTopicName = cfg.Name
    return nil
}
// ... implement other interface methods ...

func TestCreateTopic_ValidatesName(t *testing.T) {
    mock := &mockBroker{}
    svc := NewMessageQueue(mock)

    err := svc.CreateTopic(context.Background(), "")

    if !errors.Is(err, ErrInvalidTopicName) {
        t.Fatalf("expected ErrInvalidTopicName, got: %v", err)
    }
    if mock.createTopicCalled {
        t.Error("broker should NOT be called for invalid input")
    }
}
```

### Test Backpressure (Integration)

```go
func TestBackpressure_DropOldest(t *testing.T) {
    b := New(DefaultConfig())
    ctx := context.Background()

    b.CreateTopic(ctx, model.DefaultTopicConfig("test"))

    // Tiny buffer — fills immediately
    sub := NewSubscriber("s1", "test", 3, DropOldest)
    // Don't start consuming — messages pile up

    b.Subscribe(ctx, "test", sub)

    // Publish 10 messages into a buffer of 3
    for i := 0; i < 10; i++ {
        msg := model.Message{ID: fmt.Sprintf("%d", i), Topic: "test", Payload: []byte("x")}
        b.Publish(ctx, msg)
    }

    stats := sub.Stats()
    if stats.Dropped == 0 {
        t.Error("expected some messages to be dropped")
    }
    t.Logf("dropped: %d, received: %d", stats.Dropped, stats.Received)
}
```

---

## What to Extend Next

Once the basic broker works, here are production-grade extensions:

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │   EXTENSION                  CONCEPTS USED                              │
  ├──────────────────────────────────────────────────────────────────────────┤
  │   HTTP API (publish/subscribe via REST)   Handler layer (Pattern 01)    │
  │   Persistent storage (write to disk)      Repository pattern (Pattern 02)│
  │   Consumer groups (load balancing)        Worker pools (Topic 16)       │
  │   Message ordering (per-key partitioning) Channels + mutex (Topic 15)   │
  │   Metrics endpoint (/metrics)             Struct fields (Topic 5)       │
  │   Config from YAML/env                    Functional options (Topic 5)  │
  │   Redis-backed broker                     Interface swap (Topic 7)      │
  │   Retry with exponential backoff          Retry pattern (Pattern 07)    │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Concepts Applied — Full Map

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │   TOPIC / PATTERN              WHERE USED                               │
  ├──────────────────────────────────────────────────────────────────────────┤
  │   01 Go Toolchain              go mod, go build, go run                 │
  │   02 Variables/Zero Values     Constants, iota, type declarations       │
  │   03 Arrays vs Slices          Subscriber buffer (slice not needed)     │
  │   04 Maps                      Topic registry, subscriber map           │
  │   05 Structs/Methods           Message, TopicConfig, Subscriber, Broker │
  │   06 Pointers                  *Subscriber, *topic (shared state)       │
  │   07 Interfaces                Broker interface, MessageQueue interface  │
  │   08 Error Handling            Sentinel errors, error wrapping          │
  │   09 Defer                     Resource cleanup, shutdown ordering      │
  │   10 Goroutines                Subscriber consumer, DLQ processor      │
  │   11 Channels                  Per-subscriber buffer, DLQ channel      │
  │   12 Select                    Backpressure, non-blocking ops           │
  │   13 Context                   Cancellation, propagation, shutdown      │
  │   14 WaitGroup                 Track active goroutines                  │
  │   15 Mutex vs Channels         RWMutex for topic/subscriber maps        │
  │   16 Worker Pools              Bounded subscriber channels              │
  │   17 Pipelines                 Message flow: pub → broker → sub        │
  │   18 Fan-In/Fan-Out           One message → all subscribers            │
  │   ──────────────────────────────────────────────────────────────────── │
  │   Pattern 01 Project Structure cmd/, internal/, layers                  │
  │   Pattern 02 Repository        (extensible — add persistence)           │
  │   Pattern 03 Service Layer     Validation, orchestration                │
  │   Pattern 04 Dependency Inj.   Constructor injection, mockable          │
  │   Pattern 05 Clean Arch.       Model → Broker → Service → Cmd          │
  │   Pattern 06 Pub-Sub           The broker IS a pub-sub system          │
  │   Pattern 07 Retry/Circuit     DLQ isolates failures                    │
  │   Pattern 08 Backpressure      DropOldest, DropNewest, Block            │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Milestone Complete

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                           │
  │   ✓ Topic-based pub-sub          Routes by topic name                   │
  │   ✓ Per-subscriber channels      Buffered, independent                  │
  │   ✓ Backpressure handling        3 strategies, configurable             │
  │   ✓ Context cancellation         Every operation respects ctx           │
  │   ✓ Graceful shutdown            Drain + wait + timeout                 │
  │   ✓ Dead letter queue            Failed messages preserved              │
  │   ✓ Production structure         Clean architecture, DI, interfaces     │
  │                                                                           │
  │            IN-MEMORY MESSAGE BROKER — COMPLETE                            │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```
