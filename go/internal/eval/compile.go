package eval

import (
	"context"
	"fmt"
	"time"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/static"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

type code func(r *run) any

// Method is a method an opaque type declares.
type Method struct {
	Name     string
	Min, Max int // Max is -1 with a rest parameter
	Impl     HostFunc
}

// Config is what a compile needs from the environment.
type Config struct {
	Names    *static.Names
	Hosts    []HostFunc // by static.Func.Index
	VarNames []string   // by variable index
	// Methods maps an opaque type name to its methods by name.
	Methods map[string]map[string]*Method
	Limits  Limits
}

// Program is a compiled program. It does not change, and several runs may
// use it at once.
type Program struct {
	cfg    *Config
	file   *syntax.File
	root   code
	nslots int
}

type lambdaCode struct {
	params  []paramCode
	body    code
	nparams int
}

type paramCode func(r *run, v any, present bool)

type binder func(r *run, v any)

func (l *lambdaCode) invoke(r *run, args []any) any {
	for i, p := range l.params {
		if i < len(args) {
			p(r, args[i], true)
		} else {
			p(r, nil, false)
		}
	}
	return l.body(r)
}

type compiler struct {
	cfg      *Config
	res      *static.Result
	lambdas  map[*syntax.Lambda]*lambdaCode
	lamDepth int
}

// Compile compiles a program whose static rules found nothing.
func Compile(prog *syntax.Program, file *syntax.File, res *static.Result, cfg *Config) (p *Program, err error) {
	c := &compiler{cfg: cfg, res: res, lambdas: map[*syntax.Lambda]*lambdaCode{}}
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("compile: %v", x)
		}
	}()
	root := c.block(prog.Consts, prog.Result)
	return &Program{cfg: cfg, file: file, root: root, nslots: res.NSlots}, nil
}

// Run runs the program. It returns the value or one run diagnostic, and the
// usage in both cases.
func (p *Program) Run(ctx context.Context, vars Vars) (result any, usage Usage, d *diag.Diagnostic) {
	if ctx == nil {
		ctx = context.Background()
	}
	lim := p.cfg.Limits
	r := &run{
		prog: p, ctx: ctx, vars: vars,
		slots:    make([]any, p.nslots),
		varVals:  make([]any, len(p.cfg.VarNames)),
		varSeen:  make([]bool, len(p.cfg.VarNames)),
		maxSteps: lim.Steps, maxUnits: lim.Alloc, maxDepth: lim.Depth,
	}
	if lim.Timeout > 0 {
		r.deadline = time.Now().Add(lim.Timeout)
	}
	v, f := r.protectAll(p.root)
	usage = Usage{Steps: r.steps, Alloc: r.units}
	if f != nil {
		dd := f.diagnostic(p.file)
		return nil, usage, &dd
	}
	return v, usage, nil
}

// protectAll is protect that also turns a Go panic into internal-error.
func (r *run) protectAll(c code) (v any, f *failure) {
	defer func() {
		if x := recover(); x != nil {
			if ff, ok := x.(*failure); ok {
				f = ff
				return
			}
			f = internalError(x)
		}
	}()
	if f := r.checkDeadline(); f != nil {
		return nil, f
	}
	return c(r), nil
}

func (c *compiler) measure(n syntax.Node) int { return 1 + n.Depth() - c.lamDepth }

func (c *compiler) block(consts []*syntax.Const, result syntax.Node) code {
	cs := make([]code, len(consts))
	for i, k := range consts {
		cs[i] = c.constLine(k)
	}
	res := c.expr(result)
	if len(cs) == 0 {
		return res
	}
	return func(r *run) any {
		for _, k := range cs {
			k(r)
		}
		return res(r)
	}
}

func (c *compiler) constLine(k *syntax.Const) code {
	if lam, ok := k.Value.(*syntax.Lambda); ok {
		c.lambdas[lam] = c.lambda(lam)
		return func(r *run) any { r.step(1); return nil } // a lambda evaluated
	}
	val := c.expr(k.Value)
	bind := c.pattern(k.Pattern)
	return func(r *run) any { bind(r, val(r)); return nil }
}

func (c *compiler) lambda(l *syntax.Lambda) *lambdaCode {
	saved := c.lamDepth
	c.lamDepth = l.Depth()
	defer func() { c.lamDepth = saved }()
	lc := &lambdaCode{nparams: len(l.Params)}
	for _, p := range l.Params {
		var def code
		if p.Default != nil {
			def = c.expr(p.Default)
		}
		bind := c.pattern(p.Pattern)
		lc.params = append(lc.params, func(r *run, v any, present bool) {
			if (!present || v == nil) && def != nil {
				v = def(r)
			}
			bind(r, v)
		})
	}
	lc.body = c.expr(l.Body)
	return lc
}

