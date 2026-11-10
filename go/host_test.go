package risor_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// compile compiles without the type rules: these tests are of the run.
func compile(t *testing.T, env *risor.Env, src string) *risor.Program {
	t.Helper()
	x, err := risor.Parse(src)
	assert.NoError(t, err)
	p, err := env.Compile(x, risor.NoCheck)
	assert.NoError(t, err, src)
	return p
}

func runErr(t *testing.T, env *risor.Env, src string, vars risor.Vars) risor.Diagnostic {
	t.Helper()
	_, err := compile(t, env, src).Run(context.Background(), vars)
	var e *risor.Error
	assert.True(t, errors.As(err, &e), "%s: expected a run failure, got %v", src, err)
	return e.Diagnostics[0]
}

func run(t *testing.T, env *risor.Env, src string, vars risor.Vars) any {
	t.Helper()
	v, err := compile(t, env, src).Run(context.Background(), vars)
	assert.NoError(t, err, src)
	return v
}

type token struct{ id int }

func (token) OpaqueType() string { return "Token" }

func TestDeadlineAndCancel(t *testing.T) {
	env, err := risor.NewEnv(risor.Declare("{xs: int[]}"),
		risor.Limits{Steps: risor.NoLimit, Alloc: risor.NoLimit, Timeout: time.Millisecond})
	assert.NoError(t, err)
	xs := make([]any, 3000)
	for i := range xs {
		xs[i] = i
	}
	d := runErr(t, env, "xs.map(x => xs.map(y => y + x))", risor.Map{"xs": xs})
	assert.Equal(t, d.Code, "deadline-exceeded")
	assert.Equal(t, d.Class, "limit")

	// Cancellation, also inside a try, which does not catch it.
	env2, _ := risor.NewEnv(risor.Declare("{xs: int[]}"), risor.Limits{Steps: risor.NoLimit, Alloc: risor.NoLimit})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = compile(t, env2, "try { xs.map(x => x) } catch { 0 }").Run(ctx, risor.Map{"xs": xs})
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "cancelled")
	ctx, cancel = context.WithCancel(context.Background())
	go func() { time.Sleep(time.Millisecond); cancel() }()
	_, err = compile(t, env2, "try { xs.map(x => xs.map(y => y)) } catch { 0 }").Run(ctx, risor.Map{"xs": xs})
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "cancelled")
}

func TestHostFunctions(t *testing.T) {
	env, err := risor.NewEnv(
		risor.Function{Name: "boom", Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
			panic("bug")
		}},
		risor.Function{Name: "oops", Type: "() => int !", Impl: func(c *risor.Call, args []any) (any, error) {
			return nil, errors.New("no stock")
		}},
		risor.Function{Name: "costly", Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
			if err := c.Charge(3); err != nil {
				return nil, err
			}
			return int64(1) << 60, c.Allocate(2)
		}},
		risor.Function{Name: "geo.dist", Type: "(a: float, b?: float) => float", Impl: func(c *risor.Call, args []any) (any, error) {
			return float32(len(args)), nil
		}},
	)
	assert.NoError(t, err)
	d := runErr(t, env, "try { boom() } catch { 0 }", nil)
	assert.Equal(t, d.Code, "function-panic")
	assert.Equal(t, d.Class, "host")
	assert.Equal(t, d.Message, "host function boom crashed")
	assert.Equal(t, run(t, env, "try { oops() } catch (e) { e.message }", nil), any("no stock"))
	d = runErr(t, env, "oops()", nil)
	assert.Equal(t, d.Code, "function-error")
	assert.Equal(t, d.Message, "no stock")
	// A host integer over 2^53 - 1 cannot enter: number-range, environment.
	_, u, err := compile(t, env, "costly()").Measure(context.Background(), nil)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "number-range")
	assert.Equal(t, e.Diagnostics[0].Class, "environment")
	assert.Equal(t, u, risor.Usage{Steps: 4, Alloc: 2})
	assert.Equal(t, run(t, env, "geo.dist(1) + geo.dist(1, 2)", nil), any(3.0))
}

