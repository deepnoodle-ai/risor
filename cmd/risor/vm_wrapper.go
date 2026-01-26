package main

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/risor-io/risor/bytecode"
	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
	"github.com/risor-io/risor/vm"
)

// replVM provides stateful execution for REPL and incremental evaluation.
// It maintains state across multiple executions, allowing for interactive
// sessions where variables and functions persist.
type replVM struct {
	machine  *vm.VirtualMachine
	compiler *compiler.Compiler
	env      map[string]any

	// nextIP tracks where to start execution for the next Eval call.
	// This allows incremental compilation where new code is appended
	// and we skip past previously executed (or errored) code.
	nextIP int
}

// newReplVM creates a new REPL VM with the given environment.
func newReplVM(env map[string]any) (*replVM, error) {
	var compilerOpts []compiler.Option
	if len(env) > 0 {
		names := slices.Sorted(maps.Keys(env))
		compilerOpts = append(compilerOpts, compiler.WithGlobalNames(names))
	}

	c, err := compiler.New(compilerOpts...)
	if err != nil {
		return nil, err
	}

	machine, err := vm.NewEmpty()
	if err != nil {
		return nil, err
	}

	return &replVM{
		machine:  machine,
		compiler: c,
		env:      env,
	}, nil
}

func (v *replVM) vmOpts() []vm.Option {
	var opts []vm.Option
	if len(v.env) > 0 {
		opts = append(opts, vm.WithGlobals(v.env))
	}
	if v.nextIP > 0 {
		opts = append(opts, vm.WithInstructionOffset(v.nextIP))
	}
	return opts
}

// Eval evaluates source code within this VM's context.
// Variables and functions defined in previous Eval calls remain accessible.
func (v *replVM) Eval(ctx context.Context, source string) (any, error) {
	ast, err := parser.Parse(ctx, source)
	if err != nil {
		return nil, err
	}

	code, err := v.compiler.CompileAST(ast)
	if err != nil {
		return nil, err
	}

	// Convert compiler.Code to bytecode.Code for the VM
	bc := code.ToBytecode()

	if err := v.machine.RunCode(ctx, bc, v.vmOpts()...); err != nil {
		// Advance past the erroring code so subsequent Eval calls skip it
		v.nextIP = bc.InstructionCount()
		return nil, err
	}

	// Advance past executed code for next Eval call
	v.nextIP = bc.InstructionCount()

	result, ok := v.machine.TOS()
	if !ok || result == nil {
		return nil, nil
	}

	if errObj, isErr := result.(*object.Error); isErr && errObj.IsRaised() {
		return nil, errObj.Value()
	}

	return result.Interface(), nil
}

// Run executes compiled bytecode within this VM's context.
func (v *replVM) Run(ctx context.Context, code *bytecode.Code) (any, error) {
	if err := v.machine.RunCode(ctx, code, v.vmOpts()...); err != nil {
		return nil, err
	}

	result, ok := v.machine.TOS()
	if !ok || result == nil {
		return nil, nil
	}

	return result.Interface(), nil
}

// Call invokes a function defined in the VM's context by name.
func (v *replVM) Call(ctx context.Context, name string, args ...any) (any, error) {
	obj, err := v.machine.Get(name)
	if err != nil {
		return nil, err
	}

	fn, ok := obj.(*object.Closure)
	if !ok {
		return nil, fmt.Errorf("object is not a function (got: %s)", obj.Type())
	}

	risorArgs := make([]object.Object, len(args))
	for i, arg := range args {
		risorArgs[i] = object.FromGoType(arg)
		if risorArgs[i] == nil {
			return nil, fmt.Errorf("cannot convert argument %d to Risor object", i)
		}
	}

	result, err := v.machine.Call(ctx, fn, risorArgs)
	if err != nil {
		return nil, err
	}

	return result.Interface(), nil
}

// Get retrieves a global variable by name from the VM's context.
func (v *replVM) Get(name string) (any, error) {
	obj, err := v.machine.Get(name)
	if err != nil {
		return nil, err
	}
	return obj.Interface(), nil
}

// GlobalNames returns the names of all global variables in the VM's context.
func (v *replVM) GlobalNames() []string {
	return v.machine.GlobalNames()
}
