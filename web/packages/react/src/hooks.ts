import { useCallback, useEffect, useRef, useState } from "react";
import type { CheckOptions, Description, Problem, RunResult } from "@risor/service";
import { useRisorEnvironment, useRisorService } from "./context.tsx";

export interface UseRisorCheckOptions extends CheckOptions {
  code: string;
  environment?: Description;
  /** The pause after a change before the check, in milliseconds. Default 300. */
  debounce?: number;
}

/**
 * The problems of a text, checked after each pause in typing. `validate`
 * checks the text as it is now, for a form's submit.
 */
export function useRisorCheck({ code, environment, expected, debounce = 300 }: UseRisorCheckOptions) {
  const env = useRisorEnvironment(environment);
  const service = useRisorService();
  const [problems, setProblems] = useState<Problem[]>([]);
  const [checking, setChecking] = useState(false);
  useEffect(() => {
    if (!env) return;
    let live = true;
    setChecking(true);
    const t = setTimeout(async () => {
      const ps = await env.check(code, { expected });
      if (!live || ps === null) return;
      setProblems(ps);
      setChecking(false);
    }, debounce);
    return () => {
      live = false;
      clearTimeout(t);
    };
  }, [env, code, expected, debounce]);
  const validate = useCallback(async () => {
    // Before the first effect there is no environment yet: open one for this call.
    const e = env ?? service.environment(environment);
    try {
      const ps = await e.validate(code, { expected });
      setProblems(ps);
      return ps;
    } finally {
      if (!env) e.release();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [env, service, code, expected]);
  return { problems, checking, validate };
}

/** Run a program on the host's server instead of in the worker: it returns the service's RunResult shape. */
export type RunCallback = (code: string, vars: Record<string, unknown>) => Promise<RunResult>;

export interface UseRisorRunOptions extends CheckOptions {
  code: string;
  environment?: Description;
  vars?: Record<string, unknown>;
  /** The pause after a change before the run, in milliseconds. Default 400. */
  debounce?: number;
  /** The deadline of a run in the worker. Default 2,000 ms. */
  timeoutMs?: number;
  /** Run on the host's server instead, for an environment with host functions. */
  run?: RunCallback;
  /** Run after each pause in typing. Default true. When false, call `rerun`. */
  auto?: boolean;
}

function isRunResult(r: unknown): r is RunResult {
  if (typeof r !== "object" || r === null) return false;
  const o = r as Record<string, unknown>;
  return o.ok === true ? typeof o.json === "string" : o.ok === false && Array.isArray(o.problems);
}

/** The result of running a text, after each pause in typing. Only the newest run's result is kept. */
export function useRisorRun({ code, environment, vars, expected, debounce = 400, timeoutMs, run, auto = true }: UseRisorRunOptions) {
  const env = useRisorEnvironment(environment);
  const [result, setResult] = useState<RunResult | null>(null);
  const [running, setRunning] = useState(false);
  const generation = useRef(0);
  const runRef = useRef(run);
  runRef.current = run;
  const varsKey = JSON.stringify(vars ?? {});

  const rerun = useCallback(async () => {
    const g = ++generation.current;
    if (!runRef.current && !env) return null;
    setRunning(true);
    let r: RunResult;
    try {
      r = runRef.current
        ? await runRef.current(code, JSON.parse(varsKey))
        : await env!.run(code, JSON.parse(varsKey), { expected, timeoutMs });
      if (!isRunResult(r)) throw new Error("the run callback did not return a RunResult");
    } catch (e) {
      r = { ok: false, stage: "run", problems: [{ phase: "service", code: "run-callback", message: String((e as Error)?.message ?? e) }] };
    }
    if (g !== generation.current) return r;
    setResult(r);
    setRunning(false);
    return r;
  }, [env, code, varsKey, expected, timeoutMs]);

  useEffect(() => {
    if (!auto) return;
    const t = setTimeout(() => void rerun(), debounce);
    return () => clearTimeout(t);
  }, [auto, rerun, debounce]);

  return { result, running, rerun };
}
