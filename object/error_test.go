package object

import (
	"context"
	"errors"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestErrorEquals(t *testing.T) {
	e := NewError(errors.New("a"))
	other1 := NewError(errors.New("a"))
	other2 := NewError(errors.New("b"))

	assert.Equal(t, e.Message().Value(), "a")
	assert.True(t, e.Equals(other1))
	assert.False(t, e.Equals(other2))
}

func TestErrorCompareStr(t *testing.T) {
	e := NewError(errors.New("a"))
	other1 := NewError(errors.New("a"))
	other2 := NewError(errors.New("b"))

	cmp, err := e.Compare(other1)
	assert.Nil(t, err)
	assert.Equal(t, cmp, 0)

	cmp, err = e.Compare(other2)
	assert.Nil(t, err)
	assert.Equal(t, cmp, -1)

	cmp, err = other2.Compare(e)
	assert.Nil(t, err)
	assert.Equal(t, cmp, 1)
}

func TestErrorCompareRaised(t *testing.T) {
	a := NewError(errors.New("a")).WithRaised(true)
	b := NewError(errors.New("a")) // raised is set by default

	assert.True(t, a.IsRaised())
	assert.True(t, b.IsRaised())

	result, err := a.Compare(b)
	assert.Nil(t, err)
	assert.Equal(t, result, 0)

	b.WithRaised(false)
	assert.False(t, b.IsRaised())

	result, err = a.Compare(b)
	assert.Nil(t, err)
	assert.Equal(t, result, 1)

	result, err = b.Compare(a)
	assert.Nil(t, err)
	assert.Equal(t, result, -1)
}

func TestErrorMessage(t *testing.T) {
	a := NewError(errors.New("a"))

	attr, ok := a.GetAttr("error")
	assert.True(t, ok)
	fn := attr.(*Builtin)
	result, err := fn.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "a")

	attr, ok = a.GetAttr("message")
	assert.True(t, ok)
	fn = attr.(*Builtin)
	result, err = fn.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "a")
}
