# Dependency Injection

> Connect components without hardcoding. Swap implementations without changing code.

---

## Table of Contents

1. [What is Dependency Injection?](#what-is-dependency-injection) `[PRODUCTION]`
2. [Why DI Matters](#why-di-matters) `[PRODUCTION]`
3. [Constructor Injection (Recommended)](#constructor-injection-recommended) `[PRODUCTION]`
4. [Functional Options Pattern](#functional-options-pattern) `[PRODUCTION]`
5. [DI Container (Large Applications)](#di-container-large-applications) `[PRODUCTION]`
6. [Testing with DI](#testing-with-di) `[PRODUCTION]`
7. [Dependency Graph](#dependency-graph) `[PRODUCTION]`
8. [Quick Reference](#quick-reference) `[PRODUCTION]`
9. [Common Pitfalls](#common-pitfalls) `[PRODUCTION]`

---

![Dependency Injection Overview](../assets/DI.png)

---

## What is Dependency Injection?

Dependency Injection (DI) means **passing dependencies to a component** instead of the component creating them internally.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                     WITHOUT DI (Hard Dependencies)                        │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   type UserService struct {                                               │
  │       repo *PostgresUserRepo   ◄── Concrete type (hard dependency)       │
  │   }                                                                       │
  │                                                                           │
  │   func New() *UserService {                                               │
  │       return &UserService{                                                │
  │           repo: &PostgresUserRepo{...},  ◄── Created INSIDE!             │
  │       }                                                                   │
  │   }                                                                       │
  │                                                                           │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  ✗ Can't test without real database                              │  │
  │   │  ✗ Can't swap to different DB                                    │  │
  │   │  ✗ Hard dependencies everywhere                                  │  │
  │   └───────────────────────────────────────────────────────────────────┘  │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────────────────────────┐
  │                      WITH DI (Injected Dependencies)                      │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   type UserService struct {                                               │
  │       repo UserRepository   ◄── Interface (not concrete)                 │
  │   }                                                                       │
  │                                                                           │
  │   func New(repo UserRepository) *UserService {                           │
  │       return &UserService{                                                │
  │           repo: repo,  ◄── Passed in from OUTSIDE!                       │
  │       }                                                                   │
  │   }                                                                       │
  │                                                                           │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  ✓ Easy to test with mocks                                       │  │
  │   │  ✓ Can swap implementations freely                               │  │
  │   │  ✓ Clear dependency graph                                        │  │
  │   └───────────────────────────────────────────────────────────────────┘  │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Why DI Matters

### 1. Testing

Without DI, testing requires real dependencies:

```go
// BAD: Hard to test
type UserService struct {
    db *sql.DB  // Concrete dependency
}

func New() *UserService {
    db, _ := sql.Open("postgres", "...")
    return &UserService{db: db}
}

func TestCreateUser(t *testing.T) {
    // Need a REAL database to test!
    svc := New()
    // ... test with real DB (slow, flaky)
}
```

With DI, testing is fast:

```go
// GOOD: Easy to test
type UserService struct {
    repo UserRepository  // Interface
}

func New(repo UserRepository) *UserService {
    return &UserService{repo: repo}
}

func TestCreateUser(t *testing.T) {
    // Use a mock — no database needed
    mock := &mockUserRepo{}
    svc := New(mock)
    
    user, err := svc.CreateUser("Alice", "alice@example.com")
    // ... fast, reliable test
}
```

### 2. Flexibility

Swap implementations based on environment:

```go
// Production
repo := repository.NewPostgres(os.Getenv("DB_URL"))

// Testing
repo := repository.NewInMemory()

// Development
repo := repository.NewSQLite("dev.db")

// All three implement UserRepository interface
```

---

## Constructor Injection (Recommended)

Pass ALL dependencies via constructor. This is the **standard approach in Go**.

```go
// internal/service/user.go
package service

type UserService struct {
    repo     repository.UserRepository
    logger   Logger
    metrics  Metrics
    eventBus *events.EventBus
}

// NewUserService creates a user service with all required dependencies
func NewUserService(
    repo repository.UserRepository,
    logger Logger,
    metrics Metrics,
    eb *events.EventBus,
) *UserService {
    return &UserService{
        repo:     repo,
        logger:   logger,
        metrics:  metrics,
        eventBus: eb,
    }
}
```

### The Wiring in main.go

The `main.go` file is responsible for **creating and connecting all dependencies**:

```go
// cmd/api/main.go
package main

import (
    "database/sql"
    "log"
    "net/http"
    "os"

    "myapp/internal/events"
    "myapp/internal/handler"
    "myapp/internal/middleware"
    "myapp/internal/repository"
    "myapp/internal/service"
)

func main() {
    // ═══════════════════════════════════════════
    // Layer 1: Infrastructure (external services)
    // ═══════════════════════════════════════════
    
    logger := NewLogger()           // returns *zap.Logger
    metrics := NewMetrics()         // returns *Metrics
    db := connectDatabase()         // returns *sql.DB
    eventBus := events.New()        // returns *events.EventBus
    
    // ═══════════════════════════════════════════
    // Layer 2: Repositories (data access)
    // ═══════════════════════════════════════════
    
    userRepo := repository.NewUserPostgres(db)
    orderRepo := repository.NewOrderPostgres(db)
    
    // ═══════════════════════════════════════════
    // Layer 3: Services (business logic)
    // ═══════════════════════════════════════════
    
    userService := service.NewUserService(
        userRepo,
        logger,
        metrics,
        eventBus,
    )
    
    orderService := service.NewOrderService(
        orderRepo,
        userRepo,       // OrderService needs to look up users
        logger,
        metrics,
        eventBus,
    )
    
    // ═══════════════════════════════════════════
    // Layer 4: Handlers (HTTP)
    // ═══════════════════════════════════════════
    
    userHandler := handler.NewUserHandler(userService)
    orderHandler := handler.NewOrderHandler(orderService)
    
    // ═══════════════════════════════════════════
    // Layer 5: Router + Middleware
    // ═══════════════════════════════════════════
    
    mux := http.NewServeMux()
    mux.HandleFunc("POST /users", userHandler.Create)
    mux.HandleFunc("GET /users/{id}", userHandler.Get)
    mux.HandleFunc("POST /orders", orderHandler.Create)
    
    // ═══════════════════════════════════════════
    // Layer 6: Start server
    // ═══════════════════════════════════════════
    
    server := middleware.Chain(
        mux,
        middleware.Logger(logger),
        middleware.Recovery(logger),
        middleware.Metrics(metrics),
    )
    
    log.Fatal(http.ListenAndServe(":8080", server))
}
```

---

## Functional Options Pattern

For services with **optional dependencies**, use functional options:

```go
// internal/service/user.go
package service

type UserService struct {
    repo    repository.UserRepository
    logger  Logger
    metrics Metrics
    cache   Cache  // Optional
}

// Option is a function that configures UserService
type Option func(*UserService)

// WithLogger sets a custom logger
func WithLogger(l Logger) Option {
    return func(s *UserService) {
        s.logger = l
    }
}

// WithMetrics sets custom metrics
func WithMetrics(m Metrics) Option {
    return func(s *UserService) {
        s.metrics = m
    }
}

// WithCache sets a cache layer
func WithCache(c Cache) Option {
    return func(s *UserService) {
        s.cache = c
    }
}

// NewUserService creates a service with required + optional dependencies
func NewUserService(repo repository.UserRepository, opts ...Option) *UserService {
    // Default values
    svc := &UserService{
        repo:    repo,
        logger:  &noopLogger{},
        metrics: &noopMetrics{},
    }
    
    // Apply options
    for _, opt := range opts {
        opt(svc)
    }
    
    return svc
}
```

### Usage

```go
// Minimal (required dependencies only)
svc := service.NewUserService(repo)

// With optional dependencies
svc := service.NewUserService(repo,
    service.WithLogger(zap.New()),
    service.WithMetrics(prometheus.New()),
    service.WithCache(redis.New()),
)
```

---

## DI Container (Large Applications)

For apps with 10+ services, use a **container** to manage wiring:

```go
// internal/di/container.go
package di

import (
    "database/sql"
    "myapp/internal/events"
    "myapp/internal/repository"
    "myapp/internal/service"
)

// Container manages all dependencies
type Container struct {
    db        *sql.DB
    eventBus  *events.EventBus
    
    // Repositories (lazy initialized)
    userRepo  repository.UserRepository
    orderRepo repository.OrderRepository
    
    // Services (lazy initialized)
    userService  *service.UserService
    orderService *service.OrderService
}

// NewContainer creates a new container
func NewContainer(db *sql.DB, eb *events.EventBus) *Container {
    return &Container{
        db:       db,
        eventBus: eb,
    }
}

// UserRepository returns the user repository (singleton)
func (c *Container) UserRepository() repository.UserRepository {
    if c.userRepo == nil {
        c.userRepo = repository.NewUserPostgres(c.db)
    }
    return c.userRepo
}

// OrderRepository returns the order repository (singleton)
func (c *Container) OrderRepository() repository.OrderRepository {
    if c.orderRepo == nil {
        c.orderRepo = repository.NewOrderPostgres(c.db)
    }
    return c.orderRepo
}

// UserService returns the user service (singleton)
func (c *Container) UserService() *service.UserService {
    if c.userService == nil {
        c.userService = service.NewUserService(
            c.UserRepository(),
            c.eventBus,
        )
    }
    return c.userService
}

// OrderService returns the order service (singleton)
func (c *Container) OrderService() *service.OrderService {
    if c.orderService == nil {
        c.orderService = service.NewOrderService(
            c.OrderRepository(),
            c.UserRepository(),  // Inject user repo too
            c.eventBus,
        )
    }
    return c.orderService
}
```

### Usage

```go
func main() {
    db := connectDatabase()
    eb := events.New()
    
    c := di.NewContainer(db, eb)
    
    mux := http.NewServeMux()
    mux.HandleFunc("POST /users", c.UserHandler().Create)
    mux.HandleFunc("GET /users/{id}", c.UserHandler().Get)
    // ...
}
```

---

## Testing with DI

DI makes testing **fast and isolated**:

```go
// internal/service/user_test.go
package service_test

import (
    "testing"
    "myapp/internal/service"
)

// Mock implementations
type mockUserRepo struct {
    users map[string]*model.User
}

func (m *mockUserRepo) Create(user *model.User) error {
    m.users[user.ID] = user
    return nil
}

func (m *mockUserRepo) GetByID(id string) (*model.User, error) {
    user, ok := m.users[id]
    if !ok {
        return nil, repository.ErrNotFound
    }
    return user, nil
}

// ... other methods

type mockLogger struct {
    entries []string
}

func (m *mockLogger) Info(msg string) {
    m.entries = append(m.entries, msg)
}

func (m *mockLogger) Error(msg string) {
    m.entries = append(m.entries, msg)
}

// Tests
func TestCreateUser_Success(t *testing.T) {
    // Setup with mocks
    mockRepo := &mockUserRepo{users: make(map[string]*model.User)}
    mockLog := &mockLogger{}
    mockMetrics := &mockMetrics{}
    mockBus := events.New()
    
    svc := service.NewUserService(mockRepo, mockLog, mockMetrics, mockBus)
    
    // Execute
    user, err := svc.CreateUser("Alice", "alice@example.com")
    
    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if user.Name != "Alice" {
        t.Errorf("expected Alice, got %s", user.Name)
    }
    
    // Verify side effects
    if len(mockLog.entries) == 0 {
        t.Error("expected logger to be called")
    }
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
    mockRepo := &mockUserRepo{users: make(map[string]*model.User)}
    mockLog := &mockLogger{}
    mockMetrics := &mockMetrics{}
    mockBus := events.New()
    
    svc := service.NewUserService(mockRepo, mockLog, mockMetrics, mockBus)
    
    // Create first user
    _, err := svc.CreateUser("Alice", "alice@example.com")
    if err != nil {
        t.Fatal(err)
    }
    
    // Try to create duplicate
    _, err = svc.CreateUser("Bob", "alice@example.com")
    if err == nil {
        t.Fatal("expected error for duplicate email")
    }
}
```

---

## Dependency Graph

Visualize your dependency structure:

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                         DEPENDENCY GRAPH                                  │
  │                    (arrows show "depends on" direction)                   │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │                        ┌──────────────┐                                   │
  │                        │   main.go    │                                   │
  │                        │  (wiring)    │                                   │
  │                        └──────┬───────┘                                   │
  │                               │                                            │
  │              ┌────────────────┼────────────────┐                          │
  │              ▼                ▼                ▼                          │
  │       ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                    │
  │       │  UserHandler│ │ OrderHandler│ │ AuthHandler │                    │
  │       └──────┬──────┘ └──────┬──────┘ └──────┬──────┘                    │
  │              │                │                │                          │
  │              ▼                ▼                ▼                          │
  │       ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                    │
  │       │ UserService │ │OrderService │ │AuthService  │                    │
  │       └──────┬──────┘ └──────┬──────┘ └──────┬──────┘                    │
  │              │                │                │                          │
  │              ▼                ▼                ▼                          │
  │       ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                    │
  │       │ UserRepo    │ │ OrderRepo   │ │ SessionRepo │                    │
  │       │ (interface) │ │ (interface) │ │ (interface) │                    │
  │       └─────────────┘ └─────────────┘ └─────────────┘                    │
  │                                                                           │
  │   main.go wires everything together — that's the ONLY place that        │
  │   knows about concrete implementations.                                   │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Quick Reference

| Pattern | When | Pros | Cons |
|---------|------|------|------|
| Constructor | Always | Simple, explicit | Verbose with many deps |
| Options | Optional deps | Flexible | More complex |
| Setter | Rare | Late binding | Can forget to set |
| Container | 10+ services | Organized | Extra abstraction |

---

## Common Pitfalls

1. **Concrete types in service** — Use interfaces for testability
2. **Global state** — Avoid `var db = sql.Open(...)` in globals
3. **Too many dependencies** — Service might be doing too much
4. **Circular deps** — A→B→A means bad design
5. **Not wiring in main** — Scattered `init()` functions

---

## Next Steps

- [Clean Architecture](05-clean-architecture.md) — Enforce layer boundaries
- [Pub-Sub Design](06-pub-sub-design.md) — Decouple with events
- [Milestone Project](../projects/20-layered-http-service.md) — Build it end-to-end