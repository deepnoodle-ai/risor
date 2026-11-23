// Package types holds the types of spec/core/05-types.md 2 and the
// operations of section 4: Join, Fits, Merge, Truthy, Falsy and the helpers,
// with printing (3.9) and conversion from type text (3.3).
//
// A Type is immutable. Operations return new types and never change their
// inputs.
package types

import (
	"sort"
	"strings"
	"sync/atomic"
)

// Number alternatives.
const (
	numNone uint8 = iota
	numInt
	numFloat
)

// String alternatives.
const (
	strNone uint8 = iota
	strAll
	strLits
)

// Type is any, or a set of alternatives with at most one of each kind
// (2.1, 2.3). The empty set is never.
type Type struct {
	any     bool
	null    bool
	boolean bool
	num     uint8
	str     uint8
	lits    []string // sorted and unique, when str is strLits
	list    *Type    // the element type, or nil
	rec     *Record  // or nil; flat(U) when uni is set
	// uni is a tagged union of package F5 (f05-unions.md 2.2), the map
	// alternative, with rec its flat record. Nil for a record alone.
	uni *Union
	// tested marks a type that a test of a field narrowed (F5 5.2 item 3).
	tested bool
	opaque []string // sorted and unique
	vars   []string // type variables of a signature, sorted
	// refs are the references to recursive names among the alternatives
	// (F17 12), sorted by name. Their kinds are disjoint from those of the
	// other alternatives but null.
	refs []*Named
	// unf caches the unfolding of a type with references.
	unf atomic.Pointer[Type]

	// size and depth cache Size and Depth (0: not yet known; depth is
	// stored plus one). A type is immutable once built, so each is
	// computed once, from the cached values of its parts. They are read
	// and written atomically: a type of an environment is shared by
	// compiles that run at once.
	size, depth int32
	// deep caches hasDeepRefs: 0 not yet known, 1 no, 2 yes.
	deep int32
}

// Field is one field of a record.
type Field struct {
	Key      string
	Type     *Type
	Optional bool
	// Base and Fill are kept while a schema is mapped (package F1 4.4): a
	// field that absent-is-null or defaults-applied fills has its type before
	// the fill and the fill apart, and Type is their Join. Fill is nil for a
	// field with no fill.
	Base, Fill *Type
}

// BaseOf is a field's base: its type when it has no fill.
func (f Field) BaseOf() *Type {
	if f.Fill != nil {
		return f.Base
	}
	return f.Type
}

// Record is a record type (2.2). Rest is never for an exact record.
type Record struct {
	Fields []Field // sorted by key
	Rest   *Type
	// Closable is the mark of package F1 4.1, used only while a schema is
	// mapped. A Join keeps it when both records have it.
	Closable bool
}

// The basic types.
var (
	Never  = &Type{}
	Any    = &Type{any: true}
	Null   = &Type{null: true}
	Bool   = &Type{boolean: true}
	Int    = &Type{num: numInt}
	Float  = &Type{num: numFloat}
	String = &Type{str: strAll}
)

// Lit is the literal set of the given strings.
func Lit(ss ...string) *Type {
	l := append([]string(nil), ss...)
	sort.Strings(l)
	l = uniq(l)
	return &Type{str: strLits, lits: l}
}

// List is T[].
func List(elem *Type) *Type { return &Type{list: elem} }

// Opaque is the opaque type of a name.
func Opaque(name string) *Type { return &Type{opaque: []string{name}} }

// Var is a type variable of a signature.
func Var(name string) *Type { return &Type{vars: []string{name}} }

// Rec is a record type. The fields need not be sorted. A nil rest is never.
func Rec(fields []Field, rest *Type) *Type {
	fs := append([]Field(nil), fields...)
	sort.Slice(fs, func(i, j int) bool { return fs[i].Key < fs[j].Key })
	if rest == nil {
		rest = Never
	}
	return &Type{rec: &Record{Fields: fs, Rest: rest}}
}

// MapOf is map<T>: the record with no fields and rest T.
func MapOf(t *Type) *Type { return Rec(nil, t) }

func uniq(s []string) []string {
	out := s[:0]
	for i, x := range s {
		if i == 0 || x != s[i-1] {
			out = append(out, x)
		}
	}
	return out
}

func (t *Type) clone() *Type {
	c := &Type{any: t.any, null: t.null, boolean: t.boolean, num: t.num, str: t.str, lits: t.lits,
		list: t.list, rec: t.rec, uni: t.uni, opaque: t.opaque, vars: t.vars, tested: t.tested, refs: t.refs}
	return c
}

// IsAny reports whether t is any. A reference whose definition is any is.
func (t *Type) IsAny() bool { return t.any || (len(t.refs) > 0 && !t.incomplete() && t.u().any) }

