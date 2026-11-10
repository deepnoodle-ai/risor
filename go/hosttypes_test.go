package risor_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F1 6.2: types from Go types and Go values presented to a run.

type money struct{ cents int }

func (m money) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%d.%02d", m.cents/100, m.cents%100)), nil
}

type badText struct{}

func (badText) MarshalText() ([]byte, error) { return nil, errors.New("no") }

type Base struct {
	ID      int `json:"id"`
	Created string
}

type Audit struct {
	By string `json:"by"`
}

type item struct {
	SKU   string `json:"sku"`
	Qty   int    `json:"qty"`
	Price money  `json:"price"`
}

type order struct {
	Base
	*Audit
	Items    []item            `json:"items"`
	Note     string            `json:"note,omitempty"`
	Ref      *string           `json:"ref"`
	Tags     map[string]string `json:"tags"`
	Owner    token             `json:"owner"`
	Secret   string            `json:"-"`
	Internal int               `risor:"-"`
	Renamed  int               `risor:"total" json:"sum"`
	When     time.Time         `json:"when,omitzero"`
	Weight   float32           `json:"weight"`
	Any      any               `json:"any"`
	hidden   int
}

const orderType = "{Created: string, any: any, by?: string, id: int, " +
	"items: {price: string, qty: int, sku: string}[], note?: string, owner: Token, " +
	"ref: string | null, tags: map<string>, total: int, weight: float, when?: string}"

// tokenText is opaque and a text type: opaque comes first.
type tokenText struct{}

func (tokenText) OpaqueType() string           { return "Token" }
func (tokenText) MarshalText() ([]byte, error) { return []byte("t"), nil }

type node struct {
	Next *node `json:"next"`
}

// Dominance as encoding/json: the shallower field wins; of two at one
// depth, the tagged one; else neither.
type left struct {
	A int `json:"a"`
	B int
	C int
}

type right struct {
	A int
	B int
	D int
}

type dom struct {
	left
	Left2 left `json:"l2"`
	right2
	C string
}

type right2 = right

type deep = [1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1][1]int

func typeOf[T any](t *testing.T) string {
	t.Helper()
	ty, err := risor.TypeOf[T]()
	assert.NoError(t, err)
	return ty.String()
}

func TestTypeOfRows(t *testing.T) {
	assert.Equal(t, typeOf[token](t), "Token")
	assert.Equal(t, typeOf[tokenText](t), "Token")
	assert.Equal(t, typeOf[money](t), "string")
	assert.Equal(t, typeOf[time.Time](t), "string")
	assert.Equal(t, typeOf[bool](t), "bool")
	assert.Equal(t, typeOf[string](t), "string")
	assert.Equal(t, typeOf[int8](t), "int")
	assert.Equal(t, typeOf[uint64](t), "int")
	assert.Equal(t, typeOf[float32](t), "float")
	assert.Equal(t, typeOf[[]string](t), "string[]")
	assert.Equal(t, typeOf[[3]int](t), "int[]")
	assert.Equal(t, typeOf[map[string]float64](t), "map<float>")
	assert.Equal(t, typeOf[*int](t), "int | null")
	assert.Equal(t, typeOf[*token](t), "Token | null")
	assert.Equal(t, typeOf[*money](t), "string | null")
	assert.Equal(t, typeOf[any](t), "any")
	assert.Equal(t, typeOf[[]any](t), "any[]")
	assert.Equal(t, typeOf[order](t), orderType)
	assert.Equal(t, typeOf[*order](t), orderType+" | null")
	assert.Equal(t, typeOf[struct{}](t), "{}")

	for name, f := range map[string]func() (risor.Type, error){
		"chan":        func() (risor.Type, error) { return risor.TypeOf[chan int]() },
		"func":        func() (risor.Type, error) { return risor.TypeOf[func()]() },
		"int key":     func() (risor.Type, error) { return risor.TypeOf[map[int]string]() },
		"complex":     func() (risor.Type, error) { return risor.TypeOf[complex128]() },
		"cycle":       func() (risor.Type, error) { return risor.TypeOf[node]() },
		"too deep":    func() (risor.Type, error) { return risor.TypeOf[deep]() },
		"inner chan":  func() (risor.Type, error) { return risor.TypeOf[struct{ C chan int }]() },
		"slice cycle": func() (risor.Type, error) { return risor.TypeOf[[]node]() },
	} {
		_, err := f()
		var e *risor.Error
		assert.True(t, errors.As(err, &e), name)
		assert.Equal(t, e.Diagnostics[0].Code, "bad-type", name)
	}
	// 64 levels is allowed.
	assert.Equal(t, typeOf[[1][1][1][1][1][1][1][1]int](t), "int[][][][][][][][]")
}

