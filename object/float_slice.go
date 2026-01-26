package object

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/risor-io/risor/op"
)

type FloatSlice struct {
	*base
	value []float64
}

func (f *FloatSlice) Inspect() string {
	return fmt.Sprintf("float_slice(%v)", f.value)
}

func (f *FloatSlice) Type() Type {
	return FLOAT_SLICE
}

func (f *FloatSlice) Value() []float64 {
	return f.value
}

func (f *FloatSlice) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (f *FloatSlice) Interface() interface{} {
	return f.value
}

func (f *FloatSlice) String() string {
	return f.Inspect()
}

func (f *FloatSlice) Equals(other Object) Object {
	if f == other {
		return True
	}
	return False
}

func (f *FloatSlice) IsTruthy() bool {
	return len(f.value) > 0
}

func (f *FloatSlice) RunOperation(opType op.BinaryOpType, right Object) Object {
	return TypeErrorf("type error: unsupported operation for float_slice: %v on type %s", opType, right.Type())
}

func (f *FloatSlice) Contains(item Object) *Bool {
	value, err := AsFloat(item)
	if err != nil {
		return False
	}
	for _, v := range f.value {
		if v == value {
			return True
		}
	}
	return False
}

func (f *FloatSlice) GetItem(key Object) (Object, *Error) {
	indexObj, ok := key.(*Int)
	if !ok {
		return nil, Errorf("index error: float_slice index must be an int (got %s)", key.Type())
	}
	index, err := ResolveIndex(indexObj.value, int64(len(f.value)))
	if err != nil {
		return nil, NewError(err)
	}
	return NewFloat(f.value[index]), nil
}

func (f *FloatSlice) GetSlice(slice Slice) (Object, *Error) {
	start, stop, err := ResolveIntSlice(slice, int64(len(f.value)))
	if err != nil {
		return nil, NewError(err)
	}
	return NewFloatSlice(f.value[start:stop]), nil
}

func (f *FloatSlice) SetItem(key, value Object) *Error {
	indexObj, ok := key.(*Int)
	if !ok {
		return Errorf("index error: index must be an int (got %s)", key.Type())
	}
	index, err := ResolveIndex(indexObj.value, int64(len(f.value)))
	if err != nil {
		return NewError(err)
	}
	floatVal, convErr := AsFloat(value)
	if convErr != nil {
		return convErr
	}
	f.value[index] = floatVal
	return nil
}

func (f *FloatSlice) DelItem(key Object) *Error {
	return Errorf("type error: cannot delete from float_slice")
}

func (f *FloatSlice) Len() *Int {
	return NewInt(int64(len(f.value)))
}

func (f *FloatSlice) Enumerate(ctx context.Context, fn func(key, value Object) bool) {
	for i, v := range f.value {
		if !fn(NewInt(int64(i)), NewFloat(v)) {
			return
		}
	}
}

func (f *FloatSlice) Clone() *FloatSlice {
	value := make([]float64, len(f.value))
	copy(value, f.value)
	return NewFloatSlice(value)
}

func (f *FloatSlice) Integers() []Object {
	result := make([]Object, len(f.value))
	for i, v := range f.value {
		result[i] = NewInt(int64(v))
	}
	return result
}

func (f *FloatSlice) Cost() int {
	return len(f.value)
}

func (f *FloatSlice) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.value)
}

func NewFloatSlice(value []float64) *FloatSlice {
	return &FloatSlice{value: value}
}
