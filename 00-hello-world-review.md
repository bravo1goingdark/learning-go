# Quick Reference: First Go Program

After reading the first two files, you should be able to understand this program:

```go
package main

import (
    "fmt"
    "strings"
)

func main() {
    // Variables (Topic 2)
    name := "Go Learner"
    age := 1
    
    // Basic types
    var version string = "1.22"
    const language = "Go"
    
    // Print (Topic 2: fmt package)
    fmt.Printf("Hello, %s!\n", name)
    fmt.Printf("You are %d topics into learning %s %s\n", age, language, version)
    
    // Arrays (Topic 3)
    numbers := [5]int{1, 2, 3, 4, 5}
    fmt.Println("Array:", numbers)
    
    // Slices (Topic 3)
    fruits := []string{"apple", "banana", "cherry"}
    fruits = append(fruits, "date")
    fmt.Println("Slice:", fruits)
    
    // Maps (Topic 4)
    capitals := map[string]string{
        "USA":    "Washington D.C.",
        "UK":     "London",
        "France": "Paris",
    }
    fmt.Println("Map:", capitals)
    fmt.Println("Capital of France:", capitals["France"])
    
    // Structs (Topic 5)
    person := Person{
        Name: name,
        Age:  age,
    }
    fmt.Println("Struct:", person)
    fmt.Println("Person's name:", person.Name)
    
    // Functions (Topic 2)
    result := add(10, 20)
    fmt.Println("10 + 20 =", result)
    
    // Methods (Topic 5)
    person.Greet()
    
    // Pointers (Topic 6)
    p := &person
    p.Age = 2
    fmt.Println("Updated person:", person)
    
    // Interface (Topic 7)
    var printer Printer = &person
    printer.Print()
    
    // Error handling (Topic 8)
    msg, err := divide(10, 2)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("10 / 2 =", msg)
    }
    
    // Defer (Topic 9)
    defer fmt.Println("\n--- Program finished ---")
}

// Function with multiple return values (Topic 8)
func divide(a, b int) (int, error) {
    if b == 0 {
        return 0, fmt.Errorf("cannot divide by zero")
    }
    return a / b, nil
}

// Simple function
func add(a, b int) int {
    return a + b
}

// Struct definition (Topic 5)
type Person struct {
    Name string
    Age  int
}

// Method on struct (value receiver - Topic 5)
func (p Person) Greet() {
    fmt.Printf("Hello, my name is %s and I am %d years old\n", p.Name, p.Age)
}

// Method with pointer receiver (Topic 6)
func (p *Person) Print() {
    fmt.Printf("Person{Name: %s, Age: %d}\n", p.Name, p.Age)
}

// Interface definition (Topic 7)
type Printer interface {
    Print()
}

// Note: Person already implements Printer via (p *Person) Print()
```

---

## What You've Learned

After completing topics 1-9, you should understand every part of this program:

| Line(s) | Topic |
|---------|-------|
| 8-12 | Variables & Zero Values |
| 15-16 | Constants |
| 19-20 | fmt package |
| 23-24 | Arrays |
| 27-28 | Slices & append |
| 31-35 | Maps |
| 38-41 | Structs |
| 45 | Function definitions |
| 48 | Methods (value receiver) |
| 52-53 | Pointers |
| 56-57 | Interfaces |
| 60-66 | Error handling |
| 69 | Defer |

---

## Try It Yourself

1. Copy this code to a file called `review.go`
2. Run it: `go run review.go`
3. Try modifying some parts:
   - Change the name and age
   - Add more fruits to the slice
   - Add more countries to the map
   - Create your own struct

4. After learning concurrency (Topics 10-18), come back and convert this to use goroutines!