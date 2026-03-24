# Project Structure

> How to organize Go code for maintainability and scale.

---

## Why Structure Matters

```
Your brain can only track ~7 things at once.
A good structure reduces cognitive load.
```

Bad structure → Everything is hard:
- Finding where to add code
- Understanding what code does
- Testing individual components
- Onboarding new team members

Good structure → Everything is easier:
- Clear ownership of code
- Easy to find what you need
- Natural testing boundaries
- Simple onboarding

---

## Standard Go Layout

```
myapp/
├── cmd/                    # Entry points
│   └── myapp/
│       └── main.go         # Or: api/, worker/, etc.
├── internal/               # Private code (not importable)
│   ├── handlers/           # HTTP handlers
│   ├── services/           # Business logic
│   ├── repositories/       # Data access
│   ├── models/             # Data structures
│   └── middleware/         # HTTP middleware
├── pkg/                    # Public code (importable by other projects)
│   └── utils/
├── api/                    # OpenAPI/Swagger specs
├── configs/                # Configuration files
├── scripts/                # Build/utility scripts
├── go.mod
└── README.md
```

### Key Rule: `internal` over `pkg`

```
┌─────────────────────────────────────────────────────┐
│                  Your Project                        │
│                                                      │
│   ┌─────────────┐     ┌─────────────┐               │
│   │   cmd/      │     │   internal/ │               │
│   │  (entry)    │     │  (private)  │               │
│   └─────────────┘     └─────────────┘               │
│                         ┌─────────────┐               │
│                         │    pkg/     │               │
│                         │ (public)    │               │
│                         └─────────────┘               │
└─────────────────────────────────────────────────────┘
```

- `internal/` - Anything you don't want imported by external projects
- `pkg/` - Code explicitly meant to be reusable elsewhere

---

## Layered Structure (Most Common)

For HTTP services, a simpler layered approach often works better:

```
internal/
├── handler/      # HTTP layer (decode request, encode response)
├── service/      # Business logic (validation, orchestration)
├── repository/   # Data access (database queries)
├── model/        # Data structures
└── middleware/   # Cross-cutting concerns
```

### Data Flow

```
┌──────────┐    ┌──────────┐    ┌──────────────┐
│  HTTP    │───▶│ Service  │───▶│ Repository   │
│ Request  │    │ (logic)  │    │ (data fetch) │
└──────────┘    └──────────┘    └──────────────┘
                   ▲                   │
                   └───────────────────┘
                         (return data)
```

---

## Feature-Based Structure (Alternative)

For larger projects, group by feature instead of layer:

```
internal/
├── user/
│   ├── handler.go
│   ├── service.go
│   ├── repository.go
│   └── model.go
├── order/
│   ├── handler.go
│   ├── service.go
│   └── ...
└── payment/
    └── ...
```

**When to use:**
- Team ownership by feature
- Many features with tight coupling
- Clearer boundaries for large teams

**When not to use:**
- Shared infrastructure (logging, auth)
- Many small, related features

---

## Naming Conventions

### Package Names

```go
// Good
package user          // single word, lowercase
package userService   // compound, camelCase in code, hyphen in import

// Bad
package UserService   // Don't capitalize
package user_service  // No underscores
package Users         // Avoid plurals
```

### File Names

```go
// Good
user.go              // singular, describing contents
user_handler.go      // feature + purpose
user_service_test.go // _test suffix for test files

// Bad
User.go              // Don't capitalize
users.go             // Avoid plurals
UserHandler.go       // No PascalCase
```

---

## Import Organization

Go formatter organizes imports in three groups (use `go fmt` or `goimports`):

```go
package main

import (
    // Standard library
    "context"
    "encoding/json"
    "net/http"

    // External packages
    "github.com/google/uuid"
    "go.uber.org/zap"

    // Internal packages
    "myapp/internal/handler"
    "myapp/internal/service"
)
```

---

## Project Structure in Action

```
cmd/
└── api/
    └── main.go

internal/
├── model/
│   └── user.go
├── repository/
│   └── user.go
├── service/
│   └── user.go
└── handler/
    └── user.go
```

### File: `cmd/api/main.go`

```go
package main

import (
    "log"
    "net/http"

    "learning-go/internal/handler"
    "learning-go/internal/repository"
    "learning-go/internal/service"
)

func main() {
    // Wire up dependencies (see Dependency Injection topic)
    repo := repository.NewInMemory()
    svc := service.New(repo)
    h := handler.New(svc)

    // Start server
    log.Fatal(http.ListenAndServe(":8080", h))
}
```

---

## Quick Reference

| Aspect | Recommendation |
|--------|----------------|
| Entry point | `cmd/<appname>/main.go` |
| Private code | `internal/` |
| Public code | `pkg/` |
| Package names | lowercase, singular |
| File names | lowercase, descriptive |
| Layer order | handler → service → repository |
| Group imports | stdlib → external → internal |

---

## Common Pitfalls

1. **Putting everything in root** - Hard to find anything
2. **Deep nesting** - `internal/a/b/c/d/e.go` is painful
3. **Mixed layer types** - Handler, service, and model in one file
4. **No clear boundaries** - What's public vs private?

---

## Next Steps

- [Repository Pattern](07-repository-pattern.md) - Data access abstraction
- [Service Layer](08-service-layer.md) - Business logic isolation
- [Dependency Injection](09-dependency-injection.md) - Wiring components together