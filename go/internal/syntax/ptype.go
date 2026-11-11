package syntax

import (
	"strconv"
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
)

// This file is the program type of package F4, 2.1 to 2.8
// (spec/packages/f04-conversion.md): the type text after "as" and "is".

// typeWords are the words of a program type where a PPrim is expected.
var typeWords = map[string]bool{"null": true, "bool": true, "int": true, "float": true,
	"string": true, "any": true, "never": true}

// IsTypeName reports whether w is a type name: an upper-case letter, then one
// or more of [A-Za-z0-9_] (05-types.md 3.1).
func IsTypeName(w string) bool {
	if len(w) < 2 || w[0] < 'A' || w[0] > 'Z' {
		return false
	}
	for i := 1; i < len(w); i++ {
		c := rune(w[i])
		if !isWordPart(c) {
			return false
		}
	}
	return true
}

// badTypeText reports bad-type-text at a token (F4 2.6). A token no rule
// accepts stays unexpected-token (2.6 item 3). A template or a text block
// spans the whole literal.
func (p *parser) badTypeText(t Token, where string) {
	if t.Kind == Illegal {
		p.unexpected()
	}
	p.tmpl = typeTextTemplate(t, where)
	s, e := t.ErrStart, t.ErrEnd
	switch {
	case t.Kind == TemplateHead:
		depth := 0
		for i := p.i; i < len(p.toks); i++ {
			switch p.toks[i].Kind {
			case TemplateHead:
				depth++
			case TemplateTail:
				depth--
				if depth == 0 {
					e = p.toks[i].End
					i = len(p.toks)
				}
			}
		}
		s = t.Start
	case t.Kind == TokString:
		s, e = t.Start, t.End
	}
	p.fail(diag.CodeBadTypeText, s, e, "this is not type text here: "+t.describe())
}

// ptype parses a PType: alternatives joined by "|" (2.1, 2.3).
func (p *parser) ptype() *PType {
	t := &PType{Span: Span{Start: p.tok.Start}}
	for {
		t.Alts = append(t.Alts, p.palt())
		if p.tok.Kind != Pipe {
			break
		}
		p.next()
	}
	t.Span.End = p.prevEnd
	return t
}

// palt parses a PPrim and its [] suffixes. Each suffix's [ opens a level
// that lasts to the end of the PAlt (2.8).
func (p *parser) palt() *PAlt {
	a := &PAlt{Span: Span{Start: p.tok.Start}}
	a.Prim = p.pprim()
	opened := 0
	for p.tok.Kind == LBrack && p.peek(1).Kind == RBrack {
		p.enter(p.tok.Start, p.tok.End)
		opened++
		p.next()
		p.next()
		a.Lists++
	}
	p.level -= opened
	a.Span.End = p.prevEnd
	return a
}

func (p *parser) pprim() *PPrim {
	t := p.tok
	pr := &PPrim{Span: Span{Start: t.Start}}
	switch {
	case t.Kind == Word && typeWords[t.Text]:
		pr.Kind, pr.Text = PWord, t.Text
		p.next()
	case t.isWord("map"):
		p.next()
		lt := p.tok
		if lt.Kind != Lt {
			p.badTypeText(lt, "map-open")
		}
		p.next()
		p.enter(lt.Start, lt.End)
		pr.Kind = PMap
		pr.Elem = p.ptype()
		if p.tok.Kind != Gt {
			p.badTypeText(p.tok, "map-close")
		}
		p.next()
		p.exit()
	case t.Kind == Word && IsTypeName(t.Text):
		pr.Kind, pr.Text = PName, t.Text
		p.next()
	case t.Kind == TokString && !t.Block && t.Form != '`':
		pr.Kind, pr.Text = PString, t.Str
		p.next()
	case t.Kind == LParen:
		p.next()
		p.enter(t.Start, t.End)
		pr.Kind = PParen
		pr.Elem = p.ptype()
		if p.tok.Kind != RParen {
			p.badTypeText(p.tok, "paren")
		}
		p.next()
		p.exit()
	case t.Kind == LBrace:
		p.precord(pr)
	default:
		p.badTypeText(t, "type")
	}
	pr.Span.End = p.prevEnd
	return pr
}

