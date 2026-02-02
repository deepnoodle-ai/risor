package syntax

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
)

func TestEdgeCases(t *testing.T) {
	t.Run("empty program", func(t *testing.T) {
		// Empty source should parse but have no statements
		program, err := parser.Parse(context.Background(), "", nil)
		assert.Nil(t, err)

		config := SyntaxConfig{DisallowVariableDecl: true}
		validator := NewSyntaxValidator(config)
		errs := validator.Validate(program)

		assert.Equal(t, len(errs), 0)
	})

	t.Run("all assignment operators", func(t *testing.T) {
		config := SyntaxConfig{DisallowAssignment: true}
		validator := NewSyntaxValidator(config)

		operators := []string{"=", "+=", "-=", "*=", "/="}
		for _, op := range operators {
			source := "x " + op + " 1"
			program := parse(t, source)
			errs := validator.Validate(program)
			assert.True(t, len(errs) > 0, "expected error for: %s", source)
		}
	})

	t.Run("postfix operators", func(t *testing.T) {
		config := SyntaxConfig{DisallowAssignment: true}
		validator := NewSyntaxValidator(config)

		for _, source := range []string{"x++", "x--"} {
			program := parse(t, source)
			errs := validator.Validate(program)
			assert.True(t, len(errs) > 0, "expected error for: %s", source)
		}
	})

	t.Run("index assignment", func(t *testing.T) {
		config := SyntaxConfig{DisallowAssignment: true}
		validator := NewSyntaxValidator(config)

		source := "arr[0] = 1"
		program := parse(t, source)
		errs := validator.Validate(program)
		assert.True(t, len(errs) > 0)
	})

	t.Run("template string without interpolation", func(t *testing.T) {
		config := SyntaxConfig{DisallowTemplates: true}
		validator := NewSyntaxValidator(config)

		// Plain backtick string without ${} should be allowed
		// (it's just a string literal, not a template)
		source := "`hello world`"
		program := parse(t, source)
		errs := validator.Validate(program)
		assert.Equal(t, len(errs), 0)
	})

	t.Run("arrow function with destructure param", func(t *testing.T) {
		config := SyntaxConfig{DisallowDestructure: true}
		validator := NewSyntaxValidator(config)

		source := "({a, b}) => a + b"
		program := parse(t, source)
		errs := validator.Validate(program)
		assert.True(t, len(errs) > 0)
	})

	t.Run("try without catch", func(t *testing.T) {
		config := SyntaxConfig{DisallowTryCatch: true}
		validator := NewSyntaxValidator(config)

		source := "try { 1 } finally { 2 }"
		program := parse(t, source)
		errs := validator.Validate(program)
		assert.True(t, len(errs) > 0)
	})

	t.Run("switch with default only", func(t *testing.T) {
		config := SyntaxConfig{DisallowSwitch: true}
		validator := NewSyntaxValidator(config)

		source := "switch (x) { default: 1 }"
		program := parse(t, source)
		errs := validator.Validate(program)
		assert.True(t, len(errs) > 0)
	})

	t.Run("chained method calls", func(t *testing.T) {
		config := SyntaxConfig{DisallowFuncCall: true}
		validator := NewSyntaxValidator(config)

		source := "obj.method1().method2().method3()"
		program := parse(t, source)
		errs := validator.Validate(program)
		// Should catch all 3 method calls
		assert.True(t, len(errs) >= 3)
	})

	t.Run("optional chaining method call", func(t *testing.T) {
		config := SyntaxConfig{DisallowFuncCall: true}
		validator := NewSyntaxValidator(config)

		source := "obj?.method()"
		program := parse(t, source)
		errs := validator.Validate(program)
		assert.True(t, len(errs) > 0)
	})

	t.Run("return without value", func(t *testing.T) {
		config := SyntaxConfig{DisallowReturn: true}
		validator := NewSyntaxValidator(config)

		source := "function foo() { return }"
		program := parse(t, source)
		errs := validator.Validate(program)
		assert.True(t, len(errs) > 0)
	})

	t.Run("multiple return statements", func(t *testing.T) {
		config := SyntaxConfig{DisallowReturn: true}
		validator := NewSyntaxValidator(config)

		source := "function foo() { if (x) { return 1 } else { return 2 } }"
		program := parse(t, source)
		errs := validator.Validate(program)
		assert.Equal(t, len(errs), 2)
	})

	t.Run("pipe with multiple stages", func(t *testing.T) {
		config := SyntaxConfig{DisallowPipe: true}
		validator := NewSyntaxValidator(config)

		source := "x |> foo |> bar |> baz"
		program := parse(t, source)
		errs := validator.Validate(program)
		// Even though there's one pipe expression, it should be caught once
		assert.True(t, len(errs) >= 1)
	})
}
