# 19. Fan-In / Fan-Out — Complete Deep Dive

> **Goal:** Master fan-out and fan-in — distribute work across goroutines (fan-out) and collect results back (fan-in). The foundation of concurrent processing patterns.
>
> **How this fits together:** Fan-out is the distribution part of a worker pool. Fan-in is the collection part. A pipeline is a series of fan-out/fan-in stages chained together. These are not separate patterns — they are the building blocks that worker pools (Topic 17) and pipelines (Topic 18) are composed from.

---
![Fan-In Fan-Out](../assets/18.png)

## Table of Contents

1. [What Is Fan-Out / Fan-In](#1-what-is-fan-out--fan-in) `[CORE]`
2. [Fan-Out](#2-fan-out) `[PRODUCTION]`
3. [Fan-In](#3-fan-in) `[PRODUCTION]`
4. [Fan-Out / Fan-In Together](#4-fan-out--fan-in-together) `[PRODUCTION]`
5. [Fan-Out with Context](#5-fan-out-with-context) `[PRODUCTION]`
6. [Fan-Out with Error Handling](#6-fan-out-with-error-handling) `[PRODUCTION]`
7. [Bounded Fan-Out](#7-bounded-fan-out) `[PRODUCTION]`
8. [Fan-In from Multiple Sources](#8-fan-in-from-multiple-sources) `[PRODUCTION]`
9. [Real-World Examples](#9-real-world-examples) `[PRODUCTION]`
10. [Common Pitfalls](#10-common-pitfalls) `[CORE]`

---

## 1. What Is Fan-Out / Fan-In [CORE]

```
Fan-Out: Distribute work to multiple goroutines

                    ┌── Worker 1 ──┐
   input ──────────►├── Worker 2 ──├──► output
                    ├── Worker 3 ──┤
                    └── Worker 4 ──┘

Fan-In: Collect results from multiple goroutines into one channel

   Worker 1 ──┐
   Worker 2 ──┼──► merged output
   Worker 3 ──┤
   Worker 4 ──┘
```

| Pattern | Purpose | Direction |
|---------|---------|-----------|
| Fan-Out | Distribute work | One → Many |
| Fan-In | Collect results | Many → One |

---

## 2. Fan-Out [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

Fan-out starts multiple goroutines that read from the **same channel**.

### Simple Fan-Out

```go
func worker(id int, jobs <-chan int, results chan<- int) {
    for j := range jobs {
        fmt.Printf("worker %d processing job %d\n", id, j)
        time.Sleep(time.Millisecond * 100)
        results <- j * 2
    }
}

func main() {
    jobs := make(chan int, 100)
    results := make(chan int, 100)

    // Fan-out: 5 workers reading from same jobs channel
    for w := 1; w <= 5; w++ {
        go worker(w, jobs, results)
    }

    // Send jobs
    for j := 1; j <= 20; j++ {
        jobs <- j
    }
    close(jobs)

    // Collect results
    for i := 1; i <= 20; i++ {
        fmt.Println("result:", <-results)
    }
}
```

### How It Works

- All workers compete for jobs on the **same channel**
- Each job is consumed by **exactly one** worker (no duplication)
- Go's channel ensures **no races** — only one goroutine receives each value

### Fan-Out with Varying Work

```go
func worker(id int, jobs <-chan Job) {
    for job := range jobs {
        // Different jobs take different time
        // Faster workers naturally pick up more jobs
        process(job)
    }
}
```

---

## 3. Fan-In [PRODUCTION]

Fan-in merges multiple channels into **one output channel**.

### Using WaitGroup

```go
func fanIn(channels ...<-chan string) <-chan string {
    out := make(chan string)
    var wg sync.WaitGroup

    // Start a goroutine for each input channel
    for _, ch := range channels {
        wg.Add(1)
        go func(c <-chan string) {
            defer wg.Done()
            for val := range c {
                out <- val
            }
        }(ch)
    }

    // Close output when all inputs are drained
    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

### Usage

```go
func main() {
    ch1 := make(chan string)
    ch2 := make(chan string)
    ch3 := make(chan string)

    go func() { defer close(ch1); ch1 <- "from ch1" }()
    go func() { defer close(ch2); ch2 <- "from ch2" }()
    go func() { defer close(ch3); ch3 <- "from ch3" }()

    merged := fanIn(ch1, ch2, ch3)

    for msg := range merged {
        fmt.Println(msg)
    }
}
```

### Fan-In with Select (No WaitGroup)

```go
func fanInSelect(ch1, ch2 <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for ch1 != nil || ch2 != nil {
            select {
            case v, ok := <-ch1:
                if !ok {
                    ch1 = nil
                    continue
                }
                out <- v
            case v, ok := <-ch2:
                if !ok {
                    ch2 = nil
                    continue
                }
                out <- v
            }
        }
    }()
    return out
}
```

---

## 4. Fan-Out / Fan-In Together [PRODUCTION]

The full pattern: distribute work (fan-out) then collect results (fan-in).

```
              Fan-Out                          Fan-In
              ───────                          ──────
                      ┌── Worker 1 ──┐
   generator ────────►├── Worker 2 ──├──► collector
                      ├── Worker 3 ──┤
                      └── Worker 4 ──┘
```

### Complete Implementation

```go
func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Stage 1: Generate jobs
    jobs := generate(ctx, 100)

    // Stage 2: Fan-out to workers
    const numWorkers = 10
    results := make([]<-chan Result, numWorkers)
    for i := 0; i < numWorkers; i++ {
        results[i] = worker(ctx, i, jobs)
    }

    // Stage 3: Fan-in results
    merged := fanIn(ctx, results...)

    // Stage 4: Consume
    var count int
    for r := range merged {
        if r.Err != nil {
            log.Printf("error: %v", r.Err)
            continue
        }
        count++
        log.Printf("result %d: %v", count, r.Value)
    }

    log.Printf("processed %d results", count)
}

type Result struct {
    Value int
    Err   error
}

func generate(ctx context.Context, count int) <-chan int {
    out := make(chan int, 100)
    go func() {
        defer close(out)
        for i := 0; i < count; i++ {
            select {
            case out <- i:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

func worker(ctx context.Context, id int, jobs <-chan int) <-chan Result {
    out := make(chan Result, 10)
    go func() {
        defer close(out)
        for j := range jobs {
            val, err := process(ctx, j)
            select {
            case out <- Result{Value: val, Err: err}:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

func fanIn(ctx context.Context, channels ...<-chan Result) <-chan Result {
    out := make(chan Result)
    var wg sync.WaitGroup

    for _, ch := range channels {
        wg.Add(1)
        go func(c <-chan Result) {
            defer wg.Done()
            for r := range c {
                select {
                case out <- r:
                case <-ctx.Done():
                    return
                }
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

---

## 5. Fan-Out with Context [PRODUCTION]

All goroutines respect cancellation. When context is cancelled, everything stops.

```go
func worker(ctx context.Context, id int, jobs <-chan Job) <-chan Result {
    out := make(chan Result, 10)
    go func() {
        defer close(out)
        for {
            select {
            case <-ctx.Done():
                log.Printf("worker %d: cancelled", id)
                return
            case job, ok := <-jobs:
                if !ok {
                    return
                }
                result, err := job.Process(ctx)
                select {
                case out <- Result{Value: result, Err: err}:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()
    return out
}
```

### Graceful Shutdown Sequence

```
1. SIGINT received
2. cancel() called on context
3. All workers see ctx.Done() → exit
4. All result channels closed
5. Fan-in sees all channels closed → closes output
6. Collector drains remaining → returns
```

---

## 6. Fan-Out with Error Handling [PRODUCTION]

### Fail-Fast: Cancel on First Error

```go
func processAll(ctx context.Context, items []Item) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    jobs := make(chan Item, len(items))
    for _, item := range items {
        jobs <- item
    }
    close(jobs)

    errCh := make(chan error, 10)
    var wg sync.WaitGroup

    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for item := range jobs {
                if err := process(ctx, item); err != nil {
                    select {
                    case errCh <- err:
                    default:
                    }
                    cancel() // Cancel all workers
                    return
                }
            }
        }()
    }

    go func() {
        wg.Wait()
        close(errCh)
    }()

    // Return first error
    for err := range errCh {
        return err
    }
    return nil
}
```

### Collect All Errors

```go
func processAll(ctx context.Context, items []Item) []error {
    var mu sync.Mutex
    var errs []error
    var wg sync.WaitGroup

    jobs := make(chan Item, len(items))
    for _, item := range items {
        jobs <- item
    }
    close(jobs)

    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for item := range jobs {
                if err := process(ctx, item); err != nil {
                    mu.Lock()
                    errs = append(errs, err)
                    mu.Unlock()
                }
            }
        }()
    }

    wg.Wait()
    return errs
}
```

---

## 7. Bounded Fan-Out [PRODUCTION]

Without bounds, spawning a goroutine per item creates N goroutines for N items. With 100,000 items, that's ~200MB of stack memory, massive scheduler contention, and potential OOM. Bounded fan-out limits concurrent goroutines to a fixed number (e.g., 10-100), keeping resource usage predictable regardless of input size.

Limit concurrent goroutines to prevent resource exhaustion.

### Using Semaphore Channel

```go
func boundedFanOut(ctx context.Context, items []Item, maxConcurrent int) []Result {
    semaphore := make(chan struct{}, maxConcurrent)
    var wg sync.WaitGroup
    results := make(chan Result, len(items))

    for _, item := range items {
        wg.Add(1)
        go func(it Item) {
            defer wg.Done()

            semaphore <- struct{}{}        // Acquire
            defer func() { <-semaphore }() // Release

            r := process(ctx, it)
            results <- r
        }(item)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    var out []Result
    for r := range results {
        out = append(out, r)
    }
    return out
}
```

### Using Worker Pool Instead

```go
// Cleaner approach — use a fixed worker pool
func boundedFanOut(ctx context.Context, items []Item, numWorkers int) []Result {
    jobs := make(chan Item, len(items))
    results := make(chan Result, len(items))

    for _, item := range items {
        jobs <- item
    }
    close(jobs)

    var wg sync.WaitGroup
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for item := range jobs {
                r := process(ctx, item)
                results <- r
            }
        }()
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    var out []Result
    for r := range results {
        out = append(out, r)
    }
    return out
}
```

---

## 8. Fan-In from Multiple Sources [PRODUCTION]

### Merge HTTP Response Channels

```go
func fetchAll(urls []string) <-chan Response {
    channels := make([]<-chan Response, len(urls))

    // Fan-out: one goroutine per URL
    for i, url := range urls {
        channels[i] = fetch(ctx, url)
    }

    // Fan-in: merge all response channels
    return merge(channels...)
}

func fetch(ctx context.Context, url string) <-chan Response {
    out := make(chan Response, 1)
    go func() {
        defer close(out)
        resp, err := http.Get(url)
        if err != nil {
            out <- Response{URL: url, Err: err}
            return
        }
        defer resp.Body.Close()
        body, _ := io.ReadAll(resp.Body)
        out <- Response{URL: url, Body: body}
    }()
    return out
}

func merge[T any](channels ...<-chan T) <-chan T {
    out := make(chan T)
    var wg sync.WaitGroup

    for _, ch := range channels {
        wg.Add(1)
        go func(c <-chan T) {
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

### Merge with Priority

```go
func mergeWithPriority(high, low <-chan Job) <-chan Job {
    out := make(chan Job)
    go func() {
        defer close(out)
        for high != nil || low != nil {
            select {
            case job, ok := <-high:
                if !ok {
                    high = nil
                    continue
                }
                out <- job
            case job, ok := <-low:
                if !ok {
                    low = nil
                    continue
                }
                out <- job
            }
        }
    }()
    return out
}
```

---

## 9. Real-World Examples [PRODUCTION]

### Parallel URL Checker

```go
func checkURLs(urls []string) map[string]error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    type result struct {
        url string
        err error
    }

    jobs := make(chan string, len(urls))
    results := make(chan result, len(urls))

    for _, u := range urls {
        jobs <- u
    }
    close(jobs)

    const workers = 20
    var wg sync.WaitGroup
    wg.Add(workers)

    for i := 0; i < workers; i++ {
        go func() {
            defer wg.Done()
            for url := range jobs {
                err := checkURL(ctx, url)
                results <- result{url, err}
            }
        }()
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    status := make(map[string]error)
    for r := range results {
        status[r.url] = r.err
    }
    return status
}
```

### Parallel File Processing

```go
func processFiles(paths []string) error {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    pathCh := make(chan string, len(paths))
    for _, p := range paths {
        pathCh <- p
    }
    close(pathCh)

    errCh := make(chan error, len(paths))
    var wg sync.WaitGroup

    const workers = 5
    wg.Add(workers)

    for i := 0; i < workers; i++ {
        go func() {
            defer wg.Done()
            for path := range pathCh {
                if err := processFile(ctx, path); err != nil {
                    select {
                    case errCh <- fmt.Errorf("%s: %w", path, err):
                    default:
                    }
                    cancel()
                    return
                }
            }
        }()
    }

    go func() {
        wg.Wait()
        close(errCh)
    }()

    for err := range errCh {
        return err
    }
    return nil
}
```

---

## 10. Common Pitfalls [CORE]

| Pitfall | Problem | Fix |
|---------|---------|-----|
| Forgetting to close channels | Fan-in blocks forever | Close output when workers done |
| No context | Can't cancel fan-out | Pass `ctx` to every goroutine |
| Unbounded fan-out | OOM, scheduler thrashing | Use bounded pool or semaphore |
| Race on shared slice | Data corruption | Use mutex or indexed write |
| Not draining result channel | Goroutine leak | Always consume or discard |
| Deadlock on full buffer | Workers block, nobody drains | Separate collection goroutine |
| Closing input channel twice | Panic | Track ownership clearly |

### The Golden Rules

1. **Whoever creates the channel closes it** (or the last sender)
2. **Always use context** for cancellation
3. **Buffer result channels** to avoid blocking workers
4. **Fan-in must close its output** when all inputs are drained
5. **Never assume ordering** — fan-out results arrive in any order

---

## Exercises

### Exercise 1: Fan-Out Fan-In ⭐
**Difficulty:** Beginner | **Time:** ~15 min

Create a single job channel with 12 jobs. Fan-out to 4 worker goroutines that each double the job value and send it to a result channel. Fan-in results to one output channel and collect in main.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"sync"
)

func worker(id int, jobs <-chan int, results chan<- int) {
	for j := range jobs {
		fmt.Printf("worker %d processing %d\n", id, j)
		results <- j * 2
	}
}

func fanIn(channels ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup

	for _, ch := range channels {
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

func main() {
	jobs := make(chan int, 12)
	for i := 1; i <= 12; i++ {
		jobs <- i
	}
	close(jobs)

	const numWorkers = 4
	workerResults := make([]<-chan int, numWorkers)
	for w := 0; w < numWorkers; w++ {
		ch := make(chan int, 3)
		workerResults[w] = ch
		go worker(w, jobs, ch)
	}

	// Fan-in
	merged := fanIn(workerResults...)
	for val := range merged {
		fmt.Println("result:", val)
	}
}
```

</details>

### Exercise 2: Context Cancellation in Fan-Out ⭐⭐
**Difficulty:** Intermediate | **Time:** ~15 min

Extend Exercise 1 by adding a `context.Context`. After receiving 6 results, cancel the context. Verify all workers stop and the program exits cleanly.

<details>
<summary>Solution</summary>

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func worker(ctx context.Context, id int, jobs <-chan int, results chan<- int) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("worker %d stopped\n", id)
			return
		case j, ok := <-jobs:
			if !ok {
				return
			}
			time.Sleep(100 * time.Millisecond)
			select {
			case results <- j * 2:
			case <-ctx.Done():
				return
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan int, 20)
	results := make(chan int, 20)

	for i := 1; i <= 20; i++ {
		jobs <- i
	}
	close(jobs)

	for w := 0; w < 4; w++ {
		go worker(ctx, w, jobs, results)
	}

	count := 0
	for r := range results {
		fmt.Println("result:", r)
		count++
		if count >= 6 {
			cancel()
			break
		}
	}

	time.Sleep(200 * time.Millisecond)
	fmt.Printf("collected %d results, cancelled\n", count)
}
```

</details>

### Exercise 3: Bounded Fan-Out with Semaphore ⭐⭐
**Difficulty:** Intermediate | **Time:** ~15 min

Process 20 items, but limit concurrent goroutines to 3 using a buffered channel as a semaphore. Each goroutine acquires the semaphore before processing and releases it after.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	sem := make(chan struct{}, 3) // max 3 concurrent
	var wg sync.WaitGroup
	results := make(chan string, 20)

	for i := 1; i <= 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			fmt.Printf("processing item %d\n", id)
			time.Sleep(200 * time.Millisecond)
			results <- fmt.Sprintf("item %d done", id)
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		fmt.Println(r)
	}
	fmt.Println("all done")
}
```

</details>
