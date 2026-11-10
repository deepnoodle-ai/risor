// Case files (03 3.10): reading, the header rule, and validation.

import { readFileSync } from "node:fs";
import { basename, dirname, join } from "node:path";
import { JsonError, isObject, readJson } from "./json.ts";
import type { Json, JsonObject } from "./json.ts";
import { parseType } from "../types/typetext.ts";

export type LineSpan = [number, number, number, number];

export interface StubRow {
  args?: Json[];
  /** Present when the row answers with a value. */
  result?: { value: Json };
  error?: string;
  charge: number;
  allocate: number;
}

export interface Stub {
  type?: string;
  calls: StubRow[];
}

export interface ExpectDiagnostic {
  code: string;
  class?: string;
  /** Null in a load case: the diagnostic has no span (f10 10.2 item 5). */
  span?: LineSpan | null;
  path?: (string | number)[];
  message?: string;
  data?: { value: Json };
  /** Package F8: the trace of a run failure (f08 7.2). */
  trace?: TraceEntry[];
}

export interface TraceEntry {
  span: LineSpan;
  name: string;
  index: number | null;
}

/** An entry of `verdicts` (f09 3.4). */
export interface RunnerVerdict {
  name: string;
  status: string;
  differs?: string[];
  fixture?: (string | number)[];
  message?: string;
}

/** Package F8: the three renderings (f08 7.3, 7.4). */
export interface Renderings {
  line?: string;
  detail?: string;
  json?: string;
}

export type Expect =
  | {
      kind: "result";
      type?: string;
      /** Package F1: the description of the case's environment (f01 7.2). */
      description?: { value: Json };
      /** Package F6: the JSON text of the run's value (f06 11.2). */
      json?: string;
      /** Package F2: the expected report of the analysis (f02 4.2). */
      analysis?: JsonObject;
      sites?: { class: string; span: LineSpan }[];
      value?: { value: Json };
      truthy?: boolean;
      steps?: number;
      allocated?: number;
    }
  | {
      /** Package F4: a conform result (f04 9.2). */
      kind: "conform";
      conforms: boolean;
      path?: (string | number)[];
      message?: string;
      steps?: number;
      /** Package F8: the failures of ConformAll (f08 7.5). */
      failures?: { path: (string | number)[]; message?: string }[];
    }
  | ({ kind: "render" } & Renderings)
  | {
      /** Package F9: the verdicts a runner gives the inner file (f09 3.3). */
      kind: "runner";
      invalid?: true;
      verdicts?: RunnerVerdict[];
      report?: string;
      text?: string;
    }
  | {
      kind: "failure";
      phase: "declare" | "parse" | "check" | "run";
      diagnostics: ExpectDiagnostic[];
      steps?: number;
      allocated?: number;
      /** Package F8 (f08 7.3). */
      rendered?: Renderings;
    };

export interface Case {
  name: string;
  source: string;
  version?: string;
  env?: string;
  opaque?: string[];
  methods?: Record<string, Stub>;
  functions?: Record<string, Stub>;
  reject?: string[];
  returns?: string;
  /** Package F10: the expected result by schema (f10 10.1). */
  returnsSchema?: { schema: Json; options?: string[] };
  /** Package F10: the case is run as a load, steps 0 to 4 (f10 10.2). */
  load?: boolean;
  vars?: JsonObject;
  limits?: { steps?: number; allocation?: number; depth?: number };
  precondition?: boolean;
  /** 03 3.10.3: library entries the environment removes (`len`, `string.repeat`). */
  removed?: string[];
  /** Package F6: the groups the case's environment turns on (f06 11.1). */
  groups?: string[];
  /** Package F4: named types, in order (f04 9.1). */
  types?: ({ name: string; type: string } | { name: string; schema: Json; options?: string[] })[];
  /** Package F4: a case of the second kind (f04 9.2); `max` of package F8 (f08 7.5). */
  conform?: { type: string; value: Json; max?: number };
  /** Package F8: a case of the third kind (f08 7.4). */
  render?: { diagnostics: JsonObject[]; source?: string };
  /** Package F9: a runner case, its inner file as data or text (f09 3.1). */
  runner?: Json;
  update?: boolean;
  program?: boolean;
  /** The case's `expect` as written, for the report and for update mode (f09 4, 6). */
  rawExpect?: Json;
  /** The directory of the file that holds the case (f09 3.2 item 1). */
  dir?: string;
  /** 03 3.10.3: false compiles the case with the static rules alone. */
  check?: boolean;
  /** Package F2: the text is a template, in this mode (f02 4.1). */
  template?: "string" | "value";
  /** Package F1: variables declared by JSON Schema (f01 7.1). */
  schemas?: Record<string, { schema: Json; options?: string[] }>;
  skip?: string;
  expect: Expect;
}

export type CaseFile =
  | { ok: true; cases: Case[] }
  | { ok: false; error: string; names: string[] };

export class InvalidFile extends Error {}

const STATIC_CODE_SET = new Set([
  "undefined-name", "undefined-function", "call-of-parameter", "lambda-to-lambda", "not-a-value",
  "arity", "duplicate-name", "shadows-namespace", "duplicate-key", "duplicate-case", "unreachable-arm",
]);

/** f04 6.1 item 4: in suite/conversion/, two more codes are static at a program type. */
export const CONVERSION_STATIC: ReadonlySet<string> = new Set([...STATIC_CODE_SET, "untagged-union", "check-limit"]);

