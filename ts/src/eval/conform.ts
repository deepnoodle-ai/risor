// The conformance walk (package F4, f04 4): the order in which a value is
// tested against a resolved type, what each step charges, and the path and
// template of the first failure. Used by `as` and `is` in a run (charged, with
// the depth count) and by the conformance function (f04 7.3: the same steps,
// counted against a step limit of its own).

import { isAny, isNever, printType, tagSetOf, unfold } from "../types/type.ts";
import type { Alts, Rec, Type, Union } from "../types/type.ts";
import { ABSENT, BAD_NUMBER, Classifier, compareText, mapGet, mapKeys, readHost, size, div64 } from "../value/value.ts";

/** The value of `found` for a host number that cannot enter the language (f04 4.4 item 3). */
export const CANNOT_ENTER: unique symbol = Symbol("a number that cannot enter");

export type PathStep = string | number;

export interface ConformFailure {
  path: PathStep[];
  /**
   * `depth`: the conformance function went past the default depth limit (f17 12.7 item 3).
   * `steps`: it went past its step limit (f04 7.3). Neither has a path.
   */
  template: "wrong" | "missing" | "extra" | "depth" | "steps";
  params: Record<string, unknown>;
}

/** The levels the conformance function enters at most: the default depth limit (f17 12.7 item 3). */
export const CONFORM_DEPTH = 200;

/** The failure of the conformance function past the default depth limit. */
export const DEPTH_FAILURE: ConformFailure = { path: [], template: "depth", params: { limit: CONFORM_DEPTH } };

/** The default step limit of the conformance function (f04 7.3, 03 3.6.1). */
export const CONFORM_STEPS = 1_000_000;

/** The failure of the conformance function past its step limit (f04 7.3). */
export function stepsFailure(limit: number): ConformFailure {
  return { path: [], template: "steps", params: { limit } };
}

/** The conformance function went past its depth limit. */
class DepthStop extends Error {}

/** The conformance function went past its step limit. */
class StepStop extends Error {}

/** A step count against a limit (null: none), outside a run (f04 7.3, f08 6.4). */
class Meter {
  count = 0;
  readonly limit: number | null;
  constructor(limit: number | null) {
    this.limit = limit;
  }
  charge(n: number): void {
    if (n <= 0) return;
    if (this.limit !== null && n > this.limit - this.count) throw new StepStop();
    this.count += n;
  }
}

/** What a walk charges and counts. */
export interface WalkCtx {
  readonly cls: Classifier;
  /** The step limit of the conformance function's context (f04 7.3); absent in a run. */
  readonly stepLimit?: number | null;
  charge(n: number): void;
  /** Enter a list or map (f04 5.4): the depth test, then one more level. */
  enter(): void;
  leave(): void;
}

/** The type printed by structure, only when a message is built (f04 4.4). */
function printed(t: Type): { toString(): string } {
  let s: string | null = null;
  return { toString: () => (s ??= printType(t)) };
}

const sortedKeys = new WeakMap<Rec, string[]>();

function declaredKeys(r: Rec): string[] {
  let ks = sortedKeys.get(r);
  if (ks === undefined) {
    ks = [...r.fields.keys()].sort(compareText);
    sortedKeys.set(r, ks);
  }
  return ks;
}

/** A whole number (01 6.2): an integer of magnitude at most 2^53 - 1. */
function whole(n: number): boolean {
  return Number.isSafeInteger(n);
}

class Walker {
  private readonly path: PathStep[] = [];
  private readonly ctx: WalkCtx;
  constructor(ctx: WalkCtx) {
    this.ctx = ctx;
  }

  private fail(template: ConformFailure["template"], params: Record<string, unknown>): ConformFailure {
    return { path: [...this.path], template, params };
  }

  private wrong(t: Type, found: unknown): ConformFailure {
    return this.fail("wrong", { expected: printed(t), found });
  }

