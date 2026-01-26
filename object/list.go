package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/op"
)

// List of objects
type List struct {
	*base

	// items holds the list of objects
	items []Object

	// Used to avoid the possibility of infinite recursion when inspecting.
	// Similar to the usage of Py_ReprEnter in CPython.
	inspectActive bool
}

func (ls *List) Type() Type {
	return LIST
}

func (ls *List) Value() []Object {
	return ls.items
}

func (ls *List) Inspect() string {
	// A list can contain itself. Detect if we're already inspecting the list
	// and return a placeholder if so.
	if ls.inspectActive {
		return "[...]"
	}
	ls.inspectActive = true
	defer func() { ls.inspectActive = false }()

	var out bytes.Buffer
	items := make([]string, 0)
	for _, e := range ls.items {
		items = append(items, e.Inspect())
	}
	out.WriteString("[")
	out.WriteString(strings.Join(items, ", "))
	out.WriteString("]")
	return out.String()
}

func (ls *List) GetAttr(name string) (Object, bool) {
	switch name {
	case "append":
		return &Builtin{
			name: "list.append",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("list.append", 1, len(args))
				}
				ls.Append(args[0])
				return ls
			},
		}, true
	case "clear":
		return &Builtin{
			name: "list.clear",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("list.clear", 0, len(args))
				}
				ls.Clear()
				return ls
			},
		}, true
	case "copy":
		return &Builtin{
			name: "list.copy",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("list.copy", 0, len(args))
				}
				return ls.Copy()
			},
		}, true
	case "count":
		return &Builtin{
			name: "list.count",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("list.count", 1, len(args))
				}
				return NewInt(ls.Count(args[0]))
			},
		}, true
	case "extend":
		return &Builtin{
			name: "list.extend",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("list.extend", 1, len(args))
				}
				other, err := AsList(args[0])
				if err != nil {
					return err
				}
				ls.Extend(other)
				return ls
			},
		}, true
	case "index":
		return &Builtin{
			name: "list.index",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("list.index", 1, len(args))
				}
				return NewInt(ls.Index(args[0]))
			},
		}, true
	case "insert":
		return &Builtin{
			name: "list.insert",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 2 {
					return NewArgsError("list.insert", 2, len(args))
				}
				index, err := AsInt(args[0])
				if err != nil {
					return err
				}
				ls.Insert(index, args[1])
				return ls
			},
		}, true
	case "pop":
		return &Builtin{
			name: "list.pop",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("list.pop", 1, len(args))
				}
				index, err := AsInt(args[0])
				if err != nil {
					return err
				}
				return ls.Pop(index)
			},
		}, true
	case "remove":
		return &Builtin{
			name: "list.remove",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("list.remove", 1, len(args))
				}
				ls.Remove(args[0])
				return ls
			},
		}, true
	case "reverse":
		return &Builtin{
			name: "list.reverse",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("list.reverse", 0, len(args))
				}
				ls.Reverse()
				return ls
			},
		}, true
	case "sort":
		return &Builtin{
			name: "list.sort",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 0 {
					return NewArgsError("list.sort", 0, len(args))
				}
				if err := Sort(ls.items); err != nil {
					return err
				}
				return ls
			},
		}, true
	case "map":
		return &Builtin{
			name: "list.map",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("list.map", 1, len(args))
				}
				return ls.Map(ctx, args[0])
			},
		}, true
	case "filter":
		return &Builtin{
			name: "list.filter",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("list.filter", 1, len(args))
				}
				return ls.Filter(ctx, args[0])
			},
		}, true
	case "each":
		return &Builtin{
			name: "list.each",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("list.each", 1, len(args))
				}
				return ls.Each(ctx, args[0])
			},
		}, true
	case "reduce":
		return &Builtin{
			name: "list.reduce",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 2 {
					return NewArgsError("list.reduce", 2, len(args))
				}
				return ls.Reduce(ctx, args[0], args[1])
			},
		}, true
	}
	return nil, false
}

