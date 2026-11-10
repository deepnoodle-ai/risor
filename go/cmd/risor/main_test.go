package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func put(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	assert.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	assert.NoError(t, os.WriteFile(p, []byte(text), 0o644))
	return p
}

func TestAllPass(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "a.test.json", `[{"name": "a", "source": "1", "expect": {"value": 1}}]`)
	code, out, _ := runCLI("test", dir)
	assert.Equal(t, code, 0)
	assert.Equal(t, out, "1 files, 1 cases: 1 pass, 0 fail, 0 skip, 0 invalid\n")
}

func TestFailAndReport(t *testing.T) {
	dir := t.TempDir()
	p := put(t, dir, "a.test.json", `[{"name": "a", "source": "1 + 2", "expect": {"value": 4}},
 {"name": "s", "source": "1", "skip": "later", "expect": {}}]`)
	code, out, _ := runCLI("test", p)
	assert.Equal(t, code, 1)
	assert.Equal(t, out, p+"\n  FAIL a\n    value: expected 4, found 3\n  SKIP s: later\n"+
		"1 files, 2 cases: 0 pass, 1 fail, 1 skip, 0 invalid\n")
}

func TestInvalidCaseAndFile(t *testing.T) {
	dir := t.TempDir()
	p := put(t, dir, "a.test.json", `{"env": "{n: int}", "cases": [{"name": "a", "source": "n", "vars": {"n": "x"}, "expect": {"value": 1}}]}`)
	code, out, _ := runCLI("test", p)
	assert.Equal(t, code, 2)
	assert.Equal(t, out, p+"\n  INVALID a\n    fixture .vars.n: expected int, found \"x\"\n"+
		"1 files, 1 cases: 0 pass, 0 fail, 0 skip, 1 invalid\n")
	bad := put(t, dir, "b.test.json", `{"cases": [`)
	code, out, _ = runCLI("test", bad)
	assert.Equal(t, code, 2)
	assert.True(t, bytes.HasPrefix([]byte(out), []byte(bad+"\n  INVALID\n    ")))
	assert.True(t, bytes.HasSuffix([]byte(out), []byte("1 files, 0 cases: 0 pass, 0 fail, 0 skip, 0 invalid; 1 files invalid\n")))
	code, _, errOut := runCLI("test", filepath.Join(dir, "nope"))
	assert.Equal(t, code, 2)
	assert.True(t, len(errOut) > 0)
}

func TestFindsFilesInOrder(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "b.test.json", `[{"name": "b", "source": "1", "expect": {"value": 2}}]`)
	put(t, dir, "a/z.test.risor", `[{name: "z", source: "1", expect: {value: 2}}]`)
	put(t, dir, "a.b.test.json", `[{"name": "ab", "source": "1", "expect": {"value": 2}}]`)
	put(t, dir, ".hidden/x.test.json", `[{"name": "x", "source": "1", "expect": {"value": 2}}]`)
	put(t, dir, "notes.json", `[]`)
	code, out, _ := runCLI("test", "--format", "json", dir)
	assert.Equal(t, code, 1)
	// a/z before a.b: parts are compared one by one, "a" < "a.b.test.json".
	want := `{"differs":["value"],"file":` + q(filepath.Join(dir, "a", "z.test.risor")) + `,"name":"z","status":"fail"}` + "\n" +
		`{"differs":["value"],"file":` + q(filepath.Join(dir, "a.b.test.json")) + `,"name":"ab","status":"fail"}` + "\n" +
		`{"differs":["value"],"file":` + q(filepath.Join(dir, "b.test.json")) + `,"name":"b","status":"fail"}` + "\n"
	assert.Equal(t, out, want)
}

func q(s string) string {
	var b bytes.Buffer
	b.WriteByte('"')
	for _, r := range s {
		if r == '\\' || r == '"' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

func TestRunFilterAndUpdate(t *testing.T) {
	dir := t.TempDir()
	p := put(t, dir, "u.test.json", `[{"name": "one", "source": "1"}, {"name": "two", "source": "2", "expect": {"value": 0}}]`)
	code, out, _ := runCLI("test", "--update", "--run", "two", p)
	assert.Equal(t, code, 0)
	assert.Equal(t, out, p+"\n  UPDATED two\n1 files, 1 cases: 0 pass, 0 fail, 0 skip, 0 invalid, 1 updated\n")
	data, _ := os.ReadFile(p)
	assert.Equal(t, string(data), `[{"name": "one", "source": "1"}, {"name": "two", "source": "2", "expect": {"value":2}}]`)
	// Without --update, the case with no expect makes the file invalid.
	code, _, _ = runCLI("test", p)
	assert.Equal(t, code, 2)
}

func TestJSONLines(t *testing.T) {
	dir := t.TempDir()
	p := put(t, dir, "a.test.json", `{"env": "{n: int}", "cases": [
 {"name": "s", "source": "1", "skip": "later", "expect": {}},
 {"name": "i", "source": "n", "vars": {"n": "x"}, "expect": {}},
 {"name": "p", "source": "1", "expect": {"value": 1}}]}`)
	code, out, _ := runCLI("test", "--format=json", p)
	assert.Equal(t, code, 2)
	f := q(p)
	assert.Equal(t, out, `{"file":`+f+`,"name":"s","reason":"later","status":"skip"}`+"\n"+
		`{"file":`+f+`,"fixture":["vars","n"],"message":"expected int, found \"x\"","name":"i","status":"invalid"}`+"\n"+
		`{"file":`+f+`,"name":"p","status":"pass"}`+"\n")
}

func TestUsage(t *testing.T) {
	code, _, errOut := runCLI()
	assert.Equal(t, code, 2)
	assert.True(t, len(errOut) > 0)
	code, _, _ = runCLI("test", "--format", "xml")
	assert.Equal(t, code, 2)
	code, _, _ = runCLI("eval")
	assert.Equal(t, code, 2)
}
