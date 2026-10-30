package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

var mapMethods = NewMethodRegistry[*Map]("map")

func init() {
	// Iterator-returning methods
	mapMethods.Define("keys").
		Doc("Iterate over map keys").
		Returns("iter").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			return NewMapKeyIter(m), nil
		})

	mapMethods.Define("values").
		Doc("Iterate over map values").
		Returns("iter").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			return NewMapValueIter(m), nil
		})

	mapMethods.Define("entries").
		Doc("Iterate over [key, value] pairs").
		Returns("iter").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			return NewMapItemIter(m), nil
		})

	// Callback-based iteration
	mapMethods.Define("each").
		Doc("Call function for each key-value pair").
		Arg("fn").
		Returns("null").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			callable, ok := args[0].(Callable)
			if !ok {
				return nil, newTypeErrorf("map.each() expected a function (%s given)", args[0].Type())
			}
			for _, k := range m.SortedKeys() {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				// Skip keys deleted by an earlier callback
				value, ok := m.items[k]
				if !ok {
					continue
				}
				if _, err := callable.Call(ctx, NewString(k), value); err != nil {
					return nil, err
				}
			}
			return Nil, nil
		})

	// Safe access with default
	mapMethods.Define("get").
		Doc("Get value with optional default").
		Arg("key").
		OptionalArg("default").
		Returns("any").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			key, err := Arg[*String](args, 0, "map.get")
			if err != nil {
				return nil, err
			}
			value, found := m.items[key.value]
			if found {
				return value, nil
			}
			if len(args) > 1 {
				return args[1], nil
			}
			return Nil, nil
		})

	// Remove and return
	mapMethods.Define("pop").
		Doc("Remove key and return its value").
		Arg("key").
		OptionalArg("default").
		Returns("any").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			key, err := Arg[*String](args, 0, "map.pop")
			if err != nil {
				return nil, err
			}
			value, found := m.items[key.value]
			if found {
				delete(m.items, key.value)
				return value, nil
			}
			if len(args) > 1 {
				return args[1], nil
			}
			return Nil, nil
		})

	// Set if missing
	mapMethods.Define("setdefault").
		Doc("Set value if key is missing, return final value").
		Args("key", "value").
		Returns("any").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			key, err := Arg[*String](args, 0, "map.setdefault")
			if err != nil {
				return nil, err
			}
			if _, found := m.items[key.value]; !found {
				m.items[key.value] = args[1]
			}
			return m.items[key.value], nil
		})

	// Merge another map
	mapMethods.Define("update").
		Doc("Merge another map into this one").
		Arg("other").
		Returns("null").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			other, ok := args[0].(*Map)
			if !ok {
				return nil, newTypeErrorf("map.update() expected a map (%s given)", args[0].Type())
			}
			// Short-circuit if updating with self
			if other == m {
				return Nil, nil
			}
			for k, v := range other.items {
				m.items[k] = v
			}
			return Nil, nil
		})

	// Clear all items
	mapMethods.Define("clear").
		Doc("Remove all items").
		Returns("null").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			m.items = map[string]Object{}
			return Nil, nil
		})

	// Shallow copy
	mapMethods.Define("copy").
		Doc("Create a shallow copy").
		Returns("map").
		Impl(func(m *Map, ctx context.Context, args ...Object) (Object, error) {
			return m.Copy(), nil
		})
}

// Map is a string-keyed map of objects.
//
// Iteration (each, keys, values, entries, Enumerate) visits the keys present
// when iteration starts, in sorted order. Keys deleted during iteration are
// skipped, keys added during iteration are not visited, and each step reads
// the key's current value.
type Map struct {
	items map[string]Object
}

func (m *Map) Type() Type {
	return MAP
}

// Inspect returns a string representation of the map. A nested container
// that is already being printed (a cycle), or that is nested more deeply than
// MaxNestingDepth, is shown as [...] or {...}.
func (m *Map) Inspect() string {
	return m.inspect(nil)
}

