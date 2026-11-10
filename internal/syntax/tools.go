package syntax

import "github.com/deepnoodle-ai/risor/v3/internal/diag"

// This file gives the tools of package F12 (spec/packages/f12-tools.md)
// what they read of the lexer and the parser.

// PartTokens lexes the part of a template that a segment holds again, and
// returns its tokens: they end with an EOF token at its closing brace.
func PartTokens(f *File, s Segment) []Token {
	l := &lexer{src: f.src, pos: s.Span.Start + 3}
	if _, _, err := l.part(s.Span.Start); err != nil {
		return nil
	}
	return l.toks
}

// closeAtCut ends each interpolation left open at the end of the text, the
// innermost first, with an empty closing token, then the text (F12 3.2 item
// 4).
func (l *lexer) closeAtCut() {
	for n := len(l.interps); n > 0; n = len(l.interps) {
		top := l.interps[n-1]
		l.interps = l.interps[:n-1]
		t := Token{Kind: TemplateTail, Start: l.pos, End: l.pos, Form: '`'}
		if top.blk != nil {
			t.Form, t.Block = blockForm(top.blk.raw), true
		}
		l.emit(t)
	}
	l.emit(Token{Kind: EOF, Start: l.pos, End: l.pos})
}

// CutLex lexes the text before a cut (F12 3.2 item 4). Each interpolation
// the cut leaves open ends at the cut with an empty closing token. It
// returns the tokens, ending with EOF, the index of the first closing token
// it added (or of the EOF), and the first lexical diagnostic.
func CutLex(src []rune) ([]Token, int, *Error) {
	l := &lexer{src: src, cut: true}
	if err := l.run(); err != nil {
		return nil, 0, err
	}
	end := len(l.toks) - 1
	for end > 0 && l.toks[end-1].Kind == TemplateTail && l.toks[end-1].Start == l.toks[end-1].End {
		end--
	}
	return l.toks, end, nil
}

// HoleText is the text of the hole of F12 3.4, a token no text can spell
// at its place: the parser reads it as a Name, a Word or a type name.
const HoleText = "Zz_hole_"

// repairTokens are the tokens the repair inserts, in order (F12 3.3 item 1).
var repairTokens = []Token{
	{Kind: Word, Text: "null"}, {Kind: RParen}, {Kind: RBrack}, {Kind: RBrace},
	{Kind: Semi}, {Kind: Colon}, {Kind: Arrow}, {Kind: Assign}, {Kind: LBrace},
	{Kind: Word, Text: "else"}, {Kind: Word, Text: "catch"}, {Kind: Gt},
}

// Repair parses tokens of a text before a cut (F12 3.3). The tokens from
// index end on are the end of the text: the closing tokens of CutLex and
// the EOF. Where the parser reports a diagnostic at the end, it inserts
// the closer of the innermost open bracket when the parser accepts it
// there, else the first token of the list that it accepts, and goes on
// until the program is complete. A ( whose matching ) is not among the
// tokens before the end begins a parenthesized expression (3.3 item 7).
// It returns nil when the repair fails.
func Repair(toks []Token, end int, src []rune) *Program {
	return repair(toks, end, src)
}

// innermostCloser is the closing bracket of the innermost bracket still
// open in toks, and whether one is open.
func innermostCloser(toks []Token) (Token, bool) {
	var stack []TokenKind
	for _, t := range toks {
		switch t.Kind {
		case LParen:
			stack = append(stack, RParen)
		case LBrack, OptLBrack:
			stack = append(stack, RBrack)
		case LBrace:
			stack = append(stack, RBrace)
		case RParen, RBrack, RBrace:
			if n := len(stack); n > 0 {
				stack = stack[:n-1]
			}
		}
	}
	if len(stack) == 0 {
		return Token{}, false
	}
	return Token{Kind: stack[len(stack)-1]}, true
}

func repair(toks []Token, end int, src []rune) *Program {
	cur := append([]Token(nil), toks...)
	for iter := 0; iter < 10000; iter++ {
		prog, at := parseTokens(cur, end, src)
		if prog != nil {
			return prog
		}
		if at < end {
			return nil
		}
		accepted := false
		cands := repairTokens
		if closer, ok := innermostCloser(cur[:at]); ok { // the closer first
			cands = append([]Token{closer}, repairTokens...)
		}
		for _, c := range cands {
			c.Start, c.End = cur[at].Start, cur[at].Start
			c.ErrStart, c.ErrEnd = c.Start, c.End
			next := make([]Token, 0, len(cur)+1)
			next = append(append(append(next, cur[:at]...), c), cur[at:]...)
			p2, at2 := parseTokens(next, end, src)
			if p2 != nil {
				return p2
			}
			if at2 > at {
				cur, accepted = next, true
				break
			}
		}
		if !accepted {
			return nil
		}
	}
	return nil
}

// parseTokens parses a program from tokens. It returns the program, or nil
// and the index of the token where the parser reported a diagnostic.
func parseTokens(toks []Token, end int, src []rune) (prog *Program, at int) {
	p := &parser{toks: toks, tok: toks[0], src: src, zone: end + 1}
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bailout)
			if !ok {
				panic(r)
			}
			prog, at = nil, -1
			if b.err.Code != diag.CodeTextBlockOpen && b.err.Code != diag.CodeTextBlockClose && b.err.Code != diag.CodeTextBlockIndent {
				at = b.err.at
			}
		}
	}()
	p.reach()
	return p.program(), 0
}

// ParseTokens parses a program from the tokens of a whole text, as Parse
// does, without the targeting of F8. It returns nil when it does not
// parse.
func ParseTokens(toks []Token, src []rune) *Program {
	p, _ := parseTokens(toks, len(toks), src)
	return p
}
