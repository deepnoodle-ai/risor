package object

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/op"
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
			if e.structured != nil {
				return NewString(e.structured.Kind.String()), nil
			}
			return NewString("error"), nil
		})
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

func (e *Error) Inspect() string {
	return fmt.Sprintf("error(%q)", e.err.Error())
}

func (e *Error) String() string {
	return e.err.Error()
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
	return e.Message().Value() == otherError.Message().Value()
}

func (e *Error) Message() *String {
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

func NewError(err error) *Error {
	switch err := err.(type) {
	case *Error: // unwrap to get the inner error, to avoid unhelpful nesting
		return &Error{err: err.Unwrap(), structured: err.structured}
	case *StructuredError:
		return &Error{err: err, structured: err}
	case *TypeError:
		return &Error{err: err, structured: NewStructuredError(ErrType, err.Error(), SourceLocation{}, nil)}
	case *ValueError:
		return &Error{err: err, structured: NewStructuredError(ErrValue, err.Error(), SourceLocation{}, nil)}
	case *IndexError:
		return &Error{err: err, structured: NewStructuredError(ErrValue, err.Error(), SourceLocation{}, nil)}
	default:
		return &Error{err: err}
	}
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
