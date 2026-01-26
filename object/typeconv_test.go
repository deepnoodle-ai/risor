package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestFloat64Converter(t *testing.T) {
	c := Float64Converter{}

	f, err := c.From(2.0)
	assert.Nil(t, err)
	assert.Equal(t, f, NewFloat(2.0))

	v, err := c.To(NewFloat(3.0))
	assert.Nil(t, err)
	assert.Equal(t, v, 3.0)
}

func TestMapStringConverter(t *testing.T) {
	c, err := newMapConverter(reflect.TypeOf(""))
	assert.Nil(t, err)

	m := map[string]string{
		"a": "apple",
		"b": "banana",
	}

	tM, err := c.From(m)
	assert.Nil(t, err)
	assert.Equal(t,

		tM, NewMap(map[string]Object{
			"a": NewString("apple"),
			"b": NewString("banana"),
		}))

	gM, err := c.To(NewMap(map[string]Object{
		"c": NewString("cod"),
		"d": NewString("deer"),
	}))
	assert.Nil(t, err)
	assert.Equal(t,

		gM, map[string]string{
			"c": "cod",
			"d": "deer",
		})
}

func TestMapStringInterfaceConverter(t *testing.T) {
	c, err := newMapConverter(reflect.TypeOf(""))
	assert.Nil(t, err)

	m := map[string]string{
		"a": "apple",
		"b": "banana",
	}

	tM, err := c.From(m)
	assert.Nil(t, err)
	assert.Equal(t,

		tM, NewMap(map[string]Object{
			"a": NewString("apple"),
			"b": NewString("banana"),
		}))

	gM, err := c.To(NewMap(map[string]Object{
		"c": NewString("cod"),
		"d": NewString("deer"),
	}))
	assert.Nil(t, err)
	assert.Equal(t,

		gM, map[string]string{
			"c": "cod",
			"d": "deer",
		})
}

func TestPointerConverter(t *testing.T) {
	c, err := newPointerConverter(reflect.TypeOf(float64(0)))
	assert.Nil(t, err)

	v := 2.0
	vPtr := &v

	f, err := c.From(vPtr)
	assert.Nil(t, err)
	assert.Equal(t, f, NewFloat(2.0))

	// Convert one Risor Float to a *float64
	outVal1, err := c.To(NewFloat(3.0))
	assert.Nil(t, err)
	outValPtr1, ok := outVal1.(*float64)
	assert.True(t, ok)
	assert.Equal(t, *outValPtr1, 3.0)

	// Convert a second Risor Float to a *float64
	outVal2, err := c.To(NewFloat(4.0))
	assert.Nil(t, err)
	outValPtr2, ok := outVal2.(*float64)
	assert.True(t, ok)
	assert.Equal(t, *outValPtr2, 4.0)

	// Confirm the two pointers are different
	assert.Equal(t, *outValPtr1, 3.0)
	assert.Equal(t, *outValPtr2, 4.0)
}

func TestCreatingPointerViaReflect(t *testing.T) {
	v := 3.0
	var vInterface interface{} = v

	vPointer := reflect.New(reflect.TypeOf(vInterface))
	vPointer.Elem().Set(reflect.ValueOf(v))
	floatPointer := vPointer.Interface()

	result, ok := floatPointer.(*float64)
	assert.True(t, ok)
	assert.NotNil(t, result)
	assert.Equal(t, *result, 3.0)
	assert.Equal(t, result, &v)
}

func TestSetAttributeViaReflect(t *testing.T) {
	type test struct {
		A int
	}
	tStruct := test{A: 99}
	var tInterface interface{} = tStruct

	if reflect.TypeOf(tInterface).Kind() != reflect.Ptr {
		// Create a pointer to the value
		tInterfacePointer := reflect.New(reflect.TypeOf(tInterface))
		tInterfacePointer.Elem().Set(reflect.ValueOf(tInterface))
		tInterface = tInterfacePointer.Interface()
	}

	// Set the field "A"
	value := reflect.ValueOf(tInterface)
	value.Elem().FieldByName("A").Set(reflect.ValueOf(100))

	// Confirm the field was set
	assert.Equal(t, value.Elem().FieldByName("A").Interface(), 100)
}

func TestSliceConverter(t *testing.T) {
	c, err := newSliceConverter(reflect.TypeOf(0.0))
	assert.Nil(t, err)

	v := []float64{1.0, 2.0, 3.0}

	f, err := c.From(v)
	assert.Nil(t, err)
	assert.Equal(t,

		f, NewList([]Object{
			NewFloat(1.0),
			NewFloat(2.0),
			NewFloat(3.0),
		}))

	list := NewList([]Object{
		NewFloat(9.0),
		NewFloat(-8.0),
	})
	result, err := c.To(list)
	assert.Nil(t, err)
	assert.Equal(t, result, []float64{9.0, -8.0})
}

