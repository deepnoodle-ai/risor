package object

import (
	"context"
	"fmt"
	"slices"
)

// AttrDef combines an attribute's specification with its implementation.
// It supports both properties (direct value access) and methods (callable).
type AttrDef[T any] struct {
	Spec       AttrSpec
	IsProperty bool
	MinArgs    int // Minimum required arguments (for optional arg support)
	// For methods:
	MethodImpl func(self T, ctx context.Context, args ...Object) (Object, error)
	// For properties:
	PropertyImpl func(self T) Object
}

// AttrRegistry holds all attributes (properties and methods) for a given object type.
type AttrRegistry[T any] struct {
	typeName string
	attrs    map[string]AttrDef[T]
	specs    []AttrSpec
}

// AttrBuilder provides a fluent API for defining a single attribute.
type AttrBuilder[T any] struct {
	registry    *AttrRegistry[T]
	name        string
	doc         string
	args        []string
	optionalIdx int // Index where optional args start (-1 means all required)
	returns     string
}

// NewAttrRegistry creates a registry for the given type name.
func NewAttrRegistry[T any](typeName string) *AttrRegistry[T] {
	return &AttrRegistry[T]{
		typeName: typeName,
		attrs:    make(map[string]AttrDef[T]),
	}
}

// Define starts building a new attribute definition.
// Returns an AttrBuilder for fluent configuration.
func (r *AttrRegistry[T]) Define(name string) *AttrBuilder[T] {
	return &AttrBuilder[T]{
		registry: r,
		name:     name,
	}
}

// Specs returns a copy of all registered attribute specifications in registration order.
func (r *AttrRegistry[T]) Specs() []AttrSpec {
	return slices.Clone(r.specs)
}

// GetAttr returns the named attribute bound to self.
// For properties, returns the value directly.
// For methods, returns a Builtin wrapper.
// Returns nil, false if the attribute doesn't exist.
func (r *AttrRegistry[T]) GetAttr(self T, name string) (Object, bool) {
	attr, ok := r.attrs[name]
	if !ok {
		return nil, false
	}

	if attr.IsProperty {
		return attr.PropertyImpl(self), true
	}

	// Method: wrap in Builtin with argument validation
	minArgs := attr.MinArgs
	maxArgs := len(attr.Spec.Args)
	fullName := r.typeName + "." + name
	return &Builtin{
		name: fullName,
		fn: func(ctx context.Context, args ...Object) (Object, error) {
			if len(args) < minArgs || len(args) > maxArgs {
				return nil, argsRangeError(fullName, minArgs, maxArgs, len(args))
			}
			return attr.MethodImpl(self, ctx, args...)
		},
	}, true
}

// Doc sets the attribute's documentation string.
func (b *AttrBuilder[T]) Doc(doc string) *AttrBuilder[T] {
	b.doc = doc
	return b
}

// Arg adds a required argument by name (for methods).
func (b *AttrBuilder[T]) Arg(name string) *AttrBuilder[T] {
	b.args = append(b.args, name)
	return b
}

// Args adds multiple required arguments (for methods).
func (b *AttrBuilder[T]) Args(names ...string) *AttrBuilder[T] {
	b.args = append(b.args, names...)
	return b
}

// OptionalArg adds an optional argument by name (for methods).
// Optional args must come after all required args.
func (b *AttrBuilder[T]) OptionalArg(name string) *AttrBuilder[T] {
	if b.optionalIdx == 0 {
		// First optional arg - mark where optional args start
		b.optionalIdx = len(b.args) + 1 // +1 because we use 0 as "not set"
	}
	b.args = append(b.args, name)
	return b
}

// Returns sets the return type (for documentation/tooling).
func (b *AttrBuilder[T]) Returns(typ string) *AttrBuilder[T] {
	b.returns = typ
	return b
}

// Impl sets the method implementation and registers the attribute.
// Use this for callable methods that take arguments.
// Panics if an attribute with the same name is already registered.
func (b *AttrBuilder[T]) Impl(fn func(T, context.Context, ...Object) (Object, error)) {
	r := b.registry
	if _, exists := r.attrs[b.name]; exists {
		panic(fmt.Sprintf("%s: attribute %q already registered", r.typeName, b.name))
	}
	spec := AttrSpec{
		Name:    b.name,
		Doc:     b.doc,
		Args:    b.args,
		Returns: b.returns,
	}
	// Calculate minimum required args
	minArgs := len(b.args)
	if b.optionalIdx > 0 {
		minArgs = b.optionalIdx - 1 // -1 because optionalIdx is 1-indexed
	}
	r.attrs[b.name] = AttrDef[T]{Spec: spec, MinArgs: minArgs, MethodImpl: fn}
	r.specs = append(r.specs, spec)
}

// Getter sets the property getter and registers the attribute.
// Use this for read-only properties that return a value directly.
// Panics if an attribute with the same name is already registered.
func (b *AttrBuilder[T]) Getter(fn func(T) Object) {
	r := b.registry
	if _, exists := r.attrs[b.name]; exists {
		panic(fmt.Sprintf("%s: attribute %q already registered", r.typeName, b.name))
	}
	if len(b.args) > 0 {
		panic(fmt.Sprintf("%s: property %q cannot have arguments", r.typeName, b.name))
	}
	spec := AttrSpec{
		Name:    b.name,
		Doc:     b.doc,
		Args:    nil,
		Returns: b.returns,
	}
	r.attrs[b.name] = AttrDef[T]{Spec: spec, IsProperty: true, PropertyImpl: fn}
	r.specs = append(r.specs, spec)
}

// argsError returns a grammatically correct argument count error.
func argsError(methodName string, expected, got int) error {
	if expected == 1 {
		return fmt.Errorf("%s: expected 1 argument, got %d", methodName, got)
	}
	return fmt.Errorf("%s: expected %d arguments, got %d", methodName, expected, got)
}

// argsRangeError returns a grammatically correct argument count error for methods with optional args.
func argsRangeError(methodName string, min, max, got int) error {
	if min == max {
		return argsError(methodName, min, got)
	}
	return fmt.Errorf("%s: expected %d to %d arguments, got %d", methodName, min, max, got)
}

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

// Aliases for backward compatibility during migration
type (
	MethodRegistry[T any] = AttrRegistry[T]
	MethodBuilder[T any]  = AttrBuilder[T]
	MethodDef[T any]      = AttrDef[T]
)

func NewMethodRegistry[T any](typeName string) *AttrRegistry[T] {
	return NewAttrRegistry[T](typeName)
}
