// Ruling AA7: the stack this binding needs. The deepest programs within the
// limits run in a child process whose V8 stack is 500 KB, about half of
// Node's default (984 KB). Measured: 400 KB suffices at depth limit 1,000.

import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));

test("the deepest programs run with a 500 KB stack", () => {
  const out = execFileSync(process.execPath, ["--stack-size=500", join(here, "fixtures", "deepest.ts")], { encoding: "utf8" });
  assert.equal(out.trim(), "calls 200 ok calls 1000 ok walk 1000 ok nesting 100 ok");
});

test("long flat programs need no stack (01 13.2 rule 6)", async () => {
  const { createEnv } = await import("../src/host/api.ts");
  const n = 100000;
  const env = createEnv({ vars: "{m: any}", limits: { steps: null, allocation: null } });
  const m: Record<string, unknown> = {};
  m.a = m;
  const run = (src: string) => {
    const o = env.compile(src, { check: false }).execute({ m });
    assert.ok(o.ok, o.ok ? "" : o.diagnostic.code);
    return o.value;
  };
  assert.equal(run(Array.from({ length: n }, () => "1").join(" + ")), n);
  assert.equal(run("type(m" + ".a".repeat(n) + ")"), "map");
  assert.equal(run("false ? 1 : ".repeat(n) + "2"), 2);
  assert.equal(run("if (false) { 1 } " + "else if (false) { 2 } ".repeat(n) + "else { 3 }"), 3);
  assert.equal(run(Array.from({ length: n }, (_, i) => `const a${i} = ${i};`).join(" ") + ` a${n - 1}`), n - 1);
  assert.equal(run("true" + " && true".repeat(n)), true);
});