func (p *parser) precord(pr *PPrim) {
	open := p.tok
	p.next()
	p.enter(open.Start, open.End)
	pr.Kind = PRecord
	seen := map[string]bool{}
	for p.tok.Kind != RBrace {
		if p.tok.Kind == Ellipsis {
			p.next()
			pr.HasRest = true
			if p.tok.Kind != Comma && p.tok.Kind != RBrace {
				pr.Rest = p.ptype()
			}
			if p.tok.Kind == Comma {
				p.next()
			}
			if p.tok.Kind != RBrace {
				p.badTypeText(p.tok, "record") // a field after a rest, or no "}"
			}
			break
		}
		kt := p.tok
		f := &PField{KeySpan: Span{kt.Start, kt.End}}
		switch {
		case kt.Kind == Word:
			f.Key = kt.Text
		case kt.Kind == TokString && !kt.Block && kt.Form != '`':
			f.Key = kt.Str
		default:
			p.badTypeText(kt, "record")
		}
		if seen[f.Key] {
			p.key = f.Key
			p.badTypeText(kt, "dup")
		}
		seen[f.Key] = true
		p.next()
		if p.tok.Kind == Question {
			f.Optional = true
			p.next()
		}
		if p.tok.Kind != Colon {
			p.badTypeText(p.tok, "record")
		}
		p.next()
		f.Type = p.ptype()
		pr.Fields = append(pr.Fields, f)
		if p.tok.Kind == Comma {
			p.next()
			continue
		}
		if p.tok.Kind != RBrace {
			p.badTypeText(p.tok, "record")
		}
	}
	p.next() // }
	p.exit()
}

// String prints the program type as it is written, with single spaces.
func (t *PType) String() string {
	parts := make([]string, len(t.Alts))
	for i, a := range t.Alts {
		parts[i] = a.Prim.String() + strings.Repeat("[]", a.Lists)
	}
	return strings.Join(parts, " | ")
}

// String prints the primary.
func (p *PPrim) String() string {
	switch p.Kind {
	case PWord, PName:
		return p.Text
	case PString:
		return strconv.Quote(p.Text)
	case PMap:
		return "map<" + p.Elem.String() + ">"
	case PParen:
		return "(" + p.Elem.String() + ")"
	}
	var b strings.Builder
	b.WriteString("{")
	for i, f := range p.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(f.Key))
		if f.Optional {
			b.WriteString("?")
		}
		b.WriteString(": " + f.Type.String())
	}
	if p.HasRest {
		if len(p.Fields) > 0 {
			b.WriteString(", ")
		}
		b.WriteString("...")
		if p.Rest != nil {
			b.WriteString(p.Rest.String())
		}
	}
	b.WriteString("}")
	return b.String()
}

// typeTextTemplate chooses the template of bad-type-text (F8 3.4).
func typeTextTemplate(t Token, where string) string {
	if t.Kind == TemplateHead || (t.Kind == TokString && (t.Block || t.Form == '`')) {
		return "literal"
	}
	switch where {
	case "map-open", "paren", "record":
		return where
	case "map-close":
		if t.Kind == Ge {
			return "split"
		}
		return "map-close"
	case "dup":
		return "duplicate"
	}
	switch {
	case t.Kind == TokNumber || t.Kind == Minus || t.isWord("true") || t.isWord("false"):
		return "value"
	case t.isWord("number"):
		return "number-word"
	case t.isWord("boolean"):
		return "boolean-word"
	case t.isWord("undefined") || t.isWord("void") || t.isWord("nil"):
		return "null-word"
	case t.isWord("object"):
		return "object-word"
	case t.Kind == Word && len(t.Text) == 1 && t.Text[0] >= 'A' && t.Text[0] <= 'Z':
		return "letter"
	case t.Kind == Word:
		return "lower"
	}
	return "expected"
}