func (ls *List) Map(ctx context.Context, fn Object) Object {
	callFunc, found := GetCallFunc(ctx)
	if !found {
		return EvalErrorf("eval error: list.map() context did not contain a call function")
	}
	var numParameters int
	switch obj := fn.(type) {
	case *Builtin:
		result := make([]Object, 0, len(ls.items))
		for _, value := range ls.items {
			outputValue := obj.fn(ctx, value)
			if IsError(outputValue) {
				return outputValue
			}
			result = append(result, outputValue)
		}
		return NewList(result)
	case *Function:
		numParameters = len(obj.parameters)
	default:
		return TypeErrorf("type error: list.map() expected a function (%s given)", obj.Type())
	}
	if numParameters < 1 || numParameters > 2 {
		return TypeErrorf("type error: list.map() received an incompatible function")
	}
	compiledFunc := fn.(*Function)
	var index Int
	mapArgs := make([]Object, 2)
	result := make([]Object, 0, len(ls.items))
	for i, value := range ls.items {
		index.value = int64(i)
		mapArgs[0] = &index
		mapArgs[1] = value
		var err error
		var outputValue Object
		if numParameters == 1 {
			outputValue, err = callFunc(ctx, compiledFunc, mapArgs[1:])
		} else {
			outputValue, err = callFunc(ctx, compiledFunc, mapArgs)
		}
		if err != nil {
			return Errorf(err.Error())
		}
		if IsError(outputValue) {
			return outputValue
		}
		result = append(result, outputValue)
	}
	return NewList(result)
}

func (ls *List) Filter(ctx context.Context, fn Object) Object {
	callFunc, found := GetCallFunc(ctx)
	if !found {
		return EvalErrorf("eval error: list.filter() context did not contain a call function")
	}
	switch obj := fn.(type) {
	case *Function, *Builtin:
		// Nothing do do here
	default:
		return TypeErrorf("type error: list.filter() expected a function (%s given)", obj.Type())
	}
	filterArgs := make([]Object, 1)
	var result []Object
	for _, value := range ls.items {
		filterArgs[0] = value
		decision, err := callFunc(ctx, fn.(*Function), filterArgs)
		if err != nil {
			return Errorf(err.Error())
		}
		if IsError(decision) {
			return decision
		}
		if decision.IsTruthy() {
			result = append(result, value)
		}
	}
	return NewList(result)
}

func (ls *List) Each(ctx context.Context, fn Object) Object {
	callFunc, found := GetCallFunc(ctx)
	if !found {
		return EvalErrorf("eval error: list.each() context did not contain a call function")
	}
	switch obj := fn.(type) {
	case *Function, *Builtin:
		// Nothing do do here
	default:
		return TypeErrorf("type error: list.each() expected a function (%s given)", obj.Type())
	}
	eachArgs := make([]Object, 1)
	for _, value := range ls.items {
		eachArgs[0] = value
		result, err := callFunc(ctx, fn.(*Function), eachArgs)
		if err != nil {
			return Errorf(err.Error())
		}
		if IsError(result) {
			return result
		}
	}
	return Nil
}

func (ls *List) Reduce(ctx context.Context, initial Object, fn Object) Object {
	callFunc, found := GetCallFunc(ctx)
	if !found {
		return EvalErrorf("eval error: list.reduce() context did not contain a call function")
	}
	switch obj := fn.(type) {
	case *Builtin:
		accumulator := initial
		for _, value := range ls.items {
			result := obj.fn(ctx, accumulator, value)
			if IsError(result) {
				return result
			}
			accumulator = result
		}
		return accumulator
	case *Function:
		accumulator := initial
		reduceArgs := make([]Object, 2)
		for _, value := range ls.items {
			reduceArgs[0] = accumulator
			reduceArgs[1] = value
			result, err := callFunc(ctx, obj, reduceArgs)
			if err != nil {
				return Errorf(err.Error())
			}
			if IsError(result) {
				return result
			}
			accumulator = result
		}
		return accumulator
	default:
		return TypeErrorf("type error: list.reduce() expected a function (%s given)", obj.Type())
	}
}

// Append adds an item at the end of the list.
func (ls *List) Append(obj Object) {
	ls.items = append(ls.items, obj)
}

// Clear removes all the items from the list.
func (ls *List) Clear() {
	ls.items = []Object{}
}

// Copy returns a shallow copy of the list.
func (ls *List) Copy() *List {
	result := &List{items: make([]Object, len(ls.items))}
	copy(result.items, ls.items)
	return result
}

// Count returns the number of items with the specified value.
func (ls *List) Count(obj Object) int64 {
	count := int64(0)
	for _, item := range ls.items {
		if Equals(obj, item) {
			count++
		}
	}
	return count
}

// Extend adds the items of a list to the end of the current list.
func (ls *List) Extend(other *List) {
	ls.items = append(ls.items, other.items...)
}

// Index returns the index of the first item with the specified value.
func (ls *List) Index(obj Object) int64 {
	for i, item := range ls.items {
		if Equals(obj, item) {
			return int64(i)
		}
	}
	return int64(-1)
}

