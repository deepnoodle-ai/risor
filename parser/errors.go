package parser

import (
	"fmt"

	"github.com/risor-io/risor/errors"
	"github.com/risor-io/risor/internal/token"
)

// ErrorOpts is a struct that holds a variety of error data.
// All fields are optional, although one of `Cause` or `Message`
// are recommended. If `Cause` is set, `Message` will be ignored.
type ErrorOpts struct {
	ErrType       string
	Message       string
	Cause         error
	File          string
	StartPosition token.Position
	EndPosition   token.Position
	SourceCode    string
}

// NewBaseParserError returns a new BaseParserError populated with
// the given error data.
func NewParserError(opts ErrorOpts) *BaseParserError {
	return &BaseParserError{
		errType:       opts.ErrType,
		message:       opts.Message,
		cause:         opts.Cause,
		file:          opts.File,
		startPosition: opts.StartPosition,
		endPosition:   opts.EndPosition,
		sourceCode:    opts.SourceCode,
	}
}

// ParserError is an interface that all parser errors implement.
type ParserError interface {
	Type() string
	Message() string
	Cause() error
	File() string
	StartPosition() token.Position
	EndPosition() token.Position
	SourceCode() string
	Error() string
	errors.FriendlyError
}

// BaseParserError is the simplest implementation of ParserError.
type BaseParserError struct {
	// Type of the error, e.g. "syntax error"
	errType string
	// The error message
	message string
	// The wrapped error
	cause error
	// File where the error occurred
	file string
	// Start position of the error in the input string
	startPosition token.Position
	// End position of the error in the input string
	endPosition token.Position
	// Relevant line of source code text
	sourceCode string
}

func (e *BaseParserError) Error() string {
	var msg string
	if e.cause != nil {
		msg = e.cause.Error()
	} else if e.message != "" {
		msg = e.message
	}
	if e.errType != "" {
		msg = fmt.Sprintf("%s: %s", e.errType, msg)
	}
	return msg
}

func (e *BaseParserError) FriendlyErrorMessage() string {
	formatter := errors.NewFormatter(false)
	return formatter.Format(e.ToFormatted())
}

// ToFormatted converts the parser error to a FormattedError for display.
func (e *BaseParserError) ToFormatted() *errors.FormattedError {
	start := e.StartPosition()
	end := e.EndPosition()

	message := e.message
	if e.cause != nil {
		message = e.cause.Error()
	}

	return &errors.FormattedError{
		Kind:      e.errType,
		Message:   message,
		Filename:  e.file,
		Line:      start.LineNumber(),
		Column:    start.ColumnNumber(),
		EndColumn: end.ColumnNumber(),
		SourceLines: []errors.SourceLineEntry{
			{Number: start.LineNumber(), Text: e.sourceCode, IsMain: true},
		},
	}
}

func (e *BaseParserError) Cause() error {
	return e.cause
}

func (e *BaseParserError) Message() string {
	return e.message
}

func (e *BaseParserError) Line() int {
	return e.startPosition.Line
}

func (e *BaseParserError) StartPosition() token.Position {
	return e.startPosition
}

func (e *BaseParserError) EndPosition() token.Position {
	return e.endPosition
}

func (e *BaseParserError) File() string {
	return e.file
}

func (e *BaseParserError) SourceCode() string {
	return e.sourceCode
}

func (e *BaseParserError) Unwrap() error {
	return e.cause
}

func (e *BaseParserError) Type() string {
	return e.errType
}

// NewSyntaxError returns a new SyntaxError populated with the given error data
func NewSyntaxError(opts ErrorOpts) *SyntaxError {
	opts.ErrType = "syntax error"
	return &SyntaxError{BaseParserError: NewParserError(opts)}
}

type SyntaxError struct {
	*BaseParserError
}

func tokenTypeDescription(t token.Type) string {
	switch t {
	case token.EOF:
		return "end of file"
	case token.IDENT:
		return "identifier"
	case token.NEWLINE:
		return "newline"
	default:
		return string(t)
	}
}

