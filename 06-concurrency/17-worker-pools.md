# 17. Worker Pools — Complete Deep Dive

> **Goal:** Master worker pools — bounded concurrency that processes jobs efficiently without spawning unlimited goroutines.
>
> **How this connects:** You've learned goroutines (11), channels (12), select (13), context (14), WaitGroup (15), and mutex (16). Worker pools combine all of these: goroutines as workers, channels as job queues, context for cancellation, and WaitGroup for completion tracking. This is the first "composite pattern" — a real building block you'll use in production.

---
![Worker Pools](../assets/16.png)

## Table of Contents

1. [What Is a Worker Pool](#1-what-is-a-worker-pool)
2. [Basic Worker Pool](#2-basic-worker-pool)
3. [Worker Pool with Results](#3-worker-pool-with-results)
4. [Context-Aware Worker Pool](#4-context-aware-worker-pool)
5. [Dynamic Worker Pool](#5-dynamic-worker-pool)
6. [Generic Worker Pool](#6-generic-worker-pool)
7. [Rate-Limited Worker Pool](#7-rate-limited-worker-pool)
8. [Production Patterns](#8-production-patterns)
9. [Common Pitfalls](#9-common-pitfalls)

---

## 1. What Is a Worker Pool [CORE]

A fixed number of goroutines (workers) pull jobs from a shared channel and process them concurrently.

```
                         ┌──────────────────┐
                         │    Job Source     │
                         │  (producer loop)  │
                         └────────┬─────────┘
                                  │
                                  ▼
                         ┌──────────────────┐
                         │   jobs channel   │
                         │  (buffered chan)  │
                         └───┬────┬────┬────┘
                             │    │    │
                ┌────────────┘    │    └────────────┐
                │                 │                  │
                ▼                 ▼                  ▼
         ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
         │   Worker 1   │ │   Worker 2   │ │   Worker 3   │
         │  (go func)   │ │  (go func)   │ │  (go func)   │
         └──────┬───────┘ └──────┬───────┘ └──────┬───────┘
                │                │                  │
                └────────────────┼──────────────────┘
                                 │
                                 ▼
                        ┌──────────────────┐
                        │ results channel  │
                        │  (buffered chan)  │
                        └────────┬─────────┘
                                 │
                                 ▼
                        ┌──────────────────┐
                        │    Collector     │
                        │  (range results) │
                        └──────────────────┘
```

### Why Worker Pools?

| Without Pool | With Pool |
|-------------|-----------|
| 10,000 goroutines | 10 workers |
| ~20 MB stack memory | ~20 KB stack memory |
| Scheduler thrashing | Predictable load |
| OOM risk | Bounded resource usage |

### Visual: Worker Pool States

```
  STATE 1: IDLE (no jobs)
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                           │
  │  Jobs Channel (empty)          Workers (waiting)                         │
  │  ┌──────────────────┐          ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐      │
  │  │                  │          │  W1  │ │  W2  │ │  W3  │ │  W4  │      │
  │  │    EMPTY         │          │  -   │ │  -   │ │  -   │ │  -   │      │
  │  │                  │          └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘      │
  │  └──────────────────┘             │        │        │        │          │
  │                                   ▼        ▼        ▼        ▼          │
  │                           [blocked on <-jobs channel receive]            │
  └──────────────────────────────────────────────────────────────────────────┘

  STATE 2: ACTIVE (processing jobs)
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                           │
  │  Jobs Channel (5 jobs)         Workers (processing)                      │
  │  ┌──────────────────┐          ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐      │
  │  │ [j1][j2][j3]    │          │  W1  │ │  W2  │ │  W3  │ │  W4  │      │
  │  │ [j4][j5]        │          │  j1  │ │  j2  │ │  j3  │ │  j4  │      │
  │  └──────────────────┘          └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘      │
  │                                   │        │        │        │          │
  │                                   ▼        ▼        ▼        ▼          │
  │                            [processing in parallel]                      │
  └──────────────────────────────────────────────────────────────────────────┘

  STATE 3: DRAINING (channel closed, finishing up)
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                           │
  │  Jobs Channel (empty)          Workers (finishing)                       │
  │  ┌──────────────────┐          ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐      │
  │  │                  │          │  W1  │ │  W2  │ │  W3  │ │  W4  │      │
  │  │    CLOSED        │          │  j5  │ │  -   │ │  -   │ │  -   │      │
  │  │                  │          └──┬───┘ └──────┘ └──────┘ └──────┘      │
  │  └──────────────────┘             │                                      │
  │                                   ▼                                      │
  │                            [W1 finishes j5, then exits]                  │
  │                            [all workers exit → done]                      │
  └──────────────────────────────────────────────────────────────────────────┘
```

### Visual: Bounded vs Unbounded Concurrency

```
  UNBOUNDED (no worker pool) — DANGEROUS:
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                           │
  │   Job 1 ──► ┌──────────┐                                                  │
  │   Job 2 ──► │Goroutine │   Each job spawns its own goroutine             │
  │   Job 3 ──► │  Pool    │   1000 jobs = 1000 goroutines                   │
  │   Job 4 ──► │(unbound) │   Memory: ~2KB stack × 1000 = ~2GB!            │
  │   ...    ──► └──────────┘                                                 │
  │   Job 1000                                                                   │
  │              ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┐          │
  │              │  G1  │  G2  │  G3  │  G4  │  G5  │ ...  │G1000│          │
  │              └──────┴──────┴──────┴──────┴──────┴──────┴──────┘          │
  │                                                                           │
  │   ✗ Scheduler overwhelmed — context switching, thrashing, OOM            │
  └──────────────────────────────────────────────────────────────────────────┘

  BOUNDED (with worker pool) — SAFE:
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                                                                           │
  │   Job 1 ──┐                                                              │
  │   Job 2 ──┤                                                              │
  │   Job 3 ──┼──►  ┌──────────────────────────────────────────┐            │
  │   Job 4 ──┤     │  Jobs Channel (buffered queue)           │            │
  │   ...     │     │  [j1][j2][j3][j4][j5]...[j1000]          │            │
  │   Job1000─┘     └──────────────────┬───────────────────────┘            │
  │                                   │                                       │
  │                    ┌──────────────┼──────────────┐                       │
  │                    ▼              ▼              ▼                        │
  │              ┌──────────┐  ┌──────────┐  ┌──────────┐                    │
  │              │ Worker 1 │  │ Worker 2 │  │ ... W10  │  ◄── Only 10     │
  │              │   j1     │  │   j2     │  │   j10    │      goroutines  │
  │              └──────────┘  └──────────┘  └──────────┘                    │
  │                                                                           │
  │   ✓ Bounded memory: ~2KB × 10 workers = ~20KB                           │
  │   ✓ Predictable concurrency, no scheduler thrashing                      │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Basic Worker Pool [CORE]

```go
func worker(id int, jobs <-chan int, results chan<- int) {
    for j := range jobs {
        fmt.Printf("worker %d processing job %d\n", id, j)
        time.Sleep(time.Millisecond * 100) // Simulate work
        results <- j * 2
    }
}

func main() {
    const numJobs = 100
    const numWorkers = 5

    jobs := make(chan int, numJobs)
    results := make(chan int, numJobs)

    // Start workers
    for w := 1; w <= numWorkers; w++ {
        go worker(w, jobs, results)
    }

    // Send jobs
    for j := 1; j <= numJobs; j++ {
        jobs <- j
    }
    close(jobs) // Signal workers: no more jobs

    // Collect results
    for i := 1; i <= numJobs; i++ {
        fmt.Println("result:", <-results)
    }
}
```

### Key Points

- `close(jobs)` tells workers to exit their `range` loop
- Results channel must be buffered or collected concurrently
- Workers run until the jobs channel is closed

---

## 3. Worker Pool with Results [CORE]

### Collecting with WaitGroup

```go
type Result struct {
    JobID int
    Value int
    Err   error
}

func worker(ctx context.Context, id int, jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
    defer wg.Done()

    for {
        select {
        case <-ctx.Done():
            return
        case job, ok := <-jobs:
            if !ok {
                return
            }
            val, err := process(ctx, job)
            results <- Result{JobID: job.ID, Value: val, Err: err}
        }
    }
}

func runPool(ctx context.Context, jobs []Job, numWorkers int) ([]Result, error) {
    jobCh := make(chan Job, len(jobs))
    resCh := make(chan Result, len(jobs))

    var wg sync.WaitGroup
    wg.Add(numWorkers)

    // Start workers
    for i := 0; i < numWorkers; i++ {
        go worker(ctx, i, jobCh, resCh, &wg)
    }

    // Send jobs
    for _, j := range jobs {
        jobCh <- j
    }
    close(jobCh)

    // Close results channel when all workers done
    go func() {
        wg.Wait()
        close(resCh)
    }()

    // Collect results
    var results []Result
    for r := range resCh {
        if r.Err != nil {
            cancel() // Cancel remaining workers on first error
            return nil, r.Err
        }
        results = append(results, r)
    }

    return results, nil
}
```

---

## 4. Context-Aware Worker Pool [CORE]

Workers respect cancellation — clean shutdown on timeout or signal.

```go
func worker(ctx context.Context, id int, jobs <-chan Job) {
    for {
        select {
        case <-ctx.Done():
            log.Printf("worker %d: shutting down: %v", id, ctx.Err())
            return
        case job, ok := <-jobs:
            if !ok {
                log.Printf("worker %d: jobs channel closed", id)
                return
            }
            if err := job.Execute(ctx); err != nil {
                log.Printf("worker %d: job %v failed: %v", id, job, err)
            }
        }
    }
}

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Graceful shutdown on SIGINT
    go func() {
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, os.Interrupt)
        <-sigCh
        log.Println("shutting down...")
        cancel()
    }()

    jobs := make(chan Job, 100)
    const numWorkers = 10

    for i := 0; i < numWorkers; i++ {
        go worker(ctx, i, jobs)
    }

    // Feed jobs
    feedJobs(ctx, jobs)
}
```

---

## 5. Dynamic Worker Pool [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

Scale workers up/down based on load.

```go
type DynamicPool struct {
    jobs    chan Job
    workers int
    mu      sync.Mutex
    ctx     context.Context
    cancel  context.CancelFunc
}

func NewDynamicPool(bufferSize int) *DynamicPool {
    ctx, cancel := context.WithCancel(context.Background())
    return &DynamicPool{
        jobs:    make(chan Job, bufferSize),
        ctx:     ctx,
        cancel:  cancel,
    }
}

func (p *DynamicPool) Scale(target int) {
    p.mu.Lock()
    defer p.mu.Unlock()

    for p.workers < target {
        p.workers++
        go p.runWorker(p.workers)
    }
    // Note: shrinking requires tracking worker goroutines
}

func (p *DynamicPool) runWorker(id int) {
    for {
        select {
        case <-p.ctx.Done():
            return
        case job := <-p.jobs:
            job.Execute(p.ctx)
        }
    }
}

func (p *DynamicPool) Submit(job Job) {
    select {
    case p.jobs <- job:
    case <-p.ctx.Done():
    }
}

func (p *DynamicPool) Stop() {
    p.cancel()
}
```

---

## 6. Generic Worker Pool [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16. (Go 1.18+)

```go
type Job[T any, R any] struct {
    Input  T
    Process func(T) (R, error)
}

type Result[R any] struct {
    Value R
    Err   error
}

func Pool[T any, R any](ctx context.Context, jobs []Job[T, R], workers int) []Result[R] {
    jobCh := make(chan Job[T, R], len(jobs))
    resCh := make(chan Result[R], len(jobs))

    var wg sync.WaitGroup
    wg.Add(workers)

    for i := 0; i < workers; i++ {
        go func() {
            defer wg.Done()
            for {
                select {
                case <-ctx.Done():
                    return
                case job, ok := <-jobCh:
                    if !ok {
                        return
                    }
                    val, err := job.Process(job.Input)
                    resCh <- Result[R]{Value: val, Err: err}
                }
            }
        }()
    }

    for _, j := range jobs {
        jobCh <- j
    }
    close(jobCh)

    go func() {
        wg.Wait()
        close(resCh)
    }()

    var results []Result[R]
    for r := range resCh {
        results = append(results, r)
    }
    return results
}
```

### Usage

```go
jobs := []Job[string, int]{
    {Input: "hello", Process: func(s string) (int, error) { return len(s), nil }},
    {Input: "world", Process: func(s string) (int, error) { return len(s), nil }},
    {Input: "go",    Process: func(s string) (int, error) { return len(s), nil }},
}

results := Pool(context.Background(), jobs, 2)
for _, r := range results {
    fmt.Println(r.Value) // 5, 5, 2
}
```

---

## 7. Rate-Limited Worker Pool [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

Combine worker pool with rate limiting (e.g., API calls).

```go
func rateLimitedWorker(ctx context.Context, id int, jobs <-chan Job, limiter <-chan time.Time) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-limiter: // Wait for rate limit token
            select {
            case <-ctx.Done():
                return
            case job, ok := <-jobs:
                if !ok {
                    return
                }
                job.Execute(ctx)
            }
        }
    }
}

func main() {
    const rate = 10 // 10 requests per second
    limiter := time.Tick(time.Second / rate)

    jobs := make(chan Job, 100)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    for i := 0; i < 5; i++ {
        go rateLimitedWorker(ctx, i, jobs, limiter)
    }
}
```

---

## 8. Production Patterns [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

### Worker Pool with Timeout per Job

```go
func worker(ctx context.Context, id int, jobs <-chan Job) {
    for {
        select {
        case <-ctx.Done():
            return
        case job := <-jobs:
            jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
            err := job.Execute(jobCtx)
            cancel()

            if err != nil {
                log.Printf("worker %d: job failed: %v", id, err)
            }
        }
    }
}
```

### Worker Pool with Metrics

```go
type PoolMetrics struct {
    Processed atomic.Int64
    Failed    atomic.Int64
    Duration  atomic.Int64 // nanoseconds
}

func worker(ctx context.Context, jobs <-chan Job, metrics *PoolMetrics) {
    for {
        select {
        case <-ctx.Done():
            return
        case job := <-jobs:
            start := time.Now()
            err := job.Execute(ctx)
            elapsed := time.Since(start)

            metrics.Duration.Add(elapsed.Nanoseconds())
            if err != nil {
                metrics.Failed.Add(1)
            } else {
                metrics.Processed.Add(1)
            }
        }
    }
}
```

### Queue-Based Worker Pool with Separate Completion

```go
type WorkerPool struct {
    jobs       chan Job
    results    chan Result
    wg         sync.WaitGroup
    ctx        context.Context
    cancel     context.CancelFunc
    metrics    *PoolMetrics
    stopped    atomic.Bool
}

type PoolMetrics struct {
    Submitted  atomic.Int64
    Completed atomic.Int64
    Failed    atomic.Int64
    InQueue   atomic.Int64
}

func NewWorkerPool(workers int, queueSize int) *WorkerPool {
    ctx, cancel := context.WithCancel(context.Background())

    return &WorkerPool{
        jobs:    make(chan Job, queueSize),
        results: make(chan Result, queueSize),
        ctx:     ctx,
        cancel:  cancel,
        metrics: &PoolMetrics{},
    }
}

func (p *WorkerPool) Start(workers int) {
    for i := 0; i < workers; i++ {
        p.wg.Add(1)
        go p.worker(i)
    }
}

func (p *WorkerPool) worker(id int) {
    defer p.wg.Done()

    for {
        select {
        case <-p.ctx.Done():
            // Drain remaining jobs
            for {
                select {
                case job := <-p.jobs:
                    p.metrics.Failed.Add(1)
                default:
                    return
                }
            }
        case job, ok := <-p.jobs:
            if !ok {
                return
            }

            p.metrics.InQueue.Add(-1)

            start := time.Now()
            result, err := job.Execute(p.ctx)
            elapsed := time.Since(start)

            if err != nil {
                p.metrics.Failed.Add(1)
            } else {
                p.metrics.Completed.Add(1)
            }

            select {
            case p.results <- Result{
                Value:     result,
                Err:       err,
                Latency:   elapsed,
                Completed: time.Now(),
            }:
            case <-p.ctx.Done():
                return
            }
        }
    }
}

func (p *WorkerPool) Submit(job Job) error {
    if p.stopped.Load() {
        return errors.New("pool stopped")
    }

    p.metrics.Submitted.Add(1)
    p.metrics.InQueue.Add(1)

    select {
    case p.jobs <- job:
        return nil
    case <-p.ctx.Done():
        return p.ctx.Err()
    }
}

func (p *WorkerPool) Results() <-chan Result {
    return p.results
}

func (p *WorkerPool) Stop() {
    p.stopped.Store(true)
    p.cancel()
    close(p.jobs)
    p.wg.Wait()
    close(p.results)
}

func (p *WorkerPool) Metrics() PoolMetrics {
    return PoolMetrics{
        Submitted:  p.metrics.Submitted.Load(),
        Completed: p.metrics.Completed.Load(),
        Failed:    p.metrics.Failed.Load(),
        InQueue:   p.metrics.InQueue.Load(),
    }
}
```

### Resizable Worker Pool

```go
type ResizablePool struct {
    jobs       chan Job
    workers    int
    mu         sync.RWMutex
    wg         sync.WaitGroup
    ctx        context.Context
    cancel     context.CancelFunc
}

func NewResizablePool(queueSize int) *ResizablePool {
    ctx, cancel := context.WithCancel(context.Background())
    return &ResizablePool{
        jobs:   make(chan Job, queueSize),
        ctx:    ctx,
        cancel: cancel,
    }
}

func (p *ResizablePool) Scale(n int) {
    p.mu.Lock()
    defer p.mu.Unlock()

    current := p.workers
    diff := n - current

    if diff > 0 {
        // Add workers
        for i := 0; i < diff; i++ {
            p.wg.Add(1)
            go p.worker(current + i)
        }
        p.workers = n
    } else if diff < 0 {
        // Remove workers (not implemented - requires extra tracking)
        // For now, just update the count
        p.workers = n
    }
}

func (p *ResizablePool) worker(id int) {
    defer p.wg.Done()

    for {
        select {
        case <-p.ctx.Done():
            return
        case job, ok := <-p.jobs:
            if !ok {
                return
            }
            job.Execute(p.ctx)
        }
    }
}

func (p *ResizablePool) Submit(job Job) error {
    select {
    case p.jobs <- job:
        return nil
    case <-p.ctx.Done():
        return p.ctx.Err()
    }
}

func (p *ResizablePool) Stop() {
    p.cancel()
    close(p.jobs)
    p.wg.Wait()
}
```

### Auto-Scaling Worker Pool (Kubernetes-like)

```go
type AutoScalePool struct {
    jobs         chan Job
    minWorkers   int
    maxWorkers   int
    idleTimeout  time.Duration
    currentLoad  atomic.Int64
    ctx          context.Context
    cancel       context.CancelFunc
    wg           sync.WaitGroup
    mu           sync.Mutex
}

func NewAutoScalePool(min, max int, idleTimeout time.Duration) *AutoScalePool {
    ctx, cancel := context.WithCancel(context.Background())
    p := &AutoScalePool{
        jobs:        make(chan Job, 100),
        minWorkers:  min,
        maxWorkers:  max,
        idleTimeout: idleTimeout,
        ctx:         ctx,
        cancel:      cancel,
    }

    // Start with minimum workers
    for i := 0; i < min; i++ {
        p.addWorker()
    }

    // Start scaling monitor
    go p.monitor()

    return p
}

func (p *AutoScalePool) addWorker() {
    p.wg.Add(1)
    go func() {
        defer p.wg.Done()
        p.runWorker()
    }()
}

func (p *AutoScalePool) runWorker() {
    idleTimer := time.NewTimer(p.idleTimeout)
    defer idleTimer.Stop()

    for {
        select {
        case <-p.ctx.Done():
            return
        case job := <-p.jobs:
            p.currentLoad.Add(1)
            job.Execute(p.ctx)
            p.currentLoad.Add(-1)
            idleTimer.Reset(p.idleTimeout)
        case <-idleTimer.C:
            // Check if we can scale down
            p.mu.Lock()
            if p.currentLoad.Load() == 0 {
                p.mu.Unlock()
                return // Exit idle worker
            }
            idleTimer.Reset(p.idleTimeout)
            p.mu.Unlock()
        }
    }
}

func (p *AutoScalePool) monitor() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-p.ctx.Done():
            return
        case <-ticker.C:
            load := p.currentLoad.Load()
            p.mu.Lock()
            workers := p.maxWorkers // placeholder - track this
            p.mu.Unlock()

            // Simple scaling: add worker if load > 70%
            if float64(load) > float64(workers)*0.7 && workers < p.maxWorkers {
                p.addWorker()
            }
        }
    }
}
```

---

## 9. Common Pitfalls [CORE]

| Pitfall | Problem | Fix |
|---------|---------|-----|
| Not closing jobs channel | Workers block forever | Close when done sending |
| Results channel too small | Workers block on send | Buffer size >= job count, or collect concurrently |
| No context | Can't cancel workers | Pass `context.Context` |
| Too many workers | Defeats purpose | Match to CPU or I/O capacity |
| Panicking worker | Takes down pool | Recover in worker |
| Sending after close | Panic | Track senders with WaitGroup |

### Panic-Safe Worker

```go
func safeWorker(ctx context.Context, id int, jobs <-chan Job) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("worker %d panic: %v", id, r)
        }
    }()

    for {
        select {
        case <-ctx.Done():
            return
        case job := <-jobs:
            job.Execute(ctx)
        }
    }
}
```

---

## 10. Worker Pool Testing [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

```go
func TestWorkerPool(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    jobs := make(chan int, 100)
    results := make(chan int, 100)

    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for {
                select {
                case <-ctx.Done():
                    return
                case n, ok := <-jobs:
                    if !ok {
                        return
                    }
                    results <- n * 2
                }
            }
        }(i)
    }

    // Submit jobs
    for i := 1; i <= 10; i++ {
        jobs <- i
    }
    close(jobs)

    // Collect results
    var collected []int
    for r := range results {
        collected = append(collected, r)
        if len(collected) == 10 {
            break
        }
    }

    wg.Wait()

    // Verify
    if len(collected) != 10 {
        t.Errorf("expected 10 results, got %d", len(collected))
    }
}
```

---

## 11. Monitoring Worker Pools [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

> **`expvar`** is a stdlib package that publishes named variables (integers, floats, strings, maps) via HTTP at `/debug/vars` as JSON. It's useful for runtime monitoring — you can see live metrics by hitting that endpoint. See: `go doc expvar`.

```go
import "expvar"

