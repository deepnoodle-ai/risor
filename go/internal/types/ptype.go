package types

import "github.com/deepnoodle-ai/risor/v3/internal/syntax"

// PDiag is a static diagnostic of a program type (package F4 6.1).
type PDiag struct {
	Code string
	Span syntax.Span
	Name string // an undefined name
	Tmpl string // check-limit: depth or size
}

// FromPType resolves a program type in a scope (F4 3.5, 6.1). It returns
// the type, or nil and the diagnostics of 6.1 in their order: every
// undefined name; else every PType with two records; else one check-limit
// for depth and size.
func FromPType(pt *syntax.PType, sc *Scope) (*Type, []PDiag) {
	var ds []PDiag
	var names func(t *syntax.PType)
	names = func(t *syntax.PType) {
		for _, a := range t.Alts {
			p := a.Prim
			switch p.Kind {
			case syntax.PName:
				if _, ok := sc.Named[p.Text]; !ok && !sc.Opaque[p.Text] {
					ds = append(ds, PDiag{Code: "undefined-name", Span: p.Span, Name: p.Text})
				}
			case syntax.PMap, syntax.PParen:
				names(p.Elem)
			case syntax.PRecord:
				for _, f := range p.Fields {
					names(f.Type)
				}
				if p.Rest != nil {
					names(p.Rest)
				}
			}
		}
	}
	names(pt)
	if len(ds) > 0 {
		return nil, ds
	}
	r := &presolver{sc: sc}
	t := r.typ(pt)
	if len(r.ds) > 0 {
		return nil, r.ds
	}
	if Depth(t) > MaxDepth {
		return nil, []PDiag{{Code: "check-limit", Span: pt.Span, Tmpl: "depth"}}
	}
	if Size(t) > MaxSize {
		return nil, []PDiag{{Code: "check-limit", Span: pt.Span, Tmpl: "size"}}
	}
	return t, nil
}

type presolver struct {
	sc *Scope
	ds []PDiag
}

// typ resolves a PType. A PType whose own alternatives, with parentheses
// removed, hold two or more records is untagged-union at its span; a
// parenthesized PType that reported holds no record for the PType around
// it, so each is reported once, at the innermost.
func (r *presolver) typ(t *syntax.PType) *Type {
	// F5 3.1, 3.2: the records of a PType, a union and a parenthesized PType
	// contributing their variants (one that reported, none); two or more
	// must form a tagged union, else untagged-union here.
	xs := make([]*Type, 0, len(t.Alts))
	var recs []*Record
	for _, a := range t.Alts {
		x, n := r.alt(a)
		if x.rec != nil && n > 0 {
			recs = append(recs, variants(x)...)
		}
		if x.rec != nil && a.Lists == 0 {
			x = WithoutMap(x)
		}
		xs = append(xs, x)
	}
	out := JoinAll(xs)
	if len(recs) > 0 {
		m, ok := FormUnion(recs)
		if !ok {
			r.ds = append(r.ds, PDiag{Code: "untagged-union", Span: t.Span})
			return out
		}
		out = WithMap(out, m)
	}
	return out
}

// alt resolves an alternative, and counts the records it gives to its PType.
func (r *presolver) alt(a *syntax.PAlt) (*Type, int) {
	p := a.Prim
	var t *Type
	recs := 0
	switch p.Kind {
	case syntax.PWord:
		switch p.Text {
		case "null":
			t = Null
		case "bool":
			t = Bool
		case "int":
			t = Int
		case "float":
			t = Float
		case "string":
			t = String
		case "any":
			t = Any
		default:
			t = Never
		}
	case syntax.PString:
		t = Lit(p.Text)
	case syntax.PName:
		if n, ok := r.sc.Named[p.Text]; ok {
			t = n
		} else {
			t = Opaque(p.Text)
		}
		if t.rec != nil {
			recs = 1
		}
	case syntax.PMap:
		t = MapOf(r.typ(p.Elem))
		recs = 1
	case syntax.PParen:
		before := len(r.ds)
		t = r.typ(p.Elem)
		if t.rec != nil && len(r.ds) == before {
			recs = 1
		}
	case syntax.PRecord:
		fs := make([]Field, 0, len(p.Fields))
		for _, f := range p.Fields {
			fs = append(fs, Field{Key: f.Key, Type: r.typ(f.Type), Optional: f.Optional})
		}
		rest := Never
		if p.HasRest {
			rest = Any
			if p.Rest != nil {
				rest = r.typ(p.Rest)
			}
		}
		t = Rec(fs, rest)
		recs = 1
	}
	if a.Lists > 0 {
		recs = 0
		for i := 0; i < a.Lists; i++ {
			t = List(t)
		}
	}
	return t, recs
}
