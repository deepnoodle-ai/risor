package object

import (
	"context"
	"math"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

func assertValueError(t *testing.T, err error, substr string) {
	t.Helper()
	var ve *errors.ValueError
	assert.ErrorAs(t, err, &ve)
	assert.ErrorContains(t, err, substr)
}

// Repeat must reject overflowing and oversized counts instead of
// panicking or attempting a huge allocation.
func TestStringRepeatLimits(t *testing.T) {
	s := NewString("ab")
	_, err := s.Repeat(context.Background(), NewInt(1<<62))
	assertValueError(t, err, "too large")

	_, err = NewString("a").Repeat(context.Background(), NewInt(1<<32))
	assertValueError(t, err, "too large")

	_, err = NewString("a").Repeat(context.Background(), NewInt(DefaultMaxBytesSize+1))
	assertValueError(t, err, "too large")

	_, err = s.Repeat(context.Background(), NewInt(-1))
	assertValueError(t, err, "negative")

	result, err := s.Repeat(context.Background(), NewInt(3))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "ababab")

	result, err = NewString("").Repeat(context.Background(), NewInt(math.MaxInt64))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "")
}

func TestBytesRepeatLimits(t *testing.T) {
	b := NewBytes([]byte("ab"))
	_, err := b.Repeat(context.Background(), NewInt(1<<62))
	assertValueError(t, err, "too large")

	_, err = NewBytes([]byte("a")).Repeat(context.Background(), NewInt(1<<32))
	assertValueError(t, err, "too large")

	_, err = b.Repeat(context.Background(), NewInt(-1))
	assertValueError(t, err, "negative")

	result, err := b.Repeat(context.Background(), NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Bytes).Value(), []byte("abab"))
}

func TestStringAmplificationLimits(t *testing.T) {
	input := NewString(strings.Repeat("a", 1<<20))
	big := NewString(strings.Repeat("x", 100))

	// 1 MiB input with each byte replaced by 100 bytes -> 100 MiB.
	_, err := input.ReplaceAll(context.Background(), NewString("a"), big)
	assertValueError(t, err, "too large")

	// Empty old string matches between every rune.
	_, err = input.ReplaceAll(context.Background(), NewString(""), big)
	assertValueError(t, err, "too large")

	result, err := NewString("aXa").ReplaceAll(context.Background(), NewString("X"), NewString("YY"))
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "aYYa")

	// A 1 MiB separator joining 100 items -> ~99 MiB.
	items := make([]Object, 100)
	for i := range items {
		items[i] = NewString("")
	}
	_, err = NewString(strings.Repeat("-", 1<<20)).Join(context.Background(), NewList(items))
	assertValueError(t, err, "too large")

	// Concatenation past the limit.
	half := NewString(strings.Repeat("a", DefaultMaxBytesSize/2+1))
	_, err = BinaryOp(context.Background(), op.Add, half, half)
	assertValueError(t, err, "too large")
}

func TestBytesAmplificationLimits(t *testing.T) {
	input := NewBytes([]byte(strings.Repeat("a", 1<<20)))
	big := NewBytes([]byte(strings.Repeat("x", 100)))

	_, err := input.ReplaceAll(context.Background(), NewBytes([]byte("a")), big)
	assertValueError(t, err, "too large")

	_, err = input.Replace(context.Background(), NewBytes([]byte("a")), big, NewInt(-1))
	assertValueError(t, err, "too large")

	// A bounded count keeps the result small.
	result, err := input.Replace(context.Background(), NewBytes([]byte("a")), big, NewInt(2))
	assert.Nil(t, err)
	assert.Equal(t, len(result.(*Bytes).Value()), 1<<20+2*99)

	half := NewBytes([]byte(strings.Repeat("a", DefaultMaxBytesSize/2+1)))
	_, err = BinaryOp(context.Background(), op.Add, half, half)
	assertValueError(t, err, "too large")
}

