package syntax

import (
	"errors"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/ast"
)

func TestTransformerFunc(t *testing.T) {
	// Test that TransformerFunc adapter works correctly
	called := false
	transformer := TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
		called = true
		return p, nil
	})

	program := parse(t, "1 + 2")
	result, err := transformer.Transform(program)

	assert.Nil(t, err)
	assert.True(t, called)
	assert.Equal(t, result, program)
}

func TestTransformerReturnsError(t *testing.T) {
	transformer := TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
		return nil, errors.New("transform failed")
	})

	program := parse(t, "1 + 2")
	_, err := transformer.Transform(program)

	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "transform failed")
}

func TestTransformerModifiesAST(t *testing.T) {
	// Transformer that doubles integer literals
	transformer := TransformerFunc(func(p *ast.Program) (*ast.Program, error) {
		for node := range ast.Preorder(p) {
			if intNode, ok := node.(*ast.Int); ok {
				intNode.Value *= 2
			}
		}
		return p, nil
	})

	program := parse(t, "5")
	result, err := transformer.Transform(program)

	assert.Nil(t, err)
	// Check that the integer was doubled
	intNode := result.Stmts[0].(*ast.Int)
	assert.Equal(t, intNode.Value, int64(10))
}

func TestValidatorFunc(t *testing.T) {
	// Test that ValidatorFunc adapter works correctly
	called := false
	validator := ValidatorFunc(func(p *ast.Program) []ValidationError {
		called = true
		return nil
	})

	program := parse(t, "1 + 2")
	errs := validator.Validate(program)

	assert.True(t, called)
	assert.Equal(t, len(errs), 0)
}

func TestValidatorFuncReturnsErrors(t *testing.T) {
	validator := ValidatorFunc(func(p *ast.Program) []ValidationError {
		return []ValidationError{
			{Message: "custom error 1"},
			{Message: "custom error 2"},
		}
	})

	program := parse(t, "1 + 2")
	errs := validator.Validate(program)

	assert.Equal(t, len(errs), 2)
	assert.Equal(t, errs[0].Message, "custom error 1")
	assert.Equal(t, errs[1].Message, "custom error 2")
}

func TestValidatorFuncWithNodeInspection(t *testing.T) {
	// Validator that disallows access to "secret" identifier
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

	t.Run("allows normal identifiers", func(t *testing.T) {
		program := parse(t, "x + y")
		errs := noSecrets.Validate(program)
		assert.Equal(t, len(errs), 0)
	})

	t.Run("catches secret identifier", func(t *testing.T) {
		program := parse(t, "secret + 1")
		errs := noSecrets.Validate(program)
		assert.Equal(t, len(errs), 1)
		assert.Equal(t, errs[0].Message, "access to 'secret' is not allowed")
	})

	t.Run("catches multiple secret usages", func(t *testing.T) {
		program := parse(t, "secret + secret")
		errs := noSecrets.Validate(program)
		assert.Equal(t, len(errs), 2)
	})
}
