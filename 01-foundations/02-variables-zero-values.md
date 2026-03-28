# 2. Variables & Zero Values — Complete Deep Dive

> **Goal:** Master every way to declare variables, understand zero values, and know the subtle gotchas that trip up production code.

---
![Zero Values Overview](../assets/02.png)
## Table of Contents

1. [Basic Types](#1-basic-types) `[CORE]`
2. [Variable Declaration Forms](#2-variable-declaration-forms) `[CORE]`
3. [Zero Values (Complete Reference)](#3-zero-values-complete-reference) `[CORE]`
4. [Short Declaration (`:=`)](#4-short-declaration-) `[CORE]`
5. [Type Inference](#5-type-inference) `[CORE]`
6. [Constants](#6-constants) `[CORE]`
7. [Iota (Enumerations)](#7-iota-enumerations) `[CORE]`
8. [Blank Identifier (`_`)](#8-blank-identifier-_) `[CORE]`
9. [Type Conversions](#9-type-conversions) `[CORE]`
10. [Scope & Shadowing](#10-scope--shadowing) `[CORE]`
11. [Production Patterns](#11-production-patterns) `[PRODUCTION]`
12. [Common Pitfalls](#12-common-pitfalls) `[CORE]`

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

**When to use which form:** The choice depends on scope, initialization needs, and readability. At package level, you must use `var` since `:=` isn't allowed. Within functions, prefer `:=` for local variables since it's more concise and the type is usually obvious from context. Use explicit `var` when you want zero value initialization without specifying a type, or when the variable might be reassigned later (making `:=` inappropriate). Use `var` with explicit type when the type isn't obvious from the right-hand side or when you want to make the type explicit for clarity.

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
    fmt.Println(name, age, active, score, data) // fmt.Println prints values to stdout
}
```

> **Quick reference — `fmt` package:** `fmt` is Go's standard formatted I/O package. The most-used functions:
> - `fmt.Println(a, b)` — print values separated by spaces, add newline
> - `fmt.Printf("name: %s, age: %d\n", name, age)` — formatted print (`%s`=string, `%d`=int, `%v`=default, `%+v` with field names)
> - `fmt.Sprintf(...)` — like Printf but returns a string instead of printing
> - `fmt.Errorf("msg: %w", err)` — create a formatted error (covered in error handling)

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
    name, age, err := getUser()  // OK — declares err
    if err != nil { return }
    
    name, age, err := getUser()  // COMPILE ERROR: err already declared
    // Fix: use = not :=
    name, age, err = getUser()   // OK — reassigns existing err
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
    SizeOfInt    = unsafe.Sizeof(int(0))  // unsafe.Sizeof returns the size in bytes (compile-time constant)
    // unsafe is Go's escape hatch from the type system — use sparingly.
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

> **`range` quick reference:** `for i, v := range items` iterates over a slice/map/channel, giving you the index (`i`) and value (`v`) each time. Use `_` to discard whichever you don't need. Works on slices (index+value), maps (key+value), strings (rune offset+char), and channels (value only).

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
// strconv.Itoa converts an integer to its decimal string ("65" not "A").
// The reverse is strconv.Atoi("65") → (65, nil).
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

> ⏭️ **First pass? Skip this section.** The Functional Options Pattern uses closures (covered in Topic 11). Come back after completing Phase 5.

### Configuration Pattern

```go
type Config struct {
    Host     string
    Port     int
    Debug    bool
    Timeout  time.Duration  // time.Duration represents a time span (e.g., 30*time.Second = 30s)
}

func DefaultConfig() Config {
    return Config{
        Host:    "localhost",
        Port:    8080,
        Debug:   false,
        Timeout: 30 * time.Second, // multiply a number by time.Second/Minute/Hour to create a Duration
    }
}

func main() {
    cfg := DefaultConfig()
    // Override from env/flags...
}
```

### Functional Options Pattern

**When to use this pattern:** Use functional options when a struct has multiple optional fields that may be omitted, and you want a clear API without requiring users to set dozens of parameters or remember pointer/nil conventions. It's particularly valuable when: (1) most fields have sensible defaults, (2) you expect the struct to grow more optional fields over time (backward compatible), or (3) you want named, self-documenting configuration. For simple cases with only 1-2 optional fields, plain function parameters or setters are simpler.

> **Prerequisites:** This pattern uses functions as values, variadic parameters (`...`), and closures — concepts not yet covered in detail. Here's the quick version:
> - `type Option func(*Server)` defines `Option` as a function type that takes a `*Server`.
> - `func WithHost(h string) Option { return func(s *Server) { s.host = h } }` returns a closure — an anonymous function that "remembers" the `host` value.
> - `NewServer(opts ...Option)` accepts a variable number of `Option` functions.
> - Full coverage of functions and closures in the concurrency section (Topic 11).

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

**When to use vs custom error types:** Sentinel errors (predefined error values like `ErrNotFound`) work well when you need to compare errors exactly using `==`. However, they have a limitation: you can't attach additional context without wrapping, and adding new sentinel values requires importing the package. Custom error types (structs implementing the `error` interface) are better when you need to carry payload data or when the error category is open-ended. For most production code, wrapping errors with `fmt.Errorf("%w", err)` is preferred over sentinel comparisons.

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

**Why this happens:** Go's integer types are fixed-size and overflow wraps silently because checking for overflow on every arithmetic operation would impose a significant performance cost. The language chooses performance and simplicity over safety for numeric operations.

```go
var x int8 = 127
x++          // x = -128 — wraps around, no error!
```

**Fix:** Use larger types, or check bounds explicitly.

### 2. Float Comparison

**Why this happens:** Floating-point numbers are stored in binary, and some decimal values (like 0.1, 0.2, 0.3) cannot be represented exactly in binary. The small representation error accumulates during arithmetic, making seemingly equal values slightly different. This is a fundamental property of floating-point arithmetic across all programming languages, not a Go bug.

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

**Why this exists:** String immutability provides several benefits: (1) strings can be safely shared between goroutines without synchronization, (2) string keys in maps are safe from modification, (3) memory can be optimized by reusing underlying byte arrays, and (4) it prevents a class of bugs where modifying a string unexpectedly affects other code holding a reference to it.

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

## Exercises

### Exercise 1: Print Zero Values ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Declare one variable of each basic type (`bool`, `int`, `float64`, `string`) without assigning a value. Print each with `fmt.Printf("%+v\n", ...)` to see its zero value.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func main() {
	var b bool
	var i int
	var f float64
	var s string

	fmt.Printf("bool:   %v\n", b)
	fmt.Printf("int:    %v\n", i)
	fmt.Printf("float:  %v\n", f)
	fmt.Printf("string: %q\n", s)
}
// Output:
// bool:   false
// int:    0
// float:  0
// string: ""
```

</details>

### Exercise 2: Predict Shadowing Output ⭐⭐
**Difficulty:** Beginner | **Time:** ~10 min

Read the code below. Predict every line of output before running it.

```go
package main

import "fmt"

var x = "package"

func main() {
	fmt.Println(x)
	x := "function"
	{
		x := "block"
		fmt.Println(x)
	}
	fmt.Println(x)
}
```

<details>
<summary>Answer</summary>

```
package
block
function
```

</details>

### Exercise 3: Type Conversions ⭐⭐
**Difficulty:** Beginner | **Time:** ~10 min

Write a program that: (1) declares an `int` value of `65`, (2) converts it to `float64` and prints it, (3) converts the `int` to a `string` using `strconv.Itoa` and prints it.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"strconv"
)

func main() {
	i := 65

	f := float64(i)
	fmt.Println("float64:", f)

	s := strconv.Itoa(i)
	fmt.Println("string:", s)
}
```

</details>

### Exercise 4: Blank Identifier for Error ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Use `strconv.Atoi` to convert the string `"42"` to an integer. You are certain the input is valid, so discard the error with `_` and only keep the number.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"strconv"
)

func main() {
	num, _ := strconv.Atoi("42")
	fmt.Println("parsed:", num)
}
```

</details>

---

## Next: [Arrays vs Slices →](./03-arrays-vs-slices.md)
