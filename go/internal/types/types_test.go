package types

import (
	"testing"

	"github.com/deepnoodle-ai/risor/go/v3/internal/assert"
	"github.com/deepnoodle-ai/risor/go/v3/internal/typetext"
)

var opaque = map[string]bool{"Time": true, "Date": true}

func parse(t *testing.T, s string) *Type {
	t.Helper()
	tt, err := typetext.ParseType(s)
	assert.NoError(t, err, s)
	ty, err := FromText(tt, opaque, false)
	assert.NoError(t, err, s)
	return ty
}

// Printing reads back as the same type (3.9).
func TestPrinting(t *testing.T) {
	for in, out := range map[string]string{
		"int":                                   "int",
		"null | int":                            "int | null",
		"int | float":                           "float",
		`"b" | "a" | null`:                      `"a" | "b" | null`,
		`"a" | string`:                          "string",
		"int[] | string[]":                      "(int | string)[]",
		`("a" | "b")[]`:                         `("a" | "b")[]`,
		"(int | null)[]":                        "(int | null)[]",
		"int[] | null":                          "int[] | null",
		"never[]":                               "never[]",
		"int[][]":                               "int[][]",
		"{owner: string | null, note?: string}": "{note?: string, owner: string | null}",
		"{action: string, ...}":                 "{action: string, ...}",
		"{a: int, ...float}":                    "{a: int, ...float}",
		"map<float>":                            "map<float>",
		"map<any>":                              "map<any>",
		"{}":                                    "{}",
		"Time | null | Date":                    "Date | Time | null",
		`{"a-b": int}`:                          `{"a-b": int}`,
		"bool | string | int | Time | null | int[]": "bool | int | string | int[] | Time | null",
		"any | int":            "any",
		"never | int":          "int",
		"never":                "never",
		`{x: "a | b" | "c"}[]`: `{x: "a | b" | "c"}[]`,
	} {
		ty := parse(t, in)
		assert.Equal(t, ty.String(), out, in)
		assert.True(t, Equal(parse(t, ty.String()), ty), "round trip of %s", in)
	}
}

func TestTextErrors(t *testing.T) {
	for _, s := range []string{"{a: int} | {b: int}", "{a: int} | map<int>", "Order", "{a: {b: int, b: int}}", "T"} {
		tt, err := typetext.ParseType(s)
		if err != nil {
			continue
		}
		_, err = FromText(tt, opaque, false)
		assert.Error(t, err, s)
	}
}

var samples = []string{
	"never", "any", "null", "bool", "int", "float", "string", `"a"`, `"a" | "b"`, `"" | "x"`,
	"int[]", "string[]", "never[]", "(int | null)[]", "{a: int}", "{a?: int}", "{a: int, b: string}",
	"{a: int, ...}", "map<int>", "{}", "Time", "int | null", "string | int[] | null",
	"{a: string, ...int}", "Time | Date", "bool | {x: int}",
}

// The laws of 4.1 and 4.4 on hand-made types.
func TestJoinLaws(t *testing.T) {
	var ts []*Type
	for _, s := range samples {
		ts = append(ts, parse(t, s))
	}
	for _, a := range ts {
		assert.True(t, Equal(Join(a, a), a), "idempotent %s", a)
		assert.True(t, Equal(Join(a, Never), a), "never %s", a)
		assert.True(t, Equal(Join(Truthy(a), Falsy(a)), a),
			"truthy+falsy %s", a)
		for _, b := range ts {
			assert.True(t, Equal(Join(a, b), Join(b, a)), "commutative %s %s", a, b)
			ok, _ := Fits(a, Join(a, b))
			assert.True(t, ok || a.rec != nil || b.rec != nil, "%s fits Join(%s, %s)", a, a, b)
			for _, c := range ts {
				assert.True(t, Equal(Join(Join(a, b), c), Join(a, Join(b, c))), "associative %s %s %s", a, b, c)
			}
		}
	}
}

