// Package object re-exports error types from the errors package for convenience.
package object

import (
	"github.com/risor-io/risor/errors"
)

// Re-export types from errors package for convenience
type (
	SourceLocation  = errors.SourceLocation
	StackFrame      = errors.StackFrame
	StructuredError = errors.StructuredError
	ErrorKind       = errors.ErrorKind
	FriendlyError   = errors.FriendlyError
	EvalError       = errors.EvalError
	ArgsError       = errors.ArgsError
	TypeError       = errors.TypeError
	ValueError      = errors.ValueError
	IndexError      = errors.IndexError
)

// Re-export error kind constants
const (
	ErrSyntax  = errors.ErrSyntax
	ErrType    = errors.ErrType
	ErrName    = errors.ErrName
	ErrValue   = errors.ErrValue
	ErrRuntime = errors.ErrRuntime
	ErrImport  = errors.ErrImport
)

// Re-export functions for convenience
var (
	FormatStackTrace    = errors.FormatStackTrace
	NewEvalError        = errors.NewEvalError
	NewArgsErrorType    = errors.NewArgsError
	NewTypeError        = errors.NewTypeError
	NewValueError       = errors.NewValueError
	NewIndexError       = errors.NewIndexError
	NewStructuredError  = errors.NewStructuredError
	NewStructuredErrorf = errors.NewStructuredErrorf
)

// Internal functions used by the wrapper functions in object.go
var (
	newEvalErrorf  = errors.EvalErrorf
	newArgsErrorf  = errors.ArgsErrorf
	newTypeErrorf  = errors.TypeErrorf
	newValueErrorf = errors.ValueErrorf
	newIndexErrorf = errors.IndexErrorf
)
