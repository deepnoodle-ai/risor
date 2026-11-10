// The case runner (03 3.10.5 to 3.10.8).
//
// The runner builds each case's environment through the host API, with its
// stubs as host functions, and runs every step this implementation has: the
// declare stage, the parser, the static rules (no type checker yet) and the
// evaluator. A case that needs a library entry this build does not implement
// gets `skip`, with the entries named.

import { readFileSync, readdirSync, statSync } from "node:fs";
import { basename, dirname, join } from "node:path";
import { createEnv, HostError, printSignature, RisorError, toJSON } from "../host/api.ts";
import type { Env, Expectation, HostFunction, OpaqueType } from "../host/api.ts";
import type { NamedTypeDecl } from "../host/env.ts";
import type { Diagnostic } from "../syntax/diagnostic.ts";
import { parseType } from "../types/typetext.ts";
import { plainEqual, truthy } from "../value/value.ts";
import { ANY, fromNode, typeEqual } from "../types/type.ts";
import type { Type } from "../types/type.ts";
import type { SigNode } from "../types/typetext.ts";
import { compareText, jsonString, mapGet, mapKeys, readHost } from "../value/value.ts";
import { conformAllWalk, conformWalk, freeCtx } from "../eval/conform.ts";
import { RunFailure, messageOf } from "../eval/failure.ts";
import { jsonForm, valueData, valueForm } from "./forms.ts";
import type { OpaqueLike } from "./forms.ts";
import { visibleText } from "../syntax/messages.ts";
import type { SchemaOption } from "../types/schema.ts";
import { ParsedProgram, Template, reportJSON } from "../host/api.ts";
import type { AnalysisReport } from "../host/api.ts";
import { parseTemplateText, templateProgram } from "../syntax/template.ts";
import { parse } from "../syntax/parser.ts";
import { Source } from "../syntax/source.ts";
import { formatProgram, formatTemplate, treeEqual } from "../tools/format.ts";
import { complete, hover } from "../tools/assist.ts";
import type { CompletionResult, HoverResult } from "../tools/assist.ts";
import type { Classifier } from "../value/value.ts";
import { CONFIG_STATIC, CONVERSION_STATIC, loadCaseFile, validateCaseFile } from "./casefile.ts";
import type { Case, CaseFile, ExpectDiagnostic, Renderings, Stub } from "./casefile.ts";
import { pathText, renderDetail, renderJSON, renderLine } from "../host/render.ts";
import type { RenderDiagnostic } from "../host/render.ts";
import { JsonError, isObject, readJson } from "./json.ts";
import type { Json, JsonObject } from "./json.ts";

export type VerdictKind = "pass" | "fail" | "skip" | "invalid";

export interface Difference {
  field: string;
  expected: unknown;
  found: unknown;
  /** E and F of f09 6.2: the JSON forms of what was expected and what was found. */
  e?: string;
  f?: string;
}

export interface Verdict {
  file: string;
  name: string;
  verdict: VerdictKind;
  reason?: string;
  differences?: Difference[];
  /** Package F9: a fixture that failed its test, and the message (f09 2.4). */
  fixture?: (string | number)[];
  message?: string;
  /** Package F9: the `expect` update mode would write, or null when the case is not written (f09 4.3, 4.4). */
  newExpect?: Json | null;
}

/** The static codes (01 14.4); every implementation reports them. */
export const STATIC_CODES: ReadonlySet<string> = new Set([
  "undefined-name", "undefined-function", "call-of-parameter", "lambda-to-lambda", "not-a-value",
  "arity", "duplicate-name", "shadows-namespace", "duplicate-key", "duplicate-case", "unreachable-arm",
]);

const LIMIT_CODES: ReadonlySet<string> = new Set([
  "steps-exceeded", "allocation-exceeded", "deadline-exceeded", "depth-exceeded", "cancelled",
]);

export interface RunnerOptions {
  /** Kept for callers; the library comes from this build. */
  library?: ReadonlySet<string>;
  /** Run step 4 with the type checker (default), or with the static rules alone. */
  checker?: boolean;
  /** The case is of suite/core-check/: run checked only (03 3.10.8 item 5). */
  checkOnly?: boolean;
  /** The static codes of the case's directory (f04 6.1 item 4 for suite/conversion/). */
  staticCodes?: ReadonlySet<string>;
  /** Package F9: update mode (f09 4). */
  update?: boolean;
  /** 03 3.10.9 item 3: the host's base environment, of which each case's is a child. */
  base?: Env;
  /** 03 3.10.9 item 3: the host's variables, for the names a case's `vars` does not give. */
  vars?: (name: string) => unknown;
}

export type Outcome =
  | { kind: "failure"; phase: Diagnostic["phase"]; diagnostics: Diagnostic[]; source?: Source; usage?: { steps: number; allocated: number }; type?: string | null; sites?: { class: string; span: [number, number, number, number] }[] }
  | { kind: "result"; ran: boolean; value?: unknown; usage?: { steps: number; allocated: number }; type?: string | null; sites?: { class: string; span: [number, number, number, number] }[] };

/** An opaque value of a case file (03 3.10.3 item 7): one object for each type and id. */
export class OpaqueRef {
  readonly type: string;
  readonly id: number;
  constructor(type: string, id: number) {
    this.type = type;
    this.id = id;
  }
}
const OPAQUES = new Map<string, OpaqueRef>();
function opaqueRef(type: string, id: number): OpaqueRef {
  const k = `${type}\u0000${id}`;
  let o = OPAQUES.get(k);
  if (o === undefined) OPAQUES.set(k, (o = new OpaqueRef(type, id)));
  return o;
}

/** A JSON value of a case file as a host value: `$opaque` objects become opaque values. */
export function hostValue(v: Json): unknown {
  if (Array.isArray(v)) return v.map(hostValue);
  if (isObject(v)) {
    if ("$opaque" in v) {
      const o = v.$opaque as { type: string; id: number };
      return opaqueRef(o.type, o.id);
    }
    const out: Record<string, unknown> = Object.create(null);
    for (const [k, x] of Object.entries(v)) out[k] = hostValue(x);
    return out;
  }
  return v;
}

/** A host function from a stub (03 3.10.4). `unmatched` records a call no row answers. */
/** A call that no row of a stub matched: the stub's name and the call's arguments (f09 6.2). */
export interface Unmatched {
  name: string;
  args: unknown[];
}

function stubFunction(st: Stub, cls: () => Classifier, unmatched: Unmatched[], name: string, baseType?: string): HostFunction {
  const rows = st.calls.map((r) => ({ ...r, hostArgs: r.args?.map(hostValue), hostResult: r.result === undefined ? undefined : hostValue(r.result.value) }));
  return {
    type: st.type ?? baseType!,
    call: (handle, ...args) => {
      for (const r of rows) {
        if (r.hostArgs !== undefined) {
          if (r.hostArgs.length !== args.length) continue;
          if (!r.hostArgs.every((a, i) => plainEqual(args[i], a, cls()))) continue;
        }
        handle.charge(r.charge);
        handle.allocate(r.allocate);
        if (r.error !== undefined) throw new HostError(r.error);
        return r.hostResult;
      }
      unmatched.push({ name, args });
      throw new HostError(`no row of the stub ${name} matches the call`);
    },
  };
}

function needsRun(c: Case): boolean {
  const ex = c.expect;
  return (
    c.load === true ||
    c.vars !== undefined ||
    (ex.kind === "result" && (ex.value !== undefined || ex.json !== undefined || ex.truthy !== undefined || ex.steps !== undefined || ex.allocated !== undefined)) ||
    (ex.kind === "failure" && ex.phase === "run")
  );
}

