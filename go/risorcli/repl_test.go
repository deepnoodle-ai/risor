package risorcli

import (
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/assert"
)

func TestSessionHeldInvariants(t *testing.T) {
	env, err := risor.NewEnv()
	assert.NoError(t, err)
	s := NewSession(env, risor.Map{})

	assert.Equal(t, s.Eval("const a = 7;"), EntryResult{})
	held := s.held
	assert.Equal(t, s.heldVals.Lines(), 1)

	// A failing entry holds nothing.
	r := s.Eval("const z = 1 / 0;")
	assert.True(t, len(r.Diagnostics) > 0)
	assert.Equal(t, s.held, held)
	assert.Equal(t, s.heldVals.Lines(), 1)

	// An export-only entry holds its const lines.
	const exportLine = "export const g = (x: int): int => x + 1;"
	assert.Equal(t, s.Eval(exportLine), EntryResult{})
	assert.Equal(t, s.held, held+"\n"+exportLine)
	assert.Equal(t, s.Eval("g(41)").Value, "42\n")

	// A companion const + export-only entry holds both.
	const multiExport = "const inc = 5;\nexport const addInc = (x: int): int => x + inc;"
	heldWithG := s.held
	assert.Equal(t, s.Eval(multiExport), EntryResult{})
	assert.Equal(t, s.held, heldWithG+"\n"+multiExport)
	assert.Equal(t, s.Eval("addInc(37)").Value, "42\n")

	// A runtime-failing export-only entry holds nothing.
	heldBeforeFail := s.held
	heldLinesBeforeFail := s.heldVals.Lines()
	r = s.Eval("export const bad = (x: int): int => x;\nconst boom = 1 / (a - 7);")
	assert.True(t, len(r.Diagnostics) > 0 && r.Diagnostics[0].Code == "divide-by-zero")
	assert.Equal(t, s.held, heldBeforeFail)
	assert.Equal(t, s.heldVals.Lines(), heldLinesBeforeFail)

	// Reset forgets all held lines and cached values.
	s.Reset()
	assert.Equal(t, s.held, "")
	assert.Equal(t, s.heldVals.Lines(), 0)
}
