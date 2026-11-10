// The top-level tests of a library call's arguments (04 2.6).

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
      // 04 2.6: a type variable with no bound accepts every value, null
      // included, like `any`. A bound is tested apart (ordered, number).
      return "ok";
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

/** The strings of a parameter type that is a literal set and nothing else, or null. */
function literalSet(t: TypeNode): ReadonlySet<string> | null {
  if (t.t === "literal") return new Set([t.value]);
  if (t.t === "union" && t.alts.every((a) => a.t === "literal")) return new Set(t.alts.map((a) => (a as { value: string }).value));
  return null;
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

/** The kinds of an ordered set (04 3.6 rule 4): the index and kind of the first value that fails, or null. */
export function orderedFailure(kinds: string[]): { index: number; kind: string } | null {
  if (kinds.length === 0) return null;
  const first = kinds[0];
  for (let i = 0; i < kinds.length; i++) {
    const k = kinds[i];
    if ((k !== "number" && k !== "string") || k !== first) return { index: i, kind: k };
  }
  return null;
}

/** What the test of a parameter needs, worked out once for each parameter of an entry. */
interface ParamTest {
  /** The type an argument is tested against. */
  t: TypeNode;
  bound: string | null;
  set: ReadonlySet<string> | null;
  /** The type accepts every value. */
  any: boolean;
}

const paramTests = new WeakMap<Param, ParamTest>();

function paramTest(e: Entry, p: Param): ParamTest {
  let pt = paramTests.get(p);
  if (pt === undefined) {
    let t = p.type!;
    if (p.rest && t.t === "list") t = t.elem;
    const bound = boundOf(e, t);
    const set = literalSet(t);
    pt = { t, bound, set, any: bound === null && set === null && (t.t === "var" || (t.t === "word" && t.word === "any")) };
    paramTests.set(p, pt);
  }
  return pt;
}

/** Test each argument against its parameter, left to right. Charges nothing. */
export function testArgs(run: Run, e: Entry, args: unknown[], span: Span, name: string): void {
  for (let i = 0; i < args.length; i++) {
    const p = paramAt(e, i);
    if (p.fn !== null || p.type === null) continue;
    const pt = paramTest(e, p);
    if (pt.any) continue;
    const v = args[i];
    const t = pt.t;
    const verdict = accepts(t, v, run);
    const bound = pt.bound;
    if (bound === "ordered") {
      // Every failure of kind at an `ordered` parameter is `unordered` (04 2.6):
      // the arguments bound to the variable are one ordered set, tested left to right.
      const set = args.slice(0, i + 1).filter((_, j) => {
        const q = paramAt(e, j);
        const qt = q.rest && q.type !== null && q.type.t === "list" ? q.type.elem : q.type;
        return qt !== null && qt.t === "var" && qt.name === (t as { name: string }).name;
      });
      const f = orderedFailure(set.map((x) => run.kind(x)));
      if (f !== null) throw new RunFailure("wrong-kind", span, "unordered", { name, kind: f.kind });
      continue;
    }
    if (bound === "number") {
      if (run.kind(v) !== "number") throw new RunFailure("wrong-kind", span, "argument", { name, kind: run.kind(v) });
      continue;
    }
    // f07 6.2: a parameter whose type is a literal set takes a string of the set, else `order`.
    const set = pt.set;
    if (set !== null) {
      if (run.kind(v) === "string" && set.has(v as string)) continue;
      throw new RunFailure("wrong-kind", span, "order", { name, found: v });
    }
    if (verdict === "ok") continue;
    if (verdict === "whole") throw new RunFailure("wrong-kind", span, "whole", { found: v });
    throw new RunFailure("wrong-kind", span, "argument", { name, kind: run.kind(v) });
  }
}