/** Run one case and give its verdict. */
export function runCase(c: Case, file: string, opts: RunnerOptions = {}): Verdict {
  const v = (verdict: VerdictKind, extra: Partial<Verdict> = {}): Verdict => ({ file, name: c.name, verdict, ...extra });
  if (c.skip !== undefined) return v("skip", { reason: c.skip });
  const ex = c.expect;
  // f02 4.2: an analysis case is Read, Parse, Analyze, once.
  if (ex.kind === "result" && ex.analysis !== undefined) return analysisCase(c, v);
  // f04 9.3: a conform case.
  if (c.conform !== undefined) return conformCase(c, v, opts.update === true);
  // f08 7.4: a render case.
  if (c.render !== undefined) return renderCase(c, v);
  // f12 7: a case of the tools, run once.
  if (c.tool !== undefined) return opts.checker === false ? v("skip", { reason: "a case of the tools is run once" }) : toolCase(c, v);
  // f09 3.2, 9.3 item 1: a runner case, run once, by the whole runner.
  if (c.runner !== undefined) return opts.checker === false ? v("skip", { reason: "a runner case is run once, by the runner with the checker" }) : runnerCase(c, v, opts);
  const checker = opts.checker !== false;
  const staticCodes = opts.staticCodes ?? STATIC_CODES;
  if (!checker) {
    // 03 3.11: a file that is run checked only is not run with the static rules alone.
    if (opts.checkOnly === true) return v("skip", { reason: "run checked only" });
    if (c.load === true) return v("skip", { reason: "a load is run once, checked" });
    // 03 3.10.8 item 5: a runner with no type checker.
    if (ex.kind === "result" && (ex.type !== undefined || ex.sites !== undefined)) {
      return v("skip", { reason: "needs the type checker (type or sites)" });
    }
    if (ex.kind === "failure" && ex.phase === "check" && ex.diagnostics.some((d) => !staticCodes.has(d.code))) {
      return v("skip", { reason: "needs the type checker (a check code outside the static rules)" });
    }
  }
  // 03 3.10.8 item 5: "check": false compiles with the static rules alone, and
  // compares `class` for every code; otherwise checked, and again with the
  // static rules alone, unless the case needs the checker.
  const needsChecker =
    opts.checkOnly === true ||
    // f10 10.2 item 3: a load case is run once, checked.
    c.load === true ||
    (ex.kind === "result" && (ex.type !== undefined || ex.sites !== undefined)) ||
    (ex.kind === "failure" && ex.phase === "check" && ex.diagnostics.some((d) => !staticCodes.has(d.code)));
  const runs: { checker: boolean; allClasses: boolean }[] =
    c.check === false ? [{ checker: false, allClasses: true }]
    : !checker ? [{ checker: false, allClasses: false }]
    : needsChecker ? [{ checker: true, allClasses: true }]
    : [{ checker: true, allClasses: true }, { checker: false, allClasses: false }];
  // f09 2.2: the fixtures are tested once, before the first run that reaches step 5.
  let toTest = c.precondition !== false;
  const found = new Map<string, Difference>();
  let newExpect: Json | null | undefined;
  let updateOutcome: { o: Outcome; checked: boolean } | undefined;
  for (const run of runs) {
    const e = executeCase(c, needsRun(c), run.checker, toTest, opts);
    if (e.fixtureTested) toTest = false;
    if (e.fixture !== undefined) return v("invalid", { fixture: e.fixture.path, message: e.fixture.message, newExpect: null });
    RUN_CHECKED = run.allClasses;
    CURRENT_ENV = e.env;
    if (e.missing !== null) return v("skip", { reason: `needs library entries: ${e.missing.join(", ")}` });
    const diffs = differences(c, e.outcome, e.unmatched, run.checker);
    for (const d of diffs) if (!found.has(d.field)) found.set(d.field, d);
    // f09 4.4: the new `expect` is made from the first run, the checked one unless "check" is false.
    if (opts.update === true && newExpect === undefined) {
      let o = e.outcome;
      let unmatched = e.unmatched.length > 0;
      let env = e.env;
      // A case written whole is taken through every stage: run when its program compiles.
      if (o.kind === "result" && !o.ran && !sameShape(c, o)) {
        const e2 = executeCase(c, true, run.checker, toTest, opts);
        if (e2.fixtureTested) toTest = false;
        if (e2.fixture !== undefined) return v("invalid", { fixture: e2.fixture.path, message: e2.fixture.message, newExpect: null });
        if (e2.missing !== null) return v("skip", { reason: `needs library entries: ${e2.missing.join(", ")}` });
        o = e2.outcome;
        unmatched ||= e2.unmatched.length > 0;
        env = e2.env;
      }
      updateOutcome = { o, checked: run.checker };
      newExpect = unmatched ? null : updatedExpect(c, o, run.checker, env);
    }
  }
  // f09 4.3 item 1: a case with no `expect` differs in `outcome`.
  if (c.rawExpect === undefined && updateOutcome !== undefined) {
    found.set("outcome", { field: "outcome", expected: "an expect", found: "an outcome", e: "nothing", f: jsonForm(outcomeWhole(updateOutcome.o, updateOutcome.checked)) });
  }
  const diffs = [...found.values()].sort((a, b) => fieldRank(a.field) - fieldRank(b.field));
  return diffs.length === 0 ? v("pass", { newExpect: newExpect ?? null }) : v("fail", { differences: diffs, newExpect: newExpect ?? null });
}

/** The order of the names of differences (03 3.10.8 item 2, with the names packages add). */
const FIELD_ORDER = ["outcome", "analysis", "description", "type", "sites", "value", "json", "truthy", "diagnostics", "rendered", "steps", "allocated", "functions"];

function fieldRank(f: string): number {
  const i = FIELD_ORDER.indexOf(f);
  return i < 0 ? FIELD_ORDER.length : i;
}

/** The differences of one run, each with its E and F (f09 6.2). */
function differences(c: Case, o: Outcome, unmatched: Unmatched[], checked: boolean): Difference[] {
  const diffs = compare(c, o).map((d) => withForms(c, o, d, checked));
  if (unmatched.length > 0) {
    const u = unmatched[0];
    diffs.push({ field: "functions", expected: "a row for every call", found: u.name, e: u.name, f: valueForm(u.args, CLASSIFIER, opaqueOf) });
  }
  return diffs;
}

function opaqueOf(x: unknown): OpaqueLike | null {
  return x instanceof OpaqueRef ? x : null;
}

export interface Execution {
  outcome: Outcome;
  /** Stubs that a call matched no row of (03 3.10.4). */
  unmatched: Unmatched[];
  /** Package F9: whether the fixtures were tested, and the one that failed (f09 2). */
  fixtureTested?: boolean;
  fixture?: { path: (string | number)[]; message: string };
  /** Library entries this build lacks, when the run needed them; else null. */
  missing: string[] | null;
  /** The case's environment, when it was built. */
  env: Env | null;
}

/** Run a case's steps (03 3.10.5), with or without step 5. Judges nothing. */
export function executeCase(c: Case, run: boolean, checker = true, testFixtures = false, ctx: { base?: Env; vars?: (name: string) => unknown } = {}): Execution {
  // Step 2: declare.
  const unmatched: Unmatched[] = [];
  let env: Env | undefined;
  const cls = () => env!.classifier;
  const base = ctx.base;
  const be = base?.environment;
  // 03 3.10.4: a stub with no type keeps the base environment's signature.
  const baseSig = (sig: SigNode | undefined) => (sig === undefined || be === undefined ? undefined : printSignature(sig, be.opaque, be.named));
  const opaque: Record<string, OpaqueType> = {};
  for (const t of c.opaque ?? []) opaque[t] = { test: (x) => x instanceof OpaqueRef && x.type === t, methods: {} };
  for (const [key, st] of Object.entries(c.methods ?? {})) {
    const dot = key.indexOf(".");
    const t = key.slice(0, dot);
    const m = key.slice(dot + 1);
    // A method of a type that `opaque` does not declare: let declare report it, or add it to the base's type.
    if (opaque[t] === undefined) opaque[t] = base !== undefined && be!.opaque.has(t) ? ({ methods: {} } as unknown as OpaqueType) : { test: () => false, methods: {} };
    opaque[t].methods![m] = stubFunction(st, cls, unmatched, key, baseSig(be?.methods.get(key)?.sig));
  }
  const functions: Record<string, HostFunction> = {};
  for (const [k, st] of Object.entries(c.functions ?? {})) functions[k] = stubFunction(st, cls, unmatched, k, baseSig(be?.functions.get(k)?.sig));
  const done = (outcome: Outcome): Execution => ({ outcome, unmatched, missing: null, env: env ?? null });
  let compiled: ReturnType<Env["tryCompile"]>;
  let loadType: Type | null = null;
  try {
    const options = {
      version: c.version, vars: c.env, opaque, functions, reject: c.reject, limits: c.limits, opaqueNames: c.opaque ?? [], removed: c.removed,
      types: c.types as NamedTypeDecl[] | undefined,
      groups: c.groups,
      schemas: c.schemas as Record<string, { schema: unknown; options?: SchemaOption[] }> | undefined,
      facts: c.facts?.map((someNotNull) => ({ someNotNull })),
    };
    // 03 3.10.9: every case runs in a child of the base environment; each limit given replaces the base's.
    env = base === undefined ? createEnv(options) : base.child({ ...options, opaqueNames: c.opaque ?? [], limits: c.limits === undefined ? undefined : { ...be!.limits, ...c.limits } });
    // The expected result is declared type text, resolved in every compile (03 3.3.1).
    if (!checker && c.returns !== undefined && c.returns !== "condition") env.declaredType(c.returns);
    // f10 10.1: the expected result by schema.
    const expected: Expectation | undefined = c.returnsSchema !== undefined ? { schema: c.returnsSchema.schema, options: c.returnsSchema.options as SchemaOption[] | undefined } : c.returns;
    // Steps 3 and 4: parse (a template in its mode, f02 4.1), then the check (or the static rules alone).
    let input: string | ParsedProgram = c.source;
    if (c.template !== undefined) {
      const t = parseTemplateText(c.source);
      if (!t.ok) return done({ kind: "failure", phase: "parse", diagnostics: t.diagnostics, source: t.source });
      input = new ParsedProgram(t.template.source, templateProgram(t.template, c.template));
    }
    compiled = env.tryCompile(input, { check: checker, expected: checker || c.returnsSchema !== undefined ? expected : undefined });
    if (c.load === true) loadType = env.expectedType(expected!);
  } catch (x) {
    if (!(x instanceof RisorError)) throw x;
    return done({ kind: "failure", phase: "declare", diagnostics: [...x.diagnostics] });
  }
  if (!compiled.ok) {
    return done({ kind: "failure", phase: compiled.diagnostics[0].phase, diagnostics: compiled.diagnostics, source: compiled.source });
  }
  const prog = compiled.program;
  const typeInfo = { type: prog.type, sites: prog.sites.map((x) => ({ class: x.class, span: prog.source.lineSpan(x.span) })) };
  // Step 5: run.
  if (!run) return done({ kind: "result", ran: false, ...typeInfo });
  if (compiled.program.missing.size > 0) return { outcome: { kind: "result", ran: false }, unmatched, missing: [...compiled.program.missing].sort(), env };
  // f09 2.2: the fixture test, between steps 4 and 5.
  if (testFixtures) {
    const f = fixtureTest(c, env);
    if (f !== null) return { ...done({ kind: "result", ran: false }), fixtureTested: true, fixture: f };
  }
  const vars = c.vars ?? {};
  const o = compiled.program.execute((name) => (Object.prototype.hasOwnProperty.call(vars, name) ? hostValue(vars[name]) : ctx.vars?.(name)));
  // f10 10.2 item 4: step 5a, the conformance function on the value of a load, with a maximum of 1.
  if (o.ok && loadType !== null) {
    const fs = conformAllWalk(o.value, loadType, env.classifier, 1);
    if (fs.length > 0) {
      const f = fs[0];
      const d: Diagnostic = { phase: "run", code: "not-conforming", class: "value", path: f.path, message: messageOf(new RunFailure("not-conforming", undefined, f.template, f.params), env.classifier) };
      return { ...done({ kind: "failure", phase: "run", diagnostics: [d], source: compiled.program.source, usage: o.usage, ...typeInfo }), fixtureTested: testFixtures };
    }
  }
  return {
    ...done(
      o.ok
        ? { kind: "result", ran: true, value: o.value, usage: o.usage, ...typeInfo }
        : { kind: "failure", phase: "run", diagnostics: [o.diagnostic], source: compiled.program.source, usage: o.usage, ...typeInfo },
    ),
    fixtureTested: testFixtures,
  };
}

