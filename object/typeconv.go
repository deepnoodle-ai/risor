package object

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"
	"unicode/utf8"

	"github.com/risor-io/risor/bytecode"
)

var kindConverters = map[reflect.Kind]TypeConverter{
	reflect.Bool:    &BoolConverter{},
	reflect.Int:     &IntConverter{},
	reflect.Int8:    &Int8Converter{},
	reflect.Int16:   &Int16Converter{},
	reflect.Int32:   &Int32Converter{},
	reflect.Int64:   &Int64Converter{},
	reflect.Uint:    &UintConverter{},
	reflect.Uint8:   &Uint8Converter{},
	reflect.Uint16:  &Uint16Converter{},
	reflect.Uint32:  &Uint32Converter{},
	reflect.Uint64:  &Uint64Converter{},
	reflect.Float32: &Float32Converter{},
	reflect.Float64: &Float64Converter{},
	reflect.String:  &StringConverter{},
}

var typeConverters = map[reflect.Type]TypeConverter{
	reflect.TypeOf(byte(0)):              &ByteConverter{},
	reflect.TypeOf(time.Time{}):          &TimeConverter{},
	reflect.TypeOf(bytes.NewBuffer(nil)): &BufferConverter{},
	reflect.TypeOf([]byte{}):             &ByteSliceConverter{},
	reflect.TypeOf([]float64{}):          &FloatSliceConverter{},
}

// Kinds do NOT intend to handle for now:
// * Chan
// * Complex64
// * Complex128
// * UnsafePointer

// *****************************************************************************
// Type assertion helpers
// *****************************************************************************

func AsBool(obj Object) (bool, *Error) {
	b, ok := obj.(*Bool)
	if !ok {
		return false, TypeErrorf("type error: expected a bool (%s given)", obj.Type())
	}
	return b.value, nil
}

func AsString(obj Object) (string, *Error) {
	switch obj := obj.(type) {
	case *String:
		return obj.value, nil
	case *ByteSlice:
		return string(obj.value), nil
	case *Buffer:
		return obj.value.String(), nil
	default:
		return "", TypeErrorf("type error: expected a string (%s given)", obj.Type())
	}
}

func AsInt(obj Object) (int64, *Error) {
	switch obj := obj.(type) {
	case *Int:
		return obj.value, nil
	case *Byte:
		return int64(obj.value), nil
	default:
		return 0, TypeErrorf("type error: expected an integer (%s given)", obj.Type())
	}
}

func AsByte(obj Object) (byte, *Error) {
	switch obj := obj.(type) {
	case *Int:
		return byte(obj.value), nil
	case *Byte:
		return obj.value, nil
	case *Float:
		return byte(obj.value), nil
	case *String:
		if len(obj.value) != 1 {
			return 0, TypeErrorf("type error: expected a single byte string (length %d)", len(obj.value))
		}
		return obj.value[0], nil
	default:
		return 0, TypeErrorf("type error: expected a byte (%s given)", obj.Type())
	}
}

func AsFloat(obj Object) (float64, *Error) {
	switch obj := obj.(type) {
	case *Int:
		return float64(obj.value), nil
	case *Byte:
		return float64(obj.value), nil
	case *Float:
		return obj.value, nil
	default:
		return 0.0, TypeErrorf("type error: expected a number (%s given)", obj.Type())
	}
}

func AsList(obj Object) (*List, *Error) {
	list, ok := obj.(*List)
	if !ok {
		return nil, TypeErrorf("type error: expected a list (%s given)", obj.Type())
	}
	return list, nil
}

