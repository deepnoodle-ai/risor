// Tokens (01 2.3) and reserved words (01 2.4).

import type { Span } from "./source.ts";

export type TokenKind =
  | "word"
  | "number"
  | "string" // a one-line string, "..." or '...'
  | "template" // a backtick literal, with or without interpolations
  | "punct" // brackets, separators and operators; `text` holds the token
  | "reject" // a token that no rule accepts (01 2.3 rule 4)
  | "interpEnd" // the `}` that closes an interpolation
  | "eof";

export interface Token {
  kind: TokenKind;
  start: number;
  end: number;
  /** A word's text, a punct's text, or a description of a reject token. */
  text: string;
  /** The value of a number token. */
  num?: number;
  /** The value of a one-line string. */
  str?: string;
  /** The quote of a one-line string. */
  quote?: '"' | "'";
  /** The parts of a template token. */
  parts?: TemplatePart[];
  /** The span a diagnostic names, where it is not the whole token. */
  report?: Span;
  /** White space or a comment stands directly before this token. */
  gap: boolean;
  /** The form of a literal of package F3: r", r', a text block or a raw text block (f03 5.2). */
  form?: string;
  /** A code of a text block that the parser reports when it reaches the token (f03 6.2). */
  blockError?: { code: string; start: number; end: number; template?: string };
  /** Package F12: a token that ends the text at a cut, or that the repair inserted there (f12 3.2, 3.3). */
  cut?: boolean;
  /** A prefixed literal: its word, and the quote or backtick after it (01 2.3 rule 9, f08 2.5). */
  prefix?: { word: string; quote: string; triple: boolean };
}

export type TemplatePart =
  | { kind: "text"; value: string }
  | { kind: "interp"; open: Span; tokens: Token[] };

/** The 26 reserved words of 01 2.4. */
export const RESERVED: ReadonlySet<string> = new Set([
  // words of the grammar
  "true", "false", "null", "const", "if", "else", "in", "not", "try", "catch", "match",
  // reserved for a follow-on form
  "as", "is",
  // reserved for a later version
  "nil", "let", "var", "for", "while", "do", "break", "continue", "return",
  "function", "throw", "finally", "import",
]);

export function isReserved(word: string): boolean {
  return RESERVED.has(word);
}

/** A name: a word that is not reserved and is not `_` (01 2.4 rule 4). */
export function isName(word: string): boolean {
  return word !== "_" && !RESERVED.has(word);
}

/** Tokens that no rule accepts, read as one token each (01 2.3). */
export const REJECTED_PUNCT: ReadonlySet<string> = new Set([
  "===", "!==", "+=", "-=", "*=", "/=", "%=", "**=", "??=", "||=", "&&=", "++", "|>",
]);

/** Every punctuation token, longest first within each first character. */
export const PUNCT: readonly string[] = [
  "===", "!==", "**=", "??=", "||=", "&&=", "...", "?.[",
  "==", "!=", "<=", ">=", "&&", "||", "??", "**", "=>", "?.",
  "+=", "-=", "*=", "/=", "%=", "++", "|>",
  "(", ")", "[", "]", "{", "}", ",", ";", ":", ".", "?", "|",
  "!", "-", "+", "*", "/", "%", "<", ">", "=",
];

/** Package F12: the text of the hole, a token that no text can spell (f12 3.4). */
export const HOLE = "\u2038";
