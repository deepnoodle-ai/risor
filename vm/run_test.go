package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
)

func TestRun(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "1 + 1")
	assert.Nil(t, err)
	code, err := compiler.Compile(ast)
	assert.Nil(t, err)
	result, err := Run(ctx, code)
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.Int).Value(), int64(2))
}

func TestRunEmpty(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "")
	assert.Nil(t, err)
	code, err := compiler.Compile(ast)
	assert.Nil(t, err)
	result, err := Run(ctx, code)
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)
}

func TestRunError(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "let foo = 42; foo.bar")
	assert.Nil(t, err)
	code, err := compiler.Compile(ast)
	assert.Nil(t, err)
	_, err = Run(ctx, code)
	assert.NotNil(t, err)
	// Check that the error message contains the expected content
	assert.Contains(t, err.Error(), "type error: attribute \"bar\" not found on int object")
	// Check that it's a structured error with location info
	structuredErr, ok := err.(*errz.StructuredError)
	assert.True(t, ok)
	assert.Equal(t, structuredErr.Kind, errz.ErrType)
}
