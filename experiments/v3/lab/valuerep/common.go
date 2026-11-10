// Package valuerep asks one question: what does the value representation
// cost on arithmetic and comparison chains, and on plain reads?
//
// Three representations evaluate the same hand-built closure trees:
//
//   - boxed:  func(*frame) any, the spike's design
//   - tagged: func(*frame) Value, a struct with an inline number
//   - typed:  func(*frame) int64 / float64 / bool where the type is known
//
// Every operator charges one step and checks the number range, so the three
// differ only in how a value travels between closures.
//
//	GOWORK=off go test ./lab/valuerep -run xxx -bench . -benchmem -count 6 | go run ./lab/median
package valuerep

import (
	"errors"
	"math"
)

var (
	errBudget   = errors.New("step budget exceeded")
	errType     = errors.New("type error")
	errOverflow = errors.New("integer overflow")
)

type runtime struct {
	globals []any
	steps   int64
	max     int64
}

func (rt *runtime) tick() {
	rt.steps++
	if rt.steps > rt.max {
		panic(errBudget)
	}
}

type frame struct {
	rt     *runtime
	slots  []any   // boxed and typed
	vslots []Value // tagged
	ints   []int64 // typed, with typed slots
}

// maxSafe is 2^53 - 1. A whole number is exact only within ±maxSafe, and an
// integer result outside it is an error. An int64 the evaluator holds is
// always within the range, so a sum or a difference cannot wrap.
const maxSafe = 1<<53 - 1

var errRange = errors.New("number out of range")

func safe(c int64) int64 {
	if uint64(c+maxSafe) > 2*maxSafe {
		panic(errRange)
	}
	return c
}

func addInt(a, b int64) int64 { return safe(a + b) }
func subInt(a, b int64) int64 { return safe(a - b) }

// mulInt checks the range with one float multiply. Both operands convert
// exactly, and rounding is monotonic, so the float product is within the
// range exactly when the true product is.
func mulInt(a, b int64) int64 {
	f := float64(a) * float64(b)
	if f > maxSafe || f < -maxSafe {
		panic(errRange)
	}
	return a * b
}

// finite checks a float result. Anything within the safe range passes with
// one comparison. Beyond it, an infinity is an error, and so is a whole
// number made from two whole numbers.
func finite(r, a, b float64) float64 {
	if math.Abs(r) > maxSafe {
		return finiteSlow(r, a, b)
	}
	return r
}

func finiteSlow(r, a, b float64) float64 {
	whole := func(f float64) bool { return f == math.Trunc(f) && math.Abs(f) <= maxSafe }
	if math.IsInf(r, 0) || r != r || (whole(a) && whole(b)) {
		panic(errRange)
	}
	return r
}

// The 64-bit overflow checks the first draft measured, kept for comparison.
func addWrap(a, b int64) int64 {
	c := a + b
	if (c > a) != (b > 0) {
		panic(errOverflow)
	}
	return c
}

func subWrap(a, b int64) int64 {
	c := a - b
	if (c < a) != (b > 0) {
		panic(errOverflow)
	}
	return c
}

func mulWrap(a, b int64) int64 {
	c := a * b
	if a != 0 && (c/a != b || (a == -1 && b == math.MinInt64)) {
		panic(errOverflow)
	}
	return c
}

func modInt(a, b int64) int64 {
	if b == 0 {
		panic(errType)
	}
	return a % b
}
