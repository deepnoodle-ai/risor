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
	assert.Greater(t, count, 21) // Reduced after removing try(), error(), set(), buffer(), delete() builtins
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

func TestLen(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		input    object.Object
		expected object.Object
	}{
		{"list", object.NewList([]object.Object{object.NewInt(1), object.NewInt(2)}), object.NewInt(2)},
		{"string", object.NewString("hello"), object.NewInt(5)},
		{"empty list", object.NewList([]object.Object{}), object.NewInt(0)},
		{"empty string", object.NewString(""), object.NewInt(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Len(ctx, tt.input)
			assertObjectEqual(t, result, tt.expected)
		})
	}
}

func TestLenErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	result := Len(ctx)
	assert.True(t, object.IsError(result))

	// Unsupported type
	result = Len(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestSprintf(t *testing.T) {
	ctx := context.Background()
	result := Sprintf(ctx, object.NewString("hello %s"), object.NewString("world"))
	assertObjectEqual(t, result, object.NewString("hello world"))

	result = Sprintf(ctx, object.NewString("value: %d"), object.NewInt(42))
	assertObjectEqual(t, result, object.NewString("value: 42"))
}

func TestSprintfErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	result := Sprintf(ctx)
	assert.True(t, object.IsError(result))

	// Wrong type for format string
	result = Sprintf(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestList(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result := List(ctx)
	list, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, list.Value(), 0)

	// With size
	result = List(ctx, object.NewInt(3))
	list, ok = result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, list.Value(), 3)
	for _, v := range list.Value() {
		assert.Equal(t, v, object.Nil)
	}
}

func TestListErrors(t *testing.T) {
	ctx := context.Background()
	// Negative size
	result := List(ctx, object.NewInt(-1))
	assert.True(t, object.IsError(result))

	// Non-enumerable type
	result = List(ctx, object.NewFloat(3.14))
	assert.True(t, object.IsError(result))
}

func TestString(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result := String(ctx)
	assertObjectEqual(t, result, object.NewString(""))

	// String argument
	result = String(ctx, object.NewString("hello"))
	assertObjectEqual(t, result, object.NewString("hello"))

	// Int argument
	result = String(ctx, object.NewInt(42))
	assertObjectEqual(t, result, object.NewString("42"))
}

func TestStringErrors(t *testing.T) {
	ctx := context.Background()
	// Too many arguments
	result := String(ctx, object.NewString("a"), object.NewString("b"))
	assert.True(t, object.IsError(result))
}

func TestType(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		input    object.Object
		expected string
	}{
		{object.NewInt(42), "int"},
		{object.NewFloat(3.14), "float"},
		{object.NewString("hello"), "string"},
		{object.NewBool(true), "bool"},
		{object.Nil, "nil"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := Type(ctx, tt.input)
			assertObjectEqual(t, result, object.NewString(tt.expected))
		})
	}
}

func TestTypeErrors(t *testing.T) {
	ctx := context.Background()
	result := Type(ctx)
	assert.True(t, object.IsError(result))
}

func TestAssert(t *testing.T) {
	ctx := context.Background()

	// Truthy assertion
	result := Assert(ctx, object.True)
	assert.Equal(t, result, object.Nil)

	// With message
	result = Assert(ctx, object.True, object.NewString("should pass"))
	assert.Equal(t, result, object.Nil)

	// Failed assertion
	result = Assert(ctx, object.False)
	assert.True(t, object.IsError(result))

	// Failed assertion with message
	result = Assert(ctx, object.False, object.NewString("custom error"))
	assert.True(t, object.IsError(result))
}

func TestAssertErrors(t *testing.T) {
	ctx := context.Background()
	result := Assert(ctx)
	assert.True(t, object.IsError(result))
}

func TestAny(t *testing.T) {
	ctx := context.Background()

	// Some truthy
	result := Any(ctx, object.NewList([]object.Object{object.False, object.True, object.False}))
	assert.Equal(t, result, object.True)

	// All falsy
	result = Any(ctx, object.NewList([]object.Object{object.False, object.Nil}))
	assert.Equal(t, result, object.False)

	// Empty list
	result = Any(ctx, object.NewList([]object.Object{}))
	assert.Equal(t, result, object.False)
}

func TestAnyErrors(t *testing.T) {
	ctx := context.Background()
	result := Any(ctx)
	assert.True(t, object.IsError(result))
}

