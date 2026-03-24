# Service Layer

> Where business logic lives. Orchestrates data flow and enforces rules.

---

## What Is a Service Layer?

The service layer sits between handlers (HTTP) and repositories (data access):

```
┌─────────────────────────────────────────────────────────────┐
│                         HTTP Request                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Handler                                                    │
│  - Decode request                                          │
│  - Validate input format                                   │
│  - Call service                                           │
│  - Encode response                                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Service         ◄── Business logic lives here             │
│  - Validation                                               │
│  - Business rules                                          │
│  - Orchestration                                           │
│  - Domain model creation                                   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Repository                                                 │
│  - Data persistence                                        │
│  - Query execution                                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                         Database                            │
└─────────────────────────────────────────────────────────────┘
```

---

## Service Responsibilities

```
┌─────────────────────────────────────────────────────────────┐
│                 Service Layer Responsibilities              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Validation (not format - that's handler)               │
│     - Does this operation make business sense?             │
│     - Can user perform this action?                        │
│                                                             │
│  2. Business Rules                                         │
│     - Order total calculation                              │
│     - Discount eligibility                                 │
│     - Inventory checking                                   │
│                                                             │
│  3. Orchestration                                         │
│     - Multiple repository calls                            │
│     - Transaction management                               │
│     - Event publishing                                     │
│                                                             │
│  4. Domain Model Creation                                  │
│     - Construct proper entities                            │
│     - Apply default values                                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Example: User Service

```go
// internal/service/user.go
package service

import (
    "errors"
    "time"

    "learning-go/internal/model"
    "learning-go/internal/repository"
)

var (
    ErrInvalidInput = errors.New("invalid input")
    ErrNotFound     = errors.New("not found")
)

type UserService struct {
    repo      repository.UserRepository
    eventBus  EventBus  // For pub-sub (later topic)
}

func New(repo repository.UserRepository, eb EventBus) *UserService {
    return &UserService{
        repo:     repo,
        eventBus: eb,
    }
}

func (s *UserService) CreateUser(name, email string) (*model.User, error) {
    // 1. Validation (business rules)
    if name == "" {
        return nil, errors.New("name is required")
    }

    if !isValidEmail(email) {
        return nil, errors.New("invalid email format")
    }

    // Check for duplicate
    users := s.repo.List()
    for _, u := range users {
        if u.Email == email {
            return nil, errors.New("email already exists")
        }
    }

    // 2. Domain model creation
    user := &model.User{
        ID:        generateID(),
        Name:      name,
        Email:     email,
        CreatedAt: time.Now(),
        Status:    model.StatusActive,
    }

    // 3. Persist
    if err := s.repo.Create(user); err != nil {
        return nil, err
    }

    // 4. Publish event (optional)
    if s.eventBus != nil {
        s.eventBus.Publish(UserCreatedEvent{User: user})
    }

    return user, nil
}

func (s *UserService) GetUser(id string) (*model.User, error) {
    user, err := s.repo.GetByID(id)
    if errors.Is(err, repository.ErrNotFound) {
        return nil, ErrNotFound
    }
    return user, err
}

func (s *UserService) UpdateUser(id, name string) (*model.User, error) {
    user, err := s.repo.GetByID(id)
    if err != nil {
        return nil, err
    }

    // Business rule: can only update name
    user.Name = name
    user.UpdatedAt = time.Now()

    if err := s.repo.Update(user); err != nil {
        return nil, err
    }

    return user, nil
}

func (s *UserService) DeleteUser(id string) error {
    if err := s.repo.Delete(id); err != nil {
        return err
    }

    if s.eventBus != nil {
        s.eventBus.Publish(UserDeletedEvent{UserID: id})
    }

    return nil
}

// Helpers
func isValidEmail(email string) bool {
    // Simple validation - in production use proper regex
    return email != "" && len(email) > 3
}

