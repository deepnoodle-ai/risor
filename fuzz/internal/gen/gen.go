package gen

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode/utf8"

	"github.com/deepnoodle-ai/risor/fuzz/internal/ast"
	"github.com/deepnoodle-ai/risor/fuzz/internal/types"
)

// Options steer the generator.
type Options struct {
	// Weights of the modes typed, loose and mangled. All zero means 5, 3, 2.
	Typed, Loose, Mangled int
	// LimitShare is the share of cases, in percent, that get a small limit.
	// A negative value means none.
	LimitShare int
	// NoCheckShare is the share of cases, in percent, with "check": false.
	NoCheckShare int
	// NoRemoved leaves out the case field removed.
	NoRemoved bool
	// NoF4 leaves out the forms of packages F4 and F5.
	NoF4 bool
	// NoF3 leaves out the string forms of package F3.
	NoF3 bool
	// Exclude names library entries not to call, by name.
	Exclude map[string]bool
}

type lamSig struct {
	params []*Ty
	req    int
	result *Ty
}

type bind struct {
	name string
	t    *Ty
	lam  *lamSig
}

type hostFn struct {
	ns, name string
	params   []*Ty
	req      int
	rest     bool
	result   *Ty
}

type opqMethod struct {
	name   string
	params []*Ty
	result *Ty
}

// Gen makes one case. It is not safe for use by more than one goroutine.
type Gen struct {
	r       *rand.Rand
	lib     *Library
	opts    Options
	loose   bool
	frames  [][]bind
	n       int
	size    int // nodes made so far, to bound the size of a program
	opaque  string
	methods []opqMethod
	hosts   []hostFn
	ns      string
	tryBody int
	sound   bool // the soundness mode: use the variables more, divide by them

	// Packages F4 and F5.
	declared map[string]*types.Type // the declared type of each variable
	valueTy  map[string]*types.Type // the type of each variable's value
	unions   []unionVar
	named    map[string]*types.Type // the named types of the case
	funcs   []*LibEntry
	meths   []*LibEntry
}

// Seed gives the generator's random source for one case of one round.
func Seed(seed uint64, round, index int) *rand.Rand {
	return rand.New(rand.NewPCG(seed, uint64(round)<<24|uint64(index)))
}

// Generate makes the case with the given index of the given round.
func Generate(lib *Library, opts Options, seed uint64, round, index int) *Case {
	r := Seed(seed, round, index)
	g := &Gen{r: r, lib: lib, opts: opts}
	for _, e := range lib.Functions {
		if !opts.Exclude[e.Name] {
			g.funcs = append(g.funcs, e)
		}
	}
	for _, e := range lib.Methods {
		if !opts.Exclude[e.Name] {
			g.meths = append(g.meths, e)
		}
	}
	wt, wl, wm := opts.Typed, opts.Loose, opts.Mangled
	if wt+wl+wm == 0 {
		wt, wl, wm = 5, 3, 2
	}
	mode := Typed
	switch x := r.IntN(wt + wl + wm); {
	case x < wt:
		mode = Typed
	case x < wt+wl:
		mode = Loose
	default:
		mode = Mangled
	}
	c := &Case{Name: fmt.Sprintf("r%d-c%d", round, index), Mode: mode}
	g.loose = mode == Loose || mode == Mangled && r.IntN(2) == 0
	g.declared, g.valueTy = map[string]*types.Type{}, map[string]*types.Type{}
	g.env(c)
	c.Prog = g.program()
	if !opts.NoF4 && mode == Typed && g.named["Shape"] != nil && r.IntN(6) == 0 {
		// A map literal against a union, by the expected result
		// (f05-unions.md 6).
		c.Returns = "Shape"
		c.Prog.Result = g.unionLiteral(g.named["Shape"], 2)
	}
	if mode == Mangled {
		c.Source = g.mangle(ast.Print(c.Prog))
		c.Prog = nil
	}
	c.NoCheck = r.IntN(100) < opts.NoCheckShare
	share := opts.LimitShare
	if share == 0 {
		share = 25
	}
	if r.IntN(100) < share {
		c.Limits = g.limits()
	}
	return c
}

func (g *Gen) chance(p float64) bool { return g.r.Float64() < p }

func (g *Gen) limits() *Limits {
	l := &Limits{}
	v := func(n int) *int { x := n; return &x }
	// At most one of the step limit and the unit limit is small, so that a
	// finding points at one count.
	switch g.r.IntN(4) {
	case 0:
		l.Steps = v(g.r.IntN(60))
	case 1:
		l.Allocation = v(g.r.IntN(80))
	case 2:
		l.Depth = v(1 + g.r.IntN(6))
	default:
		l.Depth = v(1 + g.r.IntN(4))
		if g.r.IntN(2) == 0 {
			l.Steps = v(g.r.IntN(80))
		} else {
			l.Allocation = v(g.r.IntN(100))
		}
	}
	return l
}

// env makes the variables, opaque type and host functions of a case.
func (g *Gen) env(c *Case) {
	r := g.r
	if r.IntN(6) == 0 {
		g.opaque = pick(r, []string{"Tok", "Clock"})
		c.Opaque = []string{g.opaque}
		if g.loose && r.IntN(10) == 0 {
			c.Opaque = append(c.Opaque, "Other")
		}
		g.opaqueStubs(c)
	}
	if r.IntN(4) == 0 {
		g.hostStubs(c)
	}
	if !g.opts.NoRemoved && r.IntN(12) == 0 {
		g.removeEntries(c)
	}
	if !g.opts.NoF4 && r.IntN(4) == 0 {
		g.namedTypes(c)
	}
	g.frames = [][]bind{nil}
	nv := r.IntN(5)
	used := map[string]bool{}
	for range nv {
		name := pick(r, varNames)
		if used[name] {
			continue
		}
		used[name] = true
		var val any
		switch {
		case r.IntN(30) == 0:
			val = deepValue(150 + r.IntN(110))
		case g.opaque != "" && r.IntN(3) == 0:
			val = g.opaqueValue()
		default:
			val = g.hostValue(2)
		}
		decl, exact := g.declType(c, val)
		v := Var{Name: name, Value: val, Type: decl}
		if dt, err := types.Parse(decl); err == nil {
			g.declared[name] = dt
		}
		g.valueTy[name] = types.OfValue(val)
		if g.loose && r.IntN(15) == 0 {
			v.Missing = true
		}
		c.Vars = append(c.Vars, v)
		t := tyOfValue(val)
		if v.Missing || !exact {
			t = tyAny
		}
		g.bindName(name, t)
	}
	if !g.opts.NoF4 && r.IntN(3) == 0 {
		g.addUnionVar(c)
	}
	if g.loose && r.IntN(12) == 0 {
		// A declaration error: a bad name, a bad type text, a clash.
		v := Var{Missing: true, Type: "int"}
		switch r.IntN(4) {
		case 0:
			v.Name = pick(r, []string{"if", "_", "a-b", "nil", "match"})
		case 1:
			v.Name = "q"
			v.Type = pick(r, []string{"int[", "{a?: int}", "Foo", "map<int", "integer", "(int)", "int | ", "{a: int, a: int}", "string[][]", "{...}", "'lit'"})
		case 2:
			v.Name = pick(r, []string{"len", "score", "geo", "type"})
		default:
			v.Name = "q"
			v.Type = "{x: " + strings.Repeat("{x: ", 64) + "int" + strings.Repeat("}", 65)
		}
		c.Vars = append(c.Vars, v)
	}
}

