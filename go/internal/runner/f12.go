package runner

import (
	"context"
	"fmt"
	"sort"
	"strings"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/format"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// The additions of package F12 7 to the runner: format, completion and
// hover cases.

// runTools runs a case of package F12.
func runTools(c *casefile.Case) Result {
	if c.Skip != nil {
		return skip(*c.Skip)
	}
	switch c.Tools.Kind {
	case "format", "formatTemplate":
		return runFormat(c)
	case "complete":
		return runComplete(c)
	}
	return runHover(c)
}

// formatText formats the text of a format case.
func formatText(c *casefile.Case, text string) (string, error) {
	if c.Tools.Kind == "formatTemplate" {
		return risor.FormatTemplate(text)
	}
	return risor.Format(text)
}

// runFormat runs a format case (7.1). The differences are outcome,
// diagnostics, formatted, idempotent, tree and run, in that order.
func runFormat(c *casefile.Case) Result {
	t := c.Tools
	e := c.Expect
	out, err := formatText(c, t.Text)
	if err != nil {
		rerr := err.(*risor.Error)
		if !e.IsFailure() || e.Phase != "parse" {
			return Result{Verdict: Fail, Diffs: []Diff{{"outcome", expectedOutcome(c), jsonForm(failureData(rerr))}}}
		}
		if d := compareDiagnostics(c, rerr.Diagnostics, true); d != nil {
			return Result{Verdict: Fail, Diffs: []Diff{*d}}
		}
		return Result{Verdict: Pass}
	}
	if e.IsFailure() || (t.Formatted == nil && !c.NoExpect) {
		return Result{Verdict: Fail, Diffs: []Diff{{"outcome", expectedOutcome(c), `{"formatted":` + value.QuoteJSON(out) + "}"}}}
	}
	var diffs []Diff
	if t.Formatted != nil && *t.Formatted != out {
		diffs = append(diffs, Diff{"formatted", value.QuoteJSON(*t.Formatted), value.QuoteJSON(out)})
	}
	diffs = append(diffs, lawDiffs(c, t.Text, out)...)
	if len(diffs) > 0 {
		return Result{Verdict: Fail, Diffs: diffs}
	}
	return Result{Verdict: Pass}
}

// lawDiffs tests the laws of 7.1 item 4 on a text and its formatted text.
func lawDiffs(c *casefile.Case, src, out string) []Diff {
	var diffs []Diff
	again, err := formatText(c, out)
	if err != nil {
		diffs = append(diffs, Diff{"idempotent", value.QuoteJSON(out), "a failure: " + err.Error()})
	} else if again != out {
		diffs = append(diffs, Diff{"idempotent", value.QuoteJSON(out), value.QuoteJSON(again)})
	}
	a, b := TreeOf(c.Tools.Kind == "formatTemplate", src), TreeOf(c.Tools.Kind == "formatTemplate", out)
	if a != b {
		diffs = append(diffs, Diff{"tree", value.QuoteJSON(a), value.QuoteJSON(b)})
	}
	if c.Vars != nil {
		ra, rb := lawRun(c, src), lawRun(c, out)
		if ra != rb {
			diffs = append(diffs, Diff{"run", ra, rb})
		}
	}
	return diffs
}

// TreeOf writes the tree of a program text, or of a template's segments,
// with spans left out (2.2 item 1). A text that does not parse gives its
// diagnostic.
func TreeOf(template bool, text string) string {
	if template {
		f := syntax.NewTemplateFile(text)
		t, err := syntax.ParseTemplate(f)
		if err != nil {
			return "parse: " + err.Code
		}
		var b strings.Builder
		for _, s := range t.Segments {
			if s.Expr == nil {
				b.WriteString("(text " + value.QuoteJSON(s.Text) + ")")
			} else {
				b.WriteString(format.Tree(s.Expr, f.Runes()))
			}
		}
		return b.String()
	}
	f := syntax.NewFile(text)
	p, err := syntax.Parse(f)
	if err != nil {
		return "parse: " + err.Code
	}
	return format.Tree(p, f.Runes())
}

// lawRun checks and runs a text in the case's environment with its
// variables, and writes what the law run compares: phase, type, value,
// code, class, message, steps and units.
func lawRun(c *casefile.Case, text string) string {
	var b strings.Builder
	fail := func(err error) string {
		e, ok := err.(*risor.Error)
		if !ok {
			return "error " + err.Error()
		}
		b.WriteString(string(e.Phase()))
		for _, d := range e.Diagnostics {
			fmt.Fprintf(&b, " [%s %s %q]", d.Code, d.Class, d.Message)
		}
		return b.String()
	}
	env, err := buildEnv(nil, c, &stubState{})
	if err != nil {
		return fail(err)
	}
	var x *risor.Expr
	if c.Tools.Kind == "formatTemplate" {
		t, err := risor.ParseTemplate(text)
		if err != nil {
			return fail(err)
		}
		x = t.Program(risor.TemplateMode("string"))
	} else if x, err = risor.Parse(text); err != nil {
		return fail(err)
	}
	cr, err := env.Check(x)
	if err != nil {
		return fail(err)
	}
	p, err := env.Compile(x)
	if err != nil {
		return fail(err)
	}
	v, u, err := p.Measure(context.Background(), risor.Map(c.Vars))
	if err != nil {
		fail(err)
		fmt.Fprintf(&b, " steps %d units %d", u.Steps, u.Alloc)
		return b.String()
	}
	typ := ""
	if cr != nil {
		typ = cr.Type
	}
	return fmt.Sprintf("value %s type %s steps %d units %d", jsonForm(v), typ, u.Steps, u.Alloc)
}

// toolsLine is the line of the outcome command for a case of package F12
// (7.7).
func toolsLine(c *casefile.Case) string {
	var b strings.Builder
	b.WriteString(`{"name":` + value.QuoteJSON(c.Name))
	t := c.Tools
	switch t.Kind {
	case "format", "formatTemplate":
		out, err := formatText(c, t.Text)
		if err != nil {
			d := err.(*risor.Error).Diagnostics[0]
			a := d.Span.Array()
			fmt.Fprintf(&b, `,"phase":"parse","diagnostics":[{"code":%s,"span":[%d,%d,%d,%d],"message":%s}]}`, value.QuoteJSON(d.Code), a[0], a[1], a[2], a[3], value.QuoteJSON(d.Message))
			return b.String()
		}
		b.WriteString(`,"formatted":` + value.QuoteJSON(out) + "}")
		return b.String()
	}
	env, err := buildEnv(nil, c, &stubState{})
	if err != nil {
		writeDeclare(&b, err)
		return b.String()
	}
	if t.Kind == "complete" {
		b.WriteString(`,"completion":` + CompletionJSON(env.Complete(t.Source, risor.Pos{Line: t.Line, Column: t.Column}, assistOpts(c)...)) + "}")
		return b.String()
	}
	b.WriteString(`,"hover":` + HoverJSON(env.Hover(t.Source, risor.Pos{Line: t.Line, Column: t.Column}, assistOpts(c)...)) + "}")
	return b.String()
}

func writeDeclare(b *strings.Builder, err error) {
	e := err.(*risor.Error)
	codes := make([]string, len(e.Diagnostics))
	for i, d := range e.Diagnostics {
		codes[i] = d.Code
	}
	sort.Strings(codes)
	b.WriteString(`,"phase":"declare","diagnostics":[`)
	for i, code := range codes {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"code":` + value.QuoteJSON(code) + "}")
	}
	b.WriteString("]}")
}

// assistOpts is the expected result of a completion or hover case.
func assistOpts(c *casefile.Case) []risor.CheckOption {
	if c.Returns == nil {
		return nil
	}
	if *c.Returns == "condition" {
		return []risor.CheckOption{risor.Condition}
	}
	return []risor.CheckOption{risor.Expect(*c.Returns)}
}

func spanJSON(s risor.Span) string {
	a := s.Array()
	return fmt.Sprintf("[%d,%d,%d,%d]", a[0], a[1], a[2], a[3])
}

func nullJSON(s *string) string {
	if s == nil {
		return "null"
	}
	return value.QuoteJSON(*s)
}

// CompletionJSON writes a completion in the form of 7.7.
func CompletionJSON(c risor.Completion) string {
	var b strings.Builder
	b.WriteString(`{"span":` + spanJSON(c.Span) + `,"items":[`)
	for i, it := range c.Items {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"label":` + value.QuoteJSON(it.Label) + `,"kind":` + value.QuoteJSON(it.Kind) +
			`,"detail":` + nullJSON(it.Detail) + `,"insert":` + value.QuoteJSON(it.Insert) + "}")
	}
	b.WriteString("]}")
	return b.String()
}

// HoverJSON writes a hover in the form of 7.7.
func HoverJSON(h *risor.HoverInfo) string {
	if h == nil {
		return "null"
	}
	decl := "null"
	if h.Declaration != nil {
		decl = spanJSON(*h.Declaration)
	}
	return `{"kind":` + value.QuoteJSON(h.Kind) + `,"span":` + spanJSON(h.Span) + `,"type":` + nullJSON(h.Type) +
		`,"signature":` + nullJSON(h.Signature) + `,"doc":` + nullJSON(h.Doc) + `,"declaration":` + decl + "}"
}

// assistCase builds the environment of a completion or hover case; a
// declare failure is compared as the case's outcome.
func assistCase(c *casefile.Case) (*risor.Env, *Result) {
	env, err := buildEnv(nil, c, &stubState{})
	if err != nil {
		rerr := err.(*risor.Error)
		e := c.Expect
		if !e.IsFailure() {
			return nil, &Result{Verdict: Fail, Diffs: []Diff{{"outcome", expectedOutcome(c), jsonForm(failureData(rerr))}}}
		}
		if d := compareDiagnostics(c, rerr.Diagnostics, true); d != nil {
			return nil, &Result{Verdict: Fail, Diffs: []Diff{*d}}
		}
		return nil, &Result{Verdict: Pass}
	}
	if c.Expect.IsFailure() {
		return nil, &Result{Verdict: Fail, Diffs: []Diff{{"outcome", expectedOutcome(c), "an environment"}}}
	}
	return env, nil
}

func textEq(want *casefile.NullText, got *string) bool {
	if want == nil {
		return true
	}
	if want.Null {
		return got == nil
	}
	return got != nil && *got == want.Text
}

// runComplete runs a completion case (7.2).
func runComplete(c *casefile.Case) Result {
	env, r := assistCase(c)
	if r != nil {
		return *r
	}
	t := c.Tools
	got := env.Complete(t.Source, risor.Pos{Line: t.Line, Column: t.Column}, assistOpts(c)...)
	w := t.Completion
	if w == nil {
		return Result{Verdict: Pass}
	}
	ok := got.Span.Array() == w.Span && len(got.Items) == len(w.Items)
	for i := 0; ok && i < len(w.Items); i++ {
		a, e := got.Items[i], w.Items[i]
		ok = a.Label == e.Label && a.Kind == e.Kind && textEq(e.Detail, a.Detail) && (e.Insert == nil || *e.Insert == a.Insert)
	}
	if !ok {
		return Result{Verdict: Fail, Diffs: []Diff{{"completion", jsonForm(rawField(c, "completion")), CompletionJSON(got)}}}
	}
	return Result{Verdict: Pass}
}

// runHover runs a hover case (7.3).
func runHover(c *casefile.Case) Result {
	env, r := assistCase(c)
	if r != nil {
		return *r
	}
	t := c.Tools
	got := env.Hover(t.Source, risor.Pos{Line: t.Line, Column: t.Column}, assistOpts(c)...)
	if !t.HasHover {
		return Result{Verdict: Pass}
	}
	w := t.Hover
	ok := true
	switch {
	case w == nil:
		ok = got == nil
	case got == nil:
		ok = false
	default:
		ok = got.Kind == w.Kind && got.Span.Array() == w.Span && textEq(w.Type, got.Type) &&
			textEq(w.Signature, got.Signature) && textEq(w.Doc, got.Doc)
		if w.HasDeclaration {
			if w.DeclarationNull {
				ok = ok && got.Declaration == nil
			} else {
				ok = ok && got.Declaration != nil && got.Declaration.Array() == *w.Declaration
			}
		}
	}
	if !ok {
		return Result{Verdict: Fail, Diffs: []Diff{{"hover", jsonForm(rawField(c, "hover")), HoverJSON(got)}}}
	}
	return Result{Verdict: Pass}
}
