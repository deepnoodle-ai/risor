package eval

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
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
	// Removed is the set of removed library entries (03-host.md 3.3.1): a
	// removed method is absent from the run, as a name no entry has.
	Removed map[string]bool
	// GoTypes are the Go types behind the variables declared from Go
	// types, by name, for the typed layer (package F11). Nil turns it off.
	GoTypes map[string]reflect.Type
	// Entry compiles the program as an entry of a REPL session (package F12
	// 5 item 6). Its first len(Held) top-level const lines are the lines
	// the session holds: they bind the values Held gives, by the slots of
	// their patterns, and are not run, so they charge nothing. RunEntry
	// then gives the values of every top-level const line.
	Entry bool
	Held  [][]any
}

// Program is a compiled program. It does not change, and several runs may
// use it at once.
type Program struct {
	cfg        *Config
	file       *syntax.File
	root       code
	nslots     int
	nvars      int     // the variables the program reads, numbered densely
	constSlots [][]int // for Entry: the slots of each top-level const line
	pool       sync.Pool
	// TypedPlans counts the reads the typed layer planned (package F11).
	TypedPlans int
}

type lambdaCode struct {
	params  []paramCode
	body    code
	nparams int
	// simple holds the slot of each parameter (-1 for none) when every
	// parameter is a name without a default; then a bind is one store.
	simple []int
	// ref is set when the typed layer may give the lambda's element
	// parameter by reference: every use of it is a planned field read
	// (typed.go, package F11).
	ref *refInfo
}

type paramCode func(r *run, v any, present bool)

type binder func(r *run, v any)

func (l *lambdaCode) invoke(r *run, args []any) any {
	for i := 0; i < l.nparams; i++ {
		if i < len(args) {
			l.bind(r, i, args[i], true)
		} else {
			l.bind(r, i, nil, false)
		}
	}
	return l.body(r)
}

// bind binds parameter i.
func (l *lambdaCode) bind(r *run, i int, v any, present bool) {
	if l.simple == nil {
		l.bindSlow(r, i, v, present)
		return
	}
	if s := l.simple[i]; s >= 0 {
		r.slots[s] = v
	}
}

//go:noinline
func (l *lambdaCode) bindSlow(r *run, i int, v any, present bool) { l.params[i](r, v, present) }

type compiler struct {
	cfg      *Config
	res      *static.Result
	lambdas  map[*syntax.Lambda]*lambdaCode
	lamDepth int
	vars     map[int]int // variable index to the program's dense index
	// goSlots are the Go types of the lambda parameters the typed layer
	// knows, while their lambda's body is compiled (typed.go).
	goSlots map[int]reflect.Type
	// slotUses counts the planned reads compiled on a typed parameter.
	slotUses map[int]int
	plans    int
}

// Compile compiles a program whose static rules found nothing.
func Compile(prog *syntax.Program, file *syntax.File, res *static.Result, cfg *Config) (p *Program, err error) {
	c := &compiler{cfg: cfg, res: res, lambdas: map[*syntax.Lambda]*lambdaCode{}, vars: map[int]int{},
		goSlots: map[int]reflect.Type{}, slotUses: map[int]int{}}
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("compile: %v", x)
		}
	}()
	var root code
	var constSlots [][]int
	if cfg.Entry {
		root, constSlots = c.entry(prog.Consts, prog.Result)
	} else {
		root = c.block(prog.Consts, prog.Result)
	}
	return &Program{cfg: cfg, file: file, root: root, nslots: res.NSlots, nvars: len(c.vars), constSlots: constSlots, TypedPlans: c.plans}, nil
}

// Run runs the program. It returns the value or one run diagnostic, and the
// usage in both cases.
func (p *Program) Run(ctx context.Context, vars Vars) (result any, usage Usage, d *diag.Diagnostic) {
	result, usage, _, d = p.RunEntry(ctx, vars)
	return result, usage, d
}

// RunEntry runs the program as Run does. For a program compiled as an
// entry (Config.Entry) that succeeds, it also gives the values of each
// top-level const line, by the slots of its pattern.
func (p *Program) RunEntry(ctx context.Context, vars Vars) (result any, usage Usage, held [][]any, d *diag.Diagnostic) {
	if ctx == nil {
		ctx = context.Background()
	}
	r := p.getRun()
	r.ctx, r.vars = ctx, vars
	if t := p.cfg.Limits.Timeout; t > 0 {
		r.deadline = time.Now().Add(t)
	}
	v, f := r.protectAll(p.root)
	usage = Usage{Steps: r.steps, Alloc: r.units}
	var trace []diag.TraceEntry
	if f != nil {
		trace = r.trace(p.file)
	} else if p.cfg.Entry {
		held = make([][]any, len(p.constSlots))
		for i, slots := range p.constSlots {
			held[i] = make([]any, len(slots))
			for j, s := range slots {
				held[i][j] = r.slots[s]
			}
		}
	}
	p.putRun(r)
	if f != nil {
		dd := f.diagnostic(p.file)
		dd.Trace = trace
		return nil, usage, nil, &dd
	}
	return v, usage, held, nil
}