// IsNever reports whether t is the empty set.
func (t *Type) IsNever() bool {
	return !t.any && !t.null && !t.boolean && t.num == numNone && t.str == strNone &&
		t.list == nil && t.rec == nil && len(t.opaque) == 0 && len(t.vars) == 0 && len(t.refs) == 0
}

// HasNull reports whether t has the null alternative, in its unfolding.
func (t *Type) HasNull() bool { return t.null || (len(t.refs) > 0 && t.u().null) }

// IsExactlyNull reports whether t is the type null and nothing else.
func (t *Type) IsExactlyNull() bool { return isNullOnly(t) }

// Elem is the element type of t's list alternative, or nil. A reference is
// unfolded.
func (t *Type) Elem() *Type { return t.u().list }

// Record is t's record alternative, or nil. A reference is unfolded.
func (t *Type) Record() *Record { return t.u().rec }

// OpaqueNames are t's opaque alternatives.
func (t *Type) OpaqueNames() []string { return t.u().opaque }

// Vars are the type variables in t, at the top only.
func (t *Type) Vars() []string { return t.vars }

// Lits are the strings of a literal set, or nil.
func (t *Type) Lits() []string {
	t = t.u()
	if t.str == strLits {
		return t.lits
	}
	return nil
}

// IsStringAll reports whether t has the alternative string.
func (t *Type) IsStringAll() bool { return t.u().str == strAll }

// HasString reports whether t has a string or a literal set.
func (t *Type) HasString() bool { return t.u().str != strNone }

// HasBool reports whether t has bool.
func (t *Type) HasBool() bool { return t.u().boolean }

// IsInt reports whether t's number alternative is int.
func (t *Type) IsInt() bool { return t.u().num == numInt }

// HasNumber reports whether t has a number alternative.
func (t *Type) HasNumber() bool { return t.u().num != numNone }

// Kinds returns the number of kinds of t (each opaque name is a kind).
// Null counts. any and never have none.
func (t *Type) Kinds() int {
	t = t.u()
	if t.any {
		return 0
	}
	n := len(t.opaque)
	for _, b := range []bool{t.null, t.boolean, t.num != numNone, t.str != strNone, t.list != nil, t.rec != nil} {
		if b {
			n++
		}
	}
	return n
}

func (t *Type) withoutNull() *Type {
	if len(t.refs) > 0 {
		// A reference whose definition has null is unfolded; the others
		// stay by name (F17 12.5 item 8).
		for _, n := range t.refs {
			if n.def != nil && n.def.null {
				return unfoldSome(t, func(d *Type) bool { return d.null }, (*Type).withoutNull)
			}
		}
	}
	if !t.null {
		return t
	}
	c := t.clone()
	c.null = false
	return c
}

// unfoldSome applies rule to t with each reference whose definition unfold
// selects replaced by that definition, and the other references kept beside
// the result. A definition has no reference at its top, and the kinds of a
// reference are disjoint from the other alternatives' but null, so the
// references kept stand beside the rule's result unchanged.
func unfoldSome(t *Type, unfold func(def *Type) bool, rule func(*Type) *Type) *Type {
	base := t.plain()
	var kept []*Named
	for _, n := range t.refs {
		if n.def == nil {
			panic(&Incomplete{Name: n.Name})
		}
		if unfold(n.def) {
			base = mergeKinds(base, n.def)
		} else {
			kept = append(kept, n)
		}
	}
	return withRefs(rule(base), kept)
}

// Nonnull is t without null; nonnull(any) is any (4.5).
func Nonnull(t *Type) *Type { return t.withoutNull() }

// WithNull is t with the null alternative added. Beside a reference null is
// kept as written, whatever its definition holds (F17 12.4).
func WithNull(t *Type) *Type {
	if t.any || t.null {
		return t
	}
	c := t.clone()
	c.null = true
	return c
}

