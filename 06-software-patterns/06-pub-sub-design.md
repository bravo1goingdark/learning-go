# Pub-Sub Design

> Decouple components with events. Publishers don't know about subscribers.

---

![Pub-Sub Design Overview](../assets/pub_sub.png)

---

## The Problem: Tight Coupling

When services directly depend on each other, every change ripples through the codebase:

```go
// BAD: UserService knows about everything
type UserService struct {
    repo     repository.UserRepository
    email    *EmailService      // Direct dependency
    logger   *Logger            // Direct dependency
    metrics  *Metrics           // Direct dependency
    cache    *CacheService      // Direct dependency
}

func (s *UserService) CreateUser(user *User) error {
    s.repo.Create(user)
    
    // Every new side effect requires changing this code
    s.email.SendWelcome(user.Email)    // ← Added later
    s.metrics.Increment("users.created") // ← Added later
    s.cache.Invalidate("user-list")      // ← Added later
    s.logger.Info("User created")        // ← Added later
    return nil
}
```

**Problems:**
- Adding a new side effect requires modifying `UserService`
- Testing requires mocking every dependency
- Removing a feature risks breaking others
- `UserService` is doing too many things

---

## The Solution: Pub-Sub (Event Bus)

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                          EVENT BUS (Pub-Sub)                              │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │                                                                           │
  │   ┌──────────────┐                                                       │
  │   │  Publisher    │                                                       │
  │   │  (UserService)│                                                       │
  │   └──────┬───────┘                                                       │
  │          │                                                                │
  │          │  Publish("user.created", event)                               │
  │          ▼                                                                │
  │   ╔════════════════════════════════════════════════════════════════╗     │
  │   ║                        EVENT BUS                                ║     │
  │   ╚══════════╤══════════════╤══════════════╤══════════════╤════════╝     │
  │              │              │              │              │               │
  │              ▼              ▼              ▼              ▼               │
  │       ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐      │
  │       │  Logger    │ │   Email    │ │  Metrics   │ │   Cache    │      │
  │       │  Handler   │ │  Handler   │ │  Handler   │ │  Handler   │      │
  │       │            │ │            │ │            │ │            │      │
  │       │ logs event │ │ sends      │ │ records    │ │ invalidates│      │
  │       │ to stdout  │ │ welcome    │ │ counters   │ │ user cache │      │
  │       └────────────┘ └────────────┘ └────────────┘ └────────────┘      │
  │                                                                           │
  │   Publisher does NOT know WHO listens.                                    │
  │   Handlers can be added/removed without touching publisher.              │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

**Benefits:**
- Add new handlers without touching the publisher
- Test handlers independently
- Remove features without code changes
- Clear separation of concerns

---

## Event Definitions

Define events as simple structs:

```go
// internal/events/events.go
package events

import "time"

// Event is the interface all domain events implement
type Event interface {
    Type() string
    Timestamp() time.Time
}

// ──── User Events ────

type UserCreatedEvent struct {
    UserID     string    `json:"user_id"`
    Email      string    `json:"email"`
    Name       string    `json:"name"`
    OccurredAt time.Time `json:"occurred_at"`
}

func (e UserCreatedEvent) Type() string      { return "user.created" }
func (e UserCreatedEvent) Timestamp() time.Time { return e.OccurredAt }

type UserUpdatedEvent struct {
    UserID     string    `json:"user_id"`
    Fields     []string  `json:"fields"` // Which fields changed
    OccurredAt time.Time `json:"occurred_at"`
}

func (e UserUpdatedEvent) Type() string      { return "user.updated" }
func (e UserUpdatedEvent) Timestamp() time.Time { return e.OccurredAt }

type UserDeletedEvent struct {
    UserID     string    `json:"user_id"`
    OccurredAt time.Time `json:"occurred_at"`
}

func (e UserDeletedEvent) Type() string      { return "user.deleted" }
func (e UserDeletedEvent) Timestamp() time.Time { return e.OccurredAt }

// ──── Order Events ────

type OrderCreatedEvent struct {
    OrderID    string    `json:"order_id"`
    UserID     string    `json:"user_id"`
    Total      float64   `json:"total"`
    OccurredAt time.Time `json:"occurred_at"`
}

func (e OrderCreatedEvent) Type() string      { return "order.created" }
func (e OrderCreatedEvent) Timestamp() time.Time { return e.OccurredAt }
```

