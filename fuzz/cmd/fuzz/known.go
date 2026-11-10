package main

import (
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
	"github.com/deepnoodle-ai/risor/fuzz/internal/gen"
)

// The known differences: five of the checkers, reported before and being
// fixed. A difference made only of these is counted apart, by name, so that
// anything new stands out. With -known=false every difference is a finding.
//
//	never-equal  a never-equal on a match whose subject is exactly null, or
//	             on a literal listed twice (types chapter 11.2)
//	lambda-body  diagnostics in the body of a const-bound lambda whose name
//	             is only read as a value (13.9, "No use")
//	default      diagnostics in a pattern's default when the value bound
//	             failed (12.2, 12.4)
//	reduce-init  cannot-infer in place of kind when the init of reduce is a
//	             lambda
var knownNames = []string{"never-equal", "lambda-body", "default", "reduce-init"}

type diagKey struct {
	code string
	span ast.Span
}

// known names the known differences a disagreement is made of, or gives ""
// when some part of it is not known.
func known(c *gen.Case, rs []res) string {
	for _, r := range rs {
		if r.line == nil {
			return ""
		}
	}
	var nodes []ast.NodeSpan
	if c.Prog != nil {
		_, nodes = ast.PrintSpans(c.Prog)
	}
	// A program that one checker rejects with never-equal alone, and another
	// accepts.
	phases := map[string]bool{}
	for _, r := range rs {
		phases[r.line.Phase] = true
	}
	if len(phases) > 1 {
		for _, r := range rs {
			if r.line.Phase != "check" {
				continue
			}
			for _, d := range r.line.Diags {
				if d["code"] != "never-equal" {
					return ""
				}
			}
		}
		return "never-equal"
	}
	if !phases["check"] {
		return ""
	}
	// The diagnostics that not every implementation gave.
	count := map[diagKey]int{}
	for _, r := range rs {
		seen := map[diagKey]bool{}
		for _, d := range r.line.Diags {
			k := key(d)
			if !seen[k] {
				seen[k] = true
				count[k]++
			}
		}
	}
	var odd []diagKey
	hasCannotInfer := false
	for k, n := range count {
		if n < len(rs) {
			odd = append(odd, k)
			if k.code == "cannot-infer" {
				hasCannotInfer = true
			}
		}
	}
	if len(odd) == 0 {
		return "" // the same set, in another order or count
	}
	names := map[string]bool{}
	for _, k := range odd {
		switch {
		case k.code == "never-equal":
			names["never-equal"] = true
		case hasCannotInfer && (k.code == "cannot-infer" || k.code == "kind" || k.code == "not-a-value"):
			names["reduce-init"] = true
		case inside(k.span, nodes, "ConstLambda"):
			names["lambda-body"] = true
		case inside(k.span, nodes, "Default"):
			names["default"] = true
		default:
			return ""
		}
	}
	var out []string
	for n := range names {
		out = append(out, n)
	}
	sort.Strings(out)
	return strings.Join(out, "+")
}

func key(d map[string]any) diagKey {
	c, _ := d["code"].(string)
	var s ast.Span
	if a, ok := d["span"].([]any); ok && len(a) == 4 {
		for i := range a {
			f, _ := a[i].(float64)
			s[i] = int(f)
		}
	}
	return diagKey{c, s}
}

func inside(s ast.Span, nodes []ast.NodeSpan, kind string) bool {
	for _, n := range nodes {
		if n.Kind == kind && s.Within(n.Span) {
			return true
		}
	}
	return false
}
