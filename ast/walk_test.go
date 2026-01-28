package ast

import (
	"testing"

	"github.com/risor-io/risor/internal/token"
)

func TestWalk(t *testing.T) {
	// Build a simple AST: let x = 1 + 2
	program := &Program{
		Stmts: []Node{
			&Var{
				Let: token.Position{Line: 1, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 1, Column: 5},
					Name:    "x",
				},
				Value: &Infix{
					X: &Int{
						ValuePos: token.Position{Line: 1, Column: 9},
						Value:    1,
					},
					OpPos: token.Position{Line: 1, Column: 11},
					Op:    "+",
					Y: &Int{
						ValuePos: token.Position{Line: 1, Column: 13},
						Value:    2,
					},
				},
			},
		},
	}

	var visited []string
	Inspect(program, func(n Node) bool {
		switch node := n.(type) {
		case *Program:
			visited = append(visited, "Program")
		case *Var:
			visited = append(visited, "Var")
		case *Infix:
			visited = append(visited, "Infix:"+node.Op)
		case *Int:
			visited = append(visited, "Int")
		}
		return true
	})

	expected := []string{"Program", "Var", "Infix:+", "Int", "Int"}
	if len(visited) != len(expected) {
		t.Errorf("expected %d nodes, got %d: %v", len(expected), len(visited), visited)
		return
	}
	for i, v := range expected {
		if visited[i] != v {
			t.Errorf("expected %q at index %d, got %q", v, i, visited[i])
		}
	}
}

