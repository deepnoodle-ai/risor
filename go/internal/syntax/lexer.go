package syntax

import (
	"strconv"
	"strings"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
)

// Error is a parse diagnostic before it is placed in a file: a code, a span
// as code point offsets, and a message.
type Error struct {
	Code    string
	Start   int
	End     int
	Message string
	core    *core

	// Template names the catalog template of the message (package F8 3.4);
	// empty for the code's only template or before the mapping. Key and Op
	// are parameters some templates take.
	Template string
	Key, Op  string
	// Rule is the rule of Core that gave a parser's diagnostic, for the
	// mapping of F8 2; at, ctx and q are the parser's state then.
	Rule string
	at   int
	ctx  []frame
	q    int
}

// Error returns the code and the message.
func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Diagnostic turns the error into a diagnostic of phase parse.
func (e *Error) Diagnostic(f *File) diag.Diagnostic {
	sp := f.Span(e.Start, e.End)
	return diag.Diagnostic{Phase: diag.PhaseParse, Code: e.Code, Span: &sp, Message: buildMessage(e, f.src)}
}

const maxWhole = 9007199254740991 // 2^53 - 1

// lexer divides the whole text into tokens (01-language.md 2). It stops at
// the first lexical diagnostic, which is the first diagnostic of the parse
// (2.11).
type lexer struct {
	src  []rune
	pos  int
	toks []Token
	// interps holds, for each interpolation that is open, the offset of the
	// opening backtick of its template and the count of "{" open inside it.
	interps []interp
	// cut is set for the text before a cut of package F12 (tools.go): an
	// interpolation the end of the text leaves open ends there.
	cut bool
}

type interp struct {
	tmplStart int
	braces    int
	headIdx   int       // index of the template's TemplateHead token
	blk       *blockLex // the text block the interpolation is in, or nil
}

// Lex divides the text of f into tokens. The last token is EOF. It returns
// the first lexical diagnostic, if there is one.
func Lex(f *File) ([]Token, *Error) {
	l := &lexer{src: f.src}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.toks, nil
}

func (l *lexer) errf(code string, start, end int, msg string) *Error {
	return &Error{Code: code, Start: start, End: end, Message: msg}
}

func (l *lexer) peek(k int) rune {
	if l.pos+k < len(l.src) {
		return l.src[l.pos+k]
	}
	return -1
}

func (l *lexer) emit(t Token) {
	if t.ErrStart == 0 && t.ErrEnd == 0 {
		t.ErrStart, t.ErrEnd = t.Start, t.End
	}
	l.toks = append(l.toks, t)
}

func (l *lexer) run() *Error {
	for {
		if err := l.skipSpace(); err != nil {
			return err
		}
		if l.pos >= len(l.src) {
			if n := len(l.interps); n > 0 && l.cut {
				l.closeAtCut()
				return nil
			}
			if n := len(l.interps); n > 0 {
				// The innermost literal still open names the message (F8
				// 3.4): a text block, or a template.
				top := l.interps[n-1]
				if top.blk != nil {
					return l.errf(diag.CodeUnterminatedString, top.tmplStart, len(l.src), "the text block has no closing delimiter")
				}
				return l.errf(diag.CodeUnterminatedString, top.tmplStart, len(l.src), "the template has no closing backtick")
			}
			l.emit(Token{Kind: EOF, Start: l.pos, End: l.pos})
			return nil
		}
		c := l.src[l.pos]
		if n := len(l.interps); n > 0 && c == '}' && l.interps[n-1].braces == 0 {
			top := l.interps[n-1]
			l.interps = l.interps[:n-1]
			var err *Error
			if top.blk != nil {
				l.pos++ // the } that closes the interpolation
				err = l.blockText(top.blk, l.pos-1)
			} else {
				err = l.templateText(top.tmplStart, l.pos, top.headIdx)
			}
			if err != nil {
				return err
			}
			continue
		}
		if err := l.token(c); err != nil {
			return err
		}
	}
}

func isLineEnd(r rune) bool { return r == '\n' || r == '\r' }

