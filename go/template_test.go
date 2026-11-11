package risor_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F2: the binding surface the suite does not reach.

func TestTemplateSurface(t *testing.T) {
	tm, err := risor.ParseTemplate("Hi ${{ name }}, $${{ x }}${{ n + 1 }}")
	assert.NoError(t, err)
	segs := tm.Segments()
	assert.Equal(t, len(segs), 4)
	assert.Equal(t, segs[0].Text, "Hi ")
	assert.True(t, segs[0].Expr == nil)
	assert.True(t, segs[1].Expr != nil)
	assert.Equal(t, segs[1].Span.String(), "[1, 4, 1, 15]")
	assert.Equal(t, segs[1].ExprSpan.String(), "[1, 8, 1, 12]")
	assert.Equal(t, segs[2].Text, ", ${{ x }}")
	assert.False(t, tm.IsSingle())
	assert.True(t, tm.Program(risor.ModeValue) == tm.Program(risor.ModeString))
	assert.True(t, tm.Part(1) == segs[3].Expr)

	env, err := risor.NewEnv(risor.VarRecord("{name: string, n: int}"))
	assert.NoError(t, err)
	p, err := env.Compile(tm.Program(risor.ModeString))
	assert.NoError(t, err)
	v, err := p.Run(context.Background(), risor.Map{"name": "Ann", "n": 1})
	assert.NoError(t, err)
	assert.Equal(t, v, "Hi Ann, ${{ x }}2")

	// A part given alone compiles and runs.
	p, err = env.Compile(segs[3].Expr)
	assert.NoError(t, err)
	v, err = p.Run(context.Background(), risor.Map{"name": "Ann", "n": 1})
	assert.NoError(t, err)
	f, _ := risor.AsFloat(v)
	assert.Equal(t, f, 2.0)

	// A single template keeps the value's type in mode value.
	single, err := risor.ParseTemplate("${{ [n, n] }}")
	assert.NoError(t, err)
	assert.True(t, single.IsSingle())
	r, err := env.Check(single.Program(risor.ModeValue))
	assert.NoError(t, err)
	assert.Equal(t, r.Type.String(), "int[]")
	r, err = env.Check(single.Program(risor.ModeString))
	assert.NoError(t, err)
	assert.Equal(t, r.Type.String(), "string")

	// A parse failure is an *Error at a position in the whole text.
	_, err = risor.ParseTemplate("line\n${{ }}")
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "unexpected-token")
	assert.Equal(t, e.Diagnostics[0].Span.String(), "[2, 5, 2, 7]")
}

func TestAnalysisSurface(t *testing.T) {
	x, err := risor.Parse(`const k = 1; geo.distance(a.loc["lat"], k) + rate(b) + xs.map(x => rate(x))`)
	assert.NoError(t, err)
	assert.Equal(t, x.Analyze().Names, []string{"a", "b", "geo", "xs"})
	refs := x.Analyze().Refs
	assert.Equal(t, refs[1].Name, "a")
	assert.Equal(t, refs[1].Path, []risor.Step{{Key: "loc"}, {Key: "lat"}})
	calls := x.Analyze().Calls
	assert.Equal(t, len(calls), 4)
	assert.Equal(t, calls[0].Name, "geo.distance")
	assert.Equal(t, calls[0].Namespace, "geo")
	rates := []risor.CallSite{calls[1], calls[3]}
	assert.Equal(t, rates[0].Name, "rate")
	assert.Equal(t, rates[1].Name, "rate")
	assert.Equal(t, len(rates), 2)
	assert.False(t, rates[0].InLambda)
	assert.True(t, rates[1].InLambda)

	// Each argument is a program of its own, with spans in the whole text.
	arg := calls[0].Args[0]
	assert.Equal(t, calls[0].ArgSpans[0].String(), "[1, 27, 1, 39]")
	assert.Equal(t, arg.Analyze().Names, []string{"a"})
	env, _ := risor.NewEnv(risor.VarRecord(`{a: {loc: {lat: float}}}`))
	r, err := env.Check(arg)
	assert.NoError(t, err)
	assert.Equal(t, r.Type.String(), "float")
	// k is bound by the program, so its argument names it as free.
	assert.Equal(t, calls[0].Args[1].Analyze().Names, []string{"k"})
	// A lambda argument is not a program.
	xsMap := calls[2]
	assert.Equal(t, xsMap.Name, "xs.map")
	assert.True(t, xsMap.Args[0] == nil)

	xa := x.Analyze()
	lit, ok := xa.Literal, xa.IsLiteral
	assert.False(t, ok)
	assert.Nil(t, lit)
	y, _ := risor.Parse(`{a: [1, -2], b: "s", a: null}`)
	ya := y.Analyze()
	lit, ok = ya.Literal, ya.IsLiteral
	assert.True(t, ok)
	assert.Equal(t, lit, map[string]any{"a": nil, "b": "s"})

	// A program's analysis has no segments; a template's has them.
	assert.True(t, x.Analyze().Segments == nil)
	tm, _ := risor.ParseTemplate("")
	a := tm.Analyze(risor.ModeString)
	assert.True(t, a.Template)
	assert.Equal(t, len(a.Segments), 0)
	assert.True(t, a.IsLiteral)
	assert.Equal(t, a.Literal, "")
}

func TestAnalysisDeepTrees(t *testing.T) {
	// Chains and operator runs open no level, so analysis must not use Go
	// stack for them.
	long := "a" + strings.Repeat(".b", 200000)
	x, err := risor.Parse(long)
	assert.NoError(t, err)
	refs := x.Analyze().Refs
	assert.Equal(t, len(refs), 1)
	assert.Equal(t, len(refs[0].Path), 200000)

	sum := "a" + strings.Repeat(" + a", 200000)
	x, err = risor.Parse(sum)
	assert.NoError(t, err)
	assert.Equal(t, len(x.Analyze().Refs), 200001)
	assert.Equal(t, x.Analyze().Names, []string{"a"})
}
