import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv } from "@deepnoodle/risor";
import { describeSample, localService, parseJsonTree, prettyJSON, TextIndex, typeOfJSON, workerService } from "../src/index.ts";
import type { WorkerLike } from "../src/index.ts";
import { answer } from "../src/worker.ts";
import type { Request, Response } from "../src/protocol.ts";

describe("TextIndex", () => {
  test("a text with no astral code point counts the same both ways", () => {
    const ix = new TextIndex("ab\ncd");
    assert.equal(ix.unitOf(4), 4);
    assert.deepEqual(ix.positionOf(4), { line: 2, column: 2 });
    assert.equal(ix.unitOfPosition(2, 2), 4);
  });

  test("an astral code point is two units and one code point", () => {
    const text = `"😀" + x\ny`;
    const ix = new TextIndex(text);
    assert.equal(ix.length, 9);
    assert.equal(ix.unitOf(3), 4); // after the closing quote
    assert.equal(ix.pointOf(4), 3);
    assert.equal(ix.pointOf(2), 1); // inside the pair: its start
    assert.deepEqual(ix.positionOf(text.indexOf("x")), { line: 1, column: 7 });
    assert.equal(ix.unitOfPosition(2, 1), text.indexOf("y"));
  });

  test("a byte order mark: Risor's offsets and positions start after it", () => {
    const ix = new TextIndex("﻿ab");
    assert.equal(ix.unitOfRisor(0), 1);
    assert.equal(ix.unitOfPosition(1, 1), 1);
    assert.deepEqual(ix.positionOf(2), { line: 1, column: 2 });
  });
});

describe("describeSample", () => {
  test("types each variable from its value", () => {
    assert.equal(typeOfJSON({ b: 1.5, a: 1, c: [{ x: 1 }, { x: 2, y: "s" }], d: [], e: null }),
      "{a: int, b: float, c: {x: int, y?: string}[], d: any[], e: null}");
  });

  test("a key that is a word prints bare, even a reserved one", () => {
    assert.equal(typeOfJSON({ match: 1, "a b": 2 }), '{"a b": int, match: int}');
    assert.equal(createEnv({ vars: { x: "{match: int}" } }).describe().variables.x, "{match: int}");
  });

  test("is the description createEnv gives for the same variables", () => {
    const sample = { order: { total: 1.5, items: [{ sku: "a" }] }, n: 3 };
    const d = describeSample(sample, { groups: ["math", "json"] });
    const variables = Object.fromEntries(Object.entries(sample).map(([k, v]) => [k, typeOfJSON(v)]));
    assert.deepEqual(d, createEnv({ vars: variables, groups: ["math", "json"] }).describe());
    assert.deepEqual(describeSample({}), createEnv().describe());
  });

  test("a key that cannot be a name is a RangeError", () => {
    assert.throws(() => describeSample({ "not a name": 1 }), RangeError);
    assert.throws(() => describeSample({ if: 1 }), RangeError);
  });
});

describe("the JSON tree", () => {
  test("keeps each number's text", () => {
    assert.deepEqual(parseJsonTree(`{"a":1.0,"b":[9007199254740993,-2e5],"c":"x\\ny","d":null,"e":true}`), {
      kind: "object",
      entries: [
        ["a", { kind: "number", text: "1.0" }],
        ["b", { kind: "array", items: [{ kind: "number", text: "9007199254740993" }, { kind: "number", text: "-2e5" }] }],
        ["c", { kind: "string", value: "x\ny" }],
        ["d", { kind: "null" }],
        ["e", { kind: "bool", value: true }],
      ],
    });
    assert.throws(() => parseJsonTree("{"), SyntaxError);
    assert.equal(prettyJSON(`{"a":[1.0,{}]}`), `{\n  "a": [\n    1.0,\n    {}\n  ]\n}`);
  });
});

