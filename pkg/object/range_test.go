package object

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestRangeBasics(t *testing.T) {
	r := NewRange(0, 5, 1)
	assert.Equal(t, r.Type(), RANGE)
	assert.Equal(t, r.Inspect(), "range(5)")
	assert.True(t, r.IsTruthy())

	r2 := NewRange(1, 5, 1)
	assert.Equal(t, r2.Inspect(), "range(1, 5)")

	r3 := NewRange(0, 10, 2)
	assert.Equal(t, r3.Inspect(), "range(0, 10, 2)")
}

func TestRangeEnumerate(t *testing.T) {
	ctx := context.Background()

	// range(5) -> 0, 1, 2, 3, 4
	r := NewRange(0, 5, 1)
	var values []int64
	r.Enumerate(ctx, func(key, value Object) bool {
		values = append(values, value.(*Int).Value())
		return true
	})
	assert.Equal(t, values, []int64{0, 1, 2, 3, 4})

	// range(1, 5) -> 1, 2, 3, 4
	r2 := NewRange(1, 5, 1)
	values = nil
	r2.Enumerate(ctx, func(key, value Object) bool {
		values = append(values, value.(*Int).Value())
		return true
	})
	assert.Equal(t, values, []int64{1, 2, 3, 4})

	// range(0, 10, 2) -> 0, 2, 4, 6, 8
	r3 := NewRange(0, 10, 2)
	values = nil
	r3.Enumerate(ctx, func(key, value Object) bool {
		values = append(values, value.(*Int).Value())
		return true
	})
	assert.Equal(t, values, []int64{0, 2, 4, 6, 8})

	// range(5, 0, -1) -> 5, 4, 3, 2, 1
	r4 := NewRange(5, 0, -1)
	values = nil
	r4.Enumerate(ctx, func(key, value Object) bool {
		values = append(values, value.(*Int).Value())
		return true
	})
	assert.Equal(t, values, []int64{5, 4, 3, 2, 1})
}

func TestRangeEnumerateEarlyStop(t *testing.T) {
	ctx := context.Background()
	r := NewRange(0, 10, 1)
	var values []int64
	r.Enumerate(ctx, func(key, value Object) bool {
		values = append(values, value.(*Int).Value())
		return len(values) < 3 // stop after 3 elements
	})
	assert.Equal(t, values, []int64{0, 1, 2})
}

func TestRangeEquals(t *testing.T) {
	r1 := NewRange(0, 5, 1)
	r2 := NewRange(0, 5, 1)
	r3 := NewRange(0, 10, 1)

	assert.True(t, r1.Equals(r2))
	assert.False(t, r1.Equals(r3))

	// Empty ranges are equal
	empty1 := NewRange(0, 0, 1)
	empty2 := NewRange(5, 0, 1)
	assert.True(t, empty1.Equals(empty2))
}

func TestRangeAttributes(t *testing.T) {
	r := NewRange(1, 10, 2)

	start, ok := r.GetAttr("start")
	assert.True(t, ok)
	assert.Equal(t, start.(*Int).Value(), int64(1))

	stop, ok := r.GetAttr("stop")
	assert.True(t, ok)
	assert.Equal(t, stop.(*Int).Value(), int64(10))

	step, ok := r.GetAttr("step")
	assert.True(t, ok)
	assert.Equal(t, step.(*Int).Value(), int64(2))
}

func TestRangeTruthiness(t *testing.T) {
	assert.True(t, NewRange(0, 5, 1).IsTruthy())
	assert.False(t, NewRange(0, 0, 1).IsTruthy())
	assert.False(t, NewRange(5, 0, 1).IsTruthy())
}

func TestRangeMap(t *testing.T) {
	ctx := context.Background()
	r := NewRange(0, 5, 1)

	// Square each value
	double := NewBuiltin("double", func(ctx context.Context, args ...Object) (Object, error) {
		v := args[0].(*Int).Value()
		return NewInt(v * v), nil
	})

	result, err := r.Map(ctx, double)
	assert.Nil(t, err)
	list := result.(*List)
	assert.Len(t, list.Value(), 5)
	expected := []int64{0, 1, 4, 9, 16}
	for i, item := range list.Value() {
		assert.Equal(t, item.(*Int).Value(), expected[i])
	}
}

func TestRangeFilter(t *testing.T) {
	ctx := context.Background()
	r := NewRange(0, 5, 1)

	// Keep values > 2
	gt2 := NewBuiltin("gt2", func(ctx context.Context, args ...Object) (Object, error) {
		return NewBool(args[0].(*Int).Value() > 2), nil
	})

	result, err := r.Filter(ctx, gt2)
	assert.Nil(t, err)
	list := result.(*List)
	assert.Len(t, list.Value(), 2)
	assert.Equal(t, list.Value()[0].(*Int).Value(), int64(3))
	assert.Equal(t, list.Value()[1].(*Int).Value(), int64(4))
}

func TestRangeEach(t *testing.T) {
	ctx := context.Background()
	r := NewRange(0, 3, 1)

	var collected []int64
	collector := NewBuiltin("collect", func(ctx context.Context, args ...Object) (Object, error) {
		collected = append(collected, args[0].(*Int).Value())
		return Nil, nil
	})

	result, err := r.Each(ctx, collector)
	assert.Nil(t, err)
	assert.Equal(t, result, Nil)
	assert.Equal(t, collected, []int64{0, 1, 2})
}

func TestRangeMethodErrors(t *testing.T) {
	ctx := context.Background()
	r := NewRange(0, 5, 1)

	// Non-callable argument
	_, err := r.Map(ctx, NewInt(42))
	assert.NotNil(t, err)

	_, err = r.Filter(ctx, NewInt(42))
	assert.NotNil(t, err)

	_, err = r.Each(ctx, NewInt(42))
	assert.NotNil(t, err)
}
