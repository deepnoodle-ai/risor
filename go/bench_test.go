package risor_test

// Benchmarks of the workloads of experiments/v3/bench, run through the real
// API: compile once, run many times.
//
//	GOWORK=off go test -run xxx -bench . -benchmem -count 6
//
// The programs are those of the spike written in v3. Two differ from the
// spike's text: v3 has no coalesce function, so "coalesce" uses ??; and
// "any" uses some. fib_22 is left out: a const-bound lambda cannot call
// itself in v3 (01-language.md 5 rule 6).

import (
	"context"
	"fmt"
	"testing"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

type benchCase struct{ name, src string }

var workflowCases = []benchCase{
	{"field", `input.event.pull_request.head.sha`},
	{"compare", `gate.failed > 0`},
	{"membership", `input.role in ["admin", "owner"]`},
	{"coalesce", `report?.status ?? approve?.status ?? "error"`},
	{"any", `!verify.some(v => v.confirmed)`},
	{"filter_map", `fetch.tests.filter(t => t.status == "fail").map(t => t.name)`},
	{"ternary", `if (run.status == "completed") { "Review posted" } else { "Review failed; see the run" }`},
	{"concat", `error.step + " failed: " + error.message`},
}

var heavyCases = []benchCase{
	{"arith_10k", `nums.filter(x => x % 3 == 0).map(x => x * 2 + 1)`},
	{"strings_1k", `names.filter(n => len(n) > 6 && n != "user_500").map(n => n + "@example.com")`},
	{"nested_100x100", `rows.map(r => r.vals.filter(v => v > r.min).map(v => v * r.scale))`},
	{"reduce_10k", `nums.reduce((a, x) => a + x * x, 0)`},
	{"let_chain", "const a = gate.failed + gate.passed;\nconst b = a * 2;\nconst c = b - gate.failed;\nif (c > a) { c - a } else { a - c }"},
}

// workflowData is the scope of experiments/v3/bench env(0).
func workflowData() risor.Map {
	tests := make([]any, 50)
	for i := range tests {
		status := "pass"
		if i%10 == 0 {
			status = "fail"
		}
		tests[i] = map[string]any{"name": fmt.Sprintf("test_%d", i), "status": status, "ms": float64(i * 3)}
	}
	verify := make([]any, 10)
	for i := range verify {
		verify[i] = map[string]any{"finding": fmt.Sprintf("f%d", i), "confirmed": false}
	}
	return risor.Map{
		"input": map[string]any{
			"role": "owner",
			"event": map[string]any{
				"action": "opened",
				"pull_request": map[string]any{
					"number": 42.0, "title": "Fix the thing", "draft": false,
					"head":   map[string]any{"sha": "0123456789abcdef", "ref": "fix"},
					"base":   map[string]any{"sha": "fedcba9876543210", "ref": "main"},
					"labels": []any{"bug", "p1"},
				},
				"repository": map[string]any{"full_name": "org/repo", "private": false},
			},
		},
		"gate":    map[string]any{"failed": 2.0, "passed": 48.0},
		"fetch":   map[string]any{"tests": tests},
		"verify":  verify,
		"report":  map[string]any{"status": "success", "body": "ok", "verdict": "APPROVE"},
		"approve": nil,
		"run":     map[string]any{"status": "completed", "error": nil},
		"error":   map[string]any{"step": "fetch", "step_id": "main/fetch#1", "type": "timeout", "message": "timed out"},
		"scan":    nil,
	}
}

// heavyData is the scope of experiments/v3/bench heavyEnv, with int64
// numbers as the spike's host gave them.
func heavyData() risor.Map {
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
	return risor.Map{
		"nums": nums, "names": names, "rows": rows,
		"gate": map[string]any{"failed": int64(2), "passed": int64(48)},
	}
}

func benchEnv(b *testing.B, data risor.Map) *risor.Env {
	decl := "{"
	for k := range data {
		decl += k + ": any, "
	}
	decl += "}"
	env, err := risor.NewEnv(risor.Declare(decl), risor.Limits{Steps: 100_000_000, Alloc: 100_000_000})
	if err != nil {
		b.Fatal(err)
	}
	return env
}

func benchRun(b *testing.B, cases []benchCase, data risor.Map) {
	env := benchEnv(b, data)
	ctx := context.Background()
	for _, c := range cases {
		x, err := risor.Parse(c.src)
		if err != nil {
			b.Fatal(err)
		}
		p, err := env.Compile(x)
		if err != nil {
			b.Fatal(c.name, err)
		}
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := p.Run(ctx, data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkWorkflow is the eight workflow expressions, small scope.
func BenchmarkWorkflow(b *testing.B) { benchRun(b, workflowCases, workflowData()) }

// BenchmarkHeavy is the compute-heavy programs.
func BenchmarkHeavy(b *testing.B) { benchRun(b, heavyCases, heavyData()) }

// BenchmarkCompile is source text to a runnable program: parse, the static
// rules and the closure compile.
func BenchmarkCompile(b *testing.B) {
	data := workflowData()
	for k, v := range heavyData() {
		data[k] = v
	}
	env := benchEnv(b, data)
	for _, c := range append(append([]benchCase{}, workflowCases...), heavyCases...) {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				x, err := risor.Parse(c.src)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := env.Compile(x); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestConcurrentRuns runs each benchmark program on 16 goroutines at once
// and checks that every run gives the value and usage of a run made alone.
// Run it under -race: the run state is pooled per program.
func TestConcurrentRuns(t *testing.T) {
	data := workflowData()
	for k, v := range heavyData() {
		data[k] = v
	}
	decl := "{"
	for k := range data {
		decl += k + ": any, "
	}
	decl += "}"
	env, err := risor.NewEnv(risor.Declare(decl), risor.Limits{Steps: 100_000_000, Alloc: 100_000_000})
	assert.NoError(t, err)
	ctx := context.Background()
	for _, c := range append(append([]benchCase{}, workflowCases...), heavyCases...) {
		x, err := risor.Parse(c.src)
		assert.NoError(t, err)
		p, err := env.Compile(x)
		assert.NoError(t, err, c.name)
		want, wantUse, err := p.Measure(ctx, data)
		assert.NoError(t, err, c.name)
		wantText := fmt.Sprint(want)
		errs := make(chan error, 16)
		for g := 0; g < 16; g++ {
			go func() {
				for i := 0; i < 20; i++ {
					got, use, err := p.Measure(ctx, data)
					switch {
					case err != nil:
						errs <- err
						return
					case use != wantUse || fmt.Sprint(got) != wantText:
						errs <- fmt.Errorf("%s: run %d differs: %v %v", c.name, i, use, wantUse)
						return
					}
				}
				errs <- nil
			}()
		}
		for g := 0; g < 16; g++ {
			assert.NoError(t, <-errs)
		}
	}
}
