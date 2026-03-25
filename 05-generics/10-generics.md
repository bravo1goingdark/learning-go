# 10. Generics (Type Parameters) — Production-Grade Deep Dive

> **Goal:** Master Go generics (1.18+) at a production level. Build type-safe, high-performance systems without runtime overhead.

---

## Table of Contents

1. [Why Generics?](#1-why-generics)
2. [Type Parameters Basics](#2-type-parameters-basics)
3. [Type Constraints](#3-type-constraints)
4. [Predeclared Constraints](#4-predeclared-constraints)
5. [Custom Constraints with Interfaces](#5-custom-constraints-with-interfaces)
6. [Type Sets and the ~ Operator](#6-type-sets-and-the--operator)
7. [Generic Structs](#7-generic-structs)
8. [Generic Methods](#8-generic-methods)
9. [Generic Interfaces](#9-generic-interfaces)
10. [Comparable Constraint Deep Dive](#10-comparable-constraint-deep-dive)
11. [Type Inference](#11-type-inference)
12. [Common Production Patterns](#12-common-production-patterns)
13. [Generics vs Interfaces Performance](#13-generics-vs-interfaces-performance)
14. [Internals: How Go Implements Generics](#14-internals-how-go-implements-generics)
15. [Testing Generic Code](#15-testing-generic-code)
16. [Common Pitfalls](#16-common-pitfalls)
17. [Production Guidelines](#17-production-guidelines)

---

## 1. Why Generics?

Generics solve the tradeoff between **reusability** and **type safety**. Before Go 1.18, you had to choose.

### The Pre-Generics Problem

```go
// Approach 1: Duplication — type-safe but maintenance nightmare
func SumInts(is []int) int {
    var total int
    for _, n := range is { total += n }
    return total
}
func SumFloats(fs []float64) float64 {
    var total float64
    for _, n := range fs { total += n }
    return total
}

// Approach 2: interface{} — reusable but loses type safety
func SumAny(s []interface{}) interface{} {
    // Runtime type switches, reflection, runtime errors...
}
```

### The Generic Solution

```go
// Best of both worlds: reusable AND type-safe
func Sum[T Numeric](nums []T) T {
    var total T
    for _, n := range nums {
        total += n
    }
    return total
}

// Compile-time type checking, zero runtime cost
_ = Sum([]int{1, 2, 3})           // T = int
_ = Sum([]float64{1.5, 2.5})     // T = float64
_ = Sum([]myInt{1, 2, 3})        // T = myInt (if underlying is int)
```

### Why This Matters in Production

| Aspect | Without Generics | With Generics |
|--------|------------------|---------------|
| **Code duplication** | High (per-type functions) | Zero |
| **Type safety** | Runtime errors possible | Compile-time guarantee |
| **Maintainability** | Hard (change in one, update all) | Easy (change once) |
| **Binary size** | Larger with duplication | Smaller with monomorphization |
| **Performance** | Interface indirection overhead | Direct calls, inlined |

---

## 2. Type Parameters Basics

Type parameters are compile-time type variables. They exist only during compilation and are substituted with concrete types.

### Syntax Breakdown

```go
func FunctionName[T, U, V any](
    param1 T,
    param2 U,
) V {
    // T, U, V are available as types here
}
```

- `T, U, V` — type parameter names (convention: single uppercase letters or descriptive names)
- `any` — constraint (what types are allowed)
- Multiple type parameters separated by commas

### Simple Examples

```go
// Single type parameter
func First[T any](s []T) T {
    if len(s) == 0 {
        var zero T
        return zero
    }
    return s[0]
}

// Multiple type parameters
func Map[T any, U any](s []T, fn func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s {
        result[i] = fn(v)
    }
    return result
}

// Type parameters on struct
type Pair[K, V any] struct {
    Key   K
    Value V
}
```

### Instantiation

When you call a generic function, Go **instantiates** it with concrete types:

```go
result := First([]int{1, 2, 3})
// At compile time, Go generates essentially:
// func First_int(s []int) int { ... }
```

This is called **monomorphization** — compile-time code generation for each type combination. In practice, this means the compiler creates a separate, specialized version of the function for each concrete type you use. `First([]int{...})` and `First([]string{...})` become two distinct functions in the binary, each optimized for its type. The result: zero runtime overhead compared to writing the function by hand for each type.

---

## 3. Type Constraints

Constraints restrict which types can be used with a type parameter. They're defined as interfaces.

### Why Constraints Matter

```go
// Without constraint: accepts ANY type
func Identity[T any](v T) T { return v }

// With constraint: accepts only comparable types
func Unique[T comparable](s []T) []T {
    seen := make(map[T]struct{})
    var result []T
    for _, v := range s {
        if _, ok := seen[v]; !ok {
            seen[v] = struct{}{}
            result = append(result, v)
        }
    }
    return result
}
```

### Constraint Types

```go
// 1. Union constraint — any of these types
func SumIntsOrFloats[T int | int64 | float64](nums []T) T { ... }

// 2. Method constraint — types with specific behavior
func Stringify[T fmt.Stringer](v T) string { return v.String() }

// 3. Combined constraint — type union + methods
type Number interface {
    ~int | ~int32 | ~int64 | ~float64 | ~float32
    String() string
}
```

### Inline Constraints

```go
// Constraint defined inline
func Max[T int | float64](a, b T) T {
    if a > b { return a }
    return b
}

// Constraint defined separately (more reusable)
type Ordered interface {
    int | float64 | string
}

func Max[T Ordered](a, b T) T { ... }
```

---

## 4. Predeclared Constraints

Go provides built-in constraints for common use cases.

### `any` — Accept Anything

```go
// Equivalent to interface{}
func Identity[T any](v T) T { return v }

// any is most flexible but least restrictive
// Use when you truly don't care about the type
```

### `comparable` — Supports Equality

```go
// Required for ==, !=, and map keys
func Contains[T comparable](s []T, target T) bool {
    for _, v := range s {
        if v == target { return true }
    }
    return false
}

_ = Contains([]int{1, 2, 3}, 2)           // OK
_ = Contains([]string{"a", "b"}, "c")    // OK
// _ = Contains([][]int{{1}}, {1})        // ERROR: slices not comparable
```

### `cmp.Ordered` — Supports Comparison (Go 1.21+)

```go
import "cmp"

func Max[T cmp.Ordered](a, b T) T {
    if a > b { return a }
    return b
}

func Min[T cmp.Ordered](a, b T) T {
    if a < b { return a }
    return b
}

// Works with: int, uint, float, string, and their variants
```

### Legacy Constraints (Go < 1.21)

```go
// Before Go 1.21, used the constraints package
import "golang.org/x/exp/constraints"

func Max[T constraints.Ordered](a, b T) T { ... }
// Now deprecated in favor of cmp package
```

---

## 5. Custom Constraints with Interfaces

Create reusable constraints for your domain.

### Simple Type Constraints

```go
// Numeric types only
type Numeric interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
    ~float32 | ~float64
}

func Sum[T Numeric](nums []T) T {
    var total T
    for _, n := range nums { total += n }
    return total
}

// Custom type works too
type MyInt int
_ = Sum([]MyInt{1, 2, 3}) // OK — MyInt's underlying is int
```

### Method Constraints

```go
// Any type with String() method
type Stringer interface {
    String() string
}

func PrintAll[T Stringer](items []T) {
    for _, item := range items {
        fmt.Println(item.String())
    }
}

// Built-in types that satisfy Stringer
_ = PrintAll([]string{"a", "b", "c"})
```

### Combined Type + Method Constraints

```go
// Must be numeric AND implement specific method
type NumericStringer interface {
    ~int | ~float64
    String() string
}

type Score int

func (s Score) String() string {
    return fmt.Sprintf("Score: %d", s)
}

func Display[T NumericStringer](v T) {
    fmt.Println(v.String()) // Uses constraint method
}

_ = Display(Score(100))
```

---

## 6. Type Sets and the ~ Operator

The `~` prefix means "this type and any type with the same underlying type".

### Without `~` — Exact Type Match

```go
type ID int

type StrictInt interface {
    int  // Only exactly int, NOT ID
}

func Process[T StrictInt](v T) T { return v }

var x int = 42
var y ID = 42

_ = Process(x) // OK
_ = Process(y) // ERROR: ID is not int
```

### With `~` — Underlying Type Match

```go
type ID int

type LooseInt interface {
    ~int  // int AND any type with underlying int
}

func Process[T LooseInt](v T) T { return v }

var x int = 42
var y ID = 42

_ = Process(x) // OK
_ = Process(y) // OK — ID's underlying type is int
```

### Practical Production Example

```go
// Define a constraint for all ID-like types
type ID interface {
    ~int64 | ~uint64 | ~string
}

// Now any ID type works
type UserID int64
type OrderID uint64
type ProductID string

func GetByID[T ID](id T) error { ... }

_ = GetByID(UserID(1))
_ = GetByID(OrderID(2))
_ = GetByID(ProductID("prod-123"))
```

### Type Set Semantics

```go
// What does this mean?
type Number interface {
    ~int | ~float64
}

// Type set = { all types whose underlying type is int } U 
//           { all types whose underlying type is float64 }

// This includes:
int, int8, int16, int32, int64
uint, uint8, uint16, uint32, uint64
float32, float64
type MyInt int          // OK (underlying int)
type MyFloat float64    // OK (underlying float64)
```

---

## 7. Generic Structs

Generic structs enable type-safe containers without runtime overhead.

### Basic Generic Struct

```go
type Box[T any] struct {
    Value T
}

func (b Box[T]) Get() T { return b.Value }
func (b *Box[T]) Set(v T) { b.Value = v }

// Instantiation
intBox := Box[int]{Value: 42}
strBox := Box[string]{Value: "hello"}
```

### Generic Stack (Production-Ready)

```go
type Stack[T any] struct {
    items []T
}

func NewStack[T any]() *Stack[T] {
    return &Stack[T]{items: make([]T, 0)}
}

func (s *Stack[T]) Push(v T) {
    s.items = append(s.items, v)
}

func (s *Stack[T]) Pop() (T, bool) {
    if len(s.items) == 0 {
        var zero T
        return zero, false
    }
    idx := len(s.items) - 1
    v := s.items[idx]
    s.items = s.items[:idx]
    return v, true
}

func (s *Stack[T]) Peek() (T, bool) {
    if len(s.items) == 0 {
        var zero T
        return zero, false
    }
    return s.items[len(s.items)-1], true
}

func (s *Stack[T]) Len() int { return len(s.items) }
func (s *Stack[T]) IsEmpty() bool { return len(s.items) == 0 }

// Usage
stack := NewStack[int]()
stack.Push(10)
stack.Push(20)
v, ok := stack.Pop() // v=20, ok=true
```

### Generic Cache with Locking

```go
type Cache[K comparable, V any] struct {
    mu   sync.RWMutex
    data map[K]V
}

func NewCache[K comparable, V any]() *Cache[K, V] {
    return &Cache[K, V]{data: make(map[K]V)}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    v, ok := c.data[key]
    return v, ok
}

func (c *Cache[K, V]) Set(key K, value V) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[key] = value
}

func (c *Cache[K, V]) Delete(key K) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.data, key)
}

func (c *Cache[K, V]) Len() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return len(c.data)
}

// Thread-safe usage
cache := NewCache[string, User]()