func TestFits(t *testing.T) {
	yes := [][2]string{
		{"int", "float"}, {`"a"`, "string"}, {`"a"`, `"a" | "b"`}, {"never", "int"}, {"int", "any"},
		{"{a: int}", "map<int>"}, {"{a: int}", "{a: int, ...}"}, {"{a: int}", "{a: int, b?: string}"},
		{"int[]", "float[]"}, {"never[]", "int[]"}, {"null", "int | null"}, {"Time", "Time | null"},
	}
	no := [][2]string{
		{"float", "int"}, {"string", `"a"`}, {"{a: int, b: int}", "{a: int}"}, {"{a?: int}", "{a: int}"},
		{"int | null", "int"}, {"Time", "Date"}, {"int[]", "string[]"}, {"any[]", "int"},
	}
	for _, p := range yes {
		ok, used := Fits(parse(t, p[0]), parse(t, p[1]))
		assert.True(t, ok, "%s fits %s", p[0], p[1])
		assert.False(t, used)
	}
	for _, p := range no {
		ok, _ := Fits(parse(t, p[0]), parse(t, p[1]))
		assert.False(t, ok, "%s does not fit %s", p[0], p[1])
	}
	ok, used := Fits(parse(t, "any[]"), parse(t, "int[]"))
	assert.True(t, ok)
	assert.True(t, used)
}

func TestJoinRecords(t *testing.T) {
	j := Join(parse(t, "{a: int, b: string}"), parse(t, "{a: float, c: bool}"))
	assert.Equal(t, j.String(), "{a: float, b?: string, c?: bool}")
	j = Join(parse(t, "{}"), parse(t, "map<string>"))
	assert.Equal(t, j.String(), "map<string>")
	j = Join(parse(t, "{a: int}"), parse(t, "map<string>"))
	assert.Equal(t, j.String(), "{a?: int | string, ...string}")
}

func TestMerge(t *testing.T) {
	m := Merge(parse(t, "{a: int}").Record(), parse(t, "{b: string}"))
	assert.Equal(t, RecType(m).String(), "{a: int, b: string}")
	m = Merge(parse(t, "{a: int}").Record(), MapOf(Lit("x")))
	assert.Equal(t, RecType(m).String(), `{a: int | "x", ..."x"}`)
	m = Merge(parse(t, "{a: int, b: int}").Record(), parse(t, "{a?: string}"))
	assert.Equal(t, RecType(m).String(), "{a: int | string, b: int}")
	m = Merge(parse(t, "{a: int}").Record(), Any)
	assert.Equal(t, RecType(m).String(), "{a: any, ...}")
}

func TestTruthyFalsy(t *testing.T) {
	for in, want := range map[string][2]string{
		"null":        {"never", "null"},
		`"" | "a"`:    {`"a"`, `""`},
		"string":      {"string", `""`},
		"int[]":       {"int[]", "never[]"},
		"never[]":     {"never", "never[]"},
		"{id: int}":   {"{id: int}", "never"},
		"{id?: int}":  {"{id?: int}", "{}"},
		"{}":          {"never", "{}"},
		"Time":        {"Time", "never"},
		"bool | null": {"bool", "bool | null"},
		"any":         {"any", "any"},
	} {
		ty := parse(t, in)
		assert.Equal(t, Truthy(ty).String(), want[0], "truthy %s", in)
		assert.Equal(t, Falsy(ty).String(), want[1], "falsy %s", in)
	}
}

func TestHelpers(t *testing.T) {
	assert.False(t, Overlap(Int, String))
	assert.True(t, Overlap(Int, Float))
	assert.False(t, Overlap(Lit("a"), Lit("b")))
	assert.True(t, Overlap(Lit("a"), String))
	assert.True(t, Overlap(Any, Int))
	assert.False(t, Overlap(Never, Any))
	assert.True(t, HasOpaque(parse(t, "{a: Time[]}")))
	assert.True(t, Finite(parse(t, `"a" | null | bool`)))
	assert.False(t, Finite(parse(t, "int")))
	assert.Equal(t, Depth(parse(t, "int[][]")), 2)
	assert.Equal(t, Depth(parse(t, "{a: int[]}")), 2)
}

