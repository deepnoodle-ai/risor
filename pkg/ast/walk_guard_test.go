package ast

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

// Walk and Preorder must visit the default arm's guard and must never pass a
// nil node to visitors.

func walkNodes(root Node) []Node {
	var nodes []Node
	Inspect(root, func(n Node) bool {
		nodes = append(nodes, n)
		return true
	})
	return nodes
}

func preorderNodes(root Node) []Node {
	var nodes []Node
	for n := range Preorder(root) {
		nodes = append(nodes, n)
	}
	return nodes
}

func TestWalkVisitsMatchDefaultGuard(t *testing.T) {
	guard := &Ident{Name: "forbidden"}
	match := &Match{
		Subject: &Ident{Name: "x"},
		Default: &MatchArm{
			Pattern: &WildcardPattern{},
			Guard:   guard,
			Result:  &Int{Value: 1},
		},
	}
	for name, nodes := range map[string][]Node{
		"Walk":     walkNodes(match),
		"Preorder": preorderNodes(match),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, nodes, Node(guard))
		})
	}
}

func TestWalkSkipsNilTemplateExprs(t *testing.T) {
	x := &Ident{Name: "x"}
	str := &String{Exprs: []Expr{nil, x, nil}}
	for name, nodes := range map[string][]Node{
		"Walk":     walkNodes(str),
		"Preorder": preorderNodes(str),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Len(t, nodes, 2)
			for _, n := range nodes {
				assert.NotNil(t, n)
			}
			assert.Contains(t, nodes, Node(x))
		})
	}
}
