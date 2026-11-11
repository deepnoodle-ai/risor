// Package hooks holds functions the root package registers for the internal
// packages that need its internals: the REPL (internal/replrun) and the case
// file runner (internal/runner). They are not part of the public surface
// (05-host-api.md E.2, E.9). The package exists to break the import cycle
// between the root package and those packages: it imports neither.
package hooks

import (
	"context"

	"github.com/deepnoodle-ai/risor/go/v3/internal/eval"
	"github.com/deepnoodle-ai/risor/go/v3/internal/syntax"
)

// RunEntry runs expr, a *risor.Expr, as an entry of a REPL session in env, a
// *risor.Env, with the values of the held const lines. It gives the value
// and the values of all of the entry's const lines. Set by the root package.
var RunEntry func(ctx context.Context, env, expr any, vars eval.Vars, held [][]any) (any, [][]any, error)

// Tree gives the syntax tree of expr, a *risor.Expr. Set by the root
// package.
var Tree func(expr any) *syntax.Program