func (c *compiler) slot(key any) int {
	if s, ok := c.res.Slots[key]; ok {
		return s
	}
	return -1
}

func (c *compiler) pattern(p syntax.Pattern) binder {
	switch p := p.(type) {
	case *syntax.NamePat:
		s := c.slot(p)
		if s < 0 {
			return func(*run, any) {}
		}
		return func(r *run, v any) { r.slots[s] = v }
	case *syntax.MapPat:
		return c.mapPat(p)
	case *syntax.ListPat:
		return c.listPat(p)
	}
	panic("pattern")
}

type fieldCode struct {
	key  string
	size int64
	slot int
	def  code
	span syntax.Span
}

func (c *compiler) mapPat(p *syntax.MapPat) binder {
	var fields []fieldCode
	named := map[string]bool{}
	for _, f := range p.Fields {
		fc := fieldCode{key: f.Key, size: value.Size(f.Key), slot: c.slot(f), span: f.Span}
		if f.Default != nil {
			fc.def = c.expr(f.Default)
		}
		fields = append(fields, fc)
		named[f.Key] = true
	}
	rest := -1
	if p.Rest != nil {
		rest = c.slot(p.Rest)
	}
	hasRest := p.Rest != nil
	sp := p.Span()
	return func(r *run, v any) {
		switch value.KindOf(v) {
		case value.Map:
		case value.Null:
			r.raise(fail("null-operand", "pattern", sp))
		default:
			r.raise(fail("wrong-kind", "pattern-map", sp, messages.Val("kind", v)))
		}
		for _, f := range fields {
			r.step(1 + f.size)
			x, ok := value.Get(v, f.key)
			if ok {
				x = r.read(x, f.span)
			}
			if (!ok || x == nil) && f.def != nil {
				x = f.def(r)
			} else if !ok {
				r.raise(fail("missing-field", "pattern", f.span, messages.Val("key", f.key)))
			}
			if f.slot >= 0 {
				r.slots[f.slot] = x
			}
		}
		if hasRest {
			var keys []string
			var w int64
			value.ForEachKey(v, func(k string) {
				if !named[k] {
					keys = append(keys, k)
					w += value.Weight(k)
				}
			})
			r.step(int64(len(keys)) + w/64)
			r.unit(1 + 2*int64(len(keys)))
			m := make(map[string]any, len(keys))
			for _, k := range keys {
				m[k], _ = value.Get(v, k)
			}
			if rest >= 0 {
				r.slots[rest] = m
			}
		}
	}
}

type elemCode struct {
	slot int
	def  code
	span syntax.Span
}

func (c *compiler) listPat(p *syntax.ListPat) binder {
	var elems []elemCode
	for _, e := range p.Elems {
		ec := elemCode{slot: c.slot(e.Name), span: e.Span}
		if e.Default != nil {
			ec.def = c.expr(e.Default)
		}
		elems = append(elems, ec)
	}
	rest := -1
	if p.Rest != nil {
		rest = c.slot(p.Rest)
	}
	hasRest := p.Rest != nil
	sp := p.Span()
	return func(r *run, v any) {
		switch value.KindOf(v) {
		case value.List:
		case value.Null:
			r.raise(fail("null-operand", "pattern", sp))
		default:
			r.raise(fail("wrong-kind", "pattern-list", sp, messages.Val("kind", v)))
		}
		n := value.Len(v)
		for i, e := range elems {
			r.step(1)
			var x any
			present := i < n
			if present {
				x = r.read(value.At(v, i), e.span)
			}
			if (!present || x == nil) && e.def != nil {
				x = e.def(r)
			} else if !present {
				r.raise(fail("index-range", "pattern", e.span,
					messages.Word("length", fmt.Sprint(n)), messages.Word("index", fmt.Sprint(i))))
			}
			if e.slot >= 0 {
				r.slots[e.slot] = x
			}
		}
		if hasRest {
			k := 0
			if n > len(elems) {
				k = n - len(elems)
			}
			r.step(int64(k))
			r.unit(1 + int64(k))
			out := make([]any, k)
			for i := range out {
				out[i] = value.At(v, len(elems)+i)
			}
			if rest >= 0 {
				r.slots[rest] = out
			}
		}
	}
}

