// The public shape of a diagnostic and of a failure (03 3.7; 05 Part E, E.8).
//
// Inside the implementation a span is two code-point offsets into the source
// (src/syntax/source.ts). A diagnostic that leaves the implementation has its
// spans as the specification writes them: four numbers, start line, start
// column, end line, end column (03 3.7.7).

import type { Diagnostic as InnerDiagnostic, Phase } from "../syntax/diagnostic.ts";
import type { LineSpan, Source } from "../syntax/source.ts";

export type { Phase };

/** A span (03 3.7.7): start line, start column, end line, end column. Lines and columns count from 1; the end is exclusive. */
export type Span = LineSpan;

/** A position (03 3.7.7): a line and a column, both from 1. A column counts code points. */
export interface Pos {
  line: number;
  column: number;
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
  if (d.span !== undefined && source != null) out.span = source.lineSpan(d.span);
  if (d.class !== undefined) out.class = d.class;
  if (d.path !== undefined) out.path = d.path;
  if (d.data !== undefined || (d.phase === "run" && d.code === "failed")) out.data = d.data ?? null;
  if (d.trace !== undefined && source != null) out.trace = d.trace.map((t) => ({ span: source.lineSpan(t.span), name: t.name, index: t.index }));
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
