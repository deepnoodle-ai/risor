// The Core checker: the type rules of 05-types.md sections 5 to 18.
//
// It runs after the static rules (05 22, src/eval/static.ts), whose
// resolution says what every name and call refers to. Each value binding has
// a slot, and the checker keeps a type for each slot. A fact {p} is a path
// key in a set. Long flat forms (a chain of binary operators, a chain of
// steps, an `else` chain, `const` lines) are checked by loops (20.6).

import type * as A from "../syntax/ast.ts";
import type { Diagnostic } from "../syntax/diagnostic.ts";
import type { Span } from "../syntax/source.ts";
import type { Resolution } from "../eval/static.ts";
import type { SigNode, TypeNode } from "./typetext.ts";
import {
  ANY, BOOL, FLOAT, INT, NEVER, NULL, STRING, alternatives, depth, falsyPart, finiteValues, fits, fromNode, hasNull,
  hasOpaque, isAny, isNever, join, joinAll, kindCount, lit, listOf, merge, nonnull, overlap, record, recordValues,
  truthyPart, withNull, mapOf, opaque, hasAny,
} from "./type.ts";
import type { Alts, Fit, Rec, Type } from "./type.ts";
import { CATALOG } from "../generated/messages.ts";

/** A signature the checker calls (13.1). */
export interface CheckSig {
  name: string;
  kind: "library" | "host";
  /** The receiver type of a method, or null for a function. */
  recv: TypeNode | null;
  sig: SigNode;
  /** `filter` and `find` (13.8). */
  narrows: boolean;
  /** Tested argument positions (10.1). */
  tested: number[];
  /** Rendered argument positions (ruling AO2). */
  renders: number[];
}

export interface CheckEnv {
  opaque: ReadonlySet<string>;
  variables: ReadonlyMap<string, Type>;
  /** Host functions by full name. */
  functions: ReadonlyMap<string, CheckSig>;
  /** Library functions by name. */
  library: ReadonlyMap<string, CheckSig>;
  /** Library methods: kind (`string`, `list`, `map`), then name. */
  libraryMethods: ReadonlyMap<string, ReadonlyMap<string, CheckSig>>;
  /** Methods of opaque types: type name, then method name. */
  methods: ReadonlyMap<string, ReadonlyMap<string, CheckSig>>;
  /** The rejected check classes (17). */
  reject: ReadonlySet<string>;
}

export interface Site {
  class: string;
  span: Span;
}

export interface CheckResult {
  /** The program's type; meaningful only when there is no diagnostic. */
  type: Type;
  sites: Site[];
  diagnostics: Diagnostic[];
  /** Spans of nodes at a `dynamic` site, for the class of a run failure (18.3). */
  dynamicSpans: Set<string>;
}

export type Expected = { kind: "none" } | { kind: "type"; type: Type } | { kind: "condition" };

const SEP = "\u0000";
type Facts = ReadonlySet<string>;
const NO_FACTS: Facts = new Set();

export function spanKey(s: Span): string {
  return `${s.start}:${s.end}`;
}

/** The type of the error value (14.2). */
export function errorValueType(): Type {
  const codes = CATALOG.filter((e) => e.phase === "run" && e.caught === true).map((e) => e.code);
  return record([
    ["code", { type: lit(...codes), optional: false }],
    ["data", { type: ANY, optional: false }],
    ["message", { type: STRING, optional: false }],
    ["path", { type: { list: { num: "int", str: "string" }, null: true }, optional: false }],
  ]);
}

const ERROR_TYPE = errorValueType();

interface DiagRec {
  code: string;
  span: Span;
  class?: string;
}

interface SiteRec {
  class: string;
  span: Span;
  owners: Span[];
}

interface Mark {
  d: number;
  s: number;
}

export function check(program: A.Program, res: Resolution, env: CheckEnv, expected: Expected, staticDiags: Diagnostic[]): CheckResult {
  const c = new Checker(res, env);
  const t = c.program(program, expected);
  const diags: Diagnostic[] = [...staticDiags];
  const seen = new Set(staticDiags.map((d) => `${d.code}:${d.span!.start}:${d.span!.end}`));
  for (const d of c.diags) {
    const k = `${d.code}:${d.span.start}:${d.span.end}`;
    if (seen.has(k)) continue;
    seen.add(k);
    const out: Diagnostic = { phase: "check", code: d.code, span: d.span, message: messageFor(d.code) };
    if (d.class !== undefined) out.class = d.class;
    diags.push(out);
  }
  diags.sort((a, b) => a.span!.start - b.span!.start || a.span!.end - b.span!.end || cmp(a.code, b.code));
  const siteSeen = new Set<string>();
  const sites: Site[] = [];
  const dynamicSpans = new Set<string>();
  for (const s of c.sites) {
    if (s.class === "dynamic") {
      dynamicSpans.add(spanKey(s.span));
      for (const o of s.owners) dynamicSpans.add(spanKey(o));
    }
    const k = `${s.class}:${s.span.start}:${s.span.end}`;
    if (siteSeen.has(k)) continue;
    siteSeen.add(k);
    sites.push({ class: s.class, span: s.span });
  }
  sites.sort((a, b) => a.span.start - b.span.start || a.span.end - b.span.end || cmp(a.class, b.class));
  return { type: t, sites, diagnostics: diags, dynamicSpans };
}

