package object

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

func TestListType(t *testing.T) {
	list := NewList(nil)
	assert.Equal(t, list.Type(), LIST)
}

func TestListValue(t *testing.T) {
	items := []Object{NewInt(1), NewInt(2)}
	list := NewList(items)
	assert.Equal(t, list.Value(), items)
}

func TestListInspect(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewString("hello")})
	assert.Equal(t, list.Inspect(), `[1, "hello"]`)
}

func TestListInspectEmpty(t *testing.T) {
	list := NewList(nil)
	assert.Equal(t, list.Inspect(), "[]")
}

func TestListInspectSelfReference(t *testing.T) {
	list := NewList(nil)
	list.Append(list)
	// Should handle self-reference without infinite loop
	inspect := list.Inspect()
	assert.Equal(t, inspect, "[[...]]")
}

func TestListString(t *testing.T) {
	list := NewList([]Object{NewInt(1)})
	assert.Equal(t, list.String(), "[1]")
}

func TestListInsert(t *testing.T) {
	one := NewInt(1)
	two := NewInt(2)
	thr := NewInt(3)

	list := NewList([]Object{one})

	list.Insert(5, two)
	assert.Equal(t, list.Value(), []Object{one, two})

	list.Insert(-10, thr)
	assert.Equal(t, list.Value(), []Object{thr, one, two})

	list.Insert(1, two)
	assert.Equal(t, list.Value(), []Object{thr, two, one, two})

	list.Insert(0, two)
	assert.Equal(t, list.Value(), []Object{two, thr, two, one, two})
}

func TestListPop(t *testing.T) {
	zero := NewString("0")
	one := NewString("1")
	two := NewString("2")

	list := NewList([]Object{zero, one, two})

	result, err := list.Pop(1)
	assert.Nil(t, err)
	val, ok := result.(*String)
	assert.True(t, ok)
	assert.Equal(t, val.Value(), "1")

	result, err = list.Pop(1)
	assert.Nil(t, err)
	val, ok = result.(*String)
	assert.True(t, ok)
	assert.Equal(t, val.Value(), "2")

	_, err = list.Pop(1)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "index error: index out of range: 1")
}

func TestListAppend(t *testing.T) {
	list := NewList([]Object{NewInt(1)})
	list.Append(NewInt(2))
	assert.Equal(t, list.Len().Value(), int64(2))
	assert.Equal(t, list.Value()[1].(*Int).Value(), int64(2))
}

func TestListClear(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2)})
	list.Clear()
	assert.Equal(t, list.Len().Value(), int64(0))
}

func TestListCopy(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2)})
	copyList := list.Copy()

	// Same values
	assert.Equal(t, copyList.Len().Value(), int64(2))

	// But different object
	list.Clear()
	assert.Equal(t, copyList.Len().Value(), int64(2))
}

func TestListCount(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(1), NewInt(1)})
	assert.Equal(t, list.Count(NewInt(1)), int64(3))
	assert.Equal(t, list.Count(NewInt(2)), int64(1))
	assert.Equal(t, list.Count(NewInt(99)), int64(0))
}

func TestListExtend(t *testing.T) {
	list1 := NewList([]Object{NewInt(1), NewInt(2)})
	list2 := NewList([]Object{NewInt(3), NewInt(4)})
	list1.Extend(list2)
	assert.Equal(t, list1.Len().Value(), int64(4))
}

func TestListIndex(t *testing.T) {
	list := NewList([]Object{NewString("a"), NewString("b"), NewString("c")})
	assert.Equal(t, list.Index(NewString("b")), int64(1))
	assert.Equal(t, list.Index(NewString("x")), int64(-1))
}

func TestListRemove(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	list.Remove(NewInt(2))
	assert.Equal(t, list.Len().Value(), int64(2))

	// Remove non-existent item (no-op)
	list.Remove(NewInt(99))
	assert.Equal(t, list.Len().Value(), int64(2))
}

func TestListReverse(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	list.Reverse()
	assert.Equal(t, list.Value()[0].(*Int).Value(), int64(3))
	assert.Equal(t, list.Value()[1].(*Int).Value(), int64(2))
	assert.Equal(t, list.Value()[2].(*Int).Value(), int64(1))
}

