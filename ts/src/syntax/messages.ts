// The messages of the parse phase (package F8, f08 3.2 to 3.4), built from
// the catalog that scripts/generate.ts makes from suite/messages-parse.json.

import { CATALOG } from "../generated/messages.ts";

const BY_CODE = new Map(CATALOG.map((e) => [e.code, e]));

/** The value of a `token` parameter for the empty span at the end of input. */
export const END: unique symbol = Symbol("end of input");

/** Visible text (f08 1.1). */
export function visibleText(s: string): string {
  let out = "";
  for (const ch of s) {
    const c = ch.codePointAt(0)!;
    if (c <= 0x1f || (c >= 0x7f && c <= 0x9f)) {
      out += c === 0x09 ? "\\t" : c === 0x0a ? "\\n" : c === 0x0d ? "\\r" : "\\u00" + c.toString(16).padStart(2, "0");
    } else out += ch;
  }
  return out;
}

function cut(s: string): string {
  const cps = Array.from(s);
  return cps.length <= 64 ? s : cps.slice(0, 61).join("") + "...";
}

function jsonQuote(s: string): string {
  let out = '"';
  for (const ch of s) {
    const c = ch.codePointAt(0)!;
    if (ch === '"') out += '\\"';
    else if (ch === "\\") out += "\\\\";
    else if (c < 0x20) {
      out += c === 0x08 ? "\\b" : c === 0x0c ? "\\f" : c === 0x0a ? "\\n" : c === 0x0d ? "\\r" : c === 0x09 ? "\\t" : "\\u00" + c.toString(16).padStart(2, "0");
    } else out += ch;
  }
  return out + '"';
}

function render(kind: string, v: unknown): string {
  switch (kind) {
    case "token":
      return v === END ? "end of input" : visibleText(String(v));
    case "char": {
      const c = v as number;
      const hex = "U+" + c.toString(16).toUpperCase().padStart(4, "0");
      return c >= 0x21 && c <= 0x7e ? `${hex} (${String.fromCodePoint(c)})` : hex;
    }
    case "string":
      return jsonQuote(String(v));
    default:
      return String(v);
  }
}

/** A parse message: the template of a code, each parameter rendered by its kind and cut (f08 3.3). */
export function parseMessage(code: string, template: string, params: Record<string, unknown> = {}): string {
  const e = BY_CODE.get(code);
  const text = e?.templates[template];
  if (e === undefined || text === undefined) return code.replace(/-/g, " ");
  return text.replace(/\{([a-z]+)\}/g, (_, name: string) => cut(render(e.params[name], params[name])));
}
