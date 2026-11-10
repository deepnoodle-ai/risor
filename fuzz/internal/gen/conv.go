package gen

import (
	"fmt"
	"strings"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
	"github.com/deepnoodle-ai/risor/fuzz/internal/types"
)

// Packages F4 and F5: x as T and x is T with program types
// (f04-conversion.md 2), named types declared by the case field types
// (f04-conversion.md 9.1), tagged unions (f05-unions.md 2), tests of a tag
// that narrow (f05-unions.md 5.2, 5.3) and map literals against a union
// (f05-unions.md 6).

// unionVar is a variable of the environment whose declared type has a
// tagged union.
type unionVar struct {
	name string
	t    *types.Type
}

// tagWords and their sets of strings make the tags of generated unions.
var tagWords = []string{"kind", "type", "op"}
var tagStrings = [][]string{{"a", "b", "c"}, {"circle", "rect", "tri"}, {"add", "del", "get"}}

// unionType makes a tagged union of two or three variants. Each variant
// requires the tag with a literal set of its own, and has up to two other
// fields; some variants are open. A shared field lets a test of two tags
// narrow to something with a field in common.
func (g *Gen) unionType() *types.Type {
	r := g.r
	i := r.IntN(len(tagWords))
	tag, strs := tagWords[i], tagStrings[i]
	n := 2 + r.IntN(2)
	var vs []*types.Record
	fieldPool := []string{"n", "s", "x", "name", "ok"}
	for v := range n {
		lits := []string{strs[v]}
		rec := &types.Record{Fields: []types.Field{{Key: tag, T: &types.Type{Lits: lits}}}}
		rec.Fields = append(rec.Fields, types.Field{Key: "id", T: types.Prim("int")})
		for _, k := range r.Perm(len(fieldPool))[:r.IntN(3)] {
			rec.Fields = append(rec.Fields, types.Field{Key: fieldPool[k] + fmt.Sprint(v), Opt: r.IntN(5) == 0, T: g.simpleType()})
		}
		if r.IntN(4) == 0 {
			rec.Rest = types.Any
		}
		vs = append(vs, rec)
	}
	t, ok := types.MakeUnion(vs)
	if !ok {
		panic("gen: a union with no tag")
	}
	return t
}

// simpleType is a type of one primitive, nullable at times, or a list of one.
func (g *Gen) simpleType() *types.Type {
	r := g.r
	t := types.Prim(pick(r, []string{"int", "float", "string", "bool"}))
	switch r.IntN(6) {
	case 0:
		t = t.Copy()
		t.Null = true
	case 1:
		t = types.ListOf(t)
	}
	return t
}

// namedTypes declares named types for a case: a union, a record, a list.
func (g *Gen) namedTypes(c *Case) {
	r := g.r
	u := g.unionType()
	item := &types.Type{Rec: &types.Record{Fields: []types.Field{{Key: "id", T: types.Prim("int")}, {Key: "name", T: types.Prim("string")}}}}
	if r.IntN(2) == 0 {
		item.Rec.Rest = types.Any
	}
	entries := []NamedType{
		{Name: "Shape", Type: u.String()},
		{Name: "Item", Type: item.String()},
		{Name: "Items", Type: "Item[]"},
	}
	g.named = map[string]*types.Type{"Shape": u, "Item": item, "Items": types.ListOf(item)}
	if g.loose && r.IntN(6) == 0 {
		// A declaration error: a name used before it is declared, a bad name,
		// two records with no tag.
		entries = append(entries, pick(r, []NamedType{
			{Name: "Early", Type: "Later"}, {Name: "x", Type: "int"},
			{Name: "Two", Type: "{a: int} | {b: string}"}, {Name: "Item", Type: "int"},
		}))
	}
	c.Types = entries[:1+r.IntN(len(entries))]
	for _, e := range []string{"Item", "Items"} {
		found := false
		for _, t := range c.Types {
			found = found || t.Name == e
		}
		if !found {
			delete(g.named, e)
		}
	}
	if g.named["Items"] != nil && g.named["Item"] == nil {
		delete(g.named, "Items")
	}
}

