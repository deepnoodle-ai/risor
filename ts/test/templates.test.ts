// Package F2: the binding surface that the suite cannot reach (f02 2, 3):
// parsing a template, its two modes, "is single", a part as its own program,
// analysis of a program and of a template, arguments as programs, and long
// flat programs analyzed without the host's stack.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, parseProgramText, parseTemplate, RisorError } from "../src/host/api.ts";

const env = createEnv({ vars: "{pr: {number: int}, name: string, xs: int[]}" });
const vars = { pr: { number: 42 }, name: "Ann", xs: [1, 2, 3] };

describe("f02 2: templates", () => {
  test("segments and is single", () => {
    const t = parseTemplate("Review of #${{ pr.number }}!");
    assert.equal(t.single, false);
    assert.deepEqual(t.segments.map((s) => s.kind), ["text", "part", "text"]);
    assert.equal(parseTemplate("${{ pr.number }}").single, true);
    assert.equal(parseTemplate(" ${{ pr.number }}").single, false);
    assert.equal(parseTemplate("").segments.length, 0);
  });
  test("the two modes", () => {
    const t = parseTemplate("${{ pr.number }}");
    assert.equal(env.compile(t.program("value")).run(vars), 42);
    assert.equal(env.compile(t.program("string")).run(vars), "42");
    const u = parseTemplate("Hello ${{ name }} and $${{ x }}");
    assert.equal(env.compile(u.program("value")).run(vars), "Hello Ann and ${{ x }}");
  });
  test("a check of a template's program: type and expected result", () => {
    assert.deepEqual(env.check(parseTemplate("${{ xs }}").program("value")).ok, true);
    const c = env.check(parseTemplate("${{ xs }}").program("value"));
    assert.ok(c.ok && c.type === "int[]");
    const d = env.check(parseTemplate("n: ${{ pr.number }}").program("string"), "int");
    assert.ok(!d.ok && d.diagnostics[0].code === "result-mismatch");
  });
  test("a part as its own program, with spans in the whole text", () => {
    const t = parseTemplate("a ${{ pr.number + 1 }} b ${{ name }}");
    assert.equal(env.compile(t.part(0)).run(vars), 43);
    assert.equal(env.compile(t.part(1)).run(vars), "Ann");
    assert.deepEqual(t.part(1).program.span, { start: 29, end: 33 });
  });
  test("a parse error is a RisorError of phase parse", () => {
    assert.throws(() => parseTemplate("${{ }}"), (e: unknown) => e instanceof RisorError && e.phase === "parse");
  });
});

describe("f02 3: analysis", () => {
  test("a program", () => {
    const r = parseProgramText("geo.distance(a, b) > limit && input.event[\"id\"]").analyze();
    assert.deepEqual(r.names, ["a", "b", "geo", "input", "limit"]);
    assert.deepEqual(r.refs.map((x) => [x.name, x.path]), [["geo", []], ["a", []], ["b", []], ["limit", []], ["input", ["event", "id"]]]);
    assert.deepEqual(r.calls.map((c) => [c.name, c.namespace, c.lambda]), [["geo.distance", "geo", false]]);
    assert.equal(r.literal, false);
    assert.equal(r.segments, null);
  });
  test("each argument of a call is a parsed program of its own; a lambda is not", () => {
    const r = parseProgramText("rate(order.total * 2, x => x)").analyze();
    const [arg, lam] = r.calls[0].args;
    assert.equal(lam.program, null);
    const e2 = createEnv({ vars: "{order: {total: float}}" });
    assert.equal(e2.compile(arg.program!).run({ order: { total: 1.5 } }), 3);
  });
  test("a template, in each mode", () => {
    const t = parseTemplate("${{ 5 }}");
    assert.deepEqual(t.analyze("value").literal, { value: 5 });
    assert.equal(t.analyze("string").literal, false);
    assert.equal(t.analyze("string").segments!.length, 1);
    assert.deepEqual(parseTemplate("x ${{ a.b }}").analyze("string").refs.map((r) => r.path), [["b"]]);
  });
  test("long flat programs and many parts need no host stack (3.1 item 5)", () => {
    const n = 100_000;
    const sum = parseProgramText(Array.from({ length: n }, (_, i) => `v${i % 10}`).join(" + ")).analyze();
    assert.equal(sum.names.length, 10);
    assert.equal(sum.refs.length, n);
    assert.equal(parseProgramText("m" + ".a".repeat(n)).analyze().refs[0].path.length, n);
    assert.equal(parseProgramText("c ? 1 : ".repeat(n) + "2").analyze().refs.length, n);
    assert.equal(parseProgramText("if (c) { 1 } " + "else if (c) { 2 } ".repeat(n) + "else { 3 }").analyze().names[0], "c");
    const many = parseTemplate("${{ x }} ".repeat(10_000));
    assert.equal(many.analyze("string").refs.length, 10_000);
  });
});
