package object

import (
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

type NilType struct{}

func (n *NilType) Attrs() []AttrSpec {
	return nil
}

func (n *NilType) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (n *NilType) SetAttr(name string, value Object) error {
	return TypeErrorf("nil has no attribute %q", name)
}

func (n *NilType) Type() Type {
	return NIL
}

func (n *NilType) Inspect() string {
	return "nil"
}

func (n *NilType) String() string {
	return "nil"
}

func (n *NilType) Interface() interface{} {
	return nil
}

func (n *NilType) Compare(other Object) (int, error) {
	if _, ok := other.(*NilType); ok {
		return 0, nil
	}
	return 0, TypeErrorf("unable to compare nil and %s", other.Type())
}

func (n *NilType) Equals(other Object) bool {
	_, ok := other.(*NilType)
	return ok
}

func (n *NilType) IsTruthy() bool {
	return false
}

func (n *NilType) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

func (n *NilType) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for nil: %v", opType)
}
