package risor

import (
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// This file is package F2, templates and analysis
// (spec/packages/f02-templates-analysis.md).

// TemplateMode is how a host takes the program of a template (F2 2.5).
type TemplateMode string

// The two modes.
const (
	// ModeString: the value is always a string.
	ModeString TemplateMode = "string"
	// ModeValue: a template that is exactly one part gives its expression's
	// value as it is; any other template is as in ModeString.
	ModeValue TemplateMode = "value"
)

// Template is a parsed template: a host text with ${{ }} parts. It does not
// change after ParseTemplate.
type Template struct {
	file     *syntax.File
	t        *syntax.HostTemplate
	segments []Segment
	str, val *Expr
}

// Segment is a text run or a part of a template (F2 2.4).
type Segment struct {
	// Text is a text run's value, with each $${{ read as ${{. Empty for a
	// part.
	Text string
	// Expr is a part's expression as a program of its own, with spans in the
	// whole template text. Nil for a text run.
	Expr *Expr
	// ExprSpan is the span of a part's expression.
	ExprSpan Span
	// Span is the whole segment: a part's span includes its delimiters.
	Span Span
}

// ParseTemplate parses a template text (F2 2.1 to 2.3). It needs no
// environment. On failure the error is an *Error with one diagnostic of
// phase parse, with a position in the whole template text.
func ParseTemplate(text string) (*Template, error) {
	if !utf8.ValidString(text) {
		text = value.ReplaceInvalid(text)
	}
	f := syntax.NewTemplateFile(text)
	ht, err := syntax.ParseTemplate(f)
	if err != nil {
		return nil, &Error{Diagnostics: []Diagnostic{err.Diagnostic(f)}}
	}
	t := &Template{file: f, t: ht}
	for _, s := range ht.Segments {
		seg := Segment{Text: s.Text, Span: f.Span(s.Span.Start, s.Span.End)}
		if s.Expr != nil {
			sp := s.Expr.Span()
			seg.Text = ""
			seg.Expr = &Expr{file: f, tree: syntax.ProgramOf(s.Expr)}
			seg.ExprSpan = f.Span(sp.Start, sp.End)
		}
		t.segments = append(t.segments, seg)
	}
	t.str = &Expr{file: f, tree: ht.String, segments: t.segments, template: true}
	t.val = t.str
	if ht.Single {
		t.val = &Expr{file: f, tree: ht.Value, segments: t.segments, template: true}
	}
	return t, nil
}

// Segments returns the template's segments in the order of the text.
func (t *Template) Segments() []Segment { return t.segments }

// Expr returns the template's program in mode string: a program whose value
// is a string. Check, compile and run it as any other.
func (t *Template) Expr() *Expr { return t.str }

// Single returns the expression of a template that is exactly one part, as
// its program in mode value, or nil.
func (t *Template) Single() *Expr {
	if t.t.Single {
		return t.val
	}
	return nil
}

// Program returns the template's program in a mode (F2 2.5). An unknown
// mode is taken as ModeString.
func (t *Template) Program(mode TemplateMode) *Expr {
	if mode == ModeValue {
		return t.val
	}
	return t.str
}

// Step is one step of a static path (F2 3.5): a key. Star is for the star
// step of package F7, and is never set by this package.
type Step struct {
	Key  string
	Star bool
}

// Ref is one read of a free name, with its static path (F2 3.5).
type Ref struct {
	Name string
	Path []Step
	// Span runs from the read to the end of the last step of its path.
	Span Span
}

// CallSite is one call of a free function name, or one call n.m(...) whose
// receiver n is a free name (F2 3.6).
type CallSite struct {
	// Name is the function's name, or "n.m" for a call n.m(...).
	Name string
	// Namespace is n for a call n.m(...); empty for a call by name.
	Namespace string
	Span      Span
	// Args holds each argument as a program of its own, with spans in the
	// whole text; nil for a lambda argument, which is not a program.
	Args []*Expr
	// ArgSpans holds the span of each argument.
	ArgSpans []Span
	// InLambda is set when the call stands inside a lambda: in its body or
	// in a default of its parameters.
	InLambda bool
}

// Analysis is the report of the analysis of a program or a template
// (F2 3.3). It needs no environment.
type Analysis struct {
	// Names are the free names read, each once, in code point order.
	Names []string
	// Refs are the reads, in source order.
	Refs []Ref
	// Calls are the calls, in source order.
	Calls []CallSite
	// Literal is the program's value when IsLiteral is set: nil, bool,
	// float64, string, []any or map[string]any.
	Literal   any
	IsLiteral bool
	// Segments are a template's segments; nil for a program, and empty for
	// the empty template.
	Segments []Segment
	// Template is set for the analysis of a template.
	Template bool
}

// Analyze analyzes the program (F2 3). It never fails, charges nothing and
// calls no host function. For a template's program it includes the
// template's segments.
func (x *Expr) Analyze() *Analysis {
	a := syntax.Analyze(x.tree)
	out := &Analysis{Names: a.Names, Segments: x.segments, Template: x.template}
	if out.Names == nil {
		out.Names = []string{}
	}
	if x.template && out.Segments == nil {
		out.Segments = []Segment{}
	}
	sp := func(s syntax.Span) Span { return x.file.Span(s.Start, s.End) }
	out.Refs = make([]Ref, len(a.Refs))
	for i, r := range a.Refs {
		path := make([]Step, len(r.Path))
		for j, k := range r.Path {
			path[j] = Step{Key: k.Key, Star: k.Star}
		}
		out.Refs[i] = Ref{Name: r.Name, Path: path, Span: sp(r.Span)}
	}
	out.Calls = make([]CallSite, len(a.Calls))
	for i, c := range a.Calls {
		cs := CallSite{Name: c.Name, Namespace: c.Namespace, Span: sp(c.Span), InLambda: c.Lambda,
			Args: make([]*Expr, len(c.Args)), ArgSpans: make([]Span, len(c.Args))}
		for j, arg := range c.Args {
			cs.ArgSpans[j] = sp(arg.Span())
			if _, ok := arg.(*syntax.Lambda); !ok {
				cs.Args[j] = &Expr{file: x.file, tree: syntax.ProgramOf(arg)}
			}
		}
		out.Calls[i] = cs
	}
	out.Literal, out.IsLiteral = syntax.Literal(x.tree)
	return out
}

// Names returns the free names the program reads, sorted and unique.
func (x *Expr) Names() []string { return x.Analyze().Names }

// Refs returns the reads of free names, in source order.
func (x *Expr) Refs() []Ref { return x.Analyze().Refs }

// Calls returns the calls of the program in source order: all of them when
// no name is given, else those whose Name is one of names.
func (x *Expr) Calls(names ...string) []CallSite {
	all := x.Analyze().Calls
	if len(names) == 0 {
		return all
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var out []CallSite
	for _, c := range all {
		if want[c.Name] {
			out = append(out, c)
		}
	}
	return out
}

// Literal returns the program's value when the whole program is a literal
// expression (F2 3.7).
func (x *Expr) Literal() (any, bool) { return syntax.Literal(x.tree) }
