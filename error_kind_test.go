package risor

import (
	"context"
	"errors"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

// An error from Eval names its kind once and ends with its location. A
// caught error's message() has neither, and kind() returns the kind (#507).
func TestErrorKindOnce(t *testing.T) {
	ctx := context.Background()
	opts := []Option{
		WithEnv(Builtins()),
		WithLimits(Limits{MaxBytesSize: 10, MaxListLength: 10}),
	}
	tests := []struct {
		src  string
		kind string // e.kind() in a catch block
		msg  string // e.message() in a catch block
		loc  string // the location on the error from Eval
	}{
		{`"a" + {}`, "type error", "unsupported operation for string: + on type map", "1:7"},
		{`"a" * "b"`, "type error", "unsupported operation for string: * on type string", "1:7"},
		{`"a" < 1`, "type error", "unable to compare string and int", "1:7"},
		{`-"a"`, "type error", "object is not a number (got string)", "1:2"},
		{`1 / 0`, "value error", "division by zero", "1:5"},
		{`[1, 2, 3, 4, 5, 6] + [1, 2, 3, 4, 5]`, "value error", "list too large (exceeds limit of 10 elements)", "1:35"},
		{`"aaaaaa" + "aaaaa"`, "value error", "string concatenation result too large (11 bytes exceeds limit of 10)", "1:12"},
		{`"a".repeat(-1)`, "value error", "negative repeat count", "1:5"},
		{`1 << -1`, "value error", "negative shift count", "1:7"},
		{`byte(1) >> -1`, "value error", "negative shift count", "1:13"},
		// Index errors are a kind of value error
		{`[1][5]`, "value error", "index out of range: 5", "1:5"},
		{`[1, 2, 3][10]`, "value error", "index out of range: 10", "1:11"},
		{`{"a": 1}[1]`, "type error", "map key must be a string (got int)", "1:10"},
		{`{}.nope`, "type error", `attribute "nope" not found on map object`, "1:4"},
		{`let x = 1; x.y = 2`, "type error", `int has no attribute "y"`, "1:12"},
		{`throw "oops"`, "runtime error", "oops", "1:7"},
	}
	for _, tt := range tests {
		_, err := Eval(ctx, tt.src, opts...)
		assert.NotNil(t, err, tt.src)
		assert.Equal(t, err.Error(), tt.kind+": "+tt.msg+" ("+tt.loc+")", tt.src)

		caught, err := Eval(ctx, "try { "+tt.src+" } catch e { [e.kind(), e.message(), string(e)] }", opts...)
		assert.Nil(t, err, tt.src)
		assert.Equal(t, caught, []any{tt.kind, tt.msg, tt.msg}, tt.src)
	}
}

// Two errors with the same message and different kinds are not equal.
func TestErrorEqualsKind(t *testing.T) {
	ctx := context.Background()
	opts := []Option{WithEnv(Builtins())}
	result, err := Eval(ctx, `
		let a = try { [1][5] } catch e { e }
		let b = try { [2][5] } catch e { e }
		[a == error("index out of range: 5"), a == b]
	`, opts...)
	assert.Nil(t, err)
	assert.Equal(t, result, []any{false, true})
}

// Every runtime error from Eval is a *StructuredError with a location. Its
// cause is the typed error when there is one, so errors.As finds both (#504).
func TestErrorShape(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		src       string
		typeErr   bool
		valueErr  bool
		indexErr  bool
		line, col int
	}{
		{`"a" + {}`, true, false, false, 1, 7},
		{`"a" < 1`, true, false, false, 1, 7},
		{`-"a"`, true, false, false, 1, 2},
		{`{}.nope`, true, false, false, 1, 4},
		{`let x = 1; x.y = 2`, true, false, false, 1, 12},
		{`"a".repeat(-1)`, false, true, false, 1, 5},
		// A call error points at the callee, not the last argument
		{"\"a\".repeat(\n  -1)", false, true, false, 1, 5},
		{"int(\n  \"x\")", false, true, false, 1, 1},
		{`[1][5]`, false, false, true, 1, 5},
		{`{"a": 1}[1]`, true, false, false, 1, 10},
		{`throw "oops"`, false, false, false, 1, 7},
		// An error raised in a function keeps the location where it was raised
		{"function f() {\n  [1][5]\n}\nf()", false, false, true, 2, 7},
		{"[1].map(x => {\n  [1][5]\n})", false, false, true, 2, 7},
	}
	for _, tt := range tests {
		_, err := Eval(ctx, tt.src, WithEnv(Builtins()))
		assert.NotNil(t, err, tt.src)
		var se *object.StructuredError
		assert.True(t, errors.As(err, &se), tt.src)
		assert.Equal(t, se.Location.Line, tt.line, tt.src)
		assert.Equal(t, se.Location.Column, tt.col, tt.src)
		var typeErr *object.TypeError
		var valueErr *object.ValueError
		var indexErr *object.IndexError
		assert.Equal(t, errors.As(err, &typeErr), tt.typeErr, tt.src)
		assert.Equal(t, errors.As(err, &valueErr), tt.valueErr, tt.src)
		assert.Equal(t, errors.As(err, &indexErr), tt.indexErr, tt.src)
	}
}

