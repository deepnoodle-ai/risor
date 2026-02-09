# Risor v1 to v2 Migration Guide

This guide covers the breaking changes and new features in Risor v2. The v2
release focuses on simplicity, TypeScript-aligned syntax, and a secure-by-default
embedding experience.

## Quick Summary

**Major themes:**

- **Sandboxed by default** — No I/O modules (os, http, exec, etc.)
- **TypeScript-aligned syntax** — Arrow functions, optional chaining, try/catch
- **Functional iteration** — No for loops; use map(), filter(), reduce()
- **Explicit type conversions** — TypeRegistry replaces reflection-based Proxy
- **Resource limits** — Built-in step limits, stack depth, and timeouts

## Syntax Changes

### Parentheses Required for if

v1 used Go-style conditions without parentheses. v2 requires TypeScript-style
parentheses.

```ts
// v1
if x > 0 {
    print(x)
}

// v2
if (x > 0) {
    print(x)
}
```

### Arrow Functions

v2 adds arrow function syntax for concise lambdas.

```ts
// v1
let double = func(x) { return x * 2 }
list.map(func(x) { return x + 1 })

// v2 - both syntaxes work
let double = func(x) { return x * 2 }  // still valid
let double = (x) => x * 2              // arrow function
let double = x => x * 2                // single param, no parens

list.map(x => x + 1)
list.filter(x => x > 0).map(x => x * 2)
```

### Optional Chaining

v2 adds the `?.` operator for safe property access.

```ts
// v1 - manual nil checks
let name = nil
if (user != nil) {
    if (user.profile != nil) {
        name = user.profile.name
    }
}

// v2 - optional chaining
let name = user?.profile?.name

// Also works with method calls
let result = obj?.method()?.value

// Combine with nullish coalescing
let name = user?.profile?.name ?? "anonymous"
```

### try/catch/finally Replaces try() Builtin

v2 uses keyword-based exception handling instead of the `try()` builtin.

```ts
// v1
let result = try(func() {
    return riskyOperation()
})
if (is_error(result)) {
    print("error:", result)
}

// v2 - try is an expression that returns a value
let result = try {
    riskyOperation()
} catch e {
    defaultValue
}

// With finally for cleanup
try {
    openFile()
} catch e {
    handleError(e)
} finally {
    closeFile()  // always runs
}

// Throw exceptions
throw "error message"
throw error("formatted error: %s", details)
```

See [exceptions.md](exceptions.md) for complete documentation.

## Removed Language Features

### For Loops Removed

v2 removes all loop constructs. Use functional iteration instead.

```ts
// v1
for i := 0; i < 10; i++ {
    print(i)
}
for item in items {
    process(item)
}

// v2 - use functional builtins
range(10).each(i => print(i))
items.each(item => process(item))

// Common patterns
items.map(x => x * 2)           // transform each element
items.filter(x => x > 0)        // select elements
items.reduce((a, b) => a + b)   // aggregate to single value

// Use recursion for complex iteration
function countdown(n) {
    if (n <= 0) { return }
    print(n)
    countdown(n - 1)
}
```

### Defer Removed

```ts
// v1
defer cleanup()
doWork()

// v2 - use try/finally
try {
    doWork()
} finally {
    cleanup()
}
```

### Concurrency Primitives Removed

Channels, `go` keyword, and `spawn` are removed. Risor v2 is single-threaded
per VM instance.

```ts
// v1
ch := make(chan, 10)
go func() { ch <- value }()
result := <-ch

// v2 - not supported
// For parallel execution, run multiple VM instances from Go
```

### Import Statements Removed

Module imports are removed. All functionality comes from the environment.

```ts
// v1
import math
from strings import split, join
result = math.sqrt(16)

// v2 - modules are in the environment
result = math.sqrt(16)  // math is provided via WithEnv()
```

### Hash Comments Removed

Only `//` comments are supported. Shebang (`#!`) is still allowed at file start.

```ts
// v1
# this is a comment
x = 1 # inline comment

// v2
// this is a comment
x = 1 // inline comment

#!/usr/bin/env risor  // shebang still works
```

### Set Literals Removed

The `{1, 2, 3}` set literal syntax is removed. Use lists instead.

```ts
// v1
items := {1, 2, 3}

// v2
items = [1, 2, 3]
```

## Removed Types

### buffer, set, float_slice Types

These types are removed entirely. Use `bytes` and `list` instead.

```ts
// v1
b := buffer()
s := set()
f := float_slice()

// v2 - not available
// Use bytes for binary data, list for collections
```

### byte_slice Renamed to bytes

