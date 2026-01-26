package risor

import (
	"github.com/risor-io/risor/compiler"
)

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
