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
// Every implementation offers F1 to F8. Package F17 (guards on match arms
// and walk) is offered by Go alone so far: a case that writes one of its
// forms, where an implementation answers as one without F17 does, is
// counted apart. -nof17 leaves the forms out.
var knownNames = []string{
	"F17 is not offered everywhere: a guard or a call of walk",
}

// known names the known difference a disagreement is, or gives "".
func known(c *gen.Case, rs []res) string {
	if !c.F17 {
		return ""
	}
	for _, r := range rs {
		if r.line != nil && withoutF17(r.line) {
			return knownNames[0]
		}
	}
	return ""
}

// withoutF17 reports whether a line is what an implementation without F17
// gives for one of its forms: a guard is unexpected-token at its if
// (01-language.md 14.3), and walk is a function the environment does not
// have.
func withoutF17(l *outcome.Line) bool {
	for _, d := range l.Diags {
		switch {
		case l.Phase == "parse" && d["code"] == "unexpected-token" && d["message"] == "unexpected if":
			return true
		case l.Phase == "check" && d["code"] == "undefined-function" && d["message"] == "no function is named walk":
			return true
		}
	}
	return false
}
