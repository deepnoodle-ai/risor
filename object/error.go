package object

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/op"
)

// Error wraps a Go error interface and implements Object.
type Error struct {
	*base
	err        error
	raised     bool
	structured *StructuredError
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
		return 0, TypeErrorf("type error: unable to compare error and %s", other.Type())
	}
	thisMsg := e.Message().Value()
	otherMsg := otherErr.Message().Value()
	if thisMsg == otherMsg && e.raised == otherErr.raised {
		return 0, nil
	}
	if thisMsg > otherMsg {
		return 1, nil
	}
	if thisMsg < otherMsg {
		return -1, nil
	}
	if e.raised && !otherErr.raised {
		return 1, nil
	}
	if !e.raised && otherErr.raised {
		return -1, nil
	}
	return 0, nil
}

func (e *Error) Equals(other Object) bool {
	otherError, ok := other.(*Error)
	if !ok {
		return false
	}
	return e.Message().Value() == otherError.Message().Value() && e.raised == otherError.raised
}

func (e *Error) GetAttr(name string) (Object, bool) {
	switch name {
	case "error":
		return NewBuiltin("error", func(ctx context.Context, args ...Object) (Object, error) {
			return e.Message(), nil
		}), true
	case "message":
		return NewBuiltin("message", func(ctx context.Context, args ...Object) (Object, error) {
			return e.Message(), nil
		}), true
	case "line":
		return NewBuiltin("line", func(ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil {
				return NewInt(int64(e.structured.Location.Line)), nil
			}
			return NewInt(0), nil
		}), true
	case "column":
		return NewBuiltin("column", func(ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil {
				return NewInt(int64(e.structured.Location.Column)), nil
			}
			return NewInt(0), nil
		}), true
	case "filename":
		return NewBuiltin("filename", func(ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil && e.structured.Location.Filename != "" {
				return NewString(e.structured.Location.Filename), nil
			}
			return Nil, nil
		}), true
	case "source":
		return NewBuiltin("source", func(ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil && e.structured.Location.Source != "" {
				return NewString(e.structured.Location.Source), nil
			}
			return Nil, nil
		}), true
	case "stack":
		return NewBuiltin("stack", func(ctx context.Context, args ...Object) (Object, error) {
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
		}), true
	case "kind":
		return NewBuiltin("kind", func(ctx context.Context, args ...Object) (Object, error) {
			if e.structured != nil {
				return NewString(e.structured.Kind.String()), nil
			}
			return NewString("error"), nil
		}), true
	default:
		return nil, false
	}
}

func (e *Error) Message() *String {
	return NewString(e.err.Error())
}

func (e *Error) WithRaised(value bool) *Error {
	e.raised = value
	return e
}

func (e *Error) IsRaised() bool {
	return e.raised
}

func (e *Error) Error() string {
	return e.err.Error()
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) RunOperation(opType op.BinaryOpType, right Object) (Object, error) {
	return nil, fmt.Errorf("type error: unsupported operation for error: %v", opType)
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
	return &Error{err: fmt.Errorf(format, args...), raised: true}
}

func (e *Error) MarshalJSON() ([]byte, error) {
	return nil, TypeErrorf("type error: unable to marshal error")
}

func NewError(err error) *Error {
	switch err := err.(type) {
	case *Error: // unwrap to get the inner error, to avoid unhelpful nesting
		return &Error{err: err.Unwrap(), raised: true, structured: err.structured}
	case *StructuredError:
		return &Error{err: err, raised: true, structured: err}
	default:
		return &Error{err: err, raised: true}
	}
}

// NewErrorFromStructured creates a new Error from a StructuredError.
func NewErrorFromStructured(se *StructuredError) *Error {
	return &Error{err: se, raised: true, structured: se}
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
