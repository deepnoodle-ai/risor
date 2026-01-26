package risor

import (
	"github.com/risor-io/risor/bytecode"
)

// ProgramStats contains statistics about compiled bytecode.
// This is useful for auditing scripts before execution.
type ProgramStats = bytecode.Stats