func (l *lexer) skipSpace() *Error {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			l.pos++
		case c == '/' && l.peek(1) == '/':
			for l.pos < len(l.src) && !isLineEnd(l.src[l.pos]) {
				l.pos++
			}
		case c == '/' && l.peek(1) == '*':
			start := l.pos
			l.pos += 2
			for {
				if l.pos >= len(l.src) {
					return l.errf(diag.CodeUnterminatedComment, start, start+2, "the comment has no closing */")
				}
				if l.src[l.pos] == '*' && l.peek(1) == '/' {
					l.pos += 2
					break
				}
				l.pos++
			}
		default:
			return nil
		}
	}
	return nil
}

func isDigit(r rune) bool     { return r >= '0' && r <= '9' }
func isLetter(r rune) bool    { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }
func isWordStart(r rune) bool { return isLetter(r) || r == '_' }
func isWordPart(r rune) bool  { return isWordStart(r) || isDigit(r) }
func isHex(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func (l *lexer) token(c rune) *Error {
	start := l.pos
	switch {
	case isDigit(c) || (c == '.' && isDigit(l.peek(1))):
		return l.number()
	case isWordStart(c):
		for l.pos < len(l.src) && isWordPart(l.src[l.pos]) {
			l.pos++
		}
		w := string(l.src[start:l.pos])
		// F3 2: r directly before a quote begins a raw string or a raw text
		// block; r''' stays the prefixed literal.
		if w == "r" {
			switch q := l.peek(0); {
			case q == '"' && l.peek(1) == '"' && l.peek(2) == '"':
				return l.textBlock(start, true)
			case q == '\'' && l.peek(1) == '\'' && l.peek(2) == '\'':
			case q == '"' || q == '\'':
				return l.rawString(start, q)
			}
		}
		if q := l.peek(0); (q == '"' || q == '\'' || q == '`') && !reserved[w] {
			l.prefixed(start, q)
			return nil
		}
		l.emit(Token{Kind: Word, Start: start, End: l.pos, Text: w})
		return nil
	case c == '"' || c == '\'':
		if l.peek(1) == c && l.peek(2) == c {
			if c == '"' {
				return l.textBlock(start, false) // F3 4
			}
			l.blockToken(c)
			return nil
		}
		return l.oneLineString(c)
	case c == '`':
		return l.templateText(start, start, -1)
	}
	kind, n := l.punct(c)
	if n == 0 {
		return l.errf(diag.CodeUnexpectedCharacter, start, start+1, "this character begins no token")
	}
	l.pos += n
	t := Token{Kind: kind, Start: start, End: l.pos}
	if kind == Illegal {
		t.Text = string(l.src[start:l.pos])
	}
	if n := len(l.interps); n > 0 {
		switch kind {
		case LBrace:
			l.interps[n-1].braces++
		case RBrace:
			l.interps[n-1].braces--
		}
	}
	l.emit(t)
	return nil
}

// punct reads the longest punctuation token at the current place. A count of
// zero means that c begins no token.
func (l *lexer) punct(c rune) (TokenKind, int) {
	p1, p2 := l.peek(1), l.peek(2)
	switch c {
	case '(':
		return LParen, 1
	case ')':
		return RParen, 1
	case '[':
		return LBrack, 1
	case ']':
		return RBrack, 1
	case '{':
		return LBrace, 1
	case '}':
		return RBrace, 1
	case ',':
		return Comma, 1
	case ';':
		return Semi, 1
	case ':':
		return Colon, 1
	case '.':
		if p1 == '.' && p2 == '.' {
			return Ellipsis, 3
		}
		return Dot, 1
	case '?':
		switch {
		case p1 == '.' && p2 == '[':
			return OptLBrack, 3
		case p1 == '.':
			return OptDot, 2
		case p1 == '?' && p2 == '=':
			return Illegal, 3
		case p1 == '?':
			return Coalesce, 2
		}
		return Question, 1
	case '=':
		switch {
		case p1 == '=' && p2 == '=':
			return Illegal, 3
		case p1 == '=':
			return Eq, 2
		case p1 == '>':
			return Arrow, 2
		}
		return Assign, 1
	case '!':
		switch {
		case p1 == '=' && p2 == '=':
			return Illegal, 3
		case p1 == '=':
			return Ne, 2
		}
		return Bang, 1
	case '|':
		switch {
		case p1 == '|' && p2 == '=':
			return Illegal, 3
		case p1 == '|':
			return OrOr, 2
		case p1 == '>':
			return Illegal, 2
		}
		return Pipe, 1
	case '&':
		switch {
		case p1 == '&' && p2 == '=':
			return Illegal, 3
		case p1 == '&':
			return AndAnd, 2
		}
		return 0, 0
	case '-':
		if p1 == '=' {
			return Illegal, 2
		}
		return Minus, 1
	case '+':
		if p1 == '+' || p1 == '=' {
			return Illegal, 2
		}
		return Plus, 1
	case '*':
		switch {
		case p1 == '*' && p2 == '=':
			return Illegal, 3
		case p1 == '*':
			return Power, 2
		case p1 == '=':
			return Illegal, 2
		}
		return Star, 1
	case '/':
		if p1 == '=' {
			return Illegal, 2
		}
		return Slash, 1
	case '%':
		if p1 == '=' {
			return Illegal, 2
		}
		return Percent, 1
	case '<':
		if p1 == '=' {
			return Le, 2
		}
		return Lt, 1
	case '>':
		if p1 == '=' {
			return Ge, 2
		}
		return Gt, 1
	}
	return 0, 0
}

// blockToken reads the block token of 2.3 rule 7.
func (l *lexer) blockToken(q rune) {
	start := l.pos
	l.pos += 3
	for l.pos < len(l.src) {
		if l.src[l.pos] == q && l.peek(1) == q && l.peek(2) == q {
			l.pos += 3
			break
		}
		l.pos++
	}
	l.emit(Token{Kind: Illegal, Start: start, End: l.pos, Text: string(l.src[start:l.pos]),
		ErrStart: start, ErrEnd: start + 3})
}

// prefixed reads the prefixed literal of 2.3 rule 9. The word is already
// read; the quote q is at the current place.
func (l *lexer) prefixed(start int, q rune) {
	wordEnd := l.pos
	l.pos++
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == q {
			l.pos++
			break
		}
		if q != '`' && isLineEnd(c) {
			break
		}
		l.pos++
	}
	l.emit(Token{Kind: Illegal, Start: start, End: l.pos, Text: string(l.src[start:l.pos]),
		ErrStart: start, ErrEnd: wordEnd})
}

