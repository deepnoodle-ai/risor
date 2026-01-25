package object

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/op"
)

var _ Callable = (*Builtin)(nil) // Ensure that *Builtin implements Callable

// BuiltinFunction holds the type of a built-in function.
type BuiltinFunction func(ctx context.Context, args ...Object) Object

// Builtin wraps func and implements Object interface.
type Builtin struct {
	*base

	// The function that this object wraps.
	fn BuiltinFunction

	// The name of the function.
	name string

	// The module the function originates from (optional)
	module *Module

	// The name of the module this function origiantes from.
	// This is only used for overriding builtins.
	moduleName string

	// If true, this function is built to handle errors and it should be
	// invoked even if one of its parameters evaluates to an error.
	isErrorHandler bool
}

func (b *Builtin) Type() Type {
	return BUILTIN
}

func (b *Builtin) Value() BuiltinFunction {
	return b.fn
}

func (b *Builtin) Interface() interface{} {
	return b.fn
}

func (b *Builtin) IsErrorHandler() bool {
	return b.isErrorHandler
}

func (b *Builtin) Call(ctx context.Context, args ...Object) Object {
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

func (b *Builtin) Equals(other Object) Object {
	if b == other {
		return True
	}
	return False
}

func (b *Builtin) RunOperation(opType op.BinaryOpType, right Object) Object {
	return TypeErrorf("type error: unsupported operation for builtin: %v", opType)
}

func (b *Builtin) MarshalJSON() ([]byte, error) {
	return nil, errz.TypeErrorf("type error: unable to marshal builtin")
}

// NewNoopBuiltin creates a builtin function that has no effect.
func NewNoopBuiltin(name string, module *Module) *Builtin {
	b := &Builtin{
		fn: func(ctx context.Context, args ...Object) Object {
			return Nil
		},
		name:   name,
		module: module,
	}
	if module != nil {
		b.moduleName = module.Name().value
	}
	return b
}

func NewBuiltin(name string, fn BuiltinFunction, module ...*Module) *Builtin {
	b := &Builtin{fn: fn, name: name}
	if len(module) == 1 {
		b.module = module[0]
	} else if len(module) > 1 {
		panic(fmt.Sprintf("NewBuiltin: too many arguments: %d", len(module)))
	}
	if b.module != nil {
		b.moduleName = b.module.Name().value
	}
	return b
}

func NewErrorHandler(name string, fn BuiltinFunction, module ...*Module) *Builtin {
	b := NewBuiltin(name, fn, module...)
	b.isErrorHandler = true
	return b
}
