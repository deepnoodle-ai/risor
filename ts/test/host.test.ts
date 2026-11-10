// Unit tests of the host API, for what no data case can raise (brief S2
// item 9, ruling W3): the deadline, cancellation, a host crash, opaque values,
// host numbers that cannot enter, ill-formed host text.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, HostError, RisorError, toJSON, truthy } from "../src/host/api.ts";

class Time {
  readonly ms: number;
  constructor(ms: number) {
    this.ms = ms;
  }
}

const opaque = {
  Time: {
    test: (v: unknown) => v instanceof Time,
    methods: {
      add: { type: "(ms: int) => Time", call: (_h: unknown, t: unknown, ms: unknown) => new Time((t as Time).ms + (ms as number)) },
    },
  },
};

function code(f: () => unknown): string {
  try {
    f();
  } catch (e) {
    assert.ok(e instanceof RisorError, String(e));
    return e.diagnostics[0].code;
  }
  assert.fail("expected a failure");
}

describe("limits a data case cannot state", () => {
  test("deadline-exceeded", () => {
    const env = createEnv({
      vars: "{xs: any}",
      limits: { timeoutMs: 1 },
      functions: {
        wait: {
          type: "() => int",
          call: () => {
            const end = Date.now() + 5;
            while (Date.now() <= end) {
              /* wait */
            }
            return 1;
          },
        },
      },
    });
    assert.equal(code(() => env.compile("wait() + 1", { check: false }).run()), "deadline-exceeded");
  });
  test("deadline-exceeded at the step interval of 1,024", () => {
    const env = createEnv({ vars: "{xs: any}", limits: { timeoutMs: 0, steps: null } });
    const xs = Array.from({ length: 5000 }, (_, i) => i);
    const start = Date.now();
    while (Date.now() === start) {
      /* let the clock move */
    }
    assert.equal(code(() => env.compile("xs.map(x => x + 1)", { check: false }).run({ xs })), "deadline-exceeded");
  });
  test("cancelled: a signal aborted before the run, and during a host function", () => {
    const ac = new AbortController();
    ac.abort();
    const env = createEnv({});
    assert.equal(code(() => env.compile("1 + 1", { check: false }).run({}, { signal: ac.signal })), "cancelled");
    const ac2 = new AbortController();
    const env2 = createEnv({ functions: { stop: { type: "() => int", call: () => (ac2.abort(), 1) } } });
    assert.equal(code(() => env2.compile("stop() + 1", { check: false }).run({}, { signal: ac2.signal })), "cancelled");
  });
  test("a limit failure is not caught", () => {
    const env = createEnv({ limits: { steps: 1 } });
    assert.equal(code(() => env.compile("try { 1 + 2 + 3 } catch { 0 }", { check: false }).run()), "steps-exceeded");
  });
});

describe("host functions", () => {
  const env = createEnv({
    functions: {
      boom: { type: "() => int", call: () => { throw "not an error"; } },
      oops: { type: "() => int", call: () => { throw new HostError("it broke"); } },
      big: { type: "() => int", call: () => 2n ** 60n },
      nan: { type: "() => float", call: () => NaN },
      negz: { type: "() => float", call: () => -0 },
      chargy: { type: "() => int", call: (h) => (h.charge(3), h.allocate(2), 7) },
      bad: { type: "() => string", call: () => "a\ud800b" },
    },
  });
  test("a thrown value that is not an error is function-panic, and catch lets it pass", () => {
    assert.equal(code(() => env.compile("try { boom() } catch { 0 }", { check: false }).run()), "function-panic");
  });
  test("an error is function-error, caught, with the host's text", () => {
    assert.equal(env.compile("try { oops() } catch (e) { e.message }", { check: false }).run(), "it broke");
  });
  test("a host integer outside the whole range cannot enter (ruling T7)", () => {
    assert.equal(code(() => env.compile("big()", { check: false }).run()), "number-range");
    assert.equal(env.compile('try { big() } catch (e) { e.code }', { check: false }).run(), "number-range");
    assert.equal(code(() => env.compile("nan()", { check: false }).run()), "number-range");
  });
  test("a host -0 reads as zero", () => {
    assert.ok(Object.is(env.compile("negz()", { check: false }).run(), 0));
  });
  test("charges through the call handle", () => {
    assert.deepEqual(env.compile("chargy() + 1", { check: false }).measure().usage, { steps: 5, allocated: 2 });
  });
  test("ill-formed host text reads as U+FFFD (03 3.4.5)", () => {
    assert.equal(env.compile("bad()", { check: false }).run(), "a�b");
    assert.equal(env.compile('bad() == "a\\u{FFFD}b"', { check: false }).run(), true);
  });
  test("arguments are not tested against the signature (ruling AC7)", () => {
    const e2 = createEnv({ functions: { id: { type: "(x: int) => any", call: (_h, x) => x } } });
    assert.equal(e2.compile('id("text")', { check: false }).run(), "text");
  });
});

