package jsonschema

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/deepnoodle-ai/risor/go/v3/internal/types"
	"github.com/deepnoodle-ai/wonton/assert"
)

func rec(closable bool, rest *types.Type, fs ...types.Field) *types.Type {
	t := types.Rec(fs, rest)
	t.Record().Closable = closable
	return t
}

func req(k string, t *types.Type) types.Field { return types.Field{Key: k, Type: t} }
func opt(k string, t *types.Type) types.Field { return types.Field{Key: k, Type: t, Optional: true} }

func TestPick(t *testing.T) {
	// any is dropped first; with nothing left, any.
	assert.Equal(t, pick([]*types.Type{types.Any, types.Int}).String(), "int")
	assert.Equal(t, pick([]*types.Type{types.Any, types.Any}).String(), "any")
	// The first that fits all the others.
	assert.Equal(t, pick([]*types.Type{types.Join(types.Int, types.Null), types.Int}).String(), "int")
	// None fits: the first.
	assert.Equal(t, pick([]*types.Type{types.String, types.Int}).String(), "string")
	// Records meet.
	a := rec(false, types.Any, req("a", types.Int))
	b := rec(false, types.Any, req("b", types.String))
	assert.Equal(t, pick([]*types.Type{a, b}).String(), "{a: int, b: string, ...}")
	// A record or null does not meet.
	an := types.Join(a, types.Null)
	assert.Equal(t, pick([]*types.Type{an, b}).String(), an.String())
}

func TestMeet(t *testing.T) {
	a := rec(true, types.Any, req("x", types.Int), opt("y", types.String))
	b := rec(true, types.Any, opt("x", types.Any), req("y", types.Lit("p")))
	m := meet(a.Record(), b.Record())
	// Required when either is; field types by Pick.
	assert.Equal(t, types.RecType(m).String(), "{x: int, y: \"p\", ...}")
	assert.True(t, m.Closable)
	// Closable only when both are.
	c := rec(false, types.Any, req("z", types.Bool))
	assert.False(t, meet(a.Record(), c.Record()).Closable)
	// Meet is commutative up to the order of fields.
	assert.Equal(t, types.RecType(meet(b.Record(), a.Record())).String(), types.RecType(m).String())
	// Nested records meet.
	n1 := rec(false, types.Any, req("o", rec(false, types.Any, req("p", types.Int))))
	n2 := rec(false, types.Any, req("o", rec(false, types.Any, req("q", types.Int))))
	assert.Equal(t, types.RecType(meet(n1.Record(), n2.Record())).String(), "{o: {p: int, q: int, ...}, ...}")
}

func TestClose(t *testing.T) {
	inner := rec(true, types.Any, req("p", types.Int))
	outer := rec(true, types.Any, req("o", inner), opt("l", types.List(inner)))
	open := rec(false, types.Any, req("k", inner))
	c := types.Close(types.Join(outer, types.Null))
	assert.Equal(t, c.String(), "{l?: {p: int}[], o: {p: int}} | null")
	// A record without the mark stays open; records inside it close.
	assert.Equal(t, types.Close(open).String(), "{k: {p: int}, ...}")
	// Close is idempotent and drops the mark.
	assert.Equal(t, types.Close(c).String(), c.String())
	assert.False(t, types.Close(inner).Record().Closable)
	// The input is not changed.
	assert.Equal(t, inner.String(), "{p: int, ...}")
}

func mapJSON(t *testing.T, text string, o Options) (*types.Type, error) {
	t.Helper()
	var v any
	assert.Nil(t, json.Unmarshal([]byte(text), &v))
	return Map(v, o, "")
}

func TestSharedRefTargets(t *testing.T) {
	// Each definition refers twice to the next: 2^30 paths, 30 targets.
	var defs []string
	for i := 0; i < 30; i++ {
		defs = append(defs, fmt.Sprintf(`"d%d": {"anyOf": [{"$ref": "#/$defs/d%d"}, {"$ref": "#/$defs/d%d"}]}`, i, i+1, i+1))
	}
	defs = append(defs, `"d30": {"type": "integer"}`)
	text := `{"$ref": "#/$defs/d0", "$defs": {` + strings.Join(defs, ",") + `}}`
	start := time.Now()
	ty, err := mapJSON(t, text, Options{})
	assert.Nil(t, err)
	assert.Equal(t, ty.String(), "int")
	assert.True(t, time.Since(start) < time.Second)

	// A record at each level, both fields one target: the type is shared.
	defs = defs[:0]
	for i := 0; i < 30; i++ {
		defs = append(defs, fmt.Sprintf(`"d%d": {"type": "array", "items": {"$ref": "#/$defs/d%d"}}`, i, i+1))
	}
	defs = append(defs, `"d30": {"type": "object", "properties": {"a": {"$ref": "#/$defs/d31"}, "b": {"$ref": "#/$defs/d31"}}, "required": ["a", "b"]}`,
		`"d31": {"type": "string"}`)
	text = `{"$ref": "#/$defs/d0", "$defs": {` + strings.Join(defs, ",") + `}}`
	ty, err = mapJSON(t, text, Options{Closed: true})
	assert.Nil(t, err)
	assert.Equal(t, ty.String(), "{a: string, b: string}"+strings.Repeat("[]", 30))
}

