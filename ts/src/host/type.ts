// The structural type API (ruling R179; 05 Part E, E.7): a `Type` a host can
// build, read, join and compare. It wraps the checker's own type model
// (src/types/type.ts), so a type a host builds is the type the checker uses,
// and a check result's type is one of these.
//
// A type is immutable and may be shared. A constructor never fails: a
// construction that has no type, or passes a limit of 05-types.md 20.4 or
// 20.7, gives an invalid type, `err()` reports it, and every declaration
// refuses it with `bad-type`. Its text form (`toString()`) is the type text of
// 05-types.md 3.9.

import * as T from "../types/type.ts";
import type { Alts, Rec } from "../types/type.ts";
import { Classifier, DEFAULT_CLASSIFIER, compareText } from "../value/value.ts";
import { declareError, RisorError } from "./error.ts";

/**
 * The kind of a type of one alternative (05 E.7): `string` covers a literal
 * set, `record` any map type, `union` a tagged union, `var` a type variable of
 * a signature. `mixed` is a type of two or more alternatives, so `T | null` is
 * mixed. `invalid` is an invalid type.
 */
export type TypeKind = "never" | "any" | "null" | "bool" | "int" | "float" | "string" | "list" | "record" | "union" | "opaque" | "var" | "mixed" | "invalid";

/** A field of a record type (05 E.7). */
export interface FieldType {
  key: string;
  type: Type;
  optional: boolean;
}

// A type variable of a printed signature is held as an opaque name that
// starts with U+FFFF (src/host/api.ts printSignature).
const VAR = 0xffff;

let wrap: (t: T.Type, cls?: Classifier) => Type;
let unwrap: (t: Type) => T.Type;
let classifierOf: (t: Type) => Classifier;
let invalidOf: (message: string) => Type;

/** A type (03; 05-types.md). Build one with the constructors of this module, `env.parseType` or `typeFromSchema`. */
export class Type {
  // The type, or null for an invalid type.
  readonly #t: T.Type | null;
  readonly #err: string | null;
  // The opaque tests of the environment the type was read in, for the
  // conformance function; the default has none.
  readonly #cls: Classifier;

  private constructor(t: T.Type | null, err: string | null, cls: Classifier) {
    this.#t = t;
    this.#err = err;
    this.#cls = cls;
  }

  static {
    wrap = (t, cls = DEFAULT_CLASSIFIER) => {
      if (T.depth(t, 70) > 64) return new Type(null, "a type deeper than 64", cls);
      if (T.typeSize(t) > T.SIZE_LIMIT) return new Type(null, "a type larger than 100,000", cls);
      return new Type(t, null, cls);
    };
    unwrap = (t) => {
      if (t.#t === null) throw declareError("bad-type", t.#err!);
      return t.#t;
    };
    classifierOf = (t) => t.#cls;
    invalidOf = (message) => new Type(null, message, DEFAULT_CLASSIFIER);
  }

  #alts(): Alts | null {
    const t = this.#t;
    return t === null || T.isAny(t) ? null : t;
  }

  /** The kind of the type (see `TypeKind`). */
  kind(): TypeKind {
    const t = this.#t;
    if (t === null) return "invalid";
    if (T.isAny(t)) return "any";
    if (T.isNever(t)) return "never";
    const alts = T.alternatives(t);
    if (alts.length > 1) return "mixed";
    const a = alts[0];
    if (a.null) return "null";
    if (a.bool) return "bool";
    if (a.num) return a.num;
    if (a.str) return "string";
    if (a.list) return "list";
    if (a.map) return a.union ? "union" : "record";
    return [...a.opaque!][0].charCodeAt(0) === VAR ? "var" : "opaque";
  }

  /** The element type of the list, or undefined when the type has no list alternative. */
  elem(): Type | undefined {
    const l = this.#alts()?.list;
    return l === undefined ? undefined : wrap(l, this.#cls);
  }

  #record(): Rec | undefined {
    return this.#alts()?.map;
  }

