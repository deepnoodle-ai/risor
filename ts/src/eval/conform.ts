// The conformance walk (package F4, f04 4): the order in which a value is
// tested against a resolved type, what each step charges, and the path and
// template of the first failure. Used by `as` and `is` in a run (charged, with
// the depth count) and by the conformance function (f04 7: nothing charged).

import { isAny, printType, tagSetOf } from "../types/type.ts";
import type { Alts, Rec, Type, Union } from "../types/type.ts";
import { ABSENT, BAD_NUMBER, Classifier, compareText, mapGet, mapKeys, readHost, size } from "../value/value.ts";

/** The value of `found` for a host number that cannot enter the language (f04 4.4 item 3). */
export const CANNOT_ENTER: unique symbol = Symbol("a number that cannot enter");

export type PathStep = string | number;

export interface ConformFailure {
  path: PathStep[];
  template: "wrong" | "missing" | "extra";
  params: Record<string, unknown>;
}

/** What a walk charges and counts. */
export interface WalkCtx {
  readonly cls: Classifier;
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

  /** Test(v, T, path) of f04 4.2. `charged`: the step of 4.2 step 3 was the lookup of a declared field. */
  test(raw: unknown, t: Type, charged = false): ConformFailure | null {
    if (isAny(t)) return null;
    const ctx = this.ctx;
    const v = readHost(raw);
    const k = v === BAD_NUMBER ? null : ctx.cls.kind(v);
    const enters = (k === "list" && t.list !== undefined) || (k === "map" && t.map !== undefined);
    if (enters) ctx.enter();
    if (!charged) ctx.charge(1);
    const out = this.alternative(v, k, t);
    if (enters) ctx.leave();
    return out;
  }

  private alternative(v: unknown, k: string | null, t: Exclude<Type, { any: true }>): ConformFailure | null {
    switch (k) {
      case null:
        return this.wrong(t, CANNOT_ENTER);
      case "null":
        return t.null ? null : this.wrong(t, v);
      case "bool":
        return t.bool ? null : this.wrong(t, v);
      case "number":
        if (t.num === "float" || (t.num === "int" && whole(v as number))) return null;
        return this.wrong(t, v);
      case "string": {
        const s = t.str;
        if (s === undefined) return this.wrong(t, v);
        if (s === "string") return null;
        this.ctx.charge(size(v as string));
        return s.has(v as string) ? null : this.wrong(t, v);
      }
      case "list": {
        if (t.list === undefined) return this.wrong(t, v);
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
        if (t.map === undefined) return this.wrong(t, v);
        return t.union === undefined ? this.record(v as object, t.map) : this.union(v as object, t.union);
      default: {
        const name = this.ctx.cls.opaqueName(v);
        return name !== null && t.opaque?.has(name) === true ? null : this.wrong(t, v);
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
    const sz = Math.floor(together / 64);
    if (!r.rest.null && !r.rest.bool && !r.rest.num && !r.rest.str && !r.rest.list && !r.rest.map && !(r.rest.opaque && r.rest.opaque.size > 0)) {
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

/** Test a value against a resolved type (f04 4.2). */
export function conformWalk(v: unknown, t: Type, ctx: WalkCtx): ConformFailure | null {
  return new Walker(ctx).test(v, t);
}

/** The conformance function's context (f04 7.1 item 1): nothing charged, no limit. */
export function freeCtx(cls: Classifier): WalkCtx {
  return { cls, charge() {}, enter() {}, leave() {} };
}
