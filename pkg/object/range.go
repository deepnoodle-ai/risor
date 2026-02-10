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
		Returns("nil").
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

func (r *Range) Interface() any {
	// Return a slice of the range values
	var result []int64
	r.Enumerate(context.Background(), func(key, value Object) bool {
		result = append(result, value.(*Int).Value())
		return true
	})
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

// length returns the number of elements in the range.
func (r *Range) length() int64 {
	if r.step > 0 {
		if r.start >= r.stop {
			return 0
		}
		return (r.stop - r.start + r.step - 1) / r.step
	}
	// step < 0
	if r.start <= r.stop {
		return 0
	}
	return (r.start - r.stop - r.step - 1) / (-r.step)
}

// Enumerate iterates over the range values.
func (r *Range) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	idx := int64(0)
	if r.step > 0 {
		for val := r.start; val < r.stop; val += r.step {
			if !fn(NewInt(idx), NewInt(val)) {
				return
			}
			idx++
		}
	} else {
		for val := r.start; val > r.stop; val += r.step {
			if !fn(NewInt(idx), NewInt(val)) {
				return
			}
			idx++
		}
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
	result := make([]Object, 0, r.length())
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
			result = append(result, value)
		}
		return true
	})
	if callErr != nil {
		return nil, callErr
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
	return Nil, nil
}

// Start returns the start value.
func (r *Range) Start() int64 { return r.start }

// Stop returns the stop value.
func (r *Range) Stop() int64 { return r.stop }

// Step returns the step value.
func (r *Range) Step() int64 { return r.step }