func generateID() string {
    return "user-" + time.Now().Format("20060102150405")
}
```

---

## Handler Calls Service

```go
// internal/handler/user.go
package handler

import (
    "encoding/json"
    "net/http"

    "learning-go/internal/model"
    "learning-go/internal/service"
)

type UserHandler struct {
    svc *service.UserService
}

func New(svc *service.UserService) *UserHandler {
    return &UserHandler{svc: svc}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    // 1. Decode request (handler responsibility)
    var req struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    // 2. Call service (validation + business logic)
    user, err := h.svc.CreateUser(req.Name, req.Email)
    if err != nil {
        // Map error to HTTP status
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // 3. Encode response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")

    user, err := h.svc.GetUser(id)
    if err != nil {
        if err == service.ErrNotFound {
            http.Error(w, "user not found", http.StatusNotFound)
            return
        }
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(user)
}
```

---

## Orchestration: Complex Operations

```go
// Example: Order service with orchestration
func (s *OrderService) CreateOrder(userID string, items []OrderItem) (*Order, error) {
    // 1. Validate user exists
    user, err := s.userRepo.GetByID(userID)
    if err != nil {
        return nil, errors.New("user not found")
    }

    // 2. Check inventory
    for _, item := range items {
        available, err := s.inventoryRepo.Check(item.ProductID, item.Quantity)
        if err != nil || !available {
            return nil, errors.New("product unavailable")
        }
    }

    // 3. Calculate totals
    subtotal := s.calculateSubtotal(items)
    tax := s.calculateTax(subtotal)
    shipping := s.calculateShipping(items)
    total := subtotal + tax + shipping

    // 4. Create order
    order := &Order{
        ID:          generateOrderID(),
        UserID:      userID,
        Items:       items,
        Subtotal:    subtotal,
        Tax:         tax,
        Shipping:    shipping,
        Total:       total,
        Status:      StatusPending,
        CreatedAt:   time.Now(),
    }

    if err := s.orderRepo.Create(order); err != nil {
        return nil, err
    }

    // 5. Reserve inventory
    for _, item := range items {
        s.inventoryRepo.Reserve(item.ProductID, item.Quantity)
    }

    // 6. Publish event
    s.eventBus.Publish(OrderCreatedEvent{Order: order})

    return order, nil
}
```

---

## Error Handling Strategy

```go
// Define service-specific errors
var (
    ErrInvalidInput    = errors.New("invalid input")
    ErrNotFound        = errors.New("not found")
    ErrAlreadyExists   = errors.New("already exists")
    ErrUnauthorized    = errors.New("unauthorized")
    ErrForbidden       = errors.New("forbidden")
)

// Handler maps to HTTP status
func mapError(err error) (int, string) {
    switch {
    case errors.Is(err, ErrNotFound):
        return http.StatusNotFound, err.Error()
    case errors.Is(err, ErrAlreadyExists):
        return http.StatusConflict, err.Error()
    case errors.Is(err, ErrUnauthorized):
        return http.StatusUnauthorized, err.Error()
    case errors.Is(err, ErrForbidden):
        return http.StatusForbidden, err.Error()
    default:
        return http.StatusInternalServerError, "internal error"
    }
}
```

---

## Quick Reference

| Layer | Responsibility | Examples |
|-------|-----------------|-----------|
| Handler | HTTP parsing, response encoding | Decode JSON, set headers |
| Service | Business logic, validation | Calculate totals, check rules |
| Repository | Data persistence | SQL queries, cache ops |

---

## Common Pitfalls

1. **Business logic in handler** - Should be in service
2. **Data access in handler** - Should go through service → repo
3. **Too much code in service** - Extract to domain models
4. **No transactions** - Complex operations may need atomicity

---

## Next Steps

- [Dependency Injection](09-dependency-injection.md) - Connect the layers
- [Clean Architecture](10-clean-architecture.md) - Boundaries and rules
- [Milestone Project](20-layered-http-service.md) - Build complete service