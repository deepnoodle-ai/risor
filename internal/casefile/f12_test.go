package casefile

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// The case fields of package F12 7.

func TestToolsCases(t *testing.T) {
	f, err := load(t, `{"source": "1", "env": "{x: int}", "cases": [
		{"name": "f", "format": "x+1", "vars": {"x": 1}, "expect": {"formatted": "x + 1\n"}},
		{"name": "t", "formatTemplate": "${{a}}", "expect": {"formatted": "${{ a }}"}},
		{"name": "p", "format": "1 +", "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token"}]}},
		{"name": "c", "complete": "a\nb‸c", "returns": "int", "expect": {"completion": {"span": [2, 2, 2, 2], "items": [{"label": "x", "kind": "variable", "detail": null, "insert": "x"}]}}},
		{"name": "h", "hover": "‸x", "expect": {"hover": {"kind": "variable", "span": [1, 1, 1, 2], "type": "int", "declaration": null}}},
		{"name": "n", "hover": "‸1", "expect": {"hover": null}},
		{"name": "d", "complete": "‸", "env": "{x: nope}", "expect": {"phase": "declare", "diagnostics": [{"code": "bad-type"}]}}
	]}`)
	assert.NoError(t, err)
	c := f.Cases[3]
	assert.Equal(t, c.Tools.Source, "a\nbc")
	assert.Equal(t, c.Tools.Line, 2)
	assert.Equal(t, c.Tools.Column, 2)
	assert.True(t, c.Tools.Completion.Items[0].Detail.Null)
	assert.Nil(t, c.Tools.Formatted)
	assert.Equal(t, *f.Cases[0].Tools.Formatted, "x + 1\n")
	h := f.Cases[4].Tools.Hover
	assert.Equal(t, h.Type.Text, "int")
	assert.True(t, h.HasDeclaration && h.DeclarationNull)
	assert.Nil(t, h.Doc)
	assert.True(t, f.Cases[5].Tools.HasHover)
	assert.Nil(t, f.Cases[5].Tools.Hover)
}

func TestToolsInvalid(t *testing.T) {
	for _, text := range []string{
		`{"format": "x", "cases": [{"name": "a", "expect": {"formatted": "x\n"}}]}`,                                   // not in a header
		`[{"name": "a", "format": 1, "expect": {"formatted": "x\n"}}]`,                                                // not a string
		`[{"name": "a", "format": "x", "opaque": ["T"], "expect": {"formatted": "x\n"}}]`,                             // a field 7.1 does not allow
		`[{"name": "a", "format": "x", "expect": {"formatted": "x\n", "value": 1}}]`,                                  // not a format result
		`[{"name": "a", "format": "x", "expect": {"phase": "run", "diagnostics": [{"code": "x"}]}}]`,                  // a failure of another phase
		`[{"name": "a", "complete": "x", "expect": {"completion": {"span": [1, 1, 1, 1], "items": []}}}]`,             // no mark
		`[{"name": "a", "complete": "‸x‸", "expect": {"completion": {"span": [1, 1, 1, 1], "items": []}}}]`,           // two marks
		`[{"name": "a", "complete": "‸", "vars": {}, "expect": {"completion": {"span": [1, 1, 1, 1], "items": []}}}]`, // vars
		`[{"name": "a", "complete": "‸", "expect": {"completion": {"span": [1, 1, 1, 1], "items": [{"label": "x", "kind": "thing"}]}}}]`,
		`[{"name": "a", "complete": "‸", "expect": {"completion": {"span": [1, 1, 1, 1], "items": [{"label": "x", "kind": "name", "extra": 1}]}}}]`,
		`[{"name": "a", "hover": "‸x", "expect": {"hover": {"kind": "name", "span": [1, 1, 1, 2], "type": 1}}}]`,
		`[{"name": "a", "hover": "‸x", "expect": {"completion": {"span": [1, 1, 1, 1], "items": []}}}]`,
		`[{"name": "a", "source": "1", "expect": {"formatted": "1\n"}}]`, // formatted in a program case
		`[{"name": "a", "source": "1", "format": "1", "expect": {"formatted": "1\n"}}]`,
	} {
		_, err := load(t, text)
		assert.NotNil(t, err, text)
	}
}
