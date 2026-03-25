# Clean Architecture

> Organize code so business logic is independent of external concerns.

---

## The Core Principle

**Dependencies point inward.** Inner layers never import outer layers.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                      CLEAN ARCHITECTURE LAYERS                            │
  │                  (dependencies point INWARD only)                         │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  INFRASTRUCTURE (outermost)                                       │  │
  │   │  DBs, Message Queues, External APIs, Frameworks                   │  │
  │   │                                                                    │  │
  │   │   ┌─────────────────────────────────────────────────────────────┐ │  │
  │   │   │  PRESENTATION                                               │ │  │
  │   │   │  HTTP Handlers, CLI, gRPC endpoints                         │ │  │
  │   │   │                                                              │ │  │
  │   │   │   ┌───────────────────────────────────────────────────────┐ │ │  │
  │   │   │   │  APPLICATION (Use Cases)                              │ │ │  │
  │   │   │   │  Business workflows, orchestration                    │ │ │  │
  │   │   │   │                                                        │ │ │  │
  │   │   │   │   ┌───────────────────────────────────────────────┐   │ │ │  │
  │   │   │   │   │  DOMAIN (innermost)                            │   │ │ │  │
  │   │   │   │   │  Entities, Value Objects, Business Rules       │   │ │ │  │
  │   │   │   │   │                                                │   │ │ │  │
  │   │   │   │   │   NEVER depends on outer layers                │   │ │ │  │
  │   │   │   │   │   Pure business logic — no imports             │   │ │ │  │
  │   │   │   │   └───────────────────────────────────────────────┘   │ │ │  │
  │   │   │   └───────────────────────────────────────────────────────┘ │ │  │
  │   │   └─────────────────────────────────────────────────────────────┘ │  │
  │   └───────────────────────────────────────────────────────────────────┘  │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## The Four Layers

### 1. Domain (Innermost)

**No imports from outer layers.** Pure business logic.

```go
// internal/domain/entity/user.go
package entity

import "time"

type Status string

const (
    StatusActive   Status = "active"
    StatusInactive Status = "inactive"
    StatusDeleted  Status = "deleted"
)

// User is a domain entity with business rules
type User struct {
    ID        string
    Name      string
    Email     string
    Status    Status
    Role      string
    CreatedAt time.Time
    UpdatedAt time.Time
}

// CanPerformAction is a business rule
func (u *User) CanPerformAction() bool {
    return u.Status == StatusActive
}

// Suspend sets user status to inactive
func (u *User) Suspend() {
    u.Status = StatusInactive
}

// Activate sets user status to active
func (u *User) Activate() {
    u.Status = StatusActive
}
```

```go
// internal/domain/rule/user_rules.go
package rule

import (
    "errors"
    "strings"
)

// ValidateEmail checks email format
func ValidateEmail(email string) error {
    email = strings.TrimSpace(email)
    if email == "" {
        return errors.New("email is required")
    }
    if !strings.Contains(email, "@") {
        return errors.New("invalid email format")
    }
    return nil
}

// ValidateName checks name length
func ValidateName(name string) error {
    name = strings.TrimSpace(name)
    if name == "" {
        return errors.New("name is required")
    }
    if len(name) > 100 {
        return errors.New("name too long")
    }
    return nil
}
```

### 2. Application (Use Cases)

**Depends on Domain only.** Defines interfaces (ports) for external services.

```go
// internal/application/port/user_repo.go
package port

import "myapp/internal/domain/entity"

// UserRepository is the PORT - defined here, implemented elsewhere
type UserRepository interface {
    Save(user *entity.User) error
    FindByID(id string) (*entity.User, error)
    FindByEmail(email string) (*entity.User, error)
    Update(user *entity.User) error
    Delete(id string) error
    List(filter UserFilter) ([]*entity.User, error)
}

type UserFilter struct {
    Status string
    Limit  int
    Offset int
}
```

