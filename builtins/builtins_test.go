package builtins

import (
	"context"
	"testing"

	"github.com/risor-io/risor/object"
	"github.com/stretchr/testify/require"
)

func TestBuiltins(t *testing.T) {
	m := Builtins()
	count := len(m)
	require.Greater(t, count, 30)
}

type testCase struct {
	input    object.Object
	expected object.Object
}

func TestMake(t *testing.T) {
	ctx := context.Background()

	tests := []testCase{
		{object.NewBuiltin("list", nil), object.NewList([]object.Object{})},
		{object.NewBuiltin("map", nil), object.NewMap(map[string]object.Object{})},
		{object.NewBuiltin("set", nil), object.NewSet([]object.Object{})},

		{object.NewList([]object.Object{
			object.NewString("ignored"),
		}), object.NewList([]object.Object{})},

		{object.NewMap(map[string]object.Object{
			"ignored": object.NewString("ignored"),
		}), object.NewMap(map[string]object.Object{})},

		{object.NewSet([]object.Object{
			object.NewString("ignored"),
		}), object.NewSet([]object.Object{})},
	}

	for _, tt := range tests {
		t.Run(tt.input.Inspect(), func(t *testing.T) {
			result := Make(ctx, tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSorted(t *testing.T) {
	ctx := context.Background()
	tests := []testCase{
		{
			object.NewList([]object.Object{
				object.NewInt(3),
				object.NewInt(1),
				object.NewInt(2),
			}),
			object.NewList([]object.Object{
				object.NewInt(1),
				object.NewInt(2),
				object.NewInt(3),
			}),
		},
		{
			object.NewList([]object.Object{
				object.NewInt(3),
				object.NewInt(1),
				object.NewString("nope"),
			}),
			object.TypeErrorf("type error: unable to compare string and int"),
		},
		{
			object.NewList([]object.Object{
				object.NewString("b"),
				object.NewString("c"),
				object.NewString("a"),
			}),
			object.NewList([]object.Object{
				object.NewString("a"),
				object.NewString("b"),
				object.NewString("c"),
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.input.Inspect(), func(t *testing.T) {
			result := Sorted(ctx, tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSortedWithFunc(t *testing.T) {
	ctx := context.Background()
	// We'll sort this list of integers
	input := object.NewList([]object.Object{
		object.NewInt(3),
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(99),
		object.NewInt(0),
	})
	// This function will be called for each comparison
	callFn := func(ctx context.Context, fn *object.Function, args []object.Object) (object.Object, error) {
		require.Len(t, args, 2)
		a := args[0].(*object.Int).Value()
		b := args[1].(*object.Int).Value()
		return object.NewBool(b < a), nil // descending order
	}
	ctx = object.WithCallFunc(ctx, callFn)

	// This sort function isn't actually used here in the test. This value
	// will be passed to callFn but we don't use it.
	var sortFn *object.Function

	// Confirm Sorted returns the expected sorted list
	result := Sorted(ctx, input, sortFn)
	require.Equal(t, object.NewList([]object.Object{
		object.NewInt(99),
		object.NewInt(3),
		object.NewInt(2),
		object.NewInt(1),
		object.NewInt(0),
	}), result)
}

func TestCoalesce(t *testing.T) {
	ctx := context.Background()
	tests := []testCase{
		{
			object.NewList([]object.Object{
				object.NewInt(3),
				object.NewInt(1),
				object.NewInt(2),
			}),
			object.NewInt(3),
		},
		{
			object.NewList([]object.Object{
				object.Nil,
				object.Nil,
				object.NewString("yup"),
			}),
			object.NewString("yup"),
		},
		{
			object.NewList([]object.Object{}),
			object.Nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.input.Inspect(), func(t *testing.T) {
			result := Coalesce(ctx, tt.input.(*object.List).Value()...)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestChunk(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		input    object.Object
		size     int64
		expected object.Object
	}{
		{
			object.NewList([]object.Object{
				object.NewInt(1),
				object.NewInt(2),
				object.NewInt(3),
			}),
			2,
			object.NewList([]object.Object{
				object.NewList([]object.Object{
					object.NewInt(1),
					object.NewInt(2),
				}),
				object.NewList([]object.Object{
					object.NewInt(3),
				}),
			}),
		},
		{
			object.NewList([]object.Object{
				object.NewString("a"),
				object.NewString("b"),
				object.NewString("c"),
				object.NewString("d"),
			}),
			2,
			object.NewList([]object.Object{
				object.NewList([]object.Object{
					object.NewString("a"),
					object.NewString("b"),
				}),
				object.NewList([]object.Object{
					object.NewString("c"),
					object.NewString("d"),
				}),
			}),
		},
		{
			object.NewString("wrong"),
			2,
			object.TypeErrorf("type error: chunk() expected a list (string given)"),
		},
		{
			object.NewList([]object.Object{}),
			-1,
			object.Errorf("value error: chunk() size must be > 0 (-1 given)"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.input.Inspect(), func(t *testing.T) {
			result := Chunk(ctx, tt.input, object.NewInt(tt.size))
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestTry(t *testing.T) {
	okFunc := object.NewBuiltin("ok",
		func(ctx context.Context, args ...object.Object) object.Object {
			return object.NewString("ok")
		})

	errFunc := object.NewBuiltin("err",
		func(ctx context.Context, args ...object.Object) object.Object {
			return object.Errorf("kaboom")
		})

	fatalFunc := object.NewBuiltin("fatal",
		func(ctx context.Context, args ...object.Object) object.Object {
			return object.EvalErrorf("fatal explosion")
		})

	ctx := context.Background()
	var result object.Object

	result = Try(ctx, okFunc)
	require.Equal(t, object.NewString("ok"), result)

	result = Try(ctx, errFunc)
	require.Equal(t, object.Nil, result)

	result = Try(ctx, errFunc, object.NewString("fallback"))
	require.Equal(t, object.NewString("fallback"), result)

	result = Try(ctx, errFunc, okFunc, errFunc)
	require.Equal(t, object.NewString("ok"), result)

	result = Try(ctx, errFunc, fatalFunc, okFunc)
	require.Equal(t, object.EvalErrorf("fatal explosion").WithRaised(true), result)
}
