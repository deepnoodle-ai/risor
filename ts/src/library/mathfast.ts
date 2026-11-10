// The fast phase of the correctly rounded functions of package F13 (f13 2.1).
//
// Each function computes its value as a double-double h + l (an unevaluated
// sum of two doubles, about 106 bits) with a proven bound err on its error,
// and returns the double that both ends of [h + l - err, h + l + err] round
// to (`finish`). When the two ends round to different doubles it returns NaN,
// and the entry runs the exact path of mathcr.ts. With the bounds below that
// happens for about one call in 2^20 or fewer, and for the values that are
// exactly half way between two doubles (only `pow` has them).
//
// No digit of a result comes from a platform function. The arithmetic is IEEE
// addition, subtraction, multiplication and division of doubles, which every
// engine rounds correctly, and the exact error-free transformations built on
// them: TwoSum, FastTwoSum (Dekker), and TwoProduct by Veltkamp's
// splitting (Dekker), since JavaScript has no fused multiply-add. Math.floor,
// Math.round, Math.abs, Math.sqrt (correctly rounded by IEEE 754) and exact
// scaling by powers of two are the only other operations. The tables of
// mathtables.ts are computed by the exact kernels and checked by a test.
//
// The bounds. u = 2^-53. Each operation on double-doubles below (mul, add of
// two terms that do not cancel by more than half, div) has a relative error
// of at most 2^-100; the published bounds for double-word arithmetic are
// smaller (at most 7u^2 for the product, 3u^2 for the sum, 15u^2 for the
// quotient, from memory). Each function's comment adds up its errors to
// an analytic bound, and the rounding test uses a bound at least 2^9 times
// larger: a bound that is too wide only costs speed. test/mathfast.test.ts
// compares fast results with the exact path, and measures the actual error of
// many of them against the exact value: it must be at most 2^-8 of the bound.

import {
  ATAN, EXP_T1, EXP_T2, INV_LN10_H, INV_LN10_L, LN2_1, LN2_2, LN2_3, LN2_4096_1, LN2_4096_2, LN2_4096_3,
  LOG_C, LOG_T, PI_H, PI_L, PIO2_1, PIO2_2, PIO2_3, PIO2_4, PIO2_H, PIO2_L, SINCOS, SIXTH_H, SIXTH_L, THIRD_H, THIRD_L, TWO_OVER_PI,
} from "./mathtables.ts";

// ---- bits and powers of two ---------------------------------------------------

const F64 = new Float64Array(1);
const U32 = new Uint32Array(F64.buffer);
/** The index of the high word of a double in U32: 1 on a little-endian platform. */
const HI = new Uint8Array(new Uint16Array([1]).buffer)[0] === 1 ? 1 : 0;
const LO = 1 - HI;

/** 2^k for -1022 <= k <= 1023, built from its bits. */
function pow2(k: number): number {
  U32[HI] = (k + 1023) << 20;
  U32[LO] = 0;
  return F64[0];
}

/** x * 2^k, exact when the result is a normal double; Infinity past the largest. */
function ldexp(x: number, k: number): number {
  if (k > 1000) {
    x *= pow2(1000);
    k -= 1000;
  } else if (k < -1000) {
    x *= pow2(-1000);
    k += 1000;
  }
  return x * pow2(k);
}

const P2_24 = pow2(24);
const P2_M24 = pow2(-24);
const P2_M27 = pow2(-27);
const P2_M28 = pow2(-28);
const P2_M54 = pow2(-54);
const P2_M60 = pow2(-60);
const P2_60 = pow2(60);
const P2_64 = pow2(63) * 2;
const P2_600 = pow2(600);
const P2_M1000 = pow2(-1000);
const P2_M900 = pow2(-900);
const P2_54 = pow2(54);
/** The relative bounds of the rounding tests (see each function). */
const B78 = pow2(-78);
const B80 = pow2(-80);
const B92 = pow2(-92);
const B93 = pow2(-93);

// ---- double-double arithmetic -------------------------------------------------
//
// The operations on pairs are written out in each kernel, so that each runs on
// local variables whatever the engine's inlining decides:
// - TwoSum(a, b): s = a + b, t = s - a, e = (a - (s - t)) + (b - t);
// - FastTwoSum(a, b) (Dekker), for |a| >= |b|: s = a + b, e = b - (s - a);
// - TwoProduct(a, b) (Dekker): p = a b, e = (((ah bh - p) + ah bl) + al bh) +
//   al bl, with a = ah + al and b = bh + bl split by Veltkamp (c = (2^27 + 1)
//   a, ah = c - (c - a)) into halves of at most 26 bits; a factor of at most
//   26 bits is its own high half. Exact for |a|, |b| < 2^995 and a product
//   whose error term is not below 2^-969;
// - the product of pairs: TwoProduct of the high parts plus ah bl + al bh;
// - the sum of pairs: TwoSum (or FastTwoSum) of the high parts plus the low.
// A kernel leaves its result in D[0] (the high part) and D[1] (the low).

/** The two parts of a pair operation's result: D[0] high, D[1] low. */
const D = new Float64Array(2);
/**
 * Other results, in a typed array so that a double is stored unboxed: G[0], G[1]
 * the reduced argument; G[2], G[3] sin r; G[4], G[5] cos r; G[6], G[7]
 * sqrt(1 - x^2); G[8] to G[11] the last rounding test, for the tests; G[12]
 * the absolute error of the reduced argument.
 */
const G = new Float64Array(13);

/** 2^27 + 1, Veltkamp's constant for doubles. */
const SPLIT = 134217729;

// ---- the rounding test --------------------------------------------------------

/**
 * For the tests: the double-double, error bound and scale of the last rounding
 * test, or null when there was none since the last call of clearLastFast.
 */
export function lastFast(): { h: number; l: number; err: number; k: number } | null {
  return G[10] < 0 ? null : { h: G[8], l: G[9], err: G[10], k: G[11] };
}

/** For the tests: forget the last rounding test. */
export function clearLastFast(): void {
  G[10] = -1;
}

/**
 * The double nearest the exact value v * 2^k, where |v - (h + l)| <= err and
 * |h + l| >= 2^-960; or NaN when [h + l - err, h + l + err] does not round to
 * one double.
 *
 * r = h + l rounded and r + t = h + l exactly (TwoSum). The midpoints between
 * r and its neighbours are ulp(r)/2 from r, or ulp(r)/4 below a power of two
 * (taken on both sides then). When |t| + err is less than that distance d,
 * the whole interval is strictly between the two midpoints and rounds to r.
 * The sum |t| + err is rounded, but a double below the power of two d is at
 * most d (1 - 2^-53), and its rounding error at most 2^-53 of it, so the exact
 * sum is below d too. Scaling by 2^k keeps the rounding when the result is a
 * normal double, which each caller ensures, and gives Infinity exactly when
 * the rounded value is 2^1024 or more.
 */
