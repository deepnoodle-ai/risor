package vm

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Spreading a range into a list literal must enforce the list length limit
// instead of materializing an unbounded slice.
func TestSpreadRangeLimit(t *testing.T) {
	_, err := run(context.Background(), `[...range(1 << 23)]`)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "list too large"))

	_, err = run(context.Background(), `[1, ...range(1 << 60)]`)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "list too large"))
}

// The limit error is catchable like any other value error.
func TestSpreadRangeLimitCatchable(t *testing.T) {
	result, err := run(context.Background(), `
	let r = "none"
	try { [...range(1 << 23)] } catch e { r = e.kind() }
	r
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("value error"))
}

func TestSpreadStillWorks(t *testing.T) {
	result, err := run(context.Background(), `
	let base = [1]
	let a = [...base, ...range(2, 4), ...{x: 1}]
	[a, base]
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `[[1, 2, 3, "x"], [1]]`)
}

// Canceling the context stops a spread of a huge range promptly.
func TestSpreadRangeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := run(ctx, `[...range(1 << 22)]`)
	assert.NotNil(t, err)
}

// Limits stored in the context apply, and vm.WithLimits takes precedence
// over them.
func TestLimitsFromContextAndOption(t *testing.T) {
	ast, err := parser.Parse(context.Background(), `"a".repeat(11)`, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, nil)
	assert.Nil(t, err)

	small := object.WithLimits(context.Background(), object.Limits{MaxBytesSize: 10})
	_, err = Run(small, code)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "too large"))

	result, err := Run(small, code, WithLimits(object.Limits{MaxBytesSize: 20}))
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString(strings.Repeat("a", 11)))
}
