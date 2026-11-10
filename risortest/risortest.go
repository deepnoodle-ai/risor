// Package risortest runs Risor case files inside Go's own test framework
// (package F9 7 of the specification). A case file declares an
// environment, stubs host functions with tables and lists cases; Run gives
// each case a subtest:
//
//	func TestDiscount(t *testing.T) {
//		risortest.Run(t, env, "testdata/discount.test.json")
//	}
//
// With RISOR_UPDATE=1 in the environment, Run writes each case's actual
// outcome into its expect (update mode).
package risortest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/runner"
)

// Option changes how a file is loaded and its cases run.
type Option func(*options)

type options struct {
	vars    risor.Vars
	read    func(ref string) (string, risor.Origin, error)
	base    *risor.Env
	update  bool
	filter  string
	setUpd  bool
	program bool
}

// Vars gives variables for the names a case does not supply. They are not
// fixtures, and are not tested.
func Vars(v risor.Vars) Option { return func(o *options) { o.vars = v } }

// Sources gives the reader of a case's file: it returns the text and its
// origin. Spans are positions in the text it returns.
func Sources(read func(ref string) (string, risor.Origin, error)) Option {
	return func(o *options) { o.read = read }
}

// Base gives the base environment to Load and Parse (F9 7.1, 7.2): the
// rule of a stub with no type (03-host.md 3.10.10 item 11) is checked
// against it at the load, and every case of the file runs in a child of
// it. Without Base the base is the default environment. Run sets it from
// its own argument.
func Base(env *risor.Env) Option { return func(o *options) { o.base = env } }

// Update turns update mode on or off, whatever RISOR_UPDATE says.
func Update(on bool) Option { return func(o *options) { o.update, o.setUpd = on, true } }

// Filter runs only the cases whose names contain text, code point for
// code point.
func Filter(text string) Option { return func(o *options) { o.filter = text } }

func collect(opts []Option) *options {
	o := &options{}
	for _, f := range opts {
		f(o)
	}
	if !o.setUpd {
		o.update = os.Getenv("RISOR_UPDATE") == "1"
	}
	return o
}

// File is a valid case file.
type File struct {
	path  string // the path it was loaded from, or empty
	lf    *runner.LoadedFile
	opts  *options
	cases []*Case
}

// Case is one case of a file.
type Case struct {
	f *File
	c *casefile.Case
}

// Diff is one difference of a case that failed: its name, and E and F of
// the report, each in its JSON form.
type Diff struct {
	Name, Expected, Found string
}

// Verdict is what running a case gave.
type Verdict struct {
	// Name is the case's name.
	Name string
	// Status is pass, fail, skip, invalid or, in update mode, updated.
	Status string
	// Diffs are the differences of a case that failed, in order.
	Diffs []Diff
	// Fixture and Message say which fixture failed, for invalid.
	Fixture []any
	Message string
	// Reason is the reason of a skipped case.
	Reason string
	// Expect is the expect update mode writes, in its JSON form; empty
	// unless Status is updated.
	Expect string
	r      runner.Result
}

// Report is the lines of the report for the case: none when it passed.
func (v Verdict) Report() string { return runner.CaseReport(v.r) }

func (o *options) caseOptions() casefile.Options {
	co := casefile.Options{Update: o.update}
	if o.base != nil {
		co.BaseVar, co.BaseFunc = runnerBaseNames(o.base)
	}
	if o.read != nil {
		co.Read = func(ref string) (string, error) {
			text, _, err := o.read(ref)
			return text, err
		}
	}
	return co
}

// Load reads a case file from a path: a program case file when its name
// ends in .test.risor, else JSON. An invalid file is an error that holds
// the reason, and no case is run.
func Load(path string, opts ...Option) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	o := collect(opts)
	o.program = strings.HasSuffix(path, ".test.risor")
	f, err := load(data, path, o)
	if err != nil {
		return nil, err
	}
	f.path = path
	return f, nil
}

// Parse reads a case file from its text: a program case file when program
// is true, else JSON. A file named in it is read relative to the current
// directory, or by the reader of Sources.
func Parse(text []byte, program bool, opts ...Option) (*File, error) {
	o := collect(opts)
	o.program = program
	return load(text, "case-file.json", o)
}

