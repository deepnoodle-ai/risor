package parser

import "github.com/risor-io/risor/internal/token"

// Precedence order for operators (from lowest to highest)
// Note: Higher numbers = higher precedence (binds tighter)
const (
	_ int = iota
	LOWEST
	NULLISH     // ??
	PIPE        // |>
	COND        // OR or AND
	ASSIGN      // =
	EQUALS      // == or !=
	LESSGREATER // > or <
	SUM         // + or -
	PRODUCT     // * / %
	POWER       // ** (highest arithmetic precedence, right-associative)
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index], map[key]
	OPTCHAIN    // ?.
	HIGHEST
)

// Precedences for each token type
var precedences = map[token.Type]int{
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
	token.BITOR:           PRODUCT,
	token.CARET:           PRODUCT,
	token.GT_GT:           PRODUCT,
	token.LT_LT:           PRODUCT,
	token.POW:             POWER,
	token.MOD:             PRODUCT, // % has same precedence as * and /
	token.AND:             COND,
	token.OR:              COND,
	token.PIPE:            PIPE,
	token.LPAREN:          CALL,
	token.PERIOD:          INDEX,
	token.QUESTION_DOT:    OPTCHAIN,
	token.LBRACKET:        INDEX,
	token.IN:              LESSGREATER,
	token.NOT:             LESSGREATER,
}
