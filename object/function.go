package object

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/risor-io/risor/op"
)

// Function is a function that has been compiled to bytecode.
type Function struct {
	*base
	name          string
	parameters    []string
	defaults      []Object
	defaultsCount int
	code          *Code
	freeVars      []*Cell
}

func (f *Function) Type() Type {
	return FUNCTION
}

func (f *Function) Name() string {
	return f.name
}

func (f *Function) Inspect() string {
	var out bytes.Buffer
	parameters := make([]string, 0)
	for i, name := range f.parameters {
		if def := f.defaults[i]; def != nil {
			name += "=" + def.Inspect()
		}
		parameters = append(parameters, name)
	}
	out.WriteString("func")
	if f.name != "" {
		out.WriteString(" " + f.name)
	}
	out.WriteString("(")
	out.WriteString(strings.Join(parameters, ", "))
	out.WriteString(") {")
	lines := strings.Split(f.Code().Source, "\n")
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

func (f *Function) String() string {
	if f.name != "" {
		return fmt.Sprintf("func %s() { ... }", f.name)
	}
	return "func() { ... }"
}

func (f *Function) Interface() interface{} {
	return nil
}

func (f *Function) RunOperation(opType op.BinaryOpType, right Object) Object {
	return NewError(fmt.Errorf("eval error: unsupported operation for function: %v", opType))
}

func (f *Function) Equals(other Object) Object {
	if f == other {
		return True
	}
	return False
}

func (f *Function) Instructions() []op.Code {
	return f.code.Instructions
}

func (f *Function) FreeVars() []*Cell {
	return f.freeVars
}

func (f *Function) Code() *Code {
	return f.code
}

func (f *Function) Parameters() []string {
	return f.parameters
}

func (f *Function) Defaults() []Object {
	return f.defaults
}

func (f *Function) RequiredArgsCount() int {
	return len(f.parameters) - f.defaultsCount
}

func (f *Function) LocalsCount() int {
	return int(f.code.Symbols.Size())
}

type FunctionOpts struct {
	Name           string
	ParameterNames []string
	Defaults       []Object
	Code           *Code
}

func NewFunction(opts FunctionOpts) *Function {
	var defaultsCount int
	for _, value := range opts.Defaults {
		if value != nil {
			defaultsCount++
		}
	}
	return &Function{
		name:          opts.Name,
		parameters:    opts.ParameterNames,
		defaults:      opts.Defaults,
		defaultsCount: defaultsCount,
		code:          opts.Code,
	}
}

func NewClosure(
	fn *Function,
	code *Code,
	freeVars []*Cell,
) *Function {
	return &Function{
		name:          fn.name,
		parameters:    fn.parameters,
		defaults:      fn.defaults,
		defaultsCount: fn.defaultsCount,
		code:          code,
		freeVars:      freeVars,
	}
}
