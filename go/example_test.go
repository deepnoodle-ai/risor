package risor_test

import (
	"context"
	"fmt"

	"github.com/deepnoodle-ai/risor/go/v3"
)

// Example is the program in README.md: declare the host's data, compile once,
// run many times.
func Example() {
	env, err := risor.NewEnv(
		risor.Var("tests", "{name: string, status: string, ms: int}[]"),
	)
	if err != nil {
		panic(err)
	}
	expr, err := risor.Parse(`
const failed = tests.filter(t => t.status == "fail");
{
  verdict: if (!failed) { "pass" } else { "fail" },
  names: failed[*].name,
}`)
	if err != nil {
		panic(err)
	}
	prog, err := env.Compile(expr)
	if err != nil {
		panic(err)
	}
	out, err := prog.Run(context.Background(), risor.Map(map[string]any{
		"tests": []any{
			map[string]any{"name": "login", "status": "pass", "ms": 12},
			map[string]any{"name": "upload", "status": "fail", "ms": 340},
		},
	}))
	if err != nil {
		panic(err)
	}
	fmt.Println(out)
	// Output: map[names:[upload] verdict:fail]
}
