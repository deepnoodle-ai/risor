package main

import (
	"errors"
	"strings"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

func TestLimitFlagsAndWholeBounds(t *testing.T) {
	dir := t.TempDir()
	lit := put(t, dir, "lit.risor", "1\n")
	alloc := put(t, dir, "alloc.risor", "[1]\n")

	// --allocation 0 allows literal evaluation (0 units) and stops [1] (2 units).
	code, out, errOut := runCLI("eval", lit, "--allocation", "0")
	assert.Equal(t, code, 0, errOut)
	assert.Equal(t, out, "1\n")
	code, _, errOut = runCLI("eval", alloc, "--allocation", "0")
	assert.Equal(t, code, 1)
	assert.True(t, strings.Contains(errOut, "error[allocation-exceeded]"), errOut)

	// --steps 0 stops even a 1-step binary operator.
	add := put(t, dir, "add.risor", "1 + 2\n")
	code, _, errOut = runCLI("eval", add, "--steps", "0")
	assert.Equal(t, code, 1)
	assert.True(t, strings.Contains(errOut, "error[steps-exceeded]"), errOut)

	// --depth 0 and --depth 1001 are declare-phase bad-limit errors (exit 1).
	for _, d := range []string{"0", "1001"} {
		code, _, errOut = runCLI("eval", lit, "--depth", d)
		assert.Equal(t, code, 1, d)
		assert.True(t, strings.Contains(errOut, "error[bad-limit]"), errOut)
	}

	// Whole numbers stop at 2^53 - 1 (9007199254740991).
	for _, flag := range []string{"--steps", "--allocation"} {
		code, _, errOut = runCLI("eval", lit, flag, "9007199254740991")
		assert.Equal(t, code, 0, errOut)
	}
	for _, flag := range []string{"--steps", "--allocation", "--depth"} {
		for _, bad := range []string{"-1", "1.5", "9007199254740992"} {
			code, _, _ = runCLI("eval", lit, flag, bad)
			assert.Equal(t, code, 2, flag+"="+bad)
		}
	}
}

func TestDescriptionLimitsAndCLIOverrides(t *testing.T) {
	dir := t.TempDir()
	// A description with null steps/allocation turns both limits off; CLI flags
	// replace only the limits named on the command line (F12 4.3 item 2).
	desc := put(t, dir, "desc.json", `{"functions":{},"limits":{"allocation":null,"depth":50,"steps":null},"opaque":{},"reject":["constant-condition"],"removed":[],"variables":{},"version":"3.0"}`)
	prog := put(t, dir, "p.risor", "range(200).sum()\n")

	code, out, errOut := runCLI("eval", prog, "--description", desc)
	assert.Equal(t, code, 0, errOut)
	assert.Equal(t, out, "19900\n")

	code, _, errOut = runCLI("eval", prog, "--description", desc, "--steps", "10")
	assert.Equal(t, code, 1)
	assert.True(t, strings.Contains(errOut, "error[steps-exceeded]"), errOut)

	code, _, errOut = runCLI("eval", prog, "--description", desc, "--allocation", "10")
	assert.Equal(t, code, 1)
	assert.True(t, strings.Contains(errOut, "error[allocation-exceeded]"), errOut)

	deep := put(t, dir, "deep.risor", "[[[[[[1]]]]]] == [[[[[[1]]]]]]\n")
	code, out, errOut = runCLI("eval", deep, "--description", desc)
	assert.Equal(t, code, 0, errOut)
	assert.Equal(t, out, "true\n")

	code, _, errOut = runCLI("eval", deep, "--description", desc, "--depth", "5")
	assert.Equal(t, code, 1)
	assert.True(t, strings.Contains(errOut, "error[depth-exceeded]"), errOut)

	// Host Limits and Description enforce the 2^53 - 1 whole-number bound.
	for _, lim := range []risor.Limits{
		{Steps: 1 << 53},
		{Alloc: 1 << 53},
	} {
		_, err := risor.NewEnv(lim)
		var e *risor.Error
		assert.True(t, errors.As(err, &e))
		assert.Equal(t, e.Diagnostics[0].Code, "bad-limit")
	}
	for _, text := range []string{
		`{"functions":{},"limits":{"allocation":1,"depth":200,"steps":9007199254740992},"opaque":{},"reject":[],"removed":[],"variables":{},"version":"3.0"}`,
		`{"functions":{},"limits":{"allocation":9007199254740992,"depth":200,"steps":1},"opaque":{},"reject":[],"removed":[],"variables":{},"version":"3.0"}`,
	} {
		decl, err := risor.Description([]byte(text))
		assert.NoError(t, err)
		_, err = risor.NewEnv(decl)
		var e *risor.Error
		assert.True(t, errors.As(err, &e))
		assert.Equal(t, e.Diagnostics[0].Code, "bad-limit")
	}
}
