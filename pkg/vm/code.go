package vm

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// loadedCode wraps bytecode.Code with VM-specific runtime data.
// It caches converted constants and stores the mutable globals array.
type loadedCode struct {
	*bytecode.Code
	Instructions      []op.Code
	Constants         []object.Object
	Globals           []object.Object
	Names             []string
	Locations         []object.SourceLocation
	ExceptionHandlers []bytecode.ExceptionHandler

	// Optimization metadata from compiler
	MaxCallArgs int // Maximum argument count from any Call opcode
}

func wrapCode(bc *bytecode.Code) *loadedCode {
	// Note that this does NOT set the Globals field.
	c := &loadedCode{
		Code:         bc,
		Instructions: make([]op.Code, bc.InstructionCount()),
		Constants:    make([]object.Object, bc.ConstantCount()),
		Names:        make([]string, bc.NameCount()),
		Locations:    make([]object.SourceLocation, bc.LocationCount()),
		MaxCallArgs:  bc.MaxCallArgs(),
	}

	// Copy exception handlers
	c.ExceptionHandlers = make([]bytecode.ExceptionHandler, bc.ExceptionHandlerCount())
	for i := 0; i < bc.ExceptionHandlerCount(); i++ {
		c.ExceptionHandlers[i] = bc.ExceptionHandlerAt(i)
	}

	// Copy instructions
	for i := 0; i < bc.InstructionCount(); i++ {
		c.Instructions[i] = bc.InstructionAt(i)
	}

	// Copy names
	for i := 0; i < bc.NameCount(); i++ {
		c.Names[i] = bc.NameAt(i)
	}

	// Copy and convert locations (reconstruct Filename and Source from Code)
	filename := bc.Filename()
	for i := 0; i < bc.LocationCount(); i++ {
		loc := bc.LocationAt(i)
		c.Locations[i] = object.SourceLocation{
			Filename:  filename,
			Line:      loc.Line,
			Column:    loc.Column,
			EndColumn: loc.EndColumn,
			Source:    bc.GetSourceLine(loc.Line),
		}
	}

	// Convert constants from any types to object.Object
	for i := 0; i < bc.ConstantCount(); i++ {
		constant := bc.ConstantAt(i)
		switch constant := constant.(type) {
		case int:
			c.Constants[i] = object.NewInt(int64(constant))
		case int64:
			c.Constants[i] = object.NewInt(constant)
		case float64:
			c.Constants[i] = object.NewFloat(constant)
		case string:
			c.Constants[i] = object.NewString(constant)
		case bool:
			c.Constants[i] = object.NewBool(constant)
		case *bytecode.Function:
			c.Constants[i] = object.NewClosure(constant)
		case nil:
			c.Constants[i] = object.Nil
		default:
			panic(fmt.Sprintf("unsupported constant type: %T", constant))
		}
	}
	return c
}

func (c *loadedCode) GlobalsCount() int {
	return len(c.Globals)
}

// LocalsCount returns the number of local variables in this code.
func (c *loadedCode) LocalsCount() int {
	return c.Code.LocalCount()
}

// CodeName returns the name of this code block.
func (c *loadedCode) CodeName() string {
	return c.Code.Name()
}

// LocationAt returns the source location for the instruction at the given index.
func (c *loadedCode) LocationAt(ip int) object.SourceLocation {
	if ip < 0 || ip >= len(c.Locations) {
		return object.SourceLocation{}
	}
	return c.Locations[ip]
}

func loadChildCode(root *loadedCode, bc *bytecode.Code) *loadedCode {
	c := wrapCode(bc)
	c.Globals = root.Globals
	return c
}

func loadRootCode(bc *bytecode.Code, globals map[string]object.Object) *loadedCode {
	c := wrapCode(bc)
	globalCount := bc.GlobalCount()
	c.Globals = make([]object.Object, globalCount)
	for i := 0; i < globalCount; i++ {
		name := bc.GlobalNameAt(i)
		if value, found := globals[name]; found {
			c.Globals[i] = value
		}
	}
	return c
}
