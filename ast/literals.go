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
	ValuePos token.Position // position of the literal
	Literal  string         // the literal text (e.g., "42", "0x2a")
	Value    int64          // the parsed value
}

func (x *Int) exprNode() {}

func (x *Int) Pos() token.Position { return x.ValuePos }
func (x *Int) End() token.Position { return x.ValuePos.Advance(len(x.Literal)) }

func (x *Int) String() string { return x.Literal }

// Float is an expression node that holds a floating point literal.
type Float struct {
	ValuePos token.Position // position of the literal
	Literal  string         // the literal text
	Value    float64        // the parsed value
}

func (x *Float) exprNode() {}

func (x *Float) Pos() token.Position { return x.ValuePos }
func (x *Float) End() token.Position { return x.ValuePos.Advance(len(x.Literal)) }

func (x *Float) String() string { return x.Literal }

// Nil is an expression node that holds a nil literal.
type Nil struct {
	NilPos token.Position // position of "nil" keyword
}

func (x *Nil) exprNode() {}

func (x *Nil) Pos() token.Position { return x.NilPos }
func (x *Nil) End() token.Position { return x.NilPos.Advance(3) } // len("nil")

func (x *Nil) String() string { return "nil" }

// Bool is an expression node that holds a boolean literal.
type Bool struct {
	ValuePos token.Position // position of "true" or "false"
	Literal  string         // "true" or "false"
	Value    bool           // the boolean value
}

func (x *Bool) exprNode() {}

func (x *Bool) Pos() token.Position { return x.ValuePos }
func (x *Bool) End() token.Position { return x.ValuePos.Advance(len(x.Literal)) }

func (x *Bool) String() string { return x.Literal }

// DefaultValue is an expression node used in map shorthand syntax {a = expr}.
// It represents an identifier with a default value, used for destructuring.
type DefaultValue struct {
	Name    *Ident // the identifier
	Default Expr   // the default value expression
}

func (x *DefaultValue) exprNode() {}

func (x *DefaultValue) Pos() token.Position { return x.Name.Pos() }
func (x *DefaultValue) End() token.Position {
	if x.Default != nil {
		return x.Default.End()
	}
	return x.Name.End()
}

func (x *DefaultValue) String() string {
	return x.Name.String() + " = " + x.Default.String()
}

// String is an expression node that holds a string literal.
type String struct {
	ValuePos token.Position // position of opening quote
	Literal  string         // the raw literal including quotes
	Value    string         // the unquoted string value
	Template *tmpl.Template // template if this is a template string
	Exprs    []Expr         // embedded expressions for templates
}

func (x *String) exprNode() {}

func (x *String) Pos() token.Position { return x.ValuePos }
func (x *String) End() token.Position { return x.ValuePos.Advance(len(x.Literal)) }

func (x *String) String() string { return fmt.Sprintf("%q", x.Value) }

// FuncParam represents a function parameter, which can be a simple identifier
// or a destructuring pattern (object or array).
type FuncParam interface {
	Node
	funcParam()
	// ParamNames returns all variable names introduced by this parameter.
	ParamNames() []string
}

// Ensure *Ident implements FuncParam
func (x *Ident) funcParam() {}

// ParamNames returns the single variable name for an identifier parameter.
func (x *Ident) ParamNames() []string { return []string{x.Name} }

// ObjectDestructureParam represents object destructuring in parameter position.
// Example: function foo({a, b}) { ... }
type ObjectDestructureParam struct {
	Lbrace   token.Position       // position of "{"
	Bindings []DestructureBinding // bindings to extract (reused from statements.go)
	Rbrace   token.Position       // position of "}"
}

func (x *ObjectDestructureParam) funcParam() {}

func (x *ObjectDestructureParam) Pos() token.Position { return x.Lbrace }
func (x *ObjectDestructureParam) End() token.Position { return x.Rbrace.Advance(1) }

