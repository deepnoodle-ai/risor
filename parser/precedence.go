package parser

import "github.com/risor-io/risor/token"

// Precedence order for operators
const (
	_ int = iota
	LOWEST
	NULLISH     // ??
	PIPE        // |
	COND        // OR or AND
	ASSIGN      // =
	TERNARY     // ? :
	EQUALS      // == or !=
	LESSGREATER // > or <
	SUM         // + or -
	PRODUCT     // * or /
	POWER       // **
	MOD         // %
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index], map[key]
	OPTCHAIN    // ?.
	HIGHEST
)

// Precedences for each token type
var precedences = map[token.Type]int{
	token.QUESTION:        TERNARY,
	token.NULLISH:         NULLISH,
	token.ASSIGN:          ASSIGN,
	token.EQ:              EQUALS,
	token.NOT_EQ:          EQUALS,
	token.LT:              LESSGREATER,
	token.LT_EQUALS:       LESSGREATER,
	token.GT:              LESSGREATER,
	token.GT_EQUALS:       LESSGREATER,
	token.PLUS:            SUM,
	token.PLUS_EQUALS:     SUM,
	token.MINUS:           SUM,
	token.MINUS_EQUALS:    SUM,
	token.SLASH:           PRODUCT,
	token.SLASH_EQUALS:    PRODUCT,
	token.ASTERISK:        PRODUCT,
	token.ASTERISK_EQUALS: PRODUCT,
	token.AMPERSAND:       PRODUCT,
	token.GT_GT:           PRODUCT,
	token.LT_LT:           PRODUCT,
	token.POW:             POWER,
	token.MOD:             MOD,
	token.AND:             COND,
	token.OR:              COND,
	token.PIPE:            PIPE,
	token.PIPE_GT:         PIPE,
	token.LPAREN:          CALL,
	token.PERIOD:          INDEX,
	token.QUESTION_DOT:    OPTCHAIN,
	token.LBRACKET:        INDEX,
	token.IN:              PREFIX,
	token.NOT:             PREFIX,
	token.RANGE:           PREFIX,
	token.SEND:            CALL,
}
