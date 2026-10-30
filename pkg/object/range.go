package object

import (
	"context"
	"fmt"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

var rangeAttrs = NewAttrRegistry[*Range]("range")

func init() {
	rangeAttrs.Define("start").
		Doc("The start value of the range").
		Returns("int").
		Getter(func(r *Range) Object {
			return NewInt(r.start)
		})

	rangeAttrs.Define("stop").
		Doc("The stop value of the range (exclusive)").
		Returns("int").
		Getter(func(r *Range) Object {
			return NewInt(r.stop)
		})

	rangeAttrs.Define("step").
		Doc("The step value of the range").
		Returns("int").
		Getter(func(r *Range) Object {
			return NewInt(r.step)
		})

	rangeAttrs.Define("map").
		Doc("Apply function to each value, return new list").
		Arg("fn").
		Returns("list").
		Impl(func(r *Range, ctx context.Context, args ...Object) (Object, error) {
			return r.Map(ctx, args[0])
		})

	rangeAttrs.Define("filter").
		Doc("Return list of values where function returns true").
		Arg("fn").
		Returns("list").
		Impl(func(r *Range, ctx context.Context, args ...Object) (Object, error) {
			return r.Filter(ctx, args[0])
		})

	rangeAttrs.Define("each").
		Doc("Call function for each value").
		Arg("fn").
		Returns("null").
		Impl(func(r *Range, ctx context.Context, args ...Object) (Object, error) {
			return r.Each(ctx, args[0])
		})
}

// Range represents a lazy sequence of integers, similar to Python's range.
// It stores start, stop, and step values and generates integers on demand.
type Range struct {
	start int64
	stop  int64
	step  int64
}

// NewRange creates a new Range object. Panics if step is zero.
func NewRange(start, stop, step int64) *Range {
	if step == 0 {
		panic("range step cannot be zero")
	}
	return &Range{start: start, stop: stop, step: step}
}

func (r *Range) Attrs() []AttrSpec {
	return rangeAttrs.Specs()
}

func (r *Range) GetAttr(name string) (Object, bool) {
	return rangeAttrs.GetAttr(r, name)
}

func (r *Range) SetAttr(name string, value Object) error {
	return fmt.Errorf("attribute error: range object does not support attribute assignment")
}

func (r *Range) Type() Type { return RANGE }

func (r *Range) Inspect() string {
	if r.step == 1 {
		if r.start == 0 {
			return fmt.Sprintf("range(%d)", r.stop)
		}
		return fmt.Sprintf("range(%d, %d)", r.start, r.stop)
	}
	return fmt.Sprintf("range(%d, %d, %d)", r.start, r.stop, r.step)
}

// Interface returns the range values as a []int64. It has no context, so it
// cannot see configured limits: ranges longer than DefaultMaxListLength are
// not materialized and return nil. Use ToInterface to apply the limit in a
// context and get a value error instead.
func (r *Range) Interface() any {
	return interfaceOrNil(r)
}

func (r *Range) toInterface(b *convBudget) (any, error) {
	n := r.length()
	if err := b.take(n); err != nil {
		return nil, err
	}
	return r.values(n), nil
}

// values returns the first n values of the range. n must not exceed the
// range length.
func (r *Range) values(n uint64) []int64 {
	result := make([]int64, 0, n)
	value := r.start
	for i := uint64(0); i < n; i++ {
		result = append(result, value)
		value += r.step
	}
	return result
}

func (r *Range) IsTruthy() bool {
	return r.length() > 0
}

func (r *Range) Equals(other Object) bool {
	otherRange, ok := other.(*Range)
	if !ok {
		return false
	}
	// Two ranges are equal if they produce the same sequence
	// Empty ranges are equal regardless of start/stop/step
	if r.length() == 0 && otherRange.length() == 0 {
		return true
	}
	return r.start == otherRange.start &&
		r.stop == otherRange.stop &&
		r.step == otherRange.step
}

func (r *Range) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for range: %v", opType)
}

