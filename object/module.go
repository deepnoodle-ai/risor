package object

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/bytecode"
	"github.com/risor-io/risor/op"
)

type Module struct {
	*base
	name         string
	code         *bytecode.Code
	builtins     map[string]Object
	globals      []Object
	globalsIndex map[string]int
	callable     BuiltinFunction
}

func (m *Module) Type() Type {
	return MODULE
}

func (m *Module) Inspect() string {
	return m.String()
}

func (m *Module) GetAttr(name string) (Object, bool) {
	switch name {
	case "__name__":
		return NewString(m.name), true
	}
	if builtin, found := m.builtins[name]; found {
		return builtin, true
	}
	if index, found := m.globalsIndex[name]; found {
		return m.globals[index], true
	}
	return nil, false
}

func (m *Module) SetAttr(name string, value Object) error {
	return TypeErrorf("type error: cannot modify module attributes")
}

// Override provides a mechanism to modify module attributes after loading.
// Whether or not this is exposed to Risor scripts changes the security posture
// of reusing modules. By default, this is not exposed to scripting. Overriding
// with a value of nil is equivalent to deleting the attribute.
func (m *Module) Override(name string, value Object) error {
	if name == "__name__" {
		return TypeErrorf("type error: cannot override attribute %q", name)
	}
	if _, found := m.builtins[name]; found {
		if value == nil {
			delete(m.builtins, name)
			return nil
		}
		m.builtins[name] = value
		return nil
	}
	if index, found := m.globalsIndex[name]; found {
		if value == nil {
			delete(m.globalsIndex, name)
			return nil
		}
		m.globals[index] = value
		return nil
	}
	return TypeErrorf("type error: module has no attribute %q", name)
}

func (m *Module) Interface() interface{} {
	return nil
}

func (m *Module) String() string {
	return fmt.Sprintf("module(%s)", m.name)
}

func (m *Module) Name() *String {
	return NewString(m.name)
}

func (m *Module) Code() *bytecode.Code {
	return m.code
}

func (m *Module) Compare(other Object) (int, error) {
	otherMod, ok := other.(*Module)
	if !ok {
		return 0, TypeErrorf("type error: unable to compare module and %s", other.Type())
	}
	if m.name == otherMod.name {
		return 0, nil
	}
	if m.name > otherMod.name {
		return 1, nil
	}
	return -1, nil
}

func (m *Module) RunOperation(opType op.BinaryOpType, right Object) Object {
	return TypeErrorf("type error: unsupported operation for module: %v", opType)
}

func (m *Module) Equals(other Object) Object {
	if m == other {
		return True
	}
	return False
}

func (m *Module) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("type error: unable to marshal module")
}

func (m *Module) UseGlobals(globals []Object) {
	if len(globals) != len(m.globals) {
		panic(fmt.Sprintf("invalid module globals length: %d, expected: %d",
			len(globals), len(m.globals)))
	}
	m.globals = globals
}

func (m *Module) Call(ctx context.Context, args ...Object) Object {
	if m.callable == nil {
		return TypeErrorf("type error: module %q is not callable", m.name)
	}
	return m.callable(ctx, args...)
}

func NewModule(name string, code *bytecode.Code) *Module {
	globalsIndex := map[string]int{}
	globalsCount := code.GlobalCount()
	globals := make([]Object, globalsCount)
	for i := 0; i < globalsCount; i++ {
		globalName := code.GlobalNameAt(i)
		globalsIndex[globalName] = i
		// Initialize all globals to nil - they'll be set during VM execution
		globals[i] = Nil
	}
	return &Module{
		name:         name,
		builtins:     map[string]Object{},
		code:         code,
		globals:      globals,
		globalsIndex: globalsIndex,
	}
}

func NewBuiltinsModule(name string, contents map[string]Object, callableOption ...BuiltinFunction) *Module {
	builtins := map[string]Object{}
	for k, v := range contents {
		builtins[k] = v
	}
	var callable BuiltinFunction
	if len(callableOption) > 0 {
		callable = callableOption[0]
	}
	m := &Module{
		name:         name,
		builtins:     builtins,
		callable:     callable,
		globalsIndex: map[string]int{},
	}
	for _, v := range builtins {
		if builtin, ok := v.(*Builtin); ok {
			builtin.module = m
		}
	}
	return m
}
