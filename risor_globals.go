package risor

import (
	"github.com/risor-io/risor/builtins"
	modBase64 "github.com/risor-io/risor/modules/base64"
	modBytes "github.com/risor-io/risor/modules/bytes"
	modDns "github.com/risor-io/risor/modules/dns"
	modErrors "github.com/risor-io/risor/modules/errors"
	modExec "github.com/risor-io/risor/modules/exec"
	modFilepath "github.com/risor-io/risor/modules/filepath"
	modFmt "github.com/risor-io/risor/modules/fmt"
	modHTTP "github.com/risor-io/risor/modules/http"
	modJSON "github.com/risor-io/risor/modules/json"
	modMath "github.com/risor-io/risor/modules/math"
	modNet "github.com/risor-io/risor/modules/net"
	modOs "github.com/risor-io/risor/modules/os"
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
		modHTTP.Builtins(),
		modFmt.Builtins(),
		modOs.Builtins(),
		modDns.Builtins(),
	}
	for _, builtins := range moduleBuiltins {
		for k, v := range builtins {
			globals[k] = v
		}
	}

	// Add default modules as globals
	modules := map[string]object.Object{
		"base64":   modBase64.Module(),
		"bytes":    modBytes.Module(),
		"errors":   modErrors.Module(),
		"exec":     modExec.Module(),
		"filepath": modFilepath.Module(),
		"fmt":      modFmt.Module(),
		"http":     modHTTP.Module(),
		"json":     modJSON.Module(),
		"math":     modMath.Module(),
		"net":      modNet.Module(),
		"os":       modOs.Module(),
		"rand":     modRand.Module(),
		"regexp":   modRegexp.Module(),
		"strconv":  modStrconv.Module(),
		"strings":  modStrings.Module(),
		"time":     modTime.Module(),
	}
	for k, v := range modules {
		globals[k] = v
	}

	return globals
}
