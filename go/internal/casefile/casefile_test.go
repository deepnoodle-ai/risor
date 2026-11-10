package casefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func load(t *testing.T, text string) (*File, error) {
	t.Helper()
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "prog.risor"), []byte("1 + 2"), 0o644))
	path := filepath.Join(dir, "cases.json")
	assert.NoError(t, os.WriteFile(path, []byte(text), 0o644))
	return Load(path)
}

func TestValid(t *testing.T) {
	f, err := load(t, `{
		"env": "{d: int}",
		"source": "1 / d",
		"functions": {"rate": {"type": "(code: string) => float !",
			"calls": [{"args": ["A"], "result": 0.1, "charge": 2}, {"error": "no"}]},
			"g": "() => int"},
		"limits": {"steps": 10, "depth": 1000},
		"precondition": false,
		"cases": [
			{"name": "a", "vars": {"d": 0}, "expect": {"phase": "run", "steps": 1,
				"diagnostics": [{"code": "divide-by-zero", "span": [1, 1, 1, 6], "class": "divide"}]}},
			{"name": "b", "file": "prog.risor", "expect": {}},
			{"name": "c", "source": "x", "env": "{x: int}", "expect": {"value": 2.0, "truthy": true}},
			{"name": "d", "skip": "later", "expect": {"type": "int", "sites": [{"class": "divide", "span": [1, 1, 1, 2]}]}}
		]}`)
	assert.NoError(t, err)
	assert.Equal(t, len(f.Cases), 4)
	a := f.Cases[0]
	assert.Equal(t, a.Source, "1 / d")
	assert.Equal(t, *a.Env, "{d: int}")
	assert.Equal(t, a.Expect.Phase, "run")
	assert.Equal(t, *a.Expect.Steps, int64(1))
	assert.Equal(t, *a.Expect.Diagnostics[0].Span, [4]int{1, 1, 1, 6})
	assert.Equal(t, a.Functions["rate"].Calls[0].Charge, int64(2))
	assert.Equal(t, *a.Functions["rate"].Calls[1].Error, "no")
	assert.Equal(t, *a.Functions["g"].Type, "() => int")
	assert.Equal(t, *a.Limits.Depth, int64(1000))
	assert.Equal(t, f.Cases[1].Source, "1 + 2")
	assert.Equal(t, f.Cases[1].File, "prog.risor")
	assert.Equal(t, f.Cases[2].Source, "x")
	assert.Equal(t, *f.Cases[2].Env, "{x: int}")
	assert.Equal(t, f.Cases[2].Expect.Value, any(2.0))
	assert.Equal(t, *f.Cases[3].Skip, "later")

	// Ruling AP1: check in the header and in a case.
	f, err = load(t, `{"check": false, "cases": [
		{"name": "a", "source": "1", "expect": {"phase": "check", "diagnostics": [{"code": "arity"}]}},
		{"name": "b", "source": "1", "check": true, "expect": {"type": "int"}}]}`)
	assert.NoError(t, err)
	assert.False(t, *f.Cases[0].Check)
	assert.True(t, *f.Cases[1].Check)

	// A list of cases with no header.
	f, err = load(t, `[{"name": "x", "source": "1", "expect": {}}]`)
	assert.NoError(t, err)
	assert.Equal(t, len(f.Cases), 1)
}

