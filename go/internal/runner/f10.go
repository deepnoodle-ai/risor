package runner

import (
	"fmt"

	risor "github.com/deepnoodle-ai/risor/v3"
	"github.com/deepnoodle-ai/risor/v3/internal/casefile"
	"github.com/deepnoodle-ai/risor/v3/internal/diag"
)

// The additions of package F10 to the runner.

func schemaOpts(names []string) []risor.SchemaOption {
	out := make([]risor.SchemaOption, len(names))
	for i, n := range names {
		out[i] = risor.SchemaOption(n)
	}
	return out
}

// resultType is the expected type of a case, resolved in its environment:
// its returns as type text, or its returnsSchema (F10 2.1).
func resultType(env *risor.Env, c *casefile.Case) (risor.Type, error) {
	if rs := c.ReturnsSchema; rs != nil {
		for _, o := range rs.Options {
			if o != string(risor.Closed) {
				return risor.Type{}, &risor.Error{Diagnostics: []risor.Diagnostic{{Phase: diag.PhaseDeclare,
					Code: diag.CodeBadType, Message: fmt.Sprintf("the option %s does not apply to an expected result", o)}}}
			}
		}
		return risor.TypeFromSchema(rs.Schema, schemaOpts(rs.Options)...)
	}
	return env.ParseType(*c.Returns)
}
