package ast

import (
	"bytes"
	"strings"

	"github.com/risor-io/risor/token"
)

// Ident is an expression node that refers to a variable by name.
type Ident struct {
	token token.Token
	value string
}

// NewIdent creates a new Ident node.
func NewIdent(token token.Token) *Ident {
	return &Ident{token: token, value: token.Literal}
}

func (i *Ident) ExpressionNode() {}

func (i *Ident) IsExpression() bool { return true }

func (i *Ident) Token() token.Token { return i.token }

func (i *Ident) Literal() string { return i.value }

func (i *Ident) String() string { return i.value }

// Prefix is an operator expression where the operator precedes the operand.
// Examples include "!false" and "-x".
type Prefix struct {
	token token.Token

	// operator holds the operator being invoked (e.g. "!")
	operator string

	// right holds the thing to be operated upon
	right Expression
}

// NewPrefix creates a new Prefix node.
func NewPrefix(token token.Token, right Expression) *Prefix {
	return &Prefix{token: token, operator: token.Literal, right: right}
}

func (p *Prefix) ExpressionNode() {}

func (p *Prefix) IsExpression() bool { return true }

func (p *Prefix) Token() token.Token { return p.token }

func (p *Prefix) Literal() string { return p.token.Literal }

func (p *Prefix) Operator() string { return p.operator }

func (p *Prefix) Right() Expression { return p.right }

func (p *Prefix) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(p.operator)
	out.WriteString(p.right.String())
	out.WriteString(")")
	return out.String()
}

// Spread represents a spread expression (...expr) used in array literals,
// object literals, and function calls. Also used for rest parameters.
type Spread struct {
	token token.Token
	value Expression
}

// NewSpread creates a new Spread node.
func NewSpread(token token.Token, value Expression) *Spread {
	return &Spread{token: token, value: value}
}

func (s *Spread) ExpressionNode() {}

func (s *Spread) IsExpression() bool { return true }

func (s *Spread) Token() token.Token { return s.token }

func (s *Spread) Literal() string { return s.token.Literal }

func (s *Spread) Value() Expression { return s.value }

func (s *Spread) String() string {
	var out bytes.Buffer
	out.WriteString("...")
	if s.value != nil {
		out.WriteString(s.value.String())
	}
	return out.String()
}

// Infix is an operator expression where the operator is between the operands.
// Examples include "x + y" and "5 - 1".
type Infix struct {
	token token.Token

	// left side expression
	left Expression

	// operator e.g. "+", "-", "==", etc.
	operator string

	// right side expression
	right Expression
}

// NewInfix creates a new Infix node.
func NewInfix(token token.Token, left Expression, operator string, right Expression) *Infix {
	return &Infix{token: token, left: left, operator: operator, right: right}
}

func (i *Infix) ExpressionNode() {}

func (i *Infix) IsExpression() bool { return true }

func (i *Infix) Token() token.Token { return i.token }

func (i *Infix) Literal() string { return i.token.Literal }

func (i *Infix) Left() Expression { return i.left }

func (i *Infix) Operator() string { return i.operator }

func (i *Infix) Right() Expression { return i.right }

func (i *Infix) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(i.left.String())
	out.WriteString(" " + i.operator + " ")
	out.WriteString(i.right.String())
	out.WriteString(")")
	return out.String()
}

// If is an expression node that represents an if/else expression
type If struct {
	token       token.Token // the "if" token
	condition   Expression  // the condition to be evaluated
	consequence *Block      // block to evaluate if the condition is true
	alternative *Block      // block to evaluate if the condition is false
}

// NewIf creates a new If node.
func NewIf(token token.Token, condition Expression, consequence *Block, alternative *Block) *If {
	return &If{token: token, condition: condition, consequence: consequence, alternative: alternative}
}

func (i *If) ExpressionNode() {}

func (i *If) IsExpression() bool { return true }

func (i *If) Token() token.Token { return i.token }

func (i *If) Literal() string { return i.token.Literal }

func (i *If) Condition() Expression { return i.condition }

func (i *If) Consequence() *Block { return i.consequence }

func (i *If) Alternative() *Block { return i.alternative }

func (i *If) String() string {
	var out bytes.Buffer
	out.WriteString("if ")
	out.WriteString(i.condition.String())
	out.WriteString(" ")
	out.WriteString(i.consequence.String())
	if i.alternative != nil {
		out.WriteString(" else ")
		out.WriteString(i.alternative.String())
	}
	return out.String()
}