/** The fixture test (f09 2): vars, then functions, then methods, each in code point order; the first failure. */
function fixtureTest(c: Case, env: Env): { path: (string | number)[]; message: string } | null {
  const e = env.environment;
  const free = freeCtx(env.classifier);
  const test = (value: Json, t: Type, place: (string | number)[]) => {
    const f = conformWalk(hostValue(value), t, free);
    if (f === null) return null;
    return { path: [...place, ...f.path], message: messageOf(new RunFailure("not-conforming", undefined, f.template, f.params), env.classifier) };
  };
  for (const k of Object.keys(c.vars ?? {}).sort(compareText)) {
    const t = e.variableTypes.get(k);
    if (t === undefined) continue;
    const r = test(c.vars![k], t, ["vars", k]);
    if (r !== null) return r;
  }
  // f14 10.2: each host fact whose members vars all supplies, in order: one of them is not null.
  const vars = c.vars ?? {};
  for (let i = 0; i < (c.facts ?? []).length; i++) {
    const ms = c.facts![i];
    if (!ms.every((m) => Object.prototype.hasOwnProperty.call(vars, m))) continue;
    if (ms.every((m) => vars[m] === null)) return { path: ["facts", i], message: `at least one of ${ms.join(", ")} must not be null` };
  }
  const stubs = (kind: "functions" | "methods", table: Record<string, Stub> | undefined, sigOf: (k: string) => SigNode | undefined) => {
    for (const k of Object.keys(table ?? {}).sort(compareText)) {
      const sig = sigOf(k);
      if (sig === undefined) continue;
      const rows = table![k].calls;
      for (let i = 0; i < rows.length; i++) {
        const row = rows[i];
        if (row.result !== undefined) {
          // f09 2.1 item 5: each type variable read as any.
          const t = fromNode(sig.result, { opaque: e.opaque, named: e.named, variable: () => ANY });
          const r = test(row.result.value, t, [kind, k, "calls", i, "result"]);
          if (r !== null) return r;
        }
        if (row.error !== undefined && !sig.canFail) {
          return { path: [kind, k, "calls", i, "error"], message: "the signature has no !, so the stub may not return an error" };
        }
      }
    }
    return null;
  };
  return stubs("functions", c.functions, (k) => e.functions.get(k)?.sig) ?? stubs("methods", c.methods, (k) => e.methods.get(k)?.sig);
}

function compareRenderings(want: Renderings, ds: RenderDiagnostic[], source: string | null, prefix?: string): Difference[] {
  const out: Difference[] = [];
  const name = (k: string) => (prefix === undefined ? k : `${prefix}.${k}`);
  if (want.line !== undefined && renderLine(ds) !== want.line) out.push({ field: name("line"), expected: want.line, found: renderLine(ds) });
  if (want.detail !== undefined && renderDetail(ds, source) !== want.detail) out.push({ field: name("detail"), expected: want.detail, found: renderDetail(ds, source) });
  if (want.json !== undefined && renderJSON(ds, CLASSIFIER) !== want.json) out.push({ field: name("json"), expected: want.json, found: renderJSON(ds, CLASSIFIER) });
  return out;
}

/** The diagnostics of a render case, as given (f08 7.4). */
export function renderCaseDiagnostics(c: Case): RenderDiagnostic[] {
  return c.render!.diagnostics.map((d) => {
    const r: RenderDiagnostic = { phase: d.phase as string, code: d.code as string, message: d.message as string };
    if (typeof d.class === "string") r.class = d.class;
    if (Array.isArray(d.span)) r.span = d.span as [number, number, number, number];
    if (Array.isArray(d.path)) r.path = d.path as (string | number)[];
    if ("data" in d) r.data = hostValue(d.data);
    if (Array.isArray(d.trace)) r.trace = d.trace as unknown as RenderDiagnostic["trace"];
    return r;
  });
}

function renderCase(c: Case, v: (verdict: VerdictKind, extra?: Partial<Verdict>) => Verdict): Verdict {
  const ex = c.expect as Renderings;
  const d = compareRenderings(ex, renderCaseDiagnostics(c), c.render!.source ?? null);
  return d.length === 0 ? v("pass") : v("fail", { differences: d });
}

/** The field of the case's `expect` as written, or undefined. */
function rawField(c: Case, k: string): Json | undefined {
  const r = c.rawExpect;
  return isObject(r) && k in r ? r[k] : undefined;
}

/** The columns a phase may write for a diagnostic (f09 4.4). */
const COLUMNS: Record<string, string[]> = {
  declare: ["code"],
  parse: ["code", "span", "message"],
  check: ["code", "class", "span", "message"],
  run: ["code", "class", "span", "path", "message", "data", "trace"],
};

/** A diagnostic as case-file data, with these columns, those it has (f09 4.4). */
function diagData(d: Diagnostic, columns: string[], source: Source | undefined): Json {
  const out: { [k: string]: Json } = Object.create(null);
  const limit = LIMIT_CODES.has(d.code);
  for (const col of columns) {
    switch (col) {
      case "code":
        out.code = d.code;
        break;
      case "class":
        if (d.class !== undefined) out.class = d.class;
        break;
      case "span":
        if (d.span !== undefined && !limit && source !== undefined) out.span = source.lineSpan(d.span);
        break;
      case "message":
        if (d.phase !== "declare") out.message = d.message;
        break;
      case "path":
        if (d.code === "not-conforming" && d.path !== undefined) out.path = d.path;
        break;
      case "data":
        if (d.code === "failed") {
          const v = valueData(d.data, CLASSIFIER, opaqueOf);
          out.data = v.ok ? v.data : null;
        }
        break;
      case "trace":
        if (!limit && d.trace !== undefined && source !== undefined) out.trace = d.trace.map((t) => ({ index: t.index, name: t.name, span: source.lineSpan(t.span) }));
        break;
    }
  }
  return out;
}

/** The diagnostics of a failure written with these columns: the first for parse and declare (f09 4.4). */
function diagsData(o: Extract<Outcome, { kind: "failure" }>, columns: string[] | null): Json[] {
  const list = o.phase === "parse" || o.phase === "declare" || o.phase === "run" ? o.diagnostics.slice(0, 1) : o.diagnostics;
  const cols = columns ?? COLUMNS[o.phase].filter((c) => c !== "trace");
  return list.map((d) => diagData(d, cols, o.source));
}

/** The outcome as an `expect` written whole (f09 4.4). */
function outcomeWhole(o: Outcome, checked: boolean): Json {
  if (o.kind === "failure") return { diagnostics: diagsData(o, null), phase: o.phase };
  const out: { [k: string]: Json } = Object.create(null);
  if (checked && o.type != null) out.type = o.type;
  if (o.ran) {
    const v = valueData(o.value, CLASSIFIER, opaqueOf);
    out.value = v.ok ? v.data : null;
    if (o.usage !== undefined) {
      out.steps = o.usage.steps;
      out.allocated = o.usage.allocated;
    }
  }
  return out;
}

/** The columns of an expected list of diagnostics: `code` and every field one of them has (f09 4.4). */
function expectedColumns(c: Case): string[] {
  const raw = rawField(c, "diagnostics");
  const cols = new Set(["code"]);
  if (Array.isArray(raw)) for (const d of raw) if (isObject(d)) for (const k of Object.keys(d)) cols.add(k);
  return ["code", "class", "span", "path", "message", "data", "trace"].filter((k) => cols.has(k));
}

