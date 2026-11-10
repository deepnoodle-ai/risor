// Package F1: the type of a JSON Schema (f01-host-types.md sections 3 and 4).
//
// A schema is a plain JSON value. Only what the rules read is looked at, so a
// keyword the mapping does not read is never checked (3.1 item 1). The type
// of a `$ref` target is mapped once and shared, never copied (3.10 item 5).

import {
  ANY, BOOL, FLOAT, INT, NEVER, NULL, STRING, SIZE_LIMIT, TypeAcc, fits, isAny, isNever, join, lit, listOf, nonnull, record, typeSize,
} from "./type.ts";
import type { Alts, Field, Rec, Type } from "./type.ts";

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
  if (left.every(onlyRecord)) {
    let r = left[0].map!;
    for (let i = 1; i < left.length; i++) r = meet(r, (left[i] as Alts).map!);
    return { map: r };
  }
  for (const t of left) if (left.every((u) => u === t || fits(t, u) === 1)) return t;
  return left[0];
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
      const fields = new Map<string, Field>();
      for (const [k, f] of t.map.fields) fields.set(k, { type: close(f.type, memo), optional: f.optional });
      o.map = { fields, rest: t.map.closable ? NEVER : close(t.map.rest, memo) };
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

class Mapper {
  private readonly root: unknown;
  private readonly opts: ReadonlySet<SchemaOption>;
  /** The type and depth of each `$ref` target, mapped once (3.10 item 5). */
  private readonly targets = new Map<unknown, Mapped>();
  /** Targets whose mapping is in progress, the root among them (3.7 item 7). */
  private readonly progress = new Set<unknown>();

  constructor(root: unknown, opts: ReadonlySet<SchemaOption>) {
    this.root = root;
    this.opts = opts;
  }

  run(): Type {
    this.progress.add(this.root);
    const m = this.map(this.root, true, 0);
    let t = m.type;
    if (this.opts.has("closed")) t = close(t);
    if (typeDepth(t) > MAX_DEPTH) fail("the type is deeper than 64");
    return t;
  }

  /** M(S), at `level` levels below the root. Its size is tested (f01 3.10 item 8). */
  private map(s: unknown, isRoot: boolean, level: number): Mapped {
    const m = this.map0(s, isRoot, level);
    if (typeSize(m.type) > SIZE_LIMIT) fail("the type is larger than 100,000");
    return m;
  }

  private map0(s: unknown, isRoot: boolean, level: number): Mapped {
    if (level > MAX_DEPTH) fail("the schema is deeper than 64");
    if (s === true) return { type: ANY, depth: 0 };
    if (s === false) return { type: NEVER, depth: 0 };
    if (!isObj(s)) fail("a schema is an object or a boolean");
    if (!isRoot && has(s, "$id")) fail("$id below the root");
    if (has(s, "$dynamicRef") || has(s, "$recursiveRef")) fail("$dynamicRef and $recursiveRef are not read");
    if (has(s, "$ref")) {
      const target = this.resolve(s.$ref);
      const m = this.target(target, level);
      return { type: s.nullable === true ? join(m.type, NULL) : m.type, depth: m.depth + 1 };
    }
    const parts: Type[] = [];
    let depth = 0;
    const sub = (x: unknown): Type => {
      const m = this.map(x, false, level + 1);
      depth = Math.max(depth, m.depth + 1);
      return m.type;
    };
    if (has(s, "type") || has(s, "enum") || has(s, "const")) parts.push(this.kindPart(s, sub));
    for (const k of ["allOf", "anyOf", "oneOf"] as const) {
      if (!has(s, k)) continue;
      const branches = s[k];
      if (!Array.isArray(branches) || branches.length === 0) fail(`${k} is a non-empty array of schemas`);
      const types = branches.map(sub);
      if (k === "allOf") parts.push(pick(types));
      else {
        tagTest(types);
        let t: Type = NEVER;
        for (const x of types) t = join(t, x);
        parts.push(t);
      }
    }
    let t = parts.length === 0 ? ANY : pick(parts);
    if (s.nullable === true) t = join(t, NULL);
    return { type: t, depth };
  }

  /** The type of a `$ref` target, mapped once and shared. */
  private target(target: unknown, level: number): Mapped {
    if (this.progress.has(target)) fail("a cycle of $ref");
    const known = this.targets.get(target);
    if (known !== undefined) {
      if (level + 1 + known.depth > MAX_DEPTH) fail("the schema is deeper than 64");
      return known;
    }
    this.progress.add(target);
    try {
      const m = this.map(target, target === this.root, level + 1);
      this.targets.set(target, m);
      return m;
    } finally {
      this.progress.delete(target);
    }
  }

