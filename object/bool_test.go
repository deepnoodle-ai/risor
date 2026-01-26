package object_test

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
)

func TestBool(t *testing.T) {
	b := object.NewBool(true)
	obj, ok := b.GetAttr("foo")
	assert.False(t, ok)
	assert.Nil(t, obj)

	// err := b.SetAttr("foo", object.NewInt(int64(1)))
	// require.Error(t, err)
}
