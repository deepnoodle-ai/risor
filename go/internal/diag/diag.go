// Package diag holds the one shape of a diagnostic (03-host.md 3.7), shared by
// every stage of the implementation. The public package re-exports it.
package diag

import (
	"fmt"
	"strings"
)

// Phase is the stage that reported a diagnostic (03-host.md 3.7.2).
type Phase string

// The four phases.
const (
	PhaseDeclare Phase = "declare"
	PhaseParse   Phase = "parse"
	PhaseCheck   Phase = "check"
	PhaseRun     Phase = "run"
)

// Pos is a position in the source text. Line and Column count from 1. A line
// ends at U+000A only, and a column counts code points (03-host.md 3.7.7).
type Pos struct {
	Line   int
	Column int
}

// Span is a start and an exclusive end position in the source text.
type Span struct {
	Start Pos
	End   Pos
}

// Array returns the span as the four numbers of a case file:
// start line, start column, end line, end column.
func (s Span) Array() [4]int {
	return [4]int{s.Start.Line, s.Start.Column, s.End.Line, s.End.Column}
}

// String formats the span as "[l, c, l, c]".
func (s Span) String() string {
	return fmt.Sprintf("[%d, %d, %d, %d]", s.Start.Line, s.Start.Column, s.End.Line, s.End.Column)
}

// Diagnostic is one problem found by a stage (03-host.md 3.7.1).
type Diagnostic struct {
	// Phase is the stage that found the problem.
	Phase Phase
	// Code names the kind of problem, such as "unexpected-token".
	Code string
	// Class is the class of a run diagnostic, or of a check diagnostic that a
	// rejected check class caused. Empty when the rule gives none.
	Class string
	// Span is where the problem is. Nil when the rule gives none.
	Span *Span
	// Message is one sentence.
	Message string
	// Path is the place of a not-conforming failure in the tested value. Each
	// step is a string key or an int index. Nil for every other code.
	Path []any
	// Data is the second argument of fail, for the code "failed".
	Data any
	// HasData reports whether Data is present. A failed run with no second
	// argument has HasData true and Data holding the null value.
	HasData bool
	// Trace is, for a run diagnostic, the calls in progress when the run
	// failed, from the outermost in (package F8 4). It may be empty. Nil
	// for every other phase.
	Trace []TraceEntry
}

// TraceEntry is one call in progress of a trace (package F8 4.2).
type TraceEntry struct {
	// Span is the span of the call.
	Span Span
	// Name is the name of the const-bound lambda, or of the library entry
	// as written.
	Name string
	// Index is the index of the element a library entry gave its call of
	// the lambda, from 0, or -1 for a const-bound lambda (null in JSON).
	Index int
}

// String formats the diagnostic for people.
func (d Diagnostic) String() string {
	var b strings.Builder
	b.WriteString(string(d.Phase))
	b.WriteString(" ")
	b.WriteString(d.Code)
	if d.Span != nil {
		b.WriteString(" at ")
		b.WriteString(d.Span.String())
	}
	if d.Message != "" {
		b.WriteString(": ")
		b.WriteString(d.Message)
	}
	return b.String()
}

// Error is a failure of a stage: a list of diagnostics of one phase.
type Error struct {
	Diagnostics []Diagnostic
}

// Error returns the first diagnostic as text.
func (e *Error) Error() string {
	if e == nil || len(e.Diagnostics) == 0 {
		return "risor: failure with no diagnostic"
	}
	s := e.Diagnostics[0].String()
	if n := len(e.Diagnostics) - 1; n > 0 {
		s += fmt.Sprintf(" (and %d more)", n)
	}
	return s
}

// Phase returns the phase of the failure.
func (e *Error) Phase() Phase {
	if e == nil || len(e.Diagnostics) == 0 {
		return ""
	}
	return e.Diagnostics[0].Phase
}

// The parse codes of Core (01-language.md 14.2).
const (
	CodeUnexpectedToken     = "unexpected-token"
	CodeUnexpectedCharacter = "unexpected-character"
	CodeUnterminatedString  = "unterminated-string"
	CodeUnterminatedComment = "unterminated-comment"
	CodeBadEscape           = "bad-escape"
	CodeLineContinuation    = "line-continuation"
	CodeBadNumber           = "bad-number"
	CodeLiteralRange        = "literal-range"
	CodeTextBlockOpen       = "text-block-open"
	CodeTextBlockClose      = "text-block-close"
	CodeTextBlockIndent     = "text-block-indent"
	CodeNestingExceeded     = "nesting-exceeded"
	CodeBadTypeText         = "bad-type-text"
	CodeReservedWord        = "reserved-word"
)

// The declare codes of Core (03-host.md 3.7.3).
const (
	CodeNameClash          = "name-clash"
	CodeReservedNamespace  = "reserved-namespace"
	CodeBadName            = "bad-name"
	CodeBadSignature       = "bad-signature"
	CodeBadType            = "bad-type"
	CodeBadClass           = "bad-class"
	CodeUnsupportedVersion = "unsupported-version"
	CodeNoImplementation   = "no-implementation"
	CodeBadLimit           = "bad-limit"
)

// LimitCodes are the run codes of class limit (03-host.md 3.7.4).
var LimitCodes = map[string]bool{
	"steps-exceeded":      true,
	"allocation-exceeded": true,
	"depth-exceeded":      true,
	"deadline-exceeded":   true,
	"cancelled":           true,
}
