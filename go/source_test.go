package risor_test

import (
	"context"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

// The size of a source text (spec/core/01-language.md 2.1 items 4 and 5).
// The suite tests the rule with a lowered source limit; these tests take
// the default, 1,048,576 code points, which a case file cannot hold
// compactly.

func sizeErr(t *testing.T, err error) risor.Diagnostic {
	t.Helper()
	e, ok := err.(*risor.Error)
	assert.True(t, ok)
	assert.Equal(t, len(e.Diagnostics), 1)
	d := e.Diagnostics[0]
	assert.Equal(t, d.Phase, risor.PhaseParse)
	assert.Equal(t, d.Code, "source-too-large")
	assert.Nil(t, d.Span)
	return d
}

// padded is the program 1 with spaces after it, n code points in all.
func padded(n int) string { return "1" + strings.Repeat(" ", n-1) }

func TestSourceAtTheDefaultLimit(t *testing.T) {
	assert.Equal(t, risor.MaxSource, 1048576)
	_, err := risor.Parse(padded(risor.MaxSource))
	assert.Nil(t, err)
	d := sizeErr(t, func() error { _, err := risor.Parse(padded(risor.MaxSource + 1)); return err }())
	assert.Equal(t, d.Message, "the source text is longer than the limit of 1048576 code points")
}

func TestSourceCountsCodePoints(t *testing.T) {
	// A string of U+1F600: four bytes each in UTF-8, two units in UTF-16.
	inner := risor.MaxSource - 2
	src := `"` + strings.Repeat("\U0001F600", inner) + `"`
	assert.Equal(t, len(src), 4*inner+2)
	x, err := risor.Parse(src)
	assert.Nil(t, err)
	assert.NotNil(t, x)
	src = `"` + strings.Repeat("\U0001F600", inner+1) + `"`
	_, err = risor.Parse(src)
	sizeErr(t, err)
}

func TestSourceCountsTheDecodedText(t *testing.T) {
	// E2 82 is one maximal ill-formed subpart: one U+FFFD (03-host.md 3.4.5).
	bad := "\xe2\x82"
	src := `"` + strings.Repeat(bad, risor.MaxSource-2) + `"`
	_, err := risor.Parse(src)
	assert.Nil(t, err)
	_, err = risor.Parse(src + " ")
	sizeErr(t, err)
}

func TestSourceLeadingBOMCounts(t *testing.T) {
	_, err := risor.Parse("\xef\xbb\xbf" + padded(risor.MaxSource-1))
	assert.Nil(t, err)
	_, err = risor.Parse("\xef\xbb\xbf" + padded(risor.MaxSource))
	sizeErr(t, err)
}

func TestSourceLimitOption(t *testing.T) {
	_, err := risor.Parse("1 + 2", risor.SourceLimit(5))
	assert.Nil(t, err)
	d := sizeErr(t, func() error { _, err := risor.Parse("1 + 23", risor.SourceLimit(5)); return err }())
	assert.Equal(t, d.Message, "the source text is longer than the limit of 5 code points")
	// The last option given wins.
	_, err = risor.Parse("1 + 23", risor.SourceLimit(5), risor.SourceLimit(6))
	assert.Nil(t, err)
	for _, bad := range []risor.SourceLimit{-1, risor.MaxSource + 1} {
		_, err = risor.Parse("1", bad)
		e := err.(*risor.Error)
		assert.Equal(t, e.Phase(), risor.PhaseDeclare)
		assert.Equal(t, e.Diagnostics[0].Code, "bad-limit")
	}
	_, err = risor.Parse("1", risor.SourceLimit(risor.MaxSource))
	assert.Nil(t, err)
}

func TestSourceLimitTemplate(t *testing.T) {
	_, err := risor.ParseTemplate(strings.Repeat("a", risor.MaxSource))
	assert.Nil(t, err)
	_, err = risor.ParseTemplate(strings.Repeat("a", risor.MaxSource+1))
	sizeErr(t, err)
	_, err = risor.ParseTemplate("ab${{ 1 }}", risor.SourceLimit(9))
	sizeErr(t, err)
}

func TestSourceLimitFormat(t *testing.T) {
	_, err := risor.Format(padded(risor.MaxSource + 1))
	sizeErr(t, err)
	_, err = risor.FormatTemplate(strings.Repeat("a", risor.MaxSource+1))
	sizeErr(t, err)
	out, err := risor.Format("1+2", risor.SourceLimit(3))
	assert.Nil(t, err)
	assert.Equal(t, out, "1 + 2\n")
	_, err = risor.Format("1 + 2", risor.SourceLimit(4))
	sizeErr(t, err)
}

func TestSourceLimitAssist(t *testing.T) {
	env, err := risor.NewEnv(risor.Var("user", "{name: string}"))
	assert.Nil(t, err)
	at := risor.Pos{Line: 1, Column: 4}
	c := env.Complete("use"+strings.Repeat(" ", risor.MaxSource), at)
	assert.Equal(t, len(c.Items), 0)
	assert.Equal(t, c.Span, risor.Span{Start: at, End: at})
	c = env.Complete("use", at, risor.SourceLimit(3))
	assert.Equal(t, len(c.Items), 1)
	c = env.Complete("use ", at, risor.SourceLimit(3))
	assert.Equal(t, len(c.Items), 0)
	c = env.Complete("use", at, risor.SourceLimit(-1))
	assert.Equal(t, len(c.Items), 0)
	on := risor.Pos{Line: 1, Column: 2}
	assert.NotNil(t, env.Hover("user", on, risor.SourceLimit(4)))
	assert.Nil(t, env.Hover("user ", on, risor.SourceLimit(4)))
	assert.Nil(t, env.Hover("user"+strings.Repeat(" ", risor.MaxSource), on))
}

func TestSourceLimitLoad(t *testing.T) {
	_, _, err := risor.LoadValue(context.Background(), nil, "{a: 1}", nil, risor.SourceLimit(5))
	le := err.(*risor.LoadError)
	assert.Equal(t, le.Step, risor.StepParse)
	assert.Equal(t, le.Diagnostics[0].Code, "source-too-large")
	v, _, err := risor.LoadValue(context.Background(), nil, "{a: 1}", nil, risor.SourceLimit(6))
	assert.Nil(t, err)
	assert.Equal(t, v, any(map[string]any{"a": float64(1)}))
	_, _, err = risor.LoadValue(context.Background(), nil, "1", nil, risor.SourceLimit(-1))
	assert.Equal(t, err.(*risor.LoadError).Step, risor.StepDeclare)
}

func TestSourceTooLargeRenders(t *testing.T) {
	_, err := risor.Parse("1 + 23", risor.SourceLimit(5))
	ds := err.(*risor.Error).Diagnostics
	assert.True(t, strings.Contains(risor.RenderLine(ds), "source-too-large"))
	assert.True(t, strings.Contains(risor.RenderDetail(ds, "1 + 23"), "source-too-large"))
	assert.True(t, strings.Contains(risor.RenderJSON(ds), `"source-too-large"`))
}
