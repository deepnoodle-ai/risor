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
