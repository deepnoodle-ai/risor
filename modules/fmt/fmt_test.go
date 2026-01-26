package fmt

import (
	"context"
	"testing"

	"github.com/risor-io/risor/object"
	"github.com/stretchr/testify/require"
)

func TestErrorf(t *testing.T) {
	ctx := context.Background()
	result := Errorf(ctx, object.NewString("hello %s\n"), object.NewString("world"))
	require.IsType(t, &object.Error{}, result)
	require.Equal(t, "hello world\n", result.(*object.Error).Message().Value())
}

func TestSprintf(t *testing.T) {
	ctx := context.Background()
	result := Sprintf(ctx, object.NewString("hello %s\n"), object.NewString("world"))
	require.Equal(t, "hello world\n", result.(*object.String).Value())
}
