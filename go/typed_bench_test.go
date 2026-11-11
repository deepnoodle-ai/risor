package risor_test

// Benchmarks of package F11, the typed layer: the workflow expressions on
// data a host passes as Go structs, declared from their Go types, against
// the same data as plain maps.
//
//	GOWORK=off go test -run xxx -bench 'Typed' -benchmem -count 6

import (
	"context"
	"fmt"
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
)

type tHead struct {
	SHA string `json:"sha"`
	Ref string `json:"ref"`
}

type tPullRequest struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Draft  bool     `json:"draft"`
	Head   tHead    `json:"head"`
	Base   *tHead   `json:"base"`
	Labels []string `json:"labels"`
}

type tEvent struct {
	Action      string       `json:"action"`
	PullRequest tPullRequest `json:"pull_request"`
}

type tInput struct {
	Role  string `json:"role"`
	Event tEvent `json:"event"`
}

type tTest struct {
	Name   string  `json:"name"`
	Status string  `json:"status"`
	MS     float64 `json:"ms"`
}

type tFetch struct {
	Tests []tTest `json:"tests"`
}

type tGate struct {
	Failed int `json:"failed"`
	Passed int `json:"passed"`
}

type tReport struct {
	Status string `json:"status"`
}

func typedStructs() (risor.Map, map[string]risor.Type) {
	tests := make([]tTest, 100)
	for i := range tests {
		st := "pass"
		if i%10 == 0 {
			st = "fail"
		}
		tests[i] = tTest{Name: fmt.Sprintf("test_%d", i), Status: st, MS: float64(i * 3)}
	}
	in := &tInput{Role: "owner", Event: tEvent{Action: "opened", PullRequest: tPullRequest{
		Number: 42, Title: "Fix the thing", Head: tHead{SHA: "0123456789abcdef", Ref: "fix"},
		Base: &tHead{SHA: "fedcba9876543210", Ref: "main"}, Labels: []string{"bug", "p1"}}}}
	data := risor.Map{
		"input":  in,
		"fetch":  &tFetch{Tests: tests},
		"gate":   &tGate{Failed: 2, Passed: 48},
		"report": &tReport{Status: "success"},
	}
	ty := func(t risor.Type, err error) risor.Type {
		if err != nil {
			panic(err)
		}
		return t
	}
	types := map[string]risor.Type{
		"input":  ty(risor.TypeOf[tInput]()),
		"fetch":  ty(risor.TypeOf[tFetch]()),
		"gate":   ty(risor.TypeOf[tGate]()),
		"report": ty(risor.TypeOf[tReport]()),
	}
	return data, types
}

func typedMaps() risor.Map {
	tests := make([]any, 100)
	for i := range tests {
		st := "pass"
		if i%10 == 0 {
			st = "fail"
		}
		tests[i] = map[string]any{"name": fmt.Sprintf("test_%d", i), "status": st, "ms": float64(i * 3)}
	}
	return risor.Map{
		"input": map[string]any{"role": "owner", "event": map[string]any{"action": "opened",
			"pull_request": map[string]any{"number": 42.0, "title": "Fix the thing", "draft": false,
				"head":   map[string]any{"sha": "0123456789abcdef", "ref": "fix"},
				"base":   map[string]any{"sha": "fedcba9876543210", "ref": "main"},
				"labels": []any{"bug", "p1"}}}},
		"fetch":  map[string]any{"tests": tests},
		"gate":   map[string]any{"failed": 2.0, "passed": 48.0},
		"report": map[string]any{"status": "success"},
	}
}

var typedCases = []benchCase{
	{"field4", `input.event.pull_request.head.sha`},
	{"field3", `input.event.action`},
	{"optional_ptr", `input.event.pull_request.base?.ref ?? "none"`},
	{"compare", `gate.failed > 0`},
	{"membership", `input.role in ["admin", "owner"]`},
	{"filter_map_100", `fetch.tests.filter(t => t.status == "fail").map(t => t.name)`},
	{"star_100", `fetch.tests[*].name`},
	{"some_100", `fetch.tests.some(t => t.ms > 1000)`},
	{"map_100", `fetch.tests.map(t => t.name)`},
	{"count_100", `fetch.tests.count(t => t.status == "fail")`},
	{"find_100", `fetch.tests.find(t => t.name == "test_50")?.ms`},
	{"reduce_100", `fetch.tests.reduce((a, t) => a + t.ms, 0)`},
	{"sortBy_100", `fetch.tests.sortBy(t => t.ms, "desc")`},
}

func benchTyped(b *testing.B, data risor.Map, decls []risor.Decl, opts ...risor.CheckOption) {
	env, err := risor.NewEnv(append(decls, risor.Limits{Steps: 100_000_000, Alloc: 100_000_000})...)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	for _, c := range typedCases {
		x, err := risor.Parse(c.src)
		if err != nil {
			b.Fatal(err)
		}
		p, err := env.Compile(x, opts...)
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

// BenchmarkTypedMaps is the expressions on plain maps, declared by type
// text.
func BenchmarkTypedMaps(b *testing.B) {
	_, types := typedStructs()
	var decls []risor.Decl
	for k, t := range types {
		decls = append(decls, risor.VarType(k, t))
	}
	benchTyped(b, typedMaps(), decls)
}

// BenchmarkTypedStructs is the expressions on Go structs declared with
// TypeOf: the typed layer applies.
func BenchmarkTypedStructs(b *testing.B) {
	data, types := typedStructs()
	var decls []risor.Decl
	for k, t := range types {
		decls = append(decls, risor.VarType(k, t))
	}
	benchTyped(b, data, decls)
}

// BenchmarkTypedStructsAny is the same structs declared any: the general
// path.
func BenchmarkTypedStructsAny(b *testing.B) {
	data, _ := typedStructs()
	var decls []risor.Decl
	for k := range data {
		decls = append(decls, risor.Var(k, "any"))
	}
	benchTyped(b, data, decls)
}

// BenchmarkTypedStructsOff is the structs declared with TypeOf and the
// typed layer off: the general path, as before package F11.
func BenchmarkTypedStructsOff(b *testing.B) {
	data, types := typedStructs()
	decls := []risor.Decl{risor.TypedLayer(false)}
	for k, t := range types {
		decls = append(decls, risor.VarType(k, t))
	}
	benchTyped(b, data, decls)
}
