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
	AND             Type = "&&"
	ARROW           Type = "=>"
	ASSIGN          Type = "="
	ASTERISK        Type = "*"
	ASTERISK_EQUALS Type = "*="
	BACKTICK        Type = "`"
	CARET           Type = "^"
	BANG            Type = "!"
	CASE            Type = "case"
	COLON           Type = ":"
	COMMA           Type = ","
	CONST           Type = "CONST"
	DEFAULT         Type = "DEFAULT"
	FUNCTION        Type = "FUNCTION"
	ELSE            Type = "ELSE"
	EOF             Type = "EOF"
	EQ              Type = "=="
	FALSE           Type = "FALSE"
	FLOAT           Type = "FLOAT"
	GT              Type = ">"
	GT_GT           Type = ">>"
	GT_EQUALS       Type = ">="
	IDENT           Type = "IDENT"
	IF              Type = "IF"
	ILLEGAL         Type = "ILLEGAL"
	INT             Type = "INT"
	LBRACE          Type = "{"
	LBRACKET        Type = "["
	LPAREN          Type = "("
	LT              Type = "<"
	LT_LT           Type = "<<"
	LT_EQUALS       Type = "<="
	LET             Type = "LET"
	MINUS           Type = "-"
	MINUS_EQUALS    Type = "-="
	MINUS_MINUS     Type = "--"
	MOD             Type = "%"
	NOT_EQ          Type = "!="
	NIL             Type = "nil"
	NOT             Type = "NOT"
	NULLISH         Type = "??"
	PIPE            Type = "|>"
	BITOR           Type = "|"
	OR              Type = "||"
	PERIOD          Type = "."
	PLUS            Type = "+"
	AMPERSAND       Type = "&"
	PLUS_EQUALS     Type = "+="
	PLUS_PLUS       Type = "++"
	POW             Type = "**"
	QUESTION        Type = "?"
	QUESTION_DOT    Type = "?."
	RBRACE          Type = "}"
	RBRACKET        Type = "]"
	RETURN          Type = "RETURN"
	RPAREN          Type = ")"
	SEMICOLON       Type = ";"
	SPREAD          Type = "..."
	SLASH           Type = "/"
	SLASH_EQUALS    Type = "/="
	STRING          Type = "STRING"
	STRUCT          Type = "STRUCT"
	MATCH           Type = "match"
	SWITCH          Type = "switch"
	TEMPLATE        Type = "TEMPLATE"
	TRUE            Type = "TRUE"
	NEWLINE         Type = "EOL"
	IN              Type = "IN"
	TRY             Type = "TRY"
	CATCH           Type = "CATCH"
	FINALLY         Type = "FINALLY"
	THROW           Type = "THROW"
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
	"match":    MATCH,
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
