// Adversarial regression tests for limits, the host boundary, nested runs with
// remaining(), host data conformance, and editor tools (issue #704).

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import {
  conform,
  conformAll,
  createEnv,
  decode,
  format,
  formatTemplate,
  HostError,
  jsonText,
  parse,
  parseTemplate,
  renderJSON,
  RisorError,
  typeInt,
  listType,
  type HostFunction,
} from "../src/index.ts";
import { execute } from "../src/host/api.ts";
import { tokens } from "../src/tools/tokens.ts";

function errorCode(fn: () => unknown): string {
  try {
    fn();
  } catch (e) {
    if (e instanceof RisorError && e.diagnostics.length > 0) return e.diagnostics[0].code;
    throw e;
  }
  assert.fail("expected RisorError");
}

describe("adversarial: limits and OOM-before-charge", () => {
  const env = createEnv({
    groups: ["time", "json", "base64", "math"],
    varRecord: "{ s: string, xs: any[], m: map<any>, x: any }",
  });

  test("exponential string/list blowups stop at step or allocation limit", () => {
    const vars = { s: "a", xs: [], m: {}, x: null };
    assert.equal(errorCode(() => env.compile(parse(`"a".repeat(1000000000000000)`)).run(vars)), "steps-exceeded");
    assert.equal(errorCode(() => env.compile(parse(`range(1000000000000000)`)).run(vars)), "steps-exceeded");
    assert.equal(errorCode(() => env.compile(parse(`2 ** 10000`)).run(vars)), "number-range");

    const tiny = env.with({ limits: { steps: 10_000_000, allocation: 5 } });
    // String concatenation blowup: 2^20 chars.
    const dbl = Array.from({ length: 20 }, (_, i) =>
      i === 0 ? `const a0 = "0123456789abcdef";` : `const a${i} = a${i - 1} + a${i - 1};`,
    ).join("\n") + "\na19";
    assert.equal(errorCode(() => tiny.compile(parse(dbl)).run(vars)), "allocation-exceeded");
  });

  test("OOM-before-charge: json.parse, string.split, replaceAll, slice, padStart, Rendering", () => {
    const tiny = env.with({ limits: { steps: 10_000_000, allocation: 5 } });
    const bigJson = "[" + Array(50_000).fill("1").join(",") + "]";
    assert.equal(
      errorCode(() => tiny.compile(parse("json.parse(s)")).run({ s: bigJson, xs: [], m: {}, x: null })),
      "allocation-exceeded",
    );

    const bigCsv = "a,".repeat(100_000);
    assert.equal(
      errorCode(() => tiny.compile(parse(`s.split(",")`)).run({ s: bigCsv, xs: [], m: {}, x: null })),
      "allocation-exceeded",
    );
    assert.equal(
      errorCode(() => tiny.compile(parse(`s.replaceAll(",", "XXXXXXXXXX")`)).run({ s: bigCsv, xs: [], m: {}, x: null })),
      "allocation-exceeded",
    );
    assert.equal(
      errorCode(() => tiny.compile(parse(`s.slice(0, 50000)`)).run({ s: bigCsv, xs: [], m: {}, x: null })),
      "allocation-exceeded",
    );
    assert.equal(
      errorCode(() => tiny.compile(parse(`"x".padStart(100000, s)`)).run({ s: "a".repeat(100_000), xs: [], m: {}, x: null })),
      "allocation-exceeded",
    );

    const manyStrings = Array(10_000).fill("abcdef0123456789");
    assert.equal(
      errorCode(() => tiny.compile(parse("`${xs}`")).run({ s: "", xs: manyStrings, m: {}, x: null })),
      "allocation-exceeded",
    );
    assert.equal(
      errorCode(() => tiny.compile(parse("json.stringify(xs)")).run({ s: "", xs: manyStrings, m: {}, x: null })),
      "allocation-exceeded",
    );
    // Rendering past the 64 KB buffer cap still produces exact text when the budget allows.
    const bigRender = env.with({ limits: { steps: 1_000_000, allocation: 100_000 } });
    const rendered = bigRender.compile(parse("`${xs}`")).run({ s: "", xs: manyStrings, m: {}, x: null }) as string;
    assert.ok(rendered.length > 150_000);
    assert.equal(rendered.startsWith(`["abcdef0123456789"`), true);
  });

  test("time.duration and time.parse stay linear on pathological inputs", () => {
    const dur = "0." + "0".repeat(10_000) + "1s" + "1s".repeat(10_000);
    const t0 = performance.now();
    const got = env.compile(parse("time.duration(s)")).run({ s: dur, xs: [], m: {}, x: null });
    assert.equal(got, 10_000);
    assert.ok(performance.now() - t0 < 2_000, "time.duration took too long");

    // Huge fraction uses O(1) scratch memory and charges 0 allocation units.
    const zeroAlloc = env.with({ limits: { steps: 1_000_000, allocation: 0 } });
    const hugeFrac = "1." + "0".repeat(200_000) + "5s";
    const measured = zeroAlloc.compile(parse("time.duration(s)")).measure({ s: hugeFrac, xs: [], m: {}, x: null });
    assert.equal(measured.value, 1);
    assert.equal(measured.usage.allocated, 0);

    // Large carry sums propagate without dropping high digits.
    const prog = env.compile(parse("time.duration(s)"));
    const evalDur = (s: string) => prog.run({ s, xs: [], m: {}, x: null }) as number;
    assert.equal(evalDur("1d".repeat(200_000)), 17_280_000_000);
    assert.equal(evalDur("100000000d".repeat(2_000)), 1.728e16);
    assert.equal(errorCode(() => evalDur("1" + "0".repeat(308) + "s" + "1" + "0".repeat(308) + "s")), "number-range");

    // Exact midpoint tiebreaking around 1 + 2^-53 (midpoint between 1 and 1 + 2^-52).
    const nextUp = 1 + Number.EPSILON;
    const midExact = "1.00000000000000011102230246251565404236316680908203125";
    const belowMid = "1.00000000000000011102230246251565404236316680908203124" + "9".repeat(1120 - 53) + "s";
    const tiny = "0." + "0".repeat(1120) + "1s";
    assert.equal(evalDur(`${midExact}s0.${"0".repeat(200_000)}1s`), nextUp);
    assert.equal(evalDur(belowMid + tiny.repeat(9)), 1);
    assert.equal(evalDur(belowMid + tiny.repeat(10)), 1);
    assert.equal(evalDur(belowMid + tiny.repeat(11)), nextUp);
    const ninesWin = `0.${"0".repeat(1120)}${"9".repeat(1024)}s`;
    const tiny2 = `0.${"0".repeat(2144)}1s`;
    assert.equal(evalDur(belowMid + ninesWin + tiny2.repeat(10)), 1);
    assert.equal(evalDur(belowMid + ninesWin + tiny2.repeat(11)), nextUp);

    const ts = "2020-01-01T00:00:00." + "1".repeat(50_000) + "Z";
    const t1 = performance.now();
    const parsed = env.compile(parse("time.parse(s)")).run({ s: ts, xs: [], m: {}, x: null }) as number;
    assert.ok(Math.abs(parsed - 1577836800.1111112) < 1e-6);
    assert.ok(performance.now() - t1 < 2_000, "time.parse took too long");

    // Negative epoch with long fractional seconds exercises borrow path.
    const preEpoch = "1969-12-31T23:59:59." + "5" + "0".repeat(20_000) + "Z";
    assert.equal(env.compile(parse("time.parse(s)")).run({ s: preEpoch, xs: [], m: {}, x: null }), -0.5);
  });

  test("source-size limit enforced across all entry points", () => {
    const over = "1" + " ".repeat(1_048_576);
    assert.equal(errorCode(() => parse(over)), "source-too-large");
    assert.equal(errorCode(() => parseTemplate(over)), "source-too-large");
    assert.equal(errorCode(() => format(over)), "source-too-large");
    assert.equal(errorCode(() => formatTemplate(over)), "source-too-large");
    assert.deepEqual(env.complete(over, { line: 1, column: 2 }).items, []);
    assert.equal(env.hover(over, { line: 1, column: 2 }), null);
  });

  test("parser nesting, type depth, and checker type-size DAG bounds", () => {
    assert.equal(errorCode(() => parse("(".repeat(500))), "nesting-exceeded");
    const dag = Array.from({ length: 25 }, (_, i) =>
      i === 0 ? "const a0 = {x: 1, y: 1};" : `const a${i} = {x: a${i - 1}, y: a${i - 1}};`,
    ).join("\n") + "\na24";
    assert.equal(errorCode(() => env.compile(parse(dag))), "check-limit");
    assert.equal(errorCode(() => createEnv({ varRecord: `{ x: ${"int" + "[]".repeat(65)} }` })), "bad-type");
  });

  test("deadline stops tight step loops promptly", () => {
    const dl = createEnv({ limits: { steps: null, allocation: null, deadlineMs: 25 } });
    const t0 = performance.now();
    const code = errorCode(() => dl.compile(parse("const xs = range(10000); xs.map(x => xs.map(y => x + y))")).run({}));
    assert.equal(code, "deadline-exceeded");
    assert.ok(performance.now() - t0 < 2_000, "deadline took too long");
  });

  test("depth limit across recursion and walk", () => {
    const rec = createEnv({ limits: { depth: 20 } });
    assert.equal(
      errorCode(() => rec.compile(parse("const f = (n: int): int => n <= 0 ? 0 : f(n - 1); f(100)")).run({})),
      "depth-exceeded",
    );
    assert.equal(
      errorCode(() => rec.compile(parse("walk(0, (n, d) => d < 50 ? [n + 1] : null)")).run({})),
      "depth-exceeded",
    );
  });
});

