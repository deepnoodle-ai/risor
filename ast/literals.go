package ast

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/risor-io/risor/internal/tmpl"
	"github.com/risor-io/risor/internal/token"
)

// Int is an expression node that holds an integer literal.
type Int struct {
	token token.Token // the token containing the number
	value int64       // the value of the int
}

// NewInt creates a new Int node.
func NewInt(token token.Token, value int64) *Int {
	return &Int{token: token, value: value}
}

func (i *Int) ExpressionNode() {}

func (i *Int) IsExpression() bool { return true }

func (i *Int) Token() token.Token { return i.token }

func (i *Int) Literal() string { return i.token.Literal }

func (i *Int) Value() int64 { return i.value }

func (i *Int) String() string { return i.token.Literal }

// Float is an expression node that holds a floating point literal.
type Float struct {
	token token.Token // the token containing the number
	value float64     // the value of the float
}

// NewFloat creates a new Float node.
func NewFloat(token token.Token, value float64) *Float {
	return &Float{token: token, value: value}
}

func (f *Float) ExpressionNode() {}

func (f *Float) IsExpression() bool { return true }

func (f *Float) Token() token.Token { return f.token }

func (f *Float) Literal() string { return f.token.Literal }

func (f *Float) Value() float64 { return f.value }

func (f *Float) String() string { return f.token.Literal }

// Nil is an expression node that holds a nil literal.
type Nil struct {
	token token.Token // token containing "nil"
}

// NewNil creates a new Nil node.
func NewNil(token token.Token) *Nil {
	return &Nil{token: token}
}

func (n *Nil) ExpressionNode() {}

func (n *Nil) IsExpression() bool { return true }

func (n *Nil) Token() token.Token { return n.token }

func (n *Nil) Literal() string { return n.token.Literal }

func (n *Nil) String() string { return n.token.Literal }

// Bool is an expression node that holds a boolean literal.
type Bool struct {
	// Token holds the actual token
	token token.Token

	// value stores the bools' value: true, or false.
	value bool
}

// NewBool creates a new Bool node.
func NewBool(token token.Token, value bool) *Bool {
	return &Bool{token: token, value: value}
}

func (b *Bool) ExpressionNode() {}

func (b *Bool) IsExpression() bool { return true }

func (b *Bool) Token() token.Token { return b.token }

func (b *Bool) Literal() string { return b.token.Literal }

func (b *Bool) Value() bool { return b.value }

func (b *Bool) String() string { return b.token.Literal }

// Func is an expression node that holds a function literal.
type Func struct {
	token token.Token

	name *Ident

	// parameters is the list of parameters the function receives.
	parameters []*Ident

	// defaults holds any default values for arguments which aren't specified.
	defaults map[string]Expression

	// restParam is the rest parameter (e.g., ...args) that collects remaining arguments.
	// Nil if the function doesn't have a rest parameter.
	restParam *Ident

	// body contains the set of statements within the function.
	body *Block
}

// NewFunc creates a new Func node.
func NewFunc(token token.Token, name *Ident, parameters []*Ident, defaults map[string]Expression, restParam *Ident, body *Block) *Func {
	return &Func{
		token:      token,
		name:       name,
		parameters: parameters,
		defaults:   defaults,
		restParam:  restParam,
		body:       body,
	}
}

// RestParam returns the rest parameter if one exists, otherwise nil.
func (f *Func) RestParam() *Ident { return f.restParam }

func (f *Func) ExpressionNode() {}

func (f *Func) IsExpression() bool { return f.name == nil }

func (f *Func) Token() token.Token { return f.token }

func (f *Func) Literal() string { return f.token.Literal }

func (f *Func) Name() *Ident { return f.name }

func (f *Func) Parameters() []*Ident { return f.parameters }

func (f *Func) ParameterNames() []string {
	names := make([]string, 0, len(f.parameters))
	for _, p := range f.parameters {
		names = append(names, p.value)
	}
	return names
}

func (f *Func) Defaults() map[string]Expression { return f.defaults }

func (f *Func) Body() *Block { return f.body }

