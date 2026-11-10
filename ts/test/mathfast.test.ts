// Tests of the fast phase of package F13 (src/library/mathfast.ts): its
// results against the exact path of mathcr.ts on random and edge arguments,
// its actual error against the bound its rounding test uses, and its tables
// against the exact kernels.
//
// `npm test` runs a short version. RISOR_MATH_LONG=1 runs the long one: a
// million random arguments for each function and every edge set, with the
// error measured on a hundred thousand of them; it prints a summary line for
// each function.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { mathTables } from "../scripts/mathtables.ts";
import {
  crAcos, crAsin, crAtan, crAtan2, crCos, crExp, crLog, crLog10, crPow, crSin, crTan, decompose,
  exactAcos, exactApprox, exactAsin, exactAtan, exactAtan2, exactCos, exactExp, exactLog, exactLog10, exactPow,
  exactSin, exactTan,
} from "../src/library/mathcr.ts";
import {
  clearLastFast, fastAcos, fastAsin, fastAtan, fastAtan2, fastCos, fastExp, fastLog, fastLog10, fastPow, fastSin,
  fastTan, lastFast,
} from "../src/library/mathfast.ts";

const LONG = (process.env.RISOR_MATH_LONG ?? "") !== "" && process.env.RISOR_MATH_LONG !== "0";
const RANDOM = LONG ? 1_000_000 : 400;
const MEASURED = LONG ? 100_000 : 150;
/** The actual error must be at most this fraction of the bound the rounding test uses. */
const MARGIN = 2 ** -8;

// ---- arguments ------------------------------------------------------------------

type Rnd = () => number;

