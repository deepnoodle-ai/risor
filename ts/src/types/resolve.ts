// Resolving a program type (package F4, f04 3.5 and 6.1): names, records,
// then depth and size. Used in every compile, the static rules alone included.

import type { Span } from "../syntax/source.ts";
import type { TypeNode } from "./typetext.ts";
import { depth, findTag, fromNode, isAny, SIZE_LIMIT, typeSize, variantsOf } from "./type.ts";
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
  // 2. Records (f05 3.2): each Type from the innermost out; one that holds an error is not tested again.
  const ctx = { opaque: names.opaque, named: names.named };
  const unions: { code: string; span: Span }[] = [];
  const visit = (n: TypeNode): boolean => {
    switch (n.t) {
      case "list": return visit(n.elem);
      case "map": return visit(n.rest);
      case "record": {
        let ok = true;
        for (const f of n.fields) ok = visit(f.type) && ok;
        if (typeof n.rest !== "string") ok = visit(n.rest) && ok;
        return ok;
      }
      case "union": {
        let ok = true;
        for (const a of n.alts) ok = visit(a) && ok;
        if (!ok) return false;
        const recs = n.alts.flatMap((a) => {
          const x = fromNode(a, ctx);
          return isAny(x) ? [] : variantsOf(x);
        });
        if (recs.length >= 2 && findTag(recs) === null) {
          unions.push({ code: "untagged-union", span: spans.get(n)! });
          return false;
        }
        return true;
      }
    }
    return true;
  };
  visit(node);
  if (unions.length > 0) return { ok: false, diagnostics: unions };
  // 3. Depth and size: one check-limit for both (3.5 steps 5 and 6).
  const t = fromNode(node, ctx);
  if (depth(t, 70) > 64 || typeSize(t) > SIZE_LIMIT) return { ok: false, diagnostics: [{ code: "check-limit", span: typeSpan }] };
  return { ok: true, type: t };
}
