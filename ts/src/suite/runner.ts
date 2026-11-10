// The case runner (03 3.10.5 to 3.10.8).
//
// Stage S1 has the declare stage (by the grammar of type text), the parser,
// and no evaluator and no static rules yet. A case whose outcome depends on
// what S1 lacks gets the verdict `skip` with the reason.

import { readFileSync, readdirSync, statSync } from "node:fs";
import { basename, join } from "node:path";
import { declare } from "../host/env.ts";
import type { Declarations } from "../host/env.ts";
import type { Diagnostic } from "../syntax/diagnostic.ts";
import { parse } from "../syntax/parser.ts";
import type { Source } from "../syntax/source.ts";
import { loadCaseFile, validateCaseFile } from "./casefile.ts";
import type { Case, CaseFile, ExpectDiagnostic } from "./casefile.ts";
import { JsonError, isObject, readJson } from "./json.ts";
import type { Json } from "./json.ts";

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

/** The static codes (01 14.4); every implementation reports them (ruling A1). */
export const STATIC_CODES: ReadonlySet<string> = new Set([
  "undefined-name", "undefined-function", "call-of-parameter", "lambda-to-lambda", "not-a-value",
  "arity", "duplicate-name", "shadows-namespace", "duplicate-key", "duplicate-case", "unreachable-arm",
]);

const LIMIT_CODES: ReadonlySet<string> = new Set([
  "steps-exceeded", "allocation-exceeded", "deadline-exceeded", "depth-exceeded", "cancelled",
]);

export interface RunnerOptions {
  /** The library function names of the base environment. */
  library: ReadonlySet<string>;
}

type Outcome =
  | { kind: "failure"; phase: Diagnostic["phase"]; diagnostics: Diagnostic[]; source?: Source }
  | { kind: "result" };

/** Run one case and give its verdict. */
export function runCase(c: Case, file: string, opts: RunnerOptions): Verdict {
  const v = (verdict: VerdictKind, extra: Partial<Verdict> = {}): Verdict => ({ file, name: c.name, verdict, ...extra });
  if (c.skip !== undefined) return v("skip", { reason: c.skip });
  const ex = c.expect;
  // 03 3.10.8 item 5 and ruling A3: no type checker.
  if (ex.kind === "result" && (ex.type !== undefined || ex.sites !== undefined)) {
    return v("skip", { reason: "needs the type checker (type or sites)" });
  }
  if (ex.kind === "failure" && ex.phase === "check" && ex.diagnostics.some((d) => !STATIC_CODES.has(d.code))) {
    return v("skip", { reason: "needs the type checker (a check code outside the static rules)" });
  }

  // Step 2: declare.
  const functions: Record<string, string> = {};
  for (const [k, s] of Object.entries(c.functions ?? {})) functions[k] = s.type!;
  const decls: Declarations = {
    version: c.version, env: c.env, opaque: c.opaque, methods: c.methods, functions,
    reject: c.reject, returns: c.returns, limits: c.limits,
  };
  const d = declare(decls, opts.library);
  let outcome: Outcome;
  if (!d.ok) {
    outcome = { kind: "failure", phase: "declare", diagnostics: d.diagnostics };
  } else {
    // Step 3: parse.
    const p = parse(c.source);
    if (!p.ok) outcome = { kind: "failure", phase: "parse", diagnostics: p.diagnostics, source: p.source };
    else {
      // Steps 4 and 5 come with stage S2. Judge what can be judged now.
      if (ex.kind === "failure" && (ex.phase === "parse" || ex.phase === "declare")) {
        return v("fail", { differences: [{ field: "outcome", expected: `failure of phase ${ex.phase}`, found: "the program parses" }] });
      }
      const needsRun =
        c.vars !== undefined ||
        (ex.kind === "result" && (ex.value !== undefined || ex.truthy !== undefined || ex.steps !== undefined || ex.allocated !== undefined)) ||
        (ex.kind === "failure" && ex.phase === "run");
      if (ex.kind === "failure" && ex.phase === "check") return v("skip", { reason: "needs the static rules (S2)" });
      if (needsRun) return v("skip", { reason: "needs the evaluator (S2)" });
      // A result with no field to compare: the program parses. The static
      // rules of step 4 come with S2.
      outcome = { kind: "result" };
    }
  }
  const diffs = compare(c, outcome);
  return diffs.length === 0 ? v("pass") : v("fail", { differences: diffs });
}

function compare(c: Case, o: Outcome): Difference[] {
  const ex = c.expect;
  if (ex.kind === "result") {
    if (o.kind !== "result") {
      return [{ field: "outcome", expected: "a result", found: describeFailure(o) }];
    }
    return [];
  }
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
  return ok ? [] : [{ field: "diagnostics", expected: ex.diagnostics, found }];
}

function describeFailure(o: Outcome): string {
  if (o.kind !== "failure") return "a result";
  const d = o.diagnostics[0];
  const span = d.span !== undefined && o.source !== undefined ? ` at ${JSON.stringify(o.source.lineSpan(d.span))}` : "";
  return `failure of phase ${o.phase}: ${d.code}${span} (${d.message})`;
}

interface FoundDiag {
  code: string;
  span?: [number, number, number, number];
  message: string;
  path?: (string | number)[];
}

function foundDiag(d: Diagnostic, source?: Source): FoundDiag {
  const f: FoundDiag = { code: d.code, message: d.message };
  if (d.span !== undefined && source !== undefined) f.span = source.lineSpan(d.span);
  if (d.path !== undefined) f.path = d.path;
  return f;
}

function diagMatches(e: ExpectDiagnostic, f: FoundDiag | undefined): boolean {
  if (f === undefined) return false;
  if (e.code !== f.code) return false;
  // `class` is compared only after a check (03 3.10.7 item 8): never in S1.
  if (e.span !== undefined && !LIMIT_CODES.has(e.code)) {
    if (f.span === undefined || e.span.some((n, i) => n !== f.span![i])) return false;
  }
  if (e.message !== undefined && e.message !== f.message) return false;
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
  return verdictsOf(loadCaseFile(path), file, opts);
}

function verdictsOf(cf: CaseFile, file: string, opts: RunnerOptions): Verdict[] {
  if (!cf.ok) {
    const names = cf.names.length > 0 ? cf.names : ["(file)"];
    return names.map((name) => ({ file, name, verdict: "invalid" as const, reason: cf.error }));
  }
  return cf.cases.map((c) => runCase(c, file, opts));
}

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
