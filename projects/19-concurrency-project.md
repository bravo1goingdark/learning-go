# 19. Project: Concurrent Worker Pool with Graceful Shutdown

> **Goal:** Build a production-ready concurrent worker pool that processes data efficiently with proper graceful shutdown handling. This project combines topics 11-19.

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Thinking Process & Design](#2-thinking-process--design)
3. [Architecture](#3-architecture)
4. [Step 1 — Job & Worker Types](#4-step-1--job--worker-types)
5. [Step 2 — Worker Pool Implementation](#5-step-2--worker-pool-implementation)
6. [Step 3 — Graceful Shutdown with Context](#6-step-3--graceful-shutdown-with-context)
7. [Step 4 — Result Collection](#7-step-4--result-collection)
8. [Step 5 — CLI Interface (main.go)](#8-step-5--cli-interface-maingo)
9. [Step 6 — Tests](#9-step-6--tests)
10. [Build & Run](#10-build--run)
11. [Concept Map](#11-concept-map)

---

## 1. Project Overview

A CLI tool that:

- **Processes URLs concurrently** using a bounded worker pool
- **Handles graceful shutdown** — completes current jobs before stopping
- **Reports progress** in real-time
- **Collects results** from all workers
- **Handles errors** without crashing workers

### Sample Input (`urls.txt`)

```
https://golang.org
https://github.com
https://stackoverflow.com
https://golang.org/doc
https://github.com/golang/go
```

### CLI Usage

```bash
# Process URLs with 5 workers
./urlworker urls.txt --workers 5

# Process with timeout
./urlworker urls.txt --workers 5 --timeout 30s

# Quiet mode (less output)
./urlworker urls.txt --workers 5 --quiet
```

### Expected Output

```
$ ./urlworker urls.txt --workers 3
[worker-1] Started
[worker-2] Started  
[worker-3] Started
[worker-0] Processing: https://golang.org
[worker-1] Processing: https://github.com
[worker-2] Processing: https://stackoverflow.com
[worker-1] Completed: https://github.com (200 OK, 2.1s)
[worker-0] Completed: https://golang.org (200 OK, 1.5s)
[worker-2] Completed: https://stackoverflow.com (200 OK, 3.2s)
[worker-0] Processing: https://golang.org/doc
[worker-1] Processing: https://github.com/golang/go
[worker-1] Completed: https://github.com/golang/go (200 OK, 1.8s)
[worker-0] Completed: https://golang.org/doc (200 OK, 1.2s)
[worker-0] Stopped
[worker-1] Stopped
[worker-2] Stopped
--- Results ---
Total: 5, Success: 5, Failed: 0
```

---

## 2. Thinking Process & Design

### Why Do We Need a Worker Pool?

**Without a worker pool:**
```
10,000 URLs → 10,000 goroutines
- Each goroutine = ~2KB stack
- Total memory = ~20MB
- Too many! Causes scheduler thrashing
```

**With a worker pool:**
```
10,000 URLs → 10 workers (bounded)
- 10 goroutines = ~20KB
- URL queue in channel
- Memory efficient!
```

### Why Do We Need Graceful Shutdown?

**Bad shutdown (immediate):**
```go
// Just kill everything - lose work in progress!
os.Exit(0)
```

**Graceful shutdown:**
```go
// 1. Stop accepting new jobs
// 2. Let current jobs finish
// 3. Clean up resources
// 4. Exit
```

### Design Decisions

| Decision | Reasoning |
|----------|-----------|
| Use channels for job queue | Natural Go pattern, thread-safe |
| Use context for cancellation | Standard Go pattern, propagates to all workers |
| Use WaitGroup for completion | Simple, tells us when all done |
| Separate result channel | Avoid blocking workers on result collection |
| Mutex for metrics | Thread-safe counter updates |

---

## 3. Architecture

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                      WORKER POOL ARCHITECTURE                             │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │                        ┌──────────────────┐                               │
  │                        │  main (main.go)  │                               │
  │                        │                  │                               │
  │                        │  • Parse args    │                               │
  │                        │  • Create pool   │                               │
  │                        │  • Wait & collect│                               │
  │                        └────────┬─────────┘                               │
  │                                 │                                          │
  │                                 ▼                                          │
  │                        ┌──────────────────┐                               │
  │                        │   Job Queue      │  (buffered channel)           │
  │                        │   chan *Job      │                               │
  │                        └───┬──────┬───────┘                               │
  │                            │      │                                        │
  │               ┌────────────┘      └────────────┐                          │
  │               │                                 │                          │
  │               ▼                                 ▼                          │
  │        ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
  │        │   Worker 1   │  │   Worker 2   │  │   Worker 3   │              │
  │        │  (go func)   │  │  (go func)   │  │  (go func)   │              │
  │        └──────┬───────┘  └──────┬───────┘  └──────┬───────┘              │
  │               │                 │                  │                       │
  │               └─────────────────┼──────────────────┘                       │
  │                                 │                                          │
  │                                 ▼                                          │
  │                        ┌──────────────────┐                               │
  │                        │  Result Channel  │  (buffered channel)           │
  │                        │  chan Result     │                               │
  │                        └────────┬─────────┘                               │
  │                                 │                                          │
  │                                 ▼                                          │
  │                        ┌──────────────────┐                               │
  │                        │ Result Collector │                               │
  │                        │ (goroutine)      │                               │
  │                        └──────────────────┘                               │
  │                                                                           │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  GRACEFUL SHUTDOWN FLOW:                                          │  │
  │   │                                                                    │  │
  │   │  1. SIGINT received  ──►  context.Cancel() called                 │  │
  │   │  2. Workers see ctx.Done()  ──►  finish current job               │  │
  │   │  3. Workers exit gracefully                                       │  │
  │   │  4. WaitGroup.Wait()  ──►  all workers done                      │  │
  │   │  5. Print final results                                           │  │
  │   └───────────────────────────────────────────────────────────────────┘  │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

### Visual: Worker Lifecycle

```
IDLE STATE:
Jobs: [j1][j2][j3]...
Workers: [W1: waiting] [W2: waiting] [W3: waiting]

ACTIVE STATE:
Jobs: [j4][j5]...
Workers: [W1: j1 ✓] [W2: j2 ✓] [W3: j3 ✓]

SHUTDOWN STATE:
Jobs: empty (no more to process)
Workers: [W1: j4 ✓] [W2: done ✓] [W3: done ✓]

STOPPED:
All workers exited
```

---

## 4. Step 1 — Job & Worker Types

> **Topics Used:** Structs (Topic 5), Interfaces (Topic 7), Pointers (Topic 6)

### `types.go`

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Job represents a unit of work to be processed.
type Job struct {
	ID      int
	URL     string
	Timeout time.Duration
}

// Result holds the outcome of processing a job.
type JobResult struct {
	JobID     int
	URL      string
	Status   int
	Duration time.Duration
	Err      error
}

// Worker represents a worker that processes jobs.
type Worker struct {
	ID      int
	Jobs    <-chan *Job
	Results chan<- JobResult
	Context context.Context
	Group   *sync.WaitGroup
}

// NewWorker creates a new worker.
func NewWorker(id int, jobs <-chan *Job, results chan<- JobResult, ctx context.Context, wg *sync.WaitGroup) *Worker {
	return &Worker{
		ID:      id,
		Jobs:    jobs,
		Results: results,
		Context: ctx,
		Group:   wg,
	}
}

// Start begins the worker's processing loop.
// Topic 14: WaitGroup - tracks worker lifecycle
// Topic 11: Channels - receive jobs, send results
// Topic 13: Context - graceful shutdown
func (w *Worker) Start() {
	w.Group.Add(1)
	go func() {
		defer w.Group.Done()
		fmt.Printf("[worker-%d] Started\n", w.ID)

		for {
			select {
			case <-w.Context.Done():
				fmt.Printf("[worker-%d] Stopped\n", w.ID)
				return

			case job, ok := <-w.Jobs:
				if !ok {
					// Channel closed - no more jobs
					fmt.Printf("[worker-%d] Stopped\n", w.ID)
					return
				}
				w.process(job)
			}
		}
	}()
}

// process handles a single job.
// Topic 12: Select - handle job or context cancellation
// Topic 8: Error handling - wrap errors properly
func (w *Worker) process(job *Job) {
	fmt.Printf("[worker-%d] Processing: %s\n", w.ID, job.URL)

	start := time.Now()

	// Create request with timeout
	req, err := http.NewRequestWithContext(w.Context, "GET", job.URL, nil)
	if err != nil {
		w.Results <- JobResult{
			JobID:   job.ID,
			URL:    job.URL,
			Err:    fmt.Errorf("create request: %w", err),
		}
		return
	}

	// Make HTTP request
	client := &http.Client{Timeout: job.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		w.Results <- JobResult{
			JobID:   job.ID,
			URL:    job.URL,
			Err:    fmt.Errorf("request failed: %w", err),
		}
		return
	}
	defer resp.Body.Close()

	// Success
	duration := time.Since(start)
	fmt.Printf("[worker-%d] Completed: %s (%d, %v)\n", w.ID, job.URL, resp.StatusCode, duration)

	w.Results <- JobResult{
		JobID:     job.ID,
		URL:      job.URL,
		Status:   resp.StatusCode,
		Duration: duration,
	}
}
```

---

## 5. Step 2 — Worker Pool Implementation

> **Topics Used:** Channels (11), WaitGroup (14), Context (13), Mutex (15)

### `pool.go`

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// PoolConfig holds configuration for the worker pool.
type PoolConfig struct {
	NumWorkers int
	JobTimeout time.Duration
	QueueSize  int
}

// Pool manages the worker pool.
type Pool struct {
	config    PoolConfig
	jobs      chan *Job
	results   chan JobResult
	ctx       context.Context
	cancel    context.CancelFunc
	workers   []*Worker
	wg        sync.WaitGroup
	metrics   Metrics
}

// Metrics holds running statistics.
type Metrics struct {
	Total   int64
	Success int64
	Failed  int64
	mu       sync.Mutex
}

// NewPool creates a new worker pool.
func NewPool(cfg PoolConfig) *Pool {
	ctx, cancel := context.WithCancel(context.Background())

	return &Pool{
		config:  cfg,
		jobs:    make(chan *Job, cfg.QueueSize),
		results: make(chan JobResult, cfg.QueueSize),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start creates and starts all workers.
func (p *Pool) Start() {
	fmt.Printf("Starting pool with %d workers...\n", p.config.NumWorkers)

	p.workers = make([]*Worker, p.config.NumWorkers)

	for i := 0; i < p.config.NumWorkers; i++ {
		worker := NewWorker(i, p.jobs, p.results, p.ctx, &p.wg)
		worker.Start()
		p.workers[i] = worker
	}
}

// Submit adds a job to the queue.
func (p *Pool) Submit(job *Job) error {
	select {
	case p.jobs <- job:
		atomic.AddInt64(&p.metrics.Total, 1)
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// Shutdown gracefully stops all workers.
// Topic 13: Context cancellation for graceful shutdown
// Topic 14: WaitGroup for completion tracking
func (p *Pool) Shutdown() {
	fmt.Println("\n--- Initiating graceful shutdown ---")

	// Step 1: Cancel context - signals workers to stop
	p.cancel()

	// Step 2: Close job channel - signals no more jobs
	close(p.jobs)

	// Step 3: Wait for all workers to finish
	p.wg.Wait()

	// Step 4: Close results channel
	close(p.results)

	fmt.Println("--- All workers stopped ---")
}

// CollectResults gathers results from the result channel.
func (p *Pool) CollectResults() []JobResult {
	var results []JobResult

	for result := range p.results {
		p.updateMetrics(result)
		results = append(results, result)
	}

	return results
}

func (p *Pool) updateMetrics(r JobResult) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	if r.Err != nil {
		p.metrics.Failed++
	} else if r.Status >= 200 && r.Status < 300 {
		p.metrics.Success++
	}
}

// Metrics returns current metrics.
func (p *Pool) Metrics() Metrics {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	return Metrics{
		Total:   p.metrics.Total,
		Success: p.metrics.Success,
		Failed:  p.metrics.Failed,
	}
}

// Jobs returns the job channel for submitting work.
func (p *Pool) Jobs() chan<- *Job {
	return p.jobs
}
```

---

## 6. Step 3 — Graceful Shutdown with Context

> **Topics Used:** Context (13), Select (12), Channels (11)

### `shutdown.go`

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// SetupGracefulShutdown creates a context that cancels on SIGINT/SIGTERM.
// This is the standard pattern for graceful shutdown in Go.
func SetupGracefulShutdown() (context.Context, context.CancelFunc) {
	// Create a context that will be cancelled on signal
	ctx, cancel := context.WithCancel(context.Background())

	// Channel to receive signals
	sigCh := make(chan os.Signal, 1)

	// Notify on SIGINT (Ctrl+C) and SIGTERM (docker/k8s)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start goroutine to handle signals
	go func() {
		sig := <-sigCh
		fmt.Printf("\nReceived signal: %v\n", sig)
		fmt.Println("Initiating graceful shutdown...")
		cancel()
	}()

	return ctx, cancel
}

// RunWithGracefulShutdown runs the main function with graceful shutdown support.
func RunWithGracefulShutdown(fn func(ctx context.Context) error) {
	ctx, cancel := SetupGracefulShutdown()
	defer cancel()

	if err := fn(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

---

## 7. Step 4 — Result Collection

### `collector.go`

```go
package main

import (
	"fmt"
	"time"
)

// ResultAggregator collects and displays results.
type ResultAggregator struct {
	results   []JobResult
	startTime time.Time
}

// NewResultAggregator creates a new aggregator.
func NewResultAggregator() *ResultAggregator {
	return &ResultAggregator{
		results:   make([]JobResult, 0),
		startTime: time.Now(),
	}
}

// Add adds a result to the collection.
func (ra *ResultAggregator) Add(r JobResult) {
	ra.results = append(ra.results, r)
}

// Display prints the final summary.
func (ra *ResultAggregator) Display() {
	duration := time.Since(ra.startTime)

	fmt.Println("\n--- Results ---")

	// Group by status
	byStatus := make(map[int]int)
	var errors []JobResult

	for _, r := range ra.results {
		if r.Err != nil {
			errors = append(errors, r)
		} else {
			byStatus[r.Status]++
		}
	}

	// Print errors first
	if len(errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range errors {
			fmt.Printf("  - %s: %v\n", e.URL, e.Err)
		}
	}

	// Print status summary
	if len(byStatus) > 0 {
		fmt.Println("\nStatus codes:")
		for status, count := range byStatus {
			fmt.Printf("  %d: %d URLs\n", status, count)
		}
	}

	// Print summary
	success := len(ra.results) - len(errors)
	fmt.Printf("\nTotal: %d, Success: %d, Failed: %d (%.1fs)\n",
		len(ra.results),
		success,
		len(errors),
		duration.Seconds(),
	)
}

// AddFromChannel collects results until channel is closed.
func (ra *ResultAggregator) AddFromChannel(results chan JobResult) {
	for r := range results {
		ra.Add(r)
	}
}
```

---

## 8. Step 5 — CLI Interface (main.go)

### `main.go`

```go
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	// Parse command line flags
	// Topic 2: Variable declaration and zero values
	workers := flag.Int("workers", 3, "Number of concurrent workers")
	timeout := flag.Duration("timeout", 10*time.Second, "Timeout per request")
	queueSize := flag.Int("queue", 100, "Job queue size")
	quiet := flag.Bool("quiet", false, "Suppress detailed output")

	flag.Parse()

	// Get input file from positional arguments
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file> [flags]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	inputFile := args[0]

	// Run with graceful shutdown
	RunWithGracefulShutdown(func(ctx context.Context) error {
		return run(ctx, inputFile, *workers, *timeout, *queueSize, *quiet)
	})
}

func run(ctx context.Context, inputFile string, workers int, timeout time.Duration, queueSize int, quiet bool) error {
	// Read URLs from file
	urls, err := readURLs(inputFile)
	if err != nil {
		return fmt.Errorf("read URLs: %w", err)
	}

	if len(urls) == 0 {
		fmt.Println("No URLs to process")
		return nil
	}

	fmt.Printf("Loaded %d URLs, starting %d workers...\n", len(urls), workers)

	// Create and start pool
	pool := NewPool(PoolConfig{
		NumWorkers: workers,
		JobTimeout: timeout,
		QueueSize:  queueSize,
	})
	pool.Start()

	// Submit all jobs
	// Topic 11: Non-blocking submit with context check
	for i, url := range urls {
		url := strings.TrimSpace(url)
		if url == "" {
			continue
		}

		job := &Job{
			ID:      i,
			URL:     url,
			Timeout: timeout,
		}

		if err := pool.Submit(job); err != nil {
			if !quiet {
				fmt.Printf("Failed to submit %s: %v\n", url, err)
			}
		}
	}

	// Collect results in a separate goroutine
	// Topic 11: Goroutine for concurrent result collection
	aggregator := NewResultAggregator()
	go func() {
		aggregator.AddFromChannel(pool.Results())
	}()

	// Wait for context cancellation (signal) or all jobs done
	// Topic 13: Context controls when we stop waiting
	<-ctx.Done()

	// Graceful shutdown
	pool.Shutdown()

	// Wait for result collection to complete
	// Give a moment for final results to be collected
	time.Sleep(100 * time.Millisecond)

	// Display results
	aggregator.Display()

	return nil
}

func readURLs(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := scanner.Text()
		// Skip empty lines and comments
		if strings.TrimSpace(url) != "" && !strings.HasPrefix(url, "#") {
			urls = append(urls, url)
		}
	}

	return urls, scanner.Err()
}
```

---

## 9. Step 6 — Tests

### `pool_test.go`

```go
package main

import (
	"context"
	"testing"
	"time"
)

func TestPoolCreation(t *testing.T) {
	cfg := PoolConfig{
		NumWorkers: 3,
		JobTimeout: 5 * time.Second,
		QueueSize:  10,
	}

	pool := NewPool(cfg)

	if pool == nil {
		t.Fatal("expected pool to not be nil")
	}

	if pool.config.NumWorkers != 3 {
		t.Errorf("expected 3 workers, got %d", pool.config.NumWorkers)
	}
}

func TestPoolSubmit(t *testing.T) {
	pool := NewPool(PoolConfig{NumWorkers: 2, QueueSize: 5})
	pool.Start()
	defer pool.Shutdown()

	// Submit a job
	job := &Job{
		ID:      1,
		URL:     "https://golang.org",
		Timeout: 5 * time.Second,
	}

	err := pool.Submit(job)
	if err != nil {
		t.Errorf("submit failed: %v", err)
	}
}

func TestGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	pool := NewPool(PoolConfig{NumWorkers: 2})
	pool.Start()

	// Cancel after short delay
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Shutdown should complete without panic
	pool.Shutdown()
}

func TestMetrics(t *testing.T) {
	pool := NewPool(PoolConfig{NumWorkers: 1})

	// Add some mock results
	pool.results <- JobResult{Status: 200} // success
	pool.results <- JobResult{Status: 200} // success
	pool.results <- JobResult{Err: fmt.Errorf("error")} // failure

	close(pool.results)

	_ = pool.CollectResults()

	metrics := pool.Metrics()
	if metrics.Total != 3 {
		t.Errorf("expected total 3, got %d", metrics.Total)
	}
	if metrics.Success != 2 {
		t.Errorf("expected 2 successes, got %d", metrics.Success)
	}
	if metrics.Failed != 1 {
		t.Errorf("expected 1 failure, got %d", metrics.Failed)
	}
}
```

---

## 10. Build & Run

```bash
# Create test URL file
cat > urls.txt << EOF
https://golang.org
https://github.com
https://stackoverflow.com
https://golang.org/doc
https://github.com/golang/go
EOF

# Initialize module
go mod init urlworker

# Build
go build -o urlworker .

# Run
./urlworker urls.txt --workers 3

# Run with custom settings
./urlworker urls.txt --workers 5 --timeout 30s --quiet

# Run tests
go test -v ./...

# Run with race detector
go test -race ./...
```

---

## 11. Concept Map

Every topic from 10-18 is used in this project:

| # | Topic | Where Used | Example |
|---|-------|------------|---------|
| 10 | **Goroutines** | Each worker runs in a goroutine | `go func() { ... }()` |
| 11 | **Channels** | Job queue, result channel | `jobs chan *Job` |
| 12 | **Select** | Worker selects between job or ctx.Done | `select { case <-ctx.Done(): ... case job := <-w.Jobs: ... }` |
| 13 | **Context** | Graceful shutdown propagation | `context.WithCancel()` |
| 14 | **WaitGroup** | Track worker completion | `wg.Add(1)`, `wg.Done()` |
| 15 | **Mutex vs Channels** | Metrics need mutex | `metrics.mu.Lock()` |
| 16 | **Worker Pools** | This entire project! | Bounded concurrency |
| 17 | **Pipelines** | Jobs → Workers → Results | Pipeline pattern |
| 18 | **Fan-In/Fan-Out** | Multiple workers → result collector | Fan-in pattern |

---

> **You've built a production-grade concurrent system in Go. The patterns here are used in real systems like Kubernetes, Docker, and Terraform.**