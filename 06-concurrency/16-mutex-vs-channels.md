# 16. Mutex vs Channels — Complete Deep Dive

> **Goal:** Know when to use `sync.Mutex` and when to use channels. They solve different problems. The wrong choice leads to complex, buggy code.

---
![Mutex vs Channels](../assets/15.png)

## Table of Contents

1. [The Core Difference](#1-the-core-difference)
2. [When to Use Mutex](#2-when-to-use-mutex)
3. [When to Use Channels](#3-when-to-use-channels)
4. [Mutex Deep Dive](#4-mutex-deep-dive)
5. [RWMutex](#5-rwmutex)
6. [sync.Map](#6-syncmap)
7. [sync.Once](#7-synconce)
8. [Channel as Mutex](#8-channel-as-mutex)
9. [Side-by-Side Comparison](#9-side-by-side-comparison)
10. [Decision Flowchart](#10-decision-flowchart)

---

## 1. The Core Difference

| | Mutex | Channel |
|-|-------|---------|
| **Mental model** | Shared room, one at a time | Conveyor belt, pass items |
| **Protects** | Shared state | Data flow between goroutines |
| **Synchronization** | Lock/unlock | Send/receive |
| **Ownership** | No ownership transfer | Ownership transferred on send |
| **Complexity** | Simple for single resource | Better for pipelines/workflows |

> **Rule of thumb:**
> - **Mutex** = "protect this data structure"
> - **Channel** = "pass work between goroutines"

---

## 2. When to Use Mutex

Use a mutex when **multiple goroutines access shared state** and you need mutual exclusion.

### Caches, Counters, Maps

```go
type SafeCounter struct {
    mu sync.Mutex
    m  map[string]int
}

func (c *SafeCounter) Inc(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.m[key]++
}

func (c *SafeCounter) Get(key string) int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.m[key]
}
```

### Connection Pool

```go
type Pool struct {
    mu    sync.Mutex
    conns []Connection
}

func (p *Pool) Get() Connection {
    p.mu.Lock()
    defer p.mu.Unlock()
    if len(p.conns) == 0 {
        return newConnection()
    }
    conn := p.conns[len(p.conns)-1]
    p.conns = p.conns[:len(p.conns)-1]
    return conn
}

func (p *Pool) Put(conn Connection) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.conns = append(p.conns, conn)
}
```

---

## 3. When to Use Channels

Use channels when you need to **coordinate goroutines** or **pass data between them**.

### Work Distribution

```go
func distribute(jobs []Job, workers int) []Result {
    jobCh := make(chan Job, len(jobs))
    resCh := make(chan Result, len(jobs))

    for _, j := range jobs {
        jobCh <- j
    }
    close(jobCh)

    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := range jobCh {
                resCh <- process(j)
            }
        }()
    }

    go func() {
        wg.Wait()
        close(resCh)
    }()

    var results []Result
    for r := range resCh {
        results = append(results, r)
    }
    return results
}
```

### Signal Completion

```go
done := make(chan struct{})
go func() {
    doWork()
    done <- struct{}{}
}()
<-done
```

---

## 4. Mutex Deep Dive

### Basic Mutex

```go
var mu sync.Mutex
mu.Lock()
// Critical section — only one goroutine here at a time
mu.Unlock()
```

### Rules

- `Lock()` blocks until the mutex is available
- Only the **locking goroutine** should unlock
- Always unlock — use `defer` to guarantee it
- A locked mutex locked again by the same goroutine = **deadlock**

### Protecting Struct Fields

```go
type UserCache struct {
    mu    sync.Mutex
    users map[string]User
}

func NewUserCache() *UserCache {
    return &UserCache{
        users: make(map[string]User),
    }
}

func (c *UserCache) Set(id string, u User) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.users[id] = u
}

func (c *UserCache) Get(id string) (User, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    u, ok := c.users[id]
    return u, ok
}

func (c *UserCache) Delete(id string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.users, id)
}
```

### Scoped Lock Pattern

```go
func (c *UserCache) Count() int {
    c.mu.Lock()
    n := len(c.users)
    c.mu.Unlock()
    return n
    // Lock released before return — defer not needed for simple cases
}
```

---

## 5. RWMutex

`sync.RWMutex` allows **multiple readers** OR **one writer**.

```go
type Config struct {
    mu   sync.RWMutex
    data map[string]string
}

func (c *Config) Get(key string) string {
    c.mu.RLock()         // Read lock — multiple readers allowed
    defer c.mu.RUnlock()
    return c.data[key]
}

func (c *Config) Set(key, val string) {
    c.mu.Lock()          // Write lock — exclusive access
    defer c.mu.Unlock()
    c.data[key] = val
}
```

### When to Use RWMutex

| Scenario | Use |
|----------|-----|
| Many reads, rare writes | `RWMutex` |
| Mostly writes | `Mutex` (RWMutex overhead not worth it) |
| Equal reads/writes | Benchmark both, `Mutex` often wins |

### Read/Write Behavior

```
RLock()  ──► Goroutine A reads  ──► RUnlock()
RLock()  ──► Goroutine B reads  ──► RUnlock()  (concurrent — OK)
RLock()  ──► Goroutine C reads  ──► RUnlock()

Lock()   ──► Goroutine D writes ──► Unlock()   (exclusive — blocks all)
```

---

## 6. sync.Map

Optimized for two common patterns: keys rarely change, or goroutines read/write disjoint sets of keys.

```go
var m sync.Map

// Store
m.Store("key", "value")

// Load
val, ok := m.Load("key")

// Load or Store (atomic)
val, loaded := m.LoadOrStore("key", "value")

// Delete
m.Delete("key")

// Range (iterate all keys)
m.Range(func(key, value any) bool {
    fmt.Println(key, value)
    return true // return false to stop
})
```

### When to Use sync.Map

| Pattern | Use `sync.Map`? |
|---------|----------------|
| Key written once, read many times | Yes |
| Multiple goroutines read/write disjoint keys | Yes |
| Structured data, many writes to same keys | No — use `map` + `Mutex` |

---

## 7. sync.Once

Ensures a function runs **exactly once**, even with concurrent calls.

```go
var (
    instance *DB
    once     sync.Once
)

func GetDB() *DB {
    once.Do(func() {
        instance = &DB{}
        instance.Connect()
    })
    return instance
}
```

### Thread-Safe Singleton

```go
type Service struct {
    client *http.Client
}

var (
    svc  *Service
    once sync.Once
)

func GetService() *Service {
    once.Do(func() {
        svc = &Service{
            client: &http.Client{Timeout: 10 * time.Second},
        }
    })
    return svc
}
```

### Rules

- `Do` takes a **function with no args and no return**
- The function runs **exactly once** — subsequent calls are no-ops
- Safe for concurrent use

---

## 8. Channel as Mutex

You can use a buffered channel of size 1 as a mutex. **Don't do this in production** — use `sync.Mutex`.

```go
sem := make(chan struct{}, 1) // Buffer of 1 = mutex

sem <- struct{}{}       // Lock
// Critical section
<-sem                   // Unlock
```

### Why Mutex Is Better

```go
// Channel mutex: 5-10x slower
sem <- struct{}{} // Lock — involves channel internals, memory barriers
<-sem             // Unlock

// sync.Mutex: direct atomic operations
mu.Lock()   // Fast atomic CAS
mu.Unlock() // Fast atomic release
```

| Feature | Channel Mutex | sync.Mutex |
|---------|--------------|------------|
| Performance | Slower | Faster |
| Readability | Confusing | Clear intent |
| TryLock | Possible with select | `TryLock()` in Go 1.18+ |
| Use | Learning only | Production code |

---

## 9. Side-by-Side Comparison

### Same Problem, Two Solutions

**Problem:** Multiple goroutines increment a counter.

#### Mutex Solution

```go
type Counter struct {
    mu    sync.Mutex
    value int
}

func (c *Counter) Inc() {
    c.mu.Lock()
    c.value++
    c.mu.Unlock()
}

func (c *Counter) Value() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.value
}
```

#### Channel Solution

```go
type Counter struct {
    inc   chan struct{}
    get   chan int
}

func NewCounter() *Counter {
    c := &Counter{
        inc: make(chan struct{}),
        get: make(chan int),
    }
    go c.run()
    return c
}

func (c *Counter) run() {
    var val int
    for {
        select {
        case <-c.inc:
            val++
        case c.get <- val:
        }
    }
}

func (c *Counter) Inc()  { c.inc <- struct{}{} }
func (c *Counter) Value() int { return <-c.get }
```

#### Verdict

| | Mutex | Channel |
|-|-------|---------|
| Lines of code | 12 | 22 |
| Goroutines | 0 extra | 1 extra |
| Performance | Fast | Slower |
| Complexity | Simple | More complex |

**Winner for this case: Mutex.** Channels add overhead with no benefit.

---

## 10. Decision Flowchart

```
Do you need to protect shared state?
│
├── YES ──► Is it read-heavy, write-light?
│           │
│           ├── YES ──► sync.RWMutex
│           │
│           └── NO  ──► sync.Mutex
│
└── NO  ──► Do you need to pass data between goroutines?
            │
            ├── YES ──► Channel
            │
            └── NO  ──► Do you need to signal completion?
                        │
                        ├── YES ──► sync.WaitGroup or chan struct{}
                        │
                        └── NO  ──► Do you need one-time initialization?
                                    │
                                    └── YES ──► sync.Once
```

### Quick Reference

| Need | Use |
|------|-----|
| Protect shared map/counter | `sync.Mutex` |
| Protect read-heavy data | `sync.RWMutex` |
| Pass work to goroutines | Channel (buffered) |
| Wait for goroutines | `sync.WaitGroup` |
| One-time init | `sync.Once` |
| Concurrent map (special cases) | `sync.Map` |
| Pipeline processing | Channels |
| Graceful shutdown | `context.Context` + WaitGroup |
