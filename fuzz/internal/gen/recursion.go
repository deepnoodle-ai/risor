package gen

import (
	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
)

// This file writes the forms of package F17, step E0
// (spec/packages/f17-recursion.md): guards on the arms of match, and calls
// of the library function walk.

// guardArms gives some arms of a match a guard (2.1). It may add a guarded
// _ arm before the last plain _, and, in a typed program, a plain arm after
// a guarded one that lists its literals again, which a guarded arm allows
// (4.1).
func (g *Gen) guardArms(m *ast.Match, t *Ty, d int) {
	r := g.r
	g.f17 = true
	var arms []ast.Arm
	for _, a := range m.Arms {
		if a.Default || r.IntN(3) > 0 {
			arms = append(arms, a)
			continue
		}
		a.Guard = g.test(d)
		arms = append(arms, a)
		if !g.loose && r.IntN(3) == 0 {
			lits := make([]ast.Expr, len(a.Lits))
			for i, l := range a.Lits {
				lits[i] = ast.CloneExpr(l)
			}
			arms = append(arms, ast.Arm{Lits: lits, Result: g.expr(t, d)})
		}
	}
	if r.IntN(4) == 0 {
		gd := ast.Arm{Default: true, Guard: g.test(d), Result: g.expr(t, d)}
		at := len(arms)
		for i, a := range arms {
			if a.Default && a.Guard == nil {
				at = i
				break
			}
		}
		if g.loose && r.IntN(4) == 0 {
			at = len(arms) // after a plain _: unreachable-arm
		}
		arms = append(arms[:at:at], append([]ast.Arm{gd}, arms[at:]...)...)
	}
	m.Arms = arms
}

// walkCall makes a call of walk (5) whose result fits t: a walk bounded by
// its depth parameter, a walk of any data, and in loose programs the calls
// that the static rules or the argument tests reject.
func (g *Gen) walkCall(t *Ty, d int) ast.Expr {
	r := g.r
	if g.opts.Exclude["walk"] {
		return nil
	}
	g.f17 = true
	et := tyAny
	if t.K == KList && t.Elem != nil {
		et = t.Elem
	}
	if et.K == KAny {
		et = g.randTy(1)
	}
	call := func(args ...ast.Expr) ast.Expr { return &ast.Call{Name: "walk", Args: args} }
	if g.loose && r.IntN(4) == 0 {
		switch r.IntN(5) {
		case 0: // arity, a static rule
			return call(g.expr(et, d))
		case 1: // a value at children: arg-mismatch, or the argument test
			return call(g.expr(et, d), g.expr(g.randTy(1), d))
		case 2: // a lambda at root: not-a-value
			return call(g.callback(&LibFunc{Params: []string{"any"}, Result: "any"}, nil, d), g.walkFn(et, d))
		case 3: // a lambda with three parameters: arity on the lambda
			lam := g.walkFn(et, d).(*ast.Lambda)
			lam.Params = append(lam.Params, ast.Param{Pat: &ast.NamePat{Name: g.fresh("p")}})
			return call(g.expr(et, d), lam)
		default: // three arguments
			return call(g.expr(et, d), g.walkFn(et, d), g.expr(g.randTy(1), d))
		}
	}
	if r.IntN(3) == 0 && (g.loose || !g.opts.NoF4) {
		// Any data: the children are a field of each node, or the node. A
		// typed program writes the root `as any`, so that the reads of the
		// callback are checked against any and not against whatever type the
		// root's expression has.
		g.push()
		n, k := g.fresh("n"), g.fresh("k")
		g.bindName(n, tyAny)
		g.bindName(k, tyInt)
		var body ast.Expr = &ast.Member{X: &ast.Ident{Name: n}, Name: pick(r, []string{"kids", "a", "b"}), Opt: r.IntN(3) == 0}
		if r.IntN(3) == 0 {
			body = &ast.Ident{Name: n}
		}
		if r.IntN(2) == 0 {
			body = &ast.Cond{Test: &ast.Binary{Op: "<", X: &ast.Ident{Name: k}, Y: g.smallInt(0, 3)}, Then: body, Else: &ast.Null{}}
		}
		lam := &ast.Lambda{Params: []ast.Param{{Pat: &ast.NamePat{Name: n}}, {Pat: &ast.NamePat{Name: k}}}, Body: body}
		if r.IntN(2) == 0 {
			lam.Params, lam.Bare = nil, n
			if c, ok := body.(*ast.Cond); ok {
				lam.Body = c.Then
			}
		}
		g.pop()
		root := g.expr(tyAny, d)
		if !g.loose {
			root = &ast.As{Form: "as", X: root, Type: "any"}
		}
		return call(root, lam)
	}
	return call(g.expr(et, d), g.walkFn(et, d))
}

// walkFn makes a callback of walk: (node, depth) => depth < K ? [...] :
// null, whose list holds nodes of type et made with the node in scope. K is
// small and the list short, so a walk visits at most 15 nodes. A loose
// program may give a result of any kind, or a list with no bound, which
// stops at the depth limit.
func (g *Gen) walkFn(et *Ty, d int) ast.Expr {
	r := g.r
	g.push()
	defer g.pop()
	n, k := g.fresh("n"), g.fresh("k")
	g.bindName(n, et)
	g.bindName(k, tyInt)
	var items []ast.Item
	for range r.IntN(3) {
		items = append(items, ast.Item{X: g.expr(et, d-1)})
	}
	kids := &ast.List{Items: items}
	var body ast.Expr = &ast.Cond{Test: &ast.Binary{Op: "<", X: &ast.Ident{Name: k}, Y: g.smallInt(0, 3)}, Then: kids, Else: &ast.Null{}}
	if r.IntN(4) == 0 {
		body = &ast.If{Tests: []ast.Expr{&ast.Binary{Op: ">=", X: &ast.Ident{Name: k}, Y: g.smallInt(0, 3)}},
			Then: []*ast.Branch{{Result: &ast.Null{}}}, Else: &ast.Branch{Result: kids}}
	}
	if g.loose && r.IntN(4) == 0 {
		body = g.expr(g.randTy(1), d)
	}
	if g.loose && r.IntN(8) == 0 && len(items) > 0 {
		body = kids
	}
	params := []ast.Param{{Pat: &ast.NamePat{Name: n}}, {Pat: &ast.NamePat{Name: k}}}
	return &ast.Lambda{Params: params, Body: body}
}
