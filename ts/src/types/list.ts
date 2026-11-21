// Resolving a list of named types together (package F17, f17 12.2): the type
// lines of a program, or the named types that one environment declares. A
// name may stand for any item of its list; the items on a cycle of uses are
// recursive, and each gets a reference. The components are resolved one at a
// time, each after those it names. Every walk here is a loop with its own
// stack, so a list of thousands of items needs no deep native stack.

import type { Span } from "../syntax/source.ts";
import type { TypeNode } from "./typetext.ts";
import { isAny, kindsOf, newRef, refType, sortedKeys, unfold, variantsPrinted, withoutRefs } from "./type.ts";
import type { FromContext, Ref, Type } from "./type.ts";
import { resolveText } from "./resolve.ts";
import type { TypeDiag } from "./resolve.ts";

export interface ListItem {
  name: string;
  /** The type text; none for an item whose type is given whole. */
  node?: TypeNode;
  /** The spans of the text's nodes, for diagnostics; none in an environment. */
  spans?: ReadonlyMap<TypeNode, Span>;
  typeSpan?: Span;
  nameSpan?: Span;
  /** A type given whole: a named type by schema, or one a host built. It names no item. */
  fixed?: Type;
}

export interface ListResult {
  /** What each item declares (f17 12.2 item 8): a recursive item its reference; undefined when it fails. */
  types: (Type | undefined)[];
  /** The diagnostics of each item. An item that fails because another of its component failed has none. */
  diagnostics: TypeDiag[][];
}

/** A use (f17 12.1): a name in an item's text that names an item, or the outside, or nothing. */
interface Use {
  node: TypeNode;
  name: string;
  /** The item named, -1 for a type outside the list, null for nothing. */
  j: number | null;
  guarded: boolean;
}

const NO_SPAN: Span = { start: 0, end: 0 };
const NO_SPANS: ReadonlyMap<TypeNode, Span> = new Map();

/**
 * Resolve a list (f17 12.2). `lookup(name, i)` gives the item a name in item
 * i's text names (12.2 item 1): its index, -1 for a type of `outer`, or null
 * for a name that names nothing.
 */
export function resolveList(items: readonly ListItem[], lookup: (name: string, at: number) => number | null, outer: { opaque: ReadonlySet<string>; named: ReadonlyMap<string, Type> }): ListResult {
  const n = items.length;
  const uses: Use[][] = items.map((it, i) => (it.node === undefined ? [] : usesOf(it.node, (name) => lookup(name, i))));
  const edges = uses.map((us) => us.filter((u) => u.j !== null && u.j >= 0).map((u) => u.j!));
  const diagnostics: TypeDiag[][] = items.map(() => []);
  const types: (Type | undefined)[] = new Array(n).fill(undefined);
  const failed: boolean[] = new Array(n).fill(false);
  const recursive: boolean[] = new Array(n).fill(false);
  const refs: (Ref | undefined)[] = new Array(n).fill(undefined);
  const spanOf = (i: number, node: TypeNode) => items[i].spans?.get(node) ?? NO_SPAN;
  const nameSpan = (i: number) => items[i].nameSpan ?? NO_SPAN;

  /** Resolve item i's text. `inC` and `done`: its component, and those of it resolved so far. */
  const resolveItem = (i: number, inC: ReadonlySet<number>, done: ReadonlyMap<number, Type>) => {
    const isRec = recursive[i];
    const ctx: FromContext = {
      opaque: outer.opaque,
      named: outer.named,
      // f17 12.2 item 4: what a name of item j stands for.
      name: (name, guarded) => {
        const j = lookup(name, i);
        if (j === null || j < 0) return undefined;
        if (inC.has(j)) return guarded ? refType(refs[j]!) : done.get(j);
        // A reference that reaches the top of a recursive item's text through an item that is not recursive is
        // replaced by its definition (R307), so no definition holds a reference at its top.
        if (!recursive[j]) return guarded || !isRec || types[j] === undefined ? types[j] : unfold(types[j]!);
        if (guarded || !isRec) return refType(refs[j]!);
        return refs[j]!.def;
      },
    };
    return resolveText(items[i].node!, items[i].typeSpan ?? NO_SPAN, items[i].spans ?? NO_SPANS, ctx);
  };

  for (const comp of components(n, edges)) {
    comp.sort((a, b) => a - b);
    const inC = new Set(comp);
    const isRec = comp.length > 1 || edges[comp[0]].includes(comp[0]);
    for (const i of comp) recursive[i] = isRec;
    let bad = false;
    // 12.2 items 2 and 7: a name that names nothing, or a failed item of another component.
    for (const i of comp) {
      for (const u of uses[i]) {
        if (u.j === null || (u.j >= 0 && !inC.has(u.j) && failed[u.j])) {
          diagnostics[i].push({ code: "undefined-name", span: spanOf(i, u.node), template: "type", params: { name: u.name } });
          bad = true;
        }
      }
    }
    // 12.2 item 3: a cycle of uses at top places. With none, the order of the top places resolves the component.
    let order = comp;
    if (isRec) {
      const at = new Map(comp.map((v, k) => [v, k]));
      const top = comp.map((i) => uses[i].filter((u) => !u.guarded && u.j !== null && u.j >= 0 && inC.has(u.j)).map((u) => at.get(u.j!)!));
      const sub = components(comp.length, top);
      for (const s of sub) {
        if (s.length > 1 || top[s[0]].includes(s[0])) {
          for (const k of s) {
            diagnostics[comp[k]].push({ code: "circular-type", span: nameSpan(comp[k]), template: "direct", params: { name: items[comp[k]].name } });
            bad = true;
          }
        }
      }
      // Each sub-component is one item now, and they come out each after those it names.
      order = sub.map((s) => comp[s[0]]);
    }
    if (bad) {
      for (const i of comp) failed[i] = true;
      continue;
    }
    if (!isRec) {
      const i = comp[0];
      if (items[i].fixed !== undefined) {
        types[i] = items[i].fixed;
        continue;
      }
      const r = resolveItem(i, inC, new Map());
      if (r.ok) types[i] = r.type;
      else {
        diagnostics[i].push(...r.diagnostics);
        failed[i] = true;
      }
      continue;
    }
    // A recursive component: a reference for each item, then each text, each after the items it names at a top place.
    for (const i of comp) refs[i] = newRef(items[i].name);
    const done = new Map<number, Type>();
    let fails = false;
    for (const i of order) {
      if (uses[i].some((u) => !u.guarded && u.j !== null && u.j >= 0 && inC.has(u.j) && !done.has(u.j))) {
        fails = true;
        continue;
      }
      const r = resolveItem(i, inC, done);
      if (r.ok) done.set(i, r.type);
      else {
        diagnostics[i].push(...r.diagnostics);
        fails = true;
      }
    }
    if (!fails) {
      for (const i of comp) refs[i]!.def = done.get(i);
      // 12.2 item 6: a reference beside a type of its kind, the first of each definition.
      for (const i of comp) {
        const r = besideRef(refs[i]!.def!);
        if (r !== null) {
          diagnostics[i].push({ code: "circular-type", span: nameSpan(i), template: "union", params: { name: r.name } });
          fails = true;
        }
      }
    }
    if (fails) {
      for (const i of comp) failed[i] = true;
      continue;
    }
    for (const i of comp) types[i] = refType(refs[i]!);
  }
  return { types, diagnostics };
}

