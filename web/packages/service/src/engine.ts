// The engine: the service's calls over the `risor` package, synchronous.
// The worker runs one; localService wraps one in promises. It loads all of
// Risor, so the main entry imports it only through localService.

import { complete, createEnv, envFromDescription, formatProgram, hover, RisorError, Source, toJSON } from "risor";
import type { Description, Diagnostic, Env } from "risor";
import { TextIndex } from "./positions.ts";
import type { CheckOptions, Completion, FormatResult, Hover, Problem, RunOptions, RunResult } from "./types.ts";

import { DEFAULT_TIMEOUT_MS } from "./defaults.ts";

interface Held {
  description: Description | undefined;
  env: Env;
  /** The environment for runs, with a deadline, by deadline. */
  runners: Map<number, Env>;
  /** A problem that stops every run, when the description declares a function. */
  noRun: Problem | null;
  /** A problem of the description itself: it is not one, or it does not declare. */
  bad: Problem | null;
}

/** A diagnostic as a problem, with its span in UTF-16 offsets of the text. */
export function toProblem(d: Diagnostic, index: TextIndex | null): Problem {
  const p: Problem = { phase: d.phase, code: d.code, message: d.message };
  if (d.class) p.class = d.class;
  if (d.span && index) {
    p.from = index.unitOfRisor(d.span.start);
    p.to = index.unitOfRisor(d.span.end);
    const pos = new Source(index.text).position(d.span.start);
    p.line = pos.line;
    p.column = pos.column;
  }
  if (d.trace && d.trace.length > 0 && index) {
    const src = new Source(index.text);
    p.trace = d.trace.map((t) => {
      const pos = src.position(t.span.start);
      return { from: index.unitOfRisor(t.span.start), line: pos.line, column: pos.column, name: t.name };
    });
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
    let bad: Problem | null = null;
    if (description === undefined || description === null) env = createEnv();
    else {
      try {
        env = envFromDescription(description);
      } catch (e) {
        env = createEnv();
        const ds = diagnosticsOf(e);
        bad = e instanceof RisorError
          ? toProblem(ds[0], null)
          : serviceProblem("bad-description", `The environment is not a description (${(e as Error).message}). Pass the JSON that Env.Describe() or env.describe() makes.`);
        if (typeof console !== "undefined") console.error(`@risor/service: ${bad.message}`);
      }
    }
    const fns = description && (Object.keys(description.functions ?? {}).length > 0 || Object.values(description.opaque ?? {}).some((m) => Object.keys(m).length > 0));
    const noRun = fns
      ? serviceProblem("host-functions", "This environment declares host functions, which run only on the host. Give the preview a `run` callback that runs the program on your server.")
      : null;
    this.held.set(id, { description, env, runners: new Map(), noRun, bad });
  }

  close(id: number): void {
    this.held.delete(id);
  }

  private get(id: number): Held {
    const h = this.held.get(id);
    if (!h) throw new Error(`@risor/service: no environment ${id}`);
    return h;
  }

  check(id: number, text: string, options: CheckOptions = {}): Problem[] {
    const h = this.get(id);
    const index = new TextIndex(text);
    const out: Problem[] = h.bad ? [h.bad] : [];
    try {
      const c = h.env.check(text, options.expected);
      if (!c.ok) out.push(...c.diagnostics.map((d) => toProblem(d, index)));
    } catch (e) {
      out.push(...diagnosticsOf(e).map((d) => toProblem(d, index)));
    }
    return out;
  }

  complete(id: number, text: string, offset: number, options: CheckOptions = {}): Completion | null {
    const h = this.get(id);
    const index = new TextIndex(text);
    try {
      const r = complete(h.env, text, index.positionOf(offset), options.expected);
      const [l1, c1, l2, c2] = r.span;
      return { from: index.unitOfPosition(l1, c1), to: index.unitOfPosition(l2, c2), items: r.items };
    } catch {
      return null;
    }
  }

  hover(id: number, text: string, offset: number, options: CheckOptions = {}): Hover | null {
    const h = this.get(id);
    const index = new TextIndex(text);
    try {
      const r = hover(h.env, text, index.positionOf(offset), options.expected);
      if (!r) return null;
      const [l1, c1, l2, c2] = r.span;
      return { from: index.unitOfPosition(l1, c1), to: index.unitOfPosition(l2, c2), kind: r.kind, type: r.type, signature: r.signature, doc: r.doc };
    } catch {
      return null;
    }
  }

  run(id: number, text: string, vars: Record<string, unknown> = {}, options: RunOptions = {}): RunResult {
    const h = this.get(id);
    if (h.noRun) return { ok: false, stage: "declare", problems: [h.noRun] };
    if (h.bad) return { ok: false, stage: "declare", problems: [h.bad] };
    const timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    let env = h.runners.get(timeoutMs);
    if (!env) {
      try {
        env = h.env.child({ limits: { ...h.env.environment.limits, timeoutMs } });
      } catch (e) {
        return { ok: false, stage: "declare", problems: diagnosticsOf(e).map((d) => toProblem(d, null)) };
      }
      h.runners.set(timeoutMs, env);
    }
    const index = new TextIndex(text);
    let c;
    try {
      c = env.tryCompile(text, { check: options.check ?? true, expected: options.expected });
    } catch (e) {
      return { ok: false, stage: "declare", problems: diagnosticsOf(e).map((d) => toProblem(d, index)) };
    }
    if (!c.ok) return { ok: false, stage: "compile", problems: c.diagnostics.map((d) => toProblem(d, index)) };
    const t0 = performance.now();
    const o = c.program.execute(vars);
    const ms = performance.now() - t0;
    if (!o.ok) return { ok: false, stage: "run", problems: [toProblem(o.diagnostic, index)], usage: o.usage, ms };
    try {
      return { ok: true, json: toJSON(o.value), type: c.program.type, usage: o.usage, ms };
    } catch (e) {
      return { ok: false, stage: "run", problems: diagnosticsOf(e).map((d) => toProblem(d, index)), usage: o.usage, ms };
    }
  }

  format(text: string): FormatResult {
    const r = formatProgram(text);
    if (r.ok) return { ok: true, text: r.text };
    const index = new TextIndex(text);
    return { ok: false, problems: r.diagnostics.map((d) => toProblem(d, index)) };
  }
}
