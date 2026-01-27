package object_test

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/op"
)

func TestBool(t *testing.T) {
	b := object.NewBool(true)
	obj, ok := b.GetAttr("foo")
	assert.False(t, ok)
	assert.Nil(t, obj)
}

func TestBoolType(t *testing.T) {
	assert.Equal(t, object.True.Type(), object.BOOL)
	assert.Equal(t, object.False.Type(), object.BOOL)
}

func TestBoolValue(t *testing.T) {
	assert.True(t, object.True.Value())
	assert.False(t, object.False.Value())
}

func TestBoolInspect(t *testing.T) {
	assert.Equal(t, object.True.Inspect(), "true")
	assert.Equal(t, object.False.Inspect(), "false")
}

func TestBoolHashKey(t *testing.T) {
	trueKey := object.True.HashKey()
	falseKey := object.False.HashKey()

	assert.Equal(t, trueKey.Type, object.BOOL)
	assert.Equal(t, trueKey.IntValue, int64(1))

	assert.Equal(t, falseKey.Type, object.BOOL)
	assert.Equal(t, falseKey.IntValue, int64(0))

	// Same values should have same hash
	assert.Equal(t, object.NewBool(true).HashKey(), trueKey)
	assert.Equal(t, object.NewBool(false).HashKey(), falseKey)
}

func TestBoolInterface(t *testing.T) {
	assert.Equal(t, object.True.Interface(), true)
	assert.Equal(t, object.False.Interface(), false)
}

func TestBoolString(t *testing.T) {
	assert.Equal(t, object.True.String(), "true")
	assert.Equal(t, object.False.String(), "false")
}

func TestBoolCompare(t *testing.T) {
	// Equal
	cmp, err := object.True.Compare(object.True)
	assert.Nil(t, err)
	assert.Equal(t, cmp, 0)

	cmp, err = object.False.Compare(object.False)
	assert.Nil(t, err)
	assert.Equal(t, cmp, 0)

	// true > false
	cmp, err = object.True.Compare(object.False)
	assert.Nil(t, err)
	assert.Equal(t, cmp, 1)

	// false < true
	cmp, err = object.False.Compare(object.True)
	assert.Nil(t, err)
	assert.Equal(t, cmp, -1)

	// Error on different type
	_, err = object.True.Compare(object.NewInt(1))
	assert.NotNil(t, err)
}

func TestBoolEquals(t *testing.T) {
	assert.True(t, object.True.Equals(object.True))
	assert.True(t, object.False.Equals(object.False))
	assert.False(t, object.True.Equals(object.False))
	assert.False(t, object.False.Equals(object.True))

	// Different type
	assert.False(t, object.True.Equals(object.NewInt(1)))
}

func TestBoolIsTruthy(t *testing.T) {
	assert.True(t, object.True.IsTruthy())
	assert.False(t, object.False.IsTruthy())
}

func TestBoolRunOperation(t *testing.T) {
	_, err := object.True.RunOperation(op.Add, object.False)
	assert.NotNil(t, err)
}

func TestBoolMarshalJSON(t *testing.T) {
	data, err := object.True.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, string(data), "true")

	data, err = object.False.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, string(data), "false")
}

func TestNewBool(t *testing.T) {
	// NewBool returns singleton True/False
	assert.Equal(t, object.NewBool(true), object.True)
	assert.Equal(t, object.NewBool(false), object.False)
}

func TestNot(t *testing.T) {
	assert.Equal(t, object.Not(object.True), object.False)
	assert.Equal(t, object.Not(object.False), object.True)
}

func TestObjectEquals(t *testing.T) {
	// The package-level Equals function
	assert.True(t, object.Equals(object.NewInt(1), object.NewInt(1)))
	assert.False(t, object.Equals(object.NewInt(1), object.NewInt(2)))
}
