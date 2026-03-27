# Standard Library Cookbook

> Quick recipes for common tasks in Go. Copy-paste and adapt.

---

## File Operations

### Read Entire File

```go
data, err := os.ReadFile("config.json")
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(data))
```

### Write to File

```go
err := os.WriteFile("output.txt", []byte("Hello, Go!"), 0644)
if err != nil {
    log.Fatal(err)
}
```

### Read File Line by Line

```go
file, err := os.Open("data.txt")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

scanner := bufio.NewScanner(file)
for scanner.Scan() {
    line := scanner.Text()
    fmt.Println(line)
}

if err := scanner.Err(); err != nil {
    log.Fatal(err)
}
```

### Create Directory

```go
os.MkdirAll("path/to/dir", 0755)
```

### List Files in Directory

```go
entries, err := os.ReadDir(".")
if err != nil {
    log.Fatal(err)
}
for _, entry := range entries {
    fmt.Println(entry.Name(), entry.IsDir())
}
```

### Check if File Exists

```go
if _, err := os.Stat("file.txt"); os.IsNotExist(err) {
    fmt.Println("File does not exist")
}
```

---

## String Operations

### Split and Join

```go
// Split
parts := strings.Split("a,b,c", ",")      // ["a", "b", "c"]
words := strings.Fields("hello   world")   // ["hello", "world"]

// Join
joined := strings.Join([]string{"a", "b"}, "-")  // "a-b"
```

### Contains, HasPrefix, HasSuffix

```go
strings.Contains("hello world", "world")   // true
strings.HasPrefix("hello", "he")           // true
strings.HasSuffix("hello", "lo")           // true
```

### Replace

```go
result := strings.Replace("foo bar foo", "foo", "baz", 1)  // "baz bar foo"
result = strings.ReplaceAll("foo bar foo", "foo", "baz")    // "baz bar baz"
```

### Trim

```go
strings.TrimSpace("  hello  ")      // "hello"
strings.TrimPrefix("hello.go", "hello.")  // "go"
strings.TrimSuffix("hello.go", ".go")     // "hello"
```

### Convert Case

```go
strings.ToLower("HELLO")   // "hello"
strings.ToUpper("hello")   // "HELLO"
strings.Title("hello world") // "Hello World"
```

### String to Number

```go
n, err := strconv.Atoi("42")       // int
f, err := strconv.ParseFloat("3.14", 64)  // float64
b, err := strconv.ParseBool("true") // bool
```

### Number to String

```go
s := strconv.Itoa(42)              // "42"
s = strconv.FormatFloat(3.14, 'f', 2, 64)  // "3.14"
s = strconv.FormatBool(true)       // "true"
```

---

## Time Operations

### Current Time

```go
now := time.Now()                    // Current local time
utc := now.UTC()                     // Convert to UTC
unix := now.Unix()                   // Unix timestamp
```

### Format Time

```go
now.Format("2006-01-02 15:04:05")   // "2024-01-15 10:30:00"
now.Format(time.RFC3339)            // "2024-01-15T10:30:00Z"
now.Format("Monday, Jan 2")         // "Monday, Jan 15"
```

### Parse Time

```go
t, err := time.Parse("2006-01-02", "2024-01-15")
t, err := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
```

### Duration

```go
d := 5 * time.Second
d = 30 * time.Minute
d = 2 * time.Hour
d = time.Duration(100) * time.Millisecond

// Parse duration
d, err := time.ParseDuration("1h30m")

// Add duration
future := time.Now().Add(24 * time.Hour)

// Subtract
diff := time.Since(start)   // time.Duration since start
diff = time.Until(deadline) // time.Duration until deadline
```

### Sleep

```go
time.Sleep(2 * time.Second)
```

### Timer & Ticker

```go
// One-time timer
timer := time.NewTimer(5 * time.Second)
<-timer.C  // Blocks until timer fires

// Periodic ticker
ticker := time.NewTicker(1 * time.Second)
defer ticker.Stop()

for range ticker.C {
    fmt.Println("Tick")
}
```

---

## HTTP

### GET Request

```go
resp, err := http.Get("https://api.example.com/data")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()

body, err := io.ReadAll(resp.Body)
```

### POST JSON

