package casefile

import (
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
)

// The additions of package F8 7 to the case format.

// Renderings are the expected forms of F8 5: each one nil when not given.
type Renderings struct {
	Line, Detail, JSON *string
}

// Render is the field render of a render case (F8 7.4).
type Render struct {
	Source      *string
	Diagnostics []diag.Diagnostic
}

// Failure is one entry of failures in a conform result (F8 7.5).
type Failure struct {
	Path    []any
	Message *string
}

var (
	renderCaseFields = set("name", "opaque", "render", "skip", "expect")
	renderFields     = set("source", "diagnostics")
	renderDiagFields = set("phase", "code", "message", "class", "span", "path", "data", "trace")
	renderingFields  = set("line", "detail", "json")
	traceFields      = set("span", "name", "index")
	failureEntry     = set("path", "message")
)

// renderings reads an object of F8 7.3.
func (v *validator) renderings(x any, where string) (*Renderings, error) {
	m, ok := x.(map[string]any)
	if !ok || len(m) == 0 {
		return nil, v.invalid("%s is not an object with one or more of line, detail and json", where)
	}
	if err := v.checkFields(m, renderingFields, where); err != nil {
		return nil, err
	}
	r := &Renderings{}
	for k, p := range map[string]**string{"line": &r.Line, "detail": &r.Detail, "json": &r.JSON} {
		if x, ok := m[k]; ok {
			s, ok := x.(string)
			if !ok {
				return nil, v.invalid("%s: %s is not a string", where, k)
			}
			*p = &s
		}
	}
	return r, nil
}

// trace reads a trace of F8 7.2.
func (v *validator) trace(x any, where string) ([]diag.TraceEntry, error) {
	a, ok := x.([]any)
	if !ok {
		return nil, v.invalid("%s: trace is not a list", where)
	}
	out := []diag.TraceEntry{}
	for _, e := range a {
		m, ok := e.(map[string]any)
		if !ok || len(m) != 3 {
			return nil, v.invalid("%s: a trace entry is not an object with span, name and index", where)
		}
		if err := v.checkFields(m, traceFields, where+": trace entry"); err != nil {
			return nil, err
		}
		sp, err := v.span(m["span"], where)
		if err != nil {
			return nil, err
		}
		name, ok := m["name"].(string)
		if !ok {
			return nil, v.invalid("%s: a trace entry's name is not a string", where)
		}
		te := diag.TraceEntry{Span: spanOf(*sp), Name: name, Index: -1}
		if m["index"] != nil {
			n, ok := isWhole(m["index"])
			if !ok || n < 0 {
				return nil, v.invalid("%s: a trace entry's index is not null or a whole number of 0 or more", where)
			}
			te.Index = int(n)
		}
		out = append(out, te)
	}
	return out, nil
}

func spanOf(s [4]int) diag.Span {
	return diag.Span{Start: diag.Pos{Line: s[0], Column: s[1]}, End: diag.Pos{Line: s[2], Column: s[3]}}
}

