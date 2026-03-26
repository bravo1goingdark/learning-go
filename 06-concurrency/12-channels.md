# 12. Channels — Complete Deep Dive

> **Goal:** Master channels — unbuffered, buffered, directional types, and every production pattern. Channels are Go's way to communicate between goroutines.

---
![Channels](../assets/11.png)
## Table of Contents

1. [Channel Basics](#1-channel-basics-core)
2. [Unbuffered Channels](#2-unbuffered-channels-core)
3. [Buffered Channels](#3-buffered-channels-core)
4. [Directional Channels](#4-directional-channels-core)
5. [Closing Channels](#5-closing-channels-core)
6. [Range Over Channel](#6-range-over-channel-core)
7. [Nil Channels](#7-nil-channels-core)
8. [Channel Internals](#8-channel-internals-internals)
9. [Common Patterns](#9-common-patterns-production)
10. [Common Pitfalls](#10-common-pitfalls-core)

---

## 1. Channel Basics [CORE]

Goroutines can share memory, but unprotected shared memory causes data races. Channels provide a safer alternative: instead of sharing memory and locking it, you **communicate** values between goroutines. This follows Go's philosophy: *"Do not communicate by sharing memory; instead, share memory by communicating."*

A channel is a **typed conduit** for sending and receiving values between goroutines.

```go
ch := make(chan int)       // Unbuffered channel of int
ch := make(chan string, 5) // Buffered channel, capacity 5
```

### Operations

```go
ch <- value    // Send
val := <-ch    // Receive
val, ok := <-ch // Receive with open check
close(ch)      // Close channel
```

### Rules

- Sends and receives **block** until the other side is ready
- Only the **sender** should close a channel
- Receivers can check if a channel is closed: `val, ok := <-ch`
- Closing a closed channel **panics**
- Sending to a closed channel **panics**
- Receiving from a closed channel returns the zero value

---

## 2. Unbuffered Channels [CORE]

Unbuffered channels provide **synchronous** communication. Every send blocks until a receive happens.

### Synchronization

```
  UNBUFFERED CHANNEL SYNCHRONIZATION:

  ┌──────────┐                                ┌──────────┐
  │  Sender   │                                │ Receiver  │
  └─────┬────┘                                └─────┬────┘
        │                                            │
        │  ch <- "hello"                             │
        │  ┌────────────────────┐                    │
        │  │     BLOCKED        │  ◄── waiting       │
        │  │  for receiver      │      for recv      │
        │  └─────────┬──────────┘                    │
        │            │                               │
        │            │     msg := <-ch               │
        │            │     ┌────────────────────┐    │
        │            └────►│     RECEIVED       │    │
        │                  │  (unblocks sender) │    │
        │                  └────────────────────┘    │
        │                                            │
        ▼  ◄── both goroutines continue ──►          ▼
```

```go
func main() {
    ch := make(chan string) // Unbuffered

    go func() {
        ch <- "hello" // Blocks until main receives
        fmt.Println("sent!")
    }()

    msg := <-ch // Blocks until goroutine sends
    fmt.Println(msg)
    // Output:
    // hello
    // sent!
}
```

### Use Case: Signal Synchronization

```go
func main() {
    done := make(chan struct{}) // Unbuffered, zero-size type

    go func() {
        doWork()
        done <- struct{}{} // Signal completion
    }()

    <-done // Wait for completion
}
```

---

## 3. Buffered Channels [CORE]

**When to choose buffered vs unbuffered:** Use unbuffered when you need synchronization — a handshake where sender and receiver must meet (e.g., signaling completion). Use buffered when the producer and consumer run at different speeds, or when you want to decouple timing. The buffer size = how many items can be produced before the consumer must catch up. If the consumer is slower than the producer and the buffer fills, the producer blocks.

Buffered channels allow sends **without blocking** until the buffer is full.

### Behavior

```go
ch := make(chan int, 3) // Buffer size 3

ch <- 1 // Does NOT block — buffer has space
ch <- 2 // Does NOT block
ch <- 3 // Does NOT block
ch <- 4 // BLOCKS — buffer is full, waiting for receiver
```

### Buffer States

```
Capacity: 3

make(chan int, 3)       → [  ][  ][  ]  empty, len=0 cap=3
ch <- 1                  → [1 ][  ][  ]  len=1 cap=3
ch <- 2                  → [1 ][2 ][  ]  len=2 cap=3
ch <- 3                  → [1 ][2 ][3 ]  len=3 cap=3
ch <- 4                  → BLOCKS
<-ch                     → [2 ][3 ][  ]  len=2 cap=3, unblocks sender
```

### Use Case: Decouple Producer/Consumer Speed

```go
func main() {
    results := make(chan int, 10) // Buffer absorbs bursts

    go producer(results)

    for r := range results {
        fmt.Println(r)
    }
}

func producer(ch chan<- int) {
    for i := 0; i < 100; i++ {
        ch <- i // Won't block if buffer has space
    }
    close(ch)
}
```

---

## Visual: Buffered Channel States

### Channel Internal Structure

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                      CHANNEL INTERNAL STRUCTURE (hchan)                   │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   hchan struct:                                                          │
  │   ┌──────────────┬──────────────┬──────────────┐                         │
  │   │   qcount     │  dataqsiz    │   closed     │  ◄── counters/flags    │
  │   │ (items in Q) │  (capacity)  │ (is closed?) │                         │
  │   ├──────────────┼──────────────┼──────────────┤                         │
  │   │   sendx      │   recvx     │    lock      │                         │
  │   │ (send index) │ (recv index) │   (mutex)    │                         │
  │   └──────────────┴──────┬───────┴──────────────┘                         │
  │                         │                                                 │
  │                         ▼                                                 │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  buf  ──  RING BUFFER (circular queue)                            │  │
  │   │                                                                    │  │
  │   │    recvx                        sendx                             │  │
  │   │      ▼                            ▼                                │  │
  │   │   ┌──────┬──────┬──────┬──────┬──────┐                            │  │
  │   │   │  30  │  40  │      │  10  │  20  │                            │  │
  │   │   └──────┴──────┴──────┴──────┴──────┘                            │  │
  │   │    [idx0] [idx1] [idx2] [idx3] [idx4]                              │  │
  │   │    ◄─── unread ───►  ◄── empty ──►                                │  │
  │   └───────────────────────────────────────────────────────────────────┘  │
  │                                                                           │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  sendq ── blocked senders queue    recvq ── blocked receivers Q  │  │
  │   │  [G-wait1] [G-wait2] ...           [G-waitA] [G-waitB] ...      │  │
  │   └───────────────────────────────────────────────────────────────────┘  │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

### Visual: Buffered Channel Operations

```
make(chan int, 3)  →  Buffer: [   ][   ][   ]   qcount=0  sendx=0  recvx=0

ch <- 1            →  Buffer: [ 1 ][   ][   ]   qcount=1  sendx=1  recvx=0
                                      ↑
                                    write here

ch <- 2            →  Buffer: [ 1 ][ 2 ][   ]   qcount=2  sendx=2
                                      ↑           ↑
                                    write     read here

ch <- 3            →  Buffer: [ 1 ][ 2 ][ 3 ]   qcount=3  sendx=3
                                              ↑
                                           write here (FULL!)

ch <- 4            →  BLOCKS! Buffer full
                    →  sender added to sendq queue

<-ch               →  Read: 1
                    →  Buffer: [ 2 ][ 3 ][   ]   qcount=2  sendx=3  recvx=1
                    →  wakes up blocked sender from sendq
                    →  sender writes 4 at index 3:
                    →  Buffer: [ 2 ][ 3 ][ 4 ]   qcount=3  sendx=4  recvx=1

<-ch               →  Read: 2
                    →  Buffer: [ 3 ][ 4 ][   ]   qcount=2  sendx=4  recvx=2

<-ch               →  Read: 3
                    →  Buffer: [ 4 ][   ][   ]   qcount=1  sendx=4  recvx=3

<-ch               →  Read: 4
                    →  Buffer: [   ][   ][   ]   qcount=0  sendx=4  recvx=4
```

### Visual: Unbuffered vs Buffered

```
  UNBUFFERED (capacity = 0):

  ┌──────────┐                                ┌──────────┐
  │  Sender   │                                │ Receiver  │
  └─────┬────┘                                └─────┬────┘
        │                                            │
        │  send ──► ┌────────────────┐               │
        │           │    BLOCKED     │──────────►    │
        │           │ (no buffer!)   │    receive    │
        │           └────────────────┘               │
        │                                            │
        ▼  ◄── handshake complete, both continue ►   ▼


  BUFFERED (capacity = 3):

  ┌──────────┐    ┌────────────────────────┐    ┌──────────┐
  │  Sender   │    │      BUFFER (cap=3)    │    │ Receiver  │
  └─────┬────┘    │  ┌──────┬──────┬──────┐│    └─────┬────┘
        │         │  │  1   │  2   │  3   ││          │
        │  send ──┼─►└──────┴──────┴──────┘┼──► recv  │
        │         │  (blocks if full)      │          │
        │         └────────────────────────┘          │
        ▼                                             ▼
```

### Visual: Channel Close States

```go
ch := make(chan int, 3)

// Before close
ch <- 1
ch <- 2
// Buffer: [1][2][ ]

close(ch)

// After close
// Buffer: [1][2][ ]   closed = 1 (true)
//
// Receiving from closed channel:
val, ok := <-ch  // val=1, ok=true  (still returns values)
val, ok := <-ch  // val=2, ok=true
val, ok := <-ch  // val=0, ok=false (channel is now empty & closed)
val, ok := <-ch  // val=0, ok=false (always returns zero value, ok=false)

// Sending to closed channel PANICS:
// ch <- 3  // panic: send on closed channel
```

---

## 4. Directional Channels [CORE]

Restrict channel to send-only or receive-only at the **type level**.

```go
chan int        // Bidirectional — can send AND receive
chan<- int      // Send-only — can only send
<-chan int      // Receive-only — can only receive
```

### Function Signatures

```go
// Producer: only sends
func producer(out chan<- int) {
    for i := 0; i < 10; i++ {
        out <- i
    }
    close(out)
}

// Consumer: only receives
func consumer(in <-chan int) {
    for val := range in {
        fmt.Println(val)
    }
}

func main() {
    ch := make(chan int, 5)

    go producer(ch)  // chan int → chan<- int (widening OK)
    consumer(ch)     // chan int → <-chan int (widening OK)
}
```

### Why Directional Types?

```go
// Compile error — prevents bugs
func bad(out chan<- int) {
    val := out // ERROR: cannot receive from send-only channel
}

// Compile error — prevents bugs
func bad2(in <-chan int) {
    in <- 42 // ERROR: cannot send to receive-only channel
}
```

| Direction | Syntax | Can Send | Can Receive |
|-----------|--------|----------|-------------|
| Bidirectional | `chan T` | Yes | Yes |
| Send-only | `chan<- T` | Yes | No |
| Receive-only | `<-chan T` | No | Yes |

---

## 5. Closing Channels [CORE]

### Rules

| Action | Closed? | Result |
|--------|---------|--------|
| `close(ch)` | Not closed | Closes channel |
| `close(ch)` | Already closed | **PANIC** |
| `ch <- v` | Closed | **PANIC** |
| `v := <-ch` | Closed | Returns zero value, `ok = false` |

### Only Sender Closes

```go
func producer(ch chan<- int) {
    for i := 0; i < 5; i++ {
        ch <- i
    }
    close(ch) // Sender closes — never the receiver
}

func consumer(ch <-chan int) {
    for {
        val, ok := <-ch
        if !ok {
            break // Channel closed
        }
        fmt.Println(val)
    }
}
```

### Detecting Close

```go
// Method 1: range (preferred)
for val := range ch {
    fmt.Println(val)
}

// Method 2: ok idiom
val, ok := <-ch
if !ok {
    // channel is closed
}

// Method 3: select
select {
case val, ok := <-ch:
    if !ok {
        // closed
    }
}
```

---

## 6. Range Over Channel [CORE]

`range` on a channel receives values until the channel is **closed**.

```go
func main() {
    ch := make(chan int, 5)

    go func() {
        for i := 0; i < 5; i++ {
            ch <- i
        }
        close(ch) // Must close, or range blocks forever
    }()

    for val := range ch {
        fmt.Println(val)
    }
    // Output: 0 1 2 3 4
}
```

**Without `close(ch)`, `range` blocks forever** — waiting for more values that never come.

---

## 7. Nil Channels [CORE]

A nil channel is the **zero value** of a channel. Operations on it **block forever**.

```go
var ch chan int // nil channel

ch <- 1   // Blocks forever
<-ch      // Blocks forever
close(ch) // PANIC: close of nil channel
```

### Use Case: Disable a Case in Select

```go
func process(ch1, ch2 <-chan int, done chan<- struct{}) {
    for {
        select {
        case v := <-ch1:
            if v == -1 {
                ch1 = nil // Disable this case forever
            }
            fmt.Println("ch1:", v)
        case v := <-ch2:
            fmt.Println("ch2:", v)
        case <-time.After(time.Second):
            done <- struct{}{}
            return
        }
    }
}
```

---

## 8. Channel Internals [INTERNALS]

> ⏭️ **First pass? Skip this section.** This covers Go runtime internals. Come back when curious about how Go works under the hood.

> **Connection to Topic 3 (Slices):** Both slices and buffered channels use a **ring buffer** internally. A slice's backing array is accessed via index; a channel's `buf` uses `sendx` and `recvx` indices that wrap around when they reach `dataqsiz`. Understanding one helps you understand the other — the key difference is that channels add locking and goroutine queues on top.

### hchan struct (simplified)

```go
type hchan struct {
    qcount   uint      // Total data in queue
    dataqsiz uint      // Buffer size (0 = unbuffered)
    buf      unsafe.Pointer // Ring buffer
    elemsize uint16
    closed   uint32
    sendx    uint      // Send index
    recvx    uint      // Receive index
    recvq    waitq     // Blocked receivers
    sendq    waitq     // Blocked senders
    lock     mutex     // Protects all fields
}
```

### Unbuffered Channel Flow

```
Sender                         Receiver
  │                              │
  │  lock                        │
  │  recvq not empty?            │
  │  → copy data to receiver     │
  │  → wake receiver             │
  │  unlock                      │
  │                              │
  │  OR                          │
  │                              │
  │  recvq empty?                │
  │  → enqueue in sendq          │
  │  → park (block)              │
  │  ... waiting ...             │
  │  ... receiver arrives ...    │
  │  → copy data                 │
  │  → wake sender               │
```

### Buffered Channel Flow

```
Send:
  buf not full?  → write to buf[sendx], sendx++, qcount++
  buf full?      → enqueue sender in sendq, park

Receive:
  buf not empty? → read from buf[recvx], recvx++, qcount--
  buf empty?     → enqueue receiver in recvq, park
```

---

## 9. Common Patterns [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

### Signal-Only (Done Channel)

```go
done := make(chan struct{})

go func() {
    doWork()
    done <- struct{}{}
}()

<-done
```

### Fan-Out Signal (Close Broadcasts to All)

```go
stop := make(chan struct{})

for i := 0; i < 5; i++ {
    go func(id int) {
        for {
            select {
            case <-stop:
                fmt.Printf("worker %d stopping\n", id)
                return
            default:
                // do work
            }
        }
    }(i)
}

time.Sleep(time.Second)
close(stop) // All goroutines receive the close signal
time.Sleep(time.Millisecond)
```

### Request-Response

```go
type Request struct {
    Data    int
    Reply   chan int // Embedded reply channel
}

func handler(reqs <-chan Request) {
    for req := range reqs {
        result := process(req.Data)
        req.Reply <- result // Send back to caller
    }
}

func main() {
    reqs := make(chan Request)

    go handler(reqs)

    reply := make(chan int)
    reqs <- Request{Data: 42, Reply: reply}
    result := <-reply
}
```

---

## 10. Common Pitfalls [CORE]

| Pitfall | Problem | Fix |
|---------|---------|-----|
| Receiver closes channel | Breaks sender contract | Only sender closes |
| Close twice | Panic | Track ownership clearly |
| Send to closed channel | Panic | Ensure sender done before close |
| Forgetting to close | `range` blocks forever | Close when done sending |
| Using channel as mutex | Overkill for simple state | Use `sync.Mutex` |
| Unbuffered when buffered needed | Unnecessary blocking | Choose buffer size wisely |
| Too-large buffer | Hides backpressure issues | Keep buffer small |

---

## 11. Production Patterns [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

### Broker Pattern with Multiple Producers

```go
type Broker struct {
    jobs   chan Job
    result chan Result
    quit   chan struct{}
}

func NewBroker(bufferSize int) *Broker {
    return &Broker{
        jobs:   make(chan Job, bufferSize),
        result: make(chan Result, bufferSize),
        quit:   make(chan struct{}),
    }
}

func (b *Broker) Submit(job Job) error {
    select {
    case b.jobs <- job:
        return nil
    case <-b.quit:
        return errors.New("broker stopped")
    }
}

func (b *Broker) Start(workers int, processFn func(Job) Result) {
    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for {
                select {
                case <-b.quit:
                    return
                case job := <-b.jobs:
                    b.result <- processFn(job)
                }
            }
        }(i)
    }
    wg.Wait()
}

func (b *Broker) Stop() {
    close(b.quit)
}

func (b *Broker) Results() <-chan Result {
    return b.result
}
```

### Channel Pool (Object Pool for Channels)

```go
type ChannelPool struct {
    get    chan chan int
    return chan chan int
    size   int
}

func NewChannelPool(size int) *ChannelPool {
    pool := &ChannelPool{
        get:    make(chan chan int),
        return: make(chan chan int),
        size:   size,
    }

    // Pre-create channels
    for i := 0; i < size; i++ {
        pool.get <- make(chan int, 1)
    }

    return pool
}

func (p *ChannelPool) Get() chan int {
    return <-p.get
}

func (p *ChannelPool) Put(ch chan int) {
    // Drain channel before returning
    for {
        select {
        case <-ch:
        default:
            p.get <- ch
            return
        }
    }
}
```

### Debounced Channel

```go
func debounce(input <-chan func(), delay time.Duration) <-chan func() {
    output := make(chan func())
    var pending func()

    go func() {
        var timer *time.Timer
        for {
            // Wait for new item or pending timeout
            if pending != nil {
                if timer == nil {
                    timer = time.AfterFunc(delay, func() {
                        output <- pending
                        pending = nil
                    })
                } else {
                    timer.Reset(delay)
                }
            }

            item, ok := <-input
            if !ok {
                if timer != nil {
                    timer.Stop()
                }
                close(output)
                return
            }
            pending = item
        }
    }()
    return output
}
```

### Throttled Channel

```go
func throttle(input <-chan Request, rate int) <-chan Request {
    output := make(chan Request)

    go func() {
        ticker := time.NewTicker(time.Second / time.Duration(rate))
        defer ticker.Stop()

        var q []Request
        for {
            select {
            case req := <-input:
                q = append(q, req)
            case <-ticker.C:
                if len(q) > 0 {
                    output <- q[0]
                    q = q[1:]
                }
            }
        }
    }()
    return output
}
```

### Split Channel (Fan-Out)

```go
func split(input <-chan T) (<-chan T, <-chan T) {
    out1 := make(chan T)
    out2 := make(chan T)

    go func() {
        defer close(out1)
        defer close(out2)

        i := 0
        for v := range input {
            if i%2 == 0 {
                out1 <- v
            } else {
                out2 <- v
            }
            i++
        }
    }()

    return out1, out2
}
```

---

## 12. Buffer Size Guidelines [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

### Rule of Thumb

| Scenario | Recommended Buffer |
|----------|-------------------|
| Producer faster than consumer | Buffer = consumer rate × latency |
| Producer slower than consumer | Buffer = 1 (unbuffered or small) |
| Unknown/balanced | Buffer = 10-100 |
| Workers with bounded queue | Worker queue = capacity |

### Calculate Backpressure

```go
func calculateBuffer(opsPerSec float64, latency time.Duration) int {
    // buffer = ops_per_second * latency_seconds * safety_factor
    buffer := int(opsPerSec * latency.Seconds() * 2)
    if buffer < 1 {
        buffer = 1
    }
    return buffer
}

// Example: 100 ops/sec, 50ms latency = 100 * 0.05 * 2 = 10
buffer := calculateBuffer(100, 50*time.Millisecond) // 10
```

---

## 13. Testing Channels [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing Topics 11-16.

### Deterministic Testing with time Package

```go
func TestChannelTimeout(t *testing.T) {
    ch := make(chan string, 1)

    select {
    case ch <- "hello":
        t.Fatal("should not receive")
    case <-time.After(10 * time.Millisecond):
        // Expected timeout
    }
}
```

### Race Condition Detection

```bash
go test -race ./...
go run -race main.go
```

### Chaos Testing

```go
func TestChannelChaos(t *testing.T) {
    for i := 0; i < 1000; i++ {
        ch := make(chan int, 1)
        var wg sync.WaitGroup
        wg.Add(2)

        go func() {
            defer wg.Done()
            ch <- 1
        }()

        go func() {
            defer wg.Done()
            ch <- 2
        }()

        wg.Wait()
        // With race detector, this will catch data races
    }
}
```

---

## Exercises

### Exercise 1: Producer-Consumer ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create one goroutine that sends the integers 0 through 9 on a channel, then closes it. The main goroutine receives and prints each value using `range` over the channel.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func main() {
	ch := make(chan int)

	go func() {
		for i := 0; i < 10; i++ {
			ch <- i
		}
		close(ch)
	}()

	for val := range ch {
		fmt.Println(val)
	}
}
```

</details>

### Exercise 2: Buffered Channel Blocking ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a buffered channel with capacity 3. Send 5 items to it. For the first 3 sends, print "sent without blocking". For sends 4 and 5, demonstrate that the send blocks by wrapping it in a goroutine with a print before and after.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"time"
)

func main() {
	ch := make(chan int, 3)

	for i := 1; i <= 3; i++ {
		ch <- i
		fmt.Printf("sent %d (no block)\n", i)
	}

	// These will block unless we drain the channel first
	go func() {
		ch <- 4
		fmt.Println("sent 4 (after blocking)")
	}()
	go func() {
		ch <- 5
		fmt.Println("sent 5 (after blocking)")
	}()

	time.Sleep(100 * time.Millisecond)

	// Drain to unblock the goroutines
	for i := 0; i < 5; i++ {
		fmt.Println("received:", <-ch)
	}
}
```

</details>

### Exercise 3: Pipeline ⭐⭐
**Difficulty:** Intermediate | **Time:** ~15 min

Build a pipeline: goroutine A generates integers 1–5, goroutine B receives each and doubles it, main collects and prints the doubled values.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func main() {
	nums := make(chan int)
	doubled := make(chan int)

	// Stage A: generate
	go func() {
		defer close(nums)
		for i := 1; i <= 5; i++ {
			nums <- i
		}
	}()

	// Stage B: double
	go func() {
		defer close(doubled)
		for n := range nums {
			doubled <- n * 2
		}
	}()

	// Main: collect
	for val := range doubled {
		fmt.Println(val)
	}
}
```

</details>

### Exercise 4: Close and Range ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a channel, send 7 string values from a goroutine, close the channel, and use `range` in main to collect all values into a slice. Print the slice.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func main() {
	ch := make(chan string)

	go func() {
		defer close(ch)
		for _, word := range []string{"the", "quick", "brown", "fox", "jumps", "over", "lazy"} {
			ch <- word
		}
	}()

	var collected []string
	for word := range ch {
		collected = append(collected, word)
	}
	fmt.Println(collected)
}
```

</details>
