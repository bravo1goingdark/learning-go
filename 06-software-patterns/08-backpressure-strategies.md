# Backpressure Strategies

> Handle system overload gracefully. Don't let one slow component crash everything.

---

## What is Backpressure?

Backpressure is the mechanism of **signaling upstream components to slow down** when downstream components can't keep up.

```
┌─────────────────────────────────────────────────────────────┐
│                    The Problem                               │
│                                                              │
│  Fast Producer (1000/sec)     Slow Consumer (10/sec)        │
│         │                          ▲                        │
│         │     ┌────────────┐       │                        │
│         └────▶│   Queue    │───────┘                        │
│               │            │                                │
│               │ Growing... │                                │
│               │ Growing... │                                │
│               │  OOM! 💥   │                                │
│               └────────────┘                                │
│                                                              │
│  Without backpressure:                                       │
│  - Queue grows unbounded                                    │
│  - Memory exhaustion                                        │
│  - Latency spikes (old messages stuck)                     │
│  - Process crash (OOM killer)                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Backpressure Strategies Overview

| Strategy | Behavior | Use When |
|----------|----------|----------|
| **Bounded Queue** | Block when full | Reliable processing needed |
| **Drop Newest** | Discard new items | Analytics, metrics |
| **Drop Oldest** | Discard old items | Real-time data |
| **Rate Limiting** | Throttle producer | API protection |
| **Load Shedding** | Drop low-priority work | System under stress |
| **Timeout** | Give up after waiting | User-facing requests |

---

## 1. Bounded Queue (Most Common)

Use a channel with a fixed buffer size. When full, the producer blocks or rejects.

```
┌─────────────────────────────────────────────────────────────┐
│                  Bounded Channel                            │
│                                                              │
│   make(chan Job, 100)  ← Buffer size = 100                 │
│                                                              │
│   [ 1 ][ 2 ][ 3 ]...[ 100 ]                                │
│      ▲                          ▲                           │
│   Producer blocks           Consumer pulls                  │
│   when full                 when available                  │
└─────────────────────────────────────────────────────────────┘
```

```go
// internal/worker/pool.go
package worker

import (
    "errors"
    "sync"
)

var (
    ErrQueueFull  = errors.New("queue is full")
    ErrPoolClosed = errors.New("pool is closed")
)

// Job represents a unit of work
type Job struct {
    ID      string
    Payload interface{}
    Result  chan error // Caller waits for result
}

// Pool is a bounded worker pool with backpressure
type Pool struct {
    jobs    chan Job
    workers int
    wg      sync.WaitGroup
    closed  chan struct{}
}

// New creates a worker pool with bounded queue
func New(workers, queueSize int) *Pool {
    return &Pool{
        jobs:    make(chan Job, queueSize), // BOUNDED!
        workers: workers,
        closed:  make(chan struct{}),
    }
}

// Start begins processing jobs
func (p *Pool) Start(handler func(Job)) {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go func() {
            defer p.wg.Done()
            for {
                select {
                case job, ok := <-p.jobs:
                    if !ok {
                        return // Channel closed
                    }
                    handler(job)
                case <-p.closed:
                    return
                }
            }
        }()
    }
}

// Submit adds a job to the queue (non-blocking)
// Returns ErrQueueFull if queue is at capacity
func (p *Pool) Submit(job Job) error {
    select {
    case p.jobs <- job:
        return nil
    default:
        return ErrQueueFull // BACKPRESSURE: reject!
    }
}

// SubmitBlocking adds a job, blocking until space is available
func (p *Pool) SubmitBlocking(job Job) error {
    select {
    case p.jobs <- job:
        return nil
    case <-p.closed:
        return ErrPoolClosed
    }
}

// SubmitWithTimeout tries to add a job within a timeout
func (p *Pool) SubmitWithTimeout(job Job, timeout time.Duration) error {
    select {
    case p.jobs <- job:
        return nil
    case <-time.After(timeout):
        return ErrQueueFull
    case <-p.closed:
        return ErrPoolClosed
    }
}

// Close shuts down the pool gracefully
func (p *Pool) Close() {
    close(p.closed)
    close(p.jobs)
    p.wg.Wait()
}
```

### Usage

```go
func main() {
    // 10 workers, queue holds 100 jobs max
    pool := worker.New(10, 100)
    pool.Start(func(job worker.Job) {
        // Process job
        err := processJob(job)
        if job.Result != nil {
            job.Result <- err
        }
    })
    defer pool.Close()

    // Submit jobs with backpressure
    for i := 0; i < 10000; i++ {
        job := worker.Job{
            ID:      fmt.Sprintf("job-%d", i),
            Payload: i,
        }

        err := pool.Submit(job)
        if errors.Is(err, worker.ErrQueueFull) {
            // Handle backpressure!
            log.Printf("queue full, rejecting job %d", i)
            continue
        }
    }
}
```

---

## 2. Drop Newest (Lossy)

When queue is full, **discard incoming items**. Use for metrics, logging, non-critical data.

```go
// internal/buffer/drop_newest.go
package buffer