/** E and F of a difference (f09 6.2). */
function withForms(c: Case, o: Outcome, d: Difference, checked: boolean): Difference {
  const raw = (k: string) => {
    const r = rawField(c, k);
    return r === undefined ? "null" : jsonForm(r);
  };
  const vf = (v: unknown) => valueForm(v, CLASSIFIER, opaqueOf);
  let e = jsonForm((d.expected ?? null) as Json);
  let f = "null";
  switch (d.field) {
    case "outcome":
      e = c.rawExpect === undefined ? "nothing" : jsonForm(c.rawExpect);
      f = jsonForm(outcomeWhole(o, checked));
      break;
    case "type":
      e = raw("type");
      f = o.type == null ? "null" : jsonString(o.type);
      break;
    case "sites":
      e = raw("sites");
      f = jsonForm((o.sites ?? []).map((x) => ({ class: x.class, span: x.span })) as Json);
      break;
    case "value":
      e = raw("value");
      f = o.kind === "result" ? vf(o.value) : "null";
      break;
    case "truthy":
      e = raw("truthy");
      f = o.kind === "result" ? String(truthy(o.value, CLASSIFIER)) : "null";
      break;
    case "json":
      e = raw("json");
      f = d.found === null ? "null" : jsonString(String(d.found));
      break;
    case "description":
      e = raw("description");
      f = d.found === undefined ? "null" : vf(d.found);
      break;
    case "diagnostics":
      e = raw("diagnostics");
      f = o.kind === "failure" ? jsonForm(diagsData(o, expectedColumns(c))) : "null";
      break;
    case "rendered":
      e = raw("rendered");
      f = jsonForm(d.found as Json);
      break;
    case "steps":
    case "allocated":
      e = raw(d.field);
      f = o.usage === undefined ? "null" : String(o.usage[d.field]);
      break;
  }
  return { ...d, e, f };
}

/** The lines of a case in the report (f09 6.1). */
export function reportLines(v: Verdict, status: string = v.verdict): string {
  const name = visibleText(v.name);
  switch (status) {
    case "fail": {
      let out = `  FAIL ${name}\n`;
      for (const d of v.differences ?? []) {
        if (d.field === "functions") out += `    functions: no row of ${d.e} matches ${d.f}\n`;
        else out += `    ${d.field}: expected ${d.e ?? jsonForm((d.expected ?? null) as Json)}, found ${d.f ?? JSON.stringify(d.found ?? null)}\n`;
      }
      return out;
    }
    case "invalid":
      return `  INVALID ${name}\n    fixture ${pathText(v.fixture ?? [])}: ${visibleText(v.message ?? "")}\n`;
    case "skip":
      return `  SKIP ${name}: ${visibleText(v.reason ?? "")}\n`;
    case "updated":
      return `  UPDATED ${name}\n`;
  }
  return "";
}

/** The three answers of a conform case (f04 9.3), or its declare failure. */
export type ConformRun =
  | { ok: false; diagnostics: Diagnostic[] }
  | {
      ok: true;
      fn: Diagnostic | null;
      asRun: { ok: true; value: unknown; steps: number } | { ok: false; diagnostic: Diagnostic; steps: number };
      isRun: { ok: true; value: unknown; steps: number } | { ok: false; diagnostic: Diagnostic; steps: number };
      value: unknown;
      /** ConformAll's list, when the case has `max` (f08 7.5). */
      all: Diagnostic[] | null;
    };

export function runConform(c: Case): ConformRun {
  const cf = c.conform!;
  const opaque: Record<string, OpaqueType> = {};
  for (const t of c.opaque ?? []) opaque[t] = { test: (x) => x instanceof OpaqueRef && x.type === t, methods: {} };
  try {
    // Steps 1 and 2: declare, then resolve the type as declared text.
    const env = createEnv({ version: c.version, opaque, opaqueNames: c.opaque ?? [], types: c.types as NamedTypeDecl[] | undefined });
    env.declaredType(cf.type);
    const value = hostValue(cf.value);
    // Step 3: the function.
    const fn = env.conform(value, cf.type);
    // Step 4: two runs in a child that declares `v: any`, with the static rules alone.
    const child = env.child({ vars: "{v: any}" });
    const runOf = (form: string) => {
      const p = child.compile(`v ${form} ${cf.type}`, { check: false });
      const o = p.execute({ v: value });
      return o.ok ? { ok: true as const, value: o.value, steps: o.usage.steps } : { ok: false as const, diagnostic: o.diagnostic, steps: o.usage.steps };
    };
    const all = cf.max === undefined ? null : env.conformAll(value, cf.type, cf.max);
    return { ok: true, fn, asRun: runOf("as"), isRun: runOf("is"), value, all };
  } catch (x) {
    if (!(x instanceof RisorError)) throw x;
    return { ok: false, diagnostics: [...x.diagnostics] };
  }
}

function conformCase(c: Case, v: (verdict: VerdictKind, extra?: Partial<Verdict>) => Verdict, update = false): Verdict {
  const r = runConform(c);
  const ex = c.expect;
  if (update) {
    const verdict = conformCase(c, v);
    return { ...verdict, newExpect: verdict.verdict === "pass" ? null : conformExpect(c, r) };
  }
  if (!r.ok) {
    if (ex.kind === "failure") return finish(c, { kind: "failure", phase: "declare", diagnostics: r.diagnostics }, [], v);
    return v("fail", { differences: [{ field: "outcome", expected: "a conform result", found: describeFailure({ kind: "failure", phase: "declare", diagnostics: r.diagnostics }) }] });
  }
  if (ex.kind !== "conform") return v("fail", { differences: [{ field: "outcome", expected: `failure of phase ${ex.kind === "failure" ? ex.phase : "?"}`, found: "a conform result" }] });
  const d: Difference[] = [];
  const asOk = r.asRun.ok && plainEqual(r.asRun.value, r.value, CLASSIFIER);
  const isOk = r.isRun.ok && r.isRun.value === true;
  const fnOk = r.fn === null;
  // A run that failed by something other than not-conforming is a disagreement too.
  const asDiag = r.asRun.ok ? null : r.asRun.diagnostic;
  const agree = asOk === isOk && isOk === fnOk && (r.isRun.ok) && (asOk || asDiag?.code === "not-conforming");
  if (!agree || fnOk !== ex.conforms) {
    d.push({ field: "conforms", expected: ex.conforms, found: { function: fnOk, as: r.asRun.ok ? asOk : asDiag, is: r.isRun.ok ? r.isRun.value : r.isRun.diagnostic } });
  }
  if (ex.path !== undefined) {
    const same = (p: unknown) => JSON.stringify(p) === JSON.stringify(ex.path);
    if (!same(r.fn?.path) || !same(asDiag?.path)) d.push({ field: "path", expected: ex.path, found: { function: r.fn?.path, as: asDiag?.path } });
  }
  if (ex.message !== undefined) {
    if (r.fn?.message !== ex.message || asDiag?.message !== ex.message) d.push({ field: "message", expected: ex.message, found: { function: r.fn?.message, as: asDiag?.message } });
  }
  if (r.all !== null) {
    // f08 7.5 item 2: empty exactly when it conforms; the first entry is the function's.
    const first = r.all[0];
    const agrees = (r.all.length === 0) === fnOk && (fnOk || (JSON.stringify(first.path) === JSON.stringify(r.fn!.path) && first.message === r.fn!.message));
    if (!agrees && !d.some((x) => x.field === "conforms")) d.push({ field: "conforms", expected: ex.conforms, found: { conformAll: r.all.map((x) => ({ path: x.path, message: x.message })) } });
  }
  if (ex.steps !== undefined && (r.asRun.steps !== ex.steps || r.isRun.steps !== ex.steps)) {
    d.push({ field: "steps", expected: ex.steps, found: { as: r.asRun.steps, is: r.isRun.steps } });
  }
  if (ex.failures !== undefined && r.all !== null) {
    const all = r.all;
    const same = all.length === ex.failures.length && ex.failures.every((f, i) => JSON.stringify(f.path) === JSON.stringify(all[i].path) && (f.message === undefined || f.message === all[i].message));
    if (!same) d.push({ field: "failures", expected: ex.failures, found: all.map((x) => ({ path: x.path, message: x.message })) });
  }
  return d.length === 0 ? v("pass") : v("fail", { differences: d });
}

/** The report of the analysis of a case's text, or its parse failure (f02 3, 4.2). */
export function analysisOf(c: Case): { ok: true; report: AnalysisReport; source: Source } | { ok: false; diagnostics: Diagnostic[]; source: Source } {
  if (c.template !== undefined) {
    const t = parseTemplateText(c.source);
    if (!t.ok) return t;
    return { ok: true, report: new Template(t.template).analyze(c.template), source: t.template.source };
  }
  const p = parse(c.source);
  if (!p.ok) return p;
  return { ok: true, report: new ParsedProgram(p.source, p.program).analyze(), source: p.source };
}

function analysisCase(c: Case, v: (verdict: VerdictKind, extra?: Partial<Verdict>) => Verdict): Verdict {
  const ex = c.expect as { analysis: JsonObject };
  const a = analysisOf(c);
  if (!a.ok) {
    return v("fail", { differences: [{ field: "outcome", expected: "a result", found: describeFailure({ kind: "failure", phase: "parse", diagnostics: a.diagnostics, source: a.source }) }] });
  }
  const found = JSON.parse(reportJSON(a.report, a.source)) as Record<string, unknown>;
  let same = true;
  for (const [k, want] of Object.entries(ex.analysis)) {
    if (k === "literal") {
      const lit = a.report.literal;
      if (want === false || lit === false) same &&= want === false && lit === false;
      else same &&= plainEqual(lit.value, hostValue((want as JsonObject).value), CLASSIFIER);
    } else same &&= JSON.stringify(found[k]) === JSON.stringify(want);
  }
  return same ? v("pass") : v("fail", { differences: [{ field: "analysis", expected: ex.analysis, found }] });
}

