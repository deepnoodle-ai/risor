package risor_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/deepnoodle-ai/risor/v3/internal/runner"
)

// report fails the test for each fail and invalid verdict, and logs the
// count of each verdict; with -v also by file and the reasons for skip.
func report(t *testing.T, label string, all []runner.Result) {
	t.Helper()
	byFile := map[string][]runner.Result{}
	reasons := map[string]int{}
	for _, r := range all {
		switch r.Verdict {
		case runner.Fail, runner.Invalid:
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
		filepath.Join("..", "suite", "host-types")} // package F1, host types
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
		var all []runner.Result
		// suite/core-check is run with the checker only (3.10.13).
		runner.CheckedOnly = filepath.Base(root) == "core-check" || filepath.Base(root) == "host-types"
		for _, f := range files {
			all = append(all, runner.RunFile(f)...)
		}
		runner.CheckedOnly = false
		report(t, root, all)
	}
}

// TestCatalogCases runs the cases of suite/library.json and
// suite/messages.json.
func TestCatalogCases(t *testing.T) {
	for _, f := range []struct{ path, key string }{
		{filepath.Join("..", "suite", "library.json"), "name"},
		{filepath.Join("..", "suite", "messages.json"), "code"},
	} {
		if _, err := os.Stat(f.path); err != nil {
			t.Logf("%s: not there yet", f.path)
			continue
		}
		report(t, f.path, runner.RunEntries(f.path, f.key))
	}
}
