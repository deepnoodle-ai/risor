// Package risorcli is a CLI-only support package for the risor command-line
// program (module github.com/deepnoodle-ai/risor/go/cmd/risor). It exists so
// that the CLI module can stay separate from github.com/deepnoodle-ai/risor/go/v3
// without importing go/internal/, keeping go/v3 dependency-free.
//
// This package is not part of the Risor v3 host embedding API
// (spec/core/03-host.md) and carries no host API stability guarantee: hosts
// should embed github.com/deepnoodle-ai/risor/go/v3 and test with
// github.com/deepnoodle-ai/risor/go/v3/risortest. Symbols here may change or
// be removed to follow the CLI.
package risorcli

import (
	"encoding/json"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/go/v3/internal/eval"
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