func TestTypeOfDominance(t *testing.T) {
	// a: left's tagged A beats right's untagged A. b: two untagged, dropped.
	// C: dom's own C is shallower. A and D: only right has them.
	assert.Equal(t, typeOf[dom](t), "{A: int, C: string, D: int, a: int, l2: {B: int, C: int, a: int}}")
}

func TestStructPresentation(t *testing.T) {
	ty, err := risor.TypeOf[order]()
	assert.NoError(t, err)
	env, err := risor.NewEnv(risor.VarType("o", ty), risor.OpaqueType{Name: "Token"})
	assert.NoError(t, err)
	ref := "r1"
	o := order{
		Base: Base{ID: 7, Created: "today"}, Items: []item{{SKU: "a", Qty: 2, Price: money{150}}},
		Ref: &ref, Owner: token{1}, Secret: "s", Internal: 3, Renamed: 9, Weight: 1.5, hidden: 4,
	}
	eval := func(src string, v any) any {
		t.Helper()
		x, err := risor.Parse(src)
		assert.NoError(t, err)
		p, err := env.Compile(x)
		assert.NoError(t, err, src)
		out, err := p.Run(context.Background(), risor.Map{"o": v})
		assert.NoError(t, err, src)
		if f, ok := risor.AsFloat(out); ok {
			return f
		}
		return out
	}
	// Names: risor tag, json tag, Go name; promoted fields; skipped fields.
	assert.Equal(t, eval("o.id", o), 7.0)
	assert.Equal(t, eval("o.Created", o), "today")
	assert.Equal(t, eval("o.total", o), 9.0)
	assert.Equal(t, eval(`"Secret" in o || "Internal" in o || "sum" in o || "hidden" in o`, o), false)
	// A text type is its text; a pointer is its target.
	assert.Equal(t, eval("o.items[0].price", o), "1.50")
	assert.Equal(t, eval("o.ref", o), "r1")
	assert.Equal(t, eval("o.ref", order{}), nil)
	// omitempty and omitzero: absent when encoding/json would omit.
	assert.Equal(t, eval(`"note" in o`, o), false)
	assert.Equal(t, eval(`o?.note ?? "none"`, o), "none")
	o2 := o
	o2.Note = "n"
	o2.When = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	assert.Equal(t, eval("o?.note", o2), "n")
	assert.Equal(t, eval(`o?.when ?? "never"`, o), "never")
	assert.Equal(t, eval("o?.when", o2), "2026-01-02T03:04:05Z")
	// A nil embedded pointer: its fields are absent.
	assert.Equal(t, eval(`o?.by ?? "nobody"`, o), "nobody")
	o2.Audit = &Audit{By: "me"}
	assert.Equal(t, eval("o?.by", o2), "me")
	// Nil slices and maps are empty.
	assert.Equal(t, eval("len(o.tags)", o), 0.0)
	assert.Equal(t, eval("len(o.items)", order{}), 0.0)
	// An opaque field is opaque.
	assert.Equal(t, eval("o.owner", o), token{1})
	// A pointer to a struct reads as the struct.
	assert.Equal(t, eval("o.id", &o), 7.0)
	// The whole record, as JSON: the keys a run sees.
	assert.Equal(t, eval("len(o)", o), 9.0)
	assert.Equal(t, eval("len(o)", o2), 12.0)
}

func TestTextMarshalerError(t *testing.T) {
	// BB21: a MarshalText that fails, and a nil pointer, present null.
	env, err := risor.NewEnv(risor.Var("v", "string | null"))
	assert.NoError(t, err)
	x, _ := risor.Parse("[v, type(v)]")
	p, err := env.Compile(x)
	assert.NoError(t, err)
	for _, in := range []any{badText{}, &badText{}, (*money)(nil), (*badText)(nil)} {
		out, err := p.Run(context.Background(), risor.Map{"v": in})
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprint(out), "[<nil> null]", fmt.Sprintf("%T", in))
	}
	out, err := p.Run(context.Background(), risor.Map{"v": &money{5}})
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprint(out), "[0.05 string]")
}

