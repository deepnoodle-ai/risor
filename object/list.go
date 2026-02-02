package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/op"
)

var listMethods = NewMethodRegistry[*List]("list")

func init() {
	listMethods.Define("append").
		Doc("Add item to end of list").
		Arg("item").
		Returns("list").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			ls.Append(args[0])
			return ls, nil
		})

	listMethods.Define("clear").
		Doc("Remove all items").
		Returns("list").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			ls.Clear()
			return ls, nil
		})

	listMethods.Define("copy").
		Doc("Create a shallow copy").
		Returns("list").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			return ls.Copy(), nil
		})

	listMethods.Define("count").
		Doc("Count occurrences of item").
		Arg("item").
		Returns("int").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			return NewInt(ls.Count(args[0])), nil
		})

	listMethods.Define("each").
		Doc("Call function for each item").
		Arg("fn").
		Returns("nil").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			return ls.Each(ctx, args[0])
		})

	listMethods.Define("extend").
		Doc("Add all items from another list").
		Arg("items").
		Returns("list").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			other, err := AsList(args[0])
			if err != nil {
				return nil, err
			}
			ls.Extend(other)
			return ls, nil
		})

	listMethods.Define("filter").
		Doc("Keep items where fn returns true").
		Arg("fn").
		Returns("list").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			return ls.Filter(ctx, args[0])
		})

	listMethods.Define("index").
		Doc("Find first index of item (-1 if not found)").
		Arg("item").
		Returns("int").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			return NewInt(ls.Index(args[0])), nil
		})

	listMethods.Define("insert").
		Doc("Insert item at index").
		Args("index", "item").
		Returns("list").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			index, err := AsInt(args[0])
			if err != nil {
				return nil, err
			}
			ls.Insert(index, args[1])
			return ls, nil
		})

	listMethods.Define("map").
		Doc("Transform each item with fn").
		Arg("fn").
		Returns("list").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			return ls.Map(ctx, args[0])
		})

	listMethods.Define("pop").
		Doc("Remove and return item at index").
		Arg("index").
		Returns("any").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			index, err := AsInt(args[0])
			if err != nil {
				return nil, err
			}
			return ls.Pop(index)
		})

	listMethods.Define("reduce").
		Doc("Reduce list to single value").
		Args("initial", "fn").
		Returns("any").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			return ls.Reduce(ctx, args[0], args[1])
		})

	listMethods.Define("remove").
		Doc("Remove first occurrence of item").
		Arg("item").
		Returns("nil").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			ls.Remove(args[0])
			return ls, nil
		})

	listMethods.Define("reverse").
		Doc("Reverse list in place").
		Returns("list").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			ls.Reverse()
			return ls, nil
		})

	listMethods.Define("sort").
		Doc("Sort list in place").
		Returns("list").
		Impl(func(ls *List, ctx context.Context, args ...Object) (Object, error) {
			if err := Sort(ls.items); err != nil {
				return nil, err
			}
			return ls, nil
		})
}

// List of objects
type List struct {
	// items holds the list of objects
	items []Object

	// Used to avoid the possibility of infinite recursion when inspecting.
	// Similar to the usage of Py_ReprEnter in CPython.
	inspectActive bool
}

func (ls *List) Attrs() []AttrSpec {
	return listMethods.Specs()
}

func (ls *List) GetAttr(name string) (Object, bool) {
	return listMethods.GetAttr(ls, name)
}

