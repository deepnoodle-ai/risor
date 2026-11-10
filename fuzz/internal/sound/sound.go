// Package sound judges the outcome of a checked program against the promise
// of spec/core/05-types.md 18.4, from the outcome line alone: the type and
// sites the check gave, and the value or failure of the run.
package sound

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
	"github.com/deepnoodle-ai/risor/fuzz/internal/outcome"
	"github.com/deepnoodle-ai/risor/fuzz/internal/types"
)

// Verdict is the judgement of one outcome line.
type Verdict struct {
	Clean     bool   // the check reported no diagnostic
	Outcome   string // "value", or the code of the run failure
	Violation string // "" when the promise holds; else what broke
	Promise   int    // the promise broken, 1 to 4; 0 for a class that disagrees
	Undecided string // why the fuzzer could not decide, if it could not
}

// siteClass gives, for each run code, the class of site a failure with
// that code must be at (18.4, 18.5): "" for a code that may occur anywhere
// (promise 3).
var siteClass = map[string]string{
	"null-operand":     "dynamic",
	"wrong-kind":       "dynamic",
	"missing-field":    "dynamic",
	"index-range":      "index",
	"divide-by-zero":   "divide",
	"invalid-argument": "value",
	"failed":           "value",
	"function-error":   "host",
	// f04-conversion.md 6.6: a failed as is at its value site.
	"not-conforming": "value",
}

var anywhere = map[string]bool{
	"number-range": true, "steps-exceeded": true, "allocation-exceeded": true,
	"depth-exceeded": true, "deadline-exceeded": true, "cancelled": true,
	"function-panic": true,
}

var promiseOf = map[string]int{
	"dynamic": 1, "index": 2, "divide": 2, "value": 2, "host": 2,
}

// Judge decides whether one line keeps the promise. nodes are the spans of
// the program's nodes, from ast.PrintSpans.
func Judge(l *outcome.Line, nodes []ast.NodeSpan) Verdict {
	if l.Phase == "check" || l.Phase == "parse" || l.Phase == "declare" || !l.HasType {
		return Verdict{}
	}
	v := Verdict{Clean: true}
	if l.Phase == "" {
		v.Outcome = "value"
		t, err := types.Parse(l.Type)
		if err != nil {
			v.Undecided = "type text: " + err.Error()
			return v
		}
		if !types.Conforms(l.Value, t) {
			v.Promise = 4
			v.Violation = fmt.Sprintf("promise 4: the value does not conform to %s", l.Type)
		}
		return v
	}
	code := l.Code()
	v.Outcome = code
	if anywhere[code] {
		return v
	}
	want, ok := siteClass[code]
	if !ok {
		v.Promise = 1
		v.Violation = fmt.Sprintf("a checked program with conforming values failed with %s", code)
		return v
	}
	span, ok := toSpan(l.Diags[0]["span"])
	if !ok {
		v.Undecided = "the failure has no span"
		return v
	}
	at, matched := AtSite(span, want, l.Sites, nodes)
	if !matched {
		v.Undecided = fmt.Sprintf("no node of the program has the span %v", span)
		return v
	}
	if !at {
		v.Promise = promiseOf[want]
		v.Violation = fmt.Sprintf("promise %d: %s at %v is not at a %s site", v.Promise, code, span, want)
		return v
	}
	if class, _ := l.Diags[0]["class"].(string); class != want {
		v.Violation = fmt.Sprintf("%s at a %s site has the class %q", code, want, class)
	}
	return v
}

// AtSite says whether a failure with the span f is at a site of the class
// want: a site on the failing node, or on its operand, receiver, index or
// argument; for a pattern, on its key or name or the value bound (18.3).
// It also reports whether any node of the program has the span f.
func AtSite(f ast.Span, want string, sites []any, nodes []ast.NodeSpan) (at, matched bool) {
	cands := map[ast.Span]bool{}
	for _, n := range nodes {
		if n.Span == f {
			matched = true
			cands[f] = true
			for _, c := range n.Children {
				cands[c] = true
			}
		}
	}
	for _, s := range sites {
		m, _ := s.(map[string]any)
		if m == nil || m["class"] != want {
			continue
		}
		if sp, ok := toSpan(m["span"]); ok && cands[sp] {
			return true, matched
		}
	}
	return false, matched
}

func toSpan(v any) (ast.Span, bool) {
	a, ok := v.([]any)
	if !ok || len(a) != 4 {
		return ast.Span{}, false
	}
	var s ast.Span
	for i := range a {
		f, ok := a[i].(float64)
		if !ok {
			return ast.Span{}, false
		}
		s[i] = int(f)
	}
	return s, true
}
