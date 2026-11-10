package runner

import (
	"os"
	"path/filepath"

	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
)

// The paths of the conformance suite this implementation runs (03-host.md
// 3.10.13 and 3.11; package F9 8.6), and how each is run.

// SuiteDirs are the directories of the suite, Core's and those of the
// packages offered.
var SuiteDirs = []string{
	"core-run", "core-check",
	"strings",    // F3
	"host-types", // F1
	"templates",  // F2
	"analysis",
	"conversion", // F4
	"unions",     // F5
	"groups",     // F6
	"queries",    // F7
	"errors",     // F8
	"testing",    // F9
	"config",     // F10
	"math",       // F13
}

// SuiteCatalog is a catalog file and the field that names its entries.
type SuiteCatalog struct{ Rel, Key string }

// SuiteCatalogs are the catalog files of the suite.
var SuiteCatalogs = []SuiteCatalog{
	{"library.json", "name"},
	{"library-groups.json", "name"},   // F6
	{"library-queries.json", "name"},  // F7
	{"library-math.json", "name"},     // F13
	{"messages-queries.json", "code"}, // F7
	{"messages-parse.json", "code"},   // F8
	{"messages-check.json", "code"},   // F8
	{"messages.json", "code"},
}

// RunSuiteDir runs the case files of one directory of the suite.
func RunSuiteDir(dir string) ([]Result, error) {
	files, err := Files(dir)
	if err != nil {
		return nil, err
	}
	b := filepath.Base(dir)
	// core-check, and host-types, run with the checker only (3.10.13).
	CheckedOnly = b == "core-check" || b == "host-types"
	if b == "conversion" || b == "unions" || b == "config" { // F4 6.1 item 4, F5 11.2 item 3, F10 10.3 item 3
		casefile.ExtraStatic = casefile.ConversionStatic
	}
	defer func() {
		CheckedOnly = false
		casefile.ExtraStatic = map[string]bool{}
	}()
	var all []Result
	for _, f := range files {
		all = append(all, RunFile(f)...)
	}
	return all, nil
}

// RunSuiteCatalog runs the cases of one catalog file.
func RunSuiteCatalog(path, key string) ([]Result, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	// Every case of messages-check.json runs checked only.
	CheckedOnly = filepath.Base(path) == "messages-check.json"
	defer func() { CheckedOnly = false }()
	return RunEntries(path, key), nil
}
