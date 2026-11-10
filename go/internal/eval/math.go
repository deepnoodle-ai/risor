package eval

import (
	"github.com/deepnoodle-ai/risor/v3/internal/crmath"
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
)

// This file is the eleven entries of package F13, math functions
// (spec/packages/f13-math.md): each gives the correctly rounded result,
// computed by internal/crmath.

// mathEntries are the eleven entries; groups.go adds them to the entries of
// the groups.
var mathEntries = map[string]libFunc{
	"math.pow":   mathTwo(crmath.Pow),
	"math.exp":   mathOne(crmath.Exp),
	"math.log":   mathOne(crmath.Log),
	"math.log10": mathOne(crmath.Log10),
	"math.sin":   mathOne(crmath.Sin),
	"math.cos":   mathOne(crmath.Cos),
	"math.tan":   mathOne(crmath.Tan),
	"math.asin":  mathOne(crmath.Asin),
	"math.acos":  mathOne(crmath.Acos),
	"math.atan":  mathOne(crmath.Atan),
	"math.atan2": mathTwo(crmath.Atan2),
}

func mathOne(f func(float64) (float64, crmath.Status)) libFunc {
	return func(r *run, c *libCall) any {
		v, st := f(c.args[0].(float64))
		return r.mathResult(c, v, st)
	}
}

func mathTwo(f func(float64, float64) (float64, crmath.Status)) libFunc {
	return func(r *run, c *libCall) any {
		v, st := f(c.args[0].(float64), c.args[1].(float64))
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
