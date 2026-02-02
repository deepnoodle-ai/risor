package syntax

import (
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/v2/ast"
	"github.com/deepnoodle-ai/risor/v2/internal/token"
)

// ValidationError represents a syntax restriction violation.
type ValidationError struct {
	Message  string         // description of the violation
	Node     ast.Node       // the offending node
	Position token.Position // source location
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	pos := e.Position
	if pos.File != "" {
		return fmt.Sprintf("%s at %s:%d:%d", e.Message, pos.File, pos.LineNumber(), pos.ColumnNumber())
	}
	return fmt.Sprintf("%s at line %d, column %d", e.Message, pos.LineNumber(), pos.ColumnNumber())
}

// ValidationErrors wraps multiple validation errors.
type ValidationErrors struct {
	Errors []ValidationError
}

// NewValidationErrors creates a ValidationErrors from a slice of errors.
func NewValidationErrors(errs []ValidationError) *ValidationErrors {
	return &ValidationErrors{Errors: errs}
}

// Error implements the error interface.
func (e *ValidationErrors) Error() string {
	switch len(e.Errors) {
	case 0:
		return "no validation errors"
	case 1:
		return e.Errors[0].Error()
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "%d validation errors:\n", len(e.Errors))
		for _, err := range e.Errors {
			fmt.Fprintf(&b, "  - %s\n", err.Error())
		}
		return b.String()
	}
}

// Unwrap returns the first error for errors.Is/As compatibility.
func (e *ValidationErrors) Unwrap() error {
	if len(e.Errors) > 0 {
		return &e.Errors[0]
	}
	return nil
}

// Validator inspects an AST and returns validation errors.
// Validators should not modify the AST.
type Validator interface {
	// Validate checks the AST and returns any validation errors.
	// Multiple errors may be returned to show all violations at once.
	Validate(program *ast.Program) []ValidationError
}

// ValidatorFunc is an adapter to use a function as a Validator.
type ValidatorFunc func(*ast.Program) []ValidationError

// Validate implements the Validator interface.
func (f ValidatorFunc) Validate(p *ast.Program) []ValidationError {
	return f(p)
}
