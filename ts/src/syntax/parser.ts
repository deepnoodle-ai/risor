// The parser (01 section 3): hand-written recursive descent, with precedence
// climbing (a Pratt parser) for the binary operators.
//
// The parser stops at the first diagnostic (03 3.7.2 allows it). It recurses
// only where the text opens a nesting level (01 13.1), so its stack depth is
// bounded by the limit of 100 levels. What opens no level is read by a loop:
// the operands of a left-associative chain of operators, the steps of a chain,
// the `else` branch of `? :`, an `else if` chain and the `const` lines.

import type * as A from "./ast.ts";
import { ParseError } from "./diagnostic.ts";
import type { Diagnostic } from "./diagnostic.ts";
import { lex } from "./lexer.ts";
import { Source } from "./source.ts";
import type { Span } from "./source.ts";
import { isReserved } from "./token.ts";
import type { Token } from "./token.ts";
import type { TypeNode } from "../types/typetext.ts";
import { END, parseMessage } from "./messages.ts";

/** The source nesting limit (01 13.2). Fixed by the specification. */
export const MAX_LEVELS = 100;

export type ParseResult =
  | { ok: true; program: A.Program; source: Source }
  | { ok: false; diagnostics: Diagnostic[]; source: Source };

/** Parse a program. Never throws for any text. */
export function parse(text: string | Source): ParseResult {
  const source = typeof text === "string" ? new Source(text) : text;
  try {
    return { ok: true, program: parseProgram(source), source };
  } catch (e) {
    if (e instanceof ParseError) return { ok: false, diagnostics: [e.diagnostic], source };
    throw e;
  }
}

/** Parse a program. Throws ParseError at the first diagnostic. */
/**
 * Parse the tokens of a template part, which end with an `interpEnd` token at
 * the closing brace or delimiter (f02 2.2). Throws ParseError.
 */
export function parsePartTokens(tokens: Token[], source?: Source): A.Expr {
  return new Parser(tokens, 1, source).part();
}

export function parseProgram(source: Source): A.Program {
  const tokens = lex(source);
  return new Parser(tokens, 0, source).program();
}

const OPENERS = new Set(["(", "[", "?.[", "{"]);
const CLOSERS = new Set([")", "]", "}"]);

/** Tokens that could continue an operand: a binary operator, `?` or a step (01 3.4 rule 4). */
const CONTINUES = new Set(["+", "-", "*", "/", "%", "**", "==", "!=", "<", "<=", ">", ">=", "&&", "||", "??", "?", ".", "?.", "[", "?.[", "("]);

const PREC_OR = 3;
const PREC_REL = 6;
const PREC_ADD = 7;

class Parser {
  private toks: Token[];
  private i = 0;
  private level = 0;
  /** The most levels open so far, for a lambda's maxLevel. */
  private maxLevel = 0;
  private prevEnd = 0;
  /** The outer span of an expression written in parentheses. */
  private readonly parens = new WeakMap<object, Span>();
  /** The source text, for the `token` of a message (f08 3.2). */
  private readonly src: Source | undefined;
  /**
   * What each open level is, innermost last: the production the parser reads
   * there, for the rows of f08 2.6 and 2.7.
   */
  private readonly labels: { kind: string; at: Token; arrow?: boolean; sep?: boolean }[] = [];
  /** The token after the last complete chain: a complete operand has been read (f08 2.2). */
  private afterOperand: Token | null = null;
  /** The token of the last operand word (f08 1.1), and the token after it. */
  private operandWord: Token | null = null;
  private afterOperandWord: Token | null = null;
  /** The token after the last program type (f08 2.7 E24). */
  private afterType: Token | null = null;
  /** The token after the last `if` or `try` expression (f08 2.6 D4). */
  private afterBranch: Token | null = null;
  /** The last call by name, with the token after its `)` (f08 2.7 E15). */
  private lastCall: { name: string; word: Token; after: Token } | null = null;
  /** Set while the parser needs the start of an operand (f08 2.2). */
  private operandStart = false;

  constructor(tokens: Token[], level = 0, source?: Source) {
    this.toks = tokens;
    this.level = level;
    this.maxLevel = level;
    this.src = source;
  }

  /** The expression of a template part (f02 2.2 items 8, 9, 11): one Expr, at level 1. */
  part(): A.Expr {
    const e = this.expr();
    if (this.cur.kind !== "interpEnd" || this.cur.text === "lone") this.unexpected(this.cur);
    return e;
  }

  // ---- tokens -------------------------------------------------------------

  private get cur(): Token {
    return this.toks[this.i];
  }

  private peek(k: number): Token {
    const j = Math.min(this.i + k, this.toks.length - 1);
    return this.toks[j];
  }

  private advance(): Token {
    const t = this.toks[this.i];
    if (t.kind !== "eof" && t.kind !== "interpEnd") this.i++;
    this.prevEnd = t.end;
    return t;
  }

  private isP(t: Token, text: string): boolean {
    return t.kind === "punct" && t.text === text;
  }

  private isW(t: Token, word: string): boolean {
    return t.kind === "word" && t.text === word;
  }

  private expect(text: string): Token {
    const t = this.cur;
    if (!this.isP(t, text)) this.unexpected(t);
    return this.advance();
  }

  /** The token where the `:` after a string key or a computed key is needed, when it is not `:` (f08 3.4 template `key`). */
  private keyColon: Token | null = null;

  /** The `:` after a string key or a computed key, in a map literal or a map pattern. */
  private expectKeyColon(): void {
    if (!this.isP(this.cur, ":")) this.keyColon = this.cur;
    this.expect(":");
  }

  /** The template of `unexpected-token` at a token (f08 3.4). */
  private unexpectedTemplate(t: Token): string {
    return t === this.keyColon ? "key" : "default";
  }

  // ---- diagnostics --------------------------------------------------------

  /** A parse diagnostic, with its message from the catalog (f08 3.4). */
  private fail(code: string, span: Span, template = "default", params: Record<string, unknown> = {}): never {
    const ps = { token: this.tokenText(span), ...params };
    throw new ParseError(code, { start: span.start, end: span.end }, parseMessage(code, template, ps));
  }

  /** The `token` of a message: the text at a span, or the end of input (f08 3.2). */
  private tokenText(span: Span): string | typeof END {
    if (span.end <= span.start) return END;
    const text = this.src === undefined ? "" : this.src.slice(span.start, span.end);
    // A span from `not` to the end of `in` is the text `not in` (f08 3.2).
    return /^not(\s|\/\*[\s\S]*?\*\/|\/\/[^\n]*\n)+in$/.test(text) ? "not in" : text;
  }

  /** The token before the current one, in the token list being read. */
  private prevTok(): Token | undefined {
    return this.i > 0 ? this.toks[this.i - 1] : undefined;
  }

  private top(): { kind: string; at: Token; arrow?: boolean; sep?: boolean } | undefined {
    return this.labels[this.labels.length - 1];
  }

