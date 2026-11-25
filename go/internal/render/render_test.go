package render

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
)

// Package F19 5.2: the JSON form writes a cause that is a Risor diagnostic,
// to a depth of 8, found through wrapping; any other cause is not written.
func TestJSONCause(t *testing.T) {
	d := diag.Diagnostic{Phase: diag.PhaseRun, Code: "failed", Message: "m", HasData: true}
	for i := 0; i < 10; i++ {
		inner := &diag.Error{Diagnostics: []diag.Diagnostic{d}}
		d = diag.Diagnostic{Phase: diag.PhaseRun, Code: "function-error", Message: "m",
			Cause: fmt.Errorf("wrapped: %w", inner)}
	}
	out := JSON([]diag.Diagnostic{d})
	assert.Equal(t, strings.Count(out, `"cause"`), 8)

	plain := diag.Diagnostic{Phase: diag.PhaseRun, Code: "function-error", Message: "m", Cause: errors.New("m")}
	assert.Equal(t, JSON([]diag.Diagnostic{plain}), `{"code":"function-error","message":"m","phase":"run","trace":[]}`+"\n")
}
