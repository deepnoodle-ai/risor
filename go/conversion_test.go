package risor_test

import (
	"context"
	"errors"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F4: the binding surface the suite does not reach.

func conformErr(t *testing.T, v any, typ risor.Type) risor.Diagnostic {
	t.Helper()
	err := risor.Conform(v, typ)
	var e *risor.Error
	assert.True(t, errors.As(err, &e), "expected a failure")
	return e.Diagnostics[0]
}

func TestParseTypeAndConform(t *testing.T) {
	env, err := risor.NewEnv(
		risor.NamedType("Line", "{sku: string, qty: int}"),
		risor.NamedType("Order", "{id: string, lines: Line[], ...}"),
	)
	assert.NoError(t, err)
	order, err := env.ParseType("Order | null")
	assert.NoError(t, err)
	assert.Equal(t, order.String(), "{id: string, lines: {qty: int, sku: string}[], ...} | null")
	assert.NoError(t, risor.Conform(nil, order))
	assert.NoError(t, risor.Conform(map[string]any{"id": "a", "lines": []any{}, "x": 1}, order))
	d := conformErr(t, map[string]any{"id": "a", "lines": []any{map[string]any{"sku": "x", "qty": "2"}}}, order)
	assert.Equal(t, d.Phase, risor.PhaseRun)
	assert.Equal(t, d.Code, "not-conforming")
	assert.Equal(t, d.Class, "value")
	assert.True(t, d.Span == nil)
	assert.Equal(t, d.Path, []any{"lines", 0.0, "qty"})
	assert.Equal(t, d.Message, "expected int, found \"2\"")
	// A name the environment does not declare is bad-type.
	_, err = env.ParseType("Item")
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
}

type cell struct {
	ID   int    `json:"id"`
	Next *cell  `json:"next"`
	Tags []byte `json:"tags"`
}

func TestConformHostValues(t *testing.T) {
	env, _ := risor.NewEnv()
	typ, err := env.ParseType("{id: int, next: any, tags: int[]}")
	assert.NoError(t, err)
	// Go structs are read as records; cyclic data is not followed under any.
	c := &cell{ID: 1, Tags: []byte{1, 2}}
	c.Next = c
	assert.NoError(t, risor.Conform(c, typ))
	// A host number that cannot enter conforms to nothing.
	num, _ := env.ParseType("int")
	d := conformErr(t, int64(1)<<60, num)
	assert.Equal(t, d.Message, "expected int, found number")
	// A typed rest tests every other key and reports the least failing one.
	rest, _ := env.ParseType("{...int}")
	d = conformErr(t, map[string]any{"b": "x", "a": "y", "c": 1}, rest)
	assert.Equal(t, d.Path, []any{"a"})
	// Nothing is converted or changed.
	m := map[string]any{"n": 3.0}
	nint, _ := env.ParseType("{n: int}")
	assert.NoError(t, risor.Conform(m, nint))
	assert.Equal(t, m["n"], 3.0)
}

func TestNamedTypeDecls(t *testing.T) {
	ty, err := risor.TypeFromSchema([]byte(`{"type": "array", "items": {"type": "string"}}`))
	assert.NoError(t, err)
	env, err := risor.NewEnv(
		risor.Declare("{x: any}"),
		risor.NamedTypeSchema("Id", map[string]any{"type": "integer"}),
		risor.NamedTypeOf("Tags", ty),
		risor.NamedType("Doc", "{id: Id, tags: Tags}"),
	)
	assert.NoError(t, err)
	x, _ := risor.Parse("x as Doc")
	r, err := env.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, r.Type, "{id: int, tags: string[]}")
	assert.Equal(t, r.Sites[0].Class, "value")
	d, err := env.Describe()
	assert.NoError(t, err)
	assert.Contains(t, string(d), `"types":{"Doc":"{id: int, tags: string[]}","Id":"int","Tags":"string[]"}`)
	// The description builds the same named types.
	decl, err := risor.Description(d)
	assert.NoError(t, err)
	e2, err := risor.NewEnv(decl)
	assert.NoError(t, err)
	d2, _ := e2.Describe()
	assert.Equal(t, string(d2), string(d))
	// No named type, no key.
	plain, _ := risor.NewEnv()
	d, _ = plain.Describe()
	assert.NotContains(t, string(d), `"types"`)

	// Errors at declare.
	for code, decls := range map[string][]risor.Decl{
		"bad-name":   {risor.NamedType("T", "int")},
		"name-clash": {risor.NamedType("Ab", "int"), risor.NamedType("Ab", "int")},
		"bad-type":   {risor.NamedType("Ab", "Cd"), risor.NamedType("Cd", "int")},
	} {
		_, err := risor.NewEnv(decls...)
		var e *risor.Error
		assert.True(t, errors.As(err, &e), code)
		assert.Equal(t, e.Diagnostics[0].Code, code)
	}
	_, err = risor.NewEnv(risor.OpaqueType{Name: "Time"}, risor.NamedType("Time", "int"))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "name-clash")
}

