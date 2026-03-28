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

### What We're Building

A CLI tool that processes URLs concurrently using a bounded worker pool with graceful shutdown:

- **Processes URLs concurrently** using a bounded worker pool
- **Handles graceful shutdown** — completes current jobs before stopping
- **Reports progress** in real-time
- **Collects results** from all workers
- **Handles errors** without crashing workers

### Why This Project?

| Why This Matters | Explanation |
|-----------------|-------------|
| **Real-world usage** | Every backend service needs background workers — HTTP clients, job processors, queue consumers |
| **Concurrency mastery** | Combines goroutines, channels, WaitGroup, context — all Topics 11-19 |
| **Production skills** | Graceful shutdown isn't optional — it's required for zero-downtime deployments |
| **Error handling** | One crashing worker shouldn't kill the entire pool |

### How It Works (Intuition)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        WORKER POOL FLOW                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   URLs Channel ──► Job Queue ──► Worker 1 ──► Results                     │
│                       │              │                                     │
│                       │              └──► Worker 2 ──► Results             │
│                       │              │                                     │
│                       │              └──► Worker 3 ──► Results             │
│                       │                                                   │
│                       ▼                                                   │
│                  (bounded, blocks if full)                                 │
│                                                                             │
│   GRACEFUL SHUTDOWN:                                                       │
│   1. Close URL channel (no new jobs)                                       │
│   2. Wait for workers to finish current job                                │
│   3. Collect final results                                                 │
│   4. Exit                                                                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key insight:** The worker pool is a **pipeline**:
- **Stage 1:** Read URLs from file → send to job channel
- **Stage 2:** Workers pull from job channel → process → send results
- **Stage 3:** Main goroutine collects results

Each stage runs concurrently. Channels connect them without shared state.

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

### What / Why / How

**What:** Define Job, JobResult, and Worker types.

**Why:**
- **Job** — represents work to do (URL + timeout)
- **JobResult** — represents work completion (status + error)
- **Worker** — the goroutine that processes jobs

**How:**
- Job has channel inputs (receive-only) and outputs (send-only)
- Worker runs in a goroutine, receives jobs, sends results
- Context propagates cancellation to all workers

### `types.go`

