// Package crmath computes the correctly rounded results of package F13
// (spec/packages/f13-math.md): of the exact value of each function at its
// double arguments, the nearest double, ties to even, subnormals included.
//
// No result comes from Go's math package, which does not promise correct
// rounding. Each function evaluates its exact value with math/big at a
// working precision w, with a relative error far below 2^-(w-Guard), and
// rounds it when the whole interval of that error rounds to one double;
// else it doubles w and tries again (Ziv's strategy). The arguments where
// the exact value is a double or a midpoint between two doubles, at which
// no precision decides, are found first: the trivial arguments, the powers
// of ten for log10, and the exact cases of pow. Elsewhere the value is
// transcendental, so it is never a midpoint, and the loop ends.
//
// That evaluation is the slow path. Each entry first tries a fast phase in
// double-double arithmetic with a proven error bound (fast.go), which
// decides nearly every call in tens of nanoseconds; the slow path runs only
// when the fast phase's error interval straddles a rounding boundary.
package crmath

import (
	"math"
	"math/big"
	"sync"
	"sync/atomic"
)

// Status is how a call ends.
type Status int

// The ends of a call.
const (
	OK       Status = iota
	NotReal         // the exact value is not a real number
	Overflow        // the value rounds to 2^1024 or more
)

// Guard is the bits of the working precision not trusted: the evaluation's
// rounding errors and truncated series stay well below 2^-(w-Guard).
const Guard = 40

const (
	startPrec = 128
	maxPrec   = 1 << 14
)

// Rounds counts the evaluations and the evaluations past the first
// precision, for measurement.
var Rounds, Retries atomic.Int64

// Fallbacks counts the calls whose fast phase did not decide the result
// and that ran the slow path, for measurement.
var Fallbacks atomic.Int64

// ziv evaluates f at growing precision until its value rounds.
func ziv(f func(w uint) *big.Float) (float64, Status) {
	for w := uint(startPrec); ; w *= 2 {
		Rounds.Add(1)
		if w > startPrec {
			Retries.Add(1)
		}
		v := f(w)
		if r, st, ok := roundTo(v, w); ok {
			return r, st
		}
		if w >= maxPrec {
			panic("crmath: the rounding is not decided")
		}
	}
}

// roundTo rounds v, which is within a relative 2^-(w-Guard) of the exact
// value, when the whole interval rounds to one double.
func roundTo(v *big.Float, w uint) (float64, Status, bool) {
	if v.Sign() == 0 {
		return 0, OK, true
	}
	p := w + 16
	d := new(big.Float).SetPrec(p).Abs(v)
	d.SetMantExp(d, -int(w-Guard))
	lo := new(big.Float).SetPrec(p).Sub(v, d)
	hi := new(big.Float).SetPrec(p).Add(v, d)
	fl, _ := lo.Float64()
	fh, _ := hi.Float64()
	if fl != fh {
		return 0, OK, false
	}
	if math.IsInf(fl, 0) {
		return 0, Overflow, true
	}
	if fl == 0 {
		fl = 0 // one zero
	}
	return fl, OK, true
}

func fl(x float64, w uint) *big.Float { return new(big.Float).SetPrec(w).SetFloat64(x) }

func nf(w uint) *big.Float { return new(big.Float).SetPrec(w) }

func ival(n int64, w uint) *big.Float { return new(big.Float).SetPrec(w).SetInt64(n) }

// --- constants -------------------------------------------------------------

type constCache struct {
	mu   sync.Mutex
	prec uint
	v    *big.Float
	make func(w uint) *big.Float
}

func (c *constCache) get(w uint) *big.Float {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.prec < w {
		p := w + 64
		if p < 256 {
			p = 256
		}
		c.v, c.prec = c.make(p), p
	}
	return new(big.Float).SetPrec(w).Set(c.v)
}

var (
	piC  = &constCache{make: machinPi}
	ln2C = &constCache{make: ln2}
	l10C = &constCache{make: func(w uint) *big.Float { return logBig(fl(10, w), w) }}
)

func pi(w uint) *big.Float { return piC.get(w) }

// atanInv is atan(1/n) by its series.
func atanInv(n int64, w uint) *big.Float {
	p := w + 16
	x := nf(p).Quo(ival(1, p), ival(n, p))
	x2 := nf(p).Mul(x, x)
	sum := nf(p).Set(x)
	term := nf(p).Set(x)
	eps := nf(p).SetMantExp(ival(1, p), -int(p))
	for k := int64(1); ; k++ {
		term.Mul(term, x2)
		t := nf(p).Quo(term, ival(2*k+1, p))
		if k%2 == 1 {
			sum.Sub(sum, t)
		} else {
			sum.Add(sum, t)
		}
		if t.Cmp(eps) < 0 {
			break
		}
	}
	return sum
}

