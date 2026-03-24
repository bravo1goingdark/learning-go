# Pub-Sub Design

> Decouple components with events. Publishers don't know about subscribers.

---

## The Problem

Tight coupling makes systems fragile:

```go
// Tightly coupled: Service knows about everything
type UserService struct {
    repo     repository.UserRepository
    email    *EmailService      // Hard dependency
    logger   *Logger
    metrics  *Metrics
    cache    *CacheService
}

func (s *UserService) CreateUser(u *User) error {
    s.repo.Create(u)
    
    // What if we want to add more side effects?
    // - Send welcome email
    // - Update analytics
    // - Notify admin
    // - Clear cache
    // Every change requires modifying this code!
}
```

---

## The Solution: Pub-Sub

```
┌─────────────────────────────────────────────────────────────────┐
│                         Event Bus                              │
│                                                              │
│   Publish ──────────────────────────────────────────────►      │
│     │                                                        │
│     │         Subscribers:                                  │
│     ├──────────────► Logger                                 │
│     ├──────────────► Email Service                          │
│     ├──────────────► Metrics                                 │
│     └──────────────► Cache                                   │
│                                                              │
└─────────────────────────────────────────────────────────────────┘
```

Publishers emit events. Subscribers react. They don't know about each other.

---

## Event Definition

```go
// internal/events/events.go
package events

import "time"

// Event is the interface all events implement
type Event interface {
    Type() string
    Timestamp() time.Time
}

// UserCreatedEvent is emitted when a user is created
type UserCreatedEvent struct {
    UserID    string    `json:"user_id"`
    Email     string    `json:"email"`
    Name      string    `json:"name"`
    Timestamp time.Time `json:"timestamp"`
}

func (e UserCreatedEvent) Type() string {
    return "user.created"
}

func (e UserCreatedEvent) Timestamp() time.Time {
    return e.Timestamp
}

// UserDeletedEvent
type UserDeletedEvent struct {
    UserID    string    `json:"user_id"`
    Timestamp time.Time `json:"timestamp"`
}

func (e UserDeletedEvent) Type() string {
    return "user.deleted"
}

func (e UserDeletedEvent) Timestamp() time.Time {
    return e.Timestamp
}
```

---

## Event Bus Implementation

```go
// internal/events/bus.go
package events

import (
    "sync"
)

// Handler is the interface for event handlers
type Handler interface {
    Handle(event Event) error
}

// EventBus is the pub-sub mechanism
type EventBus struct {
    mu         sync.RWMutex
    subscribers map[string][]Handler
}

func New() *EventBus {
    return &EventBus{
        subscribers: make(map[string][]Handler),
    }
}

// Subscribe adds a handler for a specific event type
func (b *EventBus) Subscribe(eventType string, handler Handler) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

// Publish sends an event to all subscribers
func (b *EventBus) Publish(event Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    handlers, ok := b.subscribers[event.Type()]
    if !ok {
        return
    }

    // Fire and forget - don't block the publisher
    for _, handler := range handlers {
        go func(h Handler) {
            if err := h.Handle(event); err != nil {
                // Log error but don't fail
                // In production, use proper logging
            }
        }(handler)
    }
}
```

---

## Subscribers (Handlers)

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
    fmt.Printf("[LOG] Event: %s, Data: %+v\n", event.Type(), event)
    return nil
}
```

```go
// internal/events/handlers/email.go
package handlers

import (
    "fmt"
    "learning-go/internal/events"
)

type EmailHandler struct{}

func NewEmailHandler() *EmailHandler {
    return &EmailHandler{}
}

func (h *EmailHandler) Handle(event events.Event) error {
    switch e := event.(type) {
    case events.UserCreatedEvent:
        fmt.Printf("[EMAIL] Sending welcome to: %s\n", e.Email)
        // Send welcome email
    }
    return nil
}
```

```go
// internal/events/handlers/metrics.go
package handlers

import (
    "fmt"
    "learning-go/internal/events"
)