func TestListReversed(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	reversed := list.Reversed()

	// Original unchanged
	assert.Equal(t, list.Value()[0].(*Int).Value(), int64(1))

	// Reversed copy
	assert.Equal(t, reversed.Value()[0].(*Int).Value(), int64(3))
}

func TestListInterface(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewString("hello")})
	iface := list.Interface()
	result := iface.([]interface{})
	assert.Equal(t, result[0], int64(1))
	assert.Equal(t, result[1], "hello")
}

func TestListCompare(t *testing.T) {
	list1 := NewList([]Object{NewInt(1), NewInt(2)})
	list2 := NewList([]Object{NewInt(1), NewInt(2)})
	list3 := NewList([]Object{NewInt(1)})
	list4 := NewList([]Object{NewInt(1), NewInt(3)})

	// Equal
	cmp, err := list1.Compare(list2)
	assert.Nil(t, err)
	assert.Equal(t, cmp, 0)

	// Different sizes
	cmp, err = list1.Compare(list3)
	assert.Nil(t, err)
	assert.True(t, cmp > 0)

	cmp, err = list3.Compare(list1)
	assert.Nil(t, err)
	assert.True(t, cmp < 0)

	// Same size, different values
	cmp, err = list1.Compare(list4)
	assert.Nil(t, err)
	assert.True(t, cmp < 0) // 2 < 3

	// Different type
	_, err = list1.Compare(NewInt(1))
	assert.NotNil(t, err)
}

func TestListEquals(t *testing.T) {
	list1 := NewList([]Object{NewInt(1), NewInt(2)})
	list2 := NewList([]Object{NewInt(1), NewInt(2)})
	list3 := NewList([]Object{NewInt(1)})
	list4 := NewList([]Object{NewInt(1), NewInt(3)})

	assert.True(t, list1.Equals(list2))
	assert.False(t, list1.Equals(list3))
	assert.False(t, list1.Equals(list4))
	assert.False(t, list1.Equals(NewString("test")))
}

func TestListIsTruthy(t *testing.T) {
	assert.False(t, NewList(nil).IsTruthy())
	assert.False(t, NewList([]Object{}).IsTruthy())
	assert.True(t, NewList([]Object{NewInt(1)}).IsTruthy())
}

func TestListKeys(t *testing.T) {
	list := NewList([]Object{NewString("a"), NewString("b"), NewString("c")})
	keys := list.Keys().(*List)
	assert.Equal(t, keys.Len().Value(), int64(3))
	assert.Equal(t, keys.Value()[0].(*Int).Value(), int64(0))
	assert.Equal(t, keys.Value()[1].(*Int).Value(), int64(1))
	assert.Equal(t, keys.Value()[2].(*Int).Value(), int64(2))
}

func TestListGetItem(t *testing.T) {
	list := NewList([]Object{NewInt(10), NewInt(20), NewInt(30)})

	// Valid index
	val, err := list.GetItem(NewInt(1))
	assert.Nil(t, err)
	assert.Equal(t, val.(*Int).Value(), int64(20))

	// Negative index
	val, err = list.GetItem(NewInt(-1))
	assert.Nil(t, err)
	assert.Equal(t, val.(*Int).Value(), int64(30))

	// Out of range
	_, err = list.GetItem(NewInt(10))
	assert.NotNil(t, err)

	// Wrong type
	_, err = list.GetItem(NewString("test"))
	assert.NotNil(t, err)
}

func TestListGetSlice(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4), NewInt(5)})

	// Valid slice
	result, err := list.GetSlice(Slice{Start: NewInt(1), Stop: NewInt(3)})
	assert.Nil(t, err)
	resultList := result.(*List)
	assert.Equal(t, resultList.Len().Value(), int64(2))

	// Nil start
	result, err = list.GetSlice(Slice{Start: nil, Stop: NewInt(2)})
	assert.Nil(t, err)
	assert.Equal(t, result.(*List).Len().Value(), int64(2))

	// Nil stop
	result, err = list.GetSlice(Slice{Start: NewInt(3), Stop: nil})
	assert.Nil(t, err)
	assert.Equal(t, result.(*List).Len().Value(), int64(2))
}

