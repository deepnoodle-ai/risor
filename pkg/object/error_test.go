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

func TestErrorEqualsKind(t *testing.T) {
	// Same message, different kinds: not equal.
	plain := NewError(errors.New("index out of range: 5"))
	value := NewError(IndexErrorf("index out of range: 5"))
	typed := NewError(TypeErrorf("index out of range: 5"))
	assert.Equal(t, plain.Message().Value(), value.Message().Value())
	assert.False(t, plain.Equals(value))
	assert.False(t, value.Equals(plain))
	assert.False(t, value.Equals(typed))

	// Same message, same kind: equal.
	assert.True(t, value.Equals(NewError(ValueErrorf("index out of range: 5"))))
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

// Errorf formats an Object argument as Risor prints it. A primitive formats
// as its Go value, so verbs such as %d and %q work.
func TestErrorfObjectArgs(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewString("a")})
	m := NewMap(map[string]Object{"k": NewInt(2)})
	err := Errorf("bad %v and %v", list, m)
	assert.Equal(t, err.Error(), `bad [1, "a"] and {"k": 2}`)

	err = Errorf("%q has %d items, %v", NewString("x"), NewInt(3), True)
	assert.Equal(t, err.Error(), `"x" has 3 items, true`)

	assert.Equal(t, Errorf("got %v", Nil).Error(), "got null")
	assert.Equal(t, Errorf("got %s", "plain").Error(), "got plain")
}

// Errorf does not convert an Object to Go, so the conversion limits do not
// apply. Before, a value that failed to convert formatted as <nil>.
func TestErrorfNoConversion(t *testing.T) {
	// A range longer than DefaultMaxListLength
	long := NewRange(0, DefaultMaxListLength+1, 1)
	assert.Nil(t, long.Interface())
	assert.Equal(t, Errorf("bad %v", long).Error(),
		fmt.Sprintf("bad range(%d)", DefaultMaxListLength+1))

	// A list that contains itself
	cyclic := NewList([]Object{NewInt(1)})
	cyclic.Append(cyclic)
	assert.Nil(t, cyclic.Interface())
	assert.Equal(t, Errorf("bad %v", cyclic).Error(), "bad [1, [...]]")
}

func TestErrorfSharedReferencesBounded(t *testing.T) {
	// Each level references the previous one twice, so the full text would
	// repeat the innermost list 2^40 times
	shared := NewList([]Object{NewInt(1)})
	for range 40 {
		shared = NewList([]Object{shared, shared})
	}
	assert.Equal(t, Errorf("bad %v", shared).Error(), "bad [...]")

	m := NewMap(map[string]Object{"a": shared})
	assert.Equal(t, Errorf("bad %v", m).Error(), "bad {...}")

	// A small value with shared references still formats in full
	small := NewList([]Object{NewInt(1)})
	pair := NewList([]Object{small, small})
	assert.Equal(t, Errorf("bad %v", pair).Error(), "bad [[1], [1]]")
}
