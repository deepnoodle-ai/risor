package object

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/risor-io/risor/bytecode"
	"github.com/risor-io/risor/op"
)

// Closure is a runtime function instance with captured variables.
// It references an immutable bytecode.Function for its signature and code,
// and holds runtime state like default values (as Objects) and free variables.
type Closure struct {
	*base
	fn            *bytecode.Function // Immutable function template
	defaults      []Object           // Pre-converted default values
	defaultsCount int                // Number of non-nil defaults
	freeVars      []*Cell            // Captured variables (closure state)
}

func (f *Closure) Type() Type {
	return FUNCTION
}

// Name returns the function name (delegates to bytecode.Function).
func (f *Closure) Name() string {
	return f.fn.Name()
}

func (f *Closure) Inspect() string {
	var out bytes.Buffer
	parameters := make([]string, 0)
	for i := 0; i < f.fn.ParameterCount(); i++ {
		name := f.fn.Parameter(i)
		if i < len(f.defaults) {
			if def := f.defaults[i]; def != nil {
				name += "=" + def.Inspect()
			}
		}
		parameters = append(parameters, name)
	}
	out.WriteString("func")
	if f.fn.Name() != "" {
		out.WriteString(" " + f.fn.Name())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(parameters, ", "))
	out.WriteString(") {")
	var source string
	if code := f.fn.Code(); code != nil {
		source = code.Source()
	}
	lines := strings.Split(source, "\n")
	if len(lines) == 1 {
		out.WriteString(" " + lines[0] + " }")
	} else if len(lines) == 0 {
		out.WriteString(" }")
	} else {
		for _, line := range lines {
			out.WriteString("\n    " + line)
		}
		out.WriteString("\n}")
	}
	return out.String()
}

func (f *Closure) String() string {
	if f.fn.Name() != "" {
		return fmt.Sprintf("func %s() { ... }", f.fn.Name())
	}
	return "func() { ... }"
}

func (f *Closure) Interface() interface{} {
	return nil
}

func (f *Closure) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (f *Closure) RunOperation(opType op.BinaryOpType, right Object) Object {
	return TypeErrorf("type error: unsupported operation for function: %v", opType)
}

func (f *Closure) Equals(other Object) Object {
	if f == other {
		return True
	}
	return False
}

// FreeVarCount returns the number of captured variables.
func (f *Closure) FreeVarCount() int {
	return len(f.freeVars)
}

// FreeVar returns the captured variable at the given index.
func (f *Closure) FreeVar(index int) *Cell {
	return f.freeVars[index]
}

// Code returns the bytecode for this function's body.
func (f *Closure) Code() *bytecode.Code {
	return f.fn.Code()
}

// BytecodeFunction returns the underlying bytecode.Function.
func (f *Closure) BytecodeFunction() *bytecode.Function {
	return f.fn
}

// ParameterCount returns the number of parameters (delegates to bytecode.Function).
func (f *Closure) ParameterCount() int {
	return f.fn.ParameterCount()
}

// Parameter returns the parameter name at the given index (delegates to bytecode.Function).
func (f *Closure) Parameter(index int) string {
	return f.fn.Parameter(index)
}

// DefaultCount returns the number of default parameter values.
func (f *Closure) DefaultCount() int {
	return len(f.defaults)
}

// Default returns the default parameter value at the given index.
func (f *Closure) Default(index int) Object {
	if index < 0 || index >= len(f.defaults) {
		return nil
	}
	return f.defaults[index]
}

// RequiredArgsCount returns the minimum number of arguments required.
func (f *Closure) RequiredArgsCount() int {
	return f.fn.ParameterCount() - f.defaultsCount
}

// RestParam returns the rest parameter name (delegates to bytecode.Function).
func (f *Closure) RestParam() string {
	return f.fn.RestParam()
}

// HasRestParam returns true if the function has a rest parameter.
func (f *Closure) HasRestParam() bool {
	return f.fn.HasRestParam()
}

// LocalsCount returns the number of local variables.
func (f *Closure) LocalsCount() int {
	return f.fn.LocalCount()
}

func (f *Closure) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("type error: unable to marshal function")
}

func (f *Closure) Call(ctx context.Context, args ...Object) Object {
	callFunc, found := GetCallFunc(ctx)
	if !found {
		return Errorf("eval error: context did not contain a call function")
	}
	result, err := callFunc(ctx, f, args)
	if err != nil {
		return NewError(err)
	}
	return result
}

// NewClosure creates a Closure from a bytecode.Function template.
// The defaults are converted from Go types to Object types.
func NewClosure(fn *bytecode.Function) *Closure {
	// Convert parameter defaults to Objects
	var defaults []Object
	var defaultsCount int
	for i := 0; i < fn.DefaultCount(); i++ {
		value := fn.Default(i)
		if value != nil {
			defaultsCount++
			defaults = append(defaults, FromGoType(value))
		} else {
			defaults = append(defaults, nil)
		}
	}

	return &Closure{
		fn:            fn,
		defaults:      defaults,
		defaultsCount: defaultsCount,
	}
}

// CloneWithCaptures creates a new closure from an existing closure with captured variables.
// The defaults slice is shared (not copied) since it's immutable after construction.
func CloneWithCaptures(c *Closure, freeVars []*Cell) *Closure {
	return &Closure{
		fn:            c.fn,
		defaults:      c.defaults,
		defaultsCount: c.defaultsCount,
		freeVars:      freeVars,
	}
}
