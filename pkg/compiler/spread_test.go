package compiler

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/internal/token"
	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

func parseAndCompile(t *testing.T, input string) (*Code, error) {
	t.Helper()
	prog, err := parser.Parse(context.Background(), input, nil)
	assert.Nil(t, err)
	c, err := New(nil)
	assert.Nil(t, err)
	return c.CompileAST(prog)
}

func hasOpcode(code *Code, want op.Code) bool {
	for i := 0; i < code.InstructionCount(); i++ {
		if op.Code(code.Instruction(i)) == want {
			return true
		}
	}
	return false
}

func TestObjectCallSpread(t *testing.T) {
	inputs := []string{
		"let o = {}; o.add(...[1, 2])",
		"let o = {}; o.add(1, ...[2])",
		"let o = {}; o?.add(...[1, 2])",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			code, err := parseAndCompile(t, input)
			assert.Nil(t, err)
			assert.True(t, hasOpcode(code, op.CallSpread))
		})
	}
}

func TestObjectCallSpreadInPipe(t *testing.T) {
	_, err := parseAndCompile(t, "let o = {}; let x = 1; x |> o.add(...[1])")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "spread arguments not supported in pipe expressions")
}

// AST nodes that are only valid inside specific constructs must produce a
// compile error, not a panic, if they reach the compiler on their own.
func TestUnknownNodeCompileError(t *testing.T) {
	pos := token.Position{Line: 0, Column: 0}
	x := &ast.Ident{NamePos: pos, Name: "x"}
	nodes := []ast.Node{
		&ast.Spread{Ellipsis: pos, X: x},
		&ast.DefaultValue{Name: x, Default: &ast.Int{ValuePos: pos, Literal: "1", Value: 1}},
		&ast.List{Lbrack: pos, Items: []ast.Expr{
			&ast.DefaultValue{Name: x, Default: &ast.Int{ValuePos: pos, Literal: "1", Value: 1}},
		}},
	}
	for _, node := range nodes {
		c, err := New(nil)
		assert.Nil(t, err)
		_, err = c.CompileAST(node)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "unexpected")
	}
}
