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
// generator learns it, and the entry names the implementations that lack the
// package, so that the same answer from one that offers it stays a finding.
//
//	e3-ts-rust  a program with a parameter of a function type (F17 step E3,
//	            f17-recursion.md 13). Go offers the step; TypeScript and
//	            Rust do not yet, and keep Core's rules for such a parameter
//	            (call-of-parameter, lambda-to-lambda, a value). The
//	            difference is known when the implementations that lack the
//	            step agree with one another and those that offer it agree
//	            with one another. Remove the entry when TypeScript and Rust
//	            offer the step; -noe3 leaves the forms out.
var knownNames = []string{"e3-ts-rust"}

// lacksE3 names the implementations that do not offer F17's step E3 yet.
var lacksE3 = map[string]bool{"ts": true, "rust": true}

// known names the known difference a disagreement is, or gives "".
func known(cfg *config, c *gen.Case, rs []res) string {
	if !c.E3 {
		return ""
	}
	for i := range rs {
		for j := i + 1; j < len(rs); j++ {
			if lacksE3[cfg.impls[i].Name] != lacksE3[cfg.impls[j].Name] {
				continue
			}
			if rs[i].fail != "" || rs[j].fail != "" || outcome.Diff(rs[i].line, rs[j].line) != "" {
				return ""
			}
		}
	}
	return "e3-ts-rust"
}
