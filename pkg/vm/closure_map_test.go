package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

// closureWithoutCells builds code that runs LoadClosure inside a try block
// with freeCount values on the stack that are not cells. The catch block
// leaves the error on top of the stack.
func closureWithoutCells(freeCount int) *bytecode.Code {
	child := bytecode.NewCode(bytecode.CodeParams{
		ID:           "child",
		Instructions: []op.Code{op.Nil, op.ReturnValue},
	})
	fn := bytecode.NewFunction(bytecode.FunctionParams{ID: "fn", Code: child})
	var insts []op.Code
	// PushExcept with the catch offset filled in below
	insts = append(insts, op.PushExcept, 0, 0)
	for i := 0; i < freeCount; i++ {
		insts = append(insts, op.LoadConst, 1)
	}
	insts = append(insts, op.LoadClosure, 0, op.Code(freeCount))
	// Skip the catch block, which is one Nop
	insts = append(insts, op.PopExcept, op.JumpForward, 3)
	catchStart := len(insts)
	insts = append(insts, op.Nop)
	insts[1] = op.Code(catchStart)
	return bytecode.NewCode(bytecode.CodeParams{
		ID:           "main",
		Children:     []*bytecode.Code{child},
		Instructions: insts,
		Constants:    []any{fn, int64(1)},
	})
}

// A catch block catches the "expected cell" error of LoadClosure, and the
// handler receives the error, whatever the free variable count.
func TestLoadClosureErrorIsCaught(t *testing.T) {
	for _, freeCount := range []int{1, 2, 3} {
		result, err := Run(context.Background(), closureWithoutCells(freeCount))
		assert.Nil(t, err, freeCount)
		errObj, ok := result.(*object.Error)
		assert.True(t, ok, "freeCount %d: got %v", freeCount, result)
		assert.Contains(t, errObj.Message().Value(), "expected cell")
	}
}

// In a map literal, a later duplicate key wins, with or without a spread.
func TestMapLiteralDuplicateKeys(t *testing.T) {
	result, err := run(context.Background(), `[{a: 1, a: 2}, {...{}, a: 1, a: 2}, {a: 1, b: 2, a: 3}]`)
	assert.Nil(t, err)
	assert.Equal(t, result, object.NewList([]object.Object{
		object.NewMap(map[string]object.Object{"a": object.NewInt(2)}),
		object.NewMap(map[string]object.Object{"a": object.NewInt(2)}),
		object.NewMap(map[string]object.Object{"a": object.NewInt(3), "b": object.NewInt(2)}),
	}))
}
