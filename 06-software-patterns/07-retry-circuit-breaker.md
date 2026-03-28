# Retry + Circuit Breaker

> Handle transient failures gracefully. Protect against cascade failures.

---

## Table of Contents

1. [The Problem](#the-problem) `[PRODUCTION]`
2. [Retry Pattern](#retry-pattern) `[PRODUCTION]`
3. [Full Retry Implementation](#full-retry-implementation) `[PRODUCTION]`
4. [Circuit Breaker Pattern](#circuit-breaker-pattern) `[PRODUCTION]`
5. [Circuit Breaker Implementation](#circuit-breaker-implementation) `[PRODUCTION]`
6. [Retry + Circuit Breaker Combined](#retry--circuit-breaker-combined) `[PRODUCTION]`
7. [When to Use Which](#when-to-use-which) `[PRODUCTION]`
8. [What to Retry (and What NOT to)](#what-to-retry-and-what-not-to) `[PRODUCTION]`
9. [Idempotency and Retries](#idempotency-and-retries) `[PRODUCTION]`
10. [Quick Reference](#quick-reference) `[PRODUCTION]`
11. [Common Pitfalls](#common-pitfalls) `[PRODUCTION]`

---

![Retry + Circuit Breaker Overview](../assets/retry_circuit.png)

---

## The Problem

Network calls fail. Services go down. Databases become unavailable.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                     COMMON FAILURE SCENARIOS                              │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │
  │   │   Database   │  │  External    │  │   Network    │                  │
  │   │  connection  │  │  API returns │  │   hiccup     │                  │
  │   │  timeout     │  │  500 error   │  │  (packet     │                  │
  │   │              │  │              │  │   loss)      │                  │
  │   └──────────────┘  └──────────────┘  └──────────────┘                  │
  │                                                                           │
  │   ┌──────────────┐  ┌──────────────┐                                    │
  │   │  Service     │  │  Rate        │                                    │
  │   │  restarted,  │  │  limited     │                                    │
  │   │  not ready   │  │  (429 Too    │                                    │
  │   │              │  │  Many Reqs)  │                                    │
  │   └──────────────┘  └──────────────┘                                    │
  │                                                                           │
  │   All TRANSIENT failures — they fix themselves with time                 │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

**Naive solution:** Just retry immediately!

**Problem:** Retrying immediately can make things WORSE:

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                          RETRY STORM                                      │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   INITIAL REQUESTS:                                                      │
  │                                                                           │
  │   Client 1 ──────✗──────►  Service (overloaded)                          │
  │   Client 2 ──────✗──────►  Service (overloaded)                          │
  │   Client 3 ──────✗──────►  Service (overloaded)                          │
  │                                                                           │
  │           │  All clients retry IMMEDIATELY at the same time:             │
  │           ▼                                                               │
  │                                                                           │
  │   IMMEDIATE RETRIES (no backoff):                                        │
  │                                                                           │
  │   Client 1 ──────✗──────►  Service (even MORE overloaded!)              │
  │   Client 2 ──────✗──────►  Service (CRASH!)                              │
  │   Client 3 ──────✗──────►  Service (DEAD)                                │
  │                                                                           │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  ⚠  CASCADING FAILURE — retries make the problem WORSE           │  │
  │   └───────────────────────────────────────────────────────────────────┘  │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Retry Pattern

### Simple Retry

Retry a failed operation up to N times:

```go
func retry[T any](fn func() (T, error), maxAttempts int, delay time.Duration) (T, error) {
    var lastErr error

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        result, err := fn()
        if err == nil {
            return result, nil
        }

        lastErr = err

        // Wait before next attempt (except last)
        if attempt < maxAttempts {
            time.Sleep(delay)
        }
    }

    var zero T
    return zero, lastErr
}
```

**Problem:** Fixed delay can cause thundering herd.

### Exponential Backoff

Double the delay after each failure:

```
Attempt 1: wait 100ms
Attempt 2: wait 200ms
Attempt 3: wait 400ms
Attempt 4: wait 800ms
Attempt 5: wait 1600ms
```

```go
func retryWithBackoff[T any](
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
            
            // Double the delay
            delay *= 2
            if delay > maxDelay {
                delay = maxDelay
            }
        }
    }

    var zero T
    return zero, lastErr
}
```

### Exponential Backoff with Jitter

Add randomness to prevent thundering herd:

```go
func retryWithJitter[T any](
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
            // Add random jitter: 50% to 150% of delay
            jitter := time.Duration(float64(delay) * (0.5 + rand.Float64()))
            time.Sleep(jitter)

            // Exponential backoff
            delay *= 2
            if delay > maxDelay {
                delay = maxDelay
            }
        }
    }

    var zero T
    return zero, lastErr
}
```

---

## Full Retry Implementation

```go
// internal/resilience/retry.go
package resilience

import (
    "errors"
    "math/rand"
    "time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
    MaxAttempts int           // How many times to try
    InitialDelay time.Duration // First retry delay
    MaxDelay     time.Duration // Maximum delay between retries
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     5 * time.Second,
    }
}

// RetryableFunc is a function that can be retried
type RetryableFunc[T any] func() (T, error)

// Retry executes a function with retry logic
func Retry[T any](fn RetryableFunc[T], config RetryConfig) (T, error) {
    var lastErr error
    delay := config.InitialDelay

    for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
        result, err := fn()
        if err == nil {
            return result, nil
        }

        lastErr = err

        // Check if error is retryable
        if !isRetryable(err) {
            var zero T
            return zero, err
        }

        // Wait before retry (except last attempt)
        if attempt < config.MaxAttempts {
            // Add jitter: 50% to 150% of delay
            jitter := time.Duration(float64(delay) * (0.5 + rand.Float64()))
            time.Sleep(jitter)

            // Exponential backoff
            delay *= 2
            if delay > config.MaxDelay {
                delay = config.MaxDelay
            }
        }
    }

    var zero T
    return zero, lastErr
}

// isRetryable checks if an error should be retried
func isRetryable(err error) bool {
    // Don't retry these errors
    var notRetryable interface{ NotRetryable() bool }
    if errors.As(err, &notRetryable) {
        return false
    }

    // Retry timeout errors, network errors, 5xx errors
    // Don't retry 4xx errors (client errors)
    return true
}
```

### Usage

```go
// Retry an HTTP call
result, err := resilience.Retry(func() (*http.Response, error) {
    resp, err := http.Get("https://api.example.com/data")
    if err != nil {
        return nil, err // Network error — retryable
    }
    if resp.StatusCode >= 500 {
        return nil, fmt.Errorf("server error: %d", resp.StatusCode) // Retryable
    }
    if resp.StatusCode >= 400 {
        return nil, &ClientError{Status: resp.StatusCode} // Not retryable
    }
    return resp, nil
}, resilience.DefaultRetryConfig())
```

---

## Circuit Breaker Pattern

A circuit breaker **stops calling a failing service** to prevent cascade failures.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                      CIRCUIT BREAKER STATE MACHINE                        │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │                                                                           │
  │                     ┌─────────────────────────┐                           │
  │                     │       CLOSED            │                           │
  │                     │    (Normal Operation)   │                           │
  │                     │                         │                           │
  │                     │  • Requests pass through│                           │
  │                     │  • Failures counted     │                           │
  │                     └───────────┬─────────────┘                           │
  │                                 │                                         │
  │                     failures > N│                                         │
  │                                 ▼                                         │
  │                     ┌─────────────────────────┐                           │
  │                     │        OPEN             │                           │
  │                     │   (Circuit Tripped)     │                           │
  │                     │                         │                           │
  │                     │  • FAIL FAST            │                           │
  │                     │  • Don't call service   │                           │
  │                     │  • Protects downstream  │                           │
  │                     └───────────┬─────────────┘                           │
  │                                 │                                         │
  │                     timeout     │                                         │
  │                     expires     │                                         │
  │                                 ▼                                         │
  │                     ┌─────────────────────────┐                           │
  │                     │      HALF-OPEN          │                           │
  │                     │  (Testing Recovery)     │                           │
  │                     │                         │                           │
  │                     │  • Allow ONE request    │  ─── success ──► CLOSED  │
  │                     │  • Test if service      │                           │
  │                     │    recovered            │  ─── failure ──► OPEN    │
  │                     └─────────────────────────┘                           │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
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

// CircuitState represents the circuit breaker state
type CircuitState int

const (
    StateClosed   CircuitState = iota // Normal operation
    StateOpen                          // Circuit tripped, failing fast
    StateHalfOpen                      // Testing if service recovered
)

func (s CircuitState) String() string {
    switch s {
    case StateClosed:
        return "CLOSED"
    case StateOpen:
        return "OPEN"
    case StateHalfOpen:
        return "HALF-OPEN"
    default:
        return "UNKNOWN"
    }
}

var (
    ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreaker protects against cascade failures
type CircuitBreaker struct {
    mu sync.Mutex

    // Configuration
    failureThreshold int           // Failures before opening
    successThreshold int           // Successes to close (from half-open)
    resetTimeout     time.Duration // Time to wait before trying again

    // State
    state            CircuitState
    failures         int
    successes        int
    lastFailureTime  time.Time
}

// NewCircuitBreaker creates a circuit breaker
func NewCircuitBreaker(failureThreshold, successThreshold int, resetTimeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        failureThreshold: failureThreshold,
        successThreshold: successThreshold,
        resetTimeout:     resetTimeout,
        state:            StateClosed,
    }
}

// Execute runs a function through the circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
    // Check if we can execute
    if !cb.allowExecution() {
        return ErrCircuitOpen
    }

    // Execute the function
    err := fn()

    // Record result
    if err != nil {
        cb.recordFailure()
    } else {
        cb.recordSuccess()
    }

    return err
}

// State returns the current state
func (cb *CircuitBreaker) State() CircuitState {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    return cb.state
}

func (cb *CircuitBreaker) allowExecution() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    switch cb.state {
    case StateClosed:
        return true

    case StateOpen:
        // Check if enough time has passed to try again
        if time.Since(cb.lastFailureTime) > cb.resetTimeout {
            cb.state = StateHalfOpen
            cb.successes = 0
            return true
        }
        return false

    case StateHalfOpen:
        return true // Allow request to test recovery

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
        // Failed during test, go back to open
        cb.state = StateOpen
    } else if cb.failures >= cb.failureThreshold {
        // Too many failures, trip the circuit
        cb.state = StateOpen
    }
}

func (cb *CircuitBreaker) recordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if cb.state == StateHalfOpen {
        cb.successes++
        if cb.successes >= cb.successThreshold {
            // Service recovered, close the circuit
            cb.state = StateClosed
            cb.failures = 0
            cb.successes = 0
        }
    } else {
        // Reset failure counter on success in closed state
        cb.failures = 0
    }
}
```

### Usage

```go
// Create a circuit breaker for an external API
apiBreaker := resilience.NewCircuitBreaker(
    5,               // Open after 5 failures
    3,               // Close after 3 successes
    30*time.Second,  // Wait 30 seconds before testing
)

// Use it to protect API calls
func callExternalAPI() error {
    return apiBreaker.Execute(func() error {
        resp, err := http.Get("https://external-api.com/data")
        if err != nil {
            return err
        }
        defer resp.Body.Close()

        if resp.StatusCode >= 500 {
            return fmt.Errorf("server error: %d", resp.StatusCode)
        }

        return nil
    })
}
```

---

## Retry + Circuit Breaker Combined

Use both patterns together for maximum resilience:

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                    COMBINED RESILIENCE PATTERN                            │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │                                                                           │
  │   ┌──────────┐     ┌───────────────────┐     ┌───────────┐     ┌──────┐ │
  │   │          │     │  Circuit Breaker  │     │   Retry   │     │      │ │
  │   │ Request  │────►│                   │────►│  (with    │────►│Service│ │
  │   │          │     │  • CLOSED: pass   │     │  backoff) │     │      │ │
  │   └──────────┘     │  • OPEN: fail fast│     │  • attempt 1│   └──────┘ │
  │                    │  • HALF-OPEN: test│     │  • attempt 2│             │
  │                    └────────┬──────────┘     │  • attempt 3│             │
  │                             │                └──────┬──────┘             │
  │                             │                       │                    │
  │                    circuit OPEN            all retries fail              │
  │                             │                       │                    │
  │                             ▼                       ▼                    │
  │                    ┌──────────────────┐    ┌──────────────────┐          │
  │                    │    FAIL FAST     │    │   Return error   │          │
  │                    │  (no call made)  │    │  to caller       │          │
  │                    └──────────────────┘    └──────────────────┘          │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

```go
// internal/resilience/client.go
package resilience

import (
    "net/http"
    "time"
)

type ResilientClient struct {
    httpClient *http.Client
    breaker    *CircuitBreaker
    retryConfig RetryConfig
}

func NewResilientClient() *ResilientClient {
    return &ResilientClient{
        httpClient: &http.Client{Timeout: 10 * time.Second},
        breaker: NewCircuitBreaker(
            5,              // 5 failures to open
            3,              // 3 successes to close
            30*time.Second, // 30s reset timeout
        ),
        retryConfig: RetryConfig{
            MaxAttempts:  3,
            InitialDelay: 100 * time.Millisecond,
            MaxDelay:     2 * time.Second,
        },
    }
}

func (c *ResilientClient) Get(url string) (*http.Response, error) {
    // Circuit breaker wraps retry
    var resp *http.Response
    
    err := c.breaker.Execute(func() error {
        var retryErr error
        resp, retryErr = Retry(func() (*http.Response, error) {
            r, err := c.httpClient.Get(url)
            if err != nil {
                return nil, err
            }
            if r.StatusCode >= 500 {
                r.Body.Close()
                return nil, fmt.Errorf("server error: %d", r.StatusCode)
            }
            return r, nil
        }, c.retryConfig)
        return retryErr
    })

    return resp, err
}
```

---

## When to Use Which

| Pattern | Use When |
|---------|----------|
| **Retry** | Transient failures (network, timeout, 5xx) |
| **Exponential Backoff** | Multiple clients may retry at once |
| **Jitter** | High traffic, many concurrent clients |
| **Circuit Breaker** | Protecting against downstream failures |
| **Both** | Critical external service calls |

---

## What to Retry (and What NOT to)

```
✓ DO retry:
  - Network timeouts
  - Connection errors
  - 5xx server errors
  - Rate limit (with backoff)

✗ DON'T retry:
  - 4xx client errors (your fault)
  - Authentication failures
  - Validation errors
  - Non-idempotent operations (without safeguards)
```

---

## Idempotency and Retries

If an operation can be retried safely, it's **idempotent**:

```go
// ✓ Idempotent: safe to retry
GET /users/123          // Read operations
PUT /users/123          // Update with same data
DELETE /users/123       // Delete is idempotent

// ✗ Not idempotent: risky to retry
POST /orders            // Creates new order each time!
POST /payments          // Charges twice!
```

For non-idempotent operations, use **idempotency keys**:

```go
func (s *PaymentService) Charge(amount float64, idempotencyKey string) error {
    // Check if already processed
    if s.alreadyProcessed(idempotencyKey) {
        return nil // Already done, safe to return success
    }
    
    // Process payment
    if err := s.processPayment(amount); err != nil {
        return err
    }
    
    // Mark as processed
    s.markProcessed(idempotencyKey)
    return nil
}
```

---

## Quick Reference

| Config | Recommended Value |
|--------|-------------------|
| Retry max attempts | 3-5 |
| Initial delay | 100-500ms |
| Max delay | 5-30s |
| Circuit breaker failure threshold | 5-10 |
| Circuit breaker reset timeout | 30-60s |
| Jitter range | ±50% of delay |

---

## Common Pitfalls

1. **Infinite retries** — Always set max attempts
2. **Retrying non-idempotent ops** — Can cause duplicates
3. **No circuit breaker** — Downstream failure cascades
4. **Too aggressive retry** — Makes problems worse
5. **Not logging failures** — Can't debug production issues

---

## Next Steps

- [Backpressure Strategies](08-backpressure-strategies.md) — Handle system overload
- [Milestone Project](../projects/20-layered-http-service.md) — Build resilient service