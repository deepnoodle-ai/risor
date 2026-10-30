package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

// hostClosure returns a closure with no free variables that runs the given
// instructions. Validate does not check the free variables of a block that
// no instruction loads, so a host can call such a closure.
func hostClosure(instructions ...op.Code) *object.Closure {
	body := bytecode.NewCode(bytecode.CodeParams{ID: "host.0", Name: "f", Instructions: instructions})
	return object.NewClosure(bytecode.NewFunction(bytecode.FunctionParams{
		ID: "f", Name: "f", Code: body,
	}))
}

// A free variable index past the closure's captures is a catchable error,
// not a Go panic.
func TestFreeVariableOutOfRange(t *testing.T) {
	tests := []struct {
		name string
		fn   *object.Closure
	}{
		{"load", hostClosure(op.LoadFree, 0, op.ReturnValue)},
		{"store", hostClosure(op.Nil, op.StoreFree, 0, op.Nil, op.ReturnValue)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globals := map[string]any{"f": tt.fn}
			_, err := run(context.Background(), `f()`, runOpts{Globals: globals})
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "no free variable at index 0")

			result, err := run(context.Background(),
				`try { f() } catch e { string(e) }`, runOpts{Globals: globals})
			assert.Nil(t, err)
			assert.Equal(t, result, object.NewString("no free variable at index 0"))
		})
	}
}

// A MAKE_CELL for an enclosing frame with a local index past that frame's
// locals is a catchable error, not a Go panic. Validate does not check it,
// since the enclosing frame depends on the call stack.
func TestMakeCellLocalOutOfRange(t *testing.T) {
	body := bytecode.NewCode(bytecode.CodeParams{
		ID: "main.0", Name: "f",
		Instructions: []op.Code{op.MakeCell, 5, 1, op.ReturnValue},
	})
	main := bytecode.NewCode(bytecode.CodeParams{
		ID:           "main",
		Children:     []*bytecode.Code{body},
		Instructions: []op.Code{op.LoadClosure, 0, 0, op.Call, 0},
		Constants: []any{bytecode.NewFunction(bytecode.FunctionParams{
			ID: "f", Name: "f", Code: body,
		})},
	})
	assert.Nil(t, bytecode.Validate(main))
	_, err := Run(context.Background(), main)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no local at index 5")
}
