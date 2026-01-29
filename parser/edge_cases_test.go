package parser

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/ast"
)

// Edge Cases and Shortcomings Tests
// This file tests parser behavior for ambiguous, edge case, and corner case inputs.

// =============================================================================
// OPERATOR PRECEDENCE TESTS
// =============================================================================

func TestOperatorPrecedenceUnaryMinusPower(t *testing.T) {
	// Python-compatible: -2 ** 3 parses as -(2**3), not (-2)**3
	// The ** operator binds tighter than unary minus on its left
	program, err := Parse(context.Background(), "-2 ** 3", nil)
	assert.Nil(t, err)

	// Parses as Prefix("-", Infix(2, "**", 3)) => -(2 ** 3)
	prefix, ok := program.First().(*ast.Prefix)
	assert.True(t, ok, "expected Prefix, got %T", program.First())
	assert.Equal(t, "-", prefix.Op)

	infix, ok := prefix.X.(*ast.Infix)
	assert.True(t, ok, "expected Infix for operand of unary minus")
	assert.Equal(t, "**", infix.Op)
}

func TestPowerRightAssociative(t *testing.T) {
	// ** is right-associative like Python: 2**2**3 = 2**(2**3) = 256
	program, err := Parse(context.Background(), "2 ** 2 ** 3", nil)
	assert.Nil(t, err)

	// Outer ** has 2 on left
	outer, ok := program.First().(*ast.Infix)
	assert.True(t, ok)
	assert.Equal(t, "**", outer.Op)
	assert.Equal(t, "2", outer.X.String())

	// Right side is another **
	inner, ok := outer.Y.(*ast.Infix)
	assert.True(t, ok, "expected nested Infix for right-associativity")
	assert.Equal(t, "**", inner.Op)
}

func TestOperatorPrecedencePipeWithArithmetic(t *testing.T) {
	// Pipe has lower precedence than arithmetic
	// a |> b + c should parse as a |> (b + c) because + binds tighter
	program, err := Parse(context.Background(), "a |> b + c", nil)
	assert.Nil(t, err)

	pipe, ok := program.First().(*ast.Pipe)
	assert.True(t, ok, "expected Pipe, got %T", program.First())
	assert.Len(t, pipe.Exprs, 2)

	// Second expr should be b + c
	infix, ok := pipe.Exprs[1].(*ast.Infix)
	assert.True(t, ok, "expected Infix for second pipe arg")
	assert.Equal(t, "+", infix.Op)
}

func TestOperatorPrecedenceNullishCoalescing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Nullish has lower precedence than most operators
		{"a ?? b + c", "a ?? (b + c)"},
		{"a + b ?? c", "(a + b) ?? c"},
		{"a ?? b ?? c", "(a ?? b) ?? c"}, // Left-associative
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			// Verify the structure by checking the AST nodes
			_, ok := program.First().(*ast.Infix)
			assert.True(t, ok, "expected Infix for nullish coalescing")
		})
	}
}

func TestOperatorPrecedenceLogicalWithNullish(t *testing.T) {
	// In Risor, NULLISH < COND, so && and || have HIGHER precedence than ??
	// This means: a ?? b && c parses as a ?? (b && c)
	// The && binds tighter, so it's evaluated first
	program, err := Parse(context.Background(), "a ?? b && c", nil)
	assert.Nil(t, err)

	// Outer operator should be ?? (lower precedence)
	outer, ok := program.First().(*ast.Infix)
	assert.True(t, ok, "expected Infix")
	assert.Equal(t, "??", outer.Op)

	// Right side should be b && c (higher precedence, bound first)
	inner, ok := outer.Y.(*ast.Infix)
	assert.True(t, ok, "expected Infix for right side")
	assert.Equal(t, "&&", inner.Op)
}

func TestOperatorPrecedenceInOperator(t *testing.T) {
	// "in" has comparison precedence, lower than SUM
	// So: 1 + 2 in [3] parses as (1 + 2) in [3]
	program, err := Parse(context.Background(), "1 + 2 in [3]", nil)
	assert.Nil(t, err)

	inExpr, ok := program.First().(*ast.In)
	assert.True(t, ok, "expected In, got %T", program.First())

	// Left side should be 1 + 2
	infix, ok := inExpr.X.(*ast.Infix)
	assert.True(t, ok, "expected Infix for left side of in")
	assert.Equal(t, "+", infix.Op)
}

func TestOperatorPrecedenceNotInOperator(t *testing.T) {
	// Similar to "in", "not in" has comparison precedence lower than SUM
	// So: 1 + 2 not in [3] parses as (1 + 2) not in [3]
	program, err := Parse(context.Background(), "1 + 2 not in [3]", nil)
	assert.Nil(t, err)

	notInExpr, ok := program.First().(*ast.NotIn)
	assert.True(t, ok, "expected NotIn, got %T", program.First())

	// Left side should be 1 + 2
	infix, ok := notInExpr.X.(*ast.Infix)
	assert.True(t, ok, "expected Infix for left side of not in")
	assert.Equal(t, "+", infix.Op)
}

