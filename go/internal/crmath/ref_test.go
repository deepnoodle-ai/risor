package crmath

import (
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

// The reference of these tests computes each function at refPrec bits by
// another method than the package's: π by the arithmetic-geometric mean
// (the package uses Machin's formula); e^x by e^n e^f with e = Σ 1/k! (the
// package reduces by ln 2); the logarithm by Halley's iteration on the
// reference exp (the package by atanh); sine and cosine by their series on
// the argument reduced by 2π (the package reduces by π/2); the arctangent
// by Newton's iteration on the reference sine and cosine (the package
// halves the angle and sums a series).

const refPrec = 320

func rf(x float64, p uint) *big.Float { return new(big.Float).SetPrec(p).SetFloat64(x) }

var refPiCache struct {
	sync.Mutex
	p uint
	v *big.Float
}

// refPi is π by the Gauss–Legendre algorithm.
func refPi(p uint) *big.Float {
	refPiCache.Lock()
	defer refPiCache.Unlock()
	if refPiCache.p >= p {
		return new(big.Float).SetPrec(p).Set(refPiCache.v)
	}
	w := p + 64
	one := new(big.Float).SetPrec(w).SetInt64(1)
	a := new(big.Float).SetPrec(w).Set(one)
	b := new(big.Float).SetPrec(w).Quo(one, new(big.Float).SetPrec(w).Sqrt(new(big.Float).SetPrec(w).SetInt64(2)))
	t := new(big.Float).SetPrec(w).Quo(one, new(big.Float).SetPrec(w).SetInt64(4))
	pp := new(big.Float).SetPrec(w).Set(one)
	for i := 0; i < 40; i++ {
		an := new(big.Float).SetPrec(w).Add(a, b)
		an.Quo(an, new(big.Float).SetPrec(w).SetInt64(2))
		bn := new(big.Float).SetPrec(w).Mul(a, b)
		bn.Sqrt(bn)
		d := new(big.Float).SetPrec(w).Sub(a, an)
		d.Mul(d, d)
		d.Mul(d, pp)
		t.Sub(t, d)
		pp.Mul(pp, new(big.Float).SetPrec(w).SetInt64(2))
		a, b = an, bn
	}
	s := new(big.Float).SetPrec(w).Add(a, b)
	s.Mul(s, s)
	s.Quo(s, new(big.Float).SetPrec(w).Mul(t, new(big.Float).SetPrec(w).SetInt64(4)))
	refPiCache.p, refPiCache.v = p, s
	return new(big.Float).SetPrec(p).Set(s)
}

var refE = func() *big.Float {
	w := uint(refPrec + 128)
	sum := new(big.Float).SetPrec(w).SetInt64(1)
	term := new(big.Float).SetPrec(w).SetInt64(1)
	for k := int64(1); k < 200; k++ {
		term.Quo(term, new(big.Float).SetPrec(w).SetInt64(k))
		sum.Add(sum, term)
	}
	return sum
}()

func refExp(x *big.Float, p uint) *big.Float {
	w := p + 64
	xf, _ := x.Float64()
	n := math.Round(xf)
	f := new(big.Float).SetPrec(w).Sub(x, rf(n, w))
	sum := new(big.Float).SetPrec(w).SetInt64(1)
	term := new(big.Float).SetPrec(w).SetInt64(1)
	for k := int64(1); k < 400; k++ {
		term.Mul(term, f)
		term.Quo(term, new(big.Float).SetPrec(w).SetInt64(k))
		sum.Add(sum, term)
		if term.Sign() == 0 || term.MantExp(nil) < -int(w)-8 {
			break
		}
	}
	// e^n by squaring.
	en := new(big.Float).SetPrec(w).SetInt64(1)
	base := new(big.Float).SetPrec(w).Set(refE)
	m := int64(math.Abs(n))
	for m > 0 {
		if m&1 == 1 {
			en.Mul(en, base)
		}
		base.Mul(base, base)
		m >>= 1
	}
	if n < 0 {
		en.Quo(new(big.Float).SetPrec(w).SetInt64(1), en)
	}
	return new(big.Float).SetPrec(p).Mul(sum, en)
}

func refLog(x *big.Float, p uint) *big.Float {
	w := p + 64
	// The first guess is taken from the mantissa, which is in [0.5, 1), and
	// not from math.Log of x itself: on some platforms math.Log is far off
	// for a subnormal argument, and Newton's steps below do not recover.
	m := new(big.Float)
	e := x.MantExp(m)
	mf, _ := m.Float64()
	y := rf(math.Log(mf)+float64(e)*math.Ln2, w)
	for i := 0; i < 6; i++ {
		ey := refExp(y, w)
		num := new(big.Float).SetPrec(w).Sub(x, ey)
		den := new(big.Float).SetPrec(w).Add(x, ey)
		num.Quo(num, den)
		num.Mul(num, new(big.Float).SetPrec(w).SetInt64(2))
		y.Add(y, num)
	}
	return new(big.Float).SetPrec(p).Set(y)
}

func refSinCos(x *big.Float, p uint) (*big.Float, *big.Float) {
	_, e := x.Float64()
	_ = e
	ex := x.MantExp(nil)
	if ex < 0 {
		ex = 0
	}
	w := p + uint(ex) + 128
	twoPi := refPi(w)
	twoPi.Mul(twoPi, new(big.Float).SetPrec(w).SetInt64(2))
	q := new(big.Float).SetPrec(w).Quo(x, twoPi)
	qi, _ := q.Int(nil)
	r := new(big.Float).SetPrec(w).Sub(new(big.Float).SetPrec(w).Set(x), new(big.Float).SetPrec(w).Mul(new(big.Float).SetPrec(w).SetInt(qi), twoPi))
	r.SetPrec(p + 64)
	wp := p + 64
	r2 := new(big.Float).SetPrec(wp).Mul(r, r)
	sin := new(big.Float).SetPrec(wp).Set(r)
	cos := new(big.Float).SetPrec(wp).SetInt64(1)
	ts := new(big.Float).SetPrec(wp).Set(r)
	tc := new(big.Float).SetPrec(wp).SetInt64(1)
	for n := int64(1); n < 300; n++ {
		tc.Mul(tc, r2)
		tc.Quo(tc, new(big.Float).SetPrec(wp).SetInt64((2*n-1)*(2*n)))
		ts.Mul(ts, r2)
		ts.Quo(ts, new(big.Float).SetPrec(wp).SetInt64((2*n)*(2*n+1)))
		if n%2 == 1 {
			cos.Sub(cos, tc)
			sin.Sub(sin, ts)
		} else {
			cos.Add(cos, tc)
			sin.Add(sin, ts)
		}
	}
	return sin, cos
}

func refAtan(z *big.Float, p uint) *big.Float {
	w := p + 64
	zf, _ := z.Float64()
	if math.IsInf(zf, 0) || math.Abs(zf) > 1e30 {
		hp := refPi(w)
		hp.Quo(hp, new(big.Float).SetPrec(w).SetInt64(2))
		inv := new(big.Float).SetPrec(w).Quo(new(big.Float).SetPrec(w).SetInt64(1), z)
		a := refAtan(inv, w)
		if zf > 0 {
			return new(big.Float).SetPrec(p).Sub(hp, a)
		}
		return new(big.Float).SetPrec(p).Sub(new(big.Float).Neg(hp), a)
	}
	y := rf(math.Atan(zf), w)
	for i := 0; i < 6; i++ {
		s, c := refSinCos(y, w)
		f := new(big.Float).SetPrec(w).Sub(s, new(big.Float).SetPrec(w).Mul(z, c))
		d := new(big.Float).SetPrec(w).Add(c, new(big.Float).SetPrec(w).Mul(z, s))
		f.Quo(f, d)
		y.Sub(y, f)
	}
	return new(big.Float).SetPrec(p).Set(y)
}

type refResult struct {
	v  float64
	st Status
}

func refRound(v *big.Float) refResult {
	f, _ := v.Float64()
	if math.IsInf(f, 0) {
		return refResult{0, Overflow}
	}
	if f == 0 {
		f = 0
	}
	return refResult{f, OK}
}

func ref(name string, x, y float64) refResult {
	p := uint(refPrec)
	switch name {
	case "exp":
		if x > 1000 {
			return refResult{0, Overflow}
		}
		if x < -1000 {
			return refResult{0, OK}
		}
		return refRound(refExp(rf(x, p), p))
	case "log":
		if x <= 0 {
			return refResult{0, NotReal}
		}
		return refRound(refLog(rf(x, p), p))
	case "log10":
		if x <= 0 {
			return refResult{0, NotReal}
		}
		l := refLog(rf(x, p), p)
		return refRound(l.Quo(l, refLog(rf(10, p), p)))
	case "sin":
		s, _ := refSinCos(rf(x, p), p)
		return refRound(s)
	case "cos":
		_, c := refSinCos(rf(x, p), p)
		return refRound(c)
	case "tan":
		s, c := refSinCos(rf(x, p), p)
		return refRound(s.Quo(s, c))
	case "atan":
		return refRound(refAtan(rf(x, p), p))
	case "asin", "acos":
		if x < -1 || x > 1 {
			return refResult{0, NotReal}
		}
		var a *big.Float
		hp := refPi(p + 64)
		hp.Quo(hp, new(big.Float).SetPrec(p+64).SetInt64(2))
		switch {
		case x == 1:
			a = new(big.Float).SetPrec(p + 64).Set(hp)
		case x == -1:
			a = new(big.Float).SetPrec(p + 64).Neg(hp)
		default:
			w := p + 64
			bx := rf(x, w)
			one := new(big.Float).SetPrec(w).SetInt64(1)
			d := new(big.Float).SetPrec(w).Sub(one, new(big.Float).SetPrec(w).Mul(bx, bx))
			d.Sqrt(d)
			a = refAtan(new(big.Float).SetPrec(w).Quo(bx, d), w)
		}
		if name == "asin" {
			return refRound(a)
		}
		return refRound(new(big.Float).SetPrec(p+64).Sub(hp, a))
	case "atan2":
		y, x = x, y // the arguments are (y, x)
		w := p + 64
		if y == 0 && x >= 0 {
			return refResult{0, OK}
		}
		pi := refPi(w)
		if y == 0 {
			return refRound(pi)
		}
		if x == 0 {
			pi.Quo(pi, new(big.Float).SetPrec(w).SetInt64(2))
			if y < 0 {
				pi.Neg(pi)
			}
			return refRound(pi)
		}
		a := refAtan(new(big.Float).SetPrec(w).Quo(rf(y, w), rf(x, w)), w)
		if x < 0 {
			if y > 0 {
				a.Add(a, pi)
			} else {
				a.Sub(a, pi)
			}
		}
		return refRound(a)
	case "pow":
		if y == 0 {
			return refResult{1, OK}
		}
		if x == 0 {
			if y > 0 {
				return refResult{0, OK}
			}
			return refResult{0, NotReal}
		}
		yInt := y == math.Trunc(y)
		if x < 0 && !yInt {
			return refResult{0, NotReal}
		}
		neg := x < 0 && math.Abs(y) < 1<<53 && math.Mod(y, 2) != 0
		ax := math.Abs(x)
		var r refResult
		if yInt && math.Abs(y) <= 64 {
			q := new(big.Rat).SetFloat64(ax)
			res := new(big.Rat).SetInt64(1)
			for i := 0; i < int(math.Abs(y)); i++ {
				res.Mul(res, q)
			}
			if y < 0 {
				res.Inv(res)
			}
			f, _ := res.Float64()
			r = refResult{f, OK}
			if math.IsInf(f, 0) {
				r = refResult{0, Overflow}
			}
		} else {
			t := new(big.Float).SetPrec(p+64).Mul(rf(y, p+64), refLog(rf(ax, p+64), p+64))
			tf, _ := t.Float64()
			switch {
			case tf > 1000:
				r = refResult{0, Overflow}
			case tf < -1000:
				r = refResult{0, OK}
			default:
				r = refRound(refExp(t, p))
			}
		}
		if neg && r.st == OK && r.v != 0 {
			r.v = -r.v
		}
		return r
	}
	panic(name)
}

// The functions, Go's own for the log of disagreements, and arguments.
type fn struct {
	name string
	one  func(float64) (float64, Status)
	two  func(float64, float64) (float64, Status)
	goF  func(x, y float64) float64
	gen  func(r *rand.Rand) (float64, float64)
}

func randBits(r *rand.Rand, maxExp int) float64 {
	for {
		e := r.IntN(maxExp+1074) - 1074
		f := math.Ldexp(1+r.Float64(), e)
		if r.IntN(2) == 0 {
			f = -f
		}
		if !math.IsInf(f, 0) {
			return f
		}
	}
}

var fns = []fn{
	{"exp", Exp, nil, func(x, _ float64) float64 { return math.Exp(x) }, func(r *rand.Rand) (float64, float64) {
		if r.IntN(2) == 0 {
			return -745.2 + r.Float64()*(709.8+745.2), 0
		}
		return randBits(r, 9), 0
	}},
	{"log", Log, nil, func(x, _ float64) float64 { return math.Log(x) }, func(r *rand.Rand) (float64, float64) {
		switch r.IntN(3) {
		case 0:
			return 1 + (r.Float64()-0.5)*math.Ldexp(1, -r.IntN(50)), 0
		}
		return math.Abs(randBits(r, 1023)), 0
	}},
	{"log10", Log10, nil, func(x, _ float64) float64 { return math.Log10(x) }, func(r *rand.Rand) (float64, float64) {
		return math.Abs(randBits(r, 1023)), 0
	}},
	{"sin", Sin, nil, func(x, _ float64) float64 { return math.Sin(x) }, func(r *rand.Rand) (float64, float64) {
		if r.IntN(2) == 0 {
			return (r.Float64() - 0.5) * 20, 0
		}
		return randBits(r, 1023), 0
	}},
	{"cos", Cos, nil, func(x, _ float64) float64 { return math.Cos(x) }, func(r *rand.Rand) (float64, float64) {
		if r.IntN(2) == 0 {
			return (r.Float64() - 0.5) * 20, 0
		}
		return randBits(r, 1023), 0
	}},
	{"tan", Tan, nil, func(x, _ float64) float64 { return math.Tan(x) }, func(r *rand.Rand) (float64, float64) {
		if r.IntN(2) == 0 {
			return (r.Float64() - 0.5) * 20, 0
		}
		return randBits(r, 1023), 0
	}},
	{"asin", Asin, nil, func(x, _ float64) float64 { return math.Asin(x) }, func(r *rand.Rand) (float64, float64) {
		if r.IntN(3) == 0 {
			return math.Copysign(1-math.Ldexp(r.Float64(), -r.IntN(53)), r.Float64()-0.5), 0
		}
		return r.Float64()*2 - 1, 0
	}},
	{"acos", Acos, nil, func(x, _ float64) float64 { return math.Acos(x) }, func(r *rand.Rand) (float64, float64) {
		if r.IntN(3) == 0 {
			return math.Copysign(1-math.Ldexp(r.Float64(), -r.IntN(53)), r.Float64()-0.5), 0
		}
		return r.Float64()*2 - 1, 0
	}},
	{"atan", Atan, nil, func(x, _ float64) float64 { return math.Atan(x) }, func(r *rand.Rand) (float64, float64) {
		if r.IntN(2) == 0 {
			return (r.Float64() - 0.5) * 8, 0
		}
		return randBits(r, 1023), 0
	}},
	{"atan2", nil, Atan2, func(y, x float64) float64 { return math.Atan2(y, x) }, func(r *rand.Rand) (float64, float64) {
		if r.IntN(2) == 0 {
			return r.Float64()*4 - 2, r.Float64()*4 - 2
		}
		return randBits(r, 1023), randBits(r, 1023)
	}},
	{"pow", nil, Pow, func(x, y float64) float64 { return math.Pow(x, y) }, func(r *rand.Rand) (float64, float64) {
		switch r.IntN(4) {
		case 0:
			return -(r.Float64() * 10), float64(r.IntN(41) - 20)
		case 1:
			return r.Float64() * 1000, (r.Float64() - 0.5) * 100
		case 2:
			return math.Abs(randBits(r, 20)), (r.Float64() - 0.5) * 60
		}
		return r.Float64() * 4, float64(r.IntN(129) - 64)
	}},
}

func (f fn) call(x, y float64) (float64, Status) {
	if f.one != nil {
		return f.one(x)
	}
	return f.two(x, y)
}

// hard are the hard arguments of each function.
var hard = map[string][][2]float64{
	"exp": {{709.782712893384, 0}, {709.7827128933841, 0}, {-745.1332191019411, 0}, {-745.1332191019412, 0},
		{-708.3964185322641, 0}, {-740, 0}, {1e-300, 0}, {-1e-300, 0}, {5e-324, 0}, {1, 0}, {0.5, 0}, {-0.5, 0}, {7.124576406741286e-218, 0}},
	"log": {{1 + 0x1p-52, 0}, {1 - 0x1p-53, 0}, {2.718281828459045, 0}, {5e-324, 0}, {math.MaxFloat64, 0},
		{0x1.fffffffffffffp-1, 0}, {1.0000000000000002, 0}, {0.9999999999999999, 0}, {2, 0}, {10, 0}},
	"log10": {{1e-300, 0}, {1e300, 0}, {1e23, 0}, {1e22, 0}, {0.1, 0}, {5e-324, 0}, {2, 0}},
	"sin": {{0x1.6ac5b262ca1ffp+849, 0}, {6381956970095103 * 0x1p797, 0}, {1e22, 0}, {3.141592653589793, 0},
		{1.5707963267948966, 0}, {5e-324, 0}, {1e-310, 0}, {math.MaxFloat64, 0}, {355, 0}, {103993, 0}, {1.4e-8, 0}},
	"cos": {{0x1.6ac5b262ca1ffp+849, 0}, {6381956970095103 * 0x1p797, 0}, {1e22, 0}, {1.5707963267948966, 0},
		{3.141592653589793, 0}, {math.MaxFloat64, 0}, {1e-8, 0}, {355, 0}},
	"tan":  {{1.5707963267948966, 0}, {6381956970095103 * 0x1p797, 0}, {1e22, 0}, {1e300, 0}, {5e-324, 0}, {1, 0}, {-1, 0}},
	"asin": {{1, 0}, {-1, 0}, {0.5, 0}, {1 - 0x1p-53, 0}, {-1 + 0x1p-53, 0}, {5e-324, 0}, {1e-9, 0}},
	"acos": {{1, 0}, {-1, 0}, {0.5, 0}, {1 - 0x1p-53, 0}, {-1 + 0x1p-53, 0}, {0, 0}, {5e-324, 0}},
	"atan": {{1e300, 0}, {-1e300, 0}, {1, 0}, {5e-324, 0}, {1e-9, 0}, {-0.5, 0}, {0x1p53, 0}},
	"atan2": {{1, -1e-300}, {-1, -1}, {5e-324, -1}, {-5e-324, -1}, {1e300, 1e-300}, {1e-300, 1e300}, {1, 1},
		{5e-324, 1e300}, {1, 0}, {-1, 0}, {0, -1}, {0, 0}},
	"pow": {{2, -1075}, {2, -1074}, {2, 1023}, {2, 1024}, {0.5, 1075}, {-2, 1025}, {-1, 1e300}, {-1, 3},
		{9, 0.5}, {16, 0.25}, {1.5, 2}, {-3, 3}, {0x1p-537, 2}, {10, 308.25}, {2, 1023.9999999999999},
		{1.1, 10}, {2, 0.5}, {10, 400}, {0.1, 3}, {3, -1}, {4, 1.5}, {4, -0.5}, {65536, 0.0625}, {0x1p-1074, 0.5},
		{0x1p-1074, 1.5}, {1e300, 1e-300}, {1.0000000000000002, 1e16}, {0.9999999999999999, 1e17}},
}

func TestReference(t *testing.T) {
	n := 300
	if s := os.Getenv("RISOR_CRMATH_N"); s != "" {
		n, _ = strconv.Atoi(s)
	}
	if testing.Short() {
		n = 200
	}
	for _, f := range fns {
		t.Run(f.name, func(t *testing.T) {
			type arg struct{ x, y float64 }
			var args []arg
			for _, h := range hard[f.name] {
				args = append(args, arg{h[0], h[1]})
			}
			rng := rand.New(rand.NewPCG(1, uint64(len(f.name))))
			for i := 0; i < n; i++ {
				x, y := f.gen(rng)
				args = append(args, arg{x, y})
			}
			var mu sync.Mutex
			bad, goDiffs := 0, 0
			var times []time.Duration
			var wg sync.WaitGroup
			ch := make(chan arg)
			for g := 0; g < runtime.NumCPU(); g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for a := range ch {
						t0 := time.Now()
						v, st := f.call(a.x, a.y)
						d := time.Since(t0)
						want := ref(f.name, a.x, a.y)
						gv := f.goF(a.x, a.y)
						mu.Lock()
						times = append(times, d)
						if st != want.st || (st == OK && v != want.v) {
							bad++
							if bad < 20 {
								t.Errorf("%s(%v, %v) = %v %v, want %v %v", f.name, a.x, a.y, v, st, want.v, want.st)
							}
						}
						if st == OK && !math.IsNaN(gv) && !math.IsInf(gv, 0) && gv != v && !(gv == 0 && v == 0) {
							goDiffs++
							if goDiffs <= 5 {
								t.Logf("Go's math differs: %s(%v, %v): Go %v, correctly rounded %v", f.name, a.x, a.y, gv, v)
							}
						}
						mu.Unlock()
					}
				}()
			}
			for _, a := range args {
				ch <- a
			}
			close(ch)
			wg.Wait()
			sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
			med := times[len(times)/2]
			p99 := times[len(times)*99/100]
			t.Logf("%s: %d arguments, %d wrong, Go's math differs on %d; time median %v, p99 %v, max %v",
				f.name, len(args), bad, goDiffs, med, p99, times[len(times)-1])
		})
	}
	r, rt := Rounds.Load(), Retries.Load()
	t.Logf("evaluations %d, past the first precision %d (%.3f%%)", r, rt, 100*float64(rt)/float64(max(r, 1)))
	_ = fmt.Sprint
}
