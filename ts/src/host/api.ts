// The host API (03-host.md; design 05 Part C, as far as Core needs).
//
//   const env = createEnv({ vars: "{order: {total: float}}", functions: { ... } });
//   const program = env.compile("order.total * 2");
//   program.run({ order: { total: 10 } });          // 20, or throws RisorError
//   program.execute(vars);                          // { ok, value | diagnostic, usage }
//
// An environment and a program do not change once built, and a program may be
// run many times. Host data is read in place, never copied.

import type { Diagnostic } from "../syntax/diagnostic.ts";
import { parse } from "../syntax/parser.ts";
import { Source } from "../syntax/source.ts";
import { arity, parseSignature } from "../types/typetext.ts";
import { compileProgram, HostError } from "../eval/compile.ts";
import type { CallHandle, CompileEnv, HostImpl } from "../eval/compile.ts";
import { RunFailure, messageOf, uncheckedClass } from "../eval/failure.ts";
import { ABSENT_VAR, Run } from "../eval/run.ts";
import type { VarSupply } from "../eval/run.ts";
import { staticRules } from "../eval/static.ts";
import type { Counts, StaticEnv } from "../eval/static.ts";
import { functionCounts, FUNCTIONS, GROUP_CONSTANTS, GROUP_FUNCTIONS } from "../library/registry.ts";
import "../library/groups.ts";
import "../library/core.ts";
import { Classifier, DEFAULT_CLASSIFIER, jsonString, mapGet, mapKeys, readHost, sortKeys, truthy as truthyOf, BAD_NUMBER } from "../value/value.ts";
import { numberToText } from "../value/number.ts";
import { declare, DEFAULT_LIMITS } from "./env.ts";
import { check, spanKey } from "../types/check.ts";
import type { CheckEnv, CheckSig, Expected, Site } from "../types/check.ts";
import { depth, fromNode, printType, opaque as opaqueType, SIZE_LIMIT, typeSize } from "../types/type.ts";
import { conformAllWalk, conformWalk, freeCtx } from "../eval/conform.ts";
import { SchemaError, typeOfSchema } from "../types/schema.ts";
import type { SchemaOption } from "../types/schema.ts";
import { compareText } from "../value/value.ts";
import { analyze } from "../syntax/analysis.ts";
import type { Report } from "../syntax/analysis.ts";
import { parseTemplateText, templateProgram } from "../syntax/template.ts";
import type { ParsedTemplateData, Segment, TemplateMode } from "../syntax/template.ts";
import type { Span } from "../syntax/source.ts";
import type { SigNode, TypeNode } from "../types/typetext.ts";
import type { Type } from "../types/type.ts";
import { parseType as parseTypeText, parseReceiver, parseEnvRecord } from "../types/typetext.ts";
import { METHODS } from "../library/registry.ts";
import type { Declarations, Environment, NamedTypeDecl } from "./env.ts";

export { HostError };
export type { CallHandle };

export interface HostFunction {
  /** The signature in type text: "(s: string, n?: int) => string !". */
  type: string;
  /**
   * Synchronous. Throw an `Error` (a `HostError`, say) to return an error:
   * `function-error`. Any other thrown value is a crash: `function-panic`.
   */
  call(handle: CallHandle, ...args: unknown[]): unknown;
}

export interface OpaqueType {
  /** Whether a host value carries this type. */
  test(value: unknown): boolean;
  /** Methods: the receiver is the first argument of `call`. */
  methods?: Record<string, HostFunction>;
}

export interface Limits {
  /** null turns the limit off. */
  steps?: number | null;
  allocation?: number | null;
  depth?: number;
  /** The deadline of one run, in milliseconds, or null for none. */
  timeoutMs?: number | null;
}

export interface EnvOptions {
  version?: string;
  /** One record type in type text: "{user: {age: int}}". */
  vars?: string;
  opaque?: Record<string, OpaqueType>;
  /** Keys may have one namespace: "geo.distance". */
  functions?: Record<string, HostFunction>;
  reject?: string[];
  limits?: Limits;
  /** The opaque type names as a list, when it is given apart from `opaque` (a case file's `opaque`). */
  opaqueNames?: string[];
  /** Variables declared by JSON Schema, each with its options (package F1, f01 2). */
  schemas?: Record<string, { schema: unknown; options?: SchemaOption[] }>;
  /** Variables each declared by its own type text, as a description gives them (f01 5.4). */
  variables?: Record<string, string>;
  /** Library entries the environment does not have (03 3.3.1). */
  removed?: string[];
  /** Internal: the variables a child keeps from its parent (03 3.3.2). */
  inherited?: Declarations["inherited"];
  /** Package F6: the opt-in groups to turn on: `json`, `time`, `math`, `base64` (f06 2.1). */
  groups?: string[];
  /** Package F4: named types, in the order declared (f04 3.1). */
  types?: NamedTypeDecl[];
  /** Internal: the named types a child keeps from its parent (f04 3.4). */
  inheritedTypes?: Declarations["inheritedTypes"];
}

/** An opaque type in a child environment: with no `test`, it adds methods to an inherited type (03 3.3.2). */
export interface ChildOpaqueType {
  test?(value: unknown): boolean;
  methods?: Record<string, HostFunction>;
}

/** The declarations of a child environment (03 3.3.2). */
export interface ChildOptions extends Omit<EnvOptions, "opaque" | "inherited" | "inheritedTypes"> {
  opaque?: Record<string, ChildOpaqueType>;
}

export interface Usage {
  steps: number;
  allocated: number;
}

export type Vars = { readonly [name: string]: unknown } | ReadonlyMap<string, unknown> | ((name: string) => unknown);

export type Outcome =
  | { ok: true; value: unknown; usage: Usage }
  | { ok: false; diagnostic: Diagnostic; usage: Usage };

export class RisorError extends Error {
  readonly phase: Diagnostic["phase"];
  readonly diagnostics: readonly Diagnostic[];
  readonly usage?: Usage;
  constructor(diagnostics: Diagnostic[], usage?: Usage) {
    super(diagnostics.map((d) => `${d.code}: ${d.message}`).join("; "));
    this.phase = diagnostics[0].phase;
    this.diagnostics = diagnostics;
    if (usage !== undefined) this.usage = usage;
  }
}