func TestStructConverter(t *testing.T) {
	type foo struct {
		A int
		B string
	}
	f := foo{A: 1, B: "two"}

	// Create a StructConverter for the type foo
	c, err := newStructConverter(reflect.TypeOf(f))
	assert.Nil(t, err)

	// "From" should wrap the foo in a Proxy. The Proxy will hold a copy of the
	// foo struct since it is a value type.
	proxyObj, err := c.From(f)
	assert.Nil(t, err)
	proxy, ok := proxyObj.(*Proxy)
	assert.True(t, ok)
	value, ok := proxy.GetAttr("A")
	assert.True(t, ok)
	assert.Equal(t, value, NewInt(1))
	value, ok = proxy.GetAttr("B")
	assert.True(t, ok)
	assert.Equal(t, value, NewString("two"))

	// Given a Proxy, "To" should unwrap it back to a foo struct
	fObj, err := c.To(proxyObj)
	assert.Nil(t, err)
	fCopy, ok := fObj.(foo)
	assert.True(t, ok)
	assert.Equal(t, fCopy, f)

	// Given a Map, "To" should unwrap it back to a foo struct
	fObj, err = c.To(NewMap(map[string]Object{
		"A": NewInt(1),
		"B": NewString("two"),
		"C": NewString("ignored"),
	}))
	assert.Nil(t, err)
	fCopy, ok = fObj.(foo)
	assert.True(t, ok)
	assert.Equal(t, fCopy, f)
}

func TestStructPointerConverter(t *testing.T) {
	type foo struct {
		A int
		B string
	}
	f := foo{A: 1, B: "two"}
	fPtr := &f

	// Create a StructConverter for the pointer type *foo
	c, err := newStructConverter(reflect.TypeOf(fPtr))
	assert.Nil(t, err)

	// "From" should wrap the *foo in a Proxy
	proxyObj, err := c.From(fPtr)
	assert.Nil(t, err)
	proxy, ok := proxyObj.(*Proxy)
	assert.True(t, ok)
	value, ok := proxy.GetAttr("A")
	assert.True(t, ok)
	assert.Equal(t, value, NewInt(1))
	value, ok = proxy.GetAttr("B")
	assert.True(t, ok)
	assert.Equal(t, value, NewString("two"))

	// Given a Proxy, "To" should unwrap it back to the exact same *foo pointer
	fObj, err := c.To(proxyObj)
	assert.Nil(t, err)
	fPtrCopy, ok := fObj.(*foo)
	assert.True(t, ok)
	assert.Equal(t, fPtrCopy, fPtr)

	// Given a Map, "To" should return a new *foo pointer, where the underlying
	// foo struct has the same values as the Map
	fObj, err = c.To(NewMap(map[string]Object{
		"A": NewInt(1),
		"B": NewString("two"),
		"C": NewString("ignored"),
	}))
	assert.Nil(t, err)
	fPtrCopy, ok = fObj.(*foo)
	assert.True(t, ok)
	assert.Equal(t, fPtrCopy, fPtr)
}

type testState struct {
	Count int
}

func (s *testState) GetCount() int {
	return s.Count
}

type testService struct {
	Name  string
	State testState
}

func (s *testService) GetName() string {
	return s.Name
}

func (s *testService) GetState() *testState {
	return &s.State
}

func TestNestedStructsConverter(t *testing.T) {
	svc := &testService{
		Name: "sauron",
		State: testState{
			Count: 42,
		},
	}

	// Create a StructConverter for the pointer type *testService
	c, err := newStructConverter(reflect.TypeOf(svc))
	assert.Nil(t, err)

	// "From" should wrap the *testService in a Proxy
	proxyObj, err := c.From(svc)
	assert.Nil(t, err)
	proxy, ok := proxyObj.(*Proxy)
	assert.True(t, ok)
	value, ok := proxy.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, value, NewString("sauron"))

	// Access the State attribute, which is a nested struct
	value, ok = proxy.GetAttr("State")
	assert.True(t, ok)
	stateProxy, ok := value.(*Proxy)
	assert.True(t, ok)
	value, ok = stateProxy.GetAttr("Count")
	assert.True(t, ok)
	assert.Equal(t, value, NewInt(42))

	// Access the GetState method
	value, ok = proxy.GetAttr("GetState")
	assert.True(t, ok)
	stateFunc, ok := value.(*Builtin)
	assert.True(t, ok)
	assert.NotNil(t, stateFunc)
	assert.Equal(t, stateFunc.Name(), "*object.testService.GetState")

	// Call GetState and confirm a Proxy is returned that wraps the *testState
	result := stateFunc.Call(context.Background())
	resultProxy, ok := result.(*Proxy)
	assert.True(t, ok)
	value, ok = resultProxy.GetAttr("Count")
	assert.True(t, ok)
	assert.Equal(t, value, NewInt(42))
}

