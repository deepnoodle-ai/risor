// Case files (03 3.10): reading, the header rule, and validation.

import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
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
  span?: LineSpan;
  path?: (string | number)[];
  message?: string;
  data?: { value: Json };
}

export type Expect =
  | {
      kind: "result";
      type?: string;
      sites?: { class: string; span: LineSpan }[];
      value?: { value: Json };
      truthy?: boolean;
      steps?: number;
      allocated?: number;
    }
  | {
      kind: "failure";
      phase: "declare" | "parse" | "check" | "run";
      diagnostics: ExpectDiagnostic[];
      steps?: number;
      allocated?: number;
    };

export interface Case {
  name: string;
  source: string;
  version?: string;
  env?: string;
  opaque?: string[];
  methods?: Record<string, string>;
  functions?: Record<string, Stub>;
  reject?: string[];
  returns?: string;
  vars?: JsonObject;
  limits?: { steps?: number; allocation?: number; depth?: number };
  precondition?: boolean;
  skip?: string;
  expect: Expect;
}

export type CaseFile =
  | { ok: true; cases: Case[] }
  | { ok: false; error: string; names: string[] };

export class InvalidFile extends Error {}

const LIMIT_CODES = new Set(["steps-exceeded", "allocation-exceeded", "deadline-exceeded", "depth-exceeded", "cancelled"]);

/** The fields a header and a case share (03 3.10.3). `precondition` is ruling A4. */
const SHARED = ["source", "file", "version", "env", "opaque", "methods", "functions", "reject", "returns", "vars", "limits", "precondition"];
const HEADER_FIELDS = new Set(["cases", ...SHARED]);
const CASE_FIELDS = new Set(["name", ...SHARED, "skip", "expect"]);

/** Read and validate a case file on disk. */
export function loadCaseFile(path: string): CaseFile {
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
  return validateCaseFile(json, dirname(path));
}

/** Validate a parsed case file: an object with `cases`, or an array of cases. */
export function validateCaseFile(json: Json, dir: string): CaseFile {
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
      const c = buildCase(header, raw, dir);
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
      if (typeof v !== "string") fail(`${where}: methods.${k} is not a string`);
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

function buildCase(header: JsonObject, raw: JsonObject, dir: string): Case {
  for (const k of Object.keys(raw)) {
    if (!CASE_FIELDS.has(k)) fail(`a case has a field ${k} that 3.10.3 does not list`);
  }
  if (typeof raw.name !== "string") fail("a case has no name, or its name is not a string");
  const name = raw.name;
  const where = `case ${JSON.stringify(name)}`;
  checkShared(raw, where);
  if ("skip" in raw && typeof raw.skip !== "string") fail(`${where}: skip is not a string`);
  if (!("expect" in raw)) fail(`${where} has no expect`);

  // The header rule (3.10.2): a field of the case replaces the header's.
  const merged: JsonObject = Object.create(null);
  for (const [k, v] of Object.entries(header)) if (k !== "cases") merged[k] = v;
  if ("source" in raw || "file" in raw) {
    delete merged.source;
    delete merged.file;
  }
  for (const [k, v] of Object.entries(raw)) merged[k] = v;

  const hasSource = "source" in merged;
  const hasFile = "file" in merged;
  if (hasSource === hasFile) fail(`${where} needs exactly one of source and file`);
  let source: string;
  if (hasFile) {
    try {
      source = readFileSync(join(dir, ...(merged.file as string).split("/")), "utf8");
    } catch {
      fail(`${where}: the file ${merged.file as string} cannot be read`);
    }
  } else {
    source = merged.source as string;
  }

  const c: Case = { name, source, expect: buildExpect(raw.expect, where) };
  if ("version" in merged) c.version = merged.version as string;
  if ("env" in merged) c.env = merged.env as string;
  if ("opaque" in merged) c.opaque = merged.opaque as string[];
  if ("methods" in merged) c.methods = { ...(merged.methods as Record<string, string>) };
  if ("functions" in merged) {
    c.functions = {};
    for (const [k, v] of Object.entries(merged.functions as JsonObject)) {
      const stub = toStub(v);
      // 3.10.10 item 11: the base environment declares no host function.
      if (stub.type === undefined) fail(`${where}: the stub ${k} has no type and the base environment does not declare it`);
      c.functions[k] = stub;
    }
  }
  if ("reject" in merged) c.reject = merged.reject as string[];
  if ("returns" in merged) c.returns = merged.returns as string;
  if ("vars" in merged) c.vars = merged.vars as JsonObject;
  if ("limits" in merged) c.limits = { ...(merged.limits as object) };
  if ("precondition" in merged) c.precondition = merged.precondition as boolean;
  if ("skip" in raw) c.skip = raw.skip as string;

  // 3.10.10 item 9: a key of vars that is not a field of env.
  if (c.vars !== undefined) {
    const keys = Object.keys(c.vars);
    if (keys.length > 0) {
      if (c.env === undefined) fail(`${where}: vars gives ${keys[0]}, and env declares no variable`);
      const t = parseType(c.env);
      // A malformed env is the declare error bad-type, not an invalid file.
      if (t.ok && t.node.t === "record") {
        const fields = new Set(t.node.fields.map((f) => f.key));
        for (const k of keys) if (!fields.has(k)) fail(`${where}: vars gives ${k}, which is not a field of env`);
      }
    }
  }
  return c;
}

function checkSpan(v: Json, where: string): LineSpan {
  if (!Array.isArray(v) || v.length !== 4 || !v.every((x) => isInt(x, 1))) fail(`${where}: a span is not four whole numbers of 1 or more`);
  return v as LineSpan;
}

const DIAG_CLASSES = new Set(["constant-condition", "dynamic", "index", "divide", "value", "host", "environment", "range", "limit"]);
const SITE_CLASSES = new Set(["dynamic", "index", "divide", "value", "host"]);

function buildExpect(v: Json, where: string): Expect {
  if (!isObject(v)) fail(`${where}: expect is not an object`);
  if ("phase" in v) {
    for (const k of Object.keys(v)) {
      if (!["phase", "diagnostics", "steps", "allocated"].includes(k)) {
        fail(`${where}: expect is a failure and has the field ${k}`);
      }
    }
    const phase = v.phase;
    if (phase !== "declare" && phase !== "parse" && phase !== "check" && phase !== "run") fail(`${where}: bad phase`);
    if (!Array.isArray(v.diagnostics) || v.diagnostics.length === 0) fail(`${where}: diagnostics is not a non-empty list`);
    const diagnostics = v.diagnostics.map((d) => buildDiag(d, where));
    const out: Expect = { kind: "failure", phase, diagnostics };
    for (const k of ["steps", "allocated"] as const) {
      if (k in v) {
        if (phase !== "run") fail(`${where}: ${k} in an expect whose phase is not run`);
        if (!isInt(v[k], 0)) fail(`${where}: ${k} is not a whole number of 0 or more`);
        out[k] = v[k] as number;
      }
    }
    // Ruling A5: a case that expects a limit code states no counts.
    if ((out.steps !== undefined || out.allocated !== undefined) && diagnostics.some((d) => LIMIT_CODES.has(d.code))) {
      fail(`${where}: a case that expects a limit code states steps or allocated`);
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
        d.span = checkSpan(val, where);
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
      default:
        fail(`${where}: a diagnostic has the field ${k}`);
    }
  }
  return d;
}