/** The static codes of suite/config/ (f10 10.3 item 3): Core's and `untagged-union`. */
export const CONFIG_STATIC: ReadonlySet<string> = new Set([...STATIC_CODE_SET, "untagged-union"]);

const LIMIT_CODES = new Set(["steps-exceeded", "allocation-exceeded", "deadline-exceeded", "depth-exceeded", "cancelled"]);

/** The fields a header and a case share (03 3.10.3). `precondition` is 3.10.3 too. */
const SHARED = ["source", "file", "version", "env", "opaque", "methods", "functions", "reject", "returns", "returnsSchema", "load", "vars", "limits", "precondition", "check", "schemas", "template", "removed", "types", "conform", "groups"];
const HEADER_FIELDS = new Set(["cases", ...SHARED]);
const CASE_FIELDS = new Set(["name", ...SHARED, "skip", "expect", "render", "runner", "update", "program"]);

/** Read and validate a case file on disk. */
export interface ReadOptions {
  /** Accept cases with no `expect` (the outcome command, suite/README.md item 1). */
  noExpect?: boolean;
  /** f09 7.1: the host's reader for `file`; by default a path relative to the file's directory. */
  read?: (ref: string) => string;
  /** 03 3.10.9 item 3: whether the base environment declares a host function, or a method `T.m`. */
  declares?: (name: string) => boolean;
}

export function loadCaseFile(path: string, opts: ReadOptions = {}): CaseFile {
  let text: string;
  try {
    text = readFileSync(path, "utf8");
  } catch (e) {
    return { ok: false, error: `cannot read ${path}: ${(e as Error).message}`, names: [] };
  }
  let json: Json;
  try {
    json = readJson(text);
  } catch (e) {
    if (e instanceof JsonError) return { ok: false, error: `not JSON: ${e.message}`, names: [] };
    throw e;
  }
  return validateCaseFile(json, dirname(path), opts);
}

/** Validate a parsed case file: an object with `cases`, or an array of cases. */
export function validateCaseFile(json: Json, dir: string, opts: ReadOptions = {}): CaseFile {
  try {
    let header: JsonObject = Object.create(null);
    let cases: Json;
    if (Array.isArray(json)) {
      cases = json;
    } else if (isObject(json) && "cases" in json) {
      header = json;
      cases = json.cases;
      for (const k of Object.keys(header)) {
        if (!HEADER_FIELDS.has(k)) throw new InvalidFile(`the header has a field ${k} that 3.10.3 does not list`);
      }
      checkShared(header, "the header");
      if (!Array.isArray(cases)) throw new InvalidFile("`cases` is not an array");
    } else {
      throw new InvalidFile("the file is neither an object with `cases` nor an array");
    }
    const out: Case[] = [];
    const seen = new Set<string>();
    for (const raw of cases) {
      if (!isObject(raw)) throw new InvalidFile("a case is not an object");
      const c = buildCase(header, raw, dir, opts);
      if (seen.has(c.name)) throw new InvalidFile(`the name ${JSON.stringify(c.name)} is used twice`);
      seen.add(c.name);
      out.push(c);
    }
    return { ok: true, cases: out };
  } catch (e) {
    if (e instanceof InvalidFile) return { ok: false, error: e.message, names: caseNames(json) };
    throw e;
  }
}

function caseNames(json: Json): string[] {
  const list = Array.isArray(json) ? json : isObject(json) && Array.isArray(json.cases) ? json.cases : [];
  return list.map((c, i) => (isObject(c) && typeof c.name === "string" ? c.name : `#${i + 1}`));
}

function fail(msg: string): never {
  throw new InvalidFile(msg);
}

function isInt(v: Json | undefined, min: number): v is number {
  return typeof v === "number" && Number.isInteger(v) && v >= min;
}

function stringArray(v: Json, what: string): string[] {
  if (!Array.isArray(v) || !v.every((x) => typeof x === "string")) fail(`${what} is not a list of strings`);
  return v as string[];
}

