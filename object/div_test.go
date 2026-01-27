package object

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/op"
)

func TestDivisionByZero(t *testing.T) {
	a := NewInt(10)
	b := NewInt(0)
	_, err := a.RunOperation(op.Divide, b)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "division by zero")
}

func TestModuloByZero(t *testing.T) {
	a := NewInt(10)
	b := NewInt(0)
	_, err := a.RunOperation(op.Modulo, b)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "division by zero")
}

func TestByteDivisionByZero(t *testing.T) {
	a := NewByte(10)
	b := NewByte(0)
	_, err := a.RunOperation(op.Divide, b)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "division by zero")
}

func TestByteModuloByZero(t *testing.T) {
	a := NewByte(10)
	b := NewByte(0)
	_, err := a.RunOperation(op.Modulo, b)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "division by zero")
}

func TestByteDivisionByZeroInt(t *testing.T) {
	a := NewByte(10)
	b := NewInt(0)
	_, err := a.RunOperation(op.Divide, b)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "division by zero")
}
