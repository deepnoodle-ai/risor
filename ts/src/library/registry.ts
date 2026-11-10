// The library's records (04-library.md), from the generated copy of
// suite/library.json, and the calling convention of an entry.

import { LIBRARY } from "../generated/library.ts";
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
    counts: { min, max: rest ? Infinity : params.length, fns: params.map((p) => p.fn), rest },
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

export function method(kind: Kind, name: string): Entry | undefined {
  return METHODS.get(kind)?.get(name);
}

/** Give an entry its implementation. */
export function implement(kind: "string" | "list" | "map" | null, name: string, impl: Impl): void {
  const e = kind === null ? FUNCTIONS.get(name) : METHODS.get(kind)!.get(name);
  if (e === undefined) throw new Error(`no library entry ${kind ?? ""} ${name}`);
  e.impl = impl;
}

/** The counts of every library function, for the static rules. */
export function functionCounts(): Map<string, Counts> {
  return new Map([...FUNCTIONS].map(([k, e]) => [k, e.counts]));
}