// unionStrings merges two sorted sets of strings, in time linear in both.
func unionStrings(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := make([]string, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			out = append(out, a[i])
			i++
		case a[i] > b[j]:
			out = append(out, b[j])
			j++
		default:
			out = append(out, a[i])
			i, j = i+1, j+1
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}

// JoinAll is the Join of several types (4.1), in time n log n in their
// number and the strings of their literal sets: the literal
// sets are gathered and sorted once, and the rest is joined as a balanced
// tree, so no set is copied once for each type joined.
func JoinAll(ts []*Type) *Type {
	for _, t := range ts {
		if hasDeepRefs(t) {
			// With a reference the types are joined from the left, in order
			// (F17 12.5 item 4): a Join of references is not associative as
			// written, and the pairs it enters depend on the order. That
			// holds for a reference inside a list or a record too.
			r := Never
			for _, t := range ts {
				r = Join(r, t)
			}
			return r
		}
	}
	var lits []string
	all := false
	rest := make([]*Type, 0, len(ts))
	for _, t := range ts {
		if t.any {
			return Any
		}
		switch t.str {
		case strAll:
			all = true
		case strLits:
			lits = append(lits, t.lits...)
		}
		if t.str != strNone {
			c := t.clone()
			c.str, c.lits = strNone, nil
			if c.IsNever() {
				continue
			}
			t = c
		}
		rest = append(rest, t)
	}
	r := joinTree(rest)
	switch {
	case all:
		r = Join(r, String)
	case len(lits) > 0:
		r = Join(r, Lit(lits...))
	}
	return r
}

// HasDeepRefs reports whether t has a reference at any depth of its parts,
// not looking into the definitions.
func HasDeepRefs(t *Type) bool { return hasDeepRefs(t) }

// hasDeepRefs reports whether t has a reference at any depth of its parts,
// not looking into the definitions. It is cached, as Size is.
func hasDeepRefs(t *Type) bool {
	switch atomic.LoadInt32(&t.deep) {
	case 1:
		return false
	case 2:
		return true
	}
	has := len(t.refs) > 0
	if !has && t.list != nil {
		has = hasDeepRefs(t.list)
	}
	if !has && t.rec != nil {
	recs:
		for _, r := range ownVariants(t) {
			for _, f := range r.Fields {
				if hasDeepRefs(f.Type) {
					has = true
					break recs
				}
			}
			if r.Rest != nil && hasDeepRefs(r.Rest) {
				has = true
				break
			}
		}
	}
	v := int32(1)
	if has {
		v = 2
	}
	atomic.StoreInt32(&t.deep, v)
	return has
}

func joinTree(ts []*Type) *Type {
	switch len(ts) {
	case 0:
		return Never
	case 1:
		return ts[0]
	}
	m := len(ts) / 2
	return Join(joinTree(ts[:m]), joinTree(ts[m:]))
}

// Join is the type of a value of type a or b (4.1). With references it is
// one computation of F17 12.5.
func Join(a, b *Type) *Type { return (&rctx{}).join(a, b) }

func (c *rctx) join(a, b *Type) *Type {
	if a.any || b.any {
		return Any
	}
	if a == b {
		return a
	}
	if a.IsNever() {
		return b
	}
	if b.IsNever() {
		return a
	}
	if len(a.refs) > 0 || len(b.refs) > 0 {
		return c.joinRefs(a, b)
	}
	return c.joinPlain(a, b)
}

// joinPlain is the Join of Core by kind, for types with no reference at the
// top. Their parts are joined in the computation c.
func (c *rctx) joinPlain(a, b *Type) *Type {
	if a.any || b.any {
		return Any
	}
	if a.IsNever() {
		return b
	}
	if b.IsNever() {
		return a
	}
	r := &Type{
		null:    a.null || b.null,
		boolean: a.boolean || b.boolean,
		num:     max(a.num, b.num),
	}
	switch {
	case a.str == strAll || b.str == strAll:
		r.str = strAll
	case a.str == strLits || b.str == strLits:
		r.str = strLits
		r.lits = unionStrings(a.lits, b.lits)
	}
	switch {
	case a.list != nil && b.list != nil:
		r.list = c.join(a.list, b.list)
	case a.list != nil:
		r.list = a.list
	default:
		r.list = b.list
	}
	switch {
	case a.rec != nil && b.rec != nil:
		// F5 4.1: a union survives only beside the same union; otherwise the
		// Join of the flats. A Join never makes a union.
		if a.uni != nil && b.uni != nil && c.sameUnion(a.uni, b.uni) {
			r.rec, r.uni = a.rec, a.uni
		} else {
			r.rec = c.joinRec(a.rec, b.rec)
		}
	case a.rec != nil:
		r.rec, r.uni = a.rec, a.uni
	default:
		r.rec, r.uni = b.rec, b.uni
	}
	r.opaque = unionStrings(a.opaque, b.opaque)
	r.vars = unionStrings(a.vars, b.vars)
	if len(r.opaque) == 0 {
		r.opaque = nil
	}
	if len(r.vars) == 0 {
		r.vars = nil
	}
	return r
}

func (r *Record) field(k string) (Field, bool) {
	i := sort.Search(len(r.Fields), func(i int) bool { return r.Fields[i].Key >= k })
	if i < len(r.Fields) && r.Fields[i].Key == k {
		return r.Fields[i], true
	}
	return Field{}, false
}

// Field returns the field of key k.
func (r *Record) Field(k string) (Field, bool) { return r.field(k) }

func joinRec(a, b *Record) *Record { return (&rctx{}).joinRec(a, b) }

func (c *rctx) joinRec(a, b *Record) *Record {
	// A fill is kept apart through a Join (F1 4.4 item 3): a key in both has
	// the Join of the bases and of the fills; a key in one only has its base
	// joined with the other's rest, and keeps its fill. A field with a fill
	// is required.
	field := func(k string, base, fill *Type, optional bool) Field {
		if fill == nil {
			return Field{Key: k, Type: base, Optional: optional}
		}
		return Field{Key: k, Type: c.join(base, fill), Base: base, Fill: fill}
	}
	// The keys in code point order (F17 12.5): each key of either record,
	// then the rests.
	var fs []Field
	keyOf := func(k string) {
		f, inA := a.field(k)
		g, inB := b.field(k)
		switch {
		case inA && inB:
			var fill *Type
			if f.Fill != nil || g.Fill != nil {
				fill = c.join(orNever(f.Fill), orNever(g.Fill))
			}
			fs = append(fs, field(k, c.join(f.BaseOf(), g.BaseOf()), fill, f.Optional || g.Optional))
		case inA:
			t := f.BaseOf()
			if !b.Rest.IsNever() {
				t = c.join(t, b.Rest)
			}
			fs = append(fs, field(k, t, f.Fill, true))
		default:
			t := g.BaseOf()
			if !a.Rest.IsNever() {
				t = c.join(t, a.Rest)
			}
			fs = append(fs, field(k, t, g.Fill, true))
		}
	}
	i, j := 0, 0
	for i < len(a.Fields) || j < len(b.Fields) {
		switch {
		case j >= len(b.Fields) || (i < len(a.Fields) && a.Fields[i].Key < b.Fields[j].Key):
			keyOf(a.Fields[i].Key)
			i++
		case i >= len(a.Fields) || b.Fields[j].Key < a.Fields[i].Key:
			keyOf(b.Fields[j].Key)
			j++
		default:
			keyOf(a.Fields[i].Key)
			i, j = i+1, j+1
		}
	}
	// F1 3.6 item 8: closable when one is closable and the other is closable
	// or exact.
	exact := func(r *Record) bool { return r.Rest.IsNever() && !r.Closable }
	cl := (a.Closable && (b.Closable || exact(b))) || (b.Closable && exact(a))
	return &Record{Fields: fs, Rest: c.join(a.Rest, b.Rest), Closable: cl}
}

func orNever(t *Type) *Type {
	if t == nil {
		return Never
	}
	return t
}

// Fits is the assignability relation (4.2). It also reports whether the
// fit used any.
func Fits(s, t *Type) (ok, usedAny bool) {
	f := &fitter{c: &rctx{}}
	ok = f.fits(s, t)
	return ok, f.used
}

type fitter struct {
	used bool
	c    *rctx
}

func (f *fitter) fits(s, t *Type) bool {
	if s.IsNever() || t.any {
		return true
	}
	if s.any {
		f.used = true
		return true
	}
	if len(s.refs) == 0 && len(t.refs) == 0 {
		return f.plain(s, t)
	}
	// F17 12.4: a reference that both sides hold fits at once; otherwise the
	// pair is entered, unless it is in progress, and both are unfolded.
	s2 := dropShared(s, t)
	if s2.IsNever() {
		return true
	}
	if s2.incomplete() {
		s2.u() // panics Incomplete with the name
	}
	if t.incomplete() {
		t.u()
	}
	if f.c.find(f.c.fit, s2, t) {
		return true
	}
	if f.c.find(f.c.fitFail, s2, t) {
		return false
	}
	f.c.enter()
	f.c.fit = append(f.c.fit, pairT{s2, t})
	ok := f.plain(s2.u(), t.u())
	f.c.fit = f.c.fit[:len(f.c.fit)-1]
	if !ok {
		f.c.fitFail = append(f.c.fitFail, pairT{s2, t})
	}
	return ok
}

// plain is the fit of Core, alternative by alternative, in the order of
// printing (F17 12.5): bool, number, string, list, map, opaque names, null.
// It stops at the first that does not fit.
func (f *fitter) plain(s, t *Type) bool {
	if s.IsNever() || t.any {
		return true
	}
	if s.any {
		f.used = true
		return true
	}
	if s.boolean && !t.boolean {
		return false
	}
	switch s.num {
	case numInt:
		if t.num == numNone {
			return false
		}
	case numFloat:
		if t.num != numFloat {
			return false
		}
	}
	switch s.str {
	case strAll:
		if t.str != strAll {
			return false
		}
	case strLits:
		switch t.str {
		case strNone:
			return false
		case strLits:
			for _, l := range s.lits {
				if !contains(t.lits, l) {
					return false
				}
			}
		}
	}
	if s.list != nil && (t.list == nil || !f.fits(s.list, t.list)) {
		return false
	}
	if s.rec != nil && (t.rec == nil || !f.fitsMap(s, t)) {
		return false
	}
	for _, o := range s.opaque {
		if !contains(t.opaque, o) {
			return false
		}
	}
	if s.null && !t.null {
		return false
	}
	return true
}

// fitsMap is the map row of 4.2 with F5 4.2: each variant of S fits some
// variant of T; the fit uses any when some variant of S fits none without
// using any.
func (f *fitter) fitsMap(s, t *Type) bool {
	if s.uni == nil && t.uni == nil {
		return f.fitsRec(s.rec, t.rec)
	}
	for _, sv := range variants(s) {
		found, clean := false, false
		for _, tv := range variants(t) {
			sub := &fitter{c: f.c}
			if sub.fitsRec(sv, tv) {
				found = true
				if !sub.used {
					clean = true
					break
				}
			}
		}
		if !found {
			return false
		}
		if !clean {
			f.used = true
		}
	}
	return true
}

func (f *fitter) fitsRec(s, t *Record) bool {
	for _, tf := range t.Fields {
		if sf, ok := s.field(tf.Key); ok {
			// The mark first, then the type (F17 12.5).
			if (sf.Optional && !tf.Optional) || !f.fits(sf.Type, tf.Type) {
				return false
			}
		} else if !tf.Optional || !f.fits(s.Rest, tf.Type) {
			return false
		}
	}
	for _, sf := range s.Fields {
		if _, ok := t.field(sf.Key); !ok && !f.fits(sf.Type, t.Rest) {
			return false
		}
	}
	return f.fits(s.Rest, t.Rest)
}

func contains(s []string, x string) bool {
	i := sort.SearchStrings(s, x)
	return i < len(s) && s[i] == x
}

// Merge writes the entries of a map of record type m over a map of record
// type r (4.3). m may be any.
func Merge(r *Record, m *Type) *Record {
	m = m.u()
	if m.any {
		fs := make([]Field, len(r.Fields))
		for i, f := range r.Fields {
			fs[i] = Field{Key: f.Key, Type: Any, Optional: f.Optional}
		}
		return &Record{Fields: fs, Rest: Any}
	}
	mr := m.rec
	var fs []Field
	for _, mf := range mr.Fields {
		if !mf.Optional {
			fs = append(fs, Field{Key: mf.Key, Type: mf.Type, Optional: false})
			continue
		}
		rt := r.Rest
		req := false
		if rf, ok := r.field(mf.Key); ok {
			rt, req = rf.Type, !rf.Optional
		}
		fs = append(fs, Field{Key: mf.Key, Type: Join(mf.Type, rt), Optional: !req})
	}
	for _, rf := range r.Fields {
		if _, ok := mr.field(rf.Key); ok {
			continue
		}
		if mr.Rest.IsNever() {
			fs = append(fs, rf)
		} else {
			fs = append(fs, Field{Key: rf.Key, Type: Join(rf.Type, mr.Rest), Optional: rf.Optional})
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].Key < fs[j].Key })
	return &Record{Fields: fs, Rest: Join(r.Rest, mr.Rest)}
}

