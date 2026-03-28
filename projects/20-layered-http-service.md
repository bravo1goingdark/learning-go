# Layered HTTP Service Project

> Build a complete CRUD API using all software patterns learned.

---

## Table of Contents

1. [Project Overview](#project-overview) `[CORE]`
2. [Step 1: Project Structure](#step-1-project-structure) `[PRODUCTION]`
3. [Step 2: Model](#step-2-model) `[CORE]`
4. [Step 3: Repository (Interface + Implementation)](#step-3-repository-interface--implementation) `[PRODUCTION]`
5. [Step 4: Events](#step-4-events) `[PRODUCTION]`
6. [Step 5: Service](#step-5-service) `[PRODUCTION]`
7. [Step 6: Handler](#step-6-handler) `[PRODUCTION]`
8. [Step 7: Middleware](#step-7-middleware) `[PRODUCTION]`
9. [Step 8: Main (Wiring)](#step-8-main-wiring) `[PRODUCTION]`
10. [Step 9: Build and Run](#step-9-build-and-run) `[CORE]`
11. [Expected Output](#expected-output) `[CORE]`
12. [Quick Reference](#quick-reference) `[PRODUCTION]`
13. [Extensions](#extensions) `[PRODUCTION]`

---

## Project Overview

### What We're Building

Build a user management HTTP service with:

- **Repository Pattern** - In-memory data storage with interface
- **Service Layer** - Business logic and validation
- **Dependency Injection** - Clean component wiring
- **Pub-Sub** - Event-driven notifications
- **HTTP Handlers** - RESTful API endpoints

### Why This Project?

| Why This Matters | Explanation |
|-----------------|-------------|
| **Production-ready** | Every real Go service uses layered architecture |
| **Pattern mastery** | Repository, Service, DI, Events - all in one project |
| **Extensibility** | Easy to swap in-memory for real database later |
| **Testability** | Each layer can be tested independently |

### How It Works (Intuition)

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP Layer                           │
│         POST /users    GET /users/{id}   DELETE /users/{id}│
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼ (depends on interface)
┌─────────────────────────────────────────────────────────────┐
│                      Service Layer                          │
│          CreateUser    GetUser    UpdateUser   DeleteUser   │
│              │            │            │            │       │
│              ▼            ▼            ▼            ▼       │
│          Validation   Validation   Validation   Validation │
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼ (depends on interface)
┌─────────────────────────────────────────────────────────────┐
│                    Repository Layer                         │
│              InMemoryUserRepository (interface)             │
│                    │                                        │
│                    ▼                                        │
│              In-memory map                                  │
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼ (fire-and-forget events)
┌─────────────────────────────────────────────────────────────┐
│                       Event Bus                            │
│               (UserCreated, UserUpdated, UserDeleted)      │
│                    │                                        │
│                    ▼                                        │
│              Event handlers (logging, etc.)                │
└─────────────────────────────────────────────────────────────┘
```

**Key insight:** Each layer **only knows about the layer below it**. The HTTP layer doesn't know about the map - it only knows about the Service interface. This makes testing and swapping implementations easy.

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP Layer                           │
│         POST /users    GET /users/{id}   DELETE /users/{id}│
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Service Layer                          │
│          CreateUser    GetUser    UpdateUser   DeleteUser   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Repository Layer                         │
│              InMemoryUserRepository (interface)             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Event Bus                             │
│               (UserCreated, UserUpdated, UserDeleted)       │
└─────────────────────────────────────────────────────────────┘
```

---

## Step 1: Project Structure

### What / Why / How

**What:** The directory layout follows Go's project conventions.

**Why:**
- `cmd/` — executables (main.go)
- `internal/` — private packages (can't be imported by external modules)
- Layered folders: model → repository → service → handler

**How:**
- Each layer in its own folder
- Dependencies point inward (handler → service → repository → model)

```
06-software-patterns-project/
├── cmd/                          # Executable entry points
│   └── server/
│       └── main.go               # Wiring everything together
├── internal/                    # Private packages (internal/)
│   ├── model/                    # Domain models (innermost)
│   │   └── user.go              # User struct, constants
│   ├── repository/               # Data access layer
│   │   └── user.go              # Repository interface + impl
│   ├── service/                  # Business logic layer
│   │   └── user.go              # Service interface + impl
│   ├── handler/                  # HTTP handlers
│   │   └── user.go              # HTTP endpoint handlers
│   ├── events/                   # Event bus (pub-sub)
│   │   ├── bus.go               # Event system
│   │   └── handlers/
│   │       └── logger.go       # Event handlers
│   └── middleware/               # HTTP middleware
│       └── middleware.go        # Logging, auth, etc.
├── go.mod
└── README.md
```

---

## Step 2: Model

### What / Why / How

**What:** Define the User struct and related constants.

**Why:**
- Model is the **innermost layer** — no dependencies on outer layers
- Contains pure data, no behavior
- Constants for status values avoid magic strings

**How:**
- Struct with JSON tags for HTTP serialization
- Constants for valid status values

### `internal/model/user.go`

```go
package model

import "time"

// ============================================================================
// USER STRUCT
// ============================================================================

// User represents a user in our system.
// We use JSON struct tags for automatic marshaling/unmarshaling.
//
// Why struct tags?
// - Go doesn't serialize field names by default
// - JSON tags tell the encoder what JSON keys to use
// - Without tags: {"Name": "Alice"} - with tags: {"name": "alice"}
//
// Topic 5 (Structs): Struct with fields and tags
type User struct {
	ID        string    `json:"id"`        // Unique identifier
	Name      string    `json:"name"`      // User's display name
	Email     string    `json:"email"`     // User's email (unique)
	Status    string    `json:"status"`    // User status (active/inactive)
	CreatedAt time.Time `json:"created_at"` // When created
	UpdatedAt time.Time `json:"updated_at"` // Last update time
}

// ============================================================================
// STATUS CONSTANTS
// ============================================================================

// Constants avoid "magic strings" throughout the codebase.
//
// Why constants?
// - Type safety: StatusActive can't be misspelled
// - IDE autocomplete: Type StatusActive. <tab> shows options
// - Single source of truth: change once, everywhere updates
const (
	StatusActive   = "active"   // User can log in, use system
	StatusInactive = "inactive" // User cannot log in
)
```

---

## Step 3: Repository (Interface + Implementation)

### What / Why / How

**What:** Define repository interface and in-memory implementation.

**Why:**
- **Interface** — defines contract, allows swapping implementations
- **In-memory impl** — simple, fast, good for development/testing
- **Later** — swap to PostgreSQL/MySQL without changing service layer

**How:**
- Interface defines CRUD methods
- Implementation uses `map[string]*model.User` for storage
- RWMutex for thread-safe access

### Intuition: Repository Pattern

```
WITHOUT REPOSITORY (PROBLEM):
  func GetUser(id string) *User {
      //直接访问数据库
      // If DB changes → change EVERY call site!
  }

WITH REPOSITORY (SOLUTION):
  func GetUser(id string) *User {
      repo.GetByID(id)  // Don't care WHERE data comes from
  }
  
  // Later: repo := NewPostgreSQLRepo()
  // Or:    repo := NewMockRepo()  // For testing!
```

### `internal/repository/user.go` — Interface

```go
package repository

import "learning-go/internal model"

// ============================================================================
// REPOSITORY INTERFACE
// ============================================================================

// UserRepository defines the contract for user data access.
// ANY implementation (database, cache, mock) satisfies this interface.
//
// Why an interface?
// - Service layer depends on ABSTRACTION, not CONCRETION
// - Can swap implementations without changing service code
// - Can use mock in tests
//
// Topic 7 (Interfaces): Define behavior, not implementation
type UserRepository interface {
	Create(user *model.User) error      // Add new user
	GetByID(id string) (*model.User, error) // Fetch by ID
	Update(user *model.User) error      // Update existing
	Delete(id string) error             // Remove by ID
	List() []*model.User                // Get all users
}

// Re-use model's not-found error
var (
	ErrNotFound = model.ErrNotFound
)
```

### `internal/repository/memory.go` — Implementation

```go
package repository

import (
	"errors" // errors.Is for error checking
	"sync"  // RWMutex for thread safety

	"learning-go/internal/model"
)

// ============================================================================
// IN-MEMORY REPOSITORY
// ============================================================================

// userRepository stores users in a map.
// In-memory = fast, but lost on restart.
//
// Why a map?
// - O(1) lookup by ID
// - Simple to implement
// - Good for development/testing
//
// Thread Safety:
// - RWMutex allows MANY readers OR ONE writer
// - Multiple goroutines can read simultaneously
// - Write operations are exclusive
type userRepository struct {
	mu   sync.RWMutex           // Read-write mutex
	data map[string]*model.User // ID -> User map
}

// NewInMemory creates a new in-memory repository.
// Returns interface (not concrete type) — caller depends on abstraction.
func NewInMemory() UserRepository {
	return &userRepository{
		// Map must be initialized before use!
		// nil map read = "", nil map write = panic
		data: make(map[string]*model.User),
	}
}

// ============================================================================
// CRUD OPERATIONS
// ============================================================================

// Create adds a new user to the repository.
func (r *userRepository) Create(user *model.User) error {
	// Write lock: only one goroutine can write at a time
	r.mu.Lock()
	defer r.mu.Unlock() // Unlock when function returns

	// Check if user already exists
	if _, exists := r.data[user.ID]; exists {
		return errors.New("user already exists")
	}

	// Store user in map
	r.data[user.ID] = user
	return nil
}

// GetByID retrieves a user by their ID.
func (r *userRepository) GetByID(id string) (*model.User, error) {
	// Read lock: many goroutines can read simultaneously
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Map lookup: returns zero-value if not found
	user, exists := r.data[id]
	if !exists {
		return nil, ErrNotFound // Return sentinel error
	}

	// Return copy to prevent external mutation
	// (*user) dereferences to get the User struct
	return user, nil
}

// Update modifies an existing user.
func (r *userRepository) Update(user *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if user exists
	if _, exists := r.data[user.ID]; !exists {
		return ErrNotFound
	}

	// Update in place
	r.data[user.ID] = user
	return nil
}

// Delete removes a user by ID.
func (r *userRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check existence
	if _, exists := r.data[id]; !exists {
		return ErrNotFound
	}

	// Delete from map
	delete(r.data, id)
	return nil
}

// List returns all users.
func (r *userRepository) List() []*model.User {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create new slice to return
	// Pre-allocate with exact capacity
	users := make([]*model.User, 0, len(r.data))
	
	// Range over map - order is random (that's OK for this use case)
	for _, user := range r.data {
		users = append(users, user)
	}
	
	return users
}
```

func (r *userRepository) Create(user *model.User) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.data[user.ID]; exists {
        return errors.New("user already exists")
    }

    r.data[user.ID] = user
    return nil
}

func (r *userRepository) GetByID(id string) (*model.User, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    user, ok := r.data[id]
    if !ok {
        return nil, errors.New("user not found")
    }

    return user, nil
}

func (r *userRepository) Update(user *model.User) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.data[user.ID]; !exists {
        return errors.New("user not found")
    }

    r.data[user.ID] = user
    return nil
}

func (r *userRepository) Delete(id string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.data[id]; !exists {
        return errors.New("user not found")
    }

    delete(r.data, id)
    return nil
}

func (r *userRepository) List() []*model.User {
    r.mu.RLock()
    defer r.mu.RUnlock()

    users := make([]*model.User, 0, len(r.data))
    for _, u := range r.data {
        users = append(users, u)
    }

    return users
}
```

---

## Step 4: Events

```go
// internal/events/events.go
package events

import "time"

type Event interface {
    Type() string
    Timestamp() time.Time
}

type UserCreatedEvent struct {
    UserID    string    `json:"user_id"`
    Email     string    `json:"email"`
    Name      string    `json:"name"`
    Timestamp time.Time `json:"timestamp"`
}

func (e UserCreatedEvent) Type() string    { return "user.created" }
func (e UserCreatedEvent) Timestamp() time.Time { return e.Timestamp }

type UserUpdatedEvent struct {
    UserID    string    `json:"user_id"`
    Name      string    `json:"name"`
    Timestamp time.Time `json:"timestamp"`
}

func (e UserUpdatedEvent) Type() string    { return "user.updated" }
func (e UserUpdatedEvent) Timestamp() time.Time { return e.Timestamp }

type UserDeletedEvent struct {
    UserID    string    `json:"user_id"`
    Timestamp time.Time `json:"timestamp"`
}

func (e UserDeletedEvent) Type() string    { return "user.deleted" }
func (e UserDeletedEvent) Timestamp() time.Time { return e.Timestamp }
```

```go
// internal/events/bus.go
package events

import "sync"

type Handler interface {
    Handle(event Event) error
}

type EventBus struct {
    mu          sync.RWMutex
    subscribers map[string][]Handler
}

func New() *EventBus {
    return &EventBus{
        subscribers: make(map[string][]Handler),
    }
}

func (b *EventBus) Subscribe(eventType string, handler Handler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

func (b *EventBus) Publish(event Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    handlers, ok := b.subscribers[event.Type()]
    if !ok {
        return
    }

    for _, handler := range handlers {
        go func(h Handler) { _ = h.Handle(event) }(handler)
    }
}
```

```go
// internal/events/handlers/logger.go
package handlers

import (
    "fmt"

    "learning-go/internal/events"
)

type LoggerHandler struct{}

func NewLoggerHandler() *LoggerHandler {
    return &LoggerHandler{}
}

func (h *LoggerHandler) Handle(event events.Event) error {
    fmt.Printf("[EVENT] %s: %+v\n", event.Type(), event)
    return nil
}
```

---

## Step 5: Service

```go
// internal/service/user.go
package service

import (
    "errors"
    "time"

    "learning-go/internal/events"
    "learning-go/internal/model"
    "learning-go/internal/repository"
)

var (
    ErrInvalidInput = errors.New("invalid input")
    ErrNotFound     = errors.New("user not found")
    ErrAlreadyExists = errors.New("user already exists")
)

type UserService struct {
    repo     repository.UserRepository
    eventBus *events.EventBus
}

func New(repo repository.UserRepository, eb *events.EventBus) *UserService {
    return &UserService{
        repo:     repo,
        eventBus: eb,
    }
}

func (s *UserService) CreateUser(name, email string) (*model.User, error) {
    if name == "" {
        return nil, ErrInvalidInput
    }
    if email == "" {
        return nil, ErrInvalidInput
    }

    // Check duplicates
    users := s.repo.List()
    for _, u := range users {
        if u.Email == email {
            return nil, ErrAlreadyExists
        }
    }

    user := &model.User{
        ID:        "user-" + time.Now().Format("20060102150405"),
        Name:      name,
        Email:     email,
        Status:    model.StatusActive,
        CreatedAt: time.Now(),
    }

    if err := s.repo.Create(user); err != nil {
        return nil, err
    }

    if s.eventBus != nil {
        s.eventBus.Publish(events.UserCreatedEvent{
            UserID:    user.ID,
            Email:     user.Email,
            Name:      user.Name,
            Timestamp: time.Now(),
        })
    }

    return user, nil
}

func (s *UserService) GetUser(id string) (*model.User, error) {
    user, err := s.repo.GetByID(id)
    if err != nil {
        return nil, ErrNotFound
    }
    return user, nil
}

func (s *UserService) UpdateUser(id, name string) (*model.User, error) {
    user, err := s.repo.GetByID(id)
    if err != nil {
        return nil, ErrNotFound
    }

    user.Name = name
    user.UpdatedAt = time.Now()

    if err := s.repo.Update(user); err != nil {
        return nil, err
    }

    if s.eventBus != nil {
        s.eventBus.Publish(events.UserUpdatedEvent{
            UserID:    user.ID,
            Name:      user.Name,
            Timestamp: time.Now(),
        })
    }

    return user, nil
}

func (s *UserService) DeleteUser(id string) error {
    if err := s.repo.Delete(id); err != nil {
        return ErrNotFound
    }

    if s.eventBus != nil {
        s.eventBus.Publish(events.UserDeletedEvent{
            UserID:    id,
            Timestamp: time.Now(),
        })
    }

    return nil
}

func (s *UserService) ListUsers() []*model.User {
    return s.repo.List()
}
```

---

## Step 6: Handler

```go
// internal/handler/user.go
package handler

import (
    "encoding/json"
    "errors"
    "net/http"

    "learning-go/internal/service"
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
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    user, err := h.svc.CreateUser(req.Name, req.Email)
    if err != nil {
        status := http.StatusBadRequest
        if errors.Is(err, service.ErrAlreadyExists) {
            status = http.StatusConflict
        }
        http.Error(w, err.Error(), status)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        http.Error(w, "missing id", http.StatusBadRequest)
        return
    }

    user, err := h.svc.GetUser(id)
    if err != nil {
        http.Error(w, "user not found", http.StatusNotFound)
        return
    }

    json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        http.Error(w, "missing id", http.StatusBadRequest)
        return
    }

    var req struct {
        Name string `json:"name"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    user, err := h.svc.UpdateUser(id, req.Name)
    if err != nil {
        http.Error(w, "user not found", http.StatusNotFound)
        return
    }

    json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        http.Error(w, "missing id", http.StatusBadRequest)
        return
    }

    if err := h.svc.DeleteUser(id); err != nil {
        http.Error(w, "user not found", http.StatusNotFound)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
    users := h.svc.ListUsers()
    json.NewEncoder(w).Encode(users)
}
```

---

## Step 7: Middleware

```go
// internal/middleware/middleware.go
package middleware

import (
    "log"
    "time"
)

func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        next.ServeHTTP(w, r)

        log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
    })
}

func Recovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("panic: %v", err)
                http.Error(w, "internal server error", http.StatusInternalServerError)
            }
        }()

        next.ServeHTTP(w, r)
    })
}
```

---

## Step 8: Main (Wiring)

```go
// cmd/server/main.go
package main

import (
    "log"
    "net/http"

    "learning-go/internal/events"
    "learning-go/internal/events/handlers"
    "learning-go/internal/handler"
    "learning-go/internal/middleware"
    "learning-go/internal/repository"
    "learning-go/internal/service"
)

func main() {
    // Events
    eb := events.New()
    eb.Subscribe("user.created", handlers.NewLoggerHandler())
    eb.Subscribe("user.updated", handlers.NewLoggerHandler())
    eb.Subscribe("user.deleted", handlers.NewLoggerHandler())

    // Repository
    userRepo := repository.NewInMemory()

    // Service (DI)
    userSvc := service.New(userRepo, eb)

    // Handler
    userHandler := handler.New(userSvc)

    // Router
    // NOTE: The "METHOD /path" syntax (e.g., "POST /users") requires Go 1.22+.
    // For Go 1.21 and earlier, use a third-party router like gorilla/mux or chi.
    mux := http.NewServeMux()
    mux.HandleFunc("POST /users", userHandler.Create)
    mux.HandleFunc("GET /users", userHandler.List)
    mux.HandleFunc("GET /users/{id}", userHandler.Get)
    mux.HandleFunc("PUT /users/{id}", userHandler.Update)
    mux.HandleFunc("DELETE /users/{id}", userHandler.Delete)

    // Middleware chain
    wrapped := middleware.Logger(middleware.Recovery(mux))

    log.Println("Starting server on :8080")
    log.Fatal(http.ListenAndServe(":8080", wrapped))
}
```

---

## Step 9: Build and Run

```bash
# Initialize module
go mod init learning-go

# Build
go build -o server ./cmd/server

# Run
./server

# Test
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice", "email": "alice@example.com"}'

curl http://localhost:8080/users

curl http://localhost:8080/users/user-20240324120000

curl -X DELETE http://localhost:8080/users/user-20240324120000
```

---

## Expected Output

```bash
# POST /users
$ curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice", "email": "alice@example.com"}'

{"id":"user-20240324120000","name":"Alice","email":"alice@example.com","status":"active","created_at":"2024-03-24T12:00:00Z","updated_at":"2024-03-24T12:00:00Z"}

# Console output (event log)
[EVENT] user.created: {UserID:user-20240324120000 Email:alice@example.com ...}
```

---

## Quick Reference

| Layer | File | Purpose |
|-------|------|---------|
| Model | `model/user.go` | Data structure |
| Repository | `repository/user.go` | Data access interface + implementation |
| Service | `service/user.go` | Business logic |
| Handler | `handler/user.go` | HTTP handling |
| Events | `events/` | Pub-sub system |
| Main | `cmd/server/main.go` | Wire everything together |

---

## Extensions

Try adding:
1. **Validation middleware** - Request validation
2. **Auth middleware** - Basic auth
3. **Pagination** - Add offset/limit to List
4. **PostgreSQL repo** - Swap in-memory for real DB
5. **Rate limiting** - Add backpressure
6. **Tests** - Unit test service with mock repo