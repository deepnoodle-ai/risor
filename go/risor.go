// Package risor is the Go implementation of Risor v3, a small expression
// language that a host program embeds. The language is defined by its
// specification (spec/core in the repository) and its conformance suite.
//
// A host works in four stages (spec/core/03-host.md 3.2): Parse turns source
// text into an Expr; Check and Compile take an Expr and an environment;
// Run takes a compiled program and the values of its variables. Every stage
// reports a failure as an *Error holding diagnostics of one phase.
//
// # Stack
//
// The parser and the evaluator recurse only where the text opens a level of
// nesting, and through calls in progress and walks over values, so a run's
// recursion is bounded by the limits (02-evaluation.md 8.5). A long flat
// program (100,000 terms, chain steps, else-if parts or const lines) costs
// no stack. This binding needs 2 MiB of goroutine stack to parse at the
// nesting limit and to run at the depth limits 200 and 400 (03-host.md
// 3.9.1). Go grows a goroutine's stack as needed, by doubling, so a host
// needs no setting for this, and the stack a run holds is the next power of
// two above the bytes it uses. The deepest cases are calls that flatMap
// makes of a lambda, at depth 400. On linux/amd64 with Go 1.26.11 each link
// of a chain of them used 1,360 bytes (read from a stack trace, before each
// call had its own frame), about 545 KiB in all, so the goroutine holds 1
// MiB. On darwin/arm64 with Go 1.26.2 a level of that chain uses about
// 1,320 bytes and a level of a lambda that gives its own name to flatMap
// about 1,400 (measured by the deepest run under a capped stack), 515 KiB
// and 547 KiB at depth 400, so each holds 1 MiB. A chain of calls by name,
// a lambda that calls itself by name, and a walk of a value nested 400 deep
// each hold at most 256 KiB (darwin/arm64). TestStackNeeds runs the deepest
// cases at both limits with the stack capped at 2 MiB divided by 1.5, which
// a stack of 1 MiB passes and one of 2 MiB does not.
//
// # Memory
//
// On 64-bit platforms, the heap memory a run creates as values of the
// language is at most 256 bytes per allocation unit charged plus 16 bytes per
// step charged (02-evaluation.md 8.5 item 2). The most bytes per unit come
// from a string just under a unit's 64 weight in three-byte characters: 189
// bytes, rounded to Go's 192-byte size class, and a 16-byte string header
// boxed in an interface. A number a step makes is boxed in 8 bytes. This is
// measured, not derived: TestMemoryConstants runs the worst shapes (such
// strings, empty and one-entry maps, empty lists, boxed numbers) on Go 1.26,
// whose maps are Swiss tables, and checks each run's bytes against these
// constants.
package risor

import (
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
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
// spec/core/01-language.md 14.1. A text longer than the source limit,
// MaxSource or a lower SourceLimit, fails with source-too-large before it
// is read.
func Parse(source string, opts ...ParseOption) (*Expr, error) {
	if err := checkSource(source, opts); err != nil {
		return nil, err
	}
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

// Text is the program's own text: the source between the start and the
// end of its span, without the white space and comments around it. For a
// program made of a part of a larger text, such as a call argument
// (CallSite.Args) or a template's part, it is that part alone.
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
