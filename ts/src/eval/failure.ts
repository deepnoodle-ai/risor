// Run failures and their messages (01 14.5, 03 3.7, 3.8).
//
// A failure is raised as a JavaScript exception and unwinds to the innermost
// `try` in progress (02 12.1). Its message is built only when it is needed:
// for a `catch (err)` (and charged there, 02 12.4) or for the host's
// diagnostic (outside the usage).

import { CATALOG } from "../generated/messages.ts";
import type { CatalogEntry } from "../generated/messages.ts";
import type { Span } from "../syntax/source.ts";
import { numberToText } from "../value/number.ts";
import { Classifier, cpLength, jsonString } from "../value/value.ts";

const BY_CODE = new Map<string, CatalogEntry>(CATALOG.map((e) => [e.code, e]));

export const LIMIT_CODES: ReadonlySet<string> = new Set([
  "steps-exceeded", "allocation-exceeded", "deadline-exceeded", "depth-exceeded", "cancelled",
]);

/** Whether `catch` catches a code (02 12.2): the catalog's field `caught`. */
export function caught(code: string): boolean {
  return BY_CODE.get(code)?.caught === true;
}

export class RunFailure extends Error {
  readonly code: string;
  /** The span of the node that raised it. Absent for a limit raised outside the source. */
  span: Span | undefined;
  /** The catalog template, or null for `failed` and `function-error`. */
  readonly template: string | null;
  readonly params: Record<string, unknown>;
  /** For `failed`: the second argument of `fail`, or null. */
  readonly data: unknown;
  /** For `failed` and `function-error`: the message itself. */
  readonly text: string | null;
  /** The host's own error, for `function-error`. */
  override readonly cause?: unknown;

  constructor(code: string, span: Span | undefined, template: string | null, params: Record<string, unknown> = {}, extra: { data?: unknown; text?: string; cause?: unknown } = {}) {
    super(code);
    this.code = code;
    this.span = span;
    this.template = template;
    this.params = params;
    this.data = extra.data ?? null;
    this.text = extra.text ?? null;
    if (extra.cause !== undefined) this.cause = extra.cause;
  }
}

function cut(s: string): string {
  if (s.length <= 64 || cpLength(s) <= 64) return s;
  const cps = Array.from(s);
  return cps.slice(0, 61).join("") + "...";
}

function renderParam(kind: string, v: unknown, cls: Classifier): string {
  switch (kind) {
    case "word":
    case "kind":
    case "type":
      return String(v);
    case "number":
      return numberToText(v as number);
    case "string":
      return jsonString(String(v));
    case "value":
      switch (cls.kind(v)) {
        case "null":
          return "null";
        case "bool":
          return String(v);
        case "number":
          return numberToText(Number(v));
        case "string":
          return jsonString(v as string);
        default:
          return cls.kind(v);
      }
  }
  return String(v);
}

/** The message of a failure, built from the catalog (03 3.8). */
export function messageOf(f: RunFailure, cls: Classifier): string {
  if (f.text !== null) return f.text;
  const entry = BY_CODE.get(f.code);
  if (entry === undefined || f.template === null) return f.code;
  const text = entry.templates[f.template];
  if (text === undefined) throw new Error(`no template ${f.template} for ${f.code}`);
  return text.replace(/\{([a-z]+)\}/g, (_, name: string) => cut(renderParam(entry.params[name], f.params[name], cls)));
}

/** The class of a run diagnostic in a program compiled without a check (03 3.7.6). */
export function uncheckedClass(f: RunFailure): string | undefined {
  switch (f.code) {
    case "null-operand":
    case "wrong-kind":
    case "missing-field":
    case "no-match":
      return "dynamic";
    case "number-range":
      return f.template === "host" ? "environment" : "range";
    case "missing-variable":
      return "environment";
    case "not-conforming":
    case "invalid-argument":
    case "failed":
      return "value";
    case "index-range":
      return "index";
    case "divide-by-zero":
      return "divide";
    case "function-error":
    case "function-panic":
      return "host";
    case "internal-error":
      return undefined;
  }
  return LIMIT_CODES.has(f.code) ? "limit" : undefined;
}

export function templatesOf(code: string): string[] {
  return Object.keys(BY_CODE.get(code)?.templates ?? {});
}