```ts
// v1
data := byte_slice([72, 101, 108, 108, 111])

// v2
data = bytes([72, 101, 108, 108, 111])
```

### Proxy and Go Reflection Types Removed

The `go_*` types (GoType, GoField, GoMethod) and Proxy are removed. Use
TypeRegistry for custom Go type conversions.

```go
// v1 - automatic struct wrapping
env := map[string]any{
    "user": User{Name: "Alice"},  // wrapped as Proxy
}
risor.Eval(ctx, `user.Name`, risor.WithEnv(env))

// v2 - explicit type conversion
registry := risor.NewTypeRegistry().
    RegisterFromGo(reflect.TypeOf(User{}), func(v any) (object.Object, error) {
        u := v.(User)
        return object.NewMap(map[string]object.Object{
            "name": object.NewString(u.Name),
        }), nil
    }).
    Build()

risor.Eval(ctx, `user.name`,
    risor.WithEnv(map[string]any{"user": User{Name: "Alice"}}),
    risor.WithTypeRegistry(registry))

// Or implement RisorValuer on your type
func (u User) RisorValue() object.Object {
    return object.NewMap(map[string]object.Object{
        "name": object.NewString(u.Name),
    })
}
```

## Removed Modules

The following modules are removed to make Risor secure by default:

| Module | Purpose | Alternative |
|--------|---------|-------------|
| `os` | Filesystem, env vars | Provide via custom builtins |
| `http` | HTTP client/server | Provide via custom builtins |
| `exec` | Command execution | Provide via custom builtins |
| `ssh` | SSH connections | Provide via custom builtins |
| `dns` | DNS lookups | Provide via custom builtins |
| `net` | Network operations | Provide via custom builtins |
| `bcrypt` | Password hashing | Provide via custom builtins |
| `filepath` | Path manipulation | Use string operations |
| `errors` | Error utilities | Use error() builtin |
| `fmt` | print/printf | Use custom print builtins |

**Available modules in v2:** `math`, `rand`, `regexp`

To add I/O capabilities, provide custom builtins in your environment:

```go
env := risor.Builtins()
env["readFile"] = myReadFileBuiltin
env["httpGet"] = myHttpGetBuiltin
risor.Eval(ctx, source, risor.WithEnv(env))
```

## Removed Builtins

| Builtin | v2 Alternative |
|---------|----------------|
| `delete(container, key)` | Removed, no replacement |
| `make(type, size)` | Not needed |
| `iter(container)` | Use enumeration methods |
| `is_hashable(value)` | Not needed |
| `try(func)` | `try { } catch e { }` |
| `print(...)` / `printf(...)` | Provide via custom builtins |

## New Features

### range() Builtin

Creates lazy integer sequences (like Python 3).

```ts
range(5)           // 0, 1, 2, 3, 4
range(1, 5)        // 1, 2, 3, 4
range(0, 10, 2)    // 0, 2, 4, 6, 8
range(5, 0, -1)    // 5, 4, 3, 2, 1

// Convert to list
list(range(5))     // [0, 1, 2, 3, 4]

// Use with functional methods
range(10).filter(x => x % 2 == 0).map(x => x * x)
```

### error() Builtin

Creates error values without throwing.

```ts
let err = error("file %s not found", filename)
print(err.message())  // "file example.txt not found"

// Throw when needed
if (!valid) {
    throw error("validation failed: %s", reason)
}
```

### TypeRegistry for Custom Type Conversions

Replace Proxy with explicit type converters.

```go
registry := risor.NewTypeRegistry().
    RegisterFromGo(reflect.TypeOf(Point{}), func(v any) (object.Object, error) {
        p := v.(Point)
        return object.NewMap(map[string]object.Object{
            "x": object.NewInt(int64(p.X)),
            "y": object.NewInt(int64(p.Y)),
        }), nil
    }).
    Build()

result, err := risor.Eval(ctx, "point.x + point.y",
    risor.WithEnv(map[string]any{"point": Point{X: 10, Y: 20}}),
    risor.WithTypeRegistry(registry))
```

### RisorValuer Interface

Go types can implement automatic conversion.

```go
type User struct {
    ID   int
    Name string
}

func (u User) RisorValue() object.Object {
    return object.NewMap(map[string]object.Object{
        "id":   object.NewInt(int64(u.ID)),
        "name": object.NewString(u.Name),
    })
}

// No registry needed - automatic conversion
env := risor.Builtins()
env["user"] = User{ID: 1, Name: "Alice"}
result, _ := risor.Eval(ctx, `user.name`, risor.WithEnv(env))
// result = "Alice"
```

### Resource Limits

Control script execution resources.

