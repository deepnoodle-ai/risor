//go:build !race

package risor

import (
	"context"
	"runtime"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// The constants that risor.go publishes (02-evaluation.md 8.5 item 2).
const (
	bytesPerUnit = 256
	bytesPerStep = 16
)

// TestMemoryConstants measures the heap memory of runs that make the shapes
// with the most bytes per unit: short strings of three-byte characters just
// under a unit's 64 weight, one-entry and empty maps, empty lists, and boxed
// numbers. Each run's bytes stay under bytesPerUnit per unit charged plus
// bytesPerStep per step charged. The race detector changes allocation, so
// the test does not run under it.
func TestMemoryConstants(t *testing.T) {
	env, err := NewEnv(Limits{Steps: NoLimit, Allocation: NoLimit})
	assert.NoError(t, err)
	for _, src := range []string{
		`range(10000).map(x => "€".repeat(63))`,
		`range(10000).map(x => "😀".repeat(31))`,
		`range(10000).map(x => "a".repeat(63))`,
		`range(10000).map(x => "€" + string(x))`,
		`range(10000).map(x => {k: x})`,
		`range(10000).map(x => {a: x, b: x, c: x, d: x, e: x, f: x, g: x, h: x, i: x})`,
		`range(10000).map(x => {})`,
		`range(10000).map(x => [])`,
		`range(10000).map(x => [x])`,
		`range(10000).map(x => x * 1.5)`,
		`range(10000).reduce((a, x) => a + x * 1.5, 0)`,
		`range(500).reduce((m, x) => {...m, [string(x)]: x}, {})`,
		`range(10000).map(x => {key: string(x), value: x}).toMap()`,
	} {
		x, err := Parse(src)
		assert.NoError(t, err)
		p, err := env.Compile(x)
		assert.NoError(t, err, src)
		_, _, err = p.Measure(context.Background(), nil) // fill the pool of run states
		assert.NoError(t, err)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, u, err := p.Measure(context.Background(), nil)
		runtime.ReadMemStats(&after)
		assert.NoError(t, err)
		bytes := after.TotalAlloc - before.TotalAlloc
		bound := uint64(bytesPerUnit*u.Allocated + bytesPerStep*u.Steps)
		assert.True(t, bytes <= bound, "%s: %d bytes, bound %d (%d units, %d steps)", src, bytes, bound, u.Allocated, u.Steps)
	}
}
