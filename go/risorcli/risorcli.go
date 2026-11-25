// Package risorcli holds the engine-backed helpers for the risor command-line
// program (module github.com/deepnoodle-ai/risor/go/cmd/risor): the REPL
// session, case-file and conformance-suite runners, text value rendering, and
// strict JSON decoding. It is not part of the Risor v3 host API
// (spec/core/03-host.md); its symbols may change to follow the CLI.
package risorcli

import (
	"encoding/json"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/go/v3/internal/eval"
	"github.com/deepnoodle-ai/risor/go/v3/internal/render"
)

// DecodeJSON reads one JSON value under the strict rules of the specification
// (no repeated object keys, no trailing text, no numbers that read as
// infinity, and a nesting limit).
func DecodeJSON(data []byte) (any, error) {
	return casefile.DecodeJSON(data)
}

// RenderText renders a value a run returned (01-language.md 12) under the
// environment's depth limit. It is not a run, so it charges no steps or units.
func RenderText(env *risor.Env, v any) (string, error) {
	depth := eval.DefaultLimits.Depth
	if desc, err := env.Describe(); err == nil {
		var d struct {
			Limits struct{ Depth int } `json:"limits"`
		}
		if json.Unmarshal(desc, &d) == nil && d.Limits.Depth > 0 {
			depth = d.Limits.Depth
		}
	}
	text, d := eval.RenderText(v, depth)
	if d != nil {
		return "", &risor.Error{Diagnostics: []risor.Diagnostic{*d}}
	}
	return text, nil
}

// Visible writes each control character in s as an escape (F8 1.1).
func Visible(s string) string {
	return render.Visible(s)
}
