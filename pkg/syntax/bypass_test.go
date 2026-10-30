package syntax

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// A rest parameter is spread syntax.
func TestDisallowSpreadRejectsRestParam(t *testing.T) {
	validator := NewSyntaxValidator(SyntaxConfig{DisallowSpread: true})
	tests := []struct {
		source string
		col    int
	}{
		{`function f(...args) { return args }`, 14},
		{`let f = function(a, ...rest) { return rest }`, 23},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			errs := validator.Validate(parse(t, tt.source))
			assert.Len(t, errs, 1)
			assert.Equal(t, errs[0].Message, "spread syntax is not allowed")
			assert.Equal(t, errs[0].Position.Column, tt.col)
		})
	}
	// A function without a rest parameter is fine
	errs := validator.Validate(parse(t, `function f(a, b = 1) { return a }`))
	assert.Len(t, errs, 0)
}

// Each pipe stage calls a function, so a pipe is a function call.
func TestDisallowFuncCallRejectsPipe(t *testing.T) {
	validator := NewSyntaxValidator(SyntaxConfig{DisallowFuncCall: true})
	for _, source := range []string{`"42" |> int`, `x |> f |> g`} {
		t.Run(source, func(t *testing.T) {
			errs := validator.Validate(parse(t, source))
			assert.Len(t, errs, 1)
			assert.Equal(t, errs[0].Message, "function calls are not allowed")
		})
	}
	// With pipes also disallowed, the pipe error comes first
	both := NewSyntaxValidator(SyntaxConfig{DisallowFuncCall: true, DisallowPipe: true})
	errs := both.Validate(parse(t, `x |> f`))
	assert.Len(t, errs, 1)
	assert.Equal(t, errs[0].Message, "pipe expressions are not allowed")
}
