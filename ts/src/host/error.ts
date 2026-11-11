// The public shape of a diagnostic and of a failure (03 3.7; 05 Part E, E.8).
//
// Inside the implementation a span is two code-point offsets into the source
// (src/syntax/source.ts). A span that leaves the implementation is a record of
// two positions, each a line and a column (03 3.7.7; 05 E.8). The four numbers
// of 03 3.7.7 item 5 are only how a span is written, in JSON and in rendered
// text.

import type { Diagnostic as InnerDiagnostic, Phase } from "../syntax/diagnostic.ts";
import type { LineSpan, Source, Span as InnerSpan } from "../syntax/source.ts";

export type { Phase };

/** A position (03 3.7.7): a line and a column, both from 1. A column counts code points. */
export interface Pos {
  line: number;
  column: number;
}

/** A span (03 3.7.7; 05 E.8): a start position and an end position. The end is exclusive. */
export interface Span {
  start: Pos;
  end: Pos;
}

/** Internal: the span of code-point offsets, as lines and columns. */
export function spanOf(source: Source, s: InnerSpan): Span {
  return { start: source.position(s.start), end: source.position(s.end) };
}

/** Internal: a span from its four numbers. */
export function spanFromLine(s: LineSpan): Span {
  return { start: { line: s[0], column: s[1] }, end: { line: s[2], column: s[3] } };
}

/** Internal: the four numbers a span is written as (03 3.7.7 item 5). */
export function lineSpanOf(s: Span): LineSpan {
  return [s.start.line, s.start.column, s.end.line, s.end.column];
}

/** A call in progress when a run failed (package F8, f08 4). */
export interface TraceEntry {
  span: Span;
  name: string;
  index: number | null;
}

/** The one shape of a diagnostic (03 3.7.1). */
export interface Diagnostic {
  phase: Phase;
  code: string;
  message: string;
  /** Absent for `declare`, and where 03 3.2.4 says so. */
  span?: Span;
  class?: string;
  path?: (string | number)[];
  /** For `failed`: the second argument of `fail`. */
  data?: unknown;
  /** Package F8: the calls in progress when a run failed, from the outermost in (f08 4). */
  trace?: TraceEntry[];
  /** For `function-error`: the host's own error. */
  cause?: unknown;
}

/** What a run used (03 3.5). */
export interface Usage {
  steps: number;
  allocated: number;
}

/** A diagnostic with its spans as lines and columns. A diagnostic with no span needs no source. */
export function publicDiagnostic(d: InnerDiagnostic, source: Source | null | undefined): Diagnostic {
  const out: Diagnostic = { phase: d.phase, code: d.code, message: d.message };
  if (d.span !== undefined && source != null) out.span = spanOf(source, d.span);
  if (d.class !== undefined) out.class = d.class;
  if (d.path !== undefined) out.path = d.path;
  if (d.data !== undefined || (d.phase === "run" && d.code === "failed")) out.data = d.data ?? null;
  if (d.trace !== undefined && source != null) out.trace = d.trace.map((t) => ({ span: spanOf(source, t.span), name: t.name, index: t.index }));
  else if (d.trace !== undefined) out.trace = [];
  const cause = (d as { cause?: unknown }).cause;
  if (cause !== undefined) out.cause = cause;
  return out;
}

/** A failure with its diagnostics (05 Part E, E.8). `Error` is a global of JavaScript, so this is `RisorError`. */
export class RisorError extends Error {
  readonly phase: Phase;
  readonly diagnostics: readonly Diagnostic[];
  readonly usage?: Usage;
  constructor(diagnostics: Diagnostic[], usage?: Usage) {
    super(diagnostics.map((d) => `${d.code}: ${d.message}`).join("; "));
    this.phase = diagnostics[0].phase;
    this.diagnostics = diagnostics;
    if (usage !== undefined) this.usage = usage;
  }
}

/** A RisorError from diagnostics of the implementation, their spans read in the source. */
export function failure(ds: InnerDiagnostic[], source: Source | null | undefined, usage?: Usage): RisorError {
  return new RisorError(ds.map((d) => publicDiagnostic(d, source)), usage);
}

/** A declare failure with one diagnostic. */
export function declareError(code: string, message: string): RisorError {
  return new RisorError([{ phase: "declare", code, message }]);
}

/** Internal: a diagnostic as the renderings read it, its spans written as four numbers. */
export function renderForm(d: Diagnostic): Omit<Diagnostic, "span" | "trace"> & { span?: LineSpan; trace?: { span: LineSpan; name: string; index: number | null }[] } {
  const { span, trace, ...rest } = d;
  return {
    ...rest,
    ...(span !== undefined ? { span: lineSpanOf(span) } : {}),
    ...(trace !== undefined ? { trace: trace.map((t) => ({ span: lineSpanOf(t.span), name: t.name, index: t.index })) } : {}),
  };
}
