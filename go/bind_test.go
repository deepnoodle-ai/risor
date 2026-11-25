package risor_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/assert"
)

// Func and Methods (bind.go): signatures derived from Go types.

// label is an opaque type with pointer receivers, and a method promoted
// from an embedded struct.
type label struct {
	text string
	base
}

type base struct{ shown bool }

func (b *base) Shown() bool { return b.shown }
func (b *base) Show()       { b.shown = true }

func (*label) OpaqueType() string          { return "Label" }
func (l *label) Text() string              { return l.text }
func (l *label) SetText(s string)          { l.text = s }
func (l *label) Len() int                  { return len(l.text) }
func (l *label) URL() string               { return "https://x.test/" + l.text }
func (l *label) HTTPStatus() int           { return 200 }
func (l *label) UserID() string            { return "u-" + l.text }
func (l *label) Fail(msg string) error     { return errors.New(msg) }
func (l *label) Words(sep string) []string { return strings.Split(l.text, sep) }
func (l *label) Join(xs []string) string   { return strings.Join(xs, l.text) }
func (l *label) Grow(c *risor.Call, n int) (int, error) {
	if err := c.Charge(int64(n)); err != nil {
		return 0, err
	}
	l.text += strings.Repeat("+", n)
	return len(l.text), c.Allocate(1)
}

// purse is an opaque type with value receivers.
type purse struct{ cents int64 }

func (purse) OpaqueType() string  { return "Money" }
func (m purse) Cents() int64      { return m.cents }
func (m purse) Add(o purse) purse { return purse{m.cents + o.cents} }
func (m purse) String() string    { return fmt.Sprintf("$%d.%02d", m.cents/100, m.cents%100) }
func (m purse) Split(n int) []purse {
	return []purse{{m.cents / int64(n)}, {m.cents - m.cents/int64(n)}}
}
func (m purse) Over(limit float64) bool { return float64(m.cents) > limit }

// outcome runs src, checked unless noCheck, and gives its value or its
// failure as text, so that two environments can be compared.
func bindOutcome(t *testing.T, env *risor.Env, src string, vars risor.Vars, noCheck bool) string {
	t.Helper()
	x, err := risor.Parse(src)
	assert.NoError(t, err, src)
	var opts []risor.CheckOption
	if noCheck {
		opts = append(opts, risor.NoCheck)
	}
	p, err := env.Compile(x, opts...)
	if err != nil {
		return "compile: " + err.Error()
	}
	v, u, err := p.Measure(context.Background(), vars)
	if err != nil {
		var e *risor.Error
		assert.True(t, errors.As(err, &e), src)
		d := e.Diagnostics[0]
		return fmt.Sprintf("%s %s: %s %v", d.Code, d.Class, d.Message, u)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("value %#v %v", v, u)
	}
	return fmt.Sprintf("%s %v", b, u)
}

// expectAll compares each program's outcome with what is wanted, and
// reports every difference at once.
func expectAll(t *testing.T, cases map[string]string, got func(src string) string) {
	t.Helper()
	var diffs []string
	for _, src := range sortedStrings(cases) {
		if g := got(src); g != cases[src] {
			diffs = append(diffs, fmt.Sprintf("%s\n\tgot  %s\n\twant %s", src, g, cases[src]))
		}
	}
	if len(diffs) > 0 {
		t.Log("\n" + strings.Join(diffs, "\n"))
	}
	assert.Equal(t, len(diffs), 0)
}