func TestRefCycle(t *testing.T) {
	// F17 12.4: a cycle through an array item is a recursive type, named by
	// the last token of the pointer.
	ty, err := mapJSON(t, `{"$ref": "#/$defs/tree", "$defs": {"tree": {"type": "array", "items": {"$ref": "#/$defs/tree"}}}}`, Options{})
	assert.Nil(t, err)
	assert.Equal(t, ty.String(), "Tree")
	assert.Equal(t, types.Unfold(ty).String(), "Tree[]")
	// A one-letter name is a type variable: bad-type.
	_, err = mapJSON(t, `{"$ref": "#/$defs/a", "$defs": {"a": {"type": "array", "items": {"$ref": "#/$defs/a"}}}}`, Options{})
	assert.NotNil(t, err)
	// The root, named Root with no other name.
	ty, err = mapJSON(t, `{"type": "object", "properties": {"next": {"anyOf": [{"$ref": "#"}, {"type": "null"}]}}, "additionalProperties": false}`, Options{})
	assert.Nil(t, err)
	assert.Equal(t, ty.String(), "Root")
	assert.Equal(t, types.Unfold(ty).String(), "{next?: Root | null}")
	// With no guard between: bad-type.
	_, err = mapJSON(t, `{"$ref": "#/$defs/aa", "$defs": {"aa": {"anyOf": [{"$ref": "#/$defs/aa"}, {"type": "null"}]}}}`, Options{})
	assert.NotNil(t, err)
	// allOf on a schema in progress: bad-type.
	_, err = mapJSON(t, `{"type": "object", "properties": {"x": {"allOf": [{"$ref": "#"}, {"type": "object"}]}}}`, Options{})
	assert.NotNil(t, err)
	// Mutual recursion: only the target met again is named.
	ty, err = mapJSON(t, `{"$ref": "#/$defs/dir", "$defs": {"dir": {"type": "object", "properties": {"files": {"type": "array", "items": {"$ref": "#/$defs/file"}}}, "required": ["files"], "additionalProperties": false}, "file": {"type": "object", "properties": {"parent": {"$ref": "#/$defs/dir"}}, "additionalProperties": false}}}`, Options{})
	assert.Nil(t, err)
	assert.Equal(t, ty.String(), "Dir")
	assert.Equal(t, types.Unfold(ty).String(), "{files: {parent?: Dir}[]}")
	// One target used twice side by side is no cycle.
	ty, err = mapJSON(t, `{"type": "object", "properties": {"x": {"$ref": "#/$defs/a"}, "y": {"$ref": "#/$defs/a"}}, "$defs": {"a": {"type": "boolean"}}}`, Options{})
	assert.Nil(t, err)
	assert.Equal(t, ty.String(), "{x?: bool, y?: bool, ...}")
}

func TestDepths(t *testing.T) {
	nest := func(n int) string {
		return strings.Repeat(`{"type": "array", "items": `, n) + `{"type": "integer"}` + strings.Repeat(`}`, n)
	}
	_, err := mapJSON(t, nest(64), Options{})
	assert.Nil(t, err)
	_, err = mapJSON(t, nest(65), Options{})
	assert.NotNil(t, err)
	// The depth of the type, from a const alone.
	deep := func(n int) string { return strings.Repeat("[", n) + "1" + strings.Repeat("]", n) }
	_, err = mapJSON(t, `{"const": `+deep(64)+`}`, Options{})
	assert.Nil(t, err)
	_, err = mapJSON(t, `{"const": `+deep(65)+`}`, Options{})
	assert.NotNil(t, err)
}

func TestLiteralNumbers(t *testing.T) {
	assert.Equal(t, Literal(2.0).String(), "int")
	assert.Equal(t, Literal(9007199254740991.0).String(), "int")
	assert.Equal(t, Literal(9007199254740992.0).String(), "float")
	assert.Equal(t, Literal(1e20).String(), "float")
	assert.Equal(t, Literal(1.5).String(), "float")
}

