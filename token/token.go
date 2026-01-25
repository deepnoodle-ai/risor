// Package token defines language keywords and tokens used when lexing source code.
package token

// Type describes the type of a token as a string.
type Type string

// Position points to a particular location in an input string.
type Position struct {
	Value     rune
	Char      int
	LineStart int
	Line      int
	Column    int
	File      string
}

// LineNumber returns the 1-indexed line number for this position in the input.
func (p Position) LineNumber() int {
	return p.Line + 1
}

// ColumnNumber returns the 1-indexed column number for this position in the input.
func (p Position) ColumnNumber() int {
	return p.Column + 1
}

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
	FOR             = "FOR"
	GT              = ">"
	GT_GT           = ">>"
	GT_EQUALS       = ">="
	GO              = "GO"
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
	PIPE_GT         = "|>"
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
	SEND            = "<-"
	SLASH           = "/"
	SLASH_EQUALS    = "/="
	STRING          = "STRING"
	STRUCT          = "STRUCT"
	SWITCH          = "switch"
	TEMPLATE        = "TEMPLATE"
	TRUE            = "TRUE"
	NEWLINE         = "EOL"
	IMPORT          = "IMPORT"
	BREAK           = "BREAK"
	CONTINUE        = "CONTINUE"
	IN              = "IN"
	RANGE           = "RANGE"
	FROM            = "FROM"
	AS              = "AS"
)

// Reserved keywords
var keywords = map[string]Type{
	"as":       AS,
	"break":    BREAK,
	"case":     CASE,
	"const":    CONST,
	"continue": CONTINUE,
	"default":  DEFAULT,
	"else":     ELSE,
	"false":    FALSE,
	"for":      FOR,
	"from":     FROM,
	"function": FUNCTION,
	"go":       GO, // Reserved keyword (concurrency removed but kept for future)
	"if":       IF,
	"import":   IMPORT,
	"in":       IN,
	"let":      LET,
	"nil":      NIL,
	"not":      NOT,
	"range":    RANGE,
	"return":   RETURN,
	"struct":   STRUCT,
	"switch":   SWITCH,
	"true":     TRUE,
}

// LookupIdentifier used to determinate whether identifier is keyword nor not
func LookupIdentifier(identifier string) Type {
	if tok, ok := keywords[identifier]; ok {
		return tok
	}
	return IDENT
}