func sortedStrings(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func newEnv(t *testing.T, decls ...risor.Decl) *risor.Env {
	t.Helper()
	env, err := risor.NewEnv(decls...)
	assert.NoError(t, err)
	return env
}

// declareErr builds an environment that must fail, and gives its first
// diagnostic.
func declareErr(t *testing.T, decls ...risor.Decl) risor.Diagnostic {
	t.Helper()
	_, err := risor.NewEnv(decls...)
	var e *risor.Error
	assert.True(t, errors.As(err, &e), "expected a declare error, got %v", err)
	assert.Equal(t, e.Diagnostics[0].Phase, risor.PhaseDeclare)
	return e.Diagnostics[0]
}

func TestMethodName(t *testing.T) {
	for goName, want := range map[string]string{
		"Name":        "name",
		"SetText":     "setText",
		"URL":         "url",
		"ID":          "id",
		"HTTPStatus":  "httpStatus",
		"UserID":      "userID",
		"MyURL":       "myURL",
		"X":           "x",
		"XAxis":       "xAxis",
		"ID2":         "id2",
		"HTTP2Server": "http2Server",
		"IOReader":    "ioReader",
		"ABTest":      "abTest",
		"A1":          "a1",
		"Get_Name":    "get_Name",
		// The limit of the rule: a plural initialism splits before its s.
		"URLs": "urLs",
	} {
		assert.Equal(t, risor.MethodName(goName), want, goName)
	}
}

func TestFuncSignatures(t *testing.T) {
	type order struct {
		ID    string  `json:"id"`
		Total float64 `json:"total"`
		Note  string  `json:"note,omitempty"`
	}
	for _, c := range []struct {
		f          risor.Function
		typ        string
		reflective bool
	}{
		{risor.Func("upper", strings.ToUpper), "(a1: string) => string", false},
		{risor.Func("f", func() {}), "() => null", false},
		{risor.Func("f", func() error { return nil }), "() => null !", false},
		{risor.Func("f", strconv.Atoi), "(a1: string) => int !", false},
		{risor.Func("f", strings.Repeat), "(a1: string, a2: int) => string", false},
		{risor.Func("f", math.Max), "(a1: float, a2: float) => float", false},
		{risor.Func("f", func(a, b, c bool) bool { return a }), "(a1: bool, a2: bool, a3: bool) => bool", false},
		{risor.Func("f", func(v any) any { return v }), "(a1: any) => any", false},
		{risor.Func("f", func(n int64) (int64, error) { return n, nil }), "(a1: int) => int !", false},
		{risor.Func("f", func(c *risor.Call, s string) (any, error) { return s, nil }), "(a1: string) => any !", false},
		{risor.Func("f", func(ctx context.Context, n int) (bool, error) { return true, nil }), "(a1: int) => bool !", false},
		{risor.Func("f", func(ctx context.Context) error { return nil }), "() => null !", false},
		{risor.Func("f", func(c *risor.Call) {}), "() => null", true},
		{risor.Func("f", func(n int, s string) string { return s }), "(a1: int, a2: string) => string", true},
		{risor.Func("f", func(xs []string) []string { return xs }), "(a1: string[]) => string[]", true},
		{risor.Func("f", func(m map[string]int) int { return len(m) }), "(a1: map<int>) => int", true},
		{risor.Func("f", func(o order) *order { return &o }), "(a1: {id: string, note?: string, total: float}) => {id: string, note?: string, total: float} | null", true},
		{risor.Func("f", func(n int8, u uint16) float32 { return 0 }), "(a1: int, a2: int) => float", true},
		{risor.Func("f", func(t time.Time) time.Duration { return 0 }), "(a1: string) => int", true},
		{risor.Func("f", func(s string) *label { return &label{text: s} }), "(a1: string) => Label", true},
		{risor.Func("f", func(m purse, n *purse) purse { return m }), "(a1: Money, a2: Money | null) => Money", true},
		{risor.Func("geo.dist", math.Hypot, risor.Params("dx", "dy")), "(dx: float, dy: float) => float", false},
	} {
		assert.Equal(t, c.f.Type, c.typ)
		assert.Equal(t, c.f.Reflective(), c.reflective, c.typ)
		assert.NotNil(t, c.f.Impl, c.typ)
	}
	assert.False(t, risor.Function{Name: "f", Type: "() => int"}.Reflective())
}

func TestFuncDeclareErrors(t *testing.T) {
	type withIface struct{ S fmt.Stringer }
	for _, c := range []struct {
		f    risor.Function
		code string
		msg  string
	}{
		{risor.Func("f", 42), "bad-type", "f: int is not a Go function"},
		{risor.Func("f", nil), "bad-type", "f: nil is not a Go function"},
		{risor.Func("f", fmt.Sprintf), "bad-type", "f: func(string, ...interface {}) string is variadic, and a variadic Go function has no derived signature: write one whose last parameter is rest, with risor.Signature"},
		{risor.Func("f", func() (int, int, error) { return 0, 0, nil }), "bad-type", "f: func() (int, int, error) has more than two results"},
		{risor.Func("f", func() (int, int) { return 0, 0 }), "bad-type", "f: func() (int, int) has two results, and the second is not error"},
		{risor.Func("f", func(ch chan int) {}), "bad-type", "f: parameter 1: chan int: no rule of the binding covers chan int"},
		{risor.Func("f", func(c complex128) {}), "bad-type", "f: parameter 1: complex128: no rule of the binding covers complex128"},
		{risor.Func("f", func(c *risor.Call, s string, ch chan int) {}), "bad-type", "f: parameter 2: chan int: no rule of the binding covers chan int"},
		{risor.Func("f", func(m map[int]string) {}), "bad-type", "f: parameter 1: map[int]string: a map key must be a string, found int"},
		{risor.Func("f", func() chan int { return nil }), "bad-type", "f: the result: chan int: no rule of the binding covers chan int"},
		{risor.Func("f", func(s string, c *risor.Call) {}), "bad-type", "f: a *risor.Call is only the first parameter"},
		{risor.Func("f", func(l *lazy) {}), "bad-type", "f: parameter 1: *risor_test.lazy: the opaque name of its zero value panics: runtime error: invalid memory address or nil pointer dereference"},
		{risor.Func("f", func(g func(int) int) int { return 0 }), "bad-signature", "f: parameter 1 is a Go function, and a host function takes no function"},
		{risor.Func("f", func(s fmt.Stringer) string { return "" }), "bad-type", "f: the Go interface type fmt.Stringer has no derived type: write the signature in Type"},
		{risor.Func("f", func(w withIface) {}), "bad-type", "f: the Go interface type fmt.Stringer has no derived type: write the signature in Type"},
		{risor.Func("f", func(e error) {}), "bad-type", "f: the Go interface type error has no derived type: write the signature in Type"},
		{risor.Func("f", func() (error, error) { return nil, nil }), "bad-type", "f: the Go interface type error has no derived type: write the signature in Type"},
		{risor.Func("f", func(a, b int) int { return 0 }, risor.Params("a")), "bad-signature", "f: Params gives 1 names, and the Go function takes 2 parameters"},
		{risor.Func("f", func(a, b int) int { return 0 }, risor.Params("a", "if")), "bad-signature", `f: "if" is not a name for a parameter`},
		{risor.Func("f", func(a, b int) int { return 0 }, risor.Params("a", "a")), "bad-signature", "f: parameter a is named twice"},
		// The opaque type of a parameter must be declared (03-host.md 3.3.4).
		{risor.Func("f", func(m purse) int { return 0 }), "bad-signature", "function f: Money is not a declared opaque type or named type"},
	} {
		d := declareErr(t, c.f)
		assert.Equal(t, d.Code, c.code, c.msg)
		assert.Equal(t, d.Message, c.msg)
	}
	// A derivation error leaves Type empty and gives no implementation.
	f := risor.Func("f", func(ch chan int) {})
	assert.Equal(t, f.Type, "")
	assert.Nil(t, f.Impl)
	// A nil function of a Go type is declared, and the compile reports that
	// it has no implementation.
	var nilFn func(string) string
	env := newEnv(t, risor.Func("f", nilFn))
	x, _ := risor.Parse(`f("a")`)
	_, err := env.Compile(x)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "no-implementation")
}

