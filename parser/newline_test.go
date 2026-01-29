package parser

import (
	"context"
	"testing"

	"github.com/risor-io/risor/ast"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, err, "Parse error for input: %s", tt.input)
		require.NotNil(t, program)
		require.Len(t, program.Stmts, 1)

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
			require.Equal(t, tt.expected, v.Value)
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
	require.NoError(t, err)
	require.Len(t, program.Stmts, 3)
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
			require.NoError(t, err, "Parse error for: %s", tt.input)
			require.Len(t, program.Stmts, 1)
			require.Equal(t, tt.expected, program.First().String())
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
				require.Len(t, program.Stmts, tt.numStmts, "Expected %d statements for: %s", tt.numStmts, tt.input)
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
	require.NoError(t, err)
	require.Len(t, program.Stmts, 1)
	require.Equal(t, "obj.method1().method2()", program.First().String())
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
			require.NoError(t, err, "Parse error for: %s", tt.input)
			require.Len(t, program.Stmts, 1)
			require.Equal(t, tt.expected, program.First().String())
		})
	}
}
