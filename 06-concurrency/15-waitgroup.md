# 15. WaitGroup — Complete Deep Dive

> **Goal:** Master `sync.WaitGroup` — wait for a collection of goroutines to finish. The simplest synchronization primitive in Go.
>
> **How this connects:** Topics 11-14 taught you to create goroutines (11), communicate via channels (12), multiplex with select (13), and cancel with context (14). But how do you wait for multiple goroutines to finish? You could use channels, but that's verbose. `sync.WaitGroup` is the simpler tool for this exact job: "launch N goroutines, wait for all N to complete."

---
![WaitGroup](../assets/14.png)

## Table of Contents

1. [What Is WaitGroup](#1-what-is-waitgroup)
2. [Basic Usage](#2-basic-usage)
3. [API Reference](#3-api-reference)
4. [Waiting for N Goroutines](#4-waiting-for-n-goroutines)
5. [Dynamic Goroutine Spawning](#5-dynamic-goroutine-spawning)
6. [WaitGroup vs Channels](#6-waitgroup-vs-channels)
7. [Common Patterns](#7-common-patterns)
8. [Common Pitfalls](#8-common-pitfalls)

---

## 1. What Is WaitGroup [CORE]

A `WaitGroup` waits for a collection of goroutines to finish. The main goroutine calls `Add` to set the number of goroutines to wait for, then each goroutine calls `Done` when finished, and `Wait` blocks until all goroutines are done.

```
main goroutine                     worker goroutines
     │                                   │
     │── Add(N) ──►                      │
     │── go worker1() ───────────────────┤
     │── go worker2() ───────────────────┤
     │── go worker3() ───────────────────┤
     │                                   │
     │── Wait() ── (blocks)              │── ... working ...
     │                                   │── Done()
     │                                   │── Done()
     │                                   │── Done()
     │◄── (unblocks, counter = 0) ───────┤
     │                                   │
     │  continue                          │
```

---

## 2. Basic Usage [CORE]

```go
func main() {
    var wg sync.WaitGroup

    for i := 0; i < 5; i++ {
        wg.Add(1) // Increment counter BEFORE starting goroutine

        go func(id int) {
            defer wg.Done() // Decrement counter when done

            fmt.Printf("Worker %d starting\n", id)
            time.Sleep(time.Second)
            fmt.Printf("Worker %d done\n", id)
        }(i)
    }

    wg.Wait() // Block until counter reaches 0
    fmt.Println("All workers done")
}
```

### Output

```
Worker 0 starting
Worker 3 starting
Worker 1 starting
Worker 2 starting
Worker 4 starting
Worker 3 done
Worker 0 done
Worker 1 done
Worker 2 done
Worker 4 done
All workers done
```

---

## 3. API Reference [CORE]

```go
var wg sync.WaitGroup
```

| Method | Purpose | Notes |
|--------|---------|-------|
| `wg.Add(delta int)` | Increment counter by `delta` | Must call **before** goroutine starts |
| `wg.Done()` | Decrement counter by 1 | Usually via `defer wg.Done()` |
| `wg.Wait()` | Block until counter is 0 | Returns immediately if counter is already 0 |

### Counter Rules

- Counter must be **>= 0** at all times
- Calling `Done()` when counter is 0 **panics**
- Calling `Wait()` with counter > 0 **blocks**
- Calling `Add()` after `Wait()` returns is fine

---

## 4. Waiting for N Goroutines [CORE]

### Known Count

```go
func fetchAll(urls []string) []Result {
    var wg sync.WaitGroup
    results := make(chan Result, len(urls))

    wg.Add(len(urls)) // One Add for all

    for _, url := range urls {
        go func(u string) {
            defer wg.Done()
            r := fetch(u)
            results <- r
        }(url)
    }

    wg.Wait()
    close(results)

    var out []Result
    for r := range results {
        out = append(out, r)
    }
    return out
}
```

### One Add Per Goroutine

```go
var wg sync.WaitGroup

for i := 0; i < 5; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        work(id)
    }(i)
}

wg.Wait()
```

---

## 5. Dynamic Goroutine Spawning [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

When goroutines spawn more goroutines, use nested WaitGroups.

```go
func processTree(root *Node) {
    var wg sync.WaitGroup

    var walk func(n *Node)
    walk = func(n *Node) {
        defer wg.Done()
        process(n)

        for _, child := range n.Children {
            wg.Add(1)
            go walk(child)
        }
    }

    wg.Add(1)
    go walk(root)
    wg.Wait()
}
```

### Recursive Fan-Out

```go
func crawl(ctx context.Context, url string, depth int, wg *sync.WaitGroup, seen *sync.Map) {
    defer wg.Done()

    if depth <= 0 {
        return
    }

    links := fetchLinks(ctx, url)
    for _, link := range links {
        if _, loaded := seen.LoadOrStore(link, true); loaded {
            continue
        }
        wg.Add(1)
        go crawl(ctx, link, depth-1, wg, seen)
    }
}

func main() {
    var wg sync.WaitGroup
    seen := &sync.Map{}

    wg.Add(1)
    go crawl(context.Background(), "https://example.com", 3, &wg, seen)

    wg.Wait()
}
```

---

## 6. WaitGroup vs Channels [CORE]

| Feature | WaitGroup | Channel |
|---------|-----------|---------|
| Purpose | Wait for completion | Send/receive values |
| Result collection | No built-in result | Yes — send results back |
| Complexity | Simple counter | More flexible |
| Use when | Fire-and-forget | Need results or coordination |

### WaitGroup: Fire and Forget

```go
var wg sync.WaitGroup

for _, task := range tasks {
    wg.Add(1)
    go func(t Task) {
        defer wg.Done()
        t.Run() // Don't care about result
    }(task)
}
wg.Wait()
```

### Channel: Need Results

```go
results := make(chan Result, len(tasks))

for _, task := range tasks {
    go func(t Task) {
        results <- t.Run()
    }(task)
}

var all []Result
for range tasks {
    all = append(all, <-results)
}
```

### Both Together

```go
func processAll(tasks []Task) []Result {
    var wg sync.WaitGroup
    results := make(chan Result, len(tasks))

    for _, t := range tasks {
        wg.Add(1)
        go func(task Task) {
            defer wg.Done()
            results <- task.Run()
        }(t)
    }

    // Close channel after all goroutines done
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

## 7. Common Patterns [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

### Parallel Map

```go
func parallelMap[T, U any](items []T, fn func(T) U) []U {
    results := make([]U, len(items))
    var wg sync.WaitGroup
    wg.Add(len(items))

    for i, item := range items {
        go func(idx int, val T) {
            defer wg.Done()
            results[idx] = fn(val)
        }(i, item)
    }

    wg.Wait()
    return results
}
```

### Batch Processing with Limit

```go
func processBatch(items []Item, batchSize int) {
    for i := 0; i < len(items); i += batchSize {
        end := i + batchSize
        if end > len(items) {
            end = len(items)
        }

        var wg sync.WaitGroup
        for _, item := range items[i:end] {
            wg.Add(1)
            go func(it Item) {
                defer wg.Done()
                process(it)
            }(item)
        }
        wg.Wait()
    }
}
```

### Graceful Shutdown

```go
func main() {
    var wg sync.WaitGroup
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            worker(ctx, id)
        }(i)
    }

    // Wait for shutdown signal
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, os.Interrupt)
    <-sig

    cancel()   // Signal all workers to stop
    wg.Wait()  // Wait for all workers to finish
}
```

---

## 8. Common Pitfalls [CORE]

| Pitfall | Problem | Fix |
|---------|---------|-----|
| `Add` inside goroutine | Race — `Wait()` may return before `Add` | Call `Add` before `go` |
| Forgetting `Done()` | `Wait()` blocks forever | Use `defer wg.Done()` |
| Negative counter | Panic | Balance `Add` and `Done` calls |
| Passing `WaitGroup` by value | Each copy has its own counter | Pass `*sync.WaitGroup` |
| Reusing WaitGroup before done | Undefined behavior | Create new WaitGroup per batch |
| Calling `Add(0)` | No effect | Only useful with `Done()` |

### The Add-Before-Go Rule

```go
// WRONG — race condition
for i := 0; i < 5; i++ {
    go func() {
        wg.Add(1)        // May run after wg.Wait()
        defer wg.Done()
        work()
    }()
}
wg.Wait() // May return before all goroutines start

// RIGHT — Add before go
for i := 0; i < 5; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        work()
    }()
}
wg.Wait()
```

---

## Exercises

### Exercise 1: Random Sleep Workers ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Launch 5 goroutines. Each sleeps for a random duration between 100–500ms, then prints its ID and the duration it slept. Use a `sync.WaitGroup` to wait for all to finish.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			d := time.Duration(r.Intn(400)+100) * time.Millisecond
			time.Sleep(d)
			fmt.Printf("worker %d slept %v\n", id, d)
		}(i)
	}

	wg.Wait()
	fmt.Println("all workers done")
}
```

</details>

### Exercise 2: Parallel URL Fetcher ⭐⭐
**Difficulty:** Intermediate | **Time:** ~10 min

Write a function that takes a slice of URL strings and launches a goroutine for each one. Each goroutine prints "fetching <url>" (simulate, no real HTTP). Use a WaitGroup to wait for all to complete.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"sync"
	"time"
)

func fetchAll(urls []string) {
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			fmt.Printf("fetching %s\n", u)
			time.Sleep(100 * time.Millisecond) // simulate network
			fmt.Printf("done %s\n", u)
		}(url)
	}

	wg.Wait()
	fmt.Println("all fetches complete")
}

func main() {
	urls := []string{
		"https://example.com/a",
		"https://example.com/b",
		"https://example.com/c",
		"https://example.com/d",
	}
	fetchAll(urls)
}
```

</details>

### Exercise 3: Panic from Add After Wait ⭐⭐
**Difficulty:** Intermediate | **Time:** ~10 min

Demonstrate that calling `wg.Add(1)` after `wg.Wait()` has already returned (and the counter was 0) is safe, but calling `wg.Done()` when the counter is 0 causes a panic. Write both cases.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	// Case 1: Add after Wait returns — this is safe
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("first goroutine done")
	}()
	wg.Wait()
	fmt.Println("first wait returned")

	wg.Add(1) // Safe: Wait already returned, reusing is OK
	go func() {
		defer wg.Done()
		fmt.Println("second goroutine done")
	}()
	wg.Wait()
	fmt.Println("second wait returned")

	// Case 2: Done when counter is 0 — panics
	// Uncomment to see the panic:
	// var wg2 sync.WaitGroup
	// wg2.Done() // panic: sync: negative WaitGroup counter
}
```

</details>