/** The names of type text, with where each stands (f17 12.1), in source order. */
function usesOf(node: TypeNode, lookup: (name: string) => number | null): Use[] {
  const out: Use[] = [];
  const stack: { n: TypeNode; g: boolean }[] = [{ n: node, g: false }];
  while (stack.length > 0) {
    const { n, g } = stack.pop()!;
    switch (n.t) {
      case "name":
        out.push({ node: n, name: n.name, j: lookup(n.name), guarded: g });
        break;
      case "list":
        stack.push({ n: n.elem, g: true });
        break;
      case "map":
        stack.push({ n: n.rest, g: true });
        break;
      case "record":
        if (typeof n.rest !== "string") stack.push({ n: n.rest, g: true });
        for (let k = n.fields.length - 1; k >= 0; k--) stack.push({ n: n.fields[k].type, g: true });
        break;
      case "union":
        for (let k = n.alts.length - 1; k >= 0; k--) stack.push({ n: n.alts[k], g });
        break;
    }
  }
  return out;
}

/**
 * The strongly connected components of a graph (Tarjan's algorithm, with an
 * explicit stack), each after every component it reaches.
 */
function components(n: number, edges: readonly (readonly number[])[]): number[][] {
  const index = new Array<number>(n).fill(-1);
  const low = new Array<number>(n).fill(0);
  const onStack = new Array<boolean>(n).fill(false);
  const stack: number[] = [];
  const out: number[][] = [];
  let counter = 0;
  for (let s = 0; s < n; s++) {
    if (index[s] !== -1) continue;
    const work: { v: number; k: number }[] = [{ v: s, k: 0 }];
    index[s] = low[s] = counter++;
    stack.push(s);
    onStack[s] = true;
    while (work.length > 0) {
      const fr = work[work.length - 1];
      const v = fr.v;
      if (fr.k < edges[v].length) {
        const w = edges[v][fr.k++];
        if (index[w] === -1) {
          index[w] = low[w] = counter++;
          stack.push(w);
          onStack[w] = true;
          work.push({ v: w, k: 0 });
        } else if (onStack[w]) low[v] = Math.min(low[v], index[w]);
        continue;
      }
      work.pop();
      if (work.length > 0) {
        const u = work[work.length - 1].v;
        low[u] = Math.min(low[u], low[v]);
      }
      if (low[v] === index[v]) {
        const c: number[] = [];
        let w: number;
        do {
          w = stack.pop()!;
          onStack[w] = false;
          c.push(w);
        } while (w !== v);
        out.push(c);
      }
    }
  }
  return out;
}

/**
 * f17 12.2 item 6: the first reference of a definition that shares a kind
 * other than null with another alternative where it stands, in a walk that
 * takes a type before its parts: its references in the order printed, the
 * list's element, then each record by the order of F5's variants, its fields
 * in key order and its rest. A reference is not followed.
 */
export function besideRef(t: Type): Ref | null {
  const seen = new Set<object>();
  const stack: Type[] = [t];
  while (stack.length > 0) {
    const x = stack.pop()!;
    if (isAny(x) || seen.has(x)) continue;
    seen.add(x);
    if (x.refs !== undefined) {
      for (const r of x.refs) {
        if (r.def === undefined) continue;
        const others = kindsOf(withoutRefs(x));
        for (const o of x.refs) if (o !== r && o.def !== undefined) for (const k of kindsOf(o.def)) others.add(k);
        for (const k of kindsOf(r.def)) if (others.has(k)) return r;
      }
    }
    // The parts, pushed in reverse so that they come off the stack in order.
    const parts: Type[] = [];
    if (x.list) parts.push(x.list);
    for (const v of variantsPrinted(x)) {
      for (const k of sortedKeys(v)) parts.push(v.fields.get(k)!.type);
      parts.push(v.rest);
    }
    for (let k = parts.length - 1; k >= 0; k--) stack.push(parts[k]);
  }
  return null;
}
