package object

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
)

// =============================================================================
// TEST STRUCTS
// =============================================================================

type testPerson struct {
	Name string
	Age  int
}

func (p *testPerson) Greet() string {
	return "Hello, " + p.Name
}

func (p *testPerson) SetName(name string) {
	p.Name = name
}

func (p *testPerson) Add(x, y int) int {
	return x + y
}

func (p *testPerson) GetAgePtr() *int {
	return &p.Age
}

// unexportedMethod should not be accessible
func (p *testPerson) unexportedMethod() {}

type testAddress struct {
	Street string
	City   string
	Zip    int
}

func (a *testAddress) FullAddress() string {
	return fmt.Sprintf("%s, %s %d", a.Street, a.City, a.Zip)
}

type testEmployee struct {
	Name    string
	Address testAddress
	Manager *testPerson
}

type testWithSlice struct {
	Items []string
	Nums  []int
}

type testWithMap struct {
	Data map[string]int
}

type testWithPointer struct {
	Value *int
	Text  *string
}

type testWithTime struct {
	Created time.Time
	Updated *time.Time
}

type testWithBytes struct {
	Data []byte
}

type testWithBool struct {
	Active   bool
	Verified bool
}

type testWithFloat struct {
	Score   float64
	Percent float32
}

type TestEmbeddedPerson struct {
	Name string
	Age  int
}

type testEmbedded struct {
	TestEmbeddedPerson
	Title string
}

type testWithInterface struct {
	Stringer fmt.Stringer
	Any      any
}

type testEmpty struct{}

type testWithUnexported struct {
	Public  string
	private string
}

type testValueReceiver struct {
	Value int
}

func (t testValueReceiver) GetValue() int {
	return t.Value
}

func (t testValueReceiver) Double() int {
	return t.Value * 2
}

type testPointerReceiver struct {
	Value int
}

func (t *testPointerReceiver) GetValue() int {
	return t.Value
}

func (t *testPointerReceiver) SetValue(v int) {
	t.Value = v
}

type testMethodReturnsError struct {
	Value int
}

func (t *testMethodReturnsError) MayFail(shouldFail bool) (int, error) {
	if shouldFail {
		return 0, errors.New("intentional failure")
	}
	return t.Value, nil
}

func (t *testMethodReturnsError) AlwaysFails() error {
	return errors.New("always fails")
}

type testMethodMultiReturn struct {
	X, Y int
}

func (t *testMethodMultiReturn) GetBoth() (int, int) {
	return t.X, t.Y
}

func (t *testMethodMultiReturn) Swap() (int, int) {
	return t.Y, t.X
}

type testMethodWithContext struct {
	Value string
}

func (t *testMethodWithContext) GetWithContext(ctx context.Context) string {
	if ctx.Err() != nil {
		return "cancelled"
	}
	return t.Value
}

type testDeeplyNested struct {
	Level1 struct {
		Level2 struct {
			Level3 struct {
				Value string
			}
		}
	}
}

// =============================================================================
// BASIC FIELD ACCESS TESTS
// =============================================================================

func TestGoStruct_FieldAccess(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	assert.Equal(t, goStruct.Type(), GOSTRUCT)
	assert.True(t, goStruct.IsTruthy())

	// Access Name field
	nameObj, ok := goStruct.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, nameObj.(*String).Value(), "Alice")

	// Access Age field
	ageObj, ok := goStruct.GetAttr("Age")
	assert.True(t, ok)
	assert.Equal(t, ageObj.(*Int).Value(), int64(30))

	// Non-existent field
	_, ok = goStruct.GetAttr("NonExistent")
	assert.False(t, ok)
}

