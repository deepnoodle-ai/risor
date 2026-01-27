package object

import (
	"fmt"

	"github.com/risor-io/risor/op"
)

// Internal: do not use. Cell is an implementation detail for closure variable capture.
type Cell struct {
	value *Object
}

func (c *Cell) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (c *Cell) SetAttr(name string, value Object) error {
	return TypeErrorf("type error: cell has no attribute %q", name)
}

func (c *Cell) IsTruthy() bool {
	return true
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

func (c *Cell) Equals(other Object) bool {
	otherCell, ok := other.(*Cell)
	if !ok {
		return false
	}
	return c == otherCell
}

func (c *Cell) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, fmt.Errorf("type error: unsupported operation for cell: %v", opType)
}

func (c *Cell) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("type error: unable to marshal cell")
}

func NewCell(value *Object) *Cell {
	return &Cell{value: value}
}
