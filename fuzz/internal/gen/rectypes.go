package gen

import (
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
)

// This file writes the forms of package F17, step E2
// (spec/packages/f17-recursion.md 12): type lines that name themselves and
// one another, values of those types, field reads, joins of two names with
// one structure, as and is, and typed recursive lambdas that walk the data.
// A loose case also writes a cycle with no guard, a reference beside a type
// of its kind, a wrong field and a value that does not fit.

// recShape is one recursive type: its type lines, written from the names
// the generator gives them, a finite value of its first line, and the uses
// of such a value. Each use gives an expression and its rough type.
type recShape struct {
	lines func(n []string) []string
	value func(g *Gen, n []string, depth int) ast.Expr
	uses  []func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty)
	// walk is the body of a typed recursive lambda over a value of the
	// first line: its parameter, result type and body.
	walk func(g *Gen, n []string, self, p string) (string, ast.Expr)
	// bad is a field no line has.
	bad string
}

func word(k string, x ast.Expr) ast.Entry { return ast.Entry{Kind: ast.EntryWord, Key: k, X: x} }
func smap(es ...ast.Entry) *ast.Map    { return &ast.Map{Entries: es} }
func sstr(s string) ast.Expr           { return &ast.Str{Val: s, Quote: '"'} }
func smem(x ast.Expr, f string) ast.Expr {
	return &ast.Member{X: x, Name: f}
}

// sumOf is xs.reduce((a, b) => a + b, 0).
func (g *Gen) sumOf(xs ast.Expr) ast.Expr {
	a, b := g.fresh("a"), g.fresh("b")
	add := &ast.Lambda{Params: []ast.Param{{Pat: &ast.NamePat{Name: a}}, {Pat: &ast.NamePat{Name: b}}}, Body: sbin("+", sid(a), sid(b))}
	return smeth(xs, "reduce", add, snum(0))
}

// mapField is xs.map(k => k.f).
func (g *Gen) mapField(xs ast.Expr, f string) ast.Expr {
	k := g.fresh("k")
	return smeth(xs, "map", &ast.Lambda{Bare: k, Body: smem(sid(k), f)})
}

var recShapes = []recShape{
	// A tree.
	{
		lines: func(n []string) []string { return []string{"{name: string, kids: " + n[0] + "[]}"} },
		value: func(g *Gen, n []string, d int) ast.Expr { return g.treeValue(d) },
		uses: []func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty){
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) { return smem(x, "name"), tyStr },
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) {
				return scall("len", smem(x, "kids")), tyInt
			},
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) {
				return g.mapField(smem(x, "kids"), "name"), listOf(tyStr, -1)
			},
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) {
				return &ast.Member{X: &ast.Index{X: smem(x, "kids"), I: snum(0), Opt: true}, Name: "name", Opt: true}, tyAny
			},
		},
		walk: func(g *Gen, n []string, self, p string) (string, ast.Expr) {
			return "int", sbin("+", snum(1), g.sumOf(smeth(smem(sid(p), "kids"), "map", sid(self))))
		},
		bad: "kid",
	},
	// A list of cells.
	{
		lines: func(n []string) []string { return []string{"{head: int, tail: " + n[0] + " | null}"} },
		value: func(g *Gen, n []string, d int) ast.Expr { return g.cellsValue(1 + g.r.IntN(4)) },
		uses: []func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty){
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) { return smem(x, "head"), tyInt },
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) {
				return &ast.Member{X: smem(x, "tail"), Name: "head", Opt: true}, tyAny
			},
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) {
				return sbin("==", smem(x, "tail"), &ast.Null{}), tyBool
			},
		},
		walk: func(g *Gen, n []string, self, p string) (string, ast.Expr) {
			t := smem(sid(p), "tail")
			return "int", sbin("+", smem(sid(p), "head"), scond(sbin("==", t, &ast.Null{}), snum(0), scall(self, t)))
		},
		bad: "hed",
	},
	// JSON.
	{
		lines: func(n []string) []string {
			return []string{"null | bool | float | string | " + n[0] + "[] | map<" + n[0] + ">"}
		},
		value: func(g *Gen, n []string, d int) ast.Expr { return g.jsonValue(d) },
		uses: []func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty){
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) {
				return &ast.As{Form: "is", X: x, Type: n[0] + "[]"}, tyBool
			},
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) { return sbin("==", x, &ast.Null{}), tyBool },
		},
		walk: func(g *Gen, n []string, self, p string) (string, ast.Expr) {
			return "int", scond(&ast.As{Form: "is", X: sid(p), Type: n[0] + "[]"},
				sbin("+", snum(1), g.sumOf(smeth(sid(p), "map", sid(self)))), snum(1))
		},
		bad: "",
	},
	// A tagged union of expressions.
	{
		lines: func(n []string) []string {
			return []string{`{k: "num", v: float} | {k: "add", l: ` + n[0] + ", r: " + n[0] + "}"}
		},
		value: func(g *Gen, n []string, d int) ast.Expr { return g.exprValue(d) },
		uses: []func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty){
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) { return smem(x, "k"), tyStr },
		},
		walk: func(g *Gen, n []string, self, p string) (string, ast.Expr) {
			return "float", &ast.Match{Subject: smem(sid(p), "k"), Arms: []ast.Arm{
				{Lits: []ast.Expr{sstr("num")}, Result: smem(sid(p), "v")},
				{Lits: []ast.Expr{sstr("add")}, Result: sbin("+", scall(self, smem(sid(p), "l")), scall(self, smem(sid(p), "r")))},
			}}
		},
		bad: "w",
	},
	// Two types that name each other.
	{
		lines: func(n []string) []string {
			return []string{"{name: string, files: " + n[1] + "[], dirs: " + n[0] + "[]}", "{name: string, parent: " + n[0] + " | null}"}
		},
		value: func(g *Gen, n []string, d int) ast.Expr { return g.dirValue(d) },
		uses: []func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty){
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) {
				return g.mapField(smem(x, "files"), "name"), listOf(tyStr, -1)
			},
			func(g *Gen, n []string, x ast.Expr) (ast.Expr, *Ty) { return scall("len", smem(x, "dirs")), tyInt },
		},
		walk: func(g *Gen, n []string, self, p string) (string, ast.Expr) {
			return "int", sbin("+", scall("len", smem(sid(p), "files")), g.sumOf(smeth(smem(sid(p), "dirs"), "map", sid(self))))
		},
		bad: "file",
	},
}