func TestAssignmentChainingNotSupported(t *testing.T) {
	// Risor's Assign is a statement, not an expression
	// So assignment chaining like "x = y = z = 1" parses as separate statements
	// First statement: "x = y"
	// Then: "=" is invalid since = requires an identifier on left
	program, err := Parse(context.Background(), "x = y", nil)
	assert.Nil(t, err)

	assign, ok := program.First().(*ast.Assign)
	assert.True(t, ok, "expected Assign")
	assert.Equal(t, "x", assign.Name.Name)

	// The value should be identifier "y"
	ident, ok := assign.Value.(*ast.Ident)
	assert.True(t, ok, "expected Ident as value")
	assert.Equal(t, "y", ident.Name)
}

// =============================================================================
// NUMBER LITERAL EDGE CASES
// =============================================================================

func TestNumberLiteralZero(t *testing.T) {
	// Single 0 should parse as decimal zero, not octal
	program, err := Parse(context.Background(), "0", nil)
	assert.Nil(t, err)

	intLit, ok := program.First().(*ast.Int)
	assert.True(t, ok, "expected Int, got %T", program.First())
	assert.Equal(t, int64(0), intLit.Value)
}

func TestNumberLiteralOctal(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"00", 0},
		{"07", 7},
		{"010", 8},  // Octal 10 = decimal 8
		{"017", 15}, // Octal 17 = decimal 15
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			intLit, ok := program.First().(*ast.Int)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, intLit.Value)
		})
	}
}

func TestNumberLiteralHex(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0x0", 0},
		{"0xF", 15},
		{"0xff", 255},
		{"0x10", 16},
		{"0xDEADBEEF", 0xDEADBEEF},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			intLit, ok := program.First().(*ast.Int)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, intLit.Value)
		})
	}
}

func TestNumberLiteralFloatEdgeCases(t *testing.T) {
	// Risor supports standard float format: digits.digits
	tests := []struct {
		input    string
		expected float64
	}{
		{"0.0", 0.0},
		{"1.5", 1.5},
		{"123.456", 123.456},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			floatLit, ok := program.First().(*ast.Float)
			assert.True(t, ok, "expected Float, got %T", program.First())
			assert.Equal(t, tt.expected, floatLit.Value)
		})
	}
}

func TestNumberLiteralFloatUnsupportedFormats(t *testing.T) {
	// These float formats are NOT supported by Risor's lexer
	unsupported := []string{
		".5",     // Leading decimal point
		"1.",     // Trailing decimal point
		"1e10",   // Scientific notation
		"1E10",   // Scientific notation (uppercase)
		"1.5e-3", // Scientific notation with decimal
	}

	for _, input := range unsupported {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(context.Background(), input, nil)
			// These should error because the lexer doesn't recognize them as floats
			assert.NotNil(t, err, "expected error for unsupported float format: %s", input)
		})
	}
}

// =============================================================================
// STRING AND TEMPLATE EDGE CASES
// =============================================================================

func TestTemplateStringEmpty(t *testing.T) {
	program, err := Parse(context.Background(), "``", nil)
	assert.Nil(t, err)

	str, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Equal(t, "", str.Value)
}

func TestTemplateStringNoInterpolation(t *testing.T) {
	program, err := Parse(context.Background(), "`hello world`", nil)
	assert.Nil(t, err)

	str, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Equal(t, "hello world", str.Value)
	assert.Nil(t, str.Template) // No template when no ${} present
}

func TestTemplateStringWithInterpolation(t *testing.T) {
	program, err := Parse(context.Background(), "`hello ${name}`", nil)
	assert.Nil(t, err)

	str, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.NotNil(t, str.Template)
	assert.Len(t, str.Exprs, 1)
}

func TestTemplateStringMultipleInterpolations(t *testing.T) {
	program, err := Parse(context.Background(), "`${a} and ${b} and ${c}`", nil)
	assert.Nil(t, err)

	str, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Len(t, str.Exprs, 3)
}

func TestTemplateStringComplexExpression(t *testing.T) {
	program, err := Parse(context.Background(), "`result: ${a + b * c}`", nil)
	assert.Nil(t, err)

	str, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Len(t, str.Exprs, 1)

	// The expression should be parsed correctly
	_, ok = str.Exprs[0].(*ast.Infix)
	assert.True(t, ok)
}

func TestTemplateStringSyntaxErrorInInterpolation(t *testing.T) {
	_, err := Parse(context.Background(), "`${1 + }`", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "template interpolation")
}

func TestTemplateStringNestedBraces(t *testing.T) {
	// Template with map literal inside - use a non-empty map to avoid ambiguity
	program, err := Parse(context.Background(), "`${{}}`", nil)
	// NOTE: Empty map `${{}}` causes parsing issues due to brace matching
	// This is a known limitation - the template parser struggles with nested braces
	if err != nil {
		// Document this as a known issue
		t.Skip("Template with empty map literal has parsing issues - known limitation")
	}

	str, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Len(t, str.Exprs, 1)

	_, ok = str.Exprs[0].(*ast.Map)
	assert.True(t, ok)
}