// Insert adds an item at the specified position.
func (ls *List) Insert(index int64, obj Object) {
	// Negative index is relative to the end of the list
	if index < 0 {
		index = int64(len(ls.items)) + index
		if index < 0 {
			index = 0
		}
	}
	if index == 0 {
		ls.items = append([]Object{obj}, ls.items...)
		return
	}
	if index >= int64(len(ls.items)) {
		ls.items = append(ls.items, obj)
		return
	}
	ls.items = append(ls.items, nil)
	copy(ls.items[index+1:], ls.items[index:])
	ls.items[index] = obj
}

// Pop removes the item at the specified position.
func (ls *List) Pop(index int64) Object {
	idx, err := ResolveIndex(index, int64(len(ls.items)))
	if err != nil {
		return Errorf(err.Error())
	}
	result := ls.items[idx]
	ls.items = append(ls.items[:idx], ls.items[idx+1:]...)
	return result
}

// Remove removes the first item with the specified value.
func (ls *List) Remove(obj Object) {
	index := ls.Index(obj)
	if index == -1 {
		return
	}
	ls.items = append(ls.items[:index], ls.items[index+1:]...)
}

// Reverse reverses the order of the list.
func (ls *List) Reverse() {
	for i, j := 0, len(ls.items)-1; i < j; i, j = i+1, j-1 {
		ls.items[i], ls.items[j] = ls.items[j], ls.items[i]
	}
}

func (ls *List) Interface() interface{} {
	items := make([]interface{}, 0, len(ls.items))
	for _, item := range ls.items {
		items = append(items, item.Interface())
	}
	return items
}

func (ls *List) String() string {
	return ls.Inspect()
}

func (ls *List) Compare(other Object) (int, error) {
	otherList, ok := other.(*List)
	if !ok {
		return 0, errz.TypeErrorf("type error: unable to compare list and %s", other.Type())
	}
	if len(ls.items) > len(otherList.items) {
		return 1, nil
	} else if len(ls.items) < len(otherList.items) {
		return -1, nil
	}
	for i := 0; i < len(ls.items); i++ {
		comparable, ok := ls.items[i].(Comparable)
		if !ok {
			return 0, errz.TypeErrorf("type error: %s object is not comparable", ls.items[i].Type())
		}
		comp, err := comparable.Compare(otherList.items[i])
		if err != nil {
			return 0, err
		}
		if comp != 0 {
			return comp, nil
		}
	}
	return 0, nil
}

func (ls *List) Equals(other Object) Object {
	if other.Type() != LIST {
		return False
	}
	otherList := other.(*List)
	if len(ls.items) != len(otherList.items) {
		return False
	}
	for i, v := range ls.items {
		otherV := otherList.items[i]
		if !Equals(v, otherV) {
			return False
		}
	}
	return True
}

func (ls *List) IsTruthy() bool {
	return len(ls.items) > 0
}

func (ls *List) Reversed() *List {
	result := &List{items: make([]Object, 0, len(ls.items))}
	size := len(ls.items)
	for i := 0; i < size; i++ {
		result.items = append(result.items, ls.items[size-1-i])
	}
	return result
}

func (ls *List) Keys() Object {
	items := make([]Object, 0, len(ls.items))
	for i := 0; i < len(ls.items); i++ {
		items = append(items, NewInt(int64(i)))
	}
	return NewList(items)
}

func (ls *List) GetItem(key Object) (Object, *Error) {
	indexObj, ok := key.(*Int)
	if !ok {
		return nil, TypeErrorf("type error: list index must be an int (got %s)", key.Type())
	}
	idx, err := ResolveIndex(indexObj.value, int64(len(ls.items)))
	if err != nil {
		return nil, Errorf(err.Error())
	}
	return ls.items[idx], nil
}

// GetSlice implements the [start:stop] operator for a container type.
func (ls *List) GetSlice(s Slice) (Object, *Error) {
	start, stop, err := ResolveIntSlice(s, int64(len(ls.items)))
	if err != nil {
		return nil, Errorf(err.Error())
	}
	items := ls.items[start:stop]
	itemsCopy := make([]Object, len(items))
	copy(itemsCopy, items)
	return NewList(itemsCopy), nil
}

// SetItem implements the [key] = value operator for a container type.
func (ls *List) SetItem(key, value Object) *Error {
	indexObj, ok := key.(*Int)
	if !ok {
		return TypeErrorf("type error: list index must be an int (got %s)", key.Type())
	}
	idx, err := ResolveIndex(indexObj.value, int64(len(ls.items)))
	if err != nil {
		return Errorf(err.Error())
	}
	ls.items[idx] = value
	return nil
}

