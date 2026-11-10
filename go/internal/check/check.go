// Package check is the Core type checker of spec/core/05-types.md, sections
// 4 to 21. It takes a parsed program, the result of the static rules
// (internal/static, section 22) and the types of an environment, and gives
// the program's type, its sites and the diagnostics of the type rules.
package check

import (
	"sort"
	"strconv"

	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/static"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
)

// ExpectKind is the kind of an expected result (03-host.md 3.2.3).
type ExpectKind uint8

// The kinds of expected result.
const (
	ExpectNone ExpectKind = iota
	ExpectType
	ExpectCondition
)

// Expect is the expected result of a check.
type Expect struct {
	Kind ExpectKind
	Type *types.Type
}

// Env is what the type rules read from an environment.
type Env struct {
	// VarTypes are the declared types of the variables, by the index of
	// static.Names.Vars.
	VarTypes []*types.Type
	// Hosts are the host functions' signatures, by full name.
	Hosts map[string]*types.Sig
	// Methods are the opaque types' methods, by type name then method name.
	Methods map[string]map[string]*types.Sig
	// Reject is the set of rejected check classes.
	Reject map[string]bool
	// Removed is the set of removed library entries, by the names of
	// 03-host.md 3.3.1 (len, string.repeat).
	Removed map[string]bool
	// Facts are the host facts someNotNull of package F14, each the
	// indexes of its members in VarTypes.
	Facts [][]int
}

// Diag is one diagnostic of the type rules.
type Diag struct {
	Code  string
	Span  syntax.Span
	Class string
	// Tmpl and Params are the message's template and parameters (package
	// F8 3.5). The text of the span is added where the message is built.
	Tmpl   string
	Params []messages.Param
}

// pending is the template and parameters of the next diagnostic.
type pending struct {
	tmpl   string
	params []messages.Param
}

// m names the template and parameters of the next diagnostic.
func (c *checker) m(tmpl string, ps ...messages.Param) *checker {
	c.pend = pending{tmpl, ps}
	return c
}

// named names the call of the next value or host site.
func (c *checker) named(n syntax.Node) *checker {
	c.siteNode = n
	return c
}

// mustFitM is mustFit with the message of its failure, built only when it
// fails: the template names the parameters, w is the operator of in-left
// or the call of body.
func (c *checker) mustFitM(sp syntax.Span, t, want *types.Type, code, tmpl, w string) bool {
	ok, used := types.Fits(t, want)
	if !ok {
		switch tmpl {
		case "in-left":
			c.m(tmpl, word("op", w), ty("type", t))
		case "body":
			c.m(tmpl, word("name", w), ty("found", t), ty("expected", want))
		default:
			c.m(tmpl, ty("type", t))
		}
		c.diag(code, sp)
		return false
	}
	if used {
		c.dyn(sp)
	}
	return true
}

// callName is the name of a call as a message gives it: a namespaced
// function by its full name, a method by its name.
func (c *checker) callName(n syntax.Node) string {
	switch n := n.(type) {
	case *syntax.Call:
		if n.Namespace != "" {
			return n.Namespace + "." + n.Name
		}
		return n.Name
	case *syntax.MethodCall:
		if id, ok := n.X.(*syntax.Ident); ok && c.res.Calls[n] != nil {
			return id.Name + "." + n.Name
		}
		return n.Name
	}
	return ""
}

func useTmpl(uses int) string {
	if uses > MaxUses {
		return "bodies"
	}
	return "nesting"
}

func limitTmpl(t *types.Type) string {
	if types.Size(t) > types.MaxSize {
		return "size"
	}
	return "depth"
}

func boundTmpl(b string) string {
	if b == "ordered" {
		return "ordered"
	}
	return "number"
}

func opWord(op string) string {
	if op == "not-in" {
		return "not in"
	}
	return op
}

func params(ps ...messages.Param) []messages.Param { return ps }