  /** `unexpected-token` at a token that does not fit (01 3.5), with the rows of f08 2.5 to 2.7. */
  private unexpected(t: Token): never {
    // A text block's code is reported when the parser reaches it, before the fit test (f03 6.2).
    if (t.blockError !== undefined) this.blockFail(t);
    const operandStart = this.operandStart;
    this.operandStart = false;
    if (t.kind === "reject") this.rejected(t, operandStart);
    if (t.kind === "string") {
      // 01 14.3: the word `r` or `R`, white space, then a quote (f08 2.5 C10).
      const prev = this.prevTok();
      if (prev !== undefined && prev.kind === "word" && (prev.text === "r" || prev.text === "R") && t.gap && this.toks[this.i] === t) {
        this.fail("string-prefix", prev, "space");
      }
    }
    if (t.kind === "word" && t.text === "not" && this.toks[this.i] === t && this.isW(this.peek(1), "in")) {
      // 01 3.5: an operator `not in` that cannot follow spans both words.
      if (t === this.afterBranch) this.fail("needs-parentheses", { start: t.start, end: this.peek(1).end }, "after-branch", { token: "not in" });
      this.fail("unexpected-token", { start: t.start, end: this.peek(1).end }, this.unexpectedTemplate(t), { token: "not in" });
    }
    this.otherRows(t, operandStart);
    // f08 2.6 D4: an operator, a `?` or a step after the last branch of an if or the handler of a try.
    // It is tried after every row of 2.7 (f08 2.1 item 6).
    if (t === this.afterBranch && (CONTINUES.has(t.text) && t.kind === "punct" || this.isW(t, "in") || this.isW(t, "as") || this.isW(t, "is"))) this.fail("needs-parentheses", t, "after-branch");
    this.fail("unexpected-token", t, this.unexpectedTemplate(t));
  }

  /**
   * The rows tried before a row of f08 2.6 or 2.7 that the parser applies at a
   * fixed place (f08 2.1 item 2): a text block's own code (2.9 item 3), then a
   * token that no rule accepts, which only 2.5 maps, then the rows of 2.7
   * numbered below `row` (0 for a row of 2.6).
   */
  private earlier(t: Token, row = 0): void {
    if (t.blockError !== undefined) this.blockFail(t);
    if (t.kind === "reject") this.rejected(t, false);
    if (row > 0) this.otherRows(t, false, row);
  }

  /** f08 2.5: tokens that no rule accepts, C1 to C9. */
  private rejected(t: Token, operandStart: boolean): never {
    const span = t.report ?? t;
    switch (t.text) {
      case "===":
        this.fail("strict-equality", span, "equal");
      case "!==":
        this.fail("strict-equality", span, "not-equal");
      case "+=": case "-=": case "*=": case "/=": case "%=": case "**=": case "??=": case "||=": case "&&=":
        this.fail("assignment", span, "compound");
      case "++":
        this.fail("assignment", span, "increment");
      case "|>":
        this.fail("pipe-operator", span);
      case "a text block":
        this.fail("text-block-quote", span);
    }
    const pre = t.prefix;
    if (pre !== undefined) {
      if (pre.word === "r" && pre.triple) this.fail("text-block-quote", span);
      const prev = this.prevTok();
      const place = operandStart || (prev !== undefined && (this.isP(prev, ".") || this.isP(prev, "?.")));
      if (place) {
        if (pre.quote === "`") this.fail("tagged-template", span);
        this.fail("string-prefix", span, "prefix");
      }
    }
    this.fail("unexpected-token", span, this.unexpectedTemplate(t));
  }

  /** f08 2.7: the rows E3 to E25 that need only the failing token and the parser's state, those below `below`. */
  private otherRows(t: Token, operandStart: boolean, below = Infinity): void {
    const prev = this.prevTok();
    const top = this.top();
    const next = this.peek(1);
    const isWord = (x: Token | undefined, ...ws: string[]) => x !== undefined && x.kind === "word" && ws.includes(x.text);
    // A complete operand has been read (f08 2.2): an if or a try is one too.
    const complete = t === this.afterOperand || t === this.afterBranch;
    // E3
    if (isWord(t, "finally")) this.fail("finally-clause", t);
    if (below <= 7) return;
    // E7: `=` after a complete operand.
    if (this.isP(t, "=") && complete) this.fail("assignment", t, "assign");
    if (below <= 8) return;
    // E8: `:` in the expression of an index step.
    if (this.isP(t, ":") && top?.kind === "index") this.fail("slice-syntax", t);
    // E9: `:` after a name in parentheses.
    if (this.isP(t, ":") && top?.kind === "paren" && prev !== undefined && prev.kind === "word" && this.i >= 2 && this.toks[this.i - 2] === top.at) this.fail("type-annotation", t);
    if (below <= 10) return;
    // E10
    if (this.isP(t, "|")) this.fail("bitwise-operator", t, "or");
    // E11
    if (this.isP(t, "*") && prev !== undefined && (this.isP(prev, ".") || this.isP(prev, "?."))) this.fail("star-form", t);
    if (this.isP(t, "]") && top?.kind === "index" && prev === top.at) this.fail("star-form", t);
    // E12
    if (this.isP(t, "?") && top?.kind === "index" && prev === top.at) this.fail("filter-step", t);
    if (below <= 13) return;
    // E13, E14
    if (isWord(t, "satisfies")) this.fail("satisfies-operator", t);
    if (isWord(t, "instanceof")) this.fail("instanceof-operator", t);
    if (below <= 15) return;
    // E15 to E18: the token after an operand word.
    const word = this.operandWord;
    if (word !== null && t === this.afterOperandWord) {
      if (isWord(word, "switch", "case")) this.fail("switch-statement", word);
      if (isWord(word, "typeof")) this.fail("typeof-operator", word);
      if (isWord(word, "interface", "enum", "type")) this.fail("type-declaration", word, "default", { token: word.text });
      if (isWord(word, "export")) this.fail("export-statement", word);
    }
    if (this.isP(t, "{") && this.lastCall !== null && this.lastCall.name === "switch" && t === this.lastCall.after) this.fail("switch-statement", this.lastCall.word);
    if (below <= 19) return;
    // E19, E20: `<` where an operand must begin.
    if (this.isP(t, "<") && operandStart) {
      if (this.isP(next, "<") && !next.gap) this.fail("heredoc", t);
      this.fail("angle-cast", t);
    }
    if (below <= 22) return;
    // E22: a hyphenated key in a map literal or a map pattern.
    if (this.isP(t, "-") && (top?.kind === "map" || top?.kind === "mappat") && prev !== undefined && prev.kind === "word") {
      const before = this.i >= 2 ? this.toks[this.i - 2] : undefined;
      if (before !== undefined && (before === top.at || this.isP(before, ","))) {
        let j = this.i;
        let end = prev.end;
        let last = prev;
        while (this.isP(this.toks[j], "-") && !this.toks[j].gap && this.toks[j].start === last.end) {
          const w = this.toks[j + 1];
          if (w === undefined || (w.kind !== "word" && w.kind !== "number") || w.gap) break;
          end = w.end;
          last = w;
          j += 2;
        }
        if (last !== prev && this.isP(this.toks[j], ":")) {
          const key = this.src === undefined ? "" : this.src.slice(prev.start, end);
          this.fail("unquoted-key", { start: prev.start, end }, "default", { key });
        }
      }
    }
    // E23: a lambda body in braces read as a map; not a `=` where the `:` after a string or computed key is needed.
    if (top?.kind === "map" && top.arrow === true && top.sep !== true && !(this.isP(t, "=") && t === this.keyColon)) this.fail("block-without-const", top.at);
    // E24: after a program type.
    if (t === this.afterType && (this.isP(t, "+") || this.isP(t, "-") || this.isP(t, "*") || this.isP(t, "/") || this.isP(t, "%") || this.isP(t, "**") || this.isP(t, ".") || this.isP(t, "?.") || this.isP(t, "?.[") || this.isP(t, "[") || this.isP(t, "(") || this.isP(t, "{"))) {
      this.fail("needs-parentheses", t, "after-type");
    }
    // E25
    if (this.isP(t, "!") && complete && isWord(next, "is")) this.fail("negated-is", t);
  }

