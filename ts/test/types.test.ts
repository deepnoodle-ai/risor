// Unit tests of the checker for what the data cannot hold (05):
// the laws of Join, Fits, Merge, Truthy and Falsy on hand-made types, printing
// read back through the parser, and the limits of 05-types.md section 20.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import {
  ANY, BOOL, FLOAT, INT, NEVER, NULL, STRING, falsyPart, fits, fromNode, join, lit, listOf, mapOf, merge, opaque,
  printType, record, truthyPart, typeEqual, withNull,
} from "../src/types/type.ts";
import type { Type } from "../src/types/type.ts";
import { parseType } from "../src/types/typetext.ts";
import { createEnv } from "../src/host/api.ts";

const req = (type: Type) => ({ type, optional: false });
const opt = (type: Type) => ({ type, optional: true });

const SAMPLES: Type[] = [
  ANY, NEVER, NULL, BOOL, INT, FLOAT, STRING, lit("a"), lit("a", "b"), lit(""), withNull(INT), listOf(INT), listOf(NEVER),
  listOf(withNull(STRING)), opaque("Time"), withNull(opaque("Time")), mapOf(FLOAT), mapOf(ANY), record([]),
  record([["a", req(INT)]]), record([["a", opt(INT)], ["b", req(withNull(STRING))]]), record([["a", req(INT)]], ANY),
  record([["x", req(listOf(record([["y", opt(BOOL)]])))]]), { num: "int", str: new Set(["fail", "pass"]), null: true },
];

const eq = (a: Type, b: Type, what: string) => assert.ok(typeEqual(a, b), `${what}: ${printType(a)} vs ${printType(b)}`);

describe("05 4.1: Join", () => {
  test("commutative, associative, idempotent; never is the identity; any absorbs", () => {
    for (const a of SAMPLES) {
      eq(join(a, a), a, `idempotent ${printType(a)}`);
      eq(join(a, NEVER), a, `never ${printType(a)}`);
      eq(join(a, ANY), ANY, `any ${printType(a)}`);
      for (const b of SAMPLES) {
        eq(join(a, b), join(b, a), `commutative ${printType(a)}, ${printType(b)}`);
        for (const c of SAMPLES.slice(0, 12)) eq(join(join(a, b), c), join(a, join(b, c)), "associative");
      }
    }
  });
  test("two records join into one record", () => {
    const t = join(record([["a", req(INT)]]), record([["b", req(STRING)]]));
    assert.equal(printType(t), "{a?: int, b?: string}");
    assert.equal(printType(join(INT, FLOAT)), "float");
    assert.equal(printType(join(lit("a"), STRING)), "string");
  });
});

describe("05 4.2: Fits", () => {
  test("reflexive, and each side fits the Join", () => {
    for (const a of SAMPLES) {
      assert.notEqual(fits(a, a), 0, `reflexive ${printType(a)}`);
      for (const b of SAMPLES) assert.notEqual(fits(a, join(a, b)), 0, `${printType(a)} fits Join with ${printType(b)}`);
    }
  });
  test("the examples of 4.2, and fits that use any", () => {
    const a = record([["a", req(INT)]]);
    assert.equal(fits(a, mapOf(INT)), 1);
    assert.equal(fits(a, record([["a", req(INT)]], ANY)), 1);
    assert.equal(fits(a, record([["a", req(INT)], ["b", opt(STRING)]])), 1);
    assert.equal(fits(record([["a", req(INT)], ["b", req(INT)]]), a), 0);
    assert.equal(fits(record([["a", opt(INT)]]), a), 0);
    assert.equal(fits(ANY, INT), 2);
    assert.equal(fits(listOf(ANY), listOf(INT)), 2);
    assert.equal(fits(NEVER, INT), 1);
    assert.equal(fits(INT, ANY), 1);
  });
});

describe("05 4.3 and 7.6: Merge", () => {
  test("a spread and a computed key", () => {
    const r = record([["a", req(INT)]]).map!;
    assert.equal(printType({ map: merge(r, record([["b", req(STRING)]]).map!) }), "{a: int, b: string}");
    assert.equal(printType({ map: merge(r, mapOf(lit("x")).map!) }), '{a: int | "x", ..."x"}');
    assert.equal(printType({ map: merge(r, ANY) }), "{a: any, ...}");
  });
});

describe("05 4.4: Truthy and Falsy", () => {
  test("Join(Truthy(T), Falsy(T)) is T", () => {
    for (const t of SAMPLES) {
      if (t === ANY) continue;
      const back = join(truthyPart(t), falsyPart(t));
      // The parts of a list and a record are themselves types, so compare by Fits both ways.
      assert.notEqual(fits(back, t), 0, printType(t));
      assert.notEqual(fits(t, back), 0, printType(t));
    }
  });
  test("the rows of the table", () => {
    assert.equal(printType(truthyPart(NULL)), "never");
    assert.equal(printType(falsyPart(record([["id", req(INT)]]))), "never");
    assert.equal(printType(falsyPart(opaque("Time"))), "never");
    assert.equal(printType(falsyPart(STRING)), '""');
    assert.equal(printType(falsyPart(listOf(INT))), "never[]");
  });
});