/** Check the JSON types of the fields a header and a case share. */
function checkShared(o: JsonObject, where: string): void {
  const str = (k: string) => {
    if (k in o && typeof o[k] !== "string") fail(`${where}: ${k} is not a string`);
  };
  str("source");
  str("env");
  str("returns");
  if ("returnsSchema" in o) {
    // f10 10.1: an object with `schema` and the optional `options`.
    const r = o.returnsSchema;
    if (!isObject(r) || !("schema" in r) || !Object.keys(r).every((k) => k === "schema" || k === "options")) fail(`${where}: returnsSchema is not an object with schema and options`);
    if ("options" in r) {
      const op = r.options;
      const names = ["closed", "absent-is-null", "defaults-applied"];
      if (!Array.isArray(op) || !op.every((x) => typeof x === "string" && names.includes(x)) || new Set(op).size !== op.length) fail(`${where}: the options of returnsSchema`);
    }
  }
  if ("load" in o && typeof o.load !== "boolean") fail(`${where}: load is not true or false`);
  if ("file" in o && (typeof o.file !== "string" || o.file.length === 0)) fail(`${where}: file is not a non-empty string`);
  if ("version" in o && (typeof o.version !== "string" || !/^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/.test(o.version))) {
    fail(`${where}: version is not of the form major.minor`);
  }
  if ("opaque" in o) stringArray(o.opaque, `${where}: opaque`);
  if ("reject" in o) stringArray(o.reject, `${where}: reject`);
  if ("methods" in o) {
    const m = o.methods;
    if (!isObject(m)) fail(`${where}: methods is not an object`);
    for (const [k, v] of Object.entries(m)) {
      if (!/^[A-Z][A-Za-z0-9_]+\.[A-Za-z_][A-Za-z0-9_]*$/.test(k)) fail(`${where}: methods has the key ${k}`);
      checkStub(v, `${where}: methods.${k}`);
    }
  }
  if ("functions" in o) {
    const f = o.functions;
    if (!isObject(f)) fail(`${where}: functions is not an object`);
    for (const [k, v] of Object.entries(f)) checkStub(v, `${where}: functions.${k}`);
  }
  if ("vars" in o && !isObject(o.vars)) fail(`${where}: vars is not an object`);
  if ("limits" in o) {
    const l = o.limits;
    if (!isObject(l)) fail(`${where}: limits is not an object`);
    for (const [k, v] of Object.entries(l)) {
      if (k === "steps" || k === "allocation") {
        if (!isInt(v, 0)) fail(`${where}: limits.${k} is not a whole number of 0 or more`);
      } else if (k === "depth") {
        if (!isInt(v, 1) || v > 1000) fail(`${where}: limits.depth is not from 1 to 1000`);
      } else fail(`${where}: limits has a field ${k}`);
    }
  }
  if ("precondition" in o && typeof o.precondition !== "boolean") fail(`${where}: precondition is not a boolean`);
  if ("check" in o && typeof o.check !== "boolean") fail(`${where}: check is not a boolean`);
  if ("types" in o) {
    // f04 9.1: the invalid-file rules of `types`.
    const ts = o.types;
    if (!Array.isArray(ts)) fail(`${where}: types is not an array`);
    for (const t of ts) {
      if (!isObject(t)) fail(`${where}: an entry of types is not an object`);
      for (const k of Object.keys(t)) if (!["name", "type", "schema", "options"].includes(k)) fail(`${where}: an entry of types has the field ${k}`);
      if (typeof t.name !== "string") fail(`${where}: an entry of types has no name, or a name that is not a string`);
      if (("type" in t) === ("schema" in t)) fail(`${where}: an entry of types needs exactly one of type and schema`);
      if ("type" in t && typeof t.type !== "string") fail(`${where}: the type of an entry of types is not a string`);
      if ("options" in t) {
        const op = t.options;
        const names = ["closed", "absent-is-null", "defaults-applied"];
        if ("type" in t || !Array.isArray(op) || !op.every((x) => typeof x === "string" && names.includes(x)) || new Set(op).size !== op.length) {
          fail(`${where}: the options of an entry of types`);
        }
      }
    }
  }
  if ("groups" in o) {
    // f06 11.1 item 4.
    const g = o.groups;
    if (!Array.isArray(g) || !g.every((x) => typeof x === "string") || new Set(g).size !== g.length) fail(`${where}: groups is not a list of strings, each once`);
  }
  if ("conform" in o) {
    const cf = o.conform;
    if (!isObject(cf) || typeof cf.type !== "string" || !("value" in cf) || !Object.keys(cf).every((k) => k === "type" || k === "value" || k === "max")) fail(`${where}: conform is not an object of type and value`);
    if ("max" in cf && !isInt(cf.max, 1)) fail(`${where}: conform.max is not a whole number of 1 or more`);
  }
  if ("removed" in o && !(Array.isArray(o.removed) && o.removed.every((x) => typeof x === "string"))) fail(`${where}: removed is not a list of strings`);
  if ("template" in o && o.template !== "string" && o.template !== "value") fail(`${where}: template is not "string" or "value"`);
  if ("schemas" in o) {
    // f01 7.1: the invalid-file rules of `schemas`.
    const sc = o.schemas;
    if (!isObject(sc)) fail(`${where}: schemas is not an object`);
    for (const [k, v] of Object.entries(sc)) {
      if (!isObject(v) || !("schema" in v)) fail(`${where}: schemas.${k} is not an object with schema`);
      for (const f of Object.keys(v)) if (f !== "schema" && f !== "options") fail(`${where}: schemas.${k} has the field ${f}`);
      if ("options" in v) {
        const op = v.options;
        const names = ["closed", "absent-is-null", "defaults-applied"];
        if (!Array.isArray(op) || !op.every((x) => typeof x === "string" && names.includes(x)) || new Set(op).size !== op.length) {
          fail(`${where}: schemas.${k}.options is not a list of the three option names, each once`);
        }
      }
    }
  }
}

function checkStub(v: Json, where: string): void {
  if (typeof v === "string") return;
  if (!isObject(v)) fail(`${where} is neither a string nor an object`);
  for (const k of Object.keys(v)) if (k !== "type" && k !== "calls") fail(`${where} has a field ${k}`);
  if ("type" in v && typeof v.type !== "string") fail(`${where}.type is not a string`);
  if ("calls" in v) {
    if (!Array.isArray(v.calls)) fail(`${where}.calls is not a list`);
    for (const row of v.calls) {
      if (!isObject(row)) fail(`${where}: a row is not an object`);
      for (const k of Object.keys(row)) {
        if (!["args", "result", "error", "charge", "allocate"].includes(k)) fail(`${where}: a row has a field ${k}`);
      }
      if ("args" in row && !Array.isArray(row.args)) fail(`${where}: args is not a list`);
      if ("error" in row && typeof row.error !== "string") fail(`${where}: error is not a string`);
      if (("result" in row) === ("error" in row)) fail(`${where}: a row needs exactly one of result and error`);
      if ("charge" in row && !isInt(row.charge, 0)) fail(`${where}: charge is not a whole number of 0 or more`);
      if ("allocate" in row && !isInt(row.allocate, 0)) fail(`${where}: allocate is not a whole number of 0 or more`);
    }
  }
}