  /** Resolve a `$ref` in the root (3.7). */
  private resolve(ref: unknown): unknown {
    if (typeof ref !== "string" || !ref.startsWith("#")) fail("$ref is a local JSON Pointer");
    const frag = percentDecode(ref.slice(1));
    if (frag === "") return this.root;
    if (!frag.startsWith("/")) fail("$ref is a JSON Pointer, not a plain name");
    let cur: unknown = this.root;
    for (const raw of frag.slice(1).split("/")) {
      if (/~[^01]|~$/.test(raw)) fail("a bad ~ escape in a JSON Pointer");
      const tok = raw.replace(/~1/g, "/").replace(/~0/g, "~");
      if (Array.isArray(cur)) {
        if (!/^(0|[1-9][0-9]*)$/.test(tok) || Number(tok) >= cur.length) fail("a JSON Pointer names nothing");
        cur = cur[Number(tok)];
      } else if (isObj(cur)) {
        if (!has(cur, tok)) fail("a JSON Pointer names nothing");
        cur = cur[tok];
      } else fail("a JSON Pointer names nothing");
    }
    if (!isObj(cur) && typeof cur !== "boolean") fail("$ref names what is not a schema");
    return cur;
  }

  /** The kind part (3.2, 3.3). */
  private kindPart(s: Obj, sub: (x: unknown) => Type): Type {
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
      // One accumulating Join: n log n (ruling CG10). Each literal type is tested (3.10 item 8).
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

  private named(n: string, s: Obj, sub: (x: unknown) => Type): Type {
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
  private elements(s: Obj, sub: (x: unknown) => Type): Type {
    let e: Type = NEVER;
    const isSchema = (x: unknown) => isObj(x) || typeof x === "boolean";
    if (has(s, "prefixItems")) {
      if (!Array.isArray(s.prefixItems) || !s.prefixItems.every(isSchema)) fail("prefixItems is an array of schemas");
      for (const x of s.prefixItems) e = join(e, sub(x));
    }
    if (!has(s, "items")) return join(e, ANY);
    const items = s.items;
    if (Array.isArray(items)) {
      if (!items.every(isSchema)) fail("items is a schema or an array of schemas");
      for (const x of items) e = join(e, sub(x));
      if (has(s, "additionalItems")) {
        if (!isSchema(s.additionalItems)) fail("additionalItems is a schema");
        e = join(e, sub(s.additionalItems));
      } else e = join(e, ANY);
      return e;
    }
    if (!isSchema(items)) fail("items is a schema or an array of schemas");
    return join(e, sub(items));
  }

  /** The record of an object (3.8, 4). */
  private object(s: Obj, sub: (x: unknown) => Type): Rec {
    const isSchema = (x: unknown) => isObj(x) || typeof x === "boolean";
    let rest: Type = ANY;
    const explicit = has(s, "additionalProperties");
    if (explicit) {
      if (!isSchema(s.additionalProperties)) fail("additionalProperties is a schema");
      rest = sub(s.additionalProperties);
      if (has(s, "patternProperties")) {
        const pp = s.patternProperties;
        if (!isObj(pp) || !Object.values(pp).every(isSchema)) fail("patternProperties is an object of schemas");
        for (const k of Object.keys(pp)) rest = join(rest, sub(pp[k]));
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
      for (const k of Object.keys(props)) {
        const ps = props[k];
        const t = sub(ps);
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

/** The test of 3.9: records of one `anyOf` or `oneOf` that have a tag are bad-type until F5. */
function tagTest(types: Type[]): void {
  const recs = types.filter((t): t is Alts & { map: Rec } => !isAny(t) && t.map !== undefined).map((t) => t.map);
  if (recs.length < 2) return;
  const literalSet = (t: Type): ReadonlySet<string> | null => {
    if (isAny(t) || !t.str || t.str === "string") return null;
    if (t.null || t.bool || t.num || t.list || t.map || (t.opaque && t.opaque.size > 0)) return null;
    return t.str;
  };
  for (const [k] of recs[0].fields) {
    const sets: ReadonlySet<string>[] = [];
    let tag = true;
    for (const r of recs) {
      const f = r.fields.get(k);
      const set = f !== undefined && !f.optional ? literalSet(f.type) : null;
      if (set === null) {
        tag = false;
        break;
      }
      sets.push(set);
    }
    if (!tag) continue;
    const seen = new Set<string>();
    let disjoint = true;
    for (const set of sets) {
      for (const x of set) {
        if (seen.has(x)) disjoint = false;
      }
      for (const x of set) seen.add(x);
    }
    if (disjoint) fail(`the branches have the tag ${k}: a tagged union needs package F5`);
  }
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

/** M(S) with the options given (f01 3, 4). Throws SchemaError (`bad-type`). */
export function typeOfSchema(schema: unknown, options: Iterable<SchemaOption> = []): Type {
  return new Mapper(schema, new Set(options)).run();
}

export { nonnull };
