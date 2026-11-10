// Package hostcall asks one question: how should the evaluator call a Go
// function that the host registers?
//
// The options, from the least to the most machinery:
//
//   - native:  the host writes func(ctx, args []any) (any, error) itself
//
//   - adapter: a generic wrapper turns func(A, B) (R, error) into the native
//     signature with type assertions; no reflection runs per call
//
//   - convert: the same, with a converter per parameter chosen once, so an
//     int parameter accepts the evaluator's int64
//
//   - typed:   the wrapper is bound to typed argument closures, so nothing
//     is boxed on the way in or out
//
//   - reflect: reflect.Value.Call with the type information cached
//
//     GOWORK=off go test ./lab/hostcall -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package hostcall

import (
	"context"
	"fmt"
	"reflect"
)

type frame struct {
	ctx     context.Context
	globals []any
	args    []any // argument stack, reused across calls
}

type node func(*frame) any

func glob(i int) node { return func(fr *frame) any { return fr.globals[i] } }

// Func is the native signature. args is valid only during the call.
type Func func(ctx context.Context, args []any) (any, error)

// call evaluates the arguments onto the frame's argument stack and calls fn.
func call(fn Func, args ...node) node {
	return func(fr *frame) any {
		base := len(fr.args)
		for _, a := range args {
			fr.args = append(fr.args, a(fr))
		}
		out, err := fn(fr.ctx, fr.args[base:])
		clear(fr.args[base:])
		fr.args = fr.args[:base]
		if err != nil {
			panic(err)
		}
		return out
	}
}

// ---------------------------------------------------------------- adapter

func argError(i int, v any) error { return fmt.Errorf("argument %d: unexpected %T", i, v) }

// Adapt2 wraps a two-argument function. A and B must be types the evaluator
// produces: int64, float64, string, bool, []any, map[string]any or any.
func Adapt2[A, B, R any](fn func(A, B) (R, error)) Func {
	return func(_ context.Context, args []any) (any, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("want 2 arguments, got %d", len(args))
		}
		a, ok := args[0].(A)
		if !ok {
			return nil, argError(0, args[0])
		}
		b, ok := args[1].(B)
		if !ok {
			return nil, argError(1, args[1])
		}
		return fn(a, b)
	}
}

func Adapt3[A, B, C, R any](fn func(A, B, C) (R, error)) Func {
	return func(_ context.Context, args []any) (any, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("want 3 arguments, got %d", len(args))
		}
		a, ok := args[0].(A)
		if !ok {
			return nil, argError(0, args[0])
		}
		b, ok := args[1].(B)
		if !ok {
			return nil, argError(1, args[1])
		}
		c, ok := args[2].(C)
		if !ok {
			return nil, argError(2, args[2])
		}
		return fn(a, b, c)
	}
}

// ---------------------------------------------------------------- convert

// converter returns the function that turns an evaluator value into a T. It
// is chosen once, when the host registers the function. A type the evaluator
// produces is asserted. Go's other integer types are converted from int64.
func converter[T any]() func(any) (T, bool) {
	var zero T
	switch any(zero).(type) {
	case int:
		return func(v any) (T, bool) {
			i, ok := v.(int64)
			return any(int(i)).(T), ok
		}
	case int32:
		return func(v any) (T, bool) {
			i, ok := v.(int64)
			return any(int32(i)).(T), ok && int64(int32(i)) == i
		}
	}
	return func(v any) (T, bool) {
		t, ok := v.(T)
		return t, ok
	}
}

// result returns the function that turns a T back into an evaluator value.
func result[T any]() func(T) any {
	var zero T
	switch any(zero).(type) {
	case int:
		return func(v T) any { return int64(any(v).(int)) }
	}
	return func(v T) any { return v }
}

func Convert3[A, B, C, R any](fn func(A, B, C) (R, error)) Func {
	ca, cb, cc, cr := converter[A](), converter[B](), converter[C](), result[R]()
	return func(_ context.Context, args []any) (any, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("want 3 arguments, got %d", len(args))
		}
		a, ok := ca(args[0])
		if !ok {
			return nil, argError(0, args[0])
		}
		b, ok := cb(args[1])
		if !ok {
			return nil, argError(1, args[1])
		}
		c, ok := cc(args[2])
		if !ok {
			return nil, argError(2, args[2])
		}
		r, err := fn(a, b, c)
		if err != nil {
			return nil, err
		}
		return cr(r), nil
	}
}

// ------------------------------------------------------------------ typed

// Typed2 binds a function to typed argument closures at compile time. The
// compiler holds each argument as an any that contains a func(*frame) T. The
// assertions below run once, at compile time, and the returned closure calls
// fn with no boxing.
func Typed2[A, B, R any](fn func(A, B) (R, error)) func(args ...any) (any, bool) {
	return func(args ...any) (any, bool) {
		if len(args) != 2 {
			return nil, false
		}
		a, ok := args[0].(func(*frame) A)
		if !ok {
			return nil, false
		}
		b, ok := args[1].(func(*frame) B)
		if !ok {
			return nil, false
		}
		return func(fr *frame) R {
			r, err := fn(a(fr), b(fr))
			if err != nil {
				panic(err)
			}
			return r
		}, true
	}
}

func Typed3[A, B, C, R any](fn func(A, B, C) (R, error)) func(args ...any) (any, bool) {
	return func(args ...any) (any, bool) {
		if len(args) != 3 {
			return nil, false
		}
		a, ok := args[0].(func(*frame) A)
		if !ok {
			return nil, false
		}
		b, ok := args[1].(func(*frame) B)
		if !ok {
			return nil, false
		}
		c, ok := args[2].(func(*frame) C)
		if !ok {
			return nil, false
		}
		return func(fr *frame) R {
			r, err := fn(a(fr), b(fr), c(fr))
			if err != nil {
				panic(err)
			}
			return r
		}, true
	}
}

// ---------------------------------------------------------------- reflect

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Reflect wraps any function. The type information is read once. Each call
// converts the arguments, calls through reflect, and boxes the result. This
// is the shape of expr's callPreparedReflect and of Risor v2's GoFunc.
func Reflect(fn any) Func {
	fv := reflect.ValueOf(fn)
	ft := fv.Type()
	params := make([]reflect.Type, ft.NumIn())
	for i := range params {
		params[i] = ft.In(i)
	}
	hasErr := ft.NumOut() == 2 && ft.Out(1) == errorType
	return func(_ context.Context, args []any) (any, error) {
		if len(args) != len(params) {
			return nil, fmt.Errorf("want %d arguments, got %d", len(params), len(args))
		}
		in := make([]reflect.Value, len(args))
		for i, a := range args {
			v := reflect.ValueOf(a)
			if v.Type() != params[i] {
				if !v.CanConvert(params[i]) {
					return nil, argError(i, a)
				}
				v = v.Convert(params[i])
			}
			in[i] = v
		}
		out := fv.Call(in)
		if hasErr && !out[1].IsNil() {
			return nil, out[1].Interface().(error)
		}
		r := out[0]
		if r.CanInt() {
			return r.Int(), nil
		}
		return r.Interface(), nil
	}
}