describe("05 3.9: printing reads back as the same type", () => {
  const ctx = { opaque: new Set(["Time"]) };
  for (const t of SAMPLES) {
    test(printType(t), () => {
      const p = parseType(printType(t));
      assert.ok(p.ok, printType(t));
      eq(fromNode(p.node, ctx), t, "round trip");
    });
  }
  test("the examples of 3.9", () => {
    assert.equal(printType(listOf(NEVER)), "never[]");
    assert.equal(printType(listOf(listOf(INT))), "int[][]");
    assert.equal(printType(withNull(lit("fail", "pass"))), '"fail" | "pass" | null');
    assert.equal(printType(listOf(withNull(INT))), "(int | null)[]");
    assert.equal(printType(record([["action", req(STRING)]], ANY)), "{action: string, ...}");
    assert.equal(printType(record([["owner", req(withNull(STRING))], ["note", opt(STRING)]])), "{note?: string, owner: string | null}");
  });
});

describe("05 section 20: the checker's limits", () => {
  const env = createEnv({ vars: "{x: int}" });
  const codes = (src: string) => {
    const c = env.check(src);
    return c.ok ? [] : c.diagnostics.map((d) => d.code);
  };
  test("20.2: the chain f0 to f100", () => {
    const chain = (n: number) => {
      let s = "const f0 = q => q;";
      for (let i = 1; i <= n; i++) s += ` const f${i} = q => f${i - 1}(q);`;
      return s + ` f${n}(1)`;
    };
    assert.deepEqual(codes(chain(99)), []);
    assert.deepEqual(codes(chain(100)), ["check-limit"]);
  });
  test("20.3: the 1,001st use", () => {
    const uses = (n: number) => "const f = q => q; [" + Array.from({ length: n }, () => "f(1)").join(", ") + "]";
    assert.deepEqual(codes(uses(1000)), []);
    assert.deepEqual(codes(uses(1001)), ["check-limit"]);
  });
  test("20.4: a type the checker builds deeper than 64", () => {
    assert.deepEqual(codes("[".repeat(64) + "]".repeat(64)), []);
    const c = env.check("[".repeat(65) + "]".repeat(65));
    assert.ok(!c.ok && c.diagnostics.some((d) => d.code === "check-limit"));
  });
  test("20.6: long flat programs are checked by loops", () => {
    const n = 100000;
    assert.deepEqual(codes(Array.from({ length: n }, () => "x").join(" + ")), []);
    assert.deepEqual(codes("x > 0" + " && x > 0".repeat(n)), []);
    assert.deepEqual(codes("x > 0 ? 1 : ".repeat(n) + "2"), []);
    assert.deepEqual(codes("if (x > 0) { 1 } " + "else if (x > 1) { 2 } ".repeat(n) + "else { 3 }"), []);
    assert.deepEqual(codes(Array.from({ length: n }, (_, i) => `const a${i} = ${i};`).join(" ") + " a0"), []);
    assert.deepEqual(codes("{a: {a: 1}}" + ".a".repeat(2)), []);
  });
  test("100 levels of nesting are checked without the host's stack", () => {
    assert.deepEqual(codes("(".repeat(100) + "x" + ")".repeat(100)), []);
    assert.deepEqual(codes("[".repeat(64) + "x" + "]".repeat(64)), []);
    assert.deepEqual(codes("-".repeat(100) + "x"), []);
  });
});

describe("the check stage of the host API (03 3.2)", () => {
  const env = createEnv({ vars: "{report: {verdict: string} | null}" });
  test("check alone gives the type and the sites", () => {
    const c = env.check('report && report.verdict == "APPROVE"');
    assert.ok(c.ok);
    assert.equal(c.type, "bool | null");
  });
  test("an expected result", () => {
    const c = env.check('report && report.verdict == "APPROVE"', "bool");
    assert.ok(!c.ok);
    assert.equal(c.diagnostics[0].code, "result-mismatch");
    assert.ok(env.check('report && report.verdict == "APPROVE"', "condition").ok);
  });
  test("compile with a check fails on a type error; without one it runs", () => {
    assert.throws(() => env.compile("report.verdict"));
    assert.equal(env.compile("report.verdict", { check: false }).run({ report: { verdict: "x" } }), "x");
  });
  test("a checked and an unchecked run of one program agree, but for the class (03 3.7.6)", () => {
    const e2 = createEnv({ vars: "{o: {a: int}}" });
    const checked = e2.compile("o.a + 1").execute({ o: {} });
    const unchecked = e2.compile("o.a + 1", { check: false }).execute({ o: {} });
    assert.ok(!checked.ok && !unchecked.ok);
    assert.deepEqual([checked.diagnostic.code, checked.diagnostic.span, checked.diagnostic.message], [unchecked.diagnostic.code, unchecked.diagnostic.span, unchecked.diagnostic.message]);
    assert.deepEqual(checked.usage, unchecked.usage);
    assert.equal(checked.diagnostic.class, "environment");
    assert.equal(unchecked.diagnostic.class, "dynamic");
  });
});