func (g *Gen) opaqueStubs(c *Case) {
	o := g.opaque
	opq := func(id int) any {
		return map[string]any{"$opaque": map[string]any{"type": o, "id": float64(id)}}
	}
	ot := &Ty{K: KOpq, Opq: o, Len: -1}
	c.Methods = []Stub{
		{Name: o + ".label", Type: "() => string", Rows: []Row{
			{HasArgs: true, Args: []any{opq(1)}, Result: "one", Charge: g.r.IntN(3)},
			{HasArgs: true, Args: []any{opq(2)}, Result: "😀two"},
			{Result: "other", Allocate: g.r.IntN(3)},
		}},
		{Name: o + ".next", Type: "(n: int) => " + o, Rows: []Row{
			{HasArgs: true, Args: []any{opq(1), 1.0}, Result: opq(2)},
			{Result: opq(3)},
		}},
		{Name: o + ".size", Type: "() => int", Rows: []Row{{Result: 3.0, Charge: 1}}},
		{Name: o + ".broken", Type: "() => string", Rows: []Row{{IsError: true, Error: "the clock is broken"}}},
	}
	g.methods = []opqMethod{
		{name: "label", result: tyStr},
		{name: "next", params: []*Ty{tyInt}, result: ot},
		{name: "size", result: tyInt},
		{name: "broken", result: tyStr},
	}
}

func (g *Gen) hostStubs(c *Case) {
	g.ns = "geo"
	c.Functions = []Stub{
		{Name: "lookup", Type: "(key: string) => any", Rows: []Row{
			{HasArgs: true, Args: []any{"a"}, Result: map[string]any{"n": 1.0, "s": "x"}, Charge: 2},
			{HasArgs: true, Args: []any{"b"}, Result: []any{1.0, 2.0, 3.0}, Allocate: 4},
			{Result: nil},
		}},
		{Name: "score", Type: "(x: any, y?: int) => int", Rows: []Row{
			{HasArgs: true, Args: []any{1.0}, Result: 10.0},
			{Result: 7.0, Charge: 1},
		}},
		{Name: "geo.dist", Type: "(...xs: float[]) => float", Rows: []Row{
			{HasArgs: true, Args: []any{}, Result: 0.0},
			{Result: 2.5},
		}},
		{Name: "oops", Type: "() => string", Rows: []Row{{IsError: true, Error: "oops: no luck"}}},
	}
	g.hosts = []hostFn{
		{name: "lookup", params: []*Ty{tyStr}, req: 1, result: tyAny},
		{name: "score", params: []*Ty{tyAny, tyInt}, req: 1, result: tyInt},
		{ns: "geo", name: "dist", params: []*Ty{tyNum}, rest: true, result: tyNum},
		{name: "oops", result: tyStr},
	}
}

// Scope.

func (g *Gen) push() { g.frames = append(g.frames, nil) }
func (g *Gen) pop()  { g.frames = g.frames[:len(g.frames)-1] }
func (g *Gen) bindName(n string, t *Ty) {
	f := len(g.frames) - 1
	g.frames[f] = append(g.frames[f], bind{name: n, t: t})
}
func (g *Gen) bindLam(n string, s *lamSig) {
	f := len(g.frames) - 1
	g.frames[f] = append(g.frames[f], bind{name: n, lam: s})
}

// visible lists the innermost binding of each name.
func (g *Gen) visible() []bind {
	seen := map[string]bool{}
	var out []bind
	for i := len(g.frames) - 1; i >= 0; i-- {
		for j := len(g.frames[i]) - 1; j >= 0; j-- {
			b := g.frames[i][j]
			if !seen[b.name] {
				seen[b.name] = true
				out = append(out, b)
			}
		}
	}
	return out
}

func (g *Gen) fresh(prefix string) string {
	g.n++
	return fmt.Sprintf("%s%d", prefix, g.n)
}

// randTy picks a rough type. Typed programs never pick an opaque type here.
func (g *Gen) randTy(depth int) *Ty {
	n := g.r.IntN(10)
	if depth <= 0 && n >= 7 {
		n = g.r.IntN(7)
	}
	switch n {
	case 0, 1:
		return tyInt
	case 2:
		return tyNum
	case 3, 4:
		return tyStr
	case 5:
		return tyBool
	case 6:
		if g.loose && g.opaque != "" && g.r.IntN(3) == 0 {
			return &Ty{K: KOpq, Opq: g.opaque, Len: -1}
		}
		if g.r.IntN(2) == 0 {
			return tyNull
		}
		return tyAny
	case 7, 8:
		return listOf(g.randTy(depth-1), -1)
	default:
		k := 1 + g.r.IntN(3)
		var fs []TField
		seen := map[string]bool{}
		for range k {
			name := pick(g.r, fieldNames[:10])
			if seen[name] {
				continue
			}
			seen[name] = true
			fs = append(fs, TField{Name: name, T: g.randTy(depth - 1)})
		}
		return recordOf(fs)
	}
}

// program makes the tree of a program.
func (g *Gen) program() *ast.Program {
	p := &ast.Program{}
	if g.sound {
		// Smaller programs, so that a site and its failure stand out.
		p.Consts = g.consts(g.r.IntN(3), 2)
		p.Result = g.expr(g.randTy(2), 3)
		return p
	}
	p.Consts = g.consts(g.r.IntN(5), 3)
	p.Result = g.expr(g.randTy(2), 4)
	p.Semi = g.r.IntN(5) == 0
	return p
}

