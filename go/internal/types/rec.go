package types

import (
	"sort"
	"sync/atomic"
)

// This file is the recursive named types of package F17, step E2
// (spec/packages/f17-recursion.md 12): references, their unfolding, and the
// coinductive computations of Fits, Equal, Join and disjoint, which visit
// pairs in a fixed order and enter at most MaxPairs of them.

// Named is a recursive name: a named type that refers to itself, directly
// or through other names of its list. A reference to it is an alternative
// of a type. Its definition is set once, when its list is resolved, before
// the type is shared.
type Named struct {
	Name string
	def  *Type
	seq  int64
}

var namedSeq atomic.Int64

// NewNamed makes the recursive name of one declaration.
func NewNamed(name string) *Named { return &Named{Name: name, seq: namedSeq.Add(1)} }

// Def is the definition: the type the name stands for, one level deep. Nil
// until the list is resolved.
func (n *Named) Def() *Type { return n.def }

// SetDef sets the definition. The definition holds no reference at its
// top: every name at a top place was replaced (12.2).
func (n *Named) SetDef(t *Type) { n.def = t }

// Complete reports whether the definition is known.
func (n *Named) Complete() bool { return n.def != nil }

// Ref is the type that is a reference to n.
func Ref(n *Named) *Type { return &Type{refs: []*Named{n}} }

// Refs are the references among t's alternatives, in the order printed.
func (t *Type) Refs() []*Named { return t.refs }

// HasRefs reports whether t has a reference among its alternatives.
func (t *Type) HasRefs() bool { return len(t.refs) > 0 }

// Unfold is t with each reference among its alternatives replaced by its
// definition: the type one level down. A type with no reference is itself.
func Unfold(t *Type) *Type { return t.u() }

// MaxPairs is the most pairs one computation of Fits, Equal, Join or
// disjoint may enter (12.5).
const MaxPairs = 4096

// PairLimit is the panic value of a computation that passes MaxPairs. The
// checker reports it as check-limit, template pairs.
type PairLimit struct{}

// Incomplete is the panic value of a rule that looks into a reference whose
// definition is not known yet: a reference of a list being resolved.
type Incomplete struct{ Name string }

// Guard runs fn and reports whether a computation in it passed MaxPairs.
func Guard(fn func()) (limit bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*PairLimit); !ok {
				panic(r)
			}
			limit = true
		}
	}()
	fn()
	return false
}

// u is the unfolding of t: its plain alternatives with the alternatives of
// each reference's definition. The kinds of a reference are disjoint from
// those of the other alternatives but null, so this is a union of slots.
func (t *Type) u() *Type {
	if len(t.refs) == 0 {
		return t
	}
	if c := t.unf.Load(); c != nil {
		return c
	}
	r := t.plain()
	for _, n := range t.refs {
		if n.def == nil {
			panic(&Incomplete{Name: n.Name})
		}
		r = mergeKinds(r, n.def)
	}
	t.unf.Store(r)
	return r
}

// plain is t without its references.
func (t *Type) plain() *Type {
	if len(t.refs) == 0 {
		return t
	}
	c := t.clone()
	c.refs = nil
	return c
}

// mergeKinds is the union of two types with no reference at the top. Their
// kinds are disjoint but for null, so no part is joined; should two share a
// kind, the Join of Core gives the union.
func mergeKinds(a, b *Type) *Type {
	switch {
	case a.any || b.any:
		return Any
	case a.IsNever():
		return b
	case b.IsNever():
		return a
	}
	return (&rctx{}).joinPlain(a, b)
}

// incomplete reports whether t has a reference whose definition is not
// known.
func (t *Type) incomplete() bool {
	for _, n := range t.refs {
		if n.def == nil {
			return true
		}
	}
	return false
}

// kind bits of a type's alternatives, null apart.
const (
	kBool = 1 << iota
	kNum
	kStr
	kList
	kMap
)

// kinds gives the kinds of t's unfolding but null: a bit set, and the
// opaque names.
func kinds(t *Type) (bits int, opaque []string) {
	u := t.u()
	if u.any {
		return kBool | kNum | kStr | kList | kMap, nil
	}
	if u.boolean {
		bits |= kBool
	}
	if u.num != numNone {
		bits |= kNum
	}
	if u.str != strNone {
		bits |= kStr
	}
	if u.list != nil {
		bits |= kList
	}
	if u.rec != nil {
		bits |= kMap
	}
	return bits, u.opaque
}