describe("adversarial: opaque values, prototype isolation, and host functions", () => {
  test("prototype properties cannot leak or pollute maps", () => {
    const env = createEnv({ groups: ["json"], varRecord: "{ m: map<any> }" });
    assert.equal(env.compile(parse(`m["__proto__"] ?? "safe"`)).run({ m: {} }), "safe");
    assert.equal(env.compile(parse(`m["constructor"] ?? "safe"`)).run({ m: {} }), "safe");
    assert.equal(env.compile(parse(`m["toString"] ?? "safe"`)).run({ m: {} }), "safe");
    assert.equal(env.compile(parse(`m["hasOwnProperty"] ?? "safe"`)).run({ m: {} }), "safe");

    const lit = env.compile(parse(`{"__proto__": {polluted: 1}, "constructor": 2}`)).run({ m: {} });
    const decoded = decode(lit) as Record<string, unknown>;
    assert.equal(Object.getPrototypeOf(decoded), Object.prototype);
    assert.equal(Object.prototype.hasOwnProperty.call(decoded, "__proto__"), true);
    assert.equal(({} as Record<string, unknown>).polluted, undefined);

    const parsed = decode(env.compile(parse(`json.parse("{\\"__proto__\\":{\\"polluted\\":1}}")`)).run({ m: {} })) as Record<string, unknown>;
    assert.equal(Object.getPrototypeOf(parsed), Object.prototype);
    assert.equal(Object.prototype.hasOwnProperty.call(parsed, "__proto__"), true);
    assert.equal(({} as Record<string, unknown>).polluted, undefined);
  });

  test("call.charge and call.allocate sanitize fractional, negative, NaN, and non-finite counts", () => {
    const f: HostFunction = {
      type: "() => int !",
      call(h) {
        h.charge(-10);
        h.charge(NaN);
        h.allocate(-5);
        h.allocate(NaN);
        h.charge(1.5); // rounds up to 2
        h.allocate(1.2); // rounds up to 2
        const rem = h.remaining();
        assert.equal(Number.isInteger(rem.steps), true);
        assert.equal(Number.isInteger(rem.allocation), true);
        const child = createEnv({ limits: rem });
        return child.compile(parse("1 + 1")).run({});
      },
    };
    const env = createEnv({ limits: { steps: 100, allocation: 100 }, functions: { f } });
    const res = env.compile(parse("f()")).measure({});
    assert.equal(res.value, 2);
    assert.deepEqual(res.usage, { steps: 3, allocated: 2 });

    // Non-finite charge when steps limit is off must not poison steps or deadline checks.
    const inf: HostFunction = {
      type: "() => int !",
      call(h) {
        h.charge(Infinity);
        h.allocate(Infinity);
        return 1;
      },
    };
    const unlimited = createEnv({ limits: { steps: null, allocation: null }, functions: { inf } });
    const out = unlimited.compile(parse("inf()")).measure({});
    assert.equal(out.value, 1);
    assert.equal(Number.isFinite(out.usage.steps), true);
    assert.equal(Number.isFinite(out.usage.allocated), true);

    // Non-finite charge when steps limit is finite still triggers steps-exceeded.
    const limited = createEnv({ limits: { steps: 100 }, functions: { inf } });
    assert.equal(errorCode(() => limited.compile(parse("inf()")).run({})), "steps-exceeded");
  });

  test("host panic is uncatchable and nested cause chain renders to depth 8", () => {
    const crash: HostFunction = {
      type: "() => int !",
      call() {
        throw "not-an-error";
      },
    };
    const e1 = createEnv({ functions: { crash } });
    assert.equal(errorCode(() => e1.compile(parse("try { crash() } catch { 0 }")).run({})), "function-panic");

    // 12 nested host calls wrapping RisorError: JSON rendering caps cause depth at 8.
    let inner = createEnv({
      functions: { leaf: { type: "() => int !", call() { throw new HostError("boom"); } } },
    });
    for (let i = 0; i < 12; i++) {
      const nextEnv = inner;
      const callExpr = parse(i === 0 ? "leaf()" : `c${i - 1}()`);
      inner = createEnv({
        functions: {
          [`c${i}`]: {
            type: "() => int !",
            call(h) {
              return nextEnv.with({ limits: h.remaining() }).compile(callExpr).run({});
            },
          },
        },
      });
    }
    try {
      inner.compile(parse("c11()")).run({});
      assert.fail("expected error");
    } catch (e) {
      assert.ok(e instanceof RisorError);
      let d = JSON.parse(renderJSON(e.diagnostics)) as Record<string, unknown>;
      let depth = 0;
      while (d.cause !== undefined) {
        depth++;
        d = d.cause as Record<string, unknown>;
      }
      assert.equal(depth, 8);
    }
  });
});