```go
data := map[string]string{"name": "Alice"}
jsonBytes, _ := json.Marshal(data)

resp, err := http.Post("https://api.example.com/users", "application/json", bytes.NewReader(jsonBytes))
```

### Custom Request

```go
client := &http.Client{Timeout: 10 * time.Second}

req, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
req.Header.Set("Authorization", "Bearer token123")
req.Header.Set("Accept", "application/json")

resp, err := client.Do(req)
```

### Simple Server

```go
http.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, %s!", r.URL.Query().Get("name"))
})

log.Fatal(http.ListenAndServe(":8080", nil))
```

---

## JSON

### Marshal (Go → JSON)

```go
user := User{Name: "Alice", Age: 30}
data, err := json.Marshal(user)
data, err = json.MarshalIndent(user, "", "  ")  // Pretty
```

### Unmarshal (JSON → Go)

```go
var user User
err := json.Unmarshal(data, &user)
```

### Decode from Reader

```go
decoder := json.NewDecoder(resp.Body)
var user User
decoder.Decode(&user)
```

### Encode to Writer

```go
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(user)
```

---

## Database

### Connect (PostgreSQL)

```go
db, err := sql.Open("postgres", "host=localhost user=myuser dbname=mydb sslmode=disable")
defer db.Close()
db.Ping()
```

### Query Single Row

```go
var name string
err := db.QueryRow("SELECT name FROM users WHERE id = $1", id).Scan(&name)
```

### Query Multiple Rows

```go
rows, err := db.Query("SELECT id, name FROM users")
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    rows.Scan(&id, &name)
}
```

### Insert

```go
var id int
err := db.QueryRow(
    "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
    name, email,
).Scan(&id)
```

### Transaction

```go
tx, err := db.Begin()
defer tx.Rollback()

tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)
tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)

tx.Commit()
```

---

## Logging

### Basic Logging

```go
log.Println("message")
log.Printf("value: %v", value)
log.Fatal("error")  // Prints + exits
```

### Structured Logging (slog)

```go
slog.Info("user created", "name", name, "id", id)
slog.Error("failed", "error", err)
slog.Debug("debug info", "key", value)
```

### JSON Logging

```go
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
slog.Info("message", "key", value)
```

---

## Concurrency

### Goroutine

```go
go func() {
    // async work
}()
```

### WaitGroup

```go
var wg sync.WaitGroup

for i := 0; i < 5; i++ {
    wg.Add(1)
    go func(n int) {
        defer wg.Done()
        fmt.Println(n)
    }(i)
}

wg.Wait()
```

### Mutex

```go
var mu sync.Mutex
var counter int

mu.Lock()
counter++
mu.Unlock()
```

### Channel

```go
ch := make(chan int, 10)  // Buffered
ch <- 42                  // Send
value := <-ch             // Receive
close(ch)                 // Close

// Range over channel
for v := range ch {
    fmt.Println(v)
}
```

### Select

```go
select {
case msg := <-ch1:
    fmt.Println(msg)
case ch2 <- value:
    fmt.Println("sent")
case <-time.After(5 * time.Second):
    fmt.Println("timeout")
}
```

### Context

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Use ctx in HTTP requests, database queries, etc.
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
```

---

## Testing

### Basic Test

```go
func TestAdd(t *testing.T) {
    got := Add(2, 3)
    want := 5
    if got != want {
        t.Errorf("Add(2,3) = %d; want %d", got, want)
    }
}
```

### Table-Driven Test

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        a, b, want int
    }{
        {1, 2, 3},
        {0, 0, 0},
        {-1, 1, 0},
    }

    for _, tt := range tests {
        t.Run(fmt.Sprintf("%d+%d", tt.a, tt.b), func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.want {
                t.Errorf("got %d, want %d", got, tt.want)
            }
        })
    }
}
```

### Benchmark

```go
func BenchmarkAdd(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Add(2, 3)
    }
}
```

---

## Commands

```bash
# Run
go run main.go

# Build
go build -o app .

# Test
go test ./...
go test -v ./...
go test -race ./...
go test -cover ./...
go test -bench=. ./...

# Format
go fmt ./...
goimports -w .

# Lint
go vet ./...
staticcheck ./...

# Dependencies
go mod init module-name
go mod tidy
go get package@version

# Documentation
go doc fmt.Println
go doc -all strings
```
