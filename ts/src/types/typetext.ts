// Type text (05 3.1, 3.2): a parser that yields a tree.
//
// Every implementation parses type text, with or without a type checker
// (ruling A2): it needs the names of declared variables and the count of
// parameters of each declared function. Text that does not fit the grammar
// is an error; the caller turns it into `bad-type` or `bad-signature`.
// Rules beyond the grammar (two records, an undeclared name, type depth, a key
// written twice) belong to the type checker and are not tested here.

import { ParseError } from "../syntax/diagnostic.ts";
import { lex } from "../syntax/lexer.ts";
import { Source } from "../syntax/source.ts";
import type { Token } from "../syntax/token.ts";

export type TypeWord = "null" | "bool" | "int" | "float" | "string" | "any" | "never";

export type TypeNode =
  | { t: "word"; word: TypeWord }
  | { t: "literal"; value: string }
  | { t: "map"; rest: TypeNode }
  | { t: "record"; fields: RecordField[]; rest: RecordRest }
  | { t: "name"; name: string }
  | { t: "var"; name: string }
  | { t: "list"; elem: TypeNode }
  | { t: "union"; alts: TypeNode[] };

export interface RecordField {
  key: string;
  optional: boolean;
  type: TypeNode;
}

/** `exact`: no Rest. `any`: `...` alone. Otherwise `...T`. */
export type RecordRest = "exact" | "any" | TypeNode;

export interface FnNode {
  t: "fn";
  params: { name: string; type: TypeNode }[];
  result: TypeNode | "unknown";
}

export interface SigParam {
  name: string;
  optional: boolean;
  rest: boolean;
  type: TypeNode | FnNode;
}

export interface SigNode {
  params: SigParam[];
  result: TypeNode;
  canFail: boolean;
  bounds: { name: string; bound: "ordered" | "number" }[];
}

export interface MethodNode {
  recv: TypeNode;
  name: string;
  sig: SigNode;
}

export type TypeTextResult<T> = { ok: true; node: T } | { ok: false; message: string };

/** The bracket nesting limit of type text (05 20.5). */
export const MAX_TYPE_BRACKETS = 64;

const TYPE_WORDS: ReadonlySet<string> = new Set(["null", "bool", "int", "float", "string", "any", "never"]);

class TypeTextError extends Error {}

/** Parse a `Type` (05 3.1). Type variables are not allowed. */
export function parseType(text: string): TypeTextResult<TypeNode> {
  return run(text, (p) => p.type(), false);
}

/** Parse a `Sig` (05 3.1). */
export function parseSignature(text: string): TypeTextResult<SigNode> {
  return run(text, (p) => p.sig(), true);
}

/** Parse a `Method` (05 3.1): `Recv "." Word ":" Sig`. */
export function parseMethod(text: string): TypeTextResult<MethodNode> {
  return run(text, (p) => p.method(), true);
}

/** Parse a `Recv` (05 3.1), which is an `Alt`. */
export function parseReceiver(text: string): TypeTextResult<TypeNode> {
  return run(text, (p) => p.alt(), true);
}

function run<T>(text: string, f: (p: TypeParser) => T, vars: boolean): TypeTextResult<T> {
  let tokens: Token[];
  try {
    tokens = lex(new Source(text));
  } catch (e) {
    if (e instanceof ParseError) return { ok: false, message: e.diagnostic.message };
    throw e;
  }
  const depth = textDepth(tokens);
  if (depth > MAX_TYPE_BRACKETS) return { ok: false, message: `type text deeper than ${MAX_TYPE_BRACKETS}` };
  const p = new TypeParser(tokens, vars);
  try {
    const node = f(p);
    p.end();
    return { ok: true, node };
  } catch (e) {
    if (e instanceof TypeTextError) return { ok: false, message: e.message };
    throw e;
  }
}

/**
 * The depth of type text (05 20.5, ruling AA3): a `[]` suffix is 1 deeper
 * than what it follows; a bracketed form is 1 deeper than its contents.
 */
function textDepth(toks: Token[]): number {
  const stack: number[] = [0];
  let last = 0; // the depth of the primary just read
  for (let i = 0; i < toks.length; i++) {
    const t = toks[i];
    if (t.kind !== "punct") {
      last = 0;
      continue;
    }
    switch (t.text) {
      case "[":
        if (i + 1 < toks.length && toks[i + 1].kind === "punct" && toks[i + 1].text === "]") {
          last += 1;
          stack[stack.length - 1] = Math.max(stack[stack.length - 1], last);
          i++;
          continue;
        }
        stack.push(0);
        continue;
      case "(":
      case "{":
      case "<":
        stack.push(0);
        continue;
      case ")":
      case "}":
      case ">":
      case "]":
        if (stack.length > 1) {
          last = 1 + stack.pop()!;
          stack[stack.length - 1] = Math.max(stack[stack.length - 1], last);
        }
        continue;
      default:
        last = 0;
    }
  }
  return stack[0];
}

