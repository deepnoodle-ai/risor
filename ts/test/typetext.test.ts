import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { arity, parseMethod, parseReceiver, parseSignature, parseType } from "../src/types/typetext.ts";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { SUITE } from "./suite-report.ts";

describe("05 3.1 and 3.2: type text", () => {
  const good = [
    "null", "bool", "int", "float", "string", "any", "never", '"fail" | "pass"', "'a'", "int[]", "int[][]",
    "(int | string)[]", "map<int>", "map<map<int>>", "{}", "{a: int}", "{a?: int, b: string | null}",
    "{a: int, ...}", "{a: int, ...int}", "{...}", "{...any,}", '{"a-b": int, if: bool,}', "Time", "Time | null",
    "{action: string, ...}", "{note?: string, owner: string | null}", "int /* c */ | float",
  ];
  for (const t of good) test(`accepts ${t}`, () => assert.ok(parseType(t).ok, t));
  const bad = [
    "", "number", "boolean", "undefined", "void", "nil", "object", "Array<int>", "int?", "condition",
    "unknown", "T", "map", "map<int", "{a: int", "{a}", "{a int}", "int |", "1", "`a`", "{...int, a: int}",
    "int[", "(int", "int string", "{a: int,, b: int}", "map<>", "$",
  ];
  for (const t of bad) test(`refuses ${JSON.stringify(t)}`, () => assert.equal(parseType(t).ok, false, t));
  test("the tree", () => {
    const r = parseType("{a?: int[], ...string} | null");
    assert.ok(r.ok);
    assert.deepEqual(r.node, {
      t: "union",
      alts: [
        { t: "record", fields: [{ key: "a", optional: true, type: { t: "list", elem: { t: "word", word: "int" } } }], rest: { t: "word", word: "string" } },
        { t: "word", word: "null" },
      ],
    });
  });
  test("bracket nesting past 64 (05 20.5)", () => {
    assert.ok(parseType("(".repeat(63) + "int" + ")".repeat(63)).ok);
    assert.equal(parseType("(".repeat(65) + "int" + ")".repeat(65)).ok, false);
  });
});

describe("signatures and methods", () => {
  test("host signatures", () => {
    const s = parseSignature("(s: string, n?: int) => string !");
    assert.ok(s.ok);
    assert.deepEqual(arity(s.node), { min: 1, max: 2 });
    assert.equal(s.node.canFail, true);
    const r = parseSignature("(...xs: T[]) => T where T: ordered");
    assert.ok(r.ok);
    assert.deepEqual(arity(r.node), { min: 0, max: Infinity });
    assert.deepEqual(r.node.bounds, [{ name: "T", bound: "ordered" }]);
    assert.ok(parseSignature("() => int").ok);
    assert.ok(parseSignature("(x: (int | string)) => int").ok);
  });
  test("function parameters", () => {
    const s = parseSignature("(f: (x: T, i: int) => unknown, g: () => U) => T[]");
    assert.ok(s.ok);
    assert.equal(s.node.params[0].type.t, "fn");
    assert.equal(s.node.params[1].type.t, "fn");
  });
  test("refused signatures", () => {
    for (const t of ["(x) => int", "(x: int,) => int", "x: int => int", "(x: int)", "(x: int) => unknown", "() => int where T", "() => int where T: string", "(x: int) => int !!"]) {
      assert.equal(parseSignature(t).ok, false, t);
    }
  });
  test("methods", () => {
    const m = parseMethod("Time.format: (layout: string) => string");
    assert.ok(m.ok);
    assert.equal(m.node.name, "format");
    assert.ok(parseReceiver("{key: string, value: T}[]").ok);
  });
  test("every signature and receiver of library.json parses", () => {
    const lib = JSON.parse(readFileSync(join(SUITE, "library.json"), "utf8")) as { name: string; signature: string; receiver?: string | null }[];
    for (const e of lib) {
      assert.ok(parseSignature(e.signature).ok, `${e.name}: ${e.signature}`);
      if (e.receiver) assert.ok(parseReceiver(e.receiver).ok, `${e.name}: ${e.receiver}`);
    }
  });
});
