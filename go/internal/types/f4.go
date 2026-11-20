package types

// This file is the helpers of package F4 6.2: surely fits, disjoint and
// without.

// SurelyFits reports whether every value of S conforms to T: Fits(S, T)
// holds and did not use any. any surely fits only any.
func SurelyFits(s, t *Type) bool {
	if t.any {
		return true
	}
	if s.any {
		return false
	}
	ok, used := Fits(s, t)
	return ok && !used
}

// alts gives the alternatives of a type, each as a type of one
// alternative. A literal set is one alternative.
func alts(t *Type) []*Type {
	var out []*Type
	if t.null {
		out = append(out, Null)
	}
	if t.boolean {
		out = append(out, Bool)
	}
	switch t.num {
	case numInt:
		out = append(out, Int)
	case numFloat:
		out = append(out, Float)
	}
	switch t.str {
	case strAll:
		out = append(out, String)
	case strLits:
		out = append(out, &Type{str: strLits, lits: t.lits})
	}
	if t.list != nil {
		out = append(out, &Type{list: t.list})
	}
	for _, v := range variants(t) { // F5: each variant is an alternative
		out = append(out, &Type{rec: v})
	}
	for _, o := range t.opaque {
		out = append(out, Opaque(o))
	}
	for _, v := range t.vars {
		out = append(out, Var(v))
	}
	return out
}

// WithoutType is without(X, T): the type left of X when a value of type X
// is known not to conform to T.
func WithoutType(x, t *Type) *Type {
	if x.IsAny() {
		return Any
	}
	// F17 12.4: a reference that shares a kind with T, null included, is
	// unfolded; one that shares none is an alternative T does not take.
	var kept []*Named
	if len(x.refs) > 0 {
		p := x.plain()
		for _, n := range x.refs {
			r := Ref(n)
			if SharesKind(r, t) || (n.def.null && t.HasNull()) {
				p = mergeKinds(p, n.def)
			} else {
				kept = append(kept, n)
			}
		}
		x = p
	}
	out := Never
	var left []*Record // the variants left, which may form a smaller union
	for _, a := range alts(x) {
		if SurelyFits(a, t) {
			continue
		}
		if a.rec != nil {
			left = append(left, a.rec)
			continue
		}
		if a.str == strLits && t.str == strLits {
			var left []string
			for _, l := range a.lits {
				if !hasString(t.lits, l) {
					left = append(left, l)
				}
			}
			if len(left) == 0 {
				continue
			}
			a = &Type{str: strLits, lits: left}
		}
		out = Join(out, a)
	}
	if len(left) > 0 {
		out = WithMap(out, MapOfVariants(left))
	}
	return withRefs(out, kept)
}

func hasString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// Disjoint reports whether no value conforms to both a and b, as far as
// the rule of F4 6.2 can show. It is sound and not complete. With
// references it is one computation of F17 12.5: a pair met again while in
// progress is not shown disjoint.
func Disjoint(a, b *Type) bool { return (&rctx{}).disjoint(a, b) }

func (c *rctx) disjoint(a, b *Type) bool {
	if a.IsNever() || b.IsNever() {
		return true
	}
	if len(a.refs) > 0 || len(b.refs) > 0 {
		if c.find(c.disjt, a, b) {
			return false
		}
		c.enter()
		c.disjt = append(c.disjt, pairT{a, b})
		defer func() { c.disjt = c.disjt[:len(c.disjt)-1] }()
		a, b = a.u(), b.u()
		if a.IsNever() || b.IsNever() {
			return true
		}
	}
	if a.any || b.any {
		return false
	}
	if (a.null && b.null) || (a.boolean && b.boolean) || (a.num != numNone && b.num != numNone) {
		return false
	}
	for _, o := range a.opaque {
		if hasString(b.opaque, o) {
			return false
		}
	}
	if a.str != strNone && b.str != strNone {
		if a.str == strAll || b.str == strAll {
			return false
		}
		for _, l := range a.lits {
			if hasString(b.lits, l) {
				return false
			}
		}
	}
	if a.list != nil && b.list != nil {
		return false
	}
	if a.rec != nil && b.rec != nil {
		// F5 4.5: unless every variant of a is disjoint from every one of b.
		for _, va := range variants(a) {
			for _, vb := range variants(b) {
				if !c.disjointRec(va, vb) {
					return false
				}
			}
		}
	}
	return true
}

// disjointRec: two records share a value unless some key required in
// either has disjoint types in the two.
func (c *rctx) disjointRec(a, b *Record) bool {
	typeOf := func(r *Record, k string) *Type {
		if f, ok := r.field(k); ok {
			return f.Type
		}
		return r.Rest
	}
	test := func(r, other *Record) bool {
		for _, f := range r.Fields {
			if !f.Optional && c.disjoint(f.Type, typeOf(other, f.Key)) {
				return true
			}
		}
		return false
	}
	return test(a, b) || test(b, a)
}