describe("localService", () => {
  const service = localService();
  const env = service.environment(describeSample({ order: { total: 1.5, items: [{ sku: "a" }] } }));

  test("check: a problem has UTF-16 offsets and Risor's line and column", async () => {
    const text = `"😀" + order.totl`;
    const ps = (await env.check(text))!;
    assert.equal(ps.length, 1);
    assert.equal(ps[0].code, "no-field");
    assert.equal(text.slice(ps[0].from, ps[0].to), "totl");
    assert.equal(ps[0].line, 1);
    assert.equal(ps[0].column, text.indexOf("totl"));
  });

  test("check with an expected type", async () => {
    assert.deepEqual(await env.check("order.total > 1", { expected: "bool" }), []);
    const ps = (await env.check("order.total", { expected: "bool" }))!;
    assert.equal(ps.length, 1);
  });

  test("complete gives the fields of a record, at UTF-16 offsets", async () => {
    const text = `"😀" + order.it`;
    const c = (await env.complete(text, text.length))!;
    assert.ok(c.items.some((i) => i.label === "items"), JSON.stringify(c.items.map((i) => i.label)));
    assert.equal(text.slice(c.from, c.to), "it");
  });

  test("hover gives the type of a name", async () => {
    const text = "order.total > 1";
    const h = (await env.hover(text, 8))!;
    assert.equal(h.type, "float");
    assert.equal(text.slice(h.from, h.to), "total");
  });

  test("run gives JSON text, the type and the usage", async () => {
    const r = await env.run("{t: order.total * 2, one: 1.0}", { order: { total: 1.5, items: [] } });
    if (!r.ok) throw new Error(JSON.stringify(r.problems));
    assert.equal(r.json, `{"one":1,"t":3}`);
    assert.equal(r.type, "{one: int, t: float}");
    assert.ok(r.usage.steps > 0);
  });

  test("run: a compile error, a run error, and a limit", async () => {
    const c = await env.run("order.nope");
    assert.equal(c.ok, false);
    assert.equal(!c.ok && c.stage, "compile");
    const r = await env.run(`fail("no")`, { order: { total: 1, items: [] } });
    assert.equal(!r.ok && r.stage, "run");
    const tight = service.environment({ ...describeSample({}), limits: { steps: 100, allocation: null, depth: 200 } });
    const l = await tight.run("range(1000).map(i => i)");
    assert.equal(!l.ok && l.problems[0].code, "steps-exceeded");
  });

  test("run: the deadline ends a long run", async () => {
    const free = service.environment({ ...describeSample({}), limits: { steps: null, allocation: null, depth: 200 } });
    const t0 = Date.now();
    const r = await free.run("len(range(100000).map(i => len(range(1000).map(j => j))))", {}, { timeoutMs: 50 });
    assert.equal(!r.ok && r.problems[0].code, "deadline-exceeded");
    assert.ok(Date.now() - t0 < 2_000);
  });

  test("a description with a host function checks, completes and hovers, and does not run", async () => {
    const d = { ...describeSample({}), functions: { "geo.distance": "(a: float, b: float) => float" } };
    const e = service.environment(d);
    assert.deepEqual(await e.check("geo.distance(1.0, 2.0) > 3.0"), []);
    const c = (await e.complete("geo.di", 6))!;
    assert.ok(c.items.some((i) => i.label === "distance"));
    const r = await e.run("1");
    assert.equal(!r.ok && r.problems[0].code, "host-functions");
  });

  test("a value that is not a description: one console error, and the empty environment", async () => {
    const orig = console.error;
    const logged: unknown[] = [];
    console.error = (...a: unknown[]) => logged.push(a);
    try {
      const e = service.environment({ variables: { x: "int" } } as never);
      assert.deepEqual(await e.check("1 + 1"), []);
      const r = await e.run("1 + 1");
      assert.ok(r.ok);
      assert.equal(logged.length, 1);
      assert.match(String((logged[0] as unknown[])[0]), /not a description/);
    } finally {
      console.error = orig;
    }
  });

  test("format", async () => {
    assert.deepEqual(await service.format("{a:1,b:[1,2]}"), { ok: true, text: "{a: 1, b: [1, 2]}\n" });
    const bad = await service.format("{a:");
    assert.equal(bad.ok, false);
  });
});