---

## Event Bus Implementation

```go
// internal/events/bus.go
package events

import (
    "context"
    "log"
    "sync"
)

// Handler processes events
type Handler interface {
    Handle(ctx context.Context, event Event) error
}

// HandlerFunc allows using functions as handlers
type HandlerFunc func(ctx context.Context, event Event) error

func (f HandlerFunc) Handle(ctx context.Context, event Event) error {
    return f(ctx, event)
}

// EventBus routes events to handlers
type EventBus struct {
    mu          sync.RWMutex
    subscribers map[string][]Handler
    logger      *log.Logger
}

// New creates a new event bus
func New(logger *log.Logger) *EventBus {
    return &EventBus{
        subscribers: make(map[string][]Handler),
        logger:      logger,
    }
}

// Subscribe registers a handler for an event type
func (b *EventBus) Subscribe(eventType string, handler Handler) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

// SubscribeFunc registers a function handler for an event type
func (b *EventBus) SubscribeFunc(eventType string, fn func(ctx context.Context, event Event) error) {
    b.Subscribe(eventType, HandlerFunc(fn))
}

// Publish sends an event to all subscribers asynchronously
func (b *EventBus) Publish(ctx context.Context, event Event) {
    b.mu.RLock()
    handlers := b.subscribers[event.Type()]
    b.mu.RUnlock()

    if len(handlers) == 0 {
        return
    }

    // Fan out to all handlers
    var wg sync.WaitGroup
    for _, handler := range handlers {
        wg.Add(1)
        go func(h Handler) {
            defer wg.Done()
            if err := h.Handle(ctx, event); err != nil {
                b.logger.Printf("error handling %s: %v", event.Type(), err)
            }
        }(handler)
    }
    wg.Wait()
}

// PublishAsync sends an event to all subscribers without waiting
func (b *EventBus) PublishAsync(ctx context.Context, event Event) {
    b.mu.RLock()
    handlers := b.subscribers[event.Type()]
    b.mu.RUnlock()

    for _, handler := range handlers {
        go func(h Handler) {
            if err := h.Handle(ctx, event); err != nil {
                b.logger.Printf("error handling %s: %v", event.Type(), err)
            }
        }(handler)
    }
}
```

---

## Event Handlers (Subscribers)

### Logger Handler

```go
// internal/events/handlers/logger.go
package handlers

import (
    "context"
    "fmt"

    "myapp/internal/events"
)

type LoggerHandler struct {
    prefix string
}

func NewLoggerHandler(prefix string) *LoggerHandler {
    return &LoggerHandler{prefix: prefix}
}

func (h *LoggerHandler) Handle(ctx context.Context, event events.Event) error {
    fmt.Printf("[%s] Event: %s, Timestamp: %v\n",
        h.prefix,
        event.Type(),
        event.Timestamp().Format("2006-01-02 15:04:05"),
    )
    return nil
}
```

### Email Handler

