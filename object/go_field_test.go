package object

import (
	"reflect"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

type testStruct struct {
	Name    string `json:"name"`
	Age     int    `json:"age"`
	Enabled bool   `json:"enabled"`
}

func TestGoField(t *testing.T) {
	// Create a test instance and get its type
	s := &testStruct{}
	typ := reflect.TypeOf(s).Elem()

	// Test string field
	nameField, ok := typ.FieldByName("Name")
	assert.True(t, ok)
	field, err := newGoField(nameField)
	assert.Nil(t, err)

	assert.Equal(t, field.Name(), "Name")
	assert.Equal(t, field.GoType().Name(), "string")
	assert.Equal(t, string(field.Tag()), `json:"name"`)

	// Test int field
	ageField, ok := typ.FieldByName("Age")
	assert.True(t, ok)
	field, err = newGoField(ageField)
	assert.Nil(t, err)

	assert.Equal(t, field.Name(), "Age")
	assert.Equal(t, field.GoType().Name(), "int")
	assert.Equal(t, string(field.Tag()), `json:"age"`)

	// Test bool field
	enabledField, ok := typ.FieldByName("Enabled")
	assert.True(t, ok)
	field, err = newGoField(enabledField)
	assert.Nil(t, err)

	assert.Equal(t, field.Name(), "Enabled")
	assert.Equal(t, field.GoType().Name(), "bool")
	assert.Equal(t, string(field.Tag()), `json:"enabled"`)
}

type complexStruct struct {
	Data    map[string]interface{} `json:"data"`
	Numbers []int                  `json:"numbers"`
	Ptr     *string                `json:"ptr"`
}

func TestGoFieldComplexTypes(t *testing.T) {
	s := complexStruct{}
	typ := reflect.TypeOf(s)

	// Test map field
	dataField, ok := typ.FieldByName("Data")
	assert.True(t, ok)
	field, err := newGoField(dataField)
	assert.Nil(t, err)

	assert.Equal(t, field.Name(), "Data")
	assert.Equal(t, field.GoType().Name(), "map[string]interface {}")
	assert.Equal(t, string(field.Tag()), `json:"data"`)

	// Test slice field
	numbersField, ok := typ.FieldByName("Numbers")
	assert.True(t, ok)
	field, err = newGoField(numbersField)
	assert.Nil(t, err)

	assert.Equal(t, field.Name(), "Numbers")
	assert.Equal(t, field.GoType().Name(), "[]int")
	assert.Equal(t, string(field.Tag()), `json:"numbers"`)

	// Test pointer field
	ptrField, ok := typ.FieldByName("Ptr")
	assert.True(t, ok)
	field, err = newGoField(ptrField)
	assert.Nil(t, err)

	assert.Equal(t, field.Name(), "Ptr")
	assert.Equal(t, field.GoType().Name(), "*string")
	assert.Equal(t, string(field.Tag()), `json:"ptr"`)
}

type nestedStruct struct {
	Inner struct {
		Value string `json:"value"`
	} `json:"inner"`
}

func TestGoFieldNestedStruct(t *testing.T) {
	s := &nestedStruct{}
	typ := reflect.TypeOf(s).Elem()

	// Test nested struct field
	innerField, ok := typ.FieldByName("Inner")
	assert.True(t, ok)
	field, err := newGoField(innerField)
	assert.Nil(t, err)

	assert.Equal(t, field.Name(), "Inner")
	assert.Equal(t, field.GoType().Name(), "*struct { Value string \"json:\\\"value\\\"\" }")
	assert.Equal(t, string(field.Tag()), `json:"inner"`)
}

func TestGoFieldGetAttr(t *testing.T) {
	s := &testStruct{}
	typ := reflect.TypeOf(s).Elem()
	nameField, ok := typ.FieldByName("Name")
	assert.True(t, ok)
	field, err := newGoField(nameField)
	assert.Nil(t, err)

	// Test GetAttr for name
	nameAttr, ok := field.GetAttr("name")
	assert.True(t, ok)
	assert.Equal(t, nameAttr.(*String).value, "Name")

	// Test GetAttr for type
	typeAttr, ok := field.GetAttr("type")
	assert.True(t, ok)
	assert.Equal(t, typeAttr.(*GoType).Name(), "string")

	// Test GetAttr for tag
	tagAttr, ok := field.GetAttr("tag")
	assert.True(t, ok)
	assert.Equal(t, tagAttr.(*String).value, `json:"name"`)

	// Test GetAttr for non-existent attribute
	_, ok = field.GetAttr("nonexistent")
	assert.False(t, ok)
}
