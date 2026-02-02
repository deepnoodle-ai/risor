package object

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestNewBuiltin(t *testing.T) {
	fn := func(ctx context.Context, args ...Object) (Object, error) {
		return NewInt(42), nil
	}

	b := NewBuiltin("test", fn)
	assert.Equal(t, b.Name(), "test")
	assert.Equal(t, b.Key(), "test")
	assert.Equal(t, b.Inspect(), "builtin(test)")
}

func TestBuiltinInModule(t *testing.T) {
	fn := func(ctx context.Context, args ...Object) (Object, error) {
		return NewInt(42), nil
	}

	b := NewBuiltin("sqrt", fn).InModule("math")
	assert.Equal(t, b.Name(), "sqrt")
	assert.Equal(t, b.Key(), "math.sqrt")
}

func TestBuiltinWithModule(t *testing.T) {
	fn := func(ctx context.Context, args ...Object) (Object, error) {
		return NewInt(42), nil
	}

	mod := NewBuiltinsModule("mymod", nil)
	b := NewBuiltin("func1", fn).WithModule(mod)
	assert.Equal(t, b.Name(), "func1")
	assert.Equal(t, b.Key(), "mymod.func1")
	assert.Equal(t, b.Inspect(), "builtin(mymod.func1)")
}

func TestBuiltinWithNilModule(t *testing.T) {
	fn := func(ctx context.Context, args ...Object) (Object, error) {
		return NewInt(42), nil
	}

	b := NewBuiltin("test", fn).WithModule(nil)
	assert.Equal(t, b.Name(), "test")
	assert.Equal(t, b.Key(), "test")
}

func TestNewNoopBuiltin(t *testing.T) {
	b := NewNoopBuiltin("noop")
	assert.Equal(t, b.Name(), "noop")

	result, err := b.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)
}
