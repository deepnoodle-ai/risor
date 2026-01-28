package bytecode

import (
	"bytes"
	"fmt"
	"strings"
)

// Function represents a compiled function template.
// It is immutable after creation and contains all the static information
// needed to create closures at runtime.
type Function struct {
	id            string
	name          string
	parameters    []string
	defaults      []any
	restParam     string // Name of rest parameter (empty if none)
	code          *Code
	requiredCount int // Precomputed: len(parameters) - len(defaults)
}

// FunctionParams contains parameters for creating a new Function.
type FunctionParams struct {
	ID         string
	Name       string
	Parameters []string
	Defaults   []any
	RestParam  string
	Code       *Code
}

// NewFunction creates a new immutable Function from the given parameters.
// Input slices are copied to ensure immutability.
func NewFunction(params FunctionParams) *Function {
	parameters := copyStrings(params.Parameters)
	defaults := copyAny(params.Defaults)

	// Count non-nil defaults to compute required args correctly.
	// A parameter with a nil default still requires an argument.
	defaultsWithValue := 0
	for _, d := range defaults {
		if d != nil {
			defaultsWithValue++
		}
	}

	return &Function{
		id:            params.ID,
		name:          params.Name,
		parameters:    parameters,
		defaults:      defaults,
		restParam:     params.RestParam,
		code:          params.Code,
		requiredCount: len(parameters) - defaultsWithValue,
	}
}

// ID returns the unique identifier for this function.
func (f *Function) ID() string {
	return f.id
}

// Name returns the function name, or empty string for anonymous functions.
func (f *Function) Name() string {
	return f.name
}

// Code returns the compiled bytecode for this function's body.
func (f *Function) Code() *Code {
	return f.code
}

// ParameterCount returns the number of parameters.
func (f *Function) ParameterCount() int {
	return len(f.parameters)
}

// Parameter returns the name of the parameter at the given index.
func (f *Function) Parameter(index int) string {
	return f.parameters[index]
}

// DefaultCount returns the number of default parameter values.
func (f *Function) DefaultCount() int {
	return len(f.defaults)
}

// Default returns the default value at the given index.
// May return nil if no default is set for that parameter.
func (f *Function) Default(index int) any {
	return f.defaults[index]
}

// RequiredArgsCount returns the minimum number of arguments required.
// This is precomputed during construction for O(1) access.
func (f *Function) RequiredArgsCount() int {
	return f.requiredCount
}

// LocalCount returns the number of local variables in the function body.
func (f *Function) LocalCount() int {
	if f.code == nil {
		return 0
	}
	return f.code.LocalCount()
}

// RestParam returns the name of the rest parameter, or empty string if none.
func (f *Function) RestParam() string {
	return f.restParam
}

// HasRestParam returns true if the function has a rest parameter.
func (f *Function) HasRestParam() bool {
	return f.restParam != ""
}

// String returns a string representation of the function.
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
