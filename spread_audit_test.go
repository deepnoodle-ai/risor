package risor

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestMethodCallSpread(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{`let o = {add: function(a, b) { return a + b }}; o.add(...[1, 2])`, int64(3)},
		{`let o = {add: function(a, b, c) { return a + b + c }}; o.add(1, ...[2], 3)`, int64(6)},
		{`let o = {add: function(a, b) { return a + b }}; o?.add(...[1, 2])`, int64(3)},
		{`let o = nil; o?.add(...[1, 2])`, nil},
		{`let o = {f: function(...xs) { return xs }}; o.f(...[1, 2], ...[3])`, []any{int64(1), int64(2), int64(3)}},
		{`let args = ["-"]; "a-b".split(...args)`, []any{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Eval(context.Background(), tt.input, WithEnv(Builtins()))
			assert.Nil(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestMisplacedSpreadAndDefaultValueAreSyntaxErrors(t *testing.T) {
	inputs := []string{
		`let x = [1]; ...x`,
		`let x = [1]; let y = ...x`,
		`let a = 2; [a = 1]`,
		`let a = 2; {a = 1}`,
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := Eval(context.Background(), input)
			assert.NotNil(t, err)
		})
	}
}
