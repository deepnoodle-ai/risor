package op

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestGetInfo(t *testing.T) {
	info := GetInfo(LoadClosure)
	assert.Equal(t, info.Name, "LOAD_CLOSURE")
	assert.Equal(t, info.OperandCount, 2)
	assert.Equal(t, info.Code, LoadClosure)
}
