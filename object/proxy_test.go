package object_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
)

// Used to confirm we can proxy method calls that use complex types.
type ProxyTestOpts struct {
	A int
	B string
	C bool `json:"c"`
}

// We use this struct embedded in ProxyService to prove that methods provided by
// embedded structs are also proxied.
type ProxyServiceEmbedded struct{}

func (e ProxyServiceEmbedded) Flub(opts ProxyTestOpts) string {
	return fmt.Sprintf("flubbed:%d.%s.%v", opts.A, opts.B, opts.C)
}

func (e ProxyServiceEmbedded) Increment(ctx context.Context, i int64) int64 {
	return i + 1
}

// This represents a "service" provided by Go code that we want to call from
// Risor code using a proxy.
type ProxyService struct {
	ProxyServiceEmbedded
}

func (pt *ProxyService) ToUpper(s string) string {
	return strings.ToUpper(s)
}

func (pt *ProxyService) ParseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

type proxyTestType1 []string

func (p proxyTestType1) Len() int {
	return len(p)
}

func TestProxyNonStruct(t *testing.T) {
	proxy, err := object.NewProxy(proxyTestType1{"a", "b", "c"})
	assert.Nil(t, err)
	fmt.Println(proxy)

	goType := proxy.GoType()
	fmt.Println("goType:", goType)

	assert.Equal(t, goType.AttributeNames(), []string{"Len"})
	attr, ok := goType.GetAttribute("Len")
	assert.True(t, ok)
	assert.Equal(t, attr.Name(), "Len")

	method, ok := attr.(*object.GoMethod)
	assert.True(t, ok)
	assert.Equal(t, method.Name(), "Len")
	assert.Equal(t, method.NumIn(), 1)
	assert.Equal(t, method.NumOut(), 1)

	m, ok := proxy.GetAttr("Len")
	assert.True(t, ok)
	lenBuiltin, ok := m.(*object.Builtin)
	assert.True(t, ok)
	res := lenBuiltin.Call(context.Background())
	assert.Equal(t, res.(*object.Int).Value(), int64(3))
}

type proxyTestType2 struct {
	A    int
	B    map[string]int
	c    string
	Anon struct {
		X int
	}
	Nested proxyTestType1
}

func (p proxyTestType2) D(x int, y float32) (int, error) {
	return x + int(y), nil
}

func TestProxyTestType2(t *testing.T) {
	proxy, err := object.NewProxy(&proxyTestType2{
		A: 99,
		B: map[string]int{
			"foo": 123,
			"bar": 456,
		},
		c:    "hello",
		Anon: struct{ X int }{99},
		Nested: proxyTestType1{
			"baz",
			"qux",
		},
	})
	assert.Nil(t, err)
	fmt.Println(proxy)

	goType := proxy.GoType()
	assert.Equal(t, goType.Name(), "*object_test.proxyTestType2")
	fmt.Println("goType:", goType)

	assert.Equal(t, goType.AttributeNames(), []string{"A", "Anon", "B", "D", "Nested"})

	aAttr, ok := goType.GetAttribute("A")
	assert.True(t, ok)
	assert.Equal(t, aAttr.Name(), "A")
	field, ok := aAttr.(*object.GoField)
	assert.True(t, ok)
	assert.Equal(t, field.Name(), "A")
	assert.Equal(t, field.ReflectType().Name(), "int")

	anonAttr, ok := goType.GetAttribute("Anon")
	assert.True(t, ok)
	assert.Equal(t, anonAttr.Name(), "Anon")
	field, ok = anonAttr.(*object.GoField)
	assert.True(t, ok)
	assert.Equal(t, field.Name(), "Anon")
	assert.Equal(t, field.ReflectType().Name(), "")
	assert.Equal(t, field.GoType().AttributeNames(), []string{"X"})

	attr, ok := goType.GetAttribute("D")
	assert.True(t, ok)
	assert.Equal(t, attr.Name(), "D")

	method, ok := attr.(*object.GoMethod)
	assert.True(t, ok)
	assert.Equal(t, method.Name(), "D")
	assert.Equal(t, method.NumIn(), 3)
	assert.Equal(t, method.NumOut(), 2)

	in0 := method.InType(0)
	assert.Equal(t, in0.Name(), "*object_test.proxyTestType2")
	in1 := method.InType(1)
	assert.Equal(t, in1.Name(), "int")
	in2 := method.InType(2)
	assert.Equal(t, in2.Name(), "float32")

	out0 := method.OutType(0)
	assert.Equal(t, out0.Name(), "int")
	out1 := method.OutType(1)
	assert.Equal(t, out1.Name(), "error")

	assert.True(t, method.ProducesError())
	assert.Equal(t, method.ErrorIndices(), []int{1})

	nestedAttr, ok := goType.GetAttribute("Nested")
	assert.True(t, ok)
	assert.Equal(t, nestedAttr.Name(), "Nested")
	field, ok = nestedAttr.(*object.GoField)
	assert.True(t, ok)
	assert.Equal(t, field.Name(), "Nested")
	assert.Equal(t, field.ReflectType().Name(), "proxyTestType1")
	assert.Equal(t, field.GoType().AttributeNames(), []string{"Len"})

	ptt1, err := object.NewGoType(reflect.TypeOf(proxyTestType1{}))
	assert.Nil(t, err)
	assert.True(t, object.Equals(field.GoType(), ptt1))

	aValue, getOk := proxy.GetAttr("A")
	assert.True(t, getOk)
	assert.Equal(t, aValue, object.NewInt(99))
}

