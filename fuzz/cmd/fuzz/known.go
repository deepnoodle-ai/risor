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
// Every implementation offers the packages up to F17's step E0. Step E1 of
// F17, typed self-recursion, is built in Go alone so far: a case that writes
// a lambda that refers to its own name, where an implementation answers
// recursion-needs-types, is counted apart. That covers the new text of its
// message too. -noe1 leaves the lambdas out.
var knownNames = []string{
	"F17 step E1 is not offered everywhere: a lambda that refers to its own name",
}

// known names the known difference a disagreement is, or gives "".
func known(c *gen.Case, rs []res) string {
	if !c.E1 {
		return ""
	}
	for _, r := range rs {
		if r.line != nil && selfRefRefused(r.line) {
			return knownNames[0]
		}
	}
	return ""
}

// selfRefRefused reports whether a line has recursion-needs-types, as an
// implementation without step E1 gives for every self-reference.
func selfRefRefused(l *outcome.Line) bool {
	if l.Phase != "check" {
		return false
	}
	for _, d := range l.Diags {
		if d["code"] == "recursion-needs-types" {
			return true
		}
	}
	return false
}
