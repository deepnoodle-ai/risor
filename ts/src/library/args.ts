// The top-level tests of a library call's arguments (04 2.6, rulings AC5, AC6).

import { RunFailure } from "../eval/failure.ts";
import type { Run } from "../eval/run.ts";
import type { Span } from "../syntax/source.ts";
import type { TypeNode } from "../types/typetext.ts";
import { isWhole } from "../value/number.ts";
import type { Entry, Param } from "./registry.ts";

type Verdict = "ok" | "kind" | "whole";

function accepts(t: TypeNode, v: unknown, run: Run): Verdict {
  const k = run.kind(v);
  switch (t.t) {
    case "word":
      switch (t.word) {
        case "any":
          return "ok";
        case "null":
          return k === "null" ? "ok" : "kind";
        case "bool":
          return k === "bool" ? "ok" : "kind";
        case "int":
          return k !== "number" ? "kind" : isWhole(v as number) ? "ok" : "whole";
        case "float":
          return k === "number" ? "ok" : "kind";
        case "string":
          return k === "string" ? "ok" : "kind";
        case "never":
          return "kind";
      }
      return "kind";
    case "literal":
      return k === "string" ? "ok" : "kind";
    case "list":
      return k === "list" ? "ok" : "kind";
    case "map":
    case "record":
      return k === "map" ? "ok" : "kind";
    case "name":
      return k === "opaque" ? "ok" : "kind";
    case "var":
      return k === "null" ? "kind" : "ok";
    case "union": {
      let best: Verdict = "kind";
      for (const a of t.alts) {
        const r = accepts(a, v, run);
        if (r === "ok") return "ok";
        if (r === "whole") best = "whole";
      }
      return best;
    }
  }
}

export function paramAt(e: Entry, i: number): Param {
  return i < e.params.length ? e.params[i] : e.params[e.params.length - 1];
}

/** Bounds of the signature's type variables: `ordered` or `number`. */
function boundOf(e: Entry, t: TypeNode | null): string | null {
  if (t === null || t.t !== "var") return null;
  const bounds = (e.record.signature.match(/where (.*)$/)?.[1] ?? "").split(",").map((b) => b.trim());
  for (const b of bounds) {
    const [v, bound] = b.split(":").map((x) => x.trim());
    if (v === t.name) return bound;
  }
  return null;
}

/** Test each argument against its parameter, left to right. Charges nothing. */
export function testArgs(run: Run, e: Entry, args: unknown[], span: Span, name: string): void {
  for (let i = 0; i < args.length; i++) {
    const p = paramAt(e, i);
    if (p.fn !== null || p.type === null) continue;
    const v = args[i];
    let t = p.type;
    if (p.rest && t.t === "list") t = t.elem;
    const verdict = accepts(t, v, run);
    if (verdict === "ok") {
      if (boundOf(e, t) !== "ordered") continue;
      const k = run.kind(v);
      if (k === "number" || k === "string") continue;
      throw new RunFailure("wrong-kind", span, "unordered", { name, kind: k });
    }
    if (boundOf(e, t) === "ordered") throw new RunFailure("wrong-kind", span, "unordered", { name, kind: run.kind(v) });
    if (verdict === "whole") throw new RunFailure("wrong-kind", span, "whole", { found: v });
    throw new RunFailure("wrong-kind", span, "argument", { name, kind: run.kind(v) });
  }
}
