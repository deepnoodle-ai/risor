package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A parameter that has no default and no argument is Nil, not a value left
// in vm.tmp by an earlier call. Validate rejects this function, since it has
// fewer defaults than parameters, so this runs it without validation.
func TestMissingDefaultIsNil(t *testing.T) {
	body := bytecode.NewCode(bytecode.CodeParams{
		ID: "main.0", Name: "f", LocalCount: 2,
		Instructions: []op.Code{op.LoadFast, 1, op.ReturnValue},
	})
	fn := bytecode.NewFunction(bytecode.FunctionParams{
		ID: "f", Name: "f", Parameters: []string{"x", "y"},
		Defaults: []any{int64(99)}, Code: body,
	})
	main := bytecode.NewCode(bytecode.CodeParams{
		ID:       "main",
		Children: []*bytecode.Code{body},
		Instructions: []op.Code{
			op.LoadClosure, 0, 0, op.LoadConst, 1, op.LoadConst, 1, op.Call, 2, op.PopTop,
			op.LoadClosure, 0, 0, op.LoadConst, 1, op.Call, 1,
		},
		Constants: []any{fn, int64(7)},
	})
	result, err := Run(context.Background(), main)
	assert.Nil(t, err)
	assert.Equal(t, result, object.Nil)
}

// A nil default before a non-nil default is Nil in the frame, not a raw Go
// nil.
func TestNilDefaultBeforeOtherDefaults(t *testing.T) {
	result, err := run(context.Background(), `
	function f(a = 1, b = nil, c = 2) { return [a, b, c] }
	f(10)`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewList([]object.Object{
		object.NewInt(10), object.Nil, object.NewInt(2),
	}))
}

// RETURN_VALUE in the root frame ends the run with the returned value. The
// compiler never emits it, but hand-built bytecode may.
func TestReturnFromRootFrame(t *testing.T) {
	main := bytecode.NewCode(bytecode.CodeParams{
		Instructions: []op.Code{op.LoadConst, 0, op.ReturnValue, op.LoadConst, 1},
		Constants:    []any{int64(5), int64(6)},
	})
	assert.Nil(t, bytecode.Validate(main))
	result, err := Run(context.Background(), main)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewInt(5))
}