/** A worker that answers in the same process, after a turn, and can hang on runs. */
class FakeWorker implements WorkerLike {
  onmessage: ((e: MessageEvent<Response>) => void) | null = null;
  terminated = false;
  received: Request[] = [];
  static hang = false;
  static made = 0;
  constructor() {
    FakeWorker.made++;
  }
  postMessage(m: Request): void {
    structuredClone(m); // throws for a value a real worker cannot receive
    this.received.push(m);
    if (m.op === "run" && FakeWorker.hang) return;
    setTimeout(() => {
      if (!this.terminated) this.onmessage?.({ data: answer(m) } as MessageEvent<Response>);
    }, 1);
  }
  terminate(): void {
    this.terminated = true;
  }
}

describe("workerService", () => {
  test("calls go through the worker", async () => {
    const s = workerService({ worker: () => new FakeWorker() });
    const env = s.environment(describeSample({ a: 1 }));
    assert.deepEqual(await env.check("a + 1"), []);
    const r = await env.run("a + 1", { a: 2 });
    assert.ok(r.ok && r.json === "3");
    s.dispose();
  });

  test("a check made while another waits replaces it", async () => {
    const s = workerService({ worker: () => new FakeWorker() });
    const env = s.environment(describeSample({ a: 1 }));
    const p1 = env.check("a +"); // sent
    const p2 = env.check("a + "); // waits, then replaced
    const p3 = env.check("a + 1"); // waits, then sent
    const [r1, r2, r3] = await Promise.all([p1, p2, p3]);
    assert.equal(r1!.length, 1);
    assert.equal(r2, null);
    assert.deepEqual(r3, []);
    // validate is never replaced.
    const [v1, v2] = await Promise.all([env.validate("a +"), env.validate("a + 1")]);
    assert.equal(v1.length, 1);
    assert.deepEqual(v2, []);
    s.dispose();
  });

  test("a run that does not answer ends the worker; the next call gets a new one with the environment", async () => {
    FakeWorker.made = 0;
    FakeWorker.hang = true;
    const s = workerService({ worker: () => new FakeWorker(), graceMs: 10 });
    const env = s.environment(describeSample({ a: 1 }));
    const r = await env.run("a", { a: 1 }, { timeoutMs: 10 });
    assert.equal(!r.ok && r.stage, "stopped");
    FakeWorker.hang = false;
    assert.deepEqual(await env.check("a + 1"), []);
    assert.equal(FakeWorker.made, 2);
    s.dispose();
  });

  test("a worker's load time does not count against the first run", async () => {
    // The worker answers nothing for 60 ms, as if it were still loading; the run's deadline and grace are 20 ms.
    class Slow extends FakeWorker {
      private readonly start = Date.now();
      override postMessage(m: Request): void {
        this.received.push(m);
        setTimeout(() => {
          if (!this.terminated) this.onmessage?.({ data: answer(m) } as MessageEvent<Response>);
        }, Math.max(1, 60 - (Date.now() - this.start)));
      }
    }
    const s = workerService({ worker: () => new Slow(), graceMs: 10 });
    const env = s.environment(describeSample({ a: 1 }));
    const r = await env.run("a", { a: 1 }, { timeoutMs: 10 });
    assert.ok(r.ok);
    s.dispose();
  });

  test("a check waiting when the worker is ended is answered by the new worker", async () => {
    FakeWorker.hang = true;
    let current: FakeWorker | null = null;
    const s = workerService({ worker: () => (current = new FakeWorker()), graceMs: 10 });
    const env = s.environment(describeSample({ a: 1 }));
    const run = env.run("a", { a: 1 }, { timeoutMs: 10 });
    const first = current!;
    // Hold the check's answer in the first worker, so it is still in flight when the run times out.
    first.postMessage = (m: Request) => {
      first.received.push(m);
    };
    const check = env.check("a +");
    assert.equal((await run).ok, false);
    FakeWorker.hang = false;
    const ps = await check;
    assert.equal(ps!.length, 1);
    assert.notEqual(current, first);
    s.dispose();
  });

  test("a value the worker cannot receive gives a run result, not a rejection", async () => {
    const s = workerService({ worker: () => new FakeWorker() });
    const env = s.environment(describeSample({ a: 1 }));
    const r = await env.run("a", { a: () => 1 } as unknown as Record<string, unknown>);
    assert.equal(!r.ok && r.stage, "run");
    assert.match(!r.ok ? r.problems[0].message : "", /cannot be sent to the worker/);
    assert.deepEqual(await env.check("a + 1"), []); // the service still works
    s.dispose();
  });

  test("a deadline past setTimeout's range does not stop the run at once", async () => {
    const s = workerService({ worker: () => new FakeWorker() });
    const env = s.environment(describeSample({ a: 1 }));
    const r = await env.run("a", { a: 1 }, { timeoutMs: 3_000_000_000 }); // about 35 days
    assert.ok(r.ok);
    s.dispose();
  });

  test("a worker that does not load in time fails what is pending; the next call gets a new worker", async () => {
    let made = 0;
    class Stalled extends FakeWorker {
      override postMessage(m: Request): void {
        this.received.push(m);
      }
    }
    const s = workerService({ worker: () => (made++ === 0 ? new Stalled() : new FakeWorker()), loadTimeoutMs: 20 });
    const env = s.environment(describeSample({ a: 1 }));
    const [ps, r] = await Promise.all([env.check("a + 1"), env.run("a", { a: 1 })]);
    assert.match(ps![0].message, /did not load within 20 ms/);
    assert.equal(r.ok, false);
    assert.deepEqual(await env.check("a + 1"), []);
    assert.equal(made, 2);
    s.dispose();
  });

  test("dispose resolves what is pending", async () => {
    FakeWorker.hang = true;
    const s = workerService({ worker: () => new FakeWorker() });
    const env = s.environment();
    const r = env.run("1");
    s.dispose();
    assert.equal((await r).ok, false);
    FakeWorker.hang = false;
  });
});

