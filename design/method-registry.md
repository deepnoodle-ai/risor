# Method Registry Design

**Status:** Proposal
**Author:** Claude
**Date:** 2026-01-27

## Summary

Replace the current dual-maintenance pattern (AttrSpec slice + GetAttr switch) with a unified `MethodRegistry` that combines method specification and implementation in a single definition, using a fluent builder API for ergonomic method registration.

## Motivation

The current approach to defining object methods has several pain points:

1. **Dual Maintenance** - Adding a method requires updating two separate places: the `*Attrs` slice and the `GetAttr` switch statement. These can drift out of sync.

2. **No Compile-Time Validation** - If a spec exists without an implementation (or vice versa), there's no error until runtime.

3. **Repetitive Boilerplate** - Every method case in `GetAttr` repeats the same pattern: create `&Builtin{}`, validate arg count, call underlying method.

4. **Duplicated Knowledge** - The arg count is specified in `AttrSpec.Args` but manually validated again in each `GetAttr` case.

5. **Manual Argument Handling** - Each implementation must index into `args[]` and type-assert, which is error-prone and repetitive.

### Current Pattern

```go
// string.go - Two places to maintain

var stringAttrs = []AttrSpec{
    {Name: "contains", Doc: "Check if substring exists", Args: []string{"substr"}, Returns: "bool"},
    {Name: "split", Doc: "Split by separator", Args: []string{"sep"}, Returns: "list"},
    // ... 17 more entries
}

func (s *String) GetAttr(name string) (Object, bool) {
    switch name {
    case "contains":
        return &Builtin{
            name: "string.contains",
            fn: func(ctx context.Context, args ...Object) (Object, error) {
                if len(args) != 1 {
                    return nil, fmt.Errorf("string.contains: expected 1 argument, got %d", len(args))
                }
                return s.Contains(args[0]), nil
            },
        }, true
    case "split":
        // ... same boilerplate pattern
    // ... 17 more cases
    }
    return nil, false
}
```

## Design

### Core Types

```go
// MethodDef combines a method's specification with its implementation.
type MethodDef[T any] struct {
    Spec AttrSpec
    Impl func(self T, ctx context.Context, args ...Object) (Object, error)
}

// MethodRegistry holds all methods for a given object type.
type MethodRegistry[T any] struct {
    typeName string
    methods  map[string]MethodDef[T]
    specs    []AttrSpec // cached, ordered list for Attrs()
}

// MethodBuilder provides a fluent API for defining a single method.
type MethodBuilder[T any] struct {
    registry *MethodRegistry[T]
    name     string
    doc      string
    args     []string
    returns  string
}
```

### Fluent Builder API

```go
// NewMethodRegistry creates a registry for the given type name.
func NewMethodRegistry[T any](typeName string) *MethodRegistry[T]

// Define starts building a new method definition.
// Returns a MethodBuilder for fluent configuration.
func (r *MethodRegistry[T]) Define(name string) *MethodBuilder[T]

// Specs returns a copy of all registered method specifications in registration order.
func (r *MethodRegistry[T]) Specs() []AttrSpec

// GetAttr returns a Builtin for the named method bound to self.
// Returns nil, false if the method doesn't exist.
func (r *MethodRegistry[T]) GetAttr(self T, name string) (Object, bool)

// MethodBuilder methods (each returns the builder for chaining):

// Doc sets the method's documentation string.
func (b *MethodBuilder[T]) Doc(doc string) *MethodBuilder[T]

// Arg adds a required argument by name.
func (b *MethodBuilder[T]) Arg(name string) *MethodBuilder[T]

// Args adds multiple required arguments.
func (b *MethodBuilder[T]) Args(names ...string) *MethodBuilder[T]

// Returns sets the return type (for documentation/tooling).
func (b *MethodBuilder[T]) Returns(typ string) *MethodBuilder[T]

// Impl sets the implementation and registers the method.
// Panics if a method with the same name is already registered.
func (b *MethodBuilder[T]) Impl(fn func(T, context.Context, ...Object) (Object, error))
```

### Typed Argument Helper

To reduce error-prone `args[i].(Type)` patterns, provide a generic helper:

```go
// Arg extracts and type-asserts an argument from the args slice.
// Returns a descriptive error if the index is out of bounds or the type doesn't match.
func Arg[T Object](args []Object, index int, methodName string) (T, error) {
    var zero T
    if index >= len(args) {
        return zero, fmt.Errorf("%s: missing argument at index %d", methodName, index)
    }
    v, ok := args[index].(T)
    if !ok {
        return zero, fmt.Errorf("%s: argument %d: expected %T, got %T",
            methodName, index, zero, args[index])
    }
    return v, nil
}
```

### Implementation

