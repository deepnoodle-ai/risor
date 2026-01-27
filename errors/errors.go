// Package errors defines error types with source locations and stack traces.
package errors

import (
	"fmt"
	"strings"
)

// SourceLocation represents a position in source code.
type SourceLocation struct {
	Filename string
	Line     int    // 1-based line number
	Column   int    // 1-based column number
	Source   string // The line of source code
}

// String returns a formatted string representation of the source location.
func (s SourceLocation) String() string {
	if s.Filename != "" {
		return fmt.Sprintf("%s:%d:%d", s.Filename, s.Line, s.Column)
	}
	return fmt.Sprintf("%d:%d", s.Line, s.Column)
}

// IsZero returns true if the location has not been set.
func (s SourceLocation) IsZero() bool {
	return s.Line == 0 && s.Column == 0
}

// StackFrame represents a single frame in the call stack.
type StackFrame struct {
	Function string
	Location SourceLocation
}

// String returns a formatted string representation of the stack frame.
func (f StackFrame) String() string {
	if f.Function != "" {
		return fmt.Sprintf("at %s (%s)", f.Function, f.Location.String())
	}
	return fmt.Sprintf("at %s", f.Location.String())
}

// FormatStackTrace formats a slice of stack frames as a human-readable string.
func FormatStackTrace(frames []StackFrame) string {
	if len(frames) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Stack trace:\n")
	for _, frame := range frames {
		b.WriteString("  ")
		b.WriteString(frame.String())
		b.WriteString("\n")
	}
	return b.String()
}

var typeErrorsAreFatal = false

// FriendlyError is an interface for errors that have a human friendly message
// in addition to a the lower level default error message.
type FriendlyError interface {
	Error() string
	FriendlyErrorMessage() string
}

// FormattableError is an interface for errors that can be formatted with
// the enhanced error formatter (with colors, source context, etc).
type FormattableError interface {
	Error() string
	ToFormatted() *FormattedError
}

// FatalError is an interface for errors that may or may not be fatal.
type FatalError interface {
	Error() string
	IsFatal() bool
}

// EvalError is used to indicate an unrecoverable error that occurred
// during program evaluation. All EvalErrors are considered fatal errors.
type EvalError struct {
	Err error
}

func (r *EvalError) Error() string {
	return r.Err.Error()
}

func (r *EvalError) Unwrap() error {
	return r.Err
}

func (r *EvalError) IsFatal() bool {
	return true
}

func NewEvalError(err error) *EvalError {
	return &EvalError{Err: err}
}

func EvalErrorf(format string, args ...any) *EvalError {
	return NewEvalError(fmt.Errorf(format, args...))
}

// ArgsError is used to indicate an error that occurred while processing
// function arguments. All ArgsErrors are considered fatal errors. This should
// be reserved for use in cases where a function call basically should not
// compile due to the number of arguments passed.
type ArgsError struct {
	Err error
}

func (a *ArgsError) Error() string {
	return a.Err.Error()
}

func (a *ArgsError) Unwrap() error {
	return a.Err
}

func (a *ArgsError) IsFatal() bool {
	return true
}

func NewArgsError(err error) *ArgsError {
	return &ArgsError{Err: err}
}

func ArgsErrorf(format string, args ...any) *ArgsError {
	return NewArgsError(fmt.Errorf(format, args...))
}

// TypeError is used to indicate an invalid type was supplied. These may or may
// not be fatal errors depending on typeErrorsAreFatal setting.
type TypeError struct {
	Err     error
	isFatal bool
}

func (t *TypeError) Error() string {
	return t.Err.Error()
}

func (t *TypeError) Unwrap() error {
	return t.Err
}

func (t *TypeError) IsFatal() bool {
	return t.isFatal
}

func NewTypeError(err error) *TypeError {
	return &TypeError{Err: err, isFatal: typeErrorsAreFatal}
}

func TypeErrorf(format string, args ...any) *TypeError {
	return NewTypeError(fmt.Errorf(format, args...))
}

// AreTypeErrorsFatal returns whether type errors are considered fatal.
func AreTypeErrorsFatal() bool {
	return typeErrorsAreFatal
}

// SetTypeErrorsAreFatal sets whether type errors should be considered fatal.
func SetTypeErrorsAreFatal(fatal bool) {
	typeErrorsAreFatal = fatal
}