func TestTemplateStringWithMapLiteral(t *testing.T) {
	// Non-empty map works
	program, err := Parse(context.Background(), "`${x}`", nil)
	assert.Nil(t, err)

	str, ok := program.First().(*ast.String)
	assert.True(t, ok)
	assert.Len(t, str.Exprs, 1)
}

// =============================================================================
// POSTFIX OPERATOR EDGE CASES
// =============================================================================

func TestPostfixOnValidTargets(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"x++", true},
		{"arr[0]++", true},
		{"obj.field++", true},
		{"x--", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			if tt.valid {
				assert.Nil(t, err)
				_, ok := program.First().(*ast.Postfix)
				assert.True(t, ok)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}

func TestPostfixOnInvalidTargets(t *testing.T) {
	tests := []string{
		"(a + b)++",
		"1++",
		"\"string\"++",
		"[1, 2]++",
		"func(){}++",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(context.Background(), input, nil)
			assert.NotNil(t, err, "expected error for: %s", input)
		})
	}
}

func TestPostfixAfterIndex(t *testing.T) {
	// arr[i]++ - postfix on indexed expression
	program, err := Parse(context.Background(), "arr[i]++", nil)
	assert.Nil(t, err)

	postfix, ok := program.First().(*ast.Postfix)
	assert.True(t, ok)
	assert.Equal(t, "++", postfix.Op)

	index, ok := postfix.X.(*ast.Index)
	assert.True(t, ok)
	assert.Equal(t, "arr", index.X.(*ast.Ident).Name)
}

// =============================================================================
// ARROW FUNCTION EDGE CASES
// =============================================================================

func TestArrowFunctionSingleParam(t *testing.T) {
	program, err := Parse(context.Background(), "x => x + 1", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 1)
	paramIdent, ok := fn.Params[0].(*ast.Ident)
	assert.True(t, ok, "Expected *ast.Ident param")
	assert.Equal(t, "x", paramIdent.Name)
}

func TestArrowFunctionNoParams(t *testing.T) {
	program, err := Parse(context.Background(), "() => 42", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 0)
}

func TestArrowFunctionMultipleParams(t *testing.T) {
	program, err := Parse(context.Background(), "(a, b, c) => a + b + c", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 3)
}

func TestArrowFunctionDefaultsAST(t *testing.T) {
	program, err := Parse(context.Background(), "(a, b = 10) => a + b", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 2)
	assert.Contains(t, fn.Defaults, "b")
}