func collectRange(r *Range, max int) []int64 {
	var values []int64
	r.Enumerate(context.Background(), func(key, value Object) bool {
		values = append(values, value.(*Int).Value())
		return len(values) < max
	})
	return values
}

// Length must not overflow and enumeration must not wrap around.
func TestRangeLengthOverflow(t *testing.T) {
	full := NewRange(math.MinInt64, math.MaxInt64, 1)
	assert.Equal(t, full.length(), uint64(math.MaxUint64))
	assert.True(t, full.IsTruthy())

	rev := NewRange(math.MaxInt64, math.MinInt64, -1)
	assert.Equal(t, rev.length(), uint64(math.MaxUint64))

	// step == MinInt64
	minStep := NewRange(math.MaxInt64, math.MinInt64, math.MinInt64)
	assert.Equal(t, minStep.length(), uint64(2))
	assert.Equal(t, collectRange(minStep, 10), []int64{math.MaxInt64, -1})

	big := NewRange(0, math.MaxInt64, math.MaxInt64)
	assert.Equal(t, big.length(), uint64(1))

	assert.Equal(t, NewRange(0, 10, 3).length(), uint64(4))
	assert.Equal(t, NewRange(10, 0, -3).length(), uint64(4))
	assert.Equal(t, NewRange(5, 5, 1).length(), uint64(0))
	assert.Equal(t, NewRange(5, 6, -1).length(), uint64(0))
}

func TestRangeEnumerateNoWraparound(t *testing.T) {
	r := NewRange(9223372036854775800, math.MaxInt64, 10)
	assert.Equal(t, collectRange(r, 10), []int64{9223372036854775800})

	r = NewRange(-9223372036854775800, math.MinInt64, -10)
	assert.Equal(t, collectRange(r, 10), []int64{-9223372036854775800})

	r = NewRange(math.MaxInt64-2, math.MaxInt64, 1)
	assert.Equal(t, collectRange(r, 10), []int64{math.MaxInt64 - 2, math.MaxInt64 - 1})
}

func TestRangeMapLimits(t *testing.T) {
	identity := NewBuiltin("identity", func(ctx context.Context, args ...Object) (Object, error) {
		return args[0], nil
	})
	ctx := context.Background()

	// Previously panicked in makeslice (negative length).
	_, err := NewRange(math.MinInt64, math.MaxInt64, 1).Map(ctx, identity)
	assertValueError(t, err, "list too large")

	_, err = NewRange(0, 1<<60, 1).Map(ctx, identity)
	assertValueError(t, err, "list too large")

	result, err := NewRange(0, 3, 1).Map(ctx, identity)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), "[0, 1, 2]")
}

func TestCollectLimits(t *testing.T) {
	ctx := context.Background()
	_, err := CollectValues(ctx, NewRange(0, 1<<60, 1))
	assertValueError(t, err, "list too large")

	_, err = CollectKeys(ctx, NewRange(0, 1<<60, 1))
	assertValueError(t, err, "list too large")

	_, err = CollectValues(ctx, NewRange(0, DefaultMaxListLength+1, 1))
	assertValueError(t, err, "list too large")

	values, err := CollectValues(ctx, NewRange(0, 3, 1))
	assert.Nil(t, err)
	assert.Len(t, values, 3)

	keys, err := CollectKeys(ctx, NewList([]Object{NewString("a"), NewString("b")}))
	assert.Nil(t, err)
	assert.Equal(t, NewList(keys).Inspect(), "[0, 1]")

	assert.Nil(t, CheckListLength(context.Background(), DefaultMaxListLength))
	assertValueError(t, CheckListLength(context.Background(), DefaultMaxListLength+1), "list too large")
}

func TestRangeInterfaceLimit(t *testing.T) {
	assert.Nil(t, NewRange(0, 1<<60, 1).Interface())
	assert.Equal(t, NewRange(0, 3, 1).Interface(), []int64{0, 1, 2})
}

