// Package F16 slice D3: SchemaOf, a type as a JSON Schema (f16 11.5).
//
// SchemaOf reads a type and nothing else: it charges nothing and needs no
// environment (11.5 item 8). Its text is the JSON form of the schema, with
// the keys of every object in code point order and no white space (01 12), so
// two implementations give the same text for one type.

import { isAny, isNever, sortedKeys, variantsPrinted } from "./type.ts";
import type { Alts, Rec, Ref, Type } from "./type.ts";
import { compareText, jsonString } from "../value/value.ts";

/** SchemaOf refuses a type (11.5): the `declare` error `bad-type`, with this message. */
export class SchemaRefused extends Error {}

/** A type variable is held as an opaque name that begins with U+FFFF (f01 5.2). */
const VAR = 0xffff;

type Json = boolean | string | Json[] | { [k: string]: Json };

/** SchemaOf(T) as a JSON value: objects, arrays, strings and booleans. Throws SchemaRefused. */
export function schemaOf(t: Type): Json {
  return new Writer().root(t);
}

/** The text of SchemaOf(T) (11.5): keys in code point order, no white space. Throws SchemaRefused. */
export function schemaText(t: Type): string {
  return jsonText(schemaOf(t));
}

/** The JSON form of a schema value, keys in code point order (01 12). */
export function jsonText(v: Json): string {
  if (typeof v === "boolean") return String(v);
  if (typeof v === "string") return jsonString(v);
  if (Array.isArray(v)) return "[" + v.map(jsonText).join(",") + "]";
  const keys = Object.keys(v).sort(compareText);
  return "{" + keys.map((k) => jsonString(k) + ":" + jsonText(v[k])).join(",") + "}";
}

/** A plain object, for the host. */
function obj(): { [k: string]: Json } {
  return {};
}

/** Set a key that a type names (a field's key, a type's name): `__proto__` among them is an own key. */
function put(o: { [k: string]: Json }, k: string, v: Json): void {
  Object.defineProperty(o, k, { value: v, enumerable: true, writable: true, configurable: true });
}

class Writer {
  /** The recursive types reached, by name (11.5 items 2 and 3). */
  private readonly refs = new Map<string, Ref>();
  private readonly queue: Ref[] = [];

  root(t: Type): Json {
    const s = this.type(t);
    if (this.queue.length === 0) return s;
    const defs = obj();
    // Each recursive type that T reaches, through the definitions of the references it reaches, once.
    for (let i = 0; i < this.queue.length; i++) {
      const r = this.queue[i];
      if (r.def === undefined) throw new SchemaRefused(`the recursive type ${r.name} is not complete`);
      put(defs, r.name, this.type(r.def));
    }
    // A root that reaches a reference is an object: `false` and `{}` reach none.
    const out = s as { [k: string]: Json };
    out["$defs"] = defs;
    return out;
  }

  private ref(r: Ref): Json {
    const known = this.refs.get(r.name);
    if (known === undefined) {
      this.refs.set(r.name, r);
      this.queue.push(r);
    } else if (known !== r) throw new SchemaRefused(`two recursive types are named ${r.name}`);
    const o = obj();
    o["$ref"] = `#/$defs/${r.name}`;
    return o;
  }

  type(t: Type): Json {
    if (isAny(t)) return obj();
    if (isNever(t)) return false;
    const alts = this.alternatives(t);
    if (alts.length === 1) return alts[0];
    const o = obj();
    o.anyOf = alts;
    return o;
  }

  /** The schema of each alternative, in the order of printing (05 3.9; f17 12.6 item 3). */
  private alternatives(t: Alts): Json[] {
    if (t.opaque !== undefined) {
      for (const n of t.opaque) {
        if (n.charCodeAt(0) === VAR) throw new SchemaRefused("a type variable has no JSON Schema");
      }
      if (t.opaque.size > 0) throw new SchemaRefused("an opaque type has no JSON Schema");
    }
    const out: Json[] = [];
    const typed = (name: string) => {
      const o = obj();
      o.type = name;
      return o;
    };
    if (t.bool) out.push(typed("boolean"));
    if (t.num) out.push(typed(t.num === "int" ? "integer" : "number"));
    if (t.str !== undefined) {
      const o = typed("string");
      if (t.str !== "string") {
        const vs = [...t.str].sort(compareText);
        if (vs.length === 1) o.const = vs[0];
        else o.enum = vs;
      }
      out.push(o);
    }
    if (t.list !== undefined) {
      const o = typed("array");
      o.items = this.type(t.list);
      out.push(o);
    }
    if (t.union !== undefined) for (const v of variantsPrinted(t)) out.push(this.record(v));
    else if (t.map !== undefined) out.push(this.record(t.map));
    if (t.refs !== undefined) for (const r of t.refs) out.push(this.ref(r));
    if (t.null) out.push(typed("null"));
    return out;
  }

  private record(r: Rec): Json {
    const o = obj();
    o.type = "object";
    const keys = sortedKeys(r);
    if (keys.length > 0) {
      const props = obj();
      const required: string[] = [];
      for (const k of keys) {
        const f = r.fields.get(k)!;
        let s = this.type(f.type);
        // 11.5 item 1: a field's description beside its other keys; `false` has no keys.
        if (f.desc !== undefined && s !== false) {
          const d = obj();
          for (const [kk, vv] of Object.entries(s as { [k: string]: Json })) put(d, kk, vv);
          d.description = f.desc;
          s = d;
        }
        put(props, k, s);
        if (!f.optional) required.push(k);
      }
      o.properties = props;
      if (required.length > 0) o.required = required;
    }
    if (isNever(r.rest)) o.additionalProperties = false;
    else if (!isAny(r.rest)) o.additionalProperties = this.type(r.rest);
    return o;
  }
}
