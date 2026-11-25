package eval

import (
	"context"
	"sort"
	"time"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/go/v3/internal/static"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// entryPoint is one compiled exported entry point (package F16 slice D2,
// spec/packages/f16-entry-points.md 12.4).
type entryPoint struct {
	name       string
	needs      []code // non-lambda top-level const lines, in text order
	lam        *lambdaCode
	paramSpans []syntax.Span
}

// compileEntries compiles the top-level const lines of prog, its optional
// result expression, and its exported entry points with their needs.
func (c *compiler) compileTop(prog *syntax.Program) (root code, hasResult bool, entries map[string]*entryPoint) {
	constCodes := make([]code, len(prog.Consts))
	for i, k := range prog.Consts {
		constCodes[i] = c.constLine(k)
	}
	if prog.Result != nil {
		hasResult = true
		res := c.expr(prog.Result)
		if len(constCodes) == 0 {
			root = res
		} else {
			root = func(r *run) any {
				for _, k := range constCodes {
					k(r)
				}
				return res(r)
			}
		}
	}
	needs := computeNeeds(prog, c.res)
	for i, k := range prog.Consts {
		if !k.Exported {
			continue
		}
		lam, isLam := k.Value.(*syntax.Lambda)
		np, isName := k.Pattern.(*syntax.NamePat)
		if !isLam || k.Type != nil || !isName || np.Discard {
			continue
		}
		if entries == nil {
			entries = map[string]*entryPoint{}
		}
		var nc []code
		for _, j := range needs[i] {
			if _, isL := prog.Consts[j].Value.(*syntax.Lambda); !isL {
				nc = append(nc, constCodes[j])
			}
		}
		spans := make([]syntax.Span, len(lam.Params))
		for pi, p := range lam.Params {
			spans[pi] = p.Pattern.Span()
		}
		entries[np.Name] = &entryPoint{
			name:       np.Name,
			needs:      nc,
			lam:        c.lambdas[lam],
			paramSpans: spans,
		}
	}
	return root, hasResult, entries
}

// computeNeeds finds, for each top-level const line i, the sorted indices of
// the top-level const lines it transitively refers to (F16 12.4 item 1).
func computeNeeds(prog *syntax.Program, res *static.Result) [][]int {
	n := len(prog.Consts)
	slotOwner := map[int]int{}
	lamOwner := map[*syntax.Lambda]int{}
	for i, k := range prog.Consts {
		if lam, ok := k.Value.(*syntax.Lambda); ok && k.Type == nil {
			if np, ok := k.Pattern.(*syntax.NamePat); ok && !np.Discard {
				lamOwner[lam] = i
				continue
			}
		}
		for _, key := range patternSlotKeys(k.Pattern) {
			if s, ok := res.Slots[key]; ok && s >= 0 {
				slotOwner[s] = i
			}
		}
	}
	direct := make([][]int, n)
	for i, k := range prog.Consts {
		seen := map[int]bool{i: true}
		add := func(j int) {
			if !seen[j] {
				seen[j] = true
				direct[i] = append(direct[i], j)
			}
		}
		syntax.Walk(k, func(node syntax.Node) bool {
			switch node := node.(type) {
			case *syntax.Ident:
				if b := res.Idents[node]; b != nil {
					switch b.Kind {
					case static.Value:
						if j, ok := slotOwner[b.Slot]; ok {
							add(j)
						}
					case static.ConstLambda:
						if j, ok := lamOwner[b.Lambda]; ok {
							add(j)
						}
					}
				}
			case *syntax.Call:
				if callee := res.Calls[node]; callee != nil && callee.Kind == static.Lambda {
					if j, ok := lamOwner[callee.Lambda]; ok {
						add(j)
					}
				}
			}
			return true
		})
	}
	out := make([][]int, n)
	for i := range prog.Consts {
		if !prog.Consts[i].Exported {
			continue
		}
		visited := map[int]bool{i: true}
		queue := append([]int(nil), direct[i]...)
		for _, j := range queue {
			visited[j] = true
		}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, next := range direct[cur] {
				if !visited[next] {
					visited[next] = true
					queue = append(queue, next)
				}
			}
		}
		delete(visited, i)
		if len(visited) > 0 {
			ns := make([]int, 0, len(visited))
			for j := range visited {
				ns = append(ns, j)
			}
			sort.Ints(ns)
			out[i] = ns
		}
	}
	return out
}

func patternSlotKeys(p syntax.Pattern) []any {
	var keys []any
	switch p := p.(type) {
	case *syntax.NamePat:
		if !p.Discard {
			keys = append(keys, p)
		}
	case *syntax.MapPat:
		for _, f := range p.Fields {
			keys = append(keys, f)
		}
		if p.Rest != nil && !p.Rest.Discard {
			keys = append(keys, p.Rest)
		}
	case *syntax.ListPat:
		for _, e := range p.Elems {
			if e.Name != nil && !e.Name.Discard {
				keys = append(keys, e.Name)
			}
		}
		if p.Rest != nil && !p.Rest.Discard {
			keys = append(keys, p.Rest)
		}
	}
	return keys
}

// invokeEntry invokes an entry point's lambda with its own frame linked to
// frame 0 (the program's top-level frame), reading each present host value
// before binding it.
func (l *lambdaCode) invokeEntry(r *run, args []any, present []bool, spans []syntax.Span) any {
	fp, base := r.enter(0, l.size)
	for i := 0; i < l.nparams; i++ {
		if i < len(present) && present[i] {
			v := r.read(args[i], spans[i])
			l.bind(r, i, v, true)
		} else {
			l.bind(r, i, nil, false)
		}
	}
	v := l.body(r)
	r.leave(fp, base)
	return v
}

// HasResult reports whether the compiled program has a result expression.
func (p *Program) HasResult() bool { return p.hasResult }

// CallEntry runs the named entry point with its pre-conformed arguments
// (package F16 slice D2, 12.4). present[i] reports whether argument i was
// supplied; omitted parameters evaluate their defaults.
func (p *Program) CallEntry(ctx context.Context, name string, args []any, present []bool, vars Vars) (result any, usage Usage, d *diag.Diagnostic) {
	ep := p.entries[name]
	if ep == nil {
		return nil, Usage{}, &diag.Diagnostic{Phase: diag.PhaseDeclare, Code: diag.CodeBadName,
			Message: "unknown entry point " + name}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	r := p.getRun()
	r.ctx, r.vars = ctx, vars
	if t := p.cfg.Limits.Timeout; t > 0 {
		r.deadline = time.Now().Add(t)
	}
	v, f := r.protectAll(func(r *run) any {
		for _, k := range ep.needs {
			k(r)
		}
		r.step(1)
		r.enterCall(1)
		out := ep.lam.invokeEntry(r, args, present, ep.paramSpans)
		r.depth--
		return out
	})
	usage = Usage{Steps: r.steps, Allocated: r.units}
	var trace []diag.TraceEntry
	if f != nil {
		if f.hasData && r.hasDeepPtr {
			f.data = value.UnwrapDeep(f.data)
		}
		trace = r.trace(p.file)
	} else if r.hasDeepPtr {
		v = value.UnwrapDeep(v)
	}
	p.putRun(r)
	if f != nil {
		dd := f.diagnostic(p.file, p.whole)
		dd.Trace = trace
		return nil, usage, &dd
	}
	return v, usage, nil
}
