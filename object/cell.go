package object

import (
	"fmt"

	"github.com/risor-io/risor/op"
)

type Cell struct {
	*base
	value *Object
}

func (c *Cell) Inspect() string {
	return c.String()
}

func (c *Cell) String() string {
	if c.value == nil {
		return "cell()"
	}
	return fmt.Sprintf("cell(%s)", *c.value)
}

func (c *Cell) Value() Object {
	if c.value == nil {
		return nil
	}
	return *c.value
}

func (c *Cell) Set(value Object) {
	*c.value = value
}

func (c *Cell) Type() Type {
	return CELL
}

func (c *Cell) Interface() interface{} {
	if c.value == nil {
		return nil
	}
	return (*c.value).Interface()
}

func (c *Cell) Equals(other Object) Object {
	if c == other {
		return True
	}
	return False
}

func (c *Cell) RunOperation(opType op.BinaryOpType, right Object) Object {
	return TypeErrorf("type error: unsupported operation for cell: %v", opType)
}

func (c *Cell) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("type error: unable to marshal cell")
}

func NewCell(value *Object) *Cell {
	return &Cell{value: value}
}