// A Type the host writes replaces the derived one: the way to use a Go
// interface type, or to narrow a derived signature.
func TestFuncHostSignature(t *testing.T) {
	describe := risor.Func("describe", func(s fmt.Stringer) string { return "[" + s.String() + "]" })
	assert.Equal(t, describe.Type, "")
	describe.Type = "(m: Money) => string"
	find := risor.Func("find", func(id string) *purse { return &purse{100} })
	assert.Equal(t, find.Type, "(a1: string) => Money | null")
	find.Type = "(id: string) => Money"
	env := newEnv(t, risor.Methods[purse]("Money"), describe, find)
	vars := risor.Map{"m": purse{250}, "blob": 5.0}
	env, err := env.With(risor.Var("m", "Money"), risor.Var("blob", "any"))
	assert.NoError(t, err)
	assert.Equal(t, bindOutcome(t, env, `describe(m)`, vars, false), `"[$2.50]" {1 0}`)
	assert.Equal(t, bindOutcome(t, env, `find("a").cents()`, vars, false), `100 {2 0}`)
	// A value that is not a fmt.Stringer is refused by the adapter.
	assert.Equal(t, bindOutcome(t, env, `describe(blob)`, vars, false),
		"function-error environment: argument 1 of describe: expected fmt.Stringer, found 5 {1 0}")

	// The text must take the Go function's parameters, all required.
	bad := risor.Func("f", func(a, b int) int { return a })
	bad.Type = "(a: int) => int"
	d := declareErr(t, bad)
	assert.Equal(t, d.Code, "bad-signature")
	assert.Equal(t, d.Message, "f: the signature has 1 parameters, and the Go function takes 2")
	bad.Type = "(a: int, b?: int) => int"
	d = declareErr(t, bad)
	assert.Equal(t, d.Code, "bad-signature")
	assert.Equal(t, d.Message, "f: parameter b is optional, and the Go function takes every parameter")
	bad.Type = "(a: int, ...b: int[]) => int"
	d = declareErr(t, bad)
	assert.Equal(t, d.Code, "bad-signature")
	assert.Equal(t, d.Message, "f: parameter b is rest, and the Go function is not variadic")
	// A shape error is reported whatever Type says.
	bad = risor.Func("f", func() (int, int) { return 0, 0 })
	bad.Type = "() => int"
	assert.Equal(t, declareErr(t, bad).Code, "bad-type")
}

// Signature gives the written signature in the call of Func, so a var
// initializer needs no second statement.
func TestFuncSignatureOption(t *testing.T) {
	choose := risor.Func("choose", func(options []any) any { return options[0] },
		risor.Signature("(options: T[]) => T"))
	assert.Equal(t, choose.Type, "(options: T[]) => T")
	describe := risor.Func("describe", func(s fmt.Stringer) string { return "[" + s.String() + "]" },
		risor.Signature("(m: Money) => string"))
	env := newEnv(t, risor.Methods[purse]("Money"), choose, describe, risor.Var("m", "Money"))
	vars := risor.Map{"m": purse{250}}
	assert.Equal(t, bindOutcome(t, env, `choose([1, 2]) + 1`, vars, false), `2 {2 3}`)
	assert.Equal(t, bindOutcome(t, env, `describe(m)`, vars, false), `"[$2.50]" {1 0}`)
	// It also reaches a method through ForMethod.
	o := risor.Methods[purse]("Money", risor.ForMethod("Cents", risor.Signature("() => number")))
	assert.Equal(t, o.Methods["cents"].Type, "() => number")
	// The written signature is tested as one assigned to Type.
	d := declareErr(t, risor.Func("f", func(a, b int) int { return a }, risor.Signature("(a: int) => int")))
	assert.Equal(t, d.Message, "f: the signature has 1 parameters, and the Go function takes 2")
}