var (
    poolSubmitted = expvar.NewInt("worker_pool_submitted")
    poolCompleted = expvar.NewInt("worker_pool_completed")
    poolFailed    = expvar.NewInt("worker_pool_failed")
    poolInQueue   = expvar.NewInt("worker_pool_in_queue")
)

func init() {
    // Register with /debug/vars
    expvar.Publish("worker_pool", expvar.Func(func() interface{} {
        return map[string]interface{}{
            "submitted": poolSubmitted.Value(),
            "completed": poolCompleted.Value(),
            "failed":    poolFailed.Value(),
            "in_queue":  poolInQueue.Value(),
        }
    }))
}
```

---

## 12. Debugging Worker Pools [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

```go
// Add to worker to log queue depth periodically
func (p *WorkerPool) worker(id int) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-p.ctx.Done():
            return
        case job := <-p.jobs:
            select {
            case p.jobs <- job:
                // Check queue depth
                qLen := len(p.jobs)
                if qLen > 80 {
                    log.Printf("WARN: worker %d: queue depth %d", id, qLen)
                }
            default:
            }
        case <-ticker.C:
            log.Printf("DEBUG: worker %d idle, queue: %d", id, len(p.jobs))
        }
    }
}
```

---

## Exercises

### Exercise 1: Basic Worker Pool ⭐
**Difficulty:** Beginner | **Time:** ~15 min

Build a worker pool with 3 workers and 10 jobs. Each job is an integer. A worker receives a job, prints which worker is processing it, and sends `job * 2` as a result. Collect all results in main.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"sync"
)

func worker(id int, jobs <-chan int, results chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs {
		fmt.Printf("worker %d processing job %d\n", id, j)
		results <- j * 2
	}
}

func main() {
	const numWorkers = 3
	const numJobs = 10

	jobs := make(chan int, numJobs)
	results := make(chan int, numJobs)

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for w := 1; w <= numWorkers; w++ {
		go worker(w, jobs, results, &wg)
	}

	for j := 1; j <= numJobs; j++ {
		jobs <- j
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		fmt.Println("result:", r)
	}
}
```

