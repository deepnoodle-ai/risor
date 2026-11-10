// The case runner (03 3.10.5 to 3.10.8).
//
// The runner builds each case's environment through the host API, with its
// stubs as host functions, and runs every step this implementation has: the
// declare stage, the parser, the static rules (no type checker yet) and the
// evaluator. A case that needs a library entry this build does not implement
// gets `skip`, with the entries named.

import { readFileSync, readdirSync, statSync } from "node:fs";
import { basename, dirname, join } from "node:path";
import { createEnv, HostError, RisorError, toJSON } from "../host/api.ts";
import type { Env, HostFunction, OpaqueType } from "../host/api.ts";
import type { NamedTypeDecl } from "../host/env.ts";
import type { Diagnostic } from "../syntax/diagnostic.ts";
import type { Source } from "../syntax/source.ts";
import { parseType } from "../types/typetext.ts";
import { plainEqual, truthy } from "../value/value.ts";
import { typeEqual } from "../types/type.ts";
import type { SchemaOption } from "../types/schema.ts";
import { ParsedProgram, Template, reportJSON } from "../host/api.ts";
import type { AnalysisReport } from "../host/api.ts";
import { parseTemplateText, templateProgram } from "../syntax/template.ts";
import { parse } from "../syntax/parser.ts";
import type { Classifier } from "../value/value.ts";
import { CONVERSION_STATIC, loadCaseFile, validateCaseFile } from "./casefile.ts";
import type { Case, CaseFile, ExpectDiagnostic, Stub } from "./casefile.ts";
import { JsonError, isObject, readJson } from "./json.ts";
import type { Json, JsonObject } from "./json.ts";

export type VerdictKind = "pass" | "fail" | "skip" | "invalid";

export interface Difference {
  field: string;
  expected: unknown;
  found: unknown;
}

export interface Verdict {
  file: string;
  name: string;
  verdict: VerdictKind;
  reason?: string;
  differences?: Difference[];
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
function stubFunction(st: Stub, cls: () => Classifier, unmatched: string[], name: string): HostFunction {
  const rows = st.calls.map((r) => ({ ...r, hostArgs: r.args?.map(hostValue), hostResult: r.result === undefined ? undefined : hostValue(r.result.value) }));
  return {
    type: st.type!,
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
      unmatched.push(name);
      throw new HostError(`no row of the stub ${name} matches the call`);
    },
  };
}

