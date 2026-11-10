// Package check is the Core type checker of spec/core/05-types.md, sections
// 4 to 21. It takes a parsed program, the result of the static rules
// (internal/static, section 22) and the types of an environment, and gives
// the program's type, its sites and the diagnostics of the type rules.
package check

import (
	"sort"

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
}

// Diag is one diagnostic of the type rules.
type Diag struct {
	Code  string
	Span  syntax.Span
	Class string
}

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

// facts are the not-null facts in force (10.4).
type facts []path

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
		if q.hasPrefix(p) {
			return true
		}
	}
	return false
}

func union(a, b facts) facts { return a.with(b) }

func intersect(a, b facts) facts {
	var out facts
	for _, p := range a {
		for _, q := range b {
			if p.equal(q) {
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
	env      *Env
	res      *static.Result
	diags    []Diag
	sites    []Site
	slots    []*types.Type
	ty       map[syntax.Node]*types.Type
	lambdas  map[*syntax.Lambda]*lamInfo
	used     map[*syntax.Lambda]bool
	uses     int
	base     int // expanded nesting at depth lamDepth (20.2)
	lamDepth int
	drop     int // > 0 while sites are dropped (14.1, 13.9)
	dynSites map[syntax.Span]bool
}

// Check runs the type rules on a program whose names res resolved.
func Check(prog *syntax.Program, res *static.Result, env *Env, exp Expect) *Result {
	c := &checker{
		env: env, res: res,
		slots:    make([]*types.Type, res.NSlots),
		ty:       map[syntax.Node]*types.Type{},
		lambdas:  map[*syntax.Lambda]*lamInfo{},
		used:     map[*syntax.Lambda]bool{},
		dynSites: map[syntax.Span]bool{},
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
	return c.result(prog, t)
}

func (c *checker) result(prog *syntax.Program, t *types.Type) *Result {
	r := &Result{Type: t, Dynamic: c.covered(prog)}
	seen := map[Diag]bool{}
	for _, d := range c.diags {
		if !seen[d] {
			seen[d] = true
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
	c.diags = append(c.diags, Diag{Code: code, Span: sp})
}

// site records a site (18.3), or its diagnostic when the class is rejected
// (17). It reports whether the class is rejected.
func (c *checker) site(class string, sp syntax.Span) bool {
	if class == "dynamic" {
		c.dynSites[sp] = true
	}
	if c.drop > 0 {
		return false
	}
	if c.env.Reject[class] {
		code := "may-fail"
		if class == "dynamic" {
			code = "unchecked"
		}
		c.diags = append(c.diags, Diag{Code: code, Span: sp, Class: class})
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
		c.diags = append(c.diags, Diag{Code: "always-truthy", Span: n.Span(), Class: "constant-condition"})
	case types.Truthy(t).IsNever():
		c.diags = append(c.diags, Diag{Code: "always-falsy", Span: n.Span(), Class: "constant-condition"})
	}
}

// mustFit is "e must fit T" of 1.3. It reports whether e fits.
func (c *checker) mustFit(sp syntax.Span, t, want *types.Type, code string) bool {
	ok, used := types.Fits(t, want)
	if !ok {
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
		c.diag("maybe-null", sp)
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
	for _, b := range c.res.Idents {
		if b.Kind == static.ConstLambda {
			c.used[b.Lambda] = true
		}
	}
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
	for _, e := range library.Entries {
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
			}
			return true
		case *syntax.MapPat:
			// A field's failure is at a site on its key (ruling AO6).
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
