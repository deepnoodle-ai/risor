// Types (05-types.md sections 2 to 4): sets of alternatives with one slot
// for each kind, records, `any` and `never`, and the operations Join, Fits,
// Merge, Truthy, Falsy, the helpers, printing and reading type text.

import type { TypeNode } from "./typetext.ts";
import { compareText, jsonString } from "../value/value.ts";

export interface Field {
  /** The field's type: Join(base, fill) when it carries a fill. */
  type: Type;
  optional: boolean;
  /** Package F1 (f01 4.4): a fill kept apart from the base until the meets are done. */
  fill?: Type;
  base?: Type;
}

/** A record (2.2). `rest` is NEVER for an exact record, ANY for an open one. */
export interface Rec {
  fields: ReadonlyMap<string, Field>;
  rest: Type;
  /** The mark of package F1's option `closed` (f01 4.1): the rest becomes never at Close. */
  closable?: true;
}

export interface Alts {
  readonly any?: false;
  readonly null?: true;
  readonly bool?: true;
  readonly num?: "int" | "float";
  /** `string`, or a literal set. */
  readonly str?: "string" | ReadonlySet<string>;
  /** The element type of a list. */
  readonly list?: Type;
  readonly map?: Rec;
  /** Opaque type names. */
  readonly opaque?: ReadonlySet<string>;
}

export interface AnyType {
  readonly any: true;
}

export type Type = AnyType | Alts;

export const ANY: AnyType = { any: true };
export const NEVER: Alts = {};
export const NULL: Alts = { null: true };
export const BOOL: Alts = { bool: true };
export const INT: Alts = { num: "int" };
export const FLOAT: Alts = { num: "float" };
export const STRING: Alts = { str: "string" };

export function isAny(t: Type): t is AnyType {
  return t.any === true;
}

export function isNever(t: Type): boolean {
  if (isAny(t)) return false;
  return !t.null && !t.bool && !t.num && !t.str && !t.list && !t.map && !(t.opaque && t.opaque.size > 0);
}

export function lit(...values: string[]): Alts {
  return { str: new Set(values) };
}

export function listOf(e: Type): Alts {
  return { list: e };
}

export function record(fields: [string, Field][], rest: Type = NEVER): Alts {
  return { map: { fields: new Map(fields), rest } };
}

export function mapOf(rest: Type): Alts {
  return { map: { fields: new Map(), rest } };
}

export function opaque(name: string): Alts {
  return { opaque: new Set([name]) };
}

export function hasNull(t: Type): boolean {
  return !isAny(t) && t.null === true;
}

/** The alternatives of a type, as one-kind types (no `any`). */
export function alternatives(t: Alts): Alts[] {
  const out: Alts[] = [];
  if (t.bool) out.push(BOOL);
  if (t.num) out.push({ num: t.num });
  if (t.str) out.push({ str: t.str });
  if (t.list) out.push({ list: t.list });
  if (t.map) out.push({ map: t.map });
  if (t.opaque) for (const n of [...t.opaque].sort(compareText)) out.push({ opaque: new Set([n]) });
  if (t.null) out.push(NULL);
  return out;
}

/** The number of kinds of a type: each opaque name is a kind of its own. */
export function kindCount(t: Alts): number {
  return alternatives(t).length;
}

// ---- Join (4.1) -----------------------------------------------------------------

/** A key in both records of a Join; a fill is kept apart (f01 4.4 item 3). */
function joinField(fa: Field, fb: Field): Field {
  const type = join(fa.type, fb.type);
  if (fa.fill === undefined && fb.fill === undefined) return { type, optional: fa.optional || fb.optional };
  const fill = fa.fill === undefined ? fb.fill! : fb.fill === undefined ? fa.fill : join(fa.fill, fb.fill);
  return { type, optional: false, base: join(fa.base ?? fa.type, fb.base ?? fb.type), fill };
}

/** A key in one record of a Join only: its base joined with the other's rest; it keeps its fill. */
function oneSide(f: Field, rest: Type): Field {
  const type = isNever(rest) ? f.type : join(f.type, rest);
  if (f.fill === undefined) return { type, optional: true };
  return { type, optional: false, base: isNever(rest) ? f.base! : join(f.base!, rest), fill: f.fill };
}

