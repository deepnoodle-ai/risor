// The host API (03-host.md; 05 Part E, the TypeScript column).
//
//   const env = createEnv({ varRecord: "{order: {total: float}}", functions: { ... } });
//   const program = env.compile(parse("order.total * 2"));
//   program.run({ order: { total: 10 } });          // 20, or throws RisorError
//   program.measure(vars);                          // { value, usage }
//
// An environment and a program do not change once built, and a program may be
// run many times. Host data is read in place, never copied.
//
// `src/index.ts` exports the public surface. What this module exports beyond
// it (marked "Internal") serves the suite runner, the tools and the tests.

import type { Diagnostic as InnerDiagnostic } from "../syntax/diagnostic.ts";
import { parse as parseSource } from "../syntax/parser.ts";
import { Source } from "../syntax/source.ts";
import type { Position } from "../syntax/source.ts";
import { arity, parseSignature } from "../types/typetext.ts";
import { compileProgram, HostError } from "../eval/compile.ts";
import type { CallHandle, CompileEnv, HostImpl } from "../eval/compile.ts";
import { LIMIT_CODES, RunFailure, messageOf, uncheckedClass } from "../eval/failure.ts";
import { ABSENT_VAR, Run } from "../eval/run.ts";
import type { VarSource } from "../eval/run.ts";
import { staticRules } from "../eval/static.ts";
import type { Counts, StaticEnv } from "../eval/static.ts";
import { functionCounts, FUNCTIONS, GROUP_CONSTANTS, GROUP_FUNCTIONS } from "../library/registry.ts";
import "../library/groups.ts";
import "../library/math.ts";
import "../library/core.ts";
import { Classifier, DEFAULT_CLASSIFIER, jsonString, mapGet, mapKeys, readHost, sortKeys, truthy as truthyOf, BAD_NUMBER } from "../value/value.ts";
import { numberToText } from "../value/number.ts";
import { declare, DEFAULT_LIMITS } from "./env.ts";
import { check, spanKey } from "../types/check.ts";
import type { CheckEnv, CheckSig, Expected as CheckExpected, Site as InnerSite } from "../types/check.ts";
import { depth, fromNode, printType, opaque as opaqueType, SIZE_LIMIT, typeSize } from "../types/type.ts";
import type { Type as InnerType } from "../types/type.ts";
import { conformAllWalk, conformWalk, freeCtx } from "../eval/conform.ts";
import { SchemaError, typeOfSchema } from "../types/schema.ts";
import type { SchemaOption } from "../types/schema.ts";
import { compareText } from "../value/value.ts";
import { analyze } from "../syntax/analysis.ts";
import type { Report } from "../syntax/analysis.ts";
import { parseTemplateText, templateProgram } from "../syntax/template.ts";
import type { ParsedTemplateData, TemplateMode } from "../syntax/template.ts";
import type { Span as InnerSpan } from "../syntax/source.ts";
import type { SigNode, TypeNode } from "../types/typetext.ts";
import { parseType as parseTypeText, parseReceiver, parseEnvRecord } from "../types/typetext.ts";
import { METHODS } from "../library/registry.ts";
import type { Declarations, Environment, NamedTypeDecl } from "./env.ts";
import { declareError, failure, publicDiagnostic, RisorError } from "./error.ts";
import type { Diagnostic, Pos, Span, Usage } from "./error.ts";
import { innerType, Type, typeClassifier, typeOf } from "./type.ts";
import { formatProgram as formatProgramResult, formatTemplate as formatTemplateResult } from "../tools/format.ts";
import { complete as assistComplete, hover as assistHover } from "../tools/assist.ts";
import type { CompletionResult, HoverResult } from "../tools/assist.ts";
import { renderDetail as renderDetailOf, renderJSON as renderJSONOf, renderLine as renderLineOf } from "./render.ts";

export { HostError, RisorError };
export type { Diagnostic, Pos, Span, Usage, TemplateMode, SchemaOption };

/** The handle a host function is called with (03 3.5): it charges steps and allocation units. */
export type Call = CallHandle;

/** A completion (package F12, f12 3.1): the span it replaces and the candidates. */
export type Completion = CompletionResult;
/** A hover record (package F12, f12 3.13). */
export type Hover = HoverResult;
export type { Candidate } from "../tools/assist.ts";

export interface HostFunction {
  /** The signature in type text: "(s: string, n?: int) => string !". */
  type: string;
  /**
   * Synchronous. Throw an `Error` (a `HostError`, say) to return an error:
   * `function-error`. Any other thrown value is a crash: `function-panic`.
   */
  call(handle: Call, ...args: unknown[]): unknown;
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
  /** The deadline of one run, in milliseconds, or null for none. */
  deadlineMs?: number | null;
  depth?: number;
}

/** A named type (package F4, f04 3.1): by type text, by JSON Schema with F1's options, or by a `Type`. */
export type NamedType =
  | { name: string; type: string }
  | { name: string; schema: unknown; options?: SchemaOption[] }
  | { name: string; typeOf: Type };

/** The declarations of an environment (03 3.3.1; 05 Part E, E.3). */
export interface EnvOptions {
  version?: string;
  /** Variables, each by its own type text: `{age: "int"}`. */
  vars?: Record<string, string>;
  /** All variables as one record type in type text: `"{user: {age: int}}"`. */
  varRecord?: string;
  /** Variables declared by JSON Schema, each with its options (package F1, f01 2). */
  varSchemas?: Record<string, { schema: unknown; options?: SchemaOption[] }>;
  /** Variables, each by a `Type`. */
  varTypes?: Record<string, Type>;
  /** Host functions. Keys may have one namespace: "geo.distance". */
  functions?: Record<string, HostFunction>;
  /** Opaque types, by name. */
  opaque?: Record<string, OpaqueType>;
  /** Package F4: named types, in the order declared (f04 3.1). */
  namedTypes?: NamedType[];
  /** Package F6: the opt-in groups to turn on: `json`, `time`, `math`, `base64` (f06 2.1). */
  groups?: string[];
  /** Library entries the environment does not have (03 3.3.1). */
  without?: string[];
  /** The check classes rejected (05 17). */
  reject?: string[];
  /** Package F14: host facts; each lists variables of which at least one is not null in every run (f14 2). */
  someNotNull?: string[][];
  limits?: Limits;
}

