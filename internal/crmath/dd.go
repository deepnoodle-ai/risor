package crmath

import "math"

// Double-double arithmetic for the fast phase.
//
// A pair (h, l) stands for the exact sum h + l. A pair is normalized when
// |l| ≤ ulp(h)/2; every function here returns a normalized pair, and the
// bounds below assume normalized arguments and no underflow or overflow.
// The bounds are those proven by Joldes, Muller and Popescu, "Tight and
// rigorous error bounds for basic building blocks of double-word
// arithmetic", ACM TOMS 44(2), 2017, with u = 2^-53; the algorithm numbers
// are theirs. Each function's error is relative to its exact result.
//
// Go may fuse x*y + z into one rounding. Every product that feeds a sum is
// written float64(x*y), which the specification of Go makes round, or
// math.FMA where one rounding is wanted. The compiler decides nothing.

// twoSum gives s + e = a + b exactly, s = fl(a + b) (Knuth).
func twoSum(a, b float64) (float64, float64) {
	s := a + b
	bb := s - a
	return s, (a - (s - bb)) + (b - bb)
}

// fastTwoSum gives s + e = a + b exactly when a = 0 or |a| ≥ |b| (Dekker).
func fastTwoSum(a, b float64) (float64, float64) {
	s := a + b
	return s, b - (s - a)
}

// twoProd gives p + e = a·b exactly, p = fl(a·b), when a·b does not
// underflow.
func twoProd(a, b float64) (float64, float64) {
	p := float64(a * b)
	return p, math.FMA(a, b, -p)
}

// ddAdd is AccurateDWPlusDW (Alg. 6): relative error ≤ 3u² + 13u³ < 2^-104,
// whatever the signs.
func ddAdd(xh, xl, yh, yl float64) (float64, float64) {
	sh, sl := twoSum(xh, yh)
	th, tl := twoSum(xl, yl)
	sl += th
	sh, sl = fastTwoSum(sh, sl)
	sl += tl
	return fastTwoSum(sh, sl)
}

// ddAddD is DWPlusFP (Alg. 4): relative error ≤ 2u² = 2^-105.
func ddAddD(xh, xl, y float64) (float64, float64) {
	sh, sl := twoSum(xh, y)
	sl += xl
	return fastTwoSum(sh, sl)
}

// ddAddFast adds pairs a and b with |b| ≤ 3|a|/4, as where a Horner step
// adds a coefficient to a much smaller product: SloppyDWPlusDW (Alg. 5),
// whose error the condition bounds. a_h + b_h is exact as a pair (|b_h| ≤
// |a_h|); the two roundings of e + (a_l + b_l) err by ≤ u·2u|a| + u(u|s| +
// 2u|a|) ≤ 5.75u²|a| with |s| ≤ 1.75|a|(1+u); and |a + b| ≥ |a|/4. So the
// relative error is ≤ 23u² < 2^-101.
func ddAddFast(ah, al, bh, bl float64) (float64, float64) {
	s, e := fastTwoSum(ah, bh)
	e += al + bl
	return fastTwoSum(s, e)
}

// ddAddDFast adds a double y to a pair x with |x| ≤ 3|y|/4: as ddAddFast
// with a = (y, 0), relative error < 2^-101.
func ddAddDFast(xh, xl, y float64) (float64, float64) {
	s, e := fastTwoSum(y, xh)
	e += xl
	return fastTwoSum(s, e)
}

// ddMul is DWTimesDW3 (Alg. 12): relative error ≤ 4u² = 2^-104.
func ddMul(xh, xl, yh, yl float64) (float64, float64) {
	ch, cl1 := twoProd(xh, yh)
	tl0 := float64(xl * yl)
	tl1 := math.FMA(xh, yl, tl0)
	cl2 := math.FMA(xl, yh, tl1)
	return fastTwoSum(ch, cl1+cl2)
}

// ddMulD is DWTimesFP3 (Alg. 9): relative error ≤ 2u² = 2^-105.
func ddMulD(xh, xl, y float64) (float64, float64) {
	ch, cl1 := twoProd(xh, y)
	return fastTwoSum(ch, math.FMA(xl, y, cl1))
}

