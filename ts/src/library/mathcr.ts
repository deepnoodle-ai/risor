// The correctly rounded functions of package F13 (f13 2.1): of the exact value
// of the function at its arguments, the nearest double, ties to even.
//
// The method is the chapter's to leave open (f13 2.1 item 5). Each entry first
// tries the fast phase of mathfast.ts, which decides nearly every call in
// double-double arithmetic. The calls it leaves are decided here, by the exact
// path, which is also the oracle of the fast phase's tests.
//
// In the exact path every value is computed in binary fixed point on BigInt,
// at a working precision p, with a bound on its error. When both ends of the
// interval round to the same double, that double is the result; otherwise p
// grows and the value is computed again. No platform function gives a result:
// `Math` is used only for estimates that pick a reduction or a precision,
// never for a digit of a result.
//
// An exact value that is a double stops the search, since both ends of a small
// interval around it round to it. An exact value that is half way between two
// doubles never does: it occurs only in `pow`, whose exact cases are computed
// as rationals (powExact) or tested exactly (powMidpoint).

import { fastAcos, fastAsin, fastAtan, fastAtan2, fastCos, fastExp, fastLog, fastLog10, fastPow, fastSin, fastTan } from "./mathfast.ts";

/** A value m * 2^e with an error of at most err * 2^e. */
export interface Approx {
  m: bigint;
  e: number;
  err: bigint;
}

/** The precisions tried, in bits beyond the guard bits of each function. */
const PRECISIONS = [64, 160, 400, 1000, 2500, 6000];

// ---- doubles ------------------------------------------------------------------

const view = new DataView(new ArrayBuffer(8));

/** A finite nonzero double as an odd or even integer m and an exponent q: |x| = m * 2^q. */
export function decompose(x: number): { m: bigint; q: number } {
  view.setFloat64(0, Math.abs(x));
  const hi = view.getUint32(0);
  const lo = view.getUint32(4);
  const be = (hi >>> 20) & 0x7ff;
  const frac = (BigInt(hi & 0xfffff) << 32n) | BigInt(lo);
  if (be === 0) return { m: frac, q: -1074 };
  return { m: frac | (1n << 52n), q: be - 1075 };
}

/** The exponent of a finite nonzero double: floor(log2 |x|). */
function exponent(x: number): number {
  const { m, q } = decompose(x);
  return q + bitLength(m) - 1;
}

export function bitLength(n: bigint): number {
  if (n < 0n) n = -n;
  return n === 0n ? 0 : n.toString(2).length;
}

/** 2^k as a double, for any k with a finite nonzero result, or the steps of a scaling. */
function scale(x: number, k: number): number {
  while (k > 600) {
    x *= 2 ** 600;
    k -= 600;
  }
  while (k < -600) {
    x *= 2 ** -600;
    k += 600;
  }
  return x * 2 ** k;
}

/** m * 2^e rounded to the nearest double, ties to even, subnormals on their grid; ±Infinity past the largest. */
export function roundExact(m: bigint, e: number): number {
  if (m === 0n) return 0;
  const neg = m < 0n;
  if (neg) m = -m;
  const len = bitLength(m);
  const top = e + len - 1; // floor(log2 value)
  // The bits the result keeps: 53, fewer below 2^-1022.
  let bits = 53;
  if (top < -1022) bits = 53 - (-1022 - top);
  let q: bigint;
  let qe: number;
  if (bits <= 0) {
    // Below half the least subnormal, or at most its half: round on the subnormal grid.
    const shift = -1074 - e; // to the grid of 2^-1074
    if (shift > len + 1) return neg ? -0 + 0 : 0;
    const s = BigInt(shift);
    q = m >> s;
    const rem = m - (q << s);
    const half = 1n << (s - 1n);
    if (rem > half || (rem === half && (q & 1n) === 1n)) q += 1n;
    qe = -1074;
  } else {
    const shift = len - bits;
    if (shift <= 0) {
      q = m;
      qe = e;
    } else {
      const s = BigInt(shift);
      q = m >> s;
      const rem = m - (q << s);
      const half = 1n << (s - 1n);
      if (rem > half || (rem === half && (q & 1n) === 1n)) q += 1n;
      qe = e + shift;
    }
  }
  if (q === 0n) return 0;
  if (qe + bitLength(q) - 1 >= 1024) return neg ? -Infinity : Infinity;
  const r = scale(Number(q), qe);
  return neg ? -r : r;
}

