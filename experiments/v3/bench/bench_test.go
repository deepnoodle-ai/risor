// Package bench compares expr and Risor on expressions typical of workflow
// definitions: compile once at save, evaluate on every Apply.
//
//	GOWORK=off go test ./bench -run xxx -bench . -benchmem -count 6
package bench

import (
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/deepnoodle-ai/expr"
	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/wonton/assert"
)

// A case is one workflow expression written in each language.
type benchCase struct {
	name  string
	expr  string
	risor string
}

var cases = []benchCase{
	{"field", `input.event.pull_request.head.sha`, `input.event.pull_request.head.sha`},
	{"compare", `gate.failed > 0`, `gate.failed > 0`},
	{"membership", `contains(["admin", "owner"], input.role)`, `input.role in ["admin", "owner"]`},
	{"coalesce", `coalesce(report?.status, approve?.status, "error")`, `coalesce(report?.status, approve?.status, "error")`},
	{"any", `!any(verify, it.confirmed)`, `!any(verify.map(v => v.confirmed))`},
	{
		"filter_map", `fetch.tests | filter(it.status == "fail") | map(it.name)`,
		`fetch.tests.filter(t => t.status == "fail").map(t => t.name)`,
	},
	{
		"ternary", `if(run.status == "completed", "Review posted", "Review failed; see the run")`,
		`if (run.status == "completed") { "Review posted" } else { "Review failed; see the run" }`,
	},
	{"concat", `error.step + " failed: " + error.message`, `error.step + " failed: " + error.message`},
}

// env is a run's scope as Apply binds it: input, every step by name, and
// the globals. The values are JSON-shaped, as the engine's are.
func env(extra int) map[string]any {
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
	e := map[string]any{
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
				"repository": map[string]any{"full_name": "deepnoodle-ai/risor", "private": false},
			},
		},
		"gate":    map[string]any{"failed": 2.0, "passed": 48.0},
		"fetch":   map[string]any{"tests": tests},
		"verify":  verify,
		"report":  map[string]any{"status": "success", "body": "ok", "verdict": "APPROVE"},
		"approve": nil,
		"run":     map[string]any{"status": "completed", "error": nil},
		"error":   map[string]any{"step": "fetch", "step_id": "main/fetch#1", "type": "timeout", "message": "timed out"},
	}
	// A step whose output no expression reads, to show what an evaluation
	// pays for the rest of the run's state.
	if extra > 0 {
		rows := make([]any, extra)
		for i := range rows {
			rows[i] = map[string]any{"id": float64(i), "path": fmt.Sprintf("src/file_%d.go", i), "line": float64(i)}
		}
		e["scan"] = map[string]any{"rows": rows}
	} else {
		e["scan"] = nil
	}
	return e
}

// --- expr, configured as workflow@v0.1.0/expr.go configures it.

var exprOptions = func() []expr.Option {
	all := map[string]any{}
	for _, g := range []map[string]any{expr.Builtins(), expr.StringFuncs(), expr.MathFuncs(), expr.CollectionFuncs()} {
		maps.Copy(all, g)
	}
	all["coalesce"] = expr.Func(func(_ context.Context, args []any) (any, error) {
		for _, a := range args {
			if a != nil {
				return a, nil
			}
		}
		return nil, nil
	})
	return []expr.Option{expr.WithFunctions(all), expr.WithEvalBudget(1_000_000)}
}()

// --- Risor: one expression, arrow functions allowed, no rand module.

var risorSyntax = func() risor.SyntaxConfig {
	c := risor.ExpressionOnly
	c.DisallowFuncDef = false // arrow functions, for filter and map
	c.DisallowReturn = false  // an arrow body compiles to a return
	c.DisallowIf = false
	return c
}()

func risorEnv(data map[string]any) map[string]any {
	e := risor.Builtins()
	delete(e, "rand")
	delete(e, "error") // the engine binds error itself
	maps.Copy(e, data)
	return e
}

// preconverted is risorEnv with the data converted to Risor objects once,
// as a host that caches each step's converted output would pass it.
func preconverted(data map[string]any) map[string]any {
	e := risorEnv(nil)
	objs, err := object.AsObjects(data)
	if err != nil {
		panic(err)
	}
	for k, v := range objs {
		e[k] = v
	}
	return e
}

func risorOpts(e map[string]any) []risor.Option {
	return []risor.Option{risor.WithEnv(e), risor.WithSyntax(risorSyntax), risor.WithMaxSteps(1_000_000)}
}

// TestAgree checks that both languages give the same value for every case,
// so the benchmarks compare like with like.
func TestAgree(t *testing.T) {
	ctx := context.Background()
	data := env(0)
	for _, c := range cases {
		p, err := expr.Compile(c.expr, exprOptions...)
		assert.NoError(t, err, "%s: expr compile", c.name)
		want, err := p.Run(ctx, data)
		assert.NoError(t, err, "%s: expr run", c.name)
		for _, e := range []map[string]any{risorEnv(data), preconverted(data)} {
			code, err := risor.Compile(ctx, c.risor, risorOpts(e)...)
			assert.NoError(t, err, "%s: risor compile", c.name)
			got, err := risor.Run(ctx, code, risorOpts(e)...)
			assert.NoError(t, err, "%s: risor run", c.name)
			assert.Equal(t, got, want, "%s: risor against expr", c.name)
		}
		t.Logf("%-11s %v", c.name, want)
	}
}

func BenchmarkCompile(b *testing.B) {
	ctx := context.Background()
	e := risorEnv(env(0))
	opts := risorOpts(e)
	for _, c := range cases {
		b.Run(c.name+"/expr", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := expr.Compile(c.expr, exprOptions...); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(c.name+"/risor", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := risor.Compile(ctx, c.risor, opts...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkRun evaluates each compiled expression against a run's scope.
// "risor" passes Go values, which Risor converts on every run. "risor_pre"
// passes values converted once.
func BenchmarkRun(b *testing.B) {
	ctx := context.Background()
	for _, size := range []int{0, 2000} {
		data := env(size)
		plain, pre := risorEnv(data), preconverted(data)
		for _, c := range cases {
			p, err := expr.Compile(c.expr, exprOptions...)
			if err != nil {
				b.Fatal(err)
			}
			code, err := risor.Compile(ctx, c.risor, risorOpts(plain)...)
			if err != nil {
				b.Fatal(err)
			}
			name := fmt.Sprintf("%s/extra=%d", c.name, size)
			b.Run(name+"/expr", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := p.Run(ctx, data); err != nil {
						b.Fatal(err)
					}
				}
			})
			for _, v := range []struct {
				name string
				env  map[string]any
			}{{"risor", plain}, {"risor_pre", pre}} {
				opts := risorOpts(v.env)
				b.Run(name+"/"+v.name, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						if _, err := risor.Run(ctx, code, opts...); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}