```go
package main

import (
	"context"   // Context for cancellation propagation
	"fmt"      // Formatted output
	"net/http" // HTTP client
	"time"     // Timeout handling
)

// ============================================================================
// JOB TYPE
// ============================================================================

// Job represents a unit of work to be processed.
// We use a struct because it groups related data.
//
// Why separate Job from Result?
// - Job is the INPUT (what to do)
// - Result is the OUTPUT (what happened)
// - Keeping them separate lets us retry failed jobs easily
//
// Topic 5 (Structs): Struct with named fields
type Job struct {
	ID      int           // Unique job identifier (for tracking)
	URL     string        // The URL to process
	Timeout time.Duration // Max time to spend on this job
}

// ============================================================================
// RESULT TYPE  
// ============================================================================

// JobResult holds the outcome of processing a job.
// Includes both success (Status) and failure (Err) information.
//
// Why include both?
// - Status = 0 means error occurred
// - Status > 0 means success (HTTP status code)
// - Caller can check either field depending on needs
type JobResult struct {
	JobID     int           // Which job this result is for
	URL      string        // Echo back the URL (for logging)
	Status   int           // HTTP status code (0 if error)
	Duration time.Duration // How long it took
	Err      error         // Error if failed (nil if success)
}

// ============================================================================
// WORKER TYPE
// ============================================================================

// Worker processes Jobs from a channel and sends Results.
// Contains all the "ingredients" needed to do its job.
//
// Why use channels in the struct?
// - jobs (<-chan *Job): receive-only channel for receiving work
// - results (chan<- JobResult): send-only channel for sending results
// - Directional channels prevent accidental misuse
//
// Topic 12 (Channels): Directional channel types
type Worker struct {
	ID      int             // Worker identifier (for logging)
	Jobs    <-chan *Job   // Receive jobs from here (read-only)
	Results chan<- JobResult // Send results to here (write-only)
	Context context.Context // Cancellation signal
	Group   *sync.WaitGroup // Track when this worker finishes
}

// ============================================================================
// WORKER CONSTRUCTOR
// ============================================================================

// NewWorker creates a new Worker with all dependencies injected.
// Returns *Worker (pointer) because Worker is large (contains channels).
//
// Why pass channels and context as parameters?
// - Constructor doesn't create them, caller creates
// - Worker receives ready-to-use channels
// - This is "dependency injection" for goroutines
//
// Topic 6 (Pointers): Returns pointer to avoid copying
// Topic 4 (DI): Dependencies passed in, not created internally
func NewWorker(id int, jobs <-chan *Job, results chan<- JobResult, ctx context.Context, wg *sync.WaitGroup) *Worker {
	return &Worker{
		ID:      id,
		Jobs:    jobs,    // Receive-only end
		Results: results,  // Send-only end
		Context: ctx,     // Cancellation from main
		Group:   wg,      // WaitGroup for tracking
	}
}

// ============================================================================
// WORKER PROCESSING LOOP
// ============================================================================

// Start begins the worker's processing loop.
// Runs in a goroutine — returns immediately.
//
// How it works:
// 1. Register with WaitGroup (tell main we're running)
// 2. Loop forever: receive job OR check context
// 3. When context cancelled OR channel closed, exit
//
// Topic 14 (WaitGroup): wg.Add(1) registers this goroutine
// Topic 11 (Goroutines): go func() starts concurrent execution
// Topic 13 (Context): <-ctx.Done() receives cancellation signal
// Topic 12 (Select): Multiplex between job receive and context done
func (w *Worker) Start() {
	w.Group.Add(1) // Register this goroutine with the WaitGroup
	go func() {
		// defer ensures wg.Done() runs even if we panic
		// This is CRITICAL — forgetting = goroutine leak!
		defer w.Group.Done()
		
		fmt.Printf("[worker-%d] Started\n", w.ID)

		// Infinite loop: keep processing until told to stop
		for {
			select {
			// Check if context was cancelled (shutdown signal)
			case <-w.Context.Done():
				fmt.Printf("[worker-%d] Stopped\n", w.ID)
				return // Exit the goroutine

			// Try to receive a job from the jobs channel
			case job, ok := <-w.Jobs:
				// ok = false means channel was closed (no more jobs)
				if !ok {
					fmt.Printf("[worker-%d] Stopped\n", w.ID)
					return // Exit gracefully
				}
				// We got a valid job, process it
				w.process(job)
			}
		}
	}()
}

// ============================================================================
// JOB PROCESSING
// ============================================================================

// process handles a single job: make HTTP request, send result.
// This is where the actual work happens.
//
// How it works:
// 1. Log start
// 2. Create HTTP request with context (for timeout/cancellation)
// 3. Make request
// 4. Send result (success or failure)
//
// Topic 12 (Select): Handled in process() itself
// Topic 8 (Error Handling): Proper error wrapping with %w
func (w *Worker) process(job *Job) {
	fmt.Printf("[worker-%d] Processing: %s\n", w.ID, job.URL)

	start := time.Now()

	// Create HTTP request with timeout baked into context
	// http.NewRequestWithContext returns error if URL is invalid
	// The request carries the context for cancellation/timeout
	req, err := http.NewRequestWithContext(w.Context, "GET", job.URL, nil)
	if err != nil {
		// Failed to create request (invalid URL?)
		// Send error result, don't block the worker
		w.Results <- JobResult{
			JobID: job.ID,
			URL:   job.URL,
			Err:   fmt.Errorf("create request: %w", err), // Wrap for error chain
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

> **Topics Used:** Context (Topic 13), Select (Topic 12), Channels (Topic 11)

### What / Why / How

**What:** Handle SIGINT/SIGTERM signals and gracefully shut down the worker pool.

**Why:**
- Ctrl+C (SIGINT) or container restart (SIGTERM) should complete current work
- Don't kill workers mid-job — that loses work
- Clean shutdown = wait for workers to finish + collect results

**How:**
1. Create channel to receive OS signals
2. Notify on SIGINT/SIGTERM
3. When signal received, cancel context
4. Workers see context cancelled → finish current job → exit

### Intuition: Signal Flow

```
OS sends SIGINT (Ctrl+C)
        │
        ▼