function finish(h: number, l: number, err: number, k: number): number {
  G[8] = h;
  G[9] = l;
  G[10] = err;
  G[11] = k;
  const r = h + l;
  const bb = r - h;
  const t = h - (r - bb) + (l - bb);
  F64[0] = r;
  const hi = U32[HI];
  const pow = (hi & 0xfffff) === 0 && U32[LO] === 0;
  // ulp(r)/2 = 2^(E - 53), E the exponent of r (at least -960); ulp(r)/4 when r is a power of two.
  U32[HI] = (hi & 0x7ff00000) - 0x3500000;
  U32[LO] = 0;
  const d = pow ? F64[0] * 0.5 : F64[0];
  if (Math.abs(t) + err < d) return k === 0 ? r : ldexp(r, k);
  return NaN;
}

// ---- exp ----------------------------------------------------------------------

/** Veltkamp's split of x into a high half of 26 bits and the rest. */
function splitHi(x: number): number {
  const c = SPLIT * x;
  return c - (c - x);
}

// Constants split once into halves for TwoProduct.
const LN2_4096_2H = splitHi(LN2_4096_2);
const LN2_4096_2L = LN2_4096_2 - LN2_4096_2H;
const LN2_2H = splitHi(LN2_2);
const LN2_2L = LN2_2 - LN2_2H;
const INV_LN10_HH = splitHi(INV_LN10_H);
const INV_LN10_HL = INV_LN10_H - INV_LN10_HH;
const PIO2_HH = splitHi(PIO2_H);
const PIO2_HL = PIO2_H - PIO2_HH;

/** 4096 / ln 2, to pick the reduction; any nearby value would do. */
const INV_LN2_4096 = 1 / (LN2_4096_1 + LN2_4096_2);

/**
 * exp(xh + xl) = (D[0] + D[1]) * 2^k, returning k, for |xh| <= 710 and |xl| <= ulp(xh).
 *
 * Method: K = round(x 4096 / ln 2) and r = x - K ln2/4096, |K| < 2^23 and |r|
 * <= 0.51 ln2/4096 < 2^-13.4. Then exp(x) = 2^(K >> 12) 2^(i/64) 2^(j/4096)
 * exp(r), i = (K >> 6) & 63, j = K & 63, from two tables of 64 double-doubles.
 * exp(r) = 1 + r + r^2 (1/2 + q), q = r/6 + ... + r^5/5040 in doubles (the
 * omitted terms are below 2^-120).
 *
 * Errors, relative to the result (which is in [1, 2)):
 * - r: ln2/4096 is L1 + L2 + L3 to 2^-148, L1 of 30 bits so K L1 is exact,
 *   K L2 by TwoProduct; the low parts are added in doubles of at most 2^-43
 *   (2^-65 when xl = 0): at most 2^-94 (2^-115);
 * - q: |q| < 2^-16, its rounding, its coefficients and the omitted rl: at most
 *   2^-66, times r^2 < 2^-26.8: 2^-92.8;
 * - the tables (2^-106 each) and five double-double operations: 2^-98.
 * So at most 2^-92 when xl = 0, and 2^-91 otherwise.
 */
function expKernel(xh: number, xl: number): number {
  // The operations on pairs are written out: TwoSum, FastTwoSum when the
  // order of the magnitudes is known, and TwoProduct.
  const K = Math.round(xh * INV_LN2_4096);
  // r = xh + xl - K ln2/4096 = rh + rl. TwoSum(xh, -K L1), K L1 exact.
  const m1 = -K * LN2_4096_1;
  const s1 = xh + m1;
  let t = s1 - xh;
  const e1 = xh - (s1 - t) + (m1 - t);
  // K L2 = p + pe exactly: Dekker's product with K unsplit (|K| < 2^23).
  const p = K * LN2_4096_2;
  const pe = K * LN2_4096_2H - p + K * LN2_4096_2L;
  // TwoSum(s1, -p).
  const s2 = s1 - p;
  t = s2 - s1;
  const e2 = s1 - (s2 - t) + (-p - t);
  const low = e2 + e1 - pe - K * LN2_4096_3 + xl;
  // TwoSum(s2, low).
  const rh = s2 + low;
  t = rh - s2;
  const rl = s2 - (rh - t) + (low - t);
  const q = rh * (1 / 6 + rh * (1 / 24 + rh * (1 / 120 + rh * (1 / 720 + rh * (1 / 5040)))));
  // c = 1/2 + q, FastTwoSum.
  const ch = 0.5 + q;
  const cl = q - (ch - 0.5);
  // r^2 = sh + sl: TwoProduct(rh, rh) and 2 rh rl.
  let c = SPLIT * rh;
  const rhh = c - (c - rh);
  const rhl = rh - rhh;
  const sh = rh * rh;
  const sl = rhh * rhh - sh + 2 * rhh * rhl + rhl * rhl + 2 * rh * rl;
  // u = r^2 c.
  c = SPLIT * sh;
  const shh = c - (c - sh);
  const shl = sh - shh;
  c = SPLIT * ch;
  const chh = c - (c - ch);
  const chl = ch - chh;
  const uh = sh * ch;
  const ul = shh * chh - uh + shh * chl + shl * chh + shl * chl + (sh * cl + sl * ch);
  // exp(r) - 1 = r + u = ph + pl; |u| <= 2^-14 |r|, FastTwoSum.
  const ph = rh + uh;
  const pl = uh - (ph - rh) + (rl + ul);
  // T = 2^(i/64) 2^(j/4096) = th + tl.
  const i = 2 * ((K >> 6) & 63);
  const j = 2 * (K & 63);
  const ah = EXP_T1[i];
  const bh = EXP_T2[j];
  c = SPLIT * ah;
  const ahh = c - (c - ah);
  const ahl = ah - ahh;
  c = SPLIT * bh;
  const bhh = c - (c - bh);
  const bhl = bh - bhh;
  const th = ah * bh;
  const tl = ahh * bhh - th + ahh * bhl + ahl * bhh + ahl * bhl + (ah * EXP_T2[j + 1] + EXP_T1[i + 1] * bh);
  // T p.
  c = SPLIT * th;
  const thh = c - (c - th);
  const thl = th - thh;
  c = SPLIT * ph;
  const phh = c - (c - ph);
  const phl = ph - phh;
  const vh = th * ph;
  const vl = thh * phh - vh + thh * phl + thl * phh + thl * phl + (th * pl + tl * ph);
  // T + T p; |T p| <= 2^-13 T, FastTwoSum.
  const h = th + vh;
  D[1] = vh - (h - th) + (tl + vl);
  D[0] = h;
  return K >> 12;
}

/**
 * exp(x). Below 2^-54 in magnitude the result is 1: exp(x) is then strictly
 * between 1 - 2^-54 and 1 + 2^-53, the midpoints around 1. From 710, exp(x) >
 * 2^1024 (ln 2^1024 = 709.78...): Infinity. Below -745.14, exp(x) < 2^-1075
 * (ln 2^-1075 = -745.133...), half the least subnormal: 0. From there to -708
 * the result is near or among the subnormals and the exact path decides it.
 * Otherwise the kernel's 2^-91 and the rounding test's 2^-80 (a margin of
 * 2^11); past 709.78 the scaling gives Infinity exactly when the rounded value
 * is 2^1024.
 */
