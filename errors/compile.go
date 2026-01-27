package errors

import (
	"fmt"
	"strings"
)

// CompileError represents a compilation error with rich context.
type CompileError struct {
	Code        ErrorCode
	Message     string
	Filename    string
	Line        int
	Column      int
	EndColumn   int
	SourceLine  string
	Suggestions []Suggestion
	Note        string
}

// Error implements the error interface.
func (e *CompileError) Error() string {
	var b strings.Builder
	b.WriteString("compile error: ")
	b.WriteString(e.Message)
	if e.Filename != "" || e.Line > 0 {
		b.WriteString("\n\nlocation: ")
		if e.Filename != "" {
			b.WriteString(e.Filename)
			b.WriteString(":")
		}
		fmt.Fprintf(&b, "%d:%d", e.Line, e.Column)
		fmt.Fprintf(&b, " (line %d, column %d)", e.Line, e.Column)
	}
	return b.String()
}

// FriendlyErrorMessage returns a human-friendly error message.
func (e *CompileError) FriendlyErrorMessage() string {
	formatted := e.ToFormatted()
	formatter := NewFormatter(false)
	return formatter.Format(formatted)
}

// ToFormatted converts to the FormattedError type for display.
func (e *CompileError) ToFormatted() *FormattedError {
	fe := &FormattedError{
		Code:     e.Code,
		Kind:     "error",
		Message:  e.Message,
		Filename: e.Filename,
		Line:     e.Line,
		Column:   e.Column,
		Note:     e.Note,
	}

	if e.SourceLine != "" {
		fe.SourceLines = []SourceLineEntry{
			{Number: e.Line, Text: e.SourceLine, IsMain: true},
		}
	}

	if len(e.Suggestions) > 0 {
		fe.Hint = FormatSuggestions(e.Suggestions)
	}

	return fe
}

// CompileErrors holds multiple compile errors.
type CompileErrors struct {
	Errors []*CompileError
}

// Error implements the error interface.
func (e *CompileErrors) Error() string {
	if len(e.Errors) == 0 {
		return ""
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", e.Errors[0].Error(), len(e.Errors)-1)
}

// FriendlyErrorMessage returns a human-friendly error message for all errors.
func (e *CompileErrors) FriendlyErrorMessage() string {
	if len(e.Errors) == 0 {
		return ""
	}

	var formatted []*FormattedError
	for _, err := range e.Errors {
		formatted = append(formatted, err.ToFormatted())
	}

	formatter := NewFormatter(false)
	return formatter.FormatMultiple(formatted)
}

// Add adds a compile error to the collection.
func (e *CompileErrors) Add(err *CompileError) {
	e.Errors = append(e.Errors, err)
}

// Count returns the number of errors.
func (e *CompileErrors) Count() int {
	return len(e.Errors)
}

// HasErrors returns true if there are any errors.
func (e *CompileErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// ToError returns the errors as a single error, or nil if empty.
func (e *CompileErrors) ToError() error {
	if len(e.Errors) == 0 {
		return nil
	}
	if len(e.Errors) == 1 {
		return e.Errors[0]
	}
	return e
}
