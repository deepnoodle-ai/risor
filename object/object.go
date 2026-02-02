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

	"github.com/deepnoodle-ai/risor/v2/op"
)

// Type of an object as a string.
type Type string

// Type constants
const (
	BOOL          Type = "bool"
	BUILTIN       Type = "builtin"
	BYTE          Type = "byte"
	BYTES         Type = "bytes"
	CELL          Type = "cell"
	COLOR         Type = "color"
	COMPLEX       Type = "complex"
	COMPLEX_SLICE Type = "complex_slice"
	DYNAMIC_ATTR  Type = "dynamic_attr"
	ERROR         Type = "error"
	FLOAT         Type = "float"
	FUNCTION      Type = "function"
	INT           Type = "int"
	LIST          Type = "list"
	MAP           Type = "map"
	MODULE        Type = "module"
	NIL           Type = "nil"
	PARTIAL       Type = "partial"
	RANGE         Type = "range"
	RESULT        Type = "result"
	STRING        Type = "string"
	TIME          Type = "time"
	GOFUNC        Type = "go_func"
	GOSTRUCT      Type = "go_struct"
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

	// Returns true if the given object is equal to this object.
	Equals(other Object) bool

	// Attrs returns the attribute specifications for this object type.
	// Used for introspection, documentation, and tooling (autocomplete, etc.).
	// Returns nil for types with no attributes.
	Attrs() []AttrSpec

	// GetAttr returns the attribute with the given name from this object.
	GetAttr(name string) (Object, bool)

	// SetAttr sets the attribute with the given name on this object.
	SetAttr(name string, value Object) error

	// IsTruthy returns true if the object is considered "truthy".
	IsTruthy() bool

	// RunOperation runs an operation on this object with the given
	// right-hand side object.
	RunOperation(opType op.BinaryOpType, right Object) (Object, error)
}

// Slice is used to specify a range or slice of items in a container.
type Slice struct {
	Start Object
	Stop  Object
}

// Enumerable is an interface for types that can be iterated with a callback.
// The callback receives the key and value for each element. Return false to stop.
type Enumerable interface {
	Enumerate(ctx context.Context, fn func(key, value Object) bool)
}

type Container interface {
	Enumerable

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

// Callable is an interface for objects that can be invoked as functions.
// Both *Builtin and *Closure implement this interface, allowing code to
// call functions without knowing their concrete type.
//
// For closures, Call() uses the CallFunc stored in the context (set by the VM)
// to execute the closure's bytecode. For builtins, Call() invokes the wrapped
// Go function directly.
//
// List methods like Map, Filter, Each, and Reduce accept any Callable,
// enabling both builtins and closures to be used as callbacks.
type Callable interface {
	// Call invokes the callable with the given arguments and returns the result.
	Call(ctx context.Context, args ...Object) (Object, error)
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
	return NewError(newEvalErrorf(format, args...))
}

// ArgsErrorf returns a Risor Error object containing an arguments error.
func ArgsErrorf(format string, args ...interface{}) *Error {
	return NewError(newArgsErrorf(format, args...))
}

// TypeErrorf returns a Risor Error object containing a type error.
func TypeErrorf(format string, args ...interface{}) *Error {
	return NewError(newTypeErrorf(format, args...))
}

// ValueErrorf returns a Risor Error object containing a value error.
func ValueErrorf(format string, args ...interface{}) *Error {
	return NewError(newValueErrorf(format, args...))
}

// IndexErrorf returns a Risor Error object containing an index error.
func IndexErrorf(format string, args ...interface{}) *Error {
	return NewError(newIndexErrorf(format, args...))
}