export function fastExp(x: number): number {
  if (Math.abs(x) < P2_M54) return 1;
  if (x >= 710) return Infinity;
  if (x < -745.14) return 0;
  if (x <= -708) return NaN;
  const k = expKernel(x, 0);
  return finish(D[0], D[1], Math.abs(D[0]) * B80, k);
}

// ---- log ----------------------------------------------------------------------

/**
 * ln x = D[0] + D[1] for a finite x > 0, with a relative error below 2^-90.
 *
 * Method: x = 2^e m, m in [1, 2); i = round((m - 1) 512) from the top 20 bits
 * of m, folded to i = 0 and e + 1 at i = 512 (m/2 in [1 - 2^-11, 1)); c_i =
 * 1/(1 + i/512) rounded (a table); z = m c_i - 1, exact as a double-double by
 * TwoProduct (m c_i is within 2^-9 of 1, so its high part less 1 is exact);
 * |z| < 2^-9.99. ln x = e ln2 + (-ln c_i) + log1p(z).
 *
 * log1p(z) = z - z^2/2 + z^3 P, P = 1/3 - z/4 + z^2 R, R = 1/5 - z/6 + ... -
 * z^6/11 in doubles (the omitted terms are below 2^-119 relative to z). z^2 is
 * a double-double by TwoProduct, z^3 = z^2 z and z^3 P double-double products;
 * P is the pair (1/3 high, -zh/4) by FastTwoSum, plus 1/3 low, -zl/4 and zh^2 R in
 * doubles. Errors, relative to log1p(z): zh^2 R (2^-22.3) rounded and R's own
 * error, at most 2^-73 in P, times z^2: 2^-93; the double-double operations:
 * 2^-100. So 2^-92.9.
 *
 * The sum: e ln2 with ln2 = L1 + L2 + L3 to 2^-150, L1 of 42 bits so e L1 is
 * exact (|e| < 2^11), e L2 by TwoProduct; -ln c_i to 2^-106; the high parts
 * added by TwoSum and the low parts in doubles: at most 2^-104 relative to the
 * result (|ln x| >= 2^-11 unless e = i = 0, when the sum is log1p(z) alone).
 * |log1p(z)| is at most 1.03 |ln x|. So below 2^-92.8 in all: 2^-90 is used.
 */
function logKernel(x: number): void {
  F64[0] = x;
  let hi = U32[HI];
  let e = (hi >>> 20) - 1023;
  if (e === -1023) {
    // A subnormal: scaled by 2^54, exactly.
    F64[0] = x * P2_54;
    hi = U32[HI];
    e = (hi >>> 20) - 1023 - 54;
  }
  let i = ((hi & 0xfffff) + 1024) >>> 11;
  U32[HI] = (hi & 0xfffff) | 0x3ff00000;
  let m = F64[0];
  if (i === 512) {
    i = 0;
    e += 1;
    m *= 0.5;
  }
  // The operations on pairs are written out, as in expKernel.
  let zh: number;
  let zl: number;
  let c: number;
  if (i === 0) {
    zh = m - 1; // exact (Sterbenz)
    zl = 0;
  } else {
    // TwoProduct(m, c_i); then p - 1 is exact (Sterbenz) and FastTwoSum.
    const ci = LOG_C[i];
    c = SPLIT * m;
    const mh = c - (c - m);
    const ml = m - mh;
    c = SPLIT * ci;
    const cih = c - (c - ci);
    const cil = ci - cih;
    const p = m * ci;
    const pe = mh * cih - p + mh * cil + ml * cih + ml * cil;
    const a = p - 1;
    zh = a + pe;
    zl = pe - (zh - a);
  }
  const R = 0.2 + zh * (-1 / 6 + zh * (1 / 7 + zh * (-1 / 8 + zh * (1 / 9 + zh * (-0.1 + zh * (1 / 11))))));
  // P = 1/3 - z/4 + z^2 R as a pair: FastTwoSum(1/3 high, -zh/4).
  const q4 = -0.25 * zh;
  const ph = THIRD_H + q4;
  const pl = q4 - (ph - THIRD_H) + THIRD_L - 0.25 * zl + zh * zh * R;
  // z^2 = s2h + s2l: TwoProduct(zh, zh) and 2 zh zl.
  c = SPLIT * zh;
  const zhh = c - (c - zh);
  const zhl = zh - zhh;
  const s2h = zh * zh;
  const s2l = zhh * zhh - s2h + 2 * zhh * zhl + zhl * zhl + 2 * zh * zl;
  // z^3 = z^2 z.
  c = SPLIT * s2h;
  const ah = c - (c - s2h);
  const al = s2h - ah;
  const z3h = s2h * zh;
  const z3l = ah * zhh - z3h + ah * zhl + al * zhh + al * zhl + (s2h * zl + s2l * zh);
  // w = z^3 P.
  c = SPLIT * z3h;
  const bh = c - (c - z3h);
  const bl = z3h - bh;
  c = SPLIT * ph;
  const phh = c - (c - ph);
  const phl = ph - phh;
  const wh = z3h * ph;
  const wl = bh * phh - wh + bh * phl + bl * phh + bl * phl + (z3h * pl + z3l * ph);
  // v = -z^2/2 + w; |w| <= 2^-9 z^2/2, FastTwoSum.
  const vh = -0.5 * s2h + wh;
  const vl = wh - (vh + 0.5 * s2h) + (wl - 0.5 * s2l);
  // log1p(z) = z + v; |v| <= 2^-10 |z|, FastTwoSum.
  const lh = zh + vh;
  const ll = vh - (lh - zh) + (zl + vl);
  if (e === 0 && i === 0) {
    D[0] = lh;
    D[1] = ll;
    return;
  }
  // TwoSum(e L1, -ln c_i); e L1 exact.
  const ti = 2 * i;
  const a = e * LN2_1;
  const b = LOG_T[ti];
  const s = a + b;
  let t = s - a;
  const se = a - (s - t) + (b - t);
  // e L2 = p + pe exactly: Dekker's product with e unsplit (|e| < 2^11).
  const p = e * LN2_2;
  const pe = e * LN2_2H - p + e * LN2_2L;
  // TwoSum(s, p), TwoSum(s2, log1p).
  const s2 = s + p;
  t = s2 - s;
  const s2e = s - (s2 - t) + (p - t);
  const s3 = s2 + lh;
  t = s3 - s2;
  const s3e = s2 - (s3 - t) + (lh - t);
  const low = se + s2e + s3e + pe + LOG_T[ti + 1] + e * LN2_3 + ll;
  // |low| is a few units of s3, which is at least 2^-11: FastTwoSum.
  const h = s3 + low;
  D[1] = low - (h - s3);
  D[0] = h;
}

/** ln x for x > 0: the kernel's 2^-90, and 2^-80 in the rounding test. */
export function fastLog(x: number): number {
  if (x === 1) return 0;
  logKernel(x);
  return finish(D[0], D[1], Math.abs(D[0]) * B80, 0);
}

