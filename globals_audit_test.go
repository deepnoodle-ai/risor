package risor

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Audit C-04: reading a hoisted function before its declaration executes
// must evaluate to nil rather than crash the host.
func TestAuditEvalGlobalReadBeforeInit(t *testing.T) {
	result, err := Eval(context.Background(), `
	let f = foo
	function foo() { return 1 }
	f
	`)
	assert.Nil(t, err)
	assert.Nil(t, result)
}
