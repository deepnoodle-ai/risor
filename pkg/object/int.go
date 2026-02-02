package object

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

// Int wraps int64 and implements Object and Hashable interfaces.
// Int is immutable: the value is set at construction and cannot be changed.
type Int struct {
	value int64
}

func (i *Int) Attrs() []AttrSpec {
	return nil
}

func (i *Int) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (i *Int) SetAttr(name string, value Object) error {
	return TypeErrorf("int has no attribute %q", name)
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
		return 0, TypeErrorf("unable to compare int and %s", other.Type())
	}
}

func (i *Int) Equals(other Object) bool {
	switch other := other.(type) {
	case *Int:
		return i.value == other.value
	case *Float:
		return float64(i.value) == other.value
	case *Byte:
		return i.value == int64(other.value)
	}
	return false
}

func (i *Int) IsTruthy() bool {
	return i.value != 0
}

func (i *Int) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	switch right := right.(type) {
	case *Int:
		return i.runOperationInt(opType, right.value)
	case *Float:
		return i.runOperationFloat(opType, right.value)
	case *Byte:
		rightInt := int64(right.value)
		return i.runOperationInt(opType, rightInt)
	default:
		return nil, newTypeErrorf("unsupported operation for int: %v on type %s", opType, right.Type())
	}
}

func (i *Int) runOperationInt(opType op.BinaryOpType, right int64) (Object, error) {
	switch opType {
	case op.Add:
		return NewInt(i.value + right), nil
	case op.Subtract:
		return NewInt(i.value - right), nil
	case op.Multiply:
		return NewInt(i.value * right), nil
	case op.Divide:
		if right == 0 {
			return nil, newValueErrorf("division by zero")
		}
		return NewInt(i.value / right), nil
	case op.Modulo:
		if right == 0 {
			return nil, newValueErrorf("division by zero")
		}
		return NewInt(i.value % right), nil
	case op.Xor:
		return NewInt(i.value ^ right), nil
	case op.Power:
		return NewInt(int64(math.Pow(float64(i.value), float64(right)))), nil
	case op.LShift:
		return NewInt(i.value << uint(right)), nil
	case op.RShift:
		return NewInt(i.value >> uint(right)), nil
	case op.BitwiseAnd:
		return NewInt(i.value & right), nil
	case op.BitwiseOr:
		return NewInt(i.value | right), nil
	default:
		return nil, newTypeErrorf("unsupported operation for int: %v on type int", opType)
	}
}

func (i *Int) runOperationFloat(opType op.BinaryOpType, right float64) (Object, error) {
	iValue := float64(i.value)
	switch opType {
	case op.Add:
		return NewFloat(iValue + right), nil
	case op.Subtract:
		return NewFloat(iValue - right), nil
	case op.Multiply:
		return NewFloat(iValue * right), nil
	case op.Divide:
		return NewFloat(iValue / right), nil
	case op.Power:
		return NewInt(int64(math.Pow(float64(i.value), float64(right)))), nil
	default:
		return nil, newTypeErrorf("unsupported operation for int: %v on type float", opType)
	}
}

func (i *Int) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

// NewInt returns an *Int for the given value. Small integers (-10 to 255)
// are returned from a pre-allocated cache, so the same pointer may be
// returned for equal values. This is safe because Int is immutable.
func NewInt(value int64) *Int {
	if value >= 0 && value < positiveCacheSize {
		return positiveCache[value]
	}
	if value < 0 && value >= -negativeCacheSize {
		return negativeCache[-value-1]
	}
	return &Int{value: value}
}

// Int caches hold pre-allocated Int objects for small integers.
// The caches are initialized once at package load time and are read-only
// thereafter, making them safe for concurrent use across multiple VMs.
const (
	positiveCacheSize = 256 // 0 to 255
	negativeCacheSize = 10  // -1 to -10
)

var (
	positiveCache []*Int
	negativeCache []*Int
)

func init() {
	positiveCache = make([]*Int, positiveCacheSize)
	for i := range positiveCacheSize {
		positiveCache[i] = &Int{value: int64(i)}
	}
	negativeCache = make([]*Int, negativeCacheSize)
	for i := range negativeCacheSize {
		negativeCache[i] = &Int{value: int64(-i - 1)}
	}
}
