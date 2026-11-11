package risor_test

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F10: the load and the decode, through the binding.

func decodeFail(t *testing.T, v any, dst any) []risor.Diagnostic {
	t.Helper()
	err := risor.Decode(v, dst)
	var e *risor.Error
	assert.True(t, errors.As(err, &e), "expected a decode failure")
	for _, d := range e.Diagnostics {
		assert.Equal(t, d.Phase, risor.PhaseRun)
		assert.True(t, d.Span == nil)
	}
	return e.Diagnostics
}

func paths(ds []risor.Diagnostic) [][]any {
	var out [][]any
	for _, d := range ds {
		out = append(out, d.Path)
	}
	return out
}

type named struct {
	A int    `risor:"alpha" json:"ignored"`
	B string `json:"beta"`
	C bool
}

func TestDecodeNames(t *testing.T) {
	var n named
	assert.NoError(t, risor.Decode(map[string]any{"alpha": 1.0, "beta": "x", "C": true}, &n))
	assert.Equal(t, n, named{A: 1, B: "x", C: true})
	// Keys match exactly: no case folding.
	ds := decodeFail(t, map[string]any{"alpha": 1.0, "beta": "x", "c": true}, &named{})
	assert.Equal(t, paths(ds), [][]any{{"C"}, {"c"}})
	assert.Equal(t, ds[0].Message, `required field "C" is missing`)
	assert.Equal(t, ds[1].Message, `the type does not allow key "c"`)
	assert.Equal(t, ds[0].Code, "not-conforming")
	assert.Equal(t, ds[0].Class, "value")
}

type skipped struct {
	A int `json:"-"`
	B int `risor:"-"`
	C int
}

func TestDecodeSkippedFields(t *testing.T) {
	s := skipped{A: 7, B: 8}
	assert.NoError(t, risor.Decode(map[string]any{"C": 1.0}, &s))
	assert.Equal(t, s, skipped{A: 7, B: 8, C: 1})
	ds := decodeFail(t, map[string]any{"C": 1.0, "A": 2.0}, &skipped{})
	assert.Equal(t, paths(ds), [][]any{{"A"}})
}

type withDefaults struct {
	Port int    `json:"port,omitempty"`
	Host string `json:"host,omitzero"`
	Name string `json:"name"`
}

func TestDecodeOptionalKeepsDst(t *testing.T) {
	cfg := withDefaults{Port: 8080, Host: "localhost"}
	assert.NoError(t, risor.Decode(map[string]any{"name": "a", "port": 9.0}, &cfg))
	assert.Equal(t, cfg, withDefaults{Port: 9, Host: "localhost", Name: "a"})
	ds := decodeFail(t, map[string]any{"port": 1.0}, &withDefaults{})
	assert.Equal(t, paths(ds), [][]any{{"name"}})
}

type Inner struct {
	X int `json:"x"`
}

type outer struct {
	Inner
	*Ptr
	Y int `json:"y"`
}

type Ptr struct {
	Z int `json:"z"`
}

func TestDecodeEmbedded(t *testing.T) {
	var o outer
	assert.NoError(t, risor.Decode(map[string]any{"x": 1.0, "y": 2.0, "z": 3.0}, &o))
	assert.Equal(t, o.X, 1)
	assert.Equal(t, o.Y, 2)
	assert.True(t, o.Ptr != nil)
	assert.Equal(t, o.Z, 3)
	// A field through an embedded pointer is optional.
	var p outer
	assert.NoError(t, risor.Decode(map[string]any{"x": 1.0, "y": 2.0}, &p))
	assert.True(t, p.Ptr == nil)
}

type numbers struct {
	I8  int8    `json:"i8"`
	U   uint    `json:"u"`
	I   int     `json:"i"`
	F32 float32 `json:"f32"`
	F64 float64 `json:"f64"`
	Any any     `json:"any"`
}

func TestDecodeNumbers(t *testing.T) {
	var n numbers
	assert.NoError(t, risor.Decode(map[string]any{"i8": -128.0, "u": 3.0, "i": 1e15, "f32": 0.1, "f64": 0.1, "any": 2.0}, &n))
	assert.Equal(t, n, numbers{I8: -128, U: 3, I: 1e15, F32: float32(0.1), F64: 0.1, Any: 2.0})
	base := func() map[string]any {
		return map[string]any{"i8": 1.0, "u": 1.0, "i": 1.0, "f32": 1.0, "f64": 1.0, "any": nil}
	}
	for _, c := range []struct {
		key string
		v   any
		msg string
	}{
		{"i8", 300.0, "300 does not fit int8"},
		{"i8", 1.5, "expected int, found 1.5"},
		{"u", -1.0, "-1 does not fit uint"},
		{"f32", 1e39, "1e+39 does not fit float32"},
		{"i", "1", `expected int, found "1"`},
	} {
		v := base()
		v[c.key] = c.v
		ds := decodeFail(t, v, &numbers{})
		assert.Equal(t, len(ds), 1)
		assert.Equal(t, ds[0].Path, []any{c.key})
		assert.Equal(t, ds[0].Message, c.msg)
	}
	// float32 rounds to nearest; float64 is exact.
	var f float32
	assert.NoError(t, risor.Decode(16777217.0, &f))
	assert.Equal(t, f, float32(16777216))
}

