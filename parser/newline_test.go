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
		program, err := Parse(context.Background(), tt.input)
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
	program, err := Parse(context.Background(), input)
	if err != nil {
		t.Log(err.Error())
	}
	require.NoError(t, err)
	require.Len(t, program.Stmts, 3)
}
