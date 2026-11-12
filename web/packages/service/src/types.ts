// The data the service gives. Everything here is JSON, so it can cross to a
// web worker and back, and a host's server can make the same shapes.
//
// Offsets (`from`, `to`) count UTF-16 code units, as the DOM and CodeMirror
// do. Lines and columns start at 1 and count code points, as Risor prints
// them in messages.

import type { Description } from "@deepnoodle/risor";

export type { Description };

/** One problem with a text: a parse, check or run error, or a problem of the service. */
export interface Problem {
  phase: "declare" | "parse" | "check" | "run" | "service";
  code: string;
  message: string;
  /** The span, in UTF-16 offsets. Absent when the problem has no place. */
  from?: number;
  to?: number;
  line?: number;
  column?: number;
  /** The class of a run failure (03 3.7.6). */
  class?: string;
  /** The calls in progress when a run failed, outermost first. */
  trace?: { from: number; line: number; column: number; name: string }[];
}

export interface CompletionItem {
  label: string;
  /** The kind of the candidate: "variable", "function", "method", "field", "keyword", ... */
  kind: string;
  /** Its type or signature, when it has one. */
  detail: string | null;
  /** The text that replaces the range. */
  insert: string;
}

export interface Completion {
  from: number;
  to: number;
  items: CompletionItem[];
}

export interface Hover {
  from: number;
  to: number;
  kind: string;
  type: string | null;
  signature: string | null;
  doc: string | null;
}

export interface Usage {
  steps: number;
  allocated: number;
}

/**
 * The result of a run. `json` is Risor's JSON text of the value, so each
 * number keeps the text Risor gave it. A run the service ended because it
 * did not answer in time has the stage `stopped`.
 */
export type RunResult =
  | { ok: true; json: string; type: string | null; usage: Usage; ms: number }
  | { ok: false; stage: "declare" | "compile" | "run" | "stopped"; problems: Problem[]; usage?: Usage; ms?: number };

export type FormatResult = { ok: true; text: string } | { ok: false; problems: Problem[] };

export interface CheckOptions {
  /** The type the program must have, in type text: "bool", "{id: int}[]". */
  expected?: string;
}

export interface RunOptions extends CheckOptions {
  /** Check the program before it runs. Default true. */
  check?: boolean;
  /** The deadline of the run, in milliseconds. Default 2,000. A description has no deadline of its own. */
  timeoutMs?: number;
}

/** An environment the service keeps, so each keystroke does not rebuild it. */
export interface RisorEnvironment {
  /**
   * The problems of a text. A call made while an earlier `check` of this
   * environment is still waiting replaces it: the earlier call resolves
   * `null`. A fast typist does not queue work.
   */
  check(text: string, options?: CheckOptions): Promise<Problem[] | null>;
  /** The problems of a text, never replaced by a later call. For a form's submit. */
  validate(text: string, options?: CheckOptions): Promise<Problem[]>;
  /** Completion at a UTF-16 offset. Replaced like `check`. */
  complete(text: string, offset: number, options?: CheckOptions): Promise<Completion | null>;
  /** Hover at a UTF-16 offset. Replaced like `check`. */
  hover(text: string, offset: number, options?: CheckOptions): Promise<Hover | null>;
  run(text: string, vars?: Record<string, unknown>, options?: RunOptions): Promise<RunResult>;
  /** Forget the environment in the worker. A later call opens it again. */
  release(): void;
}

export interface RisorService {
  /** An environment from a description (f01 5.3). No description is the empty environment. */
  environment(description?: Description): RisorEnvironment;
  format(text: string): Promise<FormatResult>;
  /** End the worker, if there is one. Pending calls resolve as stopped. */
  dispose(): void;
}