```go
package object

import (
    "context"
    "fmt"
    "slices"
)

type MethodDef[T any] struct {
    Spec AttrSpec
    Impl func(self T, ctx context.Context, args ...Object) (Object, error)
}

type MethodRegistry[T any] struct {
    typeName string
    methods  map[string]MethodDef[T]
    specs    []AttrSpec
}

type MethodBuilder[T any] struct {
    registry *MethodRegistry[T]
    name     string
    doc      string
    args     []string
    returns  string
}

func NewMethodRegistry[T any](typeName string) *MethodRegistry[T] {
    return &MethodRegistry[T]{
        typeName: typeName,
        methods:  make(map[string]MethodDef[T]),
    }
}

func (r *MethodRegistry[T]) Define(name string) *MethodBuilder[T] {
    return &MethodBuilder[T]{
        registry: r,
        name:     name,
    }
}

func (r *MethodRegistry[T]) Specs() []AttrSpec {
    return slices.Clone(r.specs)
}

func (r *MethodRegistry[T]) GetAttr(self T, name string) (Object, bool) {
    m, ok := r.methods[name]
    if !ok {
        return nil, false
    }
    expectedArgs := len(m.Spec.Args)
    fullName := r.typeName + "." + name
    return &Builtin{
        name: fullName,
        fn: func(ctx context.Context, args ...Object) (Object, error) {
            if len(args) != expectedArgs {
                return nil, argsError(fullName, expectedArgs, len(args))
            }
            return m.Impl(self, ctx, args...)
        },
    }, true
}

func (b *MethodBuilder[T]) Doc(doc string) *MethodBuilder[T] {
    b.doc = doc
    return b
}

func (b *MethodBuilder[T]) Arg(name string) *MethodBuilder[T] {
    b.args = append(b.args, name)
    return b
}

func (b *MethodBuilder[T]) Args(names ...string) *MethodBuilder[T] {
    b.args = append(b.args, names...)
    return b
}

func (b *MethodBuilder[T]) Returns(typ string) *MethodBuilder[T] {
    b.returns = typ
    return b
}

func (b *MethodBuilder[T]) Impl(fn func(T, context.Context, ...Object) (Object, error)) {
    r := b.registry
    if _, exists := r.methods[b.name]; exists {
        panic(fmt.Sprintf("%s: method %q already registered", r.typeName, b.name))
    }
    spec := AttrSpec{
        Name:    b.name,
        Doc:     b.doc,
        Args:    b.args,
        Returns: b.returns,
    }
    r.methods[b.name] = MethodDef[T]{Spec: spec, Impl: fn}
    r.specs = append(r.specs, spec)
}

// argsError returns a grammatically correct argument count error.
func argsError(methodName string, expected, got int) error {
    if expected == 1 {
        return fmt.Errorf("%s: expected 1 argument, got %d", methodName, got)
    }
    return fmt.Errorf("%s: expected %d arguments, got %d", methodName, expected, got)
}

// Arg extracts and type-asserts an argument from the args slice.
func Arg[T Object](args []Object, index int, methodName string) (T, error) {
    var zero T
    if index >= len(args) {
        return zero, fmt.Errorf("%s: missing argument at index %d", methodName, index)
    }
    v, ok := args[index].(T)
    if !ok {
        return zero, fmt.Errorf("%s: argument %d: expected %T, got %T",
            methodName, index, zero, args[index])
    }
    return v, nil
}
```

### Usage Example

After migration, `string.go` becomes:

```go
package object

import "context"

var stringMethods = NewMethodRegistry[*String]("string")

func init() {
    stringMethods.Define("contains").
        Doc("Check if substring exists").
        Arg("substr").
        Returns("bool").
        Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
            return s.Contains(args[0]), nil
        })

    stringMethods.Define("split").
        Doc("Split by separator").
        Arg("sep").
        Returns("list").
        Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
            return s.Split(args[0])
        })

    stringMethods.Define("replace").
        Doc("Replace occurrences of old with new").
        Args("old", "new").
        Returns("string").
        Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
            old, err := Arg[*String](args, 0, "string.replace")
            if err != nil {
                return nil, err
            }
            new, err := Arg[*String](args, 1, "string.replace")
            if err != nil {
                return nil, err
            }
            return s.Replace(old, new), nil
        })

    stringMethods.Define("to_lower").
        Doc("Convert to lowercase").
        Returns("string").
        Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
            return s.ToLower(), nil
        })

    // ... remaining methods
}

func (s *String) Attrs() []AttrSpec {
    return stringMethods.Specs()
}

func (s *String) GetAttr(name string) (Object, bool) {
    return stringMethods.GetAttr(s, name)
}
```

### Comparison: Before and After

**Before (current pattern):**
```go
// Spec definition (location 1)
{Name: "contains", Doc: "Check if substring exists", Args: []string{"substr"}, Returns: "bool"},

// Implementation (location 2)
case "contains":
    return &Builtin{
        name: "string.contains",
        fn: func(ctx context.Context, args ...Object) (Object, error) {
            if len(args) != 1 {
                return nil, fmt.Errorf("string.contains: expected 1 argument, got %d", len(args))
            }
            return s.Contains(args[0]), nil
        },
    }, true
```

**After (fluent builder):**
```go
stringMethods.Define("contains").
    Doc("Check if substring exists").
    Arg("substr").
    Returns("bool").
    Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
        return s.Contains(args[0]), nil
    })
```

## Benefits

