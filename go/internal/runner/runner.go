// Package runner runs case files (spec/core/03-host.md 3.10) against the
// implementation and gives each case a verdict.
//
// This implementation has no type checker yet. By 3.10.8 item 5 and ruling
// A3 it gives skip to a case that states type or sites, or that expects a
// check code outside the static list. It compares class by the column of a
// program compiled without a check (03-host.md 3.7.6).
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/eval"
	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// Verdict is the verdict of a case (3.10.8).
type Verdict string

// The four verdicts.
const (
	Pass    Verdict = "pass"
	Fail    Verdict = "fail"
	Skip    Verdict = "skip"
	Invalid Verdict = "invalid"
)

// Diff is one difference of a failed case.
type Diff struct {
	Name     string // outcome, type, sites, value, truthy, diagnostics, steps, allocated, functions
	Expected string
	Found    string
}

// Result is the verdict of one case.
type Result struct {
	File    string
	Case    string
	Verdict Verdict
	// Reason says why a case was skipped or why its file is invalid.
	Reason string
	Diffs  []Diff
}

// String formats the result for people, with its differences.
func (r Result) String() string {
	s := fmt.Sprintf("%s: %s: %s", filepath.Base(r.File), r.Case, r.Verdict)
	if r.Reason != "" {
		s += " (" + r.Reason + ")"
	}
	for _, d := range r.Diffs {
		s += fmt.Sprintf("\n    %s: expected %s, found %s", d.Name, d.Expected, d.Found)
	}
	return s
}

// RunFile reads a case file and runs every case of it.
func RunFile(path string) []Result {
	f, err := casefile.Load(path)
	if err != nil {
		inv, _ := err.(*casefile.InvalidError)
		reason := err.Error()
		var names []string
		if inv != nil {
			reason, names = inv.Reason, inv.Names
		}
		if len(names) == 0 {
			names = []string{"(file)"}
		}
		out := make([]Result, len(names))
		for i, n := range names {
			out[i] = Result{File: path, Case: n, Verdict: Invalid, Reason: reason}
		}
		return out
	}
	// 03-host.md 3.11 item 7 (ruling BA11): a file of a package directory
	// whose name ends -check.json is run checked only, as core-check is.
	if CheckOnlyFile(path) && !CheckedOnly {
		CheckedOnly = true
		defer func() { CheckedOnly = false }()
	}
	out := make([]Result, 0, len(f.Cases))
	for _, c := range f.Cases {
		r := RunCase(c)
		r.File = path
		out = append(out, r)
	}
	return out
}

// CheckOnlyFile reports whether a case file is run checked only by its
// name (03-host.md 3.11 item 7).
func CheckOnlyFile(path string) bool { return strings.HasSuffix(filepath.Base(path), "-check.json") }

// Files returns the case files a path names: the path itself if it is a
// file, else every .json file under the directory, in order.
// Offered is the one list of the follow-on packages this implementation
// offers (03-host.md 3.11 item 2: F3, F1, F2, F4, F5, F6, F7, ... in that
// order, each with all before it).
var Offered = map[string]bool{"F1": true, "F2": true, "F3": true, "F4": true, "F5": true, "F6": true, "F7": true, "F8": true}

// Skipped returns whether a case file is one a runner of this
// implementation does not run: its name begins until-fNN- and package NN is
// offered (03-host.md 3.11 item 4, ruling BA8).
func Skipped(path string) bool {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "until-f") {
		return false
	}
	rest := base[len("until-f"):]
	i := strings.IndexByte(rest, '-')
	if i <= 0 {
		return false
	}
	return Offered["F"+strings.TrimLeft(rest[:i], "0")]
}

