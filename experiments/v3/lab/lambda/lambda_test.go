package lambda

import (
	"reflect"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func newFrame() *frame {
	nums := make([]any, 10_000)
	for i := range nums {
		nums[i] = int64(i)
	}
	rows := make([]any, 100)
	for i := range rows {
		vals := make([]any, 100)
		for j := range vals {
			vals[j] = int64(j)
		}
		rows[i] = map[string]any{"min": int64(i), "vals": vals}
	}
	rt := &runtime{globals: []any{nums, int64(5000), rows}}
	f := &frame{rt: rt}
	f.slots = f.inline[:]
	return f
}

var cases = []struct {
	name  string
	impls map[string]node
}{
	{
		// nums.filter(x => x > limit) over 10,000 ints
		"filter_10k", map[string]node{
			"call_pooled": filterCall(glob(0), function(&proto{nparams: 1, nslots: 1, recycle: true, body: greater(slot(0), glob(1))})),
			"call_alloc":  filterCall(glob(0), function(&proto{nparams: 1, nslots: 1, body: greater(slot(0), glob(1))})),
			"inline":      filterInline(glob(0), 0, greater(slot(0), glob(1))),
		},
	},
	{
		// rows.map(r => r.vals.filter(v => v > r.min)): the inner literal
		// captures r, so the spike cannot pool the outer frames.
		"nested_100x100", map[string]node{
			"call_pooled": mapCall(glob(2), function(&proto{
				nparams: 1, nslots: 1,
				body: filterCall(field(slot(0), "vals"), function(&proto{
					nparams: 1, nslots: 1, recycle: true,
					body: greater(slot(0), field(outer(0), "min")),
				})),
			})),
			"inline": mapInline(glob(2), 0,
				filterInline(field(slot(0), "vals"), 1, greater(slot(1), field(slot(0), "min")))),
		},
	},
}

func TestAgree(t *testing.T) {
	for _, c := range cases {
		var want any
		for name, n := range c.impls {
			fr := newFrame()
			fr.rt.left = 1 << 40
			got, err := n(fr)
			assert.NoError(t, err, "%s/%s", c.name, name)
			if want == nil {
				want = got
				continue
			}
			assert.True(t, reflect.DeepEqual(got, want), "%s/%s", c.name, name)
		}
	}
}

var sink any

func BenchmarkLambda(b *testing.B) {
	for _, c := range cases {
		for _, name := range []string{"call_pooled", "call_alloc", "inline"} {
			n, ok := c.impls[name]
			if !ok {
				continue
			}
			b.Run(c.name+"/"+name, func(b *testing.B) {
				fr := newFrame()
				b.ReportAllocs()
				for b.Loop() {
					fr.rt.left = 1 << 40
					sink, _ = n(fr)
				}
			})
		}
	}
}
