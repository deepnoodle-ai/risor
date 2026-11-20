package risor_test

import (
	"errors"
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

// Step E2 (12.4): a Go struct that contains itself, through TypeOf.
type Node struct {
	Name     string `json:"name"`
	Size     int    `json:"size"`
	Children []Node `json:"children"`
}

func TestTypeOfRecursiveStruct(t *testing.T) {
	ty, err := risor.TypeOf[Node]()
	assert.NoError(t, err)
	assert.Equal(t, ty.String(), "Node")
	env, err := risor.NewEnv(risor.VarType("tree", ty))
	assert.NoError(t, err)
	src := "const total = (n: Node): int => n.size + n.children.map(total).sum(); total(tree)"
	// The name Node in the program is no type of the environment.
	assert.Equal(t, checkErr(t, env, src)[0].Code, "undefined-name")
	env, err = risor.NewEnv(risor.NamedTypeOf("Node", ty), risor.VarType("tree", ty))
	assert.NoError(t, err)
	tree := Node{"a", 1, []Node{{"b", 2, nil}, {"c", 3, []Node{{"d", 4, nil}}}}}
	assert.Equal(t, runIn(t, env, src, risor.Map{"tree": tree}), float64(10))
	// A wrong field inside the tree names the type.
	ds := checkErr(t, env, "const f = (n: Node): int => len(n.childs); f(tree)")
	assert.Equal(t, ds[0].Message, `Node has no field "childs"`)
}

func TestConformDepth(t *testing.T) {
	// 12.7: the conformance function stops at the default depth limit.
	env, err := risor.NewEnv(risor.NamedType("Tree", "{kids: Tree[]}"))
	assert.NoError(t, err)
	ty, err := env.ParseType("Tree")
	assert.NoError(t, err)
	var v any = map[string]any{"kids": []any{}}
	for i := 1; i < 100; i++ {
		v = map[string]any{"kids": []any{v}}
	}
	// 100 maps and 100 lists: 200 levels.
	assert.NoError(t, risor.Conform(v, ty))
	assert.Equal(t, len(risor.ConformAll(v, ty, 10)), 0)
	v = map[string]any{"kids": []any{v}}
	err = risor.Conform(v, ty)
	var e *risor.Error
	assert.True(t, errors.As(err, &e))
	assert.Equal(t, e.Diagnostics[0].Code, "depth-exceeded")
	assert.Equal(t, e.Diagnostics[0].Message, "calls and nested values went deeper than the depth limit of 200")
	ds := risor.ConformAll(v, ty, 10)
	assert.Equal(t, len(ds), 1)
	assert.Equal(t, ds[0].Code, "depth-exceeded")
}
