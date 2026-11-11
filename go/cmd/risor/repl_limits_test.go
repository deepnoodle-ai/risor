package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestReplHeldConstsDoNotRechargeBudget(t *testing.T) {
	env, err := risor.NewEnv(risor.Limits{Steps: 600, Alloc: 600})
	assert.NoError(t, err)
	s := &session{env: env, vars: risor.Map{}}

	// Each range(500) takes 501 steps and 501 allocation units. Re-evaluating
	// held lines on later entries would exceed the 600-step / 600-unit limit.
	assert.Equal(t, s.eval("const xs = range(500);"), result{})
	assert.Equal(t, s.eval("const ys = range(500);"), result{})
	assert.Equal(t, s.eval("len(xs) + len(ys)"), result{value: "1000\n"})

	// Destructuring patterns, discard bindings, and lambdas closing over held
	// values keep their values without re-charging.
	assert.Equal(t, s.eval("const [first, ...rest] = xs;"), result{})
	assert.Equal(t, s.eval("const {a, ...more} = {a: 10, b: 20, c: 30};"), result{})
	assert.Equal(t, s.eval("const _ = range(500);"), result{})
	assert.Equal(t, s.eval("const addFirst = z => z + first + a + len(rest) + len(more);"), result{})
	assert.Equal(t, s.eval("addFirst(5)"), result{value: "516\n"})

	// Shadowing a held binding preserves the old value for held lambdas while
	// exposing the new value to later entries, still without re-charging.
	assert.Equal(t, s.eval("const a = 99;"), result{})
	assert.Equal(t, s.eval("[a, addFirst(5)]"), result{value: "[99,516]\n"})

	// An entry that itself exceeds the limit still fails with the session's
	// configured limit in the diagnostic, and holds nothing.
	r := s.eval("const tooBig = range(700);")
	assert.Equal(t, r.diags, "error[steps-exceeded]: the run needed more than its limit of 600 steps\n")
	assert.Equal(t, s.eval("len(xs)"), result{value: "500\n"})

	// :reset clears held bindings and cached slot values.
	in := &scanReader{sc: bufio.NewScanner(strings.NewReader(":reset\nxs\nconst ok = 42;\nok\n"))}
	var out, errb bytes.Buffer
	assert.Equal(t, repl(s, in, &out, &errb), 0)
	assert.Equal(t, errb.String(), "")
	assert.True(t, strings.Contains(out.String(), "error[undefined-name]"), out.String())
	assert.True(t, strings.HasSuffix(out.String(), "42\n"), out.String())
}