func TestListSetItem(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})

	// Valid set
	err := list.SetItem(NewInt(1), NewInt(99))
	assert.Nil(t, err)
	assert.Equal(t, list.Value()[1].(*Int).Value(), int64(99))

	// Wrong index type
	err = list.SetItem(NewString("test"), NewInt(1))
	assert.NotNil(t, err)

	// Out of range
	err = list.SetItem(NewInt(10), NewInt(1))
	assert.NotNil(t, err)
}

func TestListDelItem(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})

	// Valid delete
	err := list.DelItem(NewInt(1))
	assert.Nil(t, err)
	assert.Equal(t, list.Len().Value(), int64(2))

	// Wrong type
	err = list.DelItem(NewString("test"))
	assert.NotNil(t, err)

	// Out of range
	err = list.DelItem(NewInt(10))
	assert.NotNil(t, err)
}

func TestListContains(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	assert.Equal(t, list.Contains(NewInt(2)), True)
	assert.Equal(t, list.Contains(NewInt(99)), False)
}

func TestListLen(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	assert.Equal(t, list.Len().Value(), int64(3))
}

func TestListSize(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	assert.Equal(t, list.Size(), 3)
}

func TestListEnumerate(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewString("a"), NewString("b"), NewString("c")})

	var indices []int64
	var values []string
	list.Enumerate(ctx, func(key, value Object) bool {
		indices = append(indices, key.(*Int).Value())
		values = append(values, value.(*String).Value())
		return true
	})

	assert.Equal(t, indices, []int64{0, 1, 2})
	assert.Equal(t, values, []string{"a", "b", "c"})
}

func TestListEnumerateEarlyStop(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4), NewInt(5)})

	count := 0
	list.Enumerate(ctx, func(key, value Object) bool {
		count++
		return count < 2
	})

	assert.Equal(t, count, 2)
}

func TestListRunOperation(t *testing.T) {
	list1 := NewList([]Object{NewInt(1), NewInt(2)})
	list2 := NewList([]Object{NewInt(3), NewInt(4)})

	// Add lists
	result, err := list1.RunOperation(op.Add, list2)
	assert.Nil(t, err)
	resultList := result.(*List)
	assert.Equal(t, resultList.Len().Value(), int64(4))

	// Unsupported operation
	_, err = list1.RunOperation(op.Subtract, list2)
	assert.NotNil(t, err)

	// Unsupported type
	_, err = list1.RunOperation(op.Add, NewInt(1))
	assert.NotNil(t, err)
}

func TestListMarshalJSON(t *testing.T) {
	list := NewList([]Object{NewInt(1), NewString("hello")})
	data, err := list.MarshalJSON()
	assert.Nil(t, err)
	assert.True(t, len(data) > 0)
}

func TestNewStringList(t *testing.T) {
	list := NewStringList([]string{"a", "b", "c"})
	assert.Equal(t, list.Len().Value(), int64(3))
	assert.Equal(t, list.Value()[0].(*String).Value(), "a")
}

func TestListGetAttrAppend(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1)})

	appendFn, ok := list.GetAttr("append")
	assert.True(t, ok)

	result, err := appendFn.(*Builtin).Call(ctx, NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t, result, list)
	assert.Equal(t, list.Len().Value(), int64(2))
}

func TestListGetAttrAppendError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	appendFn, _ := list.GetAttr("append")

	_, err := appendFn.(*Builtin).Call(ctx)
	assert.NotNil(t, err)
}

func TestListGetAttrClear(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1), NewInt(2)})

	clear, ok := list.GetAttr("clear")
	assert.True(t, ok)

	result, err := clear.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.Equal(t, result, list)
	assert.Equal(t, list.Len().Value(), int64(0))
}

func TestListGetAttrClearError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	clear, _ := list.GetAttr("clear")
	_, err := clear.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestListGetAttrCopy(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1)})

	copyFn, ok := list.GetAttr("copy")
	assert.True(t, ok)

	result, err := copyFn.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.Equal(t, result.(*List).Len().Value(), int64(1))
}

func TestListGetAttrCopyError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	copyFn, _ := list.GetAttr("copy")
	_, err := copyFn.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestListGetAttrCount(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(1)})

	count, ok := list.GetAttr("count")
	assert.True(t, ok)

	result, err := count.(*Builtin).Call(ctx, NewInt(1))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(2))
}

