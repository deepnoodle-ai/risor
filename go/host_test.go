package risor_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	risor "github.com/deepnoodle-ai/risor/go/v3"
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
	env, err := risor.NewEnv(risor.VarRecord("{xs: int[]}"),
		risor.Limits{Steps: risor.NoLimit, Allocation: risor.NoLimit, Deadline: time.Millisecond})
	assert.NoError(t, err)
	xs := make([]any, 3000)
	for i := range xs {
		xs[i] = i
	}
	d := runErr(t, env, "xs.map(x => xs.map(y => y + x))", risor.Map{"xs": xs})
	assert.Equal(t, d.Code, "deadline-exceeded")
	assert.Equal(t, d.Class, "limit")

	// Cancellation, also inside a try, which does not catch it.
	env2, _ := risor.NewEnv(risor.VarRecord("{xs: int[]}"), risor.Limits{Steps: risor.NoLimit, Allocation: risor.NoLimit})
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

// 03-host.md 3.5.3 items 5 to 7: a host function whose error is a failure of
// Risor, such as a nested run's, gives function-error with the inner
// diagnostic's message, keeps that failure as the cause, and carries no
// data. A chain of nested runs gives the innermost message.
func TestNestedRisorError(t *testing.T) {
	inner, err := risor.NewEnv(risor.VarRecord("{city: string}"))
	assert.NoError(t, err)
	tool := compile(t, inner, `city == "Paris" ? 18 : fail("unknown city " + string(city), {city})`)
	call := func(c *risor.Call, args []any) (any, error) {
		v, err := tool.Run(c.Context(), risor.Map{"city": args[0]})
		if err != nil {
			return nil, fmt.Errorf("tool weather: %w", err) // found through wrapping
		}
		return v, nil
	}
	mid, err := risor.NewEnv(risor.VarRecord("{city: string}"),
		risor.Function{Name: "weather", Type: "(city: string) => float !", Impl: call})
	assert.NoError(t, err)
	// The middle program calls weather, so its failure is a function-error
	// whose own message came from the innermost run.
	agent := compile(t, mid, "weather(city) + 1")
	outer, err := risor.NewEnv(risor.Function{Name: "ask", Type: "(city: string) => float !",
		Impl: func(c *risor.Call, args []any) (any, error) {
			return agent.Run(c.Context(), risor.Map{"city": args[0]})
		}})
	assert.NoError(t, err)

	d := runErr(t, mid, "weather(city)", risor.Map{"city": "Atlantis"})
	assert.Equal(t, d.Code, "function-error")
	assert.Equal(t, d.Message, "unknown city Atlantis")
	var cause *risor.Error
	assert.True(t, errors.As(d.Cause, &cause))
	assert.Equal(t, cause.Diagnostics[0].Code, "failed")
	assert.Equal(t, cause.Diagnostics[0].Data, any(map[string]any{"city": "Atlantis"}))

	d = runErr(t, outer, `ask("Atlantis")`, nil)
	assert.Equal(t, d.Code, "function-error")
	assert.Equal(t, d.Message, "unknown city Atlantis")
	assert.True(t, errors.As(d.Cause, &cause))
	assert.Equal(t, cause.Diagnostics[0].Code, "function-error")

	// The error value carries the innermost data (package F19 5.1).
	v := run(t, outer, `try { ask("Atlantis") } catch (e) { e }`, nil)
	assert.Equal(t, v, any(map[string]any{"code": "function-error",
		"message": "unknown city Atlantis", "path": nil, "data": map[string]any{"city": "Atlantis"}}))
	// The JSON form writes the chain of causes (F19 5.2).
	assert.True(t, strings.HasPrefix(risor.RenderJSON([]risor.Diagnostic{d}),
		`{"cause":{"cause":{"class":"value","code":"failed","data":{"city":"Atlantis"},"message":"unknown city Atlantis"`))
	// Any other error keeps its text, and a diagnostic of another code has
	// no cause.
	d = runErr(t, outer, `fail("x")`, nil)
	assert.Nil(t, d.Cause)
}