func TestHostNumbers(t *testing.T) {
	env, _ := risor.NewEnv(risor.Declare("{x: int, xs: int[], m: map<int>}"))
	big := int64(1) << 60
	vars := risor.Map{"x": big, "xs": []int64{1, big}, "m": map[string]uint64{"a": 2, "b": uint64(big)}}
	for _, src := range []string{"x", "x + 1", "xs[1]", "m.b", "xs.map(v => v)", "`${xs}`", "xs == xs"} {
		d := runErr(t, env, src, vars)
		assert.Equal(t, d.Code, "number-range", src)
		assert.Equal(t, d.Class, "environment", src)
		assert.Equal(t, d.Message, "a number from the host is out of range", src)
	}
	// What does not read the element does not fail (02-evaluation.md 3.3).
	assert.Equal(t, run(t, env, "[xs[0], m.a, len(xs)]", vars), any([]any{1.0, 2.0, 2.0}))
	v := run(t, env, "[...xs]", vars).([]any)
	assert.Equal(t, v[1], any(big))
	// Typed host values read in place.
	assert.Equal(t, run(t, env, "xs[0] + m.a", vars), any(3.0))
	// A finite host float of any size enters.
	assert.Equal(t, run(t, env, "x + 1", risor.Map{"x": 1e20}), any(1e20))
	// -0.0 from the host reads as zero.
	negZero := -1 / (1e308 * 1e308)
	assert.Equal(t, run(t, env, "`${x}`", risor.Map{"x": negZero}), any("0"))
}

func TestOpaqueValues(t *testing.T) {
	env, err := risor.NewEnv(
		risor.Declare("{t: Token, u: Token}"),
		risor.OpaqueType{Name: "Token", Methods: map[string]risor.Function{
			"id": {Type: "() => int", Impl: func(c *risor.Call, args []any) (any, error) {
				return args[0].(token).id, nil
			}},
			"add": {Type: "(n: int) => int", Impl: func(c *risor.Call, args []any) (any, error) {
				return float64(args[0].(token).id) + args[1].(float64), nil
			}},
		}},
		risor.Function{Name: "same", Type: "(a: Token) => Token", Impl: func(c *risor.Call, args []any) (any, error) {
			return args[0], nil
		}},
	)
	assert.NoError(t, err)
	vars := risor.Map{"t": token{1}, "u": token{2}}
	cases := map[string]any{
		"if (t) { 1 } else { 2 }": 1.0,
		"!t":                      false,
		"t && 5":                  5.0,
		"t ? 1 : 2":               1.0,
		"t == same(t)":            true,
		"t == u":                  false,
		"t != u":                  true,
		"[t, u][1] == u":          true,
		"{a: t}.a == t":           true,
		"t in [u, t]":             true,
		"type(t)":                 "opaque",
		"t.id() + u.id()":         3.0,
		"t.add(10)":               11.0,
		"bool(t)":                 true,
		"t ?? 1":                  token{1},
	}
	for src, want := range cases {
		assert.Equal(t, run(t, env, src, vars), want, src)
	}
	for src, tmpl := range map[string]string{
		"`${t}`":         "an opaque value cannot be rendered as text",
		"string([1, t])": "an opaque value cannot be rendered as text",
		"t.id(x => x)":   "1 is not a count of arguments that method id takes", "t.add(x => x)": "argument 1 of method add cannot be a function",
		"t.nope()":             "a value of kind opaque has no method nope",
		"t.id(1)":              "1 is not a count of arguments that method id takes",
		"t.a":                  "cannot read field a of a value of kind opaque",
		"t[0]":                 "cannot index a value of kind opaque",
		"t + 1":                "cannot apply + to opaque and number",
		"t < u":                "cannot apply < to opaque and opaque",
		"\"a\" in t":           "cannot apply in to string and opaque",
		"[...t]":               "only a list can be spread into a list, found opaque",
		"{...t}":               "only a map can be spread into a map, found opaque",
		"{ const [a] = t; a }": "a list pattern needs a list, found opaque",
		"len(t)":               "len cannot take a value of kind opaque",
	} {
		d := runErr(t, env, src, vars)
		assert.Equal(t, d.Code, "wrong-kind", src)
		assert.Equal(t, d.Message, tmpl, src)
	}
}

