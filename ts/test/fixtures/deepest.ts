// The deepest programs within the limits (02 8.4, 01 13.2, 03 3.9.1). Run by
// deep.test.ts in a child process with a small V8 stack. Prints one line.

import { createEnv, execute, parse } from "../../src/host/api.ts";
import { FORMS, VALUES, VARS, deepSource } from "./deep-forms.ts";

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

/**
 * A call of the library function `walk` (package F17) over a chain of nodes
 * `depth - 1` levels deep: the depth count reaches the limit at the deepest
 * node (f17 5.4). A walk keeps its place in a stack of its own and costs no
 * native stack for its levels (f17 5.3).
 */
function walkEntry(depth: number): string {
  const env = createEnv({ limits: { depth, steps: null, allocation: null } });
  const o = execute(env.compile(parse(`walk(0, (x, d) => d < ${depth - 1} ? [x + 1] : null)[${depth - 1}]`), { check: false }), {});
  if (!o.ok) return o.diagnostic.code;
  return o.value === depth - 1 ? "ok" : "wrong-value";
}

/**
 * Package F17, step E1 (f17 11.6): a typed lambda that calls itself, by name,
 * through `map` and through `flatMap`, each call adding 1 to the depth count.
 * `d(depth - 1)` makes `depth` calls and gives its value; `d(depth)` is
 * `depth-exceeded`, not a stack overflow.
 */
const SELF: Record<string, { src: string; value: unknown }> = {
  self: { src: "const d = (n: int): int => n <= 0 ? 0 : d(n - 1); d(N)", value: 0 },
  "self-map": { src: "const d = (n: int): int => n <= 0 ? 0 : [n - 1].map(d)[0]; d(N)", value: 0 },
  "self-flatMap": { src: "const d = (n: int): int[] => n <= 0 ? [] : [n - 1].flatMap(d); d(N)", value: [] },
};

function self(name: string, depth: number): string {
  const { src, value } = SELF[name];
  const env = createEnv({ limits: { depth, steps: null, allocation: null } });
  const run = (n: number) => execute(env.compile(parse(src.replace("N", String(n)))), {});
  const within = run(depth - 1);
  if (!within.ok) return within.diagnostic.code;
  if (JSON.stringify(within.value) !== JSON.stringify(value)) return "wrong-value";
  const past = run(depth);
  return past.ok ? "no-limit" : past.diagnostic.code === "depth-exceeded" ? "ok" : past.diagnostic.code;
}

/**
 * Package F17, step E3 (f17 13.4): recursions through a function parameter.
 * `max(L)` is the largest N whose run stays within the depth limit L; N + 1
 * is `depth-exceeded`, not a stack overflow. f17 13.4 item 5 claims no bound
 * for these programs, so the binding tests its own: a call of a function
 * parameter, a lambda bound to one and given on, and one given to `map`.
 */
const FN_PARAMS: Record<string, { src: string; value: unknown; max: (limit: number) => number }> = {
  // N + 1 calls of r, then f(0), in the middle of `? :`, adds 2.
  "fn-bottom": { src: "const r = (f: (x: int) => int, n: int): int => n <= 0 ? f(0) : r(f, n - 1); r(x => x + 1, N)", value: 1, max: (l) => l - 3 },
  // For each level, d, apply and the lambda bound to f each add 1: 3N + 1.
  "fn-apply": { src: "const apply = (f: (x: int) => int, x: int): int => f(x); const d = (n: int): int => n <= 0 ? 0 : apply(y => d(y), n - 1); d(N)", value: 0, max: (l) => Math.floor((l - 1) / 3) },
  // For each level, d and g each add 1, d through the measure of `map`: 2N + 2.
  "fn-map": { src: "const g = (f: (x: int) => int, n: int): int => n <= 0 ? 0 : [n - 1].map(f)[0]; const d = (n: int): int => g(d, n); d(N)", value: 0, max: (l) => Math.floor((l - 2) / 2) },
};

function fnParams(name: string, depth: number): string {
  const { src, value, max } = FN_PARAMS[name];
  const env = createEnv({ limits: { depth, steps: null, allocation: null } });
  const run = (n: number) => execute(env.compile(parse(src.replace("N", String(n)))), {});
  const within = run(max(depth));
  if (!within.ok) return within.diagnostic.code;
  if (JSON.stringify(within.value) !== JSON.stringify(value)) return "wrong-value";
  const past = run(max(depth) + 1);
  return past.ok ? "no-limit" : past.diagnostic.code === "depth-exceeded" ? "ok" : past.diagnostic.code;
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

/** A form of deep-forms.ts, its calls through a library entry, at the depth limit `depth`. */
function form(name: string, depth: number): string {
  const f = FORMS.find((x) => x.name === name)!;
  const env = createEnv({ varRecord: VARS, limits: { depth, steps: null, allocation: null } });
  const o = execute(env.compile(parse(deepSource(f, depth)), { check: false }), VALUES);
  if (!o.ok) return o.diagnostic.code;
  return JSON.stringify(o.value) === JSON.stringify(f.expect) ? "ok" : "wrong-value";
}

// 03 3.9.1: each deepest case at the default depth limit, 200, and at the largest, 400.
const out = ["nesting 100", nesting()];
for (const depth of [200, 400]) {
  out.push(`calls ${depth}`, calls(depth), `walk ${depth}`, walk(depth), `walk() ${depth}`, walkEntry(depth));
  for (const f of FORMS) out.push(`${f.name} ${depth}`, form(f.name, depth));
  for (const name of Object.keys(SELF)) out.push(`${name} ${depth}`, self(name, depth));
  for (const name of Object.keys(FN_PARAMS)) out.push(`${name} ${depth}`, fnParams(name, depth));
}
console.log(out.join(" "));