/** Parse the record of an environment (03 3.10.3, rulings T4, AG4): a `Record` with no optional field and no rest. */
export function parseEnvRecord(text: string): TypeTextResult<TypeNode> {
  const r = run(text, (p) => p.envRecord(), false);
  if (!r.ok) return r;
  const n = r.node as Extract<TypeNode, { t: "record" }>;
  if (n.rest !== "exact") return { ok: false, message: "the record of an environment has no rest" };
  if (n.fields.some((f) => f.optional)) return { ok: false, message: "the record of an environment has no optional field" };
  return r;
}

function isUpper(c: string): boolean {
  return c >= "A" && c <= "Z";
}

class TypeParser {
  private i = 0;
  private depth = 0;

  private readonly toks: Token[];
  private readonly vars: boolean;

  constructor(toks: Token[], vars: boolean) {
    this.toks = toks;
    this.vars = vars;
  }

  private get cur(): Token {
    return this.toks[this.i];
  }

  private peek(k: number): Token {
    return this.toks[Math.min(this.i + k, this.toks.length - 1)];
  }

  private isP(t: Token, s: string): boolean {
    return t.kind === "punct" && t.text === s;
  }

  private isW(t: Token, s: string): boolean {
    return t.kind === "word" && t.text === s;
  }

  private advance(): Token {
    const t = this.cur;
    if (t.kind !== "eof") this.i++;
    return t;
  }

  private fail(t: Token): never {
    const what = t.kind === "eof" ? "the end of the text" : t.kind === "word" || t.kind === "punct" ? `\`${t.text}\`` : `a ${t.kind}`;
    throw new TypeTextError(`unexpected ${what} in type text`);
  }

  private expect(s: string): Token {
    if (!this.isP(this.cur, s)) this.fail(this.cur);
    return this.advance();
  }

  private open(): void {
    this.depth++;
    if (this.depth > MAX_TYPE_BRACKETS) {
      throw new TypeTextError(`type text nests more than ${MAX_TYPE_BRACKETS} brackets`);
    }
  }

  private close(): void {
    this.depth--;
  }

  end(): void {
    if (this.cur.kind !== "eof") this.fail(this.cur);
  }

  envRecord(): TypeNode {
    if (!this.isP(this.cur, "{")) this.fail(this.cur);
    return this.record();
  }

  /** Type = Alt { "|" Alt }. */
  type(): TypeNode {
    const alts = [this.alt()];
    while (this.isP(this.cur, "|")) {
      this.advance();
      alts.push(this.alt());
    }
    return alts.length === 1 ? alts[0] : { t: "union", alts };
  }

  /** Alt = Prim { "[" "]" }. */
  alt(): TypeNode {
    let n = this.prim();
    while (this.isP(this.cur, "[")) {
      this.advance();
      this.open();
      this.expect("]");
      this.close();
      n = { t: "list", elem: n };
    }
    return n;
  }

  private prim(): TypeNode {
    const t = this.cur;
    if (t.kind === "string") {
      this.advance();
      return { t: "literal", value: t.str! };
    }
    if (t.kind === "word") {
      const w = t.text;
      if (TYPE_WORDS.has(w)) {
        this.advance();
        return { t: "word", word: w as TypeWord };
      }
      if (w === "map") {
        this.advance();
        this.expect("<");
        this.open();
        const rest = this.type();
        this.expect(">");
        this.close();
        return { t: "map", rest };
      }
      if (isUpper(w[0])) {
        this.advance();
        if (w.length === 1) {
          if (!this.vars) throw new TypeTextError(`the type variable ${w} may stand only in a signature`);
          return { t: "var", name: w };
        }
        return { t: "name", name: w };
      }
      this.fail(t);
    }
    if (this.isP(t, "{")) return this.record();
    if (this.isP(t, "(")) {
      this.advance();
      this.open();
      const n = this.type();
      this.expect(")");
      this.close();
      return n;
    }
    this.fail(t);
  }