type nullable struct {
	P *int  `json:"p"`
	A any   `json:"a"`
	N int   `json:"n"`
	S []int `json:"s"`
}

func TestDecodeNull(t *testing.T) {
	one := 1
	n := nullable{P: &one, A: "x", N: 5}
	ds := decodeFail(t, map[string]any{"p": nil, "a": nil, "n": nil, "s": nil}, &n)
	assert.Equal(t, paths(ds), [][]any{{"n"}, {"s"}})
	assert.Equal(t, ds[0].Message, "expected int, found null")
	var m nullable
	assert.NoError(t, risor.Decode(map[string]any{"p": nil, "a": nil, "n": 1.0, "s": []any{}}, &m))
	assert.True(t, m.P == nil)
	assert.True(t, m.A == nil)
	assert.Equal(t, m.S, []int{})
}

type rawJSON struct{ text string }

func (r *rawJSON) UnmarshalJSON(b []byte) error {
	if string(b) == `"bad"` {
		return errors.New("bad value")
	}
	r.text = string(b)
	return nil
}

type unmarshalers struct {
	When time.Time `json:"when"`
	Raw  rawJSON   `json:"raw"`
}

func TestDecodeUnmarshalers(t *testing.T) {
	var u unmarshalers
	assert.NoError(t, risor.Decode(map[string]any{"when": "2026-01-02T03:04:05Z", "raw": map[string]any{"b": 1.0, "a": "x"}}, &u))
	assert.Equal(t, u.When, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	assert.Equal(t, u.Raw.text, `{"a":"x","b":1}`)
	ds := decodeFail(t, map[string]any{"when": "yesterday", "raw": "bad"}, &unmarshalers{})
	assert.Equal(t, paths(ds), [][]any{{"raw"}, {"when"}})
	ds = decodeFail(t, map[string]any{"when": 1.0, "raw": 1.0}, &unmarshalers{})
	assert.Equal(t, ds[0].Path, []any{"when"})
	assert.Equal(t, ds[0].Message, "expected string, found 1")
}

func TestDecodeOpaque(t *testing.T) {
	var tok token
	ds := decodeFail(t, "x", &tok)
	assert.Equal(t, ds[0].Path, []any{})
	// An opaque value is never decoded, into any either.
	var a any
	ds = decodeFail(t, []any{token{1}}, &a)
	assert.Equal(t, ds[0].Path, []any{0.0})
}

func TestDecodeInterfaceIsPlainData(t *testing.T) {
	var a any
	src := map[string]any{"l": []any{1.0, "s", true, nil}, "m": map[string]any{"k": 2.0}}
	assert.NoError(t, risor.Decode(src, &a))
	assert.Equal(t, a, any(src))
	// It copies: changing the source does not change the result.
	src["l"].([]any)[0] = 9.0
	assert.Equal(t, a.(map[string]any)["l"].([]any)[0], any(1.0))
	var m map[string][]string
	assert.NoError(t, risor.Decode(map[string]any{"a": []any{"x"}}, &m))
	assert.Equal(t, m, map[string][]string{"a": {"x"}})
	var arr [2]int
	assert.NoError(t, risor.Decode([]any{1.0, 2.0}, &arr))
	assert.Equal(t, arr, [2]int{1, 2})
	ds := decodeFail(t, []any{1.0}, &arr)
	assert.Equal(t, ds[0].Path, []any{})
}

func TestDecodeFailures(t *testing.T) {
	// A host number that cannot enter the language.
	var f float64
	ds := decodeFail(t, math.NaN(), &f)
	assert.Equal(t, ds[0].Code, "number-range")
	// Nesting deeper than 200, under any.
	var deep any = 1.0
	for i := 0; i < 210; i++ {
		deep = []any{deep}
	}
	var a any
	ds = decodeFail(t, deep, &a)
	assert.Equal(t, ds[0].Code, "depth-exceeded")
	assert.True(t, ds[0].Path == nil)
	// A destination that is not a non-nil pointer.
	err := risor.Decode(1.0, f)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Phase, risor.PhaseDeclare)
	// Failures are reported in the walk's order, every one.
	ds = decodeFail(t, map[string]any{"beta": 1.0, "zz": 1.0}, &named{})
	assert.Equal(t, paths(ds), [][]any{{"C"}, {"alpha"}, {"beta"}, {"zz"}})
}

type appConfig struct {
	Listen  string   `json:"listen"`
	MaxBody int      `json:"max_body,omitempty"`
	Hosts   []string `json:"hosts"`
}

func loadErr(t *testing.T, err error) *risor.LoadError {
	t.Helper()
	var le *risor.LoadError
	assert.True(t, errors.As(err, &le), "expected a load failure, got %v", err)
	return le
}

