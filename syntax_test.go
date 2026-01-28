package risor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/ast"
)

// =============================================================================
// PRESET INTEGRATION TESTS
// =============================================================================

func TestWithSyntaxExpressionOnly(t *testing.T) {
	ctx := context.Background()

	t.Run("allows expressions", func(t *testing.T) {
		result, err := Eval(ctx, "1 + 2", WithSyntax(ExpressionOnly))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})

	t.Run("allows variable access", func(t *testing.T) {
		result, err := Eval(ctx, "x * y",
			WithEnv(map[string]any{"x": int64(5), "y": int64(6)}),
			WithSyntax(ExpressionOnly))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(30))
	})

	t.Run("allows function calls", func(t *testing.T) {
		env := Builtins()
		result, err := Eval(ctx, "len([1, 2, 3])",
			WithEnv(env),
			WithSyntax(ExpressionOnly))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})

	t.Run("disallows variable declarations", func(t *testing.T) {
		_, err := Eval(ctx, "let x = 1", WithSyntax(ExpressionOnly))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "variable declarations are not allowed"))
	})

	t.Run("disallows function definitions", func(t *testing.T) {
		_, err := Eval(ctx, "function foo() { 1 }", WithSyntax(ExpressionOnly))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "function definitions are not allowed"))
	})

	t.Run("disallows if expressions", func(t *testing.T) {
		_, err := Eval(ctx, "if (true) { 1 }", WithSyntax(ExpressionOnly))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "if expressions are not allowed"))
	})

	t.Run("disallows assignment", func(t *testing.T) {
		_, err := Eval(ctx, "x = 1",
			WithEnv(map[string]any{"x": int64(0)}),
			WithSyntax(ExpressionOnly))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "assignment is not allowed"))
	})
}

func TestWithSyntaxBasicScripting(t *testing.T) {
	ctx := context.Background()

	t.Run("allows variable declarations", func(t *testing.T) {
		result, err := Eval(ctx, "let x = 1; x + 2", WithSyntax(BasicScripting))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})

	t.Run("allows if expressions", func(t *testing.T) {
		result, err := Eval(ctx, "let x = 5; if (x > 3) { 10 } else { 0 }", WithSyntax(BasicScripting))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(10))
	})

	t.Run("allows try/catch", func(t *testing.T) {
		result, err := Eval(ctx, "try { 42 } catch { 0 }", WithSyntax(BasicScripting))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(42))
	})

	t.Run("allows switch", func(t *testing.T) {
		result, err := Eval(ctx, "let x = 2; switch (x) { case 1: 10 case 2: 20 default: 0 }", WithSyntax(BasicScripting))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(20))
	})

	t.Run("allows destructuring", func(t *testing.T) {
		result, err := Eval(ctx, "let {a, b} = {a: 1, b: 2}; a + b", WithSyntax(BasicScripting))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})

	t.Run("allows spread", func(t *testing.T) {
		result, err := Eval(ctx, "let arr = [1, 2]; [...arr, 3]", WithSyntax(BasicScripting))
		assert.Nil(t, err)
		assert.Equal(t, result, []any{int64(1), int64(2), int64(3)})
	})

	t.Run("allows pipe", func(t *testing.T) {
		result, err := Eval(ctx, `[1, 2, 3] | len`,
			WithEnv(Builtins()),
			WithSyntax(BasicScripting))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})

	t.Run("disallows function definitions", func(t *testing.T) {
		_, err := Eval(ctx, "function foo() { 1 }", WithSyntax(BasicScripting))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "function definitions are not allowed"))
	})

	t.Run("disallows arrow functions", func(t *testing.T) {
		_, err := Eval(ctx, "x => x + 1", WithSyntax(BasicScripting))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "function definitions are not allowed"))
	})
}

func TestFullLanguageAllowsEverything(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		source string
		env    map[string]any
	}{
		{"1 + 2", nil},
		{"let x = 1; x", nil},
		{"y = 2", map[string]any{"y": int64(0)}},
		{"if (true) { 1 }", nil},
		{"function foo() { return 1 }; foo()", nil},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			opts := []Option{WithSyntax(FullLanguage)}
			if tt.env != nil {
				opts = append(opts, WithEnv(tt.env))
			}
			_, err := Eval(ctx, tt.source, opts...)
			assert.Nil(t, err)
		})
	}
}

// =============================================================================
// CUSTOM VALIDATOR TESTS
// =============================================================================

func TestWithCustomValidator(t *testing.T) {
	ctx := context.Background()

	noSecrets := ValidatorFunc(func(p *ast.Program) []ValidationError {
		var errs []ValidationError
		for node := range ast.Preorder(p) {
			if ident, ok := node.(*ast.Ident); ok && ident.Name == "secret" {
				errs = append(errs, ValidationError{
					Message:  "access to 'secret' is not allowed",
					Node:     node,
					Position: node.Pos(),
				})
			}
		}
		return errs
	})

	t.Run("allows normal access", func(t *testing.T) {
		result, err := Eval(ctx, "x + y",
			WithEnv(map[string]any{"x": int64(1), "y": int64(2)}),
			WithValidator(noSecrets))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})

	t.Run("disallows secret access", func(t *testing.T) {
		_, err := Eval(ctx, "secret + 1",
			WithEnv(map[string]any{"secret": int64(42)}),
			WithValidator(noSecrets))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "access to 'secret' is not allowed"))
	})
}