// A variadic Go function is declared with a written signature whose last
// parameter is rest. The rest arguments reach its ...T, each converted.
func TestFuncVariadic(t *testing.T) {
	concat := risor.Func("concat", func(lists ...[]any) []any {
		out := []any{}
		for _, l := range lists {
			out = append(out, l...)
		}
		return out
	}, risor.Signature("(...lists: T[][]) => T[]"))
	join := risor.Func("join", func(sep string, parts ...string) string { return strings.Join(parts, sep) },
		risor.Signature("(sep: string, ...parts: string[]) => string"))
	count := risor.Func("count", func(xs ...any) int { return len(xs) },
		risor.Signature("(...xs: any[]) => int"))
	assert.True(t, concat.Reflective())
	env := newEnv(t, concat, join, count, risor.Var("blob", "any"))
	vars := risor.Map{"blob": 5.0}
	expectAll(t, map[string]string{
		`concat([1], [], [2, 3])`:  `[1,2,3] {1 6}`,
		`concat()`:                 `[] {1 0}`,
		`join("-", "a", "b", "c")`: `"a-b-c" {1 0}`,
		`join("-")`:                `"" {1 0}`,
		`count(1, "a", null)`:      `3 {1 0}`,
		`count()`:                  `0 {1 0}`,
	}, func(src string) string { return bindOutcome(t, env, src, vars, false) })
	// A program not checked reaches the adapter with any argument: a rest
	// argument that does not convert is named by its place in the call.
	assert.Equal(t, bindOutcome(t, env, `join("-", "a", 5)`, vars, true),
		`function-error host: argument 3 of join: expected string, found 5 {1 0}`)
	assert.Equal(t, bindOutcome(t, env, `concat([1], [2], "x")`, vars, true),
		`function-error host: argument 3 of concat: expected any[], found "x" {1 4}`)

	// Derived, the signature is refused; written, its last parameter must be
	// rest.
	f := risor.Func("f", func(xs ...int) int { return len(xs) })
	assert.Equal(t, f.Type, "")
	assert.Equal(t, declareErr(t, f).Code, "bad-type")
	for sig, msg := range map[string]string{
		"(xs: int[]) => int":            "f: the Go function is variadic, and the last parameter, xs, is not rest",
		"() => int":                     "f: the Go function is variadic, and the signature has no rest parameter",
		"(a: int, ...xs: int[]) => int": "f: the signature has 2 parameters, and the Go function takes 1",
	} {
		f.Type = sig
		d := declareErr(t, f)
		assert.Equal(t, d.Code, "bad-signature", sig)
		assert.Equal(t, d.Message, msg)
	}
}

// The fast adapters and the adapters by reflection give the same outcome:
// the same value or failure, and the same usage.
func TestFastAndReflectiveAgree(t *testing.T) {
	type key struct{}
	fns := []struct {
		name  string
		fn    any
		calls []string
	}{
		{"f", strings.ToUpper, []string{`f("abc")`, `f(1)`, `f(null)`, `f(blob)`, `f(big)`}},
		{"f", func(n int) int { return n * 2 }, []string{`f(21)`, `f(1.5)`, `f(1e20)`, `f(-1e15)`, `f("1")`, `f(blob)`}},
		{"f", func(n int64) int64 { return n << 10 }, []string{`f(3)`, `f(9007199254740991)`, `f(0.5)`, `f(true)`}},
		{"f", func(x float64) float64 { return x / 3 }, []string{`f(1)`, `f(-0)`, `f("x")`, `f(blob)`}},
		{"f", func(b bool) bool { return !b }, []string{`f(true)`, `f(0)`, `f(null)`}},
		{"f", func(v any) any { return v }, []string{`f(null)`, `f([1, "a"])`, `f({a: 1})`, `f(blob)`}},
		{"f", strings.Repeat, []string{`f("ab", 3)`, `f("ab", -1)`, `f(1, 2)`, `f("ab", "x")`}},
		{"f", func(a, b, c float64) float64 { return a*b + c }, []string{`f(2, 3, 4)`, `f(2, 3, "4")`}},
		{"f", strconv.Atoi, []string{`f("42")`, `f("x")`, `try { f("x") } catch (e) { e.message }`}},
		{"f", func() error { return errors.New("no") }, []string{`f()`, `try { f() } catch { 1 }`}},
		{"f", func() int64 { return 1 << 60 }, []string{`f()`, `[f()][0]`}},
		{"f", func() float64 { return math.Inf(1) }, []string{`f()`}},
		{"f", func() int { panic("bug") }, []string{`f()`, `try { f() } catch { 1 }`}},
		{"f", func(s string) {}, []string{`f("a")`, `f(2)`}},
		{"f", func(c *risor.Call, s string) (string, error) {
			if err := c.Charge(int64(len(s))); err != nil {
				return "", err
			}
			return s + s, c.Allocate(1)
		}, []string{`f("abc")`, `f(long)`, `f(1)`}},
		{"f", func(ctx context.Context, n int) (bool, error) {
			v, _ := ctx.Value(key{}).(string)
			return v == "on" && n > 0, nil
		}, []string{`f(1)`, `f(0)`}},
		{"f", func(ctx context.Context) (string, error) {
			v, _ := ctx.Value(key{}).(string)
			return v, nil
		}, []string{`f()`}},
		{"f", func(c *risor.Call) (int, error) { return 1, c.Charge(2) }, []string{`f()`}},
		{"f", func(xs []int, m map[string]string) int { return len(xs) + len(m) }, []string{`f([1, 2], {a: "b"})`, `f([1, 2.5], {})`, `f([1], {a: 1})`, `f(null, {})`}},
	}
	ctx := context.WithValue(context.Background(), key{}, "on")
	vars := risor.Map{"blob": 7.0, "big": strings.Repeat("é", 10), "long": strings.Repeat("x", 100)}
	reached := 0 // runs that reached an adapter with a value it refused
	for _, c := range fns {
		fast, slow := risor.Func(c.name, c.fn), risor.ReflectiveFunc(c.name, c.fn)
		assert.True(t, slow.Reflective())
		assert.Equal(t, fast.Type, slow.Type)
		decls := []risor.Decl{risor.Var("blob", "any"), risor.Var("big", "string"), risor.Var("long", "string"), risor.Limits{Steps: 50}}
		envFast := newEnv(t, append(decls, fast)...)
		envSlow := newEnv(t, append(decls, slow)...)
		// Checked programs refuse most wrong literals; the unchecked runs
		// bring them to the adapters.
		for _, src := range c.calls {
			for _, opts := range [][]risor.CheckOption{nil, {risor.NoCheck}} {
				x, _ := risor.Parse(src)
				pf, errF := envFast.Compile(x, opts...)
				ps, errS := envSlow.Compile(x, opts...)
				assert.Equal(t, fmt.Sprint(errF), fmt.Sprint(errS), src)
				if errF != nil {
					continue
				}
				vf, uf, ef := pf.Measure(ctx, vars)
				vs, us, es := ps.Measure(ctx, vars)
				assert.Equal(t, vf, vs, src)
				assert.Equal(t, uf, us, src)
				assert.Equal(t, fmt.Sprint(ef), fmt.Sprint(es), src)
				if ef != nil && strings.Contains(ef.Error(), "argument 1 of f") {
					reached++
				}
			}
		}
	}
	assert.True(t, reached >= 15, "only %d runs reached an adapter with a refused argument", reached)
}

