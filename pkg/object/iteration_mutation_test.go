package object

import (
	"context"
	"errors"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Tests for changes to maps and lists during iteration, and for context
// checks in container iteration.

// callback wraps a Go function as a builtin for use with List/Map methods.
func callback(fn func(args ...Object) Object) *Builtin {
	return NewBuiltin("callback", func(ctx context.Context, args ...Object) (Object, error) {
		return fn(args...), nil
	})
}

func testMap() *Map {
	return NewMap(map[string]Object{"a": NewInt(1), "b": NewInt(2), "c": NewInt(3)})
}

func callMapMethod(t *testing.T, m *Map, name string, args ...Object) (Object, error) {
	t.Helper()
	method, ok := m.GetAttr(name)
	assert.True(t, ok)
	return method.(Callable).Call(context.Background(), args...)
}

func TestMapEachDeleteDuringIteration(t *testing.T) {
	m := testMap()
	var seen []Object
	_, err := callMapMethod(t, m, "each", callback(func(args ...Object) Object {
		seen = append(seen, args[1])
		m.Delete("b")
		m.Delete("c")
		return Nil
	}))
	assert.Nil(t, err)
	assert.Equal(t, seen, []Object{NewInt(1)})
}

func TestMapEnumerateDeleteDuringIteration(t *testing.T) {
	m := testMap()
	var keys []string
	m.Enumerate(context.Background(), func(k, v Object) bool {
		assert.NotNil(t, v)
		keys = append(keys, k.(*String).value)
		m.Clear()
		return true
	})
	assert.Equal(t, keys, []string{"a"})

	// Values seen are current, and added keys are not visited
	m = testMap()
	var values []Object
	m.Enumerate(context.Background(), func(k, v Object) bool {
		values = append(values, v)
		m.Set("b", NewInt(20))
		m.Set("z", NewInt(26))
		return true
	})
	assert.Equal(t, values, []Object{NewInt(1), NewInt(20), NewInt(3)})
}

func TestMapIteratorsDeleteDuringIteration(t *testing.T) {
	ctx := context.Background()
	for _, it := range []func(*Map) *Iter{NewMapKeyIter, NewMapValueIter, NewMapItemIter} {
		m := testMap()
		var indexes []Object
		it(m).Enumerate(ctx, func(k, v Object) bool {
			assert.NotNil(t, v)
			if pair, ok := v.(*List); ok {
				assert.NotNil(t, pair.items[1])
			}
			indexes = append(indexes, k)
			m.Delete("b")
			return true
		})
		// Indexes stay contiguous after skipping a deleted key
		assert.Equal(t, indexes, []Object{NewInt(0), NewInt(1)})
	}

	// Collecting an iterator after clearing mid-iteration yields no raw nils
	m := testMap()
	items := NewMapItemIter(m)
	var collected []Object
	items.Enumerate(ctx, func(k, v Object) bool {
		m.Clear()
		collected = append(collected, v)
		return true
	})
	assert.Len(t, collected, 1)
	_, err := ToInterface(NewList(collected))
	assert.Nil(t, err)
}

func TestListRemoveClearsTailSlot(t *testing.T) {
	ls := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	backing := ls.items
	_, err := ls.Pop(0)
	assert.Nil(t, err)
	assert.Nil(t, backing[2])
	assert.Equal(t, ls.items, []Object{NewInt(2), NewInt(3)})

	ls = NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	backing = ls.items
	ls.Remove(NewInt(2))
	assert.Nil(t, backing[2])
	assert.Equal(t, ls.items, []Object{NewInt(1), NewInt(3)})

	ls = NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	backing = ls.items
	assert.Nil(t, ls.DelItem(NewInt(-1)))
	assert.Nil(t, backing[2])
	assert.Equal(t, ls.items, []Object{NewInt(1), NewInt(2)})
}

func TestListEachMutateDuringIteration(t *testing.T) {
	ctx := context.Background()

	// Popping the front: each step reads the element now at that index and
	// iteration stops once the index passes the end of the shrunk list.
	ls := NewList([]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4)})
	var seen []Object
	_, err := ls.Each(ctx, callback(func(args ...Object) Object {
		seen = append(seen, args[0])
		_, _ = ls.Pop(0)
		return Nil
	}))
	assert.Nil(t, err)
	assert.Equal(t, seen, []Object{NewInt(1), NewInt(3)})

	// Appending: new elements are not visited, so this terminates
	ls = NewList([]Object{NewInt(1), NewInt(2)})
	seen = nil
	_, err = ls.Each(ctx, callback(func(args ...Object) Object {
		seen = append(seen, args[0])
		ls.Append(args[0])
		return Nil
	}))
	assert.Nil(t, err)
	assert.Equal(t, seen, []Object{NewInt(1), NewInt(2)})
	assert.Equal(t, ls.Size(), 4)

	// Clearing stops iteration and never yields nil
	ls = NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	mapped, err := ls.Map(ctx, callback(func(args ...Object) Object {
		ls.Clear()
		return args[0]
	}))
	assert.Nil(t, err)
	assert.Equal(t, mapped.(*List).items, []Object{NewInt(1)})

	ls = NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	filtered, err := ls.Filter(ctx, callback(func(args ...Object) Object {
		assert.NotNil(t, args[0])
		_, _ = ls.Pop(-1)
		return True
	}))
	assert.Nil(t, err)
	assert.Equal(t, filtered.(*List).items, []Object{NewInt(1), NewInt(2)})

	ls = NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	sum, err := ls.Reduce(ctx, NewInt(0), callback(func(args ...Object) Object {
		_, _ = ls.Pop(-1)
		return NewInt(args[0].(*Int).value + args[1].(*Int).value)
	}))
	assert.Nil(t, err)
	assert.Equal(t, sum, NewInt(3))

	ls = NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	var enumerated []Object
	ls.Enumerate(ctx, func(k, v Object) bool {
		assert.NotNil(t, v)
		enumerated = append(enumerated, v)
		ls.Clear()
		return true
	})
	assert.Equal(t, enumerated, []Object{NewInt(1)})
}

func TestContainerIterationContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	noop := callback(func(args ...Object) Object { return True })

	ls := NewList([]Object{NewInt(1), NewInt(2)})
	_, err := ls.Each(ctx, noop)
	assert.True(t, errors.Is(err, context.Canceled))
	_, err = ls.Map(ctx, noop)
	assert.True(t, errors.Is(err, context.Canceled))
	_, err = ls.Filter(ctx, noop)
	assert.True(t, errors.Is(err, context.Canceled))
	_, err = ls.Reduce(ctx, Nil, noop)
	assert.True(t, errors.Is(err, context.Canceled))

	calls := 0
	ls.Enumerate(ctx, func(k, v Object) bool { calls++; return true })
	assert.Equal(t, calls, 0)

	m := testMap()
	method, _ := m.GetAttr("each")
	_, err = method.(Callable).Call(ctx, noop)
	assert.True(t, errors.Is(err, context.Canceled))
	m.Enumerate(ctx, func(k, v Object) bool { calls++; return true })
	assert.Equal(t, calls, 0)

	// Cancellation mid-iteration stops at the next step
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	ls = NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	_, err = ls.Each(ctx, callback(func(args ...Object) Object {
		calls++
		cancel()
		return Nil
	}))
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, calls, 1)
}