// number reads a number token (2.5) and checks it against Number.
func (l *lexer) number() *Error {
	start := l.pos
	hex := l.src[l.pos] == '0' && (l.peek(1) == 'x' || l.peek(1) == 'X')
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case isWordPart(c):
			l.pos++
		case (c == '+' || c == '-') && !hex && l.pos > start && (l.src[l.pos-1] == 'e' || l.src[l.pos-1] == 'E'):
			l.pos++
		case c == '.':
			n := l.peek(1)
			if n == -1 || isDigit(n) || !(isLetter(n) || n == '_') {
				l.pos++
				continue
			}
			goto done
		default:
			goto done
		}
	}
done:
	text := string(l.src[start:l.pos])
	v, code := numberValue(text)
	if code != "" {
		msg := "this is not a valid number"
		if code == diag.CodeLiteralRange {
			msg = "this number is out of range"
		}
		return l.errf(code, start, l.pos, msg)
	}
	l.emit(Token{Kind: TokNumber, Start: start, End: l.pos, Text: text, Num: v})
	return nil
}

// numberValue checks a number token against the grammar Number of 2.5 and
// gives its value, or bad-number or literal-range.
func numberValue(s string) (float64, string) {
	i := 0
	digits := func(isD func(byte) bool) bool {
		// Digit { [ "_" ] Digit }
		if i >= len(s) || !isD(s[i]) {
			return false
		}
		i++
		for i < len(s) {
			if s[i] == '_' {
				if i+1 < len(s) && isD(s[i+1]) {
					i += 2
					continue
				}
				return false
			}
			if !isD(s[i]) {
				break
			}
			i++
		}
		return true
	}
	isDec := func(b byte) bool { return b >= '0' && b <= '9' }
	isHx := func(b byte) bool { return isHex(rune(b)) }
	if strings.HasPrefix(s, "0x") {
		i = 2
		if !digits(isHx) || i != len(s) {
			return 0, diag.CodeBadNumber
		}
		h := strings.TrimLeft(strings.ReplaceAll(s[2:], "_", ""), "0")
		if len(h) > 14 {
			return 0, diag.CodeLiteralRange
		}
		if h == "" {
			return 0, ""
		}
		u, err := strconv.ParseUint(h, 16, 64)
		if err != nil || u > maxWhole {
			return 0, diag.CodeLiteralRange
		}
		return float64(u), ""
	}
	// Int = "0" | NonZero { [ "_" ] Digit }
	if i < len(s) && s[i] == '0' {
		i++
	} else if !digits(isDec) {
		return 0, diag.CodeBadNumber
	}
	isInt := true
	if i < len(s) && s[i] == '.' {
		i++
		isInt = false
		if !digits(isDec) {
			return 0, diag.CodeBadNumber
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		isInt = false
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if !digits(isDec) {
			return 0, diag.CodeBadNumber
		}
	}
	if i != len(s) {
		return 0, diag.CodeBadNumber
	}
	clean := strings.ReplaceAll(s, "_", "")
	if isInt {
		if len(clean) > 16 {
			return 0, diag.CodeLiteralRange
		}
		u, err := strconv.ParseUint(clean, 10, 64)
		if err != nil || u > maxWhole {
			return 0, diag.CodeLiteralRange
		}
		return float64(u), ""
	}
	f, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		// The grammar is checked above, so the only failure left is a value
		// that is not finite. An underflow gives zero with no error.
		return 0, diag.CodeLiteralRange
	}
	return f, ""
}

