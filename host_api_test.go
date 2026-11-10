package risor_test

import (
	"context"
	"testing"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestLibraryNames(t *testing.T) {
	env, err := risor.NewEnv(risor.Groups("json"), risor.Without("len", "string.repeat"))
	assert.NoError(t, err)
	ln := env.LibraryNames()
	assert.True(t, contains(ln.Functions, "abs"))
	assert.False(t, contains(ln.Functions, "len"))
	assert.Equal(t, ln.Namespaces, []string{"json"})
	assert.Equal(t, ln.GroupFunctions, []string{"json.parse", "json.stringify"})
	assert.True(t, contains(ln.Methods["list"], "map"))
	assert.False(t, contains(ln.Methods["string"], "repeat"))
	assert.True(t, contains(ln.Methods["map"], "keys"))
	plain, _ := risor.NewEnv()
	assert.True(t, contains(plain.LibraryNames().Functions, "len"))
	assert.Equal(t, len(plain.LibraryNames().Namespaces), 0)
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func TestExprText(t *testing.T) {
	x, err := risor.Parse("  // note\n const a = 1;\n f(a + 1, `x${a}`, n => n) ")
	assert.NoError(t, err)
	assert.Equal(t, x.Text(), "const a = 1;\n f(a + 1, `x${a}`, n => n)")
	cs := x.Calls("f")
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].Args[0].Text(), "a + 1")
	assert.Equal(t, cs[0].Args[1].Text(), "`x${a}`")
	assert.Equal(t, cs[0].Args[0].Source(), x.Source())
	tp, err := risor.ParseTemplate("Hello ${{ user.name }}!")
	assert.NoError(t, err)
	assert.Equal(t, tp.Segments()[1].Expr.Text(), "user.name")
}

func TestSchemaOfGoValues(t *testing.T) {
	s := map[string]any{
		"type":     "object",
		"required": []string{"kind"},
		"properties": map[string]map[string]any{
			"kind": {"enum": []string{"a", "b"}},
			"n":    {"type": "integer", "minimum": 0},
		},
		"additionalProperties": false,
	}
	ty, err := risor.TypeFromSchema(s)
	assert.NoError(t, err)
	assert.Equal(t, ty.String(), `{kind: "a" | "b", n?: int}`)
	_, err = risor.TypeFromSchema(map[int]any{1: "x"})
	assert.Error(t, err)
	_, err = risor.TypeFromSchema(map[string]any{"type": make(chan int)})
	assert.Error(t, err)
	env, err := risor.NewEnv(risor.VarSchema("v", s))
	assert.NoError(t, err)
	x, _ := risor.Parse(`v.kind`)
	cr, err := env.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, cr.Type, `"a" | "b"`)
}

func TestChildEnvironments(t *testing.T) {
	parent, err := risor.NewEnv(risor.Declare("{a: int, t: Token}"), risor.OpaqueType{Name: "Token"},
		risor.NamedType("Pair", "{x: int, y: int}"))
	assert.NoError(t, err)
	pair, _ := parent.ParseType("Pair")
	// A child that adds variables shares the parent's types.
	child, err := parent.With(risor.VarType("p", pair), risor.Var("b", "string"))
	assert.NoError(t, err)
	x, _ := risor.Parse(`[a, p.x, b, t]`)
	cr, err := child.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, cr.Type, "(int | string | Token)[]")
	// A child that replaces a variable, and one that declares an opaque
	// type and a named type, make their types again.
	c2, err := parent.With(risor.Var("a", "string"))
	assert.NoError(t, err)
	x2, _ := risor.Parse(`a`)
	cr2, _ := c2.Check(x2)
	assert.Equal(t, cr2.Type, "string")
	c3, err := parent.With(risor.OpaqueType{Name: "Other"}, risor.NamedType("Pair", "{z: Other}"), risor.Var("q", "Pair"))
	assert.NoError(t, err)
	x3, _ := risor.Parse(`[q.z, t]`)
	cr3, err := c3.Check(x3)
	assert.NoError(t, err)
	assert.Equal(t, cr3.Type, "(Other | Token)[]")
	// The parent is unchanged, and a child runs.
	_, err = parent.Check(x)
	assert.Error(t, err)
	p, err := child.Compile(x)
	assert.NoError(t, err)
	v, err := p.Run(context.Background(), risor.Map{"a": 1.0, "p": map[string]any{"x": 2.0, "y": 3.0}, "b": "s", "t": nil})
	assert.NoError(t, err)
	assert.Equal(t, v, any([]any{1.0, 2.0, "s", nil}))
}
