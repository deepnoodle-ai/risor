package risor_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F6: the binding surface the suite does not reach.

func runIn(t *testing.T, env *risor.Env, src string, vars risor.Map) any {
	t.Helper()
	x, err := risor.Parse(src)
	assert.NoError(t, err)
	p, err := env.Compile(x)
	assert.NoError(t, err, src)
	v, err := p.Run(context.Background(), vars)
	assert.NoError(t, err, src)
	return v
}

func TestGroupsDecl(t *testing.T) {
	env, err := risor.NewEnv(risor.Groups("json", "math", "json"))
	assert.NoError(t, err)
	assert.Equal(t, runIn(t, env, `json.stringify({b: [1], a: math.pi})`, nil), `{"a":3.141592653589793,"b":[1]}`)
	// A child has its parent's groups and its own; removal is by full name
	// and adds up.
	child, err := env.With(risor.Groups("base64"), risor.Without("json.parse", "math.e"))
	assert.NoError(t, err)
	assert.Equal(t, runIn(t, child, `base64.decode(base64.encode("é"))`, nil), "é")
	ds := checkErr(t, child, `json.parse("1")`)
	assert.Equal(t, ds[0].Code, "undefined-function")
	ds = checkErr(t, child, `math.e`)
	assert.Equal(t, ds[0].Code, "not-a-value")
	d, err := child.Describe()
	assert.NoError(t, err)
	assert.Contains(t, string(d), `"groups":["base64","json","math"]`)
	assert.Contains(t, string(d), `"removed":["json.parse","math.e"]`)
	decl, err := risor.Description(d)
	assert.NoError(t, err)
	e2, err := risor.NewEnv(decl)
	assert.NoError(t, err)
	d2, _ := e2.Describe()
	assert.Equal(t, string(d2), string(d))
	// No group on, no key.
	plain, _ := risor.NewEnv()
	d, _ = plain.Describe()
	assert.NotContains(t, string(d), `"groups"`)

	// Errors at declare.
	for code, decls := range map[string][]risor.Decl{
		"bad-name":           {risor.Groups("core")},
		"name-clash":         {risor.Groups("time"), risor.Var("time", "float")},
		"reserved-namespace": {risor.Groups("json"), risor.Function{Name: "json.parse", Type: "(s: string) => any"}},
	} {
		_, err := risor.NewEnv(decls...)
		var e *risor.Error
		assert.True(t, errors.As(err, &e), code)
		assert.Equal(t, e.Diagnostics[0].Code, code, code)
	}
	// A variable named like a group that is off is a variable.
	off, err := risor.NewEnv(risor.Var("time", "float"))
	assert.NoError(t, err)
	assert.Equal(t, runIn(t, off, "time + 1", risor.Map{"time": 1.0}), 2.0)
}

type tok struct{}

func (tok) OpaqueType() string { return "Tok" }

func TestJSONText(t *testing.T) {
	s, err := risor.JSONText(map[string]any{"～": 1.0, "😀": []any{nil, true, "a\"b "}})
	assert.NoError(t, err)
	assert.Equal(t, s, "{\"～\":1,\"😀\":[null,true,\"a\\\"b \"]}")
	_, err = risor.JSONText([]any{tok{}})
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "wrong-kind")
	assert.True(t, e.Diagnostics[0].Span == nil)
	deep := any(1.0)
	for i := 0; i < 201; i++ {
		deep = []any{deep}
	}
	_, err = risor.JSONText(deep)
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "depth-exceeded")
}

func TestJSONParseDeepText(t *testing.T) {
	// The pass keeps its own stack: a text far past the depth limit is
	// not-json or depth-exceeded without using the host's stack.
	env, _ := risor.NewEnv(risor.Groups("json"), risor.Declare("{s: string}"), risor.Limits{Steps: risor.NoLimit})
	x, _ := risor.Parse(`try { json.parse(s) } catch (e) { e.code }`)
	p, err := env.Compile(x)
	assert.NoError(t, err)
	open := strings.Repeat("[", 1_000_000)
	v, err := p.Run(context.Background(), risor.Map{"s": open})
	assert.NoError(t, err)
	assert.Equal(t, v, "invalid-argument")
	_, err = p.Run(context.Background(), risor.Map{"s": open + strings.Repeat("]", 1_000_000)})
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "depth-exceeded")
}
