package runner

import (
	"fmt"
	"strings"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// The additions of package F8 7 to the runner.

// traceMatches compares an expected trace with the actual one (7.2).
func traceMatches(want, got []diag.TraceEntry) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

func formatTrace(t []diag.TraceEntry) string {
	parts := make([]string, len(t))
	for i, e := range t {
		idx := "null"
		if e.Index >= 0 {
			idx = fmt.Sprint(e.Index)
		}
		parts[i] = fmt.Sprintf("%s %s %s", e.Name, e.Span.String(), idx)
	}
	return "trace [" + strings.Join(parts, ", ") + "]"
}

// renderDiffs compares the forms given with those of ds (7.3, 7.4); the
// differences are named line, detail and json, in that order.
func renderDiffs(want *casefile.Renderings, ds []diag.Diagnostic, source string, hasSource bool, prefix string) []Diff {
	var diffs []Diff
	if want.Line != nil {
		if got := risor.RenderLine(ds); got != *want.Line {
			diffs = append(diffs, Diff{prefix + "line", *want.Line, got})
		}
	}
	if want.Detail != nil {
		var got string
		if hasSource {
			got = risor.RenderDetail(ds, source)
		} else {
			got = risor.RenderDetail(ds)
		}
		if got != *want.Detail {
			diffs = append(diffs, Diff{prefix + "detail", *want.Detail, got})
		}
	}
	if want.JSON != nil {
		if got := risor.RenderJSON(ds); got != *want.JSON {
			diffs = append(diffs, Diff{prefix + "json", *want.JSON, got})
		}
	}
	return diffs
}

// renderedDiffs compares the field rendered of a failure (7.3): for phase
// parse the first diagnostic only, else the whole list.
func renderedDiffs(c *casefile.Case, err *risor.Error) []Diff {
	e := c.Expect
	if e.Rendered == nil {
		return nil
	}
	ds := err.Diagnostics
	if err.Phase() == diag.PhaseParse && len(ds) > 1 {
		ds = ds[:1]
	}
	return renderDiffs(e.Rendered, ds, c.Source, true, "rendered ")
}

// runRender runs a render case (7.4).
func runRender(c *casefile.Case) Result {
	r := c.Render
	src, has := "", r.Source != nil
	if has {
		src = *r.Source
	}
	return finish(renderDiffs(c.Expect.Render, r.Diagnostics, src, has, ""), &stubState{})
}

// conformAllDiffs runs ConformAll for a conform case with max (7.5): the
// agreement of 6.1 is the difference conforms, the list the difference
// failures.
func conformAllDiffs(c *casefile.Case, o conformOutcome, fs []risor.Diagnostic) (agree, list *Diff) {
	switch {
	case (len(fs) == 0) != o.conforms:
		agree = &Diff{"conforms", "ConformAll agrees with the function", fmt.Sprintf("%d failures where conforms is %v", len(fs), o.conforms)}
	case len(fs) > 0 && (!value.Equal(fs[0].Path, o.path) || fs[0].Message != o.message):
		agree = &Diff{"conforms", "ConformAll's first failure is the function's", fmtValue(fs[0].Path) + " " + fs[0].Message}
	}
	want := c.Expect.Conform.Failures
	if want == nil {
		return agree, nil
	}
	ok := len(want) == len(fs)
	for i := 0; ok && i < len(want); i++ {
		if !value.Equal(want[i].Path, fs[i].Path) || (want[i].Message != nil && *want[i].Message != fs[i].Message) {
			ok = false
		}
	}
	if !ok {
		list = &Diff{"failures", formatFailures(want), formatGotFailures(fs)}
	}
	return agree, list
}

func formatFailures(fs []casefile.Failure) string {
	parts := make([]string, len(fs))
	for i, f := range fs {
		parts[i] = fmtValue(f.Path)
		if f.Message != nil {
			parts[i] += fmt.Sprintf(" %q", *f.Message)
		}
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

func formatGotFailures(fs []risor.Diagnostic) string {
	parts := make([]string, len(fs))
	for i, f := range fs {
		parts[i] = fmtValue(f.Path) + fmt.Sprintf(" %q", f.Message)
	}
	return "[" + strings.Join(parts, "; ") + "]"
}
