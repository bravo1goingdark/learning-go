# 07 — Graceful Shutdown

> **Decision:** When the process receives SIGINT/SIGTERM, we must drain in-flight messages before exiting. Losing messages on shutdown is unacceptable in production.

**Reference:** `06-concurrency/14-context.md` + `06-concurrency/15-waitgroup.md`

---

## Table of Contents

1. [Why Graceful Shutdown Matters](#why-graceful-shutdown-matters) `[CORE]`
2. [The Shutdown Sequence](#the-shutdown-sequence) `[CORE]`
3. [Code: Shutdown in Broker](#code-shutdown-in-broker) `[CORE]`
4. [Code: Subscriber Drain on Shutdown](#code-subscriber-drain-on-shutdown) `[CORE]`
5. [Code: Signal Handling in main.go](#code-signal-handling-in-maingo) `[CORE]`
6. [Common Pitfalls](#common-pitfalls) `[PRODUCTION]`

---

## Why Graceful Shutdown Matters

```
  WITHOUT GRACEFUL SHUTDOWN:

  SIGINT received
       │
       └──► os.Exit(0) immediately
             • 47 messages in subscriber buffers → LOST
             • 3 goroutines still running → LEAKED
             • DLQ not flushed → LOST


  WITH GRACEFUL SHUTDOWN:

  SIGINT received
       │
       ├──► Stop accepting new publishes
       ├──► Drain subscriber buffers
       ├──► Wait for goroutines (with timeout)
       ├──► Flush DLQ
       └──► os.Exit(0)
             • 0 messages lost
             • 0 goroutines leaked
```

---

## The Shutdown Sequence

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                      GRACEFUL SHUTDOWN FLOW                              │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   ┌─────────────┐                                                        │
  │   │   SIGINT    │                                                        │
  │   │  SIGTERM    │                                                        │
  │   └──────┬──────┘                                                        │
  │          │                                                                │
  │          ▼                                                                │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  PHASE 1: Signal Reception                                       │   │
  │   │                                                                    │   │
  │   │  signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)          │   │
  │   │  <-sigCh  ◄── blocks until signal arrives                       │   │
  │   │                                                                    │   │
  │   │  Code:                                                            │   │
  │   │    sigCh := make(chan os.Signal, 1)                              │   │
  │   │    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)        │   │
  │   │    <-sigCh  // blocks here                                      │   │
  │   └──────────────────────────────────┬───────────────────────────────┘   │
  │                                      │                                    │
  │                                      ▼                                    │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  PHASE 2: Cancel Context                                         │   │
  │   │                                                                    │   │
  │   │  cancel()  ◄── ctx.Done() fires in EVERY goroutine              │   │
  │   │                                                                    │   │
  │   │  What happens:                                                    │   │
  │   │  • Broker.Start() sees ctx.Done() → stops accepting publishes   │   │
  │   │  • Subscriber.Start() sees ctx.Done() → drains remaining msgs   │   │
  │   │  • DLQ processor sees ctx.Done() → stops processing             │   │
  │   │  • HTTP server sees ctx.Done() → stops accepting connections    │   │
  │   └──────────────────────────────────┬───────────────────────────────┘   │
  │                                      │                                    │
  │                                      ▼                                    │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  PHASE 3: Drain Subscribers                                      │   │
  │   │                                                                    │   │
  │   │  for each topic:                                                  │   │
  │   │    for each subscriber:                                           │   │
  │   │      close(sub.ch)      ◄── no more messages accepted            │   │
  │   │      <-sub.done         ◄── wait for consumer to drain           │   │
  │   │                                                                    │   │
  │   │  This ensures every buffered message reaches its handler.        │   │
  │   └──────────────────────────────────┬───────────────────────────────┘   │
  │                                      │                                    │
  │                                      ▼                                    │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  PHASE 4: Wait for Goroutines                                    │   │
  │   │                                                                    │   │
  │   │  wg.Wait()  ◄── blocks until ALL goroutines call wg.Done()      │   │
  │   │                                                                    │   │
  │   │  With timeout:                                                    │   │
  │   │    done := make(chan struct{})                                    │   │
  │   │    go func() { wg.Wait(); close(done) }()                       │   │
  │   │    select {                                                       │   │
  │   │    case <-done:        // all done                               │   │
  │   │    case <-ctx.Done():  // timeout — force exit                  │   │
  │   │    }                                                              │   │
  │   └──────────────────────────────────┬───────────────────────────────┘   │
  │                                      │                                    │
  │                                      ▼                                    │
  │   ┌──────────────────────────────────────────────────────────────────┐   │
  │   │  PHASE 5: Print Stats & Exit                                     │   │
  │   │                                                                    │   │
  │   │  log.Printf("topics=%d subs=%d dlq=%d", ...)                    │   │
  │   │  os.Exit(0)                                                       │   │
  │   └──────────────────────────────────────────────────────────────────┘   │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Code: Shutdown in Broker

**What:** The broker's `Shutdown()` method marks itself closed, copies topics into a slice, then drains each topic's subscribers.

**Why copy topics into a slice first?** If we iterate the map while holding `RLock`, and `topic.close()` tries to `Lock` the topic, we risk deadlock if another goroutine holds topic `Lock` and waits for broker `RLock`. Copying breaks the lock dependency chain.

**How:** Lock broker → set `closed = true` → RLock → copy topics slice → RUnlock → drain each topic (without holding broker lock).

```go
// internal/broker/broker.go
// TOPIC 13: Context propagation
// TOPIC 14: WaitGroup
// TOPIC 9:  Defer

func (b *inMemoryBroker) Shutdown(ctx context.Context) error {
    // Phase 1: Mark as closed (reject new publishes)
    b.mu.Lock()
    b.closed = true
    b.mu.Unlock()

    log.Println("broker: stopping new publishes")

    // Phase 2: Close all topics (drains each topic's subscribers)
    // Why copy into a slice first?
    // If we iterate the map while holding RLock, and topic.close()
    // tries to Lock the topic, we risk deadlock if another goroutine
    // holds topic Lock and waits for broker RLock.
    b.mu.RLock()
    topics := make([]*topic, 0, len(b.topics))
    for _, t := range b.topics {
        topics = append(topics, t)
    }
    b.mu.RUnlock()

    // Now we can drain WITHOUT holding the broker lock
    for _, t := range topics {
        t.close() // closes subscriber channels, waits for drains
    }

    log.Println("broker: all subscribers drained")
```

---

## Code: Subscriber Drain on Shutdown

```go
// internal/broker/subscriber.go
// TOPIC 11: Channel close, range
// TOPIC 13: Context Done

func (s *Subscriber) Start(ctx context.Context, handler func(model.Message) error) {
    go func() {
        defer close(s.done) // signal broker we're done

        for {
            select {
            case <-ctx.Done():
                // Context cancelled — drain remaining messages
                s.drainRemaining(handler)
                return

            case msg, ok := <-s.ch:
                if !ok {
                    // Channel closed by broker — drain remaining
                    s.drainRemaining(handler)
                    return
                }

                if err := handler(msg); err != nil {
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

func (s *Subscriber) drainRemaining(handler func(model.Message) error) {
    count := 0
    for {
        select {
        case msg, ok := <-s.ch:
            if !ok {
                return
            }
            handler(msg) // best-effort during shutdown
            count++
        default:
            // Nothing left in buffer
            if count > 0 {
                log.Printf("subscriber %s: drained %d messages on shutdown", s.ID[:8], count)
            }
            return
        }
    }
}
```

### Why `drainRemaining` Uses `select` with `default`?

```
  for {
      select {
      case msg, ok := <-s.ch:
          if !ok { return }     // channel closed
          handler(msg)          // process it
      default:
          return                // nothing left — exit
      }
  }
```

If we used `for msg := range s.ch`, it would block forever on an empty-but-open channel. The `select` with `default` makes it non-blocking — it exits immediately when the buffer is empty.

---

## Code: Signal Handling in main.go

```go
// cmd/server/main.go
// TOPIC 13: signal.Notify with channels

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Setup signal handler
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    // ... start services ...

    // Block until signal
    log.Println("press Ctrl+C to stop")
    sig := <-sigCh
    log.Printf("received %v, shutting down...", sig)

    // Cancel context — triggers shutdown everywhere
    cancel()

    // Give components time to drain
    // Why a NEW context instead of reusing ctx?
    // ctx is already cancelled. Passing a cancelled context to Shutdown()
    // would immediately return ctx.Err() — no draining would happen.
    // We need a FRESH context with a timeout for the drain phase.
    shutdownCtx, shutdownCancel := context.WithTimeout(
        context.Background(), 10*time.Second,
    )
    defer shutdownCancel()

    // Shutdown broker
    broker.Shutdown(shutdownCtx)

    // Wait for all goroutines
    wg.Wait()
    log.Println("shutdown complete")
}
```

### Why Two Contexts?

```
  ctx (lifecycle context)          shutdownCtx (drain context)
  ┌─────────────────────┐         ┌─────────────────────┐
  │ Created at startup  │         │ Created at shutdown │
  │ Cancelled on SIGINT │         │ 10s timeout         │
  │ Propagated to all   │         │ Passed ONLY to      │
  │ goroutines          │         │ Shutdown() methods  │
  └─────────────────────┘         └─────────────────────┘

  Timeline:
  1. <-sigCh        ◄── SIGINT received
  2. cancel()       ◄── ctx.Done() fires everywhere, goroutines start draining
  3. shutdownCtx    ◄── new context with 10s budget
  4. Shutdown(shutdownCtx) ◄── broker drains within 10s
  5. wg.Wait()      ◄── wait for all goroutines
```

---

## Common Pitfalls

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │   PITFALL                         FIX                                    │
  ├──────────────────────────────────────────────────────────────────────────┤
  │   Using os.Exit() directly        Always drain before exiting          │
  │   No shutdown timeout             Add context.WithTimeout               │
  │   Forgetting signal.Notify        Signal goes to default handler        │
  │   Buffered channel for signals    make(chan, 1) prevents missed signals │
  │   Not checking ctx.Done()         Goroutine leaks                       │
  │   Closing channel from wrong side Only broker closes, not subscriber    │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Next

Shutdown is handled. Now let's add **resilience** — dead letter queue, retry logic. → `08-resilience.md`