/** log10 x for x > 0: ln x (2^-90) times 1/ln 10 (2^-106, and 2^-100 for the product); 2^-80 in the test. */
export function fastLog10(x: number): number {
  if (x === 1) return 0;
  logKernel(x);
  // ln x times 1/ln 10: TwoProduct with the constant's halves, and the low parts.
  const lh = D[0];
  const c = SPLIT * lh;
  const lhh = c - (c - lh);
  const lhl = lh - lhh;
  const p = lh * INV_LN10_H;
  const pe = lhh * INV_LN10_HH - p + lhh * INV_LN10_HL + lhl * INV_LN10_HH + lhl * INV_LN10_HL + (lh * INV_LN10_L + D[1] * INV_LN10_H);
  return finish(p, pe, Math.abs(p) * B80, 0);
}

// ---- sin, cos, tan ------------------------------------------------------------

/** 2/pi in limbs of 24 bits, after four zero limbs: PIO[4 + i] = P[i]. */
const PIO = new Float64Array(4 + TWO_OVER_PI.length);
PIO.set(TWO_OVER_PI, 4);
const Y = new Float64Array(10);

/**
 * Payne and Hanek's reduction of x >= 0.78: the quadrant q in 0..3, and G[0] +
 * G[1] = r with x = (4n + q) pi/2 + r, |r| <= pi/4 (1 + 2^-50), and a relative
 * error of r below 2^-101. Returns -1 when the reduction cannot promise that
 * (it never happens for a double; then the exact path runs).
 *
 * x = (X0 + X1 2^-24 + X2 2^-48) 2^e0, the X of 24 bits, and 2/pi = sum of
 * P[i] 2^(-24 (i + 1)). The terms of x 2/pi are grouped by n = i + j, Q_n =
 * sum of X_j P[n - j], weight 2^(e0 - 24 (n + 1)), each below 2^50, so exact
 * in doubles. The terms whose weight is 4 or more are multiples of 4 and are
 * left out: x 2/pi is wanted modulo 4. Ten groups from n_s, whose weight 2^w0
 * has -22 <= w0 <= 1, are computed and their carries propagated, exactly, to
 * limbs of 24 bits Y0..Y9, Yn of weight 2^(w0 - 24 n); the carry out of the
 * top is a multiple of 4. The groups after the tenth add less than 2^(w0 -
 * 190).
 *
 * The top limb modulo 2^(2 - w0) and the second, A = 48 bits at most from 2^1
 * to 2^(w0 - 24), give q = floor(A) and t0 = A - q, both exact; the limbs
 * after them are below 2^(w0 - 24), so the fraction f is below 1/2 when t0 is.
 * When t0 >= 1/2, q + 1 is taken and the fraction is 1 - f, formed exactly by
 * complementing the limbs, so the fraction is always a sum of terms of one
 * sign. As a double-double: t0 and the sums of pairs of limbs (48 bits, exact)
 * when t0 != 0, else from the first nonzero limb a <= 3 (f >= 2^(w0 - 72));
 * the limbs and groups left out are below 2^-117 of it, and the one rounding
 * of the low part 2^-105. Times pi/2 (a double-double product): 2^-101. (For
 * a double f is at least 2^-62, so a is at most 2.)
 */
function reduce(x: number): number {
  F64[0] = x;
  const e0 = ((U32[HI] >>> 20) & 0x7ff) - 1023 - 23;
  const s = x * pow2(-e0);
  const X0 = Math.floor(s);
  const f = (s - X0) * P2_24;
  const X1 = Math.floor(f);
  const X2 = (f - X1) * P2_24;
  const ns = Math.ceil((e0 - 1) / 24) - 1;
  const w0 = e0 - 24 * (ns + 1);
  for (let n = 0; n < 10; n++) {
    const b = ns + n + 4;
    Y[n] = X0 * PIO[b] + X1 * PIO[b - 1] + X2 * PIO[b - 2];
  }
  let carry = 0;
  for (let n = 9; n >= 0; n--) {
    const v = Y[n] + carry;
    carry = Math.floor(v * P2_M24);
    Y[n] = v - carry * P2_24;
  }
  // Y[0] is an integer below 2^24, so the mask is exact.
  const A = (Y[0] & ((1 << (2 - w0)) - 1)) * pow2(w0) + Y[1] * pow2(w0 - 24);
  let q = Math.floor(A);
  let t0 = A - q;
  let sign = 1;
  if (t0 >= 0.5) {
    q += 1;
    sign = -1;
    t0 = 1 - t0 - pow2(w0 - 24);
    for (let n = 2; n < 10; n++) Y[n] = 16777215 - Y[n];
    Y[9] += 1;
  }
  // f = fh + fl: FastTwoSum of the first exact sums; then f pi/2.
  let w = pow2(w0 - 48);
  let fh: number;
  let fl: number;
  if (t0 !== 0) {
    const v2 = Y[2] * w;
    w *= P2_M24;
    const v3 = Y[3] * w;
    w *= P2_M24;
    const v4 = Y[4] * w;
    w *= P2_M24;
    const v5 = Y[5] * w;
    w *= P2_M24;
    const v6 = Y[6] * w;
    const b = v2 + v3;
    fh = t0 + b;
    fl = b - (fh - t0) + (v4 + v5 + v6);
  } else {
    let a = 2;
    for (; a <= 3 && Y[a] === 0; a++) w *= P2_M24;
    if (a > 3) return -1;
    const v0 = Y[a] * w;
    w *= P2_M24;
    const v1 = Y[a + 1] * w;
    w *= P2_M24;
    const v2 = Y[a + 2] * w;
    w *= P2_M24;
    const v3 = Y[a + 3] * w;
    w *= P2_M24;
    const v4 = Y[a + 4] * w;
    w *= P2_M24;
    const v5 = Y[a + 5] * w;
    const a1 = v0 + v1;
    const b = v2 + v3;
    fh = a1 + b;
    fl = b - (fh - a1) + (v4 + v5);
  }
  const c = SPLIT * fh;
  const fhh = c - (c - fh);
  const fhl = fh - fhh;
  const ph = fh * PIO2_H;
  const pl = fhh * PIO2_HH - ph + fhh * PIO2_HL + fhl * PIO2_HH + fhl * PIO2_HL + (fh * PIO2_L + fl * PIO2_H);
  const h = ph + pl;
  G[1] = sign * (pl - (h - ph));
  G[0] = sign * h;
  return q & 3;
}

