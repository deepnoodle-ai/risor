package risortest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/risortest"
	"github.com/deepnoodle-ai/wonton/assert"
)

func write(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	assert.NoError(t, os.WriteFile(p, []byte(text), 0o644))
	return p
}

const passing = `{"env": "{n: int}", "cases": [
  {"name": "double", "source": "n * 2", "vars": {"n": 3}, "expect": {"value": 6}},
  {"name": "later", "source": "n", "skip": "not yet", "expect": {}}
]}`

func TestRunPasses(t *testing.T) {
	p := write(t, t.TempDir(), "a.test.json", passing)
	risortest.Run(t, nil, p)
}

func TestVerdicts(t *testing.T) {
	f, err := risortest.Parse([]byte(`[
  {"name": "a", "source": "1 + 2", "expect": {"value": 4}},
  {"name": "b", "env": "{n: int}", "source": "n", "vars": {"n": "x"}, "expect": {"value": 1}},
  {"name": "c", "source": "1", "skip": "waits", "expect": {}}
]`), false)
	assert.NoError(t, err)
	cs := f.Cases()
	assert.Equal(t, len(cs), 3)
	a := cs[0].Run(context.Background())
	assert.Equal(t, a.Status, "fail")
	assert.Equal(t, a.Diffs, []risortest.Diff{{Name: "value", Expected: "4", Found: "3"}})
	assert.Equal(t, a.Report(), "  FAIL a\n    value: expected 4, found 3\n")
	b := cs[1].Run(context.Background())
	assert.Equal(t, b.Status, "invalid")
	assert.Equal(t, b.Fixture, []any{"vars", "n"})
	assert.Equal(t, b.Message, `expected int, found "x"`)
	assert.Equal(t, b.Report(), "  INVALID b\n    fixture .vars.n: expected int, found \"x\"\n")
	c := cs[2].Run(context.Background())
	assert.Equal(t, c.Status, "skip")
	assert.Equal(t, c.Reason, "waits")
}

func TestInvalidFile(t *testing.T) {
	_, err := risortest.Parse([]byte(`[{"name": "a", "source": "1"}]`), false)
	assert.Error(t, err)
	_, err = risortest.Load(write(t, t.TempDir(), "x.test.json", `{"cases": [`))
	assert.Error(t, err)
}

func TestFilter(t *testing.T) {
	f, err := risortest.Parse([]byte(passing), false, risortest.Filter("doub"))
	assert.NoError(t, err)
	assert.Equal(t, len(f.Cases()), 1)
	assert.Equal(t, f.Cases()[0].Name(), "double")
}

func TestUpdate(t *testing.T) {
	text := "[\n  {\"name\": \"sum\", \"source\": \"1 + 2\"},\n  {\"name\": \"ok\", \"source\": \"1\", \"expect\": {\"value\": 1}}\n]\n"
	f, err := risortest.Parse([]byte(text), false, risortest.Update(true))
	assert.NoError(t, err)
	var vs []risortest.Verdict
	for _, c := range f.Cases() {
		vs = append(vs, c.Run(context.Background()))
	}
	assert.Equal(t, vs[0].Status, "updated")
	assert.Equal(t, vs[0].Expect, `{"allocated":0,"steps":1,"type":"int","value":3}`)
	assert.Equal(t, vs[1].Status, "pass")
	out, err := f.Update(vs)
	assert.NoError(t, err)
	assert.Equal(t, string(out), "[\n  {\"name\": \"sum\", \"source\": \"1 + 2\", \"expect\": {\"allocated\":0,\"steps\":1,\"type\":\"int\",\"value\":3}},\n  {\"name\": \"ok\", \"source\": \"1\", \"expect\": {\"value\": 1}}\n]\n")
	// Only some of the cases run: nothing to write (F9 4.2 item 4).
	none, err := f.Update(vs[1:])
	assert.NoError(t, err)
	assert.True(t, none == nil)
	// Every case run, nothing updated: nothing to write.
	g, err := risortest.Parse([]byte(passing), false, risortest.Update(true))
	assert.NoError(t, err)
	var gs []risortest.Verdict
	for _, c := range g.Cases() {
		gs = append(gs, c.Run(context.Background()))
	}
	none, err = g.Update(gs)
	assert.NoError(t, err)
	assert.True(t, none == nil)
	// Without update mode a case with no expect makes the file invalid.
	_, err = risortest.Parse([]byte(text), false, risortest.Update(false))
	assert.Error(t, err)
}

