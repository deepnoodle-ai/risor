// Package risor is the Go implementation of Risor v3, a small expression
// language that a host program embeds. The language is defined by its
// specification (spec/core in the repository) and its conformance suite.
//
// A host works in four stages (spec/core/03-host.md 3.2): Parse turns source
// text into a Program; Check and Compile take a Program and an environment;
// Run takes a compiled program and the values of its variables. Every stage
// reports a failure as an *Error holding diagnostics of one phase.
package risor

import (
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
)

// Diagnostic is one problem found by a stage, in the one shape of
// spec/core/03-host.md 3.7.
type Diagnostic = diag.Diagnostic

// Span is a start and an exclusive end position in the source text.
type Span = diag.Span

// Pos is a line and a column, both counted from 1. A line ends at U+000A and
// a column counts code points.
type Pos = diag.Pos

// Phase is the stage that reported a diagnostic: declare, parse, check or run.
type Phase = diag.Phase

// The four phases.
const (
	PhaseDeclare = diag.PhaseDeclare
	PhaseParse   = diag.PhaseParse
	PhaseCheck   = diag.PhaseCheck
	PhaseRun     = diag.PhaseRun
)

// Error is the failure of a stage: a list of diagnostics of one phase.
type Error = diag.Error

// Program is a parsed program. It does not change after Parse, and may be
// used by several compiles at the same time.
type Program struct {
	file *syntax.File
	tree *syntax.Program
}

// Parse parses the source text of a program. A text gives the same Program,
// or the same diagnostics, in every environment. On failure the error is an
// *Error with one diagnostic of phase parse: the first one of
// spec/core/01-language.md 14.1.
func Parse(source string) (*Program, error) {
	f := syntax.NewFile(source)
	tree, err := syntax.Parse(f)
	if err != nil {
		return nil, &Error{Diagnostics: []Diagnostic{err.Diagnostic(f)}}
	}
	return &Program{file: f, tree: tree}, nil
}

// Source returns the program's source text.
func (p *Program) Source() string { return string(p.file.Runes()) }
