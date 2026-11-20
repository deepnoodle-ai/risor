package gen

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
)

// This file writes the forms of package F17, step E1
// (spec/packages/f17-recursion.md 11): const-bound lambdas whose parameters
// and result have types and that refer to their own names. Some end: a
// number falls to 0, or the data runs out. Some reach the depth limit, and
// some the step limit. Some pass their own name to map, flatMap, filter or
// reduce. A loose case may leave a type out, which is
// recursion-needs-types, or write a type that does not fit. The bodies are
// trees, not text, so the soundness judge finds the node of every failure.

// selfRec is one shape of a recursive lambda: its parameters' types, its
// result's type, and its body, built from its own name and its parameters'
// names.
type selfRec struct {
	params []string
	result string
	body   func(g *Gen, self string, p []string) ast.Expr
	// res is the generator's type of the result.
	res *Ty
	// ends is set when a call with any arguments of the parameters' types
	// ends at a value or at the depth limit, in a number of steps linear in
	// the depth limit. A lambda that may not is called only where it is
	// written, with a small argument.
	ends bool
}

func sid(n string) ast.Expr                  { return &ast.Ident{Name: n} }
func snum(n int) ast.Expr                    { return &ast.Num{Text: fmt.Sprint(n)} }
func sbin(op string, x, y ast.Expr) ast.Expr { return &ast.Binary{Op: op, X: x, Y: y} }
func scond(t, a, b ast.Expr) ast.Expr        { return &ast.Cond{Test: t, Then: a, Else: b} }
func scall(f string, args ...ast.Expr) ast.Expr {
	return &ast.Call{Name: f, Args: args}
}
func smeth(x ast.Expr, m string, args ...ast.Expr) ast.Expr {
	return &ast.MethodCall{X: x, Name: m, Args: args}
}
func slist(xs ...ast.Expr) *ast.List {
	l := &ast.List{}
	for _, x := range xs {
		l.Items = append(l.Items, ast.Item{X: x})
	}
	return l
}

// done is p <= 0, the test of a number that falls.
func done(p string) ast.Expr { return sbin("<=", sid(p), snum(0)) }

// less is p - 1.
func less(p string) ast.Expr { return sbin("-", sid(p), snum(1)) }

var selfRecs = []selfRec{
	// A number that falls to 0.
	{[]string{"int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		return scond(done(p[0]), snum(0), sbin("+", scall(f, less(p[0])), sid(p[0])))
	}, tyInt, true},
	{[]string{"int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		return scond(done(p[0]), snum(1), sbin("*", snum(2), scall(f, less(p[0]))))
	}, tyInt, true},
	{[]string{"int", "int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		return scond(done(p[0]), sid(p[1]), scall(f, less(p[0]), sbin("+", sid(p[1]), sid(p[0]))))
	}, tyInt, true},
	{[]string{"int"}, "string", func(g *Gen, f string, p []string) ast.Expr {
		return scond(done(p[0]), &ast.Str{Quote: '"'}, sbin("+", scall(f, less(p[0])), &ast.Str{Val: "a", Quote: '"'}))
	}, tyStr, true},
	{[]string{"int"}, "int[]", func(g *Gen, f string, p []string) ast.Expr {
		l := slist(sid(p[0]))
		l.Items = append(l.Items, ast.Item{Spread: true, X: scall(f, less(p[0]))})
		return scond(done(p[0]), slist(), l)
	}, listOf(tyInt, -1), true},
	{[]string{"int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		k := g.fresh("k")
		return &ast.If{
			Tests: []ast.Expr{done(p[0])},
			Then:  []*ast.Branch{{Result: snum(0)}},
			Else: &ast.Branch{
				Consts: []*ast.Const{{Pat: &ast.NamePat{Name: k}, Value: less(p[0])}},
				Result: sbin("+", scall(f, sid(k)), snum(1)),
			},
		}
	}, tyInt, true},
	// Its own name given to a library method.
	{[]string{"int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		return scond(done(p[0]), snum(0), sbin("+", &ast.Index{X: smeth(slist(less(p[0])), "map", sid(f)), I: snum(0)}, snum(1)))
	}, tyInt, true},
	{[]string{"int"}, "int[]", func(g *Gen, f string, p []string) ast.Expr {
		return scond(done(p[0]), slist(snum(0)), smeth(slist(less(p[0])), "flatMap", sid(f)))
	}, listOf(tyInt, -1), true},
	{[]string{"int"}, "int[]", func(g *Gen, f string, p []string) ast.Expr {
		x := g.fresh("x")
		lam := &ast.Lambda{Bare: x, Body: scond(sbin("==", sid(x), sid(p[0])), slist(sid(x)), scall(f, sid(x)))}
		return scond(done(p[0]), slist(), smeth(slist(sid(p[0]), less(p[0])), "flatMap", lam))
	}, listOf(tyInt, -1), true},
	{[]string{"int"}, "bool", func(g *Gen, f string, p []string) ast.Expr {
		n := scall("len", smeth(slist(less(p[0])), "filter", sid(f)))
		return scond(done(p[0]), &ast.Bool{V: true}, sbin("==", n, snum(1)))
	}, tyBool, true},
	{[]string{"int", "int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		return scond(done(p[1]), sid(p[0]), smeth(slist(less(p[1])), "reduce", sid(f), sbin("+", sid(p[0]), sid(p[1]))))
	}, tyInt, true},
	{[]string{"int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		k := g.fresh("k")
		lam := &ast.Lambda{Bare: k, Body: sbin("+", scall(f, sid(k)), sid(p[0]))}
		return scond(done(p[0]), snum(0), &ast.Index{X: smeth(slist(less(p[0])), "map", lam), I: snum(0)})
	}, tyInt, true},
	// The depth of data.
	{[]string{"any"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		a, b := g.fresh("a"), g.fresh("b")
		add := &ast.Lambda{Params: []ast.Param{{Pat: &ast.NamePat{Name: a}}, {Pat: &ast.NamePat{Name: b}}}, Body: sbin("+", sid(a), sid(b))}
		sum := smeth(smeth(sid(p[0]), "map", sid(f)), "reduce", add, snum(1))
		return scond(&ast.As{Form: "is", X: sid(p[0]), Type: "any[]"}, sum, snum(1))
	}, tyInt, true},
	{[]string{"any"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		next := &ast.Member{X: sid(p[0]), Name: "next", Opt: true}
		return scond(sbin("==", sid(p[0]), &ast.Null{}), snum(0), sbin("+", snum(1), scall(f, next)))
	}, tyInt, true},
	// The depth limit, whatever the argument.
	{[]string{"int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		return scall(f, sbin("+", sid(p[0]), snum(1)))
	}, tyInt, true},
	{[]string{"int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		return sbin("+", snum(1), scall(f, less(p[0])))
	}, tyInt, true},
	// The step limit: two calls of itself at each level.
	{[]string{"int"}, "int", func(g *Gen, f string, p []string) ast.Expr {
		return scond(done(p[0]), snum(1), sbin("+", scall(f, less(p[0])), scall(f, less(p[0]))))
	}, tyInt, false},
}

