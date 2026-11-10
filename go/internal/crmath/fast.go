package crmath

import (
	"math"
	"math/bits"
)

// The fast phase (CORE-MATH and CRlibm use the same scheme). Each entry
// first evaluates its function in double-double arithmetic (dd.go) with a
// proven relative error bound, and rounds with roundDD when the whole error
// interval rounds to one double, which is nearly every call. Else the entry
// runs its slow path, the big-number evaluation, which is exact in the
// limit and decides every case.
//
// Error accounting. u = 2^-53. A table entry or constant is within 2^-106
// relative of its value (tables.go). Each double-double operation adds at
// most 2^-101 relative (dd.go); the analyses below round every such bound
// up. Every kernel's total bound is below 2^-90 (2^-81 for pow), and the
// bound passed to the rounding test is 2^-72 (2^-70 for pow), at least 2^11
// times the proven bound, so a slip of a few bits in an analysis costs
// nothing in correctness. The rounding test fails, and the slow path runs,
// for a few calls in a million on random arguments.
//
// Arguments for which the result is decided without a series (a tiny x
// for sine and the like) are decided here with the proof given in place.

// epsFast is the bound the rounding test uses for every kernel whose proven
// bound is below 2^-90.
const epsFast = 0x1p-72

// --- exp -----------------------------------------------------------------

// expDD gives e^(th + tl) as (hi + lo)·2^m, hi + lo in [0.99, 2), for a
// normalized pair with |th| ≤ 1000.
//
// Method. k = round(t·4096/ln 2), r = t − k·ln2/4096, |r| ≤ ln2/8192 +
// 2^-42 < 2^-13.4. e^t = 2^(k/4096)·e^r, and k = 4096m + 64i + j gives
// 2^(k/4096) = 2^m·2^(i/64)·2^(j/4096), the last two from tables.
//
// Reduction error. k·(ln2/4096)_h is exact as a pair (twoProd, |k| < 2^23)
// and t_h minus it exact as a pair (twoSum); the pair ln2/4096 is off by
// ≤ 2^-106·2^-12.5 and |k| < 2^23, so ≤ 2^-95.5. The four double
// operations that gather the small parts (each of magnitude ≤ 2^-43) err by
// ≤ 4·2^-53·2^-41 = 2^-92. So r is within 2^-91.5 of t − k·ln2/4096, and
// e^r within a relative 2^-91.4 of its value.
//
// Polynomial. e^r = 1 + r(1 + r(1/2 + r(1/6 + r(1/24 + r(1/120 +
// r/720))))) + R, |R| ≤ |r|^7/5040·e^|r| < 2^-106. The innermost three
// levels are in doubles: their value is ≈ 1/24, their rounding error and
// that of the double 1/24, 1/120, 1/720 is < 2^-50·2^-4.5, and is scaled by
// r³ < 2^-40, so < 2^-94. The four outer levels are pairs, each a product
// and an addition of a coefficient at least 2^12 times the product
// (ddAddFast): < 4·(2^-104 + 2^-101) < 2^-98.8. Two table products:
// < 2·2^-104 + 2·2^-106.
//
// Total: < 2^-91.4 + 2^-94 + 2^-98.8 + 2^-102 < 2^-91.
func expDD(th, tl float64) (hi, lo float64, m int) {
	T := tab()
	const invL = 4096 / math.Ln2 // any double near it: k only selects
	k := math.RoundToEven(float64(th * invL))
	ph, pe := twoProd(k, T.ln2by4096[0])
	s, se := twoSum(th, -ph)
	rest := ((se - pe) + tl) - float64(k*T.ln2by4096[1])
	rh, rl := twoSum(s, rest)

	y := math.FMA(rh, math.FMA(rh, 1.0/720, 1.0/120), 1.0/24)
	yh, yl := ddMulD(rh, rl, y)
	yh, yl = ddAddFast(T.c6[0], T.c6[1], yh, yl)
	yh, yl = ddMul(rh, rl, yh, yl)
	yh, yl = ddAddDFast(yh, yl, 0.5)
	yh, yl = ddMul(rh, rl, yh, yl)
	yh, yl = ddAddDFast(yh, yl, 1)
	yh, yl = ddMul(rh, rl, yh, yl)
	yh, yl = ddAddDFast(yh, yl, 1)

	ki := int64(k)
	i, j := (ki>>6)&63, ki&63
	a, b := T.exp64[i], T.exp4096[j]
	ph, pl := ddMul(a[0], a[1], b[0], b[1])
	hi, lo = ddMul(ph, pl, yh, yl)
	return hi, lo, int(ki >> 12)
}

