// Package runner runs case files (spec/core/03-host.md 3.10) against the
// implementation and gives each case a verdict.
//
// This implementation has no type checker yet. By 3.10.8 item 5 and ruling
// A3 it gives skip to a case that states type or sites, or that expects a
// check code outside the static list, and it does not compare class.
package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
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
	out := make([]Result, 0, len(f.Cases))
	for _, c := range f.Cases {
		r := RunCase(c)
		r.File = path
		out = append(out, r)
	}
	return out
}

// Files returns the case files a path names: the path itself if it is a
// file, else every .json file under the directory, in order.
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
		if !d.IsDir() && strings.HasSuffix(p, ".json") {
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

func runCase(c *casefile.Case) Result {
	if c.Skip != nil {
		return skip(*c.Skip)
	}
	e := c.Expect
	if e.Type != nil || e.HasSites {
		return skip("needs the type checker")
	}
	// What S1 cannot judge yet: a later stage decides the outcome. A parse
	// failure is still judged, since the outcome is then known.
	later := ""
	switch e.Phase {
	case "declare":
		return skip("S1: declare is not built yet")
	case "check":
		later = "S1: the static rules are not built yet"
	case "run":
		later = "S1: the evaluator is not built yet"
	case "":
		if e.HasValue || e.Truthy != nil || e.Steps != nil || e.Allocated != nil || c.Vars != nil {
			later = "S1: the evaluator is not built yet"
		} else if c.Functions != nil || c.Opaque != nil || c.Methods != nil || c.HasReject ||
			(c.Version != "" && c.Version != "3.0") {
			return skip("S1: declare is not built yet")
		}
	}
	// Step 2, declare: S1 checks only the grammar of the type text (A2).
	if d := declareGrammar(c); d != nil {
		return compare(c, d)
	}
	// Step 3, parse.
	_, err := risor.Parse(c.Source)
	if err != nil {
		return compare(c, err.(*risor.Error))
	}
	if later != "" {
		return skip(later)
	}
	if e.Phase == "" {
		// The program parses. The static rules of S2 would run here.
		return Result{Verdict: Pass, Reason: "parsed; static rules not run (S1)"}
	}
	return compare(c, nil)
}

// declareGrammar checks the grammar of the case's type text (ruling A2).
func declareGrammar(c *casefile.Case) *risor.Error {
	var ds []risor.Diagnostic
	bad := func(code, msg string) {
		ds = append(ds, risor.Diagnostic{Phase: diag.PhaseDeclare, Code: code, Message: msg})
	}
	if c.Env != nil {
		if ty, err := typetext.ParseType(*c.Env); err != nil {
			bad(diag.CodeBadType, err.Error())
		} else if _, ok := ty.RecordFields(); !ok {
			bad(diag.CodeBadType, "env is not one record type")
		}
	}
	if c.Returns != nil && *c.Returns != "condition" {
		if _, err := typetext.ParseType(*c.Returns); err != nil {
			bad(diag.CodeBadType, err.Error())
		}
	}
	names := make([]string, 0, len(c.Functions))
	for n := range c.Functions {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if _, err := typetext.ParseSig(*c.Functions[n].Type); err != nil {
			bad(diag.CodeBadSignature, n+": "+err.Error())
		}
	}
	for k, m := range c.Methods {
		if _, err := typetext.ParseSig(m); err != nil {
			bad(diag.CodeBadSignature, k+": "+err.Error())
		}
	}
	if len(ds) == 0 {
		return nil
	}
	return &risor.Error{Diagnostics: ds}
}

// compare compares the outcome with expect (3.10.7). err is the failure, or
// nil for a result.
func compare(c *casefile.Case, err *risor.Error) Result {
	e := c.Expect
	if err == nil {
		if e.IsFailure() {
			return Result{Verdict: Fail, Diffs: []Diff{{"outcome", "a failure of phase " + e.Phase, "a result"}}}
		}
		return Result{Verdict: Pass}
	}
	ph := string(err.Phase())
	if !e.IsFailure() || e.Phase != ph {
		want := "a result"
		if e.IsFailure() {
			want = "a failure of phase " + e.Phase
		}
		return Result{Verdict: Fail, Diffs: []Diff{{"outcome", want, "a failure of phase " + ph + ": " + err.Error()}}}
	}
	if d := compareDiagnostics(e, err.Diagnostics); d != nil {
		return Result{Verdict: Fail, Diffs: []Diff{*d}}
	}
	return Result{Verdict: Pass}
}

func formatDiags(ds []risor.Diagnostic) string {
	parts := make([]string, len(ds))
	for i, d := range ds {
		parts[i] = d.Code
		if d.Span != nil {
			parts[i] += " " + d.Span.String()
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
		if d.Message != nil {
			parts[i] += fmt.Sprintf(" %q", *d.Message)
		}
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

func diagMatches(want *casefile.ExpDiag, got risor.Diagnostic) bool {
	if want.Code != got.Code {
		return false
	}
	// Class is compared only after a check (3.10.7 item 8); there is none.
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
	// data is compared with the language's == once values exist (S2).
	return true
}

func compareDiagnostics(e *casefile.Expect, got []risor.Diagnostic) *Diff {
	d := &Diff{Name: "diagnostics", Expected: formatExp(e.Diagnostics), Found: formatDiags(got)}
	switch e.Phase {
	case "parse": // compared with the start of what the parser reported
		if len(got) < len(e.Diagnostics) {
			return d
		}
		for i, w := range e.Diagnostics {
			if !diagMatches(w, got[i]) {
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
				if !used[j] && diagMatches(w, g) {
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
			if !diagMatches(w, got[i]) {
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
