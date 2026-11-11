// Package replrun runs the entries of a REPL session (package F12 5 item 6) for
// cmd/risor. It is not part of the public surface (05-host-api.md E.9): the
// root package registers the run of an entry here, since it needs the
// internals of an environment and of a compiled program (internal/hooks).
package replrun

import (
	"context"

	"github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/risor/go/v3/internal/hooks"
)

// Held is the values of the const lines a REPL session holds. The zero Held
// holds no line.
type Held struct{ vals [][]any }

// Lines is the number of const lines held.
func (h Held) Lines() int { return len(h.vals) }

// RunEntry checks, compiles and runs x as an entry of a REPL session that
// holds h. The first h.Lines() const lines of x must be the held lines,
// unchanged: they take the values h keeps, and are not run, so the usage is
// the entry's own. On success it gives the value and the values of all of
// x's const lines, to hold for the next entry.
func RunEntry(ctx context.Context, env *risor.Env, x *risor.Expr, vars risor.Vars, h Held) (any, Held, error) {
	v, held, err := hooks.RunEntry(ctx, env, x, vars, h.vals)
	if err != nil {
		return nil, Held{}, err
	}
	return v, Held{held}, nil
}
