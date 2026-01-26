package risor

import (
	"bytes"
	"io"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/dis"
)

// ProgramStats contains statistics about a compiled Program.
// This is useful for auditing scripts before execution.
type ProgramStats struct {
	// InstructionCount is the total number of bytecode instructions.
	InstructionCount int

	// ConstantCount is the number of constants in the constant pool.
	ConstantCount int

	// GlobalCount is the number of global variables.
	GlobalCount int

	// FunctionCount is the number of functions defined in the program.
	FunctionCount int

	// SourceBytes is the size of the original source code in bytes.
	SourceBytes int
}

// Program is the compiled representation of Risor source code.
// It is immutable after creation and safe for concurrent use.
// Multiple goroutines can call Run on the same Program simultaneously.
type Program struct {
	code *compiler.Code // Internal, immutable bytecode

	// Metadata
	source   string
	filename string
}

// Source returns the original source code that was compiled.
func (p *Program) Source() string {
	return p.source
}

// Filename returns the filename associated with this program, if any.
func (p *Program) Filename() string {
	return p.filename
}

// GlobalNames returns the names of all global variables defined in this program.
func (p *Program) GlobalNames() []string {
	return p.code.GlobalNames()
}

// Code returns the internal compiler.Code for use by the VM.
// This is an internal method and should not be used directly.
func (p *Program) Code() *compiler.Code {
	return p.code
}

// Stats returns statistics about the compiled program.
// This is useful for auditing scripts before execution, for example
// to reject scripts that are too large or complex.
func (p *Program) Stats() ProgramStats {
	// Count functions by looking for Function constants
	functionCount := 0
	for i := 0; i < p.code.ConstantsCount(); i++ {
		if _, ok := p.code.Constant(i).(*compiler.Function); ok {
			functionCount++
		}
	}

	return ProgramStats{
		InstructionCount: p.code.InstructionCount(),
		ConstantCount:    p.code.ConstantsCount(),
		GlobalCount:      p.code.GlobalsCount(),
		FunctionCount:    functionCount,
		SourceBytes:      len(p.source),
	}
}

// Disassemble returns a string representation of the program's bytecode.
// This is useful for debugging and understanding what a script will do.
func (p *Program) Disassemble() (string, error) {
	var buf bytes.Buffer
	if err := p.DisassembleWriter(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// DisassembleWriter writes a string representation of the program's bytecode
// to the given writer. This is useful for debugging and understanding what
// a script will do.
func (p *Program) DisassembleWriter(w io.Writer) error {
	instructions, err := dis.Disassemble(p.code)
	if err != nil {
		return err
	}
	dis.Print(instructions, w)
	return nil
}

// FunctionNames returns the names of all functions defined in the program.
// Anonymous functions are not included.
func (p *Program) FunctionNames() []string {
	var names []string
	for i := 0; i < p.code.ConstantsCount(); i++ {
		if fn, ok := p.code.Constant(i).(*compiler.Function); ok {
			if name := fn.Name(); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}
