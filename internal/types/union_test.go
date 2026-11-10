package types

import (
	"testing"

	"github.com/deepnoodle-ai/risor/v3/internal/typetext"
	"github.com/deepnoodle-ai/wonton/assert"
)

func uparse(t *testing.T, text string) *Type {
	t.Helper()
	tt, err := typetext.ParseType(text)
	assert.NoError(t, err)
	ty, err := FromText(tt, nil, false)
	assert.NoError(t, err)
	return ty
}

func TestFindTag(t *testing.T) {
	recs := func(text string) []*Record { return variants(parse(t, text)) }
	tag, ok := FindTag(recs(`{a: "x", b: "q"} | {a: "y", b: "p"}`))
	assert.True(t, ok)
	assert.Equal(t, tag, "a") // the least of two keys that qualify
	tag, ok = FindTag(recs(`{a: "x", k: "1"} | {a: "x", k: "2"} | {a: "y", k: "3"}`))
	assert.True(t, ok)
	assert.Equal(t, tag, "k")
	for _, bad := range []string{`{kind: string} | {kind: "a"}`, `{kind: "a"} | {kind?: "b"}`,
		`{kind: "a" | "b"} | {kind: "b"}`, `{kind: "a" | null} | {kind: "b"}`, `{kind: "a"} | map<int>`} {
		tt, _ := typetext.ParseType(bad)
		_, err := FromText(tt, nil, false)
		assert.NotNil(t, err, bad)
	}
}

func TestUnionOps(t *testing.T) {
	shape := parse(t, `{kind: "circle", r: float} | {kind: "rect", h: float, w: float}`)
	assert.Equal(t, shape.String(), `{kind: "circle", r: float} | {h: float, kind: "rect", w: float}`)
	assert.Equal(t, List(shape).String(), `({kind: "circle", r: float} | {h: float, kind: "rect", w: float})[]`)
	// Join keeps a union only beside the same union.
	assert.True(t, Join(shape, parse(t, `{kind: "rect", h: float, w: float} | {kind: "circle", r: float}`)).uni != nil)
	j := Join(shape, parse(t, `{kind: "circle", r: int}`))
	assert.Equal(t, j.String(), `{h?: float, kind: "circle" | "rect", r?: float, w?: float}`)
	// Unite forms a union; size is the sum of the variants.
	c, r := parse(t, `{kind: "circle", r: float}`), parse(t, `{kind: "rect", h: float, w: float}`)
	assert.True(t, Equal(Unite(c, r), shape))
	assert.Equal(t, Size(shape), 2+1+1+1+1+1)
	// Fits: each variant of S fits some variant of T.
	assert.True(t, SurelyFits(c, shape))
	ok, _ := Fits(RecType(shape.rec), shape)
	assert.False(t, ok)
	// Hit, All and without.
	assert.Equal(t, Hit(shape, "kind", []string{"circle"}).String(), `{kind: "circle", r: float}`)
	assert.Equal(t, WithoutType(shape, All(shape, "kind", []string{"circle"})).String(), `{h: float, kind: "rect", w: float}`)
	assert.True(t, Disjoint(shape, parse(t, `{kind: "tri", ...}`)))
	// A sub-union finds its own tag.
	three := parse(t, `{a: "x", k: "1"} | {a: "x", k: "2"} | {a: "y", k: "3"}`)
	left := WithoutType(three, parse(t, `{a: "x", k: "1"}`))
	assert.Equal(t, left.uni.Tag, "a")
}

func TestFillsThroughJoin(t *testing.T) {
	// A fill is kept apart through a Join, and the field is required.
	a := &Record{Fields: []Field{{Key: "a", Type: Join(String, Null), Base: String, Fill: Null}}, Rest: Never}
	b := &Record{Fields: []Field{{Key: "b", Type: Int}}, Rest: Never}
	j := joinRec(a, b)
	f, _ := j.field("a")
	assert.False(t, f.Optional)
	assert.Equal(t, f.Fill.String(), "null")
	assert.Equal(t, f.Type.String(), "string | null")
	// A closable record joined with an exact one stays closable.
	a.Closable = true
	assert.True(t, joinRec(a, b).Closable)
}
