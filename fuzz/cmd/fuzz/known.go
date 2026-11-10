package main

import (
	"regexp"
	"strings"

	"github.com/deepnoodle-ai/risor/fuzz/internal/gen"
)

// Known differences are counted apart, by name, so that anything new stands
// out. With -known=false every difference is a finding.
//
// The only known difference left is one the specification makes on purpose:
// the implementations offer different packages (03-host.md 3.11), and a
// package gives a text that Core rejects a meaning of its own, or a parse
// code of its own at the same span. The outcome command has no way to turn
// a package off, so a case that holds such a text is run with the package by
// the implementations that offer it and without it by the others.
//
//	package-parse  those without the package agree on a parse failure with
//	               a code that Core reports; those with it fail to parse
//	               with a targeted code no Core implementation reports (F8),
//	               at the same span or at an earlier token the package
//	               looks at
//	package-form   those without the package agree on a parse failure, and
//	               the text holds the projection [* (F7)
//	package-sortBy those without F7 report arity on a call of sortBy, which
//	               F7 gives a second parameter, the direction
//
// The checker differences of the earlier rounds (never-equal on a null
// subject or a repeated literal, the body of a lambda read only as a value,
// the defaults of a pattern whose value failed, cannot-infer for the init of
// reduce) were fixed in all three implementations and are no longer listed.
var knownNames = []string{"package-parse", "package-form", "package-sortBy"}

// coreParseCodes are the parse codes every implementation reports: those of
// Core (01-language.md 14.2), with bad-type-text, which F4 reports
// (f04-conversion.md 2.6). The texts with as and is that F4 reads are
// compared exactly, now that every implementation offers F4 and F5.
var coreParseCodes = map[string]bool{
	"unexpected-token": true, "unexpected-character": true,
	"unterminated-string": true, "unterminated-comment": true,
	"bad-escape": true, "line-continuation": true, "bad-number": true,
	"literal-range": true, "nesting-exceeded": true, "text-block-open": true,
	"text-block-close": true, "text-block-indent": true, "reserved-word": true,
	"bad-type-text": true,
}

// laterForm finds a form that only a later package reads: the projection
// [*] (F7).
var laterForm = regexp.MustCompile(`\[\s*\*`)

// sortByDirection finds a call of sortBy with a second argument, the
// direction that F7 adds; in Core it is arity.
var sortByDirection = regexp.MustCompile(`\.sortBy\(`)

// known names the known difference a disagreement is, or gives "".
func known(c *gen.Case, rs []res) string {
	if k := sortByKnown(c, rs); k != "" {
		return k
	}
	// Every implementation fails to parse, and the text holds a later form:
	// those with the package fail later, or with another code.
	allParse := true
	for _, r := range rs {
		if r.line == nil || r.line.Phase != "parse" {
			allParse = false
		}
	}
	if allParse && laterForm.MatchString(c.Text()) {
		return "package-form"
	}
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
	if laterForm.MatchString(c.Text()) {
		return "package-form"
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

// sortByKnown recognizes the difference of F7's direction of sortBy: every
// implementation fails the check; those without F7 report arity on a call of
// sortBy, and those with it do not.
func sortByKnown(c *gen.Case, rs []res) string {
	if !sortByDirection.MatchString(c.Text()) {
		return ""
	}
	arity, other := 0, 0
	for _, r := range rs {
		if r.line == nil || r.line.Phase != "check" {
			return ""
		}
		has := false
		for _, d := range r.line.Diags {
			if d["code"] == "arity" && sortByDirection.MatchString(spanText(c.Text(), d["span"])) {
				has = true
			}
		}
		if has {
			arity++
		} else {
			other++
		}
	}
	if arity > 0 && other > 0 {
		return "package-sortBy"
	}
	return ""
}

// spanText gives the text of a span [l1, c1, l2, c2]: lines end at U+000A,
// columns count code points (03-host.md 3.7.7).
func spanText(src string, span any) string {
	s, ok := span.([]any)
	if !ok || len(s) != 4 {
		return ""
	}
	n := make([]int, 4)
	for i := range s {
		f, _ := s[i].(float64)
		n[i] = int(f)
	}
	var out []rune
	line, col := 1, 1
	for _, r := range src {
		after := line > n[0] || line == n[0] && col >= n[1]
		before := line < n[2] || line == n[2] && col < n[3]
		if after && before {
			out = append(out, r)
		}
		if r == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return strings.TrimSpace(string(out))
}