func TestGoStruct_AllFieldTypes(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("string field", func(t *testing.T) {
		s := &testPerson{Name: "test"}
		gs := NewGoStruct(reflect.ValueOf(s), registry)
		v, ok := gs.GetAttr("Name")
		assert.True(t, ok)
		assert.Equal(t, v.(*String).Value(), "test")
	})

	t.Run("int field", func(t *testing.T) {
		s := &testPerson{Age: 42}
		gs := NewGoStruct(reflect.ValueOf(s), registry)
		v, ok := gs.GetAttr("Age")
		assert.True(t, ok)
		assert.Equal(t, v.(*Int).Value(), int64(42))
	})

	t.Run("bool field", func(t *testing.T) {
		s := &testWithBool{Active: true, Verified: false}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		v, ok := gs.GetAttr("Active")
		assert.True(t, ok)
		assert.Equal(t, v.(*Bool).Value(), true)

		v, ok = gs.GetAttr("Verified")
		assert.True(t, ok)
		assert.Equal(t, v.(*Bool).Value(), false)
	})

	t.Run("float fields", func(t *testing.T) {
		s := &testWithFloat{Score: 3.14, Percent: 0.5}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		v, ok := gs.GetAttr("Score")
		assert.True(t, ok)
		assert.Equal(t, v.(*Float).Value(), 3.14)

		v, ok = gs.GetAttr("Percent")
		assert.True(t, ok)
		// float32 converted to float64
		assert.Equal(t, v.(*Float).Value(), float64(float32(0.5)))
	})

	t.Run("slice field", func(t *testing.T) {
		s := &testWithSlice{Items: []string{"a", "b", "c"}, Nums: []int{1, 2, 3}}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		v, ok := gs.GetAttr("Items")
		assert.True(t, ok)
		list := v.(*List)
		assert.Equal(t, list.Len().Value(), int64(3))

		v, ok = gs.GetAttr("Nums")
		assert.True(t, ok)
		list = v.(*List)
		assert.Equal(t, list.Len().Value(), int64(3))
	})

	t.Run("map field", func(t *testing.T) {
		s := &testWithMap{Data: map[string]int{"a": 1, "b": 2}}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		v, ok := gs.GetAttr("Data")
		assert.True(t, ok)
		m := v.(*Map)
		assert.Equal(t, m.Size(), 2)
	})

	t.Run("time field", func(t *testing.T) {
		now := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
		s := &testWithTime{Created: now}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		v, ok := gs.GetAttr("Created")
		assert.True(t, ok)
		tm := v.(*Time)
		assert.Equal(t, tm.Value().Year(), 2024)
	})

	t.Run("bytes field", func(t *testing.T) {
		s := &testWithBytes{Data: []byte("hello")}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		v, ok := gs.GetAttr("Data")
		assert.True(t, ok)
		b := v.(*Bytes)
		assert.Equal(t, string(b.Value()), "hello")
	})
}

// =============================================================================
// FIELD SET TESTS
// =============================================================================

func TestGoStruct_FieldSet(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// Set Name field
	err := goStruct.SetAttr("Name", NewString("Bob"))
	assert.Nil(t, err)
	assert.Equal(t, person.Name, "Bob")

	// Set Age field
	err = goStruct.SetAttr("Age", NewInt(25))
	assert.Nil(t, err)
	assert.Equal(t, person.Age, 25)

	// Try to set non-existent field
	err = goStruct.SetAttr("NonExistent", NewString("value"))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no field")
}

func TestGoStruct_SetAllFieldTypes(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("set string", func(t *testing.T) {
		s := &testPerson{}
		gs := NewGoStruct(reflect.ValueOf(s), registry)
		err := gs.SetAttr("Name", NewString("test"))
		assert.Nil(t, err)
		assert.Equal(t, s.Name, "test")
	})

	t.Run("set int", func(t *testing.T) {
		s := &testPerson{}
		gs := NewGoStruct(reflect.ValueOf(s), registry)
		err := gs.SetAttr("Age", NewInt(42))
		assert.Nil(t, err)
		assert.Equal(t, s.Age, 42)
	})

	t.Run("set bool", func(t *testing.T) {
		s := &testWithBool{}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		err := gs.SetAttr("Active", True)
		assert.Nil(t, err)
		assert.Equal(t, s.Active, true)

		err = gs.SetAttr("Verified", False)
		assert.Nil(t, err)
		assert.Equal(t, s.Verified, false)
	})

	t.Run("set float", func(t *testing.T) {
		s := &testWithFloat{}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		err := gs.SetAttr("Score", NewFloat(3.14))
		assert.Nil(t, err)
		assert.Equal(t, s.Score, 3.14)
	})

	t.Run("set slice", func(t *testing.T) {
		s := &testWithSlice{}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		list := NewList([]Object{NewString("x"), NewString("y")})
		err := gs.SetAttr("Items", list)
		assert.Nil(t, err)
		assert.Equal(t, len(s.Items), 2)
		assert.Equal(t, s.Items[0], "x")
	})

	t.Run("set time", func(t *testing.T) {
		s := &testWithTime{}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		now := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		err := gs.SetAttr("Created", NewTime(now))
		assert.Nil(t, err)
		assert.Equal(t, s.Created.Year(), 2024)
	})
}

