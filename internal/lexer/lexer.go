// Package lexer provides a Lexer that takes Risor source code as input and
// outputs a stream of tokens to be consumed by a parser.
package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/risor-io/risor/internal/token"
)

// NumberType describes the type of a number that is being lexed.
type NumberType string

const (
	NumberTypeInvalid NumberType = "invalid"
	NumberTypeDecimal NumberType = "decimal"
	NumberTypeHex     NumberType = "hex"
	NumberTypeOctal   NumberType = "octal"
	NumberTypeBinary  NumberType = "binary"
)

// Lexer holds our object-state.
type Lexer struct {
	// The index of the current character
	position int

	// The index of the next character
	nextPosition int

	// The current character
	ch rune

	// A rune slice of our input string
	characters []rune

	// Previous token
	prevToken token.Token

	// The index of the current line, for the current character
	line int

	// The character index where the current line began
	lineStart int

	// The column, within the current line, of the current character
	column int

	// The position of the start of the current token
	tokenStartPosition token.Position

	// Name of the file be read
	file string
}

// Option is a configuration function for a Lexer.
type Option func(*Lexer)

// WithFile sets the file name for the Lexer.
func WithFile(file string) Option {
	return func(l *Lexer) {
		l.file = file
	}
}

// New returns a Lexer instance for the given string input.
func New(input string, options ...Option) *Lexer {
	l := &Lexer{
		characters:   []rune(input),
		column:       -1, // -1 = before the first column
		position:     -1, // -1 = before the first character
		nextPosition: 0,  //  0 = read the first character next
	}
	for _, option := range options {
		option(l)
	}
	l.readChar()
	return l
}

// Filename returns the name of the file being read.
func (l *Lexer) Filename() string {
	return l.file
}

// State captures the lexer state for save/restore operations.
type State struct {
	position           int
	nextPosition       int
	ch                 rune
	prevToken          token.Token
	line               int
	lineStart          int
	column             int
	tokenStartPosition token.Position
}

// SaveState returns the current lexer state for later restoration.
func (l *Lexer) SaveState() State {
	return State{
		position:           l.position,
		nextPosition:       l.nextPosition,
		ch:                 l.ch,
		prevToken:          l.prevToken,
		line:               l.line,
		lineStart:          l.lineStart,
		column:             l.column,
		tokenStartPosition: l.tokenStartPosition,
	}
}

// RestoreState restores the lexer to a previously saved state.
func (l *Lexer) RestoreState(s State) {
	l.position = s.position
	l.nextPosition = s.nextPosition
	l.ch = s.ch
	l.prevToken = s.prevToken
	l.line = s.line
	l.lineStart = s.lineStart
	l.column = s.column
	l.tokenStartPosition = s.tokenStartPosition
}

// SetFilename sets the name of the file being read.
func (l *Lexer) SetFilename(file string) {
	l.file = file
}

// Position returns the current read position of the Lexer as a Position object.
func (l *Lexer) Position() token.Position {
	return token.Position{
		Char:      l.position,
		LineStart: l.lineStart,
		Line:      l.line,
		Column:    l.column,
		File:      l.file,
	}
}

