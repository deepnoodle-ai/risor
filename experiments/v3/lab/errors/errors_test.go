package errors

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func newFrame() *frame {
	nums := make([]any, 10_000)
	for i := range nums {
		nums[i] = int64(i)
	}
	return &frame{
		rt: &runtime{max: 1 << 40, globals: []any{
			int64(1), int64(2), int64(3), int64(4), int64(5),
			map[string]any{"event": map[string]any{"pull_request": map[string]any{"head": map[string]any{"sha": "0123456789abcdef"}}}},
			nums,
		}},
		slots: make([]any, 1),
	}
}

func withStack(fr *frame) *frame {
	fr.rt.stack = make([]frame, 64)
	rows := make([]any, 10_000)
	for i := range rows {
		rows[i] = map[string]any{"min": int64(i % 100)}
	}
	fr.rt.globals = append(fr.rt.globals, rows)
	return fr
}

// fib(n) = if (n < 2) { n } else { fib(n - 1) + fib(n - 2) }
func pfib() pnode {
	fn := &pfunc{}
	n := pslot(0)
	fn.body = pif(pless(n, pkonst(int64(2)), 1), n,
		padd(pcall(fn, padd(n, pkonst(int64(-1)), 2)), pcall(fn, padd(n, pkonst(int64(-2)), 3)), 4))
	return pcall(fn, pkonst(int64(20)))
}

func efib() enode {
	fn := &efunc{}
	n := eslot(0)
	fn.body = eif(eless(n, ekonst(int64(2)), 1), n,
		eadd(ecall(fn, eadd(n, ekonst(int64(-1)), 2)), ecall(fn, eadd(n, ekonst(int64(-2)), 3)), 4))
	return ecall(fn, ekonst(int64(20)))
}

// nest wraps a node in n more closures, to make a deep tree.
func pnest(x pnode, n int) pnode {
	for range n {
		x = pparen(x)
	}
	return x
}

func enest(x enode, n int) enode {
	for range n {
		x = eparen(x)
	}
	return x
}

var cases = []struct {
	name string
	p    pnode
	e    enode
	fail bool
}{
	{
		"ok/arith", // a + b + c + d < e
		pless(padd(padd(padd(pglob(0), pglob(1), 3), pglob(2), 7), pglob(3), 11), pglob(4), 15),
		eless(eadd(eadd(eadd(eglob(0), eglob(1), 3), eglob(2), 7), eglob(3), 11), eglob(4), 15),
		false,
	},
	{
		"ok/arith_typed", // the same, in typed closures
		pbox(piless(piadd(piadd(piadd(piglob(0), piglob(1)), piglob(2)), piglob(3)), piglob(4))),
		ebox(eiless(eiadd(eiadd(eiadd(eiglob(0), eiglob(1)), eiglob(2)), eiglob(3)), eiglob(4))),
		false,
	},
	{
		"ok/field_chain", // input.event.pull_request.head.sha
		pfield(pfield(pfield(pfield(pglob(5), "event", 6), "pull_request", 12), "head", 25), "sha", 30),
		efield(efield(efield(efield(eglob(5), "event", 6), "pull_request", 12), "head", 25), "sha", 30),
		false,
	},
	{
		"ok/count_10k", // nums.count(x => x + 1 < 5000)
		pcount(pglob(6), pless(padd(pslot(0), pkonst(int64(1)), 20), pkonst(int64(5000)), 24)),
		ecount(eglob(6), eless(eadd(eslot(0), ekonst(int64(1)), 20), ekonst(int64(5000)), 24)),
		false,
	},
	{
		"ok/count_fields_10k", // rows.count(r => r.min < 50), which allocates nothing
		pcount(pglob(7), pless(pfield(pslot(0), "min", 18), pkonst(int64(50)), 24)),
		ecount(eglob(7), eless(efield(eslot(0), "min", 18), ekonst(int64(50)), 24)),
		false,
	},
	{"ok/fib_20", pfib(), efib(), false},
	{
		"ok/try", // try(input.event.pull_request, "none")
		ptry(pfield(pfield(pglob(5), "event", 6), "pull_request", 12), pkonst("none")),
		etry(efield(efield(eglob(5), "event", 6), "pull_request", 12), ekonst("none")),
		false,
	},
	{
		"fail/depth_4", // input.event.missing.head.sha
		pfield(pfield(pfield(pfield(pglob(5), "event", 6), "missing", 12), "head", 25), "sha", 30),
		efield(efield(efield(efield(eglob(5), "event", 6), "missing", 12), "head", 25), "sha", 30),
		true,
	},
	{
		"fail/depth_40",
		pnest(pfield(pglob(5), "missing", 6), 40),
		enest(efield(eglob(5), "missing", 6), 40),
		true,
	},
	{
		"fail/try", // try(input.missing.x, "none")
		ptry(pfield(pfield(pglob(5), "missing", 6), "x", 12), pkonst("none")),
		etry(efield(efield(eglob(5), "missing", 6), "x", 12), ekonst("none")),
		false,
	},
}

func TestAgree(t *testing.T) {
	for _, c := range cases {
		pv, perr := prun(c.p, withStack(newFrame()))
		ev, eerr := erun(c.e, withStack(newFrame()))
		assert.Equal(t, pv, ev, c.name)
		assert.Equal(t, perr != nil, c.fail, c.name)
		assert.Equal(t, eerr != nil, c.fail, c.name)
		if c.fail {
			assert.Equal(t, perr.Error(), eerr.Error(), c.name)
		}
	}
}

var (
	sink    any
	sinkErr error
)

func BenchmarkErrors(b *testing.B) {
	for _, c := range cases {
		b.Run(c.name+"/panic", func(b *testing.B) {
			fr := withStack(newFrame())
			b.ReportAllocs()
			for b.Loop() {
				fr.rt.steps = 0
				sink, sinkErr = prun(c.p, fr)
			}
		})
		b.Run(c.name+"/error", func(b *testing.B) {
			fr := withStack(newFrame())
			b.ReportAllocs()
			for b.Loop() {
				fr.rt.steps = 0
				sink, sinkErr = erun(c.e, fr)
			}
		})
		b.Run(c.name+"/error_net", func(b *testing.B) {
			fr := withStack(newFrame())
			b.ReportAllocs()
			for b.Loop() {
				fr.rt.steps = 0
				sink, sinkErr = erunNet(c.e, fr)
			}
		})
	}
}
