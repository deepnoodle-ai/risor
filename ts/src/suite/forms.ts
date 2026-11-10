// The JSON form of package F9 (f09 1.1): no white space, keys in code point
// order, numbers in the rendering of 01 6.3, an opaque value written as a case
// file writes it. Used by the report (f09 6) and by update mode (f09 4.5).

import { numberToText } from "../value/number.ts";
import { BAD_NUMBER, Classifier, compareText, jsonString, mapGet, mapKeys, readHost } from "../value/value.ts";
import type { Json } from "./json.ts";

/** An opaque value of a case file: its type and id (03 3.10.3 item 7). */
export interface OpaqueLike {
  readonly type: string;
  readonly id: number;
}

/** The JSON form of case-file data. */
export function jsonForm(v: Json): string {
  if (v === null) return "null";
  if (typeof v === "boolean") return String(v);
  if (typeof v === "number") return numberToText(v);
  if (typeof v === "string") return jsonString(v);
  if (Array.isArray(v)) return "[" + v.map(jsonForm).join(",") + "]";
  const keys = Object.keys(v).sort(compareText);
  return "{" + keys.map((k) => jsonString(k) + ":" + jsonForm(v[k])).join(",") + "}";
}

/** The JSON form of a value of the language. `opaqueOf` tells an opaque value of a case file. */
export function valueForm(v: unknown, cls: Classifier, opaqueOf: (x: unknown) => OpaqueLike | null): string {
  v = readHost(v);
  if (v === BAD_NUMBER) return "null";
  const o = opaqueOf(v);
  if (o !== null) return `{"$opaque":{"id":${numberToText(o.id)},"type":${jsonString(o.type)}}}`;
  switch (cls.kind(v)) {
    case "null":
      return "null";
    case "bool":
      return String(v);
    case "number":
      return numberToText(v as number);
    case "string":
      return jsonString(v as string);
    case "list":
      return "[" + (v as unknown[]).map((x) => valueForm(x, cls, opaqueOf)).join(",") + "]";
    case "map": {
      const keys = mapKeys(v as object).sort(compareText);
      return "{" + keys.map((k) => jsonString(k) + ":" + valueForm(mapGet(v as object, k), cls, opaqueOf)).join(",") + "}";
    }
    default:
      return "null";
  }
}

/** A value of the language as case-file data, or null when it holds a map keyed `$opaque` (f09 4.3). */
export function valueData(v: unknown, cls: Classifier, opaqueOf: (x: unknown) => OpaqueLike | null): { ok: true; data: Json } | { ok: false } {
  let bad = false;
  const walk = (x: unknown): Json => {
    x = readHost(x);
    if (x === BAD_NUMBER) return null;
    const o = opaqueOf(x);
    if (o !== null) return { $opaque: { type: o.type, id: o.id } };
    switch (cls.kind(x)) {
      case "null":
        return null;
      case "bool":
        return x as boolean;
      case "number":
        return x as number;
      case "string":
        return x as string;
      case "list":
        return (x as unknown[]).map(walk);
      case "map": {
        const out: { [k: string]: Json } = Object.create(null);
        for (const k of mapKeys(x as object)) {
          if (k === "$opaque") bad = true;
          out[k] = walk(mapGet(x as object, k));
        }
        return out;
      }
      default:
        bad = true;
        return null;
    }
  };
  const data = walk(v);
  return bad ? { ok: false } : { ok: true, data };
}
