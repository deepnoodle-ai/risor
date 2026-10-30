package object

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v2/pkg/bytecode"
)

var (
	errorInterface   = reflect.TypeOf((*error)(nil)).Elem()
	contextInterface = reflect.TypeOf((*context.Context)(nil)).Elem()
	bytesType        = reflect.TypeOf([]byte(nil))
	anySliceType     = reflect.TypeOf([]any(nil))
	anyMapType       = reflect.TypeOf(map[string]any(nil))
)

// *****************************************************************************
// Type assertion helpers
// *****************************************************************************

func AsBool(obj Object) (bool, error) {
	b, ok := obj.(*Bool)
	if !ok {
		return false, newTypeErrorf("expected a bool (%s given)", obj.Type())
	}
	return b.value, nil
}

func AsString(obj Object) (string, error) {
	switch obj := obj.(type) {
	case *String:
		return obj.value, nil
	default:
		return "", newTypeErrorf("expected a string (%s given)", obj.Type())
	}
}

func AsInt(obj Object) (int64, error) {
	switch obj := obj.(type) {
	case *Int:
		return obj.value, nil
	case *Byte:
		return int64(obj.value), nil
	default:
		return 0, newTypeErrorf("expected an integer (%s given)", obj.Type())
	}
}

func AsByte(obj Object) (byte, error) {
	switch obj := obj.(type) {
	case *Int:
		return byte(obj.value), nil
	case *Byte:
		return obj.value, nil
	case *Float:
		return floatToByte(obj.value)
	case *String:
		if len(obj.value) != 1 {
			return 0, newTypeErrorf("expected a single byte string (length %d)", len(obj.value))
		}
		return obj.value[0], nil
	default:
		return 0, newTypeErrorf("expected a byte (%s given)", obj.Type())
	}
}

// floatToByte converts f the way byte(int) treats an integer: truncate toward
// zero, then wrap modulo 256. Go leaves out-of-range float-to-integer
// conversion to the platform, so the wrap is done in float64, where math.Mod
// is exact. NaN and ±Inf have no integer value and raise a value error.
func floatToByte(f float64) (byte, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, newValueErrorf("cannot convert %v to byte", f)
	}
	m := math.Mod(math.Trunc(f), 256) // an integer in (-256, 256)
	if m < 0 {
		m += 256
	}
	return byte(m), nil
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
		return 0.0, newTypeErrorf("expected a number (%s given)", obj.Type())
	}
}

func AsList(obj Object) (*List, error) {
	list, ok := obj.(*List)
	if !ok {
		return nil, newTypeErrorf("expected a list (%s given)", obj.Type())
	}
	return list, nil
}

func AsStringSlice(obj Object) ([]string, error) {
	list, ok := obj.(*List)
	if !ok {
		return nil, newTypeErrorf("expected a list (%s given)", obj.Type())
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
		return nil, newTypeErrorf("expected a map (%s given)", obj.Type())
	}
	return m, nil
}

func AsTime(obj Object) (time.Time, error) {
	t, ok := obj.(*Time)
	if !ok {
		return time.Time{}, newTypeErrorf("expected a time (%s given)", obj.Type())
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
		return nil, newTypeErrorf("expected bytes (%s given)", obj.Type())
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
		return nil, newTypeErrorf("expected a readable object (%s given)", obj.Type())
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
		return nil, newTypeErrorf("expected a writable object (%s given)", obj.Type())
	}
}

