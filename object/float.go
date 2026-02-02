package object

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/deepnoodle-ai/risor/v2/op"
)

// Float wraps float64 and implements Object and Hashable interfaces.
type Float struct {
	value float64
}

func (f *Float) Attrs() []AttrSpec {
	return nil
}

func (f *Float) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (f *Float) SetAttr(name string, value Object) error {
	return TypeErrorf("float has no attribute %q", name)
}

func (f *Float) Inspect() string {
	return strconv.FormatFloat(f.value, 'f', -1, 64)
}

func (f *Float) Type() Type {
	return FLOAT
}

func (f *Float) Value() float64 {
	return f.value
}

func (f *Float) Interface() interface{} {
	return f.value
}

func (f *Float) String() string {
	return f.Inspect()
}

func (f *Float) Compare(other Object) (int, error) {
	switch other := other.(type) {
	case *Float:
		if f.value == other.value {
			return 0, nil
		}
		if f.value > other.value {
			return 1, nil
		}
		return -1, nil
	case *Int:
		otherFloat := float64(other.value)
		if f.value == otherFloat {
			return 0, nil
		}
		if f.value > otherFloat {
			return 1, nil
		}
		return -1, nil
	case *Byte:
		otherFloat := float64(other.value)
		if f.value == otherFloat {
			return 0, nil
		}
		if f.value > otherFloat {
			return 1, nil
		}
		return -1, nil
	default:
		return 0, TypeErrorf("unable to compare float and %s", other.Type())
	}
}

func (f *Float) Equals(other Object) bool {
	switch other := other.(type) {
	case *Int:
		return f.value == float64(other.value)
	case *Float:
		return f.value == other.value
	case *Byte:
		return f.value == float64(other.value)
	}
	return false
}

func (f *Float) IsTruthy() bool {
	return f.value != 0.0
}

func (f *Float) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	switch right := right.(type) {
	case *Int:
		return f.runOperationFloat(opType, float64(right.value))
	case *Float:
		return f.runOperationFloat(opType, right.value)
	case *Byte:
		rightFloat := float64(right.value)
		return f.runOperationFloat(opType, rightFloat)
	default:
		return nil, newTypeErrorf("unsupported operation for float: %v on type %s", opType, right.Type())
	}
}

func (f *Float) runOperationFloat(opType op.BinaryOpType, right float64) (Object, error) {
	switch opType {
	case op.Add:
		return NewFloat(f.value + right), nil
	case op.Subtract:
		return NewFloat(f.value - right), nil
	case op.Multiply:
		return NewFloat(f.value * right), nil
	case op.Divide:
		return NewFloat(f.value / right), nil
	case op.Power:
		return NewFloat(math.Pow(f.value, right)), nil
	default:
		return nil, newTypeErrorf("unsupported operation for float: %v", opType)
	}
}

func (f *Float) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.value)
}

func NewFloat(value float64) *Float {
	return &Float{value: value}
}