  /**
   * Test(v, T, path) of f04 4.2. `charged`: the step of 4.2 step 3 was the
   * lookup of a declared field. With package F17 the walk tests the value
   * against the unfolding of T, and a failure prints T as it is: a recursive
   * type by its name (f17 12.7 items 1 and 2).
   */
  test(raw: unknown, t0: Type, charged = false): ConformFailure | null {
    const t = unfold(t0);
    if (isAny(t)) return null;
    const ctx = this.ctx;
    const v = readHost(raw);
    const k = v === BAD_NUMBER ? null : ctx.cls.kind(v);
    const enters = (k === "list" && t.list !== undefined) || (k === "map" && t.map !== undefined);
    if (enters) ctx.enter();
    if (!charged) ctx.charge(1);
    const out = this.alternative(v, k, t, t0);
    if (enters) ctx.leave();
    return out;
  }

  private alternative(v: unknown, k: string | null, t: Exclude<Type, { any: true }>, shown: Type): ConformFailure | null {
    const wrong = (found: unknown) => this.wrong(shown, found);
    switch (k) {
      case null:
        return wrong(CANNOT_ENTER);
      case "null":
        return t.null ? null : wrong(v);
      case "bool":
        return t.bool ? null : wrong(v);
      case "number":
        if (t.num === "float" || (t.num === "int" && whole(v as number))) return null;
        return wrong(v);
      case "string": {
        const s = t.str;
        if (s === undefined) return wrong(v);
        if (s === "string") return null;
        this.ctx.charge(size(v as string));
        return s.has(v as string) ? null : wrong(v);
      }
      case "list": {
        if (t.list === undefined) return wrong(v);
        const xs = v as unknown[];
        for (let i = 0; i < xs.length; i++) {
          this.path.push(i);
          const f = this.test(xs[i], t.list);
          this.path.pop();
          if (f !== null) return f;
        }
        return null;
      }
      case "map":
        if (t.map === undefined) return wrong(v);
        return t.union === undefined ? this.record(v as object, t.map) : this.union(v as object, t.union);
      default: {
        const name = this.ctx.cls.opaqueName(v);
        return name !== null && t.opaque?.has(name) === true ? null : wrong(v);
      }
    }
  }

  /** f05 7.2: the tag first, then the rows of f04 4.3 for its variant, passing over the tag. */
  private union(m: object, u: Union): ConformFailure | null {
    const tag = u.tag;
    this.ctx.charge(1 + size(tag));
    const val = mapGet(m, tag);
    this.path.push(tag);
    if (val === ABSENT) {
      const f = this.fail("missing", { key: tag });
      this.path.pop();
      return f;
    }
    const tagType: Alts = { str: new Set(u.variants.flatMap((v) => [...tagSetOf(v, tag)])) };
    const f = this.test(val, tagType, true);
    this.path.pop();
    if (f !== null) return f;
    const s = readHost(val) as string;
    const variant = u.variants.find((v) => tagSetOf(v, tag).has(s))!;
    return this.record(m, variant, tag);
  }

  /** f04 4.3. `skip`: the tag of a union, already looked up and tested (f05 7.2 step 3). */
  private record(m: object, r: Rec, skip: string | null = null): ConformFailure | null {
    const ctx = this.ctx;
    let present = skip === null ? 0 : 1;
    for (const key of declaredKeys(r)) {
      if (key === skip) continue;
      const fl = r.fields.get(key)!;
      ctx.charge(1 + size(key));
      const val = mapGet(m, key);
      if (val === ABSENT) {
        if (!fl.optional) {
          this.path.push(key);
          const f = this.fail("missing", { key });
          this.path.pop();
          return f;
        }
        continue;
      }
      present++;
      this.path.push(key);
      const f = this.test(val, fl.type, true);
      this.path.pop();
      if (f !== null) return f;
    }
    if (isAny(r.rest)) return null;
    const keys = mapKeys(m);
    if (keys.length - present === 0) return null;
    const others = keys.filter((x) => !r.fields.has(x)).sort(compareText);
    let together = 0;
    for (const x of others) together += x.length;
    const sz = div64(together);
    if (isNever(r.rest)) {
      // An exact record (4.3 step 4).
      ctx.charge(others.length + sz);
      this.path.push(others[0]);
      const f = this.fail("extra", { key: others[0] });
      this.path.pop();
      return f;
    }
    // Another rest (4.3 step 5): every other key, each to its end.
    ctx.charge(sz);
    let first: ConformFailure | null = null;
    for (const x of others) {
      this.path.push(x);
      const f = this.test(mapGet(m, x), r.rest);
      this.path.pop();
      if (f !== null && first === null) first = f;
    }
    return first;
  }
}

