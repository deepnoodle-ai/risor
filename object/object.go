// Package object provides the standard set of Risor object types.
//
// For external users of Risor, often an object.Object interface
// will be type asserted to a specific object type, such as *object.Float.
//
// For example:
//
//	switch obj := obj.(type) {
//	case *object.String:
//		// do something with obj.Value()
//	case *object.Float:
//		// do something with obj.Value()
//	}
//
// The Type() method of each object may also be used to get a string
// name of the object type, such as "string" or "float".
package object

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/risor-io/risor/errz"
	"github.com/risor-io/risor/op"
)

// Type of an object as a string.
type Type string

// Type constants
const (
	BOOL          Type = "bool"
	BUFFER        Type = "buffer"
	BUILTIN       Type = "builtin"
	BYTE          Type = "byte"
	BYTE_SLICE    Type = "byte_slice"
	CELL          Type = "cell"
	COLOR         Type = "color"
	COMPLEX       Type = "complex"
	COMPLEX_SLICE Type = "complex_slice"
	DYNAMIC_ATTR  Type = "dynamic_attr"
	ERROR         Type = "error"
	FLOAT         Type = "float"
	FLOAT_SLICE   Type = "float_slice"
	FUNCTION      Type = "function"
	GO_FIELD      Type = "go_field"
	GO_METHOD     Type = "go_method"
	GO_TYPE       Type = "go_type"
	INT           Type = "int"
	INT_ITER      Type = "int_iter"
	ITER_ENTRY    Type = "iter_entry"
	LIST          Type = "list"
	LIST_ITER     Type = "list_iter"
	MAP           Type = "map"
	MAP_ITER      Type = "map_iter"
	MODULE        Type = "module"
	NIL           Type = "nil"
	PARTIAL       Type = "partial"
	PROXY         Type = "proxy"
	RESULT        Type = "result"
	SET           Type = "set"
	SET_ITER      Type = "set_iter"
	SLICE_ITER    Type = "slice_iter"
	STRING        Type = "string"
	STRING_ITER   Type = "string_iter"
	TIME          Type = "time"
)

var (
	Nil   = &NilType{}
	True  = &Bool{value: true}
	False = &Bool{value: false}
)

// Object is the interface that all object types in Risor must implement.
type Object interface {
	// Type of the object.
	Type() Type

	// Inspect returns a string representation of the given object.
	Inspect() string

	// Interface converts the given object to a native Go value.
	Interface() interface{}

	// Returns True if the given object is equal to this object.
	Equals(other Object) Object

	// GetAttr returns the attribute with the given name from this object.
	GetAttr(name string) (Object, bool)

	// SetAttr sets the attribute with the given name on this object.
	SetAttr(name string, value Object) error

	// IsTruthy returns true if the object is considered "truthy".
	IsTruthy() bool

	// RunOperation runs an operation on this object with the given
	// right-hand side object.
	RunOperation(opType op.BinaryOpType, right Object) Object

	// Cost returns the incremental processing cost of this object.
	Cost() int
}

// Slice is used to specify a range or slice of items in a container.
type Slice struct {
	Start Object
	Stop  Object
}

// IteratorEntry is a single item returned by an iterator.
type IteratorEntry interface {
	Object
	Key() Object
	Value() Object
	Primary() Object
}

// Iterator is an interface used to iterate over a container.
type Iterator interface {
	Object

	// Next advances the iterator and then returns the current object and a
	// bool indicating whether the returned item is valid. Once Next() has been
	// called, the Entry() method can be used to get an IteratorEntry.
	Next(context.Context) (Object, bool)

	// Entry returns the current entry in the iterator and a bool indicating
	// whether the returned item is valid.
	Entry() (IteratorEntry, bool)
}

// Iterable is an interface that exposes an iterator for an Object.
type Iterable interface {
	Iter() Iterator
}

type Container interface {
	Iterable

	// GetItem implements the [key] operator for a container type.
	GetItem(key Object) (Object, *Error)

	// GetSlice implements the [start:stop] operator for a container type.
	GetSlice(s Slice) (Object, *Error)

	// SetItem implements the [key] = value operator for a container type.
	SetItem(key, value Object) *Error

	// DelItem implements the del [key] operator for a container type.
	DelItem(key Object) *Error

	// Contains returns true if the given item is found in this container.
	Contains(item Object) *Bool

	// Len returns the number of items in this container.
	Len() *Int
}

// Callable is an interface that exposes a Call method.
type Callable interface {
	// Call invokes the callable with the given arguments and returns the result.
	Call(ctx context.Context, args ...Object) Object
}

// Hashable types can be hashed and consequently used in a set.
type Hashable interface {
	// Hash returns a hash key for the given object.
	HashKey() HashKey
}

// Comparable is an interface used to compare two objects.
//
//	-1 if this < other
//	 0 if this == other
//	 1 if this > other
type Comparable interface {
	Compare(other Object) (int, error)
}

func CompareTypes(a, b Object) int {
	aType := a.Type()
	bType := b.Type()
	if aType != bType {
		if aType < bType {
			return -1
		}
		return 1
	}
	return 0
}

// HashKey is used to identify unique values in a set.
type HashKey struct {
	// Type of the object being referenced.
	Type Type
	// FltValue is used as the key for floats.
	FltValue float64
	// IntValue is used as the key for integers.
	IntValue int64
	// StrValue is used as the key for strings.
	StrValue string
}

// AttrResolver is an interface used to resolve dynamic attributes on an object.
type AttrResolver interface {
	ResolveAttr(ctx context.Context, name string) (Object, error)
}

type ResolveAttrFunc func(ctx context.Context, name string) (Object, error)

// Keys returns the keys of an object map as a sorted slice of strings.
func Keys(m map[string]Object) []string {
	var names []string
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// PrintableValue returns a value that should be used when printing an object.
func PrintableValue(obj Object) interface{} {
	switch obj := obj.(type) {
	// Primitive types have their underlying Go value passed to fmt.Printf
	// so that Go's Printf-style formatting directives work as expected. Also,
	// with these types there's no good reason for the print format to differ.
	case *String,
		*Int,
		*Float,
		*Byte,
		*Error,
		*Bool:
		return obj.Interface()
	// For time objects, as a personal preference, I'm using RFC3339 format
	// rather than Go's default time print format, which I find less readable.
	case *Time:
		return obj.Value().Format(time.RFC3339)
	}
	// For everything else, convert the object to a string directly, relying
	// on the object type's String() or Inspect() methods. This gives the author
	// of new types the ability to customize the object print string. Note that
	// Risor map and list objects fall into this category on purpose and the
	// print format for these is intentionally a bit different than the print
	// format for the equivalent Go type (maps and slices).
	switch obj := obj.(type) {
	case fmt.Stringer:
		return obj.String()
	default:
		return obj.Inspect()
	}
}

// EvalErrorf returns a Risor Error object containing an eval error.
func EvalErrorf(format string, args ...interface{}) *Error {
	return NewError(errz.EvalErrorf(format, args...))
}

// ArgsErrorf returns a Risor Error object containing an arguments error.
func ArgsErrorf(format string, args ...interface{}) *Error {
	return NewError(errz.ArgsErrorf(format, args...))
}

// TypeErrorf returns a Risor Error object containing a type error.
func TypeErrorf(format string, args ...interface{}) *Error {
	return NewError(errz.TypeErrorf(format, args...))
}
