// The deepest programs within the limits (02 8.4, 01 13.2). Run by deep.test.ts in
// a child process with a small V8 stack. Prints one line per program.

import { createEnv, execute, parse } from "../../src/host/api.ts";

/** depth - 1 calls in a chain, at nesting 0, then a walk of one level: the depth count reaches the limit. */
function calls(depth: number): string {
  const n = depth - 1;
  let src = "const f0 = x => [x == y, `${x}`];\n";
  for (let i = 1; i < n; i++) src += `const f${i} = x => f${i - 1}(x);\n`;
  src += `f${n - 1}(y)`;
  const env = createEnv({ varRecord: "{y: any}", limits: { depth, steps: null, allocation: null } });
  const o = execute(env.compile(parse(src), { check: false }), { y: [1] });
  return o.ok ? "ok" : o.diagnostic.code;
}

/** A walk alone, over data nested to the depth limit. */
function walk(depth: number): string {
  let y: unknown = 1;
  for (let i = 0; i < depth; i++) y = [y];
  const env = createEnv({ varRecord: "{y: any, z: any}", limits: { depth, steps: null, allocation: null } });
  const o = execute(env.compile(parse("[y == z, `${y}`]"), { check: false }), { y, z: y });
  return o.ok ? "ok" : o.diagnostic.code;
}

/** 100 levels of source nesting, in each kind of opener. */
function nesting(): string {
  const env = createEnv({ varRecord: "{c: any}", limits: { steps: null, allocation: null } });
  const srcs = [
    "(".repeat(100) + "1" + ")".repeat(100),
    "[".repeat(100) + "]".repeat(100),
    "-".repeat(100) + "1",
    "2" + " ** 1".repeat(100),
    "`${".repeat(100) + "1" + "}`".repeat(100),
    "c ? ".repeat(100) + "1" + " : 2".repeat(100),
  ];
  for (const s of srcs) {
    const o = execute(env.compile(parse(s), { check: false }), { c: true });
    if (!o.ok) return o.diagnostic.code;
  }
  return "ok";
}

console.log(["calls 200", calls(200), "calls 1000", calls(1000), "walk 1000", walk(1000), "nesting 100", nesting()].join(" "));