// machinPi is π = 16 atan(1/5) − 4 atan(1/239).
func machinPi(w uint) *big.Float {
	p := w + 16
	a := nf(p).Mul(ival(16, p), atanInv(5, p))
	b := nf(p).Mul(ival(4, p), atanInv(239, p))
	return nf(w).Sub(a, b)
}

// ln2 is 2 atanh(1/3).
func ln2(w uint) *big.Float { return atanhSeries(nf(w+16).Quo(ival(1, w+16), ival(3, w+16)), w) }

// atanhSeries is 2 atanh(t) = 2 Σ t^(2k+1) / (2k+1), for |t| ≤ 1/3.
func atanhSeries(t *big.Float, w uint) *big.Float {
	p := w + 16
	t = nf(p).Set(t)
	t2 := nf(p).Mul(t, t)
	sum := nf(p).Set(t)
	term := nf(p).Set(t)
	lim := new(big.Float).SetPrec(p).Abs(t)
	lim.SetMantExp(lim, -int(p))
	for k := int64(1); ; k++ {
		term.Mul(term, t2)
		x := nf(p).Quo(term, ival(2*k+1, p))
		sum.Add(sum, x)
		if new(big.Float).Abs(x).Cmp(lim) <= 0 {
			break
		}
	}
	return nf(w).Mul(sum, ival(2, p))
}

// --- kernels ---------------------------------------------------------------

// expBig is e^x, for |x| up to a few thousand.
func expBig(x *big.Float, w uint) *big.Float {
	if x.Sign() == 0 {
		return ival(1, w)
	}
	p := w + 32
	xe := x.MantExp(nil)
	if xe < 0 {
		xe = 0
	}
	l2 := ln2C.get(p + uint(xe))
	kf := nf(p).Quo(x, l2)
	kf64, _ := kf.Float64()
	k := int64(math.Round(kf64))
	r := nf(p+uint(xe)).Sub(x, nf(p+uint(xe)).Mul(ival(k, p+uint(xe)), l2))
	r.SetPrec(p)
	const s = 10
	r.SetMantExp(r, -s)
	// Taylor series of e^r, |r| < 2^-10.
	sum := ival(1, p)
	term := ival(1, p)
	eps := nf(p).SetMantExp(ival(1, p), -int(p))
	for n := int64(1); ; n++ {
		term.Mul(term, r)
		term.Quo(term, ival(n, p))
		sum.Add(sum, term)
		if new(big.Float).Abs(term).Cmp(eps) < 0 {
			break
		}
	}
	for i := 0; i < s; i++ {
		sum.Mul(sum, sum)
	}
	return nf(w).SetMantExp(sum, int(k))
}

// logBig is the natural logarithm of a positive x.
func logBig(x *big.Float, w uint) *big.Float {
	p := w + 32
	m := nf(p)
	e := x.MantExp(m) // x = m 2^e, m in [0.5, 1)
	if m.Cmp(fl(math.Sqrt2/2, p)) < 0 {
		m.SetMantExp(m, 1)
		e--
	}
	one := ival(1, p)
	num := nf(p).Sub(m, one)
	den := nf(p).Add(m, one)
	t := nf(p).Quo(num, den)
	l := atanhSeries(t, p)
	if e != 0 {
		l.Add(l, nf(p).Mul(ival(int64(e), p), ln2C.get(p)))
	}
	return nf(w).Set(l)
}

// reduceHalfPi gives r and q with x = q π/2 + r, |r| ≤ π/4 (about), q mod 4.
func reduceHalfPi(x float64, w uint) (*big.Float, int) {
	if math.Abs(x) < 0.78 {
		return fl(x, w), 0
	}
	_, e := math.Frexp(x)
	if e < 0 {
		e = 0
	}
	p := w + uint(e) + 128
	hp := pi(p)
	hp.SetMantExp(hp, -1)
	bx := fl(x, p)
	kq := nf(p).Quo(bx, hp)
	// k = round(x / (π/2)), as a big integer.
	kq.Add(kq, fl(math.Copysign(0.5, x), p))
	ki, _ := kq.Int(nil)
	kb := nf(p).SetInt(ki)
	r := nf(p).Sub(bx, nf(p).Mul(kb, hp))
	q := int(new(big.Int).And(ki, big.NewInt(3)).Int64())
	return nf(w + 64).Set(r), q
}

