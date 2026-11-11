package risor_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Rules seen at the Go API: a const-bound lambda named as an argument is a
// use (05-types.md 13.9); a pattern's failure is at a site on the value
// bound (18.3); a host site is on the failing call only (18.5); and child
// environments (03-host.md 3.3.2) with the removed library entries (3.3.1).

func TestNamedLambdaOnAnyReceiver(t *testing.T) {
	// A const-bound lambda named as an argument of a method on an any
	// receiver is a use: its body is checked there.
	env, err := risor.NewEnv(risor.VarRecord("{m: any}"))
	assert.NoError(t, err)
	ds := checkErr(t, env, "const f4 = () => ``.l; m.r(f4)")
	assert.Equal(t, ds[0].Code, "kind")
	assert.Equal(t, ds[0].Span.Array(), [4]int{1, 18, 1, 20})
	// Its sites are kept.
	env, err = risor.NewEnv(risor.VarRecord("{m: any, b: any}"))
	assert.NoError(t, err)
	x, _ := risor.Parse("const f = () => b.x; m.r(f)")
	r, err := env.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, len(r.Sites), 2)
	assert.Equal(t, r.Sites[0].Span.Array(), [4]int{1, 17, 1, 18})
}

func TestPatternOnDynamicValue(t *testing.T) {
	// A pattern's failure is at a site when the site is on the value bound.
	env, err := risor.NewEnv(risor.VarRecord("{items: any}"))
	assert.NoError(t, err)
	x, err := risor.Parse("const {a: k7} = {c: items}.values().find(() => false) ?? {}; false")
	assert.NoError(t, err)
	p, err := env.Compile(x)
	assert.NoError(t, err)
	_, err = p.Run(context.Background(), risor.Map{"items": 0})
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "missing-field")
	assert.Equal(t, e.Diagnostics[0].Class, "dynamic")
}

func TestRemovedEntries(t *testing.T) {
	parent, err := risor.NewEnv(risor.Without("string.repeat", "len"), risor.VarRecord("{s: string}"))
	assert.NoError(t, err)
	// A child keeps the parent's removals; removing again is no error.
	child, err := parent.With(risor.Without("len", "list.map"))
	assert.NoError(t, err)
	ds := checkErr(t, child, `s.repeat(2)`)
	assert.Equal(t, ds[0].Code, "no-method")
	ds = checkErr(t, child, `len(s)`)
	assert.Equal(t, ds[0].Code, "undefined-function")
	d, err := child.Describe()
	assert.NoError(t, err)
	assert.Contains(t, string(d), `"removed":["len","list.map","string.repeat"]`)
	// The description builds the same removals.
	decl, err := risor.FromDescription(d)
	assert.NoError(t, err)
	e2, err := risor.NewEnv(decl)
	assert.NoError(t, err)
	d2, _ := e2.Describe()
	assert.Equal(t, string(d2), string(d))
	// A name that is no entry is bad-name.
	for _, n := range []string{"lenn", "repeat", "string.map", "number.abs", "list", "list.len"} {
		_, err = risor.NewEnv(risor.Without(n))
		var e *risor.Error
		assert.True(t, errors.As(err, &e), n)
		assert.Equal(t, e.Diagnostics[0].Code, "bad-name", n)
	}
}

func TestLambdaFactsWhereWritten(t *testing.T) {
	// The body sees the facts where the lambda was written, not those at the
	// call: a narrowing at the call does not reach it.
	env, err := risor.NewEnv(risor.VarRecord("{m: any, o: {n: string} | null}"))
	assert.NoError(t, err)
	ds := checkErr(t, env, "const f = () => o.n; o ? m.r(f) : 0")
	assert.Equal(t, ds[0].Code, "maybe-null")
	assert.Equal(t, ds[0].Span.Array(), [4]int{1, 17, 1, 18})
	// Written under the narrowing, the body checks clean.
	x, _ := risor.Parse("o ? { const f = () => o.n; m.r(f) } : 0")
	_, err = env.Check(x)
	assert.NoError(t, err)
}

func TestHostSiteOnTheFailingCallOnly(t *testing.T) {
	// The host site of rate("x") is on that call; geo.distance, not
	// marked !, returns an error: class environment, not host.
	rate := func(c *risor.Call, args []any) (any, error) { return "x", nil }
	far := func(c *risor.Call, args []any) (any, error) { return nil, errors.New("far") }
	env, err := risor.NewEnv(
		risor.Function{Name: "rate", Type: "(code: string) => string !", Impl: rate},
		risor.Function{Name: "geo.distance", Type: "(a: string, b: string) => float", Impl: far},
	)
	assert.NoError(t, err)
	x, _ := risor.Parse(`geo.distance(rate("x"), "b")`)
	r, err := env.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, len(r.Sites), 1)
	assert.Equal(t, r.Sites[0].Class, "host")
	p, err := env.Compile(x)
	assert.NoError(t, err)
	_, err = p.Run(context.Background(), nil)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "function-error")
	assert.Equal(t, e.Diagnostics[0].Class, "environment")
	// Unchecked, the class is host.
	p, err = env.Compile(x, risor.NoCheck)
	assert.NoError(t, err)
	_, err = p.Run(context.Background(), nil)
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Class, "host")
}

