package parser_test

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/ast"
	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Every AST the parser accepts must be bounded by MaxDepth so it can be
// walked, stringified and compiled without overflowing the Go stack.

// longChains returns inputs that build left-deep ASTs of n levels from a
// single parseNode frame.
func longChains(n int) map[string]string {
	return map[string]string{
		"attr":     "x" + strings.Repeat(".a", n),
		"optional": "x" + strings.Repeat("?.a", n),
		"infix":    "1" + strings.Repeat("+1", n),
		"logical":  "a" + strings.Repeat("&&a", n),
		"call":     "f" + strings.Repeat("()", n),
		"index":    "a" + strings.Repeat("[0]", n),
		"method":   "x" + strings.Repeat(".m()", n),
		"newline":  "x" + strings.Repeat("\n.a", n),
	}
}

// exercise walks, stringifies and compiles an accepted program. A stack
// overflow here is fatal to the test process.
func exercise(t *testing.T, prog *ast.Program) {
	t.Helper()
	count := 0
	ast.Inspect(prog, func(n ast.Node) bool {
		count++
		return true
	})
	for range ast.Preorder(prog) {
	}
	_ = prog.String()
	_, _ = compiler.Compile(prog, nil)
	assert.True(t, count > 0)
}

func TestLongChainsRejected(t *testing.T) {
	ctx := context.Background()
	for name, input := range longChains(500_000) {
		t.Run(name, func(t *testing.T) {
			for _, cfg := range []*parser.Config{nil, {MaxDepth: 10}} {
				prog, err := parser.Parse(ctx, input, cfg)
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), "maximum nesting depth exceeded")
				if prog != nil {
					exercise(t, prog)
				}
			}
		})
	}
}

func TestChainsWithinLimitAccepted(t *testing.T) {
	ctx := context.Background()
	for name, input := range longChains(parser.DefaultMaxDepth - 10) {
		t.Run(name, func(t *testing.T) {
			prog, err := parser.Parse(ctx, input, nil)
			assert.Nil(t, err)
			exercise(t, prog)
		})
	}
}

func TestChainDepthSmallLimit(t *testing.T) {
	ctx := context.Background()
	_, err := parser.Parse(ctx, "x.a.a.a", &parser.Config{MaxDepth: 10})
	assert.Nil(t, err)
	_, err = parser.Parse(ctx, "x"+strings.Repeat(".a", 20), &parser.Config{MaxDepth: 10})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth exceeded")
}

// Chain depth and nesting depth share one budget, so mixing them cannot
// exceed MaxDepth either.
func TestChainInsideNesting(t *testing.T) {
	ctx := context.Background()
	input := strings.Repeat("(", 300) + "x" + strings.Repeat(".a", 300) + strings.Repeat(")", 300)
	_, err := parser.Parse(ctx, input, nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth exceeded")
}

// Multiple statements each get the full budget; the chain budget is
// released when a statement finishes.
func TestChainBudgetReleased(t *testing.T) {
	ctx := context.Background()
	line := "x" + strings.Repeat(".a", 50) + "\n"
	prog, err := parser.Parse(ctx, strings.Repeat(line, 100), &parser.Config{MaxDepth: 60})
	assert.Nil(t, err)
	assert.Len(t, prog.Stmts, 100)
}

func elseIfChain(n int) string {
	var sb strings.Builder
	sb.WriteString("if (x) { 1 }")
	for i := 0; i < n; i++ {
		sb.WriteString(" else if (x) { 1 }")
	}
	sb.WriteString(" else { 2 }")
	return sb.String()
}

func TestElseIfChainRejected(t *testing.T) {
	ctx := context.Background()
	input := elseIfChain(500_000)
	for _, cfg := range []*parser.Config{nil, {MaxDepth: 10}} {
		prog, err := parser.Parse(ctx, input, cfg)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "maximum nesting depth exceeded")
		if prog != nil {
			exercise(t, prog)
		}
	}
}

func TestElseIfChainWithinLimitAccepted(t *testing.T) {
	ctx := context.Background()
	prog, err := parser.Parse(ctx, elseIfChain(parser.DefaultMaxDepth-10), nil)
	assert.Nil(t, err)
	exercise(t, prog)

	_, err = parser.Parse(ctx, elseIfChain(5), &parser.Config{MaxDepth: 10})
	assert.Nil(t, err)
	_, err = parser.Parse(ctx, elseIfChain(20), &parser.Config{MaxDepth: 10})
	assert.NotNil(t, err)
}

func TestMatchDefaultGuardRejected(t *testing.T) {
	ctx := context.Background()
	_, err := parser.Parse(ctx, "match x { 1 => 2, _ if forbidden() => 3 }", nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "default arm")

	// Guards on non-default arms are still allowed.
	_, err = parser.Parse(ctx, "match x { 1 if y => 2, _ => 3 }", nil)
	assert.Nil(t, err)
}

func TestEmptyTemplateInterpolationRejected(t *testing.T) {
	ctx := context.Background()
	for _, input := range []string{"`${}`", "`a${ }b`", "`${x}${}`"} {
		_, err := parser.Parse(ctx, input, nil)
		assert.NotNil(t, err, input)
		assert.Contains(t, err.Error(), "empty template interpolation")
	}
	_, err := parser.Parse(ctx, "`a${x}b`", nil)
	assert.Nil(t, err)
}

// Template interpolations are parsed by a nested parser, which must inherit
// the remaining depth budget instead of starting over at DefaultMaxDepth.
func TestTemplateInheritsDepthBudget(t *testing.T) {
	ctx := context.Background()
	inner := strings.Repeat("(", 30) + "x" + strings.Repeat(")", 30)
	_, err := parser.Parse(ctx, "`${"+inner+"}`", &parser.Config{MaxDepth: 10})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth exceeded")

	_, err = parser.Parse(ctx, "`${(x)}`", &parser.Config{MaxDepth: 10})
	assert.Nil(t, err)

	// Budget used by enclosing expressions counts against the template.
	outer := strings.Repeat("(", 8) + "`${((x))}`" + strings.Repeat(")", 8)
	_, err = parser.Parse(ctx, outer, &parser.Config{MaxDepth: 10})
	assert.NotNil(t, err)
}

// A postfix ++ or -- wraps its operand, so it costs one level like an infix
// operator does.
func TestPostfixCountsTowardDepth(t *testing.T) {
	ctx := context.Background()
	_, err := parser.Parse(ctx, "x++", &parser.Config{MaxDepth: 2})
	assert.Nil(t, err)
	_, err = parser.Parse(ctx, "x++", &parser.Config{MaxDepth: 1})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "maximum nesting depth exceeded")
}