// escape reads an escape at the current place, which holds "\". It appends
// the code point to b. A return of stop true with no error means the
// literal ends unterminated at the current place.
func (l *lexer) escape(b *strings.Builder, inTemplate bool) (stop bool, err *Error) {
	bs := l.pos
	n := l.peek(1)
	if n == -1 {
		l.pos++ // the "\" is part of the literal
		return true, nil
	}
	if isLineEnd(n) {
		if inTemplate {
			return true, l.errf(diag.CodeLineContinuation, bs, bs+1, "a \\ before a line end is not allowed in a template")
		}
		l.pos++ // the "\" is part of the literal; it stops at the line end
		return true, nil
	}
	simple := map[rune]rune{'"': '"', '\'': '\'', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}
	if r, ok := simple[n]; ok {
		b.WriteRune(r)
		l.pos += 2
		return false, nil
	}
	if inTemplate && (n == '`' || n == '$') {
		b.WriteRune(n)
		l.pos += 2
		return false, nil
	}
	if n != 'u' {
		return true, l.errf(diag.CodeBadEscape, bs, bs+2, "this escape is not valid")
	}
	if l.peek(2) == '{' {
		j := l.pos + 3
		for j < len(l.src) && isHex(l.src[j]) {
			j++
		}
		count := j - (l.pos + 3)
		if count == 0 || count > 6 || j >= len(l.src) || l.src[j] != '}' {
			end := j
			if count == 0 {
				end = l.pos + 3 // to the "{"
			}
			return true, l.errf(diag.CodeBadEscape, bs, end, "this \\u{...} escape is not valid")
		}
		v, _ := strconv.ParseUint(string(l.src[l.pos+3:j]), 16, 32)
		if v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF) {
			return true, l.errf(diag.CodeBadEscape, bs, j+1, "this escape names no code point")
		}
		b.WriteRune(rune(v))
		l.pos = j + 1
		return false, nil
	}
	v, ok := l.hex4(l.pos)
	if !ok {
		j := l.pos + 2
		for j < len(l.src) && j < l.pos+6 && isHex(l.src[j]) {
			j++
		}
		return true, l.errf(diag.CodeBadEscape, bs, j, "\\u needs four hex digits")
	}
	switch {
	case v >= 0xD800 && v <= 0xDBFF:
		if lo, ok := l.hex4(l.pos + 6); ok && lo >= 0xDC00 && lo <= 0xDFFF {
			b.WriteRune(rune(0x10000 + (v-0xD800)<<10 + (lo - 0xDC00)))
			l.pos += 12
			return false, nil
		}
		return true, l.errf(diag.CodeBadEscape, bs, bs+6, "this surrogate escape has no pair")
	case v >= 0xDC00 && v <= 0xDFFF:
		return true, l.errf(diag.CodeBadEscape, bs, bs+6, "this surrogate escape has no pair")
	}
	b.WriteRune(rune(v))
	l.pos += 6
	return false, nil
}

