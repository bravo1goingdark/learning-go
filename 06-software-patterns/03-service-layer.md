# Service Layer

> Where business logic lives. Orchestrates data flow and enforces rules.

---

![Service Layer Overview](../assets/service_layer.png)

---

## What Is a Service Layer?

The service layer is the **heart of your application** — it contains all business logic and orchestrates data operations.

```
  ┌──────────────────────────────────────────────────────────────────────────┐
  │                          THREE-LAYER ARCHITECTURE                         │
  ├──────────────────────────────────────────────────────────────────────────┤
  │                                                                           │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │                         HTTP REQUEST                               │  │
  │   └───────────────────────────────────┬───────────────────────────────┘  │
  │                                       │                                    │
  │                                       ▼                                    │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  HANDLER LAYER (Protocol)                                         │  │
  │   │                                                                    │  │
  │   │   ✓ Parse request  (JSON/protobuf → Go struct)                   │  │
  │   │   ✓ Validate FORMAT (missing fields, bad types)                  │  │
  │   │   ✓ Call service method                                           │  │
  │   │   ✓ Map errors → HTTP status codes                               │  │
  │   │   ✓ Encode response (Go struct → JSON)                           │  │
  │   │   ✗ Does NOT contain business logic                               │  │
  │   └───────────────────────────────────┬───────────────────────────────┘  │
  │                                       │                                    │
  │                                       ▼                                    │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  SERVICE LAYER (Domain)             ◄── BUSINESS LOGIC LIVES HERE │  │
  │   │                                                                    │  │
  │   │   ✓ Validate business rules (does this operation make sense?)    │  │
  │   │   ✓ Enforce constraints (is user allowed to do this?)            │  │
  │   │   ✓ Calculate derived values (totals, scores, status)            │  │
  │   │   ✓ Orchestrate multiple repositories                            │  │
  │   │   ✓ Publish domain events                                         │  │
  │   │   ✗ Does NOT know about HTTP, JSON, or databases                 │  │
  │   └───────────────────────────────────┬───────────────────────────────┘  │
  │                                       │                                    │
  │                                       ▼                                    │
  │   ┌───────────────────────────────────────────────────────────────────┐  │
  │   │  REPOSITORY LAYER (Persistence)                                   │  │
  │   │                                                                    │  │
  │   │   ✓ Execute SQL queries                                           │  │
  │   │   ✓ Read/write to database                                        │  │
  │   │   ✓ Return domain models                                          │  │
  │   │   ✗ Does NOT contain business logic                               │  │
  │   └───────────────────────────────────┬───────────────────────────────┘  │
  │                                       │                                    │
  │                                       ▼                                    │
  │                              ┌──────────────┐                             │
  │                              │   DATABASE   │                             │
  │                              └──────────────┘                             │
  │                                                                           │
  └──────────────────────────────────────────────────────────────────────────┘
```

---

## Service Responsibilities (Detailed)

### 1. Business Validation

Check if an operation **makes sense** in your business domain:

```go
func (s *OrderService) CreateOrder(userID string, items []OrderItem) (*Order, error) {
    // Handler already checked JSON is valid and fields exist.
    // Service checks business rules:
    
    // Rule: Can't create empty order
    if len(items) == 0 {
        return nil, errors.New("order must have at least one item")
    }
    
    // Rule: Can't order more than 100 items
    if len(items) > 100 {
        return nil, errors.New("maximum 100 items per order")
    }
    
    // Rule: Each item must have positive quantity
    for _, item := range items {
        if item.Quantity <= 0 {
            return nil, errors.New("quantity must be positive")
        }
    }
    
    // Rule: User must exist and be active
    user, err := s.userRepo.GetByID(userID)
    if err != nil {
        return nil, ErrUserNotFound
    }
    if user.Status != "active" {
        return nil, errors.New("user account is suspended")
    }
    
    // ...
}
```

### 2. Business Rules Enforcement

Enforce constraints that **require looking at data**:

```go
func (s *TransferService) Transfer(fromID, toID string, amount float64) error {
    // Rule: Can't transfer to yourself
    if fromID == toID {
        return errors.New("cannot transfer to self")
    }
    
    // Rule: Must have sufficient balance (needs data from repo)
    balance, err := s.accountRepo.GetBalance(fromID)
    if err != nil {
        return err
    }
    if balance < amount {
        return ErrInsufficientFunds
    }
    
    // Rule: Daily limit (needs to check today's transfers)
    todayTransferred, err := s.transferRepo.GetDailyTotal(fromID)
    if err != nil {
        return err
    }
    if todayTransferred + amount > 10000 {
        return ErrDailyLimitExceeded
    }
    
    // Perform the transfer (orchestration)
    return s.txManager.Do(context.Background(), func(tx Transaction) error {
        if err := tx.Debit(fromID, amount); err != nil {
            return err
        }
        if err := tx.Credit(toID, amount); err != nil {
            return err
        }
        return tx.RecordTransfer(fromID, toID, amount)
    })
}
```

### 3. Orchestration

Coordinate multiple repository calls:

```go
func (s *CheckoutService) Checkout(cartID string) (*Order, error) {
    // 1. Get cart
    cart, err := s.cartRepo.GetByID(cartID)
    if err != nil {
        return nil, ErrCartNotFound
    }
    
    // 2. Validate all items are available
    for _, item := range cart.Items {
        available, err := s.inventoryRepo.CheckStock(item.ProductID, item.Quantity)
        if err != nil {
            return nil, err
        }
        if !available {
            return nil, fmt.Errorf("product %s is out of stock", item.ProductID)
        }
    }
    
    // 3. Create order from cart
    order := s.createOrderFromCart(cart)
    
    // 4. Persist order + reserve inventory (atomic)
    if err := s.txManager.Do(ctx, func(tx Transaction) error {
        if err := tx.CreateOrder(order); err != nil {
            return err
        }
        for _, item := range order.Items {
            if err := tx.ReserveStock(item.ProductID, item.Quantity); err != nil {
                return err
            }
        }
        return nil
    }); err != nil {
        return nil, err
    }
    
    // 5. Publish event for other services
    s.eventBus.Publish(OrderCreatedEvent{
        OrderID: order.ID,
        UserID:  order.UserID,
        Total:   order.Total,
    })
    
    // 6. Clear the cart
    s.cartRepo.Delete(cartID)
    
    return order, nil
}
```

### 4. Domain Model Creation

Create entities with proper defaults:

```go
func (s *UserService) CreateUser(name, email string) (*model.User, error) {
    // Validation
    if name == "" || email == "" {
        return nil, ErrInvalidInput
    }
    
    // Check uniqueness
    existing, _ := s.userRepo.GetByEmail(email)
    if existing != nil {
        return nil, ErrEmailAlreadyExists
    }
    
    // Create domain model with business defaults
    user := &model.User{
        ID:        uuid.New().String(),
        Name:      name,
        Email:     strings.ToLower(email),  // Normalize
        Status:    model.StatusPending,     // Business rule: new users start pending
        Role:      model.RoleUser,          // Business rule: default role
        CreatedAt: time.Now(),
    }
    
    // Persist
    if err := s.userRepo.Create(user); err != nil {
        return nil, err
    }
    
    return user, nil
}
```

---

## Full Service Example

