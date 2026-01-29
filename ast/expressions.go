package ast

import (
	"bytes"
	"strings"

	"github.com/risor-io/risor/internal/token"
)

// Ident is an expression node that refers to a variable by name.
type Ident struct {
	NamePos token.Position // position of identifier
	Name    string         // identifier name
}

func (x *Ident) exprNode() {}

func (x *Ident) Pos() token.Position { return x.NamePos }
func (x *Ident) End() token.Position { return x.NamePos.Advance(len(x.Name)) }

func (x *Ident) String() string { return x.Name }

// Prefix is an operator expression where the operator precedes the operand.
// Examples include "!false" and "-x".
type Prefix struct {
	OpPos token.Position // position of operator
	Op    string         // operator: "!", "-", "not"
	X     Expr           // operand
}

func (x *Prefix) exprNode() {}

func (x *Prefix) Pos() token.Position { return x.OpPos }
func (x *Prefix) End() token.Position {
	if x.X != nil {
		return x.X.End()
	}
	return x.OpPos.Advance(len(x.Op))
}

func (x *Prefix) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(x.Op)
	out.WriteString(x.X.String())
	out.WriteString(")")
	return out.String()
}

// Spread represents a spread expression (...expr) used in array literals,
// object literals, and function calls. Also used for rest parameters.
type Spread struct {
	Ellipsis token.Position // position of "..."
	X        Expr           // expression being spread; nil for rest parameters
}

func (x *Spread) exprNode() {}

func (x *Spread) Pos() token.Position { return x.Ellipsis }
func (x *Spread) End() token.Position {
	if x.X != nil {
		return x.X.End()
	}
	return x.Ellipsis.Advance(3) // len("...")
}

func (x *Spread) String() string {
	var out bytes.Buffer
	out.WriteString("...")
	if x.X != nil {
		out.WriteString(x.X.String())
	}
	return out.String()
}

// Infix is an operator expression where the operator is between the operands.
// Examples include "x + y" and "5 - 1".
type Infix struct {
	X     Expr           // left operand
	OpPos token.Position // position of operator
	Op    string         // operator: "+", "-", "*", "/", etc.
	Y     Expr           // right operand
}

func (x *Infix) exprNode() {}

func (x *Infix) Pos() token.Position {
	if x.X != nil {
		return x.X.Pos()
	}
	return x.OpPos
}

func (x *Infix) End() token.Position {
	if x.Y != nil {
		return x.Y.End()
	}
	return x.OpPos.Advance(len(x.Op))
}

func (x *Infix) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(x.X.String())
	out.WriteString(" " + x.Op + " ")
	out.WriteString(x.Y.String())
	out.WriteString(")")
	return out.String()
}

// If is an expression node that represents an if/else expression.
type If struct {
	If          token.Position // position of "if" keyword
	Lparen      token.Position // position of "("
	Cond        Expr           // condition
	Rparen      token.Position // position of ")"
	Consequence *Block         // then branch
	Alternative *Block         // else branch; nil if no else
}

func (x *If) exprNode() {}

func (x *If) Pos() token.Position { return x.If }
func (x *If) End() token.Position {
	if x.Alternative != nil {
		return x.Alternative.End()
	}
	if x.Consequence != nil {
		return x.Consequence.End()
	}
	return x.Rparen.Advance(1)
}

func (x *If) String() string {
	var out bytes.Buffer
	out.WriteString("if (")
	out.WriteString(x.Cond.String())
	out.WriteString(") ")
	out.WriteString(x.Consequence.String())
	if x.Alternative != nil {
		out.WriteString(" else ")
		out.WriteString(x.Alternative.String())
	}
	return out.String()
}

// Call is an expression node that describes the invocation of a function.
type Call struct {
	Fun    Expr           // function expression
	Lparen token.Position // position of "("
	Args   []Node         // function arguments (Expr or Spread)
	Rparen token.Position // position of ")"
}

func (x *Call) exprNode() {}

func (x *Call) Pos() token.Position {
	if x.Fun != nil {
		return x.Fun.Pos()
	}
	return x.Lparen
}
func (x *Call) End() token.Position { return x.Rparen.Advance(1) }

