package syntax

// Walk calls fn for n and then for each node under it, in written order.
// When fn returns false, Walk does not go under that node. It keeps its own
// stack, so a tree of any depth costs no Go stack.
func Walk(n Node, fn func(Node) bool) {
	stack := []Node{n}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil || !fn(n) {
			continue
		}
		kids := Children(n)
		for i := len(kids) - 1; i >= 0; i-- {
			stack = append(stack, kids[i])
		}
	}
}

// Children returns the nodes directly under n, in written order. A pattern
// field's or element's default is a child of its pattern.
func Children(n Node) []Node {
	var out []Node
	add := func(ns ...Node) {
		for _, x := range ns {
			if x != nil {
				out = append(out, x)
			}
		}
	}
	switch n := n.(type) {
	case *Program:
		for _, c := range n.Consts {
			add(c)
		}
		add(n.Result)
	case *Const:
		add(n.Pattern, n.Value)
	case *Block:
		for _, c := range n.Consts {
			add(c)
		}
		add(n.Result)
	case *Template:
		for _, p := range n.Parts {
			add(p.Expr)
		}
	case *List:
		add(n.Items...)
	case *Map:
		add(n.Entries...)
	case *Entry:
		add(n.KeyExpr, n.Value)
	case *Spread:
		add(n.Value)
	case *Unary:
		add(n.X)
	case *Binary:
		add(n.X, n.Y)
	case *Cond:
		add(n.Test, n.Then, n.Else)
	case *Member:
		add(n.X)
	case *Index:
		add(n.X, n.Index)
	case *Call:
		add(n.Args...)
	case *MethodCall:
		add(n.X)
		add(n.Args...)
	case *OptChain:
		add(n.X)
	case *Lambda:
		for _, p := range n.Params {
			add(p)
		}
		add(n.Body)
	case *Param:
		add(n.Pattern, n.Default)
	case *Match:
		add(n.Subject)
		for _, a := range n.Arms {
			add(a)
		}
	case *Arm:
		add(n.Literals...)
		add(n.Result)
	case *Try:
		add(n.Body, n.Handler)
	case *MapPat:
		for _, f := range n.Fields {
			add(f.Default)
		}
	case *ListPat:
		for _, e := range n.Elems {
			add(e.Default)
		}
	}
	return out
}
