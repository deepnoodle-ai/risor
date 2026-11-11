// Unit tests of package F4 for what no data case can raise: the binding's
// conformance function on host data, child environments with named types
// (f04 3.4), and a description that reads back (f04 8).

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { conform, conformAll, createEnv, Env, execute, parse, RisorError } from "../src/host/api.ts";

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
  const env = createEnv({ namedTypes: types });

  test("null when the value conforms; the diagnostic, with no span, when not", () => {
    assert.equal(conform({ id: "a", lines: [] }, env.parseType("Order")), null);
    const d = conform({ id: "a", lines: [{ sku: "x", qty: 1 }, { sku: "y", qty: "2" }] }, env.parseType("Order"));
    assert.deepEqual(d, { phase: "run", code: "not-conforming", class: "value", path: ["lines", 1, "qty"], message: 'expected int, found "2"', trace: [] });
  });

  test("host maps, bigints and numbers that cannot enter", () => {
    assert.equal(conform(new Map<string, unknown>([["sku", "x"], ["qty", 2n]]), env.parseType("Line")), null);
    assert.equal(conform({ sku: "x", qty: Infinity }, env.parseType("Line"))?.message, "expected int, found number");
  });

  test("cyclic data cannot make it loop: its depth is bounded by the type", () => {
    const a: Record<string, unknown> = { id: "a", lines: [] };
    a.self = a;
    assert.equal(conform(a, env.parseType("Order")), null);
    assert.equal(conform(a, env.parseType("{id: string, self: {id: int, ...}, ...}"))?.message, "expected int, found \"a\"");
  });

  test("bad type text is the declare error bad-type", () => {
    assert.equal(code(() => conform(1, env.parseType("Item"))), "bad-type");
    assert.equal(code(() => conform(1, env.parseType("{a: int} | {b: int}"))), "bad-type");
  });

  test("it agrees with as and is", () => {
    const child = env.with({ varRecord: "{v: any}" });
    const v = { id: "a", lines: [{ sku: "x" }] };
    assert.equal(child.compile(parse("v is Order")).run({ v }), false);
    const o = execute(child.compile(parse("v as Order")), { v });
    assert.ok(!o.ok);
    assert.deepEqual(o.diagnostic.path, conform(v, env.parseType("Order"))?.path);
  });
});

describe("named types in a child environment (f04 3.4)", () => {
  test("a child's named type replaces the parent's in the child only; a parent's type keeps what it had", () => {
    const parent = createEnv({ namedTypes: types, varRecord: "{v: any}" });
    const child = parent.with({ namedTypes: [{ name: "Line", type: "{sku: string}" }] });
    assert.equal(child.compile(parse("v is Line")).run({ v: { sku: "x" } }), true);
    assert.equal(parent.compile(parse("v is Line")).run({ v: { sku: "x" } }), false);
    // Order was resolved with the parent's Line.
    assert.equal(child.compile(parse("v is Order")).run({ v: { id: "a", lines: [{ sku: "x" }] } }), false);
  });

  test("a parent's opaque name for a named type, or a named type's name for an opaque type, is name-clash", () => {
    const parent = createEnv({ namedTypes: types, opaque: { Time: { test: () => false } } });
    assert.equal(code(() => parent.with({ namedTypes: [{ name: "Time", type: "int" }] })), "name-clash");
    assert.equal(code(() => parent.with({ opaque: { Line: { test: () => false } } })), "name-clash");
  });
});

describe("the description (f04 8)", () => {
  test("types, printed by structure, read back", () => {
    const env = createEnv({ namedTypes: types, varRecord: "{o: Order}", functions: { total: { type: "(o: Order) => float", call: () => 1 } } });
    const d = env.describe();
    assert.deepEqual(d.types, { Line: "{qty: int, sku: string}", Order: "{id: string, lines: {qty: int, sku: string}[], ...}" });
    assert.equal(d.variables.o, "{id: string, lines: {qty: int, sku: string}[], ...}");
    assert.equal(d.functions.total, "(o: {id: string, lines: {qty: int, sku: string}[], ...}) => float");
    const back = Env.fromDescription(JSON.parse(JSON.stringify(d)));
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
    const all = conformAll(v, env.parseType(t), 3);
    assert.deepEqual(all.map((d) => [d.path, d.message]), [
      [["id"], "expected string, found 7"],
      [["lines", 1, "qty"], 'expected int, found "2"'],
      [["note"], 'the type does not allow key "note"'],
    ]);
    assert.equal(conformAll(v, env.parseType(t), 2).length, 2);
    assert.equal(conformAll(v, env.parseType(t), 0).length, 1);
    assert.deepEqual(all[0].path, conform(v, env.parseType(t))?.path);
  });
});
