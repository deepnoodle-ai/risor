package runner

import (
	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// Update mode (package F9 4).

// whole is a conform result written whole (4.4).
func (o conformOutcome) whole() any {
	m := map[string]any{"conforms": o.conforms, "steps": float64(o.steps)}
	if !o.conforms {
		m["path"], m["message"] = o.path, o.message
	}
	return m
}

// update runs a case in update mode and gives its update status (4.3).
func (cr *caseRun) update() Result {
	c := cr.c
	if c.Skip != nil || c.Expect.Runner != nil || c.Render != nil || c.Expect.Analysis != nil || c.Tools != nil { // F12 7: not written by update mode
		return runCase(cr)
	}
	if c.NoExpect {
		cr.forceRun = true
	}
	r := runCase(cr)
	if (r.Verdict == Pass && !c.NoExpect) || r.Verdict == Skip || r.Verdict == Invalid || hasDiff(r, "functions") {
		return r
	}
	notWritten := func() Result {
		if c.NoExpect {
			out := cr.first
			found := "nothing"
			if out != nil {
				found = jsonForm(out.whole())
			} else if cr.conf != nil {
				found = jsonForm(cr.conf.whole())
			}
			return Result{Verdict: Fail, Diffs: []Diff{{"outcome", "nothing", found}}}
		}
		return r
	}
	e := c.Expect
	if c.Conform != nil {
		o := cr.conf
		if o == nil || o.declare != nil {
			if o != nil && !c.NoExpect && e.Phase == "declare" && len(o.declare.Diagnostics) == 1 {
				return written(r, failureNew(e, o.declare.Diagnostics, "declare", nil))
			}
			return notWritten()
		}
		if !o.agree {
			return notWritten()
		}
		var m map[string]any
		if !c.NoExpect && e.Conform != nil {
			m = map[string]any{"conforms": o.conforms}
			old := e.Conform
			if !o.conforms {
				if old.HasPath {
					m["path"] = o.path
				}
				if old.Message != nil {
					m["message"] = o.message
				}
			}
			if old.Steps != nil {
				m["steps"] = float64(o.steps)
			}
			if old.Failures != nil && c.Conform.Max != nil {
				fs := make([]any, len(o.all))
				for i, f := range o.all {
					fs[i] = map[string]any{"path": f.Path, "message": f.Message}
				}
				m["failures"] = fs
			}
		} else {
			m = o.whole().(map[string]any)
		}
		return written(r, m)
	}
	o := cr.first
	if o == nil {
		return notWritten()
	}
	if o.err != nil && o.err.Phase() == diag.PhaseDeclare {
		if c.NoExpect || e.Phase != "declare" || len(o.err.Diagnostics) != 1 {
			return notWritten()
		}
	}
	same := !c.NoExpect && ((o.err == nil && !e.IsFailure()) || (o.err != nil && e.Phase == string(o.err.Phase())))
	if !same && o.err == nil && !o.ran {
		// Written whole: taken through every stage (4.4).
		again := &caseRun{ctx: cr.ctx, c: c, forceRun: true, tested: true}
		if c.Check != nil && !*c.Check {
			again.runMode(false, true)
		} else {
			again.runMode(true, true)
		}
		o = again.first
	}
	var m map[string]any
	switch {
	case o.err != nil && same:
		m = failureNew(e, o.err.Diagnostics, string(o.err.Phase()), &o.usage)
		if e.Rendered != nil && !diag.LimitCodes[o.err.Diagnostics[0].Code] {
			ds := o.err.Diagnostics
			if o.err.Phase() == diag.PhaseParse {
				ds = ds[:1]
			}
			forms := map[string]any{}
			if e.Rendered.Line != nil {
				forms["line"] = renderForm("line", ds, c.Source)
			}
			if e.Rendered.Detail != nil {
				forms["detail"] = renderForm("detail", ds, c.Source)
			}
			if e.Rendered.JSON != nil {
				forms["json"] = renderForm("json", ds, c.Source)
			}
			m["rendered"] = forms
		}
	case o.err != nil:
		m = failureData(o.err)
	case same:
		m = map[string]any{}
		if e.Type != nil && o.cr != nil {
			m["type"] = o.cr.Type
		}
		if e.HasSites && o.cr != nil {
			m["sites"] = sitesData(o.cr.Sites)
		}
		if e.HasValue {
			m["value"] = o.v
		}
		if e.Truthy != nil {
			m["truthy"] = value.Truthy(o.v)
		}
		if e.Steps != nil {
			m["steps"] = float64(o.usage.Steps)
		}
		if e.Allocated != nil {
			m["allocated"] = float64(o.usage.Alloc)
		}
		if e.HasDescription && o.env != nil {
			if text, err := o.env.Describe(); err == nil {
				if d, err := casefile.DecodeJSON(text); err == nil {
					m["description"] = d
				}
			}
		}
		if e.JSON != nil {
			if t, err := risor.JSONText(o.v); err == nil {
				m["json"] = t
			}
		}
	default:
		m = o.whole().(map[string]any)
	}
	if holdsOpaqueKey(m["value"]) || depthOf(m, 1) > casefile.MaxDepth-2 {
		return notWritten()
	}
	return written(r, m)
}

func written(r Result, m map[string]any) Result {
	return Result{Verdict: Pass, Updated: true, NewExpect: jsonForm(m), Case: r.Case}
}

func hasDiff(r Result, name string) bool {
	for _, d := range r.Diffs {
		if d.Name == name {
			return true
		}
	}
	return false
}

// failureNew is a failure of the same phase (4.4): code and the columns of
// the old list, and the counts the old expect had.
func failureNew(e *casefile.Expect, ds []diag.Diagnostic, phase string, u *risor.Usage) map[string]any {
	cols := map[string]bool{"code": true}
	for _, d := range e.Diagnostics {
		if d.Class != nil {
			cols["class"] = true
		}
		if d.Span != nil {
			cols["span"] = true
		}
		if d.HasPath {
			cols["path"] = true
		}
		if d.Message != nil {
			cols["message"] = true
		}
		if d.HasData {
			cols["data"] = true
		}
		if d.Trace != nil {
			cols["trace"] = true
		}
	}
	m := map[string]any{"phase": phase, "diagnostics": diagsData(ds, cols, phase)}
	if phase == "run" && u != nil && !diag.LimitCodes[ds[0].Code] {
		if e.Steps != nil {
			m["steps"] = float64(u.Steps)
		}
		if e.Allocated != nil {
			m["allocated"] = float64(u.Alloc)
		}
	}
	return m
}

// holdsOpaqueKey reports whether a value holds a map of the language with
// the key $opaque (4.3).
func holdsOpaqueKey(v any) bool {
	if _, ok := v.(casefile.OpaqueValue); ok {
		return false
	}
	v, _ = value.Read(v)
	switch value.KindOf(v) {
	case value.List:
		for i, n := 0, value.Len(v); i < n; i++ {
			if holdsOpaqueKey(value.At(v, i)) {
				return true
			}
		}
	case value.Map:
		if _, ok := value.Get(v, "$opaque"); ok {
			return true
		}
		for _, k := range value.Keys(v) {
			x, _ := value.Get(v, k)
			if holdsOpaqueKey(x) {
				return true
			}
		}
	}
	return false
}

// depthOf counts nesting as JSON text does, the top value at level 1.
func depthOf(v any, d int) int {
	if _, ok := v.(casefile.OpaqueValue); ok {
		return d + 1
	}
	v, _ = value.Read(v)
	best := d
	switch value.KindOf(v) {
	case value.List:
		for i, n := 0, value.Len(v); i < n; i++ {
			if x := depthOf(value.At(v, i), d+1); x > best {
				best = x
			}
		}
	case value.Map:
		for _, k := range value.Keys(v) {
			x, _ := value.Get(v, k)
			if y := depthOf(x, d+1); y > best {
				best = y
			}
		}
	}
	return best
}

// renderForm is one rendering of F8 5 of the diagnostics, with the source.
func renderForm(form string, ds []diag.Diagnostic, source string) string {
	switch form {
	case "line":
		return risor.RenderLine(ds)
	case "detail":
		return risor.RenderDetail(ds, source)
	}
	return risor.RenderJSON(ds)
}
