// Package ast defines the abstract syntax tree representation of Risor code.
package ast

import "github.com/risor-io/risor/internal/token"

// Node represents a portion of the syntax tree. All nodes have position
// information indicating where they appear in the source code.
type Node interface {
	// Pos returns the position of the first character belonging to the node.
	Pos() token.Position

	// End returns the position of the first character immediately after the node.
	End() token.Position

	// String returns a human friendly representation of the Node. This should
	// be similar to the original source code, but not necessarily identical.
	String() string
}

// Stmt represents a statement node. Statements cause side effects but
// do not evaluate to a value.
type Stmt interface {
	Node
	stmtNode()
}

// Expr represents an expression node. Expressions evaluate to a value
// and may be embedded within other expressions.
type Expr interface {
	Node
	exprNode()
}

// BadExpr represents an expression containing syntax errors.
// It is used by the parser to continue parsing after an error,
// allowing subsequent errors to be detected without giving up.
type BadExpr struct {
	From token.Position // start of bad expression
	To   token.Position // end of bad expression
}

func (x *BadExpr) exprNode() {}

func (x *BadExpr) Pos() token.Position { return x.From }
func (x *BadExpr) End() token.Position { return x.To }
func (x *BadExpr) String() string      { return "<bad expression>" }

// BadStmt represents a statement containing syntax errors.
// It is used by the parser to continue parsing after an error,
// allowing subsequent errors to be detected without giving up.
type BadStmt struct {
	From token.Position // start of bad statement
	To   token.Position // end of bad statement
}

func (x *BadStmt) stmtNode() {}

func (x *BadStmt) Pos() token.Position { return x.From }
func (x *BadStmt) End() token.Position { return x.To }
func (x *BadStmt) String() string      { return "<bad statement>" }