func TestChildEnvironmentsReplace(t *testing.T) {
	impl := func(c *risor.Call, args []any) (any, error) { return 1.0, nil }
	parent, err := risor.NewEnv(
		risor.Var("x", "int"),
		risor.Function{Name: "f", Type: "() => int", Impl: impl},
		risor.Function{Name: "geo.distance", Type: "() => float", Impl: impl},
		risor.OpaqueType{Name: "Time", Methods: map[string]risor.Function{"format": {Type: "(layout: string) => string", Impl: impl}}},
		risor.Without("len"),
		risor.Reject("value"),
		risor.Limits{Steps: 500, Depth: 50},
	)
	assert.NoError(t, err)
	describe := func(e *risor.Env) string {
		d, err := e.Describe()
		assert.NoError(t, err)
		return string(d)
	}
	// Replacement across kinds in one space of names: a function replaces a
	// variable, a variable replaces a function. A variable Time is in the
	// other space from the opaque type Time: both stay, and do not clash.
	child, err := parent.With(
		risor.Function{Name: "x", Type: "() => string", Impl: impl},
		risor.Var("f", "string"),
		risor.Var("Time", "bool"),
	)
	assert.NoError(t, err)
	d := describe(child)
	assert.Contains(t, d, `"functions":{"geo.distance":"() => float","x":"() => string"}`)
	assert.Contains(t, d, `"opaque":{"Time":{"format":"(layout: string) => string"}}`)
	assert.Contains(t, d, `"variables":{"Time":"bool","f":"string"}`)
	// Limits, rejected classes and removals are inherited unless given.
	assert.Contains(t, d, `"limits":{"allocation":1000000,"depth":50,"steps":500}`)
	assert.Contains(t, d, `"reject":["value"]`)
	assert.Contains(t, d, `"removed":["len"]`)
	// Removals add up and are not undone; a child's own limits and classes
	// replace the parent's.
	child2, err := child.With(risor.Without("abs"), risor.Reject(), risor.Limits{Steps: 9})
	assert.NoError(t, err)
	d = describe(child2)
	assert.Contains(t, d, `"removed":["abs","len"]`)
	assert.Contains(t, d, `"reject":[]`)
	assert.Contains(t, d, `"limits":{"allocation":1000000,"depth":50,"steps":9}`)
	// A clash with something inherited: a variable geo beside the inherited
	// function geo.distance.
	_, err = parent.With(risor.Var("geo", "int"))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "name-clash")
	// Two kinds of one name in one environment still clash.
	_, err = parent.With(risor.Var("y", "int"), risor.Function{Name: "y", Type: "() => int", Impl: impl})
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "name-clash")
}

func TestOpaqueTypeInChild(t *testing.T) {
	impl := func(c *risor.Call, args []any) (any, error) { return "", nil }
	parent, err := risor.NewEnv(risor.OpaqueType{Name: "Time", Methods: map[string]risor.Function{
		"format": {Type: "(layout: string) => string", Impl: impl},
		"year":   {Type: "() => int", Impl: impl},
	}})
	assert.NoError(t, err)
	opaque := func(e *risor.Env) string {
		d, err := e.Describe()
		assert.NoError(t, err)
		s := string(d)
		i := strings.Index(s, `"opaque":`)
		j := strings.Index(s, `,"reject"`)
		return s[i:j]
	}
	// Declared again in a child: replaced whole, the parent's methods gone.
	child, err := parent.With(risor.OpaqueType{Name: "Time", Methods: map[string]risor.Function{
		"unix": {Type: "() => int", Impl: impl},
	}})
	assert.NoError(t, err)
	assert.Equal(t, opaque(child), `"opaque":{"Time":{"unix":"() => int"}}`)
	// Only a method: added, or replacing the one of that name; the rest stay.
	child, err = parent.With(
		risor.Method("Time", "year", risor.Function{Type: "() => float", Impl: impl}),
		risor.Method("Time", "unix", risor.Function{Type: "() => int", Impl: impl}),
	)
	assert.NoError(t, err)
	assert.Equal(t, opaque(child), `"opaque":{"Time":{"format":"(layout: string) => string","unix":"() => int","year":"() => float"}}`)
	// The parent is unchanged.
	assert.Equal(t, opaque(parent), `"opaque":{"Time":{"format":"(layout: string) => string","year":"() => int"}}`)
}

type sz0 struct{ L, R int }
type sz1 struct{ L, R sz0 }
type sz2 struct{ L, R sz1 }
type sz3 struct{ L, R sz2 }
type sz4 struct{ L, R sz3 }
type sz5 struct{ L, R sz4 }
type sz6 struct{ L, R sz5 }
type sz7 struct{ L, R sz6 }
type sz8 struct{ L, R sz7 }
type sz9 struct{ L, R sz8 }
type sz10 struct{ L, R sz9 }
type sz11 struct{ L, R sz10 }
type sz12 struct{ L, R sz11 }
type sz13 struct{ L, R sz12 }
type sz14 struct{ L, R sz13 }
type sz15 struct{ L, R sz14 }
type sz30 struct{ A, B, C, D, E, F, G, H sz15 }

func TestTypeSizeLimit(t *testing.T) {
	// A Go type that uses one struct in many places is built once; over the
	// size limit it is bad-type.
	_, err := risor.TypeOf[sz14]()
	assert.NoError(t, err)
	_, err = risor.TypeOf[sz15]()
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
	start := time.Now()
	_, err = risor.TypeOf[sz30]()
	assert.True(t, errors.As(err, &e))
	assert.True(t, time.Since(start) < time.Second)

	// The doubling chain of 20.7: check-limit on the map literal of line 16,
	// at once.
	src := "const a0 = {l: 1, r: 1};\n"
	for k := 1; k <= 40; k++ {
		src += fmt.Sprintf("const a%d = {l: a%d, r: a%d};\n", k, k-1, k-1)
	}
	src += "[a40, a39]"
	env, _ := risor.NewEnv()
	start = time.Now()
	ds := checkErr(t, env, src)
	assert.True(t, time.Since(start) < time.Second)
	assert.Equal(t, ds[0].Code, "check-limit")
	assert.Equal(t, ds[0].Span.Array(), [4]int{16, 13, 16, 29})
}
