// Resolving a program type (package F4, f04 3.5 and 6.1): names, records,
// then depth and size. Used in every compile, the static rules alone included.

import type { Span } from "../syntax/source.ts";
import type { TypeNode } from "./typetext.ts";
import { depth, fromNode, isAny, SIZE_LIMIT, typeSize } from "./type.ts";
import type { Type } from "./type.ts";

/** The type names of an environment: opaque types and named types (f04 3.3). */
export interface TypeNames {
  opaque: ReadonlySet<string>;
  named: ReadonlyMap<string, Type>;
}

export type Resolved = { ok: true; type: Type } | { ok: false; diagnostics: { code: string; span: Span }[] };

/** Resolve a program type; the diagnostics of f04 6.1, in source order. */
export function resolveProgramType(node: TypeNode, typeSpan: Span, spans: ReadonlyMap<TypeNode, Span>, names: TypeNames): Resolved {
  // 1. Names.
  const undef: { code: string; span: Span }[] = [];
  const visitNames = (n: TypeNode): void => {
    switch (n.t) {
      case "name":
        if (!names.opaque.has(n.name) && !names.named.has(n.name)) undef.push({ code: "undefined-name", span: spans.get(n)! });
        return;
      case "list": return visitNames(n.elem);
      case "map": return visitNames(n.rest);
      case "record":
        for (const f of n.fields) visitNames(f.type);
        if (typeof n.rest !== "string") visitNames(n.rest);
        return;
      case "union":
        for (const a of n.alts) visitNames(a);
        return;
    }
  };
  visitNames(node);
  if (undef.length > 0) return { ok: false, diagnostics: undef };
  // 2. Records: the innermost PType whose alternatives, parentheses removed, hold two (3.5 step 4).
  const recs = (n: TypeNode): number => {
    switch (n.t) {
      case "record": case "map": return 1;
      case "union": return n.alts.reduce((k, a) => k + recs(a), 0);
      case "name": {
        const t = names.named.get(n.name);
        return t !== undefined && !isAny(t) && t.map ? 1 : 0;
      }
    }
    return 0;
  };
  const unions: { code: string; span: Span }[] = [];
  const visitRecs = (n: TypeNode): void => {
    switch (n.t) {
      case "list": return visitRecs(n.elem);
      case "map": return visitRecs(n.rest);
      case "record":
        for (const f of n.fields) visitRecs(f.type);
        if (typeof n.rest !== "string") visitRecs(n.rest);
        return;
      case "union":
        if (recs(n) >= 2 && !n.alts.some((a) => a.t === "union" && recs(a) >= 2)) unions.push({ code: "untagged-union", span: spans.get(n)! });
        for (const a of n.alts) visitRecs(a);
        return;
    }
  };
  visitRecs(node);
  if (unions.length > 0) return { ok: false, diagnostics: unions };
  // 3. Depth and size: one check-limit for both (3.5 steps 5 and 6).
  const t = fromNode(node, { opaque: names.opaque, named: names.named });
  if (depth(t, 70) > 64 || typeSize(t) > SIZE_LIMIT) return { ok: false, diagnostics: [{ code: "check-limit", span: typeSpan }] };
  return { ok: true, type: t };
}
