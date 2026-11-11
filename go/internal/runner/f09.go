package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/bridge"
	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/eval"
	"github.com/deepnoodle-ai/risor/go/v3/internal/render"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// The additions of package F9 to the runner: the fixture test, runner
// cases, update mode, program case files and the report.

// Ctx is what the cases of a run share beyond the case file: the base
// environment, the host's variables, and update mode (F9 7.1).
type Ctx struct {
	// Base is the base environment; nil is the default environment.
	Base *risor.Env
	// Vars gives the variables a case does not supply.
	Vars risor.Vars
	// Update runs the cases in update mode (F9 4).
	Update bool
	// Context is the context of each run; nil is the background.
	Context context.Context
}

func (c *Ctx) context() context.Context {
	if c.Context == nil {
		return context.Background()
	}
	return c.Context
}

var defaultCtx = &Ctx{}

// caseRun is the state of one case across its runs.
type caseRun struct {
	ctx *Ctx
	c   *casefile.Case
	// tested is set once the fixture test is made (F9 2.2).
	tested bool
	// first is the outcome of the first run: the checked one, or the run
	// with the static rules alone under "check": false.
	first *runOutcome
	// forceRun takes the case through every stage (F9 4.4).
	forceRun bool
	conf     *conformOutcome
}

// vars are the case's variables, then the host's.
func (cr *caseRun) vars() risor.Vars {
	m := risor.Map(cr.c.Vars)
	if cr.ctx.Vars == nil {
		return m
	}
	return layered{m, cr.ctx.Vars}
}

type layered struct {
	top  risor.Map
	base risor.Vars
}

func (l layered) Var(name string) (any, bool) {
	if v, ok := l.top[name]; ok {
		return v, true
	}
	return l.base.Var(name)
}

// runOutcome is what one run of a case gave (F9 1.1).
type runOutcome struct {
	checked bool
	env     *risor.Env
	err     *risor.Error
	cr      *risor.CheckResult
	v       any
	ran     bool
	usage   risor.Usage
}

// whole is the outcome with the fields of an expect written whole (4.4).
func (o *runOutcome) whole() any {
	if o.err != nil {
		return failureData(o.err)
	}
	m := map[string]any{}
	if o.cr != nil {
		m["type"] = o.cr.Type
	}
	if o.ran {
		m["value"] = o.v
		m["steps"] = float64(o.usage.Steps)
		m["allocated"] = float64(o.usage.Alloc)
	}
	return m
}

// wholeColumns are the columns a failure written whole has (4.4).
var wholeColumns = map[string][]string{
	"declare": {"code"},
	"parse":   {"code", "span", "message"},
	"check":   {"code", "class", "span", "message"},
	"run":     {"code", "class", "span", "path", "message", "data"},
}

func columns(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

func failureData(err *risor.Error) map[string]any {
	ph := string(err.Phase())
	return map[string]any{"phase": ph, "diagnostics": diagsData(err.Diagnostics, columns(wholeColumns[ph]...), ph)}
}

// expectedColumns are code and every field some expected diagnostic has.
func expectedColumns(c *casefile.Case) map[string]bool {
	cols := map[string]bool{"code": true}
	for _, d := range c.Expect.Diagnostics {
		if d.Class != nil {
			cols["class"] = true
		}
		if d.Span != nil || d.SpanNull {
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
	return cols
}

// diagsData writes the diagnostics a phase writes, with the columns given
// that each one has (4.4).
func diagsData(ds []risor.Diagnostic, cols map[string]bool, phase string) []any {
	if (phase == "parse" || phase == "run" || phase == "declare") && len(ds) > 1 {
		ds = ds[:1]
	}
	out := make([]any, 0, len(ds))
	for _, d := range ds {
		m := map[string]any{"code": d.Code}
		limit := diag.LimitCodes[d.Code]
		if cols["class"] && d.Class != "" {
			m["class"] = d.Class
		}
		if cols["span"] && d.Span != nil && !limit {
			m["span"] = spanData(*d.Span)
		}
		if cols["path"] && d.Code == "not-conforming" {
			m["path"] = append([]any{}, d.Path...)
		}
		if cols["message"] && d.Phase != diag.PhaseDeclare {
			m["message"] = d.Message
		}
		if cols["data"] && d.Code == "failed" && d.HasData {
			m["data"] = d.Data
		}
		if cols["trace"] && d.Phase == diag.PhaseRun && !limit {
			tr := make([]any, len(d.Trace))
			for i, t := range d.Trace {
				var idx any
				if t.Index >= 0 {
					idx = float64(t.Index)
				}
				tr[i] = map[string]any{"index": idx, "name": t.Name, "span": spanData(t.Span)}
			}
			m["trace"] = tr
		}
		out = append(out, m)
	}
	return out
}

func spanData(s diag.Span) []any {
	a := s.Array()
	return []any{float64(a[0]), float64(a[1]), float64(a[2]), float64(a[3])}
}

func sitesData(ss []risor.Site) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = map[string]any{"class": s.Class, "span": spanData(s.Span)}
	}
	return out
}

func sameSites(want []casefile.Site, got []risor.Site) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i].Class != got[i].Class || want[i].Span != got[i].Span.Array() {
			return false
		}
	}
	return true
}