// badError is a host error whose methods crash.
type badError struct{ inError bool }

func (e badError) Error() string {
	if e.inError {
		panic("bug in Error")
	}
	return "offline"
}

func (e badError) Unwrap() error {
	if !e.inError {
		panic("bug in Unwrap")
	}
	return nil
}

// 03-host.md 3.5.3: a crash in host code is function-panic. The methods of a
// host's error are host code: reading the error for its message must not let
// their crash leave the run as an internal error.
func TestHostErrorThatCrashes(t *testing.T) {
	for _, e := range []badError{{inError: false}, {inError: true}} {
		env, err := risor.NewEnv(risor.Function{Name: "h", Type: "() => int !",
			Impl: func(c *risor.Call, args []any) (any, error) { return nil, e }})
		assert.NoError(t, err)
		d := runErr(t, env, "try { h() } catch { 0 }", nil)
		assert.Equal(t, d.Code, "function-panic")
		assert.Equal(t, d.Message, "host function h crashed")
	}
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
	assert.Equal(t, u, risor.Usage{Steps: 4, Allocated: 2})
	assert.Equal(t, run(t, env, "geo.dist(1) + geo.dist(1, 2)", nil), any(3.0))
}

// 03-host.md 3.4.4: a NaN and an infinity from the host cannot enter, in a
// plain float64 as in any other host number type, from a variable, an element,
// a field, a pointer or a host function's result.
func TestHostNaNAndInfinity(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(-1)
	type box struct {
		F float64
		G float32
	}
	f := risor.Function{Name: "bad", Type: "() => float", Impl: func(c *risor.Call, args []any) (any, error) { return nan, nil }}
	g := risor.Function{Name: "bads", Type: "() => float[]", Impl: func(c *risor.Call, args []any) (any, error) { return []any{1.0, inf}, nil }}
	env, err := risor.NewEnv(risor.VarRecord("{x: float, y: float, xs: float[], ys: float[], m: map<float>, s: {F: float, G: float}, p: float}"), f, g)
	assert.NoError(t, err)
	vars := risor.Map{"x": nan, "y": math.Inf(1), "xs": []any{1.0, inf}, "ys": []float64{1, nan},
		"m": map[string]any{"a": nan}, "s": box{F: nan, G: float32(math.Inf(1))}, "p": &nan}
	for _, src := range []string{"x", "y", "x == x", "y > 1", "[x]", "`${y}`", "xs[1]", "ys[1]", "m.a", "s.F", "s.G", "p",
		"xs.sum()", "ys.map(v => v)", "xs == xs", "`${xs}`", "bad()", "bads()[1]", "bads().sum()"} {
		d := runErr(t, env, src, vars)
		assert.Equal(t, d.Code, "number-range", src)
		assert.Equal(t, d.Class, "environment", src)
		assert.Equal(t, d.Message, "a number from the host is out of range", src)
	}
	assert.Equal(t, run(t, env, "try { x } catch (e) { e.code }", vars), any("number-range"))
	// What does not read the element does not fail (02-evaluation.md 3.3).
	assert.Equal(t, run(t, env, "len(xs) + len(bads())", vars), any(4.0))
	assert.Equal(t, run(t, env, "xs[0] + ys[0]", vars), any(2.0))
}

