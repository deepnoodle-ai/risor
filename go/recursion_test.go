package risor_test

import (
	"testing"

	risor "github.com/deepnoodle-ai/risor/go/v3"
	"github.com/deepnoodle-ai/wonton/assert"
)

// Package F17, step E0 (spec/packages/f17-recursion.md): what the suite does
// not reach. The suite cannot hold a Go struct or a value that holds itself.

type treeNode struct {
	Name string     `json:"name"`
	Kids []treeNode `json:"kids"`
}

func TestWalkGoStructs(t *testing.T) {
	// Nodes read in place from Go structs (5.2 item 5).
	env, err := risor.NewEnv(risor.VarRecord("{t: any}"))
	assert.NoError(t, err)
	tree := treeNode{"a", []treeNode{{"b", nil}, {"c", []treeNode{{"d", nil}}}}}
	assert.Equal(t, runIn(t, env, "walk(t, x => x.kids)[*].name", risor.Map{"t": tree}),
		[]any{"a", "b", "c", "d"})
}

func TestWalkValueThatHoldsItself(t *testing.T) {
	// A host list that is its own element stops at the depth limit (5.4
	// item 4), not at a native stack overflow.
	env, err := risor.NewEnv(risor.VarRecord("{t: any}"))
	assert.NoError(t, err)
	l := []any{nil}
	l[0] = l
	d := runErr(t, env, "walk(t, x => x)", risor.Map{"t": l})
	assert.Equal(t, d.Code, "depth-exceeded")
}

func TestWalkAtTheLargestDepth(t *testing.T) {
	// A walk keeps its own stack (5.3): 400 levels of host lists at the
	// largest depth limit, and one more fails.
	env, err := risor.NewEnv(risor.VarRecord("{t: any}"),
		risor.Limits{Depth: 400, Steps: risor.NoLimit, Allocation: risor.NoLimit})
	assert.NoError(t, err)
	var v any = []any{}
	for i := 1; i < 400; i++ {
		v = []any{v}
	}
	out := run(t, env, "walk(t, x => x)", risor.Map{"t": v})
	assert.Equal(t, len(out.([]any)), 400)
	d := runErr(t, env, "walk(t, x => x)", risor.Map{"t": []any{v}})
	assert.Equal(t, d.Code, "depth-exceeded")
}

func TestGuardRuns(t *testing.T) {
	// A guard is tested after its arm's literal, and a falsy one passes to
	// the next arm (3).
	env, err := risor.NewEnv(risor.VarRecord("{x: int, y: int}"))
	assert.NoError(t, err)
	src := `match x { 1 if (y > 0) => "a", 1 => "b", _ => "c" }`
	assert.Equal(t, runIn(t, env, src, risor.Map{"x": 1, "y": 1}), "a")
	assert.Equal(t, runIn(t, env, src, risor.Map{"x": 1, "y": 0}), "b")
	assert.Equal(t, runIn(t, env, src, risor.Map{"x": 2, "y": 1}), "c")
}
