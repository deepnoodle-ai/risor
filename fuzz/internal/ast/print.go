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

type printer struct {
	b strings.Builder

	// With rec set, the printer records the span of each node it writes.
	rec       bool
	line, col int
	spans     map[Expr]Span
	nodes     []NodeSpan
}

// Span is a span of 03-host.md 3.7.7: line and column of the start, and of
// the place after the end; columns count code points.
type Span [4]int

// NodeSpan is the span of one node, and the spans of the nodes a run failure
// there may be "at" (05-types.md 18.3): its operands, receiver, index and
// arguments; for a pattern, its key or name and the value bound.
type NodeSpan struct {
	Kind     string
	Span     Span
	Children []Span
}

type patSpan struct {
	kind  string
	span  Span
	key   *Span
	param bool
}

func (w *printer) s(t string) {
	w.b.WriteString(t)
	if w.rec {
		for _, r := range t {
			if r == '\n' {
				w.line, w.col = w.line+1, 1
			} else {
				w.col++
			}
		}
	}
}

func (w *printer) pos() [2]int { return [2]int{w.line, w.col} }

func (w *printer) span(start [2]int) Span { return Span{start[0], start[1], w.line, w.col} }

// PrintSpans writes a program and gives the span of every node.
func PrintSpans(p *Program) (string, []NodeSpan) {
	w := printer{rec: true, line: 1, col: 1, spans: map[Expr]Span{}}
	w.consts(p.Consts)
	w.expr(p.Result, LevelExpr)
	if p.Semi {
		w.s(";")
	}
	return w.b.String(), w.nodes
}