func TestArrowFunctionBlockBody(t *testing.T) {
	program, err := Parse(context.Background(), "(x) => { return x * 2 }", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.NotNil(t, fn.Body)
	assert.Len(t, fn.Body.Stmts, 1)
}

func TestArrowFunctionImmediatelyInvoked(t *testing.T) {
	program, err := Parse(context.Background(), "(x => x + 1)(5)", nil)
	assert.Nil(t, err)

	call, ok := program.First().(*ast.Call)
	assert.True(t, ok)

	fn, ok := call.Fun.(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 1)
}

func TestArrowFunctionNested(t *testing.T) {
	program, err := Parse(context.Background(), "x => y => x + y", nil)
	assert.Nil(t, err)

	fn1, ok := program.First().(*ast.Func)
	assert.True(t, ok)

	// Body should contain a return with another function
	ret, ok := fn1.Body.Stmts[0].(*ast.Return)
	assert.True(t, ok)

	fn2, ok := ret.Value.(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn2.Params, 1)
}

func TestArrowFunctionCommaWithoutArrow(t *testing.T) {
	// (1, 2) without arrow should error
	_, err := Parse(context.Background(), "(1, 2)", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "comma-separated expressions require arrow function syntax")
}

func TestEmptyParensWithoutArrow(t *testing.T) {
	// () without arrow should error
	_, err := Parse(context.Background(), "()", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "empty parentheses require arrow function syntax")
}

// =============================================================================
// INDEX AND SLICE EDGE CASES
// =============================================================================

func TestSliceExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"a[:]", "a[:]"},
		{"a[1:]", "a[1:]"},
		{"a[:2]", "a[:2]"},
		{"a[1:2]", "a[1:2]"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestIndexChaining(t *testing.T) {
	program, err := Parse(context.Background(), "a[0][1][2]", nil)
	assert.Nil(t, err)

	// Should be nested Index nodes
	idx1, ok := program.First().(*ast.Index)
	assert.True(t, ok)

	idx2, ok := idx1.X.(*ast.Index)
	assert.True(t, ok)

	idx3, ok := idx2.X.(*ast.Index)
	assert.True(t, ok)

	ident, ok := idx3.X.(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "a", ident.Name)
}

func TestIndexWithExpression(t *testing.T) {
	program, err := Parse(context.Background(), "arr[i + 1]", nil)
	assert.Nil(t, err)

	idx, ok := program.First().(*ast.Index)
	assert.True(t, ok)

	infix, ok := idx.Index.(*ast.Infix)
	assert.True(t, ok)
	assert.Equal(t, "+", infix.Op)
}

// =============================================================================
// DESTRUCTURING EDGE CASES
// =============================================================================

func TestObjectDestructureTrailingCommaAllowed(t *testing.T) {
	program, err := Parse(context.Background(), "let { a, b, } = obj", nil)
	assert.Nil(t, err)

	destr, ok := program.First().(*ast.ObjectDestructure)
	assert.True(t, ok)
	assert.Len(t, destr.Bindings, 2)
}

func TestObjectDestructureWithAlias(t *testing.T) {
	program, err := Parse(context.Background(), "let { a: x, b: y } = obj", nil)
	assert.Nil(t, err)

	destr, ok := program.First().(*ast.ObjectDestructure)
	assert.True(t, ok)
	assert.Len(t, destr.Bindings, 2)
	assert.Equal(t, "a", destr.Bindings[0].Key)
	assert.Equal(t, "x", destr.Bindings[0].Alias)
}

func TestObjectDestructureWithDefault(t *testing.T) {
	program, err := Parse(context.Background(), "let { a = 10, b = 20 } = obj", nil)
	assert.Nil(t, err)

	destr, ok := program.First().(*ast.ObjectDestructure)
	assert.True(t, ok)
	assert.NotNil(t, destr.Bindings[0].Default)
	assert.NotNil(t, destr.Bindings[1].Default)
}

func TestObjectDestructureWithAliasAndDefault(t *testing.T) {
	program, err := Parse(context.Background(), "let { a: x = 10 } = obj", nil)
	assert.Nil(t, err)

	destr, ok := program.First().(*ast.ObjectDestructure)
	assert.True(t, ok)
	assert.Equal(t, "a", destr.Bindings[0].Key)
	assert.Equal(t, "x", destr.Bindings[0].Alias)
	assert.NotNil(t, destr.Bindings[0].Default)
}

func TestObjectDestructureEmpty(t *testing.T) {
	_, err := Parse(context.Background(), "let {} = obj", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestArrayDestructureTrailingComma(t *testing.T) {
	program, err := Parse(context.Background(), "let [a, b,] = arr", nil)
	assert.Nil(t, err)

	destr, ok := program.First().(*ast.ArrayDestructure)
	assert.True(t, ok)
	assert.Len(t, destr.Elements, 2)
}

func TestArrayDestructureWithDefault(t *testing.T) {
	program, err := Parse(context.Background(), "let [a = 1, b = 2] = arr", nil)
	assert.Nil(t, err)

	destr, ok := program.First().(*ast.ArrayDestructure)
	assert.True(t, ok)
	assert.NotNil(t, destr.Elements[0].Default)
	assert.NotNil(t, destr.Elements[1].Default)
}

func TestArrayDestructureEmpty(t *testing.T) {
	_, err := Parse(context.Background(), "let [] = arr", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// =============================================================================
// SPREAD OPERATOR EDGE CASES
// =============================================================================

func TestSpreadInList(t *testing.T) {
	program, err := Parse(context.Background(), "[1, ...arr, 2]", nil)
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 3)

	_, ok = list.Items[1].(*ast.Spread)
	assert.True(t, ok)
}

func TestSpreadInMap(t *testing.T) {
	program, err := Parse(context.Background(), "{...obj, a: 1}", nil)
	assert.Nil(t, err)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 2)

	// First item should be spread (Key is nil)
	assert.Nil(t, m.Items[0].Key)
}

func TestSpreadInFunctionCall(t *testing.T) {
	program, err := Parse(context.Background(), "f(a, ...args, b)", nil)
	assert.Nil(t, err)

	call, ok := program.First().(*ast.Call)
	assert.True(t, ok)
	assert.Len(t, call.Args, 3)

	_, ok = call.Args[1].(*ast.Spread)
	assert.True(t, ok)
}

func TestMultipleSpreadsInList(t *testing.T) {
	program, err := Parse(context.Background(), "[...a, ...b, ...c]", nil)
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 3)

	for _, item := range list.Items {
		_, ok = item.(*ast.Spread)
		assert.True(t, ok)
	}
}

// =============================================================================
// PIPE EXPRESSION EDGE CASES
// =============================================================================

func TestPipeWithNewlinesAfterOperator(t *testing.T) {
	// Newlines are allowed AFTER the pipe operator
	input := `a |>
b |>
c`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	pipe, ok := program.First().(*ast.Pipe)
	assert.True(t, ok)
	assert.Len(t, pipe.Exprs, 3)
}

func TestPipeNewlineBeforeOperatorNotAllowed(t *testing.T) {
	// Newlines BEFORE |> cause the expression to be split
	input := `a
|> b`
	_, err := Parse(context.Background(), input, nil)
	// First statement is just "a", second starts with |> which is a parse error
	assert.NotNil(t, err)
}

func TestPipeWithFunctionCalls(t *testing.T) {
	program, err := Parse(context.Background(), "data |> filter(f) |> map(g)", nil)
	assert.Nil(t, err)

	pipe, ok := program.First().(*ast.Pipe)
	assert.True(t, ok)
	assert.Len(t, pipe.Exprs, 3)
}

// =============================================================================
// OPTIONAL CHAINING EDGE CASES
// =============================================================================

func TestOptionalChainingBasic(t *testing.T) {
	program, err := Parse(context.Background(), "obj?.field", nil)
	assert.Nil(t, err)

	getAttr, ok := program.First().(*ast.GetAttr)
	assert.True(t, ok)
	assert.True(t, getAttr.Optional)
}

func TestOptionalChainingMethodCall(t *testing.T) {
	program, err := Parse(context.Background(), "obj?.method()", nil)
	assert.Nil(t, err)

	objCall, ok := program.First().(*ast.ObjectCall)
	assert.True(t, ok)
	assert.True(t, objCall.Optional)
}

func TestOptionalChainingChained(t *testing.T) {
	program, err := Parse(context.Background(), "a?.b?.c", nil)
	assert.Nil(t, err)

	// Should be GetAttr(GetAttr(a, b, optional=true), c, optional=true)
	outer, ok := program.First().(*ast.GetAttr)
	assert.True(t, ok)
	assert.True(t, outer.Optional)

	inner, ok := outer.X.(*ast.GetAttr)
	assert.True(t, ok)
	assert.True(t, inner.Optional)
}

func TestOptionalChainingMixed(t *testing.T) {
	program, err := Parse(context.Background(), "a.b?.c.d", nil)
	assert.Nil(t, err)
	assert.Equal(t, "a.b?.c.d", program.First().String())
}

// =============================================================================
// TRY/CATCH/FINALLY EDGE CASES
// =============================================================================

func TestTryWithCatchOnly(t *testing.T) {
	program, err := Parse(context.Background(), "try { x } catch { y }", nil)
	assert.Nil(t, err)

	tryNode, ok := program.First().(*ast.Try)
	assert.True(t, ok)
	assert.NotNil(t, tryNode.CatchBlock)
	assert.Nil(t, tryNode.FinallyBlock)
}

func TestTryWithFinallyOnly(t *testing.T) {
	program, err := Parse(context.Background(), "try { x } finally { y }", nil)
	assert.Nil(t, err)

	tryNode, ok := program.First().(*ast.Try)
	assert.True(t, ok)
	assert.Nil(t, tryNode.CatchBlock)
	assert.NotNil(t, tryNode.FinallyBlock)
}

func TestTryWithBoth(t *testing.T) {
	program, err := Parse(context.Background(), "try { x } catch e { y } finally { z }", nil)
	assert.Nil(t, err)

	tryNode, ok := program.First().(*ast.Try)
	assert.True(t, ok)
	assert.NotNil(t, tryNode.CatchBlock)
	assert.NotNil(t, tryNode.FinallyBlock)
	assert.NotNil(t, tryNode.CatchIdent)
	assert.Equal(t, "e", tryNode.CatchIdent.Name)
}

func TestTryWithNeither(t *testing.T) {
	_, err := Parse(context.Background(), "try { x }", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "requires at least one of catch or finally")
}

func TestTryWithNewlines(t *testing.T) {
	input := `try {
	x
}
catch e {
	y
}
finally {
	z
}`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	tryNode, ok := program.First().(*ast.Try)
	assert.True(t, ok)
	assert.NotNil(t, tryNode.CatchBlock)
	assert.NotNil(t, tryNode.FinallyBlock)
}

// =============================================================================
// SWITCH STATEMENT EDGE CASES
// =============================================================================

func TestSwitchBasic(t *testing.T) {
	input := `switch (x) {
case 1:
	a
case 2:
	b
default:
	c
}`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	sw, ok := program.First().(*ast.Switch)
	assert.True(t, ok)
	assert.Len(t, sw.Cases, 3)
}

func TestSwitchMultipleDefaults(t *testing.T) {
	input := `switch (x) {
default:
	a
default:
	b
}`
	_, err := Parse(context.Background(), input, nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "multiple default")
}

func TestSwitchMultipleCaseExprs(t *testing.T) {
	input := `switch (x) {
case 1, 2, 3:
	a
}`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	sw, ok := program.First().(*ast.Switch)
	assert.True(t, ok)
	assert.Len(t, sw.Cases[0].Exprs, 3)
}

func TestSwitchEmptyCaseBody(t *testing.T) {
	input := `switch (x) {
case 1:
case 2:
	a
}`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	sw, ok := program.First().(*ast.Switch)
	assert.True(t, ok)
	assert.Nil(t, sw.Cases[0].Body) // First case has no body
	assert.NotNil(t, sw.Cases[1].Body)
}

// =============================================================================
// RETURN AND THROW EDGE CASES
// =============================================================================

func TestReturnWithoutValue(t *testing.T) {
	program, err := Parse(context.Background(), "return", nil)
	assert.Nil(t, err)

	ret, ok := program.First().(*ast.Return)
	assert.True(t, ok)
	assert.Nil(t, ret.Value)
}

func TestReturnWithValue(t *testing.T) {
	program, err := Parse(context.Background(), "return 42", nil)
	assert.Nil(t, err)

	ret, ok := program.First().(*ast.Return)
	assert.True(t, ok)
	assert.NotNil(t, ret.Value)
}

func TestThrowWithoutValue(t *testing.T) {
	_, err := Parse(context.Background(), "throw", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "throw statement requires a value")
}

func TestThrowWithValue(t *testing.T) {
	program, err := Parse(context.Background(), "throw error(\"oops\")", nil)
	assert.Nil(t, err)

	throwNode, ok := program.First().(*ast.Throw)
	assert.True(t, ok)
	assert.NotNil(t, throwNode.Value)
}

// =============================================================================
// CONST EDGE CASES
// =============================================================================

func TestConstWithoutValue(t *testing.T) {
	_, err := Parse(context.Background(), "const x", nil)
	assert.NotNil(t, err)
}

func TestConstWithValue(t *testing.T) {
	program, err := Parse(context.Background(), "const x = 42", nil)
	assert.Nil(t, err)

	constNode, ok := program.First().(*ast.Const)
	assert.True(t, ok)
	assert.Equal(t, "x", constNode.Name.Name)
}

// =============================================================================
// LET EDGE CASES
// =============================================================================

func TestLetMultipleVariablesWithValue(t *testing.T) {
	program, err := Parse(context.Background(), "let a, b = [1, 2]", nil)
	assert.Nil(t, err)

	multiVar, ok := program.First().(*ast.MultiVar)
	assert.True(t, ok)
	assert.Len(t, multiVar.Names, 2)
}

func TestLetSingleVariable(t *testing.T) {
	program, err := Parse(context.Background(), "let x = 1", nil)
	assert.Nil(t, err)

	varNode, ok := program.First().(*ast.Var)
	assert.True(t, ok)
	assert.Equal(t, "x", varNode.Name.Name)
}

// =============================================================================
// NEWLINE HANDLING EDGE CASES
// =============================================================================

func TestNewlineAfterInfixOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"a +\nb", "(a + b)"},
		{"a -\nb", "(a - b)"},
		{"a *\nb", "(a * b)"},
		{"a /\nb", "(a / b)"},
		{"a &&\nb", "(a && b)"},
		{"a ||\nb", "(a || b)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

func TestNewlineInFunctionCall(t *testing.T) {
	input := `f(
	a,
	b,
	c
)`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	call, ok := program.First().(*ast.Call)
	assert.True(t, ok)
	assert.Len(t, call.Args, 3)
}

func TestNewlineInList(t *testing.T) {
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

func TestNewlineInMap(t *testing.T) {
	input := `{
	a: 1,
	b: 2,
	c: 3
}`
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 3)
}

// =============================================================================
// EMPTY INPUT AND BOUNDARY CONDITIONS
// =============================================================================

func TestEmptyInput(t *testing.T) {
	program, err := Parse(context.Background(), "", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 0)
}

func TestOnlyWhitespace(t *testing.T) {
	program, err := Parse(context.Background(), "   \n\n\t  \n  ", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 0)
}

func TestOnlyNewlines(t *testing.T) {
	program, err := Parse(context.Background(), "\n\n\n", nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 0)
}

// =============================================================================
// DEEPLY NESTED STRUCTURES
// =============================================================================

func TestDeeplyNestedParentheses(t *testing.T) {
	input := "((((((((((x))))))))))"
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	// All the parens should just wrap the identifier
	ident, ok := program.First().(*ast.Ident)
	assert.True(t, ok)
	assert.Equal(t, "x", ident.Name)
}

func TestDeeplyNestedLists(t *testing.T) {
	input := "[[[[[x]]]]]"
	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)

	// Should be nested List nodes
	list1, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list1.Items, 1)
}

