# 10. Goroutines — Complete Deep Dive

> **Goal:** Master goroutines from creation to production patterns. Understand the scheduler, stack growth, and common pitfalls.

---
![Goroutines](../assets/10.png)
## Table of Contents

1. [What Is a Goroutine](#1-what-is-a-goroutine)
2. [Creating Goroutines](#2-creating-goroutines)
3. [Goroutine Internals](#3-goroutine-internals)
4. [Goroutine Lifecycle](#4-goroutine-lifecycle)
5. [Closure Gotchas](#5-closure-gotchas)
6. [Goroutine Leaks](#6-goroutine-leaks)
7. [GOMAXPROCS & Scheduler](#7-gomaxprocs--scheduler)
8. [Stack Growth](#8-stack-growth)
9. [Common Pitfalls](#9-common-pitfalls)

---

## 1. What Is a Goroutine

A goroutine is a **lightweight thread** managed by the Go runtime. Not an OS thread.

| Property | OS Thread | Goroutine |
|----------|-----------|-----------|
| Stack size | 1-8 MB (fixed) | 2 KB (grows as needed) |
| Creation cost | ~1ms | ~0.3μs |
| Switching cost | ~1-10μs | ~0.2μs |
| Max concurrent | ~10,000 | ~1,000,000+ |
| Managed by | OS kernel | Go runtime scheduler |

---

## 2. Creating Goroutines

### Basic Syntax

```go
go functionName()
go func() { /* ... */ }()
go func(arg int) { /* ... */ }(value)
```

### Function Call

```go
func worker(id int) {
    fmt.Printf("Worker %d running\n", id)
}

func main() {
    go worker(1)
    go worker(2)

    time.Sleep(time.Second) // Don't do this in production — use WaitGroup
}
```

### Anonymous Function

```go
func main() {
    go func() {
        fmt.Println("goroutine running")
    }()

    time.Sleep(time.Millisecond)
}
```

### With Arguments (Safe)

```go
func main() {
    for i := 0; i < 5; i++ {
        go func(n int) { // Pass i as argument — copies the value
            fmt.Println(n)
        }(i)
    }

    time.Sleep(time.Millisecond)
    // Output: 0, 1, 2, 3, 4 (order varies)
}
```

---

## 3. Goroutine Internals

### G-M-P Model

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                        GO SCHEDULER (G-M-P Model)                        │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                          │
  │   G = Goroutine (unit of work)                                          │
  │   M = Machine  (OS thread that executes)                                │
  │   P = Processor (logical processor, manages run queue)                  │
  │                                                                          │
  │                                                                          │
  │   ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐                    │
  │   │  G1  │  │  G2  │  │  G3  │  │  G4  │  │  G5  │   ... G-n          │
  │   └──┬───┘  └──┬───┘  └──┬───┘  └──┬───┘  └──┬───┘                    │
  │      │         │         │         │         │                          │
  │      └─────────┼─────────┘         └─────────┼─────────┐               │
  │                │                             │         │               │
  │                ▼                             ▼         │               │
  │   ┌──────────────────────┐  ┌──────────────────────┐  │               │
  │   │   LOCAL RUN QUEUE    │  │   LOCAL RUN QUEUE    │  │               │
  │   │   [G1] [G2] [G3]    │  │   [G4] [G5]         │  │               │
  │   └──────────┬───────────┘  └──────────┬───────────┘  │               │
  │              │                          │              │               │
  │   ┌──────────▼──────────┐  ┌──────────▼──────────┐   │               │
  │   │        P1           │  │        P2           │   │               │
  │   │   (Processor 1)     │  │   (Processor 2)     │   │               │
  │   └──────────┬──────────┘  └──────────┬──────────┘   │               │
  │              │                          │              │               │
  │   ┌──────────▼──────────┐  ┌──────────▼──────────┐   │               │
  │   │        M1           │  │        M2           │   │               │
  │   │    (OS Thread)      │  │    (OS Thread)      │   │               │
  │   └─────────────────────┘  └─────────────────────┘   │               │
  │                                                       │               │
  │   ┌─────────────────────────────────────────────────┐ │               │
  │   │              GLOBAL RUN QUEUE                    │◄┘               │
  │   │   [G6] [G7] [G8] [G9] ...                      │                 │
  │   └─────────────────────────────────────────────────┘                 │
  │                                                                          │
  │   ┌──────────────────────────────────────────────────────────────────┐  │
  │   │  RULES:                                                          │  │
  │   │  • Each P has a LOCAL run queue (lock-free access)              │  │
  │   │  • WORK STEALING: idle P steals half of another P's queue      │  │
  │   │  • HANDOFF: if M blocks (syscall), P moves to another M        │  │
  │   │  • PREEMPTION: Go 1.14+ async preemption via signals           │  │
  │   └──────────────────────────────────────────────────────────────────┘  │
  │                                                                          │
  └──────────────────────────────────────────────────────────────────────────┘
```

| Component | Role |
|-----------|------|
| **G** | Goroutine — the unit of work |
| **M** | Machine — OS thread that executes code |
| **P** | Processor — manages local run queue, has resources to run G |

### Key Rules

- Each P has a **local run queue** (lock-free access)
- Work stealing: idle P steals from other P's queues
- **Handoff**: if M blocks (syscall), P is handed to another M
- **Preemption**: Go 1.14+ supports async preemption via signals

---

## 4. Goroutine Lifecycle

```
                            ┌──────────────┐
                            │   Created    │
                            └──────┬───────┘
                                   │  go func()
                                   ▼
                            ┌──────────────┐
                            │   Runnable   │  ◄── In run queue, waiting for P
                            └──────┬───────┘
                                   │  P picks it up
                                   ▼
                            ┌──────────────┐
                            │   Running    │  ◄── Executing on an M
                            └───┬──────┬───┘
                                │      │
                   blocked      │      │  done / return
                   (ch, I/O)    │      │
                                ▼      ▼
                    ┌──────────────┐  ┌──────────────┐
                    │   Waiting    │  │     Dead     │
                    │  (blocked)   │  │  (finished)  │
                    └──────┬───────┘  └──────────────┘
                           │
                           │  unblocked (data ready, ch recv)
                           ▼
                    ┌──────────────┐
                    │   Runnable   │  ◄── Back in run queue
                    └──────────────┘
```

---

## 5. Closure Gotchas

### The Classic Bug

```go
func main() {
    for i := 0; i < 5; i++ {
        go func() {
            fmt.Println(i) // BUG: all goroutines may print 5
        }()
    }
    time.Sleep(time.Millisecond)
}
```

**Why?** The closure captures `i` **by reference**. By the time goroutines run, the loop may have finished and `i == 5`.

### Fix 1: Pass as Argument

```go
for i := 0; i < 5; i++ {
    go func(n int) {
        fmt.Println(n) // Each goroutine gets its own copy
    }(i)
}
```

### Fix 2: Shadow Variable

```go
for i := 0; i < 5; i++ {
    i := i // New variable in loop scope
    go func() {
        fmt.Println(i) // Captures the shadowed copy
    }()
}
```

### Fix 3: Range Over Slice

```go
values := []int{0, 1, 2, 3, 4}
for _, v := range values {
    go func(n int) {
        fmt.Println(n)
    }(v)
}
```

---

## 6. Goroutine Leaks

A goroutine leak happens when a goroutine **never exits** — it stays alive forever, consuming memory.

### Common Causes

```go
// Leak 1: Nobody reads from channel
func leak1() {
    ch := make(chan int)
    go func() {
        ch <- 1 // Blocks forever if nobody reads
    }()
    // Forgot to read from ch
}

// Leak 2: Waiting on channel that never sends
func leak2() {
    ch := make(chan int)
    go func() {
        val := <-ch // Blocks forever if nobody sends
        fmt.Println(val)
    }()
    // Forgot to send to ch
}

// Leak 3: Infinite loop without exit condition
func leak3() {
    go func() {
        for {
            // Runs forever, never exits
        }
    }()
}
```

### Prevention

```go
// Always use context for cancellation
func safe(ctx context.Context) {
    ch := make(chan int, 1)

    go func() {
        select {
        case ch <- 1:
        case <-ctx.Done():
            return // Goroutine exits on cancellation
        }
    }()
}
```

---

## 7. GOMAXPROCS & Scheduler

### GOMAXPROCS

```go
import "runtime"

func main() {
    // Default: number of logical CPUs
    fmt.Println(runtime.GOMAXPROCS(0)) // Print current value

    // Set to 4 (only useful for specific benchmarks)
    runtime.GOMAXPROCS(4)

    // Check CPU count
    fmt.Println(runtime.NumCPU())
}
```

| Call | Effect |
|------|--------|
| `GOMAXPROCS(0)` | Returns current value, no change |
| `GOMAXPROCS(n)` | Sets max P count to n |
| `NumCPU()` | Returns logical CPU count |

### Scheduler Hints

```go
// Yield current goroutine's time slice
runtime.Gosched()

// Suggest garbage collection
runtime.GC()

// Get current goroutine ID (debugging only)
// Not officially supported — ID may change
```

---

## 8. Stack Growth

Goroutines start with a **2 KB stack** that grows dynamically.

```
Initial: 2 KB
         ↓ (function call depth increases)
         4 KB
         ↓
         8 KB
         ↓
         ... grows up to 1 GB (default max)
```

### Stack vs Heap

```go
// Stack: local variables, function params
func example() int {
    x := 42 // On stack (if it doesn't escape)
    return x
}

// Heap: escapes via pointer return
func escape() *int {
    x := 42 // Escapes to heap — compiler decides
    return &x
}
```

Check escape analysis:

```bash
go build -gcflags="-m" main.go
```

---

## 9. Common Pitfalls

| Pitfall | Problem | Fix |
|---------|---------|-----|
| `time.Sleep` to wait | Race conditions, flaky tests | Use `sync.WaitGroup` or channels |
| Closure captures loop var | All goroutines see final value | Pass as argument or shadow |
| No exit condition | Goroutine leak | Use `context.Context` or `done` channel |
| Blocking in goroutine | Delays other work | Use non-blocking patterns, `select` |
| Calling `os.Exit` | Deferred functions don't run | Return errors instead |
| Assuming order | Goroutines run in any order | Use synchronization primitives |
| Too many goroutines | OOM, scheduler thrashing | Use worker pools with bounded concurrency |

### Safe Pattern: Always Have an Exit

```go
func safeWorker(ctx context.Context, jobs <-chan int) {
    for {
        select {
        case <-ctx.Done():
            return // Clean exit
        case job, ok := <-jobs:
            if !ok {
                return // Channel closed
            }
            process(job)
        }
    }
}
```

---

## 10. Production Best Practices

### 1. Always Use Context for Cancellation

```go
// BAD — no way to stop the goroutine
func processBad(data []Data) {
    for _, d := range data {
        go func(d Data) {
            heavyProcessing(d)
        }(d)
    }
}

// GOOD — graceful shutdown via context
func processGood(ctx context.Context, data []Data) error {
    var wg sync.WaitGroup
    errCh := make(chan error, len(data))

    for _, d := range data {
        wg.Add(1)
        go func(d Data) {
            defer wg.Done()
            if err := heavyProcessing(ctx, d); err != nil {
                select {
                case errCh <- err:
                default:
                }
            }
        }(d)
    }

    // Wait for completion or cancellation
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        close(errCh)
        return nil
    case <-ctx.Done():
        return ctx.Err()
    case err := <-errCh:
        return err
    }
}
```

### 2. Track Goroutine Count in Production

```go
import "runtime/debug"

func init() {
    // Dump goroutine stack trace if too many goroutines
    go func() {
        for {
            time.Sleep(10 * time.Second)
            if n := runtime.NumGoroutine(); n > 10000 {
                log.Printf("WARNING: %d goroutines running\n", n)
                debug.PrintStack()
            }
        }
    }()
}
```

### 3. Graceful Shutdown Pattern

```go
type Server struct {
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

func NewServer() *Server {
    ctx, cancel := context.WithCancel(context.Background())
    return &Server{ctx: ctx, cancel: cancel}
}

func (s *Server) Start(handler func(ctx context.Context)) {
    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        handler(s.ctx)
    }()
}

func (s *Server) Stop() {
    s.cancel()       // Cancel context
    s.wg.Wait()      // Wait for goroutines to finish
}

func (s *Server) AddGoroutine(fn func()) {
    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        fn()
    }()
}
```

### 4. Structured Logging for Goroutines

```go
import "github.com/google/uuid"

func withGoroutineID(ctx context.Context) context.Context {
    return context.WithValue(ctx, "goroutineID", uuid.New().String())
}

// In your worker
func worker(ctx context.Context, jobs <-chan Job) {
    gid := ctx.Value("goroutineID")
    log := log.With().Str("goroutine", gid.(string)).Logger()

    for {
        select {
        case <-ctx.Done():
            log.Info("shutting down")
            return
        case job := <-jobs:
            log.Debug().Int("job", job.ID).Msg("processing")
            // process...
        }
    }
}
```

---

## 11. Debugging Goroutines

### Using runtime/pprof

```go
import (
    "runtime/pprof"
    "net/http"
)

func main() {
    // Serve pprof at /debug/pprof/
    go func() {
        http.ListenAndServe("localhost:6060", nil)
    }()

    // In your code, trigger a dump:
    pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
}
```

### Using trace

```go
func main() {
    f, _ := os.Create("trace.out")
    defer f.Close()
    trace.Start(f)
    defer trace.Stop()

    // Your code here
}
```

Run: `go tool trace trace.out`

### Using GODEBUG

```bash
# Show scheduler trace events
GODEBUG=schedtrace=1000 ./myapp

# Show detailed scheduler state
GODEBUG=scheddetail=1 ./myapp
```

---

## 12. Performance Considerations

### When to Use Goroutines

| Use Case | Recommendation |
|----------|---------------|
| I/O-bound (HTTP, DB, files) | Thousands of goroutines — I/O wait is the bottleneck |
| CPU-bound (computation) | Number of goroutines = NumCPU — more causes context switching |
| Mixed I/O + CPU | Use worker pool with N goroutines |

### GOMAXPROCS Best Practices

```go
// Default is usually optimal
runtime.GOMAXPROCS(0) // Returns current value

// Only change for specific benchmarks
// For CPU-bound work:
runtime.GOMAXPROCS(runtime.NumCPU())

// For I/O-bound, leave as default
```

### Memory for Goroutines

```
Default stack: 2 KB
Max stack: 1 GB (can grow dynamically)

1 million goroutines ≈ 2 GB base + stack growth
```

---

## 13. Common Production Patterns

### Pipeline with Backpressure

```go
func processWithBackpressure(ctx context.Context, jobs <-chan Job) <-chan Result {
    out := make(chan Result, 10) // Buffer provides backpressure

    go func() {
        defer close(out)
        for {
            select {
            case <-ctx.Done():
                return
            case job, ok := <-jobs:
                if !ok {
                    return
                }
                result, err := process(job)
                select {
                case out <- Result{Value: result, Err: err}:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()
    return out
}
```

### ErrGroup for Structured Error Handling

```go
import "golang.org/x/sync/errgroup"

func processFiles(files []string) error {
    g := errgroup.WithContext(context.Background())

    for _, file := range files {
        file := file // capture loop variable
        g.Go(func() error {
            return processFile(file)
        })
    }

    return g.Wait() // Returns first error or nil
}
```

### Semaphore for Rate Limiting

```go
func boundedProcess(ctx context.Context, items []Item, limit int) {
    sem := make(chan struct{}, limit)

    var wg sync.WaitGroup
    for _, item := range items {
        wg.Add(1)
        go func(it Item) {
            defer wg.Done()
            sem <- struct{}{}        // Acquire
            defer func() { <-sem }() // Release

            process(it)
        }(item)
    }
    wg.Wait()
}
```