// getRun gives a run state from the program's pool, or a new one. A run
// state is used by one run at a time; the pool makes a program that runs
// many times allocate no state per run.
func (p *Program) getRun() *run {
	if x, ok := p.pool.Get().(*run); ok {
		return x
	}
	lim := p.cfg.Limits
	r := &run{
		prog:     p,
		slots:    make([]any, p.nslots),
		varVals:  make([]any, p.nvars),
		varSeen:  make([]bool, p.nvars),
		maxSteps: lim.Steps, maxUnits: lim.Alloc, maxDepth: lim.Depth,
		unitEdge: lim.Alloc,
	}
	if lim.Alloc < 0 {
		r.unitEdge = math.MaxInt64
	}
	r.setStepEdge()
	return r
}

// putRun clears a run state of the run's values and returns it to the pool.
// The clearing is bounded by the program's size and the depth limit, not by
// host data.
func (p *Program) putRun(r *run) {
	r.ctx, r.vars, r.pending = nil, nil, nil
	clear(r.slots)
	clear(r.varVals)
	clear(r.varSeen)
	clear(r.args[:cap(r.args)])
	r.args = r.args[:0]
	for _, c := range r.lcs {
		*c = libCall{}
	}
	r.lcTop = 0
	r.strs = nil
	r.steps, r.units, r.depth, r.walk = 0, 0, 0, 0
	r.deadline = time.Time{}
	r.setStepEdge()
	p.pool.Put(r)
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

// entry compiles the top level of a REPL entry (Config.Entry). A held
// lambda line is compiled, since calls name it, but not run.
func (c *compiler) entry(consts []*syntax.Const, result syntax.Node) (code, [][]int) {
	held := c.cfg.Held
	if len(held) > len(consts) {
		panic("more held values than const lines")
	}
	slots := make([][]int, len(consts))
	for i, k := range consts {
		slots[i] = c.patternSlots(k.Pattern)
		if i < len(held) {
			if lam, ok := k.Value.(*syntax.Lambda); ok {
				c.lambdas[lam] = c.lambda(lam)
			}
		}
	}
	body := c.block(consts[len(held):], result)
	return func(r *run) any {
		for i, vals := range held {
			for j, s := range slots[i] {
				r.slots[s] = vals[j]
			}
		}
		return body(r)
	}, slots
}

// patternSlots are the slots a const pattern binds, in the order of the
// pattern. A slot depends on the pattern alone, so a held line's slots
// line up in every entry. _ has no slot and is left out.
func (c *compiler) patternSlots(p syntax.Pattern) []int {
	var out []int
	add := func(key any) {
		if s := c.slot(key); s >= 0 {
			out = append(out, s)
		}
	}
	switch p := p.(type) {
	case *syntax.NamePat:
		add(p)
	case *syntax.MapPat:
		for _, f := range p.Fields {
			add(f)
		}
		if p.Rest != nil {
			add(p.Rest)
		}
	case *syntax.ListPat:
		for _, e := range p.Elems {
			if e.Name != nil {
				add(e.Name)
			}
		}
		if p.Rest != nil {
			add(p.Rest)
		}
	}
	return out
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
	simple := make([]int, 0, len(l.Params))
	for _, p := range l.Params {
		if np, ok := p.Pattern.(*syntax.NamePat); ok && p.Default == nil && simple != nil {
			simple = append(simple, c.slot(np))
		} else {
			simple = nil
		}
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
	lc.simple = simple
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
	case *syntax.Member, *syntax.Index, *syntax.MethodCall, *syntax.OptChain, *syntax.Each:
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
	case *syntax.As:
		return c.as(n)
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
		i, ok := c.vars[b.Var]
		if !ok {
			i = len(c.vars)
			c.vars[b.Var] = i
		}
		name, sp := b.Name, n.Span()
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
		depth, walk, nargs, lcTop := r.depth, r.walk, len(r.args), r.lcTop
		v, f := r.protect(body)
		if f == nil {
			return v
		}
		if !messages.Caught(f.code) {
			r.raise(f)
		}
		r.depth, r.walk, r.lcTop = depth, walk, lcTop
		r.popArgs(nargs)
		if named {
			var msg string
			var steps, units int64
			msg = f.message()
			if !f.ownMsg {
				sz := value.Size(msg)
				steps, units = 1+sz, 1+sz
			}
			if f.hasPath {
				units += 1 + int64(len(f.path)) // F4 5.5
			}
			r.step(steps)
			r.unit(9 + units)
			ev := map[string]any{"code": f.code, "message": msg, "path": nil, "data": nil}
			if f.hasPath {
				ev["path"] = append([]any{}, f.path...)
			}
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
	exprCount := 0
	for _, p := range n.Parts {
		if p.Expr != nil {
			parts = append(parts, part{x: c.expr(p.Expr)})
			exprCount++
		} else if p.Text != "" {
			parts = append(parts, part{text: p.Text})
		}
	}
	sp := n.Span()
	return func(r *run) any {
		var b renderer
		b.r, b.span = r, sp
		var stackVals [8]any
		vals := stackVals[:0]
		if exprCount > len(stackVals) {
			vals = make([]any, 0, exprCount)
		}
		for _, p := range parts {
			if p.x == nil {
				b.addString(p.text)
				continue
			}
			v := p.x(r)
			vals = append(vals, v)
			b.top(v)
		}
		sz := b.resultWeight() / 64
		r.step(1 + sz)
		r.unit(1 + sz)
		return b.finish(func(out *strings.Builder) {
			vi := 0
			for _, p := range parts {
				if p.x == nil {
					out.WriteString(p.text)
					continue
				}
				writeTop(r, out, sp, vals[vi])
				vi++
			}
		})
	}
}

func (c *compiler) list(n *syntax.List) code {
	type item struct {
		x      code
		spread bool
		span   syntax.Span
	}
	items := make([]item, len(n.Items))
	hasSpread := false
	for i, it := range n.Items {
		if s, ok := it.(*syntax.Spread); ok {
			items[i] = item{x: c.expr(s.Value), spread: true, span: s.Span()}
			hasSpread = true
		} else {
			items[i] = item{x: c.expr(it)}
		}
	}
	if !hasSpread {
		return func(r *run) any {
			var stackVals [8]any
			vals := stackVals[:0]
			if len(items) > len(stackVals) {
				vals = make([]any, 0, len(items))
			}
			for _, it := range items {
				vals = append(vals, it.x(r))
			}
			r.unit(1 + int64(len(vals)))
			out := make([]any, len(vals))
			copy(out, vals)
			return out
		}
	}
	return func(r *run) any {
		type evalItem struct {
			v      any
			length int
			spread bool
		}
		var stackVals [8]evalItem
		vals := stackVals[:0]
		if len(items) > len(stackVals) {
			vals = make([]evalItem, 0, len(items))
		}
		total := 0
		for _, it := range items {
			v := it.x(r)
			if !it.spread {
				vals = append(vals, evalItem{v: v})
				total++
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
			vals = append(vals, evalItem{v: v, length: k, spread: true})
			total += k
		}
		r.unit(1 + int64(total))
		out := make([]any, 0, total)
		for _, ev := range vals {
			if !ev.spread {
				out = append(out, ev.v)
				continue
			}
			for i := 0; i < ev.length; i++ {
				out = append(out, value.At(ev.v, i))
			}
		}
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
		type evalEntry struct {
			key    string
			v      any
			spread bool
		}
		var stackVals [8]evalEntry
		vals := stackVals[:0]
		if len(entries) > len(stackVals) {
			vals = make([]evalEntry, 0, len(entries))
		}
		maxSpread := 0
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
				if keys > maxSpread {
					maxSpread = keys
				}
				vals = append(vals, evalEntry{v: v, spread: true})
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
				vals = append(vals, evalEntry{key: ks, v: v})
			default:
				v := e.x(r)
				r.step(e.size)
				vals = append(vals, evalEntry{key: e.key, v: v})
			}
		}
		// Any spread gives a lower bound on the final map's key count, so a
		// spread that alone exceeds the unit limit fails before allocating out.
		if r.maxUnits >= 0 && 1+2*int64(maxSpread) > r.maxUnits-r.units {
			r.unit(1 + 2*int64(maxSpread))
		}
		capHint := len(entries)
		if maxSpread > capHint {
			capHint = maxSpread
		}
		out := make(map[string]any, capHint)
		for _, ev := range vals {
			if ev.spread {
				value.ForEachKey(ev.v, func(k string) { out[k], _ = value.Get(ev.v, k) })
			} else {
				out[ev.key] = ev.v
			}
		}
		r.unit(1 + 2*int64(len(out)))
		return out
	}
}
