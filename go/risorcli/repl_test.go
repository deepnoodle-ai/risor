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

func TestStandaloneParseErrorsDoNotUsePriorShadowMap(t *testing.T) {
	env, err := risor.NewEnv()
	assert.NoError(t, err)
	s := NewSession(env, risor.Map{})

	// Shadowing `a` rewrites `const a = 2;` to `const a__1 = 2;`.
	assert.Equal(t, s.Eval("const a = 2;"), EntryResult{})
	assert.Equal(t, s.Eval("const a = 10;"), EntryResult{})

	// EOFError parses the pending input standalone and must report coordinates
	// in the pending input, not shifted by the previous entry's rename map.
	ds, err := s.EOFError("12345 +")
	assert.NoError(t, err)
	assert.True(t, len(ds) > 0 && ds[0].Span != nil)
	assert.Equal(t, ds[0].Span.Start, risor.Pos{Line: 1, Column: 8})

	// Immediate standalone parse errors in Eval must also keep their own coordinates.
	r := s.Eval("12345 + )")
	assert.NoError(t, r.Err)
	assert.True(t, len(r.Diagnostics) > 0 && r.Diagnostics[0].Span != nil)
	assert.Equal(t, r.Diagnostics[0].Span.Start, risor.Pos{Line: 1, Column: 9})
}