func (g *Gen) treeValue(d int) ast.Expr {
	kids := slist()
	if d > 0 {
		for range g.r.IntN(3) {
			kids.Items = append(kids.Items, ast.Item{X: g.treeValue(d - 1)})
		}
	}
	return smap(word("name", sstr(pick(g.r, fieldNames[:6]))), word("kids", kids))
}

func (g *Gen) cellsValue(n int) ast.Expr {
	var tail ast.Expr = &ast.Null{}
	for range n {
		tail = smap(word("head", snum(g.r.IntN(10))), word("tail", tail))
	}
	return tail
}

func (g *Gen) jsonValue(d int) ast.Expr {
	switch x := g.r.IntN(7); {
	case d <= 0 || x == 0:
		return pick(g.r, []ast.Expr{&ast.Null{}, &ast.Bool{V: true}, snum(g.r.IntN(5)), sstr("j")})
	case x < 4:
		l := slist()
		for range g.r.IntN(3) {
			l.Items = append(l.Items, ast.Item{X: g.jsonValue(d - 1)})
		}
		return l
	default:
		m := smap()
		for i := range g.r.IntN(3) {
			m.Entries = append(m.Entries, word(fmt.Sprintf("j%d", i), g.jsonValue(d-1)))
		}
		return m
	}
}

func (g *Gen) exprValue(d int) ast.Expr {
	if d <= 0 || g.r.IntN(3) == 0 {
		return smap(word("k", sstr("num")), word("v", snum(g.r.IntN(9))))
	}
	return smap(word("k", sstr("add")), word("l", g.exprValue(d-1)), word("r", g.exprValue(d-1)))
}

func (g *Gen) dirValue(d int) ast.Expr {
	files := slist()
	for range g.r.IntN(3) {
		files.Items = append(files.Items, ast.Item{X: smap(word("name", sstr(pick(g.r, fieldNames[:6]))), word("parent", &ast.Null{}))})
	}
	dirs := slist()
	if d > 0 {
		for range g.r.IntN(2) {
			dirs.Items = append(dirs.Items, ast.Item{X: g.dirValue(d - 1)})
		}
	}
	return smap(word("name", sstr("d")), word("files", files), word("dirs", dirs))
}

