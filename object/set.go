package object

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/risor-io/risor/op"
)

type Set struct {
	*base
	items map[HashKey]Object
}

func (s *Set) Type() Type {
	return SET
}

func (s *Set) Value() map[HashKey]Object {
	return s.items
}

func (s *Set) Inspect() string {
	var out bytes.Buffer
	items := make([]string, 0, len(s.items))
	for _, item := range s.SortedItems() {
		items = append(items, item.Inspect())
	}
	out.WriteString("{")
	out.WriteString(strings.Join(items, ", "))
	out.WriteString("}")
	return out.String()
}

func (s *Set) String() string {
	return s.Inspect()
}

func (s *Set) GetAttr(name string) (Object, bool) {
	switch name {
	case "add":
		return &Builtin{
			name: "set.add",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("set.add", 1, len(args))
				}
				return s.Add(args[0])
			},
		}, true
	case "clear":
		return &Builtin{
			name: "set.clear",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("set.clear", 0, len(args))
				}
				s.Clear()
				return s
			},
		}, true
	case "remove":
		return &Builtin{
			name: "set.remove",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("set.remove", 1, len(args))
				}
				return s.Remove(args[0])
			},
		}, true
	case "union":
		return &Builtin{
			name: "set.union",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("set.union", 1, len(args))
				}
				other, err := AsSet(args[0])
				if err != nil {
					return err
				}
				return s.Union(other)
			},
		}, true
	case "intersection":
		return &Builtin{
			name: "set.intersection",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("set.intersection", 1, len(args))
				}
				other, err := AsSet(args[0])
				if err != nil {
					return err
				}
				return s.Intersection(other)
			},
		}, true
	}
	return nil, false
}

func (s *Set) Interface() interface{} {
	items := make([]interface{}, 0, len(s.items))
	for _, item := range s.SortedItems() {
		items = append(items, item.Interface())
	}
	return items
}

func (s *Set) Size() int {
	return len(s.items)
}

func (s *Set) SortedItems() []Object {
	items := make([]Object, 0, len(s.items))
	for _, v := range s.items {
		items = append(items, v)
	}
	sort.Slice(items, func(i, j int) bool {
		h1 := items[i].(Hashable).HashKey()
		h2 := items[j].(Hashable).HashKey()
		if h1.Type != h2.Type {
			return h1.Type < h2.Type
		}
		if h1.IntValue != h2.IntValue {
			return h1.IntValue < h2.IntValue
		}
		if h1.StrValue != h2.StrValue {
			return h1.StrValue < h2.StrValue
		}
		if h1.FltValue != h2.FltValue {
			return h1.FltValue < h2.FltValue
		}
		return false
	})
	return items
}

func (s *Set) Add(items ...Object) Object {
	for _, item := range items {
		hashable, ok := item.(Hashable)
		if !ok {
			return TypeErrorf("type error: %s object is unhashable", item.Type())
		}
		s.items[hashable.HashKey()] = item
	}
	return s
}

func (s *Set) Remove(items ...Object) Object {
	for _, item := range items {
		hashable, ok := item.(Hashable)
		if !ok {
			return TypeErrorf("type error: %s object is unhashable", item.Type())
		}
		delete(s.items, hashable.HashKey())
	}
	return s
}

func (s *Set) Clear() {
	s.items = map[HashKey]Object{}
}

// Union returns a new set that is the union of the two sets.
func (s *Set) Union(other *Set) *Set {
	union := &Set{items: map[HashKey]Object{}}
	for k, v := range s.items {
		union.items[k] = v
	}
	for k, v := range other.items {
		union.items[k] = v
	}
	return union
}

// Intersection returns a new set that is the intersection of the two sets.
func (s *Set) Intersection(other *Set) *Set {
	intersection := &Set{items: map[HashKey]Object{}}
	for k, v := range s.items {
		if _, ok := other.items[k]; ok {
			intersection.items[k] = v
		}
	}
	return intersection
}

// Difference returns a new set that is the difference of the two sets.
func (s *Set) Difference(other *Set) *Set {
	difference := &Set{items: map[HashKey]Object{}}
	for k, v := range s.items {
		if _, ok := other.items[k]; !ok {
			difference.items[k] = v
		}
	}
	return difference
}

func (s *Set) List() *List {
	return &List{items: s.SortedItems()}
}

func (s *Set) Equals(other Object) Object {
	if other.Type() != SET {
		return False
	}
	otherSet := other.(*Set)
	if len(s.items) != len(otherSet.items) {
		return False
	}
	for k, v := range s.items {
		if otherV, ok := otherSet.items[k]; !ok || !v.Equals(otherV).(*Bool).value {
			return False
		}
	}
	return True
}

func (s *Set) GetItem(key Object) (Object, *Error) {
	hashable, ok := key.(Hashable)
	if !ok {
		return nil, TypeErrorf("type error: %s object is unhashable", key.Type())
	}
	if _, ok := s.items[hashable.HashKey()]; ok {
		return True, nil
	}
	return False, nil
}

// GetSlice implements the [start:stop] operator for a container type.
func (s *Set) GetSlice(slice Slice) (Object, *Error) {
	return nil, TypeErrorf("type error: set does not support get slice")
}

// SetItem assigns a value to the given key in the map.
func (s *Set) SetItem(key, value Object) *Error {
	return TypeErrorf("type error: set does not support set item")
}

// DelItem deletes the item with the given key from the map.
func (s *Set) DelItem(key Object) *Error {
	hashable, ok := key.(Hashable)
	if !ok {
		return TypeErrorf("type error: %s object is unhashable", key.Type())
	}
	delete(s.items, hashable.HashKey())
	return nil
}

// Contains returns true if the given item is found in this container.
func (s *Set) Contains(key Object) *Bool {
	hashable, ok := key.(Hashable)
	if !ok {
		return False
	}
	_, ok = s.items[hashable.HashKey()]
	return NewBool(ok)
}

func (s *Set) IsTruthy() bool {
	return len(s.items) > 0
}

func (s *Set) RunOperation(opType op.BinaryOpType, right Object) Object {
	return TypeErrorf("type error: unsupported operation for set: %v", opType)
}

// Len returns the number of items in this container.
func (s *Set) Len() *Int {
	return NewInt(int64(len(s.items)))
}

func (s *Set) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	for _, v := range s.items {
		if !fn(v, v) {
			return
		}
	}
}

func (s *Set) Keys() []HashKey {
	items := s.SortedItems()
	keys := make([]HashKey, 0, len(items))
	for _, item := range items {
		hashable, ok := item.(Hashable)
		if !ok {
			panic(TypeErrorf("type error: %s object is unhashable", item.Type()))
		}
		keys = append(keys, hashable.HashKey())
	}
	return keys
}

func (s *Set) Cost() int {
	return len(s.items) * 8
}

func (s *Set) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.SortedItems())
}

func NewSet(items []Object) Object {
	s := &Set{items: map[HashKey]Object{}}
	for _, item := range items {
		if result := s.Add(item); IsError(result) {
			return result
		}
	}
	return s
}

func NewSetWithSize(size int) *Set {
	return &Set{items: make(map[HashKey]Object, size)}
}