// SharesKind reports whether a and b have a kind other than null in
// common. Either may be a reference.
func SharesKind(a, b *Type) bool {
	if a.IsAny() || b.IsAny() {
		return !a.IsNever() && !b.IsNever()
	}
	ab, ao := kinds(a)
	bb, bo := kinds(b)
	if ab&bb != 0 {
		return true
	}
	for _, o := range ao {
		if contains(bo, o) {
			return true
		}
	}
	return false
}

// withRefs is t with the references rs added, sorted.
func withRefs(t *Type, rs []*Named) *Type {
	if len(rs) == 0 {
		return t
	}
	if t.any {
		return t
	}
	c := t.clone()
	c.refs = sortRefs(append(append([]*Named(nil), t.refs...), rs...))
	return c
}

// sortRefs orders references by name, then by declaration, dropping one
// that stands twice.
func sortRefs(rs []*Named) []*Named {
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].Name != rs[j].Name {
			return rs[i].Name < rs[j].Name
		}
		return rs[i].seq < rs[j].seq
	})
	out := rs[:0]
	for i, r := range rs {
		if i == 0 || r != rs[i-1] {
			out = append(out, r)
		}
	}
	return out
}

func hasRef(rs []*Named, n *Named) bool {
	for _, r := range rs {
		if r == n {
			return true
		}
	}
	return false
}

// identical reports whether two types are the same as written: the same
// alternatives, each reference the same reference, and each part
// identical. Nothing is unfolded. It decides when a pair is met again.
func identical(a, b *Type) bool {
	if a == b {
		return true
	}
	if a.any != b.any || a.null != b.null || a.boolean != b.boolean || a.num != b.num || a.str != b.str {
		return false
	}
	if !eqStrings(a.lits, b.lits) || !eqStrings(a.opaque, b.opaque) || !eqStrings(a.vars, b.vars) {
		return false
	}
	if len(a.refs) != len(b.refs) {
		return false
	}
	for i := range a.refs {
		if a.refs[i] != b.refs[i] {
			return false
		}
	}
	if (a.list == nil) != (b.list == nil) || (a.list != nil && !identical(a.list, b.list)) {
		return false
	}
	if (a.rec == nil) != (b.rec == nil) || (a.uni == nil) != (b.uni == nil) {
		return false
	}
	if a.uni != nil {
		if len(a.uni.Variants) != len(b.uni.Variants) {
			return false
		}
		for i := range a.uni.Variants {
			if !identicalRec(a.uni.Variants[i], b.uni.Variants[i]) {
				return false
			}
		}
		return true
	}
	return a.rec == nil || identicalRec(a.rec, b.rec)
}

func identicalRec(a, b *Record) bool {
	if a == b {
		return true
	}
	if len(a.Fields) != len(b.Fields) || !identical(a.Rest, b.Rest) {
		return false
	}
	for i, f := range a.Fields {
		g := b.Fields[i]
		if f.Key != g.Key || f.Optional != g.Optional || !identical(f.Type, g.Type) {
			return false
		}
	}
	return true
}

// rctx is one computation: the pairs in progress of each operation, and
// the count of pairs entered.
type rctx struct {
	n                     int
	fit, joins, eq, disjt []pairT
	// fitFail and eqFail are the pairs that have failed in this
	// computation: met again, each fails at once (12.4).
	fitFail, eqFail []pairT
}

type pairT struct{ a, b *Type }

func (c *rctx) find(st []pairT, a, b *Type) bool {
	for i := len(st) - 1; i >= 0; i-- {
		if identical(st[i].a, a) && identical(st[i].b, b) {
			return true
		}
	}
	return false
}

// enter counts a pair entered, and stops the computation past MaxPairs.
func (c *rctx) enter() {
	c.n++
	if c.n > MaxPairs {
		panic(&PairLimit{})
	}
}

// dropShared is s without the references t also has: each fits at once.
func dropShared(s, t *Type) *Type {
	var keep []*Named
	for _, r := range s.refs {
		if !hasRef(t.refs, r) {
			keep = append(keep, r)
		}
	}
	if len(keep) == len(s.refs) {
		return s
	}
	c := s.clone()
	c.refs = keep
	return c
}