func expFast(x float64) (float64, Status, bool) {
	if !(math.Abs(x) <= 1000) {
		return 0, OK, false
	}
	if math.Abs(x) < 0x1p-54 {
		// |e^x − 1| < 2^-54·(1 + 2^-53): for x > 0 below half the gap
		// above 1 (2^-53), for x < 0 below half the gap below 1 (2^-54),
		// and never equal to it. So e^x rounds to 1.
		return 1, OK, true
	}
	hi, lo, m := expDD(x, 0)
	return roundDD(hi, lo, m, epsFast)
}

// --- log -----------------------------------------------------------------

// log1pDD is log(1 + z) for a normalized pair z, |z| < 2^-9.
//
// log(1 + z) = z·Σ_{k=0}^{9} c_k z^k + R, c_k = (−1)^k/(k + 1), |R| ≤
// |z|^11/11/(1 − |z|); relative to |log(1 + z)| ≥ |z|(1 − |z|/2) that is
// < 2^-93.4. Horner's rule, from the inside:
//
//   - k = 5 … 9 in doubles on z_h: value ≈ −1/6; the four FMAs and the
//     doubles' coefficients err by < 2^-53, scaled by |z|^5 < 2^-45:
//     < 2^-98.
//   - k = 4: z_h times that, by twoProd, plus the pair 1/5. Dropping z_l
//     (|z_l| ≤ 2^-63) here errs by < 2^-63·2^-2.5·|z|^4 < 2^-101.
//   - k = 3 … 0: a pair product and a pair or double coefficient. Each
//     product is < 2^-8 of its coefficient, so the additions are ddAddFast,
//     and an error at level k reaches the result scaled by |z|^k ≤ 1.
//     Four levels and the final product: < 5·(2^-104 + 2^-101) < 2^-98.5.
//
// Total relative error < 2^-93.4 + 2^-98 + 2^-98.5 + 2^-101 < 2^-93.
func log1pDD(zh, zl float64) (float64, float64) {
	T := tab()
	t := math.FMA(zh, -1.0/10, 1.0/9)
	t = math.FMA(zh, t, -1.0/8)
	t = math.FMA(zh, t, 1.0/7)
	t = math.FMA(zh, t, -1.0/6)
	h, l := twoProd(zh, t)
	h, l = ddAddFast(T.l1p[4][0], T.l1p[4][1], h, l)
	h, l = ddMul(zh, zl, h, l)
	h, l = ddAddDFast(h, l, -0.25)
	h, l = ddMul(zh, zl, h, l)
	h, l = ddAddFast(T.l1p[2][0], T.l1p[2][1], h, l)
	h, l = ddMul(zh, zl, h, l)
	h, l = ddAddDFast(h, l, -0.5)
	h, l = ddMul(zh, zl, h, l)
	h, l = ddAddDFast(h, l, 1)
	return ddMul(zh, zl, h, l)
}

// logDD is the natural logarithm of a positive finite x, as a pair with
// relative error < 2^-91.
//
// For x in [1 − 2^-10, 1 + 2^-9), log x = log1p(x − 1), x − 1 exact
// (Sterbenz): relative error < 2^-93.
//
// Else x = 2^e·m, m in [1, 2) (a subnormal x is first scaled by 2^52), and
// i = the top 9 bits of m's fraction. log x = e·ln 2 − log r_i + log1p(z),
// z = m·r_i − 1, computed exactly as a pair: twoProd, then p_h − 1 exact
// (Sterbenz), then twoSum. For i > 0, r_i = fl(1/mid_i) and m is within
// 2^-10 of mid_i, so |z| < 2^-10/mid_i + 2^-52 < 2^-10; for i = 0, r_0 = 1
// and z = m − 1 < 2^-9.
//
// Absolute errors: log1p < 2^-93·2^-9; −log r_i < 2^-106·ln 2; their sum
// (ddAdd) < 2^-104·ln 2; e·ln 2 < (2^-105 + 2^-106)|e| ln 2; together
// < 2^-101.7 + 2^-104.9·|e|. The last sum adds 2^-104 relative.
// Cancellation is bounded: outside the first branch |log x| ≥
// log(1 + 2^-9) > 2^-9.01 when e = 0, > 2^-10 when e = −1, and ≥ |e| ln 2/2
// otherwise. So the relative error is < 2^-91.
func logDD(x float64) (float64, float64) {
	if x >= 1-0x1p-10 && x < 1+0x1p-9 {
		return log1pDD(x-1, 0)
	}
	T := tab()
	e := 0
	if x < 0x1p-1022 {
		x *= 0x1p52
		e = -52
	}
	b := math.Float64bits(x)
	e += int(b>>52) - 1023
	m := math.Float64frombits(b&(1<<52-1) | 1023<<52)
	i := (b >> 43) & 511
	ph, pl := twoProd(m, T.logR[i])
	zh, zl := twoSum(ph-1, pl)
	h, l := log1pDD(zh, zl)
	if i != 0 {
		h, l = ddAdd(T.logRL[i][0], T.logRL[i][1], h, l)
	}
	if e == 0 {
		return h, l
	}
	eh, el := ddMulD(T.ln2[0], T.ln2[1], float64(e))
	return ddAdd(eh, el, h, l)
}

