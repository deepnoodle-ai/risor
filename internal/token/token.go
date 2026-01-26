// Package token defines language keywords and tokens used when lexing source code.
package token

// Type describes the type of a token as a string.
type Type string

// Position points to a particular location in an input string.
type Position struct {
	Char      int    // byte offset within the file
	LineStart int    // byte offset of the start of the current line
	Line      int    // 0-indexed line number
	Column    int    // 0-indexed column number
	File      string // filename
}

// LineNumber returns the 1-indexed line number for this position in the input.
func (p Position) LineNumber() int {
	return p.Line + 1
}

// ColumnNumber returns the 1-indexed column number for this position in the input.
func (p Position) ColumnNumber() int {
	return p.Column + 1
}

// Advance returns a new Position advanced by n bytes.
// Used for computing End positions from a start position.
// Note: This assumes the advance does not cross line boundaries.
func (p Position) Advance(n int) Position {
	return Position{
		Char:      p.Char + n,
		LineStart: p.LineStart,
		Line:      p.Line,
		Column:    p.Column + n,
		File:      p.File,
	}
}

// IsValid returns true if this position has been set.
func (p Position) IsValid() bool {
	return p.File != "" || p.Line > 0 || p.Column > 0 || p.Char > 0
}

// NoPos is the zero value Position, representing an invalid/unset position.
var NoPos = Position{}

// Token represents one token lexed from the input source code.
type Token struct {
	Type          Type
	Literal       string
	StartPosition Position
	EndPosition   Position
}

// Token types
const (
	AND             = "&&"
	ARROW           = "=>"
	ASSIGN          = "="
	ASTERISK        = "*"
	ASTERISK_EQUALS = "*="
	BACKTICK        = "`"
	BANG            = "!"
	CASE            = "case"
	COLON           = ":"
	COMMA           = ","
	CONST           = "CONST"
	DEFAULT         = "DEFAULT"
	FUNCTION        = "FUNCTION"
	ELSE            = "ELSE"
	EOF             = "EOF"
	EQ              = "=="
	FALSE           = "FALSE"
	FLOAT           = "FLOAT"
	GT              = ">"
	GT_GT           = ">>"
	GT_EQUALS       = ">="
	IDENT           = "IDENT"
	IF              = "IF"
	ILLEGAL         = "ILLEGAL"
	INT             = "INT"
	LBRACE          = "{"
	LBRACKET        = "["
	LPAREN          = "("
	LT              = "<"
	LT_LT           = "<<"
	LT_EQUALS       = "<="
	LET             = "LET"
	MINUS           = "-"
	MINUS_EQUALS    = "-="
	MINUS_MINUS     = "--"
	MOD             = "%"
	NOT_EQ          = "!="
	NIL             = "nil"
	NOT             = "NOT"
	NULLISH         = "??"
	PIPE            = "|"
	OR              = "||"
	PERIOD          = "."
	PLUS            = "+"
	AMPERSAND       = "&"
	PLUS_EQUALS     = "+="
	PLUS_PLUS       = "++"
	POW             = "**"
	QUESTION        = "?"
	QUESTION_DOT    = "?."
	RBRACE          = "}"
	RBRACKET        = "]"
	RETURN          = "RETURN"
	RPAREN          = ")"
	SEMICOLON       = ";"
	SPREAD          = "..."
	SLASH           = "/"
	SLASH_EQUALS    = "/="
	STRING          = "STRING"
	STRUCT          = "STRUCT"
	SWITCH          = "switch"
	TEMPLATE        = "TEMPLATE"
	TRUE            = "TRUE"
	NEWLINE         = "EOL"
	IN              = "IN"
	TRY             = "TRY"
	CATCH           = "CATCH"
	FINALLY         = "FINALLY"
	THROW           = "THROW"
)

// Reserved keywords
var keywords = map[string]Type{
	"case":     CASE,
	"const":    CONST,
	"default":  DEFAULT,
	"else":     ELSE,
	"false":    FALSE,
	"function": FUNCTION,
	"if":       IF,
	"in":       IN,
	"let":      LET,
	"nil":      NIL,
	"not":      NOT,
	"return":   RETURN,
	"struct":   STRUCT,
	"switch":   SWITCH,
	"throw":    THROW,
	"true":     TRUE,
	"try":      TRY,
	"catch":    CATCH,
	"finally":  FINALLY,
}

// LookupIdentifier used to determinate whether identifier is keyword nor not
func LookupIdentifier(identifier string) Type {
	if tok, ok := keywords[identifier]; ok {
		return tok
	}
	return IDENT
}