func repeatList(item Object, n int) *List {
	items := make([]Object, n)
	for i := range items {
		items[i] = item
	}
	return NewList(items)
}

// One conversion counts every element it produces, including those of a
// container that is referenced more than once.
func TestToInterfaceBudget(t *testing.T) {
	ctx := WithLimits(context.Background(), Limits{MaxListLength: 10})
	inner := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})

	// 2 + 2*3 = 8 elements
	got, err := ToInterface(ctx, repeatList(inner, 2))
	assert.Nil(t, err)
	assert.Equal(t, got, []any{[]any{int64(1), int64(2), int64(3)}, []any{int64(1), int64(2), int64(3)}})

	// 3 + 3*3 = 12 elements
	_, err = ToInterface(ctx, repeatList(inner, 3))
	assertValueError(t, err, "value too large to convert (exceeds limit of 10 elements)")

	// Ranges count their elements: 1 + 6 = 7, then 2 + 2*6 = 14
	r := NewRange(0, 6, 1)
	got, err = ToInterface(ctx, NewList([]Object{r}))
	assert.Nil(t, err)
	assert.Equal(t, got, []any{[]int64{0, 1, 2, 3, 4, 5}})
	_, err = ToInterface(ctx, repeatList(r, 2))
	assertValueError(t, err, "value too large to convert")
	_, err = ToInterface(ctx, NewRange(0, 1<<60, 1))
	assertValueError(t, err, "value too large to convert")

	// Map entries and iterator items count too: 1 + 5*2 = 11
	m := NewMap(map[string]Object{"a": inner, "b": NewInt(1)})
	_, err = ToInterface(ctx, NewMap(map[string]Object{"x": m, "y": NewInt(1)}))
	assert.Nil(t, err)
	_, err = ToInterface(ctx, NewMapValueIter(NewMap(map[string]Object{
		"a": inner, "b": inner, "c": inner,
	})))
	assertValueError(t, err, "value too large to convert")
}

// Under the default limits, a list that references one list many times is
// rejected once the budget is spent, instead of building billions of
// elements. Interface returns nil in that case.
func TestToInterfaceBudgetDefault(t *testing.T) {
	inner := repeatList(NewInt(1), 1<<12)
	outer := repeatList(inner, 1<<20) // 1<<32 elements in total

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := ToInterface(context.Background(), outer)
	runtime.ReadMemStats(&after)
	assertValueError(t, err, "value too large to convert")
	// The budget bounds the work at about DefaultMaxListLength elements
	// (64 MiB of interface slots), far below the 64 GiB without it.
	assert.True(t, after.TotalAlloc-before.TotalAlloc < 256<<20)

	assert.Nil(t, outer.Interface())
}

