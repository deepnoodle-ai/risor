// The lexer (01 section 2).
//
// The whole text is read into tokens before it is parsed (01 2.11). The first
// lexical diagnostic is thrown as a ParseError at the place the lexer meets
// it. Templates nest, and their interpolations hold tokens, so the lexer keeps
// an explicit stack of frames rather than recursing: a template nested
// 100,000 deep must reach the parser's nesting limit, not the host's stack.

import { ParseError } from "./diagnostic.ts";
import type { Source, Span } from "./source.ts";
import { fromCodePoints } from "./source.ts";
import { PUNCT, REJECTED_PUNCT, isReserved } from "./token.ts";
import type { TemplatePart, Token } from "./token.ts";
import { MAX_WHOLE } from "../value/number.ts";

const LF = 0x0a;
const CR = 0x0d;
const TAB = 0x09;
const SPACE = 0x20;
const DQ = 0x22; // "
const SQ = 0x27; // '
const BT = 0x60; // `
const BS = 0x5c; // \
const DOLLAR = 0x24;
const LBRACE = 0x7b;
const RBRACE = 0x7d;

function isDigit(c: number): boolean {
  return c >= 0x30 && c <= 0x39;
}
function isHex(c: number): boolean {
  return isDigit(c) || (c >= 0x41 && c <= 0x46) || (c >= 0x61 && c <= 0x66);
}
function isLetter(c: number): boolean {
  return (c >= 0x41 && c <= 0x5a) || (c >= 0x61 && c <= 0x7a);
}
function isWordStart(c: number): boolean {
  return isLetter(c) || c === 0x5f;
}
function isWordPart(c: number): boolean {
  return isWordStart(c) || isDigit(c);
}
function isSpace(c: number): boolean {
  return c === SPACE || c === TAB || c === LF || c === CR;
}
function hexValue(c: number): number {
  if (isDigit(c)) return c - 0x30;
  if (c >= 0x61) return c - 0x61 + 10;
  return c - 0x41 + 10;
}

type Frame =
  | { t: "top"; tokens: Token[] }
  | { t: "interp"; tokens: Token[]; depth: number }
  | { t: "tmpl"; token: Token; parts: TemplatePart[]; text: number[] };

/** Read the whole source into tokens. Throws ParseError on the first lexical diagnostic. */
export function lex(src: Source): Token[] {
  return new Lexer(src).run();
}

class Lexer {
  private readonly cps: Uint32Array;
  private pos = 0;
  private gap = false;
  private readonly stack: Frame[] = [];

  constructor(src: Source) {
    this.cps = src.cps;
  }

  private at(i: number): number {
    return i < this.cps.length ? this.cps[i] : -1;
  }

  private fail(code: string, start: number, end: number, message: string): never {
    throw new ParseError(code, { start, end }, message);
  }

  run(): Token[] {
    const top: Frame = { t: "top", tokens: [] };
    this.stack.push(top);
    for (;;) {
      const f = this.stack[this.stack.length - 1];
      if (f.t === "tmpl") {
        this.templateText(f);
        continue;
      }
      this.skipTrivia();
      if (this.pos >= this.cps.length) {
        if (f.t === "top") {
          f.tokens.push(this.tok("eof", this.pos, this.pos, ""));
          return f.tokens;
        }
        // End of input inside an interpolation: the innermost open template.
        this.unterminatedTemplate();
      }
      const c = this.cps[this.pos];
      if (c === BT) {
        const token = this.tok("template", this.pos, this.pos, "`");
        token.parts = [];
        this.pos++;
        this.stack.push({ t: "tmpl", token, parts: token.parts, text: [] });
        continue;
      }
      if (f.t === "interp") {
        if (c === LBRACE) f.depth++;
        else if (c === RBRACE) {
          if (f.depth === 0) {
            f.tokens.push(this.tok("interpEnd", this.pos, this.pos + 1, "}"));
            this.pos++;
            this.stack.pop();
            continue;
          }
          f.depth--;
        }
      }
      f.tokens.push(this.next());
    }
  }