/**
 * Test a value against a resolved type (f04 4.2). With the conformance
 * function's context, DEPTH_FAILURE past its depth limit, and the failure
 * `steps` past its step limit.
 */
export function conformWalk(v: unknown, t: Type, ctx: WalkCtx): ConformFailure | null {
  try {
    return new Walker(ctx).test(v, t);
  } catch (e) {
    if (e instanceof DepthStop) return DEPTH_FAILURE;
    if (e instanceof StepStop) return stepsFailure(ctx.stepLimit!);
    throw e;
  }
}

/**
 * The conformance function's context (f04 7.1 item 1, 7.3). It charges the
 * steps a run's walk charges, against `steps` (null: no limit), the default
 * 1,000,000. It counts the levels it enters, and stops past the default depth
 * limit, 200 (f17 12.7 item 3): a recursive type does not bound its depth.
 */
export function freeCtx(cls: Classifier, steps: number | null = CONFORM_STEPS): WalkCtx {
  let level = 0;
  const meter = new Meter(steps);
  return {
    cls,
    stepLimit: steps,
    charge(n: number) {
      meter.charge(n);
    },
    enter() {
      if (++level > CONFORM_DEPTH) throw new DepthStop();
    },
    leave() {
      level--;
    },
  };
}

/**
 * The walk of every failure (package F8, f08 6.2): failures in order, up to
 * `max`, under the step limit `steps` (null: none) by f08 6.4. First the walk
 * of f04 4.2: a limit there is the whole list. Then All, from a new count,
 * charging at the places that walk charges; past the limit the list ends with
 * the failure `steps`.
 */
