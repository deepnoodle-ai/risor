package format

import (
	"strconv"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
)

// Tree writes a tree with its spans left out, for the law "the same tree"
// (2.2 item 1): two trees are equal when their texts are. src is the text
// the tree was parsed from; it gives the form of a template.
func Tree(n syntax.Node, src []rune) string {
	var b strings.Builder
	tree(&b, n, src)
	return b.String()
}

func tree(b *strings.Builder, n syntax.Node, src []rune) {
	w := b.WriteString
	sub := func(x syntax.Node) {
		w(" ")
		tree(b, x, src)
	}
	switch n := n.(type) {
	case nil:
		w("nil")
	case *syntax.Program:
		w("(program")
		for _, c := range n.Consts {
			sub(c)
		}
		sub(n.Result)
		w(")")
	case *syntax.Const:
		w("(const")
		sub(n.Pattern)
		sub(n.Value)
		w(")")
	case *syntax.Block:
		w("(block")
		for _, c := range n.Consts {
			sub(c)
		}
		sub(n.Result)
		w(")")
	case *syntax.Ident:
		w("(id " + n.Name + ")")
	case *syntax.Number:
		w("(num " + strconv.FormatFloat(n.Value, 'g', -1, 64) + ")")
	case *syntax.String:
		w("(str " + strconv.QuoteRune(n.Form) + " " + strconv.Quote(n.Value) + ")")
	case *syntax.Template:
		form := "`"
		if s := n.Span().Start; s < len(src) {
			switch {
			case src[s] == 'r':
				form = "r\"\"\""
			case src[s] == '"':
				form = "\"\"\""
			case src[s] == '`':
				form = "`"
			default:
				form = string(src[s])
			}
		}
		w("(tpl " + form)
		for _, p := range n.Parts {
			if p.Expr != nil {
				sub(p.Expr)
			} else {
				w(" " + strconv.Quote(p.Text))
			}
		}
		w(")")
	case *syntax.Bool:
		w(strconv.FormatBool(n.Value))
	case *syntax.Null:
		w("null")
	case *syntax.List:
		w("(list")
		for _, x := range n.Items {
			sub(x)
		}
		w(")")
	case *syntax.Map:
		w("(map")
		for _, x := range n.Entries {
			sub(x)
		}
		w(")")
	case *syntax.Entry:
		switch {
		case n.Shorthand:
			w("(short " + n.Key + ")")
		case n.KeyForm == syntax.KeyComputed:
			w("(entry []")
			sub(n.KeyExpr)
			sub(n.Value)
			w(")")
		default:
			w("(entry " + strconv.Itoa(int(n.KeyForm)) + " " + strconv.Quote(n.Key))
			sub(n.Value)
			w(")")
		}
	case *syntax.Spread:
		w("(...")
		sub(n.Value)
		w(")")
	case *syntax.Unary:
		w("(" + n.Op)
		sub(n.X)
		w(")")
	case *syntax.Binary:
		w("(" + n.Op)
		sub(n.X)
		sub(n.Y)
		w(")")
	case *syntax.Cond:
		w("(" + n.Form)
		sub(n.Test)
		sub(n.Then)
		sub(n.Else)
		w(")")
	case *syntax.Member:
		w("(. " + strconv.FormatBool(n.Optional) + " " + n.Name)
		sub(n.X)
		w(")")
	case *syntax.Index:
		w("([] " + strconv.FormatBool(n.Optional))
		sub(n.X)
		sub(n.Index)
		w(")")
	case *syntax.Call:
		w("(call " + n.Namespace + " " + n.Name)
		for _, a := range n.Args {
			sub(a)
		}
		w(")")
	case *syntax.MethodCall:
		w("(mcall " + strconv.FormatBool(n.Optional) + " " + n.Name)
		sub(n.X)
		for _, a := range n.Args {
			sub(a)
		}
		w(")")
	case *syntax.OptChain:
		w("(opt")
		sub(n.X)
		w(")")
	case *syntax.Lambda:
		w("(lambda")
		for _, p := range n.Params {
			sub(p)
		}
		sub(n.Body)
		w(")")
	case *syntax.Param:
		w("(param")
		sub(n.Pattern)
		if n.Default != nil {
			sub(n.Default)
		}
		w(")")
	case *syntax.Match:
		w("(match")
		sub(n.Subject)
		for _, a := range n.Arms {
			sub(a)
		}
		w(")")
	case *syntax.Arm:
		w("(arm " + strconv.FormatBool(n.Default))
		for _, l := range n.Literals {
			sub(l)
		}
		sub(n.Result)
		w(")")
	case *syntax.Try:
		w("(try " + n.Name)
		sub(n.Body)
		sub(n.Handler)
		w(")")
	case *syntax.NamePat:
		if n.Discard {
			w("_")
		} else {
			w("(name " + n.Name + ")")
		}
	case *syntax.MapPat:
		w("(mappat")
		for _, f := range n.Fields {
			w(" (field " + strconv.Itoa(int(f.KeyForm)) + " " + strconv.Quote(f.Key) + " " + f.Name)
			if f.Default != nil {
				sub(f.Default)
			}
			w(")")
		}
		if n.Rest != nil {
			w(" (rest " + n.Rest.Name + ")")
		}
		w(")")
	case *syntax.ListPat:
		w("(listpat")
		for _, e := range n.Elems {
			w(" (elem")
			sub(e.Name)
			if e.Default != nil {
				sub(e.Default)
			}
			w(")")
		}
		if n.Rest != nil {
			w(" (rest " + n.Rest.Name + ")")
		}
		w(")")
	case *syntax.As:
		w("(" + n.Form)
		sub(n.X)
		w(" ")
		ptypeTree(b, n.Type)
		w(")")
	case *syntax.Each:
		w("(each " + strconv.FormatBool(n.Optional))
		sub(n.X)
		for _, r := range n.Reads {
			sub(r)
		}
		w(")")
	default:
		w("(?)")
	}
}

func ptypeTree(b *strings.Builder, t *syntax.PType) {
	if t == nil {
		b.WriteString("nil")
		return
	}
	b.WriteString("(type")
	for _, a := range t.Alts {
		b.WriteString(" (alt " + strconv.Itoa(a.Lists) + " ")
		p := a.Prim
		b.WriteString("(" + strconv.Itoa(int(p.Kind)) + " " + strconv.Quote(p.Text))
		if p.Elem != nil {
			b.WriteString(" ")
			ptypeTree(b, p.Elem)
		}
		for _, f := range p.Fields {
			b.WriteString(" (f " + strconv.Quote(f.Key) + " " + strconv.FormatBool(f.Optional) + " ")
			ptypeTree(b, f.Type)
			b.WriteString(")")
		}
		if p.HasRest {
			b.WriteString(" (rest ")
			ptypeTree(b, p.Rest)
			b.WriteString(")")
		}
		b.WriteString("))")
	}
	b.WriteString(")")
}