/** The language's truthiness test (03 3.2.4). Not JavaScript's: [] and {} are falsy. */
export function truthy(value: unknown): boolean {
  return truthyOf(value);
}

/** The JSON text of a result (03 3.2.4). Throws RisorError on an opaque value or on nesting past 200. */
export function toJSON(value: unknown, cls: Classifier = DEFAULT_CLASSIFIER): string {
  const fail = (code: string, template: string): never => {
    throw new RisorError([{ phase: "run", code, message: messageOf(new RunFailure(code, undefined, template, { limit: 200 }), cls) }]);
  };
  const walk = (v: unknown, depth: number): string => {
    v = readHost(v);
    if (v === BAD_NUMBER) fail("number-range", "host");
    switch (cls.kind(v)) {
      case "null": return "null";
      case "bool": return String(v);
      case "number": return numberToText(Number(v));
      case "string": return jsonString(v as string);
      case "opaque": return fail("wrong-kind", "json");
      case "list":
        if (depth >= 200) fail("depth-exceeded", "default");
        return "[" + (v as unknown[]).map((x) => walk(x, depth + 1)).join(",") + "]";
      case "map": {
        if (depth >= 200) fail("depth-exceeded", "default");
        const keys = sortKeys(mapKeys(v as object));
        return "{" + keys.map((k) => jsonString(k) + ":" + walk(mapGet(v as object, k), depth + 1)).join(",") + "}";
      }
    }
  };
  return walk(value, 0);
}

function countsOf(type: string): Counts {
  const s = parseSignature(type);
  if (!s.ok) return { min: 0, max: Infinity, fns: [], rest: true };
  const a = arity(s.node);
  return { min: a.min, max: a.max, fns: s.node.params.map(() => null), rest: a.max === Infinity, names: s.node.params.map((p) => p.name) };
}

function hostImpl(f: HostFunction): HostImpl {
  return { counts: countsOf(f.type), call: (handle, args) => f.call(handle, ...args) };
}

/** Build an environment. Throws RisorError of phase `declare`. */
export function createEnv(options: EnvOptions = {}): Env {
  const methods: Record<string, string> = {};
  for (const [t, o] of Object.entries(options.opaque ?? {})) {
    for (const [m, f] of Object.entries(o.methods ?? {})) methods[`${t}.${m}`] = f.type;
  }
  const functions: Record<string, string> = {};
  for (const [k, f] of Object.entries(options.functions ?? {})) functions[k] = f.type;
  const limits = options.limits ?? {};
  const t = limits.timeoutMs;
  if (t !== undefined && t !== null && (typeof t !== "number" || !Number.isFinite(t) || t < 0)) {
    throw new RisorError([{ phase: "declare", code: "bad-limit", message: "the deadline must be a number of milliseconds of 0 or more, or off" }]);
  }
  const decls: Declarations = {
    version: options.version,
    env: options.vars,
    opaque: options.opaqueNames ?? Object.keys(options.opaque ?? {}),
    methods,
    functions,
    reject: options.reject,
    schemas: options.schemas,
    variableTexts: options.variables,
    removed: options.removed,
    groups: options.groups,
    inherited: options.inherited,
    types: options.types,
    inheritedTypes: options.inheritedTypes,
    limits: {
      ...(limits.steps !== undefined ? { steps: limits.steps } : {}),
      ...(limits.allocation !== undefined ? { allocation: limits.allocation } : {}),
      ...(limits.depth !== undefined ? { depth: limits.depth } : {}),
    },
  };
  const removed = new Set(options.removed ?? []);
  const d = declare(decls, new Set([...FUNCTIONS.keys()].filter((k) => !removed.has(k))));
  if (!d.ok) throw new RisorError(d.diagnostics);
  return new Env(d.env, options, limits.timeoutMs ?? null);
}

/** The type of a JSON Schema with options, printed (package F1, f01 3, 4). Throws RisorError `bad-type`. */
export function typeFromSchema(schema: unknown, options: SchemaOption[] = []): string {
  try {
    return printType(typeOfSchema(schema, options));
  } catch (e) {
    if (e instanceof SchemaError) throw new RisorError([{ phase: "declare", code: "bad-type", message: e.message }]);
    throw e;
  }
}

/** The description of an environment (f01 5.3). */
export interface Description {
  version: string;
  variables: Record<string, string>;
  functions: Record<string, string>;
  opaque: Record<string, Record<string, string>>;
  removed: string[];
  reject: string[];
  limits: { steps: number | null; allocation: number | null; depth: number };
  /** Package F4 (f04 8): each named type, printed by structure; only when there is one. */
  types?: Record<string, string>;
  /** Package F6 (f06 2.7): the groups that are on, in code point order; only when one is. */
  groups?: string[];
}

/** The JSON text of a description (f01 5.3 item 5): keys in code point order, no white space. */
export function descriptionText(d: Description): string {
  return toJSON(d);
}

/**
 * An environment from a description (f01 5.4). It has no implementation of
 * any function or method: compiling against it fails with `no-implementation`
 * while it declares one. Throws RisorError (`declare`) for an error of 03
 * 3.3.4, and a plain Error for a value that is not a description.
 */