function joinStr(a: "string" | ReadonlySet<string> | undefined, b: "string" | ReadonlySet<string> | undefined): "string" | ReadonlySet<string> | undefined {
  if (a === undefined) return b;
  if (b === undefined) return a;
  if (a === "string" || b === "string") return "string";
  return new Set([...a, ...b]);
}

function joinRec(a: Rec, b: Rec): Rec {
  const fields = new Map<string, Field>();
  for (const [k, fa] of a.fields) {
    const fb = b.fields.get(k);
    if (fb !== undefined) fields.set(k, joinField(fa, fb));
    else fields.set(k, oneSide(fa, b.rest));
  }
  for (const [k, fb] of b.fields) {
    if (a.fields.has(k)) continue;
    fields.set(k, oneSide(fb, a.rest));
  }
  const out: Rec = { fields, rest: join(a.rest, b.rest) };
  // f01 3.6 item 8: a Join of two records is closable when one is closable and the other closable or exact.
  if ((a.closable && (b.closable || isNever(b.rest))) || (b.closable && isNever(a.rest))) out.closable = true;
  return out;
}

const joinMemo = new WeakMap<object, WeakMap<object, Type>>();

export function join(a: Type, b: Type): Type {
  if (isAny(a) || isAny(b)) return ANY;
  if (a === b) return a;
  // A type is held with shared parts: the Join of two is kept, so joining
  // two records that share parts does no work twice (05 20.7, ruling CG5).
  if ((a.list || a.map) && (b.list || b.map)) {
    let m = joinMemo.get(a);
    const hit = m?.get(b);
    if (hit !== undefined) return hit;
    const out = join0(a, b);
    if (m === undefined) joinMemo.set(a, (m = new WeakMap()));
    m.set(b, out);
    return out;
  }
  return join0(a, b);
}

function join0(a: Alts, b: Alts): Type {
  const out: { -readonly [K in keyof Alts]: Alts[K] } = {};
  if (a.null || b.null) out.null = true;
  if (a.bool || b.bool) out.bool = true;
  if (a.num || b.num) out.num = a.num === "float" || b.num === "float" ? "float" : "int";
  const s = joinStr(a.str, b.str);
  if (s !== undefined) out.str = s;
  if (a.list && b.list) out.list = join(a.list, b.list);
  else if (a.list || b.list) out.list = a.list ?? b.list;
  if (a.map && b.map) out.map = joinRec(a.map, b.map);
  else if (a.map || b.map) out.map = a.map ?? b.map;
  if (a.opaque || b.opaque) out.opaque = new Set([...(a.opaque ?? []), ...(b.opaque ?? [])]);
  return out;
}

/** The Join of many types; literal sets are merged in one pass (ruling CG10). */
export function joinAll(ts: Type[]): Type {
  const acc = new TypeAcc();
  for (const x of ts) acc.add(x);
  return acc.type();
}

/**
 * An accumulating Join (ruling CG10): the literal sets go into one mutable
 * set, so joining n types is at most n log n, and the size of the Join so far
 * is known at any time without building it.
 */
export class TypeAcc {
  private other: Alts = NEVER;
  private readonly lits = new Set<string>();
  private str = false;
  private anyType = false;

  add(t: Type): void {
    if (isAny(t)) {
      this.anyType = true;
      return;
    }
    if (this.anyType) return;
    if (t.str !== undefined) {
      if (t.str === "string") this.str = true;
      else if (!this.str) for (const x of t.str) this.lits.add(x);
    }
    if (t.null || t.bool || t.num || t.list || t.map || (t.opaque && t.opaque.size > 0)) {
      const { str: _, ...rest } = t;
      this.other = join(this.other, rest) as Alts;
    }
  }

  size(): number {
    if (this.anyType) return 1;
    const sum = altSum(this.other) + (this.str ? 1 : this.lits.size);
    return sum === 0 ? 1 : Math.min(sum, SIZE_LIMIT + 1);
  }

  type(): Type {
    if (this.anyType) return ANY;
    if (!this.str && this.lits.size === 0) return this.other;
    return { ...this.other, str: this.str ? "string" : new Set(this.lits) };
  }
}