// RecType wraps a record as a type.
func RecType(r *Record) *Type { return &Type{rec: r} }

// SetField returns the record with field k set, required, to t.
func SetField(r *Record, k string, t *Type) *Record {
	var fs []Field
	for _, f := range r.Fields {
		if f.Key != k {
			fs = append(fs, f)
		}
	}
	fs = append(fs, Field{Key: k, Type: t})
	sort.Slice(fs, func(i, j int) bool { return fs[i].Key < fs[j].Key })
	return &Record{Fields: fs, Rest: r.Rest}
}

// Without returns the record without the fields of the given keys.
func Without(r *Record, keys map[string]bool) *Record {
	var fs []Field
	for _, f := range r.Fields {
		if !keys[f.Key] {
			fs = append(fs, f)
		}
	}
	return &Record{Fields: fs, Rest: r.Rest}
}

// Truthy is the type of the truthy values of t (4.4). A reference stays
// when Truthy keeps its definition whole (F17 12.4).
func Truthy(t *Type) *Type {
	if len(t.refs) > 0 {
		// Each reference that Truthy would not keep whole is unfolded; the
		// others stay by name (F17 12.5 item 8).
		return unfoldSome(t, func(d *Type) bool { return !truthyWhole(d) }, Truthy)
	}
	if t.any || t.IsNever() {
		return t
	}
	r := &Type{boolean: t.boolean, num: t.num, opaque: t.opaque, vars: t.vars}
	switch t.str {
	case strAll:
		r.str = strAll
	case strLits:
		var l []string
		for _, s := range t.lits {
			if s != "" {
				l = append(l, s)
			}
		}
		if len(l) > 0 {
			r.str, r.lits = strLits, l
		}
	}
	if t.list != nil && !t.list.IsNever() {
		r.list = t.list
	}
	if t.rec != nil && !(len(t.rec.Fields) == 0 && t.rec.Rest.IsNever()) {
		r.rec, r.uni = t.rec, t.uni
	}
	return r
}

