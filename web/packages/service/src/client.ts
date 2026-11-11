// The worker client: a RisorService whose calls run in a web worker.
//
// - Check, complete and hover are gated per environment and kind: one is in
//   the worker at a time, and a call made while another waits replaces it
//   (the replaced call resolves null).
// - Runs go one at a time. Each has a timer: its deadline plus a grace. When
//   the timer fires, the worker is stuck past the deadline that should have
//   ended the run, so the client ends the worker, resolves the run as
//   stopped, and starts a new worker that gets every live environment and
//   every call still waiting.

import type { Description } from "risor";
import { DEFAULT_TIMEOUT_MS } from "./defaults.ts";
import type { Request, Response } from "./protocol.ts";
import type { CheckOptions, Completion, FormatResult, Hover, Problem, RisorEnvironment, RisorService, RunOptions, RunResult } from "./types.ts";

/** The part of a Worker the client uses. */
export interface WorkerLike {
  postMessage(message: Request): void;
  terminate(): void;
  onmessage: ((e: MessageEvent<Response>) => void) | null;
}

export interface WorkerServiceOptions {
  /** Make the worker. By default the package's own worker script, as a module worker. */
  worker?: () => WorkerLike;
  /** Time past a run's deadline before the worker is ended. Default 1,500 ms. */
  graceMs?: number;
}

type Body = Request extends infer R ? (R extends { id: number } ? Omit<R, "id"> : never) : never;

interface Pending {
  body: Body;
  resolve: (result: unknown) => void;
  fail: (message: string) => void;
}

interface Gate {
  busy: boolean;
  /** The call waiting for the gate, and how to resolve it as replaced. */
  waiting: { pending: Pending; replace: () => void } | null;
}

function defaultWorker(): WorkerLike {
  return new Worker(new URL("./worker.ts", import.meta.url), { type: "module" }) as unknown as WorkerLike;
}

const stopped = (timeoutMs: number): RunResult => ({
  ok: false,
  stage: "stopped",
  problems: [{ phase: "service", code: "stopped", message: `The run did not end within ${timeoutMs} ms and was stopped.` }],
});

export function workerService(options: WorkerServiceOptions = {}): RisorService {
  return new WorkerService(options);
}

class WorkerService implements RisorService {
  private readonly make: () => WorkerLike;
  private readonly graceMs: number;
  private worker: WorkerLike | null = null;
  private nextId = 1;
  private nextEnv = 1;
  private readonly inFlight = new Map<number, Pending>();
  private readonly envs = new Map<number, Description | undefined>();
  private readonly gates = new Map<string, Gate>();
  private readonly runs: { pending: Pending; timeoutMs: number }[] = [];
  private running: { id: number; timer: ReturnType<typeof setTimeout> } | null = null;
  private disposed = false;

  constructor(options: WorkerServiceOptions) {
    this.make = options.worker ?? defaultWorker;
    this.graceMs = options.graceMs ?? 1_500;
  }

  private ensure(): WorkerLike {
    if (this.worker) return this.worker;
    const w = this.make();
    w.onmessage = (e) => this.receive(e.data);
    this.worker = w;
    for (const [env, description] of this.envs) w.postMessage({ id: this.nextId++, op: "open", env, description });
    return w;
  }

  private post(p: Pending): number {
    const id = this.nextId++;
    this.inFlight.set(id, p);
    this.ensure().postMessage({ ...p.body, id } as Request);
    return id;
  }

  private receive(m: Response): void {
    const p = this.inFlight.get(m.id);
    if (!p) return;
    this.inFlight.delete(m.id);
    if ("error" in m) p.fail(m.error);
    else p.resolve(m.result);
  }

  /** Send a call and resolve with its answer. */
  private call<T>(body: Body, onError: (message: string) => T): Promise<T> {
    if (this.disposed) return Promise.resolve(onError("the service was disposed"));
    return new Promise<T>((resolve) => {
      this.post({ body, resolve: resolve as (r: unknown) => void, fail: (msg) => resolve(onError(msg)) });
    });
  }

  /** A call through the gate of its environment and kind. */
  private gated<T>(key: string, body: Body, onError: (message: string) => T): Promise<T | null> {
    if (this.disposed) return Promise.resolve(onError("the service was disposed"));
    let g = this.gates.get(key);
    if (!g) {
      g = { busy: false, waiting: null };
      this.gates.set(key, g);
    }
    const gate = g;
    return new Promise<T | null>((resolve) => {
      const p: Pending = {
        body,
        resolve: (r) => {
          resolve(r as T);
          this.release(gate);
        },
        fail: (msg) => {
          resolve(onError(msg));
          this.release(gate);
        },
      };
      if (!gate.busy) {
        gate.busy = true;
        this.post(p);
      } else {
        gate.waiting?.replace();
        gate.waiting = { pending: p, replace: () => resolve(null) };
      }
    });
  }