function toStub(v: Json): Stub {
  if (typeof v === "string") return { type: v, calls: [] };
  const o = v as JsonObject;
  const calls: StubRow[] = ((o.calls as Json[] | undefined) ?? []).map((r) => {
    const row = r as JsonObject;
    const out: StubRow = { charge: (row.charge as number | undefined) ?? 0, allocate: (row.allocate as number | undefined) ?? 0 };
    if ("args" in row) out.args = row.args as Json[];
    if ("result" in row) out.result = { value: row.result };
    if ("error" in row) out.error = row.error as string;
    return out;
  });
  return { type: o.type as string | undefined, calls };
}

function buildCase(header: JsonObject, raw: JsonObject, dir: string, opts: ReadOptions): Case {
  for (const k of Object.keys(raw)) {
    if (!CASE_FIELDS.has(k)) fail(`a case has a field ${k} that 3.10.3 does not list`);
  }
  if (typeof raw.name !== "string") fail("a case has no name, or its name is not a string");
  const name = raw.name;
  const where = `case ${JSON.stringify(name)}`;
  checkShared(raw, where);
  if ("skip" in raw && typeof raw.skip !== "string") fail(`${where}: skip is not a string`);
  if (!("expect" in raw) && !opts.noExpect) fail(`${where} has no expect`);
  // f09 9.2 items 1 to 6: a runner case.
  for (const k of ["update", "program"]) {
    if (k in raw && typeof raw[k] !== "boolean") fail(`${where}: ${k} is not true or false`);
    if (k in raw && !("runner" in raw)) fail(`${where}: ${k} in a case with no runner`);
    if (raw[k] === true && typeof raw.runner !== "string") fail(`${where}: ${k} true beside a runner that is not a string`);
  }
  if ("runner" in raw) {
    const r = raw.runner;
    if (typeof r !== "string" && !Array.isArray(r) && !isObject(r)) fail(`${where}: runner is not a string, an object or an array`);
    const merged: JsonObject = Object.create(null);
    for (const [k, v] of Object.entries(header)) if (!["cases", "source", "file", "conform"].includes(k)) merged[k] = v;
    for (const [k, v] of Object.entries(raw)) merged[k] = v;
    for (const k of Object.keys(merged)) {
      if (!["name", "runner", "update", "program", "skip", "expect"].includes(k)) fail(`${where}: a runner case has the field ${k}`);
    }
    const c: Case = { name, source: "", expect: "expect" in raw ? runnerExpect(raw.expect, raw.update === true, where) : { kind: "runner" }, runner: r, dir };
    if (raw.update === true) c.update = true;
    if (raw.program === true) c.program = true;
    if ("skip" in raw) c.skip = raw.skip as string;
    if ("expect" in raw) c.rawExpect = raw.expect;
    return c;
  }

  // The header rule (3.10.2): a field of the case replaces the header's.
  const merged: JsonObject = Object.create(null);
  for (const [k, v] of Object.entries(header)) if (k !== "cases") merged[k] = v;
  // f10 10.1 item 1: returns and returnsSchema count as one field for the header rule.
  if ("returns" in raw || "returnsSchema" in raw) {
    delete merged.returns;
    delete merged.returnsSchema;
  }
  if ("source" in raw || "file" in raw || "conform" in raw || "render" in raw) {
    delete merged.source;
    delete merged.file;
    delete merged.conform;
  }
  for (const [k, v] of Object.entries(raw)) merged[k] = v;

  const hasSource = "source" in merged;
  const hasFile = "file" in merged;
  const hasConform = "conform" in merged;
  const hasRender = "render" in merged;
  if (Number(hasSource) + Number(hasFile) + Number(hasConform) + Number(hasRender) !== 1) fail(`${where} needs exactly one of source, file, conform and render`);
  let source = "";
  if (hasRender) {
    // f08 7.4.
    for (const k of Object.keys(merged)) {
      if (!["name", "opaque", "render", "skip", "expect"].includes(k)) fail(`${where}: a render case has the field ${k}`);
    }
    checkRender(merged.render, (merged.opaque as string[] | undefined) ?? [], where);
  } else if (hasConform) {
    // f04 9.2 item 2: the fields a conform case may have.
    for (const k of Object.keys(merged)) {
      if (!["name", "version", "opaque", "types", "conform", "skip", "expect"].includes(k)) fail(`${where}: a conform case has the field ${k}`);
    }
  } else if (hasFile) {
    try {
      source = opts.read !== undefined ? opts.read(merged.file as string) : readFileSync(join(dir, ...(merged.file as string).split("/")), "utf8");
    } catch {
      fail(`${where}: the file ${merged.file as string} cannot be read`);
    }
  } else if (hasSource) {
    source = merged.source as string;
  }

  const c: Case = { name, source, expect: "expect" in raw ? buildExpect(raw.expect, where, hasRender) : { kind: "result" } };
  if ("version" in merged) c.version = merged.version as string;
  if ("env" in merged) c.env = merged.env as string;
  if ("opaque" in merged) c.opaque = merged.opaque as string[];
  if ("methods" in merged) {
    c.methods = {};
    for (const [k, v] of Object.entries(merged.methods as JsonObject)) {
      const stub = toStub(v);
      if (stub.type === undefined && opts.declares?.(k) !== true) fail(`${where}: the method stub ${k} has no type and the base environment does not declare it`);
      c.methods[k] = stub;
    }
  }
  if ("functions" in merged) {
    c.functions = {};
    for (const [k, v] of Object.entries(merged.functions as JsonObject)) {
      const stub = toStub(v);
      // 3.10.10 item 11: the base environment declares no host function.
      if (stub.type === undefined && opts.declares?.(k) !== true) fail(`${where}: the stub ${k} has no type and the base environment does not declare it`);
      c.functions[k] = stub;
    }
  }
  if ("reject" in merged) c.reject = merged.reject as string[];
  if ("returns" in merged) c.returns = merged.returns as string;
  if ("returnsSchema" in merged) c.returnsSchema = merged.returnsSchema as Case["returnsSchema"];
  if (c.returns !== undefined && c.returnsSchema !== undefined) fail(`${where}: both returns and returnsSchema`);
  if (merged.load === true) {
    // f10 10.2 items 1 and 2.
    c.load = true;
    if (c.returnsSchema === undefined && (c.returns === undefined || c.returns === "condition")) fail(`${where}: load with no expected type`);
    if (merged.check === false) fail(`${where}: load with "check": false`);
  }
  // f10 10.2 item 5: "span": null only in a load case.
  if (c.load !== true && c.expect.kind === "failure" && c.expect.diagnostics.some((d) => d.span === null)) fail(`${where}: "span": null in a case that is not a load`);
  if ("vars" in merged) c.vars = merged.vars as JsonObject;
  if ("limits" in merged) c.limits = { ...(merged.limits as object) };
  if ("precondition" in merged) c.precondition = merged.precondition as boolean;
  if ("removed" in merged) c.removed = merged.removed as string[];
  if ("schemas" in merged) c.schemas = merged.schemas as Record<string, { schema: Json; options?: string[] }>;
  if ("template" in merged) c.template = merged.template as "string" | "value";
  if ("types" in merged) c.types = merged.types as Case["types"];
  if ("groups" in merged) c.groups = merged.groups as string[];
  if (hasConform) c.conform = merged.conform as { type: string; value: Json; max?: number };
  if (hasRender) c.render = merged.render as { diagnostics: JsonObject[]; source?: string };
  if (c.expect.kind === "render" && !hasRender) fail(`${where}: a render result in a case without render`);
  if (hasRender && "expect" in raw && c.expect.kind !== "render") fail(`${where}: a render case expects no renderings`);
  if (c.expect.kind === "conform" && c.expect.failures !== undefined && c.conform?.max === undefined) fail(`${where}: failures without max`);
  // f04 9.2 item 5: a conform result only in a conform case; a conform case expects one or a declare failure.
  if (c.expect.kind === "conform" && !hasConform) fail(`${where}: a conform result in a case without conform`);
  if (hasConform && "expect" in raw && c.expect.kind !== "conform" && !(c.expect.kind === "failure" && c.expect.phase === "declare")) {
    fail(`${where}: a conform case expects neither a conform result nor a declare failure`);
  }
  // f02 4.2 item 3, 4.5 item 3: an analysis case has none of these fields.
  if (c.expect.kind === "result" && c.expect.analysis !== undefined) {
    for (const k of ["env", "opaque", "methods", "functions", "reject", "returns", "returnsSchema", "load", "vars", "limits", "check", "precondition", "version"]) {
      if (k in merged) fail(`${where}: an analysis case has the field ${k}`);
    }
  }
  if ("check" in merged) {
    c.check = merged.check as boolean;
    // 03 3.10.10: a case compiled without a check states no type or sites, and expects static check codes only.
    const ex = c.expect;
    if (c.check === false) {
      if (ex.kind === "result" && (ex.type !== undefined || ex.sites !== undefined)) fail(`${where}: "check": false with type or sites`);
      const staticCodes = basename(dir) === "conversion" || basename(dir) === "unions" ? CONVERSION_STATIC : basename(dir) === "config" ? CONFIG_STATIC : STATIC_CODE_SET;
      if (ex.kind === "failure" && ex.phase === "check" && ex.diagnostics.some((d) => !staticCodes.has(d.code))) {
        fail(`${where}: "check": false and a check code that is not static`);
      }
    }
  }
  if ("skip" in raw) c.skip = raw.skip as string;
  if ("expect" in raw) c.rawExpect = raw.expect;
  c.dir = dir;

  // 3.10.3 items 7 and 9: opaque values.
  const declared = new Set(c.opaque ?? []);
  const opaqueOk = (v: Json, at: string) => checkOpaque(v, declared, `${where}: ${at}`);
  if (c.vars !== undefined) opaqueOk(c.vars, "vars");
  for (const st of [...Object.values(c.functions ?? {}), ...Object.values(c.methods ?? {})]) {
    for (const row of st.calls) {
      if (row.args !== undefined) opaqueOk(row.args, "a stub's args");
      if (row.result !== undefined) opaqueOk(row.result.value, "a stub's result");
    }
  }
  if (c.expect.kind === "result" && c.expect.value !== undefined) opaqueOk(c.expect.value.value, "the expected value");
  if (c.conform !== undefined) opaqueOk(c.conform.value, "the conform value");

  // 3.10.10 item 9: a key of vars that is not a field of env.
  if (c.vars !== undefined) {
    const keys = Object.keys(c.vars);
    if (keys.length > 0) {
      const schemaVars = new Set(Object.keys(c.schemas ?? {}));
      const t = c.env === undefined ? null : parseType(c.env);
      // A malformed env is the declare error bad-type, not an invalid file.
      if (t === null || (t.ok && t.node.t === "record")) {
        const fields = new Set(t === null ? [] : (t as { node: { fields: { key: string }[] } }).node.fields.map((f) => f.key));
        // f01 7.1 item 5: a key of vars may name a variable of schemas.
        for (const k of keys) if (!fields.has(k) && !schemaVars.has(k)) fail(`${where}: vars gives ${k}, which is not a field of env or a key of schemas`);
      }
    }
  }
  return c;
}

