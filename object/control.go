package object

import (
	"fmt"

	"github.com/cloudcmds/tamarin/v2/op"
)

// Control is used internally during evaluation of "break", "continue", and
// "return" statements.
type Control struct {
	keyword string // "break", "continue", or "return"
	value   Object // optional value associated with the control statement.
}

func (c *Control) Type() Type {
	return CONTROL
}

func (c *Control) Value() Object {
	return c.value
}

func (c *Control) Keyword() string {
	return c.keyword
}

func (c *Control) Inspect() string {
	return c.value.Inspect()
}

func (c *Control) String() string {
	if c.value == nil {
		return c.keyword
	}
	return fmt.Sprintf("%s(%s)", c.keyword, c.value)
}

func (c *Control) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (c *Control) Interface() interface{} {
	return c.value.Interface()
}

func (c *Control) Equals(other Object) Object {
	switch other := other.(type) {
	case *Control:
		if c.keyword != other.keyword {
			return False
		}
		return c.value.Equals(other.value)
	default:
		return False
	}
}

func (c *Control) IsTruthy() bool {
	return true
}

func (c *Control) RunOperation(opType op.BinaryOpType, right Object) Object {
	return NewError(fmt.Errorf("unsupported operation for control: %v", opType))
}

func NewReturn(value Object) *Control {
	if value == nil {
		value = Nil
	}
	return &Control{keyword: "return", value: value}
}

func NewBreak() *Control {
	return &Control{keyword: "break"}
}

func NewContinue() *Control {
	return &Control{keyword: "continue"}
}