func TestTypeFromSchema(t *testing.T) {
	ty, err := risor.TypeFromSchema([]byte(`{"type": "object", "properties": {"a": {"type": "integer"}}, "required": ["a"], "additionalProperties": false}`), risor.Closed)
	assert.NoError(t, err)
	assert.Equal(t, ty.String(), "{a: int}")
	ty, err = risor.TypeFromSchema(map[string]any{"type": "string"})
	assert.NoError(t, err)
	assert.Equal(t, ty.String(), "string")
	_, err = risor.TypeFromSchema([]byte(`{"type": "int"}`))
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
	_, err = risor.TypeFromSchema(true, risor.SchemaOption("open"))
	assert.True(t, errors.As(err, &e))
	// A declared variable by schema; a clash with type text.
	_, err = risor.NewEnv(risor.Var("x", "int"), risor.VarSchema("x", true))
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "name-clash")
	// A type naming an opaque type the environment does not declare.
	tt, _ := risor.TypeOf[token]()
	_, err = risor.NewEnv(risor.VarType("t", tt))
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "bad-type")
}

func describe(t *testing.T, env *risor.Env) string {
	t.Helper()
	d, err := env.Describe()
	assert.NoError(t, err)
	return string(d)
}

func TestDescriptionRoundTrip(t *testing.T) {
	native := func(c *risor.Call, args []any) (any, error) { return 1.0, nil }
	e1, err := risor.NewEnv(
		risor.Declare(`{order: {id: int, note?: string, "a-b"?: int, ...}, t: Time | null}`),
		risor.VarSchema("m", map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "number"}}),
		risor.OpaqueType{Name: "Time", Methods: map[string]risor.Function{
			"format": {Type: "(layout:string)=>string", Impl: native},
			"add":    {Type: "(d: Time, ...more: int[]) => Time !", Impl: native},
		}},
		risor.OpaqueType{Name: "Point"},
		risor.Function{Name: "rate", Type: "(code: string, n?: int) => float !", Impl: native},
		risor.Function{Name: "big", Type: "(a: T, b: T|null) => T where T: ordered", Impl: native},
		risor.Without("string.toUpperCase", "abs"),
		risor.RejectClasses("value", "dynamic"),
		risor.Limits{Steps: risor.NoLimit, Alloc: risor.ZeroLimit, Depth: 50},
	)
	assert.NoError(t, err)
	d1 := describe(t, e1)
	assert.Equal(t, d1, `{"functions":{"big":"(a: T, b: T | null) => T where T: ordered","rate":"(code: string, n?: int) => float !"},`+
		`"limits":{"allocation":0,"depth":50,"steps":null},`+
		`"opaque":{"Point":{},"Time":{"add":"(d: Time, ...more: int[]) => Time !","format":"(layout: string) => string"}},`+
		`"reject":["dynamic","value"],"removed":["abs","string.toUpperCase"],`+
		`"variables":{"m":"map<float>","order":"{\"a-b\"?: int, id: int, note?: string, ...}","t":"Time | null"},"version":"3.0"}`)

	decl, err := risor.Description([]byte(d1))
	assert.NoError(t, err)
	e2, err := risor.NewEnv(decl)
	assert.NoError(t, err)
	assert.Equal(t, describe(t, e2), d1)

	// The two check every program alike.
	for _, src := range []string{
		"order.id + 1",
		"order.note",
		"m.x * 2",
		"rate(\"x\")",
		"big(1, 2)",
		"t?.format(\"y\")",
		"\"a\".toUpperCase()",
		"order.missing",
		"[1, 2][order.id]",
	} {
		x, err := risor.Parse(src)
		assert.NoError(t, err)
		r1, err1 := e1.Check(x)
		r2, err2 := e2.Check(x)
		assert.Equal(t, fmt.Sprint(r1), fmt.Sprint(r2), src)
		assert.Equal(t, fmt.Sprint(err1), fmt.Sprint(err2), src)
	}
	// E2 has no implementations: compile fails while it declares one.
	x, _ := risor.Parse("1")
	_, err = e2.Compile(x)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "no-implementation")
	// Without functions and methods, it compiles.
	plain, _ := risor.NewEnv(risor.Declare("{a: int}"))
	decl, err = risor.Description([]byte(describe(t, plain)))
	assert.NoError(t, err)
	e3, err := risor.NewEnv(decl)
	assert.NoError(t, err)
	_, err = e3.Compile(x)
	assert.NoError(t, err)
}