export function envFromDescription(description: unknown): Env {
  const d = description as Record<string, unknown>;
  const bad = (what: string): never => {
    throw new Error(`not a description: ${what}`);
  };
  const isObj = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null && !Array.isArray(v);
  const strings = (v: unknown, what: string): Record<string, string> => {
    if (!isObj(v) || !Object.values(v).every((x) => typeof x === "string")) bad(what);
    return v as Record<string, string>;
  };
  if (!isObj(d)) bad("not an object");
  const keys = ["version", "variables", "functions", "opaque", "removed", "reject", "limits"];
  for (const k of Object.keys(d)) if (!keys.includes(k) && k !== "types" && k !== "groups") bad(`the key ${k}`);
  for (const k of keys) if (!(k in d)) bad(`no key ${k}`);
  // f04 8 item 3: each entry of `types` is a named type by type text.
  const groups = "groups" in d ? (Array.isArray(d.groups) && d.groups.every((x) => typeof x === "string") ? (d.groups as string[]) : bad("groups")) : undefined;
  const types = "types" in d ? Object.entries(strings(d.types, "types")).map(([name, type]) => ({ name, type })) : undefined;
  if (typeof d.version !== "string") bad("version");
  const variables = strings(d.variables, "variables");
  const functions = strings(d.functions, "functions");
  if (!isObj(d.opaque)) bad("opaque");
  const opaque: Record<string, OpaqueType> = {};
  for (const [t, ms] of Object.entries(d.opaque as Record<string, unknown>)) {
    const methods: Record<string, HostFunction> = {};
    for (const [m, sig] of Object.entries(strings(ms, `the methods of ${t}`))) methods[m] = { type: sig, call: noImplementation };
    opaque[t] = { test: () => false, methods };
  }
  const list = (v: unknown, what: string): string[] => {
    if (!Array.isArray(v) || !v.every((x) => typeof x === "string")) bad(what);
    return v as string[];
  };
  const removed = list(d.removed, "removed");
  const reject = list(d.reject, "reject");
  if (!isObj(d.limits)) bad("limits");
  const lim = d.limits as Record<string, unknown>;
  const lk = ["steps", "allocation", "depth"];
  for (const k of Object.keys(lim)) if (!lk.includes(k)) bad(`the limit ${k}`);
  for (const k of lk) if (!(k in lim)) bad(`no limit ${k}`);
  const num = (v: unknown, nullable: boolean, what: string) => {
    if (v === null && nullable) return null;
    if (typeof v !== "number") bad(what);
    return v as number;
  };
  const env = createEnv({
    version: d.version as string,
    variables,
    opaque,
    functions: Object.fromEntries(Object.entries(functions).map(([k, sig]) => [k, { type: sig, call: noImplementation }])),
    removed,
    reject,
    types,
    groups,
    limits: {
      steps: num(lim.steps, true, "steps"),
      allocation: num(lim.allocation, true, "allocation"),
      depth: num(lim.depth, false, "depth") as number,
    },
  });
  env.noImplementation = true;
  return env;
}

function noImplementation(): never {
  throw new Error("no implementation");
}

export type CompileResult = { ok: true; program: Program } | { ok: false; diagnostics: Diagnostic[]; source: Source };

/** The expected result (03 3.2.3): type text, "condition", or a type by JSON Schema (package F10, f10 2.1). */
export type Expectation = string | { schema: unknown; options?: SchemaOption[] };

export interface CompileOptions {
  /** Apply the type rules (05-types.md). Default true. The static rules always apply. */
  check?: boolean;
  /** The expected result (03 3.2.3; f10 2.1). */
  expected?: Expectation;
}

/** What a load takes besides the text (package F10, f10 5.1). */
export interface LoadOptions {
  /** The variables of the run (03 3.4.1). */
  vars?: Vars;
  signal?: AbortSignal;
  /** The most failures step 4 reports (f10 5.2 item 1). Default 1; below 1 is read as 1. */
  max?: number;
  /** Make step 5, the decode into plain data (f10 6.6). Default true. */
  decode?: boolean;
}

/** The outcome of a load (f10 5.2 item 4). */
export type LoadOutcome<T = unknown> =
  | { ok: true; value: T; usage: Usage }
  | { ok: false; step: 0 | 1 | 2 | 3 | 4 | 5; diagnostics: Diagnostic[]; usage?: Usage; source?: Source };

/** A load that failed (f10 5.2): the step, the diagnostics and the usage. */
export class LoadError extends RisorError {
  readonly step: number;
  constructor(step: number, diagnostics: Diagnostic[], usage?: Usage) {
    super(diagnostics, usage);
    this.step = step;
  }
}

/**
 * The decode into plain data (package F10, f10 6.1 and 6.6): a copy in which a
 * map is an object whose own properties are exactly its keys, and a list an
 * Array. An opaque value is not-conforming at its path; a host number that
 * cannot enter the language is number-range; nesting deeper than 200 is
 * depth-exceeded with no path. Not a run: nothing is charged.
 */
export function decode(value: unknown, cls: Classifier = DEFAULT_CLASSIFIER): { ok: true; value: unknown } | { ok: false; diagnostic: Diagnostic } {
  class Stop {
    readonly d: Diagnostic;
    constructor(d: Diagnostic) {
      this.d = d;
    }
  }
  const walk = (v: unknown, path: (string | number)[]): unknown => {
    v = readHost(v);
    if (v === BAD_NUMBER) throw new Stop({ phase: "run", code: "number-range", path, message: "the host number cannot enter the language" });
    switch (cls.kind(v)) {
      case "null": return null;
      case "bool": return v;
      case "number": return Number(v) + 0;
      case "string": return v;
      case "list": {
        if (path.length >= 200) throw new Stop({ phase: "run", code: "depth-exceeded", message: messageOf(new RunFailure("depth-exceeded", undefined, "default", { limit: 200 }), cls) });
        return (v as unknown[]).map((x, i) => walk(x, [...path, i]));
      }
      case "map": {
        if (path.length >= 200) throw new Stop({ phase: "run", code: "depth-exceeded", message: messageOf(new RunFailure("depth-exceeded", undefined, "default", { limit: 200 }), cls) });
        const out: Record<string, unknown> = {};
        for (const k of mapKeys(v as object)) Object.defineProperty(out, k, { value: walk(mapGet(v as object, k), [...path, k]), enumerable: true, writable: true, configurable: true });
        return out;
      }
      default:
        throw new Stop({ phase: "run", code: "not-conforming", class: "value", path, message: "an opaque value is not data, and is not decoded" });
    }
  };
  try {
    return { ok: true, value: walk(value, []) };
  } catch (e) {
    if (e instanceof Stop) return { ok: false, diagnostic: e.d };
    throw e;
  }
}

/** The result of a check (03 3.2.1): the type and sites, or the diagnostics. */
export type CheckOutcome =
  | { ok: true; type: string; sites: Site[]; source: Source }
  | { ok: false; diagnostics: Diagnostic[]; source: Source };