func TestProxyCall(t *testing.T) {
	proxy, err := object.NewProxy(&proxyTestType2{})
	assert.Nil(t, err)

	m, ok := proxy.GetAttr("D")
	assert.True(t, ok)

	b, ok := m.(*object.Builtin)
	assert.True(t, ok)

	result := b.Call(context.Background(),
		object.NewInt(1),
		object.NewFloat(2.0))

	assert.Equal(t, result, object.NewInt(3))
}

func TestProxySetGetAttr(t *testing.T) {
	proxy, err := object.NewProxy(&proxyTestType2{})
	assert.Nil(t, err)

	// A starts at 0
	value, ok := proxy.GetAttr("A")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewInt(0))

	// Set to 42
	assert.Nil(t, proxy.SetAttr("A", object.NewInt(42)))

	// Confirm 42
	value, ok = proxy.GetAttr("A")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewInt(42))

	// Set to -3
	assert.Nil(t, proxy.SetAttr("A", object.NewInt(-3)))

	// Confirm -3
	value, ok = proxy.GetAttr("A")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewInt(-3))
}

type proxyTestType3 struct {
	A int
	P *string
	I io.Reader
	M map[string]int
	S []string
}

func TestProxySetGetAttrNil(t *testing.T) {
	proxy, err := object.NewProxy(&proxyTestType3{})
	assert.Nil(t, err)

	// A is not nillable
	err = proxy.SetAttr("A", object.Nil)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "type error: expected int (nil given)")

	// P starts at nil
	value, ok := proxy.GetAttr("P")
	assert.True(t, ok)
	assert.Equal(t, value, object.Nil)

	// Set to "abc"
	assert.Nil(t, proxy.SetAttr("P", object.NewString("abc")))

	// Confirm "abc"
	value, ok = proxy.GetAttr("P")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewString("abc"))

	// Set to nil
	assert.Nil(t, proxy.SetAttr("P", object.Nil))

	// Confirm nil
	value, ok = proxy.GetAttr("P")
	assert.True(t, ok)
	assert.Equal(t, value, object.Nil)

	// I starts at nil
	value, ok = proxy.GetAttr("I")
	assert.True(t, ok)
	assert.Equal(t, value, object.Nil)

	// Set to "abc"
	assert.Nil(t, proxy.SetAttr("I", object.NewBuffer(bytes.NewBufferString("abc"))))

	// Confirm "abc"
	value, ok = proxy.GetAttr("I")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewBuffer(bytes.NewBufferString("abc")))

	// Set to nil
	assert.Nil(t, proxy.SetAttr("I", object.Nil))

	// Confirm nil
	value, ok = proxy.GetAttr("I")
	assert.True(t, ok)
	assert.Equal(t, value, object.Nil)

	// M starts at nil
	value, ok = proxy.GetAttr("M")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewMap(map[string]object.Object{}))

	// Set to {"a": 1, "b": 2, "c": 3}
	assert.Nil(t, proxy.SetAttr("M", object.NewMap(map[string]object.Object{
		"a": object.NewInt(1),
		"b": object.NewInt(2),
		"c": object.NewInt(3),
	})))

	// Confirm {"a": 1, "b": 2, "c": 3}
	value, ok = proxy.GetAttr("M")
	assert.True(t, ok)
	assert.Equal(t,

		value, object.NewMap(map[string]object.Object{
			"a": object.NewInt(1),
			"b": object.NewInt(2),
			"c": object.NewInt(3),
		}))

	// Set to nil
	assert.Nil(t, proxy.SetAttr("M", object.Nil))

	// Confirm nil
	value, ok = proxy.GetAttr("M")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewMap(map[string]object.Object{}))

	// S starts at nil
	value, ok = proxy.GetAttr("S")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewList([]object.Object{}))

	// Set to ["a", "b", "c"]
	assert.Nil(t, proxy.SetAttr("S", object.NewStringList([]string{"a", "b", "c"})))

	// Confirm ["a", "b", "c"]
	value, ok = proxy.GetAttr("S")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewStringList([]string{"a", "b", "c"}))

	// Set to nil
	assert.Nil(t, proxy.SetAttr("S", object.Nil))

	// Confirm nil
	value, ok = proxy.GetAttr("S")
	assert.True(t, ok)
	assert.Equal(t, value, object.NewList([]object.Object{}))
}

