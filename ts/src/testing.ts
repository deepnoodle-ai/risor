// "@deepnoodle/risor/testing": the binding of package F9 (f09 7.1, 7.3). It runs a case
// file inside the host's own test framework, and imports none.

import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Env } from "./host/api.ts";
import { loadCaseFile as readCaseFile, validateCaseFile } from "./suite/casefile.ts";
import type { Case as SuiteCase, CaseFile as SuiteFile, ReadOptions } from "./suite/casefile.ts";
import { JsonError, readJson } from "./suite/json.ts";
import type { Json } from "./suite/json.ts";
import { programCaseFile, reportLines, runCase, tooDeep, updatedText, updateStatus } from "./suite/runner.ts";
import type { Verdict as SuiteVerdict } from "./suite/runner.ts";

/** The options of a case file (f09 7.1). */
export interface CaseOptions {
  /** Update mode (f09 4). By default `process.env.RISOR_UPDATE === "1"` where `process` exists. */
  update?: boolean;
  /** Variables for the names a case's `vars` does not give (03 3.10.9 item 3). They are not fixtures. */
  vars?: (name: string) => unknown;
  /** A reader for `file`: the text of a reference. By default a path relative to the file. */
  read?: (ref: string) => string;
  /** Only the cases whose names contain this text are run (f09 8.1). */
  filter?: string;
  /**
   * The base environment (f09 7.1, 7.3 item 5): the file is checked against it
   * when it is loaded (a stub with no type is valid when it declares the name,
   * 03 3.10.10 item 11), and every case runs in a child of it. By default the
   * default environment.
   */
  base?: Env;
}

/** A difference of a failed case: its name, and E and F (f09 6.2). */
export interface Difference {
  readonly name: string;
  readonly expected: string;
  readonly found: string;
}

/** The verdict of a case (f09 7.1). */
export interface Verdict {
  /** The verdict, or in update mode the update status (f09 4.3). */
  readonly status: "pass" | "fail" | "skip" | "invalid" | "updated";
  readonly differences: readonly Difference[];
  readonly fixture?: readonly (string | number)[];
  readonly message?: string;
  readonly reason?: string;
  /** The `expect` update mode would write, or null. */
  readonly expect: unknown;
  /** The lines of the report for this case (f09 6.1). */
  readonly report: string;
}

export interface Case {
  readonly name: string;
  /** Run the case in a child of the base of its load (f09 7.3 item 5). */
  run(): Verdict;
}

export interface CaseFile {
  readonly cases: readonly Case[];
  /** The new text of the file after update mode, or null when nothing is written (f09 4.5). */
  updatedText(): string | null;
}

type TestFn = ((name: string, fn: () => void) => void) & { skip?: (name: string, fn: () => void) => void };

function updateOn(options: CaseOptions): boolean {
  if (options.update !== undefined) return options.update;
  const p = (globalThis as { process?: { env?: Record<string, string | undefined> } }).process;
  return p?.env?.RISOR_UPDATE === "1";
}

/** A file: its cases, and what update mode writes back. */
class File implements CaseFile {
  readonly cases: Case[];
  private readonly verdicts: (SuiteVerdict | undefined)[];
  private readonly statuses: (string | undefined)[];
  private readonly text: string | null;
  private readonly isArray: boolean;
  private readonly onDone: ((text: string) => void) | null;

  constructor(f: Extract<SuiteFile, { ok: true }>, text: string | null, isArray: boolean, options: CaseOptions, writable: boolean, onDone: ((text: string) => void) | null) {
    const update = writable && updateOn(options);
    const selected = f.cases.filter((c) => options.filter === undefined || c.name.includes(options.filter));
    this.verdicts = selected.map(() => undefined);
    this.statuses = selected.map(() => undefined);
    this.text = text;
    this.isArray = isArray;
    this.onDone = update ? onDone : null;
    // Positions of the selected cases among all, for the text.
    const index = selected.map((c) => f.cases.indexOf(c));
    this.all = f.cases.length;
    this.index = index;
    this.cases = selected.map((c, k) => ({ name: c.name, run: () => this.runOne(c, k, options.base, options, update) }));
    // A case with `skip` is never run: its status is known now.
    selected.forEach((c, k) => {
      if (c.skip !== undefined) {
        this.verdicts[k] = { file: "", name: c.name, verdict: "skip", reason: c.skip };
        this.statuses[k] = "skip";
      }
    });
    this.skips = new Set(selected.filter((c) => c.skip !== undefined).map((c) => c.name));
  }

  private readonly skips: Set<string>;

  /** Whether a case has `skip` (f09 7.3 item 2). */
  skipped(name: string): boolean {
    return this.skips.has(name);
  }

