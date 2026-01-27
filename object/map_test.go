package object

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/op"
)

func TestMapType(t *testing.T) {
	m := NewMap(nil)
	assert.Equal(t, m.Type(), MAP)
}

func TestMapInspect(t *testing.T) {
	m := NewMap(map[string]Object{
		"a": NewInt(1),
		"b": NewString("hello"),
	})
	inspect := m.Inspect()
	// Keys are sorted
	assert.Equal(t, inspect, `{"a": 1, "b": "hello"}`)
}

func TestMapInspectEmpty(t *testing.T) {
	m := NewMap(nil)
	assert.Equal(t, m.Inspect(), "{}")
}

func TestMapInspectSelfReference(t *testing.T) {
	m := NewMap(nil)
	m.Set("self", m)
	// Should handle self-reference without infinite loop
	inspect := m.Inspect()
	assert.Equal(t, inspect, `{"self": {...}}`)
}

func TestMapString(t *testing.T) {
	m := NewMap(map[string]Object{"x": NewInt(42)})
	assert.Equal(t, m.String(), `{"x": 42}`)
}

func TestMapValue(t *testing.T) {
	items := map[string]Object{"key": NewInt(1)}
	m := NewMap(items)
	assert.Equal(t, m.Value(), items)
}

func TestMapSetAttr(t *testing.T) {
	m := NewMap(nil)
	err := m.SetAttr("key", NewInt(100))
	assert.Nil(t, err)
	val := m.Get("key")
	assert.Equal(t, val.(*Int).Value(), int64(100))
}

func TestMapGetAttrKeys(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	keys, ok := m.GetAttr("keys")
	assert.True(t, ok)
	result := keys.(*Builtin).Call(ctx)
	list := result.(*List)
	assert.Len(t, list.Value(), 2)
	// Keys are sorted
	assert.Equal(t, list.Value()[0].(*String).Value(), "a")
	assert.Equal(t, list.Value()[1].(*String).Value(), "b")
}

func TestMapGetAttrKeysError(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	keys, _ := m.GetAttr("keys")
	result := keys.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestMapGetAttrValues(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	values, ok := m.GetAttr("values")
	assert.True(t, ok)
	result := values.(*Builtin).Call(ctx)
	list := result.(*List)
	assert.Len(t, list.Value(), 2)
	// Values are in key-sorted order
	assert.Equal(t, list.Value()[0].(*Int).Value(), int64(1))
	assert.Equal(t, list.Value()[1].(*Int).Value(), int64(2))
}

func TestMapGetAttrValuesError(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	values, _ := m.GetAttr("values")
	result := values.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestMapGetAttrGet(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"key": NewInt(42)})

	get, ok := m.GetAttr("get")
	assert.True(t, ok)

	// Get existing key
	result := get.(*Builtin).Call(ctx, NewString("key"))
	assert.Equal(t, result.(*Int).Value(), int64(42))

	// Get missing key - returns nil
	result = get.(*Builtin).Call(ctx, NewString("missing"))
	assert.Equal(t, result, Nil)

	// Get missing key with default
	result = get.(*Builtin).Call(ctx, NewString("missing"), NewString("default"))
	assert.Equal(t, result.(*String).Value(), "default")
}

func TestMapGetAttrGetErrors(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	get, _ := m.GetAttr("get")

	// No args
	result := get.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	// Too many args
	result = get.(*Builtin).Call(ctx, NewString("a"), NewString("b"), NewString("c"))
	assert.True(t, IsError(result))

	// Wrong type for key
	result = get.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestMapGetAttrClear(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"key": NewInt(42)})

	clear, ok := m.GetAttr("clear")
	assert.True(t, ok)

	result := clear.(*Builtin).Call(ctx)
	assert.Equal(t, result, m)
	assert.Equal(t, m.Size(), 0)
}

func TestMapGetAttrClearError(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	clear, _ := m.GetAttr("clear")
	result := clear.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestMapGetAttrCopy(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"key": NewInt(42)})

	copyFn, ok := m.GetAttr("copy")
	assert.True(t, ok)

	result := copyFn.(*Builtin).Call(ctx)
	copyMap := result.(*Map)

	// Copy has same values
	assert.Equal(t, copyMap.Get("key").(*Int).Value(), int64(42))

	// But is a different object
	m.Set("key", NewInt(100))
	assert.Equal(t, copyMap.Get("key").(*Int).Value(), int64(42))
}

