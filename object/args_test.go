package object_test

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/object"
)

func TestNewArgsError(t *testing.T) {
	err := object.NewArgsError("foo", 2, 3)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: foo() takes exactly 2 arguments (3 given)")
}

func TestNewArgsRangeError(t *testing.T) {
	// Same min and max should use "exactly" message
	err := object.NewArgsRangeError("foo", 2, 2, 3)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: foo() takes exactly 2 arguments (3 given)")

	// Adjacent values (min+1 == max) should use "X or Y" message
	err = object.NewArgsRangeError("bar", 1, 2, 5)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: bar() takes 1 or 2 arguments (5 given)")

	// Range should use "between X and Y" message
	err = object.NewArgsRangeError("baz", 1, 4, 6)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: baz() takes between 1 and 4 arguments (6 given)")
}

func TestRequire(t *testing.T) {
	var err *object.Error

	err = object.Require(
		"foo",
		1,
		[]object.Object{object.NewInt(1)},
	)
	assert.Nil(t, err)

	err = object.Require(
		"foo",
		1,
		[]object.Object{
			object.NewInt(1),
			object.NewInt(1),
			object.NewInt(1),
		},
	)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: foo() takes exactly 1 argument (3 given)")

	err = object.Require(
		"bar",
		2,
		[]object.Object{object.NewInt(1)},
	)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: bar() takes exactly 2 arguments (1 given)")
}

func TestRequireRange(t *testing.T) {
	var err *object.Error

	err = object.RequireRange(
		"foo",
		1,
		3,
		[]object.Object{object.NewInt(1)},
	)
	assert.Nil(t, err)

	err = object.RequireRange(
		"foo",
		1,
		3,
		[]object.Object{
			object.NewInt(1),
			object.NewInt(1),
			object.NewInt(1),
			object.NewInt(1),
		},
	)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: foo() takes at most 3 arguments (4 given)")

	err = object.RequireRange(
		"foo",
		1,
		3,
		[]object.Object{},
	)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: foo() takes at least 1 argument (0 given)")

	// Test pluralization is based on required count, not given count
	err = object.RequireRange(
		"bar",
		2,
		4,
		[]object.Object{object.NewInt(1)},
	)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: bar() takes at least 2 arguments (1 given)")

	// Test "at most 1 argument" uses singular
	err = object.RequireRange(
		"baz",
		0,
		1,
		[]object.Object{object.NewInt(1), object.NewInt(2)},
	)
	assert.NotNil(t, err)
	assert.Equal(t, err.Message().Value(), "args error: baz() takes at most 1 argument (2 given)")
}
