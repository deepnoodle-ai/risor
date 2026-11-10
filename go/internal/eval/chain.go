package eval

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/static"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// argCode is one argument of a call. A lambda argument is passed as its
// *lambdaCode.
type argCode struct {
	x       code
	lam     *lambdaCode
	inPlace bool
	nparams int
}

func (a *argCode) isLambda() bool { return a.lam != nil }

func (c *compiler) arg(n syntax.Node) argCode {
	switch n := n.(type) {
	case *syntax.Lambda:
		return argCode{lam: c.lambda(n), inPlace: true, nparams: len(n.Params)}
	case *syntax.Ident:
		if b := c.res.Idents[n]; b != nil && b.Kind == static.ConstLambda {
			return argCode{lam: c.lambdas[b.Lambda], nparams: len(b.Lambda.Params)}
		}
	}
	return argCode{x: c.expr(n)}
}

func (c *compiler) args(ns []syntax.Node) []argCode {
	out := make([]argCode, len(ns))
	for i, n := range ns {
		out[i] = c.arg(n)
	}
	return out
}

// evalArgs evaluates the arguments left to right. A lambda written in place
// charges 1 step where it stands (02-evaluation.md 5.2).
func (r *run) evalArgs(as []argCode) []any {
	out := make([]any, len(as))
	for i := range as {
		out[i] = r.argValue(&as[i])
	}
	return out
}

func (r *run) argValue(a *argCode) any {
	if a.lam != nil {
		if a.inPlace {
			r.step(1)
		}
		return a.lam
	}
	return a.x(r)
}

// pushArgs evaluates the arguments onto the run's argument stack, for a
// callee that does not keep the slice. popArgs(base) removes them. A try
// that catches a failure restores the stack's length.
func (r *run) pushArgs(as []argCode) (args []any, base int) {
	base = len(r.args)
	n := len(as)
	for range as {
		r.args = append(r.args, nil)
	}
	for i := range as {
		// An argument's evaluation may grow the stack, so the store reads
		// r.args after it.
		v := r.argValue(&as[i])
		r.args[base+i] = v
	}
	return r.args[base : base+n : base+n], base
}

// popArgs leaves the values in place; the run's end clears them.
func (r *run) popArgs(base int) { r.args = r.args[:base] }

// pushCall takes a library call from the run's stack of them. popCall gives
// it back. A try that catches a failure restores the stack's top.
func (r *run) pushCall() *libCall {
	if r.lcTop == len(r.lcs) {
		r.lcs = append(r.lcs, libCall{})
		// The append may have moved the stack; the calls in progress keep
		// pointers into the old array, which stays valid.
	}
	c := &r.lcs[r.lcTop]
	r.lcTop++
	return c
}

// popCall leaves the call's fields in place; the next push or the run's
// end clears them.
func (r *run) popCall(*libCall) { r.lcTop-- }

type stepKind uint8

const (
	stepMember stepKind = iota
	stepIndex
	stepMethod
)

type stepCode struct {
	kind     stepKind
	optional bool
	name     string
	size     int64
	idx      code
	args     []argCode
	span     syntax.Span
	measure  int
	// plans holds the library method for a string, a list and a map
	// receiver, found when the chain is compiled.
	plans [3]methodPlan
}

// methodPlan is a library method selected for one receiver kind, with the
// result of its shape test, which depends only on the call's text.
type methodPlan struct {
	e      *library.Entry
	info   *entryInfo
	shape  string // "" when the shape test passes
	params []messages.Param
}

func planIndex(k value.Kind) int { return int(k - value.String) }

func (c *compiler) plan(s *stepCode) {
	for _, k := range []value.Kind{value.String, value.List, value.Map} {
		p := &s.plans[planIndex(k)]
		e, ok := library.Method(k, s.name)
		if !ok {
			continue
		}
		p.e, p.info = e, infoOf(e)
		name := messages.Word("name", s.name)
		n := len(s.args)
		if n < e.MinArgs() || (e.MaxArgs() >= 0 && n > e.MaxArgs()) {
			p.shape, p.params = "count", []messages.Param{messages.Word("count", numText(n)), name}
			continue
		}
		for i := range s.args {
			pa := e.ParamAt(i)
			if pa.Callback >= 0 && !s.args[i].isLambda() {
				p.shape, p.params = "callback", []messages.Param{messages.Word("count", numText(i+1)), name}
				break
			}
			if pa.Callback < 0 && s.args[i].isLambda() {
				p.shape, p.params = "not-callback", []messages.Param{messages.Word("count", numText(i+1)), name}
				break
			}
		}
		if p.shape != "" {
			continue
		}
		for i := range s.args {
			pa := e.ParamAt(i)
			if s.args[i].isLambda() && s.args[i].nparams > pa.Callback {
				p.shape, p.params = "parameters", []messages.Param{name, messages.Word("count", numText(pa.Callback))}
				break
			}
		}
	}
}