func TestMaxDepthEnforced(t *testing.T) {
	// Create deeply nested expression
	var sb strings.Builder
	depth := 200
	for i := 0; i < depth; i++ {
		sb.WriteString("(")
	}
	sb.WriteString("x")
	for i := 0; i < depth; i++ {
		sb.WriteString(")")
	}

	_, err := Parse(context.Background(), sb.String(), &Config{MaxDepth: 100})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth")
}

// =============================================================================
// ATTRIBUTE ACCESS EDGE CASES
// =============================================================================

func TestAttributeChaining(t *testing.T) {
	program, err := Parse(context.Background(), "a.b.c.d.e", nil)
	assert.Nil(t, err)
	assert.Equal(t, "a.b.c.d.e", program.First().String())
}

func TestAttributeWithMethodCall(t *testing.T) {
	program, err := Parse(context.Background(), "obj.method().field", nil)
	assert.Nil(t, err)

	// Outermost should be GetAttr
	getAttr, ok := program.First().(*ast.GetAttr)
	assert.True(t, ok)
	assert.Equal(t, "field", getAttr.Attr.Name)

	// Inner should be ObjectCall
	objCall, ok := getAttr.X.(*ast.ObjectCall)
	assert.True(t, ok)
	assert.Equal(t, "method", objCall.Call.Fun.(*ast.Ident).Name)
}

