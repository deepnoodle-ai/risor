# Type Registry for Risor v2

## Status

Implemented

## Summary

The `TypeRegistry` handles conversion between Go values and Risor Objects.
It replaces the previous global, mutex-protected type converter system with
an explicit registry configured on the VM.

## API

### Core Types

```go
// TypeRegistry handles conversion between Go values and Risor Objects.
// It is immutable after construction and safe for concurrent use.
type TypeRegistry struct { ... }

// FromGo converts a Go value to a Risor Object.
func (r *TypeRegistry) FromGo(v any) (Object, error)

// ToGo converts a Risor Object to a Go value of the specified type.
func (r *TypeRegistry) ToGo(obj Object, targetType reflect.Type) (any, error)

// FromGoFunc converts a Go value to a Risor Object.
type FromGoFunc func(v any) (Object, error)

// ToGoFunc converts a Risor Object to a Go value of a specific type.
type ToGoFunc func(obj Object, targetType reflect.Type) (any, error)
```

### Registry Builder

```go
// Create a custom registry
registry := risor.NewTypeRegistry().
    RegisterFromGo(reflect.TypeOf(MyType{}), myFromGoFunc).
    RegisterToGo(reflect.TypeOf(MyType{}), myToGoFunc).
    Build()

// Use with Eval
result, err := risor.Eval(ctx, source,
    risor.WithEnv(risor.Builtins()),
    risor.WithTypeRegistry(registry))
```

### RisorValuer Interface

Go types can implement `RisorValuer` for automatic conversion without
registering a custom converter:

```go
type RisorValuer interface {
    RisorValue() Object
}

// Example
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
```

### Default Registry

`DefaultRegistry()` returns a registry with converters for all built-in types:
- Primitives: bool, int/int8/.../int64, uint/uint8/.../uint64, float32/float64, string
- Containers: slices, arrays, maps (string keys only)
- Special types: []byte, time.Time, json.Number
- Pointers: automatically dereferenced/created

### Convenience Functions

```go
// FromGoType converts using DefaultRegistry, returns error as *Error Object
func FromGoType(obj any) Object

// AsObjects converts a map using DefaultRegistry
func AsObjects(m map[string]any) (map[string]Object, error)

// AsObjectsWithRegistry converts using a specific registry
func AsObjectsWithRegistry(m map[string]any, registry *TypeRegistry) (map[string]Object, error)
```

## VM Integration

```go
// Option to set registry on VM
vm.WithTypeRegistry(registry *object.TypeRegistry)

// Get registry from VM (returns DefaultRegistry if not set)
vm.TypeRegistry() *object.TypeRegistry
```

## Example Usage

### Custom Type with Registry

```go
type Point struct {
    X, Y int
}

registry := risor.NewTypeRegistry().
    RegisterFromGo(reflect.TypeOf(Point{}), func(v any) (object.Object, error) {
        p := v.(Point)
        return object.NewMap(map[string]object.Object{
            "x": object.NewInt(int64(p.X)),
            "y": object.NewInt(int64(p.Y)),
        }), nil
    }).
    RegisterToGo(reflect.TypeOf(Point{}), func(obj object.Object, _ reflect.Type) (any, error) {
        m, err := object.AsMap(obj)
        if err != nil {
            return nil, err
        }
        x, _ := object.AsInt(m.Get("x"))
        y, _ := object.AsInt(m.Get("y"))
        return Point{X: int(x), Y: int(y)}, nil
    }).
    Build()

result, err := risor.Eval(ctx, "point.x + point.y",
    risor.WithEnv(map[string]any{"point": Point{X: 10, Y: 20}}),
    risor.WithTypeRegistry(registry))
// result = 30
```

### Custom Type with RisorValuer

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

// Automatic conversion - no registry configuration needed
result, err := risor.Eval(ctx, "user.name",
    risor.WithEnv(map[string]any{"user": User{ID: 1, Name: "Alice"}}))
// result = "Alice"
```

## Design Decisions

1. **No global state**: Registry is explicit, passed to VM
2. **Immutable after construction**: No mutex needed, safe for concurrent use
3. **Unified numeric handling**: One function handles all int/uint/float conversions
4. **Clear extension point**: `RegisterFromGo` / `RegisterToGo` via builder
5. **Optional interface**: `RisorValuer` for zero-config custom types
6. **Consistent errors**: All conversions return `(T, error)`

## Files

| File | Description |
|------|-------------|
| `object/typeconv.go` | TypeRegistry, RegistryBuilder, converters |
| `vm/vm.go` | typeRegistry field, TypeRegistry() method |
| `vm/options.go` | WithTypeRegistry option |
| `risor.go` | NewTypeRegistry(), WithTypeRegistry() |