/** The double the approximation rounds to, or null when its interval rounds to two doubles. */
function decide(a: Approx): number | null {
  const lo = roundExact(a.m - a.err, a.e);
  const hi = roundExact(a.m + a.err, a.e);
  if (lo !== hi) return null;
  return lo === 0 ? 0 : lo;
}

/** Try each precision until the result is decided. */
function ziv(f: (p: number) => Approx | number): number {
  for (const p of PRECISIONS) {
    const a = f(p);
    if (typeof a === "number") return a;
    const d = decide(a);
    if (d !== null) return d;
  }
  throw new Error("the correctly rounded result was not decided");
}

// ---- fixed point --------------------------------------------------------------

/** x as a fixed-point number of scale S (x * 2^S), truncated toward zero. */
export function fixed(x: number, S: number): bigint {
  if (x === 0) return 0n;
  const { m, q } = decompose(x);
  const k = q + S;
  const v = k >= 0 ? m << BigInt(k) : m >> BigInt(-k);
  return x < 0 ? -v : v;
}

/** The least S at which every bit of x is kept by `fixed`. */
function exactScale(x: number): number {
  const { m, q } = decompose(x);
  let tz = 0;
  let n = m;
  while (n > 0n && (n & 1n) === 0n) {
    n >>= 1n;
    tz++;
  }
  return Math.max(0, -(q + tz));
}

function isqrt(n: bigint): bigint {
  if (n < 2n) return n;
  let x = 1n << BigInt(Math.ceil(bitLength(n) / 2));
  for (;;) {
    const y = (x + n / x) >> 1n;
    if (y >= x) return x;
    x = y;
  }
}

const abs = (n: bigint) => (n < 0n ? -n : n);

/** atanh(1/n) at scale S, by its series: sum of 1 / ((2i+1) n^(2i+1)). Error at most a few units. */
function atanhInv(n: bigint, S: number): bigint {
  const one = 1n << BigInt(S);
  let term = one / n;
  const n2 = n * n;
  let sum = 0n;
  for (let k = 1n; term !== 0n; k += 2n) {
    sum += term / k;
    term /= n2;
  }
  return sum;
}

/** atan(1/n) at scale S, by its series. */
function atanInv(n: bigint, S: number): bigint {
  const one = 1n << BigInt(S);
  let term = one / n;
  const n2 = n * n;
  let sum = 0n;
  let sign = 1n;
  for (let k = 1n; term !== 0n; k += 2n) {
    sum += (sign * term) / k;
    term /= n2;
    sign = -sign;
  }
  return sum;
}

const LN2 = new Map<number, bigint>();
const PI = new Map<number, bigint>();

/** ln 2 at scale S, with an error of at most 1. */
export function ln2(S: number): bigint {
  let v = LN2.get(S);
  if (v === undefined) {
    // ln 2 = 2 atanh(1/3), computed with 16 guard bits.
    v = (2n * atanhInv(3n, S + 16)) >> 16n;
    LN2.set(S, v);
  }
  return v;
}

/** pi at scale S, with an error of at most 1. */
export function pi(S: number): bigint {
  let v = PI.get(S);
  if (v === undefined) {
    // Machin: pi = 16 atan(1/5) - 4 atan(1/239).
    v = (16n * atanInv(5n, S + 16) - 4n * atanInv(239n, S + 16)) >> 16n;
    PI.set(S, v);
  }
  return v;
}

// ---- the cores ----------------------------------------------------------------

/**
 * exp(T * 2^-S) for a fixed-point T with an error of at most errT: the value
 * E * 2^(k - S), and its error in units of 2^(k - S).
 */
