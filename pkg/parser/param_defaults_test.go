package parser

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// A parameter without a default may not follow one with a default, and only
// one "_" parameter may have a default. The error points at the parameter.
func TestParamDefaultsRejected(t *testing.T) {
	tests := []struct {
		input    string
		message  string
		startCol int
		endCol   int
	}{
		{`function g(_ = 1, _) {}`, `parameter "_" needs a default value, since a parameter before it has one`, 18, 18},
		{`function g(a = 1, bc) {}`, `parameter "bc" needs a default value, since a parameter before it has one`, 18, 19},
		{`function g(a = 1, {b}) {}`, `destructuring parameter needs a default value, since a parameter before it has one`, 18, 18},
		{`function g(a = 1, [b]) {}`, `destructuring parameter needs a default value, since a parameter before it has one`, 18, 18},
		{`function f(_ = 1, _ = 2) {}`, `only one "_" parameter can have a default value`, 18, 18},
		{`function f(x, _ = 1, y = 2, _ = 3) {}`, `only one "_" parameter can have a default value`, 28, 28},
		{`(_ = 1, _) => 1`, `parameter "_" needs a default value, since a parameter before it has one`, 8, 8},
		{`(a = 1, b) => 1`, `parameter "b" needs a default value, since a parameter before it has one`, 8, 8},
		{`(a = 1, {b}) => 1`, `destructuring parameter needs a default value, since a parameter before it has one`, 8, 8},
		{`(_ = 1, _ = 2) => 1`, `only one "_" parameter can have a default value`, 8, 8},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			errs := parseErrors(t, tt.input)
			assert.Equal(t, errs[0].Message(), tt.message)
			assert.Equal(t, errs[0].StartPosition().Column, tt.startCol)
			assert.Equal(t, errs[0].EndPosition().Column, tt.endCol)
		})
	}
}

func TestParamDefaultsAccepted(t *testing.T) {
	for _, input := range []string{
		`function f(_, _) {}`,
		`function f(_, _ = 1) {}`,
		`function f(_ = 1) {}`,
		`function f(a, b = 1, c = 2) {}`,
		`function f({a = 1}, b) {}`,
		`function f(a = 1, ...rest) {}`,
		`(_, _ = 1) => 1`,
		`(a, b = 1) => 1`,
		`({a = 1}, b) => 1`,
	} {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(t.Context(), input, nil)
			assert.Nil(t, err)
		})
	}
}

// An arrow function destructuring key is an identifier, as in function
// parameters. A quoted or template key is rejected, not silently used.
func TestArrowDestructureKeyMustBeIdentifier(t *testing.T) {
	for _, input := range []string{
		`({"k": a}) => a`,
		"({`k`: a}) => a",
		"({`${x}`: a}) => a",
		`({a, "b": c}) => a`,
	} {
		t.Run(input, func(t *testing.T) {
			errs := parseErrors(t, input)
			assert.Contains(t, errs[0].Message(), "expected identifier in destructuring pattern")
		})
	}
	for _, input := range []string{`({a, b}) => a`, `({a: x, b = 1}) => a`} {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(t.Context(), input, nil)
			assert.Nil(t, err)
		})
	}
}
