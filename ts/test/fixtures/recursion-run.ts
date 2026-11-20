// One part of test/recursion.test.ts, in a process of its own with a small
// stack: `node recursion-run.ts PART`. Each part builds long chains of
// recursive types, whose comparison passes the bound of 4,096 pairs (f17 12.5
// item 7), and walks values deeper than the conformance function's limit
// (12.7 item 3). It prints `ok`, or it throws. A RangeError is the stack.

import assert from "node:assert/strict";
import { conform, conformAll, createEnv, execute, format, parse, tryCompile } from "../../src/host/api.ts";
import { tokens } from "../../src/tools/tokens.ts";

/** The types of each chain: more than the bound, so a comparison along it reaches the bound. */
const N = 6000;

/** `type S0 = {a: S1}; ...; type S{N-1} = {a: S0};` as type lines or as named types. */
function chain(name: string, n: number): { name: string; type: string }[] {
  const out: { name: string; type: string }[] = [];
  for (let i = 0; i < n; i++) out.push({ name: `${name}${i}`, type: `{a: ${name}${(i + 1) % n}}` });
  return out;
}

/** A value nested `levels` maps deep: {a: {a: ... null}}. */
function nested(levels: number): unknown {
  let v: unknown = null;
  for (let i = 0; i < levels; i++) v = { a: v };
  return v;
}

function through<T>(what: string, f: () => T): T {
  try {
    return f();
  } catch (e) {
    if (e instanceof RangeError) throw new Error(`${what}: ${e.message}`);
    throw e;
  }
}

const parts: Record<string, () => void> = {
  // Type lines: S0 fits T0 enters a new pair at each level.
  "type lines"() {
    const lines = [...chain("S", N), ...chain("T", N)].map((l) => `type ${l.name} = ${l.type};`).join("\n");
    const source = `${lines}\nconst s: S0 = x as S0;\nconst t: T0 = s;\n1`;
    const env = createEnv({ vars: { x: "any" } });
    const r = through("compile", () => tryCompile(env, source));
    assert.equal(r.ok, false);
    if (!r.ok) {
      assert.deepEqual(r.diagnostics.map((d) => d.code), ["check-limit"]);
      assert.match(r.diagnostics[0].message, /4,096|pairs/);
    }
    // The other entry points that read the program.
    through("parse", () => parse(source));
    through("format", () => format(source));
    through("tokens", () => tokens(source));
    const at = source.indexOf("T0 = s");
    through("hover", () => env.hover(source, { line: source.slice(0, at).split("\n").length, column: 7 }));
    through("completion", () => env.complete(source, { line: source.slice(0, at).split("\n").length, column: 10 }));
  },

  // An environment's named types: one list of 12,000 items (f17 12.4).
  "named types"() {
    const env = through("declare", () => createEnv({ namedTypes: [...chain("S", N), ...chain("T", N)], vars: { s: "S0" } }));
    const r = through("compile", () => tryCompile(env, "const t: T0 = s; 1"));
    assert.equal(r.ok, false);
    if (!r.ok) assert.deepEqual(r.diagnostics.map((d) => d.code), ["check-limit"]);
    // A program the bound does not stop: each step of the read is one level.
    const ok = through("compile", () => tryCompile(env, "s.a.a.a"));
    assert.equal(ok.ok, true);
    // The description lists every named type by its unfolding.
    const d = through("describe", () => env.describe());
    assert.equal(d.types?.S0, "{a: S1}");
    assert.equal(d.types?.[`S${N - 1}`], "{a: S0}");
  },

  // as and is follow the data, to the depth limit of the run (f17 12.7 item 1).
  "as"() {
    const env = createEnv({ namedTypes: chain("S", N), vars: { x: "any" } });
    const r = tryCompile(env, "x as S0");
    assert.equal(r.ok, true);
    if (!r.ok) return;
    const o = through("run", () => execute(r.program, { x: nested(1000) }));
    assert.equal(o.ok, false);
    if (!o.ok) assert.equal(o.diagnostic.code, "depth-exceeded");
  },

  // The conformance function stops past 200 levels (f17 12.7 item 3).
  "conform"() {
    const env = createEnv({ namedTypes: [{ name: "Deep", type: "{a: Deep | null}" }] });
    const t = env.parseType("Deep");
    const d = through("conform", () => conform(nested(100000), t));
    assert.equal(d?.code, "depth-exceeded");
    const all = through("conformAll", () => conformAll(nested(100000), t, 10));
    assert.deepEqual(all.map((x) => x.code), ["depth-exceeded"]);
    const issues = through("validate", () => t["~standard"].validate(nested(100000)));
    assert.ok("issues" in issues && issues.issues !== undefined && issues.issues.length === 1);
  },
};

const name = process.argv[2];
const part = parts[name];
if (part === undefined) throw new Error(`no part ${name}`);
part();
process.stdout.write("ok\n");
