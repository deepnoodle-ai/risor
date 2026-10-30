package compiler

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Audit C-04: a function declared inside a top-level block is scoped to that
// block. It must not be hoisted into the root symbol table, where it would
// occupy a global slot that is never initialized.
func TestAuditBlockFunctionNotHoisted(t *testing.T) {
	parsed, err := parser.Parse(context.Background(), "function foo() { return 42 }\nfoo", nil)
	assert.Nil(t, err)
	assert.Len(t, parsed.Stmts, 2)

	// The parser never produces a bare top-level block, so build the AST
	// directly: { function foo() { return 42 } }; foo
	program := &ast.Program{Stmts: []ast.Node{
		&ast.Block{Stmts: []ast.Node{parsed.Stmts[0]}},
		parsed.Stmts[1],
	}}
	_, err = Compile(program, nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), `undefined variable "foo"`)

	// Referencing the function within its block is fine, and only the block
	// scoped slot exists (no duplicate, never-initialized root slot).
	program = &ast.Program{Stmts: []ast.Node{
		&ast.Block{Stmts: []ast.Node{parsed.Stmts[0], parsed.Stmts[1]}},
	}}
	code, err := Compile(program, nil)
	assert.Nil(t, err)
	count := 0
	for i := 0; i < code.GlobalCount(); i++ {
		if code.GlobalNameAt(i) == "foo" {
			count++
		}
	}
	assert.Equal(t, count, 1)
}
