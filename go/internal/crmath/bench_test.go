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
