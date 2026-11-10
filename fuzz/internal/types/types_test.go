package types

import (
	"encoding/json"
	"testing"
)

func TestPrintParse(t *testing.T) {
	for _, s := range []string{
		"any", "never", "int", "float | null", "never[]", "int[][]", "(int | string)[]",
		`"fail" | "pass" | null`, "int[] | null", "(int | null)[]", "{action: string, ...}",
		"{note?: string, owner: string | null}", "map<any>", "map<float>", "{}",
		`{"a-b": int, b: {c: bool[]}, ...int}`, "bool | int | string | Tok | null",
	} {
		ty, err := Parse(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if got := ty.String(); got != s {
			t.Errorf("%s printed as %s", s, got)
		}
	}
	ty, _ := Parse("string | int | null | float")
	if ty.String() != "float | string | null" {
		t.Errorf("got %s", ty)
	}
}

func TestConforms(t *testing.T) {
	cases := []struct {
		typ, val string
		want     bool
	}{
		{"int", "2.0", true}, {"int", "2.5", false}, {"int", "9007199254740992", false},
		{"float", "2.5", true}, {"string", `"42"`, true}, {"int", `"42"`, false},
		{`"a" | "b"`, `"b"`, true}, {`"a" | "b"`, `"c"`, false},
		{"{id: string, note?: string}", `{"id": "a", "note": null}`, false},
		{"{id: string, note?: string}", `{"id": "a"}`, true},
		{"{id: string}", `{"id": "a", "x": 1}`, false},
		{"{id: string, ...}", `{"id": "a", "x": 1}`, true},
		{"map<int>", `{"x": 1, "y": 2}`, true}, {"map<int>", `{"x": "1"}`, false},
		{"never[]", `[]`, true}, {"int[]", `[1, null]`, false}, {"(int | null)[]", `[1, null]`, true},
		{"Tok", `{"$opaque": {"type": "Tok", "id": 1}}`, true},
		{"map<any>", `{"$opaque": {"type": "Tok", "id": 1}}`, false},
		{"any", `{"$opaque": {"type": "Tok", "id": 1}}`, true},
		{"never", `null`, false},
	}
	for _, c := range cases {
		ty, err := Parse(c.typ)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal([]byte(c.val), &v); err != nil {
			t.Fatal(err)
		}
		if got := Conforms(v, ty); got != c.want {
			t.Errorf("%s against %s: got %v", c.val, c.typ, got)
		}
	}
}

func TestOfValue(t *testing.T) {
	var v any
	_ = json.Unmarshal([]byte(`{"a": [1, 2.5, null], "b": [{"x": 1}, {"y": "s"}], "c": []}`), &v)
	want := "{a: (float | null)[], b: {x?: int, y?: string}[], c: never[]}"
	if got := OfValue(v).String(); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if !Conforms(v, OfValue(v)) {
		t.Error("a value does not conform to its own type")
	}
}

func TestUnions(t *testing.T) {
	for _, c := range []struct{ in, out string }{
		{`{kind: "rect", w: float, h: float} | {kind: "circle", r: float} | null`, `{kind: "circle", r: float} | {h: float, kind: "rect", w: float} | null`},
		{`{a: "x", b: "q"} | {a: "y", b: "p"}`, `{a: "x", b: "q"} | {a: "y", b: "p"}`},
		{`({kind: "a"} | {kind: "b"})[]`, `({kind: "a"} | {kind: "b"})[]`},
		{`({kind: "a"} | {kind: "b"}) | {kind: "c"}`, `{kind: "a"} | {kind: "b"} | {kind: "c"}`},
		{`{s: {kind: "a"} | {kind: "b", ...}}`, `{s: {kind: "a"} | {kind: "b", ...}}`},
	} {
		ty, err := Parse(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if ty.String() != c.out {
			t.Errorf("%s printed as %s", c.in, ty)
		}
	}
	for _, bad := range []string{`{a: int} | {b: string}`, `{kind: "a" | "b"} | {kind: "b"}`, `{kind: "a"} | {kind?: "b"}`, `{kind: "a"} | map<int>`} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
	shape, _ := Parse(`{kind: "circle", r: float} | {kind: "rect", h: float, w: float}`)
	for val, want := range map[string]bool{
		`{"kind": "rect", "w": 2, "h": 3}`: true, `{"kind": "rect", "w": 2}`: false,
		`{"kind": "oval", "r": 1}`: false, `{"r": 1}`: false, `{"kind": "circle", "r": 1, "note": "x"}`: false,
		`"circle"`: false, `{"kind": "circle", "r": 1}`: true,
	} {
		var v any
		_ = json.Unmarshal([]byte(val), &v)
		if Conforms(v, shape) != want {
			t.Errorf("%s: want %v", val, want)
		}
	}
}