describe("opaque values (03 3.4.6)", () => {
  const env = createEnv({ vars: "{t: Time, u: Time}", opaque });
  const t = new Time(1);
  test("truthy as a condition", () => {
    assert.equal(env.compile('t ? "yes" : "no"', { check: false }).run({ t, u: t }), "yes");
    assert.equal(env.compile("!t", { check: false }).run({ t, u: t }), false);
    assert.equal(truthy(t), true);
  });
  test("== by identity", () => {
    assert.equal(env.compile("t == u", { check: false }).run({ t, u: t }), true);
    assert.equal(env.compile("t == u", { check: false }).run({ t, u: new Time(1) }), false);
  });
  test("in a list and a map, and returned as itself", () => {
    const v = env.compile("[t, {a: t}]", { check: false }).run({ t, u: t }) as unknown[];
    assert.equal(v[0], t);
    assert.equal(env.compile("t in [u]", { check: false }).run({ t, u: t }), true);
  });
  test("a declared method", () => {
    const r = env.compile("t.add(5)", { check: false }).run({ t, u: t }) as Time;
    assert.equal(r.ms, 6);
  });
  test("the shape test on an opaque receiver", () => {
    assert.equal(code(() => env.compile("t.nope()", { check: false }).run({ t, u: t })), "wrong-kind");
    assert.equal(code(() => env.compile("t.add(x => x)", { check: false }).run({ t, u: t })), "wrong-kind");
  });
  test("cannot be rendered or written as JSON", () => {
    assert.equal(code(() => env.compile("`${t}`", { check: false }).run({ t, u: t })), "wrong-kind");
    assert.equal(code(() => toJSON([t], env.classifier)), "wrong-kind");
    assert.equal(env.compile("type(t)", { check: false }).run({ t, u: t }), "opaque");
  });
  test("a class instance that no opaque type declares is opaque", () => {
    const e2 = createEnv({ vars: "{x: any}" });
    assert.equal(e2.compile("type(x)", { check: false }).run({ x: new Time(1) }), "opaque");
  });
});

describe("host values read in place", () => {
  const env = createEnv({ vars: "{m: any}" });
  test("a property holding undefined is absent; the prototype chain is not read", () => {
    assert.equal(code(() => env.compile("m.a", { check: false }).run({ m: { a: undefined } })), "missing-field");
    assert.equal(code(() => env.compile("m.constructor", { check: false }).run({ m: {} })), "missing-field");
    assert.equal(env.compile("len(m)", { check: false }).run({ m: { a: 1, b: undefined } }), 1);
    assert.equal(env.compile("!m", { check: false }).run({ m: { b: undefined } }), true);
  });
  test("a Map with string keys is a map", () => {
    assert.equal(env.compile("m.a + 1", { check: false }).run({ m: new Map([["a", 1]]) }), 2);
  });
  test("a variable not supplied is missing-variable, and is asked for once", () => {
    let asked = 0;
    const e = createEnv({ vars: "{x: int}" });
    assert.equal(code(() => e.compile("x", { check: false }).run({})), "missing-variable");
    e.compile("x + x", { check: false }).run((n: string) => (asked++, n === "x" ? 1 : undefined));
    assert.equal(asked, 1);
  });
  test("toJSON", () => {
    assert.equal(toJSON(new Map<string, unknown>([["b", [1, "x"]], ["a", null]])), '{"a":null,"b":[1,"x"]}');
  });
});

describe("ruling AM4: bad-limit", () => {
  const declareCode = (limits: Record<string, unknown>) => {
    try {
      createEnv({ limits: limits as never });
      return "ok";
    } catch (e) {
      assert.ok(e instanceof RisorError);
      assert.equal(e.phase, "declare");
      return e.diagnostics[0].code;
    }
  };
  test("a limit that is not a whole number", () => {
    assert.equal(declareCode({ steps: 1.5 }), "bad-limit");
    assert.equal(declareCode({ allocation: -1 }), "bad-limit");
    assert.equal(declareCode({ steps: Infinity }), "bad-limit");
    assert.equal(declareCode({ steps: "10" }), "bad-limit");
    assert.equal(declareCode({ timeoutMs: -5 }), "bad-limit");
  });
  test("a depth limit outside 1 to 1,000", () => {
    assert.equal(declareCode({ depth: 0 }), "bad-limit");
    assert.equal(declareCode({ depth: 1001 }), "bad-limit");
    assert.equal(declareCode({ depth: 2.5 }), "bad-limit");
  });
  test("limits in range, and limits turned off, are accepted", () => {
    assert.equal(declareCode({ steps: 0, allocation: 0, depth: 1 }), "ok");
    assert.equal(declareCode({ steps: null, allocation: null, depth: 1000, timeoutMs: null }), "ok");
  });
});
