package object

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestContextCallFunc(t *testing.T) {
	callFunc, ok := GetCallFunc(context.Background())
	assert.False(t, ok)
	assert.Nil(t, callFunc)

	ctx := WithCallFunc(context.Background(),
		func(ctx context.Context, fn *Closure, args []Object) (Object, error) {
			return NewInt(42), nil
		})
	callFunc, ok = GetCallFunc(ctx)
	assert.True(t, ok)
	assert.NotNil(t, callFunc)

	result, err := callFunc(context.Background(),
		NewClosure(bytecode.NewFunction(bytecode.FunctionParams{})),
		[]Object{})
	assert.Nil(t, err)
	assert.Equal(t, result, NewInt(42))
}
