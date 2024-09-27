package object

import (
	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/op"
)

type NilType struct {
	*base
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

func (n *NilType) HashKey() HashKey {
	return HashKey{Type: n.Type()}
}

func (n *NilType) Compare(other Object) (int, error) {
	if _, ok := other.(*NilType); ok {
		return 0, nil
	}
	return 0, errz.TypeErrorf("type error: unable to compare nil and %s", other.Type())
}

func (n *NilType) Equals(other Object) Object {
	if other.Type() == NIL {
		return True
	}
	return False
}

func (n *NilType) IsTruthy() bool {
	return false
}

func (n *NilType) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

func (n *NilType) RunOperation(opType op.BinaryOpType, right Object) Object {
	return TypeErrorf("type error: unsupported operation for nil: %v", opType)
}
