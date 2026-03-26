# 4. Maps — Complete Deep Dive

> **Goal:** Master Go maps — creation, usage, concurrency, and every production pitfall.

![Maps Overview](../assets/04.png)

---

## Table of Contents

1. [Map Basics](#1-map-basics)
2. [Creation Methods](#2-creation-methods)
3. [CRUD Operations](#3-crud-operations)
4. [Iteration](#4-iteration)
5. [Map Internals](#5-map-internals)
6. [Key Types](#6-key-types)
7. [Nil Map Behavior](#7-nil-map-behavior)
8. [Concurrency](#8-concurrency)
9. [Common Patterns](#9-common-patterns)
10. [Performance](#10-performance)
11. [Common Pitfalls](#11-common-pitfalls)

---

## 1. Map Basics [CORE]

A map is an **unordered** collection of key-value pairs. It's a reference type (like slices and channels).

```go
// Syntax: map[KeyType]ValueType
var m map[string]int
```

### Properties

| Property | Value |
|----------|-------|
| Ordered | **No** — iteration order is random |
| Key type | Must be comparable (`==`, `!=`) |
| Value type | Any type |
| Zero value | `nil` |
| Reference type | Modifying a copy affects original |
| Concurrent safe | **No** — need `sync.Map` or mutex |

---

## 2. Creation Methods [CORE]

### Method 1: `make()`

```go
m := make(map[string]int)       // Empty, ready to use
m := make(map[string]int, 100)  // Hint: ~100 entries (performance optimization)
```

### Method 2: Literal

```go
m := map[string]int{
    "alice": 25,
    "bob":   30,
    "charlie": 35,
}
```

### Method 3: Nil Map (Read-Only)

```go
var m map[string]int
// m == nil
// Can READ (returns zero value)
// Can NOT WRITE (panics)
```

### Pre-allocation

```go
// If you know approximate size, pre-allocate
m := make(map[string]int, 10000)
// Avoids rehashing during insertion
```

---

## 3. CRUD Operations [CORE]

### Create / Update

```go
m := make(map[string]int)
m["alice"] = 25    // Create
m["alice"] = 26    // Update
```

### Read

```go
age := m["alice"]         // Returns zero value if key doesn't exist
age, exists := m["alice"] // Comma-ok idiom
```

### Delete

```go
delete(m, "alice")  // No-op if key doesn't exist
```

### Check Existence (Comma-Ok)

```go
if age, ok := m["alice"]; ok {
    fmt.Printf("Alice is %d\n", age)
} else {
    fmt.Println("Alice not found")
}
```

**This is critical.** Without comma-ok, you can't distinguish "key has value 0" from "key doesn't exist."

### Full CRUD Example

```go
func main() {
    // Create
    scores := map[string]int{
        "alice": 100,
        "bob":   85,
    }

    // Read with existence check
    if score, ok := scores["alice"]; ok {
        fmt.Printf("Alice: %d\n", score)
    }

    // Update
    scores["alice"] = 110

    // Delete
    delete(scores, "bob")

    // Check if key exists
    _, exists := scores["bob"]
    fmt.Printf("Bob exists: %v\n", exists)  // false

    // Get with default
    charlieScore := scores["charlie"]  // 0 (zero value)
    fmt.Printf("Charlie: %d\n", charlieScore)
}
```

---

## 4. Iteration [CORE]

### Basic Range

```go
m := map[string]int{"a": 1, "b": 2, "c": 3}

for key, value := range m {
    fmt.Printf("%s: %d\n", key, value)
}
```

### Order is Random

Go intentionally randomizes map iteration to prevent programs from depending on order. If your code depends on map order, it is already broken — the randomization just makes the bug obvious during development rather than silently corrupting production data.

```go
m := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}

// Each run prints in DIFFERENT order
for k := range m {
    fmt.Println(k)
}
// Run 1: c, a, e, b, d
// Run 2: e, b, a, d, c
// Run 3: a, d, c, e, b
```

**Why random?** Go intentionally randomizes map iteration to prevent programs from depending on order.

### Sorted Iteration

```go
import "sort"

m := map[string]int{"c": 3, "a": 1, "b": 2}

// Get sorted keys
keys := make([]string, 0, len(m))
for k := range m {
    keys = append(keys, k)
}
sort.Strings(keys)

// Iterate in sorted order
for _, k := range keys {
    fmt.Printf("%s: %d\n", k, m[k])
}
// Output: a: 1, b: 2, c: 3
```

### Keys Only / Values Only

```go
// Keys only
for key := range m {
    fmt.Println(key)
}

// Values only
for _, value := range m {
    fmt.Println(value)
}
```

### Get All Keys / Values

```go
import "maps"

keys := slices.Collect(maps.Keys(m))   // Go 1.23+
values := slices.Collect(maps.Values(m))  // Go 1.23+

// Or manually
keys := make([]string, 0, len(m))
for k := range m {
    keys = append(keys, k)
}
```

---

## 5. Map Internals [INTERNALS]

> ⏭️ **First pass? Skip this section.** This covers low-level internals. Come back after completing Topics 1-10.

### How Maps Work

A map is backed by a hash table with buckets:

```
  ┌───────────────────────────────────────────────────────────────────────────┐
  │                           MAP STRUCTURE                                   │
  ├───────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   map[string]int                                                         │
  │                                                                           │
  │   ┌─────────────────────────────────────────────────────┐                │
  │   │                MAP HEADER (hmap)                     │                │
  │   │  ┌──────────┬──────┬───────┬──────────┬───────────┐ │                │
  │   │  │  count   │  B   │ flags │  hash0   │  buckets  │ │                │
  │   │  │    5     │  3   │   0   │ 0xabc..  │   0x123 ──┼─┼──► bucket[]  │
  │   │  └──────────┴──────┴───────┴──────────┴───────────┘ │                │
  │   └─────────────────────────────────────────────────────┘                │
  │                              │                                            │
  │                              ▼                                            │
  │   ┌────────────────────────────────────────────────────────────────────┐ │
  │   │                  BUCKET ARRAY  (2^B = 8 buckets)                   │ │
  │   │                                                                     │ │
  │   │  bkt[0]    bkt[1]    bkt[2]    bkt[3]   ...    bkt[7]             │ │
  │   │  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐       ┌──────┐           │ │
  │   │  │      │  │ "a"  │  │      │  │ "d"  │       │      │           │ │
  │   │  │  10  │  │ 100  │  │  20  │  │ 400  │       │  50  │           │ │
  │   │  │      │  ├──────┤  │      │  ├──────┤       ├──────┤           │ │
  │   │  │      │  │ "b"  │  │ "c"  │  │      │       │ "g"  │           │ │
  │   │  │      │  │ 200  │  │ 300  │  │      │       │ 700  │           │ │
  │   │  └──────┘  └──────┘  └──────┘  └──────┘       └──────┘           │ │
  │   │     │                                                    │         │ │
  │   │     └──── overflow chain ────────────────────────────────┘         │ │
  │   │            (when bucket fills up)                                  │ │
  │   └────────────────────────────────────────────────────────────────────┘ │
  │                                                                           │
  └───────────────────────────────────────────────────────────────────────────┘
```

### Visual: Hash Table Bucket Structure

```
  SINGLE BUCKET (max 8 key-value pairs):

  ┌───────────────────────────────────────────────────────────────┐
  │                      BUCKET HEADER                            │
  │  ┌─────────┬─────────┬─────────┬─────────┬─────────┬───────┐ │
  │  │tophash[0│tophash[1│tophash[2│  ...    │tophash[7│overflow│ │
  │  │  = h1   │  = h2   │  = h3   │         │  = h8   │  ptr──┼─┼──► next
  │  └─────────┴─────────┴─────────┴─────────┴─────────┴───────┘ │
  ├───────────────────────────────────────────────────────────────┤
  │                      DATA SECTION                             │
  │                                                               │
  │  ┌─────────────────────────────────────────────────────────┐ │
  │  │  tophash   │      keys (8 * K)      │   values (8 * V) │ │
  │  │  8 bytes   │  [k1][k2][k3]...[k8]   │ [v1][v2][v3]...  │ │
  │  └─────────────────────────────────────────────────────────┘ │
  │                                                               │
  │  K = sizeof(key type)    V = sizeof(value type)              │
  └───────────────────────────────────────────────────────────────┘
```

### Visual: Lookup Process

```
  LOOKUP: m["b"]   where   m = {"a":100, "b":200, "c":300}

  Step 1 ── Compute hash
  ┌─────────────────────────────────────────────────────────┐
  │  hash("b") = 0x6E8A2B4C...                             │
  └─────────────────────────────────────────────────────────┘
            │
            ▼
  Step 2 ── Extract top 8 bits for bucket index
  ┌─────────────────────────────────────────────────────────┐
  │  topbits = 0x6E  →  bucket index = 0x6E % 2^3 = 6     │
  └─────────────────────────────────────────────────────────┘
            │
            ▼
  Step 3 ── Search bucket[6]
  ┌─────────────────────────────────────────────────────────┐
  │  bucket[6]:                                             │
  │   ┌──────────────────────┬──────────────────────────┐   │
  │   │  tophash  │  key     │  value                   │   │
  │   ├───────────┼──────────┼──────────────────────────┤   │
  │   │  0x6E     │  "b"     │  200   ◄── MATCH! ✓     │   │
  │   ├───────────┼──────────┼──────────────────────────┤   │
  │   │  ...      │  ...     │  ...                     │   │
  │   └───────────┴──────────┴──────────────────────────┘   │
  └─────────────────────────────────────────────────────────┘
            │
            ▼
  Step 4 ── If NOT found in bucket → follow overflow chain
  ┌─────────────────────────────────────────────────────────┐
  │  bucket ──► overflow ──► overflow ──► nil (not found)   │
  │                                              │          │
  │                            return zero value ◄┘         │
  └─────────────────────────────────────────────────────────┘
```

### Visual: Map Growth (Rehashing)

> **Rehashing** is what happens when a map runs out of bucket space. Go creates a new array with twice as many buckets and redistributes every existing key into the new buckets using its hash. This is O(n) but happens infrequently, so the *amortized* cost per insertion is still O(1). During growth, Go migrates buckets incrementally (not all at once) to avoid long pauses.

```
  BEFORE GROWTH (B=2, 4 buckets, load ~6.75):
  ┌──────────────────────────────────────────────────────────┐
  │  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐                │
  │  │bkt[0]│  │bkt[1]│  │bkt[2]│  │bkt[3]│  27 entries    │
  │  │  7   │  │  7   │  │  7   │  │  6   │  load = 6.75   │
  │  └──────┘  └──────┘  └──────┘  └──────┘  threshold! ⚠  │
  └──────────────────────────────────────────────────────────┘
                            │
                            │  load > 6.5 → GROW
                            ▼
  AFTER GROWTH (B=3, 8 buckets, load ~3.4):
  ┌──────────────────────────────────────────────────────────┐
  │  ┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌────┐│┌────┐
  │  │bkt[0]││bkt[1]││bkt[2]││bkt[3]││bkt[4]││bkt[5]││[6] │││[7] │
  │  │  3   ││  4   ││  3   ││  3   ││  4   ││  3   ││ 3  │││ 4  │
  │  └──────┘└──────┘└──────┘└──────┘└──────┘└──────┘└────┘│└────┘
  │                                                          │
  │  old entries migrate lazily (incremental rehashing)      │
  │  each access moves a few entries from old → new          │
  └──────────────────────────────────────────────────────────┘
```

### Size of a Map Header

```go
// A map variable is a pointer to the map header
// The variable itself is 8 bytes (pointer on 64-bit)
fmt.Println(unsafe.Sizeof(m))  // 8
```

---

## 6. Key Types [CORE]

### Comparable Types (Can Be Keys)

```go
// ✅ Valid keys
map[int]string
map[string]int
map[float64]bool  // ⚠️ See floating-point warning below
map[bool]int
map[[3]int]string  // Arrays are comparable
map[struct{X, Y int}]string  // Structs with comparable fields
```

### Non-Comparable Types (Cannot Be Keys)

```go
// ❌ Invalid keys
map[[]int]string      // Slices not comparable
map[map[int]int]string // Maps not comparable
map[func()]string     // Functions not comparable
```

### Structs as Keys

```go
type Point struct {
    X, Y int
}

visited := map[Point]bool{}
visited[Point{1, 2}] = true
visited[Point{3, 4}] = true

if visited[Point{1, 2}] {
    fmt.Println("Already visited (1, 2)")
}
```

### Slices as Keys (Workaround)

> **Packages used below:** `encoding/hex` converts bytes to hex strings. `encoding/binary` writes integers as raw bytes in big-endian order (most significant byte first). `fmt.Sprintf("%v", slice)` is the simpler approach — it converts anything to its default string representation.

```go
// Convert slice to string
import "encoding/hex"

func sliceKey(s []int) string {
    b := make([]byte, len(s)*8)
    for i, v := range s {
        // binary.BigEndian.PutUint64 writes a uint64 as 8 bytes in big-endian order
        // See: go doc encoding/binary.PutUint64
        binary.BigEndian.PutUint64(b[i*8:], uint64(v))
    }
    return hex.EncodeToString(b)
}

// Or simpler for small slices
func sliceKey(s []int) string {
    return fmt.Sprintf("%v", s)
}
```

### Floating-Point Keys (Warning)

```go
m := map[float64]string{}
m[1.0] = "one"
m[1.0000000000000001] = "almost one"  // Might overwrite "one"!

// IEEE 754 precision issues
a := 0.1 + 0.2
b := 0.3
fmt.Println(a == b)           // false
fmt.Println(m[a])             // might not find
```

**Rule:** Avoid float keys. Use string representation if needed.

---

## 7. Nil Map Behavior [CORE]

### Reading from Nil Map

```go
var m map[string]int
fmt.Println(m["key"])  // 0 — returns zero value, NO panic
fmt.Println(len(m))    // 0
fmt.Println(m == nil)  // true

for k, v := range m {  // No iterations
    // Never executes
}
```

### Writing to Nil Map — PANIC

```go
var m map[string]int
m["key"] = 1  // PANIC: assignment to entry in nil map
```

### Delete from Nil Map

```go
var m map[string]int
delete(m, "key")  // No-op, no panic
```

### Production Rule

**Always initialize maps before writing.**

```go
// In a struct — initialize in constructor
type Service struct {
    cache map[string]*User
}

func NewService() *Service {
    return &Service{
        cache: make(map[string]*User),  // Don't forget this!
    }
}
```

---

## 8. Concurrency [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

> **Prerequisite:** This section uses `sync.Mutex`, `sync.RWMutex`, and goroutines — all covered in detail in the Concurrency section (Topics 11-16). If you haven't read those yet, skim this section now and revisit after learning concurrency. The key concept: maps are NOT safe for concurrent read/write. You need a lock.

### Maps Are NOT Concurrent-Safe

Maps use internal hash table state that is not atomic. When one goroutine reads a bucket while another rehashes it, the Go runtime detects this and **panics** with `concurrent map read and map write`. This is a hard crash — not just a data race. It always panics, even without `-race`.

```go
// DATA RACE — will panic with -race flag
m := make(map[string]int)

go func() {
    for {
        m["key"]++
    }
}()

go func() {
    for {
        _ = m["key"]
    }
}()
```

### Option 1: `sync.Map`

```go
import "sync"

var m sync.Map

// Store
m.Store("key", "value")

// Load
val, ok := m.Load("key")

// Load or Store (atomic)
val, loaded := m.LoadOrStore("key", "value")

// Delete
m.Delete("key")

// Range (iterate)
m.Range(func(key, value any) bool {
    fmt.Println(key, value)
    return true  // return false to stop
})
```

**When to use `sync.Map`:**
- Keys are mostly read, rarely written
- Different goroutines access disjoint sets of keys
- You don't know the key set in advance

**When NOT to use `sync.Map`:**
- Keys are frequently written — use `sync.RWMutex` + regular map
- You need type safety — `sync.Map` uses `any` (no generics until Go 1.24)

### Option 2: `sync.RWMutex` + Regular Map

```go
type SafeMap struct {
    mu sync.RWMutex
    m  map[string]int
}

func NewSafeMap() *SafeMap {
    return &SafeMap{m: make(map[string]int)}
}

func (s *SafeMap) Get(key string) (int, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    val, ok := s.m[key]
    return val, ok
}

func (s *SafeMap) Set(key string, value int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.m[key] = value
}

func (s *SafeMap) Delete(key string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.m, key)
}

func (s *SafeMap) Len() int {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return len(s.m)
}
```

**This is the preferred approach for most production code.** Type-safe, simple, and `RWMutex` allows concurrent reads.

### Option 3: Sharded Map (High Concurrency)

> **`fnv.New32a()`** creates a fast, non-cryptographic hash function (FNV-1a). It's used here to deterministically map string keys to shard indices. `import "hash/fnv"` is needed. See: `go doc hash/fnv`.

```go
const shards = 32

type ShardedMap struct {
    shards [shards]struct {
        sync.RWMutex
        m map[string]int
    }
}

func NewShardedMap() *ShardedMap {
    sm := &ShardedMap{}
    for i := range sm.shards {
        sm.shards[i].m = make(map[string]int)
    }
    return sm
}

func (sm *ShardedMap) getShard(key string) *struct {
    sync.RWMutex
    m map[string]int
} {
    h := fnv.New32a()
    h.Write([]byte(key))
    return &sm.shards[h.Sum32()%shards]
}

func (sm *ShardedMap) Get(key string) (int, bool) {
    shard := sm.getShard(key)
    shard.RLock()
    defer shard.RUnlock()
    val, ok := shard.m[key]
    return val, ok
}

func (sm *ShardedMap) Set(key string, value int) {
    shard := sm.getShard(key)
    shard.Lock()
    defer shard.Unlock()
    shard.m[key] = value
}
```

This reduces lock contention by distributing keys across 32 independent shards.

---

## 9. Common Patterns [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

### Counting / Frequency

```go
func wordCount(s string) map[string]int {
    counts := make(map[string]int)
    for _, word := range strings.Fields(s) {
        // strings.Fields splits a string by whitespace: "hello world foo" → ["hello", "world", "foo"]
        // See: go doc strings.Fields
        counts[word]++
    }
    return counts
}
```

### Group By

```go
type User struct {
    Name string
    Age  int
    City string
}

func groupByCity(users []User) map[string][]User {
    groups := make(map[string][]User)
    for _, u := range users {
        groups[u.City] = append(groups[u.City], u)
    }
    return groups
}
```

### Set (Using `map[T]struct{}`)

> **Generics Primer:** The Set and Cache patterns below use Go generics `[T comparable]`. If this syntax is unfamiliar, `comparable` means "any type that supports `==`". See [Topic 10: Generics](../05-generics/10-generics.md) for the full explanation.

```go
type Set[T comparable] map[T]struct{}

func NewSet[T comparable](items ...T) Set[T] {
    s := make(Set[T], len(items))
    for _, item := range items {
        s[item] = struct{}{}
    }
    return s
}

func (s Set[T]) Add(item T)    { s[item] = struct{}{} }
func (s Set[T]) Remove(item T) { delete(s, item) }
func (s Set[T]) Has(item T) bool {
    _, ok := s[item]
    return ok
}

// Usage
visited := NewSet("alice", "bob")
visited.Add("charlie")
if visited.Has("alice") {
    fmt.Println("Alice was visited")
}
```

**Why `struct{}`?** It's zero bytes — `bool` is 1 byte. For millions of entries, this matters.

### Cache / Memoization

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
    val, ok := c.data[key]
    return val, ok
}

func (c *Cache[K, V]) Set(key K, value V) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[key] = value
}

func (c *Cache[K, V]) GetOrCompute(key K, compute func() V) V {
    c.mu.RLock()
    if val, ok := c.data[key]; ok {
        c.mu.RUnlock()
        return val
    }
    c.mu.RUnlock()

    c.mu.Lock()
    defer c.mu.Unlock()
    // Double-check after acquiring write lock
    if val, ok := c.data[key]; ok {
        return val
    }
    val := compute()
    c.data[key] = val
    return val
}
```

### Invert Map

```go
func invert[K, V comparable](m map[K]V) map[V]K {
    result := make(map[V]K, len(m))
    for k, v := range m {
        result[v] = k
    }
    return result
}
```

### Merge Maps

```go
func merge[K comparable, V any](maps ...map[K]V) map[K]V {
    result := make(map[K]V)
    for _, m := range maps {
        for k, v := range m {
            result[k] = v
        }
    }
    return result
}
```

### Get with Default

```go
func getOrDefault[K comparable, V any](m map[K]V, key K, defaultVal V) V {
    if val, ok := m[key]; ok {
        return val
    }
    return defaultVal
}
```

---

## 10. Performance [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

### Pre-allocation

```go
// SLOW — triggers multiple rehashings
m := make(map[string]int)
for i := 0; i < 1000000; i++ {
    m[fmt.Sprintf("key%d", i)] = i
}

// FAST — single allocation
m := make(map[string]int, 1000000)
for i := 0; i < 1000000; i++ {
    m[fmt.Sprintf("key%d", i)] = i
}
```

### Key Type Matters

```go
// SLOW — string comparison is O(n)
m := make(map[string]int)

// FASTER — int comparison is O(1)
m := make(map[int]int)

// FASTEST — use int keys when possible
```

### `delete` vs Setting to Zero

```go
// delete removes the key entirely
delete(m, "key")

// Setting to zero keeps the key
m["key"] = 0  // Key still exists in map!
```

**Always use `delete` to remove entries.** Setting to zero wastes memory.

### Iteration Overhead

```go
// Creating slices of keys has allocation overhead
keys := make([]string, 0, len(m))
for k := range m {
    keys = append(keys, k)
}
sort.Strings(keys)

// If you only need to process all entries, just range directly
for k, v := range m {
    process(k, v)
}
```

---

## 11. Common Pitfalls [CORE]

### 1. Writing to Nil Map

```go
var m map[string]int
m["key"] = 1  // PANIC

// Fix:
m := make(map[string]int)
m["key"] = 1  // OK
```

### 2. Assuming Map Order

```go
m := map[int]string{1: "a", 2: "b", 3: "c"}
for k, v := range m {
    fmt.Println(k, v)
}
// Order: 2 b, 3 c, 1 a (or any other permutation)

// Fix: sort keys if order matters
```

### 3. Using Map as Set (Without `struct{}`)

```go
// WRONG — wastes memory
set := map[string]bool{}
set["key"] = true

// RIGHT — zero-byte value
set := map[string]struct{}{}
set["key"] = struct{}{}
```

### 4. Concurrent Map Access

```go
// PANIC: concurrent map read and map write
go func() { m["key"] = 1 }()
go func() { _ = m["key"] }()

// Fix: use sync.Map or RWMutex
```

### 5. Comparing Maps with `==`

```go
a := map[string]int{"x": 1}
b := map[string]int{"x": 1}
// a == b  // COMPILE ERROR: map can only be compared to nil

// Fix: use reflect.DeepEqual or manual comparison
import "reflect"
reflect.DeepEqual(a, b)  // true
```

### 6. Deleting During Iteration

```go
m := map[string]int{"a": 1, "b": 2, "c": 3}

// Safe — delete during iteration is OK in Go
for k, v := range m {
    if v == 2 {
        delete(m, k)
    }
}
```

**Unlike slices, deleting from a map during iteration is safe.** The Go spec guarantees this.

### 7. Float Keys

```go
m := map[float64]int{}
m[1.0] = 1
m[1.0000000000000001] = 2  // Might overwrite the first entry!

// Fix: avoid float keys or use string representation
```

### 8. Not Checking Comma-Ok

```go
age := m["alice"]  // 0 — is this the real age or missing key?

// Fix:
age, ok := m["alice"]
if !ok {
    // Handle missing key
}
```

---

## Quick Reference

```go
// Creation
m := make(map[string]int)          // Empty
m := make(map[string]int, 100)     // Pre-allocated
m := map[string]int{"a": 1}       // Literal
var m map[string]int               // Nil

// Operations
m["key"] = value                   // Set
val := m["key"]                    // Get (zero value if missing)
val, ok := m["key"]               // Get with existence check
delete(m, "key")                   // Delete
len(m)                             // Length

// Iteration
for k, v := range m { ... }       // Key-value
for k := range m { ... }          // Keys only
for _, v := range m { ... }       // Values only

// Concurrency
var sm sync.Map
sm.Store(key, val)
val, ok := sm.Load(key)
sm.Delete(key)
sm.Range(func(k, v any) bool { ... })

// Utilities (Go 1.21+)
// maps is a stdlib package: import "maps"
maps.Clone(m)                      // Deep copy — returns a new map with same keys/values
maps.Equal(m1, m2)                 // Compare — true if both maps have same keys and values

// Utilities (Go 1.23+)
// These return iterators (a new Go 1.23 feature for lazy sequences)
maps.Keys(m)                       // Iterator of keys
maps.Values(m)                     // Iterator of values
// Use slices.Collect() to convert an iterator to a slice: slices.Collect(maps.Keys(m))
```

---

## 12. Production Best Practices [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

### Map with Pre-allocation

```go
// Pre-allocate if you know the size
func preallocateMap(n int) map[string]int {
    m := make(map[string]int, n) // capacity hint
    // Adding n items won't cause rehashing
    return m
}
```

### Cache Pattern

A simple `map` + `sync.RWMutex` cache works when you don't need eviction — data fits in memory and you control the key set. Add LRU eviction when: (1) your working set is larger than memory, (2) you want hot data to stay cached, or (3) you need a maximum cache size. The LRU adds `O(1)` overhead per access for the linked list.

```go
type Cache struct {
    mu    sync.RWMutex
    items map[string]interface{}
    ttl   time.Duration
}

func NewCache(ttl time.Duration) *Cache {
    return &Cache{
        items: make(map[string]interface{}),
        ttl:   ttl,
    }
}

func (c *Cache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    val, ok := c.items[key]
    return val, ok
}

func (c *Cache) Set(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = value
}

func (c *Cache) Delete(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.items, key)
}

// LRU Cache implementation
type LRUCache struct {
    mu       sync.Mutex
    capacity int
    items    map[string]*list.Element
    order    *list.List
}

type cacheItem struct {
    key   string
    value interface{}
}

func NewLRUCache(capacity int) *LRUCache {
    return &LRUCache{
        capacity: capacity,
        items:    make(map[string]*list.Element),
        order:    list.New(),
    }
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if elem, ok := c.items[key]; ok {
        c.order.MoveToFront(elem)
        return elem.Value.(*cacheItem).value, true
    }
    return nil, false
}

func (c *LRUCache) Set(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if elem, ok := c.items[key]; ok {
        elem.Value.(*cacheItem).value = value
        c.order.MoveToFront(elem)
        return
    }

    if len(c.items) >= c.capacity {
        // Remove least recently used
        oldest := c.order.Back()
        if oldest != nil {
            c.order.Remove(oldest)
            delete(c.items, oldest.Value.(*cacheItem).key)
        }
    }

    item := &cacheItem{key: key, value: value}
    elem := c.order.PushFront(item)
    c.items[key] = elem
}
```

### GroupBy Pattern

```go
func groupBy(items []Item, keyFn func(Item) string) map[string][]Item {
    groups := make(map[string][]Item)
    for _, item := range items {
        key := keyFn(item)
        groups[key] = append(groups[key], item)
    }
    return groups
}

// Usage
type User struct {
    Name string
    Role string
}

users := []User{
    {Name: "Alice", Role: "admin"},
    {Name: "Bob", Role: "user"},
    {Name: "Charlie", Role: "admin"},
}

byRole := groupBy(users, func(u User) string { return u.Role })
// byRole["admin"] = [{Alice admin} {Charlie admin}]
// byRole["user"] = [{Bob user}]
```

### Count Frequency

```go
func countFrequency(items []string) map[string]int {
    freq := make(map[string]int)
    for _, item := range items {
        freq[item]++
    }
    return freq
}

// With atomic for concurrent access
// sync/atomic provides low-level atomic operations — covered in Topic 16 (Mutex vs Channels).
import "sync/atomic"

type AtomicCounter struct {
    counters map[string]*atomic.Int64
    mu       sync.RWMutex
}

func (a *AtomicCounter) Inc(key string) int64 {
    a.mu.RLock()
    if c, ok := a.counters[key]; ok {
        a.mu.RUnlock()
        return c.Add(1)
    }
    a.mu.RUnlock()

    a.mu.Lock()
    if c, ok := a.counters[key]; ok {
        a.mu.Unlock()
        return c.Add(1)
    }
    c := &atomic.Int64{}
    c.Store(1)
    a.counters[key] = c
    a.mu.Unlock()
    return 1
}
```

---

## 13. Performance Considerations [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

### Hash Collision Attack Mitigation

```go
// In Go 1.18+, the runtime uses random hash seeds by default
// which makes it harder to craft inputs that cause collisions

// To check current behavior:
func checkMapStats() {
    m := make(map[string]int)
    
    // Insert many keys
    for i := 0; i < 10000; i++ {
        m[fmt.Sprintf("key-%d", i)] = i
    }
    
    // This is fast due to hash randomization
}
```

### Map vs Slice for Small N

```go
// For small N (< 50), linear search in slice may be faster
func linearSearch(slice []Item, key string) *Item {
    for i := range slice {
        if slice[i].ID == key {
            return &slice[i]
        }
    }
    return nil
}

// For larger N, map is faster
func mapSearch(m map[string]*Item, key string) *Item {
    return m[key]
}

// Benchmark to decide:
// go test -bench=. -benchmem
```

### Reducing Allocations

```go
// BAD: String concatenation in map key
m[key1+key2] = value // Creates new string each time

// GOOD: Use struct as key
type CompositeKey struct {
    Part1 string
    Part2 string
}

m[CompositeKey{Part1: "a", Part2: "b"}] = value

// GOOD: Use bytes.Buffer for building keys
func buildKey(parts ...string) string {
    b := new(strings.Builder)
    for _, p := range parts {
        b.WriteString(p)
        b.WriteByte(0) // separator
    }
    return b.String()
}
```

---

## 14. Testing Maps [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing the projects.

```go
func TestMapOperations(t *testing.T) {
    m := NewUserMap()
    
    // Test Set and Get
    m.Set("alice", User{Name: "Alice", Age: 30})
    u, ok := m.Get("alice")
    if !ok {
        t.Fatal("expected to find alice")
    }
    if u.Name != "Alice" {
        t.Errorf("expected Alice, got %s", u.Name)
    }
    
    // Test Delete
    m.Delete("alice")
    _, ok = m.Get("alice")
    if ok {
        t.Error("expected alice to be deleted")
    }
    
    // Test Len
    m.Set("bob", User{Name: "Bob", Age: 25})
    if m.Len() != 1 {
        t.Errorf("expected length 1, got %d", m.Len())
    }
}

func TestMapConcurrency(t *testing.T) {
    m := &sync.Map{}
    var wg sync.WaitGroup
    
    // Concurrent writes
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            m.Store(fmt.Sprintf("key-%d", n), n)
        }(i)
    }
    wg.Wait()
    
    // Concurrent reads
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            m.Load(fmt.Sprintf("key-%d", n))
        }(i)
    }
    wg.Wait()
}
```

---

## 15. Debugging Maps [INTERNALS]

> ⏭️ **First pass? Skip this section.** This covers low-level internals. Come back after completing Topics 1-10.

```go
import "runtime/debug"

func diagnoseMap() {
    // Print map info via reflection
    m := map[string]int{"a": 1, "b": 2}
    
    // Use printf with %p to see address
    fmt.Printf("map address: %p\n", m)
    
    // Check for nil map
    var nilMap map[string]int
    fmt.Printf("nil map: %v\n", nilMap) // prints: map[]
    fmt.Printf("nil map len: %d\n", len(nilMap)) // safe, prints 0
    
    // Writing to nil map PANICS!
    // nilMap["a"] = 1 // This would panic
}
```

---

## Exercises

### Exercise 1: Word Frequency Counter ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Given the sentence `"the quick brown fox jumps over the lazy dog the fox"`, build a `map[string]int` that counts how many times each word appears. Print each word and its count.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"strings"
)

func main() {
	sentence := "the quick brown fox jumps over the lazy dog the fox"
	freq := make(map[string]int)
	for _, word := range strings.Fields(sentence) {
		freq[word]++
	}
	for word, count := range freq {
		fmt.Printf("%-8s %d\n", word, count)
	}
}
```

</details>

### Exercise 2: Check Before Delete ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a map of student scores. Write a function that takes a key, uses the `value, ok` pattern to check if the key exists, prints the score if it does, and only then deletes the key. Demonstrate both the found and not-found cases.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func removeIfExists(m map[string]int, key string) {
	if score, ok := m[key]; ok {
		fmt.Printf("Found %s = %d, deleting...\n", key, score)
		delete(m, key)
	} else {
		fmt.Printf("%s not found\n", key)
	}
}

func main() {
	scores := map[string]int{"alice": 95, "bob": 82, "charlie": 78}
	removeIfExists(scores, "alice")
	removeIfExists(scores, "dave")
	fmt.Println("remaining:", scores)
}
```

</details>

### Exercise 3: Print Map Keys in Sorted Order ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a `map[string]int` with at least 5 entries. Iterate over the map, collect the keys into a slice, sort them with `sort.Strings`, then print the key-value pairs in alphabetical order.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"sort"
)

func main() {
	m := map[string]int{"banana": 2, "apple": 5, "cherry": 1, "date": 3, "elderberry": 4}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("%-12s %d\n", k, m[k])
	}
}
```

</details>

### Exercise 4: Nil Map Panic and Fix ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Demonstrate that writing to a nil map causes a panic (use `defer` + `recover` to catch it). Then fix the problem by initializing the map with `make()` and repeat the write successfully.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func main() {
	// Demonstrate nil map panic
	var m map[string]int
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("caught panic:", r)
			}
		}()
		m["key"] = 1 // panics
	}()

	// Fix: initialize with make
	m = make(map[string]int)
	m["key"] = 1
	fmt.Println("fixed map:", m) // map[key:1]
}
```

</details>

---

## Next: [Structs & Methods →](./05-structs-and-methods.md)
