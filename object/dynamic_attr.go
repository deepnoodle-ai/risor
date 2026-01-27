package object

import (
	"context"
	"errors"
	"fmt"

	"github.com/risor-io/risor/op"
)

// DynamicAttr is an Object that represents an attribute that can be dynamically
// resolved to a concrete Object at runtime.
type DynamicAttr struct {
	name  string
	value Object
	fn    ResolveAttrFunc
}

func (d *DynamicAttr) Inspect() string {
	return fmt.Sprintf("dynamic_attr(%s)", d.name)
}

func (d *DynamicAttr) Type() Type {
	return DYNAMIC_ATTR
}

func (d *DynamicAttr) Interface() interface{} {
	return d.value
}

func (d *DynamicAttr) String() string {
	return d.Inspect()
}

func (d *DynamicAttr) Equals(other Object) bool {
	otherDynamic, ok := other.(*DynamicAttr)
	if !ok {
		return false
	}
	return d == otherDynamic
}

func (d *DynamicAttr) IsTruthy() bool {
	return d.value != nil
}

func (d *DynamicAttr) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, fmt.Errorf("type error: unsupported operation for dynamic_attr: %v", opType)
}

func (d *DynamicAttr) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal error: unable to marshal dynamic_attr")
}

func (d *DynamicAttr) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (d *DynamicAttr) SetAttr(name string, value Object) error {
	return TypeErrorf("type error: unable to set attribute on dynamic_attr")
}


func (d *DynamicAttr) ResolveAttr(ctx context.Context, name string) (Object, error) {
	if d.value != nil {
		return d.value, nil
	}
	attr, err := d.fn(ctx, name)
	if err != nil {
		return nil, err
	}
	d.value = attr
	return attr, nil
}

func NewDynamicAttr(name string, fn ResolveAttrFunc) *DynamicAttr {
	return &DynamicAttr{name: name, fn: fn}
}
