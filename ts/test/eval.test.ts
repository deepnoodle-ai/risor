// Unit tests of the evaluator: the examples of 01-language.md section 15 and
// the worked counts of 02-evaluation.md 7 that need no library entry beyond
// S2's. The suite's case files are the main test; these pin the chapters.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, execute, parse } from "../src/host/api.ts";
import { plainEqual } from "../src/value/value.ts";

const env = createEnv({ varRecord: "{u: any, m: any, s: any, t: any, xs: any, d: any, n: any}" });
const vars = { u: { name: "", tags: [] } };

function outcome(src: string, v: Record<string, unknown> = vars): { value?: unknown; code?: string } {
  const o = execute(env.compile(parse(src), { check: false }), v);
  return o.ok ? { value: o.value } : { code: o.diagnostic.code };
}

describe("01 section 15", () => {
  const values: [string, unknown][] = [
    ["7 / 2", 3.5], ["-7 % 3", -1], ["2 ** 10", 1024], ["2 ** -1", 0.5], ["0.1 + 0.2", 0.30000000000000004],
    ["`${1e21} ${0.000001} ${1e-7}`", "1e+21 0.000001 1e-7"], ['u.name || "anonymous"', "anonymous"],
    ['u.name ?? "anonymous"', ""], ["!u.tags", true], ["[] && 1", []], ["1 == true", false],
    ["[1, [2]] == [1, [2]]", true], ['"a" < "b" && "ab" > "a"', true], ['"" in "abc"', true],
    ['"x" in {x: null}', true], ["u?.missing", null], ["null?.a.b.c", null], ['"héllo"[1]', "é"],
    ["[1, 2]?.[2]", null], ["{ const [a, ...r] = [1, 2, 3]; {a, r} }", { a: 1, r: [2, 3] }],
    ["{ const {a = 5, b} = {a: null, b: 2}; a + b }", 7], ["{ const f = (x, n = 2) => x * n; f(3) + f(3, null) }", 12],
    ['if (0) { "a" } else if ("") { "b" } else { "c" }', "c"], ["(if (true) { 1 } else { 2 }) + 3", 4],
    ['match 2 { 1 | 2 => "low", _ => "high" }', "low"], ['{...{a: 1, b: 2}, b: 3, [("c")]: 4}', { a: 1, b: 3, c: 4 }],
    ['`${[1, "a"]}-${null}-${{b: 1, a: true}}`', '[1,"a"]--{"a":true,"b":1}'],
    ["try { 1 / 0 } catch (e) { e.code }", "divide-by-zero"],
    ['try { fail("no", {n: 1}) } catch (e) { e }', { code: "failed", message: "no", path: null, data: { n: 1 } }],
    ['try { [][0] } catch { "none" }', "none"], ["{if: 1}.if", 1],
  ];
  for (const [src, want] of values) {
    test(src, () => {
      const o = outcome(src);
      assert.ok(o.code === undefined, `${src}: ${o.code}`);
      assert.ok(plainEqual(o.value, want), `${src}: ${JSON.stringify(o.value)}`);
    });
  }
  const codes: [string, string][] = [
    ["9007199254740991 + 1", "number-range"], ["1e308 * 10", "number-range"], ["1 / 0", "divide-by-zero"],
    ['"n=" + 5', "wrong-kind"], ["1 in null", "null-operand"], ["u.missing", "missing-field"],
    ["(null?.a).b", "null-operand"], ["5?.a", "wrong-kind"], ["[1, 2][2]", "index-range"],
    ["{ const {a} = {}; a }", "missing-field"], ['match "c" { "a" => 1, "b" => 2 }', "no-match"], ["[...null]", "null-operand"],
  ];
  for (const [src, want] of codes) test(src, () => assert.equal(outcome(src).code, want));
});

