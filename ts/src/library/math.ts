// The eleven entries of package F13 (f13 4): correctly rounded functions of
// the group `math`. Each runs after the tests of 04 2.6 and the call's step;
// then its value test (f13 3 item 2), which finds `not-real` from the
// arguments alone; then its 200 steps; then the result, by mathcr.ts.

import { RunFailure } from "../eval/failure.ts";
import { norm } from "../value/number.ts";
import { crAcos, crAsin, crAtan, crAtan2, crCos, crExp, crLog, crLog10, crPow, crSin, crTan } from "./mathcr.ts";
import { implement } from "./registry.ts";
import type { CallContext } from "./registry.ts";

/** The cost of each entry beyond the call's step (f13 3). */
const STEPS = 200;

function entry(name: string, notReal: (args: number[]) => boolean, f: (args: number[]) => number): void {
  implement(null, name, (c: CallContext) => {
    const args = c.args as number[];
    if (notReal(args)) throw new RunFailure("invalid-argument", c.span, "not-real", { name });
    c.run.charge(STEPS);
    const r = f(args);
    // f13 2.1 item 2: an exact value that rounds to 2^1024 or more is not a number.
    if (!Number.isFinite(r)) throw new RunFailure("number-range", c.span, "entry", { name });
    return norm(r);
  });
}

const never = () => false;

// f13 4.1: y zero gives 1; x zero gives 0, or not-real for y below zero; x below zero needs a whole y.
entry(
  "math.pow",
  ([x, y]) => y !== 0 && ((x === 0 && y < 0) || (x < 0 && !Number.isInteger(y))),
  ([x, y]) => (y === 0 ? 1 : x === 0 ? 0 : crPow(x, y)),
);
entry("math.exp", never, ([x]) => crExp(x));
entry("math.log", ([x]) => x <= 0, ([x]) => crLog(x));
entry("math.log10", ([x]) => x <= 0, ([x]) => crLog10(x));
entry("math.sin", never, ([x]) => crSin(x));
entry("math.cos", never, ([x]) => crCos(x));
entry("math.tan", never, ([x]) => crTan(x));
entry("math.asin", ([x]) => x < -1 || x > 1, ([x]) => crAsin(x));
entry("math.acos", ([x]) => x < -1 || x > 1, ([x]) => crAcos(x));
entry("math.atan", never, ([x]) => crAtan(x));
entry("math.atan2", never, ([y, x]) => crAtan2(y, x));
