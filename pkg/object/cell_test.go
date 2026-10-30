package object

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestCellBasics(t *testing.T) {
	var obj Object = NewInt(42)
	c := NewCell(&obj)

	assert.Equal(t, c.Type(), CELL)
	assert.Equal(t, c.Value().(*Int).Value(), int64(42))
	assert.True(t, c.IsTruthy())
}

func TestCellSet(t *testing.T) {
	var obj Object = NewInt(1)
	c := NewCell(&obj)

	c.Set(NewInt(99))
	assert.Equal(t, c.Value().(*Int).Value(), int64(99))
}

func TestCellSetNilPointer(t *testing.T) {
	// Cell with nil pointer should not panic on Set
	c := NewCell(nil)
	c.Set(NewInt(42)) // Should not panic
	assert.Nil(t, c.Value())
}

func TestCellValueNil(t *testing.T) {
	c := NewCell(nil)
	assert.Nil(t, c.Value())
	assert.Equal(t, c.String(), "cell()")
}

func TestCellEquals(t *testing.T) {
	var obj1 Object = NewInt(1)
	var obj2 Object = NewInt(1)
	c1 := NewCell(&obj1)
	c2 := NewCell(&obj2)

	// Cells are equal only by identity
	assert.True(t, c1.Equals(c1))
	assert.False(t, c1.Equals(c2))
	assert.False(t, c1.Equals(NewInt(1)))
}

func TestCellInterface(t *testing.T) {
	var obj Object = NewInt(42)
	c := NewCell(&obj)

	assert.Equal(t, c.Interface(), int64(42))

	// Nil cell returns nil interface
	nilCell := NewCell(nil)
	assert.Nil(t, nilCell.Interface())
}

func TestCellInspect(t *testing.T) {
	var obj Object = NewList([]Object{NewInt(1), NewString("a")})
	c := NewCell(&obj)
	assert.Equal(t, c.Inspect(), `cell([1, "a"])`)
	assert.Equal(t, c.String(), `cell([1, "a"])`)
	assert.Equal(t, c.Interface(), []any{int64(1), "a"})
}

func TestCellHoldsItself(t *testing.T) {
	var self Object
	c := NewCell(&self)
	self = c

	assert.Equal(t, c.Inspect(), "cell(cell(...))")
	assert.Equal(t, c.String(), "cell(cell(...))")
	assert.Nil(t, c.Interface())
	assert.Equal(t, Errorf("bad %v", c).Error(), "bad cell(cell(...))")
}

func TestCellListCycle(t *testing.T) {
	// list -> cell -> list
	var value Object
	c := NewCell(&value)
	ls := NewList([]Object{c})
	value = ls

	assert.Equal(t, ls.Inspect(), "[cell([...])]")
	assert.Equal(t, ls.String(), "[cell([...])]")
	assert.Equal(t, c.Inspect(), "cell([cell(...)])")
	assert.Equal(t, c.String(), "cell([cell(...)])")
	assert.Nil(t, ls.Interface())
	assert.Nil(t, c.Interface())
	assert.Equal(t, Errorf("bad %v", ls).Error(), "bad [cell([...])]")
	assert.Equal(t, Errorf("bad %v", c).Error(), "bad cell([cell(...)])")
}