// buildRender builds a render case (F8 7.4).
func (v *validator) buildRender(c *Case, m map[string]any, where string) (*Case, error) {
	if err := v.checkFields(m, renderCaseFields, where); err != nil {
		return nil, err
	}
	c.Skip = optString(m, "skip")
	if x, ok := m["opaque"]; ok {
		c.Opaque, _ = stringList(x)
	}
	rm, ok := m["render"].(map[string]any)
	if !ok {
		return nil, v.invalid("%s: render is not an object", where)
	}
	if err := v.checkFields(rm, renderFields, where+": render"); err != nil {
		return nil, err
	}
	r := &Render{}
	var lines []int // the length of each line of the source, in code points
	if x, ok := rm["source"]; ok {
		s, ok := x.(string)
		if !ok {
			return nil, v.invalid("%s: render: source is not a string", where)
		}
		r.Source = &s
		s = strings.TrimPrefix(s, "\uFEFF")
		for _, l := range strings.Split(s, "\n") {
			lines = append(lines, len([]rune(l)))
		}
	}
	ds, ok := rm["diagnostics"].([]any)
	if !ok || len(ds) == 0 {
		return nil, v.invalid("%s: render needs a non-empty list of diagnostics", where)
	}
	inside := func(line, col int) bool {
		return line <= len(lines) && col <= lines[line-1]+1
	}
	for i, x := range ds {
		w := fmt.Sprintf("%s: render: diagnostic %d", where, i)
		dm, ok := x.(map[string]any)
		if !ok {
			return nil, v.invalid("%s is not an object", w)
		}
		if err := v.checkFields(dm, renderDiagFields, w); err != nil {
			return nil, err
		}
		var d diag.Diagnostic
		ph, ok1 := dm["phase"].(string)
		code, ok2 := dm["code"].(string)
		msg, ok3 := dm["message"].(string)
		if !ok1 || !phases[ph] || !ok2 || !ok3 {
			return nil, v.invalid("%s needs a phase, a code and a message", w)
		}
		d.Phase, d.Code, d.Message = diag.Phase(ph), code, msg
		if x, ok := dm["class"]; ok {
			s, ok := x.(string)
			if !ok {
				return nil, v.invalid("%s: class is not a string", w)
			}
			d.Class = s
		}
		if x, ok := dm["span"]; ok {
			sp, err := v.span(x, w)
			if err != nil {
				return nil, err
			}
			if r.Source != nil && (!inside(sp[0], sp[1]) || !inside(sp[2], sp[3])) {
				return nil, v.invalid("%s: the span is outside the source text", w)
			}
			s := spanOf(*sp)
			d.Span = &s
		}
		if x, ok := dm["path"]; ok {
			if code != "not-conforming" {
				return nil, v.invalid("%s: path on a code other than not-conforming", w)
			}
			ed, err := v.diagnostic(map[string]any{"code": code, "path": x}, w)
			if err != nil {
				return nil, err
			}
			d.Path = ed.Path
			if d.Path == nil {
				d.Path = []any{}
			}
		}
		if x, ok := dm["data"]; ok {
			if code != "failed" {
				return nil, v.invalid("%s: data on a code other than failed", w)
			}
			val, err := v.opaque(x, c.Opaque, w)
			if err != nil {
				return nil, err
			}
			d.Data, d.HasData = val, true
		}
		if x, ok := dm["trace"]; ok {
			if ph != "run" {
				return nil, v.invalid("%s: trace on a phase other than run", w)
			}
			tr, err := v.trace(x, w)
			if err != nil {
				return nil, err
			}
			d.Trace = tr
		}
		r.Diagnostics = append(r.Diagnostics, d)
	}
	c.Render = r
	if v.noExpect {
		if _, ok := m["expect"]; !ok {
			c.Expect = &Expect{}
			return c, nil
		}
	}
	em, ok := m["expect"]
	if !ok {
		return nil, v.invalid("%s has no expect", where)
	}
	if mm, ok := em.(map[string]any); ok && len(mm) == 0 && v.noExpect {
		c.Expect = &Expect{}
		return c, nil
	}
	rs, err := v.renderings(em, where+": expect")
	if err != nil {
		return nil, err
	}
	c.Expect = &Expect{Render: rs}
	return c, nil
}

// conformMax reads max of a conform case (F8 7.5).
func (v *validator) conformMax(cf map[string]any, where string) (*int64, error) {
	x, ok := cf["max"]
	if !ok {
		return nil, nil
	}
	n, ok := isWhole(x)
	if !ok || n < 1 {
		return nil, v.invalid("%s: max is not a whole number of 1 or more", where)
	}
	return &n, nil
}

// failures reads failures of a conform result (F8 7.5).
func (v *validator) failures(x any, where string) ([]Failure, error) {
	a, ok := x.([]any)
	if !ok {
		return nil, v.invalid("%s: failures is not a list", where)
	}
	out := []Failure{}
	for _, e := range a {
		m, ok := e.(map[string]any)
		if !ok {
			return nil, v.invalid("%s: a failure is not an object", where)
		}
		if err := v.checkFields(m, failureEntry, where+": failure"); err != nil {
			return nil, err
		}
		p, ok := m["path"]
		if !ok {
			return nil, v.invalid("%s: a failure has no path", where)
		}
		ed, err := v.diagnostic(map[string]any{"code": "not-conforming", "path": p}, where)
		if err != nil {
			return nil, err
		}
		f := Failure{Path: ed.Path}
		if x, ok := m["message"]; ok {
			s, ok := x.(string)
			if !ok {
				return nil, v.invalid("%s: a failure's message is not a string", where)
			}
			f.Message = &s
		}
		out = append(out, f)
	}
	return out, nil
}
