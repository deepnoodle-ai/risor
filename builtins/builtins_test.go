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
	assert.Greater(t, count, 22) // error() restored; try(), set(), buffer(), delete() still removed
}

func TestError(t *testing.T) {
	ctx := context.Background()

	// Simple error message
	result, err := Error(ctx, object.NewString("something went wrong"))
	assert.Nil(t, err)
	errObj, ok := result.(*object.Error)
	assert.True(t, ok)
	assert.Equal(t, errObj.Value().Error(), "something went wrong")

	// Formatted error message
	result, err = Error(ctx, object.NewString("file %s not found at line %d"), object.NewString("test.txt"), object.NewInt(42))
	assert.Nil(t, err)
	errObj, ok = result.(*object.Error)
	assert.True(t, ok)
	assert.Equal(t, errObj.Value().Error(), "file test.txt not found at line 42")

	// No arguments - should error
	_, err = Error(ctx)
	assert.NotNil(t, err)
}

type testCase struct {
	input    object.Object
	expected object.Object
}

func TestSorted(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		input       object.Object
		expected    object.Object
		expectError bool
	}{
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
			false,
		},
		{
			object.NewList([]object.Object{
				object.NewInt(3),
				object.NewInt(1),
				object.NewString("nope"),
			}),
			nil,
			true,
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
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.input.Inspect(), func(t *testing.T) {
			result, err := Sorted(ctx, tt.input)
			if tt.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assertObjectEqual(t, result, tt.expected)
			}
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
	result, err := Sorted(ctx, input, sortFn)
	assert.Nil(t, err)
	assert.Equal(t,

		result, object.NewList([]object.Object{
			object.NewInt(99),
			object.NewInt(3),
			object.NewInt(2),
			object.NewInt(1),
			object.NewInt(0),
		}))
}

func TestSortedWithBuiltin(t *testing.T) {
	ctx := context.Background()
	input := object.NewList([]object.Object{
		object.NewInt(3),
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(99),
		object.NewInt(0),
	})

	// Create a builtin comparator for descending order
	descending := object.NewBuiltin("descending", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		if len(args) != 2 {
			return nil, object.TypeErrorf("expected 2 arguments, got %d", len(args))
		}
		a, ok1 := args[0].(*object.Int)
		b, ok2 := args[1].(*object.Int)
		if !ok1 || !ok2 {
			return nil, object.TypeErrorf("expected int arguments")
		}
		// Return true if b < a (descending order)
		return object.NewBool(b.Value() < a.Value()), nil
	})

	result, err := Sorted(ctx, input, descending)
	assert.Nil(t, err)

	expected := object.NewList([]object.Object{
		object.NewInt(99),
		object.NewInt(3),
		object.NewInt(2),
		object.NewInt(1),
		object.NewInt(0),
	})
	assertObjectEqual(t, result, expected)
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
			result, err := Coalesce(ctx, tt.input.(*object.List).Value()...)
			assert.Nil(t, err)
			assertObjectEqual(t, result, tt.expected)
		})
	}
}

func TestChunk(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		input       object.Object
		size        int64
		expected    object.Object
		expectError bool
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
			false,
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
			false,
		},
		{
			object.NewString("wrong"),
			2,
			nil,
			true,
		},
		{
			object.NewList([]object.Object{}),
			-1,
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.input.Inspect(), func(t *testing.T) {
			result, err := Chunk(ctx, tt.input, object.NewInt(tt.size))
			if tt.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assertObjectEqual(t, result, tt.expected)
			}
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
			result, err := Len(ctx, tt.input)
			assert.Nil(t, err)
			assertObjectEqual(t, result, tt.expected)
		})
	}
}

func TestLenErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	_, err := Len(ctx)
	assert.NotNil(t, err)

	// Unsupported type
	_, err = Len(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestSprintf(t *testing.T) {
	ctx := context.Background()
	result, err := Sprintf(ctx, object.NewString("hello %s"), object.NewString("world"))
	assert.Nil(t, err)
	assertObjectEqual(t, result, object.NewString("hello world"))

	result, err = Sprintf(ctx, object.NewString("value: %d"), object.NewInt(42))
	assert.Nil(t, err)
	assertObjectEqual(t, result, object.NewString("value: 42"))
}

func TestSprintfErrors(t *testing.T) {
	ctx := context.Background()
	// Wrong argument count
	_, err := Sprintf(ctx)
	assert.NotNil(t, err)

	// Wrong type for format string
	_, err = Sprintf(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestList(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result, err := List(ctx)
	assert.Nil(t, err)
	list, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, list.Value(), 0)
}

func TestListErrors(t *testing.T) {
	ctx := context.Background()

	// Int is not supported
	_, err := List(ctx, object.NewInt(3))
	assert.NotNil(t, err)

	// Negative int is not supported
	_, err = List(ctx, object.NewInt(-1))
	assert.NotNil(t, err)

	// Non-enumerable type
	_, err = List(ctx, object.NewFloat(3.14))
	assert.NotNil(t, err)
}

func TestString(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result, err := String(ctx)
	assert.Nil(t, err)
	assertObjectEqual(t, result, object.NewString(""))

	// String argument
	result, err = String(ctx, object.NewString("hello"))
	assert.Nil(t, err)
	assertObjectEqual(t, result, object.NewString("hello"))

	// Int argument
	result, err = String(ctx, object.NewInt(42))
	assert.Nil(t, err)
	assertObjectEqual(t, result, object.NewString("42"))
}

func TestStringErrors(t *testing.T) {
	ctx := context.Background()
	// Too many arguments
	_, err := String(ctx, object.NewString("a"), object.NewString("b"))
	assert.NotNil(t, err)
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
			result, err := Type(ctx, tt.input)
			assert.Nil(t, err)
			assertObjectEqual(t, result, object.NewString(tt.expected))
		})
	}
}

func TestTypeErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Type(ctx)
	assert.NotNil(t, err)
}

func TestAssert(t *testing.T) {
	ctx := context.Background()

	// Truthy assertion
	result, err := Assert(ctx, object.True)
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)

	// With message
	result, err = Assert(ctx, object.True, object.NewString("should pass"))
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)

	// Failed assertion
	_, err = Assert(ctx, object.False)
	assert.NotNil(t, err)

	// Failed assertion with message
	_, err = Assert(ctx, object.False, object.NewString("custom error"))
	assert.NotNil(t, err)
}

func TestAssertErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Assert(ctx)
	assert.NotNil(t, err)
}

func TestAny(t *testing.T) {
	ctx := context.Background()

	// Some truthy
	result, err := Any(ctx, object.NewList([]object.Object{object.False, object.True, object.False}))
	assert.Nil(t, err)
	assert.Equal(t, result, object.True)

	// All falsy
	result, err = Any(ctx, object.NewList([]object.Object{object.False, object.Nil}))
	assert.Nil(t, err)
	assert.Equal(t, result, object.False)

	// Empty list
	result, err = Any(ctx, object.NewList([]object.Object{}))
	assert.Nil(t, err)
	assert.Equal(t, result, object.False)
}

func TestAnyErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Any(ctx)
	assert.NotNil(t, err)
}

func TestAll(t *testing.T) {
	ctx := context.Background()

	// All truthy
	result, err := All(ctx, object.NewList([]object.Object{object.True, object.NewInt(1), object.NewString("yes")}))
	assert.Nil(t, err)
	assert.Equal(t, result, object.True)

	// Some falsy
	result, err = All(ctx, object.NewList([]object.Object{object.True, object.False}))
	assert.Nil(t, err)
	assert.Equal(t, result, object.False)

	// Empty list (vacuous truth)
	result, err = All(ctx, object.NewList([]object.Object{}))
	assert.Nil(t, err)
	assert.Equal(t, result, object.True)
}

func TestAllErrors(t *testing.T) {
	ctx := context.Background()
	_, err := All(ctx)
	assert.NotNil(t, err)
}

