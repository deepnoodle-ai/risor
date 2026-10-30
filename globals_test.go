package risor

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Reading a hoisted function before its declaration executes
// must evaluate to nil rather than crash the host.
func TestEvalGlobalReadBeforeInit(t *testing.T) {
	result, err := Eval(context.Background(), `
	let f = foo
	function foo() { return 1 }
	f
	`)
	assert.Nil(t, err)
	assert.Nil(t, result)
}
