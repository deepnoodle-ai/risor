package object

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"
	"unicode/utf8"

	"github.com/risor-io/risor/bytecode"
)

var (
	errorInterface   = reflect.TypeOf((*error)(nil)).Elem()
	contextInterface = reflect.TypeOf((*context.Context)(nil)).Elem()
)

// *****************************************************************************
// Type assertion helpers
// *****************************************************************************

func AsBool(obj Object) (bool, error) {
	b, ok := obj.(*Bool)
	if !ok {
		return false, fmt.Errorf("type error: expected a bool (%s given)", obj.Type())
	}
	return b.value, nil
}

func AsString(obj Object) (string, error) {
	switch obj := obj.(type) {
	case *String:
		return obj.value, nil
	default:
		return "", fmt.Errorf("type error: expected a string (%s given)", obj.Type())
	}
}

func AsInt(obj Object) (int64, error) {
	switch obj := obj.(type) {
	case *Int:
		return obj.value, nil
	case *Byte:
		return int64(obj.value), nil
	default:
		return 0, fmt.Errorf("type error: expected an integer (%s given)", obj.Type())
	}
}

func AsByte(obj Object) (byte, error) {
	switch obj := obj.(type) {
	case *Int:
		return byte(obj.value), nil
	case *Byte:
		return obj.value, nil
	case *Float:
		return byte(obj.value), nil
	case *String:
		if len(obj.value) != 1 {
			return 0, fmt.Errorf("type error: expected a single byte string (length %d)", len(obj.value))
		}
		return obj.value[0], nil
	default:
		return 0, fmt.Errorf("type error: expected a byte (%s given)", obj.Type())
	}
}

func AsFloat(obj Object) (float64, error) {
	switch obj := obj.(type) {
	case *Int:
		return float64(obj.value), nil
	case *Byte:
		return float64(obj.value), nil
	case *Float:
		return obj.value, nil
	default:
		return 0.0, fmt.Errorf("type error: expected a number (%s given)", obj.Type())
	}
}

func AsList(obj Object) (*List, error) {
	list, ok := obj.(*List)
	if !ok {
		return nil, fmt.Errorf("type error: expected a list (%s given)", obj.Type())
	}
	return list, nil
}