// jsonForm writes a value in its JSON form (F9 1.1): a value of the
// language, or of a case file. An opaque value is written as a case file
// writes one.
func jsonForm(v any) string {
	var b strings.Builder
	writeForm(&b, v)
	return b.String()
}

func writeForm(b *strings.Builder, v any) {
	if o, ok := v.(casefile.OpaqueValue); ok {
		fmt.Fprintf(b, `{"$opaque":{"id":%d,"type":%s}}`, o.ID, value.QuoteJSON(o.Type))
		return
	}
	switch x := v.(type) {
	case int:
		b.WriteString(strconv.Itoa(x))
		return
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
		return
	}
	v, _ = value.Read(v)
	switch value.KindOf(v) {
	case value.Null:
		b.WriteString("null")
	case value.Bool:
		b.WriteString(strconv.FormatBool(v.(bool)))
	case value.Number:
		b.WriteString(value.FormatNumber(v.(float64)))
	case value.String:
		b.WriteString(value.QuoteJSON(v.(string)))
	case value.List:
		b.WriteByte('[')
		for i, n := 0, value.Len(v); i < n; i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			writeForm(b, value.At(v, i))
		}
		b.WriteByte(']')
	case value.Map:
		b.WriteByte('{')
		for i, k := range value.Keys(v) {
			if i > 0 {
				b.WriteByte(',')
			}
			x, _ := value.Get(v, k)
			b.WriteString(value.QuoteJSON(k))
			b.WriteByte(':')
			writeForm(b, x)
		}
		b.WriteByte('}')
	case value.OpaqueKind:
		fmt.Fprintf(b, `{"$opaque":{"id":0,"type":%s}}`, value.QuoteJSON(value.OpaqueTypeName(v)))
	default:
		b.WriteString("null")
	}
}

// stubType is a stub's signature text: its own, or the base's (03-host.md
// 3.10.4).
func stubType(base *risor.Env, name string, s *casefile.Stub) string {
	if s.Type != nil {
		return *s.Type
	}
	if base != nil {
		if t, ok := bridge.FuncText(base, name); ok {
			return t
		}
	}
	return ""
}

const noBangMessage = "the signature has no !, so the stub may not return an error"

// fixtures is the fixture test of F9 2: the first fixture that fails, its
// place with the conformance path, and the message.
func fixtures(env *risor.Env, c *casefile.Case) ([]any, string, bool) {
	vars, funcs, methods := bridge.EnvTypes(env)
	keys := make([]string, 0, len(c.Vars))
	for k := range c.Vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t := vars[k]
		if t == nil {
			continue
		}
		if p, msg, ok := eval.Conform(c.Vars[k], t); !ok {
			return append([]any{"vars", k}, p...), msg, true
		}
	}
	// F14 10.2: each host fact whose members vars all supplies.
	for i, f := range c.Facts {
		all, nulls := true, 0
		for _, m := range f {
			v, ok := c.Vars[m]
			if !ok {
				all = false
				break
			}
			if v == nil {
				nulls++
			}
		}
		if all && len(f) > 0 && nulls == len(f) {
			return []any{"facts", float64(i)}, "at least one of " + strings.Join(f, ", ") + " must not be null", true
		}
	}
	rows := func(kind string, names []string, stubs map[string]*casefile.Stub, sig func(string) *types.Sig) ([]any, string, bool) {
		sort.Strings(names)
		for _, n := range names {
			s := sig(n)
			if s == nil {
				continue
			}
			res := anyVars(s.Result)
			for i, row := range stubs[n].Calls {
				place := []any{kind, n, "calls", float64(i)}
				if row.Error != nil {
					if !s.Fails {
						return append(place, "error"), noBangMessage, true
					}
					continue
				}
				if p, msg, ok := eval.Conform(row.Result, res); !ok {
					return append(append(place, "result"), p...), msg, true
				}
			}
		}
		return nil, "", false
	}
	fn := make([]string, 0, len(c.Functions))
	for k := range c.Functions {
		fn = append(fn, k)
	}
	if p, m, bad := rows("functions", fn, c.Functions, func(n string) *types.Sig { return funcs[n] }); bad {
		return p, m, bad
	}
	mn := make([]string, 0, len(c.Methods))
	for k := range c.Methods {
		mn = append(mn, k)
	}
	return rows("methods", mn, c.Methods, func(n string) *types.Sig {
		t, m, _ := strings.Cut(n, ".")
		return methods[t][m]
	})
}

