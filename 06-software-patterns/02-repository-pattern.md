# Repository Pattern

> Abstract data access. Swap databases without changing business logic.

---

## The Problem

```
┌──────────────────────────────────────────────────────────┐
│  Service Layer                                           │
│                                                          │
│  func (s *UserService) CreateUser(u *User) error {      │
│      // Direct database code here?                       │
│      // Hard to test                                     │
│      // Hard to swap database                            │
│  }                                                       │
└──────────────────────────────────────────────────────────┘
```

If you put database code directly in services:
- **Testing** is hard - need real database
- **Swapping** databases is painful
- **Duplication** across services
- **Violates** single responsibility

---

## The Solution: Repository Interface

```
┌────────────────────────────────────────────────────────────┐
│                     Service Layer                         │
│                                                             │
│  func (s *UserService) CreateUser(u *User) error {        │
│      return s.repo.Create(u)  // Calls interface          │
│  }                                                         │
└────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────────┐
│                   Repository Interface                    │
│                                                             │
│  type UserRepository interface {                          │
│      Create(u *User) error                                │
│      GetByID(id string) (*User, error)                    │
│      Update(u *User) error                                │
│      Delete(id string) error                              │
│      List() ([]*User, error)                               │
│  }                                                         │
└────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────────┐
│                  Implementation                            │
│                                                             │
│  PostgreSQL repo  │  In-memory repo  │  Mock repo (tests) │
└────────────────────────────────────────────────────────────┘
```

---

## Interface Definition

```go
// internal/repository/user.go
package repository

import "learning-go/internal/model"

// UserRepository defines the contract for user data access
type UserRepository interface {
    Create(user *model.User) error
    GetByID(id string) (*model.User, error)
    Update(user *model.User) error
    Delete(id string) error
    List() ([]*model.User, error)
}
```

---

## Implementation: In-Memory

```go
// internal/repository/memory.go
package repository

import (
    "errors"
    "sync"

    "learning-go/internal/model"
)

var (
    ErrNotFound = errors.New("user not found")
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
        return nil, ErrNotFound
    }

    return user, nil
}

func (r *userRepository) Update(user *model.User) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.data[user.ID]; !exists {
        return ErrNotFound
    }

    r.data[user.ID] = user
    return nil
}

func (r *userRepository) Delete(id string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.data[id]; !exists {
        return ErrNotFound
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

## Implementation: PostgreSQL (Concept)

```go
// internal/repository/postgres.go
package repository

type postgresUserRepository struct {
    db *sql.DB
}

func NewPostgres(conn string) (UserRepository, error) {
    db, err := sql.Open("postgres", conn)
    if err != nil {
        return nil, err
    }

    return &postgresUserRepository{db: db}, nil
}

func (r *postgresUserRepository) Create(user *model.User) error {
    query := `
        INSERT INTO users (id, name, email, created_at)
        VALUES ($1, $2, $3, $4)
    `

    _, err := r.db.Exec(query, user.ID, user.Name, user.Email, user.CreatedAt)
    return err
}

func (r *postgresUserRepository) GetByID(id string) (*model.User, error) {
    query := `SELECT id, name, email, created_at FROM users WHERE id = $1`

    var user model.User
    err := r.db.QueryRow(query, id).Scan(
        &user.ID, &user.Name, &user.Email, &user.CreatedAt,
    )

    if errors.Is(err, sql.ErrNoRows) {
        return nil, ErrNotFound
    }

    return &user, err
}

// ... other methods
```

---

## Usage in Service

```go
// internal/service/user.go
package service

type UserService struct {
    repo repository.UserRepository  // Depends on interface, not implementation
}

func New(repo repository.UserRepository) *UserService {
    return &UserService{repo: repo}
}

func (s *UserService) CreateUser(name, email string) (*model.User, error) {
    // Validation (business logic)
    if name == "" {
        return nil, errors.New("name is required")
    }

    if email == "" {
        return nil, errors.New("email is required")
    }

    // Create domain model
    user := &model.User{
        ID:        uuid.New().String(),
        Name:      name,
        Email:     email,
        CreatedAt: time.Now(),
    }

    // Persist via repository
    if err := s.repo.Create(user); err != nil {
        return nil, err
    }

    return user, nil
}

func (s *UserService) GetUser(id string) (*model.User, error) {
    return s.repo.GetByID(id)
}
```

---

## Testing with Mocks

```go
// internal/service/user_test.go
package service

import (
    "errors"
    "testing"

    "learning-go/internal/model"
    "learning-go/internal/repository"
)

// Mock repository for testing
type mockUserRepository struct {
    users map[string]*model.User
}

func newMockUserRepository() *mockUserRepository {
    return &mockUserRepository{
        users: make(map[string]*model.User),
    }
}

func (m *mockUserRepository) Create(user *model.User) error {
    if _, exists := m.users[user.ID]; exists {
        return errors.New("user already exists")
    }
    m.users[user.ID] = user
    return nil
}

func (m *mockUserRepository) GetByID(id string) (*model.User, error) {
    user, ok := m.users[id]
    if !ok {
        return nil, repository.ErrNotFound
    }
    return user, nil
}

func (m *mockUserRepository) Update(user *model.User) error {
    if _, ok := m.users[user.ID]; !ok {
        return repository.ErrNotFound
    }
    m.users[user.ID] = user
    return nil
}

func (m *mockUserRepository) Delete(id string) error {
    if _, ok := m.users[id]; !ok {
        return repository.ErrNotFound
    }
    delete(m.users, id)
    return nil
}

func (m *mockUserRepository) List() []*model.User {
    users := make([]*model.User, 0, len(m.users))
    for _, u := range m.users {
        users = append(users, u)
    }
    return users
}

func TestUserService_CreateUser(t *testing.T) {
    mock := newMockUserRepository()
    svc := New(mock)

    user, err := svc.CreateUser("Alice", "alice@example.com")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if user.Name != "Alice" {
        t.Errorf("expected name Alice, got %s", user.Name)
    }
}
```

---

## Benefits Summary

```
┌──────────────────────────────────────────────────────────┐
│                    Benefits                              │
├──────────────────────────────────────────────────────────┤
│  ✓ Testable - Mock repository for unit tests             │
│  ✓ Swappable - Switch DB without changing service       │
│  ✓ Single responsibility - Data access in one place     │
│  ✓ Composability - Easy to add new implementations      │
└──────────────────────────────────────────────────────────┘
```

---

## Quick Reference

| Concept | Description |
|---------|-------------|
| Repository interface | Defines data operations |
| Concrete implementation | Actual DB logic (Postgres, Redis, etc.) |
| Mock for testing | In-memory fake, no DB needed |
| ErrNotFound | Sentinel error for missing records |

---

## Common Pitfalls

1. **Too many methods** - Only expose what's needed
2. **Leaking DB types** - Use domain models, not `sql.Rows`
3. **Transaction management** - Consider in the interface design
4. **Over-abstraction** - Don't create interfaces for everything

---

## Next Steps

- [Service Layer](08-service-layer.md) - Business logic orchestration
- [Dependency Injection](09-dependency-injection.md) - Wiring it all together
- [Milestone Project](20-layered-http-service.md) - Build it end-to-end