signal.Notify channel receives it
        │
        ▼
goroutine calls cancel()
        │
        ▼
ctx.Done() fires in ALL workers
        │
        ▼
Workers finish current job, exit gracefully
        │
        ▼
WaitGroup.Wait() returns
        │
        ▼
Main exits cleanly
```

### `shutdown.go`

```go
package main

import (
	"context" // Context for cancellation
	"fmt"    // Logging
	"os"     // Stderr for errors
	"os/signal" // Signal handling
	"syscall" // Signal constants
)

// ============================================================================
// SIGNAL SETUP
// ============================================================================

// SetupGracefulShutdown creates a context that cancels on SIGINT/SIGTERM.
// This is the STANDARD pattern for graceful shutdown in Go.
//
// How it works:
// 1. Create cancelable context
// 2. Create signal channel (buffered = 1)
// 3. Tell Go to deliver signals to our channel
// 4. Start goroutine to wait for signals
// 5. When signal arrives, call cancel()
//
// Why buffered channel?
// - OS sends signal even if nobody listening yet
// - Buffer of 1 prevents signal loss
// - If signal arrives before we read, it's buffered
//
// Topic 13 (Context): Cancellation propagates to all workers
// Topic 11 (Channels): Signal communication
func SetupGracefulShutdown() (context.Context, context.CancelFunc) {
	// Create context that can be cancelled
	// WithCancel returns (ctx, cancelFunc)
	// Calling cancel() sets ctx.Err() to context.Canceled
	ctx, cancel := context.WithCancel(context.Background())

	// Create buffered channel for signals
	// Size = 1 because we only care about "signal received"
	// Don't need to queue multiple signals
	sigCh := make(chan os.Signal, 1)

	// Tell Go to deliver these signals to our channel
	// SIGINT = Ctrl+C, SIGTERM = Docker/Kubernetes stop
	// This DOES NOT block - just registers the handler
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start goroutine to wait for signal
	// This runs independently, doesn't block main
	go func() {
		// <-sigCh blocks until signal arrives
		// Then we log and cancel the context
		sig := <-sigCh
		fmt.Printf("\nReceived signal: %v\n", sig)
		fmt.Println("Initiating graceful shutdown...")
		cancel() // This triggers ctx.Done() everywhere!
	}()

	return ctx, cancel
}

// ============================================================================
// RUN WITH SHUTDOWN
// ============================================================================

// RunWithGracefulShutdown runs the main function with graceful shutdown support.
// Handles setup and cleanup automatically.
//
// Why a wrapper?
// - Single entry point for the entire program
// - Handles both normal exit and signal exit
// - defer ensures context is cancelled if main exits early
func RunWithGracefulShutdown(fn func(ctx context.Context) error) {
	// Setup graceful shutdown
	ctx, cancel := SetupGracefulShutdown()
	
	// defer cancel() ensures context is cancelled on ANY exit
	// Even if fn() panics, cancel() runs first
	defer cancel()

	// Run the main function with our context
	// This is where actual work happens
	if err := fn(ctx); err != nil {
		// Print error to stderr (not stdout)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1) // Non-zero = error
	}
}
```

---

## 7. Step 4 — Result Collection

### What / Why / How

**What:** Collect results from workers and display a summary.

**Why:**
- Workers run concurrently, results come in any order
- Need to collect all results before exiting
- Summary shows success/failure counts

**How:**
- Separate goroutine collects results from channel
- Main waits for all jobs to be submitted + all results to be collected

### Intuition: Result Collection Flow

```
MAIN THREAD                    WORKER THREADS
===========                    ==============

Submit job ──────────────────► Worker receives job
Submit job ──────────────────► Worker processes
Submit job ──────────────────► 
         │                        │
         │                   Results ──────► Result channel
         │                   Results ──────►
         │                        │
         ▼                        ▼
   (wait for done)          (finish job)

