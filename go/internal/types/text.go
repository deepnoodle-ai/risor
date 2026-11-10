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
	allowVars bool
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
	defer catch(&err)
	c := &conv{opaque: opaque, allowVars: allowVars}
	out = c.typ(t)
	if Depth(out) > MaxDepth {
		c.fail("the type is deeper than %d", MaxDepth)
	}
	return out, nil
}

func (c *conv) typ(t *typetext.Type) *Type {
	r := Never
	recs := 0
	for _, a := range t.Alts {
		x := c.alt(a)
		if x.rec != nil {
			recs++
			if recs > 1 {
				c.fail("two records in one type")
			}
		}
		r = Join(r, x)
	}
	return r
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
		if !c.opaque[p.Text] {
			c.fail("%s is not a declared opaque type", p.Text)
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
	defer catch(&err)
	c := &conv{opaque: opaque, allowVars: true}
	out = &Sig{Result: c.typ(s.Result), Fails: s.Fails, Bounds: map[string]string{}}
	check := func(t *Type) *Type {
		if Depth(t) > MaxDepth {
			c.fail("the type is deeper than %d", MaxDepth)
		}
		return t
	}
	check(out.Result)
	if recv != nil {
		out.Recv = check(c.alt(recv))
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