  private unterminatedTemplate(): never {
    for (let i = this.stack.length - 1; i >= 0; i--) {
      const f = this.stack[i];
      if (f.t === "tmpl") {
        this.fail("unterminated-string", f.token.start, this.cps.length, "the template has no closing `");
      }
    }
    throw new Error("unreachable");
  }

  private tok(kind: Token["kind"], start: number, end: number, text: string): Token {
    const t: Token = { kind, start, end, text, gap: this.gap };
    this.gap = false;
    return t;
  }

  private parentTokens(): Token[] {
    const f = this.stack[this.stack.length - 1];
    if (f.t === "tmpl") throw new Error("unreachable");
    return f.tokens;
  }

  // White space and comments (01 2.2).
  private skipTrivia(): void {
    const cps = this.cps;
    for (;;) {
      const c = this.at(this.pos);
      if (c >= 0 && isSpace(c)) {
        this.pos++;
        this.gap = true;
        continue;
      }
      if (c === 0x2f && this.at(this.pos + 1) === 0x2f) {
        this.pos += 2;
        while (this.pos < cps.length && cps[this.pos] !== LF && cps[this.pos] !== CR) this.pos++;
        this.gap = true;
        continue;
      }
      if (c === 0x2f && this.at(this.pos + 1) === 0x2a) {
        const start = this.pos;
        this.pos += 2;
        for (;;) {
          if (this.pos >= cps.length) {
            this.fail("unterminated-comment", start, start + 2, "the comment has no closing */");
          }
          if (cps[this.pos] === 0x2a && this.at(this.pos + 1) === 0x2f) {
            this.pos += 2;
            break;
          }
          this.pos++;
        }
        this.gap = true;
        continue;
      }
      return;
    }
  }

  // One token outside a template's text. The caller handles ` and the
  // braces of an interpolation's depth.
  private next(): Token {
    const cps = this.cps;
    const start = this.pos;
    const c = cps[start];

    // The block token (01 2.3 rule 7).
    if ((c === DQ || c === SQ) && this.at(start + 1) === c && this.at(start + 2) === c) {
      let i = start + 3;
      let end = cps.length;
      for (; i + 2 < cps.length + 0; i++) {
        if (cps[i] === c && cps[i + 1] === c && cps[i + 2] === c) {
          end = i + 3;
          break;
        }
      }
      this.pos = end;
      const t = this.tok("reject", start, end, "a text block");
      t.report = { start, end: start + 3 };
      return t;
    }
    if (c === DQ || c === SQ) return this.oneLineString();
    if (isDigit(c) || (c === 0x2e && isDigit(this.at(start + 1)))) return this.number();
    if (isWordStart(c)) return this.word();

    for (const p of PUNCT) {
      if (this.matches(start, p)) {
        this.pos = start + p.length;
        return this.tok(REJECTED_PUNCT.has(p) ? "reject" : "punct", start, this.pos, p);
      }
    }
    this.fail("unexpected-character", start, start + 1, `unexpected character ${describeCp(c)}`);
  }

  private matches(at: number, p: string): boolean {
    for (let k = 0; k < p.length; k++) {
      if (this.at(at + k) !== p.charCodeAt(k)) return false;
    }
    return true;
  }

  private word(): Token {
    const cps = this.cps;
    const start = this.pos;
    let i = start + 1;
    while (i < cps.length && isWordPart(cps[i])) i++;
    const text = fromCodePoints(cps, start, i);
    const q = this.at(i);
    // The prefixed literal (01 2.3 rule 9, 2.9 rule 1).
    if ((q === DQ || q === SQ || q === BT) && !isReserved(text)) {
      let j = i + 1;
      let end = cps.length;
      for (; j < cps.length; j++) {
        const d = cps[j];
        if (d === q) {
          end = j + 1;
          break;
        }
        if (q !== BT && (d === LF || d === CR)) {
          end = j;
          break;
        }
      }
      this.pos = end;
      const t = this.tok("reject", start, end, "a prefixed literal");
      t.report = { start, end: i };
      return t;
    }
    this.pos = i;
    return this.tok("word", start, i, text);
  }

