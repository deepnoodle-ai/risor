package vm

import (
	"context"

	"github.com/risor-io/risor/builtins"
	"github.com/risor-io/risor/compiler"
	modBytes "github.com/risor-io/risor/modules/bytes"
	modFmt "github.com/risor-io/risor/modules/fmt"
	modJSON "github.com/risor-io/risor/modules/json"
	modMath "github.com/risor-io/risor/modules/math"
	modRand "github.com/risor-io/risor/modules/rand"
	modStrconv "github.com/risor-io/risor/modules/strconv"
	modStrings "github.com/risor-io/risor/modules/strings"
	modTime "github.com/risor-io/risor/modules/time"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
)

// Run the given code in a new Virtual Machine and return the result.
func Run(ctx context.Context, main *compiler.Code, options ...Option) (object.Object, error) {
	machine := New(main, options...)
	if err := machine.Run(ctx); err != nil {
		return nil, err
	}
	if result, exists := machine.TOS(); exists {
		return result, nil
	}
	return object.Nil, nil
}

// RunCodeOnVM runs the given compiled code on an existing Virtual Machine and returns the result.
// This allows reusing a VM instance to run multiple different code objects sequentially.
func RunCodeOnVM(ctx context.Context, vm *VirtualMachine, code *compiler.Code, opts ...Option) (object.Object, error) {
	if err := vm.RunCode(ctx, code, opts...); err != nil {
		return nil, err
	}
	if result, exists := vm.TOS(); exists {
		return result, nil
	}
	return object.Nil, nil
}

type runOpts struct {
	Globals map[string]interface{}
}

// Run the given source code in a new VM. Used for testing.
func run(ctx context.Context, source string, opts ...runOpts) (object.Object, error) {
	vm, err := newVM(ctx, source, opts...)
	if err != nil {
		return nil, err
	}
	if err := vm.Run(ctx); err != nil {
		return nil, err
	}
	if result, exists := vm.TOS(); exists {
		return result, nil
	}
	return object.Nil, nil
}

// Return a new VM that's ready to run the given source code. Used for testing.
func newVM(ctx context.Context, source string, opts ...runOpts) (*VirtualMachine, error) {
	ast, err := parser.Parse(ctx, source)
	if err != nil {
		return nil, err
	}
	globals := basicBuiltins()
	if len(opts) > 0 {
		for k, v := range opts[0].Globals {
			globals[k] = v
		}
	}
	var globalNames []string
	for k := range globals {
		globalNames = append(globalNames, k)
	}
	main, err := compiler.Compile(ast, compiler.WithGlobalNames(globalNames))
	if err != nil {
		return nil, err
	}
	return New(main, WithGlobals(globals)), nil
}

// Builtins to be used in VM tests.
func basicBuiltins() map[string]any {
	globals := map[string]any{
		"bytes":   modBytes.Module(),
		"json":    modJSON.Module(),
		"math":    modMath.Module(),
		"rand":    modRand.Module(),
		"strconv": modStrconv.Module(),
		"strings": modStrings.Module(),
		"time":    modTime.Module(),
	}
	for k, v := range builtins.Builtins() {
		globals[k] = v
	}
	for k, v := range modFmt.Builtins() {
		globals[k] = v
	}
	return globals
}
