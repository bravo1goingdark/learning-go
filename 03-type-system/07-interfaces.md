# 7. Interfaces (Implicit) — Complete Deep Dive

> **Goal:** Master Go's most powerful feature. Interfaces are implicit — no `implements` keyword.

![Interfaces Overview](../assets/07.png)

---

## Table of Contents

1. [Interface Basics](#1-interface-basics)
2. [Implicit Satisfaction](#2-implicit-satisfaction)
3. [Interface Internals](#3-interface-internals)
4. [The Empty Interface (`any`)](#4-the-empty-interface-any)
5. [Type Assertions](#5-type-assertions)
6. [Type Switches](#6-type-switches)
7. [Nil Interface vs Interface Holding Nil](#7-nil-interface-vs-interface-holding-nil)
8. [Interface Composition](#8-interface-composition)
9. [Standard Library Interfaces](#9-standard-library-interfaces)
10. [Design Principles](#10-design-principles)
11. [Common Patterns](#11-common-patterns)
12. [Common Pitfalls](#12-common-pitfalls)

---

## 1. Interface Basics

An interface defines a **set of methods**. Any type that implements all those methods satisfies the interface.

### Definition

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type Closer interface {
    Close() error
}
```

### Implementation

```go
type File struct {
    name string
}

func (f *File) Read(p []byte) (n int, err error) {
    // ... read from file
    return len(p), nil
}

func (f *File) Write(p []byte) (n int, err error) {
    // ... write to file
    return len(p), nil
}

func (f *File) Close() error {
    // ... close file
    return nil
}
```

`*File` satisfies `Reader`, `Writer`, and `Closer` — **automatically**, with no declaration.

---

## 2. Implicit Satisfaction

### No `implements` Keyword

```go
// Java — explicit
public class File implements Reader, Writer, Closer { }

// Go — implicit
type File struct { }
func (f *File) Read(p []byte) (n int, err error)  { return 0, nil }
func (f *File) Write(p []byte) (n int, err error) { return 0, nil }
func (f *File) Close() error                       { return nil }
// File automatically satisfies Reader, Writer, Closer
```

### Retroactive Interface Implementation

```go
// You can create an interface for a type you don't own

// In your code:
type Stringer interface {
    String() string
}

// time.Duration from standard library already has String() method
// It satisfies your Stringer interface automatically

func printValue(s Stringer) {
    fmt.Println(s.String())
}

func main() {
    printValue(5 * time.Second)  // "5s"
}
```

### Design Implication

> "Accept interfaces, return structs."

- **Accept interfaces:** Functions should accept the smallest interface they need
- **Return structs:** Functions should return concrete types

```go
// Good — accepts smallest interface
func copyData(dst io.Writer, src io.Reader) error {
    _, err := io.Copy(dst, src)
    return err
}

// Bad — accepts concrete type unnecessarily
func copyData(dst *os.File, src *os.File) error {
    // Now can't use bytes.Buffer, http.Response.Body, etc.
}
```

---

## 3. Interface Internals

An interface variable is a **two-word** structure:

```
  ┌──────────────────────────────────────────────────────────────┐
  │              INTERFACE VARIABLE (16 bytes)                    │
  ├────────────────────────┬─────────────────────────────────────┤
  │   Type pointer (8B)    │       Data pointer (8B)             │
  │   unsafe.Pointer       │       unsafe.Pointer                │
  └───────────┬────────────┴──────────────────┬──────────────────┘
              │                               │
              ▼                               ▼
  ┌───────────────────────┐       ┌───────────────────────┐
  │   type descriptor     │       │    actual value       │
  │   (runtime type info) │       │    (heap allocated    │
  │   - methods           │       │     if needed)        │
  │   - size              │       │                       │
  │   - name              │       │                       │
  └───────────────────────┘       └───────────────────────┘
```

### The `eface` and `iface`

```go
// Empty interface (any) — eface
type eface struct {
    _type *_type        // pointer to type descriptor
    data  unsafe.Pointer // pointer to value
}

// Non-empty interface — iface
type iface struct {
    tab  *itab          // pointer to interface table (type + method set)
    data unsafe.Pointer  // pointer to value
}
```

### Implications

```go
var r io.Reader

// r is nil — both type and data pointers are nil
fmt.Println(r == nil)  // true

buf := bytes.NewBufferString("hello")
r = buf
// r's type pointer → *bytes.Buffer
// r's data pointer → buf

fmt.Println(r == nil)  // false — type pointer is set
```

### Size of Interface

```go
var r io.Reader
fmt.Println(unsafe.Sizeof(r))  // 16 (two 8-byte pointers on 64-bit)

var a any
fmt.Println(unsafe.Sizeof(a))  // 16
```

---

## 4. The Empty Interface (`any`)

### Definition

```go
type any = interface{}

// These are identical:
var x any
var y interface{}
```

`any` matches **every type** — it has no methods.

### When to Use

```go
// 1. When you truly don't know the type
func Println(a ...any)  // fmt.Println accepts anything

// 2. JSON decoding
var data any
json.Unmarshal(jsonBytes, &data)

// 3. Generic container (pre-Go 1.18)
type Cache struct {
    data map[string]any
}
```

### Type Safety is Lost

```go
func process(v any) {
    // v is "anything" — need type assertion to use it
    s, ok := v.(string)
    if ok {
        fmt.Println("It's a string:", s)
    }
}
```

**Avoid `any` when possible.** Use generics (Go 1.18+) or specific interfaces instead.

---

## 5. Type Assertions

### Basic Assertion

```go
var r io.Reader = bytes.NewBufferString("hello")

// Assertion — extract concrete type
buf := r.(*bytes.Buffer)  // Panics if r is not *bytes.Buffer

// Safe assertion — returns (value, ok)
buf, ok := r.(*bytes.Buffer)
if ok {
    fmt.Println(buf.String())
}
```

### Assertion on `any`

```go
func process(v any) {
    switch val := v.(type) {
    case string:
        fmt.Println("string:", val)
    case int:
        fmt.Println("int:", val)
    case bool:
        fmt.Println("bool:", val)
    default:
        fmt.Printf("unknown: %T\n", val)
    }
}
```

### Assertion to Interface

```go
var r io.Reader = bytes.NewBufferString("hello")

// Assert to another interface
w, ok := r.(io.Writer)  // Does *bytes.Buffer also implement Writer?
if ok {
    w.Write([]byte("world"))
}
```

---

## 6. Type Switches

```go
func describe(v any) string {
    switch v := v.(type) {
    case nil:
        return "nil"
    case int:
        return fmt.Sprintf("int: %d", v)
    case string:
        return fmt.Sprintf("string: %q", v)
    case bool:
        return fmt.Sprintf("bool: %t", v)
    case error:
        return fmt.Sprintf("error: %s", v.Error())
    case fmt.Stringer:
        return fmt.Sprintf("stringer: %s", v.String())
    default:
        return fmt.Sprintf("unknown: %T", v)
    }
}
```

### Matching Multiple Types

```go
func isNumeric(v any) bool {
    switch v.(type) {
    case int, int8, int16, int32, int64:
        return true
    case uint, uint8, uint16, uint32, uint64:
        return true
    case float32, float64:
        return true
    default:
        return false
    }
}
```

### Match Order Matters

```go
// More specific interfaces first
switch v := v.(type) {
case error:           // Check error first (more specific)
    return v.Error()
case fmt.Stringer:    // Then Stringer (less specific)
    return v.String()
}
```

---

## 7. Nil Interface vs Interface Holding Nil

**This is the #1 interface bug in production Go.**

### The Bug

```go
func getUser() (*User, error) {
    return nil, nil  // Returns nil pointer, nil error
}

func main() {
    user, err := getUser()
    if err != nil {
        fmt.Println("Error:", err)  // This prints!
    }
}
```

Wait — `err` is `nil`, so why does it print?

Actually, let me show the REAL bug:

```go
type MyError struct {
    Msg string
}

func (e *MyError) Error() string {
    return e.Msg
}

func getUser() (*User, error) {
    var err *MyError  // nil pointer
    return nil, err   // Returns nil pointer wrapped in non-nil interface!
}

func main() {
    user, err := getUser()
    fmt.Println(err == nil)  // FALSE!
    // err is a non-nil interface containing a nil *MyError
}
```

### The Explanation

```
Interface = (type pointer, data pointer)

Case 1: var err error = nil
  type pointer = nil
  data pointer = nil
  err == nil → TRUE

Case 2: var err error = (*MyError)(nil)
  type pointer → *MyError (NOT nil!)
  data pointer = nil
  err == nil → FALSE!
```

### The Fix

```go
func getUser() (*User, error) {
    var err *MyError  // nil
    if err != nil {
        return nil, err  // Only return if actually set
    }
    return nil, nil  // Clean nil
}

// Or return the error interface directly
func getUser() (*User, error) {
    return nil, nil  // Always return typed nil
}
```

### Production Rule

**Never return a typed nil as an interface. Always check before returning.**

```go
// WRONG
func doSomething() error {
    var err *MyError  // nil
    // ... some logic ...
    return err  // Returns nil *MyError wrapped in non-nil interface
}

// RIGHT
func doSomething() error {
    // ... some logic ...
    return nil  // Returns actual nil interface
}

// ALSO RIGHT
func doSomething() error {
    err := someFunc()  // returns *MyError
    if err != nil {
        return err
    }
    return nil
}
```

---

## 8. Interface Composition

### Combining Interfaces

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type Closer interface {
    Close() error
}

// Composed interface
type ReadWriter interface {
    Reader
    Writer
}

type ReadWriteCloser interface {
    Reader
    Writer
    Closer
}
```

### Interface Embedding Hierarchy

```
io.Reader
    ↓
io.ReadWriter ← io.Reader + io.Writer
    ↓
io.ReadWriteCloser ← io.Reader + io.Writer + io.Closer
```

### Production Pattern

```go
// Small interfaces — compose as needed
type Querier interface {
    Query(ctx context.Context, sql string, args ...any) (*Rows, error)
}

type Execer interface {
    Exec(ctx context.Context, sql string, args ...any) (Result, error)
}

type DB interface {
    Querier
    Execer
}

// Functions accept the smallest interface they need
func GetUser(q Querier, id int) (*User, error) { ... }
func SaveUser(e Execer, u *User) error { ... }
func Migrate(db DB) error { ... }  // Needs both
```

---

## 9. Standard Library Interfaces

### Most Important Interfaces

```go
// io — the foundation
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type Closer interface {
    Close() error
}

type ReadWriter interface {
    Reader
    Writer
}

type ReadCloser interface {
    Reader
    Closer
}

type WriteCloser interface {
    Writer
    Closer
}

type ReadWriteCloser interface {
    Reader
    Writer
    Closer
}
```

```go
// fmt — string representation
type Stringer interface {
    String() string
}

type GoStringer interface {
    GoString() string
}
```

```go
// encoding — serialization
type Marshaler interface {
    MarshalJSON() ([]byte, error)
}

type Unmarshaler interface {
    UnmarshalJSON(data []byte) error
}

type TextMarshaler interface {
    MarshalText() (text []byte, err error)
}

type TextUnmarshaler interface {
    UnmarshalText(text []byte) error
}
```

```go
// sort — ordering
type Interface interface {
    Len() int
    Less(i, j int) bool
    Swap(i, j int)
}
```

```go
// sort (Go 1.21+) — ordered constraint
type Ordered interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
        ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
        ~float32 | ~float64 |
        ~string
}
```

### How to Satisfy `io.Reader`

```go
type MyReader struct {
    data []byte
    pos  int
}

func (r *MyReader) Read(p []byte) (n int, err error) {
    if r.pos >= len(r.data) {
        return 0, io.EOF
    }
    n = copy(p, r.data[r.pos:])
    r.pos += n
    return n, nil
}

// MyReader now satisfies io.Reader
// Can be used with io.Copy, io.ReadAll, http handlers, etc.
```

---

## 10. Design Principles

### Principle 1: Accept Interfaces, Return Structs

```go
// Good
func Process(r io.Reader) ([]byte, error) {
    return io.ReadAll(r)
}

func NewProcessor() *Processor {
    return &Processor{...}
}
```

### Principle 2: Keep Interfaces Small

```go
// Bad — too many methods
type Service interface {
    Create(ctx context.Context, user *User) error
    Read(ctx context.Context, id string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter Filter) ([]*User, error)
    Search(ctx context.Context, query string) ([]*User, error)
    Count(ctx context.Context) (int, error)
    // ... 10 more methods
}

// Good — single responsibility
type Creator interface {
    Create(ctx context.Context, user *User) error
}

type Reader interface {
    Read(ctx context.Context, id string) (*User, error)
}

type Lister interface {
    List(ctx context.Context, filter Filter) ([]*User, error)
}
```

### Principle 3: Define Interfaces at the Consumer

```go
// Package A: returns concrete type (doesn't define interface)
package userrepo

type Repository struct { ... }

func (r *Repository) GetByID(id string) (*User, error) { ... }
func (r *Repository) Save(user *User) error { ... }

// Package B: defines interface it needs (consumer defines contract)
package userservice

type UserGetter interface {
    GetByID(id string) (*User, error)
}

type Service struct {
    repo UserGetter  // Interface, not concrete type
}

func NewService(repo UserGetter) *Service {
    return &Service{repo: repo}
}
```

### Principle 4: The Zero Interface

```go
// If a type has no meaningful methods, don't create an interface
// Just use the concrete type

// WRONG
type ConfigReader interface { }
type Config struct { ... }
func LoadConfig(r ConfigReader) (*Config, error) { ... }

// RIGHT — no interface needed
func LoadConfig(path string) (*Config, error) { ... }
```

---

## 11. Common Patterns

### Mock-Friendly Code

```go
// Define interface for external dependency
type Clock interface {
    Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

type FakeClock struct {
    time time.Time
}

func (f FakeClock) Now() time.Time { return f.time }

// Use in production
type Service struct {
    clock Clock
}

func NewService(clock Clock) *Service {
    return &Service{clock: clock}
}

// In tests
func TestService(t *testing.T) {
    fake := FakeClock{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
    svc := NewService(fake)
    // ...
}
```

### Decorator Pattern

```go
type Handler interface {
    ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type LoggingMiddleware struct {
    next  Handler
    logger *slog.Logger
}

func (m *LoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    m.next.ServeHTTP(w, r)
    m.logger.Info("request",
        "method", r.Method,
        "path", r.URL.Path,
        "duration", time.Since(start),
    )
}
```

### Strategy Pattern

```go
type Sorter interface {
    Sort(data []int)
}

type BubbleSort struct{}
func (BubbleSort) Sort(data []int) { /* ... */ }

type QuickSort struct{}
func (QuickSort) Sort(data []int) { /* ... */ }

func Process(data []int, sorter Sorter) {
    sorter.Sort(data)
    // ...
}
```

### Optional Interface Check (Compile-Time)

```go
// Ensure *File implements io.ReadWriteCloser at compile time
var _ io.ReadWriteCloser = (*File)(nil)

// If File doesn't implement the interface, this line causes a compile error
```

---

## 12. Common Pitfalls

### 1. Nil Interface Holding Nil Pointer

```go
// BUG
func doWork() error {
    var err *MyError  // nil
    // ... work ...
    return err  // Returns non-nil interface with nil data
}

// FIX
func doWork() error {
    // ... work ...
    return nil
}
```

### 2. Fat Interfaces

```go
// BAD — impossible to mock, couples everything
type UserService interface {
    Create(*User) error
    Read(string) (*User, error)
    Update(*User) error
    Delete(string) error
    List(Filter) ([]*User, error)
    Authenticate(string, string) (*User, error)
    SendEmail(*User, string) error
    GenerateReport() ([]byte, error)
}
```

### 3. Defining Interface at Producer

```go
// BAD — producer defines interface
package db

type Querier interface {
    Query(sql string, args ...any) (*Rows, error)
}

// GOOD — consumer defines interface
package service

type Querier interface {
    Query(sql string, args ...any) (*Rows, error)
}
```

### 4. Interface for Single Implementation

```go
// BAD — unnecessary abstraction
type ConfigLoader interface {
    Load(path string) (*Config, error)
}

type FileConfigLoader struct{}
func (f FileConfigLoader) Load(path string) (*Config, error) { ... }

// Only one implementation exists — use concrete type directly
```

### 5. Comparing Interfaces

```go
var a io.Reader = bytes.NewBufferString("hello")
var b io.Reader = bytes.NewBufferString("hello")

// Comparing interfaces compares both type and data
fmt.Println(a == b)  // false — different pointers
```

### 6. Forgetting Pointer Receiver for Interface

```go
type Stringer interface {
    String() string
}

type User struct {
    Name string
}

// Value receiver
func (u User) String() string {
    return u.Name
}

func main() {
    u := User{Name: "Alice"}
    var s Stringer = u    // OK
    var s Stringer = &u   // Also OK
}
```

```go
// Pointer receiver
func (u *User) String() string {
    return u.Name
}

func main() {
    u := User{Name: "Alice"}
    var s Stringer = u    // ❌ COMPILE ERROR
    var s Stringer = &u   // ✅ OK
}
```

---

## Quick Reference

```go
// Definition
type Reader interface {
    Read(p []byte) (n int, err error)
}

// Implicit satisfaction
type MyReader struct{}
func (r *MyReader) Read(p []byte) (int, error) { return 0, nil }
// *MyReader satisfies Reader

// Composition
type ReadWriter interface {
    Reader
    Writer
}

// Empty interface
var x any = 42

// Type assertion
s, ok := x.(string)

// Type switch
switch v := x.(type) {
case string: fmt.Println(v)
case int:    fmt.Println(v)
}

// Nil interface vs interface holding nil
var err error = nil          // nil interface
var err error = (*E)(nil)    // non-nil interface with nil data

// Compile-time check
var _ io.Reader = (*MyReader)(nil)
```

---

## 13. Production Patterns

### Dependency Injection

```go
// Define interface in your business logic package
type Storage interface {
    Get(key string) ([]byte, error)
    Put(key string, value []byte) error
    Delete(key string) error
}

// Implement with concrete type
type FileStorage struct {
    path string
}

func (f *FileStorage) Get(key string) ([]byte, error) {
    return os.ReadFile(filepath.Join(f.path, key))
}

func (f *FileStorage) Put(key string, value []byte) error {
    return os.WriteFile(filepath.Join(f.path, key), value, 0644)
}

func (f *FileStorage) Delete(key string) error {
    return os.Remove(filepath.Join(f.path, key))
}

// Use in service
type Service struct {
    storage Storage
}

func NewService(s Storage) *Service {
    return &Service{storage: s}
}

// Easy to mock in tests
type MockStorage struct {
    data map[string][]byte
}

func (m *MockStorage) Get(key string) ([]byte, error) {
    if v, ok := m.data[key]; ok {
        return v, nil
    }
    return nil, errors.New("not found")
}
```

### Interface Composition

```go
// io package uses this pattern extensively
type ReadWriter interface {
    Reader
    Writer
}

type ReadSeeker interface {
    Reader
    Seeker
}

type ReadWriteSeeker interface {
    ReadWriter
    Seeker
}

// Build your own
type Processor interface {
    Input() Reader
    Output() Writer
    Process() error
}
```

### Table-Driven Tests with Interfaces

```go
type Formatter interface {
    Format(data Data) string
}

func TestFormatter(t *testing.T) {
    tests := []struct {
        name     string
        formatter Formatter
        input     Data
        expected  string
    }{
        {
            name:      "JSON",
            formatter: &JSONFormatter{},
            input:     Data{Name: "test"},
            expected:  `{"name":"test"}`,
        },
        {
            name:      "XML",
            formatter: &XMLFormatter{},
            input:     Data{Name: "test"},
            expected:  "<data><name>test</name></data>",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := tt.formatter.Format(tt.input)
            if result != tt.expected {
                t.Errorf("got %s, want %s", result, tt.expected)
            }
        })
    }
}
```

---

## 14. Error Handling with Interfaces

```go
type ErrorHandler interface {
    Handle(err error) bool // returns true if handled
}

// Chain of responsibility pattern
type ErrorChain struct {
    handlers []ErrorHandler
}

func (c *ErrorChain) Handle(err error) {
    for _, h := range c.handlers {
        if h.Handle(err) {
            return
        }
    }
    // Default handling
    log.Printf("unhandled error: %v", err)
}

type RetryHandler struct {
    maxRetries int
}

func (r *RetryHandler) Handle(err error) bool {
    var netErr net.Error
    if errors.As(err, &netErr) && netErr.Temporary() {
        // Handle retry
        return true
    }
    return false
}
```

---

## 15. Interface Performance

```go
// Interfaces add a small indirection cost
func viaInterface(i interface{}) int {
    return i.(someType).method()
}

// Direct call is faster
func directCall(s *someType) int {
    return s.method()
}

// But interface cost is usually negligible
// Only optimize if profiling shows it's a bottleneck
```

---

## 16. Common Mistakes

### Defining Interfaces Too Early

```go
// BAD: Define interface before you need it
type Repository interface {
    Get(id string) (User, error)
    Create(user User) error
    // ... 20 more methods
}

// GOOD: Define interface where you need it (consumer side)
func NewService(db Database) *Service {
    // Database interface only needs the methods we use
}
```

### Not Using Interface for Testability

```go
// BAD: Hard to test
type Service struct {
    db *sql.DB
}

func (s *Service) GetUser(id string) (User, error) {
    return s.db.QueryRow("SELECT * FROM users WHERE id = ?", id)
}

// GOOD: Use interface
type DB interface {
    QueryRow(query string, args ...interface{}) *sql.Row
}

type Service struct {
    db DB
}
```

### Interface Pollution

```go
// BAD: Too many small interfaces
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
type Closer interface { Close() error }
// ... then compose them everywhere

// GOOD: Use standard library interfaces when possible
// io.Reader, io.Writer, io.Closer, etc.
```

---

## 17. Testing with Interfaces

```go
// Create mock implementations
type MockRepository struct {
    users map[string]User
}

func (m *MockRepository) Get(id string) (User, error) {
    if user, ok := m.users[id]; ok {
        return user, nil
    }
    return User{}, errors.New("not found")
}

func (m *MockRepository) Create(user User) error {
    m.users[user.ID] = user
    return nil
}

// Use in tests
func TestService(t *testing.T) {
    mock := &MockRepository{users: make(map[string]User)}
    svc := NewService(mock)

    // Test
    err := svc.CreateUser(User{ID: "1", Name: "Alice"})
    if err != nil {
        t.Fatal(err)
    }

    user, err := svc.GetUser("1")
    if err != nil {
        t.Fatal(err)
    }

    if user.Name != "Alice" {
        t.Errorf("expected Alice, got %s", user.Name)
    }
}
```

---

## 18. Interface vs Concrete Types

```go
// Use concrete types when:
// - You only have one implementation
// - Performance is critical
// - The type is simple and stable

type Cache struct {
    data map[string]interface{}
}

func NewCache() *Cache {
    return &Cache{data: make(map[string]interface{})}
}

// Use interfaces when:
// - You have or might have multiple implementations
// - You need to mock in tests
// - You want loose coupling
```

---

## 19. Best Practices Summary

1. **Define interfaces at the consumer side** — not the producer
2. **Keep interfaces small** — 1-3 methods is ideal
3. **Use standard library interfaces** — `io.Reader`, `io.Writer`, etc.
4. **Accept interfaces, return concrete types** — makes testing easier
5. **Don't create interfaces prematurely** — create when you need them
6. **Use interface composition** — combine small interfaces
7. **Document interface contracts** — what does each method do?

---

## Next: [Error Handling →](./08-error-handling.md)
