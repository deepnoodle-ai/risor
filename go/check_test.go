package risor_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

func checkErr(t *testing.T, env *risor.Env, src string, opts ...risor.CheckOption) []risor.Diagnostic {
	t.Helper()
	x, err := risor.Parse(src)
	assert.NoError(t, err)
	_, err = env.Check(x, opts...)
	var e *risor.Error
	assert.True(t, errors.As(err, &e), "%s: expected a check failure", src)
	return e.Diagnostics
}

func TestCheckAPI(t *testing.T) {
	env, err := risor.NewEnv(risor.VarRecord("{gate: {failed: int, items: {name: string}[]}, blob: any}"))
	assert.NoError(t, err)
	x, _ := risor.Parse("gate.items[0].name")
	r, err := env.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, r.Type.String(), "string")
	assert.Equal(t, len(r.Sites), 1)
	assert.Equal(t, r.Sites[0].Class, "index")
	assert.Equal(t, r.Sites[0].Span.Array(), [4]int{1, 1, 1, 14})
	// The expected result.
	ds := checkErr(t, env, `gate.failed > 0 ? 1 : "a"`, risor.Expect("int"))
	assert.Equal(t, ds[0].Code, "result-mismatch")
	assert.Equal(t, ds[0].Span.Array(), [4]int{1, 23, 1, 26})
	ds = checkErr(t, env, `gate`, risor.Condition)
	assert.Equal(t, ds[0].Code, "always-truthy")
	assert.Equal(t, ds[0].Class, "constant-condition")
	// An expected type that does not parse is a declare error.
	_, err = env.Check(x, risor.Expect("number"))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
	// Strict mode: dynamic rejected.
	strict, _ := risor.NewEnv(risor.VarRecord("{blob: any}"), risor.Reject("constant-condition", "dynamic"))
	ds = checkErr(t, strict, "blob.x")
	assert.Equal(t, ds[0].Code, "unchecked")
	assert.Equal(t, ds[0].Class, "dynamic")
}

// 05-types.md 19.5: the errors of declared type text that
// need the checker.
func TestDeclareWithChecker(t *testing.T) {
	for _, d := range []risor.Decl{
		risor.VarRecord("{q: Foo}"),
		risor.VarRecord("{q: {a: int, a: int}}"),
		risor.VarRecord("{q: {a: int} | {b: int}}"),
	} {
		_, err := risor.NewEnv(d)
		var e *risor.Error
		assert.True(t, errors.As(err, &e))
		assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
	}
	// An error in a host signature is bad-signature (03-host.md 3.3.4).
	_, err0 := risor.NewEnv(risor.Function{Name: "f", Type: "(t: Order) => int"})
	var e0 *risor.Error
	assert.True(t, errors.As(err0, &e0))
	assert.Equal(t, e0.Diagnostics[0].Code, "bad-signature")
	_, err := risor.NewEnv(risor.VarRecord("{t: Time}"), risor.OpaqueType{Name: "Time"})
	assert.NoError(t, err)
}

// 05-types.md 20.3: at most 1,000 lambda body checks.
func TestCheckLimitUses(t *testing.T) {
	env, _ := risor.NewEnv()
	calls := strings.Repeat("f(1), ", 1000)
	x, _ := risor.Parse("const f = x => x; [" + calls + "]")
	_, err := env.Check(x)
	assert.NoError(t, err)
	ds := checkErr(t, env, "const f = x => x; ["+calls+"f(1)]")
	assert.Equal(t, ds[0].Code, "check-limit")
	assert.Equal(t, ds[0].Span.Array(), [4]int{1, 6020, 1, 6024})
}

// 05-types.md 20.2: the expanded nesting of a use.
func TestCheckLimitNesting(t *testing.T) {
	env, _ := risor.NewEnv()
	body := strings.Repeat("(", 49) + "x" + strings.Repeat(")", 49)
	ok := "const g = x => " + body + "; " + strings.Repeat("(", 50) + "g(1)" + strings.Repeat(")", 50)
	x, _ := risor.Parse(ok)
	_, err := env.Check(x)
	assert.NoError(t, err)
	bad := "const g = x => " + body + "; " + strings.Repeat("(", 51) + "g(1)" + strings.Repeat(")", 51)
	ds := checkErr(t, env, bad)
	assert.Equal(t, ds[0].Code, "check-limit")
}

// 05-types.md 20.4: a type the checker builds deeper than 64.
func TestCheckLimitTypeDepth(t *testing.T) {
	env, _ := risor.NewEnv()
	x, _ := risor.Parse(strings.Repeat("[", 64) + "1" + strings.Repeat("]", 64))
	r, err := env.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, r.Type.String(), "int"+strings.Repeat("[]", 64))
	ds := checkErr(t, env, strings.Repeat("[", 65)+"1"+strings.Repeat("]", 65))
	assert.Equal(t, ds[0].Code, "check-limit")
	assert.Equal(t, ds[0].Span.Array(), [4]int{1, 1, 1, 132})
}

// The class of a run failure in a checked program (03-host.md 3.7.6), and
// the same failure unchecked.
func TestCheckedClass(t *testing.T) {
	env, _ := risor.NewEnv(risor.VarRecord("{x: {a: int}, y: any, s: \"a\" | \"b\"}"))
	vars := risor.Map{"x": map[string]any{}, "y": map[string]any{}, "s": "c"}
	for _, c := range []struct{ src, checked, unchecked string }{
		{"x.a", "environment", "dynamic"},
		{"y.a", "dynamic", "dynamic"},
		{`match s { "a" => 1, "b" => 2 }`, "environment", "dynamic"},
		{"y.a.b", "dynamic", "dynamic"},
	} {
		x, err := risor.Parse(c.src)
		assert.NoError(t, err)
		for _, mode := range []struct {
			opts  []risor.CheckOption
			class string
		}{{nil, c.checked}, {[]risor.CheckOption{risor.NoCheck}, c.unchecked}} {
			p, err := env.Compile(x, mode.opts...)
			assert.NoError(t, err, c.src)
			_, u, err := p.Measure(context.Background(), vars)
			var e *risor.Error
			assert.True(t, errors.As(err, &e), c.src)
			assert.Equal(t, e.Diagnostics[0].Class, mode.class, c.src)
			assert.Equal(t, u, risor.Usage{Steps: u.Steps, Allocated: u.Allocated})
		}
	}
}

// What opens no level is checked by a loop (20.6): long flat programs check.
func TestCheckFlatPrograms(t *testing.T) {
	env, _ := risor.NewEnv(risor.VarRecord("{c: bool, m: map<any>}"))
	n := 100000
	var consts strings.Builder
	for i := 0; i < n; i++ {
		consts.WriteString("const a" + strings.Repeat("x", 0) + itoa(i) + " = " + itoa(i) + ";\n")
	}
	consts.WriteString("a0")
	for src, want := range map[string]string{
		strings.Repeat("1 + ", n) + "1":                                          "int",
		"m" + strings.Repeat("?.a", n):                                           "any",
		strings.Repeat("c ? 1 : ", n) + "2":                                      "int",
		"if (c) { 1 }" + strings.Repeat(" else if (c) { 1 }", n) + " else { 2 }": "int",
		consts.String():                      "int",
		"[" + strings.Repeat("1, ", n) + "]": "int[]",
	} {
		x, err := risor.Parse(src)
		assert.NoError(t, err)
		r, err := env.Check(x)
		assert.NoError(t, err)
		assert.Equal(t, r.Type.String(), want)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}