func AsStringSlice(obj Object) ([]string, error) {
	list, ok := obj.(*List)
	if !ok {
		return nil, fmt.Errorf("type error: expected a list (%s given)", obj.Type())
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

func AsMap(obj Object) (*Map, error) {
	m, ok := obj.(*Map)
	if !ok {
		return nil, fmt.Errorf("type error: expected a map (%s given)", obj.Type())
	}
	return m, nil
}

func AsTime(obj Object) (time.Time, error) {
	t, ok := obj.(*Time)
	if !ok {
		return time.Time{}, fmt.Errorf("type error: expected a time (%s given)", obj.Type())
	}
	return t.value, nil
}

func AsBytes(obj Object) ([]byte, error) {
	switch obj := obj.(type) {
	case *Bytes:
		return obj.value, nil
	case *String:
		return []byte(obj.value), nil
	case io.Reader:
		data, err := io.ReadAll(obj)
		if err != nil {
			return nil, err
		}
		return data, nil
	default:
		return nil, fmt.Errorf("type error: expected bytes (%s given)", obj.Type())
	}
}

func AsReader(obj Object) (io.Reader, error) {
	if o, ok := obj.(interface{ AsReader() (io.Reader, error) }); ok {
		return o.AsReader()
	}
	switch obj := obj.(type) {
	case *Bytes:
		return bytes.NewBuffer(obj.value), nil
	case *String:
		return bytes.NewBufferString(obj.value), nil
	case io.Reader:
		return obj, nil
	default:
		return nil, fmt.Errorf("type error: expected a readable object (%s given)", obj.Type())
	}
}

func AsWriter(obj Object) (io.Writer, error) {
	if o, ok := obj.(interface{ AsWriter() (io.Writer, error) }); ok {
		return o.AsWriter()
	}
	switch obj := obj.(type) {
	case io.Writer:
		return obj, nil
	default:
		return nil, fmt.Errorf("type error: expected a writable object (%s given)", obj.Type())
	}
}

func AsError(obj Object) (*Error, error) {
	err, ok := obj.(*Error)
	if !ok {
		return nil, fmt.Errorf("type error: expected an error object (%s given)", obj.Type())
	}
	return err, nil
}

// *****************************************************************************
// RisorValuer interface
// *****************************************************************************

// RisorValuer is implemented by Go types that know how to become Risor Objects.
// When converting Go values to Risor Objects, the TypeRegistry checks for this
// interface first, allowing custom types to define their own conversion.
type RisorValuer interface {
	RisorValue() Object
}

// *****************************************************************************
// TypeRegistry
// *****************************************************************************

// FromGoFunc converts a Go value to a Risor Object.
type FromGoFunc func(v any) (Object, error)

// ToGoFunc converts a Risor Object to a Go value of a specific type.
type ToGoFunc func(obj Object, targetType reflect.Type) (any, error)

// TypeRegistry handles conversion between Go values and Risor Objects.
// It is immutable after construction and safe for concurrent use.
type TypeRegistry struct {
	fromGo map[reflect.Type]FromGoFunc
	toGo   map[reflect.Type]ToGoFunc
}

// FromGo converts a Go value to a Risor Object.
func (r *TypeRegistry) FromGo(v any) (Object, error) {
	if v == nil {
		return Nil, nil
	}

	// Check if value implements RisorValuer
	if rv, ok := v.(RisorValuer); ok {
		return rv.RisorValue(), nil
	}

	// Check if value is already an Object
	if obj, ok := v.(Object); ok {
		return obj, nil
	}

	typ := reflect.TypeOf(v)

	// Check for exact type match
	if fn, ok := r.fromGo[typ]; ok {
		return fn(v)
	}

	// Handle by kind for common cases
	return r.fromGoByKind(v, typ)
}

func (r *TypeRegistry) fromGoByKind(v any, typ reflect.Type) (Object, error) {
	rv := reflect.ValueOf(v)

	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return NewInt(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return NewInt(int64(rv.Uint())), nil
	case reflect.Float32, reflect.Float64:
		return NewFloat(rv.Float()), nil
	case reflect.Bool:
		return NewBool(rv.Bool()), nil
	case reflect.String:
		return NewString(rv.String()), nil
	case reflect.Slice:
		return r.fromGoSlice(rv)
	case reflect.Array:
		return r.fromGoArray(rv)
	case reflect.Map:
		return r.fromGoMap(rv)
	case reflect.Ptr:
		if rv.IsNil() {
			return Nil, nil
		}
		return r.FromGo(rv.Elem().Interface())
	case reflect.Interface:
		if rv.IsNil() {
			return Nil, nil
		}
		return r.FromGo(rv.Elem().Interface())
	case reflect.Func:
		// Functions can't be converted automatically
		return nil, fmt.Errorf("cannot convert function type %s to Risor object", typ)
	default:
		return nil, fmt.Errorf("unsupported type: %s (kind: %s)", typ, typ.Kind())
	}
}

func (r *TypeRegistry) fromGoSlice(rv reflect.Value) (Object, error) {
	// Special case for []byte
	if rv.Type().Elem().Kind() == reflect.Uint8 {
		return NewBytes(rv.Bytes()), nil
	}

	count := rv.Len()
	items := make([]Object, 0, count)
	for i := 0; i < count; i++ {
		item, err := r.FromGo(rv.Index(i).Interface())
		if err != nil {
			return nil, fmt.Errorf("failed to convert slice element %d: %w", i, err)
		}
		items = append(items, item)
	}
	return NewList(items), nil
}

func (r *TypeRegistry) fromGoArray(rv reflect.Value) (Object, error) {
	count := rv.Len()
	items := make([]Object, 0, count)
	for i := 0; i < count; i++ {
		item, err := r.FromGo(rv.Index(i).Interface())
		if err != nil {
			return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
		}
		items = append(items, item)
	}
	return NewList(items), nil
}

func (r *TypeRegistry) fromGoMap(rv reflect.Value) (Object, error) {
	if rv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("unsupported map key type: %s (only string keys supported)", rv.Type().Key())
	}

	result := make(map[string]Object, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		key := iter.Key().String()
		val, err := r.FromGo(iter.Value().Interface())
		if err != nil {
			return nil, fmt.Errorf("failed to convert map value for key %q: %w", key, err)
		}
		result[key] = val
	}
	return NewMap(result), nil
}

// ToGo converts a Risor Object to a Go value of the specified type.
func (r *TypeRegistry) ToGo(obj Object, targetType reflect.Type) (any, error) {
	// Handle nil
	if obj == nil || obj.Type() == NIL {
		return reflect.Zero(targetType).Interface(), nil
	}

	// Check for registered converter
	if fn, ok := r.toGo[targetType]; ok {
		return fn(obj, targetType)
	}

	// Handle by kind
	return r.toGoByKind(obj, targetType)
}

func (r *TypeRegistry) toGoByKind(obj Object, target reflect.Type) (any, error) {
	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return toNumeric(obj, target)
	case reflect.Bool:
		return toBool(obj)
	case reflect.String:
		return toGoString(obj)
	case reflect.Slice:
		return r.toGoSlice(obj, target)
	case reflect.Array:
		return r.toGoArray(obj, target)
	case reflect.Map:
		return r.toGoMap(obj, target)
	case reflect.Ptr:
		return r.toGoPointer(obj, target)
	case reflect.Interface:
		if target.NumMethod() == 0 {
			// any / interface{}
			return obj.Interface(), nil
		}
		// Check for specific interfaces
		if target.Implements(errorInterface) {
			return toGoError(obj)
		}
		if target.Implements(contextInterface) {
			return nil, errors.New("context conversion not supported via ToGo")
		}
		return nil, fmt.Errorf("unsupported interface type: %s", target)
	default:
		return nil, fmt.Errorf("unsupported target type: %s (kind: %s)", target, target.Kind())
	}
}