func ty(name string, t *types.Type) messages.Param { return messages.Word(name, t.String()) }
func word(name, w string) messages.Param           { return messages.Word(name, w) }
func keyP(k string) messages.Param                 { return messages.Val("key", k) }
func num(name string, n int) messages.Param        { return messages.Word(name, strconv.Itoa(n)) }

// Site is one site (18.3).
type Site struct {
	Class string
	Span  syntax.Span
}

// Result is the outcome of a check (6.1).
type Result struct {
	Type  *types.Type
	Sites []Site
	Diags []Diag
	// Dynamic holds the span of every node at a dynamic site: a node with a
	// dynamic site, or one whose operand, receiver, index or argument has
	// one (18.3). The class of a run failure in a checked run uses it.
	Dynamic map[syntax.Span]bool
	// Host holds the span of every call with a host site, dropped or not:
	// a function-error there has the class host, elsewhere environment
	// (18.5).
	Host map[syntax.Span]bool
	// Value holds the span of every node with a value site, dropped or not:
	// a not-conforming there has the class value (F4 6.6).
	Value map[syntax.Span]bool
}

// MaxUses is the most lambda body checks of 20.3; MaxNesting is the
// expanded nesting of 20.2.
const (
	MaxUses    = 1000
	MaxNesting = 100
)

// path is a path of 10.3: a binding and field keys. root is a slot of a
// value binding, or -1 - i for environment variable i.
type path struct {
	root int
	keys []string
}

func (p path) equal(q path) bool {
	if p.root != q.root || len(p.keys) != len(q.keys) {
		return false
	}
	for i := range p.keys {
		if p.keys[i] != q.keys[i] {
			return false
		}
	}
	return true
}

// hasPrefix reports whether q starts with p.
func (q path) hasPrefix(p path) bool {
	if p.root != q.root || len(p.keys) > len(q.keys) {
		return false
	}
	for i := range p.keys {
		if p.keys[i] != q.keys[i] {
			return false
		}
	}
	return true
}

// factKind is the kind of a fact: not null (10.4), or the two of package F4
// 6.4.
type factKind uint8

const (
	factNonnull factKind = iota
	factConforms
	factNotConforms
	// factNull is null(p) of package F14 4.1, about a member path.
	factNull
)

// fact is one fact about a path.
type fact struct {
	p    path
	kind factKind
	t    *types.Type // for the two facts of F4
	// field is set for a fact of a test of a field (F5 5.2, 5.3).
	field bool
}

func nn(p path) fact { return fact{p: p} }

func (a fact) same(b fact) bool {
	return a.kind == b.kind && a.p.equal(b.p) && (a.kind == factNonnull || a.kind == factNull || types.Equal(a.t, b.t))
}

// facts are the facts in force (10.4), in the order of their tests.
type facts []fact

func (f facts) with(more facts) facts {
	if len(more) == 0 {
		return f
	}
	out := make(facts, 0, len(f)+len(more))
	return append(append(out, f...), more...)
}

// covers reports whether some fact's path starts with p: then p is not
// null, and every field on the way to it is present.
func (f facts) covers(p path) bool {
	for _, q := range f {
		if q.kind == factNonnull && q.p.hasPrefix(p) {
			return true
		}
	}
	return false
}

// apply gives the type of an expression that is the path p, under the
// facts of F4 6.4, in the order of their tests.
func (f facts) apply(t *types.Type, p path) *types.Type {
	for _, q := range f {
		if q.kind == factNonnull || q.kind == factNull || !q.p.equal(p) {
			continue
		}
		if q.kind == factConforms {
			if !types.SurelyFits(t, q.t) {
				t = q.t
			}
		} else {
			t = types.WithoutType(t, q.t)
		}
		if q.field {
			t = types.MarkTested(t)
		}
	}
	return t
}

func union(a, b facts) facts { return a.with(b) }

