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
		mark := len(stack)
		stack = AppendChildren(stack, n)
		// The children were appended in written order; reverse them so the
		// first is popped first.
		for i, j := mark, len(stack)-1; i < j; i, j = i+1, j-1 {
			stack[i], stack[j] = stack[j], stack[i]
		}
	}
}

// Children returns the nodes directly under n, in written order. A pattern
// field's or element's default is a child of its pattern.
func Children(n Node) []Node { return AppendChildren(nil, n) }

// AppendChildren appends the nodes directly under n to out, in written
// order, and returns the extended slice. Walk uses it with one buffer, so a
// walk allocates nothing for each node.
func AppendChildren(out []Node, n Node) []Node {
	add := func(x Node) {
		if x != nil {
			out = append(out, x)
		}
	}
	addAll := func(xs []Node) {
		for _, x := range xs {
			add(x)
		}
	}
	switch n := n.(type) {
	case *Program:
		for _, c := range n.Consts {
			add(c)
		}
		add(n.Result)
	case *Const:
		add(n.Pattern)
		add(n.Value)
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
		addAll(n.Items)
	case *Map:
		addAll(n.Entries)
	case *Entry:
		add(n.KeyExpr)
		add(n.Value)
	case *Spread:
		add(n.Value)
	case *Unary:
		add(n.X)
	case *Binary:
		add(n.X)
		add(n.Y)
	case *Cond:
		add(n.Test)
		add(n.Then)
		add(n.Else)
	case *Member:
		add(n.X)
	case *Index:
		add(n.X)
		add(n.Index)
	case *Call:
		addAll(n.Args)
	case *MethodCall:
		add(n.X)
		addAll(n.Args)
	case *OptChain:
		add(n.X)
	case *As:
		add(n.X)
	case *Each:
		add(n.X)
		addAll(n.Reads)
	case *Lambda:
		for _, p := range n.Params {
			add(p)
		}
		add(n.Body)
	case *Param:
		add(n.Pattern)
		add(n.Default)
	case *Match:
		add(n.Subject)
		for _, a := range n.Arms {
			add(a)
		}
	case *Arm:
		addAll(n.Literals)
		add(n.Result)
	case *Try:
		add(n.Body)
		add(n.Handler)
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
