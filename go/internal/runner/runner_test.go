package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/go/v3/internal/casefile"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestVerdicts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cases.json")
	assert.NoError(t, os.WriteFile(path, []byte(`[
		{"name": "right code and span", "source": "1 +",
		 "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token", "span": [1, 4, 1, 4]}]}},
		{"name": "wrong span", "source": "1 +",
		 "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token", "span": [1, 3, 1, 4]}]}},
		{"name": "parses but a failure is expected", "source": "1",
		 "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token"}]}},
		{"name": "a value from text that does not parse", "source": "1 +",
		 "expect": {"value": 1}},
		{"name": "a value", "source": "1 + 1", "expect": {"value": 2}},
		{"name": "only checks", "source": "1 + 1", "expect": {}},
		{"name": "needs the checker", "source": "1", "expect": {"type": "int"}},
		{"name": "skipped", "source": "1", "skip": "why", "expect": {}},
		{"name": "bad env", "source": "1", "env": "{a: number}",
		 "expect": {"phase": "parse", "diagnostics": [{"code": "unexpected-token"}]}}
	]`), 0o644))
	rs := RunFile(path)
	got := map[string]Verdict{}
	for _, r := range rs {
		got[r.Case] = r.Verdict
	}
	assert.Equal(t, got, map[string]Verdict{
		"right code and span":                   Pass,
		"wrong span":                            Fail,
		"parses but a failure is expected":      Fail,
		"a value from text that does not parse": Fail,
		"a value":                               Pass,
		"only checks":                           Pass,
		"needs the checker":                     Pass,
		"skipped":                               Skip,
		"bad env":                               Fail,
	})
	assert.Equal(t, rs[1].Diffs[0].Name, "diagnostics")
	assert.Equal(t, rs[2].Diffs[0].Name, "outcome")
	assert.Equal(t, Counts(rs), map[Verdict]int{Pass: 4, Fail: 4, Skip: 1, Invalid: 0})

	assert.NoError(t, os.WriteFile(path, []byte(`[{"name": "a"}, {"name": "b"}]`), 0o644))
	rs = RunFile(path)
	assert.Equal(t, len(rs), 2)
	assert.Equal(t, rs[0].Verdict, Invalid)
}

func TestOutcomeDescriptionWithEmptyFacts(t *testing.T) {
	// F14 10.4: a case with facts, even an empty list, in the case or in
	// the header, writes description after name; a case without does not.
	path := filepath.Join(t.TempDir(), "facts.json")
	assert.NoError(t, os.WriteFile(path, []byte(`[
		{"name": "empty", "env": "{a: int}", "facts": [], "source": "1"},
		{"name": "none", "env": "{a: int}", "source": "1"}
	]`), 0o644))
	f, err := casefile.LoadNoExpect(path)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(Outcome(f.Cases[0]), `{"name":"empty","description":`), Outcome(f.Cases[0]))
	assert.False(t, strings.Contains(Outcome(f.Cases[1]), `"description"`), Outcome(f.Cases[1]))
	head := filepath.Join(t.TempDir(), "head.json")
	assert.NoError(t, os.WriteFile(head, []byte(`{"facts": [], "cases": [
		{"name": "from the header", "env": "{a: int}", "source": "1"}
	]}`), 0o644))
	f, err = casefile.LoadNoExpect(head)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(Outcome(f.Cases[0]), `{"name":"from the header","description":`), Outcome(f.Cases[0]))
}