// 03-host.md 3.10.10, item by item, and ruling A5.
func TestInvalid(t *testing.T) {
	c := func(fields string) string {
		return `[{"name": "x", "source": "1", ` + fields + `}]`
	}
	for _, bad := range []struct{ why, text string }{
		{"1 not JSON", `[{"name": "x",`},
		{"1 a repeated key", `[{"name": "x", "name": "y", "source": "1", "expect": {}}]`},
		{"1 trailing text", `[] []`},
		{"2 neither", `"cases"`},
		{"2 no cases", `{"source": "1"}`},
		{"3 header field", `{"name": "x", "cases": []}`},
		{"3 header skip", `{"skip": "x", "cases": []}`},
		{"3 case field", c(`"expect": {}, "note": 1`)},
		{"3 expect field", c(`"expect": {"phase": "parse", "diagnostics": [{"code": "x"}], "value": 1}`)},
		{"3 result field", c(`"expect": {"code": "x"}`)},
		{"3 diagnostic field", c(`"expect": {"phase": "parse", "diagnostics": [{"code": "x", "hint": 1}]}`)},
		{"3 stub field", c(`"expect": {}, "functions": {"f": {"type": "() => int", "doc": "x"}}`)},
		{"3 row field", c(`"expect": {}, "functions": {"f": {"type": "() => int", "calls": [{"result": 1, "x": 1}]}}`)},
		{"3 limits field", c(`"expect": {}, "limits": {"time": 1}`)},
		{"4 source type", `[{"name": "x", "source": 1, "expect": {}}]`},
		{"4 name type", `[{"name": 1, "source": "1", "expect": {}}]`},
		{"4 version pattern", c(`"expect": {}, "version": "3"`)},
		{"4 methods key", c(`"expect": {}, "methods": {"time.format": "() => string"}`)},
		{"4 depth range", c(`"expect": {}, "limits": {"depth": 0}`)},
		{"4 depth range", c(`"expect": {}, "limits": {"depth": 1001}`)},
		{"4 steps whole", c(`"expect": {}, "limits": {"steps": 1.5}`)},
		{"4 span", c(`"expect": {"phase": "parse", "diagnostics": [{"code": "x", "span": [0, 1, 1, 1]}]}`)},
		{"4 span length", c(`"expect": {"phase": "parse", "diagnostics": [{"code": "x", "span": [1, 1, 1]}]}`)},
		{"4 code pattern", c(`"expect": {"phase": "parse", "diagnostics": [{"code": "Bad"}]}`)},
		{"4 class", c(`"expect": {"phase": "run", "diagnostics": [{"code": "x", "class": "nope"}]}`)},
		{"4 phase", c(`"expect": {"phase": "compile", "diagnostics": [{"code": "x"}]}`)},
		{"4 empty diagnostics", c(`"expect": {"phase": "run", "diagnostics": []}`)},
		{"4 truthy", c(`"expect": {"truthy": 1}`)},
		{"4 skip", c(`"expect": {}, "skip": true`)},
		{"4 precondition", c(`"expect": {}, "precondition": "no"`)},
		{"4 path", c(`"expect": {"phase": "run", "diagnostics": [{"code": "x", "path": [-1]}]}`)},
		{"5 no name", `[{"source": "1", "expect": {}}]`},
		{"5 name twice", `[{"name": "x", "source": "1", "expect": {}}, {"name": "x", "source": "2", "expect": {}}]`},
		{"6 no expect", `[{"name": "x", "source": "1"}]`},
		{"7 neither", `[{"name": "x", "expect": {}}]`},
		{"7 both", `[{"name": "x", "source": "1", "file": "prog.risor", "expect": {}}]`},
		{"8 unreadable", `[{"name": "x", "file": "nope.risor", "expect": {}}]`},
		{"9 vars key", c(`"expect": {}, "env": "{a: int}", "vars": {"b": 1}`)},
		{"9 vars no env", c(`"expect": {}, "vars": {"b": 1}`)},
		{"10 both", c(`"expect": {}, "functions": {"f": {"type": "() => int", "calls": [{"result": 1, "error": "x"}]}}`)},
		{"10 neither", c(`"expect": {}, "functions": {"f": {"type": "() => int", "calls": [{"args": []}]}}`)},
		{"11 no type", c(`"expect": {}, "functions": {"f": {"calls": []}}`)},
		{"12 steps with parse", c(`"expect": {"phase": "parse", "steps": 1, "diagnostics": [{"code": "x"}]}`)},
		{"12 allocated with check", c(`"expect": {"phase": "check", "allocated": 1, "diagnostics": [{"code": "x"}]}`)},
		{"13 infinity", c(`"expect": {"value": 1e400}`)},
		{"A5 limit code with steps", c(`"expect": {"phase": "run", "steps": 3, "diagnostics": [{"code": "steps-exceeded"}]}`)},
		{"AP1 check not a boolean", c(`"expect": {}, "check": "no"`)},
		{"AP1 check false with type", c(`"expect": {"type": "int"}, "check": false`)},
		{"AP1 check false with sites", c(`"expect": {"sites": []}, "check": false`)},
		{"AP1 check false with a type rule code", c(`"expect": {"phase": "check", "diagnostics": [{"code": "kind"}]}, "check": false`)},
		{"AP1 header check false with type", `{"check": false, "cases": [{"name": "x", "source": "1", "expect": {"type": "int"}}]}`},
	} {
		_, err := load(t, bad.text)
		assert.Error(t, err, bad.why)
		_, ok := err.(*InvalidError)
		assert.True(t, ok, bad.why)
	}
}

func TestHeaderRule(t *testing.T) {
	f, err := load(t, `{"file": "prog.risor", "env": "{a: int}", "vars": {"a": 1},
		"cases": [
			{"name": "x", "source": "a", "expect": {}},
			{"name": "y", "env": "{b: int}", "vars": {"b": 2}, "expect": {}},
			{"name": "z", "expect": {}}
		]}`)
	assert.NoError(t, err)
	assert.Equal(t, f.Cases[0].Source, "a")
	assert.Equal(t, f.Cases[0].File, "")
	assert.Equal(t, *f.Cases[1].Env, "{b: int}")
	assert.Equal(t, f.Cases[1].Vars["b"], any(2.0))
	assert.Equal(t, f.Cases[2].Source, "1 + 2")
	// Laid over whole: a case's env replaces the header's, and the header's
	// vars then name a field env does not declare.
	_, err = load(t, `{"env": "{a: int}", "vars": {"a": 1}, "cases": [
		{"name": "y", "source": "1", "env": "{b: int}", "expect": {}}]}`)
	assert.Error(t, err)
	inv := err.(*InvalidError)
	assert.True(t, strings.Contains(inv.Reason, `"a"`))
	assert.Equal(t, inv.Names, []string{"y"})
}