```go
// internal/events/handlers/email.go
package handlers

import (
    "context"
    "fmt"

    "myapp/internal/events"
)

type EmailHandler struct {
    emailClient EmailClient
}

type EmailClient interface {
    SendWelcomeEmail(email, name string) error
    SendGoodbyeEmail(email string) error
}

func NewEmailHandler(client EmailClient) *EmailHandler {
    return &EmailHandler{emailClient: client}
}

func (h *EmailHandler) Handle(ctx context.Context, event events.Event) error {
    switch e := event.(type) {
    case events.UserCreatedEvent:
        fmt.Printf("Sending welcome email to %s (%s)\n", e.Name, e.Email)
        // return h.emailClient.SendWelcomeEmail(e.Email, e.Name)
        return nil

    case events.UserDeletedEvent:
        fmt.Printf("Sending goodbye email for user %s\n", e.UserID)
        // return h.emailClient.SendGoodbyeEmail(e.Email)
        return nil

    default:
        return nil // Ignore unknown events
    }
}
```

### Metrics Handler

```go
// internal/events/handlers/metrics.go
package handlers

import (
    "context"
    "fmt"

    "myapp/internal/events"
)

type MetricsHandler struct {
    metrics Metrics
}

type Metrics interface {
    Increment(name string)
    Observe(name string, value float64)
}

func NewMetricsHandler(m Metrics) *MetricsHandler {
    return &MetricsHandler{metrics: m}
}

func (h *MetricsHandler) Handle(ctx context.Context, event events.Event) error {
    switch event.Type() {
    case "user.created":
        fmt.Println("metrics: users.created +1")
    case "user.deleted":
        fmt.Println("metrics: users.deleted +1")
    case "order.created":
        fmt.Println("metrics: orders.created +1")
    }
    return nil
}
```

### Cache Handler

```go
// internal/events/handlers/cache.go
package handlers

import (
    "context"
    "fmt"

    "myapp/internal/events"
)

type CacheHandler struct {
    cache Cache
}

type Cache interface {
    Invalidate(key string) error
    InvalidatePattern(pattern string) error
}

func NewCacheHandler(cache Cache) *CacheHandler {
    return &CacheHandler{cache: cache}
}

func (h *CacheHandler) Handle(ctx context.Context, event events.Event) error {
    switch e := event.(type) {
    case events.UserCreatedEvent:
        fmt.Println("cache: invalidate user-list")
        // h.cache.Invalidate("user-list")
        return nil

    case events.UserUpdatedEvent:
        fmt.Printf("cache: invalidate user:%s\n", e.UserID)
        // h.cache.Invalidate("user:" + e.UserID)
        return nil

    case events.UserDeletedEvent:
        fmt.Printf("cache: invalidate user:%s and user-list\n", e.UserID)
        // h.cache.Invalidate("user:" + e.UserID)
        // h.cache.Invalidate("user-list")
        return nil

    default:
        return nil
    }
}
```

---

## Using Events in Service

The service **publishes events** without knowing who listens:

```go
// internal/service/user.go
package service

import (
    "context"
    "time"

    "myapp/internal/events"
    "myapp/internal/model"
    "myapp/internal/repository"
)

type UserService struct {
    repo     repository.UserRepository
    eventBus *events.EventBus
}

func NewUserService(repo repository.UserRepository, eb *events.EventBus) *UserService {
    return &UserService{
        repo:     repo,
        eventBus: eb,
    }
}

func (s *UserService) CreateUser(ctx context.Context, name, email string) (*model.User, error) {
    // Create user
    user := &model.User{
        ID:        generateID(),
        Name:      name,
        Email:     email,
        Status:    "active",
        CreatedAt: time.Now(),
    }

    if err := s.repo.Create(user); err != nil {
        return nil, err
    }

    // Publish event — service doesn't know or care who listens
    s.eventBus.PublishAsync(ctx, events.UserCreatedEvent{
        UserID:     user.ID,
        Email:      user.Email,
        Name:       user.Name,
        OccurredAt: time.Now(),
    })

    return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, id, name string) (*model.User, error) {
    user, err := s.repo.GetByID(id)
    if err != nil {
        return nil, err
    }

    user.Name = name
    user.UpdatedAt = time.Now()

    if err := s.repo.Update(user); err != nil {
        return nil, err
    }

    s.eventBus.PublishAsync(ctx, events.UserUpdatedEvent{
        UserID:     id,
        Fields:     []string{"name"},
        OccurredAt: time.Now(),
    })

    return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
    if err := s.repo.Delete(id); err != nil {
        return err
    }

    s.eventBus.PublishAsync(ctx, events.UserDeletedEvent{
        UserID:     id,
        OccurredAt: time.Now(),
    })

    return nil
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

    "myapp/internal/events"
    "myapp/internal/events/handlers"
    "myapp/internal/handler"
    "myapp/internal/repository"
    "myapp/internal/service"
)

func main() {
    logger := log.New(log.Writer(), "", log.LstdFlags)

    // 1. Create event bus
    eb := events.New(logger)

    // 2. Create and register handlers
    emailClient := NewRealEmailClient() // Your email implementation
    metricsClient := NewPrometheusMetrics()
    cacheClient := NewRedisCache()

    eb.Subscribe("user.created", handlers.NewLoggerHandler("APP"))
    eb.Subscribe("user.created", handlers.NewEmailHandler(emailClient))
    eb.Subscribe("user.created", handlers.NewMetricsHandler(metricsClient))
    eb.Subscribe("user.created", handlers.NewCacheHandler(cacheClient))

    eb.Subscribe("user.updated", handlers.NewLoggerHandler("APP"))
    eb.Subscribe("user.updated", handlers.NewCacheHandler(cacheClient))

    eb.Subscribe("user.deleted", handlers.NewLoggerHandler("APP"))
    eb.Subscribe("user.deleted", handlers.NewMetricsHandler(metricsClient))
    eb.Subscribe("user.deleted", handlers.NewCacheHandler(cacheClient))

    // 3. Create services with event bus
    userRepo := repository.NewUserPostgres(db)
    userService := service.NewUserService(userRepo, eb)

    // 4. Create handlers
    userHandler := handler.NewUserHandler(userService)

    // 5. Routes
    mux := http.NewServeMux()
    mux.HandleFunc("POST /users", userHandler.Create)
    mux.HandleFunc("GET /users/{id}", userHandler.Get)
    mux.HandleFunc("PUT /users/{id}", userHandler.Update)
    mux.HandleFunc("DELETE /users/{id}", userHandler.Delete)

    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

---

## Synchronous vs Asynchronous Publishing

| Method | Use When |
|--------|----------|
| `Publish` (sync) | All handlers must succeed (transactions) |
| `PublishAsync` (fire-and-forget) | Side effects are optional (logging, metrics) |

```go
// Sync: wait for all handlers
s.eventBus.Publish(ctx, event)

// Async: fire and forget
s.eventBus.PublishAsync(ctx, event)
```

---

## Benefits Summary

| Benefit | Explanation |
|---------|-------------|
| **Decoupling** | Service doesn't know about email, metrics, etc. |
| **Extensibility** | Add new handlers without touching service |
| **Testability** | Test each handler independently |
| **Scalability** | Can move to external message queue later |
| **Maintainability** | Clear separation of concerns |

---

## Quick Reference

| Concept | Purpose |
|---------|---------|
| Event | Something that happened (past tense) |
| Handler | Reacts to an event |
| Event Bus | Routes events to handlers |
| Subscribe | Register handler for event type |
| Publish | Send event to all subscribers |

---

## Common Pitfalls

1. **Over-pub-sub** — Not every action needs events
2. **Eventual consistency** — Async means delays between action and side effects
3. **Missing error handling** — Handlers can silently fail
4. **No ordering** — Events may arrive out of order
5. **Leaky abstraction** — Handlers shouldn't call back to publisher

---

## Next Steps

- [Retry + Circuit Breaker](07-retry-circuit-breaker.md) — Handle failures
- [Backpressure Strategies](08-backpressure-strategies.md) — Handle overload
- [Milestone Project](../projects/20-layered-http-service.md) — Build complete service