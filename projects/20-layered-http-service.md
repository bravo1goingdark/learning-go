# Layered HTTP Service Project

> Build a complete CRUD API using all software patterns learned.

---

## Project Overview

Build a user management HTTP service with:

- **Repository Pattern** - In-memory data storage with interface
- **Service Layer** - Business logic and validation
- **Dependency Injection** - Clean component wiring
- **Pub-Sub** - Event-driven notifications
- **HTTP Handlers** - RESTful API endpoints

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

```
06-software-patterns-project/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── model/
│   │   └── user.go
│   ├── repository/
│   │   └── user.go
│   ├── service/
│   │   └── user.go
│   ├── handler/
│   │   └── user.go
│   ├── events/
│   │   ├── bus.go
│   │   └── handlers/
│   │       └── logger.go
│   └── middleware/
│       └── middleware.go
├── go.mod
└── README.md
```

---

## Step 2: Model

```go
// internal/model/user.go
package model

import "time"

type User struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

const (
    StatusActive   = "active"
    StatusInactive = "inactive"
)
```

---

## Step 3: Repository (Interface + Implementation)

```go
// internal/repository/user.go
package repository

import "learning-go/internal/model"

type UserRepository interface {
    Create(user *model.User) error
    GetByID(id string) (*model.User, error)
    Update(user *model.User) error
    Delete(id string) error
    List() []*model.User
}

var (
    ErrNotFound = model.ErrNotFound
)
```

```go
// internal/repository/memory.go
package repository

import (
    "errors"
    "sync"

    "learning-go/internal/model"
)

type userRepository struct {
    mu   sync.RWMutex
    data map[string]*model.User
}

func NewInMemory() UserRepository {
    return &userRepository{
        data: make(map[string]*model.User),
    }
}

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