func AsStringSlice(obj Object) ([]string, *Error) {
	list, ok := obj.(*List)
	if !ok {
		return nil, TypeErrorf("type error: expected a list (%s given)", obj.Type())
	}
	result := make([]string, 0, len(list.items))
	for _, item := range list.items {
		s, err := AsString(item)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}

func AsMap(obj Object) (*Map, *Error) {
	m, ok := obj.(*Map)
	if !ok {
		return nil, TypeErrorf("type error: expected a map (%s given)", obj.Type())
	}
	return m, nil
}

func AsTime(obj Object) (result time.Time, err *Error) {
	s, ok := obj.(*Time)
	if !ok {
		return time.Time{}, TypeErrorf("type error: expected a time (%s given)", obj.Type())
	}
	return s.value, nil
}

func AsSet(obj Object) (*Set, *Error) {
	set, ok := obj.(*Set)
	if !ok {
		return nil, TypeErrorf("type error: expected a set (%s given)", obj.Type())
	}
	return set, nil
}

func AsBytes(obj Object) ([]byte, *Error) {
	switch obj := obj.(type) {
	case *ByteSlice:
		return obj.value, nil
	case *Buffer:
		return obj.value.Bytes(), nil
	case *String:
		return []byte(obj.value), nil
	case io.Reader:
		bytes, err := io.ReadAll(obj)
		if err != nil {
			return nil, NewError(err)
		}
		return bytes, nil
	default:
		return nil, TypeErrorf("type error: expected bytes (%s given)", obj.Type())
	}
}

func AsReader(obj Object) (io.Reader, *Error) {
	if o, ok := obj.(interface{ AsReader() (io.Reader, *Error) }); ok {
		return o.AsReader()
	}
	switch obj := obj.(type) {
	case *ByteSlice:
		return bytes.NewBuffer(obj.value), nil
	case *String:
		return bytes.NewBufferString(obj.value), nil
	case io.Reader:
		return obj, nil
	default:
		return nil, TypeErrorf("type error: expected a readable object (%s given)", obj.Type())
	}
}

func AsWriter(obj Object) (io.Writer, *Error) {
	if o, ok := obj.(interface{ AsWriter() (io.Writer, *Error) }); ok {
		return o.AsWriter()
	}
	switch obj := obj.(type) {
	case *Buffer:
		return obj.value, nil
	case io.Writer:
		return obj, nil
	default:
		return nil, TypeErrorf("type error: expected a writable object (%s given)", obj.Type())
	}
}

func AsError(obj Object) (*Error, *Error) {
	err, ok := obj.(*Error)
	if !ok {
		return nil, TypeErrorf("type error: expected an error object (%s given)", obj.Type())
	}
	return err, nil
}

// *****************************************************************************
// Converting types from Go to Risor
// *****************************************************************************

func FromGoType(obj interface{}) Object {
	switch obj := obj.(type) {
	case nil:
		return Nil
	case int:
		return NewInt(int64(obj))
	case int16:
		return NewInt(int64(obj))
	case int32:
		return NewInt(int64(obj))
	case int64:
		return NewInt(obj)
	case uint:
		return NewInt(int64(obj))
	case uint16:
		return NewInt(int64(obj))
	case uint32:
		return NewInt(int64(obj))
	case uint64:
		return NewInt(int64(obj))
	case float32:
		return NewFloat(float64(obj))
	case float64:
		return NewFloat(obj)
	case json.Number:
		if n, err := obj.Float64(); err == nil {
			return NewFloat(n)
		}
		return NewString(obj.String())
	case string:
		return NewString(obj)
	case []string:
		return NewStringList(obj)
	case byte:
		return NewByte(obj)
	case []byte:
		return NewByteSlice(obj)
	case *bytes.Buffer:
		return NewBuffer(obj)
	case *bytecode.Function:
		return NewClosure(obj)
	case bool:
		if obj {
			return True
		}
		return False
	// case [16]uint8:
	// 	return NewString(uuid.UUID(obj).String())
	case time.Time:
		return NewTime(obj)
	case []interface{}:
		items := make([]Object, 0, len(obj))
		for _, item := range obj {
			listItem := FromGoType(item)
			if IsError(listItem) {
				return listItem
			}
			items = append(items, listItem)
		}
		return NewList(items)
	case map[string]interface{}:
		m := make(map[string]Object, len(obj))
		for k, v := range obj {
			valueObj := FromGoType(v)
			if IsError(valueObj) {
				return valueObj
			}
			m[k] = valueObj
		}
		return NewMap(m)
	case Object:
		return obj
	default:
		return TypeErrorf("type error: unmarshaling %v (%v)",
			obj, reflect.TypeOf(obj))
	}
}

// AsObjects transform a map containing arbitrary Go types to a map of
// Risor objects, using the best type converter for each type. If an item
// in the map is of a type that can't be converted, an error is returned.
func AsObjects(m map[string]any) (map[string]Object, error) {
	result := make(map[string]Object, len(m))
	for k, v := range m {
		switch v := v.(type) {
		case Object:
			result[k] = v
		default:
			converter, err := NewTypeConverter(reflect.TypeOf(v))
			if err != nil {
				return nil, err
			}
			value, err := converter.From(v)
			if err != nil {
				return nil, err
			}
			result[k] = value
		}
	}
	return result, nil
}

// *****************************************************************************
// TypeConverter interface and implementations
//   - These are applicable when the Go type(s) are known in advance
// *****************************************************************************

// TypeConverter is an interface used to convert between Go and Risor objects
// for a single Go type.
type TypeConverter interface {
	// To converts to a Go object from a Risor object.
	To(Object) (interface{}, error)

	// From converts a Go object to a Risor object.
	From(interface{}) (Object, error)
}

// NewTypeConverter returns a TypeConverter for the given Go kind and type.
// Converters are cached internally for reuse.
func NewTypeConverter(typ reflect.Type) (TypeConverter, error) {
	goTypeMutex.Lock()
	defer goTypeMutex.Unlock()

	return createTypeConverter(typ)
}

// The caller must hold the goTypeMutex lock.
func createTypeConverter(typ reflect.Type) (TypeConverter, error) {
	if conv, ok := typeConverters[typ]; ok {
		return conv, nil
	}
	conv, err := getTypeConverter(typ)
	if err != nil {
		return nil, err
	}
	typeConverters[typ] = conv
	return conv, nil
}

// SetTypeConverter sets a TypeConverter for the given Go type. This is not
// typically used, since the default converters should typically be sufficient.
func SetTypeConverter(typ reflect.Type, conv TypeConverter) {
	goTypeMutex.Lock()
	defer goTypeMutex.Unlock()

	typeConverters[typ] = conv
}

// getTypeConverter returns a TypeConverter for the given Go type.
// The caller must hold the goTypeMutex lock.
func getTypeConverter(typ reflect.Type) (TypeConverter, error) {
	kind := typ.Kind()
	if conv, ok := kindConverters[kind]; ok {
		return conv, nil
	}
	if conv, ok := typeConverters[typ]; ok {
		return conv, nil
	}
	var err error
	var converter TypeConverter
	switch kind {
	case reflect.Struct:
		converter, err = newStructConverter(typ)
		if err != nil {
			return nil, err
		}
	case reflect.Pointer:
		indirectType := typ.Elem()
		if indirectKind := indirectType.Kind(); indirectKind == reflect.Struct {
			converter, err = newStructConverter(typ)
			if err != nil {
				return nil, err
			}
		} else {
			converter, err = newPointerConverter(typ.Elem())
			if err != nil {
				return nil, err
			}
		}
	case reflect.Slice:
		converter, err = newSliceConverter(typ.Elem())
		if err != nil {
			return nil, err
		}
	case reflect.Array:
		converter, err = newArrayConverter(typ.Elem(), typ.Len())
		if err != nil {
			return nil, err
		}
	case reflect.Map:
		if typ.Key().Kind() == reflect.String {
			converter, err = newMapConverter(typ.Elem())
			if err != nil {
				return nil, err
			}
		} else {
			return nil, TypeErrorf("type error: unsupported map key type in %s", typ)
		}
	case reflect.Interface:
		if typ.Implements(errorInterface) {
			converter = &ErrorConverter{}
		} else if typ.Implements(contextInterface) {
			converter = &ContextConverter{}
		} else { // TODO: io.*?
			converter = &DynamicConverter{}
		}
	default:
		return nil, TypeErrorf("type error: unsupported kind: %s", kind)
	}
	return converter, nil
}

// BoolConverter converts between bool and *Bool.
type BoolConverter struct{}

func (c *BoolConverter) To(obj Object) (interface{}, error) {
	b, ok := obj.(*Bool)
	if !ok {
		return nil, TypeErrorf("type error: expected bool (%s given)", obj.Type())
	}
	return b.value, nil
}

func (c *BoolConverter) From(obj interface{}) (Object, error) {
	return NewBool(obj.(bool)), nil
}

// ByteConverter converts between byte and *Byte.
type ByteConverter struct{}

func (c *ByteConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return obj.value, nil
	case *Int:
		return byte(obj.value), nil
	case *Float:
		return byte(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected byte (%s given)", obj.Type())
	}
}

func (c *ByteConverter) From(obj interface{}) (Object, error) {
	return NewByte(obj.(byte)), nil
}

// RuneConverter converts between rune and *String.
type RuneConverter struct{}

func (c *RuneConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *String:
		if len(obj.value) != 1 {
			return nil, TypeErrorf("type error: expected single rune string (got length %d)", len(obj.value))
		}
		r, _ := utf8.DecodeRuneInString(obj.value)
		return r, nil
	case *Int:
		return rune(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected string (%s given)", obj.Type())
	}
}

func (c *RuneConverter) From(obj interface{}) (Object, error) {
	return NewString(string([]rune{obj.(rune)})), nil
}

// IntConverter converts between int and *Int.
type IntConverter struct{}

func (c *IntConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return int(obj.value), nil
	case *Int:
		return int(obj.value), nil
	case *Float:
		return int(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *IntConverter) From(obj interface{}) (Object, error) {
	return NewInt(int64(obj.(int))), nil
}

// Int8Converter converts between int8 and *Int.
type Int8Converter struct{}

func (c *Int8Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return int8(obj.value), nil
	case *Int:
		return int8(obj.value), nil
	case *Float:
		return int8(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *Int8Converter) From(obj interface{}) (Object, error) {
	return NewInt(int64(obj.(int8))), nil
}

// Int16Converter converts between int16 and *Int.
type Int16Converter struct{}

func (c *Int16Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return int16(obj.value), nil
	case *Int:
		return int16(obj.value), nil
	case *Float:
		return int16(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *Int16Converter) From(obj interface{}) (Object, error) {
	return NewInt(int64(obj.(int16))), nil
}

// Int32Converter converts between int32 and *Int.
type Int32Converter struct{}

func (c *Int32Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return int32(obj.value), nil
	case *Int:
		return int32(obj.value), nil
	case *Float:
		return int32(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *Int32Converter) From(obj interface{}) (Object, error) {
	return NewInt(int64(obj.(int32))), nil
}

// Int64Converter converts between int64 and *Int.
type Int64Converter struct{}

func (c *Int64Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return int64(obj.value), nil
	case *Int:
		return int64(obj.value), nil
	case *Float:
		return int64(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *Int64Converter) From(obj interface{}) (Object, error) {
	return NewInt(obj.(int64)), nil
}

// UintConverter converts between uint and *Int.
type UintConverter struct{}

func (c *UintConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return uint(obj.value), nil
	case *Int:
		return uint(obj.value), nil
	case *Float:
		return uint(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *UintConverter) From(obj interface{}) (Object, error) {
	return NewInt(int64(obj.(uint))), nil
}

// Uint8Converter converts between uint8 and *Int.
type Uint8Converter struct{}

func (c *Uint8Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return uint8(obj.value), nil
	case *Int:
		return uint8(obj.value), nil
	case *Float:
		return uint8(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *Uint8Converter) From(obj interface{}) (Object, error) {
	return NewInt(int64(obj.(uint8))), nil
}

// Uint16Converter converts between uint16 and *Int.
type Uint16Converter struct{}

func (c *Uint16Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return uint16(obj.value), nil
	case *Int:
		return uint16(obj.value), nil
	case *Float:
		return uint16(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *Uint16Converter) From(obj interface{}) (Object, error) {
	return NewInt(int64(obj.(uint16))), nil
}

// Uint32Converter converts between uint32 and *Int.
type Uint32Converter struct{}

func (c *Uint32Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return uint32(obj.value), nil
	case *Int:
		return uint32(obj.value), nil
	case *Float:
		return uint32(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *Uint32Converter) From(obj interface{}) (Object, error) {
	return NewInt(int64(obj.(uint32))), nil
}

// Uint64Converter converts between uint64 and *Int.
type Uint64Converter struct{}

func (c *Uint64Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return uint64(obj.value), nil
	case *Int:
		return uint64(obj.value), nil
	case *Float:
		return uint64(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected int (%s given)", obj.Type())
	}
}

func (c *Uint64Converter) From(obj interface{}) (Object, error) {
	return NewInt(int64(obj.(uint64))), nil
}

// Float32Converter converts between float32 and *Float.
type Float32Converter struct{}

func (c *Float32Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return float32(obj.value), nil
	case *Int:
		return float32(obj.value), nil
	case *Float:
		return float32(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected float (%s given)", obj.Type())
	}
}

func (c *Float32Converter) From(obj interface{}) (Object, error) {
	return NewFloat(float64(obj.(float32))), nil
}

// Float64Converter converts between float64 and *Float.
type Float64Converter struct{}

func (c *Float64Converter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Byte:
		return float64(obj.value), nil
	case *Int:
		return float64(obj.value), nil
	case *Float:
		return obj.value, nil
	default:
		return nil, TypeErrorf("type error: expected float (%s given)", obj.Type())
	}
}

func (c *Float64Converter) From(obj interface{}) (Object, error) {
	return NewFloat(obj.(float64)), nil
}

// StringConverter converts between string and *String.
type StringConverter struct{}

func (c *StringConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *ByteSlice:
		return string(obj.value), nil
	case *Buffer:
		return obj.value.String(), nil
	case *String:
		return obj.value, nil
	default:
		return nil, TypeErrorf("type error: expected string (%s given)", obj.Type())
	}
}

func (c *StringConverter) From(obj interface{}) (Object, error) {
	return NewString(obj.(string)), nil
}

// ByteSliceConverter converts between []byte and *ByteSlice.
type ByteSliceConverter struct{}

func (c *ByteSliceConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *ByteSlice:
		return obj.value, nil
	case *Buffer:
		return obj.value.Bytes(), nil
	case *String:
		return []byte(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected bytes (%s given)", obj.Type())
	}
}

func (c *ByteSliceConverter) From(obj interface{}) (Object, error) {
	return NewByteSlice(obj.([]byte)), nil
}

// FloatSliceConverter converts between []float64 and *FloatSlice.
type FloatSliceConverter struct{}

func (c *FloatSliceConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *FloatSlice:
		return obj.value, nil
	default:
		return nil, TypeErrorf("type error: expected float_slice (%s given)", obj.Type())
	}
}

func (c *FloatSliceConverter) From(obj interface{}) (Object, error) {
	return NewFloatSlice(obj.([]float64)), nil
}

// TimeConverter converts between time.Time and *Time.
type TimeConverter struct{}

func (c *TimeConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Time:
		return obj.value, nil
	case *String:
		return time.Parse(time.RFC3339, obj.value)
	default:
		return nil, TypeErrorf("type error: expected time (%s given)", obj.Type())
	}
}

func (c *TimeConverter) From(obj interface{}) (Object, error) {
	return NewTime(obj.(time.Time)), nil
}

// BufferConverter converts between *bytes.Buffer and *Buffer.
type BufferConverter struct{}

func (c *BufferConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Buffer:
		return obj.value, nil
	case *ByteSlice:
		return bytes.NewBuffer(obj.value), nil
	default:
		return nil, TypeErrorf("type error: expected buffer (%s given)", obj.Type())
	}
}

func (c *BufferConverter) From(obj interface{}) (Object, error) {
	return NewBuffer(obj.(*bytes.Buffer)), nil
}

// DynamicConverter converts between interface{} and the appropriate Risor type.
// This is slow and should only be used to handle unknown types.
type DynamicConverter struct{}

func (c *DynamicConverter) To(obj Object) (interface{}, error) {
	return obj.Interface(), nil
}

func (c *DynamicConverter) From(obj interface{}) (Object, error) {
	if obj == nil {
		return Nil, nil
	}
	typ := reflect.TypeOf(obj)
	conv, err := NewTypeConverter(typ)
	if err != nil {
		return nil, err
	}
	return conv.From(obj)
}

// MapConverter converts between map[string]interface{} and *Map.
type MapConverter struct {
	valueConverter TypeConverter
	valueType      reflect.Type
}

func (c *MapConverter) To(obj Object) (interface{}, error) {
	if obj.Type() == NIL {
		return nil, nil
	}
	tMap, ok := obj.(*Map)
	if !ok {
		return nil, TypeErrorf("type error: expected map (%s given)", obj.Type())
	}
	keyType := reflect.TypeOf("")
	mapType := reflect.MapOf(keyType, c.valueType)
	gMap := reflect.MakeMapWithSize(mapType, tMap.Size())
	for k, v := range tMap.items {
		conv, err := c.valueConverter.To(v)
		if err != nil {
			return nil, err
		}
		gMap.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(conv))
	}
	return gMap.Interface(), nil
}

func (c *MapConverter) From(obj interface{}) (Object, error) {
	m := reflect.ValueOf(obj)
	o := make(map[string]Object, m.Len())
	for _, key := range m.MapKeys() {
		v := m.MapIndex(key)
		conv, err := c.valueConverter.From(v.Interface())
		if err != nil {
			return nil, err
		}
		o[key.Interface().(string)] = conv
	}
	return NewMap(o), nil
}

func newMapConverter(valueType reflect.Type) (*MapConverter, error) {
	valueConverter, err := createTypeConverter(valueType)
	if err != nil {
		return nil, TypeErrorf("type error: unsupported map value type %s", valueType)
	}
	return &MapConverter{
		valueConverter: valueConverter,
		valueType:      valueType,
	}, nil
}

// StructConverter converts between a Go struct and a Risor Proxy.
// Works with structs as values or pointers.
type StructConverter struct {
	typ         reflect.Type
	goType      *GoType
	isValueType bool
}

func (c *StructConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Proxy:
		// Return the object wrapped by the proxy
		if c.isValueType {
			return reflect.ValueOf(obj.obj).Elem().Interface(), nil
		}
		return obj.obj, nil
	case *Map:
		// Create a new struct. The "value" here is a pointer to the new struct.
		value := c.goType.New()
		// Get the underlying struct so that we can set its fields.
		structValue := value.Elem()
		for k, value := range obj.items {
			// If the struct has a field with the same name as a key, set it.
			if f := structValue.FieldByName(k); f.CanSet() {
				if attr, ok := c.goType.GetAttribute(k); ok {
					if attrField, ok := attr.(*GoField); ok {
						attrValue, err := attrField.converter.To(value)
						if err != nil {
							return nil, err
						}
						f.Set(reflect.ValueOf(attrValue))
					}
				}
			}
		}
		if c.goType.IsPointerType() {
			return value.Interface(), nil
		}
		return structValue.Interface(), nil
	default:
		return nil, TypeErrorf("type error: expected a proxy or map (%s given)", obj.Type())
	}
}

func (c *StructConverter) From(obj interface{}) (Object, error) {
	// Sanity check that the object is of the expected type
	typ := reflect.TypeOf(obj)
	if typ != c.typ {
		return nil, TypeErrorf("type error: expected %s (%s given)", c.typ, typ)
	}
	// Wrap the object in a proxy
	return NewProxy(obj)
}

// newStructConverter creates a TypeConverter for a given type of struct.
func newStructConverter(typ reflect.Type) (*StructConverter, error) {
	goType, err := newGoType(typ)
	if err != nil {
		return nil, err
	}
	return &StructConverter{
		typ:         typ,
		goType:      goType,
		isValueType: !goType.IsPointerType(),
	}, nil
}

// PointerConverter converts between *T and the Risor equivalent of T.
type PointerConverter struct {
	valueConverter TypeConverter
}

func (c *PointerConverter) To(obj Object) (interface{}, error) {
	if obj.Type() == NIL {
		return nil, nil
	}
	v, err := c.valueConverter.To(obj)
	if err != nil {
		return nil, err
	}
	vp := reflect.New(reflect.TypeOf(v))
	vp.Elem().Set(reflect.ValueOf(v))
	return vp.Interface(), nil
}

func (c *PointerConverter) From(obj interface{}) (Object, error) {
	v := reflect.ValueOf(obj)
	if v.IsZero() {
		return Nil, nil
	}
	return c.valueConverter.From(v.Elem().Interface())
}

// newPointerConverter creates a TypeConverter for pointers that point to
// items of the given type.
func newPointerConverter(indirectType reflect.Type) (*PointerConverter, error) {
	indirectConv, err := createTypeConverter(indirectType)
	if err != nil {
		return nil, err
	}
	return &PointerConverter{valueConverter: indirectConv}, nil
}

// SliceConverter converts between []T and the Risor equivalent of []T.
type SliceConverter struct {
	valueConverter TypeConverter
	valueType      reflect.Type
}

func (c *SliceConverter) To(obj Object) (interface{}, error) {
	if obj.Type() == NIL {
		return nil, nil
	}
	list, ok := obj.(*List)
	if !ok {
		return nil, TypeErrorf("type error: expected a list (%s given)", obj.Type())
	}
	slice := reflect.MakeSlice(reflect.SliceOf(c.valueType), 0, len(list.items))
	for _, v := range list.items {
		item, err := c.valueConverter.To(v)
		if err != nil {
			return nil, TypeErrorf("type error: failed to convert slice element: %v", err)
		}
		slice = reflect.Append(slice, reflect.ValueOf(item))
	}
	return slice.Interface(), nil
}

func (c *SliceConverter) From(iface interface{}) (Object, error) {
	v := reflect.ValueOf(iface)
	count := v.Len()
	items := make([]Object, 0, count)
	for i := 0; i < count; i++ {
		item, err := c.valueConverter.From(v.Index(i).Interface())
		if err != nil {
			return nil, TypeErrorf("type error: failed to convert slice element: %v", err)
		}
		items = append(items, item)
	}
	return NewList(items), nil
}

// newSliceConverter creates a TypeConverter for slices containing the given
// value type, where the items can be converted using the given TypeConverter.
func newSliceConverter(indirectType reflect.Type) (*SliceConverter, error) {
	indirectConv, err := createTypeConverter(indirectType)
	if err != nil {
		return nil, err
	}
	return &SliceConverter{
		valueType:      indirectType,
		valueConverter: indirectConv,
	}, nil
}

// ArrayConverter converts between []T and the Risor equivalent of []T.
type ArrayConverter struct {
	valueConverter TypeConverter
	valueType      reflect.Type
	len            int
}

func (c *ArrayConverter) To(obj Object) (interface{}, error) {
	list, ok := obj.(*List)
	if !ok {
		return nil, TypeErrorf("type error: expected a list (%s given)", obj.Type())
	}
	array := reflect.New(reflect.ArrayOf(c.len, c.valueType))
	arrayElem := array.Elem()
	for i, v := range list.items {
		item, err := c.valueConverter.To(v)
		if err != nil {
			return nil, TypeErrorf("type error: failed to convert element: %v", err)
		}
		arrayElem.Index(i).Set(reflect.ValueOf(item))
	}
	return arrayElem.Interface(), nil
}

func (c *ArrayConverter) From(iface interface{}) (Object, error) {
	v := reflect.ValueOf(iface)
	count := v.Len()
	items := make([]Object, 0, count)
	for i := 0; i < count; i++ {
		item, err := c.valueConverter.From(v.Index(i).Interface())
		if err != nil {
			return nil, TypeErrorf("type error: failed to convert slice element: %v", err)
		}
		items = append(items, item)
	}
	return NewList(items), nil
}

// newArrayConverter creates a TypeConverter for arrays containing the given
// value type, where the items can be converted using the given TypeConverter.
func newArrayConverter(indirectType reflect.Type, length int) (*ArrayConverter, error) {
	if length < 0 {
		return nil, fmt.Errorf("value error: invalid array length: %d", length)
	}
	indirectConv, err := createTypeConverter(indirectType)
	if err != nil {
		return nil, err
	}
	return &ArrayConverter{
		valueType:      indirectType,
		valueConverter: indirectConv,
		len:            length,
	}, nil
}

// ErrorConverter converts between error and *Error or *String.
type ErrorConverter struct{}

func (c *ErrorConverter) To(obj Object) (interface{}, error) {
	switch obj := obj.(type) {
	case *Error:
		return obj.Value(), nil
	case *String:
		return errors.New(obj.Value()), nil
	default:
		return nil, TypeErrorf("type error: expected a string (%s given)", obj.Type())
	}
}

func (c *ErrorConverter) From(obj interface{}) (Object, error) {
	return NewError(obj.(error)), nil
}

// ContextConverter converts between context.Context and Context.
type ContextConverter struct{}

func (c *ContextConverter) To(obj Object) (interface{}, error) {
	// Not actually called, but needed to satisfy the Converter interface.
	return nil, errors.New("not implemented")
}

func (c *ContextConverter) From(obj interface{}) (Object, error) {
	// Not actually called, but needed to satisfy the Converter interface.
	return nil, errors.New("not implemented")
}
