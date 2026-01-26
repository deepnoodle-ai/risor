package ast

import (
	"bytes"
	"strings"

	"github.com/risor-io/risor/internal/token"
)

// Var is a statement that declares a new variable with an initial value.
// This is used for "let x = value" statements.
type Var struct {
	token token.Token

	// name is the name of the variable being declared
	name *Ident

	// value is the initial value of the variable
	value Expression
}

// NewVar creates a new Var node for a variable declaration.
func NewVar(token token.Token, name *Ident, value Expression) *Var {
	return &Var{token: token, name: name, value: value}
}

func (s *Var) StatementNode() {}

func (s *Var) IsExpression() bool { return false }

func (s *Var) Token() token.Token { return s.token }

func (s *Var) Literal() string { return s.token.Literal }

func (s *Var) Value() (string, Expression) { return s.name.value, s.value }

func (s *Var) String() string {
	var out bytes.Buffer
	out.WriteString(s.Literal() + " ")
	out.WriteString(s.name.Literal())
	out.WriteString(" = ")
	if s.value != nil {
		out.WriteString(s.value.String())
	}
	return out.String()
}

// MultiVar is a statement that declares multiple variables at once.
// This is used for "let x, y = [1, 2]" statements where the right-hand side
// is unpacked into multiple variables.
type MultiVar struct {
	token token.Token
	names []*Ident   // names being declared
	value Expression // value to unpack into the variables
}

// NewMultiVar creates a new MultiVar node for a multi-variable declaration.
func NewMultiVar(token token.Token, names []*Ident, value Expression) *MultiVar {
	return &MultiVar{token: token, names: names, value: value}
}

func (s *MultiVar) StatementNode() {}

func (s *MultiVar) IsExpression() bool { return false }

func (s *MultiVar) Token() token.Token { return s.token }

func (s *MultiVar) Literal() string { return s.token.Literal }

func (s *MultiVar) Value() ([]string, Expression) {
	names := make([]string, 0, len(s.names))
	for _, name := range s.names {
		names = append(names, name.value)
	}
	return names, s.value
}

func (s *MultiVar) String() string {
	names, expr := s.Value()
	namesStr := strings.Join(names, ", ")
	var out bytes.Buffer
	// Use the token literal (e.g., "let") for declarations
	out.WriteString(s.Literal() + " ")
	out.WriteString(namesStr)
	out.WriteString(" = ")
	out.WriteString(expr.String())
	return out.String()
}

// DestructureBinding represents a single binding in object destructuring.
// It has a key (property name to extract), an optional alias (local variable name),
// and an optional default value.
type DestructureBinding struct {
	Key     string     // Property name to extract from object
	Alias   string     // Local variable name (empty means use Key as name)
	Default Expression // Default value if property is nil (optional)
}

// ObjectDestructure is a statement that extracts properties from an object.
// This is used for "let { a, b } = obj" or "let { a: x, b: y } = obj" statements.
type ObjectDestructure struct {
	token    token.Token
	bindings []DestructureBinding
	value    Expression
}

// NewObjectDestructure creates a new ObjectDestructure node.
func NewObjectDestructure(token token.Token, bindings []DestructureBinding, value Expression) *ObjectDestructure {
	return &ObjectDestructure{token: token, bindings: bindings, value: value}
}

func (s *ObjectDestructure) StatementNode() {}

func (s *ObjectDestructure) IsExpression() bool { return false }

func (s *ObjectDestructure) Token() token.Token { return s.token }

func (s *ObjectDestructure) Literal() string { return s.token.Literal }

func (s *ObjectDestructure) Bindings() []DestructureBinding { return s.bindings }

func (s *ObjectDestructure) Value() Expression { return s.value }