/** The library's signatures, for the checker (05 13.1). */
function librarySig(e: { name: string; record: { receiver?: string; signature: string; [k: string]: unknown } }, kind: "library"): CheckSig {
  const s = parseSignature(e.record.signature);
  if (!s.ok) throw new Error(`library signature ${e.name}`);
  let recv = null;
  if (e.record.receiver) {
    const r = parseReceiver(e.record.receiver);
    if (!r.ok) throw new Error(`library receiver ${e.name}`);
    recv = r.node;
  }
  return {
    name: e.name, kind, recv, sig: s.node,
    narrows: e.record.narrows !== undefined,
    tested: Array.isArray(e.record.tested) ? (e.record.tested as number[]) : [],
    renders: Array.isArray(e.record.renders) ? (e.record.renders as string[]).map((n) => s.node.params.findIndex((p) => p.name === n)).filter((i) => i >= 0) : [],
  };
}
const LIBRARY_FUNCTIONS = new Map([...FUNCTIONS].map(([k, e]) => [k, librarySig(e, "library")]));
const GROUP_SIGS = new Map([...GROUP_FUNCTIONS].map(([k, e]) => [k, librarySig(e, "library")]));
const LIBRARY_METHODS = new Map([...METHODS].map(([kind, m]) => [kind, new Map([...m].map(([k, e]) => [k, librarySig(e, "library")]))]));

export class Env {
  readonly environment: Environment;
  private readonly compileEnv: CompileEnv;
  private readonly staticEnv: StaticEnv;
  readonly classifier: Classifier;
  readonly timeoutMs: number | null;

  private readonly options: EnvOptions;

  constructor(environment: Environment, options: EnvOptions, timeoutMs: number | null) {
    this.environment = environment;
    this.options = options;
    this.timeoutMs = timeoutMs;
    const fns = new Map<string, HostImpl>();
    for (const [k, f] of Object.entries(options.functions ?? {})) fns.set(k, hostImpl(f));
    const methods = new Map<string, Map<string, HostImpl>>();
    const tests: [string, (v: unknown) => boolean][] = [];
    for (const [t, o] of Object.entries(options.opaque ?? {})) {
      tests.push([t, o.test]);
      methods.set(t, new Map(Object.entries(o.methods ?? {}).map(([m, f]) => [m, hostImpl(f)])));
    }
    this.classifier = new Classifier(tests);
    this.compileEnv = { functions: fns, methods, removed: environment.removed };
    const hostCounts = new Map<string, Counts>();
    for (const [k, f] of fns) hostCounts.set(k, f.counts);
    // f06 2.5: a group that is on is a namespace; its entries are library functions, its constants are read.
    const on = environment.groups;
    const groupEntries = [...GROUP_FUNCTIONS].filter(([k, e]) => on.has(e.group) && !environment.removed.has(k));
    const constants = new Map([...GROUP_CONSTANTS].filter(([k, c]) => on.has(c.group) && !environment.removed.has(k)).map(([k, c]) => [k, c.value]));
    this.staticEnv = {
      variables: new Set(environment.variables.keys()),
      hostFunctions: hostCounts,
      namespaces: on.size === 0 ? environment.namespaces : new Set([...environment.namespaces, ...on]),
      library: new Map([...functionCounts()].filter(([k]) => !environment.removed.has(k)).concat(groupEntries.map(([k, e]) => [k, e.counts]))),
      typeNames: { opaque: environment.opaque, named: environment.named },
      constants,
    };
    const hostSig = (name: string, sig: import("../types/typetext.ts").SigNode, recv: import("../types/typetext.ts").TypeNode | null): CheckSig => ({
      name, kind: "host", recv, sig, narrows: false, tested: [], renders: [],
    });
    const methodSigs = new Map<string, Map<string, CheckSig>>();
    for (const m of environment.methods.values()) {
      if (!methodSigs.has(m.type)) methodSigs.set(m.type, new Map());
      methodSigs.get(m.type)!.set(m.name, hostSig(`${m.type}.${m.name}`, m.sig, { t: "name", name: m.type }));
    }
    this.checkEnv = {
      opaque: environment.opaque,
      named: environment.named,
      variables: environment.variableTypes,
      functions: new Map([...environment.functions].map(([k, fn]) => [k, hostSig(k, fn.sig, null)])),
      library: environment.removed.size === 0 && groupEntries.length === 0
        ? LIBRARY_FUNCTIONS
        : new Map([...LIBRARY_FUNCTIONS].filter(([k]) => !environment.removed.has(k)).concat(groupEntries.map(([k]) => [k, GROUP_SIGS.get(k)!]))),
      libraryMethods:
        environment.removed.size === 0
          ? LIBRARY_METHODS
          : new Map([...LIBRARY_METHODS].map(([kind, m]) => [kind, new Map([...m].filter(([k]) => !environment.removed.has(`${kind}.${k}`)))])),
      methods: methodSigs,
      reject: environment.reject,
    };
  }

  private readonly checkEnv: CheckEnv;

  /** The expected result as the checker takes it. Throws RisorError `bad-type` for bad text or a bad schema. */
  private expected(text: Expectation | undefined): Expected {
    if (text === undefined) return { kind: "none" };
    if (text === "condition") return { kind: "condition" };
    const fail = (m: string): never => {
      throw new RisorError([{ phase: "declare", code: "bad-type", message: `the expected result: ${m}` }]);
    };
    if (typeof text !== "string") {
      // f10 2.1: only `closed` is accepted; the schema's errors and the limits of F1 3.10 are bad-type.
      const opts = text.options ?? [];
      for (const o of opts) if (o !== "closed") fail(`the option ${o} states what a host did to a value it supplied, and nobody supplies a result`);
      let t: Type;
      try {
        t = typeOfSchema(text.schema, opts);
      } catch (e) {
        if (e instanceof SchemaError) return fail(e.message);
        throw e;
      }
      if (depth(t, 70) > 64) fail("a type deeper than 64");
      if (typeSize(t) > SIZE_LIMIT) fail("a type larger than 100,000");
      return { kind: "type", type: t };
    }
    const t = parseTypeText(text);
    if (!t.ok) fail(t.message);
    try {
      return { kind: "type", type: fromNode((t as { node: import("../types/typetext.ts").TypeNode }).node, { opaque: this.environment.opaque, named: this.environment.named }) };
    } catch (e) {
      return fail((e as Error).message);
    }
  }

  /** The type of an expected result that is a type (f10 5.2 step 0). Throws RisorError `bad-type`. */
  expectedType(e: Expectation): Type {
    const x = this.expected(e);
    if (x.kind !== "type") throw new RisorError([{ phase: "declare", code: "bad-type", message: "the expected result is not a type" }]);
    return x.type;
  }