// Ternary is an expression node that defines a ternary expression and evaluates
// to one of two values based on a condition.
type Ternary struct {
	token token.Token

	// condition is the thing that is evaluated to determine
	// which expression should be returned
	condition Expression

	// ifTrue is the expression to return if the condition is true.
	ifTrue Expression

	// ifFalse is the expression to return if the condition is not true.
	ifFalse Expression
}

// NewTernary creates a new Ternary node.
func NewTernary(token token.Token, condition Expression, ifTrue Expression, ifFalse Expression) *Ternary {
	return &Ternary{token: token, condition: condition, ifTrue: ifTrue, ifFalse: ifFalse}
}

func (t *Ternary) ExpressionNode() {}

func (t *Ternary) IsExpression() bool { return true }

func (t *Ternary) Token() token.Token { return t.token }

func (t *Ternary) Literal() string { return t.token.Literal }

func (t *Ternary) Condition() Expression { return t.condition }

func (t *Ternary) IfTrue() Expression { return t.ifTrue }

func (t *Ternary) IfFalse() Expression { return t.ifFalse }

func (t *Ternary) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(t.condition.String())
	out.WriteString(" ? ")
	out.WriteString(t.ifTrue.String())
	out.WriteString(" : ")
	out.WriteString(t.ifFalse.String())
	out.WriteString(")")
	return out.String()
}

// Call is an expression node that describes the invocation of a function.
type Call struct {
	token     token.Token // the '(' token
	function  Expression  // the function being called
	arguments []Node      // the arguments supplied to the call
}

// NewCall creates a new Call node.
func NewCall(token token.Token, function Expression, arguments []Node) *Call {
	return &Call{token: token, function: function, arguments: arguments}
}

func (c *Call) ExpressionNode() {}

func (c *Call) IsExpression() bool { return true }

func (c *Call) Token() token.Token { return c.token }

func (c *Call) Literal() string { return c.token.Literal }

func (c *Call) Function() Expression { return c.function }

func (c *Call) Arguments() []Node { return c.arguments }

