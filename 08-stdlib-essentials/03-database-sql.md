# 3. Database SQL — Working with Databases

> **Goal:** Learn to connect to databases, execute queries, and handle results using Go's `database/sql` package.

---

## Table of Contents

1. [Setup & Connection](#1-setup--connection-core)
2. [Basic Queries](#2-basic-queries-core)
3. [CRUD Operations](#3-crud-operations-core)
4. [Prepared Statements](#4-prepared-statements-core)
5. [Transactions](#5-transactions-core)
6. [Error Handling](#6-error-handling-core)
7. [Common Pitfalls](#7-common-pitfalls-core)

---

## 1. Setup & Connection [CORE]

### Install Driver

```bash
# PostgreSQL
go get github.com/lib/pq

# MySQL
go get github.com/go-sql-driver/mysql

# SQLite
go get github.com/mattn/go-sqlite3
```

### Open Connection

```go
import (
    "database/sql"
    "log"
    _ "github.com/lib/pq"  // PostgreSQL driver
)

func main() {
    // Connection string
    connStr := "host=localhost port=5432 user=myuser password=mypass dbname=mydb sslmode=disable"

    db, err := sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Test connection
    if err := db.Ping(); err != nil {
        log.Fatal("Cannot connect:", err)
    }

    fmt.Println("Connected!")
}
```

### Connection Pool Settings

```go
db.SetMaxOpenConns(25)                 // Max open connections
db.SetMaxIdleConns(5)                  // Max idle connections
db.SetConnMaxLifetime(5 * time.Minute) // Max connection lifetime
```

### Environment Variables

```go
func connectDB() (*sql.DB, error) {
    host := os.Getenv("DB_HOST")
    port := os.Getenv("DB_PORT")
    user := os.Getenv("DB_USER")
    pass := os.Getenv("DB_PASS")
    name := os.Getenv("DB_NAME")

    connStr := fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        host, port, user, pass, name,
    )

    return sql.Open("postgres", connStr)
}
```

---

## 2. Basic Queries [CORE]

### Query Single Row

```go
type User struct {
    ID    int
    Name  string
    Email string
}

func getUser(db *sql.DB, id int) (*User, error) {
    var user User

    err := db.QueryRow(
        "SELECT id, name, email FROM users WHERE id = $1", id,
    ).Scan(&user.ID, &user.Name, &user.Email)

    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user %d not found", id)
    }
    if err != nil {
        return nil, err
    }

    return &user, nil
}
```

### Query Multiple Rows

```go
func listUsers(db *sql.DB) ([]User, error) {
    rows, err := db.Query("SELECT id, name, email FROM users ORDER BY name")
    if err != nil {
        return nil, err
    }
    defer rows.Close()  // ALWAYS close rows

    var users []User
    for rows.Next() {
        var u User
        if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
            return nil, err
        }
        users = append(users, u)
    }

    // Check for errors from iterating
    if err := rows.Err(); err != nil {
        return nil, err
    }

    return users, nil
}
```

### Query with Parameters

```go
func searchUsers(db *sql.DB, name string) ([]User, error) {
    // Use $1, $2, etc. for PostgreSQL
    // Use ?, ?, etc. for MySQL/SQLite
    rows, err := db.Query(
        "SELECT id, name, email FROM users WHERE name ILIKE $1",
        "%"+name+"%",
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // ... scan rows ...
}
```

---

## 3. CRUD Operations [CORE]

### Create (Insert)

```go
func createUser(db *sql.DB, name, email string) (*User, error) {
    var user User

    err := db.QueryRow(
        `INSERT INTO users (name, email) 
         VALUES ($1, $2) 
         RETURNING id, name, email`,
        name, email,
    ).Scan(&user.ID, &user.Name, &user.Email)

    if err != nil {
        return nil, fmt.Errorf("insert user: %w", err)
    }

    return &user, nil
}

// Alternative: Exec (no RETURNING)
func createUserExec(db *sql.DB, name, email string) (int64, error) {
    result, err := db.Exec(
        "INSERT INTO users (name, email) VALUES ($1, $2)",
        name, email,
    )
    if err != nil {
        return 0, err
    }

    return result.LastInsertId()  // MySQL; PostgreSQL use RETURNING
}
```

### Read (Select)

```go
// Single user
user, err := getUser(db, 1)

// All users
users, err := listUsers(db)

// With filter
users, err := searchUsers(db, "alice")
```

### Update

```go
func updateUser(db *sql.DB, id int, name, email string) error {
    result, err := db.Exec(
        "UPDATE users SET name = $1, email = $2 WHERE id = $3",
        name, email, id,
    )
    if err != nil {
        return err
    }

    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if rowsAffected == 0 {
        return fmt.Errorf("user %d not found", id)
    }

    return nil
}
```

### Delete

```go
func deleteUser(db *sql.DB, id int) error {
    result, err := db.Exec("DELETE FROM users WHERE id = $1", id)
    if err != nil {
        return err
    }

    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if rowsAffected == 0 {
        return fmt.Errorf("user %d not found", id)
    }

    return nil
}
```

---

## 4. Prepared Statements [CORE]

Prepared statements are faster for repeated queries and prevent SQL injection.

### Basic Usage

```go
func createUserPrepared(db *sql.DB, users []User) error {
    stmt, err := db.Prepare("INSERT INTO users (name, email) VALUES ($1, $2)")
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, u := range users {
        _, err := stmt.Exec(u.Name, u.Email)
        if err != nil {
            return err
        }
    }

    return nil
}
```

### Prepared Statement in Struct

```go
type UserRepo struct {
    db        *sql.DB
    insertStmt *sql.sql.Stmt
    getByIDStmt *sql.Stmt
}

func NewUserRepo(db *sql.DB) (*UserRepo, error) {
    insertStmt, err := db.Prepare("INSERT INTO users (name, email) VALUES ($1, $2)")
    if err != nil {
        return nil, err
    }

    getByIDStmt, err := db.Prepare("SELECT id, name, email FROM users WHERE id = $1")
    if err != nil {
        insertStmt.Close()
        return nil, err
    }

    return &UserRepo{
        db:          db,
        insertStmt:  insertStmt,
        getByIDStmt: getByIDStmt,
    }, nil
}

func (r *UserRepo) Close() {
    r.insertStmt.Close()
    r.getByIDStmt.Close()
}
```

---

## 5. Transactions [CORE]

### Basic Transaction

```go
func transferMoney(db *sql.DB, fromID, toID int, amount float64) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()  // Rollback if not committed

    // Debit from account
    _, err = tx.Exec(
        "UPDATE accounts SET balance = balance - $1 WHERE id = $2",
        amount, fromID,
    )
    if err != nil {
        return err
    }

    // Credit to account
    _, err = tx.Exec(
        "UPDATE accounts SET balance = balance + $1 WHERE id = $2",
        amount, toID,
    )
    if err != nil {
        return err
    }

    return tx.Commit()  // Commit if all succeeded
}
```

### Transaction with Query

```go
func getOrCreateUser(tx *sql.Tx, email string) (*User, error) {
    var user User

    // Try to find existing
    err := tx.QueryRow(
        "SELECT id, name, email FROM users WHERE email = $1", email,
    ).Scan(&user.ID, &user.Name, &user.Email)

    if err == sql.ErrNoRows {
        // Create new user
        err = tx.QueryRow(
            "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email",
            email, email,
        ).Scan(&user.ID, &user.Name, &user.Email)
    }

    if err != nil {
        return nil, err
    }

    return &user, nil
}
```

---

## 6. Error Handling [CORE]

### Check for No Rows

```go
user, err := getUser(db, 999)
if err == sql.ErrNoRows {
    fmt.Println("User not found")
    return
}
if err != nil {
    log.Fatal(err)
}
```

### Check RowsAffected

```go
result, err := db.Exec("DELETE FROM users WHERE id = $1", id)
if err != nil {
    return err
}

count, err := result.RowsAffected()
if err != nil {
    return err
}

if count == 0 {
    return fmt.Errorf("no user with id %d", id)
}
```

### Custom Error Types

```go
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
)

func getUser(db *sql.DB, id int) (*User, error) {
    var user User
    err := db.QueryRow("SELECT id, name FROM users WHERE id = $1", id).
        Scan(&user.ID, &user.Name)

    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("query user: %w", err)
    }
    return &user, nil
}

// Usage
user, err := getUser(db, 1)
if errors.Is(err, ErrNotFound) {
    // Handle not found
}
```

---

## 7. Common Pitfalls [CORE]

### 1. Forgetting to Close Rows

```go
// WRONG — leaks resources
rows, _ := db.Query("SELECT id FROM users")
for rows.Next() { ... }

// RIGHT
rows, err := db.Query("SELECT id FROM users")
if err != nil {
    return err
}
defer rows.Close()
for rows.Next() { ... }
```

### 2. Not Checking rows.Err()

```go
// WRONG — might miss errors
for rows.Next() {
    // scan
}
return nil  // might be hiding an error

// RIGHT
for rows.Next() {
    // scan
}
return rows.Err()  // check for iteration errors
```

### 3. SQL Injection (String Concatenation)

```go
// WRONG — SQL INJECTION VULNERABILITY!
query := fmt.Sprintf("SELECT * FROM users WHERE name = '%s'", name)
rows, _ := db.Query(query)

// RIGHT — use parameterized queries
rows, _ := db.Query("SELECT * FROM users WHERE name = $1", name)
```

### 4. Forgetting defer rows.Close()

```go
// WRONG — rows never closed if function returns early
rows, err := db.Query("SELECT ...")
if err != nil {
    return err
}
// ... process rows ...
rows.Close()  // might not reach here

// RIGHT — defer ensures cleanup
rows, err := db.Query("SELECT ...")
if err != nil {
    return err
}
defer rows.Close()
```

### 5. Using Query for INSERT/UPDATE/DELETE

```go
// WRONG — Query expects rows back
db.Query("INSERT INTO users (name) VALUES ($1)", name)

// RIGHT — use Exec for non-SELECT
db.Exec("INSERT INTO users (name) VALUES ($1)", name)
```

---

## Quick Reference

```go
// Connection
db, err := sql.Open("postgres", connStr)
err = db.Ping()
defer db.Close()

// Single row
err := db.QueryRow("SELECT ...", args...).Scan(&dest...)

// Multiple rows
rows, err := db.Query("SELECT ...", args...)
defer rows.Close()
for rows.Next() { rows.Scan(&dest...) }
rows.Err()

// Insert/Update/Delete
result, err := db.Exec("INSERT ...", args...)
id, _ := result.LastInsertId()
count, _ := result.RowsAffected()

// Transaction
tx, err := db.Begin()
defer tx.Rollback()
tx.Exec(...)
tx.Query(...)
tx.Commit()

// Prepared Statement
stmt, err := db.Prepare("SELECT ...")
defer stmt.Close()
stmt.Query(args...)
```

---

## Exercises

### Exercise 1: Create and Query Users ⭐
**Difficulty:** Beginner | **Time:** ~15 min

Create a SQLite database with a `users` table. Insert 3 users, then query and print all of them.

<details>
<summary>Solution</summary>

```go
package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID    int
	Name  string
	Email string
}

func main() {
	db, err := sql.Open("sqlite3", "./test.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Insert users
	users := []struct{ name, email string }{
		{"Alice", "alice@example.com"},
		{"Bob", "bob@example.com"},
		{"Charlie", "charlie@example.com"},
	}

	stmt, err := db.Prepare("INSERT INTO users (name, email) VALUES (?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	for _, u := range users {
		_, err := stmt.Exec(u.name, u.email)
		if err != nil {
			log.Println("Insert error:", err)
		}
	}

	// Query all
	rows, err := db.Query("SELECT id, name, email FROM users ORDER BY name")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Users:")
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  %d: %s (%s)\n", u.ID, u.Name, u.Email)
	}
}
```

</details>

### Exercise 2: Transaction Example ⭐⭐
**Difficulty:** Beginner | **Time:** ~15 min

Create a function that inserts multiple users in a transaction. If any insert fails, roll back all changes.

<details>
<summary>Solution</summary>

```go
package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func insertUsers(db *sql.DB, users []struct{ name, email string }) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO users (name, email) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, u := range users {
		if _, err := stmt.Exec(u.name, u.email); err != nil {
			return fmt.Errorf("insert %s: %w", u.name, err)
		}
	}

	return tx.Commit()
}

func main() {
	db, err := sql.Open("sqlite3", "./test.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT, email TEXT UNIQUE)")

	users := []struct{ name, email string }{
		{"Alice", "alice@example.com"},
		{"Bob", "bob@example.com"},
	}

	if err := insertUsers(db, users); err != nil {
		log.Println("Transaction failed:", err)
	} else {
		fmt.Println("Users inserted successfully")
	}
}
```

</details>

---

## Next: [Logging →](./04-logging.md)
