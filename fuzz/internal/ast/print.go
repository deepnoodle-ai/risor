package ast

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Levels of 01-language.md 3.3, loosest first. A node of a lower level than
// its place needs is printed in parentheses.
const (
	LevelExpr    = 1 // ? :, if, try, a lambda
	LevelCoal    = 2 // ??
	LevelOr      = 3
	LevelAnd     = 4
	LevelEqual   = 5
	LevelRel     = 6
	LevelAdd     = 7
	LevelMul     = 8
	LevelUnary   = 9
	LevelPower   = 10
	LevelPrimary = 11
)

// BinaryLevel gives the level of a binary operator, or 0 if op is not one.
func BinaryLevel(op string) int {
	switch op {
	case "??":
		return LevelCoal
	case "||":
		return LevelOr
	case "&&":
		return LevelAnd
	case "==", "!=":
		return LevelEqual
	case "<", "<=", ">", ">=", "in", "not in":
		return LevelRel
	case "+", "-":
		return LevelAdd
	case "*", "/", "%":
		return LevelMul
	case "**":
		return LevelPower
	}
	return 0
}

// Level gives the level of an expression node.
func Level(e Expr) int {
	switch e := e.(type) {
	case *Cond, *If, *Try, *Lambda:
		return LevelExpr
	case *Binary:
		return BinaryLevel(e.Op)
	case *Unary:
		return LevelUnary
	}
	return LevelPrimary
}

// Print writes a program as source text.
func Print(p *Program) string {
	var w printer
	w.consts(p.Consts)
	w.expr(p.Result, LevelExpr)
	if p.Semi {
		w.s(";")
	}
	return w.b.String()
}

// PrintExpr writes one expression as source text.
func PrintExpr(e Expr) string {
	var w printer
	w.expr(e, LevelExpr)
	return w.b.String()
}

type printer struct{ b strings.Builder }

func (w *printer) s(t string) { w.b.WriteString(t) }

func (w *printer) consts(cs []*Const) {
	for _, c := range cs {
		w.s("const ")
		w.pattern(c.Pat)
		w.s(" = ")
		w.expr(c.Value, LevelExpr)
		w.s("; ")
	}
}

func (w *printer) paren(e Expr) {
	w.s("(")
	w.expr(e, LevelExpr)
	w.s(")")
}

