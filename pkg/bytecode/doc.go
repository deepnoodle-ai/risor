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
// This package depends only on [github.com/deepnoodle-ai/risor/v2/pkg/op] to avoid
// circular dependencies with the object package. Constants are stored as
// []any and converted to object.Object by the VM at load time.
//
// # Loading Bytecode
//
// [Marshal] writes a Code and its descendants as JSON, and [Unmarshal] reads
// it back. Unmarshal returns an error for malformed input instead of
// panicking. It checks that the code blocks form a tree and then calls
// [Validate], which checks that:
//
//   - every instruction is complete and has a known opcode
//   - every constant, name, local, and global operand is in range
//   - every jump and exception handler target starts an instruction
//   - constants have supported types and functions have valid code
//   - no block declares more local slots than it uses
//   - source location columns are not negative or larger than a 32-bit int
//
// These checks are structural. They do not make bytecode from an untrusted
// source safe to run. Know these limits before you load it:
//
//   - Loading does not check how instructions use the value stack or the
//     types of values at run time. The VM reports such faults as errors, but
//     the behavior of code that causes them is not specified.
//   - Bytecode can read and write any variable slot of its own program,
//     including the globals the host passed in and variables declared with
//     const. It can also
//     capture the local variables of a calling frame. The compiler never
//     emits such code, so rules that the compiler enforces, such as "a script
//     cannot reassign a constant", do not hold for loaded bytecode. Bytecode
//     cannot reach the host beyond the values in its env.
//   - Validation does not bound memory use. A function can use up to 65,535
//     local slots, and each call frame allocates all of them, so deep
//     recursion can allocate about 1 GB. Bound this with the maximum frame
//     depth (risor.WithMaxStackDepth or vm.WithMaxFrameDepth) and the value
//     limits (WithLimits).
//   - Columns are not checked against the source line, since code compiled
//     without its original source can have columns past the stored lines.
//     Risor's error formatting clamps carets to the line. A host that draws
//     its own caret from a location's Column or EndColumn must clamp it too.
//   - Loading itself uses memory in proportion to the input, by a large
//     factor: a 27 MB input used about 1.2 GB to load. Limit the size of
//     untrusted input before you call Unmarshal.
//
// Where you can, compile untrusted source instead of loading untrusted
// bytecode.
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
//	// Compile source to bytecode. Pass the source so that errors show the
//	// line they occur on.
//	code, err := compiler.Compile(ast, &compiler.Config{Source: source})
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
