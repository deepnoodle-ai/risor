package types

import (
	"testing"

	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
	"github.com/deepnoodle-ai/wonton/assert"
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
