// Types (05-types.md sections 2 to 4): sets of alternatives with one slot
// for each kind, records, `any` and `never`, and the operations Join, Fits,
// Merge, Truthy, Falsy, the helpers, printing and reading type text.
//
// Package F17, step E2 (f17 12): a reference to a recursive type is an
// alternative of a type, as an opaque name is. Fits, Join, Equal and disjoint
// follow references by the rules of f17 12.5. Those computations run on an
// explicit stack (`drive`), so a computation that enters 4,096 pairs needs no
// deep native stack; a type with no reference anywhere takes the Core path.

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

/**
 * A reference to a recursive type (f17 12.1). Two references are the same
 * when they are one object: of one declaration, whatever their names.
 */
export interface Ref {
  readonly name: string;
  /** The definition, one level deep (f17 12.2 item 4); undefined while the reference is not complete. */
  def: Type | undefined;
  readonly id: number;
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
  /**
   * The record of the map alternative. With a tagged union (package F5,
   * f05 2.2) it is flat(U), the Join of the variants (f05 2.3), so that every
   * rule that reads "the record of a type" sees flat(U) (f05 4.3).
   */
  readonly map?: Rec;
  /** Package F5: the map alternative is this tagged union, and `map` is its flat. */
  readonly union?: Union;
  /** Opaque type names. */
  readonly opaque?: ReadonlySet<string>;
  /** Package F17: references to recursive types, in code point order of their names; never empty. */
  readonly refs?: readonly Ref[];
}

/** A tagged union (f05 2.2): two or more records with a tag, held as a set. */
export interface Union {
  readonly variants: readonly Rec[];
  /** The least key that is a tag of the variants (f05 2.1). */
  readonly tag: string;
}

export interface AnyType {
  readonly any: true;
}

export type Type = AnyType | Alts;

type Mutable = { -readonly [K in keyof Alts]: Alts[K] };

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
  return !t.null && !t.bool && !t.num && !t.str && !t.list && !t.map && !(t.opaque && t.opaque.size > 0) && t.refs === undefined;
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

/** Whether null is an alternative: written, or of the definition of a reference (f17 12.1). */
export function hasNull(t: Type): boolean {
  if (isAny(t)) return false;
  if (t.null === true) return true;
  if (t.refs === undefined) return false;
  return t.refs.some((r) => r.def !== undefined && !isAny(r.def) && r.def.null === true);
}

/** The alternatives of a type's unfolding, as one-kind types (no `any`). */
export function alternatives(t0: Alts): Alts[] {
  const t = unfold(t0);
  if (isAny(t)) return [];
  const out: Alts[] = [];
  if (t.bool) out.push(BOOL);
  if (t.num) out.push({ num: t.num });
  if (t.str) out.push({ str: t.str });
  if (t.list) out.push({ list: t.list });
  if (t.map) out.push(t.union ? { map: t.map, union: t.union } : { map: t.map });
  if (t.opaque) for (const n of [...t.opaque].sort(compareText)) out.push({ opaque: new Set([n]) });
  if (t.null) out.push(NULL);
  return out;
}

/** The number of kinds of a type: each opaque name is a kind of its own. */
export function kindCount(t: Alts): number {
  return alternatives(t).length;
}

// ---- references (package F17, f17 12.1) -------------------------------------------

let refSerial = 0;

/** A new reference, not complete. */
export function newRef(name: string): Ref {
  return { name, def: undefined, id: ++refSerial };
}

const refTypeMemo = new WeakMap<Ref, Alts>();

/** The type whose one alternative is the reference. */
export function refType(r: Ref): Alts {
  let t = refTypeMemo.get(r);
  if (t === undefined) refTypeMemo.set(r, (t = { refs: [r] }));
  return t;
}

/** The order of references among alternatives (f17 12.6 item 3): their names, in code point order. */
export function compareRefs(a: Ref, b: Ref): number {
  return compareText(a.name, b.name) || a.id - b.id;
}

function refsUnion(a: readonly Ref[] | undefined, b: readonly Ref[] | undefined): readonly Ref[] | undefined {
  if (a === undefined || a.length === 0) return b !== undefined && b.length > 0 ? b : undefined;
  if (b === undefined || b.length === 0) return a;
  const out = [...a];
  for (const r of b) if (!out.includes(r)) out.push(r);
  return out.sort(compareRefs);
}

/** A type less its references. */
export function withoutRefs(t: Alts): Alts {
  if (t.refs === undefined) return t;
  const { refs: _, ...rest } = t;
  return rest;
}

function withRefs(t: Alts, refs: readonly Ref[] | undefined): Alts {
  const base = withoutRefs(t);
  return refs !== undefined && refs.length > 0 ? { ...base, refs } : base;
}

/** Whether the type has a reference that is not complete among its alternatives. */
export function hasIncomplete(t: Type): boolean {
  return !isAny(t) && t.refs !== undefined && t.refs.some((r) => r.def === undefined);
}

/** A rule looked into a reference whose definition is not known yet (f17 12.4 item 4). */
export class IncompleteRef extends Error {}

/** Merge the alternatives of b into a, where they share no kind but null (f17 12.2 item 5). */
export function mergeKinds(a: Alts, b: Type): Type {
  if (isAny(b)) return ANY;
  if ((a.list && b.list) || (a.map && b.map)) return join(a, b);
  const out: Mutable = { ...a };
  if (b.null) out.null = true;
  if (b.bool) out.bool = true;
  if (b.num) out.num = a.num === "float" || b.num === "float" ? "float" : "int";
  const s = joinStr(a.str, b.str);
  if (s !== undefined) out.str = s;
  if (b.list) out.list = b.list;
  if (b.map) {
    out.map = b.map;
    if (b.union) out.union = b.union;
  }
  if (b.opaque && b.opaque.size > 0) out.opaque = new Set([...(a.opaque ?? []), ...b.opaque]);
  const rs = refsUnion(a.refs, b.refs);
  if (rs !== undefined) out.refs = rs;
  return out;
}

const unfoldMemo = new WeakMap<object, Type>();

/**
 * The unfolding (f17 12.1): each reference among the alternatives replaced by
 * the alternatives of its definition. A definition holds no reference at its
 * top, so one level is enough. Throws IncompleteRef for a reference whose
 * definition is not known.
 */
export function unfold(t: Type): Type {
  if (isAny(t) || t.refs === undefined) return t;
  const m = unfoldMemo.get(t);
  if (m !== undefined) return m;
  let out: Type = withoutRefs(t);
  for (const r of t.refs) {
    if (r.def === undefined) throw new IncompleteRef(`${r.name} is not complete`);
    if (!isAny(out)) out = mergeKinds(out, r.def);
  }
  unfoldMemo.set(t, out);
  return out;
}

/** Replace the given references of t by their definitions. */
function unfoldSome(t: Alts, which: (r: Ref) => boolean): Type {
  if (t.refs === undefined) return t;
  const keep = t.refs.filter((r) => !which(r));
  if (keep.length === t.refs.length) return t;
  let out: Type = withRefs(t, keep);
  for (const r of t.refs) {
    if (!which(r)) continue;
    if (r.def === undefined) throw new IncompleteRef(`${r.name} is not complete`);
    if (!isAny(out)) out = mergeKinds(out as Alts, r.def);
  }
  return out;
}

