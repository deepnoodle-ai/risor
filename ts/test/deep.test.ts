// 02 8.4, 01 13.2, 03 3.9.1: the stack this binding needs. The binding
// documents Node's default stack, 984 KB (src/index.ts, STACK), so the deepest
// cases run in a child process whose V8 stack is that divided by 1.5, 656 KB:
// 100 levels of source nesting, and at the default depth limit, 200, and the
// largest, 400, calls by name, a walk of nested data, a call of the library
// function `walk` whose nodes reach the limit (f17 5.3, 5.4), the forms of
// fixtures/deep-forms.ts, whose calls go through a library entry that calls a
// lambda, a typed lambda that calls itself by name, through `map` and
// through `flatMap` (f17 11.6), and recursions through a function parameter
// (f17 13.4).

import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));

test("the deepest programs run with a 656 KB stack", () => {
  const out = execFileSync(process.execPath, ["--stack-size=656", join(here, "fixtures", "deepest.ts")], { encoding: "utf8" });
  assert.equal(
    out.trim(),
    [
      "nesting 100 ok",
      "calls 200 ok walk 200 ok walk() 200 ok map 200 ok flatMap 200 ok heaviest 200 ok self 200 ok self-map 200 ok self-flatMap 200 ok fn-bottom 200 ok fn-apply 200 ok fn-map 200 ok",
      "calls 400 ok walk 400 ok walk() 400 ok map 400 ok flatMap 400 ok heaviest 400 ok self 400 ok self-map 400 ok self-flatMap 400 ok fn-bottom 400 ok fn-apply 400 ok fn-map 400 ok",
    ].join(" "),
  );
});

test("long flat programs need no stack (01 13.2 rule 6)", async () => {
  const { createEnv, execute, parse } = await import("../src/host/api.ts");
  const n = 100000;
  const env = createEnv({ varRecord: "{m: any}", limits: { steps: null, allocation: null } });
  const m: Record<string, unknown> = {};
  m.a = m;
  const run = (src: string) => {
    const o = execute(env.compile(parse(src), { check: false }), { m });
    assert.ok(o.ok, o.ok ? "" : o.diagnostic.code);
    return o.value;
  };
  assert.equal(run(Array.from({ length: n }, () => "1").join(" + ")), n);
  assert.equal(run("type(m" + ".a".repeat(n) + ")"), "map");
  assert.equal(run("false ? 1 : ".repeat(n) + "2"), 2);
  assert.equal(run("if (false) { 1 } " + "else if (false) { 2 } ".repeat(n) + "else { 3 }"), 3);
  assert.equal(run(Array.from({ length: n }, (_, i) => `const a${i} = ${i};`).join(" ") + ` a${n - 1}`), n - 1);
  assert.equal(run("true" + " && true".repeat(n)), true);
  // A chain of method calls, and steps after an optional one: one spine (src/eval/compile.ts).
  assert.equal(run("len([1]" + ".reverse()".repeat(n) + ")"), 1);
  assert.equal(run("type(m" + "?.a".repeat(n) + ")"), "map");
});
