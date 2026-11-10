package check

import (
	"github.com/deepnoodle-ai/risor/v3/internal/static"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// synth checks a node and gives its type (1.1).
func (c *checker) synth(n syntax.Node, fs facts) *types.Type {
	t := c.synth0(n, fs)
	c.ty[n] = t
	return t
}

func (c *checker) synth0(n syntax.Node, fs facts) *types.Type {
	switch n := n.(type) {
	case *syntax.Number:
		if value.IsWhole(n.Value) {
			return types.Int
		}
		return types.Float
	case *syntax.String:
		return types.Lit(n.Value)
	case *syntax.Bool:
		return types.Bool
	case *syntax.Null:
		return types.Null
	case *syntax.Ident:
		return c.ident(n, fs)
	case *syntax.Template:
		for _, p := range n.Parts {
			if p.Expr == nil {
				continue
			}
			c.rendered(p.Expr, c.synth(p.Expr, fs))
		}
		return types.String
	case *syntax.List:
		return c.list(n, fs)
	case *syntax.Map:
		return c.mapLit(n, fs)
	case *syntax.Unary:
		return c.unary(n, fs)
	case *syntax.Binary:
		return c.binary(n, fs)
	case *syntax.Cond:
		return c.cond(n, fs)
	case *syntax.Block:
		for _, k := range n.Consts {
			c.constLine(k, fs)
		}
		return c.synth(n.Result, fs)
	case *syntax.Member, *syntax.Index, *syntax.MethodCall, *syntax.OptChain:
		if mc, ok := n.(*syntax.MethodCall); ok && c.res.NSCalls[mc] {
			return c.call(mc, mc.Args, fs)
		}
		return c.chain(n, fs)
	case *syntax.Call:
		return c.call(n, n.Args, fs)
	case *syntax.Match:
		return c.match(n, fs)
	case *syntax.Try:
		return c.try(n, fs)
	}
	return types.Never
}

// pathOf gives the path of an expression (10.3), by a loop over its steps.
func (c *checker) pathOf(n syntax.Node) (path, bool) {
	var keys []string
	for {
		switch t := n.(type) {
		case *syntax.OptChain:
			n = t.X
			continue
		case *syntax.Member:
			keys = append(keys, t.Name)
			n = t.X
			continue
		case *syntax.Index:
			s, isStr := t.Index.(*syntax.String)
			if !isStr {
				return path{}, false
			}
			keys = append(keys, s.Value)
			n = t.X
			continue
		case *syntax.Ident:
			b := c.res.Idents[t]
			if b == nil {
				return path{}, false
			}
			for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
				keys[i], keys[j] = keys[j], keys[i]
			}
			switch b.Kind {
			case static.Value:
				if b.Slot < 0 {
					return path{}, false
				}
				return path{root: b.Slot, keys: keys}, true
			case static.EnvVar:
				return path{root: -1 - b.Var, keys: keys}, true
			}
		}
		return path{}, false
	}
}

// factsOf gives the true and false facts of a tested expression (10.5).
func (c *checker) factsOf(n syntax.Node) (t, f facts) {
	switch n := n.(type) {
	case *syntax.Unary:
		if n.Op == "!" {
			t, f = c.factsOf(n.X)
			return f, t
		}
	case *syntax.Binary:
		switch n.Op {
		case "&&":
			td, fd := c.factsOf(n.X)
			te, fe := c.factsOf(n.Y)
			return union(td, te), intersect(fd, fe)
		case "||":
			td, fd := c.factsOf(n.X)
			te, fe := c.factsOf(n.Y)
			return intersect(td, te), union(fd, fe)
		case "==", "!=":
			var other syntax.Node
			if _, ok := n.X.(*syntax.Null); ok {
				other = n.Y
			} else if _, ok := n.Y.(*syntax.Null); ok {
				other = n.X
			}
			if other == nil {
				return nil, nil
			}
			p, ok := c.pathOf(other)
			if !ok {
				return nil, nil
			}
			if n.Op == "!=" {
				return facts{p}, nil
			}
			return nil, facts{p}
		}
	}
	if p, ok := c.pathOf(n); ok {
		return facts{p}, nil
	}
	return nil, nil
}