func AsError(obj Object) (*Error, error) {
	err, ok := obj.(*Error)
	if !ok {
		return nil, newTypeErrorf("expected an error object (%s given)", obj.Type())
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

// FromGo converts a Go value to a Risor Object. It returns ErrNestingDepth if
// v is nested more deeply than MaxNestingDepth or contains a cycle.
func (r *TypeRegistry) FromGo(v any) (Object, error) {
	return r.fromGoPath(v, nil)
}

// goRef identifies a Go slice, map, or pointer visited by FromGo, for cycle
// detection. Arrays are values and have no identity, so their ptr is zero.
type goRef struct {
	typ reflect.Type
	ptr uintptr
	len int
}

// enterGo returns the path extended with ref, or ErrNestingDepth if the path
// is full or ref is already on it.
func enterGo(path []goRef, ref goRef) ([]goRef, error) {
	if len(path) >= MaxNestingDepth {
		return nil, ErrNestingDepth
	}
	if ref.ptr != 0 {
		for _, ancestor := range path {
			if ancestor == ref {
				return nil, ErrNestingDepth
			}
		}
	}
	return append(path, ref), nil
}

// fromGoPath converts v, where path holds the Go values enclosing it.
func (r *TypeRegistry) fromGoPath(v any, path []goRef) (Object, error) {
	// A nil pointer is null, whatever its type. Check before the interface
	// checks below: a typed nil Object must not reach the VM, and calling
	// RisorValue or a registered converter on a nil receiver can panic.
	if isNilValue(v) {
		return Nil, nil
	}

	// Check if value implements RisorValuer
	if rv, ok := v.(RisorValuer); ok {
		obj := rv.RisorValue()
		if isNilValue(obj) {
			return Nil, nil
		}
		return obj, nil
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
	return r.fromGoByKind(v, typ, path)
}

// isNilValue reports whether v is nil or a nil pointer.
func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}

// fromGoField converts an addressable struct field. A field that holds a
// struct by value becomes a GoStruct that points at the field, so writes
// through it change the enclosing struct, not a copy. A struct type with a
// registered converter, such as time.Time, or one that implements
// RisorValuer converts as FromGo does.
func (r *TypeRegistry) fromGoField(field reflect.Value) (Object, error) {
	if field.Kind() == reflect.Struct && field.CanAddr() {
		if _, ok := r.fromGo[field.Type()]; !ok {
			if _, ok := field.Interface().(RisorValuer); !ok {
				return NewGoStruct(field.Addr(), r), nil
			}
		}
	}
	return r.FromGo(field.Interface())
}

func (r *TypeRegistry) fromGoByKind(v any, typ reflect.Type, path []goRef) (Object, error) {
	rv := reflect.ValueOf(v)

	if isSyncType(typ) {
		return nil, fmt.Errorf("unsupported type: %s (sync types are not available to scripts)", typ)
	}

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
		return r.fromGoSlice(rv, path)
	case reflect.Array:
		return r.fromGoArray(rv, path)
	case reflect.Map:
		return r.fromGoMap(rv, path)
	case reflect.Ptr:
		if rv.IsNil() {
			return Nil, nil
		}
		// Check if it's a pointer to a struct
		if rv.Elem().Kind() == reflect.Struct {
			return NewGoStruct(rv, r), nil
		}
		path, err := enterGo(path, goRef{typ: typ, ptr: rv.Pointer()})
		if err != nil {
			return nil, err
		}
		return r.fromGoPath(rv.Elem().Interface(), path)
	case reflect.Interface:
		if rv.IsNil() {
			return Nil, nil
		}
		return r.fromGoPath(rv.Elem().Interface(), path)
	case reflect.Func:
		if rv.IsNil() {
			return Nil, nil
		}
		return NewGoFunc(rv, typ.String(), r), nil
	case reflect.Struct:
		// Wrap as GoStruct (create pointer for addressability)
		ptrVal := reflect.New(typ)
		ptrVal.Elem().Set(rv)
		return NewGoStruct(ptrVal, r), nil
	default:
		return nil, fmt.Errorf("unsupported type: %s (kind: %s)", typ, typ.Kind())
	}
}

func (r *TypeRegistry) fromGoSlice(rv reflect.Value, path []goRef) (Object, error) {
	// Special case for []byte
	if rv.Type().Elem().Kind() == reflect.Uint8 {
		return NewBytes(rv.Bytes()), nil
	}

	count := rv.Len()
	path, err := enterGo(path, goRef{typ: rv.Type(), ptr: rv.Pointer(), len: count})
	if err != nil {
		return nil, err
	}
	items := make([]Object, 0, count)
	for i := 0; i < count; i++ {
		item, err := r.fromGoPath(rv.Index(i).Interface(), path)
		if errors.Is(err, ErrNestingDepth) {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("failed to convert slice element %d: %w", i, err)
		}
		items = append(items, item)
	}
	return NewList(items), nil
}

func (r *TypeRegistry) fromGoArray(rv reflect.Value, path []goRef) (Object, error) {
	count := rv.Len()
	path, err := enterGo(path, goRef{typ: rv.Type()})
	if err != nil {
		return nil, err
	}
	items := make([]Object, 0, count)
	for i := 0; i < count; i++ {
		item, err := r.fromGoPath(rv.Index(i).Interface(), path)
		if errors.Is(err, ErrNestingDepth) {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
		}
		items = append(items, item)
	}
	return NewList(items), nil
}

func (r *TypeRegistry) fromGoMap(rv reflect.Value, path []goRef) (Object, error) {
	if rv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("unsupported map key type: %s (only string keys supported)", rv.Type().Key())
	}

	path, err := enterGo(path, goRef{typ: rv.Type(), ptr: rv.Pointer()})
	if err != nil {
		return nil, err
	}
	result := make(map[string]Object, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		key := iter.Key().String()
		val, err := r.fromGoPath(iter.Value().Interface(), path)
		if errors.Is(err, ErrNestingDepth) {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("failed to convert map value for key %q: %w", key, err)
		}
		result[key] = val
	}
	return NewMap(result), nil
}

