package rand

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestRandom(t *testing.T) {
	ctx := context.Background()

	// Test that Random returns a value in [0, 1)
	for range 100 {
		result, err := Random(ctx)
		assert.Nil(t, err)
		f, ok := result.(*object.Float)
		assert.True(t, ok)
		assert.True(t, f.Value() >= 0.0)
		assert.True(t, f.Value() < 1.0)
	}
}

func TestRandomErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	_, err := Random(ctx, object.NewInt(1))
	assert.NotNil(t, err)
}

func TestInt(t *testing.T) {
	ctx := context.Background()

	// Test with no arguments - returns a non-negative value
	for range 100 {
		result, err := Int(ctx)
		assert.Nil(t, err)
		intVal, ok := result.(*object.Int)
		assert.True(t, ok)
		assert.True(t, intVal.Value() >= 0)
	}
}

func TestIntWithMax(t *testing.T) {
	ctx := context.Background()

	// Test with one argument - returns value in [0, n)
	n := int64(100)
	for range 100 {
		result, err := Int(ctx, object.NewInt(n))
		assert.Nil(t, err)
		intVal, ok := result.(*object.Int)
		assert.True(t, ok)
		assert.True(t, intVal.Value() >= 0)
		assert.True(t, intVal.Value() < n)
	}
}

func TestIntWithRange(t *testing.T) {
	ctx := context.Background()

	// Test with two arguments - returns value in [min, max)
	min := int64(10)
	max := int64(20)
	for range 100 {
		result, err := Int(ctx, object.NewInt(min), object.NewInt(max))
		assert.Nil(t, err)
		intVal, ok := result.(*object.Int)
		assert.True(t, ok)
		assert.True(t, intVal.Value() >= min)
		assert.True(t, intVal.Value() < max)
	}
}

func TestIntErrors(t *testing.T) {
	ctx := context.Background()

	// Too many arguments
	_, err := Int(ctx, object.NewInt(1), object.NewInt(2), object.NewInt(3))
	assert.NotNil(t, err)

	// Invalid max (zero or negative)
	_, err = Int(ctx, object.NewInt(0))
	assert.NotNil(t, err)

	_, err = Int(ctx, object.NewInt(-5))
	assert.NotNil(t, err)

	// Invalid range (max <= min)
	_, err = Int(ctx, object.NewInt(10), object.NewInt(5))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Int(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestRandint(t *testing.T) {
	ctx := context.Background()

	// Test that randint returns a value in [a, b] inclusive
	a := int64(5)
	b := int64(10)
	for range 100 {
		result, err := Randint(ctx, object.NewInt(a), object.NewInt(b))
		assert.Nil(t, err)
		intVal, ok := result.(*object.Int)
		assert.True(t, ok)
		assert.True(t, intVal.Value() >= a)
		assert.True(t, intVal.Value() <= b) // inclusive
	}
}

func TestRandintErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	_, err := Randint(ctx)
	assert.NotNil(t, err)

	_, err = Randint(ctx, object.NewInt(1))
	assert.NotNil(t, err)

	// b < a
	_, err = Randint(ctx, object.NewInt(10), object.NewInt(5))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Randint(ctx, object.NewString("a"), object.NewInt(10))
	assert.NotNil(t, err)
}

func TestUniform(t *testing.T) {
	ctx := context.Background()

	// Test that uniform returns a value in [a, b]
	a := 2.5
	b := 7.5
	for range 100 {
		result, err := Uniform(ctx, object.NewFloat(a), object.NewFloat(b))
		assert.Nil(t, err)
		f, ok := result.(*object.Float)
		assert.True(t, ok)
		assert.True(t, f.Value() >= a)
		assert.True(t, f.Value() <= b)
	}
}

func TestUniformErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	_, err := Uniform(ctx)
	assert.NotNil(t, err)

	_, err = Uniform(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Uniform(ctx, object.NewString("a"), object.NewFloat(10))
	assert.NotNil(t, err)
}

func TestNormal(t *testing.T) {
	ctx := context.Background()

	// Test with no arguments (standard normal)
	for range 100 {
		result, err := Normal(ctx)
		assert.Nil(t, err)
		_, ok := result.(*object.Float)
		assert.True(t, ok)
	}
}

func TestNormalWithParams(t *testing.T) {
	ctx := context.Background()

	// Test with mu and sigma
	mu := 10.0
	sigma := 2.0
	for range 100 {
		result, err := Normal(ctx, object.NewFloat(mu), object.NewFloat(sigma))
		assert.Nil(t, err)
		_, ok := result.(*object.Float)
		assert.True(t, ok)
	}
}

func TestNormalErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count (1 argument not allowed)
	_, err := Normal(ctx, object.NewFloat(1.0))
	assert.NotNil(t, err)

	// Negative sigma
	_, err = Normal(ctx, object.NewFloat(0), object.NewFloat(-1))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Normal(ctx, object.NewString("a"), object.NewFloat(1))
	assert.NotNil(t, err)
}

func TestExponential(t *testing.T) {
	ctx := context.Background()

	// Test with no arguments (lambda=1)
	for range 100 {
		result, err := Exponential(ctx)
		assert.Nil(t, err)
		f, ok := result.(*object.Float)
		assert.True(t, ok)
		assert.True(t, f.Value() >= 0.0)
	}
}

