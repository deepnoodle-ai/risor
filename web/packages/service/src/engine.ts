// The engine: the service's calls over the `@deepnoodle/risor` package, synchronous.
// The worker runs one; localService wraps one in promises. It loads all of
// Risor, so the main entry imports it only through localService.

import { createEnv, Env, format, jsonText, parse, RisorError } from "@deepnoodle/risor";
import type { Description, Diagnostic, Span } from "@deepnoodle/risor";
import { TextIndex } from "./positions.ts";
import type { CheckOptions, Completion, FormatResult, Hover, Problem, RunOptions, RunResult } from "./types.ts";

import { DEFAULT_TIMEOUT_MS } from "./defaults.ts";

interface Held {
  description: Description | undefined;
  env: Env;
  /** The environment for runs, with a deadline, by deadline. */
  runners: Map<number, Env>;
  /** A problem that stops every run, when the description declares a function or a method. */
  noRun: Problem | null;
}

/** A span of lines and columns, as UTF-16 offsets of the text. */
function unitsOf(s: Span, index: TextIndex): { from: number; to: number } {
  return { from: index.unitOfPosition(s.start.line, s.start.column), to: index.unitOfPosition(s.end.line, s.end.column) };
}

/** A diagnostic as a problem, with its span in UTF-16 offsets of the text. */
export function toProblem(d: Diagnostic, index: TextIndex | null): Problem {
  const p: Problem = { phase: d.phase, code: d.code, message: d.message };
  if (d.class) p.class = d.class;
  if (d.span && index) {
    Object.assign(p, unitsOf(d.span, index));
    p.line = d.span.start.line;
    p.column = d.span.start.column;
  }
  if (d.trace && d.trace.length > 0 && index) {
    p.trace = d.trace.map((t) => ({ from: unitsOf(t.span, index).from, line: t.span.start.line, column: t.span.start.column, name: t.name }));
  }
  return p;
}

function diagnosticsOf(e: unknown): Diagnostic[] {
  if (e instanceof RisorError) return [...e.diagnostics];
  return [{ phase: "declare", code: "internal-error", message: String((e as Error)?.message ?? e) }];
}

function serviceProblem(code: string, message: string): Problem {
  return { phase: "service", code, message };
}

export class Engine {
  private readonly held = new Map<number, Held>();

  open(id: number, description: Description | undefined): void {
    let env: Env;
    if (description === undefined || description === null) env = createEnv();
    else {
      try {
        env = Env.fromDescription(description);
      } catch (e) {
        // The developer's problem, not the user's: logged once, and the text
        // is checked against the empty environment (PRD UC-1).
        env = createEnv();
        const message = e instanceof RisorError
          ? diagnosticsOf(e)[0].message
          : `The environment is not a description (${(e as Error).message}). Pass the JSON that Env.Describe() or env.describe() makes.`;
        if (typeof console !== "undefined") console.error(`@deepnoodle/risor-service: ${message}`);
      }
    }
    const fns = description && (Object.keys(description.functions ?? {}).length > 0 || Object.values(description.opaque ?? {}).some((m) => Object.keys(m).length > 0));
    const noRun = fns
      ? serviceProblem("host-functions", "This environment declares host functions, which run only on the host. Give the preview a `run` callback that runs the program on your server.")
      : null;
    this.held.set(id, { description, env, runners: new Map(), noRun });
  }

  close(id: number): void {
    this.held.delete(id);
  }

  private get(id: number): Held {
    const h = this.held.get(id);
    if (!h) throw new Error(`@deepnoodle/risor-service: no environment ${id}`);
    return h;
  }

  check(id: number, text: string, options: CheckOptions = {}): Problem[] {
    const h = this.get(id);
    const index = new TextIndex(text);
    // A bad description is the developer's problem, reported once on the
    // console when the environment opened; the user's text is checked
    // against the empty environment.
    const out: Problem[] = [];
    try {
      h.env.check(parse(text), options.expected);
    } catch (e) {
      out.push(...diagnosticsOf(e).map((d) => toProblem(d, index)));
    }
    return out;
  }

  complete(id: number, text: string, offset: number, options: CheckOptions = {}): Completion | null {
    const h = this.get(id);
    const index = new TextIndex(text);
    try {
      const r = h.env.complete(text, index.positionOf(offset), options.expected);
      return { ...unitsOf(r.span, index), items: r.items };
    } catch {
      return null;
    }
  }

  hover(id: number, text: string, offset: number, options: CheckOptions = {}): Hover | null {
    const h = this.get(id);
    const index = new TextIndex(text);
    try {
      const r = h.env.hover(text, index.positionOf(offset), options.expected);
      if (!r) return null;
      return { ...unitsOf(r.span, index), kind: r.kind, type: r.type, signature: r.signature, doc: r.doc };
    } catch {
      return null;
    }
  }

  run(id: number, text: string, vars: Record<string, unknown> = {}, options: RunOptions = {}): RunResult {
    const h = this.get(id);
    // A bad description was logged when the environment opened, and h.env is
    // then the empty environment. The run uses it, as check does (PRD UC-1):
    // the developer's mistake is on the console, not in the user's result.
    if (h.noRun) return { ok: false, stage: "declare", problems: [h.noRun] };
    const timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    let env = h.runners.get(timeoutMs);
    if (!env) {
      try {
        env = h.env.with({ limits: { ...h.env.describe().limits, deadlineMs: timeoutMs } });
      } catch (e) {
        return { ok: false, stage: "declare", problems: diagnosticsOf(e).map((d) => toProblem(d, null)) };
      }
      h.runners.set(timeoutMs, env);
    }
    const index = new TextIndex(text);
    let program;
    try {
      program = env.compile(parse(text), { check: options.check ?? true, expected: options.expected });
    } catch (e) {
      const stage = e instanceof RisorError && e.phase === "declare" ? "declare" : "compile";
      return { ok: false, stage, problems: diagnosticsOf(e).map((d) => toProblem(d, index)) };
    }
    const t0 = performance.now();
    let o;
    try {
      o = program.measure(vars);
    } catch (e) {
      const ms = performance.now() - t0;
      const usage = e instanceof RisorError ? e.usage : undefined;
      return { ok: false, stage: "run", problems: diagnosticsOf(e).map((d) => toProblem(d, index)), ...(usage ? { usage } : {}), ms };
    }
    const ms = performance.now() - t0;
    try {
      return { ok: true, json: jsonText(o.value), type: program.checkResult ? program.checkResult.type.toString() : null, usage: o.usage, ms };
    } catch (e) {
      return { ok: false, stage: "run", problems: diagnosticsOf(e).map((d) => toProblem(d, index)), usage: o.usage, ms };
    }
  }

  format(text: string): FormatResult {
    try {
      return { ok: true, text: format(text) };
    } catch (e) {
      const index = new TextIndex(text);
      return { ok: false, problems: diagnosticsOf(e).map((d) => toProblem(d, index)) };
    }
  }
}
