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
//   `Number(text)`. ECMAScript lets an engine round a decimal with more than
//   20 significant digits inexactly; V8 rounds correctly (questions file).
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
  const x = Number(text);
  if (!Number.isFinite(x)) return null;
  return norm(x);
}
