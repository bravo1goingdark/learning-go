# 2. HTTP Basics — Building Web Services

> **Goal:** Learn to build HTTP servers and make HTTP requests using Go's standard library.

---

## Table of Contents

1. [HTTP Client (Making Requests)](#1-http-client-making-requests) `[CORE]`
2. [HTTP Server (Handling Requests)](#2-http-server-handling-requests) `[CORE]`
3. [Routing](#3-routing) `[CORE]`
4. [Request Handling](#4-request-handling) `[CORE]`
5. [Middleware](#5-middleware) `[PRODUCTION]`
6. [JSON APIs](#6-json-apis) `[CORE]`
7. [Common Pitfalls](#7-common-pitfalls) `[CORE]`

---

## 1. HTTP Client (Making Requests) [CORE]

### GET Request

```go
package main

import (
    "fmt"
    "io"
    "net/http"
)

func main() {
    resp, err := http.Get("https://api.github.com/users/golang")
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    defer resp.Body.Close()  // ALWAYS close the body

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        fmt.Println("Error:", err)
        return
    }

    fmt.Println("Status:", resp.StatusCode)
    fmt.Println("Body:", string(body))
}
```

### POST Request with JSON Body

```go
import (
    "bytes"
    "encoding/json"
    "net/http"
)

type User struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func createUser(user User) error {
    data, err := json.Marshal(user)
    if err != nil {
        return err
    }

    resp, err := http.Post(
        "https://api.example.com/users",
        "application/json",
        bytes.NewReader(data),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("API error: %s - %s", resp.Status, body)
    }

    return nil
}
```

### Custom Client with Timeout

```go
client := &http.Client{
    Timeout: 10 * time.Second,
}

req, err := http.NewRequest("GET", "https://api.example.com/data", nil)
if err != nil {
    log.Fatal(err)
}

// Add headers
req.Header.Set("Authorization", "Bearer token123")
req.Header.Set("Accept", "application/json")

resp, err := client.Do(req)
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
```

### Query Parameters

```go
params := url.Values{}
params.Add("page", "1")
params.Add("limit", "20")
params.Add("search", "alice")

req, _ := http.NewRequest("GET", "https://api.example.com/users", nil)
req.URL.RawQuery = params.Encode()

// Final URL: https://api.example.com/users?limit=20&page=1&search=alice
```

---

## 2. HTTP Server (Handling Requests) [CORE]

### Simple Server

```go
package main

import (
    "fmt"
    "log"
    "net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, World!")
}

func main() {
    http.HandleFunc("/", helloHandler)

    log.Println("Server starting on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatal(err)
    }
}
```

### Multiple Routes

```go
func main() {
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/api/users", usersHandler)
    http.HandleFunc("/api/health", healthHandler)

    log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Welcome to the API")
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Users endpoint")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "OK")
}
```

### Read Request Body

```go
func createUserHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Error reading body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    var user User
    if err := json.Unmarshal(body, &user); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    fmt.Fprintf(w, "Created user: %s", user.Name)
}
```

### Read URL Parameters

```go
// Route: /users/{id} (Go 1.22+)
func getUserHandler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")  // Go 1.22+
    fmt.Fprintf(w, "User ID: %s", id)
}
```

### Read Query Parameters

```go
func searchHandler(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")       // Single value
    page := r.URL.Query().Get("page")
    tags := r.URL.Query()["tag"]           // Multiple values

    fmt.Fprintf(w, "Search: %s, Page: %s, Tags: %v", query, page, tags)
}

// URL: /search?q=alice&page=1&tag=go&tag=web
```

### Read Headers

```go
func authHandler(w http.ResponseWriter, r *http.Request) {
    token := r.Header.Get("Authorization")
    if token == "" {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    fmt.Fprintf(w, "Token: %s", token)
}
```

---

## 3. Routing [CORE]

### Method-Based Routing (Go 1.22+)

```go
func main() {
    mux := http.NewServeMux()

    mux.HandleFunc("GET /api/users", listUsers)
    mux.HandleFunc("POST /api/users", createUser)
    mux.HandleFunc("GET /api/users/{id}", getUser)
    mux.HandleFunc("PUT /api/users/{id}", updateUser)
    mux.HandleFunc("DELETE /api/users/{id}", deleteUser)

    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

### Path Parameters (Go 1.22+)

```go
func getUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")  // Extract {id} from path
    fmt.Fprintf(w, "Getting user %s", id)
}
```

### Wildcard Routes

```go
// Matches /files/anything/here
mux.HandleFunc("/files/{path...}", func(w http.ResponseWriter, r *http.Request) {
    path := r.PathValue("path")  // "documents/report.pdf"
    fmt.Fprintf(w, "File: %s", path)
})
```

### Custom Router (Older Go Versions)

For Go < 1.22, use a third-party router or manual parsing:

```go
func getUserHandler(w http.ResponseWriter, r *http.Request) {
    // Parse /users/{id} manually
    path := strings.TrimPrefix(r.URL.Path, "/users/")
    if path == "" {
        http.Error(w, "Missing ID", http.StatusBadRequest)
        return
    }
    fmt.Fprintf(w, "User ID: %s", path)
}
```

---

## 4. Request Handling [CORE]

### Complete CRUD Handler

```go
type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

var users = map[string]User{}  // In-memory store

func usersHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        listUsers(w, r)
    case http.MethodPost:
        createUser(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func listUsers(w http.ResponseWriter, r *http.Request) {
    result := make([]User, 0, len(users))
    for _, u := range users {
        result = append(result, u)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}

func createUser(w http.ResponseWriter, r *http.Request) {
    var user User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    user.ID = fmt.Sprintf("user-%d", len(users)+1)
    users[user.ID] = user

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}
```

### Write JSON Response Helper

```go
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        log.Printf("writeJSON error: %v", err)
    }
}

func writeError(w http.ResponseWriter, status int, message string) {
    writeJSON(w, status, map[string]string{"error": message})
}

// Usage
func getUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    user, ok := users[id]
    if !ok {
        writeError(w, http.StatusNotFound, "user not found")
        return
    }
    writeJSON(w, http.StatusOK, user)
}
```

---

## 5. Middleware [PRODUCTION]

> ⏭️ **First pass? Skip this section.** Come back after completing projects.

### What is Middleware?

A function that wraps another handler to add behavior (logging, auth, CORS).

### Logging Middleware

```go
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Wrap response writer to capture status code
        wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

        next.ServeHTTP(wrapped, r)

        log.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
    })
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}
```

### CORS Middleware

```go
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### Auth Middleware

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" || !isValidToken(token) {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Chain Middleware

```go
func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", usersHandler)

    // Apply middleware in order: auth → cors → logging → handler
    handler := authMiddleware(corsMiddleware(loggingMiddleware(mux)))

    log.Fatal(http.ListenAndServe(":8080", handler))
}
```

---

## 6. JSON APIs [CORE]

### Complete JSON API Example

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "sync"
)

type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type UserStore struct {
    mu    sync.RWMutex
    users map[string]User
}

func NewUserStore() *UserStore {
    return &UserStore{
        users: make(map[string]User),
    }
}

func (s *UserStore) List() []User {
    s.mu.RLock()
    defer s.mu.RUnlock()

    result := make([]User, 0, len(s.users))
    for _, u := range s.users {
        result = append(result, u)
    }
    return result
}

func (s *UserStore) Get(id string) (User, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    u, ok := s.users[id]
    return u, ok
}

func (s *UserStore) Create(u User) {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.users[u.ID] = u
}

func main() {
    store := NewUserStore()

    http.HandleFunc("GET /api/users", func(w http.ResponseWriter, r *http.Request) {
        users := store.List()
        writeJSON(w, http.StatusOK, users)
    })

    http.HandleFunc("GET /api/users/{id}", func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        user, ok := store.Get(id)
        if !ok {
            writeError(w, http.StatusNotFound, "user not found")
            return
        }
        writeJSON(w, http.StatusOK, user)
    })

    http.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
        var user User
        if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
            writeError(w, http.StatusBadRequest, "invalid JSON")
            return
        }
        if user.ID == "" {
            writeError(w, http.StatusBadRequest, "ID required")
            return
        }

        store.Create(user)
        writeJSON(w, http.StatusCreated, user)
    })

    log.Println("Server on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, map[string]string{"error": msg})
}
```

---

## 7. Common Pitfalls [CORE]

### 1. Not Closing Response Body

```go
// WRONG — leaks resources
resp, _ := http.Get("https://api.example.com")
body, _ := io.ReadAll(resp.Body)
// resp.Body never closed!