export function expFixed(T: bigint, S: number, errT: bigint): Approx {
  const one = 1n << BigInt(S);
  const L = ln2(S);
  // k = round(T / ln2), then R = T - k ln2, |R| <= ln2 / 2 or a little more.
  let k = (2n * T + L) / (2n * L);
  if (2n * T + L < 0n && (2n * T + L) % (2n * L) !== 0n) k -= 1n;
  const R = T - k * L;
  const errR = errT + abs(k) + 1n;
  // exp(R) = exp(R / 2^8)^(2^8).
  const H = 8;
  const r = R >> BigInt(H);
  let term = one;
  let sum = one;
  let terms = 0n;
  for (let i = 1n; term !== 0n; i++) {
    term = (term * r) / (i << BigInt(S));
    sum += term;
    terms++;
  }
  let E = sum;
  for (let i = 0; i < H; i++) E = (E * E) >> BigInt(S);
  // The error: that of r (errR / 2^8 + 1) and of the series, grown 2^8 times
  // by the squarings, which add a unit each; exp(R) < 2.
  const err = ((errR >> BigInt(H)) + 2n + terms) * 4n * (1n << BigInt(H)) + 4n * BigInt(H);
  return { m: E, e: Number(k) - S, err };
}

/** ln x for a double x > 0, at scale S: the fixed-point value and its error. */
export function logFixed(x: number, S: number): { v: bigint; err: bigint } {
  const one = 1n << BigInt(S);
  const { m, q } = decompose(x);
  // x = f * 2^n with f in [0.75, 1.5).
  let n = q + bitLength(m) - 1;
  let F = q - n + S >= 0 ? m << BigInt(q - n + S) : m >> BigInt(n - q - S);
  if (F >= one + (one >> 1n)) {
    n += 1;
    F >>= 1n;
  }
  // ln f = 2 atanh(z), z = (f - 1) / (f + 1), |z| <= 0.2.
  const z = ((F - one) << BigInt(S)) / (F + one);
  const z2 = (z * z) >> BigInt(S);
  let term = z;
  let sum = 0n;
  let terms = 0n;
  for (let k = 1n; term !== 0n; k += 2n) {
    sum += term / k;
    // Division truncates toward zero, so a negative term also reaches zero.
    term = (term * z2) / one;
    terms++;
  }
  const v = 2n * sum + BigInt(n) * ln2(S);
  return { v, err: 2n * (terms + 4n) + BigInt(Math.abs(n)) + 2n };
}

/** atan(N / D) for N, D >= 0, not both zero, at scale S: the value and its error. */
export function atanRatio(N: bigint, D: bigint, S: number): { v: bigint; err: bigint } {
  const one = 1n << BigInt(S);
  if (N === 0n) return { v: 0n, err: 0n };
  const big = N > D;
  let t = big ? (D << BigInt(S)) / N : (N << BigInt(S)) / D;
  // atan t = 2 atan(t / (1 + sqrt(1 + t^2))), three times: t <= tan(pi/32).
  const H = 3;
  for (let i = 0; i < H; i++) {
    const root = isqrt((one << BigInt(S)) + t * t);
    t = (t << BigInt(S)) / (one + root);
  }
  const t2 = (t * t) >> BigInt(S);
  let term = t;
  let sum = 0n;
  let terms = 0n;
  let sign = 1n;
  for (let k = 1n; term !== 0n; k += 2n) {
    sum += (sign * term) / k;
    term = (term * t2) >> BigInt(S);
    sign = -sign;
    terms++;
  }
  let v = sum << BigInt(H);
  let err = (terms + 4n * BigInt(H) + 4n) << BigInt(H);
  if (big) {
    v = (pi(S) >> 1n) - v;
    err += 2n;
  }
  return { v, err };
}

