# Risor Examples

This directory contains examples demonstrating Risor's features and Go integration patterns.

## Risor Scripts

Script examples are in [`scripts/`](./scripts/). Run them with:

```bash
risor examples/scripts/<filename>.risor
```

### Language Fundamentals

| Example                                                | Description                                          |
| ------------------------------------------------------ | ---------------------------------------------------- |
| [fibonacci.risor](./scripts/fibonacci.risor)           | Classic recursive Fibonacci                          |
| [closures.risor](./scripts/closures.risor)             | Closures, memoization, partial application           |
| [destructuring.risor](./scripts/destructuring.risor)   | Object/array destructuring with defaults and aliases |
| [switch_ternary.risor](./scripts/switch_ternary.risor) | Switch statements and ternary expressions            |
| [numeric_types.risor](./scripts/numeric_types.risor)   | Type coercion, bitwise operations, comparisons       |

### Collections and Iteration

| Example                                                  | Description                                         |
| -------------------------------------------------------- | --------------------------------------------------- |
| [list_operations.risor](./scripts/list_operations.risor) | `map`, `filter`, `reduce`, `chunk`, spread operator |
| [range_iteration.risor](./scripts/range_iteration.risor) | Lazy ranges, pagination, batching patterns          |
| [string_methods.risor](./scripts/string_methods.risor)   | String manipulation and methods                     |

### Functional Programming

| Example                                                            | Description                                       |
| ------------------------------------------------------------------ | ------------------------------------------------- |
| [higher_order.risor](./scripts/higher_order.risor)                 | Function composition, pipes, currying             |
| [recursive_algorithms.risor](./scripts/recursive_algorithms.risor) | Sort, search, tree traversal (Risor has no loops) |
| [data_transformation.risor](./scripts/data_transformation.risor)   | E-commerce data pipeline patterns                 |

### Error Handling and Control Flow

| Example                                                      | Description                                         |
| ------------------------------------------------------------ | --------------------------------------------------- |
| [error_handling.risor](./scripts/error_handling.risor)       | `try`/`catch`/`finally`, error inspection, `assert` |
| [optional_chaining.risor](./scripts/optional_chaining.risor) | `?.` safe navigation, `??` nullish coalescing       |
| [validation.risor](./scripts/validation.risor)               | Composable validation rules                         |
| [state_machine.risor](./scripts/state_machine.risor)         | FSM patterns with pure functions                    |

### Modules

| Example                                              | Description                                |
| ---------------------------------------------------- | ------------------------------------------ |
| [math_module.risor](./scripts/math_module.risor)     | Math functions and constants               |
| [time_module.risor](./scripts/time_module.risor)     | Time parsing, formatting, arithmetic       |
| [rand_module.risor](./scripts/rand_module.risor)     | Random numbers, shuffling, UUID generation |
| [regexp_module.risor](./scripts/regexp_module.risor) | Pattern matching, validation, extraction   |
| [encoding.risor](./scripts/encoding.risor)           | JSON, Base64, hex, CSV, URL encoding       |

## Go Integration

Go examples are in [`go/`](./go/). Each example is a standalone module.

### Running Go Examples

```bash
cd examples/go/<example>
go run .
```

### Examples

| Example                                  | Description                                         |
| ---------------------------------------- | --------------------------------------------------- |
| [quickstart](./go/quickstart/)           | Minimal example using `risor.Eval`                  |
| [custom_env](./go/custom_env/)           | Adding custom functions and data to the environment |
| [concurrent_vms](./go/concurrent_vms/)   | Compile once, execute in parallel goroutines        |
| [error_handling](./go/error_handling/)   | Handling Risor errors in Go code                    |
| [expression_eval](./go/expression_eval/) | Dynamic rule evaluation (pricing engine)            |
| [struct](./go/struct/)                   | Exposing Go structs with methods to scripts         |
| [isolated_io](./go/isolated_io/)         | Running scripts with minimal/no builtins            |

## Quick Reference

### Basic Execution

```go
// Empty environment (sandboxed)
result, _ := risor.Eval(ctx, "1 + 2")

// With standard library
result, _ := risor.Eval(ctx, source, risor.WithEnv(risor.Builtins()))

// With custom variables
env := risor.Builtins()
env["input"] = 42
result, _ := risor.Eval(ctx, "input * 2", risor.WithEnv(env))
```

### Compile Once, Run Many

```go
// Compile with template environment
env := risor.Builtins()
env["x"] = 0  // placeholder
code, _ := risor.Compile(ctx, "x * x", risor.WithEnv(env))

// Run with different values (keys must match)
env["x"] = 5
result1, _ := risor.Run(ctx, code, risor.WithEnv(env))

env["x"] = 10
result2, _ := risor.Run(ctx, code, risor.WithEnv(env))
```

### Resource Limits

```go
// Limit execution steps
result, err := risor.Eval(ctx, source,
    risor.WithEnv(risor.Builtins()),
    risor.WithMaxSteps(10000))

// Limit stack depth
result, err := risor.Eval(ctx, source,
    risor.WithEnv(risor.Builtins()),
    risor.WithMaxStackDepth(100))

// Set timeout
result, err := risor.Eval(ctx, source,
    risor.WithEnv(risor.Builtins()),
    risor.WithTimeout(100*time.Millisecond))
```