func TestHostNumbers(t *testing.T) {
	env, _ := risor.NewEnv(risor.VarRecord("{x: int, xs: int[], m: map<int>}"))
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
		risor.VarRecord("{t: Token, u: Token}"),
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
	env, _ := risor.NewEnv(risor.VarRecord("{s: string}"))
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
// (01-language.md 13.2 rule 6). n is four times the rule's 10,000, and each
// text stays within the size of a source text (2.1).
func TestFlatPrograms(t *testing.T) {
	n := 40000
	env, _ := risor.NewEnv(risor.VarRecord("{c: bool, m: map<any>}"))
	var consts strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&consts, "const a%d = %d;\n", i, i)
	}
	fmt.Fprintf(&consts, "a%d", n-1)
	cases := map[string]any{
		strings.Repeat("1 + ", n) + "1":                                          float64(n + 1),
		"m" + strings.Repeat("?.a", n):                                           nil,
		strings.Repeat("c ? 1 : ", n) + "2":                                      2.0,
		"if (c) { 1 }" + strings.Repeat(" else if (c) { 1 }", n) + " else { 2 }": 2.0,
		consts.String():                     float64(n - 1),
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

// StackNeeds documents and tests the stack this binding needs
// (03-host.md 3.9.1): to parse at the nesting limit, and to run at the
// default depth limit, 200, and the largest, 400, with calls by name, calls
// that map and flatMap make of a lambda, self-recursion through each, calls
// through a function parameter, and walks. stackDoc is the stack
// the binding documents (risor.go); the test caps every goroutine's stack at
// stackDoc divided by 1.5, so a deepest case that needs more crashes the
// test. Go grows a goroutine's stack as needed, and it grows by doubling: a
// case that needs a little more than 512 KiB holds a stack of 1 MiB, and
// the cap is tested against the stack held, not the bytes in use.
const (
	stackDoc = 2 << 20
	stackCap = stackDoc * 2 / 3
)

// linkChain is a program of n + 1 const-bound lambdas, f0 to fn, each link
// calling the one before by the text of link, and a call of fn.
func linkChain(base, link string, n int) string {
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, link+"\n", i, i-1)
	}
	fmt.Fprintf(&b, "f%d(1)", n)
	return b.String()
}

func TestStackNeeds(t *testing.T) {
	// The documented stack is that of an ordinary build (03-host.md 3.9.1).
	// The race detector makes frames larger, so under it the deepest cases
	// still run, with no cap.
	if !raceBuild {
		old := debug.SetMaxStack(stackCap)
		defer debug.SetMaxStack(old)
	}
	// Parse at the nesting limit: 100 levels of every kind.
	deep := strings.Repeat("[(-{a: ", 25) + "1" + strings.Repeat("})]", 25)
	_, err := risor.Parse(deep)
	assert.NoError(t, err)

	for _, depth := range []int{200, 400} {
		env, _ := risor.NewEnv(risor.VarRecord("{v: any, xs: int[]}"), risor.Limits{Depth: depth, Steps: risor.NoLimit, Allocation: risor.NoLimit})
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
		// Chains whose every link adds 1 to the depth count: by name, and
		// through a call that map or flatMap makes of a lambda. With depth - 1
		// links the count at f0 is the limit; one more link passes it. The
		// checker bounds the nesting of uses at 100 (05-types.md 20.2), so
		// these compile without the type rules, as compile does.
		chains := []struct{ base, link string }{
			{"const f0 = x => x;", "const f%d = x => f%d(x);"},
			{"const f0 = x => x;", "const f%d = x => [x].map(f%d)[0];"},
			{"const f0 = x => [x];", "const f%d = x => [x].flatMap(f%d);"},
		}
		for _, c := range chains {
			assert.NotNil(t, run(t, env, linkChain(c.base, c.link, depth-1), nil), c.link)
			d = runErr(t, env, linkChain(c.base, c.link, depth), nil)
			assert.Equal(t, d.Code, "depth-exceeded", c.link)
		}
		// Self-recursion (F17 11): a typed lambda that calls itself by name,
		// and that gives its own name to map and to flatMap. Each call adds
		// 1, so d(depth - 1) makes depth calls, the limit, and d(depth) one
		// more.
		for _, s := range selfRecursions {
			assert.NotNil(t, run(t, env, fmt.Sprintf(s, depth-1), nil), s)
			d = runErr(t, env, fmt.Sprintf(s, depth), nil)
			assert.Equal(t, d.Code, "depth-exceeded", s)
		}
		// Calls through a function parameter (F17 13.4 item 6). The first
		// reaches a depth count of 2N + 2, the second 2N + 3; n is the
		// largest N within the limit, and n + 1 passes it.
		for _, s := range fnParamCalls {
			n := (depth - s.extra) / 2
			assert.NotNil(t, run(t, env, fmt.Sprintf(s.src, n), nil), s.src)
			d = runErr(t, env, fmt.Sprintf(s.src, n+1), nil)
			assert.Equal(t, d.Code, "depth-exceeded", s.src)
		}
	}
}

