package types

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/go/v3/internal/typetext"
)

// MaxDepth is the limit on the depth of a type (2.5, 20.4).
const MaxDepth = 64

// TextError is type text that the grammar accepts and the rules of 3.3 to
// 3.6 do not: the errors of 19.5 that need a checker.
type TextError struct{ Msg string }

func (e *TextError) Error() string { return e.Msg }

type conv struct {
	opaque    map[string]bool
	named     map[string]*Type
	allowVars bool
	// look resolves a name of a list being resolved (F17 12.2), with
	// whether it stands at a guarded place; false for a name outside it.
	look  func(name string, guarded bool) (*Type, bool)
	guard int
}

// Scope is what type text may name: the opaque types and the named types of
// an environment (package F4 3.3).
type Scope struct {
	Opaque map[string]bool
	Named  map[string]*Type
}

func (c *conv) check(t *Type) *Type {
	if Depth(t) > MaxDepth {
		c.fail("the type is deeper than %d", MaxDepth)
	}
	if Size(t) > MaxSize {
		c.fail("the type is larger than %d (05-types.md 20.7)", MaxSize)
	}
	return t
}

func (c *conv) fail(format string, args ...any) {
	panic(&TextError{Msg: fmt.Sprintf(format, args...)})
}

func catch(err *error) {
	if r := recover(); r != nil {
		te, ok := r.(*TextError)
		if !ok {
			panic(r)
		}
		*err = te
	}
}

// FromText converts a parsed Type. opaque holds the declared opaque type
// names. Type variables are allowed only with allowVars.
func FromText(t *typetext.Type, opaque map[string]bool, allowVars bool) (out *Type, err error) {
	return FromTextIn(t, &Scope{Opaque: opaque}, allowVars)
}

// FromTextIn converts a parsed Type in a scope: a Name is an opaque type or
// a named type (F4 3.5).
func FromTextIn(t *typetext.Type, sc *Scope, allowVars bool) (out *Type, err error) {
	defer catch(&err)
	c := &conv{opaque: sc.Opaque, named: sc.Named, allowVars: allowVars}
	out = c.check(c.typ(t))
	return out, nil
}

func (c *conv) typ(t *typetext.Type) *Type {
	alts := make([]*Type, len(t.Alts))
	for i, a := range t.Alts {
		alts[i] = c.alt(a)
	}
	out, ok := Alternatives3(alts)
	if !ok {
		c.fail("two or more records that form no tagged union in one type")
	}
	return out
}

// Alternatives3 joins the alternatives of one Type of type text
// (05-types.md 3.4; F5 3.1; F17 12.3). A reference whose definition is
// known and that shares a kind other than null with another alternative
// counts as its definition; any other reference stands as it is, beside the
// rest. The records, a union contributing its variants, must form a tagged
// union when they are two or more: false otherwise.
func Alternatives3(alts []*Type) (*Type, bool) {
	alts = unfoldShared(alts)
	xs := make([]*Type, 0, len(alts))
	var refs []*Named
	var recs []*Record
	for _, x := range alts {
		refs = append(refs, x.refs...)
		x = x.plain()
		if x.rec != nil {
			recs = append(recs, variants(x)...)
			x = WithoutMap(x)
		}
		xs = append(xs, x)
	}
	out := JoinAll(xs)
	if len(recs) > 0 {
		m, ok := FormUnion(recs)
		if !ok {
			return nil, false
		}
		out = WithMap(out, m)
	}
	return withRefs(out, refs), true
}

// unfoldShared unfolds each reference whose definition is known and that
// shares a kind other than null with another alternative.
func unfoldShared(alts []*Type) []*Type {
	any := false
	for _, x := range alts {
		if len(x.refs) > 0 {
			any = true
		}
	}
	if !any {
		return alts
	}
	out := append([]*Type(nil), alts...)
	for changed := true; changed; {
		changed = false
		for i, x := range out {
			if len(x.refs) == 0 {
				continue
			}
			p := x.plain()
			var kept []*Named
			for _, n := range x.refs {
				if n.def != nil && sharesKnown(Ref(n), out, i, n) {
					p = mergeKinds(p, n.def)
					changed = true
				} else {
					kept = append(kept, n)
				}
			}
			if len(kept) != len(x.refs) {
				out[i] = withRefs(p, kept)
			}
		}
	}
	return out
}

// sharesKnown reports whether r shares a kind other than null with an
// alternative of alts other than the reference n at i, looking only at
// what is known: a reference whose definition is not known has no kind
// yet.
func sharesKnown(r *Type, alts []*Type, at int, n *Named) bool {
	for j, y := range alts {
		parts := []*Type{y.plain()}
		for _, m := range y.refs {
			if (j == at && m == n) || m.def == nil {
				continue
			}
			parts = append(parts, Ref(m))
		}
		for _, p := range parts {
			if !p.IsNever() && SharesKind(r, p) {
				return true
			}
		}
	}
	return false
}

func (c *conv) alt(a *typetext.Alt) *Type {
	if a.Lists > 0 {
		c.guard++
		defer func() { c.guard-- }()
	}
	t := c.prim(a.Prim)
	for i := 0; i < a.Lists; i++ {
		t = List(t)
	}
	return t
}