/** Every `$opaque` object is of the form of 3.10.3 item 7, with a declared type. */
function checkOpaque(v: Json, declared: ReadonlySet<string>, where: string): void {
  const stack: Json[] = [v];
  while (stack.length > 0) {
    const x = stack.pop()!;
    if (Array.isArray(x)) stack.push(...x);
    else if (isObject(x)) {
      if ("$opaque" in x) {
        const o = x.$opaque;
        const ok =
          Object.keys(x).length === 1 && isObject(o) && Object.keys(o).length === 2 &&
          typeof o.type === "string" && declared.has(o.type) && isInt(o.id, 0);
        if (!ok) fail(`${where}: a $opaque object is not of the form of 3.10.3 item 7, or names an undeclared type`);
        continue;
      }
      stack.push(...Object.values(x));
    }
  }
}

/** The forms of an expected analysis (f02 4.3, 4.5 item 4). */
function checkAnalysis(v: Json, where: string): void {
  const w = `${where}: analysis`;
  if (!isObject(v)) fail(`${w} is not an object`);
  const exact = (o: Json, keys: string[], what: string): JsonObject => {
    if (!isObject(o) || Object.keys(o).length !== keys.length || !keys.every((k) => k in o)) fail(`${w}: ${what} is not of its form`);
    return o;
  };
  const strings = (x: Json, what: string) => {
    if (!Array.isArray(x) || !x.every((s) => typeof s === "string")) fail(`${w}: ${what} is not a list of strings`);
  };
  for (const [k, x] of Object.entries(v)) {
    switch (k) {
      case "names":
        strings(x, "names");
        break;
      case "refs":
        if (!Array.isArray(x)) fail(`${w}: refs is not a list`);
        for (const r of x) {
          const o = exact(r, ["name", "path", "span"], "a ref");
          if (typeof o.name !== "string") fail(`${w}: a ref's name`);
          // f07 9.3: a step is a string or {"star": true}.
          if (!Array.isArray(o.path) || !o.path.every((p) => typeof p === "string" || (isObject(p) && Object.keys(p).length === 1 && p.star === true))) fail(`${w}: a ref's path`);
          checkSpan(o.span, w);
        }
        break;
      case "calls":
        if (!Array.isArray(x)) fail(`${w}: calls is not a list`);
        for (const c of x) {
          const o = exact(c, ["name", "namespace", "span", "args", "lambda"], "a call");
          if (typeof o.name !== "string" || (o.namespace !== null && typeof o.namespace !== "string") || typeof o.lambda !== "boolean") fail(`${w}: a call`);
          checkSpan(o.span, w);
          if (!Array.isArray(o.args)) fail(`${w}: a call's args`);
          for (const a of o.args) checkSpan(a, w);
        }
        break;
      case "literal":
        if (x !== false) exact(x, ["value"], "literal");
        break;
      case "segments":
        if (x === null) break;
        if (!Array.isArray(x)) fail(`${w}: segments is not a list`);
        for (const g of x) {
          if (isObject(g) && "text" in g) {
            const o = exact(g, ["text", "span"], "a segment");
            if (typeof o.text !== "string") fail(`${w}: a segment's text`);
            checkSpan(o.span, w);
          } else {
            const o = exact(g, ["part", "span"], "a segment");
            checkSpan(o.part, w);
            checkSpan(o.span, w);
          }
        }
        break;
      default:
        fail(`${w} has the field ${k}`);
    }
  }
}