/** sin and cos of x, reduced by multiples of pi/2, at scale S: the two values, their error, and k mod 4. */
export function sinCos(x: number, S: number): { s: bigint; c: bigint; err: bigint; quadrant: number } {
  const one = 1n << BigInt(S);
  const X = fixed(x, S);
  // pi/2 at a scale with room for k * (pi/2) to stay within a unit of scale S.
  const ex = Math.max(0, exponent(x));
  const extra = ex + 8;
  const S2 = S + extra;
  const half = pi(S2) >> 1n;
  const X2 = X << BigInt(extra);
  let k = (2n * X2 + half) / (2n * half);
  if (2n * X2 + half < 0n && (2n * X2 + half) % (2n * half) !== 0n) k -= 1n;
  const R = (X2 - k * half) >> BigInt(extra);
  // sin and cos of R, |R| <= pi/4 or a little more, by their series.
  const R2 = (R * R) >> BigInt(S);
  let s = 0n;
  let term = R;
  let terms = 0n;
  for (let i = 1n; term !== 0n; i += 2n) {
    s += term;
    term = -(term * R2) / ((i + 1n) * (i + 2n) << BigInt(S));
    terms++;
  }
  let c = 0n;
  term = one;
  for (let i = 0n; term !== 0n; i += 2n) {
    c += term;
    term = -(term * R2) / ((i + 1n) * (i + 2n) << BigInt(S));
    terms++;
  }
  const quadrant = Number(((k % 4n) + 4n) % 4n);
  return { s, c, err: terms + 8n, quadrant };
}

/** N * 2^S / D with N, D carrying errors eN, eD: the quotient and a bound on its error. */
function divide(N: bigint, eN: bigint, D: bigint, eD: bigint, S: number): { v: bigint; err: bigint } | null {
  const aD = abs(D);
  if (aD <= 2n * eD) return null;
  const v = (N << BigInt(S)) / D;
  // |N/D - n/d| <= (eN + |N/D| eD) / (|D| - eD), in units of 2^-S.
  const err = ((eN << BigInt(S)) + abs(v) * eD) / (aD - eD) + 2n;
  return { v, err };
}

// ---- the entries --------------------------------------------------------------
//
// Each entry first tries the fast phase of mathfast.ts, which gives its result
// in double-double arithmetic with a proven error bound and returns NaN when
// that interval does not decide the rounding. Only then is the exact search
// below run. The exact path is also the oracle of the tests of the fast phase.

/** The guard bits beyond p at which each function works. */
const GUARD = 64;

/** The number of calls that the fast phase did not decide (for the tests and the bench). */
export const fallbacks = { count: 0 };

export function crExp(x: number): number {
  const r = fastExp(x);
  if (r === r) return r;
  fallbacks.count++;
  return exactExp(x);
}
export function crLog(x: number): number {
  const r = fastLog(x);
  if (r === r) return r;
  fallbacks.count++;
  return exactLog(x);
}
export function crLog10(x: number): number {
  const r = fastLog10(x);
  if (r === r) return r;
  fallbacks.count++;
  return exactLog10(x);
}
export function crSin(x: number): number {
  const r = fastSin(x);
  if (r === r) return r;
  fallbacks.count++;
  return exactSin(x);
}
export function crCos(x: number): number {
  const r = fastCos(x);
  if (r === r) return r;
  fallbacks.count++;
  return exactCos(x);
}
export function crTan(x: number): number {
  const r = fastTan(x);
  if (r === r) return r;
  fallbacks.count++;
  return exactTan(x);
}
export function crAtan(x: number): number {
  const r = fastAtan(x);
  if (r === r) return r;
  fallbacks.count++;
  return exactAtan(x);
}
export function crAtan2(y: number, x: number): number {
  const r = fastAtan2(y, x);
  if (r === r) return r;
  fallbacks.count++;
  return exactAtan2(y, x);
}
export function crAsin(x: number): number {
  const r = fastAsin(x);
  if (r === r) return r;
  fallbacks.count++;
  return exactAsin(x);
}
export function crAcos(x: number): number {
  const r = fastAcos(x);
  if (r === r) return r;
  fallbacks.count++;
  return exactAcos(x);
}
export function crPow(x: number, y: number): number {
  const r = fastPow(x, y);
  if (r === r) return r;
  fallbacks.count++;
  return exactPow(x, y);
}

// ---- the exact path -----------------------------------------------------------

export function exactExp(x: number): number {
  if (x === 0) return 1;
  if (x > 710) return Infinity;
  if (x < -746) return 0;
  return ziv((p) => expAt(x, p));
}

