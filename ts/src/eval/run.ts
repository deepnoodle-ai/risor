// The state of one run: usage, the four limits, slots, variables (02 sections 5, 6, 8).

import type { Span } from "../syntax/source.ts";
import { BAD_NUMBER, Classifier, readHost } from "../value/value.ts";
import type { Kind } from "../value/value.ts";
import { RunFailure } from "./failure.ts";
import type { WalkCtx } from "./conform.ts";

export interface RunLimits {
  /** null: off. */
  steps: number | null;
  allocation: number | null;
  depth: number;
  /** Wall-clock milliseconds for the run, or null. */
  deadlineMs: number | null;
}

/** How the run asks the host for a variable: ABSENT_VAR when it is not supplied. */
export const ABSENT_VAR: unique symbol = Symbol("absent variable");
export type VarSupply = (name: string) => unknown;

export class Run {
  steps = 0;
  units = 0;
  /** The depth count of the calls in progress (02 8.4). */
  depth = 0;
  readonly stepLimit: number;
  readonly unitLimit: number;
  readonly depthLimit: number;
  readonly slots: unknown[];
  readonly cls: Classifier;
  private readonly deadline: number | null;
  readonly signal: AbortSignal | undefined;
  private nextCheck = 1024;
  private readonly supply: VarSupply;
  private readonly varCache = new Map<string, unknown>();

  constructor(limits: RunLimits, slotCount: number, cls: Classifier, supply: VarSupply, signal?: AbortSignal) {
    this.stepLimit = limits.steps ?? Infinity;
    this.unitLimit = limits.allocation ?? Infinity;
    this.depthLimit = limits.depth;
    this.deadline = limits.deadlineMs === null ? null : Date.now() + limits.deadlineMs;
    this.signal = signal;
    this.slots = new Array<unknown>(slotCount).fill(null);
    this.cls = cls;
    this.supply = supply;
    this.checkTime();
  }

  /** Charge steps (02 8.2). */
  charge(n: number): void {
    if (n === 0) return;
    if (this.steps + n > this.stepLimit) {
      throw new RunFailure("steps-exceeded", undefined, "default", { limit: this.stepLimit });
    }
    this.steps += n;
    if (this.steps >= this.nextCheck) {
      this.nextCheck = this.steps + 1024;
      this.checkTime();
    }
  }

  /** Charge allocation units (02 8.2). */
  alloc(n: number): void {
    if (n === 0) return;
    if (this.units + n > this.unitLimit) {
      throw new RunFailure("allocation-exceeded", undefined, "default", { limit: this.unitLimit });
    }
    this.units += n;
  }

  /** The deadline and the cancellation signal (02 8.3). */
  checkTime(): void {
    if (this.signal?.aborted) throw new RunFailure("cancelled", undefined, "default");
    if (this.deadline !== null && Date.now() > this.deadline) throw new RunFailure("deadline-exceeded", undefined, "default");
  }

  private walk: WalkCtx | null = null;

  /** The context of a conformance walk in this run (f04 5.3, 5.4). */
  walkCtx(): WalkCtx {
    if (this.walk === null) {
      const r = this;
      this.walk = {
        cls: this.cls,
        charge: (n) => r.charge(n),
        enter: () => {
          r.testDepth(r.depth + 1);
          r.depth++;
        },
        leave: () => {
          r.depth--;
        },
      };
    }
    return this.walk;
  }

  /** Test a depth count against the limit (02 8.4). */
  testDepth(count: number): void {
    if (count > this.depthLimit) {
      throw new RunFailure("depth-exceeded", undefined, "default", { limit: this.depthLimit });
    }
  }

  kind(v: unknown): Kind {
    return this.cls.kind(v);
  }

  /** Read a value taken out of host data or a container (03 3.4.4). */
  read(v: unknown, span: Span): unknown {
    const r = readHost(v);
    if (r === BAD_NUMBER) throw new RunFailure("number-range", span, "host");
    return r;
  }

  /** Read a variable of the environment, asking the host at most once (03 3.4.1). */
  variable(name: string, span: Span): unknown {
    if (this.varCache.has(name)) return this.read(this.varCache.get(name), span);
    const raw = this.supply(name);
    if (raw === ABSENT_VAR) throw new RunFailure("missing-variable", span, "default", { name });
    this.varCache.set(name, raw);
    return this.read(raw, span);
  }
}
