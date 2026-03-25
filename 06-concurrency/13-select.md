# 13. Select — Complete Deep Dive

> **Goal:** Master `select` — Go's multiplexer for channels. Handle multiple channel operations, timeouts, and non-blocking patterns.

---
![Select Keyword](../assets/12.png)
## Table of Contents

1. [Select Basics](#1-select-basics)
2. [How Select Works](#2-how-select-works)
3. [Default Case (Non-Blocking)](#3-default-case-non-blocking)
4. [Timeouts with time.After](#4-timeouts-with-timeafter)
5. [Select with Done Channel](#5-select-with-done-channel)
6. [Select with Nil Channels](#6-select-with-nil-channels)
7. [For-Select Loop](#7-for-select-loop)
8. [Common Patterns](#8-common-patterns)
9. [Common Pitfalls](#9-common-pitfalls)

---

## 1. Select Basics

`select` lets a goroutine wait on **multiple channel operations** simultaneously.

```go
select {
case msg := <-ch1:
    fmt.Println("received from ch1:", msg)
case ch2 <- 42:
    fmt.Println("sent to ch2")
case msg := <-ch3:
    fmt.Println("received from ch3:", msg)
}
```

### Rules

- Blocks until **one** case can proceed
- If multiple cases are ready, **one is chosen at random** (fairness)
- If no case is ready and no `default`, it **blocks**
- `default` makes it **non-blocking**

---

## 2. How Select Works

```
select {
case msg := <-ch1:      // Case 1: receive from ch1
case ch2 <- value:      // Case 2: send to ch2
case msg := <-ch3:      // Case 3: receive from ch3
default:                 // Case 4: non-blocking fallback
}
```

### Execution Flow

```
1. Evaluate all channel expressions and send/receive expressions
2. Check each case:
   a. Can proceed? → mark as ready
   b. Cannot? → skip
3. If any case is ready:
   a. Pick one at random (if multiple ready)
   b. Execute that case
4. If no case is ready:
   a. Has default? → execute default
   b. No default? → block, go to step 2
```

### Random Selection Prevents Starvation

```go
ch1 := make(chan int, 1)
ch2 := make(chan int, 1)

ch1 <- 1
ch2 <- 2

// Both ch1 and ch2 are ready
select {
case v := <-ch1:
    fmt.Println("ch1:", v)
case v := <-ch2:
    fmt.Println("ch2:", v)
}
// Could print either ch1 or ch2 — chosen randomly
```

---

## 3. Default Case (Non-Blocking)

Adding `default` makes `select` return immediately if no channel is ready.

### Non-Blocking Send

```go
select {
case ch <- value:
    fmt.Println("sent")
default:
    fmt.Println("channel full or no receiver, skipping")
}
```

### Non-Blocking Receive

```go
select {
case msg := <-ch:
    fmt.Println("received:", msg)
default:
    fmt.Println("nothing available, continuing")
}
```

### Polling Pattern

```go
func poll(ch <-chan int) {
    for {
        select {
        case val := <-ch:
            process(val)
        default:
            // No data, do other work
            time.Sleep(100 * time.Millisecond)
        }
    }
}
```

**Warning:** `default` in a loop = busy-wait (burns CPU). Use carefully.

---

## 4. Timeouts with time.After

`time.After` returns a channel that sends the current time after a duration.

```go
select {
case msg := <-ch:
    fmt.Println("received:", msg)
case <-time.After(5 * time.Second):
    fmt.Println("timeout — giving up")
}
```

### Per-Operation Timeout

```go
func fetchWithTimeout(ctx context.Context, url string) ([]byte, error) {
    resultCh := make(chan []byte, 1)
    errCh := make(chan error, 1)

    go func() {
        data, err := http.Get(url)
        if err != nil {
            errCh <- err
            return
        }
        defer data.Body.Close()
        body, _ := io.ReadAll(data.Body)
        resultCh <- body
    }()

    select {
    case body := <-resultCh:
        return body, nil
    case err := <-errCh:
        return nil, err
    case <-time.After(10 * time.Second):
        return nil, fmt.Errorf("request timed out")
    }
}
```

### Tick + Timeout

```go
func heartbeat(done <-chan struct{}) {
    tick := time.Tick(500 * time.Millisecond)
    timeout := time.After(10 * time.Second)

    for {
        select {
        case <-tick:
            fmt.Println("heartbeat")
        case <-timeout:
            fmt.Println("timed out")
            return
        case <-done:
            fmt.Println("done")
            return
        }
    }
}
```

---

## 5. Select with Done Channel

### Context Cancellation Pattern

```go
func worker(ctx context.Context, ch <-chan int) {
    for {
        select {
        case <-ctx.Done():
            fmt.Println("cancelled:", ctx.Err())
            return
        case val := <-ch:
            process(val)
        }
    }
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    ch := make(chan int)
    go worker(ctx, ch)

    // Send work...
    ch <- 1
    ch <- 2
}
```

### Graceful Shutdown

```go
func run(ctx context.Context) {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

    for {
        select {
        case <-ctx.Done():
            fmt.Println("context done")
            return
        case sig := <-sigCh:
            fmt.Println("signal:", sig)
            return
        }
    }
}
```

---

## 6. Select with Nil Channels

Operations on nil channels **block forever**, effectively **disabling** that case.

```go
func merge(ch1, ch2 <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)

        for ch1 != nil || ch2 != nil {
            select {
            case v, ok := <-ch1:
                if !ok {
                    ch1 = nil // Disable this case
                    continue
                }
                out <- v
            case v, ok := <-ch2:
                if !ok {
                    ch2 = nil // Disable this case
                    continue
                }
                out <- v
            }
        }
    }()
    return out
}
```

### Why This Works

| ch1 state | Effect in select |
|-----------|-----------------|
| `chan int` | Case active — can receive |
| `nil` | Case blocked forever — skipped |

---

## 7. For-Select Loop

The most common concurrency pattern in Go.

```go
for {
    select {
    case <-done:
        return
    case msg := <-inCh:
        result := process(msg)
        outCh <- result
    }
}
```

### With Drain Phase

```go
func worker(ctx context.Context, ch <-chan int) {
    for {
        select {
        case <-ctx.Done():
            // Drain remaining items
            for {
                select {
                case msg := <-ch:
                    process(msg)
                default:
                    return // Nothing left
                }
            }
        case msg := <-ch:
            process(msg)
        }
    }
}
```

### With Tick

```go
func monitor(ctx context.Context, ch <-chan Event) {
    tick := time.NewTicker(time.Second)
    defer tick.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case event := <-ch:
            handleEvent(event)
        case <-tick.C:
            collectMetrics()
        }
    }
}
```

---

## 8. Common Patterns

### Multiplexing (Merge Channels)

```go
func fanIn(chs ...<-chan int) <-chan int {
    out := make(chan int)
    var wg sync.WaitGroup

    for _, ch := range chs {
        wg.Add(1)
        go func(c <-chan int) {
            defer wg.Done()
            for v := range c {
                out <- v
            }
        }(ch)
    }

    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

### Rate Limiting

```go
func rateLimited(work <-chan Job, rate time.Duration) {
    ticker := time.NewTicker(rate)
    defer ticker.Stop()

    for job := range work {
        <-ticker.C // Wait for tick
        process(job)
    }
}
```

### Cancellable Read

```go
func readWithCancel(ctx context.Context, ch <-chan int) (int, error) {
    select {
    case val := <-ch:
        return val, nil
    case <-ctx.Done():
        return 0, ctx.Err()
    }
}
```

---

## 9. Common Pitfalls

| Pitfall | Problem | Fix |
|---------|---------|-----|
| Empty select `{}` | Blocks forever — deadlock | Use for specific purpose only |
| `default` in tight loop | Busy-wait, burns CPU | Add sleep or remove `default` |
| Nil channel without intent | Blocks case forever | Set to nil only to disable case |
| Missing timeout | Hangs forever | Always use `time.After` or `ctx.Done()` |
| Multiple sends in select | Only one fires | Design for single operation per select |
| Forgetting `ok` check | Miss closed channel | Always use `val, ok := <-ch` |