func TestProxyOnStructValue(t *testing.T) {
	p, err := object.NewProxy(proxyTestType2{A: 99})
	assert.NoError(t, err)
	assert.Equal(t, p.GoType().Name(), "*object_test.proxyTestType2")
	attr, ok := p.GetAttr("A")
	assert.True(t, ok)
	assert.Equal(t, attr, object.NewInt(99))
}

func TestProxyBytesBuffer(t *testing.T) {
	ctx := context.Background()
	buf := bytes.NewBuffer([]byte("abc"))
	var reader io.Reader = buf

	// Creating a proxy on an interface really means creating a proxy on the
	// underlying concrete type.
	proxy, err := object.NewProxy(reader)
	assert.Nil(t, err)

	// Confirm the GoType is actually *bytes.Buffer
	goType := proxy.GoType()
	assert.Equal(t, goType.Name(), "*bytes.Buffer")

	// The proxy should have attributes available for all public attributes
	// on *bytes.Buffer
	method, ok := proxy.GetAttr("Len")
	assert.True(t, ok)

	// Confirm we can call a method
	lenMethod, ok := method.(*object.Builtin)
	assert.True(t, ok)
	assert.Equal(t, lenMethod.Call(ctx), object.NewInt(3))

	// Write to the buffer and confirm the length changes
	buf.WriteString("defg")
	assert.Equal(t, lenMethod.Call(ctx), object.NewInt(7))

	// Confirm we can call Bytes() and get a byte_slice back
	getBytes, ok := proxy.GetAttr("Bytes")
	assert.True(t, ok)
	bytes := getBytes.(*object.Builtin).Call(ctx)
	assert.Equal(t, bytes, object.NewByteSlice([]byte("abcdefg")))
}

func TestProxyMethodError(t *testing.T) {
	// Using the ReadByte method as an example, call it in a situation that will
	// have it return an error, then confirm a Risor *Error is returned.

	// func (b *Buffer) ReadByte() (byte, error)
	// If no byte is available, it returns error io.EOF.

	ctx := context.Background()
	buf := bytes.NewBuffer(nil) // empty buffer!
	proxy, err := object.NewProxy(buf)
	assert.Nil(t, err)

	method, ok := proxy.GetAttr("ReadByte")
	assert.True(t, ok)

	readByte, ok := method.(*object.Builtin)
	assert.True(t, ok)

	result := readByte.Call(ctx)
	errObj, ok := result.(*object.Error)
	assert.True(t, ok)
	assert.Equal(t, errObj.Value().Error(), "EOF")
}