// SkippedFiles lists the until- files of a directory that Files leaves
// out.
func SkippedFiles(path string) []string {
	var out []string
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".json") && Skipped(p) {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// Files returns the case files of a path, less the until- files of the
// packages this implementation offers.
func Files(path string) ([]string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return []string{path}, nil
	}
	var out []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".json") && !Skipped(p) {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// Counts returns the number of results of each verdict.
func Counts(rs []Result) map[Verdict]int {
	m := map[Verdict]int{Pass: 0, Fail: 0, Skip: 0, Invalid: 0}
	for _, r := range rs {
		m[r.Verdict]++
	}
	return m
}

// FormatCounts formats counts in the order pass, fail, skip, invalid.
func FormatCounts(m map[Verdict]int) string {
	return fmt.Sprintf("pass %d, fail %d, skip %d, invalid %d", m[Pass], m[Fail], m[Skip], m[Invalid])
}

func skip(reason string) Result { return Result{Verdict: Skip, Reason: reason} }

// RunCase runs one valid case (3.10.5).
func RunCase(c *casefile.Case) Result {
	r := runCase(c)
	r.Case = c.Name
	return r
}

// staticCodes are the eleven static codes (05-types.md 22.1).
var staticCodes = casefile.StaticCodes

// checkedClass are the codes whose class differs between a checked and an
// unchecked program (03-host.md 3.7.6).
var checkedClass = map[string]bool{"null-operand": true, "wrong-kind": true,
	"missing-field": true, "no-match": true, "not-conforming": true, "function-error": true}

// stubState records whether a call of a stub matched no row (3.10.4).
type stubState struct {
	unmatched []string
	checked   bool
}

func (st *stubState) native(name string, s *casefile.Stub) risor.Native {
	return func(call *risor.Call, args []any) (any, error) {
		for _, row := range s.Calls {
			if row.HasArgs {
				if len(row.Args) != len(args) {
					continue
				}
				match := true
				for i := range args {
					if !value.Equal(args[i], row.Args[i]) {
						match = false
						break
					}
				}
				if !match {
					continue
				}
			}
			if err := call.Charge(row.Charge); err != nil {
				return nil, err
			}
			if err := call.Allocate(row.Allocate); err != nil {
				return nil, err
			}
			if row.Error != nil {
				return nil, errors.New(*row.Error)
			}
			return row.Result, nil
		}
		st.unmatched = append(st.unmatched, name)
		return nil, fmt.Errorf("no row of the stub %s matches the call", name)
	}
}

// buildEnv builds the case's environment as a child of the base (3.10.9).
func buildEnv(c *casefile.Case, st *stubState) (*risor.Env, error) {
	base, err := risor.NewEnv()
	if err != nil {
		return nil, err
	}
	var decls []risor.Decl
	if c.Version != "" {
		decls = append(decls, risor.Version(c.Version))
	}
	if c.Env != nil {
		decls = append(decls, risor.Declare(*c.Env))
	}
	schemaNames := make([]string, 0, len(c.Schemas))
	for k := range c.Schemas {
		schemaNames = append(schemaNames, k)
	}
	sort.Strings(schemaNames)
	for _, k := range schemaNames {
		sv := c.Schemas[k]
		opts := make([]risor.SchemaOption, len(sv.Options))
		for i, o := range sv.Options {
			opts[i] = risor.SchemaOption(o)
		}
		decls = append(decls, risor.VarSchema(k, sv.Schema, opts...))
	}
	for _, o := range c.Opaque {
		decls = append(decls, risor.OpaqueType{Name: o})
	}
	if c.Groups != nil { // F6 11.1
		decls = append(decls, risor.Groups(c.Groups...))
	}
	for _, nt := range c.Types { // F4 9.1, in order
		if nt.Type != nil {
			decls = append(decls, risor.NamedType(nt.Name, *nt.Type))
			continue
		}
		opts := make([]risor.SchemaOption, len(nt.Options))
		for i, o := range nt.Options {
			opts[i] = risor.SchemaOption(o)
		}
		decls = append(decls, risor.NamedTypeSchema(nt.Name, nt.Schema, opts...))
	}
	names := make([]string, 0, len(c.Methods))
	for k := range c.Methods {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		typ, m, _ := strings.Cut(k, ".")
		s := c.Methods[k]
		decls = append(decls, risor.AddMethod(typ, m, risor.Function{Type: *s.Type, Impl: st.native(k, s)}))
	}
	names = names[:0]
	for k := range c.Functions {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		s := c.Functions[k]
		decls = append(decls, risor.Function{Name: k, Type: *s.Type, Impl: st.native(k, s)})
	}
	if c.HasReject {
		decls = append(decls, risor.RejectClasses(c.Reject...))
	}
	if len(c.Removed) > 0 {
		decls = append(decls, risor.Without(c.Removed...))
	}
	if c.Limits != nil {
		l := risor.Limits{}
		if c.Limits.Steps != nil {
			l.Steps = *c.Limits.Steps
			if l.Steps == 0 {
				l.Steps = zeroLimit
			}
		}
		if c.Limits.Allocation != nil {
			l.Alloc = *c.Limits.Allocation
			if l.Alloc == 0 {
				l.Alloc = zeroLimit
			}
		}
		if c.Limits.Depth != nil {
			l.Depth = int(*c.Limits.Depth)
		}
		decls = append(decls, l)
	}
	if c.Returns != nil && *c.Returns != "condition" {
		if _, err := typetext.ParseType(*c.Returns); err != nil {
			return nil, &risor.Error{Diagnostics: []risor.Diagnostic{{Phase: diag.PhaseDeclare, Code: diag.CodeBadType, Message: err.Error()}}}
		}
	}
	env, err := base.With(decls...)
	if err != nil {
		return nil, err
	}
	// The expected result is resolved at declare, in every run: records that
	// form no union there are bad-type (F5 3.2 item 2).
	if c.Returns != nil && *c.Returns != "condition" {
		if _, err := env.ParseType(*c.Returns); err != nil {
			return nil, err
		}
	}
	return env, nil
}

// zeroLimit stands for a limit of 0, since a zero field of risor.Limits
// keeps the default.
const zeroLimit = risor.ZeroLimit

// missingEntries lists the library entries a program calls that this
// implementation does not have yet.
func missingEntries(x *risor.Expr, c *casefile.Case) []string {
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	risor.WalkCalls(x, func(name string, method bool) {
		if method {
			for _, k := range []value.Kind{value.String, value.List, value.Map} {
				if e, ok := library.Method(k, name); ok && !eval.Implemented(e) {
					add(k.String() + "." + name)
				}
			}
			return
		}
		if _, host := c.Functions[name]; host {
			return
		}
		if e, ok := library.Functions[name]; ok && !eval.Implemented(e) {
			add(name)
		}
	})
	sort.Strings(out)
	return out
}

func runCase(c *casefile.Case) Result {
	if c.Skip != nil {
		return skip(*c.Skip)
	}
	if c.Expect.Analysis != nil {
		return runAnalysis(c)
	}
	if c.Conform != nil {
		return runConform(c)
	}
	if c.Render != nil { // F8 7.8 item 3
		return runRender(c)
	}
	// Ruling AP1: a case with "check": false is compiled with the static
	// rules alone, and its class is compared for every code.
	if c.Check != nil && !*c.Check {
		return runMode(c, false, true)
	}
	// Ruling AP2: checked, then with the static rules alone; one verdict.
	r := runMode(c, true, true)
	if r.Verdict != Pass || CheckedOnly {
		return r
	}
	// The case is also run with the static rules alone (3.10.8 item 5),
	// unless it needs the checker.
	e := c.Expect
	if e.Type != nil || e.HasSites || c.Schemas != nil || e.HasDescription { // F5 11.2 item 2
		return r
	}
	if e.Phase == "check" {
		for _, d := range e.Diagnostics {
			if !staticCodes[d.Code] && !casefile.ExtraStatic[d.Code] {
				return r
			}
		}
	}
	u := runMode(c, false, false)
	if u.Verdict != Pass {
		for i := range u.Diffs {
			u.Diffs[i].Name += " (unchecked)"
		}
	}
	return u
}

// CheckedOnly makes RunCase run each case with the checker only, as the
// directory suite/core-check is run.
var CheckedOnly bool

// runMode runs a case with the checker, or with the static rules alone.
// allClasses compares class for every code.
func runMode(c *casefile.Case, checked, allClasses bool) Result {
	e := c.Expect
	st := &stubState{checked: allClasses}
	env, err := buildEnv(c, st)
	if err != nil {
		return compareFailure(c, err.(*risor.Error), nil, st)
	}
	x, err := parseCase(c)
	if err != nil {
		return compareFailure(c, err.(*risor.Error), nil, st)
	}
	var opts []risor.CheckOption
	if !checked {
		opts = append(opts, risor.NoCheck)
	}
	if c.Returns != nil && checked {
		if *c.Returns == "condition" {
			opts = append(opts, risor.Condition)
		} else {
			opts = append(opts, risor.Expect(*c.Returns))
		}
	}
	var cr *risor.CheckResult
	if checked {
		cr, err = env.Check(x, opts...)
		if err != nil {
			return compareFailure(c, err.(*risor.Error), nil, st)
		}
	}
	prog, err := env.Compile(x, opts...)
	if err != nil {
		return compareFailure(c, err.(*risor.Error), nil, st)
	}
	needRun := c.Vars != nil || e.HasValue || e.JSON != nil || e.Truthy != nil || e.Steps != nil || e.Allocated != nil || e.Phase == "run"
	if !needRun {
		return compareResult(c, env, cr, nil, false, nil, st)
	}
	v, usage, err := prog.Measure(context.Background(), risor.Map(c.Vars))
	if err != nil {
		return compareFailure(c, err.(*risor.Error), &usage, st)
	}
	return compareResult(c, env, cr, v, true, &usage, st)
}

func fmtValue(v any) string {
	defer func() { recover() }()
	return value.Debug(v)
}

func countDiff(name string, want *int64, got int64) *Diff {
	if want == nil || *want == got {
		return nil
	}
	return &Diff{name, fmt.Sprint(*want), fmt.Sprint(got)}
}

func finish(diffs []Diff, st *stubState) Result {
	if len(st.unmatched) > 0 {
		diffs = append(diffs, Diff{"functions", "a row for every call", "no row matched a call of " + strings.Join(st.unmatched, ", ")})
	}
	if len(diffs) > 0 {
		return Result{Verdict: Fail, Diffs: diffs}
	}
	return Result{Verdict: Pass}
}

func compareResult(c *casefile.Case, env *risor.Env, cr *risor.CheckResult, v any, ran bool, u *risor.Usage, st *stubState) Result {
	e := c.Expect
	if e.IsFailure() {
		return finish([]Diff{{"outcome", "a failure of phase " + e.Phase, "a result " + fmtValue(v)}}, st)
	}
	var diffs []Diff
	if e.HasDescription {
		// F1 7.2: compared as JSON data.
		text, err := env.Describe()
		var got any
		if err == nil {
			got, err = casefile.DecodeJSON(text)
		}
		if err != nil || !value.Equal(got, e.Description) {
			want, _ := json.Marshal(e.Description)
			diffs = append(diffs, Diff{"description", string(want), string(text)})
		}
	}
	if e.Type != nil {
		found := "no type"
		if cr != nil {
			found = cr.Type
		}
		if cr == nil || !sameType(*e.Type, cr.Type, c.Opaque) {
			diffs = append(diffs, Diff{"type", *e.Type, found})
		}
	}
	if e.HasSites {
		want, got := formatSites(e.Sites), "[]"
		if cr != nil {
			got = formatGot(cr.Sites)
		}
		if want != got {
			diffs = append(diffs, Diff{"sites", want, got})
		}
	}
	if e.HasValue && !value.Equal(v, e.Value) {
		diffs = append(diffs, Diff{"value", fmtValue(e.Value), fmtValue(v)})
	}
	if e.JSON != nil { // F6 11.2: after value
		got, err := risor.JSONText(v)
		if err != nil {
			got = "a failure: " + err.Error()
		}
		if got != *e.JSON {
			diffs = append(diffs, Diff{"json", *e.JSON, got})
		}
	}
	if e.Truthy != nil && *e.Truthy != value.Truthy(v) {
		diffs = append(diffs, Diff{"truthy", fmt.Sprint(*e.Truthy), fmt.Sprint(value.Truthy(v))})
	}
	if u != nil {
		if d := countDiff("steps", e.Steps, u.Steps); d != nil {
			diffs = append(diffs, *d)
		}
		if d := countDiff("allocated", e.Allocated, u.Alloc); d != nil {
			diffs = append(diffs, *d)
		}
	}
	return finish(diffs, st)
}

func compareFailure(c *casefile.Case, err *risor.Error, u *risor.Usage, st *stubState) Result {
	e := c.Expect
	ph := string(err.Phase())
	if !e.IsFailure() || e.Phase != ph {
		want := "a result"
		if e.IsFailure() {
			want = "a failure of phase " + e.Phase
		}
		return finish([]Diff{{"outcome", want, "a failure of phase " + ph + ": " + err.Error()}}, st)
	}
	var diffs []Diff
	if d := compareDiagnostics(e, err.Diagnostics, st.checked); d != nil {
		diffs = append(diffs, *d)
	}
	diffs = append(diffs, renderedDiffs(c, err)...)
	if u != nil && ph == "run" && !diag.LimitCodes[err.Diagnostics[0].Code] {
		if d := countDiff("steps", e.Steps, u.Steps); d != nil {
			diffs = append(diffs, *d)
		}
		if d := countDiff("allocated", e.Allocated, u.Alloc); d != nil {
			diffs = append(diffs, *d)
		}
	}
	return finish(diffs, st)
}

func formatDiags(ds []risor.Diagnostic) string {
	parts := make([]string, len(ds))
	for i, d := range ds {
		parts[i] = d.Code
		if d.Class != "" {
			parts[i] += " (" + d.Class + ")"
		}
		if d.Span != nil {
			parts[i] += " " + d.Span.String()
		}
		if d.Message != "" {
			parts[i] += fmt.Sprintf(" %q", d.Message)
		}
		if d.HasData {
			parts[i] += " data " + fmtValue(d.Data)
		}
		if d.Phase == diag.PhaseRun && len(d.Trace) > 0 {
			parts[i] += " " + formatTrace(d.Trace)
		}
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

func formatExp(ds []*casefile.ExpDiag) string {
	parts := make([]string, len(ds))
	for i, d := range ds {
		parts[i] = d.Code
		if d.Span != nil {
			parts[i] += fmt.Sprintf(" [%d, %d, %d, %d]", d.Span[0], d.Span[1], d.Span[2], d.Span[3])
		}
		if d.Class != nil {
			parts[i] += " (" + *d.Class + ")"
		}
		if d.Message != nil {
			parts[i] += fmt.Sprintf(" %q", *d.Message)
		}
		if d.HasData {
			parts[i] += " data " + fmtValue(d.Data)
		}
		if len(d.Trace) > 0 {
			parts[i] += " " + formatTrace(d.Trace)
		}
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

func diagMatches(want *casefile.ExpDiag, got risor.Diagnostic, checked bool) bool {
	if want.Code != got.Code {
		return false
	}
	// Class is compared where the two columns of 03-host.md 3.7.6 agree.
	// For the codes below the class depends on the check (3.10.7 item 8),
	// and this implementation compiles without one.
	if want.Class != nil && (checked || !checkedClass[got.Code]) && *want.Class != got.Class {
		return false
	}
	if want.Span != nil && !diag.LimitCodes[got.Code] {
		if got.Span == nil || got.Span.Array() != *want.Span {
			return false
		}
	}
	if want.Message != nil && *want.Message != got.Message {
		return false
	}
	if want.HasPath {
		if len(want.Path) != len(got.Path) {
			return false
		}
		for i := range want.Path {
			if fmt.Sprint(want.Path[i]) != fmt.Sprint(got.Path[i]) {
				return false
			}
		}
	}
	if want.HasData && (!got.HasData || !value.Equal(want.Data, got.Data)) {
		return false
	}
	if want.Trace != nil && !diag.LimitCodes[got.Code] && !traceMatches(want.Trace, got.Trace) { // F8 7.2
		return false
	}
	return true
}

func compareDiagnostics(e *casefile.Expect, got []risor.Diagnostic, checked bool) *Diff {
	d := &Diff{Name: "diagnostics", Expected: formatExp(e.Diagnostics), Found: formatDiags(got)}
	switch e.Phase {
	case "parse": // compared with the start of what the parser reported
		if len(got) < len(e.Diagnostics) {
			return d
		}
		for i, w := range e.Diagnostics {
			if !diagMatches(w, got[i], checked) {
				return d
			}
		}
	case "declare": // without regard to order
		if len(got) != len(e.Diagnostics) {
			return d
		}
		used := make([]bool, len(got))
		for _, w := range e.Diagnostics {
			found := false
			for j, g := range got {
				if !used[j] && diagMatches(w, g, checked) {
					used[j], found = true, true
					break
				}
			}
			if !found {
				return d
			}
		}
	default: // whole, in order
		if len(got) != len(e.Diagnostics) {
			return d
		}
		for i, w := range e.Diagnostics {
			if !diagMatches(w, got[i], checked) {
				return d
			}
		}
	}
	return nil
}

// RunEntries runs the cases of a catalog file whose entries each carry a
// list of cases with no header: suite/library.json and suite/messages.json
// (3.10.1 item 3). key names the field that names an entry.
func RunEntries(path, key string) []Result {
	data, err := os.ReadFile(path)
	if err != nil {
		return []Result{{File: path, Case: "(file)", Verdict: Invalid, Reason: err.Error()}}
	}
	v, err := casefile.DecodeJSON(data)
	if err != nil {
		return []Result{{File: path, Case: "(file)", Verdict: Invalid, Reason: err.Error()}}
	}
	entries, _ := v.([]any)
	var out []Result
	for _, e := range entries {
		m, _ := e.(map[string]any)
		name, _ := m[key].(string)
		cases, ok := m["cases"]
		if !ok {
			continue
		}
		f, err := casefile.FromValue(cases, path)
		if err != nil {
			out = append(out, Result{File: path, Case: name, Verdict: Invalid, Reason: err.Error()})
			continue
		}
		for _, c := range f.Cases {
			r := RunCase(c)
			r.File = path
			r.Case = name + ": " + r.Case
			out = append(out, r)
		}
	}
	return out
}

// sameType compares two type texts by structure (3.10.6).
func sameType(want, got string, opaque []string) bool {
	set := map[string]bool{}
	for _, o := range opaque {
		set[o] = true
	}
	parse := func(s string) *types.Type {
		tt, err := typetext.ParseType(s)
		if err != nil {
			return nil
		}
		t, err := types.FromText(tt, set, false)
		if err != nil {
			return nil
		}
		return t
	}
	a, b := parse(want), parse(got)
	return a != nil && b != nil && types.Equal(a, b)
}

func formatSites(ss []casefile.Site) string {
	parts := make([]string, len(ss))
	for i, s := range ss {
		parts[i] = fmt.Sprintf("%s [%d, %d, %d, %d]", s.Class, s.Span[0], s.Span[1], s.Span[2], s.Span[3])
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

func formatGot(ss []risor.Site) string {
	parts := make([]string, len(ss))
	for i, s := range ss {
		a := s.Span.Array()
		parts[i] = fmt.Sprintf("%s [%d, %d, %d, %d]", s.Class, a[0], a[1], a[2], a[3])
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

// parseCase is step 3 of 3.10.5: the text as a program, or as a template in
// the case's mode (F2 4.1).
func parseCase(c *casefile.Case) (*risor.Expr, error) {
	if c.Template == "" {
		return risor.Parse(c.Source)
	}
	t, err := risor.ParseTemplate(c.Source)
	if err != nil {
		return nil, err
	}
	return t.Program(risor.TemplateMode(c.Template)), nil
}

// runAnalysis runs an analysis case (F2 4.2): Read, Parse, Analyze. Each
// field the case states is compared whole.
func runAnalysis(c *casefile.Case) Result {
	x, err := parseCase(c)
	if err != nil {
		return Result{Verdict: Fail, Diffs: []Diff{{"outcome", "an analysis", "a failure of phase parse: " + err.Error()}}}
	}
	text := AnalysisJSON(x.Analyze())
	got, err := casefile.DecodeJSON([]byte(text))
	if err != nil {
		panic(err)
	}
	gm := got.(map[string]any)
	keys := make([]string, 0, len(c.Expect.Analysis))
	for k := range c.Expect.Analysis {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !value.Equal(gm[k], c.Expect.Analysis[k]) {
			want, _ := json.Marshal(c.Expect.Analysis)
			return Result{Verdict: Fail, Diffs: []Diff{{"analysis", string(want), text}}}
		}
	}
	return Result{Verdict: Pass}
}

// AnalysisJSON writes a report in its JSON form (F2 3.3, 4.3, 4.8): the
// fields in the order of 3.3, and those of each reference, call and segment
// in the order of 4.3.
func AnalysisJSON(a *risor.Analysis) string {
	var b strings.Builder
	span := func(s risor.Span) {
		x := s.Array()
		fmt.Fprintf(&b, "[%d,%d,%d,%d]", x[0], x[1], x[2], x[3])
	}
	b.WriteString(`{"names":[`)
	for i, n := range a.Names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(value.QuoteJSON(n))
	}
	b.WriteString(`],"refs":[`)
	for i, r := range a.Refs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"name":` + value.QuoteJSON(r.Name) + `,"path":[`)
		for j, st := range r.Path {
			if j > 0 {
				b.WriteByte(',')
			}
			if st.Star { // F7 9.3
				b.WriteString(`{"star":true}`)
				continue
			}
			b.WriteString(value.QuoteJSON(st.Key))
		}
		b.WriteString(`],"span":`)
		span(r.Span)
		b.WriteByte('}')
	}
	b.WriteString(`],"calls":[`)
	for i, c := range a.Calls {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"name":` + value.QuoteJSON(c.Name) + `,"namespace":`)
		if c.Namespace == "" {
			b.WriteString("null")
		} else {
			b.WriteString(value.QuoteJSON(c.Namespace))
		}
		b.WriteString(`,"span":`)
		span(c.Span)
		b.WriteString(`,"args":[`)
		for j, s := range c.ArgSpans {
			if j > 0 {
				b.WriteByte(',')
			}
			span(s)
		}
		fmt.Fprintf(&b, `],"lambda":%t}`, c.InLambda)
	}
	b.WriteString(`],"literal":`)
	if a.IsLiteral {
		b.WriteString(`{"value":`)
		writeJSON(&b, a.Literal)
		b.WriteByte('}')
	} else {
		b.WriteString("false")
	}
	b.WriteString(`,"segments":`)
	if !a.Template {
		b.WriteString("null")
	} else {
		b.WriteByte('[')
		for i, sg := range a.Segments {
			if i > 0 {
				b.WriteByte(',')
			}
			if sg.Expr == nil {
				b.WriteString(`{"text":` + value.QuoteJSON(sg.Text) + `,"span":`)
			} else {
				b.WriteString(`{"part":`)
				span(sg.ExprSpan)
				b.WriteString(`,"span":`)
			}
			span(sg.Span)
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	b.WriteByte('}')
	return b.String()
}

// conformOutcome is what the three ways of F4 9.3 give for a conform case.
type conformOutcome struct {
	declare  *risor.Error
	conforms bool
	path     []any
	message  string
	steps    int64
	agree    bool
	why      string             // the first disagreement
	all      []risor.Diagnostic // ConformAll's list, with max (F8 7.5)
}

// doConform runs a conform case by F4 9.3: declare, resolve, the function,
// and the runs of v as T and v is T with the static rules alone.
func doConform(c *casefile.Case) conformOutcome {
	var o conformOutcome
	env, err := buildEnv(c, &stubState{})
	if err != nil {
		o.declare = err.(*risor.Error)
		return o
	}
	t, err := env.ParseType(c.Conform.Type)
	if err != nil {
		o.declare = err.(*risor.Error)
		return o
	}
	ferr := risor.Conform(c.Conform.Value, t)
	if c.Conform.Max != nil {
		o.all = risor.ConformAll(c.Conform.Value, t, int(*c.Conform.Max))
	}
	o.conforms = ferr == nil
	if ferr != nil {
		d := ferr.(*risor.Error).Diagnostics[0]
		o.path, o.message = d.Path, d.Message
	}
	o.agree = true
	disagree := func(why string) {
		if o.agree {
			o.agree, o.why = false, why
		}
	}
	child, err := env.With(risor.Var("v", "any"))
	if err != nil {
		disagree("the child environment: " + err.Error())
		return o
	}
	run := func(src string) (any, risor.Usage, error) {
		x, err := risor.Parse(src)
		if err != nil {
			return nil, risor.Usage{}, err
		}
		p, err := child.Compile(x, risor.NoCheck)
		if err != nil {
			return nil, risor.Usage{}, err
		}
		return p.Measure(context.Background(), risor.Map{"v": c.Conform.Value})
	}
	av, au, aerr := run("v as " + c.Conform.Type)
	o.steps = au.Steps
	if aerr != nil {
		e, ok := aerr.(*risor.Error)
		if !ok || e.Diagnostics[0].Code != "not-conforming" {
			disagree("v as T: " + aerr.Error())
		} else {
			d := e.Diagnostics[0]
			if o.conforms {
				disagree("v as T fails where the function passes")
			} else if !value.Equal(d.Path, o.path) || d.Message != o.message {
				disagree("v as T: path or message differs from the function")
			}
		}
	} else if !o.conforms {
		disagree("v as T passes where the function fails")
	} else if !value.Equal(av, c.Conform.Value) {
		disagree("v as T gives another value")
	}
	iv, iu, ierr := run("v is " + c.Conform.Type)
	switch {
	case ierr != nil:
		disagree("v is T: " + ierr.Error())
	case iv != o.conforms:
		disagree("v is T gives " + fmt.Sprint(iv))
	case iu.Steps != au.Steps:
		disagree(fmt.Sprintf("v is T charges %d steps, v as T %d", iu.Steps, au.Steps))
	}
	return o
}

// runConform runs and compares a conform case (F4 9.2, 9.3).
func runConform(c *casefile.Case) Result {
	o := doConform(c)
	e := c.Expect
	if o.declare != nil {
		return compareFailure(c, o.declare, nil, &stubState{})
	}
	if e.IsFailure() {
		return Result{Verdict: Fail, Diffs: []Diff{{"outcome", "a failure of phase " + e.Phase, "a conform result"}}}
	}
	r := e.Conform
	var diffs []Diff
	if !o.agree {
		diffs = append(diffs, Diff{"conforms", "the three ways agree", o.why})
	}
	if r.Conforms != o.conforms {
		diffs = append(diffs, Diff{"conforms", fmt.Sprint(r.Conforms), fmt.Sprint(o.conforms)})
	}
	if r.HasPath && !value.Equal(r.Path, o.path) {
		diffs = append(diffs, Diff{"path", fmtValue(r.Path), fmtValue(o.path)})
	}
	if r.Message != nil && *r.Message != o.message {
		diffs = append(diffs, Diff{"message", *r.Message, o.message})
	}
	var list *Diff
	if c.Conform.Max != nil { // F8 7.5
		var agree *Diff
		agree, list = conformAllDiffs(c, o, o.all)
		if agree != nil {
			diffs = append([]Diff{*agree}, diffs...)
		}
	}
	if d := countDiff("steps", r.Steps, o.steps); d != nil {
		diffs = append(diffs, *d)
	}
	if list != nil {
		diffs = append(diffs, *list)
	}
	if len(diffs) > 0 {
		return Result{Verdict: Fail, Diffs: diffs}
	}
	return Result{Verdict: Pass}
}
