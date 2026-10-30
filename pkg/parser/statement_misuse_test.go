package parser

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// A statement produces no value, so it cannot be a call argument. The error
// points at the statement.
func TestStatementCallArgumentRejected(t *testing.T) {
	tests := []struct {
		input    string
		message  string
		startCol int
		endCol   int
	}{
		{`f(x = 2)`, "expected an expression in call arguments (got an assignment)", 2, 6},
		{`f(x += 2)`, "expected an expression in call arguments (got an assignment)", 2, 7},
		{`f(1, x++)`, "expected an expression in call arguments (got a ++ statement)", 5, 7},
		{`f(x--)`, "expected an expression in call arguments (got a -- statement)", 2, 4},
		{`f(o.a = 2)`, "expected an expression in call arguments (got an assignment)", 2, 8},
		{`f(a[0] = 2)`, "expected an expression in call arguments (got an assignment)", 2, 9},
		{`f(x = 2, ...arr)`, "expected an expression in call arguments (got an assignment)", 2, 6},
		{`o.m(x = 2)`, "expected an expression in call arguments (got an assignment)", 4, 8},
		{`o?.m(x++)`, "expected an expression in call arguments (got a ++ statement)", 5, 7},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			errs := parseErrors(t, tt.input)
			assert.Equal(t, len(errs), 1)
			assert.Equal(t, errs[0].Message(), tt.message)
			assert.Equal(t, errs[0].StartPosition().Column, tt.startCol)
			assert.Equal(t, errs[0].EndPosition().Column, tt.endCol)
		})
	}
}

// Expressions that look like statements remain valid call arguments.
func TestExpressionCallArgumentsAccepted(t *testing.T) {
	for _, input := range []string{
		`f(x == 2)`,
		`f((a = 1) => a)`,
		`f(({a = 1}) => a)`,
		`f(function() { x = 1 })`,
		`f(...xs, y)`,
	} {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(t.Context(), input, nil)
			assert.Nil(t, err)
		})
	}
}

// Like assignment, ++ and -- cannot target an optional chain. The error
// points at the operator.
func TestPostfixOnOptionalChainRejected(t *testing.T) {
	for _, input := range []string{`o?.x++`, `o?.x--`} {
		t.Run(input, func(t *testing.T) {
			errs := parseErrors(t, input)
			assert.Equal(t, len(errs), 1)
			assert.Equal(t, errs[0].Message(), "cannot apply postfix operator to an optional chain")
			assert.Equal(t, errs[0].StartPosition().Column, 4)
		})
	}
	// Only an optional target is rejected. An optional link earlier in the
	// chain does not short-circuit the rest of it, so the target is defined.
	for _, input := range []string{`o.x++`, `o?.x.y++`, `o?.x[0]++`} {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(t.Context(), input, nil)
			assert.Nil(t, err)
		})
	}
}
