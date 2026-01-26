package object

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestListInsert(t *testing.T) {
	one := NewInt(1)
	two := NewInt(2)
	thr := NewInt(3)

	list := NewList([]Object{one})

	list.Insert(5, two)
	assert.Equal(t, list.Value(), []Object{one, two})

	list.Insert(-10, thr)
	assert.Equal(t, list.Value(), []Object{thr, one, two})

	list.Insert(1, two)
	assert.Equal(t, list.Value(), []Object{thr, two, one, two})

	list.Insert(0, two)
	assert.Equal(t, list.Value(), []Object{two, thr, two, one, two})
}

func TestListPop(t *testing.T) {
	zero := NewString("0")
	one := NewString("1")
	two := NewString("2")

	list := NewList([]Object{zero, one, two})

	val, ok := list.Pop(1).(*String)
	assert.True(t, ok)
	assert.Equal(t, val.Value(), "1")

	val, ok = list.Pop(1).(*String)
	assert.True(t, ok)
	assert.Equal(t, val.Value(), "2")

	err, ok := list.Pop(1).(*Error)
	assert.True(t, ok)
	assert.Equal(t, err.Message().Value(), "index error: index out of range: 1")
}
