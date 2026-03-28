# 06 — Wiring: main.go

> **Decision:** `main.go` is the ONLY place that knows concrete types. It creates everything, wires dependencies, and starts the application.

**Reference:** `06-software-patterns/04-dependency-injection.md` + `01-project-structure.md`

---

## Table of Contents

1. [The Dependency Graph](#the-dependency-graph) `[CORE]`
2. [Configuration](#configuration) `[CORE]`
3. [main.go — The Entry Point](#maingo--the-entry-point) `[CORE]`
4. [Wiring Diagram](#wiring-diagram) `[CORE]`
5. [Why This Order Matters](#why-this-order-matters) `[CORE]`
6. [The Demo Function](#the-demo-function) `[PRODUCTION]`

---

## The Dependency Graph

```
  ┌──────────────────────┐
  │      main.go         │  ◄── knows ALL concrete types
  │                      │
  │  1. Create config    │
  │  2. Create broker    │
  │  3. Create service   │
  │  4. Start goroutines │
  │  5. Wait for signals │
  │  6. Shutdown         │
  └──────┬───────────────┘
         │
         │ wires
         ▼
  ┌──────────────────────┐
  │  service.MessageQueue │  ◄── knows broker.Broker interface
  └──────┬───────────────┘
         │ uses
         ▼
  ┌──────────────────────┐
  │  broker.Broker       │  ◄── knows broker.subscriber, broker.topic
  └──────┬───────────────┘
         │ uses
         ▼
  ┌──────────────────────┐
  │  model.Message       │  ◄── knows nothing
  └──────────────────────┘
```

**Rule:** Dependencies point DOWN. Nobody imports `cmd/`.

---

## Configuration

**What:** A plain struct holding all configurable values — buffer sizes, timeouts, limits.

**Why a separate config package?** (Pattern 01: Project structure) Configuration is a cross-cutting concern. Both broker and server read from it. Keeping it in `internal/config/` avoids circular imports. Using `Default()` with sensible defaults means you don't need a config file to run the demo.

**How:** Nested structs (`Config.Broker.MaxTopics`) organize related settings. The `Default()` function returns a fully populated config — no nil fields, no zero-value surprises.

```go
// internal/config/config.go
// TOPIC 5: Struct with defaults, zero values

package config

import "time"

type Config struct {
    Broker BrokerConfig
    Server ServerConfig
}

type BrokerConfig struct {
    DefaultBufferSize int           // subscriber channel capacity
    MaxTopics         int           // max topics allowed
    MaxSubscribers    int           // max subscribers per topic
    MaxMessageBytes   int           // max message payload size
    ShutdownTimeout   time.Duration // graceful shutdown deadline
}

type ServerConfig struct {
    Addr            string
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
    ShutdownTimeout time.Duration
}

func Default() Config {
    return Config{
        Broker: BrokerConfig{
            DefaultBufferSize: 100,
            MaxTopics:         1000,
            MaxSubscribers:    100,
            MaxMessageBytes:   1_048_576, // 1MB
            ShutdownTimeout:   10 * time.Second,
        },
        Server: ServerConfig{
            Addr:            ":8080",
            ReadTimeout:     5 * time.Second,
            WriteTimeout:    10 * time.Second,
            ShutdownTimeout: 15 * time.Second,
        },
    }
}
```

**Why a separate config package?** (Pattern 01: Project structure) Configuration is a cross-cutting concern. Both broker and server read from it. Keeping it in `internal/config/` avoids circular imports.

---

## main.go — The Entry Point

```go
// cmd/server/main.go
// TOPIC 13: Context with cancellation
// TOPIC 14: WaitGroup for goroutine tracking
// TOPIC 9:  Defer for cleanup
// PATTERN 04: Dependency injection

package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "sync"
    "syscall"

    "mini-mq/internal/broker"
    "mini-mq/internal/config"
    "mini-mq/internal/service"
)

func main() {
    // 1. Load configuration
    cfg := config.Default()

    // 2. Create context with cancellation (for shutdown)
    //    TOPIC 13: context.WithCancel
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // 3. Create broker (concrete implementation)
    //    PATTERN 04: constructor injection
    b := broker.New(broker.Config{
        DefaultBufferSize: cfg.Broker.DefaultBufferSize,
        MaxTopics:         cfg.Broker.MaxTopics,
    })

    // 4. Create service (inject broker interface)
    //    TOPIC 7: accepts interface, not concrete type
    svc := service.NewMessageQueue(b)

    // 5. Setup signal handler for graceful shutdown
    //    TOPIC 13: signal handling with channels
    sigCh := make(chan os.Signal, 1)           // ◄── buffer=1, see below
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    // 6. WaitGroup for all goroutines
    //    TOPIC 14: sync.WaitGroup
    var wg sync.WaitGroup

    // 7. Start broker
    wg.Add(1)  // ◄── MUST be before `go`, see below
    go func() {
        defer wg.Done()
        if err := b.Start(ctx); err != nil {
            log.Printf("broker error: %v", err)
            cancel() // trigger shutdown
        }
    }()

    // 8. Start demo publishers/subscribers (temporary — file 09 shows real usage)
    wg.Add(1)
    go func() {
        defer wg.Done()
        runDemo(ctx, svc)
    }()
```

### Why `make(chan os.Signal, 1)` — Buffer Size 1?

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                           │
  │   make(chan os.Signal, 0)  UNBUFFERED                                    │
  │   • Signal blocks until we read from the channel                         │
  │   • If we're busy elsewhere, the OS can't deliver the signal            │
  │   • Risk: missed signals                                                 │
  │                                                                           │
  │   make(chan os.Signal, 1)  BUFFERED (CORRECT)                            │
  │   • OS deposits the signal into the buffer                               │
  │   • We read it when ready                                                │
  │   • Buffer=1 is enough — we only need one signal to start shutdown      │
  │                                                                           │
  │   make(chan os.Signal, 100)  OVER-ALLOCATED                              │
  │   • Wastes memory                                                        │
  │   • No benefit — we only care about "signal received"                    │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

### Why `wg.Add(1)` BEFORE `go func()`?

```
  // BAD: race condition
  go func() {
      wg.Add(1)       // ◄── might run AFTER wg.Wait()
      defer wg.Done()
      work()
  }()
  wg.Wait()           // ◄── might see Add(0) and return immediately

  // GOOD: guaranteed ordering
  wg.Add(1)           // ◄── happens-before the goroutine starts
  go func() {
      defer wg.Done()
      work()
  }()
  wg.Wait()           // ◄── correctly waits
```

If `wg.Add(1)` runs inside the goroutine, the main goroutine might reach `wg.Wait()` before the child goroutine has called `Add(1)`. `wg.Wait()` with a counter of 0 returns immediately — goroutine leaked.

### Why `defer cancel()` at the Top?

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel() // ◄── always call, even on panic
```

**Reference:** Topic 9 — Defer. If `main()` panics (unexpected nil pointer, etc.), `defer cancel()` still runs. Without it, child goroutines block forever waiting on `ctx.Done()`. The `defer` guarantees cleanup regardless of how main exits.

    // 9. Wait for shutdown signal
    log.Println("mini-mq started. Press Ctrl+C to stop.")
    <-sigCh

    // 10. Graceful shutdown
    log.Println("shutting down...")
    cancel() // signal all goroutines to stop

    // 11. Create shutdown context with timeout
    shutdownCtx, shutdownCancel := context.WithTimeout(
        context.Background(),
        cfg.Broker.ShutdownTimeout,
    )
    defer shutdownCancel()

    // 12. Shutdown broker (drains subscribers)
    if err := b.Shutdown(shutdownCtx); err != nil {
        log.Printf("broker shutdown error: %v", err)
    }

    // 13. Wait for all goroutines
    wg.Wait()

    // 14. Print final stats
    stats := b.Stats()
    log.Printf("final stats: topics=%d subscribers=%d dlq=%d",
        stats.TotalTopics, stats.TotalSubscribers, stats.DLQSize)

    log.Println("goodbye.")
}
```

---

## Wiring Diagram

```
  main()
    │
    ├──► cfg := config.Default()
    │
    ├──► ctx, cancel := context.WithCancel(context.Background())
    │
    ├──► b := broker.New(brokerConfig)
    │         │
    │         └──► creates: map[string]*topic{}
    │              creates: DeadLetterQueue
    │
    ├──► svc := service.NewMessageQueue(b)
    │         │
    │         └──► stores: broker reference (interface)
    │
    ├──► signal.Notify(sigCh, SIGINT, SIGTERM)
    │
    ├──► go b.Start(ctx)        ◄── broker goroutine
    ├──► go runDemo(ctx, svc)   ◄── demo goroutine
    │
    ├──► <-sigCh                ◄── blocks until Ctrl+C
    │
    ├──► cancel()               ◄── ctx.Done() fires everywhere
    │
    ├──► b.Shutdown(shutdownCtx) ◄── drain subscribers
    │
    ├──► wg.Wait()              ◄── wait for all goroutines
    │
    └──► log final stats, exit
```

---

## Why This Order Matters

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │   STEP                    WHY FIRST/LAST                                │
  ├──────────────────────────────────────────────────────────────────────────┤
  │   1. Config first         Everything depends on config values           │
  │   2. Context second       Passed to every component                     │
  │   3. Broker third         Service depends on broker                     │
  │   4. Service fourth       Depends on broker                             │
  │   5. Signal handler       Must be setup before blocking operations      │
  │   6. Start goroutines     Only after all components exist               │
  │   7. Block on signal      Main goroutine waits here                     │
  │   8. Cancel context       Triggers shutdown in all goroutines           │
  │   9. Shutdown broker      Drain in-flight work                          │
  │  10. Wait for goroutines  Ensure everything finished                    │
  │  11. Print stats          Last thing before exit                        │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## The Demo Function

This is a temporary placeholder — file 09 has the full demo.

```go
// Temporary demo — replace with your own application logic
func runDemo(ctx context.Context, svc service.MessageQueue) {
    // Create a topic
    if err := svc.CreateTopic(ctx, "orders"); err != nil {
        log.Printf("create topic: %v", err)
        return
    }

    // Subscribe
    sub, err := svc.Subscribe(ctx, "orders", 50)
    if err != nil {
        log.Printf("subscribe: %v", err)
        return
    }

    // Start consuming
    sub.Start(ctx, func(msg model.Message) error {
        log.Printf("[%s] received: %s", sub.ID[:8], string(msg.Payload))
        return nil
    })

    // Publish messages
    for i := 0; i < 10; i++ {
        payload := []byte(fmt.Sprintf("order #%d", i))
        _, err := svc.Publish(ctx, "orders", payload, nil)
        if err != nil {
            log.Printf("publish: %v", err)
        }
    }

    // Keep running until context cancelled
    <-ctx.Done()
}
```

---

## Next

The wiring is complete. Now let's handle **graceful shutdown** properly — the hardest part of any service. → `07-graceful-shutdown.md`
