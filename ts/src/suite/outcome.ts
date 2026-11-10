// The outcome of a case as one line of JSON (ruling AJ, round 10): what the
// differential fuzzer compares between the three implementations.

import { numberToText } from "../value/number.ts";
import { jsonString, mapGet, mapKeys, readHost, sortKeys, BAD_NUMBER } from "../value/value.ts";
import type { Case } from "./casefile.ts";
import { CLASSIFIER, OpaqueRef, executeCase } from "./runner.ts";

/** A result as JSON (03 3.2.4), an opaque value as a case file writes it. */
export function valueJSON(v: unknown, depth = 0): string {
  if (depth > 1100) return "null";
  v = readHost(v);
  if (v === BAD_NUMBER) return "null";
  if (v instanceof OpaqueRef) return `{"$opaque":{"type":${jsonString(v.type)},"id":${v.id}}}`;
  switch (CLASSIFIER.kind(v)) {
    case "null":
      return "null";
    case "bool":
      return String(v);
    case "number":
      return numberToText(Number(v));
    case "string":
      return jsonString(v as string);
    case "list":
      return "[" + (v as unknown[]).map((x) => valueJSON(x, depth + 1)).join(",") + "]";
    case "map":
      return "{" + sortKeys(mapKeys(v as object)).map((k) => jsonString(k) + ":" + valueJSON(mapGet(v as object, k), depth + 1)).join(",") + "}";
    default:
      return '{"$opaque":null}';
  }
}

/** The line of ruling AJ item 3 for one case. Every `expect` is ignored. */
export function outcomeLine(c: Case): string {
  const e = executeCase(c, true);
  const o = e.outcome;
  const parts: string[] = [`"name":${jsonString(c.name)}`];
  if (o.kind === "result") {
    parts.push(`"phase":null`);
    if (o.ran) parts.push(`"value":${valueJSON(o.value)}`);
  } else {
    parts.push(`"phase":${jsonString(o.phase)}`);
    const span = (d: { span?: { start: number; end: number } }) => (d.span !== undefined && o.source !== undefined ? JSON.stringify(o.source.lineSpan(d.span)) : "null");
    let diags: string[];
    switch (o.phase) {
      case "parse":
        diags = [o.diagnostics[0]].map((d) => `{"code":${jsonString(d.code)},"span":${span(d)}}`);
        break;
      case "declare":
        diags = o.diagnostics.map((d) => d.code).sort().map((code) => `{"code":${jsonString(code)}}`);
        break;
      case "check":
        diags = o.diagnostics.map((d) => `{"code":${jsonString(d.code)},"span":${span(d)}}`);
        break;
      default:
        diags = o.diagnostics.map(
          (d) => `{"code":${jsonString(d.code)},"span":${span(d)},"class":${d.class === undefined ? "null" : jsonString(d.class)},"message":${jsonString(d.message)}}`,
        );
    }
    parts.push(`"diagnostics":[${diags.join(",")}]`);
  }
  // Ruling AK3: a failure of class `limit` has no usage in its line.
  const limit = o.kind === "failure" && o.diagnostics.some((d) => d.class === "limit");
  if (o.usage !== undefined && !limit) parts.push(`"steps":${o.usage.steps}`, `"units":${o.usage.allocated}`);
  return `{${parts.join(",")}}`;
}
