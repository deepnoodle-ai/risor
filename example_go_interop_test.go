package risor_test

import (
	"context"
	"fmt"
	"time"

	"github.com/deepnoodle-ai/risor/v2"
)

// Script values pass to Go functions: null becomes Go nil, a number converts
// to a defined type such as time.Duration, and a script function sent to an
// any parameter is a type error.
func Example_goInterop() {
	env := map[string]any{
		"describe": func(v any) string {
			if v == nil {
				return "Go nil"
			}
			return fmt.Sprintf("%#v", v)
		},
		"wait":  func(d time.Duration) string { return d.String() },
		"count": func(m map[string]any) int { return len(m) },
	}
	ctx := context.Background()
	for _, script := range []string{
		`describe(null)`,
		`count({a: null})`,
		`wait(1500000000)`,
		`describe(function() {})`,
	} {
		result, err := risor.Eval(ctx, script, risor.WithEnv(env))
		fmt.Println(result, err)
	}
	// Output:
	// Go nil <nil>
	// 1 <nil>
	// 1.5s <nil>
	// <nil> func(interface {}) string: argument 1: type error: cannot convert function to a Go value
}
