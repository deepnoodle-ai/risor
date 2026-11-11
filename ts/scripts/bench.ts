// The shared benchmark programs of suite/bench, run through the host API
// only, as the Go and Rust harnesses run them:
// `node scripts/bench.ts [NAME-FILTER]`, from ts/.
//
// For each program it first checks the value, steps and units against the
// file's expect, then measures:
//
//   compile_ns  env.compile of the source text (parse, check, compile), in an
//               environment built once;
//   run_ns      one run of the compiled program on the file's vars.
//
// Each is the median of 15 samples, after a warm-up of 200 ms; a sample is a
// batch of calls that takes at least 10 ms, divided by its size. It prints
// one JSON line for each program: impl, program, compile_ns, run_ns, steps,
// allocated.

import { readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { isDeepStrictEqual } from "node:util";
import { createEnv, jsonText, parse } from "../src/index.ts";

interface BenchFile {
  env: string;
  limits: { steps: number; allocation: number };
  vars: Record<string, unknown>;
  cases: { name: string; source: string; expect: { value: unknown; steps: number; allocated: number } }[];
}

function median(fn: () => void): number {
  const warmMs = 200, minSampleMs = 10, samples = 15;
  const start = performance.now();
  for (let calls = 0; calls < 10 || performance.now() - start < warmMs; calls++) fn();
  const batch = (n: number): number => {
    const t = performance.now();
    for (let i = 0; i < n; i++) fn();
    return performance.now() - t;
  };
  let n = 1;
  while (batch(n) < minSampleMs) n *= 2;
  const per: number[] = [];
  for (let i = 0; i < samples; i++) per.push((batch(n) * 1e6) / n);
  per.sort((a, b) => a - b);
  return per[samples >> 1]!;
}

const dir = join(dirname(fileURLToPath(import.meta.url)), "..", "..", "suite", "bench");
const filter = process.argv[2] ?? "";
for (const fn of readdirSync(dir).filter((f) => f.endsWith(".json")).sort()) {
  const f = JSON.parse(readFileSync(join(dir, fn), "utf8")) as BenchFile;
  const env = createEnv({ varRecord: f.env, limits: { steps: f.limits.steps, allocation: f.limits.allocation } });
  for (const c of f.cases) {
    const name = `${fn.replace(/\.json$/, "")}/${c.name}`;
    if (!name.includes(filter)) continue;
    const p = env.compile(parse(c.source));
    const { value, usage } = p.measure(f.vars);
    const got: unknown = JSON.parse(jsonText(value));
    if (!isDeepStrictEqual(got, c.expect.value)) throw new Error(`${name}: value differs from expect: ${jsonText(value).slice(0, 200)}`);
    if (usage.steps !== c.expect.steps || usage.allocated !== c.expect.allocated) {
      throw new Error(`${name}: usage ${JSON.stringify(usage)}, expect steps ${c.expect.steps} allocated ${c.expect.allocated}`);
    }
    const compileNs = median(() => { env.compile(parse(c.source)); });
    const runNs = median(() => { p.run(f.vars); });
    console.log(JSON.stringify({ impl: "ts", program: name, compile_ns: compileNs, run_ns: runNs, steps: usage.steps, allocated: usage.allocated }));
  }
}
