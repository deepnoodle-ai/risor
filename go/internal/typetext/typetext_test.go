package typetext

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestTypes(t *testing.T) {
	for _, c := range [][2]string{
		{`int`, `int`},
		{`int | string[]`, `int | string[]`},
		{`(int | string)[]`, `(int | string)[]`},
		{`int[][]`, `int[][]`},
		{`map<map<int>>`, `map<map<int>>`},
		{`"fail" | 'pass' | null`, `"fail" | "pass" | null`},
		{`{a: int, "b-c"?: string | null, if: bool,}`, `{a: int, b-c?: string | null, if: bool}`},
		{`{action: string, ...}`, `{action: string, ...}`},
		{`{...float}`, `{...float}`},
		{`{}`, `{}`},
		{`Time | null`, `Time | null`},
		{`any`, `any`},
		{`never[]`, `never[]`},
		{"/* c */ int // d", `int`},
		{`{a: int, b: int}`, `{a: int, b: int}`},
		{`{a: int} | {b: string}`, `{a: int} | {b: string}`},
	} {
		ty, err := ParseType(c[0])
		assert.NoError(t, err, c[0])
		assert.Equal(t, ty.String(), c[1], c[0])
	}
	for _, bad := range []string{
		`number`, `boolean`, `undefined`, `void`, `nil`, `object`, `Array<int>`,
		`int?`, `condition`, `unknown`, `T`, `int |`, `{a int}`, `{a: int,,}`,
		"`a`", `map<int`, `int[`, `(int`, `{...int, a: int}`, `"a`, `#`, `foo`,
		`_X`, `map`, `int int`, `{,}`, strings.Repeat("(", 65) + "int" + strings.Repeat(")", 65),
	} {
		_, err := ParseType(bad)
		assert.Error(t, err, bad)
	}
	_, err := ParseType(strings.Repeat("(", 64) + "int" + strings.Repeat(")", 64))
	assert.NoError(t, err)
	_, err = ParseType("int" + strings.Repeat("[]", 64))
	assert.NoError(t, err)
	_, err = ParseType("int" + strings.Repeat("[]", 65))
	assert.Error(t, err)
	_, err = ParseType(strings.Repeat("(", 32) + "int" + strings.Repeat("[]", 32) + strings.Repeat(")", 32))
	assert.NoError(t, err)
	_, err = ParseType(strings.Repeat("(", 32) + "int" + strings.Repeat("[]", 33) + strings.Repeat(")", 32))
	assert.Error(t, err)
	ty, _ := ParseType(`{order: {total: float}, tier: "gold" | "basic"}`)
	r, ok := ty.RecordFields()
	assert.True(t, ok)
	assert.Equal(t, len(r.Fields), 2)
	assert.Equal(t, r.Fields[1].Key, "tier")
}

func TestSigs(t *testing.T) {
	for _, c := range []struct {
		text     string
		min, max int
	}{
		{`() => int`, 0, 0},
		{`(x: T) => T where T: number`, 1, 1},
		{`(a: T, b: T, ...rest: T[]) => T where T: ordered`, 2, -1},
		{`(message: string, data?: any) => never !`, 1, 2},
		{`(f: (item: T, index: int) => unknown) => T[]`, 1, 1},
		{`(f: () => int) => int`, 1, 1},
		{`(f: (acc: A, item: T, index: int) => A, init: A) => A`, 2, 2},
		{`(code: string) => float !`, 1, 1},
		{`(x: (int | string)) => int`, 1, 1},
		{`(old: string, new: string) => string !`, 2, 2},
		{`(x: T, y: U) => T | U where T: ordered, U: number`, 2, 2},
	} {
		s, err := ParseSig(c.text)
		assert.NoError(t, err, c.text)
		assert.Equal(t, s.MinArgs(), c.min, c.text)
		assert.Equal(t, s.MaxArgs(), c.max, c.text)
	}
	for _, bad := range []string{
		`int`, `(x) => int`, `(x: int)`, `(x: int) => `, `(x: int,) => int`,
		`() => int where T: string`, `() => int where x: number`, `(...: int) => int`,
		`(f: (x) => int) => int`, `(f: (x: int) => int,) => int`, `() => unknown`,
		`() => int ! !`,
	} {
		_, err := ParseSig(bad)
		assert.Error(t, err, bad)
	}
	m, err := ParseMethod(`Time.format: (layout: string) => string`)
	assert.NoError(t, err)
	assert.Equal(t, m.Recv.String(), "Time")
	assert.Equal(t, m.Name, "format")
	m, err = ParseMethod(`T[].first: () => T | null`)
	assert.NoError(t, err)
	assert.Equal(t, m.Recv.String(), "T[]")
}

// Every signature and receiver of library.json parses.
func TestLibrarySignatures(t *testing.T) {
	data, err := os.ReadFile("../../../suite/library.json")
	if err != nil {
		t.Skip("suite/library.json is not there")
	}
	var entries []struct {
		Name      string `json:"name"`
		Receiver  string `json:"receiver"`
		Signature string `json:"signature"`
	}
	assert.NoError(t, json.Unmarshal(data, &entries))
	assert.True(t, len(entries) > 0)
	for _, e := range entries {
		_, err := ParseSig(e.Signature)
		assert.NoError(t, err, "%s: %s", e.Name, e.Signature)
		if e.Receiver != "" {
			_, err := ParseRecv(e.Receiver)
			assert.NoError(t, err, "%s: %s", e.Name, e.Receiver)
		}
	}
}

// F3 8.2: a raw string is a String of type text; a text block is not.
func TestRawStringInTypeText(t *testing.T) {
	ty, err := ParseType(`r"a\b" | {r"x-y": int}`)
	assert.NoError(t, err)
	assert.Equal(t, ty.String(), `"a\\b" | {x-y: int}`)
	_, err = ParseType("\"\"\"\n  a\n  \"\"\"")
	assert.Error(t, err)
}
