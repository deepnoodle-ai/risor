# Risor Design Review

A first-principles review of Risor's architecture for v2.

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

## Completed Work

The following major changes have been implemented:

| Area | What Changed |
|------|--------------|
| **Object Interface** | 8-method core + `Callable` interface. `Equals` returns `bool`, `RunOperation` returns `(Object, error)`. `Cost()` removed. |
| **Error Handling** | Builtins return `(Object, error)`. Errors are values; `throw` triggers exceptions. `raised` flag removed. |
| **Global State** | Removed `typeErrorsAreFatal`, `FatalError`, `IsFatal()`. Removed global registries. |
| **Type Conversion** | `TypeRegistry` system (immutable, VM-owned). `RisorValuer` interface for custom types. |
| **Map Semantics** | Dot syntax accesses methods first, then keys (Python-style shadowing). Use `m["key"]` for data that shadows a method name. Maps have 10 methods: `keys()`, `values()`, `entries()`, `each()`, `get()`, `pop()`, `setdefault()`, `update()`, `clear()`, `copy()`. Iterator-returning methods are lazy. |
| **Callable Dispatch** | List methods use `Callable` interface uniformly. No more panics. |
| **Documentation** | Compiler two-pass strategy documented. Concurrency contract documented. Global name binding documented. Result conversion rules documented. |
| **Proxy System** | Removed entirely. Go values convert at embedding boundary via `TypeRegistry`. |
| **Base Type** | Removed `*base` embedding. Types use standalone default functions. |
| **Immutable Primitives** | All primitive types (`Int`, `Float`, `String`, `Bool`, `Byte`) have unexported value fields. |
| **Thread Safety** | Risor objects are not thread-safe. Don't share them across goroutines. (Same constraint as Python/JS.) |
| **Language Semantics** | `docs/semantics.md` specifies numeric coercion, equality, ordering, truthiness, iteration order, and error propagation. |

---

## Remaining Work

### P1-5: Resource Limits (High Priority)

**Problem:** No resource limits for embedded execution. `Compile` uses `context.Background()` internally.

**Proposal:**
- Add `Compile(ctx, source, opts)` for cancellation
- Add VM options: `WithMaxSteps(int)`, `WithMaxStackDepth(int)`, `WithTimeout(duration)`

**Reason:** Embedders need predictable termination. Untrusted code must not run forever.

```go
// VM options
func WithMaxSteps(n int) Option
func WithMaxStackDepth(n int) Option
func WithTimeout(d time.Duration) Option

// In VM execution loop
func (vm *VM) Run(ctx context.Context) (Object, error) {
    if vm.cfg.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, vm.cfg.Timeout)
        defer cancel()
    }

    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }

        vm.steps++
        if vm.cfg.MaxSteps > 0 && vm.steps > vm.cfg.MaxSteps {
            return nil, ErrStepLimitExceeded
        }

        if len(vm.stack) > vm.cfg.MaxStackDepth {
            return nil, ErrStackOverflow
        }
        // Execute instruction...
    }
}
```

---

### A-2: Error Equality Semantics

**Problem:** `Error.Equals()` ignores structured data (filename, line, stack). Two errors from different sources compare equal if messages match.

**Proposal:** Define error equality as message-only (current behavior) but document explicitly. Add `Error.Same(other *Error) bool` for identity comparison including location.

**Reason:** Users need both: value equality for error handling, identity equality for debugging/logging.

---

## Deferred (Post-v2)

| ID | Problem | Reason for Deferral |
|----|---------|---------------------|
| P4-2 | Python-derived opcode names inconsistent | Churn without functional benefit |
| P4-3 | No module/import system | Scope control; may not fit embedding-first philosophy |

---

## Open Questions

- [ ] Which resource limits are essential for v2? (P1-5)
- [ ] Should limits be enforced in the VM, compiler, or both? (P1-5)
- [ ] Should containers cache their method builtins? (performance)
- [ ] Is the Error attribute-as-method pattern intentional? (consistency)

---

## Reference: Current Object Interface

```go
// Core interface - every value (8 methods)
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

**What implements Callable:**
- `*Builtin` — built-in functions
- `*Closure` — user-defined functions
- `*Module` — callable modules (optional, e.g. `http(url)`)

---

## Reference: Error Model

Errors in Risor v2 follow the Python exception model:

1. **Errors are values** — You can create, inspect, store, and pass them around
2. **`throw` is an action** — Only `throw` triggers exception handling
3. **Caught errors become values again** — In a catch block, the error is just a value

```risor
// Create an error value
let err = error("file %s not found", filename)

// Throw when needed
throw err

// Catch to get it as a value
try {
    might_fail()
} catch e {
    print(e.message())
}

// Operations that fail throw automatically
try {
    let x = 1 + "foo"  // throws type error
} catch e {
    print(e.message())
}
```

For embedders:
```go
result, err := risor.Eval(ctx, source, opts...)
if err != nil {
    switch e := err.(type) {
    case *errors.CompileError:
        // Compilation failed
    case *object.StructuredError:
        // Runtime error with location info
    default:
        // Other error
    }
}
```