// ParamNames returns all variable names introduced by this destructuring parameter.
func (x *ObjectDestructureParam) ParamNames() []string {
	names := make([]string, len(x.Bindings))
	for i, b := range x.Bindings {
		if b.Alias != "" {
			names[i] = b.Alias
		} else {
			names[i] = b.Key
		}
	}
	return names
}

func (x *ObjectDestructureParam) String() string {
	var out bytes.Buffer
	out.WriteString("{")
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
	out.WriteString("}")
	return out.String()
}

// ArrayDestructureParam represents array destructuring in parameter position.
// Example: function foo([a, b]) { ... }
type ArrayDestructureParam struct {
	Lbrack   token.Position            // position of "["
	Elements []ArrayDestructureElement // elements to extract (reused from statements.go)
	Rbrack   token.Position            // position of "]"
}

func (x *ArrayDestructureParam) funcParam() {}

func (x *ArrayDestructureParam) Pos() token.Position { return x.Lbrack }
func (x *ArrayDestructureParam) End() token.Position { return x.Rbrack.Advance(1) }

// ParamNames returns all variable names introduced by this destructuring parameter.
func (x *ArrayDestructureParam) ParamNames() []string {
	names := make([]string, len(x.Elements))
	for i, e := range x.Elements {
		names[i] = e.Name.Name
	}
	return names
}

func (x *ArrayDestructureParam) String() string {
	var out bytes.Buffer
	out.WriteString("[")
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
	out.WriteString("]")
	return out.String()
}

// Func is an expression node that holds a function literal.
type Func struct {
	Func      token.Position  // position of "function" keyword
	Name      *Ident          // function name; nil for anonymous functions
	Lparen    token.Position  // position of "("
	Params    []FuncParam     // parameter names or destructuring patterns
	Defaults  map[string]Expr // default values for simple parameters
	RestParam *Ident          // rest parameter (e.g., ...args); nil if none
	Rparen    token.Position  // position of ")"
	Body      *Block          // function body
}

func (x *Func) exprNode() {}
func (x *Func) stmtNode() {} // named functions are also statements

func (x *Func) Pos() token.Position { return x.Func }

func (x *Func) End() token.Position {
	if x.Body != nil {
		return x.Body.End()
	}
	return x.Rparen.Advance(1)
}

func (x *Func) String() string {
	var out bytes.Buffer
	params := make([]string, 0, len(x.Params))
	for _, p := range x.Params {
		params = append(params, p.String())
	}
	out.WriteString("function")
	if x.Name != nil {
		out.WriteString(" ")
		out.WriteString(x.Name.Name)
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") { ")
	out.WriteString(x.Body.String())
	out.WriteString(" }")
	return out.String()
}

// List is an expression node that builds a list data structure.
type List struct {
	Lbrack token.Position // position of "["
	Items  []Expr         // list elements
	Rbrack token.Position // position of "]"
}

func (x *List) exprNode() {}

func (x *List) Pos() token.Position { return x.Lbrack }
func (x *List) End() token.Position { return x.Rbrack.Advance(1) }

func (x *List) String() string {
	var out bytes.Buffer
	elements := make([]string, 0, len(x.Items))
	for _, el := range x.Items {
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
	Key   Expr // nil for spread expressions
	Value Expr
}

// Map is an expression node that builds a map data structure.
type Map struct {
	Lbrace token.Position // position of "{"
	Items  []MapItem      // ordered items (key-value pairs or spreads)
	Rbrace token.Position // position of "}"
}

func (x *Map) exprNode() {}

func (x *Map) Pos() token.Position { return x.Lbrace }
func (x *Map) End() token.Position { return x.Rbrace.Advance(1) }

// HasSpread returns true if any items are spread expressions
func (x *Map) HasSpread() bool {
	for _, item := range x.Items {
		if item.Key == nil {
			return true
		}
	}
	return false
}

func (x *Map) String() string {
	var out bytes.Buffer
	pairs := make([]string, 0, len(x.Items))
	for _, item := range x.Items {
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
