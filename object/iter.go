package object

import (
	"context"
	"fmt"

	"github.com/deepnoodle-ai/risor/v2/op"
)

// ITER type constant
const ITER Type = "iter"

// Iter is a lazy iterator that wraps a generator function.
// It implements Enumerable so it can be used with spread, list(), etc.
type Iter struct {
	// description for Inspect/debugging
	desc string

	// generator yields key-value pairs to the callback.
	// Return false from the callback to stop iteration.
	generator func(ctx context.Context, fn func(key, value Object) bool)
}

func (it *Iter) Type() Type {
	return ITER
}

func (it *Iter) Inspect() string {
	return fmt.Sprintf("iter(%s)", it.desc)
}

func (it *Iter) String() string {
	return it.Inspect()
}

func (it *Iter) Interface() any {
	// Collect to slice for Go interop
	var items []any
	it.Enumerate(context.Background(), func(key, value Object) bool {
		items = append(items, value.Interface())
		return true
	})
	return items
}

func (it *Iter) Equals(other Object) bool {
	// Iterators are only equal to themselves
	return it == other
}

func (it *Iter) Attrs() []AttrSpec {
	return nil
}

func (it *Iter) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (it *Iter) SetAttr(name string, value Object) error {
	return fmt.Errorf("iter has no attribute %q", name)
}

func (it *Iter) IsTruthy() bool {
	return true
}

func (it *Iter) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, fmt.Errorf("unsupported operation for iter: %v", opType)
}

// Enumerate implements Enumerable, allowing Iter to be used with spread, list(), etc.
func (it *Iter) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	it.generator(ctx, fn)
}

// NewIter creates a new iterator with a description and generator function.
func NewIter(desc string, gen func(ctx context.Context, fn func(key, value Object) bool)) *Iter {
	return &Iter{
		desc:      desc,
		generator: gen,
	}
}

// NewMapKeyIter creates an iterator over map keys.
func NewMapKeyIter(m *Map) *Iter {
	return NewIter("map.keys", func(ctx context.Context, fn func(key, value Object) bool) {
		for i, k := range m.SortedKeys() {
			if ctx.Err() != nil {
				return
			}
			if !fn(NewInt(int64(i)), NewString(k)) {
				return
			}
		}
	})
}

// NewMapValueIter creates an iterator over map values.
func NewMapValueIter(m *Map) *Iter {
	return NewIter("map.values", func(ctx context.Context, fn func(key, value Object) bool) {
		for i, k := range m.SortedKeys() {
			if ctx.Err() != nil {
				return
			}
			if !fn(NewInt(int64(i)), m.items[k]) {
				return
			}
		}
	})
}

// NewMapItemIter creates an iterator over map [key, value] pairs.
func NewMapItemIter(m *Map) *Iter {
	return NewIter("map.entries", func(ctx context.Context, fn func(key, value Object) bool) {
		for i, k := range m.SortedKeys() {
			if ctx.Err() != nil {
				return
			}
			pair := NewList([]Object{NewString(k), m.items[k]})
			if !fn(NewInt(int64(i)), pair) {
				return
			}
		}
	})
}