func (s *ObjectDestructure) String() string {
	var out bytes.Buffer
	out.WriteString(s.Literal() + " { ")
	for i, b := range s.bindings {
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
	out.WriteString(s.value.String())
	return out.String()
}

// ArrayDestructureElement represents a single element binding in array destructuring.
type ArrayDestructureElement struct {
	Name    *Ident     // Variable name to bind
	Default Expression // Default value if element is nil (optional)
}

// ArrayDestructure is a statement that extracts elements from an array.
// This is used for "let [a, b] = arr" or "let [a = 1, b = 2] = arr" statements.
type ArrayDestructure struct {
	token    token.Token
	elements []ArrayDestructureElement // elements to destructure
	value    Expression                // the array to destructure
}

// NewArrayDestructure creates a new ArrayDestructure node.
func NewArrayDestructure(token token.Token, elements []ArrayDestructureElement, value Expression) *ArrayDestructure {
	return &ArrayDestructure{token: token, elements: elements, value: value}
}

func (s *ArrayDestructure) StatementNode() {}

func (s *ArrayDestructure) IsExpression() bool { return false }

func (s *ArrayDestructure) Token() token.Token { return s.token }

func (s *ArrayDestructure) Literal() string { return s.token.Literal }

func (s *ArrayDestructure) Elements() []ArrayDestructureElement { return s.elements }

func (s *ArrayDestructure) Value() Expression { return s.value }

func (s *ArrayDestructure) String() string {
	var out bytes.Buffer
	out.WriteString(s.Literal() + " [")
	for i, e := range s.elements {
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
	out.WriteString(s.value.String())
	return out.String()
}

// Const is a statement that defines a named constant.
type Const struct {
	// the "const" token
	token token.Token

	// name of the constant
	name *Ident

	// value of the constant
	value Expression
}

// NewConst creates a new Const node.
func NewConst(token token.Token, name *Ident, value Expression) *Const {
	return &Const{token: token, name: name, value: value}
}

func (c *Const) StatementNode() {}

func (c *Const) IsExpression() bool { return false }

func (c *Const) Token() token.Token { return c.token }

func (c *Const) Literal() string { return c.token.Literal }

func (c *Const) Value() (string, Expression) { return c.name.value, c.value }

func (c *Const) String() string {
	var out bytes.Buffer
	out.WriteString(c.Literal() + " ")
	out.WriteString(c.name.Literal())
	out.WriteString(" = ")
	if c.value != nil {
		out.WriteString(c.value.String())
	}
	return out.String()
}

// Return defines a return statement.
type Return struct {
	// "return"
	token token.Token

	// optional value
	value Expression
}

// NewReturn creates a new Return node.
func NewReturn(token token.Token, value Expression) *Return {
	return &Return{token: token, value: value}
}

func (r *Return) StatementNode() {}

func (r *Return) IsExpression() bool { return false }

func (r *Return) Token() token.Token { return r.token }

func (r *Return) Literal() string { return r.token.Literal }

func (r *Return) Value() Expression { return r.value }

func (r *Return) String() string {
	var out bytes.Buffer
	out.WriteString(r.Literal())
	if r.value != nil {
		out.WriteString(" " + r.value.String())
	}
	return out.String()
}

// Block is a node that holds a sequence of statements. This is used to
// represent the body of a function, loop, or a conditional.
type Block struct {
	token      token.Token // the opening "{" token
	statements []Node      // the statements in the block
}

// NewBlock creates a new Block node.
func NewBlock(token token.Token, statements []Node) *Block {
	return &Block{token: token, statements: statements}
}

func (b *Block) StatementNode() {}

func (b *Block) IsExpression() bool { return false }

func (b *Block) Token() token.Token { return b.token }

func (b *Block) Literal() string { return b.token.Literal }

func (b *Block) Statements() []Node { return b.statements }

func (b *Block) EndsWithReturn() bool {
	count := len(b.statements)
	if count == 0 {
		return false
	}
	last := b.statements[count-1]
	_, isReturn := last.(*Return)
	return isReturn
}

func (b *Block) String() string {
	var out bytes.Buffer
	for i, s := range b.statements {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString(s.String())
	}
	return out.String()
}

// Assign is a statement node used to describe a variable assignment.
type Assign struct {
	token    token.Token
	name     *Ident // this may be nil, e.g. `[0, 1, 2][0] = 3`
	index    *Index
	operator string
	value    Expression
}

// NewAssign creates a new Assign node.
func NewAssign(operator token.Token, name *Ident, value Expression) *Assign {
	return &Assign{token: operator, name: name, operator: operator.Literal, value: value}
}

// NewAssignIndex creates a new Assign node for an index assignment.
func NewAssignIndex(operator token.Token, index *Index, value Expression) *Assign {
	return &Assign{token: operator, index: index, operator: operator.Literal, value: value}
}

func (a *Assign) StatementNode() {}

func (a *Assign) IsExpression() bool { return false }

func (a *Assign) Token() token.Token { return a.token }

func (a *Assign) Literal() string { return a.token.Literal }

func (a *Assign) Name() string { return a.name.value }

func (a *Assign) NameIdent() *Ident { return a.name }

func (a *Assign) Index() *Index { return a.index }

func (a *Assign) Operator() string { return a.operator }

func (a *Assign) Value() Expression { return a.value }

func (a *Assign) String() string {
	var out bytes.Buffer
	if a.index != nil {
		out.WriteString(a.index.String())
	} else {
		out.WriteString(a.name.value)
	}
	out.WriteString(" " + a.operator + " ")
	out.WriteString(a.value.String())
	return out.String()
}

// Postfix is a statement node that describes a postfix expression like "x++".
type Postfix struct {
	token token.Token
	// operator holds the postfix token, e.g. ++
	operator string
}

// NewPostfix creates a new Postfix node.
func NewPostfix(token token.Token, operator string) *Postfix {
	return &Postfix{token: token, operator: operator}
}

func (p *Postfix) StatementNode() {}

func (p *Postfix) IsExpression() bool { return false }

func (p *Postfix) Token() token.Token { return p.token }

func (p *Postfix) Literal() string { return p.token.Literal }

func (p *Postfix) Operator() string { return p.operator }

func (p *Postfix) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(p.token.Literal)
	out.WriteString(p.operator)
	out.WriteString(")")
	return out.String()
}

// SetAttr is a statement node that describes setting an attribute on an object.
type SetAttr struct {
	token token.Token

	// object whose attribute is being accessed
	object Expression

	// The attribute itself
	attribute *Ident

	// The value for the attribute
	value Expression
}

// NewSetAttr creates a new SetAttr node.
func NewSetAttr(token token.Token, object Expression, attribute *Ident, value Expression) *SetAttr {
	return &SetAttr{token: token, object: object, attribute: attribute, value: value}
}

func (p *SetAttr) StatementNode() {}

func (e *SetAttr) IsExpression() bool { return false }

func (e *SetAttr) Token() token.Token { return e.token }

func (e *SetAttr) Literal() string { return e.token.Literal }

func (e *SetAttr) Object() Expression { return e.object }

func (e *SetAttr) Name() string { return e.attribute.value }

func (e *SetAttr) Value() Expression { return e.value }

func (e *SetAttr) String() string {
	var out bytes.Buffer
	out.WriteString(e.object.String())
	out.WriteString(".")
	out.WriteString(e.attribute.value)
	out.WriteString(" = ")
	out.WriteString(e.value.String())
	return out.String()
}

// Try represents a try/catch/finally statement.
type Try struct {
	token        token.Token // "try" token
	body         *Block      // try block
	catchIdent   *Ident      // catch variable (nil if `catch { }`)
	catchBlock   *Block      // catch block (nil if no catch)
	finallyBlock *Block      // finally block (nil if no finally)
}

// NewTry creates a new Try node.
func NewTry(token token.Token, body *Block, catchIdent *Ident, catchBlock *Block, finallyBlock *Block) *Try {
	return &Try{
		token:        token,
		body:         body,
		catchIdent:   catchIdent,
		catchBlock:   catchBlock,
		finallyBlock: finallyBlock,
	}
}

func (t *Try) StatementNode() {}

func (t *Try) IsExpression() bool { return true }

func (t *Try) Token() token.Token { return t.token }

func (t *Try) Literal() string { return t.token.Literal }

func (t *Try) Body() *Block { return t.body }

func (t *Try) CatchIdent() *Ident { return t.catchIdent }

func (t *Try) CatchBlock() *Block { return t.catchBlock }

func (t *Try) FinallyBlock() *Block { return t.finallyBlock }

func (t *Try) String() string {
	var out bytes.Buffer
	out.WriteString("try ")
	out.WriteString(t.body.String())
	if t.catchBlock != nil {
		out.WriteString(" catch ")
		if t.catchIdent != nil {
			out.WriteString(t.catchIdent.String())
			out.WriteString(" ")
		}
		out.WriteString(t.catchBlock.String())
	}
	if t.finallyBlock != nil {
		out.WriteString(" finally ")
		out.WriteString(t.finallyBlock.String())
	}
	return out.String()
}

// Throw represents a throw statement.
type Throw struct {
	token token.Token
	value Expression
}

// NewThrow creates a new Throw node.
func NewThrow(token token.Token, value Expression) *Throw {
	return &Throw{token: token, value: value}
}

func (t *Throw) StatementNode() {}

func (t *Throw) IsExpression() bool { return false }

func (t *Throw) Token() token.Token { return t.token }

func (t *Throw) Literal() string { return t.token.Literal }

func (t *Throw) Value() Expression { return t.value }

func (t *Throw) String() string {
	var out bytes.Buffer
	out.WriteString("throw ")
	if t.value != nil {
		out.WriteString(t.value.String())
	}
	return out.String()
}