func (w *printer) consts(cs []*Const) {
	for _, c := range cs {
		w.s("const ")
		pats := w.pattern(c.Pat)
		w.s(" = ")
		w.expr(c.Value, LevelExpr)
		w.s("; ")
		if w.rec {
			if _, ok := c.Value.(*Lambda); ok {
				w.nodes = append(w.nodes, NodeSpan{Kind: "ConstLambda", Span: w.spans[c.Value]})
			}
			// 12.3: the value bound is the const's value expression.
			vs, ok := w.spans[c.Value]
			for _, ps := range pats {
				ns := NodeSpan{Kind: ps.kind, Span: ps.span}
				if ps.key != nil {
					ns.Children = append(ns.Children, *ps.key)
				}
				if ok {
					ns.Children = append(ns.Children, vs)
				}
				w.nodes = append(w.nodes, ns)
			}
		}
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
	start := w.pos()
	if w.rec {
		defer func() {
			if _, isBlock := e.(*Block); isBlock && len(e.(*Block).Consts) == 0 {
				return
			}
			sp := w.span(start)
			w.spans[e] = sp
			ns := NodeSpan{Kind: fmt.Sprintf("%T", e)[5:], Span: sp}
			for _, c := range Children(e) {
				// Parentheses have no node (01-language.md 4.2): the child
				// is the expression inside them.
				for {
					if cs, ok := w.spans[c]; ok {
						ns.Children = append(ns.Children, cs)
					}
					if p, ok := c.(*Paren); ok {
						c = p.X
						continue
					}
					if b, ok := c.(*Block); ok && len(b.Consts) == 0 {
						c = b.Result
						continue
					}
					break
				}
			}
			w.nodes = append(w.nodes, ns)
		}()
	}
	switch e := e.(type) {
	case *Num:
		w.s(e.Text)
	case *Str:
		w.s(StrText(e))
	case *Template:
		if e.Block {
			w.s("\"\"\"\n" + e.Indent)
			start := true
			for _, p := range e.Parts {
				if p.X == nil {
					var t string
					t, start = blockText(p.Text, e.Indent, start)
					w.s(t)
					continue
				}
				w.s("${")
				w.expr(p.X, LevelExpr)
				w.s("}")
				start = false
			}
			w.s("\n" + e.Indent + "\"\"\"")
			break
		}
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
			st := w.pos()
			if it.Spread {
				w.s("...")
			}
			w.expr(it.X, LevelExpr)
			if it.Spread && w.rec {
				w.nodes = append(w.nodes, NodeSpan{Kind: "Spread", Span: w.span(st), Children: []Span{w.spans[it.X]}})
			}
		}
		w.s("]")
	case *Map:
		w.s("{")
		for i, en := range e.Entries {
			if i > 0 {
				w.s(", ")
			}
			st := w.pos()
			switch en.Kind {
			case EntryWord:
				w.s(en.Key + ": ")
				w.expr(en.X, LevelExpr)
			case EntryString:
				w.s(keyText(en.Key, en.Raw) + ": ")
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
			if w.rec && (en.Kind == EntryComputed || en.Kind == EntrySpread) {
				ch := []Span{w.spans[en.X]}
				if en.Kind == EntryComputed {
					ch = []Span{w.spans[en.KeyX]}
				}
				w.nodes = append(w.nodes, NodeSpan{Kind: map[EntryKind]string{EntryComputed: "Entry", EntrySpread: "Spread"}[en.Kind], Span: w.span(st), Children: ch})
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
			w.paren(e.Subject)
		} else {
			w.expr(e.Subject, LevelExpr)
		}
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
				ps := w.pattern(p.Pat)
				if w.rec && len(ps) > 0 {
					// 12.3: for a parameter, the value bound is its pattern.
					for _, q := range ps {
						ns := NodeSpan{Kind: q.kind, Span: q.span}
						if q.key != nil {
							ns.Children = append(ns.Children, *q.key)
						}
						ns.Children = append(ns.Children, ps[len(ps)-1].span)
						w.nodes = append(w.nodes, ns)
					}
				}
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

// pattern writes a pattern and gives the spans of its parts: the pattern
// itself, each field and each element, each with the span of its key or
// name.
func (w *printer) pattern(p Pattern) []patSpan {
	var out []patSpan
	start := w.pos()
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
			fs := w.pos()
			var key Span
			switch {
			case f.Key == "":
				w.s(f.Name)
				key = w.span(fs)
			case f.KeyStr:
				w.s(keyText(f.Key, f.Raw))
				key = w.span(fs)
				w.s(": " + f.Name)
			default:
				w.s(f.Key)
				key = w.span(fs)
				w.s(": " + f.Name)
			}
			if f.Default != nil {
				w.s(" = ")
				w.expr(f.Default, LevelExpr)
				w.defaultSpan(f.Default)
			}
			out = append(out, patSpan{kind: "Field", span: w.span(fs), key: &key})
		}
		if p.Rest != "" {
			if n > 0 {
				w.s(", ")
			}
			w.s("..." + p.Rest)
		}
		w.s("}")
		out = append(out, patSpan{kind: "MapPat", span: w.span(start)})
	case *ListPat:
		w.s("[")
		n := 0
		for _, el := range p.Elems {
			if n > 0 {
				w.s(", ")
			}
			n++
			es := w.pos()
			w.s(el.Name)
			name := w.span(es)
			if el.Default != nil {
				w.s(" = ")
				w.expr(el.Default, LevelExpr)
				w.defaultSpan(el.Default)
			}
			out = append(out, patSpan{kind: "Elem", span: w.span(es), key: &name})
		}
		if p.Rest != "" {
			if n > 0 {
				w.s(", ")
			}
			w.s("..." + p.Rest)
		}
		w.s("]")
		out = append(out, patSpan{kind: "ListPat", span: w.span(start)})
	}
	return out
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

// StrText writes a string literal in the form its node asks for, or as a
// one-line string when that form cannot hold the value.
func StrText(e *Str) string {
	switch {
	case e.Block == 2 && rawBlockOK(e.Val):
		var b strings.Builder
		b.WriteString("r\"\"\"\n")
		for i, line := range strings.Split(e.Val, "\n") {
			if i > 0 {
				b.WriteString("\n")
			}
			if line != "" {
				b.WriteString(e.Indent + line)
			}
		}
		b.WriteString("\n" + e.Indent + "\"\"\"")
		return b.String()
	case e.Block != 0:
		t, _ := blockText(e.Val, e.Indent, true)
		return "\"\"\"\n" + e.Indent + t + "\n" + e.Indent + "\"\"\""
	case e.Raw && rawOK(e.Val, e.Quote):
		return "r" + string(e.Quote) + e.Val + string(e.Quote)
	}
	return QuoteString(e.Val, e.Quote, e.Esc)
}

func keyText(k string, raw bool) string {
	if raw && rawOK(k, '"') {
		return `r"` + k + `"`
	}
	return QuoteString(k, '"', 0)
}

// rawOK says whether a raw string in quote q can hold v (F3 section 3).
func rawOK(v string, q byte) bool {
	if q != '"' && q != '\'' {
		return false
	}
	return !strings.ContainsAny(v, string(q)+"\n\r")
}

// rawBlockOK says whether a raw text block can hold v: no """, no carriage
// return, and no line of only spaces and tabs (it would read as empty).
func rawBlockOK(v string) bool {
	if strings.Contains(v, `"""`) || strings.Contains(v, "\r") {
		return false
	}
	for _, line := range strings.Split(v, "\n") {
		if line != "" && strings.Trim(line, " \t") == "" {
			return false
		}
	}
	return true
}

// blockText escapes text for a text block (F3 section 4): every \, " and
// ${, a carriage return and other control characters, and a space or tab at
// the start of a line, so that no line reads as blank. Each line feed is
// followed by the indentation. start says whether the text begins a line;
// the result says whether the text ends at the start of one.
func blockText(t, indent string, start bool) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(t); {
		r, n := utf8.DecodeRuneInString(t[i:])
		switch {
		case r == '\n':
			b.WriteString("\n" + indent)
			start = true
			i += n
			continue
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '$' && i+1 < len(t) && t[i+1] == '{':
			b.WriteString(`\$`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t' && start:
			b.WriteString(`\t`)
		case r == ' ' && start:
			b.WriteString(`\u0020`)
		case r < 0x20 && r != '\t' || r == 0x7f:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
		start = false
		i += n
	}
	return b.String(), start
}

// defaultSpan records the span of a pattern's default as a node of kind
// Default.
func (w *printer) defaultSpan(d Expr) {
	if w.rec {
		if sp, ok := w.spans[d]; ok {
			w.nodes = append(w.nodes, NodeSpan{Kind: "Default", Span: sp})
		}
	}
}

// Within says whether the span s lies within the span outer.
func (s Span) Within(outer Span) bool {
	starts := s[0] > outer[0] || s[0] == outer[0] && s[1] >= outer[1]
	ends := s[2] < outer[2] || s[2] == outer[2] && s[3] <= outer[3]
	return starts && ends
}