describe("02 section 7: worked counts", () => {
  const usage = (src: string, v: Record<string, unknown> = {}) => execute(env.compile(parse(src), { check: false }), v).usage;
  const rows: [string, Record<string, unknown>, number, number][] = [
    ["1 + 2 * 3", {}, 2, 0],
    ["const xs = [1, 2, 3]; [...xs, 4]", {}, 3, 9],
    ["const m = {a: 1, b: 2}; {...m, b: 3, c: 4}", {}, 2, 12],
    ["`n=${n}, xs=${xs}`", { n: 5, xs: [1, 2] }, 3, 1],
    ["`${m}`", { m: { b: 1, a: 2 } }, 5, 1],
    ["[s + t, s == u, s < t]", { s: "a".repeat(100), t: "b".repeat(40), u: "a".repeat(70) + "c" }, 6, 7],
    ['if (d > 10) { "big" } else if (d > 0) { "small" } else { "neg" }', { d: 5 }, 2, 0],
    ["xs.map(x => x * 2)", { xs: [1, 2, 3] }, 11, 4],
    ["const sq = x => x * x; sq(3) + sq(4)", {}, 6, 0],
    ["const {a, b = 10, ...rest} = m; [a, b, rest]", { m: { a: 1, c: 2, d: 3 } }, 4, 9],
    ['[[1, 2, 3] == [1, 2, 4], "b" in ["a", "b", "c"]]', {}, 8, 15],
    ["{a: 1, b: 2} == {b: 2, a: 1}", {}, 4, 10],
    ["try { 1 / d } catch (err) { err.code }", { d: 0 }, 3, 10],
    ['try { fail("no stock", {sku: s}) } catch (err) { err.data.sku }', { s: "a" }, 3, 12],
    ["try { xs[5] } catch { -1 }", { xs: [1, 2] }, 2, 0],
    ["[1, 2] in [[1, 3], [1, 2]]", {}, 10, 12],
    ["xs.map(1)", { xs: [1, 2] }, 0, 0],
    ["try { xs.map(1) } catch (err) { err.code }", { xs: [1, 2] }, 2, 10],
  ];
  for (const [src, v, steps, units] of rows) {
    test(src, () => assert.deepEqual(usage(src, v), { steps, allocated: units }));
  }
});

describe("02 8.4: an empty pair is entered, with the depth test", () => {
  const at = (depth: number, src: string) => {
    const o = execute(createEnv({ limits: { depth } }).compile(parse(src), { check: false }));
    return o.ok ? "ok" : o.diagnostic.code;
  };
  test("== on empty lists and maps, at the limit and one past it", () => {
    assert.equal(at(1, "[] == []"), "ok");
    assert.equal(at(1, "[[]] == [[]]"), "depth-exceeded");
    assert.equal(at(2, "[[]] == [[]]"), "ok");
    assert.equal(at(1, "{a: {}} == {a: {}}"), "depth-exceeded");
    assert.equal(at(1, "const f = x => x == []; f([])"), "depth-exceeded");
    assert.equal(at(2, "const f = x => x == []; f([])"), "ok");
  });
  test("a rendering walk enters an empty list or map", () => {
    assert.equal(at(1, "`${[[]]}`"), "depth-exceeded");
    assert.equal(at(2, "`${[[]]}`"), "ok");
    assert.equal(at(1, "string([{}])"), "depth-exceeded");
  });
  test("the empty pair still charges nothing", () => {
    assert.deepEqual(execute(createEnv({}).compile(parse("[] == []"), { check: false })).usage, { steps: 0, allocated: 2 });
  });
});

describe("02 5.3: == on maps visits the left map in key order when a value is a list or a map", () => {
  test("the first key's lists pass the depth limit before the last key's long string passes the step limit", () => {
    let deep: unknown = 1;
    for (let i = 0; i < 10; i++) deep = [deep];
    const long = "x".repeat(64 * 50);
    const m = { a: deep, z: long };
    const n = { a: deep, z: long };
    const env = createEnv({ varRecord: "{m: any, n: any}", limits: { depth: 5, steps: 20 } });
    const o = execute(env.compile(parse("m == n"), { check: false }), { m, n });
    assert.ok(!o.ok);
    assert.equal(o.diagnostic.code, "depth-exceeded");
  });
});
