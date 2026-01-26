package risor

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
	"github.com/risor-io/risor/vm"
)

//go:generate go run ./cmd/risor-modgen

// Compile parses and compiles source code into an executable Program.
// The returned Program is immutable and safe for concurrent use.
// Multiple goroutines can execute the same Program simultaneously.
func Compile(source string, options ...Option) (*Program, error) {
	cfg := newConfig(options...)

	// Parse the source code to create the AST
	var parserOpts []parser.Option
	if cfg.filename != "" {
		parserOpts = append(parserOpts, parser.WithFilename(cfg.filename))
	}
	ast, err := parser.Parse(context.Background(), source, parserOpts...)
	if err != nil {
		return nil, err
	}

	// Compile the AST to bytecode
	code, err := compiler.Compile(ast, cfg.CompilerOpts()...)
	if err != nil {
		return nil, err
	}

	return &Program{
		code:     code,
		source:   source,
		filename: cfg.filename,
	}, nil
}

// Run executes a compiled Program and returns the result as a native Go value.
// Each call creates fresh runtime state, allowing concurrent execution of the
// same Program.
func Run(ctx context.Context, p *Program, options ...Option) (any, error) {
	cfg := newConfig(options...)

	// Use the specified VM if provided
	if cfg.vm != nil {
		result, err := vm.RunCodeOnVM(ctx, cfg.vm, p.code, cfg.VMOpts()...)
		if err != nil {
			return nil, err
		}
		return result.Interface(), nil
	}

	// Run the bytecode in a new VM then return the result as a Go value
	result, err := vm.Run(ctx, p.code, cfg.VMOpts()...)
	if err != nil {
		return nil, err
	}
	return result.Interface(), nil
}

// Eval is a convenience function that compiles and runs source code.
// It is equivalent to Compile() followed by Run().
// Returns the result as a native Go value.
func Eval(ctx context.Context, source string, options ...Option) (any, error) {
	program, err := Compile(source, options...)
	if err != nil {
		return nil, err
	}
	return Run(ctx, program, options...)
}

// EvalCode evaluates the precompiled code and returns the result.
//
// Deprecated: Use Run with a *Program instead. This function will be removed in v2.
// Example migration:
//
//	// Old way:
//	code, _ := compiler.Compile(ast)
//	result, _ := EvalCode(ctx, code)
//
//	// New way:
//	program, _ := Compile(source)
//	result, _ := Run(ctx, program)
func EvalCode(ctx context.Context, main *compiler.Code, options ...Option) (object.Object, error) {
	cfg := newConfig(options...)

	// Use the specified VM if provided
	if cfg.vm != nil {
		return vm.RunCodeOnVM(ctx, cfg.vm, main, cfg.VMOpts()...)
	}

	// Eval the bytecode in a VM then return the top-of-stack (TOS) value
	return vm.Run(ctx, main, cfg.VMOpts()...)
}

// Call evaluates the precompiled code and then calls the named function.
// The supplied arguments are passed in the function call. The result of
// the function call is returned.
//
// Deprecated: Use VM.Call instead. This function will be removed in v2.
// Example migration:
//
//	// Old way:
//	code, _ := compiler.Compile(ast)
//	result, _ := Call(ctx, code, "myFunc", args)
//
//	// New way:
//	vm, _ := NewVM(opts...)
//	vm.Eval(ctx, source)
//	result, _ := vm.Call(ctx, "myFunc", goArgs...)
func Call(
	ctx context.Context,
	main *compiler.Code,
	functionName string,
	args []object.Object,
	options ...Option,
) (object.Object, error) {
	cfg := newConfig(options...)

	// Determine whether to use an existing VM or create a new one
	var err error
	var machine *vm.VirtualMachine
	if cfg.vm != nil {
		machine = cfg.vm
	} else {
		machine, err = vm.NewEmpty()
		if err != nil {
			return nil, err
		}
	}

	// Run the code to evaluate globals etc.
	if err := machine.RunCode(ctx, main, cfg.VMOpts()...); err != nil {
		return nil, err
	}

	// Get the requested function
	obj, err := machine.Get(functionName)
	if err != nil {
		return nil, err
	}
	fn, ok := obj.(*object.Function)
	if !ok {
		return nil, fmt.Errorf("object is not a function (got: %s)", obj.Type())
	}

	// Call the function
	return machine.Call(ctx, fn, args)
}
