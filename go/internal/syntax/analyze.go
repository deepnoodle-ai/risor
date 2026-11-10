package syntax

import (
	"sort"
)

// This file is the analysis of package F2, section 3
// (spec/packages/f02-templates-analysis.md): the free names a program reads,
// their static paths, and its calls, found from the tree alone.

// Ref is one read of a free name (F2 3.5), with its span as offsets.
type Ref struct {
	Name string
	Path []string
	Span Span
}

// CallSite is one call of F2 3.6.
type CallSite struct {
	Name string
	// Namespace is the receiver's name for a call n.m(...), else empty.
	Namespace string
	Span      Span
	Args      []Node
	Lambda    bool
}

// Analysis is the report of F2 3.3 without literal and segments.
type Analysis struct {
	Names []string
	Refs  []Ref
	Calls []CallSite
}

type taskKind uint8

const (
	tVisit taskKind = iota
	tBind
	tRestore
)

type task struct {
	kind   taskKind
	n      Node
	name   string
	mark   int
	lambda bool
}

type analyzer struct {
	stack []task
	// The names bound where the walk is: a count for each name, and the log
	// of bindings that a restore unwinds.
	count map[string]int
	log   []string
	out   Analysis
}

// Analyze analyzes a program (F2 3.1). It never fails. It keeps its own
// stack, so a tree of any depth costs no Go stack.
func Analyze(p *Program) *Analysis {
	a := &analyzer{count: map[string]int{}}
	a.push(task{kind: tVisit, n: p})
	for len(a.stack) > 0 {
		t := a.stack[len(a.stack)-1]
		a.stack = a.stack[:len(a.stack)-1]
		switch t.kind {
		case tBind:
			a.count[t.name]++
			a.log = append(a.log, t.name)
		case tRestore:
			for len(a.log) > t.mark {
				n := a.log[len(a.log)-1]
				a.log = a.log[:len(a.log)-1]
				a.count[n]--
			}
		case tVisit:
			a.visit(t.n, t.lambda)
		}
	}
	seen := map[string]bool{}
	for _, r := range a.out.Refs {
		if !seen[r.Name] {
			seen[r.Name] = true
			a.out.Names = append(a.out.Names, r.Name)
		}
	}
	sort.Strings(a.out.Names)
	sort.SliceStable(a.out.Refs, func(i, j int) bool { return a.out.Refs[i].Span.Start < a.out.Refs[j].Span.Start })
	sort.SliceStable(a.out.Calls, func(i, j int) bool { return a.out.Calls[i].Span.Start < a.out.Calls[j].Span.Start })
	return &a.out
}

func (a *analyzer) push(t task) { a.stack = append(a.stack, t) }

func (a *analyzer) free(name string) bool { return a.count[name] == 0 }

// seq pushes tasks so that they run in the order given.
func (a *analyzer) seq(ts []task) {
	for i := len(ts) - 1; i >= 0; i-- {
		a.push(ts[i])
	}
}

func visit(n Node, lambda bool) task { return task{kind: tVisit, n: n, lambda: lambda} }

// pattern gives the tasks of a pattern: each default seen with the names to
// its left, then its name bound (01-language.md 8.3, 8.4).
func pattern(p Pattern, lambda bool) []task {
	var ts []task
	bind := func(n *NamePat) {
		if n != nil && !n.Discard {
			ts = append(ts, task{kind: tBind, name: n.Name})
		}
	}
	switch p := p.(type) {
	case *NamePat:
		bind(p)
	case *MapPat:
		for _, f := range p.Fields {
			if f.Default != nil {
				ts = append(ts, visit(f.Default, lambda))
			}
			ts = append(ts, task{kind: tBind, name: f.Name})
		}
		bind(p.Rest)
	case *ListPat:
		for _, e := range p.Elems {
			if e.Default != nil {
				ts = append(ts, visit(e.Default, lambda))
			}
			bind(e.Name)
		}
		bind(p.Rest)
	}
	return ts
}

func (a *analyzer) consts(cs []*Const, result Node, lambda bool) []task {
	var ts []task
	for _, c := range cs {
		ts = append(ts, visit(c.Value, lambda))
		ts = append(ts, pattern(c.Pattern, lambda)...)
	}
	return append(ts, visit(result, lambda))
}