func TestToGoBudget(t *testing.T) {
	ctx := WithLimits(context.Background(), Limits{MaxListLength: 10})
	reg := DefaultRegistry()
	inner := NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	sliceType := reflect.TypeOf([][]int64{})

	got, err := reg.ToGo(ctx, repeatList(inner, 2), sliceType)
	assert.Nil(t, err)
	assert.Equal(t, got, [][]int64{{1, 2, 3}, {1, 2, 3}})
	_, err = reg.ToGo(ctx, repeatList(inner, 3), sliceType)
	assertValueError(t, err, "value too large to convert (exceeds limit of 10 elements)")

	// An any target converts as ToInterface does
	_, err = reg.ToGo(ctx, repeatList(inner, 3), reflect.TypeOf((*any)(nil)).Elem())
	assertValueError(t, err, "value too large to convert")

	// A range converts to a slice or array like a list of its values
	got, err = reg.ToGo(ctx, NewRange(0, 4, 1), reflect.TypeOf([]int{}))
	assert.Nil(t, err)
	assert.Equal(t, got, []int{0, 1, 2, 3})
	got, err = reg.ToGo(ctx, NewRange(5, 0, -1), reflect.TypeOf([2]int64{}))
	assert.Nil(t, err)
	assert.Equal(t, got, [2]int64{5, 4})
	got, err = reg.ToGo(ctx, NewRange(0, 2, 1), reflect.TypeOf(&[]int64{}))
	assert.Nil(t, err)
	assert.Equal(t, *got.(*[]int64), []int64{0, 1})
	_, err = reg.ToGo(ctx, repeatList(NewRange(0, 6, 1), 2), sliceType)
	assertValueError(t, err, "value too large to convert")
	_, err = reg.ToGo(ctx, NewRange(0, 1<<60, 1), reflect.TypeOf([]int64{}))
	assertValueError(t, err, "value too large to convert")

	// An array charges only the elements it holds
	_, err = reg.ToGo(ctx, NewRange(0, 1<<60, 1), reflect.TypeOf([3]int64{}))
	assert.Nil(t, err)

	// Map entries count
	mapType := reflect.TypeOf(map[string][]int64{})
	_, err = reg.ToGo(ctx, NewMap(map[string]Object{"a": inner, "b": inner}), mapType)
	assert.Nil(t, err)
	_, err = reg.ToGo(ctx, NewMap(map[string]Object{"a": inner, "b": inner, "c": inner}), mapType)
	assertValueError(t, err, "value too large to convert")
}

// ToInterfaces shares one budget across its values and converts each value
// at the same nesting depth as ToInterface.
func TestToInterfaces(t *testing.T) {
	ctx := WithLimits(context.Background(), Limits{MaxListLength: 10})
	inner := NewList([]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4)})

	got, err := ToInterfaces(ctx, []Object{inner, inner, NewInt(5)})
	assert.Nil(t, err)
	assert.Equal(t, got, []any{[]any{int64(1), int64(2), int64(3), int64(4)}, []any{int64(1), int64(2), int64(3), int64(4)}, int64(5)})
	_, err = ToInterfaces(ctx, []Object{inner, inner, inner})
	assertValueError(t, err, "value too large to convert (exceeds limit of 10 elements)")

	_, err = ToInterfaces(context.Background(), []Object{nestedLists(MaxNestingDepth)})
	assert.Nil(t, err)
	_, err = ToInterfaces(context.Background(), []Object{nestedLists(MaxNestingDepth + 1)})
	assert.ErrorIs(t, err, ErrNestingDepth)
}

// Split allocates for the parts it finds, not for the list length limit.
func TestStringSplitAllocation(t *testing.T) {
	ctx := context.Background()
	s := NewString(strings.Repeat("a", 1<<22))
	sep := NewString(",")
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	result, err := s.Split(ctx, sep)
	runtime.ReadMemStats(&after)
	assert.Nil(t, err)
	assert.Equal(t, result.(*List).Size(), 1)
	// Before, SplitN allocated 4M slots (64 MiB) for this one-part result
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(1<<20))
}

// Conversion stops at the nesting limit, also when ranges are converted
// inside the walk, instead of recursing into a deeply nested value.
func TestToInterfaceRangeDepth(t *testing.T) {
	ctx := context.Background()
	nest := func(n int) Object {
		var obj Object = NewRange(0, 2, 1)
		for i := 0; i < n; i++ {
			obj = NewList([]Object{obj})
		}
		return obj
	}
	_, err := ToInterface(ctx, nest(MaxNestingDepth-1))
	assert.Nil(t, err)
	_, err = ToInterface(ctx, nest(MaxNestingDepth+1))
	assert.ErrorIs(t, err, ErrNestingDepth)

	var deep Object = NewInt(1)
	for i := 0; i < 1_000_000; i++ {
		deep = NewList([]Object{deep})
	}
	_, err = ToInterface(ctx, deep)
	assert.ErrorIs(t, err, ErrNestingDepth)
	_, err = DefaultRegistry().ToGo(ctx, deep, reflect.TypeOf([]any{}))
	assert.ErrorIs(t, err, ErrNestingDepth)
}

