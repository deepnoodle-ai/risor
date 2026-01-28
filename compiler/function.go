package compiler

import (
	"bytes"
	"fmt"
	"strings"
)

type Function struct {
	id         string
	name       string
	parameters []string
	defaults   []any
	restParam  string // Name of rest parameter (empty if none)
	code       *Code
}

func (f *Function) ID() string {
	return f.id
}

func (f *Function) Name() string {
	return f.name
}

func (f *Function) Code() *Code {
	return f.code
}

func (f *Function) ParametersCount() int {
	return len(f.parameters)
}

func (f *Function) Parameter(index int) string {
	return f.parameters[index]
}

func (f *Function) DefaultsCount() int {
	return len(f.defaults)
}

func (f *Function) Default(index int) any {
	return f.defaults[index]
}

// RequiredArgsCount returns the minimum number of arguments required.
// A parameter requires an argument if it has no default value (nil in defaults array).
func (f *Function) RequiredArgsCount() int {
	// Count non-nil defaults (nil means no default for that parameter)
	// Only consider defaults up to the parameter count to avoid negative results
	paramCount := len(f.parameters)
	required := paramCount
	for i, d := range f.defaults {
		if i >= paramCount {
			break
		}
		if d != nil {
			required--
		}
	}
	return required
}

func (f *Function) LocalsCount() int {
	return int(f.code.LocalsCount())
}

func (f *Function) String() string {
	var out bytes.Buffer
	parameters := make([]string, 0)
	for i, name := range f.parameters {
		if i < len(f.defaults) {
			if def := f.defaults[i]; def != nil {
				name += "=" + fmt.Sprintf("%v", def)
			}
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
	var source string
	if f.code != nil {
		source = f.code.Source()
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

type FunctionOpts struct {
	ID         string
	Name       string
	Parameters []string
	Defaults   []any
	RestParam  string // Name of rest parameter (empty if none)
	Code       *Code
}

func NewFunction(opts FunctionOpts) *Function {
	return &Function{
		id:         opts.ID,
		name:       opts.Name,
		parameters: opts.Parameters,
		defaults:   opts.Defaults,
		restParam:  opts.RestParam,
		code:       opts.Code,
	}
}

func (f *Function) RestParam() string {
	return f.restParam
}

func (f *Function) HasRestParam() bool {
	return f.restParam != ""
}
