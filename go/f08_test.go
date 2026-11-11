package risor_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F8: the trace, the renderings, ConformAll and the messages of the
// check phase, through the binding.

// failure compiles and runs src and returns the failure's diagnostics.
func failure(t *testing.T, env *risor.Env, src string, vars risor.Map) []risor.Diagnostic {
	t.Helper()
	x, err := risor.Parse(src)
	if err == nil {
		var p *risor.Program
		if p, err = env.Compile(x, risor.NoCheck); err == nil {
			_, err = p.Run(context.Background(), vars)
		}
	}
	var e *risor.Error
	assert.True(t, errors.As(err, &e), "expected a failure of %q", src)
	return e.Diagnostics
}

const orders = "const total = o => o.lines.map(l => l.qty * l.price).sum();\norders.map(o => total(o))"

func ordersData() risor.Map {
	line := func(p float64) map[string]any { return map[string]any{"qty": 10.0, "price": p} }
	o := func(ls ...map[string]any) map[string]any {
		xs := make([]any, len(ls))
		for i, l := range ls {
			xs[i] = l
		}
		return map[string]any{"lines": xs}
	}
	return risor.Map{"orders": []any{o(line(1)), o(line(2)), o(line(1), line(1.7e308))}}
}

func TestTraceExample(t *testing.T) {
	env, err := risor.NewEnv(risor.Var("orders", "{lines: {qty: float, price: float}[]}[]"))
	assert.NoError(t, err)
	ds := failure(t, env, orders, ordersData())
	assert.Equal(t, len(ds), 1)
	d := ds[0]
	assert.Equal(t, d.Code, "number-range")
	assert.Equal(t, sp(*d.Span), "1:37-1:52")
	var got []string
	for _, e := range d.Trace {
		got = append(got, e.Name+" "+sp(e.Span)+" "+idx(e.Index))
	}
	assert.Equal(t, got, []string{"map 2:1-2:26 2", "total 2:17-2:25 -1", "map 1:20-1:53 1"})
	want := "error[number-range] 1:37-1:52: the result of * is out of range\n" +
		"    const total = o => o.lines.map(l => l.qty * l.price).sum();\n" +
		"                                        ^^^^^^^^^^^^^^^\n" +
		"  in map, element 1, at 1:20-1:53\n" +
		"  in total, at 2:17-2:25\n" +
		"  in map, element 2, at 2:1-2:26\n"
	assert.Equal(t, risor.RenderDetail(ds, orders), want)
	assert.Equal(t, risor.RenderLine(ds), "error[number-range] 1:37-1:52: the result of * is out of range\n")
	assert.True(t, strings.HasSuffix(risor.RenderJSON(ds), `"trace":[{"index":2,"name":"map","span":[2,1,2,26]},`+
		`{"index":null,"name":"total","span":[2,17,2,25]},{"index":1,"name":"map","span":[1,20,1,53]}]}`+"\n"))
}

func sp(s risor.Span) string {
	a := s.Array()
	return fmt.Sprintf("%d:%d-%d:%d", a[0], a[1], a[2], a[3])
}

func idx(n int) string {
	if n < 0 {
		return "-1"
	}
	return string(rune('0' + n))
}

func TestTraceRows(t *testing.T) {
	env, _ := risor.NewEnv()
	for _, c := range []struct{ src, trace string }{
		{"const f = x => 1 / x; f(0)", "f 1:23-1:27 -1"},
		{"[1, 0].map(x => 1 / x)", "map 1:1-1:23 1"},
		{"const f = x => 1 / x; f(f(0))", "f 1:25-1:29 -1"},
		{"[[1], 2].flatMap(x => x)", ""},
		{"try { [0].map(x => 1 / x) } catch { [1, 0].map(x => 1 / x) }", "map 1:37-1:59 1"},
	} {
		ds := failure(t, env, c.src, nil)
		var got []string
		for _, e := range ds[0].Trace {
			got = append(got, e.Name+" "+sp(e.Span)+" "+idx(e.Index))
		}
		assert.Equal(t, strings.Join(got, ", "), c.trace, c.src)
		assert.True(t, ds[0].Trace != nil, "a run diagnostic has a trace, maybe empty")
	}
	// A run that does not fail, then one that fails: the stack of calls is
	// reused, and the second trace holds only its own calls.
	x, _ := risor.Parse("const f = x => 1 / x; [1, 2].map(y => f(y - n))")
	env2, _ := risor.NewEnv(risor.Var("n", "int"))
	p, err := env2.Compile(x)
	assert.NoError(t, err)
	_, err = p.Run(context.Background(), risor.Map{"n": 0.0})
	assert.NoError(t, err)
	_, err = p.Run(context.Background(), risor.Map{"n": 2.0})
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, len(e.Diagnostics[0].Trace), 2)
	assert.Equal(t, e.Diagnostics[0].Trace[0].Index, 1)
}

