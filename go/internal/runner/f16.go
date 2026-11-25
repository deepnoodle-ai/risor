package runner

import (
	"fmt"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// compareEntries compares CheckResult.Entries against expect.entries
// (package F16 slice D2, 12.10).
func compareEntries(c *casefile.Case, want []casefile.ExpEntry, cr *risor.CheckResult) *Diff {
	if cr == nil {
		return &Diff{"entries", jsonForm(rawField(c, "entries")), "no check result"}
	}
	got := cr.Entries
	if len(got) != len(want) {
		return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
	}
	for i, w := range want {
		g := got[i]
		if g.Name != w.Name {
			return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
		}
		if w.HasDoc {
			if w.Doc == nil && g.Doc != "" || w.Doc != nil && g.Doc != *w.Doc {
				return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
			}
		}
		if w.HasParams {
			if len(g.Params) != len(w.Params) {
				return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
			}
			for j, wp := range w.Params {
				gp := g.Params[j]
				if (wp.Name == nil && gp.Name != "") || (wp.Name != nil && gp.Name != *wp.Name) {
					return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
				}
				if !sameType(wp.Type, gp.Type.String(), c.Opaque) {
					return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
				}
				if wp.HasDoc {
					if (wp.Doc == nil && gp.Doc != "") || (wp.Doc != nil && gp.Doc != *wp.Doc) {
						return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
					}
				}
				if gp.Optional != wp.Optional {
					return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
				}
			}
		}
		if w.HasInput {
			if w.Input == nil {
				if g.InputType != nil {
					return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
				}
			} else if g.InputType == nil || !sameType(*w.Input, g.InputType.String(), c.Opaque) {
				return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
			}
		}
		if w.HasInputSchema {
			if d := compareInputSchema(w.InputSchema, g.InputType); d != nil {
				return &Diff{"entries", jsonForm(rawField(c, "entries")), fmt.Sprintf("entry %s inputSchema: %s", g.Name, d.Found)}
			}
		}
		if w.Result != nil {
			if g.ResultType == nil || !sameType(*w.Result, g.ResultType.String(), c.Opaque) {
				return &Diff{"entries", jsonForm(rawField(c, "entries")), entriesJSON(got)}
			}
		}
	}
	return nil
}

func compareInputSchema(want any, t *risor.Type) *Diff {
	if t == nil {
		if want == nil {
			return nil
		}
		return &Diff{"schema", jsonForm(want), "null"}
	}
	return compareTypeSchema(want, *t)
}

func compareTypeSchema(want any, ty risor.Type) *Diff {
	text, err := risor.SchemaOf(ty)
	if err != nil {
		if want == nil {
			return nil
		}
		return &Diff{"schema", jsonForm(want), "refused: " + err.Error()}
	}
	got, derr := casefile.DecodeJSON(text)
	if derr != nil || want == nil || !value.Equal(got, want) {
		return &Diff{"schema", jsonForm(want), string(text)}
	}
	back, err := risor.TypeFromSchema(text)
	if err != nil {
		return &Diff{"schema", jsonForm(want), "the law: " + err.Error()}
	}
	again, err := risor.SchemaOf(back)
	if err != nil || !back.Equal(ty) {
		return &Diff{"schema", jsonForm(want), "the law: maps back to " + back.String()}
	}
	g2, _ := casefile.DecodeJSON(again)
	if !value.Equal(sortAnyOf(inlineDefs(got, g2)), sortAnyOf(g2)) {
		return &Diff{"schema", jsonForm(want), "the law: maps back to the schema " + string(again)}
	}
	return nil
}

func entriesJSON(es []risor.Entry) string {
	out := make([]any, len(es))
	for i, e := range es {
		m := map[string]any{"name": e.Name}
		if e.Doc != "" {
			m["doc"] = e.Doc
		} else {
			m["doc"] = nil
		}
		ps := make([]any, len(e.Params))
		for j, p := range e.Params {
			pm := map[string]any{"type": p.Type.String(), "optional": p.Optional}
			if p.Name != "" {
				pm["name"] = p.Name
			} else {
				pm["name"] = nil
			}
			if p.Doc != "" {
				pm["doc"] = p.Doc
			} else {
				pm["doc"] = nil
			}
			ps[j] = pm
		}
		m["params"] = ps
		if e.InputType != nil {
			m["input"] = e.InputType.String()
		} else {
			m["input"] = nil
		}
		if e.ResultType != nil {
			m["result"] = e.ResultType.String()
		} else {
			m["result"] = nil
		}
		out[i] = m
	}
	return jsonForm(out)
}
