package crmath

import (
	"math"
	"math/big"
	mrand "math/rand"
	"math/rand/v2"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// TestFastAgainstSlow compares every entry, with its fast phase, against the
// same entry with the slow path alone, on edge arguments and on random
// arguments over each domain. The two must give the same double and status
// everywhere.
//
// The number of random arguments per function is 20,000, or
// RISOR_CRMATH_FAST_N:
//
//	RISOR_CRMATH_FAST_N=1000000 GOWORK=off go test -run FastAgainstSlow -v -timeout 1h ./internal/crmath
func TestFastAgainstSlow(t *testing.T) {
	n := 20000
	if s := os.Getenv("RISOR_CRMATH_FAST_N"); s != "" {
		v, err := strconv.Atoi(s)
		assert.Nil(t, err)
		n = v
	}
	if testing.Short() {
		n = 2000
	}
	for _, f := range fastFns {
		t.Run(f.name, func(t *testing.T) {
			args := f.edges()
			ne := len(args)
			rng := rand.New(rand.NewPCG(11, uint64(len(f.name))))
			for range n {
				x, y := f.gen(rng)
				args = append(args, [2]float64{x, y})
			}
			var bad atomic.Int64
			var mu sync.Mutex
			run := func(args [][2]float64) int64 {
				before := Fallbacks.Load()
				var wg sync.WaitGroup
				ch := make(chan [2]float64, 256)
				for range runtime.NumCPU() {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for a := range ch {
							v, st := f.fn(a[0], a[1], true)
							w, sw := f.fn(a[0], a[1], false)
							if st != sw || math.Float64bits(v) != math.Float64bits(w) {
								if bad.Add(1) <= 20 {
									mu.Lock()
									t.Errorf("%s(%v, %v): fast %v (%x) %v, slow %v (%x) %v", f.name, a[0], a[1],
										v, math.Float64bits(v), st, w, math.Float64bits(w), sw)
									mu.Unlock()
								}
							}
						}
					}()
				}
				for _, a := range args {
					ch <- a
				}
				close(ch)
				wg.Wait()
				return Fallbacks.Load() - before
			}
			fe := run(args[:ne])
			fr := run(args[ne:])
			t.Logf("%s: %d edge and %d random arguments, %d differ; fell back to the slow path: edge %d (%.4f%%), random %d (%.5f%%)",
				f.name, ne, n, bad.Load(), fe, 100*float64(fe)/float64(max(ne, 1)), fr, 100*float64(fr)/float64(max(n, 1)))
			assert.Equal(t, bad.Load(), int64(0))
		})
	}
}

type fastFn struct {
	name  string
	fn    func(x, y float64, fast bool) (float64, Status)
	gen   func(r *rand.Rand) (float64, float64)
	edges func() [][2]float64
}

func one(f func(float64, bool) (float64, Status)) func(x, y float64, fast bool) (float64, Status) {
	return func(x, _ float64, fast bool) (float64, Status) { return f(x, fast) }
}

// anyBits is a double with uniformly random bits: every exponent equally
// likely, subnormals included.
func anyBits(r *rand.Rand) float64 {
	for {
		v := math.Float64frombits(r.Uint64())
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return v
		}
	}
}

// mix picks one of the generators at random.
func mix(gs ...func(r *rand.Rand) float64) func(r *rand.Rand) (float64, float64) {
	return func(r *rand.Rand) (float64, float64) { return gs[r.IntN(len(gs))](r), 0 }
}

func uniform(lo, hi float64) func(r *rand.Rand) float64 {
	return func(r *rand.Rand) float64 { return lo + r.Float64()*(hi-lo) }
}

func positive(r *rand.Rand) float64 { return math.Abs(anyBits(r)) }

// near1 is 1 + d with |d| of any size below 2^-1.
func near1(r *rand.Rand) float64 {
	d := math.Ldexp(r.Float64(), -1-r.IntN(60))
	if r.IntN(2) == 0 {
		d = -d / 2
	}
	return 1 + d
}

