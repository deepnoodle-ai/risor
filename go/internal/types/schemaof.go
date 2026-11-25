package types

import (
	"errors"
	"sort"
	"strings"
)

// SchemaOf gives the JSON Schema of t (f16-entry-points.md 11.5), as its
// JSON text: no white space, the keys of every object in code point order.
// It refuses a type that holds an opaque type or a type variable, and one
// that reaches two recursive types of one name.
func SchemaOf(t *Type) (string, error) {
	w := &schemaWriter{defs: map[string]*Named{}}
	root := w.schema(t)
	if len(w.order) > 0 {
		// Each recursive type reached, through the definitions too: a
		// definition may reach another, so the list grows as it is read.
		defs := map[string]any{}
		for i := 0; i < len(w.order); i++ {
			n := w.order[i]
			defs[n.Name] = w.schema(n.Def())
		}
		// A type that reaches a reference has a schema that is an object.
		root.(map[string]any)["$defs"] = defs
	}
	if w.err != nil {
		return "", w.err
	}
	var b strings.Builder
	writeJSON(&b, root)
	return b.String(), nil
}

type schemaWriter struct {
	defs  map[string]*Named
	order []*Named
	err   error
}

func (w *schemaWriter) fail(msg string) any {
	if w.err == nil {
		w.err = errors.New(msg)
	}
	return map[string]any{}
}

func kind(k string) map[string]any { return map[string]any{"type": k} }

func (w *schemaWriter) schema(t *Type) any {
	if w.err != nil {
		return map[string]any{}
	}
	if t.any {
		return map[string]any{}
	}
	if len(t.opaque) > 0 {
		return w.fail("an opaque type has no JSON Schema")
	}
	if len(t.vars) > 0 {
		return w.fail("a type variable has no JSON Schema")
	}
	var alts []any
	if t.boolean {
		alts = append(alts, kind("boolean"))
	}
	switch t.num {
	case numInt:
		alts = append(alts, kind("integer"))
	case numFloat:
		alts = append(alts, kind("number"))
	}
	switch t.str {
	case strAll:
		alts = append(alts, kind("string"))
	case strLits:
		s := kind("string")
		if len(t.lits) == 1 {
			s["const"] = t.lits[0]
		} else {
			e := make([]any, len(t.lits))
			for i, l := range t.lits {
				e[i] = l
			}
			s["enum"] = e
		}
		alts = append(alts, s)
	}
	if t.list != nil {
		s := kind("array")
		s["items"] = w.schema(t.list)
		alts = append(alts, s)
	}
	switch {
	case t.uni != nil:
		for _, v := range t.uni.Variants {
			alts = append(alts, w.record(v))
		}
	case t.rec != nil:
		alts = append(alts, w.record(t.rec))
	}
	for _, n := range t.refs {
		if m, ok := w.defs[n.Name]; !ok {
			w.defs[n.Name] = n
			w.order = append(w.order, n)
		} else if m != n {
			return w.fail("two recursive types are named " + n.Name)
		}
		alts = append(alts, map[string]any{"$ref": "#/$defs/" + n.Name})
	}
	if t.null {
		alts = append(alts, kind("null"))
	}
	switch len(alts) {
	case 0:
		return false
	case 1:
		return alts[0]
	}
	return map[string]any{"anyOf": alts}
}

func (w *schemaWriter) record(r *Record) any {
	s := kind("object")
	if len(r.Fields) > 0 {
		props := map[string]any{}
		var req []any
		for _, f := range r.Fields {
			fs := w.schema(f.Type)
			// The description of a field of type never is dropped: false
			// has no keys.
			if m, ok := fs.(map[string]any); ok && f.Doc != "" {
				m["description"] = f.Doc
			}
			props[f.Key] = fs
			if !f.Optional {
				req = append(req, f.Key)
			}
		}
		s["properties"] = props
		if len(req) > 0 {
			s["required"] = req
		}
	}
	switch {
	case r.Rest.IsNever():
		s["additionalProperties"] = false
	case r.Rest.any:
	default:
		s["additionalProperties"] = w.schema(r.Rest)
	}
	return s
}

// writeJSON writes a schema value: a map with its keys in code point order,
// a list, a string, or a boolean.
func writeJSON(b *strings.Builder, v any) {
	switch x := v.(type) {
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		b.WriteString(quote(x))
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSON(b, e)
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(quote(k))
			b.WriteByte(':')
			writeJSON(b, x[k])
		}
		b.WriteByte('}')
	}
}
