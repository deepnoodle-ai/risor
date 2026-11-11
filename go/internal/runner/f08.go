package runner

import (
	"fmt"
	"strconv"
	"strings"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
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

// renderForms gives the forms of ds that want names.
func renderForms(want *casefile.Renderings, ds []diag.Diagnostic, source string, hasSource bool) (line, detail, js string) {
	if want.Line != nil {
		line = risor.RenderLine(ds)
	}
	if want.Detail != nil {
		if hasSource {
			detail = risor.RenderDetail(ds, source)
		} else {
			detail = risor.RenderDetail(ds)
		}
	}
	if want.JSON != nil {
		js = risor.RenderJSON(ds)
	}
	return
}

// renderDiffs compares the forms given with those of ds (7.4); the
// differences are named line, detail and json, in that order.
func renderDiffs(want *casefile.Renderings, ds []diag.Diagnostic, source string, hasSource bool) []Diff {
	line, detail, js := renderForms(want, ds, source, hasSource)
	var diffs []Diff
	if want.Line != nil && line != *want.Line {
		diffs = append(diffs, Diff{"line", value.QuoteJSON(*want.Line), value.QuoteJSON(line)})
	}
	if want.Detail != nil && detail != *want.Detail {
		diffs = append(diffs, Diff{"detail", value.QuoteJSON(*want.Detail), value.QuoteJSON(detail)})
	}
	if want.JSON != nil && js != *want.JSON {
		diffs = append(diffs, Diff{"json", value.QuoteJSON(*want.JSON), value.QuoteJSON(js)})
	}
	return diffs
}

// renderedDiffs compares the field rendered of a failure (7.3): for phase
// parse the first diagnostic only, else the whole list. It is one
// difference, rendered.
func renderedDiffs(c *casefile.Case, err *risor.Error) []Diff {
	e := c.Expect
	if e.Rendered == nil {
		return nil
	}
	ds := err.Diagnostics
	if err.Phase() == diag.PhaseParse && len(ds) > 1 {
		ds = ds[:1]
	}
	w := e.Rendered
	line, detail, js := renderForms(w, ds, c.Source, true)
	got := map[string]any{}
	same := true
	if w.Line != nil {
		got["line"], same = line, same && line == *w.Line
	}
	if w.Detail != nil {
		got["detail"], same = detail, same && detail == *w.Detail
	}
	if w.JSON != nil {
		got["json"], same = js, same && js == *w.JSON
	}
	if same {
		return nil
	}
	return []Diff{{"rendered", jsonForm(rawField(c, "rendered")), jsonForm(got)}}
}

// runRender runs a render case (7.4).
func runRender(c *casefile.Case) Result {
	r := c.Render
	src, has := "", r.Source != nil
	if has {
		src = *r.Source
	}
	return finish(renderDiffs(c.Expect.Render, r.Diagnostics, src, has), &stubState{})
}

// conformAllDiffs runs ConformAll for a conform case with max (7.5): the
// agreement of 6.1 is the difference conforms, the list the difference
// failures.
func conformAllDiffs(c *casefile.Case, o conformOutcome, fs []risor.Diagnostic) (agree, list *Diff) {
	switch {
	case (len(fs) == 0) != o.conforms:
		agree = &Diff{"conforms", strconv.FormatBool(o.conforms), value.QuoteJSON(fmt.Sprintf("%d failures from ConformAll", len(fs)))}
	case len(fs) > 0 && (!value.Equal(fs[0].Path, o.path) || fs[0].Message != o.message):
		agree = &Diff{"conforms", jsonForm(o.path), jsonForm(fs[0].Path)}
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
		got := make([]any, len(fs))
		for i, f := range fs {
			got[i] = map[string]any{"path": f.Path, "message": f.Message}
		}
		list = &Diff{"failures", jsonForm(rawField(c, "failures")), jsonForm(got)}
	}
	return agree, list
}