func (a *analyzer) visit(n Node, lambda bool) {
	switch n := n.(type) {
	case nil:
	case *Program:
		a.seq(a.consts(n.Consts, n.Result, lambda))
	case *Block:
		ts := a.consts(n.Consts, n.Result, lambda)
		a.seq(append(ts, task{kind: tRestore, mark: len(a.log)}))
	case *Lambda:
		var ts []task
		for _, p := range n.Params {
			if p.Default != nil {
				ts = append(ts, visit(p.Default, true))
			}
			ts = append(ts, pattern(p.Pattern, true)...)
		}
		ts = append(ts, visit(n.Body, true), task{kind: tRestore, mark: len(a.log)})
		a.seq(ts)
	case *Try:
		m := len(a.log)
		ts := []task{visit(n.Body, lambda), {kind: tRestore, mark: m}}
		if n.Name != "" {
			ts = append(ts, task{kind: tBind, name: n.Name})
		}
		a.seq(append(ts, visit(n.Handler, lambda), task{kind: tRestore, mark: m}))
	case *Ident:
		if a.free(n.Name) {
			a.out.Refs = append(a.out.Refs, Ref{Name: n.Name, Path: []string{}, Span: n.Span()})
		}
	case *Call:
		if a.free(n.Name) {
			a.out.Calls = append(a.out.Calls, CallSite{Name: n.Name, Span: n.Span(), Args: n.Args, Lambda: lambda})
		}
		a.nodes(n.Args, lambda)
	case *Member, *Index, *MethodCall, *OptChain:
		a.chain(n, lambda)
	default:
		a.nodes(Children(n), lambda)
	}
}

func (a *analyzer) nodes(ns []Node, lambda bool) {
	for i := len(ns) - 1; i >= 0; i-- {
		a.push(visit(ns[i], lambda))
	}
}

// chain walks a chain from its outermost step down to its primary, without
// recursion, and reports the read of a free primary with its path (F2 3.5).
func (a *analyzer) chain(top Node, lambda bool) {
	var spine []Node // outermost first
	cur := top
loop:
	for {
		switch c := cur.(type) {
		case *Member:
			spine = append(spine, c)
			cur = c.X
		case *Index:
			spine = append(spine, c)
			a.push(visit(c.Index, lambda))
			cur = c.X
		case *MethodCall:
			spine = append(spine, c)
			a.nodes(c.Args, lambda)
			if id, ok := c.X.(*Ident); ok && !c.Optional && a.free(id.Name) {
				a.out.Calls = append(a.out.Calls, CallSite{Name: id.Name + "." + c.Name, Namespace: id.Name,
					Span: c.Span(), Args: c.Args, Lambda: lambda})
			}
			cur = c.X
		case *OptChain:
			spine = append(spine, c)
			cur = c.X
		default:
			break loop
		}
	}
	id, ok := cur.(*Ident)
	if !ok {
		a.push(visit(cur, lambda))
		return
	}
	if !a.free(id.Name) {
		return
	}
	ref := Ref{Name: id.Name, Path: []string{}, Span: id.Span()}
	inner := Node(id)
	for i := len(spine) - 1; i >= 0; i-- {
		s := spine[i]
		// A parenthesis ends a chain: the step starts before its operand.
		if s.Span().Start != inner.Span().Start {
			break
		}
		if m, ok := s.(*Member); ok {
			ref.Path = append(ref.Path, m.Name)
			ref.Span.End = m.NameSpan.End
		} else if x, ok := s.(*Index); ok {
			str, ok := x.Index.(*String)
			if !ok {
				break
			}
			ref.Path = append(ref.Path, str.Value)
			ref.Span.End = x.Span().End
		} else if _, ok := s.(*MethodCall); ok {
			break
		}
		inner = s
	}
	a.out.Refs = append(a.out.Refs, ref)
}

// Literal reports the value of a program that is a literal expression
// (F2 3.7): null, a bool, a float64, a string, []any or map[string]any.
func Literal(p *Program) (any, bool) {
	if len(p.Consts) > 0 {
		return nil, false
	}
	return literal(p.Result)
}

func literal(n Node) (any, bool) {
	switch n := n.(type) {
	case *Null:
		return nil, true
	case *Bool:
		return n.Value, true
	case *Number:
		return n.Value, true
	case *String:
		return n.Value, true
	case *Unary:
		if num, ok := n.X.(*Number); ok && n.Op == "-" {
			v := -num.Value
			if v == 0 {
				v = 0
			}
			return v, true
		}
	case *List:
		out := make([]any, 0, len(n.Items))
		for _, it := range n.Items {
			v, ok := literal(it)
			if !ok {
				return nil, false
			}
			out = append(out, v)
		}
		return out, true
	case *Map:
		out := make(map[string]any, len(n.Entries))
		for _, e := range n.Entries {
			en, ok := e.(*Entry)
			if !ok || en.Shorthand || en.KeyForm == KeyComputed {
				return nil, false
			}
			v, ok := literal(en.Value)
			if !ok {
				return nil, false
			}
			out[en.Key] = v
		}
		return out, true
	}
	return nil, false
}
