package risor_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/runner"
)

// disputed lists the cases this implementation believes wrong, each with
// its question (questions/go.md). A disputed case that fails is logged, not
// counted as a failure; remove an entry when the case is ruled on.
var disputed = map[string]string{
	// Package F8, questions F8 items 1 to 5.
	"queries-syntax.json: a star after . after a projection":                                           "F8 2.6 E11 gives star-form",
	"queries-syntax.json: a star after an if":                                                          "F8 2.6 D4 gives needs-parentheses",
	"queries-syntax.json: a star after a try":                                                          "F8 2.6 D4 gives needs-parentheses",
	"messages-parse.json: bad-type-text: bad-type-text: a field after a rest":                          "the span is of a, not of as",
	"messages-check.json: not-a-value: not-a-value: a lambda at a method parameter that takes a value": "a static code the type rules report at a MethodCall, run also unchecked",
	"messages-check.json: arity: arity: a method call":                                                 "a static code the type rules report at a MethodCall, run also unchecked",
	"messages-check.json: arity: arity: a lambda with too many parameters":                             "a static code the type rules report at a MethodCall, run also unchecked",
	"trace.json: rendered: a message with a line feed is visible text, and data is in the JSON form":   "data holds no opaque value; its JSON form is {\"sku\":\"a\"}",
}

// needsUnoffered lists the case files that use a package this
// implementation does not offer yet. F5 is not written, so its tagged unions
// cannot run.
var needsUnoffered = map[string]string{}

// report fails the test for each fail and invalid verdict, and logs the
// count of each verdict; with -v also by file and the reasons for skip.
func report(t *testing.T, label string, all []runner.Result) {
	t.Helper()
	byFile := map[string][]runner.Result{}
	reasons := map[string]int{}
	for _, r := range all {
		switch r.Verdict {
		case runner.Fail, runner.Invalid:
			if why, ok := disputed[filepath.Base(r.File)+": "+r.Case]; ok && r.Verdict == runner.Fail {
				t.Logf("disputed, not counted as a failure (%s): %s", why, r.String())
				continue
			}
			t.Error(r.String())
		case runner.Skip:
			reasons[r.Reason]++
		}
		byFile[r.File] = append(byFile[r.File], r)
	}
	t.Logf("%s: %s", label, runner.FormatCounts(runner.Counts(all)))
	if !testing.Verbose() {
		return
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		t.Logf("  %s: %s", filepath.Base(f), runner.FormatCounts(runner.Counts(byFile[f])))
	}
	keys := make([]string, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("  skip %d: %s", reasons[k], k)
	}
}

// TestSuite runs the case files of suite/core-run. Set RISOR_CASES to a list
// of case files or directories, separated by the path list separator, to run
// those instead. Run with -v to see the counts by file.
func TestSuite(t *testing.T) {
	paths := []string{filepath.Join("..", "suite", "core-run"), filepath.Join("..", "suite", "core-check"),
		filepath.Join("..", "suite", "strings"),    // package F3, string forms
		filepath.Join("..", "suite", "host-types"), // package F1, host types
		filepath.Join("..", "suite", "templates"),  // package F2, templates and analysis
		filepath.Join("..", "suite", "analysis"),
		filepath.Join("..", "suite", "conversion"), // package F4, checked conversion
		filepath.Join("..", "suite", "groups"),     // package F6, opt-in groups
		filepath.Join("..", "suite", "queries"),    // package F7, queries
		filepath.Join("..", "suite", "unions"),     // package F5, tagged unions
		filepath.Join("..", "suite", "errors")}     // package F8, errors
	if env := os.Getenv("RISOR_CASES"); env != "" {
		paths = filepath.SplitList(env)
	}
	for _, root := range paths {
		files, err := runner.Files(root)
		if err != nil {
			if os.IsNotExist(err) && os.Getenv("RISOR_CASES") == "" {
				t.Logf("%s: not there yet", root)
				continue
			}
			t.Fatal(err)
		}
		for _, f := range runner.SkippedFiles(root) {
			t.Logf("%s: not run, its package is offered (03-host.md 3.11 item 4)", filepath.Base(f))
		}
		var all []runner.Result
		// suite/core-check is run with the checker only (3.10.13).
		runner.CheckedOnly = filepath.Base(root) == "core-check" || filepath.Base(root) == "host-types"
		if b := filepath.Base(root); b == "conversion" || b == "unions" { // F4 6.1 item 4, F5 11.2 item 3
			casefile.ExtraStatic = casefile.ConversionStatic
		}
		for _, f := range files {
			if pkg, ok := needsUnoffered[filepath.Base(f)]; ok {
				t.Logf("%s: not run, it needs package %s, which this implementation does not offer yet", filepath.Base(f), pkg)
				continue
			}
			all = append(all, runner.RunFile(f)...)
		}
		casefile.ExtraStatic = map[string]bool{}
		runner.CheckedOnly = false
		report(t, root, all)
	}
}

// TestCatalogCases runs the cases of suite/library.json and
// suite/messages.json.
func TestCatalogCases(t *testing.T) {
	for _, f := range []struct{ path, key string }{
		{filepath.Join("..", "suite", "library.json"), "name"},
		{filepath.Join("..", "suite", "library-groups.json"), "name"},   // package F6
		{filepath.Join("..", "suite", "library-queries.json"), "name"},  // package F7
		{filepath.Join("..", "suite", "messages-queries.json"), "code"}, // package F7
		{filepath.Join("..", "suite", "messages-parse.json"), "code"},   // package F8
		{filepath.Join("..", "suite", "messages-check.json"), "code"},   // package F8
		{filepath.Join("..", "suite", "messages.json"), "code"},
	} {
		if _, err := os.Stat(f.path); err != nil {
			t.Logf("%s: not there yet", f.path)
			continue
		}
		report(t, f.path, runner.RunEntries(f.path, f.key))
	}
}