// expr writes e where a node of at least level need may stand.
func (w *printer) expr(e Expr, need int) {
	if Level(e) < need {
		w.paren(e)
		return
	}
	switch e := e.(type) {
	case *Num:
		w.s(e.Text)
	case *Str:
		w.s(QuoteString(e.Val, e.Quote, e.Esc))
	case *Template:
		w.s("`")
		for _, p := range e.Parts {
			if p.X == nil {
				w.s(templateText(p.Text))
				continue
			}
			w.s("${")
			w.expr(p.X, LevelExpr)
			w.s("}")
		}
		w.s("`")
	case *Bool:
		if e.V {
			w.s("true")
		} else {
			w.s("false")
		}
	case *Null:
		w.s("null")
	case *Ident:
		w.s(e.Name)
	case *List:
		w.s("[")
		for i, it := range e.Items {
			if i > 0 {
				w.s(", ")
			}
			if it.Spread {
				w.s("...")
			}
			w.expr(it.X, LevelExpr)
		}
		w.s("]")
	case *Map:
		w.s("{")
		for i, en := range e.Entries {
			if i > 0 {
				w.s(", ")
			}
			switch en.Kind {
			case EntryWord:
				w.s(en.Key + ": ")
				w.expr(en.X, LevelExpr)
			case EntryString:
				w.s(QuoteString(en.Key, '"', 0) + ": ")
				w.expr(en.X, LevelExpr)
			case EntryComputed:
				w.s("[")
				w.expr(en.KeyX, LevelExpr)
				w.s("]: ")
				w.expr(en.X, LevelExpr)
			case EntryShorthand:
				w.s(en.Key)
			case EntrySpread:
				w.s("...")
				w.expr(en.X, LevelExpr)
			}
		}
		w.s("}")
	case *Unary:
		w.s(e.Op)
		// 3.4 rule 1: - or ! may not apply directly to a Power with **.
		if b, ok := e.X.(*Binary); ok && b.Op == "**" {
			w.paren(e.X)
		} else {
			w.expr(e.X, LevelUnary)
		}
	case *Binary:
		w.binary(e)
	case *Cond:
		w.expr(e.Test, LevelCoal)
		w.s(" ? ")
		w.expr(e.Then, LevelExpr)
		w.s(" : ")
		w.expr(e.Else, LevelExpr)
	case *If:
		for i, t := range e.Tests {
			if i > 0 {
				w.s(" else ")
			}
			w.s("if (")
			w.expr(t, LevelExpr)
			w.s(") ")
			w.branch(e.Then[i])
		}
		w.s(" else ")
		w.branch(e.Else)
	case *Block:
		if len(e.Consts) == 0 {
			// The reducer may remove every const line. A block needs one, so
			// the rest is written as the value it has.
			w.paren(e.Result)
			return
		}
		w.s("{ ")
		w.consts(e.Consts)
		w.expr(e.Result, LevelExpr)
		w.s(" }")
	case *Try:
		w.s("try ")
		w.branch(e.Body)
		w.s(" catch ")
		if e.Name != "" {
			w.s("(" + e.Name + ") ")
		}
		w.branch(e.Handler)
	case *Match:
		w.s("match ")
		sub := PrintExpr(e.Subject)
		// 3.4 rule 6: a subject may not start with {, if or try.
		if strings.HasPrefix(sub, "{") || startsWord(sub, "if") || startsWord(sub, "try") {
			sub = "(" + sub + ")"
		}
		w.s(sub)
		w.s(" {")
		for i, a := range e.Arms {
			if i > 0 {
				w.s(",")
			}
			w.s(" ")
			if a.Default {
				w.s("_")
			} else {
				for j, l := range a.Lits {
					if j > 0 {
						w.s(" | ")
					}
					w.expr(l, LevelExpr)
				}
			}
			w.s(" => ")
			w.expr(a.Result, LevelExpr)
		}
		w.s(" }")
	case *Call:
		if e.NS != "" {
			w.s(e.NS + ".")
		}
		w.s(e.Name)
		w.args(e.Args)
	case *Member:
		w.expr(e.X, LevelPrimary)
		w.s(dot(e.Opt) + e.Name)
	case *Index:
		w.expr(e.X, LevelPrimary)
		if e.Opt {
			w.s("?.[")
		} else {
			w.s("[")
		}
		w.expr(e.I, LevelExpr)
		w.s("]")
	case *MethodCall:
		w.expr(e.X, LevelPrimary)
		w.s(dot(e.Opt) + e.Name)
		w.args(e.Args)
	case *Lambda:
		if e.Bare != "" {
			w.s(e.Bare)
		} else {
			w.s("(")
			for i, p := range e.Params {
				if i > 0 {
					w.s(", ")
				}
				w.pattern(p.Pat)
				if p.Default != nil {
					w.s(" = ")
					w.expr(p.Default, LevelExpr)
				}
			}
			w.s(")")
		}
		w.s(" => ")
		w.expr(e.Body, LevelExpr)
	case *Paren:
		w.paren(e.X)
	case *Raw:
		w.s(e.Text)
	default:
		panic(fmt.Sprintf("ast: cannot print %T", e))
	}
}

func dot(opt bool) string {
	if opt {
		return "?."
	}
	return "."
}

