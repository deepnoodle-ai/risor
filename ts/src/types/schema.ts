// Package F1: the type of a JSON Schema (f01-host-types.md sections 3 and 4).
//
// A schema is a plain JSON value. Only what the rules read is looked at, so a
// keyword the mapping does not read is never checked (3.1 item 1). The type
// of a `$ref` target is mapped once and shared, never copied (3.10 item 5).
//
// Package F17 (f17 12.4): a target met again while it is being mapped, with
// an array's items or an object's properties between, is a recursive type: it
// gets a reference, whose definition is set when the mapping ends.

import {
  ANY, BOOL, FLOAT, INT, NEVER, NULL, STRING, SIZE_LIMIT, TypeAcc, fits, isAny, isNever, join, lit, listOf, nonnull, record, typeSize,
  mapAlt, variantsOf, withoutMap, IncompleteRef, PairLimit, kindsOf, mergeKinds, newRef, refType, withoutRefs, unfold,
} from "./type.ts";
import { compareText } from "../value/value.ts";
import type { Alts, Field, Rec, Ref, Type } from "./type.ts";
import { besideRef } from "./list.ts";

export type SchemaOption = "closed" | "absent-is-null" | "defaults-applied";
export const SCHEMA_OPTIONS: readonly SchemaOption[] = ["closed", "absent-is-null", "defaults-applied"];

/** An error of a schema: the `declare` error `bad-type` (3.11). */
export class SchemaError extends Error {}

const MAX_DEPTH = 64;

type Obj = Record<string, unknown>;

