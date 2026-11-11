package risor_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/deepnoodle-ai/risor/go/v3/internal/runner"
)

// disputed lists the cases this implementation believes wrong, each with
// its question (questions/go.md). A disputed case that fails is logged, not
// counted as a failure; remove an entry when the case is ruled on.
var disputed = map[string]string{}

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
	var paths []string
	for _, d := range runner.SuiteDirs {
		paths = append(paths, filepath.Join("..", "suite", d))
	}
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
		for _, f := range files {
			if pkg, ok := needsUnoffered[filepath.Base(f)]; ok {
				t.Logf("%s: not run, it needs package %s, which this implementation does not offer yet", filepath.Base(f), pkg)
			}
		}
		all, err := runner.RunSuiteDir(root)
		if err != nil {
			t.Fatal(err)
		}
		report(t, root, all)
	}
}

// TestCatalogCases runs the cases of suite/library.json and
// suite/messages.json.
func TestCatalogCases(t *testing.T) {
	for _, c := range runner.SuiteCatalogs {
		path := filepath.Join("..", "suite", c.Rel)
		rs, err := runner.RunSuiteCatalog(path, c.Key)
		if err != nil {
			t.Logf("%s: not there yet", path)
			continue
		}
		report(t, path, rs)
	}
}