function expAt(x: number, p: number): Approx {
  const S = p + GUARD;
  return expFixed(fixed(x, S), S, 1n);
}

export function exactLog(x: number): number {
  if (x === 1) return 0;
  return ziv((p) => logAt(x, p));
}

function logAt(x: number, p: number): Approx {
  // ln x is at least 2^-54 in magnitude for a double x other than 1.
  const S = p + GUARD + 60;
  const r = logFixed(x, S);
  return { m: r.v, e: -S, err: r.err };
}

export function exactLog10(x: number): number {
  if (x === 1) return 0;
  return ziv((p) => log10At(x, p));
}

function log10At(x: number, p: number): Approx {
  const S = p + GUARD + 60;
  const a = logFixed(x, S);
  const b = logFixed(10, S);
  const d = divide(a.v, a.err, b.v, b.err, S);
  return d === null ? { m: 0n, e: 0, err: 1n << 4000n } : { m: d.v, e: -S, err: d.err };
}

export function exactSin(x: number): number {
  if (x === 0) return 0;
  return ziv((p) => sinAt(x, p));
}

function sinAt(x: number, p: number): Approx {
  const S = p + GUARD + Math.max(0, -exponent(x)) + exactScaleMargin(x);
  const r = sinCos(x, S);
  const v = [r.s, r.c, -r.s, -r.c][r.quadrant];
  return { m: v, e: -S, err: r.err };
}

export function exactCos(x: number): number {
  if (x === 0) return 1;
  return ziv((p) => cosAt(x, p));
}

function cosAt(x: number, p: number): Approx {
  const S = p + GUARD + exactScaleMargin(x);
  const r = sinCos(x, S);
  const v = [r.c, -r.s, -r.c, r.s][r.quadrant];
  return { m: v, e: -S, err: r.err };
}

export function exactTan(x: number): number {
  if (x === 0) return 0;
  return ziv((p) => tanAt(x, p));
}

function tanAt(x: number, p: number): Approx {
  const S = p + GUARD + Math.max(0, -exponent(x)) + exactScaleMargin(x);
  const r = sinCos(x, S);
  const odd = r.quadrant % 2 === 1;
  const d = odd ? divide(-r.c, r.err, r.s, r.err, S) : divide(r.s, r.err, r.c, r.err, S);
  return d === null ? { m: 0n, e: 0, err: 1n << 4000n } : { m: d.v, e: -S, err: d.err };
}

/** Room for every bit of x in its fixed-point form at scale S. */
function exactScaleMargin(x: number): number {
  return Math.max(0, exactScale(x) - GUARD);
}

export function exactAtan(x: number): number {
  if (x === 0) return 0;
  return ziv((p) => atanAt(x, p));
}

function atanAt(x: number, p: number): Approx {
  const S = Math.max(p + GUARD + Math.max(0, -exponent(x)), exactScale(x));
  const { m, q } = decompose(x);
  // |x| = m * 2^q as a ratio of integers.
  const N = q >= 0 ? m << BigInt(q) : m;
  const D = q >= 0 ? 1n : 1n << BigInt(-q);
  const r = atanRatio(N, D, S);
  return { m: x < 0 ? -r.v : r.v, e: -S, err: r.err };
}

export function exactAtan2(y: number, x: number): number {
  if (y === 0) return x < 0 ? crPi() : 0;
  if (x === 0) return y > 0 ? crHalfPi() : -crHalfPi();
  return ziv((p) => atan2At(y, x, p));
}

function atan2At(y: number, x: number, p: number): Approx {
  const ey = exponent(y);
  const ex = exponent(x);
  const S = p + GUARD + Math.max(0, ex - ey);
  const a = decompose(y);
  const b = decompose(x);
  const lo = Math.min(a.q, b.q);
  const N = a.m << BigInt(a.q - lo);
  const D = b.m << BigInt(b.q - lo);
  const r = atanRatio(N, D, S);
  let v = x > 0 ? r.v : pi(S) - r.v;
  const err = r.err + 2n;
  if (y < 0) v = -v;
  return { m: v, e: -S, err };
}

