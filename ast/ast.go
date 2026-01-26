// Package ast defines the abstract syntax tree representation of Risor code.
package ast

import "github.com/risor-io/risor/internal/token"

// Node represents a portion of the syntax tree. All nodes have a token, which is
// the token that begins the node. A Node may be an Expression, in which case
// it evaluates to a value.
type Node interface {
	// Token returns the token where this Node begins.
	Token() token.Token

	// Literal returns the string from the first token that defines the Node.
	Literal() string

	// String returns a human friendly representation of the Node. This should
	// be similar to the original source code, but not necessarily identical.
	String() string

	// IsExpression returns true if this Node evalutes to a value.
	IsExpression() bool
}

// Statement represents a snippet of Risor code that causes a side effect, but
// does not evaluate to a value.
type Statement interface {
	// Node is embedded here to indicate that all statements are AST nodes.
	Node

	// StatementNode signals that this Node is a statement.
	StatementNode()
}

// Expression represents a snippet of Risor code that evaluates to a value.
// Expressions may be embedded within other expressions.
type Expression interface {
	// Node is embedded here to indicate that all expressions are AST nodes.
	Node

	// ExpressionNode signals that this Node is an expression.
	ExpressionNode()
}
