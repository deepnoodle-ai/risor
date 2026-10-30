package parser

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Spread is only valid in list literals, map literals, call arguments and
// rest parameters. Everywhere else it is a syntax error.
func TestSpreadOutsideValidContexts(t *testing.T) {
	inputs := []string{
		"...x",
		"let a = ...x",
		"1 + ...x",
		"return ...x",
		"(...x)",
		"(...x) => x",
		"x = ...y",
		"[...a + b]",
		"f(...a + b)",
		"if (...x) { 1 }",
		"{a: ...x}",
		"x[...y]",
		"[...(...x)]",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(context.Background(), input, nil)
			assert.NotNil(t, err)
		})
	}
}

func TestSpreadMisplacedErrorMessage(t *testing.T) {
	_, err := Parse(context.Background(), "let a = ...x", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "spread operator")
}

func TestSpreadInValidContexts(t *testing.T) {
	inputs := []string{
		"[...x]",
		"[1, ...x, 2]",
		"[...x.y, ...f(), ...[1], ...a[0]]",
		"{...x}",
		"{a: 1, ...x}",
		"f(...x)",
		"f(1, ...x, 2)",
		"f(\n  ...x,\n)",
		"o.m(...x)",
		"o?.m(...x)",
		"function f(a, ...rest) { return rest }",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(context.Background(), input, nil)
			assert.Nil(t, err)
		})
	}
}

// `a = 1` inside a list or map literal is only valid when the literal is an
// arrow function destructuring parameter.
func TestDefaultValueOutsideArrowParams(t *testing.T) {
	inputs := []string{
		"[a = 1]",
		"{a = 1}",
		"let x = [a = 1]",
		"let x = {a = 1}",
		"f([a = 1])",
		"f({a = 1})",
		"[[a = 1]]",
		"[{a = 1}]",
		"{b: [a = 1]}",
		"([a = 1])",
		"({a = 1})",
		"([a = 1] + 1)",
		"({a = 1}.a)",
		"(([a = 1])) => a",
		"(x = [a = 1]) => x",
		"([[a = 1]]) => 1",
		"(x) => [a = 1]",
		"([a = 1]).b",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(context.Background(), input, nil)
			assert.NotNil(t, err)
		})
	}
}

func TestDefaultValueErrorMessage(t *testing.T) {
	_, err := Parse(context.Background(), "[a = 1]", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "arrow function")
}

func TestDefaultValueInArrowParams(t *testing.T) {
	inputs := []string{
		"([a = 1]) => a",
		"({a = 1}) => a",
		"([a, b = 2]) => a + b",
		"({a, b = 2}) => a + b",
		"(x, [a = 1], {b = 2}) => x + a + b",
		"let f = ([a = [1, 2]]) => a",
		"let f = ({a = {b: 1}}) => a",
		"xs.map(([k, v = 0]) => k)",
		"([a = 1, b = ([c = 2]) => c]) => b(a)",
		"([a = (([b = 1]) => b)()]) => a",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			prog, err := Parse(context.Background(), input, nil)
			assert.Nil(t, err)
			ast.Inspect(prog, func(n ast.Node) bool {
				_, isDefault := n.(*ast.DefaultValue)
				assert.False(t, isDefault)
				return true
			})
		})
	}
}