// 02-evaluation.md 8.4 item 5: an empty pair is entered by ==, and an empty list by a
// rendering walk, and the depth test is made for it.
func TestEmptyContainersAtTheDepthLimit(t *testing.T) {
	for _, c := range []struct {
		src   string
		depth int
		ok    bool
	}{
		{"[[]] == [[]]", 2, true},
		{"[[]] == [[]]", 1, false},
		{"{a: {}} == {a: {}}", 2, true},
		{"{a: {}} == {a: {}}", 1, false},
		{"`${[[]]}`", 2, true},
		{"`${[[]]}`", 1, false},
		{"[] == []", 1, true},
	} {
		env, _ := risor.NewEnv(risor.Limits{Depth: c.depth})
		_, err := compile(t, env, c.src).Run(context.Background(), nil)
		if c.ok {
			assert.NoError(t, err, "%s at depth %d", c.src, c.depth)
			continue
		}
		var e *risor.Error
		assert.True(t, errors.As(err, &e), c.src)
		assert.Equal(t, e.Diagnostics[0].Code, "depth-exceeded", c.src)
	}
}

func TestInvalidText(t *testing.T) {
	env, _ := risor.NewEnv(risor.Declare("{s: string}"))
	// E2 82 41: one U+FFFD for the maximal subpart E2 82, then A (03-host.md 3.4.5).
	assert.Equal(t, run(t, env, "[len(s), s]", risor.Map{"s": "\xe2\x82A\xff"}), any([]any{3.0, "\ufffdA\ufffd"}))
	assert.Equal(t, run(t, env, "s == \"\\uFFFD\"", risor.Map{"s": "\xf0\x9f\x98"}), any(true))
	x, err := risor.Parse("\"\xe2\x82A\"")
	assert.NoError(t, err)
	p, _ := env.Compile(x)
	v, _ := p.Run(context.Background(), nil)
	assert.Equal(t, v, any("\ufffdA"))
}

// What opens no level costs no stack: these long flat programs run
// (01-language.md 13.2 rule 6).
func TestFlatPrograms(t *testing.T) {
	n := 100000
	env, _ := risor.NewEnv(risor.Declare("{c: bool, m: map<any>}"))
	var consts strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&consts, "const a%d = %d;\n", i, i)
	}
	consts.WriteString("a99999")
	cases := map[string]any{
		strings.Repeat("1 + ", n) + "1":                                          float64(n + 1),
		"m" + strings.Repeat("?.a", n):                                           nil,
		strings.Repeat("c ? 1 : ", n) + "2":                                      2.0,
		"if (c) { 1 }" + strings.Repeat(" else if (c) { 1 }", n) + " else { 2 }": 2.0,
		consts.String():                     99999.0,
		strings.Repeat("null ?? ", n) + "3": 3.0,
	}
	for src, want := range cases {
		x, err := risor.Parse(src)
		assert.NoError(t, err)
		p, err := env.Compile(x)
		assert.NoError(t, err)
		v, err := p.Run(context.Background(), risor.Map{"c": false, "m": map[string]any{}})
		assert.NoError(t, err)
		assert.Equal(t, v, want)
	}
}

// StackNeeds documents and tests the stack this binding needs:
// to parse at the nesting limit, and to run at the depth limits 200 and
// 1,000. Go grows a goroutine's stack as needed; the test caps it at
// stackCap to show that the deepest programs fit.
const stackCap = 1 << 20

func TestStackNeeds(t *testing.T) {
	old := debug.SetMaxStack(stackCap)
	defer debug.SetMaxStack(old)
	// Parse at the nesting limit: 100 levels of every kind.
	deep := strings.Repeat("[(-{a: ", 25) + "1" + strings.Repeat("})]", 25)
	_, err := risor.Parse(deep)
	assert.NoError(t, err)

	for _, depth := range []int{200, 1000} {
		env, _ := risor.NewEnv(risor.Declare("{v: any, xs: int[]}"), risor.Limits{Depth: depth, Steps: risor.NoLimit, Alloc: risor.NoLimit})
		// A value nested to the depth limit, compared and rendered.
		var v any = []any{}
		for i := 1; i < depth; i++ {
			v = []any{v}
		}
		assert.Equal(t, run(t, env, "v == v", risor.Map{"v": v}), any(true))
		_ = run(t, env, "`${v}`", risor.Map{"v": v})
		d := runErr(t, env, "`${[v]}`", risor.Map{"v": v})
		assert.Equal(t, d.Code, "depth-exceeded")
		// Calls in progress past the limit: const-bound lambdas, each calling
		// the one before inside 98 brackets, so each call adds 99.
		var b strings.Builder
		b.WriteString("const f0 = x => x;\n")
		k := depth/99 + 2
		for i := 1; i <= k; i++ {
			fmt.Fprintf(&b, "const f%d = x => %sf%d(x)%s;\n", i, strings.Repeat("[", 98), i-1, strings.Repeat("]", 98))
		}
		fmt.Fprintf(&b, "f%d(1)", k)
		d = runErr(t, env, b.String(), nil)
		assert.Equal(t, d.Code, "depth-exceeded")
	}
}

