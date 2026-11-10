package risor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v3/internal/runner"
)

// TestSuite runs the case files of suite/core-run. Set RISOR_CASES to a list
// of case files or directories, separated by the path list separator, to run
// those instead. Run with -v to see the count of each verdict.
//
// A case with the verdict fail or invalid fails the test.
func TestSuite(t *testing.T) {
	paths := []string{filepath.Join("..", "suite", "core-run")}
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
		for _, f := range files {
			rs := runner.RunFile(f)
			for _, r := range rs {
				switch r.Verdict {
				case runner.Fail, runner.Invalid:
					t.Error(r.String())
				}
			}
			all = append(all, rs...)
		}
		t.Logf("%s: %d files, %s", root, len(files), runner.FormatCounts(runner.Counts(all)))
		if testing.Verbose() {
			reasons := map[string]int{}
			for _, r := range all {
				if r.Verdict == runner.Skip {
					reasons[r.Reason]++
				}
			}
			for reason, n := range reasons {
				t.Logf("  skip %d: %s", n, strings.TrimSpace(reason))
			}
		}
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
		rs := runner.RunEntries(f.path, f.key)
		for _, r := range rs {
			if r.Verdict == runner.Fail || r.Verdict == runner.Invalid {
				t.Error(r.String())
			}
		}
		t.Logf("%s: %s", f.path, runner.FormatCounts(runner.Counts(rs)))
	}
}
