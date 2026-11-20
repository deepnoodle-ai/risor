package gen

import (
	"strings"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
)

// This file writes the forms of package F17, step E3
// (spec/packages/f17-recursion.md 13): lambdas with a parameter of a
// function type, which the body calls, gives to a library method or gives on
// to another such lambda, and the calls that give a lambda at it: one
// written there or the name of a const-bound lambda. A loose case may break
// one static rule of 13.2: a function type left out, a value at a function
// parameter, a function parameter read as a value, a call with another count,
// or a higher-order lambda given to map.

// fnHelper is one const-bound lambda that takes a function, as the
// generator writes it.
type fnHelper struct {
	name   string
	params []fnHelperParam
	result *Ty
	lam    *ast.Lambda
}

// fnHelperParam is a parameter of a helper: a function parameter when fn is
// set, else a value parameter of type t.
type fnHelperParam struct {
	name string
	fn   *LibFunc
	t    *Ty
}

// fnText prints a function type: each listed parameter as a word and its
// type, then the result.
func fnText(fn *LibFunc) string {
	words := []string{"x", "y", "z"}
	var b strings.Builder
	b.WriteString("(")
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(words[i] + ": " + p)
	}
	b.WriteString(") => " + fn.Result)
	return b.String()
}

// fnShape builds a helper from the names of its parameters and of the
// helpers written before it in the same block.
type fnShape struct {
	params []fnHelperParam // names are filled in by the builder
	result *Ty
	body   func(g *Gen, p []string, prev []*fnHelper) ast.Expr
	// prev is the count of helpers this one needs written before it, each
	// made from the shape at that index of fnShapes.
	prev []int
}

func fnOf(result string, params ...string) *LibFunc {
	return &LibFunc{Params: params, Result: result}
}

func vparam(t *Ty) fnHelperParam      { return fnHelperParam{t: t} }
func fparam(f *LibFunc) fnHelperParam { return fnHelperParam{fn: f} }

var tyInts = listOf(tyInt, -1)
var tyStrs = listOf(tyStr, -1)

var fnShapes = []fnShape{
	// 0: apply, f(x) + x.
	{[]fnHelperParam{fparam(fnOf("int", "int")), vparam(tyInt)}, tyInt, func(g *Gen, p []string, _ []*fnHelper) ast.Expr {
		return sbin("+", scall(p[0], sid(p[1])), sid(p[1]))
	}, nil},
	// 1: given to map.
	{[]fnHelperParam{vparam(tyInts), fparam(fnOf("string", "int"))}, tyStrs, func(g *Gen, p []string, _ []*fnHelper) ast.Expr {
		return smeth(sid(p[0]), "map", sid(p[1]))
	}, nil},
	// 2: a predicate given to filter.
	{[]fnHelperParam{vparam(tyInts), fparam(fnOf("unknown", "int"))}, tyInts, func(g *Gen, p []string, _ []*fnHelper) ast.Expr {
		return smeth(sid(p[0]), "filter", sid(p[1]))
	}, nil},
	// 3: given to reduce, with a seed.
	{[]fnHelperParam{vparam(tyInts), fparam(fnOf("int", "int", "int")), vparam(tyInt)}, tyInt, func(g *Gen, p []string, _ []*fnHelper) ast.Expr {
		return smeth(sid(p[0]), "reduce", sid(p[1]), sid(p[2]))
	}, nil},
	// 4: called in a lambda given to map.
	{[]fnHelperParam{vparam(tyInts), fparam(fnOf("int", "int"))}, tyInts, func(g *Gen, p []string, _ []*fnHelper) ast.Expr {
		x := g.fresh("x")
		return smeth(sid(p[0]), "map", &ast.Lambda{Bare: x, Body: sbin("+", scall(p[1], sid(x)), snum(1))})
	}, nil},
	// 5: a function type of no parameters.
	{[]fnHelperParam{fparam(fnOf("string"))}, tyStr, func(g *Gen, p []string, _ []*fnHelper) ast.Expr {
		return sbin("+", scall(p[0]), &ast.Str{Val: "!", Quote: '"'})
	}, nil},
	// 6: two listed parameters.
	{[]fnHelperParam{fparam(fnOf("string", "int", "string")), vparam(tyInt)}, tyStr, func(g *Gen, p []string, _ []*fnHelper) ast.Expr {
		return scall(p[0], sid(p[1]), &ast.Str{Val: "k", Quote: '"'})
	}, nil},
	// 7: given to sortBy.
	{[]fnHelperParam{vparam(tyStrs), fparam(fnOf("int", "string"))}, tyStrs, func(g *Gen, p []string, _ []*fnHelper) ast.Expr {
		return smeth(sid(p[0]), "sortBy", sid(p[1]))
	}, nil},
	// 8: a result unknown, tested.
	{[]fnHelperParam{fparam(fnOf("unknown", "int")), vparam(tyInt)}, tyBool, func(g *Gen, p []string, _ []*fnHelper) ast.Expr {
		return scond(scall(p[0], sid(p[1])), &ast.Bool{V: true}, &ast.Bool{V: false})
	}, nil},
	// 9: given on, twice, to the helper of shape 0.
	{[]fnHelperParam{fparam(fnOf("int", "int")), vparam(tyInt)}, tyInt, func(g *Gen, p []string, prev []*fnHelper) ast.Expr {
		h := prev[0].name
		return scall(h, sid(p[0]), scall(h, sid(p[0]), sid(p[1])))
	}, []int{0}},
	// 10: given on to the helper of shape 1.
	{[]fnHelperParam{fparam(fnOf("string", "int")), vparam(tyInt)}, tyStrs, func(g *Gen, p []string, prev []*fnHelper) ast.Expr {
		return scall(prev[0].name, slist(sid(p[1]), sbin("+", sid(p[1]), snum(1))), sid(p[0]))
	}, []int{1}},
}