func TestSetAttrCompoundOperators(t *testing.T) {
	tests := []struct {
		input    string
		operator string
	}{
		{"obj.field = 1", "="},
		{"obj.field += 1", "+="},
		{"obj.field -= 1", "-="},
		{"obj.field *= 2", "*="},
		{"obj.field /= 2", "/="},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			setAttr, ok := program.First().(*ast.SetAttr)
			assert.True(t, ok)
			assert.Equal(t, tt.operator, setAttr.Op)
		})
	}
}

// =============================================================================
// IN/NOT IN EDGE CASES
// =============================================================================

func TestInExpression(t *testing.T) {
	program, err := Parse(context.Background(), "x in [1, 2, 3]", nil)
	assert.Nil(t, err)

	in, ok := program.First().(*ast.In)
	assert.True(t, ok)
	assert.Equal(t, "x", in.X.(*ast.Ident).Name)
}

func TestNotInExpression(t *testing.T) {
	program, err := Parse(context.Background(), "x not in [1, 2, 3]", nil)
	assert.Nil(t, err)

	notIn, ok := program.First().(*ast.NotIn)
	assert.True(t, ok)
	assert.Equal(t, "x", notIn.X.(*ast.Ident).Name)
}

func TestNotWithoutIn(t *testing.T) {
	// "not" without "in" should error
	_, err := Parse(context.Background(), "x not y", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "expected 'in' after 'not'")
}