func TestRunUpdatesWithTheVariable(t *testing.T) {
	p := write(t, t.TempDir(), "u.test.json", `[{"name": "a", "source": "2 * 3", "expect": {"value": 0}}]`)
	t.Setenv("RISOR_UPDATE", "1")
	risortest.Run(t, nil, p)
	data, err := os.ReadFile(p)
	assert.NoError(t, err)
	assert.Equal(t, string(data), `[{"name": "a", "source": "2 * 3", "expect": {"value":6}}]`)
}

func TestBaseVarsAndSources(t *testing.T) {
	base, err := risor.NewEnv(
		risor.Var("rate", "float"),
		risor.Function{Name: "lookup", Type: "(code: string) => float !", Impl: func(*risor.Call, []any) (any, error) { return 0.0, nil }},
	)
	assert.NoError(t, err)
	// A stub with no type takes the base's signature; rate comes from the
	// host's variables, and the source from the reader.
	text := `[{"name": "a", "file": "prog.risor", "functions": {"lookup": {"calls": [{"args": ["A"], "result": 2}]}},
  "expect": {"value": 5}}]`
	read := func(ref string) (string, risor.Origin, error) {
		assert.Equal(t, ref, "prog.risor")
		return `lookup("A") * 2 + rate`, risor.Origin{Name: ref}, nil
	}
	f, err := risortest.Parse([]byte(text), false, risortest.Base(base), risortest.Sources(read),
		risortest.Vars(risor.Map{"rate": 1.0}))
	assert.NoError(t, err)
	v := f.Cases()[0].Run(context.Background())
	assert.Equal(t, v.Status, "pass", v.Report())
	// Against the default environment the untyped stub is an invalid file.
	_, err = risortest.Parse([]byte(text), false, risortest.Sources(read))
	assert.Error(t, err)
}

func TestProgramFile(t *testing.T) {
	p := write(t, t.TempDir(), "p.test.risor", `{cases: [1, 2].map(n => {name: `+"`n${n}`"+`, source: `+"`${n} + 1`"+`, expect: {value: n + 1}})}`)
	f, err := risortest.Load(p)
	assert.NoError(t, err)
	assert.Equal(t, len(f.Cases()), 2)
	for _, c := range f.Cases() {
		assert.Equal(t, c.Run(context.Background()).Status, "pass")
	}
	_, err = risortest.Load(write(t, t.TempDir(), "q.test.risor", `{cases: q}`))
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "undefined-name"))
}

func TestUpdateWaitsForEverySelectedCase(t *testing.T) {
	// The file is written once, when every case the helper selects has run
	// (F9 4.2 item 4). The selected cases are those its filter keeps.
	text := "[\n  {\"name\": \"one\", \"source\": \"1 + 2\"},\n  {\"name\": \"two\", \"source\": \"2 + 2\"}\n]\n"
	f, err := risortest.Parse([]byte(text), false, risortest.Update(true))
	assert.NoError(t, err)
	first := f.Cases()[0].Run(context.Background())
	assert.Equal(t, first.Status, "updated")
	// The host's framework ran only the first: the file is left as it was.
	out, err := f.Update([]risortest.Verdict{first})
	assert.NoError(t, err)
	assert.True(t, out == nil)
	// The same case twice is not every case.
	out, err = f.Update([]risortest.Verdict{first, first})
	assert.NoError(t, err)
	assert.True(t, out == nil)
	second := f.Cases()[1].Run(context.Background())
	out, err = f.Update([]risortest.Verdict{first, second})
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(out), `"value":3`) && strings.Contains(string(out), `"value":4`))
	// With the helper's own filter, the one case it selects is every case.
	g, err := risortest.Parse([]byte(text), false, risortest.Update(true), risortest.Filter("two"))
	assert.NoError(t, err)
	assert.Equal(t, len(g.Cases()), 1)
	out, err = g.Update([]risortest.Verdict{g.Cases()[0].Run(context.Background())})
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(out), `"value":4`) && !strings.Contains(string(out), `"value":3`))
}

func TestCaseRunsInTheBaseOfItsLoad(t *testing.T) {
	// A case runs in a child of the base its file was loaded with (F9 7.1,
	// 7.2): rate is declared by the base, not by the case.
	base, err := risor.NewEnv(risor.Var("rate", "float"))
	assert.NoError(t, err)
	text := []byte(`[{"name": "a", "source": "rate * 2", "vars": {"rate": 2}, "expect": {"value": 4}}]`)
	f, err := risortest.Parse(text, false, risortest.Base(base))
	assert.NoError(t, err)
	v := f.Cases()[0].Run(context.Background())
	assert.Equal(t, v.Status, "pass", v.Report())
	// Loaded with no base, the base is the default environment, which does
	// not declare rate: the file is invalid at the load.
	_, err = risortest.Parse(text, false)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), `vars has "rate", which env does not declare`))
}
