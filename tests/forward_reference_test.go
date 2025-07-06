package tests

import (
	"context"
	"testing"

	"github.com/risor-io/risor"
	"github.com/stretchr/testify/require"
)

func TestForwardReference(t *testing.T) {
	t.Run("forward reference now works", func(t *testing.T) {
		// This should now work with forward references
		code := `
func say() {
    print(hello())
}

func hello() {
    return "hello"
}

say()
`
		ctx := context.Background()

		// Now this should work without error
		_, err := risor.Eval(ctx, code)

		// It should not error
		require.Nil(t, err)
	})

	t.Run("forward reference returns correct value", func(t *testing.T) {
		// This should work and return "hello"
		code := `
func say() {
    return hello()
}

func hello() {
    return "hello"
}

say()
`
		ctx := context.Background()
		result, err := risor.Eval(ctx, code)

		// This should work without error and return the correct value
		require.Nil(t, err)
		require.Equal(t, "\"hello\"", result.Inspect())
	})
}
