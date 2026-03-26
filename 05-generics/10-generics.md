# 10. Generics (Type Parameters) — Production-Grade Deep Dive

> **Goal:** Master Go generics (1.18+) at a production level. Build type-safe, high-performance systems without runtime overhead.

---

## Table of Contents

1. [Why Generics?](#1-why-generics-core)
2. [Type Parameters Basics](#2-type-parameters-basics-core)
3. [Type Constraints](#3-type-constraints-core)
4. [Predeclared Constraints](#4-predeclared-constraints-core)
5. [Custom Constraints with Interfaces](#5-custom-constraints-with-interfaces-core)
6. [Type Sets and the ~ Operator](#6-type-sets-and-the--operator-core)
7. [Generic Structs](#7-generic-structs-core)
8. [Generic Methods](#8-generic-methods-production)
9. [Generic Interfaces](#9-generic-interfaces-production)
10. [Comparable Constraint Deep Dive](#10-comparable-constraint-deep-dive-core)
11. [Type Inference](#11-type-inference-core)
12. [Common Production Patterns](#12-common-production-patterns-production)
13. [Generics vs Interfaces Performance](#13-generics-vs-interfaces-performance-internals)
14. [Internals: How Go Implements Generics](#14-internals-how-go-implements-generics-internals)
15. [Testing Generic Code](#15-testing-generic-code-production)
16. [Common Pitfalls](#16-common-pitfalls-core)
17. [Production Guidelines](#17-production-guidelines-production)

---

## 1. Why Generics? [CORE]

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

## 2. Type Parameters Basics [CORE]

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

## 3. Type Constraints [CORE]

A constraint is a compile-time contract. It tells the compiler: "T can be any type, as long as it supports X." Without constraints, the compiler can't allow operations like `+` or `==` on T because it doesn't know if T supports them — what if T is a struct? Constraints bridge this gap by specifying what operations T must support.

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

## 4. Predeclared Constraints [CORE]

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

## 5. Custom Constraints with Interfaces [CORE]

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

## 6. Type Sets and the ~ Operator [CORE]

**The problem the tilde solves:** You define `type UserID int64`. You want a generic function that works with `int64` and `UserID`. Without `~`, the constraint `int64` rejects `UserID` because Go treats them as different types. The `~` prefix says "accept any type whose *underlying* type is int64," which includes `UserID`. This is essential when your codebase uses custom types for domain concepts (UserID, OrderID, Price — all `int64` underneath).

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

## 7. Generic Structs [CORE]

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
cache := NewCache[string, int]()
cache.Set("key", 42)
v, ok := cache.Get("key") // v=42, ok=true
```

---

## 8. Generic Methods [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

Go supports generic methods on structs and types, enabling type-safe operations on container types.

### Basic Generic Methods

```go
type Container[T any] struct {
    value T
}

// Generic method — type parameter on the method itself
func (c Container[T]) Transform[U any](fn func(T) U) U {
    return fn(c.value)
}

// Usage
c := Container[int]{value: 42}
result := c.Transform(func(n int) string {
    return fmt.Sprintf("value is %d", n)
}) // result = "value is 42", U inferred as string
```

### Generic Methods on Non-Generic Types

```go
type Logger struct{}

// The type parameter is on the method, not the struct
func (Logger) Log[T any](val T) {
    fmt.Printf("[LOG] %v\n", val)
}

log := Logger{}
log.Log(42)           // T = int
log.Log("hello")      // T = string
log.Log([]int{1, 2}) // T = []int
```

### Chaining with Generic Methods

```go
type Result[T any] struct {
    val T
    err error
}

func (r Result[T]) Map[U any](fn func(T) U) Result[U] {
    if r.err != nil {
        return Result[U]{err: r.err}
    }
    return Result[U]{val: fn(r.val)}
}

func (r Result[T]) FlatMap[U any](fn func(T) (U, error)) Result[U] {
    if r.err != nil {
        return Result[U]{err: r.err}
    }
    val, err := fn(r.val)
    return Result[U]{val: val, err: err}
}

// Chaining
result := Result[int]{val: 10}.
    Map(func(n int) int { return n * 2 }).
    Map(func(n int) string { return fmt.Sprintf("%d", n) })
// result.val = "20"
```

---

## 9. Generic Interfaces [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

Interfaces can have type parameters, enabling generic contracts for collections and algorithms.

### Basic Generic Interface

```go
type Repository[T any] interface {
    Save(entity T) error
    FindByID(id string) (T, error)
    Delete(id string) error
}

// Implement for User
type UserRepo struct{ /* ... */ }
func (r *UserRepo) Save(u User) error { /* ... */ }
func (r *UserRepo) FindByID(id string) (User, error) { /* ... */ }
func (r *UserRepo) Delete(id string) error { /* ... */ }
```

### Generic Interface with Multiple Type Parameters

```go
type Mapper[K comparable, V any] interface {
    Get(key K) (V, bool)
    Set(key K, value V)
    Delete(key K)
    Keys() []K
}

// Can be implemented by map, cache, distributed store, etc.
type MapMapper[K comparable, V any] struct {
    data map[K]V
}
```

### Constraint Interfaces as Type Parameters

```go
// An interface that itself takes type parameters
type Sortable[T any] interface {
    Len() int
    Less(i, j int) bool
    Swap(i, j int)
}

func Sort[T any, S Sortable[T]](s S) {
    for i := 0; i < s.Len(); i++ {
        for j := i + 1; j < s.Len(); j++ {
            if s.Less(j, i) {
                s.Swap(i, j)
            }
        }
    }
}
```

---

## 10. Comparable Constraint Deep Dive [CORE]

The `comparable` constraint is more nuanced than it appears. Understanding it prevents subtle bugs.

### What `comparable` Actually Means

`comparable` includes all types that support `==` and `!=`. This is a specific subset of all Go types.

```
  ╔══════════════════════════════════════════════════════════════════════════╗
  ║                     WHAT IS "comparable"?                               ║
  ╠══════════════════════════════════════════════════════════════════════════╣
  ║                                                                          ║
  ║  ╔════════════════════════════════════╗   ╔════════════════════════════╗║
  ║  ║      COMPARABLE (supports ==)     ║   ║    NOT COMPARABLE          ║║
  ║  ╠════════════════════════════════════╣   ╠════════════════════════════╣║
  ║  ║  • int, int8, int16, int32, int64 ║   ║  • slices ([]int, []str)   ║║
  ║  ║  • uint, uint8-64, uintptr        ║   ║  • maps (map[K]V)         ║║
  ║  ║  • float32, float64               ║   ║  • functions               ║║
  ║  ║  • bool                           ║   ║  • structs with            ║║
  ║  ║  • string                        ║   ║    slice/map/func fields   ║║
  ║  ║  • pointers (to comp. types)     ║   ║                            ║║
  ║  ║  • structs (all fields comp.)    ║   ║                            ║║
  ║  ║  • arrays (elems comparable)     ║   ║                            ║║
  ║  ║  • interfaces (dynamic type)     ║   ║                            ║║
  ║  ╚════════════════════════════════════╝   ╚════════════════════════════╝║
  ║                                                                          ║
  ╚══════════════════════════════════════════════════════════════════════════╝
```

**Reading this diagram:**

- **Left column (COMPARABLE):** These types can be used with `==` and `!=`, and can be map keys. This includes all basic types (int, string, float), pointers, and structs where every field is comparable.
- **Right column (NOT COMPARABLE):** These types cannot be compared with `==`. Slices, maps, and functions are inherently non-comparable. Structs become non-comparable if they contain any non-comparable field (like a slice or map).
- **Why this matters:** If you write `func Unique[T comparable](s []T)`, T must be from the left column. Trying to use a slice as T will give a compile error.

### Struct Comparability

A struct is comparable only if **all** its fields are comparable:

```go
// Comparable — all fields are basic types
type User struct {
    ID   int
    Name string
}

// NOT comparable — contains a slice
type Config struct {
    Name  string
    Tags  []string  // slice makes the whole struct non-comparable
}

// Usage with maps
users := map[User]string{} // OK
configs := map[Config]string{} // Compile error if Config has non-comparable fields
```

### Using `comparable` for Map Keys

```go
// Generic frequency counter
func Frequency[T comparable](items []T) map[T]int {
    counts := make(map[T]int)
    for _, item := range items {
        counts[item]++
    }
    return counts
}

_ = Frequency([]int{1, 2, 2, 3})         // map[1:1 2:2 3:1]
_ = Frequency([]string{"a", "b", "a"})   // map[a:2 b:1]
```

### Combining `comparable` with Methods

```go
type Entity interface {
    comparable
    ID() string
}

func FindByIDs[T Entity](items []T, ids []string) []T {
    idSet := make(map[string]struct{})
    for _, id := range ids {
        idSet[id] = struct{}{}
    }

    var result []T
    for _, item := range items {
        if _, ok := idSet[item.ID()]; ok {
            result = append(result, item)
        }
    }
    return result
}
```

---

## 11. Type Inference [CORE]

Go's type inference reduces verbosity by letting the compiler figure out type arguments.

### Basic Inference

```go
func Identity[T any](v T) T { return v }

// Explicit type argument (verbose)
result := Identity[int](42)

// Inferred — compiler knows T = int from the argument
result := Identity(42)
```

### Inference from Function Arguments

```go
func Map[T, U any](s []T, fn func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s {
        result[i] = fn(v)
    }
    return result
}

// T inferred from []int, U inferred from return type of func
result := Map([]int{1, 2, 3}, func(n int) string {
    return strconv.Itoa(n)
}) // result is []string
```

### When Inference Fails

```go
// No arguments to infer from — must specify type argument
var zero int = Zero[int]() // Must be explicit

func Zero[T any]() T {
    var z T
    return z
}
```

### Inference with Composite Types

```go
func Process[K comparable, V any](m map[K]V) []K {
    keys := make([]K, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    return keys
}

// K = string, V = int — inferred from map argument
keys := Process(map[string]int{"a": 1, "b": 2})
```

### Unification-Based Inference

Go uses constraint unification — it matches types across all uses of a type parameter:

```go
func Combine[T any](a, b []T) []T {
    return append(a, b...)
}

// All three uses of T must unify to the same type
result := Combine([]int{1, 2}, []int{3, 4}) // T = int
```

---

## 12. Common Production Patterns [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

### Functional Options Pattern with Generics

```go
type Option[T any] func(*T)

func WithTimeout[T any](d time.Duration) Option[T] {
    return func(cfg *T) {
        // Type-safe but cfg fields depend on concrete T
    }
}

func NewServer(opts ...Option[ServerConfig]) *Server {
    cfg := defaultConfig()
    for _, opt := range opts {
        opt(&cfg)
    }
    return &Server{config: cfg}
}
```

### Generic Result Type

```go
type Result[T any] struct {
    val T
    err error
}

func OK[T any](val T) Result[T] {
    return Result[T]{val: val}
}

func Err[T any](err error) Result[T] {
    return Result[T]{err: err}
}

func (r Result[T]) Value() (T, error) {
    return r.val, r.err
}

func (r Result[T]) Map[U any](fn func(T) U) Result[U] {
    if r.err != nil {
        return Err[U](r.err)
    }
    return OK(fn(r.val))
}
```

### Generic Pool

```go
type Pool[T any] struct {
    mu    sync.Mutex
    items []T
    alloc func() T
    reset func(T) T
}

func NewPool[T any](alloc func() T, reset func(T) T) *Pool[T] {
    return &Pool[T]{alloc: alloc, reset: reset}
}

func (p *Pool[T]) Get() T {
    p.mu.Lock()
    defer p.mu.Unlock()

    if len(p.items) == 0 {
        return p.alloc()
    }
    item := p.items[len(p.items)-1]
    p.items = p.items[:len(p.items)-1]
    return p.reset(item)
}

func (p *Pool[T]) Put(item T) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.items = append(p.items, item)
}
```

### Generic Retry

```go
func Retry[T any](attempts int, delay time.Duration, fn func() (T, error)) (T, error) {
    var lastErr error
    for i := 0; i < attempts; i++ {
        val, err := fn()
        if err == nil {
            return val, nil
        }
        lastErr = err
        if i < attempts-1 {
            time.Sleep(delay)
        }
    }
    var zero T
    return zero, fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}
```

---

## 13. Generics vs Interfaces Performance [INTERNALS]

> ⏭️ **First pass? Skip this section.** This covers compiler internals. Come back after completing Topics 11-19.

### The Tradeoff

```
  ╔═══════════════════════════════════════════════════════════════════════════════════╗
  ║               INTERFACE (Dynamic Dispatch)     vs    GENERICS (Static Dispatch) ║
  ╠═══════════════════════════════════════════════════════════════════════════════════╣
  ║                                                                                   ║
  ║      ┌──────────────────┐                    ┌──────────────────┐                ║
  ║      │      CALLER       │                    │      CALLER       │                ║
  ║      └────────┬─────────┘                    └────────┬─────────┘                ║
  ║               │                                       │                          ║
  ║               ▼                                       ▼                          ║
  ║      ┌──────────────────┐                    ┌──────────────────┐                ║
  ║      │   interface{}    │                    │   concrete T     │                ║
  ║      │    (fat ptr)     │                    │    (direct)      │                ║
  ║      │  type + value    │                    │                  │                ║
  ║      └────────┬─────────┘                    └────────┬─────────┘                ║
  ║               │                                       │                          ║
  ║               ▼                                       ▼                          ║
  ║      ┌──────────────────┐                    ┌──────────────────┐                ║
  ║      │  vtable lookup   │                    │  inlined direct  │                ║
  ║      │ type assertion   │                    │     call         │                ║
  ║      │ method dispatch  │                    │                  │                ║
  ║      └────────┬─────────┘                    └────────┬─────────┘                ║
  ║               │                                       │                          ║
  ║      ┌────────▼─────────┐                    ┌────────▼─────────┐                ║
  ║      │ Runtime overhead │                    │  Zero overhead  │                ║
  ║      │ (indirection)    │                    │ (same as typed) │                ║
  ║      └──────────────────┘                    └──────────────────┘                ║
  ║                                                                                   ║
  ╚═══════════════════════════════════════════════════════════════════════════════════╝
```

**Reading this diagram:**

- **Left side (Interface):** Uses dynamic dispatch. At runtime, the interface holds both the type and value (fat pointer). When you call a method, it looks up the method in a vtable and dispatches. This adds overhead — there's an indirection for every call.
- **Right side (Generics):** Uses static dispatch. The compiler generates specialized code for each type. There's no runtime lookup — it's a direct function call, often inlined. Zero overhead compared to writing the function manually for each type.
- **Tradeoff:** Interfaces are flexible (any type can implement them) but slower. Generics are fast but less flexible (must know types at compile time).

| Scenario | Use Interfaces | Use Generics |
|----------|---------------|-------------|
| Known set of implementations | ✓ | |
| Type-safe containers | | ✓ |
| Plugin architecture | ✓ | |
| Algorithms (sort, search) | | ✓ |
| Method polymorphism | ✓ | |
| Performance-critical loops | | ✓ |
| Dependency injection | ✓ | |
| Data structure internals | | ✓ |

---

## 14. Internals: How Go Implements Generics [INTERNALS]

> ⏭️ **First pass? Skip this section.** This covers compiler internals. Come back after completing Topics 11-19.

### GC Shape Stenciling (Go 1.18+)

Go uses **GC shape stenciling** — a hybrid between full monomorphization and dynamic dispatch.

```
  ╔═══════════════════════════════════════════════════════════════════════════════╗
  ║                 PURE MONOMORPHISM (C++)      vs    GC SHAPE STENCILING (Go)   ║
  ╠═══════════════════════════════════════════════════════════════════════════════╣
  ║                                                                           ║
  ║  ┌──────────────────────────────────────┐   ┌──────────────────────────────────┐ ║
  ║  │  Generated Code (one copy per type)  │   │  Generated Code (shared shapes)  │ ║
  ║  ├──────────────────────────────────────┤   ├──────────────────────────────────┤ ║
  ║  │                                      │   │                                  │ ║
  ║  │  func Sum_int(nums []int) int       │   │  func Sum·int·(nums []int) int  │ ║
  ║  │      // full code for int           │   │      // shared code, + dict      │ ║
  ║  │                                      │   │                                  │ ║
  ║  │  func Sum_float64(nums []float64)   │   │  func Sum·float64·(...)         │ ║
  ║  │      // full code for float64      │   │      // same code, diff dict     │ ║
  ║  │                                      │   │                                  │ ║
  ║  │  func Sum_string(nums []string)     │   │  func Sum·string·(...)          │ ║
  ║  │      // full code for string       │   │      // same code, diff dict     │ ║
  ║  │                                      │   │                                  │ ║
  ║  └──────────────────────────────────────┘   └──────────────────────────────────┘ ║
  ║                                                                           ║
  ║  Binary size: LARGE (N copies)          Binary size: SMALL (1 per shape)    ║
  ║                                                                           ║
  ╚═══════════════════════════════════════════════════════════════════════════════╝
```

**Reading this diagram:**

- **Left (Pure Monomorphization - C++ style):** For each type you use, the compiler generates a completely separate copy of the function. `Sum_int()`, `Sum_float64()`, `Sum_string()` — each has full code. Result: large binary, but fastest possible.
- **Right (GC Shape Stenciling - Go style):** Go groups types by their "shape" (memory layout). Types with the same shape share one code path. The compiler passes a small "dictionary" telling the code how to handle the specific type. Result: smaller binary, slightly more runtime work but often optimized away.
- **Shape examples:** All pointers have the same shape. All ints have the same shape. Strings all have the same shape (ptr + len).

At runtime, Go passes a "dictionary" of type-specific operations:

```
  ╔═══════════════════════════════════════════════════════════════════════════════╗
  ║                         DICTIONARY PASSING AT RUNTIME                        ║
  ╠═══════════════════════════════════════════════════════════════════════════════╣
  ║                                                                           ║
  ║  Your Code:                          Compiler Generates:                   ║
  ║  ┌────────────────────────────────┐   ┌──────────────────────────────────┐    ║
  ║  │ func Sum[T Numeric](nums []T) │   │ func Sum_int(nums []int) int  │    ║
  ║  │     total += n                 │   │     total += n   ← direct    │    ║
  ║  │     return total               │   │     return total             │    ║
  ║  │ }                              │   │ }                             │    ║
  ║  └────────────────────────────────┘   └──────────────────────────────────┘    ║
  ║                                                                           ║
  ║  At call site, Go passes a "dictionary" containing:                        ║
  ║  ┌─────────────────────────────────────────────────────────────────────────┐ ║
  ║  │ • How to add two T values (int add, float add, etc.)                  │ ║
  ║  │ • How to compare T values (<, >, ==)                                  │ ║
  ║  │ • The size of T (for slice indexing)                                  │ ║
  ║  └─────────────────────────────────────────────────────────────────────────┘ ║
  ║                                                                           ║
  ║  For hot paths: compiler inlines and eliminates dictionary overhead        ║
  ║                                                                           ║
  ╚═══════════════════════════════════════════════════════════════════════════════╝
```

**Reading this diagram:**

- **Your Code (top left):** You write one generic function `Sum[T Numeric]`. You don't know what T will be — int, float64, or something else.
- **Compiler Generates (top right):** The compiler creates a specialized version for each type you actually use. For `Sum([]int{...})`, it creates `Sum_int()`.
- **Dictionary (middle):** The dictionary is a small hidden parameter passed at runtime. It contains pointers to type-specific operations: how to add, how to compare, the size, etc.
- **Hot vs Cold (bottom):** If a generic function is called in a tight loop (hot path), the compiler inlines it and eliminates dictionary lookups entirely. For rarely-called code (cold path), the dictionary adds tiny overhead but saves binary size.

- **Hot path**: Compiler fully specializes → zero overhead
- **Cold path**: Shared code with dictionary → slight overhead
- **Result**: Generics are as fast as hand-written code for most workloads

---

## 15. Testing Generic Code [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

### Table-Driven Tests for Generic Functions

```go
func TestUnique[T comparable](t *testing.T) {
    tests := []struct {
        name     string
        input    []T
        expected []T
    }{
        {"empty", []T{}, []T{}},
        {"no duplicates", []T{T(1), T(2)}, []T{T(1), T(2)}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Unique(tt.input)
            if !reflect.DeepEqual(got, tt.expected) {
                t.Errorf("got %v, want %v", got, tt.expected)
            }
        })
    }
}

func TestUniqueInt(t *testing.T)  { TestUnique[int](t) }
func TestUniqueString(t *testing.T) { TestUnique[string](t) }
```

### Testing Constraint Satisfaction at Compile Time

```go
// Verify at compile time that types satisfy your constraint
var _ Numeric = int(0)
var _ Numeric = float64(0)
// var _ Numeric = "string" // Compile error — string is not Numeric
```

---

## 16. Common Pitfalls [CORE]

### Pitfall 1: Over-Using `any`

```go
// BAD — no type safety
func Process[T any](items []T) []T { return items }

// GOOD — constrain to what you actually need
func Process[T cmp.Ordered](items []T) []T { /* can compare */ return items }
```

### Pitfall 2: Not Understanding `~`

```go
type MyInt int

func Process1[T int](v T) T { return v }    // MyInt fails
func Process2[T ~int](v T) T { return v }    // MyInt works

_ = Process1(MyInt(42)) // ERROR
_ = Process2(MyInt(42)) // OK
```

### Pitfall 3: No Constraint on Operators

```go
// ERROR: can't use + on type T
func Add[T any](a, b T) T { return a + b }

// FIX: constrain T to types that support +
func Add[T int | float64 | string](a, b T) T { return a + b }
```

### Pitfall 4: Generic Methods in Interfaces (Not Allowed)

```go
// This DOESN'T work:
type Container interface {
    Transform[T any](fn func(T) T) T // ERROR: methods can't have type params
}

// Workaround: make the interface itself generic
type Container[T any] interface {
    Transform(fn func(T) T) T
}
```

### Pitfall 5: Nil Pointer with Generic Types

```go
func First[T any](s []T) T {
    if len(s) == 0 {
        var zero T
        return zero // Returns nil for pointer types!
    }
    return s[0]
}

var users []*User
u := First(users) // u is nil, not an empty User!
```

---

## 17. Production Guidelines [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

### 1. Prefer Generics for Data Structures

```go
// Good: type-safe data structure
type Queue[T any] struct { /* ... */ }
type Stack[T any] struct { /* ... */ }
type Cache[K comparable, V any] struct { /* ... */ }
```

### 2. Constrain T as Tightly as Possible

```go
// BAD: too permissive
func Max[T any](a, b T) T { /* can't compare */ }

// GOOD: tight constraint
func Max[T cmp.Ordered](a, b T) T {
    if a > b { return a }
    return b
}
```

### 3. Use Generics for Algorithms, Interfaces for Behavior

```go
// Generics for algorithms
func Filter[T any](items []T, pred func(T) bool) []T { /* ... */ }
func Map[T, U any](items []T, fn func(T) U) []U { /* ... */ }

// Interfaces for behavior
type Storage interface {
    Save(key string, data []byte) error
    Load(key string) ([]byte, error)
}
```

### 4. Document Your Constraints

```go
// Numeric accepts all signed/unsigned ints and floats.
// Custom types with underlying numeric types are also supported via ~.
type Numeric interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
    ~float32 | ~float64
}
```

### 5. Test with Multiple Types

```go
// Test with at least:
// - A basic type (int, string)
// - A pointer type (*User)
// - A custom type (type MyInt int)
```

### 6. Don't Generic-ify Everything

```go
// BAD: generic for no reason
func Print[T any](val T) { fmt.Println(val) }

// GOOD: just use any
func Print(val any) { fmt.Println(val) }

// Generics pay off when:
// 1. You need type safety across multiple types
// 2. You need to preserve type information through operations
// 3. You're building reusable data structures or algorithms
```

### Summary Decision Tree

```
  ╔═══════════════════════════════════════════════════════════════════════════════╗
  ║                      DECISION TREE: GENERICS vs INTERFACES                   ║
  ╚═══════════════════════════════════════════════════════════════════════════════╝

      Need to write code that works with multiple types?
                          │
                          ▼
              ┌───────────────────────────┐
              │  Are you defining        │
              │  BEHAVIOR (contracts)?   │
              └────────────┬────────────┘
                           │
              ┌────────────┴────────────┐
              │                         │
             Yes                        No
              │                         │
              ▼                         ▼
      ┌───────────────┐        ┌─────────────────────────┐
      │   USE         │        │  Are you building a   │
      │  INTERFACES   │        │  DATA STRUCTURE?       │
      └───────────────┘        └───────────┬─────────────┘
                                           │
                              ┌────────────┴────────────┐
                              │                         │
                             Yes                        No
                              │                         │
                              ▼                         ▼
                      ┌───────────────┐      ┌─────────────────────────┐
                      │    USE        │      │  Need TYPE SAFETY?     │
                      │  GENERICS     │      │  (compile-time checks) │
                      │  (type-safe)  │      └───────────┬─────────────┘
                      └───────────────┘                  │
                                           ┌────────────┴────────────┐
                                           │                         │
                                          Yes                        No
                                           │                         │
                                           ▼                         ▼
                                   ┌───────────────┐     ┌───────────────────┐
                                   │    USE        │     │   USE INTERFACES │
                                   │  GENERICS     │     │   (flexibility)  │
                                   │  (statically  │     │   or plain any   │
                                   │   type-safe)  │     └───────────────────┘
                                   └───────────────┘

  ╔═══════════════════════════════════════════════════════════════════════════════╗
  ║  EXAMPLES:                                                                    ║
  ║  ┌─────────────────────────────────────────────────────────────────────────┐ ║
  ║  │ GENERICS:         │  INTERFACES:                                       │ ║
  ║  │  • type Stack[T]  │  • type Writer interface { Write([]byte) }        │ ║
  ║  │  • type Cache[K,V]│  • type Repository interface { Save(), Get() }    │ ║
  ║  │  • func Map[T,U]  │  • Plugin systems, dependency injection            │ ║
  ║  │  • func Filter[T] │                                                   │ ║
  ║  └─────────────────────────────────────────────────────────────────────────┘ ║
   ╚═══════════════════════════════════════════════════════════════════════════════╝
```

**Reading this diagram:**

- **Start at the top:** Ask yourself — what am I building?
- **Defining BEHAVIOR (interfaces):** If you're defining contracts that different implementations must satisfy, use interfaces. Examples: `Writer`, `Reader`, `Repository` — these define what operations exist, not how data is stored.
- **DATA STRUCTURE (generics):** If you're building containers that hold values, use generics. Examples: `Stack[T]`, `Cache[K,V]`, `Queue[T]` — type-safe containers.
- **Need TYPE SAFETY (generics):** If you want compile-time guarantees that wrong types won't compile, use generics. Examples: `Map[T,U]`, `Filter[T]` — algorithms that preserve type information.
- **Don't need type safety (interfaces/any):** If flexibility is more important than type safety, use interfaces or plain `any`. Examples: Logging, serialization, plugin systems.

---

## Exercises

### Exercise 1: Generic Map Function ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Write a generic `Map[T, U any](s []T, f func(T) U) []U` function that applies `f` to every element of `s` and returns a new slice. Test it by mapping `[]int{1, 2, 3}` to `[]string{"1", "2", "3"}` using `strconv.Itoa`.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"strconv"
)

func Map[T, U any](s []T, f func(T) U) []U {
	result := make([]U, len(s))
	for i, v := range s {
		result[i] = f(v)
	}
	return result
}

func main() {
	nums := []int{1, 2, 3}
	strs := Map(nums, func(n int) string {
		return strconv.Itoa(n)
	})
	fmt.Println(strs) // [1 2 3]
}
```

</details>

### Exercise 2: Generic Filter Function ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Write a generic `Filter[T any](s []T, pred func(T) bool) []T` function that returns a new slice containing only elements for which `pred` returns `true`. Test it by filtering even numbers from `[]int{1, 2, 3, 4, 5, 6}`.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func Filter[T any](s []T, pred func(T) bool) []T {
	var result []T
	for _, v := range s {
		if pred(v) {
			result = append(result, v)
		}
	}
	return result
}

func main() {
	nums := []int{1, 2, 3, 4, 5, 6}
	evens := Filter(nums, func(n int) bool {
		return n%2 == 0
	})
	fmt.Println(evens) // [2 4 6]
}
```

</details>

### Exercise 3: Generic Stack ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a generic `Stack[T any]` struct backed by a slice. Implement `Push(v T)`, `Pop() (T, bool)`, `Peek() (T, bool)`, and `Len() int` methods. `Pop` and `Peek` should return the zero value and `false` when the stack is empty. Test it with `Stack[string]`.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

type Stack[T any] struct {
	items []T
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

func (s *Stack[T]) Len() int {
	return len(s.items)
}

func main() {
	var st Stack[string]
	st.Push("hello")
	st.Push("world")
	fmt.Println(st.Len()) // 2

	if v, ok := st.Peek(); ok {
		fmt.Println("Peek:", v) // world
	}
	if v, ok := st.Pop(); ok {
		fmt.Println("Pop:", v) // world
	}
	fmt.Println(st.Len()) // 1
}
```

</details>

### Exercise 4: Ordered Constraint and Min Function ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Define an `Ordered` constraint interface using `~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string`. Write a `Min[T Ordered](a, b T) T` function that returns the smaller value. Test it with `int`, `float64`, and `string`.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 |
		~string
}

func Min[T Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func main() {
	fmt.Println(Min(3, 7))         // 3
	fmt.Println(Min(2.5, 1.8))     // 1.8
	fmt.Println(Min("apple", "banana")) // apple
}
```

</details>