func TestGoStruct_SetWithTypeConversion(t *testing.T) {
	registry := DefaultRegistry()

	t.Run("int to float field", func(t *testing.T) {
		s := &testWithFloat{}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		err := gs.SetAttr("Score", NewInt(42))
		assert.Nil(t, err)
		assert.Equal(t, s.Score, 42.0)
	})

	t.Run("float to int field", func(t *testing.T) {
		s := &testPerson{}
		gs := NewGoStruct(reflect.ValueOf(s), registry)

		err := gs.SetAttr("Age", NewFloat(25.7))
		assert.Nil(t, err)
		assert.Equal(t, s.Age, 25) // Truncated
	})
}

func TestGoStruct_SetTypeError(t *testing.T) {
	registry := DefaultRegistry()
	s := &testPerson{}
	gs := NewGoStruct(reflect.ValueOf(s), registry)

	// String to int should fail
	err := gs.SetAttr("Age", NewString("not a number"))
	assert.NotNil(t, err)
}

// =============================================================================
// METHOD CALL TESTS
// =============================================================================

func TestGoStruct_MethodCall(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// Get method as attribute
	greetObj, ok := goStruct.GetAttr("Greet")
	assert.True(t, ok)

	goFunc, ok := greetObj.(*GoFunc)
	assert.True(t, ok)

	// Call the method
	result, err := goFunc.Call(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "Hello, Alice")
}

func TestGoStruct_MethodWithArgs(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// Get Add method
	addObj, ok := goStruct.GetAttr("Add")
	assert.True(t, ok)

	goFunc, ok := addObj.(*GoFunc)
	assert.True(t, ok)

	// Call with arguments
	result, err := goFunc.Call(context.Background(), NewInt(10), NewInt(20))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(30))
}

func TestGoStruct_MethodMutatesState(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// Get SetName method
	setNameObj, ok := goStruct.GetAttr("SetName")
	assert.True(t, ok)

	goFunc, ok := setNameObj.(*GoFunc)
	assert.True(t, ok)

	// Call the method
	_, err := goFunc.Call(context.Background(), NewString("Bob"))
	assert.Nil(t, err)

	// Verify the state was mutated
	assert.Equal(t, person.Name, "Bob")

	// Also verify via GetAttr
	nameObj, ok := goStruct.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, nameObj.(*String).Value(), "Bob")
}

func TestGoStruct_CallMethod(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// Use the convenience CallMethod
	result, err := goStruct.CallMethod(context.Background(), "Greet")
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "Hello, Alice")

	// Call with args
	result, err = goStruct.CallMethod(context.Background(), "Add", NewInt(5), NewInt(7))
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(12))

	// Non-existent method
	_, err = goStruct.CallMethod(context.Background(), "NonExistent")
	assert.NotNil(t, err)
}

func TestGoStruct_MethodReturnsError(t *testing.T) {
	s := &testMethodReturnsError{Value: 42}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	// Success case
	result, err := goStruct.CallMethod(context.Background(), "MayFail", False)
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(42))

	// Error case
	_, err = goStruct.CallMethod(context.Background(), "MayFail", True)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "intentional failure")

	// Always fails
	_, err = goStruct.CallMethod(context.Background(), "AlwaysFails")
	assert.NotNil(t, err)
}

