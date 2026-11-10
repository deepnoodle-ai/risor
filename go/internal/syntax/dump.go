package syntax

import (
	"strconv"
	"strings"
)

// Dump writes a node as an S-expression, for tests and debugging. Spans are
// not written.
func Dump(n Node) string {
	var b strings.Builder
	dump(&b, n)
	return b.String()
}

func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

func dump(b *strings.Builder, n Node) {
	w := b.WriteString
	list := func(head string, xs ...Node) {
		w("(" + head)
		for _, x := range xs {
			w(" ")
			dump(b, x)
		}
		w(")")
	}
	switch n := n.(type) {
	case nil:
		w("nil")
	case *Program:
		w("(program")
		for _, c := range n.Consts {
			w(" ")
			dump(b, c)
		}
		w(" ")
		dump(b, n.Result)
		w(")")
	case *Const:
		list("const", n.Pattern, n.Value)
	case *Block:
		w("(block")
		for _, c := range n.Consts {
			w(" ")
			dump(b, c)
		}
		w(" ")
		dump(b, n.Result)
		w(")")
	case *Ident:
		w(n.Name)
	case *Number:
		w(num(n.Value))
	case *String:
		w(strconv.Quote(n.Value))
	case *Template:
		w("(template")
		for _, p := range n.Parts {
			w(" ")
			if p.Expr != nil {
				dump(b, p.Expr)
			} else {
				w(strconv.Quote(p.Text))
			}
		}
		w(")")
	case *Bool:
		w(strconv.FormatBool(n.Value))
	case *Null:
		w("null")
	case *List:
		list("list", n.Items...)
	case *Map:
		list("map", n.Entries...)
	case *Entry:
		switch {
		case n.Shorthand:
			w("(entry " + n.Key + ")")
		case n.KeyForm == KeyComputed:
			list("entry", n.KeyExpr, n.Value)
		default:
			w("(entry " + strconv.Quote(n.Key) + " ")
			dump(b, n.Value)
			w(")")
		}
	case *Spread:
		list("...", n.Value)
	case *Unary:
		list(n.Op, n.X)
	case *Binary:
		list(n.Op, n.X, n.Y)
	case *Cond:
		list(n.Form, n.Test, n.Then, n.Else)
	case *Member:
		op := "."
		if n.Optional {
			op = "?."
		}
		w("(" + op + " ")
		dump(b, n.X)
		w(" " + n.Name + ")")
	case *Index:
		op := "[]"
		if n.Optional {
			op = "?.[]"
		}
		list(op, n.X, n.Index)
	case *Call:
		name := n.Name
		if n.Namespace != "" {
			name = n.Namespace + "." + name
		}
		list("call "+name, n.Args...)
	case *MethodCall:
		op := "."
		if n.Optional {
			op = "?."
		}
		w("(" + op + n.Name + " ")
		dump(b, n.X)
		for _, a := range n.Args {
			w(" ")
			dump(b, a)
		}
		w(")")
	case *OptChain:
		list("optchain", n.X)
	case *Lambda:
		w("(lambda (")
		for i, p := range n.Params {
			if i > 0 {
				w(" ")
			}
			dump(b, p)
		}
		w(") ")
		dump(b, n.Body)
		w(")")
	case *Param:
		if n.Default == nil {
			dump(b, n.Pattern)
		} else {
			list("=", n.Pattern, n.Default)
		}
	case *Match:
		w("(match ")
		dump(b, n.Subject)
		for _, a := range n.Arms {
			w(" ")
			dump(b, a)
		}
		w(")")
	case *Arm:
		if n.Default {
			w("(_ ")
		} else {
			w("(")
			for i, l := range n.Literals {
				if i > 0 {
					w("|")
				}
				dump(b, l)
			}
			w(" ")
		}
		dump(b, n.Result)
		w(")")
	case *Try:
		w("(try ")
		dump(b, n.Body)
		if n.Name != "" {
			w(" " + n.Name)
		}
		w(" ")
		dump(b, n.Handler)
		w(")")
	case *NamePat:
		if n.Discard {
			w("_")
		} else {
			w(n.Name)
		}
	case *MapPat:
		w("{")
		for i, f := range n.Fields {
			if i > 0 {
				w(" ")
			}
			if f.Key != f.Name || f.KeyForm == KeyString {
				w(strconv.Quote(f.Key) + ":")
			}
			w(f.Name)
			if f.Default != nil {
				w("=")
				dump(b, f.Default)
			}
		}
		if n.Rest != nil {
			if len(n.Fields) > 0 {
				w(" ")
			}
			w("..." + n.Rest.Name)
		}
		w("}")
	case *ListPat:
		w("[")
		for i, e := range n.Elems {
			if i > 0 {
				w(" ")
			}
			dump(b, e.Name)
			if e.Default != nil {
				w("=")
				dump(b, e.Default)
			}
		}
		if n.Rest != nil {
			if len(n.Elems) > 0 {
				w(" ")
			}
			w("..." + n.Rest.Name)
		}
		w("]")
	case *Bad:
		w("(bad)")
	default:
		w("(?)")
	}
}