// 02-evaluation.md 5.3 item 3: map == visits the left map's keys in order when a value is a
// list or a map, so the deep pair under the first key meets the depth limit
// before the long string under the last key passes the step limit.
func TestMapEqualKeyOrder(t *testing.T) {
	env, _ := risor.NewEnv(risor.Declare("{m: any, n: any}"), risor.Limits{Depth: 5, Steps: 10})
	deep := func() any {
		var v any = []any{1}
		for i := 0; i < 10; i++ {
			v = []any{v}
		}
		return v
	}
	long := strings.Repeat("a", 6400)
	for i := 0; i < 20; i++ { // Go's map order varies from run to run
		m := map[string]any{"a": deep(), "z": long}
		n := map[string]any{"a": deep(), "z": long}
		d := runErr(t, env, "m == n", risor.Map{"m": m, "n": n})
		assert.Equal(t, d.Code, "depth-exceeded")
	}
}

func TestBadLimit(t *testing.T) {
	for _, l := range []risor.Limits{{Depth: 1001}, {Depth: -1}, {Steps: -5}, {Alloc: -9}} {
		_, err := risor.NewEnv(l)
		var e *risor.Error
		assert.True(t, errors.As(err, &e))
		assert.Equal(t, e.Diagnostics[0].Code, "bad-limit")
	}
}

func TestDeclare(t *testing.T) {
	for _, c := range []struct {
		decls []risor.Decl
		code  string
	}{
		{[]risor.Decl{risor.Declare("int")}, "bad-type"},
		{[]risor.Decl{risor.Declare("{a?: int}")}, "bad-type"},
		{[]risor.Decl{risor.Declare("{a: int, ...}")}, "bad-type"},
		{[]risor.Decl{risor.Declare("{a: int, a: int}")}, "name-clash"},
		{[]risor.Decl{risor.Declare(`{"a-b": int}`)}, "bad-name"},
		{[]risor.Decl{risor.Declare(`{if: int}`)}, "bad-name"},
		{[]risor.Decl{risor.Function{Name: "len", Type: "() => int"}}, "name-clash"},
		{[]risor.Decl{risor.Function{Name: "json.f", Type: "() => int"}}, "reserved-namespace"},
		{[]risor.Decl{risor.Function{Name: "a.b.c", Type: "() => int"}}, "bad-name"},
		{[]risor.Decl{risor.Function{Name: "f", Type: "(f: (x: int) => int) => int"}}, "bad-signature"},
		{[]risor.Decl{risor.Function{Name: "f", Type: "(a?: int, b: int) => int"}}, "bad-signature"},
		{[]risor.Decl{risor.Function{Name: "geo", Type: "() => int"}, risor.Function{Name: "geo.d", Type: "() => int"}}, "name-clash"},
		{[]risor.Decl{risor.Declare("{geo: int}"), risor.Function{Name: "geo.d", Type: "() => int"}}, "name-clash"},
		{[]risor.Decl{risor.OpaqueType{Name: "T"}}, "bad-name"},
		{[]risor.Decl{risor.Version("4.0")}, "unsupported-version"},
		{[]risor.Decl{risor.RejectClasses("nope")}, "bad-class"},
	} {
		_, err := risor.NewEnv(c.decls...)
		var e *risor.Error
		assert.True(t, errors.As(err, &e), c.code)
		assert.Equal(t, e.Diagnostics[0].Code, c.code)
	}
	// A variable may be named like a library function (03-host.md 3.3.4).
	env, err := risor.NewEnv(risor.Declare("{type: string, max: int}"))
	assert.NoError(t, err)
	assert.Equal(t, run(t, env, "type + string(max)", risor.Map{"type": "a", "max": 1}), any("a1"))
	// A function with no implementation fails the compile.
	env, _ = risor.NewEnv(risor.Function{Name: "f", Type: "() => int"})
	x, _ := risor.Parse("f()")
	_, err = env.Compile(x)
	assert.Equal(t, err.(*risor.Error).Diagnostics[0].Code, "no-implementation")
}