func (m *Map) inspect(path nestPath[Object]) string {
	var out bytes.Buffer
	pairs := make([]string, 0, len(m.items))
	for _, k := range m.SortedKeys() {
		v := m.items[k]
		if isNestedContainer(v) {
			var err error
			if path, err = path.descend(m, v); err != nil {
				pairs = append(pairs, fmt.Sprintf("%q: %s", k, inspectPlaceholder(v)))
				continue
			}
		}
		pairs = append(pairs, fmt.Sprintf("%q: %s", k, inspectNested(v, path)))
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")
	return out.String()
}

func (m *Map) String() string {
	return m.Inspect()
}

func (m *Map) Value() map[string]Object {
	return m.items
}

func (m *Map) SetAttr(name string, value Object) error {
	// Dot syntax only updates existing keys. Use bracket syntax to add new keys.
	if _, exists := m.items[name]; !exists {
		return fmt.Errorf("key error: %q does not exist (use m[%q] = value to add new keys)", name, name)
	}
	m.Set(name, value)
	return nil
}

func (m *Map) Attrs() []AttrSpec {
	return mapMethods.Specs()
}

func (m *Map) GetAttr(name string) (Object, bool) {
	// Methods take priority over map keys (Python-style shadowing).
	// Use bracket syntax m["keys"] to access a key that shadows a method.
	if method, ok := mapMethods.GetAttr(m, name); ok {
		return method, true
	}
	// Fall back to map data
	o, ok := m.items[name]
	return o, ok
}

func (m *Map) ListItems() *List {
	items := make([]Object, 0, len(m.items))
	for _, k := range m.SortedKeys() {
		items = append(items, NewList([]Object{NewString(k), m.items[k]}))
	}
	return NewList(items)
}

func (m *Map) Clear() {
	m.items = map[string]Object{}
}

func (m *Map) Copy() *Map {
	items := make(map[string]Object, len(m.items))
	for k, v := range m.items {
		items[k] = v
	}
	return &Map{items: items}
}

func (m *Map) Pop(key string, def Object) Object {
	value, found := m.items[key]
	if found {
		delete(m.items, key)
		return value
	}
	if def != nil {
		return def
	}
	return Nil
}

func (m *Map) SetDefault(key string, value Object) Object {
	if _, found := m.items[key]; !found {
		m.items[key] = value
	}
	return m.items[key]
}

func (m *Map) Update(other *Map) {
	for k, v := range other.items {
		m.items[k] = v
	}
}

func (m *Map) SortedKeys() []string {
	keys := make([]string, 0, len(m.items))
	for k := range m.items {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (m *Map) Keys() *List {
	items := make([]Object, 0, len(m.items))
	for _, k := range m.SortedKeys() {
		items = append(items, NewString(k))
	}
	return &List{items: items}
}

func (m *Map) Values() *List {
	items := make([]Object, 0, len(m.items))
	for _, k := range m.SortedKeys() {
		items = append(items, m.items[k])
	}
	return &List{items: items}
}

func (m *Map) GetWithObject(key *String) Object {
	value, found := m.items[key.value]
	if !found {
		return Nil
	}
	return value
}

func (m *Map) Get(key string) Object {
	value, found := m.items[key]
	if !found {
		return Nil
	}
	return value
}

func (m *Map) GetWithDefault(key string, defaultValue Object) Object {
	value, found := m.items[key]
	if !found {
		return defaultValue
	}
	return value
}

func (m *Map) Delete(key string) Object {
	delete(m.items, key)
	return Nil
}

func (m *Map) Set(key string, value Object) {
	m.items[key] = value
}

func (m *Map) Size() int {
	return len(m.items)
}

// Interface converts the map to a map[string]any, converting values
// recursively. It returns nil if the map is nested more deeply than
// MaxNestingDepth or contains a cycle; use ToInterface to get the error.
func (m *Map) Interface() interface{} {
	result, err := m.toInterface(nil)
	if err != nil {
		return nil
	}
	return result
}

func (m *Map) toInterface(path nestPath[Object]) (any, error) {
	result := make(map[string]any, len(m.items))
	for k, v := range m.items {
		if isNestedContainer(v) {
			var err error
			if path, err = path.descend(m, v); err != nil {
				return nil, err
			}
		}
		value, err := toInterface(v, path)
		if err != nil {
			return nil, err
		}
		result[k] = value
	}
	return result, nil
}

// Equals reports whether other is a map with the same keys and equal values.
// It returns false if the maps are too deeply nested or cyclic.
func (m *Map) Equals(other Object) bool {
	return m.equals(other, nil)
}

func (m *Map) equals(other Object, path nestPath[objectPair]) bool {
	otherMap, ok := other.(*Map)
	if !ok {
		return false
	}
	if len(m.items) != len(otherMap.items) {
		return false
	}
	for k, v := range m.items {
		otherValue, found := otherMap.items[k]
		if !found {
			return false
		}
		if isNestedContainer(v) {
			var err error
			if path, err = path.descend(objectPair{m, otherMap}, objectPair{v, otherValue}); err != nil {
				return false
			}
		}
		if !equalsNested(v, otherValue, path) {
			return false
		}
	}
	return true
}

func (m *Map) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for map: %v", opType)
}

func (m *Map) GetItem(key Object) (Object, *Error) {
	strObj, ok := key.(*String)
	if !ok {
		return nil, TypeErrorf("map key must be a string (got %s)", key.Type())
	}
	value, found := m.items[strObj.value]
	if !found {
		return nil, Errorf("key error: %q", strObj.Value())
	}
	return value, nil
}

// GetSlice implements the [start:stop] operator for a container type.
func (m *Map) GetSlice(s Slice) (Object, *Error) {
	return nil, TypeErrorf("map does not support slice operations")
}

// SetItem assigns a value to the given key in the map.
func (m *Map) SetItem(key, value Object) *Error {
	strObj, ok := key.(*String)
	if !ok {
		return TypeErrorf("map key must be a string (got %s)", key.Type())
	}
	m.items[strObj.value] = value
	return nil
}

// DelItem deletes the item with the given key from the map.
func (m *Map) DelItem(key Object) *Error {
	strObj, ok := key.(*String)
	if !ok {
		return TypeErrorf("map key must be a string (got %s)", key.Type())
	}
	delete(m.items, strObj.value)
	return nil
}

// Contains returns true if the given item is found in this container.
func (m *Map) Contains(key Object) *Bool {
	strObj, ok := key.(*String)
	if !ok {
		return False
	}
	_, found := m.items[strObj.value]
	return NewBool(found)
}

func (m *Map) IsTruthy() bool {
	return len(m.items) > 0
}

// Len returns the number of items in this container.
func (m *Map) Len() *Int {
	return NewInt(int64(len(m.items)))
}

// Enumerate calls fn for each key and value, following the iteration rules
// described on Map. It stops early if ctx is canceled; callers should check
// ctx.Err() afterwards.
func (m *Map) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	for _, k := range m.SortedKeys() {
		if ctx.Err() != nil {
			return
		}
		value, ok := m.items[k]
		if !ok {
			continue
		}
		if !fn(NewString(k), value) {
			return
		}
	}
}

func (m *Map) StringKeys() []string {
	keys := make([]string, 0, len(m.items))
	for k := range m.items {
		keys = append(keys, k)
	}
	return keys
}

// MarshalJSON encodes the map as a JSON object with sorted keys. It returns
// ErrNestingDepth if the map is too deeply nested or contains a cycle.
func (m *Map) MarshalJSON() ([]byte, error) {
	return m.marshalJSON(nil)
}

func (m *Map) marshalJSON(path nestPath[Object]) ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	for i, k := range m.SortedKeys() {
		if i > 0 {
			out.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		out.Write(key)
		out.WriteByte(':')
		v := m.items[k]
		if isNestedContainer(v) {
			if path, err = path.descend(m, v); err != nil {
				return nil, err
			}
		}
		data, err := marshalJSONNested(v, path)
		if err != nil {
			return nil, err
		}
		out.Write(data)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

func NewMap(m map[string]Object) *Map {
	if m == nil {
		m = map[string]Object{}
	}
	return &Map{items: m}
}
