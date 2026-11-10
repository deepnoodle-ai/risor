package eval

import (
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A quick formula gives the value the formula tree gives, for every entry,
// over sizes that include the saturation points.
func TestQuickFormulaMatchesTree(t *testing.T) {
	sizes := []int64{0, 1, 2, 63, 64, 1000, 1 << 40, 1<<62 - 1, 1 << 62}
	texts := []any{nil, "", "abc", strings.Repeat("é", 100), strings.Repeat("x", 200), 3.0}
	quick := 0
	for _, e := range library.Entries {
		in := infoOf(e)
		for _, pair := range []struct {
			q *quickFormula
			f *library.Formula
		}{{&in.steps, e.StepsF}, {&in.units, e.UnitsF}} {
			if pair.q.kind == 0 {
				continue
			}
			quick++
			for _, n := range sizes {
				for _, txt := range texts {
					args := make([]any, len(e.Params))
					for i := range args {
						args[i] = txt
					}
					c := &libCall{e: e, info: in, recv: txt, args: args}
					f := fenv{c: c, n: n, k: n / 2, v: n}
					want := pair.f.Eval(&f)
					got := pair.q.eval(c, &f)
					assert.Equal(t, got, want, Key(e)+" "+pair.f.String())
				}
			}
		}
	}
	assert.True(t, quick > 40)
}
