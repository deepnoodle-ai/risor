package parser

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/ast"
)

// Tests for literal parsing (literals.go)
// - Integer literals
// - Float literals
// - Boolean literals
// - Nil literal
// - String literals (including template strings)
// - List literals
// - Map literals
// - Function literals
// - Spread expressions

func TestInt(t *testing.T) {
	tests := []struct {
		input   string
		value   int64
		literal string
	}{
		{"0", 0, "0"},
		{"5", 5, "5"},
		{"10", 10, "10"},
		{"9876543210", 9876543210, "9876543210"},
		{"0x10", 16, "0x10"},
		{"0x1a", 26, "0x1a"},
		{"0x1A", 26, "0x1A"},
		{"010", 8, "010"},
		{"011", 9, "011"},
		{"0755", 493, "0755"},
		{"00", 0, "00"},
		{"100", 100, "100"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			integer, ok := program.First().(*ast.Int)
			assert.True(t, ok, "got %T", program.First())
			assert.Equal(t, tt.value, integer.Value)
			assert.Equal(t, tt.literal, integer.Literal)
			assert.NotEqual(t, integer.Pos(), integer.End()) // has span
		})
	}
}

func TestIntAST(t *testing.T) {
	program, err := Parse(context.Background(), "42")
	assert.Nil(t, err)

	integer, ok := program.First().(*ast.Int)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Equal(t, int64(42), integer.Value)
	assert.Equal(t, "42", integer.Literal)
	assert.Equal(t, "42", integer.String())
	assert.Equal(t, 0, integer.ValuePos.Line)
	assert.Equal(t, 0, integer.ValuePos.Column)
}

func TestFloat(t *testing.T) {
	tests := []struct {
		input   string
		value   float64
		literal string
	}{
		{"0.0", 0.0, "0.0"},
		{"1.5", 1.5, "1.5"},
		{"3.14159", 3.14159, "3.14159"},
		{"0.001", 0.001, "0.001"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			float, ok := program.First().(*ast.Float)
			assert.True(t, ok, "got %T", program.First())
			assert.Equal(t, tt.value, float.Value)
			assert.Equal(t, tt.literal, float.Literal)
		})
	}
}

func TestFloatAST(t *testing.T) {
	program, err := Parse(context.Background(), "3.14")
	assert.Nil(t, err)

	float, ok := program.First().(*ast.Float)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Equal(t, 3.14, float.Value)
	assert.Equal(t, "3.14", float.Literal)
	assert.Equal(t, "3.14", float.String())
}

func TestBool(t *testing.T) {
	tests := []struct {
		input   string
		value   bool
		literal string
	}{
		{"true", true, "true"},
		{"false", false, "false"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			exp, ok := program.First().(*ast.Bool)
			assert.True(t, ok)
			assert.Equal(t, tt.value, exp.Value)
			assert.Equal(t, tt.literal, exp.Literal)
		})
	}
}

func TestBoolAST(t *testing.T) {
	program, err := Parse(context.Background(), "true")
	assert.Nil(t, err)

	b, ok := program.First().(*ast.Bool)
	assert.True(t, ok)

	// Verify AST node fields
	assert.True(t, b.Value)
	assert.Equal(t, "true", b.Literal)
	assert.Equal(t, "true", b.String())
}

func TestNil(t *testing.T) {
	program, err := Parse(context.Background(), "nil")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	n, ok := program.First().(*ast.Nil)
	assert.True(t, ok)
	assert.Equal(t, "nil", n.String())
}

func TestString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello world"`, "hello world"},
		{`"with \"quotes\""`, `with "quotes"`},
		{`"line\nbreak"`, "line\nbreak"},
		{`'single quotes'`, "single quotes"},
		{`""`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			literal, ok := program.First().(*ast.String)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, literal.Value)
		})
	}
}

func TestStringAST(t *testing.T) {
	program, err := Parse(context.Background(), `"hello"`)
	assert.Nil(t, err)

	str, ok := program.First().(*ast.String)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Equal(t, "hello", str.Value)
	assert.Equal(t, "hello", str.Literal)
	assert.Nil(t, str.Template) // No interpolation
	assert.Nil(t, str.Exprs)
}

func TestBacktick(t *testing.T) {
	input := "`" + `\\n\t foo bar /hey there/` + "`"
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	expr, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Equal(t, `\\n\t foo bar /hey there/`, expr.Value)
}

