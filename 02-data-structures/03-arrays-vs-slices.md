# 3. Arrays vs Slices — Complete Deep Dive

> **Goal:** Understand arrays as the foundation, slices as the fat pointer on top, and every production gotcha that exists.

> **Terminology:**
> - **Backing array** — the underlying fixed-size array that a slice points to. When you `append` beyond capacity, Go allocates a new, larger backing array and copies data into it.
> - **Fat pointer** — a pointer that carries extra metadata. A Go slice is a "fat pointer" (24 bytes on 64-bit systems): it's a pointer to the backing array *plus* a length and a capacity. A regular pointer is just 8 bytes.

![Arrays and Slices](../assets/03.png)

---

## Table of Contents

1. [Arrays](#1-arrays)
2. [Slices — The Three Fields](#2-slices--the-three-fields)
3. [Slice Creation Methods](#3-slice-creation-methods)
4. [Slicing Operations](#4-slicing-operations)
5. [The `append` Function](#5-the-append-function)
6. [Capacity Growth Strategy](#6-capacity-growth-strategy)
7. [The Shared-Array Bug](#7-the-shared-array-bug)
8. [Copy](#8-copy)
9. [Nil Slice vs Empty Slice](#9-nil-slice-vs-empty-slice)
10. [Slice Internals (Memory Layout)](#10-slice-internals-memory-layout)
11. [Passing Slices to Functions](#11-passing-slices-to-functions)
12. [Common Patterns](#12-common-patterns)
13. [Performance Considerations](#13-performance-considerations)
14. [Common Pitfalls](#14-common-pitfalls)

---

## 1. Arrays [CORE]

### Declaration

```go
// Fixed size — part of the type
var a [5]int                    // [0, 0, 0, 0, 0]
var b [3]string                 // ["", "", ""]
var c [2]bool                   // [false, false]

// With initialization
d := [5]int{1, 2, 3, 4, 5}
e := [5]int{1, 2}              // [1, 2, 0, 0, 0] — rest are zero
f := [...]int{1, 2, 3}         // [3]int — compiler counts

// Named indices
g := [5]int{0: 10, 2: 30, 4: 50}  // [10, 0, 30, 0, 50]
```

### Key Properties

1. **Size is part of the type:** `[5]int` and `[3]int` are different types
2. **Value type:** Assigning or passing copies the entire array
3. **Comparable:** Arrays of same type can be compared with `==`

```go
a := [3]int{1, 2, 3}
b := [3]int{1, 2, 3}
c := [4]int{1, 2, 3, 4}

fmt.Println(a == b)  // true — same type, same values
// fmt.Println(a == c)  // COMPILE ERROR: mismatched types [3]int and [4]int
```

### Array as Value Type (Copy Semantics)

```go
func modifyArray(a [5]int) {
    a[0] = 999  // Modifies the COPY
}

func main() {
    arr := [5]int{1, 2, 3, 4, 5}
    modifyArray(arr)
    fmt.Println(arr)  // [1, 2, 3, 4, 5] — unchanged!
}
```

### When to Use Arrays

**Almost never.** Use arrays when:
- You need a fixed-size buffer (e.g., `[64]byte` for a hash)
- You need value semantics (comparison, stack allocation)
- Interfacing with code that requires `[N]T`

**In production Go, 99% of the time you use slices.**

### Why Slices Exist

Arrays have two limitations that make them impractical for most code:

1. **Fixed size** — `[5]int` and `[6]int` are different types. You can't write a function that accepts "any int array."
2. **Copy semantics** — Passing an array to a function copies the entire array. For a 1MB array, that's a 1MB copy every call.

Slices solve both problems. A slice is a small (24-byte) header that *points to* a backing array. Passing a slice to a function copies only the header — the underlying data is shared. And slices can grow dynamically via `append`. This is why Go programs almost always use slices.

### Arrays in Practice

```go
// MD5 hash — fixed 16 bytes
type MD5 [16]byte

// IPv4 address
type IPv4 [4]byte

// Stack-allocated ring buffer
var ring [256]byte
```

---

## 2. Slices — The Three Fields [CORE]

A slice is a **descriptor** (fat pointer) with three fields:

```
  ┌─────────────────────────────────────────────────┐
  │              SLICE HEADER (24 bytes)            │
  ├───────────────────┬──────────────┬──────────────┤
  │  Pointer (8 bytes)│ Len (8 bytes)│ Cap (8 bytes)│
  │  unsafe.Pointer   │     int      │     int      │
  └─────────┬─────────┴──────┬──────┴──────┬───────┘
            │                │             │
            │                ▼             ▼
            │           len(s) = 3    cap(s) = 10
            │
            ▼
   ┌────┬────┬────┬────┬────┬────┬────┬────┬────┬────┐
   │ 0  │ 0  │ 0  │    │    │    │    │    │    │    │
   └────┴────┴────┴────┴────┴────┴────┴────┴────┴────┘
    ◄──────── accessible ────────►◄───── reserved ─────►
```

### The Three Fields Explained

```go
s := make([]int, 3, 10)
//        ^type  ^len  ^cap

fmt.Println(len(s))  // 3 — number of accessible elements
fmt.Println(cap(s))  // 10 — size of underlying array from start of slice
fmt.Println(s)       // [0, 0, 0] — the 3 accessible elements
```

```
Underlying array:  [0] [0] [0] [ ] [ ] [ ] [ ] [ ] [ ] [ ]
                    ↑                   ↑                   ↑
                 s[0]                s[3]              cap=10
                 len=3              (not accessible)
```

### Reading len and cap

```go
s := []int{1, 2, 3}
fmt.Println(len(s))  // 3
fmt.Println(cap(s))  // 3 (initial capacity equals length for literals)

s = append(s, 4)
fmt.Println(len(s))  // 4
fmt.Println(cap(s))  // 6 (or larger — grew automatically)
```

---

## 3. Slice Creation Methods [CORE]

### Method 1: Literal

```go
s := []int{1, 2, 3, 4, 5}
// len=5, cap=5
```

### Method 2: `make()`

```go
s := make([]int, 5)
// len=5, cap=5, values: [0, 0, 0, 0, 0]

s := make([]int, 0, 10)
// len=0, cap=10, values: []

s := make([]int, 5, 10)
// len=5, cap=10, values: [0, 0, 0, 0, 0]
```

### Method 3: Nil Slice

```go
var s []int
// len=0, cap=0, s == nil
```

### Method 4: From Array

```go
arr := [5]int{1, 2, 3, 4, 5}
s := arr[1:4]  // [2, 3, 4], len=3, cap=4
```

### Method 5: Slicing a Slice

```go
s := []int{1, 2, 3, 4, 5}
sub := s[1:3]  // [2, 3], len=2, cap=4
```

### Pre-allocation — Why It Matters

```go
// BAD — repeated allocations
var result []int
for i := 0; i < 1000000; i++ {
    result = append(result, i)  // Grows many times
}

// GOOD — single allocation
result := make([]int, 0, 1000000)
for i := 0; i < 1000000; i++ {
    result = append(result, i)  // Never grows
}
```

---

## 4. Slicing Operations [CORE]

**Before seeing the syntax, understand the core concept:** Slicing does NOT copy data. It creates a new slice header pointing into the same backing array. This is efficient (no allocation) but dangerous — modifying a sub-slice can silently modify the original. This is the #1 source of slice bugs in production Go.

### Basic Slicing

```go
s := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

// s[low:high] — elements from low (inclusive) to high (exclusive)
a := s[2:5]    // [2, 3, 4]
b := s[:3]     // [0, 1, 2]
c := s[7:]     // [7, 8, 9]
d := s[:]      // [0, 1, 2, 3, 4, 5, 6, 7, 8, 9] — full slice
```

### Full Slicing Expression (Go 1.2+)

```go
s := []int{0, 1, 2, 3, 4, 5}

// s[low:high:max]
// max limits the capacity of the resulting slice
a := s[2:4:5]
// a = [2, 3]
// len(a) = 2 (4-2)
// cap(a) = 3 (5-2)
```

This is **critical for preventing the shared-array bug** — more on this below.

### Memory Layout of Slicing

```go
s := []int{0, 1, 2, 3, 4, 5}
sub := s[2:4]

// s:   pointer ──→ [0] [1] [2] [3] [4] [5]
//                   len=6, cap=6

// sub: pointer ──→ [0] [1] [2] [3] [4] [5]
//                          ↑
//                       sub[0] = 2
//                   len=2, cap=4 (6-2)
```

**Slicing does NOT copy data.** Both slices point to the same underlying array.

---

## 5. The `append` Function [CORE]

### Basic Usage

```go
s := []int{1, 2, 3}
s = append(s, 4)        // [1, 2, 3, 4]
s = append(s, 5, 6, 7)  // [1, 2, 3, 4, 5, 6, 7]

// Append another slice
t := []int{8, 9}
s = append(s, t...)     // [1, 2, 3, 4, 5, 6, 7, 8, 9]
//                    ^ spread operator
```

### What Happens Inside `append`

```go
s := make([]int, 3, 5)  // [0, 0, 0], len=3, cap=5

// Append within capacity — no reallocation
s = append(s, 1)  // len=4, cap=5 — reuses same array

// Append beyond capacity — reallocation
s = append(s, 2, 3, 4)  // len=7 — new array allocated, data copied
```

### The Return Requirement

```go
// WRONG
func addElement(s []int, v int) {
    s = append(s, v)  // Modifies local copy of slice header
}

// RIGHT
func addElement(s []int, v int) []int {
    return append(s, v)
}

// ALSO RIGHT (if you want to modify via pointer)
func addElement(s *[]int, v int) {
    *s = append(*s, v)
}
```

**Why?** `append` may return a slice pointing to a **different** underlying array. You must capture the return value.

---

## 6. Capacity Growth Strategy [CORE]

### Growth Algorithm (Go 1.18+)

```
Current cap < 256:     new cap = 2 * old cap
Current cap >= 256:    new cap = old cap + (old cap + 3*256) / 4
                       ≈ 1.25x for large slices
```

### Visual Example

```go
s := []int{}
for i := 0; i < 20; i++ {
    s = append(s, i)
    fmt.Printf("len=%d, cap=%d\n", len(s), cap(s))
}
```

Output:
```
len=1,  cap=1
len=2,  cap=2
len=3,  cap=4
len=4,  cap=4
len=5,  cap=8
len=6,  cap=8
len=7,  cap=8
len=8,  cap=8
len=9,  cap=16
len=10, cap=16
...
len=17, cap=32
```

### Why This Matters

```go
// If you know the final size, pre-allocate
result := make([]Type, 0, expectedSize)
```

### Benchmark Proof

```go
func BenchmarkAppendNoPrealloc(b *testing.B) {
    for i := 0; i < b.N; i++ {
        var s []int
        for j := 0; j < 10000; j++ {
            s = append(s, j)
        }
    }
}

func BenchmarkAppendPrealloc(b *testing.B) {
    for i := 0; i < b.N; i++ {
        s := make([]int, 0, 10000)
        for j := 0; j < 10000; j++ {
            s = append(s, j)
        }
    }
}
```

The pre-allocated version is typically 2-10x faster.

---

## 7. The Shared-Array Bug [CORE]

This is the **#1 slice bug** in production Go code.

### The Bug

```go
func getUsers() []User {
    allUsers := []User{alice, bob, charlie, dave, eve}
    return allUsers[:3]  // Return first 3
}

func main() {
    users := getUsers()  // [alice, bob, charlie]
    
    // Later, someone appends
    users = append(users, frank)
    
    // Original allUsers is MODIFIED because they share the same underlying array!
    // allUsers = [alice, bob, charlie, frank, eve]  ← frank overwrote dave!
}
```

### Why It Happens

```
allUsers ──→ [alice] [bob] [charlie] [dave] [eve]
              len=5, cap=5

users ─────→ [alice] [bob] [charlie] [dave] [eve]
              len=3, cap=5 (still has capacity!)

append(users, frank) — cap is 5, len is 3, fits!
Result: [alice] [bob] [charlie] [frank] [eve]
         ↑ dave was overwritten!
```

### The Fix: Full Slice Expression

```go
func getUsers() []User {
    allUsers := []User{alice, bob, charlie, dave, eve}
    return allUsers[:3:3]  // len=3, cap=3 — NO room to append
}
```

Now:
```
users ──→ [alice] [bob] [charlie]  ← same memory
           len=3, cap=3

append(users, frank) — cap is 3, len is 3, must reallocate!
New array allocated: [alice] [bob] [charlie] [frank]
allUsers remains:    [alice] [bob] [charlie] [dave] [eve]
```

### Production Rule

**When returning a sub-slice from a function, ALWAYS use the full slice expression `s[low:high:max]` to cap capacity.**

```go
// WRONG — can corrupt original
func firstN(s []int, n int) []int {
    return s[:n]
}

// RIGHT — capped capacity
func firstN(s []int, n int) []int {
    result := make([]int, n)
    copy(result, s[:n])
    return result
}

// ALSO RIGHT — if you want zero-copy and accept capacity sharing
func firstN(s []int, n int) []int {
    return s[:n:n]  // Cap = len, append will reallocate
}
```

---

## 8. Copy [CORE]

### Basic Copy

```go
src := []int{1, 2, 3, 4, 5}
dst := make([]int, 3)      // Destination must be pre-allocated

n := copy(dst, src)
// dst = [1, 2, 3]
// n = 3 (number of elements copied — min of len(dst), len(src))
```

### Copy Creates Independent Data

```go
src := []int{1, 2, 3}
dst := make([]int, len(src))
copy(dst, src)

dst[0] = 999
fmt.Println(src)  // [1, 2, 3] — unchanged
fmt.Println(dst)  // [999, 2, 3]
```

### Copy Patterns

> The `[T any]` syntax below is Go generics — `T` is a type parameter meaning "works with any type." Full explanation in [Topic 10: Generics](../05-generics/10-generics.md).

```go
// Clone a slice
func clone[T any](s []T) []T {
    result := make([]T, len(s))
    copy(result, s)
    return result
}

// Remove element at index i
func removeAt[T any](s []T, i int) []T {
    copy(s[i:], s[i+1:])
    return s[:len(s)-1]
}

// Insert element at index i
func insertAt[T any](s []T, i int, v T) []T {
    s = append(s, v)            // Grow by 1
    copy(s[i+1:], s[i:])       // Shift right
    s[i] = v                    // Insert
    return s
}
```

---

## 9. Nil Slice vs Empty Slice [CORE]

For most operations, nil and empty slices behave identically — `len()`, `cap()`, `range`, and `append` all work the same. The distinction matters in exactly one place: **serialization**. If your API returns `null` when you meant `[]`, clients may break. Understanding this difference prevents subtle API bugs.

```go
var nilSlice []int         // nil — len=0, cap=0
emptySlice := []int{}      // not nil — len=0, cap=0
emptySlice2 := make([]int, 0)  // not nil — len=0, cap=0
```

### Comparison

```go
fmt.Println(nilSlice == nil)    // true
fmt.Println(emptySlice == nil)  // false
```

### JSON Serialization

> **JSON in Go:** `encoding/json` converts Go values to/from JSON. `json.Marshal(val)` returns `([]byte, error)` — the JSON representation. Struct tags like `` `json:"items"` `` control the output field name. A nil slice serializes as `null`, an empty slice as `[]`.

```go
type Response struct {
    Items []string `json:"items"`
}

r1 := Response{Items: nil}     // JSON: {"items":null}
r2 := Response{Items: []string{}}  // JSON: {"items":[]}
```

**Production rule:** If your API should return `[]` not `null`, initialize with empty slice:

```go
func getItems() []Item {
    items := []Item{}  // Not nil
    // ... fill items ...
    return items
}
```

### When Does It Matter?

| Operation | nil slice | empty slice |
|-----------|-----------|-------------|
| `len(s)` | 0 | 0 |
| `cap(s)` | 0 | 0 |
| `range s` | no iterations | no iterations |
| `append(s, v)` | works | works |
| `s == nil` | true | false |
| `json.Marshal` | `"null"` | `"[]"` |
| `bytes.Buffer.Write(s)` | 0 bytes | 0 bytes |

**Rule of thumb:** Treat nil and empty slices the same in your logic. Only care about the difference at API boundaries (JSON).

---

## 10. Slice Internals (Memory Layout) [INTERNALS]

> ⏭️ **First pass? Skip this section.** This covers low-level internals. Come back after completing Topics 1-10.

### Slice Structure

A slice is a **struct** containing three fields:

```go
type sliceHeader struct {
    ptr   uintptr  // pointer to underlying array
    len   int      // length of slice
    cap   int      // capacity (allocated size)
}
```

### Visual: Slice Header + Underlying Array

```
  ┌──────────────────────────────────────────────────────────────────────┐
  │                       SLICE STRUCTURE                                 │
  ├──────────────────────────────────────────────────────────────────────┤
  │                                                                       │
  │   sliceHeader (24 bytes on 64-bit)                                   │
  │   ┌──────────────┬──────────┬──────────┐                             │
  │   │  ptr  (8B)   │ len (8B) │ cap (8B) │                             │
  │   └──────┬───────┴──────────┴──────────┘                             │
  │          │                                                            │
  └──────────┼───────────────────────────────────────────────────────────┘
             │
             ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │                  UNDERLYING ARRAY (on Heap)                          │
  │                                                                       │
  │   Index:  [0]    [1]    [2]    [3]    [4]    [5]    [6]    [7]  [8]  │
  │          ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┬──────┬───┐
  │   Data:  │  10  │  20  │  30  │  40  │  50  │  60  │  70  │  80 │ 90│
  │          └──┬───┴──────┴──────┴──────┴──────┴──────┴──────┴──────┴───┘
  │             │                                                         │
  │        ptr ─┘                                                         │
  │                                                                       │
  │   len = 5  →  accessible elements: [0]..[4]                          │
  │   cap = 9  →  allocated space:      [0]..[8]                          │
  │                                                                       │
  └──────────────────────────────────────────────────────────────────────┘
```

### Visual: How Slices Point to Arrays

```go
s := []int{10, 20, 30, 40, 50}
```

```
  SLICE s                           UNDERLYING ARRAY
  ┌──────────────────────┐         ┌──────┬──────┬──────┬──────┬──────┐
  │ ptr ──────────────────┼────────►│  10  │  20  │  30  │  40  │  50  │
  │ len = 5              │         ├──────┼──────┼──────┼──────┼──────┤
  │ cap = 5              │         │ [0]  │ [1]  │ [2]  │ [3]  │ [4]  │
  └──────────────────────┘         └──────┴──────┴──────┴──────┴──────┘
```

### Visual: Sub-slicing Shares the Array

```go
s := []int{10, 20, 30, 40, 50, 60, 70}
s1 := s[1:4]  // [20, 30, 40]
s2 := s[2:6]  // [30, 40, 50, 60]
```

```
  ORIGINAL SLICE s
  ┌──────────────────────┐
  │ ptr ───────────┬─────┤
  │ len = 7        │     │
  │ cap = 7        │     │
  └────────────────┘     │
         │               │
         ▼               │
  ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┐
  │  10  │  20  │  30  │  40  │  50  │  60  │  70  │
  ├──────┴──────┴──────┴──────┴──────┴──────┴──────┤
  │ [0]    [1]    [2]    [3]    [4]    [5]    [6]  │
  └────────────────────────────────────────────────┘
       ▲           ▲                     ▲
       │           │                     │
       │           │                     └── s2 ends here
       │           └── s1 ends here
       └── s1 starts here


  s1 = s[1:4]         points to indices [1], [2], [3]
  ┌──────────────────────┐
  │ ptr ──────────┬──────┤
  │ len = 3       │      │
  │ cap = 6       │      │  ← cap from original slice
  └──────────────────────┘

  s2 = s[2:6]         points to indices [2], [3], [4], [5]
  ┌──────────────────────┐
  │ ptr ────────────┬────┤
  │ len = 4         │    │
  │ cap = 5         │    │  ← cap from original slice
  └──────────────────────┘
```

### Visual: append() When Capacity is Sufficient

```go
s := []int{10, 20, 30}  // len=3, cap=3
s = append(s, 40)
```

```
  BEFORE append (len=3, cap=3):
  ┌──────────────────┐          ┌───────────────────────┐
  │  s               │          │   Underlying Array     │
  │  ptr ────────────┼─────────►│ [0]    [1]    [2]     │
  │  len = 3         │          │  10     20     30     │
  │  cap = 3         │          └───────────────────────┘
  └──────────────────┘

  AFTER append(s, 40)  →  space available, no reallocation:
  ┌──────────────────┐          ┌───────────────────────┐
  │  s               │          │   Underlying Array     │
  │  ptr ────────────┼─────────►│ [0]    [1]    [2]    [3]    │
  │  len = 4         │          │  10     20     30    [40]   │
  │  cap = 4         │          └──────────────────────────┘
  └──────────────────┘                   ▲
                                     written here
                                   (no new allocation)
```

### Visual: append() When Capacity is FULL (Reallocation)

```go
s := []int{10, 20, 30}  // len=3, cap=3
s = append(s, 40)       // exceeds capacity, triggers reallocation
```

```
  BEFORE (capacity FULL, len=3, cap=3):
  ┌──────────────────┐          ┌───────────────────────┐
  │  s               │          │    OLD ARRAY           │
  │  ptr ────────────┼─────────►│ [0]    [1]    [2]     │
  │  len = 3         │          │  10     20     30     │
  │  cap = 3         │          └───────────────────────┘
  └──────────────────┘                  │  │  │
                                        │  │  └── copy
                                copy ───┘  └────── copy

  REALLOCATION (new array at 2x capacity):
  ┌──────────────────┐          ┌───────────────────────────────┐
  │  s               │          │    NEW ARRAY (2x capacity)    │
  │  ptr ────────────┼─────X    │ [0]    [1]    [2]    [3]     │
  │  len = 4         │    ╱     │  10     20     30    [40]    │
  │  cap = 6         │   ╱      └───────────────────────────────┘
  └──────────────────┘  ╱                   ▲
          │             ╱               new element
          │            ╱
          │           ╱   (old array freed by GC)
          └──────────╱
           ptr updated ──► new array
```

### Visual: Copy Function

```go
src := []int{1, 2, 3, 4, 5}
dst := make([]int, 3)
n := copy(dst, src)  // copies 3 elements
```

```
  BEFORE copy:
  src:  ┌──────┬──────┬──────┬──────┬──────┐
        │   1  │   2  │   3  │   4  │   5  │   (len=5)
        └──────┴──────┴──────┴──────┴──────┘
                       │  │  │
              copy ────┘  │  └──── copy
                 copy ────┘
  dst:  ┌──────┬──────┬──────┐
        │   0  │   0  │   0  │                (len=3)
        └──────┴──────┴──────┘

  AFTER copy (n = 3):
  src:  ┌──────┬──────┬──────┬──────┬──────┐
        │   1  │   2  │   3  │   4  │   5  │   (unchanged)
        └──────┴──────┴──────┴──────┴──────┘
  dst:  ┌──────┬──────┬──────┐
        │   1  │   2  │   3  │                (n = 3 copied)
        └──────┴──────┴──────┘
```

### What the Runtime Sees

```go
// This is essentially what a slice looks like in memory:
type sliceHeader struct {
    Data unsafe.Pointer  // Pointer to underlying array
    Len  int             // Number of elements
    Cap  int             // Capacity from Data to end of array
}
```

### Visual Memory Layout

```
  STACK:                          HEAP:
  ┌───────────────────┐          ┌──────┬──────┬──────┬──────┬──────┐
  │  slice variable   │          │  1   │  2   │  3   │      │      │
  │  ┌──────────────┐ │    ┌────►├──────┼──────┼──────┼──────┼──────┤
  │  │ Data ────────┼─┼────┘     │ [0]  │ [1]  │ [2]  │ [3]  │ [4]  │
  │  │ Len = 3      │ │          └──────┴──────┴──────┴──────┴──────┘
  │  │ Cap = 5      │ │           ◄── accessible ──►◄── reserved ──►
  │  └──────────────┘ │
  └───────────────────┘
```

### Size of a Slice Header

```go
// A slice header is always 24 bytes on 64-bit systems:
// - 8 bytes pointer
// - 8 bytes length
// - 8 bytes capacity
fmt.Println(unsafe.Sizeof([]int{}))  // 24
```

### Implications

1. **Passing slices to functions is cheap** — you copy 24 bytes, not the data
2. **Two slices can share the same underlying array** — the shared-array bug
3. **`append` may or may not share** — you can't tell without checking pointer identity

---

## 11. Passing Slices to Functions [CORE]

### Pass by Value (Default)

```go
func process(s []int) {
    // s is a copy of the slice HEADER
    // But the DATA is shared
    s[0] = 999    // Modifies original data
    s = append(s, 4)  // Does NOT modify original slice's len
}

func main() {
    s := []int{1, 2, 3}
    process(s)
    fmt.Println(s)      // [999, 2, 3] — s[0] was modified
    fmt.Println(len(s)) // 3 — len unchanged (append returned new header)
}
```

### Return New Slice

```go
func appendOne(s []int) []int {
    return append(s, 1)
}

func main() {
    s := []int{1, 2}
    s = appendOne(s)  // Must capture return
    fmt.Println(s)    // [1, 2, 1]
}
```

### Modify via Pointer

```go
func appendOne(s *[]int) {
    *s = append(*s, 1)
}

func main() {
    s := []int{1, 2}
    appendOne(&s)
    fmt.Println(s)  // [1, 2, 1]
}
```

**Prefer returning new slices over passing pointers.** It's more idiomatic.

---

## 12. Common Patterns [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

> **Generics Primer:** The functions below use Go generics syntax like `[T any]` and `[T comparable]`. If this is unfamiliar, here's the quick version:
> - `[T any]` means "this function works with any type `T`" — `T` is a placeholder the compiler fills in when you call the function.
> - `[T comparable]` means "any type `T` that supports `==` and `!=`" (ints, strings, structs of comparable fields — but not slices or maps).
> - Full explanation in [Topic 10: Generics](../05-generics/10-generics.md).

### Filter

```go
func filter[T any](s []T, predicate func(T) bool) []T {
    result := make([]T, 0)
    for _, v := range s {
        if predicate(v) {
            result = append(result, v)
        }
    }
    return result
}

// Usage
evens := filter([]int{1, 2, 3, 4, 5}, func(n int) bool {
    return n%2 == 0
})
```

### Map

```go
func mapSlice[T, U any](s []T, f func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s {
        result[i] = f(v)
    }
    return result
}
```

### Reduce

```go
func reduce[T, U any](s []T, initial U, f func(U, T) U) U {
    result := initial
    for _, v := range s {
        result = f(result, v)
    }
    return result
}
```

### Reverse In-Place

```go
func reverse[T any](s []T) {
    for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
        s[i], s[j] = s[j], s[i]
    }
}
```

### Remove Duplicates

```go
func unique[T comparable](s []T) []T {
    seen := make(map[T]struct{})
    result := make([]T, 0, len(s))
    for _, v := range s {
        if _, ok := seen[v]; !ok {
            seen[v] = struct{}{}
            result = append(result, v)
        }
    }
    return result
}
```

### Chunk

```go
func chunks[T any](s []T, size int) [][]T {
    var chunks [][]T
    for i := 0; i < len(s); i += size {
        end := i + size
        if end > len(s) {
            end = len(s)
        }
        chunks = append(chunks, s[i:end:end])  // Capped capacity!
    }
    return chunks
}
```

### Contains

```go
func contains[T comparable](s []T, v T) bool {
    for _, item := range s {
        if item == v {
            return true
        }
    }
    return false
}
```

### Delete at Index

```go
// Order preserved
func deleteOrdered[T any](s []T, i int) []T {
    copy(s[i:], s[i+1:])
    return s[:len(s)-1]
}

// Order NOT preserved (O(1))
func deleteUnordered[T any](s []T, i int) []T {
    s[i] = s[len(s)-1]
    return s[:len(s)-1]
}
```

---

## 13. Performance Considerations [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

Slices are the most-used data structure in Go. In hot paths — processing thousands of items per request, handling high-throughput data pipelines — slice performance directly impacts latency. These patterns are standard practice, not premature optimization.

### Pre-allocate When Size is Known

```go
// SLOW — O(n log n) allocations
var s []int
for i := 0; i < n; i++ {
    s = append(s, i)
}

// FAST — O(1) allocation
s := make([]int, 0, n)
for i := 0; i < n; i++ {
    s = append(s, i)
}
```

### Use `copy` Instead of Loop

```go
// SLOW
for i, v := range src {
    dst[i] = v
}

// FAST — optimized by runtime, can use memcpy
copy(dst, src)
```

### Avoid Repeated Slicing

```go
// SLOW — creates new slice headers each time
for i := 0; i < len(s); i++ {
    process(s[i : i+1])
}

// FAST — single slice
for i := 0; i < len(s); i++ {
    process(s[i])
}
```

### Slice of Pointers vs Slice of Values

```go
// Slice of values — data contiguous in memory (cache-friendly)
users := []User{{Name: "Alice"}, {Name: "Bob"}}

// Slice of pointers — data scattered on heap (cache-unfriendly)
users := []*User{{Name: "Alice"}, {Name: "Bob"}}
```

Use pointer slices when:
- Structs are very large (>64 bytes)
- You need to pass elements to functions that modify them
- Multiple slices need to reference the same objects

---

## 14. Common Pitfalls [CORE]

### 1. The Shared-Array Bug (Most Common)

```go
// BUG
func getFirstN(s []int, n int) []int {
    return s[:n]
}

// FIX
func getFirstN(s []int, n int) []int {
    return s[:n:n]  // Cap capacity
}
```

### 2. Forgetting to Capture `append` Return

```go
// BUG
func addItem(s []int, v int) {
    s = append(s, v)  // Local modification only
}

// FIX
func addItem(s []int, v int) []int {
    return append(s, v)
}
```

### 3. Modifying Slice During Iteration

```go
// BUG — may skip elements or go out of bounds
for i, v := range s {
    if v == target {
        s = append(s[:i], s[i+1:]...)  // Slice modified during range!
    }
}

// FIX — iterate backwards or collect indices first
for i := len(s) - 1; i >= 0; i-- {
    if s[i] == target {
        s = append(s[:i], s[i+1:]...)
    }
}
```

### 4. Appending in a Loop (Exponential Growth)

```go
// May cause many reallocations
var s []int
for i := 0; i < 1000000; i++ {
    s = append(s, i)  // Grows ~20 times
}

// Pre-allocate
s := make([]int, 0, 1000000)
```

### 5. Nil Slice JSON

```go
type Response struct {
    Data []Item `json:"data"`
}

// Returns {"data": null} — clients may not handle well
resp := Response{Data: nil}

// Returns {"data": []} — better for API consumers
resp := Response{Data: []Item{}}
```

### 6. Slice as Map Key

```go
// COMPILE ERROR: invalid map key type []int
m := map[[]int]string{}

// Fix: use string key
key := fmt.Sprintf("%v", slice)
m[key] = "value"
```

### 7. Comparing Slices

```go
a := []int{1, 2, 3}
b := []int{1, 2, 3}
// a == b  // COMPILE ERROR: slice can only be compared to nil

// Fix: use reflect.DeepEqual or slices.Equal (Go 1.21+)
import "slices"
fmt.Println(slices.Equal(a, b))  // true

// Or manually
func equal(a, b []int) bool {
    if len(a) != len(b) {
        return false
    }
    for i := range a {
        if a[i] != b[i] {
            return false
        }
    }
    return true
}
```

---

## 15. Production Patterns [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

**Choose your data structure based on access pattern:**
- **Ring Buffer** — fixed-size sliding window (last N events, rate limiting, log rotation)
- **Stack** (LIFO) — processing nested structures (parsers, DFS, undo history)
- **Queue** (FIFO) — processing items in order (job queues, BFS, message buffers)

### Ring Buffer (Circular Buffer)

```go
type RingBuffer struct {
    data  []int
    head  int
    tail  int
    count int
}

func NewRingBuffer(cap int) *RingBuffer {
    return &RingBuffer{
        data: make([]int, cap),
    }
}

func (r *RingBuffer) Push(n int) {
    r.data[r.tail] = n
    r.tail = (r.tail + 1) % len(r.data)
    if r.count < len(r.data) {
        r.count++
    } else {
        r.head = (r.head + 1) % len(r.data)
    }
}

func (r *RingBuffer) Pop() (int, bool) {
    if r.count == 0 {
        return 0, false
    }
    n := r.data[r.head]
    r.head = (r.head + 1) % len(r.data)
    r.count--
    return n, true
}
```

### Stack Implementation

> These data structures use Go generics (`[T any]`). If unfamiliar, see [Topic 10: Generics](../05-generics/10-generics.md).

```go
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
```

### Queue Implementation

```go
type Queue[T any] struct {
    items []T
}

func (q *Queue[T]) Enqueue(v T) {
    q.items = append(q.items, v)
}

func (q *Queue[T]) Dequeue() (T, bool) {
    if len(q.items) == 0 {
        var zero T
        return zero, false
    }
    v := q.items[0]
    q.items = q.items[1:]
    return v, true
}

func (q *Queue[T]) Len() int {
    return len(q.items)
}
```

---

## 16. Memory Optimization [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

### Pre-allocation

```go
// BAD: Multiple reallocations
func bad(n int) []int {
    var s []int
    for i := 0; i < n; i++ {
        s = append(s, i)
    }
    return s
}

// GOOD: Pre-allocate
func good(n int) []int {
    s := make([]int, 0, n)
    for i := 0; i < n; i++ {
        s = append(s, i)
    }
    return s
}
```

### Reuse Slice with Clear

```go
func processBatch(items []Item) []Result {
    results := make([]Result, 0, len(items))
    
    for _, item := range items {
        results = append(results, transform(item))
    }
    
    return results
}

// Clear and reuse (for hot paths)
func reuseSlice() {
    s := make([]int, 0, 1000)
    
    for {
        // Process batch
        s = s[:0] // Clear without reallocating
        // Fill s...
    }
}
```

---

## 17. Slice vs Array Performance [INTERNALS]

> ⏭️ **First pass? Skip this section.** This covers low-level internals. Come back after completing Topics 1-10.

```go
func benchmark() {
    // Arrays are on stack - faster for small fixed size
    arr := [3]int{1, 2, 3}
    
    // Slices are on heap when appended
    slc := []int{1, 2, 3}
    
    // For small fixed collections, arrays can be faster
    // For dynamic collections, slices are more convenient
}
```

---

## 18. Testing Slices [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

```go
func TestSliceOperations(t *testing.T) {
    // Test append
    s := []int{}
    s = append(s, 1, 2, 3)
    if len(s) != 3 {
        t.Errorf("expected len 3, got %d", len(s))
    }
    
    // Test subslice
    sub := s[1:3]
    if sub[0] != 2 {
        t.Errorf("expected 2, got %d", sub[0])
    }
    
    // Test copy
    dst := make([]int, 3)
    copy(dst, s)
    if dst[0] != 1 {
        t.Errorf("expected 1, got %d", dst[0])
    }
}

func TestSliceContains(t *testing.T) {
    s := []int{1, 2, 3, 4, 5}
    
    // Manual check
    found := false
    for _, v := range s {
        if v == 3 {
            found = true
            break
        }
    }
    if !found {
        t.Error("expected to find 3")
    }
    
    // Using slices package (Go 1.21+)
    if !slices.Contains(s, 3) {
        t.Error("expected to contain 3")
    }
}
```

---

## Exercises

### Exercise 1: Append and Track Length/Capacity ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create an empty slice of integers. Append 5 elements one at a time. After each `append`, print the element, `len`, and `cap`. Observe when the capacity doubles versus when it stays the same.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func main() {
	s := []int{}
	for i := 1; i <= 5; i++ {
		s = append(s, i*10)
		fmt.Printf("appended %2d → len=%d, cap=%d\n", i*10, len(s), cap(s))
	}
}
```

</details>

### Exercise 2: Shared-Array Bug and the Fix ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a slice `original := []int{1, 2, 3, 4, 5}`. Take a subslice `sub := original[1:4]`, append a value to `sub`, and print `original` to show it was corrupted. Then fix the bug using `copy()` so appending to the subslice no longer affects the original.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func main() {
	// Bug: shared backing array
	original := []int{1, 2, 3, 4, 5}
	sub := original[1:4]
	sub = append(sub, 99)
	fmt.Println("corrupted:", original) // [1 2 3 99 5]

	// Fix: independent copy
	original2 := []int{1, 2, 3, 4, 5}
	sub2 := make([]int, 3)
	copy(sub2, original2[1:4])
	sub2 = append(sub2, 99)
	fmt.Println("safe:     ", original2) // [1 2 3 4 5]
	fmt.Println("sub2:     ", sub2)       // [2 3 4 99]
}
```

</details>

### Exercise 3: Nil Slice vs Empty Slice ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Declare a nil slice and an empty slice (using both `[]int{}` and `make([]int, 0)`). For each one, print `== nil`, `len()`, and `cap()`. Confirm that nil and empty slices behave the same for `len`/`cap`/`append` but differ in `== nil` comparison.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func main() {
	var nilSlice []int
	emptyLit := []int{}
	emptyMake := make([]int, 0)

	printInfo := func(name string, s []int) {
		fmt.Printf("%-12s  nil=%-5v  len=%d  cap=%d\n", name, s == nil, len(s), cap(s))
	}

	printInfo("nilSlice", nilSlice)
	printInfo("emptyLit", emptyLit)
	printInfo("emptyMake", emptyMake)

	// All work identically for append
	nilSlice = append(nilSlice, 1)
	emptyLit = append(emptyLit, 1)
	fmt.Println("After append:", nilSlice, emptyLit) // [1] [1]
}
```

</details>

### Exercise 4: Reverse a Slice ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Write a function `func reverse(s []int) []int` that returns a **new** slice with the elements in reversed order. The original slice must remain unchanged. Call it from `main` and print both slices.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func reverse(s []int) []int {
	result := make([]int, len(s))
	for i, v := range s {
		result[len(s)-1-i] = v
	}
	return result
}

func main() {
	original := []int{1, 2, 3, 4, 5}
	reversed := reverse(original)
	fmt.Println("original:", original) // [1 2 3 4 5]
	fmt.Println("reversed:", reversed) // [5 4 3 2 1]
}
```

</details>

---

## Next: [Maps →](./04-maps.md)