function needsRun(c: Case): boolean {
  const ex = c.expect;
  return (
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
  if (c.conform !== undefined) return conformCase(c, v);
  const checker = opts.checker !== false;
  const staticCodes = opts.staticCodes ?? STATIC_CODES;
  if (!checker) {
    // 03 3.11: a file that is run checked only is not run with the static rules alone.
    if (opts.checkOnly === true) return v("skip", { reason: "run checked only" });
    // 03 3.10.8 item 5: a runner with no type checker.
    if (ex.kind === "result" && (ex.type !== undefined || ex.sites !== undefined)) {
      return v("skip", { reason: "needs the type checker (type or sites)" });
    }
    if (ex.kind === "failure" && ex.phase === "check" && ex.diagnostics.some((d) => !staticCodes.has(d.code))) {
      return v("skip", { reason: "needs the type checker (a check code outside the static rules)" });
    }
  }
  // 03 3.10.8: "check": false compiles with the static rules alone, and
  // compares `class` for every code.
  if (c.check === false) return judge(c, v, false, true);
  if (!checker) return judge(c, v, false, false);
  // 03 3.10.8: checked, and again with the static rules alone, unless the
  // case needs the checker or is of suite/core-check/.
  const needsChecker =
    opts.checkOnly === true ||
    (ex.kind === "result" && (ex.type !== undefined || ex.sites !== undefined)) ||
    (ex.kind === "failure" && ex.phase === "check" && ex.diagnostics.some((d) => !staticCodes.has(d.code)));
  const first = judge(c, v, true, true);
  if (needsChecker || first.verdict !== "pass") return first;
  const second = judge(c, v, false, false);
  if (second.verdict === "pass") return first;
  return { ...second, reason: "with the static rules alone" };
}

function judge(c: Case, v: (verdict: VerdictKind, extra?: Partial<Verdict>) => Verdict, checker: boolean, allClasses: boolean): Verdict {
  const e = executeCase(c, needsRun(c), checker);
  RUN_CHECKED = allClasses;
  CURRENT_ENV = e.env;
  if (e.missing !== null) return v("skip", { reason: `needs library entries: ${e.missing.join(", ")}` });
  return finish(c, e.outcome, e.unmatched, v);
}

export interface Execution {
  outcome: Outcome;
  /** Stubs that a call matched no row of (03 3.10.4). */
  unmatched: string[];
  /** Library entries this build lacks, when the run needed them; else null. */
  missing: string[] | null;
  /** The case's environment, when it was built. */
  env: Env | null;
}

/** Run a case's steps (03 3.10.5), with or without step 5. Judges nothing. */
export function executeCase(c: Case, run: boolean, checker = true): Execution {
  // Step 2: declare.
  const unmatched: string[] = [];
  let env: Env | undefined;
  const cls = () => env!.classifier;
  const opaque: Record<string, OpaqueType> = {};
  for (const t of c.opaque ?? []) opaque[t] = { test: (x) => x instanceof OpaqueRef && x.type === t, methods: {} };
  for (const [key, st] of Object.entries(c.methods ?? {})) {
    const dot = key.indexOf(".");
    const t = key.slice(0, dot);
    const m = key.slice(dot + 1);
    // A method of a type that `opaque` does not declare: let declare report it.
    if (opaque[t] === undefined) opaque[t] = { test: () => false, methods: {} };
    opaque[t].methods![m] = stubFunction(st, cls, unmatched, key);
  }
  const functions: Record<string, HostFunction> = {};
  for (const [k, st] of Object.entries(c.functions ?? {})) functions[k] = stubFunction(st, cls, unmatched, k);
  const done = (outcome: Outcome): Execution => ({ outcome, unmatched, missing: null, env: env ?? null });
  let compiled: ReturnType<Env["tryCompile"]>;
  try {
    env = createEnv({
      version: c.version, vars: c.env, opaque, functions, reject: c.reject, limits: c.limits, opaqueNames: c.opaque ?? [], removed: c.removed,
      types: c.types as NamedTypeDecl[] | undefined,
      groups: c.groups,
      schemas: c.schemas as Record<string, { schema: unknown; options?: SchemaOption[] }> | undefined,
    });
    // The expected result is declared type text, resolved in every compile (03 3.3.1).
    if (!checker && c.returns !== undefined && c.returns !== "condition") env.declaredType(c.returns);
    // Steps 3 and 4: parse (a template in its mode, f02 4.1), then the check (or the static rules alone).
    let input: string | ParsedProgram = c.source;
    if (c.template !== undefined) {
      const t = parseTemplateText(c.source);
      if (!t.ok) return done({ kind: "failure", phase: "parse", diagnostics: t.diagnostics, source: t.source });
      input = new ParsedProgram(t.template.source, templateProgram(t.template, c.template));
    }
    compiled = env.tryCompile(input, { check: checker, expected: checker ? c.returns : undefined });
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
  const vars = c.vars ?? {};
  const o = compiled.program.execute((name) => (Object.prototype.hasOwnProperty.call(vars, name) ? hostValue(vars[name]) : undefined));
  return done(
    o.ok
      ? { kind: "result", ran: true, value: o.value, usage: o.usage, ...typeInfo }
      : { kind: "failure", phase: "run", diagnostics: [o.diagnostic], source: compiled.program.source, usage: o.usage, ...typeInfo },
  );
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
    return { ok: true, fn, asRun: runOf("as"), isRun: runOf("is"), value };
  } catch (x) {
    if (!(x instanceof RisorError)) throw x;
    return { ok: false, diagnostics: [...x.diagnostics] };
  }
}

function conformCase(c: Case, v: (verdict: VerdictKind, extra?: Partial<Verdict>) => Verdict): Verdict {
  const r = runConform(c);
  const ex = c.expect;
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
  if (ex.steps !== undefined && (r.asRun.steps !== ex.steps || r.isRun.steps !== ex.steps)) {
    d.push({ field: "steps", expected: ex.steps, found: { as: r.asRun.steps, is: r.isRun.steps } });
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
  return f;
}

function diagMatches(e: ExpectDiagnostic, f: FoundDiag | undefined): boolean {
  if (f === undefined) return false;
  if (e.code !== f.code) return false;
  // 03 3.10.7 item 8: `class` is compared, except for the five codes whose class
  // depends on the check, since this runner does not check.
  if (e.class !== undefined && (RUN_CHECKED || !CHECK_DEPENDENT.has(e.code)) && e.class !== f.class) return false;
  if (e.span !== undefined && !LIMIT_CODES.has(e.code)) {
    if (f.span === undefined || e.span.some((n, i) => n !== f.span![i])) return false;
  }
  if (e.message !== undefined && e.message !== f.message) return false;
  if (e.data !== undefined && !plainEqual(f.rawData, hostValue(e.data.value), CLASSIFIER)) return false;
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
  const staticCodes = dir === "conversion" || dir === "unions" ? CONVERSION_STATIC : undefined;
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

/** The packages this implementation offers: F1 to F7. */
export const OFFERED_PACKAGES: ReadonlySet<number> = new Set([1, 2, 3, 4, 5, 6, 7]);

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