// ---- Size (05 20.7, ruling CG) -------------------------------------------------------

/** The limit on the size of a type (05 20.7). */
export const SIZE_LIMIT = 100_000;
const sizeMemo = new WeakMap<object, number>();

function sat(n: number): number {
  return n > SIZE_LIMIT ? SIZE_LIMIT + 1 : n;
}

/** The sum of the sizes of a type's alternatives. */
function altSum(t: Alts): number {
  const m = sizeMemo.get(t);
  if (m !== undefined) return m;
  let n = 0;
  if (t.null) n++;
  if (t.bool) n++;
  if (t.num) n++;
  if (t.str) n += t.str === "string" ? 1 : t.str.size;
  if (t.opaque) n += t.opaque.size;
  if (t.list) n = sat(n + 1 + typeSize(t.list));
  if (t.map) n = sat(n + recSize(t.map));
  n = sat(n);
  sizeMemo.set(t, n);
  return n;
}

function recSize(r: Rec): number {
  const m = sizeMemo.get(r);
  if (m !== undefined) return m;
  let n = 1 + (isNever(r.rest) ? 0 : typeSize(r.rest));
  for (const f of r.fields.values()) {
    n = sat(n + typeSize(f.type));
    if (n > SIZE_LIMIT) break;
  }
  n = sat(n);
  sizeMemo.set(r, n);
  return n;
}

/** The size of a type (05 20.7), saturating at SIZE_LIMIT + 1. Shared parts are counted once each time they stand, never expanded. */
export function typeSize(t: Type): number {
  if (isAny(t)) return 1;
  const n = altSum(t);
  return n === 0 ? 1 : n;
}

// ---- Fits (4.2) -----------------------------------------------------------------

/** 0: does not fit. 1: fits. 2: fits, and the fit used any. */
export type Fit = 0 | 1 | 2;

function both(a: Fit, b: Fit): Fit {
  if (a === 0 || b === 0) return 0;
  return a === 2 || b === 2 ? 2 : 1;
}

const fitsMemo = new WeakMap<object, WeakMap<object, Fit>>();

export function fits(s: Type, t: Type): Fit {
  if (isAny(t)) return 1;
  if (isAny(s)) return 2;
  if (s === t && !hasAny(s)) return 1;
  if ((s.list || s.map) && (t.list || t.map)) {
    let m = fitsMemo.get(s);
    const hit = m?.get(t);
    if (hit !== undefined) return hit;
    const out = fits0(s, t);
    if (m === undefined) fitsMemo.set(s, (m = new WeakMap()));
    m.set(t, out);
    return out;
  }
  return fits0(s, t);
}

function fits0(s: Alts, t: Alts): Fit {
  if (isNever(s)) return 1;
  let r: Fit = 1;
  if (s.null && !t.null) return 0;
  if (s.bool && !t.bool) return 0;
  if (s.num) {
    if (!t.num) return 0;
    if (s.num === "float" && t.num === "int") return 0;
  }
  if (s.str) {
    if (!t.str) return 0;
    if (s.str === "string") {
      if (t.str !== "string") return 0;
    } else if (t.str !== "string") {
      const ts = t.str;
      for (const x of s.str) if (!ts.has(x)) return 0;
    }
  }
  if (s.list) {
    if (!t.list) return 0;
    r = both(r, fits(s.list, t.list));
    if (r === 0) return 0;
  }
  if (s.opaque) {
    for (const n of s.opaque) if (!t.opaque || !t.opaque.has(n)) return 0;
  }
  if (s.map) {
    if (!t.map) return 0;
    r = both(r, fitsRec(s.map, t.map));
  }
  return r;
}

function fitsRec(s: Rec, t: Rec): Fit {
  let r: Fit = 1;
  for (const [k, tf] of t.fields) {
    const sf = s.fields.get(k);
    if (sf !== undefined) {
      if (sf.optional && !tf.optional) return 0;
      r = both(r, fits(sf.type, tf.type));
    } else {
      if (!tf.optional) return 0;
      r = both(r, fits(s.rest, tf.type));
    }
    if (r === 0) return 0;
  }
  for (const [k, sf] of s.fields) {
    if (t.fields.has(k)) continue;
    r = both(r, fits(sf.type, t.rest));
    if (r === 0) return 0;
  }
  return both(r, fits(s.rest, t.rest));
}