  /** Read type text against this environment's opaque types. Throws on bad text. */
  typeOf(text: string): Type {
    const t = parseTypeText(text);
    if (!t.ok) throw new Error(t.message);
    return fromNode(t.node, { opaque: this.environment.opaque, named: this.environment.named });
  }

  /** Set on an environment built from a description: it has no implementation (f01 5.4 item 4). */
  noImplementation = false;

  /** The description of this environment (f01 5.3). */
  describe(): Description {
    const e = this.environment;
    const sorted = <T>(m: Iterable<[string, T]>) => Object.fromEntries([...m].sort((a, b) => compareText(a[0], b[0])));
    const opaque: Record<string, Record<string, string>> = {};
    for (const t of [...e.opaque].sort(compareText)) opaque[t] = {};
    for (const m of e.methods.values()) opaque[m.type][m.name] = printSignature(m.sig, e.opaque, e.named);
    for (const t of Object.keys(opaque)) opaque[t] = sorted(Object.entries(opaque[t]));
    return {
      version: e.version,
      variables: sorted([...e.variableTypes].map(([k, t]) => [k, printType(t)])),
      functions: sorted([...e.functions].map(([k, f]) => [k, printSignature(f.sig, e.opaque, e.named)])),
      opaque,
      removed: [...e.removed].sort(compareText),
      reject: [...e.reject].sort(compareText),
      limits: { steps: e.limits.steps, allocation: e.limits.allocation, depth: e.limits.depth },
      ...(e.named.size > 0 ? { types: sorted([...e.named].map(([k, t]) => [k, printType(t)])) } : {}),
      ...(e.groups.size > 0 ? { groups: [...e.groups].sort(compareText) } : {}),
    };
  }

  /**
   * The conformance function (package F4, f04 7): null when the value conforms
   * to the type, else the diagnostic, with no span. Not a run: nothing is
   * charged and no limit applies. Throws RisorError `bad-type` for bad text.
   */
  conform(value: unknown, typeText: string): Diagnostic | null {
    const t = this.declaredType(typeText);
    const f = conformWalk(value, t, freeCtx(this.classifier));
    if (f === null) return null;
    const failure = new RunFailure("not-conforming", undefined, f.template, f.params);
    return { phase: "run", code: "not-conforming", class: "value", path: f.path, message: messageOf(failure, this.classifier), trace: [] };
  }

  /**
   * The conformance function that reports every failure, up to `max` (package
   * F8, f08 6.1). A maximum below 1 is read as 1. Nothing is charged.
   */
  conformAll(value: unknown, typeText: string, max: number): Diagnostic[] {
    const t = this.declaredType(typeText);
    return conformAllWalk(value, t, this.classifier, Math.max(1, Math.floor(max))).map((f) => {
      const failure = new RunFailure("not-conforming", undefined, f.template, f.params);
      return { phase: "run" as const, code: "not-conforming", class: "value", path: f.path, message: messageOf(failure, this.classifier), trace: [] };
    });
  }

  /**
   * A load (package F10, f10 5.2): declare T, parse, check and compile with T
   * as the expected result, run, test the value against T by the conformance
   * function that reports every failure, and decode it into plain data. A
   * schema is mapped with `closed` and no other option (f10 5.6). Never throws
   * for a failure of a step: the outcome gives the step and its diagnostics.
   */
  tryLoad<T = unknown>(text: string, expected: string | { schema: unknown }, options: LoadOptions = {}): LoadOutcome<T> {
    // Step 0: declare.
    let exp: Expected;
    try {
      if (expected === "condition") throw new RisorError([{ phase: "declare", code: "bad-type", message: "the expected result of a load is a type" }]);
      exp = this.expected(typeof expected === "string" ? expected : { schema: expected.schema, options: ["closed"] });
    } catch (e) {
      if (e instanceof RisorError) return { ok: false, step: 0, diagnostics: [...e.diagnostics] };
      throw e;
    }
    const type = (exp as { type: Type }).type;
    // Steps 1 and 2: parse, then check and compile. A load never compiles without the check.
    let compiled: CompileResult;
    try {
      compiled = this.tryCompile(text, { expected: typeof expected === "string" ? expected : { schema: expected.schema, options: ["closed"] } });
    } catch (e) {
      if (e instanceof RisorError) return { ok: false, step: 0, diagnostics: [...e.diagnostics] };
      throw e;
    }
    if (!compiled.ok) return { ok: false, step: compiled.diagnostics[0].phase === "parse" ? 1 : 2, diagnostics: compiled.diagnostics, source: compiled.source };
    // Step 3: run.
    const o = compiled.program.execute(options.vars, { signal: options.signal });
    if (!o.ok) return { ok: false, step: 3, diagnostics: [o.diagnostic], usage: o.usage, source: compiled.program.source };
    // Step 4: conform, every failure up to the maximum.
    const fs = conformAllWalk(o.value, type, this.classifier, Math.max(1, Math.floor(options.max ?? 1)));
    if (fs.length > 0) {
      const diagnostics = fs.map((f): Diagnostic => ({ phase: "run", code: "not-conforming", class: "value", path: f.path, message: messageOf(new RunFailure("not-conforming", undefined, f.template, f.params), this.classifier) }));
      return { ok: false, step: 4, diagnostics, usage: o.usage };
    }
    // Step 5: the decode into plain data.
    if (options.decode === false) return { ok: true, value: o.value as T, usage: o.usage };
    const d = decode(o.value, this.classifier);
    if (!d.ok) return { ok: false, step: 5, diagnostics: [d.diagnostic], usage: o.usage };
    return { ok: true, value: d.value as T, usage: o.usage };
  }

  /** A load (f10 5.2) that gives the value, or throws LoadError with the step that failed. */
  load<T = unknown>(text: string, expected: string | { schema: unknown }, options: LoadOptions = {}): T {
    const r = this.tryLoad<T>(text, expected, options);
    if (!r.ok) throw new LoadError(r.step, r.diagnostics, r.usage);
    return r.value;
  }

