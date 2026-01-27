package object

import (
	"fmt"
	"math"

	"github.com/risor-io/risor/op"
)

// Byte wraps byte and implements Object and Hashable interface.
type Byte struct {
	*base
	value byte
}

func (b *Byte) Type() Type {
	return BYTE
}

func (b *Byte) Value() byte {
	return b.value
}

func (b *Byte) Inspect() string {
	return fmt.Sprintf("%d", b.value)
}

func (b *Byte) HashKey() HashKey {
	return HashKey{Type: b.Type(), IntValue: int64(b.value)}
}

func (b *Byte) Interface() interface{} {
	return b.value
}

func (b *Byte) String() string {
	return b.Inspect()
}

func (b *Byte) Compare(other Object) (int, error) {
	switch other := other.(type) {
	case *Float:
		thisFloat := float64(b.value)
		if thisFloat == other.value {
			return 0, nil
		}
		if thisFloat > other.value {
			return 1, nil
		}
		return -1, nil
	case *Int:
		thisInt := int64(b.value)
		if thisInt == other.value {
			return 0, nil
		}
		if thisInt > other.value {
			return 1, nil
		}
		return -1, nil
	case *Byte:
		if b.value == other.value {
			return 0, nil
		}
		if b.value > other.value {
			return 1, nil
		}
		return -1, nil
	default:
		return 0, TypeErrorf("type error: unable to compare byte and %s", other.Type())
	}
}

func (b *Byte) Equals(other Object) bool {
	switch other := other.(type) {
	case *Byte:
		return b.value == other.value
	case *Int:
		return int64(b.value) == other.value
	case *Float:
		return float64(b.value) == other.value
	}
	return false
}

func (b *Byte) IsTruthy() bool {
	return b.value > 0
}

func (b *Byte) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	switch right := right.(type) {
	case *Byte:
		return b.runOperationByte(opType, right.value)
	case *Int:
		return b.runOperationInt(opType, right.value)
	case *Float:
		return b.runOperationFloat(opType, right.value)
	default:
		return nil, fmt.Errorf("type error: unsupported operation for byte: %v on type %s", opType, right.Type())
	}
}

func (b *Byte) runOperationByte(opType op.BinaryOpType, right byte) (Object, error) {
	switch opType {
	case op.Add:
		return NewByte(b.value + right), nil
	case op.Subtract:
		return NewByte(b.value - right), nil
	case op.Multiply:
		return NewByte(b.value * right), nil
	case op.Divide:
		return NewByte(b.value / right), nil
	case op.Modulo:
		return NewByte(b.value % right), nil
	case op.Xor:
		return NewByte(b.value ^ right), nil
	case op.Power:
		return NewByte(byte(math.Pow(float64(b.value), float64(right)))), nil
	case op.LShift:
		return NewByte(b.value << right), nil
	case op.RShift:
		return NewByte(b.value >> right), nil
	case op.BitwiseAnd:
		return NewByte(b.value & right), nil
	case op.BitwiseOr:
		return NewByte(b.value | right), nil
	default:
		return nil, fmt.Errorf("type error: unsupported operation for byte: %v on type byte", opType)
	}
}

func (b *Byte) runOperationInt(opType op.BinaryOpType, right int64) (Object, error) {
	switch opType {
	case op.Add:
		return NewInt(int64(b.value) + right), nil
	case op.Subtract:
		return NewInt(int64(b.value) - right), nil
	case op.Multiply:
		return NewInt(int64(b.value) * right), nil
	case op.Divide:
		return NewInt(int64(b.value) / right), nil
	case op.Modulo:
		return NewInt(int64(b.value) % right), nil
	case op.Xor:
		return NewInt(int64(b.value) ^ right), nil
	case op.Power:
		return NewInt(int64(math.Pow(float64(b.value), float64(right)))), nil
	case op.LShift:
		return NewInt(int64(b.value) << right), nil
	case op.RShift:
		return NewInt(int64(b.value) >> right), nil
	case op.BitwiseAnd:
		return NewInt(int64(b.value) & right), nil
	case op.BitwiseOr:
		return NewInt(int64(b.value) | right), nil
	default:
		return nil, fmt.Errorf("type error: unsupported operation for byte: %v on type int", opType)
	}
}

func (b *Byte) runOperationFloat(opType op.BinaryOpType, right float64) (Object, error) {
	switch opType {
	case op.Add:
		return NewFloat(float64(b.value) + right), nil
	case op.Subtract:
		return NewFloat(float64(b.value) - right), nil
	case op.Multiply:
		return NewFloat(float64(b.value) * right), nil
	case op.Divide:
		return NewFloat(float64(b.value) / right), nil
	case op.Power:
		return NewFloat(math.Pow(float64(b.value), right)), nil
	default:
		return nil, fmt.Errorf("type error: unsupported operation for byte: %v on type float", opType)
	}
}

func (b *Byte) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%d", b.value)), nil
}

func NewByte(value byte) *Byte {
	return byteCache[value]
}

var byteCache = []*Byte{}

func init() {
	byteCache = make([]*Byte, 256)
	for i := 0; i < 256; i++ {
		byteCache[i] = &Byte{value: byte(i)}
	}
}
