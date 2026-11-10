package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Tests of the command-line program that a golden directory cannot hold.

func TestInputNotUTF8(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.risor")
	assert.NoError(t, os.WriteFile(p, []byte{'1', 0xff, '\n'}, 0o644))
	for _, args := range [][]string{{"eval", p}, {"check", p}, {"fmt", p}} {
		code, out, errOut := runCLI(args...)
		assert.Equal(t, code, 2, args)
		assert.Equal(t, out, "", args)
		assert.True(t, strings.Contains(errOut, "not UTF-8"), args)
	}
	var out, errb bytes.Buffer
	code := runStdin([]string{"fmt"}, bytes.NewReader([]byte{0xfe}), &out, &errb)
	assert.Equal(t, code, 2)
}

func TestColorWhen(t *testing.T) {
	var b bytes.Buffer
	t.Setenv("NO_COLOR", "1")
	assert.True(t, (&colorFlag{"always"}).on(&b, &b)) // always ignores NO_COLOR
	assert.False(t, (&colorFlag{"never"}).on(&b, &b))
	assert.False(t, (&colorFlag{"auto"}).on(&b, &b))
	t.Setenv("NO_COLOR", "")
	// A buffer is not a terminal: auto writes no colour.
	assert.False(t, (&colorFlag{"auto"}).on(&b, &b))
	assert.False(t, isTerminal(&b))
	code, _, _ := runCLI("eval", "x.risor", "--color", "sometimes")
	assert.Equal(t, code, 2)
}

// Removing every SGR sequence from a coloured rendering gives the text the
// chapter fixes (F12 4.2 item 4).
func TestColorRemoved(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.risor")
	assert.NoError(t, os.WriteFile(p, []byte("const a = nope;\n{x: a + \"s\", y: missing}\n"), 0o644))
	_, plain, _ := runCLI("check", p, "--color", "never")
	_, colored, _ := runCLI("check", p, "--color", "always")
	assert.True(t, strings.Contains(colored, "\x1b["))
	stripped := colored
	for {
		i := strings.Index(stripped, "\x1b[")
		if i < 0 {
			break
		}
		j := strings.IndexByte(stripped[i:], 'm')
		stripped = stripped[:i] + stripped[i+j+1:]
	}
	assert.Equal(t, stripped, plain)
}

func TestFmtWriteLeavesFormattedFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.risor")
	assert.NoError(t, os.WriteFile(p, []byte("x + 1\n"), 0o644))
	st, err := os.Stat(p)
	assert.NoError(t, err)
	code, out, _ := runCLI("fmt", "--write", p)
	assert.Equal(t, code, 0)
	assert.Equal(t, out, "")
	st2, err := os.Stat(p)
	assert.NoError(t, err)
	assert.Equal(t, st2.ModTime(), st.ModTime()) // not written: it did not differ
}

func TestReplSession(t *testing.T) {
	s := &session{}
	var err error
	s.env, s.vars, err = newEnvFlags().environment()
	assert.NoError(t, err)
	assert.True(t, s.eval("[1,").more)
	assert.True(t, s.eval("const a = ").more)
	assert.True(t, s.eval("`open ${").more)
	r := s.eval("const a = 2;")
	assert.Equal(t, r, result{})
	assert.Equal(t, s.eval("a * 3").value, "6\n")
	// A held lambda keeps the binding it was made with.
	s.eval("const f = x => x * a;")
	s.eval("const a = 10;")
	assert.Equal(t, s.eval("[a, f(1)]").value, "[10,2]\n")
	// A pattern binds again by shadowing too.
	s.eval("const {a, b} = {a: 7, b: 8};")
	assert.Equal(t, s.eval("[a, b, f(1)]").value, "[7,8,2]\n")
	assert.Equal(t, s.eval("{a}").value, "{\"a\":7}\n")
	// A failing entry holds nothing.
	held := s.held
	r = s.eval("const z = 1 / 0;")
	assert.True(t, r.diags != "")
	assert.Equal(t, s.held, held)
	assert.Equal(t, s.eval("\"s\"").value, "\"s\"\n")
}

