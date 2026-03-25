# Week 4 — Systems Project: Mini Message Queue

> **Goal:** Build an in-memory message broker from scratch using everything learned in Topics 1-19 and Software Patterns 01-08. No external dependencies — pure Go.

---

## What We're Building

A **topic-based pub-sub message broker** that runs in-process. Publishers send messages to named topics. Subscribers receive messages on per-subscriber channels. The broker handles backpressure, cancellation, and graceful shutdown.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                      MINI MESSAGE QUEUE                                  │
  │                                                                          │
  │   Publishers                        Broker                    Subscribers│
  │                                                                          │
  │   ┌──────────┐              ┌──────────────────────┐      ┌──────────┐  │
  │   │ OrderSvc │──publish───►│                      │      │ EmailSvc │  │
  │   └──────────┘              │                      │─────►│ (sub 1)  │  │
  │                             │   Topic: "orders"    │      └──────────┘  │
  │   ┌──────────┐              │                      │      ┌──────────┐  │
  │   │ APISvc   │──publish───►│   • fan-out to all   │─────►│ Analytics│  │
  │   └──────────┘              │   • per-sub channel  │      │ (sub 2)  │  │
  │                             │   • backpressure     │      └──────────┘  │
  │   ┌──────────┐              │   • dead letter Q    │      ┌──────────┐  │
  │   │ Worker   │──publish───►│                      │─────►│ Logger   │  │
  │   └──────────┘              └──────────────────────┘      │ (sub 3)  │  │
  │                                                            └──────────┘  │
  │                                                                          │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Design Decisions

Every decision maps to a concept we've learned.

### 1. Project Structure → `06-software-patterns/01-project-structure.md`

```
  mini-mq/
  ├── cmd/
  │   └── server/
  │       └── main.go              # Entry point, DI wiring
  ├── internal/
  │   ├── model/
  │   │   ├── message.go           # Message struct
  │   │   └── topic.go             # Topic config
  │   ├── broker/
  │   │   ├── broker.go            # Broker interface + implementation
  │   │   ├── topic.go             # Topic registry
  │   │   ├── subscriber.go        # Subscriber with bounded channel
  │   │   └── dlq.go               # Dead letter queue
  │   ├── service/
  │   │   └── mq.go                # Service layer (validation, orchestration)
  │   └── config/
  │       └── config.go            # Configuration
  └── go.mod
```

**Why `internal/`?** This broker is application code — not a public library. Nobody outside this project should import it. (Topic 01: internal vs pkg)

### 2. Layered Architecture → `06-software-patterns/05-clean-architecture.md`

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                          │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  INFRASTRUCTURE                                                  │   │
  │   │  cmd/server/main.go — wires everything, starts HTTP + broker    │   │
  │   │                                                                   │   │
  │   │   ┌────────────────────────────────────────────────────────────┐ │   │
  │   │   │  SERVICE LAYER                                            │ │   │
  │   │   │  internal/service/mq.go — validate, orchestrate, publish  │ │   │
  │   │   │                                                            │ │   │
  │   │   │   ┌─────────────────────────────────────────────────────┐ │ │   │
  │   │   │   │  BROKER LAYER (Domain)                              │ │ │   │
  │   │   │   │  internal/broker/ — topic registry, fan-out,        │ │ │   │
  │   │   │   │  subscriber channels, backpressure, DLQ             │ │ │   │
  │   │   │   │                                                       │ │ │   │
  │   │   │   │   ┌──────────────────────────────────────────────┐  │ │ │   │
  │   │   │   │   │  MODEL (innermost)                            │  │ │ │   │
  │   │   │   │   │  internal/model/ — Message, Topic types       │  │ │ │   │
  │   │   │   │   │  No imports from outer layers                 │  │ │ │   │
  │   │   │   │   └──────────────────────────────────────────────┘  │ │ │   │
  │   │   │   └─────────────────────────────────────────────────────┘ │ │   │
  │   │   └────────────────────────────────────────────────────────────┘ │   │
  │   └──────────────────────────────────────────────────────────────────┘   │
  │                                                                          │
  └──────────────────────────────────────────────────────────────────────────┘
```

**Dependencies point inward.** Model never imports broker. Broker never imports service. Service never imports cmd.

### 3. Dependency Injection → `06-software-patterns/04-dependency-injection.md`

```go
// cmd/server/main.go — the ONLY place that knows concrete types

