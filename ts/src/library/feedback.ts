// Package F19: hints from tables (f19 3). A check diagnostic of four kinds
// takes a fix from the foreign names of suite/foreign-names.json or from the
// `wrong` lists of the library records. Each step looks a text up by exact
// text: it is no search (f19 3.4 item 7).

import { FOREIGN_NAMES } from "../generated/foreign-names.ts";
import { compareText } from "../value/value.ts";
import { FUNCTIONS, GROUP_FUNCTIONS, METHODS } from "./registry.ts";
import type { Entry } from "./registry.ts";

/** The entries an environment offers (f19 1.1): in a group that is on, and not removed. */
export interface Offered {
  /** The library functions, by full name (`len`, `json.parse`). */
  functions: ReadonlyMap<string, unknown>;
  /** The library methods, by receiver kind, then name. */
  methods: ReadonlyMap<string, ReadonlyMap<string, unknown>>;
}

/** A receiver kind that the lookup of f19 3.4 row 4 reads. */
export type HintKind = "string" | "list" | "map" | "number";

/** An entry a text can name: a function by its full name, or a method by its kind and name. */
type Named = { fn: string } | { kind: string; name: string };

const byWrong = new Map<string, Named[]>();
/** Functions by the name written after a dot, or by their name: `sqrt` names `math.sqrt` (f19 3.4 row 4). */
const fnByName = new Map<string, string[]>();
/** Methods by name. */
const methodByName = new Map<string, { kind: string; name: string }[]>();

function add<T>(m: Map<string, T[]>, k: string, v: T): void {
  const xs = m.get(k);
  if (xs === undefined) m.set(k, [v]);
  else xs.push(v);
}

function wrongOf(e: Entry): string[] {
  const w = e.record.wrong;
  return Array.isArray(w) ? (w as string[]) : [];
}

for (const [full, e] of [...FUNCTIONS, ...GROUP_FUNCTIONS] as [string, Entry][]) {
  for (const w of wrongOf(e)) add(byWrong, w, { fn: full });
  const dot = full.lastIndexOf(".");
  add(fnByName, dot < 0 ? full : full.slice(dot + 1), full);
}
for (const [kind, m] of METHODS) {
  for (const [name, e] of m) {
    for (const w of wrongOf(e)) add(byWrong, w, { kind, name });
    add(methodByName, name, { kind, name });
  }
}

function offered(o: Offered, n: Named): boolean {
  return "fn" in n ? o.functions.has(n.fn) : o.methods.get(n.kind)?.has(n.name) === true;
}

/** f19 3.4 items 2 and 3: the distinct entry forms, in code point order, the first three, joined. */
function forms(ns: Named[]): string | null {
  if (ns.length === 0) return null;
  const set = new Set(ns.map((n) => ("fn" in n ? `use ${n.fn}(...)` : `use x.${n.name}(...)`)));
  return [...set].sort(compareText).slice(0, 3).join(" or ");
}

/** An entry named as a removal names it (03 3.3.1): `round`, `json.parse`, or a method as `list.last`. */
function offeredByName(o: Offered, name: string): boolean {
  const dot = name.indexOf(".");
  const kind = dot < 0 ? "" : name.slice(0, dot);
  if (kind === "string" || kind === "list" || kind === "map") return o.methods.get(kind)?.has(name.slice(dot + 1)) === true;
  return o.functions.has(name);
}

function foreign(o: Offered, written: string, place: string, kind?: string): string | null {
  for (const r of FOREIGN_NAMES) {
    if (r.written !== written || r.place !== place) continue;
    if (place === "method" && !(r.kinds ?? []).includes(kind as "string")) continue;
    // f19 3.3 item 2: a row applies only when every entry its fix names is offered.
    if (!(r.entries ?? []).every((e) => offeredByName(o, e))) continue;
    return r.fix;
  }
  return null;
}

/** The offered entries whose `wrong` list holds the text, that `keep` keeps. */
function wrongEntries(o: Offered, text: string, keep: (n: Named) => boolean = () => true): Named[] {
  return (byWrong.get(text) ?? []).filter((n) => keep(n) && offered(o, n));
}

/**
 * f19 3.4 rows 1 and 2, item 1: an `undefined-name` N of template `name`;
 * `step` is the name f of a step `.f` or `?.f` after it, or null.
 */
export function nameHint(o: Offered, name: string, step: string | null): string | null {
  if (step !== null) {
    const text = `${name}.${step}`;
    const fix = foreign(o, text, "name-field") ?? forms(wrongEntries(o, text));
    if (fix !== null) return fix;
  }
  return foreign(o, name, "name");
}

/** f19 3.4 rows 1 and 3: an `undefined-function` named `name`, its namespace included. */
export function functionHint(o: Offered, name: string): string | null {
  return (
    foreign(o, name, "function") ??
    forms(wrongEntries(o, name)) ??
    forms((methodByName.get(name) ?? []).filter((n) => offered(o, n)))
  );
}

/** f19 3.4 rows 1 and 4: a `no-method` named `name` on a receiver of one kind. */
export function methodHint(o: Offered, kind: HintKind, name: string): string | null {
  return (
    foreign(o, name, "method", kind) ??
    forms(wrongEntries(o, name, (n) => !("fn" in n) && n.kind === kind)) ??
    forms(wrongEntries(o, name, (n) => "fn" in n)) ??
    forms((fnByName.get(name) ?? []).map((fn) => ({ fn })).filter((n) => offered(o, n)))
  );
}

/** f19 3.4 row 5: a `kind` of template `field-read` on a field named `name`. */
export function fieldReadHint(o: Offered, name: string): string | null {
  return forms(wrongEntries(o, name, (n) => "fn" in n));
}