// buildHelper writes the const line of a helper of shape i, with the
// helpers it needs before it.
func (g *Gen) buildHelper(i int, consts *[]*ast.Const) *fnHelper {
	s := fnShapes[i]
	var prev []*fnHelper
	for _, j := range s.prev {
		prev = append(prev, g.buildHelper(j, consts))
	}
	h := &fnHelper{name: g.fresh("h"), result: s.result}
	var names []string
	lam := &ast.Lambda{Result: annotText(s.result)}
	for _, p := range s.params {
		if p.fn != nil {
			p.name = g.fresh("f")
		} else {
			p.name = g.fresh("v")
		}
		names = append(names, p.name)
		h.params = append(h.params, p)
		typ := ""
		if p.fn != nil {
			typ = fnText(p.fn)
		} else {
			typ = annotText(p.t)
		}
		lam.Params = append(lam.Params, ast.Param{Pat: &ast.NamePat{Name: p.name}, Type: typ})
	}
	lam.Body = s.body(g, names, prev)
	h.lam = lam
	*consts = append(*consts, &ast.Const{Pat: &ast.NamePat{Name: h.name}, Value: lam})
	return h
}

// fnArg writes an argument at a function parameter: a lambda written
// there, the name of a const-bound lambda that fits, or a lambda that calls
// the helper again, as apply(y => apply(z => z, y + 10), 1).
func (g *Gen) fnArg(h *fnHelper, fn *LibFunc, d int) ast.Expr {
	if len(fn.Params) == 1 && fn.Params[0] == "int" && fn.Result == "int" && h.params[0].fn == fn && len(h.params) == 2 && g.r.IntN(5) == 0 {
		y := g.fresh("y")
		g.push()
		g.bindName(y, tyInt)
		inner := g.callback(fn, nil, max(d-1, 0))
		g.pop()
		return &ast.Lambda{Bare: y, Body: scall(h.name, inner, sbin("+", sid(y), g.smallInt(0, 20)))}
	}
	return g.callback(fn, nil, max(d-1, 0))
}

// fnParamConst writes a const line whose value is a block with helpers
// that take a function and a call of the last, or nil.
func (g *Gen) fnParamConst(d int) *ast.Const {
	if g.opts.NoE3 || g.opts.NoF16 {
		return nil
	}
	r := g.r
	g.e3, g.f16 = true, true
	var consts []*ast.Const
	h := g.buildHelper(r.IntN(len(fnShapes)), &consts)
	call := &ast.Call{Name: h.name}
	for _, p := range h.params {
		if p.fn != nil {
			call.Args = append(call.Args, g.fnArg(h, p.fn, d))
		} else {
			call.Args = append(call.Args, g.expr(p.t, max(d-1, 0)))
		}
	}
	var res ast.Expr = call
	t := h.result
	if g.loose && r.IntN(3) == 0 {
		res, t = g.breakFnParam(h, call)
	}
	v := g.fresh("c")
	g.bindName(v, t)
	return &ast.Const{Pat: &ast.NamePat{Name: v}, Value: &ast.Block{Consts: consts, Result: res}}
}

// breakFnParam breaks one static rule of f17-recursion.md 13.2 in a loose
// case, and gives the result and its type.
func (g *Gen) breakFnParam(h *fnHelper, call *ast.Call) (ast.Expr, *Ty) {
	r := g.r
	fi := -1
	for i, p := range h.params {
		if p.fn != nil {
			fi = i
			break
		}
	}
	p := h.params[fi]
	switch r.IntN(5) {
	case 0: // no function type: call-of-parameter, fn-hint (rule 1)
		h.lam.Params[fi].Type = ""
	case 1: // a value at the function parameter (rule 3)
		call.Args[fi] = g.expr(g.randTy(1), 1)
	case 2: // read as a value (rule 4)
		h.lam.Body = slist(sid(p.name))
	case 3: // a call with another count (rule 7)
		args := make([]ast.Expr, len(p.fn.Params)+1)
		for i := range args {
			args[i] = snum(i)
		}
		h.lam.Body = scall(p.name, args...)
	default: // a higher-order lambda given to map (rule 8)
		return smeth(slist(snum(1)), "map", sid(h.name)), g.randTy(1)
	}
	return call, h.result
}
