# Go Quick Reference Card

> A one-page cheat sheet for common Go patterns. Print it or keep it handy!

---

## Basics

### Hello World
```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```

### Variables
```go
var x int              // Declaration
x := 42                // Short declaration (inside functions)
var y = 42             // Type inference
const PI = 3.14        // Constants
```

### Basic Types
```go
bool, int, int8-64, uint, uint8-64     // Numeric
float32, float64                        // Floating point
string                                  // Text
byte, rune                              // Character types
```

---

## Collections

### Arrays (fixed size)
```go
arr := [5]int{1, 2, 3, 4, 5}
```

### Slices (dynamic)
```go
slice := []int{1, 2, 3}
slice = append(slice, 4)
slice = slice[1:3]  // subslice
```

### Maps (key-value)
```go
m := map[string]int{"a": 1, "b": 2}
m["c"] = 3
delete(m, "a")
v, ok := m["b"]  // check existence
```

---

## Functions

```go
func greet(name string) string {
    return "Hello, " + name
}

// Multiple return values (error handling)
func divide(a, b int) (int, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}
```

---

## Structs & Methods

```go
type Person struct {
    Name string
    Age  int
}

// Value receiver
func (p Person) Greet() {
    fmt.Println("Hello, I'm", p.Name)
}

// Pointer receiver (modifies original)
func (p *Person) Birthday() {
    p.Age++
}
```

---

## Interfaces

```go
type Writer interface {
    Write([]byte) (int, error)
}

// Implicit implementation - no "implements" keyword
type File struct{}
func (f *File) Write(data []byte) (int, error) { return len(data), nil }
```

---

## Error Handling

```go
// Return error
result, err := doSomething()
if err != nil {
    return fmt.Errorf("failed: %w", err)
}

// Check error type
if errors.Is(err, ErrNotFound) { ... }
var customErr *CustomError
if errors.As(err, &customErr) { ... }
```

---

## Concurrency

### Goroutines
```go
go func() { ... }()
go functionName(args)
```

### Channels
```go
ch := make(chan int)        // unbuffered
ch := make(chan int, 10)   // buffered
ch <- value                 // send
value := <-ch              // receive
close(ch)                  // close
```

### Select
```go
select {
case v := <-ch1:
    fmt.Println(v)
case ch2 <- data:
    fmt.Println("sent")
case <-time.After(time.Second):
    fmt.Println("timeout")
}
```

### WaitGroup
```go
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // work
}()
wg.Wait()
```

### Mutex
```go
var mu sync.Mutex
mu.Lock()
// critical section
mu.Unlock()
// or
mu.Lock()
defer mu.Unlock()
```

### Context
```go
ctx, cancel := context.WithCancel(context.Background())
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Hour))
defer cancel()
```

---

## Common Packages

| Package | Use |
|---------|-----|
| `fmt` | Printf, Println, Sprintf |
| `os` | File operations, environment |
| `io` | Readers, Writers |
| `json` | JSON marshal/unmarshal |
| `time` | Time, durations |
| `strings` | String manipulation |
| `strconv` | String conversion |
| `errors` | Error handling |
| `sync` | Mutex, WaitGroup, Pool |

---

## Testing

```go
import "testing"

func TestFunction(t *testing.T) {
    result := myFunc(1, 2)
    if result != 3 {
        t.Errorf("expected 3, got %d", result)
    }
}

func BenchmarkFunction(b *testing.B) {
    for i := 0; i < b.N; i++ {
        myFunc(1, 2)
    }
}
```

### Run Tests
```bash
go test ./...           # All tests
go test -v             # Verbose
go test -race          # Race detector
go test -cover         # Coverage
go test -run TestName  # Specific test
go test -bench=.      # Run benchmarks
```

---

## CLI Commands

```bash
go run main.go         # Run
go build .            # Compile
go test ./...         # Test
go fmt ./...          # Format
go vet ./...          # Lint
go mod init myapp     # Initialize module
go mod tidy           # Clean up deps
go get package@version # Add dependency
go doc fmt.Println    # View docs
```

---

## Pro Tips

- Use `go run hello.go` to quickly test
- `go build -o app .` creates a binary named "app"
- Always check `err != nil` after functions that return errors
- Use `defer` for cleanup (file close, mutex unlock)
- Slices are reference types; arrays are value types
- Maps are reference types
- Interface{} is like "any" type
- `:=` only works inside functions
- Use `go fmt` before committing