func (c *checker) ident(n *syntax.Ident, fs facts) *types.Type {
	b := c.res.Idents[n]
	if b == nil {
		return types.Never
	}
	var t *types.Type
	switch b.Kind {
	case static.Value:
		if b.Slot < 0 {
			return types.Never
		}
		t = c.slots[b.Slot]
	case static.EnvVar:
		t = c.env.VarTypes[b.Var]
	default:
		return types.Never
	}
	if p, ok := c.pathOf(n); ok && fs.covers(p) {
		t = types.Nonnull(t)
	}
	return t
}

func (c *checker) depthLimit(n syntax.Node, t *types.Type) *types.Type {
	if types.Depth(t) > types.MaxDepth {
		c.diag("check-limit", n.Span())
		return types.Never
	}
	return t
}

func (c *checker) list(n *syntax.List, fs facts) *types.Type {
	elem := types.Never
	for _, it := range n.Items {
		if s, ok := it.(*syntax.Spread); ok {
			t := c.synth(s.Value, fs)
			c.ty[s] = t
			sp := s.Value.Span()
			t = c.nullable(sp, t)
			switch {
			case t.IsAny():
				if !c.dyn(sp) {
					elem = types.Join(elem, types.Any)
				}
			case t.IsNever():
			case t.Kinds() == 1 && t.Elem() != nil:
				elem = types.Join(elem, t.Elem())
			default:
				c.diag("kind", sp)
			}
			continue
		}
		elem = types.Join(elem, c.synth(it, fs))
	}
	return c.depthLimit(n, types.List(elem))
}

func (c *checker) mapLit(n *syntax.Map, fs facts) *types.Type {
	rec := &types.Record{Rest: types.Never}
	for _, e := range n.Entries {
		switch e := e.(type) {
		case *syntax.Entry:
			if e.KeyForm == syntax.KeyComputed {
				kt := c.synth(e.KeyExpr, fs)
				ksp := e.KeyExpr.Span()
				kt = c.nullable(ksp, kt)
				c.mustFit(ksp, kt, types.String, "kind")
				v := c.synth(e.Value, fs)
				c.ty[e] = v
				rec = types.Merge(rec, types.MapOf(v))
				continue
			}
			v := c.synth(e.Value, fs)
			c.ty[e] = v
			rec = types.SetField(rec, e.Key, v)
		case *syntax.Spread:
			t := c.synth(e.Value, fs)
			c.ty[e] = t
			sp := e.Value.Span()
			t = c.nullable(sp, t)
			switch {
			case t.IsAny():
				if !c.dyn(sp) {
					rec = types.Merge(rec, types.Any)
				}
			case t.IsNever():
			case t.Kinds() == 1 && t.Record() != nil:
				rec = types.Merge(rec, t)
			default:
				c.diag("kind", sp)
			}
		}
	}
	return c.depthLimit(n, types.RecType(rec))
}

func (c *checker) unary(n *syntax.Unary, fs facts) *types.Type {
	t := c.synth(n.X, fs)
	sp := n.X.Span()
	if n.Op == "!" {
		c.tested(n.X, t)
		return types.Bool
	}
	t = c.nullable(sp, t)
	if t.IsAny() {
		if c.dyn(sp) {
			return types.Never
		}
		return types.Any
	}
	if ok, _ := types.Fits(t, types.Float); !ok {
		c.diag("kind", sp)
		return types.Never
	}
	return t
}

func fits(t, want *types.Type) bool { ok, _ := types.Fits(t, want); return ok }

func isNonZeroNumber(n syntax.Node) bool {
	num, ok := n.(*syntax.Number)
	return ok && num.Value != 0
}