func TestListGetAttrCountError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	count, _ := list.GetAttr("count")
	_, err := count.(*Builtin).Call(ctx)
	assert.NotNil(t, err)
}

func TestListGetAttrExtend(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1)})

	extend, ok := list.GetAttr("extend")
	assert.True(t, ok)

	result, err := extend.(*Builtin).Call(ctx, NewList([]Object{NewInt(2), NewInt(3)}))
	assert.Nil(t, err)
	assert.Equal(t, result, list)
	assert.Equal(t, list.Len().Value(), int64(3))
}

func TestListGetAttrExtendError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	extend, _ := list.GetAttr("extend")

	// Wrong arg count
	_, err := extend.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = extend.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestListGetAttrIndex(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewString("a"), NewString("b"), NewString("c")})

	index, ok := list.GetAttr("index")
	assert.True(t, ok)

	result, err := index.(*Builtin).Call(ctx, NewString("b"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(1))
}

func TestListGetAttrIndexError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	index, _ := list.GetAttr("index")
	_, err := index.(*Builtin).Call(ctx)
	assert.NotNil(t, err)
}

func TestListGetAttrInsert(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1), NewInt(3)})

	insert, ok := list.GetAttr("insert")
	assert.True(t, ok)

	result, err := insert.(*Builtin).Call(ctx, NewInt(1), NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t, result, list)
	assert.Equal(t, list.Len().Value(), int64(3))
}

func TestListGetAttrInsertError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	insert, _ := list.GetAttr("insert")

	// Wrong arg count
	_, err := insert.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)

	// Wrong type for index
	_, err = insert.(*Builtin).Call(ctx, NewString("x"), NewInt(1))
	assert.NotNil(t, err)
}

func TestListGetAttrPop(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})

	pop, ok := list.GetAttr("pop")
	assert.True(t, ok)

	result, err := pop.(*Builtin).Call(ctx, NewInt(1))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(2))
	assert.Equal(t, list.Len().Value(), int64(2))
}

func TestListGetAttrPopError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	pop, _ := list.GetAttr("pop")

	// Wrong arg count
	_, err := pop.(*Builtin).Call(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = pop.(*Builtin).Call(ctx, NewString("x"))
	assert.NotNil(t, err)
}

func TestListGetAttrRemove(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})

	remove, ok := list.GetAttr("remove")
	assert.True(t, ok)

	result, err := remove.(*Builtin).Call(ctx, NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t, result, list)
	assert.Equal(t, list.Len().Value(), int64(2))
}

func TestListGetAttrRemoveError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	remove, _ := list.GetAttr("remove")
	_, err := remove.(*Builtin).Call(ctx)
	assert.NotNil(t, err)
}

func TestListGetAttrReverse(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})

	reverse, ok := list.GetAttr("reverse")
	assert.True(t, ok)

	result, err := reverse.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.Equal(t, result, list)
	assert.Equal(t, list.Value()[0].(*Int).Value(), int64(3))
}

func TestListGetAttrReverseError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	reverse, _ := list.GetAttr("reverse")
	_, err := reverse.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestListGetAttrSort(t *testing.T) {
	ctx := context.Background()
	list := NewList([]Object{NewInt(3), NewInt(1), NewInt(2)})

	sort, ok := list.GetAttr("sort")
	assert.True(t, ok)

	result, err := sort.(*Builtin).Call(ctx)
	assert.Nil(t, err)
	assert.Equal(t, result, list)
	assert.Equal(t, list.Value()[0].(*Int).Value(), int64(1))
	assert.Equal(t, list.Value()[1].(*Int).Value(), int64(2))
	assert.Equal(t, list.Value()[2].(*Int).Value(), int64(3))
}

func TestListGetAttrSortError(t *testing.T) {
	ctx := context.Background()
	list := NewList(nil)
	sort, _ := list.GetAttr("sort")
	_, err := sort.(*Builtin).Call(ctx, NewInt(1))
	assert.NotNil(t, err)
}

func TestListGetAttrInvalid(t *testing.T) {
	list := NewList(nil)
	_, ok := list.GetAttr("invalid_method")
	assert.False(t, ok)
}

