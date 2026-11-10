package check

import (
	"github.com/deepnoodle-ai/risor/v3/internal/static"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/types"
)

// Types is what the assist stage of package F12 reads of a check
// (f12-tools.md 3.1 item 2, 3.12): the type of each node the check reached,
// at the first check that reached it, and the type each slot was first
// bound to. A lambda checked more than once, as a const lambda is at each
// use, so takes its first check.
type Types struct {
	Nodes map[syntax.Node]*types.Type
	Slots []*types.Type
}

// CheckTypes is Check, and gives the types of the nodes and of the slots
// beside the result. It takes the same path as Check, and also records the
// first type of each node and slot, which an ordinary check does not.
func CheckTypes(prog *syntax.Program, res *static.Result, env *Env, exp Expect) (*Result, *Types) {
	var c checker
	t := c.run(prog, res, env, exp, true)
	slots := make([]*types.Type, len(c.slots))
	for i, s := range c.firstSlots {
		if s == nil {
			s = c.slots[i]
		}
		slots[i] = s
	}
	return c.result(prog, t), &Types{Nodes: c.firstTy, Slots: slots}
}