import "sync"

// DropNewestBuffer discards new items when full
type DropNewestBuffer[T any] struct {
    mu      sync.Mutex
    items   []T
    max     int
    dropped int64 // Track how many we dropped
}

func NewDropNewest[T any](maxSize int) *DropNewestBuffer[T] {
    return &DropNewestBuffer[T]{
        items: make([]T, 0, maxSize),
        max:   maxSize,
    }
}

// Add adds an item. If full, the new item is dropped.
func (b *DropNewestBuffer[T]) Add(item T) bool {
    b.mu.Lock()
    defer b.mu.Unlock()

    if len(b.items) >= b.max {
        b.dropped++
        return false // Dropped!
    }

    b.items = append(b.items, item)
    return true
}

// Drain returns and clears all items
func (b *DropNewestBuffer[T]) Drain() []T {
    b.mu.Lock()
    defer b.mu.Unlock()

    result := make([]T, len(b.items))
    copy(result, b.items)
    b.items = b.items[:0]

    return result
}

// Dropped returns how many items were dropped
func (b *DropNewestBuffer[T]) Dropped() int64 {
    b.mu.Lock()
    defer b.mu.Unlock()
    return b.dropped
}
```

---

## 3. Drop Oldest (Ring Buffer)

When full, **discard the oldest items**. Use for real-time data where latest is most important.

```go
// internal/buffer/ring.go
package buffer

import "sync"

// RingBuffer is a fixed-size circular buffer
type RingBuffer[T any] struct {
    mu    sync.Mutex
    items []T
    size  int
    head  int // Write position
    count int
}

func NewRing[T any](size int) *RingBuffer[T] {
    return &RingBuffer[T]{
        items: make([]T, size),
        size:  size,
    }
}

// Add adds an item. If full, overwrites oldest.
func (r *RingBuffer[T]) Add(item T) {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.items[r.head] = item
    r.head = (r.head + 1) % r.size

    if r.count < r.size {
        r.count++
    }
}

// Latest returns the most recent N items
func (r *RingBuffer[T]) Latest(n int) []T {
    r.mu.Lock()
    defer r.mu.Unlock()

    if n > r.count {
        n = r.count
    }

    result := make([]T, n)
    for i := 0; i < n; i++ {
        idx := (r.head - 1 - i + r.size) % r.size
        result[i] = r.items[idx]
    }

    return result
}
```

---

## 4. Rate Limiting (Token Bucket)

Smooth out request rate. Only allow N requests per second.

```go
// internal/resilience/ratelimiter.go
package resilience

import (
    "sync"
    "time"
)

// TokenBucketRateLimiter implements token bucket algorithm
type TokenBucketRateLimiter struct {
    mu         sync.Mutex
    tokens     float64
    maxTokens  float64
    refillRate float64 // tokens per second
    lastRefill time.Time
}

// NewTokenBucket creates a rate limiter
// maxTokens = burst capacity, refillRate = sustained rate per second
func NewTokenBucket(maxTokens int, refillRatePerSecond float64) *TokenBucketRateLimiter {
    return &TokenBucketRateLimiter{
        tokens:     float64(maxTokens),
        maxTokens:  float64(maxTokens),
        refillRate: refillRatePerSecond,
        lastRefill: time.Now(),
    }
}

// Allow checks if a request is allowed (consumes 1 token)
func (tb *TokenBucketRateLimiter) Allow() bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    tb.refill()

    if tb.tokens >= 1.0 {
        tb.tokens--
        return true
    }

    return false
}

// AllowN checks if N requests are allowed
func (tb *TokenBucketRateLimiter) AllowN(n int) bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    tb.refill()

    if tb.tokens >= float64(n) {
        tb.tokens -= float64(n)
        return true
    }

    return false
}

func (tb *TokenBucketRateLimiter) refill() {
    now := time.Now()
    elapsed := now.Sub(tb.lastRefill).Seconds()

    // Add tokens based on elapsed time
    tb.tokens += elapsed * tb.refillRate
    if tb.tokens > tb.maxTokens {
        tb.tokens = tb.maxTokens
    }

    tb.lastRefill = now
}
```

### Usage in HTTP Middleware

```go
// internal/middleware/ratelimit.go
package middleware

import (
    "net/http"
    "myapp/internal/resilience"
)

