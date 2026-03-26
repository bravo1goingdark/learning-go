# Go Deep Learning — Getting Started Guide

> This guide is for people who **know at least one programming language** (Python, JavaScript, Java, etc.) and want to learn Go from scratch to production-ready code.

---

## How This Guide Works

This guide teaches Go by building up concepts **gradually**. Each topic builds on previous ones.

### Difficulty Markers

Every section in every file is tagged so you know what to read on each pass:

| Marker | Meaning | When to read it |
|--------|---------|----------------|
| `[CORE]` | Must-read fundamentals | **First pass — read everything tagged CORE** |
| `[PRODUCTION]` | Real-world patterns | After CORE is solid, or when building projects |
| `[INTERNALS]` | Compiler/runtime deep dives | Advanced — revisit when optimizing or curious |

**First pass strategy:** Read only `[CORE]` sections. Skip `[PRODUCTION]` and `[INTERNALS]`. Come back to them after you've completed the projects.

### Learning Path & Time Estimates

| Phase | Topics | Est. Time | Prerequisites |
|-------|--------|-----------|---------------|
| **Phase 1: Foundations** | 00, 01, 02 | ~6 hours | None |
| **Phase 2: Data & Types** | 03, 04, 05, 06, 07 | ~10 hours | Phase 1 |
| **Phase 3: Error Handling** | 08, 09 | ~4 hours | Phase 2 |
| **Phase 4: Generics** | 10 | ~3 hours | Phase 3 |
| 📋 **Project 1** | CSV Processor | ~6 hours | Phases 1-3 |
| **Phase 5: Concurrency** | 11-16 | ~10 hours | Phase 4 |
| **Phase 6: Advanced Concurrency** | 17-19 | ~6 hours | Phase 5 |
| 📋 **Project 2** | Worker Pool | ~6 hours | Phases 5-6 |
| **Phase 7: Software Patterns** | 01-08 | ~8 hours | Phase 6 |
| 📋 **Project 3** | HTTP Service | ~8 hours | Phase 7 |
| **Phase 8: Systems Project** | 01-09 | ~10 hours | All phases |
| **Total** | | **~77 hours** | |

> 💡 Each topic has 3-5 exercises at the end. Do them — they reinforce what you read.

```
Phase 1: FOUNDATIONS
    ├── 00: Hello World Review   ← Warm-up, see what Go looks like
    ├── 01: Go Toolchain         ← Commands you'll use daily
    └── 02: Variables            ← Types, declarations, zero values

Phase 2: DATA & TYPES
    ├── 03: Arrays & Slices      ← Ordered collections
    ├── 04: Maps                 ← Key-value storage
    ├── 05: Structs & Methods    ← Custom data types
    ├── 06: Pointers             ← Memory addresses
    └── 07: Interfaces           ← Polymorphism

Phase 3: ERROR HANDLING
    ├── 08: Error Handling       ← Go's error model
    └── 09: Defer                ← Cleanup patterns

Phase 4: GENERICS
    └── 10: Generics             ← Type parameters, constraints
         ⚠️ Read BEFORE concurrency — several concurrency topics use [T any]

  📋 Project 1: CLI CSV Processor (Topics 1-9)

Phase 5: CONCURRENCY
    ├── 11: Goroutines           ← Lightweight threads
    ├── 12: Channels             ← Communication between goroutines
    ├── 13: Select               ← Multiplexing
    ├── 14: Context              ← Cancellation & deadlines
    ├── 15: WaitGroup            ← Waiting for goroutines
    └── 16: Mutex vs Channels    ← When to use which

Phase 6: ADVANCED CONCURRENCY
    ├── 17: Worker Pools         ← Bounded concurrency
    ├── 18: Pipelines            ← Data processing streams
    └── 19: Fan-In/Fan-Out       ← Parallel processing

  📋 Project 2: Worker Pool (Topics 11-19)

Phase 7: SOFTWARE PATTERNS
    ├── 01: Project Structure    ← How to organize Go code
    ├── 02: Repository Pattern   ← Data access abstraction
    ├── 03: Service Layer        ← Business logic isolation
    ├── 04: Dependency Injection ← Wiring components
    ├── 05: Clean Architecture   ← Layer boundaries
    ├── 06: Pub-Sub Design       ← Event-driven decoupling
    ├── 07: Retry + Circuit Breaker ← Resilience patterns
    └── 08: Backpressure         ← Handle system overload

  📋 Project 3: Layered HTTP Service (Software Patterns)

Phase 8: SYSTEMS PROJECT
    └── Message Broker (Topics 1-19 + Software Patterns)
```

---

## Before You Start

### 1. Install Go

```bash
# Check if Go is installed
go version

# If not installed, download from: https://go.dev/doc/install
# Or use your package manager:
# macOS: brew install go
# Linux: sudo apt install golang
# Windows: Download installer from go.dev
```

### 2. Create Your Workspace