// unionVars adds a variable of a union type to the environment, with a
// value that conforms.
func (g *Gen) addUnionVar(c *Case) {
	r := g.r
	u := g.unionType()
	if r.IntN(4) == 0 {
		u = u.Copy()
		u.Null = true
	}
	name := pick(r, []string{"ev", "shape", "msg"})
	for _, v := range c.Vars {
		if v.Name == name {
			return
		}
	}
	val := g.valueFor(u, 3, r.IntN(3) == 0)
	text := u.String()
	if g.named["Shape"] != nil && !u.Null && r.IntN(3) == 0 {
		u, text = g.named["Shape"], "Shape"
		val = g.valueFor(u, 3, false)
	}
	c.Vars = append(c.Vars, Var{Name: name, Type: text, Value: val})
	g.unions = append(g.unions, unionVar{name: name, t: u})
	g.declared[name] = u
	g.bindName(name, tyAny)
}

// progType writes a program type for a subject of type s, known by its
// declared type or its value's type. A typed program asks for a type that
// is neither surely fitted by s nor disjoint from it, so that the checker
// reports no always-truthy, always-falsy or never-conforms.
func (g *Gen) progType(s *types.Type) string {
	r := g.r
	if g.loose && r.IntN(4) == 0 {
		return pick(r, []string{
			"int", "Foo", "{a: int} | {b: string}", `"a" | "b"`, "map<int>",
			"any", "never", "{x: int, ...}", "int[] | null", "Item", "(string | int)[]",
			`{kind: "a"} | {kind: "b", n: int}`, "{}", "string?",
		})
	}
	if g.named != nil && r.IntN(5) == 0 {
		for _, n := range []string{"Shape", "Item", "Items"} {
			if g.named[n] != nil && r.IntN(2) == 0 {
				return n
			}
		}
	}
	switch {
	case s == nil || s.Any:
		return pick(r, []string{"int", "float", "string", "bool", "null", "int[]", "string[]",
			"map<any>", "{id: int, ...}", `"a" | "b"`, "int | string", "any[]", "string | null",
			`{kind: "a", ...} | {kind: "b", ...}`})
	case len(s.Union) > 0:
		v := pick(r, s.Union)
		if r.IntN(2) == 0 {
			return (&types.Type{Rec: v}).String()
		}
		lit := v.Fields[0].T.Lits[0]
		for _, f := range v.Fields {
			if f.Key == s.Tag {
				lit = f.T.Lits[0]
			}
		}
		return fmt.Sprintf("{%s: %q, ...}", s.Tag, lit)
	case s.Null:
		if r.IntN(3) == 0 {
			return "null"
		}
		t := s.Copy()
		t.Null = false
		if t.IsNever() {
			return "null"
		}
		return t.String()
	}
	return s.String()
}

// subjectFor picks a variable to test with is, and its type: an any
// variable with its value's type, a nullable or union variable.
func (g *Gen) subjectFor() (string, *types.Type, *types.Type, bool) {
	var names []string
	for _, b := range g.visible() {
		if t, ok := g.declared[b.name]; ok && b.lam == nil {
			if t.Any || t.Null || len(t.Union) > 0 {
				names = append(names, b.name)
			}
		}
	}
	if len(names) == 0 {
		return "", nil, nil, false
	}
	n := pick(g.r, names)
	return n, g.declared[n], g.valueTy[n], true
}

// isExpr makes x is T, a bool.
func (g *Gen) isExpr(t *Ty, d int) ast.Expr {
	if g.opts.NoF4 {
		return nil
	}
	name, decl, _, ok := g.subjectFor()
	if !ok {
		if !g.loose {
			return nil
		}
		return &ast.As{Form: "is", X: g.expr(g.randTy(1), d), Type: g.progType(nil)}
	}
	return &ast.As{Form: "is", X: &ast.Ident{Name: name}, Type: g.progType(decl)}
}

