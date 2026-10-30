package risor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
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
		src string
		msg string // the message, with its kind once
		loc string // the location suffix on the error from Eval
	}{
		{`"a" + {}`, "type error: unsupported operation for string: + on type map", " (1:7)"},
		{`"a" * "b"`, "type error: unsupported operation for string: * on type string", " (1:7)"},
		{`"a" < 1`, "type error: unable to compare string and int", " (1:7)"},
		{`-"a"`, "type error: object is not a number (got string)", " (1:2)"},
		{`1 / 0`, "value error: division by zero", " (1:5)"},
		{`[1, 2, 3, 4, 5, 6] + [1, 2, 3, 4, 5]`, "value error: list too large (exceeds limit of 10 elements)", " (1:35)"},
		{`"aaaaaa" + "aaaaa"`, "value error: string concatenation result too large (11 bytes exceeds limit of 10)", " (1:12)"},
		{`"a".repeat(-1)`, "value error: negative repeat count", ""},
		{`[1][5]`, "index error: index out of range: 5", ""},
		{`{"a": 1}[1]`, "type error: map key must be a string (got int)", ""},
		{`{}.nope`, `type error: attribute "nope" not found on map object`, " (1:4)"},
		{`let x = 1; x.y = 2`, `type error: int has no attribute "y"`, ""},
	}
	for _, tt := range tests {
		_, err := Eval(ctx, tt.src, opts...)
		assert.NotNil(t, err, tt.src)
		assert.Equal(t, err.Error(), tt.msg+tt.loc, tt.src)

		caught, err := Eval(ctx, "try { "+tt.src+" } catch e { e.message() }", opts...)
		assert.Nil(t, err, tt.src)
		assert.True(t, strings.HasPrefix(caught.(string), tt.msg), caught, tt.src)
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
