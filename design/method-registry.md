# Attribute Registry Design

**Status:** Implemented
**Author:** Claude
**Date:** 2026-01-27

## Summary

Replace the current dual-maintenance pattern (AttrSpec slice + GetAttr switch) with a unified `AttrRegistry` that combines attribute specification and implementation in a single definition, using a fluent builder API. The registry supports both **methods** (callable attributes) and **properties** (read-only value attributes).

## Motivation

The current approach to defining object attributes has several pain points:

1. **Dual Maintenance** - Adding an attribute requires updating two separate places: the `*Attrs` slice and the `GetAttr` switch statement. These can drift out of sync.

2. **No Compile-Time Validation** - If a spec exists without an implementation (or vice versa), there's no error until runtime.

3. **Repetitive Boilerplate** - Every method case in `GetAttr` repeats the same pattern: create `&Builtin{}`, validate arg count, call underlying method.

4. **Duplicated Knowledge** - The arg count is specified in `AttrSpec.Args` but manually validated again in each `GetAttr` case.

5. **No Property Support** - Properties (like `range.start`) require separate handling from methods, with no shared documentation infrastructure.

## Design

### Core Types

```go
// AttrDef combines an attribute's specification with its implementation.
type AttrDef[T any] struct {
    Spec         AttrSpec
    IsProperty   bool
    MethodImpl   func(self T, ctx context.Context, args ...Object) (Object, error)
    PropertyImpl func(self T) Object
}

// AttrRegistry holds all attributes for a given object type.
type AttrRegistry[T any] struct {
    typeName string
    attrs    map[string]AttrDef[T]
    specs    []AttrSpec
}

// AttrBuilder provides a fluent API for defining a single attribute.
type AttrBuilder[T any] struct {
    registry *AttrRegistry[T]
    name     string
    doc      string
    args     []string
    returns  string
}
```

### Fluent Builder API

```go
// NewAttrRegistry creates a registry for the given type name.
func NewAttrRegistry[T any](typeName string) *AttrRegistry[T]

// Define starts building a new attribute definition.
func (r *AttrRegistry[T]) Define(name string) *AttrBuilder[T]

// Specs returns a copy of all registered attribute specifications.
func (r *AttrRegistry[T]) Specs() []AttrSpec

// GetAttr returns the named attribute bound to self.
// For properties, returns the value directly.
// For methods, returns a Builtin wrapper.
func (r *AttrRegistry[T]) GetAttr(self T, name string) (Object, bool)

// AttrBuilder methods:
func (b *AttrBuilder[T]) Doc(doc string) *AttrBuilder[T]
func (b *AttrBuilder[T]) Arg(name string) *AttrBuilder[T]
func (b *AttrBuilder[T]) Args(names ...string) *AttrBuilder[T]
func (b *AttrBuilder[T]) Returns(typ string) *AttrBuilder[T]

// Impl registers a callable method.
func (b *AttrBuilder[T]) Impl(fn func(T, context.Context, ...Object) (Object, error))

// Getter registers a read-only property.
func (b *AttrBuilder[T]) Getter(fn func(T) Object)
```

### Usage Examples

**Methods** (callable attributes that return a Builtin):

```go
var stringAttrs = NewAttrRegistry[*String]("string")

func init() {
    stringAttrs.Define("split").
        Doc("Split by separator").
        Arg("sep").
        Returns("list").
        Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
            return s.Split(args[0])
        })

    stringAttrs.Define("to_lower").
        Doc("Convert to lowercase").
        Returns("string").
        Impl(func(s *String, ctx context.Context, args ...Object) (Object, error) {
            return s.ToLower(), nil
        })
}
```

**Properties** (read-only attributes that return values directly):

```go
var rangeAttrs = NewAttrRegistry[*Range]("range")

func init() {
    rangeAttrs.Define("start").
        Doc("The start value of the range").
        Returns("int").
        Getter(func(r *Range) Object {
            return NewInt(r.start)
        })

    rangeAttrs.Define("stop").
        Doc("The stop value of the range (exclusive)").
        Returns("int").
        Getter(func(r *Range) Object {
            return NewInt(r.stop)
        })
}
```

**Type implementation:**

```go
func (r *Range) Attrs() []AttrSpec {
    return rangeAttrs.Specs()
}

func (r *Range) GetAttr(name string) (Object, bool) {
    return rangeAttrs.GetAttr(r, name)
}
```

## Implementation Status

### Migrated Types

| Type | Methods | Properties | Total |
|------|---------|------------|-------|
| String | 18 | 0 | 18 |
| List | 15 | 0 | 15 |
| Bytes | 15 | 0 | 15 |
| Time | 6 | 0 | 6 |
| Error | 8 | 0 | 8 |
| Color | 1 | 0 | 1 |
| Range | 0 | 3 | 3 |
| Module | 0 | 1 | 1 |
| Builtin | 0 | 2 | 2 |

**Total: 9 types, 63 methods, 6 properties**

### Types Without Attributes

These types have `GetAttr` returning `nil, false`:
- Int, Float, Byte, Bool
- Closure, Partial, Cell
- NilType, DynamicAttr

### Special Cases

**Map** - Uses `GetAttr` for key lookup, not method dispatch. Not a candidate for the registry.

## Benefits

| Aspect | Before | After |
|--------|--------|-------|
| Definitions per attribute | 2 (spec + case) | 1 (Define chain) |
| Lines per attribute | ~12 | ~5 |
| Sync errors possible | Yes | No |
| Arg count validation | Manual | Automatic |
| Property support | Ad-hoc | Unified |
| Duplicate detection | None | Panic at init |
| Introspection | Partial | Complete |

## Backward Compatibility

Type aliases are provided for migration:

```go
type MethodRegistry[T any] = AttrRegistry[T]
type MethodBuilder[T any] = AttrBuilder[T]

func NewMethodRegistry[T any](typeName string) *AttrRegistry[T] {
    return NewAttrRegistry[T](typeName)
}
```

## Future Extensions

### Variadic Methods

```go
stringAttrs.Define("format").
    Doc("Format with arguments").
    Arg("template").
    Variadic().
    Returns("string").
    Impl(...)
```

### Optional Arguments

```go
listAttrs.Define("pop").
    Doc("Remove and return item").
    OptionalArg("index", NewInt(-1)).
    Returns("any").
    Impl(...)
```

### Writable Properties

```go
attrs.Define("name").
    Doc("The object name").
    Returns("string").
    Getter(func(o *Obj) Object { return NewString(o.name) }).
    Setter(func(o *Obj, v Object) error { o.name = v.(*String).Value(); return nil })
```