func TestMapGetAttrCopyError(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	copyFn, _ := m.GetAttr("copy")
	result := copyFn.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestMapGetAttrItems(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	items, ok := m.GetAttr("items")
	assert.True(t, ok)

	result := items.(*Builtin).Call(ctx)
	list := result.(*List)
	assert.Len(t, list.Value(), 2)

	// Items are key-sorted pairs
	pair0 := list.Value()[0].(*List).Value()
	assert.Equal(t, pair0[0].(*String).Value(), "a")
	assert.Equal(t, pair0[1].(*Int).Value(), int64(1))

	pair1 := list.Value()[1].(*List).Value()
	assert.Equal(t, pair1[0].(*String).Value(), "b")
	assert.Equal(t, pair1[1].(*Int).Value(), int64(2))
}

func TestMapGetAttrItemsError(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	items, _ := m.GetAttr("items")
	result := items.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestMapGetAttrPop(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"key": NewInt(42)})

	pop, ok := m.GetAttr("pop")
	assert.True(t, ok)

	// Pop existing key
	result := pop.(*Builtin).Call(ctx, NewString("key"))
	assert.Equal(t, result.(*Int).Value(), int64(42))
	assert.Equal(t, m.Size(), 0)

	// Pop missing key without default - returns nil
	result = pop.(*Builtin).Call(ctx, NewString("missing"))
	assert.Equal(t, result, Nil)

	// Pop missing key with default
	result = pop.(*Builtin).Call(ctx, NewString("missing"), NewString("default"))
	assert.Equal(t, result.(*String).Value(), "default")
}

func TestMapGetAttrPopErrors(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	pop, _ := m.GetAttr("pop")

	// No args
	result := pop.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	// Too many args
	result = pop.(*Builtin).Call(ctx, NewString("a"), NewString("b"), NewString("c"))
	assert.True(t, IsError(result))

	// Wrong type for key
	result = pop.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestMapGetAttrSetDefault(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"existing": NewInt(1)})

	setdefault, ok := m.GetAttr("setdefault")
	assert.True(t, ok)

	// Set default for missing key
	result := setdefault.(*Builtin).Call(ctx, NewString("new"), NewInt(99))
	assert.Equal(t, result.(*Int).Value(), int64(99))
	assert.Equal(t, m.Get("new").(*Int).Value(), int64(99))

	// Set default for existing key - returns existing value
	result = setdefault.(*Builtin).Call(ctx, NewString("existing"), NewInt(999))
	assert.Equal(t, result.(*Int).Value(), int64(1))
}

func TestMapGetAttrSetDefaultErrors(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	setdefault, _ := m.GetAttr("setdefault")

	// Wrong arg count
	result := setdefault.(*Builtin).Call(ctx, NewString("key"))
	assert.True(t, IsError(result))

	// Wrong type for key
	result = setdefault.(*Builtin).Call(ctx, NewInt(1), NewInt(2))
	assert.True(t, IsError(result))
}

func TestMapGetAttrUpdate(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"a": NewInt(1)})

	update, ok := m.GetAttr("update")
	assert.True(t, ok)

	other := NewMap(map[string]Object{"b": NewInt(2), "a": NewInt(10)})
	result := update.(*Builtin).Call(ctx, other)
	assert.Equal(t, result, m)

	// Updated
	assert.Equal(t, m.Get("a").(*Int).Value(), int64(10))
	assert.Equal(t, m.Get("b").(*Int).Value(), int64(2))
}

func TestMapGetAttrUpdateErrors(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	update, _ := m.GetAttr("update")

	// Wrong arg count
	result := update.(*Builtin).Call(ctx)
	assert.True(t, IsError(result))

	// Wrong type
	result = update.(*Builtin).Call(ctx, NewInt(1))
	assert.True(t, IsError(result))
}

func TestMapGetAttrFallback(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})

	// Falls back to getting value from map
	val, ok := m.GetAttr("key")
	assert.True(t, ok)
	assert.Equal(t, val.(*Int).Value(), int64(42))

	// Missing key
	_, ok = m.GetAttr("missing")
	assert.False(t, ok)
}

func TestMapGet(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})

	// Existing key
	val := m.Get("key")
	assert.Equal(t, val.(*Int).Value(), int64(42))

	// Missing key
	val = m.Get("missing")
	assert.Equal(t, val, Nil)
}

func TestMapGetWithObject(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})

	// Existing key
	val := m.GetWithObject(NewString("key"))
	assert.Equal(t, val.(*Int).Value(), int64(42))

	// Missing key
	val = m.GetWithObject(NewString("missing"))
	assert.Equal(t, val, Nil)
}

func TestMapGetWithDefault(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})

	// Existing key
	val := m.GetWithDefault("key", NewInt(0))
	assert.Equal(t, val.(*Int).Value(), int64(42))

	// Missing key - returns default
	val = m.GetWithDefault("missing", NewInt(0))
	assert.Equal(t, val.(*Int).Value(), int64(0))
}

func TestMapSet(t *testing.T) {
	m := NewMap(nil)
	m.Set("key", NewInt(42))
	assert.Equal(t, m.Get("key").(*Int).Value(), int64(42))
}

func TestMapDelete(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})
	m.Delete("key")
	assert.Equal(t, m.Get("key"), Nil)
}

func TestMapSize(t *testing.T) {
	m := NewMap(map[string]Object{"a": NewInt(1), "b": NewInt(2)})
	assert.Equal(t, m.Size(), 2)
}

func TestMapInterface(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})
	iface := m.Interface()
	result := iface.(map[string]any)
	assert.Equal(t, result["key"], int64(42))
}

