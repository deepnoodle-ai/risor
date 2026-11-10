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
  }

  /** Parse, apply the static rules and compile. Problems are data. */
  tryCompile(text: string): CompileResult {
    const p = parse(text);
    if (!p.ok) return { ok: false, diagnostics: p.diagnostics, source: p.source };
    const s = staticRules(p.program, this.staticEnv);
    if (s.diagnostics.length > 0) return { ok: false, diagnostics: s.diagnostics, source: p.source };
    const c = compileProgram(p.program, s.resolution, this.compileEnv);
    return { ok: true, program: new Program(this, p.source, c.run, s.resolution.slotCount, c.missing) };
  }

  /** Compile a program. Throws RisorError of phase `parse` or `check`. */
  compile(text: string): Program {
    const r = this.tryCompile(text);
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

  constructor(env: Env, source: Source, code: (r: Run) => unknown, slotCount: number, missing: Set<string>) {
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
    const cls = uncheckedClass(f);
    if (cls !== undefined) d.class = cls;
    if (f.code === "failed") d.data = f.data;
    if (f.cause !== undefined) (d as Diagnostic & { cause?: unknown }).cause = f.cause;
    return d;
  }
}

export { ABSENT_VAR };