func TestGoStruct_MethodMultipleReturns(t *testing.T) {
	s := &testMethodMultiReturn{X: 10, Y: 20}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	result, err := goStruct.CallMethod(context.Background(), "GetBoth")
	assert.Nil(t, err)

	list, ok := result.(*List)
	assert.True(t, ok)
	assert.Equal(t, list.Len().Value(), int64(2))

	items := list.Value()
	assert.Equal(t, items[0].(*Int).Value(), int64(10))
	assert.Equal(t, items[1].(*Int).Value(), int64(20))
}

func TestGoStruct_MethodWithContext(t *testing.T) {
	s := &testMethodWithContext{Value: "hello"}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	// Normal call
	result, err := goStruct.CallMethod(context.Background(), "GetWithContext")
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "hello")

	// Cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err = goStruct.CallMethod(ctx, "GetWithContext")
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "cancelled")
}

// =============================================================================
// VALUE VS POINTER RECEIVER TESTS
// =============================================================================

func TestGoStruct_ValueReceiver(t *testing.T) {
	s := &testValueReceiver{Value: 42}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	// Value receiver methods should be accessible
	result, err := goStruct.CallMethod(context.Background(), "GetValue")
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(42))

	result, err = goStruct.CallMethod(context.Background(), "Double")
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(84))
}

func TestGoStruct_PointerReceiver(t *testing.T) {
	s := &testPointerReceiver{Value: 42}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	// Pointer receiver methods should work
	result, err := goStruct.CallMethod(context.Background(), "GetValue")
	assert.Nil(t, err)
	assert.Equal(t, result.(*Int).Value(), int64(42))

	// Method that modifies state
	_, err = goStruct.CallMethod(context.Background(), "SetValue", NewInt(100))
	assert.Nil(t, err)
	assert.Equal(t, s.Value, 100)
}

// =============================================================================
// NESTED STRUCT TESTS
// =============================================================================

func TestGoStruct_NestedStruct(t *testing.T) {
	emp := &testEmployee{
		Name: "Alice",
		Address: testAddress{
			Street: "123 Main St",
			City:   "Springfield",
			Zip:    12345,
		},
	}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(emp), registry)

	// Access nested struct
	addrObj, ok := goStruct.GetAttr("Address")
	assert.True(t, ok)

	// The nested struct should be wrapped as GoStruct
	addrStruct, ok := addrObj.(*GoStruct)
	assert.True(t, ok)

	// Access nested field
	streetObj, ok := addrStruct.GetAttr("Street")
	assert.True(t, ok)
	assert.Equal(t, streetObj.(*String).Value(), "123 Main St")

	// Call method on nested struct
	result, err := addrStruct.CallMethod(context.Background(), "FullAddress")
	assert.Nil(t, err)
	assert.Equal(t, result.(*String).Value(), "123 Main St, Springfield 12345")
}

func TestGoStruct_NestedPointer(t *testing.T) {
	manager := &testPerson{Name: "Bob", Age: 45}
	emp := &testEmployee{
		Name:    "Alice",
		Manager: manager,
	}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(emp), registry)

	// Access nested pointer
	managerObj, ok := goStruct.GetAttr("Manager")
	assert.True(t, ok)

	managerStruct, ok := managerObj.(*GoStruct)
	assert.True(t, ok)

	nameObj, ok := managerStruct.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, nameObj.(*String).Value(), "Bob")
}

func TestGoStruct_NilNestedPointer(t *testing.T) {
	emp := &testEmployee{
		Name:    "Alice",
		Manager: nil,
	}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(emp), registry)

	// Access nil nested pointer
	managerObj, ok := goStruct.GetAttr("Manager")
	assert.True(t, ok)
	assert.Equal(t, managerObj, Nil)
}