// truthyWhole reports whether a reference stays whole under Truthy (F17
// 12.5 item 8): its definition has no null, no literal set with "", no
// never[] and no record that can be empty, one with no required field. A
// union cannot be empty: each variant requires its tag.
func truthyWhole(d *Type) bool {
	if d.any || d.null || (d.str == strLits && contains(d.lits, "")) || (d.list != nil && d.list.IsNever()) {
		return false
	}
	if d.rec == nil || d.uni != nil {
		return true
	}
	for _, f := range d.rec.Fields {
		if !f.Optional {
			return true
		}
	}
	return false
}

// Falsy is the type of the falsy values of t (4.4).
func Falsy(t *Type) *Type {
	t = t.u()
	if t.any || t.IsNever() {
		return t
	}
	r := &Type{null: t.null, boolean: t.boolean, num: t.num}
	switch t.str {
	case strAll:
		r.str, r.lits = strLits, []string{""}
	case strLits:
		if contains(t.lits, "") {
			r.str, r.lits = strLits, []string{""}
		}
	}
	if t.list != nil {
		r.list = Never
	}
	if t.rec != nil {
		req := false
		for _, f := range t.rec.Fields {
			req = req || !f.Optional
		}
		if !req {
			r.rec = &Record{Rest: Never}
		}
	}
	return r
}