func logFast(x float64) (float64, Status, bool) {
	if !(x > 0 && x <= math.MaxFloat64) {
		return 0, OK, false
	}
	h, l := logDD(x)
	return roundDD(h, l, 0, epsFast)
}

// log10Fast: log x · (1/ln 10), one more product and a constant: relative
// error < 2^-91 + 2^-102 + 2^-106 < 2^-90.9.
func log10Fast(x float64) (float64, Status, bool) {
	if !(x > 0 && x <= math.MaxFloat64) {
		return 0, OK, false
	}
	T := tab()
	h, l := logDD(x)
	h, l = ddMul(h, l, T.ilog10[0], T.ilog10[1])
	return roundDD(h, l, 0, epsFast)
}

// --- sin, cos, tan -------------------------------------------------------

// reduce gives r = (rh, rl) and q with a = (4N + q)·π/2 + r·(1 + δ) for some
// integer N, |r| ≤ π/4·(1 + 2^-50), and |δ| ≤ dr, for a finite a ≥ 0.
//
// Below 0.785 < π/4, r = a, q = 0, δ = 0.
//
// Else the reduction is Payne and Hanek's, in integers. a = M·2^E, M < 2^53
// an integer. a·2/π = M·Σ b_i 2^(E−i) over the bits b_i of 2/π. A term with
// E − i ≥ 2 is a multiple of 4 and is dropped; so are the terms past a
// window of 192 bits from s = max(1, E − 1), which add less than
// M·2^(E−s−191) ≤ 2^-137. The window times M is P, a 245-bit integer, and
// y = P/2^pos, pos = s + 191 − E in [190, 245], is a·2/π mod 4 less some ε in
// [0, 2^-137). (The table of 2/π is exact to 2^-1280; with a < 2^1024 its
// error adds < 2^-255.) q is the integer part of y, plus one when the
// fraction is 1/2 or more, and then f = frac − 1, so |f| ≤ 1/2.
//
// f is taken from its top 106 bits (an error < 2^-105 of |f|) into a pair,
// and r = f·π/2 by a pair product (π/2 within 2^-106, product within
// 2^-104). So |δ| < 2^-137/|f| + 2^-103.2; dr is set to 2^-135/|f_h| +
// 2^-100. f is never 0: no double is a multiple of π/2 but 0. The closest
// approach of a double to such a multiple is about 2^-61 (Muller, Elementary
// Functions, 11.3), so dr < 2^-71.5 always, but nothing here relies on it:
// the rounding test is given dr, and a dr above 2^-60 sends the call to the
// slow path.
func reduce(a float64) (rh, rl float64, q uint64, dr float64) {
	if a < 0.785 {
		return a, 0, 0, 0
	}
	T := tab()
	b := math.Float64bits(a)
	M := b&(1<<52-1) | 1<<52
	E := int(b>>52) - 1075
	s := 1
	if E-1 > 1 {
		s = E - 1
	}
	w, sh := (s-1)/64, uint(s-1)%64
	tp := &T.twoOverPi
	c2 := tp[w]<<sh | tp[w+1]>>(64-sh)
	c1 := tp[w+1]<<sh | tp[w+2]>>(64-sh)
	c0 := tp[w+2]<<sh | tp[w+3]>>(64-sh)
	h0, p0 := bits.Mul64(M, c0)
	h1, l1 := bits.Mul64(M, c1)
	h2, l2 := bits.Mul64(M, c2)
	p1, c := bits.Add64(l1, h0, 0)
	p2, c := bits.Add64(l2, h1, c)
	p3 := h2 + c
	P := [4]uint64{p0, p1, p2, p3}
	pos := s + 191 - E // in [190, 245]

	q = wordAt(&P, pos) & 3
	// F = P mod 2^pos.
	keepLow(&P, pos)
	neg := wordAt(&P, pos-1)&1 == 1
	if neg {
		// G = 2^pos − F, the magnitude of f = F/2^pos − 1.
		var br uint64
		for k := range 4 {
			P[k], br = bits.Sub64(0, P[k], br)
		}
		keepLow(&P, pos)
		q++
	}
	L := 0
	for k := 3; k >= 0; k-- {
		if P[k] != 0 {
			L = 64*k + bits.Len64(P[k])
			break
		}
	}
	if L == 0 {
		return 0, 0, 0, math.Inf(1) // not reached
	}
	const m53 = 1<<53 - 1
	fh := float64(float64(wordAt(&P, L-53)&m53) * pow2(L-53-pos))
	fl := float64(float64(wordAt(&P, L-106)&m53) * pow2(L-106-pos))
	fh, fl = fastTwoSum(fh, fl)
	if neg {
		fh, fl = -fh, -fl
	}
	rh, rl = ddMul(fh, fl, T.halfPi[0], T.halfPi[1])
	dr = 0x1p-135/math.Abs(fh) + 0x1p-100
	return rh, rl, q & 3, dr
}

