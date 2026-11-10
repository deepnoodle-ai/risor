package eval

import (
	"fmt"
	"strconv"

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
		a := &as[i]
		if a.lam != nil {
			if a.inPlace {
				r.step(1)
			}
			out[i] = a.lam
			continue
		}
		out[i] = a.x(r)
	}
	return out
}

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
			steps = append(steps, &stepCode{kind: stepMethod, optional: t.Optional, name: t.Name,
				args: c.args(t.Args), span: t.Span(), measure: c.measure(t)})
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

func (r *run) member(s *stepCode, v any) any {
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
	e, ok := library.Method(k, s.name)
	if !ok {
		r.shapeFail(s, "method", messages.Val("kind", v), name)
	}
	n := len(s.args)
	if n < e.MinArgs() || (e.MaxArgs() >= 0 && n > e.MaxArgs()) {
		r.shapeFail(s, "count", messages.Word("count", numText(n)), name)
	}
	for i := range s.args {
		p := e.ParamAt(i)
		if p.Callback >= 0 && !s.args[i].isLambda() {
			r.shapeFail(s, "callback", messages.Word("count", numText(i+1)), name)
		}
		if p.Callback < 0 && s.args[i].isLambda() {
			r.shapeFail(s, "not-callback", messages.Word("count", numText(i+1)), name)
		}
	}
	for i := range s.args {
		p := e.ParamAt(i)
		if s.args[i].isLambda() && s.args[i].nparams > p.Callback {
			r.shapeFail(s, "parameters", name, messages.Word("count", numText(p.Callback)))
		}
	}
	args := r.evalArgs(s.args)
	lc := &libCall{e: e, recv: v, args: args, span: s.span, measure: s.measure}
	r.testArgs(lc)
	r.step(1)
	return r.runEntry(lc)
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
	return r.hostCall(m.Impl, append([]any{v}, args...), s.span, s.name)
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
			vals := r.evalArgs(args)
			r.step(1)
			r.enterCall(m)
			v := lam.invoke(r, vals)
			r.depth -= m
			return v
		}
	case static.Host:
		f := callee.Host
		return func(r *run) any {
			vals := r.evalArgs(args)
			r.step(1)
			return r.hostCall(r.prog.cfg.Hosts[f.Index], vals, sp, f.FullName())
		}
	case static.Lib:
		e := callee.Lib
		m := c.measure(n)
		return func(r *run) any {
			vals := r.evalArgs(args)
			lc := &libCall{e: e, args: vals, span: sp, measure: m}
			r.testArgs(lc)
			r.step(1)
			return r.runEntry(lc)
		}
	}
	panic("call")
}

// hostCall calls a host function (03-host.md 3.5).
func (r *run) hostCall(f HostFunc, args []any, sp syntax.Span, name string) any {
	res, err, crashed := r.safeCall(f, args)
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

func (r *run) safeCall(f HostFunc, args []any) (res any, err error, crashed any) {
	defer func() {
		if x := recover(); x != nil {
			crashed = x
		}
	}()
	if f == nil {
		return nil, nil, "no implementation"
	}
	res, err = f(&Call{r: r}, args)
	return res, err, nil
}
