package risor_test

import (
	"context"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F7: what the suite does not reach.

func TestProjectionAPI(t *testing.T) {
	// The star step in the analysis of the Go API.
	x, err := risor.Parse("orders[*].lines[*].qty.sum()")
	assert.NoError(t, err)
	refs := x.Refs()
	assert.Equal(t, refs[0].Path, []risor.Step{{Star: true}, {Key: "lines"}, {Star: true}, {Key: "qty"}})
	x, _ = risor.Parse(`orders["*"]`)
	assert.Equal(t, x.Refs()[0].Path, []risor.Step{{Key: "*"}})

	// A host list of structs, projected.
	type line struct {
		SKU string `json:"sku"`
		Qty int    `json:"qty"`
	}
	env, err := risor.NewEnv(risor.Declare("{lines: {sku: string, qty: int}[]}"))
	assert.NoError(t, err)
	assert.Equal(t, runIn(t, env, "lines[*].qty.sum()", risor.Map{"lines": []line{{"a", 2}, {"b", 3}}}), 5.0)
}

func TestHundredStars(t *testing.T) {
	// 100 stars parse and run; the 101st star's [ is nesting-exceeded.
	src := "x" + strings.Repeat("[*]", 100)
	env, _ := risor.NewEnv(risor.Declare("{x: any}"))
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