func (r *TypeRegistry) toGoSlice(obj Object, target reflect.Type) (any, error) {
	if obj.Type() == NIL {
		return reflect.Zero(target).Interface(), nil
	}

	// Special case: []byte from Bytes or String
	if target.Elem().Kind() == reflect.Uint8 {
		switch v := obj.(type) {
		case *Bytes:
			return v.value, nil
		case *String:
			return []byte(v.value), nil
		}
	}

	list, ok := obj.(*List)
	if !ok {
		return nil, fmt.Errorf("type error: expected a list, got %s", obj.Type())
	}

	elemType := target.Elem()
	slice := reflect.MakeSlice(target, 0, len(list.items))
	for i, item := range list.items {
		elem, err := r.ToGo(item, elemType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert slice element %d: %w", i, err)
		}
		slice = reflect.Append(slice, reflect.ValueOf(elem))
	}
	return slice.Interface(), nil
}

func (r *TypeRegistry) toGoArray(obj Object, target reflect.Type) (any, error) {
	list, ok := obj.(*List)
	if !ok {
		return nil, fmt.Errorf("type error: expected a list, got %s", obj.Type())
	}

	elemType := target.Elem()
	arrayLen := target.Len()
	array := reflect.New(target).Elem()

	for i, item := range list.items {
		if i >= arrayLen {
			break
		}
		elem, err := r.ToGo(item, elemType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
		}
		array.Index(i).Set(reflect.ValueOf(elem))
	}
	return array.Interface(), nil
}

func (r *TypeRegistry) toGoMap(obj Object, target reflect.Type) (any, error) {
	if obj.Type() == NIL {
		return reflect.Zero(target).Interface(), nil
	}

	m, ok := obj.(*Map)
	if !ok {
		return nil, fmt.Errorf("type error: expected a map, got %s", obj.Type())
	}

	if target.Key().Kind() != reflect.String {
		return nil, fmt.Errorf("unsupported map key type: %s", target.Key())
	}

	valueType := target.Elem()
	result := reflect.MakeMapWithSize(target, m.Size())

	for k, v := range m.items {
		goValue, err := r.ToGo(v, valueType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert map value for key %q: %w", k, err)
		}
		result.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(goValue))
	}
	return result.Interface(), nil
}

