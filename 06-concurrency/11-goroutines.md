# 11. Goroutines — Complete Deep Dive

> **Goal:** Master goroutines from creation to production patterns. Understand the scheduler, stack growth, and common pitfalls.

> **Bridge from Topics 1-10:** Everything you've built so far has been sequential — one operation at a time. A `for` loop processes items one by one. A function call blocks until it returns. Concurrency changes this: you can start multiple operations at the same time and coordinate their results. Go makes this safe and efficient with goroutines and channels. This section starts that journey.

---
![Goroutines](../assets/10.png)
## Table of Contents

1. [What Is a Goroutine](#1-what-is-a-goroutine)
2. [Creating Goroutines](#2-creating-goroutines)
   - [What `go` Does to the Flow of Execution](#what-go-does-to-the-flow-of-execution)
   - [What Happens When You Write `go`](#what-happens-when-you-write-go)
   - [Critical Behaviors of `go`](#critical-behaviors-of-go)
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

### What `go` Does to the Flow of Execution

Without `go`, function calls are **sequential** — each call blocks until it finishes, then the next line runs:

```go
func main() {
    fmt.Println("A")
    worker(1)       // blocks until worker(1) finishes
    fmt.Println("B") // runs AFTER worker(1) returns
    worker(2)       // blocks until worker(2) finishes
    fmt.Println("C") // runs AFTER worker(2) returns
}
// Guaranteed order: A → worker1 → B → worker2 → C
```

With `go`, the function call **launches and immediately returns** — the caller moves to the next line without waiting:

```go
func main() {
    fmt.Println("A")
    go worker(1)     // launches worker(1), doesn't wait
    fmt.Println("B") // runs IMMEDIATELY — worker(1) may not have started yet
    go worker(2)     // launches worker(2), doesn't wait
    fmt.Println("C") // runs IMMEDIATELY
}
// A, B, C print fast. worker(1) and worker(2) run whenever the scheduler picks them up.
// Order of worker output is unpredictable — could be worker1→worker2 or worker2→worker1 or interleaved.
```

The key shift:

```
    ┌───────────────────────────────────┐     ┌───────────────────────────────────┐
    │          WITHOUT go               │     │           WITH go                 │
    │       (sequential/blocking)       │     │        (concurrent/fire)          │
    ├───────────────────────────────────┤     ├───────────────────────────────────┤
    │                                   │     │                                   │
    │  Line 1: fmt.Println("A")         │     │  Line 1: fmt.Println("A")         │
    │                                   │     │                                   │
    │  Line 2: worker(1)                │     │  Line 2: go worker(1)             │
    │           └── blocks ◄── waits    │     │           └── launches, returns   │
    │                                   │     │                                   │
    │  Line 3: fmt.Println("B")         │     │  Line 3: fmt.Println("B")         │
    │           └── after worker(1)     │     │           └── runs immediately    │
    │                                   │     │                                   │
    │  Line 4: worker(2)                │     │  Line 4: go worker(2)             │
    │           └── blocks ◄── waits    │     │           └── launches, returns   │
    │                                   │     │                                   │
    │  Line 5: fmt.Println("C")         │     │  Line 5: fmt.Println("C")         │
    │           └── after worker(2)     │     │           └── runs immediately    │
    │                                   │     │                                   │
    ├───────────────────────────────────┤     ├───────────────────────────────────┤
    │  Output order (guaranteed):       │     │  Output order (unpredictable):    │
    │  A → worker1 → B → worker2 → C   │     │  A → B → C                        │
    │                                   │     │  worker1,worker2 run in background│
    └───────────────────────────────────┘     └───────────────────────────────────┘
```

**Reading this diagram:**

- **Left box (WITHOUT go):** Your code runs top to bottom, one line at a time. When you call `worker(1)`, everything stops and waits until `worker(1)` finishes. Only then does line 3 (`fmt.Println("B")`) run. This is like standing in a single-file line — nobody skips ahead.
- **Right box (WITH go):** Adding `go` changes the rule. `go worker(1)` fires off the worker in the background and the very next line runs immediately. The main goroutine doesn't wait. It's like telling someone "start working on this" while you keep doing your own thing.
- **Bottom row:** Shows what the output order looks like. Without `go`, you always get `A → worker1 → B → worker2 → C`. With `go`, you get `A → B → C` fast, and the worker output can appear anywhere — you don't know when the scheduler will run them.

Timeline view:

```
  WITHOUT go:

  ──┬────────┬──────────────┬──────────────┬──────────────┬──────────────► time
    │        │              │              │              │
    A     worker(1)          B           worker(2)         C
           blocks here                 blocks here
                                        until done                  until done


  WITH go:

  ──┬────────┬──────────────┬──────────────┬──────────────┬────────────► time
    │        │              │              │              │
    A     go worker(1)       B          go worker(2)       C
           launches,                 launches,
           doesn't wait              doesn't wait

         ┌──────────────────────────────────────────────────────┐
         │  worker(1) and worker(2) run concurrently on         │
         │  separate goroutines — their output can interleave   │
         │  in any order                                        │
         └──────────────────────────────────────────────────────┘
```

**Reading this diagram:**

- **Top timeline (WITHOUT go):** Time flows left to right (the `► time` arrow). Each event happens one after another. `A` prints, then `worker(1)` runs and the program sits there waiting (the big gap), then `B` prints, then `worker(2)` runs and waits again, then `C` prints. Total time = sum of everything.
- **Bottom timeline (WITH go):** `A` prints, `go worker(1)` fires instantly (no waiting), `B` prints right away, `go worker(2)` fires instantly, `C` prints right away. The main goroutine finishes fast. Meanwhile, `worker(1)` and `worker(2)` run in the background concurrently — their execution time doesn't block the main flow at all.

Three things change when you add `go`:

1. **The caller doesn't block.** It moves to the next line immediately.
2. **No guaranteed order.** The goroutine and the caller run concurrently — output can interleave in any way.
3. **No return values.** The caller can't wait for a result because it has already moved on. Use channels (Topic 12) to send results back.

### Function Call

```go
func worker(id int) {
    fmt.Printf("Worker %d running\n", id)
}

func main() {
    go worker(1)
    go worker(2)

    time.Sleep(time.Second) // DON'T do this in production — main might exit before goroutines finish. Use sync.WaitGroup (Topic 15) instead.
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

### What Happens When You Write `go`

The `go` keyword works with **any** function or method call — not just goroutine-specific code. You can take any existing function and run it concurrently by prefixing it with `go`.

```go
// These all work — go just needs a function call
go fmt.Println("hello")           // builtin
go http.ListenAndServe(":8080", nil) // stdlib
go myStruct.Method(arg)           // method call
go func() { doWork() }()          // anonymous function
go namedFunc(arg)                 // named function
```

When the runtime encounters `go`, it does the following **immediately**:

1. **Arguments are evaluated** in the calling goroutine (eager evaluation)
2. **A new goroutine (G) is created** — gets its own stack (2 KB), program counter, and ID
3. **G is placed in a run queue** — either the current P's local queue or the global queue
4. **The calling goroutine continues** — does NOT wait for the new goroutine

```
  ┌─────────────────────────────────────┐      ┌─────────────────────────────────────┐
  │       CALLER GOROUTINE              │      │       NEW GOROUTINE (G)             │
  ├─────────────────────────────────────┤      ├─────────────────────────────────────┤
  │                                     │      │                                     │
  │  go worker(42)                      │      │                                     │
  │       │                             │      │                                     │
  │       ├── 1. args evaluated (42)    │──────│──►  receives copy of 42             │
  │       │                             │      │                                     │
  │       ├── 2. G created              │      │    sits in run queue (Runnable)     │
  │       │                             │      │                                     │
  │       ├── 3. G queued               │      │                                     │
  │       │                             │      │                                     │
  │  next line executes immediately     │      │    scheduler picks it up eventually │
  │  (does NOT wait for G)              │      │    and runs it on an M              │
  │                                     │      │                                     │
  └─────────────────────────────────────┘      └─────────────────────────────────────┘
```

**Reading this diagram:**

- **Left box (CALLER):** This is your main goroutine — the one that wrote `go worker(42)`. It does three things in order: (1) evaluates the argument `42`, (2) creates a new goroutine, (3) queues it. Then it immediately moves to the next line of code. It does NOT wait.
- **The `──────` arrow in the middle:** This shows what transfers between the two goroutines. The value `42` is copied into the new goroutine. This is why passing arguments is safe — each goroutine gets its own copy.
- **Right box (NEW GOROUTINE):** The new goroutine receives the copied `42` and sits in the run queue with state "Runnable." Eventually, the scheduler (a P) picks it up and runs it on an OS thread (an M). This delay can be microseconds or longer — you don't control when it starts.

### Critical Behaviors of `go`

**Return values are discarded.** There is no way to get a return value directly from a goroutine — use channels or shared state instead.

```go
// This compiles but the return value is lost
go func() int {
    return 42
}() // return value goes nowhere

// Use a channel to get results
ch := make(chan int, 1)
go func() {
    ch <- 42
}()
result := <-ch
```

**A panic in a goroutine crashes the entire program.** Unlike a function call where the caller can recover, a goroutine panic is unrecoverable unless that goroutine itself has a `defer/recover`.

```go
// BAD — panic crashes the whole program
go func() {
    panic("boom") // program exits
}()

// GOOD — recover inside the goroutine
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("recovered: %v", r)
        }
    }()
    panic("boom") // caught by recover above
}()
```

**The parent doesn't know when the goroutine finishes.** The calling code continues immediately. If `main()` returns, all goroutines are killed instantly — no cleanup, no deferred functions.

```go
func main() {
    go func() {
        time.Sleep(time.Hour) // Never runs — main exits first
        fmt.Println("done")
    }()
    // main returns here, program exits, goroutine is killed
}
```

---

## 3. Goroutine Internals

### G-M-P Model

```
                       GO SCHEDULER (G-M-P Model)
  ╔═══════════════════════════════════════════════════════════════════════════╗
  ║                                                                           ║
  ║   G = Goroutine  (unit of work)                                          ║
  ║   M = Machine    (OS thread that executes)                               ║
  ║   P = Processor  (logical processor, manages run queue)                  ║
  ║                                                                           ║
  ╠═══════════════════════════════════════════════════════════════════════════╣
  ║                                                                           ║
  ║   ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐                          ║
  ║   │  G1  │ │  G2  │ │  G3  │ │  G4  │ │  G5  │  ... G-n                ║
  ║   └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘                          ║
  ║      │        │        │        │        │                               ║
  ║      └────────┴───┐    └────────┴───┐                                    ║
  ║                   │                 │                                    ║
  ║                   ▼                 ▼                                    ║
  ║         ┌──────────────────┐ ┌──────────────────┐                        ║
  ║         │  LOCAL RUN QUEUE │ │  LOCAL RUN QUEUE │                        ║
  ║         │  [G1] [G2] [G3] │ │  [G4] [G5]      │                        ║
  ║         └────────┬─────────┘ └────────┬─────────┘                        ║
  ║                  │                    │                                  ║
  ║         ┌────────▼─────────┐ ┌────────▼─────────┐                        ║
  ║         │       P1         │ │       P2         │                        ║
  ║         │   (Processor)    │ │   (Processor)    │                        ║
  ║         └────────┬─────────┘ └────────┬─────────┘                        ║
  ║                  │                    │                                  ║
  ║         ┌────────▼─────────┐ ┌────────▼─────────┐                        ║
  ║         │       M1         │ │       M2         │                        ║
  ║         │   (OS Thread)    │ │   (OS Thread)    │                        ║
  ║         └──────────────────┘ └──────────────────┘                        ║
  ║                                                                           ║
  ║         ┌────────────────────────────────────────┐                        ║
  ║         │         GLOBAL RUN QUEUE               │                        ║
  ║         │  [G6] [G7] [G8] [G9] [G10] ...        │                        ║
  ║         └────────────────────────────────────────┘                        ║
  ║                                                                           ║
  ╠═══════════════════════════════════════════════════════════════════════════╣
  ║                                                                           ║
  ║   RULES:                                                                  ║
  ║   ┌─────────────────────────────────────────────────────────────────────┐ ║
  ║   │ • Each P has a LOCAL run queue (lock-free access)                  │ ║
  ║   │ • WORK STEALING: idle P steals half of another P's queue          │ ║
  ║   │ • HANDOFF: if M blocks (syscall), P moves to another M            │ ║
  ║   │ • PREEMPTION: Go 1.14+ async preemption via signals               │ ║
  ║   └─────────────────────────────────────────────────────────────────────┘ ║
  ║                                                                           ║
  ╚═══════════════════════════════════════════════════════════════════════════╝
```

**Reading this diagram — top to bottom, left to right:**

- **Top row (G1–G5):** These are goroutines — your units of work. Every `go` statement creates one of these. There can be millions.
- **Arrows going down:** Goroutines get assigned to run queues. G1, G2, G3 go to P1's local queue. G4, G5 go to P2's local queue. This assignment is done by the Go scheduler.
- **LOCAL RUN QUEUE:** Each P (processor) has its own private queue. This is fast because there's no locking — only that P reads from it. Think of it like a personal to-do list for each worker.
- **P1, P2 (Processor):** These are logical processors. By default, you get one per CPU core. A P is what actually runs goroutines. It picks the next G from its local queue and executes it.
- **M1, M2 (Machine):** These are real OS threads. P needs an M to actually run code on the CPU. If M1 blocks (e.g., a syscall), P1 can detach and grab a different M — this is the handoff rule.
- **GLOBAL RUN QUEUE:** Overflow goroutines go here. If a P's local queue is full, or if goroutines haven't been assigned yet, they sit in the global queue. Any idle P can steal from here.
- **Bottom (RULES):** Four key behaviors: (1) local queues are lock-free for speed, (2) idle P steals work from busy P (work stealing), (3) if an M blocks on a syscall, P moves to a free M (handoff), (4) since Go 1.14, long-running goroutines can be preempted (interrupted) via OS signals so they don't starve others.

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
                            ┌──────────────────┐
                            │     Created      │
                            └────────┬─────────┘
                                     │  go func()
                                     ▼
                            ┌──────────────────┐
                            │    Runnable      │  In run queue, waiting for P
                            └────────┬─────────┘
                                     │  P picks it up
                                     ▼
                            ┌──────────────────┐
                            │    Running       │  Executing on an M
                            └────┬─────────┬───┘
                                 │         │
                    blocked      │         │  done / return
                    (ch, I/O)    │         │
                                 ▼         ▼
                     ┌──────────────────┐  ┌──────────────────┐
                     │    Waiting       │  │      Dead        │
                     │   (blocked)      │  │   (finished)     │
                     └────────┬─────────┘  └──────────────────┘
                              │
                              │  unblocked (data ready, ch recv)
                              ▼
                     ┌──────────────────┐
                     │    Runnable      │  Back in run queue
                     └──────────────────┘
```

**Reading this diagram — follow the arrows:**

- **Created:** You write `go func()`. The goroutine now exists in memory with its own stack, but it hasn't started running yet. Think of it as a task you wrote on a sticky note.
- **Runnable:** The goroutine is placed in a run queue (local or global). It's ready to run and waiting for a P to pick it up. There may be many goroutines ahead of it in the queue.
- **Running:** A P has picked this goroutine and is executing it on an OS thread (M). This is the only state where your code actually runs.
- **From Running, two paths branch out:**
  - **Left arrow → Waiting:** The goroutine hit a blocking operation — waiting on a channel receive, a mutex lock, an I/O read, or `time.Sleep`. It's parked. It does NOT occupy a P while waiting, so the P can go run other goroutines.
  - **Right arrow → Dead:** The goroutine finished its function (returned). It's done forever. Memory will be garbage collected.
- **Waiting → Runnable (the loop back):** When the blocking operation completes (data arrives on the channel, I/O finishes, etc.), the goroutine goes back to Runnable. It re-enters a run queue and waits for a P to pick it up again. It does NOT resume where it left off on the same P — it could be picked up by any P.

---

## 5. Closure Gotchas

### Closures Primer

A **closure** is an anonymous function that captures variables from its surrounding scope. In Go, closures capture variables **by reference** — they hold a pointer to the outer variable, not a copy.

```go
x := 10
fn := func() {
    fmt.Println(x) // fn "closes over" x — reads the actual x, not a snapshot
}
x = 20
fn() // prints 20, not 10
```

This is powerful but dangerous with goroutines, because the captured variable may change before the goroutine runs.

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

> **`runtime`** provides access to Go's runtime system. `runtime.GOMAXPROCS(n)` sets the maximum number of OS threads that can execute Go code simultaneously (default = number of logical CPUs). `runtime.NumCPU()` returns the CPU count. `runtime.NumGoroutine()` returns the number of currently running goroutines. See: `go doc runtime.GOMAXPROCS`.

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

> **`errgroup`** (from `golang.org/x/sync/errgroup`) manages a group of goroutines. `g.Go(func() error)` starts a goroutine. `g.Wait()` blocks until all goroutines finish and returns the first non-nil error (or nil if all succeeded). It's the standard way to run multiple goroutines and collect errors. Install: `go get golang.org/x/sync`. See: `go doc golang.org/x/sync/errgroup`.

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