// Next returns the next Token from the input that is being lexed.
func (l *Lexer) Next() (token.Token, error) {
	var tok token.Token
	l.skipTabsAndSpaces()
	l.tokenStartPosition = l.Position()

	// skip shebang line (only at start of file)
	if l.ch == rune('#') && l.peekChar() == rune('!') && l.line == 0 && l.position <= 1 {
		l.skipComment()
		return l.Next()
	}

	// skip single-line comments
	if l.ch == rune('/') && l.peekChar() == rune('/') {
		l.skipComment()
		return l.Next()
	}

	// multi-line comments
	if l.ch == rune('/') && l.peekChar() == rune('*') {
		l.skipMultiLineComment()
	}

	if l.prevToken.Type == token.EOF {
		// Once we encounter one null byte, stop reading the input
		return l.newToken(token.EOF, string(rune(0))), nil
	}

	switch l.ch {
	case rune('&'):
		if l.peekChar() == rune('&') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.AND, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.AMPERSAND, string(l.ch))
		}
	case rune('|'):
		if l.peekChar() == rune('|') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.OR, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.PIPE, string(l.ch))
		}
	case rune('^'):
		tok = l.newToken(token.CARET, string(l.ch))
	case rune('='):
		if l.peekChar() == rune('=') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.EQ, string(ch)+string(l.ch))
		} else if l.peekChar() == rune('>') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.ARROW, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.ASSIGN, string(l.ch))
		}
	case rune(';'):
		tok = l.newToken(token.SEMICOLON, string(l.ch))
	case rune('?'):
		if l.peekChar() == rune('.') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.QUESTION_DOT, string(ch)+string(l.ch))
		} else if l.peekChar() == rune('?') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.NULLISH, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.QUESTION, string(l.ch))
		}
	case rune('('):
		tok = l.newToken(token.LPAREN, string(l.ch))
	case rune(')'):
		tok = l.newToken(token.RPAREN, string(l.ch))
	case rune(','):
		tok = l.newToken(token.COMMA, string(l.ch))
	case rune('.'):
		if l.peekChar() == rune('.') && l.peekCharN(2) == rune('.') {
			l.readChar() // consume second '.'
			l.readChar() // consume third '.'
			tok = l.newToken(token.SPREAD, "...")
		} else {
			tok = l.newToken(token.PERIOD, string(l.ch))
		}
	case rune('+'):
		if l.peekChar() == rune('+') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.PLUS_PLUS, string(ch)+string(l.ch))
		} else if l.peekChar() == rune('=') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.PLUS_EQUALS, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.PLUS, string(l.ch))
		}
	case rune('%'):
		tok = l.newToken(token.MOD, string(l.ch))
	case rune('{'):
		tok = l.newToken(token.LBRACE, string(l.ch))
	case rune('}'):
		tok = l.newToken(token.RBRACE, string(l.ch))
	case rune('-'):
		if l.peekChar() == rune('-') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.MINUS_MINUS, string(ch)+string(l.ch))
		} else if l.peekChar() == rune('=') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.MINUS_EQUALS, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.MINUS, string(l.ch))
		}
	case rune('/'):
		if l.peekChar() == rune('=') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.SLASH_EQUALS, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.SLASH, string(l.ch))
		}
	case rune('*'):
		if l.peekChar() == rune('*') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.POW, string(ch)+string(l.ch))
		} else if l.peekChar() == rune('=') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.ASTERISK_EQUALS, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.ASTERISK, string(l.ch))
		}
	case rune('<'):
		if l.peekChar() == rune('<') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.LT_LT, string(ch)+string(l.ch))
		} else if l.peekChar() == rune('=') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.LT_EQUALS, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.LT, string(l.ch))
		}
	case rune('>'):
		if l.peekChar() == rune('>') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.GT_GT, string(ch)+string(l.ch))
		} else if l.peekChar() == rune('=') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.GT_EQUALS, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.GT, string(l.ch))
		}
	case rune('~'):
		return token.Token{}, fmt.Errorf("unexpected character: %q", l.ch)
	case rune('!'):
		if l.peekChar() == rune('=') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.NOT_EQ, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.BANG, string(l.ch))
		}
	case rune('\''):
		s, err := l.readString('\'')
		if err != nil {
			tok = l.newToken(token.STRING, s)
			l.readChar()
			l.prevToken = tok
			return tok, err
		}
		tok = l.newToken(token.STRING, s)
	case rune('"'):
		s, err := l.readString('"')
		if err != nil {
			tok = l.newToken(token.STRING, s)
			l.readChar()
			l.prevToken = tok
			return tok, err
		}
		tok = l.newToken(token.STRING, s)
	case rune('`'):
		s, err := l.readBacktick()
		if err != nil {
			tok = l.newToken(token.TEMPLATE, s)
			l.readChar()
			l.prevToken = tok
			return tok, err
		}
		tok = l.newToken(token.TEMPLATE, s)
	case rune('['):
		tok = l.newToken(token.LBRACKET, string(l.ch))
	case rune(']'):
		tok = l.newToken(token.RBRACKET, string(l.ch))
	case rune(':'):
		tok = l.newToken(token.COLON, string(l.ch))
	case rune('\r'):
		if l.peekChar() == rune('\n') {
			ch := l.ch
			l.readChar()
			tok = l.newToken(token.NEWLINE, string(ch)+string(l.ch))
		} else {
			tok = l.newToken(token.NEWLINE, string(l.ch))
		}
	case rune('\n'):
		tok = l.newToken(token.NEWLINE, string(l.ch))
	case rune(0):
		tok = l.newToken(token.EOF, "")
	default:
		var err error
		if isDigit(l.ch) {
			tok, err = l.readDecimal()
			if err != nil {
				return token.Token{}, err
			}
			l.readChar()
			l.prevToken = tok
			return tok, nil
		}
		ident, err := l.readIdentifier()
		if err != nil {
			return token.Token{}, err
		}
		if ident == "as" && l.prevToken.Type == token.PERIOD {
			tok = l.newToken(token.IDENT, ident)
		} else {
			tok = l.newToken(token.LookupIdentifier(ident), ident)
		}
		l.readChar()
		l.prevToken = tok
		return tok, nil
	}
	l.readChar()
	l.prevToken = tok
	return tok, nil
}