// recTypeConst writes a const line whose value is a block over a recursive
// type of the program, or nil.
func (g *Gen) recTypeConst() *ast.Const {
	if g.opts.NoE2 || g.opts.NoF16 {
		return nil
	}
	r := g.r
	s := recShapes[r.IntN(len(recShapes))]
	if g.opts.NoF4 && strings.Contains(s.lines([]string{"X", "Y"})[0], "map<") {
		return nil // as and is are forms of F4; JSON is tested by is
	}
	g.e2, g.f16 = true, true
	k := len(s.lines([]string{"X", "Y"}))
	names := make([]string, k)
	for i := range names {
		names[i] = fmt.Sprintf("Rt%d", len(g.types)+i)
	}
	for i, text := range s.lines(names) {
		g.types = append(g.types, ast.TypeLine{Name: names[i], Type: text})
	}
	var consts []*ast.Const
	v := g.fresh("v")
	val := s.value(g, names, 1+r.IntN(3))
	if g.loose && r.IntN(4) == 0 {
		val = g.leaf(g.randTy(1)) // a value that may not fit
	}
	consts = append(consts, &ast.Const{Pat: &ast.NamePat{Name: v}, Type: names[0], Value: val})
	if g.loose && r.IntN(5) == 0 {
		switch r.IntN(3) {
		case 0: // circular-type, direct
			n := fmt.Sprintf("Rt%d", len(g.types))
			g.types = append(g.types, ast.TypeLine{Name: n, Type: n + " | null"})
		case 1: // circular-type, union
			n := fmt.Sprintf("Rt%d", len(g.types))
			g.types = append(g.types, ast.TypeLine{Name: n, Type: "{next: " + n + " | {end: bool}}"})
		default: // a name of a later line, at the top of an earlier one
			n := fmt.Sprintf("Rt%d", len(g.types))
			g.types = append(g.types, ast.TypeLine{Name: n, Type: names[0] + " | null"})
		}
	}
	var res ast.Expr
	var t *Ty
	switch x := r.IntN(8); {
	case x < 2 && !g.opts.NoE1:
		// A typed recursive lambda that walks the value (F17 11).
		f, p := g.fresh("f"), g.fresh("p")
		rt, body := s.walk(g, names, f, p)
		lam := &ast.Lambda{Params: []ast.Param{{Pat: &ast.NamePat{Name: p}, Type: names[0]}}, Result: rt, Body: body}
		consts = append(consts, &ast.Const{Pat: &ast.NamePat{Name: f}, Value: lam})
		g.e1 = true
		res, t = scall(f, sid(v)), tyNum
		if rt == "int" {
			t = tyInt
		}
	case x < 3:
		// A second name with the same structure: a fit and a join.
		alias := fmt.Sprintf("Rt%d", len(g.types))
		text := s.lines(append([]string{alias}, names[1:]...))[0]
		g.types = append(g.types, ast.TypeLine{Name: alias, Type: text})
		w := g.fresh("w")
		consts = append(consts, &ast.Const{Pat: &ast.NamePat{Name: w}, Type: alias, Value: sid(v)})
		res, t = scall("len", slist(sid(v), sid(w))), tyInt
		if r.IntN(2) == 0 {
			res, t = scall("len", slist(scond(&ast.Bool{V: r.IntN(2) == 0}, sid(v), sid(w)))), tyInt
		}
	case x < 4 && !g.opts.NoF4:
		// as and is on data of type any.
		a := g.fresh("a")
		consts = append(consts, &ast.Const{Pat: &ast.NamePat{Name: a}, Type: "any", Value: s.value(g, names, 2)})
		if r.IntN(2) == 0 {
			res, t = &ast.As{Form: "is", X: sid(a), Type: names[0]}, tyBool
		} else {
			u := s.uses[r.IntN(len(s.uses))]
			res, t = u(g, names, &ast.Paren{X: &ast.As{Form: "as", X: sid(a), Type: names[0]}})
		}
	default:
		u := s.uses[r.IntN(len(s.uses))]
		res, t = u(g, names, sid(v))
	}
	if g.loose && s.bad != "" && r.IntN(5) == 0 {
		res, t = smem(sid(v), s.bad), tyAny // no-field, naming the type
	}
	name := g.fresh("c")
	g.bindName(name, t)
	return &ast.Const{Pat: &ast.NamePat{Name: name}, Value: &ast.Block{Consts: consts, Result: res}}
}
