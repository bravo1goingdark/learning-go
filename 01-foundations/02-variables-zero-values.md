# 2. Variables & Zero Values — Complete Deep Dive

> **Goal:** Master every way to declare variables, understand zero values, and know the subtle gotchas that trip up production code.

---

## Table of Contents

1. [Basic Types](#1-basic-types)
2. [Variable Declaration Forms](#2-variable-declaration-forms)
3. [Zero Values (Complete Reference)](#3-zero-values-complete-reference)
4. [Short Declaration (`:=`)](#4-short-declaration-)
5. [Type Inference](#5-type-inference)
6. [Constants](#6-constants)
7. [Iota (Enumerations)](#7-iota-enumerations)
8. [Blank Identifier (`_`)](#8-blank-identifier-_)
9. [Type Conversions](#9-type-conversions)
10. [Scope & Shadowing](#10-scope--shadowing)
11. [Production Patterns](#11-production-patterns)
12. [Common Pitfalls](#12-common-pitfalls)

---

## 1. Basic Types

### Numeric Types

| Type | Size | Description | Zero Value |
|------|------|-------------|------------|
| `bool` | 1 byte | `true` or `false` | `false` |
| `int` | 32 or 64 bits | Platform-dependent | `0` |
| `int8` | 8 bits | -128 to 127 | `0` |
| `int16` | 16 bits | -32768 to 32767 | `0` |
| `int32` | 32 bits | ~±2.1 billion | `0` |
| `int64` | 64 bits | ~±9.2 quintillion | `0` |
| `uint` | 32 or 64 bits | Platform-dependent unsigned | `0` |
| `uint8` | 8 bits | 0 to 255 (alias: `byte`) | `0` |
| `uint16` | 16 bits | 0 to 65535 | `0` |
| `uint32` | 32 bits | ~4.3 billion | `0` |
| `uint64` | 64 bits | ~18.4 quintillion | `0` |
| `uintptr` | platform | Large enough to store pointer | `0` |
| `float32` | 32 bits | ~6-7 decimal digits precision | `0.0` |
| `float64` | 64 bits | ~15-16 decimal digits precision | `0.0` |
| `complex64` | 64 bits | float32 real + imaginary | `(0+0i)` |
| `complex128` | 128 bits | float64 real + imaginary | `(0+0i)` |

### String & Byte Types

| Type | Description | Zero Value |
|------|-------------|------------|
| `string` | Immutable sequence of bytes (UTF-8) | `""` |
| `byte` | Alias for `uint8` | `0` |
| `rune` | Alias for `int32`, represents Unicode code point | `0` |

### `int` vs `int64`

```go
// On 64-bit systems: int == int64
// On 32-bit systems: int == int32
// This matters for:
var x int = 1000000000000000000  // Works on 64-bit, overflows on 32-bit
var y int64 = 1000000000000000000 // Always works
```

**Rule:** Use `int` for counts, indices, lengths. Use specific sizes (`int64`) when interfacing with databases, protobuf, or APIs.

---

## 2. Variable Declaration Forms

### Form 1: `var` with Type (Package Level)

```go
package main

// Package-level variables — must use 'var' keyword
var name string          // Zero value: ""
var age int              // Zero value: 0
var active bool          // Zero value: false
var score float64        // Zero value: 0.0
var data []byte          // Zero value: nil

func main() {
    // These are valid here
    fmt.Println(name, age, active, score, data)
}
```

### Form 2: `var` with Initializer

```go
var name string = "Alice"
var age int = 30
var active bool = true
```

### Form 3: `var` with Type Inference

```go
var name = "Alice"       // string inferred
var age = 30             // int inferred
var active = true        // bool inferred
var pi = 3.14159         // float64 inferred
```

### Form 4: Short Declaration `:=` (Function Scope Only)

```go
func main() {
    name := "Alice"      // string
    age := 30            // int
    active := true       // bool
    pi := 3.14159        // float64
}
```

### Form 5: Multiple Declaration

```go
var (
    name   string = "Alice"
    age    int    = 30
    active bool   = true
)
```

```go
// Short form
name, age, active := "Alice", 30, true
```

### Form 6: `new()` — Returns Pointer

```go
p := new(int)     // p is *int, points to zero value (0)
*p = 42           // Now the int is 42

s := new(string)  // s is *string, points to ""
*s = "hello"
```

**Rarely used.** Prefer `&TypeName{}` for structs:

```go
// Preferred
p := &Person{Name: "Alice"}

// Not preferred
p := new(Person)
p.Name = "Alice"
```

---

## 3. Zero Values (Complete Reference)

**Every variable in Go is initialized to its zero value.** There is no uninitialized memory. This is a core Go design decision.

### Zero Value Table

| Type | Zero Value | Notes |
|------|------------|-------|
| `bool` | `false` | |
| `int`, `int8/16/32/64` | `0` | |
| `uint`, `uint8/16/32/64` | `0` | |
| `float32`, `float64` | `0.0` | |
| `complex64`, `complex128` | `(0+0i)` | |
| `string` | `""` | Empty string, NOT nil |
| `byte` | `0` | Alias for uint8 |
| `rune` | `0` | Alias for int32 |
| `pointer` | `nil` | |
| `slice` | `nil` | |
| `map` | `nil` | |
| `channel` | `nil` | |
| `function` | `nil` | |
| `interface` | `nil` | |
| `struct` | All fields zero | Struct itself is NOT nil |
| `array` | All elements zero | Array itself is NOT nil |

![Zero Values Overview](assets/zeroVar.png)

### Zero Values in Action

```go
package main

import "fmt"

type User struct {
    Name    string
    Age     int
    Active  bool
    Score   float64
    Tags    []string
    Meta    map[string]string
}

func main() {
    var u User
    fmt.Printf("%+v\n", u)
    // Output: {Name: Age:0 Active:false Score:0 Tags:[] Meta:map[]}
    //         ↑ all fields are usable (except Tags/Meta — see below)
}
```

### Zero Values Are Usable (Mostly)

```go
var s string
fmt.Println(s)           // "" — prints nothing, no panic
fmt.Println(len(s))      // 0 — string functions work on zero strings
fmt.Println(s == "")     // true

var n int
fmt.Println(n)           // 0
fmt.Println(n + 1)       // 1 — arithmetic works

var b bool
fmt.Println(b)           // false
fmt.Println(!b)          // true — boolean operations work
```

### Zero Values That Are NOT Usable

```go
var m map[string]int
m["key"] = 1             // PANIC: assignment to entry in nil map

var s []int
s = append(s, 1)         // WORKS! append handles nil slices

var c chan int
c <- 1                   // PANIC: send on nil channel

var fn func()
fn()                     // PANIC: nil function call
```

**Key insight:** 
- **Usable:** `string`, `int`, `bool`, `float`, `struct`, `array`, `slice` (for reading)
- **NOT usable:** `map` (write), `channel` (send/recv), `function` (call), `pointer` (dereference)

### Zero Value Design Philosophy

In Java/TS:
```java
String name;           // null — NPE risk
Integer age;           // null — NPE risk
Map<String, int> meta; // null — NPE risk
```

In Go:
```go
var name string           // "" — always safe
var age int               // 0 — always safe
var meta map[string]int   // nil — safe for READ, panics on WRITE
```

Go eliminates the "is this null?" question for basic types. You only need to check nil for pointers, maps, channels, functions, and interfaces.

---

## 4. Short Declaration (`:=`)

### Rules

1. **Only inside functions** — can't use `:=` at package level
2. **Creates a new variable** in the current scope — unless already declared
3. **At least one NEW variable** must appear on the left side

```go
func main() {
    x := 10        // new variable x
    x := 20        // COMPILE ERROR: no new variable
    x = 20         // OK — assignment, not declaration
    
    x, y := 10, 20 // x already exists, y is new — OK
}
```

### The Multi-Return Trap

```go
func getUser() (string, int, error) {
    return "Alice", 30, nil
}

func main() {
    // WRONG — creates new err each time
    name, age, err := getUser()
    if err != nil { return }
    
    name, age, err := getUser()  // COMPILE ERROR: err already declared
    // Fix:
    name, age, err = getUser()   // Use = not :=
}
```

### Block Scope Trap

```go
func main() {
    x := 10
    
    if true {
        x := 20    // NEW variable x — shadows outer x
        fmt.Println(x)  // 20
    }
    
    fmt.Println(x)     // 10 — outer x unchanged
}
```

---

## 5. Type Inference

Go infers types from the right-hand side of `:=` or `var` without explicit type.

```go
x := 42              // int
y := 3.14            // float64 (NOT float32)
z := "hello"         // string
b := true            // bool
c := 'A'             // rune (int32)
f := func() {}       // func()

// Composite types
s := []int{1, 2, 3}           // []int
m := map[string]int{"a": 1}   // map[string]int
ch := make(chan int)          // chan int
```

### Numeric Literals

```go
x := 42        // int (decimal)
x := 0x2A      // int (hex)
x := 052       // int (octal) — leading zero!
x := 0b101010  // int (binary)
x := 1_000_000 // int (underscores for readability, Go 1.13+)

y := 3.14      // float64
y := 1e10      // float64 (scientific notation)
y := 0x1p-2    // float64 (hex float)

c := 1 + 2i    // complex128
```

### Explicit Type When Needed

```go
var x int32 = 42     // Force int32, not int
var y float32 = 3.14 // Force float32, not float64
```

---

## 6. Constants

### Basic Constants

```go
const Pi = 3.14159
const MaxRetries = 3
const Hostname = "api.example.com"
```

Constants are evaluated at **compile time**, not runtime.

### Typed vs Untyped Constants

```go
const typed int = 42         // Typed — can only assign to int
const untyped = 42           // Untyped — adapts to context
```

```go
const x = 42    // untyped constant
var a int = x   // OK — x adapts to int
var b int64 = x // OK — x adapts to int64
var c float64 = x // OK — x adapts to float64

const y int = 42 // typed constant
var d int64 = y  // COMPILE ERROR: cannot use y (type int) as type int64
```

**Untyped constants are more flexible.** They have arbitrary precision.

```go
const Huge = 1 << 100        // This compiles — arbitrary precision
// var x int = Huge           // This would overflow at runtime
var x = Huge / (1 << 99)     // 8 — evaluated at compile time, no overflow
```

### Constant Block

```go
const (
    StatusOK       = 200
    StatusNotFound = 404
    StatusError    = 500
    
    DefaultTimeout = 30  // seconds
    MaxConnections = 100
)
```

### `iota` for Enums

```go
type Direction int

const (
    North Direction = iota  // 0
    East                    // 1
    South                   // 2
    West                    // 3
)
```

### Constants Can Be Functions

```go
const (
    SizeOfInt    = unsafe.Sizeof(int(0))
    // ^ evaluated at compile time because unsafe.Sizeof is a compile-time function
)
```

Constants CANNOT be:
- Computed from function calls (except compile-time functions)
- Created from variables
- Assigned at runtime

---

## 7. Iota (Enumerations)

`iota` is a counter that increments in each `const` line.

### Basic Usage

```go
const (
    A = iota  // 0
    B         // 1
    C         // 2
    D         // 3
)
```

### Skipping Values

```go
const (
    _  = iota  // 0 (discarded)
    KB = 1 << (10 * iota)  // 1 << 10 = 1024
    MB                          // 1 << 20 = 1048576
    GB                          // 1 << 30 = 1073741824
    TB                          // 1 << 40
)
```

### Bitmask Flags

```go
type Permission uint8

const (
    Read    Permission = 1 << iota  // 0001
    Write                           // 0010
    Execute                         // 0100
    All     = Read | Write | Execute // 0111
)

func main() {
    var p Permission = Read | Write
    fmt.Println(p & Read != 0)   // true
    fmt.Println(p & Execute != 0) // false
}
```

### Weekday Enum with String

```go
type Weekday int

const (
    Sunday Weekday = iota
    Monday
    Tuesday
    Wednesday
    Thursday
    Friday
    Saturday
)

func (d Weekday) String() string {
    return [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}[d]
}

func main() {
    fmt.Println(Monday)  // "Monday" (Stringer interface)
}
```

### Production Enum Pattern

```go
type Status int

const (
    StatusPending Status = iota + 1  // Start at 1
    StatusActive
    StatusInactive
    StatusDeleted
)

// String() makes Status implement fmt.Stringer
func (s Status) String() string {
    switch s {
    case StatusPending:
        return "pending"
    case StatusActive:
        return "active"
    case StatusInactive:
        return "inactive"
    case StatusDeleted:
        return "deleted"
    default:
        return "unknown"
    }
}

// MarshalText implements encoding.TextMarshaler for JSON/YAML
func (s Status) MarshalText() ([]byte, error) {
    return []byte(s.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (s *Status) UnmarshalText(data []byte) error {
    switch string(data) {
    case "pending":
        *s = StatusPending
    case "active":
        *s = StatusActive
    case "inactive":
        *s = StatusInactive
    case "deleted":
        *s = StatusDeleted
    default:
        return fmt.Errorf("unknown status: %s", data)
    }
    return nil
}
```

---

## 8. Blank Identifier (`_`)

The blank identifier discards values. It's a write-only variable.

### Discard Return Values

```go
// Function returns (result, error) — ignore result
_, err := doSomething()
if err != nil {
    return err
}

// Function returns (result, error) — ignore error (RARE — only if you truly don't care)
result, _ := doSomething()
```

### Import for Side Effects

```go
import (
    _ "net/http/pprof"    // Registers HTTP handlers, doesn't import names
    _ "github.com/lib/pq" // Registers database driver
    _ "image/png"         // Registers PNG decoder
)
```

### Unused Variables

```go
// COMPILE ERROR: x declared but not used
func main() {
    x := 10
}

// FIX: assign to blank
func main() {
    x := 10
    _ = x  // OK
}
```

### Range Without Index/Value

```go
// Only need value
for _, v := range items {
    fmt.Println(v)
}

// Only need index
for i := range items {
    fmt.Println(i)
}

// Neither
for range items {
    fmt.Println("loop")
}
```

---

## 9. Type Conversions

Go requires **explicit** type conversions. No implicit conversions.

### Numeric Conversions

```go
var i int = 42
var f float64 = float64(i)   // Must cast
var u uint = uint(f)          // Must cast

// This won't compile:
// var f float64 = i  // COMPILE ERROR: cannot use i (type int) as type float64
```

### String Conversions

```go
// String to []byte
s := "hello"
b := []byte(s)     // Creates a copy
s2 := string(b)    // Back to string

// String to []rune
runes := []rune("Hello, 世界")
s3 := string(runes)

// Int to string — CAREFUL!
s4 := string(65)          // "A" — interprets as Unicode code point!
s5 := strconv.Itoa(65)    // "65" — what you usually want
```

### Interface Conversions

```go
var i interface{} = "hello"

// Type assertion (runtime check)
s, ok := i.(string)    // s = "hello", ok = true
n, ok := i.(int)       // n = 0, ok = false

// Type assertion without ok (panics on failure)
s := i.(string)        // Works
n := i.(int)           // PANIC
```

### Unsafe Conversions (Rare)

```go
import "unsafe"

// Convert string to []byte WITHOUT copying (dangerous!)
func stringToBytes(s string) []byte {
    return *(*[]byte)(unsafe.Pointer(
        &struct {
            string
            Cap int
        }{s, len(s)},
    ))
}
```

**Don't do this unless you know exactly what you're doing.** The normal `[]byte(s)` is fine for 99% of cases.

---

## 10. Scope & Shadowing

### Scope Levels

```go
package main

var packageLevel = "I'm accessible everywhere in this package"

func outer() {
    var functionLevel = "I'm accessible in outer() and nested blocks"
    
    if true {
        var blockLevel = "I'm accessible only in this if block"
        fmt.Println(blockLevel, functionLevel, packageLevel)
    }
    
    // fmt.Println(blockLevel)  // COMPILE ERROR: blockLevel undefined
}
```

### Variable Shadowing

Shadowing occurs when a variable in an inner scope has the same name as one in an outer scope.

```go
package main

import "fmt"

var x = "package-level"

func main() {
    fmt.Println(x)  // "package-level"
    
    x := "function-level"  // Shadows package-level x
    fmt.Println(x)         // "function-level"
    
    if true {
        x := "block-level"  // Shadows function-level x
        fmt.Println(x)      // "block-level"
    }
    
    fmt.Println(x)  // "function-level" — back to function scope
}
```

### The Import Shadowing Trap

```go
import "fmt"

func main() {
    fmt := "oops"     // Shadows the fmt package!
    fmt.Println("hi") // COMPILE ERROR: fmt.Println undefined (type string has no field or method Println)
}
```

### The `:=` Shadowing Trap

```go
func main() {
    x, err := doA()  // x and err created
    if err != nil { return }
    
    if true {
        x, err := doB()  // NEW x and err — shadows outer ones!
        if err != nil { return }
        use(x)
    }
    
    use(x)  // Uses the OUTER x, not the inner one
    // The inner err is gone — outer err is still from doA()
}
```

**Fix:** Always check which variables are new in a `:=` statement.

---

## 11. Production Patterns

### Configuration Pattern

```go
type Config struct {
    Host     string
    Port     int
    Debug    bool
    Timeout  time.Duration
}

func DefaultConfig() Config {
    return Config{
        Host:    "localhost",
        Port:    8080,
        Debug:   false,
        Timeout: 30 * time.Second,
    }
}

func main() {
    cfg := DefaultConfig()
    // Override from env/flags...
}
```

### Functional Options Pattern

```go
type Server struct {
    host    string
    port    int
    timeout time.Duration
    maxConn int
}

type Option func(*Server)

func WithHost(host string) Option {
    return func(s *Server) { s.host = host }
}

func WithPort(port int) Option {
    return func(s *Server) { s.port = port }
}

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func NewServer(opts ...Option) *Server {
    s := &Server{
        host:    "localhost",
        port:    8080,
        timeout: 30 * time.Second,
        maxConn: 1000,
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
srv := NewServer(
    WithHost("0.0.0.0"),
    WithPort(9090),
    WithTimeout(60 * time.Second),
)
```

### Sentinel Values

```go
var (
    ErrNotFound    = errors.New("not found")
    ErrTimeout     = errors.New("timeout")
    ErrPermission  = errors.New("permission denied")
)

// In production, you'd use custom error types instead of sentinels for wrapping
```

---

## 12. Common Pitfalls

### 1. Integer Overflow

```go
var x int8 = 127
x++          // x = -128 — wraps around, no error!
```

**Fix:** Use larger types, or check bounds explicitly.

### 2. Float Comparison

```go
a := 0.1 + 0.2
b := 0.3
fmt.Println(a == b)  // false!

// Fix: use tolerance
const epsilon = 1e-9
if math.Abs(a-b) < epsilon {
    // Close enough
}
```

### 3. String is Immutable

```go
s := "hello"
s[0] = 'H'  // COMPILE ERROR: cannot assign to s[0]

// Fix: convert to []byte
b := []byte(s)
b[0] = 'H'
s = string(b)  // "Hello"
```

### 4. Rune vs Byte

```go
s := "Hello, 世界"
fmt.Println(len(s))           // 13 (bytes)
fmt.Println(len([]rune(s)))   // 9 (runes/characters)

for i, r := range s {
    fmt.Printf("byte %d: rune %c\n", i, r)
}
// Iterates by rune, but index is in bytes
```

### 5. Zero Value Slices/Maps

```go
var s []int
fmt.Println(s == nil)  // true
fmt.Println(len(s))    // 0
s = append(s, 1)       // WORKS — nil slice append returns new slice

var m map[string]int
fmt.Println(m == nil)  // true
m["key"] = 1           // PANIC — must initialize first
m = make(map[string]int)
m["key"] = 1           // OK
```

### 6. The `:=` with Multiple Return Values

```go
// Bug: shadowing the error variable
func process() error {
    data, err := readData()
    if err != nil { return err }
    
    // This creates a NEW err variable in the if block
    if result, err := transform(data); err != nil {
        return err  // Returns the inner err — correct
    } else {
        use(result)
    }
    // err here is still the original err from readData()
    // This is confusing — avoid this pattern
}
```

---

## Quick Reference

```go
// Declaration forms
var x int                    // Zero value
var x int = 42               // Explicit type + value
var x = 42                   // Inferred type
x := 42                      // Short form (function scope only)
var (x, y, z int)            // Multiple declaration
x, y := 1, 2                 // Multiple short declaration

// Constants
const x = 42                 // Untyped
const x int = 42             // Typed
const (A, B, C = iota, iota, iota) // 0, 1, 2

// Type conversion
f := float64(i)              // Explicit conversion required
s := string(b)               // []byte to string
b := []byte(s)               // string to []byte
s := strconv.Itoa(i)         // int to decimal string

// Blank identifier
_, err := doSomething()      // Discard first return
_ = unusedVar                // Silence "unused" error
import _ "pkg"               // Import for side effects
```

---

## 13. Production Patterns

### Configuration with Zero Values

```go
type Config struct {
    Host        string
    Port        int
    Timeout     time.Duration
    MaxRetries  int
    Debug       bool
}

func DefaultConfig() *Config {
    return &Config{
        Host:       "localhost",
        Port:       8080,
        Timeout:    30 * time.Second,
        MaxRetries: 3,
        Debug:      false,
    }
}

func LoadConfig() *Config {
    cfg := DefaultConfig()
    
    if v := os.Getenv("HOST"); v != "" {
        cfg.Host = v
    }
    if v := os.Getenv("PORT"); v != "" {
        if port, err := strconv.Atoi(v); err == nil {
            cfg.Port = port
        }
    }
    
    return cfg
}
```

### Option Pattern for Configuration

```go
type Server struct {
    host string
    port int
}

type ServerOption func(*Server)

func WithHost(host string) ServerOption {
    return func(s *Server) {
        s.host = host
    }
}

func WithPort(port int) ServerOption {
    return func(s *Server) {
        s.port = port
    }
}

func NewServer(opts ...ServerOption) *Server {
    s := &Server{host: "localhost", port: 8080}
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
server := NewServer(WithHost("0.0.0.0"), WithPort(9090))
```

### Functional Options Pattern

```go
type Cache struct {
    maxSize    int
    ttl        time.Duration
    onEvict    func(key, value interface{})
}

type CacheOption func(*Cache)

func MaxSize(n int) CacheOption {
    return func(c *Cache) { c.maxSize = n }
}

func TTL(d time.Duration) CacheOption {
    return func(c *Cache) { c.ttl = d }
}

func OnEvict(fn func(key, value interface{})) CacheOption {
    return func(c *Cache) { c.onEvict = fn }
}

func NewCache(opts ...CacheOption) *Cache {
    c := &Cache{maxSize: 1000, ttl: time.Hour}
    for _, opt := range opts {
        opt(c)
    }
    return c
}
```

---

## 14. Working with Constants

### Iota Usage

```go
const (
    StatusPending int = iota  // 0
    StatusRunning             // 1
    StatusSuccess             // 2
    StatusFailed              // 3
)

// With bit flags
const (
    FlagRead  = 1 << iota // 1
    FlagWrite             // 2
    FlagExecute           // 4
)

// Custom iota values
const (
    _ = iota
    KB = 1 << (10 * iota) // 1024
    MB                    // 1048576
    GB                    // 1073741824
)
```

### Typed vs Untyped Constants

```go
const Pi = 3.14159              // Untyped
const PiFloat float64 = 3.14159 // Typed

func demo() {
    var f float64 = Pi      // OK
    var c complex128 = Pi  // OK
    var f2 float64 = PiFloat // OK
    // var c2 complex128 = PiFloat // ERROR
}
```

---

## 15. Zero Value Gotchas

### Slices (Safe to use nil)

```go
func appendToNil() {
    var s []int
    s = append(s, 1, 2, 3) // Works fine
}
```

### Maps (Reading safe, writing panics)

```go
func mapNil() {
    var m map[string]int
    _ = m["key"]   // OK - returns 0
    // m["key"] = 1 // PANIC!
}

func mapSafe() {
    m := make(map[string]int)
    m["key"] = 1 // OK
}
```

### Pointers (Writing panics)

```go
func pointerNil() {
    var p *User
    // p.Name = "Alice" // PANIC!
}

func pointerSafe() {
    p := &User{}
    p.Name = "Alice" // OK
}
```

---

## Next: [Arrays vs Slices →](./03-arrays-vs-slices.md)
