package object

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/pkg/op"
)

var errorMethods = NewMethodRegistry[*Error]("error")

func init() {
	errorMethods.Define("error").
		Doc("Get the error message (alias for message)").
		Returns("string").
		Impl(func(e *Error, ctx context.Context, args ...Object) (Object, error) {
			return e.Message(), nil
		})

	errorMethods.Define("message").
		Doc("Get the error message").
		Returns("string").
		Impl(func(e *Error, ctx context.Context, args ...Object) (Object, error) {
			return e.Message(), nil
		})

	errorMethods.Define("line").
		Doc("Get the line number where the error occurred").
		Returns("int").
		Impl(func(e *Error, ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil {
				return NewInt(int64(e.structured.Location.Line)), nil
			}
			return NewInt(0), nil
		})

	errorMethods.Define("column").
		Doc("Get the column number where the error occurred").
		Returns("int").
		Impl(func(e *Error, ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil {
				return NewInt(int64(e.structured.Location.Column)), nil
			}
			return NewInt(0), nil
		})

	errorMethods.Define("filename").
		Doc("Get the filename where the error occurred").
		Returns("string").
		Impl(func(e *Error, ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil && e.structured.Location.Filename != "" {
				return NewString(e.structured.Location.Filename), nil
			}
			return Nil, nil
		})

	errorMethods.Define("source").
		Doc("Get the source code context of the error").
		Returns("string").
		Impl(func(e *Error, ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil && e.structured.Location.Source != "" {
				return NewString(e.structured.Location.Source), nil
			}
			return Nil, nil
		})

	errorMethods.Define("stack").
		Doc("Get the stack trace as a list of frames").
		Returns("list").
		Impl(func(e *Error, ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil && len(e.structured.Stack) > 0 {
				frames := make([]Object, len(e.structured.Stack))
				for i, frame := range e.structured.Stack {
					frames[i] = NewMap(map[string]Object{
						"function": NewString(frame.Function),
						"line":     NewInt(int64(frame.Location.Line)),
						"column":   NewInt(int64(frame.Location.Column)),
						"filename": NewString(frame.Location.Filename),
					})
				}
				return NewList(frames), nil
			}
			return NewList(nil), nil
		})

	errorMethods.Define("kind").
		Doc("Get the error kind (e.g., 'type', 'value', 'error')").
		Returns("string").
		Impl(func(e *Error, ctx context.Context, args ...Object) (Object, error) {
			return NewString(e.kind()), nil
		})
}

// kind returns the error kind that kind() reports. An error of no known kind
// is "error".
func (e *Error) kind() string {
	if e.structured != nil {
		return e.structured.Kind.String()
	}
	return "error"
}

// Error wraps a Go error interface and implements Object.
//
// Errors are values. You can create them, inspect them, store them, and pass
// them around like any other value. Use "throw" to trigger exception handling.
//
// Example:
//
//	let err = error("something went wrong")
//	print(err.message())     // inspect
//	print(`Error: ${err}`)   // stringify
//	throw err                // only throw triggers exception handling
type Error struct {
	err        error
	structured *StructuredError
}

func (e *Error) Attrs() []AttrSpec {
	return errorMethods.Specs()
}

func (e *Error) GetAttr(name string) (Object, bool) {
	return errorMethods.GetAttr(e, name)
}

func (e *Error) SetAttr(name string, value Object) error {
	return TypeErrorf("error has no attribute %q", name)
}

func (e *Error) IsTruthy() bool {
	return true
}

func (e *Error) Type() Type {
	return ERROR
}

// Inspect and String show the message only, like message(). Error returns
// the full Go error string, with the kind and location.
func (e *Error) Inspect() string {
	return fmt.Sprintf("error(%q)", e.Message().Value())
}

func (e *Error) String() string {
	return e.Message().Value()
}

func (e *Error) Value() error {
	return e.err
}

func (e *Error) Interface() interface{} {
	return e.err
}

func (e *Error) Compare(other Object) (int, error) {
	otherErr, ok := other.(*Error)
	if !ok {
		return 0, TypeErrorf("unable to compare error and %s", other.Type())
	}
	thisMsg := e.Message().Value()
	otherMsg := otherErr.Message().Value()
	if thisMsg == otherMsg {
		return 0, nil
	}
	if thisMsg > otherMsg {
		return 1, nil
	}
	return -1, nil
}

