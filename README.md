# Go Deep Learning

A complete, structured path to master Go from **zero** to production-ready code.

> **New to Go?** Start with [GETTING_STARTED.md](GETTING_STARTED.md) first!

## Quick Start

1. **Install Go**: https://go.dev/doc/install
2. **Read the Getting Started Guide**: [GETTING_STARTED.md](GETTING_STARTED.md)
3. **Follow the order** below, one topic at a time
4. **Build the final project** to put it all together

---

## Study Order

Work through these folders in order. Each one builds on the previous.

### [01-foundations/](01-foundations/) — Start Here
> The basics you must know before touching anything else.

| # | Topic | What You'll Learn |
|---|-------|-------------------|
| 01 | [Go Toolchain](01-foundations/01-go-toolchain.md) | go mod, go build, go test, go vet, profiling |
| 02 | [Variables & Zero Values](01-foundations/02-variables-zero-values.md) | Declaration forms, basic types, scoping rules |

### [02-data-structures/](02-data-structures/) — Build On Foundations
> Go's built-in data structures. Know them inside-out.

| # | Topic | What You'll Learn |
|---|-------|-------------------|
| 03 | [Arrays vs Slices](02-data-structures/03-arrays-vs-slices.md) | Arrays, slices, backing arrays, append mechanics |
| 04 | [Maps](02-data-structures/04-maps.md) | Map internals, concurrency, iteration gotchas |
| 05 | [Structs & Methods](02-data-structures/05-structs-and-methods.md) | Embedding, methods, composition over inheritance |

### [03-type-system/](03-type-system/) — Understand Go's Type Model
> Pointers and interfaces — the key to idiomatic Go.

| # | Topic | What You'll Learn |
|---|-------|-------------------|
| 06 | [Pointers](03-type-system/06-pointers.md) | Pointer basics, escape analysis, nil safety |
| 07 | [Interfaces](03-type-system/07-interfaces.md) | Implicit satisfaction, type assertions, empty interface |

### [04-error-handling/](04-error-handling/) — Write Robust Code
> Go's error model is unique. Master it before production.

| # | Topic | What You'll Learn |
|---|-------|-------------------|
| 08 | [Error Handling](04-error-handling/08-error-handling.md) | Error values, wrapping, sentinel errors, custom types |
| 09 | [Defer In Depth](04-error-handling/09-defer-in-depth.md) | Defer order, resource cleanup, panic/recover |

### [05-concurrency/](05-concurrency/) — Master Concurrency
> Go's killer feature. Learn goroutines, channels, and real-world patterns.

| # | Topic | What You'll Learn |
|---|-------|-------------------|
| 10 | [Goroutines](05-concurrency/10-goroutines.md) | G-M-P scheduler, closure gotchas, goroutine leaks |
| 11 | [Channels](05-concurrency/11-channels.md) | Buffered/unbuffered, directional types, closing rules |
| 12 | [Select](05-concurrency/12-select.md) | Multiplexing, timeouts, non-blocking, nil channels |
| 13 | [Context](05-concurrency/13-context.md) | Cancellation, deadlines, values, HTTP propagation |
| 14 | [WaitGroup](05-concurrency/14-waitgroup.md) | Waiting for goroutines, fire-and-forget, graceful shutdown |
| 15 | [Mutex vs Channels](05-concurrency/15-mutex-vs-channels.md) | When to use which, RWMutex, sync.Map, sync.Once |
| 16 | [Worker Pools](05-concurrency/16-worker-pools.md) | Bounded concurrency, context-aware pools, generics |
| 17 | [Pipelines](05-concurrency/17-pipelines.md) | Chained stages, generators, context-aware pipelines |
| 18 | [Fan-In / Fan-Out](05-concurrency/18-fan-in-fan-out.md) | Distribute work, merge results, bounded fan-out |

---

### [projects/09-project-csv-processor.md](projects/09-project-csv-processor.md) — Put Topics 1-9 Together
> Build a CLI CSV processor. Uses concepts from topics 1-9 (Foundations → Error Handling).

| Concepts Used |
|---------------|
| Toolchain, Variables, Slices, Maps, Structs, Pointers, Interfaces, Error Handling, Defer |

**Build:** `go build -o csvproc .`  
**Run:** `./csvproc view data.csv`

---

### [projects/19-concurrency-project.md](projects/19-concurrency-project.md) — Put Topics 10-18 Together
> Build a concurrent worker pool with graceful shutdown. Uses concepts from topics 10-18 (Concurrency).

| Concepts Used |
|---------------|
| Goroutines, Channels, Select, Context, WaitGroup, Mutex, Worker Pools, Pipelines, Fan-In/Fan-Out |

**Build:** `go build -o urlworker .`  
**Run:** `./urlworker urls.txt --workers 5`

---

## Rules

1. **Don't skip ahead.** If a later topic feels confusing, revisit the folder it depends on.
2. **Practice after each file.** Read, then write code. Reading alone won't stick.
3. **Revisit.** Come back to earlier folders as your understanding deepens.
