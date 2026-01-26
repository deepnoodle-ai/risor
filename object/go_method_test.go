package object

import (
	"reflect"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

type fooStruct struct{}

func (f *fooStruct) Inc(x int) error {
	return nil
}

func TestGoMethod(t *testing.T) {
	f := &fooStruct{}
	typ := reflect.TypeOf(f)

	m, ok := typ.MethodByName("Inc")
	assert.True(t, ok)

	method, err := newGoMethod(typ, m)
	assert.Nil(t, err)

	assert.Equal(t, method.Name(), "Inc")
	assert.Equal(t, method.NumIn(), 2)
	assert.Equal(t, method.NumOut(), 1)

	in1 := method.InType(1)
	assert.Equal(t, in1.Name(), "int")

	out0 := method.OutType(0)
	assert.Equal(t, out0.Name(), "error")
}

type reflectService struct{}

func (svc *reflectService) Test() *reflect.Value {
	return nil
}

func TestGoMethodError(t *testing.T) {
	svc := &reflectService{}
	typ := reflect.TypeOf(svc)

	m, ok := typ.MethodByName("Test")
	assert.True(t, ok)

	_, err := newGoMethod(typ, m)
	assert.NotNil(t, err)

	expectedErr := `type error: (*object.reflectService).Test has input parameter of type *object.reflectService; 
(*object.reflectService).Test has output parameter of type *reflect.Value; 
(*reflect.Value).CanConvert has input parameter of type reflect.Type; 
(reflect.Type).Field has output parameter of type reflect.StructField; 
unsupported kind: uintptr`

	assert.Equal(t, err.Error(), expectedErr)
}
