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
export function parseProgram(source: Source): A.Program {
  const tokens = lex(source);
  return new Parser(tokens).program();
}

const OPENERS = new Set(["(", "[", "?.[", "{"]);
const CLOSERS = new Set([")", "]", "}"]);

const PREC_OR = 3;
const PREC_REL = 6;
const PREC_ADD = 7;

class Parser {
  private toks: Token[];
  private i = 0;
  private level = 0;
  private prevEnd = 0;
  /** The outer span of an expression written in parentheses. */
  private readonly parens = new WeakMap<object, Span>();

  constructor(tokens: Token[]) {
    this.toks = tokens;
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

  // ---- diagnostics --------------------------------------------------------

  private fail(code: string, span: Span, message: string): never {
    throw new ParseError(code, { start: span.start, end: span.end }, message);
  }

  /** `unexpected-token` at a token that does not fit (01 3.5). */
  private unexpected(t: Token): never {
    if (t.kind === "reject") {
      this.fail("unexpected-token", t.report ?? t, `${t.text} is not allowed here`);
    }
    if (t.kind === "string") {
      // 01 14.3: the word `r` or `R`, white space, then a quote.
      const prev = this.i > 0 ? this.toks[this.i - 1] : undefined;
      if (prev !== undefined && prev.kind === "word" && (prev.text === "r" || prev.text === "R") && t.gap && this.toks[this.i] === t) {
        this.fail("unexpected-token", prev, "a prefix before a string literal is not allowed");
      }
    }
    if (t.kind === "eof") this.fail("unexpected-token", t, "unexpected end of input");
    if (t.kind === "word" && t.text === "not" && this.toks[this.i] === t && this.isW(this.peek(1), "in")) {
      // Ruling R37: an operator `not in` that cannot follow spans both words.
      this.fail("unexpected-token", { start: t.start, end: this.peek(1).end }, "unexpected `not in`");
    }
    const what = t.kind === "word" || t.kind === "punct" ? `\`${t.text}\`` : t.kind === "interpEnd" ? "`}`" : `a ${t.kind}`;
    this.fail("unexpected-token", t, `unexpected ${what}`);
  }

  private reservedWord(t: Token): never {
    this.fail("reserved-word", t, `\`${t.text}\` is a reserved word`);
  }

  /** Open a nesting level at a token (01 13.1). */
  private open(span: Span): void {
    if (this.level >= MAX_LEVELS) {
      this.fail("nesting-exceeded", span, `more than ${MAX_LEVELS} levels of nesting`);
    }
    this.level++;
  }

  private close(): void {
    this.level--;
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
    this.expect("=");
    let value: A.Expr | A.Lambda;
    if (this.lambdaStartsHere()) {
      value = this.lambda((arrow) => {
        // 01 3.4 rule 9: only a Name may bind a lambda.
        if (!(pattern.kind === "NamePat" && pattern.name !== null)) {
          this.fail("unexpected-token", arrow, "a lambda may be bound only to a name");
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
      this.fail("unexpected-token", lb, "a branch is written like a map");
    }
    if (this.isP(this.cur, "}")) {
      this.fail("unexpected-token", lb, "a branch may not be empty");
    }
    const consts = this.consts();
    const result = this.expr();
    if (this.isP(this.cur, ":") || this.isP(this.cur, ",")) {
      this.fail("unexpected-token", lb, "a branch is written like a map");
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
        break;
      }
      if (this.isW(t, "try")) {
        e = this.tryExpr();
        break;
      }
      const b = this.binary();
      if (this.isP(this.cur, "?")) {
        const q = this.advance();
        this.open(q);
        const then = this.expr();
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
    for (;;) {
      const lp = this.expect("(");
      this.open(lp);
      const test = this.expr();
      this.expect(")");
      this.close();
      const then = this.branch();
      parts.push({ start: ifTok.start, test, then });
      if (!this.isW(this.cur, "else")) this.unexpected(this.cur); // 01 3.4 rule 11
      this.advance();
      if (this.isW(this.cur, "if")) {
        ifTok = this.advance();
        continue;
      }
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
      this.fail("unexpected-token", tryTok, "`try` takes a branch, not arguments"); // 01 3.4 rule 10
    }
    const body = this.branch();
    if (!this.isW(this.cur, "catch")) this.unexpected(this.cur); // rule 12
    this.advance();
    let name: string | null = null;
    let nameSpan: Span | null = null;
    if (this.isP(this.cur, "(")) {
      const lp = this.advance();
      this.open(lp);
      const t = this.nameToken();
      name = t.text;
      nameSpan = { start: t.start, end: t.end };
      this.expect(")");
      this.close();
    }
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
      return x;
    }
    return this.climb(left, PREC_OR);
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
        // 01 14.3: `x as T` at the word; `!x as T` at the `!`.
        if (this.isBareNot(left)) this.fail("unexpected-token", { start: left.span.start, end: left.span.start + 1 }, "write the `!` expression in parentheses");
        this.fail("unexpected-token", t, `\`${t.text}\` is not in Core`);
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
        this.fail("unexpected-token", { start: left.span.start, end: left.span.start + 1 }, "write the `!` expression in parentheses, or use `not in`");
      }
      this.advance();
      if (op === "not-in") this.advance();
      this.open(opSpan);
      const right = this.climb(this.unary(null), p + 1);
      this.close();
      left = this.binaryNode(op, left, right);
    }
  }

  /** Unary = ( "!" | "-" ) Unary | Power. */
  private unary(prefix: Token | null): A.Expr {
    const t = this.cur;
    if (this.isP(t, "!") || this.isP(t, "-")) {
      this.advance();
      this.open(t);
      // Ruling R37: the span of 3.4 rule 1 is the first prefix operator.
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
        this.fail("unexpected-token", prefix, `write \`${prefix.text}\` and \`**\` with parentheses`);
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
      if (this.isP(t, "[") || this.isP(t, "?.[")) {
        const opt = t.text === "?.[";
        this.advance();
        this.open(t);
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
    return x;
  }

  private primary(): A.Expr {
    const t = this.cur;
    switch (t.kind) {
      case "number":
        this.advance();
        return { kind: "Number", value: t.num!, span: { start: t.start, end: t.end } };
      case "string":
        this.advance();
        return { kind: "String", value: t.str!, form: t.quote!, span: { start: t.start, end: t.end } };
      case "template":
        return this.template();
      case "word":
        return this.wordPrimary(t);
      case "punct":
        switch (t.text) {
          case "(": {
            this.advance();
            this.open(t);
            const e = this.expr();
            this.expect(")");
            this.close();
            this.parens.set(e, this.span(t.start));
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
        // 01 3.4 rule 5: an if or a try is not an operand.
        this.fail("unexpected-token", t, `write the \`${t.text}\` in parentheses`);
    }
    if (isReserved(t.text)) this.reservedWord(t);
    if (t.text === "_") this.unexpected(t);
    const level = this.level;
    this.advance();
    if (this.isP(this.cur, "(")) {
      const args = this.args();
      return { kind: "Call", namespace: null, name: t.text, nameSpan: { start: t.start, end: t.end }, args, level, span: this.span(t.start) };
    }
    return { kind: "Ident", name: t.text, span: { start: t.start, end: t.end } };
  }

  private template(): A.Expr {
    const t = this.advance();
    const parts = t.parts!;
    const span = { start: t.start, end: t.end };
    if (!parts.some((p) => p.kind === "interp")) {
      const value = parts.map((p) => (p.kind === "text" ? p.value : "")).join("");
      return { kind: "String", value, form: "`", span };
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
    return { kind: "Template", parts: out, form: "`", span };
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
    const lb = this.advance();
    this.open(lb);
    const entries: (A.Entry | A.Spread)[] = [];
    for (;;) {
      const t = this.cur;
      if (this.isP(t, "}")) break;
      if (this.isP(t, "...")) {
        this.advance();
        const value = this.expr();
        entries.push({ kind: "Spread", value, span: { start: t.start, end: this.outer(value).end } });
      } else if (t.kind === "word" && this.isP(this.peek(1), ":")) {
        this.advance();
        this.advance();
        const value = this.expr();
        entries.push({
          kind: "Entry", key: { kind: "word", text: t.text, span: { start: t.start, end: t.end } },
          value, shorthand: false, span: { start: t.start, end: this.outer(value).end },
        });
      } else if (t.kind === "string") {
        this.advance();
        this.expect(":");
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
        this.expect(":");
        const value = this.expr();
        entries.push({
          kind: "Entry", key: { kind: "expr", expr: k }, value, shorthand: false,
          span: { start: t.start, end: this.outer(value).end },
        });
      } else if (t.kind === "word") {
        const n = this.nameToken();
        const span = { start: n.start, end: n.end };
        entries.push({
          kind: "Entry", key: { kind: "word", text: n.text, span }, value: { kind: "Ident", name: n.text, span },
          shorthand: true, span,
        });
      } else {
        this.unexpected(t);
      }
      if (this.isP(this.cur, ",")) {
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
      this.fail("unexpected-token", s, "a `match` needs a subject; write it in parentheses");
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
      if (isReserved(t.text)) this.reservedWord(t);
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
    const params: A.Param[] = [];
    if (t.kind === "word") {
      this.advance();
      const span = { start: t.start, end: t.end };
      const pattern: A.NamePat = { kind: "NamePat", name: t.text === "_" ? null : t.text, span };
      params.push({ kind: "Param", pattern, default: null, span });
    } else {
      const lp = this.advance();
      this.open(lp);
      for (;;) {
        if (this.isP(this.cur, ")")) break;
        params.push(this.param());
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
    return { kind: "Lambda", params, body, level, span: { start: t.start, end: this.outer(body).end } };
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
  private nameToken(): Token {
    const t = this.cur;
    if (t.kind === "word") {
      if (isReserved(t.text)) this.reservedWord(t);
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
    this.open(lb);
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
        this.expect(":");
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
      fields.push({ key, name: n.text, nameSpan: { start: n.start, end: n.end }, default: dflt, span: { start: t.start, end } });
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