  #field(key: string, f: T.Field): FieldType {
    return { key, type: wrap(f.type, this.#cls), optional: f.optional };
  }

  /**
   * The fields of the record, in code point order of their keys. For a tagged
   * union, the fields of the record a read sees (f05 2.3). Empty when the type
   * has no map alternative.
   */
  fields(): FieldType[] {
    const r = this.#record();
    if (r === undefined) return [];
    return [...r.fields.keys()].sort(compareText).map((k) => this.#field(k, r.fields.get(k)!));
  }

  /** One field of the record by its key, or undefined. */
  field(key: string): FieldType | undefined {
    const f = this.#record()?.fields.get(key);
    return f === undefined ? undefined : this.#field(key, f);
  }

  /** The rest of the map alternative: `typeNever` for an exact record, `typeAny` for an open one; undefined when there is none. */
  rest(): Type | undefined {
    const r = this.#record();
    return r === undefined ? undefined : wrap(r.rest, this.#cls);
  }

  /**
   * The alternatives, one of each kind: null, bool, the number, the string or
   * literal set, the list, the map type, and each opaque type. `any` is its own
   * one alternative; never and an invalid type have none.
   */
  alternatives(): Type[] {
    const t = this.#t;
    if (t === null) return [];
    if (T.isAny(t)) return [this];
    const out: Alts[] = [];
    if (t.null) out.push(T.NULL);
    if (t.bool) out.push(T.BOOL);
    if (t.num) out.push({ num: t.num });
    if (t.str) out.push({ str: t.str });
    if (t.list) out.push({ list: t.list });
    if (t.map) out.push(t.union ? { map: t.map, union: t.union } : { map: t.map });
    for (const n of [...(t.opaque ?? [])].sort(compareText)) out.push({ opaque: new Set([n]) });
    return out.map((a) => wrap(a, this.#cls));
  }

  /** The variants of a tagged union, in printing order (f05 2.4); empty when the type has none. */
  variants(): Type[] {
    const a = this.#alts();
    if (a?.union === undefined) return [];
    const tag = a.union.tag;
    const least = (v: Rec) => [...T.tagSetOf(v, tag)].sort(compareText)[0];
    return [...a.union.variants].sort((x, y) => compareText(least(x), least(y))).map((v) => wrap({ map: v }, this.#cls));
  }

  /** The strings of the literal set, sorted; empty when the string alternative is every string or there is none. */
  literals(): string[] {
    const s = this.#alts()?.str;
    return s === undefined || s === "string" ? [] : [...s].sort(compareText);
  }

  /** The tag key of a tagged union (f05 2.1), or undefined. */
  tag(): string | undefined {
    return this.#alts()?.union?.tag;
  }

  /** The type without null. */
  nonNull(): Type {
    return this.#t === null ? this : wrap(T.nonnull(this.#t), this.#cls);
  }

  /** Whether null is one of the alternatives. `any` is not nullable by this test. */
  isNullable(): boolean {
    return this.#t !== null && T.hasNull(this.#t);
  }

  /** Whether the map alternative is an exact record: no keys beyond its fields. */
  isExact(): boolean {
    const r = this.#record();
    return r !== undefined && T.isNever(r.rest);
  }

  /** The name of an opaque type, or "" for any other kind. */
  opaqueName(): string {
    return this.kind() === "opaque" ? [...this.#alts()!.opaque!][0] : "";
  }

  /** Whether every value of this type is a value of `want` (05 4.2). False when either is invalid. */
  fits(want: Type): boolean {
    if (this.#t === null || want.#t === null) return false;
    return T.fits(this.#t, want.#t) !== 0;
  }

  /** Whether the two are the same type. False when either is invalid. */
  equal(u: Type): boolean {
    if (this.#t === null || u.#t === null) return false;
    return T.typeEqual(this.#t, u.#t);
  }

  /** The type text (05-types.md 3.9). An invalid type prints as `invalid`. */
  toString(): string {
    return this.#t === null ? "invalid" : T.printType(this.#t);
  }

  /** The `bad-type` failure of an invalid type, or undefined. */
  err(): RisorError | undefined {
    return this.#err === null ? undefined : declareError("bad-type", this.#err);
  }
}

/** Internal: wrap a type of the checker; read one, throwing `bad-type` for an invalid type; its opaque tests. */
export { wrap as typeOf, unwrap as innerType, classifierOf as typeClassifier };

export const typeNever: Type = wrap(T.NEVER);
export const typeAny: Type = wrap(T.ANY);
export const typeNull: Type = wrap(T.NULL);
export const typeBool: Type = wrap(T.BOOL);
export const typeInt: Type = wrap(T.INT);
export const typeFloat: Type = wrap(T.FLOAT);
export const typeString: Type = wrap(T.STRING);

/** The first invalid type, or null. */
function firstInvalid(ts: Type[]): Type | null {
  return ts.find((t) => t.kind() === "invalid") ?? null;
}

/** The classifier of the first type that has opaque tests. */
function clsOf(ts: Type[]): Classifier {
  for (const t of ts) {
    const c = classifierOf(t);
    if (c !== DEFAULT_CLASSIFIER) return c;
  }
  return DEFAULT_CLASSIFIER;
}

/** Build from valid parts, or give the first invalid part. */
function build(parts: Type[], f: (ts: T.Type[]) => T.Type | string): Type {
  const bad = firstInvalid(parts);
  if (bad !== null) return bad;
  const r = f(parts.map(unwrap));
  return typeof r === "string" ? invalidOf(r) : wrap(r, clsOf(parts));
}

/** A list type: `T[]`. */
export function listType(elem: Type): Type {
  return build([elem], ([e]) => T.listOf(e));
}

/** A map type: `map<T>`, a record with no fields and rest T. */
export function mapType(value: Type): Type {
  return build([value], ([v]) => T.mapOf(v));
}

/** The type or null: `T | null`. */
export function nullable(t: Type): Type {
  return build([t], ([x]) => T.withNull(x));
}

/** The union of one or more string literals: `"a" | "b"`. No string is no type. */
export function literalType(...values: string[]): Type {
  if (values.length === 0) return invalidOf("a literal type has one or more strings");
  return wrap(T.lit(...values));
}

/**
 * A record type: its fields, and the type of its other keys, `typeNever` for
 * an exact record and `typeAny` for an open one. A key given twice is no type.
 */
export function recordType(fields: FieldType[], rest: Type): Type {
  return build([...fields.map((f) => f.type), rest], (ts) => {
    const seen = new Set<string>();
    const out: [string, T.Field][] = [];
    for (let i = 0; i < fields.length; i++) {
      const k = fields[i].key;
      if (seen.has(k)) return `the key ${k} is written twice in one record`;
      seen.add(k);
      out.push([k, { type: ts[i], optional: fields[i].optional === true }]);
    }
    return T.record(out, ts[fields.length]);
  });
}

/** An exact record type: `recordType(fields, typeNever)`. */
export function exactRecordType(...fields: FieldType[]): Type {
  return recordType(fields, typeNever);
}

/**
 * The union of types, as type text `A | B` gives it (f05 3.1): the records
 * among them form a tagged union, and two or more records that form none are
 * no type.
 */
export function unionType(...ts: Type[]): Type {
  return build(ts, (xs) => {
    const acc = new T.TypeAcc();
    const recs: Rec[] = [];
    for (const t of xs) {
      if (!T.isAny(t) && t.map) {
        recs.push(...T.variantsOf(t));
        acc.add(T.withoutMap(t));
      } else acc.add(t);
    }
    if (recs.length > 0) {
      const m = T.mapAlt(recs);
      if (m === null) return "records that form no tagged union in one type";
      acc.add(m);
    }
    return acc.type();
  });
}

/**
 * The tagged union of two or more records (package F5, f05 2.1). A type that
 * is not a record, or records with no tag, are no type.
 */
export function taggedUnionType(...records: Type[]): Type {
  return build(records, (xs) => {
    const recs: Rec[] = [];
    for (let i = 0; i < xs.length; i++) {
      const k = records[i].kind();
      if (k !== "record" && k !== "union") return `a variant of a tagged union is a record, not ${records[i].toString()}`;
      recs.push(...T.variantsOf(xs[i] as Alts));
    }
    if (recs.length < 2) return "a tagged union has two or more records";
    const m = T.mapAlt(recs);
    if (m === null || T.findTag(recs) === null) return "the records have no tag";
    return m;
  });
}

/** The opaque type of this name. An environment that does not declare it refuses it with `bad-type`. */
export function opaqueTypeNamed(name: string): Type {
  if (!/^[A-Z][A-Za-z0-9_]+$/.test(name)) return invalidOf(`${name} is not an opaque type name`);
  return wrap(T.opaque(name));
}

/** The join of two types (05-types.md 4.1). */
export function join(a: Type, b: Type): Type {
  return build([a, b], ([x, y]) => T.join(x, y));
}

/** Internal: the opaque type names a type mentions. */
export function opaqueNamesIn(t: T.Type): Set<string> {
  const out = new Set<string>();
  const seen = new Set<object>();
  const walk = (x: T.Type): void => {
    if (T.isAny(x) || seen.has(x)) return;
    seen.add(x);
    for (const n of x.opaque ?? []) out.add(n);
    if (x.list) walk(x.list);
    const recs = x.union ? [...x.union.variants, x.map!] : x.map ? [x.map] : [];
    for (const r of recs) {
      for (const f of r.fields.values()) walk(f.type);
      walk(r.rest);
    }
  };
  walk(t);
  return out;
}
