// Package alloc asks one question: what does an allocation budget cost on
// the hot path, when every list, map and string the evaluator creates is
// charged in units before its memory is taken?
//
// The unit: a list costs 1 + its length, a map 1 + twice its entries, and a
// string 1 + its size in blocks (lab/textsize). A number, a bool and nil
// cost nothing. Text work also costs steps in proportion to its size.
//
// Each expression is built twice: charging steps only, and charging steps
// and units. A filter's output size is known only after its predicate has
// run, so it is built three ways: growing the output as it goes, growing it
// and charging each element before it is appended, and recording the
// predicate's results in a bitmap so that the exact size is charged and
// allocated once.
//
//	GOWORK=off go test ./lab/alloc -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package alloc

import (
	"errors"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/experiments/v3/lab/textsize"
)

var (
	errSteps = errors.New("step budget exceeded")
	errUnits = errors.New("allocation budget exceeded")
	errType  = errors.New("type error")
)

type runtime struct {
	globals []any
	slots   []any
	steps   int64    // steps left
	units   int64    // allocation units left
	bits    []uint64 // scratch for filter
	charge  bool     // charge allocation units and text size
}

func (rt *runtime) step(n int64) error {
	rt.steps -= n
	if rt.steps < 0 {
		return errSteps
	}
	return nil
}

// alloc charges n units. It is called before the memory is taken.
func (rt *runtime) alloc(n int64) error {
	rt.units -= n
	if rt.units < 0 {
		return errUnits
	}
	return nil
}

type node func(*runtime) (any, error)

func glob(i int) node  { return func(rt *runtime) (any, error) { return rt.globals[i], nil } }
func slot(i int) node  { return func(rt *runtime) (any, error) { return rt.slots[i], nil } }
func konst(v any) node { return func(*runtime) (any, error) { return v, nil } }

func greater(x, y node) node {
	return func(rt *runtime) (any, error) {
		a, err := x(rt)
		if err != nil {
			return nil, err
		}
		b, err := y(rt)
		if err != nil {
			return nil, err
		}
		if err := rt.step(1); err != nil {
			return nil, err
		}
		if a, ok := a.(int64); ok {
			if b, ok := b.(int64); ok {
				return a > b, nil
			}
		}
		return nil, errType
	}
}

// add is + on ints and on strings. A string result is charged by size.
func add(x, y node) node {
	return func(rt *runtime) (any, error) {
		a, err := x(rt)
		if err != nil {
			return nil, err
		}
		b, err := y(rt)
		if err != nil {
			return nil, err
		}
		switch a := a.(type) {
		case int64:
			if b, ok := b.(int64); ok {
				if err := rt.step(1); err != nil {
					return nil, err
				}
				return a + b, nil
			}
		case string:
			if b, ok := b.(string); ok {
				if !rt.charge {
					if err := rt.step(1); err != nil {
						return nil, err
					}
					return a + b, nil
				}
				n := textsize.BlocksOf2(a, b)
				if err := rt.step(1 + n); err != nil {
					return nil, err
				}
				if err := rt.alloc(1 + n); err != nil {
					return nil, err
				}
				return a + b, nil
			}
		}
		return nil, errType
	}
}

// list is a list literal.
func list(items ...node) node {
	return func(rt *runtime) (any, error) {
		if rt.charge {
			if err := rt.alloc(1 + int64(len(items))); err != nil {
				return nil, err
			}
		}
		out := make([]any, len(items))
		for i, item := range items {
			v, err := item(rt)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}
}

// mapList is list.map(x => body), with x in slot 0. The output size is the
// input size, so it is charged before the output is made.
func mapList(src, body node) node {
	return func(rt *runtime) (any, error) {
		l, err := src(rt)
		if err != nil {
			return nil, err
		}
		items := l.([]any)
		if rt.charge {
			if err := rt.alloc(1 + int64(len(items))); err != nil {
				return nil, err
			}
		}
		out := make([]any, len(items))
		for i, item := range items {
			if err := rt.step(2); err != nil { // the element and the call
				return nil, err
			}
			rt.slots[0] = item
			if out[i], err = body(rt); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
}

// filterGrow is list.filter(x => body) with append. With charging on, each
// kept element is charged before it is appended, but append takes memory
// ahead of the charge when it doubles the capacity.
func filterGrow(src, body node) node {
	return func(rt *runtime) (any, error) {
		l, err := src(rt)
		if err != nil {
			return nil, err
		}
		if rt.charge {
			if err := rt.alloc(1); err != nil {
				return nil, err
			}
		}
		out := []any{}
		for _, item := range l.([]any) {
			if err := rt.step(2); err != nil {
				return nil, err
			}
			rt.slots[0] = item
			keep, err := body(rt)
			if err != nil {
				return nil, err
			}
			if keep == true {
				if rt.charge {
					if err := rt.alloc(1); err != nil {
						return nil, err
					}
				}
				out = append(out, item)
			}
		}
		return out, nil
	}
}

// filterExact records each result of the predicate in a bitmap, which is
// scratch space of one bit per input element. Then it charges the exact
// output size and allocates it once.
func filterExact(src, body node) node {
	return func(rt *runtime) (any, error) {
		l, err := src(rt)
		if err != nil {
			return nil, err
		}
		items := l.([]any)
		words := (len(items) + 63) / 64
		if cap(rt.bits) < words {
			rt.bits = make([]uint64, words)
		}
		bits := rt.bits[:words]
		clear(bits)
		kept := 0
		for i, item := range items {
			if err := rt.step(2); err != nil {
				return nil, err
			}
			rt.slots[0] = item
			keep, err := body(rt)
			if err != nil {
				return nil, err
			}
			if keep == true {
				bits[i/64] |= 1 << (i % 64)
				kept++
			}
		}
		if rt.charge {
			if err := rt.alloc(1 + int64(kept)); err != nil {
				return nil, err
			}
		}
		out := make([]any, 0, kept)
		for i, item := range items {
			if bits[i/64]&(1<<(i%64)) != 0 {
				out = append(out, item)
			}
		}
		return out, nil
	}
}

// join is list.join(sep). The size of the result is the sum of the weights
// of the pieces, so charging it means reading every piece once more.
func join(src node, sep string) node {
	return func(rt *runtime) (any, error) {
		l, err := src(rt)
		if err != nil {
			return nil, err
		}
		items := l.([]any)
		parts := make([]string, len(items)) // scratch, one slot per element
		bytes, weight := 0, 0
		for i, item := range items {
			s, ok := item.(string)
			if !ok {
				return nil, errType
			}
			parts[i] = s
			bytes += len(s) + len(sep)
		}
		if err := rt.step(int64(len(items))); err != nil {
			return nil, err
		}
		if rt.charge {
			var n int64
			if bytes >= textsize.Block {
				for _, s := range parts {
					weight += textsize.Weight(s)
				}
				weight += textsize.Weight(sep) * max(len(parts)-1, 0)
				n = int64(weight / textsize.Block)
			}
			if err := rt.step(n); err != nil {
				return nil, err
			}
			if err := rt.alloc(1 + n); err != nil {
				return nil, err
			}
		}
		return strings.Join(parts, sep), nil
	}
}
