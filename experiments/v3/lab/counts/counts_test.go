package counts

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

const (
	gX = iota
	gFlag
	gInput
	gRole
	gBroken
)

func newRuntime() *runtime {
	return &runtime{globals: []any{
		int64(7), true,
		map[string]any{"event": map[string]any{"pull_request": map[string]any{"head": map[string]any{"sha": "0123456789abcdef"}}}},
		"reviewer",
		map[string]any{"event": map[string]any{"pull_request": map[string]any{}}},
	}}
}

var roles = []string{"admin", "owner", "maintainer", "billing", "security", "reviewer", "triage", "guest"}

type variant struct {
	name string
	n    node
}

var cases = []struct {
	name     string
	variants []variant
	// preserving reports whether the variant must match the reference's
	// count and its outcome at every budget.
}{
	{
		// x * (60 * 60 * 24)
		"fold", []variant{
			{"reference", mul(glob(gX), mul(mul(konst(int64(60)), konst(int64(60)), 1), konst(int64(24)), 1), 1)},
			{"free", mul(glob(gX), konst(int64(86400)), 1)},
			{"preserving", mul(glob(gX), konst(int64(86400)), 3)},
		},
	},
	{
		// flag && 60 * 60 * 24: the folded constant has no operator above it
		"fold_bare", []variant{
			{"reference", and(glob(gFlag), mul(mul(konst(int64(60)), konst(int64(60)), 1), konst(int64(24)), 1))},
			{"free", and(glob(gFlag), konst(int64(86400)))},
			{"preserving", and(glob(gFlag), charged(int64(86400), 2))},
		},
	},
	{
		// input.event.pull_request.head.sha
		"path", []variant{
			{"reference", field(field(field(field(glob(gInput), "event"), "pull_request"), "head"), "sha")},
			{"free", pathFree(glob(gInput), "event", "pull_request", "head", "sha")},
			{"preserving", pathPreserving(glob(gInput), "event", "pull_request", "head", "sha")},
		},
	},
	{
		// the same path over data where the third field is missing
		"path_missing", []variant{
			{"reference", field(field(field(field(glob(gBroken), "event"), "pull_request"), "head"), "sha")},
			{"free", pathFree(glob(gBroken), "event", "pull_request", "head", "sha")},
			{"preserving", pathPreserving(glob(gBroken), "event", "pull_request", "head", "sha")},
		},
	},
	{
		// role in [8 constants], found at the sixth
		"in", []variant{
			{"reference", inScan(glob(gRole), roles)},
			{"free", inSetFree(glob(gRole), roles)},
			{"preserving", inSetPreserving(glob(gRole), roles)},
		},
	},
}

// TestPreserving runs the reference and the preserving variant at every
// budget from 0 up, and requires the same value, the same error and the same
// steps left.
func TestPreserving(t *testing.T) {
	for _, c := range cases {
		ref, pres := c.variants[0].n, c.variants[2].n
		for budget := int64(0); budget <= 12; budget++ {
			a, b := newRuntime(), newRuntime()
			a.left, b.left = budget, budget
			av, aerr := ref(a)
			bv, berr := pres(b)
			assert.Equal(t, bv, av, "%s at %d", c.name, budget)
			assert.Equal(t, berr, aerr, "%s at %d", c.name, budget)
			if aerr != errSteps {
				assert.Equal(t, b.left, a.left, "%s at %d: steps left", c.name, budget)
			}
		}
	}
}

var (
	sink    any
	sinkErr error
)

func BenchmarkCounts(b *testing.B) {
	for _, c := range cases {
		for _, v := range c.variants {
			b.Run(c.name+"/"+v.name, func(b *testing.B) {
				rt := newRuntime()
				b.ReportAllocs()
				for b.Loop() {
					rt.left = 1 << 40
					sink, sinkErr = v.n(rt)
				}
			})
		}
	}
}
