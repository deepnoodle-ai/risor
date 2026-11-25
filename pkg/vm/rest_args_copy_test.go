package vm

import (
	"context"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Issue #606: a rest parameter does not share the backing array of the
// args slice that a Go caller passes to vm.Call or Closure.Call.

func inspectAll(objs []object.Object) string {
	return object.NewList(objs).Inspect()
}

func TestRestParamDoesNotShareCallerArgs(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		body string
	}{
		{"insert front", `r.insert(0, 99)`},
		{"insert middle", `r.insert(1, 99)`},
		{"append", `r.append(99)`},
		{"assign", `r[0] = 99`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine, _ := newBuiltinsVM(t, `function f(...r) { `+tt.body+`; return r }`, nil)
			assert.Nil(t, machine.Run(ctx))
			fn, err := machine.Get("f")
			assert.Nil(t, err)

			backing := []object.Object{object.NewInt(1), object.NewInt(2), object.NewInt(3), object.NewInt(4)}
			args := backing[:3]
			result, err := machine.Call(ctx, fn.(*object.Closure), args)
			assert.Nil(t, err)
			assert.Equal(t, inspectAll(backing), "[1, 2, 3, 4]")
			assert.Equal(t, inspectAll(args), "[1, 2, 3]")

			// The caller writing into its slice afterward does not change
			// the list the script returned.
			before := result.Inspect()
			args[0] = object.NewInt(-1)
			assert.Equal(t, result.Inspect(), before)
		})
	}
}

func TestRestParamDoesNotShareClosureCallArgs(t *testing.T) {
	ctx := context.Background()
	backing := []object.Object{object.NewInt(1), object.NewInt(2), object.NewInt(3), object.NewInt(4)}
	args := backing[:3]
	callWith := object.NewBuiltin("call_with", func(ctx context.Context, in ...object.Object) (object.Object, error) {
		return in[0].(*object.Closure).Call(ctx, args...)
	})
	machine, _ := newBuiltinsVM(t, `call_with(function(first, ...r) {
		r.append(99)
		r[0] = 98
		r.insert(0, 97)
		return r
	})`, map[string]any{"call_with": callWith})
	assert.Nil(t, machine.Run(ctx))
	result, ok := machine.TOS()
	assert.True(t, ok)
	assert.Equal(t, result.Inspect(), "[97, 98, 3, 99]")
	assert.Equal(t, inspectAll(backing), "[1, 2, 3, 4]")
}
