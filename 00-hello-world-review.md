# Quick Reference: First Go Program

After reading the first two files, you should be able to understand this program.

> ℹ️ **Note:** Some code in this file previews Topics 6-7 (pointers, interfaces). It's OK to not understand everything yet — come back after Phase 2.

> **Before you run this:** Save the code below as `review.go`, then run it with:
> ```bash
> go run review.go
> ```
> `go run` compiles and runs the program in one step. For production, use `go build` instead (covered in Topic 1).

### Key Concepts Before Reading the Code

- **`package main`** — Every Go file starts with a `package` declaration. `package main` means "this is an executable program" (it has a `main()` function). Other packages (like `fmt`, `strings`) are libraries you import.
- **`import`** — Brings in code from the standard library or other modules. `fmt` provides formatted I/O (`Println`, `Printf`). `strings` provides string manipulation functions.
- **`func main()`** — The entry point. When you run the program, Go calls `main()` first.
- **`func` keyword** — Defines a function. Syntax: `func name(params) returnType { body }`.

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

## Exercises

### Exercise 1: Hello, Name! ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Modify the `main()` function so that instead of hardcoding `"Go Learner"`, it reads a name from the command line using `os.Args`. If no name is provided, default to `"World"`.

<details>
<summary>Solution</summary>

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	name := "World"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	fmt.Printf("Hello, %s!\n", name)
}
```

</details>

### Exercise 2: Add Your Own Function ⭐
**Difficulty:** Beginner | **Time:** ~10 min

Add a function called `greet(name string) string` that returns `"Hello, <name>!"`. Call it from `main()` and print the result.

<details>
<summary>Solution</summary>

```go
package main

import "fmt"

func greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func main() {
	msg := greet("Go Learner")
	fmt.Println(msg)
}
```

</details>

### Exercise 3: Predict the Output ⭐⭐
**Difficulty:** Beginner | **Time:** ~10 min

Before running the code below, predict what it prints. Then run it to check.

```go
package main

import "fmt"

func main() {
	name := "Go"
	age := 1
	fmt.Println(name, "is", age, "year(s) old")

	result := add(3, 4)
	fmt.Println("3 + 4 =", result)

	numbers := [3]int{10, 20, 30}
	fmt.Println("Array:", numbers)
	fmt.Println("First:", numbers[0])
}

func add(a, b int) int {
	return a + b
}
```

<details>
<summary>Answer</summary>

```
Go is 1 year(s) old
3 + 4 = 7
Array: [10 20 30]
First: 10
```

</details>

---

## Try It Yourself

1. Copy this code to a file called `review.go`
2. Run it: `go run review.go`
3. Try modifying some parts:
   - Change the name and age
   - Add more fruits to the slice
   - Add more countries to the map
   - Create your own struct

4. After learning concurrency (Topics 11-19), come back and convert this to use goroutines!