func TestDescriptionErrors(t *testing.T) {
	base := `{"functions":{},"limits":{"allocation":1,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{},"version":"3.0"}`
	_, err := risor.Description([]byte(base))
	assert.NoError(t, err)
	for _, bad := range []string{
		`[]`, `{}`, `nope`,
		`{"functions":{},"limits":{"allocation":1,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{},"version":3}`,
		`{"functions":{},"limits":{"allocation":1,"depth":200},"opaque":{},"reject":[],"removed":[],"variables":{},"version":"3.0"}`,
		`{"functions":{},"limits":{"allocation":1,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{"a":1},"version":"3.0"}`,
		`{"functions":{},"limits":{"allocation":1,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{},"version":"3.0","named":{}}`,
	} {
		_, err := risor.Description([]byte(bad))
		assert.NotNil(t, err, bad)
	}
	// Declaration errors are diagnostics.
	for code, text := range map[string]string{
		"bad-type":            `{"functions":{},"limits":{"allocation":1,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{"a":"intt"},"version":"3.0"}`,
		"bad-signature":       `{"functions":{"f":"int"},"limits":{"allocation":1,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{},"version":"3.0"}`,
		"bad-class":           `{"functions":{},"limits":{"allocation":1,"depth":200,"steps":1},"opaque":{},"reject":["x"],"removed":[],"variables":{},"version":"3.0"}`,
		"bad-limit":           `{"functions":{},"limits":{"allocation":1.5,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{},"version":"3.0"}`,
		"unsupported-version": `{"functions":{},"limits":{"allocation":1,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{},"version":"9.0"}`,
		"bad-name":            `{"functions":{},"limits":{"allocation":1,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{"if":"int"},"version":"3.0"}`,
	} {
		decl, err := risor.Description([]byte(text))
		assert.NoError(t, err, code)
		_, err = risor.NewEnv(decl)
		var e *risor.Error
		assert.True(t, errors.As(err, &e), code)
		assert.Equal(t, e.Diagnostics[0].Code, code)
	}
}

func TestUnexportedEmbedded(t *testing.T) {
	ty, err := risor.TypeOf[dom]()
	assert.NoError(t, err)
	env, err := risor.NewEnv(risor.VarType("d", ty))
	assert.NoError(t, err)
	x, _ := risor.Parse("[d.a, d.A, d.D, d.C, d.l2.a]")
	p, err := env.Compile(x)
	assert.NoError(t, err)
	v := dom{left: left{A: 1}, right2: right{A: 2, D: 4}, C: "c", Left2: left{A: 5}}
	for _, in := range []any{v, &v} {
		out, err := p.Run(context.Background(), risor.Map{"d": in})
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprint(out), "[1 2 4 c 5]")
	}
}

type stringer struct{ s string }

func (x stringer) String() string { return x.s }

type withIfaces struct {
	Err  error        `json:"err"`
	Name fmt.Stringer `json:"name"`
	Val  any          `json:"val"`
}

func TestInterfaceTypes(t *testing.T) {
	// BB23: every interface type is any.
	assert.Equal(t, typeOf[fmt.Stringer](t), "any")
	assert.Equal(t, typeOf[error](t), "any")
	assert.Equal(t, typeOf[interface{ OpaqueType() string }](t), "any")
	assert.Equal(t, typeOf[withIfaces](t), "{err: any, name: any, val: any}")
	ty, _ := risor.TypeOf[withIfaces]()
	env, err := risor.NewEnv(risor.VarType("w", ty))
	assert.NoError(t, err)
	x, _ := risor.Parse("[w.err, w.name, w.val]")
	p, err := env.Compile(x)
	assert.NoError(t, err)
	// The value held is presented by its own type's rules; nil is null.
	out, err := p.Run(context.Background(), risor.Map{"w": withIfaces{}})
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprint(out), "[<nil> <nil> <nil>]")
	x, _ = risor.Parse("[len(w.name), w.val.price, type(w.err)]")
	p, err = env.Compile(x, risor.NoCheck)
	assert.NoError(t, err)
	out, err = p.Run(context.Background(), risor.Map{"w": withIfaces{Err: errors.New("e"), Name: stringer{"n"}, Val: item{Price: money{100}}}})
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprint(out), "[0 1.00 map]")
}

type inner struct {
	X int `json:"x"`
}

type viaUnexportedPointer struct {
	*inner
	Y int `json:"y"`
}

func TestUnexportedEmbeddedPointer(t *testing.T) {
	// BB22: a field reached through an unexported embedded pointer cannot be
	// read without unsafe, so it is not presented.
	assert.Equal(t, typeOf[viaUnexportedPointer](t), "{y: int}")
	ty, _ := risor.TypeOf[viaUnexportedPointer]()
	env, err := risor.NewEnv(risor.VarType("v", ty))
	assert.NoError(t, err)
	x, _ := risor.Parse("[len(v), v.y]")
	p, err := env.Compile(x)
	assert.NoError(t, err)
	out, err := p.Run(context.Background(), risor.Map{"v": viaUnexportedPointer{inner: &inner{X: 1}, Y: 2}})
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprint(out), "[1 2]")
}
