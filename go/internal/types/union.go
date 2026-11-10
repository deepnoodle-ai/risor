package types

import "sort"

// This file is the tagged union of package F5 (spec/packages/f05-unions.md):
// two or more records told apart by a key each requires with its own
// strings.

// Union is a tagged union: its variants, in code point order of the least
// string of each tag set, and its tag.
type Union struct {
	Variants []*Record
	Tag      string
	tagType  *Type
}

// Union returns the tagged union of t's map alternative, or nil.
func (t *Type) Union() *Union { return t.uni }

// variants are the variants of t's map alternative: of a union its
// variants, of a record that record alone.
func variants(t *Type) []*Record {
	if t.uni != nil {
		return t.uni.Variants
	}
	if t.rec != nil {
		return []*Record{t.rec}
	}
	return nil
}

// Variants is variants(t), for the checker.
func Variants(t *Type) []*Record { return variants(t) }

// litOnly reports whether a type is a literal set and nothing else: a tag
// set (2.1 item 1.2).
func litOnly(t *Type) bool {
	return !t.any && t.str == strLits && !t.null && !t.boolean && t.num == numNone &&
		t.list == nil && t.rec == nil && len(t.opaque) == 0 && len(t.vars) == 0
}

// FindTag finds the tag of records (2.1): the least key that each requires
// with a literal set and nothing else, no string in two of them.
func FindTag(recs []*Record) (string, bool) {
	if len(recs) < 2 {
		return "", false
	}
	for _, f := range recs[0].Fields { // sorted by key: the first that holds is the least
		if f.Optional || f.Fill != nil || !litOnly(f.Type) {
			continue
		}
		seen := map[string]bool{}
		ok := true
		for _, r := range recs {
			// A key that any record fills is no tag (F5 8.1 item 6, 8.3
			// item 8; ruling BH8).
			g, has := r.field(f.Key)
			if !has || g.Optional || g.Fill != nil || !litOnly(g.Type) {
				ok = false
				break
			}
			for _, l := range g.Type.lits {
				if seen[l] {
					ok = false
					break
				}
				seen[l] = true
			}
			if !ok {
				break
			}
		}
		if ok {
			return f.Key, true
		}
	}
	return "", false
}

// tagSet is the tag set of a variant.
func tagSet(r *Record, tag string) []string {
	f, _ := r.field(tag)
	return f.Type.lits
}

// newUnion makes the union of records that form one, or false.
func newUnion(recs []*Record) (*Union, bool) {
	tag, ok := FindTag(recs)
	if !ok {
		return nil, false
	}
	vs := append([]*Record(nil), recs...)
	sort.Slice(vs, func(i, j int) bool { return tagSet(vs[i], tag)[0] < tagSet(vs[j], tag)[0] })
	u := &Union{Variants: vs, Tag: tag}
	var all []string
	for _, v := range vs {
		all = append(all, tagSet(v, tag)...)
	}
	u.tagType = Lit(all...)
	return u, true
}

// flatOf is flat(U) (2.3): the Join of the variants as records.
func flatOf(vs []*Record) *Record {
	r := vs[0]
	for _, v := range vs[1:] {
		r = joinRec(r, v)
	}
	return r
}

// MapOfVariants is "the type of V" (4.5 item 2) for records that form a
// union or are one record: a record, a union, or never. Records that form no
// union give the record Join of them.
func MapOfVariants(vs []*Record) *Type {
	switch len(vs) {
	case 0:
		return Never
	case 1:
		return RecType(vs[0])
	}
	if u, ok := newUnion(vs); ok {
		return &Type{rec: flatOf(u.Variants), uni: u}
	}
	return RecType(flatOf(vs))
}

// FormUnion is the map alternative of records that form a tagged union, or
// false when they form none (3.1, 8.1).
func FormUnion(vs []*Record) (*Type, bool) {
	if len(vs) == 1 {
		return RecType(vs[0]), true
	}
	if _, ok := newUnion(vs); !ok {
		return nil, false
	}
	return MapOfVariants(vs), true
}

