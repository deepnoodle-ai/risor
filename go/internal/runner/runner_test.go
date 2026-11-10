package runner

import (
	"os"
	"path/filepath"
	"testing"

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
		"needs the checker":                     Skip,
		"skipped":                               Skip,
		"bad env":                               Fail,
	})
	assert.Equal(t, rs[1].Diffs[0].Name, "diagnostics")
	assert.Equal(t, rs[2].Diffs[0].Name, "outcome")
	assert.Equal(t, Counts(rs), map[Verdict]int{Pass: 3, Fail: 4, Skip: 2, Invalid: 0})

	assert.NoError(t, os.WriteFile(path, []byte(`[{"name": "a"}, {"name": "b"}]`), 0o644))
	rs = RunFile(path)
	assert.Equal(t, len(rs), 2)
	assert.Equal(t, rs[0].Verdict, Invalid)
}
