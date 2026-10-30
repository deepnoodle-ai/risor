package vm

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// C-10: spreading a range into a list literal must enforce MaxListLength
// instead of materializing an unbounded slice.
func TestAuditSpreadRangeLimit(t *testing.T) {
	_, err := run(context.Background(), `[...range(1 << 23)]`)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "list too large"))

	_, err = run(context.Background(), `[1, ...range(1 << 60)]`)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "list too large"))
}

// The limit error is catchable like any other value error.
func TestAuditSpreadRangeLimitCatchable(t *testing.T) {
	result, err := run(context.Background(), `
	let r = "none"
	try { [...range(1 << 23)] } catch e { r = e.kind() }
	r
	`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewString("value error"))
}

func TestAuditSpreadStillWorks(t *testing.T) {
	result, err := run(context.Background(), `
	let base = [1]
	let a = [...base, ...range(2, 4), ...{x: 1}]
	[a, base]
	`)
	assert.Nil(t, err)
	assert.Equal(t, result.Inspect(), `[[1, 2, 3, "x"], [1]]`)
}

// Canceling the context stops a spread of a huge range promptly.
func TestAuditSpreadRangeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := run(ctx, `[...range(1 << 22)]`)
	assert.NotNil(t, err)
}
