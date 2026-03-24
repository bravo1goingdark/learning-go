# 13. Context — Complete Deep Dive

> **Goal:** Master `context.Context` — cancellation, deadlines, values, and every production pattern. Context is how Go propagates cancellation across API boundaries.

---

## Table of Contents

1. [What Is Context](#1-what-is-context)
2. [Creating Contexts](#2-creating-contexts)
3. [Cancellation](#3-cancellation)
4. [Deadlines & Timeouts](#4-deadlines--timeouts)
5. [Context Values](#5-context-values)
6. [Context Propagation](#6-context-propagation)
7. [Context in HTTP Servers](#7-context-in-http-servers)
8. [ErrCause Inspection](#8-errcause-inspection)
9. [Common Patterns](#9-common-patterns)
10. [Common Pitfalls](#10-common-pitfalls)

---

## 1. What Is Context

`context.Context` carries **cancellation signals**, **deadlines**, and **request-scoped values** across API boundaries and between goroutines.

```go
type Context interface {
    Deadline() (deadline time.Time, ok bool)
    Done() <-chan struct{}
    Err() error
    Value(key any) any
}
```

### Interface Methods

| Method | Purpose |
|--------|---------|
| `Deadline()` | When the context expires (if set) |
| `Done()` | Channel closed on cancellation/timeout |
| `Err()` | Why it was cancelled (`Canceled` or `DeadlineExceeded`) |
| `Value(key)` | Retrieve request-scoped value |

---

## 2. Creating Contexts

### `context.Background()`

The root context. Use at the top level: `main`, `init`, tests.

```go
ctx := context.Background() // Never cancelled, no deadline, no values
```

### `context.TODO()`

Placeholder when you're unsure which context to use.

```go
ctx := context.TODO() // Same as Background, signals "fix me later"
```

### When to Use Which

| Function | Use |
|----------|-----|
| `main()`, `init()` | `Background()` |
| Tests | `Background()` or `TODO()` |
| Not sure yet | `TODO()` |
| Everything else | Derive from parent context |

---

## 3. Cancellation

### `context.WithCancel`

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel() // Always defer — releases resources

go func() {
    <-ctx.Done()
    fmt.Println("cancelled:", ctx.Err())
}()

// Later...
cancel() // Triggers ctx.Done() for all goroutines using ctx
```

### Cancellation Propagates Down

```
                    Background()
                         │
                   ┌─────┴─────┐
                   │ WithCancel │  ← parent
                   └─────┬─────┘
                   ┌─────┴─────┐
                   │ WithCancel │  ← child
                   └─────┬─────┘
                   ┌─────┴─────┐
                   │ WithCancel │  ← grandchild
                   └───────────┘

cancel() on parent → child cancelled → grandchild cancelled
```

```go
parent, cancelParent := context.WithCancel(context.Background())
child, cancelChild := context.WithCancel(parent)

// Cancelling parent also cancels child
cancelParent()
<-child.Done() // Unblocks — child is cancelled

// Cancelling child does NOT cancel parent
cancelChild()
// parent is still alive
```

---

## 4. Deadlines & Timeouts

### `context.WithTimeout`

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

select {
case result := <-doWork(ctx):
    fmt.Println("done:", result)
case <-ctx.Done():
    fmt.Println("timed out:", ctx.Err())
    // Output: timed out: context deadline exceeded
}
```

### `context.WithDeadline`

```go
deadline := time.Now().Add(10 * time.Second)
ctx, cancel := context.WithDeadline(context.Background(), deadline)
defer cancel()
```

### Comparison

| Function | Takes | Use When |
|----------|-------|----------|
| `WithTimeout` | `time.Duration` | "Wait at most N seconds" |
| `WithDeadline` | `time.Time` | "Must finish by 3:00 PM" |

### Handling Deadline Errors

```go
func fetch(ctx context.Context, url string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        if ctx.Err() != nil {
            return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
        }
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

---

## 5. Context Values

### Storing Values

```go
type key string

const requestIDKey key = "requestID"

ctx := context.WithValue(context.Background(), requestIDKey, "abc-123")
```

### Retrieving Values

```go
val := ctx.Value(requestIDKey)
if rid, ok := val.(string); ok {
    fmt.Println("request ID:", rid)
}
```

### Rules for Context Keys

1. **Use unexported custom type** as key — prevents collisions
2. **Store only request-scoped data** — trace IDs, auth tokens
3. **Never store optional function params** — pass them explicitly

```go
// GOOD: custom unexported type
type contextKey string
const traceIDKey contextKey = "traceID"

// BAD: string key — can collide with other packages
ctx := context.WithValue(ctx, "traceID", "abc") // Don't do this
```

---

## 6. Context Propagation

### The Rule

> **Accept `context.Context` as the first parameter. Pass it to every downstream call.**

```go
// GOOD
func HandleRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
    user := authenticate(ctx, r)
    data := fetchFromDB(ctx, user.ID)
    render(w, data)
}

func authenticate(ctx context.Context, r *http.Request) User {
    // Uses ctx for tracing, cancellation
}

func fetchFromDB(ctx context.Context, id int) Data {
    // Uses ctx for query timeout
}
```

### Context Chain in Real Code

```go
func ProcessOrder(ctx context.Context, orderID string) error {
    ctx, span := tracer.Start(ctx, "ProcessOrder") // Tracing
    defer span.End()

    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    order, err := getOrder(ctx, orderID)
    if err != nil {
        return fmt.Errorf("get order: %w", err)
    }

    if err := chargePayment(ctx, order); err != nil {
        return fmt.Errorf("charge: %w", err)
    }

    if err := sendConfirmation(ctx, order); err != nil {
        return fmt.Errorf("confirm: %w", err)
    }

    return nil
}
```

---

## 7. Context in HTTP Servers

### Automatic Context Cancellation

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // Auto-cancelled when client disconnects

    select {
    case result := <-slowOperation(ctx):
        fmt.Fprintf(w, "result: %s", result)
    case <-ctx.Done():
        log.Println("client disconnected:", ctx.Err())
        // Response already failed — can't write to w
    }
}
```

### With Timeout

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    data, err := fetchWithContext(ctx)
    if err != nil {
        http.Error(w, err.Error(), http.StatusGatewayTimeout)
        return
    }
    json.NewEncoder(w).Encode(data)
}
```

### Middleware: Injecting Values

```go
func WithRequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rid := r.Header.Get("X-Request-ID")
        if rid == "" {
            rid = uuid.New().String()
        }

        ctx := context.WithValue(r.Context(), requestIDKey, rid)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## 8. ErrCause Inspection

### `ctx.Err()` Returns

| Context cancelled by | `ctx.Err()` returns |
|---------------------|---------------------|
| `cancel()` called | `context.Canceled` |
| Timeout expired | `context.DeadlineExceeded` |
| Deadline passed | `context.DeadlineExceeded` |
| Parent cancelled | Parent's error (propagated) |

```go
func handleErr(err error) {
    switch {
    case errors.Is(err, context.Canceled):
        log.Println("operation cancelled by caller")
    case errors.Is(err, context.DeadlineExceeded):
        log.Println("operation timed out")
    default:
        log.Println("operation failed:", err)
    }
}
```

---

## 9. Common Patterns

### Fan-Out with Shared Context

```go
func processAll(ctx context.Context, items []Item) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    errCh := make(chan error, len(items))

    for _, item := range items {
        go func(it Item) {
            errCh <- processItem(ctx, it)
        }(item)
    }

    for range items {
        if err := <-errCh; err != nil {
            cancel() // Cancel all remaining goroutines
            return err
        }
    }
    return nil
}
```

### Context-Aware Retry

```go
func retry(ctx context.Context, fn func() error, max int) error {
    for i := 0; i < max; i++ {
        if err := fn(); err == nil {
            return nil
        }

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(backoff(i)):
            // retry
        }
    }
    return fmt.Errorf("failed after %d retries", max)
}
```

---

## 10. Common Pitfalls

| Pitfall | Problem | Fix |
|---------|---------|-----|
| Not deferring `cancel()` | Resource/goroutine leak | Always `defer cancel()` |
| Passing `nil` context | Panic | Use `context.Background()` or `TODO()` |
| Using context for optional params | Wrong abstraction | Pass params explicitly |
| String keys for values | Namespace collisions | Use unexported custom type keys |
| Not checking `ctx.Err()` | Swallows cancellation | Always check after select/blocked call |
| Storing structs in values | Type assertion pain | Store simple types, use typed keys |
| Calling cancel too early | Downstream work cancelled | Call `defer cancel()` at the right scope |

---

## 11. Production Best Practices

### HTTP Middleware with Context

```go
func tracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx, span := tracer.Start(r.Context(), r.URL.Path)
        defer span.End()

        // Add trace ID to response headers
        w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        user, err := validateToken(token)
        if err != nil {
            http.Error(w, "invalid token", http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), userKey{}, user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Database Operations with Context

```go
func queryWithTimeout(ctx context.Context, db *sql.DB, query string) (*sql.Rows, error) {
    // Context timeout applies to query execution
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    return db.QueryContext(ctx, query)
}

func transactionWithRetry(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
    maxRetries := 3

    for attempt := 0; attempt < maxRetries; attempt++ {
        tx, err := db.BeginTx(ctx, nil)
        if err != nil {
            return fmt.Errorf("begin tx: %w", err)
        }

        if err := fn(tx); err != nil {
            tx.Rollback()
            if isRetryable(err) && attempt < maxRetries-1 {
                time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
                continue
            }
            return fmt.Errorf("tx failed: %w", err)
        }

        if err := tx.Commit(); err != nil {
            if isRetryable(err) && attempt < maxRetries-1 {
                continue
            }
            return fmt.Errorf("commit: %w", err)
        }

        return nil
    }
    return errors.New("max retries exceeded")
}
```

### gRPC Interceptors

```go
// Server interceptor for timeout
func serverTimeoutInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        ctx, cancel := context.WithTimeout(ctx, timeout)
        defer cancel()
        return handler(ctx, req)
    }
}

// Client interceptor for tracing
func clientTracingInterceptor() grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        ctx, span := tracer.Start(ctx, method)
        defer span.End()

        span.SetAttributes(attribute.String("grpc.method", method))

        err := invoker(ctx, method, req, reply, cc, opts...)
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
        }
        return err
    }
}
```

### Graceful Shutdown with Context

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Setup signal handler
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigCh
        log.Println("Shutting down gracefully...")
        cancel()
    }()

    // Start server
    srv := &http.Server{Addr: ":8080"}
    go func() {
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    // Wait for shutdown signal
    <-ctx.Done()

    // Graceful shutdown with timeout
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer shutdownCancel()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        log.Printf("Server shutdown error: %v", err)
    }

    log.Println("Server stopped")
}
```

### Request Context vs Background

```go
// BAD: Using Background() loses cancellation
func badHandler(w http.ResponseWriter, r *http.Request) {
    ctx := context.Background() // Loses request cancellation!

    result, err := fetchData(ctx, r.URL.Query())
    // ...
}

// GOOD: Use request context
func goodHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // Preserves client disconnect cancellation

    result, err := fetchData(ctx, r.URL.Query())
    // ...
}

// GOOD: Derive from request context with timeout
func bestHandler(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()

    result, err := fetchData(ctx, r.URL.Query())
    // ...
}
```

### Context Value Pattern for Request-Scoped Data

```go
// Define keys as unexported custom types
type (
    userIDKey    struct{}
    requestIDKey struct{}
    traceIDKey   struct{}
)

// Functions to set values
func WithUserID(ctx context.Context, userID string) context.Context {
    return context.WithValue(ctx, userIDKey{}, userID)
}

func UserID(ctx context.Context) string {
    if v := ctx.Value(userIDKey{}); v != nil {
        return v.(string)
    }
    return ""
}

func WithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestID(ctx context.Context) string {
    if v := ctx.Value(requestIDKey{}); v != nil {
        return v.(string)
    }
    return ""
}

// Usage in handler
func handle(w http.ResponseWriter, r *http.Request) {
    ctx := WithRequestID(r.Context(), generateRequestID())

    user, _ := getUser(r) // Assume user is authenticated
    ctx = WithUserID(ctx, user.ID)

    processRequest(ctx, r) // Pass ctx down
}
```

---

## 12. Testing with Context

```go
func TestContextCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())

    result, err := doWork(ctx)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    cancel()

    // This should fail because context is cancelled
    _, err = doWork(ctx)
    if !errors.Is(err, context.Canceled) {
        t.Fatalf("expected context.Canceled, got: %v", err)
    }
}

func TestContextTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    done := make(chan struct{})
    go func() {
        time.Sleep(200 * time.Millisecond)
        close(done)
    }()

    select {
    case <-done:
        t.Fatal("should have timed out")
    case <-ctx.Done():
        // Expected
    }
}

func TestContextMock(t *testing.T) {
    // Create a context that can be manually cancelled
    ctx, cancel := context.WithCancel(context.Background())

    // Start worker
    done := make(chan struct{})
    go func() {
        <-ctx.Done()
        close(done)
    }()

    // Cancel after some work
    time.Sleep(50 * time.Millisecond)
    cancel()

    select {
    case <-done:
        // Success
    case <-time.After(time.Second):
        t.Fatal("worker didn't stop")
    }
}
```
