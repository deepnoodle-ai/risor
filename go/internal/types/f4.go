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
	if x.any {
		return Any
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
	return out
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
// the rule of F4 6.2 can show. It is sound and not complete.
func Disjoint(a, b *Type) bool {
	if a.IsNever() || b.IsNever() {
		return true
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
				if !disjointRec(va, vb) {
					return false
				}
			}
		}
	}
	return true
}

// disjointRec: two records share a value unless some key required in
// either has disjoint types in the two.
func disjointRec(a, b *Record) bool {
	typeOf := func(r *Record, k string) *Type {
		if f, ok := r.field(k); ok {
			return f.Type
		}
		return r.Rest
	}
	test := func(r, other *Record) bool {
		for _, f := range r.Fields {
			if !f.Optional && Disjoint(f.Type, typeOf(other, f.Key)) {
				return true
			}
		}
		return false
	}
	return test(a, b) || test(b, a)
}
