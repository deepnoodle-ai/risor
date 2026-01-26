package object

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/op"
)

// Int wraps int64 and implements Object and Hashable interfaces.
type Int struct {
	*base
	value int64
}

func (i *Int) Inspect() string {
	return fmt.Sprintf("%d", i.value)
}

func (i *Int) Type() Type {
	return INT
}

func (i *Int) Value() int64 {
	return i.value
}

func (i *Int) HashKey() HashKey {
	return HashKey{Type: i.Type(), IntValue: i.value}
}

func (i *Int) Interface() interface{} {
	return i.value
}

func (i *Int) String() string {
	return i.Inspect()
}

func (i *Int) Compare(other Object) (int, error) {
	switch other := other.(type) {
	case *Float:
		thisFloat := float64(i.value)
		if thisFloat == other.value {
			return 0, nil
		}
		if thisFloat > other.value {
			return 1, nil
		}
		return -1, nil
	case *Int:
		if i.value == other.value {
			return 0, nil
		}
		if i.value > other.value {
			return 1, nil
		}
		return -1, nil
	case *Byte:
		if i.value == int64(other.value) {
			return 0, nil
		}
		if i.value > int64(other.value) {
			return 1, nil
		}
		return -1, nil
	default:
		return 0, errz.TypeErrorf("type error: unable to compare int and %s", other.Type())
	}
}

func (i *Int) Equals(other Object) Object {
	switch other := other.(type) {
	case *Int:
		if i.value == other.value {
			return True
		}
	case *Float:
		if float64(i.value) == other.value {
			return True
		}
	case *Byte:
		if i.value == int64(other.value) {
			return True
		}
	}
	return False
}

func (i *Int) IsTruthy() bool {
	return i.value != 0
}

func (i *Int) RunOperation(opType op.BinaryOpType, right Object) Object {
	switch right := right.(type) {
	case *Int:
		return i.runOperationInt(opType, right.value)
	case *Float:
		return i.runOperationFloat(opType, right.value)
	case *Byte:
		rightInt := int64(right.value)
		return i.runOperationInt(opType, rightInt)
	default:
		return TypeErrorf("type error: unsupported operation for int: %v on type %s", opType, right.Type())
	}
}

func (i *Int) runOperationInt(opType op.BinaryOpType, right int64) Object {
	switch opType {
	case op.Add:
		return NewInt(i.value + right)
	case op.Subtract:
		return NewInt(i.value - right)
	case op.Multiply:
		return NewInt(i.value * right)
	case op.Divide:
		return NewInt(i.value / right)
	case op.Modulo:
		return NewInt(i.value % right)
	case op.Xor:
		return NewInt(i.value ^ right)
	case op.Power:
		return NewInt(int64(math.Pow(float64(i.value), float64(right))))
	case op.LShift:
		return NewInt(i.value << uint(right))
	case op.RShift:
		return NewInt(i.value >> uint(right))
	case op.BitwiseAnd:
		return NewInt(i.value & right)
	case op.BitwiseOr:
		return NewInt(i.value | right)
	default:
		return TypeErrorf("type error: unsupported operation for int: %v on type int", opType)
	}
}

func (i *Int) runOperationFloat(opType op.BinaryOpType, right float64) Object {
	iValue := float64(i.value)
	switch opType {
	case op.Add:
		return NewFloat(iValue + right)
	case op.Subtract:
		return NewFloat(iValue - right)
	case op.Multiply:
		return NewFloat(iValue * right)
	case op.Divide:
		return NewFloat(iValue / right)
	case op.Power:
		return NewInt(int64(math.Pow(float64(i.value), float64(right))))
	default:
		return TypeErrorf("type error: unsupported operation for int: %v on type float", opType)
	}
}

func (i *Int) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

func (i *Int) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	if i.value >= 0 {
		for j := int64(0); j < i.value; j++ {
			if !fn(NewInt(j), NewInt(j)) {
				return
			}
		}
	} else {
		for j := int64(0); j > i.value; j-- {
			if !fn(NewInt(-j), NewInt(j)) {
				return
			}
		}
	}
}

func NewInt(value int64) *Int {
	if value >= 0 && value < tableSize {
		return intCache[value]
	}
	return &Int{value: value}
}

const tableSize = 256

var intCache = []*Int{}

func init() {
	intCache = make([]*Int, tableSize)
	for i := 0; i < tableSize; i++ {
		intCache[i] = &Int{value: int64(i)}
	}
}