/** The `expect` of a runner case (f09 3.3, 9.2 items 5 and 6). */
function runnerExpect(v: Json, update: boolean, where: string): Expect {
  if (!isObject(v)) fail(`${where}: expect is not an object`);
  if ("invalid" in v) {
    if (v.invalid !== true || Object.keys(v).length !== 1) fail(`${where}: invalid is not true alone`);
    return { kind: "runner", invalid: true };
  }
  if (!("verdicts" in v)) fail(`${where}: a runner case expects neither invalid nor verdicts`);
  for (const k of Object.keys(v)) if (!["verdicts", "report", "text"].includes(k)) fail(`${where}: a runner expect has the field ${k}`);
  if (!Array.isArray(v.verdicts)) fail(`${where}: verdicts is not a list`);
  const statuses = ["pass", "fail", "skip", "invalid", ...(update ? ["updated"] : [])];
  const verdicts = v.verdicts.map((e): RunnerVerdict => {
    if (!isObject(e) || typeof e.name !== "string" || typeof e.status !== "string" || !statuses.includes(e.status)) fail(`${where}: an entry of verdicts`);
    for (const k of Object.keys(e)) if (!["name", "status", "differs", "fixture", "message"].includes(k)) fail(`${where}: an entry of verdicts has the field ${k}`);
    const out: RunnerVerdict = { name: e.name, status: e.status };
    if ("differs" in e) {
      if (e.status !== "fail" || !Array.isArray(e.differs) || !e.differs.every((x) => typeof x === "string")) fail(`${where}: differs`);
      out.differs = e.differs as string[];
    }
    if ("fixture" in e) {
      if (e.status !== "invalid" || !Array.isArray(e.fixture) || e.fixture.length > 64 || !e.fixture.every((x) => typeof x === "string" || isInt(x, 0))) fail(`${where}: fixture`);
      out.fixture = e.fixture as (string | number)[];
    }
    if ("message" in e) {
      if (e.status !== "invalid" || typeof e.message !== "string") fail(`${where}: message`);
      out.message = e.message;
    }
    return out;
  });
  const out: Expect = { kind: "runner", verdicts };
  if ("report" in v) {
    if (typeof v.report !== "string") fail(`${where}: report is not a string`);
    out.report = v.report;
  }
  if ("text" in v) {
    if (typeof v.text !== "string" || !update) fail(`${where}: text`);
    out.text = v.text;
  }
  return out;
}

