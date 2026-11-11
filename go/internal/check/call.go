package check

import (
	"sort"

	"github.com/deepnoodle-ai/risor/go/v3/internal/library"
	"github.com/deepnoodle-ai/risor/go/v3/internal/static"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// chain walks a chain from left to right (9.1), by a loop.
func (c *checker) chain(n syntax.Node, fs facts) *types.Type {
	if oc, ok := n.(*syntax.OptChain); ok {
		n = oc.X
	}
	if m, ok := n.(*syntax.Member); ok {
		if _, ok := c.res.Consts[m]; ok { // F6 10 item 2
			return types.Float
		}
	}
	var nodes []syntax.Node
	x := n
loop:
	for {
		switch t := x.(type) {
		case *syntax.Member:
			if _, ok := c.res.Consts[t]; ok {
				break loop
			}
			nodes = append(nodes, t)
			x = t.X
		case *syntax.Index:
			nodes = append(nodes, t)
			x = t.X
		case *syntax.Each:
			nodes = append(nodes, t)
			x = t.X
		case *syntax.MethodCall:
			if c.res.NSCalls[t] {
				break loop
			}
			nodes = append(nodes, t)
			x = t.X
		default:
			break loop
		}
	}
	cur := c.synth(x, fs)
	// A path matters only where a fact is in force (10.4).
	var p path
	isPath := false
	if len(fs) > 0 {
		p, isPath = c.pathOf(x)
	}
	skipped := false
	var recv syntax.Node = x
	for i := len(nodes) - 1; i >= 0; i-- {
		s := nodes[i]
		rs := recv.Span()
		optional := false
		switch s := s.(type) {
		case *syntax.Member:
			optional = s.Optional
		case *syntax.Index:
			optional = s.Optional
		case *syntax.MethodCall:
			optional = s.Optional
		case *syntax.Each:
			optional = s.Optional
			if optional && cur.IsAny() { // F7 5.1: body?.[*] is any[] | null
				skipped = true
			}
		}
		if cur.HasNull() && !cur.IsAny() {
			if optional {
				skipped = true
			} else {
				c.m("", ty("type", cur)).diag("maybe-null", rs)
			}
			cur = types.Nonnull(cur)
		}
		if !cur.IsAny() && !cur.IsNever() && cur.Kinds() != 1 {
			c.m("receiver", ty("type", cur)).diag("kind", rs)
			cur = types.Never
		}
		switch s := s.(type) {
		case *syntax.Member:
			if isPath {
				p.keys = append(p.keys, s.Name)
			}
			cur = c.readKey(cur, s.Name, s.NameSpan, rs, optional, isPath && fs.covers(p))
			if isPath {
				cur = fs.apply(cur, p)
			}
		case *syntax.Index:
			idx := c.synth(s.Index, fs)
			if str, ok := s.Index.(*syntax.String); ok && cur.Record() != nil {
				if isPath {
					p.keys = append(p.keys, str.Value)
				}
				cur = c.readKey(cur, str.Value, s.Index.Span(), rs, optional, isPath && fs.covers(p))
				if isPath { // a fact applies to a path only (10.4)
					cur = fs.apply(cur, p)
				}
			} else {
				isPath = false
				cur = c.index(s, cur, idx, rs, optional)
			}
		case *syntax.MethodCall:
			isPath = false
			cur = c.method(s, recv, cur, rs, fs)
		case *syntax.Each:
			isPath = false
			cur = c.project(s, cur, rs, fs)
		}
		c.setTy(s, cur)
		recv = s
	}
	if skipped {
		cur = types.WithNull(cur)
	}
	return cur
}

// readKey is a field read of 9.2: .f, or [e] with a String node.
func (c *checker) readKey(cur *types.Type, key string, keySpan, recvSpan syntax.Span, optional, present bool) *types.Type {
	switch {
	case cur.IsAny():
		if c.dyn(recvSpan) {
			return types.Never
		}
		return types.Any
	case cur.IsNever():
		return types.Never
	case cur.Record() != nil:
		t := c.readField(cur.Record(), key, keySpan, recvSpan, optional, present)
		if present {
			t = types.Nonnull(t)
		}
		return t
	}
	c.m("field-read", ty("type", cur)).diag("kind", recvSpan)
	return types.Never
}

// index is [e] by 9.2, other than a field read.
func (c *checker) index(s *syntax.Index, cur, idx *types.Type, recvSpan syntax.Span, optional bool) *types.Type {
	isp := s.Index.Span()
	switch {
	case cur.IsAny():
		if c.dyn(recvSpan) {
			return types.Never // rejected: the receiver is never, no index site
		}
		// A plain index on an any receiver may fail as index-range.
		if !optional && c.site("index", s.Span()) {
			return types.Never
		}
		return types.Any
	case cur.IsNever():
		return types.Never
	case cur.Record() != nil:
		idx = c.nullable(isp, idx)
		if !c.mustFitM(isp, idx, types.String, "kind", "key", "") {
			return types.Never // one mistake, one diagnostic
		}
		r := cur.Record()
		v := r.Rest
		for _, f := range r.Fields {
			v = types.Join(v, f.Type)
		}
		if optional {
			return types.WithNull(v)
		}
		if r.Rest.IsAny() {
			if c.dyn(recvSpan) {
				return types.Never
			}
		} else {
			c.m("index", ty("type", cur)).diag("maybe-absent", isp)
		}
		return v
	case cur.Elem() != nil || cur.HasString():
		idx = c.nullable(isp, idx)
		c.mustFitM(isp, idx, types.Int, "kind", "index", "")
		t := types.String
		if cur.Elem() != nil {
			t = cur.Elem()
		}
		if optional {
			return types.WithNull(t)
		}
		c.site("index", s.Span())
		return t
	}
	c.m("index-read", ty("type", cur)).diag("kind", recvSpan)
	return types.Never
}

// lambdaArg reports whether an argument is a lambda argument (13.2), and
// gives its lambda.
func (c *checker) lambdaArg(a syntax.Node) (*syntax.Lambda, bool) {
	switch a := a.(type) {
	case *syntax.Lambda:
		return a, false
	case *syntax.Ident:
		if b := c.res.Idents[a]; b != nil && b.Kind == static.ConstLambda {
			return b.Lambda, true
		}
	}
	return nil, false
}

// synthPlain synthesizes the plain arguments of a call that is not checked
// further.
func (c *checker) synthPlain(args []syntax.Node, fs facts) {
	for _, a := range args {
		if l, _ := c.lambdaArg(a); l == nil {
			c.synth(a, fs)
		}
	}
}

// method resolves and checks a method call (13.3).
func (c *checker) method(m *syntax.MethodCall, recvNode syntax.Node, recv *types.Type, recvSpan syntax.Span, fs facts) *types.Type {
	switch {
	case recv.IsAny():
		rejected := c.dyn(recvSpan)
		for _, a := range m.Args {
			if l, named := c.lambdaArg(a); l != nil {
				// A const-bound lambda named here is a use: every
				// parameter any, sites kept, its body checked in the scope
				// and under the facts where it was written (13.9).
				if named {
					base, ok := c.use(a, l)
					if !ok {
						continue
					}
					info := c.lambdas[l]
					c.withLambda(l, info, base, func() {
						for _, p := range l.Params {
							c.bindParam(p, types.Any, true, info.facts)
						}
						c.synth(l.Body, info.facts)
					})
					continue
				}
				for _, p := range l.Params {
					c.bindParam(p, types.Any, true, fs)
				}
				c.synth(l.Body, fs)
				continue
			}
			c.synth(a, fs)
		}
		// 13.3: the call may fail by a library method or an opaque method of
		// that name marked !.
		sp := m.Span()
		if c.libFails(m.Name) && c.named(m).site("value", sp) {
			rejected = true
		}
		if c.hostFails(m.Name) && c.named(m).site("host", sp) {
			rejected = true
		}
		if rejected {
			return types.Never
		}
		return types.Any
	case recv.IsNever():
		c.synthPlain(m.Args, fs)
		return types.Never
	}
	var sig *types.Sig
	var entry *library.Entry
	switch {
	case recv.HasString():
		entry = c.libMethod(value.String, m.Name)
	case recv.Elem() != nil:
		entry = c.libMethod(value.List, m.Name)
	case recv.Record() != nil:
		entry = c.libMethod(value.Map, m.Name)
	case len(recv.OpaqueNames()) == 1:
		sig = c.env.Methods[recv.OpaqueNames()[0]][m.Name]
	}
	if entry != nil {
		sig = LibSig(entry)
	}
	if sig == nil {
		c.m("", ty("type", recv), word("name", m.Name)).diag("no-method", m.NameSpan)
		c.synthPlain(m.Args, fs)
		return types.Never
	}
	return c.callSig(m, sig, entry, recvNode, recv, m.Args, fs, true)
}

// call checks a Call, or a MethodCall that calls a namespace (5.3, 13).
func (c *checker) call(n syntax.Node, args []syntax.Node, fs facts) *types.Type {
	callee := c.res.Calls[n]
	if callee == nil {
		c.synthPlain(args, fs)
		return types.Never
	}
	switch callee.Kind {
	case static.Lambda:
		return c.callLambda(n, callee.Lambda, args, fs)
	case static.Host:
		return c.callSig(n, c.env.Hosts[callee.Host.FullName()], nil, nil, nil, args, fs, false)
	case static.Lib:
		return c.callSig(n, LibSig(callee.Lib), callee.Lib, nil, nil, args, fs, false)
	}
	c.synthPlain(args, fs)
	return types.Never
}

// use counts a use of a const-bound lambda against the limits of 20.2 and
// 20.3. It reports whether the use may be checked, and its expanded
// nesting base.
func (c *checker) use(at syntax.Node, l *syntax.Lambda) (int, bool) {
	c.uses++
	info := c.lambdas[l]
	base := c.expanded(at) + 1
	if c.uses > MaxUses || info == nil || base+info.maxRel > MaxNesting {
		c.m(useTmpl(c.uses), word("name", c.callName(at))).diag("check-limit", at.Span())
		return 0, false
	}
	return base, true
}

// callLambda is a call by name of a const-bound lambda (13.9).
func (c *checker) callLambda(n syntax.Node, l *syntax.Lambda, args []syntax.Node, fs facts) *types.Type {
	min := 0
	for i, p := range l.Params {
		if p.Default == nil {
			min = i + 1
		}
	}
	if len(args) < min || len(args) > len(l.Params) {
		c.synthPlain(args, fs)
		return types.Never
	}
	ts := make([]*types.Type, len(args))
	for i, a := range args {
		if lam, _ := c.lambdaArg(a); lam != nil {
			ts[i] = types.Never
			continue
		}
		ts[i] = c.synth(a, fs)
	}
	base, ok := c.use(n, l)
	if !ok {
		return types.Never
	}
	info := c.lambdas[l]
	var out *types.Type
	c.withLambda(l, info, base, func() {
		for i, p := range l.Params {
			if i < len(ts) {
				c.bindParam(p, ts[i], true, info.facts)
			} else {
				c.bindParam(p, types.Never, false, info.facts)
			}
		}
		out = c.synth(l.Body, info.facts)
	})
	return out
}

type plainArg struct {
	node syntax.Node
	t    *types.Type
	p    *types.Type
}

// bind gathers the solutions of the type variables of P from A (13.4).
func bind(p, a *types.Type, sol map[string]*types.Type) {
	if a.IsAny() {
		vs := map[string]bool{}
		types.VarsIn(p, vs)
		for v := range vs {
			gain(sol, v, types.Any)
		}
		return
	}
	switch {
	case len(p.Vars()) == 1 && p.Kinds() == 0:
		gain(sol, p.Vars()[0], a)
	case p.HasNull():
		bind(types.Nonnull(p), types.Nonnull(a), sol)
	case p.Elem() != nil && p.Kinds() == 1 && a.Elem() != nil:
		bind(p.Elem(), a.Elem(), sol)
	case p.Record() != nil && p.Kinds() == 1 && a.Record() != nil:
		pr, ar := p.Record(), a.Record()
		if len(pr.Fields) == 0 {
			j := ar.Rest
			for _, f := range ar.Fields {
				j = types.Join(j, f.Type)
			}
			bind(pr.Rest, j, sol)
			return
		}
		for _, f := range pr.Fields {
			if g, ok := ar.Field(f.Key); ok {
				bind(f.Type, g.Type, sol)
			}
		}
	}
}

func gain(sol map[string]*types.Type, v string, t *types.Type) {
	if s, ok := sol[v]; ok {
		sol[v] = types.Join(s, t)
	} else {
		sol[v] = t
	}
}

// callSig solves and checks one call of a library entry, a host function or
// a method (13.5). entry is nil for a host function or method.
func (c *checker) callSig(n syntax.Node, sig *types.Sig, entry *library.Entry, recvNode syntax.Node, recv *types.Type,
	args []syntax.Node, fs facts, isMethod bool) *types.Type {
	sp := n.Span()
	if len(args) < sig.Min() || (sig.Max() >= 0 && len(args) > sig.Max()) {
		if isMethod {
			c.m(static.ArityTemplate(sig.Min(), sig.Max()), word("name", c.callName(n)), num("count", len(args)), num("required", sig.Min()), num("most", sig.Max())).diag("arity", sp)
		}
		c.synthPlain(args, fs)
		return types.Never
	}
	sol := map[string]*types.Type{}
	if recv != nil && sig.Recv != nil {
		bind(sig.Recv, recv, sol)
	}
	var plains []plainArg
	type lamArg struct {
		node  syntax.Node
		lam   *syntax.Lambda
		named bool
		fn    *types.Fn
	}
	var lams []lamArg
	for i, a := range args {
		p := sig.ParamAt(i)
		if l, named := c.lambdaArg(a); l != nil {
			if p.Fn == nil {
				if isMethod {
					c.m("argument", word("name", c.callName(n)), word("param", p.Name)).diag("not-a-value", a.Span())
				}
				continue
			}
			lams = append(lams, lamArg{a, l, named, p.Fn})
			continue
		}
		t := c.synth(a, fs)
		if p.Fn != nil {
			c.m("function", word("param", p.Name), word("name", c.callName(n))).diag("arg-mismatch", a.Span())
			continue
		}
		if entry != nil {
			for _, ti := range entry.Tested {
				if ti == i {
					c.tested(a, t)
				}
			}
			for _, rn := range entry.Renders {
				if rn == p.Name {
					c.rendered(a, t)
				}
			}
		}
		bind(p.Type, t, sol)
		plains = append(plains, plainArg{a, t, p.Type})
	}
	// 20.7: each solution is tested when it is made; over the limit, the
	// call is not checked further.
	solOver := func() bool {
		for _, t := range sol {
			if over(t) {
				c.m(limitTmpl(t)).diag("check-limit", sp)
				return true
			}
		}
		return false
	}
	if solOver() {
		return types.Never
	}
	var narrowed *types.Type
	for _, la := range lams {
		c.curCall = n
		n := c.checkLambda(la.node, la.lam, la.named, la.fn, sol, fs, isMethod, entry != nil && entry.Narrows != "")
		if n != nil {
			narrowed = n
		}
		if solOver() {
			return types.Never
		}
	}
	vars := make([]string, 0, len(sig.Bounds))
	for v := range sig.Bounds {
		vars = append(vars, v)
	}
	sort.Strings(vars)
	for _, v := range vars {
		s := sol[v]
		if s == nil {
			s = types.Never
		}
		okF, usedF := types.Fits(s, types.Float)
		ok, used := okF, usedF
		if sig.Bounds[v] == "ordered" && !okF {
			ok, used = types.Fits(s, types.String)
		}
		if !ok {
			c.m(boundTmpl(sig.Bounds[v]), word("name", c.callName(n)), ty("found", s)).diag("arg-mismatch", sp)
		} else if used {
			c.dyn(sp)
		}
	}
	if recv != nil && sig.Recv != nil {
		c.fitForm(recvNode, recv, types.Subst(sig.Recv, sol), formArg)
	}
	for _, pa := range plains {
		c.fitForm(pa.node, pa.t, types.Subst(pa.p, sol), formArg)
	}
	res := types.Subst(sig.Result, sol)
	if narrowed != nil {
		if entry.Name == "filter" {
			res = types.List(narrowed)
		} else {
			res = types.WithNull(narrowed)
		}
	}
	if sig.Fails {
		if entry != nil {
			c.named(n).site("value", sp)
		} else {
			c.named(n).site("host", sp)
		}
	}
	return c.depthLimit(n, res)
}

// checkLambda checks a lambda argument against its Fn (13.5 step 3). With
// narrows, it returns the narrowed element type of 13.8, or nil.
func (c *checker) checkLambda(at syntax.Node, l *syntax.Lambda, named bool, fn *types.Fn, sol map[string]*types.Type,
	fs facts, isMethod, narrows bool) *types.Type {
	name := c.curCall
	if len(l.Params) > len(fn.Params) {
		if isMethod {
			c.m("parameters", word("name", c.callName(name)), num("most", len(fn.Params))).diag("arity", at.Span())
		}
		return nil
	}
	qs := make([]*types.Type, len(fn.Params))
	for i, q := range fn.Params {
		vs := map[string]bool{}
		types.VarsIn(q, vs)
		for v := range vs {
			if _, ok := sol[v]; !ok && i < len(l.Params) {
				c.m("", word("name", c.callName(name))).diag("cannot-infer", at.Span())
				return nil
			}
		}
		qs[i] = types.Subst(q, sol)
	}
	lf := fs
	base, depth := c.base, c.lamDepth
	if named {
		b, ok := c.use(at, l)
		if !ok {
			return nil
		}
		info := c.lambdas[l]
		lf, base, depth = info.facts, b, l.Depth()
	}
	var narrowed *types.Type
	check := func() *types.Type {
		sb, sd := c.base, c.lamDepth
		c.base, c.lamDepth = base, depth
		defer func() { c.base, c.lamDepth = sb, sd }()
		for i, p := range l.Params {
			c.bindParam(p, qs[i], true, lf)
		}
		b := c.synth(l.Body, lf)
		if narrows && len(l.Params) > 0 {
			if np, ok := l.Params[0].Pattern.(*syntax.NamePat); ok && !np.Discard && l.Params[0].Default == nil {
				if slot, ok := c.res.Slots[np]; ok && slot >= 0 {
					t := c.slots[slot]
					tf, _ := c.factsOf(l.Body)
					if tf.covers(path{root: slot}) {
						t = types.Nonnull(t)
					}
					t = tf.apply(t, path{root: slot}) // F4 6.5
					narrowed = t
				}
			}
		}
		return b
	}
	dl, sl := len(c.diags), len(c.sites)
	b := check()
	if fn.Result == nil {
		c.tested(l.Body, b)
		return narrowed
	}
	r := fn.Result
	rv := map[string]bool{}
	types.VarsIn(r, rv)
	// The seed rule (13.5 step 3a): it fires also when B fits
	// A0 only by a fit that used any. The second check binds nothing.
	vars := make([]string, 0, len(rv))
	for x := range rv {
		vars = append(vars, x)
	}
	sort.Strings(vars)
	for _, x := range vars {
		inQ := false
		for _, q := range fn.Params {
			vs := map[string]bool{}
			types.VarsIn(q, vs)
			inQ = inQ || vs[x]
		}
		a0, had := sol[x]
		if inQ && had {
			if ok, used := types.Fits(b, a0); !ok || used {
				c.diags, c.sites = c.diags[:dl], c.sites[:sl]
				sol[x] = types.Join(a0, b)
				for i, q := range fn.Params {
					qs[i] = types.Subst(q, sol)
				}
				b = check()
				c.mustFitM(l.Body.Span(), b, types.Subst(r, sol), "arg-mismatch", "body", c.callName(name))
				return narrowed
			}
		}
	}
	bind(r, b, sol)
	// The body is always fitted (13.5 step 3).
	c.mustFitM(l.Body.Span(), b, types.Subst(r, sol), "arg-mismatch", "body", c.callName(name))
	return narrowed
}

type formKind uint8

const (
	formResult formKind = iota
	formArg
)

// fitForm is "e fits T through its form" (15.1), with the code of 15.2 at
// each place of failure.
func (c *checker) fitForm(e syntax.Node, t, want *types.Type, k formKind) {
	ok, used := types.Fits(t, want)
	if ok {
		if used {
			c.dyn(e.Span())
		}
		return
	}
	// fail reports at sp: the value of type at does not fit w; with at
	// nil, required field key has no entry; with w never, the record
	// expected does not allow key.
	fail := func(sp syntax.Span, at *types.Type, w *types.Type, expected *types.Type, key string) {
		code, tmpl := "arg-mismatch", "argument"
		if k == formResult {
			code, tmpl = "result-mismatch", "result"
		} else if at != nil && at.HasNull() && fits(types.Nonnull(at), w) {
			c.m("", ty("type", at)).diag("maybe-null", sp)
			return
		}
		switch {
		case at == nil:
			c.m("missing", keyP(key))
		case expected != nil:
			c.m("extra", ty("expected", expected), keyP(key))
		default:
			c.m(tmpl, ty("expected", w), ty("found", at))
		}
		c.diag(code, sp)
	}
	tyOf := func(n syntax.Node) *types.Type {
		if t, ok := c.ty[n]; ok {
			return t
		}
		return types.Never
	}
	switch e := e.(type) {
	case *syntax.List:
		if el := want.Elem(); el != nil {
			for _, it := range e.Items {
				if s, ok := it.(*syntax.Spread); ok {
					st := tyOf(s)
					if ok, used := types.Fits(st, types.List(el)); !ok {
						fail(s.Value.Span(), st, types.List(el), nil, "")
					} else if used {
						c.dyn(s.Value.Span())
					}
					continue
				}
				c.fitForm(it, tyOf(it), el, k)
			}
			return
		}
	case *syntax.Map:
		if noComputed(e) {
			if u := want.Union(); u != nil {
				c.fitMapUnion(e, c.givers(e), u, k, fail)
				return
			}
			if r := want.Record(); r != nil {
				c.fitMapRecord(e, c.givers(e), r, want, k, fail)
				return
			}
		}
	case *syntax.Cond:
		for {
			c.fitForm(e.Then, tyOf(e.Then), want, k)
			next, ok := e.Else.(*syntax.Cond)
			if !ok {
				c.fitForm(e.Else, tyOf(e.Else), want, k)
				return
			}
			e = next
		}
	case *syntax.Match:
		for _, a := range e.Arms {
			c.fitForm(a.Result, tyOf(a.Result), want, k)
		}
		return
	case *syntax.Block:
		c.fitForm(e.Result, tyOf(e.Result), want, k)
		return
	}
	fail(e.Span(), t, want, nil, "")
}

// libMethod is the library method of a kind by name, unless the environment
// removed it (03-host.md 3.3.1).
func (c *checker) libMethod(k value.Kind, name string) *library.Entry {
	e, ok := library.Method(k, name)
	if !ok || c.env.Removed[library.KindWord(k)+"."+name] {
		return nil
	}
	return e
}

// libFails reports whether some library method of that name, for any
// receiver kind and not removed, is marked ! (13.3; 03-host.md 3.3.1).
func (c *checker) libFails(name string) bool {
	for _, k := range library.MethodKinds(name) {
		if e := c.libMethod(k, name); e != nil && LibSig(e).Fails {
			return true
		}
	}
	return false
}

// hostFails reports whether a method of that name of a declared opaque type
// is marked ! (13.3).
func (c *checker) hostFails(name string) bool {
	for _, ms := range c.env.Methods {
		if s := ms[name]; s != nil && s.Fails {
			return true
		}
	}
	return false
}

// project checks a projection (f07-queries.md 5.1, 5.2). cur is the type of
// the first star's receiver after steps 2 to 4 of 9.1, and rs its span.
func (c *checker) project(e *syntax.Each, cur *types.Type, rs syntax.Span, fs facts) *types.Type {
	failed, skippedElem := false, false
	// star gives the element type of a star's receiver.
	star := func(cur *types.Type, rs syntax.Span) *types.Type {
		switch {
		case cur.IsAny():
			if c.dyn(rs) {
				failed = true
				return types.Never
			}
			return types.Any
		case cur.IsNever():
			return types.Never
		case cur.Kinds() == 1 && cur.Elem() != nil:
			return cur.Elem()
		}
		c.m("star", ty("type", cur)).diag("kind", rs)
		failed = true
		return types.Never
	}
	never := cur.IsNever()
	el := star(cur, rs)
	lv := e
	for {
		recvSpan := lv.StarSpan
		var next *syntax.Each
		for _, rd := range lv.Reads {
			if nx, ok := rd.(*syntax.Each); ok {
				next = nx
				break
			}
			if never || failed {
				if ix, ok := rd.(*syntax.Index); ok {
					c.synth(ix.Index, fs)
				}
				continue
			}
			optional := false
			switch rd := rd.(type) {
			case *syntax.Member:
				optional = rd.Optional
			case *syntax.Index:
				optional = rd.Optional
			}
			if el.HasNull() && !el.IsAny() {
				if optional {
					skippedElem = true
				} else {
					c.m("", ty("type", el)).diag("maybe-null", recvSpan)
				}
				el = types.Nonnull(el)
			}
			if !el.IsAny() && !el.IsNever() && el.Kinds() != 1 {
				c.m("receiver", ty("type", el)).diag("kind", recvSpan)
				failed = true
				continue
			}
			before, nd := el, len(c.diags)
			switch rd := rd.(type) {
			case *syntax.Member:
				el = c.readKey(el, rd.Name, rd.NameSpan, recvSpan, optional, false)
			case *syntax.Index:
				idx := c.synth(rd.Index, fs)
				if str, ok := rd.Index.(*syntax.String); ok && el.Record() != nil {
					el = c.readKey(el, str.Value, rd.Index.Span(), recvSpan, optional, false)
				} else {
					el = c.index(rd, el, idx, recvSpan, optional)
				}
			}
			// Only a diagnostic that leaves never sets failed (f07-queries.md
			// 5.1): a field whose type is never gives never[] (5.2 item 2).
			if el.IsNever() && !before.IsNever() && len(c.diags) > nd {
				failed = true
			}
			c.setTy(rd, el)
			recvSpan = rd.Span()
		}
		if next == nil {
			break
		}
		if !never && !failed {
			if el.HasNull() && !el.IsAny() {
				if next.Optional {
					skippedElem = true
				} else {
					c.m("", ty("type", el)).diag("maybe-null", recvSpan)
				}
				el = types.Nonnull(el)
			} else if next.Optional && el.IsAny() {
				skippedElem = true
			}
			if !el.IsAny() && !el.IsNever() && el.Kinds() != 1 {
				c.m("receiver", ty("type", el)).diag("kind", recvSpan)
				failed = true
			} else {
				el = star(el, recvSpan)
			}
		}
		lv = next
	}
	if never || failed {
		return types.Never
	}
	if skippedElem {
		el = types.WithNull(el)
	}
	return types.List(el)
}
