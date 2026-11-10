package alloc

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

const (
	gA = iota
	gB
	gC
	gNums
	gLimit
	gNames
	gFirst
	gLast
)

func newRuntime(charge bool) *runtime {
	nums := make([]any, 10_000)
	for i := range nums {
		nums[i] = int64(i)
	}
	names := make([]any, 1_000)
	for i := range names {
		names[i] = fmt.Sprintf("user_%d", i)
	}
	return &runtime{
		globals: []any{int64(1), int64(2), int64(3), nums, int64(5000), names, "Ada", "Lovelace"},
		slots:   make([]any, 1),
		charge:  charge,
	}
}

var cases = []struct {
	name   string
	impls  []string
	build  func(impl string) node
	units  int64 // allocation units the expression must cost
	result func(any) int
}{
	{
		// [a, b, c]
		name: "list_literal", impls: []string{""},
		build: func(string) node { return list(glob(gA), glob(gB), glob(gC)) },
		units: 4,
	},
	{
		// nums.filter(x => x > limit).map(x => x + 1) over 10,000
		name: "filter_map_10k", impls: []string{"grow", "exact"},
		build: func(impl string) node {
			pred := greater(slot(0), glob(gLimit))
			f := filterGrow(glob(gNums), pred)
			if impl == "exact" {
				f = filterExact(glob(gNums), pred)
			}
			return mapList(f, add(slot(0), konst(int64(1))))
		},
		units: (1 + 4999) + (1 + 4999),
	},
	{
		// "Hello, " + first + " " + last: three strings, all short
		name: "concat_short", impls: []string{""},
		build: func(string) node {
			return add(add(add(konst("Hello, "), glob(gFirst)), konst(" ")), glob(gLast))
		},
		units: 3,
	},
	{
		// names.map(n => n + "@example.com") over 1,000
		name: "map_concat_1k", impls: []string{""},
		build: func(string) node { return mapList(glob(gNames), add(slot(0), konst("@example.com"))) },
		units: 1 + 1000 + 1000,
	},
	{
		// names.join(", ") over 1,000: one string of 9,890 characters
		name: "join_1k", impls: []string{""},
		build: func(string) node { return join(glob(gNames), ", ") },
		units: 1 + 9890/64,
	},
}

const budget = 1 << 40

func TestUnits(t *testing.T) {
	for _, c := range cases {
		var want any
		for _, impl := range c.impls {
			n := c.build(impl)
			plain := newRuntime(false)
			plain.steps = budget
			v, err := n(plain)
			assert.NoError(t, err)
			if want == nil {
				want = v
			}
			assert.True(t, reflect.DeepEqual(v, want), "%s/%s", c.name, impl)

			rt := newRuntime(true)
			rt.steps, rt.units = budget, budget
			v, err = n(rt)
			assert.NoError(t, err)
			assert.True(t, reflect.DeepEqual(v, want), "%s/%s", c.name, impl)
			assert.Equal(t, budget-rt.units, c.units, "%s/%s units", c.name, impl)

			// One unit fewer fails, and exactly enough passes.
			rt.steps, rt.units = budget, c.units-1
			_, err = n(rt)
			assert.Equal(t, err, errUnits, "%s/%s", c.name, impl)
			rt.steps, rt.units = budget, c.units
			_, err = n(rt)
			assert.NoError(t, err, "%s/%s", c.name, impl)
		}
	}
}

var (
	sink    any
	sinkErr error
)

func BenchmarkAlloc(b *testing.B) {
	for _, c := range cases {
		for _, impl := range c.impls {
			name := c.name
			if impl != "" {
				name += "_" + impl
			}
			n := c.build(impl)
			for _, charge := range []bool{false, true} {
				mode := "steps_only"
				if charge {
					mode = "steps_and_units"
				}
				b.Run(name+"/"+mode, func(b *testing.B) {
					rt := newRuntime(charge)
					b.ReportAllocs()
					for b.Loop() {
						rt.steps, rt.units = budget, budget
						sink, sinkErr = n(rt)
					}
				})
			}
		}
	}
}
