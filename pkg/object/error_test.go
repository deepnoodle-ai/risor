package object

import (
	"context"
	"errors"
	"fmt"
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

func TestErrorEqualsIdentity(t *testing.T) {
	e := NewError(errors.New("boom"))
	assert.True(t, e.Equals(e))
}

func TestErrorEqualsCrossType(t *testing.T) {
	e := NewError(errors.New("boom"))
	assert.False(t, e.Equals(NewString("boom")))
}

func TestErrorEqualsWrappedSentinel(t *testing.T) {
	sentinel := errors.New("file does not exist")
	wrapped := fmt.Errorf("read /tmp/x: %w", sentinel)

	sentinelErr := NewError(sentinel)
	wrappedErr := NewError(wrapped)

	// == must match a wrapped error against its sentinel in both directions,
	// since == is symmetric in script-land.
	assert.True(t, wrappedErr.Equals(sentinelErr))
	assert.True(t, sentinelErr.Equals(wrappedErr))
}

func TestErrorEqualsDeepChain(t *testing.T) {
	sentinel := errors.New("not exist")
	level1 := fmt.Errorf("open: %w", sentinel)
	level2 := fmt.Errorf("read: %w", level1)
	level3 := fmt.Errorf("parse: %w", level2)

	sentinelErr := NewError(sentinel)
	deepErr := NewError(level3)

	assert.True(t, deepErr.Equals(sentinelErr))
	assert.True(t, sentinelErr.Equals(deepErr))
}

func TestErrorEqualsUnrelatedSentinels(t *testing.T) {
	a := NewError(errors.New("a"))
	b := NewError(errors.New("b"))
	assert.False(t, a.Equals(b))
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
