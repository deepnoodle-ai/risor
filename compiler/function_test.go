package compiler

import (
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestFunctionRequiredArgsCount(t *testing.T) {
	tests := []struct {
		name     string
		params   []string
		defaults []any
		expected int
	}{
		{
			name:     "no defaults",
			params:   []string{"a", "b", "c"},
			defaults: []any{},
			expected: 3,
		},
		{
			name:     "all defaults",
			params:   []string{"a", "b"},
			defaults: []any{1, 2},
			expected: 0,
		},
		{
			name:     "partial defaults with nil entries",
			params:   []string{"a", "b", "c"},
			defaults: []any{nil, 10, nil}, // a=required, b=10, c=required
			expected: 2,
		},
		{
			name:     "only last param has default",
			params:   []string{"a", "b", "c"},
			defaults: []any{nil, nil, "default"},
			expected: 2,
		},
		{
			name:     "no params",
			params:   []string{},
			defaults: []any{},
			expected: 0,
		},
		{
			name:     "defaults shorter than params",
			params:   []string{"a", "b", "c"},
			defaults: []any{1}, // only first param has default
			expected: 2,
		},
		{
			name:     "defaults longer than params",
			params:   []string{"a"},
			defaults: []any{1, 2, 3}, // extra defaults ignored, shouldn't go negative
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := NewFunction(FunctionOpts{
				Parameters: tt.params,
				Defaults:   tt.defaults,
			})
			assert.Equal(t, fn.RequiredArgsCount(), tt.expected)
		})
	}
}

func TestFunctionStringWithShortDefaults(t *testing.T) {
	// This test verifies that String() doesn't panic when defaults array
	// is shorter than parameters array
	fn := NewFunction(FunctionOpts{
		Name:       "test",
		Parameters: []string{"a", "b", "c"},
		Defaults:   []any{1}, // Only one default, but three params
	})

	// Should not panic
	str := fn.String()

	// Verify the output contains expected parts
	assert.True(t, strings.Contains(str, "func test"))
	assert.True(t, strings.Contains(str, "a=1"))
	assert.True(t, strings.Contains(str, ", b,"))
}

func TestFunctionStringWithNilDefaults(t *testing.T) {
	fn := NewFunction(FunctionOpts{
		Name:       "greet",
		Parameters: []string{"name", "greeting"},
		Defaults:   []any{nil, "Hello"},
	})

	str := fn.String()

	// Should show default for greeting but not for name
	assert.True(t, strings.Contains(str, "greeting=Hello"))
	assert.True(t, !strings.Contains(str, "name="))
}

func TestFunctionStringEmptyDefaults(t *testing.T) {
	fn := NewFunction(FunctionOpts{
		Name:       "add",
		Parameters: []string{"x", "y"},
		Defaults:   []any{}, // Empty defaults
	})

	// Should not panic
	str := fn.String()
	assert.True(t, strings.Contains(str, "(x, y)"))
}
