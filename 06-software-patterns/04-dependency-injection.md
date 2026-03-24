# Dependency Injection

> Connect components without hardcoding. Swap implementations without changing code.

---

## The Problem

Hard dependencies make testing impossible and flexibility nonexistent:

```go
// Bad: Concrete dependency
type UserService struct {
    repo *PostgresUserRepo  // Can't swap, hard to test
}

func New() *UserService {
    return &UserService{
        repo: &PostgresUserRepo{},  // Hardcoded
    }
}
```

---

## Constructor-Based DI

Pass dependencies via constructor:

```go
// Good: Dependency via interface
type UserService struct {
    repo repository.UserRepository  // Interface, not concrete type
}

func New(repo repository.UserRepository) *UserService {
    return &UserService{repo: repo}
}
```

Now the caller decides what implementation to use:

```go
// Production: real database
repo := repository.NewPostgres("connection-string")
svc := service.New(repo)

// Testing: mock repository
mock := &mockUserRepository{}
svc := service.New(mock)
```

---

## Types of DI in Go

### 1. Constructor Injection (Recommended)

```go
func NewUserService(
    repo repository.UserRepository,
    logger Logger,
    metrics Metrics,
) *UserService {
    return &UserService{
        repo:    repo,
        logger:  logger,
        metrics: metrics,
    }
}
```

### 2. Functional Options (For Optional Dependencies)

```go
type UserService struct {
    repo     repository.UserRepository
    logger   Logger
    metrics  Metrics
    cache    Cache  // optional
}

type Option func(*UserService)

func WithLogger(l Logger) Option {
    return func(s *UserService) {
        s.logger = l
    }
}

func New(repo repository.UserRepository, opts ...Option) *UserService {
    svc := &UserService{
        repo:    repo,
        logger:  &defaultLogger{},
        metrics: &noOpMetrics{},
    }

    for _, opt := range opts {
        opt(svc)
    }

    return svc
}

// Usage:
svc := service.New(repo, service.WithLogger(zap.New()))
```

### 3. Setter Injection (Rare)

```go
type Service struct {
    logger Logger  // Can be set after construction
}

func (s *Service) SetLogger(l Logger) {
    s.logger = l
}
```

---

## Wiring in main.go

```go
// cmd/api/main.go
package main

import (
    "log"
    "net/http"

    "learning-go/internal/handler"
    "learning-go/internal/middleware"
    "learning-go/internal/repository"
    "learning-go/internal/service"
)

func main() {
    // 1. Create infrastructure
    logger := NewZapLogger()
    metrics := NewPrometheusMetrics()

    // 2. Create repositories
    userRepo := repository.NewInMemory()

    // 3. Create services (inject dependencies)
    userSvc := service.New(userRepo, logger, metrics)

    // 4. Create handlers (inject services)
    userHandler := handler.New(userSvc)

    // 5. Set up router with middleware
    mux := http.NewServeMux()
    mux.HandleFunc("POST /users", userHandler.Create)
    mux.HandleFunc("GET /users/{id}", userHandler.Get)

    // Wrap with middleware
    wrapped := middleware.Logging(logger)(
        middleware.Recovery(logger)(
            middleware.Metrics(metrics)(mux),
        ),
    )

    // 6. Start server
    log.Fatal(http.ListenAndServe(":8080", wrapped))
}
```

---

## DI Container (For Large Applications)

For complex apps, use a container to manage wiring:

```go
// internal/di/container.go
package di

type Container struct {
    userRepo    repository.UserRepository
    orderRepo   repository.OrderRepository
    userService *service.UserService
    orderService *service.OrderService
}

func New() *Container {
    return &Container{}
}

func (c *Container) UserRepository() repository.UserRepository {
    if c.userRepo == nil {
        c.userRepo = repository.NewInMemory()  // Or from config
    }
    return c.userRepo
}

func (c *Container) UserService() *service.UserService {
    if c.userService == nil {
        c.userService = service.New(
            c.UserRepository(),
            c.Logger(),
            c.Metrics(),
        )
    }
    return c.userService
}

// ... similar for other services
```

**Usage:**

```go
func main() {
    c := di.New()
    h := handler.New(c.UserService())
    // ...
}
```

---

## Testing with DI

```go
// internal/service/user_test.go
package service

import (
    "testing"
)

func TestCreateUser(t *testing.T) {
    // Create mock dependencies
    mockRepo := &mockUserRepository{}
    mockLogger := &mockLogger{}
    mockMetrics := &mockMetrics{}

    // Inject into service
    svc := New(mockRepo, mockLogger, mockMetrics)

    // Test
    user, err := svc.CreateUser("Alice", "alice@example.com")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Verify mock was called
    if !mockRepo.CreateCalled {
        t.Error("expected Create to be called on repo")
    }
}
```

---

## Visual: Dependency Flow

```
┌─────────────────────────────────────────────────────────────┐
│                        main.go                              │
│                                                              │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐ │
│   │  Handler    │◀────│  Service    │◀────│  Repository │ │
│   └─────────────┘     └─────────────┘     └─────────────┘ │
│         │                   │                   │           │
│         └───────────────────┴───────────────────┘           │
│                             │                               │
│                        main()                               │
│                   (wires everything)                       │
└─────────────────────────────────────────────────────────────┘
```

---

## Quick Reference

| Pattern | Use When |
|---------|----------|
| Constructor | Most cases, required dependencies |
| Options pattern | Optional dependencies, configs |
| Setter | Rare, late binding needed |
| Container | Large apps with many deps |

---

## Common Pitfalls

1. **No interfaces** - Use interfaces for testability
2. **Global state** - Avoid singletons, prefer DI
3. **Too many dependencies** - Consider if class does too much
4. **Circular dependencies** - A depends on B, B depends on A

---

## Next Steps

- [Clean Architecture](10-clean-architecture.md) - Enforce boundaries
- [Pub-Sub Design](11-pub-sub-design.md) - Decouple with events
- [Milestone Project](20-layered-http-service.md) - Put it all together