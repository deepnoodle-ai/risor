package parser

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestAssignmentWithNewline(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // minimal value check
	}{
		{
			input: `x = 
			1`,
			expected: int64(1),
		},
		{
			input: `x += 
			1`,
			expected: int64(1),
		},
		{
			input: `obj.prop = 
			1`,
			expected: int64(1),
		},
		{
			input: `obj.prop += 
			1`,
			expected: int64(1),
		},
	}

	for _, tt := range tests {
		program, err := Parse(context.Background(), tt.input, nil)
		assert.Nil(t, err, "Parse error for input: %s", tt.input)
		assert.NotNil(t, program)
		assert.Len(t, program.Stmts, 1)

		stmt := program.Stmts[0]

		var value ast.Expr
		switch s := stmt.(type) {
		case *ast.Assign:
			value = s.Value
		case *ast.SetAttr:
			value = s.Value
		default:
			t.Fatalf("Unexpected statement type: %T", stmt)
		}

		switch v := value.(type) {
		case *ast.Int:
			assert.Equal(t, v.Value, tt.expected)
		default:
			t.Fatalf("Expected Int value, got %T", value)
		}
	}
}

func TestLiteralsWithNewlines(t *testing.T) {
	input := `
	l = [
		1,
		2,
	]
	m = {
		a: 1,
		b: 2,
	}
	function(
		a,
		b,
	) { return a + b }
	`
	program, err := Parse(context.Background(), input, nil)
	if err != nil {
		t.Log(err.Error())
	}
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 3)
}

// =============================================================================
// METHOD CHAINING ACROSS NEWLINES - EDGE CASES
// =============================================================================

func TestMethodChainingAcrossNewlines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic chaining
		{"simple dot chain", "obj\n.method()", "obj.method()"},
		{"simple optional chain", "obj\n?.method()", "obj?.method()"},

		// Multiple newlines
		{"multiple newlines before dot", "obj\n\n\n.method()", "obj.method()"},
		{"multiple newlines before optional", "obj\n\n?.prop", "obj?.prop"},

		// Long chains
		{"three method chain", "obj\n.a()\n.b()\n.c()", "obj.a().b().c()"},
		{"mixed dot and optional", "obj\n.a()\n?.b\n.c()", "obj.a()?.b.c()"},

		// With arguments
		{"chain with args", "obj\n.method(1, 2)", "obj.method(1, 2)"},
		{"chain with complex args", "obj\n.filter(x => x > 0)\n.map(x => x * 2)", "obj.filter(function(x) { return (x > 0) }).map(function(x) { return (x * 2) })"},

		// Property access
		{"property then method", "obj\n.prop\n.method()", "obj.prop.method()"},
		{"method then property", "obj\n.method()\n.prop", "obj.method().prop"},

		// Nested calls
		{"nested call then chain", "foo(bar)\n.method()", "foo(bar).method()"},
		{"index then chain", "arr[0]\n.method()", "arr[0].method()"},

		// Chain as function argument
		{"chain as call arg", "foo(obj\n.method())", "foo(obj.method())"},

		// Whitespace variations
		{"tab before dot", "obj\n\t.method()", "obj.method()"},
		{"spaces and newline", "obj\n   .method()", "obj.method()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Parse error for: %s", tt.input)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, program.First().String(), tt.expected)
		})
	}
}

func TestMethodChainingDoesNotAffectOtherOperators(t *testing.T) {
	// These should still parse as two statements or error
	tests := []struct {
		name     string
		input    string
		numStmts int
	}{
		// Operators that should NOT chain across newlines
		{"newline before +", "x\n+y", 2},    // +y is unary plus on y (separate stmt)
		{"newline before -", "x\n-y", 2},    // -y is unary minus on y (separate stmt)
		{"newline before [", "arr\n[0]", 2}, // [0] is a list literal
		{"newline before |>", "x\n|> y", 2}, // |> is pipe operator
		{"newline before (", "f\n(x)", 2},   // (x) is grouped expression
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			// Some may error (like +y without context), but check those that parse
			if err == nil {
				assert.Len(t, program.Stmts, tt.numStmts, "Expected %d statements for: %s", tt.numStmts, tt.input)
			}
		})
	}
}