type MetricsHandler struct{}

func NewMetricsHandler() *MetricsHandler {
    return &MetricsHandler{}
}

func (h *MetricsHandler) Handle(event events.Event) error {
    switch event.Type() {
    case "user.created":
        fmt.Println("[METRICS] user.created counter++")
    case "user.deleted":
        fmt.Println("[METRICS] user.deleted counter++")
    }
    return nil
}
```

---

## Using the Event Bus in Service

```go
// internal/service/user.go
package service

import (
    "time"

    "learning-go/internal/events"
    "learning-go/internal/model"
    "learning-go/internal/repository"
)

type UserService struct {
    repo    repository.UserRepository
    eventBus *events.EventBus
}

func New(repo repository.UserRepository, eb *events.EventBus) *UserService {
    return &UserService{
        repo:    repo,
        eventBus: eb,
    }
}

func (s *UserService) CreateUser(name, email string) (*model.User, error) {
    user := &model.User{
        ID:        "user-" + time.Now().Format("20060102150405"),
        Name:      name,
        Email:     email,
        CreatedAt: time.Now(),
    }

    if err := s.repo.Create(user); err != nil {
        return nil, err
    }

    // Publish event - don't care who listens
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

func (s *UserService) DeleteUser(id string) error {
    if err := s.repo.Delete(id); err != nil {
        return err
    }

    if s.eventBus != nil {
        s.eventBus.Publish(events.UserDeletedEvent{
            UserID:    id,
            Timestamp: time.Now(),
        })
    }

    return nil
}
```

---

## Wiring Everything

```go
// cmd/api/main.go
package main

import (
    "net/http"

    "learning-go/internal/events"
    "learning-go/internal/events/handlers"
    "learning-go/internal/handler"
    "learning-go/internal/repository"
    "learning-go/internal/service"
)

func main() {
    // Create event bus
    eb := events.New()

    // Register subscribers
    eb.Subscribe("user.created", handlers.NewLoggerHandler())
    eb.Subscribe("user.created", handlers.NewEmailHandler())
    eb.Subscribe("user.created", handlers.NewMetricsHandler())
    eb.Subscribe("user.deleted", handlers.NewLoggerHandler())
    eb.Subscribe("user.deleted", handlers.NewMetricsHandler())

    // Create services with event bus
    userRepo := repository.NewInMemory()
    userSvc := service.New(userRepo, eb)

    // Create handler
    userHandler := handler.New(userSvc)

    // Routes
    http.HandleFunc("POST /users", userHandler.Create)
    http.HandleFunc("DELETE /users/{id}", userHandler.Delete)
}
```

---

## Event-Driven Flow

```
┌──────────┐    Create    ┌──────────┐    Publish    ┌──────────┐
│  Client  │──────────────▶│ Service  │──────────────▶│ EventBus │
└──────────┘              └──────────┘              └─────┬────┘
                                                            │
                ┌──────────────────┬──────────────────────┤
                ▼                  ▼                      ▼
          ┌──────────┐      ┌──────────┐          ┌──────────┐
          │  Logger  │      │  Email   │          │  Metrics  │
          └──────────┘      └──────────┘          └──────────┘
```

---

## Benefits

| Benefit | Description |
|---------|-------------|
| Decoupling | Components don't know about each other |
| Extensibility | Add new handlers without changing service |
| Scalability | Async processing, multiple subscribers |
| Testing | Test handlers independently |

---

## Common Pitfalls

1. **Over-pub-sub** - Not every action needs events
2. **Eventual consistency** - Async means delays
3. **Missing events** - Forgot to publish for subscribers
4. **No ordering** - Events may arrive out of order

---

## Next Steps

- [Retry + Circuit Breaker](12-retry-circuit-breaker.md) - Handle failures
- [Backpressure Strategies](13-backpressure-strategies.md) - Handle overload
- [Milestone Project](20-layered-http-service.md) - Build complete service