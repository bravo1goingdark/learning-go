# Repository Pattern

> Abstract data access. Swap databases without changing business logic.

---

## What is a Repository?

A repository is an **abstraction layer** between your business logic and your data storage. It hides the details of how data is fetched and stored.

Think of it as a **collection-like interface** to your database:

```
┌─────────────────────────────────────────────────────────────┐
│  Service thinks it's working with a simple collection:     │
│                                                              │
│    user, err := repo.GetByID("user-123")                   │
│    users, err := repo.List()                                │
│    err := repo.Create(user)                                 │
│                                                              │
│  But behind the scenes:                                     │
│    - SQL queries                                             │
│    - Connection pooling                                      │
│    - Transaction management                                  │
│    - Caching                                                 │
└─────────────────────────────────────────────────────────────┘
```

---

## The Problem Without Repository

```go
// BAD: SQL in service code
func (s *UserService) GetUser(id string) (*User, error) {
    var user User
    err := s.db.QueryRow(
        "SELECT id, name, email FROM users WHERE id = $1", id,
    ).Scan(&user.ID, &user.Name, &user.Email)
    
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    return &user, err
}

// Now this service:
// - Cannot be tested without a real database
// - Cannot switch to Redis/MongoDB easily
// - Has SQL scattered across multiple services
// - Violates single responsibility
```

---

## The Solution: Interface + Implementation

```
┌─────────────────────────────────────────────────────────────┐
│                      Service Layer                          │
│                                                              │
│  func (s *UserService) GetUser(id string) (*User, error) { │
│      return s.repo.GetByID(id)  // Just a method call!     │
│  }                                                          │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ uses interface
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                  Repository Interface                        │
│                                                              │
│  type UserRepository interface {                            │
│      Create(user *User) error                               │
│      GetByID(id string) (*User, error)                      │
│      Update(user *User) error                               │
│      Delete(id string) error                                │
│      List(filter Filter) ([]*User, error)                   │
│      ListByEmail(email string) ([]*User, error)             │
│      Count() (int, error)                                    │
│  }                                                           │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ implemented by
                           ▼
┌─────────────────────────────────────────────────────────────┐
│              Concrete Implementations                        │
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ PostgreSQL  │  │  In-Memory  │  │    Mock     │        │
│  │  (prod)     │  │  (testing)  │  │  (testing)  │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                              │
│  All implement the same interface                           │
└─────────────────────────────────────────────────────────────┘
```

---

## Defining the Interface

Place the interface in the **same package** as where it's used (service layer):

```go
// internal/repository/user.go
package repository

import "myapp/internal/model"

// UserRepository defines how we interact with user data.
// The service depends on this interface, NOT on any implementation.
type UserRepository interface {
    // Create inserts a new user. Returns ErrAlreadyExists if ID exists.
    Create(user *model.User) error

    // GetByID returns a user by ID. Returns ErrNotFound if not found.
    GetByID(id string) (*model.User, error)

    // Update modifies an existing user. Returns ErrNotFound if not found.
    Update(user *model.User) error

    // Delete removes a user. Returns ErrNotFound if not found.
    Delete(id string) error

    // List returns all users matching the filter.
    List(filter UserFilter) ([]*model.User, error)

    // Count returns the total number of users.
    Count() (int, error)
}

// UserFilter allows filtering users
type UserFilter struct {
    Status string // "active", "inactive", or empty for all
    Limit  int
    Offset int
}
```

---

## In-Memory Implementation

Perfect for **testing** and **development**:

```go
// internal/repository/user_memory.go
package repository

import (
    "errors"
    "sync"
    "time"

    "myapp/internal/model"
)

type userMemoryRepository struct {
    mu    sync.RWMutex
    users map[string]*model.User
}

func NewUserMemory() UserRepository {
    return &userMemoryRepository{
        users: make(map[string]*model.User),
    }
}

func (r *userMemoryRepository) Create(user *model.User) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Check for duplicate
    if _, exists := r.users[user.ID]; exists {
        return ErrAlreadyExists
    }

    // Set timestamps
    now := time.Now()
    user.CreatedAt = now
    user.UpdatedAt = now

    // Store
    r.users[user.ID] = user
    return nil
}

func (r *userMemoryRepository) GetByID(id string) (*model.User, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    user, ok := r.users[id]
    if !ok {
        return nil, ErrNotFound
    }

    // Return a copy to prevent mutations
    copy := *user
    return &copy, nil
}

func (r *userMemoryRepository) Update(user *model.User) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.users[user.ID]; !exists {
        return ErrNotFound
    }

    user.UpdatedAt = time.Now()
    r.users[user.ID] = user
    return nil
}

func (r *userMemoryRepository) Delete(id string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.users[id]; !exists {
        return ErrNotFound
    }

    delete(r.users, id)
    return nil
}

func (r *userMemoryRepository) List(filter UserFilter) ([]*model.User, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var result []*model.User

    for _, user := range r.users {
        // Apply filter
        if filter.Status != "" && user.Status != filter.Status {
            continue
        }

        copy := *user
        result = append(result, &copy)

        // Apply limit
        if filter.Limit > 0 && len(result) >= filter.Limit {
            break
        }
    }

    return result, nil
}

func (r *userMemoryRepository) Count() (int, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    return len(r.users), nil
}
```