// unitNear1 is ±(1 − d), d of any size, for asin and acos.
func unitNear1(r *rand.Rand) float64 {
	v := 1 - math.Ldexp(r.Float64(), -r.IntN(60))
	if r.IntN(2) == 0 {
		v = -v
	}
	return v
}

// neighbors gives each of xs and its k neighbors on each side.
func neighbors(k int, xs ...float64) [][2]float64 {
	var out [][2]float64
	for _, x := range xs {
		lo, hi := x, x
		out = append(out, [2]float64{x, 0})
		for range k {
			lo = math.Nextafter(lo, math.Inf(-1))
			hi = math.Nextafter(hi, math.Inf(1))
			for _, v := range []float64{lo, hi} {
				if !math.IsInf(v, 0) { // the entries take finite numbers only
					out = append(out, [2]float64{v, 0})
				}
			}
		}
	}
	return out
}

func withNeg(a [][2]float64) [][2]float64 {
	out := a
	for _, v := range a {
		out = append(out, [2]float64{-v[0], v[1]})
	}
	return out
}

// someSubnormals and some tiny and huge arguments.
func extremes() []float64 {
	xs := []float64{5e-324, 1e-323, 2.5e-323, 1e-310, 0x1p-1022, 0x1.fffffffffffffp-1023, 1e-300, 1e-200,
		1e-100, 0x1p-60, 0x1p-30, 0x1p-29, 0x1p-28, 0x1p-27, 0x1p-26, 1e-8, 0.1, 0.5, 1, 2, 10, 1e10,
		1e100, 1e300, math.MaxFloat64}
	r := rand.New(rand.NewPCG(3, 3))
	for range 200 {
		xs = append(xs, math.Float64frombits(r.Uint64()>>12)) // subnormal
	}
	return xs
}

func flts(a [][2]float64) []float64 {
	var out []float64
	for _, v := range a {
		out = append(out, v[0])
	}
	return out
}

// bigNearest is the double nearest v.
func bigNearest(v *big.Float) float64 { f, _ := v.Float64(); return f }

// multiplesOf gives the doubles nearest k·c for small k and for random k of
// every size up to 2^1000, c a big constant.
func multiplesOf(c *big.Float, small int, random int) []float64 {
	var xs []float64
	for k := 1; k <= small; k++ {
		xs = append(xs, bigNearest(new(big.Float).SetPrec(2200).Mul(c, big.NewFloat(float64(k)))))
	}
	r := mrand.New(mrand.NewSource(5))
	for range random {
		b := 1 + r.Intn(1000)
		k := new(big.Int).Rand(r, new(big.Int).Lsh(big.NewInt(1), uint(b)))
		k.SetBit(k, b, 1)
		v := new(big.Float).SetPrec(2200).SetInt(k)
		v.Mul(v, c)
		if f := bigNearest(v); !math.IsInf(f, 0) {
			xs = append(xs, f)
		}
	}
	return xs
}

func trigEdges() [][2]float64 {
	hp := pi(2200)
	hp.SetMantExp(hp, -1)
	xs := multiplesOf(hp, 3000, 3000)
	q := pi(2200)
	q.SetMantExp(q, -2) // π/4: where the reduction starts
	xs = append(xs, bigNearest(q), 0.785, 0.78)
	xs = append(xs, extremes()...)
	for j := 1; j <= 51; j++ {
		xs = append(xs, (float64(j)+0.5)/64) // table boundaries
	}
	xs = append(xs, 0x1.6ac5b262ca1ffp+849, 6381956970095103*0x1p797, 1e22, 355, 103993, 0x1p1023)
	return withNeg(neighbors(3, xs...))
}