/** A trace of f08 7.2. */
function traceOf(v: Json, where: string): TraceEntry[] {
  if (!Array.isArray(v)) fail(`${where}: trace is not a list`);
  return v.map((e) => {
    if (!isObject(e) || Object.keys(e).length !== 3 || typeof e.name !== "string" || !(e.index === null || isInt(e.index, 0))) fail(`${where}: a trace entry is not of its form`);
    return { span: checkSpan(e.span, where), name: e.name, index: e.index as number | null };
  });
}

/** An object of renderings (f08 7.3). */
function renderings(v: Json, where: string): Renderings {
  if (!isObject(v) || Object.keys(v).length === 0) fail(`${where}: rendered is not an object of renderings`);
  const out: Renderings = {};
  for (const [k, x] of Object.entries(v)) {
    if ((k !== "line" && k !== "detail" && k !== "json") || typeof x !== "string") fail(`${where}: rendered has the field ${k}`);
    out[k] = x;
  }
  return out;
}

/** The field `render` (f08 7.4). */
function checkRender(v: Json, opaque: string[], where: string): void {
  if (!isObject(v) || !Object.keys(v).every((k) => k === "diagnostics" || k === "source")) fail(`${where}: render is not of its form`);
  if ("source" in v && typeof v.source !== "string") fail(`${where}: render.source is not a string`);
  const ds = v.diagnostics;
  if (!Array.isArray(ds) || ds.length === 0) fail(`${where}: render.diagnostics is not a non-empty list`);
  const lines = typeof v.source === "string" ? v.source.replace(/^\uFEFF/, "").split("\n") : null;
  for (const d of ds) {
    if (!isObject(d) || typeof d.phase !== "string" || typeof d.code !== "string" || typeof d.message !== "string") fail(`${where}: a rendered diagnostic`);
    for (const k of Object.keys(d)) if (!["phase", "code", "message", "class", "span", "path", "data", "trace"].includes(k)) fail(`${where}: a rendered diagnostic has the field ${k}`);
    if ("span" in d) {
      const sp = checkSpan(d.span, where);
      // f08 7.4 item 3: a span inside the source text.
      const ok = (l: number, c: number) => lines !== null && l <= lines.length && c <= [...lines[l - 1]].length + 1;
      if (lines !== null && (!ok(sp[0], sp[1]) || !ok(sp[2], sp[3]))) fail(`${where}: a span outside the source text`);
    }
    if ("data" in d) {
      if (d.code !== "failed") fail(`${where}: data on a code other than failed`);
      checkOpaque(d.data, new Set(opaque), where);
    }
    if ("path" in d && d.code !== "not-conforming") fail(`${where}: path on a code other than not-conforming`);
    if ("class" in d && typeof d.class !== "string") fail(`${where}: class is not a string`);
    if ("trace" in d) {
      if (d.phase !== "run") fail(`${where}: trace on a phase other than run`);
      traceOf(d.trace, where);
    }
  }
}

function checkSpan(v: Json, where: string): LineSpan {
  if (!Array.isArray(v) || v.length !== 4 || !v.every((x) => isInt(x, 1))) fail(`${where}: a span is not four whole numbers of 1 or more`);
  return v as LineSpan;
}

const DIAG_CLASSES = new Set(["constant-condition", "dynamic", "index", "divide", "value", "host", "environment", "range", "limit"]);
const SITE_CLASSES = new Set(["dynamic", "index", "divide", "value", "host"]);

