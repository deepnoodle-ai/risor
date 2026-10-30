package object

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
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
		return compareFloats(f.value, other.value), nil
	case *Int:
		return -compareIntFloat(other.value, f.value), nil
	case *Byte:
		return -compareIntFloat(int64(other.value), f.value), nil
	default:
		return 0, TypeErrorf("unable to compare float and %s", other.Type())
	}
}

func (f *Float) Equals(other Object) bool {
	switch other := other.(type) {
	case *Int:
		return !math.IsNaN(f.value) && compareIntFloat(other.value, f.value) == 0
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

// compareFloats orders a and b like Compare: NaN sorts after every other
// value and equal to itself, so sorting is well defined. The comparison
// operators do not use this order for NaN; see isUnordered.
func compareFloats(a, b float64) int {
	aNaN, bNaN := math.IsNaN(a), math.IsNaN(b)
	switch {
	case aNaN && bNaN:
		return 0
	case aNaN:
		return 1
	case bNaN:
		return -1
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// compareIntFloat orders i and f exactly, without converting i to float64,
// which would round integers above 2^53. NaN sorts after every integer.
func compareIntFloat(i int64, f float64) int {
	switch {
	case math.IsNaN(f):
		return -1
	case f >= 1<<63:
		return -1
	case f < -(1 << 63):
		return 1
	}
	// f is within int64 range, so its integer part converts exactly
	t := math.Trunc(f)
	ti := int64(t)
	switch {
	case i < ti:
		return -1
	case i > ti:
		return 1
	case f > t:
		return -1
	case f < t:
		return 1
	}
	return 0
}

func NewFloat(value float64) *Float {
	return &Float{value: value}
}
