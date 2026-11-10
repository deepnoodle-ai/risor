package risor

import (
	"fmt"
	"sort"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/eval"
	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/static"
)

// This file is package F6, opt-in groups (spec/packages/f06-groups.md).

// Groups turns opt-in groups on (F6 2.1): json, time, math and base64. A
// group that is on is a namespace of library functions and constants. A
// child has its parent's groups and those it declares; it cannot turn one
// off. Any other name is the declare error bad-name.
func Groups(names ...string) Decl {
	return declFunc(func(e *Env) {
		for _, g := range names {
			if !library.Groups[g] {
				e.errf(diag.CodeBadName, fmt.Sprintf("%q is not an opt-in group", g))
				continue
			}
			if e.groups == nil {
				e.groups = map[string]bool{}
			}
			e.groups[g] = true
		}
	})
}

// groupNames adds the groups that are on to what the static rules read
// (F6 2.5): each a namespace, its entries library functions by full name,
// and its constants. A removed entry or constant is absent.
func (e *Env) groupNames(names *static.Names) {
	if len(e.groups) == 0 {
		return
	}
	for g := range e.groups {
		names.Namespaces[g] = true
	}
	for n, en := range library.GroupFunctions {
		if e.groups[library.Namespace(n)] && !e.removed[n] {
			names.Library[n] = en
		}
	}
	for n, c := range library.Constants {
		if e.groups[library.Namespace(n)] && !e.removed[n] {
			if names.Constants == nil {
				names.Constants = map[string]float64{}
			}
			names.Constants[n] = *c.Value
		}
	}
}

// groupList is the groups that are on, in code point order.
func (e *Env) groupList() []string {
	out := make([]string, 0, len(e.groups))
	for g := range e.groups {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// JSONText is the JSON text of a value a run returned (03-host.md 3.2.4):
// the JSON form of 01-language.md 12, keys in code point order. It fails
// with an *Error of phase run, wrong-kind on an opaque value, number-range
// on a host number that cannot enter the language, or depth-exceeded past
// the default depth limit, with no span. For the same value it gives the
// text json.stringify gives.
func JSONText(v any) (string, error) {
	text, d := eval.JSONText(v)
	if d != nil {
		return "", &Error{Diagnostics: []Diagnostic{*d}}
	}
	return text, nil
}
