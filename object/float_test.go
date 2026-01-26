package object

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestFloatBasics(t *testing.T) {
	value := NewFloat(-2)
	assert.Equal(t, value.Type(), FLOAT)
	assert.Equal(t, value.Value(), float64(-2))
	assert.Equal(t, value.String(), "-2")
	assert.Equal(t, value.Inspect(), "-2")
	assert.Equal(t, value.Interface(), float64(-2))
}
