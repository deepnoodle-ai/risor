package check

import (
	"sort"

	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
)

// This file is the table "A map literal against a record" of 05-types.md
// 15.1, and the row of package F5 6 that uses it.

// giver is one entry of a map literal, by what it gives (15.1).
type giver struct {
	en     *syntax.Entry // a written entry, or nil
	spread *syntax.Spread
	// m is the operand's type with null removed: a record, any, or nil
	// for a spread that gives nothing (never, or reported as kind).
	m *types.Type
}

func (c *checker) givers(e *syntax.Map) []giver {
	gs := make([]giver, len(e.Entries))
	for i, en := range e.Entries {
		switch en := en.(type) {
		case *syntax.Entry:
			gs[i].en = en
		case *syntax.Spread:
			gs[i].spread = en
			m := types.Nonnull(c.tyOf(en))
			if m.IsAny() || (!m.IsNever() && m.Kinds() == 1 && m.Record() != nil) {
				gs[i].m = m
			}
		}
	}
	return gs
}

func (c *checker) tyOf(n syntax.Node) *types.Type {
	if t, ok := c.ty[n]; ok {
		return t
	}
	return types.Never
}

// gives is what an entry gives for key k; other asks for the other key.
func (g *giver) gives(c *checker, k string, other bool) (surely, may bool, t *types.Type) {
	if g.en != nil {
		if !other && g.en.Key == k {
			return true, true, c.tyOf(g.en.Value)
		}
		return false, false, nil
	}
	switch {
	case g.m == nil:
		return false, false, nil
	case g.m.IsAny():
		return false, true, types.Any
	}
	r := g.m.Record()
	if !other {
		if f, ok := r.Field(k); ok {
			return !f.Optional, true, f.Type
		}
	}
	if !r.Rest.IsNever() {
		return false, true, r.Rest
	}
	return false, false, nil
}

// contributors are the entries that contribute key k (15.1).
func (c *checker) contributors(gs []giver, k string, other bool) []int {
	last := -1
	for i := range gs {
		if s, _, _ := gs[i].gives(c, k, other); s {
			last = i
		}
	}
	var out []int
	for i := range gs {
		if i < last {
			continue
		}
		if _, m, _ := gs[i].gives(c, k, other); m {
			out = append(out, i)
		}
	}
	return out
}

func noComputed(m *syntax.Map) bool {
	for _, en := range m.Entries {
		if e, ok := en.(*syntax.Entry); ok && e.KeyForm == syntax.KeyComputed {
			return false
		}
	}
	return true
}

// failer reports a failure at sp: the type at does not fit w; with at nil,
// required field key has no entry; with key set and at not nil, the record
// expected does not allow key.
type failer func(sp syntax.Span, at, w, expected *types.Type, key string)

// fitMapRecord fits a map literal with no computed key to a record by the
// table of 15.1.
func (c *checker) fitMapRecord(e *syntax.Map, gs []giver, r *types.Record, want *types.Type, k formKind, fail failer) {
	keys := map[string]bool{}
	for _, f := range r.Fields {
		keys[f.Key] = true
	}
	for _, g := range gs {
		switch {
		case g.en != nil:
			keys[g.en.Key] = true
		case g.m != nil && !g.m.IsAny():
			for _, f := range g.m.Record().Fields {
				keys[f.Key] = true
			}
		}
	}
	order := make([]string, 0, len(keys))
	for key := range keys {
		order = append(order, key)
	}
	sort.Strings(order)
	spreadDone := make([]bool, len(gs))
	spreadDyn := make([]bool, len(gs))
	check := func(key string, other bool) {
		rt, listed := r.Rest, false
		if !other {
			if f, ok := r.Field(key); ok {
				rt, listed = f.Type, true
			}
		}
		for _, i := range c.contributors(gs, key, other) {
			g := &gs[i]
			if g.en != nil {
				if !listed && r.Rest.IsNever() {
					fail(g.en.KeySpan, c.tyOf(g.en.Value), types.Never, want, key)
				} else {
					c.fitForm(g.en.Value, c.tyOf(g.en.Value), rt, k)
				}
				continue
			}
			if spreadDone[i] {
				continue
			}
			_, _, gt := g.gives(c, key, other)
			ok, used := types.Fits(gt, rt)
			sp := g.spread.Value.Span()
			switch {
			case !ok:
				spreadDone[i] = true
				if !listed && r.Rest.IsNever() && !other {
					fail(sp, gt, types.Never, want, key)
				} else {
					fail(sp, gt, rt, nil, "")
				}
			case used && !spreadDyn[i]:
				spreadDyn[i] = true
				c.dyn(sp)
			}
		}
	}
	for _, key := range order {
		check(key, false)
	}
	check("", true)
	for _, f := range r.Fields {
		if f.Optional {
			continue
		}
		surely := false
		for i := range gs {
			if s, _, _ := gs[i].gives(c, f.Key, false); s {
				surely = true
				break
			}
		}
		if !surely {
			fail(e.Span(), nil, want, nil, f.Key)
			return
		}
	}
}

// fitMapUnion is the row of package F5 6: the contributors of the tag
// choose the variant, and the map is fitted to it.
func (c *checker) fitMapUnion(e *syntax.Map, gs []giver, u *types.Union, k formKind, fail failer) {
	surely := false
	for i := range gs {
		if s, _, _ := gs[i].gives(c, u.Tag, false); s {
			surely = true
		}
	}
	if !surely {
		fail(e.Span(), nil, nil, nil, u.Tag)
		return
	}
	cs := c.contributors(gs, u.Tag, false)
	var ts []*types.Type
	for _, i := range cs {
		_, _, t := gs[i].gives(c, u.Tag, false)
		ts = append(ts, t)
	}
	j := types.JoinAll(ts)
	var v *types.Record
	if ls := j.Lits(); ls != nil && j.Kinds() == 1 && !j.HasNull() {
		v = types.VariantOf(u, ls[0])
		for _, l := range ls[1:] {
			if types.VariantOf(u, l) != v {
				v = nil
			}
		}
	}
	if v == nil {
		for _, i := range cs {
			g := &gs[i]
			_, _, t := g.gives(c, u.Tag, false)
			if g.en != nil {
				fail(g.en.Value.Span(), t, types.TagType(u), nil, "")
			} else {
				fail(g.spread.Value.Span(), t, types.TagType(u), nil, "")
			}
		}
		return
	}
	vt := types.RecType(v)
	c.fitMapRecord(e, gs, v, vt, k, fail)
}