// consts makes n const lines in the current frame.
func (g *Gen) consts(n, d int) []*ast.Const {
	var out []*ast.Const
	for range n {
		out = append(out, g.constLine(d))
	}
	return out
}

func (g *Gen) constLine(d int) *ast.Const {
	r := g.r
	switch x := r.IntN(20); {
	case x < 5:
		return g.lambdaConst(d)
	case x < 8:
		return g.mapPatConst(d)
	case x < 10:
		return g.listPatConst(d)
	default:
		t := g.randTy(2)
		v := g.expr(t, d)
		name := g.fresh("c")
		if g.loose && r.IntN(15) == 0 {
			if vs := g.visible(); len(vs) > 0 {
				name = pick(r, vs).name // duplicate-name or a shadow
			}
		}
		if g.loose && r.IntN(40) == 0 {
			name = pick(r, reservedList)
		}
		if g.loose && g.ns != "" && r.IntN(30) == 0 {
			name = g.ns
		}
		g.bindName(name, t)
		return &ast.Const{Pat: &ast.NamePat{Name: name}, Value: v}
	}
}

func (g *Gen) lambdaConst(d int) *ast.Const {
	r := g.r
	n := r.IntN(4)
	sig := &lamSig{result: g.randTy(1)}
	lam := &ast.Lambda{}
	g.push()
	for i := range n {
		pt := g.randTy(1)
		sig.params = append(sig.params, pt)
		name := g.fresh("p")
		p := ast.Param{Pat: &ast.NamePat{Name: name}}
		if i >= n-1 && r.IntN(3) == 0 || r.IntN(8) == 0 {
			p.Default = g.expr(pt, 1)
		}
		lam.Params = append(lam.Params, p)
		g.bindName(name, pt)
	}
	for i, p := range lam.Params {
		if p.Default == nil {
			sig.req = i + 1
		}
	}
	lam.Body = g.expr(sig.result, d)
	g.pop()
	if n == 1 && lam.Params[0].Default == nil && r.IntN(2) == 0 {
		lam.Bare = lam.Params[0].Pat.(*ast.NamePat).Name
	}
	name := g.fresh("f")
	g.bindLam(name, sig)
	return &ast.Const{Pat: &ast.NamePat{Name: name}, Value: lam}
}

func (g *Gen) mapPatConst(d int) *ast.Const {
	r := g.r
	t := g.randTy(2)
	if t.K != KMap || t.Fields == nil {
		t = recordOf([]TField{{Name: "a", T: tyInt}, {Name: "b", T: tyStr}})
	}
	v := g.expr(t, d)
	pat := &ast.MapPat{}
	var binds []bind
	for _, f := range t.Fields {
		if r.IntN(4) == 0 {
			continue
		}
		fl := ast.Field{Name: g.fresh("k")}
		if isName(f.Name) && r.IntN(3) == 0 {
			fl.Name = f.Name
			if g.lookup(f.Name) != nil {
				fl.Name = g.fresh("k")
				fl.Key = f.Name
			}
		} else {
			fl.Key = f.Name
			fl.KeyStr = !isWord(f.Name) || r.IntN(4) == 0
			fl.Raw = fl.KeyStr && !g.opts.NoF3 && r.IntN(4) == 0
		}
		if fl.Key == "" && fl.Name != f.Name {
			fl.Key = f.Name
		}
		if r.IntN(4) == 0 {
			fl.Default = g.expr(f.T, 1)
		}
		pat.Fields = append(pat.Fields, fl)
		binds = append(binds, bind{name: fl.Name, t: f.T})
	}
	if g.loose && r.IntN(3) == 0 {
		// A key that is absent, with a default; or, when loose, none.
		fl := ast.Field{Key: "zz", Name: g.fresh("k")}
		if !g.loose || r.IntN(2) == 0 {
			fl.Default = g.expr(tyInt, 1)
		}
		pat.Fields = append(pat.Fields, fl)
		binds = append(binds, bind{name: fl.Name, t: tyInt})
	}
	if r.IntN(3) == 0 {
		pat.Rest = g.fresh("r")
		binds = append(binds, bind{name: pat.Rest, t: mapAny()})
	}
	for _, b := range binds {
		g.bindName(b.name, b.t)
	}
	return &ast.Const{Pat: pat, Value: v}
}

func (g *Gen) lookup(name string) *bind {
	for _, b := range g.visible() {
		if b.name == name {
			return &b
		}
	}
	return nil
}

func (g *Gen) listPatConst(d int) *ast.Const {
	r := g.r
	et := g.randTy(1)
	n := r.IntN(4)
	lit := &ast.List{}
	for range n {
		lit.Items = append(lit.Items, ast.Item{X: g.expr(et, d-1)})
	}
	pat := &ast.ListPat{}
	k := r.IntN(n + 2)
	var binds []bind
	for i := range k {
		el := ast.Elem{Name: "_"}
		if r.IntN(4) > 0 {
			el.Name = g.fresh("e")
			binds = append(binds, bind{name: el.Name, t: et})
		}
		if i >= n && (!g.loose || r.IntN(2) == 0) || r.IntN(6) == 0 {
			el.Default = g.expr(et, 1)
		}
		pat.Elems = append(pat.Elems, el)
	}
	if r.IntN(3) == 0 {
		pat.Rest = g.fresh("r")
		binds = append(binds, bind{name: pat.Rest, t: listOf(et, -1)})
	}
	for _, b := range binds {
		g.bindName(b.name, b.t)
	}
	return &ast.Const{Pat: pat, Value: lit}
}

// Expressions.

type form func(t *Ty, d int) ast.Expr

// expr makes an expression of type t with depth budget d.
func (g *Gen) expr(t *Ty, d int) ast.Expr {
	g.size++
	if g.loose && g.chance(0.3) {
		t = g.randTy(1)
	}
	if d <= 0 || g.size > 400 {
		return g.leaf(t)
	}
	forms := g.forms(t)
	total := 0
	for _, f := range forms {
		total += f.w
	}
	for range 6 {
		x := g.r.IntN(total)
		for _, f := range forms {
			if x < f.w {
				if e := f.f(t, d-1); e != nil {
					return e
				}
				break
			}
			x -= f.w
		}
	}
	return g.leaf(t)
}