/**
 * sin r and cos r for r = rh + rl, |r| <= pi/4 (1 + 2^-30): into G[2] + G[3]
 * (when want & 1) and G[4] + G[5] (when want & 2).
 *
 * Method: j = round(256 rh), |j| <= 201, and t = r - j/256 (rh - j/256 is
 * exact), |t| <= 2^-9 (1 + 2^-20). With S and C sin and cos of j/256 (a table
 * of double-doubles) and z = cos t - 1, sin r = S + S z + C sin t and cos r =
 * C + C z - S sin t. With u = t^2 <= 2^-18:
 * - sin t = t + t u A, A = -1/6 + u B, B = 1/120 - u/5040 + u^2/9! - u^3/11!
 *   in doubles (omitted terms below 2^-140). B's error is at most 2^-58.5, so
 *   A's at most 2^-76 with the rounding of u B and the omitted low part of u,
 *   and sin t's relative error at most 2^-94 + 2^-100 (operations);
 * - z = u D, D = -1/2 + u E, E = 1/24 - u/720 + u^2/8! - u^3/10! in doubles.
 *   E's error is at most 2^-56, so D's at most 2^-73, and z's absolute error
 *   at most 2^-91 + 2^-118.
 * The sums: |C sin t| <= 2^-9 (1 + 2^-20) and |S| >= sin(1/256) when j != 0,
 * so sin r cancels by about half at most, and cos r >= 0.7: sin r within
 * 2.02 (2^-91 + 2^-94 2^-9 + 2^-100) < 2^-89.5 and cos r within 2^-90.5,
 * with the 2^-106 of the tables and the 2^-101 of r.
 *
 * The operations on pairs are written out: TwoSum, FastTwoSum when the order
 * of the magnitudes is known, and TwoProduct.
 */
function sinCosR(rh: number, rl: number, want: number): void {
  const j = Math.round(rh * 256);
  // t = th + tl: TwoSum(rh - j/256, rl).
  const a0 = rh - j * 0.00390625;
  const th = a0 + rl;
  let s = th - a0;
  const tl = a0 - (th - s) + (rl - s);
  // u = t^2: TwoProduct(th, th) and 2 th tl.
  let c = SPLIT * th;
  const thh = c - (c - th);
  const thl = th - thh;
  const uh = th * th;
  const ul = thh * thh - uh + 2 * thh * thl + thl * thl + 2 * th * tl;
  c = SPLIT * uh;
  const uhh = c - (c - uh);
  const uhl = uh - uhh;
  // A = -1/6 + u B: FastTwoSum(-1/6 high, u B), less 1/6 low.
  const B = 1 / 120 + uh * (-1 / 5040 + uh * (1 / 362880 - uh * (1 / 39916800)));
  const w = uh * B;
  const ah = w - SIXTH_H;
  const al = w - (ah + SIXTH_H) - SIXTH_L;
  // u A.
  c = SPLIT * ah;
  const ahh = c - (c - ah);
  const ahl = ah - ahh;
  const bh = uh * ah;
  const bl = uhh * ahh - bh + uhh * ahl + uhl * ahh + uhl * ahl + (uh * al + ul * ah);
  // y = t u A.
  c = SPLIT * bh;
  const bhh = c - (c - bh);
  const bhl = bh - bhh;
  const yh = th * bh;
  const yl = thh * bhh - yh + thh * bhl + thl * bhh + thl * bhl + (th * bl + tl * bh);
  // sin t = t + y; |y| <= 2^-20 |t|, FastTwoSum.
  const sth = th + yh;
  const stl = yh - (sth - th) + (tl + yl);
  if (j === 0 && want === 1) {
    G[2] = sth;
    G[3] = stl;
    return;
  }
  // z = u D, D = -1/2 + u E: FastTwoSum(-1/2, u E).
  const E = 1 / 24 + uh * (-1 / 720 + uh * (1 / 40320 - uh * (1 / 3628800)));
  const w2 = uh * E;
  const dh = w2 - 0.5;
  const dl = w2 - (dh + 0.5);
  c = SPLIT * dh;
  const dhh = c - (c - dh);
  const dhl = dh - dhh;
  const zh = uh * dh;
  const zl = uhh * dhh - zh + uhh * dhl + uhl * dhh + uhl * dhl + (uh * dl + ul * dh);
  if (j === 0) {
    G[2] = sth;
    G[3] = stl;
    // cos t = 1 + z, FastTwoSum.
    const ch = 1 + zh;
    G[5] = zh - (ch - 1) + zl;
    G[4] = ch;
    return;
  }
  const k = 4 * (j < 0 ? -j : j);
  const Sh = j < 0 ? -SINCOS[k] : SINCOS[k];
  const Sl = j < 0 ? -SINCOS[k + 1] : SINCOS[k + 1];
  const Ch = SINCOS[k + 2];
  const Cl = SINCOS[k + 3];
  c = SPLIT * Sh;
  const Shh = c - (c - Sh);
  const Shl = Sh - Shh;
  c = SPLIT * Ch;
  const Chh = c - (c - Ch);
  const Chl = Ch - Chh;
  c = SPLIT * sth;
  const sthh = c - (c - sth);
  const sthl = sth - sthh;
  c = SPLIT * zh;
  const zhh = c - (c - zh);
  const zhl = zh - zhh;
  if (want & 1) {
    // C sin t and S z.
    const ph = Ch * sth;
    const pl = Chh * sthh - ph + Chh * sthl + Chl * sthh + Chl * sthl + (Ch * stl + Cl * sth);
    const qh = Sh * zh;
    const ql = Shh * zhh - qh + Shh * zhl + Shl * zhh + Shl * zhl + (Sh * zl + Sl * zh);
    // m = C sin t + S z; |S z| <= 2^-9 |C sin t|, FastTwoSum.
    const mh = ph + qh;
    const ml = qh - (mh - ph) + (pl + ql);
    // sin r = S + m; |m| <= |S|, FastTwoSum.
    const h = Sh + mh;
    G[3] = mh - (h - Sh) + (Sl + ml);
    G[2] = h;
  }
  if (want & 2) {
    // C z and S sin t.
    const ph = Ch * zh;
    const pl = Chh * zhh - ph + Chh * zhl + Chl * zhh + Chl * zhl + (Ch * zl + Cl * zh);
    const qh = Sh * sth;
    const ql = Shh * sthh - qh + Shh * sthl + Shl * sthh + Shl * sthl + (Sh * stl + Sl * sth);
    // m = C z - S sin t: TwoSum.
    const mh = ph - qh;
    s = mh - ph;
    const ml = ph - (mh - s) + (-qh - s) + (pl - ql);
    // cos r = C + m; C >= 0.7 > |m|, FastTwoSum.
    const h = Ch + mh;
    G[5] = mh - (h - Ch) + (Cl + ml);
    G[4] = h;
  }
}

/** 2/pi, to pick the multiple of pi/2; any nearby value would do. */
const INV_PIO2 = 1 / PIO2_H;
/** PIO2_3 split in two halves of at most 26 bits (Veltkamp). */
const PIO2_3H = splitHi(PIO2_3);
const PIO2_3L = PIO2_3 - PIO2_3H;
const P2_20 = pow2(20);
const P2_M20 = pow2(-20);

/**
 * The reduced argument of x >= 2^-27 into G[0] + G[1] and its quadrant, or
 * -1; the bound on its absolute error into G[12].
 *
 * Below 0.78 (< pi/4) x is its own reduction. Below 2^20, Cody and Waite's: k
 * = round(x 2/pi) < 2^20, pi/2 = C1 + C2 + C3 + C4 to 2^-170, C1 and C2 of 33
 * bits so that k C1 and k C2 are exact, k C3 exact by TwoProduct (k has 20
 * bits, so k times each half of C3 is exact). x - k C1, less k C2, less k C3
 * by TwoSum: exact. The five low parts (three below 2^-54 each, k C3's below
 * 2^-102, k C4 below 2^-102) are added in doubles, with rounding errors below
 * 2^-103 in all; k (pi/2 - C1 - ... - C4) is below 2^-150. So the absolute
 * error of r is below 2^-102.9, and 2^-93 is passed on (see reducedErr).
 * |r| <= pi/4 (1 + 2^-30): k comes from a rounded 2/pi. Above 2^20, and when
 * |r| < 2^-20, Payne and Hanek's (reduce): a relative error of 2^-101.
 */