func TestTemplateStringInterpolation(t *testing.T) {
	t.Run("simple interpolation", func(t *testing.T) {
		input := "`hello ${name}`"
		program, err := Parse(context.Background(), input)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)

		str, ok := program.First().(*ast.String)
		assert.True(t, ok)
		assert.NotNil(t, str.Template)
		assert.Len(t, str.Exprs, 1)

		ident, ok := str.Exprs[0].(*ast.Ident)
		assert.True(t, ok)
		assert.Equal(t, "name", ident.Name)
	})

	t.Run("multiple interpolations", func(t *testing.T) {
		input := "`${a} and ${b}`"
		program, err := Parse(context.Background(), input)
		assert.Nil(t, err)

		str, ok := program.First().(*ast.String)
		assert.True(t, ok)
		assert.NotNil(t, str.Template)
		assert.Len(t, str.Exprs, 2)
	})

	t.Run("expression interpolation", func(t *testing.T) {
		input := "`result: ${1 + 2}`"
		program, err := Parse(context.Background(), input)
		assert.Nil(t, err)

		str, ok := program.First().(*ast.String)
		assert.True(t, ok)
		assert.NotNil(t, str.Template)
		assert.Len(t, str.Exprs, 1)

		_, ok = str.Exprs[0].(*ast.Infix)
		assert.True(t, ok)
	})

	t.Run("no interpolation", func(t *testing.T) {
		input := "`plain string`"
		program, err := Parse(context.Background(), input)
		assert.Nil(t, err)

		str, ok := program.First().(*ast.String)
		assert.True(t, ok)
		assert.Nil(t, str.Template)
	})
}

func TestList(t *testing.T) {
	program, err := Parse(context.Background(), "[1, 2*2, 3+3]")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	ll, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, ll.Items, 3)

	testIntegerLiteral(t, ll.Items[0], 1)
	testInfixExpression(t, ll.Items[1], 2, "*", 2)
	testInfixExpression(t, ll.Items[2], 3, "+", 3)
}

func TestListAST(t *testing.T) {
	program, err := Parse(context.Background(), "[1, 2, 3]")
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Len(t, list.Items, 3)
	assert.Equal(t, "[1, 2, 3]", list.String())

	// Verify individual items
	for i, item := range list.Items {
		integer, ok := item.(*ast.Int)
		assert.True(t, ok)
		assert.Equal(t, int64(i+1), integer.Value)
	}
}

func TestListEmpty(t *testing.T) {
	program, err := Parse(context.Background(), "[]")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 0)
}

func TestListWithTrailingComma(t *testing.T) {
	program, err := Parse(context.Background(), "[1, 2, 3,]")
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 3)
}

func TestListWithNewlines(t *testing.T) {
	input := `[
		1,
		2,
		3
	]`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 3)
}

func TestMap(t *testing.T) {
	input := `{"one":1, "two":2, "three":3}`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 3)

	expected := map[string]int64{
		"one":   1,
		"two":   2,
		"three": 3,
	}
	for _, item := range m.Items {
		literal, ok := item.Key.(*ast.String)
		assert.True(t, ok)
		expectedValue := expected[literal.Value]
		testIntegerLiteral(t, item.Value, expectedValue)
	}
}

func TestMapAST(t *testing.T) {
	program, err := Parse(context.Background(), `{a: 1, b: 2}`)
	assert.Nil(t, err)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)

	// Verify AST node fields
	assert.Len(t, m.Items, 2)

	// First item
	key1, ok := m.Items[0].Key.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "a", key1.Name)

	val1, ok := m.Items[0].Value.(*ast.Int)
	assert.True(t, ok)
	assert.Equal(t, int64(1), val1.Value)

	// Second item
	key2, ok := m.Items[1].Key.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "b", key2.Name)
}

func TestMapEmpty(t *testing.T) {
	program, err := Parse(context.Background(), "{}")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 0)
}

func TestMapIdentifierKey(t *testing.T) {
	input := "{ one: 1 }"
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 1)

	ident, ok := m.Items[0].Key.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "one", ident.String())
}

func TestMapWithExpressionValues(t *testing.T) {
	input := `{"one":0+1, "two":10 - 8, "three": 15/5}`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 3)

	tests := map[string]func(ast.Expr){
		"one": func(e ast.Expr) {
			testInfixExpression(t, e, 0, "+", 1)
		},
		"two": func(e ast.Expr) {
			testInfixExpression(t, e, 10, "-", 8)
		},
		"three": func(e ast.Expr) {
			testInfixExpression(t, e, 15, "/", 5)
		},
	}
	for _, item := range m.Items {
		literal, ok := item.Key.(*ast.String)
		assert.True(t, ok)
		testFunc, ok := tests[literal.Value]
		assert.True(t, ok, literal.Value)
		testFunc(item.Value)
	}
}

func TestMapWithNewlines(t *testing.T) {
	input := `{
		"a": "b",

		"c": "d",

	}`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 2)
}

func TestFunc(t *testing.T) {
	program, err := Parse(context.Background(), "function f(x, y=3) { x + y; }")
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	function, ok := program.First().(*ast.Func)
	assert.True(t, ok)

	// Verify name
	assert.NotNil(t, function.Name)
	assert.Equal(t, "f", function.Name.Name)

	// Verify params
	params := function.Params
	assert.Len(t, params, 2)
	testLiteralExpression(t, params[0], "x")
	testLiteralExpression(t, params[1], "y")

	// Verify defaults
	assert.Len(t, function.Defaults, 1)
	assert.Contains(t, function.Defaults, "y")

	// Verify body
	assert.Len(t, function.Body.Stmts, 1)
	bodyStmt, ok := function.Body.Stmts[0].(*ast.Infix)
	assert.True(t, ok)
	assert.Equal(t, "(x + y)", bodyStmt.String())
}

