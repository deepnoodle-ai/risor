package risor

import (
	"context"
	"math"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestMinInt64LiteralEval(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{"-9223372036854775808", int64(math.MinInt64)},
		{"-9223372036854775808 == -9223372036854775807 - 1", true},
		{"[-9223372036854775808][0]", int64(math.MinInt64)},
		{"-0x8000000000000000", int64(math.MinInt64)},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Eval(context.Background(), tt.input)
			assert.Nil(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
}
