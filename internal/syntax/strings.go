package syntax

import (
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
)

// This file is follow-on package F3, string forms (spec/packages/
// f03-strings.md): the raw string r"..." and r'...', and the text block """
// and r""".

// rawString reads a raw string (F3 3). The word r is read; the quote is at
// the current place.
func (l *lexer) rawString(start int, q rune) *Error {
	l.pos++
	cs := l.pos
	for {
		if l.pos >= len(l.src) || isLineEnd(l.src[l.pos]) {
			return l.errf(diag.CodeUnterminatedString, start, l.pos, "the raw string has no closing quote")
		}
		if l.src[l.pos] == q {
			v := string(l.src[cs:l.pos])
			l.pos++
			l.emit(Token{Kind: TokString, Start: start, End: l.pos, Str: v, Form: 'r'})
			return nil
		}
		l.pos++
	}
}

// Block element kinds.
const (
	elChar uint8 = iota
	elEsc
	elLineEnd
	elInterp
)

// blockEl is one element of a text block's content, in source order: a code
// point, an escape, a line end in text, or the start of an interpolation.
type blockEl struct {
	kind uint8
	src  int // offset of the element in the source
	val  string
}

// blockLex is a text block being read.
type blockLex struct {
	start  int // the first code point: the " or the r
	raw    bool
	els    []blockEl
	pieces []int // token indices of the block's pieces, in order
}

func isSpaceTab(r rune) bool { return r == ' ' || r == '\t' }

// textBlock reads a text block (F3 4). The current place holds the opening
// delimiter's """; start is its first code point, the r of a raw block.
func (l *lexer) textBlock(start int, raw bool) *Error {
	l.pos += 3
	delimEnd := l.pos
	// The opening line (4.1).
	p := l.pos
	for p < len(l.src) && isSpaceTab(l.src[p]) {
		p++
	}
	switch {
	case p >= len(l.src):
		return l.errf(diag.CodeUnterminatedString, start, len(l.src), "the text block has no closing delimiter")
	case isLineEnd(l.src[p]):
		if l.src[p] == '\r' && p+1 < len(l.src) && l.src[p+1] == '\n' {
			p++
		}
		l.pos = p + 1
	default:
		// text-block-open: the literal runs raw to the next """ (4.1 item 3).
		l.pos = delimEnd
		for l.pos < len(l.src) {
			if l.src[l.pos] == '"' && l.peek(1) == '"' && l.peek(2) == '"' {
				l.pos += 3
				break
			}
			l.pos++
		}
		l.emit(Token{Kind: TokString, Start: start, End: l.pos, Form: blockForm(raw), Block: true,
			BlockErr: &Error{Code: diag.CodeTextBlockOpen, Start: start, End: delimEnd,
				Message: "a text block's first line must end after its opening delimiter"}})
		return nil
	}
	b := &blockLex{start: start, raw: raw}
	return l.blockText(b, start)
}

func blockForm(raw bool) rune {
	if raw {
		return 'R'
	}
	return 'T'
}

// blockText reads a text block's content from the current place, up to the
// next interpolation or the closing delimiter (4.2). pieceStart is where
// the piece token starts: the block's start, or the } of an interpolation.
func (l *lexer) blockText(b *blockLex, pieceStart int) *Error {
	for {
		if l.pos >= len(l.src) {
			return l.errf(diag.CodeUnterminatedString, b.start, len(l.src), "the text block has no closing delimiter")
		}
		c := l.src[l.pos]
		switch {
		case c == '"' && l.peek(1) == '"' && l.peek(2) == '"':
			closeStart := l.pos
			l.pos += 3
			t := Token{Start: pieceStart, End: l.pos, Form: blockForm(b.raw), Block: true}
			if len(b.pieces) == 0 {
				t.Kind = TokString
			} else {
				t.Kind = TemplateTail
				t.ErrStart, t.ErrEnd = pieceStart, pieceStart+1
				l.toks[b.pieces[0]].ErrEnd = l.pos
			}
			b.pieces = append(b.pieces, len(l.toks))
			l.emit(t)
			l.finishBlock(b, closeStart)
			return nil
		case !b.raw && c == '\\':
			bs := l.pos
			var sb strings.Builder
			stop, err := l.escape(&sb, true)
			if err != nil {
				return err
			}
			if stop {
				return l.errf(diag.CodeUnterminatedString, b.start, len(l.src), "the text block has no closing delimiter")
			}
			b.els = append(b.els, blockEl{kind: elEsc, src: bs, val: sb.String()})
		case !b.raw && c == '$' && l.peek(1) == '{':
			b.els = append(b.els, blockEl{kind: elInterp, src: l.pos})
			l.pos += 2
			t := Token{Start: pieceStart, End: l.pos, Form: blockForm(b.raw), Block: true}
			if len(b.pieces) == 0 {
				t.Kind = TemplateHead
				t.ErrStart, t.ErrEnd = pieceStart, l.pos
			} else {
				t.Kind = TemplateMiddle
				t.ErrStart, t.ErrEnd = pieceStart, pieceStart+1
			}
			b.pieces = append(b.pieces, len(l.toks))
			l.emit(t)
			l.interps = append(l.interps, interp{tmplStart: b.start, headIdx: b.pieces[0], blk: b})
			return nil
		case isLineEnd(c):
			b.els = append(b.els, blockEl{kind: elLineEnd, src: l.pos})
			l.pos++
			if c == '\r' && l.peek(0) == '\n' {
				l.pos++
			}
		default:
			b.els = append(b.els, blockEl{kind: elChar, src: l.pos, val: string(c)})
			l.pos++
		}
	}
}

// finishBlock removes the indentation, gives the pieces their text, and
// keeps the block's first diagnostic on its first token (4.3 to 4.6, 6.1).
func (l *lexer) finishBlock(b *blockLex, closeStart int) {
	var lines [][]blockEl
	cur := []blockEl{}
	for _, e := range b.els {
		if e.kind == elLineEnd {
			lines = append(lines, cur)
			cur = []blockEl{}
			continue
		}
		cur = append(cur, e)
	}
	closing := cur
	first := &l.toks[b.pieces[0]]
	var indent []string
	for _, e := range closing {
		if e.kind != elChar || !isSpaceTab([]rune(e.val)[0]) {
			first.BlockErr = &Error{Code: diag.CodeTextBlockClose, Start: closeStart, End: closeStart + 3,
				Message: "a text block's closing delimiter must stand on a line of its own"}
			return
		}
		indent = append(indent, e.val)
	}
	parts := []string{""}
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		blank := true
		for _, e := range line {
			if e.kind != elChar || !isSpaceTab([]rune(e.val)[0]) {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		for j := range indent {
			e := line[j]
			if e.kind != elChar || e.val != indent[j] {
				if first.BlockErr == nil {
					first.BlockErr = &Error{Code: diag.CodeTextBlockIndent, Start: e.src, End: e.src + 1,
						Message: "this line does not start with the indentation of the closing line"}
				}
				break
			}
		}
		if first.BlockErr != nil {
			continue
		}
		for _, e := range line[len(indent):] {
			if e.kind == elInterp {
				parts[len(parts)-1] = sb.String()
				sb.Reset()
				parts = append(parts, "")
				continue
			}
			sb.WriteString(e.val)
		}
	}
	if first.BlockErr != nil {
		return
	}
	parts[len(parts)-1] = sb.String()
	for i, idx := range b.pieces {
		l.toks[idx].Str = parts[i]
	}
}
