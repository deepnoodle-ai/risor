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
	m := NewMap(map[string]Object{"key": NewInt(1)})

	// Update existing key works
	err := m.SetAttr("key", NewInt(100))
	assert.Nil(t, err)
	val := m.Get("key")
	assert.Equal(t, val.(*Int).Value(), int64(100))
}

func TestMapSetAttrNewKeyError(t *testing.T) {
	m := NewMap(map[string]Object{"a": NewInt(1)})

	// Adding new key via SetAttr fails
	err := m.SetAttr("b", NewInt(2))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	// Key was not added
	_, exists := m.items["b"]
	assert.False(t, exists)
}

func TestMapGetAttrReturnsMapValues(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})

	// GetAttr returns map values
	val, ok := m.GetAttr("key")
	assert.True(t, ok)
	assert.Equal(t, val.(*Int).Value(), int64(42))

	// Missing key returns false
	_, ok = m.GetAttr("missing")
	assert.False(t, ok)
}

func TestMapKeys(t *testing.T) {
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	keys := m.Keys()
	assert.Len(t, keys.Value(), 2)
	// Keys are sorted
	assert.Equal(t, keys.Value()[0].(*String).Value(), "a")
	assert.Equal(t, keys.Value()[1].(*String).Value(), "b")
}

func TestMapValues(t *testing.T) {
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	values := m.Values()
	assert.Len(t, values.Value(), 2)
	// Values are in key-sorted order
	assert.Equal(t, values.Value()[0].(*Int).Value(), int64(1))
	assert.Equal(t, values.Value()[1].(*Int).Value(), int64(2))
}

func TestMapListItems(t *testing.T) {
	m := NewMap(map[string]Object{
		"b": NewInt(2),
		"a": NewInt(1),
	})

	items := m.ListItems()
	assert.Len(t, items.Value(), 2)

	// Items are key-sorted pairs
	pair0 := items.Value()[0].(*List).Value()
	assert.Equal(t, pair0[0].(*String).Value(), "a")
	assert.Equal(t, pair0[1].(*Int).Value(), int64(1))

	pair1 := items.Value()[1].(*List).Value()
	assert.Equal(t, pair1[0].(*String).Value(), "b")
	assert.Equal(t, pair1[1].(*Int).Value(), int64(2))
}

func TestMapClear(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})
	m.Clear()
	assert.Equal(t, m.Size(), 0)
}

func TestMapCopy(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})
	copyMap := m.Copy()

	// Copy has same values
	assert.Equal(t, copyMap.Get("key").(*Int).Value(), int64(42))

	// But is a different object
	m.Set("key", NewInt(100))
	assert.Equal(t, copyMap.Get("key").(*Int).Value(), int64(42))
}

func TestMapPop(t *testing.T) {
	m := NewMap(map[string]Object{"key": NewInt(42)})

	// Pop existing key
	result := m.Pop("key", nil)
	assert.Equal(t, result.(*Int).Value(), int64(42))
	assert.Equal(t, m.Size(), 0)

	// Pop missing key without default - returns nil
	result = m.Pop("missing", nil)
	assert.Equal(t, result, Nil)

	// Pop missing key with default
	result = m.Pop("missing", NewString("default"))
	assert.Equal(t, result.(*String).Value(), "default")
}

func TestMapSetDefault(t *testing.T) {
	m := NewMap(map[string]Object{"existing": NewInt(1)})

	// Set default for missing key
	result := m.SetDefault("new", NewInt(99))
	assert.Equal(t, result.(*Int).Value(), int64(99))
	assert.Equal(t, m.Get("new").(*Int).Value(), int64(99))

	// Set default for existing key - returns existing value
	result = m.SetDefault("existing", NewInt(999))
	assert.Equal(t, result.(*Int).Value(), int64(1))
}

func TestMapUpdate(t *testing.T) {
	m := NewMap(map[string]Object{"a": NewInt(1)})
	other := NewMap(map[string]Object{"b": NewInt(2), "a": NewInt(10)})

	m.Update(other)

	// Updated
	assert.Equal(t, m.Get("a").(*Int).Value(), int64(10))
	assert.Equal(t, m.Get("b").(*Int).Value(), int64(2))
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
