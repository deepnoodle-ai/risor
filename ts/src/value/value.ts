// Values (01 section 6, 03 3.4).
//
// A value of the language is a JavaScript value, read in place:
//
// | Kind   | Held as |
// | ------ | ------- |
// | null   | `null`; `undefined` where a host value is read (an array hole, a result) |
// | bool   | `boolean` |
// | number | `number` (see number.ts); a host `bigint` in the safe range is read as a number |
// | string | `string`, always well formed: host text is made well formed where it is read |
// | list   | an `Array`, the run's own or the host's |
// | map    | an `RMap` (a map the run created), a host `Map` with string keys, or a host plain object |
// | opaque | any other object, or one that a declared opaque type's test accepts |
//
// A host plain object is a map of its own enumerable string keys. A property
// that holds `undefined` is absent. Nothing from the prototype chain is read.
// The run never changes a value.

/** A map the run created. Its keys are strings and its values are values. */
export class RMap extends Map<string, unknown> {}

export type Kind = "null" | "bool" | "number" | "string" | "list" | "map" | "opaque";

/** A host number that cannot enter the language (03 3.4.4). */
export const BAD_NUMBER: unique symbol = Symbol("bad number");
/** A key that a map does not hold. */
export const ABSENT: unique symbol = Symbol("absent");

/** Read a host value: what enters the language, or BAD_NUMBER. */
export function readHost(v: unknown): unknown {
  switch (typeof v) {
    case "undefined":
      return null;
    case "number":
      if (!Number.isFinite(v)) return BAD_NUMBER;
      return v + 0;
    case "bigint":
      if (v > 9007199254740991n || v < -9007199254740991n) return BAD_NUMBER;
      return Number(v);
    case "string":
      return v.isWellFormed() ? v : v.toWellFormed();
    default:
      return v;
  }
}

/** Tells the kind of a value, and the declared opaque type of a host object. */
export class Classifier {
  private readonly tests: [string, (v: unknown) => boolean][];
  private readonly cache = new WeakMap<object, string | null>();

  constructor(tests: [string, (v: unknown) => boolean][] = []) {
    this.tests = tests;
  }

  kind(v: unknown): Kind {
    if (v === null || v === undefined) return "null";
    switch (typeof v) {
      case "boolean":
        return "bool";
      case "number":
      case "bigint":
        return "number";
      case "string":
        return "string";
      case "object":
        if (Array.isArray(v)) return "list";
        if (v instanceof RMap) return "map";
        if (this.tests.length > 0 && this.opaqueName(v) !== null) return "opaque";
        if (v instanceof Map) return "map";
        {
          const p = Object.getPrototypeOf(v);
          if (p === null || p === Object.prototype) return "map";
        }
        return "opaque";
      default:
        return "opaque";
    }
  }

  /** The declared opaque type name of a host object, or null. */
  opaqueName(v: unknown): string | null {
    if (typeof v !== "object" || v === null || Array.isArray(v) || v instanceof RMap) return null;
    let n = this.cache.get(v);
    if (n === undefined) {
      n = null;
      for (const [name, test] of this.tests) {
        if (test(v)) {
          n = name;
          break;
        }
      }
      this.cache.set(v, n);
    }
    return n;
  }
}

// ---- maps -------------------------------------------------------------------

const hasOwn = Object.prototype.hasOwnProperty;
const isEnum = Object.prototype.propertyIsEnumerable;

/** The raw value under a key of a map, or ABSENT. */
export function mapGet(m: object, key: string): unknown {
  if (m instanceof Map) {
    const v = m.get(key);
    return v === undefined ? ABSENT : v;
  }
  if (!hasOwn.call(m, key) || !isEnum.call(m, key)) return ABSENT;
  const v = (m as Record<string, unknown>)[key];
  return v === undefined ? ABSENT : v;
}

/** The keys of a map, in no order. */
export function mapKeys(m: object): string[] {
  if (m instanceof RMap) return [...m.keys()];
  if (m instanceof Map) {
    const out: string[] = [];
    for (const [k, v] of m) if (typeof k === "string" && v !== undefined) out.push(k);
    return out;
  }
  const out: string[] = [];
  for (const k of Object.keys(m)) if ((m as Record<string, unknown>)[k] !== undefined) out.push(k);
  return out;
}

export function mapSize(m: object): number {
  return m instanceof RMap ? m.size : mapKeys(m).length;
}

export function mapIsEmpty(m: object): boolean {
  if (m instanceof RMap) return m.size === 0;
  if (m instanceof Map) {
    for (const [k, v] of m) if (typeof k === "string" && v !== undefined) return false;
    return true;
  }
  for (const k in m) {
    if (hasOwn.call(m, k) && (m as Record<string, unknown>)[k] !== undefined) return false;
  }
  return true;
}

// ---- text -------------------------------------------------------------------

/** The weight of a string (02 4): its UTF-16 length. */
export function weight(s: string): number {
  return s.length;
}

