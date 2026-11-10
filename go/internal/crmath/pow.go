package crmath

import (
	"math"
	"math/big"
	"math/bits"
)

// Pow is x to the power y (f13-math.md 4.1).
func Pow(x, y float64) (float64, Status) {
	switch {
	case y == 0:
		return 1, OK
	case x == 0:
		if y > 0 {
			return 0, OK
		}
		return 0, NotReal
	}
	yInt := y == math.Trunc(y)
	if x < 0 && !yInt {
		return 0, NotReal
	}
	neg := false
	if x < 0 {
		x = -x
		// Every double of magnitude 2^53 or more is an even integer.
		neg = math.Abs(y) < 1<<53 && math.Mod(y, 2) != 0
	}
	if x == 1 {
		return sign(1, neg), OK
	}
	if v, st, ok := powExact(x, y); ok {
		if st != OK {
			return 0, st
		}
		return sign(v, neg), OK
	}
	v, st := ziv(func(w uint) *big.Float {
		p := w + 32
		t := nf(p).Mul(fl(y, p), logBig(fl(x, p+16), p+16))
		// Far outside the range of doubles the value is decided.
		if t.Cmp(ival(1000, p)) > 0 {
			return nf(w).SetMantExp(ival(1, w), 2000)
		}
		if t.Cmp(ival(-1000, p)) < 0 {
			return nf(w).SetMantExp(ival(1, w), -2000)
		}
		return expBig(t, w)
	})
	if st != OK {
		return 0, st
	}
	return sign(v, neg), OK
}

func sign(v float64, neg bool) float64 {
	if neg && v != 0 {
		return -v
	}
	return v
}

// powExact gives x^y for x > 0, x ≠ 1, where the exact value is a double or
// a midpoint between two doubles: x a power of two, an integer y with a
// small power, or y = n/2^k where x has an exact 2^k-th root. ok is false
// where the exact value is neither, and the value is transcendental or
// irrational.
func powExact(x, y float64) (float64, Status, bool) {
	// The 2^k-th root, for y = n / 2^k.
	r := x
	n := y
	for n != math.Trunc(n) {
		s := math.Sqrt(r)
		if s*s != r || !exactSquare(s, r) {
			return 0, OK, false
		}
		r, n = s, n*2
	}
	// Now x^y = r^n, n an integer (as a double).
	frac, e := math.Frexp(r) // r = frac 2^e, frac in [0.5, 1)
	if frac == 0.5 {
		// r = 2^(e-1): the value is 2^((e-1) n).
		E := float64(e-1) * n
		switch {
		case E > 1023:
			return 0, Overflow, true
		case E < -1075:
			return 0, OK, true
		case E == -1075:
			return 0, OK, true // half of the least double: the even zero
		}
		return math.Ldexp(1, int(E)), OK, true
	}
	if n < 0 || n > 64 {
		// The odd part of r is above 1: r^n for n > 64 has more than 54
		// significant bits, and for n < 0 is no dyadic number.
		return 0, OK, false
	}
	// r = M 2^E, M odd. r^n is exact or a midpoint only if M^n has at most
	// 54 significant bits.
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

// oddMant gives r = M 2^E with M odd.
func oddMant(r float64) (uint64, int) {
	frac, e := math.Frexp(r)
	M := uint64(math.Ldexp(frac, 53))
	E := e - 53
	z := bits.TrailingZeros64(M)
	return M >> z, E + z
}

// exactSquare reports whether s² is exactly r.
func exactSquare(s, r float64) bool {
	bs := new(big.Float).SetPrec(120).SetFloat64(s)
	bs.Mul(bs, bs)
	return bs.Cmp(new(big.Float).SetFloat64(r)) == 0
}