  private blockFail(t: Token): never {
    const e = t.blockError!;
    this.fail(e.code, e, e.template ?? "default");
  }

  /**
   * `reserved-word` (01 14.2), with the rows of f08 2.4. `place`: `name` where
   * the grammar needs a Name, `operand` where an operand must begin, and
   * `shorthand` for a shorthand entry of a map literal.
   */
  private reservedWord(t: Token, place: "name" | "operand" | "shorthand" = "name"): never {
    if (place !== "name") {
      const codes: Record<string, string> = {
        let: "let-binding", var: "let-binding", for: "loop-statement", while: "loop-statement", do: "loop-statement",
        break: "loop-statement", continue: "loop-statement", return: "return-statement", function: "function-declaration",
        throw: "throw-statement", finally: "finally-clause", import: "import-statement", nil: "nil-literal",
      };
      const code = codes[t.text];
      if (code !== undefined) this.fail(code, t);
    }
    // B9: the first entry of a map literal that is a lambda body.
    const top = this.top();
    if (place === "shorthand" && top?.kind === "map" && top.arrow === true && top.sep !== true) this.fail("block-without-const", top.at);
    this.fail("reserved-word", t, place === "operand" ? "operand" : "name");
  }

  /** Open a nesting level at a token (01 13.1), and say what the parser reads there. */
  private open(span: Span, kind = "other"): void {
    if (this.level >= MAX_LEVELS) {
      this.fail("nesting-exceeded", span);
    }
    this.level++;
    if (this.level > this.maxLevel) this.maxLevel = this.level;
    this.labels.push({ kind, at: span as Token });
  }

  private close(): void {
    this.level--;
    this.labels.pop();
  }

  private outer(e: A.Expr | A.Lambda | A.Pattern): Span {
    return this.parens.get(e) ?? e.span;
  }

  private span(start: number): Span {
    return { start, end: this.prevEnd };
  }

  // ---- programs, consts, blocks, branches --------------------------------

  program(): A.Program {
    const start = this.cur.start;
    const consts = this.consts();
    const result = this.expr();
    if (this.isP(this.cur, ";")) this.advance();
    if (this.cur.kind !== "eof") this.unexpected(this.cur);
    return { kind: "Program", consts, result, span: { start, end: this.prevEnd } };
  }

  private consts(): A.Const[] {
    const out: A.Const[] = [];
    while (this.isW(this.cur, "const")) out.push(this.constLine());
    return out;
  }

  private constLine(): A.Const {
    const start = this.advance().start; // const
    const pattern = this.pattern();
    // f08 2.7 E9: `:` after the pattern of a const.
    if (this.isP(this.cur, ":")) this.fail("type-annotation", this.cur);
    this.expect("=");
    let value: A.Expr | A.Lambda;
    if (this.lambdaStartsHere()) {
      value = this.lambda((arrow) => {
        // 01 3.4 rule 9: only a Name may bind a lambda.
        if (!(pattern.kind === "NamePat" && pattern.name !== null)) {
          this.fail("lambda-in-pattern", arrow);
        }
      });
    } else {
      value = this.expr();
    }
    this.expect(";");
    return { kind: "Const", pattern, value, span: this.span(start) };
  }

  /** A branch (01 3.1, 3.2 rule 2, 3.4 rules 7 and 8). */
  private branch(): A.Expr {
    const lb = this.cur;
    if (!this.isP(lb, "{")) this.unexpected(lb);
    this.advance();
    this.open(lb);
    if (this.isP(this.cur, "...")) {
      this.fail("branch-like-map", lb);
    }
    if (this.isP(this.cur, "}")) {
      this.fail("empty-branch", lb);
    }
    const consts = this.consts();
    const result = this.expr();
    if (this.isP(this.cur, ":") || this.isP(this.cur, ",")) {
      this.fail("branch-like-map", lb);
    }
    this.expect("}");
    this.close();
    if (consts.length === 0) return result;
    return { kind: "Block", consts, result, span: this.span(lb.start) };
  }

  private block(): A.Block {
    const lb = this.advance();
    this.open(lb);
    const consts = this.consts();
    const result = this.expr();
    this.expect("}");
    this.close();
    return { kind: "Block", consts, result, span: this.span(lb.start) };
  }

  // ---- expressions --------------------------------------------------------

  /** Expr = If | Try | Binary [ "?" Expr ":" Expr ]. The else part is a loop. */
  private expr(): A.Expr {
    const pending: { test: A.Expr; then: A.Expr }[] = [];
    let e: A.Expr;
    for (;;) {
      const t = this.cur;
      if (this.isW(t, "if")) {
        e = this.ifExpr();
        this.afterBranch = this.cur;
        break;
      }
      if (this.isW(t, "try")) {
        e = this.tryExpr();
        this.afterBranch = this.cur;
        break;
      }
      const b = this.binary();
      if (this.isP(this.cur, "?")) {
        const q = this.advance();
        this.open(q);
        const then = this.expr();
        // f08 2.7 E21: a `?` directly followed by `[` is an optional index written without the dot.
        const afterQ = this.toks[this.toks.indexOf(q) + 1];
        if (!this.isP(this.cur, ":") && afterQ !== undefined && this.isP(afterQ, "[") && !afterQ.gap && afterQ.start === q.end) {
          this.earlier(this.cur, 21);
          this.fail("optional-index", q);
        }
        this.expect(":");
        this.close();
        pending.push({ test: b, then });
        continue;
      }
      e = b;
      break;
    }
    for (let k = pending.length - 1; k >= 0; k--) {
      const { test, then } = pending[k];
      e = {
        kind: "Cond", form: "?:", test, then, else: e,
        span: { start: this.outer(test).start, end: this.outer(e).end },
      };
    }
    return e;
  }