  /** Type text resolved as declared type text (05 19.5, f04 7.1 item 4). Throws RisorError `bad-type`. */
  declaredType(text: string): Type {
    const fail = (m: string): never => {
      throw new RisorError([{ phase: "declare", code: "bad-type", message: `the type: ${m}` }]);
    };
    const p = parseTypeText(text);
    if (!p.ok) return fail(p.message);
    let t: Type;
    try {
      t = fromNode(p.node, { opaque: this.environment.opaque, named: this.environment.named });
    } catch (e) {
      return fail((e as Error).message);
    }
    if (depth(t, 70) > 64) fail("a type deeper than 64");
    if (typeSize(t) > SIZE_LIMIT) fail("a type larger than 100,000");
    return t;
  }

  /** Check a program (03 3.2.1): parse, the static rules, the type rules. */
  check(text: string | ParsedProgram, expected?: Expectation): CheckOutcome {
    const exp = this.expected(expected);
    const p = parseInput(text);
    if (!p.ok) return { ok: false, diagnostics: p.diagnostics, source: p.source };
    const s = staticRules(p.program, this.staticEnv, p.source);
    const c = check(p.program, s.resolution, this.checkEnv, exp, s.diagnostics, p.source);
    if (c.diagnostics.length > 0) return { ok: false, diagnostics: c.diagnostics, source: p.source };
    return { ok: true, type: printType(c.type), sites: c.sites, source: p.source };
  }

  /**
   * A child environment (03 3.3.2). A child's declaration replaces the parent's
   * declaration of the same name in the same space: variables and host
   * functions are one space, opaque types another (03 3.3.4 item 1). An opaque
   * type the child declares (with `test`) replaces the parent's whole; one given
   * with methods only adds or replaces those methods. The clash rules
   * apply to the child as it stands. Rejected classes and limits are the
   * parent's unless given; removed entries add up. Throws RisorError of phase
   * `declare`.
   */
  child(options: ChildOptions = {}): Env {
    const p = this.options;
    const pe = this.environment;
    const valueNames = new Set<string>([...Object.keys(options.variables ?? {}), ...Object.keys(options.schemas ?? {}), ...Object.keys(options.functions ?? {})]);
    if (options.vars !== undefined) {
      const t = parseEnvRecord(options.vars);
      if (t.ok && t.node.t === "record") for (const f of t.node.fields) valueNames.add(f.key);
    }
    const inherited = new Map<string, { node: TypeNode; type: Type | undefined }>();
    for (const [n, node] of pe.variables) if (!valueNames.has(n)) inherited.set(n, { node, type: pe.variableTypes.get(n) });
    const functions: Record<string, HostFunction> = {};
    for (const [k, f] of Object.entries(p.functions ?? {})) if (!valueNames.has(k)) functions[k] = f;
    Object.assign(functions, options.functions ?? {});
    // Opaque types: the parent's, less those the child declares.
    const childTypes = new Set(options.opaqueNames ?? Object.entries(options.opaque ?? {}).filter(([, o]) => o.test !== undefined).map(([k]) => k));
    const opaqueNames = [...pe.opaque].filter((t) => !childTypes.has(t));
    const opaque: Record<string, OpaqueType> = {};
    for (const t of opaqueNames) opaque[t] = { test: p.opaque?.[t]?.test ?? (() => false), methods: { ...(p.opaque?.[t]?.methods ?? {}) } };
    for (const [t, o] of Object.entries(options.opaque ?? {})) {
      if (childTypes.has(t)) opaque[t] = { test: o.test ?? (() => false), methods: { ...(o.methods ?? {}) } };
      else if (opaque[t] !== undefined) Object.assign(opaque[t].methods!, o.methods ?? {});
      // A method of a type neither declares: declare reports it (bad-signature).
      else opaque[t] = { test: () => false, methods: { ...(o.methods ?? {}) } };
    }
    for (const t of childTypes) if (!opaqueNames.includes(t)) opaqueNames.push(t);
    // f04 3.4: the parent's named types, less those the child declares.
    const childNamed = new Set((options.types ?? []).map((t) => t.name));
    const inheritedTypes = new Map([...pe.named].filter(([k]) => !childNamed.has(k)));
    const env = createEnv({
      version: options.version ?? p.version,
      vars: options.vars,
      variables: options.variables,
      schemas: options.schemas,
      inherited,
      types: options.types,
      inheritedTypes,
      functions,
      opaque,
      opaqueNames,
      reject: options.reject ?? (p.reject !== undefined ? p.reject : [...pe.reject]),
      limits: options.limits ?? p.limits,
      removed: [...pe.removed, ...(options.removed ?? [])],
      // f06 2.6 item 1: the parent's groups and the child's own.
      groups: [...pe.groups, ...(options.groups ?? [])],
    });
    env.noImplementation = this.noImplementation;
    return env;
  }

  /** Parse, apply the static rules and compile. Problems are data. */
  tryCompile(text: string | ParsedProgram, options: CompileOptions = {}): CompileResult {
    if (this.noImplementation && (this.environment.functions.size > 0 || this.environment.methods.size > 0)) {
      throw new RisorError([{ phase: "declare", code: "no-implementation", message: "the environment declares a function with no implementation" }]);
    }
    const checked = options.check !== false;
    // f10 2.1 item 6: a compile without a check still maps a schema and reports its errors.
    if (!checked && options.expected !== undefined && typeof options.expected !== "string") this.expected(options.expected);
    const exp = checked ? this.expected(options.expected) : { kind: "none" as const };
    const p = parseInput(text);
    if (!p.ok) return { ok: false, diagnostics: p.diagnostics, source: p.source };
    const s = staticRules(p.program, this.staticEnv, p.source);
    let dynamicSpans: Set<string> | null = null;
    let hostSpans: Set<string> | null = null;
    let valueSpans: Set<string> | null = null;
    let type: string | null = null;
    let sites: Site[] = [];
    if (checked) {
      const c = check(p.program, s.resolution, this.checkEnv, exp, s.diagnostics, p.source);
      if (c.diagnostics.length > 0) return { ok: false, diagnostics: c.diagnostics, source: p.source };
      dynamicSpans = c.dynamicSpans;
      hostSpans = c.hostSpans;
      valueSpans = c.valueSpans;
      type = printType(c.type);
      sites = c.sites;
    } else if (s.diagnostics.length > 0) return { ok: false, diagnostics: s.diagnostics, source: p.source };
    const c = compileProgram(p.program, s.resolution, this.compileEnv);
    const prog = new Program(this, p.source, c.run, s.resolution.slotCount, c.missing, dynamicSpans, hostSpans, valueSpans);
    prog.type = type;
    prog.sites = sites;
    return { ok: true, program: prog };
  }

