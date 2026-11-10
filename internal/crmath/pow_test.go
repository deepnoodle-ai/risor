package crmath

import (
	"math"
	"math/big"
	"math/bits"
	"math/rand/v2"
	"os"
	"strconv"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// powExactBig is powExact as it was with math/big, the oracle for the
// integer version.
func powExactBig(x, y float64) (float64, Status, bool) {
	r := x
	n := y
	for n != math.Trunc(n) {
		s := math.Sqrt(r)
		bs := new(big.Float).SetPrec(120).SetFloat64(s)
		bs.Mul(bs, bs)
		if bs.Cmp(new(big.Float).SetFloat64(r)) != 0 {
			return 0, OK, false
		}
		r, n = s, n*2
	}
	frac, e := math.Frexp(r)
	if frac == 0.5 {
		E := float64(e-1) * n
		switch {
		case E > 1023:
			return 0, Overflow, true
		case E <= -1075:
			return 0, OK, true
		}
		return math.Ldexp(1, int(E)), OK, true
	}
	if n < 0 || n > 64 {
		return 0, OK, false
	}
	M, E := oddMant(r)
	if (bits.Len64(M)-1)*int(n) > 54 {
		return 0, OK, false
	}
	P := new(big.Int).Exp(new(big.Int).SetUint64(M), big.NewInt(int64(n)), nil)
	v, _ := new(big.Float).SetPrec(128).SetMantExp(new(big.Float).SetPrec(128).SetInt(P), E*int(n)).Float64()
	if math.IsInf(v, 0) {
		return 0, Overflow, true
	}
	return v, OK, true
}

// TestPowExact compares powExact with powExactBig on edge arguments and on
// random ones built to have exact or nearly exact powers: short odd
// mantissas at every exponent to every power up to 64 (so results that are
// exact, midpoints, subnormal, zero or out of range), squares and fourth
// powers under fractional exponents, and random doubles. 200,000 random
// arguments, or RISOR_CRMATH_POW_N. Then it compares Pow on the edges with
// the slow path alone.
func TestPowExact(t *testing.T) {
	n := 200000
	if s := os.Getenv("RISOR_CRMATH_POW_N"); s != "" {
		v, err := strconv.Atoi(s)
		assert.Nil(t, err)
		n = v
	}
	if testing.Short() {
		n = 20000
	}
	edges := [][2]float64{{2, -1075}, {2, -1074}, {2, 1023}, {2, 1024}, {10, 22}, {10, 23}, {3, 3}, {1.5, 2},
		{0.1, 2}, {4, 0.5}, {8, 1.0 / 3}, {9, 1.5}, {16, 0.25}, {16, 0.75}, {0x1p-1074, 0.5}, {0x1p-1074, 1.5},
		{0x1.8p-1074, 2}, {3, 34}, {3, 35}, {5, 23}, {7, 19}, {7, 20}, {0x1.8p-540, 2}, {0x1.8p-600, 2},
		{0x1.8p+600, 2}, {0x1.fffffffffffffp+1023, 1}, {0x1.fffffffffffffp+511, 2}, {1e-200, 0.5}, {1e-320, 0.5},
		{6.25e-322, 0.5}, {0x1.21p-1060, 0.5}, {3, 64}, {3, 65}, {1.0000000000000002, 2}, {0.9999999999999999, 2}}
	for _, a := range edges {
		checkPowExact(t, a[0], a[1])
	}
	rng := rand.New(rand.NewPCG(13, 13))
	var found, over, zero, sub, odd54 int
	for range n {
		var x, y float64
		switch rng.IntN(5) {
		case 0: // a short odd mantissa at any exponent, to an integer power
			m := float64(rng.Uint64N(1<<uint(1+rng.IntN(27))) | 1)
			x = math.Ldexp(m, rng.IntN(2098)-1074-bits.Len64(uint64(m)))
			y = float64(rng.IntN(70))
		case 1: // a power whose odd part has exactly 54 bits: a midpoint
			k := 2 + rng.IntN(6)
			m := float64(rng.Uint64N(1<<uint(54/k)) | 1)
			x = math.Ldexp(m, rng.IntN(200)-100)
			y = float64(k)
		case 2: // a square or fourth power under a fractional exponent
			s := math.Ldexp(float64(rng.Uint64N(1<<26)|1), rng.IntN(1000)-500)
			x = s * s
			if rng.IntN(2) == 0 {
				x *= x
			}
			y = float64(rng.IntN(40)-20) / float64(int(1)<<uint(rng.IntN(3)))
		case 3: // a random double, an integer power
			x = math.Abs(anyBits(rng))
			y = float64(rng.IntN(130) - 65)
		default: // a random double, a random power
			x = math.Abs(anyBits(rng))
			y = (rng.Float64() - 0.5) * 8
		}
		if x == 0 || x == 1 || math.IsInf(x, 0) {
			continue
		}
		checkPowExact(t, x, y)
		if t.Failed() {
			return
		}
		if v, st, ok := powExact(x, y); ok {
			found++
			switch {
			case st == Overflow:
				over++
			case v == 0:
				zero++
			case v < 0x1p-1022:
				sub++
			}
			if r := float64(y); r == math.Trunc(r) && r > 0 && r <= 64 {
				if M, _ := oddMant(x); M > 1 && bits.Len64(M)*int(r) >= 54 {
					odd54++
				}
			}
		}
	}
	t.Logf("%d arguments: exact check decided %d (out of range %d, zero %d, subnormal %d, odd part of 54 or more bits %d)",
		n, found, over, zero, sub, odd54)
	for _, a := range append(edges, [2]float64{-3, 3}, [2]float64{-1.5, 2}, [2]float64{-2, -1075}, [2]float64{-0.1, 3}) {
		v, st := pow(a[0], a[1], true)
		w, sw := pow(a[0], a[1], false)
		assert.Equal(t, st, sw, "pow(%v, %v)", a[0], a[1])
		assert.Equal(t, math.Float64bits(v), math.Float64bits(w), "pow(%v, %v)", a[0], a[1])
		want := ref("pow", a[0], a[1])
		assert.Equal(t, st, want.st, "pow(%v, %v) against the reference", a[0], a[1])
		assert.Equal(t, v, want.v, "pow(%v, %v) against the reference", a[0], a[1])
	}
}

func checkPowExact(t *testing.T, x, y float64) {
	t.Helper()
	v, st, ok := powExact(x, y)
	w, sw, okw := powExactBig(x, y)
	if ok != okw || st != sw || math.Float64bits(v) != math.Float64bits(w) {
		t.Errorf("powExact(%x, %v) = %x %v %v, the big version %x %v %v", x, y, v, st, ok, w, sw, okw)
	}
}
