package eval

import (
	"github.com/deepnoodle-ai/risor/go/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/go/v3/internal/value"
)

// This file is the library entry of package F17, recursion
// (spec/packages/f17-recursion.md 5): walk.

func init() {
	implemented["walk"] = walk
	// The table of entry information was made before init: refresh the
	// entries this file implements.
	for e, in := range infos {
		if f, ok := implemented[Key(e)]; ok {
			in.impl = f
		}
	}
}

// walkLevel is one level of a walk's own stack: a list of children and the
// position of the next child to visit (5.3).
type walkLevel struct {
	list   any
	pos, n int
}

// walk is 5.2: the nodes in pre-order. The place of the walk is kept in an
// explicit stack, one level for each depth, so a deep tree costs no native
// stack (5.3).
func walk(r *run, c *libCall) any {
	lam := c.args[1].(*lambdaCode)
	out := []any{}
	var stack []walkLevel
	// visit charges a node's 2 steps, makes the depth test, calls children
	// with the node and its depth, and tests the result (5.2 items 3 and 4,
	// 5.4). A child is read after its steps (5.2 item 5).
	visit := func(node any, d int, child bool) {
		r.step(2)
		if child {
			node = r.read(node, c.span)
		}
		c.idx = len(out)
		out = append(out, node)
		m := c.measure + d
		r.enterCall(m)
		if lam.nparams > 0 {
			lam.bind(r, 0, node, true)
		}
		if lam.nparams > 1 {
			lam.bind(r, 1, numBox(float64(d)), true)
			for k := 2; k < lam.nparams; k++ {
				lam.bind(r, k, nil, false)
			}
		}
		v := lam.body(r)
		r.depth -= m
		switch value.KindOf(v) {
		case value.Null:
		case value.List:
			stack = append(stack, walkLevel{list: v, n: value.Len(v)})
		default:
			r.raise(fail("wrong-kind", "result", c.span, messages.Word("name", c.e.Name), messages.Val("kind", v)))
		}
	}
	visit(c.args[0], 0, false)
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.pos == top.n {
			stack = stack[:len(stack)-1]
			continue
		}
		node := value.At(top.list, top.pos)
		top.pos++
		// The children on level i of the stack have depth i + 1.
		visit(node, len(stack), true)
	}
	f := fenv{c: c, k: int64(len(out))}
	r.libSteps(c, f, 2*int64(len(out)))
	r.libUnits(c, f)
	return out
}
