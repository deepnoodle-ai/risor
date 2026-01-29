/**
 * Operator precedence levels for Pratt parsing.
 * Higher numbers = higher precedence (binds tighter).
 */

import { TokenKind } from "../token/token.js";

export const enum Precedence {
  LOWEST = 1,
  NULLISH = 2, // ??
  PIPE = 3, // |
  COND = 4, // && ||
  ASSIGN = 5, // =
  EQUALS = 6, // == !=
  LESSGREATER = 7, // > < >= <= in
  SUM = 8, // + -
  PRODUCT = 9, // * / % & ^ >> <<
  POWER = 10, // ** (right-associative)
  PREFIX = 11, // -X !X
  CALL = 12, // fn()
  INDEX = 13, // arr[i] obj.prop
  OPTCHAIN = 14, // ?.
  HIGHEST = 15,
}

/**
 * Get the precedence for a token type.
 */
export function getPrecedence(kind: TokenKind): Precedence {
  switch (kind) {
    case TokenKind.NULLISH:
      return Precedence.NULLISH;
    case TokenKind.ASSIGN:
      return Precedence.ASSIGN;
    case TokenKind.EQ:
    case TokenKind.NOT_EQ:
      return Precedence.EQUALS;
    case TokenKind.LT:
    case TokenKind.LT_EQUALS:
    case TokenKind.GT:
    case TokenKind.GT_EQUALS:
    case TokenKind.IN:
    case TokenKind.NOT:
      return Precedence.LESSGREATER;
    case TokenKind.PLUS:
    case TokenKind.PLUS_EQUALS:
    case TokenKind.MINUS:
    case TokenKind.MINUS_EQUALS:
      return Precedence.SUM;
    case TokenKind.SLASH:
    case TokenKind.SLASH_EQUALS:
    case TokenKind.ASTERISK:
    case TokenKind.ASTERISK_EQUALS:
    case TokenKind.MOD:
    case TokenKind.AMPERSAND:
    case TokenKind.CARET:
    case TokenKind.GT_GT:
    case TokenKind.LT_LT:
      return Precedence.PRODUCT;
    case TokenKind.POW:
      return Precedence.POWER;
    case TokenKind.AND:
    case TokenKind.OR:
      return Precedence.COND;
    case TokenKind.PIPE:
      return Precedence.PIPE;
    case TokenKind.LPAREN:
      return Precedence.CALL;
    case TokenKind.PERIOD:
    case TokenKind.LBRACKET:
      return Precedence.INDEX;
    case TokenKind.QUESTION_DOT:
      return Precedence.OPTCHAIN;
    default:
      return Precedence.LOWEST;
  }
}
