// The library: the registry against suite/library.json (04 1),
// host numbers read by entries (03 3.4.4), and the work bound of the
// algorithms this binding wrote itself (02 8.5).

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { FUNCTIONS, METHODS } from "../src/library/registry.ts";
import type { Entry } from "../src/library/registry.ts";
import { parseReceiver, parseSignature } from "../src/types/typetext.ts";
import { createEnv, RisorError } from "../src/host/api.ts";
import { SUITE } from "./suite-report.ts";
import { CASEMAP_UNICODE } from "../src/library/text.ts";

interface Rec { name: string; receiver?: string; signature: string; steps: string; units: string }

describe("the registry is suite/library.json", () => {
  const records = JSON.parse(readFileSync(join(SUITE, "library.json"), "utf8")) as Rec[];
  const registered: Entry[] = [...FUNCTIONS.values(), ...[...METHODS.values()].flatMap((m) => [...m.values()])];
  test("53 entries, each implemented, none added in code alone", () => {
    assert.equal(records.length, 53);
    assert.equal(registered.length, 53);
    for (const e of registered) assert.ok(e.impl !== null, `${e.name} has no implementation`);
  });
  for (const r of records) {
    test(`${r.receiver ?? ""}${r.receiver ? "." : ""}${r.name}`, () => {
      assert.ok(r.steps !== undefined && r.units !== undefined, "a record has steps and units (04 1.3 item 1)");
      const kind = r.receiver === undefined ? null : (() => {
        const t = parseReceiver(r.receiver!);
        assert.ok(t.ok);
        return t.node.t === "word" ? "string" : t.node.t === "list" ? "list" : "map";
      })();
      const e = kind === null ? FUNCTIONS.get(r.name) : METHODS.get(kind)!.get(r.name);
      assert.ok(e !== undefined, "registered");
      assert.equal(e.kind, kind);
      assert.equal(e.record.signature, r.signature);
      const s = parseSignature(r.signature);
      assert.ok(s.ok);
      assert.deepEqual(
        e.params.map((p) => [p.name, p.optional, p.rest, p.fn]),
        s.node.params.map((p) => [p.name, p.optional, p.rest, p.type.t === "fn" ? p.type.params.length : null]),
      );
      const min = s.node.params.filter((p) => !p.optional && !p.rest).length;
      const max = s.node.params.some((p) => p.rest) ? Infinity : s.node.params.length;
      assert.deepEqual([e.counts.min, e.counts.max], [min, max]);
    });
  }
  test("the case table is Unicode 16.0.0", () => assert.equal(CASEMAP_UNICODE, "16.0.0"));
});

describe("03 3.4.4: an entry that reads an element reads a host number", () => {
  const env = createEnv({ vars: "{xs: any}" });
  const code = (src: string, xs: unknown[]) => {
    const o = env.compile(src, { check: false }).execute({ xs });
    return o.ok ? "ok" : o.diagnostic.code;
  };
  const big = [1, 2n ** 60n];
  for (const src of ["xs.sort()", "xs.map(x => 1)", "xs.sum()", "xs.min()", "xs.unique()", "xs.last()", "string(xs)", "xs.join(\",\")"]) {
    test(`${src} fails with number-range`, () => assert.equal(code(src, big), "number-range"));
  }
  test("NaN in an element handed to a callback", () => assert.equal(code("xs.filter(x => true)", [NaN]), "number-range"));
  for (const src of ["len(xs)", "xs.reverse()", "xs.slice(0)", "[...xs]", "xs.first()"]) {
    test(`${src} copies or measures without reading the element`, () => assert.equal(code(src, big), "ok"));
  }
  test("the failure has the span of the call (04 2.6 item 6)", () => {
    try {
      env.compile("1 + xs.sort()", { check: false }).run({ xs: big });
      assert.fail("no failure");
    } catch (e) {
      assert.ok(e instanceof RisorError);
      assert.deepEqual(e.diagnostics[0].span, { start: 4, end: 13 });
    }
  });
});

describe("work in proportion to the charge", () => {
  const env = createEnv({ vars: "{s: any, t: any, xs: any}", limits: { steps: null, allocation: null } });
  const timed = (src: string, vars: Record<string, unknown>) => {
    const t0 = performance.now();
    const o = env.compile(src, { check: false }).execute(vars);
    const ms = performance.now() - t0;
    assert.ok(o.ok, o.ok ? "" : o.diagnostic.code);
    return { ms, steps: o.usage.steps, value: o.value };
  };
  // A text of 2,000,000 `a`, searched for 100 `a` then a `b`: a naive search
  // does 200,000,000 comparisons; the bound allows a constant times 2,000,000.
  const s = "a".repeat(2_000_000);
  const t = "a".repeat(100) + "b";
  test("a long search (in, split, replaceAll) is linear", () => {
    for (const src of ["t in s", "s.split(t)", "s.replaceAll(t, \"x\")"]) {
      const r = timed(src, { s, t });
      assert.ok(r.ms < 1500, `${src}: ${r.ms} ms`);
    }
  });
  test("a sort of 200,000 elements", () => {
    const xs = Array.from({ length: 200_000 }, (_, i) => (i * 7919) % 200_003);
    const r = timed("xs.sort()", { xs });
    assert.ok(r.ms < 1500, `${r.ms} ms`);
    const v = r.value as number[];
    for (let i = 1; i < v.length; i++) assert.ok(v[i - 1] <= v[i]);
  });
  test("a join of 200,000 strings", () => {
    const xs = Array.from({ length: 200_000 }, () => "word");
    const r = timed('xs.join(", ")', { xs });
    assert.ok(r.ms < 1500, `${r.ms} ms`);
    assert.equal((r.value as string).length, 200_000 * 4 + 199_999 * 2);
  });
});

describe("04 2.6: a type variable with no bound accepts null", () => {
  test("[].reduce(p => 0, null) gives null", () => {
    const env = createEnv({});
    assert.equal(env.compile("[].reduce(p => 0, null)", { check: false }).run(), null);
  });
});
