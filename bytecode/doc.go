// Package bytecode provides immutable representations of compiled Risor code.
//
// This package defines the output of compilation: pure data structures that
// represent compiled bytecode, function templates, and associated metadata.
// These types are designed to be created once during compilation and shared
// safely across multiple goroutines and VM instances.
//
// # Key Types
//
//   - [Code]: An immutable compiled code block (module, function body, etc.)
//   - [Function]: An immutable function template with parameters and code reference
//   - [ExceptionHandler]: Describes a try/catch/finally block (value type)
//   - [SourceLocation]: Maps bytecode to source positions (value type)
//
// # Immutability Guarantees
//
// All types in this package are immutable after construction:
//
//   - No mutation methods exist on any type
//   - All fields are unexported
//   - Constructors copy input slices to prevent caller mutation
//   - Accessors return values or immutable pointers, never mutable slices
//
// Index-based access is used for all collections:
//
//	// Correct: index-based access
//	code.InstructionAt(0)
//	code.ConstantAt(i)
//	code.ChildAt(j)
//
//	// NOT provided: methods that return slices
//	// code.Instructions() - does not exist
//	// code.Constants() - does not exist
//
// # Package Dependencies
//
// This package depends only on [github.com/risor-io/risor/op] to avoid
// circular dependencies with the object package. Constants are stored as
// []any and converted to object.Object by the VM at load time.
//
// # Usage
//
// The compiler produces bytecode.Code which can be:
//
//   - Executed directly by the VM
//   - Serialized for caching or distribution
//   - Inspected for debugging or analysis
//
// Example:
//
//	// Compile source to bytecode
//	code, err := compiler.Compile(ast)
//	if err != nil {
//	    return err
//	}
//
//	// Inspect the compiled code
//	fmt.Printf("Instructions: %d\n", code.InstructionCount())
//	fmt.Printf("Constants: %d\n", code.ConstantCount())
//
//	// Execute on a VM
//	result, err := vm.Run(ctx, code)
package bytecode