describe("adversarial: nested runs with remaining() and host conformance", () => {
  test("nested runs cannot amplify budget across catch or loops", () => {
    const childEnv = createEnv();
    const sub: HostFunction = {
      type: "() => int !",
      call(h) {
        const child = childEnv.with({ limits: h.remaining() });
        const o = execute(child.compile(parse("range(20).map(x => x + 1)")), {}, { signal: h.signal });
        h.charge(o.usage.steps);
        h.allocate(o.usage.allocated);
        if (!o.ok && o.diagnostic.code === "steps-exceeded") h.charge(Number.MAX_SAFE_INTEGER);
        if (!o.ok && o.diagnostic.code === "allocation-exceeded") h.allocate(Number.MAX_SAFE_INTEGER);
        if (!o.ok) throw new HostError(o.diagnostic.message);
        return 1;
      },
    };
    const env = createEnv({ limits: { steps: 100, allocation: 100 }, functions: { sub } });
    assert.equal(errorCode(() => env.compile(parse("try { range(10).map(_ => sub()) } catch { [] }")).run({})), "steps-exceeded");
  });

  test("cyclic host objects, huge lists, bad numbers, and lone surrogates", () => {
    const cyc: Record<string, unknown> = {};
    cyc.self = cyc;
    const recEnv = createEnv({ namedTypes: [{ name: "Node", type: "{ self: Node }" }], varRecord: "{ x: any }" });
    assert.equal(errorCode(() => recEnv.compile(parse("x as Node")).run({ x: cyc })), "depth-exceeded");
    const nodeType = recEnv.check(parse("x as Node")).type;
    assert.equal(conform(cyc, nodeType)?.code, "depth-exceeded");
    assert.equal(conformAll(cyc, nodeType, 10)[0]?.code, "depth-exceeded");
    assert.equal(errorCode(() => jsonText(cyc)), "depth-exceeded");

    // Huge list stops at conform step limit and caps errors.
    const bigBad = Array(10_000).fill("bad");
    const ds = conformAll(bigBad, listType(typeInt), 5, { steps: 20 });
    assert.equal(ds.length, 5);

    // Bad host numbers and lone surrogates.
    const env = createEnv({ varRecord: "{ x: any, s: string }" });
    assert.equal(errorCode(() => env.compile(parse("x")).run({ x: NaN, s: "" })), "number-range");
    assert.equal(errorCode(() => env.compile(parse("x")).run({ x: Infinity, s: "" })), "number-range");
    assert.equal(errorCode(() => env.compile(parse("x")).run({ x: 9007199254740992n, s: "" })), "number-range");
    assert.equal(Object.is(env.compile(parse("x")).run({ x: -0, s: "" }), 0), true);
    assert.equal(env.compile(parse("s")).run({ x: 0, s: "a\uD800b" }), "a\uFFFDb");
  });
});

