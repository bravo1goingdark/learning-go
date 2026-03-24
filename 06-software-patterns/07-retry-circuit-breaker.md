# Retry + Circuit Breaker

> Handle transient failures gracefully. Protect against cascade failures.

---

## The Problem

Network calls fail. Sometimes temporarily:

```
┌─────────────────────────────────────────────────────────────┐
│                    Failure Scenarios                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  • Database temporarily unavailable                        │
│  • Network timeout                                         │
│  • External API rate limited                               │
│  • Service momentarily overloaded                          │
│                                                             │
│  Solution: RETRY after brief delay                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

But naive retry can make things worse:

```
┌─────────────────────────────────────────────────────────────┐
│                    Retry Storm                              │
│                                                             │
│  Request ──✗──▶ Service (overloaded)                       │
│       │                                                     │
│       │ ✗ ✗ ✗ ✗ ✗ ✗ ✗ ✗ ✗ ✗ ... (spamming)                   │
│       ▼                                                     │
│  Service crashes completely (cascade failure!)             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Retry Pattern

### Simple Retry

```go
func retry[T any](fn func() (T, error), maxAttempts int, delay time.Duration) (T, error) {
    var lastErr error

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        result, err := fn()
        if err == nil {
            return result, nil
        }

        lastErr = err

        // Don't sleep on last attempt
        if attempt < maxAttempts {
            time.Sleep(delay * time.Duration(attempt)) // Linear backoff
        }
    }

    return *new(T), lastErr
}
```

### With Exponential Backoff

```go
func withExponentialBackoff[T any](
    fn func() (T, error),
    maxAttempts int,
    initialDelay time.Duration,
    maxDelay time.Duration,
) (T, error) {
    var lastErr error
    delay := initialDelay

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        result, err := fn()
        if err == nil {
            return result, nil
        }

        lastErr = err

        if attempt < maxAttempts {
            time.Sleep(delay)
            delay = delay * 2
            if delay > maxDelay {
                delay = maxDelay
            }
        }
    }

    return *new(T), lastErr
}
```

### With Jitter (Recommended)

Prevents thundering herd when many clients retry at once:

```go
func withJitter[T any](
    fn func() (T, error),
    maxAttempts int,
    baseDelay time.Duration,
) (T, error) {
    var lastErr error

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        result, err := fn()
        if err == nil {
            return result, nil
        }

        lastErr = err

        if attempt < maxAttempts {
            // Random delay between 0.5x and 1.5x base
            jitter := rand.Int63n(int64(baseDelay)) + int64(baseDelay/2)
            time.Sleep(time.Duration(jitter))
        }
    }

    return *new(T), lastErr
}
```

---

## Circuit Breaker Pattern

Circuit breaker prevents cascade failures:

```
┌─────────────────────────────────────────────────────────────┐
│                  Circuit Breaker States                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   CLOSED (Normal)                                           │
│   ────────────                                              │
│   Requests pass through. Failures counted.                 │
│                                                             │
│       ▼ (too many failures)                                 │
│                                                             │
│   OPEN (Protected)                                          │
│   ────────────                                             │
│   Requests fail fast. Don't overload service.             │
│                                                             │
│       ▼ (timeout passes)                                    │
│                                                             │
│   HALF-OPEN (Testing)                                       │
│   ─────────────────                                         │
│   Allow limited requests to test recovery.                 │
│                                                             │
│       ▼ (succeeds)                                          │
│                                                             │
│   CLOSED (Normal)                                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Circuit Breaker Implementation

```go
// internal/resilience/circuitbreaker.go
package resilience

import (
    "errors"
    "sync"
    "time"
)

type CircuitState int

const (
    StateClosed CircuitState = iota
    StateOpen
    StateHalfOpen
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type CircuitBreaker struct {
    mu             sync.Mutex
    state          CircuitState

    // Configuration
    failureThreshold int
    successThreshold int
    timeout          time.Duration

    // State tracking
    failures        int
    successes       int
    lastFailureTime time.Time
}

func NewCircuitBreaker(failureThreshold int, successThreshold int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        failureThreshold: failureThreshold,
        successThreshold: successThreshold,
        timeout:          timeout,
        state:           StateClosed,
    }
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    if !cb.allowRequest() {
        return ErrCircuitOpen
    }

    err := fn()

    if err != nil {
        cb.recordFailure()
    } else {
        cb.recordSuccess()
    }

    return err
}

func (cb *CircuitBreaker) allowRequest() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    switch cb.state {
    case StateClosed:
        return true

    case StateOpen:
        // Check if timeout has passed
        if time.Since(cb.lastFailureTime) > cb.timeout {
            cb.state = StateHalfOpen
            cb.successes = 0
            return true
        }
        return false

    case StateHalfOpen:
        return true // Allow one request to test

    default:
        return true
    }
}

func (cb *CircuitBreaker) recordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.lastFailureTime = time.Now()
    cb.failures++

    if cb.state == StateHalfOpen {
        cb.state = StateOpen // Failed during test, go back to open
    } else if cb.failures >= cb.failureThreshold {
        cb.state = StateOpen
    }
}

func (cb *CircuitBreaker) recordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.successes++

    if cb.state == StateHalfOpen {
        if cb.successes >= cb.successThreshold {
            cb.state = StateClosed
            cb.failures = 0
        }
    } else {
        cb.failures = 0 // Reset on success in closed state
    }
}
```

---

## Using Retry + Circuit Breaker Together

```go
type HTTPClient struct {
    cb *resilience.CircuitBreaker
}

func NewHTTPClient() *HTTPClient {
    return &HTTPClient{
        cb: resilience.NewCircuitBreaker(
            5,           // failure threshold
            3,           // success threshold to close
            30*time.Second, // timeout before trying again
        ),
    }
}

func (c *HTTPClient) Get(url string) ([]byte, error) {
    return c.cb.Execute(func() error {
        return withRetry(func() error {
            // Actual HTTP call here
            resp, err := http.Get(url)
            if err != nil {
                return err
            }
            defer resp.Body.Close()

            if resp.StatusCode >= 500 {
                return errors.New("server error")
            }
            return nil
        }, 3, 100*time.Millisecond)
    })
}
```

---

## Go Libraries

For production, use well-tested libraries:

### go-retryablehttp

```go
client := retryablehttp.NewClient()
client.RetryMax = 3
client.RetryWaitMin = 100 * time.Millisecond
client.RetryWaitMax = 30 * time.Second

resp, err := client.Get("http://example.com")
```

### hystrix (from Netflix)

```go
hystrix.ConfigureCommand("my-service", hystrix.CommandConfig{
    Timeout:               1000,
    MaxConcurrentRequests: 10,
    ErrorPercentThreshold: 25,
})

err := hystrix.Do("my-service", func() error {
    // call service
}, nil)
```

---

## Quick Reference

| Pattern | Purpose | Use When |
|---------|---------|----------|
| Retry | Handle transient failures | Network calls, DB |
| Exponential backoff | Prevent retry storms | Multiple clients |
| Jitter | Randomize retry timing | High traffic |
| Circuit breaker | Prevent cascade failures | External services |

---

## Common Pitfalls

1. **No retry limit** - Infinite retries crash your service
2. **Retry non-idempotent operations** - Payments twice!
3. **No circuit breaker** - Overloaded downstream crashes
4. **Ignore partial failures** - Some requests may partially succeed

---

## Next Steps

- [Backpressure Strategies](13-backpressure-strategies.md) - Handle system overload
- [Milestone Project](20-layered-http-service.md) - Build resilient service