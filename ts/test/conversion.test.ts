// Unit tests of package F4 for what no data case can raise: the binding's
// conformance function on host data, child environments with named types
// (f04 3.4), and a description that reads back (f04 8).

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, envFromDescription, RisorError } from "../src/host/api.ts";

function code(f: () => unknown): string {
  try {
    f();
  } catch (e) {
    assert.ok(e instanceof RisorError, String(e));
    return e.diagnostics[0].code;
  }
  assert.fail("no error");
}

const types = [
  { name: "Line", type: "{sku: string, qty: int}" },
  { name: "Order", type: "{id: string, lines: Line[], ...}" },
];

describe("the conformance function (f04 7)", () => {
  const env = createEnv({ types });

  test("null when the value conforms; the diagnostic, with no span, when not", () => {
    assert.equal(env.conform({ id: "a", lines: [] }, "Order"), null);
    const d = env.conform({ id: "a", lines: [{ sku: "x", qty: 1 }, { sku: "y", qty: "2" }] }, "Order");
    assert.deepEqual(d, { phase: "run", code: "not-conforming", class: "value", path: ["lines", 1, "qty"], message: 'expected int, found "2"', trace: [] });
  });

  test("host maps, bigints and numbers that cannot enter", () => {
    assert.equal(env.conform(new Map<string, unknown>([["sku", "x"], ["qty", 2n]]), "Line"), null);
    assert.equal(env.conform({ sku: "x", qty: Infinity }, "Line")?.message, "expected int, found number");
  });

  test("cyclic data cannot make it loop: its depth is bounded by the type", () => {
    const a: Record<string, unknown> = { id: "a", lines: [] };
    a.self = a;
    assert.equal(env.conform(a, "Order"), null);
    assert.equal(env.conform(a, "{id: string, self: {id: int, ...}, ...}")?.message, "expected int, found \"a\"");
  });

  test("bad type text is the declare error bad-type", () => {
    assert.equal(code(() => env.conform(1, "Item")), "bad-type");
    assert.equal(code(() => env.conform(1, "{a: int} | {b: int}")), "bad-type");
  });

  test("it agrees with as and is", () => {
    const child = env.child({ vars: "{v: any}" });
    const v = { id: "a", lines: [{ sku: "x" }] };
    assert.equal(child.compile("v is Order").run({ v }), false);
    const o = child.compile("v as Order").execute({ v });
    assert.ok(!o.ok);
    assert.deepEqual(o.diagnostic.path, env.conform(v, "Order")?.path);
  });
});

describe("named types in a child environment (f04 3.4)", () => {
  test("a child's named type replaces the parent's in the child only; a parent's type keeps what it had", () => {
    const parent = createEnv({ types, vars: "{v: any}" });
    const child = parent.child({ types: [{ name: "Line", type: "{sku: string}" }] });
    assert.equal(child.compile("v is Line").run({ v: { sku: "x" } }), true);
    assert.equal(parent.compile("v is Line").run({ v: { sku: "x" } }), false);
    // Order was resolved with the parent's Line.
    assert.equal(child.compile("v is Order").run({ v: { id: "a", lines: [{ sku: "x" }] } }), false);
  });

  test("a parent's opaque name for a named type, or a named type's name for an opaque type, is name-clash", () => {
    const parent = createEnv({ types, opaque: { Time: { test: () => false } } });
    assert.equal(code(() => parent.child({ types: [{ name: "Time", type: "int" }] })), "name-clash");
    assert.equal(code(() => parent.child({ opaque: { Line: { test: () => false } } })), "name-clash");
  });
});

describe("the description (f04 8)", () => {
  test("types, printed by structure, read back", () => {
    const env = createEnv({ types, vars: "{o: Order}", functions: { total: { type: "(o: Order) => float", call: () => 1 } } });
    const d = env.describe();
    assert.deepEqual(d.types, { Line: "{qty: int, sku: string}", Order: "{id: string, lines: {qty: int, sku: string}[], ...}" });
    assert.equal(d.variables.o, "{id: string, lines: {qty: int, sku: string}[], ...}");
    assert.equal(d.functions.total, "(o: {id: string, lines: {qty: int, sku: string}[], ...}) => float");
    const back = envFromDescription(JSON.parse(JSON.stringify(d)));
    assert.deepEqual(back.describe(), d);
  });

  test("no key types when there is no named type", () => {
    assert.equal("types" in createEnv({}).describe(), false);
  });
});

describe("ConformAll (f08 6)", () => {
  const env = createEnv({});
  const t = "{id: string, lines: {qty: int, sku: string}[]}";
  const v = { id: 7, lines: [{ sku: "x", qty: 1 }, { sku: "y", qty: "2" }], note: 1 };
  test("every failure up to the maximum, the first as the conformance function gives it", () => {
    const all = env.conformAll(v, t, 3);
    assert.deepEqual(all.map((d) => [d.path, d.message]), [
      [["id"], "expected string, found 7"],
      [["lines", 1, "qty"], 'expected int, found "2"'],
      [["note"], 'the type does not allow key "note"'],
    ]);
    assert.equal(env.conformAll(v, t, 2).length, 2);
    assert.equal(env.conformAll(v, t, 0).length, 1);
    assert.deepEqual(all[0].path, env.conform(v, t)?.path);
  });
});
