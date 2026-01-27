package ast

import "iter"

// Visitor defines the interface for AST traversal. If Visit returns nil,
// children of the node are not visited. Otherwise, the returned Visitor
// is used to visit children.
type Visitor interface {
	Visit(node Node) (w Visitor)
}

// Walk traverses an AST in depth-first order. It starts by calling
// v.Visit(node); if the returned visitor w is not nil, Walk is invoked
// recursively with visitor w for each of the non-nil children of node.
func Walk(v Visitor, node Node) {
	if v = v.Visit(node); v == nil {
		return
	}

	// Walk children based on node type
	switch n := node.(type) {
	case *Program:
		for _, stmt := range n.Stmts {
			Walk(v, stmt)
		}

	// Statements
	case *Var:
		if n.Value != nil {
			Walk(v, n.Value)
		}
	case *MultiVar:
		if n.Value != nil {
			Walk(v, n.Value)
		}
	case *ObjectDestructure:
		for _, b := range n.Bindings {
			if b.Default != nil {
				Walk(v, b.Default)
			}
		}
		if n.Value != nil {
			Walk(v, n.Value)
		}
	case *ArrayDestructure:
		for _, e := range n.Elements {
			if e.Name != nil {
				Walk(v, e.Name)
			}
			if e.Default != nil {
				Walk(v, e.Default)
			}
		}
		if n.Value != nil {
			Walk(v, n.Value)
		}
	case *Const:
		if n.Value != nil {
			Walk(v, n.Value)
		}
	case *Return:
		if n.Value != nil {
			Walk(v, n.Value)
		}
	case *Block:
		for _, stmt := range n.Stmts {
			Walk(v, stmt)
		}
	case *Assign:
		if n.Name != nil {
			Walk(v, n.Name)
		}
		if n.Index != nil {
			Walk(v, n.Index)
		}
		if n.Value != nil {
			Walk(v, n.Value)
		}
	case *SetAttr:
		if n.X != nil {
			Walk(v, n.X)
		}
		if n.Value != nil {
			Walk(v, n.Value)
		}
	case *Try:
		if n.Body != nil {
			Walk(v, n.Body)
		}
		if n.CatchIdent != nil {
			Walk(v, n.CatchIdent)
		}
		if n.CatchBlock != nil {
			Walk(v, n.CatchBlock)
		}
		if n.FinallyBlock != nil {
			Walk(v, n.FinallyBlock)
		}
	case *Throw:
		if n.Value != nil {
			Walk(v, n.Value)
		}
	case *Postfix:
		if n.X != nil {
			Walk(v, n.X)
		}

	// Error recovery nodes
	case *BadExpr:
		// No children
	case *BadStmt:
		// No children

	// Expressions
	case *Ident:
		// No children
	case *Int:
		// No children
	case *Float:
		// No children
	case *Bool:
		// No children
	case *Nil:
		// No children
	case *String:
		// String may contain template expressions
		for _, expr := range n.Exprs {
			Walk(v, expr)
		}
	case *Prefix:
		if n.X != nil {
			Walk(v, n.X)
		}
	case *Spread:
		if n.X != nil {
			Walk(v, n.X)
		}
	case *Infix:
		if n.X != nil {
			Walk(v, n.X)
		}
		if n.Y != nil {
			Walk(v, n.Y)
		}
	case *If:
		if n.Cond != nil {
			Walk(v, n.Cond)
		}
		if n.Consequence != nil {
			Walk(v, n.Consequence)
		}
		if n.Alternative != nil {
			Walk(v, n.Alternative)
		}
	case *Call:
		if n.Fun != nil {
			Walk(v, n.Fun)
		}
		for _, arg := range n.Args {
			Walk(v, arg)
		}
	case *GetAttr:
		if n.X != nil {
			Walk(v, n.X)
		}
	case *Pipe:
		for _, expr := range n.Exprs {
			Walk(v, expr)
		}
	case *ObjectCall:
		if n.X != nil {
			Walk(v, n.X)
		}
		if n.Call != nil {
			Walk(v, n.Call)
		}
	case *Index:
		if n.X != nil {
			Walk(v, n.X)
		}
		if n.Index != nil {
			Walk(v, n.Index)
		}
	case *Slice:
		if n.X != nil {
			Walk(v, n.X)
		}
		if n.Low != nil {
			Walk(v, n.Low)
		}
		if n.High != nil {
			Walk(v, n.High)
		}
	case *Case:
		for _, expr := range n.Exprs {
			Walk(v, expr)
		}
		if n.Body != nil {
			Walk(v, n.Body)
		}
	case *Switch:
		if n.Value != nil {
			Walk(v, n.Value)
		}
		for _, c := range n.Cases {
			Walk(v, c)
		}
	case *In:
		if n.X != nil {
			Walk(v, n.X)
		}
		if n.Y != nil {
			Walk(v, n.Y)
		}
	case *NotIn:
		if n.X != nil {
			Walk(v, n.X)
		}
		if n.Y != nil {
			Walk(v, n.Y)
		}
	case *List:
		for _, item := range n.Items {
			Walk(v, item)
		}
	case *Map:
		for _, pair := range n.Items {
			if pair.Key != nil {
				Walk(v, pair.Key)
			}
			Walk(v, pair.Value)
		}
	case *Func:
		if n.Name != nil {
			Walk(v, n.Name)
		}
		for _, param := range n.Params {
			Walk(v, param)
		}
		for _, def := range n.Defaults {
			if def != nil {
				Walk(v, def)
			}
		}
		if n.RestParam != nil {
			Walk(v, n.RestParam)
		}
		if n.Body != nil {
			Walk(v, n.Body)
		}
	}
}