  private ifExpr(): A.Expr {
    const parts: { start: number; test: A.Expr; then: A.Expr }[] = [];
    let ifTok = this.advance();
    let elseBranch: A.Expr;
    // f08 2.7 E4: the form of an if.
    const form = () => {
      this.earlier(this.cur, 4);
      this.fail("if-form", this.cur, "form");
    };
    for (;;) {
      if (!this.isP(this.cur, "(")) form();
      const lp = this.expect("(");
      this.open(lp, "if-test");
      const test = this.expr();
      // f08 2.7 E5: a `,` after the test.
      if (this.isP(this.cur, ",")) this.fail("if-form", this.cur, "call");
      this.expect(")");
      this.close();
      if (!this.isP(this.cur, "{")) form();
      const then = this.branch();
      parts.push({ start: ifTok.start, test, then });
      // 01 3.4 rule 11 (f08 2.6 D11).
      if (!this.isW(this.cur, "else")) {
        this.earlier(this.cur);
        this.fail("missing-else", this.cur);
      }
      this.advance();
      if (this.isW(this.cur, "if")) {
        ifTok = this.advance();
        continue;
      }
      if (!this.isP(this.cur, "{")) form();
      elseBranch = this.branch();
      break;
    }
    const end = this.prevEnd;
    let e = elseBranch;
    for (let k = parts.length - 1; k >= 0; k--) {
      const p = parts[k];
      e = { kind: "Cond", form: "if", test: p.test, then: p.then, else: e, span: { start: p.start, end } };
    }
    return e;
  }

  private tryExpr(): A.Try {
    const tryTok = this.advance();
    if (this.isP(this.cur, "(")) {
      this.fail("try-call", tryTok); // 01 3.4 rule 10
    }
    // f08 2.7 E6, 2.6 D14: the body and the handler are written in braces.
    const braces = (row: number) => {
      if (this.isP(this.cur, "{")) return;
      this.earlier(this.cur, row);
      this.fail("try-form", this.cur);
    };
    braces(6);
    const body = this.branch();
    // 01 3.4 rule 12 (f08 2.6 D12, D13).
    if (!this.isW(this.cur, "catch")) {
      this.earlier(this.cur);
      if (this.isW(this.cur, "finally")) this.fail("finally-clause", this.cur);
      this.fail("missing-catch", this.cur);
    }
    this.advance();
    let name: string | null = null;
    let nameSpan: Span | null = null;
    if (this.isP(this.cur, "(")) {
      const lp = this.advance();
      this.open(lp);
      // f08 2.6 D15: a token where the name should stand, or after it.
      const n = this.cur;
      if (n.kind === "word" && isReserved(n.text)) this.reservedWord(n);
      if (n.kind !== "word" || n.text === "_") {
        this.earlier(n);
        this.fail("catch-name", n);
      }
      const t = this.advance();
      name = t.text;
      nameSpan = { start: t.start, end: t.end };
      if (!this.isP(this.cur, ")")) {
        this.earlier(this.cur);
        this.fail("catch-name", this.cur);
      }
      this.advance();
      this.close();
    }
    braces(0);
    const handler = this.branch();
    return { kind: "Try", body, name, nameSpan, handler, span: this.span(tryTok.start) };
  }

  /** Binary = Or | Coal (01 3.2 rule 4). */
  private binary(): A.Expr {
    const left = this.climb(this.unary(null), PREC_ADD);
    if (this.isP(this.cur, "??")) {
      let x = left;
      while (this.isP(this.cur, "??")) {
        const op = this.advance();
        this.open(op);
        const y = this.climb(this.unary(null), PREC_ADD);
        this.close();
        x = this.binaryNode("??", x, y);
      }
      // 01 3.4 rule 3 (f08 2.6 D3): `??` does not mix with the operators of levels 3 to 6.
      const p = this.prec();
      if (p >= PREC_OR && p <= PREC_REL) {
        const t = this.cur;
        if (this.isW(t, "not") && this.isW(this.peek(1), "in")) this.fail("needs-parentheses", { start: t.start, end: this.peek(1).end }, "coalesce", { token: "not in" });
        if (!this.isW(t, "not")) this.fail("needs-parentheses", t, "coalesce");
      }
      return x;
    }
    const out = this.climb(left, PREC_OR);
    if (out !== left && this.isP(this.cur, "??")) this.fail("needs-parentheses", this.cur, "coalesce");
    return out;
  }

  private binaryNode(op: A.BinaryOp, x: A.Expr, y: A.Expr): A.Binary {
    return { kind: "Binary", op, x, y, span: { start: this.outer(x).start, end: this.outer(y).end } };
  }

  /** The level of the binary operator at the current token, or 0. `??` is not here. */
  private prec(): number {
    const t = this.cur;
    if (t.kind === "punct") {
      switch (t.text) {
        case "||": return 3;
        case "&&": return 4;
        case "==": case "!=": return 5;
        case "<": case "<=": case ">": case ">=": return 6;
        case "+": case "-": return 7;
        case "*": case "/": case "%": return 8;
      }
      return 0;
    }
    if (t.kind === "word") {
      if (t.text === "in" || t.text === "as" || t.text === "is") return 6;
      // `not` is read as the start of `not in`; a `not` without `in` fails
      // at the token after it, where the text stops fitting (01 3.5).
      if (t.text === "not") return 6;
    }
    return 0;
  }

  private isBareNot(e: A.Expr): boolean {
    return e.kind === "Unary" && e.op === "!" && !this.parens.has(e);
  }

  /** Precedence climbing over levels 3 to 8 (01 3.3). */
  private climb(left: A.Expr, minPrec: number): A.Expr {
    for (;;) {
      const p = this.prec();
      if (p === 0 || p < minPrec) return left;
      const t = this.cur;
      if (t.kind === "word" && (t.text === "as" || t.text === "is")) {
        left = this.test(left, t);
        // f04 2.4: an operator of levels 7 to 10 cannot follow a program type.
        const n = this.cur;
        if (n.kind === "punct" && (n.text === "+" || n.text === "-" || n.text === "*" || n.text === "/" || n.text === "%")) this.unexpected(n);
        continue;
      }
      if (t.kind === "word" && t.text === "not" && !this.isW(this.peek(1), "in")) {
        this.advance();
        this.unexpected(this.cur);
      }
      let op: A.BinaryOp;
      let opSpan: Span;
      if (t.kind === "word" && t.text === "not") {
        op = "not-in";
        opSpan = { start: t.start, end: this.peek(1).end };
      } else {
        op = (t.text === "in" ? "in" : t.text) as A.BinaryOp;
        opSpan = t;
      }
      if ((op === "in" || op === "not-in") && this.isBareNot(left)) {
        // 01 3.4 rule 2.
        this.fail("needs-parentheses", { start: left.span.start, end: left.span.start + 1 }, "in");
      }
      this.advance();
      if (op === "not-in") this.advance();
      this.open(opSpan);
      const right = this.climb(this.unary(null), p + 1);
      this.close();
      left = this.binaryNode(op, left, right);
    }
  }

