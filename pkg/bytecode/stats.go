package bytecode

// Stats contains statistics about compiled bytecode.
// This is useful for auditing scripts before execution.
type Stats struct {
	// InstructionCount is the total number of bytecode instructions.
	InstructionCount int

	// ConstantCount is the number of constants in the constant pool.
	ConstantCount int

	// GlobalCount is the number of global variables. Every code shares the
	// globals of its root code, so this is the global count of the root.
	GlobalCount int

	// FunctionCount is the number of functions in the constant pool. It
	// excludes functions nested inside those functions.
	FunctionCount int

	// SourceBytes is the size of the original source code in bytes.
	SourceBytes int
}
