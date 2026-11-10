package risor

import (
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/render"
)

// The three renderings of package F8 5. Each writes the diagnostics one
// after another, each one's text ending with a line feed, and is the same
// text in every implementation.

// TraceEntry is one call in progress of a run diagnostic's trace.
type TraceEntry = diag.TraceEntry

// RenderLine writes each diagnostic on one line:
// "error[code] L:C-L:C: message at path".
func RenderLine(ds []Diagnostic) string { return render.Line(ds) }

// RenderDetail writes each diagnostic in the detail form: its line, an
// excerpt of the source text with carets under the span, and the trace,
// innermost first. source is the text that was parsed; with none given,
// the excerpt is left out. At most one source is used.
func RenderDetail(ds []Diagnostic, source ...string) string {
	if len(source) == 0 {
		return render.Detail(ds, "", false)
	}
	return render.Detail(ds, source[0], true)
}

// RenderJSON writes each diagnostic as one JSON object on a line, keys in
// code point order.
func RenderJSON(ds []Diagnostic) string { return render.JSON(ds) }