function reduceAbs(ax: number): number {
  G[12] = 0;
  if (ax < 0.78) {
    G[0] = ax;
    G[1] = 0;
    return 0;
  }
  if (ax >= P2_20) return reduce(ax);
  const k = Math.round(ax * INV_PIO2);
  // TwoSum(x, -k C1), TwoSum(s1, -k C2), TwoSum(s2, -k C3): each exact.
  const m1 = -k * PIO2_1;
  const s1 = ax + m1;
  let t = s1 - ax;
  const e1 = ax - (s1 - t) + (m1 - t);
  const m2 = -k * PIO2_2;
  const s2 = s1 + m2;
  t = s2 - s1;
  const e2 = s1 - (s2 - t) + (m2 - t);
  // k C3 = p + pe exactly: Dekker's product with k unsplit.
  const p = k * PIO2_3;
  const pe = k * PIO2_3H - p + k * PIO2_3L;
  const s3 = s2 - p;
  t = s3 - s2;
  const e3 = s2 - (s3 - t) + (-p - t);
  const low = e3 + e1 + e2 - pe - k * PIO2_4;
  // TwoSum(s3, low).
  const rh = s3 + low;
  // Near a multiple of pi/2 the absolute bound is too coarse: the relative one of Payne and Hanek.
  if (Math.abs(rh) < P2_M20) return reduce(ax);
  t = rh - s3;
  G[1] = s3 - (rh - t) + (low - t);
  G[0] = rh;
  G[12] = B93;
  return k & 3;
}

/**
 * The relative error that an absolute error ea of the reduced argument r
 * gives sin r, cos r, tan r and cot r: at most ea/|r| times |r cot r|, |r tan
 * r| or |r / (sin r cos r)|, each at most 2 for |r| <= pi/4 (1 + 2^-30). So
 * 2 ea / |r|, and 4 ea / |rh| covers |r| >= |rh| / 2.
 */
function reducedErr(): number {
  return G[12] === 0 ? 0 : (4 * G[12]) / Math.abs(G[0]);
}

/**
 * sin x. Below 2^-27 in magnitude the result is x: sin x is between x - x^3/6
 * and x, and x^3/6 < |x| 2^-55, less than the distance from x to a midpoint.
 * Otherwise sin or cos of the reduced argument (2^-89.5 relative), and 2^-78
 * in the rounding test (a margin of 2^11).
 */
export function fastSin(x: number): number {
  const ax = Math.abs(x);
  if (ax < P2_M27) return x;
  const q = reduceAbs(ax);
  if (q < 0) return NaN;
  let h: number;
  let l: number;
  if ((q & 1) === 0) {
    sinCosR(G[0], G[1], 1);
    h = G[2];
    l = G[3];
  } else {
    sinCosR(G[0], G[1], 2);
    h = G[4];
    l = G[5];
  }
  if ((q >= 2) !== (x < 0)) {
    h = -h;
    l = -l;
  }
  return finish(h, l, Math.abs(h) * (B78 + reducedErr()), 0);
}

/**
 * cos x. Below 2^-27 in magnitude the result is 1: cos x is between 1 - x^2/2
 * and 1, above the midpoint 1 - 2^-54. Otherwise as sin.
 */
export function fastCos(x: number): number {
  const ax = Math.abs(x);
  if (ax < P2_M27) return 1;
  const q = reduceAbs(ax);
  if (q < 0) return NaN;
  let h: number;
  let l: number;
  if ((q & 1) === 0) {
    sinCosR(G[0], G[1], 2);
    h = G[4];
    l = G[5];
  } else {
    sinCosR(G[0], G[1], 1);
    h = G[2];
    l = G[3];
  }
  if (q === 1 || q === 2) {
    h = -h;
    l = -l;
  }
  return finish(h, l, Math.abs(h) * (B78 + reducedErr()), 0);
}

/**
 * tan x. Below 2^-27 in magnitude the result is x: tan x is between x and x +
 * x^3/2, and x^3/2 < |x| 2^-55. Otherwise sin r / cos r, or -cos r / sin r in
 * an odd quadrant: 2^-89.5 + 2^-90.5 + 2^-100 (the quotient), below 2^-88.5;
 * 2^-78 in the rounding test.
 */
export function fastTan(x: number): number {
  const ax = Math.abs(x);
  if (ax < P2_M27) return x;
  const q = reduceAbs(ax);
  if (q < 0) return NaN;
  sinCosR(G[0], G[1], 3);
  if ((q & 1) === 0) div(G[2], G[3], G[4], G[5]);
  else div(-G[4], -G[5], G[2], G[3]);
  let h = D[0];
  let l = D[1];
  if (x < 0) {
    h = -h;
    l = -l;
  }
  return finish(h, l, Math.abs(h) * (B78 + reducedErr()), 0);
}

// ---- atan, atan2, asin, acos --------------------------------------------------

/**
 * atan z into D[0] + D[1], for z = zh + zl, 0 <= z <= 1 + 2^-50, with a relative
 * error below 2^-88.8.
 *
 * Method: j = round(256 zh), c = j/256, and atan z = atan c + atan d, d = (z -
 * c) / (1 + z c), |d| <= 2^-9. zh - c is exact (Sterbenz, or c = 0), z c by
 * TwoProduct; d is a double-double quotient (2^-100). With v = d^2 <= 2^-18,
 * atan d = d + d v A, A = -1/3 + v B, B = 1/5 - v/7 + v^2/9 - v^3/11 in doubles
 * (omitted terms below 2^-110 of d). B's error is at most 2^-55, so A's at most
 * 2^-71.5 with the rounding of v B and the omitted low part of v, and atan d's
 * relative error at most 2^-89.5 + 2^-100. When j != 0, |atan d| <= atan c / 2,
 * so the sum cancels by half at most: 2^-88.8, with the table's 2^-106.
 */
