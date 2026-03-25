# 18. Pipelines — Complete Deep Dive

> **Goal:** Master Go pipelines — chain stages of goroutines connected by channels. Each stage transforms data and passes it downstream.
>
> **Pipelines vs Worker Pools:** Worker pools distribute identical jobs to interchangeable workers (parallel processing). Pipelines chain sequential stages where each stage transforms data (stream processing). Use worker pools when all jobs are the same. Use pipelines when data flows through transformation stages (e.g., read → parse → validate → transform → write).

---
![Pipelines](../assets/17.png)

## Table of Contents

1. [What Is a Pipeline](#1-what-is-a-pipeline)
2. [Basic Pipeline](#2-basic-pipeline)
3. [Pipeline Stages](#3-pipeline-stages)
4. [Generator Pattern](#4-generator-pattern)
5. [Pipeline with Error Handling](#5-pipeline-with-error-handling)
6. [Pipeline with Context](#6-pipeline-with-context)
7. [Bounded Pipelines](#7-bounded-pipelines)
8. [Real-World Example: Log Processing](#8-real-world-example-log-processing)
9. [Common Pitfalls](#9-common-pitfalls)

---

## 1. What Is a Pipeline

A pipeline is a series of stages connected by channels. Each stage is a goroutine that:

1. **Receives** values from an upstream channel
2. **Transforms** them
3. **Sends** results to a downstream channel

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                          PIPELINE FLOW                                    │
  │                                                                          │
  │   ┌───────────────┐    ┌───────────────┐    ┌───────────────┐           │
  │   │    STAGE 1    │    │    STAGE 2    │    │    STAGE 3    │           │
  │   │               │    │               │    │               │           │
  │   │   gen()       │    │  double()     │    │  filter()     │           │
  │   │               │    │               │    │               │           │
  │   │  produces     │    │  multiplies   │    │  keeps evens  │           │
  │   │  values       │    │  by 2         │    │  only         │           │
  │   │               │    │               │    │               │           │
  │   │  goroutine    │    │  goroutine    │    │  goroutine    │           │
  │   └───────┬───────┘    └───────┬───────┘    └───────┬───────┘           │
  │           │                    │                    │                    │
  │           ▼                    ▼                    ▼                    │
  │      ┌─────────┐          ┌─────────┐          ┌─────────┐             │
  │      │channel1 │─────────►│channel2 │─────────►│channel3 │──► output  │
  │      │         │          │         │          │         │             │
  │      │ [1][2]  │          │ [2][4]  │          │ [2][4]  │             │
  │      │ [3][4]  │          │ [6][8]  │          │ [6][8]  │             │
  │      └─────────┘          └─────────┘          └─────────┘             │
  │                                                                          │
  │   Each channel connects two stages. Data flows LEFT → RIGHT.            │
  │   Each stage runs in its own goroutine.                                  │
  │                                                                          │
  └──────────────────────────────────────────────────────────────────────────┘
```

### Visual: Data Flow Through Pipeline

```
Step 1: gen() produces
        gen ──► [1] [2] [3] [4] [5] ──► channel1
        
Step 2: double() transforms  
        channel1 ──► [1] [2] [3] [4] [5]
                      │
                      ▼ (×2)
        channel2 ──► [2] [4] [6] [8] [10]

Step 3: filter() filters
        channel2 ──► [2] [4] [6] [8] [10]
                      │
                      ▼ (keep even)
        channel3 ──► [2] [4] [6] [8] [10]

Step 4: sink() consumes
        channel3 ──► [2] [4] [6] [8] [10]
                      │
                      ▼ (print)
        output ──► 2
                   4
                   6
                   8
                   10
```

### Properties

- Each stage runs in its own goroutine
- Stages are **composable** — rearrange, add, remove stages
- Upstream closes its output channel when done → downstream sees close → exits
- **Bounded memory** — doesn't load entire dataset

---

## 2. Basic Pipeline

```go
func main() {
    // Stage 1: Generate
    gen := func(nums ...int) <-chan int {
        out := make(chan int)
        go func() {
            defer close(out)
            for _, n := range nums {
                out <- n
            }
        }()
        return out
    }

    // Stage 2: Transform (multiply by 2)
    double := func(in <-chan int) <-chan int {
        out := make(chan int)
        go func() {
            defer close(out)
            for n := range in {
                out <- n * 2
            }
        }()
        return out
    }

    // Stage 3: Transform (add 1)
    addOne := func(in <-chan int) <-chan int {
        out := make(chan int)
        go func() {
            defer close(out)
            for n := range in {
                out <- n + 1
            }
        }()
        return out
    }

    // Build pipeline
    for n := range addOne(double(gen(1, 2, 3, 4, 5))) {
        fmt.Println(n)
    }
    // Output: 3 5 7 9 11
    // (1*2+1, 2*2+1, 3*2+1, 4*2+1, 5*2+1)
}
```

### Data Flow

```
gen(1,2,3,4,5) → [1,2,3,4,5] → double → [2,4,6,8,10] → addOne → [3,5,7,9,11]
```

---

## 3. Pipeline Stages

### Stage Template

Every pipeline stage follows the same pattern:

```go
func stageName(in <-chan InputType) <-chan OutputType {
    out := make(chan OutputType)
    go func() {
        defer close(out) // Always close output when done
        for val := range in {
            result := transform(val)
            out <- result
        }
    }()
    return out
}
```

### Filter Stage

```go
func filter(in <-chan int, predicate func(int) bool) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            if predicate(n) {
                out <- n
            }
        }
    }()
    return out
}

// Usage: filter even numbers
evens := filter(input, func(n int) bool { return n%2 == 0 })
```

### Batch Stage

```go
func batch(in <-chan int, size int) <-chan []int {
    out := make(chan []int)
    go func() {
        defer close(out)
        buf := make([]int, 0, size)
        for n := range in {
            buf = append(buf, n)
            if len(buf) == size {
                out <- buf
                buf = make([]int, 0, size)
            }
        }
        if len(buf) > 0 {
            out <- buf
        }
    }()
    return out
}
```

### Map Stage

```go
func mapStage[T, U any](in <-chan T, fn func(T) U) <-chan U {
    out := make(chan U)
    go func() {
        defer close(out)
        for val := range in {
            out <- fn(val)
        }
    }()
    return out
}
```

---

## 4. Generator Pattern

A generator is the **first stage** — it produces values and closes when done.

### Finite Generator

```go
func generate(nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            out <- n
        }
    }()
    return out
}
```

### Infinite Generator

```go
func counter(start int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for i := start; ; i++ {
            select {
            case out <- i:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}
```

### File Line Generator

```go
func readLines(path string) (<-chan string, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }

    out := make(chan string)
    go func() {
        defer close(out)
        defer f.Close()

        scanner := bufio.NewScanner(f)
        for scanner.Scan() {
            out <- scanner.Text()
        }
    }()

    return out, nil
}
```

---

## 5. Pipeline with Error Handling

### Result Type Pattern

```go
type Result[T any] struct {
    Value T
    Err   error
}

func parseLines(in <-chan string) <-chan Result[int] {
    out := make(chan Result[int])
    go func() {
        defer close(out)
        for line := range in {
            n, err := strconv.Atoi(line)
            out <- Result[int]{Value: n, Err: err}
        }
    }()
    return out
}

func collectResults[T any](in <-chan Result[T]) ([]T, error) {
    var results []T
    for r := range in {
        if r.Err != nil {
            return nil, r.Err // Fail fast
        }
        results = append(results, r.Value)
    }
    return results, nil
}
```

### Separate Error Channel

```go
type Stage[T any] struct {
    Out <-chan T
    Err <-chan error
}

func parseLines(in <-chan string) Stage[int] {
    out := make(chan int)
    errs := make(chan error)

    go func() {
        defer close(out)
        defer close(errs)

        for line := range in {
            n, err := strconv.Atoi(line)
            if err != nil {
                errs <- fmt.Errorf("parse %q: %w", line, err)
                continue
            }
            out <- n
        }
    }()

    return Stage[int]{Out: out, Err: errs}
}
```

---

## 6. Pipeline with Context

Every stage respects cancellation. When a downstream stage cancels, upstream stops producing.

```go
func stage(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for {
            select {
            case <-ctx.Done():
                return
            case val, ok := <-in:
                if !ok {
                    return
                }
                result, err := process(ctx, val)
                if err != nil {
                    continue // Skip errors
                }
                select {
                case out <- result:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()
    return out
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    input := generate(ctx, 1, 2, 3, 4, 5)
    doubled := double(ctx, input)
    filtered := filter(ctx, doubled, func(n int) bool { return n > 5 })

    for n := range filtered {
        fmt.Println(n)
    }
}
```

---

## 7. Bounded Pipelines

### Fan-Out Within Pipeline Stages

```go
func parallelStage(ctx context.Context, in <-chan int, workers int) <-chan int {
    out := make(chan int)

    var wg sync.WaitGroup
    wg.Add(workers)

    for i := 0; i < workers; i++ {
        go func() {
            defer wg.Done()
            for {
                select {
                case <-ctx.Done():
                    return
                case val, ok := <-in:
                    if !ok {
                        return
                    }
                    result := slowProcess(val)
                    select {
                    case out <- result:
                    case <-ctx.Done():
                        return
                    }
                }
            }
        }()
    }

    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

```
           ┌── Worker 1 ──┐
gen() ────►├── Worker 2 ──├───► filter() ──► sink()
           └── Worker 3 ──┘
```

---

## 8. Real-World Example: Log Processing

```go
// Stage 1: Read log lines from file
func readLogs(ctx context.Context, path string) (<-chan string, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }

    out := make(chan string, 100)
    go func() {
        defer close(out)
        defer f.Close()

        scanner := bufio.NewScanner(f)
        for scanner.Scan() {
            select {
            case out <- scanner.Text():
            case <-ctx.Done():
                return
            }
        }
    }()
    return out, nil
}

// Stage 2: Parse log entries
type LogEntry struct {
    Timestamp time.Time
    Level     string
    Message   string
}

func parseLogs(ctx context.Context, in <-chan string) <-chan LogEntry {
    out := make(chan LogEntry, 100)
    go func() {
        defer close(out)
        for line := range in {
            entry, err := parseLogLine(line)
            if err != nil {
                continue
            }
            select {
            case out <- entry:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Stage 3: Filter by level
func filterByLevel(ctx context.Context, in <-chan LogEntry, level string) <-chan LogEntry {
    out := make(chan LogEntry, 100)
    go func() {
        defer close(out)
        for entry := range in {
            if entry.Level == level {
                select {
                case out <- entry:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()
    return out
}

// Stage 4: Format output
func formatLogs(ctx context.Context, in <-chan LogEntry) <-chan string {
    out := make(chan string, 100)
    go func() {
        defer close(out)
        for entry := range in {
            line := fmt.Sprintf("[%s] %s: %s",
                entry.Timestamp.Format(time.RFC3339),
                entry.Level,
                entry.Message)
            select {
            case out <- line:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Build and run pipeline
func processLogs(ctx context.Context, path string) error {
    lines, err := readLogs(ctx, path)
    if err != nil {
        return err
    }

    entries := parseLogs(ctx, lines)
    errors := filterByLevel(ctx, entries, "ERROR")
    formatted := formatLogs(ctx, errors)

    for line := range formatted {
        fmt.Println(line)
    }
    return nil
}
```

---

## 9. Common Pitfalls

| Pitfall | Problem | Fix |
|---------|---------|-----|
| Forgetting `close(out)` | Downstream blocks forever | Always `defer close(out)` |
| No context | Can't cancel pipeline | Pass `ctx` to every stage |
| Unbuffered channels | Unnecessary blocking | Use small buffer (e.g., 100) |
| Blocking on send to full channel | Deadlock if no consumer | Use `select` with `ctx.Done()` |
| Leaking goroutines on error | Memory leak | Check `ctx.Done()` in every loop |
| Too many stages | Overhead exceeds benefit | Combine simple stages |

---

## 10. Production Patterns

### Pipeline with Retry

```go
func withRetry[T any](in <-chan T, maxRetries int, delay time.Duration) <-chan T {
    out := make(chan T, 10)

    go func() {
        defer close(out)
        for val := range in {
            var err error
            for attempt := 0; attempt < maxRetries; attempt++ {
                if err = process(val); err == nil {
                    break
                }
                time.Sleep(delay * time.Duration(attempt+1))
            }
            if err == nil {
                select {
                case out <- val:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()
    return out
}
```

### Pipeline with Aggregation

```go
type Aggregator[T any] struct {
    buffer []T
    size   int
    flush  func([]T)
}

func (a *Aggregator) Add(item T) {
    a.buffer = append(a.buffer, item)
    if len(a.buffer) >= a.size {
        a.Flush()
    }
}

func (a *Aggregator) Flush() {
    if len(a.buffer) > 0 {
        a.flush(a.buffer)
        a.buffer = a.buffer[:0]
    }
}

func aggregateStage[T any](in <-chan T, size int, flushFn func([]T)) <-chan struct{} {
    out := make(chan struct{})
    agg := Aggregator[T]{size: size, flush: flushFn}

    go func() {
        defer close(out)
        for {
            select {
            case <-ctx.Done():
                agg.Flush()
                return
            case item, ok := <-in:
                if !ok {
                    agg.Flush()
                    return
                }
                agg.Add(item)
            }
        }
    }()
    return out
}
```

### Pipeline with Metrics

```go
type StageMetrics struct {
    Received    atomic.Int64
    Processed   atomic.Int64
    Errors      atomic.Int64
    Latency     atomic.Int64 // total nanoseconds
}

func metricsStage[T any](in <-chan T, name string, processFn func(T) (T, error)) (<-chan T, *StageMetrics) {
    out := make(chan T, 100)
    m := &StageMetrics{}

    go func() {
        defer close(out)
        for {
            select {
            case <-ctx.Done():
                return
            case item, ok := <-in:
                if !ok {
                    return
                }
                m.Received.Add(1)

                start := time.Now()
                result, err := processFn(item)
                elapsed := time.Since(start)

                m.Latency.Add(elapsed.Nanoseconds())

                if err != nil {
                    m.Errors.Add(1)
                    continue
                }

                m.Processed.Add(1)

                select {
                case out <- result:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()
    return out, m
}
```

### Full Pipeline with All Features

```go
type Pipeline struct {
    ctx    context.Context
    cancel context.CancelFunc
    stages []Stage
    wg     sync.WaitGroup
}

type Stage struct {
    Name   string
    In     interface{}
    Out    interface{}
    Metrics *StageMetrics
}

func NewPipeline(bufferSize int) *Pipeline {
    ctx, cancel := context.WithCancel(context.Background())
    return &Pipeline{
        ctx:    ctx,
        cancel: cancel,
    }
}

func (p *Pipeline) AddStage(name string, fn func(context.Context, <-chan interface{}) <-chan interface{}) *Pipeline {
    stage := Stage{Name: name}
    p.stages = append(p.stages, stage)
    return p
}

func (p *Pipeline) Run(input <-chan interface{}) (<-chan interface{}, error) {
    current := input
    var prevStage *Stage

    for i := range p.stages {
        stage := &p.stages[i]

        if i == 0 {
            stage.In = input
        } else {
            stage.In = prevStage.Out
        }

        out := make(chan interface{}, 100)
        stage.Out = out

        p.wg.Add(1)
        go func(s *Stage) {
            defer p.wg.Done()
            defer close(s.Out.(chan interface{}))

            in := s.In.(<-chan interface{})
            for {
                select {
                case <-p.ctx.Done():
                    return
                case item, ok := <-in:
                    if !ok {
                        return
                    }
                    // Process item
                    select {
                    case s.Out.(chan interface{}) <- item:
                    case <-p.ctx.Done():
                        return
                    }
                }
            }
        }(stage)

        prevStage = stage
    }

    return p.stages[len(p.stages)-1].Out.(<-chan interface{}), nil
}

func (p *Pipeline) Stop() {
    p.cancel()
    p.wg.Wait()
}
```

---

## 11. Testing Pipelines

```go
func TestPipeline(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Create input
    input := make(chan int, 10)
    for i := 1; i <= 5; i++ {
        input <- i
    }
    close(input)

    // Run pipeline
    output := doubleStage(ctx, input)
    output = addOneStage(ctx, output)

    // Collect results
    var results []int
    for val := range output {
        results = append(results, val)
    }

    // Verify
    expected := []int{3, 5, 7, 9, 11}
    if !reflect.DeepEqual(results, expected) {
        t.Errorf("expected %v, got %v", expected, results)
    }
}
```
| No backpressure | OOM on slow consumer | Use bounded buffers |
