package main

import (
	"github.com/deepnoodle-ai/risor/fuzz/internal/gen"
)

// Known differences are counted apart, by name, so that anything new stands
// out. With -known=false every difference is a finding.
//
// The only known difference left is one the specification makes on purpose:
// the implementations offer different packages (03-host.md 3.11), and the
// outcome command has no way to turn a package off. Every implementation
// offers F1 to F7; package F8 and later are offered by some only. F8 gives
// some texts that Core rejects a targeted parse code of its own:
//
//	package-parse  those without the package agree on a parse failure with
//	               a code that every implementation reports; those with it
//	               fail to parse with a targeted code (F8), at the same span
//	               or at an earlier token the package looks at
//
// The checker differences of earlier rounds were fixed in all three
// implementations, and the forms of F4, F5, F6 and F7 are compared exactly.
var knownNames = []string{"package-parse"}

// coreParseCodes are the parse codes every implementation reports: those of
// Core (01-language.md 14.2), with bad-type-text, which F4 reports
// (f04-conversion.md 2.6).
var coreParseCodes = map[string]bool{
	"unexpected-token": true, "unexpected-character": true,
	"unterminated-string": true, "unterminated-comment": true,
	"bad-escape": true, "line-continuation": true, "bad-number": true,
	"literal-range": true, "nesting-exceeded": true, "text-block-open": true,
	"text-block-close": true, "text-block-indent": true, "reserved-word": true,
	"bad-type-text": true,
}

// known names the known difference a disagreement is, or gives "".
func known(c *gen.Case, rs []res) string {
	var core, pkg []res
	for _, r := range rs {
		if r.line == nil {
			return ""
		}
		if r.line.Phase == "parse" && coreParseCodes[r.line.Code()] {
			core = append(core, r)
		} else {
			pkg = append(pkg, r)
		}
	}
	if len(core) == 0 || len(pkg) == 0 {
		return ""
	}
	// Those without the package must agree with one another.
	for _, r := range core[1:] {
		if r.line.Raw != "" && !sameDiag(r.line.Diags, core[0].line.Diags) {
			return ""
		}
	}
	targeted := true
	for _, r := range pkg {
		if r.line.Phase != "parse" || coreParseCodes[r.line.Code()] {
			targeted = false
		}
	}
	if targeted {
		return "package-parse"
	}
	return ""
}

func sameDiag(a, b []map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i]["code"] != b[i]["code"] || !sameValue(a[i]["span"], b[i]["span"]) {
			return false
		}
	}
	return true
}

func sameValue(a, b any) bool {
	x, ok1 := a.([]any)
	y, ok2 := b.([]any)
	if !ok1 || !ok2 || len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
