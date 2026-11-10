package types

import (
	"fmt"

	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
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
	// F5 3.1: the records of a Type, a union contributing its variants; two
	// or more must form a tagged union.
	xs := make([]*Type, 0, len(t.Alts))
	var recs []*Record
	for _, a := range t.Alts {
		x := c.alt(a)
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
			c.fail("two or more records that form no tagged union in one type")
		}
		out = WithMap(out, m)
	}
	return out
}

func (c *conv) alt(a *typetext.Alt) *Type {
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
		return MapOf(c.typ(p.Elem))
	case typetext.PrimParen:
		return c.typ(p.Elem)
	case typetext.PrimName:
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
