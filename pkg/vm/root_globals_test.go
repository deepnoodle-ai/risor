package vm

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/deepnoodle-ai/risor/v2/pkg/compiler"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/parser"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A variable declared in a block at the top level of a script uses a global
// slot. Get must still return the root variable with the same name (#492).
func TestGetReturnsRootGlobalNotBlockGlobal(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"try", `try { let foo = 1 } catch e {}; let foo = 2; foo`},
		{"catch", `try { throw "x" } catch e { let foo = 1 }; let foo = 2; foo`},
		{"catch variable", `try { throw "x" } catch foo {}; let foo = 2; foo`},
		{"finally", `try {} finally { let foo = 1 }; let foo = 2; foo`},
		{"if", `if (true) { let foo = 1 }; let foo = 2; foo`},
		{"else", `if (false) {} else { let foo = 1 }; let foo = 2; foo`},
		{"nested", `if (true) { if (true) { let foo = 1 } }; let foo = 2; foo`},
		{"root first", `let foo = 2; if (true) { let foo = 1 }; foo`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			vm, err := newVM(ctx, tt.source)
			assert.Nil(t, err)
			assert.Nil(t, vm.Run(ctx))

			result, ok := vm.TOS()
			assert.True(t, ok)
			assert.Equal(t, result, object.NewInt(2))

			value, err := vm.Get("foo")
			assert.Nil(t, err)
			assert.Equal(t, value, object.NewInt(2))
		})
	}
}

// A block variable is not a global of the script, even when no root
// variable has the same name.
func TestBlockVariableIsNotAGlobal(t *testing.T) {
	ctx := context.Background()
	vm, err := newVM(ctx, `if (true) { let bar = 1 }; let foo = 2`)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))

	_, err = vm.Get("bar")
	assert.True(t, errors.Is(err, ErrGlobalNotFound))

	names := vm.GlobalNames()
	assert.True(t, slices.Contains(names, "foo"))
	assert.False(t, slices.Contains(names, "bar"))
	assert.False(t, slices.Contains(vm.main.GlobalNames(), "bar"))
}

// An env key that contains "#" but is not of the block slot form stays
// visible to Get and GlobalNames.
func TestEnvKeyWithHashIsAGlobal(t *testing.T) {
	ctx := context.Background()
	vm, err := newVM(ctx, `if (true) { let bar = 1 }; 1`, runOpts{
		Globals: map[string]any{"a#b": 42},
	})
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))

	value, err := vm.Get("a#b")
	assert.Nil(t, err)
	assert.Equal(t, value, object.NewInt(42))
	assert.True(t, slices.Contains(vm.GlobalNames(), "a#b"))
	assert.True(t, slices.Contains(vm.main.GlobalNames(), "a#b"))
}

// RunCode carries root globals into the next code. A block variable from the
// first code must not replace a root variable with the same name.
func TestRunCodeCarriesRootGlobalNotBlockGlobal(t *testing.T) {
	ctx := context.Background()
	vm, err := newVM(ctx, `try { let foo = 1 } catch e {}; let foo = 2`)
	assert.Nil(t, err)
	assert.Nil(t, vm.Run(ctx))

	ast, err := parser.Parse(ctx, `foo`, nil)
	assert.Nil(t, err)
	code, err := compiler.Compile(ast, &compiler.Config{GlobalNames: []string{"foo"}})
	assert.Nil(t, err)
	assert.Nil(t, vm.RunCode(ctx, code))

	result, ok := vm.TOS()
	assert.True(t, ok)
	assert.Equal(t, result, object.NewInt(2))
}
