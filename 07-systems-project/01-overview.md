# Week 4 — Systems Project: Mini Message Queue

> **Goal:** Build an in-memory message broker from scratch using everything learned in Topics 1-19 and Software Patterns 01-08. No external dependencies — pure Go.

---

## Table of Contents

1. [What We're Building](#what-were-building) `[CORE]`
2. [Prerequisites](#prerequisites) `[CORE]`
3. [How Messages Flow (End-to-End)](#how-messages-flow-end-to-end) `[CORE]`
4. [Design Decisions](#design-decisions) `[CORE]`
   - [Project Structure](#1-project-structure)
   - [Layered Architecture](#2-layered-architecture)
   - [Dependency Injection](#3-dependency-injection)
   - [Per-Subscriber Bounded Channels](#4-per-subscriber-bounded-channels)
   - [Context Cancellation](#5-context-cancellation)
   - [Graceful Shutdown](#6-graceful-shutdown)
5. [Topics Covered](#topics-covered) `[CORE]`
6. [How to Approach This Project](#how-to-approach-this-project) `[CORE]`

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

## Prerequisites

Before starting this project, ensure you've completed:

**Core Topics (1-10):**
- [ ] Variables, types, zero values (Topic 2)
- [ ] Slices, maps, structs (Topics 3-5)
- [ ] Pointers and interfaces (Topics 6-7)
- [ ] Error handling and defer (Topics 8-9)
- [ ] Generics (Topic 10)

**Concurrency (Topics 11-19):**
- [ ] Goroutines and channels (Topics 11-12)
- [ ] Select and context (Topics 13-14)
- [ ] Mutex and sync primitives (Topic 16)
- [ ] Worker pools and fan-out (Topics 17-19)

**Software Patterns (01-08):**
- [ ] Project structure and dependency injection (Patterns 01, 04)
- [ ] Repository and service layer (Patterns 02, 03)
- [ ] Pub-sub and backpressure (Patterns 06, 08)

---

## How Messages Flow (End-to-End)

Before diving into design decisions, understand the complete lifecycle of a single message:

```
  STEP-BY-STEP: What happens when you call Publish("orders", msg)?

  ┌────────────────────────────────────────────────────────────────────────────┐
  │  Step 1: Publisher calls svc.Publish(ctx, "orders", payload, headers)     │
  │          │                                                                 │
  │          ▼                                                                 │
  │  Step 2: Service layer validates                                          │
  │          • Is "orders" a valid topic name? (length, charset)              │
  │          • Is payload under 1MB?                                           │
  │          • Is the topic reserved (_dlq, _system)?                         │
  │          │                                                                 │
  │          ▼                                                                 │
  │  Step 3: Service creates Message struct                                   │
  │          • Generates UUID for Message.ID                                  │
  │          • Sets Timestamp = time.Now()                                    │
  │          │                                                                 │
  │          ▼                                                                 │
  │  Step 4: Service calls broker.Publish(ctx, msg)                           │
  │          │                                                                 │
  │          ▼                                                                 │
  │  Step 5: Broker checks ctx.Done() — fast exit if shutting down           │
  │          │                                                                 │
  │          ▼                                                                 │
  │  Step 6: Broker looks up topic "orders" in map (RLock)                   │
  │          │                                                                 │
  │          ▼                                                                 │
  │  Step 7: Topic.fanOut(msg) — loop over all subscribers                   │
  │          │                                                                 │
  │          ├──► sub1.Publish(ctx, msg) ──► channel buffer ──► consumer      │
  │          ├──► sub2.Publish(ctx, msg) ──► channel buffer ──► consumer      │
  │          └──► sub3.Publish(ctx, msg) ──► channel buffer ──► consumer      │
  │          │                                                                 │
  │          ▼                                                                 │
  │  Step 8: If any sub.Publish() fails (buffer full) → DLQ entry            │
  │          │                                                                 │
  │          ▼                                                                 │
  │  Step 9: Return nil (success) or error to caller                          │
  │                                                                            │
  └────────────────────────────────────────────────────────────────────────────┘
```

**Key insight:** The publish path is **synchronous** — it blocks until all subscribers receive the message (or backpressure is applied). The consumer path is **asynchronous** — subscribers process messages in their own goroutines.

---

## Design Decisions

### 1. Project Structure

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

**Why `internal/`?** Go's `internal` directory convention is a compile-time visibility rule: packages inside `internal/` can only be imported by code in the parent directory tree. So `mini-mq/internal/broker` can be imported by `mini-mq/cmd/server/main.go` (same module), but *not* by code in another module. This is Go's way of enforcing encapsulation — the broker is an implementation detail, not a public API. Use `pkg/` for code you *want* external consumers to import.

### 2. Layered Architecture

**What:** The system is organized in concentric layers. Each layer only depends on the layer inside it.

**Why:** Dependencies that point inward mean the core domain (Message, Topic) never changes when the outer layers (HTTP, CLI, config) change. You can swap the HTTP handler for a gRPC handler without touching the broker.

**How:** Each layer defines an interface that the outer layer depends on. The concrete implementation is injected at startup.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                          │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  INFRASTRUCTURE (outermost)                                      │   │
  │   │  cmd/server/main.go — knows ALL concrete types                   │   │
  │   │  Wires dependencies, starts HTTP server, listens for signals     │   │
  │   │                                                                   │   │
  │   │   ┌────────────────────────────────────────────────────────────┐ │   │
  │   │   │  SERVICE LAYER                                            │ │   │
  │   │   │  internal/service/mq.go — validates input, orchestrates   │ │   │
  │   │   │  Depends on: broker.Broker interface (NOT concrete)       │ │   │
  │   │   │                                                            │ │   │
  │   │   │   ┌─────────────────────────────────────────────────────┐ │ │   │
  │   │   │   │  BROKER LAYER (Domain Core)                         │ │ │   │
  │   │   │   │  internal/broker/ — topic registry, fan-out,        │ │ │   │
  │   │   │   │  subscriber channels, backpressure, DLQ             │ │ │   │
  │   │   │   │  Depends on: model only                             │ │ │   │
  │   │   │   │                                                       │ │ │   │
  │   │   │   │   ┌──────────────────────────────────────────────┐  │ │ │   │
  │   │   │   │   │  MODEL (innermost — zero dependencies)       │  │ │ │   │
  │   │   │   │   │  internal/model/ — Message, TopicConfig      │  │ │ │   │
  │   │   │   │   │  No imports from outer layers                │  │ │ │   │
  │   │   │   │   │  Pure data types, no behavior                │  │ │ │   │
  │   │   │   │   └──────────────────────────────────────────────┘  │ │ │   │
  │   │   │   └─────────────────────────────────────────────────────┘ │ │   │
  │   │   └────────────────────────────────────────────────────────────┘ │   │
  │   └──────────────────────────────────────────────────────────────────┘   │
  │                                                                          │
  └──────────────────────────────────────────────────────────────────────────┘
```

**Dependency rule:** Arrows point inward. Model never imports broker. Broker never imports service. Service never imports cmd. If you see an import going outward, it's a violation.

### 3. Dependency Injection

**What:** `main.go` creates all concrete types and passes them as dependencies. No component creates its own dependencies.

**Why:** We can test the service with a mock broker. We can swap the in-memory broker for a Redis-backed one later without changing the service. Each component only knows about interfaces, not implementations.

**How:** Constructor functions accept interfaces and return structs. `main.go` is the only place that calls `broker.New()`.

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

### 4. Per-Subscriber Bounded Channels

**What:** Each subscriber gets a **buffered channel** of configurable size. When the buffer is full, we apply one of three backpressure strategies.

**Why:** Without bounded channels, a slow subscriber's buffer grows forever → OOM crash. Bounded channels force us to decide: drop old messages, drop new messages, or block the publisher.

**How:** The `Subscriber` struct holds `make(chan model.Message, bufferSize)`. The `Publish()` method uses `select` with `default` for non-blocking backpressure.

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

### 5. Context Cancellation

**What:** Every operation accepts `context.Context`. When cancelled, all components stop what they're doing.

**Why:** Without context, there's no way to tell goroutines to stop. The process would hang forever on shutdown, or require `os.Exit()` which loses messages.

**How:** `main.go` creates `ctx, cancel := context.WithCancel(...)`. On SIGINT, `cancel()` fires, and every goroutine checks `<-ctx.Done()`.
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

### 6. Graceful Shutdown

**What:** On shutdown: stop accepting new messages, drain in-flight messages to subscribers, then exit.

**Why:** Calling `os.Exit()` immediately loses all messages in subscriber buffers. Graceful shutdown ensures zero message loss — every buffered message reaches its handler before the process exits.

**How:** Close subscriber channels → wait for consumer goroutines to drain → `WaitGroup.Wait()` with timeout → print stats → exit.

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
| 03 | `03-subscriber.md` | Channels, goroutines, backpressure (Topics 11, 12, 17) |
| 04 | `04-broker-core.md` | Maps, mutex, fan-out, select (Topics 4, 13, 16, 19) |
| 05 | `05-service-layer.md` | Service pattern, validation (Pattern 03) |
| 06 | `06-wiring.md` | DI, main.go wiring (Pattern 04) |
| 07 | `07-graceful-shutdown.md` | Context, WaitGroup, signal handling (Topics 14, 15) |
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
