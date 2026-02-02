package parser

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2/ast"
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

// getParamName extracts the name from a FuncParam (assumes it's an Ident)
func getParamName(p ast.FuncParam) string {
	if ident, ok := p.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

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
		{"0b0", 0, "0b0"},
		{"0b1", 1, "0b1"},
		{"0b10", 2, "0b10"},
		{"0b1010", 10, "0b1010"},
		{"0b11111111", 255, "0b11111111"},
		{"010", 8, "010"},
		{"011", 9, "011"},
		{"0755", 493, "0755"},
		{"00", 0, "00"},
		{"100", 100, "100"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
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
	program, err := Parse(context.Background(), "42", nil)
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
			program, err := Parse(context.Background(), tt.input, nil)
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
	program, err := Parse(context.Background(), "3.14", nil)
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
			program, err := Parse(context.Background(), tt.input, nil)
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
	program, err := Parse(context.Background(), "true", nil)
	assert.Nil(t, err)

	b, ok := program.First().(*ast.Bool)
	assert.True(t, ok)

	// Verify AST node fields
	assert.True(t, b.Value)
	assert.Equal(t, "true", b.Literal)
	assert.Equal(t, "true", b.String())
}

func TestNil(t *testing.T) {
	program, err := Parse(context.Background(), "nil", nil)
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
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			literal, ok := program.First().(*ast.String)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, literal.Value)
		})
	}
}

func TestStringAST(t *testing.T) {
	program, err := Parse(context.Background(), `"hello"`, nil)
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
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	expr, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Equal(t, `\\n\t foo bar /hey there/`, expr.Value)
}

func TestTemplateStringInterpolation(t *testing.T) {
	t.Run("simple interpolation", func(t *testing.T) {
		input := "`hello ${name}`"
		program, err := Parse(context.Background(), input, nil)
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
		program, err := Parse(context.Background(), input, nil)
		assert.Nil(t, err)

		str, ok := program.First().(*ast.String)
		assert.True(t, ok)
		assert.NotNil(t, str.Template)
		assert.Len(t, str.Exprs, 2)
	})

	t.Run("expression interpolation", func(t *testing.T) {
		input := "`result: ${1 + 2}`"
		program, err := Parse(context.Background(), input, nil)
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
		program, err := Parse(context.Background(), input, nil)
		assert.Nil(t, err)

		str, ok := program.First().(*ast.String)
		assert.True(t, ok)
		assert.Nil(t, str.Template)
	})
}

func TestList(t *testing.T) {
	program, err := Parse(context.Background(), "[1, 2*2, 3+3]", nil)
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
	program, err := Parse(context.Background(), "[1, 2, 3]", nil)
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
	program, err := Parse(context.Background(), "[]", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 0)
}

func TestListEmptyWithNewlines(t *testing.T) {
	program, err := Parse(context.Background(), "[\n]", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 0)
}

func TestListWithTrailingComma(t *testing.T) {
	program, err := Parse(context.Background(), "[1, 2, 3,]", nil)
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
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 3)
}

func TestMap(t *testing.T) {
	input := `{"one":1, "two":2, "three":3}`
	program, err := Parse(context.Background(), input, nil)
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
	program, err := Parse(context.Background(), `{a: 1, b: 2}`, nil)
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
	program, err := Parse(context.Background(), "{}", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 0)
}

func TestMapIdentifierKey(t *testing.T) {
	input := "{ one: 1 }"
	program, err := Parse(context.Background(), input, nil)
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
	program, err := Parse(context.Background(), input, nil)
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
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 2)
}

