package syntax

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
)

func TestMultipleErrors(t *testing.T) {
	source := `
let x = 1
let y = 2
let z = 3
`
	config := SyntaxConfig{DisallowVariableDecl: true}
	validator := NewSyntaxValidator(config)

	program := parse(t, source)
	errs := validator.Validate(program)

	assert.Equal(t, len(errs), 3)
}

func TestValidationErrorPosition(t *testing.T) {
	source := "let x = 1"
	config := SyntaxConfig{DisallowVariableDecl: true}
	validator := NewSyntaxValidator(config)

	program := parse(t, source)
	errs := validator.Validate(program)

	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Position.LineNumber(), 1)
	assert.Equal(t, errs[0].Position.ColumnNumber(), 1)
	assert.Equal(t, errs[0].Message, "variable declarations are not allowed")
}

func TestValidationErrorsWrapper(t *testing.T) {
	errs := []ValidationError{
		{Message: "error 1"},
		{Message: "error 2"},
	}

	wrapper := NewValidationErrors(errs)
	errStr := wrapper.Error()
	assert.True(t, strings.Contains(errStr, "2 validation errors"))
	assert.True(t, strings.Contains(errStr, "error 1"))
	assert.True(t, strings.Contains(errStr, "error 2"))

	// Test Unwrap
	var firstErr *ValidationError
	assert.True(t, errors.As(wrapper.Unwrap(), &firstErr))
	assert.Equal(t, firstErr.Message, "error 1")
}

func TestValidationErrorsSingleError(t *testing.T) {
	errs := []ValidationError{
		{Message: "single error"},
	}

	wrapper := NewValidationErrors(errs)
	assert.Equal(t, wrapper.Error(), "single error at line 1, column 1")
}

func TestValidationErrorsEmptySlice(t *testing.T) {
	wrapper := NewValidationErrors([]ValidationError{})
	assert.Equal(t, wrapper.Error(), "no validation errors")
	assert.Nil(t, wrapper.Unwrap())
}

func TestValidationErrorFormat(t *testing.T) {
	t.Run("error with filename", func(t *testing.T) {
		program, err := parser.Parse(context.Background(), "let x = 1", &parser.Config{
			Filename: "test.risor",
		})
		assert.Nil(t, err)

		config := SyntaxConfig{DisallowVariableDecl: true}
		validator := NewSyntaxValidator(config)
		errs := validator.Validate(program)

		assert.Equal(t, len(errs), 1)
		errStr := errs[0].Error()
		assert.True(t, strings.Contains(errStr, "test.risor"))
		assert.True(t, strings.Contains(errStr, "1:1")) // line:column
	})

	t.Run("error without filename", func(t *testing.T) {
		program := parse(t, "let x = 1")

		config := SyntaxConfig{DisallowVariableDecl: true}
		validator := NewSyntaxValidator(config)
		errs := validator.Validate(program)

		assert.Equal(t, len(errs), 1)
		errStr := errs[0].Error()
		assert.True(t, strings.Contains(errStr, "line 1"))
		assert.True(t, strings.Contains(errStr, "column 1"))
	})

	t.Run("multiple errors format", func(t *testing.T) {
		source := "let x = 1; let y = 2"
		program := parse(t, source)

		config := SyntaxConfig{DisallowVariableDecl: true}
		validator := NewSyntaxValidator(config)
		errs := validator.Validate(program)

		wrapper := NewValidationErrors(errs)
		errStr := wrapper.Error()
		assert.True(t, strings.Contains(errStr, "2 validation errors"))
	})
}

func TestValidationErrorNode(t *testing.T) {
	t.Run("error contains correct node", func(t *testing.T) {
		source := "let x = 1"
		program := parse(t, source)

		config := SyntaxConfig{DisallowVariableDecl: true}
		validator := NewSyntaxValidator(config)
		errs := validator.Validate(program)

		assert.Equal(t, len(errs), 1)
		assert.NotNil(t, errs[0].Node)

		// The node should be a Var
		_, isVar := errs[0].Node.(*ast.Var)
		assert.True(t, isVar)
	})

	t.Run("error node position matches", func(t *testing.T) {
		source := "  let x = 1" // 2 spaces before let
		program := parse(t, source)

		config := SyntaxConfig{DisallowVariableDecl: true}
		validator := NewSyntaxValidator(config)
		errs := validator.Validate(program)

		assert.Equal(t, len(errs), 1)
		// Column should be 3 (1-indexed, after 2 spaces)
		assert.Equal(t, errs[0].Position.ColumnNumber(), 3)
	})
}
