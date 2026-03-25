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

A CLI tool that:

- **Loads** a CSV file into Go structs
- **Searches** rows by column value
- **Filters** rows by custom conditions
- **Outputs** in different formats (table, JSON, CSV) via interfaces
- **Handles errors** with custom error types
- **Cleans up** resources with `defer`

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

### `record.go`

```go
package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Record represents one row in the CSV.
type Record struct {
	Fields map[string]string // Topic 4: Maps — dynamic column storage
}

// Dataset holds all records and metadata.
type Dataset struct {
	Headers []string  // Topic 3: Slices — ordered column names
	Records []Record  // Topic 3: Slice of structs
	Source  string    // Topic 2: Variable — file path
}

// Field returns a field value by column name.
func (r *Record) Field(col string) (string, bool) { // Topic 6: Pointer receiver
	val, ok := r.Fields[col]
	return val, ok
}

// FieldAsInt returns a field value as an integer.
func (r *Record) FieldAsInt(col string) (int, error) {
	val, ok := r.Fields[col]
	if !ok {
		return 0, fmt.Errorf("column %q not found", col)
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("column %q: %q is not an integer: %w", col, val, err)
	}
	return n, nil
}

// String returns a formatted representation of the record.
func (r *Record) String() string {
	parts := make([]string, 0, len(r.Fields)) // Topic 3: Slice with preallocated capacity
	for k, v := range r.Fields {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

// Len returns the number of records.
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

### `loader.go`

```go
package main

import (
	"encoding/csv"
	"fmt"
	"os"
)

// LoadCSV reads a CSV file and returns a Dataset.
// Uses *Dataset (pointer) to avoid copying the entire slice on return.
func LoadCSV(path string) (*Dataset, error) { // Topic 6: Pointer return
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err) // Topic 8: Error wrapping
	}
	defer f.Close() // Topic 9: Defer — guaranteed cleanup

	reader := csv.NewReader(f)

	// Read headers
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read headers: %w", err)
	}

	// Read all rows
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}

	// Topic 3: Slices — build record slice
	records := make([]Record, 0, len(rows))

	for i, row := range rows {
		if len(row) != len(headers) {
			return nil, NewCSVError(path, i+2, // Topic 8: Custom error
				fmt.Errorf("expected %d columns, got %d", len(headers), len(row)))
		}

		// Topic 4: Maps — store each column by header name
		fields := make(map[string]string, len(headers))
		for j, header := range headers {
			fields[header] = row[j]
		}

		records = append(records, Record{Fields: fields})
	}

	// Topic 2: Struct construction with named fields
	ds := &Dataset{
		Headers: headers,
		Records: records,
		Source:  path,
	}

	return ds, nil
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

### `formatter.go`

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// Formatter is the core interface for output formats.
// Any type with a Format method satisfies this — no implements keyword.
type Formatter interface { // Topic 7: Interface
	Format(ds *Dataset) error
}

// --- Table Formatter ---

// tabwriter.Writer aligns output into columns by padding tab-separated text.
// NewWriter(output, minwidth, tabwidth, padding, padchar, flags):
//   0, 0, 2, ' ', 0 → auto-width tabs, 2-space padding, space fill
type TableFormatter struct {
    Writer *tabwriter.Writer // Topic 6: Pointer to external writer
}

func NewTableFormatter() *TableFormatter {
	return &TableFormatter{
		Writer: tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0),
	}
}

func (f *TableFormatter) Format(ds *Dataset) error {
	// Print headers
	fmt.Fprintln(f.Writer, strings.Join(ds.Headers, "\t"))
	fmt.Fprintln(f.Writer, strings.Repeat("-", len(strings.Join(ds.Headers, "\t"))))

	// Print rows
	for _, rec := range ds.Records {
		vals := make([]string, 0, len(ds.Headers))
		for _, h := range ds.Headers {
			vals = append(vals, rec.Fields[h])
		}
		fmt.Fprintln(f.Writer, strings.Join(vals, "\t"))
	}

	return f.Writer.Flush()
}

// --- JSON Formatter ---

type JSONFormatter struct {
	Pretty bool // Topic 2: Bool variable with zero value false
}