// RIGHT
resp, err := http.Get("https://api.example.com")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
body, _ := io.ReadAll(resp.Body)
```

### 2. Ignoring Errors

```go
// WRONG
http.Get("https://api.example.com")

// RIGHT
resp, err := http.Get("https://api.example.com")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
```

### 3. Not Setting Content-Type

```go
// WRONG — client doesn't know it's JSON
json.NewEncoder(w).Encode(data)

// RIGHT
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(data)
```

### 4. Using Default Client (No Timeout)

```go
// WRONG — can hang forever
resp, err := http.Get("https://slow-api.example.com")

// RIGHT — set a timeout
client := &http.Client{Timeout: 10 * time.Second}
resp, err := client.Get("https://slow-api.example.com")
```

### 5. Reading Body Multiple Times

```go
// WRONG — body is a stream, can only read once
body1, _ := io.ReadAll(r.Body)
body2, _ := io.ReadAll(r.Body)  // body2 is empty!

// RIGHT — read once, use the bytes
body, _ := io.ReadAll(r.Body)
// Parse body as needed
```

---

## Quick Reference

```go
// Client
resp, err := http.Get(url)
resp, err := http.Post(url, contentType, body)
client := &http.Client{Timeout: 10 * time.Second}
req, _ := http.NewRequest(method, url, body)
req.Header.Set("Authorization", "Bearer token")
resp, err := client.Do(req)

