// The one shape of a diagnostic (03 3.7.1).

import type { Span } from "./source.ts";

export type Phase = "declare" | "parse" | "check" | "run";

export interface Diagnostic {
  phase: Phase;
  code: string;
  message: string;
  /** Code-point offsets into the source. Absent for `declare`. */
  span?: Span;
  class?: string;
  path?: (string | number)[];
  /** For `failed`: the second argument of `fail`. Held as a language value. */
  data?: unknown;
}

/** Thrown by the lexer and the parser at the first `parse` diagnostic. */
export class ParseError extends Error {
  readonly diagnostic: Diagnostic;
  constructor(code: string, span: Span, message: string) {
    super(message);
    this.diagnostic = { phase: "parse", code, span, message };
  }
}