  // ---- program types (package F4, f04 2) ------------------------------------

  /** `x as T`, `x is T`, at the word `as` or `is` (f04 2.1 to 2.8). */
  private test(left: A.Expr, t: Token): A.As {
    // f04 2.5: `!x as T` at the `!`.
    if (this.isBareNot(left)) this.fail("needs-parentheses", { start: left.span.start, end: left.span.start + 1 }, "not-test", { op: t.text });
    // f04 2.6 item 2: `x as const` and `x is not T` keep Core's span.
    const nx = this.peek(1);
    if ((t.text === "as" && this.isW(nx, "const")) || (t.text === "is" && this.isW(nx, "not"))) {
      this.fail(t.text === "as" ? "as-const" : "negated-is", t);
    }
    this.advance();
    this.open(t);
    const spans = new Map<TypeNode, Span>();
    const start = this.cur.start;
    const type = this.ptype(spans);
    const typeSpan = { start, end: this.prevEnd };
    this.close();
    this.afterType = this.cur;
    return { kind: "As", form: t.text as "as" | "is", x: left, type, typeSpan, typeSpans: spans, span: { start: this.outer(left).start, end: this.prevEnd } };
  }

  /**
   * `bad-type-text` (f04 2.6), with its template by f08 3.4: `where` is what
   * the parser expected, a type (`type`), the `<` or `>` of `map<`, the `)` of
   * a parenthesized type, or a part of a record type.
   */
  private badType(t: Token, where: "type" | "map-open" | "map-close" | "paren" | "record" = "type"): never {
    if (t.blockError !== undefined) this.blockFail(t);
    if (t.kind === "reject") this.unexpected(t);
    if (t.kind === "template") this.fail("bad-type-text", t, "literal");
    if (where === "map-close" && this.isP(t, ">=")) this.fail("bad-type-text", t, "split");
    if (where !== "type") this.fail("bad-type-text", t, where);
    if (t.kind === "number" || this.isW(t, "true") || this.isW(t, "false") || this.isP(t, "-")) this.fail("bad-type-text", t, "value");
    if (t.kind === "word") {
      switch (t.text) {
        case "number": this.fail("bad-type-text", t, "number-word");
        case "boolean": this.fail("bad-type-text", t, "boolean-word");
        case "undefined": case "void": case "nil": this.fail("bad-type-text", t, "null-word");
        case "object": this.fail("bad-type-text", t, "object-word");
      }
      if (/^[A-Z]$/.test(t.text)) this.fail("bad-type-text", t, "letter");
      if (/^[a-z_]/.test(t.text)) this.fail("bad-type-text", t, "lower");
    }
    this.fail("bad-type-text", t, "expected");
  }

  /** PType = PAlt { "|" PAlt }. */
  private ptype(spans: Map<TypeNode, Span>): TypeNode {
    const start = this.cur.start;
    const alts = [this.palt(spans)];
    while (this.isP(this.cur, "|")) {
      this.advance();
      alts.push(this.palt(spans));
    }
    if (alts.length === 1) return alts[0];
    const node: TypeNode = { t: "union", alts };
    spans.set(node, { start, end: this.prevEnd });
    return node;
  }

  /** PAlt = PPrim { "[" "]" }. The levels of the suffixes last to the end of the PAlt (f04 2.8). */
  private palt(spans: Map<TypeNode, Span>): TypeNode {
    let node = this.pprim(spans);
    let opened = 0;
    while (this.isP(this.cur, "[") && this.isP(this.peek(1), "]")) {
      this.open(this.cur);
      opened++;
      this.advance();
      this.advance();
      node = { t: "list", elem: node };
    }
    for (let k = 0; k < opened; k++) this.close();
    return node;
  }

  private pprim(spans: Map<TypeNode, Span>): TypeNode {
    const t = this.cur;
    if (t.kind === "word") {
      switch (t.text) {
        case "null": case "bool": case "int": case "float": case "string": case "any": case "never":
          this.advance();
          return { t: "word", word: t.text };
        case "map": {
          this.advance();
          const lt = this.cur;
          if (!this.isP(lt, "<")) this.badType(lt, "map-open");
          this.advance();
          this.open(lt);
          const rest = this.ptype(spans);
          if (!this.isP(this.cur, ">")) this.badType(this.cur, "map-close");
          this.advance();
          this.close();
          return { t: "map", rest };
        }
      }
      if (/^[A-Z][A-Za-z0-9_]+$/.test(t.text)) {
        this.advance();
        const node: TypeNode = { t: "name", name: t.text };
        spans.set(node, { start: t.start, end: t.end });
        return node;
      }
      this.badType(t);
    }
    if (t.kind === "string") {
      this.advance();
      return { t: "literal", value: t.str! };
    }
    if (this.isP(t, "(")) {
      this.advance();
      this.open(t);
      const inner = this.ptype(spans);
      if (!this.isP(this.cur, ")")) this.badType(this.cur, "paren");
      this.advance();
      this.close();
      return inner;
    }
    if (this.isP(t, "{")) return this.precord(spans);
    this.badType(t);
  }

  /** PRecord (f04 2.1). A key written twice is bad-type-text at the later key (2.6). */
  private precord(spans: Map<TypeNode, Span>): TypeNode {
    const lb = this.advance();
    this.open(lb);
    const fields: { key: string; optional: boolean; type: TypeNode }[] = [];
    let rest: "exact" | "any" | TypeNode = "exact";
    const seen = new Set<string>();
    for (;;) {
      const t = this.cur;
      if (this.isP(t, "}")) break;
      if (this.isP(t, "...")) {
        this.advance();
        rest = this.isP(this.cur, ",") || this.isP(this.cur, "}") ? "any" : this.ptype(spans);
        if (this.isP(this.cur, ",")) {
          this.advance();
          if (!this.isP(this.cur, "}")) this.badType(this.cur, "record");
        } else if (!this.isP(this.cur, "}")) this.badType(this.cur, "record");
        break;
      }
      let key: string;
      if (t.kind === "word") key = t.text;
      else if (t.kind === "string") key = t.str!;
      else this.badType(t, "record");
      if (seen.has(key)) this.fail("bad-type-text", t, "duplicate", { key });
      seen.add(key);
      this.advance();
      let optional = false;
      if (this.isP(this.cur, "?")) {
        this.advance();
        optional = true;
      }
      if (!this.isP(this.cur, ":")) this.badType(this.cur, "record");
      this.advance();
      fields.push({ key, optional, type: this.ptype(spans) });
      if (this.isP(this.cur, ",")) {
        this.advance();
        continue;
      }
      if (!this.isP(this.cur, "}")) this.badType(this.cur, "record");
      break;
    }
    this.advance();
    this.close();
    return { t: "record", fields, rest };
  }

  /** Unary = ( "!" | "-" ) Unary | Power. */
  private unary(prefix: Token | null): A.Expr {
    const t = this.cur;
    if (this.isP(t, "!") || this.isP(t, "-")) {
      this.advance();
      this.open(t);
      // 01 3.4 rule 1: the span is the first prefix operator.
      const x = this.unary(prefix ?? t);
      this.close();
      return { kind: "Unary", op: t.text as "!" | "-", x, span: { start: t.start, end: this.outer(x).end } };
    }
    return this.power(prefix);
  }