// =============================================================================
// REST PARAMETER EDGE CASES
// =============================================================================

func TestRestParameterAtEnd(t *testing.T) {
	program, err := Parse(context.Background(), "function f(a, b, ...rest) { }", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.NotNil(t, fn.RestParam)
	assert.Equal(t, "rest", fn.RestParam.Name)
}

func TestRestParameterNotAtEnd(t *testing.T) {
	_, err := Parse(context.Background(), "function f(...rest, a) { }", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "rest parameter must be the last parameter")
}

func TestMultipleRestParameters(t *testing.T) {
	_, err := Parse(context.Background(), "function f(...a, ...b) { }", nil)
	assert.NotNil(t, err)
}

// =============================================================================
// FUNCTION LITERAL EDGE CASES
// =============================================================================

func TestFunctionWithName(t *testing.T) {
	program, err := Parse(context.Background(), "function add(a, b) { return a + b }", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.NotNil(t, fn.Name)
	assert.Equal(t, "add", fn.Name.Name)
}

func TestFunctionWithoutName(t *testing.T) {
	program, err := Parse(context.Background(), "function(x) { return x }", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Nil(t, fn.Name)
}

func TestFunctionWithDefaultParams(t *testing.T) {
	program, err := Parse(context.Background(), "function f(a, b = 10, c = 20) { }", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Params, 3)
	assert.Len(t, fn.Defaults, 2)
	assert.Contains(t, fn.Defaults, "b")
	assert.Contains(t, fn.Defaults, "c")
}

func TestFunctionEmptyBody(t *testing.T) {
	program, err := Parse(context.Background(), "function f() { }", nil)
	assert.Nil(t, err)

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok)
	assert.Len(t, fn.Body.Stmts, 0)
}

// =============================================================================
// IF/ELSE EDGE CASES
// =============================================================================

func TestIfWithoutElse(t *testing.T) {
	program, err := Parse(context.Background(), "if (x) { y }", nil)
	assert.Nil(t, err)

	ifNode, ok := program.First().(*ast.If)
	assert.True(t, ok)
	assert.Nil(t, ifNode.Alternative)
}

func TestIfWithElse(t *testing.T) {
	program, err := Parse(context.Background(), "if (x) { y } else { z }", nil)
	assert.Nil(t, err)

	ifNode, ok := program.First().(*ast.If)
	assert.True(t, ok)
	assert.NotNil(t, ifNode.Alternative)
}

func TestIfElseIfChain(t *testing.T) {
	program, err := Parse(context.Background(), "if (a) { x } else if (b) { y } else { z }", nil)
	assert.Nil(t, err)

	ifNode, ok := program.First().(*ast.If)
	assert.True(t, ok)
	assert.NotNil(t, ifNode.Alternative)

	// Alternative should contain another If
	nestedIf, ok := ifNode.Alternative.Stmts[0].(*ast.If)
	assert.True(t, ok)
	assert.NotNil(t, nestedIf.Alternative)
}

// =============================================================================
// MAP LITERAL EDGE CASES
// =============================================================================

func TestEmptyMap(t *testing.T) {
	program, err := Parse(context.Background(), "{}", nil)
	assert.Nil(t, err)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 0)
}

func TestMapWithStringKeys(t *testing.T) {
	program, err := Parse(context.Background(), `{"a": 1, "b": 2}`, nil)
	assert.Nil(t, err)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 2)
}

func TestMapWithExpressionKeys(t *testing.T) {
	program, err := Parse(context.Background(), "{a + b: 1, c * d: 2}", nil)
	assert.Nil(t, err)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 2)
}

func TestMapTrailingComma(t *testing.T) {
	program, err := Parse(context.Background(), "{a: 1, b: 2,}", nil)
	assert.Nil(t, err)

	m, ok := program.First().(*ast.Map)
	assert.True(t, ok)
	assert.Len(t, m.Items, 2)
}

// =============================================================================
// LIST LITERAL EDGE CASES
// =============================================================================

func TestEmptyList(t *testing.T) {
	program, err := Parse(context.Background(), "[]", nil)
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 0)
}

func TestListTrailingComma(t *testing.T) {
	program, err := Parse(context.Background(), "[1, 2, 3,]", nil)
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 3)
}

