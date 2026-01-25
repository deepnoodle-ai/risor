package vm

import (
	"context"
	"testing"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "1 + 1")
	require.Nil(t, err)
	code, err := compiler.Compile(ast)
	require.Nil(t, err)
	result, err := Run(ctx, code)
	require.Nil(t, err)
	require.Equal(t, int64(2), result.(*object.Int).Value())
}

func TestRunEmpty(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "")
	require.Nil(t, err)
	code, err := compiler.Compile(ast)
	require.Nil(t, err)
	result, err := Run(ctx, code)
	require.Nil(t, err)
	require.Equal(t, object.Nil, result)
}

func TestRunError(t *testing.T) {
	ctx := context.Background()
	ast, err := parser.Parse(ctx, "let foo = 42; foo.bar")
	require.Nil(t, err)
	code, err := compiler.Compile(ast)
	require.Nil(t, err)
	_, err = Run(ctx, code)
	require.NotNil(t, err)
	require.Equal(t, "type error: attribute \"bar\" not found on int object", err.Error())
	errValue, ok := err.(*errz.TypeError)
	require.True(t, ok)
	require.Equal(t, "type error: attribute \"bar\" not found on int object", errValue.Error())
}