func TestTimeConverter(t *testing.T) {
	now := time.Now()
	typ := reflect.TypeOf(now)

	c, err := NewTypeConverter(typ)
	assert.Nil(t, err)

	tT, err := c.From(now)
	assert.Nil(t, err)
	assert.Equal(t, tT, NewTime(now))

	gT, err := c.To(NewTime(now))
	assert.Nil(t, err)
	goTime, ok := gT.(time.Time)
	assert.True(t, ok)
	assert.Equal(t, goTime, now)
}

func TestBufferConverter(t *testing.T) {
	buf := bytes.NewBufferString("hello")
	typ := reflect.TypeOf(buf)

	c, err := NewTypeConverter(typ)
	assert.Nil(t, err)

	tBuf, err := c.From(buf)
	assert.Nil(t, err)
	assert.Equal(t, tBuf, NewBuffer(buf))

	gBuf, err := c.To(NewBufferFromBytes([]byte("hello")))
	assert.Nil(t, err)
	goBuf, ok := gBuf.(*bytes.Buffer)
	assert.True(t, ok)
	assert.Equal(t, goBuf, buf)
}

func TestByteSliceConverter(t *testing.T) {
	buf := []byte("abc")
	typ := reflect.TypeOf(buf)

	c, err := NewTypeConverter(typ)
	assert.Nil(t, err)

	tBuf, err := c.From(buf)
	assert.Nil(t, err)
	assert.Equal(t, tBuf, NewByteSlice([]byte("abc")))

	gBuf, err := c.To(NewByteSlice([]byte("abc")))
	assert.Nil(t, err)
	goBuf, ok := gBuf.([]byte)
	assert.True(t, ok)
	assert.Equal(t, goBuf, buf)
}

func TestArrayConverterInt(t *testing.T) {
	arr := [4]int{2, 3, 4, 5}
	c, err := NewTypeConverter(reflect.TypeOf(arr))
	assert.Nil(t, err)

	tList, err := c.From(arr)
	assert.Nil(t, err)
	assert.Equal(t,

		tList, NewList([]Object{
			NewInt(2),
			NewInt(3),
			NewInt(4),
			NewInt(5),
		}))

	goValue, err := c.To(NewList([]Object{
		NewInt(-1),
		NewInt(-2),
	}))
	assert.Nil(t, err)

	goArray, ok := goValue.([4]int)
	assert.True(t, ok)
	assert.Equal(t, goArray, [4]int{-1, -2})
}

func TestArrayConverterFloat64(t *testing.T) {
	arr := [2]float64{100, 101}
	c, err := NewTypeConverter(reflect.TypeOf(arr))
	assert.Nil(t, err)

	tList, err := c.From(arr)
	assert.Nil(t, err)
	assert.Equal(t,

		tList, NewList([]Object{
			NewFloat(100),
			NewFloat(101),
		}))

	goValue, err := c.To(NewList([]Object{
		NewFloat(-1),
		NewFloat(-2),
	}))
	assert.Nil(t, err)

	goArray, ok := goValue.([2]float64)
	assert.True(t, ok)
	assert.Equal(t, goArray, [2]float64{-1, -2})
}

func TestGenericMapConverter(t *testing.T) {
	m := map[string]interface{}{
		"foo": 1,
		"bar": "two",
		"baz": []interface{}{
			"three",
			4,
			false,
			map[string]interface{}{
				"five": 5,
			},
		},
	}
	typ := reflect.TypeOf(m)

	c, err := NewTypeConverter(typ)
	assert.Nil(t, err)

	tMap, err := c.From(m)
	assert.Nil(t, err)
	assert.Equal(t,

		tMap, NewMap(map[string]Object{
			"foo": NewInt(1),
			"bar": NewString("two"),
			"baz": NewList([]Object{
				NewString("three"),
				NewInt(4),
				False,
				NewMap(map[string]Object{
					"five": NewInt(5),
				}),
			}),
		}))
}

func TestGenericMapConverterFromJSON(t *testing.T) {
	m := `{
		"foo": 1,
		"bar": "two",
		"baz": [
			"three",
			4,
			false,
			{ "five": 5 }
		]
	}`
	var v interface{}
	err := json.Unmarshal([]byte(m), &v)
	assert.Nil(t, err)

	fmt.Println(v, reflect.TypeOf(v))
	typ := reflect.TypeOf(v)

	c, err := NewTypeConverter(typ)
	assert.Nil(t, err)

	tMap, err := c.From(v)
	assert.Nil(t, err)
	assert.Equal(t,

		tMap, NewMap(map[string]Object{
			"foo": NewFloat(1),
			"bar": NewString("two"),
			"baz": NewList([]Object{
				NewString("three"),
				NewFloat(4),
				False,
				NewMap(map[string]Object{
					"five": NewFloat(5),
				}),
			}),
		}))
}