func TestConformAll(t *testing.T) {
	env, _ := risor.NewEnv()
	typ, err := env.ParseType("{id: string, lines: {qty: int, sku: string}[]}")
	assert.NoError(t, err)
	v := map[string]any{"id": 7.0, "lines": []any{
		map[string]any{"sku": "x", "qty": 1.0}, map[string]any{"sku": "y", "qty": "2"}}, "note": 1.0}
	ds := risor.ConformAll(v, typ, 3)
	assert.Equal(t, len(ds), 3)
	assert.Equal(t, ds[0].Path, []any{"id"})
	assert.Equal(t, ds[1].Path, []any{"lines", 1.0, "qty"})
	assert.Equal(t, ds[2].Message, `the type does not allow key "note"`)
	assert.Equal(t, len(risor.ConformAll(v, typ, 0)), 1) // a maximum less than 1 is 1
	first := conformErr(t, v, typ)
	assert.Equal(t, first.Path, ds[0].Path)
	assert.Equal(t, first.Message, ds[0].Message)
	assert.Equal(t, len(risor.ConformAll(map[string]any{"id": "a", "lines": []any{}}, typ, 5)), 0)
	assert.Equal(t, risor.RenderLine(ds[1:2]), "error[not-conforming]: expected int, found \"2\" at .lines[1].qty\n")
}

func TestRenderData(t *testing.T) {
	d := risor.Diagnostic{Phase: risor.PhaseRun, Code: "failed", Message: "m", HasData: true}
	js := func(data any) string {
		d.Data = data
		s := risor.RenderJSON([]risor.Diagnostic{d})
		i := strings.Index(s, `"data":`)
		j := strings.Index(s, `,"message"`)
		return s[i+7 : j]
	}
	assert.Equal(t, js(map[string]any{"b": []any{1.0, "x"}, "a": nil}), `{"a":null,"b":[1,"x"]}`)
	// Deeper than 200, or more than 10,000 code points: null.
	var deep any = 1.0
	for i := 0; i < 201; i++ {
		deep = []any{deep}
	}
	assert.Equal(t, js(deep), "null")
	assert.Equal(t, js(strings.Repeat("é", 9999)), "null")
	assert.Equal(t, js(strings.Repeat("é", 9998)), `"`+strings.Repeat("é", 9998)+`"`)
	// A list shared many times is bounded by the count, not walked whole.
	shared := make([]any, 1000)
	for i := range shared {
		shared[i] = 1.0
	}
	big := make([]any, 1000)
	for i := range big {
		big[i] = shared
	}
	assert.Equal(t, js(big), "null")
}

func TestCheckMessages(t *testing.T) {
	env, _ := risor.NewEnv(risor.Var("o", "{id: string, qty: int}"), risor.Var("n", "int | null"))
	x, _ := risor.Parse("o.nmae + (n + 1)")
	_, err := env.Compile(x)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	var msgs []string
	for _, d := range e.Diagnostics {
		msgs = append(msgs, d.Message)
	}
	assert.Equal(t, msgs, []string{`{id: string, qty: int} has no field "nmae"`, "n may be null; its type is int | null"})
	assert.Equal(t, risor.RenderLine(e.Diagnostics[:1]), "error[no-field] 1:3-1:7: {id: string, qty: int} has no field \"nmae\"\n")
}