func (c *Call) String() string {
	var out bytes.Buffer
	args := make([]string, 0)
	for _, a := range c.arguments {
		args = append(args, a.String())
	}
	out.WriteString(c.function.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

// GetAttr is an expression node that describes the access of an attribute on
// an object.
type GetAttr struct {
	token token.Token

	// object whose attribute is being accessed
	object Expression

	// The attribute itself
	attribute *Ident

	// optional indicates this is optional chaining (?.)
	optional bool
}

// NewGetAttr creates a new GetAttr node.
func NewGetAttr(token token.Token, object Expression, attribute *Ident) *GetAttr {
	return &GetAttr{token: token, object: object, attribute: attribute, optional: false}
}

// NewOptionalGetAttr creates a new GetAttr node for optional chaining (?.).
func NewOptionalGetAttr(token token.Token, object Expression, attribute *Ident) *GetAttr {
	return &GetAttr{token: token, object: object, attribute: attribute, optional: true}
}

// IsOptional returns true if this is optional chaining (?.).
func (e *GetAttr) IsOptional() bool { return e.optional }

func (e *GetAttr) ExpressionNode() {}

func (e *GetAttr) IsExpression() bool { return true }

func (e *GetAttr) Token() token.Token { return e.token }

func (e *GetAttr) Literal() string { return e.token.Literal }

func (e *GetAttr) Object() Expression { return e.object }

func (e *GetAttr) Name() string { return e.attribute.value }

func (e *GetAttr) String() string {
	var out bytes.Buffer
	out.WriteString(e.object.String())
	if e.optional {
		out.WriteString("?.")
	} else {
		out.WriteString(".")
	}
	out.WriteString(e.attribute.value)
	return out.String()
}

// Pipe is an expression node that describes a sequence of transformations
// applied to an initial value.
type Pipe struct {
	token token.Token

	// exprs contains the pipe separated expressions
	exprs []Expression
}

// NewPipe creates a new Pipe node.
func NewPipe(token token.Token, exprs []Expression) *Pipe {
	return &Pipe{token: token, exprs: exprs}
}

func (p *Pipe) ExpressionNode() {}

func (p *Pipe) IsExpression() bool { return true }

func (p *Pipe) Token() token.Token { return p.token }

func (p *Pipe) Literal() string { return p.token.Literal }

func (p *Pipe) Expressions() []Expression { return p.exprs }

func (p *Pipe) String() string {
	var out bytes.Buffer
	args := make([]string, 0)
	for _, a := range p.exprs {
		args = append(args, a.String())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(args, " | "))
	out.WriteString(")")
	return out.String()
}

// ObjectCall is an expression node that describes the invocation of a method
// on an object.
type ObjectCall struct {
	token    token.Token
	object   Expression
	call     Expression
	optional bool
}

// NewObjectCall creates a new ObjectCall node.
func NewObjectCall(token token.Token, object Expression, call Expression) *ObjectCall {
	return &ObjectCall{token: token, object: object, call: call, optional: false}
}

// NewOptionalObjectCall creates a new ObjectCall node for optional chaining (?.).
func NewOptionalObjectCall(token token.Token, object Expression, call Expression) *ObjectCall {
	return &ObjectCall{token: token, object: object, call: call, optional: true}
}

func (c *ObjectCall) ExpressionNode() {}

func (c *ObjectCall) IsExpression() bool { return true }

func (c *ObjectCall) Token() token.Token { return c.token }

func (c *ObjectCall) Literal() string { return c.token.Literal }

func (c *ObjectCall) Object() Expression { return c.object }

func (c *ObjectCall) Call() Expression { return c.call }

func (c *ObjectCall) IsOptional() bool { return c.optional }

func (c *ObjectCall) String() string {
	var out bytes.Buffer
	out.WriteString(c.object.String())
	if c.optional {
		out.WriteString("?.")
	} else {
		out.WriteString(".")
	}
	out.WriteString(c.call.String())
	return out.String()
}

// Index as an expression node that describes indexing on an object.
type Index struct {
	token token.Token

	// left is the container being indexed.
	left Expression

	// index is the value used to index the container.
	index Expression
}

// NewIndex creates a new Index node.
func NewIndex(token token.Token, left Expression, index Expression) *Index {
	return &Index{token: token, left: left, index: index}
}

func (i *Index) ExpressionNode() {}

func (i *Index) IsExpression() bool { return true }

func (i *Index) Token() token.Token { return i.token }

func (i *Index) Literal() string { return i.token.Literal }

func (i *Index) Left() Expression { return i.left }

func (i *Index) Index() Expression { return i.index }

func (i *Index) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(i.left.String())
	out.WriteString("[")
	out.WriteString(i.index.String())
	out.WriteString("])")
	return out.String()
}

// Slice is an expression node that describes a slicing operation on an object.
type Slice struct {
	token token.Token

	// left is the thing being indexed.
	left Expression

	// Optional "from" index for [from:to] style expressions
	fromIndex Expression

	// Optional "to" index for [from:to] style expressions
	toIndex Expression
}

// NewSlice creates a new Slice node.
func NewSlice(token token.Token, left Expression, fromIndex Expression, toIndex Expression) *Slice {
	return &Slice{token: token, left: left, fromIndex: fromIndex, toIndex: toIndex}
}

func (s *Slice) ExpressionNode() {}

func (s *Slice) IsExpression() bool { return true }

func (s *Slice) Token() token.Token { return s.token }

func (s *Slice) Literal() string { return s.token.Literal }

func (s *Slice) Left() Expression { return s.left }

func (s *Slice) FromIndex() Expression { return s.fromIndex }

func (s *Slice) ToIndex() Expression { return s.toIndex }

func (s *Slice) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(s.left.String())
	out.WriteString("[")
	if s.fromIndex != nil {
		out.WriteString(s.fromIndex.String())
	}
	if s.toIndex != nil {
		out.WriteString(":")
		out.WriteString(s.toIndex.String())
	}
	out.WriteString("])")
	return out.String()
}

// Case is an expression node that describes one case within a switch expression.
type Case struct {
	token token.Token

	// Default branch?
	isDefault bool

	// The thing we match
	expr []Expression

	// The code to execute if there is a match
	block *Block
}

// NewCase creates a new Case node.
func NewCase(token token.Token, expressions []Expression, block *Block) *Case {
	return &Case{token: token, expr: expressions, block: block}
}

// NewDefaultCase represents the default case within a switch expression.
func NewDefaultCase(token token.Token, block *Block) *Case {
	return &Case{token: token, isDefault: true, block: block}
}

