package library

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// This file is the lookup of package F19, feedback to authors
// (spec/packages/f19-author-feedback.md 3): a fix for four check messages,
// taken by exact text from the wrong lists of the library tables and from
// suite/foreign-names.json. It is a lookup in tables, never a search.

//go:generate go run ../gen ../../../suite/foreign-names.json foreign_gen.go library sourceForeignJSON

// Foreign is one row of suite/foreign-names.json (F19 3.3).
type Foreign struct {
	Written string   `json:"written"`
	Place   string   `json:"place"`
	Kinds   []string `json:"kinds"`
	Fix     string   `json:"fix"`
}

var foreign []Foreign

func init() {
	if err := json.Unmarshal([]byte(sourceForeignJSON), &foreign); err != nil {
		panic("library: foreign-names.json does not parse: " + err.Error())
	}
}

// Offer is what an environment offers, for the lookup: its library
// functions by full name, and its removed entries (F19 3.4 item 4).
type Offer struct {
	Funcs   map[string]*Entry
	Removed map[string]bool
}

func (o Offer) offered(e *Entry) bool {
	if e.IsMethod() {
		return !o.Removed[e.EntryName()]
	}
	f, ok := o.Funcs[e.Name]
	return ok && f == e
}

func foreignFix(written, place, kind string) string {
	for _, r := range foreign {
		if r.Written != written || r.Place != place {
			continue
		}
		if place == "method" {
			ok := false
			for _, k := range r.Kinds {
				ok = ok || k == kind
			}
			if !ok {
				continue
			}
		}
		return r.Fix
	}
	return ""
}

// form is the entry form of F19 3.4 item 2.
func form(e *Entry) string {
	if e.IsMethod() {
		return "use x." + e.Name + "(...)"
	}
	return "use " + e.Name + "(...)"
}

// forms joins the distinct forms of the entries that match, sorted, the
// first three kept (F19 3.4 item 3). "" when none matches.
func (o Offer) forms(match func(e *Entry) bool) string {
	seen := map[string]bool{}
	var fs []string
	for _, e := range AllEntries {
		if !match(e) || !o.offered(e) {
			continue
		}
		if f := form(e); !seen[f] {
			seen[f] = true
			fs = append(fs, f)
		}
	}
	sort.Strings(fs)
	if len(fs) > 3 {
		fs = fs[:3]
	}
	return strings.Join(fs, " or ")
}

func wrongHas(e *Entry, n string) bool {
	for _, w := range e.Wrong {
		if w == n {
			return true
		}
	}
	return false
}

// NameFix is the fix of an undefined-name: name alone, or name followed by
// the step .field when field is not "" (F19 3.4 rows 1 and 2, item 1).
func (o Offer) NameFix(name, field string) string {
	if field != "" {
		nf := name + "." + field
		if f := foreignFix(nf, "name-field", ""); f != "" {
			return f
		}
		if f := o.forms(func(e *Entry) bool { return wrongHas(e, nf) }); f != "" {
			return f
		}
	}
	return foreignFix(name, "name", "")
}

// FunctionFix is the fix of an undefined-function (rows 1 and 3).
func (o Offer) FunctionFix(name string) string {
	if f := foreignFix(name, "function", ""); f != "" {
		return f
	}
	if f := o.forms(func(e *Entry) bool { return wrongHas(e, name) }); f != "" {
		return f
	}
	return o.forms(func(e *Entry) bool { return e.IsMethod() && e.Name == name })
}

// MethodFix is the fix of a no-method on a receiver of kind k (rows 1 and
// 4). Only a string, a list, a map or a number takes one.
func (o Offer) MethodFix(k value.Kind, name string) string {
	switch k {
	case value.String, value.List, value.Map:
		if f := foreignFix(name, "method", KindWord(k)); f != "" {
			return f
		}
		if f := o.forms(func(e *Entry) bool { return e.IsMethod() && e.Kind == k && wrongHas(e, name) }); f != "" {
			return f
		}
	case value.Number:
	default:
		return ""
	}
	if f := o.forms(func(e *Entry) bool { return !e.IsMethod() && wrongHas(e, name) }); f != "" {
		return f
	}
	return o.forms(func(e *Entry) bool { return !e.IsMethod() && e.Name == name })
}

// FieldFix is the fix of kind, template field-read, on a field (row 5).
func (o Offer) FieldFix(field string) string {
	return o.forms(func(e *Entry) bool { return !e.IsMethod() && wrongHas(e, field) })
}
