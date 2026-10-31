package object

import (
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// Internal: do not use. Cell is an implementation detail for closure variable capture.
type Cell struct {
	value *Object
}

func (c *Cell) Attrs() []AttrSpec {
	return nil
}

func (c *Cell) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (c *Cell) SetAttr(name string, value Object) error {
	return TypeErrorf("cell has no attribute %q", name)
}

func (c *Cell) IsTruthy() bool {
	return true
}

// Inspect returns a string representation of the cell and its value. A
// container that is already being printed (a cycle), such as the cell itself,
// or that is nested more deeply than MaxNestingDepth, is shown as a
// placeholder.
func (c *Cell) Inspect() string {
	return c.inspect(nil)
}

func (c *Cell) inspect(path nestPath[Object]) string {
	if c.value == nil {
		return "cell()"
	}
	value := *c.value
	if isNestedContainer(value) {
		p, err := path.descend(c, value)
		if err != nil {
			return "cell(" + inspectPlaceholder(value) + ")"
		}
		path = p
	}
	return "cell(" + inspectNested(value, path) + ")"
}

func (c *Cell) String() string {
	return c.Inspect()
}

func (c *Cell) Value() Object {
	if c.value == nil {
		return nil
	}
	return *c.value
}

func (c *Cell) Set(value Object) {
	if c.value == nil {
		return
	}
	*c.value = value
}

// Move copies the cell's value to dst, then makes the cell read and write
// dst instead of its previous location.
func (c *Cell) Move(dst *Object) {
	if c.value != nil {
		*dst = *c.value
	}
	c.value = dst
}

func (c *Cell) Type() Type {
	return CELL
}

// Interface returns the Go value of the cell's value. It returns nil if the
// cell is empty, or if its value contains the cell or is too large to
// convert; see ToInterface.
func (c *Cell) Interface() interface{} {
	return interfaceOrNil(c)
}

func (c *Cell) Equals(other Object) bool {
	otherCell, ok := other.(*Cell)
	if !ok {
		return false
	}
	return c == otherCell
}

func (c *Cell) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for cell: %v", opType)
}

func (c *Cell) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("unable to marshal cell")
}

func NewCell(value *Object) *Cell {
	return &Cell{value: value}
}
