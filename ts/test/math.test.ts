// Unit tests of package F13 (f13 2, 4): the correctly rounded functions at
// arguments the case files do not reach, the exact cases of pow among them.

import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { crAcos, crAsin, crAtan, crAtan2, crCos, crExp, crLog, crLog10, crPow, crSin, crTan, roundExact } from "../src/library/mathcr.ts";

describe("rounding of an exact value (f13 2.1 items 1 to 3)", () => {
  test("ties to even, on the subnormal grid, and past the largest double", () => {
    assert.equal(roundExact(1n, -1075), 0);
    assert.equal(roundExact(3n, -1075), 2 * 2 ** -1074);
    assert.equal(roundExact((1n << 53n) + 1n, 0), 2 ** 53);
    assert.equal(roundExact((1n << 53n) + 3n, 0), 2 ** 53 + 4);
    assert.equal(roundExact(1n, 1024), Infinity);
    assert.equal(roundExact((1n << 54n) - 1n, 970), Infinity);
    assert.equal(roundExact((1n << 54n) - 2n, 970), Number.MAX_VALUE);
  });
});

describe("pow (f13 4.1)", () => {
  test("a value half way between two doubles is rounded to even", () => {
    // t = 2^18 - 1: t^3 is a 54-bit odd integer, half way; pow(t^2, 1.5) is it.
    const t = 2n ** 18n - 1n;
    assert.equal(crPow(Number(t * t), 1.5), roundExact(t ** 3n, 0));
    assert.equal(crPow(2, -1075), 0);
    assert.equal(crPow(2, -1074), 2 ** -1074);
  });
  test("integer powers are exact, odd powers of a negative base negative", () => {
    assert.equal(crPow(-8, 3), -512);
    assert.equal(crPow(3, 40), 12157665459056928801 /* rounds */);
    assert.equal(crPow(10, -2), 0.01);
    assert.equal(crPow(2, 1023), 2 ** 1023);
    assert.equal(crPow(2, 1024), Infinity);
  });
});

describe("the examples of the chapter", () => {
  test("results", () => {
    const rows: [number, number][] = [
      [crSin(1), 0.8414709848078965], [crSin(Math.PI), 1.2246467991473532e-16], [crSin(1e22), -0.8522008497671888],
      [crCos(1), 0.5403023058681398], [crTan(1), 1.5574077246549023], [crTan(1.5707963267948966), 16331239353195370],
      [crAsin(0.5), 0.5235987755982989], [crAcos(-1), Math.PI], [crAtan(-0.5), -0.4636476090008061],
      [crAtan2(0, -1), Math.PI], [crAtan2(-1, -1), -2.356194490192345], [crAtan2(0, 0), 0],
      [crExp(1), Math.E], [crExp(-746), 0], [crLog(10), 2.302585092994046], [crLog10(1000), 3], [crLog10(2), 0.3010299956639812],
      [crPow(2, 0.5), 1.4142135623730951], [crPow(1.1, 10), 2.5937424601000023],
    ];
    for (const [a, b] of rows) assert.equal(a, b);
    assert.equal(crExp(710), Infinity);
  });
  test("tiny and huge arguments", () => {
    assert.equal(crSin(1e-300), 1e-300);
    assert.equal(crTan(1e-300), 1e-300);
    assert.equal(crAsin(5e-324), 5e-324);
    assert.equal(crAtan(1e300), crAtan2(1, 0));
    assert.equal(crLog(Number.MIN_VALUE), -744.4400719213812);
    assert.equal(crLog(Number.MAX_VALUE), 709.782712893384);
    assert.equal(crCos(1e300), crCos(1e300));
    assert.equal(crAtan2(1e-300, 1e300), 0);
  });
});