// Advance one character in the input string. Calling this when
// we are already at the end of the input has no effect.
func (l *Lexer) readChar() {
	// Return if we are already at the end of the input. Note that
	// when position == len(l.characters) the current character is
	// considered to be EOF, so that position is considered valid.
	if l.position > len(l.characters) {
		return
	}

	// The rune we were at before we advanced. This is used to
	// understand whether we advanced to a new line.
	prevCh := l.ch

	// Move forward. In the very first call, this moves us from
	// position -1 to position 0.
	l.position = l.nextPosition
	l.nextPosition++

	// Set the current character based on the current position
	if l.position < len(l.characters) {
		l.ch = l.characters[l.position]
	} else {
		l.ch = rune(0) // EOF
	}

	// Track three values, all zero-indexed:
	//  * line: the current line number
	//  * column: the current column number, within the line
	//  * lineStart: where the current line started in the input
	if prevCh == rune('\n') {
		l.column = 0
		l.line++
		l.lineStart = l.position
	} else {
		l.column++
	}
}

func (l *Lexer) newToken(typ token.Type, literal string) token.Token {
	return token.Token{
		Type:          typ,
		Literal:       literal,
		StartPosition: l.tokenStartPosition,
		EndPosition:   l.Position(),
	}
}

// Read a single identifier
func (l *Lexer) readIdentifier() (string, error) {
	var runes []rune
	if isIdentifier(l.ch) {
		runes = append(runes, l.ch)
	} else {
		return "", fmt.Errorf("invalid identifier: %s", string(l.ch))
	}
	for isIdentifier(l.peekChar()) {
		l.readChar()
		runes = append(runes, l.ch)
	}
	if l.peekChar() > unicode.MaxASCII {
		return "", fmt.Errorf("invalid identifier: %s", string(runes)+string(l.peekChar()))
	}
	return string(runes), nil
}

// Skip over any tabs or spaces. The parser is sensitive to newlines, so we
// don't skip those.
func (l *Lexer) skipTabsAndSpaces() {
	for isTabOrSpace(l.ch) {
		l.readChar()
	}
}

// Skip a comment until the end of the line
func (l *Lexer) skipComment() {
	for l.ch != '\n' && l.ch != rune(0) {
		l.readChar()
	}
	l.skipTabsAndSpaces()
}

// Consume all tokens until we've had the close of a multi-line comment
func (l *Lexer) skipMultiLineComment() {
	found := false
	for !found {
		// break at the end of our input.
		if l.ch == rune(0) {
			found = true
		}
		// otherwise keep going until we find "*/"
		if l.ch == '*' && l.peekChar() == '/' {
			found = true
			// Our current position is "*", so skip forward to consume the "/"
			l.readChar()
		}
		l.readChar()
	}
	l.skipTabsAndSpaces()
}