function finish(c: Case, o: Outcome, unmatched: string[], v: (verdict: VerdictKind, extra?: Partial<Verdict>) => Verdict): Verdict {
  const diffs = compare(c, o);
  if (unmatched.length > 0) diffs.push({ field: "functions", expected: "a row for every call", found: `no row matched a call of ${unmatched.join(", ")}` });
  return diffs.length === 0 ? v("pass") : v("fail", { differences: diffs });
}

function compare(c: Case, o: Outcome): Difference[] {
  const ex = c.expect;
  if (ex.kind === "result") {
    if (o.kind !== "result") {
      return [{ field: "outcome", expected: "a result", found: describeFailure(o) }];
    }
    const d: Difference[] = [];
    if (ex.description !== undefined) {
      // f01 7.2: compared as JSON data.
      const found = CURRENT_ENV?.describe();
      if (found === undefined || !plainEqual(found, hostValue(ex.description.value), CLASSIFIER)) {
        d.push({ field: "description", expected: ex.description.value, found });
      }
    }
    if (ex.type !== undefined) {
      let same = false;
      try {
        same = o.type != null && CURRENT_ENV !== null && typeEqual(CURRENT_ENV.typeOf(ex.type), CURRENT_ENV.typeOf(o.type));
      } catch {
        same = false;
      }
      if (!same) d.push({ field: "type", expected: ex.type, found: o.type });
    }
    if (ex.sites !== undefined) {
      const found = o.sites ?? [];
      const same = found.length === ex.sites.length && ex.sites.every((x, i) => x.class === found[i].class && x.span.every((n, j) => n === found[i].span[j]));
      if (!same) d.push({ field: "sites", expected: ex.sites, found });
    }
    if (ex.value !== undefined && !plainEqual(o.value, hostValue(ex.value.value), CLASSIFIER)) {
      d.push({ field: "value", expected: ex.value.value, found: show(o.value) });
    }
    if (ex.json !== undefined) {
      // f06 11.2: the binding's JSON text of the value, code point by code point.
      let found: string | null;
      try {
        found = toJSON(o.value);
      } catch {
        found = null;
      }
      if (found !== ex.json) d.push({ field: "json", expected: ex.json, found });
    }
        if (ex.truthy !== undefined && truthy(o.value, CLASSIFIER) !== ex.truthy) d.push({ field: "truthy", expected: ex.truthy, found: !ex.truthy });
    if (ex.steps !== undefined && o.usage?.steps !== ex.steps) d.push({ field: "steps", expected: ex.steps, found: o.usage?.steps });
    if (ex.allocated !== undefined && o.usage?.allocated !== ex.allocated) d.push({ field: "allocated", expected: ex.allocated, found: o.usage?.allocated });
    return d;
  }
  if (ex.kind === "conform") return [{ field: "outcome", expected: "a conform result", found: "a case with a program" }];
  if (ex.kind !== "failure") return [{ field: "outcome", expected: "another kind of case", found: "a case with a program" }];
  if (o.kind !== "failure" || o.phase !== ex.phase) {
    return [{ field: "outcome", expected: `failure of phase ${ex.phase}`, found: o.kind === "result" ? "a result" : describeFailure(o) }];
  }
  const found = o.diagnostics.map((d) => foundDiag(d, o.source));
  let ok: boolean;
  if (ex.phase === "parse") {
    ok = ex.diagnostics.length <= found.length && ex.diagnostics.every((e, i) => diagMatches(e, found[i]));
  } else if (ex.phase === "declare") {
    ok = ex.diagnostics.length === found.length && matchUnordered(ex.diagnostics, found);
  } else {
    ok = ex.diagnostics.length === found.length && ex.diagnostics.every((e, i) => diagMatches(e, found[i]));
  }
  const d: Difference[] = ok ? [] : [{ field: "diagnostics", expected: ex.diagnostics, found }];
  if (ex.rendered !== undefined) {
    // f08 7.3: parse renders its first diagnostic; check the whole list; run its one.
    const list = ex.phase === "parse" ? o.diagnostics.slice(0, 1) : o.diagnostics;
    const rd = compareRenderings(ex.rendered, list.map((x) => renderable(x, o.source)), renderSource(o));
    if (rd.length > 0) {
      const found: { [k: string]: Json } = Object.create(null);
      const ds = list.map((x) => renderable(x, o.source));
      if (ex.rendered.line !== undefined) found.line = renderLine(ds);
      if (ex.rendered.detail !== undefined) found.detail = renderDetail(ds, renderSource(o));
      if (ex.rendered.json !== undefined) found.json = renderJSON(ds, CLASSIFIER);
      d.push({ field: "rendered", expected: ex.rendered, found });
    }
  }
  if (ex.phase === "run" && !found.some((f) => LIMIT_CODES.has(f.code))) {
    if (ex.steps !== undefined && o.usage?.steps !== ex.steps) d.push({ field: "steps", expected: ex.steps, found: o.usage?.steps });
    if (ex.allocated !== undefined && o.usage?.allocated !== ex.allocated) d.push({ field: "allocated", expected: ex.allocated, found: o.usage?.allocated });
  }
  return d;
}

/** Opaque values of a case are OpaqueRef objects: classified by this test. */
const CLASSIFIER_TESTS: [string, (v: unknown) => boolean][] = [];
import { Classifier as ClassifierClass } from "../value/value.ts";
export const CLASSIFIER = new (class extends ClassifierClass {
  override opaqueName(v: unknown): string | null {
    return v instanceof OpaqueRef ? v.type : super.opaqueName(v);
  }
  override kind(v: unknown) {
    return v instanceof OpaqueRef ? ("opaque" as const) : super.kind(v);
  }
})(CLASSIFIER_TESTS);

/** A value as JSON, for a report. */
export function show(v: unknown, depth = 0): unknown {
  if (depth > 50) return "...";
  if (v instanceof OpaqueRef) return { $opaque: { type: v.type, id: v.id } };
  if (Array.isArray(v)) return v.map((x) => show(x, depth + 1));
  if (v instanceof Map) return Object.fromEntries([...v].map(([k, x]) => [k, show(x, depth + 1)]));
  if (typeof v === "object" && v !== null) return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, show(x, depth + 1)]));
  return v;
}

function describeFailure(o: Outcome): string {
  if (o.kind !== "failure") return "a result";
  const d = o.diagnostics[0];
  const span = d.span !== undefined && o.source !== undefined ? ` at ${JSON.stringify(o.source.lineSpan(d.span))}` : "";
  return `failure of phase ${o.phase}: ${d.code}${span} (${d.message})`;
}

/** Whether `class` is compared for every code (03 3.10.7 item 8). */
let RUN_CHECKED = true;
/** The environment of the case being judged, to read its expected `type`. */
let CURRENT_ENV: Env | null = null;

const CHECK_DEPENDENT: ReadonlySet<string> = new Set(["null-operand", "wrong-kind", "missing-field", "no-match", "not-conforming", "function-error"]);

interface FoundDiag {
  trace?: { span: [number, number, number, number]; name: string; index: number | null }[];
  class?: string;
  data?: unknown;
  rawData?: unknown;
  code: string;
  span?: [number, number, number, number];
  message: string;
  path?: (string | number)[];
}

function foundDiag(d: Diagnostic, source?: Source): FoundDiag {
  const f: FoundDiag = { code: d.code, message: d.message };
  if (d.span !== undefined && source !== undefined) f.span = source.lineSpan(d.span);
  if (d.path !== undefined) f.path = d.path;
  if (d.class !== undefined) f.class = d.class;
  if (d.code === "failed") {
    f.data = show(d.data);
    f.rawData = d.data;
  }
  if (d.trace !== undefined && source !== undefined) f.trace = d.trace.map((t) => ({ span: source.lineSpan(t.span), name: t.name, index: t.index }));
  return f;
}

/** A diagnostic as the renderings read it (f08 5). */
function renderable(d: Diagnostic, source?: Source): RenderDiagnostic {
  const r: RenderDiagnostic = { phase: d.phase, code: d.code, message: d.message };
  if (d.class !== undefined) r.class = d.class;
  if (d.span !== undefined && source !== undefined) r.span = source.lineSpan(d.span);
  if (d.path !== undefined) r.path = d.path;
  if (d.code === "failed") r.data = d.data;
  if (d.phase === "run") r.trace = (d.trace ?? []).map((t) => ({ span: source!.lineSpan(t.span), name: t.name, index: t.index }));
  return r;
}

/** The text a case's diagnostics are rendered with: the source, or the template text (f08 7.3 item 3). */
function renderSource(o: Outcome): string | null {
  return o.kind === "failure" && o.source !== undefined ? o.source.slice(0, o.source.length) : null;
}

