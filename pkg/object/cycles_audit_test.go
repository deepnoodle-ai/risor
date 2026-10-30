package object

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Tests for audit findings C-02 (cyclic containers), H-14 (DefaultRegistry
// init race), and H-15 (Inspect data race).

func selfList() *List {
	a := NewList(nil)
	a.Append(a)
	return a
}

func selfMap() *Map {
	m := NewMap(nil)
	m.Set("self", m)
	return m
}

// mixedCycle returns a list containing a map that contains the list.
func mixedCycle() *List {
	ls := NewList(nil)
	m := NewMap(nil)
	m.Set("list", ls)
	ls.Append(m)
	return ls
}

// nestedLists returns depth lists nested inside one another.
func nestedLists(depth int) *List {
	ls := NewList([]Object{NewInt(1)})
	for i := 1; i < depth; i++ {
		ls = NewList([]Object{ls})
	}
	return ls
}

func TestCycleInspect(t *testing.T) {
	assert.Equal(t, selfList().Inspect(), "[[...]]")
	assert.Equal(t, selfMap().Inspect(), `{"self": {...}}`)
	assert.Equal(t, mixedCycle().Inspect(), `[{"list": [...]}]`)

	// A container referenced twice without a cycle prints in full
	inner := NewList([]Object{NewInt(1)})
	outer := NewList([]Object{inner, inner})
	assert.Equal(t, outer.Inspect(), "[[1], [1]]")

	// Very deep nesting is truncated rather than overflowing the stack
	deep := nestedLists(100000).Inspect()
	assert.True(t, len(deep) < 1000)
}

func TestCycleInterface(t *testing.T) {
	for _, obj := range []Object{selfList(), selfMap(), mixedCycle()} {
		assert.Nil(t, obj.Interface())
		_, err := ToInterface(obj)
		assert.True(t, errors.Is(err, ErrNestingDepth))
	}

	// Self-reference repeated at each level must fail fast, not explore an
	// exponential number of paths.
	a := NewList(nil)
	a.Append(a)
	a.Append(a)
	_, err := ToInterface(a)
	assert.True(t, errors.Is(err, ErrNestingDepth))

	// An iterator over a cyclic map's values
	_, err = ToInterface(NewMapValueIter(selfMap()))
	assert.True(t, errors.Is(err, ErrNestingDepth))
}

func TestNestingDepthLimit(t *testing.T) {
	v, err := ToInterface(nestedLists(MaxNestingDepth))
	assert.Nil(t, err)
	assert.NotNil(t, v)

	_, err = ToInterface(nestedLists(MaxNestingDepth + 1))
	assert.True(t, errors.Is(err, ErrNestingDepth))

	_, err = nestedLists(MaxNestingDepth + 1).MarshalJSON()
	assert.True(t, errors.Is(err, ErrNestingDepth))

	// A shared container that is not a cycle converts normally
	inner := NewMap(map[string]Object{"x": NewInt(1)})
	v, err = ToInterface(NewList([]Object{inner, inner}))
	assert.Nil(t, err)
	assert.Equal(t, v, []any{map[string]any{"x": int64(1)}, map[string]any{"x": int64(1)}})
}

func TestCycleEquals(t *testing.T) {
	a, b := selfList(), selfList()
	assert.False(t, a.Equals(a))
	assert.False(t, a.Equals(b))

	m := selfMap()
	assert.False(t, m.Equals(m))
	assert.False(t, m.Equals(selfMap()))

	c := mixedCycle()
	assert.False(t, c.Equals(c))
	assert.False(t, c.Equals(mixedCycle()))

	// Exponential fan-out still terminates quickly
	x := NewList(nil)
	x.Append(x)
	x.Append(x)
	assert.False(t, x.Equals(x))

	// Deep but equal structures within the limit are equal
	assert.True(t, nestedLists(MaxNestingDepth).Equals(nestedLists(MaxNestingDepth)))
	assert.False(t, nestedLists(MaxNestingDepth+1).Equals(nestedLists(MaxNestingDepth+1)))

	// Ordinary nested equality is unaffected
	assert.True(t, NewList([]Object{NewMap(map[string]Object{"a": NewList([]Object{NewInt(1)})})}).Equals(
		NewList([]Object{NewMap(map[string]Object{"a": NewList([]Object{NewInt(1)})})})))
}

func TestCycleCompare(t *testing.T) {
	a := selfList()
	_, err := a.Compare(a)
	assert.True(t, errors.Is(err, ErrNestingDepth))
	_, err = a.Compare(selfList())
	assert.True(t, errors.Is(err, ErrNestingDepth))

	// Maps are not comparable, so a mixed cycle fails with a type error
	_, err = mixedCycle().Compare(mixedCycle())
	assert.NotNil(t, err)

	_, err = nestedLists(MaxNestingDepth + 1).Compare(nestedLists(MaxNestingDepth + 1))
	assert.True(t, errors.Is(err, ErrNestingDepth))

	cmp, err := nestedLists(MaxNestingDepth).Compare(nestedLists(MaxNestingDepth))
	assert.Nil(t, err)
	assert.Equal(t, cmp, 0)
}

func TestCycleMarshalJSON(t *testing.T) {
	for _, obj := range []Object{selfList(), selfMap(), mixedCycle()} {
		_, err := json.Marshal(obj)
		assert.True(t, errors.Is(err, ErrNestingDepth))
	}

	// Output matches encoding the underlying Go slices and maps directly,
	// which is how MarshalJSON worked before cycle detection.
	items := []Object{
		NewInt(1),
		NewString("<a>"),
		NewMap(map[string]Object{"b": Nil, "<k>": NewList([]Object{True})}),
	}
	data, err := json.Marshal(NewList(items))
	assert.Nil(t, err)
	want, err := json.Marshal([]any{items[0], items[1], items[2].(*Map).items})
	assert.Nil(t, err)
	assert.Equal(t, string(data), string(want))
}

func TestCycleToGo(t *testing.T) {
	r := DefaultRegistry()
	targets := []reflect.Type{
		reflect.TypeOf([]any{}),
		reflect.TypeOf(map[string]any{}),
		reflect.TypeOf((*any)(nil)).Elem(),
		reflect.TypeOf([1]any{}),
	}
	for _, target := range targets {
		for _, obj := range []Object{selfList(), selfMap(), mixedCycle()} {
			_, err := r.ToGo(obj, target)
			assert.NotNil(t, err)
		}
	}
	_, err := r.ToGo(selfList(), reflect.TypeOf([]any{}))
	assert.True(t, errors.Is(err, ErrNestingDepth))
	// The depth error is not wrapped once per level
	assert.Equal(t, err.Error(), ErrNestingDepth.Error())

	_, err = r.ToGo(mixedCycle(), reflect.TypeOf([]map[string]any{}))
	assert.True(t, errors.Is(err, ErrNestingDepth))
}

func TestCycleFromGo(t *testing.T) {
	r := DefaultRegistry()

	s := []any{nil}
	s[0] = s
	_, err := r.FromGo(s)
	assert.True(t, errors.Is(err, ErrNestingDepth))

	m := map[string]any{}
	m["self"] = m
	_, err = r.FromGo(m)
	assert.True(t, errors.Is(err, ErrNestingDepth))

	// Mixed slice/map cycle with fan-out
	mixed := map[string]any{}
	list := []any{mixed, mixed}
	mixed["list"] = list
	_, err = r.FromGo(list)
	assert.True(t, errors.Is(err, ErrNestingDepth))

	// Pointer cycle
	p := new(any)
	*p = p
	_, err = r.FromGo(p)
	assert.True(t, errors.Is(err, ErrNestingDepth))

	// Deep non-cyclic nesting
	var deep any = 1
	for i := 0; i < MaxNestingDepth+1; i++ {
		deep = []any{deep}
	}
	_, err = r.FromGo(deep)
	assert.True(t, errors.Is(err, ErrNestingDepth))

	// Shared, non-cyclic values still convert
	shared := []any{1}
	obj, err := r.FromGo([]any{shared, shared, [2]any{shared, shared}})
	assert.Nil(t, err)
	assert.Equal(t, obj.Inspect(), "[[1], [1], [[1], [1]]]")
}

// An iterator stored in the map it iterates yields the map, which holds the
// iterator again. Converting it must stop at the cycle.
func TestCycleThroughIterator(t *testing.T) {
	values := NewMap(nil)
	values.Set("v", NewMapValueIter(values))
	entries := NewMap(nil)
	entries.Set("e", NewMapItemIter(entries))
	var self *Iter
	self = NewIter("self", func(ctx context.Context, fn func(key, value Object) bool) {
		fn(NewInt(0), self)
	})
	inList := NewList(nil)
	inMap := NewMap(map[string]Object{"list": inList})
	inList.Append(NewMapValueIter(inMap))

	for _, obj := range []Object{values, entries, self, inList, values.items["v"]} {
		assert.Nil(t, obj.Interface())
		_, err := ToInterface(obj)
		assert.True(t, errors.Is(err, ErrNestingDepth), obj.Inspect())
		for _, target := range []reflect.Type{
			reflect.TypeOf((*any)(nil)).Elem(),
			reflect.TypeOf(map[string]any{}),
			reflect.TypeOf([]any{}),
		} {
			_, err := DefaultRegistry().ToGo(obj, target)
			assert.NotNil(t, err)
		}
	}
	_, err := DefaultRegistry().ToGo(values, reflect.TypeOf(map[string]any{}))
	assert.True(t, errors.Is(err, ErrNestingDepth))

	// Inspect, Equals, and MarshalJSON do not visit an iterator's values
	assert.Equal(t, values.Inspect(), `{"v": iter(map.values)}`)
	assert.Equal(t, inList.Inspect(), "[iter(map.values)]")
	assert.True(t, values.Equals(values))
	assert.False(t, values.Equals(entries))
	assert.True(t, inList.Equals(inList))
	data, err := json.Marshal(values)
	assert.Nil(t, err)
	assert.Equal(t, string(data), `{"v":{}}`)

	// An iterator at the depth limit prints as itself, not as [...]
	deep := NewList([]Object{self})
	for i := 1; i < MaxNestingDepth; i++ {
		deep = NewList([]Object{deep})
	}
	assert.Contains(t, deep.Inspect(), "iter(self)")

	// An iterator over plain values still converts
	plain := NewMap(map[string]Object{"a": NewList([]Object{NewInt(1)})})
	result, err := ToInterface(NewMapValueIter(plain))
	assert.Nil(t, err)
	assert.Equal(t, result, []any{[]any{int64(1)}})
}

func TestInspectConcurrent(t *testing.T) {
	inner := NewMap(map[string]Object{"x": NewInt(1)})
	ls := NewList([]Object{inner, NewInt(2)})
	m := NewMap(map[string]Object{"list": ls})
	ls.Append(m) // also exercise the cycle path concurrently

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				// Concurrent readers must never see each other's traversal
				assert.Equal(t, ls.Inspect(), `[{"x": 1}, 2, {"list": [...]}]`)
				assert.Equal(t, m.Inspect(), `{"list": [{"x": 1}, 2, {...}]}`)
				_ = ls.Equals(ls)
				_, _ = ToInterface(ls)
			}
		}()
	}
	wg.Wait()
}

func TestDefaultRegistryConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	results := make([]*TypeRegistry, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = DefaultRegistry()
		}(i)
	}
	wg.Wait()
	for _, r := range results {
		assert.True(t, r == results[0])
	}
}