func (c *conv) prim(p *typetext.Prim) *Type {
	switch p.Kind {
	case typetext.PrimWord:
		switch p.Text {
		case "null":
			return Null
		case "bool":
			return Bool
		case "int":
			return Int
		case "float":
			return Float
		case "string":
			return String
		case "any":
			return Any
		}
		return Never
	case typetext.PrimString:
		return Lit(p.Text)
	case typetext.PrimMap:
		c.guard++
		defer func() { c.guard-- }()
		return MapOf(c.typ(p.Elem))
	case typetext.PrimParen:
		return c.typ(p.Elem)
	case typetext.PrimName:
		if c.look != nil {
			if t, ok := c.look(p.Text, c.guard > 0); ok {
				return t
			}
		}
		if t, ok := c.named[p.Text]; ok {
			return t
		}
		if !c.opaque[p.Text] {
			c.fail("%s is not a declared opaque type or named type", p.Text)
		}
		return Opaque(p.Text)
	case typetext.PrimVar:
		if !c.allowVars {
			c.fail("a type variable may stand only in a signature")
		}
		return Var(p.Text)
	}
	c.guard++
	defer func() { c.guard-- }()
	seen := map[string]bool{}
	var fs []Field
	for _, f := range p.Record.Fields {
		if seen[f.Key] {
			c.fail("the key %q is written twice", f.Key)
		}
		seen[f.Key] = true
		fs = append(fs, Field{Key: f.Key, Type: c.typ(f.Type), Optional: f.Optional})
	}
	rest := Never
	if p.Record.HasRest {
		rest = Any
		if p.Record.Rest != nil {
			rest = c.typ(p.Record.Rest)
		}
	}
	return Rec(fs, rest)
}

// Param is one parameter of a signature.
type Param struct {
	Name     string
	Optional bool
	Rest     bool
	// Type is the parameter's type; for a rest parameter, the type of each
	// argument it takes (P of ...rest: P[]).
	Type *Type
	// Fn is set for a parameter that takes a lambda.
	Fn *Fn
}

// Fn is the type of a callback parameter.
type Fn struct {
	Params []*Type
	// Result is nil for unknown: a predicate (13.7).
	Result *Type
}

// Sig is a signature (3.6).
type Sig struct {
	Recv   *Type // for a method; nil for a function
	Params []Param
	Result *Type
	Fails  bool
	Bounds map[string]string
}

// Min is the count of required parameters; Max the most, -1 with a rest.
func (s *Sig) Min() int {
	n := 0
	for _, p := range s.Params {
		if !p.Optional && !p.Rest {
			n++
		}
	}
	return n
}

// Max is the most arguments, -1 with a rest parameter.
func (s *Sig) Max() int {
	for _, p := range s.Params {
		if p.Rest {
			return -1
		}
	}
	return len(s.Params)
}

// ParamAt is the parameter at argument i, or nil.
func (s *Sig) ParamAt(i int) *Param {
	if i < len(s.Params) {
		return &s.Params[i]
	}
	if n := len(s.Params); n > 0 && s.Params[n-1].Rest {
		return &s.Params[n-1]
	}
	return nil
}

// FromSig converts a parsed signature, with an optional receiver.
func FromSig(recv *typetext.Alt, s *typetext.Sig, opaque map[string]bool) (out *Sig, err error) {
	return FromSigIn(recv, s, &Scope{Opaque: opaque})
}

// FromSigIn converts a parsed signature in a scope. The receiver may name an
// opaque type only (F4 3.3).
func FromSigIn(recv *typetext.Alt, s *typetext.Sig, sc *Scope) (out *Sig, err error) {
	defer catch(&err)
	c := &conv{opaque: sc.Opaque, named: sc.Named, allowVars: true}
	out = &Sig{Result: c.typ(s.Result), Fails: s.Fails, Bounds: map[string]string{}}
	check := c.check
	check(out.Result)
	if recv != nil {
		rc := &conv{opaque: sc.Opaque, allowVars: true}
		out.Recv = check(rc.alt(recv))
	}
	for _, b := range s.Bounds {
		out.Bounds[b.Var] = b.Bound
	}
	for _, p := range s.Params {
		q := Param{Name: p.Name, Optional: p.Optional, Rest: p.Rest}
		if p.Fn != nil {
			fn := &Fn{}
			for _, fp := range p.Fn.Params {
				fn.Params = append(fn.Params, check(c.typ(fp.Type)))
			}
			if p.Fn.Result != nil {
				fn.Result = check(c.typ(p.Fn.Result))
			}
			q.Fn = fn
		} else {
			t := check(c.typ(p.Type))
			if p.Rest {
				if t.list == nil || !Equal(t, List(t.list)) {
					c.fail("a rest parameter must be a list")
				}
				t = t.list
			}
			q.Type = t
		}
		out.Params = append(out.Params, q)
	}
	return out, nil
}

// TextUses lists the names in type text, in text order, each with whether
// it stands at a guarded place (F17 12.2).
func TextUses(t *typetext.Type, add func(name string, guarded bool)) {
	var typ func(t *typetext.Type, g bool)
	typ = func(t *typetext.Type, g bool) {
		for _, a := range t.Alts {
			ga := g || a.Lists > 0
			p := a.Prim
			switch p.Kind {
			case typetext.PrimName:
				add(p.Text, ga)
			case typetext.PrimMap:
				typ(p.Elem, true)
			case typetext.PrimParen:
				typ(p.Elem, ga)
			case typetext.PrimRecord:
				for _, f := range p.Record.Fields {
					typ(f.Type, true)
				}
				if p.Record.Rest != nil {
					typ(p.Record.Rest, true)
				}
			}
		}
	}
	typ(t, false)
}

// FromTextList converts the type text of an item of a list being resolved:
// look resolves the names of the list.
func FromTextList(t *typetext.Type, sc *Scope, look func(name string, guarded bool) (*Type, bool)) (out *Type, err error) {
	defer catch(&err)
	c := &conv{opaque: sc.Opaque, named: sc.Named, look: look}
	out = c.check(c.typ(t))
	return out, nil
}