func TestAll(t *testing.T) {
	ctx := context.Background()

	// All truthy
	result := All(ctx, object.NewList([]object.Object{object.True, object.NewInt(1), object.NewString("yes")}))
	assert.Equal(t, result, object.True)

	// Some falsy
	result = All(ctx, object.NewList([]object.Object{object.True, object.False}))
	assert.Equal(t, result, object.False)

	// Empty list (vacuous truth)
	result = All(ctx, object.NewList([]object.Object{}))
	assert.Equal(t, result, object.True)
}

func TestAllErrors(t *testing.T) {
	ctx := context.Background()
	result := All(ctx)
	assert.True(t, object.IsError(result))
}

func TestBool(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result := Bool(ctx)
	assert.Equal(t, result, object.False)

	// Truthy values
	result = Bool(ctx, object.True)
	assert.Equal(t, result, object.True)

	result = Bool(ctx, object.NewInt(1))
	assert.Equal(t, result, object.True)

	// Falsy values
	result = Bool(ctx, object.False)
	assert.Equal(t, result, object.False)

	result = Bool(ctx, object.Nil)
	assert.Equal(t, result, object.False)
}

func TestBoolErrors(t *testing.T) {
	ctx := context.Background()
	result := Bool(ctx, object.NewInt(1), object.NewInt(2))
	assert.True(t, object.IsError(result))
}

func TestReversed(t *testing.T) {
	ctx := context.Background()

	// List
	result := Reversed(ctx, object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
	}))
	expected := object.NewList([]object.Object{
		object.NewInt(3),
		object.NewInt(2),
		object.NewInt(1),
	})
	assertObjectEqual(t, result, expected)

	// String
	result = Reversed(ctx, object.NewString("abc"))
	assertObjectEqual(t, result, object.NewString("cba"))
}

func TestReversedErrors(t *testing.T) {
	ctx := context.Background()
	result := Reversed(ctx)
	assert.True(t, object.IsError(result))

	result = Reversed(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestKeys(t *testing.T) {
	ctx := context.Background()

	// List keys are indices
	list := object.NewList([]object.Object{object.NewString("a"), object.NewString("b")})
	result := Keys(ctx, list)
	keyList, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, keyList.Value(), 2)
}

func TestKeysErrors(t *testing.T) {
	ctx := context.Background()
	result := Keys(ctx)
	assert.True(t, object.IsError(result))
}

func TestByte(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result := Byte(ctx)
	b, ok := result.(*object.Byte)
	assert.True(t, ok)
	assert.Equal(t, b.Value(), byte(0))

	// From int
	result = Byte(ctx, object.NewInt(65))
	b, ok = result.(*object.Byte)
	assert.True(t, ok)
	assert.Equal(t, b.Value(), byte(65))

	// From float
	result = Byte(ctx, object.NewFloat(66.5))
	b, ok = result.(*object.Byte)
	assert.True(t, ok)
	assert.Equal(t, b.Value(), byte(66))

	// From string
	result = Byte(ctx, object.NewString("42"))
	b, ok = result.(*object.Byte)
	assert.True(t, ok)
	assert.Equal(t, b.Value(), byte(42))
}

func TestByteErrors(t *testing.T) {
	ctx := context.Background()
	result := Byte(ctx, object.NewInt(1), object.NewInt(2))
	assert.True(t, object.IsError(result))

	result = Byte(ctx, object.NewString("invalid"))
	assert.True(t, object.IsError(result))

	result = Byte(ctx, object.NewList([]object.Object{}))
	assert.True(t, object.IsError(result))
}

func TestInt(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result := Int(ctx)
	i, ok := result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(0))

	// From int
	result = Int(ctx, object.NewInt(42))
	i, ok = result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(42))

	// From float
	result = Int(ctx, object.NewFloat(3.7))
	i, ok = result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(3))

	// From string
	result = Int(ctx, object.NewString("123"))
	i, ok = result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(123))

	// Hex string
	result = Int(ctx, object.NewString("0xff"))
	i, ok = result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(255))
}

func TestIntErrors(t *testing.T) {
	ctx := context.Background()
	result := Int(ctx, object.NewInt(1), object.NewInt(2))
	assert.True(t, object.IsError(result))

	result = Int(ctx, object.NewString("invalid"))
	assert.True(t, object.IsError(result))

	result = Int(ctx, object.NewList([]object.Object{}))
	assert.True(t, object.IsError(result))
}

func TestFloat(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result := Float(ctx)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), float64(0))

	// From int
	result = Float(ctx, object.NewInt(42))
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), float64(42))

	// From float
	result = Float(ctx, object.NewFloat(3.14))
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), float64(3.14))

	// From string
	result = Float(ctx, object.NewString("3.14"))
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), float64(3.14))
}

