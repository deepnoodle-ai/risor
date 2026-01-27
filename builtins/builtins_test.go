package builtins

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
)

func assertObjectEqual(t *testing.T, got, want object.Object) {
	t.Helper()
	assert.True(t, object.Equals(got, want), "got %s, want %s", got.Inspect(), want.Inspect())
}

func TestBuiltins(t *testing.T) {
	m := Builtins()
	count := len(m)
	assert.Greater(t, count, 22) // Reduced after removing try(), error(), set(), buffer() builtins
}

type testCase struct {
	input    object.Object
	expected object.Object
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
			assertObjectEqual(t, result, tt.expected)
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
	callFn := func(ctx context.Context, fn *object.Closure, args []object.Object) (object.Object, error) {
		assert.Len(t, args, 2)
		a := args[0].(*object.Int).Value()
		b := args[1].(*object.Int).Value()
		return object.NewBool(b < a), nil // descending order
	}
	ctx = object.WithCallFunc(ctx, callFn)

	// This sort function isn't actually used here in the test. This value
	// will be passed to callFn but we don't use it.
	var sortFn *object.Closure

	// Confirm Sorted returns the expected sorted list
	result := Sorted(ctx, input, sortFn)
	assert.Equal(t,

		result, object.NewList([]object.Object{
			object.NewInt(99),
			object.NewInt(3),
			object.NewInt(2),
			object.NewInt(1),
			object.NewInt(0),
		}))
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
			assertObjectEqual(t, result, tt.expected)
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
			assertObjectEqual(t, result, tt.expected)
		})
	}
}
