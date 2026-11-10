package eval

import (
	"strings"
	"testing"

	"github.com/deepnoodle-ai/risor/v3/internal/library"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
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

// add and mul on whole numbers give what exact integer arithmetic gives, and
// fail exactly when it is out of range.
func TestWholeArithmeticExact(t *testing.T) {
	const max = 9007199254740991
	vals := []int64{0, 1, -1, 2, 3, 1023, 1024, 94906265, 94906266, 1 << 26, 1<<26 + 1,
		max / 2, max/2 + 1, max - 1, max, -max, -(max - 1), 1 << 52, 3 << 51}
	r := &run{}
	try := func(f func() any) (v any, failed bool) {
		defer func() {
			if x := recover(); x != nil {
				failed = true
			}
		}()
		return f(), false
	}
	for _, a := range vals {
		for _, b := range vals {
			s := a + b
			v, failed := try(func() any { return r.add(float64(a), float64(b), "+", syntax.Span{}) })
			if s > max || s < -max {
				assert.True(t, failed)
			} else {
				assert.False(t, failed)
				assert.Equal(t, v, any(float64(s)))
			}
			p, ok := mulWhole(a, b)
			v, failed = try(func() any { return r.mul(float64(a), float64(b), "*", syntax.Span{}) })
			assert.Equal(t, failed, !ok)
			if ok {
				assert.Equal(t, v, any(float64(p)))
			}
		}
	}
}