// Inspect traverses an AST in depth-first order. It calls f(node) for each
// node; if f returns true, Inspect invokes f recursively for each of the
// non-nil children of node.
func Inspect(node Node, f func(Node) bool) {
	Walk(inspector(f), node)
}

type inspector func(Node) bool

func (f inspector) Visit(node Node) Visitor {
	if f(node) {
		return f
	}
	return nil
}

// Preorder returns an iterator over all the nodes of the AST rooted at node
// in depth-first preorder.
func Preorder(root Node) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		var visit func(Node) bool
		visit = func(n Node) bool {
			if !yield(n) {
				return false
			}
			// Visit children based on node type
			switch node := n.(type) {
			case *Program:
				for _, stmt := range node.Stmts {
					if !visit(stmt) {
						return false
					}
				}
			case *Var:
				if node.Value != nil && !visit(node.Value) {
					return false
				}
			case *MultiVar:
				if node.Value != nil && !visit(node.Value) {
					return false
				}
			case *ObjectDestructure:
				for _, b := range node.Bindings {
					if b.Default != nil && !visit(b.Default) {
						return false
					}
				}
				if node.Value != nil && !visit(node.Value) {
					return false
				}
			case *ArrayDestructure:
				for _, e := range node.Elements {
					if e.Name != nil && !visit(e.Name) {
						return false
					}
					if e.Default != nil && !visit(e.Default) {
						return false
					}
				}
				if node.Value != nil && !visit(node.Value) {
					return false
				}
			case *Const:
				if node.Value != nil && !visit(node.Value) {
					return false
				}
			case *Return:
				if node.Value != nil && !visit(node.Value) {
					return false
				}
			case *Block:
				for _, stmt := range node.Stmts {
					if !visit(stmt) {
						return false
					}
				}
			case *Assign:
				if node.Name != nil && !visit(node.Name) {
					return false
				}
				if node.Index != nil && !visit(node.Index) {
					return false
				}
				if node.Value != nil && !visit(node.Value) {
					return false
				}
			case *SetAttr:
				if node.X != nil && !visit(node.X) {
					return false
				}
				if node.Value != nil && !visit(node.Value) {
					return false
				}
			case *Try:
				if node.Body != nil && !visit(node.Body) {
					return false
				}
				if node.CatchIdent != nil && !visit(node.CatchIdent) {
					return false
				}
				if node.CatchBlock != nil && !visit(node.CatchBlock) {
					return false
				}
				if node.FinallyBlock != nil && !visit(node.FinallyBlock) {
					return false
				}
			case *Throw:
				if node.Value != nil && !visit(node.Value) {
					return false
				}
			case *Postfix:
				if node.X != nil && !visit(node.X) {
					return false
				}
			case *BadExpr:
				// No children
			case *BadStmt:
				// No children
			case *Prefix:
				if node.X != nil && !visit(node.X) {
					return false
				}
			case *Spread:
				if node.X != nil && !visit(node.X) {
					return false
				}
			case *Infix:
				if node.X != nil && !visit(node.X) {
					return false
				}
				if node.Y != nil && !visit(node.Y) {
					return false
				}
			case *If:
				if node.Cond != nil && !visit(node.Cond) {
					return false
				}
				if node.Consequence != nil && !visit(node.Consequence) {
					return false
				}
				if node.Alternative != nil && !visit(node.Alternative) {
					return false
				}
			case *Call:
				if node.Fun != nil && !visit(node.Fun) {
					return false
				}
				for _, arg := range node.Args {
					if !visit(arg) {
						return false
					}
				}
			case *GetAttr:
				if node.X != nil && !visit(node.X) {
					return false
				}
			case *Pipe:
				for _, expr := range node.Exprs {
					if !visit(expr) {
						return false
					}
				}
			case *ObjectCall:
				if node.X != nil && !visit(node.X) {
					return false
				}
				if node.Call != nil && !visit(node.Call) {
					return false
				}
			case *Index:
				if node.X != nil && !visit(node.X) {
					return false
				}
				if node.Index != nil && !visit(node.Index) {
					return false
				}
			case *Slice:
				if node.X != nil && !visit(node.X) {
					return false
				}
				if node.Low != nil && !visit(node.Low) {
					return false
				}
				if node.High != nil && !visit(node.High) {
					return false
				}
			case *Case:
				for _, expr := range node.Exprs {
					if !visit(expr) {
						return false
					}
				}
				if node.Body != nil && !visit(node.Body) {
					return false
				}
			case *Switch:
				if node.Value != nil && !visit(node.Value) {
					return false
				}
				for _, c := range node.Cases {
					if !visit(c) {
						return false
					}
				}
			case *In:
				if node.X != nil && !visit(node.X) {
					return false
				}
				if node.Y != nil && !visit(node.Y) {
					return false
				}
			case *NotIn:
				if node.X != nil && !visit(node.X) {
					return false
				}
				if node.Y != nil && !visit(node.Y) {
					return false
				}
			case *List:
				for _, item := range node.Items {
					if !visit(item) {
						return false
					}
				}
			case *Map:
				for _, pair := range node.Items {
					if pair.Key != nil && !visit(pair.Key) {
						return false
					}
					if !visit(pair.Value) {
						return false
					}
				}
			case *Func:
				if node.Name != nil && !visit(node.Name) {
					return false
				}
				for _, param := range node.Params {
					if !visit(param) {
						return false
					}
				}
				for _, def := range node.Defaults {
					if def != nil && !visit(def) {
						return false
					}
				}
				if node.RestParam != nil && !visit(node.RestParam) {
					return false
				}
				if node.Body != nil && !visit(node.Body) {
					return false
				}
			case *String:
				for _, expr := range node.Exprs {
					if !visit(expr) {
						return false
					}
				}
			}
			return true
		}
		visit(root)
	}
}