  private readonly all: number;
  private readonly index: number[];

  private runOne(c: SuiteCase, k: number, base: Env | undefined, options: CaseOptions, update: boolean): Verdict {
    const v = runCase(c, "", { update, base, vars: options.vars });
    if (v.newExpect !== undefined && v.newExpect !== null && tooDeep(this.isArray, v.newExpect)) v.newExpect = null;
    const status = update && c.runner === undefined && c.render === undefined ? updateStatus(v) : v.verdict;
    this.verdicts[k] = v;
    this.statuses[k] = status;
    if (this.onDone !== null && this.statuses.every((s) => s !== undefined)) {
      const t = this.updatedText();
      if (t !== null) this.onDone(t);
    }
    return {
      status: status as Verdict["status"],
      differences: (v.differences ?? []).map((d) => ({ name: d.field, expected: d.e ?? "", found: d.f ?? "" })),
      fixture: v.fixture,
      message: v.message,
      reason: v.reason,
      expect: v.newExpect ?? null,
      report: reportLines(v, status),
    };
  }

  updatedText(): string | null {
    if (this.text === null) return null;
    const verdicts: SuiteVerdict[] = [];
    const statuses: string[] = [];
    for (let i = 0; i < this.all; i++) {
      const k = this.index.indexOf(i);
      const v = k < 0 ? undefined : this.verdicts[k];
      verdicts.push(v ?? ({ file: "", name: "", verdict: "skip" } as SuiteVerdict));
      statuses.push(k < 0 ? "skip" : (this.statuses[k] ?? "skip"));
    }
    return updatedText(this.text, null, verdicts, statuses);
  }
}

function invalid(f: SuiteFile): asserts f is Extract<SuiteFile, { ok: true }> {
  if (!f.ok) throw new Error(`invalid case file: ${f.error}`);
}

function readOptions(options: CaseOptions, update: boolean, dir: string): ReadOptions {
  const e = options.base?.environment;
  const declares = (name: string) => e !== undefined && (e.functions.has(name) || e.methods.has(name));
  return { noExpect: update, declares, read: options.read ?? ((ref) => readFileSync(join(dir, ...ref.split("/")), "utf8")) };
}

/** Load a case file from a path: a program case file when the name ends in `.test.risor`, else JSON (f09 7.3). */
export function loadCaseFile(path: string, options: CaseOptions = {}): CaseFile {
  const text = readFileSync(path, "utf8");
  if (path.endsWith(".test.risor")) {
    const f = programCaseFile(text, dirname(path));
    invalid(f);
    return new File(f, null, false, options, false, null);
  }
  const update = updateOn(options);
  const f = parseText(text, dirname(path), options, update);
  invalid(f);
  return new File(f, text, text.trimStart().startsWith("["), options, true, (t) => writeFileSync(path, t));
}

function parseText(text: string, dir: string, options: CaseOptions, update: boolean): SuiteFile {
  let json: Json;
  try {
    json = readJson(text);
  } catch (e) {
    if (e instanceof JsonError) return { ok: false, error: `not JSON: ${e.message}`, names: [] };
    throw e;
  }
  return validateCaseFile(json, dir, readOptions(options, update, dir));
}

/** A case file from parsed data (f09 7.3). Update mode gives no text for it. */
export function caseFile(data: unknown, options: CaseOptions = {}): CaseFile {
  let json: Json;
  try {
    json = readJson(JSON.stringify(data));
  } catch (e) {
    throw new Error(`invalid case file: ${(e as Error).message}`);
  }
  const f = validateCaseFile(json, ".", readOptions(options, updateOn(options), "."));
  invalid(f);
  return new File(f, null, Array.isArray(json), options, true, null);
}

/** A case file from text: a program when `program` is true, else JSON (f09 7.3). */
export function caseFileText(text: string, program: boolean, options: CaseOptions = {}): CaseFile {
  if (program) {
    const f = programCaseFile(text, ".");
    invalid(f);
    return new File(f, null, false, options, false, null);
  }
  const f = parseText(text, ".", options, updateOn(options));
  invalid(f);
  return new File(f, text, text.trimStart().startsWith("["), options, true, null);
}

/** One test for each case, named by the case (f09 7.3 items 1 and 2). */
export function defineCases(test: TestFn, file: CaseFile): void {
  for (const c of file.cases) {
    const fn = () => {
      const v = c.run();
      if (v.status === "fail" || v.status === "invalid") throw new Error(v.report);
    };
    const skipped = file instanceof File && file.skipped(c.name);
    if (skipped) {
      if (typeof test.skip === "function") test.skip(c.name, fn);
      continue;
    }
    test(c.name, fn);
  }
}