func TestExponentialWithLambda(t *testing.T) {
	ctx := context.Background()

	// Test with custom lambda
	lambda := 2.0
	for range 100 {
		result, err := Exponential(ctx, object.NewFloat(lambda))
		assert.Nil(t, err)
		f, ok := result.(*object.Float)
		assert.True(t, ok)
		assert.True(t, f.Value() >= 0.0)
	}
}

func TestExponentialErrors(t *testing.T) {
	ctx := context.Background()

	// Too many arguments
	_, err := Exponential(ctx, object.NewFloat(1), object.NewFloat(2))
	assert.NotNil(t, err)

	// Non-positive lambda
	_, err = Exponential(ctx, object.NewFloat(0))
	assert.NotNil(t, err)

	_, err = Exponential(ctx, object.NewFloat(-1))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Exponential(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestChoice(t *testing.T) {
	ctx := context.Background()

	// Create a list
	items := []object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
	}
	list := object.NewList(items)

	// Test that choice returns an element from the list
	for range 100 {
		result, err := Choice(ctx, list)
		assert.Nil(t, err)
		intVal, ok := result.(*object.Int)
		assert.True(t, ok)
		v := intVal.Value()
		assert.True(t, v == 1 || v == 2 || v == 3)
	}
}

func TestChoiceErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	_, err := Choice(ctx)
	assert.NotNil(t, err)

	// Empty list
	_, err = Choice(ctx, object.NewList([]object.Object{}))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Choice(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestSample(t *testing.T) {
	ctx := context.Background()

	// Create a list
	items := []object.Object{
		object.NewInt(1),
		object.NewInt(2),
		object.NewInt(3),
		object.NewInt(4),
		object.NewInt(5),
	}
	list := object.NewList(items)

	// Sample 3 elements
	result, err := Sample(ctx, list, object.NewInt(3))
	assert.Nil(t, err)

	resultList, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, resultList.Value(), 3)

	// Verify all elements are unique and from original list
	seen := make(map[int64]bool)
	for _, v := range resultList.Value() {
		intVal := v.(*object.Int)
		val := intVal.Value()
		assert.True(t, val >= 1 && val <= 5)
		assert.False(t, seen[val]) // should not have duplicates
		seen[val] = true
	}
}

func TestSampleZero(t *testing.T) {
	ctx := context.Background()

	list := object.NewList([]object.Object{object.NewInt(1), object.NewInt(2)})
	result, err := Sample(ctx, list, object.NewInt(0))
	assert.Nil(t, err)

	resultList, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, resultList.Value(), 0)
}

func TestSampleErrors(t *testing.T) {
	ctx := context.Background()

	list := object.NewList([]object.Object{object.NewInt(1), object.NewInt(2)})

	// Wrong argument count
	_, err := Sample(ctx)
	assert.NotNil(t, err)

	_, err = Sample(ctx, list)
	assert.NotNil(t, err)

	// k > n
	_, err = Sample(ctx, list, object.NewInt(5))
	assert.NotNil(t, err)

	// Negative k
	_, err = Sample(ctx, list, object.NewInt(-1))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Sample(ctx, object.NewString("hello"), object.NewInt(1))
	assert.NotNil(t, err)
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
	result, err := Shuffle(ctx, list)
	assert.Nil(t, err)

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
	result, err := Shuffle(ctx, list)
	assert.Nil(t, err)

	resultList, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, resultList.Value(), 0)
}

func TestShuffleErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	_, err := Shuffle(ctx)
	assert.NotNil(t, err)

	// Wrong type
	_, err = Shuffle(ctx, object.NewString("hello"))
	assert.NotNil(t, err)

	_, err = Shuffle(ctx, object.NewInt(5))
	assert.NotNil(t, err)
}

func TestBytes(t *testing.T) {
	ctx := context.Background()

	result, err := Bytes(ctx, object.NewInt(10))
	assert.Nil(t, err)

	resultList, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, resultList.Value(), 10)

	// Verify all values are bytes (0-255)
	for _, v := range resultList.Value() {
		intVal, ok := v.(*object.Int)
		assert.True(t, ok)
		assert.True(t, intVal.Value() >= 0)
		assert.True(t, intVal.Value() <= 255)
	}
}

func TestBytesZero(t *testing.T) {
	ctx := context.Background()

	result, err := Bytes(ctx, object.NewInt(0))
	assert.Nil(t, err)

	resultList, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Len(t, resultList.Value(), 0)
}

func TestBytesErrors(t *testing.T) {
	ctx := context.Background()

	// Wrong argument count
	_, err := Bytes(ctx)
	assert.NotNil(t, err)

	// Negative n
	_, err = Bytes(ctx, object.NewInt(-1))
	assert.NotNil(t, err)

	// Wrong type
	_, err = Bytes(ctx, object.NewString("hello"))
	assert.NotNil(t, err)
}

func TestModule(t *testing.T) {
	m := Module()
	assert.NotNil(t, m)
	assert.Equal(t, m.Name().Value(), "rand")

	// Verify all functions exist
	functions := []string{
		"random",
		"int",
		"randint",
		"uniform",
		"normal",
		"exponential",
		"choice",
		"sample",
		"shuffle",
		"bytes",
	}
	for _, name := range functions {
		_, ok := m.GetAttr(name)
		assert.True(t, ok)
	}
}