function buildExpect(v: Json, where: string, render = false): Expect {
  if (!isObject(v)) fail(`${where}: expect is not an object`);
  if ("phase" in v) {
    for (const k of Object.keys(v)) {
      if (!["phase", "diagnostics", "steps", "allocated", "rendered"].includes(k)) {
        fail(`${where}: expect is a failure and has the field ${k}`);
      }
    }
    const phase = v.phase;
    if (phase !== "declare" && phase !== "parse" && phase !== "check" && phase !== "run") fail(`${where}: bad phase`);
    if (!Array.isArray(v.diagnostics) || v.diagnostics.length === 0) fail(`${where}: diagnostics is not a non-empty list`);
    const diagnostics = v.diagnostics.map((d) => buildDiag(d, where));
    // 3.10.10 item 16.
    if ((phase === "parse" || phase === "declare") && diagnostics.length > 1) fail(`${where}: an expect of phase ${phase} states more than one diagnostic`);
    const out: Expect = { kind: "failure", phase, diagnostics };
    for (const k of ["steps", "allocated"] as const) {
      if (k in v) {
        if (phase !== "run") fail(`${where}: ${k} in an expect whose phase is not run`);
        if (!isInt(v[k], 0)) fail(`${where}: ${k} is not a whole number of 0 or more`);
        out[k] = v[k] as number;
      }
    }
    if ("rendered" in v) {
      // f08 7.3, 7.6 item 3.
      if (phase === "declare") fail(`${where}: rendered in a declare failure`);
      if (diagnostics.some((d) => LIMIT_CODES.has(d.code))) fail(`${where}: rendered of a limit failure`);
      out.rendered = renderings(v.rendered, where);
    }
    for (const d of diagnostics) {
      if (d.trace !== undefined && phase !== "run") fail(`${where}: trace in a diagnostic of phase ${phase}`);
      if (d.message !== undefined && phase === "declare") fail(`${where}: message in a declare diagnostic`);
    }
    // 03 3.10.10: a case that expects a limit code states no counts.
    if ((out.steps !== undefined || out.allocated !== undefined) && diagnostics.some((d) => LIMIT_CODES.has(d.code))) {
      fail(`${where}: a case that expects a limit code states steps or allocated`);
    }
    return out;
  }
  if (render) return { kind: "render", ...renderings(v, where) };
  if ("conforms" in v) {
    // f04 9.2: a conform result.
    for (const k of Object.keys(v)) if (!["conforms", "path", "message", "steps", "failures"].includes(k)) fail(`${where}: a conform result has the field ${k}`);
    if (typeof v.conforms !== "boolean") fail(`${where}: conforms is not a boolean`);
    const out: Expect = { kind: "conform", conforms: v.conforms };
    if ("path" in v) {
      if (!Array.isArray(v.path) || !v.path.every((x) => typeof x === "string" || isInt(x, 0))) fail(`${where}: path is not a path`);
      out.path = v.path as (string | number)[];
    }
    if ("message" in v) {
      if (typeof v.message !== "string") fail(`${where}: message is not a string`);
      out.message = v.message;
    }
    if (v.conforms && (out.path !== undefined || out.message !== undefined)) fail(`${where}: path or message beside conforms true`);
    if ("steps" in v) {
      if (!isInt(v.steps, 0)) fail(`${where}: steps is not a whole number of 0 or more`);
      out.steps = v.steps as number;
    }
    if ("failures" in v) {
      // f08 7.5.
      const fs = v.failures;
      if (!Array.isArray(fs)) fail(`${where}: failures is not a list`);
      out.failures = fs.map((x) => {
        if (!isObject(x) || !Array.isArray(x.path) || !Object.keys(x).every((k) => k === "path" || k === "message")) fail(`${where}: a failure is not of its form`);
        if ("message" in x && typeof x.message !== "string") fail(`${where}: a failure's message`);
        return { path: x.path as (string | number)[], ...(typeof x.message === "string" ? { message: x.message } : {}) };
      });
    }
    return out;
  }
  const out: Expect = { kind: "result" };
  for (const [k, val] of Object.entries(v)) {
    switch (k) {
      case "type":
        if (typeof val !== "string") fail(`${where}: type is not a string`);
        out.type = val;
        break;
      case "sites":
        if (!Array.isArray(val)) fail(`${where}: sites is not a list`);
        out.sites = val.map((s) => {
          if (!isObject(s)) fail(`${where}: a site is not an object`);
          for (const sk of Object.keys(s)) if (sk !== "class" && sk !== "span") fail(`${where}: a site has the field ${sk}`);
          if (typeof s.class !== "string" || !SITE_CLASSES.has(s.class)) fail(`${where}: a site has a bad class`);
          return { class: s.class, span: checkSpan(s.span, where) };
        });
        break;
      case "value":
        out.value = { value: val };
        break;
      case "analysis":
        if (Object.keys(v).length !== 1) fail(`${where}: an expect with analysis has another field`);
        checkAnalysis(val, where);
        out.analysis = val as JsonObject;
        break;
      case "description":
        out.description = { value: val };
        break;
      case "json":
        if (typeof val !== "string") fail(`${where}: json is not a string`);
        out.json = val;
        break;
      case "truthy":
        if (typeof val !== "boolean") fail(`${where}: truthy is not a boolean`);
        out.truthy = val;
        break;
      case "steps":
      case "allocated":
        if (!isInt(val, 0)) fail(`${where}: ${k} is not a whole number of 0 or more`);
        out[k] = val;
        break;
      default:
        fail(`${where}: expect has the field ${k}`);
    }
  }
  return out;
}

function buildDiag(v: Json, where: string): ExpectDiagnostic {
  if (!isObject(v)) fail(`${where}: a diagnostic is not an object`);
  if (typeof v.code !== "string" || !/^[a-z]+(-[a-z0-9]+)*$/.test(v.code)) fail(`${where}: a diagnostic has no valid code`);
  const d: ExpectDiagnostic = { code: v.code };
  for (const [k, val] of Object.entries(v)) {
    switch (k) {
      case "code":
        break;
      case "class":
        if (typeof val !== "string" || !DIAG_CLASSES.has(val)) fail(`${where}: a diagnostic has a bad class`);
        d.class = val;
        break;
      case "span":
        d.span = val === null ? null : checkSpan(val, where);
        break;
      case "path":
        if (!Array.isArray(val) || val.length > 64 || !val.every((s) => typeof s === "string" || isInt(s, 0))) {
          fail(`${where}: a diagnostic has a bad path`);
        }
        d.path = val as (string | number)[];
        break;
      case "message":
        if (typeof val !== "string") fail(`${where}: a message is not a string`);
        d.message = val;
        break;
      case "data":
        d.data = { value: val };
        break;
      case "trace":
        d.trace = traceOf(val, where);
        break;
      default:
        fail(`${where}: a diagnostic has the field ${k}`);
    }
  }
  return d;
}
