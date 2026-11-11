package eval

import (
	"math"

	"github.com/deepnoodle-ai/risor/go/v3/internal/crmath"
	"github.com/deepnoodle-ai/risor/go/v3/internal/messages"
)

// This file is the eleven entries of package F13, math functions
// (spec/packages/f13-math.md): each gives the correctly rounded result,
// computed by internal/crmath.

// mathEntries are the eleven entries; groups.go adds them to the entries of
// the groups. Each makes, in the order of f13-math.md 3: the
// not-real test, the record's steps (200), then the computation.
var mathEntries = map[string]libFunc{
	"math.pow":   mathTwo(crmath.Pow, func(x, y float64) bool { return (x == 0 && y < 0) || (x < 0 && y != math.Trunc(y)) }),
	"math.exp":   mathOne(crmath.Exp, nil),
	"math.log":   mathOne(crmath.Log, func(x float64) bool { return x <= 0 }),
	"math.log10": mathOne(crmath.Log10, func(x float64) bool { return x <= 0 }),
	"math.sin":   mathOne(crmath.Sin, nil),
	"math.cos":   mathOne(crmath.Cos, nil),
	"math.tan":   mathOne(crmath.Tan, nil),
	"math.asin":  mathOne(crmath.Asin, outsideUnit),
	"math.acos":  mathOne(crmath.Acos, outsideUnit),
	"math.atan":  mathOne(crmath.Atan, nil),
	"math.atan2": mathTwo(crmath.Atan2, nil),
}

func outsideUnit(x float64) bool { return x < -1 || x > 1 }

func mathOne(f func(float64) (float64, crmath.Status), notReal func(float64) bool) libFunc {
	return func(r *run, c *libCall) any {
		x := c.args[0].(float64)
		if notReal != nil && notReal(x) {
			r.invalid(c, "not-real", messages.Word("name", c.e.Name))
		}
		r.charge(c, fenv{c: c})
		v, st := f(x)
		return r.mathResult(c, v, st)
	}
}

func mathTwo(f func(float64, float64) (float64, crmath.Status), notReal func(float64, float64) bool) libFunc {
	return func(r *run, c *libCall) any {
		x, y := c.args[0].(float64), c.args[1].(float64)
		if notReal != nil && notReal(x, y) {
			r.invalid(c, "not-real", messages.Word("name", c.e.Name))
		}
		r.charge(c, fenv{c: c})
		v, st := f(x, y)
		return r.mathResult(c, v, st)
	}
}

func (r *run) mathResult(c *libCall, v float64, st crmath.Status) any {
	switch st {
	case crmath.NotReal:
		r.invalid(c, "not-real", messages.Word("name", c.e.Name))
	case crmath.Overflow:
		r.entryRange(c)
	}
	return zero(v)
}