```go
// internal/application/usecase/create_user.go
package usecase

import (
    "context"
    "myapp/internal/application/port"
    "myapp/internal/domain/entity"
    "myapp/internal/domain/rule"
)

// CreateUserUseCase handles the "create user" use case
type CreateUserUseCase struct {
    repo port.UserRepository
}

func NewCreateUser(repo port.UserRepository) *CreateUserUseCase {
    return &CreateUserUseCase{repo: repo}
}

func (uc *CreateUserUseCase) Execute(ctx context.Context, name, email string) (*entity.User, error) {
    // 1. Validate input (domain rules)
    if err := rule.ValidateName(name); err != nil {
        return nil, err
    }
    if err := rule.ValidateEmail(email); err != nil {
        return nil, err
    }

    // 2. Check business rule (uniqueness)
    existing, _ := uc.repo.FindByEmail(email)
    if existing != nil {
        return nil, errors.New("email already exists")
    }

    // 3. Create entity (domain)
    user := &entity.User{
        ID:        generateID(),
        Name:      name,
        Email:     email,
        Status:    entity.StatusActive,
        Role:      "user",
    }

    // 4. Persist (via port)
    if err := uc.repo.Save(user); err != nil {
        return nil, err
    }

    return user, nil
}
```

### 3. Infrastructure (Outermost)

**Implements the ports.** Contains all external integrations.

```go
// internal/infrastructure/repository/user_postgres.go
package repository

import (
    "database/sql"
    "myapp/internal/application/port"
    "myapp/internal/domain/entity"
)

type postgresUserRepository struct {
    db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) port.UserRepository {
    return &postgresUserRepository{db: db}
}

func (r *postgresUserRepository) Save(user *entity.User) error {
    _, err := r.db.Exec(
        "INSERT INTO users (id, name, email, status) VALUES ($1, $2, $3, $4)",
        user.ID, user.Name, user.Email, user.Status,
    )
    return err
}

func (r *postgresUserRepository) FindByID(id string) (*entity.User, error) {
    var user entity.User
    err := r.db.QueryRow(
        "SELECT id, name, email, status FROM users WHERE id = $1", id,
    ).Scan(&user.ID, &user.Name, &user.Email, &user.Status)
    
    if err == sql.ErrNoRows {
        return nil, nil
    }
    return &user, err
}

// ... other methods
```

### 4. Presentation (Outermost)

**Depends on Application only.** Handles protocol (HTTP, CLI, gRPC).

```go
// internal/presentation/http/user_handler.go
package http

import (
    "encoding/json"
    "net/http"
    "myapp/internal/application/usecase"
)

type UserHandler struct {
    createUser *usecase.CreateUserUseCase
    getUser    *usecase.GetUserUseCase
}

func NewUserHandler(
    create *usecase.CreateUserUseCase,
    get *usecase.GetUserUseCase,
) *UserHandler {
    return &UserHandler{
        createUser: create,
        getUser:    get,
    }
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }

    user, err := h.createUser.Execute(r.Context(), req.Name, req.Email)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

---

## Wiring (main.go)

The entry point connects all layers:

```go
// cmd/api/main.go
package main

import (
    "log"
    "net/http"

    "myapp/internal/application/usecase"
    "myapp/internal/infrastructure/repository"
    httpHandler "myapp/internal/presentation/http"
)