// selfRecConst writes a const line of a recursive lambda, or nil. A lambda
// that ends is bound, so that later expressions may call it; one that may
// not is called in a const line of its own, with a small argument.
func (g *Gen) selfRecConst(d int) *ast.Const {
	if g.opts.NoE1 || g.opts.NoF16 {
		return nil
	}
	r := g.r
	s := selfRecs[r.IntN(len(selfRecs))]
	if s.params[0] == "any" && g.opts.NoF4 {
		return nil // "is" is a form of F4
	}
	g.e1, g.f16 = true, true
	name := g.fresh("f")
	var pnames []string
	for range s.params {
		pnames = append(pnames, g.fresh("p"))
	}
	lam := &ast.Lambda{Result: s.result, Body: s.body(g, name, pnames)}
	for i, p := range pnames {
		lam.Params = append(lam.Params, ast.Param{Pat: &ast.NamePat{Name: p}, Type: s.params[i]})
	}
	if g.loose && r.IntN(3) == 0 {
		switch r.IntN(4) {
		case 0: // no result type: recursion-needs-types
			lam.Result = ""
		case 1: // a parameter with no type: recursion-needs-types
			lam.Params[r.IntN(len(lam.Params))].Type = ""
		case 2: // a result type the body does not fit
			lam.Result = pick(r, []string{"string", "bool", "int[]", "null"})
		default: // a parameter type the arguments do not fit
			lam.Params[r.IntN(len(lam.Params))].Type = pick(r, []string{"string", "bool", "int[]"})
		}
	}
	sig := &lamSig{result: s.res, req: len(s.params)}
	for _, p := range s.params {
		if p == "any" {
			sig.params = append(sig.params, tyAny)
		} else {
			sig.params = append(sig.params, tyInt)
		}
	}
	k := &ast.Const{Pat: &ast.NamePat{Name: name}, Value: lam}
	if s.ends {
		g.bindLam(name, sig)
		return k
	}
	// A lambda that may not end in steps linear in the depth limit is
	// called once, here, in a block, with an argument of at most 12.
	call := &ast.Call{Name: name}
	for range s.params {
		call.Args = append(call.Args, g.smallInt(0, 12))
	}
	v := g.fresh("c")
	g.bindName(v, s.res)
	return &ast.Const{Pat: &ast.NamePat{Name: v}, Value: &ast.Block{Consts: []*ast.Const{k}, Result: call}}
}