// valueSlot gives the slot of a name bound by const, a parameter or a
// pattern.
func (c *compiler) valueSlot(n syntax.Node) (int, bool) {
	id, ok := n.(*syntax.Ident)
	if !ok {
		return 0, false
	}
	b := c.res.Idents[id]
	if b == nil || b.Kind != static.Value {
		return 0, false
	}
	return b.Slot, true
}

// chain compiles a chain as a base and a list of steps run by a loop
// (01-language.md 7.4). An optional step that finds null makes the whole
// chain null.
func (c *compiler) chain(n syntax.Node) code {
	if oc, ok := n.(*syntax.OptChain); ok {
		n = oc.X
	}
	var nodes []syntax.Node
	x := n
loop:
	for {
		switch t := x.(type) {
		case *syntax.Member:
			nodes = append(nodes, t)
			x = t.X
		case *syntax.Index:
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
	base := c.expr(x)
	steps := make([]*stepCode, 0, len(nodes))
	for i := len(nodes) - 1; i >= 0; i-- {
		switch t := nodes[i].(type) {
		case *syntax.Member:
			steps = append(steps, &stepCode{kind: stepMember, optional: t.Optional, name: t.Name,
				size: value.Size(t.Name), span: t.Span()})
		case *syntax.Index:
			steps = append(steps, &stepCode{kind: stepIndex, optional: t.Optional, idx: c.expr(t.Index), span: t.Span()})
		case *syntax.MethodCall:
			sc := &stepCode{kind: stepMethod, optional: t.Optional, name: t.Name,
				args: c.args(t.Args), span: t.Span(), measure: c.measure(t)}
			c.plan(sc)
			steps = append(steps, sc)
		}
	}
	allMember := true
	for _, st := range steps {
		if st.kind != stepMember {
			allMember = false
		}
	}
	if allMember && len(steps) == 1 {
		st := steps[0]
		if slot, ok := c.valueSlot(x); ok {
			return func(r *run) any {
				v := r.slots[slot]
				if v == nil && st.optional {
					return nil
				}
				return r.member(st, v)
			}
		}
		return func(r *run) any {
			v := base(r)
			if v == nil && st.optional {
				return nil
			}
			return r.member(st, v)
		}
	}
	if allMember {
		return func(r *run) any {
			v := base(r)
			for _, s := range steps {
				if v == nil && s.optional {
					return nil
				}
				v = r.member(s, v)
			}
			return v
		}
	}
	return func(r *run) any {
		v := base(r)
		for _, s := range steps {
			if v == nil && s.optional {
				return nil
			}
			switch s.kind {
			case stepMember:
				v = r.member(s, v)
			case stepIndex:
				v = r.index(s, v)
			default:
				v = r.method(s, v)
			}
		}
		return v
	}
}

// member reads a field. The common case, a present field of a host map
// whose value needs no reading, is small enough to inline.
func (r *run) member(s *stepCode, v any) any {
	if m, ok := v.(map[string]any); ok {
		r.step(1 + s.size)
		if x, ok := m[s.name]; ok {
			return r.read(x, s.span)
		}
	}
	return r.memberMore(s, v)
}

// memberMore is member for a field that is missing or not of a host map.
// The map case has charged its step already.
func (r *run) memberMore(s *stepCode, v any) any {
	if _, ok := v.(map[string]any); ok {
		if s.optional {
			return nil
		}
		r.raise(fail("missing-field", "field", s.span, messages.Word("name", s.name)))
	}
	switch value.KindOf(v) {
	case value.Map:
		r.step(1 + s.size)
		x, ok := value.Get(v, s.name)
		if !ok {
			if s.optional {
				return nil
			}
			r.raise(fail("missing-field", "field", s.span, messages.Word("name", s.name)))
		}
		return r.read(x, s.span)
	case value.Null:
		r.raise(fail("null-operand", "field", s.span, messages.Word("name", s.name)))
	}
	r.raise(fail("wrong-kind", "field", s.span, messages.Word("name", s.name), messages.Val("kind", v)))
	return nil
}

func numText(n int) string { return strconv.Itoa(n) }

func (r *run) index(s *stepCode, v any) any {
	k := value.KindOf(v)
	switch k {
	case value.List, value.String, value.Map:
	case value.Null:
		r.raise(fail("null-operand", "index", s.span))
	default:
		r.raise(fail("wrong-kind", "index", s.span, messages.Val("kind", v)))
	}
	i := s.idx(r)
	if i == nil {
		r.raise(fail("null-operand", "key", s.span))
	}
	if k == value.Map {
		key, ok := i.(string)
		if !ok {
			r.raise(fail("wrong-kind", "key", s.span, messages.Val("found", i)))
		}
		r.step(1 + value.Size(key))
		x, ok := value.Get(v, key)
		if !ok {
			if s.optional {
				return nil
			}
			r.raise(fail("missing-field", "key", s.span, messages.Val("key", key)))
		}
		return r.read(x, s.span)
	}
	f, ok := i.(float64)
	if !ok || !whole(f) {
		r.raise(fail("wrong-kind", "whole", s.span, messages.Val("found", i)))
	}
	if k == value.List {
		r.step(1)
		n := value.Len(v)
		if f < 0 || f >= float64(n) {
			if s.optional {
				return nil
			}
			r.raise(fail("index-range", "index", s.span, messages.Val("index", f),
				messages.Word("length", numText(n)), messages.Word("kind", "list")))
		}
		return r.read(value.At(v, int(f)), s.span)
	}
	str := v.(string)
	r.step(1 + value.Size(str))
	if f >= 0 {
		n := 0
		for _, cp := range str {
			if float64(n) == f {
				r.unit(1)
				return string(cp)
			}
			n++
		}
	}
	if s.optional {
		return nil
	}
	r.raise(fail("index-range", "index", s.span, messages.Val("index", f),
		messages.Word("length", numText(value.RuneCount(str))), messages.Word("kind", "string")))
	return nil
}

func (r *run) shapeFail(s *stepCode, template string, params ...messages.Param) {
	r.raise(fail("wrong-kind", template, s.span, params...))
}

// method runs a method call step: the method is selected by the receiver's
// kind, then the shape test (02-evaluation.md 2.3), the arguments, the tests
// of 04-library.md 2.6, the call's step and the entry.
func (r *run) method(s *stepCode, v any) any {
	k := value.KindOf(v)
	name := messages.Word("name", s.name)
	switch k {
	case value.Null:
		r.raise(fail("null-operand", "method", s.span, name))
	case value.OpaqueKind:
		return r.opaqueMethod(s, v)
	case value.String, value.List, value.Map:
	default:
		r.shapeFail(s, "method", messages.Val("kind", v), name)
	}
	p := &s.plans[planIndex(k)]
	if p.e == nil {
		r.shapeFail(s, "method", messages.Val("kind", v), name)
	}
	if p.shape != "" {
		r.shapeFail(s, p.shape, p.params...)
	}
	args, base := r.pushArgs(s.args)
	lc := r.pushCall()
	*lc = libCall{e: p.e, info: p.info, recv: v, args: args, span: s.span, measure: s.measure}
	r.testArgs(lc)
	r.step(1)
	res := r.runEntry(lc)
	r.popCall(lc)
	r.popArgs(base)
	return res
}

func (r *run) opaqueMethod(s *stepCode, v any) any {
	name := messages.Word("name", s.name)
	m, ok := r.prog.cfg.Methods[value.OpaqueTypeName(v)][s.name]
	if !ok {
		r.shapeFail(s, "method", messages.Val("kind", v), name)
	}
	n := len(s.args)
	if n < m.Min || (m.Max >= 0 && n > m.Max) {
		r.shapeFail(s, "count", messages.Word("count", numText(n)), name)
	}
	for i := range s.args {
		if s.args[i].isLambda() {
			r.shapeFail(s, "not-callback", messages.Word("count", numText(i+1)), name)
		}
	}
	args := r.evalArgs(s.args)
	r.step(1)
	return r.hostCall(m.Impl, new(Call), append([]any{v}, args...), s.span, s.name)
}

// call compiles a call by name, or a call of a namespace.
func (c *compiler) call(n syntax.Node, argNodes []syntax.Node) code {
	callee := c.res.Calls[n]
	if callee == nil || callee.Kind == static.Unknown {
		panic("unresolved call")
	}
	args := c.args(argNodes)
	sp := n.Span()
	switch callee.Kind {
	case static.Lambda:
		lam := c.lambdas[callee.Lambda]
		m := c.measure(n)
		return func(r *run) any {
			vals, base := r.pushArgs(args)
			r.step(1)
			r.enterCall(m)
			v := lam.invoke(r, vals)
			r.depth -= m
			r.popArgs(base)
			return v
		}
	case static.Host:
		f := callee.Host
		name := f.FullName()
		return func(r *run) any {
			c, vals := r.hostFrame(args)
			r.step(1)
			return r.hostCall(r.prog.cfg.Hosts[f.Index], c, vals, sp, name)
		}
	case static.Lib:
		e := callee.Lib
		info := infoOf(e)
		m := c.measure(n)
		if e.Name == "len" && !e.IsMethod() && len(args) == 1 && !args[0].isLambda() &&
			e.Steps == "sz(x)" && e.Units == "0" && len(info.params) == 1 && info.params[0].test {
			return lenCall(e, info, args[0].x, sp)
		}
		return func(r *run) any {
			vals, base := r.pushArgs(args)
			lc := r.pushCall()
			*lc = libCall{e: e, info: info, args: vals, span: sp, measure: m}
			r.testArgs(lc)
			r.step(1)
			v := r.runEntry(lc)
			r.popCall(lc)
			r.popArgs(base)
			return v
		}
	}
	panic("call")
}

// lenCall is a call of len, made without a library call record. It tests,
// charges and fails as the general path does: the argument test, the call's
// step, then the entry's sz(x) steps and no units.
func lenCall(e *library.Entry, info *entryInfo, x code, sp syntax.Span) code {
	p := &info.params[0]
	return func(r *run) any {
		v := x(r)
		var k value.Kind
		switch v.(type) {
		case string:
			k = value.String
		case []any:
			k = value.List
		case map[string]any:
			k = value.Map
		default:
			k = value.KindOf(v)
		}
		if !p.kinds[k] {
			r.raise(fail("wrong-kind", "argument", sp, messages.Word("name", e.Name), messages.Val("kind", v)))
		}
		r.step(1)
		if s, ok := v.(string); ok {
			r.step(value.Size(s))
			return numBox(float64(utf8.RuneCountInString(s)))
		}
		return numBox(float64(value.Len(v)))
	}
}

// hostFrame evaluates a host call's arguments and makes its call handle.
// With two arguments or fewer the handle and the arguments are one
// allocation. The host may keep the slice; nothing reuses it.
func (r *run) hostFrame(as []argCode) (*Call, []any) {
	switch len(as) {
	case 0:
		return new(Call), []any{}
	case 1:
		fr := new(struct {
			c Call
			a [1]any
		})
		fr.a[0] = r.argValue(&as[0])
		return &fr.c, fr.a[:]
	case 2:
		fr := new(struct {
			c Call
			a [2]any
		})
		fr.a[0] = r.argValue(&as[0])
		fr.a[1] = r.argValue(&as[1])
		return &fr.c, fr.a[:]
	}
	return new(Call), r.evalArgs(as)
}

// hostCall calls a host function (03-host.md 3.5).
func (r *run) hostCall(f HostFunc, c *Call, args []any, sp syntax.Span, name string) any {
	c.r = r
	res, err, crashed := r.safeCall(f, c, args)
	c.end()
	if p := r.pending; p != nil {
		r.pending = nil
		r.raise(p)
	}
	if crashed != nil {
		ff := fail("function-panic", "", sp, messages.Word("name", name))
		ff.cause = fmt.Errorf("%v", crashed)
		r.raise(ff)
	}
	if err != nil {
		ff := &failure{code: "function-error", span: sp, hasSpan: true, msg: err.Error(), ownMsg: true, cause: err}
		r.raise(ff)
	}
	return r.read(res, sp)
}

func (r *run) safeCall(f HostFunc, c *Call, args []any) (res any, err error, crashed any) {
	defer func() {
		if x := recover(); x != nil {
			crashed = x
		}
	}()
	if f == nil {
		return nil, nil, "no implementation"
	}
	res, err = f(c, args)
	return res, err, nil
}