func TestLoad(t *testing.T) {
	env, err := risor.NewEnv(risor.Declare(`{env: map<string>, raw: any}`))
	assert.NoError(t, err)
	vars := risor.Map{"env": map[string]any{"PORT": "9000"}, "raw": map[string]any{"listen": 1.0, "hosts": []any{}}}
	ctx := context.Background()
	cfg := appConfig{MaxBody: 1024}
	assert.NoError(t, risor.Load(ctx, env, "{listen: `:${env?.PORT ?? \"80\"}`, hosts: [\"a\"]}", vars, &cfg))
	assert.Equal(t, cfg, appConfig{Listen: ":9000", MaxBody: 1024, Hosts: []string{"a"}})

	for _, c := range []struct {
		src  string
		step int
		code string
	}{
		{"{listen: ", risor.StepParse, ""},
		{"{listen: 1, hosts: []}", risor.StepCheck, "result-mismatch"},
		{"{listen: \"a\", hosts: [], max_bod: 1}", risor.StepCheck, "result-mismatch"},
		{"{listen: \"a\", hosts: [1 / (len(env) - 1)]}", risor.StepCheck, ""},
		{"raw", risor.StepConform, "not-conforming"},
	} {
		le := loadErr(t, risor.Load(ctx, env, c.src, vars, &appConfig{}))
		assert.Equal(t, le.Step, c.step, c.src)
		if c.code != "" {
			assert.Equal(t, le.Diagnostics[0].Code, c.code, c.src)
		}
	}
	// Step 3: a run failure, with its span and usage.
	le := loadErr(t, risor.Load(ctx, env, `{listen: string(1 / (len(env?.X ?? "") - 0)), hosts: []}`, vars, &appConfig{}))
	assert.Equal(t, le.Step, risor.StepRun)
	assert.True(t, le.HasUsage)
	assert.True(t, le.Diagnostics[0].Span != nil)
	// Step 4 has no span, a path, the run's usage; MaxFailures reports more.
	le = loadErr(t, risor.Load(ctx, env, "raw", risor.Map{"env": map[string]any{}, "raw": map[string]any{"listen": 1.0, "x": 1.0}}, &appConfig{}, risor.MaxFailures(5)))
	assert.Equal(t, le.Step, risor.StepConform)
	assert.Equal(t, paths(le.Diagnostics), [][]any{{"hosts"}, {"listen"}, {"x"}})
	assert.True(t, le.Diagnostics[0].Span == nil)
	assert.True(t, le.HasUsage)
	// It unwraps to an *Error.
	var e *risor.Error
	assert.True(t, errors.As(error(le), &e))
}

type small struct {
	N int8 `json:"n"`
}

func TestLoadStepDecodeAndDeclare(t *testing.T) {
	ctx := context.Background()
	// Step 5: D(H) is {n: int}, and 300 does not fit int8.
	le := loadErr(t, risor.Load(ctx, nil, "{n: 300}", nil, &small{}))
	assert.Equal(t, le.Step, risor.StepDecode)
	assert.Equal(t, le.Diagnostics[0].Path, []any{"n"})
	// Step 0: a host type with no derived type.
	var ch chan int
	le = loadErr(t, risor.Load(ctx, nil, "1", nil, &ch))
	assert.Equal(t, le.Step, risor.StepDeclare)
	le2 := loadErr(t, func() error { _, _, err := risor.LoadValue(ctx, nil, "1", nil, risor.LoadType("Nope")); return err }())
	assert.Equal(t, le2.Step, risor.StepDeclare)
}

func TestLoadValue(t *testing.T) {
	ctx := context.Background()
	schema := map[string]any{"type": "object", "properties": map[string]any{"port": map[string]any{"type": "integer"}}, "required": []any{"port"}}
	v, u, err := risor.LoadValue(ctx, nil, "{port: 2 * 4}", nil, risor.LoadSchema(schema))
	assert.NoError(t, err)
	assert.Equal(t, v, any(map[string]any{"port": 8.0}))
	assert.Equal(t, u.Steps, int64(1))
	// A schema is mapped with closed: a misspelled key fails at the check.
	_, _, err = risor.LoadValue(ctx, nil, "{port: 1, prot: 2}", nil, risor.LoadSchema(schema))
	le := loadErr(t, err)
	assert.Equal(t, le.Step, risor.StepCheck)
	assert.True(t, strings.Contains(le.Diagnostics[0].Message, `does not allow key "prot"`))
	// With no expected type, no step 4.
	v, _, err = risor.LoadValue(ctx, nil, "[1]", nil)
	assert.NoError(t, err)
	assert.Equal(t, v, any([]any{1.0}))
	_, _, err = risor.LoadValue(ctx, nil, "1", nil, risor.LoadType("string"))
	assert.Equal(t, loadErr(t, err).Step, risor.StepCheck)
}

func TestExpectSchemaOptions(t *testing.T) {
	env, _ := risor.NewEnv()
	x, _ := risor.Parse("{a: 1}")
	_, err := env.Compile(x, risor.ExpectSchema(map[string]any{"type": "object"}, risor.Closed, risor.AbsentIsNull))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
	// A compile without a check still maps the schema.
	_, err = env.Compile(x, risor.NoCheck, risor.ExpectSchema(map[string]any{"type": 1}))
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
}