// An error from a host function keeps its identity for errors.Is, and a
// re-thrown error keeps the location where it was first raised.
func TestErrorShapeCause(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("sentinel")
	env := map[string]any{"fail": func() error { return sentinel }}
	_, err := Eval(ctx, "\n fail()", WithEnv(env))
	assert.True(t, errors.Is(err, sentinel))
	var se *object.StructuredError
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, se.Kind, object.ErrRuntime)
	assert.Equal(t, se.Message, "sentinel")
	assert.Equal(t, se.Location.Line, 2)

	_, err = Eval(ctx, "let e = try { [1][5] } catch e { e }\nthrow e")
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, se.Location.Line, 1)
	var indexErr *object.IndexError
	assert.True(t, errors.As(err, &indexErr))
}

// An error from converting the script result to a Go value has the same
// shape as a runtime error, with no location (#528).
func TestErrorShapeResultConversion(t *testing.T) {
	ctx := context.Background()
	limits := WithLimits(Limits{MaxListLength: 10})
	_, err := Eval(ctx, "range(11)", WithEnv(Builtins()), limits)
	var se *object.StructuredError
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, se.Kind, object.ErrValue)
	assert.Equal(t, se.Message, "value too large to convert (exceeds limit of 10 elements)")
	assert.True(t, se.Location.IsZero())
	var valueErr *object.ValueError
	assert.True(t, errors.As(err, &valueErr))
	assert.Equal(t, err.Error(), "value error: value too large to convert (exceeds limit of 10 elements)")

	// Run converts the result the same way
	code, err := Compile(ctx, "range(11)", WithEnv(Builtins()))
	assert.Nil(t, err)
	_, err = Run(ctx, code, WithEnv(Builtins()), limits)
	assert.True(t, errors.As(err, &se))
	assert.True(t, errors.As(err, &valueErr))

	// A result that contains itself
	_, err = Eval(ctx, "let a = []; a.append(a); a")
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, se.Kind, object.ErrValue)
	assert.True(t, errors.As(err, &valueErr))
	assert.True(t, errors.Is(err, object.ErrNestingDepth))
}

type kindBox struct {
	Items []int64
}

// A conversion error that a host function or struct field wraps keeps its
// kind, and its message keeps the context the wrapper adds (#498).
func TestErrorKindWrapped(t *testing.T) {
	ctx := context.Background()
	env := Builtins()
	env["count"] = func(v []int64) int { return len(v) }
	env["box"] = &kindBox{}
	opts := []Option{WithEnv(env), WithLimits(Limits{MaxListLength: 10})}
	tests := []struct {
		src string
		msg string
	}{
		{`count(range(11))`, "func([]int64) int: argument 1: value too large to convert (exceeds limit of 10 elements)"},
		{`box.Items = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]`, `cannot set field "Items": value too large to convert (exceeds limit of 10 elements)`},
	}
	for _, tt := range tests {
		caught, err := Eval(ctx, "try { "+tt.src+" } catch e { [e.kind(), e.message()] }", opts...)
		assert.Nil(t, err, tt.src)
		assert.Equal(t, caught, []any{"value error", tt.msg}, tt.src)

		_, err = Eval(ctx, tt.src, opts...)
		var valueErr *object.ValueError
		assert.True(t, errors.As(err, &valueErr), tt.src)
		var se *object.StructuredError
		assert.True(t, errors.As(err, &se), tt.src)
		assert.Equal(t, se.Kind, object.ErrValue, tt.src)
		assert.Equal(t, se.Message, tt.msg, tt.src)
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