// length returns the number of elements in the range. The arithmetic is done
// in uint64 so that ranges spanning more than math.MaxInt64 (or with a step of
// math.MinInt64) do not overflow.
func (r *Range) length() uint64 {
	if r.step > 0 {
		if r.start >= r.stop {
			return 0
		}
		diff := uint64(r.stop) - uint64(r.start)
		return (diff-1)/uint64(r.step) + 1
	}
	// step < 0
	if r.start <= r.stop {
		return 0
	}
	diff := uint64(r.start) - uint64(r.stop)
	return (diff-1)/(-uint64(r.step)) + 1
}

// Enumerate iterates over the range values. It iterates by element count, so
// values near the int64 limits do not wrap around, and stops early if ctx is
// canceled. Callers should check ctx.Err() after Enumerate returns.
func (r *Range) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	n := r.length()
	val := r.start
	for i := uint64(0); i < n; i++ {
		if i%ctxCheckInterval == 0 && i > 0 && ctx.Err() != nil {
			return
		}
		if !fn(NewInt(int64(i)), NewInt(val)) {
			return
		}
		val += r.step // may wrap after the last element; never used then
	}
}

func (r *Range) Map(ctx context.Context, fn Object) (Object, error) {
	callable, ok := fn.(Callable)
	if !ok {
		return nil, newTypeErrorf("range.map() expected a function (%s given)", fn.Type())
	}
	var passIndex bool
	if closure, ok := fn.(*Closure); ok {
		count := closure.ParameterCount()
		if count < 1 || count > 2 {
			return nil, newTypeErrorf("range.map() received an incompatible function")
		}
		passIndex = count == 2
	}
	n := r.length()
	if maxLen := GetLimits(ctx).MaxListLength; n > uint64(maxLen) {
		return nil, listTooLargeError(maxLen)
	}
	result := make([]Object, 0, n)
	var callErr error
	r.Enumerate(ctx, func(key, value Object) bool {
		var outputValue Object
		var err error
		if passIndex {
			outputValue, err = callable.Call(ctx, key, value)
		} else {
			outputValue, err = callable.Call(ctx, value)
		}
		if err != nil {
			callErr = err
			return false
		}
		result = append(result, outputValue)
		return true
	})
	if callErr != nil {
		return nil, callErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return NewList(result), nil
}

func (r *Range) Filter(ctx context.Context, fn Object) (Object, error) {
	callable, ok := fn.(Callable)
	if !ok {
		return nil, newTypeErrorf("range.filter() expected a function (%s given)", fn.Type())
	}
	var result []Object
	var callErr error
	r.Enumerate(ctx, func(key, value Object) bool {
		decision, err := callable.Call(ctx, value)
		if err != nil {
			callErr = err
			return false
		}
		if decision.IsTruthy() {
			if maxLen := GetLimits(ctx).MaxListLength; int64(len(result)) >= maxLen {
				callErr = listTooLargeError(maxLen)
				return false
			}
			result = append(result, value)
		}
		return true
	})
	if callErr != nil {
		return nil, callErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return NewList(result), nil
}

func (r *Range) Each(ctx context.Context, fn Object) (Object, error) {
	callable, ok := fn.(Callable)
	if !ok {
		return nil, newTypeErrorf("range.each() expected a function (%s given)", fn.Type())
	}
	var callErr error
	r.Enumerate(ctx, func(key, value Object) bool {
		if _, err := callable.Call(ctx, value); err != nil {
			callErr = err
			return false
		}
		return true
	})
	if callErr != nil {
		return nil, callErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return Nil, nil
}

// Start returns the start value.
func (r *Range) Start() int64 { return r.start }

// Stop returns the stop value.
func (r *Range) Stop() int64 { return r.stop }

// Step returns the step value.
func (r *Range) Step() int64 { return r.step }