func TestWalkIf(t *testing.T) {
	// Build: if (true) { 1 }
	program := &Program{
		Stmts: []Node{
			&If{
				If: token.Position{Line: 1, Column: 1},
				Cond: &Bool{
					ValuePos: token.Position{Line: 1, Column: 5},
					Value:    true,
				},
				Consequence: &Block{
					Lbrace: token.Position{Line: 1, Column: 11},
					Stmts: []Node{
						&Int{
							ValuePos: token.Position{Line: 1, Column: 13},
							Value:    1,
						},
					},
					Rbrace: token.Position{Line: 1, Column: 15},
				},
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, If, Bool, Block, Int
	if count != 5 {
		t.Errorf("expected 5 nodes, got %d", count)
	}
}

func TestWalkFunc(t *testing.T) {
	// Build: func foo(x) { return x }
	xIdent := &Ident{
		NamePos: token.Position{Line: 1, Column: 14},
		Name:    "x",
	}
	program := &Program{
		Stmts: []Node{
			&Func{
				Func: token.Position{Line: 1, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 1, Column: 10},
					Name:    "foo",
				},
				Params: []FuncParam{xIdent},
				Body: &Block{
					Lbrace: token.Position{Line: 1, Column: 17},
					Stmts: []Node{
						&Return{
							Return: token.Position{Line: 1, Column: 19},
							Value: &Ident{
								NamePos: token.Position{Line: 1, Column: 26},
								Name:    "x",
							},
						},
					},
					Rbrace: token.Position{Line: 1, Column: 28},
				},
			},
		},
	}

	var nodes []string
	Inspect(program, func(n Node) bool {
		switch n.(type) {
		case *Program:
			nodes = append(nodes, "Program")
		case *Func:
			nodes = append(nodes, "Func")
		case *Ident:
			nodes = append(nodes, "Ident")
		case *Block:
			nodes = append(nodes, "Block")
		case *Return:
			nodes = append(nodes, "Return")
		}
		return true
	})

	// Program, Func, Ident (name "foo"), Ident (param x), Block, Return, Ident (return x)
	expected := []string{"Program", "Func", "Ident", "Ident", "Block", "Return", "Ident"}
	if len(nodes) != len(expected) {
		t.Errorf("expected %d nodes, got %d: %v", len(expected), len(nodes), nodes)
		return
	}
	for i, v := range expected {
		if nodes[i] != v {
			t.Errorf("expected %q at index %d, got %q", v, i, nodes[i])
		}
	}
}

func TestWalkMap(t *testing.T) {
	// Build: {"a": 1, "b": 2}
	program := &Program{
		Stmts: []Node{
			&Map{
				Lbrace: token.Position{Line: 1, Column: 1},
				Items: []MapItem{
					{
						Key: &String{
							ValuePos: token.Position{Line: 1, Column: 2},
							Value:    "a",
						},
						Value: &Int{
							ValuePos: token.Position{Line: 1, Column: 7},
							Value:    1,
						},
					},
					{
						Key: &String{
							ValuePos: token.Position{Line: 1, Column: 10},
							Value:    "b",
						},
						Value: &Int{
							ValuePos: token.Position{Line: 1, Column: 15},
							Value:    2,
						},
					},
				},
				Rbrace: token.Position{Line: 1, Column: 16},
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, Map, String, Int, String, Int
	if count != 6 {
		t.Errorf("expected 6 nodes, got %d", count)
	}
}

func TestInspectStopEarly(t *testing.T) {
	// Build: let x = 1 + 2
	program := &Program{
		Stmts: []Node{
			&Var{
				Let: token.Position{Line: 1, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 1, Column: 5},
					Name:    "x",
				},
				Value: &Infix{
					X: &Int{
						ValuePos: token.Position{Line: 1, Column: 9},
						Value:    1,
					},
					OpPos: token.Position{Line: 1, Column: 11},
					Op:    "+",
					Y: &Int{
						ValuePos: token.Position{Line: 1, Column: 13},
						Value:    2,
					},
				},
			},
		},
	}

	var visited []string
	Inspect(program, func(n Node) bool {
		switch n.(type) {
		case *Program:
			visited = append(visited, "Program")
			return true
		case *Var:
			visited = append(visited, "Var")
			return false // Stop descending into Var
		}
		return true
	})

	expected := []string{"Program", "Var"}
	if len(visited) != len(expected) {
		t.Errorf("expected %d nodes, got %d: %v", len(expected), len(visited), visited)
	}
}

func TestPreorder(t *testing.T) {
	// Build: let x = 1 + 2
	program := &Program{
		Stmts: []Node{
			&Var{
				Let: token.Position{Line: 1, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 1, Column: 5},
					Name:    "x",
				},
				Value: &Infix{
					X: &Int{
						ValuePos: token.Position{Line: 1, Column: 9},
						Value:    1,
					},
					OpPos: token.Position{Line: 1, Column: 11},
					Op:    "+",
					Y: &Int{
						ValuePos: token.Position{Line: 1, Column: 13},
						Value:    2,
					},
				},
			},
		},
	}

	var visited []string
	for n := range Preorder(program) {
		switch node := n.(type) {
		case *Program:
			visited = append(visited, "Program")
		case *Var:
			visited = append(visited, "Var")
		case *Infix:
			visited = append(visited, "Infix:"+node.Op)
		case *Int:
			visited = append(visited, "Int")
		}
	}

	expected := []string{"Program", "Var", "Infix:+", "Int", "Int"}
	if len(visited) != len(expected) {
		t.Errorf("expected %d nodes, got %d: %v", len(expected), len(visited), visited)
		return
	}
	for i, v := range expected {
		if visited[i] != v {
			t.Errorf("expected %q at index %d, got %q", v, i, visited[i])
		}
	}
}

func TestPreorderBreak(t *testing.T) {
	// Build: let x = 1 + 2
	program := &Program{
		Stmts: []Node{
			&Var{
				Let: token.Position{Line: 1, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 1, Column: 5},
					Name:    "x",
				},
				Value: &Infix{
					X: &Int{
						ValuePos: token.Position{Line: 1, Column: 9},
						Value:    1,
					},
					OpPos: token.Position{Line: 1, Column: 11},
					Op:    "+",
					Y: &Int{
						ValuePos: token.Position{Line: 1, Column: 13},
						Value:    2,
					},
				},
			},
		},
	}

	var count int
	for range Preorder(program) {
		count++
		if count == 3 {
			break
		}
	}

	if count != 3 {
		t.Errorf("expected to stop after 3 nodes, got %d", count)
	}
}

func TestWalkBadExpr(t *testing.T) {
	// BadExpr should be visited but has no children
	program := &Program{
		Stmts: []Node{
			&Var{
				Let: token.Position{Line: 1, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 1, Column: 5},
					Name:    "x",
				},
				Value: &BadExpr{
					From: token.Position{Line: 1, Column: 9},
					To:   token.Position{Line: 1, Column: 15},
				},
			},
		},
	}

	var visited []string
	Inspect(program, func(n Node) bool {
		switch n.(type) {
		case *Program:
			visited = append(visited, "Program")
		case *Var:
			visited = append(visited, "Var")
		case *BadExpr:
			visited = append(visited, "BadExpr")
		}
		return true
	})

	// Should visit: Program, Var, BadExpr
	expected := []string{"Program", "Var", "BadExpr"}
	if len(visited) != len(expected) {
		t.Errorf("expected %d nodes, got %d: %v", len(expected), len(visited), visited)
		return
	}
	for i, v := range expected {
		if visited[i] != v {
			t.Errorf("expected %q at index %d, got %q", v, i, visited[i])
		}
	}
}

func TestWalkBadStmt(t *testing.T) {
	// BadStmt should be visited but has no children
	program := &Program{
		Stmts: []Node{
			&BadStmt{
				From: token.Position{Line: 1, Column: 1},
				To:   token.Position{Line: 1, Column: 10},
			},
			&Var{
				Let: token.Position{Line: 2, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 2, Column: 5},
					Name:    "x",
				},
				Value: &Int{
					ValuePos: token.Position{Line: 2, Column: 9},
					Value:    42,
				},
			},
		},
	}

	var visited []string
	Inspect(program, func(n Node) bool {
		switch n.(type) {
		case *Program:
			visited = append(visited, "Program")
		case *BadStmt:
			visited = append(visited, "BadStmt")
		case *Var:
			visited = append(visited, "Var")
		case *Int:
			visited = append(visited, "Int")
		}
		return true
	})

	// Should visit: Program, BadStmt, Var, Int
	expected := []string{"Program", "BadStmt", "Var", "Int"}
	if len(visited) != len(expected) {
		t.Errorf("expected %d nodes, got %d: %v", len(expected), len(visited), visited)
		return
	}
	for i, v := range expected {
		if visited[i] != v {
			t.Errorf("expected %q at index %d, got %q", v, i, visited[i])
		}
	}
}

