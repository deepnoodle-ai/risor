package check

import (
	"github.com/deepnoodle-ai/risor/go/v3/internal/static"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// synth checks a node and gives its type (1.1).
func (c *checker) synth(n syntax.Node, fs facts) *types.Type {
	t := c.synth0(n, fs)
	c.setTy(n, t)
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
	case *syntax.Member, *syntax.Index, *syntax.MethodCall, *syntax.OptChain, *syntax.Each:
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
	case *syntax.As:
		return c.as(n, fs)
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
			return orFacts(td, te), union(fd, fe)
		case "in", "not-in":
			// F5 5.2: q in [L1, ..., Ln] gives the facts of q == L1 || ...
			l, ok := n.Y.(*syntax.List)
			if !ok || len(l.Items) == 0 {
				return nil, nil
			}
			var ss []string
			for _, it := range l.Items {
				s, ok := it.(*syntax.String)
				if !ok {
					return nil, nil
				}
				ss = append(ss, s.Value)
			}
			t, f = c.fieldFacts(n.X, ss)
			if n.Op == "not-in" {
				return f, t
			}
			return t, f
		case "==", "!=":
			if s, ok := n.Y.(*syntax.String); ok {
				if t, f, ok := c.fieldFactsOk(n.X, []string{s.Value}); ok {
					if n.Op == "!=" {
						return f, t
					}
					return t, f
				}
			}
			if s, ok := n.X.(*syntax.String); ok {
				if t, f, ok := c.fieldFactsOk(n.Y, []string{s.Value}); ok {
					if n.Op == "!=" {
						return f, t
					}
					return t, f
				}
			}
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
				return facts{nn(p)}, c.nullFacts(p) // F14 4.2
			}
			return c.nullFacts(p), facts{nn(p)}
		}
	case *syntax.As:
		// F4 6.4: the facts of p is T.
		t := c.res.AsTypes[n]
		if n.Form != "is" || t == nil {
			return nil, nil
		}
		p, ok := c.pathOf(n.X)
		if !ok {
			return nil, nil
		}
		tf := facts{{p: p, kind: factConforms, t: t}}
		ff := facts{{p: p, kind: factNotConforms, t: t}}
		if !t.HasNull() && !t.IsAny() {
			tf = append(tf, nn(p))
		}
		if t.IsExactlyNull() {
			tf = append(tf, c.nullFacts(p)...) // F14 4.2
		}
		if t.HasNull() {
			ff = append(ff, nn(p))
		}
		return tf, ff
	}
	if p, ok := c.pathOf(n); ok {
		return facts{nn(p)}, nil
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
	if len(fs) > 0 {
		if p, ok := c.pathOf(n); ok {
			if fs.covers(p) {
				t = types.Nonnull(t)
			}
			t = fs.apply(t, p)
		}
	}
	// F14 5.1: the effect, applied last.
	if b.Kind == static.EnvVar && len(c.env.Facts) > 0 && c.someNotNull(b.Var, fs) {
		t = types.Nonnull(t)
	}
	return t
}

// depthLimit tests a type that becomes the type of node n against the
// depth limit and the size limit (20.7): over either, it is
// check-limit on n and the node has type never. Both tests are constant
// work: the depth and the size are kept with the type.
func (c *checker) depthLimit(n syntax.Node, t *types.Type) *types.Type {
	return c.limitAt(n.Span(), t)
}

func (c *checker) limitAt(sp syntax.Span, t *types.Type) *types.Type {
	if types.Size(t) > types.MaxSize || types.Depth(t) > types.MaxDepth {
		c.m(limitTmpl(t)).diag("check-limit", sp)
		return types.Never
	}
	return t
}

// over reports whether a type is past the size limit. A Join that would
// go on from such a type stops, and the test of its node reports it.
func over(t *types.Type) bool { return types.Size(t) > types.MaxSize }

