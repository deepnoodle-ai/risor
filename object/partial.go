package object

import (
	"fmt"
	"strings"

	"github.com/risor-io/risor/op"
)

// Partial is a partially applied function
type Partial struct {
	*base
	fn   Object
	args []Object
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
	return nil, fmt.Errorf("type error: unsupported operation for partial: %v", opType)
}

func (p *Partial) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("type error: unable to marshal partial")
}

func NewPartial(fn Object, args []Object) *Partial {
	return &Partial{
		fn:   fn,
		args: args,
	}
}