// sinCosBig gives sin r and cos r by their series, for |r| ≤ 1.
func sinCosBig(r *big.Float, w uint) (*big.Float, *big.Float) {
	p := w + 32
	r = nf(p).Set(r)
	r2 := nf(p).Mul(r, r)
	eps := nf(p).SetMantExp(ival(1, p), -int(p)-8)
	sin, cos := nf(p).Set(r), ival(1, p)
	ts, tc := nf(p).Set(r), ival(1, p)
	rabs := new(big.Float).Abs(r)
	for n := int64(1); ; n++ {
		tc.Mul(tc, r2)
		tc.Quo(tc, ival((2*n-1)*(2*n), p))
		ts.Mul(ts, r2)
		ts.Quo(ts, ival((2*n)*(2*n+1), p))
		if n%2 == 1 {
			cos.Sub(cos, tc)
			sin.Sub(sin, ts)
		} else {
			cos.Add(cos, tc)
			sin.Add(sin, ts)
		}
		lim := nf(p).Mul(eps, rabs)
		if new(big.Float).Abs(ts).Cmp(lim) < 0 && new(big.Float).Abs(tc).Cmp(eps) < 0 {
			break
		}
	}
	return sin, cos
}

// sinCos gives the sine and cosine of a double.
func sinCos(x float64, w uint) (*big.Float, *big.Float) {
	r, q := reduceHalfPi(x, w)
	s, c := sinCosBig(r, w)
	switch q {
	case 1:
		s, c = c, s.Neg(s)
	case 2:
		s, c = s.Neg(s), c.Neg(c)
	case 3:
		s, c = c.Neg(c), s
	}
	return nf(w).Set(s), nf(w).Set(c)
}

// atanBig is the arctangent of z.
func atanBig(z *big.Float, w uint) *big.Float {
	p := w + 32
	z = nf(p).Set(z)
	if z.Sign() == 0 {
		return nf(w)
	}
	neg := z.Sign() < 0
	z.Abs(z)
	one := ival(1, p)
	inv := false
	if z.Cmp(one) > 0 {
		z.Quo(one, z)
		inv = true
	}
	// Halve the angle until z < 2^-8: atan z = 2 atan(z / (1 + √(1 + z²))).
	j := 0
	small := nf(p).SetMantExp(one, -8)
	for z.Cmp(small) > 0 {
		s := nf(p).Mul(z, z)
		s.Add(s, one)
		s.Sqrt(s)
		s.Add(s, one)
		z.Quo(z, s)
		j++
	}
	z2 := nf(p).Mul(z, z)
	sum := nf(p).Set(z)
	term := nf(p).Set(z)
	eps := nf(p).Mul(z, nf(p).SetMantExp(one, -int(p)-8))
	for k := int64(1); ; k++ {
		term.Mul(term, z2)
		t := nf(p).Quo(term, ival(2*k+1, p))
		if k%2 == 1 {
			sum.Sub(sum, t)
		} else {
			sum.Add(sum, t)
		}
		if t.Cmp(eps) < 0 {
			break
		}
	}
	sum.SetMantExp(sum, j)
	if inv {
		hp := pi(p)
		hp.SetMantExp(hp, -1)
		sum.Sub(hp, sum)
	}
	if neg {
		sum.Neg(sum)
	}
	return nf(w).Set(sum)
}

// --- the entries -----------------------------------------------------------

// Exp is e^x.
func Exp(x float64) (float64, Status) { return exp(x, true) }