func TestProxyHasher(t *testing.T) {
	ctx := context.Background()
	h := sha256.New()

	proxy, err := object.NewProxy(h)
	assert.Nil(t, err)

	method, ok := proxy.GetAttr("Write")
	assert.True(t, ok)
	write, ok := method.(*object.Builtin)
	assert.True(t, ok)

	method, ok = proxy.GetAttr("Sum")
	assert.True(t, ok)
	sum, ok := method.(*object.Builtin)
	assert.True(t, ok)

	result := write.Call(ctx, object.NewByteSlice([]byte("abc")))
	assert.Equal(t, result, object.NewInt(3))

	result = write.Call(ctx, object.NewByteSlice([]byte("de")))
	assert.Equal(t, result, object.NewInt(2))

	result = sum.Call(ctx, object.NewByteSlice(nil))
	byte_slice, ok := result.(*object.ByteSlice)
	assert.True(t, ok)

	other := sha256.New()
	other.Write([]byte("abcde"))
	expected := other.Sum(nil)

	assert.Equal(t, byte_slice.Value(), expected)
}

type nestedStructA struct {
	B string
}

type nestedStructConfig struct {
	A nestedStructA
}

func TestProxyNestedStruct(t *testing.T) {
	config := &nestedStructConfig{}
	proxy, err := object.NewProxy(config)
	assert.Nil(t, err)

	// Get the A field
	aField, ok := proxy.GetAttr("A")
	assert.True(t, ok)

	// Verify A is a proxy to nestedStructA
	aProxy, ok := aField.(*object.Proxy)
	assert.True(t, ok)
	assert.Equal(t, aProxy.GoType().Name(), "*object_test.nestedStructA")

	// Set B field directly on the A proxy
	err = aProxy.SetAttr("B", object.NewString("hello"))
	assert.Nil(t, err)

	// Verify the value was set correctly
	assert.Equal(t, config.A.B, "hello")
}

type testNilArg struct{}

func (t *testNilArg) Test(arg any) {
	// Method implementation doesn't matter for this test
}

func (t *testNilArg) TestMultiple(a, b any) {
	// Method implementation doesn't matter for this test
}

func (t *testNilArg) TestMixed(a string, b any) {
	// Method implementation doesn't matter for this test
}

func (t *testNilArg) TestReturnNil() any {
	return nil
}

func (t *testNilArg) TestReturnValue() string {
	return "hello"
}

func (t *testNilArg) TestReturnMultiple() (string, any) {
	return "hello", nil
}

func (t *testNilArg) TestReturnPointer() *string {
	s := "hello"
	return &s
}

func TestProxyNilArg(t *testing.T) {
	proxy, err := object.NewProxy(&testNilArg{})
	assert.Nil(t, err)

	method, ok := proxy.GetAttr("Test")
	assert.True(t, ok)

	b, ok := method.(*object.Builtin)
	assert.True(t, ok)

	// Call Test with nil argument
	result := b.Call(context.Background(), object.Nil)
	assert.Equal(t, result, object.Nil)
}

func TestProxyMultipleNilArgs(t *testing.T) {
	proxy, err := object.NewProxy(&testNilArg{})
	assert.Nil(t, err)

	method, ok := proxy.GetAttr("TestMultiple")
	assert.True(t, ok)

	b, ok := method.(*object.Builtin)
	assert.True(t, ok)

	// Call TestMultiple with two nil arguments
	result := b.Call(context.Background(), object.Nil, object.Nil)
	assert.Equal(t, result, object.Nil)
}

func TestProxyMixedArgs(t *testing.T) {
	proxy, err := object.NewProxy(&testNilArg{})
	assert.Nil(t, err)

	method, ok := proxy.GetAttr("TestMixed")
	assert.True(t, ok)

	b, ok := method.(*object.Builtin)
	assert.True(t, ok)

	// Call TestMixed with a string and nil
	result := b.Call(context.Background(), object.NewString("hello"), object.Nil)
	assert.Equal(t, result, object.Nil)
}

func TestProxyReturnNil(t *testing.T) {
	proxy, err := object.NewProxy(&testNilArg{})
	assert.Nil(t, err)

	method, ok := proxy.GetAttr("TestReturnNil")
	assert.True(t, ok)

	b, ok := method.(*object.Builtin)
	assert.True(t, ok)

	// Call TestReturnNil and verify it returns nil
	result := b.Call(context.Background())
	assert.Equal(t, result, object.Nil)
}