func TestResolveIndex(t *testing.T) {
	// Positive indices
	idx, err := ResolveIndex(0, 5)
	assert.Nil(t, err)
	assert.Equal(t, idx, int64(0))

	idx, err = ResolveIndex(4, 5)
	assert.Nil(t, err)
	assert.Equal(t, idx, int64(4))

	// Negative indices
	idx, err = ResolveIndex(-1, 5)
	assert.Nil(t, err)
	assert.Equal(t, idx, int64(4))

	idx, err = ResolveIndex(-5, 5)
	assert.Nil(t, err)
	assert.Equal(t, idx, int64(0))

	// Out of range
	_, err = ResolveIndex(5, 5)
	assert.NotNil(t, err)

	_, err = ResolveIndex(-6, 5)
	assert.NotNil(t, err)
}

func TestResolveIntSlice(t *testing.T) {
	// Basic slice
	start, stop, err := ResolveIntSlice(Slice{Start: NewInt(1), Stop: NewInt(3)}, 5)
	assert.Nil(t, err)
	assert.Equal(t, start, int64(1))
	assert.Equal(t, stop, int64(3))

	// Negative start
	start, stop, err = ResolveIntSlice(Slice{Start: NewInt(-2), Stop: NewInt(5)}, 5)
	assert.Nil(t, err)
	assert.Equal(t, start, int64(3))
	assert.Equal(t, stop, int64(5))

	// Negative stop
	start, stop, err = ResolveIntSlice(Slice{Start: NewInt(0), Stop: NewInt(-1)}, 5)
	assert.Nil(t, err)
	assert.Equal(t, start, int64(0))
	assert.Equal(t, stop, int64(4))

	// Wrong start type
	_, _, err = ResolveIntSlice(Slice{Start: NewString("x"), Stop: NewInt(3)}, 5)
	assert.NotNil(t, err)

	// Wrong stop type
	_, _, err = ResolveIntSlice(Slice{Start: NewInt(0), Stop: NewString("x")}, 5)
	assert.NotNil(t, err)

	// Start > stop
	_, _, err = ResolveIntSlice(Slice{Start: NewInt(3), Stop: NewInt(1)}, 5)
	assert.NotNil(t, err)

	// Start out of range
	_, _, err = ResolveIntSlice(Slice{Start: NewInt(10), Stop: NewInt(12)}, 5)
	assert.NotNil(t, err)

	// Stop out of range
	_, _, err = ResolveIntSlice(Slice{Start: NewInt(0), Stop: NewInt(10)}, 5)
	assert.NotNil(t, err)

	// Negative start out of range
	_, _, err = ResolveIntSlice(Slice{Start: NewInt(-10), Stop: NewInt(3)}, 5)
	assert.NotNil(t, err)

	// Negative stop out of range
	_, _, err = ResolveIntSlice(Slice{Start: NewInt(0), Stop: NewInt(-10)}, 5)
	assert.NotNil(t, err)
}

// mockCallFunc creates a context with a CallFunc that invokes the closure.
// This simulates what the VM does at runtime.
func mockCallFunc(ctx context.Context) context.Context {
	return WithCallFunc(ctx, func(ctx context.Context, fn *Closure, args []Object) (Object, error) {
		// In real usage, this would execute the closure's bytecode.
		// For testing, we can't actually run closures without the VM,
		// so we just test with builtins.
		return nil, nil
	})
}

func TestListMapWithBuiltin(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})

	// Create a builtin that doubles the value
	double := NewBuiltin("double", func(ctx context.Context, args ...Object) (Object, error) {
		if len(args) != 1 {
			return nil, TypeErrorf("expected 1 argument, got %d", len(args))
		}
		i, ok := args[0].(*Int)
		if !ok {
			return nil, TypeErrorf("expected int, got %s", args[0].Type())
		}
		return NewInt(i.Value() * 2), nil
	})

	result, err := list.Map(ctx, double)
	assert.Nil(t, err)

	expected := NewList([]Object{NewInt(2), NewInt(4), NewInt(6)})
	assert.True(t, Equals(result, expected))
}

func TestListMapNonCallableError(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2)})

	// Passing a non-callable should error
	_, err := list.Map(ctx, NewString("not a function"))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "type error")
	assert.Contains(t, err.Error(), "expected a function")
}