// WithMap replaces the map alternative of t by that of m.
func WithMap(t, m *Type) *Type {
	c := t.clone()
	c.rec, c.uni = m.rec, m.uni
	return c
}

// WithoutMap is t with no map alternative.
func WithoutMap(t *Type) *Type {
	if t.rec == nil {
		return t
	}
	c := t.clone()
	c.rec, c.uni = nil, nil
	return c
}

// sameRec reports whether two records are the same by structure (2.5).
func sameRec(a, b *Record) bool {
	if len(a.Fields) != len(b.Fields) || !Equal(a.Rest, b.Rest) {
		return false
	}
	for i, f := range a.Fields {
		g := b.Fields[i]
		if f.Key != g.Key || f.Optional != g.Optional || !Equal(f.Type, g.Type) {
			return false
		}
	}
	return true
}

// sameUnion compares two unions as sets of records.
func sameUnion(a, b *Union) bool {
	if a == b {
		return true
	}
	if len(a.Variants) != len(b.Variants) {
		return false
	}
	used := make([]bool, len(b.Variants))
	for _, v := range a.Variants {
		found := false
		for j, w := range b.Variants {
			if !used[j] && sameRec(v, w) {
				used[j], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Unite is Join, except that the map alternatives keep their variants and
// form a union when they can (4.5).
func Unite(a, b *Type) *Type {
	j := Join(a, b)
	if a.any || b.any || a.rec == nil || b.rec == nil {
		return j
	}
	var vs []*Record
	for _, v := range append(append([]*Record(nil), variants(a)...), variants(b)...) {
		dup := false
		for _, w := range vs {
			if sameRec(v, w) {
				dup = true
				break
			}
		}
		if !dup {
			vs = append(vs, v)
		}
	}
	if len(vs) >= 2 {
		if u, ok := newUnion(vs); ok {
			c := j.clone()
			c.rec, c.uni = flatOf(u.Variants), u
			return c
		}
	}
	return j
}

// keyType is the type of a key in a record: its field's type, else the
// rest (4.5 item 1).
func keyType(r *Record, k string) *Type {
	if f, ok := r.field(k); ok {
		return f.Type
	}
	return r.Rest
}

// Hit is the type of hit(M, f, S) (4.5): the variants in which the type of f
// is not disjoint from the literal set of S.
func Hit(t *Type, f string, s []string) *Type {
	lit := Lit(s...)
	var vs []*Record
	for _, v := range variants(t) {
		if !Disjoint(keyType(v, f), lit) {
			vs = append(vs, v)
		}
	}
	return MapOfVariants(vs)
}

// All is the type of all(M, f, S) (4.5): the variants that list f as a
// required field whose type surely fits the literal set of S.
func All(t *Type, f string, s []string) *Type {
	lit := Lit(s...)
	var vs []*Record
	for _, v := range variants(t) {
		if g, ok := v.field(f); ok && !g.Optional && SurelyFits(g.Type, lit) {
			vs = append(vs, v)
		}
	}
	return MapOfVariants(vs)
}

// TagType is the tag type of a union: every string of every tag set.
func TagType(u *Union) *Type { return u.tagType }

// VariantOf is the variant whose tag set holds s, or nil.
func VariantOf(u *Union, s string) *Record {
	for _, v := range u.Variants {
		if contains(tagSet(v, u.Tag), s) {
			return v
		}
	}
	return nil
}

// MarkTested marks a type that a test of a field narrowed (F5 5.2 item 3):
// a record it leaves still gives facts.
func MarkTested(t *Type) *Type {
	if t.rec == nil || t.tested {
		return t
	}
	c := t.clone()
	c.tested = true
	return c
}

// Tested reports the mark of MarkTested.
func (t *Type) Tested() bool { return t.tested }
