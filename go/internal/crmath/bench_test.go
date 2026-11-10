package crmath

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

// BenchmarkTypical is each function on its random arguments; the worst case
// of each over its hard arguments is logged.
//
//	GOWORK=off go test -run xxx -bench Typical ./internal/crmath
func BenchmarkTypical(b *testing.B) {
	for _, f := range fns {
		rng := rand.New(rand.NewPCG(7, 7))
		args := make([][2]float64, 1024)
		for i := range args {
			args[i][0], args[i][1] = f.gen(rng)
		}
		b.Run(f.name, func(b *testing.B) {
			i := 0
			for b.Loop() {
				a := args[i&1023]
				f.call(a[0], a[1])
				i++
			}
		})
		var worst time.Duration
		var wa [2]float64
		for _, h := range append(hard[f.name], args[:256]...) {
			t0 := time.Now()
			f.call(h[0], h[1])
			if d := time.Since(t0); d > worst {
				worst, wa = d, h
			}
		}
		// The median of the per-argument cost, each timed over 64 calls.
		per := make([]time.Duration, len(args))
		for i, a := range args {
			t0 := time.Now()
			for range 64 {
				f.call(a[0], a[1])
			}
			per[i] = time.Since(t0) / 64
		}
		slices.Sort(per)
		b.Logf("%s: median %v, worst %v at %v", f.name, per[len(per)/2], worst, wa)
	}
}

// BenchmarkFastPhase is the fast phase alone on the same arguments: what a
// call costs when the fast phase decides it, which is the median call.
//
//	GOWORK=off go test -run xxx -bench FastPhase ./internal/crmath
func BenchmarkFastPhase(b *testing.B) {
	phases := map[string]func(x, y float64) (float64, Status, bool){
		"exp":   func(x, _ float64) (float64, Status, bool) { return expFast(x) },
		"log":   func(x, _ float64) (float64, Status, bool) { return logFast(x) },
		"log10": func(x, _ float64) (float64, Status, bool) { return log10Fast(x) },
		"sin":   func(x, _ float64) (float64, Status, bool) { return sinFast(x) },
		"cos":   func(x, _ float64) (float64, Status, bool) { return cosFast(x) },
		"tan":   func(x, _ float64) (float64, Status, bool) { return tanFast(x) },
		"asin":  func(x, _ float64) (float64, Status, bool) { return asinFast(x) },
		"acos":  func(x, _ float64) (float64, Status, bool) { return acosFast(x) },
		"atan":  func(x, _ float64) (float64, Status, bool) { return atanFast(x) },
		"atan2": atan2Fast,
		"pow":   func(x, y float64) (float64, Status, bool) { return powFast(math.Abs(x), y) },
	}
	tab()
	for _, f := range fns {
		rng := rand.New(rand.NewPCG(7, 7))
		args := make([][2]float64, 1024)
		for i := range args {
			args[i][0], args[i][1] = f.gen(rng)
		}
		ph := phases[f.name]
		b.Run(f.name, func(b *testing.B) {
			i := 0
			for b.Loop() {
				a := args[i&1023]
				ph(a[0], a[1])
				i++
			}
		})
	}
}

// powArgs are pow's arguments of two kinds: exact, where the value is a
// double, a midpoint or found by the exact check (small integer powers,
// squares and roots, powers of two), and inexact, where the exact check
// finds nothing and the fast phase decides.
func powArgs() (exact, inexact [][2]float64) {
	rng := rand.New(rand.NewPCG(9, 9))
	exact = [][2]float64{{1.5, 2}, {10, 22}, {10, 23}, {-3, 3}, {0.1, 2}, {4, 0.5}, {2, -1075}, {9, 1.5}}
	for len(exact) < 1024 {
		switch rng.IntN(3) {
		case 0: // a short mantissa to a small integer power
			x := float64(rng.IntN(2000)-1000) / 8
			exact = append(exact, [2]float64{x, float64(rng.IntN(9))})
		case 1: // a square
			exact = append(exact, [2]float64{rng.Float64() * 100, 2})
		default: // a root of a square
			s := float64(rng.IntN(1 << 20))
			exact = append(exact, [2]float64{s * s, 0.5})
		}
	}
	for len(inexact) < 1024 {
		switch rng.IntN(3) {
		case 0:
			inexact = append(inexact, [2]float64{rng.Float64() * 100, (rng.Float64() - 0.5) * 20})
		case 1:
			inexact = append(inexact, [2]float64{rng.Float64() * 4, float64(rng.IntN(40) + 5)})
		default:
			inexact = append(inexact, [2]float64{rng.Float64() * 100, float64(rng.IntN(64)) / 8})
		}
	}
	return exact, inexact
}

// BenchmarkPow is pow on exact and inexact arguments.
//
//	GOWORK=off go test -run xxx -bench Pow ./internal/crmath
func BenchmarkPow(b *testing.B) {
	exact, inexact := powArgs()
	for _, s := range []struct {
		name string
		args [][2]float64
	}{{"exact", exact}, {"inexact", inexact}} {
		b.Run(s.name, func(b *testing.B) {
			i := 0
			for b.Loop() {
				a := s.args[i&1023]
				Pow(a[0], a[1])
				i++
			}
		})
		per := make([]time.Duration, len(s.args))
		for i, a := range s.args {
			t0 := time.Now()
			for range 64 {
				Pow(a[0], a[1])
			}
			per[i] = time.Since(t0) / 64
		}
		slices.Sort(per)
		b.Logf("pow %s: median %v, p99 %v", s.name, per[len(per)/2], per[len(per)*99/100])
	}
}
