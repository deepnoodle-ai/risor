package ast

import (
	"bytes"

	"github.com/deepnoodle-ai/risor/v2/internal/token"
)

// Program represents a complete Risor program, which consists of a series of
// statements.
type Program struct {
	Stmts []Node // statements in the program
}

func (p *Program) Pos() token.Position {
	if len(p.Stmts) > 0 {
		return p.Stmts[0].Pos()
	}
	return token.NoPos
}

func (p *Program) End() token.Position {
	if len(p.Stmts) > 0 {
		return p.Stmts[len(p.Stmts)-1].End()
	}
	return token.NoPos
}

// First returns the first statement in the program, or nil if empty.
func (p *Program) First() Node {
	if len(p.Stmts) > 0 {
		return p.Stmts[0]
	}
	return nil
}

func (p *Program) String() string {
	var out bytes.Buffer
	stmtCount := len(p.Stmts)
	for i, stmt := range p.Stmts {
		out.WriteString(stmt.String())
		if i < stmtCount-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}