function diagMatches(e: ExpectDiagnostic, f: FoundDiag | undefined): boolean {
  if (f === undefined) return false;
  if (e.code !== f.code) return false;
  // 03 3.10.7 item 8: `class` is compared, except for the five codes whose class
  // depends on the check, since this runner does not check.
  if (e.class !== undefined && (RUN_CHECKED || !CHECK_DEPENDENT.has(e.code)) && e.class !== f.class) return false;
  // f10 10.2 item 5: "span": null says the diagnostic has no span.
  if (e.span === null) {
    if (f.span !== undefined) return false;
  } else if (e.span !== undefined && !LIMIT_CODES.has(e.code)) {
    if (f.span === undefined || e.span.some((n, i) => n !== f.span![i])) return false;
  }
  if (e.message !== undefined && e.message !== f.message) return false;
  if (e.data !== undefined && !plainEqual(f.rawData, hostValue(e.data.value), CLASSIFIER)) return false;
  // f08 7.2: the trace, entry by entry; not for a limit code.
  if (e.trace !== undefined && !LIMIT_CODES.has(e.code)) {
    const t = f.trace ?? [];
    if (t.length !== e.trace.length) return false;
    for (let i = 0; i < t.length; i++) {
      const x = e.trace[i];
      if (x.name !== t[i].name || x.index !== t[i].index || x.span.some((n, j) => n !== t[i].span[j])) return false;
    }
  }
  if (e.path !== undefined) {
    if (f.path === undefined || f.path.length !== e.path.length || e.path.some((s, i) => s !== f.path![i])) return false;
  }
  return true;
}

function matchUnordered(es: ExpectDiagnostic[], fs: FoundDiag[]): boolean {
  const used = new Array<boolean>(fs.length).fill(false);
  const go = (k: number): boolean => {
    if (k === es.length) return true;
    for (let j = 0; j < fs.length; j++) {
      if (!used[j] && diagMatches(es[k], fs[j])) {
        used[j] = true;
        if (go(k + 1)) return true;
        used[j] = false;
      }
    }
    return false;
  };
  return go(0);
}

// ---- files and directories -------------------------------------------------

/**
 * Run a case file. A catalog file (`library.json`, `messages.json`: a list of
 * records that each hold `cases`) runs each record's cases as one headerless
 * list (03 3.10.1 item 3).
 */
export function runFile(path: string, opts: RunnerOptions): Verdict[] {
  const file = basename(path);
  let json: Json;
  try {
    json = readJson(readFileSync(path, "utf8"));
  } catch (e) {
    if (e instanceof JsonError || (e as NodeJS.ErrnoException).code !== undefined) {
      return [{ file, name: "(file)", verdict: "invalid", reason: (e as Error).message }];
    }
    throw e;
  }
  if (Array.isArray(json) && json.length > 0 && json.every((r) => isObject(r) && !("expect" in r)) && json.some((r) => isObject(r) && "cases" in r)) {
    // f08 7.8 item 2: every case of messages-check.json is run once, checked.
    if (basename(path) === "messages-check.json") opts = { ...opts, checkOnly: true };
    const out: Verdict[] = [];
    for (const r of json as { [k: string]: Json }[]) {
      if (!("cases" in r)) continue;
      const label = typeof r.name === "string" ? r.name : typeof r.code === "string" ? r.code : "?";
      out.push(...verdictsOf(validateCaseFile(r.cases, join(path, "..")), `${file} ${label}`, opts));
    }
    return out;
  }
  // 03 3.11: a file `until-fNN-` is not run when this implementation offers package NN.
  const until = /^until-f(\d\d)-/.exec(basename(path));
  if (until !== null && OFFERED_PACKAGES.has(Number(until[1]))) return [];
  // f01 7.3 item 1: every case of suite/host-types/ is run checked, once.
  // 03 3.11: in a package directory, a file ending `-check.json` is run checked, once.
  const dir = basename(dirname(path));
  const packageDir = basename(dirname(dirname(path))) === "suite" && dir !== "core-run" && dir !== "core-check";
  const checkOnly = opts.checkOnly ?? (["core-check", "host-types"].includes(dir) || (packageDir && basename(path).endsWith("-check.json")));
  const staticCodes = dir === "conversion" || dir === "unions" || dir === "facts" ? CONVERSION_STATIC : dir === "config" ? CONFIG_STATIC : undefined;
  // f02 4.7: suite/templates/ runs as suite/core-run/ does; an analysis case runs once.
  return verdictsOf(loadCaseFile(path), file, { ...opts, checkOnly, staticCodes });
}

function verdictsOf(cf: CaseFile, file: string, opts: RunnerOptions): Verdict[] {
  if (!cf.ok) {
    const names = cf.names.length > 0 ? cf.names : ["(file)"];
    return names.map((name) => ({ file, name, verdict: "invalid" as const, reason: cf.error }));
  }
  return cf.cases.map((c) => runCase(c, file, opts));
}

/** The packages this implementation offers: F1 to F14. */
export const OFFERED_PACKAGES: ReadonlySet<number> = new Set([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14]);

/** Every `.json` file under a directory, in sorted order. */
export function caseFiles(dir: string): string[] {
  const out: string[] = [];
  const walk = (d: string) => {
    for (const name of readdirSync(d).sort()) {
      const p = join(d, name);
      if (statSync(p).isDirectory()) walk(p);
      else if (name.endsWith(".json")) out.push(p);
    }
  };
  walk(dir);
  return out;
}

/** The library function names from a `library.json`. */
export function libraryFunctionNames(libraryJson: string): Set<string> {
  const json = readJson(readFileSync(libraryJson, "utf8"));
  const names = new Set<string>();
  if (Array.isArray(json)) {
    for (const e of json) {
      if (isObject(e) && typeof e.name === "string" && (e.receiver === undefined || e.receiver === null)) names.add(e.name);
    }
  }
  return names;
}

export function countVerdicts(vs: Verdict[]): Record<VerdictKind, number> {
  const n: Record<VerdictKind, number> = { pass: 0, fail: 0, skip: 0, invalid: 0 };
  for (const v of vs) n[v.verdict]++;
  return n;
}

/** Whether the outcome has the shape of the old `expect`: both results, or both failures of one phase (f09 4.4). */
function sameShape(c: Case, o: Outcome): boolean {
  if (c.rawExpect === undefined) return false;
  const ex = c.expect;
  if (ex.kind === "result") return o.kind === "result";
  if (ex.kind === "failure") return o.kind === "failure" && o.phase === ex.phase;
  return false;
}

/** The value as data for a new `expect`, or null when it cannot be written (f09 4.3). */
function writable(v: unknown): { ok: true; data: Json } | { ok: false } {
  return valueData(v, CLASSIFIER, opaqueOf);
}

/** The `expect` update mode writes for a case, or null when it writes none (f09 4.3, 4.4). */
function updatedExpect(c: Case, o: Outcome, checked: boolean, env: Env | null): Json | null {
  const raw = c.rawExpect;
  if (o.kind === "failure" && o.phase === "declare") {
    if (!(isObject(raw) && raw.phase === "declare")) return null;
    if (o.diagnostics.length > 1) return null;
  }
  if (!sameShape(c, o)) {
    if (o.kind === "result" && o.ran && !writable(o.value).ok) return null;
    return outcomeWhole(o, checked);
  }
  const out: { [k: string]: Json } = Object.create(null);
  if (o.kind === "result") {
    for (const k of Object.keys(raw as JsonObject)) {
      switch (k) {
        case "type":
          if (o.type != null) out.type = o.type;
          break;
        case "sites":
          out.sites = (o.sites ?? []).map((x) => ({ class: x.class, span: x.span })) as Json;
          break;
        case "value": {
          if (!o.ran) break;
          const w = writable(o.value);
          if (!w.ok) return null;
          out.value = w.data;
          break;
        }
        case "truthy":
          if (o.ran) out.truthy = truthy(o.value, CLASSIFIER);
          break;
        case "steps":
        case "allocated":
          if (o.usage !== undefined) out[k] = o.usage[k];
          break;
        case "json":
          if (o.ran) {
            try {
              out.json = toJSON(o.value);
            } catch {
              return null;
            }
          }
          break;
        case "description": {
          const d = env?.describe();
          if (d === undefined) return null;
          const w = writable(d);
          if (!w.ok) return null;
          out.description = w.data;
          break;
        }
        default:
          // A field update mode does not make (analysis) is kept as written.
          out[k] = (raw as JsonObject)[k];
      }
    }
    return out;
  }
  out.phase = o.phase;
  out.diagnostics = diagsData(o, expectedColumns(c));
  const limit = o.diagnostics.some((d) => LIMIT_CODES.has(d.code));
  if (isObject(raw) && !limit && o.usage !== undefined) {
    if ("steps" in raw) out.steps = o.usage.steps;
    if ("allocated" in raw) out.allocated = o.usage.allocated;
  }
  const rr = rawField(c, "rendered");
  if (isObject(rr) && !limit) {
    const list = o.phase === "parse" ? o.diagnostics.slice(0, 1) : o.diagnostics;
    const ds = list.map((x) => renderable(x, o.source));
    const r: { [k: string]: Json } = Object.create(null);
    if ("line" in rr) r.line = renderLine(ds);
    if ("detail" in rr) r.detail = renderDetail(ds, renderSource(o));
    if ("json" in rr) r.json = renderJSON(ds, CLASSIFIER);
    out.rendered = r;
  }
  return out;
}