// ---- Merge (4.3) ----------------------------------------------------------------

export function merge(r: Rec, m: Rec | AnyType): Rec {
  if ("any" in m && m.any === true) {
    const fields = new Map<string, Field>();
    for (const [k, f] of r.fields) fields.set(k, { type: ANY, optional: f.optional });
    return { fields, rest: ANY };
  }
  const mm = m as Rec;
  const fields = new Map<string, Field>();
  for (const [k, rf] of r.fields) {
    if (mm.fields.has(k)) continue;
    fields.set(k, isNever(mm.rest) ? rf : { type: join(rf.type, mm.rest), optional: rf.optional });
  }
  for (const [k, mf] of mm.fields) {
    if (!mf.optional) {
      fields.set(k, { type: mf.type, optional: false });
      continue;
    }
    const rf = r.fields.get(k);
    const rt = rf !== undefined ? rf.type : r.rest;
    fields.set(k, { type: join(mf.type, rt), optional: !(rf !== undefined && !rf.optional) });
  }
  return { fields, rest: join(r.rest, mm.rest) };
}

// ---- Truthy and Falsy (4.4) -----------------------------------------------------

export function truthyPart(t: Type): Type {
  if (isAny(t)) return ANY;
  const out: { -readonly [K in keyof Alts]: Alts[K] } = {};
  if (t.bool) out.bool = true;
  if (t.num) out.num = t.num;
  if (t.str) {
    if (t.str === "string") out.str = "string";
    else {
      const s = new Set([...t.str].filter((x) => x !== ""));
      if (s.size > 0) out.str = s;
    }
  }
  if (t.list && !isNever(t.list)) out.list = t.list;
  if (t.map && !(t.map.fields.size === 0 && isNever(t.map.rest))) out.map = t.map;
  if (t.opaque && t.opaque.size > 0) out.opaque = t.opaque;
  return out;
}

export function falsyPart(t: Type): Type {
  if (isAny(t)) return ANY;
  const out: { -readonly [K in keyof Alts]: Alts[K] } = {};
  if (t.null) out.null = true;
  if (t.bool) out.bool = true;
  if (t.num) out.num = t.num;
  if (t.str && (t.str === "string" || t.str.has(""))) out.str = new Set([""]);
  if (t.list) out.list = NEVER;
  if (t.map && ![...t.map.fields.values()].some((f) => !f.optional)) out.map = { fields: new Map(), rest: NEVER };
  return out;
}

// ---- Helpers (4.5) ----------------------------------------------------------------

export function nonnull(t: Type): Type {
  if (isAny(t) || !t.null) return t;
  const { null: _, ...rest } = t;
  return rest;
}

export function withNull(t: Type): Type {
  if (isAny(t)) return t;
  return { ...t, null: true };
}

export function overlap(a: Type, b: Type): boolean {
  if (isNever(a) || isNever(b)) return false;
  if (isAny(a) || isAny(b)) return true;
  if (a.null && b.null) return true;
  if (a.bool && b.bool) return true;
  if (a.num && b.num) return true;
  if (a.str && b.str) {
    if (a.str === "string" || b.str === "string") return true;
    for (const x of a.str) if ((b.str as ReadonlySet<string>).has(x)) return true;
  }
  if (a.list && b.list) return true;
  if (a.map && b.map) return true;
  if (a.opaque && b.opaque) for (const n of a.opaque) if (b.opaque.has(n)) return true;
  return false;
}

const opaqueMemo = new WeakMap<object, boolean>();
const anyMemo = new WeakMap<object, boolean>();

export function hasOpaque(t: Type, depth = 0): boolean {
  if (isAny(t) || depth > 200) return false;
  const m = opaqueMemo.get(t);
  if (m !== undefined) return m;
  const out = hasOpaque0(t, depth);
  opaqueMemo.set(t, out);
  return out;
}