// Ranges yielded by an iterator count toward the conversion budget.
func TestToInterfaceIteratorRanges(t *testing.T) {
	ctx := WithLimits(context.Background(), Limits{MaxListLength: 10})

	m := NewMap(map[string]Object{"a": NewRange(0, 3, 1)})
	got, err := ToInterface(ctx, NewMapValueIter(m))
	assert.Nil(t, err)
	assert.Equal(t, got, []any{[]int64{0, 1, 2}})

	m = NewMap(map[string]Object{"a": NewRange(0, 11, 1)})
	_, err = ToInterface(ctx, NewMapValueIter(m))
	assertValueError(t, err, "too large")
	_, err = ToInterface(ctx, NewList([]Object{NewMapValueIter(m)}))
	assertValueError(t, err, "too large")

	// An iterator stored in the map it iterates stops at the cycle
	self := NewMap(map[string]Object{})
	self.items["v"] = NewMapValueIter(self)
	_, err = ToInterface(ctx, self)
	assert.ErrorIs(t, err, ErrNestingDepth)
}

// Range.Enumerate stops promptly when the context is canceled.
func TestRangeEnumerateContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	count := 0
	NewRange(1, 1<<60, 1).Enumerate(ctx, func(key, value Object) bool {
		count++
		return true
	})
	assert.LessOrEqual(t, count, ctxCheckInterval)

	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := NewRange(0, 1<<60, 1).Each(ctx, NewBuiltin("noop",
		func(ctx context.Context, args ...Object) (Object, error) { return Nil, nil }))
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
}

// Sorting stops after an error or context cancellation.
func TestSortContext(t *testing.T) {
	items := make([]Object, 5000)
	for i := range items {
		items[i] = NewInt(int64(len(items) - i))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := SortContext(ctx, items)
	assert.NotNil(t, err)
	assert.ErrorIs(t, err, context.Canceled)

	// A sort too short to reach a periodic check still reports cancellation
	err = SortContext(ctx, []Object{NewInt(2), NewInt(1)})
	assert.NotNil(t, err)
	assert.ErrorIs(t, err, context.Canceled)

	assert.Nil(t, Sort(items))
	assert.Equal(t, items[0].(*Int).Value(), int64(1))

	mixed := []Object{NewInt(1), NewList(nil), NewInt(2)}
	err = Sort(mixed)
	assert.NotNil(t, err)
	assert.ErrorContains(t, err, "compare")
}

// ToInterface converts the value in a Cell with the limits in ctx, not with
// the default limits of Cell.Interface.
func TestToInterfaceCell(t *testing.T) {
	ctx := WithLimits(context.Background(), Limits{MaxListLength: 2})
	var value Object = NewList([]Object{NewInt(1), NewInt(2), NewInt(3)})
	_, err := ToInterface(ctx, NewCell(&value))
	assertValueError(t, err, "value too large to convert (exceeds limit of 2 elements)")

	value = NewList([]Object{NewInt(1)})
	got, err := ToInterface(ctx, NewCell(&value))
	assert.Nil(t, err)
	assert.Equal(t, got, []any{int64(1)})

	got, err = ToInterface(ctx, NewCell(nil))
	assert.Nil(t, err)
	assert.Nil(t, got)

	// A cell that holds itself, directly or through a list, is a cycle
	var self Object
	selfCell := NewCell(&self)
	self = selfCell
	_, err = ToInterface(ctx, selfCell)
	assert.ErrorIs(t, err, ErrNestingDepth)

	var held Object
	holder := NewList([]Object{NewCell(&held)})
	held = holder
	_, err = ToInterface(ctx, holder)
	assert.ErrorIs(t, err, ErrNestingDepth)
}
