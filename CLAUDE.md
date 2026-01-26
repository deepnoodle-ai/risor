# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Risor is a fast, embeddable scripting language for Go developers. Scripts compile to bytecode and run on a lightweight virtual machine. The language integrates Go standard library functions and supports both CLI usage and embedding as a Go library.

## Build Commands

```bash
# Run tests (uses gotestsum)
make test

# Run benchmarks
make bench

# Format code (uses gofumpt)
make format

# Run go generate and format
make generate

# Tidy all module dependencies
make tidy

# Update all dependencies
make update-deps

# Generate coverage report
make cover

# Build and install CLI from source (with optional modules)
cd cmd/risor && go install -tags aws,k8s,vault .

# Install VSCode extension locally
make extension-install
```

## Running a Single Test

```bash
# Run a specific test
go test -v -run TestName ./path/to/package

# Run tests in a specific package
go test ./vm/...
go test ./parser/...
```

## Architecture

### Execution Pipeline
```
Source Code → Lexer (tokens) → Parser (AST) → Compiler (Bytecode) → VM (execution)
```

### Core Components
- `lexer/` - Tokenization
- `parser/` - AST construction (recursive descent parser)
- `compiler/` - Bytecode generation with symbol table for scope tracking
- `vm/` - Virtual machine execution
- `object/` - Type system (67 files) - all Risor values implement `Object` interface
- `builtins/` - Built-in functions (len, print, type conversions, hash functions)
- `modules/` - 48+ modules wrapping Go packages and additional functionality

### Entry Points
- **CLI**: `cmd/risor/` - Uses Cobra framework, includes REPL
- **Language Server**: `cmd/risor-lsp/` - LSP implementation for IDE support
- **Library API**: `risor.Eval(ctx, source, options...)` in `risor.go`

### Module System

Modules either wrap Go standard library packages (e.g., `base64`, `strings`, `json`) or provide additional functionality (e.g., `aws`, `k8s`, `vault`).

**Module pattern:**
```go
func Module() *object.Module {
    return object.NewBuiltinsModule("name", map[string]object.Object{
        "function1": object.NewBuiltin("function1", Function1),
    })
}
```

**Function signature:**
```go
func FunctionName(ctx context.Context, args ...object.Object) object.Object {
    // Validate arguments, convert types, return results
}
```

**Key practices:**
- Validate arguments with `arg.Require()` or similar
- Convert types with `object.AsXXX()` functions (e.g., `object.AsString()`)
- Return errors with `object.NewError()`
- Each module should include a `.md` documentation file

### Optional Modules

Optional modules have their own `go.mod` and require separate `go get`. The CLI builds with `-tags aws,k8s,vault` by default. When embedding Risor, add optional modules via:

```go
risor.Eval(ctx, source, risor.WithGlobals(map[string]any{
    "aws": aws.Module(),
}))
```

### Configuration Options

```go
risor.WithGlobal(name, value)     // Add single global
risor.WithGlobals(map[string]any) // Add multiple globals
risor.WithoutDefaultGlobals()     // Disable standard library
risor.WithConcurrency()           // Enable goroutines/channels
```

## Go Workspace

This is a monorepo using Go workspaces (`go.work`) with 22 modules. Core module requires Go 1.23.0+.

## CI/CD

CircleCI runs three jobs: `test` (with codecov), `generate` (verify code generation), `format` (check gofumpt).