func RateLimit(limiter *resilience.TokenBucketRateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// In main:
limiter := resilience.NewTokenBucket(100, 10.0) // 100 burst, 10/sec sustained
mux = middleware.RateLimit(limiter)(mux)
```

---

## 5. Load Shedding

Under extreme load, **drop low-priority work** to protect critical operations.

```go
// internal/resilience/loadshedder.go
package resilience

import (
    "errors"
    "sync"
    "sync/atomic"
)

// Priority defines request priority
type Priority int

const (
    PriorityCritical Priority = iota // Payments, auth
    PriorityNormal                    // Regular API
    PriorityLow                       // Analytics, background
)

var ErrLoadShedding = errors.New("system overloaded, request rejected")

// LoadShedder rejects low-priority work under load
type LoadShedder struct {
    activeRequests int32
    
    // Limits per priority
    limits map[Priority]int32
}

func NewLoadShedder() *LoadShedder {
    return &LoadShedder{
        limits: map[Priority]int32{
            PriorityCritical: 1000,
            PriorityNormal:    500,
            PriorityLow:       100,
        },
    }
}

// Allow checks if a request should be accepted
func (ls *LoadShedder) Allow(prio Priority) (func(), error) {
    current := atomic.LoadInt32(&ls.activeRequests)
    limit := ls.limits[prio]

    if current >= limit {
        // Critical requests always allowed (up to their higher limit)
        if prio == PriorityCritical && current < ls.limits[PriorityCritical] {
            // OK
        } else {
            return nil, ErrLoadShedding
        }
    }

    atomic.AddInt32(&ls.activeRequests, 1)

    // Return release function
    release := func() {
        atomic.AddInt32(&ls.activeRequests, -1)
    }

    return release, nil
}
```

### HTTP Middleware

```go
func LoadShedding(ls *LoadShedder, prio Priority) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            release, err := ls.Allow(prio)
            if err != nil {
                http.Error(w, "service overloaded", http.StatusServiceUnavailable)
                return
            }
            defer release()

            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 6. Context-Aware Backpressure

Use context to propagate cancellation and timeouts:

```go
func (p *Pool) SubmitWithContext(ctx context.Context, job Job) error {
    select {
    case p.jobs <- job:
        return nil
    case <-ctx.Done():
        return ctx.Err() // Context cancelled/timed out
    case <-p.closed:
        return ErrPoolClosed
    }
}
```

---

## Full HTTP Server with Backpressure

```go
// cmd/api/main.go
package main

import (
    "log"
    "net/http"
    "sync/atomic"

    "myapp/internal/resilience"
)

type server struct {
    active      int32
    maxActive   int32
    rateLimiter *resilience.TokenBucketRateLimiter
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Rate limiting
    if !s.rateLimiter.Allow() {
        http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
        return
    }

    // 2. Load shedding
    current := atomic.AddInt32(&s.active, 1)
    defer atomic.AddInt32(&s.active, -1)

    if current > s.maxActive {
        http.Error(w, "service overloaded", http.StatusServiceUnavailable)
        return
    }

    // 3. Timeout context
    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()

    r = r.WithContext(ctx)

    // 4. Process request
    s.handleRequest(w, r)
}

func main() {
    srv := &server{
        maxActive:   1000,
        rateLimiter: resilience.NewTokenBucket(100, 100.0), // 100 burst, 100/sec
    }

    log.Println("Starting server on :8080")
    log.Fatal(http.ListenAndServe(":8080", srv))
}
```

---

## Choosing a Strategy

| Scenario | Best Strategy |
|----------|---------------|
| **User requests** | Rate limiting + load shedding |
| **Analytics/logs** | Drop newest |
| **Real-time data** | Drop oldest (ring buffer) |
| **Payments/critical** | Block with timeout (never drop) |
| **Background jobs** | Bounded queue |
| **External API calls** | Rate limiting + circuit breaker |

---

## Quick Reference

| Pattern | Command | Behavior |
|---------|---------|----------|
| Bounded queue | `make(chan T, N)` | Block when full |
| Drop newest | Check size before add | Discard new items |
| Ring buffer | Circular array | Overwrite oldest |
| Token bucket | Rate limiter | Smooth throughput |
| Load shedding | Priority check | Drop low-priority |

---

## Common Pitfalls

1. **Unbounded queues** — Memory leak waiting to happen
2. **No timeouts** — Requests block forever
3. **No metrics** — Can't detect overload
4. **Dropping blindly** — Should prioritize critical work
5. **No graceful degradation** — Return partial results instead of errors

---

## Next Steps

- [Milestone Project](../projects/20-layered-http-service.md) — Build complete service with all patterns