var fastFns = []fastFn{
	{"exp", one(exp), mix(uniform(-746, 710), uniform(-1, 1), uniform(-1000, 1000), func(r *rand.Rand) float64 {
		return randBits(r, 10)
	}), func() [][2]float64 {
		xs := []float64{709.782712893384, 709.7827128933841, -708.3964185322641, -708.3964185322642,
			-745.1332191019411, -745.1332191019412, -744.44007192138122, -1000, 1000, 0x1p-54, -0x1p-54, 0x1p-53, -0x1p-55}
		l2 := ln2(2200)
		for k := -1075; k <= 1024; k++ {
			xs = append(xs, bigNearest(new(big.Float).SetPrec(2200).Mul(l2, big.NewFloat(float64(k)))))
		}
		for k := -4096; k <= 4096; k++ { // near the reduction's boundaries
			v := new(big.Float).SetPrec(2200).Mul(l2, big.NewFloat((float64(k)+0.5)/4096))
			xs = append(xs, bigNearest(v))
		}
		xs = append(xs, extremes()...)
		return withNeg(neighbors(4, xs...))
	}},
	{"log", one(log), mix(positive, near1, uniform(0, 4), uniform(0.5, 2)), func() [][2]float64 {
		xs := []float64{1 - 0x1p-8, 1 + 0x1p-7, 2.718281828459045, 0.5, 2, 1.5}
		for k := -1074; k <= 1023; k++ {
			xs = append(xs, math.Ldexp(1, k))
		}
		for k := -700; k <= 700; k++ {
			xs = append(xs, bigNearest(expBig(big.NewFloat(float64(k)).SetPrec(300), 300)))
		}
		for i := range 128 {
			xs = append(xs, 1+float64(i)/128, 0.5+float64(i)/256)
		}
		xs = append(xs, extremes()...)
		out := neighbors(3, xs...)
		out = append(out, neighbors(3000, 1)...)
		return out
	}},
	{"log10", one(log10), mix(positive, near1, uniform(0, 100)), func() [][2]float64 {
		var xs []float64
		for k := -323; k <= 308; k++ {
			p := new(big.Float).SetPrec(4000).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(k))), nil))
			if k < 0 {
				p.Quo(new(big.Float).SetPrec(4000).SetInt64(1), p)
			}
			xs = append(xs, bigNearest(p))
		}
		xs = append(xs, extremes()...)
		out := neighbors(3, xs...)
		return append(out, neighbors(300, 1)...)
	}},
	{"sin", one(sin), mix(anyBits, uniform(-10, 10), uniform(-1e6, 1e6)), trigEdges},
	{"cos", one(cos), mix(anyBits, uniform(-10, 10), uniform(-1e6, 1e6)), trigEdges},
	{"tan", one(tan), mix(anyBits, uniform(-10, 10), uniform(-1e6, 1e6)), trigEdges},
	{"asin", one(asin), mix(uniform(-1, 1), unitNear1, func(r *rand.Rand) float64 {
		return math.Copysign(math.Ldexp(1+r.Float64(), -r.IntN(1075)), r.Float64()-0.5)
	}), func() [][2]float64 {
		xs := []float64{1, 0.7071067811865476, 0.5, 0x1p-28}
		xs = append(xs, extremes()[:40]...)
		for j := range 65 { // where atan's argument meets the table's boundaries
			xs = append(xs, (float64(j)+0.5)/64/math.Sqrt(1+((float64(j)+0.5)/64)*((float64(j)+0.5)/64)))
		}
		out := withNeg(neighbors(3, xs...))
		return append(out, withNeg(neighbors(3000, 1))...)
	}},
	{"acos", one(acos), mix(uniform(-1, 1), unitNear1, func(r *rand.Rand) float64 {
		return math.Copysign(math.Ldexp(1+r.Float64(), -r.IntN(1075)), r.Float64()-0.5)
	}), func() [][2]float64 {
		xs := []float64{1, 0.7071067811865476, 0.5, 0x1p-28, 0}
		xs = append(xs, extremes()[:40]...)
		out := withNeg(neighbors(3, xs...))
		return append(out, withNeg(neighbors(3000, 1))...)
	}},
	{"atan", one(atan), mix(anyBits, uniform(-4, 4), uniform(-100, 100)), func() [][2]float64 {
		xs := []float64{1, 0x1p53, 0x1p1022, 0x1p1023}
		for j := range 65 {
			xs = append(xs, (float64(j)+0.5)/64, 64/(float64(j)+0.5))
		}
		xs = append(xs, extremes()...)
		return withNeg(neighbors(3, xs...))
	}},
	{"atan2", atan2, func(r *rand.Rand) (float64, float64) {
		switch r.IntN(4) {
		case 0:
			return r.Float64()*4 - 2, r.Float64()*4 - 2
		case 1:
			return anyBits(r), anyBits(r)
		case 2: // a ratio near 2^-40 and below
			y := math.Ldexp(anyBits(r), -64)
			return y, math.Copysign(math.Ldexp(math.Abs(y), 30+r.IntN(30)), r.Float64()-0.5)
		}
		x := anyBits(r)
		return x * (1 + (r.Float64()-0.5)*0x1p-20), x
	}, func() [][2]float64 {
		out := [][2]float64{{1, -1e-300}, {-1, -1}, {5e-324, -1}, {-5e-324, -1}, {1e300, 1e-300}, {1e-300, 1e300},
			{1, 1}, {5e-324, 1e300}, {5e-324, 1}, {1e-300, 1}, {1, 0x1p40}, {1, 0x1p41}, {1, -0x1p41},
			{math.MaxFloat64, 5e-324}, {5e-324, math.MaxFloat64}, {-math.MaxFloat64, -5e-324},
			{0, -1}, {0, -5e-324}, {0, 1}, {1, 0}, {-1, 0}, {5e-324, 0}, {-math.MaxFloat64, 0}, {0, math.MaxFloat64}}
		ex := extremes()
		for i, y := range ex {
			for _, x := range ex[i%7:] {
				if i%3 == 0 {
					out = append(out, [2]float64{y, x}, [2]float64{-y, -x}, [2]float64{y, -x})
				}
			}
		}
		return out
	}},
	{"pow", pow, func(r *rand.Rand) (float64, float64) {
		switch r.IntN(6) {
		case 0:
			return -(r.Float64() * 10), float64(r.IntN(41) - 20)
		case 1:
			return r.Float64() * 1000, (r.Float64() - 0.5) * 100
		case 2:
			return math.Abs(randBits(r, 20)), (r.Float64() - 0.5) * 60
		case 3:
			return near1(r), anyBits(r)
		case 4:
			return positive(r), (r.Float64() - 0.5) * 4
		}
		return r.Float64() * 4, float64(r.IntN(129) - 64)
	}, func() [][2]float64 {
		out := append([][2]float64(nil), hard["pow"]...)
		for k := -1074; k <= 1023; k += 7 {
			x := math.Ldexp(1, k)
			out = append(out, [2]float64{x, 0.5}, [2]float64{x, 1.5}, [2]float64{x, 1.0 / 3}, [2]float64{x, -0.75},
				[2]float64{3, float64(k) / 4}, [2]float64{10, float64(k) / 3})
		}
		for b := 2; b <= 40; b++ {
			for n := -40; n <= 40; n++ {
				out = append(out, [2]float64{float64(b), float64(n)}, [2]float64{-float64(b), float64(n)},
					[2]float64{float64(b * b), float64(n) / 2}, [2]float64{1 / float64(b), float64(n)})
			}
		}
		// Near the overflow and underflow limits.
		for _, x := range []float64{2, 10, 0.5, 1.5, 7} {
			lx := math.Log2(x)
			for _, lim := range []float64{1024, -1022, -1074, -1075} {
				y := lim / lx
				for _, d := range []float64{-2e-15, -1e-16, 0, 1e-16, 2e-15} {
					out = append(out, [2]float64{x, y * (1 + d)})
				}
			}
		}
		for _, x := range flts(neighbors(5, 1)) {
			out = append(out, [2]float64{x, 1e16}, [2]float64{x, -1e17}, [2]float64{x, 0x1p60}, [2]float64{x, 3})
		}
		for _, x := range extremes() {
			out = append(out, [2]float64{x, 0.5}, [2]float64{x, -0.5}, [2]float64{x, 2}, [2]float64{x, -1.25})
		}
		return out
	}},
}