func TestMultipleValidators(t *testing.T) {
	ctx := context.Background()

	noFoo := ValidatorFunc(func(p *ast.Program) []ValidationError {
		var errs []ValidationError
		for node := range ast.Preorder(p) {
			if ident, ok := node.(*ast.Ident); ok && ident.Name == "foo" {
				errs = append(errs, ValidationError{
					Message:  "identifier 'foo' is not allowed",
					Node:     node,
					Position: node.Pos(),
				})
			}
		}
		return errs
	})

	noBar := ValidatorFunc(func(p *ast.Program) []ValidationError {
		var errs []ValidationError
		for node := range ast.Preorder(p) {
			if ident, ok := node.(*ast.Ident); ok && ident.Name == "bar" {
				errs = append(errs, ValidationError{
					Message:  "identifier 'bar' is not allowed",
					Node:     node,
					Position: node.Pos(),
				})
			}
		}
		return errs
	})

	t.Run("both validators pass", func(t *testing.T) {
		result, err := Eval(ctx, "x + y",
			WithEnv(map[string]any{"x": int64(1), "y": int64(2)}),
			WithValidator(noFoo),
			WithValidator(noBar))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})

	t.Run("first validator fails", func(t *testing.T) {
		_, err := Eval(ctx, "foo + 1",
			WithEnv(map[string]any{"foo": int64(1)}),
			WithValidator(noFoo),
			WithValidator(noBar))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "foo"))
	})

	t.Run("second validator fails", func(t *testing.T) {
		_, err := Eval(ctx, "bar + 1",
			WithEnv(map[string]any{"bar": int64(1)}),
			WithValidator(noFoo),
			WithValidator(noBar))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "bar"))
	})
}

func TestCustomValidatorWithPreset(t *testing.T) {
	ctx := context.Background()

	maxHundred := ValidatorFunc(func(p *ast.Program) []ValidationError {
		var errs []ValidationError
		for node := range ast.Preorder(p) {
			if intNode, ok := node.(*ast.Int); ok && intNode.Value > 100 {
				errs = append(errs, ValidationError{
					Message:  "integer values must not exceed 100",
					Node:     node,
					Position: node.Pos(),
				})
			}
		}
		return errs
	})

	t.Run("passes both validations", func(t *testing.T) {
		result, err := Eval(ctx, "50 + 30",
			WithSyntax(ExpressionOnly),
			WithValidator(maxHundred))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(80))
	})

	t.Run("fails preset validation", func(t *testing.T) {
		_, err := Eval(ctx, "let x = 50",
			WithSyntax(ExpressionOnly),
			WithValidator(maxHundred))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "variable declarations"))
	})

	t.Run("fails custom validation", func(t *testing.T) {
		_, err := Eval(ctx, "150 + 30",
			WithSyntax(ExpressionOnly),
			WithValidator(maxHundred))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "must not exceed 100"))
	})
}

// =============================================================================
// TRANSFORMER TESTS
// =============================================================================

func TestWithTransformer(t *testing.T) {
	ctx := context.Background()

	doubler := TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
		for node := range ast.Preorder(p) {
			if intNode, ok := node.(*ast.Int); ok {
				intNode.Value *= 2
			}
		}
		return p, nil
	})

	t.Run("transforms integers", func(t *testing.T) {
		result, err := Eval(ctx, "5 + 3", WithTransform(doubler))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(16)) // (5*2) + (3*2) = 16
	})
}

func TestMultipleTransformers(t *testing.T) {
	ctx := context.Background()

	doubler := TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
		for node := range ast.Preorder(p) {
			if intNode, ok := node.(*ast.Int); ok {
				intNode.Value *= 2
			}
		}
		return p, nil
	})

	addOne := TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
		for node := range ast.Preorder(p) {
			if intNode, ok := node.(*ast.Int); ok {
				intNode.Value++
			}
		}
		return p, nil
	})

	t.Run("transformers chain in order", func(t *testing.T) {
		// Start with 5 -> double (10) -> add one (11)
		result, err := Eval(ctx, "5",
			WithTransform(doubler),
			WithTransform(addOne))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(11))
	})

	t.Run("reverse order gives different result", func(t *testing.T) {
		// Start with 5 -> add one (6) -> double (12)
		result, err := Eval(ctx, "5",
			WithTransform(addOne),
			WithTransform(doubler))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(12))
	})
}

func TestTransformerError(t *testing.T) {
	ctx := context.Background()

	failingTransformer := TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
		return nil, errors.New("transformation failed")
	})

	_, err := Eval(ctx, "1 + 2", WithTransform(failingTransformer))
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "transformation failed"))
}