func (c *checker) list(n *syntax.List, fs facts) *types.Type {
	// The item types are joined once, at the end, so the work is not quadratic.
	var elems []*types.Type
	for _, it := range n.Items {
		if s, ok := it.(*syntax.Spread); ok {
			t := c.synth(s.Value, fs)
			c.setTy(s, t)
			sp := s.Value.Span()
			t = c.nullable(sp, t)
			switch {
			case t.IsAny():
				if !c.dyn(sp) {
					elems = append(elems, types.Any)
				}
			case t.IsNever():
			case t.Kinds() == 1 && t.Elem() != nil:
				elems = append(elems, t.Elem())
			default:
				c.m("spread-list", ty("type", t)).diag("kind", sp)
			}
			continue
		}
		elems = append(elems, c.synth(it, fs))
	}
	return c.depthLimit(n, types.List(types.JoinAll(elems)))
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
				c.mustFitM(ksp, kt, types.String, "kind", "key", "")
				v := c.synth(e.Value, fs)
				c.setTy(e, v)
				rec = types.Merge(rec, types.MapOf(v))
				continue
			}
			v := c.synth(e.Value, fs)
			c.setTy(e, v)
			rec = types.SetField(rec, e.Key, v)
		case *syntax.Spread:
			t := c.synth(e.Value, fs)
			c.setTy(e, t)
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
				c.m("spread-map", ty("type", t)).diag("kind", sp)
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
		c.m("operand", word("op", n.Op), ty("type", t)).diag("kind", sp)
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
		c.setTy(spine[i], t)
	}
	return t
}

func (c *checker) binop(n *syntax.Binary, a *types.Type, fs facts) *types.Type {
	switch n.Op {
	case "&&":
		c.tested(n.X, a)
		tf, _ := c.factsOf(n.X)
		b := c.synth(n.Y, fs.with(tf))
		return c.depthLimit(n, types.Join(types.Falsy(a), b))
	case "||":
		c.tested(n.X, a)
		_, ff := c.factsOf(n.X)
		b := c.synth(n.Y, fs.with(ff))
		return c.depthLimit(n, types.Join(types.Truthy(a), b))
	case "??":
		b := c.synth(n.Y, fs.with(c.nullOf(n.X))) // F14 4.4: b under N(a)
		return c.depthLimit(n, types.Join(types.Nonnull(a), b))
	case "==", "!=":
		b := c.synth(n.Y, fs)
		if neverEqual(a, b) {
			c.m("equality", ty("left", a), ty("right", b)).diag("never-equal", n.Span())
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
			c.m("operand", word("op", n.Op), ty("type", a)).diag("kind", xs)
			bad = true
		}
		if !b.IsAny() && !fb && !sb {
			c.m("operand", word("op", n.Op), ty("type", b)).diag("kind", ys)
			bad = true
		}
		if !bad && !anyOp && ((fa && !sa && sb && !fb) || (sa && !fa && fb && !sb)) {
			c.m("operands", word("op", n.Op), ty("left", a), ty("right", b)).diag("kind", n.Span())
			bad = true
		}
	case "**":
		if !a.IsAny() && !fits(a, types.Float) {
			c.m("operand", word("op", n.Op), ty("type", a)).diag("kind", xs)
			bad = true
		}
		if !b.IsAny() && !fits(b, types.Int) {
			c.m("exponent", ty("type", b)).diag("kind", ys)
			bad = true
		}
	default: // - * / %
		if !a.IsAny() && !fits(a, types.Float) {
			c.m("operand", word("op", n.Op), ty("type", a)).diag("kind", xs)
			bad = true
		}
		if !b.IsAny() && !fits(b, types.Float) {
			c.m("operand", word("op", n.Op), ty("type", b)).diag("kind", ys)
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
			c.m("in", ty("left", a), ty("right", b)).diag("never-equal", n.Span())
		}
	case b.Kinds() == 1 && (b.HasString() || b.Record() != nil):
		a = c.nullable(xs, a)
		c.mustFitM(xs, a, types.String, "kind", "in-left", opWord(n.Op))
	default:
		c.m("in-right", word("op", opWord(n.Op)), ty("type", b)).diag("kind", ys)
	}
	return types.Bool
}