func TestFuncCalls(t *testing.T) {
	env := newEnv(t,
		risor.Var("blob", "any"),
		risor.Func("upper", strings.ToUpper),
		risor.Func("atoi", strconv.Atoi),
		risor.Func("half", func(n int8) int8 { return n / 2 }),
		risor.Func("natural", func(n uint) uint { return n }),
		risor.Func("small", func(x float32) float32 { return x }),
		risor.Func("huge", func() int64 { return 1 << 60 }),
		risor.Func("most", func() uint64 { return math.MaxUint64 }),
		risor.Func("nan", math.NaN),
		risor.Func("inf", func() float64 { return math.Inf(1) }),
		risor.Func("inf32", func() float32 { return float32(math.Inf(-1)) }),
		risor.Func("boom", func() string { panic("bug") }),
		risor.Func("total", func(xs []struct {
			Qty int `json:"qty"`
		}) int {
			n := 0
			for _, x := range xs {
				n += x.Qty
			}
			return n
		}),
	)
	vars := risor.Map{"blob": "x"}
	expectAll(t, map[string]string{
		`upper("go")`:    `"GO" {1 0}`,
		`atoi("12") + 1`: `13 {2 0}`,
		`atoi("x")`:      `function-error host: strconv.Atoi: parsing "x": invalid syntax {1 0}`,
		`try { atoi("x") } catch (e) { e.message }`: `"strconv.Atoi: parsing \"x\": invalid syntax" {2 9}`,
		// An argument that does not convert is function-error, which catch
		// catches. Without ! on the signature its class is environment.
		`half(blob)`:                            `function-error environment: argument 1 of half: expected int, found "x" {1 0}`,
		`try { half(blob) } catch { "caught" }`: `"caught" {1 0}`,
		`half(300)`:                             `function-error environment: argument 1 of half: 300 does not fit int8 {1 0}`,
		`natural(-1)`:                           `function-error environment: argument 1 of natural: -1 does not fit uint {2 0}`,
		`small(1e300)`:                          `function-error environment: argument 1 of small: 1e+300 does not fit float32 {1 0}`,
		// A Go result past 2^53 - 1, a NaN and an infinity cannot enter.
		`huge()`:                             `number-range environment: a number from the host is out of range {1 0}`,
		`most()`:                             `number-range environment: a number from the host is out of range {1 0}`,
		`nan()`:                              `number-range environment: a number from the host is out of range {1 0}`,
		`inf()`:                              `number-range environment: a number from the host is out of range {1 0}`,
		`inf32()`:                            `number-range environment: a number from the host is out of range {1 0}`,
		`try { inf() } catch (e) { e.code }`: `"number-range" {3 10}`,
		`try { boom() } catch { 1 }`:         `function-panic host: host function boom crashed {1 0}`,
		`total([{qty: 2}, {qty: 3}])`:        `5 {1 9}`,
		`total([{qty: 2}, {qty: blob}])`:     `function-error environment: argument 1 of total, at [1].qty: expected int, found "x" {1 9}`,
	}, func(src string) string { return bindOutcome(t, env, src, vars, false) })
	// A program not checked reaches the adapter with any argument.
	expectAll(t, map[string]string{
		`upper(1)`:                       `function-error host: argument 1 of upper: expected string, found 1 {1 0}`,
		`total([{qty: 1, extra: true}])`: `function-error host: argument 1 of total, at [0].extra: the type does not allow key "extra" {1 7}`,
	}, func(src string) string { return bindOutcome(t, env, src, nil, true) })
}