// =============================================================================
// VALIDATION ORDER TESTS
// =============================================================================

func TestValidationRunsBeforeTransformation(t *testing.T) {
	ctx := context.Background()

	transformerCalled := false
	transformer := TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
		transformerCalled = true
		return p, nil
	})

	_, err := Eval(ctx, "let x = 1",
		WithSyntax(ExpressionOnly),
		WithTransform(transformer))

	assert.NotNil(t, err)
	assert.False(t, transformerCalled, "transformer should not be called if validation fails")
}

func TestCombinedValidatorsAndTransformers(t *testing.T) {
	ctx := context.Background()

	noNegatives := ValidatorFunc(func(p *ast.Program) []ValidationError {
		var errs []ValidationError
		for node := range ast.Preorder(p) {
			if prefix, ok := node.(*ast.Prefix); ok {
				if prefix.Op == "-" {
					if _, isInt := prefix.X.(*ast.Int); isInt {
						errs = append(errs, ValidationError{
							Message:  "negative numbers are not allowed",
							Node:     node,
							Position: node.Pos(),
						})
					}
				}
			}
		}
		return errs
	})

	identity := TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
		return p, nil
	})

	t.Run("passes validation and transformation", func(t *testing.T) {
		result, err := Eval(ctx, "1 + 2",
			WithSyntax(ExpressionOnly),
			WithValidator(noNegatives),
			WithTransform(identity))
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})

	t.Run("fails syntax validation", func(t *testing.T) {
		_, err := Eval(ctx, "let x = 1",
			WithSyntax(ExpressionOnly),
			WithValidator(noNegatives))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "variable declarations"))
	})

	t.Run("fails custom validation", func(t *testing.T) {
		_, err := Eval(ctx, "-5 + 3",
			WithSyntax(ExpressionOnly),
			WithValidator(noNegatives))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "negative numbers"))
	})
}

// =============================================================================
// COMPILE API TESTS
// =============================================================================

func TestCompileWithSyntax(t *testing.T) {
	ctx := context.Background()

	t.Run("compile fails with validation error", func(t *testing.T) {
		_, err := Compile(ctx, "let x = 1", WithSyntax(ExpressionOnly))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "variable declarations"))
	})

	t.Run("compile succeeds with valid code", func(t *testing.T) {
		code, err := Compile(ctx, "1 + 2", WithSyntax(ExpressionOnly))
		assert.Nil(t, err)
		assert.NotNil(t, code)

		result, err := Run(ctx, code)
		assert.Nil(t, err)
		assert.Equal(t, result, int64(3))
	})
}

func TestSyntaxWithFilename(t *testing.T) {
	ctx := context.Background()

	_, err := Eval(ctx, "let x = 1",
		WithSyntax(ExpressionOnly),
		WithFilename("test.risor"))

	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "test.risor"))
}

// =============================================================================
// ERROR TYPE TESTS
// =============================================================================

func TestValidationErrorsType(t *testing.T) {
	ctx := context.Background()

	_, err := Eval(ctx, "let x = 1; let y = 2", WithSyntax(ExpressionOnly))
	assert.NotNil(t, err)

	var validationErrs *ValidationErrors
	assert.True(t, errors.As(err, &validationErrs))
	assert.True(t, len(validationErrs.Errors) >= 1)
}

func TestValidationErrorCount(t *testing.T) {
	ctx := context.Background()

	_, err := Eval(ctx, `
		let x = 1
		let y = 2
		let z = 3
	`, WithSyntax(ExpressionOnly))

	assert.NotNil(t, err)

	var validationErrs *ValidationErrors
	assert.True(t, errors.As(err, &validationErrs))
	assert.Equal(t, len(validationErrs.Errors), 3)
}

// =============================================================================
// EDGE CASE INTEGRATION TESTS
// =============================================================================

func TestExpressionOnlyWithMethodChaining(t *testing.T) {
	ctx := context.Background()

	result, err := Eval(ctx, `"hello".to_upper().to_lower()`,
		WithEnv(Builtins()),
		WithSyntax(ExpressionOnly))
	assert.Nil(t, err)
	assert.Equal(t, result, "hello")
}

func TestExpressionOnlyWithListOperations(t *testing.T) {
	ctx := context.Background()

	// Arrow functions are function definitions, so they should be blocked
	_, err := Eval(ctx, `[1, 2, 3].map(x => x * 2)`,
		WithEnv(Builtins()),
		WithSyntax(ExpressionOnly))
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "function definitions"))
}

func TestBasicScriptingLoops(t *testing.T) {
	ctx := context.Background()

	result, err := Eval(ctx, `
		let sum = 0
		let i = 1
		if (i <= 3) {
			sum = sum + i
			i = i + 1
			if (i <= 3) {
				sum = sum + i
				i = i + 1
				if (i <= 3) {
					sum = sum + i
				}
			}
		}
		sum
	`, WithSyntax(BasicScripting))
	assert.Nil(t, err)
	assert.Equal(t, result, int64(6)) // 1 + 2 + 3
}