// Overlap is overlap(a, b) of 4.5.
func Overlap(a, b *Type) bool {
	a, b = a.u(), b.u()
	if a.IsNever() || b.IsNever() {
		return false
	}
	if a.any || b.any {
		return true
	}
	if (a.null && b.null) || (a.boolean && b.boolean) || (a.num != numNone && b.num != numNone) ||
		(a.list != nil && b.list != nil) || (a.rec != nil && b.rec != nil) {
		return true
	}
	if a.str != strNone && b.str != strNone {
		if a.str == strLits && b.str == strLits {
			for _, l := range a.lits {
				if contains(b.lits, l) {
					return true
				}
			}
		} else {
			return true
		}
	}
	for _, o := range a.opaque {
		if contains(b.opaque, o) {
			return true
		}
	}
	return false
}

// HasOpaque is has-opaque(t) of 4.5. A definition is looked at once.
func HasOpaque(t *Type) bool { return hasOpaque(t, visited{}) }

func hasOpaque(t *Type, seen visited) bool {
	if t.any {
		return false
	}
	if len(t.opaque) > 0 {
		return true
	}
	for _, n := range t.refs {
		if !seen[n] {
			seen[n] = true
			if hasOpaque(n.def, seen) {
				return true
			}
		}
	}
	if t.list != nil && hasOpaque(t.list, seen) {
		return true
	}
	for _, r := range variants(t.plain()) {
		for _, f := range r.Fields {
			if hasOpaque(f.Type, seen) {
				return true
			}
		}
		if hasOpaque(r.Rest, seen) {
			return true
		}
	}
	return false
}

// HasAny is has-any(t) of 05-types.md 4.5: t is any, or an element, field or
// rest type of t has any, at any depth. A definition is looked at once.
func HasAny(t *Type) bool { return hasAny(t, visited{}) }

func hasAny(t *Type, seen visited) bool {
	if t.any {
		return true
	}
	for _, n := range t.refs {
		if !seen[n] {
			seen[n] = true
			if hasAny(n.def, seen) {
				return true
			}
		}
	}
	if t.list != nil && hasAny(t.list, seen) {
		return true
	}
	for _, r := range variants(t.plain()) {
		for _, f := range r.Fields {
			if hasAny(f.Type, seen) {
				return true
			}
		}
		if hasAny(r.Rest, seen) {
			return true
		}
	}
	return false
}

// Finite reports whether t is finite (4.5). A reference is not: its
// definition has a list or a record.
func Finite(t *Type) bool {
	return !t.any && len(t.refs) == 0 && t.num == numNone && t.str != strAll && t.list == nil && t.rec == nil &&
		len(t.opaque) == 0 && len(t.vars) == 0
}

// Depth is the depth of a type (2.5). A type shared at many places, as a
// schema's $ref gives, is measured once.
func Depth(t *Type) int {
	if t.any || (t.list == nil && t.rec == nil) {
		return 0
	}
	if d := atomic.LoadInt32(&t.depth); d > 0 {
		return int(d - 1)
	}
	d := -1
	if t.list != nil {
		d = max(d, Depth(t.list))
	}
	if t.rec != nil {
		for _, f := range t.rec.Fields {
			d = max(d, Depth(f.Type))
		}
		d = max(d, Depth(t.rec.Rest))
	}
	atomic.StoreInt32(&t.depth, int32(d+2))
	return d + 1
}