// binary checks a Binary. A left-associative chain of operators is checked
// by a loop over its left spine (20.6).
func (c *checker) binary(n *syntax.Binary, fs facts) *types.Type {
	if n.Op == "**" {
		return c.binop(n, c.synth(n.X, fs), fs)
	}
	var spine []*syntax.Binary
	var x syntax.Node = n
	for {
		b, ok := x.(*syntax.Binary)
		if !ok || b.Op == "**" {
			break
		}
		spine = append(spine, b)
		x = b.X
	}
	t := c.synth(x, fs)
	for i := len(spine) - 1; i >= 0; i-- {
		t = c.binop(spine[i], t, fs)
		c.ty[spine[i]] = t
	}
	return t
}

func (c *checker) binop(n *syntax.Binary, a *types.Type, fs facts) *types.Type {
	switch n.Op {
	case "&&":
		c.tested(n.X, a)
		tf, _ := c.factsOf(n.X)
		b := c.synth(n.Y, fs.with(tf))
		return types.Join(types.Falsy(a), b)
	case "||":
		c.tested(n.X, a)
		_, ff := c.factsOf(n.X)
		b := c.synth(n.Y, fs.with(ff))
		return types.Join(types.Truthy(a), b)
	case "??":
		b := c.synth(n.Y, fs)
		return types.Join(types.Nonnull(a), b)
	case "==", "!=":
		b := c.synth(n.Y, fs)
		if neverEqual(a, b) {
			c.diag("never-equal", n.Span())
		}
		return types.Bool
	case "in", "not-in":
		return c.in(n, a, c.synth(n.Y, fs))
	}
	b := c.synth(n.Y, fs)
	xs, ys := n.X.Span(), n.Y.Span()
	a = c.nullable(xs, a)
	b = c.nullable(ys, b)
	order := n.Op == "<" || n.Op == "<=" || n.Op == ">" || n.Op == ">="
	anyOp := false
	rejected := false
	if a.IsAny() {
		anyOp = true
		rejected = c.dyn(xs) || rejected
	}
	if b.IsAny() {
		anyOp = true
		rejected = c.dyn(ys) || rejected
	}
	bad := false
	switch n.Op {
	case "+", "<", "<=", ">", ">=":
		fa, sa := fits(a, types.Float), fits(a, types.String)
		fb, sb := fits(b, types.Float), fits(b, types.String)
		if !a.IsAny() && !fa && !sa {
			c.diag("kind", xs)
			bad = true
		}
		if !b.IsAny() && !fb && !sb {
			c.diag("kind", ys)
			bad = true
		}
		if !bad && !anyOp && ((fa && !sa && sb && !fb) || (sa && !fa && fb && !sb)) {
			c.diag("kind", n.Span())
			bad = true
		}
	case "**":
		if !a.IsAny() && !fits(a, types.Float) {
			c.diag("kind", xs)
			bad = true
		}
		if !b.IsAny() && !fits(b, types.Int) {
			c.diag("kind", ys)
			bad = true
		}
	default: // - * / %
		if !a.IsAny() && !fits(a, types.Float) {
			c.diag("kind", xs)
			bad = true
		}
		if !b.IsAny() && !fits(b, types.Float) {
			c.diag("kind", ys)
			bad = true
		}
	}
	switch n.Op {
	case "/":
		if num, ok := n.Y.(*syntax.Number); ok && num.Value == 0 {
			c.diag("zero-divisor", ys)
		}
		if !isNonZeroNumber(n.Y) {
			c.site("divide", n.Span())
		}
	case "%":
		if !isNonZeroNumber(n.Y) {
			c.site("divide", n.Span())
		}
	case "**":
		_, bNum := n.Y.(*syntax.Number)
		if !bNum && !isNonZeroNumber(n.X) {
			c.site("divide", n.Span())
		}
	}
	if order {
		return types.Bool
	}
	if bad || rejected {
		return types.Never
	}
	if anyOp {
		return types.Any
	}
	switch n.Op {
	case "+":
		switch {
		case fits(a, types.Int) && fits(b, types.Int):
			return types.Int
		case fits(a, types.Float) && fits(b, types.Float):
			return types.Float
		}
		return types.String
	case "/":
		return types.Float
	case "**":
		_, bNum := n.Y.(*syntax.Number)
		if fits(a, types.Int) && bNum {
			return types.Int
		}
		return types.Float
	}
	if fits(a, types.Int) && fits(b, types.Int) {
		return types.Int
	}
	return types.Float
}

