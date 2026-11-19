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
// recursion-needs-types, or write a type that does not fit.

// selfRec is one shape of a recursive lambda: its parameters' types, its
// result's type, its body with %[1]s for its own name and %[2]s, %[3]s for
// its parameters, and whether it may be called with any whole number.
type selfRec struct {
	params []string
	result string
	body   string
	// res is the generator's type of the result.
	res *Ty
	// ends is set when a call with any arguments of the parameters' types
	// ends at a value or at the depth limit, in a number of steps linear in
	// the depth limit. A lambda that may not is called only where it is
	// written, with a small argument.
	ends bool
}

var selfRecs = []selfRec{
	// A number that falls to 0.
	{[]string{"int"}, "int", "%[2]s <= 0 ? 0 : %[1]s(%[2]s - 1) + %[2]s", tyInt, true},
	{[]string{"int"}, "int", "%[2]s <= 0 ? 1 : 2 * %[1]s(%[2]s - 1)", tyInt, true},
	{[]string{"int", "int"}, "int", "%[2]s <= 0 ? %[3]s : %[1]s(%[2]s - 1, %[3]s + %[2]s)", tyInt, true},
	{[]string{"int"}, "string", "%[2]s <= 0 ? \"\" : %[1]s(%[2]s - 1) + \"a\"", tyStr, true},
	{[]string{"int"}, "int[]", "%[2]s <= 0 ? [] : [%[2]s, ...%[1]s(%[2]s - 1)]", listOf(tyInt, -1), true},
	{[]string{"int"}, "int", "if (%[2]s <= 0) { 0 } else { const k = %[2]s - 1; %[1]s(k) + 1 }", tyInt, true},
	// Its own name given to a library method.
	{[]string{"int"}, "int", "%[2]s <= 0 ? 0 : [%[2]s - 1].map(%[1]s)[0] + 1", tyInt, true},
	{[]string{"int"}, "int[]", "%[2]s <= 0 ? [0] : [%[2]s - 1].flatMap(%[1]s)", listOf(tyInt, -1), true},
	{[]string{"int"}, "int[]", "%[2]s <= 0 ? [] : [%[2]s, %[2]s - 1].flatMap(x => x == %[2]s ? [x] : %[1]s(x))", listOf(tyInt, -1), true},
	{[]string{"int"}, "bool", "%[2]s <= 0 ? true : len([%[2]s - 1].filter(%[1]s)) == 1", tyBool, true},
	{[]string{"int", "int"}, "int", "%[3]s <= 0 ? %[2]s : [%[3]s - 1].reduce(%[1]s, %[2]s + %[3]s)", tyInt, true},
	{[]string{"int"}, "int", "%[2]s <= 0 ? 0 : [%[2]s - 1].map(k => %[1]s(k) + %[2]s)[0]", tyInt, true},
	// The depth of data.
	{[]string{"any"}, "int", "%[2]s is any[] ? %[2]s.map(%[1]s).reduce((a, b) => a + b, 1) : 1", tyInt, true},
	{[]string{"any"}, "int", "%[2]s == null ? 0 : 1 + %[1]s(%[2]s?.next)", tyInt, true},
	// The depth limit, whatever the argument.
	{[]string{"int"}, "int", "%[1]s(%[2]s + 1)", tyInt, true},
	{[]string{"int"}, "int", "1 + %[1]s(%[2]s - 1)", tyInt, true},
	// The step limit: two calls of itself at each level.
	{[]string{"int"}, "int", "%[2]s <= 0 ? 1 : %[1]s(%[2]s - 1) + %[1]s(%[2]s - 1)", tyInt, false},
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
	args := []any{name}
	for _, p := range pnames {
		args = append(args, p)
	}
	lam := &ast.Lambda{Result: s.result, Body: &ast.Raw{Text: fmt.Sprintf(s.body, args...)}}
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
