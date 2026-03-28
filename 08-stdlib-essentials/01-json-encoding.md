# 1. JSON & Encoding — Complete Guide

> **Goal:** Master JSON marshaling/unmarshaling, struct tags, and handle real-world edge cases.

---

## Table of Contents

1. [JSON Basics](#1-json-basics) `[CORE]`
2. [Struct Tags](#2-struct-tags) `[CORE]`
3. [Unmarshaling (JSON → Go)](#3-unmarshaling-json--go) `[CORE]`
4. [Marshaling (Go → JSON)](#4-marshaling-go--json) `[CORE]`
5. [Nested Structures](#5-nested-structures) `[CORE]`
6. [Custom JSON (Marshaler/Unmarshaler)](#6-custom-json-marshalerunmarshaler) `[PRODUCTION]`
7. [Streaming JSON](#7-streaming-json) `[PRODUCTION]`
8. [Common Pitfalls](#8-common-pitfalls) `[CORE]`

---

## 1. JSON Basics

### Import

```go
import "encoding/json"
```

### Simple Struct to JSON

```go
type User struct {
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email"`
}

func main() {
    user := User{Name: "Alice", Age: 30, Email: "alice@example.com"}

    // Marshal to JSON bytes
    data, err := json.Marshal(user)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(string(data))
    // Output: {"name":"Alice","age":30,"email":"alice@example.com"}
}
```

### JSON to Struct

```go
jsonStr := `{"name":"Bob","age":25,"email":"bob@example.com"}`

var user User
err := json.Unmarshal([]byte(jsonStr), &user)
if err != nil {
    log.Fatal(err)
}

fmt.Println(user.Name)  // Bob
fmt.Println(user.Age)   // 25
```

---

## 2. Struct Tags

Struct tags control how Go fields map to JSON keys.

### Basic Tags

```go
type User struct {
    Name  string `json:"name"`           // JSON key: "name"
    Age   int    `json:"age"`            // JSON key: "age"
    Email string `json:"email_address"`  // JSON key: "email_address" (renamed)
}
```

### Omit Empty Fields

```go
type User struct {
    Name    string `json:"name"`
    Age     int    `json:"age"`
    Bio     string `json:"bio,omitempty"`     // Omit if empty string
    Admin   bool   `json:"admin,omitempty"`   // Omit if false
    Balance float64 `json:"balance,omitempty"` // Omit if 0
}

user := User{Name: "Alice", Age: 30}
data, _ := json.Marshal(user)
fmt.Println(string(data))
// {"name":"Alice","age":30}
// bio, admin, balance omitted because they're zero values
```

### Ignore Field

```go
type User struct {
    Name     string `json:"name"`
    Password string `json:"-"`              // Never serialized to JSON
    Internal int    `json:"-"`              // Never serialized
}
```

### Force String for Numbers

```go
type Config struct {
    Port string `json:"port,string"`  // Port as string in JSON
}

// JSON: {"port":"8080"}
// Go:   Config{Port: "8080"}
```

### Tag Reference

| Tag | Effect |
|-----|--------|
| `json:"name"` | Use "name" as JSON key |
| `json:"name,omitempty"` | Omit field if zero value |
| `json:"-"` | Never include in JSON |
| `json:"name,string"` | Encode number as string |

---

## 3. Unmarshaling (JSON → Go)

### Into a Struct

```go
jsonStr := `{"name":"Alice","age":30,"email":"alice@example.com"}`

var user User
err := json.Unmarshal([]byte(jsonStr), &user)
if err != nil {
    log.Fatal(err)
}
```

### Into a Map (Unknown Structure)

```go
jsonStr := `{"name":"Alice","age":30,"active":true}`

var data map[string]interface{}
err := json.Unmarshal([]byte(jsonStr), &data)
if err != nil {
    log.Fatal(err)
}

// Access values (need type assertion)
name := data["name"].(string)       // "Alice"
age := data["age"].(float64)        // 30.0 (JSON numbers are float64)
active := data["active"].(bool)     // true
```

> **Warning:** `interface{}` requires type assertions. Prefer structs when you know the shape.

### Into a Slice

```go
jsonStr := `[{"name":"Alice"},{"name":"Bob"}]`

var users []User
err := json.Unmarshal([]byte(jsonStr), &users)
if err != nil {
    log.Fatal(err)
}

fmt.Println(len(users))  // 2
```

### Partial Unmarshal (Unknown Fields Ignored)

```go
jsonStr := `{"name":"Alice","age":30,"city":"NYC"}`

// Only name and age will be populated
var user User
json.Unmarshal([]byte(jsonStr), &user)

fmt.Println(user.Name)  // Alice
fmt.Println(user.Age)   // 30
// city is ignored (not in struct)
```

### Unknown Fields → Catch Them

```go
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
    // Catch extra fields
    Extra map[string]interface{} `json:"-"`
}

func (u *User) UnmarshalJSON(data []byte) error {
    // Unmarshal known fields
    type Alias User
    aux := &struct{ *Alias }{Alias: (*Alias)(u)}

    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }

    // Unmarshal all into map to get extra fields
    var all map[string]interface{}
    json.Unmarshal(data, &all)

    u.Extra = make(map[string]interface{})
    known := map[string]bool{"name": true, "age": true}
    for k, v := range all {
        if !known[k] {
            u.Extra[k] = v
        }
    }

    return nil
}
```

---

## 4. Marshaling (Go → JSON)

### Pretty Print

```go
user := User{Name: "Alice", Age: 30, Email: "alice@example.com"}

// Compact
data, _ := json.Marshal(user)

// Pretty (indented)
data, _ = json.MarshalIndent(user, "", "  ")
fmt.Println(string(data))
// {
//   "name": "Alice",
//   "age": 30,
//   "email": "alice@example.com"
// }
```

### Marshal a Slice

```go
users := []User{
    {Name: "Alice", Age: 30},
    {Name: "Bob", Age: 25},
}

data, _ := json.MarshalIndent(users, "", "  ")
fmt.Println(string(data))
// [
//   {"name":"Alice","age":30,"email":""},
//   {"name":"Bob","age":25,"email":""}
// ]
```

### Write JSON to File

```go
func writeJSON(path string, v interface{}) error {
    data, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}
```

### Write JSON to HTTP Response

```go
func writeJSONResponse(w http.ResponseWriter, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(v)  // Streams directly to response
}
```

---

## 5. Nested Structures

### Embedded Structs

```go
type Address struct {
    Street string `json:"street"`
    City   string `json:"city"`
    Zip    string `json:"zip"`
}

type User struct {
    Name    string  `json:"name"`
    Age     int     `json:"age"`
    Address Address `json:"address"`  // Nested struct
}

user := User{
    Name: "Alice",
    Age:  30,
    Address: Address{
        Street: "123 Main St",
        City:   "NYC",
        Zip:    "10001",
    },
}

data, _ := json.MarshalIndent(user, "", "  ")
fmt.Println(string(data))
// {
//   "name": "Alice",
//   "age": 30,
//   "address": {
//     "street": "123 Main St",
//     "city": "NYC",
//     "zip": "10001"
//   }
// }
```

### Embedded (Flattened)

```go
type User struct {
    Name string `json:"name"`
    Address  // Embedded — fields flatten into User
}

user := User{
    Name: "Alice",
    Address: Address{
        Street: "123 Main St",
        City:   "NYC",
    },
}

data, _ := json.MarshalIndent(user, "", "  ")
// {
//   "name": "Alice",
//   "street": "123 Main St",
//   "city": "NYC",
//   "zip": ""
// }
```

### Slices of Slices

```go
type Team struct {
    Name  string   `json:"name"`
    Users []User   `json:"users"`
    Tags  []string `json:"tags"`
}
```

---

## 6. Custom JSON (Marshaler/Unmarshaler)

> ⏭️ **First pass? Skip this section.** Come back after completing projects.

### Custom Marshal

```go
type Timestamp time.Time

func (t Timestamp) MarshalJSON() ([]byte, error) {
    // Format as RFC3339 string
    return []byte(`"` + time.Time(t).Format(time.RFC3339) + `"`), nil
}

func (t *Timestamp) UnmarshalJSON(data []byte) error {
    // Remove quotes
    s := strings.Trim(string(data), `"`)
    parsed, err := time.Parse(time.RFC3339, s)
    if err != nil {
        return err
    }
    *t = Timestamp(parsed)
    return nil
}

// Usage
type Event struct {
    Name      string    `json:"name"`
    CreatedAt Timestamp `json:"created_at"`
}

// JSON: {"name":"login","created_at":"2024-01-15T10:30:00Z"}
```

### Conditional Fields

```go
type Response struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
}

// Success case
resp := Response{Success: true, Data: user}
// {"success":true,"data":{...}}

// Error case
resp = Response{Success: false, Error: "not found"}
// {"success":false,"error":"not found"}
```

### Sensitive Fields (Redact in JSON)

```go
type User struct {
    Name     string `json:"name"`
    Email    string `json:"email"`
    Password string `json:"password,omitempty"`  // Omit if empty
}

// For API responses, use a separate struct
type UserPublic struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func toPublic(u User) UserPublic {
    return UserPublic{Name: u.Name, Email: u.Email}
}
```

---

## 7. Streaming JSON

> ⏭️ **First pass? Skip this section.** Come back after completing projects.

### Decode JSON Array (Large Files)

```go
func decodeLargeJSON(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    decoder := json.NewDecoder(f)

    // Read opening bracket [
    _, err := decoder.Token()
    if err != nil {
        return err
    }

    // Decode each element
    for decoder.More() {
        var user User
        if err := decoder.Decode(&user); err != nil {
            return err
        }
        fmt.Println(user.Name)
    }

    // Read closing bracket ]
    _, err = decoder.Token()
    return err
}
```

### Decode JSON Lines (NDJSON)

```go
// File format:
// {"name":"Alice","age":30}
// {"name":"Bob","age":25}

func decodeJSONLines(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        var user User
        if err := json.Unmarshal(scanner.Bytes(), &user); err != nil {
            return err
        }
        fmt.Println(user.Name)
    }

    return scanner.Err()
}
```

---

## 8. Common Pitfalls

### 1. JSON Numbers are float64

```go
var data map[string]interface{}
json.Unmarshal([]byte(`{"age":30}`), &data)

// WRONG — panic
age := data["age"].(int)

// RIGHT
age := int(data["age"].(float64))

// BETTER — use a struct
type Person struct {
    Age int `json:"age"`
}
```

### 2. Zero Values vs Missing Fields

```go
jsonStr := `{"name":"Alice","age":0}`
var user User
json.Unmarshal([]byte(jsonStr), &user)

// Can't distinguish: was age set to 0 or missing?
// Use *int to detect missing:
type User struct {
    Name string `json:"name"`
    Age  *int   `json:"age"`  // nil if missing, 0 if set to 0
}
```

### 3. Unexported Fields Ignored

```go
type User struct {
    Name  string `json:"name"`   // Exported — included
    email string `json:"email"`  // Unexported — IGNORED by json
}
```

### 4. Interface{} and Type Assertions

```go
var data map[string]interface{}
json.Unmarshal([]byte(`{"items":[1,2,3]}`), &data)

// items is []interface{}, not []int
items := data["items"].([]interface{})
for _, item := range items {
    num := item.(float64)  // Each number is float64
    fmt.Println(num)
}
```

### 5. Circular References

```go
type Node struct {
    Name     string  `json:"name"`
    Children []*Node `json:"children"`
}

// This will stack overflow if nodes reference each other
// Use custom MarshalJSON or break the cycle
```

---

## Quick Reference

```go
// Marshal (Go → JSON)
data, err := json.Marshal(value)
data, err := json.MarshalIndent(value, "", "  ")

// Unmarshal (JSON → Go)
err := json.Unmarshal(data, &value)

// Encoder/Decoder (streaming)
encoder := json.NewEncoder(writer)
encoder.Encode(value)

decoder := json.NewDecoder(reader)
decoder.Decode(&value)

// Struct tags
`json:"name"`              // Rename field
`json:"name,omitempty"`    // Omit if zero value
`json:"-"`                 // Never include
`json:"name,string"`       // Encode as string
```

---

## Exercises

### Exercise 1: Parse JSON to Struct ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Given this JSON string, unmarshal it into a `Person` struct and print the fields:

```json
{"name":"Alice","age":30,"hobbies":["reading","coding"]}
```

<details>
<summary>Solution</summary>

```go
package main

import (
	"encoding/json"
	"fmt"
)

type Person struct {
	Name    string   `json:"name"`
	Age     int      `json:"age"`
	Hobbies []string `json:"hobbies"`
}

func main() {
	jsonStr := `{"name":"Alice","age":30,"hobbies":["reading","coding"]}`

	var p Person
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Name: %s\n", p.Name)
	fmt.Printf("Age: %d\n", p.Age)
	fmt.Printf("Hobbies: %v\n", p.Hobbies)
}
```

</details>

### Exercise 2: Marshal with omitempty ⭐⭐
**Difficulty:** Beginner | **Time:** ~10 min

Create a `Product` struct with `Name`, `Price`, and `Description` (all strings). Marshal it to JSON, but omit `Description` if it's empty.

<details>
<summary>Solution</summary>

```go
package main

import (
	"encoding/json"
	"fmt"
)

type Product struct {
	Name        string `json:"name"`
	Price       string `json:"price"`
	Description string `json:"description,omitempty"`
}

func main() {
	// With description
	p1 := Product{Name: "Widget", Price: "$10", Description: "A nice widget"}
	data1, _ := json.MarshalIndent(p1, "", "  ")
	fmt.Println("With description:")
	fmt.Println(string(data1))

	// Without description (omitted)
	p2 := Product{Name: "Gadget", Price: "$20"}
	data2, _ := json.MarshalIndent(p2, "", "  ")
	fmt.Println("\nWithout description:")
	fmt.Println(string(data2))
}
```

</details>

### Exercise 3: Parse JSON Array ⭐⭐
**Difficulty:** Beginner | **Time:** ~15 min

Parse this JSON array into a slice of structs:

```json
[
  {"name":"Alice","score":95},
  {"name":"Bob","score":87},
  {"name":"Charlie","score":92}
]
```

Print each student's name and score, then calculate the average.

<details>
<summary>Solution</summary>

```go
package main

import (
	"encoding/json"
	"fmt"
)

type Student struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

func main() {
	jsonStr := `[{"name":"Alice","score":95},{"name":"Bob","score":87},{"name":"Charlie","score":92}]`

	var students []Student
	if err := json.Unmarshal([]byte(jsonStr), &students); err != nil {
		fmt.Println("Error:", err)
		return
	}

	total := 0
	for _, s := range students {
		fmt.Printf("%s: %d\n", s.Name, s.Score)
		total += s.Score
	}

	avg := float64(total) / float64(len(students))
	fmt.Printf("\nAverage: %.1f\n", avg)
}
```

</details>

### Exercise 4: API Response Handler ⭐⭐⭐
**Difficulty:** Intermediate | **Time:** ~20 min

Create structs for an API response that looks like this:

```json
{
  "status": "success",
  "data": {
    "users": [
      {"id": 1, "name": "Alice", "active": true},
      {"id": 2, "name": "Bob", "active": false}
    ],
    "total": 2
  },
  "error": null
}
```

Parse it and print only active users.

<details>
<summary>Solution</summary>

```go
package main

import (
	"encoding/json"
	"fmt"
)

type User struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type Data struct {
	Users []User `json:"users"`
	Total int    `json:"total"`
}

type APIResponse struct {
	Status string      `json:"status"`
	Data   Data        `json:"data"`
	Error  interface{} `json:"error"`
}

func main() {
	jsonStr := `{
		"status": "success",
		"data": {
			"users": [
				{"id": 1, "name": "Alice", "active": true},
				{"id": 2, "name": "Bob", "active": false}
			],
			"total": 2
		},
		"error": null
	}`

	var resp APIResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Total: %d\n", resp.Data.Total)
	fmt.Println("\nActive users:")
	for _, u := range resp.Data.Users {
		if u.Active {
			fmt.Printf("  - %s (ID: %d)\n", u.Name, u.ID)
		}
	}
}
```

</details>

---

## Next: [HTTP Basics →](./02-http-basics.md)
