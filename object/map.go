package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/risor-io/risor/op"
)

type Map struct {
	items map[string]Object

	// Used to avoid the possibility of infinite recursion when inspecting.
	// Similar to the usage of Py_ReprEnter in CPython.
	inspectActive bool
}

func (m *Map) Type() Type {
	return MAP
}

func (m *Map) Inspect() string {
	// A map can contain itself. Detect if we're already inspecting the map
	// and return a placeholder if so.
	if m.inspectActive {
		return "{...}"
	}
	m.inspectActive = true
	defer func() { m.inspectActive = false }()

	var out bytes.Buffer
	pairs := make([]string, 0)
	for _, k := range m.SortedKeys() {
		v := m.items[k]
		pairs = append(pairs, fmt.Sprintf("%q: %s", k, v.Inspect()))
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
	m.Set(name, value)
	return nil
}

func (m *Map) GetAttr(name string) (Object, bool) {
	switch name {
	case "keys":
		return &Builtin{
			name: "map.keys",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("map.keys: expected 0 arguments, got %d", len(args))
				}
				return m.Keys(), nil
			},
		}, true
	case "values":
		return &Builtin{
			name: "map.values",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("map.values: expected 0 arguments, got %d", len(args))
				}
				return m.Values(), nil
			},
		}, true
	case "get":
		return &Builtin{
			name: "map.get",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) < 1 || len(args) > 2 {
					return nil, fmt.Errorf("map.get: expected 1-2 arguments, got %d", len(args))
				}
				key, err := AsString(args[0])
				if err != nil {
					return nil, err
				}
				value, found := m.items[key]
				if !found {
					if len(args) == 2 {
						return args[1], nil
					}
					return Nil, nil
				}
				return value, nil
			},
		}, true
	case "clear":
		return &Builtin{
			name: "map.clear",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("map.clear: expected 0 arguments, got %d", len(args))
				}
				m.Clear()
				return m, nil
			},
		}, true
	case "copy":
		return &Builtin{
			name: "map.copy",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("map.copy: expected 0 arguments, got %d", len(args))
				}
				return m.Copy(), nil
			},
		}, true
	case "items":
		return &Builtin{
			name: "map.items",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("map.items: expected 0 arguments, got %d", len(args))
				}
				return m.ListItems(), nil
			},
		}, true
	case "pop":
		return &Builtin{
			name: "map.pop",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				nArgs := len(args)
				if nArgs < 1 || nArgs > 2 {
					return nil, fmt.Errorf("map.pop: expected 1-2 arguments, got %d", len(args))
				}
				key, err := AsString(args[0])
				if err != nil {
					return nil, err
				}
				var def Object
				if nArgs == 2 {
					def = args[1]
				}
				return m.Pop(key, def), nil
			},
		}, true
	case "setdefault":
		return &Builtin{
			name: "map.setdefault",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 2 {
					return nil, fmt.Errorf("map.setdefault: expected 2 arguments, got %d", len(args))
				}
				key, err := AsString(args[0])
				if err != nil {
					return nil, err
				}
				return m.SetDefault(key, args[1]), nil
			},
		}, true
	case "update":
		return &Builtin{
			name: "map.update",
			fn: func(ctx context.Context, args ...Object) (Object, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("map.update: expected 1 argument, got %d", len(args))
				}
				other, err := AsMap(args[0])
				if err != nil {
					return nil, err
				}
				m.Update(other)
				return m, nil
			},
		}, true
	}
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

func (m *Map) Interface() interface{} {
	result := make(map[string]any, len(m.items))
	for k, v := range m.items {
		result[k] = v.Interface()
	}
	return result
}

func (m *Map) Equals(other Object) bool {
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
		if !v.Equals(otherValue) {
			return false
		}
	}
	return true
}

func (m *Map) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, fmt.Errorf("type error: unsupported operation for map: %v", opType)
}

func (m *Map) GetItem(key Object) (Object, *Error) {
	strObj, ok := key.(*String)
	if !ok {
		return nil, TypeErrorf("type error: map key must be a string (got %s)", key.Type())
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
		return TypeErrorf("type error: map key must be a string (got %s)", key.Type())
	}
	m.items[strObj.value] = value
	return nil
}

// DelItem deletes the item with the given key from the map.
func (m *Map) DelItem(key Object) *Error {
	strObj, ok := key.(*String)
	if !ok {
		return TypeErrorf("type error: map key must be a string (got %s)", key.Type())
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

func (m *Map) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	for _, k := range m.SortedKeys() {
		if !fn(NewString(k), m.items[k]) {
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


func (m *Map) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.items)
}

func NewMap(m map[string]Object) *Map {
	if m == nil {
		m = map[string]Object{}
	}
	return &Map{items: m}
}
