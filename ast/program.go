package ast

import (
	"bytes"

	"github.com/risor-io/risor/internal/token"
)

// Program represents a complete Risor program, which consists of a series of
// statements.
type Program struct {
	// The list of statements which comprise the program.
	statements []Node
}

func NewProgram(statements []Node) *Program {
	return &Program{statements: statements}
}

func (p *Program) Token() token.Token {
	if len(p.statements) > 0 {
		return p.statements[0].Token()
	}
	return token.Token{}
}

func (p *Program) IsExpression() bool { return false }

func (p *Program) Literal() string { return p.Token().Literal }

func (p *Program) Statements() []Node { return p.statements }

func (p *Program) First() Node {
	if len(p.statements) > 0 {
		return p.statements[0]
	}
	return nil
}

func (p *Program) String() string {
	var out bytes.Buffer
	stmtCount := len(p.statements)
	for i, stmt := range p.statements {
		out.WriteString(stmt.String())
		if i < stmtCount-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}
