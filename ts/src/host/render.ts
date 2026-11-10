// The three renderings of diagnostics (package F8, f08 5): one line, detail
// and JSON. Each is the same text, code point for code point, in every
// implementation.

import { visible } from "../eval/failure.ts";
import type { LineSpan } from "../syntax/source.ts";
import { numberToText } from "../value/number.ts";
import { BAD_NUMBER, Classifier, compareText, jsonString, mapGet, mapKeys, readHost } from "../value/value.ts";

export interface RenderTrace {
  span: LineSpan;
  name: string;
  index: number | null;
}

/** A diagnostic as the renderings read it: spans as lines and columns. */
export interface RenderDiagnostic {
  phase: string;
  code: string;
  message: string;
  class?: string;
  span?: LineSpan;
  path?: (string | number)[];
  data?: unknown;
  trace?: RenderTrace[];
}

const WORD = /^[A-Za-z_][A-Za-z0-9_]*$/;

/** The first 61 code points and `...` when longer than 64 (03 3.8.3 item 1). */
function cut(s: string): string {
  const cps = Array.from(s);
  return cps.length <= 64 ? s : cps.slice(0, 61).join("") + "...";
}

/** A path as f08 5.1 item 5 writes it: `.word`, `["key"]`, `[3]`. */
export function pathText(path: (string | number)[]): string {
  let out = "";
  for (const p of path) {
    if (typeof p === "number") out += `[${p}]`;
    else if (WORD.test(p)) out += "." + cut(p);
    else out += `[${cut(jsonString(p))}]`;
  }
  return out;
}

const spanText = (s: LineSpan) => `${s[0]}:${s[1]}-${s[2]}:${s[3]}`;

function oneLine(d: RenderDiagnostic): string {
  let out = `error[${d.code}]`;
  if (d.span !== undefined) out += " " + spanText(d.span);
  out += ": " + visible(d.message);
  if (d.path !== undefined && d.path.length > 0) out += " at " + pathText(d.path);
  return out;
}

/** The one-line form (f08 5.2). */
export function renderLine(ds: RenderDiagnostic[]): string {
  return ds.map((d) => oneLine(d) + "\n").join("");
}

/** The excerpt and the caret line (f08 5.4). */
function excerpt(source: string, span: LineSpan): [string, string] {
  const lines = source.replace(/^﻿/, "").split("\n");
  const line = Array.from(lines[span[0] - 1] ?? "");
  const n = line.length;
  const a = span[1];
  let b = span[2] === span[0] ? span[3] : n + 1;
  if (b <= a) b = a + 1;
  const col = (c: string) => {
    const x = c.codePointAt(0)!;
    return x <= 0x1f || (x >= 0x7f && x <= 0x9f) ? " " : c;
  };
  let s = 1;
  let text: string;
  let lead = 0;
  if (n <= 120) text = line.map(col).join("");
  else {
    s = Math.min(Math.max(a - 40, 1), n - 119);
    text = (s > 1 ? "..." : "") + line.slice(s - 1, s + 119).map(col).join("") + (s + 120 <= n ? "..." : "");
    if (s > 1) lead = 3;
  }
  const carets = Math.max(1, Math.min(b, s + 120) - a);
  return [text, " ".repeat(lead + a - s) + "^".repeat(carets)];
}

/** The detail form (f08 5.3). */
export function renderDetail(ds: RenderDiagnostic[], source: string | null): string {
  let out = "";
  for (const d of ds) {
    out += oneLine(d) + "\n";
    if (d.span !== undefined && source !== null) {
      const [text, carets] = excerpt(source, d.span);
      out += "    " + text + "\n" + "    " + carets + "\n";
    }
    if (d.phase === "run" && d.trace !== undefined) {
      for (let i = d.trace.length - 1; i >= 0; i--) {
        const t = d.trace[i];
        out += `  in ${t.name}${t.index === null ? "" : `, element ${t.index}`}, at ${spanText(t.span)}\n`;
      }
    }
  }
  return out;
}

/** `data` in the JSON form (f08 5.6): opaque values as null; null when deeper than 200 or longer than 10,000 code points. */
function dataJSON(v: unknown, cls: Classifier): string {
  const LIMIT = 10000;
  let out = "";
  let over = false;
  const walk = (x: unknown, depth: number): void => {
    if (over) return;
    x = readHost(x);
    if (x === BAD_NUMBER) {
      out += "null";
      return;
    }
    switch (cls.kind(x)) {
      case "null":
        out += "null";
        break;
      case "bool":
        out += String(x);
        break;
      case "number":
        out += numberToText(x as number);
        break;
      case "string":
        out += jsonString(x as string);
        break;
      case "list": {
        if (depth + 1 > 200) {
          over = true;
          return;
        }
        out += "[";
        const xs = x as unknown[];
        for (let i = 0; i < xs.length && !over; i++) {
          if (i > 0) out += ",";
          walk(xs[i], depth + 1);
          if (out.length > LIMIT * 2) over = Array.from(out).length > LIMIT;
        }
        out += "]";
        break;
      }
      case "map": {
        if (depth + 1 > 200) {
          over = true;
          return;
        }
        out += "{";
        const keys = mapKeys(x as object).sort(compareText);
        for (let i = 0; i < keys.length && !over; i++) {
          if (i > 0) out += ",";
          out += jsonString(keys[i]) + ":";
          walk(mapGet(x as object, keys[i]), depth + 1);
          if (out.length > LIMIT * 2) over = Array.from(out).length > LIMIT;
        }
        out += "}";
        break;
      }
      default:
        out += "null";
    }
    if (!over && out.length > LIMIT && Array.from(out).length > LIMIT) over = true;
  };
  walk(v, 0);
  return over ? "null" : out;
}

/** The JSON form (f08 5.5): keys in code point order. */
export function renderJSON(ds: RenderDiagnostic[], cls: Classifier = new Classifier()): string {
  return ds
    .map((d) => {
      const parts: string[] = [];
      if (d.class !== undefined) parts.push(`"class":${jsonString(d.class)}`);
      parts.push(`"code":${jsonString(d.code)}`);
      if (d.code === "failed" && d.phase === "run") parts.push(`"data":${dataJSON(d.data ?? null, cls)}`);
      parts.push(`"message":${jsonString(d.message)}`);
      if (d.path !== undefined) parts.push(`"path":[${d.path.map((p) => (typeof p === "number" ? numberToText(p) : jsonString(p))).join(",")}]`);
      parts.push(`"phase":${jsonString(d.phase)}`);
      if (d.span !== undefined) parts.push(`"span":[${d.span.join(",")}]`);
      if (d.phase === "run") {
        const tr = (d.trace ?? []).map((t) => `{"index":${t.index === null ? "null" : t.index},"name":${jsonString(t.name)},"span":[${t.span.join(",")}]}`);
        parts.push(`"trace":[${tr.join(",")}]`);
      }
      return `{${parts.join(",")}}\n`;
    })
    .join("");
}
