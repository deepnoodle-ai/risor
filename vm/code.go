package vm

import (
	"fmt"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/op"
)

type code struct {
	*compiler.Code
	Instructions      []op.Code
	Constants         []object.Object
	Globals           []object.Object
	Names             []string
	Locations         []errz.SourceLocation
	ExceptionHandlers []*compiler.ExceptionHandler

	// Optimization metadata from compiler
	MaxCallArgs int // Maximum argument count from any Call opcode
}

func wrapCode(cc *compiler.Code) *code {
	// Note that this does NOT set the Globals field.
	c := &code{
		Code:              cc,
		Instructions:      make([]op.Code, cc.InstructionCount()),
		Constants:         make([]object.Object, cc.ConstantsCount()),
		Names:             make([]string, cc.NameCount()),
		Locations:         make([]errz.SourceLocation, cc.LocationsCount()),
		ExceptionHandlers: cc.ExceptionHandlers(),
		MaxCallArgs:       cc.MaxCallArgs(),
	}
	for i := 0; i < cc.InstructionCount(); i++ {
		c.Instructions[i] = cc.Instruction(i)
	}
	for i := 0; i < cc.NameCount(); i++ {
		c.Names[i] = cc.Name(i)
	}
	for i := 0; i < cc.LocationsCount(); i++ {
		c.Locations[i] = cc.LocationAt(i)
	}
	for i := 0; i < cc.ConstantsCount(); i++ {
		constant := cc.Constant(i)
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
		case *compiler.Function:
			c.Constants[i] = object.NewFunction(constant)
		case nil:
			c.Constants[i] = object.Nil
		default:
			panic(fmt.Sprintf("unsupported constant type: %T", constant))
		}
	}
	return c
}

func (c *code) GlobalsCount() int {
	return len(c.Globals)
}

// LocationAt returns the source location for the instruction at the given index.
func (c *code) LocationAt(ip int) errz.SourceLocation {
	if ip < 0 || ip >= len(c.Locations) {
		return errz.SourceLocation{}
	}
	return c.Locations[ip]
}

func loadChildCode(root *code, cc *compiler.Code) *code {
	c := wrapCode(cc)
	c.Globals = root.Globals
	return c
}

func loadRootCode(cc *compiler.Code, globals map[string]object.Object) *code {
	c := wrapCode(cc)
	globalNames := cc.GlobalNames()
	c.Globals = make([]object.Object, len(globalNames))
	for i, name := range globalNames {
		if value, found := globals[name]; found {
			c.Globals[i] = value
		}
	}
	return c
}
