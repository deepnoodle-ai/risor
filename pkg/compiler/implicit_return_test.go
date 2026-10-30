package compiler

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// The implicit "return nil" at the end of a function maps to the function's
// closing brace, not to line 1, column 1.
func TestImplicitReturnLocation(t *testing.T) {
	tests := []struct {
		name   string
		source string
		lines  []int // lines the function's instructions may map to
		end    int   // line and column of the closing brace
		endCol int
	}{
		{"empty body", "let a = 1\nfunction f() {\n}", []int{3}, 3, 1},
		{"statement body", "let a = 1\nfunction f() {\n  let x = 1\n}", []int{3, 4}, 4, 1},
		{"one line", "let a = 1\n\nfunction f() { let x = 1 }", []int{3}, 3, 26},
		{"arrow", "let a = 1\nlet f = () => {\n}", []int{3}, 3, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parser.Parse(context.Background(), tt.source, nil)
			assert.Nil(t, err)
			code, err := Compile(parsed, &Config{Source: tt.source})
			assert.Nil(t, err)
			assert.Equal(t, code.ChildCount(), 1)
			fn := code.ChildAt(0)
			count := fn.LocationCount()
			assert.True(t, count > 0)
			for i := 0; i < count; i++ {
				loc := fn.LocationAt(i)
				assert.Contains(t, tt.lines, loc.Line, "instruction %d at line %d", i, loc.Line)
			}
			last := fn.LocationAt(count - 1)
			assert.Equal(t, last.Line, tt.end)
			assert.Equal(t, last.Column, tt.endCol)
		})
	}
}
