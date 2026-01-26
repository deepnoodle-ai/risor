package risor

import (
	"github.com/risor-io/risor/builtins"
	modMath "github.com/risor-io/risor/modules/math"
	modRand "github.com/risor-io/risor/modules/rand"
	modRegexp "github.com/risor-io/risor/modules/regexp"
	modTime "github.com/risor-io/risor/modules/time"
	"github.com/risor-io/risor/object"
)

// DefaultGlobalsOpts are options for the DefaultGlobals function.
type DefaultGlobalsOpts struct{}

// DefaultGlobals returns a map of standard globals for Risor scripts. This
// includes only the builtins and modules that are always available, without
// pulling in additional Go modules.
func DefaultGlobals(opts ...DefaultGlobalsOpts) map[string]any {
	globals := map[string]any{}

	// Add default builtin functions as globals
	for k, v := range builtins.Builtins() {
		globals[k] = v
	}

	// Add default modules as globals
	modules := map[string]object.Object{
		"math":   modMath.Module(),
		"rand":   modRand.Module(),
		"regexp": modRegexp.Module(),
		"time":   modTime.Module(),
	}
	for k, v := range modules {
		globals[k] = v
	}

	return globals
}