func (r *TypeRegistry) toGoPointer(obj Object, target reflect.Type) (any, error) {
	if obj.Type() == NIL {
		return reflect.Zero(target).Interface(), nil
	}

	elemType := target.Elem()
	elem, err := r.ToGo(obj, elemType)
	if err != nil {
		return nil, err
	}

	ptr := reflect.New(elemType)
	ptr.Elem().Set(reflect.ValueOf(elem))
	return ptr.Interface(), nil
}

// toNumeric handles all numeric conversions.
func toNumeric(obj Object, target reflect.Type) (any, error) {
	var intVal int64
	var floatVal float64
	var isFloat bool

	switch v := obj.(type) {
	case *Int:
		intVal = v.value
	case *Float:
		floatVal, isFloat = v.value, true
	case *Byte:
		intVal = int64(v.value)
	default:
		return nil, fmt.Errorf("type error: expected number, got %s", obj.Type())
	}

	switch target.Kind() {
	case reflect.Int:
		if isFloat {
			return int(floatVal), nil
		}
		return int(intVal), nil
	case reflect.Int8:
		if isFloat {
			return int8(floatVal), nil
		}
		return int8(intVal), nil
	case reflect.Int16:
		if isFloat {
			return int16(floatVal), nil
		}
		return int16(intVal), nil
	case reflect.Int32:
		if isFloat {
			return int32(floatVal), nil
		}
		return int32(intVal), nil
	case reflect.Int64:
		if isFloat {
			return int64(floatVal), nil
		}
		return intVal, nil
	case reflect.Uint:
		if isFloat {
			return uint(floatVal), nil
		}
		return uint(intVal), nil
	case reflect.Uint8:
		if isFloat {
			return uint8(floatVal), nil
		}
		return uint8(intVal), nil
	case reflect.Uint16:
		if isFloat {
			return uint16(floatVal), nil
		}
		return uint16(intVal), nil
	case reflect.Uint32:
		if isFloat {
			return uint32(floatVal), nil
		}
		return uint32(intVal), nil
	case reflect.Uint64:
		if isFloat {
			return uint64(floatVal), nil
		}
		return uint64(intVal), nil
	case reflect.Float32:
		if isFloat {
			return float32(floatVal), nil
		}
		return float32(intVal), nil
	case reflect.Float64:
		if isFloat {
			return floatVal, nil
		}
		return float64(intVal), nil
	default:
		return nil, fmt.Errorf("cannot convert to numeric type %s", target)
	}
}

func toBool(obj Object) (bool, error) {
	b, ok := obj.(*Bool)
	if !ok {
		return false, fmt.Errorf("type error: expected bool, got %s", obj.Type())
	}
	return b.value, nil
}

func toGoString(obj Object) (string, error) {
	s, ok := obj.(*String)
	if !ok {
		return "", fmt.Errorf("type error: expected string, got %s", obj.Type())
	}
	return s.value, nil
}

func toGoError(obj Object) (error, error) {
	switch v := obj.(type) {
	case *Error:
		return v.Value(), nil
	case *String:
		return errors.New(v.Value()), nil
	default:
		return nil, fmt.Errorf("type error: expected error, got %s", obj.Type())
	}
}

// *****************************************************************************
// RegistryBuilder
// *****************************************************************************

// RegistryBuilder constructs a TypeRegistry with custom converters.
type RegistryBuilder struct {
	base   *TypeRegistry
	fromGo map[reflect.Type]FromGoFunc
	toGo   map[reflect.Type]ToGoFunc
}

// NewRegistryBuilder creates a builder starting from the default registry.
func NewRegistryBuilder() *RegistryBuilder {
	return &RegistryBuilder{
		base:   DefaultRegistry(),
		fromGo: make(map[reflect.Type]FromGoFunc),
		toGo:   make(map[reflect.Type]ToGoFunc),
	}
}

// RegisterFromGo adds a converter for Go -> Risor.
func (b *RegistryBuilder) RegisterFromGo(typ reflect.Type, fn FromGoFunc) *RegistryBuilder {
	b.fromGo[typ] = fn
	return b
}

// RegisterToGo adds a converter for Risor -> Go.
func (b *RegistryBuilder) RegisterToGo(typ reflect.Type, fn ToGoFunc) *RegistryBuilder {
	b.toGo[typ] = fn
	return b
}