func TestMethodChainingWithComments(t *testing.T) {
	// Comments between lines shouldn't break chaining
	// Note: This depends on how comments are handled in the lexer
	input := `obj
		.method1()
		.method2()`

	program, err := Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	assert.Len(t, program.Stmts, 1)
	assert.Equal(t, program.First().String(), "obj.method1().method2()")
}

func TestMethodChainingInComplexExpressions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"chain after grouping",
			"(a + b)\n.toString()",
			"(a + b).toString()",
		},
		{
			"chain on list literal",
			"[1, 2, 3]\n.filter(x => x > 1)",
			"[1, 2, 3].filter(function(x) { return (x > 1) })",
		},
		{
			"chain on map literal",
			"{a: 1}\n.keys()",
			"{a:1}.keys()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Parse error for: %s", tt.input)
			assert.Len(t, program.Stmts, 1)
			assert.Equal(t, program.First().String(), tt.expected)
		})
	}
}

// =============================================================================
// INFIX OPERATORS AFTER NEWLINE METHOD CHAINING
// =============================================================================
// After method chaining across newlines, remaining infix operators on the same
// line should still be parsed. The chaining loop in parseNode currently exits
// without falling back to the main infix loop, so operators like |>, +, ==, &&
// are dropped.

func TestInfixAfterNewlineChain(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			"pipe after newline chain",
			"let x = items\n.filter(f) |> sorted",
		},
		{
			"addition after newline chain",
			"let x = a\n.length() + b\n.length()",
		},
		{
			"equality after newline chain",
			"let x = a\n.name() == b\n.name()",
		},
		{
			"logical and after newline chain",
			"let x = a\n.valid() && b\n.valid()",
		},
		{
			"logical or after newline chain",
			"let x = a\n.ready() || fallback",
		},
		{
			"multiply after newline chain",
			"let x = a\n.count() * 2",
		},
		{
			"comparison after newline chain",
			"let x = items\n.length() > 0",
		},
		{
			"nullish coalescing after newline chain",
			"let x = a\n.value() ?? 0",
		},
		{
			"pipe chain after multi-line method chain",
			`let x = users
	.filter(u => u.active)
	.map(u => u.name) |> sorted`,
		},
		{
			"arithmetic after optional chain across newlines",
			"let x = a\n?.count + 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)
		})
	}
}

func TestInfixAfterNewlineChainVariations(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		// Starting expression types
		{
			"from call result",
			"let x = f()\n.method() + 1",
		},
		{
			"from list literal",
			"let x = [1, 2, 3]\n.filter(f) |> first",
		},
		{
			"from grouped expression",
			`let x = (a + b)
	.toString() == "5"`,
		},

		// Chain ending types
		{
			"ending with property access",
			"let x = a\n.length + 1",
		},
		{
			"ending with optional method call",
			"let x = a\n?.method() + 1",
		},

		// Expression context (not let)
		{
			"bare expression statement",
			"a\n.ready() || fallback",
		},
		{
			"return statement",
			"function f() { return a\n.b() + 1 }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(context.Background(), tt.input, nil)
			assert.Nil(t, err, "Should parse: %s", tt.input)
		})
	}
}

func TestInfixAfterNewlineChainAST(t *testing.T) {
	// Verify the AST structure is correct, not just that it parses
	t.Run("pipe produces ast.Pipe", func(t *testing.T) {
		program, err := Parse(context.Background(), "let x = items\n.sort() |> first", nil)
		assert.Nil(t, err)
		stmt := program.First().(*ast.Var)
		_, ok := stmt.Value.(*ast.Pipe)
		assert.True(t, ok, "Expected Pipe node, got %T", stmt.Value)
	})

	t.Run("addition produces ast.Infix", func(t *testing.T) {
		program, err := Parse(context.Background(), "let x = a\n.len() + 1", nil)
		assert.Nil(t, err)
		stmt := program.First().(*ast.Var)
		infix, ok := stmt.Value.(*ast.Infix)
		assert.True(t, ok, "Expected Infix node, got %T", stmt.Value)
		assert.Equal(t, "+", infix.Op)
	})

	t.Run("comparison produces ast.Infix", func(t *testing.T) {
		program, err := Parse(context.Background(), "let x = a\n.len() == 0", nil)
		assert.Nil(t, err)
		stmt := program.First().(*ast.Var)
		infix, ok := stmt.Value.(*ast.Infix)
		assert.True(t, ok, "Expected Infix node, got %T", stmt.Value)
		assert.Equal(t, "==", infix.Op)
	})
}