func TestAssignmentAfterAnEntryKey(t *testing.T) {
	// A shorthand entry and a key are not operands (f08-errors.md 1.1), so
	// a = after one is not an assignment (E7, note 1). After a shorthand
	// entry in a lambda's map body the block row E23 holds. After a string
	// key or a computed key the = stands where the : should: a map with the
	// wrong separator, unexpected-token with the template key (3.4).
	key := "a map key is followed by :, found ="
	for _, c := range []struct{ src, code, span, msg string }{
		{"[1].map(x => { a = 1 })", "block-without-const", "[1, 14, 1, 15]", ""},
		{`f(x => { "a" = 1 })`, "unexpected-token", "[1, 14, 1, 15]", key},
		{"f(x => { [k] = 1 })", "unexpected-token", "[1, 14, 1, 15]", key},
		{`{"a" = 1}`, "unexpected-token", "[1, 6, 1, 7]", key},
		{`const {"a" = 1} = m; 1`, "unexpected-token", "[1, 12, 1, 13]", key},
		{`{"a" 1}`, "unexpected-token", "[1, 6, 1, 7]", "a map key is followed by :, found 1"},
		{"f(x => { a: 1, b = 2 })", "unexpected-token", "[1, 18, 1, 19]", "unexpected ="},
		{"{a = 1}", "unexpected-token", "[1, 4, 1, 5]", "unexpected ="},
		// A value or a spread's operand is an operand.
		{"f(x => { a: b = 1 })", "assignment", "[1, 15, 1, 16]", ""},
		{"f(x => { ...a = 1 })", "assignment", "[1, 15, 1, 16]", ""},
		{"f(a = 1)", "assignment", "[1, 5, 1, 6]", ""},
	} {
		_, err := risor.Parse(c.src)
		var e *risor.Error
		assert.True(t, errors.As(err, &e), c.src)
		assert.Equal(t, e.Diagnostics[0].Code, c.code, c.src)
		assert.Equal(t, e.Diagnostics[0].Span.String(), c.span, c.src)
		if c.msg != "" {
			assert.Equal(t, e.Diagnostics[0].Message, c.msg, c.src)
		}
	}
}

func TestFuzzRoundSevenParse(t *testing.T) {
	for _, c := range []struct{ src, code, span, msg string }{
		// The start of a map entry is not where an operand must begin
		// (f08-errors.md 2.2): no C8, C9 or E20 there.
		{`{n"`, "unexpected-token", "[1, 2, 1, 3]", ""},
		{"{<", "unexpected-token", "[1, 2, 1, 3]", ""},
		{"try{\"\"}catch{{r`", "unexpected-token", "[1, 15, 1, 16]", ""},
		{`n"x"`, "string-prefix", "[1, 1, 1, 2]", ""},
		{`{a: n"x"}`, "string-prefix", "[1, 5, 1, 6]", ""},
		{"<int>x", "angle-cast", "[1, 1, 1, 2]", ""},
		// E7 needs a complete operand of an Expr: a match literal is not
		// one, a pattern's default is.
		{"match 0{1=", "unexpected-token", "[1, 10, 1, 11]", ""},
		{"match 0{_=", "unexpected-token", "[1, 10, 1, 11]", ""},
		{"const[_=t=", "assignment", "[1, 10, 1, 11]", ""},
		{"const {a = t = 1} = x; a", "assignment", "[1, 14, 1, 15]", ""},
		{"const [...r = 1] = x; r", "unexpected-token", "[1, 13, 1, 14]", ""},
		// The end of input inside a computed key: the key is the innermost
		// production, so E23 does not apply (2.1 item 7).
		{"y(()=>{[", "unexpected-token", "[1, 9, 1, 9]", ""},
		{"y(()=>{[1] 2", "block-without-const", "[1, 7, 1, 8]", ""},
		// The innermost literal still open names the message (3.4).
		{"\"\"\"\r${", "unterminated-string", "[1, 1, 1, 7]", "the text block has no closing delimiter"},
		{"\"\"\"\n${", "unterminated-string", "[1, 1, 2, 3]", "the text block has no closing delimiter"},
		{"`a${", "unterminated-string", "[1, 1, 1, 5]", "the template has no closing backtick"},
	} {
		_, err := risor.Parse(c.src)
		var e *risor.Error
		assert.True(t, errors.As(err, &e), c.src)
		assert.Equal(t, e.Diagnostics[0].Code, c.code, c.src)
		assert.Equal(t, e.Diagnostics[0].Span.String(), c.span, c.src)
		if c.msg != "" {
			assert.Equal(t, e.Diagnostics[0].Message, c.msg, c.src)
		}
	}
}

func TestIndexReadIsNoPath(t *testing.T) {
	// A fact applies to a path only (05-types.md 10.4): a string index on a
	// value that is not a path takes no fact, whatever slot a fact names.
	env, err := risor.NewEnv()
	assert.NoError(t, err)
	x, _ := risor.Parse(`const m = a; m is int[] ? 0 : {"a-b": []}?.["a-b"].filter()`)
	_, err = env.Check(x)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	codes := []string{}
	for _, d := range e.Diagnostics {
		codes = append(codes, d.Code)
	}
	assert.Equal(t, codes, []string{"undefined-name", "arity"})
	typed, err := risor.NewEnv(risor.Declare("{x: int | string}"))
	assert.NoError(t, err)
	x, _ = risor.Parse(`const m = x; m is int ? 0 : {"a": 1}["a"]`)
	r, err := typed.Check(x)
	assert.NoError(t, err)
	assert.Equal(t, r.Type, "int")
}