COLLECTOR GOROUTINE:
  for result := range results {
      aggregate.Add(result)
  }
  aggregate.Display()
```

### `collector.go`

```go
package main

import (
	"fmt"   // Formatted output
	"time"  // Timing
)

// ============================================================================
// AGGREGATOR STRUCT
// ============================================================================

// ResultAggregator collects job results and displays a summary.
// Runs in its own goroutine to collect results without blocking workers.
//
// Why a separate struct?
// - Groups related functionality
// - Holds state (results slice, start time)
// - Methods for adding results and displaying summary
type ResultAggregator struct {
	results   []JobResult // All collected results
	startTime time.Time   // When aggregation started
}

// NewResultAggregator creates a new aggregator.
// Records start time for calculating total duration.
func NewResultAggregator() *ResultAggregator {
	return &ResultAggregator{
		// Pre-allocate slice assuming some results
		// make([]JobResult, 0) = empty slice, capacity unspecified
		results:   make([]JobResult, 0),
		startTime: time.Now(), // Record when we started
	}
}

// ============================================================================
// ADD RESULT
// ============================================================================

// Add includes a result in the aggregation.
// Called by main goroutine after receiving from results channel.
func (ra *ResultAggregator) Add(r JobResult) {
	// append returns new slice (if capacity exceeded)
	// We reassign back to ra.results
	ra.results = append(ra.results, r)
}

// ============================================================================
// DISPLAY SUMMARY
// ============================================================================

