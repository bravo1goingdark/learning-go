# Progressive Exercises — Scaffolded Go Practice

> **Goal:** Practice Go with guided exercises that have `// TODO:` markers. Each exercise builds on previous concepts.

---

## Table of Contents

1. [How to Use These Exercises](#how-to-use-these-exercises) `[CORE]`
2. [Exercise Set 1: Basics (After Phase 1)](#exercise-set-1-basics-after-phase-1) `[CORE]`
3. [Exercise Set 2: Structs & Methods (After Phase 2)](#exercise-set-2-structs--methods-after-phase-2) `[CORE]`
4. [Exercise Set 3: Error Handling (After Phase 3)](#exercise-set-3-error-handling-after-phase-3) `[CORE]`
5. [Exercise Set 4: Concurrency (After Phase 5)](#exercise-set-4-concurrency-after-phase-5) `[CORE]`
6. [Exercise Set 5: JSON & HTTP (After stdlib-essentials)](#exercise-set-5-json--http-after-stdlib-essentials) `[CORE]`
7. [Progress Checklist](#progress-checklist) `[CORE]`
8. [Tips](#tips) `[CORE]`

---

## How to Use These Exercises

1. Copy the code into a `.go` file
2. Look for `// TODO:` comments
3. Fill in the code where indicated
4. Run `go run <filename>` to test
5. Check the solution in the expandable section if stuck

---

## Exercise Set 1: Basics (After Phase 1)

### Exercise 1.1: Greeting Function ⭐

```go
package main

import "fmt"

// TODO: Create a function called greet that:
//   - Takes a name (string) as parameter
//   - Returns a string: "Hello, <name>!"
//   - Use fmt.Sprintf to build the string

// func greet(...) ... {
//     
// }

func main() {
    // This should print: Hello, Alice!
    fmt.Println(greet("Alice"))

    // This should print: Hello, Bob!
    fmt.Println(greet("Bob"))
}
```

<details>
<summary>Solution</summary>

```go
func greet(name string) string {
    return fmt.Sprintf("Hello, %s!", name)
}
```

</details>

---

### Exercise 1.2: Sum of Slice ⭐

```go
package main

import "fmt"

// TODO: Create a function called sum that:
//   - Takes a slice of integers
//   - Returns the sum of all numbers
//   - Use a for loop to iterate

// func sum(...) ... {
//     
// }

func main() {
    nums := []int{1, 2, 3, 4, 5}
    fmt.Println("Sum:", sum(nums))  // Should print: Sum: 15

    empty := []int{}
    fmt.Println("Empty:", sum(empty))  // Should print: Empty: 0
}
```

<details>
<summary>Solution</summary>

```go
func sum(numbers []int) int {
    total := 0
    for _, n := range numbers {
        total += n
    }
    return total
}
```

</details>

---

### Exercise 1.3: Word Counter ⭐⭐

```go
package main

import (
    "fmt"
    "strings"
)

// TODO: Create a function called wordCount that:
//   - Takes a string as parameter
//   - Returns the number of words
//   - Hint: use strings.Fields() to split by whitespace
//   - strings.Fields returns []string

// func wordCount(...) ... {
//     
// }

func main() {
    fmt.Println(wordCount("Hello World"))          // 2
    fmt.Println(wordCount("Go is awesome"))        // 3
    fmt.Println(wordCount("  spaces   everywhere"))// 2
    fmt.Println(wordCount(""))                      // 0
}
```

<details>
<summary>Solution</summary>

```go
func wordCount(s string) int {
    if s == "" {
        return 0
    }
    return len(strings.Fields(s))
}
```

</details>

---

## Exercise Set 2: Structs & Methods (After Phase 2)

### Exercise 2.1: Bank Account ⭐⭐

```go
package main

import "fmt"

// TODO: Create a BankAccount struct with:
//   - Owner (string)
//   - Balance (float64)

// type BankAccount struct {
//     
// }

// TODO: Create methods:
//   - Deposit(amount float64) - adds to balance
//   - Withdraw(amount float64) error - subtracts from balance
//     Returns error if insufficient funds
//   - String() string - returns "Owner: $Balance"

// func (b *BankAccount) Deposit(...) {
//     
// }

// func (b *BankAccount) Withdraw(...) error {
//     
// }

// func (b *BankAccount) String() string {
//     
// }

func main() {
    account := BankAccount{Owner: "Alice", Balance: 100.0}

    account.Deposit(50.0)
    fmt.Println(account)  // Alice: $150

    err := account.Withdraw(30.0)
    if err != nil {
        fmt.Println("Error:", err)
    }
    fmt.Println(account)  // Alice: $120

    err = account.Withdraw(200.0)
    if err != nil {
        fmt.Println("Error:", err)  // Error: insufficient funds
    }
}
```

<details>
<summary>Solution</summary>

```go
type BankAccount struct {
    Owner   string
    Balance float64
}

func (b *BankAccount) Deposit(amount float64) {
    b.Balance += amount
}

func (b *BankAccount) Withdraw(amount float64) error {
    if amount > b.Balance {
        return fmt.Errorf("insufficient funds: have %.2f, need %.2f", b.Balance, amount)
    }
    b.Balance -= amount
    return nil
}

func (b *BankAccount) String() string {
    return fmt.Sprintf("%s: $%.2f", b.BOwner, b.Balance)
}
```

</details>

---

### Exercise 2.2: Shape Interface ⭐⭐

```go
package main

import (
    "fmt"
    "math"
)

// TODO: Create an interface called Shape with:
//   - Area() float64
//   - Perimeter() float64
//   - String() string

// type Shape interface {
//     
// }

// TODO: Create a Circle struct with:
//   - Radius (float64)

// type Circle struct {
//     
// }

// TODO: Create a Rectangle struct with:
//   - Width, Height (float64)

// type Rectangle struct {
//     
// }

// TODO: Implement Shape interface for Circle and Rectangle

// func (c Circle) Area() float64 { ... }
// func (c Circle) Perimeter() float64 { ... }
// func (c Circle) String() string { ... }

// func (r Rectangle) Area() float64 { ... }
// func (r Rectangle) Perimeter() float64 { ... }
// func (r Rectangle) String() string { ... }

func printShape(s Shape) {
    fmt.Printf("%s: Area=%.2f, Perimeter=%.2f\n", s, s.Area(), s.Perimeter())
}

func main() {
    c := Circle{Radius: 5}
    r := Rectangle{Width: 10, Height: 5}

    printShape(c)  // Circle(r=5.00): Area=78.54, Perimeter=31.42
    printShape(r)  // Rectangle(10.00x5.00): Area=50.00, Perimeter=30.00
}
```

<details>
<summary>Solution</summary>

```go
type Shape interface {
    Area() float64
    Perimeter() float64
    String() string
}

type Circle struct {
    Radius float64
}

func (c Circle) Area() float64 {
    return math.Pi * c.Radius * c.Radius
}

func (c Circle) Perimeter() float64 {
    return 2 * math.Pi * c.Radius
}

func (c Circle) String() string {
    return fmt.Sprintf("Circle(r=%.2f)", c.Radius)
}

type Rectangle struct {
    Width, Height float64
}

func (r Rectangle) Area() float64 {
    return r.Width * r.Height
}

func (r Rectangle) Perimeter() float64 {
    return 2 * (r.Width + r.Height)
}

func (r Rectangle) String() string {
    return fmt.Sprintf("Rectangle(%.2fx%.2f)", r.Width, r.Height)
}
```

</details>

---

## Exercise Set 3: Error Handling (After Phase 3)

### Exercise 3.1: Safe Divider ⭐⭐

```go
package main

import (
    "errors"
    "fmt"
)

// TODO: Define sentinel error
// var ErrDivisionByZero = ...

// TODO: Create a Divide function that:
//   - Takes two float64 values (a, b)
//   - Returns (float64, error)
//   - Returns ErrDivisionByZero if b is 0
//   - Returns the result of a/b otherwise

// func Divide(a, b float64) (float64, error) {
//     
// }

func main() {
    result, err := Divide(10, 3)
    if err != nil {
        if errors.Is(err, ErrDivisionByZero) {
            fmt.Println("Cannot divide by zero!")
        } else {
            fmt.Println("Error:", err)
        }
    } else {
        fmt.Printf("10 / 3 = %.2f\n", result)
    }

    _, err = Divide(10, 0)
    if errors.Is(err, ErrDivisionByZero) {
        fmt.Println("Caught division by zero!")
    }
}
```

<details>
<summary>Solution</summary>

```go
var ErrDivisionByZero = errors.New("division by zero")

func Divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, ErrDivisionByZero
    }
    return a / b, nil
}
```

</details>

---

### Exercise 3.2: Custom Validation Error ⭐⭐⭐

```go
package main

import (
    "fmt"
    "strings"
)

// TODO: Create a ValidationError struct with:
//   - Field (string)
//   - Message (string)

// type ValidationError struct {
//     
// }

// TODO: Implement the error interface for ValidationError
// func (e *ValidationError) Error() string { ... }

// TODO: Create a ValidateUser function that:
//   - Takes name and email strings
//   - Returns error if name is empty (ValidationError)
//   - Returns error if email doesn't contain "@" (ValidationError)
//   - Returns nil if valid

// func ValidateUser(name, email string) error {
//     
// }

func main() {
    err := ValidateUser("", "alice@example.com")
    if err != nil {
        var ve *ValidationError
        if errors.As(err, &ve) {
            fmt.Printf("Validation failed: field=%s, message=%s\n", ve.Field, ve.Message)
        }
    }

    err = ValidateUser("Alice", "invalid-email")
    if err != nil {
        fmt.Println("Error:", err)
    }

    err = ValidateUser("Alice", "alice@example.com")
    if err == nil {
        fmt.Println("User is valid!")
    }
}
```

<details>
<summary>Solution</summary>

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on %q: %s", e.Field, e.Message)
}

func ValidateUser(name, email string) error {
    if name == "" {
        return &ValidationError{Field: "name", Message: "cannot be empty"}
    }
    if !strings.Contains(email, "@") {
        return &ValidationError{Field: "email", Message: "must contain @ symbol"}
    }
    return nil
}
```

</details>

---

## Exercise Set 4: Concurrency (After Phase 5)

### Exercise 4.1: Parallel Sum ⭐⭐

```go
package main

import (
    "fmt"
    "sync"
)

// TODO: Create a function called parallelSum that:
//   - Takes a slice of integers
//   - Splits it into 4 parts
//   - Calculates each part's sum in a goroutine
//   - Returns the total sum
//   - Use sync.WaitGroup

// func parallelSum(numbers []int) int {
//     
// }

func main() {
    numbers := make([]int, 1000)
    for i := range numbers {
        numbers[i] = i + 1
    }

    result := parallelSum(numbers)
    fmt.Println("Sum:", result)  // Should be 500500

    // Verify
    expected := 0
    for _, n := range numbers {
        expected += n
    }
    fmt.Println("Expected:", expected)
    fmt.Println("Match:", result == expected)
}
```

<details>
<summary>Solution</summary>

```go
func parallelSum(numbers []int) int {
    numWorkers := 4
    chunkSize := len(numbers) / numWorkers

    var wg sync.WaitGroup
    results := make([]int, numWorkers)

    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()

            start := workerID * chunkSize
            end := start + chunkSize
            if workerID == numWorkers-1 {
                end = len(numbers)
            }

            sum := 0
            for _, n := range numbers[start:end] {
                sum += n
            }
            results[workerID] = sum
        }(i)
    }

    wg.Wait()

    total := 0
    for _, r := range results {
        total += r
    }
    return total
}
```

</details>

---

### Exercise 4.2: Worker Pool ⭐⭐⭐

```go
package main

import (
    "fmt"
    "sync"
)

// TODO: Create a worker pool that:
//   - Takes a slice of jobs (integers)
//   - Spins up 3 workers
//   - Each worker processes jobs from a channel
//   - Worker computes job * job (square)
//   - Results are sent back on a results channel
//   - Main collects all results

// func worker(id int, jobs <-chan int, results chan<- int, wg *sync.WaitGroup) {
//     defer wg.Done()
//     for job := range jobs {
//         // TODO: compute square and send to results
//         
//     }
// }

func main() {
    jobs := make(chan int, 10)
    results := make(chan int, 10)

    // TODO: Start 3 workers
    var wg sync.WaitGroup
    // for i := 0; i < 3; i++ {
    //     wg.Add(1)
    //     go worker(i, jobs, results, &wg)
    // }

    // TODO: Send jobs
    // for _, job := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
    //     jobs <- job
    // }
    // close(jobs)

    // TODO: Wait for workers and close results
    // go func() {
    //     wg.Wait()
    //     close(results)
    // }()

    // TODO: Collect and print results
    // for result := range results {
    //     fmt.Println("Result:", result)
    // }
}
```

<details>
<summary>Solution</summary>

```go
func worker(id int, jobs <-chan int, results chan<- int, wg *sync.WaitGroup) {
    defer wg.Done()
    for job := range jobs {
        fmt.Printf("Worker %d processing job %d\n", id, job)
        results <- job * job
    }
}

func main() {
    jobs := make(chan int, 10)
    results := make(chan int, 10)

    var wg sync.WaitGroup
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go worker(i, jobs, results, &wg)
    }

    for _, job := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
        jobs <- job
    }
    close(jobs)

    go func() {
        wg.Wait()
        close(results)
    }()

    for result := range results {
        fmt.Println("Result:", result)
    }
}
```

</details>

---

## Exercise Set 5: JSON & HTTP (After stdlib-essentials)

### Exercise 5.1: JSON API Client ⭐⭐

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// TODO: Create a Todo struct with:
//   - ID (int) json:"id"
//   - Title (string) json:"title"
//   - Completed (bool) json:"completed"

// type Todo struct {
//     
// }

// TODO: Create a fetchTodo function that:
//   - Takes an ID (int)
//   - Makes a GET request to https://jsonplaceholder.typicode.com/todos/{id}
//   - Decodes the JSON response into a Todo struct
//   - Returns (*Todo, error)
//   - Use a client with 10 second timeout

// func fetchTodo(id int) (*Todo, error) {
//     
// }

func main() {
    todo, err := fetchTodo(1)
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Todo #%d: %s (completed: %v)\n", todo.ID, todo.Title, todo.Completed)
}
```

<details>
<summary>Solution</summary>

```go
type Todo struct {
    ID        int    `json:"id"`
    Title     string `json:"title"`
    Completed bool   `json:"completed"`
}

func fetchTodo(id int) (*Todo, error) {
    client := &http.Client{Timeout: 10 * time.Second}

    url := fmt.Sprintf("https://jsonplaceholder.typicode.com/todos/%d", id)
    resp, err := client.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API returned %s", resp.Status)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var todo Todo
    if err := json.Unmarshal(body, &todo); err != nil {
        return nil, err
    }

    return &todo, nil
}
```

</details>

---

### Exercise 5.2: JSON CRUD Server ⭐⭐⭐

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
)

// TODO: Create a Product struct with:
//   - ID (string) json:"id"
//   - Name (string) json:"name"
//   - Price (float64) json:"price"

// type Product struct {
//     
// }

// TODO: Create a ProductStore with:
//   - mu sync.RWMutex
//   - products map[string]Product

// type ProductStore struct {
//     
// }

// TODO: Implement methods:
//   - NewProductStore() *ProductStore
//   - (s *ProductStore) List() []Product
//   - (s *ProductStore) Get(id string) (Product, bool)
//   - (s *ProductStore) Create(p Product)

func main() {
    // TODO: Create store

    // TODO: Register handlers:
    //   GET /api/products → list all products
    //   GET /api/products/{id} → get one product
    //   POST /api/products → create product

    log.Println("Server on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

// Helper functions
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, map[string]string{"error": msg})
}
```

<details>
<summary>Solution</summary>

```go
type Product struct {
    ID    string  `json:"id"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
}

type ProductStore struct {
    mu       sync.RWMutex
    products map[string]Product
}

func NewProductStore() *ProductStore {
    return &ProductStore{
        products: make(map[string]Product),
    }
}

func (s *ProductStore) List() []Product {
    s.mu.RLock()
    defer s.mu.RUnlock()

    result := make([]Product, 0, len(s.products))
    for _, p := range s.products {
        result = append(result, p)
    }
    return result
}

func (s *ProductStore) Get(id string) (Product, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    p, ok := s.products[id]
    return p, ok
}

func (s *ProductStore) Create(p Product) {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.products[p.ID] = p
}

func main() {
    store := NewProductStore()

    http.HandleFunc("GET /api/products", func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, store.List())
    })

    http.HandleFunc("GET /api/products/{id}", func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        product, ok := store.Get(id)
        if !ok {
            writeError(w, http.StatusNotFound, "product not found")
            return
        }
        writeJSON(w, http.StatusOK, product)
    })

    http.HandleFunc("POST /api/products", func(w http.ResponseWriter, r *http.Request) {
        var p Product
        if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
            writeError(w, http.StatusBadRequest, "invalid JSON")
            return
        }
        if p.ID == "" {
            writeError(w, http.StatusBadRequest, "ID required")
            return
        }
        store.Create(p)
        writeJSON(w, http.StatusCreated, p)
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

</details>

---

## Progress Checklist

Mark each exercise as complete:

- [ ] Exercise 1.1: Greeting Function
- [ ] Exercise 1.2: Sum of Slice
- [ ] Exercise 1.3: Word Counter
- [ ] Exercise 2.1: Bank Account
- [ ] Exercise 2.2: Shape Interface
- [ ] Exercise 3.1: Safe Divider
- [ ] Exercise 3.2: Custom Validation Error
- [ ] Exercise 4.1: Parallel Sum
- [ ] Exercise 4.2: Worker Pool
- [ ] Exercise 5.1: JSON API Client
- [ ] Exercise 5.2: JSON CRUD Server

---

## Tips

1. **Don't peek at solutions first** — try for 10-15 minutes before looking
2. **Read error messages carefully** — Go's compiler errors are helpful
3. **Use `go run <file>`** to test quickly
4. **If stuck, add `fmt.Println` statements** to debug
5. **Ask for help** on the Go Discord or Stack Overflow
