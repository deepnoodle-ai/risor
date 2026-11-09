package bench

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/deepnoodle-ai/expr"
	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/vm"
	"github.com/deepnoodle-ai/wonton/assert"

	"github.com/deepnoodle-ai/risor/v2/experiments/v3/spike"
)

// Does Risor's bytecode VM help? These benchmarks run the same Risor source
// through the VM and through spike, a closure compiler over the same AST
// that reads host values in place.
//
//	GOWORK=off go test ./bench -run TestSpike -v
//	GOWORK=off go test ./bench -run xxx -bench 'Spike' -benchmem -benchtime 400ms -count 6

const bigBudget = 100_000_000

// heavy are programs chosen to favor a VM: many operations per evaluation,
// little data crossing the host boundary relative to the work done.
var heavy = []benchCase{
	{
		"arith_10k",
		`nums | filter(it % 3 == 0) | map(it * 2 + 1)`,
		`nums.filter(x => x % 3 == 0).map(x => x * 2 + 1)`,
	},
	{
		"strings_1k",
		`names | filter(len(it) > 6 && it != "user_500") | map(it + "@example.com")`,
		`names.filter(n => len(n) > 6 && n != "user_500").map(n => n + "@example.com")`,
	},
	{
		"nested_100x100",
		`map(rows, r, map(filter(r.vals, it > r.min), it * r.scale))`,
		`rows.map(r => r.vals.filter(v => v > r.min).map(v => v * r.scale))`,
	},
	{"reduce_10k", ``, `nums.reduce(0, (a, x) => a + x * x)`},
	{"fib_22", ``, "function fib(n) {\n  if (n < 2) { return n }\n  return fib(n - 1) + fib(n - 2)\n}\nfib(22)"},
	{"let_chain", ``, "let a = gate.failed + gate.passed\nlet b = a * 2\nlet c = b - gate.failed\nif (c > a) { c - a } else { a - c }"},
}

func heavyEnv() map[string]any {
	nums := make([]any, 10_000)
	for i := range nums {
		nums[i] = int64(i)
	}
	names := make([]any, 1_000)
	for i := range names {
		names[i] = fmt.Sprintf("user_%d", i)
	}
	rows := make([]any, 100)
	for i := range rows {
		vals := make([]any, 100)
		for j := range vals {
			vals[j] = int64(j)
		}
		rows[i] = map[string]any{"min": int64(i), "scale": int64(3), "vals": vals}
	}
	return map[string]any{
		"nums": nums, "names": names, "rows": rows,
		"gate": map[string]any{"failed": int64(2), "passed": int64(48)},
	}
}

// normalize maps every number to float64 so results compare across
// implementations that differ only in int versus float.
func normalize(v any) any {
	switch v := v.(type) {
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalize(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = normalize(item)
		}
		return out
	}
	return v
}

// impl is one way to evaluate a case. raw, when set, is the same without
// converting the result back to Go.
type impl struct {
	name string
	eval func() (any, error)
}

func risorFull(e map[string]any, steps int64, syntax *risor.SyntaxConfig) []risor.Option {
	opts := []risor.Option{risor.WithEnv(e), risor.WithMaxSteps(steps)}
	if syntax != nil {
		opts = append(opts, risor.WithSyntax(*syntax))
	}
	return opts
}

// impls builds every implementation of a case over data.
func impls(tb testing.TB, c benchCase, data map[string]any, steps int64, syntax *risor.SyntaxConfig) []impl {
	ctx := context.Background()
	var out []impl
	if c.expr != "" {
		opts := append([]expr.Option{}, exprOptions...)
		opts = append(opts, expr.WithEvalBudget(int(steps)))
		p, err := expr.Compile(c.expr, opts...)
		if err != nil {
			tb.Fatalf("%s: expr compile: %v", c.name, err)
		}
		out = append(out, impl{"expr", func() (any, error) { return p.Run(ctx, data) }})
	}

	plain, pre := risorEnv(data), preconverted(data)
	code, err := risor.Compile(ctx, c.risor, risorFull(plain, steps, syntax)...)
	if err != nil {
		tb.Fatalf("%s: risor compile: %v", c.name, err)
	}
	plainOpts := risorFull(plain, steps, syntax)
	out = append(out, impl{"risor", func() (any, error) { return risor.Run(ctx, code, plainOpts...) }})

	machine, err := vm.New(code, vm.WithGlobals(pre), vm.WithMaxSteps(steps))
	if err != nil {
		tb.Fatalf("%s: vm: %v", c.name, err)
	}
	out = append(out, impl{"risor_vm", func() (any, error) {
		if err := machine.RunCode(ctx, code, vm.WithGlobals(pre)); err != nil {
			return nil, err
		}
		result, _ := machine.TOS()
		return object.ToInterface(ctx, result)
	}})
	// The floor for the VM: its own machine, globals bound once and never
	// rebound, and the result left as a Risor object. A host cannot use it
	// this way, because the data changes between evaluations, but it shows
	// what executing the bytecode costs with everything else removed.
	floor, err := vm.New(code, vm.WithGlobals(pre), vm.WithMaxSteps(steps))
	if err != nil {
		tb.Fatalf("%s: vm: %v", c.name, err)
	}
	out = append(out, impl{"risor_vm_floor", func() (any, error) {
		if err := floor.RunCode(ctx, code); err != nil {
			return nil, err
		}
		result, _ := floor.TOS()
		return result, nil
	}})

	prog, err := spike.Compile(ctx, c.risor)
	if err != nil {
		tb.Fatalf("%s: spike compile: %v", c.name, err)
	}
	out = append(out, impl{"spike", func() (any, error) { return prog.Run(data, steps) }})
	return out
}

