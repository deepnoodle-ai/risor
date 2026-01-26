package object

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/compiler"
)

func TestContextCallFunc(t *testing.T) {
	callFunc, ok := GetCallFunc(context.Background())
	assert.False(t, ok)
	assert.Nil(t, callFunc)

	ctx := WithCallFunc(context.Background(),
		func(ctx context.Context, fn *Function, args []Object) (Object, error) {
			return NewInt(42), nil
		})
	callFunc, ok = GetCallFunc(ctx)
	assert.True(t, ok)
	assert.NotNil(t, callFunc)

	result, err := callFunc(context.Background(),
		NewFunction(compiler.NewFunction(compiler.FunctionOpts{})),
		[]Object{})
	assert.Nil(t, err)
	assert.Equal(t, result, NewInt(42))
}