func (ls *List) SetAttr(name string, value Object) error {
	return TypeErrorf("list has no attribute %q", name)
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

func (ls *List) Map(ctx context.Context, fn Object) (Object, error) {
	callable, ok := fn.(Callable)
	if !ok {
		return nil, newTypeErrorf("list.map() expected a function (%s given)", fn.Type())
	}
	// Closures can accept (index, value) if they have 2 parameters
	var passIndex bool
	if closure, ok := fn.(*Closure); ok {
		count := closure.ParameterCount()
		if count < 1 || count > 2 {
			return nil, newTypeErrorf("list.map() received an incompatible function")
		}
		passIndex = count == 2
	}
	result := make([]Object, 0, len(ls.items))
	for i, value := range ls.items {
		var outputValue Object
		var err error
		if passIndex {
			outputValue, err = callable.Call(ctx, NewInt(int64(i)), value)
		} else {
			outputValue, err = callable.Call(ctx, value)
		}
		if err != nil {
			return nil, err
		}
		result = append(result, outputValue)
	}
	return NewList(result), nil
}

func (ls *List) Filter(ctx context.Context, fn Object) (Object, error) {
	callable, ok := fn.(Callable)
	if !ok {
		return nil, newTypeErrorf("list.filter() expected a function (%s given)", fn.Type())
	}
	var result []Object
	for _, value := range ls.items {
		decision, err := callable.Call(ctx, value)
		if err != nil {
			return nil, err
		}
		if decision.IsTruthy() {
			result = append(result, value)
		}
	}
	return NewList(result), nil
}

func (ls *List) Each(ctx context.Context, fn Object) (Object, error) {
	callable, ok := fn.(Callable)
	if !ok {
		return nil, newTypeErrorf("list.each() expected a function (%s given)", fn.Type())
	}
	for _, value := range ls.items {
		if _, err := callable.Call(ctx, value); err != nil {
			return nil, err
		}
	}
	return Nil, nil
}

func (ls *List) Reduce(ctx context.Context, initial Object, fn Object) (Object, error) {
	callable, ok := fn.(Callable)
	if !ok {
		return nil, newTypeErrorf("list.reduce() expected a function (%s given)", fn.Type())
	}
	accumulator := initial
	for _, value := range ls.items {
		result, err := callable.Call(ctx, accumulator, value)
		if err != nil {
			return nil, err
		}
		accumulator = result
	}
	return accumulator, nil
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
func (ls *List) Pop(index int64) (Object, error) {
	idx, err := ResolveIndex(index, int64(len(ls.items)))
	if err != nil {
		return nil, err
	}
	result := ls.items[idx]
	ls.items = append(ls.items[:idx], ls.items[idx+1:]...)
	return result, nil
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
		return 0, TypeErrorf("unable to compare list and %s", other.Type())
	}
	if len(ls.items) > len(otherList.items) {
		return 1, nil
	} else if len(ls.items) < len(otherList.items) {
		return -1, nil
	}
	for i := 0; i < len(ls.items); i++ {
		comparable, ok := ls.items[i].(Comparable)
		if !ok {
			return 0, TypeErrorf("%s object is not comparable", ls.items[i].Type())
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

func (ls *List) Equals(other Object) bool {
	otherList, ok := other.(*List)
	if !ok {
		return false
	}
	if len(ls.items) != len(otherList.items) {
		return false
	}
	for i, v := range ls.items {
		otherV := otherList.items[i]
		if !Equals(v, otherV) {
			return false
		}
	}
	return true
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
		return nil, TypeErrorf("list index must be an int (got %s)", key.Type())
	}
	idx, err := ResolveIndex(indexObj.value, int64(len(ls.items)))
	if err != nil {
		return nil, NewError(err)
	}
	return ls.items[idx], nil
}

// GetSlice implements the [start:stop] operator for a container type.
func (ls *List) GetSlice(s Slice) (Object, *Error) {
	start, stop, err := ResolveIntSlice(s, int64(len(ls.items)))
	if err != nil {
		return nil, NewError(err)
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
		return TypeErrorf("list index must be an int (got %s)", key.Type())
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
		return TypeErrorf("list index must be an int (got %s)", key.Type())
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

func (ls *List) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	switch right := right.(type) {
	case *List:
		return ls.runOperationList(opType, right)
	default:
		return nil, newTypeErrorf("unsupported operation for list: %v on type %s",
			opType, right.Type())
	}
}

func (ls *List) runOperationList(opType op.BinaryOpType, right *List) (Object, error) {
	switch opType {
	case op.Add:
		combined := make([]Object, len(ls.items)+len(right.items))
		copy(combined, ls.items)
		copy(combined[len(ls.items):], right.items)
		return NewList(combined), nil
	default:
		return nil, newTypeErrorf("unsupported operation for list: %v on type %s",
			opType, right.Type())
	}
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
		return 0, newIndexErrorf("index out of range: %d", idx)
	}
	if idx >= 0 {
		return idx, nil
	}
	// Handle negative indices, where -1 is the last item in the array
	reversed := idx + size
	if reversed < 0 || reversed > max {
		return 0, newIndexErrorf("index out of range: %d", idx)
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
			err = TypeErrorf("slice start index must be an int (got %s)", slice.Start.Type())
			return
		}
		start = startObj.value
	}
	if slice.Stop != nil {
		stopObj, ok := slice.Stop.(*Int)
		if !ok {
			err = TypeErrorf("slice stop index must be an int (got %s)", slice.Stop.Type())
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
