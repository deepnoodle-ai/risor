// A description from sample data: each key of the sample is a variable, typed
// from its value. It is built by hand, not with createEnv, so a page that
// calls it on the main thread does not load the checker. A test compares it
// with createEnv(...).describe().

import type { Description } from "risor";

const RESERVED = new Set(
  "true false null const if else in not try catch match as is nil let var for while do break continue return function throw finally import".split(" "),
);
const WORD = /^[A-Za-z_][A-Za-z0-9_]*$/;

/** Whether a key may be written bare in a record type or used as a variable name. */
export function isName(k: string): boolean {
  return WORD.test(k) && !RESERVED.has(k) && k !== "_";
}

/** A type built from JSON values: at most one alternative of each kind (05 2.1). */
interface JType {
  null?: true;
  bool?: true;
  number?: "int" | "float";
  string?: true;
  list?: JType | "empty";
  record?: { fields: Map<string, JType>; optional: Set<string> };
}

function fromJSON(v: unknown): JType {
  if (v === null || v === undefined) return { null: true };
  switch (typeof v) {
    case "boolean":
      return { bool: true };
    case "number":
      return { number: Number.isInteger(v) && Math.abs(v) <= Number.MAX_SAFE_INTEGER ? "int" : "float" };
    case "bigint":
      return { number: "int" };
    case "string":
      return { string: true };
  }
  if (Array.isArray(v)) {
    if (v.length === 0) return { list: "empty" };
    return { list: v.map(fromJSON).reduce(join) };
  }
  const fields = new Map<string, JType>();
  for (const [k, x] of Object.entries(v as Record<string, unknown>)) fields.set(k, fromJSON(x));
  return { record: { fields, optional: new Set() } };
}

/**
 * Join two types. Core has no union of records (05 3.4), so two records merge
 * into one: a field that some values lack becomes optional.
 */
function join(a: JType, b: JType): JType {
  const t: JType = {};
  if (a.null || b.null) t.null = true;
  if (a.bool || b.bool) t.bool = true;
  if (a.string || b.string) t.string = true;
  if (a.number || b.number) t.number = a.number === "float" || b.number === "float" ? "float" : "int";
  if (a.list && b.list) {
    t.list = a.list === "empty" ? b.list : b.list === "empty" ? a.list : join(a.list, b.list);
  } else if (a.list || b.list) t.list = a.list ?? b.list;
  if (a.record && b.record) {
    const fields = new Map<string, JType>();
    const optional = new Set([...a.record.optional, ...b.record.optional]);
    for (const [k, x] of a.record.fields) {
      const y = b.record.fields.get(k);
      fields.set(k, y ? join(x, y) : x);
      if (!y) optional.add(k);
    }
    for (const [k, y] of b.record.fields) {
      if (!a.record.fields.has(k)) {
        fields.set(k, y);
        optional.add(k);
      }
    }
    t.record = { fields, optional };
  } else if (a.record || b.record) t.record = a.record ?? b.record;
  return t;
}

function print(t: JType): string {
  const alts: string[] = [];
  if (t.bool) alts.push("bool");
  if (t.number) alts.push(t.number);
  if (t.string) alts.push("string");
  if (t.list) {
    const e = t.list === "empty" ? "any" : print(t.list);
    alts.push(`${e.includes("|") ? `(${e})` : e}[]`);
  }
  if (t.record) {
    // Fields in code point order, as Risor prints a record type.
    const fs = [...t.record.fields].sort((x, y) => compareCodePoints(x[0], y[0])).map(([k, x]) => {
      const name = isName(k) ? k : JSON.stringify(k);
      return `${name}${t.record!.optional.has(k) ? "?" : ""}: ${print(x)}`;
    });
    alts.push(`{${fs.join(", ")}}`);
  }
  if (t.null) alts.push("null");
  return alts.length ? alts.join(" | ") : "never";
}

/**
 * The type text of a JSON value: whole numbers are `int`, other numbers
 * `float`, objects exact records, and arrays lists of the join of their
 * elements. Records in one list merge, and a field some of them lack is
 * optional. An empty array is `any[]`.
 */
export function typeOfJSON(v: unknown): string {
  return print(fromJSON(v));
}

/**
 * The description of an environment whose variables are the keys of a
 * sample, each typed from its value with `typeOfJSON`. Throws a RangeError
 * for a key that cannot be a variable name.
 */
export function describeSample(sample: Record<string, unknown>, options: { groups?: string[] } = {}): Description {
  const variables: Record<string, string> = {};
  for (const k of Object.keys(sample).sort(compareCodePoints)) {
    if (!isName(k)) throw new RangeError(`"${k}" cannot be a variable name: a name is a word that is not reserved`);
    variables[k] = typeOfJSON(sample[k]);
  }
  const d: Description = {
    version: "3.0",
    variables,
    functions: {},
    opaque: {},
    removed: [],
    reject: ["constant-condition"],
    limits: { steps: 1_000_000, allocation: 1_000_000, depth: 200 },
  };
  if (options.groups && options.groups.length > 0) d.groups = [...new Set(options.groups)].sort(compareCodePoints);
  return d;
}

function compareCodePoints(a: string, b: string): number {
  const x = Array.from(a);
  const y = Array.from(b);
  for (let i = 0; i < Math.min(x.length, y.length); i++) {
    const d = x[i].codePointAt(0)! - y[i].codePointAt(0)!;
    if (d !== 0) return d;
  }
  return x.length - y.length;
}
