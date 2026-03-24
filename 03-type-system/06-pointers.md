# 6. Pointers — Complete Deep Dive

> **Goal:** Master Go pointers — safe, simple, and without C's complexity. No pointer arithmetic.

![Pointers Overview](../assets/06.png)

---

## Table of Contents

1. [What is a Pointer](#1-what-is-a-pointer)
2. [Declaration & Dereferencing](#2-declaration--dereferencing)
3. [Pointers with Functions](#3-pointers-with-functions)
4. [Pointers with Structs](#4-pointers-with-structs)
5. [new() vs &Type{}](#5-new-vs-type)
6. [Escape Analysis](#6-escape-analysis)
7. [When to Use Pointers](#7-when-to-use-pointers)
8. [When NOT to Use Pointers](#8-when-not-to-use-pointers)
9. [Pointer Gotchas](#9-pointer-gotchas)
10. [Production Patterns](#10-production-patterns)
11. [Common Pitfalls](#11-common-pitfalls)

---

## 1. What is a Pointer

A pointer **holds the memory address** of another variable.

```
Variable:    x = 42
Address:     0x14000100008

Pointer:     p = 0x14000100008
             p points to x
             *p = 42
```

### Go vs C Pointers

| Feature | C | Go |
|---------|---|-----|
| Pointer arithmetic | Yes | **No** |
| Manual memory management | Yes (`malloc`/`free`) | **No** (GC) |
| Dangling pointers | Yes | **No** (GC prevents) |
| Null pointer dereference | Segfault | **Panic** (catchable) |
| Pointer to pointer | Yes | Yes (but rare) |

**Go pointers are safe.** You can't do arithmetic, you can't corrupt memory, and the garbage collector handles deallocation.

---

## 2. Declaration & Dereferencing

### `&` — Address Of

```go
x := 42
p := &x  // p is *int, holds address of x

fmt.Println(x)   // 42
fmt.Println(p)   // 0x14000100008 (address)
fmt.Println(*p)  // 42 (value at address)
```

### `*` — Dereference (Read/Write Value)

```go
x := 42
p := &x

*p = 100          // Write through pointer
fmt.Println(x)    // 100 — original changed!

y := *p           // Read through pointer
fmt.Println(y)    // 100
```

### Declaration

```go
var p *int        // nil pointer — points to nothing
fmt.Println(p)    // <nil>
// *p              // PANIC: nil pointer dereference

x := 42
p = &x            // Now p points to x
fmt.Println(*p)   // 42
```

### Zero Value

```go
var p *int       // nil
var q *string    // nil
var r *User      // nil

fmt.Println(p == nil)  // true
fmt.Println(q == nil)  // true
fmt.Println(r == nil)  // true
```

---

## 3. Pointers with Functions

### Pass by Value (Default) — Does NOT Modify Original

```go
func increment(x int) {
    x++  // Modifies copy
}

func main() {
    n := 10
    increment(n)
    fmt.Println(n)  // 10 — unchanged
}
```

### Pass by Pointer — Modifies Original

```go
func increment(x *int) {
    *x++  // Modifies original through pointer
}

func main() {
    n := 10
    increment(&n)
    fmt.Println(n)  // 11
}
```

### Swap Function

```go
func swap(a, b *int) {
    *a, *b = *b, *a
}

func main() {
    x, y := 1, 2
    swap(&x, &y)
    fmt.Println(x, y)  // 2 1
}
```

### Returning Pointers

```go
func newUser(name string) *User {
    u := User{Name: name}  // Allocated on heap (escapes)
    return &u
}

func main() {
    u := newUser("Alice")
    fmt.Println(u.Name)  // "Alice"
}
```

**This works!** Go's escape analysis moves `u` to the heap because it outlives the function.

### Multiple Return (Preferred Over Pointers for Errors)

```go
// Instead of returning *int for "maybe missing" value
func findUser(id int) (*User, error) {
    user, ok := db[id]
    if !ok {
        return nil, ErrNotFound
    }
    return &user, nil
}
```

---

## 4. Pointers with Structs

### Creating Struct Pointers

```go
// Method 1: Address of struct literal
u := &User{Name: "Alice", Age: 30}

// Method 2: Address of variable
user := User{Name: "Alice"}
u := &user

// Method 3: new() — returns pointer to zero value
u := new(User)  // *User, fields are zero values
u.Name = "Alice"
```

### Field Access — Automatic Dereferencing

```go
u := &User{Name: "Alice"}

// Go auto-dereferences — no need for (*u).Name
fmt.Println(u.Name)    // "Alice" — same as (*u).Name
u.Age = 30             // Same as (*u).Age = 30
```

### Method Receivers

```go
// Pointer receiver — can modify struct
func (u *User) SetName(name string) {
    u.Name = name
}

// Value receiver — cannot modify
func (u User) GetName() string {
    return u.Name
}

func main() {
    u := &User{Name: "Alice"}
    u.SetName("Bob")    // Go auto-takes address: (*u).SetName("Bob")
    fmt.Println(u.GetName())  // "Bob"
}
```

---

## 5. `new()` vs `&Type{}`

### `new(T)` — Pointer to Zero Value

```go
p := new(int)     // *int, points to 0
*p = 42

s := new(string)  // *string, points to ""
*s = "hello"

u := new(User)    // *User, all fields zero
u.Name = "Alice"  // Auto-dereference
```

### `&T{}` — Pointer with Initializer

```go
u := &User{Name: "Alice", Age: 30}  // Preferred
```

### Comparison

```go
// new — zero value
u1 := new(User)
// u1.Name == ""
// u1.Age == 0

// & — with values
u2 := &User{Name: "Alice", Age: 30}
// u2.Name == "Alice"
// u2.Age == 30
```

**Production rule:** Use `&T{}` for structs. Use `new(T)` only for primitive types where you need a pointer to zero value.

---

## 6. Escape Analysis

Go's compiler decides whether a variable lives on the **stack** or the **heap**. You don't control this explicitly.

### Stack vs Heap

```
Stack:  Fast, auto-cleaned when function returns
Heap:   Slower, garbage collected
```

### When Variables Escape to Heap

```go
// 1. Returning pointer to local variable
func newUser(name string) *User {
    u := User{Name: name}  // Escapes to heap
    return &u
}

// 2. Assigning to interface
func process(w io.Writer) {
    // ...
}

func main() {
    buf := bytes.Buffer{}  // Escapes to heap (assigned to interface)
    process(&buf)
}

// 3. Captured by closure that outlives function
func counter() func() int {
    count := 0  // Escapes to heap
    return func() int {
        count++
        return count
    }
}

// 4. Very large structs
func process() {
    var huge [1 << 20]byte  // Escapes to heap (too large for stack)
}
```

### How to Check

```bash
go build -gcflags="-m" ./...
# Shows escape analysis decisions

go build -gcflags="-m -m" ./...
# More detailed output
```

### Example Output

```go
func newUser(name string) *User {
    u := User{Name: name}
    return &u
}
```

```bash
$ go build -gcflags="-m" .
./main.go:10:2: moved to heap: u
```

### Performance Implications

```go
// Stack allocation — fast
func process() {
    var x int = 42  // Stack
    use(x)
}

// Heap allocation — slower (GC pressure)
func process() *int {
    x := 42  // Heap (escapes)
    return &x
}
```

**Don't worry about this prematurely.** Let the compiler optimize. Only optimize when profiling shows allocation is a bottleneck.

---

## 7. When to Use Pointers

### 1. To Modify a Value in a Function

```go
func updateConfig(cfg *Config) {
    cfg.Port = 8080
}
```

### 2. To Avoid Copying Large Structs

```go
type LargeStruct struct {
    Data [1 << 20]byte
}

// BAD — copies 1MB
func process(s LargeStruct) { }

// GOOD — copies 8 bytes (pointer)
func process(s *LargeStruct) { }
```

### 3. To Share State Between Goroutines

```go
type Counter struct {
    mu    sync.Mutex
    count int
}

func (c *Counter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}

func main() {
    c := &Counter{}  // Must be pointer — shared between goroutines
    for i := 0; i < 100; i++ {
        go func() {
            c.Increment()
        }()
    }
}
```

### 4. To Distinguish "Missing" from "Zero"

```go
// Is Age 0 because it's missing, or because the user is 0?
type User struct {
    Name string
    Age  int      // Can't distinguish 0 from missing
}

// With pointer — nil means "missing"
type User struct {
    Name string
    Age  *int     // nil = missing, &0 = actually zero
}

func main() {
    u1 := User{Name: "Baby", Age: intPtr(0)}    // Age is 0
    u2 := User{Name: "Unknown"}                  // Age is nil (missing)
}

func intPtr(i int) *int { return &i }
```

### 5. For Method Receivers That Modify State

```go
func (s *Service) Start() error { ... }
func (s *Service) Stop() error { ... }
func (s *Service) Status() Status { ... }
```

### 6. To Implement Interfaces with Pointer Receivers

```go
type Writer interface {
    Write(p []byte) (n int, err error)
}

type File struct{ /* ... */ }

func (f *File) Write(p []byte) (n int, err error) {
    // Only *File satisfies Writer, not File
}
```

---

## 8. When NOT to Use Pointers

### 1. Small Value Types

```go
// DON'T — pointer adds overhead for 8-byte int
func add(a, b *int) *int { result := *a + *b; return &result }

// DO — pass values
func add(a, b int) int { return a + b }
```

### 2. Strings, Slices, Maps (Already Reference-Like)

```go
// DON'T — these are already reference types
func process(s *[]int) { ... }
func process(m *map[string]int) { ... }

// DO — pass by value
func process(s []int) { ... }
func process(m map[string]int) { ... }
```

### 3. Immutable Data

```go
type Config struct {
    Host string
    Port int
}

// Value receiver — config won't be modified
func (c Config) Host() string { return c.Host }
```

### 4. Local Variables That Don't Escape

```go
func process() {
    x := 42       // Value — stays on stack
    y := &x       // Pointer — forces x to heap (unnecessary)
    use(y)
}
```

---

## 9. Pointer Gotchas

### 1. Nil Pointer Dereference

```go
var p *int
fmt.Println(*p)  // PANIC: runtime error: nil pointer dereference

// Fix:
if p != nil {
    fmt.Println(*p)
}
```

### 2. Pointer to Loop Variable (Pre-Go 1.22)

```go
// Pre-Go 1.22: BUG — all pointers point to same variable
users := []*User{}
for _, u := range userList {
    users = append(users, &u)  // &u always points to same memory!
}
// All users in the slice point to the LAST element

// Fix (pre-1.22):
for _, u := range userList {
    u := u  // Create new variable in loop scope
    users = append(users, &u)
}

// Go 1.22+: Each iteration creates a new variable — no bug!
```

### 3. Pointer in Goroutine

```go
// BUG — pointer captured before goroutine starts
for i := 0; i < 5; i++ {
    go func() {
        fmt.Println(&i)  // All goroutines see final value of i
    }()
}

// Fix:
for i := 0; i < 5; i++ {
    i := i  // Capture value
    go func() {
        fmt.Println(i)
    }()
}
```

### 4. Pointer Comparison

```go
a := 42
b := 42
p1 := &a
p2 := &b

fmt.Println(p1 == p2)  // false — different addresses
fmt.Println(*p1 == *p2) // true — same values
```

### 5. Returning Pointer to Local Slice Element

```go
func getFirst(s []int) *int {
    return &s[0]  // Points to underlying array — OK as long as array exists
}

func main() {
    s := []int{1, 2, 3}
    p := getFirst(s)
    s = append(s, 4, 5, 6, 7, 8)  // May reallocate!
    fmt.Println(*p)  // Undefined — p may point to freed memory
}
```

---

## 10. Production Patterns

### Constructor Returning Pointer

```go
type Server struct {
    addr     string
    handler  http.Handler
    listener net.Listener
}

func NewServer(addr string, handler http.Handler) (*Server, error) {
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        return nil, fmt.Errorf("listen: %w", err)
    }
    return &Server{
        addr:     addr,
        handler:  handler,
        listener: ln,
    }, nil
}
```

### Optional Fields with Pointers

```go
type User struct {
    Name     string  `json:"name"`
    Email    string  `json:"email"`
    Age      *int    `json:"age,omitempty"`      // nil = omitted
    Bio      *string `json:"bio,omitempty"`       // nil = omitted
    Verified *bool   `json:"verified,omitempty"`  // nil = omitted
}

func main() {
    // Explicitly set age to 0
    age := 0
    u := User{Name: "Baby", Age: &age}
    // JSON: {"name":"Baby","age":0}
    
    // Age not set
    u2 := User{Name: "Unknown"}
    // JSON: {"name":"Unknown"} — age omitted entirely
}
```

### Atomic Pointer (Go 1.19+)

```go
import "sync/atomic"

type Config struct {
    Port int
}

var currentConfig atomic.Pointer[Config]

func init() {
    currentConfig.Store(&Config{Port: 8080})
}

func getConfig() *Config {
    return currentConfig.Load()  // Thread-safe read
}

func updateConfig(newCfg *Config) {
    currentConfig.Store(newCfg)  // Thread-safe write
}
```

---

## 11. Common Pitfalls

### 1. Nil Pointer Dereference

```go
var u *User
fmt.Println(u.Name)  // PANIC

// Fix: check for nil
if u != nil {
    fmt.Println(u.Name)
}
```

### 2. Pointer to Interface (Usually Wrong)

```go
// WRONG — rarely what you want
var w *io.Writer

// RIGHT
var w io.Writer
```

### 3. Unnecessary Pointer to Slice/Map

```go
// WRONG — adds indirection, no benefit
func process(s *[]int) {
    (*s)[0] = 99
}

// RIGHT — slices are already reference-like
func process(s []int) {
    s[0] = 99
}
```

### 4. Pointer Copy in Struct

```go
type User struct {
    Name string
    Data *[]byte
}

u1 := User{Name: "Alice", Data: &[]byte{1, 2, 3}}
u2 := u1  // Shallow copy — u2.Data points to same slice!

(*u2.Data)[0] = 99
fmt.Println((*u1.Data)[0])  // 99 — u1 is modified!
```

---

## Quick Reference

```go
// Declaration
var p *int           // nil pointer
x := 42; p = &x      // p points to x
p := new(int)         // pointer to zero value (0)
u := &User{Name: "Alice"}  // pointer to struct literal

// Operations
*p                    // dereference — get value
*p = 100              // dereference — set value
p == nil              // check if nil
u.Name                // auto-dereference for struct fields

// Passing
func modify(x *int) { *x = 100 }   // modify via pointer
func read(x int) { fmt.Println(x) } // read by value

// Return
func newUser() *User { return &User{Name: "Alice"} }
```

---

## 11. Production Best Practices

### When to Use Pointers

| Use Case | Recommendation |
|----------|---------------|
| Modifying caller's variable | Use pointer |
| Passing large struct (>128 bytes) | Use pointer to avoid copy |
| Returning allocated struct | Return pointer |
| Immutable data | Use value |
| Small structs (<16 bytes) | Use value |
| Maps, slices, functions | Already reference types — use values |

### Pointer vs Value Decision Tree

```go
func decidePointerOrValue(someStruct MyStruct) {
    // Q1: Do you need to modify the original?
    // Yes → Use pointer
    
    // Q2: Is the struct larger than machine word * 2?
    // (typically >16 bytes on 64-bit)
    // Yes → Use pointer
    
    // Q3: Is this a hot path (called millions of times)?
    // Consider value if small, pointer if large
    
    // Q4: Are you storing in a map?
    // Use pointer — values in maps can't be updated in place
}
```

### Memory Allocation Patterns

```go
// BAD: Returning pointer to local stack variable
func bad() *int {
    x := 42
    return &x  // x escapes to heap — compiler handles it, but pattern is confusing
}

// GOOD: Explicit allocation
func good() *int {
    return new(int) // Clear intent
}

// GOOD: Factory function
func NewUser(name string) *User {
    return &User{Name: name} // idiomatic
}

// GOOD: Pool allocation for high-frequency objects
var userPool = sync.Pool{
    New: func() interface{} {
        return &User{}
    },
}

func getUser() *User {
    u := userPool.Get().(*User)
    u.Reset() // Reset fields
    return u
}

func putUser(u *User) {
    userPool.Put(u)
}
```

---

## 12. Performance Considerations

### Pointer Indirection Cost

```go
type Heavy struct {
    Data [1024]byte // 1KB
}

// Value — each access copies the struct
func processValue(h Heavy) {
    _ = h.Data[0] // Copy on call
}

// Pointer — no copy, just pointer indirection
func processPointer(h *Heavy) {
    _ = h.Data[0] // Direct access via pointer
}

// Benchmark results typically show:
// - Small structs (<64 bytes): value is faster
// - Large structs: pointer is faster
// - Call-heavy paths: value may be faster due to cache
```

### Escape Analysis

```go
// This stays on stack — no heap allocation
func stackAlloc() {
    x := 42
    fmt.Println(x)
}

// This escapes to heap — heap allocation
func heapAlloc() *int {
    x := 42
    return &x // x escapes
}

// Check with:
go build -gcflags="-m" main.go 2>&1 | grep -E "(escapes|moved)"
```

### Benchmarks

```go
func BenchmarkValue(b *testing.B) {
    var s MyStruct
    for i := 0; i < b.N; i++ {
        processValue(s)
    }
}

func BenchmarkPointer(b *testing.B) {
    s := &MyStruct{}
    for i := 0; i < b.N; i++ {
        processPointer(s)
    }
}
```

---

## 13. Common Pitfalls

### Nil Pointer Dereference

```go
// BAD
func bad() {
    var p *int
    fmt.Println(*p) // PANIC: invalid memory address or nil pointer dereference
}

// GOOD: Check for nil
func good() {
    var p *int
    if p != nil {
        fmt.Println(*p)
    } else {
        fmt.Println("p is nil")
    }
}
```

### Returning Pointer to Internal Field

```go
// BAD: Expose internal mutable state
type Container struct {
    data []byte
}

func (c *Container) Data() *[]byte {
    return &c.data // Caller can mutate internal state!
}

// GOOD: Return immutable copy or value
func (c *Container) Data() []byte {
    result := make([]byte, len(c.data))
    copy(result, c.data)
    return result
}
```

### Pointer in Map

```go
// BAD: Pointer in map can cause issues
m := map[string]*User{
    "alice": {Name: "Alice", Age: 30},
}
m["alice"].Age = 31 // This is safe but be careful with concurrency

// If you modify the pointer value itself:
m["bob"] = &User{Name: "Bob"} // This is fine

// But with concurrent access, use sync.Map or mutex
```

### Forgetting to Initialize Pointer to Struct

```go
// BAD
func bad() {
    var u *User
    u.Name = "Alice" // PANIC: nil pointer dereference
}

// GOOD
func good() {
    u := &User{}
    u.Name = "Alice"
}

// Or use constructor
func NewUser() *User {
    return &User{Name: "Unknown"} // Pre-initialized
}
```

---

## 14. Debugging Pointers

### Print Pointer Values

```go
var p *int
fmt.Printf("%p", p)    // prints 0x0 (nil)
fmt.Printf("%v", p)    // prints <nil>

x := 42
p = &x
fmt.Printf("%p", p)    // prints 0xc000012345
fmt.Printf("%v", *p)   // prints 42
```

### Using %p with Custom Types

```go
type User struct {
    Name string
}

func printPointerInfo() {
    u1 := &User{Name: "Alice"}
    u2 := &User{Name: "Bob"}
    
    fmt.Printf("u1: %p, u2: %p\n", u1, u2) // Different addresses
    fmt.Printf("u1: %v, u2: %v\n", u1, u2) // {Alice} {Bob}
}
```

---

## 15. Testing with Pointers

```go
func TestPointerModification(t *testing.T) {
    u := &User{Name: "Alice", Age: 30}
    
    modifyUser(u)
    
    if u.Age != 31 {
        t.Errorf("expected age 31, got %d", u.Age)
    }
}

func modifyUser(u *User) {
    u.Age = 31
}

// Test for nil safety
func TestNilPointerSafety(t *testing.T) {
    var p *User
    
    // Should not panic
    result := safeGetName(p)
    if result != "" {
        t.Error("expected empty string for nil user")
    }
}

func safeGetName(u *User) string {
    if u == nil {
        return ""
    }
    return u.Name
}
```

---

## Next: [Interfaces →](./07-interfaces.md)