// MaxSize is the limit on the size of a type (05-types.md 20.7).
// The specification fixes it.
const MaxSize = 100000

// Size is the size of a type written out with no sharing (20.7), saturated
// at MaxSize + 1. It is computed once per type from the sizes of its parts,
// so it never expands the type.
func Size(t *Type) int {
	if s := atomic.LoadInt32(&t.size); s > 0 {
		return int(s)
	}
	n := 0
	add := func(k int) {
		n += k
		if n > MaxSize {
			n = MaxSize + 1
		}
	}
	if t.any {
		n = 1
	} else {
		if t.null {
			add(1)
		}
		if t.boolean {
			add(1)
		}
		if t.num != numNone {
			add(1)
		}
		switch t.str {
		case strAll:
			add(1)
		case strLits:
			add(len(t.lits))
		}
		if t.list != nil {
			add(1 + Size(t.list))
		}
		if t.rec != nil {
			// F5 2.6: a union is the sum of its variants; flat is not counted.
			for _, r := range ownVariants(t) {
				add(1)
				for _, f := range r.Fields {
					add(Size(f.Type))
				}
				if !r.Rest.IsNever() {
					add(Size(r.Rest))
				}
			}
		}
		// A reference counts 1 (F17 12.6).
		add(len(t.opaque) + len(t.vars) + len(t.refs))
		if n == 0 {
			n = 1 // never
		}
	}
	atomic.StoreInt32(&t.size, int32(n))
	return n
}

// Equal reports whether two types are the same type: the same by structure
// (F5 2.5), coinductive over references (F17 12.4). One computation.
func Equal(a, b *Type) bool { return (&rctx{}).equal(a, b) }

// equalPlain compares two types with no reference at the top, alternative
// by alternative in the order of printing (F17 12.5 item 6): bool, number,
// string, list, map, the opaque names, null. It stops at the first that
// differs, so a list or map compared before a null that differs still
// counts its pairs.
func (c *rctx) equalPlain(a, b *Type) bool {
	if a.any != b.any || a.boolean != b.boolean || a.num != b.num || a.str != b.str || !eqStrings(a.lits, b.lits) {
		return false
	}
	if (a.list == nil) != (b.list == nil) || (a.list != nil && !c.equal(a.list, b.list)) {
		return false
	}
	if (a.rec == nil) != (b.rec == nil) || (a.uni == nil) != (b.uni == nil) {
		return false
	}
	switch {
	case a.uni != nil:
		if !c.sameUnion(a.uni, b.uni) {
			return false
		}
	case a.rec != nil:
		if !c.sameRec(a.rec, b.rec) {
			return false
		}
	}
	return eqStrings(a.opaque, b.opaque) && eqStrings(a.vars, b.vars) && a.null == b.null
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Subst replaces each type variable by its solution, an unbound one by
// never (13.5 steps 5 and 6).
func Subst(t *Type, sol map[string]*Type) *Type {
	if t.any {
		return t
	}
	if len(t.vars) == 0 && !hasVars(t) {
		return t
	}
	c := t.clone()
	c.vars = nil
	if c.list != nil {
		c.list = Subst(c.list, sol)
	}
	if c.rec != nil {
		fs := make([]Field, len(c.rec.Fields))
		for i, f := range c.rec.Fields {
			fs[i] = Field{Key: f.Key, Type: Subst(f.Type, sol), Optional: f.Optional}
		}
		c.rec = &Record{Fields: fs, Rest: Subst(c.rec.Rest, sol)}
	}
	var r *Type = c
	for _, v := range t.vars {
		s := sol[v]
		if s == nil {
			s = Never
		}
		r = Join(r, s)
	}
	return r
}

func hasVars(t *Type) bool {
	if len(t.vars) > 0 {
		return true
	}
	if t.list != nil && hasVars(t.list) {
		return true
	}
	if t.rec != nil {
		for _, f := range t.rec.Fields {
			if hasVars(f.Type) {
				return true
			}
		}
		return hasVars(t.rec.Rest)
	}
	return false
}

// HasVars reports whether t holds a type variable at any depth.
func HasVars(t *Type) bool { return hasVars(t) }

// VarsIn lists the type variables in t at any depth.
func VarsIn(t *Type, into map[string]bool) {
	for _, v := range t.vars {
		into[v] = true
	}
	if t.list != nil {
		VarsIn(t.list, into)
	}
	if t.rec != nil {
		for _, f := range t.rec.Fields {
			VarsIn(f.Type, into)
		}
		VarsIn(t.rec.Rest, into)
	}
}

// String prints a type by 3.9.
func (t *Type) String() string {
	if t.any {
		return "any"
	}
	var parts []string
	if t.boolean {
		parts = append(parts, "bool")
	}
	switch t.num {
	case numInt:
		parts = append(parts, "int")
	case numFloat:
		parts = append(parts, "float")
	}
	switch t.str {
	case strAll:
		parts = append(parts, "string")
	case strLits:
		for _, l := range t.lits {
			parts = append(parts, quote(l))
		}
	}
	if t.list != nil {
		e := t.list.String()
		if strings.Contains(topLevel(e), " | ") {
			e = "(" + e + ")"
		}
		parts = append(parts, e+"[]")
	}
	switch {
	case t.uni != nil: // F5 2.4: the variants, by the least string of each tag set
		for _, v := range t.uni.Variants {
			parts = append(parts, v.String())
		}
	case t.rec != nil:
		parts = append(parts, t.rec.String())
	}
	parts = append(parts, t.opaque...)
	// F17 12.7: a reference prints by its name, after the opaque names.
	for _, n := range t.refs {
		parts = append(parts, n.Name)
	}
	parts = append(parts, t.vars...)
	if t.null {
		parts = append(parts, "null")
	}
	if len(parts) == 0 {
		return "never"
	}
	return strings.Join(parts, " | ")
}

// topLevel blanks out the bracketed parts of printed text, so that a " | "
// inside them is not seen.
func topLevel(s string) string {
	b := []byte(s)
	depth := 0
	inStr := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case inStr:
			if c == '\\' {
				b[i] = 'x'
				i++
				if i < len(b) {
					b[i] = 'x'
				}
				continue
			}
			if c == '"' {
				inStr = false
			}
			b[i] = 'x'
		case c == '"':
			inStr = true
			if depth > 0 {
				b[i] = 'x'
			}
		case c == '(' || c == '{' || c == '<' || c == '[':
			depth++
		case c == ')' || c == '}' || c == '>' || c == ']':
			depth--
		default:
			if depth > 0 {
				b[i] = 'x'
			}
		}
	}
	return string(b)
}