/** The `expect` update mode writes for a conform case, or null when its three ways disagree (f09 4.3, 4.4). */
function conformExpect(c: Case, r: ConformRun): Json | null {
  if (!r.ok) return null;
  const asOk = r.asRun.ok && plainEqual(r.asRun.value, r.value, CLASSIFIER);
  const isOk = r.isRun.ok && r.isRun.value === true;
  const fnOk = r.fn === null;
  const asDiag = r.asRun.ok ? null : r.asRun.diagnostic;
  const agree = asOk === isOk && isOk === fnOk && r.isRun.ok && (asOk || asDiag?.code === "not-conforming") && r.asRun.steps === r.isRun.steps;
  if (!agree) return null;
  const raw = c.rawExpect;
  const same = isObject(raw) && "conforms" in raw;
  const out: { [k: string]: Json } = Object.create(null);
  const has = (k: string) => !same || (raw as JsonObject)[k] !== undefined;
  out.conforms = fnOk;
  if (!fnOk && has("path")) out.path = r.fn!.path as Json;
  if (!fnOk && has("message")) out.message = r.fn!.message;
  if (has("steps")) out.steps = r.asRun.steps;
  if (same && "failures" in raw && r.all !== null) out.failures = r.all.map((x) => ({ path: x.path as Json, message: x.message }));
  return out;
}

/** The inner file of a runner case, read in the directory of the file that holds it (f09 3.2). */
function innerFile(c: Case): CaseFile {
  const dir = c.dir ?? ".";
  const opts = { noExpect: c.update === true && c.program !== true };
  if (typeof c.runner === "string") {
    if (c.program === true) return programCaseFile(c.runner, dir);
    let json: Json;
    try {
      json = readJson(c.runner);
    } catch (e) {
      if (e instanceof JsonError) return { ok: false, error: `not JSON: ${e.message}`, names: [] };
      throw e;
    }
    return validateCaseFile(json, dir, opts);
  }
  return validateCaseFile(c.runner!, dir, opts);
}

/** The update status of a case and its verdict (f09 4.3). */
export function updateStatus(v: Verdict): string {
  if (v.verdict === "skip" || v.verdict === "invalid" || v.verdict === "pass") return v.verdict;
  return v.newExpect === null || v.newExpect === undefined ? "fail" : "updated";
}

/** Run the cases of a file, with the verdicts and the update statuses (f09 3, 4). */
export function runCases(cf: Extract<CaseFile, { ok: true }>, file: string, opts: RunnerOptions, fileIsArray = false): { verdicts: Verdict[]; statuses: string[] } {
  const verdicts = cf.cases.map((x) => runCase(x, file, opts));
  for (const v of verdicts) if (v.newExpect !== undefined && v.newExpect !== null && tooDeep(fileIsArray, v.newExpect)) v.newExpect = null;
  const statuses = verdicts.map((v, i) => (opts.update === true && cf.cases[i].runner === undefined && cf.cases[i].render === undefined ? updateStatus(v) : v.verdict));
  return { verdicts, statuses };
}

/** The verdict of a case of a runner case's inner file, as 3.4 writes it. */
export interface RunnerEntry {
  name: string;
  status: string;
  differs?: string[];
  fixture?: (string | number)[];
  message?: string;
}

/** What a runner case finds (f09 3.3): its inner file is invalid, or the verdicts, the report and the text. */
export type RunnerResult = { invalid: true; reason: string } | { invalid: false; verdicts: RunnerEntry[]; report: string; text: string | null };

/** Run the inner file of a runner case (f09 3.2). */
export function runnerResult(c: Case, opts: RunnerOptions = {}): RunnerResult {
  const cf = innerFile(c);
  if (!cf.ok) return { invalid: true, reason: cf.error };
  // f09 5.4 item 1: a program case file is run as in an ordinary run, even in update mode.
  const inner: RunnerOptions = { update: c.update === true && c.program !== true };
  const isArray = typeof c.runner === "string" ? c.runner.trimStart().startsWith("[") : Array.isArray(c.runner);
  const { verdicts, statuses } = runCases(cf, "", inner, isArray);
  const entries = verdicts.map((x, i): RunnerEntry => {
    const e: RunnerEntry = { name: x.name, status: statuses[i] };
    if (statuses[i] === "fail") e.differs = (x.differences ?? []).map((y) => y.field);
    if (statuses[i] === "invalid" && x.fixture !== undefined) {
      e.fixture = x.fixture;
      e.message = x.message;
    }
    return e;
  });
  const report = verdicts.map((x, i) => reportLines(x, statuses[i])).join("");
  const text = typeof c.runner === "string" ? (inner.update === true ? updatedText(c.runner, cf, verdicts, statuses) : null) ?? c.runner : null;
  return { invalid: false, verdicts: entries, report, text };
}

/** A runner case (f09 3): its inner file's verdicts, report and updated text against what it expects. */
function runnerCase(c: Case, v: (verdict: VerdictKind, extra?: Partial<Verdict>) => Verdict, opts: RunnerOptions): Verdict {
  const ex = c.expect as Extract<Case["expect"], { kind: "runner" }>;
  const r = runnerResult(c, opts);
  if (r.invalid) {
    return ex.invalid === true ? v("pass") : v("fail", { differences: [{ field: "invalid", expected: false, found: r.reason, e: "false", f: "true" }] });
  }
  if (ex.invalid === true) return v("fail", { differences: [{ field: "invalid", expected: true, found: false, e: "true", f: "false" }] });
  const d: Difference[] = [];
  const want = ex.verdicts ?? [];
  const got = r.verdicts;
  const same =
    got.length === want.length &&
    want.every((w, i) => {
      const g = got[i];
      if (w.name !== g.name || w.status !== g.status) return false;
      if (w.differs !== undefined && JSON.stringify(w.differs) !== JSON.stringify(g.differs ?? [])) return false;
      if (w.fixture !== undefined && JSON.stringify(w.fixture) !== JSON.stringify(g.fixture)) return false;
      if (w.message !== undefined && w.message !== g.message) return false;
      return true;
    });
  if (!same) d.push({ field: "verdicts", expected: want, found: got });
  if (ex.report !== undefined && r.report !== ex.report) d.push({ field: "report", expected: ex.report, found: r.report });
  if (ex.text !== undefined && r.text !== ex.text) d.push({ field: "text", expected: ex.text, found: r.text });
  return d.length === 0 ? v("pass") : v("fail", { differences: d });
}

/** A JSON value of a text, with the offsets of its first and last code units (end exclusive). */
interface Spot {
  start: number;
  end: number;
  members?: { key: string; value: Spot }[];
  elements?: Spot[];
}

/** Scan JSON text that has been read already, keeping where each value stands (f09 4.5). */
function scanJson(text: string): Spot {
  let i = 0;
  const ws = () => {
    while (i < text.length && " \t\n\r".includes(text[i])) i++;
  };
  const str = (): string => {
    const start = i;
    i++;
    while (text[i] !== '"') i += text[i] === "\\" ? 2 : 1;
    i++;
    return JSON.parse(text.slice(start, i)) as string;
  };
  const value = (): Spot => {
    ws();
    const start = i;
    const ch = text[i];
    if (ch === "{") {
      i++;
      const members: { key: string; value: Spot }[] = [];
      ws();
      if (text[i] === "}") i++;
      else {
        for (;;) {
          ws();
          const key = str();
          ws();
          i++; // :
          members.push({ key, value: value() });
          ws();
          if (text[i++] === "}") break;
        }
      }
      return { start, end: i, members };
    }
    if (ch === "[") {
      i++;
      const elements: Spot[] = [];
      ws();
      if (text[i] === "]") i++;
      else {
        for (;;) {
          elements.push(value());
          ws();
          if (text[i++] === "]") break;
        }
      }
      return { start, end: i, elements };
    }
    if (ch === '"') {
      str();
      return { start, end: i };
    }
    while (i < text.length && !" \t\n\r,]}".includes(text[i])) i++;
    return { start, end: i };
  };
  return value();
}

/** The case objects of a scanned file, in order: the elements of an array, or of the member `cases`. */
function caseSpots(root: Spot): { spots: Spot[]; depth: number } {
  if (root.elements !== undefined) return { spots: root.elements, depth: 2 };
  const cases = root.members?.find((m) => m.key === "cases")?.value;
  return { spots: cases?.elements ?? [], depth: 3 };
}

/** The nesting of data: 0 for a scalar, one more for each array or object around the deepest. */
function nesting(v: Json): number {
  if (Array.isArray(v)) return 1 + Math.max(0, ...v.map(nesting));
  if (isObject(v)) return 1 + Math.max(0, ...Object.values(v).map(nesting));
  return 0;
}

/** f09 4.3: a new `expect` that would nest the file deeper than 03 3.10.1 item 4 allows is not written. */
export function tooDeep(fileIsArray: boolean, e: Json): boolean {
  return (fileIsArray ? 2 : 3) + nesting(e) > 1024;
}

/** The text of a file after update mode, or null when nothing is written (f09 4.5). */
export function updatedText(text: string, _cf: Extract<CaseFile, { ok: true }> | null, verdicts: Verdict[], statuses: string[]): string | null {
  if (!statuses.includes("updated")) return null;
  const { spots } = caseSpots(scanJson(text));
  const edits: { at: number; end: number; text: string }[] = [];
  statuses.forEach((st, k) => {
    if (st !== "updated") return;
    const spot = spots[k];
    const form = jsonForm(verdicts[k].newExpect as Json);
    const member = spot.members!.find((m) => m.key === "expect");
    if (member !== undefined) edits.push({ at: member.value.start, end: member.value.end, text: form });
    else {
      // After the last member in the order of the text.
      const last = spot.members![spot.members!.length - 1].value;
      edits.push({ at: last.end, end: last.end, text: `, "expect": ${form}` });
    }
  });
  edits.sort((a, b) => b.at - a.at);
  let out = text;
  for (const e of edits) out = out.slice(0, e.at) + e.text + out.slice(e.end);
  return out;
}