function hasOpaque0(t: Alts, depth: number): boolean {
  if (t.opaque && t.opaque.size > 0) return true;
  if (t.list && hasOpaque(t.list, depth + 1)) return true;
  if (t.map) {
    for (const f of t.map.fields.values()) if (hasOpaque(f.type, depth + 1)) return true;
    if (hasOpaque(t.map.rest, depth + 1)) return true;
  }
  return false;
}

/** has-any(T) (ruling AO2): T is `any`, or an element, field or rest type of T has `any`. */
export function hasAny(t: Type, depth = 0): boolean {
  if (isAny(t)) return true;
  if (depth > 200) return false;
  const m = anyMemo.get(t);
  if (m !== undefined) return m;
  const out = hasAny0(t, depth);
  anyMemo.set(t, out);
  return out;
}

function hasAny0(t: Alts, depth: number): boolean {
  if (t.list && hasAny(t.list, depth + 1)) return true;
  if (t.map) {
    for (const f of t.map.fields.values()) if (hasAny(f.type, depth + 1)) return true;
    if (hasAny(t.map.rest, depth + 1)) return true;
  }
  return false;
}

/** finite(T) and its values (4.5): null as null, true and false, each string. null if T is not finite. */
export function finiteValues(t: Type): (string | boolean | null)[] | null {
  if (isAny(t)) return null;
  if (t.num || t.list || t.map || (t.opaque && t.opaque.size > 0) || t.str === "string") return null;
  const out: (string | boolean | null)[] = [];
  if (t.null) out.push(null);
  if (t.bool) out.push(true, false);
  if (t.str) for (const s of t.str) out.push(s);
  return out;
}

/** The depth of a type (2.5). */
const depthMemo = new WeakMap<object, number>();

export function depth(t: Type, limit = 1000): number {
  if (isAny(t) || limit <= 0) return 0;
  const m = depthMemo.get(t);
  if (m !== undefined) return m;
  const out = depth0(t, limit);
  depthMemo.set(t, out);
  return out;
}

function depth0(t: Alts, limit: number): number {
  let d = 0;
  if (t.list) d = Math.max(d, 1 + depth(t.list, limit));
  if (t.map) {
    let m = depth(t.map.rest, limit);
    for (const f of t.map.fields.values()) m = Math.max(m, depth(f.type, limit));
    d = Math.max(d, 1 + m);
  }
  return d;
}

/** The join of a record's field types and its rest. */
export function recordValues(r: Rec): Type {
  let t = r.rest;
  for (const f of r.fields.values()) t = join(t, f.type);
  return t;
}

// ---- Equality, for comparing a case's `type` --------------------------------------

export function typeEqual(a: Type, b: Type): boolean {
  if (isAny(a) || isAny(b)) return isAny(a) && isAny(b);
  if (!!a.null !== !!b.null || !!a.bool !== !!b.bool || a.num !== b.num) return false;
  if ((a.str === undefined) !== (b.str === undefined)) return false;
  if (a.str !== undefined) {
    if ((a.str === "string") !== (b.str === "string")) return false;
    if (a.str !== "string") {
      const x = a.str;
      const y = b.str as ReadonlySet<string>;
      if (x.size !== y.size || [...x].some((s) => !y.has(s))) return false;
    }
  }
  if ((a.list === undefined) !== (b.list === undefined)) return false;
  if (a.list && !typeEqual(a.list, b.list!)) return false;
  const ao = a.opaque ?? new Set<string>();
  const bo = b.opaque ?? new Set<string>();
  if (ao.size !== bo.size || [...ao].some((n) => !bo.has(n))) return false;
  if ((a.map === undefined) !== (b.map === undefined)) return false;
  if (a.map) {
    const x = a.map;
    const y = b.map!;
    if (x.fields.size !== y.fields.size || !typeEqual(x.rest, y.rest)) return false;
    for (const [k, f] of x.fields) {
      const g = y.fields.get(k);
      if (g === undefined || g.optional !== f.optional || !typeEqual(f.type, g.type)) return false;
    }
  }
  return true;
}

// ---- Printing (3.9) -----------------------------------------------------------------

const WORD = /^[A-Za-z_][A-Za-z0-9_]*$/;

