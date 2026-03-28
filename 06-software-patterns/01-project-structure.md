# Project Structure

> How to organize Go code for maintainability and scale.

> **Prerequisites:** This section assumes you're comfortable with Go basics (Topics 1-9). The code examples use `net/http` (HTTP server), `encoding/json` (JSON marshal/unmarshal), and `context.Context` (request-scoped data). If any of these are unfamiliar:
> - `net/http`: `go doc net/http.ListenAndServe` — starts an HTTP server
> - `encoding/json`: `go doc encoding/json.Marshal` — converts Go values to JSON
> - `context.Context`: `go doc context.Background` — creates a root context (covered in Topic 14)

---

## Table of Contents

1. [Why Structure Matters](#why-structure-matters) `[PRODUCTION]`
2. [The `internal` vs `pkg` Rule](#the-internal-vs-pkg-rule) `[PRODUCTION]`
3. [Standard Go Layout](#standard-go-layout) `[PRODUCTION]`
4. [Layered Structure (Most Common)](#layered-structure-most-common) `[PRODUCTION]`
5. [Feature-Based Structure (Alternative)](#feature-based-structure-alternative) `[PRODUCTION]`
6. [Naming Conventions](#naming-conventions) `[PRODUCTION]`
7. [Import Organization](#import-organization) `[PRODUCTION]`
8. [Dependency Injection in main.go](#dependency-injection-in-maingo) `[PRODUCTION]`
9. [Configuration Management](#configuration-management) `[PRODUCTION]`
10. [Makefile for Development](#makefile-for-development) `[PRODUCTION]`
11. [Quick Reference](#quick-reference) `[PRODUCTION]`
12. [Common Pitfalls](#common-pitfalls) `[PRODUCTION]`

---

![Project Structure Overview](../assets/project_struct.png)

---

## Why Structure Matters

When you open a codebase, you should know where to find things in under 10 seconds.

Bad structure:
- **Cognitive overload** — everything is everywhere
- **Searching** takes minutes instead of seconds
- **Testing** is hard because code is tangled
- **Onboarding** new developers is painful

Good structure:
- **Clear ownership** — each folder has one job
- **Navigation** — find any file in seconds
- **Testing** — components have natural boundaries
- **Onboarding** — the structure itself explains the code

---

## The `internal` vs `pkg` Rule

Go has a **built-in access control mechanism** — the `internal` package.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                            YOUR PROJECT                                   │
  ├────────────────────────────────┬─────────────────────────────────────────┤
  │         internal/              │              pkg/                       │
  │                                │                                         │
  │   Can ONLY be imported by      │   Public API, importable by            │
  │   your project                 │   ANY project                          │
  │                                │                                         │
  │   ┌─────────────────────────┐  │   ┌─────────────────────────┐          │
  │   │  • Handlers             │  │   │  • Libraries            │          │
  │   │  • Services             │  │   │  • Shared utils         │          │
  │   │  • Repositories         │  │   │  • Client SDKs          │          │
  │   │  • Models               │  │   │                         │          │
  │   │  • Middleware            │  │   │                         │          │
  │   └─────────────────────────┘  │   └─────────────────────────┘          │
  │                                │                                         │
  │   ◄── 95% of your code ──►    │   ◄── 5% of your code ──►              │
  └────────────────────────────────┴─────────────────────────────────────────┘
```

**When to use `pkg`:**
- Building a library others will import
- Shared utilities used across multiple projects
- Client SDKs

**When to use `internal`:**
- Everything else (handlers, services, repos)
- Application-specific business logic
- Any code you don't want imported externally

---

## Standard Go Layout

```
myapp/
├── cmd/                         # Entry points (applications)
│   ├── api/
│   │   └── main.go             # HTTP server
│   ├── worker/
│   │   └── main.go             # Background worker
│   └── cli/
│       └── main.go             # Command-line tool
│
├── internal/                    # Private application code
│   ├── handler/                 # HTTP/GRPC handlers
│   ├── service/                 # Business logic
│   ├── repository/              # Data access
│   ├── model/                   # Domain models
│   ├── middleware/              # HTTP middleware
│   └── events/                  # Event definitions
│
├── pkg/                         # Public libraries (rare)
│   └── logger/
│
├── api/                         # API specifications
│   └── openapi.yaml
│
├── configs/                     # Configuration files
│   └── config.yaml
│
├── scripts/                     # Build scripts
│   └── migrate.sh
│
├── docs/                        # Documentation
│   └── architecture.md
│
├── test/                        # Integration tests
│   └── integration/
│
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── README.md
```

### Why `cmd/` is Separate

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                  WHY NOT JUST ONE main.go?                                │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   A real service often has MULTIPLE executables:                         │
  │                                                                           │
  │   ┌──────────────────────┐    ┌──────────────────────────────────────┐   │
  │   │  cmd/api/main.go     │───►│  HTTP API server                    │   │
  │   ├──────────────────────┤    ├──────────────────────────────────────┤   │
  │   │  cmd/worker/main.go  │───►│  Background job processor           │   │
  │   ├──────────────────────┤    ├──────────────────────────────────────┤   │
  │   │  cmd/migrate/main.go │───►│  Database migration tool            │   │
  │   ├──────────────────────┤    ├──────────────────────────────────────┤   │
  │   │  cmd/seed/main.go    │───►│  Data seeding script                │   │
  │   └──────────────────────┘    └──────────────────────────────────────┘   │
  │                                                                           │
  │          │                          │                                     │
  │          └──────────┬───────────────┘                                     │
  │                     ▼                                                     │
  │          ┌──────────────────────┐                                         │
  │          │    internal/         │  ◄── ALL executables share this code   │
  │          │  (shared packages)   │                                         │
  │          └──────────────────────┘                                         │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Layered Structure (Most Common)

For HTTP services, organize by layer — each layer has ONE job:

```
internal/
├── handler/      # HTTP request/response (protocol layer)
│   └── user.go
├── service/      # Business logic (domain layer)
│   └── user.go
├── repository/   # Data access (persistence layer)
│   └── user.go
├── model/        # Data structures (domain layer)
│   └── user.go
└── middleware/   # Cross-cutting concerns
    ├── auth.go
    ├── logging.go
    └── recovery.go
```

### What Each Layer Does

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                         HTTP REQUEST FLOW                                 │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  LAYER 1: HANDLER (protocol)                                      │  │
  │   │  ┌─────────────────────────────────────────────────────────────┐  │  │
  │   │  │  ✓ Decode JSON/protobuf → Go struct                         │  │  │
  │   │  │  ✓ Validate request FORMAT (missing fields, bad types)      │  │  │
  │   │  │  ✓ Call service method                                       │  │  │
  │   │  │  ✓ Encode response → JSON/protobuf                          │  │  │
  │   │  │  ✓ Set HTTP status codes                                     │  │  │
  │   │  │  ✗ DO NOT: business logic, database queries                 │  │  │
  │   │  └─────────────────────────────────────────────────────────────┘  │  │
  │   └──────────────────────────────────┬────────────────────────────────┘  │
  │                                      │                                    │
  │                                      ▼                                    │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  LAYER 2: SERVICE (domain / business logic)                       │  │
  │   │  ┌─────────────────────────────────────────────────────────────┐  │  │
  │   │  │  ✓ Validate business rules (does this make sense?)          │  │  │
  │   │  │  ✓ Enforce constraints (user can do this?)                  │  │  │
  │   │  │  ✓ Calculate derived values (totals, scores)                │  │  │
  │   │  │  ✓ Orchestrate multiple repositories                        │  │  │
  │   │  │  ✓ Publish domain events                                     │  │  │
  │   │  │  ✗ DO NOT: HTTP, database queries, HTTP status codes        │  │  │
  │   │  └─────────────────────────────────────────────────────────────┘  │  │
  │   └──────────────────────────────────┬────────────────────────────────┘  │
  │                                      │                                    │
  │                                      ▼                                    │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  LAYER 3: REPOSITORY (data access / persistence)                  │  │
  │   │  ┌─────────────────────────────────────────────────────────────┐  │  │
  │   │  │  ✓ Execute SQL queries                                       │  │  │
  │   │  │  ✓ Read/write to database                                    │  │  │
  │   │  │  ✓ Return domain models                                      │  │  │
  │   │  │  ✗ DO NOT: business logic, HTTP handling                     │  │  │
  │   │  └─────────────────────────────────────────────────────────────┘  │  │
  │   └──────────────────────────────────┬────────────────────────────────┘  │
  │                                      │                                    │
  │                                      ▼                                    │
  │                              ┌──────────────┐                             │
  │                              │   DATABASE   │                             │
  │                              └──────────────┘                             │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

### Example: Each Layer

**model/user.go** — Domain entity
```go
package model

import "time"

type User struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}
```

**repository/user.go** — Interface
```go
package repository

import "myapp/internal/model"

type UserRepository interface {
    Create(user *model.User) error
    GetByID(id string) (*model.User, error)
    List() []*model.User
}
```

**service/user.go** — Business logic
```go
package service

import (
    "errors"
    "myapp/internal/model"
    "myapp/internal/repository"
)

type UserService struct {
    repo repository.UserRepository
}

func New(repo repository.UserRepository) *UserService {
    return &UserService{repo: repo}
}

func (s *UserService) CreateUser(name, email string) (*model.User, error) {
    // Business validation
    if name == "" {
        return nil, errors.New("name required")
    }
    
    user := &model.User{
        ID:    generateID(),
        Name:  name,
        Email: email,
    }
    
    return user, s.repo.Create(user)
}
```

**handler/user.go** — HTTP handling
```go
package handler

import (
    "encoding/json"
    "net/http"
    "myapp/internal/service"
)

type UserHandler struct {
    svc *service.UserService
}

func New(svc *service.UserService) *UserHandler {
    return &UserHandler{svc: svc}
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
    
    user, err := h.svc.CreateUser(req.Name, req.Email)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

---

## Feature-Based Structure (Alternative)

For large projects with 10+ features, group by feature instead of layer:

```
internal/
├── user/
│   ├── handler.go
│   ├── service.go
│   ├── repository.go
│   ├── model.go
│   └── errors.go
├── order/
│   ├── handler.go
│   ├── service.go
│   ├── repository.go
│   └── model.go
├── payment/
│   ├── handler.go
│   └── ...
└── shared/                    # Cross-feature utilities
    ├── middleware/
    └── errors/
```

### When to Use Each

| Factor | Layer-Based | Feature-Based |
|--------|-------------|---------------|
| Team size | 1-5 people | 5+ people |
| Features | < 10 features | 10+ features |
| Ownership | Shared code | Feature teams |
| Scaling | Moderate | High |
| Complexity | Low-medium | High |

---

## Naming Conventions

### Package Names

```go
// ✓ Good
package user          // singular, lowercase
package httpserver    // compound, lowercase

// ✗ Bad
package UserService   // Don't capitalize
package user_service  // No underscores
package Users         // Avoid plurals
```

### File Names

```go
// ✓ Good
user.go              // singular, lowercase
user_handler.go      // feature + purpose
user_service_test.go // _test suffix for tests
main.go              // entry point

// ✗ Bad
User.go              // No PascalCase
users.go             // Avoid plurals
UserHandler.go       // No mixed case
user-handler.go      // No hyphens
```

### Function/Type Names

```go
// ✓ Good — exported names describe WHAT
func NewUserService(repo Repository) *UserService
func (s *UserService) CreateUser(name string) (*User, error)
type UserRepository interface { ... }

// ✓ Good — unexported names describe WHAT
func validateEmail(email string) bool
func hashPassword(pw string) string

// ✗ Bad
func init()           // Avoid — confusing with Go's init()
func Do()             // Do what?
func Handle()         // Handle what?
```

---

## Import Organization

Go imports are organized in three groups (enforced by `goimports`):

```go
package main

import (
    // Group 1: Standard library
    "context"
    "encoding/json"
    "net/http"
    "time"

    // Group 2: External packages
    "github.com/google/uuid"
    "go.uber.org/zap"

    // Group 3: Internal packages (your project)
    "myapp/internal/handler"
    "myapp/internal/repository"
    "myapp/internal/service"
)
```

### Import Aliases

```go
// Use when package name conflicts
import (
    "fmt"
    mysql "github.com/go-sql-driver/mysql" // aliased
)

// Use for convenience
import (
    "encoding/json"
    "net/http"
    
    httpHandler "myapp/internal/handler"
)
```

---

## Dependency Injection in main.go

The `main.go` file is your **wiring point** — it connects all layers:

```go
// cmd/api/main.go
package main

import (
    "log"
    "net/http"

    "myapp/internal/handler"
    "myapp/internal/middleware"
    "myapp/internal/repository"
    "myapp/internal/service"
)

func main() {
    // 1. Create infrastructure (external services)
    db := connectToDatabase()      // returns *sql.DB
    logger := setupLogger()        // returns *zap.Logger
    metrics := setupMetrics()      // returns *Metrics

    // 2. Create repositories (data access)
    userRepo := repository.NewUser(db, logger)
    orderRepo := repository.NewOrder(db, logger)

    // 3. Create services (business logic)
    userService := service.NewUser(userRepo, logger, metrics)
    orderService := service.NewOrder(orderRepo, userRepo, logger)

    // 4. Create handlers (HTTP layer)
    userHandler := handler.NewUser(userService)
    orderHandler := handler.NewOrder(orderService)

    // 5. Set up routes
    mux := http.NewServeMux()
    mux.HandleFunc("POST /users", userHandler.Create)
    mux.HandleFunc("GET /users/{id}", userHandler.Get)
    mux.HandleFunc("POST /orders", orderHandler.Create)

    // 6. Add middleware chain
    server := middleware.Chain(
        mux,
        middleware.Logger(logger),
        middleware.Recovery(logger),
        middleware.Metrics(metrics),
        middleware.Auth(jwtSecret),
    )

    // 7. Start server
    log.Fatal(http.ListenAndServe(":8080", server))
}
```

---

## Configuration Management

```
myapp/
├── config/
│   ├── config.yaml        # Default config
│   ├── config.dev.yaml    # Development overrides
│   └── config.prod.yaml   # Production overrides
└── internal/
    └── config/
        └── config.go      # Config struct + loader
```

```go
// internal/config/config.go
package config

import (
    "os"
    "github.com/spf13/viper"
)

type Config struct {
    Server ServerConfig `yaml:"server"`
    Database DBConfig   `yaml:"database"`
}

type ServerConfig struct {
    Host string `yaml:"host"`
    Port int    `yaml:"port"`
}

type DBConfig struct {
    Host     string `yaml:"host"`
    Port     int    `yaml:"port"`
    Name     string `yaml:"name"`
    User     string `yaml:"user"`
    Password string `yaml:"password"` // From env var
}

func Load() (*Config, error) {
    v := viper.New()
    v.SetConfigName("config")
    v.SetConfigType("yaml")
    v.AddConfigPath(".")
    
    // Environment variable overrides
    v.SetEnvPrefix("MYAPP")
    v.AutomaticEnv()
    
    if err := v.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var cfg Config
    return &cfg, v.Unmarshal(&cfg)
}
```

---

## Makefile for Development

```makefile
.PHONY: build test lint run clean

build:
	go build -o bin/api ./cmd/api

test:
	go test ./... -v -race -cover

lint:
	golangci-lint run ./...

run: build
	./bin/api

dev:
	air  # Hot reload with air

clean:
	rm -rf bin/

migrate:
	go run ./cmd/migrate up

seed:
	go run ./cmd/seed
```

---

## Quick Reference

| Aspect | Recommendation |
|--------|----------------|
| Entry point | `cmd/<app>/main.go` |
| Private code | `internal/` |
| Public code | `pkg/` (use rarely) |
| Package names | lowercase, singular |
| File names | lowercase, underscore separated |
| Layer order | handler → service → repository |
| Import order | stdlib → external → internal |
| Config | `config/` + env vars |
| API specs | `api/` |

---

## Common Pitfalls

1. **Everything in root** — No folder structure, hard to navigate
2. **Deep nesting** — `internal/a/b/c/d/e.go` is painful
3. **Mixed layers** — Handler + service + repo in one file
4. **No clear access control** — Use `internal` for privacy
5. **Circular imports** — Separate packages don't import each other
6. **god package** — One giant package that does everything

---

## Next Steps

- [Repository Pattern](02-repository-pattern.md) — Data access abstraction
- [Service Layer](03-service-layer.md) — Business logic isolation
- [Dependency Injection](04-dependency-injection.md) — Wiring components
- [Clean Architecture](05-clean-architecture.md) — Layer boundaries