// in checks in and not in (8.4).
func (c *checker) in(n *syntax.Binary, a, b *types.Type) *types.Type {
	xs, ys := n.X.Span(), n.Y.Span()
	b = c.nullable(ys, b)
	switch {
	case b.IsAny():
		c.dyn(ys)
	case b.IsNever():
	case b.Kinds() == 1 && b.Elem() != nil:
		e := b.Elem()
		if neverEqual(a, e) {
			c.diag("never-equal", n.Span())
		}
	case b.Kinds() == 1 && (b.HasString() || b.Record() != nil):
		a = c.nullable(xs, a)
		c.mustFit(xs, a, types.String, "kind")
	default:
		c.diag("kind", ys)
	}
	return types.Bool
}

// cond checks ?: and an if chain (11.1), the else chain by a loop.
func (c *checker) cond(n *syntax.Cond, fs facts) *types.Type {
	t := types.Never
	acc := fs
	for {
		tt := c.synth(n.Test, acc)
		c.tested(n.Test, tt)
		tf, ff := c.factsOf(n.Test)
		t = types.Join(t, c.synth(n.Then, acc.with(tf)))
		acc = acc.with(ff)
		next, ok := n.Else.(*syntax.Cond)
		if !ok {
			t = types.Join(t, c.synth(n.Else, acc))
			break
		}
		c.ty[n] = t
		n = next
	}
	return t
}

func litType(n syntax.Node) *types.Type {
	switch l := n.(type) {
	case *syntax.Number:
		if value.IsWhole(l.Value) {
			return types.Int
		}
		return types.Float
	case *syntax.String:
		return types.Lit(l.Value)
	case *syntax.Bool:
		return types.Bool
	}
	return types.Null
}

// match checks a match (11.2).
func (c *checker) match(n *syntax.Match, fs facts) *types.Type {
	s := c.synth(n.Subject, fs)
	p, isPath := c.pathOf(n.Subject)
	covered := map[string]bool{}
	hasDefault := false
	t := types.Never
	af := fs
	var seen []syntax.Node
	for _, a := range n.Arms {
		listsNull := false
		for _, l := range a.Literals {
			dup := false
			for _, prev := range seen {
				if litEqual(prev, l) {
					dup = true
					break
				}
			}
			seen = append(seen, l)
			_, isNull := l.(*syntax.Null)
			if isNull {
				listsNull = true
			}
			if dup {
				continue
			}
			if !isNull && neverEqual(litType(l), s) {
				c.diag("never-equal", l.Span())
				continue
			}
			switch l := l.(type) {
			case *syntax.Null:
				covered["null"] = true
			case *syntax.Bool:
				if l.Value {
					covered["true"] = true
				} else {
					covered["false"] = true
				}
			case *syntax.String:
				covered["s:"+l.Value] = true
			}
		}
		if a.Default {
			hasDefault = true
		}
		t = types.Join(t, c.synth(a.Result, af))
		if listsNull && isPath {
			af = af.with(facts{p})
		}
	}
	if !hasDefault && !exhaustive(s, covered) {
		c.diag("non-exhaustive", n.Span())
	}
	return t
}

func exhaustive(s *types.Type, covered map[string]bool) bool {
	if !types.Finite(s) {
		return false
	}
	if s.HasNull() && !covered["null"] {
		return false
	}
	if s.HasBool() && !(covered["true"] && covered["false"]) {
		return false
	}
	for _, l := range s.Lits() {
		if !covered["s:"+l] {
			return false
		}
	}
	return true
}

func litEqual(a, b syntax.Node) bool {
	switch a := a.(type) {
	case *syntax.Number:
		b, ok := b.(*syntax.Number)
		return ok && a.Value == b.Value
	case *syntax.String:
		b, ok := b.(*syntax.String)
		return ok && a.Value == b.Value
	case *syntax.Bool:
		b, ok := b.(*syntax.Bool)
		return ok && a.Value == b.Value
	case *syntax.Null:
		_, ok := b.(*syntax.Null)
		return ok
	}
	return false
}

