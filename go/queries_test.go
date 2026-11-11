package risor_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F7: what the suite does not reach.

func TestProjectionAPI(t *testing.T) {
	// The star step in the analysis of the Go API.
	x, err := risor.Parse("orders[*].lines[*].qty.sum()")
	assert.NoError(t, err)
	refs := x.Analyze().Refs
	assert.Equal(t, refs[0].Path, []risor.Step{{Star: true}, {Key: "lines"}, {Star: true}, {Key: "qty"}})
	x, _ = risor.Parse(`orders["*"]`)
	assert.Equal(t, x.Analyze().Refs[0].Path, []risor.Step{{Key: "*"}})

	// A host list of structs, projected.
	type line struct {
		SKU string `json:"sku"`
		Qty int    `json:"qty"`
	}
	env, err := risor.NewEnv(risor.VarRecord("{lines: {sku: string, qty: int}[]}"))
	assert.NoError(t, err)
	assert.Equal(t, runIn(t, env, "lines[*].qty.sum()", risor.Map{"lines": []line{{"a", 2}, {"b", 3}}}), 5.0)
}

func TestHundredStars(t *testing.T) {
	// 100 stars parse and run; the 101st star's [ is nesting-exceeded.
	src := "x" + strings.Repeat("[*]", 100)
	env, _ := risor.NewEnv(risor.VarRecord("{x: any}"))
	v := any(1.0)
	for i := 0; i < 100; i++ {
		v = []any{v}
	}
	x, err := risor.Parse(src)
	assert.NoError(t, err)
	p, err := env.Compile(x, risor.NoCheck)
	assert.NoError(t, err)
	out, err := p.Run(context.Background(), risor.Map{"x": v})
	assert.NoError(t, err)
	assert.Equal(t, out, []any{1.0})
	_, err = risor.Parse(src + "[*]")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "nesting-exceeded")
}

func TestProjectionReadFailureClass(t *testing.T) {
	// A read's failure is at the site on its receiver, the chain up to the
	// star (f07-queries.md 2.4 item 2, 5.3 item 2): dynamic, whether the
	// list is any or a list of any. A later step's receiver is the read
	// before it.
	for _, c := range []struct{ decl, src, span string }{
		{"{xs: any[]}", "xs[*].tags", "[1, 1, 1, 11]"},
		{"{xs: any}", "xs[*].tags", "[1, 1, 1, 11]"},
		{"{xs: any[]}", "xs[*].a.b", "[1, 1, 1, 10]"},
		{"{xs: {a: any}[]}", "xs[*].a.b", "[1, 1, 1, 10]"},
	} {
		env, err := risor.NewEnv(risor.VarRecord(c.decl))
		assert.NoError(t, err)
		x, err := risor.Parse(c.src)
		assert.NoError(t, err)
		p, err := env.Compile(x)
		assert.NoError(t, err)
		v := []any{true}
		if strings.Contains(c.src, ".a.") {
			v = []any{map[string]any{"a": true}}
		}
		_, err = p.Run(context.Background(), risor.Map{"xs": v})
		var e *risor.Error
		assert.True(t, errors.As(err, &e), c.decl, c.src)
		d := e.Diagnostics[0]
		assert.Equal(t, d.Code, "wrong-kind", c.decl, c.src)
		assert.Equal(t, d.Span.String(), c.span, c.decl, c.src)
		assert.Equal(t, d.Class, "dynamic", c.decl, c.src)
	}
}

func TestMethodOnNeverList(t *testing.T) {
	// A projection of a field whose type is never is never[] (f07-queries.md
	// 5.2 item 2), a list: a method call on it is checked (05-types.md 13.3).
	env, err := risor.NewEnv()
	assert.NoError(t, err)
	for _, src := range []string{
		`[{"tags": zz}][*].tags.every()`,
		`[{a: zz}][*].a[*].every()`,
		`[[zz]][*][*].every()`,
		`[zz].every()`,
	} {
		x, err := risor.Parse(src)
		assert.NoError(t, err)
		_, err = env.Check(x)
		var e *risor.Error
		assert.True(t, errors.As(err, &e), src)
		codes := []string{}
		for _, d := range e.Diagnostics {
			codes = append(codes, d.Code)
		}
		assert.Equal(t, codes, []string{"arity", "undefined-name"}, src)
	}
	// A read's diagnostic still makes the projection never: nothing more.
	x, _ := risor.Parse(`[{a: 1}][*].b.every()`)
	_, err = env.Check(x)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, len(e.Diagnostics), 1)
	assert.Equal(t, e.Diagnostics[0].Code, "no-field")
}