/** An opaque type in a child environment: with no `test`, it adds methods to an inherited type (03 3.3.2). */
export interface ChildOpaqueType {
  test?(value: unknown): boolean;
  methods?: Record<string, HostFunction>;
}

/** The declarations of a child environment (03 3.3.2). */
export interface WithOptions extends Omit<EnvOptions, "opaque"> {
  opaque?: Record<string, ChildOpaqueType>;
}

/** Internal: what the runner and a child environment add to the declarations. */
export interface InternalEnvOptions extends EnvOptions {
  /** The opaque type names as a list, when it is given apart from `opaque` (a case file's `opaque`). */
  opaqueNames?: string[];
  /** The variables a child keeps from its parent (03 3.3.2). */
  inherited?: Declarations["inherited"];
  /** The named types a child keeps from its parent (f04 3.4). */
  inheritedTypes?: Declarations["inheritedTypes"];
  /** The host facts a child keeps from its parent (f14 2.6). */
  inheritedFacts?: string[][];
}

/** Internal: the declarations of a child environment as the runner gives them. */
export interface InternalWithOptions extends Omit<InternalEnvOptions, "opaque" | "inherited" | "inheritedTypes" | "inheritedFacts"> {
  opaque?: Record<string, ChildOpaqueType>;
}

/** The variables of a run: an object, a `Map`, or a function from a name to its value. */
export type Vars = { readonly [name: string]: unknown } | ReadonlyMap<string, unknown> | ((name: string) => unknown);

/** Internal: the outcome of one run, never thrown. */
export type Outcome =
  | { ok: true; value: unknown; usage: Usage }
  | { ok: false; diagnostic: InnerDiagnostic; usage: Usage };

/** The language's truthiness test (03 3.2.4). Not JavaScript's: [] and {} are falsy. */
export function truthy(value: unknown): boolean {
  return truthyOf(value);
}

/** The JSON text of a result (03 3.2.4). Throws RisorError on an opaque value or on nesting past 200. */
export function jsonText(value: unknown): string {
  return jsonTextOf(value, DEFAULT_CLASSIFIER);
}