func exp(x float64, fast bool) (float64, Status) {
	switch {
	case x == 0:
		return 1, OK
	case x > 1000:
		return 0, Overflow
	case x < -1000:
		return 0, OK
	}
	if fast {
		if v, st, ok := expFast(x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	return ziv(func(w uint) *big.Float { return expBig(fl(x, w), w) })
}

// Log is the natural logarithm.
func Log(x float64) (float64, Status) { return log(x, true) }

func log(x float64, fast bool) (float64, Status) {
	switch {
	case x <= 0:
		return 0, NotReal
	case x == 1:
		return 0, OK
	}
	if fast {
		if v, st, ok := logFast(x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	return ziv(func(w uint) *big.Float { return logBig(fl(x, w), w) })
}

// Log10 is the base-10 logarithm.
func Log10(x float64) (float64, Status) { return log10(x, true) }

func log10(x float64, fast bool) (float64, Status) {
	if x <= 0 {
		return 0, NotReal
	}
	p := 1.0
	for k := 0; k <= 22; k++ {
		if x == p {
			return float64(k), OK
		}
		p *= 10
	}
	if fast {
		if v, st, ok := log10Fast(x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	return ziv(func(w uint) *big.Float {
		l := logBig(fl(x, w+16), w+16)
		return nf(w).Quo(l, l10C.get(w+16))
	})
}

// Sin is the sine.
func Sin(x float64) (float64, Status) { return sin(x, true) }

func sin(x float64, fast bool) (float64, Status) {
	if x == 0 {
		return 0, OK
	}
	if fast {
		if v, st, ok := sinFast(x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	return ziv(func(w uint) *big.Float { s, _ := sinCos(x, w); return s })
}

// Cos is the cosine.
func Cos(x float64) (float64, Status) { return cos(x, true) }

func cos(x float64, fast bool) (float64, Status) {
	if x == 0 {
		return 1, OK
	}
	if fast {
		if v, st, ok := cosFast(x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	return ziv(func(w uint) *big.Float { _, c := sinCos(x, w); return c })
}

// Tan is the tangent.
func Tan(x float64) (float64, Status) { return tan(x, true) }

func tan(x float64, fast bool) (float64, Status) {
	if x == 0 {
		return 0, OK
	}
	if fast {
		if v, st, ok := tanFast(x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	return ziv(func(w uint) *big.Float {
		s, c := sinCos(x, w+16)
		return nf(w).Quo(s, c)
	})
}

// Asin is the arcsine.
func Asin(x float64) (float64, Status) { return asin(x, true) }

func asin(x float64, fast bool) (float64, Status) {
	switch {
	case x < -1 || x > 1:
		return 0, NotReal
	case x == 0:
		return 0, OK
	}
	if fast {
		if v, st, ok := asinFast(x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	return ziv(func(w uint) *big.Float {
		p := w + 16
		if x == 1 || x == -1 {
			hp := pi(p)
			hp.SetMantExp(hp, -1)
			if x < 0 {
				hp.Neg(hp)
			}
			return hp
		}
		// asin x = atan(x / √((1 − x)(1 + x))), the product exact.
		bx := fl(x, p)
		one := ival(1, p)
		d := nf(p).Mul(nf(p).Sub(one, bx), nf(p).Add(one, bx))
		d.Sqrt(d)
		return atanBig(nf(p).Quo(bx, d), w)
	})
}

// Acos is the arccosine.
func Acos(x float64) (float64, Status) { return acos(x, true) }

func acos(x float64, fast bool) (float64, Status) {
	switch {
	case x < -1 || x > 1:
		return 0, NotReal
	case x == 1:
		return 0, OK
	}
	if fast {
		if v, st, ok := acosFast(x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	return ziv(func(w uint) *big.Float {
		p := w + 16
		if x == -1 {
			return pi(p)
		}
		// acos x = 2 atan(√((1 − x) / (1 + x))).
		bx := fl(x, p)
		one := ival(1, p)
		q := nf(p).Quo(nf(p).Sub(one, bx), nf(p).Add(one, bx))
		q.Sqrt(q)
		a := atanBig(q, p)
		return nf(w).SetMantExp(a, 1)
	})
}

// Atan is the arctangent.
func Atan(x float64) (float64, Status) { return atan(x, true) }

func atan(x float64, fast bool) (float64, Status) {
	if x == 0 {
		return 0, OK
	}
	if fast {
		if v, st, ok := atanFast(x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	return ziv(func(w uint) *big.Float { return atanBig(fl(x, w), w) })
}

// Atan2 is the angle of the point (x, y), in (−π, π].
func Atan2(y, x float64) (float64, Status) { return atan2(y, x, true) }

func atan2(y, x float64, fast bool) (float64, Status) {
	if fast && (x != 0 || y != 0) {
		if v, st, ok := atan2Fast(y, x); ok {
			return v, st
		}
		Fallbacks.Add(1)
	}
	switch {
	case y == 0 && x >= 0:
		return 0, OK
	case y == 0:
		return ziv(func(w uint) *big.Float { return pi(w) })
	case x == 0:
		return ziv(func(w uint) *big.Float {
			hp := pi(w)
			hp.SetMantExp(hp, -1)
			if y < 0 {
				hp.Neg(hp)
			}
			return hp
		})
	}
	return ziv(func(w uint) *big.Float {
		p := w + 16
		a := atanBig(nf(p+64).Quo(fl(y, p), fl(x, p)), p)
		if x < 0 {
			if y > 0 {
				a.Add(a, pi(p))
			} else {
				a.Sub(a, pi(p))
			}
		}
		return nf(w).Set(a)
	})
}