// anyVars replaces each type variable of t by any (F9 2.1 item 5).
func anyVars(t *types.Type) *types.Type {
	vs := map[string]bool{}
	types.VarsIn(t, vs)
	if len(vs) == 0 {
		return t
	}
	sol := map[string]*types.Type{}
	for v := range vs {
		sol[v] = types.Any
	}
	return types.Subst(t, sol)
}

// renderPath is the path text of a list of keys and indexes (F9 1.1).
func renderPath(p []any) string { return render.Path(p) }

// Report is the lines of F9 6.1 for the results of one file's cases.
func Report(rs []Result) string {
	var b strings.Builder
	for _, r := range rs {
		reportCase(&b, r)
	}
	return b.String()
}

// CaseReport is the lines of F9 6.1 for one case.
func CaseReport(r Result) string {
	var b strings.Builder
	reportCase(&b, r)
	return b.String()
}

func reportCase(b *strings.Builder, r Result) {
	name := render.Visible(r.Case)
	switch {
	case r.FileInvalid:
		b.WriteString("  INVALID\n    " + render.Visible(r.Reason) + "\n")
	case r.Updated:
		b.WriteString("  UPDATED " + name + "\n")
	case r.Verdict == Fail:
		b.WriteString("  FAIL " + name + "\n")
		for _, d := range r.Diffs {
			if d.Name == "functions" {
				b.WriteString("    functions: no row of " + d.Expected + " matches " + d.Found + "\n")
				continue
			}
			b.WriteString("    " + d.Name + ": expected " + d.Expected + ", found " + d.Found + "\n")
		}
	case r.Verdict == Invalid:
		b.WriteString("  INVALID " + name + "\n")
		if r.Fixture != nil {
			b.WriteString("    fixture " + renderPath(r.Fixture) + ": " + render.Visible(r.Message) + "\n")
		} else if r.Reason != "" {
			b.WriteString("    " + render.Visible(r.Reason) + "\n")
		}
	case r.Verdict == Skip:
		b.WriteString("  SKIP " + name + ": " + render.Visible(r.Reason) + "\n")
	}
}

// LoadProgram loads a program case file (F9 5.2): parse, check in the
// default environment, run, and read the value as the JSON of a case file.
func LoadProgram(text []byte, path string, o casefile.Options) (*casefile.File, error) {
	invalid := func(err error) error {
		var e *risor.Error
		if errors.As(err, &e) {
			return &casefile.InvalidError{Path: path, Reason: strings.TrimSuffix(risor.RenderLine(e.Diagnostics), "\n")}
		}
		return &casefile.InvalidError{Path: path, Reason: err.Error()}
	}
	x, err := risor.Parse(string(text))
	if err != nil {
		return nil, invalid(err)
	}
	env, err := risor.NewEnv()
	if err != nil {
		return nil, invalid(err)
	}
	prog, err := env.Compile(x)
	if err != nil {
		return nil, invalid(err)
	}
	v, err := prog.Run(context.Background(), nil)
	if err != nil {
		return nil, invalid(err)
	}
	raw, err := toData(v, 1)
	if err != nil {
		return nil, &casefile.InvalidError{Path: path, Reason: err.Error()}
	}
	return casefile.FromValueWith(raw, path, o)
}

