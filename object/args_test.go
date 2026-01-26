package object_test

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
)

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
}
