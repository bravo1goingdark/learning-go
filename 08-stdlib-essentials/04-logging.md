# 4. Logging — Structured Logging with slog

> **Goal:** Learn Go's logging packages — from basic `log` to structured logging with `slog`.

---

## Table of Contents

1. [Basic Logging (log)](#1-basic-logging-log) `[CORE]`
2. [Structured Logging (slog)](#2-structured-logging-slog) `[CORE]`
3. [Log Levels](#3-log-levels) `[CORE]`
4. [Custom Logger](#4-custom-logger) `[PRODUCTION]`
5. [Best Practices](#5-best-practices) `[CORE]`
6. [Common Pitfalls](#6-common-pitfalls) `[CORE]`

---

## 1. Basic Logging (log)

### Simple Logger

```go
package main

import "log"

func main() {
    log.Println("Server starting")
    log.Printf("Listening on port %d", 8080)
    log.Fatal("Something went wrong")  // Prints + exits with code 1
}
```

### Log Output

```go
// Default: 2024/01/15 10:30:00 Server starting

// Customize flags
log.SetFlags(log.LstdFlags | log.Lshortfile)
// Output: 2024/01/15 10:30:00 main.go:5: Server starting
```

### Log Flags

| Flag | Effect |
|------|--------|
| `log.Ldate` | Date: 2024/01/15 |
| `log.Ltime` | Time: 10:30:00 |
| `log.LstdFlags` | Date + Time (default) |
| `log.Lshortfile` | File:line (main.go:5) |
| `log.Llongfile` | Full path (/app/main.go:5) |
| `log.Lmicroseconds` | Microsecond precision |

### Write to File

```go
f, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
if err != nil {
    log.Fatal(err)
}
defer f.Close()

log.SetOutput(f)
log.Println("This goes to the file")
```

---

## 2. Structured Logging (slog)

`slog` (Go 1.21+) provides structured, machine-parseable logging.

### Basic Usage

```go
package main

import "log/slog"

func main() {
    slog.Info("server starting", "port", 8080)
    slog.Error("connection failed", "host", "db.example.com", "error", "timeout")
}
```

### Output (JSON)

```json
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"server starting","port":8080}
{"time":"2024-01-15T10:30:01Z","level":"ERROR","msg":"connection failed","host":"db.example.com","error":"timeout"}
```

### Output (Text)

```
time=2024-01-15T10:30:00.000Z level=INFO msg="server starting" port=8080
time=2024-01-15T10:30:01.000Z level=ERROR msg="connection failed" host=db.example.com error=timeout
```

### Key-Value Pairs

```go
// String
slog.Info("user created", "name", "Alice")

// Integer
slog.Info("request processed", "count", 42)

// Float
slog.Info("latency", "ms", 123.45)

// Bool
slog.Info("cache hit", "hit", true)

// Error
slog.Error("failed", "error", err)

// Multiple pairs
slog.Info("request",
    "method", "GET",
    "path", "/api/users",
    "status", 200,
    "duration", "45ms",
)
```

### Using slog.Attr

```go
import "log/slog"

slog.Info("user",
    slog.String("name", "Alice"),
    slog.Int("age", 30),
    slog.Bool("active", true),
    slog.Any("tags", []string{"admin", "user"}),
)
```

---

## 3. Log Levels

### Level Hierarchy

```
DEBUG < INFO < WARN < ERROR
```

### Setting Minimum Level

```go
// Only show INFO and above
slog.SetLogLoggerLevel(slog.LevelInfo)

// Show everything (DEBUG and above)
slog.SetLogLoggerLevel(slog.LevelDebug)
```

### Level Functions

```go
slog.Debug("debugging info", "var", value)      // DEBUG level
slog.Info("normal operation", "key", "value")    // INFO level
slog.Warn("potential issue", "key", "value")     // WARN level
slog.Error("something failed", "error", err)     // ERROR level
```

### Custom Level

```go
// Create a custom logger with specific level
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

logger.Debug("this will be logged")
logger.Info("this will be logged")
```

### Context-Aware Logging

```go
// Add to context
ctx := context.WithValue(context.Background(), "request_id", "abc-123")

// Log with context
slog.InfoContext(ctx, "processing request", "user_id", 42)
```

---

## 4. Custom Logger

> ⏭️ **First pass? Skip this section.** Come back after completing projects.

### JSON Logger (Production)

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

slog.SetDefault(logger)

// All logs now output as JSON
slog.Info("server started", "port", 8080)
```

### Logger with Attributes

```go
// Create logger with common fields
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).
    With(
        "service", "api-server",
        "version", "1.0.0",
        "env", "production",
    )

// All logs include these fields
logger.Info("request received", "method", "GET", "path", "/users")
// Output includes: service=api-server, version=1.0.0, env=production
```

### Per-Request Logger

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Create request-scoped logger
    reqLogger := slog.With(
        "method", r.Method,
        "path", r.URL.Path,
        "request_id", r.Header.Get("X-Request-ID"),
        "remote_addr", r.RemoteAddr,
    )

    reqLogger.Info("request started")

    // Process request...

    reqLogger.Info("request completed", "status", 200, "duration", "45ms")
}
```

### Multi-Handler (Console + File)

```go
type MultiHandler struct {
    handlers []slog.Handler
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return true
}

func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
    for _, h := range m.handlers {
        if err := h.Handle(ctx, r); err != nil {
            return err
        }
    }
    return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    handlers := make([]slog.Handler, len(m.handlers))
    for i, h := range m.handlers {
        handlers[i] = h.WithAttrs(attrs)
    }
    return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
    handlers := make([]slog.Handler, len(m.handlers))
    for i, h := range m.handlers {
        handlers[i] = h.WithGroup(name)
    }
    return &MultiHandler{handlers: handlers}
}

// Usage
file, _ := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
defer file.Close()

handler := &MultiHandler{
    handlers: []slog.Handler{
        slog.NewJSONHandler(os.Stdout, nil),                          // Console
        slog.NewJSONHandler(file, &slog.HandlerOptions{Level: slog.LevelInfo}),  // File
    },
}

logger := slog.New(handler)
logger.Info("logged to both stdout and file")
```

---

## 5. Best Practices

### 1. Use Structured Logging

```go
// BAD — unstructured
log.Printf("User %s logged in from %s", user, ip)

// GOOD — structured
slog.Info("user logged in", "user", user, "ip", ip)
```

### 2. Include Context

```go
// BAD — no context
slog.Error("failed")

// GOOD — actionable
slog.Error("failed to create user",
    "user_id", userID,
    "email", email,
    "error", err,
    "retry_count", retryCount,
)
```

### 3. Use Appropriate Levels

```go
slog.Debug("cache miss", "key", key)        // Internal details
slog.Info("user created", "id", userID)     // Normal operation
slog.Warn("slow query", "duration", "2s")   // Concerning but not failing
slog.Error("db connection failed", "error", err)  // Something broke
```

### 4. Don't Log Sensitive Data

```go
// BAD
slog.Info("user login", "password", password)

// GOOD
slog.Info("user login", "user", username)
```

### 5. Log Entry and Exit Points

```go
func processOrder(orderID string) error {
    slog.Info("processing order", "order_id", orderID)

    // ... business logic ...

    slog.Info("order processed", "order_id", orderID, "status", "completed")
    return nil
}
```

---

## 6. Common Pitfalls

### 1. Using fmt.Println for Logging

```go
// BAD — no timestamps, no levels, no structure
fmt.Println("Something happened")

// GOOD
slog.Info("something happened")
```

### 2. Logging and Returning Error

```go
// BAD — logs twice if caller also logs
func doSomething() error {
    err := something()
    if err != nil {
        slog.Error("failed", "error", err)
        return err  // Caller might log again!
    }
    return nil
}

// GOOD — decide: log OR return, not both
func doSomething() error {
    return something()  // Let caller decide whether to log
}
```

### 3. Not Handling slog in Libraries

```go
// In a library, accept a logger instead of using the default
type Service struct {
    logger *slog.Logger
}

func NewService(logger *slog.Logger) *Service {
    if logger == nil {
        logger = slog.Default()
    }
    return &Service{logger: logger}
}
```

---

## Quick Reference

```go
// Basic log
log.Println("message")
log.Printf("format %v", value)
log.Fatal("error + exit")   // os.Exit(1)
log.Panic("error + panic")  // panics

// slog
slog.Debug("msg", "key", value)
slog.Info("msg", "key", value)
slog.Warn("msg", "key", value)
slog.Error("msg", "key", value)
slog.Log(ctx, level, "msg", "key", value)

// slog attributes
slog.String("key", "value")
slog.Int("key", 42)
slog.Bool("key", true)
slog.Any("key", complexValue)

// Logger with fields
logger := slog.With("service", "api")
logger.Info("msg", "key", value)
```

---

## Exercises

### Exercise 1: Basic Structured Logging ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a program that logs user actions with structured fields.

<details>
<summary>Solution</summary>

```go
package main

import (
	"log/slog"
	"os"
)

func main() {
	// Set up JSON handler
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Log user actions
	slog.Info("user login", "user", "alice", "ip", "192.168.1.1")
	slog.Info("page view", "user", "alice", "page", "/dashboard")
	slog.Warn("failed login attempt", "user", "bob", "reason", "wrong password")
	slog.Error("payment failed", "user", "alice", "amount", 99.99, "error", "card declined")
}
```

</details>

### Exercise 2: Custom Logger with Fields ⭐⭐
**Difficulty:** Beginner | **Time:** ~15 min

Create a logger that includes service name and environment in every log message.

<details>
<summary>Solution</summary>

```go
package main

import (
	"log/slog"
	"os"
)

func main() {
	// Create logger with common fields
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).
		With(
			"service", "user-api",
			"env", "production",
			"version", "1.2.3",
		)

	// All logs include service, env, and version
	logger.Info("server started", "port", 8080)
	logger.Info("user created", "user_id", 42, "name", "Alice")
	logger.Error("database error", "error", "connection timeout")
}
```

</details>

---

## Next: [Progressive Exercises →](../projects/21-progressive-exercises.md)