```bash
# Create a folder for learning Go
mkdir -p ~/go-learning
cd ~/go-learning
```

### 3. Verify Installation

Create a file called `hello.go`:

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, Go!")
}
```

Run it:

```bash
go run hello.go
```

You should see: `Hello, Go!`

**Congratulations!** You've written your first Go program.

---

## How to Study Each Topic

### Step 1: Read [CORE] Sections Only (First Pass)

On your first read, focus **only** on sections marked `[CORE]`. Skip `[PRODUCTION]` and `[INTERNALS]` — you can revisit them later.

### Step 2: Try the Examples

Copy each code example and run it on your machine. Don't just read — **write code**.

### Step 3: Do the Exercises

Every topic ends with 3-5 exercises. Solve them before moving on. Solutions are in expandable blocks.

### Step 4: Test Your Understanding

Use the "Quick Reference" sections. Try to explain each concept out loud without looking.

### Step 5: Build the Projects

After each phase, do the corresponding project. Projects force you to combine concepts.

### Step 6: Second Pass — Add [PRODUCTION] and [INTERNALS]

After completing the projects, come back and read the `[PRODUCTION]` and `[INTERNALS]` sections. They'll make much more sense now.

---

## Essential Commands Reference

### Running Code

```bash
go run hello.go        # Run a single file
go build hello.go      # Compile to binary
go build -o myapp .    # Build and name the binary
./myapp                # Run the compiled binary
```

### Testing

```bash
go test ./...          # Run all tests
go test -v             # Verbose output
go test -race          # Check for race conditions
go test -cover         # Show coverage
```

### Getting Help

```bash
go doc fmt.Println    # View documentation
go doc -all sync       # View all docs for a package
go help go.mod         # Get help on a command
```

---

## Prerequisites

Before starting, you should know:

- **At least one programming language** (Python, JavaScript, Java, C#, etc.) — you understand variables, functions, loops, conditionals, and basic data structures
- **Command line basics** — navigate folders, run commands
- **Text editor** — VS Code (recommended), GoLand, or any editor you prefer

You **don't** need to know:
- Go specifically
- Memory management or pointers
- Concurrency concepts
- Systems programming

---

## Study Tips

### Rule 1: Don't Rush

If something doesn't make sense, re-read it. If it still doesn't make sense, check the "Common Pitfalls" section.

### Rule 2: Write Code

Reading alone won't teach you. After each concept, write your own example.

### Rule 3: Ask Questions

If you're stuck:
1. Check Go's official docs: https://go.dev/doc/
2. Search on Stack Overflow
3. Check Go by Example: https://gobyexample.com/

### Rule 4: Take Breaks

If you're confused, take a break. Come back later with fresh eyes.

---

## File Structure

```
study/
├── README.md              ← Main index
├── GETTING_STARTED.md     ← You are here
├── CHEATSHEET.md          ← Quick reference
├── 00-hello-world-review.md ← Warm-up exercise
├── 01-foundations/
│   ├── 01-go-toolchain.md
│   └── 02-variables-zero-values.md
├── 02-data-structures/
│   ├── 03-arrays-vs-slices.md
│   ├── 04-maps.md
│   └── 05-structs-and-methods.md
├── 03-type-system/
│   ├── 06-pointers.md
│   └── 07-interfaces.md
├── 04-error-handling/
│   ├── 08-error-handling.md
│   └── 09-defer-in-depth.md
├── 05-generics/
│   └── 10-generics.md
├── 06-concurrency/        ← Advanced topics
│   ├── 11-goroutines.md
│   ├── 12-channels.md
│   ├── 13-select.md
│   ├── 14-context.md
│   ├── 15-waitgroup.md
│   ├── 16-mutex-vs-channels.md
│   ├── 17-worker-pools.md
│   ├── 18-pipelines.md
│   └── 19-fan-in-fan-out.md
├── projects/
│   ├── 09-project-csv-processor.md   ← Project 1: Topics 1-9
│   ├── 19-concurrency-project.md    ← Project 2: Topics 11-19
│   └── 20-layered-http-service.md   ← Project 3: Software Patterns
├── 06-software-patterns/
│   ├── 01-project-structure.md
│   ├── 02-repository-pattern.md
│   ├── 03-service-layer.md
│   ├── 04-dependency-injection.md
│   ├── 05-clean-architecture.md
│   ├── 06-pub-sub-design.md
│   ├── 07-retry-circuit-breaker.md
│   └── 08-backpressure-strategies.md
└── assets/                ← Images and diagrams
```

---

## Next Steps

Ready? Let's begin!

### Start with the Foundations

1. **[Go Toolchain](01-foundations/01-go-toolchain.md)** — Learn the commands you'll use every day
2. **[Variables & Zero Values](01-foundations/02-variables-zero-values.md)** — Understand how data works in Go

---

**Remember**: Everyone starts as a beginner. The key is to keep practicing. You've got this!