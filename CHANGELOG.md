# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [2.0.0] - 2026-02-09

Risor v2 is a major release focused on the embedded scripting use case. It
introduces a secure-by-default sandbox, TypeScript-aligned syntax, and a
streamlined Go API.

See [v1 to v2 Migration Guide](docs/guides/migration-v2.md) for upgrade details.

### Added

- **Arrow functions** — concise lambdas: `x => x * 2`, `(a, b) => a + b`
- **Optional chaining** — safe property access: `user?.profile?.name`
- **Nullish coalescing** — defaults: `value ?? "fallback"`
- **try/catch/finally** — keyword-based exception handling as expressions
- **throw** — explicit exception throwing
- **Match expressions** — pattern matching with guard expressions
- **Spread operator** — `{...a, ...b}`, `[...a, ...b]`
- **Destructuring** — `let {name, age} = obj`, `let [a, b] = list`
- **Template strings** — `` `Hello, ${name}!` ``
- **range() builtin** — lazy integer sequences: `range(10)`, `range(1, 10, 2)`
- **error() builtin** — create error values: `error("not found: %s", id)`
- **Map methods** — Python-style methods: `.keys()`, `.values()`, `.entries()`, `.get(key, default)`
- **GoFunc and GoStruct** — Go interop types for embedding
- **TypeRegistry** — explicit type conversion for Go types
- **RisorValuer interface** — automatic Go-to-Risor conversion
- **Resource limits** — step limits, stack depth, and timeouts
- **Execution observer** — hook into VM execution for profiling and debugging
- **Pipe expressions** — `data |> transform |> filter`

### Changed

- **Secure by default** — empty environment unless explicitly configured
- **Parentheses required for if** — `if (condition)` instead of `if condition`
- **Callable returns `(Object, error)`** — explicit error returns throughout
- **BuiltinFunction returns `(Object, error)`** — explicit error returns
- **Object.Equals returns `bool`** — instead of Object
- **Parser/Compiler use Config structs** — replaces functional options
- **byte_slice renamed to bytes**
- **Only 3 built-in modules** — math, rand, regexp

### Removed

- **For loops** — use functional iteration: `.each()`, `.map()`, `.filter()`, `.reduce()`
- **Defer** — use `try/finally`
- **Concurrency primitives** — channels, `go` keyword, `spawn`
- **Import statements** — all functionality comes from the environment
- **Switch/case** — use `match` expressions or if/else
- **Set literals** — use lists
- **Hash comments** — use `//` (shebang still supported)
- **I/O modules** — os, http, exec, ssh, dns, net, bcrypt, filepath, errors, fmt
- **Proxy type** — use TypeRegistry or RisorValuer
- **buffer, set, float_slice types** — use bytes and list
- **try() builtin** — use try/catch
- **delete() builtin** — removed, no replacement

## [1.8.1] - 2025-01-15

Final v1 release. See [v1.8.1 on GitHub](https://github.com/deepnoodle-ai/risor/releases/tag/v1.8.1).