// keepLow sets P to P mod 2^n, 0 < n < 256.
func keepLow(P *[4]uint64, n int) {
	k := n / 64
	P[k] &= 1<<uint(n%64) - 1
	for k++; k < 4; k++ {
		P[k] = 0
	}
}

// wordAt is bits n … n+63 of the 256-bit integer P, zeros past its ends.
func wordAt(P *[4]uint64, n int) uint64 {
	if n < 0 {
		return P[0] << uint(-n)
	}
	k, b := n/64, uint(n%64)
	if k >= 4 {
		return 0
	}
	v := P[k] >> b
	if b > 0 && k+1 < 4 {
		v |= P[k+1] << (64 - b)
	}
	return v
}

// sinCosDD gives sin r and cos r for a normalized pair |r| ≤ π/4·(1 +
// 2^-50), each with relative error < 2^-92.
//
// Method. j = round(64 r_h), t = r − j/64 (r_h − j/64 exact by Sterbenz,
// then twoSum), |t| ≤ 2^-7 + 2^-53. With S = sin(j/64), C = cos(j/64)
// from tables, sin r = S + (S·c + C·s) and cos r = C + (C·c − S·s),
// s = sin t, c = cos t − 1.
//
// sin t = t(1 + v(−1/6 + v(1/120 + v(−1/5040 + v/362880)))) + R, v = t²,
// |R| ≤ |t|^11/11! < 2^-95.3·|t|. The two inner levels are doubles: value
// ≈ 1/5040, error < 2^-64.5, scaled by v³ < 2^-41.9: < 2^-106. Pairs
// elsewhere, four products and three additions of a coefficient far larger
// than the product (ddAddFast): relative error of s < 2^-95.3 + 2^-99
// < 2^-95.
//
// cos t − 1 = v(−1/2 + v(1/24 + v(−1/720 + v(1/40320 − v/3628800)))) + R,
// |R| ≤ t^12/12! < 2^-112. Three inner levels in doubles: error < 2^-62
// scaled by v³: < 2^-104. Pairs elsewhere, as for s: c has relative error
// < 2^-99 and absolute error < 2^-103, |c| < 2^-15.
//
// The sums: for j = 0 (S = 0, C = 1) sin r = s, cos r = 1 + c, with
// relative errors < 2^-95 and < 2^-100. For j ≥ 1, |sin r| ≥ sin(2^-7)
// > 2^-7.1 and cos r > 0.7; the absolute errors of S·c, C·s and the three
// pair operations sum to < 2^-99, which is < 2^-92 of the result.
func sinCosDD(rh, rl float64) (sh, sl, ch, cl float64) {
	T := tab()
	j := math.RoundToEven(float64(rh * 64))
	th, tl := twoSum(rh-float64(j*(1.0/64)), rl)
	vh, vl := ddMul(th, tl, th, tl)

	a := math.FMA(vh, 1.0/362880, -1.0/5040)
	ph, pl := ddMulD(vh, vl, a)
	ph, pl = ddAddFast(T.s120[0], T.s120[1], ph, pl)
	ph, pl = ddMul(vh, vl, ph, pl)
	ph, pl = ddAddFast(T.s6[0], T.s6[1], ph, pl)
	ph, pl = ddMul(vh, vl, ph, pl)
	ph, pl = ddAddDFast(ph, pl, 1)
	sth, stl := ddMul(th, tl, ph, pl)

	bq := math.FMA(vh, math.FMA(vh, -1.0/3628800, 1.0/40320), -1.0/720)
	qh, ql := ddMulD(vh, vl, bq)
	qh, ql = ddAddFast(T.c24[0], T.c24[1], qh, ql)
	qh, ql = ddMul(vh, vl, qh, ql)
	qh, ql = ddAddDFast(qh, ql, -0.5)
	cmh, cml := ddMul(vh, vl, qh, ql)

	if j == 0 {
		ch, cl = ddAddDFast(cmh, cml, 1)
		return sth, stl, ch, cl
	}
	ji := int(j)
	S, C := T.sin64[abs(ji)], T.cos64[abs(ji)]
	if ji < 0 {
		S[0], S[1] = -S[0], -S[1]
	}
	ah, al := ddMul(S[0], S[1], cmh, cml)
	bh, bl := ddMul(C[0], C[1], sth, stl)
	xh, xl := ddAdd(ah, al, bh, bl)
	sh, sl = ddAdd(S[0], S[1], xh, xl)
	ah, al = ddMul(C[0], C[1], cmh, cml)
	bh, bl = ddMul(S[0], S[1], sth, stl)
	xh, xl = ddAdd(ah, al, -bh, -bl)
	ch, cl = ddAdd(C[0], C[1], xh, xl)
	return sh, sl, ch, cl
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

// Propagation of the reduction's error δ: sin(r(1+δ)) = sin r·(1 + δ'),
// |δ'| ≤ |δ|·r/tan r ≤ |δ|; for cos, |δ'| ≤ |δ|·r tan r ≤ |δ|; for tan and
// cot, |δ'| ≤ |δ|·2r/sin 2r ≤ (π/2)|δ|, to first order; with |δ| ≤ dr ≤
// 2^-60 the second-order terms are below 2^-118. So sin, cos and tan of x
// are within 2^-90.9 + 1.6·dr, and the bound passed is epsFast + 4·dr.
const maxDr = 0x1p-60

// tiny is the bound below which sin, tan, atan and asin round to x and cos
// to 1. For |x| < 2^-28 (x's exponent ex ≤ −29), each of |sin x − x|,
// |tan x − x|, |atan x − x|, |asin x − x| is below |x|³/3·(1 + x²) <
// 2^(3ex+3)/3·(1 + 2^-56), and half the gap between x and its neighbor
// toward the value is at least 2^(ex−54); 2^(3ex+3)/3·(1+2^-56) <
// 2^(ex−54) as 2^(2ex+57) ≤ 2^-1 < 3. For a subnormal x the gap is 2^-1074
// and the difference far smaller. The difference is never zero (x ≠ 0)
// and never half the gap, so the value rounds to x. 1 − cos x < x²/2 <
// 2^-57, below half the gap below 1 (2^-54), so cos x rounds to 1.
const tiny = 0x1p-28

func sinFast(x float64) (float64, Status, bool) {
	a := math.Abs(x)
	if !(a <= math.MaxFloat64) {
		return 0, OK, false
	}
	if a < tiny {
		return x, OK, true
	}
	rh, rl, q, dr := reduce(a)
	if dr > maxDr {
		return 0, OK, false
	}
	sh, sl, ch, cl := sinCosDD(rh, rl)
	h, l := sh, sl
	switch q {
	case 1:
		h, l = ch, cl
	case 2:
		h, l = -sh, -sl
	case 3:
		h, l = -ch, -cl
	}
	if x < 0 {
		h, l = -h, -l
	}
	return roundDD(h, l, 0, epsFast+float64(4*dr))
}

func cosFast(x float64) (float64, Status, bool) {
	a := math.Abs(x)
	if !(a <= math.MaxFloat64) {
		return 0, OK, false
	}
	if a < tiny {
		return 1, OK, true
	}
	rh, rl, q, dr := reduce(a)
	if dr > maxDr {
		return 0, OK, false
	}
	sh, sl, ch, cl := sinCosDD(rh, rl)
	h, l := ch, cl
	switch q {
	case 1:
		h, l = -sh, -sl
	case 2:
		h, l = -ch, -cl
	case 3:
		h, l = sh, sl
	}
	return roundDD(h, l, 0, epsFast+float64(4*dr))
}

// tanFast: tan r = sin r/cos r for even q, −cos r/sin r for odd q, one
// division (< 2^-102) on two values within 2^-92: < 2^-90.9.
func tanFast(x float64) (float64, Status, bool) {
	a := math.Abs(x)
	if !(a <= math.MaxFloat64) {
		return 0, OK, false
	}
	if a < tiny {
		return x, OK, true
	}
	rh, rl, q, dr := reduce(a)
	if dr > maxDr {
		return 0, OK, false
	}
	sh, sl, ch, cl := sinCosDD(rh, rl)
	var h, l float64
	if q&1 == 0 {
		h, l = ddDiv(sh, sl, ch, cl)
	} else {
		h, l = ddDiv(-ch, -cl, sh, sl)
	}
	if x < 0 {
		h, l = -h, -l
	}
	return roundDD(h, l, 0, epsFast+float64(4*dr))
}

// --- atan, atan2, asin, acos ---------------------------------------------

// atanDD is atan z for a normalized pair 0 ≤ z ≤ 1 + 2^-8, with relative
// error < 2^-94 + δz, δz the relative error of z.
//
// Method. j = round(64 z_h), c = j/64, u = (z − c)/(1 + zc), so atan z =
// atan c + atan u, atan c from a table. z − c is exact (z_h − c exact by
// Sterbenz, then twoSum); 1 + zc is a pair within 2^-104 (twoProd for
// z_h·c, a double z_l·c within 2^-106 of a term ≤ 2^-53, a pair sum); the
// division adds 2^-102. |u| ≤ 2^-7(1 + 2^-45).
//
// atan u = u(1 − v/3 + v²/5 − v³/7 + v⁴/9 − v⁵/11 + v⁶/13) + R, v = u²,
// |R| ≤ |u|^15/15 < 2^-101.9·|u|. Levels from v³ on are doubles: value ≈
// 1/7, error < 2^-53, scaled by v³ < 2^-42: < 2^-95. Pairs elsewhere,
// five products and three additions of a coefficient far larger than the
// product (ddAddFast): < 2^-98.5. So atan u is within 2^-94.5 of its value.
//
// For j = 0, atan z = atan u, u = z. For j ≥ 1, atan z ≥ atan(2^-7) >
// 2^-7.1 and the absolute errors (atan u < 2^-101.5, the table < 2^-106,
// the sum, by ddAddFast as |atan u| ≤ atan(2^-7) < 3/4 atan(1/64), < 2^-101
// of it) are < 2^-94 of it.
func atanDD(zh, zl float64) (float64, float64) {
	T := tab()
	j := math.RoundToEven(float64(zh * 64))
	c := float64(j * (1.0 / 64))
	nh, nl := twoSum(zh-c, zl)
	uh, ul := nh, nl
	if j != 0 {
		ph, pl := twoProd(zh, c)
		pl += float64(zl * c)
		dh, dl := ddAddD(ph, pl, 1)
		uh, ul = ddDiv(nh, nl, dh, dl)
	}
	vh, vl := ddMul(uh, ul, uh, ul)
	t := math.FMA(vh, 1.0/13, -1.0/11)
	t = math.FMA(vh, t, 1.0/9)
	t = math.FMA(vh, t, -1.0/7)
	h, l := ddMulD(vh, vl, t)
	h, l = ddAddFast(T.a5[0], T.a5[1], h, l)
	h, l = ddMul(vh, vl, h, l)
	h, l = ddAddFast(T.a3[0], T.a3[1], h, l)
	h, l = ddMul(vh, vl, h, l)
	h, l = ddAddDFast(h, l, 1)
	h, l = ddMul(uh, ul, h, l)
	if j == 0 {
		return h, l
	}
	A := T.atan64[int(j)]
	return ddAddFast(A[0], A[1], h, l)
}

// atanFast. For |x| ≤ 1, atan |x| = atanDD(|x|). For |x| > 1, atan |x| =
// π/2 − atanDD(z), z = 1/|x| as a pair: z_h = fl(1/|x|), z_l from the exact
// remainder 1 − z_h|x| (FMA), within 2^-104; for |x| ≥ 2^1022, z is
// subnormal and its error is below 2^-1074, nothing beside π/2. The result
// is ≥ π/4 and the subtraction of a value ≤ π/4 from π/2 at most doubles
// relative errors: < 2^-92.
func atanFast(x float64) (float64, Status, bool) {
	a := math.Abs(x)
	if !(a <= math.MaxFloat64) {
		return 0, OK, false
	}
	if a < tiny {
		return x, OK, true
	}
	var h, l float64
	if a <= 1 {
		h, l = atanDD(a, 0)
	} else {
		T := tab()
		zh := 1 / a
		zl := math.FMA(-zh, a, 1) / a
		zh, zl = fastTwoSum(zh, zl)
		h, l = atanDD(zh, zl)
		h, l = ddAdd(T.halfPi[0], T.halfPi[1], -h, -l)
	}
	if x < 0 {
		h, l = -h, -l
	}
	return roundDD(h, l, 0, epsFast)
}

// atan2Fast is atan2(y, x) for finite x and y, not both 0. On an axis the
// angle is 0, π or ±π/2.
//
// a = |y|, b = |x|, scaled exactly to mantissas in [1/2, 1): a = m_a 2^ea,
// b = m_b 2^eb. With n/d the smaller over the larger, the ratio is
// z = q·2^dd, q = n/d in (1/2, 2) as a pair (the remainder by FMA is exact,
// all normal), within 2^-104, and θ = atan z ∈ [0, π/4].
//
// For dd ≥ −40, z is scaled exactly and θ = atanDD(z): < 2^-93.9. For
// dd < −40, z < 2^-39 and θ = z − z³/3 + R, |R| < z^5/5 < 2^-195·z; the
// cube is a double within 2^-51 of a term < 2^-78·z: θ is kept as
// (q − q³·2^(2dd)/3)·2^dd, within 2^-102 of its value, and may lie in the
// subnormal range, which roundDD handles.
//
// The angle is θ (|y| ≤ |x|, x > 0), π − θ (x < 0), π/2 − θ (|y| > |x|,
// x > 0) or π/2 + θ (x < 0), with the sign of y. The last three are ≥ π/4
// and the operations at most double the relative errors: < 2^-92.
func atan2Fast(y, x float64) (float64, Status, bool) {
	if !(math.Abs(x) <= math.MaxFloat64 && math.Abs(y) <= math.MaxFloat64) || x == 0 && y == 0 {
		return 0, OK, false
	}
	T := tab()
	switch {
	case y == 0 && x > 0:
		return 0, OK, true
	case y == 0: // π, a pair within 2^-106
		return roundDD(T.pi[0], T.pi[1], 0, epsFast)
	case x == 0: // ±π/2
		return roundDD(math.Copysign(T.halfPi[0], y), math.Copysign(T.halfPi[1], y), 0, epsFast)
	}
	ma, ea := math.Frexp(math.Abs(y))
	mb, eb := math.Frexp(math.Abs(x))
	swap := math.Abs(y) > math.Abs(x)
	n, d, dd := ma, mb, ea-eb
	if swap {
		n, d, dd = mb, ma, eb-ea
	}
	qh := n / d
	ql := math.FMA(-qh, d, n) / d
	qh, ql = fastTwoSum(qh, ql)
	var h, l float64
	e := 0
	if dd >= -40 {
		h, l = atanDD(math.Ldexp(qh, dd), math.Ldexp(ql, dd))
	} else {
		c := float64(float64(float64(qh*qh)*qh) * (1.0 / 3))
		h, l = ddAddD(qh, ql, -float64(c*math.Ldexp(1, 2*dd)))
		e = dd
	}
	if swap || x < 0 {
		if e != 0 {
			h, l = math.Ldexp(h, e), math.Ldexp(l, e)
			e = 0
		}
		switch {
		case !swap: // π − θ
			h, l = ddAdd(T.pi[0], T.pi[1], -h, -l)
		case x > 0: // π/2 − θ
			h, l = ddAdd(T.halfPi[0], T.halfPi[1], -h, -l)
		default: // π/2 + θ
			h, l = ddAdd(T.halfPi[0], T.halfPi[1], h, l)
		}
	}
	if y < 0 {
		h, l = -h, -l
	}
	return roundDD(h, l, e, epsFast)
}

// sqrt1mx2 gives, for 0 ≤ a ≤ 1, s = √((1 − a)(1 + a)) = √(1 − a²) as a
// pair: 1 − a and 1 + a exact as pairs (twoSum), their product < 2^-104,
// the square root < 2^-104 plus half that: < 2^-103.4.
func sqrt1mx2(a float64) (float64, float64) {
	h1, l1 := twoSum(1, -a)
	h2, l2 := twoSum(1, a)
	ph, pl := ddMul(h1, l1, h2, l2)
	return ddSqrt(ph, pl)
}

// asinFast. With a = |x| and s = √(1 − a²): for a ≤ 0.7071067811865476,
// asin a = atan(a/s), a/s ≤ 1 + 2^-52; else asin a = π/2 − atan(s/a),
// s/a < 1, and the result ≥ π/4. The ratio adds 2^-102 to s's error, and
// atan's condition number z·atan'(z)/atan z is ≤ 1, so atan is within
// 2^-94 + 2^-101.3; the subtraction from π/2 at most doubles that: < 2^-92.
func asinFast(x float64) (float64, Status, bool) {
	a := math.Abs(x)
	if !(a <= 1) {
		return 0, OK, false
	}
	if a < tiny {
		return x, OK, true
	}
	sh, sl := sqrt1mx2(a)
	var h, l float64
	if a <= 0.7071067811865476 {
		h, l = atanDD(ddDiv(a, 0, sh, sl))
	} else {
		T := tab()
		h, l = atanDD(ddDiv(sh, sl, a, 0))
		h, l = ddAdd(T.halfPi[0], T.halfPi[1], -h, -l)
	}
	if x < 0 {
		h, l = -h, -l
	}
	return roundDD(h, l, 0, epsFast)
}

// acosFast. With a = |x| and s = √(1 − a²), θ = acos a: for a ≥
// 0.7071067811865476, θ = atan(s/a); else θ = π/2 − atan(a/s) ≥ π/4. For
// x < 0, acos x = π − θ ≥ π/2. Errors as for asin: < 2^-92.
func acosFast(x float64) (float64, Status, bool) {
	a := math.Abs(x)
	if !(a <= 1) {
		return 0, OK, false
	}
	T := tab()
	sh, sl := sqrt1mx2(a)
	var h, l float64
	if a >= 0.7071067811865476 {
		h, l = atanDD(ddDiv(sh, sl, a, 0))
	} else {
		h, l = atanDD(ddDiv(a, 0, sh, sl))
		h, l = ddAdd(T.halfPi[0], T.halfPi[1], -h, -l)
	}
	if x < 0 {
		h, l = ddAdd(T.pi[0], T.pi[1], -h, -l)
	}
	return roundDD(h, l, 0, epsFast)
}

// --- pow -----------------------------------------------------------------

// epsPow is the bound for pow. t = y·log x is a pair: log x within 2^-91
// relative, the product within 2^-105, so t within 2^-90.9·|t|; outside
// |t| ≤ 746 the value is decided without it. An absolute error ε in t is a
// relative error e^ε − 1 < 1.01ε in e^t: < 746·2^-90.9·1.01 < 2^-81.3.
// (Where y·log x underflows, t is within 2^-1074 of its value and e^t near
// 1.) With expDD's 2^-91: < 2^-81.2. The test uses 2^-70.
const epsPow = 0x1p-70

// powFast is x^y for finite x > 0, x ≠ 1, finite y ≠ 0, where powExact has
// found the value neither a double nor a midpoint.
func powFast(x, y float64) (float64, Status, bool) {
	if !(x > 0 && x <= math.MaxFloat64 && math.Abs(y) <= math.MaxFloat64) {
		return 0, OK, false
	}
	lh, ll := logDD(x)
	p := float64(lh * y)
	switch {
	case p > 710:
		// t > 710(1 − 2^-50) > ln 2^1024 = 709.78…: overflow.
		return 0, Overflow, true
	case p < -746:
		// t < −745.9 < ln 2^-1075 = −745.13…: below half the least
		// subnormal, the value rounds to 0.
		return 0, OK, true
	}
	th, tl := ddMulD(lh, ll, y)
	hi, lo, m := expDD(th, tl)
	return roundDD(hi, lo, m, epsPow)
}
