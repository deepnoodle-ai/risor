package ast

import (
	"testing"

	"github.com/risor-io/risor/internal/token"
)

func TestString(t *testing.T) {
	program := &Program{
		statements: []Node{
			&Var{
				token: token.Token{Type: token.LET, Literal: "var"},
				name: &Ident{
					token: token.Token{Type: token.IDENT, Literal: "myVar"},
					value: "myVar",
				},
				value: &Ident{
					token: token.Token{Type: token.IDENT, Literal: "anotherVar"},
					value: "anotherVar",
				},
			},
		},
	}
	if program.String() != "var myVar = anotherVar" {
		t.Errorf("program.String() wrong. got=%q", program.String())
	}
}

func TestForInNode(t *testing.T) {
	// Create tokens
	forToken := token.Token{Type: token.FOR, Literal: "for"}
	identToken := token.Token{Type: token.IDENT, Literal: "x"}

	// Create AST nodes
	variable := NewIdent(identToken)
	iterable := NewIdent(token.Token{Type: token.IDENT, Literal: "items"})
	block := &Block{
		token: token.Token{Type: token.LBRACE, Literal: "{"},
		statements: []Node{
			NewIdent(token.Token{Type: token.IDENT, Literal: "print"}),
		},
	}

	// Create ForIn node
	forIn := NewForIn(forToken, []*Ident{variable}, iterable, block)

	// Test all methods
	if forIn.Token().Type != token.FOR {
		t.Errorf("forIn.Token().Type wrong. got=%s", forIn.Token().Type)
	}

	if forIn.Literal() != "for" {
		t.Errorf("forIn.Literal() wrong. got=%s", forIn.Literal())
	}

	if forIn.IsExpression() != false {
		t.Errorf("forIn.IsExpression() wrong. got=%t", forIn.IsExpression())
	}

	if forIn.Variable().Literal() != "x" {
		t.Errorf("forIn.Variable().Literal() wrong. got=%s", forIn.Variable().Literal())
	}

	if forIn.Iterable().String() != "items" {
		t.Errorf("forIn.Iterable().String() wrong. got=%s", forIn.Iterable().String())
	}

	if forIn.Consequence() == nil {
		t.Error("forIn.Consequence() should not be nil")
	}

	expected := "for x in items print"
	if forIn.String() != expected {
		t.Errorf("forIn.String() wrong. got=%q, expected=%q", forIn.String(), expected)
	}
}

func TestForInNodeComplexCase(t *testing.T) {
	// Test with more complex expressions
	forToken := token.Token{Type: token.FOR, Literal: "for"}

	// Variable
	variable := NewIdent(token.Token{Type: token.IDENT, Literal: "item"})

	// Complex iterable (list literal)
	list := &List{
		token: token.Token{Type: token.LBRACKET, Literal: "["},
		items: []Expression{
			&Int{token: token.Token{Type: token.INT, Literal: "1"}, value: 1},
			&Int{token: token.Token{Type: token.INT, Literal: "2"}, value: 2},
			&Int{token: token.Token{Type: token.INT, Literal: "3"}, value: 3},
		},
	}

	// Block with multiple statements
	block := &Block{
		token: token.Token{Type: token.LBRACE, Literal: "{"},
		statements: []Node{
			&Call{
				token:    token.Token{Type: token.IDENT, Literal: "print"},
				function: NewIdent(token.Token{Type: token.IDENT, Literal: "print"}),
				arguments: []Node{
					NewIdent(token.Token{Type: token.IDENT, Literal: "item"}),
				},
			},
		},
	}

	forIn := NewForIn(forToken, []*Ident{variable}, list, block)

	// Test String output
	result := forIn.String()
	if !contains(result, "for item in") {
		t.Errorf("forIn.String() should contain 'for item in', got=%q", result)
	}
	if !contains(result, "[1, 2, 3]") {
		t.Errorf("forIn.String() should contain array literal, got=%q", result)
	}
}

func TestForInNodeMethods(t *testing.T) {
	// Test that ForIn implements the Statement interface
	forToken := token.Token{Type: token.FOR, Literal: "for"}
	variable := NewIdent(token.Token{Type: token.IDENT, Literal: "x"})
	iterable := NewIdent(token.Token{Type: token.IDENT, Literal: "items"})
	block := &Block{token: token.Token{Type: token.LBRACE, Literal: "{"}}

	forIn := NewForIn(forToken, []*Ident{variable}, iterable, block)

	// Test that it implements Node interface
	var node Node = forIn
	if node.Token().Type != token.FOR {
		t.Errorf("ForIn should implement Node interface correctly")
	}

	// Test StatementNode method (should not panic)
	forIn.StatementNode()

	// Test that all accessors work
	if forIn.Variable() != variable {
		t.Errorf("Variable() should return the correct variable")
	}
	if forIn.Iterable() != iterable {
		t.Errorf("Iterable() should return the correct iterable")
	}
	if forIn.Consequence() != block {
		t.Errorf("Consequence() should return the correct block")
	}
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