// Read a decimal, hex, octal, or binary number
func (l *Lexer) readNumber(onlyDecimal bool) (NumberType, string, error) {
	str := string(l.ch)
	// We usually just accept digits
	accept := "0123456789"
	numberType := NumberTypeDecimal
	if !onlyDecimal {
		if l.ch == '0' && l.peekChar() == 'x' {
			// 0x prefix => hexadecimal
			accept = "0x123456789abcdefABCDEF"
			numberType = NumberTypeHex
		} else if l.ch == '0' && l.peekChar() == 'b' {
			// 0b prefix => binary
			accept = "0b01"
			numberType = NumberTypeBinary
		} else if l.ch == '0' && l.peekChar() != '.' {
			// 0 prefix => octal
			accept = "01234567"
			numberType = NumberTypeOctal
		}
	}
	for strings.Contains(accept, string(l.peekChar())) {
		l.readChar()
		str += string(l.ch)
	}
	trailing := l.peekChar()
	if unicode.IsLetter(trailing) || unicode.IsNumber(trailing) {
		return NumberTypeInvalid, "", fmt.Errorf("invalid decimal literal: %s%c", str, trailing)
	}
	return numberType, str, nil
}

// Read an integer or floating point number
func (l *Lexer) readDecimal() (token.Token, error) {
	// Read an integer
	numberType, integer, err := l.readNumber(false)
	if err != nil {
		return token.Token{}, err
	}
	hasDot := l.peekChar() == rune('.')
	if !hasDot {
		return l.newToken(token.INT, integer), nil
	}
	if numberType != NumberTypeDecimal {
		return token.Token{}, fmt.Errorf("invalid decimal literal: %s%s", integer, ".")
	}
	// Read the "."
	l.readChar()
	if isDigit(l.peekChar()) {
		l.readChar()
		numberType, fraction, err := l.readNumber(true)
		if err != nil {
			return token.Token{}, err
		}
		if numberType != NumberTypeDecimal {
			return token.Token{}, fmt.Errorf("invalid decimal literal: %s.%s", integer, fraction)
		}
		return l.newToken(token.FLOAT, integer+"."+fraction), nil
	}
	// We reach this point with something like "42.foo"
	return token.Token{}, fmt.Errorf("invalid decimal literal: %s.%c", integer, l.peekChar())
}

func (l *Lexer) readString(end rune) (string, error) {
	var err error
	var sb strings.Builder
	for {
		peekChar := l.peekChar()
		if peekChar == rune(0) || peekChar == rune('\n') {
			err = fmt.Errorf("unterminated string literal")
			break
		}
		l.readChar()
		if l.ch == end {
			break
		}
		if l.ch != '\\' {
			sb.WriteRune(l.ch)
			continue
		}
		// Handle \n, \r, \t, \", etc
		l.readChar()
		switch l.ch {
		case rune('a'):
			sb.WriteRune('\a')
		case rune('b'):
			sb.WriteRune('\b')
		case rune('f'):
			sb.WriteRune('\f')
		case rune('n'):
			sb.WriteRune('\n')
		case rune('r'):
			sb.WriteRune('\r')
		case rune('t'):
			sb.WriteRune('\t')
		case rune('v'):
			sb.WriteRune('\v')
		case rune('\\'):
			sb.WriteRune('\\')
		case rune('e'):
			sb.WriteRune(0x1B)
		case end:
			// Keep the rune, it's an escaped quote.
			sb.WriteRune(l.ch)
		case rune('x'):
			num, err := l.readEscapeSequence(2, 16)
			if err != nil {
				return "", err
			}
			sb.WriteRune(rune(num))
		case rune('u'):
			num, err := l.readEscapeSequence(4, 16)
			if err != nil {
				return "", err
			}
			sb.WriteRune(rune(num))
		case rune('U'):
			num, err := l.readEscapeSequence(8, 16)
			if err != nil {
				return "", err
			}
			sb.WriteRune(rune(num))
		case rune('0'):
			// Octal notation is 3 chars, but first has already been read.
			num, err := l.readEscapeSequence(2, 8)
			if err != nil {
				return "", err
			}
			sb.WriteByte(byte(num))
		case rune('1'):
			num, err := l.readEscapeSequence(2, 8)
			if err != nil {
				return "", err
			}
			sb.WriteByte(byte(0o100 + num))
		case rune('2'):
			num, err := l.readEscapeSequence(2, 8)
			if err != nil {
				return "", err
			}
			sb.WriteByte(byte(0o200 + num))
		case rune('3'):
			num, err := l.readEscapeSequence(2, 8)
			if err != nil {
				return "", err
			}
			// This will never overflow byte, because max of 2 octals is 0o77,
			// and 255 in octal is 0o377.
			sb.WriteByte(byte(0o300 + num))
		default:
			return "", fmt.Errorf("invalid escape sequence: %q", l.ch)
		}
	}
	return sb.String(), err
}

