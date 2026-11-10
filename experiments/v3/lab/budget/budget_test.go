package budget

import (
	"context"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
)

var modes = []struct {
	name  string
	m     mode
	clock bool
}{
	{"none", none, false},
	{"up", up, false},
	{"down", down, false},
	{"chunk", chunk, false},
	{"chunk_clock", chunk, true},
	{"ctx_each", ctxEach, false},
}

// a + b + c + d < e: four operators.
func arith(m mode) node {
	return binary(m, binary(m, binary(m, binary(m, glob(0), glob(1), addValues), glob(2), addValues), glob(3), addValues), glob(4), lessValues)
}

// nums.count(x => x < 5000) over 10,000 ints: two charges per item.
func loop(m mode) node {
	return count(m, glob(5), binary(m, slot(0), konst(int64(5000)), lessValues))
}

func newRuntime(ctx context.Context) *runtime {
	nums := make([]any, 10_000)
	for i := range nums {
		nums[i] = int64(i)
	}
	return &runtime{
		ctx: ctx, done: ctx.Done(),
		globals: []any{int64(1), int64(2), int64(3), int64(4), int64(50), nums},
		slots:   make([]any, 1),
	}
}

func (rt *runtime) start(m mode, n int64) {
	if m == chunk {
		rt.resetChunk(n)
		return
	}
	rt.reset(n)
}

func TestBudget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	for _, md := range modes {
		rt := newRuntime(ctx)
		rt.start(md.m, 1_000_000)
		v, err := loop(md.m)(rt)
		assert.NoError(t, err, md.name)
		assert.Equal(t, v, int64(5000), md.name)
		if md.m == none {
			continue
		}
		// 20,000 charges are needed. One fewer must fail, exactly.
		rt.start(md.m, 19_999)
		_, err = loop(md.m)(rt)
		assert.Equal(t, err, errBudget, md.name)
		rt.start(md.m, 20_000)
		_, err = loop(md.m)(rt)
		assert.NoError(t, err, md.name)
	}
	// A canceled context stops the chunked loop within one chunk.
	cancel()
	rt := newRuntime(ctx)
	rt.start(chunk, 1_000_000)
	_, err := loop(chunk)(rt)
	assert.Equal(t, err, context.Canceled)
	assert.True(t, 1_000_000-rt.remaining <= 2*chunkSize)

	// A deadline that has passed stops it the same way, with no signal.
	rt = newRuntime(context.Background())
	rt.deadline = time.Now().Add(-time.Second)
	rt.start(chunk, 1_000_000)
	_, err = loop(chunk)(rt)
	assert.Equal(t, err, errDeadline)
	assert.True(t, 1_000_000-rt.remaining <= 2*chunkSize)
}

var (
	sink    any
	sinkErr error
)

func BenchmarkBudget(b *testing.B) {
	// A context that can be canceled, as a host with a timeout passes.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, c := range []struct {
		name  string
		build func(mode) node
	}{{"arith", arith}, {"count_10k", loop}} {
		for _, md := range modes {
			n := c.build(md.m)
			b.Run(c.name+"/"+md.name, func(b *testing.B) {
				rt := newRuntime(ctx)
				if md.clock {
					rt.deadline = time.Now().Add(time.Hour)
				}
				b.ReportAllocs()
				for b.Loop() {
					rt.start(md.m, 1_000_000)
					sink, sinkErr = n(rt)
				}
			})
		}
	}
}
