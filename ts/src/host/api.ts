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
import { functionCounts, FUNCTIONS } from "../library/registry.ts";
import "../library/core.ts";
import { Classifier, DEFAULT_CLASSIFIER, jsonString, mapGet, mapKeys, readHost, sortKeys, truthy as truthyOf, BAD_NUMBER } from "../value/value.ts";
import { numberToText } from "../value/number.ts";
import { declare, DEFAULT_LIMITS } from "./env.ts";
import { check, spanKey } from "../types/check.ts";
import type { CheckEnv, CheckSig, Expected, Site } from "../types/check.ts";
import { fromNode, printType } from "../types/type.ts";
import type { Type } from "../types/type.ts";
import { parseType as parseTypeText, parseReceiver } from "../types/typetext.ts";
import { METHODS } from "../library/registry.ts";
import type { Declarations, Environment } from "./env.ts";

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
  return { min: a.min, max: a.max, fns: s.node.params.map(() => null), rest: a.max === Infinity };
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
    limits: {
      ...(limits.steps !== undefined ? { steps: limits.steps } : {}),
      ...(limits.allocation !== undefined ? { allocation: limits.allocation } : {}),
      ...(limits.depth !== undefined ? { depth: limits.depth } : {}),
    },
  };
  const d = declare(decls, new Set(FUNCTIONS.keys()));
  if (!d.ok) throw new RisorError(d.diagnostics);
  return new Env(d.env, options, limits.timeoutMs ?? null);
}

export type CompileResult = { ok: true; program: Program } | { ok: false; diagnostics: Diagnostic[]; source: Source };

export interface CompileOptions {
  /** Apply the type rules (05-types.md). Default true. The static rules always apply. */
  check?: boolean;
  /** The expected result (03 3.2.3): type text, or "condition". */
  expected?: string;
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
const LIBRARY_METHODS = new Map([...METHODS].map(([kind, m]) => [kind, new Map([...m].map(([k, e]) => [k, librarySig(e, "library")]))]));

export class Env {
  readonly environment: Environment;
  private readonly compileEnv: CompileEnv;
  private readonly staticEnv: StaticEnv;
  readonly classifier: Classifier;
  readonly timeoutMs: number | null;

  constructor(environment: Environment, options: EnvOptions, timeoutMs: number | null) {
    this.environment = environment;
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
    this.compileEnv = { functions: fns, methods };
    const hostCounts = new Map<string, Counts>();
    for (const [k, f] of fns) hostCounts.set(k, f.counts);
    this.staticEnv = {
      variables: new Set(environment.variables.keys()),
      hostFunctions: hostCounts,
      namespaces: environment.namespaces,
      library: functionCounts(),
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
      variables: environment.variableTypes,
      functions: new Map([...environment.functions].map(([k, fn]) => [k, hostSig(k, fn.sig, null)])),
      library: LIBRARY_FUNCTIONS,
      libraryMethods: LIBRARY_METHODS,
      methods: methodSigs,
      reject: environment.reject,
    };
  }

  private readonly checkEnv: CheckEnv;

  /** The expected result as the checker takes it. Throws RisorError `bad-type` for bad text. */
  private expected(text: string | undefined): Expected {
    if (text === undefined) return { kind: "none" };
    if (text === "condition") return { kind: "condition" };
    const fail = (m: string): never => {
      throw new RisorError([{ phase: "declare", code: "bad-type", message: `the expected result: ${m}` }]);
    };
    const t = parseTypeText(text);
    if (!t.ok) fail(t.message);
    try {
      return { kind: "type", type: fromNode((t as { node: import("../types/typetext.ts").TypeNode }).node, { opaque: this.environment.opaque }) };
    } catch (e) {
      return fail((e as Error).message);
    }
  }

  /** Read type text against this environment's opaque types. Throws on bad text. */
  typeOf(text: string): Type {
    const t = parseTypeText(text);
    if (!t.ok) throw new Error(t.message);
    return fromNode(t.node, { opaque: this.environment.opaque });
  }

  /** Check a program (03 3.2.1): parse, the static rules, the type rules. */
  check(text: string, expected?: string): CheckOutcome {
    const exp = this.expected(expected);
    const p = parse(text);
    if (!p.ok) return { ok: false, diagnostics: p.diagnostics, source: p.source };
    const s = staticRules(p.program, this.staticEnv);
    const c = check(p.program, s.resolution, this.checkEnv, exp, s.diagnostics);
    if (c.diagnostics.length > 0) return { ok: false, diagnostics: c.diagnostics, source: p.source };
    return { ok: true, type: printType(c.type), sites: c.sites, source: p.source };
  }

  /** Parse, apply the static rules and compile. Problems are data. */
  tryCompile(text: string, options: CompileOptions = {}): CompileResult {
    const checked = options.check !== false;
    const exp = checked ? this.expected(options.expected) : { kind: "none" as const };
    const p = parse(text);
    if (!p.ok) return { ok: false, diagnostics: p.diagnostics, source: p.source };
    const s = staticRules(p.program, this.staticEnv);
    let dynamicSpans: Set<string> | null = null;
    let type: string | null = null;
    let sites: Site[] = [];
    if (checked) {
      const c = check(p.program, s.resolution, this.checkEnv, exp, s.diagnostics);
      if (c.diagnostics.length > 0) return { ok: false, diagnostics: c.diagnostics, source: p.source };
      dynamicSpans = c.dynamicSpans;
      type = printType(c.type);
      sites = c.sites;
    } else if (s.diagnostics.length > 0) return { ok: false, diagnostics: s.diagnostics, source: p.source };
    const c = compileProgram(p.program, s.resolution, this.compileEnv);
    const prog = new Program(this, p.source, c.run, s.resolution.slotCount, c.missing, dynamicSpans);
    prog.type = type;
    prog.sites = sites;
    return { ok: true, program: prog };
  }

  /** Compile a program, with a check unless `check: false`. Throws RisorError of phase `declare`, `parse` or `check`. */
  compile(text: string, options: CompileOptions = {}): Program {
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

  constructor(env: Env, source: Source, code: (r: Run) => unknown, slotCount: number, missing: Set<string>, dynamicSpans: Set<string> | null = null) {
    this.dynamicSpans = dynamicSpans;
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
      } else if (f.code === "no-match" || f.code === "not-conforming") cls = "environment";
    }
    if (cls !== undefined) d.class = cls;
    if (f.code === "failed") d.data = f.data;
    if (f.cause !== undefined) (d as Diagnostic & { cause?: unknown }).cause = f.cause;
    return d;
  }
}

export { ABSENT_VAR };