func TestGoStruct_DeeplyNested(t *testing.T) {
	s := &testDeeplyNested{}
	s.Level1.Level2.Level3.Value = "deep"

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	// Navigate through levels
	level1, ok := goStruct.GetAttr("Level1")
	assert.True(t, ok)

	l1Struct := level1.(*GoStruct)
	level2, ok := l1Struct.GetAttr("Level2")
	assert.True(t, ok)

	l2Struct := level2.(*GoStruct)
	level3, ok := l2Struct.GetAttr("Level3")
	assert.True(t, ok)

	l3Struct := level3.(*GoStruct)
	value, ok := l3Struct.GetAttr("Value")
	assert.True(t, ok)
	assert.Equal(t, value.(*String).Value(), "deep")
}

// =============================================================================
// POINTER FIELD TESTS
// =============================================================================

func TestGoStruct_PointerField(t *testing.T) {
	val := 42
	text := "hello"
	s := &testWithPointer{Value: &val, Text: &text}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	// Access pointer field (should dereference)
	v, ok := goStruct.GetAttr("Value")
	assert.True(t, ok)
	assert.Equal(t, v.(*Int).Value(), int64(42))

	t2, ok := goStruct.GetAttr("Text")
	assert.True(t, ok)
	assert.Equal(t, t2.(*String).Value(), "hello")
}

func TestGoStruct_NilPointerField(t *testing.T) {
	s := &testWithPointer{Value: nil, Text: nil}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	v, ok := goStruct.GetAttr("Value")
	assert.True(t, ok)
	assert.Equal(t, v, Nil)

	t2, ok := goStruct.GetAttr("Text")
	assert.True(t, ok)
	assert.Equal(t, t2, Nil)
}

// =============================================================================
// EMBEDDED STRUCT TESTS
// =============================================================================

func TestGoStruct_EmbeddedStruct(t *testing.T) {
	s := &testEmbedded{
		TestEmbeddedPerson: TestEmbeddedPerson{Name: "Alice", Age: 30},
		Title:              "Engineer",
	}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	// Access embedded struct's fields through the embedded field name
	// Note: Go reflection shows embedded as a field with the type name
	embedded, ok := goStruct.GetAttr("TestEmbeddedPerson")
	assert.True(t, ok)

	embeddedStruct := embedded.(*GoStruct)
	name, ok := embeddedStruct.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, name.(*String).Value(), "Alice")

	// Access own field
	title, ok := goStruct.GetAttr("Title")
	assert.True(t, ok)
	assert.Equal(t, title.(*String).Value(), "Engineer")
}

// =============================================================================
// UNEXPORTED FIELD TESTS
// =============================================================================

func TestGoStruct_UnexportedField(t *testing.T) {
	s := &testWithUnexported{Public: "visible", private: "hidden"}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	// Public field accessible
	v, ok := goStruct.GetAttr("Public")
	assert.True(t, ok)
	assert.Equal(t, v.(*String).Value(), "visible")

	// Private field not accessible
	_, ok = goStruct.GetAttr("private")
	assert.False(t, ok)
}

func TestGoStruct_UnexportedMethod(t *testing.T) {
	s := &testPerson{Name: "Alice"}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	// Unexported method not accessible
	_, ok := goStruct.GetAttr("unexportedMethod")
	assert.False(t, ok)
}

// =============================================================================
// EMPTY STRUCT TESTS
// =============================================================================

func TestGoStruct_EmptyStruct(t *testing.T) {
	s := &testEmpty{}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	assert.Equal(t, goStruct.Type(), GOSTRUCT)

	// No attributes
	attrs := goStruct.Attrs()
	assert.Equal(t, len(attrs), 0)

	// Nothing to get
	_, ok := goStruct.GetAttr("anything")
	assert.False(t, ok)
}

// =============================================================================
// OBJECT INTERFACE TESTS
// =============================================================================

func TestGoStruct_Equals(t *testing.T) {
	person1 := &testPerson{Name: "Alice", Age: 30}
	person2 := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct1 := NewGoStruct(reflect.ValueOf(person1), registry)
	goStruct2 := NewGoStruct(reflect.ValueOf(person1), registry) // Same pointer
	goStruct3 := NewGoStruct(reflect.ValueOf(person2), registry) // Different pointer

	// Same pointer = equal
	assert.True(t, goStruct1.Equals(goStruct2))

	// Different pointer = not equal
	assert.False(t, goStruct1.Equals(goStruct3))

	// Different type = not equal
	assert.False(t, goStruct1.Equals(NewInt(1)))
}

