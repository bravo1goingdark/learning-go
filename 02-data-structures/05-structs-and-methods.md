# 5. Structs & Methods — Complete Deep Dive

> **Goal:** Master Go's approach to data structures. No classes. No inheritance. Composition over inheritance.

---

## Table of Contents

1. [Struct Basics](#1-struct-basics)
2. [Struct Construction](#2-struct-construction)
3. [Methods](#3-methods)
4. [Value vs Pointer Receivers](#4-value-vs-pointer-receivers)
5. [Method Sets & Interface Satisfaction](#5-method-sets--interface-satisfaction)
6. [Embedding (Composition)](#6-embedding-composition)
7. [Struct Tags](#7-struct-tags)
8. [Comparable Structs](#8-comparable-structs)
9. [Empty Struct](#9-empty-struct)
10. [Struct Memory Layout](#10-struct-memory-layout)
11. [Production Patterns](#11-production-patterns)
12. [Common Pitfalls](#12-common-pitfalls)

---

## 1. Struct Basics

### Definition

```go
type User struct {
    Name    string
    Email   string
    Age     int
    Active  bool
    created time.Time  // unexported (lowercase) — private to package
}
```

### Rules

- **Exported fields** (uppercase): accessible from other packages
- **Unexported fields** (lowercase): only accessible within the same package
- **Zero value:** all fields set to their zero values
- **No constructors** — use factory functions or struct literals

### Field Types

```go
type Order struct {
    ID        int
    Items     []OrderItem          // Slice of structs
    Metadata  map[string]string    // Map
    Customer  *Customer            // Pointer to struct
    CreatedAt time.Time
    Processed bool
}

type OrderItem struct {
    ProductID int
    Quantity  int
    Price     float64
}
```

---

## 2. Struct Construction

### Method 1: Zero Value

```go
var u User
// u.Name == ""
// u.Email == ""
// u.Age == 0
// u.Active == false
// u.created == time.Time{}
```

### Method 2: Positional Fields

```go
u := User{"Alice", "alice@example.com", 30, true, time.Now()}
```

**Don't use this.** If fields are added/reordered, code breaks silently.

### Method 3: Named Fields (Preferred)

```go
u := User{
    Name:    "Alice",
    Email:   "alice@example.com",
    Age:     30,
    Active:  true,
    created: time.Now(),
}
```

### Method 4: Factory Function

```go
func NewUser(name, email string, age int) *User {
    return &User{
        Name:    name,
        Email:   email,
        Age:     age,
        Active:  true,
        created: time.Now(),
    }
}

u := NewUser("Alice", "alice@example.com", 30)
```

**Use factory functions when:**
- Initialization logic is complex
- You need validation
- You want to ensure unexported fields are set
- You want to enforce invariants

### Method 5: Builder Pattern

```go
type UserBuilder struct {
    user User
}

func NewUserBuilder() *UserBuilder {
    return &UserBuilder{}
}

func (b *UserBuilder) Name(name string) *UserBuilder {
    b.user.Name = name
    return b
}

func (b *UserBuilder) Email(email string) *UserBuilder {
    b.user.Email = email
    return b
}

func (b *UserBuilder) Age(age int) *UserBuilder {
    b.user.Age = age
    return b
}

func (b *UserBuilder) Build() *User {
    b.user.created = time.Now()
    return &b.user
}

// Usage
u := NewUserBuilder().
    Name("Alice").
    Email("alice@example.com").
    Age(30).
    Build()
```

### Method 6: Functional Options

```go
type User struct {
    Name    string
    Email   string
    Age     int
    Active  bool
    Timeout time.Duration
}

type UserOption func(*User)

func WithAge(age int) UserOption {
    return func(u *User) { u.Age = age }
}

func WithTimeout(d time.Duration) UserOption {
    return func(u *User) { u.Timeout = d }
}

func WithActive(active bool) UserOption {
    return func(u *User) { u.Active = active }
}

func NewUser(name, email string, opts ...UserOption) *User {
    u := &User{
        Name:    name,
        Email:   email,
        Age:     18,           // default
        Active:  true,         // default
        Timeout: 30 * time.Second, // default
    }
    for _, opt := range opts {
        opt(u)
    }
    return u
}

// Usage
u := NewUser("Alice", "alice@example.com",
    WithAge(30),
    WithTimeout(60*time.Second),
)
```

---

## 3. Methods

### Method Syntax

```go
type Rect struct {
    Width, Height float64
}

// Value receiver
func (r Rect) Area() float64 {
    return r.Width * r.Height
}

// Value receiver
func (r Rect) Perimeter() float64 {
    return 2 * (r.Width + r.Height)
}

func main() {
    r := Rect{Width: 10, Height: 5}
    fmt.Println(r.Area())      // 50
    fmt.Println(r.Perimeter()) // 30
}
```

### The Receiver

```go
// func (receiver Type) MethodName(params) ReturnType
func (r Rect) Area() float64 {
//  ^receiver
    return r.Width * r.Height
}
```

The receiver is the "self" or "this" equivalent, but **explicitly named** and **explicitly typed**.

### Methods on Any Named Type

```go
type Celsius float64

func (c Celsius) Fahrenheit() Fahrenheit {
    return Fahrenheit(c*9/5 + 32)
}

func (c Celsius) Kelvin() Kelvin {
    return Kelvin(c + 273.15)
}

type Fahrenheit float64
type Kelvin float64

// Usage
temp := Celsius(100)
fmt.Println(temp.Fahrenheit())  // 212
fmt.Println(temp.Kelvin())      // 373.15
```

---

## 4. Value vs Pointer Receivers

### Value Receiver — Modifies Copy

```go
type Counter struct {
    count int
}

func (c Counter) Increment() {
    c.count++  // Modifies the COPY
}

func main() {
    c := Counter{}
    c.Increment()
    c.Increment()
    c.Increment()
    fmt.Println(c.count)  // 0 — never changed!
}
```

### Pointer Receiver — Modifies Original

```go
type Counter struct {
    count int
}

func (c *Counter) Increment() {
    c.count++  // Modifies the ORIGINAL
}

func main() {
    c := &Counter{}
    c.Increment()
    c.Increment()
    c.Increment()
    fmt.Println(c.count)  // 3
}
```

### When to Use Each

| Use Pointer Receiver | Use Value Receiver |
|---------------------|-------------------|
| Method modifies receiver | Method doesn't modify receiver |
| Struct is large (>64 bytes) | Struct is small |
| Consistency — if ANY method needs pointer, ALL should use pointer | Immutable types |
| `sync.Mutex` fields (must not copy) | Simple getters |

### The Consistency Rule

```go
// WRONG — inconsistent receivers
func (u User) Name() string       { return u.name }
func (u *User) SetName(n string)  { u.name = n }

// RIGHT — consistent pointer receivers
func (u *User) Name() string      { return u.name }
func (u *User) SetName(n string)  { u.name = n }

// RIGHT — consistent value receivers (if truly immutable)
func (u User) Name() string       { return u.name }
func (u User) Email() string      { return u.email }
```

**Production rule:** Use pointer receivers unless you have a specific reason not to. Most production structs use pointer receivers.

---

## 5. Method Sets & Interface Satisfaction

### The Method Set Rules

| Type | Method Set Contains |
|------|-------------------|
| `T` | All methods with receiver `T` |
| `*T` | All methods with receiver `T` OR `*T` |

This matters for **interface satisfaction**:

```go
type Namer interface {
    Name() string
}

type User struct {
    name string
}

// Value receiver method
func (u User) Name() string {
    return u.name
}

func main() {
    u := User{name: "Alice"}
    
    var n Namer = u    // ✅ User satisfies Namer
    var n Namer = &u   // ✅ *User also satisfies Namer (has all methods of User)
}
```

```go
type SetNamer interface {
    SetName(string)
}

type User struct {
    name string
}

// Pointer receiver method
func (u *User) SetName(n string) {
    u.name = n
}

func main() {
    u := User{name: "Alice"}
    
    var s SetNamer = u   // ❌ User does NOT satisfy SetNamer
                         //    User's method set only has value receiver methods
    var s SetNamer = &u  // ✅ *User satisfies SetNamer
}
```

### Why This Matters

```go
// If you use pointer receivers, pass pointer to functions expecting interfaces
func process(w io.Writer) {
    // ...
}

type MyWriter struct{}

func (w *MyWriter) Write(p []byte) (int, error) {
    return len(p), nil
}

func main() {
    w := MyWriter{}
    process(w)   // ❌ MyWriter doesn't satisfy io.Writer
    process(&w)  // ✅ *MyWriter satisfies io.Writer
}
```

---

## 6. Embedding (Composition)

Go has no inheritance. Instead, use **embedding** for composition.

### Basic Embedding

```go
type Point struct {
    X, Y float64
}

type Circle struct {
    Point       // Embedded struct (anonymous field)
    Radius float64
}

func main() {
    c := Circle{
        Point:  Point{X: 1, Y: 2},
        Radius: 5,
    }

    // Promoted fields — accessible directly
    fmt.Println(c.X)     // 1 (same as c.Point.X)
    fmt.Println(c.Y)     // 2 (same as c.Point.Y)
    fmt.Println(c.Radius) // 5
}
```

### Promoted Methods

```go
func (p Point) DistanceTo(other Point) float64 {
    dx := p.X - other.X
    dy := p.Y - other.Y
    return math.Sqrt(dx*dx + dy*dy)
}

func main() {
    c1 := Circle{Point: Point{0, 0}, Radius: 5}
    c2 := Circle{Point: Point{3, 4}, Radius: 3}

    // Promoted method — c1.Point.DistanceTo(c2.Point)
    fmt.Println(c1.DistanceTo(c2.Point))  // 5
}
```

### Multiple Embedding

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type ReadWriter struct {
    Reader  // embedded interface
    Writer  // embedded interface
}

// ReadWriter has both Read() and Write() methods
```

### Overriding Embedded Methods

```go
type Animal struct {
    Name string
}

func (a Animal) Speak() string {
    return "..."
}

type Dog struct {
    Animal  // Embedding
}

// Override the embedded method
func (d Dog) Speak() string {
    return "Woof!"
}

func main() {
    d := Dog{Animal: Animal{Name: "Buddy"}}
    fmt.Println(d.Speak())    // "Woof!" — calls Dog.Speak()
    fmt.Println(d.Name)       // "Buddy" — promoted from Animal
    
    // Access embedded method explicitly
    fmt.Println(d.Animal.Speak())  // "..."
}
```

### Embedding vs Inheritance

```go
// Java (inheritance)
class Dog extends Animal { }

// Go (composition)
type Dog struct {
    Animal  // embeds Animal
    Breed   string
}

// Dog IS NOT an Animal in the type system
// Dog HAS an Animal — it's composition
```

**Key difference:** Go embedding does NOT create an "is-a" relationship. A `Dog` is not an `Animal` in the type system. You must use explicit type conversion or interfaces.

### Production Pattern: Service with Embedded Dependencies

```go
type Logger struct{}

func (l *Logger) Log(msg string) {
    fmt.Println(msg)
}

type UserService struct {
    Logger       // Embedded — promotes Log() method
    repo *UserRepository
}

func (s *UserService) Create(user User) error {
    s.Log("Creating user: " + user.Name)  // Promoted method
    return s.repo.Save(user)
}
```

---

## 7. Struct Tags

Struct tags are metadata attached to struct fields. They're read at runtime via reflection.

### Syntax

```go
type User struct {
    Name  string `json:"name" db:"user_name" validate:"required"`
    Email string `json:"email,omitempty" db:"email" validate:"required,email"`
    Age   int    `json:"age,omitempty" db:"age"`
}
```

### Common Tag Keys

| Package | Key | Purpose |
|---------|-----|---------|
| `encoding/json` | `json` | JSON marshaling |
| `encoding/xml` | `xml` | XML marshaling |
| `database/sql` | `db` | SQL column mapping (via sqlx) |
| `go.mongodb.org` | `bson` | MongoDB BSON |
| `github.com/go-playground/validator` | `validate` | Validation rules |
| `github.com/fatih/structs` | `structs` | Struct manipulation |
| `github.com/mitchellh/mapstructure` | `mapstructure` | Map to struct |

### JSON Tags

```go
type Response struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email,omitempty"`   // Omit if zero value
    Password  string    `json:"-"`                  // Never serialize
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// JSON output:
// {"id":1,"name":"Alice","email":"alice@example.com","created_at":"2024-01-01T00:00:00Z"}
// Password is excluded, UpdatedAt is omitted if zero
```

### Reading Tags with Reflection

```go
import (
    "reflect"
    "strings"
)

func getFieldTag(obj any, fieldName, tagKey string) string {
    t := reflect.TypeOf(obj)
    field, ok := t.FieldByName(fieldName)
    if !ok {
        return ""
    }
    return field.Tag.Get(tagKey)
}

func main() {
    u := User{Name: "Alice", Email: "alice@example.com"}
    
    fmt.Println(getFieldTag(u, "Name", "json"))      // "name"
    fmt.Println(getFieldTag(u, "Email", "validate"))  // "required,email"
}
```

### Custom Tag Parsing

```go
func parseValidateTag(tag string) map[string]string {
    result := make(map[string]string)
    for _, part := range strings.Split(tag, ",") {
        if kv := strings.SplitN(part, "=", 2); len(kv) == 2 {
            result[kv[0]] = kv[1]
        } else {
            result[part] = "true"
        }
    }
    return result
}

// "required,min=3,max=50" → {"required": "true", "min": "3", "max": "50"}
```

---

## 8. Comparable Structs

Structs are comparable if all their fields are comparable.

```go
type Point struct {
    X, Y int
}

p1 := Point{1, 2}
p2 := Point{1, 2}
p3 := Point{2, 3}

fmt.Println(p1 == p2)  // true
fmt.Println(p1 == p3)  // false
```

### Non-Comparable Structs

```go
type Data struct {
    Values []int  // Slices are not comparable
}

d1 := Data{Values: []int{1, 2}}
d2 := Data{Values: []int{1, 2}}
// d1 == d2  // COMPILE ERROR: invalid operation

// Fix: use reflect.DeepEqual
import "reflect"
reflect.DeepEqual(d1, d2)  // true
```

### Structs as Map Keys

```go
type CacheKey struct {
    UserID int
    Path   string
}

cache := map[CacheKey]string{}
cache[CacheKey{UserID: 1, Path: "/api/users"}] = "cached-response"
```

---

## 9. Empty Struct

The empty struct `struct{}` has zero size.

```go
var s struct{}
fmt.Println(unsafe.Sizeof(s))  // 0
```

### Uses

#### As a Set Value

```go
set := map[string]struct{}{}
set["key"] = struct{}{}
if _, ok := set["key"]; ok {
    // ...
}
```

#### As a Channel Signal

```go
done := make(chan struct{})

go func() {
    // Do work...
    close(done)  // Signal completion
}()

<-done  // Wait for completion
```

#### As a Method Receiver (No State)

```go
type Validator struct{}

func (Validator) Validate(data []byte) error {
    // Stateless validation
    return nil
}
```

---

## 10. Struct Memory Layout

### Field Ordering Matters

```go
// WASTES memory (padding)
type Bad struct {
    a bool    // 1 byte + 7 padding
    b int64   // 8 bytes
    c bool    // 1 byte + 7 padding
    d int64   // 8 bytes
}
// Total: 32 bytes

// OPTIMAL (fields ordered by size)
type Good struct {
    b int64   // 8 bytes
    d int64   // 8 bytes
    a bool    // 1 byte
    c bool    // 1 byte + 6 padding
}
// Total: 24 bytes
```

### How to Check

```go
import "unsafe"

fmt.Println(unsafe.Sizeof(Bad{}))   // 32
fmt.Println(unsafe.Sizeof(Good{}))  // 24
```

### Memory Layout Visualization

```
Bad struct (32 bytes):
+--------+--------+--------+--------+
| a (1B) | pad(7) | b (8B)          |
+--------+--------+--------+--------+
| c (1B) | pad(7) | d (8B)          |
+--------+--------+--------+--------+

Good struct (24 bytes):
+--------+--------+--------+--------+
| b (8B)                        | d  |
+--------+--------+--------+--------+
| d (cont)      | a (1B)| c (1B)|pad |
+--------+--------+--------+--------+
```

**Rule:** Order fields from largest to smallest for optimal memory layout.

---

## 11. Production Patterns

### Domain Model Pattern

```go
// Internal domain model — unexported fields, methods for behavior
type User struct {
    id        UserID
    email     Email
    name      string
    createdAt time.Time
    updatedAt time.Time
}

func NewUser(email Email, name string) (*User, error) {
    if err := email.Validate(); err != nil {
        return nil, fmt.Errorf("invalid email: %w", err)
    }
    if name == "" {
        return nil, errors.New("name is required")
    }
    now := time.Now()
    return &User{
        id:        NewUserID(),
        email:     email,
        name:      name,
        createdAt: now,
        updatedAt: now,
    }, nil
}

func (u *User) Rename(name string) error {
    if name == "" {
        return errors.New("name cannot be empty")
    }
    u.name = name
    u.updatedAt = time.Now()
    return nil
}

func (u *User) ID() UserID       { return u.id }
func (u *User) Email() Email     { return u.email }
func (u *User) Name() string     { return u.name }
```

### Request/Response DTOs

```go
// API layer — exported fields with JSON tags
type CreateUserRequest struct {
    Email string `json:"email" validate:"required,email"`
    Name  string `json:"name" validate:"required,min=1,max=100"`
}

type UserResponse struct {
    ID        string    `json:"id"`
    Email     string    `json:"email"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

func ToUserResponse(u *User) UserResponse {
    return UserResponse{
        ID:        u.ID().String(),
        Email:     u.Email().String(),
        Name:      u.Name(),
        CreatedAt: u.createdAt,
    }
}
```

### Configuration Pattern

```go
type Config struct {
    Server ServerConfig `json:"server"`
    DB     DBConfig     `json:"db"`
    Redis  RedisConfig  `json:"redis"`
}

type ServerConfig struct {
    Host         string        `json:"host" default:"localhost"`
    Port         int           `json:"port" default:"8080"`
    ReadTimeout  time.Duration `json:"read_timeout" default:"30s"`
    WriteTimeout time.Duration `json:"write_timeout" default:"30s"`
}

// Load from environment, file, or defaults
func LoadConfig(path string) (*Config, error) {
    // ...
}
```

---

## 12. Common Pitfalls

### 1. Copying Mutex Fields

```go
type SafeCounter struct {
    mu    sync.Mutex
    count int
}

func (c *SafeCounter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}

// BUG — copies the mutex
func process(c SafeCounter) {
    c.Increment()  // Locks a COPY of the mutex — deadlock risk
}

// FIX — pass pointer
func process(c *SafeCounter) {
    c.Increment()
}
```

### 2. Embedding vs Field

```go
// Embedding — promotes fields and methods
type Logger struct{}
type Service struct {
    Logger  // embedded — Service has Log() method
}

// Field — no promotion
type Service struct {
    logger Logger  // must use s.logger.Log()
}
```

### 3. Method on Wrong Type

```go
type Users []User

// This method is on the SLICE type
func (u Users) Names() []string {
    names := make([]string, len(u))
    for i, user := range u {
        names[i] = user.Name
    }
    return names
}

// Can't call this on a []User directly — need type conversion
users := []User{{Name: "Alice"}, {Name: "Bob"}}
// users.Names()  // COMPILE ERROR
Users(users).Names()  // OK — explicit conversion
```

### 4. Struct Comparison with Float Fields

```go
type Point struct {
    X, Y float64
}

p1 := Point{0.1 + 0.2, 1.0}
p2 := Point{0.3, 1.0}
fmt.Println(p1 == p2)  // false! — float precision issue
```

### 5. Unexported Fields and JSON

```go
type User struct {
    name  string  // lowercase — won't appear in JSON!
    Email string
}

u := User{name: "Alice", Email: "alice@example.com"}
data, _ := json.Marshal(u)
fmt.Println(string(data))  // {"Email":"alice@example.com"}
// name is missing!
```

### 6. Forgetting to Initialize Embedded Struct

```go
type Logger struct {
    Level int
}

type Service struct {
    Logger
}

func main() {
    var s Service
    s.Level = 1  // Works — embedded struct fields are promoted
    s.Logger.Level = 1  // Same thing — explicit
}
```

---

## Quick Reference

```go
// Definition
type User struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// Construction
u := User{Name: "Alice", Email: "alice@example.com"}  // Named (preferred)
u := User{"Alice", "alice@example.com"}               // Positional (avoid)
u := NewUser("Alice", "alice@example.com")            // Factory

// Methods
func (u User) Name() string   { return u.name }  // Value receiver
func (u *User) SetName(n string) { u.name = n }   // Pointer receiver

// Embedding
type Admin struct {
    User
    Level int
}
a := Admin{User: User{Name: "Alice"}, Level: 1}
a.Name()  // Promoted from User

// Tags
type User struct {
    Name string `json:"name,omitempty" validate:"required"`
}
```

---

## Next: [Pointers →](./06-pointers.md)