func constant(v any) code { return func(*run) any { return v } }

func (c *compiler) expr(n syntax.Node) code {
	switch n := n.(type) {
	case *syntax.Number:
		return constant(n.Value)
	case *syntax.String:
		return constant(n.Value)
	case *syntax.Bool:
		return constant(n.Value)
	case *syntax.Null:
		return constant(nil)
	case *syntax.Ident:
		return c.ident(n)
	case *syntax.Block:
		return c.block(n.Consts, n.Result)
	case *syntax.Template:
		return c.template(n)
	case *syntax.List:
		return c.list(n)
	case *syntax.Map:
		return c.mapLit(n)
	case *syntax.Unary:
		return c.unary(n)
	case *syntax.Binary:
		if n.Op == "**" {
			return c.power(n)
		}
		return c.binary(n)
	case *syntax.Cond:
		return c.cond(n)
	case *syntax.Member, *syntax.Index, *syntax.MethodCall, *syntax.OptChain:
		if mc, ok := n.(*syntax.MethodCall); ok && c.res.NSCalls[mc] {
			return c.call(mc, mc.Args)
		}
		return c.chain(n)
	case *syntax.Call:
		return c.call(n, n.Args)
	case *syntax.Match:
		return c.match(n)
	case *syntax.Try:
		return c.try(n)
	}
	panic(fmt.Sprintf("cannot compile %T", n))
}

func (c *compiler) ident(n *syntax.Ident) code {
	b := c.res.Idents[n]
	if b == nil {
		panic("unresolved name " + n.Name)
	}
	switch b.Kind {
	case static.Value:
		s := b.Slot
		return func(r *run) any { return r.slots[s] }
	case static.EnvVar:
		i, name, sp := b.Var, b.Name, n.Span()
		return func(r *run) any {
			if !r.varSeen[i] {
				v, ok := r.vars.Var(name)
				if !ok {
					r.raise(fail("missing-variable", "", sp, messages.Word("name", name)))
				}
				r.varVals[i], r.varSeen[i] = v, true
			}
			return r.read(r.varVals[i], sp)
		}
	}
	panic("a const-bound lambda used as a value")
}

func (c *compiler) cond(n *syntax.Cond) code {
	var tests, thens []code
	var els code
	for {
		tests = append(tests, c.expr(n.Test))
		thens = append(thens, c.expr(n.Then))
		next, ok := n.Else.(*syntax.Cond)
		if !ok {
			els = c.expr(n.Else)
			break
		}
		n = next
	}
	return func(r *run) any {
		for i, t := range tests {
			if value.Truthy(t(r)) {
				return thens[i](r)
			}
		}
		return els(r)
	}
}

func (c *compiler) match(n *syntax.Match) code {
	subj := c.expr(n.Subject)
	type arm struct {
		lits   []any
		def    bool
		result code
	}
	var arms []arm
	for _, a := range n.Arms {
		am := arm{def: a.Default, result: c.expr(a.Result)}
		for _, l := range a.Literals {
			switch l := l.(type) {
			case *syntax.Number:
				am.lits = append(am.lits, l.Value)
			case *syntax.String:
				am.lits = append(am.lits, l.Value)
			case *syntax.Bool:
				am.lits = append(am.lits, l.Value)
			default:
				am.lits = append(am.lits, nil)
			}
		}
		arms = append(arms, am)
	}
	sp := n.Span()
	return func(r *run) any {
		v := subj(r)
		for _, a := range arms {
			r.step(1)
			if a.def {
				return a.result(r)
			}
			for _, l := range a.lits {
				if ls, ok := l.(string); ok {
					if vs, ok := v.(string); ok {
						r.step(value.Size(value.CommonPrefix(ls, vs)))
						if ls == vs {
							return a.result(r)
						}
					}
					continue
				}
				if scalarEqual(l, v) {
					return a.result(r)
				}
			}
		}
		r.raise(fail("no-match", "", sp, messages.Val("found", v)))
		return nil
	}
}

func scalarEqual(a, b any) bool {
	switch a := a.(type) {
	case nil:
		return b == nil
	case bool:
		x, ok := b.(bool)
		return ok && x == a
	case float64:
		x, ok := b.(float64)
		return ok && x == a
	case string:
		x, ok := b.(string)
		return ok && x == a
	}
	return false
}