func (x *Call) String() string {
	var out bytes.Buffer
	args := make([]string, 0, len(x.Args))
	for _, a := range x.Args {
		args = append(args, a.String())
	}
	out.WriteString(x.Fun.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

// GetAttr is an expression node that describes the access of an attribute on
// an object.
type GetAttr struct {
	X        Expr           // object expression
	Period   token.Position // position of "." or "?."
	Attr     *Ident         // attribute name
	Optional bool           // true if optional chaining (?.)
}

func (x *GetAttr) exprNode() {}

func (x *GetAttr) Pos() token.Position {
	if x.X != nil {
		return x.X.Pos()
	}
	return x.Period
}
func (x *GetAttr) End() token.Position { return x.Attr.End() }

func (x *GetAttr) String() string {
	var out bytes.Buffer
	out.WriteString(x.X.String())
	if x.Optional {
		out.WriteString("?.")
	} else {
		out.WriteString(".")
	}
	out.WriteString(x.Attr.Name)
	return out.String()
}

// Pipe is an expression node that describes a sequence of transformations
// applied to an initial value.
type Pipe struct {
	Exprs []Expr // pipe-separated expressions
}

func (x *Pipe) exprNode() {}

func (x *Pipe) Pos() token.Position {
	if len(x.Exprs) > 0 {
		return x.Exprs[0].Pos()
	}
	return token.NoPos
}

func (x *Pipe) End() token.Position {
	if len(x.Exprs) > 0 {
		return x.Exprs[len(x.Exprs)-1].End()
	}
	return token.NoPos
}

func (x *Pipe) String() string {
	var out bytes.Buffer
	args := make([]string, 0, len(x.Exprs))
	for _, a := range x.Exprs {
		args = append(args, a.String())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(args, " |> "))
	out.WriteString(")")
	return out.String()
}

// ObjectCall is an expression node that describes the invocation of a method
// on an object.
type ObjectCall struct {
	X        Expr           // object expression
	Period   token.Position // position of "." or "?."
	Call     *Call          // method call
	Optional bool           // true if optional chaining (?.)
}

func (x *ObjectCall) exprNode() {}

func (x *ObjectCall) Pos() token.Position {
	if x.X != nil {
		return x.X.Pos()
	}
	return x.Period
}
func (x *ObjectCall) End() token.Position { return x.Call.End() }

func (x *ObjectCall) String() string {
	var out bytes.Buffer
	out.WriteString(x.X.String())
	if x.Optional {
		out.WriteString("?.")
	} else {
		out.WriteString(".")
	}
	out.WriteString(x.Call.String())
	return out.String()
}

// Index is an expression node that describes indexing on an object.
type Index struct {
	X      Expr           // object expression
	Lbrack token.Position // position of "["
	Index  Expr           // index expression
	Rbrack token.Position // position of "]"
}

func (x *Index) exprNode() {}

func (x *Index) Pos() token.Position {
	if x.X != nil {
		return x.X.Pos()
	}
	return x.Lbrack
}
func (x *Index) End() token.Position { return x.Rbrack.Advance(1) }

func (x *Index) String() string {
	var out bytes.Buffer
	out.WriteString(x.X.String())
	out.WriteString("[")
	out.WriteString(x.Index.String())
	out.WriteString("]")
	return out.String()
}

// Slice is an expression node that describes a slicing operation on an object.
type Slice struct {
	X      Expr           // object expression
	Lbrack token.Position // position of "["
	Low    Expr           // begin of slice range; nil if omitted
	High   Expr           // end of slice range; nil if omitted
	Rbrack token.Position // position of "]"
}

func (x *Slice) exprNode() {}

func (x *Slice) Pos() token.Position {
	if x.X != nil {
		return x.X.Pos()
	}
	return x.Lbrack
}
func (x *Slice) End() token.Position { return x.Rbrack.Advance(1) }

func (x *Slice) String() string {
	var out bytes.Buffer
	out.WriteString(x.X.String())
	out.WriteString("[")
	if x.Low != nil {
		out.WriteString(x.Low.String())
	}
	out.WriteString(":")
	if x.High != nil {
		out.WriteString(x.High.String())
	}
	out.WriteString("]")
	return out.String()
}

// Case is an expression node that describes one case within a switch expression.
type Case struct {
	Case    token.Position // position of "case" or "default" keyword
	Exprs   []Expr         // match expressions; nil for default case
	Colon   token.Position // position of ":"
	Body    *Block         // case body
	Default bool           // true if this is the default case
}

func (x *Case) exprNode() {}

func (x *Case) Pos() token.Position { return x.Case }
func (x *Case) End() token.Position {
	if x.Body != nil {
		return x.Body.End()
	}
	return x.Colon.Advance(1)
}

func (x *Case) String() string {
	var out bytes.Buffer
	if x.Default {
		out.WriteString("default")
	} else {
		out.WriteString("case ")
		tmp := make([]string, 0, len(x.Exprs))
		for _, exp := range x.Exprs {
			tmp = append(tmp, exp.String())
		}
		out.WriteString(strings.Join(tmp, ", "))
	}
	out.WriteString(":\n")
	if x.Body != nil {
		for i, stmt := range x.Body.Stmts {
			if i > 0 {
				out.WriteString("\n")
			}
			out.WriteString("\t" + stmt.String())
		}
	}
	out.WriteString("\n")
	return out.String()
}

// Switch is an expression node that describes a switch between multiple cases.
type Switch struct {
	Switch token.Position // position of "switch" keyword
	Lparen token.Position // position of "("
	Value  Expr           // switch value
	Rparen token.Position // position of ")"
	Lbrace token.Position // position of "{"
	Cases  []*Case        // case clauses
	Rbrace token.Position // position of "}"
}

func (x *Switch) exprNode() {}

func (x *Switch) Pos() token.Position { return x.Switch }
func (x *Switch) End() token.Position { return x.Rbrace.Advance(1) }

func (x *Switch) String() string {
	var out bytes.Buffer
	out.WriteString("switch (")
	out.WriteString(x.Value.String())
	out.WriteString(") {\n")
	for _, choice := range x.Cases {
		if choice != nil {
			out.WriteString(choice.String())
		}
	}
	out.WriteString("}")
	return out.String()
}

// In is an expression node that checks whether a value is present in a container.
type In struct {
	X     Expr           // value to check
	InPos token.Position // position of "in" keyword
	Y     Expr           // container
}

func (x *In) exprNode() {}

func (x *In) Pos() token.Position {
	if x.X != nil {
		return x.X.Pos()
	}
	return x.InPos
}

func (x *In) End() token.Position {
	if x.Y != nil {
		return x.Y.End()
	}
	return x.InPos.Advance(2) // len("in")
}

func (x *In) String() string {
	var out bytes.Buffer
	out.WriteString(x.X.String())
	out.WriteString(" in ")
	out.WriteString(x.Y.String())
	return out.String()
}

// NotIn is an expression node that checks whether a value is NOT present in a container.
type NotIn struct {
	X        Expr           // value to check
	NotInPos token.Position // position of "not" keyword
	Y        Expr           // container
}

func (x *NotIn) exprNode() {}

func (x *NotIn) Pos() token.Position {
	if x.X != nil {
		return x.X.Pos()
	}
	return x.NotInPos
}

func (x *NotIn) End() token.Position {
	if x.Y != nil {
		return x.Y.End()
	}
	return x.NotInPos.Advance(6) // len("not in")
}

func (x *NotIn) String() string {
	var out bytes.Buffer
	out.WriteString(x.X.String())
	out.WriteString(" not in ")
	out.WriteString(x.Y.String())
	return out.String()
}

// Pattern is implemented by all pattern types in match expressions.
type Pattern interface {
	Node
	patternNode()
}

// LiteralPattern matches an expression value. Despite the name, it supports
// any expression (variables, function calls, etc.) that is evaluated at
// runtime and compared for equality with the match subject.
type LiteralPattern struct {
	Value Expr // the expression to evaluate and match against
}

func (p *LiteralPattern) patternNode() {}

func (p *LiteralPattern) Pos() token.Position { return p.Value.Pos() }
func (p *LiteralPattern) End() token.Position { return p.Value.End() }

func (p *LiteralPattern) String() string { return p.Value.String() }

// WildcardPattern matches any value (the _ pattern).
type WildcardPattern struct {
	Underscore token.Position // position of "_"
}

func (p *WildcardPattern) patternNode() {}

func (p *WildcardPattern) Pos() token.Position { return p.Underscore }
func (p *WildcardPattern) End() token.Position { return p.Underscore.Advance(1) }

func (p *WildcardPattern) String() string { return "_" }

// MatchArm represents one arm of a match expression: pattern => result
type MatchArm struct {
	Pattern Pattern        // the pattern to match
	Guard   Expr           // optional guard condition (Phase 4)
	Arrow   token.Position // position of "=>"
	Result  Expr           // the result expression
}

func (a *MatchArm) Pos() token.Position { return a.Pattern.Pos() }
func (a *MatchArm) End() token.Position { return a.Result.End() }

func (a *MatchArm) String() string {
	var out bytes.Buffer
	out.WriteString(a.Pattern.String())
	if a.Guard != nil {
		out.WriteString(" if ")
		out.WriteString(a.Guard.String())
	}
	out.WriteString(" => ")
	out.WriteString(a.Result.String())
	return out.String()
}

// Match is a pattern-matching expression.
type Match struct {
	Match   token.Position // position of "match" keyword
	Subject Expr           // expression being matched
	Lbrace  token.Position // position of "{"
	Arms    []*MatchArm    // match arms (non-default patterns)
	Default *MatchArm      // default arm (_ pattern), required
	Rbrace  token.Position // position of "}"
}

func (x *Match) exprNode() {}

func (x *Match) Pos() token.Position { return x.Match }
func (x *Match) End() token.Position { return x.Rbrace.Advance(1) }

func (x *Match) String() string {
	var out bytes.Buffer
	out.WriteString("match ")
	out.WriteString(x.Subject.String())
	out.WriteString(" { ")
	for i, arm := range x.Arms {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(arm.String())
	}
	if x.Default != nil {
		if len(x.Arms) > 0 {
			out.WriteString(", ")
		}
		out.WriteString(x.Default.String())
	}
	out.WriteString(" }")
	return out.String()
}
