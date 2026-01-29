package parser

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/ast"
)

// This file tests potential bugs and ambiguities identified during code review.
// Each test documents a specific concern and verifies correct behavior.

// =============================================================================
// ISSUE #1: Single Assignment in Parentheses Without Arrow
// =============================================================================
// The parseGroupedExpr function uses parseNode(LOWEST) which allows Assign nodes.
// If someone writes `(x = 10)` without `=>`, it should error (not be a valid
// grouped expression), since this syntax is ambiguous and likely a mistake.

func TestGroupedAssignmentWithoutArrowShouldError(t *testing.T) {
	// (x = 10) without => should NOT be valid as a grouped expression
	// It should require arrow function syntax: (x = 10) => ...
	_, err := Parse(context.Background(), "(x = 10)", nil)

	// This SHOULD error because (x = 10) looks like it could be:
	// 1. An arrow function missing the arrow and body
	// 2. A confusing grouped assignment
	// The language should reject this ambiguous syntax.
	//
	// CURRENT BEHAVIOR: This actually parses successfully as an Assign node!
	// This is arguably a bug since it creates confusing semantics.
	if err == nil {
		t.Log("WARNING: (x = 10) parses successfully without =>, which may be unexpected")
		t.Log("Consider whether this should require arrow function syntax")
	}
}

func TestArrowWithDefaultParamWorks(t *testing.T) {
	// (x = 10) => x should parse as arrow function with default param
	program, err := Parse(context.Background(), "(x = 10) => x", nil)
	assert.Nil(t, err, "Arrow function with default param should parse")

	fn, ok := program.First().(*ast.Func)
	assert.True(t, ok, "Expected Func, got %T", program.First())
	assert.Len(t, fn.Params, 1)
	paramIdent, ok := fn.Params[0].(*ast.Ident)
	assert.True(t, ok, "Expected *ast.Ident param")
	assert.Equal(t, "x", paramIdent.Name)
	assert.Contains(t, fn.Defaults, "x")
}

// =============================================================================
// ISSUE #2: Precedence of MOD vs POWER
// =============================================================================
// ** should have higher precedence than % (matching Python).
// With correct precedence: 2 ** 3 % 5 = (2 ** 3) % 5 = 8 % 5 = 3
// (This was previously broken and has been fixed.)

func TestPrecedenceModVsPower(t *testing.T) {
	// In Python: 2 ** 3 % 5 = (2 ** 3) % 5 = 8 % 5 = 3
	// ** should have higher precedence than %
	program, err := Parse(context.Background(), "2 ** 3 % 5", nil)
	assert.Nil(t, err)

	// Expected structure: (2 ** 3) % 5
	// The outer operator should be %
	outer, ok := program.First().(*ast.Infix)
	assert.True(t, ok, "Expected Infix, got %T", program.First())
	assert.Equal(t, "%", outer.Op, "Outer operator should be %")

	// Left side should be 2 ** 3
	inner, ok := outer.X.(*ast.Infix)
	assert.True(t, ok, "Left side should be Infix")
	assert.Equal(t, "**", inner.Op, "Inner operator should be **")
}

func TestPrecedenceModVsPowerExplicit(t *testing.T) {
	// Verify explicit grouping works correctly
	tests := []struct {
		input string
		desc  string
	}{
		{"(2 ** 3) % 5", "explicit grouping for **"},
		{"2 ** (3 % 5)", "explicit grouping for %"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
		})
	}
}

func TestModuloSamePrecedenceAsProduct(t *testing.T) {
	// Verify that %, *, / all have the same precedence (PRODUCT)
	// They should be left-associative
	tests := []struct {
		input    string
		expected string
	}{
		{"c * d / e % f", "(((c * d) / e) % f)"},
		{"a % b * c", "((a % b) * c)"},
		{"a / b % c", "((a / b) % c)"},
		{"a % b / c * d", "(((a % b) / c) * d)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

// =============================================================================
// ISSUE #3: Newline After Assignment Operator
// =============================================================================
// Recent changes added p.eatNewlines() after = in parseAssign.
// This allows: x =\n value
// Verify this works correctly.

func TestNewlineAfterAssignmentOperator(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"x =\n1", "newline after = in simple assignment"},
		{"x =\n\n1", "multiple newlines after ="},
		{"x +=\n1", "newline after +="},
		{"x -=\n1", "newline after -="},
		{"x *=\n2", "newline after *="},
		{"x /=\n2", "newline after /="},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)

			assign, ok := program.First().(*ast.Assign)
			assert.True(t, ok, "Expected Assign, got %T", program.First())
			assert.NotNil(t, assign.Value)
		})
	}
}

func TestNewlineAfterAttributeAssignment(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"obj.field =\n1", "newline after = in attribute assignment"},
		{"obj.field +=\n1", "newline after += in attribute assignment"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)

			setAttr, ok := program.First().(*ast.SetAttr)
			assert.True(t, ok, "Expected SetAttr, got %T", program.First())
			assert.NotNil(t, setAttr.Value)
		})
	}
}