func TestSizeLimit(t *testing.T) {
	// Each definition is a record of two references to the next: a type of
	// size 2^32 held with sharing is bad-type, found at once.
	var defs []string
	for i := 0; i < 30; i++ {
		defs = append(defs, fmt.Sprintf(`"d%d": {"type": "object", "properties": {"l": {"$ref": "#/$defs/d%d"}, "r": {"$ref": "#/$defs/d%d"}}, "required": ["l", "r"]}`, i, i+1, i+1))
	}
	defs = append(defs, `"d30": {"type": "integer"}`)
	text := `{"$ref": "#/$defs/d0", "$defs": {` + strings.Join(defs, ",") + `}}`
	start := time.Now()
	_, err := mapJSON(t, text, Options{Closed: true})
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "larger"))
	assert.True(t, time.Since(start) < time.Second)
	// From d17, thirteen levels of closed records over int: size 2^14 - 1,
	// within the limit.
	text = `{"$ref": "#/$defs/d17", "$defs": {` + strings.Join(defs, ",") + `}}`
	ty, err := mapJSON(t, text, Options{Closed: true})
	assert.Nil(t, err)
	assert.Equal(t, types.Size(ty), 1<<14-1)
}

// TestEnumIsNLogN compares two sizes of enum: four times the
// strings must cost far less than sixteen times the time, which a quadratic
// Join of literal sets would.
func TestEnumIsNLogN(t *testing.T) {
	timeOf := func(n int) time.Duration {
		vals := make([]any, n)
		for i := range vals {
			vals[i] = fmt.Sprintf("v%07d", n-i)
		}
		schema := map[string]any{"enum": vals}
		best := time.Duration(1 << 62)
		for k := 0; k < 3; k++ {
			start := time.Now()
			_, _ = Map(schema, Options{}, "")
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}
	small, large := timeOf(25_000), timeOf(100_000)
	if large > 9*small {
		t.Fatalf("an enum of 100,000 strings took %v, of 25,000 %v: not n log n", large, small)
	}
	// Over the size limit, the answer is bad-type.
	vals := make([]any, types.MaxSize+1)
	for i := range vals {
		vals[i] = fmt.Sprint(i)
	}
	_, err := Map(map[string]any{"enum": vals}, Options{}, "")
	assert.NotNil(t, err)
}

func TestPlaceKeys(t *testing.T) {
	// Each place has a key of its own: a property name that holds the
	// separator of the tokens is not a path of two places (F17 12.4 item 1).
	text := `{"type": "object", "required": ["a", "a\u0000properties\u0000b"],
		"properties": {
			"a": {"type": "object", "required": ["b"], "properties": {"b": {"type": "integer"}}},
			"a\u0000properties\u0000b": {"type": "string"}}}`
	ty, err := mapJSON(t, text, Options{Closed: true})
	assert.Nil(t, err)
	got := ty.String()
	assert.True(t, strings.Contains(got, `: string`), got)
	// Two pointers that decode to the same tokens name one place, and the
	// name of a recursive type is its last token.
	ty, err = mapJSON(t, `{"$ref": "#/$defs/tr%65e", "$defs": {"tree": {"type": "array", "items": {"$ref": "#/$defs/tree"}}}}`, Options{})
	assert.Nil(t, err)
	assert.Equal(t, ty.String(), "Tree")
}

func TestPickReadsEveryKind(t *testing.T) {
	// F17 12.4 item 4: Pick's step 2 reads the kinds of each type, so a
	// reference whose definition is not yet known is bad-type even after a
	// type that is not a map.
	_, err := mapJSON(t, `{"type": "object", "properties": {"x": {"allOf": [false, {"$ref": "#"}]}}}`, Options{})
	assert.NotNil(t, err)
	_, err = mapJSON(t, `{"type": "object", "properties": {"x": {"allOf": [{"$ref": "#"}, false]}}}`, Options{})
	assert.NotNil(t, err)
}

func TestPartMetAgain(t *testing.T) {
	// F17 12.4 item 1: a place a $ref reaches first, met again as a part
	// of the schema that holds it, is the recursive type.
	ty, err := mapJSON(t, `{"$ref": "#/$defs/node/items", "$defs": {"node": {"type": "array", "items": {"$ref": "#/$defs/node"}}}}`, Options{})
	assert.Nil(t, err)
	assert.Equal(t, ty.String(), "Items")
	n, _ := ty.RefOnly()
	assert.Equal(t, n.Def().String(), "Items[]")
	// With no array item, property or map between, it is an error.
	_, err = mapJSON(t, `{"$ref": "#/$defs/node/allOf/0", "$defs": {"node": {"allOf": [{"$ref": "#/$defs/node"}]}}}`, Options{})
	assert.NotNil(t, err)
}
