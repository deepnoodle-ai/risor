package crmath

import (
	"math"
	"math/big"
	"math/bits"
)

// Pow is x to the power y (f13-math.md 4.1).
func Pow(x, y float64) (float64, Status) { return pow(x, y, true) }

func pow(x, y float64, fast bool) (float64, Status) {
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
	if fast {
		if v, st, ok := powFast(x, y); ok {
			if st != OK {
				return 0, st
			}
			return sign(v, neg), OK
		}
		Fallbacks.Add(1)
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
// irrational. It uses no big numbers: square roots are checked by FMA and
// integer powers are computed in 128-bit integers.
func powExact(x, y float64) (float64, Status, bool) {
	// The 2^k-th root, for y = n / 2^k.
	r := x
	n := y
	for n != math.Trunc(n) {
		s, ok := exactSqrt(r)
		if !ok {
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
	// 54 significant bits. Past this test, M ≥ 3 has L ≥ 2 bits and
	// (L−1)n ≤ 54, so n ≤ 54 and M^n < 2^(Ln) ≤ 2^(54+n) ≤ 2^108: M^n fits
	// in 128 bits, and the value M^n·2^(En) is rounded once, correctly.
	M, E := oddMant(r)
	if (bits.Len64(M)-1)*int(n) > 54 {
		return 0, OK, false
	}
	var hi, lo uint64 = 0, 1
	for range int(n) {
		h, l := bits.Mul64(lo, M)
		hi, lo = hi*M+h, l
	}
	v, st := roundU128(hi, lo, E*int(n))
	return v, st, true
}

// exactSqrt gives s = √r and true when s² = r exactly, for a positive
// finite r. math.Sqrt rounds correctly (IEEE 754), so a perfect square's
// root is exact; FMA gives s² − r with one rounding, which is 0 only when
// s² − r is 0, as long as a nonzero s² − r is not below the least
// subnormal. s² − r is a multiple of ulp(s)²/4 ≥ 2^-1004 for r ≥ 2^-900, and
// a smaller r is first scaled by 2^1000, exactly.
func exactSqrt(r float64) (float64, bool) {
	scale := 1.0
	if r < 0x1p-900 {
		r *= 0x1p1000
		scale = 0x1p-500
	}
	s := math.Sqrt(r)
	if math.FMA(s, s, -r) != 0 {
		return 0, false
	}
	return float64(s * scale), true // s·2^-500 ≥ 2^-500: exact
}

// roundU128 is the double nearest P·2^k, P = hi·2^64 + lo > 0, ties to even,
// subnormals in the grid; Overflow when that rounds to 2^1024 or more.
func roundU128(hi, lo uint64, k int) (float64, Status) {
	L := bits.Len64(lo)
	if hi != 0 {
		L = 64 + bits.Len64(hi)
	}
	top := L - 1 + k // the exponent of P's leading bit
	if top > 1023 {
		return 0, Overflow
	}
	kept := 53 // the bits of P the result keeps, at or above 2^-1074
	if top < -1022 {
		kept = top + 1075
	}
	if kept < 0 {
		return 0, OK // P·2^k < 2^(top+1) ≤ 2^-1076: below half of 2^-1074
	}
	shift := L - kept
	if shift <= 0 {
		// P has at most kept ≤ 53 bits: P·2^k is a double.
		return math.Ldexp(float64(lo), k), OK
	}
	m := shr128(hi, lo, shift) // < 2^kept ≤ 2^53
	round := shr128(hi, lo, shift-1)&1 == 1
	sticky := hi&lowMask(shift-1-64) != 0 || lo&lowMask(shift-1) != 0
	if round && (sticky || m&1 == 1) {
		m++
	}
	v := math.Ldexp(float64(m), k+shift) // m·2^(k+shift) is a double or overflows
	if math.IsInf(v, 0) {
		return 0, Overflow
	}
	return v, OK
}

// shr128 is the low 64 bits of (hi·2^64 + lo) >> n, 0 ≤ n < 128.
func shr128(hi, lo uint64, n int) uint64 {
	switch {
	case n == 0:
		return lo
	case n < 64:
		return lo>>uint(n) | hi<<uint(64-n)
	}
	return hi >> uint(n-64)
}

// lowMask is 2^n − 1 for n in [0, 64], 0 for n < 0, all ones above.
func lowMask(n int) uint64 {
	switch {
	case n <= 0:
		return 0
	case n >= 64:
		return math.MaxUint64
	}
	return 1<<uint(n) - 1
}

// oddMant gives r = M 2^E with M odd.
func oddMant(r float64) (uint64, int) {
	frac, e := math.Frexp(r)
	M := uint64(math.Ldexp(frac, 53))
	E := e - 53
	z := bits.TrailingZeros64(M)
	return M >> z, E + z
}