// fnParamCalls are the forms of the stack test that call through a function
// parameter, with %d for N, and extra the depth count of N = 0: a recursion
// through a call of a function parameter at every level, and a chain of
// lambdas, each given at a function parameter and calling the one before.
var fnParamCalls = []struct {
	src   string
	extra int
}{
	{"const down = (f: (x: int) => int, n: int): int => n <= 0 ? 0 : f(n - 1); const d = (n: int): int => down(d, n); d(%d)", 2},
	{"const r = (f: (x: int) => int, n: int): int => n == 0 ? f(0) : r(y => f(y), n - 1); r(x => x, %d)", 3},
}

// selfRecursions are the forms of self-recursion of the stack test, with %d
// for the argument of the first call.
var selfRecursions = []string{
	"const d = (n: int): int => n <= 0 ? 0 : d(n - 1); d(%d)",
	"const d = (n: int): int => n <= 0 ? 0 : [n - 1].map(d)[0]; d(%d)",
	"const d = (n: int): int[] => n <= 0 ? [] : [n - 1].flatMap(d); d(%d)",
}

// 02-evaluation.md 5.3 item 3: map == visits the left map's keys in order when a value is a
// list or a map, so the deep pair under the first key meets the depth limit
// before the long string under the last key passes the step limit.
func TestMapEqualKeyOrder(t *testing.T) {
	env, _ := risor.NewEnv(risor.VarRecord("{m: any, n: any}"), risor.Limits{Depth: 5, Steps: 10})
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
	for _, l := range []risor.Limits{{Depth: 401}, {Depth: 1001}, {Depth: -1}, {Steps: -5}, {Allocation: -9}} {
		_, err := risor.NewEnv(l)
		var e *risor.Error
		assert.True(t, errors.As(err, &e))
		assert.Equal(t, e.Diagnostics[0].Code, "bad-limit")
	}
	// 03-host.md 3.6.1: the largest depth limit is 400.
	_, err := risor.NewEnv(risor.Limits{Depth: 400})
	assert.NoError(t, err)
}

func TestDeclare(t *testing.T) {
	for _, c := range []struct {
		decls []risor.Decl
		code  string
	}{
		{[]risor.Decl{risor.VarRecord("int")}, "bad-type"},
		{[]risor.Decl{risor.VarRecord("{a?: int}")}, "bad-type"},
		{[]risor.Decl{risor.VarRecord("{a: int, ...}")}, "bad-type"},
		{[]risor.Decl{risor.VarRecord("{a: int, a: int}")}, "name-clash"},
		{[]risor.Decl{risor.VarRecord(`{"a-b": int}`)}, "bad-name"},
		{[]risor.Decl{risor.VarRecord(`{if: int}`)}, "bad-name"},
		{[]risor.Decl{risor.Function{Name: "len", Type: "() => int"}}, "name-clash"},
		{[]risor.Decl{risor.Function{Name: "json.f", Type: "() => int"}}, "reserved-namespace"},
		{[]risor.Decl{risor.Function{Name: "a.b.c", Type: "() => int"}}, "bad-name"},
		{[]risor.Decl{risor.Function{Name: "f", Type: "(f: (x: int) => int) => int"}}, "bad-signature"},
		{[]risor.Decl{risor.Function{Name: "f", Type: "(a?: int, b: int) => int"}}, "bad-signature"},
		{[]risor.Decl{risor.Function{Name: "geo", Type: "() => int"}, risor.Function{Name: "geo.d", Type: "() => int"}}, "name-clash"},
		{[]risor.Decl{risor.VarRecord("{geo: int}"), risor.Function{Name: "geo.d", Type: "() => int"}}, "name-clash"},
		{[]risor.Decl{risor.OpaqueType{Name: "T"}}, "bad-name"},
		{[]risor.Decl{risor.Version("4.0")}, "unsupported-version"},
		{[]risor.Decl{risor.Reject("nope")}, "bad-class"},
	} {
		_, err := risor.NewEnv(c.decls...)
		var e *risor.Error
		assert.True(t, errors.As(err, &e), c.code)
		assert.Equal(t, e.Diagnostics[0].Code, c.code)
	}
	// A variable may be named like a library function (03-host.md 3.3.4).
	env, err := risor.NewEnv(risor.VarRecord("{type: string, max: int}"))
	assert.NoError(t, err)
	assert.Equal(t, run(t, env, "type + string(max)", risor.Map{"type": "a", "max": 1}), any("a1"))
	// A function with no implementation fails the compile.
	env, _ = risor.NewEnv(risor.Function{Name: "f", Type: "() => int"})
	x, _ := risor.Parse("f()")
	_, err = env.Compile(x)
	assert.Equal(t, err.(*risor.Error).Diagnostics[0].Code, "no-implementation")
}