func (c *Case) ExpressionNode() {}

func (c *Case) IsExpression() bool { return true }

func (c *Case) Token() token.Token { return c.token }

func (c *Case) Literal() string { return c.token.Literal }

func (c *Case) IsDefault() bool { return c.isDefault }

func (c *Case) Expressions() []Expression { return c.expr }

func (c *Case) Block() *Block { return c.block }

func (c *Case) String() string {
	var out bytes.Buffer
	if c.isDefault {
		out.WriteString("default")
	} else {
		out.WriteString("case ")
		tmp := []string{}
		for _, exp := range c.expr {
			tmp = append(tmp, exp.String())
		}
		out.WriteString(strings.Join(tmp, ","))
	}
	out.WriteString(":\n")
	if c.block != nil {
		for i, exp := range c.block.statements {
			if i > 0 {
				out.WriteString("\n")
			}
			out.WriteString("\t" + exp.String())
		}
	}
	out.WriteString("\n")
	return out.String()
}

// Switch is an expression node that describes a switch between multiple cases.
type Switch struct {
	// token containing "switch"
	token token.Token

	// the expression to switch on
	value Expression

	// switch cases
	choices []*Case
}

// NewSwitch creates a new Switch node.
func NewSwitch(token token.Token, value Expression, choices []*Case) *Switch {
	return &Switch{token: token, value: value, choices: choices}
}

func (s *Switch) ExpressionNode() {}

func (s *Switch) IsExpression() bool { return true }

func (s *Switch) Token() token.Token { return s.token }

func (s *Switch) Literal() string { return s.token.Literal }

func (s *Switch) Value() Expression { return s.value }

func (s *Switch) Choices() []*Case { return s.choices }

func (s *Switch) String() string {
	var out bytes.Buffer
	out.WriteString("\nswitch ")
	out.WriteString(s.value.String())
	out.WriteString(" {\n")
	for _, choice := range s.choices {
		if choice != nil {
			out.WriteString(choice.String())
		}
	}
	out.WriteString("}\n")
	return out.String()
}

// In is an expression node that checks whether a value is present in a container.
type In struct {
	token token.Token
	left  Expression
	right Expression
}

// NewIn creates a new In node.
func NewIn(token token.Token, left Expression, right Expression) *In {
	return &In{token: token, left: left, right: right}
}

func (i *In) ExpressionNode() {}

func (i *In) IsExpression() bool { return true }

func (i *In) Token() token.Token { return i.token }

func (i *In) Literal() string { return i.token.Literal }

func (i *In) Left() Expression { return i.left }

func (i *In) Right() Expression { return i.right }

func (i *In) String() string {
	var out bytes.Buffer
	out.WriteString(i.left.String())
	out.WriteString(" in ")
	out.WriteString(i.right.String())
	return out.String()
}

// NotIn is an expression node that checks whether a value is NOT present in a container.
type NotIn struct {
	token token.Token
	left  Expression
	right Expression
}

// NewNotIn creates a new NotIn node.
func NewNotIn(token token.Token, left Expression, right Expression) *NotIn {
	return &NotIn{token: token, left: left, right: right}
}

func (n *NotIn) ExpressionNode() {}

func (n *NotIn) IsExpression() bool { return true }

func (n *NotIn) Token() token.Token { return n.token }

func (n *NotIn) Literal() string { return n.token.Literal }

func (n *NotIn) Left() Expression { return n.left }

func (n *NotIn) Right() Expression { return n.right }

func (n *NotIn) String() string {
	var out bytes.Buffer
	out.WriteString(n.left.String())
	out.WriteString(" not in ")
	out.WriteString(n.right.String())
	return out.String()
}

// Range is an expression node that describes iterating over a container.
type Range struct {
	// the "range" token
	token token.Token

	// the container to iterate over
	container Node
}

// NewRange creates a new Range node.
func NewRange(token token.Token, container Node) *Range {
	return &Range{token: token, container: container}
}

func (r *Range) ExpressionNode() {}

func (r *Range) IsExpression() bool { return true }

func (r *Range) Token() token.Token { return r.token }

func (r *Range) Literal() string { return r.token.Literal }

func (r *Range) Container() Node { return r.container }

func (r *Range) String() string {
	var out bytes.Buffer
	out.WriteString(r.Literal())
	out.WriteString(" ")
	out.WriteString(r.container.String())
	return out.String()
}