// isNullOnly reports whether t is null and nothing else.
func isNullOnly(t *Type) bool {
	return t.null && !t.any && len(t.refs) == 0 && t.withoutNullPlain().IsNever()
}

func (t *Type) withoutNullPlain() *Type {
	if !t.null {
		return t
	}
	c := t.clone()
	c.null = false
	return c
}

// join with references (12.4): null is added as written; a pair that one
// side surely fits gives the other; a pair in progress gives any; else the
// references that share a kind with the other side are unfolded and the
// two are joined by kind.
func (c *rctx) joinRefs(a, b *Type) *Type {
	if isNullOnly(b) {
		return WithNull(a)
	}
	if isNullOnly(a) {
		return WithNull(b)
	}
	if a.incomplete() || b.incomplete() {
		return c.lazyJoin(a, b)
	}
	if identical(a, b) {
		return a
	}
	if a.u().any || b.u().any {
		return Any
	}
	if c.surely(a, b) {
		return b
	}
	if c.surely(b, a) {
		return a
	}
	if c.find(c.joins, a, b) {
		return Any
	}
	c.enter()
	c.joins = append(c.joins, pairT{a, b})
	defer func() { c.joins = c.joins[:len(c.joins)-1] }()
	ap, ak := splitRefs(a, b)
	bp, bk := splitRefs(b, a)
	r := c.joinPlain(ap, bp)
	return withRefs(r, append(ak, bk...))
}

// splitRefs unfolds each reference of t that shares a kind with other and
// that other does not hold; it gives the rest of t, unfolded, and the
// references kept.
func splitRefs(t, other *Type) (*Type, []*Named) {
	p := t.plain()
	var kept []*Named
	for _, n := range t.refs {
		if hasRef(other.refs, n) || !SharesKind(Ref(n), other) {
			kept = append(kept, n)
			continue
		}
		p = mergeKinds(p, n.def)
	}
	return p, kept
}

// lazyJoin keeps the references beside the other alternatives, unfolding
// none: the Join of a list being resolved (12.2). The rule of 12.2 item 6
// tests it once the list is resolved.
func (c *rctx) lazyJoin(a, b *Type) *Type {
	r := c.joinPlain(a.plain(), b.plain())
	return withRefs(r, append(append([]*Named(nil), a.refs...), b.refs...))
}

// surely reports a fit that holds without any, in this computation.
func (c *rctx) surely(s, t *Type) bool {
	if t.any {
		return true
	}
	f := &fitter{c: c}
	ok := f.fits(s, t)
	return ok && !f.used
}

// equal is Equal in a computation: coinductive over references.
func (c *rctx) equal(a, b *Type) bool {
	if a == b {
		return true
	}
	if len(a.refs) == 0 && len(b.refs) == 0 {
		return c.equalPlain(a, b)
	}
	if identical(a, b) {
		return true
	}
	if a.incomplete() || b.incomplete() {
		return false
	}
	if c.find(c.eq, a, b) {
		return true
	}
	if c.find(c.eqFail, a, b) {
		return false
	}
	c.enter()
	c.eq = append(c.eq, pairT{a, b})
	ok := c.equalPlain(a.u(), b.u())
	c.eq = c.eq[:len(c.eq)-1]
	if !ok {
		c.eqFail = append(c.eqFail, pairT{a, b})
	}
	return ok
}

// HasAnyIn and the other walks that follow references keep a set of the
// names visited, so each definition is looked at once.
type visited map[*Named]bool

// OpaqueIn lists the opaque names of t at any depth, each definition looked
// at once, in the order met.
func OpaqueIn(t *Type) []string {
	var out []string
	seen := visited{}
	var walk func(t *Type)
	walk = func(t *Type) {
		if t.any {
			return
		}
		for _, o := range t.opaque {
			if !hasString(out, o) {
				out = append(out, o)
			}
		}
		for _, n := range t.refs {
			if !seen[n] && n.def != nil {
				seen[n] = true
				walk(n.def)
			}
		}
		if t.list != nil {
			walk(t.list)
		}
		for _, r := range variants(t.plain()) {
			for _, f := range r.Fields {
				walk(f.Type)
			}
			walk(r.Rest)
		}
	}
	walk(t)
	return out
}