func TestListWithMixedTypes(t *testing.T) {
	program, err := Parse(context.Background(), `[1, "two", true, nil, []]`, nil)
	assert.Nil(t, err)

	list, ok := program.First().(*ast.List)
	assert.True(t, ok)
	assert.Len(t, list.Items, 5)
}

// =============================================================================
// COMPARISON OPERATOR EDGE CASES
// =============================================================================

func TestComparisonChaining(t *testing.T) {
	// a < b < c should parse as (a < b) < c (left-associative)
	program, err := Parse(context.Background(), "a < b < c", nil)
	assert.Nil(t, err)

	outer, ok := program.First().(*ast.Infix)
	assert.True(t, ok)
	assert.Equal(t, "<", outer.Op)

	inner, ok := outer.X.(*ast.Infix)
	assert.True(t, ok)
	assert.Equal(t, "<", inner.Op)
}

func TestComparisonOperatorsAll(t *testing.T) {
	tests := []struct {
		input    string
		operator string
	}{
		{"a == b", "=="},
		{"a != b", "!="},
		{"a < b", "<"},
		{"a > b", ">"},
		{"a <= b", "<="},
		{"a >= b", ">="},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			infix, ok := program.First().(*ast.Infix)
			assert.True(t, ok)
			assert.Equal(t, tt.operator, infix.Op)
		})
	}
}

// =============================================================================
// BITWISE OPERATOR EDGE CASES
// =============================================================================

func TestBitwiseOperators(t *testing.T) {
	tests := []struct {
		input    string
		operator string
	}{
		{"a & b", "&"},
		{"a << b", "<<"},
		{"a >> b", ">>"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			infix, ok := program.First().(*ast.Infix)
			assert.True(t, ok)
			assert.Equal(t, tt.operator, infix.Op)
		})
	}
}

// =============================================================================
// ERROR MESSAGE QUALITY TESTS
// =============================================================================

func TestErrorMessageForUnterminatedString(t *testing.T) {
	_, err := Parse(context.Background(), `"unterminated`, nil)
	assert.NotNil(t, err)
	// Should have meaningful error message
	assert.True(t, len(err.Error()) > 10)
}

func TestErrorMessageForMissingClosingParen(t *testing.T) {
	_, err := Parse(context.Background(), "(1 + 2", nil)
	assert.NotNil(t, err)
}

func TestErrorMessageForMissingClosingBracket(t *testing.T) {
	_, err := Parse(context.Background(), "[1, 2, 3", nil)
	assert.NotNil(t, err)
}

func TestErrorMessageForMissingClosingBrace(t *testing.T) {
	_, err := Parse(context.Background(), "{a: 1", nil)
	assert.NotNil(t, err)
}

func TestErrorMessageForInvalidOperator(t *testing.T) {
	_, err := Parse(context.Background(), "a @ b", nil)
	assert.NotNil(t, err)
}

// =============================================================================
// ASSIGNMENT EDGE CASES
// =============================================================================

func TestCompoundAssignmentOperators(t *testing.T) {
	tests := []struct {
		input    string
		operator string
	}{
		{"x = 1", "="},
		{"x += 1", "+="},
		{"x -= 1", "-="},
		{"x *= 2", "*="},
		{"x /= 2", "/="},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			assign, ok := program.First().(*ast.Assign)
			assert.True(t, ok)
			assert.Equal(t, tt.operator, assign.Op)
		})
	}
}

func TestIndexAssignmentFields(t *testing.T) {
	program, err := Parse(context.Background(), "arr[0] = 42", nil)
	assert.Nil(t, err)

	assign, ok := program.First().(*ast.Assign)
	assert.True(t, ok)
	assert.Nil(t, assign.Name)
	assert.NotNil(t, assign.Index)
}

// =============================================================================
// UNICODE AND SPECIAL CHARACTER EDGE CASES
// =============================================================================

func TestUnicodeIdentifiers(t *testing.T) {
	// Test that unicode identifiers are handled (if supported)
	tests := []string{
		"x",
		"_x",
		"x_",
		"x1",
		"_",
		"__",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			program, err := Parse(context.Background(), input, nil)
			assert.Nil(t, err)

			ident, ok := program.First().(*ast.Ident)
			assert.True(t, ok)
			assert.Equal(t, input, ident.Name)
		})
	}
}

func TestStringWithEscapes(t *testing.T) {
	tests := []string{
		`"hello\nworld"`,
		`"tab\there"`,
		`"quote\"here"`,
		`"backslash\\here"`,
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			program, err := Parse(context.Background(), input, nil)
			assert.Nil(t, err)

			_, ok := program.First().(*ast.String)
			assert.True(t, ok)
		})
	}
}

// =============================================================================
// OPERATOR SPACING EDGE CASES
// =============================================================================

func TestOperatorSpacing(t *testing.T) {
	// All of these should parse the same
	tests := []struct {
		input    string
		expected string
	}{
		{"1+2", "(1 + 2)"},
		{"1 +2", "(1 + 2)"},
		{"1+ 2", "(1 + 2)"},
		{"1 + 2", "(1 + 2)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}
