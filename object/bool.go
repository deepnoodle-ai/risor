package object

import (
	"fmt"

	"github.com/risor-io/risor/op"
)

// Bool wraps bool and implements Object and Hashable interface.
type Bool struct {
	*base
	value bool
}

func (b *Bool) Type() Type {
	return BOOL
}

func (b *Bool) Value() bool {
	return b.value
}

func (b *Bool) Inspect() string {
	return fmt.Sprintf("%t", b.value)
}

func (b *Bool) HashKey() HashKey {
	var value int64
	if b.value {
		value = 1
	} else {
		value = 0
	}
	return HashKey{Type: b.Type(), IntValue: value}
}

func (b *Bool) Interface() interface{} {
	return b.value
}

func (b *Bool) String() string {
	return fmt.Sprintf("bool(%t)", b.value)
}

func (b *Bool) Compare(other Object) (int, error) {
	typeComp := CompareTypes(b, other)
	if typeComp != 0 {
		return typeComp, nil
	}
	otherBool := other.(*Bool)
	if b.value == otherBool.value {
		return 0, nil
	}
	if b.value {
		return 1, nil
	}
	return -1, nil
}

func (b *Bool) Equals(other Object) Object {
	if other.Type() == BOOL && b.value == other.(*Bool).value {
		return True
	}
	return False
}

func (b *Bool) IsTruthy() bool {
	return b.value
}

func (b *Bool) RunOperation(opType op.BinaryOpType, right Object) Object {
	return NewError(fmt.Errorf("eval error: unsupported operation for bool: %v", opType))
}

func NewBool(value bool) *Bool {
	if value {
		return True
	}
	return False
}

func Not(b *Bool) *Bool {
	if b.value {
		return False
	}
	return True
}

func Equals(a, b Object) bool {
	return a.Equals(b).(*Bool).value
}