// ToGo converts a Risor Object to a Go value of the specified type. A range
// converts to a slice or array as a list of its values does. It returns
// ErrNestingDepth if obj is nested more deeply than MaxNestingDepth or
// contains a cycle. It returns a value error if the result would have more
// elements in total than the list length limit in ctx, counted as for
// ToInterface. Registered converters are called as is and are not counted.
func (r *TypeRegistry) ToGo(ctx context.Context, obj Object, targetType reflect.Type) (any, error) {
	return r.toGoPath(obj, targetType, nil, newConvBudget(ctx))
}

// toGoPath converts obj, where path holds the containers enclosing it and b
// is the element budget of the conversion.
func (r *TypeRegistry) toGoPath(obj Object, targetType reflect.Type, path nestPath[Object], b *convBudget) (any, error) {
	// Handle nil
	if obj == nil || obj.Type() == NIL {
		return reflect.Zero(targetType).Interface(), nil
	}

	// A wrapped Go value passes back to Go as itself
	if v, ok := unwrapGo(obj, targetType); ok {
		return v, nil
	}

	// Check for registered converter
	if fn, ok := r.toGo[targetType]; ok {
		return fn(obj, targetType)
	}

	// A wrapped Go value of another type is a type error. A pointer target
	// is left to toGoPointer, which converts to the element type, such as
	// any for *any.
	if targetType.Kind() != reflect.Ptr {
		switch obj := obj.(type) {
		case *GoStruct:
			return nil, newTypeErrorf("cannot use %s as %s", obj.value.Type(), targetType)
		case *GoFunc:
			return nil, newTypeErrorf("cannot use %s as %s", obj.fnType, targetType)
		}
	}

	// Handle by kind
	return r.toGoByKind(obj, targetType, path, b)
}

// toGoValue converts obj as toGoPath does and returns the result as a
// reflect.Value of type target, ready to pass to Set, Append, or Call.
func (r *TypeRegistry) toGoValue(obj Object, target reflect.Type, path nestPath[Object], b *convBudget) (reflect.Value, error) {
	v, err := r.toGoPath(obj, target, path, b)
	if err != nil {
		return reflect.Value{}, err
	}
	return reflectValue(v, target)
}

// reflectValue returns v as a reflect.Value of type t. A nil v, which is what
// null converts to for an interface target, becomes the zero value of t, so
// that it is never the invalid reflect.Value that Set, Append, and Call reject.
func reflectValue(v any, t reflect.Type) (reflect.Value, error) {
	if v == nil {
		return reflect.Zero(t), nil
	}
	rv := reflect.ValueOf(v)
	if !rv.Type().AssignableTo(t) {
		return reflect.Value{}, newTypeErrorf("cannot use %s as %s", rv.Type(), t)
	}
	return rv, nil
}

