// The Core checker: the type rules of 05-types.md sections 5 to 18.
//
// It runs after the static rules (05 22, src/eval/static.ts), whose
// resolution says what every name and call refers to. Each value binding has
// a slot, and the checker keeps a type for each slot. A fact {p} is a path
// key in a set. Long flat forms (a chain of binary operators, a chain of
// steps, an `else` chain, `const` lines) are checked by loops (20.6).

import type * as A from "../syntax/ast.ts";
import type { Source } from "../syntax/source.ts";
import { catalogMessage } from "../eval/failure.ts";
import { compareText } from "../value/value.ts";
import type { Diagnostic } from "../syntax/diagnostic.ts";
import type { Span } from "../syntax/source.ts";
import type { Resolution } from "../eval/static.ts";
import type { SigNode, TypeNode } from "./typetext.ts";
import {
  ANY, BOOL, FLOAT, INT, NEVER, NULL, STRING, alternatives, depth, falsyPart, finiteValues, fits, fromNode, hasNull,
  hasOpaque, isAny, isNever, join, joinAll, kindCount, lit, listOf, merge, nonnull, overlap, record, recordValues,
  truthyPart, withNull, mapOf, opaque, hasAny, typeSize, SIZE_LIMIT, TypeAcc, surelyFits, disjoint, without, printType, unite, hitVariants, allVariants, variantsType, tagSetOf,
} from "./type.ts";
import type { Alts, Fit, Rec, Type, Union } from "./type.ts";
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
  /** Rendered argument positions (05 13.5): an argument the entry renders as text. */
  renders: number[];
}

export interface CheckEnv {
  opaque: ReadonlySet<string>;
  /** Package F4: the named types (f04 3.3). */
  named?: ReadonlyMap<string, Type>;
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
  /** Spans of the nodes that hold a `host` site (18.3, 18.5: `function-error`). */
  hostSpans: Set<string>;
  /** Spans of the nodes that hold a `value` site (f04 6.6: a failed `as`). */
  valueSpans: Set<string>;
}

export type Expected = { kind: "none" } | { kind: "type"; type: Type } | { kind: "condition" };

const SEP = "\u0000";
/**
 * Facts are keys: a path (the presence fact {p}) or, with package F4, a fact
 * of `is` (f04 6.4): TYPE_FACT, `=` or `!`, the path, TYPE_SEP, and the type's
 * printed text, so that two facts about one type by structure are one key.
 */
const TYPE_FACT = "\u0001";
const TYPE_SEP = "\u0002";
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
  /** The template of the message and its parameters (f08 3.5). */
  template?: string;
  params?: Record<string, unknown>;
}

/** A parameter of kind `text`: the source text at a span (f08 3.2). */
export class TextAt {
  readonly span: Span;
  constructor(span: Span) {
    this.span = span;
  }
}

