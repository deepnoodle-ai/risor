package errz

import (
	"bytes"
	"fmt"
	"strings"
)

// ErrorKind represents the category of an error.
type ErrorKind int

const (
	// ErrSyntax indicates a syntax/parsing error.
	ErrSyntax ErrorKind = iota
	// ErrType indicates a type mismatch or invalid operation on a type.
	ErrType
	// ErrName indicates an undefined variable or function.
	ErrName
	// ErrValue indicates an invalid value for an operation.
	ErrValue
	// ErrRuntime indicates a general runtime error.
	ErrRuntime
	// ErrImport indicates an error importing a module.
	ErrImport
)

// String returns the string representation of the error kind.
func (k ErrorKind) String() string {
	switch k {
	case ErrSyntax:
		return "syntax error"
	case ErrType:
		return "type error"
	case ErrName:
		return "name error"
	case ErrValue:
		return "value error"
	case ErrRuntime:
		return "runtime error"
	case ErrImport:
		return "import error"
	default:
		return "error"
	}
}

// StructuredError is a rich error type with source locations, visual snippets,
// and stack traces for actionable diagnostics.
type StructuredError struct {
	Message  string
	Kind     ErrorKind
	Location SourceLocation
	Stack    []StackFrame
	Cause    error
}

// Error implements the error interface.
func (e *StructuredError) Error() string {
	if e.Location.IsZero() {
		return fmt.Sprintf("%s: %s", e.Kind.String(), e.Message)
	}
	return fmt.Sprintf("%s: %s (%d:%d)", e.Kind.String(), e.Message, e.Location.Line, e.Location.Column)
}

// Unwrap returns the underlying cause of the error.
func (e *StructuredError) Unwrap() error {
	return e.Cause
}

// IsFatal returns whether the error is considered fatal (unrecoverable).
// Type errors respect the typeErrorsAreFatal setting for backward compatibility.
func (e *StructuredError) IsFatal() bool {
	switch e.Kind {
	case ErrType:
		return typeErrorsAreFatal
	default:
		return true
	}
}

// FriendlyErrorMessage returns a human-friendly error message with visual
// context including source snippets and stack traces.
func (e *StructuredError) FriendlyErrorMessage() string {
	var msg bytes.Buffer

	// Error header with location
	if e.Location.IsZero() {
		msg.WriteString(fmt.Sprintf("%s: %s\n", e.Kind.String(), e.Message))
	} else {
		msg.WriteString(fmt.Sprintf("%s: %s (%d:%d)\n", e.Kind.String(), e.Message, e.Location.Line, e.Location.Column))
	}

	// Source snippet with caret
	if e.Location.Source != "" {
		msg.WriteString(" | ")
		msg.WriteString(e.Location.Source)
		msg.WriteString("\n")
		if e.Location.Column > 0 {
			msg.WriteString(" | ")
			msg.WriteString(strings.Repeat(" ", e.Location.Column-1))
			msg.WriteString("^\n")
		}
	}

	// Stack trace
	if len(e.Stack) > 0 {
		msg.WriteString("\n")
		msg.WriteString(FormatStackTrace(e.Stack))
	}

	return msg.String()
}

// NewStructuredError creates a new StructuredError with the given parameters.
func NewStructuredError(kind ErrorKind, message string, loc SourceLocation, stack []StackFrame) *StructuredError {
	return &StructuredError{
		Message:  message,
		Kind:     kind,
		Location: loc,
		Stack:    stack,
	}
}

// NewStructuredErrorf creates a new StructuredError with a formatted message.
func NewStructuredErrorf(kind ErrorKind, loc SourceLocation, stack []StackFrame, format string, args ...any) *StructuredError {
	return &StructuredError{
		Message:  fmt.Sprintf(format, args...),
		Kind:     kind,
		Location: loc,
		Stack:    stack,
	}
}

// WithCause wraps the error with a cause.
func (e *StructuredError) WithCause(cause error) *StructuredError {
	e.Cause = cause
	return e
}

// GetStack returns the stack frames of the error.
func (e *StructuredError) GetStack() []StackFrame {
	return e.Stack
}

// GetLocation returns the source location of the error.
func (e *StructuredError) GetLocation() SourceLocation {
	return e.Location
}
