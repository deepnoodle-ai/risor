// Package F1, host types: the description round trip and its errors (f01
// 5.3, 5.4), Pick, Meet and Close on hand-made types (3.6, 4.1), and a shared
// `$ref` target mapped once (3.10 item 5).

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, descriptionText, envFromDescription, RisorError, typeFromSchema } from "../src/host/api.ts";
import { close, meet, pick } from "../src/types/schema.ts";
import { ANY, INT, NEVER, STRING, printType, record } from "../src/types/type.ts";
import type { Rec } from "../src/types/type.ts";

const req = (type: import("../src/types/type.ts").Type) => ({ type, optional: false });
const opt = (type: import("../src/types/type.ts").Type) => ({ type, optional: true });

function declareCode(f: () => unknown): string {
  try {
    f();
  } catch (e) {
    assert.ok(e instanceof RisorError, String(e));
    return e.diagnostics[0].code;
  }
  assert.fail("expected a declare error");
}

describe("f01 5.3, 5.4: descriptions", () => {
  const e1 = createEnv({
    vars: "{n: int, t: Time | null}",
    schemas: { order: { schema: { type: "object", properties: { id: { type: "integer" }, note: { type: "string" } }, required: ["id"] } } },
    opaque: { Time: { test: () => false, methods: { format: { type: "(layout:string)=>string", call: () => "" } } } },
    functions: { rate: { type: "(code:string)=>float!", call: () => 0.1 }, "geo.distance": { type: "(a: T, b: T) => float where T: ordered", call: () => 0 } },
    reject: ["dynamic", "constant-condition"],
    limits: { steps: 500, allocation: null, depth: 10 },
  });
  test("the description and its text", () => {
    assert.equal(
      descriptionText(e1.describe()),
      '{"functions":{"geo.distance":"(a: T, b: T) => float where T: ordered","rate":"(code: string) => float !"},' +
        '"limits":{"allocation":null,"depth":10,"steps":500},"opaque":{"Time":{"format":"(layout: string) => string"}},' +
        '"reject":["constant-condition","dynamic"],"removed":[],"variables":{"n":"int","order":"{id: int, note?: string, ...}","t":"Time | null"},"version":"3.0"}',
    );
  });
  test("the round trip: the same description, and the same check", () => {
    const e2 = envFromDescription(JSON.parse(descriptionText(e1.describe())));
    assert.equal(descriptionText(e2.describe()), descriptionText(e1.describe()));
    for (const src of ["order.id + n", "order.note", "rate(order.note ?? \"x\")", "t?.format(\"iso\")", "geo.distance(1, 2)", "nope"]) {
      assert.deepEqual(JSON.stringify(e2.check(src)), JSON.stringify(e1.check(src)), src);
    }
  });
  test("an environment from a description has no implementation", () => {
    const e2 = envFromDescription(e1.describe());
    assert.equal(declareCode(() => e2.compile("n")), "no-implementation");
    const plain = envFromDescription(createEnv({ vars: "{n: int}" }).describe());
    assert.equal(plain.compile("n + 1").run({ n: 1 }), 2);
  });
  test("its declare errors are those of 03 3.3.4", () => {
    const base = createEnv({}).describe();
    assert.equal(declareCode(() => envFromDescription({ ...base, variables: { x: "number" } })), "bad-type");
    assert.equal(declareCode(() => envFromDescription({ ...base, variables: { if: "int" } })), "bad-name");
    assert.equal(declareCode(() => envFromDescription({ ...base, functions: { f: "(x) => int" } })), "bad-signature");
    assert.equal(declareCode(() => envFromDescription({ ...base, variables: { f: "int" }, functions: { f: "() => int" } })), "name-clash");
    assert.equal(declareCode(() => envFromDescription({ ...base, reject: ["nope"] })), "bad-class");
    assert.equal(declareCode(() => envFromDescription({ ...base, version: "9.9" })), "unsupported-version");
    assert.equal(declareCode(() => envFromDescription({ ...base, limits: { steps: -1, allocation: null, depth: 200 } })), "bad-limit");
  });
  test("a value that is not a description is the binding's own error (5.4 item 2)", () => {
    const base = createEnv({}).describe();
    for (const bad of [null, [], { ...base, extra: 1 }, { ...base, variables: { x: 1 } }, { ...base, limits: { steps: 1 } }]) {
      assert.throws(() => envFromDescription(bad), (e: unknown) => e instanceof Error && !(e instanceof RisorError));
    }
  });
});

describe("f01 3.6 and 4.1: Pick, Meet, Close", () => {
  const a = record([["a", opt(STRING)]], ANY).map!;
  const b = record([["b", opt(INT)]], ANY).map!;
  test("Pick drops any, meets records, else the first that fits every other", () => {
    assert.equal(printType(pick([ANY, STRING])), "string");
    assert.equal(printType(pick([STRING, INT])), "string");
    assert.equal(printType(pick([STRING, NEVER])), "never");
    assert.equal(printType(pick([{ map: a }, { map: b }])), "{a?: string, b?: int, ...}");
    assert.equal(printType(pick([])), "any");
  });
  test("Meet: a key an exact record lacks is never", () => {
    const exact = record([["a", opt(INT)]]).map!;
    const open = record([["b", req(STRING)]], ANY).map!;
    assert.equal(printType({ map: meet(exact, open) }), "{a?: int, b: never}");
  });
  test("Close is deep, and only of closable records; a Meet is closable when both are", () => {
    const ca: Rec = { ...a, closable: true };
    const cb: Rec = { ...b, closable: true };
    assert.equal(printType(close({ map: meet(ca, cb) })), "{a?: string, b?: int}");
    assert.equal(printType(close({ map: meet(ca, b) })), "{a?: string, b?: int, ...}");
    assert.equal(printType(close({ list: { map: ca } })), "{a?: string}[]");
  });
});

describe("f01 3.10 item 5: a shared $ref target is mapped once", () => {
  test("a doubling chain maps fast: under the size limit, and over it (05 20.7)", () => {
    const chain = (n: number) => {
      const defs: Record<string, unknown> = { d0: { type: "integer" } };
      for (let i = 1; i <= n; i++) {
        defs[`d${i}`] = { type: "object", properties: { a: { $ref: `#/$defs/d${i - 1}` }, b: { $ref: `#/$defs/d${i - 1}` } } };
      }
      return { $defs: defs, $ref: `#/$defs/d${n}` };
    };
    const t0 = performance.now();
    const env = createEnv({ schemas: { v: { schema: chain(14) } } });
    assert.ok(env.check("1").ok);
    assert.equal(declareCode(() => createEnv({ schemas: { v: { schema: chain(30) } } })), "bad-type");
    assert.ok(performance.now() - t0 < 1000);
  });
  test("typeFromSchema, and its bad-type", () => {
    assert.equal(typeFromSchema({ type: "object", properties: { a: { type: "string" } } }, ["closed"]), "{a?: string}");
    assert.equal(declareCode(() => typeFromSchema({ type: "int" })), "bad-type");
    assert.equal(declareCode(() => typeFromSchema({ $defs: { x: { $ref: "#/$defs/x" } }, $ref: "#/$defs/x" })), "bad-type");
  });
});