func TestFloatErrors(t *testing.T) {
	ctx := context.Background()
	result := Float(ctx, object.NewInt(1), object.NewInt(2))
	assert.True(t, object.IsError(result))

	result = Float(ctx, object.NewString("invalid"))
	assert.True(t, object.IsError(result))

	result = Float(ctx, object.NewList([]object.Object{}))
	assert.True(t, object.IsError(result))
}

func TestGetAttr(t *testing.T) {
	ctx := context.Background()

	// Existing attribute on list
	list := object.NewList([]object.Object{object.NewInt(1), object.NewInt(2)})
	result := GetAttr(ctx, list, object.NewString("append"))
	_, ok := result.(*object.Builtin)
	assert.True(t, ok)

	// With default value for missing attribute
	result = GetAttr(ctx, object.NewInt(42), object.NewString("missing"), object.NewString("default"))
	assertObjectEqual(t, result, object.NewString("default"))
}

func TestGetAttrErrors(t *testing.T) {
	ctx := context.Background()
	result := GetAttr(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))

	result = GetAttr(ctx, object.NewInt(42), object.NewInt(1))
	assert.True(t, object.IsError(result))

	// Missing attribute without default
	result = GetAttr(ctx, object.NewInt(42), object.NewString("missing"))
	assert.True(t, object.IsError(result))
}

func TestIsHashable(t *testing.T) {
	ctx := context.Background()

	// Hashable types
	result := IsHashable(ctx, object.NewString("hello"))
	assert.Equal(t, result, object.True)

	result = IsHashable(ctx, object.NewInt(42))
	assert.Equal(t, result, object.True)

	// Non-hashable types
	result = IsHashable(ctx, object.NewList([]object.Object{}))
	assert.Equal(t, result, object.False)
}

func TestIsHashableErrors(t *testing.T) {
	ctx := context.Background()
	result := IsHashable(ctx)
	assert.True(t, object.IsError(result))
}

func TestFilter(t *testing.T) {
	ctx := context.Background()

	// Create a simple builtin function that returns true for even numbers
	isEven := object.NewBuiltin("is_even", func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.False
		}
		if i, ok := args[0].(*object.Int); ok {
			return object.NewBool(i.Value()%2 == 0)
		}
		return object.False
	})

	list := object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
		object.NewInt(4),
	})

	result := Filter(ctx, list, isEven)
	expected := object.NewList([]object.Object{
		object.NewInt(2),
		object.NewInt(4),
	})
	assertObjectEqual(t, result, expected)
}

func TestFilterErrors(t *testing.T) {
	ctx := context.Background()
	result := Filter(ctx, object.NewList([]object.Object{}))
	assert.True(t, object.IsError(result))

	result = Filter(ctx, object.NewList([]object.Object{}), object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestCall(t *testing.T) {
	ctx := context.Background()

	// Call a builtin
	addOne := object.NewBuiltin("add_one", func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.Errorf("expected 1 argument")
		}
		if i, ok := args[0].(*object.Int); ok {
			return object.NewInt(i.Value() + 1)
		}
		return object.Errorf("expected int")
	})

	result := Call(ctx, addOne, object.NewInt(5))
	assertObjectEqual(t, result, object.NewInt(6))
}

func TestCallErrors(t *testing.T) {
	ctx := context.Background()
	result := Call(ctx)
	assert.True(t, object.IsError(result))

	result = Call(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestSortedMap(t *testing.T) {
	ctx := context.Background()
	m := object.NewMap(map[string]object.Object{
		"b": object.NewInt(2),
		"a": object.NewInt(1),
		"c": object.NewInt(3),
	})

	result := Sorted(ctx, m)
	list, ok := result.(*object.List)
	assert.True(t, ok)
	// Keys should be sorted
	assert.Len(t, list.Value(), 3)
}

func TestSortedString(t *testing.T) {
	ctx := context.Background()
	result := Sorted(ctx, object.NewString("cba"))
	list, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, list.Value(), 3)
}

func TestSortedErrors(t *testing.T) {
	ctx := context.Background()
	result := Sorted(ctx)
	assert.True(t, object.IsError(result))

	result = Sorted(ctx, object.NewInt(42))
	assert.True(t, object.IsError(result))

	// Second argument must be function
	result = Sorted(ctx, object.NewList([]object.Object{}), object.NewInt(42))
	assert.True(t, object.IsError(result))
}

func TestChunkErrors(t *testing.T) {
	ctx := context.Background()
	result := Chunk(ctx, object.NewList([]object.Object{}))
	assert.True(t, object.IsError(result))

	result = Chunk(ctx, object.NewList([]object.Object{}), object.NewString("invalid"))
	assert.True(t, object.IsError(result))
}
