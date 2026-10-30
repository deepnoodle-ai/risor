package rand

import (
	"context"
	"math"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/errors"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Rand.bytes caps n and honors context cancellation.
func TestBytesLimit(t *testing.T) {
	ctx := context.Background()
	_, err := Bytes(ctx, object.NewInt(math.MaxInt32))
	var ve *errors.ValueError
	assert.ErrorAs(t, err, &ve)
	assert.ErrorContains(t, err, "too large")

	_, err = Bytes(ctx, object.NewInt(object.DefaultMaxListLength+1))
	assert.ErrorContains(t, err, "too large")

	result, err := Bytes(ctx, object.NewInt(16))
	assert.Nil(t, err)
	assert.Len(t, result.(*object.List).Value(), 16)

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = Bytes(canceled, object.NewInt(object.DefaultMaxListLength))
	assert.ErrorIs(t, err, context.Canceled)
}

// Spans wider than math.MaxInt64 must not overflow or panic.
func TestIntSpanOverflow(t *testing.T) {
	ctx := context.Background()
	for range 100 {
		result, err := Int(ctx, object.NewInt(-10), object.NewInt(math.MaxInt64))
		assert.Nil(t, err)
		v := result.(*object.Int).Value()
		assert.True(t, v >= -10 && v < math.MaxInt64)

		result, err = Int(ctx, object.NewInt(math.MinInt64), object.NewInt(math.MaxInt64))
		assert.Nil(t, err)
		assert.True(t, result.(*object.Int).Value() < math.MaxInt64)

		result, err = Randint(ctx, object.NewInt(0), object.NewInt(math.MaxInt64))
		assert.Nil(t, err)
		assert.True(t, result.(*object.Int).Value() >= 0)

		_, err = Randint(ctx, object.NewInt(math.MinInt64), object.NewInt(math.MaxInt64))
		assert.Nil(t, err)

		result, err = Randint(ctx, object.NewInt(-1), object.NewInt(math.MaxInt64))
		assert.Nil(t, err)
		assert.True(t, result.(*object.Int).Value() >= -1)
	}

	// Degenerate ranges still return their only value.
	result, err := Randint(ctx, object.NewInt(math.MaxInt64), object.NewInt(math.MaxInt64))
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.Int).Value(), int64(math.MaxInt64))

	result, err = Int(ctx, object.NewInt(math.MinInt64), object.NewInt(math.MinInt64+1))
	assert.Nil(t, err)
	assert.Equal(t, result.(*object.Int).Value(), int64(math.MinInt64))
}
