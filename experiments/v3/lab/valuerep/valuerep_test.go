package valuerep

import (
	"math"
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Globals: five ints above 255, so boxing one allocates, three floats, and a
// JSON-shaped scope.
const (
	gA = iota
	gB
	gC
	gD
	gE
	gPrice
	gShip
	gLimit
	gNums
	gInput
	gGate
)

func globals() []any {
	nums := make([]any, 10_000)
	for i := range nums {
		nums[i] = int64(i)
	}
	return []any{
		int64(1200), int64(3400), int64(560), int64(7800), int64(910),
		19.99, 4.5, 20.0,
		nums,
		map[string]any{"event": map[string]any{"pull_request": map[string]any{"head": map[string]any{"sha": "0123456789abcdef"}}}},
		map[string]any{"failed": 2.0, "passed": 48.0},
	}
}

type impl struct {
	name string
	run  func(*frame) any
}

// Each case is the same expression in each representation.
var cases = []struct {
	name  string
	impls []impl
}{
	{
		// a * b + c * d - e
		"arith", []impl{
			{"boxed", arith('-', arith('+', arith('*', glob(gA), glob(gB)), arith('*', glob(gC), glob(gD))), glob(gE))},
			{"tagged", unwrapped(varith('-', varith('+', varith('*', vglob(gA), vglob(gB)), varith('*', vglob(gC), vglob(gD))), vglob(gE)))},
			{"typed", boxInt(iarith('-', iarith('+', iarith('*', iglob(gA), iglob(gB)), iarith('*', iglob(gC), iglob(gD))), iglob(gE)))},
			{"typed_overflow_check", boxInt(iarithWrap('-', iarithWrap('+', iarithWrap('*', iglob(gA), iglob(gB)), iarithWrap('*', iglob(gC), iglob(gD))), iglob(gE)))},
			{"typed_no_check", boxInt(iarithNone('-', iarithNone('+', iarithNone('*', iglob(gA), iglob(gB)), iarithNone('*', iglob(gC), iglob(gD))), iglob(gE)))},
		},
	},
	{
		// a < b && c + d > e
		"compare", []impl{
			{"boxed", and(less(glob(gA), glob(gB)), greater(arith('+', glob(gC), glob(gD)), glob(gE)))},
			{"tagged", unwrapped(vand(vless(vglob(gA), vglob(gB)), vgreater(varith('+', vglob(gC), vglob(gD)), vglob(gE))))},
			{"typed", boxBool(band(iless(iglob(gA), iglob(gB)), igreater(iarith('+', iglob(gC), iglob(gD)), iglob(gE))))},
			{"typed_generic", boxBool(band(lessG(iglob(gA), iglob(gB)), greaterG(iarith('+', iglob(gC), iglob(gD)), iglob(gE))))},
		},
	},
	{
		// price * 1.08 + ship > limit
		"float", []impl{
			{"boxed", greater(arith('+', arith('*', glob(gPrice), konst(1.08)), glob(gShip)), glob(gLimit))},
			{"tagged", unwrapped(vgreater(varith('+', varith('*', vglob(gPrice), vkonst(1.08)), vglob(gShip)), vglob(gLimit)))},
			{"typed", boxBool(fgreater(farith('+', farith('*', fglob(gPrice), fkonst(1.08)), fglob(gShip)), fglob(gLimit)))},
		},
	},
	{
		// gate.failed > 0, where failed is the float64 a JSON decoder made
		"json_compare", []impl{
			{"boxed", greater(field(glob(gGate), "failed"), konst(int64(0)))},
			{"tagged", unwrapped(vgreater(vfield(vglob(gGate), "failed"), vkonst(int64(0))))},
			{"typed", boxBool(igreater(unboxInt(field(glob(gGate), "failed")), ikonst(0)))},
		},
	},
	{
		// input.event.pull_request.head.sha
		"field_chain", []impl{
			{"boxed", field(field(field(field(glob(gInput), "event"), "pull_request"), "head"), "sha")},
			{"tagged", unwrapped(vfield(vfield(vfield(vfield(vglob(gInput), "event"), "pull_request"), "head"), "sha"))},
		},
	},
	{
		// nums.reduce(0, (acc, x) => acc + x * x % 7) over 10,000 ints
		"reduce_10k", []impl{
			{"boxed", reduce(glob(gNums), konst(int64(0)), arith('+', slot(0), arith('%', arith('*', slot(1), slot(1)), konst(int64(7)))))},
			{"tagged", unwrapped(vreduce(vglob(gNums), vkonst(int64(0)), varith('+', vslot(0), varith('%', varith('*', vslot(1), vslot(1)), vkonst(int64(7))))))},
			{"typed_boxed_slots", boxInt(ireduce(glob(gNums), 0, iarith('+', islot(0), iarith('%', iarith('*', islot(1), islot(1)), ikonst(7)))))},
			{"typed_typed_slots", boxInt(ireduceTyped(glob(gNums), 0, iarith('+', islotTyped(0), iarith('%', iarith('*', islotTyped(1), islotTyped(1)), ikonst(7)))))},
		},
	},
	{
		// nums.reduce(0, (acc, x) => acc + x * x): intermediates above 255
		"reduce_big_10k", []impl{
			{"boxed", reduce(glob(gNums), konst(int64(0)), arith('+', slot(0), arith('*', slot(1), slot(1))))},
			{"tagged", unwrapped(vreduce(vglob(gNums), vkonst(int64(0)), varith('+', vslot(0), varith('*', vslot(1), vslot(1)))))},
			{"typed_boxed_slots", boxInt(ireduce(glob(gNums), 0, iarith('+', islot(0), iarith('*', islot(1), islot(1)))))},
			{"typed_typed_slots", boxInt(ireduceTyped(glob(gNums), 0, iarith('+', islotTyped(0), iarith('*', islotTyped(1), islotTyped(1)))))},
			{"typed_typed_slots_overflow_check", boxInt(ireduceTyped(glob(gNums), 0, iarithWrap('+', islotTyped(0), iarithWrap('*', islotTyped(1), islotTyped(1)))))},
			{"typed_typed_slots_no_check", boxInt(ireduceTyped(glob(gNums), 0, iarithNone('+', islotTyped(0), iarithNone('*', islotTyped(1), islotTyped(1)))))},
		},
	},
}

func unwrapped(x vnode) func(*frame) any {
	return func(fr *frame) any { return unwrap(x(fr)) }
}

func newFrame() *frame {
	return &frame{
		rt:     &runtime{globals: globals(), max: 1 << 40},
		slots:  make([]any, 2),
		vslots: make([]Value, 2),
		ints:   make([]int64, 2),
	}
}

func TestAgree(t *testing.T) {
	for _, c := range cases {
		want := c.impls[0].run(newFrame())
		for _, im := range c.impls[1:] {
			assert.Equal(t, im.run(newFrame()), want, "%s/%s", c.name, im.name)
		}
		t.Logf("%-14s %v", c.name, want)
	}
}

var sink any

func TestRange(t *testing.T) {
	assert.Equal(t, addInt(maxSafe-1, 1), int64(maxSafe))
	assert.Panics(t, func() { addInt(maxSafe, 1) })
	assert.Panics(t, func() { subInt(-maxSafe, 1) })
	assert.Equal(t, mulInt(1<<26, 1<<26), int64(1<<52))
	assert.Panics(t, func() { mulInt(1<<27, 1<<26) })
	assert.Panics(t, func() { mulInt(-(1 << 40), 1<<40) })
	assert.Equal(t, mulInt(maxSafe, -1), int64(-maxSafe))
	assert.Panics(t, func() { finite(1e10*1.5e10, 1e10, 1.5e10) })   // two whole numbers
	assert.Equal(t, finite(1e10*1.5e10+0.5, 1e10, 1.25), 1.5e20+0.5) // one is not whole
	assert.Panics(t, func() { finite(math.Inf(1), 1e308, 10) })
	assert.Panics(t, func() { finite(float64(maxSafe)+1, maxSafe, 1) })
	assert.Equal(t, finite(0.5*1e17, 0.5, 1e17), 5e16)
}

func BenchmarkRep(b *testing.B) {
	for _, c := range cases {
		for _, im := range c.impls {
			b.Run(c.name+"/"+im.name, func(b *testing.B) {
				fr := newFrame()
				b.ReportAllocs()
				for b.Loop() {
					fr.rt.steps = 0
					sink = im.run(fr)
				}
			})
		}
	}
}