function atanK(zh: number, zl: number): void {
  const j = Math.round(zh * 256);
  const cj = j * 0.00390625;
  // n = z - c: TwoSum(zh - c, zl).
  const n0 = zh - cj;
  let dh = n0 + zl;
  let s = dh - n0;
  let dl = n0 - (dh - s) + (zl - s);
  let c: number;
  if (j !== 0) {
    // 1 + z c = eh + el: TwoProduct(zh, c) with c unsplit (9 bits), FastTwoSum(1, p).
    c = SPLIT * zh;
    const zhh = c - (c - zh);
    const zhl = zh - zhh;
    const p = zh * cj;
    const pe = zhh * cj - p + zhl * cj;
    const eh = 1 + p;
    const el = p - (eh - 1) + pe + zl * cj;
    // d = n / (1 + z c).
    div(dh, dl, eh, el);
    dh = D[0];
    dl = D[1];
  }
  // v = d^2: TwoProduct(dh, dh) and 2 dh dl.
  c = SPLIT * dh;
  const dhh = c - (c - dh);
  const dhl = dh - dhh;
  const vh = dh * dh;
  const vl = dhh * dhh - vh + 2 * dhh * dhl + dhl * dhl + 2 * dh * dl;
  // A = -1/3 + v B: FastTwoSum(-1/3 high, v B), less 1/3 low.
  const B = 0.2 + vh * (-1 / 7 + vh * (1 / 9 - vh * (1 / 11)));
  const w = vh * B;
  const ah = w - THIRD_H;
  const al = w - (ah + THIRD_H) - THIRD_L;
  // v A.
  c = SPLIT * vh;
  const vhh = c - (c - vh);
  const vhl = vh - vhh;
  c = SPLIT * ah;
  const ahh = c - (c - ah);
  const ahl = ah - ahh;
  const bh = vh * ah;
  const bl = vhh * ahh - bh + vhh * ahl + vhl * ahh + vhl * ahl + (vh * al + vl * ah);
  // y = d v A.
  c = SPLIT * bh;
  const bhh = c - (c - bh);
  const bhl = bh - bhh;
  const yh = dh * bh;
  const yl = dhh * bhh - yh + dhh * bhl + dhl * bhh + dhl * bhl + (dh * bl + dl * bh);
  // atan d = d + y; |y| <= 2^-19 |d|, FastTwoSum.
  let h = dh + yh;
  let l = yh - (h - dh) + (dl + yl);
  if (j !== 0) {
    // atan c + atan d; atan c >= 2 |atan d|, FastTwoSum.
    const th = ATAN[2 * j];
    const h2 = th + h;
    l = h - (h2 - th) + (ATAN[2 * j + 1] + l);
    h = h2;
  }
  D[0] = h;
  D[1] = l;
}

/** (nh + nl) / (dh + dl) into D[0] + D[1]: the quotient of pairs (2^-100). */
function div(nh: number, nl: number, dh: number, dl: number): void {
  const qh = nh / dh;
  let c = SPLIT * qh;
  const qhh = c - (c - qh);
  const qhl = qh - qhh;
  c = SPLIT * dh;
  const dhh = c - (c - dh);
  const dhl = dh - dhh;
  const p = qh * dh;
  const pe = qhh * dhh - p + qhh * dhl + qhl * dhh + qhl * dhl;
  // nh - p is exact (Sterbenz): p is within a few units of nh.
  const ql = (nh - p - pe + nl - qh * dl) / dh;
  D[0] = qh + ql;
  D[1] = ql - (D[0] - qh);
}

/** a - (bh + bl) into D[0] + D[1] for a pair a = ah + al with |bh| <= |ah|: FastTwoSum. */
function sub(ah: number, al: number, bh: number, bl: number): void {
  const h = ah - bh;
  D[1] = -bh - (h - ah) + (al - bl);
  D[0] = h;
}

/**
 * atan x. Below 2^-28 in magnitude the result is x: atan x is between x -
 * x^3/3 and x, and x^3/3 < |x| 2^-55. Up to 1, atan |x|; up to 2^60, pi/2 -
 * atan(1/|x|), 1/|x| a double-double quotient (2^-106), and the difference
 * cancels by half at most: 2^-88; 2^-78 in the rounding test. Above 2^60,
 * atan |x| = pi/2 - 1/|x| + e, |e| < |x|^-3 < 2^-180: with pi/2's pair
 * (2^-105.4), 1/|x| (2^-113) and pi/2's low part less it rounded once
 * (2^-107), within 2^-104.4; 2^-92 in the rounding test.
 */
export function fastAtan(x: number): number {
  const ax = Math.abs(x);
  if (ax < P2_M28) return x;
  let h: number;
  let l: number;
  let err: number;
  if (ax <= 1) {
    atanK(ax, 0);
    h = D[0];
    l = D[1];
    err = Math.abs(h) * B78;
  } else if (ax < P2_60) {
    // 1/x as a pair: zh, and the remainder 1 - zh x (TwoProduct; 1 - p exact) over x.
    const zh = 1 / ax;
    let c = SPLIT * zh;
    const zhh = c - (c - zh);
    const zhl = zh - zhh;
    c = SPLIT * ax;
    const xh = c - (c - ax);
    const xl = ax - xh;
    const p = zh * ax;
    const pe = zhh * xh - p + zhh * xl + zhl * xh + zhl * xl;
    atanK(zh, (1 - p - pe) / ax);
    sub(PIO2_H, PIO2_L, D[0], D[1]);
    h = D[0];
    l = D[1];
    err = Math.abs(h) * B78;
  } else {
    h = PIO2_H;
    l = PIO2_L - 1 / ax;
    err = B92;
  }
  if (x < 0) {
    h = -h;
    l = -l;
  }
  return finish(h, l, err, 0);
}

/**
 * atan2(y, x). y = 0 gives pi (x < 0) or 0, x = 0 gives pi/2 with y's sign:
 * pi's and pi/2's nearest doubles. Otherwise, with num the smaller of |y| and
 * |x| and den the larger, both are scaled by one power of two (exact) to put
 * den in [1, 2). A ratio q = num/den below 2^-900 (atan q = q to 2^-2700)
 * gives pi - q when |y| < |x| and x < 0, and pi/2 - q or pi/2 + q when |y| >
 * |x| and x > 0 or x < 0: within 2^-104 with the pairs of pi and pi/2, and
 * 2^-92 in the test. For x > 0 and |y| < |x| it gives a result near the
 * subnormals, which the exact path decides. Otherwise q is a double-double
 * quotient (2^-100): atan q, then pi/2 less it when |y| > |x| and pi less
 * that when x < 0 (each cancels by half at most): 2^-88; 2^-78 in the test.
 */
export function fastAtan2(y: number, x: number): number {
  if (y === 0) return x < 0 ? PI_H : 0;
  if (x === 0) return y > 0 ? PIO2_H : -PIO2_H;
  const ay = Math.abs(y);
  const ax = Math.abs(x);
  const swap = ay > ax;
  let num = swap ? ax : ay;
  let den = swap ? ay : ax;
  if (den < P2_M1000) {
    num *= P2_600;
    den *= P2_600;
  }
  F64[0] = den;
  const E = ((U32[HI] >>> 20) & 0x7ff) - 1023;
  // Exact, unless num becomes subnormal: then q < 2^-1021 and only its size matters.
  num = ldexp(num, -E);
  den = ldexp(den, -E);
  let h: number;
  let l: number;
  let err: number;
  if (num < P2_M900) {
    if (!swap && x > 0) return NaN;
    const q = num / den;
    h = swap ? PIO2_H : PI_H;
    l = swap ? (x > 0 ? PIO2_L - q : PIO2_L + q) : PI_L - q;
    err = B92;
  } else {
    div(num, 0, den, 0);
    atanK(D[0], D[1]);
    if (swap) sub(PIO2_H, PIO2_L, D[0], D[1]);
    if (x < 0) sub(PI_H, PI_L, D[0], D[1]);
    h = D[0];
    l = D[1];
    err = Math.abs(h) * B78;
  }
  if (y < 0) {
    h = -h;
    l = -l;
  }
  return finish(h, l, err, 0);
}