func TestListFilterWithBuiltin(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4)})

	// Create a builtin that returns true for even numbers
	isEven := NewBuiltin("is_even", func(ctx context.Context, args ...Object) (Object, error) {
		if len(args) != 1 {
			return nil, TypeErrorf("expected 1 argument, got %d", len(args))
		}
		i, ok := args[0].(*Int)
		if !ok {
			return False, nil
		}
		return NewBool(i.Value()%2 == 0), nil
	})

	result, err := list.Filter(ctx, isEven)
	assert.Nil(t, err)

	expected := NewList([]Object{NewInt(2), NewInt(4)})
	assert.True(t, Equals(result, expected))
}

func TestListFilterNonCallableError(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2)})

	// Passing a non-callable should error
	_, err := list.Filter(ctx, NewInt(42))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "type error")
	assert.Contains(t, err.Error(), "expected a function")
}

func TestListEachWithBuiltin(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})

	// Track which values were visited
	var visited []int64
	visitor := NewBuiltin("visitor", func(ctx context.Context, args ...Object) (Object, error) {
		if len(args) != 1 {
			return nil, TypeErrorf("expected 1 argument, got %d", len(args))
		}
		if i, ok := args[0].(*Int); ok {
			visited = append(visited, i.Value())
		}
		return Nil, nil
	})

	result, err := list.Each(ctx, visitor)
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)
	assert.Equal(t, visited, []int64{1, 2, 3})
}

func TestListEachNonCallableError(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1)})

	// Passing a non-callable should error
	_, err := list.Each(ctx, NewList(nil))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "type error")
	assert.Contains(t, err.Error(), "expected a function")
}

func TestListReduceWithBuiltin(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4)})

	// Create a builtin that sums two values
	sum := NewBuiltin("sum", func(ctx context.Context, args ...Object) (Object, error) {
		if len(args) != 2 {
			return nil, TypeErrorf("expected 2 arguments, got %d", len(args))
		}
		a, ok1 := args[0].(*Int)
		b, ok2 := args[1].(*Int)
		if !ok1 || !ok2 {
			return nil, TypeErrorf("expected int arguments")
		}
		return NewInt(a.Value() + b.Value()), nil
	})

	// Sum with initial value 0
	result, err := list.Reduce(ctx, NewInt(0), sum)
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(10))
}

func TestListReduceNonCallableError(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2)})

	// Passing a non-callable should error
	_, err := list.Reduce(ctx, NewInt(0), NewFloat(3.14))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "type error")
	assert.Contains(t, err.Error(), "expected a function")
}

func TestListReduceEmptyList(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{})

	sum := NewBuiltin("sum", func(ctx context.Context, args ...Object) (Object, error) {
		return nil, TypeErrorf("should not be called")
	})

	// Reduce on empty list returns initial value
	result, err := list.Reduce(ctx, NewInt(42), sum)
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(42))
}

func TestListMapBuiltinError(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2)})

	// Create a builtin that always errors
	alwaysError := NewBuiltin("error", func(ctx context.Context, args ...Object) (Object, error) {
		return nil, TypeErrorf("intentional error")
	})

	_, err := list.Map(ctx, alwaysError)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "intentional error")
}

func TestListFilterBuiltinError(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2)})

	// Create a builtin that always errors
	alwaysError := NewBuiltin("error", func(ctx context.Context, args ...Object) (Object, error) {
		return nil, TypeErrorf("filter error")
	})

	_, err := list.Filter(ctx, alwaysError)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "filter error")
}

func TestListEachBuiltinError(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2)})

	// Create a builtin that always errors
	alwaysError := NewBuiltin("error", func(ctx context.Context, args ...Object) (Object, error) {
		return nil, TypeErrorf("each error")
	})

	_, err := list.Each(ctx, alwaysError)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "each error")
}

func TestListReduceBuiltinError(t *testing.T) {
	ctx := mockCallFunc(context.Background())
	list := NewList([]Object{NewInt(1), NewInt(2)})

	// Create a builtin that always errors
	alwaysError := NewBuiltin("error", func(ctx context.Context, args ...Object) (Object, error) {
		return nil, TypeErrorf("reduce error")
	})

	_, err := list.Reduce(ctx, NewInt(0), alwaysError)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "reduce error")
}