func TestGoStruct_Inspect(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	inspect := goStruct.Inspect()
	assert.Contains(t, inspect, "go_struct")
	assert.Contains(t, inspect, "testPerson")

	// String() should be same as Inspect()
	assert.Equal(t, goStruct.String(), inspect)
}

func TestGoStruct_Attrs(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	attrs := goStruct.Attrs()

	// Should include fields and methods
	names := make(map[string]bool)
	for _, spec := range attrs {
		names[spec.Name] = true
	}

	assert.True(t, names["Name"])
	assert.True(t, names["Age"])
	assert.True(t, names["Greet"])
	assert.True(t, names["SetName"])
	assert.True(t, names["Add"])

	// Should not include unexported method
	assert.False(t, names["unexportedMethod"])
}

func TestGoStruct_RunOperation(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	_, err := goStruct.RunOperation(1, NewInt(1))
	assert.NotNil(t, err)
}

func TestGoStruct_Interface(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// Interface returns the original pointer
	result := goStruct.Interface()
	assert.Equal(t, result, person)
}

func TestGoStruct_Value(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// Value returns the reflect.Value
	val := goStruct.Value()
	assert.Equal(t, val.Interface(), person)
}

func TestGoStruct_StructType(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// StructType returns the type
	structType := goStruct.StructType()
	assert.Equal(t, structType.Name(), "testPerson")
}

// =============================================================================
// TYPE REGISTRY INTEGRATION TESTS
// =============================================================================

func TestGoStruct_FromTypeRegistry(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()

	// Convert via registry
	obj, err := registry.FromGo(person)
	assert.Nil(t, err)

	goStruct, ok := obj.(*GoStruct)
	assert.True(t, ok)

	nameObj, ok := goStruct.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, nameObj.(*String).Value(), "Alice")
}

func TestGoStruct_NonPointerStruct(t *testing.T) {
	// When passing a non-pointer struct, it should still work
	// (the registry creates a pointer internally)
	person := testPerson{Name: "Bob", Age: 25}

	registry := DefaultRegistry()

	obj, err := registry.FromGo(person)
	assert.Nil(t, err)

	goStruct, ok := obj.(*GoStruct)
	assert.True(t, ok)

	nameObj, ok := goStruct.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, nameObj.(*String).Value(), "Bob")
}

func TestGoStruct_SliceOfStructs(t *testing.T) {
	people := []*testPerson{
		{Name: "Alice", Age: 30},
		{Name: "Bob", Age: 25},
	}

	registry := DefaultRegistry()

	obj, err := registry.FromGo(people)
	assert.Nil(t, err)

	list, ok := obj.(*List)
	assert.True(t, ok)
	assert.Equal(t, list.Len().Value(), int64(2))

	// Access first element
	first := list.Value()[0]
	gs, ok := first.(*GoStruct)
	assert.True(t, ok)

	name, ok := gs.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, name.(*String).Value(), "Alice")
}

// =============================================================================
// METADATA CACHING TESTS
// =============================================================================

func TestGoStruct_MetadataCaching(t *testing.T) {
	// Create multiple GoStructs of the same type
	// They should share metadata

	registry := DefaultRegistry()

	p1 := &testPerson{Name: "Alice"}
	p2 := &testPerson{Name: "Bob"}
	p3 := &testPerson{Name: "Charlie"}

	gs1 := NewGoStruct(reflect.ValueOf(p1), registry)
	gs2 := NewGoStruct(reflect.ValueOf(p2), registry)
	gs3 := NewGoStruct(reflect.ValueOf(p3), registry)

	// All should work correctly
	n1, _ := gs1.GetAttr("Name")
	n2, _ := gs2.GetAttr("Name")
	n3, _ := gs3.GetAttr("Name")

	assert.Equal(t, n1.(*String).Value(), "Alice")
	assert.Equal(t, n2.(*String).Value(), "Bob")
	assert.Equal(t, n3.(*String).Value(), "Charlie")
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestGoStruct_ZeroValueFields(t *testing.T) {
	s := &testPerson{} // Zero values

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	name, ok := goStruct.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, name.(*String).Value(), "")

	age, ok := goStruct.GetAttr("Age")
	assert.True(t, ok)
	assert.Equal(t, age.(*Int).Value(), int64(0))
}

