# Error Examples

This directory contains example files that demonstrate Risor's error reporting.

## Files

### `multiple_parse_errors.risor`
Shows how Risor reports multiple syntax errors in a single file. Demonstrates:
- Error numbering (`[1/5]`, `[2/5]`, etc.)
- Rust-style location arrows (`-->`)
- Source context with line numbers
- Summary count at the end

### `single_parse_error.risor`
Shows the format for a single syntax error (no numbering).

### `undefined_variable.risor`
Demonstrates the "Did you mean?" feature when you have a typo in a variable name.

### `division_by_zero.risor`
Shows runtime error handling when a Go panic occurs (division by zero), including stack traces.

### `type_error.risor`
Shows a runtime type error (attempting to add incompatible types).

## Running Examples

```bash
# From the repository root:
go run ./cmd/risor ./cmd/risor/testdata/errors/multiple_parse_errors.risor
go run ./cmd/risor ./cmd/risor/testdata/errors/undefined_variable.risor
go run ./cmd/risor ./cmd/risor/testdata/errors/division_by_zero.risor
```

## Expected Output

### Multiple Parse Errors
```
parse error[1/5]: expected an identifier (got {)
  --> ./cmd/risor/testdata/errors/multiple_parse_errors.risor:6:26
   |
 6 | function greet(name, age {
   |                          ^
...
found 5 errors
```

### Undefined Variable ("Did you mean?")
```
[E2001] compile error: undefined variable "firstNme"

  --> ./cmd/risor/testdata/errors/undefined_variable.risor:10:28

10 | let greeting = "Hello, " + firstNme + " " + lastName
   |                            ^

hint: Did you mean 'firstName'?
```

### Division by Zero
```
value error: division by zero

  --> ./cmd/risor/testdata/errors/division_by_zero.risor:14:14

stack trace:
    at __main__ (division_by_zero.risor:14:14)
```