// toData reads a value of the language as JSON data (F9 5.3).
func toData(v any, depth int) (any, error) {
	v, _ = value.Read(v)
	switch value.KindOf(v) {
	case value.List, value.Map:
		if depth > casefile.MaxDepth {
			return nil, fmt.Errorf("the value nests deeper than %d levels", casefile.MaxDepth)
		}
	}
	switch value.KindOf(v) {
	case value.Null, value.Bool, value.String:
		return v, nil
	case value.Number:
		f := v.(float64)
		if f == 0 {
			f = 0
		}
		return f, nil
	case value.List:
		n := value.Len(v)
		out := make([]any, n)
		for i := 0; i < n; i++ {
			x, err := toData(value.At(v, i), depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = x
		}
		return out, nil
	case value.Map:
		out := map[string]any{}
		for _, k := range value.Keys(v) {
			x, _ := value.Get(v, k)
			d, err := toData(x, depth+1)
			if err != nil {
				return nil, err
			}
			out[k] = d
		}
		return out, nil
	}
	return nil, fmt.Errorf("the value holds a %s, which a case file cannot hold", value.KindOf(v))
}

// LoadedFile is a case file read for a run, with its text when it has one.
type LoadedFile struct {
	File *casefile.File
	// Text is the file's text, for update mode; nil for data.
	Text []byte
	// Program is set for a program case file, which update never writes.
	Program bool
}

// RunLoaded runs the cases of a loaded file. selected, when not nil,
// reports whether a case is run (F9 8.1 --run); a case not run has no
// result.
func RunLoaded(ctx *Ctx, lf *LoadedFile, selected func(name string) bool) []Result {
	saved := CheckedOnly
	CheckedOnly = false
	defer func() { CheckedOnly = saved }()
	run := ctx
	if lf.Program && ctx.Update {
		cp := *ctx
		cp.Update = false
		run = &cp
	}
	var out []Result
	for _, c := range lf.File.Cases {
		if selected != nil && !selected(c.Name) {
			continue
		}
		out = append(out, RunCaseWith(run, c))
	}
	return out
}

// UpdatedText is the text update mode writes for a file (F9 4.5), and
// whether it writes one: only when a case is updated.
func UpdatedText(lf *LoadedFile, rs []Result) ([]byte, bool, error) {
	if lf.Program || lf.Text == nil {
		return nil, false, nil
	}
	byName := map[string]Result{}
	any := false
	for _, r := range rs {
		if r.Updated {
			byName[r.Case] = r
			any = true
		}
	}
	if !any {
		return nil, false, nil
	}
	spans, err := casefile.CaseSpans(lf.Text)
	if err != nil {
		return nil, false, err
	}
	if len(spans) != len(lf.File.Cases) {
		return nil, false, fmt.Errorf("the text has %d cases, the file %d", len(spans), len(lf.File.Cases))
	}
	text := append([]byte{}, lf.Text...)
	for i := len(spans) - 1; i >= 0; i-- {
		r, ok := byName[lf.File.Cases[i].Name]
		if !ok {
			continue
		}
		sp := spans[i]
		var piece []byte
		var start, end int
		if sp.ExpectStart >= 0 {
			start, end, piece = sp.ExpectStart, sp.ExpectEnd, []byte(r.NewExpect)
		} else {
			start, end, piece = sp.LastEnd, sp.LastEnd, []byte(`, "expect": `+r.NewExpect)
		}
		text = append(text[:start], append(piece, text[end:]...)...)
	}
	return text, true, nil
}

// innerRun reads and runs the inner file of a runner case (F9 3.2). err
// is set when the inner file is invalid.
func innerRun(ctx *Ctx, c *casefile.Case) (*LoadedFile, []Result, error) {
	path := filepath.Join(c.Dir, "runner-case.json")
	opts := casefile.Options{Update: c.Update && !c.Program}
	if ctx.Base != nil {
		opts.BaseVar, opts.BaseFunc = BaseNames(ctx.Base)
	}
	lf := &LoadedFile{Program: c.Program}
	var err error
	switch {
	case c.RunnerText != nil && c.Program:
		lf.Text = []byte(*c.RunnerText)
		lf.File, err = LoadProgram(lf.Text, path, opts)
	case c.RunnerText != nil:
		lf.Text = []byte(*c.RunnerText)
		lf.File, err = casefile.ParseWith(lf.Text, path, opts)
	default:
		lf.File, err = casefile.FromValueWith(c.Runner, path, opts)
	}
	if err != nil {
		return nil, nil, err
	}
	inner := &Ctx{Base: ctx.Base, Vars: ctx.Vars, Update: c.Update, Context: ctx.Context}
	return lf, RunLoaded(inner, lf, nil), nil
}

// RunnerText is the text of a runner case's inner file after update mode
// (F9 3.3): its text unchanged when nothing is written.
func RunnerText(lf *LoadedFile, rs []Result) string {
	text, written, err := UpdatedText(lf, rs)
	if err != nil {
		return "an error: " + err.Error()
	}
	if written {
		return string(text)
	}
	return string(lf.Text)
}

// runRunner runs a runner case (F9 3).
func runRunner(ctx *Ctx, c *casefile.Case) Result {
	re := c.Expect.Runner
	lf, rs, err := innerRun(ctx, c)
	invalid := err != nil
	if invalid != re.Invalid {
		found := "a valid file"
		if invalid {
			found = value.QuoteJSON(err.Error())
		}
		return Result{Verdict: Fail, Diffs: []Diff{{"invalid", strconv.FormatBool(re.Invalid), found}}}
	}
	if invalid {
		return Result{Verdict: Pass}
	}
	var diffs []Diff
	if !verdictsMatch(re.Verdicts, rs) {
		diffs = append(diffs, Diff{"verdicts", expVerdictsForm(re.Verdicts), verdictsForm(rs)})
	}
	if re.Report != nil {
		if got := Report(rs); got != *re.Report {
			diffs = append(diffs, Diff{"report", value.QuoteJSON(*re.Report), value.QuoteJSON(got)})
		}
	}
	if re.Text != nil {
		if got := RunnerText(lf, rs); got != *re.Text {
			diffs = append(diffs, Diff{"text", value.QuoteJSON(*re.Text), value.QuoteJSON(got)})
		}
	}
	if len(diffs) > 0 {
		return Result{Verdict: Fail, Diffs: diffs}
	}
	return Result{Verdict: Pass}
}

// BaseNames reports what a base environment declares.
func BaseNames(base *risor.Env) (vars, funcs func(string) bool) {
	vs, fs, ms := bridge.EnvTypes(base)
	vars = func(n string) bool { _, ok := vs[n]; return ok }
	funcs = func(n string) bool {
		if _, ok := fs[n]; ok {
			return true
		}
		if t, m, ok := strings.Cut(n, "."); ok {
			_, ok := ms[t][m]
			return ok
		}
		return false
	}
	return vars, funcs
}

func verdictsMatch(want []casefile.ExpVerdict, got []Result) bool {
	if len(want) != len(got) {
		return false
	}
	for i, w := range want {
		g := got[i]
		if w.Name != g.Case || w.Status != g.Status() {
			return false
		}
		if w.Differs != nil {
			names := make([]string, len(g.Diffs))
			for j, d := range g.Diffs {
				names[j] = d.Name
			}
			if strings.Join(names, "\x00") != strings.Join(w.Differs, "\x00") || len(names) != len(w.Differs) {
				return false
			}
		}
		if w.Fixture != nil && !value.Equal(w.Fixture, g.Fixture) {
			return false
		}
		if w.Message != nil && *w.Message != g.Message {
			return false
		}
	}
	return true
}

func expVerdictsForm(vs []casefile.ExpVerdict) string {
	out := make([]any, len(vs))
	for i, v := range vs {
		m := map[string]any{"name": v.Name, "status": v.Status}
		if v.Differs != nil {
			ds := make([]any, len(v.Differs))
			for j, d := range v.Differs {
				ds[j] = d
			}
			m["differs"] = ds
		}
		if v.Fixture != nil {
			m["fixture"] = v.Fixture
		}
		if v.Message != nil {
			m["message"] = *v.Message
		}
		out[i] = m
	}
	return jsonForm(out)
}

func verdictsForm(rs []Result) string {
	out := make([]any, len(rs))
	for i, r := range rs {
		m := map[string]any{"name": r.Case, "status": r.Status()}
		if r.Verdict == Fail && !r.Updated {
			ds := make([]any, len(r.Diffs))
			for j, d := range r.Diffs {
				ds[j] = d.Name
			}
			m["differs"] = ds
		}
		if r.Fixture != nil {
			m["fixture"] = r.Fixture
			m["message"] = r.Message
		}
		out[i] = m
	}
	return jsonForm(out)
}
