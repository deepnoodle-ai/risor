package ast

import (
	"bytes"
	"strings"

	"github.com/risor-io/risor/internal/token"
)

// Var is a statement that declares a new variable with an initial value.
// This is used for "let x = value" statements.
type Var struct {
	Let   token.Position // position of "let" keyword
	Name  *Ident         // variable name
	Value Expr           // initial value
}

func (x *Var) stmtNode() {}

func (x *Var) Pos() token.Position { return x.Let }
func (x *Var) End() token.Position { return x.Value.End() }

func (x *Var) String() string {
	var out bytes.Buffer
	out.WriteString("let ")
	out.WriteString(x.Name.Name)
	out.WriteString(" = ")
	if x.Value != nil {
		out.WriteString(x.Value.String())
	}
	return out.String()
}

// MultiVar is a statement that declares multiple variables at once.
// This is used for "let x, y = [1, 2]" statements where the right-hand side
// is unpacked into multiple variables.
type MultiVar struct {
	Let   token.Position // position of "let" keyword
	Names []*Ident       // variable names
	Value Expr           // value to unpack
}

func (x *MultiVar) stmtNode() {}

func (x *MultiVar) Pos() token.Position { return x.Let }
func (x *MultiVar) End() token.Position { return x.Value.End() }

func (x *MultiVar) String() string {
	var out bytes.Buffer
	names := make([]string, 0, len(x.Names))
	for _, name := range x.Names {
		names = append(names, name.Name)
	}
	out.WriteString("let ")
	out.WriteString(strings.Join(names, ", "))
	out.WriteString(" = ")
	out.WriteString(x.Value.String())
	return out.String()
}

// DestructureBinding represents a single binding in object destructuring.
// It has a key (property name to extract), an optional alias (local variable name),
// and an optional default value.
type DestructureBinding struct {
	Key     string // property name to extract from object
	Alias   string // local variable name (empty means use Key as name)
	Default Expr   // default value if property is nil (optional)
}

// ObjectDestructure is a statement that extracts properties from an object.
// This is used for "let { a, b } = obj" or "let { a: x, b: y } = obj" statements.
type ObjectDestructure struct {
	Let      token.Position       // position of "let" keyword
	Lbrace   token.Position       // position of "{"
	Bindings []DestructureBinding // bindings to extract
	Rbrace   token.Position       // position of "}"
	Value    Expr                 // value to destructure
}

func (x *ObjectDestructure) stmtNode() {}

func (x *ObjectDestructure) Pos() token.Position { return x.Let }
func (x *ObjectDestructure) End() token.Position { return x.Value.End() }

func (x *ObjectDestructure) String() string {
	var out bytes.Buffer
	out.WriteString("let { ")
	for i, b := range x.Bindings {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(b.Key)
		if b.Alias != "" && b.Alias != b.Key {
			out.WriteString(": ")
			out.WriteString(b.Alias)
		}
		if b.Default != nil {
			out.WriteString(" = ")
			out.WriteString(b.Default.String())
		}
	}
	out.WriteString(" } = ")
	out.WriteString(x.Value.String())
	return out.String()
}

// ArrayDestructureElement represents a single element binding in array destructuring.
type ArrayDestructureElement struct {
	Name    *Ident // variable name to bind
	Default Expr   // default value if element is nil (optional)
}

// ArrayDestructure is a statement that extracts elements from an array.
// This is used for "let [a, b] = arr" or "let [a = 1, b = 2] = arr" statements.
type ArrayDestructure struct {
	Let      token.Position            // position of "let" keyword
	Lbrack   token.Position            // position of "["
	Elements []ArrayDestructureElement // elements to extract
	Rbrack   token.Position            // position of "]"
	Value    Expr                      // value to destructure
}

func (x *ArrayDestructure) stmtNode() {}

func (x *ArrayDestructure) Pos() token.Position { return x.Let }
func (x *ArrayDestructure) End() token.Position { return x.Value.End() }

func (x *ArrayDestructure) String() string {
	var out bytes.Buffer
	out.WriteString("let [")
	for i, e := range x.Elements {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(e.Name.String())
		if e.Default != nil {
			out.WriteString(" = ")
			out.WriteString(e.Default.String())
		}
	}
	out.WriteString("] = ")
	out.WriteString(x.Value.String())
	return out.String()
}

// Const is a statement that defines a named constant.
type Const struct {
	Const token.Position // position of "const" keyword
	Name  *Ident         // constant name
	Value Expr           // constant value
}

func (x *Const) stmtNode() {}

func (x *Const) Pos() token.Position { return x.Const }
func (x *Const) End() token.Position { return x.Value.End() }

func (x *Const) String() string {
	var out bytes.Buffer
	out.WriteString("const ")
	out.WriteString(x.Name.Name)
	out.WriteString(" = ")
	if x.Value != nil {
		out.WriteString(x.Value.String())
	}
	return out.String()
}

// Return defines a return statement.
type Return struct {
	Return token.Position // position of "return" keyword
	Value  Expr           // return value; nil if no value
}

func (x *Return) stmtNode() {}

func (x *Return) Pos() token.Position { return x.Return }
func (x *Return) End() token.Position {
	if x.Value != nil {
		return x.Value.End()
	}
	return x.Return.Advance(6) // len("return")
}

func (x *Return) String() string {
	var out bytes.Buffer
	out.WriteString("return")
	if x.Value != nil {
		out.WriteString(" ")
		out.WriteString(x.Value.String())
	}
	return out.String()
}