func TestSignatures(t *testing.T) {
	s, err := typetext.ParseSig("(a: T, b: T, ...rest: T[]) => T where T: ordered")
	assert.NoError(t, err)
	sig, err := FromSig(nil, s, nil)
	assert.NoError(t, err)
	assert.Equal(t, sig.Min(), 2)
	assert.Equal(t, sig.Max(), -1)
	assert.Equal(t, sig.ParamAt(5).Type.String(), "T")
	sol := map[string]*Type{"T": Int}
	assert.Equal(t, Subst(parse0(t, "(x: T) => T[] | null").Result, sol).String(), "int[] | null")
	s, _ = typetext.ParseSig("(...r: int) => int")
	_, err = FromSig(nil, s, nil)
	assert.Error(t, err)
}

func parse0(t *testing.T, s string) *Sig {
	ts, err := typetext.ParseSig(s)
	assert.NoError(t, err)
	sig, err := FromSig(nil, ts, nil)
	assert.NoError(t, err)
	return sig
}

func TestSize(t *testing.T) {
	assert.Equal(t, Size(Never), 1)
	assert.Equal(t, Size(Any), 1)
	assert.Equal(t, Size(Lit("a", "b", "c")), 3)
	assert.Equal(t, Size(Join(Join(Int, Null), Lit("a", "b"))), 4)
	assert.Equal(t, Size(List(Int)), 2)
	// A rest of never counts 0; a rest of any counts 1.
	assert.Equal(t, Size(Rec([]Field{{Key: "a", Type: Int}}, nil)), 2)
	assert.Equal(t, Size(Rec([]Field{{Key: "a", Type: Int}}, Any)), 3)
	// The doubling chain: s(k) = 2^(k+2) - 1, kept with sharing and
	// saturated above the limit, in constant work per type.
	a := Rec([]Field{{Key: "l", Type: Int}, {Key: "r", Type: Int}}, nil)
	for k := 1; k <= 40; k++ {
		a = Rec([]Field{{Key: "l", Type: a}, {Key: "r", Type: a}}, nil)
		want := 1<<(k+2) - 1
		if want > MaxSize {
			want = MaxSize + 1
		}
		assert.Equal(t, Size(a), want)
	}
	// Join of a type with itself shares it.
	assert.True(t, Join(a, a) == a)
}

func TestJoinAll(t *testing.T) {
	ts := []*Type{Lit("b"), Int, Lit("a", "c"), Null, List(Int), List(Float)}
	assert.Equal(t, JoinAll(ts).String(), `int | "a" | "b" | "c" | float[] | null`)
	acc := Never
	for _, x := range ts {
		acc = Join(acc, x)
	}
	assert.Equal(t, JoinAll(ts).String(), acc.String())
	assert.Equal(t, JoinAll([]*Type{Lit("a"), String}).String(), "string")
	assert.Equal(t, JoinAll([]*Type{Int, Any}).String(), "any")
	assert.Equal(t, JoinAll(nil).String(), "never")
}

// family is a recursive type of thirteen definitions, each but the last
// holding the next twice: two families of one structure take more than
// MaxPairs pairs to compare (12.5 item 7).
func family(p string) *Named {
	ns := make([]*Named, 13)
	for i := range ns {
		ns[i] = NewNamed(p + string(rune('A'+i)))
	}
	for i, n := range ns {
		if i == 12 {
			n.SetDef(Rec([]Field{{Key: "a", Type: Int}, {Key: "s", Type: Ref(n)}}, nil))
			continue
		}
		next := Ref(ns[i+1])
		n.SetDef(Rec([]Field{{Key: "a", Type: next}, {Key: "b", Type: next}, {Key: "s", Type: Ref(n)}}, nil))
	}
	return ns[0]
}