func TestBool(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result, err := Bool(ctx)
	assert.Nil(t, err)
	assert.Equal(t, result, object.False)

	// Truthy values
	result, err = Bool(ctx, object.True)
	assert.Nil(t, err)
	assert.Equal(t, result, object.True)

	result, err = Bool(ctx, object.NewInt(1))
	assert.Nil(t, err)
	assert.Equal(t, result, object.True)

	// Falsy values
	result, err = Bool(ctx, object.False)
	assert.Nil(t, err)
	assert.Equal(t, result, object.False)

	result, err = Bool(ctx, object.Nil)
	assert.Nil(t, err)
	assert.Equal(t, result, object.False)
}

func TestBoolErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Bool(ctx, object.NewInt(1), object.NewInt(2))
	assert.NotNil(t, err)
}

func TestReversed(t *testing.T) {
	ctx := context.Background()

	// List
	result, err := Reversed(ctx, object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
	}))
	assert.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewInt(3),
		object.NewInt(2),
		object.NewInt(1),
	})
	assertObjectEqual(t, result, expected)

	// String
	result, err = Reversed(ctx, object.NewString("abc"))
	assert.Nil(t, err)
	assertObjectEqual(t, result, object.NewString("cba"))
}

func TestReversedErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Reversed(ctx)
	assert.NotNil(t, err)

	_, err = Reversed(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestKeys(t *testing.T) {
	ctx := context.Background()

	// List keys are indices
	list := object.NewList([]object.Object{object.NewString("a"), object.NewString("b")})
	result, err := Keys(ctx, list)
	assert.Nil(t, err)
	keyList, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, keyList.Value(), 2)
}

func TestKeysErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Keys(ctx)
	assert.NotNil(t, err)
}

func TestByte(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result, err := Byte(ctx)
	assert.Nil(t, err)
	b, ok := result.(*object.Byte)
	assert.True(t, ok)
	assert.Equal(t, b.Value(), byte(0))

	// From int
	result, err = Byte(ctx, object.NewInt(65))
	assert.Nil(t, err)
	b, ok = result.(*object.Byte)
	assert.True(t, ok)
	assert.Equal(t, b.Value(), byte(65))

	// From float
	result, err = Byte(ctx, object.NewFloat(66.5))
	assert.Nil(t, err)
	b, ok = result.(*object.Byte)
	assert.True(t, ok)
	assert.Equal(t, b.Value(), byte(66))

	// From string
	result, err = Byte(ctx, object.NewString("42"))
	assert.Nil(t, err)
	b, ok = result.(*object.Byte)
	assert.True(t, ok)
	assert.Equal(t, b.Value(), byte(42))
}

func TestByteErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Byte(ctx, object.NewInt(1), object.NewInt(2))
	assert.NotNil(t, err)

	_, err = Byte(ctx, object.NewString("invalid"))
	assert.NotNil(t, err)

	_, err = Byte(ctx, object.NewList([]object.Object{}))
	assert.NotNil(t, err)
}

func TestInt(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result, err := Int(ctx)
	assert.Nil(t, err)
	i, ok := result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(0))

	// From int
	result, err = Int(ctx, object.NewInt(42))
	assert.Nil(t, err)
	i, ok = result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(42))

	// From float
	result, err = Int(ctx, object.NewFloat(3.7))
	assert.Nil(t, err)
	i, ok = result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(3))

	// From string
	result, err = Int(ctx, object.NewString("123"))
	assert.Nil(t, err)
	i, ok = result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(123))

	// Hex string
	result, err = Int(ctx, object.NewString("0xff"))
	assert.Nil(t, err)
	i, ok = result.(*object.Int)
	assert.True(t, ok)
	assert.Equal(t, i.Value(), int64(255))
}

func TestIntErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Int(ctx, object.NewInt(1), object.NewInt(2))
	assert.NotNil(t, err)

	_, err = Int(ctx, object.NewString("invalid"))
	assert.NotNil(t, err)

	_, err = Int(ctx, object.NewList([]object.Object{}))
	assert.NotNil(t, err)
}

