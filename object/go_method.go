package object

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/risor-io/risor/op"
)

// GoMethod represents a single method on a Go type. This exposes the method to
// Risor for reflection and proxying.
type GoMethod struct {
	*base
	method             reflect.Method
	inputTypes         []*GoType
	outputTypes        []*GoType
	name               *String
	numIn              *Int
	numOut             *Int
	producesErr        bool
	errorIndices       []int
	outputIsError      []bool
	hasPointerReceiver bool
}

func (m *GoMethod) Type() Type {
	return GO_METHOD
}

func (m *GoMethod) Inspect() string {
	return fmt.Sprintf("go_method(%s)", m.Name())
}

func (m *GoMethod) Interface() interface{} {
	return m.method
}

func (m *GoMethod) Equals(other Object) Object {
	if m == other {
		return True
	}
	return False
}

func (m *GoMethod) GetAttr(name string) (Object, bool) {
	switch name {
	case "name":
		return m.name, true
	case "num_in":
		return m.numIn, true
	case "num_out":
		return m.numOut, true
	case "error_indices":
		result := make([]Object, len(m.errorIndices))
		for i, index := range m.errorIndices {
			result[i] = NewInt(int64(index))
		}
		return NewList(result), true
	case "in_type":
		return &Builtin{
			name: "go_method.in_type",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("go_method.in_type", 1, len(args))
				}
				index, ok := args[0].(*Int)
				if !ok {
					return TypeErrorf("type error: go_method.in_type expected integer (%s given)", args[0].Type())
				}
				if index.value < 0 || index.value >= int64(m.NumIn()) {
					return Errorf("value error: go_method.in_type index out of range [0, %d] (%d given)",
						m.NumIn()-1, index.value)
				}
				return m.inputTypes[index.value]
			},
		}, true
	case "out_type":
		return &Builtin{
			name: "go_method.out_type",
			fn: func(ctx context.Context, args ...Object) Object {
				if len(args) != 1 {
					return NewArgsError("go_method.out_type", 1, len(args))
				}
				index, ok := args[0].(*Int)
				if !ok {
					return TypeErrorf("type error: go_method.out_type expected integer (%s given)", args[0].Type())
				}
				if index.value < 0 || index.value >= int64(m.NumIn()) {
					return Errorf("value error: go_method.out_type index out of range [0, %d] (%d given)",
						m.NumOut()-1, index.value)
				}
				return m.outputTypes[index.value]
			},
		}, true
	}
	return nil, false
}

func (m *GoMethod) IsTruthy() bool {
	return true
}

func (m *GoMethod) RunOperation(opType op.BinaryOpType, right Object) Object {
	return TypeErrorf("type error: unsupported operation on go_method (%s)", opType)
}

func (m *GoMethod) Name() string {
	return m.method.Name
}

func (m *GoMethod) NumIn() int {
	return m.method.Type.NumIn()
}

func (m *GoMethod) NumOut() int {
	return m.method.Type.NumOut()
}

func (m *GoMethod) HasPointerReceiver() bool {
	return m.hasPointerReceiver
}

func (m *GoMethod) InType(i int) *GoType {
	return m.inputTypes[i]
}

func (m *GoMethod) OutType(i int) *GoType {
	return m.outputTypes[i]
}

func (m *GoMethod) ProducesError() bool {
	return m.producesErr
}

func (m *GoMethod) ErrorIndices() []int {
	return m.errorIndices
}

func (m *GoMethod) IsOutputError(index int) bool {
	return m.outputIsError[index]
}

func (m *GoMethod) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name   string `json:"name"`
		NumIn  int64  `json:"num_in"`
		NumOut int64  `json:"num_out"`
	}{
		Name:   m.name.value,
		NumIn:  m.numIn.value,
		NumOut: m.numOut.value,
	})
}

// Returns a Risor *GoMethod Object that represents the given Go method.
// This aids in calling Go methods from Risor.
func newGoMethod(typ reflect.Type, m reflect.Method) (*GoMethod, error) {
	numIn := m.Type.NumIn()
	numOut := m.Type.NumOut()

	// name, numIn, numOut are immutable so we can create Risor objects
	// for them once and reuse them to avoid repeated allocations later
	method := &GoMethod{
		method: m,
		name:   NewString(m.Name),
		numIn:  NewInt(int64(numIn)),
		numOut: NewInt(int64(numOut)),
	}

	// Store the input type for each explicit input argument
	method.inputTypes = make([]*GoType, numIn)
	for i := 0; i < numIn; i++ {
		inputType := m.Type.In(i)
		if i == 0 && inputType.Kind() == reflect.Ptr {
			method.hasPointerReceiver = true
		}
		inputGoType, err := newGoType(inputType)
		if err != nil {
			errMsg := strings.TrimPrefix(err.Error(), "type error: ")
			return nil, TypeErrorf("type error: (%s).%s has input parameter of type %s; \n%s",
				typ, m.Name, inputType, errMsg)
		}
		method.inputTypes[i] = inputGoType
	}

	// Store the output type for each method output, taking note of whether an
	// error is produced and the index of the first error
	method.outputTypes = make([]*GoType, numOut)
	method.outputIsError = make([]bool, numOut)
	for i := 0; i < numOut; i++ {
		outputType := m.Type.Out(i)
		outputGoType, err := newGoType(outputType)
		if err != nil {
			errMsg := strings.TrimPrefix(err.Error(), "type error: ")
			return nil, TypeErrorf("type error: (%s).%s has output parameter of type %s; \n%s",
				typ, m.Name, outputType, errMsg)
		}
		method.outputTypes[i] = outputGoType
		if outputType.Implements(errorInterface) {
			method.producesErr = true
			method.errorIndices = append(method.errorIndices, i)
			method.outputIsError[i] = true
		} else {
			method.outputIsError[i] = false
		}
	}
	return method, nil
}

func getMethods(typ reflect.Type) (map[string]*GoMethod, error) {
	count := typ.NumMethod()
	methods := make(map[string]*GoMethod, count)
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}
		goMethod, err := newGoMethod(typ, method)
		if err != nil {
			return nil, err
		}
		methods[method.Name] = goMethod
	}
	return methods, nil
}