func load(data []byte, path string, o *options) (*File, error) {
	lf := &runner.LoadedFile{Text: data, Program: o.program}
	var err error
	if o.program {
		lf.File, err = runner.LoadProgram(data, path, o.caseOptions())
	} else {
		lf.File, err = casefile.ParseWith(data, path, o.caseOptions())
	}
	if err != nil {
		var inv *casefile.InvalidError
		if errors.As(err, &inv) {
			return nil, errors.New(inv.Reason)
		}
		return nil, err
	}
	f := &File{lf: lf, opts: o}
	for _, c := range lf.File.Cases {
		if o.filter != "" && !strings.Contains(c.Name, o.filter) {
			continue
		}
		f.cases = append(f.cases, &Case{f: f, c: c})
	}
	return f, nil
}

// Cases are the file's cases, in order; with Filter, those it selects.
func (f *File) Cases() []*Case { return f.cases }

// Name is the case's name.
func (c *Case) Name() string { return c.c.Name }

// Run runs the case in a child of the base its file was loaded with: the
// base of the option Base, or the default environment when none was given
// (F9 7.1, 7.2).
func (c *Case) Run(ctx context.Context) Verdict {
	o := c.f.opts
	rc := &runner.Ctx{Base: o.base, Vars: o.vars, Update: o.update && !c.f.lf.Program, Context: ctx}
	r := runner.RunCaseWith(rc, c.c)
	v := Verdict{Name: r.Case, Status: r.Status(), Fixture: r.Fixture, Message: r.Message, r: r}
	switch {
	case r.Updated:
		v.Expect = r.NewExpect
	case r.Verdict == runner.Skip:
		v.Reason = r.Reason
	case r.Verdict == runner.Fail:
		for _, d := range r.Diffs {
			v.Diffs = append(v.Diffs, Diff{Name: d.Name, Expected: d.Expected, Found: d.Found})
		}
	}
	return v
}

// Update gives the file's text with the expect of each updated verdict
// written in, and every other byte kept. It gives nil when no verdict is
// updated, or the file is a program case file, which is never written. It
// also gives nil unless the verdicts are those of every case Cases gives:
// the file is written once, when every selected case has run (F9 4.2 item
// 4), so a run of only some of them leaves it as it was.
func (f *File) Update(verdicts []Verdict) ([]byte, error) {
	if !f.allRan(verdicts) {
		return nil, nil
	}
	rs := make([]runner.Result, len(verdicts))
	for i, v := range verdicts {
		rs[i] = v.r
	}
	text, written, err := runner.UpdatedText(f.lf, rs)
	if err != nil || !written {
		return nil, err
	}
	return text, nil
}

// allRan reports whether the verdicts are one for each selected case, run.
func (f *File) allRan(verdicts []Verdict) bool {
	if len(verdicts) != len(f.cases) {
		return false
	}
	ran := map[string]int{}
	for _, v := range verdicts {
		ran[v.Name]++
	}
	for _, c := range f.cases {
		if ran[c.c.Name] == 0 {
			return false
		}
		ran[c.c.Name]--
	}
	return true
}

// Run loads the case file at path and runs each case as a subtest named by
// the case, in a child of base; a nil base is the default environment. A
// skipped case skips its subtest; a case that fails or is invalid fails it
// with the lines of its report. An invalid file fails t. With RISOR_UPDATE=1
// (or the option Update), the file is written after the subtests end, and
// only when every case selected by the option Filter has run: when go test
// -run runs only some of them, the file is left as it was (F9 4.2 item 4).
func Run(t *testing.T, base *risor.Env, path string, opts ...Option) {
	t.Helper()
	f, err := Load(path, append(opts, Base(base))...)
	if err != nil {
		t.Fatalf("%s: invalid: %v", filepath.ToSlash(path), err)
	}
	var verdicts []Verdict
	for _, c := range f.Cases() {
		t.Run(c.Name(), func(t *testing.T) {
			v := c.Run(context.Background())
			verdicts = append(verdicts, v)
			switch v.Status {
			case "skip":
				t.Skip(v.Reason)
			case "fail", "invalid":
				t.Error("\n" + v.Report())
			}
		})
	}
	if !f.opts.update {
		return
	}
	text, err := f.Update(verdicts)
	if err != nil {
		t.Fatalf("%s: update: %v", path, err)
	}
	if text != nil {
		if err := os.WriteFile(path, text, 0o644); err != nil {
			t.Fatalf("%s: update: %v", path, err)
		}
	}
}