// asExpr makes x as T. For an any variable whose value is known, T is the
// value's type, or wider, so that the run passes; at times, in a try body,
// a type the value does not conform to.
func (g *Gen) asExpr(t *Ty, d int) ast.Expr {
	if g.opts.NoF4 {
		return nil
	}
	r := g.r
	if g.sound {
		// The soundness mode has several value sets: a type made from the
		// declared type, which some values may not conform to. A failure is
		// then at the value site of the as (f04-conversion.md 6.6).
		name, decl, _, ok := g.subjectFor()
		if !ok || t.K != KAny {
			return nil
		}
		return &ast.As{Form: "as", X: &ast.Ident{Name: name}, Type: g.progType(decl)}
	}
	var cands []bind
	for _, b := range g.visible() {
		vt := g.valueTy[b.name]
		if decl, ok := g.declared[b.name]; ok && b.lam == nil && decl.Any && vt != nil && fits(b.t, t) && len(vt.String()) < 400 {
			cands = append(cands, b)
		}
	}
	if len(cands) == 0 {
		if g.loose {
			return &ast.As{Form: "as", X: g.expr(g.randTy(1), d), Type: g.progType(nil)}
		}
		return nil
	}
	b := pick(r, cands)
	vt := g.valueTy[b.name]
	text := vt.String()
	if r.IntN(3) == 0 {
		text = g.widen(vt).String()
	}
	if g.tryBody > 0 && r.IntN(3) == 0 {
		text = g.wrong(vt).String()
	}
	return &ast.As{Form: "as", X: &ast.Ident{Name: b.name}, Type: text}
}