func TestReplShowsAuthoredNames(t *testing.T) {
	s := &session{}
	var err error
	s.env, s.vars, err = newEnvFlags().environment()
	assert.NoError(t, err)
	// A held lambda that calls a name bound again keeps the old binding.
	assert.Equal(t, s.eval("const f = x => 1 / x;"), result{})
	assert.Equal(t, s.eval("const g = y => f(y);"), result{})
	assert.Equal(t, s.eval("const f = 3;"), result{})
	assert.Equal(t, s.eval("[f, g(2)]").value, "[3,0.5]\n")
	r := s.eval("g(0)")
	assert.Equal(t, r.diags, "error[divide-by-zero]: the divisor of / is zero\n  in g, at 1:1-1:5\n")
	// In the entry, a renamed name shows as written, at its own span.
	s.eval("const h = x => 1 / x;")
	r = s.eval("const h = h(1, 2);")
	assert.Equal(t, r.diags, "error[arity] 1:11-1:18: the argument count of h must be 1, found 2\n"+
		"    const h = h(1, 2);\n              ^^^^^^^\n")
	r = s.eval("const h = h(0);")
	assert.Equal(t, r.diags, "error[divide-by-zero]: the divisor of / is zero\n  in h, at 1:11-1:15\n")
	r = s.eval("const h = h;")
	assert.True(t, strings.Contains(r.diags, "1:11-1:12: h is a const-bound lambda"), r.diags)
	assert.False(t, strings.Contains(r.diags, "__"), r.diags)
}

// The rules of the command line that wonton/cli does not keep by itself
// (prepareArgs): each is an error of the command line, status 2 (F12 4.9),
// or a value taken as written.
func TestCommandLineRules(t *testing.T) {
	dir := t.TempDir()
	app := put(t, dir, "app.risor", "{w: who}\n")
	vars := put(t, dir, "v.json", `{"n": 1}`)
	desc := put(t, dir, "d.json", `{"variables": {"who": "string"}}`)
	// A value that begins with - or names a command is the flag's value.
	for _, v := range []string{"-x", "test", "--", "eval"} {
		code, out, errOut := runCLI("eval", app, "--var", "who="+v, "--format", "json")
		assert.Equal(t, code, 0, v, errOut)
		assert.Equal(t, out, `{"w":`+q(v)+"}\n", v)
	}
	code, out, _ := runCLI("eval", app, "--var", "who", "--format=json")
	assert.Equal(t, code, 2, out) // --var takes NAME=TEXT
	for _, args := range [][]string{
		{"eval", app, "--pretty=true"},
		{"fmt", "--write=false", app},
		{"eval", app, "--no-color"},
		{"fmt", "--force-color", app},
		{"eval", app, "--colour", "always"},
		{"eval", app, "--vars", vars, "--vars", vars},
		{"eval", app, "--description", desc, "--description", desc},
		{"eval", app, "--steps", "-1"},
		{"eval", app, "--steps", "x"},
		{"eval", app, "--max-failures", "0"},
		{"eval", app, "--format"},
		{"eval", app, "--color", "sometimes"},
		{"eval", app, "extra"},
		{"check", app, "--pretty"},
		{"eval", app, "--print-type"},
		{"test", "--suite", dir, dir},
		{"fmt", "--type", "int", app},
		{"nope"},
		{"-x"},
	} {
		code, out, errOut := runCLI(args...)
		assert.Equal(t, code, 2, args)
		assert.Equal(t, out, "", args)
		assert.True(t, strings.HasPrefix(errOut, "risor: "), args, errOut)
	}
	// After --, nothing is a flag.
	code, _, errOut := runCLI("fmt", "--", "--no-color")
	assert.Equal(t, code, 2)
	assert.True(t, strings.Contains(errOut, "--no-color cannot be read"), errOut)
}

func TestHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"eval", "--help"}, {"test", "-h"}} {
		code, out, errOut := runCLI(args...)
		assert.Equal(t, code, 0, args)
		assert.True(t, strings.Contains(out, "risor"), args)
		assert.Equal(t, errOut, "", args)
	}
	for _, args := range [][]string{{"version"}, {"--version"}} {
		code, out, _ := runCLI(args...)
		assert.Equal(t, code, 0, args)
		assert.Equal(t, out, version+"\n", args)
	}
	code, out, _ := runCLI("completion", "bash")
	assert.Equal(t, code, 0)
	assert.True(t, strings.Contains(out, "eval"))
}
