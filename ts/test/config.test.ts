// Unit tests of package F10 for the binding (f10 5, 6.6): the load and its
// steps, and the decode into plain data, which the case files cannot state.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, decode, LoadError, toJSON } from "../src/host/api.ts";

const SCHEMA = { type: "object", properties: { port: { type: "integer" }, hosts: { type: "array", items: { type: "string" } } }, required: ["port"] };
const env = createEnv({ vars: "{s: string, a: any}", groups: ["json"] });

describe("the steps of a load (f10 5.2)", () => {
  test("a value, decoded into plain data", () => {
    const r = env.tryLoad("{port: 80, hosts: [`a`]}", { schema: SCHEMA });
    assert.ok(r.ok);
    assert.deepEqual(r.value, { port: 80, hosts: ["a"] });
    assert.ok(r.usage.steps >= 0);
  });
  test("step 0: a bad schema; a load maps a schema with closed only", () => {
    const r = env.tryLoad("1", { schema: { type: "nope" } });
    assert.ok(!r.ok && r.step === 0 && r.diagnostics[0].code === "bad-type");
    const c = env.tryLoad("{port: 1, prot: 2}", { schema: SCHEMA });
    assert.ok(!c.ok && c.step === 2 && c.diagnostics[0].code === "result-mismatch");
  });
  test("steps 1 and 3", () => {
    const p = env.tryLoad("{port: ", "{port: int}");
    assert.ok(!p.ok && p.step === 1);
    const r = env.tryLoad("{port: int(s)}", "{port: int}", { vars: { s: "x", a: null } });
    assert.ok(!r.ok && r.step === 3 && r.diagnostics[0].span !== undefined);
  });
  test("step 4: every failure up to the maximum, no span, the run's usage", () => {
    const r = env.tryLoad("json.parse(s)", { schema: SCHEMA }, { vars: { s: '{"port": "80", "x": 1, "y": 2}', a: null }, max: 10 });
    assert.ok(!r.ok);
    assert.equal(r.step, 4);
    assert.deepEqual(r.diagnostics.map((d) => [d.path, d.message, d.span]), [
      [["port"], 'expected int, found "80"', undefined],
      [["x"], 'the type does not allow key "x"', undefined],
      [["y"], 'the type does not allow key "y"', undefined],
    ]);
    // Only step 3 charges: the usage is the run's, as a run of the same program gives it.
    assert.deepEqual(r.usage, env.compile("json.parse(s)").measure({ s: '{"port": "80", "x": 1, "y": 2}', a: null }).usage);
    const one = env.tryLoad("json.parse(s)", { schema: SCHEMA }, { vars: { s: '{"port": "80", "x": 1}', a: null }, max: 0 });
    assert.ok(!one.ok && one.diagnostics.length === 1);
  });
  test("load throws LoadError with the step", () => {
    assert.throws(() => env.load("a", "{port: int}", { vars: { s: "", a: [1] } }), (e) => e instanceof LoadError && e.step === 4);
    assert.deepEqual(env.load("{port: 1}", "{port: int}"), { port: 1 });
  });
  test("a load without decode gives the value as the run gave it", () => {
    const host = { port: 1 };
    assert.equal(env.load("a", "{port: int}", { vars: { s: "", a: host }, decode: false }), host);
    assert.notEqual(env.load("a", "{port: int}", { vars: { s: "", a: host } }), host);
  });
});

describe("the decode into plain data (f10 6.6)", () => {
  test("__proto__ and constructor are own properties, and no prototype changes", () => {
    const r = decode(new Map<string, unknown>([["__proto__", 1], ["constructor", 2]]));
    assert.ok(r.ok);
    const v = r.value as Record<string, unknown>;
    assert.deepEqual(Object.keys(v), ["__proto__", "constructor"]);
    assert.equal(Object.getPrototypeOf(v), Object.prototype);
    assert.equal(Object.getOwnPropertyDescriptor(v, "__proto__")?.value, 1);
  });
  test("-0 is 0; nesting past 200 is depth-exceeded", () => {
    const r = decode([-0]);
    assert.ok(r.ok && Object.is((r.value as number[])[0], 0));
    let deep: unknown = 1;
    for (let i = 0; i < 201; i++) deep = [deep];
    const d = decode(deep);
    assert.ok(!d.ok && d.diagnostic.code === "depth-exceeded" && d.diagnostic.path === undefined);
  });
});

describe("the JSON text of a result (f10 4.1)", () => {
  test("the examples of the table", () => {
    const run = (src: string) => toJSON(createEnv().compile(src).run());
    assert.equal(run("{max_body: 16 * 1024 * 1024, listen: \":8080\", tls: null}"), '{"listen":":8080","max_body":16777216,"tls":null}');
    assert.equal(run('{"a_b": 1, ab: 2, "a-b": 3, Z: 4, _: 5, a: 6, "1": 7, "": 8}'), '{"":8,"1":7,"Z":4,"_":5,"a":6,"a-b":3,"a_b":1,"ab":2}');
    assert.equal(run("[1e21, 0.1 * 3, 1e20, 123.0, -0, 1e-7]"), "[1e+21,0.30000000000000004,100000000000000000000,123,0,1e-7]");
  });
});