// try checks a try (14).
func (c *checker) try(n *syntax.Try, fs facts) *types.Type {
	c.drop++
	b := c.synth(n.Body, fs)
	c.drop--
	if n.Name != "" {
		c.setSlot(n, errorType)
	}
	h := c.synth(n.Handler, fs)
	return types.Join(b, h)
}

// constLine checks a const line (12.1).
func (c *checker) constLine(k *syntax.Const, fs facts) {
	if lam, ok := k.Value.(*syntax.Lambda); ok {
		if np, ok := k.Pattern.(*syntax.NamePat); ok && !np.Discard {
			info := &lamInfo{facts: fs, base: c.base, lamDepth: c.lamDepth, maxRel: maxRel(lam)}
			c.lambdas[lam] = info
			if !c.used[lam] {
				// No use: checked once, parameters any, sites dropped (13.9).
				c.drop++
				c.withLambda(lam, info, lam.Depth()-c.lamDepth+c.base, func() {
					for _, p := range lam.Params {
						c.bindParam(p, types.Any, true, info.facts)
					}
					c.synth(lam.Body, info.facts)
				})
				c.drop--
			}
			return
		}
	}
	v := c.synth(k.Value, fs)
	c.bindPattern(k.Pattern, v, k.Value.Span(), fs)
}

// maxRel is d(L) of ruling AO10: the greatest number of levels open at once
// inside the text of the lambda, parameter list included, less n(L). A node
// whose own brackets may hold nothing (a list, a map, a pattern, an argument
// or parameter list) opens one level beyond its first token.
func maxRel(l *syntax.Lambda) int {
	m := 0
	syntax.Walk(l, func(n syntax.Node) bool {
		d := n.Depth()
		switch n := n.(type) {
		case *syntax.List, *syntax.Map, *syntax.MapPat, *syntax.ListPat, *syntax.Call, *syntax.MethodCall, *syntax.Block:
			d++
		case *syntax.Lambda:
			if len(n.Params) != 1 || n.Params[0].Span().Start != n.Span().Start {
				d++ // a parenthesized parameter list
			}
		}
		m = max(m, d-l.Depth())
		return true
	})
	return m
}

// withLambda checks inside a lambda body, with the expanded nesting of 20.2:
// base is the expanded nesting at the lambda's first token.
func (c *checker) withLambda(l *syntax.Lambda, info *lamInfo, base int, fn func()) {
	sb, sd := c.base, c.lamDepth
	c.base, c.lamDepth = base, l.Depth()
	fn()
	c.base, c.lamDepth = sb, sd
}

// expanded is the expanded nesting of a node (20.2).
func (c *checker) expanded(n syntax.Node) int { return n.Depth() - c.lamDepth + c.base }

// bindParam binds a lambda parameter to the type its argument gives, with
// its default (12.4). given is false for an omitted argument.
func (c *checker) bindParam(p *syntax.Param, a *types.Type, given bool, fs facts) {
	t := a
	if p.Default != nil {
		d := c.synth(p.Default, fs)
		if given {
			t = types.Join(types.Nonnull(a), d)
		} else {
			t = d
		}
	}
	c.bindPattern(p.Pattern, t, p.Pattern.Span(), fs)
}

// read is the read of a field of a record by 9.2. It returns the type and
// whether the read is of an unlisted key of an open record.
func (c *checker) readField(r *types.Record, key string, keySpan, recvSpan syntax.Span, optional, present bool) *types.Type {
	if f, ok := r.Field(key); ok {
		if !f.Optional || present {
			return f.Type
		}
		if optional {
			return types.WithNull(f.Type)
		}
		c.diag("maybe-absent", keySpan)
		return f.Type
	}
	if present && !r.Rest.IsNever() {
		return r.Rest // a fact says the key is present (ruling AO13)
	}
	switch {
	case r.Rest.IsAny():
		if optional {
			return types.Any
		}
		if c.dyn(recvSpan) {
			return types.Never
		}
		return types.Any
	case r.Rest.IsNever():
		c.diag("no-field", keySpan)
		return types.Never
	}
	if present {
		return r.Rest
	}
	if optional {
		return types.WithNull(r.Rest)
	}
	c.diag("maybe-absent", keySpan)
	return r.Rest
}

