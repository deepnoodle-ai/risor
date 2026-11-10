package risor_test

// BenchmarkSuite runs the shared benchmark programs of suite/bench, the same
// set the TypeScript and Rust harnesses run, through the host API only.
//
//	GOWORK=off go test -run xxx -bench BenchmarkSuite -benchmem
//
// For each program it first checks the value, steps and units against the
// file's expect, then measures, as the other two harnesses do:
//
//   - compile_ns: Parse and Compile (with the check) of the source text, in
//     an environment built once;
//   - run_ns: one run of the compiled program on the file's vars.
//
// Each is the median of 15 samples, after a warm-up of 200 ms; a sample is a
// batch of calls that takes at least 10 ms, divided by its size. It prints
// one JSON line for each program (impl, program, compile_ns, run_ns, steps,
// allocated). The sub-benchmark's own ns/op is that of b.Loop over runs.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	risor "github.com/deepnoodle-ai/risor/v3"
)

type suiteBenchFile struct {
	Env    string `json:"env"`
	Limits struct {
		Steps      int64 `json:"steps"`
		Allocation int64 `json:"allocation"`
	} `json:"limits"`
	Vars  map[string]any `json:"vars"`
	Cases []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
		Expect struct {
			Value     any   `json:"value"`
			Steps     int64 `json:"steps"`
			Allocated int64 `json:"allocated"`
		} `json:"expect"`
	} `json:"cases"`
}

// suiteBenchLine is the line printed for each program.
type suiteBenchLine struct {
	Impl      string  `json:"impl"`
	Program   string  `json:"program"`
	CompileNs float64 `json:"compile_ns"`
	RunNs     float64 `json:"run_ns"`
	Steps     int64   `json:"steps"`
	Allocated int64   `json:"allocated"`
}

// suiteMedian is the median time of one call of fn, in ns.
func suiteMedian(fn func()) float64 {
	const warm, minSample, samples = 200 * time.Millisecond, 10 * time.Millisecond, 15
	start := time.Now()
	for calls := 0; calls < 10 || time.Since(start) < warm; calls++ {
		fn()
	}
	batch := func(n int) time.Duration {
		t := time.Now()
		for i := 0; i < n; i++ {
			fn()
		}
		return time.Since(t)
	}
	n := 1
	for batch(n) < minSample {
		n *= 2
	}
	per := make([]float64, samples)
	for i := range per {
		per[i] = float64(batch(n).Nanoseconds()) / float64(n)
	}
	sort.Float64s(per)
	return per[samples/2]
}

func BenchmarkSuite(b *testing.B) {
	dir := filepath.Join("..", "suite", "bench")
	names, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(names) == 0 {
		b.Fatalf("no files in %s: %v", dir, err)
	}
	ctx := context.Background()
	for _, path := range names {
		text, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		var f suiteBenchFile
		if err := json.Unmarshal(text, &f); err != nil {
			b.Fatal(path, err)
		}
		env, err := risor.NewEnv(risor.Declare(f.Env), risor.Limits{Steps: f.Limits.Steps, Alloc: f.Limits.Allocation})
		if err != nil {
			b.Fatal(path, err)
		}
		vars := risor.Map(f.Vars)
		file := strings.TrimSuffix(filepath.Base(path), ".json")
		for _, c := range f.Cases {
			name := file + "/" + c.Name
			b.Run(name, func(b *testing.B) {
				compile := func() *risor.Program {
					x, err := risor.Parse(c.Source)
					if err != nil {
						b.Fatal(err)
					}
					p, err := env.Compile(x)
					if err != nil {
						b.Fatal(err)
					}
					return p
				}
				p := compile()
				v, use, err := p.Measure(ctx, vars)
				if err != nil {
					b.Fatal(err)
				}
				got, err := risor.JSONText(v)
				if err != nil {
					b.Fatal(err)
				}
				var gotValue any
				if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
					b.Fatal(err)
				}
				if !reflect.DeepEqual(gotValue, c.Expect.Value) {
					b.Fatalf("value differs from expect: %.200s", got)
				}
				if use.Steps != c.Expect.Steps || use.Alloc != c.Expect.Allocated {
					b.Fatalf("usage %+v, expect steps %d allocated %d", use, c.Expect.Steps, c.Expect.Allocated)
				}
				compileNs := suiteMedian(func() { compile() })
				runNs := suiteMedian(func() {
					if _, err := p.Run(ctx, vars); err != nil {
						b.Fatal(err)
					}
				})
				line, _ := json.Marshal(suiteBenchLine{"go", name, compileNs, runNs, use.Steps, use.Alloc})
				fmt.Println(string(line))
				b.ReportAllocs()
				for b.Loop() {
					if _, err := p.Run(ctx, vars); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(compileNs, "compile-ns")
				b.ReportMetric(runNs, "median-run-ns")
				b.ReportMetric(float64(use.Steps), "steps")
			})
		}
	}
}