  /** Compile a program, with a check unless `check: false`. Throws RisorError of phase `declare`, `parse` or `check`. */
  compile(text: string | ParsedProgram, options: CompileOptions = {}): Program {
    const r = this.tryCompile(text, options);
    if (!r.ok) throw new RisorError(r.diagnostics);
    return r.program;
  }
}

function supplyOf(vars: Vars | undefined): VarSupply {
  if (vars === undefined) return () => ABSENT_VAR;
  if (typeof vars === "function") {
    return (name) => {
      const v = vars(name);
      return v === undefined ? ABSENT_VAR : v;
    };
  }
  if (vars instanceof Map) {
    return (name) => {
      const v = vars.get(name);
      return v === undefined ? ABSENT_VAR : v;
    };
  }
  return (name) => {
    const v = Object.prototype.hasOwnProperty.call(vars, name) ? (vars as Record<string, unknown>)[name] : undefined;
    return v === undefined ? ABSENT_VAR : v;
  };
}

export class Program {
  readonly env: Env;
  readonly source: Source;
  private readonly code: (r: Run) => unknown;
  private readonly slotCount: number;
  /** Library entries the program names that this build does not implement yet. */
  readonly missing: ReadonlySet<string>;

  /** The result type the check gave, printed (05 3.9); null when compiled without a check. */
  type: string | null = null;
  /** The sites the check gave. */
  sites: Site[] = [];
  /** Spans at a `dynamic` site, for the class of a failure in a checked program; null when not checked. */
  private readonly dynamicSpans: Set<string> | null;
  private readonly hostSpans: Set<string> | null;
  private readonly valueSpans: Set<string> | null;

  constructor(env: Env, source: Source, code: (r: Run) => unknown, slotCount: number, missing: Set<string>, dynamicSpans: Set<string> | null = null, hostSpans: Set<string> | null = null, valueSpans: Set<string> | null = null) {
    this.dynamicSpans = dynamicSpans;
    this.hostSpans = hostSpans ?? (dynamicSpans === null ? null : new Set());
    this.valueSpans = valueSpans ?? (dynamicSpans === null ? null : new Set());
    this.env = env;
    this.source = source;
    this.code = code;
    this.slotCount = slotCount;
    this.missing = missing;
  }

  /** Run once. Never throws for a failure of the program. */
  execute(vars?: Vars, options: { signal?: AbortSignal } = {}): Outcome {
    const e = this.env.environment;
    let run: Run | undefined;
    try {
      run = new Run(
        { steps: e.limits.steps, allocation: e.limits.allocation, depth: e.limits.depth ?? DEFAULT_LIMITS.depth, deadlineMs: this.env.timeoutMs },
        this.slotCount,
        this.env.classifier,
        supplyOf(vars),
        options.signal,
      );
      const value = this.code(run);
      return { ok: true, value, usage: { steps: run.steps, allocated: run.units } };
    } catch (x) {
      const usage = { steps: run?.steps ?? 0, allocated: run?.units ?? 0 };
      let f: RunFailure;
      if (x instanceof RunFailure) f = x;
      else if (x instanceof RangeError && /call stack/.test(x.message)) f = new RunFailure("internal-error", undefined, "default");
      else f = new RunFailure("internal-error", undefined, "default", {}, { cause: x });
      return { ok: false, diagnostic: this.diagnostic(f), usage };
    }
  }

  /** Run once. Throws RisorError of phase `run`. */
  run(vars?: Vars, options: { signal?: AbortSignal } = {}): unknown {
    const o = this.execute(vars, options);
    if (!o.ok) throw new RisorError([o.diagnostic], o.usage);
    return o.value;
  }

  measure(vars?: Vars, options: { signal?: AbortSignal } = {}): { value: unknown; usage: Usage } {
    const o = this.execute(vars, options);
    if (!o.ok) throw new RisorError([o.diagnostic], o.usage);
    return { value: o.value, usage: o.usage };
  }

  private diagnostic(f: RunFailure): Diagnostic {
    const d: Diagnostic = { phase: "run", code: f.code, message: messageOf(f, this.env.classifier) };
    if (f.span !== undefined) d.span = f.span;
    let cls = uncheckedClass(f);
    // The class in a checked program (03 3.7.6, 05 18.5).
    if (this.dynamicSpans !== null) {
      if (f.code === "null-operand" || f.code === "wrong-kind" || f.code === "missing-field") {
        cls = f.span !== undefined && this.dynamicSpans.has(spanKey(f.span)) ? "dynamic" : "environment";
      } else if (f.code === "no-match") cls = "environment";
      // f04 5.1 item 4: `value` at the `value` site of its `as`, else `environment`.
      else if (f.code === "not-conforming") cls = f.span !== undefined && this.valueSpans!.has(spanKey(f.span)) ? "value" : "environment";
      // The mark `!` is a promise: an error is `host` only at a `host` site on the failing call (05 18.3, 18.5).
      else if (f.code === "function-error") cls = f.span !== undefined && this.hostSpans!.has(spanKey(f.span)) ? "host" : "environment";
    }
    if (cls !== undefined) d.class = cls;
    if (f.code === "failed") d.data = f.data;
    if (f.path !== undefined) d.path = [...f.path];
    d.trace = [...f.trace];
    if (f.cause !== undefined) (d as Diagnostic & { cause?: unknown }).cause = f.cause;
    return d;
  }
}

export { ABSENT_VAR };

/** Print a signature (f01 5.2). A type variable sorts after the opaque names. */
export function printSignature(sig: SigNode, opaqueNames: ReadonlySet<string>, named?: ReadonlyMap<string, Type>): string {
  const ty = (n: TypeNode) => printType(fromNode(n, { opaque: opaqueNames, named, variable: (v) => opaqueType("\uFFFF" + v) }));
  const params = sig.params.map((p) => {
    const t = p.type.t === "fn"
      ? `(${p.type.params.map((q) => `${q.name}: ${ty(q.type)}`).join(", ")}) => ${p.type.result === "unknown" ? "unknown" : ty(p.type.result)}`
      : ty(p.type);
    return `${p.rest ? "..." : ""}${p.name}${p.optional ? "?" : ""}: ${t}`;
  });
  let out = `(${params.join(", ")}) => ${ty(sig.result)}`;
  if (sig.canFail) out += " !";
  if (sig.bounds.length > 0) {
    out += " where " + [...sig.bounds].sort((a, b) => compareText(a.name, b.name)).map((b) => `${b.name}: ${b.bound}`).join(", ");
  }
  return out;
}