  /** Power = Chain [ "**" Unary ]. */
  private power(prefix: Token | null): A.Expr {
    const base = this.chain();
    if (this.isP(this.cur, "**")) {
      if (prefix !== null) {
        // 01 3.4 rule 1, found at the `**`.
        this.fail("needs-parentheses", prefix, "power");
      }
      const op = this.advance();
      this.open(op);
      const y = this.unary(null);
      this.close();
      return this.binaryNode("**", base, y);
    }
    return base;
  }

  /** Chain = Primary { step }. A loop: steps open no level. */
  private chain(): A.Expr {
    const level = this.level;
    let x = this.primary();
    const start = this.outer(x).start;
    let optional = false;
    for (;;) {
      const t = this.cur;
      if (this.isP(t, ".") || this.isP(t, "?.")) {
        const opt = t.text === "?.";
        this.advance();
        const w = this.cur;
        if (w.kind !== "word") this.unexpected(w);
        this.advance();
        if (this.isP(this.cur, "(")) {
          const args = this.args();
          x = { kind: "MethodCall", x, name: w.text, nameSpan: { start: w.start, end: w.end }, args, optional: opt, level, span: this.span(start) };
        } else {
          x = { kind: "Member", x, name: w.text, nameSpan: { start: w.start, end: w.end }, optional: opt, span: this.span(start) };
        }
        optional ||= opt;
        continue;
      }
      if ((this.isP(t, "[") || this.isP(t, "?.[")) && this.isP(this.peek(1), "*")) {
        // f07 2.1: a star begins a projection; only a first star `?.[*]` wraps the chain (2.3 item 6).
        const opt = t.text === "?.[";
        x = this.projection(x, start, opt);
        optional ||= opt;
        continue;
      }
      if (this.isP(t, "[") || this.isP(t, "?.[")) {
        const opt = t.text === "?.[";
        this.advance();
        this.open(t, "index");
        const index = this.expr();
        this.expect("]");
        this.close();
        x = { kind: "Index", x, index, optional: opt, span: this.span(start) };
        optional ||= opt;
        continue;
      }
      break;
    }
    if (optional) x = { kind: "OptChain", x, span: { start, end: x.span.end } };
    this.afterOperand = this.cur;
    return x;
  }

  /**
   * A projection (f07 2.1 to 2.5), at its first star: the star and the reads
   * and stars that follow, up to a method call or the end of the chain. Each
   * star opens a level that closes at the end of the projection.
   */
  private projection(x: A.Expr, start: number, optional: boolean): A.Each {
    let opened = 0;
    const star = (): Span => {
      const t = this.advance(); // `[` or `?.[`
      this.open(t);
      opened++;
      this.advance(); // `*`
      if (!this.isP(this.cur, "]")) this.unexpected(this.cur);
      this.advance();
      return this.span(start);
    };
    const bad = (): A.Bad => ({ kind: "Bad", span: { start, end: start } });
    const root: A.Each = { kind: "Each", x, reads: [], optional, starSpan: { start, end: start }, recvSpan: x.span, span: { start, end: start } };
    root.starSpan = star();
    const all: A.Each[] = [root];
    let level = root;
    let recv = root.starSpan;
    for (;;) {
      const t = this.cur;
      if (this.isP(t, ".") || this.isP(t, "?.")) {
        // A method call ends the projection (f07 3.4).
        if (this.peek(1).kind === "word" && this.isP(this.peek(2), "(")) break;
        const opt = t.text === "?.";
        this.advance();
        const w = this.cur;
        if (w.kind !== "word") this.unexpected(w);
        this.advance();
        const m: A.Member = { kind: "Member", x: bad(), name: w.text, nameSpan: { start: w.start, end: w.end }, optional: opt, span: this.span(start) };
        level.reads.push(m);
        recv = m.span;
        continue;
      }
      if (this.isP(t, "[") || this.isP(t, "?.[")) {
        const opt = t.text === "?.[";
        if (this.isP(this.peek(1), "*")) {
          const later: A.Each = { kind: "Each", x: null, reads: [], optional: opt, starSpan: { start, end: start }, recvSpan: recv, span: { start, end: start } };
          later.starSpan = star();
          level.reads.push(later);
          all.push(later);
          level = later;
          recv = later.starSpan;
          continue;
        }
        this.advance();
        this.open(t, "index");
        const index = this.expr();
        this.expect("]");
        this.close();
        const ix: A.Index = { kind: "Index", x: bad(), index, optional: opt, span: this.span(start) };
        level.reads.push(ix);
        recv = ix.span;
        continue;
      }
      break;
    }
    for (let k = 0; k < opened; k++) this.close();
    for (const e of all) e.span = { start, end: this.prevEnd };
    return root;
  }

  private primary(): A.Expr {
    const t = this.cur;
    switch (t.kind) {
      case "number":
        this.advance();
        return { kind: "Number", value: t.num!, span: { start: t.start, end: t.end } };
      case "string":
        this.advance();
        return { kind: "String", value: t.str!, form: t.form ?? t.quote!, span: { start: t.start, end: t.end } };
      case "template":
        return this.template();
      case "word":
        return this.wordPrimary(t);
      case "punct":
        switch (t.text) {
          case "(": {
            this.advance();
            this.open(t, "paren");
            const e = this.expr();
            this.expect(")");
            this.close();
            this.parens.set(e, this.span(t.start));
            e.parens = true;
            return e;
          }
          case "[":
            return this.list();
          case "{": {
            const n1 = this.peek(1);
            const n2 = this.peek(2);
            if (this.isW(n1, "const") && !this.isP(n2, ":")) return this.block();
            return this.map();
          }
        }
    }
    this.operandStart = true;
    this.unexpected(t);
  }

  private wordPrimary(t: Token): A.Expr {
    switch (t.text) {
      case "true":
      case "false":
        this.advance();
        return { kind: "Bool", value: t.text === "true", span: { start: t.start, end: t.end } };
      case "null":
        this.advance();
        return { kind: "Null", span: { start: t.start, end: t.end } };
      case "match":
        return this.matchExpr();
      case "if":
      case "try":
        // 01 3.4 rule 5: an if or a try is not an operand (f08 2.6 D5).
        this.fail("needs-parentheses", t, "operand");
    }
    // f08 2.4 item 1: a word before `=>` is a parameter, a Name.
    if (isReserved(t.text)) this.reservedWord(t, this.isP(this.peek(1), "=>") ? "name" : "operand");
    if (t.text === "_") this.unexpected(t);
    const level = this.level;
    this.advance();
    if (this.isP(this.cur, "(")) {
      const args = this.args();
      this.lastCall = { name: t.text, word: t, after: this.cur };
      return { kind: "Call", namespace: null, name: t.text, nameSpan: { start: t.start, end: t.end }, args, level, span: this.span(t.start) };
    }
    this.operandWord = t;
    this.afterOperandWord = this.cur;
    return { kind: "Ident", name: t.text, span: { start: t.start, end: t.end } };
  }