function crPi(): number {
  return ziv((p) => ({ m: pi(p + GUARD), e: -(p + GUARD), err: 1n }));
}

function crHalfPi(): number {
  return ziv((p) => ({ m: pi(p + GUARD), e: -(p + GUARD) - 1, err: 1n }));
}

/** asin for -1 <= x <= 1. */
export function exactAsin(x: number): number {
  if (x === 0) return 0;
  if (x === 1) return crHalfPi();
  if (x === -1) return -crHalfPi();
  return ziv((p) => asinAt(x, p));
}

function asinAt(x: number, p: number): Approx {
  const S = Math.max(p + GUARD + Math.max(0, -exponent(x)) + 30, exactScale(x) + 1);
  const one = 1n << BigInt(S);
  const X = abs(fixed(x, S));
  // asin x = atan(x / sqrt(1 - x^2)).
  const D = isqrt((one << BigInt(S)) - X * X);
  const r = atanRatio(X, D, S);
  // D carries an error of a unit, at least 2^-27 of one: a relative error of 2^(27-S).
  const err = r.err + 4n + ((r.v >> BigInt(S - 30)) + 1n);
  return { m: x < 0 ? -r.v : r.v, e: -S, err };
}

/** acos for -1 <= x <= 1. */
export function exactAcos(x: number): number {
  if (x === 1) return 0;
  if (x === -1) return crPi();
  if (x === 0) return crHalfPi();
  return ziv((p) => acosAt(x, p));
}

function acosAt(x: number, p: number): Approx {
  const S = Math.max(p + GUARD + 60, exactScale(x) + 1);
  const one = 1n << BigInt(S);
  const X = abs(fixed(x, S));
  // acos x = atan(sqrt(1 - x^2) / x) for x > 0, and pi less that for x < 0.
  const D = isqrt((one << BigInt(S)) - X * X);
  const r = atanRatio(D, X, S);
  const v = x > 0 ? r.v : pi(S) - r.v;
  const err = r.err + 4n + ((one >> BigInt(S - 30)) + 1n);
  return { m: v, e: -S, err };
}

/**
 * For the tests: the exact path's approximation of an entry at precision p,
 * the value m * 2^e with an error of at most err * 2^e. The arguments are
 * those of the general case of the entry (no special value).
 */
export function exactApprox(name: string, x: number, y: number, p: number): Approx {
  switch (name) {
    case "exp": return expAt(x, p);
    case "log": return logAt(x, p);
    case "log10": return log10At(x, p);
    case "sin": return sinAt(x, p);
    case "cos": return cosAt(x, p);
    case "tan": return tanAt(x, p);
    case "atan": return atanAt(x, p);
    case "atan2": return atan2At(x, y, p);
    case "asin": return asinAt(x, p);
    case "acos": return acosAt(x, p);
    case "pow": {
      const a = powApprox(Math.abs(x), y, p);
      return x < 0 && Math.abs(y % 2) === 1 ? { m: -a.m, e: a.e, err: a.err } : a;
    }
  }
  throw new Error(`no entry ${name}`);
}

// ---- pow ----------------------------------------------------------------------

/** x^y for x != 0, y != 0, and y an integer when x < 0 (f13 4.1 items 1 to 4). */
export function exactPow(x: number, y: number): number {
  const neg = x < 0 && Number.isInteger(y) && Math.abs(y % 2) === 1;
  const ax = Math.abs(x);
  if (ax === 1) return neg ? -1 : 1;
  const { m, q } = decompose(ax);
  let odd = m;
  let oq = q;
  while ((odd & 1n) === 0n) {
    odd >>= 1n;
    oq++;
  }
  // An estimate of log2 of the result picks overflow and underflow far from the edges.
  const est = y * Math.log2(ax);
  if (est > 1100) return neg ? -Infinity : Infinity;
  if (est < -1200) return 0;
  // Exact: a power of two to an integer, or an integer power of moderate size.
  if (Number.isInteger(y) && (odd === 1n || Math.abs(y) <= 64)) {
    const r = powExact(odd, oq, y);
    return neg ? -r : r;
  }
  let last: Approx | null = null;
  for (const p of PRECISIONS) {
    const a = powApprox(ax, y, p);
    last = a;
    const d = decide(a);
    if (d !== null) return neg ? -d : d === 0 ? 0 : d;
    const mid = powMidpoint(a, odd, oq, y);
    if (mid !== null) return neg ? -mid : mid;
  }
  throw new Error(`math.pow was not decided at ${String(last?.e)}`);
}

