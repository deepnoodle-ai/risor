package object

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestIterType(t *testing.T) {
	it := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {})
	assert.Equal(t, it.Type(), ITER)
}

func TestIterInspect(t *testing.T) {
	it := NewIter("test.iter", func(ctx context.Context, fn func(key, value Object) bool) {})
	assert.Equal(t, it.Inspect(), "iter(test.iter)")
}

func TestIterEnumerate(t *testing.T) {
	ctx := context.Background()
	it := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {
		fn(NewInt(0), NewString("a"))
		fn(NewInt(1), NewString("b"))
		fn(NewInt(2), NewString("c"))
	})

	var values []string
	it.Enumerate(ctx, func(key, value Object) bool {
		values = append(values, value.(*String).Value())
		return true
	})

	assert.Equal(t, values, []string{"a", "b", "c"})
}

func TestIterEnumerateEarlyStop(t *testing.T) {
	ctx := context.Background()
	it := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {
		if !fn(NewInt(0), NewString("a")) {
			return
		}
		if !fn(NewInt(1), NewString("b")) {
			return
		}
		fn(NewInt(2), NewString("c"))
	})

	var count int
	it.Enumerate(ctx, func(key, value Object) bool {
		count++
		return count < 2 // Stop after 2nd iteration
	})

	assert.Equal(t, count, 2)
}

func TestIterInterface(t *testing.T) {
	it := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {
		fn(NewInt(0), NewInt(1))
		fn(NewInt(1), NewInt(2))
	})

	iface := it.Interface()
	result := iface.([]any)
	assert.Len(t, result, 2)
	assert.Equal(t, result[0], int64(1))
	assert.Equal(t, result[1], int64(2))
}

func TestIterEquals(t *testing.T) {
	it1 := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {})
	it2 := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {})

	// Only equal to itself
	assert.True(t, it1.Equals(it1))
	assert.False(t, it1.Equals(it2))
	assert.False(t, it1.Equals(NewString("test")))
}

func TestIterIsTruthy(t *testing.T) {
	it := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {})
	assert.True(t, it.IsTruthy())
}

func TestIterAttrs(t *testing.T) {
	it := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {})
	assert.Nil(t, it.Attrs())
}

func TestIterGetAttr(t *testing.T) {
	it := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {})
	_, ok := it.GetAttr("anything")
	assert.False(t, ok)
}

func TestIterSetAttr(t *testing.T) {
	it := NewIter("test", func(ctx context.Context, fn func(key, value Object) bool) {})
	err := it.SetAttr("anything", NewInt(1))
	assert.NotNil(t, err)
}

func TestMapKeyIter(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
		"c": NewInt(3),
	})

	it := NewMapKeyIter(m)
	assert.Equal(t, it.Inspect(), "iter(map.keys)")

	var keys []string
	it.Enumerate(ctx, func(key, value Object) bool {
		keys = append(keys, value.(*String).Value())
		return true
	})

	// Keys are sorted
	assert.Equal(t, keys, []string{"a", "b", "c"})
}

func TestMapValueIter(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
		"c": NewInt(3),
	})

	it := NewMapValueIter(m)
	assert.Equal(t, it.Inspect(), "iter(map.values)")

	var values []int64
	it.Enumerate(ctx, func(key, value Object) bool {
		values = append(values, value.(*Int).Value())
		return true
	})

	// Values in sorted key order
	assert.Equal(t, values, []int64{1, 2, 3})
}

func TestMapItemIter(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	it := NewMapItemIter(m)
	assert.Equal(t, it.Inspect(), "iter(map.entries)")

	var items [][]any
	it.Enumerate(ctx, func(key, value Object) bool {
		pair := value.(*List).Value()
		items = append(items, []any{
			pair[0].(*String).Value(),
			pair[1].(*Int).Value(),
		})
		return true
	})

	// Items in sorted key order
	assert.Len(t, items, 2)
	assert.Equal(t, items[0], []any{"a", int64(1)})
	assert.Equal(t, items[1], []any{"b", int64(2)})
}
