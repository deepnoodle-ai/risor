package crmath

import (
	"math/rand/v2"
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
		b.Logf("%s: worst %v at %v", f.name, worst, wa)
	}
}