func (l *Lexer) readEscapeSequence(count int, base int) (int, error) {
	const allChars = "0123456789abcdef"
	charset := allChars[:base]
	out := make([]byte, 0, count)
	for i := 0; i < count; i++ {
		l.readChar()
		if l.ch == rune(0) {
			return 0, fmt.Errorf("unterminated escape sequence")
		}
		if !strings.ContainsRune(charset, unicode.ToLower(l.ch)) {
			return 0, fmt.Errorf("illegal character %[1]U %[1]q in escape sequence", l.ch)
		}
		// Safe to convert to byte as we're working with ASCII-only charset
		out = append(out, byte(l.ch))
	}
	num, err := strconv.ParseInt(string(out), base, 32)
	if err != nil {
		return 0, fmt.Errorf("escape sequence is not a valid number (in base %d): %w", base, err)
	}
	return int(num), err
}

func (l *Lexer) readBacktick() (string, error) {
	var err error
	position := l.position + 1
	for {
		peekChar := l.peekChar()
		if peekChar == rune(0) {
			return "", fmt.Errorf("unterminated string literal")
		}
		l.readChar()
		if l.ch == '`' {
			break
		}
	}
	return string(l.characters[position:l.position]), err
}

func (l *Lexer) peekChar() rune {
	if l.nextPosition >= len(l.characters) {
		return rune(0)
	}
	return l.characters[l.nextPosition]
}

// peekCharN returns the character at the given offset from the current position.
// peekCharN(1) is equivalent to peekChar().
func (l *Lexer) peekCharN(n int) rune {
	pos := l.position + n
	if pos >= len(l.characters) {
		return rune(0)
	}
	return l.characters[pos]
}

// GetLineText returns the text of the line containing the given token.
func (l *Lexer) GetLineText(t token.Token) string {
	if len(l.characters) == 0 {
		return ""
	}

	// This is the position where the token begins
	tokenStart := t.StartPosition

	// Guard against programming errors. Raise panics to catch mistakes early.
	// Note that for the EOF token, the char offset will be equal to the number
	// of chars in the input, hence the ">" and not ">=".
	if tokenStart.Char < 0 || tokenStart.Char > len(l.characters)+1 {
		panic(fmt.Errorf("invalid token start position: %d token: %q input length: %d",
			tokenStart.Char, t.Type, len(l.characters)))
	}
	if tokenStart.Line < 0 {
		panic(fmt.Errorf("invalid token start line: %d", tokenStart.Line))
	}

	// Find the start of the line containing the given token
	start := tokenStart.Char
	if t.Type == token.EOF {
		start--
	}
	for start > 0 && l.characters[start-1] != rune('\n') {
		start--
	}
	// Find the end of that line
	end := tokenStart.Char
	if t.Type == token.EOF {
		end--
	}
	for end < len(l.characters) && l.characters[end] != rune('\n') {
		end++
	}
	// Return the line, excluding the newline character
	return string(l.characters[start:end])
}

func isIdentifier(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}

func isTabOrSpace(ch rune) bool {
	return ch == rune(' ') || ch == rune('\t')
}

func isDigit(ch rune) bool {
	return rune('0') <= ch && ch <= rune('9')
}
