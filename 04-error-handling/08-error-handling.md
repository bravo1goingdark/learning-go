# 8. Error Handling — Complete Deep Dive

> **Goal:** Master Go's unique approach to errors. No exceptions, no try/catch. Errors are values.

![Error Handling](../assets/08.png)

---

## Table of Contents

1. [The `error` Interface](#1-the-error-interface)
2. [Creating Errors](#2-creating-errors)
3. [Error Checking Patterns](#3-error-checking-patterns)
4. [Error Wrapping (Go 1.13+)](#4-error-wrapping-go-113)
5. [Sentinel Errors](#5-sentinel-errors)
6. [Custom Error Types](#6-custom-error-types)
7. [Error Inspection (`errors.Is`, `errors.As`)](#7-error-inspection-errorsis-errorsas)
8. [Panic & Recover](#8-panic--recover)
9. [Production Error Handling Patterns](#9-production-error-handling-patterns)
10. [Structured Errors](#10-structured-errors)
11. [Common Pitfalls](#11-common-pitfalls)

---

## 1. The `error` Interface

```go
type error interface {
    Error() string
}
```

That's it. One method. Any type with `Error() string` is an error.

### The Simplest Error

```go
type MyError struct {
    Msg string
}

func (e *MyError) Error() string {
    return e.Msg
}
```

### Errors as Return Values

```go
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}

func main() {
    result, err := divide(10, 0)
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Println("Result:", result)
}
```

---

## 2. Creating Errors

### `errors.New`

```go
import "errors"

var ErrNotFound = errors.New("not found")
var ErrTimeout = errors.New("timeout")

func findUser(id string) (*User, error) {
    user, ok := db[id]
    if !ok {
        return nil, ErrNotFound
    }
    return user, nil
}
```

### `fmt.Errorf`

```go
func getUser(id string) (*User, error) {
    user, err := db.Find(id)
    if err != nil {
        return nil, fmt.Errorf("getUser(%s): %w", id, err)
        //                                ^ wrapping verb
    }
    return user, nil
}
```

### Custom Error Struct

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

func validate(name string) error {
    if name == "" {
        return &ValidationError{
            Field:   "name",
            Message: "cannot be empty",
        }
    }
    return nil
}
```

---

## 3. Error Checking Patterns

### Basic Check

```go
result, err := doSomething()
if err != nil {
    return err  // Propagate up
}
```

### Early Return

```go
func process() error {
    if err := step1(); err != nil {
        return fmt.Errorf("step1: %w", err)
    }
    if err := step2(); err != nil {
        return fmt.Errorf("step2: %w", err)
    }
    if err := step3(); err != nil {
        return fmt.Errorf("step3: %w", err)
    }
    return nil
}
```

### Log and Return

```go
func handler(w http.ResponseWriter, r *http.Request) {
    user, err := getUser(r.Context(), userID)
    if err != nil {
        slog.Error("failed to get user", "error", err, "userID", userID)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return  // Return to caller, don't continue
    }
    // ...
}
```

### Retry Pattern

```go
func retry(attempts int, delay time.Duration, fn func() error) error {
    var err error
    for i := 0; i < attempts; i++ {
        if err = fn(); err == nil {
            return nil
        }
        if i < attempts-1 {
            time.Sleep(delay)
        }
    }
    return fmt.Errorf("after %d attempts, last error: %w", attempts, err)
}
```

### Ignore Error (Rare, Use Sparingly)

```go
result, _ := doSomething()  // Explicitly ignoring error

// Better: comment why you're ignoring
result, _ := doSomething()  // Error is safe to ignore: best-effort operation
```

---

## 4. Error Wrapping (Go 1.13+)

### `%w` Verb

```go
func readConfig(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("readConfig(%s): %w", path, err)
    }
    return data, nil
}

func loadApp() error {
    data, err := readConfig("config.yaml")
    if err != nil {
        return fmt.Errorf("loadApp: %w", err)
    }
    // ...
    return nil
}
```

### Error Chain

```
loadApp: readConfig(config.yaml): open config.yaml: no such file or directory
```

Each `%w` creates a link in the chain.

### Unwrapping

```go
err := loadApp()

// errors.Unwrap returns the wrapped error
cause := errors.Unwrap(err)
fmt.Println(cause)  // "readConfig(config.yaml): open config.yaml: no such file or directory"

// Keep unwrapping
cause2 := errors.Unwrap(cause)
fmt.Println(cause2)  // "open config.yaml: no such file or directory"
```

### `%w` vs `%v`

```go
// %w — wraps error (can be unwrapped with errors.Is/As)
return fmt.Errorf("operation failed: %w", err)

// %v — just formats error message (cannot be unwrapped)
return fmt.Errorf("operation failed: %v", err)
```

**Always use `%w` unless you intentionally want to hide the underlying error.**

### Multiple Error Wrapping (Go 1.20+)

```go
import "errors"

func doMultiple() error {
    var errs []error
    if err := step1(); err != nil {
        errs = append(errs, fmt.Errorf("step1: %w", err))
    }
    if err := step2(); err != nil {
        errs = append(errs, fmt.Errorf("step2: %w", err))
    }
    return errors.Join(errs...)  // Combines multiple errors
}
```

---

## 5. Sentinel Errors

Pre-defined errors used for comparison.

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrTimeout      = errors.New("timeout")
    ErrConflict     = errors.New("conflict")
)
```

### Usage

```go
func FindUser(id string) (*User, error) {
    user, ok := db[id]
    if !ok {
        return nil, ErrNotFound
    }
    return user, nil
}

func HandleRequest(id string) {
    user, err := FindUser(id)
    if errors.Is(err, ErrNotFound) {
        http.Error(w, "not found", 404)
        return
    }
    if err != nil {
        http.Error(w, "internal error", 500)
        return
    }
    // ...
}
```

### Naming Convention

```go
// Export sentinel errors as ErrXxx
var ErrNotFound = errors.New("not found")

// NOT: var NotFound = errors.New("not found")
// NOT: var ERROR_NOT_FOUND = errors.New("not found")
```

### When to Use Sentinel Errors

**Good for:**
- Well-known, fixed error conditions
- Errors that callers need to check specifically
- API contracts

**Bad for:**
- Errors with dynamic context (use custom error types)
- Internal errors (use wrapping instead)

---

## 6. Custom Error Types

### Struct Error

```go
type NotFoundError struct {
    Resource string
    ID       string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s with ID %s not found", e.Resource, e.ID)
}

func FindUser(id string) (*User, error) {
    user, ok := db[id]
    if !ok {
        return nil, &NotFoundError{Resource: "user", ID: id}
    }
    return user, nil
}
```

### Error with Status Code

```go
type HTTPError struct {
    StatusCode int
    Message    string
    Err        error  // Wrapped error
}

func (e *HTTPError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Err)
    }
    return e.Message
}

func (e *HTTPError) Unwrap() error {
    return e.Err
}

// Usage
func proxyRequest(url string) error {
    resp, err := http.Get(url)
    if err != nil {
        return &HTTPError{
            StatusCode: 502,
            Message:    "upstream request failed",
            Err:        err,
        }
    }
    if resp.StatusCode >= 400 {
        return &HTTPError{
            StatusCode: resp.StatusCode,
            Message:    fmt.Sprintf("upstream returned %d", resp.StatusCode),
        }
    }
    return nil
}
```

### Multi-Field Error

```go
type ValidationError struct {
    Field   string
    Value   any
    Message string
    Err     error
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s=%v: %s", e.Field, e.Value, e.Message)
}

func (e *ValidationError) Unwrap() error {
    return e.Err
}

func validateAge(age int) error {
    if age < 0 {
        return &ValidationError{
            Field:   "age",
            Value:   age,
            Message: "must be non-negative",
        }
    }
    if age > 150 {
        return &ValidationError{
            Field:   "age",
            Value:   age,
            Message: "must be less than 150",
        }
    }
    return nil
}
```

---

## 7. Error Inspection (`errors.Is`, `errors.As`)

### `errors.Is` — Check Equality

```go
var ErrNotFound = errors.New("not found")

func FindUser(id string) (*User, error) {
    return nil, ErrNotFound
}

func main() {
    _, err := FindUser("123")
    if errors.Is(err, ErrNotFound) {
        fmt.Println("User not found")
    }
}
```

`errors.Is` walks the error chain using `Unwrap()`.

### `errors.Is` with Wrapped Errors

```go
func FindUser(id string) (*User, error) {
    err := db.Find(id)
    if err != nil {
        return nil, fmt.Errorf("FindUser: %w", err)  // Wraps ErrNotFound
    }
    return user, nil
}

func main() {
    _, err := FindUser("123")
    // err is: "FindUser: not found"
    if errors.Is(err, ErrNotFound) {
        fmt.Println("User not found")  // TRUE — Is finds it in the chain
    }
}
```

### `errors.As` — Extract Specific Type

```go
func handle(err error) {
    var httpErr *HTTPError
    if errors.As(err, &httpErr) {
        // httpErr is the *HTTPError from the chain
        fmt.Printf("HTTP %d: %s\n", httpErr.StatusCode, httpErr.Message)
        return
    }
    
    var valErr *ValidationError
    if errors.As(err, &valErr) {
        fmt.Printf("Validation failed on %s: %s\n", valErr.Field, valErr.Message)
        return
    }
    
    fmt.Println("Unknown error:", err)
}
```

### `errors.As` with Pointer Target

```go
// WRONG — target must be a pointer
var httpErr HTTPError
errors.As(err, &httpErr)  // Does not compile as expected

// RIGHT — target is pointer to pointer
var httpErr *HTTPError
errors.As(err, &httpErr)  // Correct
```

### Comparison: `Is` vs `As`

| Function | Use Case | Target |
|----------|----------|--------|
| `errors.Is(err, target)` | Check if error equals a specific value | Sentinel error (`var ErrX = errors.New(...)`) |
| `errors.As(err, &target)` | Extract error of a specific type | Pointer to error type (`*MyError`) |

---

## 8. Panic & Recover

### Panic

```go
func dangerous() {
    panic("something went terribly wrong")
}

func main() {
    dangerous()
    fmt.Println("This never prints")
}
// Output:
// panic: something went terribly wrong
// goroutine 1 [running]:
// ...
```

### Recover

```go
func safeCall(fn func()) {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("Recovered:", r)
        }
    }()
    fn()
}

func main() {
    safeCall(func() {
        panic("oh no")
    })
    fmt.Println("Continues after recover")  // Prints!
}
```

### When to Use Panic

**Almost never in production code.** Use panic only for:

1. **Truly unrecoverable errors** — programmer mistakes
   ```go
   // Missing required configuration
   if config.Port == 0 {
       panic("PORT must be set")
   }
   ```

2. **Invariant violations** — should never happen
   ```go
   func (t Tree) Value() int {
       if t == nil {
           panic("nil tree")
       }
       return t.value
   }
   ```

3. **Library initialization failures**
   ```go
   var templates = template.Must(template.ParseGlob("templates/*.html"))
   // panics if templates can't be parsed
   ```

### When NOT to Use Panic

- User input validation
- Network errors
- File I/O errors
- Expected failure conditions

**Rule:** If you expect it might fail, return an error. If it should NEVER fail, panic.

### `template.Must` Pattern

```go
// Must wraps a function that returns (T, error) and panics on error
func Must[T any](val T, err error) T {
    if err != nil {
        panic(err)
    }
    return val
}

// Usage
var db = Must(sql.Open("postgres", connString))
var tmpl = Must(template.New("main").Parse(templateStr))
```

---

## 9. Production Error Handling Patterns

### Pattern 1: Wrap with Context

```go
func (s *Service) CreateUser(ctx context.Context, req CreateUserReq) (*User, error) {
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("CreateUser.Validate: %w", err)
    }
    
    user := &User{Name: req.Name, Email: req.Email}
    if err := s.repo.Save(ctx, user); err != nil {
        return nil, fmt.Errorf("CreateUser.Save: %w", err)
    }
    
    if err := s.mailer.SendWelcome(ctx, user); err != nil {
        // Log but don't fail — best effort
        s.logger.Error("failed to send welcome email", "error", err, "userID", user.ID)
    }
    
    return user, nil
}
```

### Pattern 2: Error Accumulation

```go
type MultiError struct {
    errs []error
}

func (e *MultiError) Error() string {
    msgs := make([]string, len(e.errs))
    for i, err := range e.errs {
        msgs[i] = err.Error()
    }
    return strings.Join(msgs, "; ")
}

func (e *MultiError) Add(err error) {
    if err != nil {
        e.errs = append(e.errs, err)
    }
}

func (e *MultiError) Err() error {
    if len(e.errs) == 0 {
        return nil
    }
    return e
}

// Or use Go 1.20+ errors.Join
func validate(req Request) error {
    var errs []error
    if req.Name == "" {
        errs = append(errs, fmt.Errorf("name is required"))
    }
    if req.Email == "" {
        errs = append(errs, fmt.Errorf("email is required"))
    }
    return errors.Join(errs...)
}
```

### Pattern 3: Typed HTTP Errors

```go
type AppError struct {
    Code       int    `json:"code"`
    Message    string `json:"message"`
    Details    any    `json:"details,omitempty"`
    Err        error  `json:"-"`
}

func (e *AppError) Error() string {
    return e.Message
}

func (e *AppError) Unwrap() error {
    return e.Err
}

var (
    ErrNotFound     = &AppError{Code: 404, Message: "not found"}
    ErrUnauthorized = &AppError{Code: 401, Message: "unauthorized"}
    ErrBadRequest   = &AppError{Code: 400, Message: "bad request"}
)

func (s *Service) GetUser(id string) (*User, error) {
    user, ok := s.db[id]
    if !ok {
        return nil, fmt.Errorf("GetUser(%s): %w", id, ErrNotFound)
    }
    return user, nil
}

// Handler
func handleError(w http.ResponseWriter, err error) {
    var appErr *AppError
    if errors.As(err, &appErr) {
        w.WriteHeader(appErr.Code)
        json.NewEncoder(w).Encode(appErr)
        return
    }
    w.WriteHeader(500)
    json.NewEncoder(w).Encode(map[string]string{
        "message": "internal server error",
    })
}
```

### Pattern 4: Domain Errors

```go
type DomainError struct {
    Op      string // Operation that failed
    Code    string // Machine-readable code
    Message string // Human-readable message
    Err     error  // Underlying error
}

func (e *DomainError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %s: %v", e.Op, e.Code, e.Err)
    }
    return fmt.Sprintf("%s: %s: %s", e.Op, e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
    return e.Err
}

// Usage
func (s *OrderService) Cancel(orderID string) error {
    order, err := s.repo.Find(orderID)
    if err != nil {
        return &DomainError{
            Op:      "OrderService.Cancel",
            Code:    "ORDER_NOT_FOUND",
            Message: fmt.Sprintf("order %s not found", orderID),
            Err:     err,
        }
    }
    
    if order.Status == StatusShipped {
        return &DomainError{
            Op:      "OrderService.Cancel",
            Code:    "ORDER_ALREADY_SHIPPED",
            Message: "cannot cancel shipped order",
        }
    }
    
    // ...
    return nil
}
```

---

## 10. Structured Errors (With `slog`)

```go
import "log/slog"

func (s *Service) Process(ctx context.Context, id string) error {
    item, err := s.repo.Find(ctx, id)
    if err != nil {
        // Log with structured context
        slog.ErrorContext(ctx, "failed to find item",
            "error", err,
            "id", id,
            "operation", "Process",
        )
        return fmt.Errorf("Process: %w", err)
    }
    
    if err := s.validate(item); err != nil {
        slog.WarnContext(ctx, "validation failed",
            "error", err,
            "id", id,
            "item_type", item.Type,
        )
        return fmt.Errorf("Process.validate: %w", err)
    }
    
    return nil
}
```

---

## 11. Common Pitfalls

### 1. Ignoring Errors

```go
// WRONG
f, _ := os.Open("file.txt")  // What if it fails?
defer f.Close()               // PANIC: nil pointer

// RIGHT
f, err := os.Open("file.txt")
if err != nil {
    return fmt.Errorf("open file: %w", err)
}
defer f.Close()
```

### 2. Logging AND Returning

```go
// WRONG — logs and returns, caller logs again = duplicate logs
func findUser(id string) (*User, error) {
    user, err := db.Find(id)
    if err != nil {
        log.Println("error finding user:", err)  // Log here
        return nil, err                           // AND return
    }
    return user, nil
}

// RIGHT — return with context, let caller decide to log
func findUser(id string) (*User, error) {
    user, err := db.Find(id)
    if err != nil {
        return nil, fmt.Errorf("findUser(%s): %w", id, err)
    }
    return user, nil
}
```

### 3. Swallowing Errors

```go
// WRONG
func process() {
    err := doSomething()
    // Error silently ignored
}

// RIGHT
func process() error {
    return doSomething()
}
```

### 4. Using `panic` for Expected Errors

```go
// WRONG
func divide(a, b float64) float64 {
    if b == 0 {
        panic("division by zero")  // This is expected, not a panic
    }
    return a / b
}

// RIGHT
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}
```

### 5. Not Wrapping Errors

```go
// WRONG — loses context
func readConfig(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err  // Caller doesn't know which operation failed
    }
    return data, nil
}

// RIGHT — adds context
func readConfig(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("readConfig(%s): %w", path, err)
    }
    return data, nil
}
```

### 6. Nil Interface Holding Nil Pointer

```go
// BUG
func doWork() error {
    var err *MyError  // nil
    return err        // Non-nil interface!
}

// FIX
func doWork() error {
    return nil
}
```

---

## Quick Reference

```go
// Creating errors
err := errors.New("message")
err := fmt.Errorf("operation %s failed: %w", name, originalErr)
err := &ValidationError{Field: "name", Message: "required"}

// Checking errors
if err != nil { return err }
if errors.Is(err, ErrNotFound) { ... }
if errors.As(err, &targetErr) { ... }

// Wrapping errors
return fmt.Errorf("context: %w", err)

// Unwrapping
cause := errors.Unwrap(err)

// Multiple errors
return errors.Join(err1, err2, err3)

// Panic/Recover
panic("unrecoverable")
if r := recover(); r != nil { ... }

// Sentinel errors
var ErrNotFound = errors.New("not found")
```

---

## 12. Production Patterns

### Error Handling in HTTP Handlers

```go
type APIError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Err     error  `json:"-"`
}

func (e *APIError) Error() string {
    return e.Message
}

func (e *APIError) Unwrap() error {
    return e.Err
}

func JSONError(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func HandleError(w http.ResponseWriter, err error) {
    var apiErr *APIError
    if errors.As(err, &apiErr) {
        JSONError(w, apiErr.Code, apiErr.Message)
        return
    }

    // Default 500
    JSONError(w, 500, "internal server error")
}

func handler(w http.ResponseWriter, r *http.Request) {
    err := doWork(r.Context())
    if err != nil {
        HandleError(w, err)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

### Error Handling with Structured Logging

```go
import "github.com/rs/zerolog"

type Service struct {
    log zerolog.Logger
}

func (s *Service) Process(ctx context.Context, id string) error {
    data, err := s.fetch(ctx, id)
    if err != nil {
        s.log.Error().
            Str("id", id).
            Str("error", err.Error()).
            Str("function", "Process").
            Msg("failed to fetch")
        return fmt.Errorf("fetch %s: %w", id, err)
    }

    result, err := s.transform(data)
    if err != nil {
        s.log.Error().
            Str("id", id).
            Str("error", err.Error()).
            Str("function", "transform").
            Msg("failed to transform")
        return fmt.Errorf("transform: %w", err)
    }

    return nil
}
```

### Retry with Backoff

```go
func retry(ctx context.Context, maxRetries int, fn func() error) error {
    var lastErr error
    for attempt := 0; attempt < maxRetries; attempt++ {
        if err := fn(); err != nil {
            lastErr = err

            // Don't retry on non-retryable errors
            if !isRetryable(err) {
                return err
            }

            // Check context
            if ctx.Err() != nil {
                return ctx.Err()
            }

            // Exponential backoff
            delay := time.Duration(attempt+1) * time.Second
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(delay):
                continue
            }
        } else {
            return nil
        }
    }
    return fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}

func isRetryable(err error) bool {
    var netErr net.Error
    if errors.As(err, &netErr) {
        return netErr.Timeout() || netErr.Temporary()
    }
    return false
}
```

### Error Handling in Goroutines

```go
func parallelWork(ctx context.Context, tasks []Task) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    errCh := make(chan error, len(tasks))
    var wg sync.WaitGroup

    for _, task := range tasks {
        wg.Add(1)
        go func(t Task) {
            defer wg.Done()
            if err := t.Execute(ctx); err != nil {
                select {
                case errCh <- err:
                default:
                }
                cancel() // Cancel remaining
            }
        }(task)
    }

    wg.Wait()
    close(errCh)

    // Return first error or nil
    for err := range errCh {
        return err
    }
    return nil
}
```

---

## 13. Testing Errors

```go
func TestErrorTypes(t *testing.T) {
    err := doThing()
    
    // Test with errors.Is
    if !errors.Is(err, ErrNotFound) {
        t.Errorf("expected ErrNotFound")
    }

    // Test with errors.As
    var validationErr *ValidationError
    if errors.As(err, &validationErr) {
        if validationErr.Field != "email" {
            t.Errorf("expected field 'email', got %s", validationErr.Field)
        }
    }
}

func TestErrorWrap(t *testing.T) {
    original := errors.New("original")
    wrapped := fmt.Errorf("wrapped: %w", original)

    if !errors.Is(wrapped, original) {
        t.Error("wrapped error should contain original")
    }
}

func TestPanicRecovery(t *testing.T) {
    defer func() {
        if r := recover(); r != nil {
            err, ok := r.(error)
            if !ok {
                t.Errorf("expected error, got %v", r)
            }
            if !errors.Is(err, ErrPanic) {
                t.Errorf("expected ErrPanic, got %v", err)
            }
        }
    }()

    // Function that panics
    doPanic()
}

func doPanic() {
    defer panic(ErrPanic)
    // ...
}
```

---

## 14. Error Handling Best Practices

### Don't Ignore Errors

```go
// BAD
func bad() {
    json.Unmarshal(data, &obj) // Ignoring error!
}

// GOOD
func good() error {
    return json.Unmarshal(data, &obj)
}

// If you must ignore
func ignore() {
    _ = json.Unmarshal(data, &obj) // Explicit ignore
}
```

### Error Wrapping Guidelines

```go
// BAD: Lost original error
return fmt.Errorf("failed")

// GOOD: Preserve original
return fmt.Errorf("fetch user: %w", err)

// GOOD: Add context
return fmt.Errorf("fetch user id=%s: %w", id, err)
```

### Sentinel vs Custom Errors

```go
// Use sentinel for package-level errors that callers check
var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")

// Use custom types for rich error information
type ValidationError struct {
    Field   string
    Message string
    Value   interface{}
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s (got %v)", e.Field, e.Message, e.Value)
}

// Using custom errors
if errors.Is(err, ErrNotFound) {
    // Handle not found
}

var ve *ValidationError
if errors.As(err, &ve) {
    fmt.Printf("validation error on field %s: %s\n", ve.Field, ve.Message)
}
```

---

## Next: [Defer In Depth →](./09-defer-in-depth.md)