// Server
http.HandleFunc("/path", handler)
http.ListenAndServe(":8080", mux)
http.ServeFile(w, r, "file.html")
http.Redirect(w, r, "/new-url", http.StatusFound)

// Handler
func handler(w http.ResponseWriter, r *http.Request) {
    r.Method                                    // GET, POST, etc.
    r.URL.Path                                  // /api/users
    r.URL.Query().Get("key")                    // ?key=value
    r.PathValue("id")                           // /users/{id} (Go 1.22+)
    r.Header.Get("Authorization")               // Request header
    json.NewDecoder(r.Body).Decode(&value)      // Parse JSON body
    w.Header().Set("Content-Type", "text/html") // Response header
    w.WriteHeader(http.StatusCreated)           // Set status code
    fmt.Fprintf(w, "Hello")                     // Write response
}
```

---

## Exercises

### Exercise 1: Simple HTTP Server ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a server that responds to:
- `GET /` → "Welcome!"
- `GET /hello` → "Hello, World!"

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome!")
	})

	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})

	log.Println("Server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

</details>

### Exercise 2: JSON API ⭐⭐
**Difficulty:** Beginner | **Time:** ~15 min

Create an API with:
- `POST /users` → accepts JSON, stores user, returns created user
- `GET /users` → returns all users as JSON

<details>
<summary>Solution</summary>

```go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var (
	mu    sync.Mutex
	users []User
)

func main() {
	http.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var user User
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		mu.Lock()
		users = append(users, user)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)
	})

	http.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		result := make([]User, len(users))
		copy(result, users)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	log.Println("Server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

</details>

### Exercise 3: HTTP Client with Error Handling ⭐⭐
**Difficulty:** Beginner | **Time:** ~15 min

Write a function that fetches JSON from a URL, decodes it, and handles errors gracefully.

<details>
<summary>Solution</summary>

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"completed"`
}

func fetchTodo(id int) (*Todo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	url := fmt.Sprintf("https://jsonplaceholder.typicode.com/todos/%d", id)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	var todo Todo
	if err := json.Unmarshal(body, &todo); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return &todo, nil
}

func main() {
	todo, err := fetchTodo(1)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Todo: %s (done: %v)\n", todo.Title, todo.Done)
}
```

</details>

---

## Next: [Database SQL →](./03-database-sql.md)
