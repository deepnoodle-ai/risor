// Unit tests of package F6 for what the data cases cannot hold: texts too
// large for a case file, and laws over many values.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { createEnv, RisorError } from "../src/host/api.ts";

const env = createEnv({ vars: "{s: string, t: float}", groups: ["json", "time"], limits: { steps: null, allocation: null, depth: 1000 } });

function code(f: () => unknown): string {
  try {
    f();
  } catch (e) {
    assert.ok(e instanceof RisorError, String(e));
    return e.diagnostics[0].code;
  }
  assert.fail("no error");
}

describe("json.parse never uses the host's stack in proportion to the nesting (f06 6.2.5 item 4)", () => {
  const p = env.compile("json.parse(s)");
  test("a text nested 200,000 deep is depth-exceeded; one left open is not-json", () => {
    assert.equal(code(() => p.run({ s: "[".repeat(200000) + "]".repeat(200000) })), "depth-exceeded");
    assert.equal(code(() => p.run({ s: "[".repeat(200000) })), "invalid-argument");
  });
});

describe("time.format and time.parse round trip (f06 7.3 item 5)", () => {
  const fmt = env.compile("time.format(t)");
  const parse = env.compile("time.parse(s)");
  test("for whole and fractional times across the range", () => {
    let x = 12345;
    for (let i = 0; i < 2000; i++) {
      x = (x * 1103515245 + 12345) % 2147483648;
      const t = -62167219200 + (x / 2147483648) * (253402300800 + 62167219200 - 1);
      const v = i % 2 === 0 ? Math.floor(t) : t;
      const text = fmt.run({ t: v }) as string;
      assert.equal(parse.run({ s: text }), v + 0, text);
    }
  });
});