func checkAgree(t *testing.T, list []benchCase, data map[string]any, steps int64, syntax *risor.SyntaxConfig) {
	for _, c := range list {
		var want any
		var from string
		for _, im := range impls(t, c, data, steps, syntax) {
			if im.name == "risor_vm_floor" {
				continue // a Risor object, not a Go value
			}
			got, err := im.eval()
			assert.NoError(t, err, "%s/%s", c.name, im.name)
			got = normalize(got)
			if from == "" {
				want, from = got, im.name
				continue
			}
			assert.True(t, reflect.DeepEqual(got, want), "%s: %s and %s disagree", c.name, im.name, from)
		}
		t.Logf("%-15s %.80s", c.name, fmt.Sprint(want))
	}
}

func TestSpikeAgree(t *testing.T) {
	checkAgree(t, cases, env(0), 1_000_000, &risorSyntax)
	checkAgree(t, heavy, heavyEnv(), bigBudget, nil)
}

// TestSpikeSemantics checks the behavior the proposal asks of a v3: a dot
// reads a field even when a method has that name, a keyword-named field is
// out of the parser's reach, errors carry positions, and the budget bounds
// builtin-driven iteration.
func TestSpikeSemantics(t *testing.T) {
	ctx := context.Background()
	data := map[string]any{
		"m":    map[string]any{"keys": "k", "get": "g", "items": int64(3)},
		"nums": heavyEnv()["nums"],
	}
	for _, tc := range []struct{ src, want string }{
		{`m.keys + m.get`, `kg`},
		{`m.items + 1`, `4`},
		{`m.missing`, `1:3: no field "missing"`},
		{`m?.missing ?? "d"`, `d`},
		{`1 + "a"`, `1:3: unsupported operation: int + string`},
		{`nums.map(x => x + 1).filter(x => x > 5)`, `step budget exceeded`},
	} {
		prog, err := spike.Compile(ctx, tc.src)
		assert.NoError(t, err, "%s: compile", tc.src)
		got, err := prog.Run(data, 5_000)
		text := fmt.Sprint(got)
		if err != nil {
			text = err.Error()
		}
		assert.Equal(t, text, tc.want, "%s", tc.src)
	}
}

func benchImpls(b *testing.B, list []benchCase, data map[string]any, steps int64, syntax *risor.SyntaxConfig) {
	for _, c := range list {
		for _, im := range impls(b, c, data, steps, syntax) {
			b.Run(c.name+"/"+im.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := im.eval(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkSpikeWorkflow is the eight workflow expressions, small scope.
func BenchmarkSpikeWorkflow(b *testing.B) {
	benchImpls(b, cases, env(0), 1_000_000, &risorSyntax)
}

// BenchmarkSpikeHeavy is the compute-heavy programs.
func BenchmarkSpikeHeavy(b *testing.B) {
	benchImpls(b, heavy, heavyEnv(), bigBudget, nil)
}

// BenchmarkSpikeCompile is source text to a runnable program.
func BenchmarkSpikeCompile(b *testing.B) {
	ctx := context.Background()
	all := append(append([]benchCase{}, cases...), heavy...)
	data := env(0)
	for k, v := range heavyEnv() {
		data[k] = v
	}
	plain := risorEnv(data)
	for _, c := range all {
		if c.expr != "" {
			b.Run(c.name+"/expr", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := expr.Compile(c.expr, exprOptions...); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
		opts := risorFull(plain, bigBudget, nil)
		b.Run(c.name+"/risor", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := risor.Compile(ctx, c.risor, opts...); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(c.name+"/spike", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := spike.Compile(ctx, c.risor); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
