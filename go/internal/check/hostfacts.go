package check

import "github.com/deepnoodle-ai/risor/go/v3/internal/syntax"

// The null fact and its one effect, of package F14 (f14-host-facts.md 4,
// 5).

// isMember reports whether environment variable v is a member of a host
// fact.
func (c *checker) isMember(v int) bool {
	for _, f := range c.env.Facts {
		for _, m := range f {
			if m == v {
				return true
			}
		}
	}
	return false
}

// memberOf gives the variable of a member path: an Ident with no step that
// resolves to a member (4.1 item 1).
func memberVar(p path) (int, bool) {
	if len(p.keys) != 0 || p.root >= 0 {
		return 0, false
	}
	return -1 - p.root, true
}

// nullFacts is {null(p)} for a member path p, else none.
func (c *checker) nullFacts(p path) facts {
	if len(c.env.Facts) == 0 {
		return nil
	}
	if v, ok := memberVar(p); ok && c.isMember(v) {
		return facts{{p: p, kind: factNull}}
	}
	return nil
}

// nullOf is N(a) of 4.4: null(p) for a member path, N(a1) ∪ N(a2) for
// a1 ?? a2, and none for any other form.
func (c *checker) nullOf(n syntax.Node) facts {
	if len(c.env.Facts) == 0 {
		return nil
	}
	switch n := n.(type) {
	case *syntax.Ident:
		if p, ok := c.pathOf(n); ok {
			return c.nullFacts(p)
		}
	case *syntax.Binary:
		if n.Op == "??" {
			return union(c.nullOf(n.X), c.nullOf(n.Y))
		}
	}
	return nil
}

// someNotNull is the effect of 5.1: some host fact that has variable v as
// a member has every other member under null in the facts in force.
func (c *checker) someNotNull(v int, fs facts) bool {
	for _, f := range c.env.Facts {
		has, all := false, true
		for _, m := range f {
			if m == v {
				has = true
				continue
			}
			if !fs.isNull(path{root: -1 - m}) {
				all = false
			}
		}
		if has && all {
			return true
		}
	}
	return false
}

// isNull reports whether null(p) is in force.
func (f facts) isNull(p path) bool {
	for _, q := range f {
		if q.kind == factNull && q.p.equal(p) {
			return true
		}
	}
	return false
}
