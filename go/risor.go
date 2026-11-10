// Package risor is the Go implementation of Risor v3, a small expression
// language that a host program embeds. The language is defined by its
// specification (spec/core in the repository) and its conformance suite.
//
// A host works in four stages (spec/core/03-host.md 3.2): Parse turns source
// text into a Program; Check and Compile take a Program and an environment;
// Run takes a compiled program and the values of its variables. Every stage
// reports a failure as an *Error holding diagnostics of one phase.
//
// # Stack
//
// The parser and the evaluator recurse only where the text opens a level of
// nesting, and through calls in progress and walks over values, so a run's
// recursion is bounded by the limits (02-evaluation.md 8.5). A long flat
// program (100,000 terms, chain steps, else-if parts or const lines) costs
// no stack. Measured on darwin/arm64 with Go 1.26: parsing at the nesting
// limit and running at the depth limits 200 and 1,000 need less than
// 512 KiB of goroutine stack. TestStackNeeds runs the deepest programs with
// the stack capped at 1 MiB. Go grows a goroutine's stack as needed, so a
// host needs no setting for this.
package risor

import (
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
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

// Expr is a parsed program. It does not change after Parse, and may be
// used by several compiles at the same time.
type Expr struct {
	file *syntax.File
	tree *syntax.Program
	// segments and template are set for a template's program (F2).
	segments []Segment
	template bool
}

// Parse parses the source text of a program. A text gives the same Expr,
// or the same diagnostics, in every environment. On failure the error is an
// *Error with one diagnostic of phase parse: the first one of
// spec/core/01-language.md 14.1.
func Parse(source string) (*Expr, error) {
	if !utf8.ValidString(source) {
		// Each maximal subpart of ill-formed UTF-8 is one U+FFFD (03-host.md 3.4.5).
		source = value.ReplaceInvalid(source)
	}
	f := syntax.NewFile(source)
	tree, err := syntax.Parse(f)
	if err != nil {
		return nil, &Error{Diagnostics: []Diagnostic{err.Diagnostic(f)}}
	}
	return &Expr{file: f, tree: tree}, nil
}

// Source returns the program's source text.
func (x *Expr) Source() string { return string(x.file.Runes()) }

// Truthy is the language's truthiness test (03-host.md 3.2.4). It takes any
// value a run can return and cannot fail.
func Truthy(v any) bool { return value.Truthy(v) }

// AsFloat returns a number of a result as a float64.
func AsFloat(v any) (float64, bool) {
	if value.KindOf(v) != value.Number {
		return 0, false
	}
	x, e := value.Read(v)
	if e != value.ReadOK {
		return 0, false
	}
	return x.(float64), true
}

// AsInt returns a number of a result as an int64 when it is whole.
func AsInt(v any) (int64, bool) {
	f, ok := AsFloat(v)
	if !ok || !value.IsWhole(f) {
		return 0, false
	}
	return int64(f), true
}

// WalkCalls calls fn with the name of each call by name and each method call
// of the program, for tools and tests. method is true for a method call.
func WalkCalls(x *Expr, fn func(name string, method bool)) {
	syntax.Walk(x.tree, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.Call:
			fn(n.Name, false)
		case *syntax.MethodCall:
			fn(n.Name, true)
		}
		return true
	})
}

// Origin says where a source text sits in a larger document: a name, and
// the line, column and indent of its first code point (03-host.md 3.7).
// A reader of case file sources returns one beside the text (package F9
// 7.2); spans stay positions in the text itself.
type Origin struct {
	Name                 string
	Line, Column, Indent int
}

// Text is the program's own text: the source between the start and the
// end of its span. For a whole program it is Source without the white
// space and comments around it; for a program made of a part of a larger
// text, such as a call argument (CallSite.Args) or a template's part, it
// is that part alone, where Source is the whole text.
func (x *Expr) Text() string {
	r := x.tree.Result
	if r == nil {
		return ""
	}
	start, end := r.Span().Start, r.Span().End
	if len(x.tree.Consts) > 0 {
		start = x.tree.Consts[0].Span().Start
	}
	return x.file.Text(start, end)
}