| Aspect | Before | After |
|--------|--------|-------|
| Definitions per method | 2 (spec + case) | 1 (Define chain) |
| Lines per method | ~12 | ~6 |
| Sync errors possible | Yes | No |
| Arg count validation | Manual | Automatic |
| Arg type extraction | Manual cast | `Arg[T]()` helper |
| Type-safe receiver | No | Yes |
| Duplicate detection | None | Panic at init |
| Spec immutability | Mutable slice | Clone on read |

## Extensions

### Variadic Methods

Add a `Variadic()` method to the builder:

```go
func (b *MethodBuilder[T]) Variadic() *MethodBuilder[T] {
    b.variadic = true
    return b
}
```

Update `AttrSpec` and validation:

```go
type AttrSpec struct {
    Name     string
    Doc      string
    Args     []string
    Variadic bool   // true if last arg can repeat
    Returns  string
}

// In GetAttr:
if m.Spec.Variadic {
    minArgs := len(m.Spec.Args) - 1
    if len(args) < minArgs {
        return nil, fmt.Errorf("%s: expected at least %d arguments, got %d",
            fullName, minArgs, len(args))
    }
} else {
    if len(args) != expectedArgs {
        return nil, argsError(fullName, expectedArgs, len(args))
    }
}
```

Usage:

```go
stringMethods.Define("join").
    Doc("Join list elements with separator").
    Arg("items").
    Variadic().
    Returns("string").
    Impl(...)
```

### Optional Arguments with Defaults

For methods with optional arguments, add support via the builder:

```go
func (b *MethodBuilder[T]) OptionalArg(name string, defaultValue Object) *MethodBuilder[T] {
    b.optionalArgs = append(b.optionalArgs, optArg{name, defaultValue})
    return b
}
```

The registry would normalize args before calling the impl, filling in defaults for missing optional args.

### Return Type Enum

Replace the informal `Returns string` with a typed enum for tooling support:

```go
type ReturnType int

const (
    ReturnAny ReturnType = iota
    ReturnString
    ReturnInt
    ReturnBool
    ReturnList
    ReturnMap
    ReturnNil
    ReturnBytes
    ReturnTime
)
```

Update the builder:

```go
func (b *MethodBuilder[T]) ReturnsType(typ ReturnType) *MethodBuilder[T] {
    b.returnType = typ
    return b
}
```

This enables IDE autocomplete and static analysis tools.

## Migration Plan

1. **Add MethodRegistry** - Create `method_registry.go` with the new types
2. **Migrate one type** - Convert `String` as a proof of concept
3. **Validate** - Ensure all tests pass, REPL `:methods` works, `risor doc` works
4. **Migrate remaining types** - Convert `List`, `Bytes`, `Time`
5. **Remove old pattern** - Delete the now-unused `*Attrs` variables

Each step is independently deployable.

## Alternatives Considered

### Test-Time Validation Only

Keep the current pattern but add tests that verify all specs have implementations:

```go
func TestStringMethodsComplete(t *testing.T) {
    s := NewString("")
    for _, spec := range stringAttrs {
        _, ok := s.GetAttr(spec.Name)
        assert.True(t, ok, "missing GetAttr case for %s", spec.Name)
    }
}
```

**Rejected because:** Doesn't reduce boilerplate, still requires dual maintenance.

### Code Generation

Generate `GetAttr` from `AttrSpec` definitions using `go generate`.

**Rejected because:** Adds build complexity, harder to debug, implementation still needs to be written somewhere.

### Reflection-Based Discovery

Use Go reflection to auto-discover methods.

**Rejected because:** Too magical, poor error messages, doesn't align with Risor's "explicit over implicit" principle.

### Direct `Add()` with Struct Literal

The original proposal used `Add(AttrSpec{...}, impl)`:

```go
stringMethods.Add(
    AttrSpec{Name: "contains", Doc: "Check if substring exists", Args: []string{"substr"}, Returns: "bool"},
    func(s *String, ctx context.Context, args ...Object) (Object, error) {
        return s.Contains(args[0]), nil
    },
)
```

**Rejected because:** Requires full struct literal with field names on every call. The fluent builder is more readable and each line has a single concern.

## Design Decisions

The following questions from the original proposal have been resolved:

### Method Ordering

**Decision:** Registration order.

Alphabetical order loses intentional grouping (e.g., `split`/`join` together, `to_upper`/`to_lower` together). Registration order lets authors organize methods logically.

### Spec Immutability

**Decision:** Return a clone from `Specs()`.

The slice is small and `Specs()` is called infrequently (tooling, not hot path). Use `slices.Clone()` to prevent accidental mutation.

### Builtin Caching

**Decision:** Defer for now.

The current pattern creates a new `*Builtin` on each `GetAttr` call. This is simple and correct. Caching would require a per-instance map (since the closure captures `self`), adding complexity. Benchmark before optimizing—method calls may not be the bottleneck.

If profiling shows this matters, add caching as a follow-up:

```go
type MethodRegistry[T any] struct {
    // ...
    cache sync.Map // map[cacheKey]*Builtin where cacheKey includes self identity
}
```