/** The canonical text of an operator (03 3.8.3). */
function opText(op: string): string {
  return op === "not-in" ? "not in" : op;
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

export function check(program: A.Program, res: Resolution, env: CheckEnv, expected: Expected, staticDiags: Diagnostic[], source?: Source): CheckResult {
  const c = new Checker(res, env);
  const t = c.program(program, expected);
  const diags: Diagnostic[] = [...staticDiags];
  const seen = new Set(staticDiags.map((d) => `${d.code}:${d.span!.start}:${d.span!.end}`));
  for (const d of c.diags) {
    const k = `${d.code}:${d.span.start}:${d.span.end}`;
    if (seen.has(k)) continue;
    seen.add(k);
    const out: Diagnostic = { phase: "check", code: d.code, span: d.span, message: checkMessage(d, source) };
    if (d.class !== undefined) out.class = d.class;
    diags.push(out);
  }
  diags.sort((a, b) => a.span!.start - b.span!.start || a.span!.end - b.span!.end || cmp(a.code, b.code));
  const siteSeen = new Set<string>();
  const sites: Site[] = [];
  const dynamicSpans = new Set<string>();
  const hostSpans = new Set<string>();
  const valueSpans = new Set<string>();
  for (const s of c.sites) {
    if (s.class === "dynamic") {
      dynamicSpans.add(spanKey(s.span));
      for (const o of s.owners) dynamicSpans.add(spanKey(o));
    }
    if (s.class === "host") hostSpans.add(spanKey(s.span));
    if (s.class === "value") valueSpans.add(spanKey(s.span));
    const k = `${s.class}:${s.span.start}:${s.span.end}`;
    if (siteSeen.has(k)) continue;
    siteSeen.add(k);
    sites.push({ class: s.class, span: s.span });
  }
  sites.sort((a, b) => a.span.start - b.span.start || a.span.end - b.span.end || cmp(a.class, b.class));
  // A callback's result is at a `dynamic` site on the lambda's body (18.3).
  const extra: string[] = [];
  for (const { body, call } of c.callbacks) if (dynamicSpans.has(spanKey(body))) extra.push(spanKey(call));
  for (const k of extra) dynamicSpans.add(k);
  return { type: t, sites, diagnostics: diags, dynamicSpans, hostSpans, valueSpans };
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
  /** Each lambda body checked as an argument of a call, with the call's span (18.3). */
  readonly callbacks: { body: Span; call: Span }[] = [];
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
    // A read of the name as a value is not a use (13.9).
    for (const [id, r] of res.refs) if (r.k === "lambda" && res.lambdaArgs.has(id)) this.used.add(r.id);
    for (const t of res.targets.values()) if (t.k === "lambda") this.used.add(t.id);
  }

  // ---- diagnostics and sites -----------------------------------------------

  private diag(code: string, span: Span, cls?: string, template?: string, params?: Record<string, unknown>): void {
    const d: DiagRec = { code, span: sp(span) };
    if (cls !== undefined) d.class = cls;
    if (template !== undefined) d.template = template;
    if (params !== undefined) d.params = params;
    this.diags.push(d);
  }

  /** A diagnostic of no class, with its template (f08 3.5). */
  private say(code: string, span: Span, template: string, params: Record<string, unknown> = {}): void {
    this.diag(code, span, undefined, template, params);
  }

  /** `maybe-null` at a span, with the type there (f08 3.5 item 3). */
  private maybeNull(span: Span, t: Type): void {
    this.say("maybe-null", span, "default", { text: new TextAt(span), type: printType(t) });
  }

  /** `kind`, by its template (f08 3.5 item 4). */
  private kind(span: Span, template: string, params: Record<string, unknown> = {}): void {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(params)) out[k] = isTypeLike(v) ? printType(v as Type) : v;
    this.say("kind", span, template, out);
  }

  /** The name a `value` or `host` site's `may-fail` names; null for the site of an `as` (f08 3.5 item 7). */
  private siteName: string | null = null;

  /** Record a site. For a rejected `dynamic` site, returns true: the node takes the type never (17). */
  private site(cls: string, span: Span, ...owners: Span[]): boolean {
    if (this.drop > 0) return false;
    if (this.env.reject.has(cls)) {
      if (cls === "dynamic") {
        this.diag("unchecked", span, "dynamic", "default", { text: new TextAt(span) });
        return true;
      }
      const template = cls === "value" ? (this.siteName === null ? "test" : "value") : cls;
      this.diag("may-fail", span, cls, template, { name: this.siteName ?? "" });
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
    const params = { text: new TextAt(span), type: printType(t) };
    if (isNever(falsyPart(t))) this.diag("always-truthy", span, "constant-condition", "default", params);
    else if (isNever(truthyPart(t))) this.diag("always-falsy", span, "constant-condition", "default", params);
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
      if (k.startsWith(TYPE_FACT)) {
        out.add(k);
        continue;
      }
      const parts = k.split(SEP);
      for (let i = 1; i <= parts.length; i++) out.add(parts.slice(0, i).join(SEP));
    }
    return out;
  }

  /** T(c) and F(c) (10.5), as path keys. */
  private facts(e: A.Expr): { t: string[]; f: string[] } {
    const memo = this.factsMemo.get(e);
    if (memo !== undefined) return memo;
    const vol = this.volatile;
    let out: { t: string[]; f: string[] } = { t: [], f: [] };
    const p = this.pathOf(e);
    if (p !== null) out = { t: [p], f: [] };
    else if (e.kind === "As") {
      // f04 6.4: the facts of `p is T`.
      const q = this.pathOf(e.x);
      const t = this.res.asTypes.get(e);
      if (e.form === "is" && q !== null && t !== undefined && t !== null) {
        const nullable = !isAny(t) && t.null === true;
        out = {
          t: [this.typeFact("=", q, t), ...(!nullable && !isAny(t) ? [q] : [])],
          f: [this.typeFact("!", q, t), ...(nullable ? [q] : [])],
        };
      }
    } else if (e.kind === "Unary" && e.op === "!") {
      const d = this.facts(e.x);
      out = { t: d.f, f: d.t };
    } else if (e.kind === "Binary") {
      if (e.op === "==" || e.op === "!=") {
        const q = e.y.kind === "Null" ? this.pathOf(e.x) : e.x.kind === "Null" ? this.pathOf(e.y) : null;
        if (q !== null) out = e.op === "!=" ? { t: [q], f: [] } : { t: [], f: [q] };
        // f05 5.2: a test of a field against a string literal.
        else if (e.y.kind === "String") out = this.fieldTest(e.x, new Set([e.y.value]), e.op === "!=");
        else if (e.x.kind === "String") out = this.fieldTest(e.y, new Set([e.x.value]), e.op === "!=");
      } else if ((e.op === "in" || e.op === "not-in") && e.y.kind === "List" && e.y.items.length > 0 && e.y.items.every((it) => it.kind === "String")) {
        out = this.fieldTest(e.x, new Set(e.y.items.map((it) => (it as A.StringLit).value)), e.op === "not-in");
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
              : { t: this.orFacts(acc.t, d.t), f: union(acc.f, d.f) };
          this.factsMemo.set(n, acc);
        }
        out = acc;
      }
    }
    // A fact of a test of a field depends on the types where it stands: not kept.
    if (this.volatile === vol) this.factsMemo.set(e, out);
    return out;
  }

  /** Bumped whenever a fact is made from the types where a test stands (f05 5.2). */
  private volatile = 0;

  /** The receiver path p and key f of a path q with at least one step (f05 5.2 item 1). */
  private fieldPath(q: A.Expr): { p: string; pNode: A.Expr; key: string } | null {
    const full = this.pathOf(q);
    if (full === null) return null;
    const qq = q.kind === "OptChain" ? q.x : q;
    let key: string;
    if (qq.kind === "Member") key = qq.name;
    else if (qq.kind === "Index" && qq.index.kind === "String") key = qq.index.value;
    else return null;
    const p = this.pathOf(qq.x);
    if (p === null) return null;
    return { p, pNode: qq.x, key };
  }

  /** Whether a test of a field about p gives facts: M is a union, or a record with such a fact in force (f05 5.2 item 3). */
  private fieldFactsHold(m: Type, _p: string): boolean {
    if (isAny(m) || !m.map) return false;
    if (m.union) return true;
    return this.narrowedByField.has(m);
  }

  /** The types that a fact of a test of a field gave a path (f05 5.2 item 3). */
  private readonly narrowedByField = new WeakSet<object>();

  /** T and F of `q == L`, `q in [...]` (f05 5.2); `negated` swaps them. */
  private fieldTest(q: A.Expr, strs: ReadonlySet<string>, negated: boolean): { t: string[]; f: string[] } {
    const fp = this.fieldPath(q);
    if (fp === null) return { t: [], f: [] };
    const pt = this.types.get(fp.pNode);
    if (pt === undefined || !this.fieldFactsHold(pt, fp.p)) return { t: [], f: [] };
    this.volatile++;
    const m = pt as Alts;
    const hit = hitVariants(m, fp.key, strs);
    const all = allVariants(m, fp.key, strs);
    const yes: string[] = hit.length > 0 ? [this.typeFact("=", fp.p, variantsType(hit), true), fp.p] : [];
    const no: string[] = all.length > 0 ? [this.typeFact("!", fp.p, variantsType(all), true)] : [];
    return negated ? { t: no, f: yes } : { t: yes, f: no };
  }

  // ---- the facts of `is` (package F4, f04 6.4) ------------------------------

  private readonly factTypes = new Map<string, { kind: "=" | "!"; path: string; type: Type; field: boolean }>();

  private typeFact(kind: "=" | "!", path: string, type: Type, field = false): string {
    const key = `${TYPE_FACT}${kind}${path}${TYPE_SEP}${printType(type)}`;
    const known = this.factTypes.get(key);
    if (known === undefined) this.factTypes.set(key, { kind, path, type, field });
    else if (field) known.field = true;
    return key;
  }

  /** T(d || e): ∩, except that one conforms fact about p on each side joins (f04 6.4 item 4). */
  private orFacts(a: string[], b: string[]): string[] {
    const conforms = (xs: string[]) => {
      const by = new Map<string, string[]>();
      for (const x of xs) {
        const tf = this.factTypes.get(x);
        if (tf === undefined || tf.kind !== "=") continue;
        if (!by.has(tf.path)) by.set(tf.path, []);
        by.get(tf.path)!.push(x);
      }
      return by;
    };
    const ca = conforms(a);
    const cb = conforms(b);
    const bs = new Set(b);
    const out: string[] = [];
    for (const x of a) {
      const tf = this.factTypes.get(x);
      if (tf !== undefined && tf.kind === "=") {
        const as = ca.get(tf.path)!;
        const other = cb.get(tf.path);
        if (as.length === 1 && other !== undefined && other.length === 1) {
          // f05 5.4 item 1: Unite, not Join.
          out.push(this.typeFact("=", tf.path, unite(tf.type, this.factTypes.get(other[0])!.type), tf.field || this.factTypes.get(other[0])!.field));
        }
        continue;
      }
      if (bs.has(x)) out.push(x);
    }
    return [...new Set(out)];
  }

  /** The facts of `is` about one path, applied in order (f04 6.4). */
  private applyTypeFacts(t: Type, path: string, f: Facts): Type {
    if (this.factTypes.size === 0) return t;
    for (const k of f) {
      if (!k.startsWith(TYPE_FACT)) continue;
      const tf = this.factTypes.get(k);
      if (tf === undefined || tf.path !== path) continue;
      const before = t;
      if (tf.kind === "=") t = surelyFits(t, tf.type) ? t : tf.type;
      else t = without(t, tf.type);
      if (tf.field) {
        // A type a fact of a test of a field gave: such a fact is in force at p (f05 5.2 item 3).
        if (t === before || this.narrowedByField.has(t) === false) t = { ...t };
        this.narrowedByField.add(t);
      }
    }
    return t;
  }

  /** `x as T`, `x is T` (f04 6.3). */
  private asNode(e: A.As, f: Facts): Type {
    const s = this.synth(e.x, f);
    const t = this.res.asTypes.get(e);
    const isForm = e.form === "is";
    if (t === undefined || t === null) return isForm ? BOOL : NEVER;
    if (isNever(s)) return isForm ? BOOL : t;
    if (surelyFits(s, t)) {
      if (isForm && this.env.reject.has("constant-condition")) this.diag("always-truthy", e.span, "constant-condition", "test", { text: new TextAt(e.span), type: printType(s) });
      return isForm ? BOOL : t;
    }
    if (disjoint(s, t)) {
      if (!isForm) this.say("never-conforms", e.span, "default", { found: printType(s), expected: printType(t) });
      else if (this.env.reject.has("constant-condition")) this.diag("always-falsy", e.span, "constant-condition", "test", { text: new TextAt(e.span), type: printType(s) });
      return isForm ? BOOL : t;
    }
    if (!isForm) {
      this.siteName = null;
      this.site("value", e.span);
    }
    return isForm ? BOOL : t;
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

  /** A pattern with nothing to read: every name is `never`, and each default is still checked (12.2). */
  private failedPattern(p: A.Pattern, f: Facts): void {
    this.neverNames(p);
    if (p.kind === "MapPat") for (const fl of p.fields) if (fl.default !== null) this.synth(fl.default, f);
    if (p.kind === "ListPat") for (const e of p.elems) if (e.default !== null) this.synth(e.default, f);
  }

  private bindPattern(p: A.Pattern, v: Type, valueSpan: Span, f: Facts): void {
    if (p.kind === "NamePat") {
      this.setSlot(p, v);
      return;
    }
    if (hasNull(v)) {
      this.maybeNull(valueSpan, v);
      v = nonnull(v);
    }
    const owners = [p.span, ...(p.kind === "MapPat" ? p.fields.map((x) => x.span) : p.elems.map((x) => x.span))];
    if (isAny(v)) {
      if (this.site("dynamic", valueSpan, ...owners)) {
        this.failedPattern(p, f);
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
          // 12.4: the index site on each element without a default.
          else this.site("index", e.pat.span);
          this.setSlot(e.pat, t);
        }
      }
      if (p.rest !== null) this.setSlot(p.rest, ANY);
      return;
    }
    if (isNever(v)) {
      this.failedPattern(p, f);
      return;
    }
    if (p.kind === "MapPat") {
      if (kindCount(v) !== 1 || !v.map) {
        this.kind(valueSpan, "pattern-map", { type: v });
        this.failedPattern(p, f);
        return;
      }
      const r = v.map;
      for (const fl of p.fields) {
        let t: Type;
        if (fl.default !== null) {
          const read = this.readKey(r, fl.key, true, fl.keySpan, fl.keySpan, false, [p.span, fl.span]);
          const d = this.synth(fl.default, f);
          t = this.sizeLimit(join(nonnull(read), d), valueSpan);
        } else {
          // 12.4: the dynamic site of an unlisted key is on the key in the pattern.
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
      this.kind(valueSpan, "pattern-list", { type: v });
      this.failedPattern(p, f);
      return;
    }
    const elem = v.list;
    for (const e of p.elems) {
      if (e.default !== null) {
        const d = this.synth(e.default, f);
        this.setSlot(e.pat, this.sizeLimit(join(nonnull(elem), d), valueSpan));
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
      case "As":
        return this.asNode(e, f);
      case "Each": {
        const r = this.eachType(e, f);
        return r.skipped ? withNull(r.type) : r.type;
      }
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
    const key = r.k === "slot" ? `s${r.slot}` : `v${r.name}`;
    if (f.has(key)) t = nonnull(t);
    return this.applyTypeFacts(t, key, f);
  }

  /** A rendered expression (05 7.3): opaque is `kind`, any is a `dynamic` site. */
  private rendered(e: A.Expr, t: Type, owners: Span[]): void {
    if (hasOpaque(t)) this.kind(e.span, "rendered", { type: t });
    else if (hasAny(t)) this.site("dynamic", e.span, ...owners);
  }

  /** The size limit (05 20.7): `check-limit` on the node, which then has type never. */
  private sizeLimit(t: Type, span: Span): Type {
    return typeSize(t) > SIZE_LIMIT ? this.sizeFail(span) : t;
  }

  private sizeFail(span: Span): Type {
    this.say("check-limit", span, "size");
    return NEVER;
  }

  private depthLimit(t: Type, span: Span): Type {
    if (depth(t, 70) > 64) {
      this.say("check-limit", span, "depth");
      return NEVER;
    }
    return t;
  }

  private list(e: A.List, f: Facts): Type {
    // The item types are joined once, at the end (05 20.7).
    const acc = new TypeAcc();
    const add = (t: Type) => acc.add(t);
    for (const it of e.items) {
      if (it.kind !== "Spread") {
        add(this.synth(it, f));
        continue;
      }
      let t = this.synth(it.value, f);
      if (hasNull(t)) {
        this.maybeNull(it.value.span, t);
        t = nonnull(t);
      }
      if (isAny(t)) {
        if (!this.site("dynamic", it.value.span, it.span)) add(ANY);
      } else if (isNever(t)) {
        // nothing
      } else if (kindCount(t) === 1 && t.list) add(t.list);
      else this.kind(it.value.span, "spread-list", { type: t });
    }
    if (acc.size() + 1 > SIZE_LIMIT) return this.sizeFail(e.span);
    return this.depthLimit(listOf(acc.type()), e.span);
  }

  private map(e: A.MapLit, f: Facts): Type {
    let r: Rec = { fields: new Map(), rest: NEVER };
    for (const en of e.entries) {
      if (en.kind === "Spread") {
        let t = this.synth(en.value, f);
        if (hasNull(t)) {
          this.maybeNull(en.value.span, t);
          t = nonnull(t);
        }
        if (isAny(t)) {
          if (!this.site("dynamic", en.value.span, en.span)) r = merge(r, ANY);
        } else if (isNever(t)) {
          // nothing
        } else if (kindCount(t) === 1 && t.map) r = merge(r, t.map);
        else this.kind(en.value.span, "spread-map", { type: t });
        continue;
      }
      if (en.key.kind === "expr") {
        let k = this.synth(en.key.expr, f);
        const v = this.synth(en.value, f);
        if (hasNull(k)) {
          this.maybeNull(en.key.expr.span, k);
          k = nonnull(k);
        }
        const fit = fits(k, STRING);
        if (fit === 0) this.kind(en.key.expr.span, "key", { type: k });
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
    return this.sizeLimit(this.depthLimit({ map: r }, e.span), e.span);
  }

  private unary(e: A.Unary, f: Facts): Type {
    let t = this.synth(e.x, f);
    if (e.op === "!") {
      this.tested(t, e.x.span);
      return BOOL;
    }
    if (hasNull(t)) {
      this.maybeNull(e.x.span, t);
      t = nonnull(t);
    }
    if (isAny(t)) return this.site("dynamic", e.x.span, e.span) ? NEVER : ANY;
    if (fits(t, FLOAT) === 0) {
      this.kind(e.x.span, "operand", { op: "-", type: t });
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
        return this.sizeLimit(join(falsyPart(a), b), n.span);
      }
      case "||": {
        this.tested(a, n.x.span);
        const b = this.synth(n.y, this.withFacts(f, this.facts(n.x).f));
        return this.sizeLimit(join(truthyPart(a), b), n.span);
      }
      case "??":
        return this.sizeLimit(join(nonnull(a), this.synth(n.y, f)), n.span);
      case "==":
      case "!=": {
        const b = this.synth(n.y, f);
        if (!overlap(a, b) && !isNever(a) && !isNever(b) && !isExactlyNull(a) && !isExactlyNull(b)) this.say("never-equal", n.span, "equality", { left: printType(a), right: printType(b) });
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
      this.maybeNull(x.span, t);
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
        this.kind(n.x.span, "operand", { op, type: a.t });
        bad = true;
      }
      if (!fb && !sb) {
        this.kind(n.y.span, "operand", { op, type: b.t });
        bad = true;
      }
      if (!bad && ((fa && !sa && sb && !fb) || (sa && !fa && fb && !sb))) {
        this.kind(n.span, "operands", { op, left: a.t, right: b.t });
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
        this.kind(n.x.span, "operand", { op, type: a.t });
        bad = true;
      }
      if (fits(b.t, INT) === 0) {
        this.kind(n.y.span, "exponent", { type: b.t });
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
      this.kind(n.x.span, "operand", { op, type: a.t });
      bad = true;
    }
    if (fits(b.t, FLOAT) === 0) {
      this.kind(n.y.span, "operand", { op, type: b.t });
      bad = true;
    }
    if (op === "/") {
      if (n.y.kind === "Number" && n.y.value === 0) this.say("zero-divisor", n.y.span, "default");
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
      this.maybeNull(n.y.span, b);
      b = nonnull(b);
    }
    if (isAny(b)) {
      this.site("dynamic", n.y.span, n.span);
      return BOOL;
    }
    if (isNever(b)) return BOOL;
    if (kindCount(b) === 1 && b.list) {
      if (!overlap(a, b.list) && !isNever(a) && !isNever(b.list) && !isExactlyNull(a) && !isExactlyNull(b.list)) this.say("never-equal", n.span, "in", { left: printType(a), right: printType(b) });
      return BOOL;
    }
    if (kindCount(b) === 1 && (b.str || b.map)) {
      // The left side "must fit string; null is maybe-null" (8.4): with a list
      // on the right, a null left operand is allowed (questions file, ts S4).
      if (hasNull(a)) {
        this.maybeNull(n.x.span, a);
        a = nonnull(a);
      }
      const fit = fits(a, STRING);
      if (fit === 0) this.kind(n.x.span, "in-left", { op: opText(n.op), type: a });
      else if (fit === 2) this.site("dynamic", n.x.span, n.span);
      return BOOL;
    }
    this.kind(n.y.span, "in-right", { op: opText(n.op), type: b });
    return BOOL;
  }

  // ---- choice (11) ----------------------------------------------------------------

  private cond(e: A.Cond, f: Facts): Type {
    const conds: A.Cond[] = [];
    const thens: Type[] = [];
    let c: A.Expr = e;
    let facts = f;
    while (c.kind === "Cond") {
      conds.push(c);
      const tt = this.synth(c.test, facts);
      this.tested(tt, c.test.span);
      const fc = this.facts(c.test);
      thens.push(this.synth(c.then, this.withFacts(facts, fc.t)));
      facts = this.withFacts(facts, fc.f);
      c = c.else;
    }
    // Each Cond's type is the Join of the branches from it on: tested from the
    // innermost outward (05 20.7), with one accumulating Join.
    let acc = new TypeAcc();
    acc.add(this.synth(c, facts));
    for (let k = conds.length - 1; k >= 0; k--) {
      acc.add(thens[k]);
      if (acc.size() > SIZE_LIMIT) {
        this.say("check-limit", conds[k].span, "size");
        acc = new TypeAcc();
      }
    }
    return acc.type();
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
    const acc = new TypeAcc();
    let hasDefault = false;
    const covered = new Set<string | boolean | null | number>();
    // f05 5.3: a `match` on a field of p narrows p in each arm.
    const fp = this.fieldPath(e.subject);
    let pt: Type | undefined = fp === null ? undefined : this.types.get(fp.pNode);
    if (fp === null || pt === undefined || !this.fieldFactsHold(pt, fp.p)) pt = undefined;
    for (const arm of e.arms) {
      let listsNull = false;
      const strs = new Set<string>();
      for (const l of arm.literals) {
        const v = l.kind === "Null" ? null : l.value;
        if (l.kind === "Null") listsNull = true;
        // 11.2: a literal equal to an earlier one is duplicate-case only.
        if (covered.has(v)) continue;
        covered.add(v);
        if (l.kind === "String") strs.add(l.value);
        // 11.2, never-equal: the conditions of `==` (8.2).
        if (l.kind !== "Null" && !isNever(s) && !isExactlyNull(s) && !overlap(this.litType(l), s)) this.say("never-equal", l.span, "match", { type: printType(s), text: new TextAt(l.span) });
      }
      if (arm.isDefault) hasDefault = true;
      let armFacts = facts;
      if (pt !== undefined && fp !== null && !isAny(pt)) {
        const allStrings = arm.literals.length > 0 && arm.literals.every((l) => l.kind === "String");
        const hit = hitVariants(pt as Alts, fp.key, strs);
        if (allStrings && strs.size > 0 && hit.length > 0) armFacts = this.withFacts(facts, [this.typeFact("=", fp.p, variantsType(hit), true), fp.p]);
        if (strs.size > 0) {
          const all = allVariants(pt as Alts, fp.key, strs);
          if (all.length > 0) {
            facts = this.withFacts(facts, [this.typeFact("!", fp.p, variantsType(all), true)]);
            pt = without(pt, variantsType(all));
          }
        }
      }
      acc.add(this.synth(arm.result, armFacts));
      if (listsNull && path !== null) facts = this.withFacts(facts, [path]);
    }
    if (!hasDefault && !isNever(s)) {
      const values = finiteValues(s);
      if (values === null || values.some((v) => !covered.has(v))) this.say("non-exhaustive", e.span, "default", { type: printType(s) });
    }
    if (acc.size() > SIZE_LIMIT) return this.sizeFail(e.span);
    return acc.type();
  }

  private tryExpr(e: A.Try, f: Facts): Type {
    this.drop++;
    const b = this.synth(e.body, f);
    this.drop--;
    const slot = this.res.catchSlots.get(e);
    if (slot !== undefined) this.slotTypes[slot] = ERROR_TYPE;
    return this.sizeLimit(join(b, this.synth(e.handler, f)), e.span);
  }

  // ---- chains (9) -------------------------------------------------------------------

  private chain(e: A.Expr, f: Facts): Type {
    const r = this.chainRaw(e, f);
    return r.skipped ? withNull(r.type) : r.type;
  }

  /** A chain's type before the null of its optional steps is added, and whether one may skip (05 9.1). */
  private chainRaw(e: A.Expr, f: Facts): { type: Type; skipped: boolean } {
    const steps: A.Expr[] = [];
    let x: A.Expr = e.kind === "OptChain" ? e.x : e;
    for (;;) {
      if (x.kind === "Member" && this.res.constants.has(x)) break; // the read of a constant (f06 10 item 2)
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
    let skipped = false;
    let cur: Type;
    if (x.kind === "Each") {
      // A projection (f07 5.1): a first star `?.[*]` sets the chain's skipped.
      const r = this.eachType(x, f);
      cur = r.type;
      skipped = r.skipped;
    } else {
      // Both constants of the groups have type float (f06 8.2).
      cur = x.kind === "Member" && this.res.constants.has(x) ? FLOAT : x.kind === "MethodCall" ? this.call(x, f) : this.synth(x, f);
    }
    if (x.kind === "MethodCall" || x.kind === "Member" || x.kind === "Each") this.types.set(x, cur);
    let path: string | null = x.kind === "Ident" ? this.pathOf(x) : null;
    let recvSpan: Span = x.span;
    for (const s of steps) {
      const optional = (s as A.Member | A.Index | A.MethodCall).optional;
      if (hasNull(cur)) {
        if (optional) skipped = true;
        else this.maybeNull(recvSpan, cur);
        cur = nonnull(cur);
      }
      if (!isAny(cur) && !isNever(cur) && kindCount(cur) !== 1) {
        this.kind(recvSpan, "receiver", { type: cur });
        cur = NEVER;
      }
      // The field this step reads, when the step is a field read.
      const key = s.kind === "Member" ? s.name : s.kind === "Index" && s.index.kind === "String" ? s.index.value : null;
      const nextPath = path !== null && key !== null ? path + SEP + key : null;
      const present = nextPath !== null && f.has(nextPath);
      cur = this.step(s, cur, recvSpan, f, present);
      if (nextPath !== null && f.has(nextPath)) cur = nonnull(cur);
      if (nextPath !== null) cur = this.applyTypeFacts(cur, nextPath, f);
      path = nextPath;
      recvSpan = s.span;
      this.types.set(s, cur);
    }
    return { type: cur, skipped };
  }

  /** The walk of a projection (f07 5.1) and its type (5.2). `skipped`: a first star `?.[*]` may find null. */
  private eachType(e: A.Each, f: Facts): { type: Type; skipped: boolean } {
    let skipped = false;
    let skippedElement = false;
    let failed = false;
    let neverRecv = false;
    const ex = e.x!;
    let cur: Type;
    // The chain before the first star is one chain with the projection: its optional steps skip the whole.
    if (ex.kind === "Member" || ex.kind === "Index" || ex.kind === "MethodCall") {
      const r = this.chainRaw(ex, f);
      this.types.set(ex, r.skipped ? withNull(r.type) : r.type);
      cur = r.type;
      skipped = r.skipped;
    } else cur = this.synth(ex, f);
    // A star on *cur*: the element type, or null when the projection fails or meets never.
    const star = (p: A.Each, first: boolean): Type | null => {
      if (hasNull(cur)) {
        if (p.optional) {
          if (first) skipped = true;
          else skippedElement = true;
        } else this.maybeNull(p.recvSpan, cur);
        cur = nonnull(cur);
      }
      if (isAny(cur)) {
        if (this.site("dynamic", p.recvSpan, p.starSpan)) {
          failed = true;
          return null;
        }
        if (p.optional) {
          if (first) skipped = true;
          else skippedElement = true;
        }
        return ANY;
      }
      if (isNever(cur)) {
        if (first) neverRecv = true;
        return NEVER;
      }
      if (kindCount(cur) === 1 && cur.list) return cur.list;
      this.kind(p.recvSpan, "star", { type: cur });
      failed = true;
      return null;
    };
    const walk = (p: A.Each, first: boolean): void => {
      const elem = star(p, first);
      if (elem === null) {
        // Nothing more is reported for the reads; the index expressions are synthesized (05 9.4).
        this.synthReads(p, f);
        return;
      }
      cur = elem;
      let recvSpan = p.starSpan;
      for (const s of p.reads) {
        if (s.kind === "Each") {
          if (isNever(cur) && !failed) {
            // A later star on never: its elements are never, nothing is reported.
            this.synthReads(s, f);
            continue;
          }
          walk(s, false);
          return;
        }
        if (hasNull(cur)) {
          if (s.optional) skippedElement = true;
          else this.maybeNull(recvSpan, cur);
          cur = nonnull(cur);
        }
        if (!isAny(cur) && !isNever(cur) && kindCount(cur) !== 1) {
          this.kind(recvSpan, "receiver", { type: cur });
          cur = NEVER;
          failed = true;
        }
        const before = this.diags.length;
        const wasNever = isNever(cur);
        cur = this.step(s, cur, recvSpan, f, false);
        if (!wasNever && isNever(cur) && this.diags.length > before) failed = true;
        this.types.set(s, cur);
        recvSpan = s.span;
      }
    };
    walk(e, true);
    if (failed || neverRecv) return { type: NEVER, skipped };
    return { type: listOf(skippedElement ? withNull(cur) : cur), skipped };
  }

  /** Synthesize the index expressions of a projection's reads, at every level. */
  private synthReads(p: A.Each, f: Facts): void {
    for (const s of p.reads) {
      if (s.kind === "Index") this.synth(s.index, f);
      else if (s.kind === "Each") this.synthReads(s, f);
    }
  }

  private step(s: A.Expr, cur: Type, recvSpan: Span, f: Facts, present: boolean): Type {
    if (s.kind === "MethodCall") return this.methodCall(s, cur, recvSpan, f);
    const optional = (s as A.Member | A.Index).optional;
    if (s.kind === "Index") {
      if (isAny(cur)) {
        this.synth(s.index, f);
        if (this.site("dynamic", recvSpan, s.span)) return NEVER;
        // A plain index on an `any` receiver also records an `index` site (9.3).
        if (!optional) this.site("index", s.span);
        return ANY;
      }
      if (isNever(cur)) {
        this.synth(s.index, f);
        return NEVER;
      }
      if (s.index.kind === "String" && cur.map) {
        this.synth(s.index, f);
        return this.readKey(cur.map, s.index.value, optional, s.index.span, recvSpan, present, [s.span], cur);
      }
      let k = this.synth(s.index, f);
      const need = cur.map ? STRING : INT;
      if (cur.map || cur.list || cur.str) {
        if (hasNull(k)) {
          this.maybeNull(s.index.span, k);
          k = nonnull(k);
        }
        const fit = fits(k, need);
        if (fit === 0) this.kind(s.index.span, cur.map ? "key" : "index", { type: k });
        else if (fit === 2) this.site("dynamic", s.index.span, s.span);
      }
      if (cur.map) {
        const r = cur.map;
        // 9.2: a key that is not a string is `kind` on the index only.
        if (fits(k, STRING) === 0 && !isNever(k)) return NEVER;
        const v = recordValues(r);
        if (optional) return withNull(v);
        if (isAny(r.rest)) this.site("dynamic", recvSpan, s.span);
        else this.say("maybe-absent", s.index.span, "index", { type: printType(cur) });
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
      this.kind(recvSpan, "index-read", { type: cur });
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
    if (cur.map) return this.readKey(cur.map, m.name, optional, m.nameSpan, recvSpan, present, [s.span], cur);
    this.kind(recvSpan, "field-read", { type: cur });
    return NEVER;
  }

  /** Read a field of a record (9.2), plain or as by `?.`. */
  private readKey(r: Rec, key: string, optional: boolean, keySpan: Span, recvSpan: Span, present: boolean, owners: Span[], whole: Type = { map: r }): Type {
    const fl = r.fields.get(key);
    if (fl !== undefined && (!fl.optional || present)) return fl.type;
    if (fl !== undefined || (!isNever(r.rest) && !isAny(r.rest))) {
      const t = fl !== undefined ? fl.type : r.rest;
      if (present) return t;
      if (optional) return withNull(t);
      this.say("maybe-absent", keySpan, "field", { key, type: printType(whole) });
      return t;
    }
    if (isAny(r.rest)) {
      if (optional || present) return ANY;
      return this.site("dynamic", recvSpan, ...owners) ? NEVER : ANY;
    }
    this.say("no-field", keySpan, "default", { key, type: printType(whole) });
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
      // A method of that name marked `!` somewhere: a `value` or `host` site on the call (13.3).
      this.siteName = s.name;
      if ([...this.env.libraryMethods.values()].some((m) => m.get(s.name)?.sig.canFail === true)) this.site("value", s.span);
      if ([...this.env.methods.values()].some((m) => m.get(s.name)?.sig.canFail === true)) this.site("host", s.span);
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
      this.say("no-method", s.nameSpan, "default", { type: printType(cur), name: s.name });
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
    const callName = isMethod ? (node as A.MethodCall).name : sig.name;
    if (args.length < min || (!rest && args.length > max)) {
      if (isMethod) this.say("arity", span, rest ? "at-least" : min === max ? "exact" : "range", { name: callName, count: args.length, required: min, most: max });
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
    // A solution over the size limit stops the call (05 20.7): check-limit on it.
    let over = false;
    const gain = (v: string, t: Type) => {
      const x = sol.has(v) ? join(sol.get(v)!, t) : t;
      if (typeSize(x) > SIZE_LIMIT) over = true;
      sol.set(v, x);
    };
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
        else if (isMethod) this.say("not-a-value", la.span, "argument", { param: p.name, name: callName });
        return;
      }
      const t = this.synth(a as A.Expr, f);
      argTypes.push(t);
      if (p.type.t === "fn") {
        this.say("arg-mismatch", a.span, "function", { param: p.name, name: callName });
        return;
      }
      this.bind(paramType(i)!, t, gain);
      if (sig.tested.includes(i)) this.tested(t, a.span);
      if (sig.renders.includes(i)) this.rendered(a as A.Expr, t, [span]);
    });
    if (over) return this.sizeFail(span);
    // 3. Lambda arguments.
    for (const { i, la } of lambdas) {
      if (this.drop === 0) this.callbacks.push({ body: sp(la.lambda.body.span), call: sp(span) });
      const fn = paramAt(i).type as Extract<SigNode["params"][number]["type"], { t: "fn" }>;
      if (la.lambda.params.length > fn.params.length) {
        if (isMethod) this.say("arity", la.span, "parameters", { name: callName, most: fn.params.length });
        continue;
      }
      const qs = fn.params.map((q) => q.type);
      if (qs.slice(0, la.lambda.params.length).some((q) => varsOf(q).some((v) => !sol.has(v)))) {
        this.say("cannot-infer", la.span, "default", { name: callName });
        continue;
      }
      const inst = (n: TypeNode) => fromNode(n, { opaque: this.env.opaque, named: this.env.named, variable: (v) => sol.get(v) ?? NEVER });
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
      // 3a. The seed rule of 13.5, also on a fit that used any.
      const qVars = new Set(qs.flatMap(varsOf));
      const seed = varsOf(r).find((v) => qVars.has(v) && before.has(v) && fits(b!.type, before.get(v)!) !== 1);
      if (seed !== undefined) {
        this.rollback(m);
        sol.set(seed, join(before.get(seed)!, b.type));
        if (typeSize(sol.get(seed)!) > SIZE_LIMIT) return this.sizeFail(span);
        b = this.lambdaUse(la, qs.map(inst), f, level, true);
        if (b === null) continue;
        // No bind on the second check (13.5).
      } else {
        this.bind(r, b.type, gain);
        if (over) return this.sizeFail(span);
      }
      // The body always fits R, every Var by its solution (13.5).
      const want = inst(r);
      if (fits(b.type, want) === 0) this.say("arg-mismatch", la.lambda.body.span, "body", { name: callName, found: printType(b.type), expected: printType(want) });
      else this.fitSimple(b.type, want, la.lambda.body.span, "arg-mismatch");
    }
    // 4. Bounds.
    for (const bd of sig.sig.bounds) {
      const t = sol.get(bd.name) ?? NEVER;
      const fit: Fit = bd.bound === "number" ? fits(t, FLOAT) : maxFit(fits(t, FLOAT), fits(t, STRING));
      if (fit === 0) this.say("arg-mismatch", span, bd.bound === "number" ? "number" : "ordered", { name: callName, found: printType(t) });
      else if (fit === 2) this.site("dynamic", span, span);
    }
    // 5. Fit, through the form.
    const inst = (n: TypeNode) => fromNode(n, { opaque: this.env.opaque, named: this.env.named, variable: (v) => sol.get(v) ?? NEVER });
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
    if (sig.sig.canFail) {
      this.siteName = callName;
      this.site(sig.kind === "library" ? "value" : "host", span);
    }
    // 13.8: filter and find narrow.
    const nw = this.narrowed;
    this.narrowed = null;
    if (sig.narrows && nw !== null) {
      const n = this.narrowedType(nw.la, nw.elem);
      result = sig.name === "find" ? withNull(n) : listOf(n);
    }
    return this.sizeLimit(this.depthLimit(result, span), span);
  }

  private narrowed: { la: LambdaArg; elem: Type } | null = null;

  private narrowedType(la: LambdaArg, elem: Type): Type {
    const p0 = la.lambda.params[0];
    if (p0 === undefined || p0.pattern.kind !== "NamePat" || p0.pattern.name === null) return elem;
    const slot = this.res.slots.get(p0.pattern);
    if (slot === undefined) return elem;
    const root = `s${slot}`;
    const t = this.facts(la.lambda.body).t;
    const out = t.some((k) => k === root || k.startsWith(root + SEP)) ? nonnull(elem) : elem;
    // f04 6.5: the facts of `is` about the parameter.
    return this.applyTypeFacts(out, root, new Set(t));
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
    // f05 4.3 item 4: a union of records binds each variant against A's record.
    if (p.t === "union" && a.map && p.alts.every((x) => x.t === "record" || x.t === "name" || (x.t === "word" && x.word === "null"))) {
      for (const x of p.alts) if (x.t === "record") this.bind(x, nonnull(a), gain);
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

  /** A place of failure (f08 3.5 item 1): `maybe-null`, or the mismatch with the type there and the type expected. */
  private fitFail(code: string, span: Span, place: Type, want: Type): void {
    if (code === "maybe-null") this.maybeNull(span, place);
    else this.say(code, span, code === "result-mismatch" ? "result" : "argument", { expected: printType(want), found: printType(place) });
  }

  /** e must fit T, not through its form: code at e. */
  private fitSimple(t: Type, want: Type, span: Span, code: string): void {
    const fit = fits(t, want);
    if (fit === 0) this.fitFail(code, span, t, want);
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
          if (sf === 0) this.fitFail(code(st, listOf(elem)), it.value.span, st, listOf(elem));
          else if (sf === 2) this.site("dynamic", it.value.span, ...owners);
        } else this.fitForm(it, elem, code, undefined, owners);
      }
      return;
    }
    if (e.kind === "Map" && want.map && e.entries.every((en) => en.kind === "Spread" || en.key.kind !== "expr")) {
      if (want.union) this.fitUnionMap(e, t, want, want.union, code, place, owners);
      else this.fitMap(e, t, want, want.map, code, place, owners);
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
    this.fitFail(code(t, want), place, t, want);
  }

  /** A map literal with no computed key against a tagged union (f05 6): the tag's contributors choose the variant. */
  private fitUnionMap(e: A.MapLit, t: Type, want: Type, u: Union, code: (place: Type, want: Type) => string, place: Span, owners: Span[]): void {
    const tag = u.tag;
    type Contributor = { span: Span; type: Type; sure: boolean };
    const cs: Contributor[] = [];
    for (const en of e.entries) {
      if (en.kind === "Entry") {
        const key = en.key.kind === "word" ? en.key.text : (en.key as { value: string }).value;
        if (key !== tag) continue;
        cs.push({ span: en.shorthand ? (en.key as { span: Span }).span : en.value.span, type: this.types.get(en.value) ?? NEVER, sure: true });
        continue;
      }
      const m = nonnull(this.types.get(en.value) ?? NEVER);
      if (isAny(m)) cs.push({ span: en.value.span, type: ANY, sure: false });
      else if (!isNever(m) && kindCount(m) === 1 && m.map) {
        const fl = m.map.fields.get(tag);
        if (fl !== undefined) cs.push({ span: en.value.span, type: fl.type, sure: !fl.optional });
        else if (!isNever(m.map.rest)) cs.push({ span: en.value.span, type: m.map.rest, sure: false });
      }
    }
    let first = -1;
    for (let i = cs.length - 1; i >= 0; i--) {
      if (cs[i].sure) {
        first = i;
        break;
      }
    }
    if (first < 0) {
      this.say(code(t, want) === "maybe-null" ? "arg-mismatch" : code(t, want), place, "missing", { key: tag });
      return;
    }
    const contributors = cs.slice(first);
    const j = joinAll(contributors.map((c) => c.type));
    const v = !isAny(j) && j.str !== undefined && j.str !== "string" && kindCount(j) === 1
      ? u.variants.find((x) => [...(j.str as ReadonlySet<string>)].every((y) => tagSetOf(x, tag).has(y)))
      : undefined;
    if (v === undefined) {
      const tagType: Alts = { str: new Set(u.variants.flatMap((x) => [...tagSetOf(x, tag)])) };
      for (const c of contributors) this.fitFail(code(c.type, tagType), c.span, c.type, tagType);
      return;
    }
    this.fitMap(e, t, want, v, code, place, owners);
  }

  /** A map literal with no computed key against a record (15.1, "A map literal against a record"). */
  private fitMap(e: A.MapLit, t: Type, want: Type, r: Rec, code: (place: Type, want: Type) => string, place: Span, owners: Span[]): void {
    type Item = { written: A.Entry; key: string } | { spread: A.Spread; m: Rec | "any" | null };
    const items: Item[] = e.entries.map((en): Item => {
      if (en.kind === "Entry") return { written: en, key: en.key.kind === "word" ? en.key.text : (en.key as { value: string }).value };
      const m = nonnull(this.types.get(en.value) ?? NEVER);
      if (isAny(m)) return { spread: en, m: "any" };
      if (!isNever(m) && kindCount(m) === 1 && m.map) return { spread: en, m: m.map };
      return { spread: en, m: null };
    });
    // What an entry gives for a key (null: the other key).
    const gives = (it: Item, k: string | null): { sure: boolean; type: Type } | null => {
      if ("written" in it) return k !== null && it.key === k ? { sure: true, type: this.types.get(it.written.value) ?? NEVER } : null;
      if (it.m === null) return null;
      if (it.m === "any") return { sure: false, type: ANY };
      const fl = k === null ? undefined : it.m.fields.get(k);
      if (fl !== undefined) return { sure: !fl.optional, type: fl.type };
      return isNever(it.m.rest) ? null : { sure: false, type: it.m.rest };
    };
    const keys = new Set<string>(r.fields.keys());
    for (const it of items) {
      if ("written" in it) keys.add(it.key);
      else if (it.m !== null && it.m !== "any") for (const k of it.m.fields.keys()) keys.add(k);
    }
    // Each required field of R is surely given by some entry: else the whole of e, once.
    // f08 3.5 item 2: the least key by code point of the required fields with no entry.
    const missing = [...r.fields].filter(([k, fl]) => !fl.optional && !items.some((it) => gives(it, k)?.sure === true)).map(([k]) => k).sort(compareText);
    if (missing.length > 0) this.say(code(t, want) === "maybe-null" ? "arg-mismatch" : code(t, want), place, "missing", { key: missing[0] });
    const reported = new Set<A.Spread>();
    for (const k of [...keys, null]) {
      const gs = items.map((it) => gives(it, k));
      let first = 0;
      for (let i = gs.length - 1; i >= 0; i--) {
        if (gs[i]?.sure === true) {
          first = i;
          break;
        }
      }
      const fl = k === null ? undefined : r.fields.get(k);
      const rt = fl !== undefined ? fl.type : r.rest;
      for (let i = first; i < items.length; i++) {
        const g = gs[i];
        if (g === null) continue;
        const it = items[i];
        if ("written" in it) {
          const en = it.written;
          if (fl === undefined && isNever(r.rest)) {
            this.say(code(NEVER, NEVER) === "result-mismatch" ? "result-mismatch" : "arg-mismatch", (en.key as { span: Span }).span, "extra", { expected: printType({ map: r }), key: it.key });
            continue;
          }
          this.fitForm(en.value, rt, code, en.shorthand ? (en.key as { span: Span }).span : undefined, owners);
          continue;
        }
        if (reported.has(it.spread)) continue;
        const fit = fits(g.type, rt);
        if (fit === 0) {
          reported.add(it.spread);
          this.fitFail(code(g.type, rt), it.spread.value.span, g.type, rt);
        } else if (fit === 2 && it.m !== "any") this.site("dynamic", it.spread.value.span, ...owners);
      }
    }
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
      if (this.uses > 1000) this.say("check-limit", span, "bodies");
      else this.say("check-limit", span, "nesting", { name: this.res.lambdaNames?.get(this.res.lambdas.indexOf(l)) ?? "" });
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
        t = a.given ? this.sizeLimit(join(nonnull(a.type), d), p.pattern.span) : d;
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

/** The message of a check diagnostic, from its template (f08 3.5). */
function checkMessage(d: DiagRec, source: Source | undefined): string {
  if (d.template === undefined) return d.code.replace(/-/g, " ");
  const params: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(d.params ?? {})) params[k] = v instanceof TextAt ? (source === undefined ? "" : source.slice(v.span.start, v.span.end)) : v;
  return catalogMessage(d.code, d.template, params);
}

export { mapOf, opaque, alternatives, joinAll };
export type { Alts };

/** Whether a parameter value is a type, to be printed (f08 3.2 item 2). */
function isTypeLike(v: unknown): boolean {
  return typeof v === "object" && v !== null && !(v instanceof TextAt) && !Array.isArray(v);
}
