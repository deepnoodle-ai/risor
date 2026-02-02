package object

import (
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// Partial is a partially applied function
type Partial struct {
	fn   Object
	args []Object
}

func (p *Partial) Attrs() []AttrSpec {
	return nil
}

func (p *Partial) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (p *Partial) SetAttr(name string, value Object) error {
	return TypeErrorf("partial has no attribute %q", name)
}

func (p *Partial) IsTruthy() bool {
	return true
}

func (p *Partial) Function() Object {
	return p.fn
}

func (p *Partial) Args() []Object {
	return p.args
}

func (p *Partial) Type() Type {
	return PARTIAL
}

func (p *Partial) Inspect() string {
	var args []string
	for _, arg := range p.args {
		args = append(args, arg.Inspect())
	}
	return fmt.Sprintf("partial(%s, %s)", p.fn.Inspect(), strings.Join(args, ", "))
}

func (p *Partial) Interface() interface{} {
	return p.fn
}

func (p *Partial) Equals(other Object) bool {
	otherPartial, ok := other.(*Partial)
	if !ok {
		return false
	}
	return p == otherPartial
}

func (p *Partial) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for partial: %v", opType)
}

func (p *Partial) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("unable to marshal partial")
}

func NewPartial(fn Object, args []Object) *Partial {
	return &Partial{
		fn:   fn,
		args: args,
	}
}
