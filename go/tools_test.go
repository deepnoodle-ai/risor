package risor_test

import (
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/runner"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
)

// Tests of package F12 that no case can hold.

// commentPlaces gives the texts made by putting a comment at each place
// between two tokens of a program: a block comment before each token, and
// a line comment after each.
func commentPlaces(text string) []string {
	f := syntax.NewFile(text)
	toks, err := syntax.Lex(f)
	if err != nil {
		return nil
	}
	src := f.Runes()
	var out []string
	for _, tk := range toks {
		if tk.Kind == syntax.TemplateMiddle || tk.Kind == syntax.TemplateTail {
			continue
		}
		out = append(out, string(src[:tk.Start])+"/* c */"+string(src[tk.Start:]))
		if tk.Kind != syntax.EOF && tk.Kind != syntax.TemplateHead {
			out = append(out, string(src[:tk.End])+" // c\n"+string(src[tk.End:]))
		}
	}
	return out
}

// TestFormatCommentsEverywhere puts a comment at every place between the
// tokens of programs of the suite, and tests that each comment is written
// once (the formatter panics otherwise) and that the laws hold.
func TestFormatCommentsEverywhere(t *testing.T) {
	texts := suiteTexts(t)
	n := 0
	for _, s := range texts {
		if s.template || len(s.text) > 160 || strings.Contains(s.text, "${") {
			continue
		}
		if _, err := risor.Format(s.text); err != nil {
			continue
		}
		for _, v := range commentPlaces(s.text) {
			out, err := risor.Format(v)
			if err != nil {
				continue // a comment inside an interpolation or a literal
			}
			n++
			again, err := risor.Format(out)
			if err != nil || again != out {
				t.Errorf("%q: not idempotent:\n%s\n---\n%s", v, out, again)
				continue
			}
			if strings.Count(out, "/* c */")+strings.Count(out, "// c") != 1 {
				t.Errorf("%q: the comment is not written once:\n%s", v, out)
			}
			if a, b := runner.TreeOf(false, v), runner.TreeOf(false, out); a != b {
				t.Errorf("%q: the tree differs:\n%s", v, out)
			}
		}
		if n > 40000 {
			break
		}
	}
	t.Logf("%d texts with a comment", n)
}

// TestAssistNeverFails runs completion and hover at every position of
// programs of the suite (F12 3.1: assist never fails). A completion's span
// ends at its position, and a hover's span holds it.
func TestAssistNeverFails(t *testing.T) {
	env, err := risor.NewEnv(risor.Declare("{x: any, n: int, s: string, xs: int[], m: {a: int, b: string}}"), risor.Groups("math", "json"))
	assert.NoError(t, err)
	texts := suiteTexts(t)
	count := 0
	for i, s := range texts {
		if s.template || len(s.text) > 120 || i%3 != 0 {
			continue
		}
		f := syntax.NewFile(s.text)
		line, col := 1, 1
		for _, r := range f.Runes() {
			pos := risor.Pos{Line: line, Column: col}
			c := env.Complete(s.text, pos)
			if c.Span.End != pos {
				t.Errorf("%q at %v: the span %v does not end at the position", s.text, pos, c.Span)
			}
			if h := env.Hover(s.text, pos); h != nil {
				if h.Span.Start.Line != line || h.Span.Start.Column > col || h.Span.End.Column <= col {
					t.Errorf("%q at %v: the hover span %v does not hold the position", s.text, pos, h.Span)
				}
			}
			count++
			if r == '\n' {
				line, col = line+1, 1
			} else {
				col++
			}
		}
	}
	t.Logf("%d positions", count)
	assert.Equal(t, risor.AssistPanics(), int64(0))
}

func TestAssistPositions(t *testing.T) {
	env, err := risor.NewEnv(risor.Declare("{user: {name: string}}"))
	assert.NoError(t, err)
	// A position in no line of the text: an empty list and no hover.
	c := env.Complete("user", risor.Pos{Line: 3, Column: 1})
	assert.Equal(t, len(c.Items), 0)
	assert.Nil(t, env.Hover("user", risor.Pos{Line: 1, Column: 9}))
	// A byte order mark is not counted.
	c = env.Complete("\uFEFFus", risor.Pos{Line: 1, Column: 3})
	assert.Equal(t, len(c.Items), 1)
	assert.Equal(t, c.Items[0].Label, "user")
	// The second line.
	c = env.Complete("const a = 1;\nuser.n", risor.Pos{Line: 2, Column: 7})
	assert.Equal(t, len(c.Items), 1)
	assert.Equal(t, c.Items[0].Insert, "name")
	h := env.Hover("const a = user;\na.name", risor.Pos{Line: 2, Column: 1})
	assert.NotNil(t, h)
	assert.Equal(t, *h.Type, "{name: string}")
	assert.Equal(t, h.Declaration.String(), "[1, 7, 1, 8]")
}

func TestFormatSmall(t *testing.T) {
	for src, want := range map[string]string{
		"":                            "",
		"x // a\n// b\n":              "x // a\n// b\n",
		"[\n// only\n]":               "[\n// only\n]\n",
		"f(\n  a, /* x */\n  b\n)":    "f(a, /* x */ b)\n",
		"{a: 1, // one\n}":            "{\n  a: 1, // one\n}\n",
		"x as {a: int, /* c */}":      "x as {a: int /* c */}\n",
		"\"\"\"\n  a\n  \"\"\".len()": "\"\"\"\n  a\n  \"\"\".len()\n",
	} {
		got, err := risor.Format(src)
		if src == "" {
			assert.NotNil(t, err) // an empty text has no expression
			continue
		}
		assert.NoError(t, err, src)
		assert.Equal(t, got, want, src)
	}
}

func TestHoverCutTakesOnlyTheNextParen(t *testing.T) {
	env, err := risor.NewEnv(risor.Declare("{xs: string[]}"))
	assert.NoError(t, err)
	// Hover's cut takes a ( only when it is the very next character (F12
	// 3.2 item 3), whether the text lexes or not (#).
	for _, text := range []string{"xs.first(", "xs.first( #"} {
		h := env.Hover(text, risor.Pos{Line: 1, Column: 5})
		assert.NotNil(t, h, text)
		assert.Equal(t, h.Kind, "method", text)
	}
	// White space or a comment between: the ( is not in the cut, and the
	// word is read as a field.
	for _, text := range []string{"xs.first (", "xs.first /* c */ (", "xs.first /* c */ ( #", "xs.first // c\n( #", "xs.first\t( #"} {
		h := env.Hover(text, risor.Pos{Line: 1, Column: 5})
		assert.NotNil(t, h, text)
		assert.Equal(t, h.Kind, "field", text)
	}
}

func TestHoverParameterTakesTheFirstCheck(t *testing.T) {
	// A const lambda is checked at each use; a parameter's detail is that
	// of the first check that reaches the position (F12 3.12).
	env, err := risor.NewEnv(risor.Declare("{}"))
	assert.NoError(t, err)
	for _, c := range []struct {
		text string
		col  int
		want string
	}{
		{`const f = x => x + x; [f(1), f("a")]`, 11, "int"},
		{`const f = x => x + x; [f(1), f("a")]`, 16, "int"},
		{`const f = x => x + x; [f("a"), f(1)]`, 16, `"a"`},
	} {
		h := env.Hover(c.text, risor.Pos{Line: 1, Column: c.col})
		assert.NotNil(t, h, c.text)
		assert.NotNil(t, h.Type, c.text)
		assert.Equal(t, *h.Type, c.want, c.text)
	}
}