func TestFuncAST(t *testing.T) {
	program, err := Parse(context.Background(), "function add(a, b) { return a + b }")
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)

	// Verify AST node fields
	assert.NotNil(t, fn.Name)
	assert.Equal(t, "add", fn.Name.Name)
	assert.Len(t, fn.Params, 2)
	assert.Equal(t, "a", fn.Params[0].Name)
	assert.Equal(t, "b", fn.Params[1].Name)
	assert.Nil(t, fn.RestParam)
	assert.NotNil(t, fn.Body)
	assert.Len(t, fn.Body.Stmts, 1)
}

func TestFuncParams(t *testing.T) {
	tests := []struct {
		input         string
		expectedParam []string
	}{
		{"function(){}", []string{}},
		{"function(x){}", []string{"x"}},
		{"function(x,y){}", []string{"x", "y"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			function, ok := program.First().(*ast.Func)
			assert.True(t, ok)
			assert.Len(t, function.Params, len(tt.expectedParam))
			for i, ident := range tt.expectedParam {
				testLiteralExpression(t, function.Params[i], ident)
			}
		})
	}
}

func TestFuncParamsWithNewlines(t *testing.T) {
	input := `function f(
		a,
		b =
			2,
		...rest
	) { return a }`
	program, err := Parse(context.Background(), input)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	function, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, function.Params, 2)
	assert.Equal(t, "a", function.Params[0].Name)
	assert.Equal(t, "b", function.Params[1].Name)
	assert.Len(t, function.Defaults, 1)
	assert.Contains(t, function.Defaults, "b")
	assert.NotNil(t, function.RestParam)
	assert.Equal(t, "rest", function.RestParam.Name)
}

func TestFuncAnonymous(t *testing.T) {
	program, err := Parse(context.Background(), "function(x) { x }")
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Nil(t, fn.Name) // Anonymous
	assert.Len(t, fn.Params, 1)
}

func TestRestParameter(t *testing.T) {
	t.Run("basic rest param", func(t *testing.T) {
		program, err := Parse(context.Background(), `function f(a, b, ...rest) { rest }`)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 2)
		assert.Equal(t, "a", fn.Params[0].Name)
		assert.Equal(t, "b", fn.Params[1].Name)
		assert.NotNil(t, fn.RestParam)
		assert.Equal(t, "rest", fn.RestParam.Name)
	})

	t.Run("rest param only", func(t *testing.T) {
		program, err := Parse(context.Background(), `function f(...args) { args }`)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 0)
		assert.NotNil(t, fn.RestParam)
		assert.Equal(t, "args", fn.RestParam.Name)
	})
}

func TestRestParameterErrors(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`function f(...a, b) {}`, "rest parameter must be the last parameter"},
		{`function f(...a, ...b) {}`, "rest parameter must be the last parameter"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

func TestSpreadOperator(t *testing.T) {
	t.Run("spread in list", func(t *testing.T) {
		program, err := Parse(context.Background(), `[1, ...arr, 2]`)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)

		list, ok := program.First().(*ast.List)
		assert.True(t, ok)
		assert.Len(t, list.Items, 3)

		// Verify spread AST
		spread, ok := list.Items[1].(*ast.Spread)
		assert.True(t, ok)
		assert.Equal(t, "arr", spread.X.String())
	})

	t.Run("spread in function call", func(t *testing.T) {
		program, err := Parse(context.Background(), `f(1, ...args, 2)`)
		assert.Nil(t, err)

		call, ok := program.First().(*ast.Call)
		assert.True(t, ok)
		assert.Len(t, call.Args, 3)

		spread, ok := call.Args[1].(*ast.Spread)
		assert.True(t, ok)
		assert.Equal(t, "args", spread.X.String())
	})

	t.Run("spread in map", func(t *testing.T) {
		program, err := Parse(context.Background(), `{a: 1, ...obj, b: 2}`)
		assert.Nil(t, err)

		m, ok := program.First().(*ast.Map)
		assert.True(t, ok)
		assert.Len(t, m.Items, 3)

		// Spread item has nil key
		assert.Nil(t, m.Items[1].Key)
		spread, ok := m.Items[1].Value.(*ast.Spread)
		assert.True(t, ok)
		assert.Equal(t, "obj", spread.X.String())
	})
}

func TestSpreadAST(t *testing.T) {
	program, err := Parse(context.Background(), "[...items]")
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)

	spread, ok := list.Items[0].(*ast.Spread)
	assert.True(t, ok)

	// Verify AST node fields
	ident, ok := spread.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "items", ident.Name)
	assert.Equal(t, "...items", spread.String())
}