// DelItem implements the del [key] operator for a container type.
func (ls *List) DelItem(key Object) *Error {
	indexObj, ok := key.(*Int)
	if !ok {
		return TypeErrorf("type error: list index must be an int (got %s)", key.Type())
	}
	idx, err := ResolveIndex(indexObj.value, int64(len(ls.items)))
	if err != nil {
		return Errorf(err.Error())
	}
	ls.items = append(ls.items[:idx], ls.items[idx+1:]...)
	return nil
}

// Contains returns true if the given item is found in this container.
func (ls *List) Contains(item Object) *Bool {
	for _, v := range ls.items {
		if Equals(v, item) {
			return True
		}
	}
	return False
}

// Len returns the number of items in this container.
func (ls *List) Len() *Int {
	return NewInt(int64(len(ls.items)))
}

func (ls *List) Size() int {
	return len(ls.items)
}

func (ls *List) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	for i, item := range ls.items {
		if !fn(NewInt(int64(i)), item) {
			return
		}
	}
}

func (ls *List) RunOperation(opType op.BinaryOpType, right Object) Object {
	switch right := right.(type) {
	case *List:
		return ls.runOperationList(opType, right)
	default:
		return TypeErrorf("type error: unsupported operation for list: %v on type %s",
			opType, right.Type())
	}
}

func (ls *List) runOperationList(opType op.BinaryOpType, right *List) Object {
	switch opType {
	case op.Add:
		combined := make([]Object, len(ls.items)+len(right.items))
		copy(combined, ls.items)
		copy(combined[len(ls.items):], right.items)
		return NewList(combined)
	default:
		return TypeErrorf("type error: unsupported operation for list: %v on type %s",
			opType, right.Type())
	}
}

func (ls *List) Cost() int {
	// It would be possible to recurse into the list and compute the cost of
	// each item, but let's avoid that since it would be an expensive op itself.
	return len(ls.items) * 8
}

func (ls *List) MarshalJSON() ([]byte, error) {
	return json.Marshal(ls.items)
}

func NewList(items []Object) *List {
	return &List{items: items}
}

func NewStringList(s []string) *List {
	array := &List{items: make([]Object, 0, len(s))}
	for _, item := range s {
		array.items = append(array.items, NewString(item))
	}
	return array
}

// ResolveIndex checks that the index is inbounds and transforms a negative
// index into the corresponding positive index. If the index is out of bounds,
// an error is returned.
func ResolveIndex(idx int64, size int64) (int64, error) {
	max := size - 1
	if idx > max {
		return 0, fmt.Errorf("index error: index out of range: %d", idx)
	}
	if idx >= 0 {
		return idx, nil
	}
	// Handle negative indices, where -1 is the last item in the array
	reversed := idx + size
	if reversed < 0 || reversed > max {
		return 0, fmt.Errorf("index error: index out of range: %d", idx)
	}
	return reversed, nil
}

// ResolveIntSlice checks that the slice start and stop indices are inbounds and
// transforms negative indices into the corresponding positive indices. If the
// slice is out of bounds, an error is returned.
func ResolveIntSlice(slice Slice, size int64) (start int64, stop int64, err error) {
	if slice.Start != nil {
		startObj, ok := slice.Start.(*Int)
		if !ok {
			err = errz.TypeErrorf("type error: slice start index must be an int (got %s)", slice.Start.Type())
			return
		}
		start = startObj.value
	}
	if slice.Stop != nil {
		stopObj, ok := slice.Stop.(*Int)
		if !ok {
			err = errz.TypeErrorf("type error: slice stop index must be an int (got %s)", slice.Stop.Type())
			return
		}
		stop = stopObj.value
	} else {
		stop = size
	}
	if start < 0 {
		start = size + start
		if start < 0 {
			err = fmt.Errorf("slice error: start index is out of range")
			return
		}
	}
	if stop < 0 {
		stop = size + stop
		if stop < 0 {
			err = fmt.Errorf("slice error: stop index is out of range")
			return
		}
	}
	if start > stop {
		err = fmt.Errorf("slice error: start index is greater than stop index")
		return
	}
	if start > size-1 {
		err = fmt.Errorf("slice error: start index is out of range")
		return
	}
	if stop > size {
		err = fmt.Errorf("slice error: stop index is out of range")
		return
	}
	return start, stop, nil
}
