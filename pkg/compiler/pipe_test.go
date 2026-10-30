package compiler

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A failed compile of a pipe must not affect later compiles into the same
// code, as in the REPL.
func TestPipeAfterFailedCompile(t *testing.T) {
	ctx := context.Background()
	c, err := New(nil)
	assert.Nil(t, err)

	ast1, err := parser.Parse(ctx, `1 |> (x => unknown_var)`, nil)
	assert.Nil(t, err)
	_, err = c.CompileAST(ast1)
	assert.NotNil(t, err)

	ast2, err := parser.Parse(ctx, `1 |> (x => x + 1)`, nil)
	assert.Nil(t, err)
	_, err = c.CompileAST(ast2)
	assert.Nil(t, err)
}