type wform struct {
	w int
	f form
}

func (g *Gen) forms(t *Ty) []wform {
	fs := []wform{
		{4, func(t *Ty, d int) ast.Expr { return g.leaf(t) }},
		{3, g.varRead},
		{2, g.cond},
		{1, g.ifExpr},
		{1, g.matchExpr},
		{1, g.block},
		{2, g.tryExpr},
		{2, g.lambdaCall},
		{1, g.coal},
		{1, g.logic},
		{6, g.libCall},
		{2, g.member},
		{1, g.index},
		{1, g.paren},
	}
	if len(g.hosts) > 0 {
		fs = append(fs, wform{2, g.hostCall})
	}
	if g.sound {
		// Sites inside a try body are dropped (18.3): fewer of them, and
		// more reads of the variables.
		fs[6].w = 0
		fs[1].w = 6
	}
	if len(g.methods) > 0 {
		fs = append(fs, wform{2, g.opqCall})
	}
	switch t.K {
	case KNum, KInt:
		fs = append(fs, wform{5, g.arith}, wform{1, g.neg})
	case KStr:
		fs = append(fs, wform{2, g.concat}, wform{3, g.template})
	case KBool:
		fs = append(fs, wform{3, g.compare}, wform{2, g.equal}, wform{2, g.inExpr}, wform{1, g.not})
	case KList:
		fs = append(fs, wform{3, g.listLit})
	case KMap:
		fs = append(fs, wform{3, g.mapLit})
	case KAny:
		fs = append(fs, wform{3, g.arith}, wform{2, g.template}, wform{2, g.listLit}, wform{2, g.mapLit}, wform{2, g.compare})
	}
	if g.loose {
		fs = append(fs, wform{1, g.looseOddity})
	}
	if !g.opts.NoF4 {
		fs = append(fs, wform{6, g.convForm})
	}
	return fs
}

func (g *Gen) num(text string) ast.Expr { return &ast.Num{Text: text} }

func (g *Gen) smallInt(lo, hi int) ast.Expr {
	n := lo + g.r.IntN(hi-lo+1)
	if n < 0 {
		return &ast.Unary{Op: "-", X: g.num(fmt.Sprint(-n))}
	}
	return g.num(fmt.Sprint(n))
}

func (g *Gen) str(s string) ast.Expr {
	q := byte('"')
	switch g.r.IntN(6) {
	case 0:
		q = '\''
	case 1:
		q = '`'
	}
	esc := 0
	if g.r.IntN(5) == 0 {
		esc = 1 + g.r.IntN(2)
	}
	e := &ast.Str{Val: s, Quote: q, Esc: esc}
	if !g.opts.NoF3 {
		// Package F3: a raw string, a text block, a raw text block. The
		// printer falls back to a one-line string when the form cannot hold
		// the value.
		switch x := g.r.IntN(100); {
		case x < 12:
			e.Raw, e.Quote = true, pick(g.r, []byte{'"', '\''})
		case x < 20:
			e.Block, e.Indent = 1, g.indent()
		case x < 24:
			e.Block, e.Indent = 2, g.indent()
		}
	}
	return e
}

func (g *Gen) indent() string {
	return pick(g.r, []string{"", "  ", "    ", "\t", " \t"})
}

// leaf makes a literal or a variable of type t.
func (g *Gen) leaf(t *Ty) ast.Expr {
	r := g.r
	if r.IntN(3) == 0 || g.sound && r.IntN(3) > 0 {
		if e := g.varRead(t, 0); e != nil {
			return e
		}
	}
	switch t.K {
	case KInt:
		if r.IntN(8) == 0 {
			return &ast.Unary{Op: "-", X: g.num(pick(r, []string{"0", "1", "3"}))}
		}
		for {
			l := pick(r, numLits)
			if l.k == KInt {
				return g.num(l.text)
			}
		}
	case KNum:
		return g.num(pick(r, numLits).text)
	case KStr:
		return g.str(pick(r, strValues))
	case KBool:
		return &ast.Bool{V: r.IntN(2) == 0}
	case KNull:
		return &ast.Null{}
	case KList:
		et := t.Elem
		if et == nil || et.K == KAny {
			et = g.randTy(0)
		}
		n := t.Len
		if n < 0 {
			n = r.IntN(4)
		}
		l := &ast.List{}
		for range n {
			l.Items = append(l.Items, ast.Item{X: g.leaf(et)})
		}
		return l
	case KMap:
		m := &ast.Map{}
		fs := t.Fields
		if fs == nil {
			fs = g.randTy(3).Fields
		}
		for _, f := range fs {
			m.Entries = append(m.Entries, g.entry(f.Name, g.leaf(f.T)))
		}
		return m
	case KOpq:
		if e := g.varRead(t, 0); e != nil {
			return e
		}
		return &ast.Null{}
	}
	// KAny
	return g.leaf(g.randTy(1))
}

// entry writes a map entry with the key k in one of its forms.
func (g *Gen) entry(k string, v ast.Expr) ast.Entry {
	r := g.r
	x := r.IntN(10)
	if !g.loose && x >= 8 {
		// A computed key gives the map the type map<T>, and a read of it
		// would be maybe-absent.
		x = r.IntN(8)
	}
	switch {
	case x < 5 && isWord(k):
		return ast.Entry{Kind: ast.EntryWord, Key: k, X: v}
	case x < 8:
		return ast.Entry{Kind: ast.EntryString, Key: k, X: v, Raw: !g.opts.NoF3 && r.IntN(4) == 0}
	default:
		var kx ast.Expr = g.str(k)
		if r.IntN(2) == 0 {
			kx = &ast.Binary{Op: "+", X: g.str(""), Y: g.str(k)}
		}
		return ast.Entry{Kind: ast.EntryComputed, KeyX: kx, X: v}
	}
}