  // Number tokens (01 2.5).
  private number(): Token {
    const cps = this.cps;
    const start = this.pos;
    const hex = cps[start] === 0x30 && (this.at(start + 1) === 0x78 || this.at(start + 1) === 0x58);
    let i = start;
    for (;;) {
      const d = this.at(i);
      if (d < 0) break;
      if (isLetter(d) || isDigit(d) || d === 0x5f) {
        i++;
        continue;
      }
      if ((d === 0x2b || d === 0x2d) && !hex && i > start && (cps[i - 1] === 0x65 || cps[i - 1] === 0x45)) {
        i++;
        continue;
      }
      if (d === 0x2e) {
        const n = this.at(i + 1);
        if (n < 0 || isDigit(n) || !(isLetter(n) || n === 0x5f)) {
          i++;
          continue;
        }
      }
      break;
    }
    this.pos = i;
    const text = fromCodePoints(cps, start, i);
    const value = numberValue(text);
    if (value === "bad") this.fail("bad-number", start, i, `${text} is not a number literal`);
    if (value === "range") this.fail("literal-range", start, i, `${text} is out of range`);
    const t = this.tok("number", start, i, text);
    t.num = value;
    return t;
  }

  // One-line strings (01 2.6).
  private oneLineString(): Token {
    const cps = this.cps;
    const start = this.pos;
    const quote = cps[start];
    const out: number[] = [];
    let i = start + 1;
    for (;;) {
      if (i >= cps.length) {
        this.fail("unterminated-string", start, cps.length, "the string has no closing quote");
      }
      const c = cps[i];
      if (c === quote) {
        i++;
        break;
      }
      if (c === LF || c === CR) {
        this.fail("unterminated-string", start, i, "the string has no closing quote on its line");
      }
      if (c === BS) {
        const n = this.at(i + 1);
        if (n < 0) this.fail("unterminated-string", start, cps.length, "the string has no closing quote");
        if (n === LF || n === CR) {
          this.fail("unterminated-string", start, i + 1, "the string has no closing quote on its line");
        }
        i = this.escape(i, false, out);
        continue;
      }
      out.push(c);
      i++;
    }
    this.pos = i;
    const t = this.tok("string", start, i, quote === DQ ? '"' : "'");
    t.str = fromCodePoints(out);
    t.quote = quote === DQ ? '"' : "'";
    return t;
  }

  /**
   * Read the escape whose `\` is at `i`, push its code point, and return the
   * offset after it (01 2.7).
   */
  private escape(i: number, inTemplate: boolean, out: number[]): number {
    const cps = this.cps;
    const n = this.at(i + 1);
    const simple: Record<number, number> = {
      0x22: 0x22, 0x27: 0x27, 0x5c: 0x5c, 0x2f: 0x2f,
      0x62: 0x08, 0x66: 0x0c, 0x6e: 0x0a, 0x72: 0x0d, 0x74: 0x09,
    };
    if (n in simple) {
      out.push(simple[n]);
      return i + 2;
    }
    if (inTemplate && (n === BT || n === DOLLAR)) {
      out.push(n);
      return i + 2;
    }
    if (n !== 0x75) {
      this.fail("bad-escape", i, i + 2, `\\${describeCp(n)} is not an escape`);
    }
    // \u
    if (this.at(i + 2) === LBRACE) {
      let j = i + 3;
      let v = 0;
      while (j < cps.length && isHex(cps[j])) {
        v = Math.min(v * 16 + hexValue(cps[j]), 0x7fffffff);
        j++;
      }
      const digits = j - (i + 3);
      if (digits === 0) this.fail("bad-escape", i, i + 3, "\\u{ needs one to six hex digits");
      if (digits > 6 || this.at(j) !== RBRACE) {
        this.fail("bad-escape", i, j, "\\u{ needs one to six hex digits and a }");
      }
      if (v > 0x10ffff || (v >= 0xd800 && v <= 0xdfff)) {
        this.fail("bad-escape", i, j + 1, "the escape does not name a code point");
      }
      out.push(v);
      return j + 1;
    }
    const v = this.hex4(i);
    if (v === null) {
      let j = i + 2;
      while (j < i + 6 && j < cps.length && isHex(cps[j])) j++;
      this.fail("bad-escape", i, j, "\\u needs four hex digits or {");
    }
    if (v >= 0xdc00 && v <= 0xdfff) {
      this.fail("bad-escape", i, i + 6, "a low surrogate escape does not complete a pair");
    }
    if (v >= 0xd800 && v <= 0xdbff) {
      const w = this.at(i + 6) === BS && this.at(i + 7) === 0x75 ? this.hex4(i + 6) : null;
      if (w === null || w < 0xdc00 || w > 0xdfff) {
        this.fail("bad-escape", i, i + 6, "a high surrogate escape needs a low surrogate escape after it");
      }
      out.push(((v - 0xd800) << 10) + (w - 0xdc00) + 0x10000);
      return i + 12;
    }
    out.push(v);
    return i + 6;
  }

