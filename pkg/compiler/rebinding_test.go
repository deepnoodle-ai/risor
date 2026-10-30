package compiler

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

func compileErr(t *testing.T, source string) error {
	t.Helper()
	parsed, err := parser.Parse(context.Background(), source, nil)
	assert.Nil(t, err)
	_, err = Compile(parsed, nil)
	return err
}

// Postfix ++ and -- assign to their operand, so they must not change a
// constant, wherever the constant is defined.
func TestPostfixOnConstant(t *testing.T) {
	tests := []string{
		`const x = 1; x++`,
		`const x = 1; x--`,
		`function g() { const x = 1; x++; return x }`,
		`function g() { const x = 1; let h = () => { x-- }; h(); return x }`,
		`if (true) { const x = 1; x++ }`,
	}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			err := compileErr(t, source)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), `cannot assign to constant "x"`)
		})
	}
}

func TestPostfixOnBlankIdentifier(t *testing.T) {
	err := compileErr(t, `_++`)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot use _ with ++")
}

func TestPostfixOnVariable(t *testing.T) {
	tests := []string{
		`let x = 1; x++`,
		`function g() { let x = 1; x--; return x }`,
		`function g() { let x = 1; let h = () => { x++ }; h(); return x }`,
	}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			assert.Nil(t, compileErr(t, source))
		})
	}
}

// A named function that is not a top-level declaration claims a new symbol
// in its scope. It must not take over an existing binding of the same name.
func TestNamedFunctionDoesNotRebind(t *testing.T) {
	tests := []string{
		`if (true) { const x = 1; function x() { return 2 }; x() }`,
		`if (true) { let x = 1; function x() { return 2 }; x() }`,
		`if (true) { function x() { return 1 }; function x() { return 2 }; x() }`,
		`function outer() { const x = 1; function x() { return 2 }; return x() }`,
		`function outer() { function x() { return 1 }; function x() { return 2 }; return x() }`,
		`const x = 1; let g = function x() { return 2 }; x()`,
		`function outer() { let x = 1; let g = function x() { return 2 }; return x }`,
	}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			err := compileErr(t, source)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), `"x" already exists`)
		})
	}
}

func TestNamedFunctionDeclarations(t *testing.T) {
	tests := []string{
		// Top-level declarations allow forward references.
		`function a() { return b() }; function b() { return 1 }; a()`,
		// A nested function may shadow a name from an outer scope.
		`const x = 1; function outer() { function x() { return 2 }; return x() }`,
		`const x = 1; if (true) { function x() { return 2 }; x() }`,
		`function outer() { function x() { return 2 }; return x() }`,
		`let g = function x() { return 2 }; g()`,
	}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			assert.Nil(t, compileErr(t, source))
		})
	}
}