func TestPreorderBadExpr(t *testing.T) {
	// Test Preorder iterator with BadExpr
	program := &Program{
		Stmts: []Node{
			&BadExpr{
				From: token.Position{Line: 1, Column: 1},
				To:   token.Position{Line: 1, Column: 10},
			},
		},
	}

	var count int
	for n := range Preorder(program) {
		count++
		if _, ok := n.(*BadExpr); ok {
			// BadExpr was visited
		}
	}

	// Should visit Program and BadExpr
	if count != 2 {
		t.Errorf("expected 2 nodes, got %d", count)
	}
}

func TestPreorderBadStmt(t *testing.T) {
	// Test Preorder iterator with BadStmt
	program := &Program{
		Stmts: []Node{
			&BadStmt{
				From: token.Position{Line: 1, Column: 1},
				To:   token.Position{Line: 1, Column: 10},
			},
		},
	}

	var count int
	for n := range Preorder(program) {
		count++
		if _, ok := n.(*BadStmt); ok {
			// BadStmt was visited
		}
	}

	// Should visit Program and BadStmt
	if count != 2 {
		t.Errorf("expected 2 nodes, got %d", count)
	}
}

// Tests for walking nodes with nil children - ensures Walk/Preorder don't panic

func TestWalkVarNilValue(t *testing.T) {
	// Walk should handle Var with nil Value
	program := &Program{
		Stmts: []Node{
			&Var{
				Let:   token.Position{Line: 1, Column: 1},
				Name:  &Ident{NamePos: token.Position{Line: 1, Column: 5}, Name: "x"},
				Value: nil,
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, Var (Value is nil, so not visited)
	if count != 2 {
		t.Errorf("expected 2 nodes, got %d", count)
	}
}

func TestWalkInfixNilOperands(t *testing.T) {
	// Walk should handle Infix with nil X or Y
	program := &Program{
		Stmts: []Node{
			&Infix{
				X:     nil,
				OpPos: token.Position{Line: 1, Column: 3},
				Op:    "+",
				Y:     &Int{ValuePos: token.Position{Line: 1, Column: 5}, Value: 2},
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, Infix, Int (X is nil)
	if count != 3 {
		t.Errorf("expected 3 nodes, got %d", count)
	}
}

func TestWalkObjectDestructureParam(t *testing.T) {
	// Walk should handle ObjectDestructureParam with defaults
	program := &Program{
		Stmts: []Node{
			&Func{
				Func:   token.Position{Line: 1, Column: 1},
				Lparen: token.Position{Line: 1, Column: 9},
				Params: []FuncParam{
					&ObjectDestructureParam{
						Lbrace: token.Position{Line: 1, Column: 10},
						Bindings: []DestructureBinding{
							{Key: "a", Default: &Int{ValuePos: token.Position{Line: 1, Column: 15}, Value: 1}},
							{Key: "b", Default: nil}, // no default
						},
						Rbrace: token.Position{Line: 1, Column: 20},
					},
				},
				Rparen: token.Position{Line: 1, Column: 21},
				Body: &Block{
					Lbrace: token.Position{Line: 1, Column: 23},
					Stmts:  []Node{},
					Rbrace: token.Position{Line: 1, Column: 25},
				},
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, Func, ObjectDestructureParam, Int (default), Block
	if count != 5 {
		t.Errorf("expected 5 nodes, got %d", count)
	}
}

func TestWalkArrayDestructureParam(t *testing.T) {
	// Walk should handle ArrayDestructureParam with defaults
	program := &Program{
		Stmts: []Node{
			&Func{
				Func:   token.Position{Line: 1, Column: 1},
				Lparen: token.Position{Line: 1, Column: 9},
				Params: []FuncParam{
					&ArrayDestructureParam{
						Lbrack: token.Position{Line: 1, Column: 10},
						Elements: []ArrayDestructureElement{
							{
								Name:    &Ident{NamePos: token.Position{Line: 1, Column: 11}, Name: "a"},
								Default: &Int{ValuePos: token.Position{Line: 1, Column: 15}, Value: 1},
							},
							{
								Name:    &Ident{NamePos: token.Position{Line: 1, Column: 18}, Name: "b"},
								Default: nil, // no default
							},
						},
						Rbrack: token.Position{Line: 1, Column: 20},
					},
				},
				Rparen: token.Position{Line: 1, Column: 21},
				Body: &Block{
					Lbrace: token.Position{Line: 1, Column: 23},
					Stmts:  []Node{},
					Rbrace: token.Position{Line: 1, Column: 25},
				},
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, Func, ArrayDestructureParam, Ident(a), Int(default), Ident(b), Block
	if count != 7 {
		t.Errorf("expected 7 nodes, got %d", count)
	}
}

func TestWalkDefaultValue(t *testing.T) {
	// Walk should handle DefaultValue node
	program := &Program{
		Stmts: []Node{
			&DefaultValue{
				Name:    &Ident{NamePos: token.Position{Line: 1, Column: 1}, Name: "x"},
				Default: &Int{ValuePos: token.Position{Line: 1, Column: 5}, Value: 42},
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, DefaultValue, Ident, Int
	if count != 4 {
		t.Errorf("expected 4 nodes, got %d", count)
	}
}

func TestPreorderVarNilValue(t *testing.T) {
	// Preorder should handle Var with nil Value
	program := &Program{
		Stmts: []Node{
			&Var{
				Let:   token.Position{Line: 1, Column: 1},
				Name:  &Ident{NamePos: token.Position{Line: 1, Column: 5}, Name: "x"},
				Value: nil,
			},
		},
	}

	var count int
	for range Preorder(program) {
		count++
	}

	// Program, Var (Value is nil)
	if count != 2 {
		t.Errorf("expected 2 nodes, got %d", count)
	}
}

func TestPreorderObjectDestructureParam(t *testing.T) {
	// Preorder should handle ObjectDestructureParam
	param := &ObjectDestructureParam{
		Lbrace: token.Position{Line: 1, Column: 10},
		Bindings: []DestructureBinding{
			{Key: "a", Default: &Int{ValuePos: token.Position{Line: 1, Column: 15}, Value: 1}},
		},
		Rbrace: token.Position{Line: 1, Column: 20},
	}

	var count int
	for range Preorder(param) {
		count++
	}

	// ObjectDestructureParam, Int (default)
	if count != 2 {
		t.Errorf("expected 2 nodes, got %d", count)
	}
}

func TestWalkStringWithExprs(t *testing.T) {
	// Walk should visit template expressions in strings
	program := &Program{
		Stmts: []Node{
			&String{
				ValuePos: token.Position{Line: 1, Column: 1},
				Literal:  `"hello {name}"`,
				Value:    "hello {name}",
				Exprs: []Expr{
					&Ident{NamePos: token.Position{Line: 1, Column: 8}, Name: "name"},
				},
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, String, Ident
	if count != 3 {
		t.Errorf("expected 3 nodes, got %d", count)
	}
}

func TestWalkTryNilParts(t *testing.T) {
	// Walk should handle Try with nil CatchBlock and FinallyBlock
	program := &Program{
		Stmts: []Node{
			&Try{
				Try: token.Position{Line: 1, Column: 1},
				Body: &Block{
					Lbrace: token.Position{Line: 1, Column: 5},
					Stmts:  []Node{},
					Rbrace: token.Position{Line: 1, Column: 7},
				},
				CatchIdent:   nil,
				CatchBlock:   nil,
				FinallyBlock: nil,
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, Try, Block
	if count != 3 {
		t.Errorf("expected 3 nodes, got %d", count)
	}
}

func TestWalkMapWithSpread(t *testing.T) {
	// Walk should handle Map with spread items (Key is nil)
	program := &Program{
		Stmts: []Node{
			&Map{
				Lbrace: token.Position{Line: 1, Column: 1},
				Items: []MapItem{
					{
						Key:   nil, // spread
						Value: &Ident{NamePos: token.Position{Line: 1, Column: 5}, Name: "other"},
					},
					{
						Key:   &String{ValuePos: token.Position{Line: 1, Column: 12}, Value: "a"},
						Value: &Int{ValuePos: token.Position{Line: 1, Column: 17}, Value: 1},
					},
				},
				Rbrace: token.Position{Line: 1, Column: 18},
			},
		},
	}

	var count int
	Inspect(program, func(n Node) bool {
		count++
		return true
	})

	// Program, Map, Ident(other), String(key), Int(value)
	if count != 5 {
		t.Errorf("expected 5 nodes, got %d", count)
	}
}