func (c *compiler) try(n *syntax.Try) code {
	body := c.expr(n.Body)
	handler := c.expr(n.Handler)
	slot := -1
	named := n.Name != ""
	if named {
		slot = c.slot(n)
	}
	return func(r *run) any {
		depth, walk := r.depth, r.walk
		v, f := r.protect(body)
		if f == nil {
			return v
		}
		if !messages.Caught(f.code) {
			r.raise(f)
		}
		r.depth, r.walk = depth, walk
		if named {
			var msg string
			var steps, units int64
			msg = f.message()
			if !f.ownMsg {
				sz := value.Size(msg)
				steps, units = 1+sz, 1+sz
			}
			r.step(steps)
			r.unit(9 + units)
			ev := map[string]any{"code": f.code, "message": msg, "path": nil, "data": nil}
			if f.code == "failed" {
				ev["data"] = f.data
			}
			if slot >= 0 {
				r.slots[slot] = ev
			}
		}
		return handler(r)
	}
}

func (c *compiler) template(n *syntax.Template) code {
	type part struct {
		text string
		x    code
	}
	var parts []part
	for _, p := range n.Parts {
		if p.Expr != nil {
			parts = append(parts, part{x: c.expr(p.Expr)})
		} else if p.Text != "" {
			parts = append(parts, part{text: p.Text})
		}
	}
	sp := n.Span()
	return func(r *run) any {
		var b renderer
		b.r, b.span = r, sp
		for _, p := range parts {
			if p.x == nil {
				b.WriteString(p.text)
				continue
			}
			b.top(p.x(r))
		}
		s := b.String()
		sz := value.Size(s)
		r.step(1 + sz)
		r.unit(1 + sz)
		return s
	}
}

func (c *compiler) list(n *syntax.List) code {
	type item struct {
		x      code
		spread bool
		span   syntax.Span
	}
	items := make([]item, len(n.Items))
	for i, it := range n.Items {
		if s, ok := it.(*syntax.Spread); ok {
			items[i] = item{x: c.expr(s.Value), spread: true, span: s.Span()}
		} else {
			items[i] = item{x: c.expr(it)}
		}
	}
	return func(r *run) any {
		out := make([]any, 0, len(items))
		for _, it := range items {
			v := it.x(r)
			if !it.spread {
				out = append(out, v)
				continue
			}
			switch value.KindOf(v) {
			case value.List:
			case value.Null:
				r.raise(fail("null-operand", "spread", it.span))
			default:
				r.raise(fail("wrong-kind", "spread-list", it.span, messages.Val("kind", v)))
			}
			k := value.Len(v)
			r.step(int64(k))
			for i := 0; i < k; i++ {
				out = append(out, value.At(v, i))
			}
		}
		r.unit(1 + int64(len(out)))
		return out
	}
}

func (c *compiler) mapLit(n *syntax.Map) code {
	type entry struct {
		key    string
		size   int64
		keyX   code
		x      code
		spread bool
		span   syntax.Span
	}
	entries := make([]entry, len(n.Entries))
	for i, e := range n.Entries {
		switch e := e.(type) {
		case *syntax.Spread:
			entries[i] = entry{x: c.expr(e.Value), spread: true, span: e.Span()}
		case *syntax.Entry:
			en := entry{x: c.expr(e.Value), span: e.Span()}
			if e.KeyForm == syntax.KeyComputed {
				en.keyX = c.expr(e.KeyExpr)
			} else {
				en.key, en.size = e.Key, value.Size(e.Key)
			}
			entries[i] = en
		}
	}
	return func(r *run) any {
		out := make(map[string]any, len(entries))
		for _, e := range entries {
			switch {
			case e.spread:
				v := e.x(r)
				switch value.KindOf(v) {
				case value.Map:
				case value.Null:
					r.raise(fail("null-operand", "spread", e.span))
				default:
					r.raise(fail("wrong-kind", "spread-map", e.span, messages.Val("kind", v)))
				}
				var w int64
				keys := 0
				value.ForEachKey(v, func(k string) { w += value.Weight(k); keys++ })
				r.step(int64(keys) + w/64)
				value.ForEachKey(v, func(k string) { out[k], _ = value.Get(v, k) })
			case e.keyX != nil:
				k := e.keyX(r)
				ks, ok := k.(string)
				if !ok {
					if k == nil {
						r.raise(fail("null-operand", "key", e.span))
					}
					r.raise(fail("wrong-kind", "key", e.span, messages.Val("found", k)))
				}
				v := e.x(r)
				r.step(value.Size(ks))
				out[ks] = v
			default:
				v := e.x(r)
				r.step(e.size)
				out[e.key] = v
			}
		}
		r.unit(1 + 2*int64(len(out)))
		return out
	}
}