func TestCompileOnceRunMany(t *testing.T) {
	env, _ := risor.NewEnv(risor.VarRecord("{n: int}"))
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
		risor.VarRecord("{xs: int[], m: map<int>, s: string}"),
		risor.Groups("json"),
		risor.Limits{Steps: risor.NoLimit, Allocation: 5},
	)
	assert.NoError(t, err)
	vars := risor.Map{"xs": xs, "m": m, "s": bigStr}

	// Each expression would build multiple megabytes if allocated before
	// charging units; under Allocation: 5 it must fail with allocation-exceeded
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
		risor.VarRecord("{s: string}"),
		risor.Limits{Steps: 2, Allocation: 2},
	)
	assert.NoError(t, err)
	assert.Equal(t, runErr(t, bothEnv, "`${s}`", vars).Code, "steps-exceeded")

	// Text past the length the walk keeps (64 KiB) is rendered a second
	// time, after the charges, to the same string.
	okEnv, err := risor.NewEnv(
		risor.VarRecord("{s: string}"),
		risor.Groups("json"),
		risor.Limits{Steps: risor.NoLimit, Allocation: risor.NoLimit},
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

// A map literal of many spreads, whose keys written pass the allocation
// limit, is charged for its entries, each key once, before it is built
// (02-evaluation.md 6.2 items 2 and 3).
func TestMapSpreadsCountedBeforeBuilt(t *testing.T) {
	vars := risor.Map{}
	decl, src := "{", "{"
	for i := 0; i < 10; i++ {
		m := map[string]any{}
		for j := i * 10; j < i*10+50; j++ {
			m[fmt.Sprintf("k%d", j)] = float64(i)
		}
		vars[fmt.Sprintf("m%d", i)] = m
		decl += fmt.Sprintf("m%d: map<float>, ", i)
	}
	decl += "}"
	for range 10 {
		for i := 0; i < 10; i++ {
			src += fmt.Sprintf("...m%d, ", i)
		}
	}
	src += "z: 1}"
	// 100 spreads write 5,001 keys; the map has 141 entries.
	limit := func(alloc int64) *risor.Env {
		env, err := risor.NewEnv(risor.VarRecord(decl), risor.Limits{Steps: risor.NoLimit, Allocation: alloc})
		assert.NoError(t, err)
		return env
	}
	m := run(t, limit(1+2*141), src, vars).(map[string]any)
	assert.Equal(t, len(m), 141)
	assert.Equal(t, m["k0"], any(0.0))
	assert.Equal(t, m["k55"], any(5.0))
	assert.Equal(t, m["k139"], any(9.0))
	assert.Equal(t, runErr(t, limit(2*141), src, vars).Code, "allocation-exceeded")
}