func TestNamedTypesInChild(t *testing.T) {
	parent, err := risor.NewEnv(
		risor.Declare("{x: any}"),
		risor.OpaqueType{Name: "Time"},
		risor.NamedType("Id", "int"),
		risor.NamedType("Ref", "{id: Id}"),
	)
	assert.NoError(t, err)
	// The child's Id replaces the parent's in the child; Ref keeps the type
	// it had.
	child, err := parent.With(risor.NamedType("Id", "string"))
	assert.NoError(t, err)
	idT, _ := child.ParseType("Id")
	refT, _ := child.ParseType("Ref")
	assert.Equal(t, idT.String(), "string")
	assert.Equal(t, refT.String(), "{id: int}")
	// A parent's opaque name as a named type, and the reverse: name-clash.
	_, err = parent.With(risor.NamedType("Time", "int"))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "name-clash")
	_, err = parent.With(risor.OpaqueType{Name: "Id"})
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "name-clash")
}

func TestConversionWithoutCheck(t *testing.T) {
	// A compile with the static rules alone resolves program types and runs
	// the walk; an undefined name is still a static diagnostic.
	env, _ := risor.NewEnv(risor.Declare("{x: any}"), risor.NamedType("Pt", "{x: int, y: int}"))
	x, _ := risor.Parse("x is Pt")
	p, err := env.Compile(x, risor.NoCheck)
	assert.NoError(t, err)
	v, err := p.Run(context.Background(), risor.Map{"x": map[string]any{"x": 1, "y": 2}})
	assert.NoError(t, err)
	assert.Equal(t, v, true)
	x, _ = risor.Parse("x is Missing")
	_, err = env.Compile(x, risor.NoCheck)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "undefined-name")
}

func TestUntaggedUnionInnermost(t *testing.T) {
	// A parenthesized part that is itself reported gives no record to the
	// part around it (F4 6.1, confirmed).
	env, _ := risor.NewEnv(risor.Declare("{x: any}"))
	ds := checkErr(t, env, "x as ({a: int} | {b: int}) | {c: int}")
	assert.Equal(t, len(ds), 1)
	assert.Equal(t, ds[0].Code, "untagged-union")
	assert.Equal(t, ds[0].Span.Array(), [4]int{1, 7, 1, 26})
	// A parenthesized record that is not reported is one record outside.
	ds = checkErr(t, env, "x as ({a: int}) | {b: int}")
	assert.Equal(t, len(ds), 1)
	assert.Equal(t, ds[0].Span.Array(), [4]int{1, 6, 1, 27})
}

// A key may be any string, the empty one among them (05-types.md 2.2): it
// is never read as "no key".
func TestEmptyKey(t *testing.T) {
	env, err := risor.NewEnv(risor.Var("b", "any"))
	assert.NoError(t, err)
	run := func(src string, v any) (any, risor.Usage, error) {
		x, err := risor.Parse(src)
		assert.NoError(t, err)
		p, err := env.Compile(x, risor.NoCheck)
		assert.NoError(t, err)
		return p.Measure(context.Background(), risor.Map{"b": v})
	}
	_, u, err := run(`b as {"": string}`, map[string]any{})
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Message, `required field "" is missing`)
	assert.Equal(t, u.Steps, int64(2))
	v, _, err := run(`b is {"": null}`, map[string]any{"": nil})
	assert.NoError(t, err)
	assert.Equal(t, v, any(true))
	ty, err := env.ParseType(`{"": int}`)
	assert.NoError(t, err)
	ds := risor.ConformAll(map[string]any{"x": 1.0}, ty, 5)
	assert.Equal(t, len(ds), 2)
	assert.Equal(t, ds[0].Path, []any{""})
	// A named type with an empty key; its name has two characters, since
	// one upper-case letter is a type variable and so bad-name.
	_, err = risor.NewEnv(risor.Var("b", "any"), risor.NamedType("E", `{"": int}`))
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-name")
	env, err = risor.NewEnv(risor.Var("b", "any"), risor.NamedType("Em", `{"": int}`))
	assert.NoError(t, err)
	_, u, err = run(`b as Em`, map[string]any{"": "x"})
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Message, `expected int, found "x"`)
	assert.Equal(t, e.Diagnostics[0].Path, []any{""})
	assert.Equal(t, u.Steps, int64(2))
}