// bindPattern binds a pattern against a type (12.2). bound is the span of
// the value bound (12.3).
func (c *checker) bindPattern(p syntax.Pattern, v *types.Type, bound syntax.Span, fs facts) {
	switch p := p.(type) {
	case *syntax.NamePat:
		c.setSlot(p, v)
	case *syntax.MapPat:
		c.mapPat(p, v, bound, fs)
	case *syntax.ListPat:
		c.listPat(p, v, bound, fs)
	}
}

func (c *checker) mapPat(p *syntax.MapPat, v *types.Type, bound syntax.Span, fs facts) {
	v = c.nullable(bound, v)
	all := func(t *types.Type) {
		for _, f := range p.Fields {
			if f.Default != nil {
				c.synth(f.Default, fs)
			}
			c.setSlot(f, t)
		}
		if p.Rest != nil {
			c.setSlot(p.Rest, t)
		}
	}
	switch {
	case v.IsAny():
		if c.dyn(bound) {
			all(types.Never)
		} else {
			all(types.Any)
		}
		return
	case v.IsNever():
		all(types.Never)
		return
	case v.Kinds() != 1 || v.Record() == nil:
		c.diag("kind", bound)
		all(types.Never)
		return
	}
	r := v.Record()
	named := map[string]bool{}
	for _, f := range p.Fields {
		named[f.Key] = true
		var t *types.Type
		// A site of an unlisted key of an open record is on the field's key
		// (ruling AO6).
		if f.Default != nil {
			rt := c.readField(r, f.Key, f.KeySpan, f.KeySpan, true, false)
			d := c.synth(f.Default, fs)
			t = types.Join(types.Nonnull(rt), d)
		} else {
			t = c.readField(r, f.Key, f.KeySpan, f.KeySpan, false, false)
		}
		c.setSlot(f, t)
	}
	if p.Rest != nil {
		c.setSlot(p.Rest, types.RecType(types.Without(r, named)))
	}
}

func (c *checker) listPat(p *syntax.ListPat, v *types.Type, bound syntax.Span, fs facts) {
	v = c.nullable(bound, v)
	all := func(t *types.Type) {
		for _, e := range p.Elems {
			if e.Default != nil {
				c.synth(e.Default, fs)
			}
			c.setSlot(e.Name, t)
		}
		if p.Rest != nil {
			c.setSlot(p.Rest, t)
		}
	}
	switch {
	case v.IsAny():
		rejected := c.dyn(bound)
		// The index site on each element without a default (ruling AO6).
		for _, e := range p.Elems {
			if e.Default == nil {
				c.site("index", e.Name.Span())
			}
		}
		if rejected {
			all(types.Never)
		} else {
			all(types.Any)
		}
		return
	case v.IsNever():
		all(types.Never)
		return
	case v.Kinds() != 1 || v.Elem() == nil:
		c.diag("kind", bound)
		all(types.Never)
		return
	}
	elem := v.Elem()
	for _, e := range p.Elems {
		if e.Default != nil {
			d := c.synth(e.Default, fs)
			c.setSlot(e.Name, types.Join(types.Nonnull(elem), d))
			continue
		}
		c.site("index", e.Name.Span())
		c.setSlot(e.Name, elem)
	}
	if p.Rest != nil {
		c.setSlot(p.Rest, v)
	}
}

// neverEqual is the condition of never-equal (8.2, 8.4, 11.2; ruling AO5).
func neverEqual(a, b *types.Type) bool {
	return !a.IsNever() && !b.IsNever() && !types.Overlap(a, b) && !a.IsExactlyNull() && !b.IsExactlyNull()
}

// rendered checks a rendered expression (ruling AO2).
func (c *checker) rendered(n syntax.Node, t *types.Type) {
	switch {
	case types.HasOpaque(t):
		c.diag("kind", n.Span())
	case types.HasAny(t):
		c.dyn(n.Span())
	}
}