func TestMapShorthand(t *testing.T) {
	t.Run("simple shorthand", func(t *testing.T) {
		// {a, b} should be equivalent to {a: a, b: b}
		program, err := Parse(context.Background(), "{a, b}", nil)
		assert.Nil(t, err)

		m, ok := program.First().(*ast.Map)
		assert.True(t, ok)
		assert.Len(t, m.Items, 2)

		// First item: key="a", value=ident(a)
		key0, ok := m.Items[0].Key.(*ast.String)
		assert.True(t, ok)
		assert.Equal(t, key0.Value, "a")
		val0, ok := m.Items[0].Value.(*ast.Ident)
		assert.True(t, ok)
		assert.Equal(t, val0.Name, "a")

		// Second item: key="b", value=ident(b)
		key1, ok := m.Items[1].Key.(*ast.String)
		assert.True(t, ok)
		assert.Equal(t, key1.Value, "b")
		val1, ok := m.Items[1].Value.(*ast.Ident)
		assert.True(t, ok)
		assert.Equal(t, val1.Name, "b")
	})

	t.Run("mixed shorthand and explicit", func(t *testing.T) {
		// {a, b: 2, c} should have shorthand for a and c, explicit for b
		program, err := Parse(context.Background(), "{a, b: 2, c}", nil)
		assert.Nil(t, err)

		m, ok := program.First().(*ast.Map)
		assert.True(t, ok)
		assert.Len(t, m.Items, 3)

		// Item 0: shorthand a
		key0, ok := m.Items[0].Key.(*ast.String)
		assert.True(t, ok)
		assert.Equal(t, key0.Value, "a")

		// Item 1: explicit b: 2
		key1, ok := m.Items[1].Key.(*ast.Ident)
		assert.True(t, ok)
		assert.Equal(t, key1.Name, "b")

		// Item 2: shorthand c
		key2, ok := m.Items[2].Key.(*ast.String)
		assert.True(t, ok)
		assert.Equal(t, key2.Value, "c")
	})

	t.Run("shorthand with default", func(t *testing.T) {
		// {a = 10} should produce a DefaultValue
		program, err := Parse(context.Background(), "{a = 10}", nil)
		assert.Nil(t, err)

		m, ok := program.First().(*ast.Map)
		assert.True(t, ok)
		assert.Len(t, m.Items, 1)

		key, ok := m.Items[0].Key.(*ast.String)
		assert.True(t, ok)
		assert.Equal(t, key.Value, "a")

		defaultVal, ok := m.Items[0].Value.(*ast.DefaultValue)
		assert.True(t, ok)
		assert.Equal(t, defaultVal.Name.Name, "a")
	})
}

func TestFunc(t *testing.T) {
	program, err := Parse(context.Background(), "function f(x, y=3) { x + y; }", nil)
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
	assert.Equal(t, "x", getParamName(params[0]))
	assert.Equal(t, "y", getParamName(params[1]))

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
	program, err := Parse(context.Background(), "function add(a, b) { return a + b }", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)

	// Verify AST node fields
	assert.NotNil(t, fn.Name)
	assert.Equal(t, "add", fn.Name.Name)
	assert.Len(t, fn.Params, 2)
	assert.Equal(t, "a", getParamName(fn.Params[0]))
	assert.Equal(t, "b", getParamName(fn.Params[1]))
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
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Len(t, program.Stmts, 1)

			function, ok := program.First().(*ast.Func)
			assert.True(t, ok)
			assert.Len(t, function.Params, len(tt.expectedParam))
			for i, expectedName := range tt.expectedParam {
				assert.Equal(t, expectedName, getParamName(function.Params[i]))
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
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)

	function, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, function.Params, 2)
	assert.Equal(t, "a", getParamName(function.Params[0]))
	assert.Equal(t, "b", getParamName(function.Params[1]))
	assert.Len(t, function.Defaults, 1)
	assert.Contains(t, function.Defaults, "b")
	assert.NotNil(t, function.RestParam)
	assert.Equal(t, "rest", function.RestParam.Name)
}

