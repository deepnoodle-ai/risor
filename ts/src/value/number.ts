// Numbers.
//
// REPRESENTATION. A Risor number is a JavaScript `number`, and nothing else.
//
// The language has one kind of number: a finite binary64 value that is not
// negative zero (01 6.1, 6.2). A "whole number" is one with no fractional
// part and a magnitude of at most 2^53 - 1 (01 1.1, 6.2 rule 3), which is
// exactly `Number.isSafeInteger`. So a `number` holds every value of the
// language exactly, and no BigInt is needed at run time:
//
// - `a + b`, `a - b`, `a * b` with both operands whole: the exact result is
//   whole exactly when the binary64 result is a safe integer. If the exact
//   result is at most 2^53 - 1 in magnitude it is representable, so the
//   binary64 operation is exact; if it is larger, the rounded result is at
//   least 2^53 (rounding is monotonic and 2^53 is representable). So the
//   rule "fails with number-range when not whole" is
//   `!Number.isSafeInteger(r)`.
// - `a % b`: binary64 remainder is exact (IEEE 754 fmod), the sign of `a`.
// - `-0` never escapes: every operation that can give it passes its result
//   through `norm` (`x + 0` turns -0 into +0 and changes nothing else).
// - NaN and the infinities never escape: a result that is not finite is
//   `number-range` (01 6.2 rule 6).
// - Number to text is ECMAScript's Number::toString (01 6.3 rule 1), which is
//   `String(x)` in every conforming engine.
// - Text to number (01 6.3 rule 2) checks the JSON grammar first and then uses
//   `parseDecimal`: `Number(text)` up to 20 significant digits, which is what
//   ECMAScript promises to round correctly, and exact BigInt arithmetic past
//   that (ruling T6).
// - Integer literals are range-checked with BigInt in the lexer only, because
//   a literal such as 9007199254740993 must be `literal-range`, and `Number()`
//   of its text would round it to a value that looks in range.

/** The largest whole number, 2^53 - 1. */
export const MAX_WHOLE = Number.MAX_SAFE_INTEGER;

/** One zero: negative zero becomes zero. */
export function norm(x: number): number {
  return x + 0;
}

/** A whole number (01 6.2 rule 3). */
export function isWhole(x: number): boolean {
  return Number.isSafeInteger(x);
}

/** Number to text (01 6.3 rule 1). */
export function numberToText(x: number): string {
  return String(norm(x));
}

const JSON_NUMBER = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$/;

/**
 * Text to number (01 6.3 rule 2): exactly the JSON number grammar. Gives
 * null when the text does not match or its value is not finite.
 */
export function textToNumber(text: string): number | null {
  if (!JSON_NUMBER.test(text)) return null;
  const x = parseDecimal(text)!;
  if (!Number.isFinite(x)) return null;
  return norm(x);
}

// ---- Exact decimal to binary64 (ruling T6) ---------------------------------
//
// ECMAScript promises a correctly rounded `Number(text)` only up to 20
// significant digits. Past that, this conversion is used: BigInt arithmetic,
// round to nearest, ties to even, subnormals included.

function bitLength(n: bigint): number {
  return n === 0n ? 0 : n.toString(2).length;
}

/** x * 2^n, exact when the result is a normal number or zero. */
function ldexp(x: number, n: number): number {
  while (n > 1000) {
    x *= 2 ** 1000;
    n -= 1000;
  }
  while (n < -1000) {
    x *= 2 ** -1000;
    n += 1000;
  }
  return x * 2 ** n;
}

/** round(num / den) to an integer, ties to even. */
function divRound(num: bigint, den: bigint): bigint {
  const q = num / den;
  const r2 = (num % den) * 2n;
  if (r2 > den || (r2 === den && (q & 1n) === 1n)) return q + 1n;
  return q;
}

/** The binary64 value nearest to digits × 10^exp10, ties to even. Infinity when too large. */
export function decimalToDouble(digits: bigint, exp10: number): number {
  if (digits === 0n) return 0;
  const nd = digits.toString().length;
  if (nd + exp10 > 310) return Infinity;
  if (nd + exp10 < -330) return 0;
  if (exp10 >= 0) return Number(digits * 10n ** BigInt(exp10)); // BigInt to Number rounds correctly
  const den = 10n ** BigInt(-exp10);
  const e2 = bitLength(digits) - bitLength(den);
  if (e2 < -1000) {
    // Maybe subnormal: the value in units of 2^-1074.
    const m = divRound(digits << 1074n, den);
    if (m < 2n ** 53n) return Number(m) * 5e-324;
  }
  // Scale so the quotient has at least 65 significant bits, then add a
  // sticky bit below them: Number() then rounds once, correctly.
  const s = 66 - e2;
  const num = s >= 0 ? digits << BigInt(s) : digits;
  const d = s >= 0 ? den : den << BigInt(-s);
  const q = num / d;
  const sticky = num % d === 0n ? 0n : 1n;
  return ldexp(Number(q * 2n + sticky), -(s + 1));
}

const DECIMAL_TEXT = /^(-?)([0-9]+)(?:\.([0-9]+))?(?:[eE]([+-]?[0-9]+))?$/;

/**
 * The nearest binary64 value of decimal text `[-]digits[.digits][e[+-]digits]`,
 * correctly rounded at any length. Infinity when too large. null when the text
 * is not of that form.
 */
export function parseDecimal(text: string): number | null {
  const m = DECIMAL_TEXT.exec(text);
  if (m === null) return null;
  const intPart = m[2];
  const frac = m[3] ?? "";
  const significant = (intPart + frac).replace(/^0+/, "").length;
  let x: number;
  if (significant <= 20) {
    x = Number(text);
  } else {
    const expText = m[4] ?? "0";
    const e = Number(expText);
    if (!Number.isFinite(e) || Math.abs(e) > 1e9) {
      x = expText.startsWith("-") ? 0 : Infinity;
    } else {
      // A binary64 value is decided by at most 768 significant digits: keep
      // 800, and a sticky 1 when a digit dropped is not zero, so the work
      // stays in proportion to the text (02 8.5).
      let digits = (intPart + frac).replace(/^0+/, "");
      let exp = e - frac.length;
      if (digits.length > 800) {
        const dropped = digits.slice(800);
        exp += dropped.length;
        digits = digits.slice(0, 800);
        if (/[1-9]/.test(dropped)) {
          digits += "1";
          exp -= 1;
        }
      }
      x = decimalToDouble(BigInt(digits), exp);
    }
    if (m[1] === "-") x = -x;
  }
  return x + 0;
}
