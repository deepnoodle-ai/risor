package runstate

import (
	"testing"
	"unsafe"

	"github.com/deepnoodle-ai/wonton/assert"
)

var env = map[string]any{
	"gate":  map[string]any{"failed": 2.0, "passed": 48.0},
	"input": map[string]any{"role": "owner"},
	"run":   map[string]any{"status": "completed"},
}

func TestRun(t *testing.T) {
	p := compile()
	for _, run := range []func(map[string]any, int64) (any, error){p.RunAllocSpike, p.RunAllocSmall, p.RunPooled} {
		v, err := run(env, 1000)
		assert.NoError(t, err)
		assert.Equal(t, v, true)
	}
	t.Logf("spikeRun %d bytes, smallRun %d bytes", unsafe.Sizeof(spikeRun{}), unsafe.Sizeof(smallRun{}))
}

var sink any

func BenchmarkRun(b *testing.B) {
	p := compile()
	impls := []struct {
		name string
		run  func(map[string]any, int64) (any, error)
	}{
		{"alloc_spike", p.RunAllocSpike},
		{"alloc_small", p.RunAllocSmall},
		{"pool", p.RunPooled},
	}
	for _, im := range impls {
		b.Run("serial/"+im.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sink, _ = im.run(env, 1000)
			}
		})
	}
	b.Run("serial/caller", func(b *testing.B) {
		r := &spikeRun{}
		b.ReportAllocs()
		for b.Loop() {
			sink, _ = p.RunWith(r, env, 1000)
		}
	})
	for _, im := range impls {
		b.Run("parallel/"+im.name, func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				var local any
				for pb.Next() {
					local, _ = im.run(env, 1000)
				}
				_ = local
			})
		})
	}
	b.Run("parallel/caller", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			r := &spikeRun{}
			var local any
			for pb.Next() {
				local, _ = p.RunWith(r, env, 1000)
			}
			_ = local
		})
	})
}