func intersect(a, b facts) facts {
	var out facts
	for _, p := range a {
		for _, q := range b {
			if p.same(q) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// orFacts is T(d || e): the facts both sides have, and, for a path about
// which each side has exactly one conforms fact, one fact of their Join,
// where the left one stood (F4 6.4 item 4).
func orFacts(a, b facts) facts {
	one := func(fs facts, p path) (fact, bool) {
		var got fact
		n := 0
		for _, q := range fs {
			if q.kind == factConforms && q.p.equal(p) {
				got = q
				n++
			}
		}
		return got, n == 1
	}
	var out facts
	for _, p := range a {
		if p.kind == factConforms {
			if _, ok := one(a, p.p); ok {
				if q, ok := one(b, p.p); ok {
					out = append(out, fact{p: p.p, kind: factConforms, t: types.Unite(p.t, q.t), field: p.field || q.field}) // F5 5.4
					continue
				}
			}
		}
		for _, q := range b {
			if p.same(q) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// lamInfo is where a const-bound lambda was written.
type lamInfo struct {
	facts    facts
	base     int
	lamDepth int
	maxRel   int
}

type checker struct {
	curCall    syntax.Node
	pend       pending
	siteNode   syntax.Node
	env        *Env
	res        *static.Result
	diags      []Diag
	sites      []Site
	slots      []*types.Type
	ty         map[syntax.Node]*types.Type
	lambdas    map[*syntax.Lambda]*lamInfo
	used       map[*syntax.Lambda]bool
	uses       int
	base       int // expanded nesting at depth lamDepth (20.2)
	lamDepth   int
	drop       int // > 0 while sites are dropped (14.1, 13.9)
	dynSites   map[syntax.Span]bool
	hostSites  map[syntax.Span]bool
	valueSites map[syntax.Span]bool
}

// Check runs the type rules on a program whose names res resolved.
func Check(prog *syntax.Program, res *static.Result, env *Env, exp Expect) *Result {
	var c checker
	t := c.run(prog, res, env, exp)
	return c.result(prog, t)
}

// run is the one path of a check, for Check and CheckTypes: it applies
// the rules, leaves what it recorded in c, and gives the program's type.
// The caller owns c, so that an ordinary check keeps it off the heap.
func (c *checker) run(prog *syntax.Program, res *static.Result, env *Env, exp Expect) *types.Type {
	*c = checker{
		env: env, res: res,
		slots:    make([]*types.Type, res.NSlots),
		ty:       map[syntax.Node]*types.Type{},
		lambdas:  map[*syntax.Lambda]*lamInfo{},
		used:     map[*syntax.Lambda]bool{},
		dynSites: map[syntax.Span]bool{}, hostSites: map[syntax.Span]bool{}, valueSites: map[syntax.Span]bool{},
	}
	for i := range c.slots {
		c.slots[i] = types.Never
	}
	c.findUses(prog)
	var fs facts
	for _, k := range prog.Consts {
		c.constLine(k, fs)
	}
	t := c.synth(prog.Result, fs)
	switch exp.Kind {
	case ExpectType:
		c.fitForm(prog.Result, t, exp.Type, formResult)
	case ExpectCondition:
		c.tested(prog.Result, t)
	}
	return t
}

func (c *checker) result(prog *syntax.Program, t *types.Type) *Result {
	r := &Result{Type: t, Dynamic: c.covered(prog), Host: c.hostSites, Value: c.valueSites}
	type dk struct {
		code, class string
		sp          syntax.Span
	}
	seen := map[dk]bool{}
	for _, d := range c.diags {
		if k := (dk{d.Code, d.Class, d.Span}); !seen[k] {
			seen[k] = true
			r.Diags = append(r.Diags, d)
		}
	}
	sort.Slice(r.Diags, func(i, j int) bool {
		a, b := r.Diags[i], r.Diags[j]
		if a.Span != b.Span {
			if a.Span.Start != b.Span.Start {
				return a.Span.Start < b.Span.Start
			}
			return a.Span.End < b.Span.End
		}
		return a.Code < b.Code
	})
	sseen := map[Site]bool{}
	for _, s := range c.sites {
		if !sseen[s] {
			sseen[s] = true
			r.Sites = append(r.Sites, s)
		}
	}
	sort.Slice(r.Sites, func(i, j int) bool {
		a, b := r.Sites[i], r.Sites[j]
		if a.Span != b.Span {
			if a.Span.Start != b.Span.Start {
				return a.Span.Start < b.Span.Start
			}
			return a.Span.End < b.Span.End
		}
		return a.Class < b.Class
	})
	return r
}

func (c *checker) diag(code string, sp syntax.Span) {
	c.diags = append(c.diags, Diag{Code: code, Span: sp, Tmpl: c.pend.tmpl, Params: c.pend.params})
	c.pend = pending{}
}

// site records a site (18.3), or its diagnostic when the class is rejected
// (17). It reports whether the class is rejected.
func (c *checker) site(class string, sp syntax.Span) bool {
	named := c.siteNode
	c.siteNode = nil
	switch class {
	case "dynamic":
		c.dynSites[sp] = true
	case "host":
		c.hostSites[sp] = true
	case "value":
		c.valueSites[sp] = true
	}
	if c.drop > 0 {
		return false
	}
	if c.env.Reject[class] {
		code, tmpl := "may-fail", class
		var ps []messages.Param
		if class == "dynamic" {
			code, tmpl = "unchecked", ""
		}
		if class == "value" || class == "host" {
			name := c.callName(named)
			ps = []messages.Param{word("name", name)}
			if name == "" {
				tmpl = "test"
			}
		}
		c.diags = append(c.diags, Diag{Code: code, Span: sp, Class: class, Tmpl: tmpl, Params: ps})
		return true
	}
	c.sites = append(c.sites, Site{Class: class, Span: sp})
	return false
}

// dyn records a dynamic site. It reports whether the class is rejected, in
// which case the node takes the type never (17).
func (c *checker) dyn(sp syntax.Span) bool { return c.site("dynamic", sp) }

// tested checks a tested position (10.2).
func (c *checker) tested(n syntax.Node, t *types.Type) {
	if !c.env.Reject["constant-condition"] || t.IsAny() || t.IsNever() {
		return
	}
	switch {
	case types.Falsy(t).IsNever():
		c.diags = append(c.diags, Diag{Code: "always-truthy", Span: n.Span(), Class: "constant-condition", Params: []messages.Param{ty("type", t)}})
	case types.Truthy(t).IsNever():
		c.diags = append(c.diags, Diag{Code: "always-falsy", Span: n.Span(), Class: "constant-condition", Params: []messages.Param{ty("type", t)}})
	}
}

// mustFit is "e must fit T" of 1.3. It reports whether e fits.
func (c *checker) mustFit(sp syntax.Span, t, want *types.Type, code string) bool {
	pend := c.pend
	c.pend = pending{}
	ok, used := types.Fits(t, want)
	if !ok {
		c.pend = pend
		c.diag(code, sp)
		return false
	}
	if used {
		c.dyn(sp)
	}
	return true
}

// nullable applies "an operand may be null" (1.3).
func (c *checker) nullable(sp syntax.Span, t *types.Type) *types.Type {
	if t.HasNull() && !t.IsAny() {
		c.m("", ty("type", t)).diag("maybe-null", sp)
		return types.Nonnull(t)
	}
	return t
}

func (c *checker) setSlot(key any, t *types.Type) {
	if s, ok := c.res.Slots[key]; ok && s >= 0 {
		c.slots[s] = t
	}
}

// findUses marks each const-bound lambda that a call or an argument names
// (13.9).
func (c *checker) findUses(prog *syntax.Program) {
	for _, callee := range c.res.Calls {
		if callee.Kind == static.Lambda {
			c.used[callee.Lambda] = true
		}
	}
	// Only a name given as an argument is a use; a read of the name as a
	// value is not-a-value and no use (13.9).
	mark := func(args []syntax.Node) {
		for _, a := range args {
			if id, ok := a.(*syntax.Ident); ok {
				if b := c.res.Idents[id]; b != nil && b.Kind == static.ConstLambda {
					c.used[b.Lambda] = true
				}
			}
		}
	}
	syntax.Walk(prog, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.Call:
			mark(n.Args)
		case *syntax.MethodCall:
			mark(n.Args)
		}
		return true
	})
}

// errorType is the type of the name of catch (err) (14.2).
var errorType = func() *types.Type {
	var codes []string
	for _, code := range messages.Codes() {
		if e, ok := messages.Lookup(code); ok && e.Phase == "run" && messages.Caught(code) {
			codes = append(codes, code)
		}
	}
	return types.Rec([]types.Field{
		{Key: "code", Type: types.Lit(codes...)},
		{Key: "data", Type: types.Any},
		{Key: "message", Type: types.String},
		{Key: "path", Type: types.WithNull(types.List(types.Join(types.Int, types.String)))},
	}, nil)
}()

var libSigs = map[*library.Entry]*types.Sig{}

// LibSig is the signature of a library entry, with its receiver.
func LibSig(e *library.Entry) *types.Sig {
	if s, ok := libSigs[e]; ok {
		return s
	}
	var recv *typetext.Alt
	if e.Receiver != "" {
		r, err := typetext.ParseRecv(e.Receiver)
		if err != nil {
			panic(err)
		}
		recv = r
	}
	s, err := types.FromSig(recv, e.Sig, nil)
	if err != nil {
		panic("check: library " + e.Name + ": " + err.Error())
	}
	libSigs[e] = s
	return s
}

func init() {
	for _, e := range library.AllEntries {
		LibSig(e)
	}
}

// covered computes Result.Dynamic: each node with a dynamic site, and each
// node whose operand, receiver, index or argument has one (18.3). For a
// const line, the site on its value covers its pattern.
func (c *checker) covered(prog *syntax.Program) map[syntax.Span]bool {
	out := map[syntax.Span]bool{}
	for sp := range c.dynSites {
		out[sp] = true
	}
	syntax.Walk(prog, func(n syntax.Node) bool {
		var kids []syntax.Node
		switch n := n.(type) {
		case *syntax.Const:
			if c.dynSites[n.Value.Span()] {
				out[n.Pattern.Span()] = true
				// A field's or an element's failure is a failure of the
				// pattern on the value bound (18.3).
				switch p := n.Pattern.(type) {
				case *syntax.MapPat:
					for _, f := range p.Fields {
						out[f.Span] = true
					}
				case *syntax.ListPat:
					for _, e := range p.Elems {
						out[e.Span] = true
					}
				}
			}
			return true
		case *syntax.Call, *syntax.MethodCall:
			// A callback's result, key or value: at a site on the body of a
			// lambda that is an argument, or that an argument names (18.3).
			var args []syntax.Node
			if call, ok := n.(*syntax.Call); ok {
				args = call.Args
			} else {
				args = n.(*syntax.MethodCall).Args
			}
			for _, a := range args {
				if l, _ := c.lambdaArg(a); l != nil && c.dynSites[l.Body.Span()] {
					out[n.Span()] = true
				}
			}
			kids = syntax.Children(n)
		case *syntax.MapPat:
			// A field's failure is at a site on its key (18.3).
			for _, f := range n.Fields {
				if c.dynSites[f.KeySpan] {
					out[f.Span] = true
				}
			}
		case *syntax.Lambda, *syntax.Block, *syntax.Program, *syntax.Try, *syntax.Match, *syntax.Cond:
			return true
		default:
			kids = syntax.Children(n)
		}
		for _, k := range kids {
			if c.dynSites[k.Span()] {
				out[n.Span()] = true
			}
		}
		return true
	})
	return out
}