/** Internal: `jsonText` with the opaque tests of an environment. */
export function jsonTextOf(value: unknown, cls: Classifier): string {
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

/** The host facts, or a declare error for a value that is not a list of lists of names. */
function hostFacts(v: unknown): string[][] | undefined {
  if (v === undefined) return undefined;
  const ok = Array.isArray(v) && v.every((m) => Array.isArray(m) && m.every((x) => typeof x === "string"));
  if (!ok) throw declareError("bad-name", "someNotNull is a list of host facts, each a list of variable names");
  return (v as string[][]).map((m) => [...m]);
}

function namedDecls(ts: NamedType[] | undefined): NamedTypeDecl[] | undefined {
  return ts?.map((t) => ("typeOf" in t ? { name: t.name, typeOf: innerType(t.typeOf) } : t));
}

/** Build an environment. Throws RisorError of phase `declare`. */
export function createEnv(options: EnvOptions = {}): Env {
  return buildEnv(options);
}

/** Internal: build an environment with the declarations only the runner and `with` give. */
export function buildEnv(options: InternalEnvOptions = {}): Env {
  const methods: Record<string, string> = {};
  for (const [t, o] of Object.entries(options.opaque ?? {})) {
    for (const [m, f] of Object.entries(o.methods ?? {})) methods[`${t}.${m}`] = f.type;
  }
  const functions: Record<string, string> = {};
  for (const [k, f] of Object.entries(options.functions ?? {})) functions[k] = f.type;
  const limits = options.limits ?? {};
  const t = limits.deadlineMs;
  if (t !== undefined && t !== null && (typeof t !== "number" || !Number.isFinite(t) || t < 0)) {
    throw declareError("bad-limit", "the deadline must be a number of milliseconds of 0 or more, or off");
  }
  const varTypes = options.varTypes === undefined ? undefined : Object.fromEntries(Object.entries(options.varTypes).map(([k, v]) => [k, innerType(v)]));
  const decls: Declarations = {
    version: options.version,
    env: options.varRecord,
    opaque: options.opaqueNames ?? Object.keys(options.opaque ?? {}),
    methods,
    functions,
    reject: options.reject,
    schemas: options.varSchemas,
    variableTexts: options.vars,
    varTypes,
    removed: options.without,
    groups: options.groups,
    inherited: options.inherited,
    types: namedDecls(options.namedTypes),
    inheritedTypes: options.inheritedTypes,
    facts: hostFacts(options.someNotNull),
    inheritedFacts: options.inheritedFacts,
    limits: {
      ...(limits.steps !== undefined ? { steps: limits.steps } : {}),
      ...(limits.allocation !== undefined ? { allocation: limits.allocation } : {}),
      ...(limits.depth !== undefined ? { depth: limits.depth } : {}),
    },
  };
  const removed = new Set(options.without ?? []);
  const d = declare(decls, new Set([...FUNCTIONS.keys()].filter((k) => !removed.has(k))));
  if (!d.ok) throw new RisorError(d.diagnostics.map((x) => publicDiagnostic(x, null)));
  return new Env(d.env, options, limits.deadlineMs ?? null);
}

/** The internal type of a JSON Schema with options. Throws RisorError `bad-type`. */
function schemaType(schema: unknown, options: SchemaOption[]): InnerType {
  try {
    return typeOfSchema(schema, options);
  } catch (e) {
    if (e instanceof SchemaError) throw declareError("bad-type", e.message);
    throw e;
  }
}

/** The type of a JSON Schema with options (package F1, f01 3, 4). Throws RisorError `bad-type`. */
export function typeFromSchema(schema: unknown, options: SchemaOption[] = []): Type {
  return typeOf(schemaType(schema, options));
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
  /** Package F14 (f14 9.1): each distinct host fact, its members in code point order; only when there is one. */
  facts?: { someNotNull: string[] }[];
}

/** The host facts of a description (f14 9.1): distinct, members sorted, lists in order name by name. */
function describedFacts(facts: string[][]): { someNotNull: string[] }[] {
  const lists = new Map<string, string[]>();
  for (const f of facts) {
    const sorted = [...new Set(f)].sort(compareText);
    lists.set(sorted.join("\u0000"), sorted);
  }
  const cmp = (a: string[], b: string[]) => {
    for (let i = 0; i < Math.min(a.length, b.length); i++) {
      const c = compareText(a[i], b[i]);
      if (c !== 0) return c;
    }
    return a.length - b.length;
  };
  return [...lists.values()].sort(cmp).map((someNotNull) => ({ someNotNull }));
}

/** The JSON text of a description (f01 5.3 item 5): keys in code point order, no white space. */
export function descriptionText(d: Description): string {
  return jsonText(d);
}

function noImplementation(): never {
  throw new Error("no implementation");
}

/** Internal: the outcome of a compile, never thrown for a problem of the program. */
export type CompileResult = { ok: true; program: Program } | { ok: false; diagnostics: InnerDiagnostic[]; source: Source };

/**
 * What the host expects of a result (03 3.2.3): type text, the word
 * "condition", a type by JSON Schema (package F10, f10 2.1), or a `Type`.
 */
export type Expected = string | { schema: unknown; options?: SchemaOption[] } | { type: Type };

export interface CompileOptions {
  /** Apply the type rules (05-types.md). Default true. The static rules always apply. */
  check?: boolean;
  /** The expected result (03 3.2.3; f10 2.1). */
  expected?: Expected;
}

/** What a load takes besides the environment and the text (package F10, f10 5.1). */
export interface LoadOptions {
  /** The type the value must have: type text, a JSON Schema, or a `Type`. */
  expected: string | { schema: unknown } | { type: Type };
  /** The variables of the run (03 3.4.1). */
  vars?: Vars;
  signal?: AbortSignal;
  /** The most failures step 4 reports (f10 5.2 item 1). Default 1; below 1 is read as 1. */
  max?: number;
  /** Make step 5, the decode into plain data (f10 6.6). Default true. */
  decode?: boolean;
}

/** Internal: the outcome of a load (f10 5.2 item 4). */
export type LoadOutcome<T = unknown> =
  | { ok: true; value: T; usage: Usage }
  | { ok: false; step: 0 | 1 | 2 | 3 | 4 | 5; diagnostics: InnerDiagnostic[]; usage?: Usage; source?: Source };

/** A load that failed (f10 5.2): the step, the diagnostics and the usage. */
export class LoadError extends RisorError {
  readonly step: number;
  constructor(step: number, diagnostics: Diagnostic[], usage?: Usage) {
    super(diagnostics, usage);
    this.step = step;
  }
}

/** Internal: the decode, its failure as data. */
export function tryDecode(value: unknown, cls: Classifier = DEFAULT_CLASSIFIER): { ok: true; value: unknown } | { ok: false; diagnostic: InnerDiagnostic } {
  class Stop {
    readonly d: InnerDiagnostic;
    constructor(d: InnerDiagnostic) {
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

/**
 * The decode into plain data (package F10, f10 6.1 and 6.6): a copy in which a
 * map is an object whose own properties are exactly its keys, and a list an
 * Array. An opaque value is not-conforming at its path; a host number that
 * cannot enter the language is number-range; nesting deeper than 200 is
 * depth-exceeded with no path. Not a run: nothing is charged. Throws
 * RisorError of phase `run`.
 */
export function decode(value: unknown): unknown {
  const d = tryDecode(value);
  if (!d.ok) throw failure([d.diagnostic], null);
  return d.value;
}

/** A place where a checked program can still fail on a value (05 18): its class and span. */
export interface Site {
  class: string;
  span: Span;
}

/** The result of a check (03 3.2.1; 05 Part E, E.4). */
export interface CheckResult {
  /** The type of the result. */
  type: Type;
  sites: Site[];
}

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

function sitesOf(sites: InnerSite[], source: Source): Site[] {
  return sites.map((x) => ({ class: x.class, span: source.lineSpan(x.span) }));
}

export class Env {
  /** Internal: the declarations, as declared. */
  readonly environment: Environment;
  private readonly compileEnv: CompileEnv;
  private readonly staticEnv: StaticEnv;
  /** Internal: the opaque tests. */
  readonly classifier: Classifier;
  /** Internal: the deadline of one run. */
  readonly deadlineMs: number | null;

  private readonly options: InternalEnvOptions;

  /** Internal. A host builds an environment with `createEnv`. */
  constructor(environment: Environment, options: InternalEnvOptions, deadlineMs: number | null) {
    this.environment = environment;
    this.options = options;
    this.deadlineMs = deadlineMs;
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
    const hostSig = (name: string, sig: SigNode, recv: TypeNode | null): CheckSig => ({
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
      facts: environment.facts,
    };
  }

  private readonly checkEnv: CheckEnv;

  /**
   * An environment from a description (f01 5.4). It has no implementation of
   * any function or method: compiling against it fails with `no-implementation`
   * while it declares one. Throws RisorError (`declare`) for an error of 03
   * 3.3.4, and a plain Error for a value that is not a description.
   */
  static fromDescription(description: unknown): Env {
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
    for (const k of Object.keys(d)) if (!keys.includes(k) && k !== "types" && k !== "groups" && k !== "facts") bad(`the key ${k}`);
    // f14 9.2: each entry of `facts` is an object {someNotNull: [names]}.
    let facts: string[][] | undefined;
    if ("facts" in d) {
      const fs = d.facts;
      if (!Array.isArray(fs) || !fs.every((x) => isObj(x) && Object.keys(x).length === 1 && Array.isArray(x.someNotNull) && (x.someNotNull as unknown[]).every((n) => typeof n === "string"))) bad("facts");
      facts = (fs as { someNotNull: string[] }[]).map((x) => [...x.someNotNull]);
    }
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
      vars: variables,
      opaque,
      functions: Object.fromEntries(Object.entries(functions).map(([k, sig]) => [k, { type: sig, call: noImplementation }])),
      without: removed,
      reject,
      namedTypes: types,
      groups,
      someNotNull: facts,
      limits: {
        steps: num(lim.steps, true, "steps"),
        allocation: num(lim.allocation, true, "allocation"),
        depth: num(lim.depth, false, "depth") as number,
      },
    });
    env.noImplementation = true;
    return env;
  }

  /** Internal: the expected result as the checker takes it. Throws RisorError `bad-type` for bad text or a bad schema. */
  expected(text: Expected | undefined): CheckExpected {
    if (text === undefined) return { kind: "none" };
    if (text === "condition") return { kind: "condition" };
    const fail = (m: string): never => {
      throw declareError("bad-type", `the expected result: ${m}`);
    };
    if (typeof text !== "string" && "type" in text) {
      const t = innerType(text.type);
      if (depth(t, 70) > 64) fail("a type deeper than 64");
      if (typeSize(t) > SIZE_LIMIT) fail("a type larger than 100,000");
      return { kind: "type", type: t };
    }
    if (typeof text !== "string") {
      // f10 2.1: only `closed` is accepted; the schema's errors and the limits of F1 3.10 are bad-type.
      const opts = text.options ?? [];
      for (const o of opts) if (o !== "closed") fail(`the option ${o} states what a host did to a value it supplied, and nobody supplies a result`);
      let t: InnerType;
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
      return { kind: "type", type: fromNode((t as { node: TypeNode }).node, { opaque: this.environment.opaque, named: this.environment.named }) };
    } catch (e) {
      return fail((e as Error).message);
    }
  }

  /** Internal: the type of an expected result that is a type (f10 5.2 step 0). Throws RisorError `bad-type`. */
  expectedType(e: Expected): InnerType {
    const x = this.expected(e);
    if (x.kind !== "type") throw declareError("bad-type", "the expected result is not a type");
    return x.type;
  }

  /** Internal: type text read against this environment's opaque and named types. Throws on bad text. */
  typeOfText(text: string): InnerType {
    const t = parseTypeText(text);
    if (!t.ok) throw new Error(t.message);
    return fromNode(t.node, { opaque: this.environment.opaque, named: this.environment.named });
  }

  /** Internal: set on an environment built from a description: it has no implementation (f01 5.4 item 4). */
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
      ...(e.facts.length > 0 ? { facts: describedFacts(e.facts) } : {}),
    };
  }

  /** The names of the library entries this environment has: functions, the entries and constants of groups that are on, and methods as `kind.name`; in code point order. */
  libraryNames(): string[] {
    const e = this.environment;
    const out: string[] = [];
    for (const k of FUNCTIONS.keys()) if (!e.removed.has(k)) out.push(k);
    for (const [k, f] of GROUP_FUNCTIONS) if (e.groups.has(f.group) && !e.removed.has(k)) out.push(k);
    for (const [k, c] of GROUP_CONSTANTS) if (e.groups.has(c.group) && !e.removed.has(k)) out.push(k);
    for (const [kind, m] of METHODS) for (const k of m.keys()) if (!e.removed.has(`${kind}.${k}`)) out.push(`${kind}.${k}`);
    return out.sort(compareText);
  }

  /**
   * A type from type text, read against this environment's opaque and named
   * types (05 19.5, f04 7.1 item 4). Throws RisorError `bad-type`.
   */
  parseType(text: string): Type {
    return typeOf(this.declaredType(text), this.classifier);
  }

  /** Internal: type text resolved as declared type text. Throws RisorError `bad-type`. */
  declaredType(text: string): InnerType {
    const fail = (m: string): never => {
      throw declareError("bad-type", `the type: ${m}`);
    };
    const p = parseTypeText(text);
    if (!p.ok) return fail(p.message);
    let t: InnerType;
    try {
      t = fromNode(p.node, { opaque: this.environment.opaque, named: this.environment.named });
    } catch (e) {
      return fail((e as Error).message);
    }
    if (depth(t, 70) > 64) fail("a type deeper than 64");
    if (typeSize(t) > SIZE_LIMIT) fail("a type larger than 100,000");
    return t;
  }

  /**
   * Check a program (03 3.2.1): the static rules and the type rules. Throws
   * RisorError of phase `declare` (a bad expected result) or `check`.
   */
  check(x: Expr, expected?: Expected): CheckResult {
    const exp = this.expected(expected);
    const s = staticRules(x.program, this.staticEnv, x.source);
    const c = check(x.program, s.resolution, this.checkEnv, exp, s.diagnostics, x.source);
    if (c.diagnostics.length > 0) throw failure(c.diagnostics, x.source);
    return { type: typeOf(c.type, this.classifier), sites: sitesOf(c.sites, x.source) };
  }

  /**
   * Internal. Package F12: the static rules and the check of a program the
   * assist stage repaired (f12 3.3 item 4), with the types of its nodes. Its
   * diagnostics are not reported.
   */
  assistCheck(program: import("../syntax/ast.ts").Program, source: Source, expected?: string): { resolution: import("../eval/static.ts").Resolution; check: import("../types/check.ts").CheckResult } {
    let exp: CheckExpected = { kind: "none" };
    try {
      exp = this.expected(expected);
    } catch {
      exp = { kind: "none" };
    }
    const s = staticRules(program, this.staticEnv, source);
    const c = check(program, s.resolution, this.checkEnv, exp, s.diagnostics, source);
    return { resolution: s.resolution, check: c };
  }

  /**
   * A child environment (03 3.3.2). A child's declaration replaces the parent's
   * declaration of the same name in the same space: variables and host
   * functions are one space, opaque types another (03 3.3.4 item 1). An opaque
   * type the child declares (with `test`) replaces the parent's whole; one given
   * with methods only adds or replaces those methods. The clash rules apply to
   * the child as it stands. Rejected classes and limits are the parent's unless
   * given; entries removed add up. Throws RisorError of phase `declare`.
   */
  with(options: WithOptions = {}): Env {
    return childEnv(this, options);
  }

  /** Internal: the parent's declarations, as given. */
  get declared(): InternalEnvOptions {
    return this.options;
  }

  /**
   * Compile a program, with a check unless `check: false`. Throws RisorError
   * of phase `declare` or `check`.
   */
  compile(x: Expr, options: CompileOptions = {}): Program {
    const r = tryCompile(this, x, options);
    if (!r.ok) throw failure(r.diagnostics, r.source);
    return r.program;
  }

  /** Completion at a position of a text that need not parse (package F12, f12 3.1). */
  complete(text: string, pos: Pos, expected?: Expected): Completion {
    return assistComplete(this, text, pos as Position, this.assistExpected(expected));
  }

  /** The hover record at a position of a text that need not parse (package F12, f12 3.13), or null. */
  hover(text: string, pos: Pos, expected?: Expected): Hover | null {
    return assistHover(this, text, pos as Position, this.assistExpected(expected));
  }

  /** The expected result as the assist stage reads it: type text or "condition". */
  private assistExpected(e: Expected | undefined): string | undefined {
    if (e === undefined || typeof e === "string") return e;
    try {
      const x = this.expected(e);
      return x.kind === "type" ? printType(x.type) : undefined;
    } catch {
      return undefined;
    }
  }
}

/** Internal: a child environment, with the declarations the runner gives. */
export function childEnv(parent: Env, options: InternalWithOptions = {}): Env {
  const p = parent.declared;
  const pe = parent.environment;
  const valueNames = new Set<string>([...Object.keys(options.vars ?? {}), ...Object.keys(options.varSchemas ?? {}), ...Object.keys(options.varTypes ?? {}), ...Object.keys(options.functions ?? {})]);
  if (options.varRecord !== undefined) {
    const t = parseEnvRecord(options.varRecord);
    if (t.ok && t.node.t === "record") for (const f of t.node.fields) valueNames.add(f.key);
  }
  const inherited = new Map<string, { node: TypeNode; type: InnerType | undefined }>();
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
  const childNamed = new Set((options.namedTypes ?? []).map((t) => t.name));
  const inheritedTypes = new Map([...pe.named].filter(([k]) => !childNamed.has(k)));
  // f14 2.6: the parent's host facts, less those that name a variable the child replaces.
  const inheritedFacts = pe.facts.filter((m) => !m.some((n) => valueNames.has(n)));
  const env = buildEnv({
    someNotNull: options.someNotNull,
    inheritedFacts,
    version: options.version ?? p.version,
    varRecord: options.varRecord,
    vars: options.vars,
    varSchemas: options.varSchemas,
    varTypes: options.varTypes,
    inherited,
    namedTypes: options.namedTypes,
    inheritedTypes,
    functions,
    opaque,
    opaqueNames,
    reject: options.reject ?? (p.reject !== undefined ? p.reject : [...pe.reject]),
    limits: options.limits ?? p.limits,
    without: [...pe.removed, ...(options.without ?? [])],
    // f06 2.6 item 1: the parent's groups and the child's own.
    groups: [...pe.groups, ...(options.groups ?? [])],
  });
  env.noImplementation = parent.noImplementation;
  return env;
}

/** Internal: parse when given text, then apply the static rules and compile. Problems are data. */
export function tryCompile(env: Env, input: string | Expr, options: CompileOptions = {}): CompileResult {
  if (env.noImplementation && (env.environment.functions.size > 0 || env.environment.methods.size > 0)) {
    throw declareError("no-implementation", "the environment declares a function with no implementation");
  }
  const checked = options.check !== false;
  // f10 2.1 item 6: a compile without a check still maps a schema and reports its errors.
  if (!checked && options.expected !== undefined && typeof options.expected !== "string") env.expected(options.expected);
  const exp = checked ? env.expected(options.expected) : { kind: "none" as const };
  let x: Expr;
  if (typeof input === "string") {
    const p = parseSource(input);
    if (!p.ok) return { ok: false, diagnostics: p.diagnostics, source: p.source };
    x = new Expr(p.source, p.program);
  } else x = input;
  const internals = env as unknown as { staticEnv: StaticEnv; checkEnv: CheckEnv; compileEnv: CompileEnv };
  const s = staticRules(x.program, internals.staticEnv, x.source);
  let dynamicSpans: Set<string> | null = null;
  let hostSpans: Set<string> | null = null;
  let valueSpans: Set<string> | null = null;
  let result: CheckResult | null = null;
  if (checked) {
    const c = check(x.program, s.resolution, internals.checkEnv, exp, s.diagnostics, x.source);
    if (c.diagnostics.length > 0) return { ok: false, diagnostics: c.diagnostics, source: x.source };
    dynamicSpans = c.dynamicSpans;
    hostSpans = c.hostSpans;
    valueSpans = c.valueSpans;
    result = { type: typeOf(c.type, env.classifier), sites: sitesOf(c.sites, x.source) };
  } else if (s.diagnostics.length > 0) return { ok: false, diagnostics: s.diagnostics, source: x.source };
  const c = compileProgram(x.program, s.resolution, internals.compileEnv);
  const prog = new Program(env, x.source, c.run, s.resolution.slotCount, c.missing, dynamicSpans, hostSpans, valueSpans, c.varCount);
  prog.checkResult = result;
  prog.whole = x.program.span;
  return { ok: true, program: prog };
}

export class Program {
  /** Internal. */
  readonly env: Env;
  /** Internal: the source the spans point into. */
  readonly source: Source;
  private readonly code: (r: Run) => unknown;
  private readonly slotCount: number;
  /** Internal: library entries the program names that this build does not implement yet. */
  readonly missing: ReadonlySet<string>;

  /** The result of the check, or null when the program was compiled with `check: false`. */
  checkResult: CheckResult | null = null;
  /** Internal: the span of the whole program, the span of a limit diagnostic raised where no node is at hand (03 3.7.1). */
  whole: InnerSpan = { start: 0, end: 0 };
  /** Spans at a `dynamic` site, for the class of a failure in a checked program; null when not checked. */
  private readonly dynamicSpans: Set<string> | null;
  private readonly hostSpans: Set<string> | null;
  private readonly valueSpans: Set<string> | null;

  private readonly varCount: number;

  /** Internal. A host compiles a program with `env.compile`. */
  constructor(env: Env, source: Source, code: (r: Run) => unknown, slotCount: number, missing: Set<string>, dynamicSpans: Set<string> | null = null, hostSpans: Set<string> | null = null, valueSpans: Set<string> | null = null, varCount = 0) {
    this.varCount = varCount;
    this.dynamicSpans = dynamicSpans;
    this.hostSpans = hostSpans ?? (dynamicSpans === null ? null : new Set());
    this.valueSpans = valueSpans ?? (dynamicSpans === null ? null : new Set());
    this.env = env;
    this.source = source;
    this.code = code;
    this.slotCount = slotCount;
    this.missing = missing;
  }

  private newRun(vars: Vars | undefined, signal: AbortSignal | undefined): Run {
    const e = this.env.environment;
    return new Run(
      { steps: e.limits.steps, allocation: e.limits.allocation, depth: e.limits.depth ?? DEFAULT_LIMITS.depth, deadlineMs: this.env.deadlineMs },
      this.slotCount,
      this.env.classifier,
      vars as VarSource,
      this.varCount,
      signal,
    );
  }

  private failureOf(x: unknown): RunFailure {
    if (x instanceof RunFailure) return x;
    if (x instanceof RangeError && /call stack/.test(x.message)) return new RunFailure("internal-error", undefined, "default");
    return new RunFailure("internal-error", undefined, "default", {}, { cause: x });
  }

  private execute(vars?: Vars, options: { signal?: AbortSignal } = {}): Outcome {
    let run: Run | undefined;
    try {
      run = this.newRun(vars, options.signal);
      const value = this.code(run);
      return { ok: true, value, usage: { steps: run.steps, allocated: run.units } };
    } catch (x) {
      const usage = { steps: run?.steps ?? 0, allocated: run?.units ?? 0 };
      return { ok: false, diagnostic: this.diagnostic(this.failureOf(x)), usage };
    }
  }

  /** Run once. Throws RisorError of phase `run`, with the usage. */
  run(vars?: Vars, options?: { signal?: AbortSignal }): unknown {
    let run: Run | undefined;
    try {
      run = this.newRun(vars, options === undefined ? undefined : options.signal);
      return this.code(run);
    } catch (x) {
      const usage = { steps: run?.steps ?? 0, allocated: run?.units ?? 0 };
      throw failure([this.diagnostic(this.failureOf(x))], this.source, usage);
    }
  }

  /** Run once and report the usage. Throws RisorError of phase `run`, whose `usage` is the usage of the failed run. */
  measure(vars?: Vars, options: { signal?: AbortSignal } = {}): { value: unknown; usage: Usage } {
    const o = execute(this, vars, options);
    if (!o.ok) throw failure([o.diagnostic], this.source, o.usage);
    return { value: o.value, usage: o.usage };
  }

  private diagnostic(f: RunFailure): InnerDiagnostic {
    const d: InnerDiagnostic = { phase: "run", code: f.code, message: messageOf(f, this.env.classifier) };
    // A limit diagnostic has a span whose value is not specified (03 3.7.1).
    if (f.span !== undefined) d.span = f.span;
    else if (LIMIT_CODES.has(f.code)) d.span = this.whole;
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
    if (f.cause !== undefined) (d as InnerDiagnostic & { cause?: unknown }).cause = f.cause;
    return d;
  }
}

/** Internal: run once. Never throws for a failure of the program. */
export function execute(p: Program, vars?: Vars, options: { signal?: AbortSignal } = {}): Outcome {
  return p["execute"](vars, options);
}

export { ABSENT_VAR };

/** Internal: print a signature (f01 5.2). A type variable sorts after the opaque names. */
export function printSignature(sig: SigNode, opaqueNames: ReadonlySet<string>, named?: ReadonlyMap<string, InnerType>): string {
  const ty = (n: TypeNode) => printType(fromNode(n, { opaque: opaqueNames, named, variable: (v) => opaqueType("￿" + v) }));
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

// ---- Conformance (package F4, f04 7; F8, f08 6.1) --------------------------------------

function conformDiagnostic(f: { template: string; params: Record<string, unknown>; path: (string | number)[] }, cls: Classifier): Diagnostic {
  const failure = new RunFailure("not-conforming", undefined, f.template, f.params as never);
  return { phase: "run", code: "not-conforming", class: "value", path: f.path, message: messageOf(failure, cls), trace: [] };
}

/**
 * The conformance function (package F4, f04 7): null when the value conforms
 * to the type, else the diagnostic, with no span. Not a run: nothing is
 * charged and no limit applies. An opaque value is told by the tests of the
 * environment the type was read in (`env.parseType`).
 */
export function conform(value: unknown, t: Type): Diagnostic | null {
  const cls = typeClassifier(t);
  const f = conformWalk(value, innerType(t), freeCtx(cls));
  return f === null ? null : conformDiagnostic(f as never, cls);
}

/**
 * The conformance function that reports every failure, up to `max` (package
 * F8, f08 6.1). A maximum below 1 is read as 1. Nothing is charged.
 */
export function conformAll(value: unknown, t: Type, max: number): Diagnostic[] {
  const cls = typeClassifier(t);
  return conformAllWalk(value, innerType(t), cls, Math.max(1, Math.floor(max))).map((f) => conformDiagnostic(f as never, cls));
}

// ---- Loading a config file (package F10, f10 5) ----------------------------------------

/**
 * Internal: a load (package F10, f10 5.2): declare T, parse, check and compile
 * with T as the expected result, run, test the value against T by the
 * conformance function that reports every failure, and decode it into plain
 * data. A schema is mapped with `closed` and no other option (f10 5.6). Never
 * throws for a failure of a step: the outcome gives the step and its
 * diagnostics.
 */
export function tryLoad<T = unknown>(env: Env, text: string, options: LoadOptions): LoadOutcome<T> {
  const expected = options.expected;
  const exp: Expected = typeof expected === "string" || "type" in expected ? expected : { schema: expected.schema, options: ["closed"] };
  // Step 0: declare.
  let type: InnerType;
  try {
    if (expected === "condition") throw declareError("bad-type", "the expected result of a load is a type");
    type = env.expectedType(exp);
  } catch (e) {
    if (e instanceof RisorError) return { ok: false, step: 0, diagnostics: e.diagnostics.map((d) => ({ phase: d.phase, code: d.code, message: d.message })) };
    throw e;
  }
  // Steps 1 and 2: parse, then check and compile. A load never compiles without the check.
  let compiled: CompileResult;
  try {
    compiled = tryCompile(env, text, { expected: exp });
  } catch (e) {
    if (e instanceof RisorError) return { ok: false, step: 0, diagnostics: e.diagnostics.map((d) => ({ phase: d.phase, code: d.code, message: d.message })) };
    throw e;
  }
  if (!compiled.ok) return { ok: false, step: compiled.diagnostics[0].phase === "parse" ? 1 : 2, diagnostics: compiled.diagnostics, source: compiled.source };
  // Step 3: run.
  const o = execute(compiled.program, options.vars, { signal: options.signal });
  if (!o.ok) return { ok: false, step: 3, diagnostics: [o.diagnostic], usage: o.usage, source: compiled.program.source };
  // Step 4: conform, every failure up to the maximum.
  const fs = conformAllWalk(o.value, type, env.classifier, Math.max(1, Math.floor(options.max ?? 1)));
  if (fs.length > 0) {
    const diagnostics = fs.map((f): InnerDiagnostic => ({ phase: "run", code: "not-conforming", class: "value", path: f.path, message: messageOf(new RunFailure("not-conforming", undefined, f.template, f.params), env.classifier) }));
    return { ok: false, step: 4, diagnostics, usage: o.usage };
  }
  // Step 5: the decode into plain data.
  if (options.decode === false) return { ok: true, value: o.value as T, usage: o.usage };
  const d = tryDecode(o.value, env.classifier);
  if (!d.ok) return { ok: false, step: 5, diagnostics: [d.diagnostic], usage: o.usage };
  return { ok: true, value: d.value as T, usage: o.usage };
}

/** Load a config file into a value (package F10, f10 5.2). Throws LoadError with the step that failed. */
export function loadValue<T = unknown>(env: Env, text: string, options: LoadOptions): T {
  const r = tryLoad<T>(env, text, options);
  if (!r.ok) throw new LoadError(r.step, r.diagnostics.map((d) => publicDiagnostic(d, r.source)), r.usage);
  return r.value;
}

// ---- Package F2: parsed programs, templates and analysis -------------------------------

/** A parsed program (03 3.2): needs no environment, and may be checked, compiled and run in any. */
export class Expr {
  /** Internal: the source the spans point into. */
  readonly source: Source;
  /** Internal: the syntax tree. */
  readonly program: import("../syntax/ast.ts").Program;
  /** Internal. A host parses with `parse`. */
  constructor(source: Source, program: import("../syntax/ast.ts").Program) {
    this.source = source;
    this.program = program;
  }
  /** The text the spans point into: the whole program or template text (f02 3). */
  get text(): string {
    return this.source.slice(0, this.source.length);
  }
  /** The analysis of this program (f02 3). */
  analyze(): Analysis {
    return new Analysis(analyze(this.program, null), this.source);
  }
}

/** Parse a program. Throws RisorError of phase `parse`. */
export function parse(text: string): Expr {
  const p = parseSource(text);
  if (!p.ok) throw failure(p.diagnostics, p.source);
  return new Expr(p.source, p.program);
}

/** A segment of a template (f02 2.4): a run of text, or a part. */
export type Segment = { kind: "text"; value: string; span: Span } | { kind: "part"; span: Span };

/** A parsed template (f02 2). */
export class Template {
  /** Internal. */
  readonly source: Source;
  private readonly data: ParsedTemplateData;
  /** Internal. A host parses with `parseTemplate`. */
  constructor(data: ParsedTemplateData) {
    this.data = data;
    this.source = data.source;
  }
  /** Exactly one part, with no text before or after it. */
  get isSingle(): boolean {
    return this.data.single;
  }
  /** The segments: text runs and parts, in order (f02 2.4). */
  get segments(): Segment[] {
    return this.data.segments.map((s) => (s.kind === "text" ? { kind: "text", value: s.value, span: this.source.lineSpan(s.span) } : { kind: "part", span: this.source.lineSpan(s.span) }));
  }
  /** The program of the template in a mode (f02 2.5). */
  program(mode: TemplateMode): Expr {
    return new Expr(this.source, templateProgram(this.data, mode));
  }
  /** The expression of part `index` alone as a program, with spans in the whole text. Parts are counted from 0, without the text runs. Undefined when there is no such part. */
  part(index: number): Expr | undefined {
    const s = this.data.segments.filter((x) => x.kind === "part")[index] as { expr: import("../syntax/ast.ts").Expr } | undefined;
    if (s === undefined) return undefined;
    return new Expr(this.source, { kind: "Program", consts: [], result: s.expr, span: { start: s.expr.span.start, end: s.expr.span.end } });
  }
  /** The analysis of the template in a mode (f02 3.1 item 6). */
  analyze(mode: TemplateMode): Analysis {
    return new Analysis(analyze(templateProgram(this.data, mode), this.data.segments), this.source);
  }
}

/** Template parse (f02 2.1). Throws RisorError of phase `parse`. */
export function parseTemplate(text: string): Template {
  const r = parseTemplateText(text);
  if (!r.ok) throw failure(r.diagnostics, r.source);
  return new Template(r.template);
}

/** A call the analysis reports (f02 3.6). Each argument is a parsed program of its own, or null for a lambda. */
export interface AnalysisCall {
  name: string;
  namespace: string | null;
  span: Span;
  args: { span: Span; program: Expr | null }[];
  lambda: boolean;
}

/** A name the program reads, and the path it reads through it (f02 3.4). A path step is a key, or a star of package F7 (f07 9.3). */
export interface Ref {
  name: string;
  path: (string | { star: true })[];
  span: Span;
}

/** The report of an analysis (f02 3.3). */
export class Analysis {
  readonly names: string[];
  readonly refs: Ref[];
  readonly calls: AnalysisCall[];
  readonly literal: { value: unknown } | false;
  readonly segments: ({ text: string; span: Span } | { part: Span; span: Span })[] | null;

  /** Internal. */
  constructor(r: Report, source: Source) {
    const sp = (s: InnerSpan) => source.lineSpan(s);
    this.names = r.names;
    this.refs = r.refs.map((x) => ({ name: x.name, path: x.path.map((p) => ("star" in p ? { star: true as const } : p.key)), span: sp(x.span) }));
    this.calls = r.calls.map((c) => ({
      name: c.name,
      namespace: c.namespace,
      span: sp(c.span),
      args: c.args.map((a) => ({
        span: sp(a.span),
        program: a.kind === "Lambda" ? null : new Expr(source, { kind: "Program", consts: [], result: a, span: { start: a.span.start, end: a.span.end } }),
      })),
      lambda: c.lambda,
    }));
    this.literal = r.literal === null ? false : r.literal;
    this.segments = r.segments === null ? null : r.segments.map((g) => (g.kind === "text" ? { text: g.value, span: sp(g.span) } : { part: sp(g.expr.span), span: sp(g.span) }));
  }

  /** The JSON form of the report (f02 3.3, 4.3, 4.8): spans as [line, column, line, column], fields in order. */
  json(): string {
    const sp = (s: Span) => JSON.stringify(s);
    const lit = this.literal === false ? "false" : `{"value":${jsonText(this.literal.value)}}`;
    const parts = [
      `"names":[${this.names.map(jsonString).join(",")}]`,
      `"refs":[${this.refs.map((x) => `{"name":${jsonString(x.name)},"path":[${x.path.map((p) => (typeof p === "string" ? jsonString(p) : '{"star":true}')).join(",")}],"span":${sp(x.span)}}`).join(",")}]`,
      `"calls":[${this.calls
        .map((c) => `{"name":${jsonString(c.name)},"namespace":${c.namespace === null ? "null" : jsonString(c.namespace)},"span":${sp(c.span)},"args":[${c.args.map((a) => sp(a.span)).join(",")}],"lambda":${c.lambda}}`)
        .join(",")}]`,
      `"literal":${lit}`,
      `"segments":${this.segments === null ? "null" : `[${this.segments.map((g) => ("text" in g ? `{"text":${jsonString(g.text)},"span":${sp(g.span)}}` : `{"part":${sp(g.part)},"span":${sp(g.span)}}`)).join(",")}]`}`,
    ];
    return `{${parts.join(",")}}`;
  }
}

// ---- Package F12: the formatter (f12 2) ------------------------------------------------

/** The canonical text of a program (f12 2.1). Throws RisorError of phase `parse`. */
export function format(text: string): string {
  const r = formatProgramResult(text);
  if (!r.ok) throw failure(r.diagnostics, new Source(text));
  return r.text;
}

/** The canonical text of a template (f12 2.14). Throws RisorError of phase `parse`. */
export function formatTemplate(text: string): string {
  const r = formatTemplateResult(text);
  if (!r.ok) throw failure(r.diagnostics, new Source(text, { keepBom: true }));
  return r.text;
}

// ---- Package F8: the renderings of diagnostics (f08 5) ---------------------------------

/** The one-line form (f08 5.2): one line for each diagnostic. */
export function renderLine(ds: readonly Diagnostic[]): string {
  return renderLineOf(ds);
}

/** The detail form (f08 5.3): each line with an excerpt of the source and the calls in progress. */
export function renderDetail(ds: readonly Diagnostic[], source: string | null): string {
  return renderDetailOf(ds, source);
}

/** The JSON form (f08 5.5): one JSON object a line, keys in code point order. */
export function renderJSON(ds: readonly Diagnostic[]): string {
  return renderJSONOf(ds);
}
