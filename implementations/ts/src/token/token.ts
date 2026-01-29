/**
 * Token types for the Risor lexer.
 */
export const enum TokenKind {
  // Literals
  INT = "INT",
  FLOAT = "FLOAT",
  STRING = "STRING",
  TEMPLATE = "TEMPLATE",
  IDENT = "IDENT",

  // Operators
  PLUS = "+",
  MINUS = "-",
  ASTERISK = "*",
  SLASH = "/",
  MOD = "%",
  POW = "**",

  // Comparison
  EQ = "==",
  NOT_EQ = "!=",
  LT = "<",
  GT = ">",
  LT_EQUALS = "<=",
  GT_EQUALS = ">=",

  // Logical
  AND = "&&",
  OR = "||",
  BANG = "!",

  // Bitwise
  AMPERSAND = "&",
  PIPE = "|",
  CARET = "^",
  LT_LT = "<<",
  GT_GT = ">>",

  // Assignment
  ASSIGN = "=",
  PLUS_EQUALS = "+=",
  MINUS_EQUALS = "-=",
  ASTERISK_EQUALS = "*=",
  SLASH_EQUALS = "/=",

  // Increment/Decrement
  PLUS_PLUS = "++",
  MINUS_MINUS = "--",

  // Punctuation
  LPAREN = "(",
  RPAREN = ")",
  LBRACKET = "[",
  RBRACKET = "]",
  LBRACE = "{",
  RBRACE = "}",
  COMMA = ",",
  SEMICOLON = ";",
  COLON = ":",
  PERIOD = ".",
  SPREAD = "...",
  ARROW = "=>",
  QUESTION = "?",
  QUESTION_DOT = "?.",
  NULLISH = "??",
  BACKTICK = "`",

  // Keywords
  LET = "let",
  CONST = "const",
  FUNCTION = "function",
  RETURN = "return",
  IF = "if",
  ELSE = "else",
  SWITCH = "switch",
  CASE = "case",
  DEFAULT = "default",
  MATCH = "match",
  TRUE = "true",
  FALSE = "false",
  NIL = "nil",
  NOT = "not",
  IN = "in",
  STRUCT = "struct",
  TRY = "try",
  CATCH = "catch",
  FINALLY = "finally",
  THROW = "throw",

  // Special
  NEWLINE = "NEWLINE",
  EOF = "EOF",
  ILLEGAL = "ILLEGAL",
}

/**
 * Keywords map for identifier lookup.
 */
const keywords: Map<string, TokenKind> = new Map([
  ["let", TokenKind.LET],
  ["const", TokenKind.CONST],
  ["function", TokenKind.FUNCTION],
  ["return", TokenKind.RETURN],
  ["if", TokenKind.IF],
  ["else", TokenKind.ELSE],
  ["switch", TokenKind.SWITCH],
  ["case", TokenKind.CASE],
  ["default", TokenKind.DEFAULT],
  ["match", TokenKind.MATCH],
  ["true", TokenKind.TRUE],
  ["false", TokenKind.FALSE],
  ["nil", TokenKind.NIL],
  ["not", TokenKind.NOT],
  ["in", TokenKind.IN],
  ["struct", TokenKind.STRUCT],
  ["try", TokenKind.TRY],
  ["catch", TokenKind.CATCH],
  ["finally", TokenKind.FINALLY],
  ["throw", TokenKind.THROW],
]);

/**
 * Look up an identifier to see if it's a keyword.
 */
export function lookupIdentifier(ident: string): TokenKind {
  return keywords.get(ident) ?? TokenKind.IDENT;
}

/**
 * Position in source code.
 */
export interface Position {
  /** Byte offset within the file */
  char: number;
  /** Byte offset of the start of the current line */
  lineStart: number;
  /** 0-indexed line number */
  line: number;
  /** 0-indexed column number */
  column: number;
  /** Filename */
  file: string;
}

/**
 * Create a new Position.
 */
export function newPosition(
  char: number,
  lineStart: number,
  line: number,
  column: number,
  file: string
): Position {
  return { char, lineStart, line, column, file };
}

/**
 * The zero value Position, representing an invalid/unset position.
 */
export const NoPos: Position = {
  char: 0,
  lineStart: 0,
  line: 0,
  column: 0,
  file: "",
};

/**
 * Check if a position is valid.
 */
export function isValidPosition(p: Position): boolean {
  return p.file !== "" || p.line > 0 || p.column > 0 || p.char > 0;
}

/**
 * Returns the 1-indexed line number.
 */
export function lineNumber(p: Position): number {
  return p.line + 1;
}

/**
 * Returns the 1-indexed column number.
 */
export function columnNumber(p: Position): number {
  return p.column + 1;
}

/**
 * Advance a position by n characters.
 */
export function advancePosition(p: Position, n: number): Position {
  return {
    char: p.char + n,
    lineStart: p.lineStart,
    line: p.line,
    column: p.column + n,
    file: p.file,
  };
}

/**
 * A token produced by the lexer.
 */
export interface Token {
  kind: TokenKind;
  literal: string;
  start: Position;
  end: Position;
}

/**
 * Create a new Token.
 */
export function newToken(
  kind: TokenKind,
  literal: string,
  start: Position,
  end: Position
): Token {
  return { kind, literal, start, end };
}
