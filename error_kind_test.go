package risor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Each error message names its error kind once, whether the error comes
// back from Eval or is caught in a script.
func TestErrorKindOnce(t *testing.T) {
	ctx := context.Background()
	opts := []Option{
		WithEnv(Builtins()),
		WithLimits(Limits{MaxBytesSize: 10, MaxListLength: 10}),
	}
	tests := []struct {
		src  string
		msg  string // the message, with its kind once
		loc  string // the location suffix on the error from Eval
		kind string // e.kind() in a catch block
	}{
		{`"a" + {}`, "type error: unsupported operation for string: + on type map", " (1:7)", "type error"},
		{`"a" * "b"`, "type error: unsupported operation for string: * on type string", " (1:7)", "type error"},
		{`"a" < 1`, "type error: unable to compare string and int", " (1:7)", "type error"},
		{`-"a"`, "type error: object is not a number (got string)", " (1:2)", "type error"},
		{`1 / 0`, "value error: division by zero", " (1:5)", "value error"},
		{`[1, 2, 3, 4, 5, 6] + [1, 2, 3, 4, 5]`, "value error: list too large (exceeds limit of 10 elements)", " (1:35)", "value error"},
		{`"aaaaaa" + "aaaaa"`, "value error: string concatenation result too large (11 bytes exceeds limit of 10)", " (1:12)", "value error"},
		{`"a".repeat(-1)`, "value error: negative repeat count", "", "value error"},
		// Index errors are a kind of value error
		{`[1][5]`, "index error: index out of range: 5", "", "value error"},
		{`{"a": 1}[1]`, "type error: map key must be a string (got int)", "", "type error"},
		{`{}.nope`, `type error: attribute "nope" not found on map object`, " (1:4)", "type error"},
		{`let x = 1; x.y = 2`, `type error: int has no attribute "y"`, "", "type error"},
	}
	for _, tt := range tests {
		_, err := Eval(ctx, tt.src, opts...)
		assert.NotNil(t, err, tt.src)
		assert.Equal(t, err.Error(), tt.msg+tt.loc, tt.src)

		caught, err := Eval(ctx, "try { "+tt.src+" } catch e { e.message() }", opts...)
		assert.Nil(t, err, tt.src)
		assert.True(t, strings.HasPrefix(caught.(string), tt.msg), caught, tt.src)

		kind, err := Eval(ctx, "try { "+tt.src+" } catch e { e.kind() }", opts...)
		assert.Nil(t, err, tt.src)
		assert.Equal(t, kind, tt.kind, tt.src)
	}
}

// Binary operator errors keep their kind and Go error type for Go callers.
func TestErrorKindStructured(t *testing.T) {
	ctx := context.Background()

	_, err := Eval(ctx, `"a" + {}`)
	var se *object.StructuredError
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, se.Kind, object.ErrType)
	assert.Equal(t, se.Message, "unsupported operation for string: + on type map")
	var typeErr *object.TypeError
	assert.True(t, errors.As(err, &typeErr))

	_, err = Eval(ctx, `"a" < 1`)
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, se.Kind, object.ErrType)
	assert.Equal(t, se.Message, "unable to compare string and int")
	assert.True(t, errors.As(err, &typeErr))

	_, err = Eval(ctx, `[1, 2] + [3]`, WithLimits(Limits{MaxListLength: 2}))
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, se.Kind, object.ErrValue)
	assert.Equal(t, se.Message, "list too large (exceeds limit of 2 elements)")
	var valueErr *object.ValueError
	assert.True(t, errors.As(err, &valueErr))
}

// opErrObject is a Go object whose RunOperation returns a StructuredError
// that has no location.
type opErrObject struct {
	*object.Int
}

func (o *opErrObject) RunOperation(opType op.BinaryOpType, right object.Object) (object.Object, error) {
	return nil, object.NewStructuredError(object.ErrValue, "custom failure", object.SourceLocation{}, nil)
}

// The VM stamps the operator location on a StructuredError from a Go object.
// As for the other operator errors above, the column is the column of the
// right operand.
func TestErrorKindStructuredFromGoObject(t *testing.T) {
	ctx := context.Background()
	env := map[string]any{"obj": &opErrObject{Int: object.NewInt(1)}}

	_, err := Eval(ctx, "let a = 1\n  obj + 1", WithEnv(env))
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "value error: custom failure (2:9)")
	var se *object.StructuredError
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, se.Kind, object.ErrValue)
	assert.Equal(t, se.Message, "custom failure")
	assert.Equal(t, se.Location.Line, 2)
	assert.Equal(t, se.Location.Column, 9)
}