  /** The value of `\uXXXX` at i, or null if four hex digits do not follow. */
  private hex4(i: number): number | null {
    let v = 0;
    for (let k = 2; k < 6; k++) {
      const d = this.at(i + k);
      if (d < 0 || !isHex(d)) return null;
      v = v * 16 + hexValue(d);
    }
    return v;
  }

  // The text of a template, up to its closing ` or the next ${ (01 2.8).
  private templateText(f: Extract<Frame, { t: "tmpl" }>): void {
    const cps = this.cps;
    let i = this.pos;
    for (;;) {
      if (i >= cps.length) {
        this.fail("unterminated-string", f.token.start, cps.length, "the template has no closing `");
      }
      const c = cps[i];
      if (c === BT) {
        if (f.text.length > 0 || f.parts.length === 0) {
          f.parts.push({ kind: "text", value: fromCodePoints(f.text) });
        }
        f.text = [];
        this.pos = i + 1;
        f.token.end = this.pos;
        this.stack.pop();
        this.parentTokens().push(f.token);
        return;
      }
      if (c === DOLLAR && this.at(i + 1) === LBRACE) {
        if (f.text.length > 0) f.parts.push({ kind: "text", value: fromCodePoints(f.text) });
        f.text = [];
        const tokens: Token[] = [];
        f.parts.push({ kind: "interp", open: { start: i, end: i + 2 }, tokens });
        this.pos = i + 2;
        this.gap = false;
        this.stack.push({ t: "interp", tokens, depth: 0 });
        return;
      }
      if (c === BS) {
        const n = this.at(i + 1);
        if (n === LF || n === CR) {
          this.fail("line-continuation", i, i + 1, "a \\ before a line end is not allowed in a template");
        }
        if (n < 0) this.fail("unterminated-string", f.token.start, cps.length, "the template has no closing `");
        i = this.escape(i, true, f.text);
        continue;
      }
      if (c === CR) {
        f.text.push(LF);
        i += this.at(i + 1) === LF ? 2 : 1;
        continue;
      }
      f.text.push(c);
      i++;
    }
  }
}

/**
 * The value of a number token's text (01 2.5), or "bad" when it does not
 * match `Number`, or "range" when it is out of range.
 */
export function numberValue(text: string): number | "bad" | "range" {
  const HEX = /^0x[0-9a-fA-F](?:_?[0-9a-fA-F])*$/;
  const DEC = /^(?:0|[1-9](?:_?[0-9])*)(\.[0-9](?:_?[0-9])*)?([eE][+-]?[0-9](?:_?[0-9])*)?$/;
  if (HEX.test(text)) {
    const v = BigInt("0x" + text.slice(2).replace(/_/g, ""));
    if (v > BigInt(MAX_WHOLE)) return "range";
    return Number(v);
  }
  const m = DEC.exec(text);
  if (m === null) return "bad";
  const clean = text.replace(/_/g, "");
  if (m[1] === undefined && m[2] === undefined) {
    const v = BigInt(clean);
    if (v > BigInt(MAX_WHOLE)) return "range";
    return Number(v);
  }
  const x = Number(clean);
  if (!Number.isFinite(x)) return "range";
  return x + 0;
}

function describeCp(c: number): string {
  if (c < 0) return "at the end of input";
  const hex = "U+" + c.toString(16).toUpperCase().padStart(4, "0");
  if (c > 0x20 && c < 0x7f) return `'${String.fromCodePoint(c)}'`;
  return hex;
}

export type { Span };