  /** Record = "{" [ ( Field { "," Field } [ "," Rest ] | Rest ) [ "," ] ] "}". */
  record(): TypeNode {
    this.advance();
    this.open();
    const fields: RecordField[] = [];
    let rest: RecordRest = "exact";
    for (;;) {
      const t = this.cur;
      if (this.isP(t, "}")) break;
      if (this.isP(t, "...")) {
        this.advance();
        rest = this.isP(this.cur, ",") || this.isP(this.cur, "}") ? "any" : this.type();
        if (this.isP(this.cur, ",")) this.advance();
        if (!this.isP(this.cur, "}")) this.fail(this.cur);
        break;
      }
      let key: string;
      if (t.kind === "word") key = t.text;
      else if (t.kind === "string") key = t.str!;
      else this.fail(t);
      this.advance();
      let optional = false;
      if (this.isP(this.cur, "?")) {
        this.advance();
        optional = true;
      }
      this.expect(":");
      fields.push({ key, optional, type: this.type() });
      if (this.isP(this.cur, ",")) {
        this.advance();
        continue;
      }
      if (this.isP(this.cur, "}")) break;
      this.fail(this.cur);
    }
    this.advance();
    this.close();
    return { t: "record", fields, rest };
  }

  /** Sig = "(" [ Param { "," Param } ] ")" "=>" Type [ "!" ] [ "where" ... ]. */
  sig(): SigNode {
    this.expect("(");
    this.open();
    const params: SigParam[] = [];
    if (!this.isP(this.cur, ")")) {
      params.push(this.param());
      while (this.isP(this.cur, ",")) {
        this.advance();
        params.push(this.param());
      }
    }
    this.expect(")");
    this.close();
    // 05 3.6: every optional parameter follows every required one; a rest
    // parameter is the last, and its type is a list.
    let seenOptional = false;
    params.forEach((p, i) => {
      if (p.rest) {
        if (i !== params.length - 1 || p.optional) throw new TypeTextError("a rest parameter must be the last");
        if (p.type.t !== "list") throw new TypeTextError("the type of a rest parameter must be a list");
      } else if (p.optional) seenOptional = true;
      else if (seenOptional) throw new TypeTextError("an optional parameter must follow every required one");
    });
    this.expect("=>");
    const result = this.type();
    let canFail = false;
    if (this.isP(this.cur, "!")) {
      this.advance();
      canFail = true;
    }
    const bounds: SigNode["bounds"] = [];
    if (this.isW(this.cur, "where")) {
      this.advance();
      for (;;) {
        const v = this.cur;
        if (!(v.kind === "word" && v.text.length === 1 && isUpper(v.text))) this.fail(v);
        this.advance();
        this.expect(":");
        const b = this.cur;
        if (!(this.isW(b, "ordered") || this.isW(b, "number"))) this.fail(b);
        this.advance();
        bounds.push({ name: v.text, bound: b.text as "ordered" | "number" });
        if (!this.isP(this.cur, ",")) break;
        this.advance();
      }
    }
    return { params, result, canFail, bounds };
  }

  /** Param = [ "..." ] Word [ "?" ] ":" ( Fn | Type ). */
  private param(): SigParam {
    let rest = false;
    if (this.isP(this.cur, "...")) {
      this.advance();
      rest = true;
    }
    const w = this.cur;
    if (w.kind !== "word") this.fail(w);
    this.advance();
    let optional = false;
    if (this.isP(this.cur, "?")) {
      this.advance();
      optional = true;
    }
    this.expect(":");
    // 05 3.2: `(` then `)`, or `(` then a Word then `:`, starts an Fn.
    const isFn =
      this.isP(this.cur, "(") &&
      (this.isP(this.peek(1), ")") || (this.peek(1).kind === "word" && this.isP(this.peek(2), ":")));
    const type = isFn ? this.fn() : this.type();
    return { name: w.text, optional, rest, type };
  }

  /** Fn = "(" [ Word ":" Type { "," Word ":" Type } ] ")" "=>" ( Type | "unknown" ). */
  private fn(): FnNode {
    this.advance();
    this.open();
    const params: FnNode["params"] = [];
    if (!this.isP(this.cur, ")")) {
      for (;;) {
        const w = this.cur;
        if (w.kind !== "word") this.fail(w);
        this.advance();
        this.expect(":");
        params.push({ name: w.text, type: this.type() });
        if (!this.isP(this.cur, ",")) break;
        this.advance();
      }
    }
    this.expect(")");
    this.close();
    this.expect("=>");
    if (this.isW(this.cur, "unknown")) {
      this.advance();
      return { t: "fn", params, result: "unknown" };
    }
    return { t: "fn", params, result: this.type() };
  }

  /** Method = Recv "." Word ":" Sig. */
  method(): MethodNode {
    const recv = this.alt();
    this.expect(".");
    const w = this.cur;
    if (w.kind !== "word") this.fail(w);
    this.advance();
    this.expect(":");
    return { recv, name: w.text, sig: this.sig() };
  }
}

/** The count of parameters of a signature: required, and the most (Infinity with a rest). */
export function arity(sig: SigNode): { min: number; max: number } {
  let min = 0;
  let max = 0;
  for (const p of sig.params) {
    if (p.rest) {
      max = Infinity;
      continue;
    }
    if (!p.optional) min++;
    max++;
  }
  return { min, max };
}