// ---- Package F2: parsed programs, templates and analysis -------------------------------

/** A parsed program (03 3.2): needs no environment, and may be checked, compiled and run in any. */
export class ParsedProgram {
  readonly source: Source;
  readonly program: import("../syntax/ast.ts").Program;
  constructor(source: Source, program: import("../syntax/ast.ts").Program) {
    this.source = source;
    this.program = program;
  }
  /** The text the spans point into: the whole program or template text (f02 3). */
  get text(): string {
    return this.source.slice(0, this.source.length);
  }
  /** The analysis of this program (f02 3). */
  analyze(): AnalysisReport {
    return reportOf(analyze(this.program, null), this.source);
  }
}

function parseInput(text: string | ParsedProgram): ReturnType<typeof parse> {
  if (typeof text === "string") return parse(text);
  return { ok: true, program: text.program, source: text.source };
}

/** Parse a program. Throws RisorError of phase `parse`. */
export function parseProgramText(text: string): ParsedProgram {
  const p = parse(text);
  if (!p.ok) throw new RisorError(p.diagnostics);
  return new ParsedProgram(p.source, p.program);
}

/** A parsed template (f02 2). */
export class Template {
  readonly source: Source;
  private readonly data: ParsedTemplateData;
  constructor(data: ParsedTemplateData) {
    this.data = data;
    this.source = data.source;
  }
  /** Exactly one part, with no text before or after it. */
  get single(): boolean {
    return this.data.single;
  }
  /** The segments: text runs and parts, in order (f02 2.4). */
  get segments(): Segment[] {
    return this.data.segments;
  }
  /** The program of the template in a mode (f02 2.5). */
  program(mode: TemplateMode): ParsedProgram {
    return new ParsedProgram(this.source, templateProgram(this.data, mode));
  }
  /** The program of one part alone, with spans in the whole text. */
  part(index: number): ParsedProgram {
    const s = this.data.segments.filter((x) => x.kind === "part")[index] as { expr: import("../syntax/ast.ts").Expr } | undefined;
    if (s === undefined) throw new RangeError("no such part");
    return new ParsedProgram(this.source, { kind: "Program", consts: [], result: s.expr, span: { start: s.expr.span.start, end: s.expr.span.end } });
  }
  /** The analysis of the template in a mode (f02 3.1 item 6). */
  analyze(mode: TemplateMode): AnalysisReport {
    return reportOf(analyze(templateProgram(this.data, mode), this.data.segments), this.source);
  }
}

/** Template parse (f02 2.1). Throws RisorError of phase `parse`. */
export function parseTemplate(text: string): Template {
  const r = parseTemplateText(text);
  if (!r.ok) throw new RisorError(r.diagnostics);
  return new Template(r.template);
}

/** A call the analysis reports (f02 3.6). Each argument is a parsed program of its own, or null for a lambda. */
export interface AnalysisCall {
  name: string;
  namespace: string | null;
  span: Span;
  args: { span: Span; program: ParsedProgram | null }[];
  lambda: boolean;
}

/** The report of an analysis (f02 3.3), in this binding's types. Spans are code-point offsets. */
export interface AnalysisReport {
  names: string[];
  /** A path step is a key, or a star of package F7 (f07 9.3). */
  refs: { name: string; path: (string | { star: true })[]; span: Span }[];
  calls: AnalysisCall[];
  literal: { value: unknown } | false;
  segments: ({ text: string; span: Span } | { part: Span; span: Span })[] | null;
}

function reportOf(r: Report, source: Source): AnalysisReport {
  return {
    names: r.names,
    refs: r.refs.map((x) => ({ name: x.name, path: x.path.map((p) => ("star" in p ? { star: true as const } : p.key)), span: x.span })),
    calls: r.calls.map((c) => ({
      name: c.name,
      namespace: c.namespace,
      span: c.span,
      args: c.args.map((a) => ({
        span: a.span,
        program: a.kind === "Lambda" ? null : new ParsedProgram(source, { kind: "Program", consts: [], result: a, span: { start: a.span.start, end: a.span.end } }),
      })),
      lambda: c.lambda,
    })),
    literal: r.literal === null ? false : r.literal,
    segments: r.segments === null ? null : r.segments.map((g) => (g.kind === "text" ? { text: g.value, span: g.span } : { part: g.expr.span, span: g.span })),
  };
}

/** The JSON form of a report (f02 3.3, 4.3, 4.8): spans as [line, column, line, column], fields in order. */
export function reportJSON(r: AnalysisReport, source: Source): string {
  const sp = (s: Span) => JSON.stringify(source.lineSpan(s));
  const lit = r.literal === false ? "false" : `{"value":${toJSON(r.literal.value)}}`;
  const parts = [
    `"names":[${r.names.map(jsonString).join(",")}]`,
    `"refs":[${r.refs.map((x) => `{"name":${jsonString(x.name)},"path":[${x.path.map((p) => (typeof p === "string" ? jsonString(p) : '{"star":true}')).join(",")}],"span":${sp(x.span)}}`).join(",")}]`,
    `"calls":[${r.calls
      .map((c) => `{"name":${jsonString(c.name)},"namespace":${c.namespace === null ? "null" : jsonString(c.namespace)},"span":${sp(c.span)},"args":[${c.args.map((a) => sp(a.span)).join(",")}],"lambda":${c.lambda}}`)
      .join(",")}]`,
    `"literal":${lit}`,
    `"segments":${r.segments === null ? "null" : `[${r.segments.map((g) => ("text" in g ? `{"text":${jsonString(g.text)},"span":${sp(g.span)}}` : `{"part":${sp(g.part)},"span":${sp(g.span)}}`)).join(",")}]`}`,
  ];
  return `{${parts.join(",")}}`;
}