describe("release", () => {
  test("an environment used after release opens again (React StrictMode does this)", async () => {
    for (const s of [localService(), workerService({ worker: () => new FakeWorker() })]) {
      const env = s.environment(describeSample({ a: 1 }));
      env.release();
      assert.deepEqual(await env.check("a + 1"), []);
      const r = await env.run("a", { a: 2 });
      assert.ok(r.ok);
      s.dispose();
    }
  });
});

describe("a worker that fails and a disposed service", () => {
  test("a worker that fails to load fails every pending call and logs one error", async () => {
    class Broken extends FakeWorker {
      override postMessage(m: Request): void {
        this.received.push(m);
        setTimeout(() => this.onerror?.(new Event("error")), 1);
      }
      onerror: ((e: Event) => void) | null = null;
    }
    let made = 0;
    const orig = console.error;
    const logged: unknown[] = [];
    console.error = (...a: unknown[]) => logged.push(a);
    try {
      const s = workerService({ worker: () => (made++, new Broken()) });
      const env = s.environment();
      const [v, r] = await Promise.all([env.validate("1"), env.run("1")]);
      assert.equal(v[0].code, "internal-error");
      assert.match(v[0].message, /worker/);
      assert.equal(!r.ok && r.stage, "run");
      assert.equal(logged.length, 1);
      // Later calls fail at once, and no new worker is made.
      assert.equal((await env.check("1"))![0].code, "internal-error");
      assert.equal(made, 1);
    } finally {
      console.error = orig;
    }
  });

  test("dispose: runs resolve stopped and no new worker starts", async () => {
    FakeWorker.hang = true;
    let made = 0;
    const s = workerService({ worker: () => (made++, new FakeWorker()) });
    const env = s.environment();
    const r1 = env.run("1");
    const r2 = env.run("2");
    const c1 = env.check("1");
    const c2 = env.check("2");
    s.dispose();
    FakeWorker.hang = false;
    const [a, b] = await Promise.all([r1, r2]);
    assert.equal(!a.ok && a.stage, "stopped");
    assert.equal(!b.ok && b.stage, "stopped");
    await Promise.all([c1, c2]);
    assert.equal(made, 1);
  });
});