// String prints a record by 3.9.
func (r *Record) String() string {
	if len(r.Fields) == 0 {
		if r.Rest.IsNever() {
			return "{}"
		}
		return "map<" + r.Rest.String() + ">"
	}
	var parts []string
	for _, f := range r.Fields {
		k := f.Key
		if !isWord(k) {
			k = quote(k)
		}
		if f.Optional {
			k += "?"
		}
		parts = append(parts, k+": "+f.Type.String())
	}
	switch {
	case r.Rest.any:
		parts = append(parts, "...")
	case !r.Rest.IsNever():
		parts = append(parts, "..."+r.Rest.String())
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func isWord(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (i > 0 && c >= '0' && c <= '9')
		if !ok {
			return false
		}
	}
	return true
}

// quote is the JSON form of a string (01-language.md 12).
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	const hex = "0123456789abcdef"
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&15])
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Close makes the rest of each closable record of t never and drops its
// mark, at every depth (package F1 4.1 item 3). A recursive name gets a
// closed name of its own, with the same name.
func Close(t *Type) *Type {
	return (&closer{memo: map[*Type]*Type{}, names: map[*Named]*Named{}}).closeT(t)
}

type closer struct {
	memo  map[*Type]*Type
	names map[*Named]*Named
}

func (k *closer) name(n *Named) *Named {
	if m, ok := k.names[n]; ok {
		return m
	}
	m := &Named{Name: n.Name, seq: n.seq}
	k.names[n] = m
	m.def = k.closeT(n.def)
	return m
}

func (k *closer) closeT(t *Type) *Type {
	if t.any || (t.list == nil && t.rec == nil && len(t.refs) == 0) {
		return t
	}
	if c, ok := k.memo[t]; ok {
		return c
	}
	c := t.clone()
	if len(t.refs) > 0 {
		rs := make([]*Named, len(t.refs))
		for i, n := range t.refs {
			rs[i] = k.name(n)
		}
		c.refs = rs
	}
	if c.list != nil {
		c.list = k.closeT(c.list)
	}
	closeRec := func(r *Record) *Record {
		fs := make([]Field, len(r.Fields))
		for i, f := range r.Fields {
			fs[i] = Field{Key: f.Key, Type: k.closeT(f.Type), Optional: f.Optional}
		}
		rest := k.closeT(r.Rest)
		if r.Closable {
			rest = Never
		}
		return &Record{Fields: fs, Rest: rest}
	}
	switch {
	case c.uni != nil: // F5 8.2: Close closes each variant
		vs := make([]*Record, len(c.uni.Variants))
		for i, v := range c.uni.Variants {
			vs[i] = closeRec(v)
		}
		u := MapOfVariants(vs)
		c.rec, c.uni = u.rec, u.uni
	case c.rec != nil:
		c.rec = closeRec(c.rec)
	}
	k.memo[t] = c
	return c
}
