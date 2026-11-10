package check

import (
	"github.com/deepnoodle-ai/risor/v3/internal/static"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
)

// Types is what the assist stage of package F12 reads of a check
// (f12-tools.md 3.1 item 2): the type of each node the check reached, at
// its last check, and the type each slot was last bound to.
type Types struct {
	Nodes map[syntax.Node]*types.Type
	Slots []*types.Type
}

// CheckTypes is Check, and gives the types of the nodes and of the slots
// beside the result. It takes the same path as Check: the checker keeps
// both tables for its own rules (a fit reads the types of a value's parts,
// and a tested union reads its subject's type), so an ordinary check
// records nothing more, and CheckTypes only hands them out.
func CheckTypes(prog *syntax.Program, res *static.Result, env *Env, exp Expect) (*Result, *Types) {
	var c checker
	t := c.run(prog, res, env, exp)
	return c.result(prog, t), &Types{Nodes: c.ty, Slots: c.slots}
}