/** The kinds of a type other than null: `bool`, `num`, `str`, `list`, `map`, and each opaque name. With references, their definitions' kinds. */
export function kindsOf(t: Type): Set<string> {
  const out = new Set<string>();
  if (isAny(t)) return out;
  if (t.bool) out.add("bool");
  if (t.num) out.add("num");
  if (t.str) out.add("str");
  if (t.list) out.add("list");
  if (t.map) out.add("map");
  if (t.opaque) for (const n of t.opaque) out.add("o:" + n);
  for (const r of t.refs ?? []) if (r.def !== undefined) for (const k of kindsOf(r.def)) out.add(k);
  return out;
}

function shares(a: Set<string>, b: Set<string>): boolean {
  for (const k of a) if (b.has(k)) return true;
  return false;
}

const refDeepMemo = new WeakMap<object, boolean>();

/** Whether a reference stands anywhere in the type, without following one. */
export function hasRefDeep(t: Type): boolean {
  if (isAny(t)) return false;
  const m = refDeepMemo.get(t);
  if (m !== undefined) return m;
  let out = t.refs !== undefined;
  if (!out && t.list) out = hasRefDeep(t.list);
  if (!out && t.map) out = recHasRef(t.map) || (t.union !== undefined && t.union.variants.some(recHasRef));
  refDeepMemo.set(t, out);
  return out;
}

function recHasRef(r: Rec): boolean {
  const m = refDeepMemo.get(r);
  if (m !== undefined) return m;
  let out = hasRefDeep(r.rest);
  if (!out) for (const f of r.fields.values()) if (hasRefDeep(f.type)) { out = true; break; }
  refDeepMemo.set(r, out);
  return out;
}

/**
 * Each type reachable from t: its parts, at any depth, and the definitions of
 * the references it reaches, each once. An explicit stack: a chain of many
 * references costs no native stack. Stops when `visit` gives true.
 */
export function someReachable(t: Type, visit: (x: Type) => boolean): boolean {
  const seen = new Set<object>();
  const stack: Type[] = [t];
  const push = (x: Type) => {
    if (!seen.has(x)) {
      seen.add(x);
      stack.push(x);
    }
  };
  seen.add(t);
  while (stack.length > 0) {
    const x = stack.pop()!;
    if (visit(x)) return true;
    if (isAny(x)) continue;
    if (x.list) push(x.list);
    const recs = x.union ? x.union.variants : x.map ? [x.map] : [];
    for (const r of recs) {
      if (seen.has(r)) continue;
      seen.add(r);
      for (const f of r.fields.values()) push(f.type);
      push(r.rest);
    }
    for (const r of x.refs ?? []) {
      if (seen.has(r) || r.def === undefined) continue;
      seen.add(r);
      push(r.def);
    }
  }
  return false;
}

/** The references a type reaches, at any depth, through definitions too. */
export function refsReached(t: Type): Ref[] {
  const out: Ref[] = [];
  const seen = new Set<Ref>();
  someReachable(t, (x) => {
    if (!isAny(x)) for (const r of x.refs ?? []) if (!seen.has(r)) {
      seen.add(r);
      out.push(r);
    }
    return false;
  });
  return out;
}

// ---- computations over references (f17 12.5) ------------------------------------------

/** The most pairs one computation enters (f17 12.5 item 7). */
export const PAIR_BOUND = 4096;

/** A computation passed the bound of 4,096 pairs (f17 12.5 item 7). `span`: where a checker reports it. */
export class PairLimit extends Error {
  span?: { start: number; end: number };
  constructor() {
    super("comparing the types takes more than 4096 pairs of types");
  }
}

/** A generator that yields the generator of a computation it needs, and is sent its result. */
type G<T> = Generator<G<unknown>, T, unknown>;

/** Run a generator and those it yields on an explicit stack. */
function drive<T>(g: G<T>): T {
  const stack: G<unknown>[] = [g];
  let input: unknown = undefined;
  for (;;) {
    const top = stack[stack.length - 1];
    const r = top.next(input);
    if (r.done === true) {
      stack.pop();
      if (stack.length === 0) return r.value as T;
      input = r.value;
    } else {
      stack.push(r.value);
      input = undefined;
    }
  }
}

/** One computation (f17 12.5 item 1): the pairs it entered, those in progress and those that failed. */
class Comp {
  entered = 0;
  private readonly ids = new Map<object, number>();
  private readonly keys = new Map<string, number>();
  readonly fitsOn = new Set<string>();
  readonly fitsFailed = new Set<string>();
  readonly joinOn = new Set<string>();
  readonly eqOn = new Set<string>();
  readonly eqFailed = new Set<string>();
  readonly disOn = new Set<string>();
  readonly disHolds = new Set<string>();

  /** f17 12.5 item 3 step 5: a pair is entered; the 4,097th ends the computation. */
  enter(): void {
    if (++this.entered > PAIR_BOUND) throw new PairLimit();
  }

  private intern(k: string): number {
    let n = this.keys.get(k);
    if (n === undefined) this.keys.set(k, (n = this.keys.size + 1));
    return n;
  }

  /** A number for a type: two types have one number when they are identical as written (f17 12.5 item 2). */
  tid(t: Type): number {
    if (isAny(t)) return 0;
    const m = this.ids.get(t);
    if (m !== undefined) return m;
    let k = "";
    if (t.null) k += "n";
    if (t.bool) k += "b";
    if (t.num) k += t.num === "int" ? "i" : "f";
    if (t.str) k += t.str === "string" ? "s" : "l" + JSON.stringify([...t.str].sort(compareText));
    if (t.list) k += "L" + this.tid(t.list);
    if (t.union) k += "U" + t.union.variants.map((v) => this.rid(v)).sort((a, b) => a - b).join(",");
    else if (t.map) k += "M" + this.rid(t.map);
    if (t.opaque && t.opaque.size > 0) k += "O" + JSON.stringify([...t.opaque].sort(compareText));
    if (t.refs) k += "R" + t.refs.map((r) => r.id).join(",");
    const n = this.intern(k);
    this.ids.set(t, n);
    return n;
  }

  private rid(r: Rec): number {
    const m = this.ids.get(r);
    if (m !== undefined) return m;
    let k = "{";
    for (const key of sortedKeys(r)) {
      const f = r.fields.get(key)!;
      k += JSON.stringify(key) + (f.optional ? "?" : "") + ":" + this.tid(f.type) + ",";
    }
    k += "..." + this.tid(r.rest) + (r.closable ? "c" : "") + "}";
    const n = this.intern(k);
    this.ids.set(r, n);
    return n;
  }

  pair(a: Type, b: Type): string {
    return `${this.tid(a)}:${this.tid(b)}`;
  }
}

let current: Comp | null = null;