func startsWord(s, w string) bool {
	if !strings.HasPrefix(s, w) {
		return false
	}
	if len(s) == len(w) {
		return true
	}
	c := s[len(w)]
	return !(c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z')
}

func (w *printer) binary(e *Binary) {
	l := BinaryLevel(e.Op)
	switch {
	case e.Op == "??":
		// Each operand of ?? is an Add (3.4 rule 3); a chain of ?? is flat
		// and reads left to right.
		if b, ok := e.X.(*Binary); ok && b.Op == "??" {
			w.expr(e.X, LevelCoal)
		} else {
			w.expr(e.X, LevelAdd)
		}
		w.s(" ?? ")
		w.expr(e.Y, LevelAdd)
		return
	case e.Op == "**":
		// The left operand is a Chain, the right a Unary (3.1 Power).
		w.expr(e.X, LevelPrimary)
		w.s(" ** ")
		w.expr(e.Y, LevelUnary)
		return
	}
	// 3.4 rule 2: the left operand of in or not in may not be a ! expression
	// written without parentheses.
	if u, ok := e.X.(*Unary); ok && u.Op == "!" && (e.Op == "in" || e.Op == "not in") {
		w.paren(e.X)
	} else {
		w.expr(e.X, l)
	}
	w.s(" " + e.Op + " ")
	w.expr(e.Y, l+1)
}

func (w *printer) branch(b *Branch) {
	w.s("{ ")
	w.consts(b.Consts)
	w.expr(b.Result, LevelExpr)
	w.s(" }")
}

func (w *printer) args(as []Expr) {
	w.s("(")
	for i, a := range as {
		if i > 0 {
			w.s(", ")
		}
		w.expr(a, LevelExpr)
	}
	w.s(")")
}

func (w *printer) pattern(p Pattern) {
	switch p := p.(type) {
	case *NamePat:
		w.s(p.Name)
	case *MapPat:
		w.s("{")
		n := 0
		for _, f := range p.Fields {
			if n > 0 {
				w.s(", ")
			}
			n++
			switch {
			case f.Key == "":
				w.s(f.Name)
			case f.KeyStr:
				w.s(QuoteString(f.Key, '"', 0) + ": " + f.Name)
			default:
				w.s(f.Key + ": " + f.Name)
			}
			if f.Default != nil {
				w.s(" = ")
				w.expr(f.Default, LevelExpr)
			}
		}
		if p.Rest != "" {
			if n > 0 {
				w.s(", ")
			}
			w.s("..." + p.Rest)
		}
		w.s("}")
	case *ListPat:
		w.s("[")
		n := 0
		for _, el := range p.Elems {
			if n > 0 {
				w.s(", ")
			}
			n++
			w.s(el.Name)
			if el.Default != nil {
				w.s(" = ")
				w.expr(el.Default, LevelExpr)
			}
		}
		if p.Rest != "" {
			if n > 0 {
				w.s(", ")
			}
			w.s("..." + p.Rest)
		}
		w.s("]")
	}
}

// QuoteString writes v as a one-line string with the quote q, or as a
// template with no interpolation when q is a backtick.
func QuoteString(v string, q byte, esc int) string {
	if q == '`' {
		return "`" + templateText(v) + "`"
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range v {
		switch {
		case r == rune(q):
			b.WriteByte('\\')
			b.WriteByte(q)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04x`, r)
		case r < 0x80:
			b.WriteRune(r)
		default:
			writeNonASCII(&b, r, esc)
		}
	}
	b.WriteByte(q)
	return b.String()
}

func writeNonASCII(b *strings.Builder, r rune, esc int) {
	switch esc {
	case 1:
		fmt.Fprintf(b, `\u{%X}`, r)
	case 2:
		if r > 0xFFFF {
			r -= 0x10000
			fmt.Fprintf(b, `\u%04X\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
		} else {
			fmt.Fprintf(b, `\u%04X`, r)
		}
	default:
		b.WriteRune(r)
	}
}

// templateText escapes the text part of a template: `, \ and ${, and a raw
// carriage return, which a template would read as a line feed (2.8 rule 3).
func templateText(t string) string {
	var b strings.Builder
	for i := 0; i < len(t); {
		r, n := utf8.DecodeRuneInString(t[i:])
		switch {
		case r == '`':
			b.WriteString("\\`")
		case r == '\\':
			b.WriteString(`\\`)
		case r == '$' && i+1 < len(t) && t[i+1] == '{':
			b.WriteString(`\$`)
		case r == '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
		i += n
	}
	return b.String()
}
