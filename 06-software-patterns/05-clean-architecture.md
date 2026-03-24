# Clean Architecture

> Organize code so business logic is independent of external concerns.

---

## The Concept

Clean Architecture separates code into layers with clear dependencies:

```
┌─────────────────────────────────────────────────────────────┐
│                    Presentation                            │
│              (Handlers, CLI, UI)                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Application                              │
│              (Use Cases, Services)                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Domain                                │
│           (Entities, Business Rules)                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Infrastructure                            │
│         (Repositories, External APIs, DB)                  │
└─────────────────────────────────────────────────────────────┘
```

**Key Rule:** Dependencies point inward. Inner layers know nothing about outer layers.

---

## Layers in Go

```
internal/
├── domain/           # Innermost - pure business logic
│   ├── entity/       # Core data structures
│   └── rule/         # Business rules
├── application/     # Use cases, orchestration
│   └── usecase/     # Business operations
├── infrastructure/  # Outermost - implementation details
│   ├── repository/  # Database implementations
│   └── external/    # API clients
└── presentation/    # HTTP handlers
    └── handler/
```

---

## Domain Layer (Innermost)

Contains business entities and rules - no external dependencies:

```go
// internal/domain/entity/user.go
package entity

import "time"

type User struct {
    ID        string
    Name      string
    Email     string
    Status    Status
    CreatedAt time.Time
}

type Status string

const (
    StatusActive  Status = "active"
    StatusInactive Status = "inactive"
    StatusDeleted   Status = "deleted"
)

// Business rules live here
func (u *User) CanPerformAction() bool {
    return u.Status == StatusActive
}

func (u *User) Deactivate() {
    u.Status = StatusInactive
}
```

```go
// internal/domain/rule/validation.go
package rule

// Pure validation logic, no dependencies
func ValidateEmail(email string) bool {
    return email != "" && len(email) >= 3
}

func ValidateName(name string) bool {
    return len(name) >= 1 && len(name) <= 100
}
```

---

## Application Layer

Use cases orchestrate domain logic:

```go
// internal/application/usecase/user.go
package usecase

import (
    "learning-go/internal/domain/entity"
    "learning-go/internal/domain/rule"
    "learning-go/internal/application/port"  // Interfaces
)

type CreateUserUseCase struct {
    userRepo port.UserRepository
}

func NewCreateUser(repo port.UserRepository) *CreateUserUseCase {
    return &CreateUserUseCase{userRepo: repo}
}

func (uc *CreateUserUseCase) Execute(name, email string) (*entity.User, error) {
    // 1. Validate input (use domain rules)
    if !rule.ValidateName(name) {
        return nil, ErrInvalidName
    }
    if !rule.ValidateEmail(email) {
        return nil, ErrInvalidEmail
    }

    // 2. Create entity (domain layer)
    user := &entity.User{
        ID:        generateID(),
        Name:      name,
        Email:     email,
        Status:   entity.StatusActive,
    }

    // 3. Persist (via port, not concrete implementation)
    if err := uc.userRepo.Save(user); err != nil {
        return nil, err
    }

    return user, nil
}
```

---

## Ports (Interfaces)

Define interfaces in application layer, implement in infrastructure:

```go
// internal/application/port/user.go
package port

import "learning-go/internal/domain/entity"

type UserRepository interface {
    Save(user *entity.User) error
    FindByID(id string) (*entity.User, error)
    Delete(id string) error
}

type UserService interface {
    Create(name, email string) (*entity.User, error)
    Get(id string) (*entity.User, error)
}
```

---

## Infrastructure Layer

Implements the ports:

```go
// internal/infrastructure/repository/user.go
package repository

import (
    "learning-go/internal/application/port"
    "learning-go/internal/domain/entity"
)

type InMemoryUserRepository struct {
    users map[string]*entity.User
}

func NewInMemoryUserRepository() port.UserRepository {
    return &InMemoryUserRepository{
        users: make(map[string]*entity.User),
    }
}

func (r *InMemoryUserRepository) Save(user *entity.User) error {
    r.users[user.ID] = user
    return nil
}

func (r *InMemoryUserRepository) FindByID(id string) (*entity.User, error) {
    user, ok := r.users[id]
    if !ok {
        return nil, ErrNotFound
    }
    return user, nil
}

func (r *InMemoryUserRepository) Delete(id string) error {
    delete(r.users, id)
    return nil
}
```

---

## Presentation Layer

HTTP handlers - the outermost layer:

```go
// internal/presentation/handler/user.go
package handler

import (
    "encoding/json"
    "net/http"

    "learning-go/internal/application/usecase"
)

type UserHandler struct {
    createUser *usecase.CreateUserUseCase
}

func New(createUser *usecase.CreateUserUseCase) *UserHandler {
    return &UserHandler{createUser: createUser}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    user, err := h.createUser.Execute(req.Name, req.Email)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    json.NewEncoder(w).Encode(user)
}
```

---

## Wiring It All Together

```go
// cmd/api/main.go
package main

import (
    "log"
    "net/http"

    "learning-go/internal/infrastructure/repository"
    "learning-go/internal/application/usecase"
    "learning-go/internal/presentation/handler"
)

func main() {
    // Infrastructure
    userRepo := repository.NewInMemoryUserRepository()

    // Application (use cases)
    createUserUC := usecase.NewCreateUser(userRepo)

    // Presentation
    userHandler := handler.New(createUserUC)

    // Server
    http.HandleFunc("POST /users", userHandler.Create)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

## Dependency Rule Visualization

```
                    ┌─────────────────────┐
                    │  Presentation       │
                    │   (HTTP Handler)    │
                    └──────────┬──────────┘
                               │ depends on
                               ▼
                    ┌─────────────────────┐
                    │  Application        │
                    │    (Use Cases)      │
                    └──────────┬──────────┘
                               │ depends on
                               ▼
                    ┌─────────────────────┐
                    │      Domain         │
                    │   (Entities, Rules) │
                    └─────────────────────┘
                               ▲
                               │ implements
                    ┌──────────┴──────────┐
                    │  Infrastructure     │
                    │   (Repository)      │
                    └─────────────────────┘
```

---

## Benefits

| Benefit | Description |
|---------|-------------|
| Testable | Domain layer has no dependencies |
| Independent | Swap DB without changing business logic |
| Maintainable | Clear responsibilities per layer |
| Scalable | Easy to add new use cases or infrastructure |

---

## Quick Reference

| Layer | Responsibility | Dependencies |
|-------|----------------|--------------|
| Domain | Entities, rules | None (pure) |
| Application | Use cases | Domain only |
| Infrastructure | Implementations | Application interfaces |
| Presentation | HTTP/CLI | Application use cases |

---

## Common Pitfalls

1. **Mixed layers** - Business logic in handlers
2. **Wrong dependencies** - Domain imports infrastructure
3. **Anemic domain** - Objects with only getters/setters, no behavior
4. **Over-engineering** - Small apps don't need full clean architecture

---

## Next Steps

- [Pub-Sub Design](11-pub-sub-design.md) - Decouple with events
- [Retry + Circuit Breaker](12-retry-circuit-breaker.md) - Resilience
- [Milestone Project](20-layered-http-service.md) - Build with clean architecture