// Display prints a formatted summary of all results.
// Shows errors, status codes, and totals.
func (ra *ResultAggregator) Display() {
	// Calculate total duration
	duration := time.Since(ra.startTime)

	fmt.Println("\n--- Results ---")

	// =========================================================================
	// COLLECT ERRORS
	// =========================================================================
	// Group results by type: errors vs successes
	// Use map to count status codes
	byStatus := make(map[int]int) // status code -> count
	var errors []JobResult       // collect all errors

	// Single pass through results
	for _, r := range ra.results {
		if r.Err != nil {
			// This result has an error
			errors = append(errors, r)
		} else {
			// Success - count by status code
			byStatus[r.Status]++
		}
	}

	// =========================================================================
	// PRINT ERRORS
	// =========================================================================
	// Show all errors first (most important for debugging)
	if len(errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range errors {
			fmt.Printf("  - %s: %v\n", e.URL, e.Err)
		}
	}

	// =========================================================================
	// PRINT STATUS CODES
	// =========================================================================
	// Show breakdown by HTTP status
	if len(byStatus) > 0 {
		fmt.Println("\nStatus codes:")
		for status, count := range byStatus {
			fmt.Printf("  %d: %d URLs\n", status, count)
		}
	}

	// =========================================================================
	// PRINT SUMMARY
	// =========================================================================
	// Calculate totals
	success := len(ra.results) - len(errors)
	
	// Print final summary
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

## 5. Step 2 — Worker Pool Implementation

### What / Why / How

**What:** Create the pool that manages workers and distributes jobs.

**Why:**
- Pool manages the bounded concurrency (N workers, not unlimited)
- Pool creates channels and passes them to workers
- Pool handles starting/stopping workers

**How:**
1. Create buffered job channel
2. Create result channel
3. Create workers with channels
4. Workers pull from job channel concurrently

### Intuition: Why Bounded?

```
UNBOUNDED (BAD):
  for _, url := range urls {
      go fetch(url)  // 10,000 URLs = 10,000 goroutines!
  }
  
  Problem: 
  - Each goroutine = ~2KB stack
  - 10,000 URLs = 20MB memory
  - OS can't handle 10,000 concurrent network connections
  - Scheduler thrashing: too many runnable goroutines

BOUNDED (GOOD):
  jobs := make(chan *Job, 100)  // Buffer = 100 jobs max
  
  for i := 0; i < 10; i++ {   // Only 10 workers
      go worker(i, jobs)
  }
  
  Benefits:
  - 10 workers = ~20KB memory
  - Job queue in channel = no heap allocation per job
  - Backpressure: if jobs channel full, main blocks
  - System stays responsive under load
```

### `pool.go`

```go
package main

import (
	"context"       // Context for cancellation
	"fmt"          // Logging
	"sync"         // WaitGroup, Mutex
)

// ============================================================================
// POOL STRUCT
// ============================================================================

// Pool manages a set of workers and job distribution.
// Holds all the state needed to run the worker pool.
//
// Why put everything in a struct?
// - Groups related configuration
// - Single parameter to pass to functions
// - Can add methods for pool control
type Pool struct {
	Workers  int           // Number of workers to spawn
	Jobs     chan *Job    // Channel for distributing work
	Results  chan JobResult // Channel for collecting results
	Context  context.Context // Cancellation signal
	Cancel   context.CancelFunc // Function to call to cancel
	Group    *sync.WaitGroup // Track worker completion
}

// ============================================================================
// POOL CONSTRUCTOR  
// ============================================================================

// NewPool creates a new Pool with specified number of workers.
// Sets up channels and context for the pool.
//
// How it works:
// 1. Create job channel with buffer (backpressure)
// 2. Create result channel
// 3. Create context for cancellation
// 4. Create WaitGroup for tracking
//
// Channel buffer size:
// - Too small: workers idle, can't keep up
// - Too much: wasted memory
// - Rule of thumb: workers * 10 is usually good
func NewPool(workers int, jobBuffer int) *Pool {
	// Create context that can be cancelled
	// context.Background() is the root, WithCancel adds cancellation
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Pool{
		Workers:  workers,                    // N workers
		Jobs:     make(chan *Job, jobBuffer), // Buffered job queue
		Results:  make(chan JobResult, jobBuffer), // Buffered results
		Context:  ctx,                       // Cancellation
		Cancel:   cancel,                     // Call to stop
		Group:    &sync.WaitGroup{},          // Track workers
	}
}

// ============================================================================
// START WORKERS
// ============================================================================

// Start spawns all workers and begins processing.
// Each worker gets its own goroutine.
//
// How it works:
// 1. Loop N times (for N workers)
// 2. Create worker with channels and context
// 3. Call Start() to begin worker goroutine
// 4. Worker registers with WaitGroup internally
func (p *Pool) Start() {
	// Create N workers
	for i := 0; i < p.Workers; i++ {
		// Each worker receives:
		// - Unique ID (i)
		// - Jobs channel (p.Jobs) - shared by all workers
		// - Results channel (p.Results) - shared by all workers
		// - Context (p.Context) - shared by all workers
		// - WaitGroup (p.Group) - shared by all workers
		worker := NewWorker(i, p.Jobs, p.Results, p.Context, p.Group)
		
		// Start launches the worker goroutine
		// Returns immediately, worker runs in background
		worker.Start()
	}
}

// ============================================================================
// JOB SUBMISSION
// ============================================================================

// Submit adds a job to the pool's job queue.
// Blocks if job channel is full (backpressure).
//
// Why block?
// - If we didn't block, we'd lose jobs
// - Caller waits until there's room
// - This is natural backpressure!
func (p *Pool) Submit(job *Job) {
	// Sending to channel blocks if channel is full
	// This backpressure prevents overwhelming the system
	p.Jobs <- job
}

// ============================================================================
// SHUTDOWN
// ============================================================================

// Shutdown signals all workers to stop and waits for them to finish.
//
// How it works:
// 1. Cancel context (workers see ctx.Done())
// 2. Close job channel (workers see <-jobs, ok=false)
// 3. Wait for all workers to call wg.Done()
// 4. Close result channel (no more results coming)
func (p *Pool) Shutdown() {
	// Step 1: Cancel context
	// This triggers <-ctx.Done() in every worker
	fmt.Println("Shutting down workers...")
	p.Cancel()
	
	// Step 2: Close job channel
	// Workers receive zero-value from closed channel
	// This signals "no more jobs"
	close(p.Jobs)
	
	// Step 3: Wait for all workers to finish
	// wg.Wait() blocks until Add(1) count reaches 0
	// Each worker calls Done() when it exits
	p.Group.Wait()
	
	// Step 4: Close result channel
	// After all workers done, no more results
	close(p.Results)
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