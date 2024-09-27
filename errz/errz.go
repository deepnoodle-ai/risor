// Package errz defines a FriendlyError interface for errors that have a human
// friendly message in addition to the default error message.
package errz

import "fmt"

var typeErrorsAreFatal = false

// FriendlyError is an interface for errors that have a human friendly message
// in addition to a the lower level default error message.
type FriendlyError interface {
	Error() string
	FriendlyErrorMessage() string
}

type Error interface {
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