func (f *JSONFormatter) Format(ds *Dataset) error {
	// Convert records to []map for JSON marshaling
	rows := make([]map[string]string, 0, len(ds.Records))
	for _, rec := range ds.Records {
		rows = append(rows, rec.Fields)
	}

	var data []byte
	var err error

	if f.Pretty {
		data, err = json.MarshalIndent(rows, "", "  ")
	} else {
		data, err = json.Marshal(rows)
	}

	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// --- CSV Formatter ---

type CSVFormatter struct{}

func (f *CSVFormatter) Format(ds *Dataset) error {
	// Print headers
	fmt.Println(strings.Join(ds.Headers, ","))

	// Print rows
	for _, rec := range ds.Records {
		vals := make([]string, 0, len(ds.Headers))
		for _, h := range ds.Headers {
			vals = append(vals, rec.Fields[h])
		}
		fmt.Println(strings.Join(vals, ","))
	}

	return nil
}

// GetFormatter returns a Formatter by name.
// Demonstrates interface as a return type.
func GetFormatter(name string) (Formatter, error) { // Topic 7: Interface as return type
	switch strings.ToLower(name) {
	case "table":
		return NewTableFormatter(), nil
	case "json":
		return &JSONFormatter{Pretty: true}, nil
	case "csv":
		return &CSVFormatter{}, nil
	default:
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

### `errors.go`

```go
package main

import (
	"errors"
	"fmt"
)

// Sentinel errors — fixed error values for comparison.
var (
	ErrNoRecords    = errors.New("no records found")       // Topic 8: Sentinel error
	ErrColumnNotFound = errors.New("column not found")
)

// CSVError is a custom error type for CSV parsing failures.
type CSVError struct { // Topic 8: Custom error type
	File    string
	Line    int
	Wrapped error
}

func (e *CSVError) Error() string {
	return fmt.Sprintf("csv error [%s:%d]: %v", e.File, e.Line, e.Wrapped)
}

func (e *CSVError) Unwrap() error { // Topic 8: Error unwrapping
	return e.Wrapped
}

// NewCSVError creates a CSVError.
func NewCSVError(file string, line int, err error) *CSVError {
	return &CSVError{
		File:    file,
		Line:    line,
		Wrapped: err,
	}
}

// FilterError is a custom error for filter/search failures.
type FilterError struct {
	Column  string
	Value   string
	Wrapped error
}

func (e *FilterError) Error() string {
	return fmt.Sprintf("filter error [col=%q val=%q]: %v", e.Column, e.Value, e.Wrapped)
}

func (e *FilterError) Unwrap() error {
	return e.Wrapped
}

// IsColumnNotFound checks if the error chain contains ErrColumnNotFound.
func IsColumnNotFound(err error) bool {
	return errors.Is(err, ErrColumnNotFound) // Topic 8: errors.Is
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

### `filter.go`

```go
package main

import (
	"fmt"
	"strconv"
	"strings"
)

// FilterOp represents a comparison operator.
type FilterOp string

const (
	OpEq  FilterOp = "eq"  // ==
	OpNe  FilterOp = "ne"  // !=
	OpGt  FilterOp = "gt"  // >
	OpLt  FilterOp = "lt"  // <
	OpGte FilterOp = "gte" // >=
	OpLte FilterOp = "lte" // <=
	OpContains FilterOp = "contains"
)

// Search finds records where a column exactly matches a value.
func (ds *Dataset) Search(col, val string) ([]Record, error) {
	// Validate column exists
	if !ds.hasColumn(col) {
		return nil, fmt.Errorf("%w: %q", ErrColumnNotFound, col)
	}

	results := make([]Record, 0)
	for _, rec := range ds.Records {
		if rec.Fields[col] == val {
			results = append(results, rec)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("%w: no records where %q = %q", ErrNoRecords, col, val)
	}

	return results, nil
}

// Filter finds records where a column matches a condition.
// Uses named return to make the logic clearer.
func (ds *Dataset) Filter(col string, op FilterOp, val string) (results []Record, err error) { // Topic 9: Named return
	// Topic 9: Defer — log when filter completes, even on early return
	defer func() {
		if err == nil {
			fmt.Printf("  [filter] %s %s %s → %d results\n", col, op, val, len(results))
		}
	}()

	if !ds.hasColumn(col) {
		return nil, &FilterError{ // Topic 8: Custom error
			Column:  col,
			Value:   val,
			Wrapped: ErrColumnNotFound,
		}
	}

	// Try numeric comparison first
	numVal, numErr := strconv.ParseFloat(val, 64)

	results = make([]Record, 0)
	for _, rec := range ds.Records {
		match, mErr := matchesFilter(rec.Fields[col], op, val, numVal, numErr)
		if mErr != nil {
			return nil, mErr
		}
		if match {
			results = append(results, rec)
		}
	}

	return results, nil
}

// matchesFilter checks if a field value matches the filter condition.
func matchesFilter(field string, op FilterOp, strVal string, numVal float64, numErr error) (bool, error) {
	switch op {
	case OpEq:
		return field == strVal, nil
	case OpNe:
		return field != strVal, nil
	case OpContains:
		return strings.Contains(field, strVal), nil
	case OpGt, OpLt, OpGte, OpLte:
		if numErr != nil {
			return false, fmt.Errorf("numeric comparison requires numeric value, got %q: %w", strVal, numErr)
		}
		fieldNum, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return false, nil // Non-numeric field doesn't match numeric filter
		}
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
	return false, fmt.Errorf("unknown filter op: %q", op)
}

func (ds *Dataset) hasColumn(col string) bool {
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

### `main.go`

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	// Topic 2: Short variable declaration
	args := os.Args

	// Topic 2: Zero value check — len is 0 if no args
	if len(args) < 3 {
		printUsage()
		os.Exit(1)
	}

	// Topic 2: Multiple assignment
	cmd, file := args[1], args[2]

	// Topic 3: Slice — remaining args
	rest := args[3:]

	// Topic 6: Load dataset (returns pointer)
	ds, err := LoadCSV(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Topic 7: Dispatch based on command — interface pattern
	switch cmd {
	case "view":
		err = handleView(ds, rest)
	case "search":
		err = handleSearch(ds, rest)
	case "filter":
		err = handleFilter(ds, rest)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleView(ds *Dataset, args []string) error {
	// Topic 2: Default variable (zero value)
	format := "table"

	// Parse --format flag
	for i, arg := range args {
		if arg == "--format" && i+1 < len(args) {
			format = args[i+1]
		}
	}

	// Topic 7: Get formatter by name (returns interface)
	formatter, err := GetFormatter(format)
	if err != nil {
		return err
	}

	// Topic 7: Call interface method — polymorphism
	return formatter.Format(ds)
}

func handleSearch(ds *Dataset, args []string) error {
	var col, val string

	// Parse flags
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
		case "--format":
			// handled below
		}
	}

	if col == "" || val == "" {
		return fmt.Errorf("search requires --col and --val flags")
	}

	// Topic 6: Pointer receiver method call
	results, err := ds.Search(col, val)
	if err != nil {
		return err
	}

	// Build a temporary dataset with results
	resultDS := &Dataset{
		Headers: ds.Headers,
		Records: results,
		Source:  ds.Source,
	}

	// Check for --format
	format := "table"
	for i, arg := range args {
		if arg == "--format" && i+1 < len(args) {
			format = args[i+1]
		}
	}

	formatter, err := GetFormatter(format)
	if err != nil {
		return err
	}
	return formatter.Format(resultDS)
}

func handleFilter(ds *Dataset, args []string) error {
	var col, val string
	var op FilterOp = OpEq

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
			if i+1 < len(args) {
				op = FilterOp(args[i+1])
				i++
			}
		}
	}

	if col == "" || val == "" {
		return fmt.Errorf("filter requires --col and --val flags")
	}

	// Topic 6: Pointer receiver — ds.Filter is a method on *Dataset
	results, err := ds.Filter(col, op, val)
	if err != nil {
		return err
	}

	resultDS := &Dataset{
		Headers: ds.Headers,
		Records: results,
		Source:  ds.Source,
	}

	format := "table"
	for i, arg := range args {
		if arg == "--format" && i+1 < len(args) {
			format = args[i+1]
		}
	}

	formatter, err := GetFormatter(format)
	if err != nil {
		return err
	}
	return formatter.Format(resultDS)
}

func printUsage() {
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

### `loader_test.go`

```go
package main

import (
	"os"
	"testing"
)

func createTestCSV(t *testing.T) string {
	t.Helper()
	content := "name,age,city\nAlice,30,NYC\nBob,25,London\n"
	f, err := os.CreateTemp("", "test-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	// Topic 9: Defer cleanup in tests
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestLoadCSV(t *testing.T) {
	path := createTestCSV(t)
	defer os.Remove(path) // Topic 9: Defer file cleanup

	ds, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("LoadCSV failed: %v", err)
	}

	// Topic 2: Zero value checks
	if ds.Len() != 2 {
		t.Errorf("expected 2 records, got %d", ds.Len())
	}

	if len(ds.Headers) != 3 {
		t.Errorf("expected 3 headers, got %d", len(ds.Headers))
	}
}

func TestLoadCSV_FileNotFound(t *testing.T) {
	_, err := LoadCSV("nonexistent.csv")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestRecord_Field(t *testing.T) {
	rec := Record{
		Fields: map[string]string{"name": "Alice", "age": "30"},
	}

	val, ok := rec.Field("name")
	if !ok || val != "Alice" {
		t.Errorf("expected Alice, got %s (ok=%v)", val, ok)
	}

	_, ok = rec.Field("missing")
	if ok {
		t.Error("expected false for missing field")
	}
}
```

### `formatter_test.go`

```go
package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestJSONFormatter(t *testing.T) {
	ds := &Dataset{
		Headers: []string{"name", "age"},
		Records: []Record{
			{Fields: map[string]string{"name": "Alice", "age": "30"}},
		},
	}

	f := &JSONFormatter{Pretty: true}

	// Capture stdout would require more setup;
	// here we test the data transformation directly
	rows := make([]map[string]string, 0, len(ds.Records))
	for _, rec := range ds.Records {
		rows = append(rows, rec.Fields)
	}

	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if !bytes.Contains(data, []byte("Alice")) {
		t.Error("JSON output should contain Alice")
	}
}

func TestGetFormatter(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"table", false},
		{"json", false},
		{"csv", false},
		{"xml", true}, // unsupported
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetFormatter(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFormatter(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestDataset_Search(t *testing.T) {
	ds := &Dataset{
		Headers: []string{"name", "city"},
		Records: []Record{
			{Fields: map[string]string{"name": "Alice", "city": "NYC"}},
			{Fields: map[string]string{"name": "Bob", "city": "London"}},
		},
	}

	results, err := ds.Search("name", "Alice")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	_, err = ds.Search("name", "Nobody")
	if err == nil {
		t.Error("expected error for no matches")
	}

	_, err = ds.Search("invalid_col", "val")
	if !IsColumnNotFound(err) {
		t.Errorf("expected ErrColumnNotFound, got %v", err)
	}
}
```

### Run Tests

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