// Build creates an immutable TypeRegistry.
func (b *RegistryBuilder) Build() *TypeRegistry {
	fromGo := make(map[reflect.Type]FromGoFunc)
	toGo := make(map[reflect.Type]ToGoFunc)

	// Copy from base
	if b.base != nil {
		for k, v := range b.base.fromGo {
			fromGo[k] = v
		}
		for k, v := range b.base.toGo {
			toGo[k] = v
		}
	}

	// Override with custom converters
	for k, v := range b.fromGo {
		fromGo[k] = v
	}
	for k, v := range b.toGo {
		toGo[k] = v
	}

	return &TypeRegistry{fromGo: fromGo, toGo: toGo}
}

// *****************************************************************************
// Default Registry
// *****************************************************************************

var defaultRegistry *TypeRegistry

// DefaultRegistry returns a TypeRegistry with converters for all built-in types.
func DefaultRegistry() *TypeRegistry {
	if defaultRegistry == nil {
		defaultRegistry = createDefaultRegistry()
	}
	return defaultRegistry
}

func createDefaultRegistry() *TypeRegistry {
	return &TypeRegistry{
		fromGo: map[reflect.Type]FromGoFunc{
			// time.Time requires special handling
			reflect.TypeOf(time.Time{}): func(v any) (Object, error) {
				return NewTime(v.(time.Time)), nil
			},
			// json.Number requires special handling
			reflect.TypeOf(json.Number("")): func(v any) (Object, error) {
				n := v.(json.Number)
				if f, err := n.Float64(); err == nil {
					return NewFloat(f), nil
				}
				return NewString(n.String()), nil
			},
			// *bytecode.Function -> Closure
			reflect.TypeOf((*bytecode.Function)(nil)): func(v any) (Object, error) {
				return NewClosure(v.(*bytecode.Function)), nil
			},
		},
		toGo: map[reflect.Type]ToGoFunc{
			// time.Time
			reflect.TypeOf(time.Time{}): func(obj Object, _ reflect.Type) (any, error) {
				switch v := obj.(type) {
				case *Time:
					return v.value, nil
				case *String:
					return time.Parse(time.RFC3339, v.value)
				default:
					return nil, fmt.Errorf("type error: expected time, got %s", obj.Type())
				}
			},
		},
	}
}

// *****************************************************************************
// Convenience functions
// *****************************************************************************

// FromGoType converts a Go value to a Risor Object using the default registry.
// On error, returns an *Error object (for backward compatibility).
// Prefer using TypeRegistry.FromGo for new code.
func FromGoType(obj interface{}) Object {
	result, err := DefaultRegistry().FromGo(obj)
	if err != nil {
		return TypeErrorf("type error: unmarshaling %v (%v): %v",
			obj, reflect.TypeOf(obj), err)
	}
	return result
}

// AsObjects transforms a map containing arbitrary Go types to a map of
// Risor objects, using the default registry. If an item in the map is of a
// type that can't be converted, an error is returned.
func AsObjects(m map[string]any) (map[string]Object, error) {
	return AsObjectsWithRegistry(m, DefaultRegistry())
}

// AsObjectsWithRegistry transforms a map containing arbitrary Go types to a map
// of Risor objects using the specified registry.
func AsObjectsWithRegistry(m map[string]any, registry *TypeRegistry) (map[string]Object, error) {
	result := make(map[string]Object, len(m))
	for k, v := range m {
		obj, err := registry.FromGo(v)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %q: %w", k, err)
		}
		result[k] = obj
	}
	return result, nil
}

// *****************************************************************************
// Rune conversion helper
// *****************************************************************************

// RuneToObject converts a rune to a Risor String object.
func RuneToObject(r rune) Object {
	return NewString(string([]rune{r}))
}

// ObjectToRune converts a Risor Object to a rune.
func ObjectToRune(obj Object) (rune, error) {
	switch v := obj.(type) {
	case *String:
		if utf8.RuneCountInString(v.value) != 1 {
			return 0, fmt.Errorf("type error: expected single rune string (got length %d)", utf8.RuneCountInString(v.value))
		}
		r, _ := utf8.DecodeRuneInString(v.value)
		return r, nil
	case *Int:
		return rune(v.value), nil
	default:
		return 0, fmt.Errorf("type error: expected string or int, got %s", obj.Type())
	}
}
