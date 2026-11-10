// The library's records (04-library.md), from the generated copy of
// suite/library.json, and the calling convention of an entry.

import { LIBRARY } from "../generated/library.ts";
import { GROUP_LIBRARY } from "../generated/library-groups.ts";
import { QUERY_LIBRARY } from "../generated/library-queries.ts";
import { MATH_LIBRARY } from "../generated/library-math.ts";
import type { LibraryRecord } from "../generated/library.ts";
import type { Span } from "../syntax/source.ts";
import { parseReceiver, parseSignature } from "../types/typetext.ts";
import type { TypeNode } from "../types/typetext.ts";
import type { Counts } from "../eval/static.ts";
import type { Run } from "../eval/run.ts";
import type { Kind } from "../value/value.ts";

export interface Param {
  name: string;
  optional: boolean;
  rest: boolean;
  /** The number of parameters of an `Fn` parameter, or null. */
  fn: number | null;
  type: TypeNode | null;
}

/** A lambda passed to an entry. The entry charges the 2 steps of the element, then calls it. */
export interface Callback {
  params: number;
  call(args: unknown[]): unknown;
  /** Package F8: the library call that calls it, for the trace, and its calls so far (f08 4.2). */
  trace?: { span: Span; name: string; count: number };
}

export interface CallContext {
  run: Run;
  /** The whole call: every failure of an entry has this span (04 2.6 item 6). */
  span: Span;
  /** The entry's name as written. */
  name: string;
  receiver: unknown;
  /** The arguments: values, and a Callback at each `Fn` parameter. */
  args: unknown[];
}

export type Impl = (c: CallContext) => unknown;

export interface Entry {
  name: string;
  /** The receiver's kind for a method, or null for a function. */
  kind: "string" | "list" | "map" | null;
  record: LibraryRecord;
  params: Param[];
  counts: Counts;
  impl: Impl | null;
}

function receiverKind(text: string): "string" | "list" | "map" {
  const t = parseReceiver(text);
  if (!t.ok) throw new Error(`library receiver ${text}: ${t.message}`);
  switch (t.node.t) {
    case "word":
      return "string";
    case "list":
      return "list";
    default:
      return "map";
  }
}

function build(rec: LibraryRecord): Entry {
  const s = parseSignature(rec.signature);
  if (!s.ok) throw new Error(`library signature ${rec.name}: ${s.message}`);
  const params: Param[] = s.node.params.map((p) => ({
    name: p.name,
    optional: p.optional,
    rest: p.rest,
    fn: p.type.t === "fn" ? p.type.params.length : null,
    type: p.type.t === "fn" ? null : p.type,
  }));
  const min = params.filter((p) => !p.optional && !p.rest).length;
  const rest = params.length > 0 && params[params.length - 1].rest;
  return {
    name: rec.name,
    kind: rec.receiver ? receiverKind(rec.receiver) : null,
    record: rec,
    params,
    counts: { min, max: rest ? Infinity : params.length, fns: params.map((p) => p.fn), rest, names: params.map((p) => p.name) },
    impl: null,
  };
}

export const FUNCTIONS = new Map<string, Entry>();
export const METHODS = new Map<string, Map<string, Entry>>([
  ["string", new Map()],
  ["list", new Map()],
  ["map", new Map()],
]);

for (const rec of LIBRARY) {
  const e = build(rec);
  if (e.kind === null) FUNCTIONS.set(e.name, e);
  else METHODS.get(e.kind)!.set(e.name, e);
}

// Package F7 (f07 6.1): six new methods, and a record that extends `list.sortBy` in its place.
for (const rec of QUERY_LIBRARY) {
  const e = build(rec);
  const ext = rec.extends as string | undefined;
  if (ext !== undefined) {
    const [kind, name] = ext.split(".");
    if (kind !== e.kind || name !== e.name || !METHODS.get(kind)!.has(name)) throw new Error(`library-queries record ${rec.name}: bad extends ${ext}`);
  } else if (e.kind === null ? FUNCTIONS.has(e.name) : METHODS.get(e.kind)!.has(e.name)) {
    throw new Error(`library-queries record ${rec.name}: a second entry of that name`);
  }
  if (e.kind === null) FUNCTIONS.set(e.name, e);
  else METHODS.get(e.kind)!.set(e.name, e);
}

/** Package F6: the opt-in groups (f06 1.2). */
export const GROUPS: ReadonlySet<string> = new Set(["json", "time", "math", "base64"]);

/** The entries of the groups, by full name (`json.parse`), each with its group. */
export const GROUP_FUNCTIONS = new Map<string, Entry & { group: string }>();

/** The constants of the groups, by full name (`math.pi`) (f06 3.3). */
export const GROUP_CONSTANTS = new Map<string, { name: string; group: string; type: string; value: number }>();

// Package F13: the eleven entries of the group math are records of a group (f13 5 item 1).
for (const rec of [...GROUP_LIBRARY, ...MATH_LIBRARY]) {
  const hasType = rec.type !== undefined;
  if (hasType === (rec.signature !== undefined)) throw new Error(`library-groups record ${rec.name}: exactly one of type and signature`);
  if (hasType) GROUP_CONSTANTS.set(rec.name, { name: rec.name, group: rec.group, type: rec.type!, value: rec.value! });
  else GROUP_FUNCTIONS.set(rec.name, { ...build(rec as unknown as LibraryRecord), group: rec.group });
}

/** A library function by name: a default entry, or an entry of a group. */
export function libraryFunction(name: string): Entry | undefined {
  return FUNCTIONS.get(name) ?? GROUP_FUNCTIONS.get(name);
}

export function method(kind: Kind, name: string): Entry | undefined {
  return METHODS.get(kind)?.get(name);
}

/** Give an entry its implementation. */
export function implement(kind: "string" | "list" | "map" | null, name: string, impl: Impl): void {
  const e = kind === null ? libraryFunction(name) : METHODS.get(kind)!.get(name);
  if (e === undefined) throw new Error(`no library entry ${kind ?? ""} ${name}`);
  e.impl = impl;
}

/** The counts of every library function, for the static rules. */
export function functionCounts(): Map<string, Counts> {
  return new Map([...FUNCTIONS].map(([k, e]) => [k, e.counts]));
}
