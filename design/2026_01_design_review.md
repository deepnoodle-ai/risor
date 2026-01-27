# Risor Design Review

A first-principles review of Risor's architecture, identifying areas for improvement and open questions for v2.

## First Principles

**What is Risor?** An embedded scripting language for Go.

**Who are the users?** Go developers adding scriptability to their applications.

**What do embedders need?**

1. **Safety** — Scripts cannot crash the host. Resource usage is bounded.
2. **Clarity** — Behavior is specified, not inferred. APIs are predictable.
3. **Simplicity** — Small API surface. Easy to integrate correctly.
4. **Performance** — Not a bottleneck. But correctness > speed.

**Design maxims for v2:**

- One way to do things (no parallel systems)
- Explicit over implicit (no global state, no magic)
- Small core, rich extensions (minimal interfaces, opt-in capabilities)
- Go-idiomatic (errors, context, options pattern)
- Safe by default (limits, no panics, immutable where possible)
- Document the contract (specify, don't imply)

---

## 1. Object Interface Decomposition

The `Object` interface has 9 methods that every type must implement:

```go
type Object interface {
    Type() Type
    Inspect() string
    Interface() interface{}
    Equals(other Object) Object
    GetAttr(name string) (Object, bool)
    SetAttr(name string, value Object) error
    IsTruthy() bool
    RunOperation(opType op.BinaryOpType, right Object) Object
    Cost() int
}
```

### Problems

1. **RunOperation conflates concerns** — Many types return `TypeErrorf(...)` unconditionally. The method asks "can this type do math?" but the interface implies all types can.

2. **GetAttr/SetAttr on every type** — Even `Nil` has these methods, though they're meaningless. Attributes are not universal.

3. **Cost() is a resource concern** — It's mixed into the core value abstraction. Not all embedders need resource limiting.

### Proposal: Simplified Interface

Keep most methods on the core interface. Extract only `Callable`.

```go
// Core interface - every value
type Object interface {
    Type() Type
    Inspect() string
    Interface() interface{}
    IsTruthy() bool
    Equals(other Object) bool
    GetAttr(name string) (Object, bool)
    SetAttr(name string, value Object) error
    RunOperation(opType op.BinaryOpType, right Object) (Object, error)
}

// Callable - the only capability interface
type Callable interface {
    Call(ctx context.Context, args ...Object) (Object, error)
}
```

**What changed from v1:**

| Method | Change |
|--------|--------|
| `Equals` | Returns `bool` instead of `Object` |
| `RunOperation` | Returns `(Object, error)` instead of `Object` |
| `Cost()` | Removed — Runtime handles resource tracking |
| `Callable` | New interface for invocable types |

**Rationale:**

- **Core stays mostly intact.** GetAttr/SetAttr and RunOperation return sensible defaults or errors for types that don't support them. No need for separate interfaces.
- **Callable is truly distinct.** Only Builtin, Closure, and callable modules can be invoked. The VM needs to know "can I call this?" — a single type assertion is clean.
- **Cost() removed.** Resource tracking is a runtime concern, not a value concern. The Runtime calculates cost based on type.

### Questions

- [ ] Should `Equals` return `bool` (Go-idiomatic) or `Object` (allows error signaling)?

---

## 2. Error Handling Unification

There are two parallel error mechanisms:

### Current State

**Compile-time errors** (`errors` package):
- `EvalError` — unrecoverable evaluation errors (fatal)
- `ArgsError` — wrong argument count (fatal)
- `TypeError` — type mismatches (configurable fatality)

**Runtime errors** (`object.Error`):
- Wraps a Go `error` with metadata
- `raised` flag distinguishes "thrown" vs "returned" errors
- Rich attributes: `.message()`, `.line()`, `.stack()`

**The bridge** — `object.TypeErrorf()` returns `*object.Error` wrapping `*errors.TypeError`:

```go
func TypeErrorf(format string, args ...interface{}) *Error {
    return NewError(newTypeErrorf(format, args...))
}
```

### Problems

1. **Unclear boundaries** — When should code use which system?
2. **The `raised` flag is subtle** — It changes error propagation semantics invisibly.
3. **Builtins return `Object`** — Callers must check if it's an `*Error`, unlike Go's `(T, error)` pattern.

### Proposal Options

**Option A: Builtins return (Object, error)**

```go
type BuiltinFunction func(ctx context.Context, args ...Object) (Object, error)
```

Pros: Go-idiomatic, explicit error handling
Cons: Changes every builtin signature, VM must handle two return values

**Option B: All errors are *Error objects**

Remove `errors` package types. The VM handles all errors uniformly as `*Error` objects.

Pros: Single error type, simpler mental model
Cons: Loses Go error wrapping/unwrapping semantics

**Option C: Keep both, document the boundary**

Compile-time uses `errors` package. Runtime uses `object.Error`. Never mix.

Pros: No code changes
Cons: Doesn't fix the confusion

### Questions

- [ ] Is Go-idiomatic error handling worth the churn?
- [ ] Should `raised` be replaced with explicit throw semantics (different type)?
- [ ] What error information is essential at runtime vs compile-time?

---

## 3. Global Mutable State

**Status: RESOLVED**

The `typeErrorsAreFatal` global and related code was dead — `IsFatal()` was never checked during execution. All errors already stopped execution regardless of this setting.

**Resolution:** Removed the dead code entirely:
- `typeErrorsAreFatal` variable
- `FatalError` interface
- `IsFatal()` methods on all error types
- `SetTypeErrorsAreFatal()` and `AreTypeErrorsFatal()` functions

Type errors are always fatal. This is the simplest and most predictable behavior.

---

## 4. Builtin Type Simplification

```go
type Builtin struct {
    fn             BuiltinFunction
    name           string
    module         *Module
    moduleName     string          // "only used for overriding builtins"
    isErrorHandler bool
}
```

### Problems

1. **Redundant fields** — `module` and `moduleName` overlap. The comment explains `moduleName` is for "overriding" but doesn't explain why both are needed.

2. **isErrorHandler is a special case** — It changes VM invocation semantics. If builtins returned `(Object, error)`, this flag wouldn't be needed.

### Proposal

If we adopt Option A from section 2 (builtins return `(Object, error)`), remove `isErrorHandler`.

For the module fields, pick one:
- Store `*Module` pointer, derive name from it
- Or store just the name string if the pointer isn't needed

### Questions

- [ ] What does "overriding builtins" mean in practice? When does this happen?
- [ ] Is `isErrorHandler` used anywhere other than the VM dispatch?

---

## 5. HashKey Memory Efficiency

```go
type HashKey struct {
    Type     Type
    FltValue float64
    IntValue int64
    StrValue string
}
```

Every hash key carries all three value fields, but only one is used per key.

### Size Analysis

- `Type`: 16 bytes (string header)
- `FltValue`: 8 bytes
- `IntValue`: 8 bytes
- `StrValue`: 16 bytes (string header)
- **Total**: 48 bytes per key

For a map with 1000 integer keys, that's ~48KB of mostly-empty fields.

### Proposal Options

**Option A: Interface-based**

```go
type HashKey interface {
    hashKey()
}

type IntHashKey int64
type FloatHashKey float64
type StringHashKey string

func (IntHashKey) hashKey()    {}
func (FloatHashKey) hashKey()  {}
func (StringHashKey) hashKey() {}
```

**Option B: Discriminated union with unsafe**

```go
type HashKey struct {
    typ  Type
    data [16]byte // Enough for string header or float64
}
```

### Questions

- [ ] Is memory usage actually a problem in practice?
- [ ] Do maps get large enough for this to matter?
- [ ] Is the complexity worth the savings?

---

## 6. Public API Ergonomics

The current API requires explicit stdlib opt-in:

```go
// Common case requires boilerplate
env := risor.Builtins()
risor.Eval(ctx, source, risor.WithEnv(env))
```

### Problem

Users who forget `WithEnv(risor.Builtins())` get an empty environment with no builtins. The secure-by-default design is good, but the common case is verbose.

### Proposal Options

**Option A: Dedicated option**

```go
risor.Eval(ctx, source, risor.WithStdlib())
```

**Option B: Convenience function**

```go
risor.EvalWithStdlib(ctx, source, opts...)
```

**Option C: Builder pattern**

```go
risor.New().
    WithStdlib().
    WithEnv(custom).
    Eval(ctx, source)
```

### Questions

- [ ] Is the current verbosity actually a problem?
- [ ] Should `WithStdlib()` be additive with `WithEnv()`?
- [ ] Does a builder pattern add value or just complexity?

---

## 7. Language Semantics Specification

There is no canonical specification for core language semantics:

- Numeric types and coercions
- Equality/ordering rules
- Truthiness
- Iteration order for maps/objects
- Error propagation and stack traces

### Problem

Embedding use cases require a stable contract. Without an explicit spec, users
must infer behavior from implementation details.

### Proposal

Add a concise "Language Semantics" document (or README section) that is versioned
and stable across v2. It does not need to be exhaustive, but should define the
behavior of core types and operators.

### Questions

- [ ] What is the minimal spec that embedders need?
- [ ] Should this be a public compatibility guarantee for v2?

---

## 8. Execution Resource Limits

The runtime does not expose explicit resource limits (time, steps, memory).
`Compile` also uses `context.Background()` internally, so it cannot be canceled.

### Problem

Embedding demands predictable termination behavior and control over resource use.

### Proposal

- Add `Compile(ctx, source, ...)` (or a `WithContext` option) for cancellation.
- Add VM options for time, step count, and possibly memory / stack depth limits.

### Questions

- [ ] Which limits are essential for v2?
- [ ] Should limits be enforced in the VM, compiler, or both?

---

## 9. Embedding Boundary and Type Conversion

**Status: RESOLVED**

The `TypeRegistry` system now handles Go ↔ Risor type conversion with clear contracts.

### Resolution

Implemented `TypeRegistry` with:
- Explicit, immutable registry for type conversions
- `DefaultRegistry()` with converters for all built-in types
- `RegistryBuilder` for custom type converters
- `RisorValuer` interface for types that self-convert
- VM-owned registry (no global mutable state)

**API:**
```go
// Use default conversion
risor.Eval(ctx, source, risor.WithEnv(map[string]any{"x": 42}))

// Custom type conversion
registry := risor.NewTypeRegistry().
    RegisterFromGo(reflect.TypeOf(MyType{}), convertMyType).
    Build()
risor.Eval(ctx, source, risor.WithTypeRegistry(registry))

// Self-converting types
type User struct { ID int; Name string }
func (u User) RisorValue() object.Object { ... }
// Works automatically without registry configuration
```

**Supported types by default:**
- Primitives: bool, int/int8/.../int64, uint/uint8/.../uint64, float32/float64, string
- Containers: slices, arrays, maps (string keys)
- Special: []byte, time.Time, json.Number
- Pointers: automatically dereferenced

**Unsupported types** (e.g., functions, channels) cause a panic at VM creation time, providing immediate feedback rather than mysterious runtime failures.

See `docs/proposals/type-registry.md` for full documentation.

---

## 10. Concurrency and Mutability Contract

`Compile` notes bytecode is safe for concurrent use, but environment values are
passed as a `map[string]any`. It is unclear whether values are copied, shared,
or assumed to be immutable.

### Problem

Ambiguous mutability makes safe embedding difficult, especially in concurrent
execution.

### Proposal

Document and enforce the concurrency contract:

- Are env values deep-copied or shared?
- Are builtins or module values expected to be thread-safe?

### Questions

- [ ] Should `WithEnv` defensively copy values or only the map?
- [ ] Do we need a "pure/immutable" module convention?

---

## 11. Observability Stability

The observer API is a strong hook, but there is no explicit stability promise.
Tooling (profilers, debuggers, coverage) will depend on these events.

### Proposal

Define a minimal, stable observer event contract for v2, even if it is small.

### Questions

- [ ] Which observer events should be stable across minor releases?
- [ ] Is a versioned observer interface needed?

---

## 12. Closure Implementation Details

The `Cell` type is used for mutable closure captures:

```go
// object/cell.go
type Cell struct {
    *base
    value Object
}
```

### Concern

`Cell` is in the public `object` package alongside user-facing types like `String` and `List`. But it's an implementation detail of closures — users never create or interact with cells directly.

### Proposal

Move `Cell` to an internal package, or document that it's not part of the public API.

### Questions

- [ ] Is `Cell` ever used outside the compiler/VM?
- [ ] Should `object` be split into public and internal types?

---

## 13. Module System Limitations

Modules are entries in the global environment map:

```go
env["math"] = modMath.Module()
```

### Current Limitations

Scripts cannot:
- Import specific functions: `from math import sqrt`
- Alias modules: `import math as m`
- Have isolated module-local state

### Questions

- [ ] Is a module/import system in scope for v2?
- [ ] Should modules be first-class (importable from files)?
- [ ] Is the current design intentionally minimal for embedding use cases?

---

## 14. Compiler Documentation

The compiler uses a two-pass approach:
1. Collect function declarations (for forward references)
2. Compile the AST

This isn't documented in the code. Someone reading `compiler.go` must discover this through code reading.

### Proposal

Add a block comment at the top of `compiler/compiler.go` explaining:
- Why two passes are needed
- What happens in each pass
- How forward references work

---

## 15. Naming Consistency

### Current Inconsistencies

| Term | Meaning | Origin |
|------|---------|--------|
| `LoadFast` | Load local variable | Python |
| `LoadFree` | Load closure variable | Correct term |
| `BinarySubscr` | Index operation | Python (abbreviated) |
| `ContainsOp` | Membership test | Not abbreviated |

### Questions

- [ ] Should Python-derived names be replaced with Go-idiomatic terms?
- [ ] Is consistency worth the churn?
- [ ] Proposed alternatives: `LoadLocal`, `LoadCapture`, `Index`, `Contains`?

---

## 16. Global Name Binding Clarity

`Compile` uses `compiler.WithGlobalNames()` based on `WithEnv` keys. This means
the set and order of global names is fixed at compile time.

### Problem

Users can re-use bytecode with different env maps that have the same keys, but
not with differing keys. This is subtle and should be explicit.

### Proposal

Document that compiled code is bound to the specific global name set supplied
at compile time.

### Questions

- [ ] Should we offer a way to rebind globals safely at runtime?
- [ ] Is this a sharp edge that needs an API safeguard?

---

## 17. Result and Error Semantics

`Run` returns `nil` only for `NilType`, and uses `Inspect()` for objects with no
Go equivalent. This behavior is thoughtful but not obvious.

### Proposal

Document result conversion rules and error semantics in the public API docs.

### Questions

- [ ] Should there be an option to return `object.Object` directly?
- [ ] Is the current "string fallback" the right default for embedding?

---

## 18. Object Package Deep Dive

A detailed analysis of the `object` package structure, patterns, and concerns.

### 18.1 Package Structure

The package contains **48 files** (~11K lines) with these categories:

| Category | Files | Purpose |
|----------|-------|---------|
| Core types | `int.go`, `float.go`, `string.go`, `bool.go`, `byte.go`, `bytes.go` | Primitives |
| Containers | `list.go`, `map.go` | Collections |
| Functions | `closure.go`, `builtin.go`, `partial.go` | Callables |
| Modules | `module.go` | Module system |
| Errors | `error.go`, `errors.go`, `errz.go` | Error handling |
| Go interop | `proxy.go`, `go_type.go`, `go_field.go`, `go_method.go` | Reflection bridge |
| Type conversion | `typeconv.go` | Go ↔ Risor conversion |
| Helpers | `base.go`, `args.go`, `operations.go`, `sort.go`, `context_values.go` | Utilities |
| Special | `nil.go`, `cell.go`, `time.go`, `color.go`, `dynamic_attr.go`, `result.go` | Misc types |

### 18.2 The `base` Type Pattern

All types embed `*base` to provide default implementations:

```go
type base struct{}

func (b *base) GetAttr(name string) (Object, bool) { return nil, false }
func (b *base) SetAttr(name string, value Object) error {
    return TypeErrorf("type error: object has no attribute %q", name)
}
func (b *base) IsTruthy() bool { return true }
func (b *base) Cost() int { return 0 }
```

**Observations:**
- `IsTruthy()` defaults to `true` — types must opt-out of truthiness
- `Cost()` defaults to `0` — no resource tracking unless overridden
- `GetAttr` returns `(nil, false)` — distinct from "has attr but it's nil"
- `SetAttr` returns an error — immutable by default

**Issue:** The `base` type is a pointer (`*base`) but is never allocated — it's always `nil`. This works because Go allows method calls on nil pointers, but it's unusual. Consider using a value type or removing it entirely.

### 18.3 Type Conversion System

**Status: RESOLVED** — Replaced with `TypeRegistry` system. See §9 and `docs/proposals/type-registry.md`.

The old system had these issues (now fixed):

- **Global converter registry** with mutex — Replaced with immutable `TypeRegistry` owned by VM
- **20+ converter types** with duplicated logic — Replaced with unified numeric handling
- **TypeConverter interface** with scattered implementations — Replaced with `RegistryBuilder` for custom converters
- **No extension point for custom types** — Added `RisorValuer` interface for self-converting types

The `As*` helper functions (`AsInt()`, `AsString()`, etc.) remain for convenience but the underlying conversion system is now cleaner.

### 18.4 Operation Dispatch

Binary operations go through `RunOperation` on each type:

```go
// In int.go
func (i *Int) RunOperation(opType op.BinaryOpType, right Object) Object {
    switch right := right.(type) {
    case *Int:
        return i.runOperationInt(opType, right.value)
    case *Float:
        return i.runOperationFloat(opType, right.value)
    case *Byte:
        return i.runOperationInt(opType, int64(right.value))
    default:
        return TypeErrorf("type error: unsupported operation for int: %v on type %s", opType, right.Type())
    }
}
```

**Observations:**

- Each numeric type duplicates operation logic for each other numeric type
- `Int` + `Float` → `Float`, `Int` + `Int` → `Int` (type promotion rules)
- No central place defining type promotion — it's scattered across files
- `operations.go` handles `&&` and `||` specially (short-circuit semantics)

**Issue:** The comment in `operations.go` says "In Risor v2, RunOperation should return a separate error value" — this is a known design debt.

### 18.5 Container Methods via GetAttr

List and Map expose methods through `GetAttr`:

```go
// In list.go
func (ls *List) GetAttr(name string) (Object, bool) {
    switch name {
    case "append":
        return &Builtin{
            name: "list.append",
            fn: func(ctx context.Context, args ...Object) Object {
                if len(args) != 1 {
                    return NewArgsError("list.append", 1, len(args))
                }
                ls.Append(args[0])
                return ls
            },
        }, true
    // ... 15 more methods
    }
}
```

**Observations:**

- Methods are created on every `GetAttr` call — no caching
- Each method closure captures `ls` — proper but allocates
- Argument validation is duplicated in every method
- `map()`, `filter()`, `reduce()`, `each()` require `GetCallFunc(ctx)` — they invoke user functions

**Issue:** The higher-order methods (`map`, `filter`, etc.) have complex control flow. They handle both `*Builtin` and `*Closure` differently. For `*Closure`, they use `GetCallFunc(ctx)` to get the VM's call function. This coupling between `object` and `vm` is indirect but real.

### 18.6 Error Object Complexity

The `Error` type has rich metadata:

```go
type Error struct {
    *base
    err        error           // Underlying Go error
    raised     bool            // Thrown vs returned
    structured *StructuredError // Location + stack trace
}
```

Attributes are exposed as methods:

```go
func (e *Error) GetAttr(name string) (Object, bool) {
    switch name {
    case "message":
        return NewBuiltin("message", func(ctx context.Context, args ...Object) Object {
            return e.Message()
        }), true
    case "stack":
        return NewBuiltin("stack", func(ctx context.Context, args ...Object) Object {
            // ... builds list of frame maps
        }), true
    // ... more attributes
    }
}
```

**Observations:**

- Attributes are methods (callable), not values — `err.message()` not `err.message`
- This is inconsistent with other types where attributes are values
- The `raised` flag is mutable via `WithRaised()` — side effect on error objects
- `NewError()` always sets `raised = true` — the flag seems to only matter for returned errors

### 18.7 Go Interop (Proxy System)

**Status: REMOVED** — The Proxy system (`Proxy`, `GoType`, `GoField`, `GoMethod`) was removed from v2.

Go values are now converted to Risor types at the embedding boundary using `TypeRegistry`:
- Primitives, slices, and maps are converted to native Risor types
- Custom types use `RisorValuer` interface or registered converters
- No reflection-based method dispatch at runtime

This simplifies the type system and eliminates the `goTypeRegistry` global state.

### 18.8 Module State Management

Modules have two kinds of state:

```go
type Module struct {
    name         string
    code         *bytecode.Code
    builtins     map[string]Object   // Immutable after creation
    globals      []Object            // Mutable during execution
    globalsIndex map[string]int
    callable     BuiltinFunction     // Optional: makes module callable
}
```

**Observations:**

- `builtins` are functions, `globals` are module-level variables
- `Override()` allows modifying builtins after creation — security consideration
- `UseGlobals()` swaps the globals slice — used for module state isolation?
- Callable modules (like `http(url)`) are supported via `callable` field

### 18.9 Specific Concerns

**Int caching:**
```go
const tableSize = 256
var intCache = []*Int{}

func NewInt(value int64) *Int {
    if value >= 0 && value < tableSize {
        return intCache[value]
    }
    return &Int{value: value}
}
```

Small integers (0-255) are cached — good for performance. But this means `NewInt(1) == NewInt(1)` (same pointer), which could cause issues if anyone mutates the `Int` (they shouldn't, but it's not enforced).

**List circular reference detection:**
```go
type List struct {
    items         []Object
    inspectActive bool  // Tracks if we're mid-inspection
}

func (ls *List) Inspect() string {
    if ls.inspectActive {
        return "[...]"
    }
    ls.inspectActive = true
    defer func() { ls.inspectActive = false }()
    // ...
}
```

This prevents infinite recursion when a list contains itself. But `inspectActive` is mutable state on what should be a value — not thread-safe.

**Context-based call function:**
```go
func GetCallFunc(ctx context.Context) (CallFunc, bool) {
    v := ctx.Value(callFuncKey)
    if v == nil {
        return nil, false
    }
    return v.(CallFunc), true
}
```

The VM stores its call function in the context so that objects can invoke closures. This is a reasonable pattern but creates implicit coupling.

### 18.10 Callable Dispatch Bug (Builtin vs Closure)

`list.filter()` and `list.each()` accept `*Builtin` in their type checks, but then invoke:

```go
decision, err := callFunc(ctx, fn.(*Closure), filterArgs)
```

If a builtin is passed, this will panic on the type assertion. `list.map()` handles builtins separately, but `filter` and `each` do not.

**Impact:** A user passing a builtin to `filter` or `each` can trigger a runtime panic instead of a recoverable Risor error.

**Proposals:**
- Use `Callable` everywhere and call `Call(ctx, ...)` instead of special-casing.
- Or explicitly disallow builtins for these methods and return a type error.

### 18.11 Attribute Name Collisions in Map

Map attributes (`m.keys`, `m.values`, `m.items`) are implemented via `GetAttr`. Keys with the same names are hidden when accessed via dot syntax.

**Impact:** `m.keys` means "method" even if `m["keys"]` exists. The only way to access such keys is bracket syntax.

**Proposal:** Document this behavior explicitly or prioritize map keys over method names for attribute access.

### 18.12 Conversion Error Signaling

**Status: RESOLVED** — All `As*` helper functions now return `(T, error)` using Go's standard error type.

The old system had these issues (now fixed):

- **`As*` functions returned `*Error`** — Now return `error` for Go-idiomatic usage
- **`FromGoType` returned `*Error` as `Object`** — Remains for backward compatibility, but `TypeRegistry.FromGo` is preferred
- **Mixed error semantics** — Now consistent: `As*` helpers and `TypeRegistry` methods all return Go errors

### 18.13 Error Equality and Structured Data

`Error.Equals()` compares only message text and the `raised` flag. It ignores structured data like filename, line, or stack trace.

**Impact:** Two errors with different sources compare equal if their messages match.

**Proposal:** Define error equality semantics explicitly (message-only vs. structured data) and document the intent.

### 18.14 Public Panics in Constructors

`NewBuiltin` and `Module.UseGlobals` panic on invalid inputs. These are reachable from embedder code.

**Impact:** A host process can crash due to a misuse of public APIs.

**Proposal:** Return explicit errors instead of panicking in public constructors.

### 18.15 Summary of Object Package Issues

| Issue | Severity | Category |
|-------|----------|----------|
| Type coercion rules scattered across files | Medium | Clarity |
| Operation dispatch duplicates logic per type | Medium | Maintainability |
| Method attributes on Error (callable vs value) | Low | Consistency |
| Builtin passed to list.filter/each can panic | High | Correctness |
| Map attribute names shadow keys | Low | Clarity |
| ~~Conversion error signaling inconsistent~~ | ~~Medium~~ | ~~Consistency~~ | **RESOLVED** — As* helpers return error |
| Error equality ignores structured data | Low | Semantics |
| Public constructors panic on bad inputs | Medium | Safety |
| ~~Global registries (typeConverters, goTypeRegistry)~~ | ~~Medium~~ | ~~Concurrency~~ | **RESOLVED** — TypeRegistry |
| Int cache could cause issues if mutated | Low | Safety |
| List inspectActive not thread-safe | Low | Concurrency |
| base is always nil pointer | Low | Style |
| Container methods allocated on every GetAttr | Low | Performance |

### Questions

- [ ] Should type coercion rules be centralized in one place?
- [ ] Should containers cache their method builtins?
- [ ] Is the Error attribute-as-method pattern intentional?
- [ ] Should Int and other primitives be made immutable by hiding the value field?
- [ ] Should conversion errors be Go errors or Risor error objects?
- [ ] Should map attribute access prioritize keys or methods?

---

## Summary

### High Priority (Clarity/Correctness)

1. **Error handling unification** — Pick one system, document boundaries
2. **Remove global state** — `typeErrorsAreFatal` should be instance-scoped
3. **Compiler documentation** — Explain the two-pass strategy
4. **Language semantics spec** — Define core behavior contract
5. **Resource limits** — Add cancellation and runtime limits for embedding
6. **Callable dispatch correctness** — Fix builtin handling in list helpers

### Medium Priority (Design Quality)

7. **Object interface decomposition** — Reduce mandatory methods
8. **Builtin simplification** — Remove redundant fields
9. **Embedding boundary** — Document or tighten type conversion rules
10. **Conversion consistency** — Unify error signaling in type conversion
11. **Concurrency contract** — Clarify env mutability and thread safety
12. **Observer stability** — Define a minimal stable contract

### Low Priority (Polish)

13. **API ergonomics** — Consider `WithStdlib()` option
14. **HashKey efficiency** — Only if memory is a real problem
15. **Naming consistency** — Only if doing major refactoring anyway
16. **Result semantics** — Document conversion and return rules
17. **Global name binding** — Make compile-time binding explicit
18. **Container attribute semantics** — Document key/method collisions

### Out of Scope (Future)

19. **Module/import system** — Deferred unless needed for v2 goals

---

## Proposals for v2

Concrete proposals derived from the analysis above, organized by priority.

### P0: Correctness (Must Fix)

| ID | Problem | Proposal | Reason | Status |
|----|---------|----------|--------|--------|
| P0-1 | `list.filter()` and `list.each()` panic when passed a builtin instead of a closure (§18.10) | Define a `Callable` interface with `Call(ctx, args) (Object, error)`. Both `*Builtin` and `*Closure` implement it. Container methods use `Callable` uniformly. | Panics from user code are unacceptable. A type error should be returned instead. | Done |
| P0-2 | Builtins return `Object` which may secretly be `*Error`, forcing callers to check (§2) | Change signature to `func(ctx, args) (Object, error)`. Remove `isErrorHandler` flag from `Builtin` struct. | Go-idiomatic error handling eliminates an entire class of bugs and removes the need for the `raised` flag on returned errors. | Done |

### P1: Foundation (High Impact)

| ID | Problem | Proposal | Reason | Status |
|----|---------|----------|--------|--------|
| P1-1 | `typeErrorsAreFatal` is global mutable state affecting all VMs (§3) | Remove the configuration entirely. Type errors are always fatal (this was already the effective behavior since `IsFatal()` was never checked). Removed: global variable, `FatalError` interface, `IsFatal()` methods, setter/getter functions. | Simplest solution. The configuration was dead code — no runtime behavior depended on it. | Done |
| P1-2 | Global registries: `typeConverters`, `goTypeRegistry` (§18.3, §18.7) | Replaced `typeConverters` with `TypeRegistry` (immutable, VM-owned). Removed `goTypeRegistry` along with the proxy system. | Type conversion is now explicit via `WithTypeRegistry` option. No global mutable state remains. | Done |
| P1-3 | Object interface has 9 methods; `Cost()` mixes resource concerns into values (§1) | Keep 8-method core interface. Change `Equals` to return `bool`, `RunOperation` to return `(Object, error)`. Remove `Cost()`. Add single `Callable` interface for invocable types. | Minimal change. Only one capability interface to remember. Cost tracking moves to Runtime. | Done |
| P1-4 | Two parallel error systems with unclear boundaries (§2) | Compile-time uses `errors` package (Go errors). Runtime uses `object.Error` (Risor errors). Document the boundary: compiler returns Go errors, VM returns Risor errors. Errors are values; `throw` is the action that triggers exception handling (see A-8). | Single mental model per phase. Clear ownership of error handling. | Partial (see A-8) |
| P1-5 | No resource limits for embedded execution (§8) | Add `Compile(ctx, source, opts)` for cancellation. Add VM options: `WithMaxSteps(int)`, `WithMaxStackDepth(int)`, `WithTimeout(duration)`. | Embedders need predictable termination. Untrusted code must not run forever. | Not Started |

### P2: Consistency (Design Quality)

| ID | Problem | Proposal | Reason | Status |
|----|---------|----------|--------|--------|
| P2-1 | Type coercion rules scattered across `As*` functions and `TypeConverter` implementations (§18.3) | Create `coercion/rules.go` with a single `NumericPromotion` table and `PromoteNumeric(left, right Object)` function. All coercion logic references this file. | One source of truth. Coercion behavior becomes auditable and testable in isolation. | Not Started |
| P2-2 | Conversion functions have inconsistent error signaling: some return `*Error` as `Object`, some return `(Object, error)` (§18.12) | All `As*` helper functions (`AsInt`, `AsString`, `AsBool`, etc.) now return `(T, error)` using Go's standard error type. `TypeRegistry` methods already returned `(Object, error)`. `FromGoType` remains for backward compatibility but is deprecated. | Consistent with P0-2. Embedders get predictable error handling. | Done |
| P2-3 | `Builtin` struct has redundant `module` and `moduleName` fields (§4) | Keep only `moduleName string`. Derive module reference when needed via lookup. | Simpler struct. Single source of truth for module association. | Won't Do |
| P2-4 | Public constructors (`NewBuiltin`, `Module.UseGlobals`) panic on invalid inputs (§18.14) | Use builder pattern for `Builtin`: `NewBuiltin(name, fn).InModule(name)`. Validate at build time, not construction. Remove panic paths. | Host processes should not crash due to API misuse. Builder pattern is ergonomic and avoids error handling ceremony. | Done |
| P2-5 | Map attribute names (`keys`, `values`, `items`) shadow map keys with the same names (§18.11) | Document behavior explicitly. Add `__method__(name)` for unambiguous method access, or reverse priority for maps (keys shadow methods). | Users need a way to access shadowed keys. Behavior should be predictable. | Not Started |

### P3: Clarity (Documentation & Polish)

| ID | Problem | Proposal | Reason | Status |
|----|---------|----------|--------|--------|
| P3-1 | Compiler two-pass strategy is undocumented (§14) | Add a block comment at the top of `compiler/compiler.go` explaining: (1) why two passes, (2) what each pass does, (3) how forward references work. | New contributors can understand the design without reverse-engineering. | Done |
| P3-2 | No language semantics specification (§7) | Create `docs/semantics.md` covering: numeric types and coercions, equality/ordering rules, truthiness, iteration order, error propagation. Version it with v2. | Embedders need a stable contract. Behavior should be specified, not inferred. | Not Started |
| P3-3 | Concurrency contract is unclear: are env values copied or shared? (§10) | Document in API docs: (1) env map is shallow-copied, values are shared, (2) builtins must be thread-safe, (3) mutable objects in env are caller's responsibility. | Safe embedding requires clear ownership rules. | Done |
| P3-4 | Global name binding at compile time is subtle (§16) | Document that compiled bytecode is bound to specific global names. Provide `Bytecode.GlobalNames()` method for introspection. | Users need to understand why reusing bytecode with different env keys fails. | Done |
| P3-5 | Result conversion rules are implicit (§17) | Document in API docs: `nil` for `NilType`, `Inspect()` string for types without Go equivalent, native Go types otherwise. Add `risor.WithRawResult()` option to return `object.Object` directly. | Embedders can choose the conversion behavior that fits their use case. | Done |

### P4: Future Consideration (Deferred)

| ID | Problem | Proposal | Reason | Status |
|----|---------|----------|--------|--------|
| P4-1 | HashKey struct wastes memory with unused fields (§5) | Consider interface-based HashKey only if profiling shows memory pressure in map-heavy workloads. | Optimization without measurement is premature. Current design is simple and correct. | Deferred |
| P4-2 | Python-derived opcode names are inconsistent (§15) | Rename only during major refactoring: `LoadFast`→`LoadLocal`, `LoadFree`→`LoadCapture`, `BinarySubscr`→`Index`. | Churn without functional benefit. Address opportunistically. | Deferred |
| P4-3 | No module/import system for scripts (§13) | Defer to post-v2 unless embedding use cases require it. Current design is intentionally minimal. | Scope control. Module systems are complex and may not fit embedding-first philosophy. | Deferred |
| P4-4 | `Cell` type is public but is an implementation detail (§12) | Move to `internal/vm/` or add `// Internal: do not use` documentation. | API surface should reflect user-facing types only. | Done |

### Additional Proposals (New)

| ID | Problem | Proposal | Reason | Status |
|----|---------|----------|--------|--------|
| A-1 | Primitive types (`Int`, `Float`, etc.) have mutable value fields; combined with caching, this is a latent bug (§18.9) | Make primitives immutable: unexport value fields, remove setters. `Int.value` becomes read-only via `Value() int64`. | Immutability enables safe caching, sharing across goroutines, and simpler reasoning. Cached `NewInt(1)` can never be corrupted. | Not Started |
| A-2 | `Error.Equals()` ignores structured data (filename, line, stack) — two errors from different sources compare equal if messages match (§18.13) | Define error equality as message-only (current behavior) but document explicitly. Add `Error.Same(other *Error) bool` for identity comparison including location. | Users need both: value equality for error handling, identity equality for debugging/logging. | Not Started |
| A-3 | `base` type is always a nil pointer (`*base`), relying on Go's nil receiver behavior (§18.2) | Remove `*base` embedding entirely. Use standalone default functions: `DefaultIsTruthy() bool`, `DefaultCost() int`. Types call these explicitly. | Nil pointer receivers are unusual and confusing. Explicit defaults are clearer. | Not Started |
| A-4 | Observer API has no stability promise; tooling (profilers, debuggers) will break on changes (§11) | Define a minimal stable observer contract for v2: `OnCall`, `OnReturn`, `OnError`, `OnLine`. Version the observer interface. Unstable events go in a separate `ExperimentalObserver` interface. | Tooling ecosystem needs a stable foundation. Versioning prevents silent breakage. | Not Started |
| A-5 | Error introspection is not first-class; embedders must know internal types to extract source locations | Add `risor.ErrorLocation(err error) (file string, line int, ok bool)` and `risor.ErrorStack(err error) []Frame` to public API. | Embedders should be able to provide good error messages without knowing Risor internals. | Not Started |
| A-6 | `List.inspectActive` flag for circular reference detection is not thread-safe (§18.9) | Pass a `seen map[*List]bool` through `Inspect()` calls instead of storing state on the object. Or accept that `Inspect()` is single-threaded and document it. | Mutable state on values causes races. Either fix it or document the constraint. | Not Started |
| A-7 | Bytecode reuse contract is implicit: same keys required, values can differ (§16) | Add `Bytecode.RequiredGlobals() []string` method. `Run()` validates env keys match at startup (not silently fail mid-execution). | Fail-fast with clear error beats mysterious "undefined" errors during execution. | Done |
| A-8 | The `raised` flag on Error was vestigial from pre-try/catch era when errors could be values OR exceptions | Remove `raised` flag entirely. Errors are values (like Python exceptions). Only `throw` triggers exception handling. Errors stringify in templates. Restored `error(msg, ...args)` builtin for creating error values. | Cleaner model: errors are data, throw is an action. No mutable state on error objects. | Done |

### Key Design Decisions

These questions must be resolved before implementation:

| Decision | Options | Recommendation | Tradeoff |
|----------|---------|----------------|----------|
| **Error handling model** | (A) `(Object, error)` returns, (B) errors-as-values, (C) keep both | **A+B: Go-idiomatic returns + errors as script values** | Builtins return `(Object, error)`. In scripts, errors are values; `throw` triggers exceptions. (Done: P0-2, A-8) |
| **Object interface size** | (A) Keep 9 methods, (B) 8-method core + Callable | **B: Simplified** | Remove Cost(), fix return types, add Callable interface. (Done: P1-3) |
| **Global state elimination** | (A) Runtime struct in context, (B) Thread-local, (C) Keep globals | **A: VM-owned state** | TypeRegistry owned by VM, no global mutable state. (Done: P1-1, P1-2) |
| **Value immutability** | (A) Immutable primitives, (B) Mutable (status quo) | **A: Immutable** | Can't modify in place, but safer |
| **Coercion centralization** | (A) Single rules table, (B) Keep scattered | **A: Centralize** | More indirection, but auditable |
| **Map key/method collision** | (A) Methods win, (B) Keys win, (C) Document only | **C: Document** | Not ideal, but low churn |

### Implementation Order

Recommended sequence for v2 development:

```
Phase 1: Correctness
  P0-1 → P0-2 (error handling change cascades through codebase)

Phase 2: Foundation
  P1-1 + P1-2 (Runtime struct)
  P1-3 (Object interface decomposition)
  P1-4 (error system documentation)
  P1-5 (resource limits)

Phase 3: Consistency
  P2-1 → P2-2 (coercion and conversion together)
  P2-3 through P2-6 (independent, can parallelize)

Phase 4: Documentation
  P3-1 through P3-5 (can happen anytime, no code dependencies)
```

### Design Principles for v2

These proposals follow a consistent philosophy:

1. **One way to do things** — No parallel error systems, no parallel conversion patterns
2. **Explicit over implicit** — No global state, no magic flags, clear ownership
3. **Small core, rich extensions** — Minimal Object interface, capabilities via opt-in interfaces
4. **Go-idiomatic** — `(T, error)` returns, context for cancellation, options pattern
5. **Safe by default** — Resource limits, no panics from public API, immutable where possible
6. **Document the contract** — Behavior is specified, not inferred from implementation

---

## Appendix: Code Sketches

Concrete examples of proposed changes.

### A. Simplified Object Interface (P1-3)

```go
// Core interface - every value (8 methods, down from 9)
type Object interface {
    Type() Type
    Inspect() string
    Interface() interface{}
    IsTruthy() bool
    Equals(other Object) bool                                  // Changed: returns bool
    GetAttr(name string) (Object, bool)
    SetAttr(name string, value Object) error
    RunOperation(opType op.BinaryOpType, right Object) (Object, error)  // Changed: returns error
}

// Callable - the only capability interface
type Callable interface {
    Call(ctx context.Context, args ...Object) (Object, error)
}
```

**What implements Callable:**

- `*Builtin` — built-in functions
- `*Closure` — user-defined functions
- `*Module` — callable modules (optional, e.g. `http(url)`)
- `*Proxy` — if the wrapped Go type is callable

**Cost tracking:** Moved to Runtime. The VM calls `runtime.Cost(obj)` which uses a type switch internally. This keeps resource concerns out of the value abstraction.

### B. Runtime Struct (P1-1, P1-2)

```go
// Runtime holds all configuration that was previously global state.
// Pass it via context or store in VM.
type Runtime struct {
    // Error handling
    FatalTypeErrors bool

    // Resource limits
    MaxSteps      int
    MaxStackDepth int
    Timeout       time.Duration

    // Registries (previously global)
    typeConverters sync.Map // map[reflect.Type]TypeConverter
    goTypeRegistry sync.Map // map[reflect.Type]*GoType
    intCache       [256]*Int
}

func NewRuntime(opts ...RuntimeOption) *Runtime {
    rt := &Runtime{
        FatalTypeErrors: false,
        MaxSteps:        0, // unlimited
        MaxStackDepth:   1000,
    }
    rt.initIntCache()
    for _, opt := range opts {
        opt(rt)
    }
    return rt
}

// Context key for runtime access
type runtimeKey struct{}

func WithRuntime(ctx context.Context, rt *Runtime) context.Context {
    return context.WithValue(ctx, runtimeKey{}, rt)
}

func GetRuntime(ctx context.Context) *Runtime {
    if rt, ok := ctx.Value(runtimeKey{}).(*Runtime); ok {
        return rt
    }
    return DefaultRuntime
}
```

### C. Builtin Signature Change (P0-2)

```go
// Before: error encoded in return value
type BuiltinFunction func(ctx context.Context, args ...Object) Object

// After: explicit error return
type BuiltinFunction func(ctx context.Context, args ...Object) (Object, error)

// Example builtin: len()
func builtinLen(ctx context.Context, args ...Object) (Object, error) {
    if len(args) != 1 {
        return nil, ArgsError("len", 1, len(args))
    }
    switch obj := args[0].(type) {
    case *String:
        return NewInt(int64(len(obj.Value()))), nil
    case *List:
        return NewInt(int64(obj.Len())), nil
    case *Map:
        return NewInt(int64(obj.Len())), nil
    default:
        return nil, TypeError("len() argument must be a sequence")
    }
}
```

### D. Callable Interface Usage (P0-1)

```go
// In list.go - fixed filter() method
func (ls *List) filter(ctx context.Context, fn Object) (Object, error) {
    callable, ok := fn.(Callable)
    if !ok {
        return nil, TypeError("filter() argument must be callable")
    }

    result := NewList(nil)
    for _, item := range ls.items {
        decision, err := callable.Call(ctx, item)
        if err != nil {
            return nil, err
        }
        if decision.IsTruthy() {  // IsTruthy is on the core Object interface
            result.Append(item)
        }
    }
    return result, nil
}
```

### E. Coercion Rules Table (P2-1)

```go
// coercion/rules.go

type NumericKind int

const (
    KindByte NumericKind = iota
    KindInt
    KindFloat
)

// Promotion rules: result type when combining two numeric types
var promotionTable = [3][3]NumericKind{
    //           Byte    Int     Float
    /* Byte */  {KindByte, KindInt, KindFloat},
    /* Int */   {KindInt, KindInt, KindFloat},
    /* Float */ {KindFloat, KindFloat, KindFloat},
}

func PromoteNumeric(left, right Object) (Object, Object, NumericKind, error) {
    lk, lok := numericKind(left)
    rk, rok := numericKind(right)
    if !lok || !rok {
        return nil, nil, 0, TypeError("operands must be numeric")
    }

    resultKind := promotionTable[lk][rk]

    // Convert both operands to result type
    l, err := convertTo(left, resultKind)
    if err != nil {
        return nil, nil, 0, err
    }
    r, err := convertTo(right, resultKind)
    if err != nil {
        return nil, nil, 0, err
    }

    return l, r, resultKind, nil
}
```

### F. Resource Limits (P1-5)

```go
// VM options
func WithMaxSteps(n int) Option {
    return func(cfg *Config) { cfg.MaxSteps = n }
}

func WithMaxStackDepth(n int) Option {
    return func(cfg *Config) { cfg.MaxStackDepth = n }
}

func WithTimeout(d time.Duration) Option {
    return func(cfg *Config) { cfg.Timeout = d }
}

// In VM execution loop
func (vm *VM) Run(ctx context.Context) (Object, error) {
    if vm.cfg.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, vm.cfg.Timeout)
        defer cancel()
    }

    for {
        // Check context cancellation
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }

        // Check step limit
        vm.steps++
        if vm.cfg.MaxSteps > 0 && vm.steps > vm.cfg.MaxSteps {
            return nil, ErrStepLimitExceeded
        }

        // Check stack depth
        if len(vm.stack) > vm.cfg.MaxStackDepth {
            return nil, ErrStackOverflow
        }

        // Execute instruction...
    }
}
```

### G. Error Model: Errors as Values (A-8)

**Implemented.** This section documents the error handling model after removing the vestigial `raised` flag.

#### The Model

Errors in Risor v2 follow the Python exception model:

1. **Errors are values** — You can create, inspect, store, and pass them around like any other value
2. **`throw` is an action** — Only `throw` triggers exception handling
3. **Caught errors become values again** — In a catch block, the error is just a value

```
┌─────────────────────────────────────────────────────────────┐
│  Error Object = Data (like any other value)                 │
│  ├── Can be created: error("msg", ...args)                  │
│  ├── Can be caught: try { ... } catch e { ... }             │
│  ├── Can be inspected: err.message(), err.stack()           │
│  ├── Can be compared: err1 == err2                          │
│  ├── Can be stringified: `${err}` or string(err)            │
│  └── Can be stored: let errors = [err1, err2]               │
│                                                             │
│  Throw = Action (triggers exception handling)               │
│  ├── Explicit: throw err                                    │
│  ├── Implicit: operation failure (1 + "foo")                │
│  └── Implicit: builtin returns error                        │
└─────────────────────────────────────────────────────────────┘
```

#### What Was Removed

The `raised` flag was a vestige of a pre-try/catch design where errors could be either:
- "Raised" (propagating as exceptions)
- "Not raised" (usable as values)

With try/catch, this distinction is unnecessary:
- All errors created by the VM have `raised=true` by default
- There was no way to create a non-raised error
- The flag was only checked in one place (BuildString opcode)

**Removed:**
- `Error.raised` field
- `Error.IsRaised()` method
- `Error.WithRaised()` method
- Raised flag in equality/comparison

#### Usage Examples

```risor
// Create an error value directly
let err = error("file %s not found", filename)

// Inspect the error
print(err.message())  // "file foo.txt not found"
print(err.stack())    // [] (no stack until thrown)

// Stringify in templates
print(`Error occurred: ${err}`)  // "Error occurred: file foo.txt not found"

// Throw when needed
throw err

// Catch an error to get it as a value
try {
    might_fail()
} catch e {
    print(e.message())  // e is a value now
}

// Store errors
let problems = []
try { op1() } catch e { problems.append(e) }
try { op2() } catch e { problems.append(e) }

// Re-throw if needed
try {
    risky()
} catch e {
    if !e.message().contains("retryable") {
        throw e  // Re-raise
    }
    // Otherwise swallow
}

// Operations that fail throw automatically
try {
    let x = 1 + "foo"  // throws type error
} catch e {
    print(e.message())  // "type error: ..."
}
```

#### For Embedders

```go
result, err := risor.Eval(ctx, source, opts...)
if err != nil {
    // Unhandled exception propagated up
    switch e := err.(type) {
    case *errors.CompileError:
        // Compilation failed
    case *object.StructuredError:
        // Runtime error with location info
    default:
        // Other error
    }
}
// result might be an *object.Error if the script returned one as a value
```