/**
 * sqrt(1 - x^2) into G[6] + G[7] for |x| < 1, with a relative error below
 * 2^-103. 1 - x^2 is exact as a double-double: x^2 by TwoProduct, 1 less its
 * high part by TwoSum, and the low part added (one rounding of a term below
 * 2^-105 of the result). s = sqrt(high part), correctly rounded, and the
 * correction (w - s^2) / 2s with s^2 by TwoProduct.
 */
function sqrt1m(ax: number): void {
  // x^2 = p + pe: TwoProduct.
  let c = SPLIT * ax;
  const xh = c - (c - ax);
  const xl = ax - xh;
  const p = ax * ax;
  const pe = xh * xh - p + 2 * xh * xl + xl * xl;
  // w = 1 - p - pe: TwoSum(1, -p), then FastTwoSum with the low parts.
  const w0 = 1 - p;
  const t = w0 - 1;
  const wl0 = 1 - (w0 - t) + (-p - t) - pe;
  const wh = w0 + wl0;
  const wl = wl0 - (wh - w0);
  // s = sqrt(wh) and the correction (w - s^2) / 2s; wh - s^2 is exact (Sterbenz).
  const r = Math.sqrt(wh);
  c = SPLIT * r;
  const rh = c - (c - r);
  const rl = r - rh;
  const rr = r * r;
  const rre = rh * rh - rr + 2 * rh * rl + rl * rl;
  const cor = (wh - rr - rre + wl) / (2 * r);
  const h = r + cor;
  G[7] = cor - (h - r);
  G[6] = h;
}

/**
 * asin x for -1 <= x <= 1. Below 2^-28 in magnitude the result is x: asin x
 * is between x and x + x^3/5, and x^3/5 < |x| 2^-55. +-1 give pi/2's nearest
 * double with x's sign. Otherwise asin |x| = atan(|x| / s), s = sqrt(1 - x^2):
 * atan of the quotient when |x| <= s, else pi/2 - atan(s / |x|). The quotient
 * (2^-100) and s (2^-103) add below 2^-99 to atan's 2^-88.8; 2^-78 in the test.
 */
export function fastAsin(x: number): number {
  const ax = Math.abs(x);
  if (ax < P2_M28) return x;
  if (ax === 1) return x > 0 ? PIO2_H : -PIO2_H;
  sqrt1m(ax);
  if (ax <= G[6]) {
    div(ax, 0, G[6], G[7]);
    atanK(D[0], D[1]);
  } else {
    div(G[6], G[7], ax, 0);
    atanK(D[0], D[1]);
    sub(PIO2_H, PIO2_L, D[0], D[1]);
  }
  let h = D[0];
  let l = D[1];
  if (x < 0) {
    h = -h;
    l = -l;
  }
  return finish(h, l, Math.abs(h) * B78, 0);
}

/**
 * acos x for -1 <= x <= 1. 1, -1 and 0 give 0, pi and pi/2 (nearest doubles).
 * Below 2^-60 in magnitude, acos x = pi/2 - x - e, |e| < |x|^3 < 2^-180: with
 * pi/2's pair (2^-105.4) and its low part less x rounded once (2^-107), within
 * 2^-104.8; 2^-92 in the rounding test. Otherwise acos |x| = atan(s
 * / |x|), s = sqrt(1 - x^2): atan of the quotient when s <= |x|, else pi/2 -
 * atan(|x| / s); for x < 0, pi less that. 2^-88.5; 2^-78 in the test.
 */
export function fastAcos(x: number): number {
  if (x === 1) return 0;
  if (x === -1) return PI_H;
  if (x === 0) return PIO2_H;
  const ax = Math.abs(x);
  if (ax < P2_M60) return finish(PIO2_H, PIO2_L - x, B92, 0);
  sqrt1m(ax);
  if (G[6] <= ax) {
    div(G[6], G[7], ax, 0);
    atanK(D[0], D[1]);
  } else {
    div(ax, 0, G[6], G[7]);
    atanK(D[0], D[1]);
    sub(PIO2_H, PIO2_L, D[0], D[1]);
  }
  if (x < 0) sub(PI_H, PI_L, D[0], D[1]);
  return finish(D[0], D[1], Math.abs(D[0]) * B78, 0);
}

// ---- pow ----------------------------------------------------------------------

/**
 * x^y for x != 0, y != 0, and y an integer when x < 0 (f13 4.1 items 1 to 4).
 *
 * |x| = 1 gives 1, or -1 for x = -1 and an odd y. Otherwise exp(T), T = y ln|x|:
 * ln|x| within 2^-90 |ln|x||, times y by TwoProduct (2^-104), so T within
 * |T| 2^-89.9, which exp turns into a relative error of |T| 2^-89.9 (1 + 2^-60);
 * with exp's own 2^-91 the result is within 2^-91 + |T| 2^-89.9. The rounding
 * test uses 2^-78 + |T| 2^-80, a margin of 2^9.9 or more. The computed T is
 * within 2^-80 of T for |T| <= 746, so T's computed value of 710 or more gives
 * a result above 2^1024 (Infinity), and -745.5 or less one below 2^-1075 (0).
 * A T from -745.5 to -708, whose result is near or among the subnormals, and a
 * y of 2^64 or more go to the exact path; so do the values half way between
 * two doubles (a y with a fraction of a few bits), which no interval decides.
 * An exact value that is a double is decided: the interval holds it. Below
 * 2^-969 the product y ln|x| is not exact, but its error is then below 2^-1074,
 * which exp turns into a relative error below 2^-1073.
 */
export function fastPow(x: number, y: number): number {
  const neg = x < 0 && Number.isInteger(y) && Math.abs(y % 2) === 1;
  const ax = Math.abs(x);
  if (ax === 1) return neg ? -1 : 1;
  if (!(Math.abs(y) < P2_64)) return NaN;
  logKernel(ax);
  const lh = D[0];
  const ll = D[1];
  // T = y ln|x|: TwoProduct(y, lh), y ll, FastTwoSum.
  let c = SPLIT * y;
  const yh = c - (c - y);
  const yl = y - yh;
  c = SPLIT * lh;
  const lhh = c - (c - lh);
  const lhl = lh - lhh;
  const p = y * lh;
  const pe = yh * lhh - p + yh * lhl + yl * lhh + yl * lhl + y * ll;
  const th = p + pe;
  if (th >= 710) return neg ? -Infinity : Infinity;
  if (th <= -745.5) return 0;
  if (th <= -708) return NaN;
  const k = expKernel(th, pe - (th - p));
  const r = finish(D[0], D[1], Math.abs(D[0]) * (B78 + Math.abs(th) * B80), k);
  return neg ? -r : r;
}