func TestMapEquals(t *testing.T) {
	m1 := NewMap(map[string]Object{"a": NewInt(1), "b": NewInt(2)})
	m2 := NewMap(map[string]Object{"a": NewInt(1), "b": NewInt(2)})
	m3 := NewMap(map[string]Object{"a": NewInt(1)})
	m4 := NewMap(map[string]Object{"a": NewInt(1), "b": NewInt(3)})

	// Equal maps
	assert.Equal(t, m1.Equals(m2), True)

	// Different sizes
	assert.Equal(t, m1.Equals(m3), False)

	// Different values
	assert.Equal(t, m1.Equals(m4), False)

	// Different type
	assert.Equal(t, m1.Equals(NewString("test")), False)
}

func TestMapRunOperation(t *testing.T) {
	m := NewMap(nil)
	result := m.RunOperation(op.Add, NewInt(1))
	assert.True(t, IsError(result))
}

func TestMapGetItem(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})

	// Existing key
	val, err := m.GetItem(NewString("key"))
	assert.Nil(t, err)
	assert.Equal(t, val.(*Int).Value(), int64(42))

	// Missing key
	_, err = m.GetItem(NewString("missing"))
	assert.NotNil(t, err)

	// Wrong key type
	_, err = m.GetItem(NewInt(1))
	assert.NotNil(t, err)
}

func TestMapGetSlice(t *testing.T) {
	m := NewMap(nil)
	_, err := m.GetSlice(Slice{})
	assert.NotNil(t, err)
}

func TestMapSetItem(t *testing.T) {
	m := NewMap(nil)

	// Valid set
	err := m.SetItem(NewString("key"), NewInt(42))
	assert.Nil(t, err)
	assert.Equal(t, m.Get("key").(*Int).Value(), int64(42))

	// Wrong key type
	err = m.SetItem(NewInt(1), NewInt(42))
	assert.NotNil(t, err)
}

func TestMapDelItem(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})

	// Valid delete
	err := m.DelItem(NewString("key"))
	assert.Nil(t, err)
	assert.Equal(t, m.Get("key"), Nil)

	// Wrong key type
	err = m.DelItem(NewInt(1))
	assert.NotNil(t, err)
}

func TestMapContains(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})

	// Contains
	assert.Equal(t, m.Contains(NewString("key")), True)

	// Does not contain
	assert.Equal(t, m.Contains(NewString("missing")), False)

	// Wrong type
	assert.Equal(t, m.Contains(NewInt(1)), False)
}

func TestMapIsTruthy(t *testing.T) {
	// Empty map is falsy
	assert.False(t, NewMap(nil).IsTruthy())

	// Non-empty map is truthy
	assert.True(t, NewMap(map[string]Object{"key": NewInt(1)}).IsTruthy())
}

func TestMapLen(t *testing.T) {
	m := NewMap(map[string]Object{"a": NewInt(1), "b": NewInt(2)})
	assert.Equal(t, m.Len().Value(), int64(2))
}

func TestMapEnumerate(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	var keys []string
	var values []int64
	m.Enumerate(ctx, func(key, value Object) bool {
		keys = append(keys, key.(*String).Value())
		values = append(values, value.(*Int).Value())
		return true
	})

	// Enumerated in sorted key order
	assert.Equal(t, keys, []string{"a", "b"})
	assert.Equal(t, values, []int64{1, 2})
}

func TestMapEnumerateEarlyStop(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"a": NewInt(1),
		"b": NewInt(2),
		"c": NewInt(3),
	})

	count := 0
	m.Enumerate(ctx, func(key, value Object) bool {
		count++
		return count < 2 // Stop after 2nd iteration
	})

	assert.Equal(t, count, 2)
}

func TestMapStringKeys(t *testing.T) {
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	keys := m.StringKeys()
	assert.Len(t, keys, 2)
	// StringKeys returns unsorted keys
	assert.True(t, (keys[0] == "a" && keys[1] == "b") || (keys[0] == "b" && keys[1] == "a"))
}

func TestMapSortedKeys(t *testing.T) {
	m := NewMap(map[string]Object{
		"c": NewInt(3),
		"a": NewInt(1),
		"b": NewInt(2),
	})

	keys := m.SortedKeys()
	assert.Equal(t, keys, []string{"a", "b", "c"})
}

func TestMapCost(t *testing.T) {
	m := NewMap(map[string]Object{
		"a": NewInt(1),
		"b": NewInt(2),
	})
	assert.Equal(t, m.Cost(), 16) // 2 items * 8
}

func TestMapMarshalJSON(t *testing.T) {
	m := NewMap(map[string]Object{
		"num": NewInt(42),
		"str": NewString("hello"),
	})
	data, err := m.MarshalJSON()
	assert.Nil(t, err)
	// JSON should be valid
	assert.True(t, len(data) > 0)
}

func TestNewMapNil(t *testing.T) {
	m := NewMap(nil)
	assert.NotNil(t, m.Value())
	assert.Len(t, m.Value(), 0)
}
