package syntax

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
)

func parse(t *testing.T, source string) *ast.Program {
	t.Helper()
	program, err := parser.Parse(context.Background(), source, nil)
	assert.Nil(t, err)
	return program
}

func TestSyntaxValidator_DisallowVariableDecl(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"1 + 2", false},
		{"let x = 1", true},
		{"const x = 1", true},
		{"let x, y = [1, 2]", true},
	}

	config := SyntaxConfig{DisallowVariableDecl: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowAssignment(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"x", false},
		{"x = 1", true},
		{"x += 1", true},
		{"x++", true},
		{"obj.attr = 1", true},
	}

	config := SyntaxConfig{DisallowAssignment: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowReturn(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"1 + 2", false},
		{"function foo() { return 1 }", true},
	}

	config := SyntaxConfig{DisallowReturn: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowFuncDef(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"foo()", false},
		{"function foo() { 1 }", true},
		{"x => x + 1", true},
		{"(x, y) => x + y", true},
	}

	config := SyntaxConfig{DisallowFuncDef: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowFuncCall(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"1 + 2", false},
		{"foo()", true},
		{"obj.method()", true},
	}

	config := SyntaxConfig{DisallowFuncCall: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowTryCatch(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"1 + 2", false},
		{"try { 1 } catch { 2 }", true},
		{"throw error(\"oops\")", true},
	}

	config := SyntaxConfig{DisallowTryCatch: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowIf(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"1 + 2", false},
		{"if (true) { 1 }", true},
		{"if (x) { 1 } else { 2 }", true},
	}

	config := SyntaxConfig{DisallowIf: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowSwitch(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"1 + 2", false},
		{"switch (x) { case 1: 1 }", true},
	}

	config := SyntaxConfig{DisallowSwitch: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowDestructure(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"1 + 2", false},
		{"let {a, b} = obj", true},
		{"let [x, y] = arr", true},
		{"function foo({a, b}) { a + b }", true},
		{"function foo([x, y]) { x + y }", true},
	}

	config := SyntaxConfig{DisallowDestructure: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowSpread(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"[1, 2, 3]", false},
		{"[...arr]", true},
		{"{...obj}", true},
		{"foo(...args)", true},
	}

	config := SyntaxConfig{DisallowSpread: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowPipe(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{"foo(x)", false},
		{"x |> foo", true},
		{"x |> foo |> bar", true},
	}

	config := SyntaxConfig{DisallowPipe: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestSyntaxValidator_DisallowTemplates(t *testing.T) {
	tests := []struct {
		source  string
		wantErr bool
	}{
		{`"hello"`, false},
		{"`hello ${name}`", true},
	}

	config := SyntaxConfig{DisallowTemplates: true}
	validator := NewSyntaxValidator(config)

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			program := parse(t, tt.source)
			errs := validator.Validate(program)
			if tt.wantErr {
				assert.True(t, len(errs) > 0, "expected error for: %s", tt.source)
			} else {
				assert.Equal(t, len(errs), 0, "unexpected error for: %s", tt.source)
			}
		})
	}
}

func TestCombinedFlags(t *testing.T) {
	t.Run("multiple flags set", func(t *testing.T) {
		config := SyntaxConfig{
			DisallowVariableDecl: true,
			DisallowFuncDef:      true,
			DisallowIf:           true,
		}
		validator := NewSyntaxValidator(config)

		// Should error on variable declaration
		program := parse(t, "let x = 1")
		errs := validator.Validate(program)
		assert.True(t, len(errs) > 0)

		// Should error on function definition
		program = parse(t, "function foo() { 1 }")
		errs = validator.Validate(program)
		assert.True(t, len(errs) > 0)

		// Should error on if
		program = parse(t, "if (true) { 1 }")
		errs = validator.Validate(program)
		assert.True(t, len(errs) > 0)

		// Should pass on simple expression
		program = parse(t, "1 + 2")
		errs = validator.Validate(program)
		assert.Equal(t, len(errs), 0)
	})

	t.Run("multiple violations in one program", func(t *testing.T) {
		config := SyntaxConfig{
			DisallowVariableDecl: true,
			DisallowIf:           true,
		}
		validator := NewSyntaxValidator(config)

		source := `
let x = 1
if (true) { 2 }
let y = 3
`
		program := parse(t, source)
		errs := validator.Validate(program)

		// Should have 3 errors: 2 variable declarations + 1 if
		assert.Equal(t, len(errs), 3)
	})
}

func TestNestedConstructs(t *testing.T) {
	t.Run("function inside function", func(t *testing.T) {
		config := SyntaxConfig{DisallowFuncDef: true}
		validator := NewSyntaxValidator(config)

		source := `function outer() { function inner() { 1 } }`
		program := parse(t, source)
		errs := validator.Validate(program)

		// Both outer and inner functions should be caught
		assert.Equal(t, len(errs), 2)
	})

	t.Run("if inside try", func(t *testing.T) {
		config := SyntaxConfig{
			DisallowTryCatch: true,
			DisallowIf:       true,
		}
		validator := NewSyntaxValidator(config)

		source := `try { if (true) { 1 } } catch { 2 }`
		program := parse(t, source)
		errs := validator.Validate(program)

		// Should catch both try and if
		assert.Equal(t, len(errs), 2)
	})

	t.Run("spread in nested list", func(t *testing.T) {
		config := SyntaxConfig{DisallowSpread: true}
		validator := NewSyntaxValidator(config)

		source := `[[...a], [...b]]`
		program := parse(t, source)
		errs := validator.Validate(program)

		// Both spreads should be caught
		assert.Equal(t, len(errs), 2)
	})

	t.Run("variable decl in function body", func(t *testing.T) {
		config := SyntaxConfig{DisallowVariableDecl: true}
		validator := NewSyntaxValidator(config)

		source := `function foo() { let x = 1; let y = 2 }`
		program := parse(t, source)
		errs := validator.Validate(program)

		// Both variable declarations should be caught
		assert.Equal(t, len(errs), 2)
	})

	t.Run("template with multiple interpolations", func(t *testing.T) {
		config := SyntaxConfig{DisallowTemplates: true}
		validator := NewSyntaxValidator(config)

		source := "`hello ${x} and ${y}`"
		program := parse(t, source)
		errs := validator.Validate(program)

		// Single template with multiple interpolations should be caught once
		assert.Equal(t, len(errs), 1)
	})
}

func TestZeroValueConfig(t *testing.T) {
	// Zero value SyntaxConfig should allow everything
	var config SyntaxConfig
	validator := NewSyntaxValidator(config)

	sources := []string{
		"let x = 1",
		"x = 2",
		"function foo() { return 1 }",
		"if (true) { 1 }",
		"switch (x) { case 1: 1 }",
		"try { 1 } catch { 2 }",
		"let {a} = obj",
		"[...arr]",
		"x |> foo",
		"`hello ${name}`",
	}

	for _, source := range sources {
		program := parse(t, source)
		errs := validator.Validate(program)
		assert.Equal(t, len(errs), 0, "unexpected error for: %s", source)
	}
}
