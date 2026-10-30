package object

import (
	"context"
	"math"
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

// C-09: repeat must reject overflowing and oversized counts instead of
// panicking or attempting a huge allocation.
func TestAuditStringRepeatLimits(t *testing.T) {
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

func TestAuditBytesRepeatLimits(t *testing.T) {
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

func TestAuditStringAmplificationLimits(t *testing.T) {
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

func TestAuditBytesAmplificationLimits(t *testing.T) {
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

// C-10: length must not overflow and enumeration must not wrap around.
func TestAuditRangeLengthOverflow(t *testing.T) {
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

func TestAuditRangeEnumerateNoWraparound(t *testing.T) {
	r := NewRange(9223372036854775800, math.MaxInt64, 10)
	assert.Equal(t, collectRange(r, 10), []int64{9223372036854775800})

	r = NewRange(-9223372036854775800, math.MinInt64, -10)
	assert.Equal(t, collectRange(r, 10), []int64{-9223372036854775800})

	r = NewRange(math.MaxInt64-2, math.MaxInt64, 1)
	assert.Equal(t, collectRange(r, 10), []int64{math.MaxInt64 - 2, math.MaxInt64 - 1})
}

func TestAuditRangeMapLimits(t *testing.T) {
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

func TestAuditCollectLimits(t *testing.T) {
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

func TestAuditRangeInterfaceLimit(t *testing.T) {
	assert.Nil(t, NewRange(0, 1<<60, 1).Interface())
	assert.Equal(t, NewRange(0, 3, 1).Interface(), []int64{0, 1, 2})
}

func TestExpandRanges(t *testing.T) {
	ctx := WithLimits(context.Background(), Limits{MaxListLength: 10})

	// Scalars and containers without ranges are returned unchanged
	s := NewString("a")
	got, err := ExpandRanges(ctx, s)
	assert.Nil(t, err)
	assert.True(t, got == s)
	plain := NewList([]Object{NewInt(1), NewMap(map[string]Object{"a": NewInt(2)})})
	got, err = ExpandRanges(ctx, plain)
	assert.Nil(t, err)
	assert.True(t, got == plain)

	// Nested ranges are replaced by lists, and the input is not modified
	r := NewRange(0, 3, 1)
	nested := NewList([]Object{NewInt(1), NewMap(map[string]Object{"r": r})})
	got, err = ExpandRanges(ctx, nested)
	assert.Nil(t, err)
	assert.Equal(t, got.Interface(), []any{int64(1), map[string]any{"r": []any{int64(0), int64(1), int64(2)}}})
	assert.True(t, nested.items[1].(*Map).items["r"] == r)

	// A range over the limit is a value error, also when nested
	_, err = ExpandRanges(ctx, NewRange(0, 11, 1))
	assertValueError(t, err, "range too large (exceeds limit of 10 elements)")
	_, err = ExpandRanges(ctx, NewList([]Object{NewRange(0, 1<<60, 1)}))
	assertValueError(t, err, "range too large")

	// A list that contains itself does not recurse forever
	cyclic := NewList([]Object{r})
	cyclic.items = append(cyclic.items, cyclic)
	got, err = ExpandRanges(ctx, cyclic)
	assert.Nil(t, err)
	assert.Equal(t, got.(*List).items[0].Inspect(), "[0, 1, 2]")
}

// H-21: Range.Enumerate stops promptly when the context is canceled.
func TestAuditRangeEnumerateContext(t *testing.T) {
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

// M-14: sorting stops after an error or context cancellation.
func TestAuditSortContext(t *testing.T) {
	items := make([]Object, 5000)
	for i := range items {
		items[i] = NewInt(int64(len(items) - i))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := SortContext(ctx, items)
	assert.NotNil(t, err)
	assert.ErrorIs(t, err, context.Canceled)

	assert.Nil(t, Sort(items))
	assert.Equal(t, items[0].(*Int).Value(), int64(1))

	mixed := []Object{NewInt(1), NewList(nil), NewInt(2)}
	err = Sort(mixed)
	assert.NotNil(t, err)
	assert.ErrorContains(t, err, "compare")
}