func TestFloat(t *testing.T) {
	ctx := context.Background()

	// No arguments
	result, err := Float(ctx)
	assert.Nil(t, err)
	f, ok := result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), float64(0))

	// From int
	result, err = Float(ctx, object.NewInt(42))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), float64(42))

	// From float
	result, err = Float(ctx, object.NewFloat(3.14))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), float64(3.14))

	// From string
	result, err = Float(ctx, object.NewString("3.14"))
	assert.Nil(t, err)
	f, ok = result.(*object.Float)
	assert.True(t, ok)
	assert.Equal(t, f.Value(), float64(3.14))
}

func TestFloatErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Float(ctx, object.NewInt(1), object.NewInt(2))
	assert.NotNil(t, err)

	_, err = Float(ctx, object.NewString("invalid"))
	assert.NotNil(t, err)

	_, err = Float(ctx, object.NewList([]object.Object{}))
	assert.NotNil(t, err)
}

func TestGetAttr(t *testing.T) {
	ctx := context.Background()

	// Existing attribute on list
	list := object.NewList([]object.Object{object.NewInt(1), object.NewInt(2)})
	result, err := GetAttr(ctx, list, object.NewString("append"))
	assert.Nil(t, err)
	_, ok := result.(*object.Builtin)
	assert.True(t, ok)

	// With default value for missing attribute
	result, err = GetAttr(ctx, object.NewInt(42), object.NewString("missing"), object.NewString("default"))
	assert.Nil(t, err)
	assertObjectEqual(t, result, object.NewString("default"))
}

func TestGetAttrErrors(t *testing.T) {
	ctx := context.Background()
	_, err := GetAttr(ctx, object.NewInt(42))
	assert.NotNil(t, err)

	_, err = GetAttr(ctx, object.NewInt(42), object.NewInt(1))
	assert.NotNil(t, err)

	// Missing attribute without default
	_, err = GetAttr(ctx, object.NewInt(42), object.NewString("missing"))
	assert.NotNil(t, err)
}

func TestFilter(t *testing.T) {
	ctx := context.Background()

	// Create a simple builtin function that returns true for even numbers
	isEven := object.NewBuiltin("is_even", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		if len(args) != 1 {
			return object.False, nil
		}
		if i, ok := args[0].(*object.Int); ok {
			return object.NewBool(i.Value()%2 == 0), nil
		}
		return object.False, nil
	})

	list := object.NewList([]object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
		object.NewInt(4),
	})

	result, err := Filter(ctx, list, isEven)
	assert.Nil(t, err)
	expected := object.NewList([]object.Object{
		object.NewInt(2),
		object.NewInt(4),
	})
	assertObjectEqual(t, result, expected)
}

func TestFilterErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Filter(ctx, object.NewList([]object.Object{}))
	assert.NotNil(t, err)

	_, err = Filter(ctx, object.NewList([]object.Object{}), object.NewInt(42))
	assert.NotNil(t, err)
}

func TestCall(t *testing.T) {
	ctx := context.Background()

	// Call a builtin
	addOne := object.NewBuiltin("add_one", func(ctx context.Context, args ...object.Object) (object.Object, error) {
		if len(args) != 1 {
			return nil, object.Errorf("expected 1 argument").Value()
		}
		if i, ok := args[0].(*object.Int); ok {
			return object.NewInt(i.Value() + 1), nil
		}
		return nil, object.Errorf("expected int").Value()
	})

	result, err := Call(ctx, addOne, object.NewInt(5))
	assert.Nil(t, err)
	assertObjectEqual(t, result, object.NewInt(6))
}

func TestCallErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Call(ctx)
	assert.NotNil(t, err)

	_, err = Call(ctx, object.NewInt(42))
	assert.NotNil(t, err)
}