  private release(gate: Gate): void {
    const next = gate.waiting;
    gate.waiting = null;
    if (next) this.post(next.pending);
    else gate.busy = false;
  }

  private run(body: Body, timeoutMs: number): Promise<RunResult> {
    if (this.disposed) return Promise.resolve(stopped(timeoutMs));
    return new Promise<RunResult>((resolve) => {
      const pending: Pending = {
        body,
        resolve: (r) => resolve(r as RunResult),
        fail: (msg) => resolve({ ok: false, stage: "run", problems: [{ phase: "service", code: "internal-error", message: msg }] }),
      };
      this.runs.push({ pending, timeoutMs });
      this.nextRun();
    });
  }

  private nextRun(): void {
    if (this.running || this.runs.length === 0) return;
    const { pending, timeoutMs } = this.runs.shift()!;
    const done = (fn: () => void) => () => {
      if (this.running) clearTimeout(this.running.timer);
      this.running = null;
      fn();
      this.nextRun();
    };
    const wrapped: Pending = {
      body: pending.body,
      resolve: (r) => done(() => pending.resolve(r))(),
      fail: (msg) => done(() => pending.fail(msg))(),
    };
    const id = this.post(wrapped);
    const timer = setTimeout(() => {
      this.inFlight.delete(id);
      this.running = null;
      this.restart();
      pending.resolve(stopped(timeoutMs));
      this.nextRun();
    }, timeoutMs + this.graceMs);
    this.running = { id, timer };
  }

  /** End the worker, and send every call still waiting to a new one. */
  private restart(): void {
    this.worker?.terminate();
    this.worker = null;
    const waiting = [...this.inFlight.values()];
    this.inFlight.clear();
    for (const p of waiting) this.post(p);
  }

  environment(description?: Description): RisorEnvironment {
    const env = this.nextEnv++;
    const service = this;
    let released = true;
    // Open the environment, again after a release: React's StrictMode
    // releases in an effect's cleanup and then uses the same object.
    const open = () => {
      if (!released) return;
      released = false;
      service.envs.set(env, description);
      if (service.worker) service.worker.postMessage({ id: service.nextId++, op: "open", env, description });
    };
    open();
    const problemsOnError = (message: string): Problem[] => [{ phase: "service", code: "internal-error", message }];
    return {
      check(text: string, options?: CheckOptions) {
        open();
        return service.gated(`${env}:check`, { op: "check", env, text, options }, problemsOnError);
      },
      validate(text: string, options?: CheckOptions) {
        open();
        return service.call({ op: "validate", env, text, options }, problemsOnError);
      },
      complete(text: string, offset: number, options?: CheckOptions) {
        open();
        return service.gated<Completion | null>(`${env}:complete`, { op: "complete", env, text, offset, options }, () => null);
      },
      hover(text: string, offset: number, options?: CheckOptions) {
        open();
        return service.gated<Hover | null>(`${env}:hover`, { op: "hover", env, text, offset, options }, () => null);
      },
      run(text: string, vars?: Record<string, unknown>, options?: RunOptions) {
        open();
        return service.run({ op: "run", env, text, vars, options }, options?.timeoutMs ?? DEFAULT_TIMEOUT_MS);
      },
      release() {
        if (released) return;
        released = true;
        service.envs.delete(env);
        for (const k of ["check", "complete", "hover"]) service.gates.delete(`${env}:${k}`);
        if (service.worker) service.worker.postMessage({ id: service.nextId++, op: "close", env });
      },
    };
  }

  format(text: string): Promise<FormatResult> {
    return this.call<FormatResult>({ op: "format", text }, (message) => ({ ok: false, problems: [{ phase: "service", code: "internal-error", message }] }));
  }

  dispose(): void {
    this.disposed = true;
    this.worker?.terminate();
    this.worker = null;
    if (this.running) clearTimeout(this.running.timer);
    this.running = null;
    for (const p of this.inFlight.values()) p.fail("the service was disposed");
    this.inFlight.clear();
    for (const r of this.runs.splice(0)) r.pending.resolve(stopped(r.timeoutMs));
    for (const g of this.gates.values()) g.waiting?.replace();
    this.gates.clear();
  }
}
