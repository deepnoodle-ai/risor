// The time of a call of each correctly rounded function of package F13:
// `node scripts/bench-math.ts [--exact]`, from ts/.
//
// For each function a fixed set of 1000 typical arguments (a seeded generator)
// is timed in batches; a sample is one pass over the set, divided by its size,
// and the median of 31 samples is printed, after a warm-up. `fallback` is the
// fraction of 100000 typical calls that the fast phase left to the exact path.
// With --exact the exact path alone is timed too, for comparison.

import {
  crAcos, crAsin, crAtan, crAtan2, crCos, crExp, crLog, crLog10, crPow, crSin, crTan,
  exactAcos, exactAsin, exactAtan, exactAtan2, exactCos, exactExp, exactLog, exactLog10, exactPow, exactSin, exactTan,
  fallbacks,
} from "../src/library/mathcr.ts";

let seed = 12345;
function rnd(): number {
  seed = (seed * 1103515245 + 12345) % 2147483648;
  return seed / 2147483648;
}

type Gen = () => [number, number];
type Loop = (a: Float64Array, b: Float64Array) => number;

// One loop for each function, so that each call site sees one function.
const sets: [string, Loop, Loop, Gen][] = [
  ["exp", (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += crExp(a[i]); return s; },
    (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactExp(a[i]); return s; }, () => [rnd() * 40 - 20, 0]],
  ["log", (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += crLog(a[i]); return s; },
    (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactLog(a[i]); return s; }, () => [10 ** (rnd() * 20 - 10), 0]],
  ["log10", (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += crLog10(a[i]); return s; },
    (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactLog10(a[i]); return s; }, () => [10 ** (rnd() * 20 - 10), 0]],
  ["sin", (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += crSin(a[i]); return s; },
    (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactSin(a[i]); return s; }, () => [rnd() * 20 - 10, 0]],
  ["cos", (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += crCos(a[i]); return s; },
    (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactCos(a[i]); return s; }, () => [rnd() * 20 - 10, 0]],
  ["tan", (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += crTan(a[i]); return s; },
    (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactTan(a[i]); return s; }, () => [rnd() * 20 - 10, 0]],
  ["asin", (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += crAsin(a[i]); return s; },
    (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactAsin(a[i]); return s; }, () => [rnd() * 2 - 1, 0]],
  ["acos", (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += crAcos(a[i]); return s; },
    (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactAcos(a[i]); return s; }, () => [rnd() * 2 - 1, 0]],
  ["atan", (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += crAtan(a[i]); return s; },
    (a) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactAtan(a[i]); return s; }, () => [rnd() * 20 - 10, 0]],
  ["atan2", (a, b) => { let s = 0; for (let i = 0; i < a.length; i++) s += crAtan2(a[i], b[i]); return s; },
    (a, b) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactAtan2(a[i], b[i]); return s; }, () => [rnd() * 20 - 10, rnd() * 20 - 10]],
  ["pow", (a, b) => { let s = 0; for (let i = 0; i < a.length; i++) s += crPow(a[i], b[i]); return s; },
    (a, b) => { let s = 0; for (let i = 0; i < a.length; i++) s += exactPow(a[i], b[i]); return s; }, () => [rnd() * 10, rnd() * 20 - 10]],
];

function args(gen: Gen, n: number): [Float64Array, Float64Array] {
  const a = new Float64Array(n);
  const b = new Float64Array(n);
  for (let i = 0; i < n; i++) [a[i], b[i]] = gen();
  return [a, b];
}

/** The median time of a call, in ns: samples of one pass over the arguments. */
function median(loop: Loop, a: Float64Array, b: Float64Array, samples: number): number {
  let sink = 0;
  for (let w = 0; w < 20; w++) sink += loop(a, b);
  const times: number[] = [];
  for (let s = 0; s < samples; s++) {
    const t = performance.now();
    sink += loop(a, b);
    times.push(((performance.now() - t) * 1e6) / a.length);
  }
  if (sink === 0.5) console.log("");
  times.sort((x, y) => x - y);
  return times[samples >> 1];
}

const exact = process.argv.includes("--exact");
for (const [name, fast, slow, gen] of sets) {
  const [a, b] = args(gen, 1000);
  const ns = median(fast, a, b, 31);
  fallbacks.count = 0;
  const [ma, mb] = args(gen, 100000);
  fast(ma, mb);
  const row: Record<string, unknown> = { fn: name, ns: Math.round(ns), fallback: fallbacks.count / ma.length };
  if (exact) row.exact_ns = Math.round(median(slow, a.subarray(0, 200), b.subarray(0, 200), 5));
  console.log(JSON.stringify(row));
}
