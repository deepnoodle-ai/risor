package main

import (
	"github.com/deepnoodle-ai/risor/fuzz/internal/gen"
	"github.com/deepnoodle-ai/risor/fuzz/internal/outcome"
)

// Known differences are counted apart, by name, so that anything new stands
// out. With -known=false every difference is a finding.
//
// The implementations may offer different packages (03-host.md 3.11), and
// the outcome command has no way to turn a package off. A package may give a
// text that an earlier one rejects a meaning or a parse code of its own, so
// a difference may come from the packages offered rather than from a fault.
// Every implementation offers the packages up to F17's step E1. A form of a
// package that only some implementations offer gets an entry here when the
// generator learns it, and the entry names the implementations that lack
// the package, so that the same answer from one that offers it stays a
// finding.
var knownNames = []string{"e2"}

// e2Lacking are the implementations that do not offer F17's step E2 yet:
// recursive type lines (spec/packages/f17-recursion.md 12). Remove a name
// when its implementation offers the step, and the entry when none is left.
var e2Lacking = map[string]bool{"ts": true}

// known names the known difference a disagreement is, or gives "". A case
// with a recursive type line is the known difference "e2" when no two
// implementations that offer step E2 disagree.
func known(cfg *config, c *gen.Case, rs []res) string {
	if !c.E2 {
		return ""
	}
	for i := range rs {
		for j := i + 1; j < len(rs); j++ {
			if e2Lacking[cfg.impls[i].Name] || e2Lacking[cfg.impls[j].Name] {
				continue
			}
			if rs[i].fail != "" || rs[j].fail != "" || outcome.Diff(rs[i].line, rs[j].line) != "" {
				return ""
			}
		}
	}
	return "e2"
}
