package types

import "github.com/deepnoodle-ai/risor/go/v3/internal/syntax"

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
	return FromPTypeList(pt, sc, nil)
}

// FromPTypeList resolves the program type of a type line of a list being
// resolved (F17 12.2): look resolves the names of the list, with whether
// each stands at a guarded place.
func FromPTypeList(pt *syntax.PType, sc *Scope, look func(name string, guarded bool) (*Type, bool)) (t *Type, ds []PDiag) {
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
	if look == nil { // the names of a list are tested by its resolver
		names(pt)
	}
	if len(ds) > 0 {
		return nil, ds
	}
	r := &presolver{sc: sc, look: look}
	if Guard(func() { t = r.typ(pt) }) {
		return nil, []PDiag{{Code: "check-limit", Span: pt.Span, Tmpl: "pairs"}}
	}
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
	sc    *Scope
	ds    []PDiag
	look  func(name string, guarded bool) (*Type, bool)
	guard int
}

// typ resolves a PType. A PType whose own alternatives, with parentheses
// removed, hold two or more records is untagged-union at its span; a
// parenthesized PType that reported holds no record for the PType around
// it, so each is reported once, at the innermost.
func (r *presolver) typ(t *syntax.PType) *Type {
	// F5 3.1, 3.2: the records of a PType, a union and a parenthesized PType
	// contributing their variants (one that reported, none); two or more
	// must form a tagged union, else untagged-union here.
	alts := make([]*Type, len(t.Alts))
	counts := make([]int, len(t.Alts))
	for i, a := range t.Alts {
		alts[i], counts[i] = r.alt(a)
	}
	// F17 12.3: a reference that shares a kind with another alternative
	// counts as its definition, whose records then count.
	shared := unfoldShared(alts)
	for i := range alts {
		if shared[i] != alts[i] && shared[i].rec != nil && t.Alts[i].Lists == 0 {
			counts[i] = 1
		}
	}
	xs := make([]*Type, 0, len(t.Alts))
	var refs []*Named
	var recs []*Record
	for i, a := range t.Alts {
		x := shared[i]
		refs = append(refs, x.refs...)
		x = x.plain()
		if x.rec != nil && counts[i] > 0 {
			recs = append(recs, variants(x)...)
		}
		if x.rec != nil && a.Lists == 0 {
			x = WithoutMap(x)
		}
		xs = append(xs, x)
	}
	out := withRefs(JoinAll(xs), refs)
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
	if a.Lists > 0 {
		r.guard++
		defer func() { r.guard-- }()
	}
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
		found := false
		if r.look != nil {
			t, found = r.look(p.Text, r.guard > 0)
		}
		if !found {
			if n, ok := r.sc.Named[p.Text]; ok {
				t = n
			} else {
				t = Opaque(p.Text)
			}
		}
		if t.rec != nil {
			recs = 1
		}
	case syntax.PMap:
		r.guard++
		t = MapOf(r.typ(p.Elem))
		r.guard--
		recs = 1
	case syntax.PParen:
		before := len(r.ds)
		t = r.typ(p.Elem)
		if t.rec != nil && len(r.ds) == before {
			recs = 1
		}
	case syntax.PRecord:
		r.guard++
		defer func() { r.guard-- }()
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

// PTypeUses lists the names in a program type, in text order, each with its
// span and whether it stands at a guarded place (F17 12.2).
func PTypeUses(pt *syntax.PType, add func(name string, sp syntax.Span, guarded bool)) {
	var typ func(t *syntax.PType, g bool)
	typ = func(t *syntax.PType, g bool) {
		for _, a := range t.Alts {
			ga := g || a.Lists > 0
			p := a.Prim
			switch p.Kind {
			case syntax.PName:
				add(p.Text, p.Span, ga)
			case syntax.PMap:
				typ(p.Elem, true)
			case syntax.PParen:
				typ(p.Elem, ga)
			case syntax.PRecord:
				for _, f := range p.Fields {
					typ(f.Type, true)
				}
				if p.Rest != nil {
					typ(p.Rest, true)
				}
			}
		}
	}
	typ(pt, false)
}
