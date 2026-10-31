package object

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

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
		Doc("Get the error kind (e.g., 'type error', 'value error', 'runtime error')").
		Returns("string").
		Impl(func(e *Error, ctx context.Context, args ...Object) (Object, error) {
			return NewString(e.kind()), nil
		})
}

// kind returns the error kind that kind() reports. An error of no known
// kind, such as one made by error("msg"), is a "runtime error": that is the
// kind the VM gives it when it is thrown, so throwing an error doesn't
// change its kind or what it equals.
func (e *Error) kind() string {
	if e.structured != nil {
		return e.structured.Kind.String()
	}
	return ErrRuntime.String()
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

// errorfMaxTextSize bounds the text of each Object argument to Errorf. It is
// compared with formatSize, which counts the length of the text that Inspect
// prints, as CheckFormatSize does. Errorf prints a string without quotes, so
// its text is never longer than the count. It matches DefaultMaxListLength,
// the element budget that bounded these arguments when Errorf converted them
// with Interface.
const errorfMaxTextSize = DefaultMaxListLength

// Errorf returns an Error with a message formatted as fmt.Errorf does. An
// Object argument is formatted with PrintableValue: a string, int, float,
// byte, bool, or error formats as its Go value, so verbs such as %d and %q
// work. Any other Object formats as Risor prints it, so a list formats as
// [1, 2] and a map as {"a": 1}. Errorf does not convert an Object to Go, so
// a large or cyclic container does not fail or format as <nil>. An Object
// whose counted size exceeds errorfMaxTextSize formats as a placeholder, [...]
// or {...}, so a container that repeats shared references cannot make Errorf
// build an unbounded message.
func Errorf(format string, a ...interface{}) *Error {
	var args []interface{}
	for _, arg := range a {
		if obj, ok := arg.(Object); ok {
			if formatSize(obj, nil, errorfMaxTextSize) > errorfMaxTextSize {
				args = append(args, inspectPlaceholder(obj))
			} else {
				args = append(args, PrintableValue(obj))
			}
		} else {
			args = append(args, arg)
		}
	}
	return &Error{err: fmt.Errorf(format, args...)}
}

// InspectMessage returns the text of obj for an error message, such as the
// message of a thrown value that is not a string. It returns the text that
// obj.Inspect returns, or a placeholder, [...] or {...}, if the counted size
// of the text exceeds errorfMaxTextSize, the bound that Errorf applies to its
// arguments. That bound is smaller than the bound of Inspect, so a value that
// repeats shared references cannot make a large message.
func InspectMessage(obj Object) string {
	if formatSize(obj, nil, errorfMaxTextSize) > errorfMaxTextSize {
		return inspectPlaceholder(obj)
	}
	return inspectNested(obj, nil)
}

// quoteMessageMaxSize is the number of bytes of a string that QuoteMessage
// quotes. Errorf bounds an Object argument by errorfMaxTextSize, which is
// large enough to print a whole value. A message that names an input, such
// as a string that did not parse, needs only enough of it to identify it.
const quoteMessageMaxSize = 64

// QuoteMessage returns s quoted as %q quotes it, for an error message that
// names an input value, such as a string that did not parse as an int. If s
// is longer than quoteMessageMaxSize bytes, it quotes only a prefix of s and
// appends "..." after the closing quote. The prefix ends on a UTF-8 boundary
// when s is valid UTF-8 there. %q can print a byte as four characters, so
// quoting the whole of a large input would make a message about four times
// the size of the input.
func QuoteMessage(s string) string {
	if len(s) <= quoteMessageMaxSize {
		return strconv.Quote(s)
	}
	n := quoteMessageMaxSize
	// Back up to the start of the rune that the cut splits. If no rune
	// starts in the last utf8.UTFMax-1 bytes, s is not valid UTF-8 there and
	// %q escapes those bytes one at a time, so cut at the limit.
	for i := n; i > n-utf8.UTFMax; i-- {
		if utf8.RuneStart(s[i]) {
			n = i
			break
		}
	}
	return strconv.Quote(s[:n]) + "..."
}

// boundedError replaces the message of err, which can quote a whole input,
// with a message that quotes the input with QuoteMessage. It unwraps to err,
// so errors.Is and errors.As still find err.
type boundedError struct {
	msg string
	err error
}

func (e *boundedError) Error() string { return e.msg }

func (e *boundedError) Unwrap() error { return e.err }

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
