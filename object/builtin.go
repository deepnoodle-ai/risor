package object

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/op"
)

var _ Callable = (*Builtin)(nil) // Ensure that *Builtin implements Callable

// BuiltinFunction holds the type of a built-in function.
type BuiltinFunction func(ctx context.Context, args ...Object) (Object, error)

// Builtin wraps func and implements Object interface.
type Builtin struct {
	// The function that this object wraps.
	fn BuiltinFunction

	// The name of the function.
	name string

	// The module the function originates from (optional). Used by GetAttr to
	// return the actual module object for the __module__ attribute.
	module *Module

	// The name of the module this function originates from. Used by Key() to
	// return the fully-qualified name (e.g., "math.sqrt"). This field takes
	// priority over module.Name() when set, allowing standalone builtins to
	// report a module name without having an actual module reference.
	moduleName string
}

func (b *Builtin) SetAttr(name string, value Object) error {
	return TypeErrorf("type error: builtin has no attribute %q", name)
}

func (b *Builtin) IsTruthy() bool {
	return true
}

func (b *Builtin) Type() Type {
	return BUILTIN
}

func (b *Builtin) Value() BuiltinFunction {
	return b.fn
}

func (b *Builtin) Interface() interface{} {
	return nil
}

func (b *Builtin) Call(ctx context.Context, args ...Object) (Object, error) {
	return b.fn(ctx, args...)
}

func (b *Builtin) Inspect() string {
	if b.module == nil {
		return fmt.Sprintf("builtin(%s)", b.name)
	}
	return fmt.Sprintf("builtin(%s.%s)", b.module.Name().value, b.name)
}

func (b *Builtin) String() string {
	return b.Inspect()
}

func (b *Builtin) Name() string {
	return b.name
}

func (b *Builtin) GetAttr(name string) (Object, bool) {
	switch name {
	case "__name__":
		return NewString(b.Key()), true
	case "__module__":
		if b.module != nil {
			return b.module, true
		}
		return Nil, true
	}
	return nil, false
}

// Returns a string that uniquely identifies this builtin function.
func (b *Builtin) Key() string {
	if b.module == nil && b.moduleName == "" {
		return b.name
	} else if b.moduleName != "" {
		return fmt.Sprintf("%s.%s", b.moduleName, b.name)
	}
	return fmt.Sprintf("%s.%s", b.module.Name().value, b.name)
}

func (b *Builtin) Equals(other Object) bool {
	otherBuiltin, ok := other.(*Builtin)
	if !ok {
		return false
	}
	return b == otherBuiltin
}

func (b *Builtin) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, fmt.Errorf("type error: unsupported operation for builtin: %v", opType)
}

func (b *Builtin) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("type error: unable to marshal builtin")
}

// NewNoopBuiltin creates a builtin function that has no effect.
// Use WithModule() to associate it with a module if needed.
func NewNoopBuiltin(name string) *Builtin {
	return &Builtin{
		fn: func(ctx context.Context, args ...Object) (Object, error) {
			return Nil, nil
		},
		name: name,
	}
}

// NewBuiltin creates a new builtin function with the given name and function.
// Use the builder methods InModule() or WithModule() to associate the builtin
// with a module.
func NewBuiltin(name string, fn BuiltinFunction) *Builtin {
	return &Builtin{fn: fn, name: name}
}

// InModule sets the module name for this builtin. This is used for the Key()
// method which returns the fully-qualified name (e.g., "math.sqrt").
func (b *Builtin) InModule(moduleName string) *Builtin {
	b.moduleName = moduleName
	return b
}

// WithModule sets the module for this builtin. The module name is derived
// from the module's name. Use this when you have a module reference.
func (b *Builtin) WithModule(module *Module) *Builtin {
	b.module = module
	if module != nil {
		b.moduleName = module.Name().value
	}
	return b
}
