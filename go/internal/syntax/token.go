package syntax

import "fmt"

// TokenKind is the class of a token (01-language.md 2.3).
type TokenKind uint8

// The token kinds.
const (
	EOF TokenKind = iota
	Word
	TokNumber
	TokString // a one-line string, or a template with no interpolation (Form '`')

	// A template with interpolations is split at each interpolation:
	// TemplateHead is "`text${", TemplateMiddle is "}text${" and
	// TemplateTail is "}text`".
	TemplateHead
	TemplateMiddle
	TemplateTail

	// Illegal is a token that no rule accepts: the last row of 2.3, the block
	// token and the prefixed literal. Wherever it stands it is
	// unexpected-token at its error span.
	Illegal

	LParen    // (
	RParen    // )
	LBrack    // [
	RBrack    // ]
	LBrace    // {
	RBrace    // }
	OptLBrack // ?.[

	Comma    // ,
	Semi     // ;
	Colon    // :
	Dot      // .
	OptDot   // ?.
	Ellipsis // ...
	Arrow    // =>
	Assign   // =
	Question // ?
	Pipe     // |

	Bang     // !
	Minus    // -
	Plus     // +
	Star     // *
	Slash    // /
	Percent  // %
	Power    // **
	Eq       // ==
	Ne       // !=
	Lt       // <
	Le       // <=
	Gt       // >
	Ge       // >=
	AndAnd   // &&
	OrOr     // ||
	Coalesce // ??
)

var kindNames = map[TokenKind]string{
	EOF: "end of input", Word: "word", TokNumber: "number", TokString: "string",
	TemplateHead: "template", TemplateMiddle: "template", TemplateTail: "template",
	Illegal: "token", LParen: "(", RParen: ")", LBrack: "[", RBrack: "]",
	LBrace: "{", RBrace: "}", OptLBrack: "?.[", Comma: ",", Semi: ";",
	Colon: ":", Dot: ".", OptDot: "?.", Ellipsis: "...", Arrow: "=>",
	Assign: "=", Question: "?", Pipe: "|", Bang: "!", Minus: "-", Plus: "+",
	Star: "*", Slash: "/", Percent: "%", Power: "**", Eq: "==", Ne: "!=",
	Lt: "<", Le: "<=", Gt: ">", Ge: ">=", AndAnd: "&&", OrOr: "||",
	Coalesce: "??",
}

// String returns the token's text, or a word for a class of tokens.
func (k TokenKind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return fmt.Sprintf("TokenKind(%d)", k)
}

// Token is one token. Offsets count code points from the start of the text.
type Token struct {
	Kind  TokenKind
	Start int
	End   int
	// Text is the source text of a Word or an Illegal token.
	Text string
	// Num is the value of a Number.
	Num float64
	// Str is the value of a String, or the value of the text part of a
	// template piece.
	Str string
	// Form is the opening quote of a String: '"', '\'' or '`'.
	Form rune
	// Block is set for the tokens of a text block (F3 4). A text block is
	// never a map key, a pattern key, a match literal or type text. With
	// F3, Form is also 'r' for a raw string, 'T' for a text block and 'R'
	// for a raw text block.
	Block bool
	// BlockErr is a text block's text-block-open, -close or -indent code,
	// which the parser reports when it reaches the token (F3 6.2).
	BlockErr *Error
	// ErrStart and ErrEnd are the span a diagnostic gives when it names this
	// token. They equal Start and End except for the block token, the
	// prefixed literal and the pieces of a template.
	ErrStart int
	ErrEnd   int
}

// The 26 reserved words (01-language.md 2.4).
var reserved = map[string]bool{
	"true": true, "false": true, "null": true, "const": true, "if": true,
	"else": true, "in": true, "not": true, "try": true, "catch": true,
	"match": true,
	"as":    true, "is": true,
	"nil": true, "let": true, "var": true, "for": true, "while": true,
	"do": true, "break": true, "continue": true, "return": true,
	"function": true, "throw": true, "finally": true, "import": true,
}

// IsReserved reports whether w is one of the 26 reserved words.
func IsReserved(w string) bool { return reserved[w] }

// IsName reports whether w is a Name: a word that is not reserved and is not
// "_" (01-language.md 2.4).
func IsName(w string) bool {
	if w == "" || w == "_" || reserved[w] {
		return false
	}
	for i := 0; i < len(w); i++ {
		c := w[i]
		if !(isWordStart(rune(c)) || (i > 0 && isDigit(rune(c)))) {
			return false
		}
	}
	return true
}

func (t Token) isWord(w string) bool { return t.Kind == Word && t.Text == w }

func (t Token) describe() string {
	switch t.Kind {
	case Word, Illegal:
		return "`" + t.Text + "`"
	case TokNumber:
		return "number"
	case TokString:
		return "string"
	case TemplateHead, TemplateMiddle, TemplateTail:
		return "template"
	case EOF:
		return "end of input"
	}
	return "`" + t.Kind.String() + "`"
}