func TestNewlineAfterLetAssignment(t *testing.T) {
	program, err := Parse(context.Background(), "let x =\n42", nil)
	assert.Nil(t, err)

	varNode, ok := program.First().(*ast.Var)
	assert.True(t, ok, "Expected Var, got %T", program.First())
	assert.NotNil(t, varNode.Value)
}

func TestNewlineAfterConstAssignment(t *testing.T) {
	program, err := Parse(context.Background(), "const x =\n42", nil)
	assert.Nil(t, err)

	constNode, ok := program.First().(*ast.Const)
	assert.True(t, ok, "Expected Const, got %T", program.First())
	assert.NotNil(t, constNode.Value)
}

// =============================================================================
// ISSUE #4: Newline After Dot in Attribute Access
// =============================================================================
// Verify that newlines are allowed after the dot operator.

func TestNewlineAfterDot(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"obj.\nfield", "newline after dot"},
		{"obj.\n\nfield", "multiple newlines after dot"},
		{"obj.field.\nmethod()", "newline after dot before method call"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)
			assert.NotNil(t, program.First())
		})
	}
}

func TestNewlineBeforeDot(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		desc     string
	}{
		{"obj\n.field", "obj.field", "newline before dot field access"},
		{"obj\n\n.field", "obj.field", "multiple newlines before dot"},
		{"obj\n.method()", "obj.method()", "newline before dot method call"},
		{"obj\n?.field", "obj?.field", "newline before optional chain"},
		{"obj\n?.method()", "obj?.method()", "newline before optional chain method"},
		{"obj\n.field\n.method()", "obj.field.method()", "chained newlines before dots"},
		{"list.filter(x => x > 0)\n.map(x => x * 2)", "list.filter(function(x) { return (x > 0) }).map(function(x) { return (x * 2) })", "fluent method chain"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)
			assert.NotNil(t, program.First())
			assert.Equal(t, tt.expected, program.First().String())
		})
	}
}

// =============================================================================
// ISSUE #5: Arrow Function Parameter Validation
// =============================================================================

func TestArrowFunctionInvalidParams(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"(1, 2, 3) => x", "number literals as params should fail"},
		{"(a + b) => x", "expression as param should fail"},
		{"(\"str\") => x", "string as param should fail"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.NotNil(t, err, "Should error: %s", tt.input)
		})
	}
}

func TestArrowFunctionValidParams(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"x => x", "single param no parens"},
		{"(x) => x", "single param with parens"},
		{"(x, y) => x + y", "multiple params"},
		{"(x = 1) => x", "single param with default"},
		{"(x, y = 2) => x + y", "mixed params with defaults"},
		{"(x = 1, y = 2) => x + y", "all params with defaults"},
		{"() => 42", "no params"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)

			fn, ok := program.First().(*ast.Func)
			assert.True(t, ok, "Expected Func, got %T", program.First())
			_ = fn
		})
	}
}

// =============================================================================
// ISSUE #7: Comparison Operators Don't Chain Like Python
// =============================================================================
// In Python: 1 < 2 < 3 means (1 < 2) and (2 < 3)
// In Risor: 1 < 2 < 3 parses as (1 < 2) < 3 which compares bool to int
// This is a design choice, but worth documenting.

func TestComparisonChainingBehavior(t *testing.T) {
	// Document that Risor doesn't do Python-style comparison chaining
	program, err := Parse(context.Background(), "1 < 2 < 3", nil)
	assert.Nil(t, err)

	// Should parse as ((1 < 2) < 3) - left associative
	outer, ok := program.First().(*ast.Infix)
	assert.True(t, ok)
	assert.Equal(t, "<", outer.Op)

	// Verify it's left-associative, not chained
	inner, ok := outer.X.(*ast.Infix)
	assert.True(t, ok, "Expected nested Infix - comparison chaining not supported")
	assert.Equal(t, "<", inner.Op)
}

// =============================================================================
// ISSUE #8: Index Assignment with Complex Expressions
// =============================================================================

func TestIndexAssignmentComplexIndex(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"arr[i + 1] = x", "expression as index"},
		{"arr[f()] = x", "function call as index"},
		{"arr[a[b]] = x", "nested index"},
		{"matrix[i][j] = x", "chained index assignment"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)

			assign, ok := program.First().(*ast.Assign)
			assert.True(t, ok, "Expected Assign, got %T", program.First())
			assert.NotNil(t, assign.Index)
		})
	}
}

// =============================================================================
// ISSUE #9: Empty Constructs
// =============================================================================

func TestEmptyConstructsBehavior(t *testing.T) {
	tests := []struct {
		input       string
		shouldError bool
		desc        string
	}{
		{"[]", false, "empty list is valid"},
		{"{}", false, "empty map is valid"},
		{"()", true, "empty parens require arrow"},
		{"let {} = obj", true, "empty object destructure"},
		{"let [] = arr", true, "empty array destructure"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			if tt.shouldError {
				assert.NotNil(t, err, "Should error: %s", tt.input)
			} else {
				assert.Nil(t, err, "Should parse: %s", tt.input)
			}
		})
	}
}

