// Resolving a program type (package F4, f04 3.5 and 6.1): names, records,
// then depth and size. Used in every compile, the static rules alone included.

import type { Span } from "../syntax/source.ts";
import type { TypeNode } from "./typetext.ts";
import { depth, findTag, fromNode, isAny, PairLimit, SIZE_LIMIT, typeSize, TypeError as RuleError, unionParts, variantsOf } from "./type.ts";
import type { FromContext, Type } from "./type.ts";

/** The type names of an environment: opaque types and named types (f04 3.3). */
export interface TypeNames {
  opaque: ReadonlySet<string>;
  named: ReadonlyMap<string, Type>;
}

/** A diagnostic of type text, with its template (f08 3.5). */
export interface TypeDiag {
  code: string;
  span: Span;
  template: string;
  params: Record<string, unknown>;
}

export type Resolved = { ok: true; type: Type } | { ok: false; diagnostics: TypeDiag[] };

/** Each name of type text for which `defined` is false: `undefined-name`, in source order (f04 6.1 step 1). */
export function undefinedNames(node: TypeNode, spans: ReadonlyMap<TypeNode, Span>, defined: (name: string) => boolean): TypeDiag[] {
  const out: TypeDiag[] = [];
  const visit = (n: TypeNode): void => {
    switch (n.t) {
      case "name":
        if (!defined(n.name)) out.push({ code: "undefined-name", span: spans.get(n)!, template: "type", params: { name: n.name } });
        return;
      case "list": return visit(n.elem);
      case "map": return visit(n.rest);
      case "record":
        for (const f of n.fields) visit(f.type);
        if (typeof n.rest !== "string") visit(n.rest);
        return;
      case "union":
        for (const a of n.alts) visit(a);
        return;
    }
  };
  visit(node);
  return out;
}

/** Resolve a program type; the diagnostics of f04 6.1, in source order. */
export function resolveProgramType(node: TypeNode, typeSpan: Span, spans: ReadonlyMap<TypeNode, Span>, names: TypeNames): Resolved {
  // 1. Names.
  const undef = undefinedNames(node, spans, (n) => names.opaque.has(n) || names.named.has(n));
  if (undef.length > 0) return { ok: false, diagnostics: undef };
  return resolveText(node, typeSpan, spans, { opaque: names.opaque, named: names.named });
}

/**
 * Steps 2 and 3 of f04 6.1 for type text whose names all resolve in `ctx`:
 * records, then depth and size. A computation past the bound of f17 12.5
 * item 7 is `check-limit`, template `pairs`, on the program type.
 */
export function resolveText(node: TypeNode, typeSpan: Span, spans: ReadonlyMap<TypeNode, Span>, ctx: FromContext): Resolved {
  try {
    // 2. Records (f05 3.2): each Type from the innermost out; one that holds an error is not tested again.
    const unions: TypeDiag[] = [];
    // `guarded`: inside a list's element, a field, a rest or a map's value (f17 12.1).
    const visit = (n: TypeNode, guarded: boolean): boolean => {
      switch (n.t) {
        case "list": return visit(n.elem, true);
        case "map": return visit(n.rest, true);
        case "record": {
          let ok = true;
          for (const f of n.fields) ok = visit(f.type, true) && ok;
          if (typeof n.rest !== "string") ok = visit(n.rest, true) && ok;
          return ok;
        }
        case "union": {
          let ok = true;
          for (const a of n.alts) ok = visit(a, guarded) && ok;
          if (!ok) return false;
          // f17 12.2 item 5: a complete reference that shares a kind counts as its definition.
          const recs = unionParts(n.alts.map((a) => fromNode(a, ctx, guarded))).flatMap((x) => (isAny(x) ? [] : variantsOf(x)));
          if (recs.length >= 2 && findTag(recs) === null) {
            unions.push({ code: "untagged-union", span: spans.get(n)!, template: "default", params: {} });
            return false;
          }
          return true;
        }
      }
      return true;
    };
    visit(node, false);
    if (unions.length > 0) return { ok: false, diagnostics: unions };
    // 3. Depth and size: one check-limit for both (3.5 steps 5 and 6). A reference has depth 0 and size 1 (f17 12.6).
    const t = fromNode(node, ctx);
    if (depth(t, 70) > 64) return { ok: false, diagnostics: [{ code: "check-limit", span: typeSpan, template: "depth", params: {} }] };
    if (typeSize(t) > SIZE_LIMIT) return { ok: false, diagnostics: [{ code: "check-limit", span: typeSpan, template: "size", params: {} }] };
    return { ok: true, type: t };
  } catch (e) {
    if (e instanceof PairLimit) return { ok: false, diagnostics: [{ code: "check-limit", span: typeSpan, template: "pairs", params: {} }] };
    // A rule beyond the grammar (05 19.5) that only declared text meets: a key written twice.
    if (e instanceof RuleError) return { ok: false, diagnostics: [{ code: "bad-type", span: typeSpan, template: "default", params: { message: e.message } }] };
    throw e;
  }
}
