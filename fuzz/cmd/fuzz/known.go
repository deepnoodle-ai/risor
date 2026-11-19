package main

import "github.com/deepnoodle-ai/risor/fuzz/internal/gen"

// Known differences are counted apart, by name, so that anything new stands
// out. With -known=false every difference is a finding.
//
// The implementations may offer different packages (03-host.md 3.11), and
// the outcome command has no way to turn a package off. A package may give a
// text that an earlier one rejects a meaning or a parse code of its own, so
// a difference may come from the packages offered rather than from a fault.
// Every implementation now offers F1 to F8, and the generator writes only
// the forms of those packages, so no difference is known: every one is a
// finding. A form of a package that only some implementations offer gets an
// entry here when the generator learns it.
var knownNames []string

// known names the known difference a disagreement is, or gives "".
func known(c *gen.Case, rs []res) string {
	return ""
}