export function conformAllWalk(v: unknown, t: Type, cls: Classifier, max: number, steps: number | null = CONFORM_STEPS): ConformFailure[] {
  // F8 6.1: a maximum below 1 is read as 1.
  if (!(max >= 1)) max = 1;
  // f08 6.4 step 1, f17 12.7 item 3: the first failure's walk, whose limit ends ConformAll.
  const first = conformWalk(v, t, freeCtx(cls, steps));
  if (first !== null && (first.template === "depth" || first.template === "steps")) return [first];
  const meter = new Meter(steps);
  const out: ConformFailure[] = [];
  const path: PathStep[] = [];
  const full = () => out.length >= max;
  const add = (template: ConformFailure["template"], params: Record<string, unknown>) => {
    out.push({ path: [...path], template, params });
  };
  const wrong0 = (ty: Type, found: unknown) => add("wrong", { expected: printed(ty), found });
  const at = (step: PathStep, f: () => void) => {
    path.push(step);
    f();
    path.pop();
  };
  // f17 12.7 item 3: the levels entered; past the default depth limit the walk ends with DEPTH_FAILURE.
  let level = 0;
  // `looked`: the value of a declared field, whose step is its lookup, already charged (f08 6.4 step 2).
  const all = (raw: unknown, shown: Type, looked = false): void => {
    const ty = unfold(shown);
    if (full() || isAny(ty)) return;
    const wrong = (_t: Type, found: unknown) => wrong0(shown, found);
    const x = readHost(raw);
    if (x === BAD_NUMBER) {
      if (!looked) meter.charge(1);
      return wrong(ty, CANNOT_ENTER);
    }
    const k = cls.kind(x);
    if ((k === "list" && ty.list !== undefined) || (k === "map" && ty.map !== undefined)) {
      if (level + 1 > CONFORM_DEPTH) throw new DepthStop();
      level++;
      try {
        if (!looked) meter.charge(1);
        all0(x, k, ty, wrong);
      } finally {
        level--;
      }
      return;
    }
    if (!looked) meter.charge(1);
    all0(x, k, ty, wrong);
  };
  const all0 = (x: unknown, k: string, ty: Exclude<Type, { any: true }>, wrong: (t: Type, found: unknown) => void): void => {
    switch (k) {
      case "null":
        if (!ty.null) wrong(ty, x);
        return;
      case "bool":
        if (!ty.bool) wrong(ty, x);
        return;
      case "number":
        if (!(ty.num === "float" || (ty.num === "int" && whole(x as number)))) wrong(ty, x);
        return;
      case "string":
        if (ty.str === undefined) return wrong(ty, x);
        if (ty.str === "string") return;
        meter.charge(size(x as string));
        if (!ty.str.has(x as string)) wrong(ty, x);
        return;
      case "list": {
        if (ty.list === undefined) return wrong(ty, x);
        const xs = x as unknown[];
        for (let i = 0; i < xs.length && !full(); i++) at(i, () => all(xs[i], ty.list!));
        return;
      }
      case "map": {
        if (ty.map === undefined) return wrong(ty, x);
        if (ty.union !== undefined) {
          // f08 6.2 step 7: the tag first; a missing or unknown tag stops this value.
          const u = ty.union;
          const tag = u.tag;
          meter.charge(1 + size(tag));
          const tv = mapGet(x as object, tag);
          if (tv === ABSENT) return at(tag, () => add("missing", { key: tag }));
          const tagType: Alts = { str: new Set(u.variants.flatMap((r) => [...tagSetOf(r, tag)])) };
          const s = readHost(tv);
          if (typeof s === "string") meter.charge(size(s));
          const variant = typeof s === "string" ? u.variants.find((r) => tagSetOf(r, tag).has(s)) : undefined;
          if (variant === undefined) return at(tag, () => wrong0(tagType, s === BAD_NUMBER ? CANNOT_ENTER : s));
          return record(x as object, variant, tag);
        }
        return record(x as object, ty.map, null);
      }
      default: {
        const name = cls.opaqueName(x);
        if (!(name !== null && ty.opaque?.has(name) === true)) wrong(ty, x);
      }
    }
  };
  const record = (m: object, r: Rec, skip: string | null): void => {
    for (const key of declaredKeys(r)) {
      if (full()) return;
      if (key === skip) continue;
      const fl = r.fields.get(key)!;
      meter.charge(1 + size(key));
      const val = mapGet(m, key);
      if (val === ABSENT) {
        if (!fl.optional) at(key, () => add("missing", { key }));
        continue;
      }
      at(key, () => all(val, fl.type, true));
    }
    if (isAny(r.rest) || full()) return;
    const others = mapKeys(m).filter((x) => !r.fields.has(x)).sort(compareText);
    const exact = isNever(r.rest);
    // f08 6.4 step 2: the other keys, charged as f04 4.3 steps 4 and 5 charge them.
    let together = 0;
    for (const x of others) together += x.length;
    meter.charge((exact ? others.length : 0) + div64(together));
    for (const key of others) {
      if (full()) return;
      if (exact) at(key, () => add("extra", { key }));
      else at(key, () => all(mapGet(m, key), r.rest));
    }
  };
  try {
    all(v, t);
  } catch (e) {
    if (e instanceof StepStop) {
      // f08 6.4 step 3: the failures found, then the limit's.
      out.push(stepsFailure(steps!));
      return out;
    }
    if (!(e instanceof DepthStop)) throw e;
    // F8 6.1 with f17 12.7 item 3: ConformAll ends with the depth failure.
    out.push(DEPTH_FAILURE);
  }
  return out;
}
