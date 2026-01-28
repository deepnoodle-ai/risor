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