describe("adversarial: editor tools on hostile text", () => {
  const env = createEnv({ varRecord: "{ a: int, s: string }" });

  test("tokens() stays linear on repeated unclosed block comments and nested templates", () => {
    const cases = [
      "/*\n".repeat(10_000),
      "`${\n".repeat(10_000),
      "const x = 1; /*\n".repeat(10_000),
      '"""\n'.repeat(10_000),
    ];
    for (const text of cases) {
      const t0 = performance.now();
      const out = tokens(text);
      assert.ok(out.length > 0);
      assert.ok(performance.now() - t0 < 2_000, `tokens() took too long on ${JSON.stringify(text.slice(0, 20))}`);
    }
  });

  test("tokens() pins colours after the line-by-line recovery fallback trips", () => {
    // 200 unclosed `${\n` lines burn past max(4096, 4 * length), switching the
    // remainder to line-by-line lexing: single-line tokens keep exact colour,
    // while multi-line constructs lose multi-line colour (opener line is invalid).
    const prefix = "`${\n".repeat(200);
    const tail = "const ok = 1 + 2;\n/* multi\nline */\n`hello\nworld`";
    const text = prefix + tail;
    const tailTokens = tokens(text)
      .filter((t) => t.start >= prefix.length)
      .map((t) => [t.class, text.slice(t.start, t.end)]);

    assert.deepEqual(tailTokens, [
      ["keyword", "const"],
      ["name", "ok"],
      ["operator", "="],
      ["number", "1"],
      ["operator", "+"],
      ["number", "2"],
      ["punctuation", ";"],
      ["invalid", "/* multi"],
      ["name", "line"],
      ["operator", "*"],
      ["operator", "/"],
      ["invalid", "`hello"],
      ["invalid", "world`"],
    ]);
  });

  test("format, complete, and hover handle truncated and pathological inputs without hanging", () => {
    const hostile = [
      "(".repeat(5_000),
      "{".repeat(5_000),
      "[".repeat(5_000),
      "const a = ; ".repeat(5_000),
      "a + ".repeat(10_000),
      "if (1) { ".repeat(500),
      '"""\n' + "a".repeat(20_000),
      "const a = 1;\0\0\0a.",
    ];
    for (const text of hostile) {
      const t0 = performance.now();
      env.complete(text, { line: 1, column: text.length + 1 });
      env.hover(text, { line: 1, column: 2 });
      try {
        format(text);
      } catch (e) {
        assert.ok(e instanceof RisorError);
      }
      assert.ok(performance.now() - t0 < 2_000, `editor tools took too long on ${JSON.stringify(text.slice(0, 20))}`);
    }
  });
});
