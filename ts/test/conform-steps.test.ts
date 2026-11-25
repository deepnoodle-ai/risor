// Unit tests of f04 7.3 and f08 6.4: the conformance function and
// conformAll count steps against a step limit. Host data that holds one list
// at every level has 2^levels places; each is charged, so the limit ends the
// walk (#678). A case file holds no shared data, so these are unit tests.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { conform, conformAll, createEnv } from "../src/host/api.ts";

/** A list whose two elements are the same inner list, at each of n levels. */
function shared(n: number, leaf: unknown): unknown {
  let v: unknown[] = [leaf, leaf];
  for (let i = 1; i < n; i++) v = [v, v];
  return v;
}

const nested = (n: number) => "int" + "[]".repeat(n);

function timed<T>(what: string, f: () => T): T {
  const start = performance.now();
  const out = f();
  const ms = performance.now() - start;
  console.log(`30 shared levels, ${what}: ${ms.toFixed(1)} ms`);
  assert.ok(ms < 1000, `${what} took ${ms} ms`);
  return out;
}

describe("the conformance function counts steps (f04 7.3)", () => {
  const env = createEnv({});

  test("30 levels of shared lists end at the default limit, quickly", () => {
    const t = env.parseType(nested(30));
    const v = shared(30, 1);
    const d = timed("conform", () => conform(v, t));
    assert.deepEqual(d, { phase: "run", code: "steps-exceeded", class: "limit", message: "the run needed more than its limit of 1000000 steps", trace: [] });
    const all = timed("conformAll", () => conformAll(v, t, 5));
    assert.equal(all.length, 1);
    assert.equal(all[0].code, "steps-exceeded");
    // A failing leaf fails at the first leaf, long before the limit.
    const bad = shared(30, "x");
    const f = timed("conform, failing leaf", () => conform(bad, t));
    assert.equal(f?.code, "not-conforming");
    assert.equal(f?.path?.length, 30);
    const fs = timed("conformAll, failing leaf", () => conformAll(bad, t, 5));
    assert.equal(fs.length, 5);
    assert.ok(fs.every((x) => x.code === "not-conforming"));
  });

  test("the option gives the limit; null turns it off; below 0 is 0", () => {
    const t = env.parseType(nested(10));
    const v = shared(10, 1);
    // 2^11 - 1 places, each charged 1.
    assert.equal(conform(v, t, { steps: 2047 }), null);
    assert.equal(conform(v, t, { steps: 2046 })?.message, "the run needed more than its limit of 2046 steps");
    assert.equal(conform(shared(16, 1), env.parseType(nested(16)), { steps: null }), null);
    assert.equal(conform(1, env.parseType("int"), { steps: -5 })?.message, "the run needed more than its limit of 0 steps");
    assert.equal(conform(1, env.parseType("any"), { steps: 0 }), null);
  });

  test("the environment's step limit plays no part (R320)", () => {
    const small = createEnv({ limits: { steps: 10 } });
    assert.equal(conform([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12], small.parseType("int[]")), null);
  });

  test("conformAll ends with the limit after the failures it found (f08 6.4 step 3)", () => {
    const t = env.parseType("int[]");
    // The first walk fails at [1] with 3 steps; All charges 4.
    const ds = conformAll([1, "a", "b"], t, 5, { steps: 3 });
    assert.deepEqual(ds.map((d) => [d.code, d.path]), [["not-conforming", [1]], ["steps-exceeded", undefined]]);
    assert.equal(conformAll([1, "a", "b"], t, 1, { steps: 3 }).length, 1);
    const two = conformAll(["a", "b", "c", "d"], t, 2, { steps: 2 });
    assert.deepEqual(two.map((d) => [d.code, d.path]), [["not-conforming", [0]], ["steps-exceeded", undefined]]);
  });
});
