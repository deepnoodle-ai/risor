package risor_test

import (
	"errors"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F14: host facts through the binding, and child environments,
// which the suite does not reach (f14-host-facts.md 2.6).

const factsEnv = "{a: {v: int} | null, b: {v: int} | null, c: {v: int} | null}"

func checkType(t *testing.T, env *risor.Env, src string) (string, string) {
	t.Helper()
	x, err := risor.Parse(src)
	assert.NoError(t, err)
	cr, err := env.Check(x)
	if err != nil {
		var e *risor.Error
		assert.True(t, errors.As(err, &e))
		return "", e.Diagnostics[0].Code
	}
	return cr.Type.String(), ""
}

func TestSomeNotNull(t *testing.T) {
	// A host fact may come before the variables it names.
	env, err := risor.NewEnv(risor.SomeNotNull("a", "b"), risor.VarRecord(factsEnv))
	assert.NoError(t, err)
	ty, _ := checkType(t, env, `(a ?? b).v`)
	assert.Equal(t, ty, "int")
	_, code := checkType(t, env, `(a ?? c).v`)
	assert.Equal(t, code, "maybe-null")
	d, err := env.Describe()
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(d), `{"facts":[{"someNotNull":["a","b"]}],`), string(d))
	// The round trip of a description.
	decl, err := risor.FromDescription(d)
	assert.NoError(t, err)
	env2, err := risor.NewEnv(decl)
	assert.NoError(t, err)
	d2, _ := env2.Describe()
	assert.Equal(t, string(d2), string(d))
}

func TestSomeNotNullErrors(t *testing.T) {
	for _, decl := range []risor.Decl{risor.SomeNotNull(), risor.SomeNotNull("a", "nope"), risor.SomeNotNull("a", "a"),
		risor.SomeNotNull("len"), risor.SomeNotNull("")} {
		_, err := risor.NewEnv(risor.VarRecord(factsEnv), decl)
		var e *risor.Error
		assert.True(t, errors.As(err, &e))
		assert.Equal(t, e.Diagnostics[0].Code, "bad-name")
		assert.Equal(t, e.Diagnostics[0].Phase, risor.PhaseDeclare)
	}
	// Each member that names nothing is one error.
	_, err := risor.NewEnv(risor.VarRecord(factsEnv), risor.SomeNotNull("x", "a", "y"))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, len(e.Diagnostics), 2)
}

func TestSomeNotNullChildren(t *testing.T) {
	parent, err := risor.NewEnv(risor.VarRecord(factsEnv), risor.SomeNotNull("a", "b"))
	assert.NoError(t, err)
	// A child keeps the parent's host fact.
	child, err := parent.With(risor.Var("d", "int"))
	assert.NoError(t, err)
	ty, _ := checkType(t, child, `(a ?? b).v`)
	assert.Equal(t, ty, "int")
	// A child that replaces a member drops the host fact whole.
	repl, err := parent.With(risor.Var("b", "{v: int} | null"))
	assert.NoError(t, err)
	_, code := checkType(t, repl, `(a ?? b).v`)
	assert.Equal(t, code, "maybe-null")
	d, _ := repl.Describe()
	assert.False(t, strings.Contains(string(d), "facts"))
	// A child's own host fact may name the variable it replaces, and the
	// parent's variables.
	own, err := parent.With(risor.Var("b", "{v: int} | null"), risor.SomeNotNull("b", "c"))
	assert.NoError(t, err)
	ty, _ = checkType(t, own, `(b ?? c).v`)
	assert.Equal(t, ty, "int")
	_, err = parent.With(risor.SomeNotNull("zz"))
	assert.Error(t, err)
	// The parent is unchanged.
	_, code = checkType(t, parent, `(b ?? c).v`)
	assert.Equal(t, code, "maybe-null")
	// A host fact of one member: that variable is not null.
	one, err := parent.With(risor.SomeNotNull("c"))
	assert.NoError(t, err)
	ty, _ = checkType(t, one, `c.v`)
	assert.Equal(t, ty, "int")
	d, _ = one.Describe()
	assert.True(t, strings.HasPrefix(string(d), `{"facts":[{"someNotNull":["a","b"]},{"someNotNull":["c"]}],`), string(d))
}

func TestSomeNotNullStaticCompile(t *testing.T) {
	// A compile with the static rules alone builds the host facts and
	// reports their errors at declare, and applies no rule of the checker.
	env, err := risor.NewEnv(risor.VarRecord(factsEnv), risor.SomeNotNull("a", "b"))
	assert.NoError(t, err)
	x, _ := risor.Parse(`(a ?? b).v`)
	_, err = env.Compile(x, risor.NoCheck)
	assert.NoError(t, err)
}