// An adapted function that takes *Call charges as a hand-written one does,
// and a charge past a limit ends the run with the limit's failure.
func TestFuncCharges(t *testing.T) {
	grow := func(c *risor.Call, s string, n int) (string, error) {
		if err := c.Charge(int64(n)); err != nil {
			return "", err
		}
		if err := c.Allocate(int64(n)); err != nil {
			return "", err
		}
		return strings.Repeat(s, n), nil
	}
	native := risor.Function{Name: "grow", Type: "(s: string, n: int) => string !", Impl: func(c *risor.Call, args []any) (any, error) {
		return grow(c, args[0].(string), int(args[1].(float64)))
	}}
	f := risor.Func("grow", grow, risor.Params("s", "n"))
	assert.Equal(t, f.Type, native.Type)
	assert.True(t, f.Reflective()) // (*Call, string, int) is not a fast shape
	for _, lim := range []risor.Limits{{}, {Steps: 4}, {Allocation: 2}} {
		a, b := newEnv(t, lim, native), newEnv(t, lim, f)
		for _, src := range []string{`grow("ab", 3)`, `len(grow("a", 2)) + 1`} {
			assert.Equal(t, bindOutcome(t, b, src, nil, false), bindOutcome(t, a, src, nil, false), src)
		}
	}
	assert.Equal(t, bindOutcome(t, newEnv(t, f), `grow("ab", 3)`, nil, false), `"ababab" {4 3}`)
	assert.Equal(t, bindOutcome(t, newEnv(t, risor.Limits{Steps: 3}, f), `grow("ab", 3)`, nil, false), `steps-exceeded limit: the run needed more than its limit of 3 steps {1 0}`)
}

func TestMethods(t *testing.T) {
	op := risor.Methods[*label]("Label", risor.ForMethod("SetText", risor.Params("text")), risor.Skip("Join"))
	assert.Equal(t, sortedKeys(op.Methods), []string{"fail", "grow", "httpStatus", "len", "setText", "show", "shown", "text", "url", "userID", "words"})
	for m, want := range map[string]struct {
		typ        string
		reflective bool
	}{
		"text":       {"() => string", false},
		"setText":    {"(text: string) => null", false},
		"len":        {"() => int", false},
		"url":        {"() => string", false},
		"httpStatus": {"() => int", false},
		"fail":       {"(a1: string) => null !", false},
		"shown":      {"() => bool", false},
		"show":       {"() => null", false},
		"words":      {"(a1: string) => string[]", true},
		"grow":       {"(a1: int) => int !", true},
	} {
		assert.Equal(t, op.Methods[m].Type, want.typ, m)
		assert.Equal(t, op.Methods[m].Reflective(), want.reflective, m)
	}
	mo := risor.Methods[purse]("Money")
	assert.Equal(t, sortedKeys(mo.Methods), []string{"add", "cents", "over", "split", "string"})
	assert.Equal(t, mo.Methods["add"].Type, "(a1: Money) => Money")
	assert.Equal(t, mo.Methods["split"].Type, "(a1: int) => Money[]")
	assert.False(t, mo.Methods["cents"].Reflective())
	assert.True(t, mo.Methods["over"].Reflective()) // (float) => bool is not a setter

	env := newEnv(t, op, mo, risor.Var("l", "Label"), risor.Var("m", "Money"),
		risor.Func("newLabel", func(s string) *label { return &label{text: s} }))
	expectAll(t, map[string]string{
		`l.text()`:                         `"hi" {1 0}`,
		`[l.setText("yo"), l.text()][1]`:   `"yo" {3 3}`,
		`l.url()`:                          `"https://x.test/hi" {1 0}`,
		`[l.shown(), l.show(), l.shown()]`: `[false,null,true] {3 4}`,
		`l.words("i")`:                     `["h",""] {1 0}`,
		`l.fail("nope")`:                   `function-error host: nope {1 0}`,
		`l.grow(2)`:                        `4 {3 1}`,
		`newLabel("x").userID()`:           `"u-x" {2 0}`,
		`m.add(m).string()`:                `"$21.00" {2 0}`,
		`m.split(2).map(x => x.cents())`:   `[525,525] {9 3}`,
		`m.over(1000.5)`:                   `true {1 0}`,
	}, func(src string) string {
		return bindOutcome(t, env, src, risor.Map{"l": &label{text: "hi"}, "m": purse{1050}}, false)
	})
	// A method changes the host's own value (design 19-11).
	l := &label{text: "hi"}
	bindOutcome(t, env, `[l.setText("yo"), l.grow(2)]`, risor.Map{"l": l}, false)
	assert.Equal(t, l.text, "yo++")
	// The fast and the reflective methods agree.
	rop, rmo := risor.ReflectiveMethods[*label]("Label", risor.ForMethod("SetText", risor.Params("text")), risor.Skip("Join")), risor.ReflectiveMethods[purse]("Money")
	for name, f := range rop.Methods {
		assert.True(t, f.Reflective(), name)
		assert.Equal(t, f.Type, op.Methods[name].Type, name)
	}
	renv := newEnv(t, rop, rmo, risor.Var("l", "Label"), risor.Var("m", "Money"), risor.Var("blob", "any"))
	fenv := newEnv(t, op, mo, risor.Var("l", "Label"), risor.Var("m", "Money"), risor.Var("blob", "any"))
	for _, src := range []string{`[l.setText("ab"), l.text(), l.len()]`, `l.setText(blob)`, `l.setText(3)`, `m.cents()`, `m.add(blob)`, `m.add(l)`, `blob.text()`, `l.fail("x")`} {
		for _, noCheck := range []bool{false, true} {
			v1 := risor.Map{"l": &label{text: "q"}, "m": purse{5}, "blob": 1.0}
			v2 := risor.Map{"l": &label{text: "q"}, "m": purse{5}, "blob": 1.0}
			assert.Equal(t, bindOutcome(t, fenv, src, v1, noCheck), bindOutcome(t, renv, src, v2, noCheck), src)
		}
	}
	assert.Equal(t, bindOutcome(t, fenv, `l.setText(3)`, risor.Map{"l": l}, true), `function-error host: argument 1 of method setText: expected string, found 3 {1 0}`)
}

