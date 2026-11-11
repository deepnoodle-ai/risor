package syntax

import (
	"strings"

	"github.com/deepnoodle-ai/risor/go/v3/internal/diag"
)

// This file is the template parse of package F2, 2.1 to 2.5
// (spec/packages/f02-templates-analysis.md): a host text with ${{ }} parts.

// FormHost is the form of the Template or String node a host template gives
// (F2 2.5 item 4). Evaluation never reads it.
const FormHost = '$'

// Segment is a text run or a part of a template (F2 2.4), as offsets in the
// template text.
type Segment struct {
	// Expr is the part's expression; nil for a text run.
	Expr Node
	// Text is a text run's value, with each $${{ read as ${{.
	Text string
	// Span is the whole segment: a text run's code points, or a part from
	// the $ of its ${{ to the end of its closing delimiter.
	Span Span
}

// HostTemplate is a parsed template.
type HostTemplate struct {
	Segments []Segment
	// Single is set when the template is exactly one part.
	Single bool
	// String and Value are the programs of the two modes (F2 2.5).
	String, Value *Program
}

// NewTemplateFile builds a File from a template text. Unlike NewFile, it
// keeps a U+FEFF at the start, which is a code point of the text (F2 2.1
// item 2).
func NewTemplateFile(source string) *File {
	src := []rune(source)
	starts := []int{0}
	for i, r := range src {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &File{src: src, lineStarts: starts}
}

// part is a part's tokens while the template is read.
type part struct {
	seg    int
	dollar int
	toks   []Token
	delim  bool // it ends with the closing delimiter }}, not a lone }
}

// ParseTemplate reads a template text (F2 2.1, 2.2) and parses its parts. It
// returns the first diagnostic by F2 2.3 item 3: a lexical diagnostic or an
// unterminated part first, in text order, then the first the parser meets.
func ParseTemplate(f *File) (*HostTemplate, *Error) {
	src := f.src
	var segs []Segment
	var parts []part
	var text strings.Builder
	textStart := 0
	flush := func(end int) {
		if end > textStart {
			segs = append(segs, Segment{Text: text.String(), Span: Span{textStart, end}})
		}
		text.Reset()
	}
	at := func(i int, s string) bool {
		for k, r := range s {
			if i+k >= len(src) || src[i+k] != r {
				return false
			}
		}
		return true
	}
	for pos := 0; pos < len(src); {
		switch {
		case at(pos, "$${{"):
			text.WriteString("${{")
			pos += 4
		case at(pos, "${{"):
			flush(pos)
			l := &lexer{src: src, pos: pos + 3}
			closeAt, delim, err := l.part(pos)
			if err != nil {
				return nil, targetLexical(err, src)
			}
			end := closeAt + 1
			if delim {
				end++
			}
			parts = append(parts, part{seg: len(segs), dollar: pos, toks: l.toks, delim: delim})
			segs = append(segs, Segment{Span: Span{pos, end}})
			pos = end
			textStart = pos
		default:
			text.WriteRune(src[pos])
			pos++
		}
	}
	flush(len(src))
	for _, pt := range parts {
		e, err := parsePart(pt, src)
		if err != nil {
			return nil, err
		}
		segs[pt.seg].Expr = e
	}
	t := &HostTemplate{Segments: segs, Single: len(segs) == 1 && segs[0].Expr != nil}
	whole := base{span: Span{0, len(src)}}
	if len(parts) == 0 {
		var b strings.Builder
		for _, s := range segs {
			b.WriteString(s.Text)
		}
		t.String = &Program{base: whole, Result: &String{base: whole, Value: b.String(), Form: FormHost}}
	} else {
		tp := make([]TemplatePart, len(segs))
		for i, s := range segs {
			tp[i] = TemplatePart{Text: s.Text, Expr: s.Expr}
		}
		t.String = &Program{base: whole, Result: &Template{base: whole, Parts: tp}}
	}
	t.Value = t.String
	if t.Single {
		e := segs[0].Expr
		t.Value = &Program{base: base{span: e.Span(), depth: 0}, Result: e}
	}
	return t, nil
}

// part reads the tokens of a part, from after its ${{ (F2 2.2). It returns
// the offset of the closing brace, and whether a } follows it to make the
// closing delimiter. The tokens end with an EOF token whose error span is the
// closing delimiter or the closing brace.
func (l *lexer) part(dollar int) (closeAt int, delim bool, err *Error) {
	depth := 0
	for {
		if err := l.skipSpace(); err != nil {
			return 0, false, err
		}
		if l.pos >= len(l.src) {
			if n := len(l.interps); n > 0 {
				start := l.interps[n-1].tmplStart
				return 0, false, l.errf(diag.CodeUnterminatedString, start, len(l.src), "the template has no closing backtick")
			}
			return 0, false, l.errf(diag.CodeUnterminatedString, dollar, len(l.src), "the part has no closing }}")
		}
		c := l.src[l.pos]
		if n := len(l.interps); n > 0 && c == '}' && l.interps[n-1].braces == 0 {
			top := l.interps[n-1]
			l.interps = l.interps[:n-1]
			var err *Error
			if top.blk != nil {
				l.pos++
				err = l.blockText(top.blk, l.pos-1)
			} else {
				err = l.templateText(top.tmplStart, l.pos, top.headIdx)
			}
			if err != nil {
				return 0, false, err
			}
			continue
		}
		outside := len(l.interps) == 0
		if outside && c == '}' && depth == 0 {
			closeAt = l.pos
			delim = l.peek(1) == '}'
			end := closeAt + 1
			if delim {
				end++
			}
			l.toks = append(l.toks, Token{Kind: EOF, Start: closeAt, End: closeAt, ErrStart: closeAt, ErrEnd: end})
			return closeAt, delim, nil
		}
		n := len(l.toks)
		if err := l.token(c); err != nil {
			return 0, false, err
		}
		if outside && len(l.toks) > n {
			switch l.toks[len(l.toks)-1].Kind {
			case LBrace:
				depth++
			case RBrace:
				depth--
			}
		}
	}
}

// parsePart parses a part's tokens as one Expr, with one level open for the
// ${{ (F2 2.2 items 8, 9 and 11).
func parsePart(pt part, src []rune) (e Node, err *Error) {
	p := &parser{toks: pt.toks, tok: pt.toks[0], src: src}
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bailout)
			if !ok {
				panic(r)
			}
			e, err = nil, p.target(b.err)
		}
	}()
	p.reach()
	p.enter(pt.dollar, pt.dollar+3)
	e = p.expr()
	if p.tok.Kind != EOF {
		p.unexpected()
	}
	if !pt.delim {
		p.unexpectedAt(p.tok) // F2 2.2 item 5: a lone closing brace
	}
	return e, nil
}

// ProgramOf is the program whose result is n, with n's span: a part of a
// template or an argument of a call, given alone (F2 3.6 item 5).
func ProgramOf(n Node) *Program {
	return &Program{base: base{span: n.Span()}, Result: n}
}