func TestGoStruct_SetToZeroValue(t *testing.T) {
	s := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	err := goStruct.SetAttr("Name", NewString(""))
	assert.Nil(t, err)
	assert.Equal(t, s.Name, "")

	err = goStruct.SetAttr("Age", NewInt(0))
	assert.Nil(t, err)
	assert.Equal(t, s.Age, 0)
}

func TestGoStruct_EmptySliceField(t *testing.T) {
	s := &testWithSlice{Items: []string{}}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	items, ok := goStruct.GetAttr("Items")
	assert.True(t, ok)

	list := items.(*List)
	assert.Equal(t, list.Len().Value(), int64(0))
}

func TestGoStruct_NilSliceField(t *testing.T) {
	s := &testWithSlice{Items: nil}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	items, ok := goStruct.GetAttr("Items")
	assert.True(t, ok)

	list := items.(*List)
	assert.Equal(t, list.Len().Value(), int64(0))
}

func TestGoStruct_NilMapField(t *testing.T) {
	s := &testWithMap{Data: nil}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	data, ok := goStruct.GetAttr("Data")
	assert.True(t, ok)

	m := data.(*Map)
	assert.Equal(t, m.Size(), 0)
}

func TestGoStruct_MethodReturnsPointer(t *testing.T) {
	s := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	result, err := goStruct.CallMethod(context.Background(), "GetAgePtr")
	assert.Nil(t, err)
	// Pointer return should be dereferenced
	assert.Equal(t, result.(*Int).Value(), int64(30))
}

func TestGoStruct_FieldShadowingMethod(t *testing.T) {
	// Field names take priority over method names if they conflict
	// (in Go, this would be a compile error, but let's test the priority)
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// "Name" is a field
	name, ok := goStruct.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, name.Type(), STRING)

	// "Greet" is a method
	greet, ok := goStruct.GetAttr("Greet")
	assert.True(t, ok)
	assert.Equal(t, greet.Type(), GOFUNC)
}

func TestGoStruct_CallNonMethodField(t *testing.T) {
	person := &testPerson{Name: "Alice", Age: 30}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(person), registry)

	// Trying to call a field that isn't a method should fail
	_, err := goStruct.CallMethod(context.Background(), "Name")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not callable")
}

func TestGoStruct_Unicode(t *testing.T) {
	s := &testPerson{Name: "アリス 🎉"}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	name, ok := goStruct.GetAttr("Name")
	assert.True(t, ok)
	assert.Equal(t, name.(*String).Value(), "アリス 🎉")
}

func TestGoStruct_LargeStruct(t *testing.T) {
	// Test with a struct that has many fields
	type largeStruct struct {
		F01, F02, F03, F04, F05 string
		F06, F07, F08, F09, F10 int
		F11, F12, F13, F14, F15 bool
		F16, F17, F18, F19, F20 float64
	}

	s := &largeStruct{
		F01: "a", F02: "b", F03: "c", F04: "d", F05: "e",
		F06: 1, F07: 2, F08: 3, F09: 4, F10: 5,
		F11: true, F12: false, F13: true, F14: false, F15: true,
		F16: 1.1, F17: 2.2, F18: 3.3, F19: 4.4, F20: 5.5,
	}

	registry := DefaultRegistry()
	goStruct := NewGoStruct(reflect.ValueOf(s), registry)

	attrs := goStruct.Attrs()
	assert.Equal(t, len(attrs), 20)

	v, ok := goStruct.GetAttr("F10")
	assert.True(t, ok)
	assert.Equal(t, v.(*Int).Value(), int64(5))

	v, ok = goStruct.GetAttr("F20")
	assert.True(t, ok)
	assert.Equal(t, v.(*Float).Value(), 5.5)
}