/** Run one computation (f17 12.5 item 1), or a part of the one in progress. */
function compute<T>(f: (c: Comp) => G<T>): T {
  if (current !== null) return drive(f(current));
  const c = new Comp();
  current = c;
  try {
    return drive(f(c));
  } finally {
    current = null;
  }
}

const keysMemo = new WeakMap<Rec, string[]>();

/** A record's keys in code point order. */
export function sortedKeys(r: Rec): string[] {
  let ks = keysMemo.get(r);
  if (ks === undefined) {
    ks = [...r.fields.keys()].sort(compareText);
    keysMemo.set(r, ks);
  }
  return ks;
}

const printedVariants = new WeakMap<Union, readonly Rec[]>();

/** The variants of a map alternative in the order of printing (f05 2.4). */
export function variantsPrinted(t: Alts): readonly Rec[] {
  if (t.union === undefined) return t.map ? [t.map] : [];
  let vs = printedVariants.get(t.union);
  if (vs === undefined) {
    const tag = t.union.tag;
    const least = (v: Rec) => [...tagSetOf(v, tag)].sort(compareText)[0];
    vs = [...t.union.variants].sort((x, y) => compareText(least(x), least(y)));
    printedVariants.set(t.union, vs);
  }
  return vs;
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

function closableJoin(a: Rec, b: Rec, out: Rec): void {
  // f01 3.6 item 8: a Join of two records is closable when one is closable and the other closable or exact.
  if ((a.closable && (b.closable || isNever(b.rest))) || (b.closable && isNever(a.rest))) out.closable = true;
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
  closableJoin(a, b, out);
  return out;
}

const joinMemo = new WeakMap<object, WeakMap<object, Type>>();

/**
 * Join(A, B) (4.1, f05 4.1). With a reference anywhere in either, the rule of
 * f17 12.5 item 4, as one computation, or as a part of the one in progress.
 */
export function join(a: Type, b: Type): Type {
  if (isAny(a) || isAny(b)) return ANY;
  if (a === b) return a;
  if (hasRefDeep(a) || hasRefDeep(b)) return compute((c) => joinG(c, a, b));
  // A type is held with shared parts: the Join of two is kept, so joining
  // two records that share parts does no work twice (05 20.7: no work grows with the expansion of a type).
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
  const out: Mutable = {};
  if (a.null || b.null) out.null = true;
  if (a.bool || b.bool) out.bool = true;
  if (a.num || b.num) out.num = a.num === "float" || b.num === "float" ? "float" : "int";
  const s = joinStr(a.str, b.str);
  if (s !== undefined) out.str = s;
  if (a.list && b.list) out.list = join(a.list, b.list);
  else if (a.list || b.list) out.list = a.list ?? b.list;
  if (a.map && b.map) {
    // f05 4.1: a union survives only beside one the same by structure; else the record Join of the flats.
    if ((a.union || b.union) && sameMapAlt(a, b)) {
      out.map = a.map;
      out.union = a.union;
    } else out.map = joinRec(a.map, b.map);
  } else if (a.map) {
    out.map = a.map;
    if (a.union) out.union = a.union;
  } else if (b.map) {
    out.map = b.map;
    if (b.union) out.union = b.union;
  }
  if (a.opaque || b.opaque) out.opaque = new Set([...(a.opaque ?? []), ...(b.opaque ?? [])]);
  return out;
}

/** null and no other alternative (f17 12.5 item 4 step 2). */
function nullAlone(t: Alts): boolean {
  return t.null === true && t.refs === undefined && kindsOf(t).size === 0;
}

/** Join with references (f17 12.5 item 4), in its steps. */
function* joinG(c: Comp, a: Type, b: Type): G<Type> {
  // 1.
  if (isAny(a) || isAny(b)) return ANY;
  if (a === b) return a;
  if (isNever(a)) return b;
  if (isNever(b)) return a;
  if (!hasRefDeep(a) && !hasRefDeep(b)) return join(a, b);
  if (a.refs === undefined && b.refs === undefined) return (yield join0G(c, a, b, undefined)) as Type;
  // 2.
  if (nullAlone(a)) return withNull(b);
  if (nullAlone(b)) return withNull(a);
  // 3.
  if (hasIncomplete(a) || hasIncomplete(b)) return (yield join0G(c, withoutRefs(a), withoutRefs(b), refsUnion(a.refs, b.refs))) as Type;
  // 4.
  if (c.tid(a) === c.tid(b)) return a;
  // 5.
  const ua = unfold(a);
  const ub = unfold(b);
  if (isAny(ua) || isAny(ub)) return ANY;
  // 6.
  if ((yield fitsG(c, a, b)) === 1) return b;
  if ((yield fitsG(c, b, a)) === 1) return a;
  // 7.
  const key = c.pair(a, b);
  if (c.joinOn.has(key)) return ANY;
  // 8.
  c.enter();
  c.joinOn.add(key);
  const ka = kindsOf(ua);
  const kb = kindsOf(ub);
  const a2 = unfoldSome(a, (r) => !(b.refs ?? []).includes(r) && r.def !== undefined && shares(kindsOf(r.def), kb));
  const b2 = unfoldSome(b, (r) => !(a.refs ?? []).includes(r) && r.def !== undefined && shares(kindsOf(r.def), ka));
  let out: Type;
  if (isAny(a2) || isAny(b2)) out = ANY;
  else out = (yield join0G(c, withoutRefs(a2), withoutRefs(b2), refsUnion(a2.refs, b2.refs))) as Type;
  c.joinOn.delete(key);
  return out;
}

/** Core's Join of two types with no reference among their alternatives, parts by joinG; `refs` beside the result. */
function* join0G(c: Comp, a: Alts, b: Alts, refs: readonly Ref[] | undefined): G<Type> {
  const out: Mutable = {};
  if (a.null || b.null) out.null = true;
  if (a.bool || b.bool) out.bool = true;
  if (a.num || b.num) out.num = a.num === "float" || b.num === "float" ? "float" : "int";
  const s = joinStr(a.str, b.str);
  if (s !== undefined) out.str = s;
  // f17 12.5 item 6: the list's elements, then the map alternatives.
  if (a.list && b.list) out.list = (yield joinG(c, a.list, b.list)) as Type;
  else if (a.list || b.list) out.list = a.list ?? b.list;
  if (a.map && b.map) {
    if ((a.union || b.union) && (yield sameMapAltG(c, a, b))) {
      out.map = a.map;
      out.union = a.union;
    } else out.map = (yield joinRecG(c, a.map, b.map)) as Rec;
  } else if (a.map) {
    out.map = a.map;
    if (a.union) out.union = a.union;
  } else if (b.map) {
    out.map = b.map;
    if (b.union) out.union = b.union;
  }
  if (a.opaque || b.opaque) out.opaque = new Set([...(a.opaque ?? []), ...(b.opaque ?? [])]);
  if (refs !== undefined && refs.length > 0) out.refs = refs;
  return out;
}

/** The Join of two records: the keys of both in code point order, then the rests (f17 12.5 item 6). */
function* joinRecG(c: Comp, a: Rec, b: Rec): G<Rec> {
  const fields = new Map<string, Field>();
  const keys = [...new Set([...sortedKeys(a), ...sortedKeys(b)])].sort(compareText);
  for (const k of keys) {
    const fa = a.fields.get(k);
    const fb = b.fields.get(k);
    if (fa !== undefined && fb !== undefined) {
      const type = (yield joinG(c, fa.type, fb.type)) as Type;
      if (fa.fill === undefined && fb.fill === undefined) fields.set(k, { type, optional: fa.optional || fb.optional });
      else {
        const fill = fa.fill === undefined ? fb.fill! : fb.fill === undefined ? fa.fill : ((yield joinG(c, fa.fill, fb.fill)) as Type);
        const base = (yield joinG(c, fa.base ?? fa.type, fb.base ?? fb.type)) as Type;
        fields.set(k, { type, optional: false, base, fill });
      }
    } else {
      const f = (fa ?? fb)!;
      const rest = fa !== undefined ? b.rest : a.rest;
      const type = isNever(rest) ? f.type : ((yield joinG(c, f.type, rest)) as Type);
      if (f.fill === undefined) fields.set(k, { type, optional: true });
      else fields.set(k, { type, optional: false, base: isNever(rest) ? f.base! : ((yield joinG(c, f.base!, rest)) as Type), fill: f.fill });
    }
  }
  const out: Rec = { fields, rest: (yield joinG(c, a.rest, b.rest)) as Type };
  closableJoin(a, b, out);
  return out;
}

/** The Join of many types; literal sets are merged in one pass (05 20.7: building a type is at most n log n). */
export function joinAll(ts: Type[]): Type {
  const acc = new TypeAcc();
  for (const x of ts) acc.add(x);
  return acc.type();
}

/**
 * An accumulating Join (05 20.7, n log n in the values joined): the literal sets go into one mutable
 * set, so joining n types is at most n log n, and the size of the Join so far
 * is known at any time without building it. A type with a reference among its
 * alternatives is joined as it comes, each Join one computation (f17 12.5 item 4).
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
    if (t.refs !== undefined || this.other.refs !== undefined) {
      this.flush();
      const j = join(this.other, t);
      if (isAny(j)) this.anyType = true;
      else this.other = j;
      return;
    }
    if (t.str !== undefined) {
      if (t.str === "string") this.str = true;
      else if (!this.str) for (const x of t.str) this.lits.add(x);
    }
    if (t.null || t.bool || t.num || t.list || t.map || (t.opaque && t.opaque.size > 0)) {
      const { str: _, ...rest } = t;
      const j = join(this.other, rest);
      if (isAny(j)) this.anyType = true;
      else this.other = j;
    }
  }

  /** Join the strings gathered so far into the rest. */
  private flush(): void {
    if (!this.str && this.lits.size === 0) return;
    const s: Alts = { str: this.str ? "string" : new Set(this.lits) };
    this.str = false;
    this.lits.clear();
    const j = join(this.other, s);
    if (isAny(j)) this.anyType = true;
    else this.other = j;
  }

  size(): number {
    if (this.anyType) return 1;
    const sum = altSum(this.other) + (this.str ? 1 : this.lits.size);
    return sum === 0 ? 1 : Math.min(sum, SIZE_LIMIT + 1);
  }

  type(): Type {
    if (this.anyType) return ANY;
    if (!this.str && this.lits.size === 0) return this.other;
    if (this.other.refs !== undefined) {
      this.flush();
      return this.anyType ? ANY : this.other;
    }
    return { ...this.other, str: this.str ? "string" : new Set(this.lits) };
  }
}