  private template(): A.Expr {
    if (this.cur.blockError !== undefined) this.blockFail(this.cur);
    const t = this.advance();
    const parts = t.parts!;
    const span = { start: t.start, end: t.end };
    if (!parts.some((p) => p.kind === "interp")) {
      const value = parts.map((p) => (p.kind === "text" ? p.value : "")).join("");
      return { kind: "String", value, form: t.form ?? "`", span };
    }
    const out: (string | A.Expr)[] = [];
    for (const p of parts) {
      if (p.kind === "text") {
        if (p.value !== "") out.push(p.value);
        continue;
      }
      this.open(p.open);
      const saved = { toks: this.toks, i: this.i, prevEnd: this.prevEnd };
      this.toks = p.tokens;
      this.i = 0;
      const e = this.expr();
      if (this.cur.kind !== "interpEnd") this.unexpected(this.cur);
      this.toks = saved.toks;
      this.i = saved.i;
      this.prevEnd = saved.prevEnd;
      this.close();
      out.push(e);
    }
    return { kind: "Template", parts: out, form: t.form === '"""' ? '"""' : "`", span };
  }

  private args(): A.Arg[] {
    const lp = this.advance();
    this.open(lp);
    const args: A.Arg[] = [];
    for (;;) {
      if (this.isP(this.cur, ")")) break;
      args.push(this.lambdaStartsHere() ? this.lambda(null) : this.expr());
      if (this.isP(this.cur, ",")) {
        this.advance();
        continue;
      }
      if (this.isP(this.cur, ")")) break;
      this.unexpected(this.cur);
    }
    this.advance();
    this.close();
    return args;
  }

  private list(): A.List {
    const lb = this.advance();
    this.open(lb);
    const items: (A.Expr | A.Spread)[] = [];
    for (;;) {
      if (this.isP(this.cur, "]")) break;
      if (this.isP(this.cur, "...")) {
        const d = this.advance();
        const value = this.expr();
        items.push({ kind: "Spread", value, span: { start: d.start, end: this.outer(value).end } });
      } else {
        items.push(this.expr());
      }
      if (this.isP(this.cur, ",")) {
        this.advance();
        continue;
      }
      if (this.isP(this.cur, "]")) break;
      this.unexpected(this.cur);
    }
    this.advance();
    this.close();
    return { kind: "List", items, span: this.span(lb.start) };
  }

  private map(): A.MapLit {
    const prev = this.prevTok();
    const lb = this.advance();
    this.open(lb, "map");
    // f08 2.4 B9, 2.7 E23: a map literal that is a lambda body.
    const label = this.top()!;
    label.arrow = prev !== undefined && this.isP(prev, "=>");
    const entries: (A.Entry | A.Spread)[] = [];
    for (;;) {
      const t = this.cur;
      if (this.isP(t, "}")) break;
      if (this.isP(t, "...")) {
        label.sep = true;
        this.advance();
        const value = this.expr();
        entries.push({ kind: "Spread", value, span: { start: t.start, end: this.outer(value).end } });
      } else if (t.kind === "word" && this.isP(this.peek(1), ":")) {
        label.sep = true;
        this.advance();
        this.advance();
        const value = this.expr();
        entries.push({
          kind: "Entry", key: { kind: "word", text: t.text, span: { start: t.start, end: t.end } },
          value, shorthand: false, span: { start: t.start, end: this.outer(value).end },
        });
      } else if (t.kind === "string") {
        this.advance();
        this.expectKeyColon();
        label.sep = true;
        const value = this.expr();
        entries.push({
          kind: "Entry", key: { kind: "string", value: t.str!, span: { start: t.start, end: t.end } },
          value, shorthand: false, span: { start: t.start, end: this.outer(value).end },
        });
      } else if (this.isP(t, "[")) {
        this.advance();
        this.open(t);
        const k = this.expr();
        this.expect("]");
        this.close();
        this.expectKeyColon();
        label.sep = true;
        const value = this.expr();
        entries.push({
          kind: "Entry", key: { kind: "expr", expr: k }, value, shorthand: false,
          span: { start: t.start, end: this.outer(value).end },
        });
      } else if (t.kind === "word") {
        // A shorthand entry reads a value (f08 2.4 item 2).
        const n = this.nameToken("shorthand");
        const span = { start: n.start, end: n.end };
        entries.push({
          kind: "Entry", key: { kind: "word", text: n.text, span }, value: { kind: "Ident", name: n.text, span },
          shorthand: true, span,
        });
      } else {
        this.unexpected(t);
      }
      if (this.isP(this.cur, ",")) {
        label.sep = true;
        this.advance();
        continue;
      }
      if (this.isP(this.cur, "}")) break;
      this.unexpected(this.cur);
    }
    this.advance();
    this.close();
    return { kind: "Map", entries, span: this.span(lb.start) };
  }

  private matchExpr(): A.Match {
    const m = this.advance();
    this.open(m);
    const s = this.cur;
    if (this.isP(s, "{") || this.isW(s, "if") || this.isW(s, "try")) {
      // 01 3.4 rule 6.
      this.fail("needs-parentheses", s, "match-subject");
    }
    const subject = this.expr();
    this.close();
    const lb = this.expect("{");
    this.open(lb);
    const arms: A.Arm[] = [];
    for (;;) {
      arms.push(this.arm());
      if (this.isP(this.cur, ",")) {
        this.advance();
        if (this.isP(this.cur, "}")) break;
        continue;
      }
      if (this.isP(this.cur, "}")) break;
      this.unexpected(this.cur);
    }
    this.advance();
    this.close();
    return { kind: "Match", subject, arms, span: this.span(m.start) };
  }

  private arm(): A.Arm {
    const start = this.cur.start;
    const literals: A.Literal[] = [];
    let isDefault = false;
    if (this.isW(this.cur, "_")) {
      this.advance();
      isDefault = true;
    } else {
      literals.push(this.literal());
      while (this.isP(this.cur, "|")) {
        this.advance();
        literals.push(this.literal());
      }
    }
    this.expect("=>");
    const result = this.expr();
    return { kind: "Arm", literals, isDefault, result, span: { start, end: this.outer(result).end } };
  }

  /** Lit = [ "-" ] Number | String | "true" | "false" | "null". */
  private literal(): A.Literal {
    const t = this.cur;
    if (this.isP(t, "-")) {
      this.advance();
      const n = this.cur;
      if (n.kind !== "number") this.unexpected(n);
      this.advance();
      return { kind: "Number", value: -n.num! + 0, span: { start: t.start, end: n.end } };
    }
    if (t.kind === "number" || t.kind === "string") return this.primary() as A.Literal;
    if (t.kind === "word") {
      if (t.text === "true" || t.text === "false" || t.text === "null") return this.primary() as A.Literal;
      if (isReserved(t.text)) this.reservedWord(t, "operand");
      // f08 2.7 E15: the word `case` at the start of a match case.
      if (t.text === "case") this.fail("switch-statement", t);
    }
    this.unexpected(t);
  }

