package rand

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
)

func TestFloat(t *testing.T) {
	ctx := context.Background()

	// Test that Float returns a value in [0, 1)
	for range 100 {
		result := Float(ctx)
		f, ok := result.(*object.Float)
		assert.True(t, ok)
		assert.True(t, f.Value() >= 0.0)
		assert.True(t, f.Value() < 1.0)
	}
}

func TestFloatErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	result := Float(ctx, object.NewInt(1))
	assert.True(t, object.IsError(result))
}

func TestInt(t *testing.T) {
	ctx := context.Background()

	// Test that Int returns a non-negative value
	for range 100 {
		result := Int(ctx)
		intVal, ok := result.(*object.Int)
		assert.True(t, ok)
		assert.True(t, intVal.Value() >= 0)
	}
}

func TestIntErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	result := Int(ctx, object.NewInt(1))
	assert.True(t, object.IsError(result))
}

func TestIntN(t *testing.T) {
	ctx := context.Background()

	// Test that IntN returns a value in [0, n)
	n := int64(100)
	for range 100 {
		result := IntN(ctx, object.NewInt(n))
		intVal, ok := result.(*object.Int)
		assert.True(t, ok)
		assert.True(t, intVal.Value() >= 0)
		assert.True(t, intVal.Value() < n)
	}
}

func TestIntNErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	result := IntN(ctx)
	assert.True(t, object.IsError(result))

	// Wrong type
	result = IntN(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))
}

func TestNormFloat(t *testing.T) {
	ctx := context.Background()

	// Test that NormFloat returns float values (normal distribution)
	// We can't test exact values, but we can verify it returns floats
	for range 100 {
		result := NormFloat(ctx)
		_, ok := result.(*object.Float)
		assert.True(t, ok)
	}
}

func TestNormFloatErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	result := NormFloat(ctx, object.NewInt(1))
	assert.True(t, object.IsError(result))
}

func TestExpFloat(t *testing.T) {
	ctx := context.Background()

	// Test that ExpFloat returns non-negative floats (exponential distribution)
	for range 100 {
		result := ExpFloat(ctx)
		f, ok := result.(*object.Float)
		assert.True(t, ok)
		assert.True(t, f.Value() >= 0.0)
	}
}

func TestExpFloatErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	result := ExpFloat(ctx, object.NewInt(1))
	assert.True(t, object.IsError(result))
}

func TestShuffle(t *testing.T) {
	ctx := context.Background()

	// Create a list
	original := []object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
		object.NewInt(4),
		object.NewInt(5),
	}
	list := object.NewList(original)

	// Shuffle the list
	result := Shuffle(ctx, list)

	// Verify it returns the same list object
	resultList, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Equal(t, resultList, list)

	// Verify length is preserved
	assert.Len(t, resultList.Value(), 5)

	// Verify all original elements are present
	values := resultList.Value()
	counts := make(map[int64]int)
	for _, v := range values {
		intVal := v.(*object.Int)
		counts[intVal.Value()]++
	}
	assert.Equal(t, counts[1], 1)
	assert.Equal(t, counts[2], 1)
	assert.Equal(t, counts[3], 1)
	assert.Equal(t, counts[4], 1)
	assert.Equal(t, counts[5], 1)
}

func TestShuffleEmpty(t *testing.T) {
	ctx := context.Background()

	// Shuffling empty list should work
	list := object.NewList([]object.Object{})
	result := Shuffle(ctx, list)

	resultList, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, resultList.Value(), 0)
}

func TestShuffleErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	result := Shuffle(ctx)
	assert.True(t, object.IsError(result))

	// Wrong type
	result = Shuffle(ctx, object.NewString("hello"))
	assert.True(t, object.IsError(result))

	result = Shuffle(ctx, object.NewInt(5))
	assert.True(t, object.IsError(result))
}

func TestModule(t *testing.T) {
	m := Module()
	assert.NotNil(t, m)
	assert.Equal(t, m.Name().Value(), "rand")

	// Verify key functions exist
	_, ok := m.GetAttr("float")
	assert.True(t, ok)

	_, ok = m.GetAttr("int")
	assert.True(t, ok)

	_, ok = m.GetAttr("intn")
	assert.True(t, ok)

	_, ok = m.GetAttr("norm_float")
	assert.True(t, ok)

	_, ok = m.GetAttr("exp_float")
	assert.True(t, ok)

	_, ok = m.GetAttr("shuffle")
	assert.True(t, ok)
}