// hex4 reads "\uXXXX" at offset at.
func (l *lexer) hex4(at int) (uint32, bool) {
	if at+6 > len(l.src) || l.src[at] != '\\' || l.src[at+1] != 'u' {
		return 0, false
	}
	var v uint32
	for _, r := range l.src[at+2 : at+6] {
		if !isHex(r) {
			return 0, false
		}
		d, _ := strconv.ParseUint(string(r), 16, 8)
		v = v<<4 | uint32(d)
	}
	return v, true
}

// oneLineString reads "..." or '...' (2.6).
func (l *lexer) oneLineString(q rune) *Error {
	start := l.pos
	l.pos++
	var b strings.Builder
	for {
		if l.pos >= len(l.src) || isLineEnd(l.src[l.pos]) {
			return l.errf(diag.CodeUnterminatedString, start, l.pos, "the string has no closing quote")
		}
		c := l.src[l.pos]
		switch c {
		case q:
			l.pos++
			l.emit(Token{Kind: TokString, Start: start, End: l.pos, Str: b.String(), Form: q})
			return nil
		case '\\':
			stop, err := l.escape(&b, false)
			if err != nil {
				return err
			}
			if stop {
				return l.errf(diag.CodeUnterminatedString, start, l.pos, "the string has no closing quote")
			}
		default:
			b.WriteRune(c)
			l.pos++
		}
	}
}

// templateText reads a text part of a template (2.8), from the current place,
// which holds the opening backtick (head true) or the "}" that closes an
// interpolation. tmplStart is the offset of the template's backtick. headIdx
// is the index of the template's TemplateHead token, or -1 at the head.
func (l *lexer) templateText(tmplStart, pieceStart int, headIdx int) *Error {
	head := headIdx < 0
	l.pos++
	var b strings.Builder
	for {
		if l.pos >= len(l.src) {
			return l.errf(diag.CodeUnterminatedString, tmplStart, len(l.src), "the template has no closing backtick")
		}
		c := l.src[l.pos]
		switch {
		case c == '`':
			l.pos++
			t := Token{Start: pieceStart, End: l.pos, Str: b.String(), Form: '`'}
			if head {
				t.Kind = TokString
			} else {
				t.Kind = TemplateTail
				t.ErrStart, t.ErrEnd = pieceStart, pieceStart+1
				// The template as a whole is the span that names its head.
				l.toks[headIdx].ErrEnd = l.pos
			}
			l.emit(t)
			return nil
		case c == '$' && l.peek(1) == '{':
			l.pos += 2
			t := Token{Start: pieceStart, End: l.pos, Str: b.String(), Form: '`'}
			if head {
				t.Kind = TemplateHead
				t.ErrStart, t.ErrEnd = pieceStart, l.pos
				headIdx = len(l.toks)
			} else {
				t.Kind = TemplateMiddle
				t.ErrStart, t.ErrEnd = pieceStart, pieceStart+1
			}
			l.emit(t)
			l.interps = append(l.interps, interp{tmplStart: tmplStart, headIdx: headIdx})
			return nil
		case c == '\\':
			stop, err := l.escape(&b, true)
			if err != nil {
				return err
			}
			if stop {
				return l.errf(diag.CodeUnterminatedString, tmplStart, len(l.src), "the template has no closing backtick")
			}
		case c == '\r':
			b.WriteRune('\n')
			l.pos++
			if l.peek(0) == '\n' {
				l.pos++
			}
		default:
			b.WriteRune(c)
			l.pos++
		}
	}
}
