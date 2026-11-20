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
	// The last step of a coalesced read under ?? reads a record by the
	// column of a coalesced read (9.2), which is the column "Optional" for
	// the key; its receiver is tested as for a plain step.
	coalesced := len(nodes) > 0 && c.res.Coalesced[nodes[0]]
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
		keyOptional := optional || (coalesced && i == 0)
		switch s := s.(type) {
		case *syntax.Member:
			if isPath {
				p.keys = append(p.keys, s.Name)
			}
			cur = c.readKey(cur, s.Name, s.NameSpan, rs, keyOptional, isPath && fs.covers(p))
			if isPath {
				cur = fs.apply(cur, p)
			}
		case *syntax.Index:
			idx := c.synth(s.Index, fs)
			if str, ok := s.Index.(*syntax.String); ok && cur.Record() != nil {
				if isPath {
					p.keys = append(p.keys, str.Value)
				}
				cur = c.readKey(cur, str.Value, s.Index.Span(), rs, keyOptional, isPath && fs.covers(p))
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

// fnArg gives the binding of an argument that names a parameter with a
// function type, or nil (f17-recursion.md 13.1).
func (c *checker) fnArg(a syntax.Node) *static.Binding {
	if id, ok := a.(*syntax.Ident); ok {
		if b := c.res.Idents[id]; b != nil && b.Kind == static.FnParam {
			return b
		}
	}
	return nil
}

// fnOf gives the types of the function type of a parameter: the type of
// each of its parameters and its result. A type with a static diagnostic
// is nil: it is fitted by nothing (F16 3.3 item 2). unknown is true when
// the result is unknown.
func (c *checker) fnOf(p *syntax.Param) (ps []*types.Type, r *types.Type, unknown bool) {
	for _, q := range p.Fn.Params {
		ps = append(ps, c.res.AnnTypes[q.Type])
	}
	if p.Fn.Result == nil {
		return ps, nil, true
	}
	return ps, c.res.AnnTypes[p.Fn.Result], false
}

// orNever is t, or never for a type with a static diagnostic.
func orNever(t *types.Type) *types.Type {
	if t == nil {
		return types.Never
	}
	return t
}

// synthPlain synthesizes the plain arguments of a call that is not checked
// further.
func (c *checker) synthPlain(args []syntax.Node, fs facts) {
	for _, a := range args {
		if l, _ := c.lambdaArg(a); l == nil && c.fnArg(a) == nil {
			c.synth(a, fs)
		}
	}
}

// fnParamPassed fits the values a function passes to a parameter with a
// function type that is given as a lambda argument: each must surely fit
// the type its function type gives that parameter, at that type (F16 3.3;
// f17-recursion.md 13.3). qs are the types passed, as many as it lists.
func (c *checker) fnParamPassed(b *static.Binding, qs []*types.Type) {
	gp, _, _ := c.fnOf(b.Param)
	for i, g := range gp {
		if g == nil || i >= len(qs) || qs[i] == nil {
			continue
		}
		if !fitsK(qs[i], g, formAnnotArg) {
			c.m("argument", ty("expected", g), ty("found", qs[i])).diag("arg-mismatch", b.Param.Fn.Params[i].Type.Span)
		}
	}
}

// fnParamResult is the result type of a parameter with a function type,
// as a function it is passed to sees it: any for unknown, never for a type
// with a static diagnostic.
func (c *checker) fnParamResult(b *static.Binding) *types.Type {
	_, r, unknown := c.fnOf(b.Param)
	if unknown {
		return types.Any
	}
	return orNever(r)
}

// method resolves and checks a method call (13.3).
func (c *checker) method(m *syntax.MethodCall, recvNode syntax.Node, recv *types.Type, recvSpan syntax.Span, fs facts) *types.Type {
	switch {
	case recv.IsAny():
		rejected := c.dyn(recvSpan)
		for _, a := range m.Args {
			if b := c.fnArg(a); b != nil {
				// Every value passed is any (f17-recursion.md 13.3).
				gp, _, _ := c.fnOf(b.Param)
				qs := make([]*types.Type, len(gp))
				for i := range qs {
					qs[i] = types.Any
				}
				c.fnParamPassed(b, qs)
				continue
			}
			if l, named := c.lambdaArg(a); l != nil {
				if static.HigherOrder(l) {
					continue // lambda-to-lambda, a static rule (F17 13.2 rule 8)
				}
				if named && c.res.SelfRefs[a] && !l.Typed() {
					continue // a reference to its own name is no use (13.9)
				}
				// An annotated parameter is given any, which fits it only
				// by a fit that used any (F16 3.4).
				for _, p := range l.Params {
					c.passed(p, types.Any, m.Name)
				}
				// A const-bound lambda named here is a use: every
				// parameter any, sites kept, its body checked in the scope
				// and under the facts where it was written (13.9).
				if named {
					if info := c.lambdas[l]; info != nil && info.once {
						continue // checked once, where it was written (F16 3.4)
					}
					base, ok := c.use(a, l)
					if !ok {
						continue
					}
					info := c.lambdas[l]
					c.withLambda(l, info, base, func() {
						c.bindArgs(l, nil, types.Any, info.facts)
						c.resultOf(l, c.synth(l.Body, info.facts), formAnnot)
					})
					continue
				}
				c.bindArgs(l, nil, types.Any, fs)
				c.resultOf(l, c.synth(l.Body, fs), formAnnot)
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
	case static.Param:
		return c.callFnParam(callee.Binding, args, fs)
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
	if c.res.SelfRefs[n] && !l.Typed() {
		// A reference to its own name (recursion-needs-types) is no use:
		// its plain arguments are synthesized, and it has type never. A
		// lambda whose parameters and result are typed is checked once,
		// and its call of itself is checked as any call of it (F17 11.3).
		c.synthPlain(args, fs)
		return types.Never
	}
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
		p := l.Params[i]
		if lam, _ := c.lambdaArg(a); lam != nil || c.fnArg(a) != nil {
			ts[i] = types.Never
			// A lambda argument at a parameter with a function type is
			// checked against it (f17-recursion.md 13.3). Elsewhere it is
			// lambda-to-lambda, a static rule, and not checked.
			if p.Fn != nil && !static.HigherOrder(lam) {
				c.userFnArg(p, a, fs, c.callName(n))
			}
			continue
		}
		ts[i] = c.synth(a, fs)
		// An argument at an annotated parameter must fit it (F16 3.4). A
		// value at a parameter with a function type is arg-mismatch, a
		// static rule (f17-recursion.md 13.2 rule 3).
		c.fitParam(p, a, ts[i], c.callName(n))
	}
	info := c.lambdas[l]
	if info != nil && info.once {
		return info.result // checked once, where it was written (F16 3.4)
	}
	base, ok := c.use(n, l)
	if !ok {
		return types.Never
	}
	var out *types.Type
	c.withLambda(l, info, base, func() {
		c.bindArgs(l, ts, nil, info.facts)
		out = c.resultOf(l, c.synth(l.Body, info.facts), formAnnot)
	})
	return out
}

// fitParam fits an argument of a call by name to its parameter's
// annotation (F16 3.4): through its form, and a fit that used any does not
// hold. With a default, null is accepted too. A plain argument at a
// parameter with an Fn is arg-mismatch by the static rules
// (f17-recursion.md 13.2 rule 3), so it is not fitted here.
func (c *checker) fitParam(p *syntax.Param, a syntax.Node, t *types.Type, name string) {
	switch {
	case p.Fn != nil:
	case p.Type != nil:
		want, ok := c.res.AnnTypes[p.Type]
		if !ok {
			return // the annotation has a static diagnostic
		}
		if p.Default != nil {
			want = types.WithNull(want)
		}
		c.fitForm(a, t, want, formAnnotArg)
	}
}

// callFnParam is a call of a parameter with a function type
// (f17-recursion.md 13.3): each plain argument must surely fit the type
// its function type gives that parameter, through its form (F16 3.3). The
// call's type is the function type's result, any for unknown. The count,
// and a lambda argument, are static rules.
func (c *checker) callFnParam(b *static.Binding, args []syntax.Node, fs facts) *types.Type {
	ps, r, unknown := c.fnOf(b.Param)
	if len(args) != len(ps) {
		c.synthPlain(args, fs)
		return types.Never
	}
	for i, a := range args {
		if l, _ := c.lambdaArg(a); l != nil || c.fnArg(a) != nil {
			continue // lambda-to-lambda, a static rule
		}
		t := c.synth(a, fs)
		if ps[i] != nil {
			c.fitForm(a, t, ps[i], formAnnotArg)
		}
	}
	if unknown {
		return types.Any
	}
	return orNever(r)
}

// userFnArg checks a lambda argument at a parameter p with a function type
// of a const-bound lambda (f17-recursion.md 13.3). A lambda written there,
// or a const-bound lambda named there that is not checked once, is checked
// against the function type as 13.5 step 3 checks a lambda against a
// library Fn, with no Var to solve. A lambda checked once, or a parameter
// with a function type passed on, is fitted. Every fit is F16 3.3's: a fit
// that uses any does not hold.
func (c *checker) userFnArg(p *syntax.Param, a syntax.Node, fs facts, name string) {
	qs, r, unknown := c.fnOf(p)
	if b := c.fnArg(a); b != nil {
		gp, _, _ := c.fnOf(b.Param)
		if len(gp) > len(qs) {
			return // arity, a static rule
		}
		c.fnParamPassed(b, qs)
		if !unknown && r != nil {
			if s := c.fnParamResult(b); !fitsK(s, r, formAnnot) {
				c.m("body", word("name", name), ty("found", s), ty("expected", r)).diag("arg-mismatch", a.Span())
			}
		}
		return
	}
	l, named := c.lambdaArg(a)
	if l == nil || len(l.Params) > len(qs) {
		return // arity, a static rule
	}
	if named && c.res.SelfRefs[a] && !l.Typed() {
		return // a reference to its own name is no use (13.9)
	}
	lf, base, depth := fs, c.base, c.lamDepth
	var once *lamInfo
	if named {
		if info := c.lambdas[l]; info != nil && info.once {
			once = info // checked once, where it was written (F16 3.4)
		} else {
			bb, ok := c.use(a, l)
			if !ok {
				return
			}
			lf, base, depth = info.facts, bb, l.Depth()
		}
	}
	for i, lp := range l.Params {
		if qs[i] != nil {
			c.passed(lp, qs[i], name)
		}
	}
	var b *types.Type
	if once != nil {
		b = once.result
	} else {
		bound := make([]*types.Type, len(qs))
		for i, q := range qs {
			bound[i] = orNever(q)
		}
		sb, sd := c.base, c.lamDepth
		c.base, c.lamDepth = base, depth
		c.bindArgs(l, bound, nil, lf)
		b = c.resultOf(l, c.synth(l.Body, lf), formAnnot)
		c.base, c.lamDepth = sb, sd
	}
	if unknown {
		c.tested(l.Body, b) // a predicate (13.7)
		return
	}
	if r != nil && !fitsK(b, r, formAnnot) {
		c.m("body", word("name", name), ty("found", b), ty("expected", r)).diag("arg-mismatch", l.Body.Span())
	}
}

// passed checks a value of type q that the library, or a method on an any
// receiver, passes to a parameter of a lambda argument (F16 3.4). It must
// fit the parameter's annotation without a fit that used any; a parameter
// with an Fn takes no value.
func (c *checker) passed(p *syntax.Param, q *types.Type, name string) {
	switch {
	case p.Fn != nil:
		c.m("fn-value", word("name", name), ty("found", q)).diag("arg-mismatch", p.Fn.Span)
	case p.Type != nil:
		want, ok := c.res.AnnTypes[p.Type]
		if !ok {
			return
		}
		if !fitsK(q, want, formAnnotArg) {
			c.m("argument", ty("expected", want), ty("found", q)).diag("arg-mismatch", p.Type.Span)
		}
	}
}

// bindArgs binds the parameters of a lambda whose body is about to be
// checked. An annotated parameter is bound to its annotation (F16 3.4).
// Any other is bound by 12.4 to ts[i]; past the end of ts, to fill when it
// is not nil, else as an omitted argument.
func (c *checker) bindArgs(l *syntax.Lambda, ts []*types.Type, fill *types.Type, fs facts) {
	for i, p := range l.Params {
		switch {
		case p.Annotated():
			c.bindAnnotated(p, fs, formAnnot)
		case i < len(ts):
			c.bindParam(p, ts[i], true, fs)
		case fill != nil:
			c.bindParam(p, fill, true, fs)
		default:
			c.bindParam(p, types.Never, false, fs)
		}
	}
}

// bindAnnotated binds an annotated parameter to its annotation: a program
// type, or never for an Fn and for a type with a static diagnostic. Its
// default must fit the annotation (F16 3.3); k is formAnnot, or formResult
// in the check of a lambda with no use.
func (c *checker) bindAnnotated(p *syntax.Param, fs facts, k formKind) {
	t, ok := types.Never, false
	if p.Type != nil {
		t, ok = c.res.AnnTypes[p.Type]
		if !ok {
			t = types.Never
		}
	}
	if p.Default != nil {
		d := c.synth(p.Default, fs)
		if ok {
			c.fitForm(p.Default, d, t, k)
		}
	}
	c.bindPattern(p.Pattern, t, p.Pattern.Span(), fs)
}

// resultOf is the result type of a lambda whose body has type b: its
// result annotation, which the body must fit (F16 3.3), or b.
func (c *checker) resultOf(l *syntax.Lambda, b *types.Type, k formKind) *types.Type {
	if l.Result == nil {
		return b
	}
	t, ok := c.res.AnnTypes[l.Result]
	if !ok {
		return types.Never
	}
	c.fitForm(l.Body, b, t, k)
	return t
}

// checkedOnce reports whether a const-bound lambda is checked once, where
// it is written (F16 3.4): it carries an annotation, and every parameter
// is annotated.
func checkedOnce(l *syntax.Lambda) bool {
	some := l.Result != nil
	for _, p := range l.Params {
		if !p.Annotated() {
			return false
		}
		some = true
	}
	return some
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
		fnp   *static.Binding // a parameter with a function type, passed on
	}
	var lams []lamArg
	for i, a := range args {
		p := sig.ParamAt(i)
		l, named := c.lambdaArg(a)
		fnp := c.fnArg(a)
		if l != nil || fnp != nil {
			if static.HigherOrder(l) {
				continue // lambda-to-lambda, a static rule (F17 13.2 rule 8)
			}
			if p.Fn == nil {
				if isMethod {
					c.m("argument", word("name", c.callName(n)), word("param", p.Name)).diag("not-a-value", a.Span())
				}
				continue
			}
			if named && c.res.SelfRefs[a] && !l.Typed() {
				continue // a reference to its own name is no use (13.9)
			}
			lams = append(lams, lamArg{a, l, named, p.Fn, fnp})
			continue
		}
		t := c.synth(a, fs)
		if p.Fn != nil {
			if id, ok := a.(*syntax.Ident); ok && c.res.Idents[id].HintParam() {
				// A parameter with no function type (f17-recursion.md 13.5).
				c.m("parameter-at-fn", word("name", id.Name)).diag("arg-mismatch", a.Span())
				continue
			}
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
		if la.fnp != nil {
			c.checkFnParam(la.node, la.fnp, la.fn, sol, isMethod)
			if solOver() {
				return types.Never
			}
			continue
		}
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
	var once *lamInfo
	if named {
		if info := c.lambdas[l]; info != nil && info.once {
			once = info // checked once, where it was written (F16 3.4)
		} else {
			b, ok := c.use(at, l)
			if !ok {
				return nil
			}
			lf, base, depth = info.facts, b, l.Depth()
		}
	}
	var narrowed *types.Type
	check := func() *types.Type {
		// Each value passed to an annotated parameter must fit it (F16
		// 3.4).
		for i, p := range l.Params {
			c.passed(p, qs[i], c.callName(name))
		}
		if once != nil {
			return once.result
		}
		sb, sd := c.base, c.lamDepth
		c.base, c.lamDepth = base, depth
		defer func() { c.base, c.lamDepth = sb, sd }()
		c.bindArgs(l, qs, nil, lf)
		b := c.synth(l.Body, lf)
		if narrows && len(l.Params) > 0 {
			narrowed = c.narrowed(l, qs[0])
		}
		return c.resultOf(l, b, formAnnot)
	}
	if once != nil && narrows && len(l.Params) > 0 {
		narrowed = c.narrowed(l, qs[0])
	}
	dl, sl := len(c.diags), len(c.sites)
	b := check()
	if fn.Result == nil {
		c.tested(l.Body, b)
		return narrowed
	}
	c.fnResult(fn, sol, qs, check, b, l.Body.Span(), name, dl, sl)
	return narrowed
}

// checkFnParam checks a parameter with a function type given as a lambda
// argument of a library entry (f17-recursion.md 13.3): 13.5 step 3, with
// its function type as the lambda's annotated type. Each value the entry
// passes must surely fit its parameter's type; its result stands for the
// body's type, and the fit of the body is at the argument.
func (c *checker) checkFnParam(at syntax.Node, b *static.Binding, fn *types.Fn, sol map[string]*types.Type, isMethod bool) {
	name := c.curCall
	gp, _, _ := c.fnOf(b.Param)
	if len(gp) > len(fn.Params) {
		if isMethod {
			c.m("parameters", word("name", c.callName(name)), num("most", len(fn.Params))).diag("arity", at.Span())
		}
		return
	}
	qs := make([]*types.Type, len(fn.Params))
	for i, q := range fn.Params {
		vs := map[string]bool{}
		types.VarsIn(q, vs)
		for v := range vs {
			if _, ok := sol[v]; !ok && i < len(gp) {
				c.m("", word("name", c.callName(name))).diag("cannot-infer", at.Span())
				return
			}
		}
		qs[i] = types.Subst(q, sol)
	}
	check := func() *types.Type {
		c.fnParamPassed(b, qs)
		return c.fnParamResult(b)
	}
	dl, sl := len(c.diags), len(c.sites)
	r := check()
	if fn.Result == nil {
		return // a predicate: its result is no tested position
	}
	c.fnResult(fn, sol, qs, check, r, at.Span(), name, dl, sl)
}

// fnResult is the end of 13.5 step 3 for a lambda argument whose body, or
// result, has type b: the seed rule of 3a, then bind(R, B) and the fit of
// B to R at body. check checks the lambda again under qs. dl and sl are the
// diagnostics and sites before the first check.
func (c *checker) fnResult(fn *types.Fn, sol map[string]*types.Type, qs []*types.Type, check func() *types.Type,
	b *types.Type, body syntax.Span, name syntax.Node, dl, sl int) {
	r := fn.Result
	rv := map[string]bool{}
	types.VarsIn(r, rv)
	// The seed rule (13.5 step 3a): it fires also when B fits
	// A0 only by a fit that used any. The second check binds nothing.
	// It reads with R in place of the Var, for walk, whose R is T[] | null
	// (F17 5.6): B is fitted to R under A0, and the new solution is the Join
	// of A0 and what bind(R, B) gathers for the Var. For reduce, whose R is
	// its Var, that is the rule as 13.5 states it.
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
			if ok, used := types.Fits(b, types.Subst(r, sol)); !ok || used {
				c.diags, c.sites = c.diags[:dl], c.sites[:sl]
				gathered := map[string]*types.Type{}
				bind(r, b, gathered)
				if g, ok := gathered[x]; ok {
					sol[x] = types.Join(a0, g)
				}
				for i, q := range fn.Params {
					qs[i] = types.Subst(q, sol)
				}
				b = check()
				c.mustFitM(body, b, types.Subst(r, sol), "arg-mismatch", "body", c.callName(name))
				return
			}
		}
	}
	bind(r, b, sol)
	// The body is always fitted (13.5 step 3).
	c.mustFitM(body, b, types.Subst(r, sol), "arg-mismatch", "body", c.callName(name))
}

// narrowed is the narrowed type N of 13.8, or nil when the first parameter
// is not a name with no default. q is the element type. The true facts are
// those of the body's last check. An annotated parameter narrows from q,
// the type the values have, and not from its annotation (F16 3.4).
func (c *checker) narrowed(l *syntax.Lambda, q *types.Type) *types.Type {
	p := l.Params[0]
	np, ok := p.Pattern.(*syntax.NamePat)
	if !ok || np.Discard || p.Default != nil {
		return nil
	}
	slot, ok := c.res.Slots[np]
	if !ok || slot < 0 {
		return nil
	}
	t := c.slots[slot]
	if p.Annotated() {
		t = q
	}
	tf, _ := c.factsOf(l.Body)
	if tf.covers(path{root: slot}) {
		t = types.Nonnull(t)
	}
	return tf.apply(t, path{root: slot}) // F4 6.5
}

type formKind uint8

const (
	formResult formKind = iota
	formArg
	// formAnnot and formAnnotArg are the fits of package F16 3.3: a value
	// against an annotation, and an argument against an annotated
	// parameter. A fit that used any does not hold there.
	formAnnot
	formAnnotArg
)

// strict reports whether a fit that used any fails (F16 3.3).
func (k formKind) strict() bool { return k == formAnnot || k == formAnnotArg }

// fitsK is Fits, or for a strict kind surely fits (F4 6.2).
func fitsK(t, want *types.Type, k formKind) bool {
	ok, used := types.Fits(t, want)
	return ok && !(used && k.strict())
}

// fitForm is "e fits T through its form" (15.1), with the code of 15.2 at
// each place of failure. For the kinds of F16 3.3, a fit that used any is a
// place of failure, with the template any.
func (c *checker) fitForm(e syntax.Node, t, want *types.Type, k formKind) {
	ok, used := types.Fits(t, want)
	if ok && !(used && k.strict()) {
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
		if k == formResult || k == formAnnot {
			code, tmpl = "result-mismatch", "result"
		} else if at != nil && at.HasNull() && fitsK(types.Nonnull(at), w, k) {
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
					if ok, used := types.Fits(st, types.List(el)); !ok || (used && k.strict()) {
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
