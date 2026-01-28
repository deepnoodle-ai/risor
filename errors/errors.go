// Package errors defines compile-time error types for the Risor language.
//
// These errors are returned by the lexer, parser, and compiler phases.
// They implement Go's error interface and include source location information
// for diagnostic messages.
//
// # Error Boundary
//
// Risor has two error systems with a clear boundary:
//
//   - Compile-time: This package. Returns Go errors with source locations.
//     Used by: lexer, parser, compiler. Returned before code executes.
//
//   - Runtime: object.Error. Risor error values visible to scripts.
//     Used by: VM, builtins, scripts. Errors are values; throw triggers exceptions.
//
// The boundary is execution: functions like risor.Eval() and compiler.Compile()
// return Go errors (from this package). Once the VM is running, errors become
// object.Error values that scripts can catch with try/catch.
package errors

import (
	"fmt"
	"strings"
)

// SourceLocation represents a position in source code.
type SourceLocation struct {
	Filename  string
	Line      int    // 1-based line number
	Column    int    // 1-based column number
	EndColumn int    // 1-based end column (0 if not set, for multi-char underlines)
	Source    string // The line of source code
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

// EvalError is used to indicate an unrecoverable error that occurred
// during program evaluation.
type EvalError struct {
	Err error
}

func (r *EvalError) Error() string {
	return r.Err.Error()
}

func (r *EvalError) Unwrap() error {
	return r.Err
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

func NewArgsError(err error) *ArgsError {
	return &ArgsError{Err: err}
}

func ArgsErrorf(format string, args ...any) *ArgsError {
	return NewArgsError(fmt.Errorf(format, args...))
}

// TypeError is used to indicate an invalid type was supplied.
type TypeError struct {
	Err error
}

func (t *TypeError) Error() string {
	return t.Err.Error()
}

func (t *TypeError) Unwrap() error {
	return t.Err
}

func NewTypeError(err error) *TypeError {
	return &TypeError{Err: err}
}

func TypeErrorf(format string, args ...any) *TypeError {
	return NewTypeError(fmt.Errorf("type error: "+format, args...))
}

// ValueError is used to indicate an invalid value for an operation.
// Examples: division by zero, invalid argument values.
type ValueError struct {
	Err error
}

func (v *ValueError) Error() string {
	return v.Err.Error()
}

func (v *ValueError) Unwrap() error {
	return v.Err
}

func NewValueError(err error) *ValueError {
	return &ValueError{Err: err}
}

func ValueErrorf(format string, args ...any) *ValueError {
	return NewValueError(fmt.Errorf("value error: "+format, args...))
}

// IndexError is used to indicate an index is out of bounds.
type IndexError struct {
	Err error
}

func (i *IndexError) Error() string {
	return i.Err.Error()
}

func (i *IndexError) Unwrap() error {
	return i.Err
}

func NewIndexError(err error) *IndexError {
	return &IndexError{Err: err}
}

func IndexErrorf(format string, args ...any) *IndexError {
	return NewIndexError(fmt.Errorf("index error: "+format, args...))
}