---

## PostgreSQL Implementation

For **production**:

```go
// internal/repository/user_postgres.go
package repository

import (
    "database/sql"
    "errors"
    "strings"

    "myapp/internal/model"
)

type userPostgresRepository struct {
    db *sql.DB
}

func NewUserPostgres(db *sql.DB) UserRepository {
    return &userPostgresRepository{db: db}
}

func (r *userPostgresRepository) Create(user *model.User) error {
    query := `
        INSERT INTO users (id, name, email, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `
    _, err := r.db.Exec(
        query,
        user.ID, user.Name, user.Email, user.Status,
        user.CreatedAt, user.UpdatedAt,
    )
    if err != nil {
        if strings.Contains(err.Error(), "duplicate key") {
            return ErrAlreadyExists
        }
        return err
    }
    return nil
}

func (r *userPostgresRepository) GetByID(id string) (*model.User, error) {
    query := `
        SELECT id, name, email, status, created_at, updated_at
        FROM users
        WHERE id = $1
    `
    var user model.User
    err := r.db.QueryRow(query, id).Scan(
        &user.ID, &user.Name, &user.Email, &user.Status,
        &user.CreatedAt, &user.UpdatedAt,
    )
    if errors.Is(err, sql.ErrNoRows) {
        return nil, ErrNotFound
    }
    return &user, err
}

func (r *userPostgresRepository) Update(user *model.User) error {
    query := `
        UPDATE users
        SET name = $2, email = $3, status = $4, updated_at = $5
        WHERE id = $1
    `
    result, err := r.db.Exec(
        query,
        user.ID, user.Name, user.Email, user.Status, user.UpdatedAt,
    )
    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }
    if rows == 0 {
        return ErrNotFound
    }
    return nil
}

func (r *userPostgresRepository) Delete(id string) error {
    result, err := r.db.Exec("DELETE FROM users WHERE id = $1", id)
    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }
    if rows == 0 {
        return ErrNotFound
    }
    return nil
}

func (r *userPostgresRepository) List(filter UserFilter) ([]*model.User, error) {
    query := "SELECT id, name, email, status, created_at, updated_at FROM users"
    args := []interface{}{}

    if filter.Status != "" {
        query += " WHERE status = $1"
        args = append(args, filter.Status)
    }

    query += " ORDER BY created_at DESC"

    if filter.Limit > 0 {
        query += " LIMIT $%d"
        query = fmt.Sprintf(query, len(args)+1)
        args = append(args, filter.Limit)
    }

    rows, err := r.db.Query(query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var users []*model.User
    for rows.Next() {
        var user model.User
        if err := rows.Scan(
            &user.ID, &user.Name, &user.Email, &user.Status,
            &user.CreatedAt, &user.UpdatedAt,
        ); err != nil {
            return nil, err
        }
        users = append(users, &user)
    }

    return users, rows.Err()
}

func (r *userPostgresRepository) Count() (int, error) {
    var count int
    err := r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
    return count, err
}
```

---

## Sentinel Errors

Define errors **with the interface**, not the implementation:

```go
// internal/repository/errors.go
package repository

import "errors"

// These errors are returned by ALL repository implementations
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrConflict      = errors.New("conflict") // optimistic locking
)
```

Then service code can check errors **without knowing the implementation**:

```go
func (s *UserService) GetUser(id string) (*User, error) {
    user, err := s.repo.GetByID(id)
    if errors.Is(err, repository.ErrNotFound) {
        return nil, service.ErrUserNotFound  // Service-level error
    }
    return user, err
}
```

---

## Testing with Mocks

The interface makes testing **fast and isolated**:

```go
// internal/service/user_test.go
package service_test

import (
    "testing"
    "myapp/internal/repository"
    "myapp/internal/service"
)

// Mock implements the interface for testing
type mockUserRepo struct {
    users map[string]*model.User
}

func newMock() *mockUserRepo {
    return &mockUserRepo{
        users: make(map[string]*model.User),
    }
}

func (m *mockUserRepo) Create(user *model.User) error {
    if _, exists := m.users[user.ID]; exists {
        return repository.ErrAlreadyExists
    }
    m.users[user.ID] = user
    return nil
}

func (m *mockUserRepo) GetByID(id string) (*model.User, error) {
    user, ok := m.users[id]
    if !ok {
        return nil, repository.ErrNotFound
    }
    return user, nil
}

func (m *mockUserRepo) Update(user *model.User) error {
    m.users[user.ID] = user
    return nil
}

func (m *mockUserRepo) Delete(id string) error {
    delete(m.users, id)
    return nil
}

func (m *mockUserRepo) List(filter repository.UserFilter) ([]*model.User, error) {
    var users []*model.User
    for _, u := range m.users {
        users = append(users, u)
    }
    return users, nil
}

func (m *mockUserRepo) Count() (int, error) {
    return len(m.users), nil
}

// Tests
func TestCreateUser(t *testing.T) {
    repo := newMock()
    svc := service.NewUserService(repo)

    user, err := svc.CreateUser("Alice", "alice@example.com")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if user.Name != "Alice" {
        t.Errorf("expected Alice, got %s", user.Name)
    }
}

func TestGetUser_NotFound(t *testing.T) {
    repo := newMock()
    svc := service.NewUserService(repo)

    _, err := svc.GetUser("nonexistent")
    if err == nil {
        t.Fatal("expected error, got nil")
    }
}
```

---

## Transaction Support

For operations that need multiple writes, add transaction support:

```go
// internal/repository/transaction.go
package repository

import "context"

// TxManager manages database transactions
type TxManager interface {
    Do(ctx context.Context, fn func(tx Transaction) error) error
}

// Transaction represents a database transaction
type Transaction interface {
    UserRepository
    OrderRepository
    // ... other repositories that need transactions
}
```

Usage in service:

```go
func (s *OrderService) CreateOrder(order *Order) error {
    return s.txManager.Do(context.Background(), func(tx Transaction) error {
        // These all happen in the same transaction
        
        if err := tx.CreateOrder(order); err != nil {
            return err
        }
        
        for _, item := range order.Items {
            if err := tx.DecreaseStock(item.ProductID, item.Quantity); err != nil {
                return err
            }
        }
        
        if err := tx.UpdateUserBalance(order.UserID, -order.Total); err != nil {
            return err
        }
        
        return nil  // Transaction committed
    })
}
```

---

## Pagination Support

Add cursor-based pagination for large datasets:

```go
type UserPagination struct {
    Cursor string  // Last ID from previous page
    Limit  int     // Page size (default: 20)
    Order  string  // "asc" or "desc" by created_at
}

func (r *userPostgresRepository) ListPaginated(ctx context.Context, p UserPagination) ([]*model.User, string, error) {
    limit := p.Limit
    if limit <= 0 {
        limit = 20
    }
    if limit > 100 {
        limit = 100
    }

    order := "DESC"
    if p.Order == "asc" {
        order = "ASC"
    }

    query := `
        SELECT id, name, email, status, created_at, updated_at
        FROM users
        WHERE id > $1
        ORDER BY created_at ` + order + `
        LIMIT $2
    `

    rows, err := r.db.QueryContext(ctx, query, p.Cursor, limit+1)
    if err != nil {
        return nil, "", err
    }
    defer rows.Close()

    var users []*model.User
    for rows.Next() {
        var user model.User
        rows.Scan(&user.ID, &user.Name, &user.Email, &user.Status, &user.CreatedAt, &user.UpdatedAt)
        users = append(users, &user)
    }

    // Has more?
    nextCursor := ""
    if len(users) > limit {
        nextCursor = users[limit-1].ID
        users = users[:limit]
    }

    return users, nextCursor, nil
}
```

---

## Quick Reference

| Concept | Purpose |
|---------|---------|
| Interface | Defines the contract |
| Implementation | Actual DB logic |
| Sentinel errors | Error constants across implementations |
| In-memory repo | Fast tests, no DB dependency |
| PostgreSQL repo | Production data access |
| Transactions | Multi-table atomic operations |
| Pagination | Large dataset handling |

---

## Benefits Summary

| Benefit | Explanation |
|---------|-------------|
| **Testable** | Mock the interface for fast unit tests |
| **Swappable** | Switch from Postgres to MongoDB without touching service code |
| **Single responsibility** | All data access in one place |
| **Consistent errors** | All implementations return the same errors |
| **Composeable** | Combine repositories for transactions |

---

## Common Pitfalls

1. **Leaking DB types** — Return domain models, not `sql.Rows`
2. **Too many methods** — Only expose what the service needs
3. **No error constants** — Use sentinel errors for common cases
4. **Not handling transactions** — Add transaction support early
5. **Over-abstracting** — Don't create interfaces for everything; only when you need abstraction

---

## Next Steps

- [Service Layer](03-service-layer.md) — Business logic orchestration
- [Dependency Injection](04-dependency-injection.md) — Wiring it all together
- [Milestone Project](../projects/20-layered-http-service.md) — Build it end-to-end