func main() {
    // Infrastructure
    db := connectToDatabase()
    userRepo := repository.NewPostgresUserRepository(db)

    // Use Cases (application layer)
    createUser := usecase.NewCreateUser(userRepo)
    getUser := usecase.NewGetUser(userRepo)

    // Handlers (presentation layer)
    userHandler := httpHandler.NewUserHandler(createUser, getUser)

    // Routes
    mux := http.NewServeMux()
    mux.HandleFunc("POST /users", userHandler.Create)
    mux.HandleFunc("GET /users/{id}", userHandler.Get)

    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

---

## Dependency Rule

The golden rule: **inner layers don't know about outer layers.**

```
                  ┌─────────────────────────────┐
                  │      Presentation            │
                  │      (HTTP Handler)          │
                  └──────────────┬──────────────┘
                                 │
                                 │  imports
                                 ▼
                  ┌─────────────────────────────┐
                  │      Application             │
                  │      (Use Cases)             │
                  └──────────────┬──────────────┘
                                 │
                                 │  imports
                                 ▼
                  ┌─────────────────────────────┐
                  │        Domain               │
                  │   (Entities, Rules)         │
                  └──────────────▲──────────────┘
                                 │
                                 │  implements
                  ┌──────────────┴──────────────┐
                  │      Infrastructure          │
                  │   (Repository impl)          │
                  └─────────────────────────────┘

  ┌──────────────────────────────────────────────────────────────────────────┐
  │  RULE:                                                                   │
  │  • Domain NEVER imports Application, Infrastructure, or Presentation    │
  │  • Application NEVER imports Infrastructure or Presentation             │
  │  • Infrastructure IMPLEMENTS Application ports (interfaces)             │
  │  • Presentation CALLS Application use cases                             │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Ports and Adapters (Hexagonal Architecture)

Clean Architecture is also known as **Hexagonal Architecture** or **Ports and Adapters**:

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                 PORTS AND ADAPTERS (Hexagonal Architecture)               │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   ┌────────────┐                                     ┌────────────┐      │
  │   │  Adapter   │                                     │  Adapter   │      │
  │   │  (HTTP)    │                                     │  (DB)      │      │
  │   │            │                                     │            │      │
  │   │ REST/gRPC  │                                     │ Postgres/  │      │
  │   │ handlers   │                                     │ Redis/MQ   │      │
  │   └─────┬──────┘                                     └──────┬─────┘      │
  │         │                                                   │            │
  │         │ implements                                  implements          │
  │         │                                                   │            │
  │   ┌─────▼─────────────────────────────────────────────────▼─────┐      │
  │   │                                                              │      │
  │   │   PORT (Interface)         PORT (Interface)                 │      │
  │   │   ┌──────────────┐        ┌──────────────┐                  │      │
  │   │   │ UserHandler  │        │ UserRepository│                 │      │
  │   │   │  interface   │        │  interface    │                 │      │
  │   │   └──────────────┘        └──────────────┘                  │      │
  │   │                                                              │      │
  │   │              ┌───────────────────────┐                       │      │
  │   │              │                       │                       │      │
  │   │              │   DOMAIN LOGIC        │                       │      │
  │   │              │   (Application Core)  │                       │      │
  │   │              │                       │                       │      │
  │   │              └───────────────────────┘                       │      │
  │   │                                                              │      │
  │   └──────────────────────────────────────────────────────────────┘      │
  │                                                                           │
  │   PORTS = Interfaces defined by your application                         │
  │   ADAPTERS = Implementations that plug into the ports                    │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## When to Use Clean Architecture

| Scenario | Use Clean Architecture? |
|----------|------------------------|
| Simple CRUD API | ❌ Overkill |
| Complex business logic | ✅ Yes |
| Multiple clients (HTTP, CLI, gRPC) | ✅ Yes |
| Need to swap databases | ✅ Yes |
| Team size > 5 people | ✅ Yes |
| Prototype/MVP | ❌ Keep it simple |

---

## Quick Reference

| Layer | Responsibility | Imports From |
|-------|----------------|--------------|
| Domain | Entities, rules | Nothing |
| Application | Use cases, ports | Domain |
| Infrastructure | Implementations | Application ports |
| Presentation | HTTP/CLI | Application use cases |

---

## Common Pitfalls

1. **Domain imports infrastructure** — Never do this
2. **Anemic domain** — Entities with no behavior
3. **Too many layers** — Over-engineering small apps
4. **Wrong port placement** — Ports should be in Application layer
5. **Missing interfaces** — Always define ports, never concrete types

---

## Next Steps

- [Pub-Sub Design](06-pub-sub-design.md) — Decouple with events
- [Retry + Circuit Breaker](07-retry-circuit-breaker.md) — Resilience
- [Milestone Project](../projects/20-layered-http-service.md) — Build it end-to-end