// cond checks ?: and an if chain (11.1), the else chain by a loop.
func (c *checker) cond(n *syntax.Cond, fs facts) *types.Type {
	// An else-if chain is checked by a loop. Each Cond's type is the Join
	// of the branches from it on, tested from the innermost out (20.7).
	var chain []*syntax.Cond
	var thens []*types.Type
	acc := fs
	var last *types.Type
	for {
		tt := c.synth(n.Test, acc)
		c.tested(n.Test, tt)
		tf, ff := c.factsOf(n.Test)
		chain = append(chain, n)
		thens = append(thens, c.synth(n.Then, acc.with(tf)))
		acc = acc.with(ff)
		next, ok := n.Else.(*syntax.Cond)
		if !ok {
			last = c.synth(n.Else, acc)
			break
		}
		n = next
	}
	// When the Join of every branch is within the limits, so is the Join
	// of the branches from any Cond on: one Join serves the chain.
	all := types.JoinAll(append(thens, last))
	if types.Size(all) <= types.MaxSize && types.Depth(all) <= types.MaxDepth {
		for _, cn := range chain {
			c.setTy(cn, all)
		}
		return all
	}
	t := last
	for i := len(chain) - 1; i >= 0; i-- {
		// Both parts were tested where they became types, so this Join
		// works on types within the limit.
		t = c.depthLimit(chain[i], types.Join(thens[i], t))
		c.setTy(chain[i], t)
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
	var arms []*types.Type // joined once, at the end
	af := fs
	var seen []syntax.Node
	// F5 5.3: a match on a field of a union narrows by each arm's strings.
	fp, fkey, mi, fok := c.fieldTest(n.Subject)
	for _, a := range n.Arms {
		listsNull := false
		kept, keptNull := 0, false // the literals 11.2 does not ignore
		var strs []string
		allStrings := !a.Default && len(a.Literals) > 0
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
			kept++
			keptNull = keptNull || isNull
			if _, isStr := l.(*syntax.String); !isStr {
				allStrings = false
			}
			if !isNull && neverEqual(litType(l), s) {
				c.m("match", ty("type", s)).diag("never-equal", l.Span())
				continue
			}
			if str, isStr := l.(*syntax.String); isStr {
				strs = append(strs, str.Value)
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
		armFs := af
		if isPath && kept == 1 && keptNull && !a.Default {
			armFs = armFs.with(c.nullFacts(p)) // F14 4.3: the arm of null alone
		}
		if fok && allStrings && len(strs) > 0 {
			if h := types.Hit(mi, fkey, strs); !h.IsNever() {
				armFs = af.with(facts{{p: fp, kind: factConforms, t: h, field: true}, nn(fp)})
			}
		}
		arms = append(arms, c.synth(a.Result, armFs))
		if fok && len(strs) > 0 {
			if at := types.All(mi, fkey, strs); !at.IsNever() {
				af = af.with(facts{{p: fp, kind: factNotConforms, t: at, field: true}})
				mi = types.MarkTested(types.WithoutType(mi, at))
			}
		}
		if listsNull && isPath {
			af = af.with(facts{nn(p)})
		}
	}
	if !hasDefault && !exhaustive(s, covered) {
		c.m("", ty("type", s)).diag("non-exhaustive", n.Span())
	}
	return c.depthLimit(n, types.JoinAll(arms))
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
	return c.depthLimit(n, types.Join(b, h))
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

// maxRel is d(L) of 20.2: the greatest number of levels open at once
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
			t = c.limitAt(p.Pattern.Span(), types.Join(types.Nonnull(a), d))
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
		c.m("field", ty("type", types.RecType(r)), keyP(key)).diag("maybe-absent", keySpan)
		return f.Type
	}
	if present && !r.Rest.IsNever() {
		return r.Rest // a fact says the key is present (9.2, 10.4)
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
		c.m("", ty("type", types.RecType(r)), keyP(key)).diag("no-field", keySpan)
		return types.Never
	}
	if present {
		return r.Rest
	}
	if optional {
		return types.WithNull(r.Rest)
	}
	c.m("field", ty("type", types.RecType(r)), keyP(key)).diag("maybe-absent", keySpan)
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
		c.m("pattern-map", ty("type", v)).diag("kind", bound)
		all(types.Never)
		return
	}
	r := v.Record()
	named := map[string]bool{}
	for _, f := range p.Fields {
		named[f.Key] = true
		var t *types.Type
		// A site of an unlisted key of an open record is on the field's key
		// (12.2).
		if f.Default != nil {
			rt := c.readField(r, f.Key, f.KeySpan, f.KeySpan, true, false)
			d := c.synth(f.Default, fs)
			t = c.limitAt(bound, types.Join(types.Nonnull(rt), d))
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
		// The index site on each element without a default (12.2).
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
		c.m("pattern-list", ty("type", v)).diag("kind", bound)
		all(types.Never)
		return
	}
	elem := v.Elem()
	for _, e := range p.Elems {
		if e.Default != nil {
			d := c.synth(e.Default, fs)
			c.setSlot(e.Name, c.limitAt(bound, types.Join(types.Nonnull(elem), d)))
			continue
		}
		c.site("index", e.Name.Span())
		c.setSlot(e.Name, elem)
	}
	if p.Rest != nil {
		c.setSlot(p.Rest, v)
	}
}

// neverEqual is the condition of never-equal (8.2, 8.4, 8.5, 11.2).
func neverEqual(a, b *types.Type) bool {
	return !a.IsNever() && !b.IsNever() && !types.Overlap(a, b) && !a.IsExactlyNull() && !b.IsExactlyNull()
}

// rendered checks a rendered expression (7.3).
func (c *checker) rendered(n syntax.Node, t *types.Type) {
	switch {
	case types.HasOpaque(t):
		c.m("rendered", ty("type", t)).diag("kind", n.Span())
	case types.HasAny(t):
		c.dyn(n.Span())
	}
}

// as checks x as T and x is T (package F4 6.3).
func (c *checker) as(n *syntax.As, fs facts) *types.Type {
	s := c.synth(n.X, fs)
	t := c.res.AsTypes[n]
	is := n.Form == "is"
	result := t
	if is {
		result = types.Bool
	}
	if t == nil { // a diagnostic of 6.1
		if is {
			return types.Bool
		}
		return types.Never
	}
	constant := func(code string) {
		if c.env.Reject["constant-condition"] {
			c.diags = append(c.diags, Diag{Code: code, Span: n.Span(), Class: "constant-condition", Tmpl: "test", Params: params(ty("type", s))})
		}
	}
	switch {
	case s.IsNever():
	case types.SurelyFits(s, t):
		if is {
			constant("always-truthy")
		}
	case types.Disjoint(s, t):
		if is {
			constant("always-falsy")
		} else {
			c.m("", ty("found", s), ty("expected", t)).diag("never-conforms", n.Span())
		}
	default:
		if !is && c.site("value", n.Span()) {
			return types.Never
		}
	}
	return result
}

// fieldTest finds the parts of a test of a field (F5 5.2 item 1, item 3):
// p, the path q is without its last step; f, the key that step names; and
// the type of p where the test stands, when it is a union or a record that a
// test of a field narrowed.
func (c *checker) fieldTest(q syntax.Node) (p path, f string, m *types.Type, ok bool) {
	if oc, isOC := q.(*syntax.OptChain); isOC {
		q = oc.X
	}
	var x syntax.Node
	switch t := q.(type) {
	case *syntax.Member:
		f, x = t.Name, t.X
	case *syntax.Index:
		s, isS := t.Index.(*syntax.String)
		if !isS {
			return p, "", nil, false
		}
		f, x = s.Value, t.X
	default:
		return p, "", nil, false
	}
	p, ok = c.pathOf(x)
	if !ok {
		return p, "", nil, false
	}
	m = c.ty[x]
	if m == nil || m.Record() == nil || (m.Union() == nil && !m.Tested()) {
		return p, "", nil, false
	}
	return p, f, m, true
}

// fieldFactsOk gives the facts of q == L for each string of ss, combined by
// ||: {p: H} and {p} when it held, {p: not A} for each string when it failed
// (F5 5.2).
func (c *checker) fieldFactsOk(q syntax.Node, ss []string) (t, f facts, ok bool) {
	p, key, m, ok := c.fieldTest(q)
	if !ok {
		return nil, nil, false
	}
	if h := types.Hit(m, key, ss); !h.IsNever() { // item 6: no fact with no variant
		t = facts{{p: p, kind: factConforms, t: h, field: true}, nn(p)}
	}
	if a := types.All(m, key, ss); !a.IsNever() {
		f = facts{{p: p, kind: factNotConforms, t: a, field: true}}
	}
	return t, f, true
}

func (c *checker) fieldFacts(q syntax.Node, ss []string) (t, f facts) {
	t, f, _ = c.fieldFactsOk(q, ss)
	return t, f
}
