// The tokens of a text for colouring it in an editor. This is a call of the
// TypeScript binding, not of the specification: an editor in the browser
// colours a program with the lexer the suite tests, instead of a second
// grammar that must be kept in step by hand.
//
// The call never throws. A text that does not lex is coloured up to the
// place of the lexical error; the rest of that line is `invalid`, and lexing
// starts again on the next line.

import { ParseError } from "../syntax/diagnostic.ts";
import { lex } from "../syntax/lexer.ts";
import { Source, fromCodePoints } from "../syntax/source.ts";
import { isReserved } from "../syntax/token.ts";
import type { Token } from "../syntax/token.ts";

/** The class of a token, for its colour. */
export type TokenClass =
  | "keyword" // a reserved word other than the constants
  | "constant" // true, false, null
  | "name" // a word that is not reserved
  | "function" // a word directly before `(`
  | "property" // a word directly after `.` or `?.`
  | "number"
  | "string" // a one-line string, a raw string, a text block
  | "template" // the text of a backtick literal, outside its interpolations
  | "interpolation" // the `${` and `}` around an interpolation
  | "operator"
  | "punctuation" // brackets and separators
  | "comment"
  | "invalid"; // a token no rule accepts, or the rest of a line that does not lex

/** One coloured range. Offsets count code points of the text as given, from 0. */
export interface ColourToken {
  class: TokenClass;
  start: number;
  end: number;
}

const BRACKETS = new Set(["(", ")", "[", "]", "{", "}", ",", ";", ":", "."]);

/**
 * The coloured ranges of a text, in order, never overlapping. White space is
 * in no range. Every other code point is in exactly one range.
 */
export function tokens(text: string): ColourToken[] {
  const all = new Source(text, { keepBom: true });
  const cps = all.cps;
  const out: ColourToken[] = [];
  let lineStart = 0;
  while (lineStart <= cps.length) {
    const { tokens: toks, error } = lexFrom(cps, lineStart);
    const end = error ?? cps.length;
    const flat: ColourToken[] = [];
    flatten(toks, lineStart, flat);
    // Keep only the ranges before the error, and fill the gaps with comments.
    let at = lineStart;
    for (const t of flat) {
      if (t.start >= end) break;
      comments(cps, at, t.start, out);
      out.push(t.end > end ? { ...t, end } : t);
      at = Math.min(t.end, end);
    }
    comments(cps, at, end, out);
    if (error === null) break;
    let eol = error;
    while (eol < cps.length && cps[eol] !== 0x0a) eol++;
    if (eol > error) out.push({ class: "invalid", start: error, end: eol });
    lineStart = eol + 1;
  }
  return out;
}

/** Lex the text from `start`; on a lexical error, the offset of the error. */
function lexFrom(cps: Uint32Array, start: number): { tokens: Token[]; error: number | null } {
  let text = fromCodePoints(cps, start);
  for (;;) {
    try {
      return { tokens: lex(new Source(text, { keepBom: true })), error: null };
    } catch (e) {
      if (!(e instanceof ParseError)) throw e;
      const at = e.diagnostic.span?.start ?? 0;
      // Lex the part before the error, so the colours reach it. The part is
      // shorter each time, so this ends.
      const prefix = cutAt(text, at);
      if (prefix.length >= text.length) return { tokens: [], error: start };
      try {
        return { tokens: lex(new Source(prefix, { keepBom: true })), error: start + at };
      } catch (e2) {
        if (!(e2 instanceof ParseError)) throw e2;
        text = prefix;
      }
    }
  }
}

/** The text before code point `n`. */
function cutAt(text: string, n: number): string {
  let i = 0;
  let units = 0;
  for (const ch of text) {
    if (i === n) break;
    units += ch.length;
    i++;
  }
  return text.slice(0, units);
}

/** Flatten tokens, those inside interpolations included, into coloured ranges. */
function flatten(toks: Token[], base: number, out: ColourToken[]): void {
  for (let i = 0; i < toks.length; i++) {
    const t = toks[i];
    const start = base + t.start;
    const end = base + t.end;
    switch (t.kind) {
      case "eof":
        break;
      case "word": {
        const prev = toks[i - 1];
        const next = toks[i + 1];
        let cls: TokenClass;
        if (t.prefix) cls = "string";
        else if (t.text === "true" || t.text === "false" || t.text === "null") cls = "constant";
        else if (isReserved(t.text)) cls = "keyword";
        else if (prev?.kind === "punct" && (prev.text === "." || prev.text === "?.")) cls = "property";
        else if (next?.kind === "punct" && next.text === "(") cls = "function";
        else cls = "name";
        out.push({ class: cls, start, end });
        break;
      }
      case "number":
        out.push({ class: "number", start, end });
        break;
      case "string":
        out.push({ class: "string", start, end });
        break;
      case "template":
        template(t, base, out);
        break;
      case "punct":
        out.push({ class: BRACKETS.has(t.text) ? "punctuation" : "operator", start, end });
        break;
      case "interpEnd":
        out.push({ class: "interpolation", start, end });
        break;
      case "reject":
        out.push({ class: "invalid", start, end });
        break;
    }
  }
}

/** A template: its text, and each interpolation's `${`, tokens and `}`. */
function template(t: Token, base: number, out: ColourToken[]): void {
  let at = base + t.start;
  for (const p of t.parts ?? []) {
    if (p.kind !== "interp") continue;
    const open = base + p.open.start;
    if (open > at) out.push({ class: "template", start: at, end: open });
    out.push({ class: "interpolation", start: open, end: base + p.open.end });
    flatten(p.tokens, base, out);
    const last = p.tokens[p.tokens.length - 1];
    at = last && last.kind === "interpEnd" ? base + last.end : base + p.open.end;
  }
  const end = base + t.end;
  if (end > at) out.push({ class: "template", start: at, end });
}

/** The comments in a gap between tokens: each run of code points that is not white space. */
function comments(cps: Uint32Array, from: number, to: number, out: ColourToken[]): void {
  let i = from;
  while (i < to) {
    while (i < to && isSpace(cps[i])) i++;
    if (i >= to) break;
    const s = i;
    while (i < to && !isSpace(cps[i])) i++;
    // A comment runs on over white space to its last code point in the gap
    // when it is a block comment; join runs on one line.
    const last = out[out.length - 1];
    if (last && last.class === "comment" && !newlineBetween(cps, last.end, s)) last.end = i;
    else out.push({ class: "comment", start: s, end: i });
  }
}

function newlineBetween(cps: Uint32Array, a: number, b: number): boolean {
  for (let i = a; i < b; i++) if (cps[i] === 0x0a) return true;
  return false;
}

function isSpace(c: number): boolean {
  return c === 0x20 || c === 0x09 || c === 0x0a || c === 0x0d;
}
