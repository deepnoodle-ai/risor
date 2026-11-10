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

/** The variables a host gives a run: a function, a Map or a plain object. */
export type VarSource = ((name: string) => unknown) | ReadonlyMap<string, unknown> | Record<string, unknown> | undefined;

/** A variable not yet asked for in this run. */
const UNSET: unique symbol = Symbol("unset");
/** A variable whose host value cannot enter the language: each read fails. */
const BAD_VAR: unique symbol = Symbol("bad variable");

const hasOwn = Object.prototype.hasOwnProperty;

function supplied(vars: VarSource, name: string): unknown {
  if (vars === undefined) return ABSENT_VAR;
  let v: unknown;
  if (typeof vars === "function") v = vars(name);
  else if (vars instanceof Map) v = vars.get(name);
  else v = hasOwn.call(vars, name) ? (vars as Record<string, unknown>)[name] : undefined;
  return v === undefined ? ABSENT_VAR : v;
}

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
  private readonly vars: VarSource;
  /** Where the variables start in `slots`: each variable of the program, by its index, holds its value once read, UNSET or BAD_VAR. */
  private readonly varBase: number;

  constructor(limits: RunLimits, slotCount: number, cls: Classifier, vars: VarSource, varCount: number, signal?: AbortSignal) {
    this.stepLimit = limits.steps ?? Infinity;
    this.unitLimit = limits.allocation ?? Infinity;
    this.depthLimit = limits.depth;
    this.deadline = limits.deadlineMs === null ? null : Date.now() + limits.deadlineMs;
    this.signal = signal;
    const n = slotCount + varCount;
    const slots = new Array<unknown>(n);
    for (let i = 0; i < slotCount; i++) slots[i] = null;
    for (let i = slotCount; i < n; i++) slots[i] = UNSET;
    this.slots = slots;
    this.varBase = slotCount;
    this.cls = cls;
    this.vars = vars;
    if (signal !== undefined || this.deadline !== null) this.checkTime();
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

  /** Read variable `i` of the program, named `name`, asking the host at most once (03 3.4.1). */
  variable(i: number, name: string, span: Span): unknown {
    const at = this.varBase + i;
    const v = this.slots[at];
    if (v !== UNSET && v !== BAD_VAR) return v;
    if (v === BAD_VAR) throw new RunFailure("number-range", span, "host");
    const raw = supplied(this.vars, name);
    if (raw === ABSENT_VAR) throw new RunFailure("missing-variable", span, "default", { name });
    const r = readHost(raw);
    if (r === BAD_NUMBER) {
      this.slots[at] = BAD_VAR;
      throw new RunFailure("number-range", span, "host");
    }
    this.slots[at] = r;
    return r;
  }
}