function cmp(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

function sp(s: Span): Span {
  return { start: s.start, end: s.end };
}

function spanOf(a: Span, b: Span): Span {
  return { start: a.start, end: b.end };
}

/** A lambda argument: a Lambda node, or the name of a const-bound lambda. */
type LambdaArg = { lambda: A.Lambda; id: number | null; span: Span };

class Checker {
  readonly diags: DiagRec[] = [];
  readonly sites: SiteRec[] = [];
  private readonly res: Resolution;
  private readonly env: CheckEnv;
  private readonly slotTypes: Type[] = [];
  private readonly types = new Map<object, Type>();
  private readonly factsMemo = new Map<object, { t: string[]; f: string[] }>();
  /** Sites are dropped while this is above 0 (14.1, 13.9). */
  private drop = 0;
  /** The facts in force where each const-bound lambda was written. */
  private readonly lambdaFacts = new Map<number, Facts>();
  private readonly used = new Set<number>();
  private uses = 0;
  /** expanded nesting of a node = node.level + offset (20.2). */
  private offset = 0;

  constructor(res: Resolution, env: CheckEnv) {
    this.res = res;
    this.env = env;
    for (const r of res.refs.values()) if (r.k === "lambda") this.used.add(r.id);
    for (const t of res.targets.values()) if (t.k === "lambda") this.used.add(t.id);
  }

  // ---- diagnostics and sites -----------------------------------------------

  private diag(code: string, span: Span, cls?: string): void {
    const d: DiagRec = { code, span: sp(span) };
    if (cls !== undefined) d.class = cls;
    this.diags.push(d);
  }

  /** Record a site. For a rejected `dynamic` site, returns true: the node takes the type never (17). */
  private site(cls: string, span: Span, ...owners: Span[]): boolean {
    if (this.drop > 0) return false;
    if (this.env.reject.has(cls)) {
      if (cls === "dynamic") {
        this.diag("unchecked", span, "dynamic");
        return true;
      }
      this.diag("may-fail", span, cls);
      return false;
    }
    this.sites.push({ class: cls, span: sp(span), owners: owners.map(sp) });
    return false;
  }

  private mark(): Mark {
    return { d: this.diags.length, s: this.sites.length };
  }

  private rollback(m: Mark): void {
    this.diags.length = m.d;
    this.sites.length = m.s;
  }

  /** A tested position (10.1, 10.2). */
  private tested(t: Type, span: Span): void {
    if (!this.env.reject.has("constant-condition")) return;
    if (isAny(t) || isNever(t)) return;
    if (isNever(falsyPart(t))) this.diag("always-truthy", span, "constant-condition");
    else if (isNever(truthyPart(t))) this.diag("always-falsy", span, "constant-condition");
  }

  // ---- paths and facts (10.3 to 10.5) ---------------------------------------

  private pathOf(e: A.Expr): string | null {
    let steps: string[] = [];
    let x: A.Expr = e;
    for (;;) {
      if (x.kind === "OptChain") x = x.x;
      else if (x.kind === "Member") {
        steps.push(x.name);
        x = x.x;
      } else if (x.kind === "Index" && x.index.kind === "String") {
        steps.push(x.index.value);
        x = x.x;
      } else break;
    }
    if (x.kind !== "Ident") return null;
    const r = this.res.refs.get(x);
    if (r === undefined || r.k === "lambda") return null;
    const root = r.k === "slot" ? `s${r.slot}` : `v${r.name}`;
    steps = steps.reverse();
    return [root, ...steps].join(SEP);
  }

  private withFacts(f: Facts, keys: string[]): Facts {
    if (keys.length === 0) return f;
    const out = new Set(f);
    for (const k of keys) {
      const parts = k.split(SEP);
      for (let i = 1; i <= parts.length; i++) out.add(parts.slice(0, i).join(SEP));
    }
    return out;
  }

  /** T(c) and F(c) (10.5), as path keys. */
  private facts(e: A.Expr): { t: string[]; f: string[] } {
    const memo = this.factsMemo.get(e);
    if (memo !== undefined) return memo;
    let out: { t: string[]; f: string[] } = { t: [], f: [] };
    const p = this.pathOf(e);
    if (p !== null) out = { t: [p], f: [] };
    else if (e.kind === "Unary" && e.op === "!") {
      const d = this.facts(e.x);
      out = { t: d.f, f: d.t };
    } else if (e.kind === "Binary") {
      if (e.op === "==" || e.op === "!=") {
        const q = e.y.kind === "Null" ? this.pathOf(e.x) : e.x.kind === "Null" ? this.pathOf(e.y) : null;
        if (q !== null) out = e.op === "!=" ? { t: [q], f: [] } : { t: [], f: [q] };
      } else if (e.op === "&&" || e.op === "||") {
        // A long chain of && and || is a left spine: walk it by a loop.
        const spine: A.Binary[] = [];
        let x: A.Expr = e;
        while (x.kind === "Binary" && (x.op === "&&" || x.op === "||") && !this.factsMemo.has(x)) {
          spine.push(x);
          x = x.x;
        }
        let acc = this.facts(x);
        for (let i = spine.length - 1; i >= 0; i--) {
          const n = spine[i];
          const d = this.facts(n.y);
          acc =
            n.op === "&&"
              ? { t: union(acc.t, d.t), f: intersect(acc.f, d.f) }
              : { t: intersect(acc.t, d.t), f: union(acc.f, d.f) };
          this.factsMemo.set(n, acc);
        }
        out = acc;
      }
    }
    this.factsMemo.set(e, out);
    return out;
  }

  // ---- program, blocks, consts ----------------------------------------------

  program(p: A.Program, expected: Expected): Type {
    let f: Facts = NO_FACTS;
    for (const c of p.consts) f = this.constLine(c, f);
    const t = this.synth(p.result, f);
    if (expected.kind === "condition") this.tested(t, p.result.span);
    else if (expected.kind === "type") this.fitForm(p.result, expected.type, () => "result-mismatch");
    return t;
  }

  private constLine(c: A.Const, f: Facts): Facts {
    if (c.value.kind === "Lambda") {
      const id = this.res.lambdas.indexOf(c.value);
      this.lambdaFacts.set(id, f);
      if (!this.used.has(id)) {
        // No use: checked once, parameters `any`, sites dropped (13.9).
        this.drop++;
        this.lambdaBody(c.value, c.value.params.map(() => ({ given: true, type: ANY })), f);
        this.drop--;
      }
      return f;
    }
    const v = this.synth(c.value, f);
    this.bindPattern(c.pattern, v, c.value.span, f);
    return f;
  }

  private block(consts: A.Const[], result: A.Expr, f: Facts): Type {
    for (const c of consts) f = this.constLine(c, f);
    return this.synth(result, f);
  }

  // ---- patterns (12) ----------------------------------------------------------

  private setSlot(node: object, t: Type): void {
    const s = this.res.slots.get(node);
    if (s !== undefined && s >= 0) this.slotTypes[s] = t;
  }

  private neverNames(p: A.Pattern): void {
    if (p.kind === "NamePat") this.setSlot(p, NEVER);
    else if (p.kind === "MapPat") {
      for (const fl of p.fields) this.setSlot(fl, NEVER);
      if (p.rest !== null) this.setSlot(p.rest, NEVER);
    } else {
      for (const e of p.elems) this.setSlot(e.pat, NEVER);
      if (p.rest !== null) this.setSlot(p.rest, NEVER);
    }
  }

  private bindPattern(p: A.Pattern, v: Type, valueSpan: Span, f: Facts): void {
    if (p.kind === "NamePat") {
      this.setSlot(p, v);
      return;
    }
    if (hasNull(v)) {
      this.diag("maybe-null", valueSpan);
      v = nonnull(v);
    }
    const owners = [p.span, ...(p.kind === "MapPat" ? p.fields.map((x) => x.span) : p.elems.map((x) => x.span))];
    if (isAny(v)) {
      if (this.site("dynamic", valueSpan, ...owners)) {
        this.neverNames(p);
        return;
      }
      // Every name is `any`; defaults are still checked.
      if (p.kind === "MapPat") {
        for (const fl of p.fields) {
          let t: Type = ANY;
          if (fl.default !== null) t = join(ANY, this.synth(fl.default, f));
          this.setSlot(fl, t);
        }
      } else {
        for (const e of p.elems) {
          let t: Type = ANY;
          if (e.default !== null) t = join(ANY, this.synth(e.default, f));
          // Ruling AO6 (b): the index site on each element without a default.
          else this.site("index", e.pat.span);
          this.setSlot(e.pat, t);
        }
      }
      if (p.rest !== null) this.setSlot(p.rest, ANY);
      return;
    }
    if (isNever(v)) {
      // Nothing to read: every name is `never`; defaults are checked.
      if (p.kind === "MapPat") for (const fl of p.fields) if (fl.default !== null) this.synth(fl.default, f);
      if (p.kind === "ListPat") for (const e of p.elems) if (e.default !== null) this.synth(e.default, f);
      this.neverNames(p);
      return;
    }
    if (p.kind === "MapPat") {
      if (kindCount(v) !== 1 || !v.map) {
        this.diag("kind", valueSpan);
        this.neverNames(p);
        return;
      }
      const r = v.map;
      for (const fl of p.fields) {
        let t: Type;
        if (fl.default !== null) {
          const read = this.readKey(r, fl.key, true, fl.keySpan, fl.keySpan, false, [p.span, fl.span]);
          const d = this.synth(fl.default, f);
          t = join(nonnull(read), d);
        } else {
          // Ruling AO6 (a): the dynamic site of an unlisted key is on the key in the pattern.
          t = this.readKey(r, fl.key, false, fl.keySpan, fl.keySpan, false, [p.span, fl.span]);
        }
        this.setSlot(fl, t);
      }
      if (p.rest !== null) {
        const named = new Set(p.fields.map((x) => x.key));
        const fields = new Map([...r.fields].filter(([k]) => !named.has(k)));
        this.setSlot(p.rest, { map: { fields, rest: r.rest } });
      }
      return;
    }
    if (kindCount(v) !== 1 || !v.list) {
      this.diag("kind", valueSpan);
      this.neverNames(p);
      return;
    }
    const elem = v.list;
    for (const e of p.elems) {
      if (e.default !== null) {
        const d = this.synth(e.default, f);
        this.setSlot(e.pat, join(nonnull(elem), d));
      } else {
        this.site("index", e.pat.span);
        this.setSlot(e.pat, elem);
      }
    }
    if (p.rest !== null) this.setSlot(p.rest, v);
  }

  // ---- expressions ------------------------------------------------------------

  synth(e: A.Expr, f: Facts): Type {
    const t = this.synth0(e, f);
    this.types.set(e, t);
    return t;
  }

  private synth0(e: A.Expr, f: Facts): Type {
    switch (e.kind) {
      case "Number":
        return Number.isSafeInteger(e.value) ? INT : FLOAT;
      case "String":
        return lit(e.value);
      case "Bool":
        return BOOL;
      case "Null":
        return NULL;
      case "Bad":
        return NEVER;
      case "Ident":
        return this.ident(e, f);
      case "Template":
        for (const part of e.parts) {
          if (typeof part === "string") continue;
          this.rendered(part, this.synth(part, f), [e.span]);
        }
        return STRING;
      case "List":
        return this.list(e, f);
      case "Map":
        return this.map(e, f);
      case "Unary":
        return this.unary(e, f);
      case "Binary":
        return this.binary(e, f);
      case "Cond":
        return this.cond(e, f);
      case "Block":
        return this.block(e.consts, e.result, f);
      case "Match":
        return this.match(e, f);
      case "Try":
        return this.tryExpr(e, f);
      case "Call":
        return this.call(e, f);
      case "OptChain":
      case "Member":
      case "Index":
      case "MethodCall":
        return this.chain(e, f);
    }
  }

  private ident(e: A.Ident, f: Facts): Type {
    const r = this.res.refs.get(e);
    if (r === undefined || r.k === "lambda") return NEVER;
    let t = r.k === "slot" ? (this.slotTypes[r.slot] ?? NEVER) : (this.env.variables.get(r.name) ?? NEVER);
    if (f.has(r.k === "slot" ? `s${r.slot}` : `v${r.name}`)) t = nonnull(t);
    return t;
  }

  /** A rendered expression (ruling AO2). */
  private rendered(e: A.Expr, t: Type, owners: Span[]): void {
    if (hasOpaque(t)) this.diag("kind", e.span);
    else if (hasAny(t)) this.site("dynamic", e.span, ...owners);
  }

  private depthLimit(t: Type, span: Span): Type {
    if (depth(t, 70) > 64) {
      this.diag("check-limit", span);
      return NEVER;
    }
    return t;
  }

  private list(e: A.List, f: Facts): Type {
    let j: Type = NEVER;
    for (const it of e.items) {
      if (it.kind !== "Spread") {
        j = join(j, this.synth(it, f));
        continue;
      }
      let t = this.synth(it.value, f);
      if (hasNull(t)) {
        this.diag("maybe-null", it.value.span);
        t = nonnull(t);
      }
      if (isAny(t)) {
        if (!this.site("dynamic", it.value.span, it.span)) j = join(j, ANY);
      } else if (isNever(t)) {
        // nothing
      } else if (kindCount(t) === 1 && t.list) j = join(j, t.list);
      else this.diag("kind", it.value.span);
    }
    return this.depthLimit(listOf(j), e.span);
  }

  private map(e: A.MapLit, f: Facts): Type {
    let r: Rec = { fields: new Map(), rest: NEVER };
    for (const en of e.entries) {
      if (en.kind === "Spread") {
        let t = this.synth(en.value, f);
        if (hasNull(t)) {
          this.diag("maybe-null", en.value.span);
          t = nonnull(t);
        }
        if (isAny(t)) {
          if (!this.site("dynamic", en.value.span, en.span)) r = merge(r, ANY);
        } else if (isNever(t)) {
          // nothing
        } else if (kindCount(t) === 1 && t.map) r = merge(r, t.map);
        else this.diag("kind", en.value.span);
        continue;
      }
      if (en.key.kind === "expr") {
        let k = this.synth(en.key.expr, f);
        const v = this.synth(en.value, f);
        if (hasNull(k)) {
          this.diag("maybe-null", en.key.expr.span);
          k = nonnull(k);
        }
        const fit = fits(k, STRING);
        if (fit === 0) this.diag("kind", en.key.expr.span);
        else if (fit === 2) this.site("dynamic", en.key.expr.span, en.span);
        r = merge(r, { fields: new Map(), rest: v });
        continue;
      }
      const key = en.key.kind === "word" ? en.key.text : en.key.value;
      const v = this.synth(en.value, f);
      const fields = new Map(r.fields);
      fields.set(key, { type: v, optional: false });
      r = { fields, rest: r.rest };
    }
    return this.depthLimit({ map: r }, e.span);
  }

  private unary(e: A.Unary, f: Facts): Type {
    let t = this.synth(e.x, f);
    if (e.op === "!") {
      this.tested(t, e.x.span);
      return BOOL;
    }
    if (hasNull(t)) {
      this.diag("maybe-null", e.x.span);
      t = nonnull(t);
    }
    if (isAny(t)) return this.site("dynamic", e.x.span, e.span) ? NEVER : ANY;
    if (fits(t, FLOAT) === 0) {
      this.diag("kind", e.x.span);
      return NEVER;
    }
    return t;
  }

  // ---- binary operators (8) -----------------------------------------------------

  private binary(e: A.Binary, f: Facts): Type {
    // The left spine, checked by a loop (20.6).
    const spine: A.Binary[] = [];
    let x: A.Expr = e;
    while (x.kind === "Binary" && x.op !== "**") {
      spine.push(x);
      x = x.x;
    }
    if (spine.length === 0) return this.binaryOp(e, this.synth(e.x, f), f);
    let t = this.synth(x, f);
    for (let i = spine.length - 1; i >= 0; i--) {
      t = this.binaryOp(spine[i], t, f);
      this.types.set(spine[i], t);
    }
    return t;
  }

  /** One binary operator, given the type of its left operand. */
  private binaryOp(n: A.Binary, a: Type, f: Facts): Type {
    switch (n.op) {
      case "&&": {
        this.tested(a, n.x.span);
        const b = this.synth(n.y, this.withFacts(f, this.facts(n.x).t));
        return join(falsyPart(a), b);
      }
      case "||": {
        this.tested(a, n.x.span);
        const b = this.synth(n.y, this.withFacts(f, this.facts(n.x).f));
        return join(truthyPart(a), b);
      }
      case "??":
        return join(nonnull(a), this.synth(n.y, f));
      case "==":
      case "!=": {
        const b = this.synth(n.y, f);
        if (!overlap(a, b) && !isNever(a) && !isNever(b) && !isExactlyNull(a) && !isExactlyNull(b)) this.diag("never-equal", n.span);
        return BOOL;
      }
      case "in":
      case "not-in":
        return this.inOp(n, a, this.synth(n.y, f));
      default:
        return this.arith(n, a, this.synth(n.y, f));
    }
  }

  /** The null and `any` rules of 8.3 for one operand. Returns its type, and whether it was `any`. */
  private operand(t: Type, x: A.Expr, n: A.Binary): { t: Type; any: boolean; never: boolean } {
    if (hasNull(t)) {
      this.diag("maybe-null", x.span);
      t = nonnull(t);
    }
    if (isAny(t)) {
      if (this.site("dynamic", x.span, n.span)) return { t: NEVER, any: false, never: true };
      return { t, any: true, never: false };
    }
    return { t, any: false, never: isNever(t) };
  }

  private arith(n: A.Binary, at: Type, bt: Type): Type {
    const a = this.operand(at, n.x, n);
    const b = this.operand(bt, n.y, n);
    const op = n.op;
    const isNum = (e: A.Expr, v: number | null) => e.kind === "Number" && (v === null || e.value !== v);
    if (op === "+" || op === "<" || op === "<=" || op === ">" || op === ">=") {
      const order = op !== "+";
      const fa = fits(a.t, FLOAT) !== 0;
      const sa = fits(a.t, STRING) !== 0;
      const fb = fits(b.t, FLOAT) !== 0;
      const sb = fits(b.t, STRING) !== 0;
      let bad = false;
      if (!fa && !sa) {
        this.diag("kind", n.x.span);
        bad = true;
      }
      if (!fb && !sb) {
        this.diag("kind", n.y.span);
        bad = true;
      }
      if (!bad && ((fa && !sa && sb && !fb) || (sa && !fa && fb && !sb))) {
        this.diag("kind", n.span);
        bad = true;
      }
      if (order) return BOOL;
      if (bad) return NEVER;
      if (a.any || b.any) return ANY;
      if (fits(a.t, INT) !== 0 && fits(b.t, INT) !== 0) return INT;
      if (fa && fb) return FLOAT;
      return STRING;
    }
    if (op === "**") {
      let bad = false;
      if (fits(a.t, FLOAT) === 0) {
        this.diag("kind", n.x.span);
        bad = true;
      }
      if (fits(b.t, INT) === 0) {
        this.diag("kind", n.y.span);
        bad = true;
      }
      if (!(n.y.kind === "Number" || isNum(n.x, 0))) this.site("divide", n.span);
      if (bad) return NEVER;
      if (a.any || b.any) return ANY;
      return fits(a.t, INT) !== 0 && n.y.kind === "Number" ? INT : FLOAT;
    }
    // -, *, /, %
    let bad = false;
    if (fits(a.t, FLOAT) === 0) {
      this.diag("kind", n.x.span);
      bad = true;
    }
    if (fits(b.t, FLOAT) === 0) {
      this.diag("kind", n.y.span);
      bad = true;
    }
    if (op === "/") {
      if (n.y.kind === "Number" && n.y.value === 0) this.diag("zero-divisor", n.y.span);
      if (!isNum(n.y, 0)) this.site("divide", n.span);
    }
    if (op === "%" && !isNum(n.y, 0)) this.site("divide", n.span);
    if (bad) return NEVER;
    if (a.any || b.any) return ANY;
    if (op === "/") return FLOAT;
    return fits(a.t, INT) !== 0 && fits(b.t, INT) !== 0 ? INT : FLOAT;
  }

  private inOp(n: A.Binary, at: Type, bt: Type): Type {
    let a = at;
    let b = bt;
    if (hasNull(b)) {
      this.diag("maybe-null", n.y.span);
      b = nonnull(b);
    }
    if (isAny(b)) {
      this.site("dynamic", n.y.span, n.span);
      return BOOL;
    }
    if (isNever(b)) return BOOL;
    if (kindCount(b) === 1 && b.list) {
      if (!overlap(a, b.list) && !isNever(a) && !isNever(b.list) && !isExactlyNull(a) && !isExactlyNull(b.list)) this.diag("never-equal", n.span);
      return BOOL;
    }
    if (kindCount(b) === 1 && (b.str || b.map)) {
      // The left side "must fit string; null is maybe-null" (8.4): with a list
      // on the right, a null left operand is allowed (questions file, ts S4).
      if (hasNull(a)) {
        this.diag("maybe-null", n.x.span);
        a = nonnull(a);
      }
      const fit = fits(a, STRING);
      if (fit === 0) this.diag("kind", n.x.span);
      else if (fit === 2) this.site("dynamic", n.x.span, n.span);
      return BOOL;
    }
    this.diag("kind", n.y.span);
    return BOOL;
  }

  // ---- choice (11) ----------------------------------------------------------------

  private cond(e: A.Cond, f: Facts): Type {
    let t: Type = NEVER;
    let c: A.Expr = e;
    let facts = f;
    while (c.kind === "Cond") {
      const tt = this.synth(c.test, facts);
      this.tested(tt, c.test.span);
      const fc = this.facts(c.test);
      t = join(t, this.synth(c.then, this.withFacts(facts, fc.t)));
      facts = this.withFacts(facts, fc.f);
      c = c.else;
    }
    return join(t, this.synth(c, facts));
  }

  private litType(l: A.Literal): Type {
    switch (l.kind) {
      case "Number":
        return Number.isSafeInteger(l.value) ? INT : FLOAT;
      case "String":
        return lit(l.value);
      case "Bool":
        return BOOL;
      case "Null":
        return NULL;
    }
  }

  private match(e: A.Match, f: Facts): Type {
    const s = this.synth(e.subject, f);
    const path = this.pathOf(e.subject);
    let facts = f;
    let t: Type = NEVER;
    let hasDefault = false;
    const covered: (string | boolean | null | number)[] = [];
    for (const arm of e.arms) {
      let listsNull = false;
      for (const l of arm.literals) {
        if (l.kind === "Null") listsNull = true;
        else if (!isNever(s) && !overlap(this.litType(l), s)) this.diag("never-equal", l.span);
        covered.push(l.kind === "Null" ? null : l.value);
      }
      if (arm.isDefault) hasDefault = true;
      t = join(t, this.synth(arm.result, facts));
      if (listsNull && path !== null) facts = this.withFacts(facts, [path]);
    }
    if (!hasDefault && !isNever(s)) {
      const values = finiteValues(s);
      if (values === null || values.some((v) => !covered.includes(v))) this.diag("non-exhaustive", e.span);
    }
    return t;
  }

  private tryExpr(e: A.Try, f: Facts): Type {
    this.drop++;
    const b = this.synth(e.body, f);
    this.drop--;
    const slot = this.res.catchSlots.get(e);
    if (slot !== undefined) this.slotTypes[slot] = ERROR_TYPE;
    return join(b, this.synth(e.handler, f));
  }

  // ---- chains (9) -------------------------------------------------------------------

  private chain(e: A.Expr, f: Facts): Type {
    const steps: A.Expr[] = [];
    let x: A.Expr = e.kind === "OptChain" ? e.x : e;
    for (;;) {
      if (x.kind === "Member" || x.kind === "Index") {
        steps.push(x);
        x = x.x;
      } else if (x.kind === "MethodCall") {
        if (this.res.targets.has(x)) break; // a call of a namespace
        steps.push(x);
        x = x.x;
      } else break;
    }
    steps.reverse();
    let cur = x.kind === "MethodCall" ? this.call(x, f) : this.synth(x, f);
    if (x.kind === "MethodCall") this.types.set(x, cur);
    let path: string | null = x.kind === "Ident" ? this.pathOf(x) : null;
    let skipped = false;
    let recvSpan: Span = x.span;
    for (const s of steps) {
      const optional = (s as A.Member | A.Index | A.MethodCall).optional;
      if (hasNull(cur)) {
        if (optional) skipped = true;
        else this.diag("maybe-null", recvSpan);
        cur = nonnull(cur);
      }
      if (!isAny(cur) && !isNever(cur) && kindCount(cur) !== 1) {
        this.diag("kind", recvSpan);
        cur = NEVER;
      }
      // The field this step reads, when the step is a field read.
      const key = s.kind === "Member" ? s.name : s.kind === "Index" && s.index.kind === "String" ? s.index.value : null;
      const nextPath = path !== null && key !== null ? path + SEP + key : null;
      const present = nextPath !== null && f.has(nextPath);
      cur = this.step(s, cur, recvSpan, f, present);
      if (nextPath !== null && f.has(nextPath)) cur = nonnull(cur);
      path = nextPath;
      recvSpan = s.span;
      this.types.set(s, cur);
    }
    return skipped ? withNull(cur) : cur;
  }

  private step(s: A.Expr, cur: Type, recvSpan: Span, f: Facts, present: boolean): Type {
    if (s.kind === "MethodCall") return this.methodCall(s, cur, recvSpan, f);
    const optional = (s as A.Member | A.Index).optional;
    if (s.kind === "Index") {
      if (isAny(cur)) {
        this.synth(s.index, f);
        return this.site("dynamic", recvSpan, s.span) ? NEVER : ANY;
      }
      if (isNever(cur)) {
        this.synth(s.index, f);
        return NEVER;
      }
      if (s.index.kind === "String" && cur.map) {
        this.synth(s.index, f);
        return this.readKey(cur.map, s.index.value, optional, s.index.span, recvSpan, present, [s.span]);
      }
      let k = this.synth(s.index, f);
      const need = cur.map ? STRING : INT;
      if (cur.map || cur.list || cur.str) {
        if (hasNull(k)) {
          this.diag("maybe-null", s.index.span);
          k = nonnull(k);
        }
        const fit = fits(k, need);
        if (fit === 0) this.diag("kind", s.index.span);
        else if (fit === 2) this.site("dynamic", s.index.span, s.span);
      }
      if (cur.map) {
        const r = cur.map;
        // Ruling AO11: a key that is not a string is `kind` on the index only.
        if (fits(k, STRING) === 0 && !isNever(k)) return NEVER;
        const v = recordValues(r);
        if (optional) return withNull(v);
        if (isAny(r.rest)) this.site("dynamic", recvSpan, s.span);
        else this.diag("maybe-absent", s.index.span);
        return v;
      }
      if (cur.list) {
        if (optional) return withNull(cur.list);
        this.site("index", s.span);
        return cur.list;
      }
      if (cur.str) {
        if (optional) return withNull(STRING);
        this.site("index", s.span);
        return STRING;
      }
      this.diag("kind", recvSpan);
      return NEVER;
    }
    // Member
    const m = s as A.Member;
    if (isAny(cur)) {
      if (optional) {
        this.site("dynamic", recvSpan, s.span);
        return ANY;
      }
      return this.site("dynamic", recvSpan, s.span) ? NEVER : ANY;
    }
    if (isNever(cur)) return NEVER;
    if (cur.map) return this.readKey(cur.map, m.name, optional, m.nameSpan, recvSpan, present, [s.span]);
    this.diag("kind", recvSpan);
    return NEVER;
  }

  /** Read a field of a record (9.2), plain or as by `?.`. */
  private readKey(r: Rec, key: string, optional: boolean, keySpan: Span, recvSpan: Span, present: boolean, owners: Span[]): Type {
    const fl = r.fields.get(key);
    if (fl !== undefined && (!fl.optional || present)) return fl.type;
    if (fl !== undefined || (!isNever(r.rest) && !isAny(r.rest))) {
      const t = fl !== undefined ? fl.type : r.rest;
      if (present) return t;
      if (optional) return withNull(t);
      this.diag("maybe-absent", keySpan);
      return t;
    }
    if (isAny(r.rest)) {
      if (optional || present) return ANY;
      return this.site("dynamic", recvSpan, ...owners) ? NEVER : ANY;
    }
    this.diag("no-field", keySpan);
    return NEVER;
  }

  // ---- calls (13) -------------------------------------------------------------------

  private lambdaArg(a: A.Arg): LambdaArg | null {
    if (a.kind === "Lambda") return { lambda: a, id: null, span: a.span };
    if (a.kind === "Ident") {
      const r = this.res.refs.get(a);
      if (r !== undefined && r.k === "lambda") return { lambda: this.res.lambdas[r.id], id: r.id, span: a.span };
    }
    return null;
  }

  /** Synth every plain argument; lambda arguments are not checked. */
  private plainArgs(args: A.Arg[], f: Facts): void {
    for (const a of args) if (this.lambdaArg(a) === null) this.synth(a as A.Expr, f);
  }

  private call(node: A.Call | A.MethodCall, f: Facts): Type {
    const target = this.res.targets.get(node);
    const args = node.args;
    if (target === undefined || target.k === "unknown") {
      this.plainArgs(args, f);
      return NEVER;
    }
    if (target.k === "lambda") return this.lambdaCall(node, target.id, f);
    const sig = target.k === "library" ? this.env.library.get(target.name) : this.env.functions.get(target.name);
    if (sig === undefined) {
      this.plainArgs(args, f);
      return NEVER;
    }
    return this.solve(sig, node, null, null, args, f, false);
  }

  private methodCall(s: A.MethodCall, cur: Type, recvSpan: Span, f: Facts): Type {
    if (isAny(cur)) {
      if (this.site("dynamic", recvSpan, s.span)) {
        this.plainArgs(s.args, f);
        return NEVER;
      }
      for (const a of s.args) {
        const la = this.lambdaArg(a);
        if (la === null) this.synth(a as A.Expr, f);
        else this.lambdaInPlace(la, la.lambda.params.map(() => ANY), f, s.level);
      }
      return ANY;
    }
    if (isNever(cur)) {
      this.plainArgs(s.args, f);
      return NEVER;
    }
    let sig: CheckSig | undefined;
    if (cur.str) sig = this.env.libraryMethods.get("string")?.get(s.name);
    else if (cur.list) sig = this.env.libraryMethods.get("list")?.get(s.name);
    else if (cur.map) sig = this.env.libraryMethods.get("map")?.get(s.name);
    else if (cur.opaque) sig = this.env.methods.get([...cur.opaque][0])?.get(s.name);
    if (sig === undefined) {
      this.diag("no-method", s.nameSpan);
      this.plainArgs(s.args, f);
      return NEVER;
    }
    return this.solve(sig, s, s.x, cur, s.args, f, true, recvSpan);
  }

  /** Solve one call of a library entry or a host function or method (13.5). */
  private solve(sig: CheckSig, node: A.Call | A.MethodCall, recv: A.Expr | null, recvType: Type | null, args: A.Arg[], f: Facts, isMethod: boolean, recvSpan?: Span): Type {
    const params = sig.sig.params;
    const span = node.span;
    // 1. Count.
    let min = 0;
    let max = 0;
    let rest = false;
    for (const p of params) {
      if (p.rest) rest = true;
      else {
        max++;
        if (!p.optional) min++;
      }
    }
    if (args.length < min || (!rest && args.length > max)) {
      if (isMethod) this.diag("arity", span);
      this.plainArgs(args, f);
      return NEVER;
    }
    const paramAt = (i: number) => (i < params.length ? params[i] : params[params.length - 1]);
    const paramType = (i: number): TypeNode | null => {
      const p = paramAt(i);
      if (p.type.t === "fn") return null;
      if (p.rest && p.type.t === "list") return p.type.elem;
      return p.type as TypeNode;
    };
    const sol = new Map<string, Type>();
    const gain = (v: string, t: Type) => sol.set(v, sol.has(v) ? join(sol.get(v)!, t) : t);
    // 2. Plain arguments, argument zero first.
    if (sig.recv !== null && recvType !== null) this.bind(sig.recv, recvType, gain);
    const argTypes: (Type | null)[] = [];
    const lambdas: { i: number; la: LambdaArg }[] = [];
    args.forEach((a, i) => {
      const la = this.lambdaArg(a);
      const p = paramAt(i);
      if (la !== null) {
        argTypes.push(null);
        if (p.type.t === "fn") lambdas.push({ i, la });
        else if (isMethod) this.diag("not-a-value", la.span);
        return;
      }
      const t = this.synth(a as A.Expr, f);
      argTypes.push(t);
      if (p.type.t === "fn") {
        this.diag("arg-mismatch", a.span);
        return;
      }
      this.bind(paramType(i)!, t, gain);
      if (sig.tested.includes(i)) this.tested(t, a.span);
      if (sig.renders.includes(i)) this.rendered(a as A.Expr, t, [span]);
    });
    // 3. Lambda arguments.
    for (const { i, la } of lambdas) {
      const fn = paramAt(i).type as Extract<SigNode["params"][number]["type"], { t: "fn" }>;
      if (la.lambda.params.length > fn.params.length) {
        if (isMethod) this.diag("arity", la.span);
        continue;
      }
      const qs = fn.params.map((q) => q.type);
      if (qs.slice(0, la.lambda.params.length).some((q) => varsOf(q).some((v) => !sol.has(v)))) {
        this.diag("cannot-infer", la.span);
        continue;
      }
      const inst = (n: TypeNode) => fromNode(n, { opaque: this.env.opaque, variable: (v) => sol.get(v) ?? NEVER });
      const r = fn.result;
      const level = node.level;
      if (r === "unknown") {
        const b = this.lambdaUse(la, qs.map(inst), f, level);
        if (b !== null) this.tested(b.type, la.lambda.body.span);
        if (b !== null && sig.narrows && i === 0) this.narrowed = { la, elem: qs.length > 0 ? inst(qs[0]) : NEVER };
        continue;
      }
      const before = new Map(sol);
      const m = this.mark();
      let b = this.lambdaUse(la, qs.map(inst), f, level);
      if (b === null) continue;
      // 3a. The seed rule, also on a fit that used any (ruling AO3).
      const qVars = new Set(qs.flatMap(varsOf));
      const seed = varsOf(r).find((v) => qVars.has(v) && before.has(v) && fits(b!.type, before.get(v)!) !== 1);
      if (seed !== undefined) {
        this.rollback(m);
        sol.set(seed, join(before.get(seed)!, b.type));
        b = this.lambdaUse(la, qs.map(inst), f, level, true);
        if (b === null) continue;
        // No bind on the second check (ruling AO3 (a)).
      } else {
        this.bind(r, b.type, gain);
      }
      // The body always fits R, every Var by its solution (ruling AO1).
      this.fitSimple(b.type, inst(r), la.lambda.body.span, "arg-mismatch");
    }
    // 4. Bounds.
    for (const bd of sig.sig.bounds) {
      const t = sol.get(bd.name) ?? NEVER;
      const fit: Fit = bd.bound === "number" ? fits(t, FLOAT) : maxFit(fits(t, FLOAT), fits(t, STRING));
      if (fit === 0) this.diag("arg-mismatch", span);
      else if (fit === 2) this.site("dynamic", span, span);
    }
    // 5. Fit, through the form.
    const inst = (n: TypeNode) => fromNode(n, { opaque: this.env.opaque, variable: (v) => sol.get(v) ?? NEVER });
    const argCode = (place: Type, want: Type) => (hasNull(place) && fits(nonnull(place), want) !== 0 ? "maybe-null" : "arg-mismatch");
    // The receiver is fitted with its type as the chain gave it, after `?.` and nonnull (9.1).
    if (sig.recv !== null && recv !== null && recvType !== null) {
      if (fits(recvType, inst(sig.recv)) !== 1 || this.types.get(recv) === undefined) this.fitForm(recv, inst(sig.recv), argCode, recvSpan, [span], recvType);
    }
    args.forEach((a, i) => {
      if (argTypes[i] === null) return;
      const p = paramType(i);
      if (p === null) return;
      this.fitForm(a as A.Expr, inst(p), argCode, undefined, [span]);
    });
    // 6. Result.
    let result = inst(sig.sig.result);
    if (sig.sig.canFail) this.site(sig.kind === "library" ? "value" : "host", span);
    // 13.8: filter and find narrow.
    const nw = this.narrowed;
    this.narrowed = null;
    if (sig.narrows && nw !== null) {
      const n = this.narrowedType(nw.la, nw.elem);
      result = sig.name === "find" ? withNull(n) : listOf(n);
    }
    return this.depthLimit(result, span);
  }

  private narrowed: { la: LambdaArg; elem: Type } | null = null;

  private narrowedType(la: LambdaArg, elem: Type): Type {
    const p0 = la.lambda.params[0];
    if (p0 === undefined || p0.pattern.kind !== "NamePat" || p0.pattern.name === null) return elem;
    const slot = this.res.slots.get(p0.pattern);
    if (slot === undefined) return elem;
    const root = `s${slot}`;
    const t = this.facts(la.lambda.body).t;
    return t.some((k) => k === root || k.startsWith(root + SEP)) ? nonnull(elem) : elem;
  }

  /** bind(P, A) of 13.4. */
  private bind(p: TypeNode, a: Type, gain: (v: string, t: Type) => void): void {
    if (isAny(a)) {
      for (const v of varsOf(p)) gain(v, ANY);
      return;
    }
    if (p.t === "var") {
      gain(p.name, a);
      return;
    }
    if (p.t === "union" && p.alts.some((x) => x.t === "word" && x.word === "null")) {
      const rest = p.alts.filter((x) => !(x.t === "word" && x.word === "null"));
      this.bind(rest.length === 1 ? rest[0] : { t: "union", alts: rest }, nonnull(a), gain);
      return;
    }
    if (p.t === "list" && a.list) {
      this.bind(p.elem, a.list, gain);
      return;
    }
    if (p.t === "map" && a.map) {
      this.bind(p.rest, recordValues(a.map), gain);
      return;
    }
    if (p.t === "record" && a.map) {
      for (const fl of p.fields) {
        const af = a.map.fields.get(fl.key);
        if (af !== undefined) this.bind(fl.type, af.type, gain);
      }
    }
  }

  /** e must fit T, not through its form: code at e. */
  private fitSimple(t: Type, want: Type, span: Span, code: string): void {
    const fit = fits(t, want);
    if (fit === 0) this.diag(code, span);
    else if (fit === 2) this.site("dynamic", span, span);
  }

  /** e must fit T through its form (15.1). */
  private fitForm(e: A.Expr, want: Type, code: (place: Type, want: Type) => string, span?: Span, owners: Span[] = [], given?: Type): void {
    const place = span ?? e.span;
    const t = given ?? this.types.get(e) ?? NEVER;
    const fit = fits(t, want);
    if (fit === 2) {
      this.site("dynamic", place, ...owners);
      return;
    }
    if (fit === 1) return;
    if (isAny(want)) return;
    if (e.kind === "List" && want.list) {
      const elem = want.list;
      for (const it of e.items) {
        if (it.kind === "Spread") {
          const st = this.types.get(it.value) ?? NEVER;
          const sf = fits(st, listOf(elem));
          if (sf === 0) this.diag(code(st, listOf(elem)), it.value.span);
          else if (sf === 2) this.site("dynamic", it.value.span, ...owners);
        } else this.fitForm(it, elem, code, undefined, owners);
      }
      return;
    }
    if (e.kind === "Map" && want.map && e.entries.every((en) => en.kind === "Entry" && en.key.kind !== "expr")) {
      const r = want.map;
      const keys = new Map<string, A.Entry>();
      for (const en of e.entries as A.Entry[]) {
        const key = en.key.kind === "word" ? en.key.text : (en.key as { value: string }).value;
        keys.set(key, en);
      }
      let missing = false;
      for (const [k, fl] of r.fields) if (!fl.optional && !keys.has(k)) missing = true;
      if (missing) this.diag(code(t, want) === "maybe-null" ? "arg-mismatch" : code(t, want), place);
      for (const [k, en] of keys) {
        const fl = r.fields.get(k);
        if (fl === undefined && isNever(r.rest)) {
          const ks = (en.key as { span: Span }).span;
          this.diag(code(NEVER, NEVER) === "result-mismatch" ? "result-mismatch" : "arg-mismatch", ks);
          continue;
        }
        this.fitForm(en.value, fl !== undefined ? fl.type : r.rest, code, en.shorthand ? (en.key as { span: Span }).span : undefined, owners);
      }
      return;
    }
    if (e.kind === "Cond") {
      let c: A.Expr = e;
      while (c.kind === "Cond") {
        this.fitForm(c.then, want, code, undefined, owners);
        c = c.else;
      }
      this.fitForm(c, want, code, undefined, owners);
      return;
    }
    if (e.kind === "Match") {
      for (const arm of e.arms) this.fitForm(arm.result, want, code, undefined, owners);
      return;
    }
    if (e.kind === "Block") {
      this.fitForm(e.result, want, code, undefined, owners);
      return;
    }
    this.diag(code(t, want), place);
  }

  // ---- lambdas (13.2, 13.9) ------------------------------------------------------------

  /** Check a lambda written in place, with its parameters bound to `qs`. */
  private lambdaInPlace(la: LambdaArg, qs: Type[], f: Facts, level: number): { type: Type } | null {
    return this.lambdaUse(la, qs, f, level);
  }

  /**
   * A use of a lambda argument: a Lambda written there, or a const-bound
   * lambda by name, which counts against the limits of 20.2 and 20.3.
   */
  private lambdaUse(la: LambdaArg, qs: Type[], f: Facts, level: number, again = false): { type: Type } | null {
    if (la.id === null) {
      const t = this.lambdaBody(la.lambda, qs.map((type) => ({ given: true, type })), f);
      return { type: t };
    }
    if (!again && !this.useLimit(la.lambda, level + 1, la.span)) return null;
    const saved = this.offset;
    this.offset = level + 1 + this.offset + 1 - la.lambda.level;
    const t = this.lambdaBody(la.lambda, qs.map((type) => ({ given: true, type })), this.lambdaFacts.get(la.id) ?? NO_FACTS);
    this.offset = saved;
    return { type: t };
  }

  /** The limits of 20.2 and 20.3 at a use. False: `check-limit` was reported. */
  private useLimit(l: A.Lambda, useLevel: number, span: Span): boolean {
    this.uses++;
    const expanded = useLevel + this.offset;
    if (this.uses > 1000 || l.maxLevel - l.level + expanded + 1 > 100) {
      this.diag("check-limit", span);
      return false;
    }
    return true;
  }

  private lambdaBody(l: A.Lambda, args: { given: boolean; type: Type }[], f: Facts): Type {
    l.params.forEach((p, i) => {
      const a = args[i] ?? { given: false, type: NEVER };
      let t: Type;
      if (p.default !== null) {
        const d = this.synth(p.default, f);
        t = a.given ? join(nonnull(a.type), d) : d;
      } else t = a.type;
      this.bindPattern(p.pattern, t, p.pattern.span, f);
    });
    return this.synth(l.body, f);
  }

  private lambdaCall(node: A.Call | A.MethodCall, id: number, f: Facts): Type {
    const l = this.res.lambdas[id];
    const args = node.args;
    let min = 0;
    l.params.forEach((p, i) => {
      if (p.default === null) min = i + 1;
    });
    if (args.length < min || args.length > l.params.length) {
      this.plainArgs(args, f);
      return NEVER;
    }
    const given = args.map((a) => (this.lambdaArg(a) !== null ? { given: true, type: NEVER } : { given: true, type: this.synth(a as A.Expr, f) }));
    if (!this.useLimit(l, node.level, node.span)) return NEVER;
    const saved = this.offset;
    this.offset = node.level + this.offset + 1 - l.level;
    const t = this.lambdaBody(l, given, this.lambdaFacts.get(id) ?? NO_FACTS);
    this.offset = saved;
    return t;
  }
}

function maxFit(a: Fit, b: Fit): Fit {
  if (a === 1 || b === 1) return 1;
  if (a === 2 || b === 2) return 2;
  return 0;
}

function isExactlyNull(t: Type): boolean {
  return !isAny(t) && t.null === true && kindCount(t) === 1;
}

function union(a: string[], b: string[]): string[] {
  return [...new Set([...a, ...b])];
}

function intersect(a: string[], b: string[]): string[] {
  const s = new Set(b);
  return a.filter((x) => s.has(x));
}

/** The type variables of type text. */
export function varsOf(n: TypeNode): string[] {
  switch (n.t) {
    case "var":
      return [n.name];
    case "list":
      return varsOf(n.elem);
    case "map":
      return varsOf(n.rest);
    case "record":
      return [...n.fields.flatMap((f) => varsOf(f.type)), ...(typeof n.rest === "object" ? varsOf(n.rest) : [])];
    case "union":
      return n.alts.flatMap(varsOf);
    default:
      return [];
  }
}

function messageFor(code: string): string {
  return code.replace(/-/g, " ");
}

export { mapOf, opaque, alternatives, joinAll };
export type { Alts };
