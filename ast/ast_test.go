package ast

import (
	"testing"

	"github.com/risor-io/risor/internal/token"
)

func TestString(t *testing.T) {
	program := &Program{
		Stmts: []Node{
			&Var{
				Let: token.Position{Line: 1, Column: 1},
				Name: &Ident{
					NamePos: token.Position{Line: 1, Column: 5},
					Name:    "myVar",
				},
				Value: &Ident{
					NamePos: token.Position{Line: 1, Column: 13},
					Name:    "anotherVar",
				},
			},
		},
	}
	if program.String() != "let myVar = anotherVar" {
		t.Errorf("program.String() wrong. got=%q", program.String())
	}
}

func TestBadExpr(t *testing.T) {
	from := token.Position{Line: 1, Column: 5, File: "test.risor"}
	to := token.Position{Line: 1, Column: 15, File: "test.risor"}

	bad := &BadExpr{From: from, To: to}

	// Test Pos() returns From
	if bad.Pos() != from {
		t.Errorf("BadExpr.Pos() = %v, want %v", bad.Pos(), from)
	}

	// Test End() returns To
	if bad.End() != to {
		t.Errorf("BadExpr.End() = %v, want %v", bad.End(), to)
	}

	// Test String() returns placeholder
	expected := "<bad expression>"
	if bad.String() != expected {
		t.Errorf("BadExpr.String() = %q, want %q", bad.String(), expected)
	}

	// Test that BadExpr implements Expr interface
	var _ Expr = bad
}

func TestBadStmt(t *testing.T) {
	from := token.Position{Line: 2, Column: 1, File: "test.risor"}
	to := token.Position{Line: 2, Column: 20, File: "test.risor"}

	bad := &BadStmt{From: from, To: to}

	// Test Pos() returns From
	if bad.Pos() != from {
		t.Errorf("BadStmt.Pos() = %v, want %v", bad.Pos(), from)
	}

	// Test End() returns To
	if bad.End() != to {
		t.Errorf("BadStmt.End() = %v, want %v", bad.End(), to)
	}

	// Test String() returns placeholder
	expected := "<bad statement>"
	if bad.String() != expected {
		t.Errorf("BadStmt.String() = %q, want %q", bad.String(), expected)
	}

	// Test that BadStmt implements Stmt interface
	var _ Stmt = bad
}

func TestBadExprInProgram(t *testing.T) {
	// Test that BadExpr can be used as a value in a Var statement
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

	// Verify the program can be stringified
	str := program.String()
	if str == "" {
		t.Error("Program with BadExpr should stringify")
	}
}

func TestBadStmtInProgram(t *testing.T) {
	// Test that BadStmt can be included in a program
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

	// Verify the program has both statements
	if len(program.Stmts) != 2 {
		t.Errorf("Expected 2 statements, got %d", len(program.Stmts))
	}

	// Verify BadStmt is first
	if _, ok := program.Stmts[0].(*BadStmt); !ok {
		t.Error("First statement should be BadStmt")
	}
}