</details>

### Exercise 2: Context Cancellation in Worker Pool ⭐⭐
**Difficulty:** Intermediate | **Time:** ~15 min

Extend Exercise 1 by adding a `context.Context`. Cancel the context after collecting 5 results. Verify that workers stop processing remaining jobs.

<details>
<summary>Solution</summary>

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func worker(ctx context.Context, id int, jobs <-chan int, results chan<- int) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("worker %d shutting down\n", id)
			return
		case j, ok := <-jobs:
			if !ok {
				return
			}
			time.Sleep(50 * time.Millisecond) // simulate work
			select {
			case results <- j * 2:
			case <-ctx.Done():
				return
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan int, 20)
	results := make(chan int, 20)

	for w := 1; w <= 3; w++ {
		go worker(ctx, w, jobs, results)
	}

	// Send 20 jobs
	for j := 1; j <= 20; j++ {
		jobs <- j
	}
	close(jobs)

	// Collect only 5, then cancel
	count := 0
	for r := range results {
		fmt.Println("result:", r)
		count++
		if count >= 5 {
			cancel()
			break
		}
	}

	time.Sleep(100 * time.Millisecond)
	fmt.Println("collected", count, "results")
}
```

</details>

### Exercise 3: Worker Pool with Timeout ⭐⭐
**Difficulty:** Intermediate | **Time:** ~15 min

Add a timeout: if any single job takes longer than 2 seconds, cancel remaining work. Use `context.WithTimeout` on a per-job or global basis.

<details>
<summary>Solution</summary>

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Job struct {
	ID   int
	Sleep time.Duration // simulate variable work time
}

func worker(ctx context.Context, id int, jobs <-chan Job, results chan<- int) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("worker %d: cancelled\n", id)
			return
		case j, ok := <-jobs:
			if !ok {
				return
			}
			fmt.Printf("worker %d: job %d (sleep %v)\n", id, j.ID, j.Sleep)

			jobCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			done := make(chan int, 1)
			go func() {
				time.Sleep(j.Sleep)
				done <- j.ID * 10
			}()

			select {
			case r := <-done:
				results <- r
			case <-jobCtx.Done():
				fmt.Printf("worker %d: job %d timed out\n", id, j.ID)
			}
			cancel()
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan Job, 10)
	results := make(chan int, 10)

	for w := 1; w <= 3; w++ {
		go worker(ctx, w, jobs, results)
	}

	jobs <- Job{ID: 1, Sleep: 500 * time.Millisecond}
	jobs <- Job{ID: 2, Sleep: 3 * time.Second} // will timeout
	jobs <- Job{ID: 3, Sleep: 100 * time.Millisecond}
	close(jobs)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for r := range results {
			fmt.Println("result:", r)
		}
	}()

	time.Sleep(4 * time.Second)
	cancel()
	close(results)
	wg.Wait()
	fmt.Println("done")
}
```

</details>