/** A program case file (f09 5.2): parse, check in the default environment, run, then read the value as JSON. */
export function programCaseFile(text: string, dir: string): CaseFile {
  const bad = (error: string): CaseFile => ({ ok: false, error, names: [] });
  const env = createEnv({ limits: { timeoutMs: null } });
  const compiled = env.tryCompile(text, { check: true });
  if (!compiled.ok) return bad(`the program does not compile: ${compiled.diagnostics[0].code}: ${compiled.diagnostics[0].message}`);
  const o = compiled.program.execute();
  if (!o.ok) return bad(`the program fails: ${o.diagnostic.code}: ${o.diagnostic.message}`);
  // 5.3: from the value to the JSON. A function cannot be data.
  const cls = env.classifier;
  let error: string | null = null;
  const walk = (x: unknown, depth: number): Json => {
    x = readHost(x);
    if (depth > 1024 && error === null) error = "the value nests deeper than 1,024 levels";
    switch (cls.kind(x)) {
      case "null":
        return null;
      case "bool":
        return x as boolean;
      case "number":
        return x as number;
      case "string":
        return x as string;
      case "list":
        return (x as unknown[]).map((y) => walk(y, depth + 1));
      case "map": {
        const out: JsonObject = Object.create(null);
        for (const k of mapKeys(x as object)) out[k] = walk(mapGet(x as object, k), depth + 1);
        return out;
      }
      default:
        if (error === null) error = `the value holds a ${cls.kind(x)}, which is not data`;
        return null;
    }
  };
  const json = walk(o.value, 1);
  if (error !== null) return bad(error);
  return validateCaseFile(json, dir);
}

// ---- package F12: the tools (f12 7) -------------------------------------------

/** A format, completion or hover case (f12 7.1 to 7.3). */
function toolCase(c: Case, v: (verdict: VerdictKind, extra?: Partial<Verdict>) => Verdict): Verdict {
  const t = c.tool!;
  if (t.kind === "format" || t.kind === "formatTemplate") {
    const d = formatDifferences(c);
    return d.length === 0 ? v("pass") : v("fail", { differences: d });
  }
  const r = assistOutcome(c);
  const ex = c.expect;
  if (!r.ok) {
    const o: Outcome = { kind: "failure", phase: "declare", diagnostics: r.diagnostics };
    const d = ex.kind === "failure" ? compare(c, o) : [{ field: "outcome", expected: t.kind, found: describeFailure(o) }];
    return d.length === 0 ? v("pass") : v("fail", { differences: d });
  }
  if (ex.kind === "failure") return v("fail", { differences: [{ field: "outcome", expected: `failure of phase ${ex.phase}`, found: t.kind }] });
  if (ex.kind === "completion") {
    const got = r.completion!;
    const same =
      JSON.stringify(got.span) === JSON.stringify(ex.span) &&
      got.items.length === ex.items.length &&
      ex.items.every((w, i) => {
        const g = got.items[i];
        return w.label === g.label && w.kind === g.kind && (w.detail === undefined || w.detail === g.detail) && (w.insert === undefined || w.insert === g.insert);
      });
    return same ? v("pass") : v("fail", { differences: [{ field: "completion", expected: { span: ex.span, items: ex.items }, found: got }] });
  }
  if (ex.kind === "hover") {
    const got = r.hover ?? null;
    let same: boolean;
    if (ex.hover === null || got === null) same = ex.hover === got;
    else {
      const w = ex.hover;
      same = Object.entries(w).every(([k, x]) => JSON.stringify(x) === JSON.stringify((got as unknown as Record<string, unknown>)[k]));
    }
    return same ? v("pass") : v("fail", { differences: [{ field: "hover", expected: ex.hover, found: got }] });
  }
  return v("fail", { differences: [{ field: "outcome", expected: "a result of the tools", found: t.kind }] });
}

/** The completion or hover of a case, in its environment (f12 7.2 item 1). */
export function assistOutcome(c: Case): { ok: true; completion?: CompletionResult; hover?: HoverResult | null } | { ok: false; diagnostics: Diagnostic[] } {
  const t = c.tool!;
  const opaque: Record<string, OpaqueType> = {};
  for (const n of c.opaque ?? []) opaque[n] = { test: () => false, methods: {} };
  for (const [key, st] of Object.entries(c.methods ?? {})) {
    const dot = key.indexOf(".");
    const ty = key.slice(0, dot);
    if (opaque[ty] === undefined) opaque[ty] = { test: () => false, methods: {} };
    opaque[ty].methods![key.slice(dot + 1)] = { type: st.type ?? "() => any", call: () => null };
  }
  const functions: Record<string, HostFunction> = {};
  for (const [k, st] of Object.entries(c.functions ?? {})) functions[k] = { type: st.type ?? "() => any", call: () => null };
  let env: Env;
  try {
    env = createEnv({ version: c.version, vars: c.env, opaque, opaqueNames: c.opaque ?? [], functions, removed: c.removed, groups: c.groups, types: c.types as NamedTypeDecl[] | undefined });
    if (c.returns !== undefined && c.returns !== "condition") env.declaredType(c.returns);
  } catch (x) {
    if (!(x instanceof RisorError)) throw x;
    return { ok: false, diagnostics: [...x.diagnostics] };
  }
  const caret = [...t.text].indexOf("\u2038");
  const text = [...t.text].filter((ch) => ch !== "\u2038").join("");
  const src = new Source(text);
  const pos = src.position(Math.min(caret, src.length));
  if (t.kind === "complete") return { ok: true, completion: complete(env, text, pos, c.returns) };
  return { ok: true, hover: hover(env, text, pos, c.returns) };
}

/** The differences of a format case (f12 7.1 items 3 to 5). */
function formatDifferences(c: Case): Difference[] {
  const t = c.tool!;
  const isT = t.kind === "formatTemplate";
  const fmt = (s: string) => (isT ? formatTemplate(s) : formatProgram(s));
  const r = fmt(t.text);
  const ex = c.expect;
  const d: Difference[] = [];
  if (!r.ok) {
    const o: Outcome = { kind: "failure", phase: "parse", diagnostics: r.diagnostics, source: isT ? parseTemplateSource(t.text) : new Source(t.text) };
    if (ex.kind === "failure") d.push(...compare(c, o));
    else d.push({ field: "outcome", expected: "a format result", found: describeFailure(o) });
    return d;
  }
  if (ex.kind === "failure") d.push({ field: "outcome", expected: `failure of phase ${ex.phase}`, found: "a format result" });
  else if (ex.kind === "formatted" && ex.formatted !== r.text) d.push({ field: "formatted", expected: ex.formatted, found: r.text });
  // The laws (f12 2.2, 7.1 item 4).
  const again = fmt(r.text);
  if (!again.ok || again.text !== r.text) d.push({ field: "idempotent", expected: r.text, found: again.ok ? again.text : "a parse failure" });
  if (!sameTree(t.text, r.text, isT)) d.push({ field: "tree", expected: "the tree of the source", found: "another tree" });
  if (c.vars !== undefined && !isT) {
    const a = lawRun(c, t.text);
    const b = lawRun(c, r.text);
    if (a !== b) d.push({ field: "run", expected: a, found: b });
  }
  return d;
}

function parseTemplateSource(text: string): Source {
  return new Source(text, { keepBom: true });
}

/** The outcome of a law run: phase, type, value, code, class, message, steps and units. */
function lawRun(c: Case, text: string): string {
  try {
    const env = createEnv({ vars: c.env });
    const r = env.tryCompile(text);
    if (!r.ok) return JSON.stringify(["compile", r.diagnostics.map((x) => [x.phase, x.code, x.class ?? null, x.message])]);
    const vars = c.vars ?? {};
    const o = r.program.execute((name) => (Object.prototype.hasOwnProperty.call(vars, name) ? hostValue(vars[name]) : undefined));
    if (o.ok) return JSON.stringify(["value", r.program.type, valueForm(o.value, CLASSIFIER, opaqueOf), o.usage.steps, o.usage.allocated]);
    return JSON.stringify(["run", r.program.type, o.diagnostic.code, o.diagnostic.class ?? null, o.diagnostic.message, o.usage.steps, o.usage.allocated]);
  } catch (x) {
    if (x instanceof RisorError) return JSON.stringify(["declare", x.diagnostics.map((y) => y.code)]);
    throw x;
  }
}

/** Two texts parse to equal trees (f12 2.2 item 1); for a template, equal segments. */
function sameTree(a: string, b: string, template: boolean): boolean {
  if (template) {
    const x = parseTemplateText(a);
    const y = parseTemplateText(b);
    if (!x.ok || !y.ok) return false;
    const sx = x.template.segments;
    const sy = y.template.segments;
    return sx.length === sy.length && sx.every((s, i) => (s.kind === "text" ? sy[i].kind === "text" && s.value === (sy[i] as { value: string }).value : sy[i].kind === "part" && treeEqual(s.expr, (sy[i] as { expr: unknown }).expr)));
  }
  const x = parse(a);
  const y = parse(b);
  return x.ok && y.ok && treeEqual(x.program, y.program);
}