func TestSortedMap(t *testing.T) {
	ctx := context.Background()
	m := object.NewMap(map[string]object.Object{
		"b": object.NewInt(2),
		"a": object.NewInt(1),
		"c": object.NewInt(3),
	})

	result, err := Sorted(ctx, m)
	assert.Nil(t, err)
	list, ok := result.(*object.List)
	assert.True(t, ok)
	// Keys should be sorted
	assert.Len(t, list.Value(), 3)
}

func TestSortedString(t *testing.T) {
	ctx := context.Background()
	result, err := Sorted(ctx, object.NewString("cba"))
	assert.Nil(t, err)
	list, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, list.Value(), 3)
}

func TestSortedErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Sorted(ctx)
	assert.NotNil(t, err)

	_, err = Sorted(ctx, object.NewInt(42))
	assert.NotNil(t, err)

	// Second argument must be function
	_, err = Sorted(ctx, object.NewList([]object.Object{}), object.NewInt(42))
	assert.NotNil(t, err)
}

func TestChunkErrors(t *testing.T) {
	ctx := context.Background()
	_, err := Chunk(ctx, object.NewList([]object.Object{}))
	assert.NotNil(t, err)

	_, err = Chunk(ctx, object.NewList([]object.Object{}), object.NewString("invalid"))
	assert.NotNil(t, err)
}

func TestRange(t *testing.T) {
	ctx := context.Background()

	// range(5)
	result, err := Range(ctx, object.NewInt(5))
	assert.Nil(t, err)
	r, ok := result.(*object.Range)
	assert.True(t, ok)
	assert.Equal(t, r.Start(), int64(0))
	assert.Equal(t, r.Stop(), int64(5))
	assert.Equal(t, r.Step(), int64(1))

	// range(1, 5)
	result, err = Range(ctx, object.NewInt(1), object.NewInt(5))
	assert.Nil(t, err)
	r, ok = result.(*object.Range)
	assert.True(t, ok)
	assert.Equal(t, r.Start(), int64(1))
	assert.Equal(t, r.Stop(), int64(5))
	assert.Equal(t, r.Step(), int64(1))

	// range(0, 10, 2)
	result, err = Range(ctx, object.NewInt(0), object.NewInt(10), object.NewInt(2))
	assert.Nil(t, err)
	r, ok = result.(*object.Range)
	assert.True(t, ok)
	assert.Equal(t, r.Start(), int64(0))
	assert.Equal(t, r.Stop(), int64(10))
	assert.Equal(t, r.Step(), int64(2))

	// range(5, 0, -1)
	result, err = Range(ctx, object.NewInt(5), object.NewInt(0), object.NewInt(-1))
	assert.Nil(t, err)
	r, ok = result.(*object.Range)
	assert.True(t, ok)
	assert.Equal(t, r.Start(), int64(5))
	assert.Equal(t, r.Stop(), int64(0))
	assert.Equal(t, r.Step(), int64(-1))
}

func TestRangeErrors(t *testing.T) {
	ctx := context.Background()

	// No arguments
	_, err := Range(ctx)
	assert.NotNil(t, err)

	// Too many arguments
	_, err = Range(ctx, object.NewInt(1), object.NewInt(2), object.NewInt(3), object.NewInt(4))
	assert.NotNil(t, err)

	// Non-int argument
	_, err = Range(ctx, object.NewString("5"))
	assert.NotNil(t, err)

	// Step of zero
	_, err = Range(ctx, object.NewInt(0), object.NewInt(10), object.NewInt(0))
	assert.NotNil(t, err)
}

func TestRangeWithList(t *testing.T) {
	ctx := context.Background()

	// Convert range to list
	r, _ := Range(ctx, object.NewInt(5))
	listResult, err := List(ctx, r)
	assert.Nil(t, err)
	list, ok := listResult.(*object.List)
	assert.True(t, ok)
	assert.Len(t, list.Value(), 5)

	expected := []int64{0, 1, 2, 3, 4}
	for i, v := range list.Value() {
		assert.Equal(t, v.(*object.Int).Value(), expected[i])
	}
}
