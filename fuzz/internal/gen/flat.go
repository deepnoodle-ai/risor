package gen

import "github.com/deepnoodle-ai/risor/fuzz/internal/ast"

// The lengths of a flat chain: enough links that an implementation which
// spends stack on each one overflows, and few enough that the chain runs
// within the default limits. A projection with a call of map costs 5 steps
// and 4 units a link.
const (
	flatMin = 1_000
	flatMax = 20_000
)

// flatChain wraps x in a long chain that opens no level of nesting, so its
// length must cost an implementation no stack (01-language.md 13.1 and 13.2
// item 6). The value of the chain is x in two lists, or x itself for a chain
// of as alone.
//
// The links are of three kinds. The receiver of a method call is no level
// (13.1). A star's level ends with its projection, which the call of map
// ends (f07-queries.md 2.5). An as opens a level for its type only
// (f04-conversion.md 2.8). The chain mixes the first two, and ends in a run
// of as.
func (g *Gen) flatChain(x ast.Expr) ast.Expr {
	r := g.r
	f4, f7 := !g.opts.NoF4, !g.opts.NoF7
	links := flatMin + r.IntN(flatMax-flatMin+1)
	if f4 && r.IntN(3) == 0 {
		// as alone, on the value itself.
		for i := 0; i < links; i++ {
			x = &ast.As{Form: "as", X: x, Type: "any"}
		}
		return x
	}
	one := func(e ast.Expr) ast.Expr { return &ast.List{Items: []ast.Item{{X: e}}} }
	x = one(one(x))
	v := g.fresh("v")
	same := func() ast.Expr { return &ast.Lambda{Bare: v, Body: &ast.Ident{Name: v}} }
	stars := 0
	if f7 {
		stars = r.IntN(3) // of every 2 links: none, one or both have a star
	}
	for i := 0; i < links; i++ {
		recv := x
		if i%2 < stars {
			recv = &ast.Star{X: x}
		}
		x = &ast.MethodCall{X: recv, Name: "map", Args: []ast.Expr{same()}}
	}
	if f4 && r.IntN(2) == 0 {
		for i, n := 0, flatMin+r.IntN(flatMax-flatMin+1); i < n; i++ {
			x = &ast.As{Form: "as", X: x, Type: "any"}
		}
	}
	return x
}