// unwrapGo returns the Go value that a GoStruct or GoFunc wraps, if it can be
// assigned to target. A struct target receives a copy of the struct.
func unwrapGo(obj Object, target reflect.Type) (any, bool) {
	switch obj := obj.(type) {
	case *GoStruct:
		if obj.value.Type().AssignableTo(target) {
			return obj.value.Interface(), true
		}
		if !obj.value.IsNil() && obj.structType.AssignableTo(target) {
			return obj.value.Elem().Interface(), true
		}
	case *GoFunc:
		if obj.fnType.AssignableTo(target) {
			return obj.fn.Interface(), true
		}
	}
	return nil, false
}

// convertTo converts v to target when target is a defined type, such as
// time.Duration, whose underlying type is the type of v.
func convertTo(v any, err error, target reflect.Type) (any, error) {
	if err != nil {
		return nil, err
	}
	rv := reflect.ValueOf(v)
	if rv.Type() == target {
		return v, nil
	}
	return rv.Convert(target).Interface(), nil
}

func (r *TypeRegistry) toGoByKind(obj Object, target reflect.Type, path nestPath[Object], b *convBudget) (any, error) {
	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		v, err := toNumeric(obj, target)
		return convertTo(v, err, target)
	case reflect.Bool:
		v, err := toBool(obj)
		return convertTo(v, err, target)
	case reflect.String:
		v, err := toGoString(obj)
		return convertTo(v, err, target)
	case reflect.Slice:
		return r.toGoSlice(obj, target, path, b)
	case reflect.Array:
		return r.toGoArray(obj, target, path, b)
	case reflect.Map:
		return r.toGoMap(obj, target, path, b)
	case reflect.Ptr:
		return r.toGoPointer(obj, target, path, b)
	case reflect.Interface:
		if target.NumMethod() == 0 {
			// any / interface{}. Lists and maps convert element by element
			// as []any and map[string]any, so that a nested value without a
			// Go form is an error here too.
			switch obj.(type) {
			case *List:
				return r.toGoSlice(obj, anySliceType, path, b)
			case *Map:
				return r.toGoMap(obj, anyMapType, path, b)
			}
			v, err := toInterface(obj, path, b)
			if err == nil && v == nil {
				// Only null may become Go nil. Closures, builtins, and
				// modules have no Go value.
				return nil, newTypeErrorf("cannot convert %s to a Go value", obj.Type())
			}
			return v, err
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

func (r *TypeRegistry) toGoSlice(obj Object, target reflect.Type, path nestPath[Object], b *convBudget) (any, error) {
	if obj.Type() == NIL {
		return reflect.Zero(target).Interface(), nil
	}

	// Special case: []byte, or a defined type such as json.RawMessage, from
	// Bytes or String
	if bytesType.ConvertibleTo(target) {
		switch v := obj.(type) {
		case *Bytes:
			return convertTo(v.value, nil, target)
		case *String:
			return convertTo([]byte(v.value), nil, target)
		}
	}
	// A slice of a defined byte type, such as []Role where Role is uint8.
	// fromGoSlice converts these to Bytes, so accept Bytes back. Go doesn't
	// allow converting []byte to []Role directly, so copy element by element.
	if target.Elem().Kind() == reflect.Uint8 {
		var data []byte
		ok := true
		switch v := obj.(type) {
		case *Bytes:
			data = v.value
		case *String:
			data = []byte(v.value)
		default:
			ok = false
		}
		if ok {
			slice := reflect.MakeSlice(target, len(data), len(data))
			for i, c := range data {
				slice.Index(i).SetUint(uint64(c))
			}
			return slice.Interface(), nil
		}
	}

	items, err := sequenceItems(obj, math.MaxInt, b)
	if err != nil {
		return nil, err
	}

	elemType := target.Elem()
	slice := reflect.MakeSlice(target, 0, len(items))
	for i, item := range items {
		if isNestedContainer(item) {
			var err error
			if path, err = path.descend(obj, item); err != nil {
				return nil, err
			}
		}
		elem, err := r.toGoValue(item, elemType, path, b)
		if errors.Is(err, ErrNestingDepth) {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("failed to convert slice element %d: %w", i, err)
		}
		slice = reflect.Append(slice, elem)
	}
	return slice.Interface(), nil
}

func (r *TypeRegistry) toGoArray(obj Object, target reflect.Type, path nestPath[Object], b *convBudget) (any, error) {
	elemType := target.Elem()
	items, err := sequenceItems(obj, target.Len(), b)
	if err != nil {
		return nil, err
	}
	array := reflect.New(target).Elem()

	for i, item := range items {
		if isNestedContainer(item) {
			var err error
			if path, err = path.descend(obj, item); err != nil {
				return nil, err
			}
		}
		elem, err := r.toGoValue(item, elemType, path, b)
		if errors.Is(err, ErrNestingDepth) {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
		}
		array.Index(i).Set(elem)
	}
	return array.Interface(), nil
}

func (r *TypeRegistry) toGoMap(obj Object, target reflect.Type, path nestPath[Object], b *convBudget) (any, error) {
	if obj.Type() == NIL {
		return reflect.Zero(target).Interface(), nil
	}

	m, ok := obj.(*Map)
	if !ok {
		return nil, newTypeErrorf("expected a map, got %s", obj.Type())
	}

	if target.Key().Kind() != reflect.String {
		return nil, fmt.Errorf("unsupported map key type: %s", target.Key())
	}

	if err := b.take(uint64(m.Size())); err != nil {
		return nil, err
	}
	keyType := target.Key()
	valueType := target.Elem()
	result := reflect.MakeMapWithSize(target, m.Size())

	for k, v := range m.items {
		if isNestedContainer(v) {
			var err error
			if path, err = path.descend(m, v); err != nil {
				return nil, err
			}
		}
		goValue, err := r.toGoValue(v, valueType, path, b)
		if errors.Is(err, ErrNestingDepth) {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("failed to convert map value for key %q: %w", k, err)
		}
		// A null value stores the zero value. It never deletes the key.
		result.SetMapIndex(reflect.ValueOf(k).Convert(keyType), goValue)
	}
	return result.Interface(), nil
}

func (r *TypeRegistry) toGoPointer(obj Object, target reflect.Type, path nestPath[Object], b *convBudget) (any, error) {
	if obj.Type() == NIL {
		return reflect.Zero(target).Interface(), nil
	}

	elemType := target.Elem()
	elem, err := r.toGoValue(obj, elemType, path, b)
	if err != nil {
		return nil, err
	}

	ptr := reflect.New(elemType)
	ptr.Elem().Set(elem)
	return ptr.Interface(), nil
}

// sequenceItems returns up to max items of a list, or values of a range, for
// conversion to a Go slice or array. It charges them to b before building
// any range values.
func sequenceItems(obj Object, max int, b *convBudget) ([]Object, error) {
	switch obj := obj.(type) {
	case *List:
		items := obj.items[:min(len(obj.items), max)]
		if err := b.take(uint64(len(items))); err != nil {
			return nil, err
		}
		return items, nil
	case *Range:
		n := min(obj.length(), uint64(max))
		if err := b.take(n); err != nil {
			return nil, err
		}
		values := obj.values(n)
		items := make([]Object, len(values))
		for i, v := range values {
			items[i] = NewInt(v)
		}
		return items, nil
	}
	return nil, newTypeErrorf("expected a list, got %s", obj.Type())
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
		return nil, newTypeErrorf("expected number, got %s", obj.Type())
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
		return false, newTypeErrorf("expected bool, got %s", obj.Type())
	}
	return b.value, nil
}

func toGoString(obj Object) (string, error) {
	s, ok := obj.(*String)
	if !ok {
		return "", newTypeErrorf("expected string, got %s", obj.Type())
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
		return nil, newTypeErrorf("expected error, got %s", obj.Type())
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

var (
	defaultRegistry     *TypeRegistry
	defaultRegistryOnce sync.Once
)

// DefaultRegistry returns a TypeRegistry with converters for all built-in types.
// It is safe for concurrent use.
func DefaultRegistry() *TypeRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = createDefaultRegistry()
	})
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
					return nil, newTypeErrorf("expected time, got %s", obj.Type())
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
		return TypeErrorf("unmarshaling %v (%v): %v",
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
			return 0, newTypeErrorf("expected single rune string (got length %d)", utf8.RuneCountInString(v.value))
		}
		r, _ := utf8.DecodeRuneInString(v.value)
		return r, nil
	case *Int:
		return rune(v.value), nil
	default:
		return 0, newTypeErrorf("expected string or int, got %s", obj.Type())
	}
}
