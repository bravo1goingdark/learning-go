# Backpressure Strategies

> Handle system overload gracefully. Don't let one slow component crash everything.

---

## The Problem

Fast producers overwhelm slow consumers:

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│  Producer   │─────▶│   Queue     │─────▶│  Consumer   │
│  1000/sec    │      │   (unbounded) │      │   10/sec    │
└─────────────┘      └─────────────┘      └─────────────┘
                                                    
                              ▼                             
                     Memory explodes!                       
                     Process killed                         
```

Without backpressure:
- **Memory exhaustion** - Unbounded queues grow forever
- **Latency spikes** - Old messages take too long
- **Cascade failure** - OOM kills the entire process

---

## Backpressure Patterns

```
┌─────────────────────────────────────────────────────────────┐
│                  Backpressure Strategies                    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Drop (oldest/newest)     - Accept some data loss       │
│  2. Block (synchronous)      - Producer waits              │
│  3.背压 (bounded queue)       - Limit queue size            │
│  4. Rate limiting             - Throttle producer           │
│  5. Load shedding             - Reject non-critical work    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 1. Bounded Queue with Goroutines

```go
// internal/worker/pool.go
package worker

type Job struct {
    ID   int
    Data string
    Done chan<- error
}

type Pool struct {
    jobs    chan Job        // Bounded queue
    workers int
}

func New(workers, queueSize int) *Pool {
    return &Pool{
        jobs:    make(chan Job, queueSize), // Bounded!
        workers: workers,
    }
}

func (p *Pool) Start(handler func(Job)) {
    for i := 0; i < p.workers; i++ {
        go func() {
            for job := range p.jobs {
                handler(job)
            }
        }()
    }
}

func (p *Pool) Submit(job Job) error {
    select {
    case p.jobs <- job:
        return nil
    default:
        return ErrQueueFull // Backpressure: reject!
    }
}
```

---

## 2. Blocking Producer (Slow Consumer)

```go
// Producer blocks when queue is full
func producer(p *Pool) {
    for i := 0; i < 1000; i++ {
        job := Job{
            ID:   i,
            Data: fmt.Sprintf("data-%d", i),
            Done: make(chan error),
        }

        // This will block if queue is full
        // Backpressure propagates to producer
        p.Submit(job) // Will wait if full
    }
}
```

---

## 3. Channel with Select + Timeout

```go
func submitWithTimeout(p *Pool, job Job, timeout time.Duration) error {
    select {
    case p.jobs <- job:
        return nil
    case <-time.After(timeout):
        return ErrTimeout // Give up after timeout
    }
}
```

---

## 4. Rate Limiting (Token Bucket)

```go
// internal/resilience/ratelimit.go
package resilience

import (
    "sync"
    "time"
)

type RateLimiter struct {
    tokens    int
    maxTokens int
    refillRate time.Duration
    lastRefill time.Time
    mu        sync.Mutex
}

func NewRateLimiter(maxTokens int, refillRate time.Duration) *RateLimiter {
    return &RateLimiter{
        tokens:    maxTokens,
        maxTokens: maxTokens,
        refillRate: refillRate,
        lastRefill: time.Now(),
    }
}

func (rl *RateLimiter) Allow() bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    rl.refill()

    if rl.tokens > 0 {
        rl.tokens--
        return true
    }

    return false
}

func (rl *RateLimiter) refill() {
    now := time.Now()
    elapsed := now.Sub(rl.lastRefill)

    // Add tokens based on elapsed time
    additional := int(elapsed / rl.refillRate)
    if additional > 0 {
        rl.tokens = min(rl.maxTokens, rl.tokens+additional)
        rl.lastRefill = now
    }
}
```

---

## 5. Load Shedding (Drop Non-Essential)

```go
type RequestType string

const (
    RequestTypeCritical RequestType = "critical"
    RequestTypeNormal   RequestType = "normal"
    RequestTypeLow     RequestType = "low"
)

type LoadShedder struct {
    limits map[RequestType]int
    counts map[RequestType]int
    mu     sync.Mutex
}

func NewLoadShedder() *LoadShedder {
    return &LoadShedder{
        limits: map[RequestType]int{
            RequestTypeCritical: 1000,
            RequestTypeNormal:    500,
            RequestTypeLow:      100,
        },
        counts: make(map[RequestType]int),
    }
}

func (ls *LoadShedder) Allow(reqType RequestType) bool {
    ls.mu.Lock()
    defer ls.mu.Unlock()

    limit := ls.limits[reqType]
    count := ls.counts[reqType]

    if count >= limit {
        // Drop low/normal if overwhelmed
        return reqType == RequestTypeCritical
    }

    ls.counts[reqType]++
    return true
}

func (ls *LoadShedder) Release(reqType RequestType) {
    ls.mu.Lock()
    defer ls.mu.Unlock()

    if ls.counts[reqType] > 0 {
        ls.counts[reqType]--
    }
}
```

---

## HTTP Server with Backpressure

```go
// cmd/api/main.go
package main

import (
    "net/http"
    "sync/atomic"
)

type server struct {
    activeRequests int32
    maxRequests    int32
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Check current load
    current := atomic.AddInt32(&s.activeRequests, 1)
    defer atomic.AddInt32(&s.activeRequests, -1)

    // Reject if overloaded
    if current > s.maxRequests {
        http.Error(w, "service overloaded", http.StatusServiceUnavailable)
        return
    }

    // Process normally
    // ...
}

func main() {
    s := &server{maxRequests: 100}
    http.ListenAndServe(":8080", s)
}
```

---

## Visual: Backpressure Flow

```
                    Normal Load                     Overload
                    
    Requests ──▶ [Queue] ──▶ Worker             Requests ──▶ [FULL] ──▶ REJECT
                    │                                   │
                    ▼                                   ▼
               Process OK                         503 Service Unavailable
               
    Memory: stable                              Memory: stable
    Latency: low                                Latency: controlled
```

---

## Choosing a Strategy

| Scenario | Strategy |
|----------|----------|
| Image processing | Bounded queue, drop oldest |
| User requests | Block with timeout |
| Analytics/Metrics | Drop newest (eventual consistency OK) |
| Critical payments | Block (don't lose data) |
| Read-heavy API | Load shed non-critical |

---

## Quick Reference

| Technique | What It Does |
|-----------|--------------|
| Bounded queue | Limit in-flight work |
| Select + timeout | Non-blocking check |
| Token bucket | Smooth rate limiting |
| Load shedding | Prioritize critical work |
| Circuit breaker | Stop calling failing service |

---

## Common Pitfalls

1. **Unbounded queues** - Memory leak waiting to happen
2. **No timeouts** - Blocked forever
3. **All-or-nothing** - Either fully backpressure or nothing
4. **Ignoring metrics** - Don't know when to shed load

---

## Next Steps

- [Milestone Project](20-layered-http-service.md) - Build complete service with all patterns