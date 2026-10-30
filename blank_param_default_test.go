package risor

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// A default on a "_" parameter makes that parameter optional, and it goes to
// the last "_" when there are several.
func TestBlankParamDefaultRuns(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{`function f(a, _ = 5) { return a }; f(1)`, int64(1)},
		{`function f(a, _ = 5) { return a }; f(1, 2)`, int64(1)},
		{`function f(_, _ = 5) { return 42 }; f(1)`, int64(42)},
		{`let g = (x, _ = 0) => x; g(7)`, int64(7)},
		{`let g = (_, x, _ = 0) => x; g(1, 8)`, int64(8)},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Eval(context.Background(), tt.input)
			assert.Nil(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
	// The first "_" stays required
	_, err := Eval(context.Background(), `function f(_, _ = 5) { return 42 }; f()`)
	assert.NotNil(t, err)
}