func tokenDescription(t token.Token) string {
	switch t.Type {
	case token.EOF:
		return "end of file"
	case token.NEWLINE:
		return "newline"
	default:
		if t.Literal == "" {
			return string(t.Type)
		}
		return t.Literal
	}
}

// Errors wraps multiple parser errors for multi-error reporting.
// It implements the error interface so it can be returned from Parse().
type Errors struct {
	errs []ParserError
}

// NewErrors creates an Errors from a slice of ParserError.
func NewErrors(errs []ParserError) *Errors {
	if len(errs) == 0 {
		return nil
	}
	return &Errors{errs: errs}
}

// Error implements the error interface. Returns the first error message.
func (e *Errors) Error() string {
	if len(e.errs) == 0 {
		return ""
	}
	if len(e.errs) == 1 {
		return e.errs[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", e.errs[0].Error(), len(e.errs)-1)
}

// Errors returns the underlying slice of parser errors.
func (e *Errors) Errors() []ParserError {
	return e.errs
}

// Count returns the number of errors.
func (e *Errors) Count() int {
	return len(e.errs)
}

// First returns the first error, or nil if empty.
func (e *Errors) First() ParserError {
	if len(e.errs) == 0 {
		return nil
	}
	return e.errs[0]
}

// FriendlyErrorMessage returns a formatted message showing all errors.
func (e *Errors) FriendlyErrorMessage() string {
	formatter := errors.NewFormatter(false)
	return formatter.FormatMultiple(e.ToFormattedMultiple())
}

// ToFormattedMultiple converts all errors to FormattedError for display.
func (e *Errors) ToFormattedMultiple() []*errors.FormattedError {
	if len(e.errs) == 0 {
		return nil
	}

	var formatted []*errors.FormattedError
	for _, err := range e.errs {
		if formattable, ok := err.(interface{ ToFormatted() *errors.FormattedError }); ok {
			formatted = append(formatted, formattable.ToFormatted())
		} else {
			// Fallback for errors that don't implement ToFormatted
			formatted = append(formatted, &errors.FormattedError{
				Kind:    "error",
				Message: err.Error(),
			})
		}
	}
	return formatted
}

// The following methods implement ParserError interface by delegating to first error.
// This provides backwards compatibility for code that type-asserts to ParserError.

// Type returns the error type of the first error.
func (e *Errors) Type() string {
	if len(e.errs) == 0 {
		return ""
	}
	return e.errs[0].Type()
}

// Message returns the message of the first error.
func (e *Errors) Message() string {
	if len(e.errs) == 0 {
		return ""
	}
	return e.errs[0].Message()
}

// Cause returns the cause of the first error.
func (e *Errors) Cause() error {
	if len(e.errs) == 0 {
		return nil
	}
	return e.errs[0].Cause()
}

// File returns the file of the first error.
func (e *Errors) File() string {
	if len(e.errs) == 0 {
		return ""
	}
	return e.errs[0].File()
}

// StartPosition returns the start position of the first error.
func (e *Errors) StartPosition() token.Position {
	if len(e.errs) == 0 {
		return token.Position{}
	}
	return e.errs[0].StartPosition()
}

// EndPosition returns the end position of the first error.
func (e *Errors) EndPosition() token.Position {
	if len(e.errs) == 0 {
		return token.Position{}
	}
	return e.errs[0].EndPosition()
}

// SourceCode returns the source code of the first error.
func (e *Errors) SourceCode() string {
	if len(e.errs) == 0 {
		return ""
	}
	return e.errs[0].SourceCode()
}

// Unwrap returns the underlying errors for use with errors.Is/As.
// This implements the Go 1.20+ multi-error interface.
func (e *Errors) Unwrap() []error {
	result := make([]error, len(e.errs))
	for i, err := range e.errs {
		result[i] = err
	}
	return result
}