/** The size of a string (02 4). */
export function size(s: string): number {
  return Math.floor(s.length / 64);
}

/** The number of code points of a well-formed string. */
export function cpLength(s: string): number {
  let n = s.length;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c >= 0xd800 && c <= 0xdbff) n--;
  }
  return n;
}

/** The one-code-point string at position i, or null. */
export function cpAt(s: string, i: number): string | null {
  if (i < 0) return null;
  let k = 0;
  for (let j = 0; j < s.length; j++) {
    const c = s.charCodeAt(j);
    const pair = c >= 0xd800 && c <= 0xdbff;
    if (k === i) return pair ? s.slice(j, j + 2) : s[j];
    if (pair) j++;
    k++;
  }
  return null;
}

/** The weight of the longest common prefix of two strings, in code points. */
export function lcpWeight(a: string, b: string): number {
  const n = Math.min(a.length, b.length);
  let i = 0;
  while (i < n && a.charCodeAt(i) === b.charCodeAt(i)) i++;
  // Do not count half of a surrogate pair.
  if (i > 0 && i < n) {
    const c = a.charCodeAt(i - 1);
    if (c >= 0xd800 && c <= 0xdbff) i--;
  } else if (i > 0 && i === n && i < Math.max(a.length, b.length)) {
    const c = a.charCodeAt(i - 1);
    if (c >= 0xd800 && c <= 0xdbff) i--;
  }
  return i;
}

/** Order strings by code point (01 6.7 rule 4). */
export function compareText(a: string, b: string): number {
  const n = Math.min(a.length, b.length);
  for (let i = 0; i < n; i++) {
    let x = a.charCodeAt(i);
    let y = b.charCodeAt(i);
    if (x !== y) {
      // Surrogates (D800-DFFF) encode code points above every BMP unit.
      if (x >= 0xd800) x = x >= 0xe000 ? x - 0x800 : x + 0x2000;
      if (y >= 0xd800) y = y >= 0xe000 ? y - 0x800 : y + 0x2000;
      return x - y;
    }
  }
  return a.length - b.length;
}

export function sortKeys(keys: string[]): string[] {
  return keys.sort(compareText);
}

/** The JSON form of a string (01 12). */
export function jsonString(s: string): string {
  let out = '"';
  let from = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    let e: string | null = null;
    if (c === 0x22) e = '\\"';
    else if (c === 0x5c) e = "\\\\";
    else if (c < 0x20) {
      switch (c) {
        case 0x08: e = "\\b"; break;
        case 0x0c: e = "\\f"; break;
        case 0x0a: e = "\\n"; break;
        case 0x0d: e = "\\r"; break;
        case 0x09: e = "\\t"; break;
        default: e = "\\u00" + c.toString(16).padStart(2, "0");
      }
    }
    if (e !== null) {
      out += s.slice(from, i) + e;
      from = i + 1;
    }
  }
  return out + s.slice(from) + '"';
}

// ---- truthiness and plain equality -------------------------------------------

/** The language's truthiness test (01 6.5, 03 3.2.4). It never fails. */
export function truthy(v: unknown, cls: Classifier = DEFAULT_CLASSIFIER): boolean {
  switch (cls.kind(v)) {
    case "null":
      return false;
    case "bool":
      return v as boolean;
    case "number":
      return typeof v === "bigint" ? v !== 0n : v !== 0 && !Number.isNaN(v);
    case "string":
      return (v as string).length > 0;
    case "list":
      return (v as unknown[]).length > 0;
    case "map":
      return !mapIsEmpty(v as object);
    default:
      return true;
  }
}

export const DEFAULT_CLASSIFIER = new Classifier();

/**
 * Equality with no charge (01 6.7), for the runner and for stubs. Host values
 * are read as a run reads them. Gives false past `depth` levels.
 */
export function plainEqual(a: unknown, b: unknown, cls: Classifier = DEFAULT_CLASSIFIER, depth = 1000): boolean {
  a = readHost(a);
  b = readHost(b);
  const ka = cls.kind(a);
  if (ka !== cls.kind(b)) return false;
  switch (ka) {
    case "null":
      return true;
    case "bool":
    case "number":
    case "string":
      return a === b;
    case "opaque":
      return a === b;
    case "list": {
      const x = a as unknown[];
      const y = b as unknown[];
      if (x.length !== y.length) return false;
      if (depth <= 0) return false;
      for (let i = 0; i < x.length; i++) if (!plainEqual(x[i], y[i], cls, depth - 1)) return false;
      return true;
    }
    case "map": {
      const kx = mapKeys(a as object);
      if (kx.length !== mapSize(b as object)) return false;
      if (depth <= 0) return false;
      for (const k of kx) {
        const w = mapGet(b as object, k);
        if (w === ABSENT || !plainEqual(mapGet(a as object, k), w, cls, depth - 1)) return false;
      }
      return true;
    }
  }
}
