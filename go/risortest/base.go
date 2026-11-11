package risortest

import (
	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/runner"
)

func runnerBaseNames(base *risor.Env) (vars, funcs func(string) bool) {
	return runner.BaseNames(base)
}