/** A seeded generator of uniform numbers in [0, 1) (mulberry32). */
function seeded(seed: number): Rnd {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** A double of magnitude in [2^lo, 2^hi) with a random 53-bit significand. */
function wide(r: Rnd, lo: number, hi: number): number {
  const e = lo + Math.floor(r() * (hi - lo));
  return (1 + r() + r() * 2 ** -32) * 2 ** e;
}

const sign = (r: Rnd, x: number): number => (r() < 0.5 ? -x : x);

/** The neighbours of x, n doubles each way. */
function around(x: number, n: number): number[] {
  const out: number[] = [];
  const f = new Float64Array(1);
  const b = new BigInt64Array(f.buffer);
  for (let i = -n; i <= n; i++) {
    f[0] = x;
    b[0] += BigInt(i) * (x < 0 ? -1n : 1n);
    if (Number.isFinite(f[0])) out.push(f[0]);
  }
  return out;
}

const one = (xs: number[]): [number, number][] => xs.map((x) => [x, 0]);

const PIO2 = Math.PI / 2;

interface Fn {
  name: string;
  fast: (a: number, b: number) => number;
  exact: (a: number, b: number) => number;
  entry: (a: number, b: number) => number;
  random: (r: Rnd) => [number, number];
  edges: [number, number][];
}

function trigEdges(): [number, number][] {
  const xs: number[] = [];
  for (let k = 1; k <= 200; k++) xs.push(...around(k * PIO2, 2), ...around(-k * PIO2, 1));
  for (const k of [1e3, 1e5, 1e6, 2 ** 20 - 1, 2 ** 20, 2 ** 20 + 1, 1e7, 1e15, 2 ** 52]) xs.push(...around(Math.round(k) * PIO2, 2));
  xs.push(...around(Math.PI / 4, 3), ...around(0.78, 3), ...around(2 ** 20, 3), ...around(2 ** -27, 3), ...around(-(2 ** -27), 2));
  xs.push(1e22, -1e22, 1e300, 2 ** 1023, Number.MAX_VALUE, -Number.MAX_VALUE, 6381956970095103 * 2 ** 797, 5e-324, 1e-310, 1, -1);
  for (let k = 0; k < 64; k++) xs.push(2 ** k, 2 ** (k * 16));
  return one(xs);
}

function logEdges(): [number, number][] {
  const xs: number[] = [Number.MIN_VALUE, 1e-310, 2 ** -1022, Number.MAX_VALUE, 2, 0.5, 10, 0.1];
  for (let k = 1; k <= 300; k++) xs.push(1 + k * 2 ** -52, 1 - k * 2 ** -53);
  for (let n = 10; n <= 52; n++) xs.push(...around(1 + 2 ** -n, 1), ...around(1 - 2 ** -n, 1));
  for (let k = -22; k <= 22; k++) xs.push(10 ** k);
  for (let k = -1074; k <= 1023; k += 7) xs.push(2 ** k);
  for (let i = 0; i <= 512; i += 1) xs.push(...around(1 + (i + 0.5) / 512, 1));
  return one(xs.filter((x) => x > 0));
}

function unitEdges(): [number, number][] {
  const xs: number[] = [0.5, -0.5, Math.SQRT1_2, -Math.SQRT1_2, 5e-324, -5e-324, 1e-310, 2 ** -60, 2 ** -28];
  for (let k = 1; k <= 200; k++) xs.push(1 - k * 2 ** -53, -1 + k * 2 ** -53);
  for (let n = 1; n <= 60; n++) xs.push(...around(2 ** -n, 1), ...around(-(2 ** -n), 1), 1 - 2 ** -n, -1 + 2 ** -n);
  return one(xs.filter((x) => Math.abs(x) <= 1));
}

function expEdges(): [number, number][] {
  const xs: number[] = [1, -1, 0.5, 2 ** -54, -(2 ** -54), 5e-324, -5e-324];
  for (const x of [709.782712893384, 709.7, 709.78, 710, -708, -708.3964185322641, -708.4, -744.4400719213812, -745.1332191019411, -745.2, -746]) xs.push(...around(x, 4));
  for (let k = -740; k <= 709; k += 3) xs.push(k);
  for (let n = 1; n <= 60; n++) xs.push(...around(2 ** -n, 1), ...around(-(2 ** -n), 1));
  return one(xs);
}

function atanEdges(): [number, number][] {
  const xs: number[] = [1, -1, 5e-324, 1e-310, Number.MAX_VALUE, 1e300, -1e300];
  xs.push(...around(1, 4), ...around(2 ** 60, 3), ...around(2 ** -28, 3), ...around(-(2 ** -28), 2));
  for (let j = 0; j <= 256; j++) xs.push(...around((j + 0.5) / 256, 1), ...around(256 / (j + 0.5), 1));
  return one(xs);
}

function atan2Edges(): [number, number][] {
  const v = [0, 1, -1, 2, -3, 1e-300, -1e-300, 1e300, -1e300, 5e-324, -5e-324, Number.MAX_VALUE, 2 ** 500, 2 ** -500, 2 ** -900, 1e-200];
  const out: [number, number][] = [];
  for (const y of v) for (const x of v) out.push([y, x]);
  for (const t of around(1, 3)) out.push([t, 1], [1, t], [-t, -1]);
  return out;
}

function powEdges(): [number, number][] {
  const t = 2 ** 18 - 1;
  const out: [number, number][] = [
    [2, -1075], [2, -1074], [2, 1023], [2, 1024], [10, 22], [10, 23], [10, -2], [10, 308], [10, 309], [10, -323], [10, -324],
    [1.5, 2], [4, 0.5], [-3, 3], [-3, 4], [-8, 3], [2, 0.5], [1.1, 10], [3, 40], [t * t, 1.5], [9, 0.5], [0.25, 0.5],
    [-1, 3], [-1, 4], [1, 1e300], [5e-324, 0.5], [5e-324, -0.5], [5e-324, 1], [Number.MAX_VALUE, 0.5], [Number.MAX_VALUE, 1],
    [1 + 2 ** -52, 2 ** 60], [1 - 2 ** -53, 2 ** 62], [2, 1e-300], [2, 5e-324], [0.5, 1074], [0.5, 1075], [7, 1 / 3],
  ];
  for (let b = 2; b <= 20; b++) for (let e = -40; e <= 40; e++) if (e !== 0) out.push([b, e], [-b, e]);
  for (let b = 2; b <= 40; b++) for (const e of [0.5, 1.5, 2.5, 0.25, 0.75, -0.5, -1.5, 0.125]) out.push([b * b, e], [b, e]);
  for (const y of [1, -1, 2, 0.5]) for (const x of around(2 ** (709.78 / y), 3)) out.push([x, y]);
  return out;
}

const randomTrig = (r: Rnd): [number, number] => {
  const u = r();
  if (u < 0.6) return [r() * 20 - 10, 0];
  if (u < 0.9) return [sign(r, wide(r, -30, 30)), 0];
  return [sign(r, wide(r, -1074, 1024)), 0];
};

const FNS: Fn[] = [
  {
    name: "exp", fast: (a) => fastExp(a), exact: (a) => exactExp(a), entry: (a) => crExp(a),
    random: (r) => (r() < 0.7 ? [r() * 1460 - 750, 0] : [sign(r, wide(r, -80, 9)), 0]), edges: expEdges(),
  },
  {
    name: "log", fast: (a) => fastLog(a), exact: (a) => exactLog(a), entry: (a) => crLog(a),
    random: (r) => (r() < 0.8 ? [wide(r, -1074, 1024), 0] : [1 + (r() - 0.5) * 2 ** -Math.floor(r() * 40), 0]), edges: logEdges(),
  },
  {
    name: "log10", fast: (a) => fastLog10(a), exact: (a) => exactLog10(a), entry: (a) => crLog10(a),
    random: (r) => (r() < 0.8 ? [wide(r, -1074, 1024), 0] : [1 + (r() - 0.5) * 2 ** -Math.floor(r() * 40), 0]), edges: logEdges(),
  },
  { name: "sin", fast: (a) => fastSin(a), exact: (a) => exactSin(a), entry: (a) => crSin(a), random: randomTrig, edges: trigEdges() },
  { name: "cos", fast: (a) => fastCos(a), exact: (a) => exactCos(a), entry: (a) => crCos(a), random: randomTrig, edges: trigEdges() },
  { name: "tan", fast: (a) => fastTan(a), exact: (a) => exactTan(a), entry: (a) => crTan(a), random: randomTrig, edges: trigEdges() },
  {
    name: "asin", fast: (a) => fastAsin(a), exact: (a) => exactAsin(a), entry: (a) => crAsin(a),
    random: (r) => (r() < 0.7 ? [r() * 2 - 1, 0] : [sign(r, r() < 0.5 ? wide(r, -60, 0) : 1 - wide(r, -53, -1)), 0]), edges: unitEdges(),
  },
  {
    name: "acos", fast: (a) => fastAcos(a), exact: (a) => exactAcos(a), entry: (a) => crAcos(a),
    random: (r) => (r() < 0.7 ? [r() * 2 - 1, 0] : [sign(r, r() < 0.5 ? wide(r, -70, 0) : 1 - wide(r, -53, -1)), 0]), edges: unitEdges(),
  },
  {
    name: "atan", fast: (a) => fastAtan(a), exact: (a) => exactAtan(a), entry: (a) => crAtan(a),
    random: (r) => (r() < 0.6 ? [r() * 20 - 10, 0] : [sign(r, wide(r, -1074, 1024)), 0]), edges: atanEdges(),
  },
  {
    name: "atan2", fast: (a, b) => fastAtan2(a, b), exact: (a, b) => exactAtan2(a, b), entry: (a, b) => crAtan2(a, b),
    random: (r) => (r() < 0.6 ? [r() * 20 - 10, r() * 20 - 10] : [sign(r, wide(r, -1074, 1024)), sign(r, wide(r, -1074, 1024))]),
    edges: atan2Edges(),
  },
  {
    name: "pow", fast: (a, b) => fastPow(a, b), exact: (a, b) => exactPow(a, b), entry: (a, b) => crPow(a, b),
    random: (r) => {
      const u = r();
      if (u < 0.5) return [r() * 10, r() * 40 - 20];
      if (u < 0.7) return [sign(r, Math.floor(r() * 100) + 2), Math.floor(r() * 80) - 40];
      const x = wide(r, -1074, 1024);
      // y with |y log2 x| up to 1200, so that results reach both ends of the range.
      return [x, ((r() * 2 - 1) * 1200) / Math.abs(Math.log2(x) || 1)];
    },
    edges: powEdges(),
  },
];

/** The domain of the exact path (math.ts tests not-real and the special cases of pow first). */
function inDomain(name: string, a: number, b: number): boolean {
  if (!Number.isFinite(a) || !Number.isFinite(b)) return false;
  switch (name) {
    case "log":
    case "log10":
      return a > 0;
    case "asin":
    case "acos":
      return a >= -1 && a <= 1;
    case "pow":
      return a !== 0 && b !== 0 && (a > 0 || Number.isInteger(b));
  }
  return true;
}

// ---- the error measurement ----------------------------------------------------

/**
 * The actual error of the last fast result over the bound its rounding test
 * used, or null when the call made no rounding test (a special value or an
 * argument the function leaves to the exact path).
 */
function errorRatio(name: string, a: number, b: number): number | null {
  const last = lastFast();
  if (last === null || last.err <= 0) return null;
  const approx = exactApprox(name, a, b, 160);
  // |h + l| 2^k and |m| 2^e on a common grid 2^g.
  const parts: [bigint, number][] = [];
  for (const v of [last.h, last.l]) {
    if (v === 0) continue;
    const d = decompose(v);
    parts.push([v < 0 ? -d.m : d.m, d.q + last.k]);
  }
  const g = Math.min(approx.e, ...parts.map((p) => p[1]));
  let fast = 0n;
  for (const [m, q] of parts) fast += m << BigInt(q - g);
  const abs = (n: bigint) => (n < 0n ? -n : n);
  const exact = abs(approx.m) << BigInt(approx.e - g);
  const diff = abs(abs(fast) - exact) + (approx.err << BigInt(approx.e - g));
  // diff 2^g / (err 2^k), by logarithms: the grid can be far from 1.
  if (diff === 0n) return 0;
  const len = diff.toString(2).length;
  const shift = Math.max(0, len - 60);
  return 2 ** (Math.log2(Number(diff >> BigInt(shift))) + shift + g - last.k - Math.log2(last.err));
}

// ---- the tests ----------------------------------------------------------------

describe("the fast phase of the math functions (f13 2.1)", () => {
  test("the tables are current with the exact kernels (node scripts/mathtables.ts)", () => {
    const file = join(dirname(fileURLToPath(import.meta.url)), "..", "src", "library", "mathtables.ts");
    assert.equal(readFileSync(file, "utf8"), mathTables());
  });

  for (const fn of FNS) {
    test(`${fn.name}: fast results equal the exact path's, and errors are within the bound`, () => {
      const r = seeded(fn.name.length * 7919 + fn.name.charCodeAt(0));
      const args: [number, number][] = [...fn.edges];
      for (let i = 0; i < RANDOM; i++) args.push(fn.random(r));
      let calls = 0;
      let fallbacks = 0;
      let edgeFallbacks = 0;
      let measured = 0;
      let worst = 0;
      const start = performance.now();
      for (let i = 0; i < args.length; i++) {
        const [a, b] = args[i];
        if (!inDomain(fn.name, a, b)) continue;
        calls++;
        clearLastFast();
        const got = fn.fast(a, b);
        const measure = i < fn.edges.length || i % Math.max(1, Math.floor(RANDOM / MEASURED)) === 0;
        if (measure) {
          const ratio = errorRatio(fn.name, a, b);
          if (ratio !== null) {
            measured++;
            if (ratio > worst) worst = ratio;
            assert.ok(ratio <= MARGIN, `${fn.name}(${a}, ${b}): error ${ratio} of the bound`);
          }
        }
        if (got !== got) {
          fallbacks++;
          if (i < fn.edges.length) edgeFallbacks++;
          continue;
        }
        const want = fn.exact(a, b);
        if (got !== want) assert.fail(`${fn.name}(${a}, ${b}): fast ${got}, exact ${want}`);
        // The entry gives the same.
        assert.equal(fn.entry(a, b), want);
      }
      if (LONG) {
        const secs = ((performance.now() - start) / 1000).toFixed(1);
        console.log(
          JSON.stringify({
            fn: fn.name, calls, edges: fn.edges.length, fallbacks, edgeFallbacks, randomFallbackRate: (fallbacks - edgeFallbacks) / (calls - fn.edges.length),
            measured, worstErrorOverBound: worst, secs,
          }),
        );
      }
    });
  }
});
