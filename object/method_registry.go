package object

import (
	"context"
	"fmt"
	"slices"
)

// MethodDef combines a method's specification with its implementation.
type MethodDef[T any] struct {
	Spec AttrSpec
	Impl func(self T, ctx context.Context, args ...Object) (Object, error)
}

// MethodRegistry holds all methods for a given object type.
type MethodRegistry[T any] struct {
	typeName string
	methods  map[string]MethodDef[T]
	specs    []AttrSpec
}

// MethodBuilder provides a fluent API for defining a single method.
type MethodBuilder[T any] struct {
	registry *MethodRegistry[T]
	name     string
	doc      string
	args     []string
	returns  string
}

// NewMethodRegistry creates a registry for the given type name.
func NewMethodRegistry[T any](typeName string) *MethodRegistry[T] {
	return &MethodRegistry[T]{
		typeName: typeName,
		methods:  make(map[string]MethodDef[T]),
	}
}

// Define starts building a new method definition.
// Returns a MethodBuilder for fluent configuration.
func (r *MethodRegistry[T]) Define(name string) *MethodBuilder[T] {
	return &MethodBuilder[T]{
		registry: r,
		name:     name,
	}
}

// Specs returns a copy of all registered method specifications in registration order.
func (r *MethodRegistry[T]) Specs() []AttrSpec {
	return slices.Clone(r.specs)
}

// GetAttr returns a Builtin for the named method bound to self.
// Returns nil, false if the method doesn't exist.
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

// Doc sets the method's documentation string.
func (b *MethodBuilder[T]) Doc(doc string) *MethodBuilder[T] {
	b.doc = doc
	return b
}

// Arg adds a required argument by name.
func (b *MethodBuilder[T]) Arg(name string) *MethodBuilder[T] {
	b.args = append(b.args, name)
	return b
}

// Args adds multiple required arguments.
func (b *MethodBuilder[T]) Args(names ...string) *MethodBuilder[T] {
	b.args = append(b.args, names...)
	return b
}

// Returns sets the return type (for documentation/tooling).
func (b *MethodBuilder[T]) Returns(typ string) *MethodBuilder[T] {
	b.returns = typ
	return b
}

// Impl sets the implementation and registers the method.
// Panics if a method with the same name is already registered.
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