// A receiver of another Go type that names the same opaque type is refused
// by the adapter, not with a crash.
type fakeLabel struct{}

func (fakeLabel) OpaqueType() string { return "Label" }

func TestMethodsReceiver(t *testing.T) {
	env := newEnv(t, risor.Methods[*label]("Label"), risor.Var("l", "Label"))
	assert.Equal(t, bindOutcome(t, env, `l.text()`, risor.Map{"l": fakeLabel{}}, false),
		"function-error environment: the receiver of method text: expected Label, found opaque {1 0}")
	env = newEnv(t, risor.ReflectiveMethods[*label]("Label"), risor.Var("l", "Label"))
	assert.Equal(t, bindOutcome(t, env, `l.text()`, risor.Map{"l": fakeLabel{}}, false),
		"function-error environment: the receiver of method text: expected Label, found opaque {1 0}")
}

// pinger has a method that takes the run's context and no argument.
type pinger struct{}

type pingKey struct{}

func (*pinger) OpaqueType() string { return "Pinger" }
func (*pinger) Ping(ctx context.Context) (string, error) {
	v, _ := ctx.Value(pingKey{}).(string)
	return "pong " + v, nil
}

func TestMethodsLead(t *testing.T) {
	for _, o := range []risor.OpaqueType{risor.Methods[*pinger]("Pinger"), risor.ReflectiveMethods[*pinger]("Pinger")} {
		assert.Equal(t, o.Methods["ping"].Type, "() => string !")
		env := newEnv(t, o, risor.Var("p", "Pinger"))
		x, _ := risor.Parse("p.ping()")
		prog, err := env.Compile(x)
		assert.NoError(t, err)
		out, err := prog.Run(context.WithValue(context.Background(), pingKey{}, "ok"), risor.Map{"p": &pinger{}})
		assert.NoError(t, err)
		assert.Equal(t, out, any("pong ok"))
	}
}

// lazy's OpaqueType reads its receiver, which TypeOf and Methods forbid.
type lazy struct{ name string }

func (l *lazy) OpaqueType() string { return l.name }

type clash struct{}

func (*clash) OpaqueType() string { return "Clash" }
func (*clash) URL() string        { return "" }
func (*clash) Url() string        { return "" }

type piped struct{}

func (*piped) OpaqueType() string         { return "Piped" }
func (*piped) Feed(ch chan int)           {}
func (*piped) Name() string               { return "p" }
func (*piped) Show(s fmt.Stringer) string { return s.String() }

