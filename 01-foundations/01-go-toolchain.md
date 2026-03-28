# 1. Go Toolchain — Complete Deep Dive

> **Goal:** Understand every command a production Go developer uses daily. No hand-waving.

![Go Toolchain Overview](../assets/01.png)

---

## Table of Contents

1. [Installation & Environment](#1-installation--environment) `[CORE]`
2. [Go Modules (`go mod`)](#2-go-modules-go-mod) `[CORE]`
3. [Building (`go build`)](#3-building-go-build) `[CORE]`
4. [Running (`go run`)](#4-running-go-run) `[CORE]`
5. [Testing (`go test`)](#5-testing-go-test) `[CORE]`
6. [Formatting (`go fmt`)](#6-formatting-go-fmt) `[CORE]`
7. [Vetting (`go vet`)](#7-vetting-go-vet) `[CORE]`
8. [Dependency Management](#8-dependency-management) `[PRODUCTION]`
9. [Cross-Compilation](#9-cross-compilation) `[PRODUCTION]`
10. [Profiling & Benchmarks](#10-profiling--benchmarks) `[PRODUCTION]`
11. [Go Workspace (Monorepo)](#11-go-workspace-monorepo) `[PRODUCTION]`
12. [Production Build Patterns](#12-production-build-patterns) `[PRODUCTION]`
13. [Common Pitfalls](#13-common-pitfalls) `[CORE]`

---

## 1. Installation & Environment

### Verify Installation

```bash
go version
# go version go1.22.5 linux/amd64

go env
# Prints all environment variables
```

### Critical Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `GOPATH` | Workspace for non-module code, caches, binaries | `~/go` |
| `GOROOT` | Go installation directory | `/usr/local/go` |
| `GOBIN` | Where `go install` puts binaries | `$GOPATH/bin` |
| `GOMODCACHE` | Where downloaded modules are stored | `$GOPATH/pkg/mod` |
| `GOFLAGS` | Default flags for all go commands | (empty) |
| `GONOSUMDB` | Modules to skip checksum verification | (empty) |
| `GONOSUMCHECK` | Deprecated, use `GONOSUMDB` | (empty) |
| `GOPROXY` | Module download proxy | `https://proxy.golang.org,direct` |
| `GONOPROXY` | Modules to bypass proxy | (empty) |
| `GOARCH` | Target architecture | host arch |
| `GOOS` | Target OS | host OS |

**Priority for day-to-day development:** You only need to remember `GOPATH`, `GOBIN`, and `GOROOT`. The proxy/checksum variables (`GOPROXY`, `GONOSUMDB`, `GONOPROXY`) matter when you work behind corporate firewalls or use private modules. `GOFLAGS` is useful for CI where you want `-mod=readonly` applied globally. The rest you can ignore until a specific need arises.

### PATH Setup

```bash
# Add to ~/.bashrc or ~/.zshrc
export PATH=$PATH:$(go env GOPATH)/bin
```

> **Why?** `go install` puts tools like `golangci-lint`, `mockgen` in `GOBIN`. You need this on PATH.

---

## 2. Go Modules (`go mod`)

Go modules replaced `GOPATH` mode. Every project has a `go.mod` file at its root.

### Initialize a Module

```bash
mkdir myproject && cd myproject
go mod init github.com/username/myproject
```

This creates `go.mod`:

```
module github.com/username/myproject

go 1.22
```

### Module Path Conventions

```
github.com/<org>/<repo>          # GitHub
gitlab.com/<org>/<repo>          # GitLab
go.<custom-domain>/<path>        # Custom vanity import path
```

The module path is the import path for all packages in the module.

### The `go.mod` File

```go
module github.com/myorg/myapp

go 1.22

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/lib/pq v1.10.9
    go.uber.org/zap v1.26.0
)

require (
    github.com/bytedance/sonic v1.9.1 // indirect
    github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abb8968 // indirect
    // ... more indirect deps
)
```

- **Direct dependencies:** your code imports these
- **Indirect dependencies:** dependencies of your dependencies (or deps not yet in `go.sum`)
- `// indirect` comment marks transitive deps

### The `go.sum` File

Cryptographic checksums of every dependency version. **Never edit manually.** Commit to version control.

### `go mod tidy`

The most important command:

```bash
go mod tidy
```

- Adds missing dependencies your code imports
- Removes unused dependencies
- Updates `go.sum`
- **Run this before every commit.** Seriously.

### `go mod download`

```bash
go mod download              # Download all dependencies
go mod download github.com/gin-gonic/gin@v1.9.1  # Download specific version
```

Downloads to `GOMODCACHE` without building anything.

### `go mod vendor`

```bash
go mod vendor
```

Copies all dependencies into a `vendor/` directory. Useful for:
- Air-gapped environments
- Docker builds without network access
- Guaranteed reproducible builds

To build with vendor:
```bash
go build -mod=vendor ./...
```

### `go mod graph`

```bash
go mod graph
```

Prints the entire dependency graph. Useful for auditing.

### `go mod why`

```bash
go mod why github.com/some/dep
```

Explains why a dependency is needed (which of your packages imports it).

### `go mod edit`

```bash
go mod edit -require github.com/gin-gonic/gin@v1.9.1
go mod edit -exclude github.com/broken/pkg@v1.0.0
go mod edit -replace github.com/old/pkg=./local-fork
go mod edit -go=1.22
```

Programmatic editing of `go.mod`. Useful in scripts/CI.

---

## 3. Building (`go build`)

### Basic Build

```bash
go build                    # Build package in current directory
go build ./...              # Build all packages in module
go build ./cmd/server       # Build specific package
go build -o myapp ./cmd/server  # Custom output name
```

### What Happens

1. Compiles the package and all dependencies
2. Links into a single static binary (by default)
3. Output binary is placed in current directory (or specified with `-o`)

### Build Flags

```bash
# Strip debug symbols (smaller binary)
go build -ldflags="-s -w" -o myapp ./cmd/server

# -s: omit symbol table
# -w: omit DWARF debug info

# Inject variables at build time
go build -ldflags="-X main.version=1.2.3 -X main.commit=$(git rev-parse HEAD)" ./cmd/server
```

In your code:
```go
package main

import "fmt"

var (
    version = "dev"
    commit  = "none"
)

func main() {
    fmt.Printf("version: %s, commit: %s\n", version, commit)
}
```

### `-ldflags` Deep Dive

```bash
# Common production flags
go build -ldflags="-s -w \
    -X main.version=${VERSION} \
    -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
    -X main.gitCommit=$(git rev-parse --short HEAD)" \
    -o myapp ./cmd/server
```

### Build Tags / Constraints

Files can have build constraints:

```go
//go:build linux
// +build linux

package main
// This file only compiles on Linux
```

```go
//go:build !windows

package main
// This file compiles on everything except Windows
```

```go
//go:build integration

package tests
// Only compiled with: go test -tags=integration
```

Multiple constraints:
```go
//go:build linux && amd64
//go:build linux && (amd64 || arm64)
//go:build !cgo  // Pure Go, no C dependencies
```

### CGO

```bash
# Disable CGO (pure Go, static binary)
CGO_ENABLED=0 go build -o myapp ./cmd/server

# Enable CGO (needed for sqlite3, etc.)
CGO_ENABLED=1 go build -o myapp ./cmd/server
```

**Production rule:** Prefer `CGO_ENABLED=0` for Docker containers. Pure Go binaries have zero external dependencies.

### Race Detector

```bash
go build -race ./cmd/server    # Compile with race detector
go run -race ./cmd/server      # Run with race detector
go test -race ./...            # Test with race detector
```

The race detector finds data races at runtime. **Never deploy `-race` binaries to production** — 10x memory overhead, 10x slower.

---

## 4. Running (`go run`)

### Basic Usage

```bash
go run main.go              # Run a single file
go run .                    # Run package in current directory
go run ./cmd/server         # Run specific package
go run -race .              # Run with race detection
```

### What Happens

1. Compiles to a temporary directory
2. Executes the binary
3. Cleans up temp files on exit

**`go run` is for development only.** Never use in production.

### `go run` vs `go build`

| | `go run` | `go build` |
|---|----------|------------|
| Output | Temp binary, auto-deleted | Permanent binary |
| Use case | Development, quick testing | Production, deployment |
| Speed | Compiles every time | Build once, run many |
| Flags | `-race`, `-ldflags` | `-race`, `-ldflags`, `-o` |

### `go install`

```bash
go install ./cmd/server             # Install binary to $GOBIN
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest  # Install remote tool
```

Unlike `go build`, `go install` puts the binary in `$GOBIN` and caches the build. This is how you install Go-based CLI tools globally.

---

## 5. Testing (`go test`)

> **Note for Beginners:** Testing is a `[CORE]` topic and deeply integrated into Go, but if you're new to Go, **learn the language fundamentals first** (variables, functions, structs, etc.). Come back here once you've written some Go code. For now, focus on Exercise 3 (line 1135) to get hands-on experience.

This is where Go shines. Testing is built into the toolchain. No frameworks required.

### Basic Test

Create a file `math.go`:
```go
package math

func Add(a, b int) int {
    return a + b
}

func Divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
}
```

Create `math_test.go`:
```go
package math

import "testing"

func TestAdd(t *testing.T) {
    got := Add(2, 3)
    want := 5
    if got != want {
        t.Errorf("Add(2,3) = %d; want %d", got, want)
    }
}

func TestDivide(t *testing.T) {
    got, err := Divide(10, 3)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got < 3.33 || got > 3.34 {
        t.Errorf("Divide(10,3) = %f; want ~3.33", got)
    }
}

func TestDivideByZero(t *testing.T) {
    _, err := Divide(10, 0)
    if err == nil {
        t.Error("expected error for division by zero; got nil")
    }
}
```

Run tests:
```bash
go test ./...               # Run all tests in module
go test ./math              # Run tests in specific package
go test -v ./...            # Verbose output
go test -run TestDivide ./math  # Run specific test by regex
go test -count=1 ./...      # Disable test caching
go test -short ./...        # Skip long tests
go test -timeout 30s ./...  # Set timeout (default 10 minutes)
```

### Table-Driven Tests

The idiomatic Go testing pattern:

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -1, -2, -3},
        {"zero", 0, 0, 0},
        {"mixed", -1, 5, 4},
        {"large", 1000000, 2000000, 3000000},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", tt.a, tt.b, got, tt.expected)
            }
        })
    }
}
```

Why table-driven?
- Easy to add test cases
- Each case gets its own name in failure output
- Subtests with `t.Run()` allow `go test -run TestAdd/positive`

### Test Helpers

```go
func TestSomething(t *testing.T) {
    // t.Helper() marks this as a helper — errors report at caller's line
    mustParse := func(t *testing.T, s string) time.Time {
        t.Helper()  // CRITICAL — without this, errors point here instead of caller
        ts, err := time.Parse(time.RFC3339, s)
        if err != nil {
            t.Fatalf("failed to parse %q: %v", s, err)
        }
        return ts
    }

    ts := mustParse(t, "2024-01-01T00:00:00Z")
    // ...
}
```

### `t.Fatal` vs `t.Error`

```go
t.Errorf("format", args...)  // Log error, continue test
t.Fatalf("format", args...)  // Log error, STOP test immediately
t.Error("msg")               // Same as Errorf with no formatting
t.Fatal("msg")               // Same as Fatalf with no formatting
```

Rule: use `Fatalf` when subsequent assertions depend on the result.

### Setup & Teardown

```go
func TestWithDB(t *testing.T) {
    // Setup
    db, err := sql.Open("postgres", "test-connection-string")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()  // Teardown — always runs

    // Seed test data
    _, err = db.Exec("INSERT INTO users (name) VALUES ('test')")
    if err != nil {
        t.Fatal(err)
    }

    // ... actual test ...
}
```

### Test Main (Package-Level Setup)

```go
var db *sql.DB

func TestMain(m *testing.M) {
    // Setup — runs once before all tests in this package
    var err error
    db, err = sql.Open("postgres", os.Getenv("TEST_DB_URL"))
    if err != nil {
        log.Fatal(err)
    }

    // Run all tests
    code := m.Run()

    // Teardown — runs once after all tests
    db.Close()

    os.Exit(code)
}
```

### Subtests

```go
func TestUserService(t *testing.T) {
    t.Run("CreateUser", func(t *testing.T) {
        // ...
    })
    t.Run("DeleteUser", func(t *testing.T) {
        // ...
    })
    t.Run("UpdateUser", func(t *testing.T) {
        // can nest further
        t.Run("WithValidData", func(t *testing.T) { /* ... */ })
        t.Run("WithInvalidData", func(t *testing.T) { /* ... */ })
    })
}
```

Run specific subtest:
```bash
go test -run TestUserService/CreateUser .
go test -run TestUserService/UpdateUser/WithValidData .
```

### Parallel Tests

```go
func TestParallel(t *testing.T) {
    t.Parallel()  // This test runs in parallel with other parallel tests

    // ...
}
```

All tests with `t.Parallel()` run concurrently. Useful for I/O-bound tests.

### Benchmarks

```go
func BenchmarkAdd(b *testing.B) {
    // b.N is set by the framework — run the function b.N times
    for i := 0; i < b.N; i++ {
        Add(2, 3)
    }
}

func BenchmarkMapLookup(b *testing.B) {
    m := make(map[int]int, 1000)
    for i := 0; i < 1000; i++ {
        m[i] = i
    }

    b.ResetTimer()  // Don't count setup time
    for i := 0; i < b.N; i++ {
        _ = m[500]
    }
}
```

Run benchmarks:
```bash
go test -bench=. ./...                    # Run all benchmarks
go test -bench=BenchmarkAdd -benchmem .   # Include memory allocation stats
go test -bench=. -benchtime=5s .          # Run for 5 seconds each
go test -bench=. -count=5 .               # Run 5 times (for statistical analysis)
```

Output:
```
BenchmarkAdd-8      1000000000    0.3118 ns/op    0 B/op    0 allocs/op
│                  │             │               │         └── allocations per op
│                  │             │               └── bytes per op
│                  │             └── nanoseconds per operation
│                  └── total iterations (auto-scaled)
└── benchmark name (-8 = GOMAXPROCS)
```

### Fuzzing (Go 1.18+)

```go
func FuzzAdd(f *testing.F) {
    // Seed corpus — known interesting inputs
    f.Add(2, 3)
    f.Add(-1, -1)
    f.Add(0, 0)

    f.Fuzz(func(t *testing.T, a, b int) {
        result := Add(a, b)
        // Property: result - a should equal b (commutative property)
        if result-a != b {
            t.Errorf("Add(%d, %d) = %d; but %d - %d = %d", a, b, result, result, a, result-a)
        }
    })
}
```

```bash
go test -fuzz=FuzzAdd .            # Run fuzzer
go test -fuzz=FuzzAdd -fuzztime=30s .  # Run for 30 seconds
go test -fuzz=.                    # Run all fuzz tests
```

The fuzzer generates random inputs, finds edge cases. Failed inputs are saved to `testdata/`.

### Coverage

```bash
go test -cover ./...                       # Show coverage percentage
go test -coverprofile=coverage.out ./...   # Write coverage data
go tool cover -html=coverage.out           # Open HTML coverage report in browser
go tool cover -func=coverage.out           # Show per-function coverage
```

**Production rule:** Aim for >80% coverage. Don't chase 100% — test behavior, not lines.

---

## 6. Formatting (`go fmt`)

### The Golden Rule

**There is one style. Use it. No debates.**

```bash
go fmt ./...                 # Format all files
gofmt -w .                   # Same thing, lower-level command
gofmt -s .                   # Simplify code (e.g., []int{T(v)} → []int{v})
```

### `go fmt` vs `gofmt`

- `go fmt` — high-level, wraps `gofmt`, runs on packages
- `gofmt` — low-level, works on files/directories

### What `go fmt` Does

- Indents with tabs (not spaces)
- Aligns struct fields, map literals
- Removes unnecessary parentheses
- Sorts imports (with `-s` flag)
- Wraps long lines consistently

### Editor Integration

**VS Code:** Install Go extension, enable format-on-save:
```json
{
    "[go]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "golang.go"
    }
}
```

**Vim/Neovim:** Use `gofmt` or `goimports` as formatter.

**GoLand:** Built-in, enabled by default.

### `goimports` (Recommended Over `go fmt`)

```bash
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .    # Formats + adds/removes imports
```

`goimports` does everything `go fmt` does, plus:
- Adds missing imports
- Removes unused imports
- Groups imports (stdlib, external, internal)

---

## 7. Vetting (`go vet`)

Static analysis that catches common mistakes.

```bash
go vet ./...                  # Run all analyzers
go vet -printf ./...          # Run specific analyzer
go vet -printf -nilfunc .     # Run multiple analyzers
```

### What It Catches

| Analyzer | Catches |
|----------|---------|
| `printf` | Mismatched format verbs in `fmt.Printf` |
| `nilfunc` | Comparing function to nil (always false for methods) |
| `composite` | Unkeyed struct literals |
| `assign` | Unused assignments |
| `bools` | Mistaken use of `==` with booleans |
| `buildtag` | Malformed `//go:build` tags |
| `structtag` | Malformed struct tags |
| `copylocks` | Copying sync.Mutex values |
| `httpresponse` | Ignoring HTTP response |
| `loopclosure` | Captured loop variable in goroutine |
| `lostcancel` | Ignoring context cancel function |
| `unmarshal` | Passing non-pointer to `json.Unmarshal` |

### Example: `go vet` Catching Bugs

```go
// This compiles but is WRONG
var mu sync.Mutex
mu2 := mu  // go vet: assignment copies lock value to mu2: sync.Mutex

// This compiles but is WRONG
fmt.Printf("Hello %s", 42)  // go vet: arg 42 for printf verb %s of wrong type: untyped int

// This compiles but is WRONG (pre-Go 1.22 loop variable capture)
for _, item := range items {
    go func() {
        fmt.Println(item)  // go vet: loop variable item captured by func literal
    }()
}
```

---

## 8. Dependency Management

> ⏭️ **First pass? Skip this section.** Come back after completing the projects in Phases 1-3. For now, you only need `go mod init`, `go mod tidy`, and `go get`.

### Versioning

Go uses semantic versioning: `v1.2.3`

```
v0.x.x — pre-release, no stability guarantee
v1.x.x — stable, backward-compatible
v2.x.x — requires /v2 in module path (v2+ breaking change convention)
```

### Upgrade Dependencies

```bash
go get github.com/some/pkg@latest          # Latest version
go get github.com/some/pkg@v1.5.0          # Specific version
go get github.com/some/pkg@main            # Latest commit on branch
go get -u ./...                            # Upgrade ALL dependencies
go get -u=patch ./...                       # Upgrade to latest PATCH only
```

### List Dependencies

```bash
go list -m -u all              # Show all deps with available updates
go list -m -json all           # Detailed JSON output
go list -m -versions github.com/gin-gonic/gin  # List all versions
```

### Dependency Auditing

```bash
# Check for known vulnerabilities
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

### `GOFLAGS` for CI

```bash
# Make CI strict
export GOFLAGS="-mod=readonly"  # Don't modify go.mod
go build ./...                  # Fails if go.mod is dirty
```

---

## 9. Cross-Compilation

> ⏭️ **First pass? Skip this section.** Come back after completing the projects in Phases 1-3.

Go compiles to any OS/architecture from any OS/architecture. No toolchain setup needed.

### List All Targets

```bash
go tool dist list
```

### Cross-Compile

```bash
# Linux AMD64 from macOS
GOOS=linux GOARCH=amd64 go build -o myapp-linux-amd64 ./cmd/server

# Linux ARM64 (e.g., AWS Graviton, Apple M1 Docker)
GOOS=linux GOARCH=arm64 go build -o myapp-linux-arm64 ./cmd/server

# Windows from Linux
GOOS=windows GOARCH=amd64 go build -o myapp.exe ./cmd/server

# macOS from Linux
GOOS=darwin GOARCH=amd64 go build -o myapp-darwin-amd64 ./cmd/server
GOOS=darwin GOARCH=arm64 go build -o myapp-darwin-arm64 ./cmd/server  # M1/M2
```

### Build All Targets (Makefile)

```makefile
VERSION := $(shell git describe --tags --always)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build-all
build-all:
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/myapp-linux-amd64   ./cmd/server
	GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/myapp-linux-arm64   ./cmd/server
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/myapp-darwin-amd64  ./cmd/server
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/myapp-darwin-arm64  ./cmd/server
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/myapp-windows-amd64.exe ./cmd/server
```

---

## 10. Profiling & Benchmarks

> ⏭️ **First pass? Skip this section.** Come back after completing the projects in Phases 1-3.

### CPU Profile

```go
import (
    "log"
    "os"
    "runtime/pprof"
)

func main() {
    f, err := os.Create("cpu.prof")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()
    pprof.StartCPUProfile(f)
    defer pprof.StopCPUProfile()

    // ... your code ...
}
```

Or use flags:
```bash
go test -cpuprofile=cpu.prof -bench=. ./...
go tool pprof cpu.prof       # Interactive profiling
```

### Memory Profile

```bash
go test -memprofile=mem.prof -bench=. ./...
go tool pprof mem.prof
```

### HTTP Server Profiling

```go
import _ "net/http/pprof"

func main() {
    go http.ListenAndServe(":6060", nil)
    // ... your server ...
}
```

Then:
```bash
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30    # CPU
go tool pprof http://localhost:6060/debug/pprof/heap                   # Memory
go tool pprof http://localhost:6060/debug/pprof/goroutine              # Goroutines
```

### Trace

```bash
go test -trace=trace.out ./...
go tool trace trace.out   # Opens browser UI
```

---

## 11. Go Workspace (Monorepo)

> ⏭️ **First pass? Skip this section.** Come back after completing the projects in Phases 1-3.

Go 1.18+ workspaces let you work on multiple modules simultaneously.

```bash
mkdir monorepo && cd monorepo
go work init                    # Creates go.work
go work use ./service-a         # Add module
go work use ./service-b         # Add module
go work use ./shared-libs       # Add module
```

`go.work`:
```
go 1.22

use (
    ./service-a
    ./service-b
    ./shared-libs
)
```

Now `service-a` can import from `shared-libs` without publishing it. Changes are reflected immediately.

**Rule:** Don't commit `go.work` to git. It's for local development only. Add to `.gitignore`.

---

## 12. Production Build Patterns

> ⏭️ **First pass? Skip this section.** Come back after completing the projects in Phases 1-3.

### Multi-Stage Docker Build

```dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server

# Stage 2: Run
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
```

### CI Pipeline (GitHub Actions)

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go vet ./...
      - run: go test -race -coverprofile=coverage.out ./...
      - run: go tool cover -func=coverage.out

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server
```

---

## 13. Common Pitfalls

### 1. Forgetting `go mod tidy` After Adding Imports

**Symptom:** CI fails with "cannot find module providing package"

**Fix:** Run `go mod tidy` before committing.

### 2. Using `go run` in Production

**Symptom:** Slow startup, temp files, no binary caching

**Fix:** Always `go build` for production.

### 3. Not Using `-race` in CI

**Symptom:** Data races in production that don't show up in local dev

**Fix:** Always run `go test -race ./...` in CI.

### 4. Committing `go.work`

**Symptom:** Other developers can't build because their workspace layout differs

**Fix:** Add `go.work` to `.gitignore`.

### 5. Using `replace` in `go.mod` for Non-Local Paths

**Symptom:** Works on your machine, fails everywhere else

**Fix:** Only use `replace` for local development forks. Remove before committing.

### 6. Not Setting `-timeout` on `go test`

**Symptom:** CI hangs forever on a deadlocked test

**Fix:** Always set `go test -timeout=60s ./...` in CI.

### 7. Ignoring `go vet` Warnings

**Symptom:** Silent bugs in production

**Fix:** Run `go vet ./...` in CI. Make it a required check.

---

## Quick Reference Card

```bash
# Development
go run .                        # Run locally
go test ./...                   # Run tests
go test -v -run TestName .      # Run specific test
go test -race ./...             # Race detection
go test -cover ./...            # Coverage
go fmt ./...                    # Format code
go vet ./...                    # Static analysis

# Dependencies
go mod init github.com/org/repo # Initialize module
go mod tidy                     # Clean up deps
go get pkg@version              # Add/update dependency
go mod download                 # Download all deps
go mod vendor                   # Vendor dependencies

# Building
go build -o app ./cmd/server    # Build binary
CGO_ENABLED=0 go build .        # Static binary
go build -ldflags="-s -w" .    # Stripped binary
GOOS=linux GOARCH=amd64 go build .  # Cross-compile

# Tools
go install tool@version         # Install CLI tool
go doc fmt.Printf               # View docs
go doc -all sync.Mutex          # Full docs
go doc -src fmt.Printf          # Source code
```

> **How to look up unfamiliar stdlib functions:** Throughout this repo you'll encounter functions like `strings.Fields`, `bytes.NewBufferString`, `json.Marshal`, `io.ReadAll`, etc. Don't memorize them. Instead:
> 1. Run `go doc <package>.<function>` (e.g., `go doc strings.Fields`)
> 2. Or browse online: https://pkg.go.dev/std
> 3. Or use your editor's hover feature (VS Code shows docs on hover)
> This skill — looking things up — is more valuable than memorizing every function.

---

## Exercises

### Exercise 1: Hello Module ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a new directory called `hello-mod`. Initialize a Go module inside it, write a `main.go` that prints `"Hello, Go Toolchain!"`, and run it with `go run`.

<details>
<summary>Solution</summary>

```bash
mkdir hello-mod && cd hello-mod
go mod init example.com/hello-mod
```

```go
// main.go
package main

import "fmt"

func main() {
	fmt.Println("Hello, Go Toolchain!")
}
```

```bash
go run main.go
```

</details>

### Exercise 2: Import Your Own Function ⭐⭐
**Difficulty:** Beginner | **Time:** ~15 min

In the same module, create a second file `greet.go` with a package-level function `func Greet(name string) string`. Call it from `main.go` and print the result.

<details>
<summary>Solution</summary>

```go
// greet.go
package main

import "fmt"

func Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
```

```go
// main.go
package main

import "fmt"

func main() {
	fmt.Println(Greet("Go"))
}
```

```bash
go run .
```

</details>

### Exercise 3: Write and Run a Test ⭐⭐
**Difficulty:** Beginner | **Time:** ~15 min

Write a `Multiply(a, b int) int` function in a new module. Create a `_test.go` file with a test for it. Run `go test -v` and confirm the test passes.

<details>
<summary>Solution</summary>

```bash
mkdir mathmod && cd mathmod
go mod init example.com/mathmod
```

```go
// math.go
package mathmod

func Multiply(a, b int) int {
	return a * b
}
```

```go
// math_test.go
package mathmod

import "testing"

func TestMultiply(t *testing.T) {
	got := Multiply(3, 4)
	want := 12
	if got != want {
		t.Errorf("Multiply(3,4) = %d; want %d", got, want)
	}
}
```

```bash
go test -v
```

</details>

### Exercise 4: Catch Bugs with `go vet` ⭐⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a file that declares an unused variable (e.g., `x := 10` with no further use). Run `go vet` and observe the error. Then fix it.

<details>
<summary>Solution</summary>

```go
// main.go
package main

import "fmt"

func main() {
	x := 10
	fmt.Println("hello")
}
```

```bash
go vet ./...
# Output: x declared but not used (or similar)
```

Fix — either use the variable or assign to `_`:

```go
func main() {
	x := 10
	_ = x
	fmt.Println("hello")
}
```

</details>

---

## Next: [Variables & Zero Values →](./02-variables-zero-values.md)