func TestCompileOnceRunMany(t *testing.T) {
	env, _ := risor.NewEnv(risor.Declare("{n: int}"))
	p := compile(t, env, "const sq = x => x * x; sq(n) + 1")
	done := make(chan bool)
	for i := 0; i < 8; i++ {
		go func(i int) {
			v, err := p.Run(context.Background(), risor.Map{"n": i})
			assert.NoError(t, err)
			assert.Equal(t, v, any(float64(i*i+1)))
			done <- true
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	assert.True(t, risor.Truthy([]any{0}))
	assert.False(t, risor.Truthy(map[string]int{}))
	n, ok := risor.AsInt(3.0)
	assert.True(t, ok)
	assert.Equal(t, n, int64(3))
}

func TestPreAllocationChargingBeforeGrowth(t *testing.T) {
	const count = 200_000
	xs := make([]int, count)
	m := make(map[string]int, count)
	for i := 0; i < count; i++ {
		xs[i] = i
		m[fmt.Sprintf("k%d", i)] = i
	}
	bigStr := strings.Repeat("a\"\\\n", count)

	env, err := risor.NewEnv(
		risor.Declare("{xs: int[], m: map<int>, s: string}"),
		risor.Groups("json"),
		risor.Limits{Steps: risor.NoLimit, Alloc: 5},
	)
	assert.NoError(t, err)
	vars := risor.Map{"xs": xs, "m": m, "s": bigStr}

	// Each expression would build multiple megabytes if allocated before
	// charging units; under Alloc: 5 it must fail with allocation-exceeded
	// and negligible heap allocation.
	for _, src := range []string{
		"[...xs]",
		"{...m}",
		"`${s}${s}`",
		"string([s])",
		"json.stringify(s)",
	} {
		p := compile(t, env, src)
		// Warm up once, then measure heap bytes allocated per failed run.
		_, _ = p.Run(context.Background(), vars)
		var m0, m1 runtime.MemStats
		runtime.ReadMemStats(&m0)
		d := runErr(t, env, src, vars)
		runtime.ReadMemStats(&m1)
		assert.Equal(t, d.Code, "allocation-exceeded", src)
		// Only [...xs], `${s}${s}` and json.stringify(s) avoid host map reflection;
		// even across all 5 cases, heap allocated is far below copying the data.
		if src == "[...xs]" || src == "`${s}${s}`" || src == "string([s])" || src == "json.stringify(s)" {
			assert.True(t, m1.TotalAlloc-m0.TotalAlloc < 64*1024, "%s allocated %d bytes", src, m1.TotalAlloc-m0.TotalAlloc)
		}
	}

	// Error precedence: an error in a later item or entry still precedes
	// allocation-exceeded, and steps-exceeded precedes allocation-exceeded.
	assert.Equal(t, runErr(t, env, "[...xs, 1 / 0]", vars).Code, "divide-by-zero")
	assert.Equal(t, runErr(t, env, "{...m, [null]: 1}", vars).Code, "null-operand")
	assert.Equal(t, runErr(t, env, "`${s}${1 / 0}`", vars).Code, "divide-by-zero")

	bothEnv, err := risor.NewEnv(
		risor.Declare("{s: string}"),
		risor.Limits{Steps: 2, Alloc: 2},
	)
	assert.NoError(t, err)
	assert.Equal(t, runErr(t, bothEnv, "`${s}`", vars).Code, "steps-exceeded")

	// Text past the length the walk keeps (64 KiB) is rendered a second
	// time, after the charges, to the same string.
	okEnv, err := risor.NewEnv(
		risor.Declare("{s: string}"),
		risor.Groups("json"),
		risor.Limits{Steps: risor.NoLimit, Alloc: risor.NoLimit},
	)
	assert.NoError(t, err)
	short := strings.Repeat("x\"\\\t\x01é😀", 6000)
	v := risor.Map{"s": short}
	assert.Equal(t, run(t, okEnv, "`<${s}>`", v), any("<"+short+">"))
	wantJSON, err := risor.JSONText(short)
	assert.NoError(t, err)
	assert.Equal(t, run(t, okEnv, "json.stringify(s)", v), any(wantJSON))
	assert.Equal(t, run(t, okEnv, "string([s])", v), any("["+wantJSON+"]"))
	assert.Equal(t, run(t, okEnv, "string([s, 1.5, true, null, {a: s}])", v),
		any("["+wantJSON+",1.5,true,null,{\"a\":"+wantJSON+"}]"))
	assert.Equal(t, run(t, okEnv, "`<${s}${1}${[s]}>`", v), any("<"+short+"1["+wantJSON+"]>"))
}
