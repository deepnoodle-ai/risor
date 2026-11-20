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
// progress is not shown disjoint, and a pair shown disjoint is disjoint at
// once when met again.
func Disjoint(a, b *Type) bool { return (&rctx{}).disjoint(a, b) }

func (c *rctx) disjoint(a, b *Type) bool {
	if a.IsNever() || b.IsNever() {
		return true
	}
	if len(a.refs) == 0 && len(b.refs) == 0 {
		return c.disjointPlain(a, b)
	}
	if c.find(c.disjt, a, b) {
		return false
	}
	// F17 12.5 item 5: for disjoint, a pair has failed when it is shown
	// disjoint, the outcome opposite to that of a pair in progress.
	if c.find(c.disjFail, a, b) {
		return true
	}
	c.enter()
	c.disjt = append(c.disjt, pairT{a, b})
	ok := c.disjointPlain(a.u(), b.u())
	c.disjt = c.disjt[:len(c.disjt)-1]
	if ok {
		c.disjFail = append(c.disjFail, pairT{a, b})
	}
	return ok
}

// disjointPlain is the rule of F4 6.2 on two types with no reference at the
// top. It takes the alternatives in the order of printing (F17 12.5 item
// 6): bool, number, string, list, map, the opaque names, null. Only the map
// enters pairs, so a kind shared before it ends the test before any pair
// is entered, and one shared after it only once the map is done.
func (c *rctx) disjointPlain(a, b *Type) bool {
	if a.IsNever() || b.IsNever() {
		return true
	}
	if a.any || b.any {
		return false
	}
	if (a.boolean && b.boolean) || (a.num != numNone && b.num != numNone) {
		return false
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
	for _, o := range a.opaque {
		if hasString(b.opaque, o) {
			return false
		}
	}
	return !(a.null && b.null)
}

// disjointRec: two records share a value unless some key required in
// either has disjoint types in the two. The keys of both records are taken
// once each, in code point order (F17 12.5 item 6), and the test stops at
// the first key whose types are disjoint.
func (c *rctx) disjointRec(a, b *Record) bool {
	typeOf := func(r *Record, k string) (*Type, bool) {
		if f, ok := r.field(k); ok {
			return f.Type, !f.Optional
		}
		return r.Rest, false
	}
	i, j := 0, 0
	for i < len(a.Fields) || j < len(b.Fields) {
		var k string
		switch {
		case j >= len(b.Fields) || (i < len(a.Fields) && a.Fields[i].Key < b.Fields[j].Key):
			k = a.Fields[i].Key
			i++
		case i >= len(a.Fields) || b.Fields[j].Key < a.Fields[i].Key:
			k = b.Fields[j].Key
			j++
		default:
			k = a.Fields[i].Key
			i++
			j++
		}
		ta, ra := typeOf(a, k)
		tb, rb := typeOf(b, k)
		if (ra || rb) && c.disjoint(ta, tb) {
			return true
		}
	}
	return false
}