func (e *Error) Equals(other Object) bool {
	otherError, ok := other.(*Error)
	if !ok {
		return false
	}
	// Match wrapped sentinels in either direction, then fall back to message
	// and kind equality for script-created errors. The message leaves out the
	// kind, so the kind must match too. The chain-walking is an
	// implementation detail and may change; see docs/direction/v3-notes.md.
	if errors.Is(e.err, otherError.err) || errors.Is(otherError.err, e.err) {
		return true
	}
	return e.kind() == otherError.kind() &&
		e.Message().Value() == otherError.Message().Value()
}

// Message returns the error message. For an error of a known kind, the
// message leaves out the kind and the source location, which kind() and
// line() and column() return.
func (e *Error) Message() *String {
	if e.structured != nil {
		return NewString(e.structured.Message)
	}
	return NewString(e.err.Error())
}

func (e *Error) Error() string {
	return e.err.Error()
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, newTypeErrorf("unsupported operation for error: %v", opType)
}

func Errorf(format string, a ...interface{}) *Error {
	var args []interface{}
	for _, arg := range a {
		if obj, ok := arg.(Object); ok {
			args = append(args, obj.Interface())
		} else {
			args = append(args, arg)
		}
	}
	return &Error{err: fmt.Errorf(format, args...)}
}

func (e *Error) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("unable to marshal error")
}

// NewError creates an Error from a Go error. It finds the error kind with
// errors.As, so a TypeError, ValueError, IndexError, or StructuredError keeps
// its kind when another error wraps it.
func NewError(err error) *Error {
	if e, ok := err.(*Error); ok { // unwrap to get the inner error, to avoid unhelpful nesting
		return &Error{err: e.Unwrap(), structured: e.structured}
	}
	var se *StructuredError
	if errors.As(err, &se) {
		if se == err {
			return &Error{err: err, structured: se}
		}
		structured := NewStructuredError(se.Kind, kindMessage(err, se, se.Message), se.Location, se.Stack)
		structured.Cause = err
		return &Error{err: err, structured: structured}
	}
	var typeErr *TypeError
	if errors.As(err, &typeErr) {
		return newKindError(err, ErrType, kindMessage(err, typeErr, typeErr.Err.Error()))
	}
	var valueErr *ValueError
	if errors.As(err, &valueErr) {
		return newKindError(err, ErrValue, kindMessage(err, valueErr, valueErr.Err.Error()))
	}
	var indexErr *IndexError
	if errors.As(err, &indexErr) {
		// Index errors are a kind of value error
		return newKindError(err, ErrValue, kindMessage(err, indexErr, indexErr.Err.Error()))
	}
	return &Error{err: err}
}

// kindMessage returns the message of err without the kind prefix and location
// of inner, which err is or wraps. When err wraps inner, the context that err
// adds stays: "argument 1: value error: x" becomes "argument 1: x".
func kindMessage(err, inner error, msg string) string {
	if err == inner {
		return msg
	}
	return strings.Replace(err.Error(), inner.Error(), msg, 1)
}

// newKindError creates an Error for an error of a known kind. The structured
// message leaves out the kind prefix, because StructuredError adds the kind.
func newKindError(err error, kind ErrorKind, msg string) *Error {
	structured := NewStructuredError(kind, msg, SourceLocation{}, nil)
	structured.Cause = err
	return &Error{err: err, structured: structured}
}

// NewErrorFromStructured creates a new Error from a StructuredError.
func NewErrorFromStructured(se *StructuredError) *Error {
	return &Error{err: se, structured: se}
}

// Structured returns the underlying StructuredError if present.
func (e *Error) Structured() *StructuredError {
	return e.structured
}

// FriendlyErrorMessage returns a human-friendly error message if the error
// has structured data, otherwise returns the standard error string.
func (e *Error) FriendlyErrorMessage() string {
	if e.structured != nil {
		return e.structured.FriendlyErrorMessage()
	}
	return e.err.Error()
}

func IsError(obj Object) bool {
	if obj != nil {
		return obj.Type() == ERROR
	}
	return false
}