  // ---- lambdas and patterns ----------------------------------------------

  /** 01 3.2 rules 5 and 6. */
  private lambdaStartsHere(): boolean {
    const t = this.cur;
    if (t.kind === "word" && !isReserved(t.text)) return this.isP(this.peek(1), "=>");
    if (!this.isP(t, "(")) return false;
    let depth = 0;
    for (let j = this.i + 1; j < this.toks.length; j++) {
      const u = this.toks[j];
      if (u.kind === "eof" || u.kind === "interpEnd") return false;
      if (u.kind !== "punct") continue;
      if (OPENERS.has(u.text)) depth++;
      else if (CLOSERS.has(u.text)) {
        depth--;
        if (depth < 0) {
          if (u.text !== ")") return false;
          const after = this.toks[j + 1];
          return after !== undefined && this.isP(after, "=>");
        }
      }
    }
    return false;
  }

  private lambda(onArrow: ((arrow: Token) => void) | null): A.Lambda {
    const t = this.cur;
    const level = this.level;
    const outerMax = this.maxLevel;
    this.maxLevel = level;
    const params: A.Param[] = [];
    if (t.kind === "word") {
      this.advance();
      const span = { start: t.start, end: t.end };
      const pattern: A.NamePat = { kind: "NamePat", name: t.text === "_" ? null : t.text, span };
      params.push({ kind: "Param", pattern, default: null, span });
    } else {
      const lp = this.advance();
      this.open(lp, "params");
      for (;;) {
        if (this.isP(this.cur, ")")) break;
        params.push(this.param());
        // f08 2.7 E9: `:` after the pattern of a parameter.
        if (this.isP(this.cur, ":")) this.fail("type-annotation", this.cur);
        if (this.isP(this.cur, ",")) {
          this.advance();
          continue;
        }
        if (this.isP(this.cur, ")")) break;
        this.unexpected(this.cur);
      }
      this.advance();
      this.close();
    }
    const arrow = this.expect("=>");
    if (onArrow !== null) onArrow(arrow);
    const body = this.expr();
    const maxLevel = this.maxLevel;
    this.maxLevel = Math.max(outerMax, maxLevel);
    return { kind: "Lambda", params, body, level, maxLevel, span: { start: t.start, end: this.outer(body).end } };
  }

  private param(): A.Param {
    const pattern = this.pattern();
    let dflt: A.Expr | null = null;
    if (this.isP(this.cur, "=")) {
      this.advance();
      dflt = this.expr();
    }
    const end = dflt !== null ? this.outer(dflt).end : pattern.span.end;
    return { kind: "Param", pattern, default: dflt, span: { start: pattern.span.start, end } };
  }

  /** A token where the grammar needs a Name (01 14.2). */
  private nameToken(place: "name" | "shorthand" = "name"): Token {
    const t = this.cur;
    if (t.kind === "word") {
      if (isReserved(t.text)) this.reservedWord(t, place);
      if (t.text === "_") this.unexpected(t);
      return this.advance();
    }
    this.unexpected(t);
  }

  private pattern(): A.Pattern {
    const t = this.cur;
    if (t.kind === "word") {
      if (t.text === "_") {
        this.advance();
        return { kind: "NamePat", name: null, span: { start: t.start, end: t.end } };
      }
      const n = this.nameToken();
      return { kind: "NamePat", name: n.text, span: { start: n.start, end: n.end } };
    }
    if (this.isP(t, "{")) return this.mapPattern();
    if (this.isP(t, "[")) return this.listPattern();
    this.unexpected(t);
  }

  private rest(): { name: string; span: Span } {
    const d = this.advance(); // ...
    const n = this.nameToken();
    return { name: n.text, span: { start: d.start, end: n.end } };
  }

  private mapPattern(): A.MapPat {
    const lb = this.advance();
    this.open(lb, "mappat");
    const fields: A.MapPatField[] = [];
    let rest: { name: string; span: Span } | null = null;
    for (;;) {
      const t = this.cur;
      if (this.isP(t, "}")) break;
      if (this.isP(t, "...")) {
        rest = this.rest();
        if (this.isP(this.cur, ",")) this.advance();
        if (!this.isP(this.cur, "}")) this.unexpected(this.cur);
        break;
      }
      let key: string;
      let n: Token;
      if (t.kind === "word" && this.isP(this.peek(1), ":")) {
        this.advance();
        this.advance();
        key = t.text;
        n = this.nameToken();
      } else if (t.kind === "string") {
        this.advance();
        this.expectKeyColon();
        key = t.str!;
        n = this.nameToken();
      } else if (t.kind === "word") {
        n = this.nameToken();
        key = n.text;
      } else {
        this.unexpected(t);
      }
      let dflt: A.Expr | null = null;
      if (this.isP(this.cur, "=")) {
        this.advance();
        dflt = this.expr();
      }
      const end = dflt !== null ? this.outer(dflt).end : n.end;
      fields.push({ key, name: n.text, nameSpan: { start: n.start, end: n.end }, keySpan: { start: t.start, end: t.end }, default: dflt, span: { start: t.start, end } });
      if (this.isP(this.cur, ",")) {
        this.advance();
        continue;
      }
      if (this.isP(this.cur, "}")) break;
      this.unexpected(this.cur);
    }
    this.advance();
    this.close();
    return { kind: "MapPat", fields, rest, span: this.span(lb.start) };
  }

  private listPattern(): A.ListPat {
    const lb = this.advance();
    this.open(lb);
    const elems: A.ListPatElem[] = [];
    let rest: { name: string; span: Span } | null = null;
    for (;;) {
      const t = this.cur;
      if (this.isP(t, "]")) break;
      if (this.isP(t, "...")) {
        rest = this.rest();
        if (this.isP(this.cur, ",")) this.advance();
        if (!this.isP(this.cur, "]")) this.unexpected(this.cur);
        break;
      }
      let pat: A.NamePat;
      if (this.isW(t, "_")) {
        this.advance();
        pat = { kind: "NamePat", name: null, span: { start: t.start, end: t.end } };
      } else {
        const n = this.nameToken();
        pat = { kind: "NamePat", name: n.text, span: { start: n.start, end: n.end } };
      }
      let dflt: A.Expr | null = null;
      if (this.isP(this.cur, "=")) {
        this.advance();
        dflt = this.expr();
      }
      const end = dflt !== null ? this.outer(dflt).end : t.end;
      elems.push({ pat, default: dflt, span: { start: t.start, end } });
      if (this.isP(this.cur, ",")) {
        this.advance();
        continue;
      }
      if (this.isP(this.cur, "]")) break;
      this.unexpected(this.cur);
    }
    this.advance();
    this.close();
    return { kind: "ListPat", elems, rest, span: this.span(lb.start) };
  }
}
