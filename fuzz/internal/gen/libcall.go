package gen

import (
	"strings"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
)

// libCall makes a call of a library entry whose result fits t. Entries,
// their parameters, callbacks and result types come from library.json.
func (g *Gen) libCall(t *Ty, d int) ast.Expr {
	r := g.r
	all := append(append([]*LibEntry(nil), g.funcs...), g.meths...)
	if len(all) == 0 {
		return nil
	}
	for _, i := range r.Perm(len(all))[:min(8, len(all))] {
		e := all[i]
		if e.Name == "fail" && g.tryBody == 0 && !g.loose {
			continue
		}
		if x := g.libCallFor(e, t, d); x != nil {
			return x
		}
	}
	return nil
}

// typeVars lists the type variables of a signature: single upper-case
// letters standing alone.
func typeVars(sig string) []string {
	var out []string
	seen := map[string]bool{}
	for i := 0; i < len(sig); i++ {
		c := sig[i]
		if c < 'A' || c > 'Z' {
			continue
		}
		if i > 0 && isWordByte(sig[i-1]) || i+1 < len(sig) && isWordByte(sig[i+1]) {
			continue
		}
		if !seen[string(c)] {
			seen[string(c)] = true
			out = append(out, string(c))
		}
	}
	return out
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func (g *Gen) boundTy(bound string) *Ty {
	switch bound {
	case "ordered":
		return pick(g.r, []*Ty{tyInt, tyInt, tyStr, tyNum})
	case "number":
		return pick(g.r, []*Ty{tyInt, tyNum})
	}
	return g.randTy(1)
}

func satisfies(t *Ty, bound string) bool {
	switch bound {
	case "ordered":
		return t.ordered()
	case "number":
		return t.isNum()
	}
	return true
}

func (g *Gen) libCallFor(e *LibEntry, want *Ty, d int) ast.Expr {
	r := g.r
	b := map[string]*Ty{}
	// Steer the type variables toward the type asked for.
	first := strings.TrimSpace(splitTop(e.Result, '|')[0])
	if want.K != KAny {
		if len(first) == 1 && first[0] >= 'A' && first[0] <= 'Z' && satisfies(want, e.Where[first]) {
			b[first] = want
		} else if len(first) == 3 && strings.HasSuffix(first, "[]") && want.K == KList && want.Elem != nil && want.Elem.K != KAny {
			if satisfies(want.Elem, e.Where[first[:1]]) {
				b[first[:1]] = want.Elem
			}
		}
	}
	for _, v := range typeVars(e.Signature + " " + e.Receiver) {
		if b[v] == nil {
			b[v] = g.boundTy(e.Where[v])
		}
	}
	rt, nullable := tyFromText(e.Result, b)
	if !g.loose && want.K != KAny && !fits(rt, want) {
		return nil
	}
	var recv ast.Expr
	if e.Receiver != "" {
		rty, _ := tyFromText(e.Receiver, b)
		if g.loose && r.IntN(4) == 0 {
			rty = g.randTy(1)
		}
		recv = g.expr(rty, d)
	}
	req := e.Required()
	n := req
	var rest *LibParam
	for i := range e.Params {
		p := &e.Params[i]
		if p.Rest {
			rest = p
		} else if p.Opt && r.IntN(2) == 0 {
			n = i + 1
		}
	}
	if n < req {
		n = req
	}
	extra := 0
	if rest != nil {
		extra = r.IntN(3)
	}
	if g.loose && r.IntN(7) == 0 {
		n, extra = r.IntN(len(e.Params)+2), 0
	}
	var args []ast.Expr
	for i := range n + extra {
		var p *LibParam
		switch {
		case i < len(e.Params) && !e.Params[i].Rest:
			p = &e.Params[i]
		case rest != nil:
			p = rest
		}
		if p == nil {
			args = append(args, g.expr(g.randTy(1), d))
			continue
		}
		switch {
		case p.Fn != nil && g.loose && r.IntN(10) == 0:
			args = append(args, g.expr(g.randTy(1), d))
		case p.Fn != nil:
			args = append(args, g.callback(p.Fn, b, d))
		case g.loose && r.IntN(15) == 0:
			args = append(args, g.callback(&LibFunc{Params: []string{"any"}, Result: "any"}, nil, d))
		default:
			args = append(args, g.argFor(e, p, b, d))
		}
	}
	var call ast.Expr
	if recv == nil {
		call = &ast.Call{Name: e.Name, Args: args}
	} else {
		call = &ast.MethodCall{X: recv, Name: e.Name, Args: args, Opt: g.loose && r.IntN(10) == 0}
	}
	if nullable && !g.loose && want.K != KAny && want.K != KNull {
		call = &ast.Binary{Op: "??", X: call, Y: g.leaf(want)}
	}
	return call
}

// paramTy picks one alternative of a parameter's type text.
func (g *Gen) paramTy(text string, b map[string]*Ty) *Ty {
	var alts []string
	for _, a := range splitTop(text, '|') {
		a = strings.TrimSpace(a)
		if a != "null" {
			alts = append(alts, a)
		}
	}
	if len(alts) == 0 {
		return tyNull
	}
	return tyAlt(pick(g.r, alts), b)
}

// argFor makes an argument for a parameter that takes a value. Counts and
// lengths are kept small so that a typed program stays within the limits.
func (g *Gen) argFor(e *LibEntry, p *LibParam, b map[string]*Ty, d int) ast.Expr {
	r := g.r
	text := p.Type
	if p.Rest {
		text = strings.TrimSuffix(strings.TrimSpace(text), "[]")
	}
	if strings.TrimSpace(text) == "int" && !g.loose {
		switch e.Name {
		case "repeat":
			return g.smallInt(0, 3)
		case "padStart", "padEnd":
			return g.smallInt(0, 10)
		case "fixed":
			return g.smallInt(0, 4)
		case "range":
			return g.smallInt(-2, 8)
		case "slice":
			return g.smallInt(-1, 5)
		}
		return g.smallInt(0, 5)
	}
	if text == "float | string" && r.IntN(2) == 0 {
		return g.str(pick(r, numericStrings))
	}
	if text == "string" && r.IntN(2) == 0 {
		return g.str(pick(r, []string{",", "", "a", " ", "b", "😀", "ab", "-"}))
	}
	return g.expr(g.paramTy(text, b), d)
}

// callback makes a lambda, or names a const-bound lambda, for a parameter
// that takes a function.
func (g *Gen) callback(fn *LibFunc, b map[string]*Ty, d int) ast.Expr {
	r := g.r
	resTy := func() *Ty {
		if fn.Result == "unknown" {
			if r.IntN(3) > 0 || !g.loose {
				return tyBool
			}
			return g.randTy(1)
		}
		t, _ := tyFromText(fn.Result, b)
		return t
	}
	if r.IntN(5) == 0 {
		for _, bd := range g.visible() {
			if bd.lam == nil || len(bd.lam.params) > len(fn.Params) {
				continue
			}
			if g.loose {
				return &ast.Ident{Name: bd.name}
			}
			ok := fits(bd.lam.result, resTy())
			for i, pt := range bd.lam.params {
				at, _ := tyFromText(fn.Params[i], b)
				if !fits(at, pt) {
					ok = false
				}
			}
			if ok {
				return &ast.Ident{Name: bd.name}
			}
		}
	}
	np := len(fn.Params)
	switch x := r.IntN(10); {
	case x < 4:
		np = min(1, np)
	case x < 5:
		np = 0
	case x < 6 && g.loose:
		np++
	}
	lam := &ast.Lambda{}
	g.push()
	for i := range np {
		pt := tyAny
		if i < len(fn.Params) {
			pt, _ = tyFromText(fn.Params[i], b)
		}
		name := g.fresh("p")
		switch {
		case r.IntN(8) == 0:
			lam.Params = append(lam.Params, ast.Param{Pat: &ast.NamePat{Name: "_"}})
		case pt.K == KMap && len(pt.Fields) > 0 && r.IntN(3) == 0:
			pat := &ast.MapPat{}
			for _, f := range pt.Fields {
				if isName(f.Name) && g.lookup(f.Name) == nil && r.IntN(2) == 0 {
					pat.Fields = append(pat.Fields, ast.Field{Name: f.Name})
					g.bindName(f.Name, f.T)
				} else {
					nm := g.fresh("k")
					pat.Fields = append(pat.Fields, ast.Field{Key: f.Name, KeyStr: !isWord(f.Name), Name: nm})
					g.bindName(nm, f.T)
				}
			}
			lam.Params = append(lam.Params, ast.Param{Pat: pat})
		default:
			lam.Params = append(lam.Params, ast.Param{Pat: &ast.NamePat{Name: name}})
			g.bindName(name, pt)
		}
	}
	lam.Body = g.expr(resTy(), d)
	g.pop()
	if np == 1 && r.IntN(2) == 0 {
		if np, ok := lam.Params[0].Pat.(*ast.NamePat); ok {
			lam.Bare = np.Name
		}
	}
	return lam
}