func TestFuncAnonymous(t *testing.T) {
	program, err := Parse(context.Background(), "function(x) { x }", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Nil(t, fn.Name) // Anonymous
	assert.Len(t, fn.Params, 1)
}

func TestRestParameter(t *testing.T) {
	t.Run("basic rest param", func(t *testing.T) {
		program, err := Parse(context.Background(), `function f(a, b, ...rest) { rest }`, nil)
		assert.Nil(t, err)
		assert.Len(t, program.Stmts, 1)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 2)
		assert.Equal(t, "a", getParamName(fn.Params[0]))
		assert.Equal(t, "b", getParamName(fn.Params[1]))
		assert.NotNil(t, fn.RestParam)
		assert.Equal(t, "rest", fn.RestParam.Name)
	})

	t.Run("rest param only", func(t *testing.T) {
		program, err := Parse(context.Background(), `function f(...args) { args }`, nil)
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
			_, err := Parse(context.Background(), tt.input, nil)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

// =============================================================================
// DESTRUCTURING PARAMETERS
// =============================================================================

func TestObjectDestructureParam(t *testing.T) {
	tests := []struct {
		input         string
		expectedNames []string
		desc          string
	}{
		{`function foo({a, b}) { a + b }`, []string{"a", "b"}, "basic object destructure"},
		{`function foo({x}) { x }`, []string{"x"}, "single binding"},
		{`function foo({a, b, c}) { a }`, []string{"a", "b", "c"}, "three bindings"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)

			fn, ok := program.First().(*ast.Func)
			assert.True(t, ok, "Expected Func, got %T", program.First())
			assert.Len(t, fn.Params, 1)

			dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
			assert.True(t, ok, "Expected ObjectDestructureParam, got %T", fn.Params[0])
			assert.Len(t, dp.Bindings, len(tt.expectedNames))

			names := dp.ParamNames()
			assert.Equal(t, tt.expectedNames, names)
		})
	}
}

func TestObjectDestructureParamWithDefaults(t *testing.T) {
	program, err := Parse(context.Background(), `function foo({a, b = 10}) { a + b }`, nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 1)

	dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
	assert.True(t, ok, "Expected ObjectDestructureParam, got %T", fn.Params[0])
	assert.Len(t, dp.Bindings, 2)
	assert.Equal(t, "a", dp.Bindings[0].Key)
	assert.Nil(t, dp.Bindings[0].Default)
	assert.Equal(t, "b", dp.Bindings[1].Key)
	assert.NotNil(t, dp.Bindings[1].Default)
}

func TestObjectDestructureParamWithAlias(t *testing.T) {
	program, err := Parse(context.Background(), `function foo({name: n}) { n }`, nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 1)

	dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
	assert.True(t, ok, "Expected ObjectDestructureParam, got %T", fn.Params[0])
	assert.Len(t, dp.Bindings, 1)
	assert.Equal(t, "name", dp.Bindings[0].Key)
	assert.Equal(t, "n", dp.Bindings[0].Alias)
}

func TestArrayDestructureParam(t *testing.T) {
	tests := []struct {
		input         string
		expectedNames []string
		desc          string
	}{
		{`function foo([a, b]) { a + b }`, []string{"a", "b"}, "basic array destructure"},
		{`function foo([x]) { x }`, []string{"x"}, "single element"},
		{`function foo([a, b, c]) { a }`, []string{"a", "b", "c"}, "three elements"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)

			fn, ok := program.First().(*ast.Func)
			assert.True(t, ok, "Expected Func, got %T", program.First())
			assert.Len(t, fn.Params, 1)

			dp, ok := fn.Params[0].(*ast.ArrayDestructureParam)
			assert.True(t, ok, "Expected ArrayDestructureParam, got %T", fn.Params[0])
			assert.Len(t, dp.Elements, len(tt.expectedNames))

			names := dp.ParamNames()
			assert.Equal(t, tt.expectedNames, names)
		})
	}
}

func TestArrayDestructureParamWithDefaults(t *testing.T) {
	program, err := Parse(context.Background(), `function foo([a, b = 10]) { a + b }`, nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 1)

	dp, ok := fn.Params[0].(*ast.ArrayDestructureParam)
	assert.True(t, ok, "Expected ArrayDestructureParam, got %T", fn.Params[0])
	assert.Len(t, dp.Elements, 2)
	assert.Equal(t, "a", dp.Elements[0].Name.Name)
	assert.Nil(t, dp.Elements[0].Default)
	assert.Equal(t, "b", dp.Elements[1].Name.Name)
	assert.NotNil(t, dp.Elements[1].Default)
}

func TestMixedDestructureAndRegularParams(t *testing.T) {
	program, err := Parse(context.Background(), `function foo(x, {a, b}, [c, d], y) { x }`, nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 4)

	// First param: regular identifier
	_, ok = fn.Params[0].(*ast.Ident)
	assert.True(t, ok, "Expected Ident for first param")

	// Second param: object destructure
	_, ok = fn.Params[1].(*ast.ObjectDestructureParam)
	assert.True(t, ok, "Expected ObjectDestructureParam for second param")

	// Third param: array destructure
	_, ok = fn.Params[2].(*ast.ArrayDestructureParam)
	assert.True(t, ok, "Expected ArrayDestructureParam for third param")

	// Fourth param: regular identifier
	_, ok = fn.Params[3].(*ast.Ident)
	assert.True(t, ok, "Expected Ident for fourth param")
}

func TestArrowFunctionWithDestructureParams(t *testing.T) {
	// Note: Arrow function destructuring requires the pattern to be parsed as Map/List first
	// then converted. This test verifies basic support.
	program, err := Parse(context.Background(), `([a, b]) => a + b`, nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 1)

	_, ok = fn.Params[0].(*ast.ArrayDestructureParam)
	assert.True(t, ok, "Expected ArrayDestructureParam for arrow param")
}

// =============================================================================
// DESTRUCTURING PARAMETERS - EDGE CASES
// =============================================================================

func TestDestructureParamWithNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			"object destructure with newlines",
			`function foo({
				a,
				b
			}) { a }`,
		},
		{
			"array destructure with newlines",
			`function foo([
				a,
				b
			]) { a }`,
		},
		{
			"mixed with newlines",
			`function foo(
				x,
				{a, b},
				[c, d],
				y
			) { x }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)
			assert.NotNil(t, program.First())
		})
	}
}

func TestDestructureParamEdgeCases(t *testing.T) {
	t.Run("empty object destructure", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo({}) { 1 }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 1)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 0)
	})

	t.Run("empty array destructure", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo([]) { 1 }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)
		assert.Len(t, fn.Params, 1)

		dp, ok := fn.Params[0].(*ast.ArrayDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Elements, 0)
	})

	t.Run("single binding object destructure", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo({x}) { x }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 1)
		assert.Equal(t, "x", dp.Bindings[0].Key)
	})

	t.Run("trailing comma in object destructure", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo({a, b,}) { a }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 2)
	})

	t.Run("trailing comma in array destructure", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo([a, b,]) { a }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ArrayDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Elements, 2)
	})

	t.Run("all bindings with defaults", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo({a = 1, b = 2, c = 3}) { a }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 3)
		for _, binding := range dp.Bindings {
			assert.NotNil(t, binding.Default, "Expected default for %s", binding.Key)
		}
	})

	t.Run("alias with default", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo({name: n = "default"}) { n }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Len(t, dp.Bindings, 1)
		assert.Equal(t, "name", dp.Bindings[0].Key)
		assert.Equal(t, "n", dp.Bindings[0].Alias)
		assert.NotNil(t, dp.Bindings[0].Default)
	})
}

func TestDestructureParamWithRestParam(t *testing.T) {
	program, err := Parse(context.Background(), `function foo({a, b}, ...rest) { a }`, nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 1)
	assert.NotNil(t, fn.RestParam)
	assert.Equal(t, "rest", fn.RestParam.Name)

	_, ok = fn.Params[0].(*ast.ObjectDestructureParam)
	assert.True(t, ok)
}

func TestDestructureParamString(t *testing.T) {
	// Test that String() methods work correctly
	t.Run("object destructure string", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo({a, b}) { a }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Equal(t, "{a, b}", dp.String())
	})

	t.Run("object destructure with alias string", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo({name: n}) { n }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Equal(t, "{name: n}", dp.String())
	})

	t.Run("object destructure with default string", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo({a = 1}) { a }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ObjectDestructureParam)
		assert.True(t, ok)
		assert.Equal(t, "{a = 1}", dp.String())
	})

	t.Run("array destructure string", func(t *testing.T) {
		program, err := Parse(context.Background(), `function foo([a, b]) { a }`, nil)
		assert.Nil(t, err)

		fn, ok := program.First().(*ast.Func)
		assert.True(t, ok)

		dp, ok := fn.Params[0].(*ast.ArrayDestructureParam)
		assert.True(t, ok)
		assert.Equal(t, "[a, b]", dp.String())
	})
}

func TestDestructureParamInFuncString(t *testing.T) {
	// Test that Func.String() correctly formats destructure params
	program, err := Parse(context.Background(), `function foo({a, b}, [c, d]) { a }`, nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)

	str := fn.String()
	assert.Contains(t, str, "function foo({a, b}, [c, d])")
}

func TestDestructureParamErrors(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		desc     string
	}{
		{
			`function foo({123}) { 1 }`,
			"expected identifier",
			"number in object destructure",
		},
		{
			`function foo([123]) { 1 }`,
			"expected identifier",
			"number in array destructure",
		},
		{
			`function foo({a +}) { 1 }`,
			"expected",
			"invalid token in object destructure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.NotNil(t, err, "Expected error for: %s", tt.input)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

func TestSpreadOperator(t *testing.T) {
	t.Run("spread in list", func(t *testing.T) {
		program, err := Parse(context.Background(), `[1, ...arr, 2]`, nil)
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
		program, err := Parse(context.Background(), `f(1, ...args, 2)`, nil)
		assert.Nil(t, err)

		call, ok := program.First().(*ast.Call)
		assert.True(t, ok)
		assert.Len(t, call.Args, 3)

		spread, ok := call.Args[1].(*ast.Spread)
		assert.True(t, ok)
		assert.Equal(t, "args", spread.X.String())
	})

	t.Run("spread in map", func(t *testing.T) {
		program, err := Parse(context.Background(), `{a: 1, ...obj, b: 2}`, nil)
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
	program, err := Parse(context.Background(), "[...items]", nil)
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