func main() {
    // 1. Create config
    cfg := config.Default()

    // 2. Create broker (concrete implementation)
    broker := broker.New(cfg.Broker)

    // 3. Create service (inject broker interface)
    svc := service.NewMessageQueue(broker)

    // 4. Create HTTP handler (inject service interface)
    handler := handler.New(svc)

    // 5. Start
    handler.Run(":8080")
}
```

**Why DI?** We can test the service with a mock broker. We can swap the in-memory broker for a Redis-backed one later without changing the service.

### 4. Per-Subscriber Bounded Channels → `05-concurrency/16-worker-pools.md` + `06-software-patterns/08-backpressure-strategies.md`

Each subscriber gets a **buffered channel**. When the buffer is full, we apply backpressure.

```
  Publisher                    Broker                         Subscriber
     │                            │                                │
     │  publish("orders", msg)    │                                │
     │ ──────────────────────────►│                                │
     │                            │                                │
     │                            │   ┌──────────────────────┐     │
     │                            │   │  Subscriber Channel  │     │
     │                            │   │  (buffered, cap=100) │     │
     │                            │   │  [m1][m2][m3]...[m99]│─────│──► consumer
     │                            │   └──────────────────────┘     │
     │                            │          ▲                     │
     │                            │          │                     │
     │                            │   If FULL: apply backpressure │
     │                            │   (drop oldest, drop newest,  │
     │                            │    or block publisher)         │
```

### 5. Context Cancellation → `05-concurrency/13-context.md`

Every operation accepts `context.Context`. When cancelled, the broker stops:
- Accepting new publishes
- Dispatching to subscribers
- Waiting for subscriber drains

```
  main()
    │
    ▼
  ctx, cancel := context.WithCancel(context.Background())
    │
    ├──► broker.Start(ctx)     ◄── listens to ctx.Done()
    ├──► subscriber.Consume(ctx) ◄── stops when ctx.Done()
    └──► httpServer.ListenAndServe()
            │
            ▼
         SIGINT received → cancel() → all goroutines see ctx.Done()
```

### 6. Graceful Shutdown → `05-concurrency/14-waitgroup.md`

On shutdown: stop accepting new messages, drain in-flight messages to subscribers, then exit.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                      GRACEFUL SHUTDOWN SEQUENCE                          │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   1. SIGINT / SIGTERM received                                          │
  │          │                                                                │
  │          ▼                                                                │
  │   2. context.Cancel() called                                            │
  │          │                                                                │
  │          ▼                                                                │
  │   3. Broker stops accepting new publishes                               │
  │          │                                                                │
  │          ▼                                                                │
  │   4. Broker drains in-flight messages to subscribers                    │
  │          │                                                                │
  │          ▼                                                                │
  │   5. Subscribers finish processing current messages                     │
  │          │                                                                │
  │          ▼                                                                │
  │   6. WaitGroup.Wait() — all goroutines done                            │
  │          │                                                                │
  │          ▼                                                                │
  │   7. Print final stats, exit                                            │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Topics Covered

| # | File | Concepts Used |
|---|------|---------------|
| 01 | This file | Architecture, design decisions |
| 02 | `02-message-model.md` | Structs, zero values, interfaces (Topics 5, 7) |
| 03 | `03-subscriber.md` | Channels, goroutines, backpressure (Topics 10, 11, 16) |
| 04 | `04-broker-core.md` | Maps, mutex, fan-out, select (Topics 4, 12, 15, 18) |
| 05 | `05-service-layer.md` | Service pattern, validation (Pattern 03) |
| 06 | `06-wiring.md` | DI, main.go wiring (Pattern 04) |
| 07 | `07-graceful-shutdown.md` | Context, WaitGroup, signal handling (Topics 13, 14) |
| 08 | `08-resilience.md` | Dead letter queue, backpressure strategies (Pattern 07, 08) |
| 09 | `09-demo.md` | Full walkthrough, test scenarios |

### Topics & Patterns Applied

| Concept | Where Used |
|---------|------------|
| **Topic 03: Arrays vs Slices** | `make([]model.TopicConfig, 0, len(b.topics))` — pre-allocated slice in `ListTopics()` |
| **Topic 06: Pointers** | `*Subscriber`, `*topic` — shared mutable state passed by pointer across goroutines |
| **Topic 17: Pipelines** | Message flow: `publish → broker.fanOut → subscriber.Publish → handler` — each stage connected by channels |
| **Pattern 06: Pub-Sub** | The broker IS a pub-sub system — topic-based fan-out to subscriber channels |

---

## How to Approach This Project

1. **Read each file in order** (01 through 09)
2. **Understand the decision** before seeing the code
3. **Implement the code** in your own `mini-mq/` project
4. **Run the demo** at the end to verify everything works

Each code block includes a comment showing which topic/pattern it uses.

---

## What This Is NOT

- Not a distributed system (single process, in-memory)
- Not persistent (messages lost on crash — that's a future project)
- Not networked (no TCP/gRPC — in-process only)
- Not a replacement for Kafka/RabbitMQ (it's a learning exercise)

## What This IS

- A working pub-sub broker in ~300 lines of Go
- A demonstration of every concept from Topics 1-19
- A production-style project structure
- A foundation you can extend (add persistence, networking, etc.)
