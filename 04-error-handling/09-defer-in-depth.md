# 9. Defer In Depth — Complete Deep Dive

> **Goal:** Master defer from fundamentals to production-level patterns. Understand exactly what happens, when, and why.

---
![Defer Keyword](../assets/09.png)
## Table of Contents

1. [What Defer Does](#1-what-defer-does)
2. [Execution Order (LIFO)](#2-execution-order-lifo)
3. [Arguments Are Evaluated Immediately](#3-arguments-are-evaluated-immediately)
4. [Deferred Closures](#4-deferred-closures)
5. [Defer and Return Values](#5-defer-and-return-values)
6. [Defer and Panic/Recover](#6-defer-and-panicrecover)
7. [Defer in Loops](#7-defer-in-loops)
8. [Defer Performance](#8-defer-performance)
9. [Production Patterns](#9-production-patterns)
10. [Advanced Patterns](#10-advanced-patterns)
11. [Common Pitfalls](#11-common-pitfalls)

---

## 1. What Defer Does

`defer` schedules a function call to run **when the surrounding function returns** — not when the block ends.

### Basic Example

```go
func example() {
    defer fmt.Println("I run LAST")
    fmt.Println("I run FIRST")
}

// Output:
// I run FIRST
// I run LAST
```

### Multiple Return Points

```go
func process() error {
    cleanup()
    defer fmt.Println("cleanup done")  // Runs no matter HOW the function returns

    if err := step1(); err != nil {
        return err  // defer runs here
    }
    if err := step2(); err != nil {
        return err  // defer runs here
    }
    return nil      // defer runs here too
}
```

### When Deferred Functions Run

| Function exits via | Defer runs? |
|-------------------|-------------|
| `return` statement | Yes |
| Fall off end of function | Yes |
| `panic` | Yes |
| `runtime.Goexit()` | Yes |
| `os.Exit()` | **No** — program terminates immediately |
| `log.Fatal()` | **No** — calls `os.Exit(1)` |

---

## 2. Execution Order (LIFO)

Deferred functions are pushed onto a **stack**. Last deferred = first executed.

### Basic LIFO

```go
func main() {
    defer fmt.Println("1")
    defer fmt.Println("2")
    defer fmt.Println("3")
}

// Output:
// 3
// 2
// 1
```

### Real-World Example

```go
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()  // 3rd: close file

    gz, err := gzip.NewReader(f)
    if err != nil {
        return err
    }
    defer gz.Close()  // 2nd: close gzip reader

    data, err := io.ReadAll(gz)
    if err != nil {
        return err
    }

    w, err := os.Create("output.txt")
    if err != nil {
        return err
    }
    defer w.Close()  // 1st: close output file (last opened)

    _, err = w.Write(data)
    return err
    // Execution order: w.Close() → gz.Close() → f.Close()
}
```

**Rule:** Defer in reverse order of acquisition. Last acquired = first released.

---

## 3. Arguments Are Evaluated Immediately

This is the **most important defer gotcha**.

### The Gotcha

```go
func main() {
    x := 10
    defer fmt.Println(x)  // Argument x is evaluated NOW → 10

    x = 20
    fmt.Println(x)  // 20

    // When defer runs, it uses the EVALUATED value (10), not the current value (20)
}

// Output:
// 20
// 10
```

### Why This Happens

```go
defer fmt.Println(x)
//     ^^^^^^^^^^^^
//     This call is prepared NOW:
//     1. Evaluate arguments: x → 10
//     2. Store function pointer: fmt.Println
//     3. Store evaluated arguments: 10
//     4. Push onto defer stack
//
// Later, when defer executes:
//     It calls fmt.Println(10) with the STORED argument
```

### Practical Impact

```go
func process(db *sql.DB) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }

    // BUG — err is evaluated NOW (nil), not later
    defer func() {
        if err != nil {
            tx.Rollback()
        } else {
            tx.Commit()
        }
    }()

    _, err = tx.Exec("INSERT ...")
    // err is now set, but defer already captured the nil value
    return err
}

// FIX — use closure to capture current value
func process(db *sql.DB) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }

    defer func() {
        if err != nil {
            tx.Rollback()
        } else {
            tx.Commit()
        }
    }()

    _, err = tx.Exec("INSERT ...")
    return err
}
```

Wait — the fix above actually works because the closure captures `err` by reference! The gotcha only applies when you **pass arguments directly**.

### The Real Gotcha

```go
func main() {
    for i := 0; i < 3; i++ {
        defer fmt.Println(i)  // i is evaluated NOW for each defer
    }
}

// Output:
// 2
// 1
// 0

// Each defer captured the value of i at the time of defer:
// defer 3: fmt.Println(2)  ← captured when i was 2
// defer 2: fmt.Println(1)  ← captured when i was 1
// defer 1: fmt.Println(0)  ← captured when i was 0
```

### With Closures (Different Behavior)

```go
func main() {
    for i := 0; i < 3; i++ {
        defer func() {
            fmt.Println(i)  // i is captured by REFERENCE
        }()
    }
}

// Output:
// 3
// 3
// 3

// All closures reference the SAME variable i
// By the time they execute, i is 3 (loop ended with i=3, then i++ → 3)
```

### Fix: Pass Argument to Closure

```go
func main() {
    for i := 0; i < 3; i++ {
        defer func(val int) {
            fmt.Println(val)  // val is a parameter — unique per call
        }(i)  // i is evaluated NOW and passed as argument
    }
}

// Output:
// 2
// 1
// 0
```

---

## 4. Deferred Closures

### Capture by Reference

```go
func example() {
    x := 10
    defer func() {
        fmt.Println(x)  // Captures x by reference
    }()
    x = 20
}

// Output: 20
// The closure sees the modified value of x
```

### Capture by Value (Using Parameter)

```go
func example() {
    x := 10
    defer func(val int) {
        fmt.Println(val)  // val is a copy — doesn't see modifications
    }(x)
    x = 20
}

// Output: 10
// The parameter captured the value at defer time
```

### Returning Values from Deferred Closures

```go
func example() (result int) {
    defer func() {
        result++  // Modifies the named return value
    }()
    return 5  // Sets result to 5, then defer runs, result becomes 6
}

// Returns 6
```

---

## 5. Defer and Return Values

This is one of Go's most subtle features.

### Named vs Unnamed Returns

```go
// Unnamed return
func add(a, b int) int {
    return a + b
}

// Named return
func add(a, b int) (result int) {
    result = a + b
    return  // naked return — returns result
}
```

### How `return` Works

```go
func example() int {
    x := 10
    return x
    // Steps:
    // 1. Evaluate x → 10
    // 2. Set return value to 10
    // 3. Run defers
    // 4. Return to caller
}
```

### Defer Can Modify Named Returns

```go
func example() (result int) {
    defer func() {
        result++  // Modifies the return value AFTER it's set
    }()
    return 5
}

// Steps:
// 1. return 5 → result = 5
// 2. defer runs → result = 6
// 3. Return 6 to caller
```

### Defer CANNOT Modify Unnamed Returns

```go
func example() int {
    result := 10
    defer func() {
        result++  // Modifies local variable, NOT return value
    }()
    return result
}

// Returns 10
// Steps:
// 1. Evaluate result → 10
// 2. Set return value to 10 (copies result into return slot)
// 3. defer runs → result = 11 (but return slot is already 10)
// 4. Return 10
```

### Deep Dive: The Return Sequence

```go
func example() (result int) {
    x := 5
    defer func() {
        result += x  // x is 5, result is 10 → result = 15
    }()
    return 10
    // Step 1: result = 10 (named return)
    // Step 2: defer → result = 15
    // Step 3: return 15
}
```

### Real-World Example: Wrap Error

```go
func ReadConfig(path string) (cfg Config, err error) {
    defer func() {
        if err != nil {
            err = fmt.Errorf("ReadConfig(%s): %w", path, err)
        }
    }()

    data, err := os.ReadFile(path)
    if err != nil {
        return cfg, err  // err is set, defer will wrap it
    }

    err = json.Unmarshal(data, &cfg)
    return cfg, err  // err might be nil or set
}
```

### Real-World Example: Record Duration

```go
func (s *Service) Process(ctx context.Context, req Request) (resp Response, err error) {
    start := time.Now()
    defer func() {
        s.metrics.RecordDuration("process", time.Since(start), err == nil)
    }()

    resp, err = s.processInternal(ctx, req)
    return resp, err
}
```

---

## 6. Defer and Panic/Recover

### Defer Runs During Panic

```go
func main() {
    defer fmt.Println("defer 1")
    defer fmt.Println("defer 2")
    panic("oh no!")
    defer fmt.Println("never runs")
}

// Output:
// defer 2
// defer 1
// panic: oh no!
```

### Recover Catches Panic

```go
func safeDivide(a, b int) (result int, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered from panic: %v", r)
        }
    }()

    return a / b, nil  // Panics if b == 0
}

func main() {
    result, err := safeDivide(10, 0)
    fmt.Println(result, err)  // 0, "recovered from panic: runtime error: integer divide by zero"
}
```

### Recover Only Works in Deferred Function

```go
func main() {
    recover()           // Does nothing — not in deferred function
    panic("oh no!")
}

func main() {
    panic("oh no!")
    recover()           // Unreachable — panic already happened
}

func main() {
    defer recover()     // Does nothing — recover is called before defer setup
    panic("oh no!")
}

func main() {
    defer func() {
        recover()       // ✅ Works — inside deferred function
    }()
    panic("oh no!")
}
```

### Recover Returns nil If No Panic

```go
func main() {
    defer func() {
        r := recover()
        fmt.Println(r)  // <nil> — no panic occurred
    }()
    fmt.Println("no panic")
}
```

### Recover in Nested Function

```go
func outer() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("outer recovered:", r)
        }
    }()

    inner()  // inner panics
}

func inner() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("inner recovered:", r)
        }
    }()
    panic("inner panic")
}

func main() {
    outer()
    // Output: "inner recovered: inner panic"
    // The inner recover catches it — outer never sees it
}
```

**Recover catches the panic at the FIRST deferred function that calls recover in the call stack.**

---

## 7. Defer in Loops

### The Problem

```go
func processFiles(paths []string) error {
    for _, path := range paths {
        f, err := os.Open(path)
        if err != nil {
            return err
        }
        defer f.Close()  // ALL defers run when processFiles returns!

        // Process file...
    }
    return nil
    // All files close NOW — after ALL processing is done
    // Could exhaust file descriptors for large lists
}
```

### Fix 1: Extract to Function

```go
func processFiles(paths []string) error {
    for _, path := range paths {
        if err := processFile(path); err != nil {
            return err
        }
    }
    return nil
}

func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()  // Closes when processFile returns

    // Process file...
    return nil
}
```

### Fix 2: Anonymous Function

```go
func processFiles(paths []string) error {
    for _, path := range paths {
        if err := func() error {
            f, err := os.Open(path)
            if err != nil {
                return err
            }
            defer f.Close()  // Closes when anonymous func returns

            // Process file...
            return nil
        }(); err != nil {
            return err
        }
    }
    return nil
}
```

### When Defer in Loop is OK

```go
// If you WANT all defers to run at the end of the function
func cleanup() {
    for _, resource := range resources {
        defer resource.Close()  // Close all in reverse order
    }
    // All resources close when cleanup() returns — this is intentional
}
```

---

## 8. Defer Performance

### Cost Per Defer (Go 1.22+)

Go 1.22 significantly improved defer performance. Before that, defer had measurable overhead.

| Go Version | Cost per defer |
|-----------|---------------|
| < 1.13 | ~50ns |
| 1.13-1.21 | ~35ns |
| 1.22+ | ~5-10ns |

### When Performance Matters

```go
// In a tight loop with millions of iterations — avoid defer
func process(items []Item) {
    for _, item := range items {
        // DON'T — millions of defers accumulate
        defer item.Cleanup()
        process(item)
    }
}

// DO — explicit cleanup
func process(items []Item) {
    for _, item := range items {
        process(item)
        item.Cleanup()
    }
}
```

### When Performance Doesn't Matter

```go
// HTTP handler — runs once per request, network I/O dominates
func handler(w http.ResponseWriter, r *http.Request) {
    db, err := sql.Open(...)
    if err != nil { return }
    defer db.Close()  // 5ns is irrelevant vs 50ms database query

    rows, err := db.Query(...)
    if err != nil { return }
    defer rows.Close()

    // ...
}
```

### Benchmark It Yourself

```go
func BenchmarkDefer(b *testing.B) {
    for i := 0; i < b.N; i++ {
        func() {
            defer func() {}()
        }()
    }
}

func BenchmarkExplicit(b *testing.B) {
    for i := 0; i < b.N; i++ {
        func() {
            // no defer
        }()
    }
}
```

---

## 9. Production Patterns

### Pattern 1: Resource Cleanup

```go
func ReadFile(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    return io.ReadAll(f)
}
```

### Pattern 2: Mutex Unlock

```go
func (c *Counter) Increment() int {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.count++
    return c.count
}
```

### Pattern 3: Transaction Commit/Rollback

```go
func Transfer(db *sql.DB, fromID, toID string, amount float64) error {
    tx, err := db.Begin()
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }

    committed := false
    defer func() {
        if !committed {
            if err := tx.Rollback(); err != nil {
                slog.Error("failed to rollback", "error", err)
            }
        }
    }()

    if _, err := tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID); err != nil {
        return fmt.Errorf("debit: %w", err)
    }

    if _, err := tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID); err != nil {
        return fmt.Errorf("credit: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }
    committed = true

    return nil
}
```

### Pattern 4: HTTP Body Close

```go
func fetch(url string) ([]byte, error) {
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()  // MUST close to reuse connections

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }

    return io.ReadAll(resp.Body)
}
```

### Pattern 5: Timing / Metrics

```go
func (s *Service) HandleRequest(ctx context.Context, req Request) (Response, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        s.metrics.Observe("request_duration_seconds", duration.Seconds())
        slog.InfoContext(ctx, "request completed",
            "duration", duration,
            "method", req.Method,
        )
    }()

    return s.process(ctx, req)
}
```

### Pattern 6: Error Wrapping with Named Returns

```go
func (s *Service) CreateOrder(ctx context.Context, req CreateOrderReq) (order *Order, err error) {
    defer func() {
        if err != nil {
            err = fmt.Errorf("CreateOrder: %w", err)
        }
    }()

    order = &Order{
        ID:     generateID(),
        UserID: req.UserID,
        Items:  req.Items,
    }

    if err = s.repo.Save(ctx, order); err != nil {
        return nil, err
    }

    if err = s.notifier.Send(ctx, order); err != nil {
        // Log but don't fail — order is already saved
        slog.ErrorContext(ctx, "failed to send notification", "error", err)
        err = nil
    }

    return order, nil
}
```

### Pattern 7: Panic Recovery in HTTP Handler

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if r := recover(); r != nil {
                slog.Error("panic recovered",
                    "error", r,
                    "path", r.URL.Path,
                    "stack", string(debug.Stack()),
                )
                http.Error(w, "internal server error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

### Pattern 8: Flush on Exit

```go
func (s *BatchWriter) Write(ctx context.Context, records []Record) error {
    for _, rec := range records {
        s.buffer = append(s.buffer, rec)
        if len(s.buffer) >= s.batchSize {
            if err := s.flush(ctx); err != nil {
                return err
            }
        }
    }
    return nil
}

func (s *BatchWriter) Process(ctx context.Context, records []Record) error {
    defer func() {
        // Flush remaining records when function exits
        if len(s.buffer) > 0 {
            if err := s.flush(ctx); err != nil {
                slog.ErrorContext(ctx, "failed to flush remaining records", "error", err)
            }
        }
    }()

    return s.Write(ctx, records)
}
```

---

## 10. Advanced Patterns

### Pattern 1: Defer with Channel Signal

```go
func (s *Worker) Run(ctx context.Context) error {
    done := make(chan struct{})
    defer close(done)  // Signal goroutines to stop

    go s.backgroundTask(done)

    // Do work...
    return s.process(ctx)
}

func (s *Worker) backgroundTask(done <-chan struct{}) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()  // Another defer!

    for {
        select {
        case <-done:
            return
        case <-ticker.C:
            s.doPeriodicWork()
        }
    }
}
```

### Pattern 2: Defer with WaitGroup

```go
func (p *Processor) ProcessAll(ctx context.Context, items []Item) error {
    var wg sync.WaitGroup
    errCh := make(chan error, len(items))

    defer wg.Wait()  // Wait for all goroutines before returning

    for _, item := range items {
        wg.Add(1)
        go func(item Item) {
            defer wg.Done()  // Always signal completion
            if err := p.process(ctx, item); err != nil {
                errCh <- err
            }
        }(item)
    }

    // Collect errors (simplified)
    // In production, use errgroup.Group instead
    select {
    case err := <-errCh:
        return err
    default:
        return nil
    }
}
```

### Pattern 3: Defer with Context Cancellation

```go
func (s *Service) LongOperation(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()  // Always cancel to release resources

    // Use ctx for all operations...
    return s.doWork(ctx)
}
```

### Pattern 4: Defer for Debugging

```go
func process(data []byte) error {
    if os.Getenv("DEBUG") != "" {
        defer func(start time.Time) {
            fmt.Printf("process took %v\n", time.Since(start))
        }(time.Now())
    }

    // ...
    return nil
}
```

### Pattern 5: Multiple Defers for Layered Cleanup

```go
func (s *Service) Import(ctx context.Context, path string) error {
    // Layer 1: Open file
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    // Layer 2: Decompress
    gz, err := gzip.NewReader(f)
    if err != nil {
        return fmt.Errorf("gzip: %w", err)
    }
    defer gz.Close()

    // Layer 3: Database transaction
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    committed := false
    defer func() {
        if !committed {
            tx.Rollback()
        }
    }()

    // Layer 4: Temporary directory
    tmpDir, err := os.MkdirTemp("", "import-*")
    if err != nil {
        return fmt.Errorf("temp dir: %w", err)
    }
    defer os.RemoveAll(tmpDir)

    // Do work...
    if err := s.doImport(ctx, gz, tx, tmpDir); err != nil {
        return err
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }
    committed = true

    return nil
    // Cleanup order:
    // 1. os.RemoveAll(tmpDir)
    // 2. tx.Rollback() (if not committed)
    // 3. gz.Close()
    // 4. f.Close()
}
```

---

## 11. Common Pitfalls

### 1. Arguments Evaluated Immediately

```go
// BUG
for i := 0; i < 3; i++ {
    defer fmt.Println(i)  // Captures value, not reference
}

// FIX
for i := 0; i < 3; i++ {
    i := i  // Create new variable
    defer func() {
        fmt.Println(i)
    }()
}
```

### 2. Loop Defer Accumulation

```go
// BUG — all files stay open until function returns
for _, path := range paths {
    f, _ := os.Open(path)
    defer f.Close()  // Might exhaust file descriptors
}

// FIX — extract to function
for _, path := range paths {
    func() {
        f, _ := os.Open(path)
        defer f.Close()
        // Process file
    }()
}
```

### 3. Defer on Nil Function

```go
var cleanup func()
defer cleanup()  // PANIC: nil function call when defer executes

// FIX
if cleanup != nil {
    defer cleanup()
}
```

### 4. Not Closing HTTP Response Body

```go
// BUG
resp, err := http.Get(url)
if err != nil {
    return err
}
// resp.Body not closed — leaks connection!

// FIX
resp, err := http.Get(url)
if err != nil {
    return err
}
defer resp.Body.Close()
```

### 5. Defer Before Error Check

```go
// BUG
f := os.Open(path)  // Might return error
defer f.Close()      // PANIC if f is nil

// FIX
f, err := os.Open(path)
if err != nil {
    return err
}
defer f.Close()
```

### 6. Defer Inside Conditional

```go
func process(useCache bool) error {
    if useCache {
        f, err := os.Open("cache.db")
        if err != nil {
            return err
        }
        defer f.Close()  // Only deferred if useCache is true
    }
    // ...
    return nil
}
```

This is correct behavior but can be confusing. Be aware that defer is only scheduled when the `defer` statement executes.

### 7. Modifying Named Return After Defer Reads It

```go
func example() (result int) {
    defer func() {
        fmt.Println("defer:", result)  // Reads current result
    }()
    result = 10
    return 20  // Sets result to 20, then defer reads 20
}

// Output: "defer: 20"
```

---

## Quick Reference

```go
// Basic defer
defer f.Close()
defer mu.Unlock()
defer cancel()

// LIFO order
defer fmt.Println("3rd")  // prints last
defer fmt.Println("2nd")  // prints second
defer fmt.Println("1st")  // prints first

// Arguments evaluated immediately
x := 10
defer fmt.Println(x)  // prints 10, even if x changes later

// Closures capture by reference
x := 10
defer func() { fmt.Println(x) }()  // prints current value of x

// Named returns can be modified
func f() (result int) {
    defer func() { result++ }()
    return 5  // returns 6
}

// Recover from panic
defer func() {
    if r := recover(); r != nil {
        log.Println("recovered:", r)
    }
}()

// Timing
start := time.Now()
defer func() {
    fmt.Println("took:", time.Since(start))
}()
```

---

## 12. Production Patterns

### Cleanup Functions

```go
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()
    
    // Process file
    return nil
}
```

### Lock Management

```go
func safeAccess(m *sync.Mutex) {
    m.Lock()
    defer m.Unlock()
    
    // Protected access
}
```

### Database Transactions

```go
func withTx(db *sql.DB, fn func(*sql.Tx) error) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer func() {
        if err != nil {
            tx.Rollback()
        } else {
            tx.Commit()
        }
    }()
    
    return fn(tx)
}
```

### HTTP Response Cleanup

```go
func handler(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    
    defer func() {
        duration := time.Since(start)
        log.Printf("%s %s %s", r.Method, r.URL.Path, duration)
    }()
    
    // Handle request
}
```

### Cleanup in Tests

```go
func TestWithTempFile(t *testing.T) {
    f, err := os.CreateTemp("", "test-*.txt")
    if err != nil {
        t.Fatal(err)
    }
    defer os.Remove(f.Name()) // Clean up file
    defer f.Close()
    
    // Test with file
}
```

---

## 13. Defer Performance

### Benchmark

```go
func BenchmarkDefer(b *testing.B) {
    for i := 0; i < b.N; i++ {
        deferFunc()
    }
}

func deferFunc() {
    defer func() {}()
}

// Defer has ~50ns overhead per call
// Use only when needed, not for trivial cases
```

### When to Use Defer

| Use | Defer? | Reason |
|-----|--------|--------|
| File close | Yes | Always run, even on error |
| Mutex unlock | Yes | Guaranteed unlock |
| Timer stop | Yes | Prevent leaks |
| Simple print | No | Use direct call |
| Hot path | No | 50ns overhead |

---

## 14. Debugging Defer

```go
// Add tracing to understand defer order
func trace(name string) func() {
    fmt.Printf("enter: %s\n", name)
    return func() {
        fmt.Printf("exit: %s\n", name)
    }
}

func example() {
    defer trace("example")()
    defer trace("first")()
    defer trace("second")()
}

// Output:
// enter: example
// enter: first
// enter: second
// exit: second
// exit: first
// exit: example
```

---

## 15. Common Mistakes

### Forgetting Deferred Function Returns Value

```go
// BAD
func bad() int {
    defer func() {
        return 5 // This doesn't change the return!
    }()
    return 1
}
// Returns 1, not 5!

// GOOD
func good() int {
    result := 1
    defer func() {
        result = 5 // This changes result
    }()
    return result
}
// Returns 5
```

### Defer in Loop

```go
// BAD: Deferred functions run after loop ends
for _, file := range files {
    defer os.Remove(file.Name()) // All run after loop!
}

// GOOD: Use anonymous function
for _, file := range files {
    func(f string) {
        defer os.Remove(f)
    }(file.Name())
}
```

### Recover Not Working

```go
// BAD: Recover in different goroutine
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Println(r)
        }
    }()
    panic("oh no") // Won't be caught by main's recover!
}()

// GOOD: Recover in same goroutine
func safe() {
    defer func() {
        if r := recover(); r != nil {
            log.Println(r)
        }
    }()
    panic("oh no") // Will be caught
}
```

---

## End of Series

| # | Topic | File |
|---|-------|------|
| 1 | Go Toolchain | [01-go-toolchain.md](./01-go-toolchain.md) |
| 2 | Variables & Zero Values | [02-variables-zero-values.md](./02-variables-zero-values.md) |
| 3 | Arrays vs Slices | [03-arrays-vs-slices.md](./03-arrays-vs-slices.md) |
| 4 | Maps | [04-maps.md](./04-maps.md) |
| 5 | Structs & Methods | [05-structs-and-methods.md](./05-structs-and-methods.md) |
| 6 | Pointers | [06-pointers.md](./06-pointers.md) |
| 7 | Interfaces | [07-interfaces.md](./07-interfaces.md) |
| 8 | Error Handling | [08-error-handling.md](./08-error-handling.md) |
| 9 | Defer In Depth | [09-defer-in-depth.md](./09-defer-in-depth.md) |