// Block is a node that holds a sequence of statements. This is used to
// represent the body of a function, loop, or a conditional.
type Block struct {
	Lbrace token.Position // position of "{"
	Stmts  []Node         // statements in the block
	Rbrace token.Position // position of "}"
}

func (x *Block) stmtNode() {}

func (x *Block) Pos() token.Position { return x.Lbrace }
func (x *Block) End() token.Position { return x.Rbrace.Advance(1) }

// EndsWithReturn returns true if the block ends with a return statement.
func (x *Block) EndsWithReturn() bool {
	if len(x.Stmts) == 0 {
		return false
	}
	_, ok := x.Stmts[len(x.Stmts)-1].(*Return)
	return ok
}

func (x *Block) String() string {
	var out bytes.Buffer
	for i, s := range x.Stmts {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString(s.String())
	}
	return out.String()
}

// Assign is a statement node used to describe a variable assignment.
type Assign struct {
	Name  *Ident         // variable name; nil for index assignment
	Index *Index         // index expression; nil for simple assignment
	OpPos token.Position // position of operator
	Op    string         // assignment operator: "=", "+=", "-=", etc.
	Value Expr           // value to assign
}

func (x *Assign) stmtNode() {}

func (x *Assign) Pos() token.Position {
	if x.Name != nil {
		return x.Name.Pos()
	}
	return x.Index.Pos()
}
func (x *Assign) End() token.Position { return x.Value.End() }

func (x *Assign) String() string {
	var out bytes.Buffer
	if x.Index != nil {
		out.WriteString(x.Index.String())
	} else {
		out.WriteString(x.Name.Name)
	}
	out.WriteString(" ")
	out.WriteString(x.Op)
	out.WriteString(" ")
	out.WriteString(x.Value.String())
	return out.String()
}

// Postfix is a statement node that describes a postfix expression like "x++".
// The operand X can be an Ident, Index, or GetAttr expression.
type Postfix struct {
	X     Expr           // operand (must be assignable: Ident, Index, or GetAttr)
	OpPos token.Position // position of operator
	Op    string         // operator: "++", "--"
}

func (x *Postfix) stmtNode() {}

func (x *Postfix) Pos() token.Position { return x.X.Pos() }
func (x *Postfix) End() token.Position { return x.OpPos.Advance(2) } // len("++") or len("--")

func (x *Postfix) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(x.X.String())
	out.WriteString(x.Op)
	out.WriteString(")")
	return out.String()
}

// SetAttr is a statement node that describes setting an attribute on an object.
type SetAttr struct {
	X      Expr           // object expression
	Period token.Position // position of "."
	Attr   *Ident         // attribute name
	OpPos  token.Position // position of operator
	Op     string         // assignment operator: "=", "+=", "-=", "*=", "/="
	Value  Expr           // value to set
}

func (x *SetAttr) stmtNode() {}

func (x *SetAttr) Pos() token.Position { return x.X.Pos() }
func (x *SetAttr) End() token.Position { return x.Value.End() }

func (x *SetAttr) String() string {
	var out bytes.Buffer
	out.WriteString(x.X.String())
	out.WriteString(".")
	out.WriteString(x.Attr.Name)
	out.WriteString(" ")
	out.WriteString(x.Op)
	out.WriteString(" ")
	out.WriteString(x.Value.String())
	return out.String()
}

// Try represents a try/catch/finally statement.
type Try struct {
	Try          token.Position // position of "try" keyword
	Body         *Block         // try block
	Catch        token.Position // position of "catch" keyword; zero if no catch
	CatchIdent   *Ident         // catch variable; nil if "catch { }"
	CatchBlock   *Block         // catch block; nil if no catch
	Finally      token.Position // position of "finally" keyword; zero if no finally
	FinallyBlock *Block         // finally block; nil if no finally
}

func (x *Try) stmtNode() {}
func (x *Try) exprNode() {} // try is also an expression

func (x *Try) Pos() token.Position { return x.Try }

func (x *Try) End() token.Position {
	if x.FinallyBlock != nil {
		return x.FinallyBlock.End()
	}
	if x.CatchBlock != nil {
		return x.CatchBlock.End()
	}
	return x.Body.End()
}

func (x *Try) String() string {
	var out bytes.Buffer
	out.WriteString("try ")
	out.WriteString(x.Body.String())
	if x.CatchBlock != nil {
		out.WriteString(" catch ")
		if x.CatchIdent != nil {
			out.WriteString(x.CatchIdent.String())
			out.WriteString(" ")
		}
		out.WriteString(x.CatchBlock.String())
	}
	if x.FinallyBlock != nil {
		out.WriteString(" finally ")
		out.WriteString(x.FinallyBlock.String())
	}
	return out.String()
}

// Throw represents a throw statement.
type Throw struct {
	Throw token.Position // position of "throw" keyword
	Value Expr           // value to throw
}

func (x *Throw) stmtNode() {}

func (x *Throw) Pos() token.Position { return x.Throw }
func (x *Throw) End() token.Position {
	if x.Value != nil {
		return x.Value.End()
	}
	return x.Throw.Advance(5) // len("throw")
}

func (x *Throw) String() string {
	var out bytes.Buffer
	out.WriteString("throw ")
	if x.Value != nil {
		out.WriteString(x.Value.String())
	}
	return out.String()
}