// ---- Size (05 20.7) ----------------------------------------------------------------

/** The limit on the size of a type (05 20.7). */
export const SIZE_LIMIT = 100_000;
const sizeMemo = new WeakMap<object, number>();

function sat(n: number): number {
  return n > SIZE_LIMIT ? SIZE_LIMIT + 1 : n;
}

/** The sum of the sizes of a type's alternatives. A reference has size 1 (f17 12.6 item 2). */
function altSum(t: Alts): number {
  const m = sizeMemo.get(t);
  if (m !== undefined) return m;
  let n = 0;
  if (t.null) n++;
  if (t.bool) n++;
  if (t.num) n++;
  if (t.str) n += t.str === "string" ? 1 : t.str.size;
  if (t.opaque) n += t.opaque.size;
  if (t.refs) n += t.refs.length;
  if (t.list) n = sat(n + 1 + typeSize(t.list));
  if (t.union) for (const v of t.union.variants) n = sat(n + recSize(v));
  else if (t.map) n = sat(n + recSize(t.map));
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

/**
 * Fits(S, T) (4.2, f05 4.2). With a reference anywhere in either, the rule of
 * f17 12.5 item 3, as one computation, or as a part of the one in progress.
 */
export function fits(s: Type, t: Type): Fit {
  if (isAny(t)) return 1;
  if (isAny(s)) return 2;
  if (hasRefDeep(s) || hasRefDeep(t)) return compute((c) => fitsG(c, s, t));
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
    // f05 4.2: each variant of S fits some variant of T; it uses any only when it fits none without.
    const tv = variantsOf(t);
    for (const sv of variantsOf(s)) {
      let best: Fit = 0;
      for (const v of tv) {
        const f = fitsRec(sv, v);
        if (f === 1) {
          best = 1;
          break;
        }
        if (f === 2) best = 2;
      }
      if (best === 0) return 0;
      r = both(r, best);
    }
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

/** Fits with references (f17 12.5 item 3), in its steps. */
function* fitsG(c: Comp, s: Type, t: Type): G<Fit> {
  // 1.
  if (isAny(t)) return 1;
  if (isAny(s)) return 2;
  if (isNever(s)) return 1;
  if (!hasRefDeep(s) && !hasRefDeep(t)) return fits(s, t);
  // 2.
  let ss: Alts = s;
  if (s.refs !== undefined && t.refs !== undefined) {
    const tr = t.refs;
    const keep = s.refs.filter((r) => !tr.includes(r));
    if (keep.length !== s.refs.length) {
      ss = withRefs(s, keep);
      if (isNever(ss)) return 1;
    }
  }
  if (ss.refs === undefined && t.refs === undefined) return (yield fitsAltsG(c, ss, t)) as Fit;
  // 3. and 4.
  const key = c.pair(ss, t);
  if (c.fitsOn.has(key)) return 1;
  if (c.fitsFailed.has(key)) return 0;
  // 5.
  c.enter();
  c.fitsOn.add(key);
  // 6.
  const us = unfold(ss);
  const ut = unfold(t);
  let r: Fit;
  if (isAny(ut)) r = 1;
  else if (isAny(us)) r = 2;
  else r = (yield fitsAltsG(c, us, ut)) as Fit;
  c.fitsOn.delete(key);
  if (r === 0) c.fitsFailed.add(key);
  return r;
}

/**
 * Core's rule (4.2, f05 4.2) for two types with no reference among their
 * alternatives, parts by fitsG, in the order of f17 12.5 item 6: bool,
 * number, string, list, map, the opaque names, null; it stops at the first
 * alternative that does not fit.
 */
function* fitsAltsG(c: Comp, s: Alts, t: Alts): G<Fit> {
  if (isNever(s)) return 1;
  let r: Fit = 1;
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
    r = both(r, (yield fitsG(c, s.list, t.list)) as Fit);
    if (r === 0) return 0;
  }
  if (s.map) {
    if (!t.map) return 0;
    const tv = variantsPrinted(t);
    for (const sv of variantsPrinted(s)) {
      let best: Fit = 0;
      for (const v of tv) {
        const f = (yield fitsRecG(c, sv, v)) as Fit;
        if (f === 1) {
          best = 1;
          break;
        }
        if (f === 2) best = 2;
      }
      if (best === 0) return 0;
      r = both(r, best);
    }
  }
  if (s.opaque) {
    for (const n of [...s.opaque].sort(compareText)) if (!t.opaque || !t.opaque.has(n)) return 0;
  }
  if (s.null && !t.null) return 0;
  return r;
}

/** Two records (f17 12.5 item 6): T's fields in key order, the mark before the type; S's other fields against T's rest; the rests. */
function* fitsRecG(c: Comp, s: Rec, t: Rec): G<Fit> {
  let r: Fit = 1;
  for (const k of sortedKeys(t)) {
    const tf = t.fields.get(k)!;
    const sf = s.fields.get(k);
    if (sf !== undefined) {
      if (sf.optional && !tf.optional) return 0;
      r = both(r, (yield fitsG(c, sf.type, tf.type)) as Fit);
    } else {
      if (!tf.optional) return 0;
      r = both(r, (yield fitsG(c, s.rest, tf.type)) as Fit);
    }
    if (r === 0) return 0;
  }
  for (const k of sortedKeys(s)) {
    if (t.fields.has(k)) continue;
    r = both(r, (yield fitsG(c, s.fields.get(k)!.type, t.rest)) as Fit);
    if (r === 0) return 0;
  }
  return both(r, (yield fitsG(c, s.rest, t.rest)) as Fit);
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

/** f17 12.5 item 8: a definition that Truthy leaves as it is: no null, no "", no never[], no record that can be empty. */
function truthyKeeps(d: Type): boolean {
  if (isAny(d)) return false;
  if (d.null) return false;
  if (d.str !== undefined && d.str !== "string" && d.str.has("")) return false;
  if (d.list !== undefined && isNever(d.list)) return false;
  if (d.map !== undefined && variantsOf(d).some((v) => [...v.fields.values()].every((f) => f.optional))) return false;
  return true;
}

export function truthyPart(t0: Type): Type {
  if (isAny(t0)) return ANY;
  let keep: readonly Ref[] | undefined;
  let t: Type = t0;
  if (t0.refs !== undefined) {
    // f17 12.5 item 8: a reference stays when its definition cannot be falsy; else the rule reads its unfolding.
    t = unfoldSome(t0, (r) => r.def === undefined || !truthyKeeps(r.def));
    if (isAny(t)) return ANY;
    keep = t.refs;
    t = withoutRefs(t);
  }
  const out: Mutable = {};
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
  if (t.map && !(t.map.fields.size === 0 && isNever(t.map.rest))) {
    out.map = t.map;
    if (t.union) out.union = t.union;
  }
  if (t.opaque && t.opaque.size > 0) out.opaque = t.opaque;
  if (keep !== undefined) out.refs = keep;
  return out;
}

export function falsyPart(t0: Type): Type {
  // f17 12.5 item 8: Falsy reads the unfolding.
  const t = unfold(t0);
  if (isAny(t)) return ANY;
  const out: Mutable = {};
  if (t.null) out.null = true;
  if (t.bool) out.bool = true;
  if (t.num) out.num = t.num;
  if (t.str && (t.str === "string" || t.str.has(""))) out.str = new Set([""]);
  if (t.list) out.list = NEVER;
  if (t.map && ![...t.map.fields.values()].some((f) => !f.optional)) out.map = { fields: new Map(), rest: NEVER };
  return out;
}

// ---- Helpers (4.5) ----------------------------------------------------------------

/** The type without null. f17 12.5 item 8: a reference whose definition has null is unfolded. */
export function nonnull(t0: Type): Type {
  if (isAny(t0)) return t0;
  let t: Type = t0;
  if (t0.refs !== undefined && t0.refs.some((r) => r.def !== undefined && !isAny(r.def) && r.def.null === true)) {
    t = unfoldSome(t0, (r) => r.def !== undefined && !isAny(r.def) && r.def.null === true);
    if (isAny(t)) return t;
  }
  if (!t.null) return t;
  const { null: _, ...rest } = t;
  return rest;
}

/** The type with null added, as written (f17 12.2 item 5). */
export function withNull(t: Type): Type {
  if (isAny(t)) return t;
  return { ...t, null: true };
}

export function overlap(a0: Type, b0: Type): boolean {
  const a = unfold(a0);
  const b = unfold(b0);
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
  let out: boolean;
  // f17: a type that reaches a reference is walked through the definitions, each once.
  if (hasRefDeep(t)) out = someReachable(t, (x) => !isAny(x) && x.opaque !== undefined && x.opaque.size > 0);
  else out = hasOpaque0(t, depth);
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

/** has-any(T) (05 4.5): T is `any`, or an element, field or rest type of T has `any`. */
export function hasAny(t: Type, depth = 0): boolean {
  if (isAny(t)) return true;
  if (depth > 200) return false;
  const m = anyMemo.get(t);
  if (m !== undefined) return m;
  let out: boolean;
  if (hasRefDeep(t)) out = someReachable(t, (x) => isAny(x));
  else out = hasAny0(t, depth);
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
export function finiteValues(t0: Type): (string | boolean | null)[] | null {
  const t = unfold(t0);
  if (isAny(t)) return null;
  if (t.num || t.list || t.map || (t.opaque && t.opaque.size > 0) || t.str === "string") return null;
  const out: (string | boolean | null)[] = [];
  if (t.null) out.push(null);
  if (t.bool) out.push(true, false);
  if (t.str) for (const s of t.str) out.push(s);
  return out;
}

/** The depth of a type (2.5). A reference has depth 0 (f17 12.6 item 1). */
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

// ---- Equality ------------------------------------------------------------------------

/**
 * Two types identical by structure, with references compared as atoms (one
 * object), nothing unfolded: for comparing a case's `type` (03 3.10.6, f17
 * 12.6 item 5).
 */
export function typeEqual(a: Type, b: Type): boolean {
  if (a === b) return true;
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
  const ar = a.refs ?? [];
  const br = b.refs ?? [];
  if (ar.length !== br.length || ar.some((r) => !br.includes(r))) return false;
  if ((a.map === undefined) !== (b.map === undefined)) return false;
  if (a.map) return sameMapAltBy(a, b, typeEqual);
  return true;
}

function recEqualBy(x: Rec, y: Rec, eq: (a: Type, b: Type) => boolean): boolean {
  if (x === y) return true;
  if (x.fields.size !== y.fields.size || !eq(x.rest, y.rest)) return false;
  for (const [k, f] of x.fields) {
    const g = y.fields.get(k);
    if (g === undefined || g.optional !== f.optional || !eq(f.type, g.type)) return false;
  }
  return true;
}

function sameMapAltBy(a: Alts, b: Alts, eq: (a: Type, b: Type) => boolean): boolean {
  const x = variantsOf(a);
  const y = variantsOf(b);
  if (x.length !== y.length) return false;
  if (x.length === 1) return recEqualBy(x[0], y[0], eq);
  return x.every((v) => y.some((w) => recEqualBy(v, w, eq)));
}

/** Two map alternatives the same by structure (f05 2.5): the same variants, in any order. */
function sameMapAlt(a: Alts, b: Alts): boolean {
  return sameMapAltBy(a, b, sameType);
}

/**
 * Equal(A, B) (f05 2.5, f04 6.2): the same by structure. With a reference
 * anywhere in either, the rule of f17 12.5 item 5, as one computation.
 */
export function sameType(a: Type, b: Type): boolean {
  if (a === b) return true;
  if (isAny(a) || isAny(b)) return isAny(a) && isAny(b);
  if (hasRefDeep(a) || hasRefDeep(b)) return compute((c) => equalG(c, a, b));
  return typeEqual(a, b);
}

/** Two records the same by structure (f05 2.5). */
export function sameRecord(x: Rec, y: Rec): boolean {
  if (x === y) return true;
  if (!recHasRef(x) && !recHasRef(y)) return recEqualBy(x, y, typeEqual);
  return compute((c) => recEqualG(c, x, y));
}

function* equalG(c: Comp, a: Type, b: Type): G<boolean> {
  if (a === b) return true;
  if (isAny(a) || isAny(b)) return isAny(a) && isAny(b);
  if (!hasRefDeep(a) && !hasRefDeep(b)) return typeEqual(a, b);
  if (a.refs === undefined && b.refs === undefined) return (yield altsEqualG(c, a, b)) as boolean;
  if (c.tid(a) === c.tid(b)) return true;
  const key = c.pair(a, b);
  if (c.eqOn.has(key)) return true;
  if (c.eqFailed.has(key)) return false;
  c.enter();
  c.eqOn.add(key);
  const ua = unfold(a);
  const ub = unfold(b);
  let r: boolean;
  if (isAny(ua) || isAny(ub)) r = isAny(ua) && isAny(ub);
  else r = (yield altsEqualG(c, ua, ub)) as boolean;
  c.eqOn.delete(key);
  if (!r) c.eqFailed.add(key);
  return r;
}

/** Two types with no reference among their alternatives: the alternatives in the order of printing (f17 12.5 item 6). */
function* altsEqualG(c: Comp, a: Alts, b: Alts): G<boolean> {
  if (!!a.bool !== !!b.bool || a.num !== b.num) return false;
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
  if (a.list && !((yield equalG(c, a.list, b.list!)) as boolean)) return false;
  if ((a.map === undefined) !== (b.map === undefined)) return false;
  if (a.map && !((yield sameMapAltG(c, a, b)) as boolean)) return false;
  const ao = a.opaque ?? new Set<string>();
  const bo = b.opaque ?? new Set<string>();
  if (ao.size !== bo.size || [...ao].some((n) => !bo.has(n))) return false;
  if (!!a.null !== !!b.null) return false;
  return true;
}

function* sameMapAltG(c: Comp, a: Alts, b: Alts): G<boolean> {
  const x = variantsPrinted(a);
  const y = variantsPrinted(b);
  if (x.length !== y.length) return false;
  for (const v of x) {
    let found = false;
    for (const w of y) {
      if ((yield recEqualG(c, v, w)) as boolean) {
        found = true;
        break;
      }
    }
    if (!found) return false;
  }
  return true;
}

/** Two records: the keys in code point order, each field's mark before its type, then the rests (f17 12.5 item 6). */
function* recEqualG(c: Comp, x: Rec, y: Rec): G<boolean> {
  if (x === y) return true;
  if (x.fields.size !== y.fields.size) return false;
  for (const k of sortedKeys(x)) {
    const f = x.fields.get(k)!;
    const g = y.fields.get(k);
    if (g === undefined || g.optional !== f.optional) return false;
    if (!((yield equalG(c, f.type, g.type)) as boolean)) return false;
  }
  return (yield equalG(c, x.rest, y.rest)) as boolean;
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
  if (t.union) {
    // f05 2.4: the variants, in code point order of the least string of each tag set.
    out.push(...variantsPrinted(t).map(printRecord));
  } else if (t.map) out.push(printRecord(t.map));
  // An opaque name; a type variable of a printed signature is held as a name
  // that starts with U+FFFF, so it sorts after every opaque name (f01 5.2).
  if (t.opaque) out.push(...[...t.opaque].sort(compareText).map((n) => (n.charCodeAt(0) === 0xffff ? n.slice(1) : n)));
  // f17 12.6 item 3: a reference by its name, after the opaque names and before null.
  if (t.refs) for (const r of t.refs) out.push(r.name);
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
  if (t.map) n += t.union ? t.union.variants.length : 1;
  if (t.opaque) n += t.opaque.size;
  if (t.refs) n += t.refs.length;
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

/** A recursive type printed by its unfolding, in which its own name stands for itself (f17 12.4, 12.8). */
export function printUnfolded(t: Type): string {
  const r = soleRef(t);
  return r !== null && r.def !== undefined ? printType(r.def) : printType(t);
}

/** The reference that is a type's one alternative, or null. */
export function soleRef(t: Type): Ref | null {
  if (isAny(t) || t.refs === undefined || t.refs.length !== 1 || !isNever(withoutRefs(t))) return null;
  return t.refs[0];
}

// ---- From type text (3.3, 3.4, 3.5, 19.5) -------------------------------------------

export class TypeError extends Error {}

export interface FromContext {
  /** Declared opaque type names. */
  opaque: ReadonlySet<string>;
  /** Package F4: the named types, each resolved (f04 3.5 step 2). */
  named?: ReadonlyMap<string, Type>;
  /** The solution of a type variable; undefined when there is none (an error). */
  variable?: (name: string) => Type | undefined;
  /**
   * Package F17: a name of the list being resolved, by where it stands: at a
   * guarded place or not (f17 12.2 item 4). Undefined for a name outside the list.
   */
  name?: (name: string, guarded: boolean) => Type | undefined;
  /** A case's `type`: a name that is no opaque type is an atom, compared by its spelling (f17 12.6 item 5). */
  atoms?: Map<string, Ref>;
}

/**
 * The alternatives of a union of resolved types, as f17 12.2 item 5 reads
 * them: a complete reference that shares a kind other than null with another
 * alternative counts as its definition.
 */
export function unionParts(xs: Type[]): Type[] {
  if (!xs.some((x) => !isAny(x) && x.refs !== undefined)) return xs;
  // The kinds of each alternative: a reference's are its definition's; one not complete has none known.
  const count = new Map<string, number>();
  const add = (ks: Set<string>) => {
    for (const k of ks) count.set(k, (count.get(k) ?? 0) + 1);
  };
  for (const x of xs) {
    if (isAny(x)) continue;
    add(kindsOf(withoutRefs(x)));
    for (const r of x.refs ?? []) if (r.def !== undefined) add(kindsOf(r.def));
  }
  return xs.map((x) => {
    if (isAny(x) || x.refs === undefined) return x;
    return unfoldSome(x, (r) => r.def !== undefined && [...kindsOf(r.def)].some((k) => (count.get(k) ?? 0) > 1));
  });
}

/** The type that type text gives. Throws TypeError on a rule beyond the grammar (19.5). `guarded`: inside a list's element, a field, a rest or a map's value. */
export function fromNode(n: TypeNode, ctx: FromContext, guarded = false): Type {
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
      return mapOf(fromNode(n.rest, ctx, true));
    case "record": {
      const fields: [string, Field][] = [];
      const seen = new Set<string>();
      for (const f of n.fields) {
        if (seen.has(f.key)) throw new TypeError(`the key ${f.key} is written twice in one record`);
        seen.add(f.key);
        fields.push([f.key, { type: fromNode(f.type, ctx, true), optional: f.optional }]);
      }
      const rest = n.rest === "exact" ? NEVER : n.rest === "any" ? ANY : fromNode(n.rest, ctx, true);
      return record(fields, rest);
    }
    case "name": {
      const own = ctx.name?.(n.name, guarded);
      if (own !== undefined) return own;
      const named = ctx.named?.get(n.name);
      if (named !== undefined) return named;
      if (ctx.opaque.has(n.name)) return opaque(n.name);
      if (ctx.atoms !== undefined) {
        let r = ctx.atoms.get(n.name);
        if (r === undefined) ctx.atoms.set(n.name, (r = newRef(n.name)));
        return refType(r);
      }
      throw new TypeError(`${n.name} is not a declared opaque type`);
    }
    case "var": {
      const v = ctx.variable?.(n.name);
      if (v === undefined) throw new TypeError(`the type variable ${n.name} has no solution`);
      return v;
    }
    case "list":
      return listOf(fromNode(n.elem, ctx, true));
    case "union": {
      // f05 3.1: the records of a Type, a parenthesized one and a named union giving their variants.
      const acc = new TypeAcc();
      const recs: Rec[] = [];
      const xs = n.alts.map((a) => fromNode(a, ctx, guarded));
      // f17 12.2 item 5: a complete reference that shares a kind with another alternative counts as its definition.
      const parts = unionParts(xs);
      n.alts.forEach((a, i) => {
        const x = parts[i];
        if (a.t !== "var" && !isAny(x) && x.map) {
          recs.push(...variantsOf(x));
          acc.add(withoutMap(x));
        } else acc.add(x);
      });
      if (recs.length > 0) {
        const m = mapAlt(recs);
        if (m === null) throw new TypeError("records that form no tagged union in one type");
        acc.add(m);
      }
      return acc.type();
    }
  }
}

// ---- the helpers of package F4 (f04 6.2) ------------------------------------------

/** S surely fits T: Fits holds, by a fit that did not use any. `any` surely fits only `any`. */
export function surelyFits(s: Type, t: Type): boolean {
  if (isAny(t)) return true;
  if (isAny(s)) return false;
  return fits(s, t) === 1;
}

/**
 * disjoint(A, B): the checker can show that no value conforms to both (f04
 * 6.2). With a reference anywhere in either, the rule of f17 12.5 item 5.
 */
export function disjoint(a: Type, b: Type): boolean {
  if (isNever(a) || isNever(b)) return true;
  if (isAny(a) || isAny(b)) return false;
  if (hasRefDeep(a) || hasRefDeep(b)) return compute((c) => disjointG(c, a, b));
  return disjoint0(a, b);
}

function disjoint0(a: Alts, b: Alts): boolean {
  if (a.null && b.null) return false;
  if (a.bool && b.bool) return false;
  if (a.num && b.num) return false;
  if (a.opaque && b.opaque && [...a.opaque].some((n) => b.opaque!.has(n))) return false;
  if (a.str && b.str) {
    if (a.str === "string" || b.str === "string") return false;
    const bs = b.str;
    if ([...a.str].some((x) => bs.has(x))) return false;
  }
  if (a.list && b.list) return false;
  // f05 4.5: unless every variant of A is disjoint from every variant of B.
  if (a.map && b.map) {
    const bv = variantsOf(b);
    if (!variantsOf(a).every((x) => bv.every((y) => recordsDisjoint(x, y)))) return false;
  }
  return true;
}

function recordsDisjoint(r: Rec, s: Rec): boolean {
  const keys = new Set<string>();
  for (const [k, f] of r.fields) if (!f.optional) keys.add(k);
  for (const [k, f] of s.fields) if (!f.optional) keys.add(k);
  for (const k of keys) {
    const a = r.fields.get(k)?.type ?? r.rest;
    const b = s.fields.get(k)?.type ?? s.rest;
    if (disjoint(a, b)) return true;
  }
  return false;
}

function* disjointG(c: Comp, a: Type, b: Type): G<boolean> {
  if (isNever(a) || isNever(b)) return true;
  if (isAny(a) || isAny(b)) return false;
  if (!hasRefDeep(a) && !hasRefDeep(b)) return disjoint0(a, b);
  if (a.refs === undefined && b.refs === undefined) return (yield altsDisjointG(c, a, b)) as boolean;
  // The same type, or a pair in progress, is not disjoint; a pair shown disjoint is so at once.
  if (c.tid(a) === c.tid(b)) return false;
  const key = c.pair(a, b);
  if (c.disOn.has(key)) return false;
  if (c.disHolds.has(key)) return true;
  c.enter();
  c.disOn.add(key);
  const ua = unfold(a);
  const ub = unfold(b);
  let r: boolean;
  if (isAny(ua) || isAny(ub)) r = false;
  else r = (yield altsDisjointG(c, ua, ub)) as boolean;
  c.disOn.delete(key);
  if (r) c.disHolds.add(key);
  return r;
}

/** Two types with no reference among their alternatives, in the order of printing (f17 12.5 item 6). */
function* altsDisjointG(c: Comp, a: Alts, b: Alts): G<boolean> {
  if (isNever(a) || isNever(b)) return true;
  if (a.bool && b.bool) return false;
  if (a.num && b.num) return false;
  if (a.str && b.str) {
    if (a.str === "string" || b.str === "string") return false;
    const bs = b.str;
    if ([...a.str].some((x) => bs.has(x))) return false;
  }
  if (a.list && b.list) return false;
  if (a.map && b.map) {
    const bv = variantsPrinted(b);
    for (const x of variantsPrinted(a)) {
      for (const y of bv) if (!((yield recordsDisjointG(c, x, y)) as boolean)) return false;
    }
  }
  if (a.opaque && b.opaque && [...a.opaque].some((n) => b.opaque!.has(n))) return false;
  if (a.null && b.null) return false;
  return true;
}

function* recordsDisjointG(c: Comp, r: Rec, s: Rec): G<boolean> {
  const keys = new Set<string>();
  for (const [k, f] of r.fields) if (!f.optional) keys.add(k);
  for (const [k, f] of s.fields) if (!f.optional) keys.add(k);
  for (const k of [...keys].sort(compareText)) {
    const a = r.fields.get(k)?.type ?? r.rest;
    const b = s.fields.get(k)?.type ?? s.rest;
    if ((yield disjointG(c, a, b)) as boolean) return true;
  }
  return false;
}

/**
 * without(X, T): what is left of X when a value of type X does not conform to
 * T (f04 6.2). f17 12.5 item 8: each reference of X that shares a kind with T,
 * null included, is unfolded; the others stay.
 */
export function without(x0: Type, t: Type): Type {
  if (isAny(x0)) return x0;
  let x: Type = x0;
  let keep: readonly Ref[] | undefined;
  if (x0.refs !== undefined) {
    const ut = unfold(t);
    const tk = kindsOf(ut);
    const tn = isAny(ut) || ut.null === true;
    x = unfoldSome(x0, (r) => r.def === undefined || isAny(ut) || shares(kindsOf(r.def), tk) || (tn && hasNull(refType(r))));
    if (isAny(x)) return x;
    keep = x.refs;
    x = withoutRefs(x);
  }
  const out: Mutable = {};
  const keep1 = (alt: Alts) => !surelyFits(alt, t);
  if (x.null && keep1(NULL)) out.null = true;
  if (x.bool && keep1(BOOL)) out.bool = true;
  if (x.num && keep1({ num: x.num })) out.num = x.num;
  if (x.str) {
    if (x.str === "string") {
      if (keep1(STRING)) out.str = "string";
    } else if (keep1({ str: x.str })) {
      const u = unfold(t);
      const ts = !isAny(u) && u.str !== undefined && u.str !== "string" ? u.str : null;
      const left = ts === null ? [...x.str] : [...x.str].filter((v) => !ts.has(v));
      if (left.length > 0) out.str = new Set(left);
    }
  }
  if (x.list && keep1({ list: x.list })) out.list = x.list;
  if (x.map) {
    // f05 4.5: the map alternative loses each variant that surely fits T.
    const left = variantsOf(x).filter((v) => keep1({ map: v }));
    if (left.length === variantsOf(x).length) {
      out.map = x.map;
      if (x.union) out.union = x.union;
    } else if (left.length > 0) Object.assign(out, variantsType(left));
  }
  if (x.opaque) {
    const names = [...x.opaque].filter((n) => keep1(opaque(n)));
    if (names.length > 0) out.opaque = new Set(names);
  }
  if (keep !== undefined) out.refs = keep;
  return out;
}

// ---- tagged unions (package F5, f05 2) ----------------------------------------------

/** The variants of a map alternative: of a union, its variants; of a record, that record. */
export function variantsOf(t: Alts): readonly Rec[] {
  if (t.union) return t.union.variants;
  return t.map ? [t.map] : [];
}

/** A type less its map alternative. */
export function withoutMap(t: Alts): Alts {
  const { map: _m, union: _u, ...rest } = t;
  return rest;
}

/** A literal set and nothing else (f05 2.1 item 1.2). */
function litSetOnly(t: Type): ReadonlySet<string> | null {
  if (isAny(t) || t.str === undefined || t.str === "string") return null;
  if (t.null || t.bool || t.num || t.list || t.map || (t.opaque && t.opaque.size > 0) || t.refs !== undefined) return null;
  return t.str;
}

/**
 * The tag of records (f05 2.1): the least key that each lists as required,
 * typed by a literal set alone, with no string in two sets. A key that a
 * record fills is no tag (f05 8.1 item 6). null when there is none.
 */
export function findTag(recs: readonly Rec[]): string | null {
  if (recs.length < 2) return null;
  const keys = [...recs[0].fields.keys()].sort(compareText);
  for (const k of keys) {
    const seen = new Set<string>();
    let ok = true;
    for (const r of recs) {
      const f = r.fields.get(k);
      const set = f === undefined || f.optional || f.fill !== undefined ? null : litSetOnly(f.type);
      if (set === null) {
        ok = false;
        break;
      }
      for (const x of set) {
        if (seen.has(x)) {
          ok = false;
          break;
        }
        seen.add(x);
      }
      if (!ok) break;
    }
    if (ok) return k;
  }
  return null;
}

const flatMemo = new WeakMap<Union, Rec>();

/** flat(U) (f05 2.3). */
function flatOf(u: Union): Rec {
  let f = flatMemo.get(u);
  if (f === undefined) {
    f = u.variants.reduce((x, y) => joinRec(x, y));
    flatMemo.set(u, f);
  }
  return f;
}

/** A type whose only alternative is a map alternative of these variants, or null when two or more form no union. */
export function mapAlt(recs: readonly Rec[]): Alts | null {
  if (recs.length === 0) return NEVER;
  if (recs.length === 1) return { map: recs[0] };
  const tag = findTag(recs);
  if (tag === null) return null;
  const u: Union = { variants: [...recs], tag };
  return { map: flatOf(u), union: u };
}

/** The type of a list of variants (f05 4.5 item 2). */
export function variantsType(recs: readonly Rec[]): Alts {
  return mapAlt(recs) ?? { map: recs.reduce((x, y) => joinRec(x, y)) };
}

/** The tag type of a union: the literal set of every string of every tag set (f05 2.3 item 2). */
export function tagSetOf(v: Rec, tag: string): ReadonlySet<string> {
  return (v.fields.get(tag)!.type as Alts).str as ReadonlySet<string>;
}

/** Unite(A, B) (f05 4.5). */
export function unite(a: Type, b: Type): Type {
  const j = join(a, b);
  if (isAny(a) || isAny(b) || isAny(j)) return j;
  const ua = unfold(a);
  const ub = unfold(b);
  if (isAny(ua) || isAny(ub) || !ua.map || !ub.map || !j.map) return j;
  const recs: Rec[] = [];
  for (const v of [...variantsOf(ua), ...variantsOf(ub)]) if (!recs.some((w) => sameRecord(v, w))) recs.push(v);
  if (recs.length < 2) return j;
  const m = mapAlt(recs);
  if (m === null) return j;
  return { ...withoutMap(j), ...m };
}

/** The variants of M in which the type of f is not disjoint from the literal set of S (f05 4.5). */
export function hitVariants(m0: Alts, f: string, strs: ReadonlySet<string>): Rec[] {
  const m = unfold(m0);
  if (isAny(m)) return [];
  const s: Alts = { str: strs };
  return variantsOf(m).filter((v) => !disjoint(keyType(v, f), s));
}

/** The variants of M that require f with a type that surely fits the literal set of S (f05 4.5). */
export function allVariants(m0: Alts, f: string, strs: ReadonlySet<string>): Rec[] {
  const m = unfold(m0);
  if (isAny(m)) return [];
  const s: Alts = { str: strs };
  return variantsOf(m).filter((v) => {
    const fl = v.fields.get(f);
    return fl !== undefined && !fl.optional && surelyFits(fl.type, s);
  });
}

/** The type of a key in a record (f05 4.5 item 1). */
export function keyType(r: Rec, k: string): Type {
  return r.fields.get(k)?.type ?? r.rest;
}
