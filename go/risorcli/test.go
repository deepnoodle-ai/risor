package risorcli

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/go/v3/internal/runner"
)

// CaseResult is the outcome of one test case for risor test.
type CaseResult struct {
	File    string
	Case    string
	Status  string // "pass", "fail", "skip", "invalid", or "updated"
	Diffs   []string
	Fixture []any
	Message string
	Reason  string
}

// CaseFileOutcome is the outcome of running one case file for risor test.
type CaseFileOutcome struct {
	Results     []CaseResult
	Report      string
	UpdatedText []byte
	Written     bool
	UpdateErr   error
}

// SuiteSection is the outcome of one directory or catalog in risor test --suite.
type SuiteSection struct {
	Rel     string
	Results []CaseResult
	Err     error
}

// RunCaseFile parses and runs one case file (program case file when path ends
// in .test.risor, else JSON). An invalid file returns a non-nil error whose
// text is the reason.
func RunCaseFile(path string, data []byte, update bool, selected func(string) bool) (CaseFileOutcome, error) {
	opts := casefile.Options{Update: update}
	lf := &runner.LoadedFile{Text: data}
	var err error
	if strings.HasSuffix(path, ".test.risor") {
		lf.Program = true
		opts.Update = false
		lf.File, err = runner.LoadProgram(data, path, opts)
	} else {
		lf.File, err = casefile.ParseWith(data, path, opts)
	}
	if err != nil {
		var inv *casefile.InvalidError
		if errors.As(err, &inv) {
			return CaseFileOutcome{}, errors.New(inv.Reason)
		}
		return CaseFileOutcome{}, err
	}
	rs := runner.RunLoaded(&runner.Ctx{Update: update}, lf, selected)
	out := CaseFileOutcome{
		Results: convertResults(rs),
		Report:  runner.Report(rs),
	}
	if update && path != "-" {
		out.UpdatedText, out.Written, out.UpdateErr = runner.UpdatedText(lf, rs)
	}
	return out, nil
}

// RunSuite runs every conformance-suite directory and catalog under dir,
// sorted by relative path (F9 8.6).
func RunSuite(dir string) []SuiteSection {
	type entry struct {
		rel string
		run func() ([]runner.Result, error)
	}
	var es []entry
	for _, d := range runner.SuiteDirs {
		p := filepath.Join(dir, d)
		es = append(es, entry{d, func() ([]runner.Result, error) { return runner.RunSuiteDir(p) }})
	}
	for _, c := range runner.SuiteCatalogs {
		p, key := filepath.Join(dir, c.Rel), c.Key
		es = append(es, entry{c.Rel, func() ([]runner.Result, error) { return runner.RunSuiteCatalog(p, key) }})
	}
	sort.Slice(es, func(i, j int) bool { return es[i].rel < es[j].rel })
	sections := make([]SuiteSection, len(es))
	for i, e := range es {
		rs, err := e.run()
		sections[i] = SuiteSection{Rel: e.rel, Results: convertResults(rs), Err: err}
	}
	return sections
}

func convertResults(rs []runner.Result) []CaseResult {
	out := make([]CaseResult, len(rs))
	for i, r := range rs {
		cr := CaseResult{
			File:    r.File,
			Case:    r.Case,
			Status:  r.Status(),
			Fixture: r.Fixture,
			Message: r.Message,
			Reason:  r.Reason,
		}
		if len(r.Diffs) > 0 {
			cr.Diffs = make([]string, len(r.Diffs))
			for j, d := range r.Diffs {
				cr.Diffs[j] = d.Name
			}
		}
		out[i] = cr
	}
	return out
}
