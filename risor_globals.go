package risor

import (
	"github.com/risor-io/risor/builtins"
	modBase64 "github.com/risor-io/risor/modules/base64"
	modBytes "github.com/risor-io/risor/modules/bytes"
	modFmt "github.com/risor-io/risor/modules/fmt"
	modJSON "github.com/risor-io/risor/modules/json"
	modMath "github.com/risor-io/risor/modules/math"
	modRand "github.com/risor-io/risor/modules/rand"
	modRegexp "github.com/risor-io/risor/modules/regexp"
	modStrconv "github.com/risor-io/risor/modules/strconv"
	modStrings "github.com/risor-io/risor/modules/strings"
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
	moduleBuiltins := []map[string]object.Object{
		builtins.Builtins(),
		modFmt.Builtins(),
	}
	for _, builtins := range moduleBuiltins {
		for k, v := range builtins {
			globals[k] = v
		}
	}

	// Add default modules as globals
	modules := map[string]object.Object{
		"base64":  modBase64.Module(),
		"bytes":   modBytes.Module(),
		"fmt":     modFmt.Module(),
		"json":    modJSON.Module(),
		"math":    modMath.Module(),
		"rand":    modRand.Module(),
		"regexp":  modRegexp.Module(),
		"strconv": modStrconv.Module(),
		"strings": modStrings.Module(),
		"time":    modTime.Module(),
	}
	for k, v := range modules {
		globals[k] = v
	}

	return globals
}
