# 10. Final Project — CLI CSV Processor

> **Goal:** Build a real CLI tool that combines **all 9 topics** into one working project. If you can build this, you know Go.

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Project Structure](#2-project-structure)
3. [Step 1 — Module Setup (Toolchain)](#3-step-1--module-setup-toolchain)
4. [Step 2 — Define Types (Structs + Variables)](#4-step-2--define-types-structs--variables)
5. [Step 3 — Load CSV (Slices + Maps + Pointers)](#5-step-3--load-csv-slices--maps--pointers)
6. [Step 4 — Define Interfaces](#6-step-4--define-interfaces)
7. [Step 5 — Custom Errors (Error Handling)](#7-step-5--custom-errors-error-handling)
8. [Step 6 — Resource Cleanup (Defer)](#8-step-6--resource-cleanup-defer)
9. [Step 7 — CLI Commands (main.go)](#9-step-7--cli-commands-maingo)
10. [Step 8 — Tests](#10-step-8--tests)
11. [Build & Run](#11-build--run)
12. [Concept Map](#12-concept-map)

---

## 1. Project Overview

### What We're Building

A CLI tool that processes CSV files with these operations:

- **View** — Display all records in a formatted table
- **Search** — Find records by exact column match
- **Filter** — Filter records by conditions (>, <, >=, <=, contains)

### Why This Project?

This project combines **all 9 foundational topics** into one cohesive application:

| Why This Matters | Explanation |
|-----------------|-------------|
| **Real-world usage** | Every company works with CSV data — this is immediately useful |
| **Complete coverage** | Every Go concept appears naturally in CSV processing |
| **Gradual complexity** | Starts simple (view), adds layers (search, filter, format) |
| **No dependencies** | Pure Go standard library — nothing to install |

### How It Works (Intuition)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CSV PROCESSOR FLOW                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   CSV File ──► Loader.Parse() ──► Dataset struct ──► Formatter.Output()   │
│                    │                    │                    │             │
│                    ▼                    ▼                    ▼             │
│              ┌──────────┐         ┌──────────┐         ┌──────────┐        │
│              │ Slices   │         │ Records  │         │ Table    │        │
│              │ Maps     │   ──►   │ (struct) │   ──►   │ JSON     │        │
│              │ Pointers │         │ Methods  │         │ CSV      │        │
│              │ Defer    │         │ Interfaces          │          │        │
│              └──────────┘         └──────────┘         └──────────┘        │
│                                                                             │
│   Operations:                                                              │
│   • view   → Load → Format(table/json/csv)                                 │
│   • search → Load → Filter(exact) → Format                                │
│   • filter → Load → Filter(conditional) → Format                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key insight:** The flow is always **Load → Process → Output**. Each phase uses different Go concepts:

- **Load:** File I/O, slices, maps, pointers, defer
- **Process:** Structs, methods, interfaces, error handling
- **Output:** Interfaces (polymorphism), formatters

### Sample CSV (`data.csv`)

```
name,age,city,email
Alice,30,New York,alice@example.com
Bob,25,London,bob@example.com
Charlie,35,Paris,charlie@example.com
Diana,28,Tokyo,diana@example.com
Eve,22,Berlin,eve@example.com
```

### CLI Usage

```bash
# View all records as a table
csvproc view data.csv

# Search for a record
csvproc search data.csv --col name --val Alice

# Filter records
csvproc filter data.csv --col age --op gt --val 25

# Output as JSON
csvproc view data.csv --format json

# Output as CSV
csvproc filter data.csv --col city --op eq --val London --format csv
```

---

## 2. Project Structure

```
csvproc/
├── go.mod
├── main.go
├── record.go        # Structs, types, variables (Topics 2, 5)
├── loader.go        # CSV loading, slices, maps (Topics 3, 4, 6)
├── formatter.go     # Interfaces for output (Topic 7)
├── errors.go        # Custom error types (Topic 8)
├── filter.go        # Search & filter logic
├── loader_test.go   # Tests
├── formatter_test.go
└── data.csv         # Sample data
```

---

## 3. Step 1 — Module Setup (Toolchain)

> **Topic 1: Go Toolchain** — `go mod`, `go build`, `go test`

```bash
mkdir csvproc && cd csvproc
go mod init github.com/yourname/csvproc
```

Create `data.csv`:

```csv
name,age,city,email
Alice,30,New York,alice@example.com
Bob,25,London,bob@example.com
Charlie,35,Paris,charlie@example.com
Diana,28,Tokyo,diana@example.com
Eve,22,Berlin,eve@example.com
```

---

## 4. Step 2 — Define Types (Structs + Variables)

> **Topic 2: Variables & Zero Values** — declaration forms, basic types  
> **Topic 5: Structs & Methods** — struct fields, methods, embedding

### What / Why / How

**What:** We define `Record` (one row) and `Dataset` (collection of rows) as Go structs.

**Why:**
- `Record` needs flexible columns → use `map[string]string` (Topic 4)
- `Dataset` needs ordered headers + all records → use `[]string` slice (Topic 3)
- Methods on structs let us encapsulate behavior (Topic 5)

**How:**
- `Record.Fields` is a map: column name → value. Maps give O(1) lookup.
- `Dataset.Headers` is a slice: preserves column order (maps are unordered)
- Pointer receivers (`*Record`) let methods modify the struct

### `record.go`

```go
// Package main defines our CSV processing types.
// All files in this project share this package (simple CLI project).

package main

// ============================================================================
// STRUCT DEFINITIONS
// ============================================================================

// Record represents ONE row in the CSV file.
// We use a map[string]string to store columns dynamically.
// Why map? Because CSV columns vary — some files have 3 columns, others have 30.
// A map lets us access any column by name without knowing ahead of time.
//
// Topic 4 (Maps): map[string]string provides O(1) lookup by column name.
// The zero value of map is nil — we must initialize with make() before use.
type Record struct {
	Fields map[string]string // column name -> value
}

// Dataset represents the ENTIRE CSV file.
// Contains headers (column names) and all records.
//
// Why separate Headers from Records?
// - Headers: slice preserves ORDER (maps are unordered)
// - Records: slice for iteration, each Record has its own map
//
// Topic 3 (Slices): []string for ordered headers, []Record for collection
// Topic 2 (Variables): Source is a string variable holding the filename
type Dataset struct {
	Headers []string  // column names in order, e.g., ["name", "age", "city"]
	Records []Record   // all rows from the CSV
	Source  string    // the filename we loaded from (for error messages)
}

// ============================================================================
// METHODS ON RECORD
// ============================================================================

// Field returns a single field value by column name.
// Uses pointer receiver (*Record) because:
// - More efficient: passing 8 bytes (pointer) vs 24 bytes (map header)
// - Allows method to potentially modify the Record later
//
// Returns (value, found) — found is false if column doesn't exist.
// This is idiomatic Go: return both value and "ok" for map lookups.
//
// Topic 6 (Pointers): *Record receiver passes pointer, not copy
func (r *Record) Field(col string) (string, bool) {
	// Two-value map lookup: val, ok = map[key]
	// ok is false if key doesn't exist (vs panic in single-value lookup)
	val, ok := r.Fields[col]
	return val, ok
}

// FieldAsInt converts a field value to integer.
// Returns error if column doesn't exist OR value isn't a valid integer.
// Uses strconv.Atoi (ASCII to integer) for conversion.
//
// Why return (int, error)?
// - Can't use zero for "not found" — 0 might be valid data!
// - Error tells caller EXACTLY what went wrong
//
// Topic 8 (Error Handling): Custom error messages with %q (quoted string)
func (r *Record) FieldAsInt(col string) (int, error) {
	// First check if column exists
	val, ok := r.Fields[col]
	if !ok {
		// fmt.Errorf with %q quotes the column name for clarity
		return 0, fmt.Errorf("column %q not found", col)
	}
	// strconv.Atoi returns error if string isn't a valid integer
	n, err := strconv.Atoi(val)
	if err != nil {
		// %w wraps the original error for error chain inspection
		return 0, fmt.Errorf("column %q: %q is not an integer: %w", col, val, err)
	}
	return n, nil
}

// String returns a human-readable representation of the Record.
// Used for debugging and default printing.
//
// How it works:
// 1. Create slice with pre-allocated capacity (optimization)
// 2. Append "key=value" pairs
// 3. Join with ", " separator
//
// Topic 3 (Slices): make([]string, 0, capacity) pre-allocates
// Topic 2 (Variables): strings.Join is more efficient than +=
func (r *Record) String() string {
	// Pre-allocate with expected size to avoid slice growth
	// Each field becomes "key=value" so capacity = len(Fields)
	parts := make([]string, 0, len(r.Fields))
	for k, v := range r.Fields {
		// fmt.Sprintf is slower but convenient; strings.Builder is faster
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

// ============================================================================
// METHODS ON DATASET
// ============================================================================

// Len returns the number of records in the dataset.
// Simple wrapper around len(Records) — but gives semantic meaning.
//
// Why a method instead of len(ds.Records) everywhere?
// - Clearer intent: ds.Len() vs len(ds.Records)
// - Can change internal representation without breaking callers
func (ds *Dataset) Len() int {
	return len(ds.Records)
}
```

### What This Covers

| Concept | Where |
|---------|-------|
| Struct definition | `Record`, `Dataset` |
| Map field | `Record.Fields map[string]string` |
| Slice fields | `Dataset.Headers []string`, `Dataset.Records []Record` |
| Pointer receiver | `(r *Record) Field(...)` |
| Zero values | `var ds Dataset` — all fields are usable zero values |
| Preallocated slice | `make([]string, 0, len(r.Fields))` |

---

## 5. Step 3 — Load CSV (Slices + Maps + Pointers)

> **Topic 3: Arrays vs Slices** — appending rows, working with slices  
> **Topic 4: Maps** — column lookup by name  
> **Topic 6: Pointers** — passing `*Dataset` to avoid copying

### What / Why / How

**What:** Read a CSV file from disk and parse it into our `Dataset` struct.

**Why:**
- CSV is the world's most common data interchange format
- Loading requires file I/O + parsing + building data structures
- Each Go concept appears naturally in the loading process

**How:**
1. **Open file** → use `os.Open` + `defer Close()` for cleanup
2. **Create CSV reader** → wraps file with CSV parser
3. **Read headers** → first line = column names (slice)
4. **Read rows** → each row = map of header→value
5. **Return pointer** → `*Dataset` avoids copying the entire structure

### Intuition: Why Pointer Return?

```
Pass by Value (copies everything):
  func LoadCSV(path string) Dataset { ... }
  
  When called: ds := LoadCSV("data.csv")
               ┌─────────────────┐
               │ Dataset struct  │  ← COPY of all data
               │ - Headers []    │     (expensive!)
               │ - Records []    │
               └─────────────────┘

Pass by Pointer (shares memory):
  func LoadCSV(path string) *Dataset { ... }
  
  When called: ds := LoadCSV("data.csv")
               ┌─────────────────┐
               │ Dataset struct  │  ← Single copy
               │ - Headers []    │     (pointer to it)
               └─────────────────┘
               │
               ▼
               ds points to original (no copy!)

Result: Pointer return is ~100x faster for large datasets.
```

### `loader.go`

```go
package main

import (
	"encoding/csv" // Go's standard library CSV parser
	"fmt"         // Error formatting
	"os"          // File operations
)

// LoadCSV reads a CSV file and returns a Dataset pointer.
//
// Why return *Dataset instead of Dataset?
// - Dataset contains []Record (potentially millions of elements)
// - Copying that slice would be extremely expensive
// - Returning a pointer shares one copy in memory
//
// Topic 6 (Pointers): *Dataset return type
// Topic 8 (Error Handling): Always return error, never panic on bad input
func LoadCSV(path string) (*Dataset, error) {
	// =========================================================================
	// STEP 1: Open the file
	// =========================================================================
	// os.Open returns (*File, error) — must check error!
	// File handle must be closed to prevent resource leak.
	//
	// Topic 9 (Defer): defer ensures Close() runs even if we return early
	// This is CRITICAL — forgetting defer Close() causes file descriptor leaks.
	f, err := os.Open(path)
	if err != nil {
		// %w wraps the original error so callers can use errors.Is()
		// %q quotes the path for readability
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	// defer runs after function returns — guaranteed cleanup
	defer f.Close()

	// =========================================================================
	// STEP 2: Create CSV reader
	// =========================================================================
	// csv.NewReader wraps *File with a buffered parser.
	// It handles quoted fields, commas in values, etc.
	reader := csv.NewReader(f)

	// =========================================================================
	// STEP 3: Read headers (first line of CSV)
	// =========================================================================
	// reader.Read() returns ([]string, error)
	// Headers become our column names.
	// e.g., ["name", "age", "city"]
	headers, err := reader.Read()
	if err != nil {
		// Error wrapping: "read headers: " + original error
		return nil, fmt.Errorf("read headers: %w", err)
	}

	// =========================================================================
	// STEP 4: Read ALL data rows
	// =========================================================================
	// ReadAll reads remaining rows into memory at once.
	// Good for small-to-medium files.
	// For huge files, you'd use Read() in a loop (streaming).
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}

	// =========================================================================
	// STEP 5: Build Record slice with pre-allocation
	// =========================================================================
	// Pre-allocate the exact capacity we need.
	// Why? append() would double capacity repeatedly (2, 4, 8, 16...)
	// Pre-allocation = O(1) vs O(n) for repeated growth.
	//
	// make([]Record, 0, len(rows)) means:
	// - slice of Records
	// - length 0 (empty)
	// - capacity = len(rows) (exact fit)
	//
	// Topic 3 (Slices): make with capacity for efficiency
	records := make([]Record, 0, len(rows))

	// =========================================================================
	// STEP 6: Parse each row into a Record
	// =========================================================================
	// i is row index (0-based); +2 for human-readable line numbers (header=1)
	// CSV row i+2 because: row 0 = line 2 (after header), row 1 = line 3, etc.
	for i, row := range rows {
		// Validate: every row should have same column count as header
		if len(row) != len(headers) {
			// Create custom error with file context
			// Topic 8 (Errors): Custom error type for CSV-specific errors
			return nil, NewCSVError(path, i+2,
				fmt.Errorf("expected %d columns, got %d", len(headers), len(row)))
		}

		// Create map: header name -> column value
		// Pre-allocate with len(headers) capacity
		fields := make(map[string]string, len(headers))

		// Loop through headers and corresponding row values
		// j is index into headers slice
		for j, header := range headers {
			// row[j] is the value for this header
			fields[header] = row[j]
		}

		// Append the new Record to our slice
		// append() returns new slice (if capacity exceeded, new array allocated)
		records = append(records, Record{Fields: fields})
	}

	// =========================================================================
	// STEP 7: Construct and return Dataset
	// =========================================================================
	// &Dataset{...} creates a pointer to a new Dataset struct.
	// Named field initialization is clearer than positional.
	//
	// Why not return Dataset directly?
	// - Would copy entire struct (Headers slice + Records slice = $$$)
	// - Pointer shares memory, no copy
	//
	// Topic 5 (Structs): Named field initialization
	ds := &Dataset{
		Headers: headers,    // the header row we read first
		Records: records,    // all data rows we parsed
		Source:  path,      // remember filename for errors
	}

	return ds, nil // nil error = success!
}
```

### What This Covers

| Concept | Where |
|---------|-------|
| `defer f.Close()` | Guaranteed file cleanup |
| Error wrapping with `%w` | `fmt.Errorf("open %q: %w", ...)` |
| Pointer return | `func LoadCSV(path string) (*Dataset, error)` |
| Slice with preallocation | `make([]Record, 0, len(rows))` |
| Map creation | `make(map[string]string, len(headers))` |
| Slice append | `records = append(records, ...)` |
| Named struct construction | `Dataset{Headers: headers, ...}` |

---

## 6. Step 4 — Define Interfaces

> **Topic 7: Interfaces (Implicit)** — polymorphism, composition, no `implements`

### What / Why / How

**What:** Define a `Formatter` interface and three implementations: Table, JSON, CSV.

**Why:**
- CLI needs multiple output formats (table for humans, JSON for machines)
- Interface lets us add new formats WITHOUT changing existing code
- Go interfaces are satisfied implicitly — no `implements` keyword needed

**How:**
1. Define `Formatter` interface with one method: `Format(*Dataset) error`
2. Create `TableFormatter`, `JSONFormatter`, `CSVFormatter` structs
3. Each struct implements `Format()` differently
4. `GetFormatter(name)` returns the appropriate implementation

### Intuition: Why Interfaces?

```
WITHOUT INTERFACES (BAD):
  func handleView(ds *Dataset, format string) {
      if format == "table" { ... }
      else if format == "json" { ... }
      else if format == "csv" { ... }
      // Adding new format = modifying this function = breaking existing code
  }

WITH INTERFACES (GOOD):
  type Formatter interface { Format(*Dataset) error }
  
  func handleView(ds *Dataset, formatter Formatter) {
      formatter.Format(ds)  // doesn't care which implementation!
  }
  // Adding new format = new struct, never touch handleView!
```

The key insight: **depend on abstractions (interfaces), not concretions**.

### `formatter.go`

```go
package main

import (
	"encoding/json" // JSON encoding/decoding
	"fmt"           // Fprint for output
	"os"             // os.Stdout for tabwriter
	"strings"        // string manipulation
	"text/tabwriter" // formats output into columns
)

// ============================================================================
// INTERFACE DEFINITION
// ============================================================================

// Formatter is the core interface for output formats.
// Any type with a Format method automatically satisfies this interface!
// No "implements" keyword needed — this is Go's implicit interface.
//
// Why this design?
// - CLI might need table (humans), JSON (APIs), CSV (other tools)
// - Each formatter has DIFFERENT implementation details
// - Interface unifies them: caller just calls Format(), doesn't care which
//
// Topic 7 (Interfaces): Interface as contract, implicit satisfaction
type Formatter interface {
	Format(ds *Dataset) error
}

// ============================================================================
// TABLE FORMATTER
// ============================================================================

// TableFormatter formats records as an aligned text table.
// Uses tabwriter for proper column alignment.
//
// Why tabwriter?
// - Manual column alignment is tedious (counting spaces)
// - tabwriter automatically pads to minimum width
// - Handles varying content lengths gracefully
//
// NewWriter parameters:
// - os.Stdout: write to terminal
// - 0, 0: minwidth, tabwidth (0 = auto)
// - 2: padding between columns
// - ' ': pad character
// - 0: flags (0 = default)
type TableFormatter struct {
	Writer *tabwriter.Writer // Topic 6: Pointer to external type
}

// NewTableFormatter creates a TableFormatter with default settings.
// Constructor pattern: separate creation from initialization.
func NewTableFormatter() *TableFormatter {
	return &TableFormatter{
		// tabwriter.NewWriter returns *tabwriter.Writer (pointer)
		Writer: tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0),
	}
}

// Format implements the Formatter interface for TableFormatter.
// Receives pointer (*TableFormatter) for efficiency.
func (f *TableFormatter) Format(ds *Dataset) error {
	// =========================================================================
	// Print headers as tab-separated row
	// =========================================================================
	// strings.Join concatenates slice with separator
	// "\t" = tab character
	// Example: "name\tage\tcity"
	fmt.Fprintln(f.Writer, strings.Join(ds.Headers, "\t"))

	// =========================================================================
	// Print separator line (dashes matching header width)
	// =========================================================================
	// strings.Repeat creates "------..." based on header length
	// Example: "-------------" for 3 headers
	headerLine := strings.Repeat("-", len(strings.Join(ds.Headers, "\t")))
	fmt.Fprintln(f.Writer, headerLine)

	// =========================================================================
	// Print each data row
	// =========================================================================
	// Range over Records slice
	for _, rec := range ds.Records {
		// Build row: collect values in order of headers
		vals := make([]string, 0, len(ds.Headers))
		
		// Loop through headers to maintain column order
		for _, h := range ds.Headers {
			// rec.Fields[h] is O(1) map lookup (Topic 4)
			vals = append(vals, rec.Fields[h])
		}
		
		// Print tab-separated row
		fmt.Fprintln(f.Writer, strings.Join(vals, "\t"))
	}

	// =========================================================================
	// Flush — CRITICAL! Writes buffered content to stdout
	// =========================================================================
	// Without Flush(), nothing appears!
	// This is deferred in the constructor pattern, but we do it explicitly here.
	return f.Writer.Flush()
}

// ============================================================================
// JSON FORMATTER
// ============================================================================

// JSONFormatter outputs records as JSON array.
// Pretty bool controls indentation (true = human-readable, false = compact).
type JSONFormatter struct {
	Pretty bool // Topic 2: Bool variable, zero value is false
}

// Format implements Formatter for JSONFormatter.
// Converts Dataset to JSON using encoding/json package.
func (f *JSONFormatter) Format(ds *Dataset) error {
	// =========================================================================
	// Convert []Record to []map[string]string for JSON
	// =========================================================================
	// JSON doesn't understand our Record type
	// Must convert to map (JSON object) for marshaling
	rows := make([]map[string]string, 0, len(ds.Records))
	for _, rec := range ds.Records {
		// Copy each record's fields to a new map
		// (not strictly necessary, but clean)
		rows = append(rows, rec.Fields)
	}

	// =========================================================================
	// Marshal to JSON (either pretty or compact)
	// =========================================================================
	var data []byte
	var err error

	if f.Pretty {
		// MarshalIndent: prefix, indent per level
		// Creates multi-line, indented JSON
		data, err = json.MarshalIndent(rows, "", "  ")
	} else {
		// Compact single-line JSON (no whitespace)
		data, err = json.Marshal(rows)
	}

	// Check for marshaling errors (shouldn't happen with simple map)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	// Print to stdout
	fmt.Println(string(data))
	return nil
}

// ============================================================================
// CSV FORMATTER
// ============================================================================

// CSVFormatter outputs records as CSV.
// Simple: just print headers and rows with commas.
type CSVFormatter struct{}

// Format outputs Dataset as CSV (reversible: can load back!)
func (f *CSVFormatter) Format(ds *Dataset) error {
	// Print headers (comma-separated)
	fmt.Println(strings.Join(ds.Headers, ","))

	// Print each row
	for _, rec := range ds.Records {
		// Build row values
		vals := make([]string, 0, len(ds.Headers))
		for _, h := range ds.Headers {
			vals = append(vals, rec.Fields[h])
		}
		// Print comma-separated
		fmt.Println(strings.Join(vals, ","))
	}
	return nil
}

// ============================================================================
// FACTORY FUNCTION
// ============================================================================

// GetFormatter returns a Formatter by name.
// Demonstrates: interface as return type + error for invalid names.
//
// Why return (Formatter, error)?
// - If name is invalid, we can't return a valid formatter
// - Error tells caller what went wrong
//
// Topic 7 (Interfaces): Interface as return type
func GetFormatter(name string) (Formatter, error) {
	// strings.ToLower makes it case-insensitive
	switch strings.ToLower(name) {
	case "table":
		// Return pointer to TableFormatter
		return NewTableFormatter(), nil
	case "json":
		// JSONFormatter with Pretty=true for readability
		return &JSONFormatter{Pretty: true}, nil
	case "csv":
		// CSVFormatter has no config, empty struct
		return &CSVFormatter{}, nil
	default:
		// Error with supported options (helpful for users!)
		return nil, fmt.Errorf("unknown format %q: supported: table, json, csv", name)
	}
}
```

### What This Covers

| Concept | Where |
|---------|-------|
| Interface definition | `Formatter` interface |
| Implicit satisfaction | `TableFormatter`, `JSONFormatter`, `CSVFormatter` all satisfy `Formatter` |
| Interface as return type | `GetFormatter() (Formatter, error)` |
| No `implements` keyword | Types satisfy `Formatter` automatically |
| Pointer to struct | `*TableFormatter`, `*JSONFormatter`, `*CSVFormatter` |

---

## 7. Step 5 — Custom Errors (Error Handling)

> **Topic 8: Error Handling** — custom types, wrapping, sentinel errors

### What / Why / How

**What:** Define custom error types for different failure scenarios.

**Why:**
- Generic errors ("something went wrong") don't help debug
- Custom errors include context: which file, which line, what value
- Sentinel errors let callers check for specific conditions

**How:**
1. **Sentinel errors** — predefined `var` for common failures
2. **Custom error types** — structs with context fields + Error() method
3. **Error wrapping** — chain errors with `%w` for debugging

### Intuition: Error Handling Patterns

```
PATTERN 1: Sentinel Errors (comparison)
─────────────────────────────────────────
  var ErrNotFound = errors.New("not found")
  
  if errors.Is(err, ErrNotFound) { ... }

PATTERN 2: Custom Error Types (structured data)
─────────────────────────────────────────
  type CSVError struct {
      File string
      Line int
      Wrapped error
  }
  
  var csvErr *CSVError
  if errors.As(err, &csvErr) {
      fmt.Println(csvErr.File, csvErr.Line)
  }

PATTERN 3: Error Wrapping (context)
─────────────────────────────────────────
  return nil, fmt.Errorf("open %s: %w", path, err)
  
  // Caller can:
  // - errors.Is(err, os.ErrNotExist) → true
  // - err.Error() → "open data.csv: no such file"
```

### `errors.go`

```go
package main

import (
	"errors"  // errors.Is, errors.As helpers
	"fmt"    // fmt.Errorf for wrapping
)

// ============================================================================
// SENTINEL ERRORS
// ============================================================================

// Sentinel errors are pre-defined error values for common failures.
// They are constants — no data, just identity.
//
// Why sentinel?
// - Callers can compare with errors.Is()
// - "ErrNoRecords" clearly means "no records found"
// - Multiple packages might define same sentinel (e.g., io.EOF)
//
// Topic 8 (Error Handling): Sentinel errors are compared with errors.Is()
var (
	// ErrNoRecords returned when filter/search yields zero results
	ErrNoRecords = errors.New("no records found")
	
	// ErrColumnNotFound returned when column doesn't exist in headers
	ErrColumnNotFound = errors.New("column not found")
)

// ============================================================================
// CUSTOM ERROR TYPE: CSVError
// ============================================================================

// CSVError provides context for CSV parsing failures.
// Includes: which file, which line, what went wrong.
//
// Why a struct?
// - Errors.New() only stores message (no structured data)
// - CSV errors need file + line for user to find the problem
// - Implements error interface so works with fmt.Errorf
//
// Topic 8 (Error Handling): Custom error type with fields
type CSVError struct {
	File    string // Which file had the error
	Line    int    // Which line number (1-indexed)
	Wrapped error  // The underlying error (optional)
}

// Error implements the error interface.
// Required method: Error() string
// Called by fmt.Println, fmt.Errorf, etc.
func (e *CSVError) Error() string {
	// fmt.Sprintf returns string, used in formatted output
	return fmt.Sprintf("csv error [%s:%d]: %v", e.File, e.Line, e.Wrapped)
}

// Unwrap implements error unwrapping for errors.Is/As.
// Returns the wrapped error, enabling the error chain.
//
// Why?
// - errors.Is(err, os.ErrNotExist) checks ALL wrapped errors
// - errors.As(err, &csvErr) finds nested CSVError
// - Without Unwrap(), wrapped errors are invisible!
func (e *CSVError) Unwrap() error {
	return e.Wrapped
}

// NewCSVError is a constructor for CSVError.
// Constructor pattern: ensures proper initialization.
func NewCSVError(file string, line int, err error) *CSVError {
	return &CSVError{
		File:    file,
		Line:    line,
		Wrapped: err, // Can be nil if no underlying error
	}
}

// ============================================================================
// CUSTOM ERROR TYPE: FilterError
// ============================================================================

// FilterError for filter/search operation failures.
// Includes column and value for debugging: "can't filter column X on value Y"
type FilterError struct {
	Column  string // Which column caused the error
	Value   string // What value was being filtered
	Wrapped error  // Underlying error
}

func (e *FilterError) Error() string {
	return fmt.Sprintf("filter error [col=%q val=%q]: %v", e.Column, e.Value, e.Wrapped)
}

func (e *FilterError) Unwrap() error {
	return e.Wrapped
}

// ============================================================================
// ERROR CHECKING HELPERS
// ============================================================================

// IsColumnNotFound is a convenience function for checking column errors.
// Wraps errors.Is for the sentinel ErrColumnNotFound.
//
// Why a function?
// - Cleaner: errors.Is(err, ErrColumnNotFound) vs IsColumnNotFound(err)
// - Documents intent: "check if column not found"
// - Can add logging/handling in one place
func IsColumnNotFound(err error) bool {
	// errors.Is traverses the error chain looking for target
	// Checks: err == target || err.Unwrap() == target || ...
	return errors.Is(err, ErrColumnNotFound)
}
```

### Usage In Other Files

```go
// Creating errors
return nil, ErrNoRecords
return nil, NewCSVError("data.csv", 42, err)

// Checking errors
if errors.Is(err, ErrColumnNotFound) { ... }

// Unwrapping to get details
var csvErr *CSVError
if errors.As(err, &csvErr) {
    fmt.Printf("Failed at %s:%d\n", csvErr.File, csvErr.Line)
}
```

### What This Covers

| Concept | Where |
|---------|-------|
| Sentinel errors | `ErrNoRecords`, `ErrColumnNotFound` |
| Custom error type | `CSVError`, `FilterError` |
| Error wrapping | `NewCSVError(file, line, err)` |
| `Unwrap()` method | Both `CSVError` and `FilterError` |
| `errors.Is` | `IsColumnNotFound()` |
| `errors.As` | Example usage block |

---

## 8. Step 6 — Resource Cleanup (Defer)

> **Topic 9: Defer In Depth** — LIFO order, guaranteed cleanup, named returns

### What / Why / How

**What:** Implement search and filter operations on Dataset.

**Why:**
- Search finds exact matches (name = "Alice")
- Filter handles comparisons (age > 25, city contains "York")
- Defer ensures logging happens even on early returns

**How:**
1. **Search** — simple exact match loop
2. **Filter** — switch on operator, handle numeric vs string
3. **Named returns** — let defer access return values

### Intuition: Why Defer Matters

```
WITHOUT DEFER:
  func Filter(col, op, val) ([]Record, error) {
      if !valid {
          return nil, err  // Must log BEFORE return
      }
      // ... lots of code ...
      if anotherError {
          return nil, err  // Must log HERE too!
      }
      return results, nil  // Must log HERE too!
  }
  // Problem: 3 places to log, easy to forget one!

WITH DEFER:
  func Filter(col, op, val) (results []Record, err error) {
      defer func() {
          if err == nil {
              log("filtered %d results", len(results))
          }
      }()
      
      if !valid {
          return nil, err  // defer runs automatically!
      }
      // ... lots of code ...
      if anotherError {
          return nil, err  // defer runs automatically!
      }
      return results, nil  // defer runs automatically!
  }
  // ONE place handles all cases!
```

### `filter.go`

```go
package main

import (
	"fmt"          // fmt.Errorf for errors
	"strconv"      // ParseFloat for numeric comparison
	"strings"      // Contains for string matching
)

// ============================================================================
// FILTER OPERATORS
// ============================================================================

// FilterOp represents a comparison operator.
// Using type alias (string) gives semantic meaning.
//
// Why string type?
// - Command-line args are strings ("gt", "lt", "eq")
// - Type safety: FilterOp prevents "age + 25" mistakes
// - IDE autocomplete: OpEq. <tab> shows all operators
type FilterOp string

// Define constants for valid operators.
// Grouping makes it clear these are related.
// Topic 2 (Variables): Constants with const block
const (
	OpEq       FilterOp = "eq"  // equal (==)
	OpNe       FilterOp = "ne"  // not equal (!=)
	OpGt       FilterOp = "gt"  // greater than (>)
	OpLt       FilterOp = "lt"  // less than (<)
	OpGte      FilterOp = "gte" // greater or equal (>=)
	OpLte      FilterOp = "lte" // less or equal (<=)
	OpContains FilterOp = "contains" // substring match
)

// ============================================================================
// SEARCH OPERATION
// ============================================================================

// Search finds records where a column exactly matches a value.
// Simple: just loop and compare.
//
// Returns ([]Record, error)
// - []Record: slice of matching records (empty if none)
// - error: only if column doesn't exist
func (ds *Dataset) Search(col, val string) ([]Record, error) {
	// =========================================================================
	// Validate: does column exist?
	// =========================================================================
	// !ds.hasColumn(col) checks headers
	if !ds.hasColumn(col) {
		// %w wraps ErrColumnNotFound for errors.Is() checks
		// This lets callers: errors.Is(err, ErrColumnNotFound)
		return nil, fmt.Errorf("%w: %q", ErrColumnNotFound, col)
	}

	// =========================================================================
	// Search: loop through records, collect matches
	// =========================================================================
	// Pre-allocate: assume some matches, but not all records
	results := make([]Record, 0)
	
	// Simple loop: O(n) where n = number of records
	for _, rec := range ds.Records {
		// Direct map lookup: O(1) per record
		if rec.Fields[col] == val {
			// Found match, append to results
			results = append(results, rec)
		}
	}

	// =========================================================================
	// Check: did we find anything?
	// =========================================================================
	// Zero results isn't always an error, but here we make it one
	// to give helpful feedback to CLI users
	if len(results) == 0 {
		return nil, fmt.Errorf("%w: no records where %q = %q", 
			ErrNoRecords, col, val)
	}

	return results, nil
}

// ============================================================================
// FILTER OPERATION
// ============================================================================

// Filter finds records matching a condition.
// Uses named return values: results and err
//
// Why named returns?
// - Allows defer to access/modify return values
// - Makes code clearer: results is initialized to nil automatically
// - Enables "return early, clean up later" pattern
//
// Topic 9 (Defer): Named return allows deferred functions to see values
func (ds *Dataset) Filter(col string, op FilterOp, val string) 
	(results []Record, err error) {
	
	// =========================================================================
	// DEFERRED LOGGING
	// =========================================================================
	// This defer runs when Filter RETURNS, regardless of how:
	// - return results, nil  → runs
	// - return nil, err      → runs
	// - panic()              → runs (before panic!)
	//
	// Topic 9 (Defer): Guaranteed to run, even on early returns
	defer func() {
		// Only log success (err == nil)
		// This gives users feedback on filter results
		if err == nil {
			fmt.Printf("  [filter] %s %s %s → %d results\n", col, op, val, len(results))
		}
	}()

	// =========================================================================
	// VALIDATE: does column exist?
	// =========================================================================
	if !ds.hasColumn(col) {
		// Return custom error with column context
		// Note: we're returning named 'err', not 'return nil, &FilterError{...}'
		// Both work, but using named return is more explicit here
		return nil, &FilterError{
			Column:  col,
			Value:   val,
			Wrapped: ErrColumnNotFound,
		}
	}

	// =========================================================================
	// PARSE VALUE: try numeric first
	// =========================================================================
	// For >, <, >=, <= we need numeric comparison.
	// Try to parse once, reuse for all rows.
	//
	// strconv.ParseFloat returns (float64, error)
	// numErr != nil means val wasn't a number
	numVal, numErr := strconv.ParseFloat(val, 64)

	// =========================================================================
	// FILTER: loop and apply condition
	// =========================================================================
	// Pre-allocate for potential matches
	results = make([]Record, 0)
	
	for _, rec := range ds.Records {
		// matchesFilter returns (bool, error)
		// Handles string ops (eq, ne, contains) and numeric ops (>, <, etc.)
		match, mErr := matchesFilter(
			rec.Fields[col],  // field value from this record
			op,               // operator
			val,              // filter value as string
			numVal,           // filter value as number (if valid)
			numErr,           // error from parsing (nil = valid number)
		)
		
		// If condition check failed (e.g., comparing non-number to number)
		if mErr != nil {
			return nil, mErr
		}
		
		// If matched, add to results
		if match {
			results = append(results, rec)
		}
	}

	// Return (named values are used automatically)
	return results, nil
}

// ============================================================================
// MATCHESFILTER: condition evaluation
// ============================================================================

// matchesFilter checks if a field value matches a filter condition.
// This is a standalone function (not a method) because:
// - It doesn't need Dataset state
// - Pure function: same inputs → same outputs = easier to test
// - Can be reused across different datasets
func matchesFilter(
	field string,    // Value from the record
	op FilterOp,     // Operator (eq, gt, contains, etc.)
	strVal string,  // Filter value as string
	numVal float64, // Filter value as number (if valid)
	numErr error,   // Error from parsing (nil = valid number)
) (bool, error) {
	
	// =========================================================================
	// STRING OPERATORS: eq, ne, contains
	// =========================================================================
	// These work on any string value
	switch op {
	case OpEq:
		// Simple string equality
		return field == strVal, nil
		
	case OpNe:
		// Not equal
		return field != strVal, nil
		
	case OpContains:
		// Substring matching
		// strings.Contains returns bool, not error
		return strings.Contains(field, strVal), nil
		
	// =========================================================================
	// NUMERIC OPERATORS: gt, lt, gte, lte
	// =========================================================================
	case OpGt, OpLt, OpGte, OpLte:
		// Check if we can do numeric comparison
		if numErr != nil {
			// val wasn't a number, can't compare numerically
			// Example: "age > abc" is invalid
			return false, fmt.Errorf(
				"numeric comparison requires numeric value, got %q: %w", 
				strVal, numErr)
		}
		
		// Parse the field value too
		fieldNum, err := strconv.ParseFloat(field, 64)
		if err != nil {
			// Field isn't a number either — doesn't match numeric filter
			// Example: "age > 25" but name = "Alice" (not a number)
			// We return false (no match), not error
			return false, nil
		}
		
		// Apply numeric comparison
		switch op {
		case OpGt:
			return fieldNum > numVal, nil
		case OpLt:
			return fieldNum < numVal, nil
		case OpGte:
			return fieldNum >= numVal, nil
		case OpLte:
			return fieldNum <= numVal, nil
		}
	}

	// =========================================================================
	// UNKNOWN OPERATOR
	// =========================================================================
	// Shouldn't happen if we validate input, but safety check
	return false, fmt.Errorf("unknown filter op: %q", op)
}

// ============================================================================
// HELPER: column existence check
// ============================================================================

// hasColumn checks if a column name exists in the headers.
// Simple linear search: O(n) where n = number of headers
//
// Why not use map for headers?
// - Headers are small (typically < 100)
// - Order matters for output (we iterate in order)
// - One-time cost: only checked on search/filter, not per-row
func (ds *Dataset) hasColumn(col string) bool {
	// Range over headers slice
	for _, h := range ds.Headers {
		if h == col {
			return true
		}
	}
	return false
}
```

### What This Covers

| Concept | Where |
|---------|-------|
| Named return values | `(results []Record, err error)` |
| Deferred closure | `defer func() { ... }()` |
| Custom error type | `&FilterError{...}` |
| Error wrapping | `fmt.Errorf("...: %w", err)` |
| Constants with `const` | `OpEq`, `OpGt`, etc. |

---

## 9. Step 7 — CLI Commands (main.go)

> **Topic 1: Toolchain** — `go run`, `go build`  
> **Topic 2: Variables** — `:=`, `var`, zero values

### What / Why / How

**What:** The main entry point that parses CLI arguments and dispatches to handlers.

**Why:**
- `main()` is the program start — orchestrates everything
- Command pattern: view/search/filter are different commands
- Flag parsing: `--col`, `--val`, `--format` etc.

**How:**
1. Parse `os.Args` directly (no external flag library)
2. Switch on command (first arg)
3. Load CSV file
4. Execute operation
5. Format and output

### Intuition: CLI Parsing

```
CLI ARGUMENTS STRUCTURE:
  os.Args[0]    = program name ("csvproc")
  os.Args[1]    = command     ("view", "search", "filter")
  os.Args[2]    = file path   ("data.csv")
  os.Args[3:]   = flags       ("--col", "name", "--format", "json")

Example: csvproc filter data.csv --col age --op gt --val 25 --format json
  
  [0] csvproc
  [1] filter
  [2] data.csv
  [3] --col
  [4] age
  [5] --op
  [6] gt
  [7] --val
  [8] 25
  [9] --format
  [10] json
```

### `main.go`

```go
package main

import (
	"fmt" // fmt.Fprintf for error output
	"os"  // os.Args, os.Exit, os.Stderr
)

// ============================================================================
// MAIN ENTRY POINT
// ============================================================================

func main() {
	// =========================================================================
	// PARSE ARGUMENTS
	// =========================================================================
	// os.Args is a []string of command-line arguments.
	// Topic 2 (Variables): Short declaration := combines var + assignment
	args := os.Args

	// =========================================================================
	// VALIDATE: minimum arguments
	// =========================================================================
	// Check we have enough arguments: at least "csvproc <cmd> <file>"
	// If len < 3, missing command or file
	//
	// Topic 2 (Zero Values): len(args) is 0 for empty slice
	// This would be weird (no args at all), but defensive programming
	if len(args) < 3 {
		printUsage()       // Show help
		os.Exit(1)        // Non-zero = error exit
	}

	// =========================================================================
	// EXTRACT COMMAND AND FILE
	// =========================================================================
	// Multiple assignment: extract two values at once
	// Topic 2 (Variables): Multiple assignment is idiomatic Go
	cmd, file := args[1], args[2]

	// =========================================================================
	// REMAINING ARGUMENTS
	// =========================================================================
	// args[3:] is slice syntax: from index 3 to end
	// Contains flags: --col, --val, --format, etc.
	// Topic 3 (Slices): Slicing creates new slice (view, not copy)
	rest := args[3:]

	// =========================================================================
	// LOAD CSV FILE
	// =========================================================================
	// LoadCSV returns (*Dataset, error)
	// We must check error — file might not exist, be malformed, etc.
	//
	// Topic 6 (Pointers): Returns pointer to avoid copying large dataset
	ds, err := LoadCSV(file)
	if err != nil {
		// Print to stderr (not stdout) so error doesn't mix with output
		// %v formats error nicely
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1) // Exit immediately with error
	}

	// =========================================================================
	// DISPATCH BY COMMAND
	// =========================================================================
	// Switch on the command (view/search/filter)
	// Topic 7 (Interfaces): Each handler uses Formatter interface
	switch cmd {
	case "view":
		// view command: display all records
		err = handleView(ds, rest)
	case "search":
		// search command: exact match
		err = handleSearch(ds, rest)
	case "filter":
		// filter command: conditional match
		err = handleFilter(ds, rest)
	default:
		// Unknown command
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		printUsage()
		os.Exit(1)
	}

	// =========================================================================
	// HANDLE ERRORS
	// =========================================================================
	// All handlers return error — check and exit if failed
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	// If we reach here, success!
}

// ============================================================================
// VIEW COMMAND HANDLER
// ============================================================================

// handleView displays all records in the specified format.
// Parameters:
// - ds: loaded dataset
// - args: remaining CLI arguments (flags)
func handleView(ds *Dataset, args []string) error {
	// =========================================================================
	// PARSE FLAGS: --format
	// =========================================================================
	// Default format is "table" (human-readable)
	// Topic 2 (Variables): Zero value of string is ""
	// We set default explicitly: format := "table"
	format := "table"

	// Loop through args looking for --format flag
	// Need bounds check: i+1 < len(args)
	for i, arg := range args {
		// Check each flag position
		if arg == "--format" && i+1 < len(args) {
			// Next argument is the format value
			format = args[i+1]
			// Don't break: could have multiple flags
			// But --format should only appear once
		}
	}

	// =========================================================================
	// GET FORMATTER
	// =========================================================================
	// GetFormatter returns Formatter interface.
	// Topic 7 (Interfaces): We don't know/care which formatter we get
	formatter, err := GetFormatter(format)
	if err != nil {
		// Invalid format: GetFormatter returns error
		return err
	}

	// =========================================================================
	// FORMAT AND OUTPUT
	// =========================================================================
	// Call Format on the interface.
	// Works for table/json/csv without knowing which!
	//
	// Topic 7 (Polymorphism): Different formatters, same call
	return formatter.Format(ds)
}

// ============================================================================
// SEARCH COMMAND HANDLER
// ============================================================================

// handleSearch finds records with exact column match.
func handleSearch(ds *Dataset, args []string) error {
	// =========================================================================
	// PARSE FLAGS: --col and --val (required)
	// =========================================================================
	var col, val string // Both start as ""

	// Loop through args with index
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--col":
			// Check we have a value after --col
			if i+1 < len(args) {
				col = args[i+1]
				i++ // Skip next arg (it's the value, not a flag)
			}
		case "--val":
			if i+1 < len(args) {
				val = args[i+1]
				i++
			}
		case "--format":
			// Will be handled after search
			// Just skip for now
		}
	}

	// =========================================================================
	// VALIDATE REQUIRED FLAGS
	// =========================================================================
	// Both --col and --val are required for search
	if col == "" || val == "" {
		return fmt.Errorf("search requires --col and --val flags")
	}

	// =========================================================================
	// EXECUTE SEARCH
	// =========================================================================
	// Dataset.Search is a method on *Dataset.
	// Topic 6 (Pointers): ds.Search() uses pointer receiver
	results, err := ds.Search(col, val)
	if err != nil {
		// Search returns error if column doesn't exist or no matches
		return err
	}

	// =========================================================================
	// BUILD RESULT DATASET
	// =========================================================================
	// Create a new Dataset with only the matching records.
	// Keep same headers (columns), replace Records with filtered.
	// Topic 5 (Structs): Struct literal with named fields
	resultDS := &Dataset{
		Headers: ds.Headers,    // Same columns
		Records: results,       // Filtered records
		Source:  ds.Source,     // Same source file
	}

	// =========================================================================
	// PARSE FORMAT FLAG
	// =========================================================================
	format := "table"
	for i, arg := range args {
		if arg == "--format" && i+1 < len(args) {
			format = args[i+1]
		}
	}

	// =========================================================================
	// FORMAT AND OUTPUT
	// =========================================================================
	formatter, err := GetFormatter(format)
	if err != nil {
		return err
	}
	return formatter.Format(resultDS)
}

// ============================================================================
// FILTER COMMAND HANDLER
// ============================================================================

// handleFilter finds records matching a condition.
func handleFilter(ds *Dataset, args []string) error {
	// =========================================================================
	// PARSE FLAGS: --col, --val, --op, --format
	// =========================================================================
	var col, val string
	var op FilterOp = OpEq // Default to equals if not specified

	// Topic 2 (Variables): Default value for FilterOp
	// Zero value of FilterOp (string) would be "", we set OpEq

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--col":
			if i+1 < len(args) {
				col = args[i+1]
				i++
			}
		case "--val":
			if i+1 < len(args) {
				val = args[i+1]
				i++
			}
		case "--op":
			// Convert string to FilterOp type
			if i+1 < len(args) {
				op = FilterOp(args[i+1])
				i++
			}
		}
	}

	// =========================================================================
	// VALIDATE REQUIRED FLAGS
	// =========================================================================
	if col == "" || val == "" {
		return fmt.Errorf("filter requires --col and --val flags")
	}

	// =========================================================================
	// EXECUTE FILTER
	// =========================================================================
	// Topic 6 (Pointers): ds.Filter uses pointer receiver
	results, err := ds.Filter(col, op, val)
	if err != nil {
		return err
	}

	// =========================================================================
	// BUILD RESULT DATASET
	// =========================================================================
	resultDS := &Dataset{
		Headers: ds.Headers,
		Records: results,
		Source:  ds.Source,
	}

	// =========================================================================
	// PARSE FORMAT FLAG
	// =========================================================================
	format := "table"
	for i, arg := range args {
		if arg == "--format" && i+1 < len(args) {
			format = args[i+1]
		}
	}

	// =========================================================================
	// FORMAT AND OUTPUT
	// =========================================================================
	formatter, err := GetFormatter(format)
	if err != nil {
		return err
	}
	return formatter.Format(resultDS)
}

// ============================================================================
// USAGE MESSAGE
// ============================================================================

// printUsage shows how to use the CLI.
// Called when arguments are missing or invalid.
func printUsage() {
	// Raw string literal (backticks) preserves formatting
	// No escaping needed for \n, \t, etc.
	fmt.Println(`csvproc — CLI CSV Processor

Usage:
  csvproc view   <file> [--format table|json|csv]
  csvproc search <file> --col <column> --val <value> [--format table|json|csv]
  csvproc filter <file> --col <column> --val <value> [--op eq|ne|gt|lt|gte|lte|contains] [--format table|json|csv]

Examples:
  csvproc view data.csv
  csvproc search data.csv --col name --val Alice
  csvproc filter data.csv --col age --op gt --val 25
  csvproc filter data.csv --col city --op eq --val London --format json`)
}
```

---

## 10. Step 8 — Tests

> **Topic 1: Toolchain** — `go test`, `go test -v`, `go test -run`  
> **Topic 8: Error Handling** — testing error paths

### What / Why / How

**What:** Unit tests for CSV loading, formatting, and filtering.

**Why:**
- Tests verify correctness — don't trust manual testing
- Tests document expected behavior
- Tests catch regressions when you make changes

**How:**
- `testing.T` for test framework
- `t.Helper()` for helper functions
- Table-driven tests for multiple cases

### `loader_test.go`

```go
package main

import (
	"os"      // os.CreateTemp, os.Remove
	"testing" // testing.T
)

// createTestCSV creates a temporary CSV file for testing.
// Returns the filename — caller is responsible for cleanup.
//
// Why a helper function?
// - Avoids duplicating file creation code in every test
// - t.Helper() marks it as test helper (better error messages)
//
// Topic 9 (Defer): Helper also uses defer for cleanup!
func createTestCSV(t *testing.T) string {
	t.Helper() // This function is a helper; failures point to CALLER
	
	// Create temp file with CSV content
	// os.CreateTemp("", "test-*.csv") creates file like "test-123456.csv"
	// Returns (*File, error)
	content := "name,age,city\nAlice,30,NYC\nBob,25,London\n"
	f, err := os.CreateTemp("", "test-*.csv")
	if err != nil {
		t.Fatal(err) // t.Fatal stops test + logs error
	}
	
	// Topic 9 (Defer): Close file when function returns
	// This runs even if we return early due to error!
	defer f.Close()
	
	// Write content to file
	// WriteString returns (bytes, error)
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	
	// Return filename so caller can use it
	return f.Name()
}

// TestLoadCSV tests the happy path: valid CSV loads correctly.
func TestLoadCSV(t *testing.T) {
	// Create test file
	path := createTestCSV(t)
	
	// Topic 9 (Defer): Cleanup temp file after test
	// This is CRITICAL — otherwise /tmp fills with test files!
	defer os.Remove(path)

	// Load the CSV
	ds, err := LoadCSV(path)
	if err != nil {
		// t.Fatalf stops test, formats error like fmt.Errorf
		t.Fatalf("LoadCSV failed: %v", err)
	}

	// Topic 2 (Zero Values): Check length
	// len() on nil slice is 0, so we don't need special handling
	if ds.Len() != 2 {
		t.Errorf("expected 2 records, got %d", ds.Len())
	}

	// Check headers
	if len(ds.Headers) != 3 {
		t.Errorf("expected 3 headers, got %d", len(ds.Headers))
	}
}

// TestLoadCSV_FileNotFound tests error case: missing file.
func TestLoadCSV_FileNotFound(t *testing.T) {
	// Try loading non-existent file
	_, err := LoadCSV("nonexistent.csv")
	
	// Expect error (error should NOT be nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestRecord_Field tests Record.Field() method.
func TestRecord_Field(t *testing.T) {
	// Create a Record directly (no CSV needed)
	rec := Record{
		Fields: map[string]string{"name": "Alice", "age": "30"},
	}

	// Test existing field
	val, ok := rec.Field("name")
	// Check both value AND existence flag
	if !ok || val != "Alice" {
		t.Errorf("expected Alice, got %s (ok=%v)", val, ok)
	}

	// Test missing field
	_, ok = rec.Field("missing")
	if ok {
		// If ok is true, field exists — but we asked for "missing"!
		t.Error("expected false for missing field")
	}
}
```

### `formatter_test.go`

```go
package main

import (
	"bytes"        // bytes.Buffer for capturing output
	"encoding/json"
	"testing"
)

// TestJSONFormatter tests JSON output format.
func TestJSONFormatter(t *testing.T) {
	// Create a Dataset with test data
	ds := &Dataset{
		Headers: []string{"name", "age"},
		Records: []Record{
			{Fields: map[string]string{"name": "Alice", "age": "30"}},
		},
	}

	// Create formatter with pretty printing
	f := &JSONFormatter{Pretty: true}

	// To test output, we manually do what Formatter does
	// (Full stdout capture would require more setup)
	rows := make([]map[string]string, 0, len(ds.Records))
	for _, rec := range ds.Records {
		rows = append(rows, rec.Fields)
	}

	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Check output contains expected data
	// bytes.Contains returns bool
	if !bytes.Contains(data, []byte("Alice")) {
		t.Error("JSON output should contain Alice")
	}
}

// TestGetFormatter tests the formatter factory.
func TestGetFormatter(t *testing.T) {
	// Table-driven test: iterate over test cases
	tests := []struct {
		name    string    // Test case name
		wantErr bool      // Should this return error?
	}{
		{"table", false}, // Valid
		{"json", false},  // Valid
		{"csv", false},   // Valid
		{"xml", true},   // Invalid!
	}

	// Loop through test cases
	for _, tt := range tests {
		// t.Run creates sub-test with name
		// "table", "json", etc.
		t.Run(tt.name, func(t *testing.T) {
			// Call GetFormatter
			_, err := GetFormatter(tt.name)
			
			// Check: (err != nil) should equal wantErr
			if (err != nil) != tt.wantErr {
				// Error state didn't match expectation
				t.Errorf("GetFormatter(%q) error = %v, wantErr %v", 
					tt.name, err, tt.wantErr)
			}
		})
	}
}

// TestDataset_Search tests the Search method.
func TestDataset_Search(t *testing.T) {
	// Create test dataset directly (no CSV file)
	ds := &Dataset{
		Headers: []string{"name", "city"},
		Records: []Record{
			{Fields: map[string]string{"name": "Alice", "city": "NYC"}},
			{Fields: map[string]string{"name": "Bob", "city": "London"}},
		},
	}

	// Test: find existing record
	results, err := ds.Search("name", "Alice")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	// Test: no matches
	_, err = ds.Search("name", "Nobody")
	// Expect error (no results)
	if err == nil {
		t.Error("expected error for no matches")
	}

	// Test: invalid column
	_, err = ds.Search("invalid_col", "val")
	// Check our helper function for column error
	if !IsColumnNotFound(err) {
		t.Errorf("expected ErrColumnNotFound, got %v", err)
	}
}

## Run Tests

```bash
go test -v
go test -run TestLoadCSV
go test -cover
```

---

## 11. Build & Run

```bash
# Initialize
go mod init github.com/yourname/csvproc

# Format
go fmt ./...

# Vet
go vet ./...

# Test
go test -v

# Build binary
go build -o csvproc .

# Run
./csvproc view data.csv
./csvproc search data.csv --col name --val Alice
./csvproc filter data.csv --col age --op gt --val 25
./csvproc filter data.csv --col city --op eq --val London --format json
./csvproc view data.csv --format csv
```

### Expected Output

```
$ ./csvproc view data.csv
name      age   city       email
------    ---   ----       -----
Alice     30    New York   alice@example.com
Bob       25    London     bob@example.com
Charlie   35    Paris      charlie@example.com
Diana     28    Tokyo      diana@example.com
Eve       22    Berlin     eve@example.com

$ ./csvproc filter data.csv --col age --op gt --val 25 --format json
  [filter] age gt 25 → 3 results
[
  {
    "name": "Alice",
    "age": "30",
    "city": "New York",
    "email": "alice@example.com"
  },
  {
    "name": "Charlie",
    "age": "35",
    "city": "Paris",
    "email": "charlie@example.com"
  },
  {
    "name": "Diana",
    "age": "28",
    "city": "Tokyo",
    "email": "diana@example.com"
  }
]
```

---

## 12. Concept Map

Every topic from the 9 files is used in this project:

| # | Topic | Where Used | Example |
|---|-------|------------|---------|
| 01 | **Go Toolchain** | `go mod init`, `go build`, `go test`, `go vet`, `go fmt` | Build & test the project |
| 02 | **Variables & Zero Values** | `:=`, `var`, named returns, zero value defaults | `format := "table"`, `op FilterOp = OpEq` |
| 03 | **Arrays vs Slices** | `[]Record`, `[]string`, `append`, `make` with capacity | `records := make([]Record, 0, len(rows))` |
| 04 | **Maps** | `map[string]string` for columns, column lookup | `rec.Fields[col]` |
| 05 | **Structs & Methods** | `Record`, `Dataset`, `CSVError`, methods on types | `func (ds *Dataset) Search(...)` |
| 06 | **Pointers** | `*Dataset` return, `*Record` receivers, avoid copying | `func LoadCSV(...) (*Dataset, error)` |
| 07 | **Interfaces** | `Formatter` interface, implicit satisfaction, polymorphism | `GetFormatter("json")` returns `Formatter` |
| 08 | **Error Handling** | Custom errors, wrapping, `errors.Is`, sentinel errors | `NewCSVError(...)`, `errors.Is(err, ErrColumnNotFound)` |
| 09 | **Defer** | `defer f.Close()`, deferred closures, named returns | Resource cleanup, filter logging |

---

> **You've now used every Go concept in a real project. Go build something of your own.**