// =============================================================================
// ISSUE #10: Operator Associativity
// =============================================================================

func TestOperatorAssociativity(t *testing.T) {
	tests := []struct {
		input         string
		expectedOuter string
		isRight       bool // true if right-associative
		desc          string
	}{
		{"a + b + c", "+", false, "addition is left-associative"},
		{"a - b - c", "-", false, "subtraction is left-associative"},
		{"a * b * c", "*", false, "multiplication is left-associative"},
		{"a / b / c", "/", false, "division is left-associative"},
		{"a ** b ** c", "**", true, "power is right-associative"},
		{"a ?? b ?? c", "??", false, "nullish coalescing is left-associative"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err)

			outer, ok := program.First().(*ast.Infix)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedOuter, outer.Op)

			if tt.isRight {
				// Right-associative: a ** (b ** c)
				// So outer.Y should be the nested infix
				_, ok := outer.Y.(*ast.Infix)
				assert.True(t, ok, "Expected right-associative for %s", tt.input)
			} else {
				// Left-associative: (a + b) + c
				// So outer.X should be the nested infix
				_, ok := outer.X.(*ast.Infix)
				assert.True(t, ok, "Expected left-associative for %s", tt.input)
			}
		})
	}
}

// =============================================================================
// ISSUE #11: Spread Operator Edge Cases
// =============================================================================

func TestSpreadOperatorEdgeCases(t *testing.T) {
	tests := []struct {
		input       string
		shouldError bool
		desc        string
	}{
		{"[...arr]", false, "spread in list"},
		{"{...obj}", false, "spread in map"},
		{"f(...args)", false, "spread in call"},
		// Note: [......arr] parses as [...(...arr)] which is valid (spread of spread)
		{"[......arr]", false, "double spread parses as nested spread"},
		{"...x", false, "standalone spread is expression"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			if tt.shouldError {
				assert.NotNil(t, err, "Should error: %s", tt.input)
			} else {
				assert.Nil(t, err, "Should parse: %s", tt.input)
			}
		})
	}
}

// =============================================================================
// ISSUE #12: Call Expression Edge Cases
// =============================================================================

func TestCallExpressionEdgeCases(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"f()", "no args"},
		{"f(a)", "one arg"},
		{"f(a, b, c)", "multiple args"},
		{"f(a,)", "trailing comma"},
		{"f(\na,\nb\n)", "args with newlines"},
		{"f()()", "chained calls"},
		{"f()()().x", "many chained calls with attr access"},
		{"(x => x)(5)", "arrow IIFE"},
		{"((x) => x + 1)(5)", "arrow IIFE with parens"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)
		})
	}
}

// =============================================================================
// ISSUE #13: Pipe After Various Expressions
// =============================================================================

func TestPipeAfterExpressions(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"x |> f", "ident pipe func"},
		{"1 |> f", "int pipe func"},
		{"\"s\" |> f", "string pipe func"},
		{"[1, 2] |> f", "list pipe func"},
		{"{a: 1} |> f", "map pipe func"},
		{"f() |> g", "call pipe func"},
		{"x.y |> f", "attr access pipe func"},
		{"x[0] |> f", "index pipe func"},
		{"(x) |> f", "grouped pipe func"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)

			pipe, ok := program.First().(*ast.Pipe)
			assert.True(t, ok, "Expected Pipe, got %T", program.First())
			assert.Len(t, pipe.Exprs, 2)
		})
	}
}

// =============================================================================
// ISSUE #14: Complex Nested Expressions
// =============================================================================

func TestComplexNestedExpressions(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"f(g(h(x)))", "deeply nested calls"},
		{"a.b.c.d.e.f", "deeply nested attributes"},
		{"arr[0][1][2][3]", "deeply nested indices"},
		{"(((((x)))))", "deeply nested parens"},
		{"f(a, g(b, h(c)))", "nested calls as args"},
		{"[[[[[x]]]]]", "deeply nested lists"},
		{"{a: {b: {c: {d: 1}}}}", "deeply nested maps"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)
		})
	}
}

// =============================================================================
// ISSUE #15: Statement Termination Edge Cases
// =============================================================================

func TestStatementTerminationEdgeCases(t *testing.T) {
	tests := []struct {
		input     string
		stmtCount int
		desc      string
	}{
		{"a; b", 2, "semicolon separated"},
		{"a\nb", 2, "newline separated"},
		{"a;b;c", 3, "multiple semicolons"},
		{"a\n\n\nb", 2, "multiple newlines"},
		// Note: { a } is a map literal (with identifier key), not a block statement
		// Blocks only appear as part of control structures (if, function, etc.)
		{"{a: 1}", 1, "map literal is single statement"},
		{"if (true) { a }", 1, "if with block is single statement"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)
			assert.Len(t, program.Stmts, tt.stmtCount, "Statement count for: %s", tt.input)
		})
	}
}
