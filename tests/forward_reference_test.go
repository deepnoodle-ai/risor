package tests

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/deepnoodle-ai/risor/v2"
)

func TestForwardReference(t *testing.T) {
	t.Run("forward reference now works", func(t *testing.T) {
		// This should now work with forward references
		code := `
function say() {
    return hello()
}

function hello() {
    return "hello"
}

say()
`
		ctx := context.Background()

		// Now this should work without error
		result, err := risor.Eval(ctx, code)

		// It should not error and return the correct value
		assert.Nil(t, err)
		assert.Equal(t, result, "hello")
	})

	t.Run("forward reference returns correct value", func(t *testing.T) {
		// This should work and return "hello"
		code := `
function say() {
    return hello()
}

function hello() {
    return "hello"
}

say()
`
		ctx := context.Background()
		result, err := risor.Eval(ctx, code)

		// This should work without error and return the correct value
		assert.Nil(t, err)
		assert.Equal(t, result, "hello")
	})
}