func TestProxyReturnValue(t *testing.T) {
	proxy, err := object.NewProxy(&testNilArg{})
	assert.Nil(t, err)

	method, ok := proxy.GetAttr("TestReturnValue")
	assert.True(t, ok)

	b, ok := method.(*object.Builtin)
	assert.True(t, ok)

	// Call TestReturnValue and verify it returns the string
	result := b.Call(context.Background())
	str, ok := result.(*object.String)
	assert.True(t, ok)
	assert.Equal(t, str.Value(), "hello")
}

func TestProxyReturnMultiple(t *testing.T) {
	proxy, err := object.NewProxy(&testNilArg{})
	assert.Nil(t, err)

	method, ok := proxy.GetAttr("TestReturnMultiple")
	assert.True(t, ok)

	b, ok := method.(*object.Builtin)
	assert.True(t, ok)

	// Call TestReturnMultiple and verify it returns both values
	result := b.Call(context.Background())
	list, ok := result.(*object.List)
	assert.True(t, ok)
	assert.Equal(t, list.Len().Value(), int64(2))

	// Check first value (string)
	str, err := list.GetItem(object.NewInt(0))
	assert.Nil(t, err)
	assert.Equal(t, str.(*object.String).Value(), "hello")

	// Check second value (nil)
	str, err = list.GetItem(object.NewInt(1))
	assert.Nil(t, err)
	assert.Equal(t, str, object.Nil)
}

func TestProxyReturnPointer(t *testing.T) {
	proxy, err := object.NewProxy(&testNilArg{})
	assert.Nil(t, err)

	method, ok := proxy.GetAttr("TestReturnPointer")
	assert.True(t, ok)

	b, ok := method.(*object.Builtin)
	assert.True(t, ok)

	// Call TestReturnPointer and verify it returns the pointer
	result := b.Call(context.Background())
	str, ok := result.(*object.String)
	assert.True(t, ok)
	assert.Equal(t, str.Value(), "hello")
}

// Vector3D is a simple 3D vector type for testing struct field setting
type Vector3D struct {
	X, Y, Z float64
}

func (v Vector3D) Add(other Vector3D) Vector3D {
	return Vector3D{
		X: v.X + other.X,
		Y: v.Y + other.Y,
		Z: v.Z + other.Z,
	}
}

func (v Vector3D) String() string {
	return fmt.Sprintf("Vector3D{X:%v, Y:%v, Z:%v}", v.X, v.Y, v.Z)
}

type VectorFactory struct{}

func (f *VectorFactory) NewVector(x, y, z float64) Vector3D {
	return Vector3D{X: x, Y: y, Z: z}
}

// TestProxyMethodReturnStructValue tests that struct values returned from methods
// can have their fields modified
func TestProxyMethodReturnStructValue(t *testing.T) {
	// Create a proxy for the vector factory
	factory, err := object.NewProxy(&VectorFactory{})
	assert.Nil(t, err)

	// Get the NewVector method
	newVectorMethod, ok := factory.GetAttr("NewVector")
	assert.True(t, ok)
	newVector, ok := newVectorMethod.(*object.Builtin)
	assert.True(t, ok)

	// Create a vector using the factory method
	ctx := context.Background()
	vector1 := newVector.Call(ctx, object.NewFloat(1), object.NewFloat(2), object.NewFloat(3))

	// Verify it's a proxy
	vectorProxy1, ok := vector1.(*object.Proxy)
	assert.True(t, ok)

	// Verify the proxy is to a *Vector3D, not a Vector3D
	assert.Equal(t, vectorProxy1.GoType().Name(), "*object_test.Vector3D")

	// Get the Add method from the vector
	addMethod, ok := vectorProxy1.GetAttr("Add")
	assert.True(t, ok)
	add, ok := addMethod.(*object.Builtin)
	assert.True(t, ok)

	// Create another vector and add them
	vector2 := newVector.Call(ctx, object.NewFloat(4), object.NewFloat(5), object.NewFloat(6))
	result := add.Call(ctx, vector2)

	// Verify result is a proxy
	resultProxy, ok := result.(*object.Proxy)
	assert.True(t, ok)

	// Verify the result proxy is to a *Vector3D, not a Vector3D. Struct values
	// should be converted to pointers automatically
	assert.Equal(t, resultProxy.GoType().Name(), "*object_test.Vector3D")

	// Now test that we can modify fields on the result
	err = resultProxy.SetAttr("X", object.NewFloat(15))
	assert.Nil(t, err, "Should be able to set field X on the result")

	// Verify the field was updated
	x, ok := resultProxy.GetAttr("X")
	assert.True(t, ok)
	assert.Equal(t, x, object.NewFloat(15))

	// Test other fields too
	err = resultProxy.SetAttr("Y", object.NewFloat(25))
	assert.Nil(t, err)
	y, ok := resultProxy.GetAttr("Y")
	assert.True(t, ok)
	assert.Equal(t, y, object.NewFloat(25))

	err = resultProxy.SetAttr("Z", object.NewFloat(35))
	assert.Nil(t, err)
	z, ok := resultProxy.GetAttr("Z")
	assert.True(t, ok)
	assert.Equal(t, z, object.NewFloat(35))
}