func (g *Gen) varRead(t *Ty, d int) ast.Expr {
	r := g.r
	if g.loose {
		switch r.IntN(30) {
		case 0:
			return &ast.Ident{Name: pick(r, []string{"zz", "undefinedName", "len", "map"})}
		case 1:
			if g.ns != "" {
				return &ast.Ident{Name: g.ns}
			}
		case 2:
			for _, b := range g.visible() {
				if b.lam != nil {
					return &ast.Ident{Name: b.name} // a lambda used as a value
				}
			}
		}
	}
	var cands []bind
	for _, b := range g.visible() {
		if b.lam == nil && (fits(b.t, t) || g.loose && r.IntN(3) == 0) {
			cands = append(cands, b)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	return &ast.Ident{Name: pick(r, cands).name}
}

func (g *Gen) cond(t *Ty, d int) ast.Expr {
	return &ast.Cond{Test: g.test(d), Then: g.expr(t, d), Else: g.expr(t, d)}
}

func (g *Gen) branch(t *Ty, d int, nconst int) *ast.Branch {
	g.push()
	defer g.pop()
	b := &ast.Branch{Consts: g.consts(nconst, d-1)}
	b.Result = g.expr(t, d)
	return b
}

func (g *Gen) ifExpr(t *Ty, d int) ast.Expr {
	n := 1 + g.r.IntN(3)
	e := &ast.If{}
	for range n {
		e.Tests = append(e.Tests, g.test(d))
		e.Then = append(e.Then, g.branch(t, d, g.r.IntN(2)*g.r.IntN(2)))
	}
	e.Else = g.branch(t, d, g.r.IntN(2)*g.r.IntN(2))
	return e
}

func (g *Gen) block(t *Ty, d int) ast.Expr {
	g.push()
	defer g.pop()
	b := &ast.Block{Consts: g.consts(1+g.r.IntN(3), d-1)}
	b.Result = g.expr(t, d)
	return b
}

func (g *Gen) matchExpr(t *Ty, d int) ast.Expr {
	r := g.r
	m := &ast.Match{}
	var lits []ast.Expr
	x := r.IntN(5)
	if x == 4 && !g.loose {
		x = 0
	}
	switch x {
	case 0, 1:
		m.Subject = g.expr(tyInt, d)
		for _, n := range r.Perm(8)[:1+r.IntN(4)] {
			lits = append(lits, g.matchNum(n-2))
		}
	case 2:
		m.Subject = g.expr(tyStr, d)
		if !g.loose {
			// A literal subject has a literal set as its type, and every
			// other literal would be never-equal.
			m.Subject = &ast.Call{Name: "string", Args: []ast.Expr{m.Subject}}
		}
		for _, i := range r.Perm(len(strValues))[:1+r.IntN(4)] {
			lit := &ast.Str{Val: strValues[i], Quote: pick(r, []byte{'"', '\''})}
			if !g.opts.NoF3 && r.IntN(4) == 0 {
				lit.Raw = true // a raw string is a literal of a match arm
			}
			if g.loose && !g.opts.NoF3 && r.IntN(15) == 0 {
				lit.Block, lit.Indent = 1, g.indent() // a text block is not
			}
			lits = append(lits, lit)
		}
	case 3:
		if !g.loose {
			m.Subject = g.expr(tyBool, d)
			lits = []ast.Expr{&ast.Bool{V: true}, &ast.Bool{V: false}}[:1+r.IntN(2)]
			break
		}
		fallthrough
	default:
		m.Subject = g.expr(g.randTy(0), d)
		lits = []ast.Expr{&ast.Bool{V: true}, &ast.Null{}, &ast.Bool{V: false}, g.matchNum(0), &ast.Str{Val: "", Quote: '"'}}
		r.Shuffle(len(lits), func(i, j int) { lits[i], lits[j] = lits[j], lits[i] })
		lits = lits[:1+r.IntN(len(lits))]
	}
	if g.loose && r.IntN(6) == 0 && len(lits) > 0 {
		lits = append(lits, lits[0]) // duplicate-case
	}
	for len(lits) > 0 {
		k := 1 + r.IntN(min(3, len(lits)))
		m.Arms = append(m.Arms, ast.Arm{Lits: lits[:k], Result: g.expr(t, d)})
		lits = lits[k:]
	}
	if !g.loose || r.IntN(3) > 0 {
		m.Arms = append(m.Arms, ast.Arm{Default: true, Result: g.expr(t, d)})
		if g.loose && r.IntN(8) == 0 {
			m.Arms = append(m.Arms, ast.Arm{Lits: []ast.Expr{g.matchNum(99)}, Result: g.expr(t, 0)})
		}
	}
	return m
}

func (g *Gen) matchNum(n int) ast.Expr {
	if g.r.IntN(10) == 0 {
		return g.num(fmt.Sprintf("%d.5", n+10))
	}
	if n < 0 {
		return &ast.Unary{Op: "-", X: g.num(fmt.Sprint(-n))}
	}
	return g.num(fmt.Sprint(n))
}

var errTy = recordOf([]TField{{Name: "code", T: tyStr}, {Name: "message", T: tyStr}, {Name: "path", T: tyNull}, {Name: "data", T: tyAny}})

func (g *Gen) tryExpr(t *Ty, d int) ast.Expr {
	r := g.r
	e := &ast.Try{}
	g.tryBody++
	if r.IntN(2) == 0 {
		e.Body = &ast.Branch{Result: g.risky(t, d)}
	} else {
		e.Body = g.branch(t, d, r.IntN(2)*r.IntN(2))
	}
	g.tryBody--
	g.push()
	if r.IntN(3) > 0 {
		e.Name = g.fresh("err")
		g.bindName(e.Name, errTy)
	}
	e.Handler = &ast.Branch{Result: g.expr(t, d)}
	if e.Name != "" && r.IntN(2) == 0 && (t.K == KStr || t.K == KAny) {
		e.Handler.Result = &ast.Member{X: &ast.Ident{Name: e.Name}, Name: pick(r, []string{"code", "message"})}
	}
	g.pop()
	return e
}

// risky makes an expression that is likely to fail with a code catch catches.
func (g *Gen) risky(t *Ty, d int) ast.Expr {
	r := g.r
	if !g.opts.NoF4 && r.IntN(5) == 0 {
		if e := g.asExpr(t, d); e != nil {
			return e
		}
	}
	x := r.IntN(9)
	if !g.loose && (x == 3 || x >= 5 && x <= 7) {
		// Each of these is a check diagnostic, so a typed program writes a
		// failure the checker cannot see instead.
		x = 0
	}
	switch x {
	case 0:
		args := []ast.Expr{g.str(pick(r, strValues))}
		if r.IntN(2) == 0 {
			args = append(args, g.expr(g.randTy(1), d))
		}
		return &ast.Call{Name: "fail", Args: args}
	case 1:
		if !t.isNum() && !g.loose {
			return g.expr(t, d)
		}
		var zero ast.Expr = g.num("0")
		if !g.loose {
			zero = &ast.Call{Name: "len", Args: []ast.Expr{g.str("")}}
		}
		return &ast.Binary{Op: pick(r, []string{"/", "%"}), X: g.expr(tyNum, d), Y: zero}
	case 2:
		et := t
		if g.loose || t.K == KAny {
			et = g.randTy(0)
		}
		return &ast.Index{X: g.expr(listOf(et, -1), d), I: g.num("99")}
	case 3:
		return &ast.Member{X: g.expr(recordOf([]TField{{Name: "a", T: t}}), d), Name: "zz"}
	case 4:
		if !t.isNum() && !g.loose {
			return g.expr(t, d)
		}
		return &ast.Binary{Op: "+", X: g.num("9007199254740991"), Y: g.expr(tyInt, d)}
	case 5:
		return &ast.Match{Subject: g.expr(tyInt, d), Arms: []ast.Arm{{Lits: []ast.Expr{g.num("99")}, Result: g.expr(t, d)}}}
	case 6:
		return &ast.Binary{Op: "*", X: g.expr(tyStr, d), Y: g.expr(tyInt, d)}
	case 7:
		return &ast.MethodCall{X: &ast.Null{}, Name: "trim"}
	default:
		return g.expr(t, d)
	}
}

func (g *Gen) lambdaCall(t *Ty, d int) ast.Expr {
	r := g.r
	var cands []bind
	for _, b := range g.visible() {
		if b.lam != nil && (fits(b.lam.result, t) || g.loose) {
			cands = append(cands, b)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	b := pick(r, cands)
	n := b.lam.req + r.IntN(len(b.lam.params)-b.lam.req+1)
	if g.loose && r.IntN(5) == 0 {
		n = r.IntN(len(b.lam.params) + 2) // arity
	}
	c := &ast.Call{Name: b.name}
	for i := range n {
		pt := tyAny
		if i < len(b.lam.params) {
			pt = b.lam.params[i]
		}
		if g.loose && r.IntN(10) == 0 {
			c.Args = append(c.Args, g.callback(&LibFunc{Params: []string{"any"}, Result: "any"}, nil, d))
			continue
		}
		if i >= b.lam.req && r.IntN(4) == 0 {
			c.Args = append(c.Args, &ast.Null{}) // takes the default
			continue
		}
		c.Args = append(c.Args, g.expr(pt, d))
	}
	return c
}

func (g *Gen) coal(t *Ty, d int) ast.Expr {
	var left ast.Expr
	switch g.r.IntN(4) {
	case 0:
		left = &ast.Null{}
	case 1:
		name := "a"
		if g.loose {
			name = pick(g.r, []string{"a", "zz"})
		}
		left = &ast.Member{X: g.expr(recordOf([]TField{{Name: "a", T: t}}), d), Name: name, Opt: true}
	case 2:
		left = &ast.MethodCall{X: g.expr(listOf(t, -1), d), Name: pick(g.r, []string{"first", "last"})}
	default:
		left = g.expr(t, d)
	}
	return &ast.Binary{Op: "??", X: left, Y: g.expr(t, d)}
}

func (g *Gen) logic(t *Ty, d int) ast.Expr {
	if !g.loose {
		if t.K != KBool {
			return nil
		}
		return &ast.Binary{Op: pick(g.r, []string{"&&", "||"}), X: g.test(d), Y: g.test(d)}
	}
	return &ast.Binary{Op: pick(g.r, []string{"&&", "||"}), X: g.expr(t, d), Y: g.expr(t, d)}
}

func (g *Gen) paren(t *Ty, d int) ast.Expr { return &ast.Paren{X: g.expr(t, d)} }

func (g *Gen) arith(t *Ty, d int) ast.Expr {
	r := g.r
	if t.K == KAny {
		t = tyNum
	}
	switch r.IntN(7) {
	case 0, 1:
		return &ast.Binary{Op: pick(r, []string{"+", "-"}), X: g.expr(t, d), Y: g.expr(t, d)}
	case 2:
		return &ast.Binary{Op: "*", X: g.expr(tyInt, d), Y: g.expr(tyInt, d)}
	case 3:
		return &ast.Binary{Op: "%", X: g.expr(t, d), Y: g.nonZero(t.K == KInt)}
	case 4:
		if t.K == KInt {
			return &ast.Binary{Op: "**", X: g.expr(tyInt, d), Y: g.smallInt(0, 3)}
		}
		return &ast.Binary{Op: "**", X: g.expr(tyNum, d), Y: g.smallInt(-2, 3)}
	case 5:
		if t.K == KInt {
			return nil
		}
		return &ast.Binary{Op: "/", X: g.expr(tyNum, d), Y: g.nonZero(false)}
	default:
		if t.K == KInt {
			return &ast.Call{Name: "len", Args: []ast.Expr{g.expr(pick(r, []*Ty{tyStr, listOf(tyAny, -1), mapAny()}), d)}}
		}
		return &ast.Binary{Op: pick(r, []string{"+", "-", "*"}), X: g.expr(tyNum, d), Y: g.expr(tyInt, d)}
	}
}

func (g *Gen) nonZero(whole bool) ast.Expr {
	if g.sound && g.r.IntN(2) == 0 {
		if e := g.varRead(tyInt, 0); e != nil {
			return e // may be zero: a divide site
		}
	}
	if whole || g.r.IntN(2) == 0 {
		return g.smallInt(1, 9)
	}
	return g.num(pick(g.r, []string{"0.5", "2.5", "0.1", "1e-7", "3"}))
}

func (g *Gen) neg(t *Ty, d int) ast.Expr { return &ast.Unary{Op: "-", X: g.expr(t, d)} }

func (g *Gen) concat(t *Ty, d int) ast.Expr {
	return &ast.Binary{Op: "+", X: g.expr(tyStr, d), Y: g.expr(tyStr, d)}
}

func (g *Gen) template(t *Ty, d int) ast.Expr {
	r := g.r
	tp := &ast.Template{}
	if !g.opts.NoF3 && r.IntN(6) == 0 {
		tp.Block, tp.Indent = true, g.indent()
	}
	n := 1 + r.IntN(3)
	for i := 0; i < n; i++ {
		if r.IntN(2) == 0 {
			tp.Parts = append(tp.Parts, ast.Part{Text: pick(r, append(strValues, "$", "${", "\\", "`", " - ", "\n"))})
		}
		var it *Ty
		for it == nil || it.K == KOpq && !g.loose {
			it = g.randTy(1)
		}
		tp.Parts = append(tp.Parts, ast.Part{X: g.expr(it, d)})
	}
	if r.IntN(2) == 0 {
		tp.Parts = append(tp.Parts, ast.Part{Text: pick(r, strValues)})
	}
	return tp
}

func (g *Gen) compare(t *Ty, d int) ast.Expr {
	ot := tyNum
	if g.r.IntN(3) == 0 {
		ot = tyStr
	}
	return &ast.Binary{Op: pick(g.r, []string{"<", "<=", ">", ">="}), X: g.expr(ot, d), Y: g.expr(ot, d)}
}

func (g *Gen) equal(t *Ty, d int) ast.Expr {
	ot := g.randTy(2)
	if !g.loose {
		ot = pick(g.r, []*Ty{tyInt, tyNum, tyStr, tyBool})
		if ot.K == KStr {
			// Two string literals have literal sets as types; one call of
			// string keeps them from being never-equal.
			return &ast.Binary{Op: pick(g.r, []string{"==", "!="}), X: &ast.Call{Name: "string", Args: []ast.Expr{g.expr(ot, d)}}, Y: g.expr(ot, d)}
		}
	}
	if g.opaque != "" && g.r.IntN(4) == 0 {
		ot = &Ty{K: KOpq, Opq: g.opaque, Len: -1}
	}
	return &ast.Binary{Op: pick(g.r, []string{"==", "!="}), X: g.expr(ot, d), Y: g.expr(ot, d)}
}

func (g *Gen) inExpr(t *Ty, d int) ast.Expr {
	op := pick(g.r, []string{"in", "not in"})
	switch g.r.IntN(3) {
	case 0:
		et := g.randTy(1)
		if !g.loose {
			et = pick(g.r, []*Ty{tyInt, tyStr})
		}
		return &ast.Binary{Op: op, X: g.nonLiteral(g.expr(et, d)), Y: g.expr(listOf(et, -1), d)}
	case 1:
		return &ast.Binary{Op: op, X: g.expr(tyStr, d), Y: g.nonLiteral(g.expr(tyStr, d))}
	default:
		return &ast.Binary{Op: op, X: g.str(pick(g.r, fieldNames)), Y: g.expr(mapAny(), d)}
	}
}

func (g *Gen) not(t *Ty, d int) ast.Expr {
	return &ast.Unary{Op: "!", X: g.test(d)}
}

func (g *Gen) listLit(t *Ty, d int) ast.Expr {
	et := tyAny
	if t.K == KList && t.Elem != nil {
		et = t.Elem
	}
	if et.K == KAny {
		et = g.randTy(1)
	}
	l := &ast.List{}
	n := g.r.IntN(4)
	for range n {
		if g.r.IntN(5) == 0 {
			l.Items = append(l.Items, ast.Item{Spread: true, X: g.expr(listOf(et, -1), d)})
		} else {
			l.Items = append(l.Items, ast.Item{X: g.expr(et, d)})
		}
	}
	return l
}

func (g *Gen) mapLit(t *Ty, d int) ast.Expr {
	r := g.r
	m := &ast.Map{}
	fs := t.Fields
	if t.K != KMap {
		fs = g.randTy(3).Fields
	}
	if r.IntN(4) == 0 {
		m.Entries = append(m.Entries, ast.Entry{Kind: ast.EntrySpread, X: g.expr(g.randTy(3), d)})
		if !g.loose && m.Entries[0].X != nil {
			// The spread must be a map.
			m.Entries[0].X = g.expr(mapAny(), d)
		}
	}
	for _, f := range fs {
		if b := g.lookup(f.Name); b != nil && b.lam == nil && fits(b.t, f.T) && r.IntN(2) == 0 {
			m.Entries = append(m.Entries, ast.Entry{Kind: ast.EntryShorthand, Key: f.Name})
			continue
		}
		m.Entries = append(m.Entries, g.entry(f.Name, g.expr(f.T, d)))
	}
	if g.loose && r.IntN(6) == 0 && len(fs) > 0 {
		m.Entries = append(m.Entries, ast.Entry{Kind: ast.EntryString, Key: fs[0].Name, X: g.leaf(tyInt)})
	}
	return m
}

func (g *Gen) member(t *Ty, d int) ast.Expr {
	r := g.r
	// A field of a variable whose type is known.
	for _, b := range g.visible() {
		if b.lam != nil || b.t.K != KMap {
			continue
		}
		for _, f := range b.t.Fields {
			if fits(f.T, t) && r.IntN(2) == 0 {
				return g.read(&ast.Ident{Name: b.name}, f.Name)
			}
		}
	}
	name := pick(r, fieldNames)
	rec := recordOf([]TField{{Name: name, T: t}})
	return g.read(g.expr(rec, d), name)
}

// read writes a read of the key k of x, as .k, ?.k, [k] or ?.[k].
func (g *Gen) read(x ast.Expr, k string) ast.Expr {
	opt := g.loose && g.r.IntN(4) == 0
	if isWord(k) && (!g.loose || g.r.IntN(3) > 0) {
		return &ast.Member{X: x, Name: k, Opt: opt}
	}
	return &ast.Index{X: x, I: g.str(k), Opt: opt}
}

func (g *Gen) index(t *Ty, d int) ast.Expr {
	r := g.r
	if t.K == KStr && r.IntN(2) == 0 {
		s := pick(r, strValues[1:])
		n := utf8.RuneCountInString(s)
		return &ast.Index{X: g.str(s), I: g.smallInt(0, n-1)}
	}
	for _, b := range g.visible() {
		if g.sound && b.lam == nil && b.t.K == KList && b.t.Len < 0 && b.t.Elem != nil && fits(b.t.Elem, t) && r.IntN(2) == 0 {
			return &ast.Index{X: &ast.Ident{Name: b.name}, I: g.smallInt(0, 2)} // an index site
		}
		if b.lam == nil && b.t.K == KList && b.t.Len > 0 && b.t.Elem != nil && fits(b.t.Elem, t) {
			return &ast.Index{X: &ast.Ident{Name: b.name}, I: g.smallInt(0, b.t.Len-1), Opt: g.loose && r.IntN(4) == 0}
		}
	}
	n := 1 + r.IntN(3)
	l := &ast.List{}
	for range n {
		l.Items = append(l.Items, ast.Item{X: g.expr(t, d)})
	}
	i := g.smallInt(0, n-1)
	if g.loose && r.IntN(3) == 0 {
		i = g.expr(g.randTy(0), d)
	}
	return &ast.Index{X: l, I: i, Opt: g.loose && r.IntN(5) == 0}
}

func (g *Gen) hostCall(t *Ty, d int) ast.Expr {
	r := g.r
	var cands []hostFn
	for _, h := range g.hosts {
		if fits(h.result, t) || t.K == KAny || g.loose {
			cands = append(cands, h)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	h := pick(r, cands)
	n := h.req + r.IntN(len(h.params)-h.req+1)
	if h.rest {
		n = r.IntN(3)
	}
	if g.loose && r.IntN(5) == 0 {
		n = r.IntN(4)
	}
	c := &ast.Call{NS: h.ns, Name: h.name}
	for i := range n {
		pt := tyAny
		if len(h.params) > 0 {
			pt = h.params[min(i, len(h.params)-1)]
		}
		if i == 0 && (h.name == "lookup" || h.name == "fetch") && r.IntN(2) == 0 {
			c.Args = append(c.Args, g.str(pick(r, []string{"a", "b", "c", ""})))
			continue
		}
		c.Args = append(c.Args, g.expr(pt, d))
	}
	if !fits(h.result, t) && t.K != KAny && !g.loose {
		return nil
	}
	return c
}

func (g *Gen) opqCall(t *Ty, d int) ast.Expr {
	r := g.r
	ot := &Ty{K: KOpq, Opq: g.opaque, Len: -1}
	var cands []opqMethod
	for _, m := range g.methods {
		if fits(m.result, t) || g.loose {
			cands = append(cands, m)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	m := pick(r, cands)
	recv := g.varRead(ot, 0)
	if recv == nil {
		return nil
	}
	c := &ast.MethodCall{X: recv, Name: m.name, Opt: g.loose && r.IntN(5) == 0}
	for _, p := range m.params {
		if p.K == KInt && r.IntN(2) == 0 {
			c.Args = append(c.Args, g.num("1"))
			continue
		}
		c.Args = append(c.Args, g.expr(p, d))
	}
	return c
}

// looseOddity makes a node that breaks a static rule or a kind rule.
func (g *Gen) looseOddity(t *Ty, d int) ast.Expr {
	r := g.r
	if !g.opts.NoF3 && r.IntN(10) == 0 {
		// A text block where only a String may stand: a map key.
		return &ast.Raw{Text: "{" + ast.StrText(&ast.Str{Val: "k", Block: 1, Indent: g.indent()}) + ": 1}"}
	}
	switch r.IntN(7) {
	case 0:
		return &ast.Call{Name: pick(r, []string{"nope", "print", "Len"}), Args: []ast.Expr{g.expr(t, d)}}
	case 1:
		for _, b := range g.visible() {
			if b.lam == nil {
				return &ast.Call{Name: b.name, Args: []ast.Expr{g.leaf(tyInt)}} // call-of-parameter
			}
		}
	case 2:
		if len(g.funcs) == 0 {
			return nil
		}
		return &ast.Call{Name: g.pickEntry(g.funcs).Name, Args: nil}
	case 3:
		return &ast.List{Items: []ast.Item{{X: g.callback(&LibFunc{Params: []string{"any"}, Result: "any"}, nil, d)}}}
	case 4:
		return &ast.Binary{Op: pick(r, []string{"+", "-", "*", "/", "%", "**", "<", "in"}), X: g.expr(g.randTy(1), d), Y: g.expr(g.randTy(1), d)}
	case 5:
		return &ast.Unary{Op: pick(r, []string{"-", "!"}), X: g.expr(g.randTy(1), d)}
	default:
		return &ast.Member{X: g.expr(g.randTy(1), d), Name: pick(r, reservedList)}
	}
	return nil
}

// pickEntry picks a library entry.
func (g *Gen) pickEntry(xs []*LibEntry) *LibEntry { return xs[g.r.IntN(len(xs))] }

// test makes the test of a condition. A typed program writes a test whose
// type can go both ways, so that the checker reports no always-truthy or
// always-falsy: a comparison, a bool, or a value that may be empty.
func (g *Gen) test(d int) ast.Expr {
	r := g.r
	if g.loose {
		return g.expr(g.randTy(1), d)
	}
	switch r.IntN(6) {
	case 0, 1:
		return g.compare(tyBool, d)
	case 2:
		return g.equal(tyBool, d)
	case 3:
		if e := g.varRead(tyBool, 0); e != nil {
			return e
		}
		return g.compare(tyBool, d)
	case 4:
		return &ast.Binary{Op: pick(r, []string{">", "=="}), X: &ast.Call{Name: "len", Args: []ast.Expr{g.expr(pick(r, []*Ty{tyStr, listOf(tyInt, -1)}), d)}}, Y: g.smallInt(0, 3)}
	default:
		return g.inExpr(tyBool, d)
	}
}

// nonLiteral wraps a string literal in a call of string, in a typed program,
// so that its type is string and not a literal set that could be
// never-equal to another.
func (g *Gen) nonLiteral(e ast.Expr) ast.Expr {
	if g.loose {
		return e
	}
	switch e.(type) {
	case *ast.Str, *ast.Template:
		return &ast.Call{Name: "string", Args: []ast.Expr{e}}
	}
	return e
}

// removeEntries removes one or two library entries from the environment. The
// program may still call them: a call of a removed entry is
// undefined-function, no-method, or wrong-kind at run time (03-host.md
// 3.3.1 item 6). A loose case may remove a name that is no entry.
func (g *Gen) removeEntries(c *Case) {
	r := g.r
	for range 1 + r.IntN(2) {
		e := g.lib.Entries[r.IntN(len(g.lib.Entries))]
		c.Removed = append(c.Removed, EntryName(e))
	}
	if g.loose && r.IntN(4) == 0 {
		c.Removed = append(c.Removed, pick(r, []string{"lenn", "repeat", "string.map", "list", "number.abs"}))
	}
}

// EntryName writes the name of an entry as the field removed does: a
// function by its name, a method by its kind and name.
func EntryName(e *LibEntry) string {
	switch {
	case e.Receiver == "":
		return e.Name
	case e.Receiver == "string":
		return "string." + e.Name
	case strings.HasPrefix(e.Receiver, "map<"):
		return "map." + e.Name
	default:
		return "list." + e.Name
	}
}
