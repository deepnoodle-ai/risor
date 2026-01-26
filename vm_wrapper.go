package risor

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
	"github.com/risor-io/risor/vm"
)

// VM provides stateful execution for REPL and incremental evaluation.
// Unlike the Eval and Run functions which create fresh state on each call,
// the VM maintains state across multiple executions, allowing for interactive
// sessions where variables and functions persist.
type VM struct {
	machine  *vm.VirtualMachine
	compiler *compiler.Compiler
	cfg      *config
}

// NewVM creates a new VM with the given options.
// The VM can be used for REPL-style incremental evaluation or for calling
// functions defined in previously run code.
func NewVM(options ...Option) (*VM, error) {
	cfg := newConfig(options...)

	c, err := compiler.New(cfg.CompilerOpts()...)
	if err != nil {
		return nil, err
	}

	machine, err := vm.NewEmpty()
	if err != nil {
		return nil, err
	}

	return &VM{
		machine:  machine,
		compiler: c,
		cfg:      cfg,
	}, nil
}

// Eval evaluates source code within this VM's context.
// Variables and functions defined in previous Eval calls remain accessible.
// This is the primary method for REPL-style interaction.
func (v *VM) Eval(ctx context.Context, source string) (any, error) {
	ast, err := parser.Parse(ctx, source)
	if err != nil {
		return nil, err
	}

	code, err := v.compiler.Compile(ast)
	if err != nil {
		return nil, err
	}

	if err := v.machine.RunCode(ctx, code, v.cfg.VMOpts()...); err != nil {
		// Update the IP to be after the last instruction, so that next
		// time around we start in the right location.
		v.machine.SetIP(code.InstructionCount())
		return nil, err
	}

	result, ok := v.machine.TOS()
	if !ok || result == nil {
		return nil, nil
	}

	// Handle error objects specially
	if errObj, isErr := result.(*object.Error); isErr && errObj.IsRaised() {
		return nil, errObj.Value()
	}

	return result.Interface(), nil
}

// Run executes a compiled Program within this VM's context.
// Unlike the top-level Run function, this maintains state across calls.
func (v *VM) Run(ctx context.Context, p *Program) (any, error) {
	if err := v.machine.RunCode(ctx, p.code, v.cfg.VMOpts()...); err != nil {
		return nil, err
	}

	result, ok := v.machine.TOS()
	if !ok || result == nil {
		return nil, nil
	}

	return result.Interface(), nil
}

// Call invokes a function defined in the VM's context by name.
// The function must have been defined in a previous Eval or Run call.
// Arguments are converted from Go types to Risor objects automatically.
func (v *VM) Call(ctx context.Context, name string, args ...any) (any, error) {
	obj, err := v.machine.Get(name)
	if err != nil {
		return nil, err
	}

	fn, ok := obj.(*object.Function)
	if !ok {
		return nil, fmt.Errorf("object is not a function (got: %s)", obj.Type())
	}

	// Convert Go args to Risor objects
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
// The value is returned as a native Go type.
func (v *VM) Get(name string) (any, error) {
	obj, err := v.machine.Get(name)
	if err != nil {
		return nil, err
	}
	return obj.Interface(), nil
}

// GetObject retrieves a global variable by name as a Risor object.
// This is useful when you need to work with the object's methods directly.
func (v *VM) GetObject(name string) (object.Object, error) {
	return v.machine.Get(name)
}

// GlobalNames returns the names of all global variables in the VM's context.
func (v *VM) GlobalNames() []string {
	return v.machine.GlobalNames()
}

// InternalVM returns the underlying vm.VirtualMachine.
// This is primarily for advanced use cases and testing.
func (v *VM) InternalVM() *vm.VirtualMachine {
	return v.machine
}