// narrowed makes v is T ? e : d, where e uses v as T.
func (g *Gen) narrowed(t *Ty, d int) ast.Expr {
	if g.opts.NoF4 {
		return nil
	}
	r := g.r
	var names []string
	for _, b := range g.visible() {
		if decl, ok := g.declared[b.name]; ok && b.lam == nil && decl.Any {
			names = append(names, b.name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	name := pick(r, names)
	kinds := []struct {
		text string
		ty   *Ty
	}{{"int", tyInt}, {"float", tyNum}, {"string", tyStr}, {"bool", tyBool}, {"int[]", listOf(tyInt, -1)}, {"string[]", listOf(tyStr, -1)}}
	k := pick(r, kinds)
	test := ast.Expr(&ast.As{Form: "is", X: &ast.Ident{Name: name}, Type: k.text})
	if r.IntN(4) == 0 {
		// f04-conversion.md 6.4 item 4: || joins two facts of is.
		other := pick(r, kinds)
		if other.ty.K == k.ty.K || other.ty.isNum() && k.ty.isNum() {
			test = &ast.Binary{Op: "||", X: test, Y: &ast.As{Form: "is", X: &ast.Ident{Name: name}, Type: other.text}}
			k.ty = tyAny // the then branch uses only what both share
		}
	}
	g.push()
	g.bindName(name, k.ty)
	then := g.expr(t, d)
	g.pop()
	return &ast.Cond{Test: test, Then: then, Else: g.expr(t, d)}
}

// tagTest narrows a union variable by a test of its tag and reads a field
// of the variant (f05-unions.md 5.2, 5.3).
func (g *Gen) tagTest(t *Ty, d int) ast.Expr {
	if g.opts.NoF4 || len(g.unions) == 0 {
		return nil
	}
	r := g.r
	u := pick(r, g.unions)
	if b := g.lookup(u.name); b == nil || b.lam != nil || b.t.K != KAny {
		return nil // shadowed
	}
	v := pick(r, u.t.Union)
	var tagLit string
	var field *types.Field
	for i := range v.Fields {
		f := &v.Fields[i]
		switch {
		case f.Key == u.t.Tag:
			tagLit = f.T.Lits[0]
		case !f.Opt && field == nil && fits(tyForTypeShallow(f.T), t):
			field = f
		}
	}
	if field == nil && !g.loose {
		return nil
	}
	recv := ast.Expr(&ast.Ident{Name: u.name})
	q := &ast.Member{X: recv, Name: u.t.Tag, Opt: u.t.Null}
	var read ast.Expr
	if field != nil {
		read = &ast.Member{X: &ast.Ident{Name: u.name}, Name: field.Key}
	} else {
		read = &ast.Member{X: &ast.Ident{Name: u.name}, Name: "zz"}
	}
	other := g.expr(t, d)
	lit := &ast.Str{Val: tagLit, Quote: '"'}
	switch r.IntN(5) {
	case 0:
		return &ast.Cond{Test: &ast.Binary{Op: "==", X: q, Y: lit}, Then: read, Else: other}
	case 1:
		return &ast.Cond{Test: &ast.Binary{Op: "!=", X: q, Y: lit}, Then: other, Else: read}
	case 2:
		return &ast.Cond{Test: &ast.Binary{Op: "in", X: q, Y: &ast.List{Items: []ast.Item{{X: lit}}}}, Then: read, Else: other}
	case 3:
		return &ast.Cond{Test: &ast.Binary{Op: "not in", X: q, Y: &ast.List{Items: []ast.Item{{X: lit}}}}, Then: other, Else: read}
	default:
		return &ast.Match{Subject: q, Arms: []ast.Arm{{Lits: []ast.Expr{lit}, Result: read}, {Default: true, Result: other}}}
	}
}

// tyForTypeShallow gives the rough type of a field's type, for a field read
// after the variant is known.
func tyForTypeShallow(t *types.Type) *Ty {
	if t.Null || t.Any {
		return tyAny
	}
	switch {
	case t.Num == 1:
		return tyInt
	case t.Num == 2:
		return tyNum
	case t.Str || len(t.Lits) > 0:
		return tyStr
	case t.Bool:
		return tyBool
	case t.List != nil:
		return listOf(tyForTypeShallow(t.List), -1)
	}
	return tyAny
}

// unionLiteral writes a map literal that fits one variant of u: the tag
// first, then each required field (f05-unions.md 6).
func (g *Gen) unionLiteral(u *types.Type, d int) *ast.Map {
	v := pick(g.r, u.Union)
	m := &ast.Map{}
	for _, f := range v.Fields {
		if f.Opt && g.r.IntN(2) == 0 {
			continue
		}
		var val ast.Expr
		if f.Key == u.Tag {
			val = &ast.Str{Val: f.T.Lits[0], Quote: '"'}
		} else {
			val = g.valueExpr(f.T, d)
		}
		m.Entries = append(m.Entries, ast.Entry{Kind: ast.EntryWord, Key: f.Key, X: val})
	}
	return m
}

// valueExpr makes an expression of a declared type: a literal of a value
// that conforms.
func (g *Gen) valueExpr(t *types.Type, d int) ast.Expr {
	return literalOf(g.valueFor(t, 2, false))
}

// literalOf writes a host value as a literal of the language.
func literalOf(v any) ast.Expr {
	switch v := v.(type) {
	case nil:
		return &ast.Null{}
	case bool:
		return &ast.Bool{V: v}
	case float64:
		text := formatNumber(v)
		if !strings.ContainsAny(text, ".e") && !types.IsWhole(v) {
			text += ".0" // a whole number past 2^53 - 1 is literal-range without a point (01-language.md 2.5)
		}
		if strings.HasPrefix(text, "-") {
			return &ast.Unary{Op: "-", X: &ast.Num{Text: strings.TrimPrefix(text, "-")}}
		}
		return &ast.Num{Text: text}
	case string:
		return &ast.Str{Val: v, Quote: '"'}
	case []any:
		l := &ast.List{}
		for _, x := range v {
			l.Items = append(l.Items, ast.Item{X: literalOf(x)})
		}
		return l
	case map[string]any:
		m := &ast.Map{}
		for _, k := range sortedKeys(v) {
			kind := ast.EntryString
			if isWord(k) {
				kind = ast.EntryWord
			}
			m.Entries = append(m.Entries, ast.Entry{Kind: kind, Key: k, X: literalOf(v[k])})
		}
		return m
	}
	return &ast.Null{}
}

// convForm makes a form of F4 or F5 of type t, or nil.
func (g *Gen) convForm(t *Ty, d int) ast.Expr {
	r := g.r
	switch r.IntN(6) {
	case 0:
		if t.K == KBool || t.K == KAny {
			return g.isExpr(t, d)
		}
	case 1:
		return g.asExpr(t, d)
	case 2:
		return g.narrowed(t, d)
	case 3, 4:
		return g.tagTest(t, d)
	default:
		if (t.K == KMap || t.K == KAny) && g.named["Shape"] != nil {
			return &ast.As{Form: "as", X: g.unionLiteral(g.named["Shape"], d), Type: "Shape"}
		}
	}
	return nil
}
