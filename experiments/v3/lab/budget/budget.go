// Package budget asks one question: what does counting steps cost per
// operation, and what is the cheapest way to also notice a canceled context?
//
// One expression and one loop are built five times, differing only in what
// each operator does before it computes:
//
//   - none:     nothing
//
//   - up:       steps++, compare with the limit (the spike)
//
//   - down:     left--, compare with zero
//
//   - chunk:    left--, compare with zero; when a chunk of 1,024 steps runs
//     out, check the context and the total, then refill
//
//   - ctx_each: ctx.Err() and a counter on every operator (expr)
//
// A sixth run, chunk_clock, is chunk with a deadline: the slow path also
// reads the clock, as an implementation with no cancellation signal must.
//
//	GOWORK=off go test ./lab/budget -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package budget

import (
	"context"
	"errors"
	"time"
)

var (
	errBudget   = errors.New("step budget exceeded")
	errDeadline = errors.New("deadline exceeded")
	errType     = errors.New("type error")
)

type mode int

const (
	none mode = iota
	up
	down
	chunk
	ctxEach
)

const chunkSize = 1024

type runtime struct {
	ctx      context.Context
	done     <-chan struct{}
	deadline time.Time // zero means none
	globals  []any
	slots    []any

	steps, max int64 // up
	left       int64 // down, chunk, ctx_each
	remaining  int64 // chunk: steps not yet handed to left
}

// reset prepares the runtime for one evaluation with a budget of n steps.
func (rt *runtime) reset(n int64) {
	rt.steps, rt.max = 0, n
	rt.left = n
	rt.remaining = 0
}

func (rt *runtime) resetChunk(n int64) {
	rt.left = min(n, chunkSize)
	rt.remaining = n - rt.left
}

// refill runs when a chunk is used up: once per 1,024 steps.
func (rt *runtime) refill() error {
	if rt.done != nil {
		select {
		case <-rt.done:
			return rt.ctx.Err()
		default:
		}
	}
	if !rt.deadline.IsZero() && time.Now().After(rt.deadline) {
		return errDeadline
	}
	if rt.remaining <= 0 {
		return errBudget
	}
	n := min(rt.remaining, chunkSize)
	rt.remaining -= n
	rt.left = n - 1 // the step that triggered the refill is charged
	return nil
}

type node func(*runtime) (any, error)

func glob(i int) node  { return func(rt *runtime) (any, error) { return rt.globals[i], nil } }
func slot(i int) node  { return func(rt *runtime) (any, error) { return rt.slots[i], nil } }
func konst(v any) node { return func(*runtime) (any, error) { return v, nil } }

func addValues(a, b any) (any, error) {
	if a, ok := a.(int64); ok {
		if b, ok := b.(int64); ok {
			return a + b, nil
		}
	}
	return nil, errType
}

func lessValues(a, b any) (any, error) {
	if a, ok := a.(int64); ok {
		if b, ok := b.(int64); ok {
			return a < b, nil
		}
	}
	return nil, errType
}

// binary builds an operator node. The five closures differ only in the
// charge between evaluating the operands and applying op.
func binary(m mode, x, y node, op func(a, b any) (any, error)) node {
	switch m {
	case none:
		return func(rt *runtime) (any, error) {
			a, err := x(rt)
			if err != nil {
				return nil, err
			}
			b, err := y(rt)
			if err != nil {
				return nil, err
			}
			return op(a, b)
		}
	case up:
		return func(rt *runtime) (any, error) {
			a, err := x(rt)
			if err != nil {
				return nil, err
			}
			b, err := y(rt)
			if err != nil {
				return nil, err
			}
			rt.steps++
			if rt.steps > rt.max {
				return nil, errBudget
			}
			return op(a, b)
		}
	case down:
		return func(rt *runtime) (any, error) {
			a, err := x(rt)
			if err != nil {
				return nil, err
			}
			b, err := y(rt)
			if err != nil {
				return nil, err
			}
			rt.left--
			if rt.left < 0 {
				return nil, errBudget
			}
			return op(a, b)
		}
	case chunk:
		return func(rt *runtime) (any, error) {
			a, err := x(rt)
			if err != nil {
				return nil, err
			}
			b, err := y(rt)
			if err != nil {
				return nil, err
			}
			rt.left--
			if rt.left < 0 {
				if err := rt.refill(); err != nil {
					return nil, err
				}
			}
			return op(a, b)
		}
	case ctxEach:
		return func(rt *runtime) (any, error) {
			a, err := x(rt)
			if err != nil {
				return nil, err
			}
			b, err := y(rt)
			if err != nil {
				return nil, err
			}
			if err := rt.ctx.Err(); err != nil {
				return nil, err
			}
			rt.left--
			if rt.left < 0 {
				return nil, errBudget
			}
			return op(a, b)
		}
	}
	panic("mode")
}

// count is list.count(pred), with the item in slot 0. It charges one step
// per item, in the same way as the operators.
func count(m mode, list, pred node) node {
	return func(rt *runtime) (any, error) {
		l, err := list(rt)
		if err != nil {
			return nil, err
		}
		var n int64
		for _, item := range l.([]any) {
			switch m {
			case up:
				rt.steps++
				if rt.steps > rt.max {
					return nil, errBudget
				}
			case down:
				rt.left--
				if rt.left < 0 {
					return nil, errBudget
				}
			case chunk:
				rt.left--
				if rt.left < 0 {
					if err := rt.refill(); err != nil {
						return nil, err
					}
				}
			case ctxEach:
				if err := rt.ctx.Err(); err != nil {
					return nil, err
				}
				rt.left--
				if rt.left < 0 {
					return nil, errBudget
				}
			}
			rt.slots[0] = item
			v, err := pred(rt)
			if err != nil {
				return nil, err
			}
			if v == true {
				n++
			}
		}
		return n, nil
	}
}
