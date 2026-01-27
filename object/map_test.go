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
	result, err := keys.(*Builtin).Call(ctx)
	assert.Nil(t, err)
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
	_, err := keys.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestMapGetAttrValues(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	values, ok := m.GetAttr("values")
	assert.True(t, ok)
	result, err := values.(*Builtin).Call(ctx)
	assert.Nil(t, err)
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
	_, err := values.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestMapGetAttrGet(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"key": NewInt(42)})

	get, ok := m.GetAttr("get")
	assert.True(t, ok)

	// Get existing key
	result, err := get.(*Builtin).Call(ctx, NewString("key"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(42))

	// Get missing key - returns nil
	result, err = get.(*Builtin).Call(ctx, NewString("missing"))
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)

	// Get missing key with default
	result, err = get.(*Builtin).Call(ctx, NewString("missing"), NewString("default"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "default")
}

func TestMapGetAttrGetErrors(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	get, _ := m.GetAttr("get")

	// No args
	_, err := get.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Too many args
	_, err = get.(*Builtin).Call(ctx, NewString("a"), NewString("b"), NewString("c"))
	assert.NotNil(t, err)

	// Wrong type for key
	_, err = get.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestMapGetAttrClear(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"key": NewInt(42)})

	clear, ok := m.GetAttr("clear")
	assert.True(t, ok)

	result, err := clear.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.Equal(t, result, m)
	assert.Equal(t, m.Size(), 0)
}

func TestMapGetAttrClearError(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	clear, _ := m.GetAttr("clear")
	_, err := clear.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestMapGetAttrCopy(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"key": NewInt(42)})

	copyFn, ok := m.GetAttr("copy")
	assert.True(t, ok)

	result, err := copyFn.(*Builtin).Call(ctx)
	assert.Nil(t, err)
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
	_, err := copyFn.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestMapGetAttrItems(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	items, ok := m.GetAttr("items")
	assert.True(t, ok)

	result, err := items.(*Builtin).Call(ctx)
	assert.Nil(t, err)
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
	_, err := items.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestMapGetAttrPop(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"key": NewInt(42)})

	pop, ok := m.GetAttr("pop")
	assert.True(t, ok)

	// Pop existing key
	result, err := pop.(*Builtin).Call(ctx, NewString("key"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(42))
	assert.Equal(t, m.Size(), 0)

	// Pop missing key without default - returns nil
	result, err = pop.(*Builtin).Call(ctx, NewString("missing"))
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)

	// Pop missing key with default
	result, err = pop.(*Builtin).Call(ctx, NewString("missing"), NewString("default"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "default")
}

func TestMapGetAttrPopErrors(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	pop, _ := m.GetAttr("pop")

	// No args
	_, err := pop.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Too many args
	_, err = pop.(*Builtin).Call(ctx, NewString("a"), NewString("b"), NewString("c"))
	assert.NotNil(t, err)

	// Wrong type for key
	_, err = pop.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestMapGetAttrSetDefault(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"existing": NewInt(1)})

	setdefault, ok := m.GetAttr("setdefault")
	assert.True(t, ok)

	// Set default for missing key
	result, err := setdefault.(*Builtin).Call(ctx, NewString("new"), NewInt(99))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(99))
	assert.Equal(t, m.Get("new").(*Int).Value(), int64(99))

	// Set default for existing key - returns existing value
	result, err = setdefault.(*Builtin).Call(ctx, NewString("existing"), NewInt(999))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(1))
}

func TestMapGetAttrSetDefaultErrors(t *testing.T) {
	ctx := context.Background()
	m := NewMap(nil)
	setdefault, _ := m.GetAttr("setdefault")

	// Wrong arg count
	_, err := setdefault.(*Builtin).Call(ctx, NewString("key"))
	assert.NotNil(t, err)

	// Wrong type for key
	_, err = setdefault.(*Builtin).Call(ctx, NewInt(1), NewInt(2))
	assert.NotNil(t, err)
}

func TestMapGetAttrUpdate(t *testing.T) {
	ctx := context.Background()
	m := NewMap(map[string]Object{"a": NewInt(1)})

	update, ok := m.GetAttr("update")
	assert.True(t, ok)

	other := NewMap(map[string]Object{"b": NewInt(2), "a": NewInt(10)})
	result, err := update.(*Builtin).Call(ctx, other)
	assert.Nil(t, err)
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
	_, err := update.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = update.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
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
	assert.True(t, m1.Equals(m2))

	// Different sizes
	assert.False(t, m1.Equals(m3))

	// Different values
	assert.False(t, m1.Equals(m4))

	// Different type
	assert.False(t, m1.Equals(NewString("test")))
}

func TestMapRunOperation(t *testing.T) {
	m := NewMap(nil)
	_, err := m.RunOperation(op.Add, NewInt(1))
	assert.NotNil(t, err)
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