function altTexts(t: Alts): string[] {
  const out: string[] = [];
  if (t.bool) out.push("bool");
  if (t.num) out.push(t.num);
  if (t.str) {
    if (t.str === "string") out.push("string");
    else out.push(...[...t.str].sort(compareText).map(jsonString));
  }
  if (t.list) {
    const e = printType(t.list);
    out.push((topLevelUnion(t.list) ? `(${e})` : e) + "[]");
  }
  if (t.map) out.push(printRecord(t.map));
  // An opaque name; a type variable of a printed signature is held as a name
  // that starts with U+FFFF, so it sorts after every opaque name (f01 5.2).
  if (t.opaque) out.push(...[...t.opaque].sort(compareText).map((n) => (n.charCodeAt(0) === 0xffff ? n.slice(1) : n)));
  if (t.null) out.push("null");
  return out;
}

/** Whether the printed text of a type has a ` | ` at the top level: counted, not printed. */
function topLevelUnion(t: Type): boolean {
  if (isAny(t)) return false;
  let n = 0;
  if (t.bool) n++;
  if (t.num) n++;
  if (t.str) n += t.str === "string" ? 1 : t.str.size;
  if (t.list) n++;
  if (t.map) n++;
  if (t.opaque) n += t.opaque.size;
  if (t.null) n++;
  return n > 1;
}

function printRecord(r: Rec): string {
  if (r.fields.size === 0) return isNever(r.rest) ? "{}" : `map<${printType(r.rest)}>`;
  const parts = [...r.fields.keys()].sort(compareText).map((k) => {
    const f = r.fields.get(k)!;
    return `${WORD.test(k) ? k : jsonString(k)}${f.optional ? "?" : ""}: ${printType(f.type)}`;
  });
  if (isAny(r.rest)) parts.push("...");
  else if (!isNever(r.rest)) parts.push(`...${printType(r.rest)}`);
  return `{${parts.join(", ")}}`;
}

export function printType(t: Type): string {
  if (isAny(t)) return "any";
  const parts = altTexts(t);
  return parts.length === 0 ? "never" : parts.join(" | ");
}

// ---- From type text (3.3, 3.4, 3.5, 19.5) -------------------------------------------

export class TypeError extends Error {}

export interface FromContext {
  /** Declared opaque type names. */
  opaque: ReadonlySet<string>;
  /** The solution of a type variable; undefined when there is none (an error). */
  variable?: (name: string) => Type | undefined;
}

/** The type that type text gives. Throws TypeError on a rule beyond the grammar (19.5). */
export function fromNode(n: TypeNode, ctx: FromContext): Type {
  switch (n.t) {
    case "word":
      switch (n.word) {
        case "null": return NULL;
        case "bool": return BOOL;
        case "int": return INT;
        case "float": return FLOAT;
        case "string": return STRING;
        case "any": return ANY;
        case "never": return NEVER;
      }
      throw new TypeError(`unknown word ${n.word}`);
    case "literal":
      return lit(n.value);
    case "map":
      return mapOf(fromNode(n.rest, ctx));
    case "record": {
      const fields: [string, Field][] = [];
      const seen = new Set<string>();
      for (const f of n.fields) {
        if (seen.has(f.key)) throw new TypeError(`the key ${f.key} is written twice in one record`);
        seen.add(f.key);
        fields.push([f.key, { type: fromNode(f.type, ctx), optional: f.optional }]);
      }
      const rest = n.rest === "exact" ? NEVER : n.rest === "any" ? ANY : fromNode(n.rest, ctx);
      return record(fields, rest);
    }
    case "name":
      if (!ctx.opaque.has(n.name)) throw new TypeError(`${n.name} is not a declared opaque type`);
      return opaque(n.name);
    case "var": {
      const v = ctx.variable?.(n.name);
      if (v === undefined) throw new TypeError(`the type variable ${n.name} has no solution`);
      return v;
    }
    case "list":
      return listOf(fromNode(n.elem, ctx));
    case "union": {
      const acc = new TypeAcc();
      let records = 0;
      for (const a of n.alts) {
        const x = fromNode(a, ctx);
        if (a.t !== "var" && !isAny(x) && x.map) records++;
        acc.add(x);
      }
      if (records > 1) throw new TypeError("two record alternatives in one type");
      return acc.type();
    }
  }
}
