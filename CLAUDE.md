# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Risor is a fast, embedded scripting language for Go. Scripts compile to bytecode
and run on a lightweight virtual machine.

We are making major modifications for a new `v2` version of the library.

Our priorities when designing and building `v2`:

- **Correctness** — Well-defined behavior
- **Clarity** — One way to do things, explicit over implicit, documented contracts
- **Foundation** — Small core that is easily built upon
- **Focused** — Solve embedding use cases well, defer features that add complexity without clear value
- **Elegant** — Intuitive scripting syntax that feels natural and expressive

## Risor Syntax Examples

```ts
// Functions and closures
function makeCounter() {
    let count = 0
    return function() {
        count++
        return count
    }
}
let counter = makeCounter()
counter() // 1
counter() // 2

// Arrow functions with list operations
let numbers = [1, 2, 3, 4, 5]
numbers.filter(x => x > 2).map(x => x * 2) // [6, 8, 10]

// Destructuring and spread
let { name, age } = {name: "Alice", age: 30}
let merged = {...{a: 1}, ...{b: 2}} // {a: 1, b: 2}
```

## Build Commands

```bash
# Run tests (uses gotestsum)
make test

# Run benchmarks
make bench

# Format code (uses gofumpt)
make format

# Tidy all module dependencies
make tidy

# Update all dependencies
make update-deps

# Generate coverage report
make cover

# Build and install CLI from source
cd cmd/risor && go install .

# Install VSCode extension locally
make extension-install
```

## Architecture

### Execution Pipeline

```
Source Code → Lexer (tokens) → Parser (AST) → Compiler (Bytecode) → VM (execution)
```

### Core Components

- `internal/lexer/` - Tokenization
- `parser/` - AST construction (recursive descent parser)
- `compiler/` - Bytecode generation with symbol table for scope tracking
- `vm/` - Virtual machine execution
- `object/` - Type system (~47 files) - all Risor values implement `Object` interface
- `builtins/` - Built-in functions (type conversions, container ops, encode/decode)
- `modules/` - 4 modules: math, rand, regexp, time

### Entry Points

- **CLI**: `cmd/risor/` - Uses Cobra framework, includes REPL
- **Language Server**: `cmd/risor-lsp/` - LSP implementation for IDE support
- **Library API**: `risor.Eval(ctx, source, options...)` in `risor.go`

### Configuration Options

By default, the Risor environment is empty (secure by default). Use `Builtins()`
for the standard library:

```go
// Empty environment (default)
risor.Eval(ctx, "1 + 2")

// With standard library
risor.Eval(ctx, source, risor.WithEnv(risor.Builtins()))

// Custom environment
risor.Eval(ctx, source, risor.WithEnv(map[string]any{"x": 42}))

// Customized standard library
env := risor.Builtins()
delete(env, "math")         // remove module
env["custom"] = myModule    // add custom module
risor.Eval(ctx, source, risor.WithEnv(env))
```

**Options:**

```go
risor.WithEnv(map[string]any)    // Provide environment variables
risor.WithFilename(string)       // Set filename for error messages
risor.WithObserver(Observer)     // Set execution observer for profiling/debugging
```

**Helpers:**

```go
risor.Builtins()                 // Returns standard library as map[string]any
```

## Go Workspace

This is a monorepo using Go workspaces (`go.work`).

## Dependencies

This project uses the [Wonton](https://github.com/deepnoodle-ai/wonton) library for:

- `cli` - CLI framework and utilities
- `assert` - Test assertions

All tests must use Wonton's `assert` package for assertions:

```go
import "github.com/deepnoodle-ai/wonton/assert"

func TestExample(t *testing.T) {
    assert.Equal(t, actual, expected)
    assert.Nil(t, err)
    assert.True(t, condition)
}
```

## CI/CD

GitHub Actions runs three jobs:

- `test` (with codecov)
- `generate` (verify code generation)
- `format` (check gofumpt)