function isObj(v: unknown): v is Obj {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function has(o: Obj, k: string): boolean {
  return Object.prototype.hasOwnProperty.call(o, k);
}

function fail(msg: string): never {
  throw new SchemaError(msg);
}

// ---- literal types (3.4) -----------------------------------------------------------

/** The literal type of a JSON value (3.4). */
export function literalType(v: unknown, depth = 0): Type {
  if (depth > 1100) fail("a value nested too deep");
  if (v === null || v === undefined) return NULL;
  if (typeof v === "boolean") return BOOL;
  if (typeof v === "number") return Number.isSafeInteger(v) ? INT : FLOAT;
  if (typeof v === "string") return lit(v);
  if (Array.isArray(v)) {
    const acc = new TypeAcc();
    for (const x of v) acc.add(literalType(x, depth + 1));
    return listOf(acc.type());
  }
  if (isObj(v)) {
    return record(Object.keys(v).map((k) => [k, { type: literalType(v[k], depth + 1), optional: false }]));
  }
  fail("not a JSON value");
}

// ---- Pick, Meet and Close (3.6, 4.1) ------------------------------------------------

/** Whether a type is one record alternative and nothing else. */
function onlyRecord(t: Type): t is Alts & { map: Rec } {
  if (isAny(t) || !t.map) return false;
  return !t.null && !t.bool && !t.num && !t.str && !t.list && !(t.opaque && t.opaque.size > 0);
}

export function pick(ts: Type[]): Type {
  const left = ts.filter((t) => !isAny(t));
  if (left.length === 0) return ANY;
  // f17 12.4 item 4: one type that is not `any` is given without looking into it.
  if (left.length === 1) return left[0];
  // Step 2 reads the kinds of each type, so it looks into each reference among them: one whose definition is not
  // known yet, as while its schema is mapped, throws IncompleteRef (bad-type).
  const kinds = left.map((t) => unfold(t));
  if (kinds.every(onlyRecord)) {
    // Step 2, with package F5 (f05 8.3): records and unions meet, folded from the left.
    let acc: Alts | null = kinds[0] as Alts;
    for (let i = 1; i < kinds.length && acc !== null; i++) acc = meetMaps(acc, kinds[i] as Alts);
    if (acc !== null) return acc;
  }
  for (const t of left) if (left.every((u) => u === t || fits(t, u) === 1)) return t;
  return left[0];
}

/** The meet of two map alternatives (f05 8.3): null when it fails. */
function meetMaps(a: Alts, b: Alts): Alts | null {
  if (!a.union && !b.union) return { map: meet(a.map!, b.map!) };
  if (a.union && b.union) return null;
  const recs = a.union ? a.union.variants.map((v) => meet(v, b.map!)) : b.union!.variants.map((v) => meet(a.map!, v));
  const m = mapAlt(recs);
  return m !== null && m.union ? m : null;
}

export function meet(a: Rec, b: Rec): Rec {
  const fields = new Map<string, Field>();
  const keys = new Set([...a.fields.keys(), ...b.fields.keys()]);
  for (const k of keys) {
    const fa = a.fields.get(k);
    const fb = b.fields.get(k);
    // f01 3.6 item 3, 4.4: the base from the two bases, the fill from each side that fills the key.
    const base = pick([fa !== undefined ? (fa.base ?? fa.type) : a.rest, fb !== undefined ? (fb.base ?? fb.type) : b.rest]);
    const fills = [fa?.fill, fb?.fill].filter((x): x is Type => x !== undefined);
    const required = (fa !== undefined && !fa.optional) || (fb !== undefined && !fb.optional);
    if (fills.length === 0) fields.set(k, { type: base, optional: !required });
    else {
      const fill = fills.length === 1 ? fills[0] : join(fills[0], fills[1]);
      fields.set(k, { type: join(base, fill), optional: false, base, fill });
    }
  }
  const out: Rec = { fields, rest: pick([a.rest, b.rest]) };
  if (a.closable && b.closable) out.closable = true;
  return out;
}

/** Close(T) (4.1 item 3): every closable record gets the rest never, at every depth. */
export function close(t: Type, memo = new Map<Type, Type>()): Type {
  if (isAny(t)) return t;
  const m = memo.get(t);
  if (m !== undefined) return m;
  let out: Type = t;
  if (t.list || t.map) {
    const o: { -readonly [K in keyof Alts]: Alts[K] } = { ...t };
    if (t.list) o.list = close(t.list, memo);
    if (t.map) {
      const closeRec = (r: Rec): Rec => {
        const fields = new Map<string, Field>();
        for (const [k, f] of r.fields) fields.set(k, { type: close(f.type, memo), optional: f.optional });
        return { fields, rest: r.closable ? NEVER : close(r.rest, memo) };
      };
      // f05 8.2 item 2: Close closes each variant of a union.
      if (t.union) Object.assign(o, mapAlt(t.union.variants.map(closeRec)));
      else o.map = closeRec(t.map);
    }
    out = o;
  }
  memo.set(t, out);
  return out;
}

/** The depth of a type (05 2.5), on a type that may share parts. */
function typeDepth(t: Type, memo = new Map<Type, number>()): number {
  if (isAny(t)) return 0;
  const m = memo.get(t);
  if (m !== undefined) return m;
  let d = 0;
  if (t.list) d = Math.max(d, 1 + typeDepth(t.list, memo));
  if (t.map) {
    let x = typeDepth(t.map.rest, memo);
    for (const f of t.map.fields.values()) x = Math.max(x, typeDepth(f.type, memo));
    d = Math.max(d, 1 + x);
  }
  memo.set(t, d);
  return d;
}

// ---- the mapping (3) ------------------------------------------------------------------

const TYPE_NAMES = new Set(["null", "boolean", "integer", "number", "string", "array", "object"]);

interface Mapped {
  type: Type;
  /** The depth of the schema (3.10). */
  depth: number;
}

/**
 * Map a schema at a place below the one being mapped: `at` is the tokens
 * from that place to it (`step`); `guarded`, an array's items or an object's
 * properties stand between (f17 12.4 item 1).
 */
type Sub = (x: unknown, at: string, guarded?: boolean) => Type;

/** The place of the root: no token. */
const ROOT = "";

/** The tokens of a place's pointer, each escaped as a JSON Pointer writes it, so that a place has one spelling. */
function step(...tokens: (string | number)[]): string {
  let out = "";
  for (const t of tokens) out += "/" + String(t).replace(/~/g, "~0").replace(/\//g, "~1");
  return out;
}

/** A type name (f04 3.2). */
const TYPE_NAME = /^[A-Z][A-Za-z0-9_]+$/;

class Mapper {
  private readonly root: unknown;
  private readonly opts: ReadonlySet<SchemaOption>;
  private readonly rootName: string;
  /**
   * The type and depth of each place of the root that is a schema object,
   * mapped once (3.10 item 5): a place has one type, however the mapping
   * meets it, through a `$ref` or as a part of the schema that holds it
   * (f17 12.4 item 1, R302). A place that holds a `$ref` is a place of its own.
   */
  private readonly targets = new Map<string, Mapped>();
  /**
   * The places whose mapping is in progress, the root among them (3.7 item 7),
   * each with the guards that stood above it.
   */
  private readonly progress = new Map<string, { guards: number }>();
  /** The array items and object properties the mapping stands in now (f17 12.4 item 1). */
  private guards = 0;
  /** The recursive types: each target met again, with its reference and, once mapped, its type. */
  private readonly recursive = new Map<string, { ref: Ref; type?: Type }>();

  constructor(root: unknown, opts: ReadonlySet<SchemaOption>, rootName: string) {
    this.root = root;
    this.opts = opts;
    this.rootName = rootName;
  }

  run(): Type {
    let t = this.map(this.root, ROOT, true, 0, "#").type;
    this.setDefinitions();
    if (this.opts.has("closed")) {
      const memo = new Map<Type, Type>();
      t = close(t, memo);
      for (const { ref } of this.recursive.values()) ref.def = close(ref.def!, memo);
    }
    if (typeDepth(t) > MAX_DEPTH) fail("the type is deeper than 64");
    // The schema's own type holds no reference beside a type of its kind either: in `anyOf` a reference stood
    // beside the other branches while it was not complete (f17 12.4 item 4), and with its definition known
    // the type would read as two alternatives of one kind, as a definition would (12.2 item 6, R304).
    if (this.recursive.size > 0) {
      const beside = besideRef(t);
      if (beside !== null) fail(`the reference ${beside.name} stands beside a type of its kind`);
    }
    for (const { ref } of this.recursive.values()) {
      // f17 12.4 item 5: 12.2 item 6, then each definition's depth and size (12.6).
      const beside = besideRef(ref.def!);
      if (beside !== null) fail(`the reference ${beside.name} stands beside a type of its kind`);
      if (typeDepth(ref.def!) > MAX_DEPTH) fail(`the type ${ref.name} is deeper than 64`);
      if (typeSize(ref.def!) > SIZE_LIMIT) fail(`the type ${ref.name} is larger than 100,000`);
    }
    return t;
  }

  /** A recursive target is mapped: it stands for its reference from now on. */
  private done(target: string, t: Type): Type {
    const r = this.recursive.get(target)!;
    r.type = t;
    return refType(r.ref);
  }

  /**
   * f17 12.4 item 5: each definition is its target's type, with each
   * reference of the schema at its top replaced by that reference's
   * definition. A reference beside an alternative of its kind there is an
   * error, as it is at any depth (12.2 item 6).
   */
  private setDefinitions(): void {
    const byRef = new Map<Ref, Type>();
    for (const { ref, type } of this.recursive.values()) byRef.set(ref, type!);
    const defs = new Map<Ref, Type>();
    const busy = new Set<Ref>();
    const defOf = (r: Ref): Type => {
      const known = defs.get(r);
      if (known !== undefined) return known;
      if (busy.has(r)) fail(`a cycle of $ref with no array item or property between, through ${r.name}`);
      busy.add(r);
      const t = byRef.get(r)!;
      let out: Type = t;
      if (!isAny(t) && t.refs !== undefined && t.refs.some((x) => byRef.has(x))) {
        const keep = t.refs.filter((x) => !byRef.has(x));
        out = keep.length > 0 ? { ...withoutRefs(t), refs: keep } : withoutRefs(t);
        for (const x of t.refs) {
          if (!byRef.has(x)) continue;
          const d = defOf(x);
          if (isAny(out)) break;
          if (isAny(d)) {
            out = ANY;
            break;
          }
          const others = kindsOf(out);
          for (const k of kindsOf(d)) if (others.has(k)) fail(`the reference ${x.name} stands beside a type of its kind`);
          out = mergeKinds(out as Alts, d);
        }
      }
      busy.delete(r);
      defs.set(r, out);
      return out;
    };
    for (const r of byRef.keys()) defOf(r);
    for (const [r, d] of defs) r.def = d;
  }

  /**
   * M(S), at `level` levels below the root. Its size is tested (f01 3.10 item
   * 8). A place is mapped once, and its type stands wherever the mapping meets
   * it. `place`: where S stands in the root, its decoded tokens as a pointer
   * (f17 12.4 item 1): a place is known by where it stands, not by the object
   * that holds it, so one object at two places is two places. `pointer`: the
   * `$ref` that meets it, which names it if it is met again while it is mapped
   * (12.4 item 2).
   */
  private map(s: unknown, place: string, isRoot: boolean, level: number, pointer: string | null = null): Mapped {
    if (!isObj(s)) return this.map0(s, place, isRoot, level);
    // f01 3.10: the edge to a target counts, also when the target is met again.
    if (level > MAX_DEPTH) fail("the schema is deeper than 64");
    const going = this.progress.get(place);
    if (going !== undefined) {
      // f17 12.4 item 1: met again with an array's items or an object's properties between, a recursive type.
      if (this.guards <= going.guards) fail("a cycle of $ref with no array item or property between");
      return { type: refType(this.recursiveRef(place, pointer)), depth: 0 };
    }
    const known = this.targets.get(place);
    if (known !== undefined) {
      if (level + known.depth > MAX_DEPTH) fail("the schema is deeper than 64");
      return known;
    }
    this.progress.set(place, { guards: this.guards });
    try {
      const m = this.map0(s, place, isRoot, level);
      if (typeSize(m.type) > SIZE_LIMIT) fail("the type is larger than 100,000");
      const out = this.recursive.has(place) ? { type: this.done(place, m.type), depth: m.depth } : m;
      this.targets.set(place, out);
      return out;
    } finally {
      this.progress.delete(place);
    }
  }

  private map0(s: unknown, place: string, isRoot: boolean, level: number): Mapped {
    if (level > MAX_DEPTH) fail("the schema is deeper than 64");
    if (s === true) return { type: ANY, depth: 0 };
    if (s === false) return { type: NEVER, depth: 0 };
    if (!isObj(s)) fail("a schema is an object or a boolean");
    if (!isRoot && has(s, "$id")) fail("$id below the root");
    if (has(s, "$dynamicRef") || has(s, "$recursiveRef")) fail("$dynamicRef and $recursiveRef are not read");
    if (has(s, "$ref")) {
      const target = this.resolve(s.$ref);
      const m = this.map(target.schema, target.place, target.place === ROOT, level + 1, s.$ref as string);
      return { type: s.nullable === true ? join(m.type, NULL) : m.type, depth: m.depth + 1 };
    }
    const parts: Type[] = [];
    let depth = 0;
    const sub: Sub = (x, at, guarded = false) => {
      if (guarded) this.guards++;
      try {
        const m = this.map(x, place + at, false, level + 1);
        depth = Math.max(depth, m.depth + 1);
        return m.type;
      } finally {
        if (guarded) this.guards--;
      }
    };
    if (has(s, "type") || has(s, "enum") || has(s, "const")) parts.push(this.kindPart(s, sub));
    for (const k of ["allOf", "anyOf", "oneOf"] as const) {
      if (!has(s, k)) continue;
      const branches = s[k];
      if (!Array.isArray(branches) || branches.length === 0) fail(`${k} is a non-empty array of schemas`);
      const types = branches.map((b, i) => sub(b, step(k, i)));
      if (k === "allOf") parts.push(pick(types));
      else parts.push(branchUnion(types));
    }
    let t = parts.length === 0 ? ANY : pick(parts);
    if (s.nullable === true) t = join(t, NULL);
    return { type: t, depth };
  }

  /** The reference of a recursive target, named as f17 12.4 item 2 says. */
  private recursiveRef(target: string, pointer: string | null): Ref {
    const known = this.recursive.get(target);
    if (known !== undefined) return known.ref;
    let name: string;
    if (target === ROOT) name = this.rootName;
    else {
      const frag = percentDecode(pointer!.slice(1));
      const raw = frag.slice(frag.lastIndexOf("/") + 1);
      const tok = raw.replace(/~1/g, "/").replace(/~0/g, "~");
      name = /^[a-z]/.test(tok) ? tok[0].toUpperCase() + tok.slice(1) : tok;
    }
    if (!TYPE_NAME.test(name)) fail(`the recursive type ${JSON.stringify(name)} has no type name`);
    for (const r of this.recursive.values()) if (r.ref.name === name) fail(`two recursive types are named ${name}`);
    const ref = newRef(name);
    this.recursive.set(target, { ref });
    return ref;
  }

  /** Resolve a `$ref` in the root (3.7): the schema it names, and its place. */
  private resolve(ref: unknown): { schema: unknown; place: string } {
    if (typeof ref !== "string" || !ref.startsWith("#")) fail("$ref is a local JSON Pointer");
    const frag = percentDecode(ref.slice(1));
    if (frag === "") return { schema: this.root, place: ROOT };
    let place = ROOT;
    if (!frag.startsWith("/")) fail("$ref is a JSON Pointer, not a plain name");
    let cur: unknown = this.root;
    for (const raw of frag.slice(1).split("/")) {
      if (/~[^01]|~$/.test(raw)) fail("a bad ~ escape in a JSON Pointer");
      const tok = raw.replace(/~1/g, "/").replace(/~0/g, "~");
      place += step(tok);
      if (Array.isArray(cur)) {
        if (!/^(0|[1-9][0-9]*)$/.test(tok) || Number(tok) >= cur.length) fail("a JSON Pointer names nothing");
        cur = cur[Number(tok)];
      } else if (isObj(cur)) {
        if (!has(cur, tok)) fail("a JSON Pointer names nothing");
        cur = cur[tok];
      } else fail("a JSON Pointer names nothing");
    }
    if (!isObj(cur) && typeof cur !== "boolean") fail("$ref names what is not a schema");
    return { schema: cur, place };
  }

  /** The kind part (3.2, 3.3). */
  private kindPart(s: Obj, sub: Sub): Type {
    let names: string[] | null = null;
    if (has(s, "type")) {
      const t = s.type;
      const list = typeof t === "string" ? [t] : Array.isArray(t) && t.length > 0 && t.every((x) => typeof x === "string") ? (t as string[]) : null;
      if (list === null || list.some((n) => !TYPE_NAMES.has(n))) fail("type is one of the seven names, or a non-empty array of them");
      names = list;
    }
    if (has(s, "const") || has(s, "enum")) {
      let values: unknown[];
      if (has(s, "const")) values = [s.const];
      else {
        if (!Array.isArray(s.enum)) fail("enum is an array");
        values = s.enum;
      }
      // One accumulating Join: n log n (05 20.7). Each literal type is tested (3.10 item 8).
      const acc = new TypeAcc();
      for (const v of values) {
        if (names !== null && !names.some((n) => allows(n, v))) continue;
        const lt = literalType(v);
        if (typeSize(lt) > SIZE_LIMIT) fail("a literal type larger than 100,000");
        acc.add(lt);
      }
      return acc.type();
    }
    let t: Type = NEVER;
    for (const n of names!) t = join(t, this.named(n, s, sub));
    return t;
  }

  private named(n: string, s: Obj, sub: Sub): Type {
    switch (n) {
      case "null": return NULL;
      case "boolean": return BOOL;
      case "integer": return INT;
      case "number": return FLOAT;
      case "string": return STRING;
      case "array": return listOf(this.elements(s, sub));
    }
    return { map: this.object(s, sub) };
  }

  /** The element type of an array (3.5). */
  private elements(s: Obj, sub: Sub): Type {
    let e: Type = NEVER;
    const isSchema = (x: unknown) => isObj(x) || typeof x === "boolean";
    if (has(s, "prefixItems")) {
      if (!Array.isArray(s.prefixItems) || !s.prefixItems.every(isSchema)) fail("prefixItems is an array of schemas");
      s.prefixItems.forEach((x, i) => (e = join(e, sub(x, step("prefixItems", i), true))));
    }
    if (!has(s, "items")) return join(e, ANY);
    const items = s.items;
    if (Array.isArray(items)) {
      if (!items.every(isSchema)) fail("items is a schema or an array of schemas");
      items.forEach((x, i) => (e = join(e, sub(x, step("items", i), true))));
      if (has(s, "additionalItems")) {
        if (!isSchema(s.additionalItems)) fail("additionalItems is a schema");
        e = join(e, sub(s.additionalItems, step("additionalItems"), true));
      } else e = join(e, ANY);
      return e;
    }
    if (!isSchema(items)) fail("items is a schema or an array of schemas");
    return join(e, sub(items, step("items"), true));
  }

  /** The record of an object (3.8, 4). */
  private object(s: Obj, sub: Sub): Rec {
    const isSchema = (x: unknown) => isObj(x) || typeof x === "boolean";
    let rest: Type = ANY;
    const explicit = has(s, "additionalProperties");
    if (explicit) {
      if (!isSchema(s.additionalProperties)) fail("additionalProperties is a schema");
      rest = sub(s.additionalProperties, step("additionalProperties"), true);
      if (has(s, "patternProperties")) {
        const pp = s.patternProperties;
        if (!isObj(pp) || !Object.values(pp).every(isSchema)) fail("patternProperties is an object of schemas");
        // f17 12.4 item 3: in code point order of the keys.
        for (const k of Object.keys(pp).sort(compareText)) rest = join(rest, sub(pp[k], step("patternProperties", k), true));
      }
    }
    let required = new Set<string>();
    if (has(s, "required")) {
      if (!Array.isArray(s.required) || !s.required.every((x) => typeof x === "string")) fail("required is an array of strings");
      required = new Set(s.required as string[]);
    }
    const fields = new Map<string, Field>();
    const hasProps = has(s, "properties");
    if (hasProps) {
      const props = s.properties;
      if (!isObj(props) || !Object.values(props).every(isSchema)) fail("properties is an object of schemas");
      // f17 12.4 item 3: the mapping meets the properties in code point order of their keys.
      for (const k of Object.keys(props).sort(compareText)) {
        const ps = props[k];
        const t = sub(ps, step("properties", k), true);
        let fill: Type | undefined;
        if (!required.has(k)) {
          if (this.opts.has("defaults-applied") && isObj(ps) && has(ps, "default")) {
            fill = literalType(ps.default);
            if (typeSize(fill) > SIZE_LIMIT) fail("the literal type of a default larger than 100,000");
          } else if (this.opts.has("absent-is-null")) fill = NULL;
        }
        // f01 4.4: a filled field is required, with base t and its fill kept apart.
        if (fill === undefined) fields.set(k, { type: t, optional: !required.has(k) });
        else fields.set(k, { type: join(t, fill), optional: false, base: t, fill });
      }
    }
    for (const k of required) if (!fields.has(k)) fields.set(k, { type: rest, optional: false });
    const r: Rec = { fields, rest };
    if (this.opts.has("closed") && hasProps && !explicit && !has(s, "patternProperties")) r.closable = true;
    return r;
  }
}

/** Whether a name of `type` allows a JSON value (3.3). */
function allows(n: string, v: unknown): boolean {
  switch (n) {
    case "null": return v === null;
    case "boolean": return typeof v === "boolean";
    case "integer": return typeof v === "number" && Number.isInteger(v);
    case "number": return typeof v === "number";
    case "string": return typeof v === "string";
    case "array": return Array.isArray(v);
    case "object": return isObj(v);
  }
  return false;
}

/** The part of an `anyOf` or `oneOf` (f05 8.1): the records of its branches may form a tagged union. */
function branchUnion(types: Type[]): Type {
  let joined: Type = NEVER;
  for (const x of types) joined = join(joined, x);
  if (isAny(joined)) return joined;
  const recs = types.flatMap((t) => (isAny(t) ? [] : variantsOf(t)));
  if (recs.length < 2) return joined;
  const m = mapAlt(recs);
  if (m === null || !m.union) return joined;
  let other: Type = NEVER;
  for (const x of types) other = join(other, withoutMap(x as Alts));
  return { ...(other as Alts), ...m };
}

/** Percent-decoding of a fragment (3.7 item 2): bytes, which must be UTF-8. */
function percentDecode(s: string): string {
  if (!s.includes("%")) return s;
  const bytes: number[] = [];
  const enc = new TextEncoder();
  for (let i = 0; i < s.length; i++) {
    if (s[i] === "%") {
      const h = s.slice(i + 1, i + 3);
      if (!/^[0-9a-fA-F]{2}$/.test(h)) fail("a bad % escape in $ref");
      bytes.push(parseInt(h, 16));
      i += 2;
    } else {
      const cp = s.codePointAt(i)!;
      const ch = String.fromCodePoint(cp);
      bytes.push(...enc.encode(ch));
      if (cp > 0xffff) i++;
    }
  }
  try {
    return new TextDecoder("utf-8", { fatal: true }).decode(new Uint8Array(bytes));
  } catch {
    fail("$ref decodes to bytes that are not UTF-8");
  }
}

/**
 * M(S) with the options given (f01 3, 4). Throws SchemaError (`bad-type`).
 * `rootName`: the name of the root when it is a recursive type (f17 12.4 item
 * 2): the named type it declares, else the variable's name with its first
 * letter upper-cased, else `Root`.
 */
export function typeOfSchema(schema: unknown, options: Iterable<SchemaOption> = [], rootName = "Root"): Type {
  try {
    return new Mapper(schema, new Set(options), rootName).run();
  } catch (e) {
    // f17 12.4 item 4: a rule that looks into a reference of the schema while it is mapped.
    if (e instanceof IncompleteRef) fail("a rule looks into a recursive type while the schema is mapped");
    if (e instanceof PairLimit) fail("a comparison of types went past 4,096 pairs");
    throw e;
  }
}

/** The root name of a schema that declares a variable (f17 12.4 item 2): its name, its first letter upper-cased. */
export function variableRootName(name: string): string {
  return /^[a-z]/.test(name) ? name[0].toUpperCase() + name.slice(1) : name;
}

export { nonnull };