/** (odd * 2^oq)^y exactly rounded, y an integer. */
function powExact(odd: bigint, oq: number, y: number): number {
  if (y > 0) return roundExact(odd ** BigInt(y), oq * y);
  const n = BigInt(-y);
  const den = odd ** n;
  if (den === 1n) return roundExact(1n, oq * y);
  // 2^(oq y) / den: not a dyadic number, so never a double nor half way between two.
  for (let extra = 64; ; extra *= 2) {
    const K = bitLength(den) + extra;
    const num = (1n << BigInt(K)) / den;
    const d = decide({ m: num, e: oq * y - K, err: 1n });
    if (d !== null) return d;
  }
}

/** exp(y ln x) for x > 0 at precision p. */
function powApprox(x: number, y: number, p: number): Approx {
  const S = p + GUARD;
  const ey = exponent(y);
  // ln x at a scale where y times its error stays within units of scale S.
  const SL = S + Math.max(0, ey + 2) + 16;
  const L = logFixed(x, SL);
  const { m, q } = decompose(y);
  // T = y ln x at scale S: L * m * 2^(q + S - SL).
  const shift = q + S - SL;
  let T = L.v * m;
  T = shift >= 0 ? T << BigInt(shift) : T >> BigInt(-shift);
  if (y < 0) T = -T;
  const errT = shift >= 0 ? (L.err * m) << BigInt(shift) : ((L.err * m) >> BigInt(-shift)) + 1n;
  return expFixed(T, S, errT + 1n);
}

/**
 * The exact case of pow that an interval cannot decide: x^y half way between
 * two doubles. y = n / 2^k with n odd and k >= 1, and the midpoint M of the
 * interval is tested: M^(2^k) = x^n.
 */
function powMidpoint(a: Approx, odd: bigint, oq: number, y: number): number | null {
  const lo = roundExact(a.m - a.err, a.e);
  const hi = roundExact(a.m + a.err, a.e);
  if (!Number.isFinite(lo) || !Number.isFinite(hi)) return null;
  // The candidate: the midpoint of two adjacent doubles in the interval.
  const dl = decompose(Math.abs(lo) === 0 ? Number.MIN_VALUE : Math.abs(lo));
  const dh = decompose(Math.abs(hi));
  // As integers on a common grid: (dl + dh) / 2.
  const g = Math.min(dl.q, dh.q) - 1;
  const ml = (Math.abs(lo) === 0 ? 0n : dl.m << BigInt(dl.q - g));
  const mh = dh.m << BigInt(dh.q - g);
  let M = (ml + mh) / 2n;
  let Me = g;
  if (M === 0n) return null;
  while ((M & 1n) === 0n) {
    M >>= 1n;
    Me++;
  }
  const { m: ym, q: yq } = decompose(Math.abs(y));
  let n = ym;
  let k = -yq;
  while ((n & 1n) === 0n && k > 0) {
    n >>= 1n;
    k--;
  }
  if (k <= 0 || k > 12) return null;
  const pow2k = 1n << BigInt(k);
  // M^(2^k) = x^n, or M^(2^k) * x^|n| = 1 for y < 0: the odd parts and the exponents.
  if (bitLength(M) * Number(pow2k) > 1_000_000 || bitLength(odd) * Number(n) > 1_000_000) return null;
  const leftOdd = M ** pow2k;
  const leftExp = BigInt(Me) * pow2k;
  if (y > 0) {
    if (leftOdd !== odd ** n || leftExp !== BigInt(oq) * n) return null;
  } else {
    if (leftOdd * odd ** n !== 1n || leftExp + BigInt(oq) * n !== 0n) return null;
  }
  // The exact value is M * 2^Me, half way: round it, ties to even.
  return roundExact(M, Me);
}