```go
// Limit instruction count
result, err := risor.Eval(ctx, source, risor.WithMaxSteps(10000))
if errors.Is(err, risor.ErrStepLimitExceeded) {
    // Script exceeded step limit
}

// Limit stack depth
result, err := risor.Eval(ctx, source, risor.WithMaxStackDepth(100))
if errors.Is(err, risor.ErrStackOverflow) {
    // Script exceeded stack depth
}

// Set execution timeout
result, err := risor.Eval(ctx, source, risor.WithTimeout(100*time.Millisecond))
if errors.Is(err, context.DeadlineExceeded) {
    // Script timed out
}
```

### Execution Observer

Hook into VM execution for profiling, debugging, or coverage.

```go
type myObserver struct{}

func (o *myObserver) OnStep(vm *vm.VM, op op.Op) {}
func (o *myObserver) OnCall(vm *vm.VM, fn object.Object, args []object.Object) {}
func (o *myObserver) OnReturn(vm *vm.VM, result object.Object) {}

result, err := risor.Eval(ctx, source,
    risor.WithEnv(risor.Builtins()),
    risor.WithObserver(&myObserver{}))
```

## Go API Changes

### Object Interface Changes

```go
// v1
type Object interface {
    Equals(other Object) Object    // returned Object (Bool or Error)
    RunOperation(op, right) Object // returned Object (result or Error)
    Cost() int                     // resource tracking
}

// v2
type Object interface {
    Equals(other Object) bool              // returns bool directly
    RunOperation(op, right) (Object, error) // explicit error return
    // Cost() removed - use resource limits instead
}
```

### Callable Interface Changes

```go
// v1
type Callable interface {
    Call(ctx, args) Object  // returned Object (result or Error)
}

// v2
type Callable interface {
    Call(ctx, args) (Object, error)  // explicit error return
}
```

### Builtin Function Signature

```go
// v1
type BuiltinFunction func(ctx context.Context, args ...Object) Object

// v2
type BuiltinFunction func(ctx context.Context, args ...Object) (Object, error)
```

### Parser and Compiler Config

Functional options replaced with Config structs.

```go
// v1
parser.Parse(ctx, source,
    parser.WithFilename("script.risor"))

compiler.Compile(ast,
    compiler.WithGlobalNames([]string{"x", "y"}))

// v2
parser.Parse(ctx, source, &parser.Config{
    Filename: "script.risor",
})

compiler.Compile(ast, &compiler.Config{
    GlobalNames: []string{"x", "y"},
    Filename:    "script.risor",
})
```

### Global Validation

v2 validates that runtime environment matches compile-time expectations.

```go
// Compile with specific globals
code, _ := risor.Compile(ctx, "x + y",
    risor.WithEnv(map[string]any{"x": 1, "y": 2}))

// Run with matching keys (values can differ)
result, _ := risor.Run(ctx, code,
    risor.WithEnv(map[string]any{"x": 10, "y": 20}))  // OK

// Run with different keys - returns error
result, err := risor.Run(ctx, code,
    risor.WithEnv(map[string]any{"a": 1, "b": 2}))  // Error!
// err: "missing required globals: [x, y]"
```

## Migration Checklist

1. **Update syntax:**
   - [ ] Add parentheses to all `if` conditions
   - [ ] Remove `delete()` calls (no replacement)
   - [ ] Replace `try()` builtin with `try/catch` blocks
   - [ ] Replace `#` comments with `//`

2. **Replace loops with functional patterns:**
   - [ ] Replace `for i := 0; i < n; i++` with `range(n).each(...)`
   - [ ] Replace `for item in list` with `list.each(...)` or `list.map(...)`
   - [ ] Replace `break`/`continue` with early returns or filter()

3. **Update removed features:**
   - [ ] Replace `defer` with `try/finally`
   - [ ] Remove `go` keyword and channel usage
   - [ ] Remove import statements
   - [ ] Remove set literals

4. **Update Go embedding code:**
   - [ ] Update Callable implementations to return `(Object, error)`
   - [ ] Update builtin functions to return `(Object, error)`
   - [ ] Replace Proxy usage with TypeRegistry or RisorValuer
   - [ ] Update parser/compiler option usage to Config structs
   - [ ] Add resource limits for untrusted scripts

5. **Provide needed capabilities:**
   - [ ] Add custom builtins for any I/O operations needed
   - [ ] Add custom builtins for print/printf if needed

## Getting Help

- [Language Semantics](semantics.md) — Type behavior and contracts
- [Exception Handling](exceptions.md) — try/catch/finally details
- [Concurrency](concurrency.md) — Thread safety for embedders
- [Type Registry](proposals/type-registry.md) — Custom type conversions