// ddDiv is DWDivDW2 (Alg. 17), with Alg. 9 for its product: relative error
// ≤ 15u² + 56u³ < 2^-102.
func ddDiv(xh, xl, yh, yl float64) (float64, float64) {
	th := xh / yh
	rh, rl := ddMulD(yh, yl, th)
	ph := xh - rh
	dl := xl - rl
	d := ph + dl
	return fastTwoSum(th, d/yh)
}

// ddSqrt is the square root of a pair a > 0 by one Newton step from
// s = √ah, which IEEE 754 rounds correctly: √a = s + (a − s²)/(√a + s).
// ah − s² is exact by FMA. Taking 2s for √a + s errs by a factor
// 1 + O(u), on a correction of size ≤ u·s, so by ≤ u²·s; the two roundings
// of the correction add ≤ 2u·(u·s). The relative error is < 4u² = 2^-104.
func ddSqrt(ah, al float64) (float64, float64) {
	if ah == 0 {
		return 0, 0
	}
	s := math.Sqrt(ah)
	r := math.FMA(-s, s, ah) + al
	return fastTwoSum(s, r/float64(2*s))
}

// pow2 is 2^n for −1022 ≤ n ≤ 1023, exactly.
func pow2(n int) float64 { return math.Float64frombits(uint64(n+1023) << 52) }

// roundDD rounds the result of a fast phase. v = (hi + lo)·2^e, a
// normalized pair, approximates the exact value V with |V − v| ≤ eps·|v|.
// When every value in [v − eps|v|, v + eps|v|] rounds to the same double
// (nearest, ties to even, subnormals in the grid) that double is V's
// correctly rounded value, and ok is true; else ok is false and the caller
// falls back to the slow path.
//
// The test computes the ends of the interval in floating point. Their
// roundings (of eps·|hi|, lo ± d, and the 1 + 2^-52 between |hi| and |v|)
// move each end inward by at most 2^-103·|v|, and every caller passes an
// eps that exceeds its proven bound by far more than that (2^-72 against
// bounds below 2^-81), so the computed interval contains the true one.
// Rounding is monotonic, so when both computed ends round to one double,
// every value between them does.
func roundDD(hi, lo float64, e int, eps float64) (float64, Status, bool) {
	if hi == 0 || math.IsNaN(hi) || math.IsInf(hi, 0) || math.IsNaN(lo) {
		return 0, OK, false
	}
	neg := hi < 0
	if neg {
		hi, lo = -hi, -lo
	}
	_, he := math.Frexp(hi) // hi in [2^(he−1), 2^he)
	E := he + e             // v < 2^E·(1 + 2^-52)
	switch {
	case E < -1100:
		// v·(1 + eps) < 2^-1099, below half the least subnormal.
		return 0, OK, true
	case E >= -1020:
		// v > 2^-1022: the result is normal or overflows. Scaling the
		// rounded value by 2^e is exact unless it overflows; a value that
		// rounds to 2^1024 or more is out of range (f13 2.1 item 2).
		d := float64(eps * hi)
		l := hi + (lo - d)
		r := hi + (lo + d)
		if l != r {
			return 0, OK, false
		}
		v := math.Ldexp(l, e)
		if math.IsInf(v, 0) {
			return 0, Overflow, true
		}
		return sign(v, neg), OK, true
	}
	// v < 2^-1021·(1 + 2^-52): the doubles up to 2^-1021 are the multiples
	// of 2^-1074, so the result is the integer nearest u = v·2^1074 ≤ 2^53,
	// times 2^-1074. uh is exact (it is at least 2^-27); ul may lose bits
	// below 2^-1074, far inside eps·u.
	s := e + 1074
	uh, ul := math.Ldexp(hi, s), math.Ldexp(lo, s)
	n := math.Floor(uh)
	g := (uh - n) + ul // in [−1/2, 3/2], with error < 2^-51
	d := float64(eps * uh)
	gl, gr := g-d, g+d
	const m = 0x1p-40 // covers the roundings of g, d, gl and gr
	switch {
	case gl > -0.5+m && gr < 0.5-m:
	case gl > 0.5+m && gr < 1.5-m:
		n++
	default:
		return 0, OK, false
	}
	return sign(math.Ldexp(n, -1074), neg), OK, true
}