func (f *Func) String() string {
	var out bytes.Buffer
	params := make([]string, 0)
	for _, p := range f.parameters {
		params = append(params, p.value)
	}
	out.WriteString(f.Literal())
	if f.name != nil {
		out.WriteString(" " + f.name.value)
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") { ")
	out.WriteString(f.body.String())
	out.WriteString(" }")
	return out.String()
}

// String is an expression node that holds a string literal.
type String struct {
	// Token is the token
	token token.Token

	// value is the value of the string.
	value string

	// template is the templatized version of the string, if any
	template *tmpl.Template

	exprs []Expression
}

// NewString creates a new String node.
func NewString(tok token.Token) *String {
	return &String{token: tok, value: tok.Literal}
}

// NewTemplatedString creates a new String node defined by a template.
func NewTemplatedString(tok token.Token, template *tmpl.Template, exprs []Expression) *String {
	return &String{token: tok, value: tok.Literal, template: template, exprs: exprs}
}

func (s *String) ExpressionNode() {}

func (s *String) IsExpression() bool { return true }

func (s *String) Token() token.Token { return s.token }

func (s *String) Literal() string { return s.token.Literal }

func (s *String) Value() string { return s.value }

func (s *String) Template() *tmpl.Template { return s.template }

func (s *String) TemplateExpressions() []Expression { return s.exprs }

func (s *String) String() string { return fmt.Sprintf("%q", s.token.Literal) }

// List is an expression node that builds a list data structure.
type List struct {
	// Token is the token
	token token.Token

	// items holds the members of the list.
	items []Expression
}

// NewList creates a new List node.
func NewList(tok token.Token, items []Expression) *List {
	return &List{token: tok, items: items}
}

func (l *List) ExpressionNode() {}

func (l *List) IsExpression() bool { return true }

func (l *List) Token() token.Token { return l.token }

func (l *List) Literal() string { return l.token.Literal }

func (l *List) Items() []Expression { return l.items }

func (l *List) String() string {
	var out bytes.Buffer
	elements := make([]string, 0)
	for _, el := range l.items {
		elements = append(elements, el.String())
	}
	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")
	return out.String()
}

// MapItem represents a single key-value pair in a map literal.
// For spread expressions (...obj), Key is nil and Value is the spread expression.
type MapItem struct {
	Key   Expression // nil for spread expressions
	Value Expression
}

// Map is an expression node that builds a map data structure.
type Map struct {
	token token.Token // the '{' token
	items []MapItem   // ordered items in the map (key-value pairs or spreads)
}

// NewMap creates a new Map node.
func NewMap(token token.Token, items []MapItem) *Map {
	return &Map{token: token, items: items}
}

func (m *Map) ExpressionNode() {}

func (m *Map) IsExpression() bool { return true }

func (m *Map) Token() token.Token { return m.token }

func (m *Map) Literal() string { return m.token.Literal }

func (m *Map) Items() []MapItem { return m.items }

// HasSpread returns true if any items are spread expressions
func (m *Map) HasSpread() bool {
	for _, item := range m.items {
		if item.Key == nil {
			return true
		}
	}
	return false
}

func (m *Map) String() string {
	var out bytes.Buffer
	pairs := make([]string, 0)
	for _, item := range m.items {
		if item.Key == nil {
			pairs = append(pairs, "..."+item.Value.String())
		} else {
			pairs = append(pairs, item.Key.String()+":"+item.Value.String())
		}
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")
	return out.String()
}

// Set is an expression node that builds a set data structure.
type Set struct {
	token token.Token  // the '{' token
	items []Expression // items in the set
}

// NewSet creates a new Set node.
func NewSet(token token.Token, items []Expression) *Set {
	return &Set{token: token, items: items}
}

func (s *Set) ExpressionNode() {}

func (s *Set) IsExpression() bool { return true }

func (s *Set) Token() token.Token { return s.token }

func (s *Set) Literal() string { return s.token.Literal }

func (s *Set) Items() []Expression { return s.items }

func (s *Set) String() string {
	var out bytes.Buffer
	items := make([]string, 0, len(s.items))
	for _, key := range s.items {
		items = append(items, key.String())
	}
	out.WriteString("{")
	out.WriteString(strings.Join(items, ", "))
	out.WriteString("}")
	return out.String()
}