// TestProxyStructConverterRoundTrip tests that struct values are properly converted
// when going from Go to Risor and back to Go
func TestProxyStructConverterRoundTrip(t *testing.T) {
	// Create a Vector3D struct
	original := Vector3D{X: 1, Y: 2, Z: 3}

	// Create a proxy for the struct
	proxy, err := object.NewProxy(original)
	assert.Nil(t, err)

	// Verify it's a pointer type in the proxy
	assert.Equal(t, proxy.GoType().Name(), "*object_test.Vector3D")

	// Modify a field
	err = proxy.SetAttr("X", object.NewFloat(10))
	assert.Nil(t, err)

	// Convert back to Go type
	// This should extract the value, not the pointer
	converter, err := object.NewTypeConverter(reflect.TypeOf(original))
	assert.Nil(t, err)

	result, err := converter.To(proxy)
	assert.Nil(t, err)

	// Verify the result is a Vector3D, not a *Vector3D
	resultVector, ok := result.(Vector3D)
	assert.True(t, ok)

	// Verify the field was modified
	assert.Equal(t, resultVector.X, 10.0)
	assert.Equal(t, resultVector.Y, 2.0)
	assert.Equal(t, resultVector.Z, 3.0)
}

// TestProxyVectorModificationTracking tests that when a struct value is returned
// from a method call, modifications to the struct value are reflected when later
// accessed from Go code
func TestProxyVectorModificationTracking(t *testing.T) {
	// Create a vector factory
	factory := &VectorFactory{}

	// Create a proxy for the factory
	factoryProxy, err := object.NewProxy(factory)
	assert.Nil(t, err)

	// Get the NewVector method
	newVectorMethod, ok := factoryProxy.GetAttr("NewVector")
	assert.True(t, ok)
	newVector, ok := newVectorMethod.(*object.Builtin)
	assert.True(t, ok)

	// Call the NewVector method to create a vector
	ctx := context.Background()
	vector := newVector.Call(ctx, object.NewFloat(1), object.NewFloat(2), object.NewFloat(3))

	// Get the Add method
	vectorProxy, ok := vector.(*object.Proxy)
	assert.True(t, ok)
	addMethod, ok := vectorProxy.GetAttr("Add")
	assert.True(t, ok)
	add, ok := addMethod.(*object.Builtin)
	assert.True(t, ok)

	// Call Add to create a result vector
	otherVector, err := object.NewProxy(Vector3D{X: 4, Y: 5, Z: 6})
	assert.Nil(t, err)
	resultVector := add.Call(ctx, otherVector)
	resultProxy, ok := resultVector.(*object.Proxy)
	assert.True(t, ok)

	// Extract the actual Go value from the proxy
	// We should be able to get the underlying Go object
	underlyingObj := resultProxy.Interface()

	// Modify the vector through the proxy
	err = resultProxy.SetAttr("X", object.NewFloat(99))
	assert.Nil(t, err)
	err = resultProxy.SetAttr("Y", object.NewFloat(88))
	assert.Nil(t, err)
	err = resultProxy.SetAttr("Z", object.NewFloat(77))
	assert.Nil(t, err)

	// Check that the modifications are reflected in the underlying Go value
	// This is important - the changes should be visible to Go code
	underlyingVector, ok := underlyingObj.(*Vector3D)
	assert.True(t, ok)
	assert.Equal(t, underlyingVector.X, 99.0)
	assert.Equal(t, underlyingVector.Y, 88.0)
	assert.Equal(t, underlyingVector.Z, 77.0)
}