func TestJoinAllFoldsWithNestedRefs(t *testing.T) {
	a, b := family("X"), family("Y")
	assert.True(t, Guard(func() { Join(List(Ref(a)), List(Ref(b))) }))
	// 12.5 item 4: with a reference the Join of more than two types folds
	// from the left, so any[] meets each list in turn and no pair of the
	// two families is compared. The reference here is inside a list.
	var r *Type
	assert.False(t, Guard(func() { r = JoinAll([]*Type{List(Any), List(Ref(a)), List(Ref(b))}) }))
	assert.Equal(t, r.String(), "any[]")
}

func TestEqualOrder(t *testing.T) {
	// 12.5 item 6: Equal visits the alternatives in the order of printing,
	// and two records' keys in code point order, each field's mark before
	// its type. A difference met later does not hide the pairs before it.
	a, b := family("X"), family("Y")
	limit := func(x, y *Type) bool { return Guard(func() { Equal(x, y) }) }
	// The lists are compared before null.
	assert.True(t, limit(WithNull(List(Ref(a))), List(Ref(b))))
	// Each field's type before the next field's mark.
	rec := func(n *Named, zOpt bool) *Type {
		return Rec([]Field{{Key: "a", Type: Ref(n)}, {Key: "z", Type: Int, Optional: zOpt}}, nil)
	}
	assert.True(t, limit(rec(a, false), rec(b, true)))
	// And before a key that only one record has.
	assert.True(t, limit(Rec([]Field{{Key: "a", Type: Ref(a)}}, nil), rec(b, false)))
	// Opaque names differ after the lists.
	assert.True(t, limit(Join(List(Ref(a)), parse(t, "Time")), List(Ref(b))))
}

func TestTruthyNonnullKeepReferences(t *testing.T) {
	// 12.5 item 8: Truthy and nonnull unfold only the references that need
	// it; the others stay by name.
	a, b := NewNamed("A"), NewNamed("B")
	a.SetDef(WithNull(Rec([]Field{{Key: "next", Type: Ref(a), Optional: true}}, nil)))
	b.SetDef(List(Ref(b)))
	ab := withRefs(Ref(a), []*Named{b})
	assert.Equal(t, ab.String(), "A | B")
	assert.Equal(t, Truthy(ab).String(), "{next?: A} | B")
	assert.Equal(t, Nonnull(ab).String(), "{next?: A} | B")
	// A reference that needs nothing stays alone.
	assert.Equal(t, Truthy(Ref(b)).String(), "B")
	assert.Equal(t, Nonnull(Ref(b)).String(), "B")
	assert.Equal(t, Nonnull(WithNull(Ref(b))).String(), "B")
}

func TestDisjointSameReference(t *testing.T) {
	// 12.5 item 5: the same reference on both sides is equal at once, so
	// not disjoint, with no pair entered.
	n := NewNamed("Node")
	n.SetDef(Rec([]Field{{Key: "next", Type: Ref(n), Optional: true}}, nil))
	c := &rctx{}
	assert.False(t, c.disjoint(Ref(n), Ref(n)))
	assert.Equal(t, c.n, 0)
	c = &rctx{}
	assert.False(t, c.disjoint(List(Ref(n)), List(Ref(n))))
	assert.Equal(t, c.n, 0)
}

func TestWithoutTypeThroughReference(t *testing.T) {
	// without(X, T) (F4 6.2) takes away the strings T takes, also through
	// a reference of T.
	n := NewNamed("Node")
	n.SetDef(Join(Rec([]Field{{Key: "next", Type: Ref(n), Optional: true}}, nil), Lit("a")))
	assert.Equal(t, WithoutType(Lit("a", "b"), Ref(n)).String(), `"b"`)
	assert.Equal(t, WithoutType(Lit("a"), Ref(n)).String(), "never")
	assert.Equal(t, WithoutType(Join(Lit("a", "b"), Int), Ref(n)).String(), `int | "b"`)
}