```go
// internal/service/user.go
package service

import (
    "context"
    "errors"
    "strings"
    "time"

    "myapp/internal/events"
    "myapp/internal/model"
    "myapp/internal/repository"
)

// Service-level errors (domain errors)
var (
    ErrInvalidInput       = errors.New("invalid input")
    ErrNotFound           = errors.New("not found")
    ErrEmailAlreadyExists = errors.New("email already exists")
    ErrUserSuspended      = errors.New("user account is suspended")
    ErrUnauthorized       = errors.New("unauthorized")
)

// UserService handles user-related business logic
type UserService struct {
    repo     repository.UserRepository
    eventBus *events.EventBus
}

// NewUserService creates a new user service
func NewUserService(repo repository.UserRepository, eb *events.EventBus) *UserService {
    return &UserService{
        repo:     repo,
        eventBus: eb,
    }
}

// CreateUser validates input and creates a new user
func (s *UserService) CreateUser(ctx context.Context, name, email string) (*model.User, error) {
    // 1. Business validation
    name = strings.TrimSpace(name)
    email = strings.TrimSpace(strings.ToLower(email))
    
    if name == "" {
        return nil, ErrInvalidInput
    }
    if email == "" || !strings.Contains(email, "@") {
        return nil, ErrInvalidInput
    }
    
    // 2. Check uniqueness
    existing, err := s.repo.List(repository.UserFilter{Email: email})
    if err != nil {
        return nil, err
    }
    if len(existing) > 0 {
        return nil, ErrEmailAlreadyExists
    }
    
    // 3. Create domain model
    user := &model.User{
        ID:        generateID(),
        Name:      name,
        Email:     email,
        Status:    model.StatusActive,
        CreatedAt: time.Now(),
    }
    
    // 4. Persist
    if err := s.repo.Create(user); err != nil {
        return nil, err
    }
    
    // 5. Publish event
    s.eventBus.Publish(events.UserCreatedEvent{
        UserID:    user.ID,
        Email:     user.Email,
        Name:      user.Name,
        OccurredAt: time.Now(),
    })
    
    return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id string) (*model.User, error) {
    user, err := s.repo.GetByID(id)
    if errors.Is(err, repository.ErrNotFound) {
        return nil, ErrNotFound
    }
    return user, err
}

// UpdateUser modifies user information
func (s *UserService) UpdateUser(ctx context.Context, id, name string) (*model.User, error) {
    // 1. Fetch existing user
    user, err := s.repo.GetByID(id)
    if err != nil {
        return nil, err
    }
    
    // 2. Business rule: can't update suspended users
    if user.Status == model.StatusSuspended {
        return nil, ErrUserSuspended
    }
    
    // 3. Update
    user.Name = strings.TrimSpace(name)
    user.UpdatedAt = time.Now()
    
    // 4. Persist
    if err := s.repo.Update(user); err != nil {
        return nil, err
    }
    
    // 5. Publish event
    s.eventBus.Publish(events.UserUpdatedEvent{
        UserID:    user.ID,
        Name:      user.Name,
        OccurredAt: time.Now(),
    })
    
    return user, nil
}

// SuspendUser sets user status to suspended (business operation)
func (s *UserService) SuspendUser(ctx context.Context, id, reason string) error {
    // 1. Fetch user
    user, err := s.repo.GetByID(id)
    if err != nil {
        return err
    }
    
    // 2. Business rule: can't suspend admins
    if user.Role == model.RoleAdmin {
        return errors.New("cannot suspend admin users")
    }
    
    // 3. Update status
    user.Status = model.StatusSuspended
    user.UpdatedAt = time.Now()
    
    // 4. Persist
    if err := s.repo.Update(user); err != nil {
        return err
    }
    
    // 5. Publish event
    s.eventBus.Publish(events.UserSuspendedEvent{
        UserID:    id,
        Reason:    reason,
        OccurredAt: time.Now(),
    })
    
    return nil
}

// DeleteUser removes a user (soft delete)
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
    // 1. Check user exists
    _, err := s.repo.GetByID(id)
    if err != nil {
        return err
    }
    
    // 2. Soft delete (set status)
    if err := s.repo.SoftDelete(id); err != nil {
        return err
    }
    
    // 3. Publish event
    s.eventBus.Publish(events.UserDeletedEvent{
        UserID:    id,
        OccurredAt: time.Now(),
    })
    
    return nil
}

// ListUsers returns paginated list of users
func (s *UserService) ListUsers(ctx context.Context, filter repository.UserFilter) ([]*model.User, int, error) {
    users, err := s.repo.List(filter)
    if err != nil {
        return nil, 0, err
    }
    
    count, err := s.repo.Count()
    if err != nil {
        return nil, 0, err
    }
    
    return users, count, nil
}
```

---

## Handler → Service Interaction

The handler's ONLY job is HTTP — no business logic:

```go
// internal/handler/user.go
package handler

import (
    "encoding/json"
    "errors"
    "net/http"
    "myapp/internal/service"
)

type UserHandler struct {
    svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
    return &UserHandler{svc: svc}
}

// Create handles POST /users
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request (handler job)
    var req struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }
    
    // 2. Call service (business logic is in service)
    user, err := h.svc.CreateUser(r.Context(), req.Name, req.Email)
    if err != nil {
        // 3. Map domain errors to HTTP status codes
        status := mapServiceError(err)
        http.Error(w, err.Error(), status)
        return
    }
    
    // 4. Send response (handler job)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}

// Get handles GET /users/{id}
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    
    user, err := h.svc.GetUser(r.Context(), id)
    if err != nil {
        status := mapServiceError(err)
        http.Error(w, err.Error(), status)
        return
    }
    
    json.NewEncoder(w).Encode(user)
}

// Update handles PUT /users/{id}
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    
    var req struct {
        Name string `json:"name"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }
    
    user, err := h.svc.UpdateUser(r.Context(), id, req.Name)
    if err != nil {
        status := mapServiceError(err)
        http.Error(w, err.Error(), status)
        return
    }
    
    json.NewEncoder(w).Encode(user)
}

// mapServiceError converts domain errors to HTTP status codes
func mapServiceError(err error) int {
    switch {
    case errors.Is(err, service.ErrNotFound):
        return http.StatusNotFound
    case errors.Is(err, service.ErrEmailAlreadyExists):
        return http.StatusConflict
    case errors.Is(err, service.ErrUserSuspended):
        return http.StatusForbidden
    case errors.Is(err, service.ErrUnauthorized):
        return http.StatusUnauthorized
    case errors.Is(err, service.ErrInvalidInput):
        return http.StatusBadRequest
    default:
        return http.StatusInternalServerError
    }
}
```

---

## When Logic Belongs in Service vs Handler

| Logic | Service or Handler? | Why |
|-------|---------------------|-----|
| Parse JSON request | Handler | Protocol concern |
| Check required fields | Handler | Request format |
| Validate email format | Both | Format = handler, business rules = service |
| Check if email exists | Service | Requires data access |
| Calculate order total | Service | Business rule |
| Set HTTP status code | Handler | Protocol concern |
| Encode JSON response | Handler | Protocol concern |
| Publish event | Service | Business concern |
| Orchestrate multiple repos | Service | Business concern |

---

## Service Dependencies

Services often depend on multiple repositories:

```go
type OrderService struct {
    orderRepo     repository.OrderRepository
    userRepo      repository.UserRepository
    inventoryRepo repository.InventoryRepository
    paymentRepo   repository.PaymentRepository
    eventBus      *events.EventBus
    txManager     repository.TxManager
}

func NewOrderService(
    orderRepo repository.OrderRepository,
    userRepo repository.UserRepository,
    inventoryRepo repository.InventoryRepository,
    paymentRepo repository.PaymentRepository,
    eb *events.EventBus,
    tx repository.TxManager,
) *OrderService {
    return &OrderService{
        orderRepo:     orderRepo,
        userRepo:      userRepo,
        inventoryRepo: inventoryRepo,
        paymentRepo:   paymentRepo,
        eventBus:      eb,
        txManager:     tx,
    }
}
```

---

## Quick Reference

| Concept | Where It Goes |
|---------|---------------|
| JSON parsing | Handler |
| Business validation | Service |
| Data access | Repository |
| Error → HTTP mapping | Handler |
| Event publishing | Service |
| Transaction coordination | Service |

---

## Common Pitfalls

1. **SQL in handler** — Business logic mixed with HTTP
2. **HTTP in service** — Service should be protocol-agnostic
3. **Giant service** — One service doing everything
4. **No error mapping** — Using HTTP status codes in service
5. **Missing transactions** — Multi-step operations without atomicity

---

## Next Steps

- [Dependency Injection](04-dependency-injection.md) — How to wire services
- [Clean Architecture](05-clean-architecture.md) — Layer boundaries
- [Pub-Sub Design](06-pub-sub-design.md) — Event-driven decoupling