func TestMethodsDeclareErrors(t *testing.T) {
	for _, c := range []struct {
		o    risor.OpaqueType
		code string
		msg  string
	}{
		{risor.Methods[label]("Label"), "bad-type", "opaque type Label: risor_test.label does not implement risor.Opaque; *risor_test.label does"},
		{risor.Methods[*label]("Lbl"), "bad-type", "opaque type Lbl: the values of *risor_test.label name the opaque type Label"},
		{risor.Methods[fmt.Stringer]("Str"), "bad-type", "opaque type Str: fmt.Stringer is an interface type, not the Go type of opaque values"},
		{risor.Methods[*strings.Builder]("Builder"), "bad-type", "opaque type Builder: *strings.Builder does not implement risor.Opaque"},
		{risor.Methods[*lazy]("Lazy"), "bad-type", "opaque type Lazy: OpaqueType of the zero *risor_test.lazy panics; it must not read its receiver"}, {risor.Methods[*clash]("Clash"), "bad-signature", "opaque type Clash: the methods URL and Url of *risor_test.clash are both named url"},
		{risor.Methods[*label]("Label", risor.Rename("Text", "url")), "bad-signature", "opaque type Label: the methods Text and URL of *risor_test.label are both named url"},
		{risor.Methods[*label]("Label", risor.Rename("Text", "a-b")), "bad-name", `opaque type Label: "a-b", the name of method Text, is not a word`},
		{risor.Methods[*label]("Label", risor.Skip("Nope")), "bad-name", "opaque type Label: *risor_test.label has no method Nope"},
		{risor.Methods[*label]("Label", risor.Rename("OpaqueType", "kind")), "bad-name", "opaque type Label: *risor_test.label has no method OpaqueType"},
		{risor.Methods[*label]("Label", risor.ForMethod("text", risor.Params("a"))), "bad-name", "opaque type Label: *risor_test.label has no method text"},
		{risor.Methods[*label]("Label", risor.ForMethod("SetText", risor.Params("a", "b"))), "bad-signature", "Label.setText: Params gives 2 names, and the Go function takes 1 parameters"},
		{risor.Methods[*piped]("Piped", risor.Skip("Show")), "bad-type", "Piped.feed: parameter 1: chan int: no rule of the binding covers chan int"},
		{risor.Methods[*piped]("Piped", risor.Skip("Feed")), "bad-type", "Piped.show: the Go interface type fmt.Stringer has no derived type: write the signature in Type"},
	} {
		d := declareErr(t, c.o)
		assert.Equal(t, d.Code, c.code, c.msg)
		assert.Equal(t, d.Message, c.msg)
	}
	// Skip what does not derive, or write its signature.
	p := risor.Methods[*piped]("Piped", risor.Skip("Feed"))
	show := p.Methods["show"]
	show.Type = "(l: Label) => string"
	p.Methods["show"] = show
	env := newEnv(t, p, risor.Methods[*label]("Label"), risor.Var("p", "Piped"))
	assert.Equal(t, bindOutcome(t, env, `p.name()`, risor.Map{"p": &piped{}}, false), `"p" {1 0}`)
	// A Rename settles a clash.
	newEnv(t, risor.Methods[*clash]("Clash", risor.Rename("Url", "link")))
}

func TestBindDescribe(t *testing.T) {
	env := newEnv(t,
		risor.Func("geo.distance", math.Hypot, risor.Params("dx", "dy")),
		risor.Func("parse", strconv.ParseBool),
		risor.Methods[purse]("Money", risor.Rename("String", "format"), risor.ForMethod("Over", risor.Params("limit"))),
	)
	d, err := env.Describe()
	assert.NoError(t, err)
	var got struct {
		Functions map[string]string            `json:"functions"`
		Opaque    map[string]map[string]string `json:"opaque"`
	}
	assert.NoError(t, json.Unmarshal(d, &got))
	assert.Equal(t, got.Functions, map[string]string{
		"geo.distance": "(dx: float, dy: float) => float",
		"parse":        "(a1: string) => bool !",
	})
	assert.Equal(t, got.Opaque, map[string]map[string]string{"Money": {
		"add":    "(a1: Money) => Money",
		"cents":  "() => int",
		"format": "() => string",
		"over":   "(limit: float) => bool",
		"split":  "(a1: int) => Money[]",
	}})
	// The description declares the same signatures.
	decl, err := risor.FromDescription(d)
	assert.NoError(t, err)
	env2 := newEnv(t, decl)
	d2, _ := env2.Describe()
	assert.Equal(t, string(d2), string(d))
}

func sortedKeys(m map[string]risor.Function) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// BenchmarkHostCall compares a hand-written Native, the fast adapter and
// the adapter by reflection for the same Go function: the call alone, and
// a run that makes the call.
//
//	go test -run xxx -bench HostCall -benchmem
func BenchmarkHostCall(b *testing.B) {
	add := func(x, y float64) float64 { return x + y }
	native := risor.Function{Name: "add", Type: "(a1: float, a2: float) => float", Impl: func(c *risor.Call, args []any) (any, error) {
		return add(args[0].(float64), args[1].(float64)), nil
	}}
	fs := []struct {
		name string
		f    risor.Function
	}{
		{"native", native},
		{"fast", risor.Func("add", add)},
		{"reflective", risor.ReflectiveFunc("add", add)},
	}
	args := []any{1.5, 2.25}
	for _, f := range fs {
		b.Run("call/"+f.name, func(b *testing.B) {
			c := new(risor.Call)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := f.f.Impl(c, args); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	for _, f := range fs {
		b.Run("run/"+f.name, func(b *testing.B) {
			env, err := risor.NewEnv(risor.Var("x", "float"), f.f)
			if err != nil {
				b.Fatal(err)
			}
			x, _ := risor.Parse("add(x, 2.25)")
			p, err := env.Compile(x)
			if err != nil {
				b.Fatal(err)
			}
			vars := risor.Map{"x": 1.5}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := p.Run(context.Background(), vars); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	ms := []struct {
		name string
		o    risor.OpaqueType
	}{
		{"fast", risor.Methods[*label]("Label")},
		{"reflective", risor.ReflectiveMethods[*label]("Label")},
	}
	for _, m := range ms {
		b.Run("method/"+m.name, func(b *testing.B) {
			impl := m.o.Methods["text"].Impl
			c := new(risor.Call)
			args := []any{&label{text: "hi"}}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := impl(c, args); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
