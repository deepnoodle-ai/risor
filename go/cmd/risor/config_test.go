package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

const appConfig = "const mb = 1024 * 1024;\n{listen: `:${env?.PORT ?? \"8080\"}`, max_body: 2 * mb, name: n}\n"

func TestEvalJSON(t *testing.T) {
	dir := t.TempDir()
	app := put(t, dir, "app.risor", appConfig)
	t.Setenv("PORT", "9000")
	code, out, errOut := runCLI("eval", app, "--format", "json", "--env", "PORT", "--var", "n=x")
	assert.Equal(t, code, 0, errOut)
	assert.Equal(t, out, `{"listen":":9000","max_body":2097152,"name":"x"}`+"\n")
	// An unset process variable is an absent key.
	code, out, _ = runCLI("eval", app, "--format=json", "--env", "NOT_SET_ANYWHERE_F10", "--var", "n=x")
	assert.Equal(t, code, 0)
	assert.Equal(t, out, `{"listen":":8080","max_body":2097152,"name":"x"}`+"\n")
}

func TestEvalText(t *testing.T) {
	dir := t.TempDir()
	app := put(t, dir, "a.risor", `{b: "x", a: [1, 2.5]}`)
	code, out, _ := runCLI("eval", app)
	assert.Equal(t, code, 0)
	assert.Equal(t, out, `{"a":[1,2.5],"b":"x"}`+"\n")
	s := put(t, dir, "s.risor", `"plain"`)
	code, out, _ = runCLI("eval", s)
	assert.Equal(t, code, 0)
	assert.Equal(t, out, "plain\n")
}

func TestEvalTypeAndSchema(t *testing.T) {
	dir := t.TempDir()
	app := put(t, dir, "app.risor", appConfig)
	schema := put(t, dir, "s.json", `{"type": "object", "properties": {"listen": {"type": "string"}, "max_body": {"type": "integer"}}, "required": ["listen"]}`)
	// The schema is closed: name is a key it does not list.
	code, out, errOut := runCLI("eval", app, "--schema", schema, "--var", "n=x", "--env", "PORT")
	assert.Equal(t, code, 1)
	assert.Equal(t, out, "")
	assert.True(t, strings.Contains(errOut, "result-mismatch"), errOut)
	code, out, _ = runCLI("eval", app, "--type", "{listen: string, max_body: int, name: string}", "--var", "n=x", "--env", "PORT", "--format", "json")
	assert.Equal(t, code, 0)
	assert.True(t, strings.HasPrefix(out, `{"listen":`))
	// Step 4 through any: a value from --vars typed any.
	vars := put(t, dir, "v.json", `{"cfg": {"port": "80"}}`)
	any := put(t, dir, "any.risor", `cfg`)
	code, _, _ = runCLI("eval", any, "--vars", vars, "--type", "{port: string}")
	assert.Equal(t, code, 0)
	code, _, errOut = runCLI("eval", any, "--vars", vars, "--type", "{port: int}")
	assert.Equal(t, code, 1)
	assert.True(t, strings.Contains(errOut, "result-mismatch"), errOut)
}

func TestEvalRunAndJSONFailures(t *testing.T) {
	dir := t.TempDir()
	run := put(t, dir, "r.risor", `fail("no")`)
	code, out, errOut := runCLI("eval", run)
	assert.Equal(t, code, 1)
	assert.Equal(t, out, "")
	assert.True(t, strings.Contains(errOut, "error[failed]"), errOut)
	parse := put(t, dir, "p.risor", `{a: `)
	code, _, _ = runCLI("eval", parse, "--format", "json")
	assert.Equal(t, code, 1)
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	app := put(t, dir, "app.risor", appConfig)
	code, out, errOut := runCLI("check", app, "--var", "n=x", "--env", "PORT", "--type", "{listen: string, max_body: int, name: string}")
	assert.Equal(t, code, 0, errOut)
	assert.Equal(t, out, "")
	code, out, _ = runCLI("check", app, "--var", "n=x", "--env", "PORT", "--type", "{listen: string}")
	assert.Equal(t, code, 1)
	assert.Equal(t, strings.Count(out, "result-mismatch"), 2) // check writes its diagnostics to standard output (F12 4.5)
	// check does not run: a program that would fail its run checks.
	f := put(t, dir, "f.risor", `fail("x")`)
	code, _, _ = runCLI("check", f)
	assert.Equal(t, code, 0)
	// A misspelled setting of an undeclared variable.
	code, out, _ = runCLI("check", app)
	assert.Equal(t, code, 1)
	assert.True(t, strings.Contains(out, "undefined-name"))
}

func TestConfigCommandLineErrors(t *testing.T) {
	dir := t.TempDir()
	app := put(t, dir, "app.risor", "1")
	vars := put(t, dir, "v.json", `{"n": 1}`)
	for _, args := range [][]string{
		{"eval"},
		{"eval", app, "--type", "int", "--schema", "s.json"},
		{"eval", app, "--format", "yaml"},
		{"eval", app, "--var", "n=1", "--vars", vars},
		{"eval", app, "--var", "n=1", "--var", "n=2"},
		{"eval", filepath.Join(dir, "missing.risor")},
		{"check", app, "--pretty"},
	} {
		code, _, _ := runCLI(args...)
		assert.Equal(t, code, 2, args)
	}
}
