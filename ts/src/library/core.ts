// The 56 entries of the Core library (04-library.md section 5), and `walk`,
// the function of the group core that package F17 adds (f17 5).
//
// Each entry runs after its receiver and arguments are evaluated, after the
// tests of 2.6 and after the call's step (src/eval/compile.ts, callEntry).
// What follows is each entry's own order, as its record's "Order" gives it.
// Every failure an entry raises has the span of the whole call (04 2.6 item 6).

import { RunFailure } from "../eval/failure.ts";
import { L, Rendering, indexIn, render, renderedText, sizeTogether, textIndex } from "../eval/ops.ts";
import { isWhole, norm, parseDecimal } from "../value/number.ts";
import { ABSENT, RMap, compareText, cpLength, mapGet, mapKeys, mapSize, size, sortKeys, truthy, div64 } from "../value/value.ts";
import { orderedFailure } from "./args.ts";
import { implement } from "./registry.ts";
import type { Callback, CallContext } from "./registry.ts";
import { codePoints, cpOffsets, mapCase, occurrences, resolve, trimWhiteSpace } from "./text.ts";


// ---- shared ---------------------------------------------------------------

/** The elements of the receiver list, each read as a host value (03 3.4.4). */
function elements(c: CallContext): unknown[] {
  const xs = c.receiver as unknown[];
  const out = new Array<unknown>(xs.length);
  for (let i = 0; i < xs.length; i++) out[i] = readElement(c, xs[i]);
  return out;
}

/**
 * An element scan before the entry's charges (04 4.4 rule 1, 4.5): when the
 * test fails at element i, charge 1 step for each element tested (i + 1),
 * then raise the failure.
 */
function scan(c: CallContext, xs: unknown[], test: (v: unknown) => RunFailure | null): void {
  for (let i = 0; i < xs.length; i++) {
    const f = test(xs[i]);
    if (f !== null) {
      c.run.charge(i + 1);
      throw f;
    }
  }
}

/** The ordered test of 04 3.6 over the elements, with the failed-scan charge. */
function orderedScan(c: CallContext, xs: unknown[]): void {
  if (allNumbersOrAllStrings(xs)) return;
  const f = orderedFailure(xs.map((x) => c.run.kind(x)));
  if (f !== null) {
    c.run.charge(f.index + 1);
    throw new RunFailure("wrong-kind", c.span, "unordered", { name: c.name, kind: f.kind });
  }
}

/** Every value a number, or every value a string: an ordered set (04 3.6 rule 4). */
function allNumbersOrAllStrings(xs: unknown[]): boolean {
  if (xs.length === 0) return true;
  const t = typeof xs[0];
  if (t !== "number" && t !== "string") return false;
  for (let i = 1; i < xs.length; i++) if (typeof xs[i] !== t) return false;
  return true;
}

function elementFail(c: CallContext, v: unknown): RunFailure {
  return new RunFailure("wrong-kind", c.span, "element", { name: c.name, kind: c.run.kind(v) });
}

function rangeFail(c: CallContext): never {
  throw new RunFailure("number-range", c.span, "entry", { name: c.name });
}

/** The weights of the strings among values, taken together, as a size. */
function stringsSize(xs: unknown[]): number {
  let w = 0;
  for (const x of xs) if (typeof x === "string") w += x.length;
  return div64(w);
}

function compareOrdered(a: unknown, b: unknown): number {
  if (typeof a === "number") return a - (b as number);
  return compareText(a as string, b as string);
}

/**
 * The positions of ordered keys (04 3.6) sorted by key, ascending or (`desc`)
 * descending; equal keys keep their order. The order is total, so it does not
 * depend on the sort used. O(n log n) comparisons.
 */
function sortedPositions(keys: unknown[], desc: boolean): number[] {
  const n = keys.length;
  const idx = new Array<number>(n);
  for (let i = 0; i < n; i++) idx[i] = i;
  if (n > 0 && typeof keys[0] === "number") {
    // The ordered test makes every key a number when the first is.
    const k = new Float64Array(n);
    for (let i = 0; i < n; i++) k[i] = keys[i] as number;
    const sorted = mergeSortNumbers(k, desc);
    for (let i = 0; i < n; i++) idx[i] = sorted[i];
    return idx;
  }
  if (desc) idx.sort((p, q) => compareOrdered(keys[q], keys[p]) || p - q);
  else idx.sort((p, q) => compareOrdered(keys[p], keys[q]) || p - q);
  return idx;
}

/**
 * The positions 0 to n - 1 sorted by their keys `k`, ascending or (`desc`)
 * descending, equal keys in the order of their positions: a stable merge sort.
 */
function mergeSortNumbers(k: Float64Array, desc: boolean): Int32Array {
  const n = k.length;
  let a = new Int32Array(n);
  let b = new Int32Array(n);
  for (let i = 0; i < n; i++) a[i] = i;
  // Runs of RUN by insertion sort.
  const RUN = 16;
  for (let lo = 0; lo < n; lo += RUN) {
    const hi = Math.min(lo + RUN, n);
    for (let i = lo + 1; i < hi; i++) {
      const p = a[i];
      const kp = k[p];
      let j = i - 1;
      while (j >= lo && (desc ? k[a[j]] < kp : k[a[j]] > kp)) {
        a[j + 1] = a[j];
        j--;
      }
      a[j + 1] = p;
    }
  }
  for (let width = RUN; width < n; width *= 2) {
    for (let lo = 0; lo < n; lo += 2 * width) {
      const mid = Math.min(lo + width, n);
      const hi = Math.min(lo + 2 * width, n);
      let i = lo;
      let j = mid;
      let o = lo;
      while (i < mid && j < hi) {
        // Take from the right run only when its key comes strictly first.
        const ki = k[a[i]];
        const kj = k[a[j]];
        if (desc ? kj > ki : kj < ki) b[o++] = a[j++];
        else b[o++] = a[i++];
      }
      while (i < mid) b[o++] = a[i++];
      while (j < hi) b[o++] = a[j++];
    }
    const t = a;
    a = b;
    b = t;
  }
  return a;
}

/** A stable sort of ordered values. */
function stableSort(xs: unknown[]): unknown[] {
  const idx = sortedPositions(xs, false);
  const out = new Array<unknown>(idx.length);
  for (let i = 0; i < idx.length; i++) out[i] = xs[idx[i]];
  return out;
}

// STACK. An entry that takes a lambda visits each element itself: 2 steps,
// then the callback (04 4.4, the rule for a callback), in a loop in the
// entry's own function. A helper with that loop, or a callback the loop
// calls for each result, would put one more frame under every call of the
// lambda, and a recursion through the entry spends it at every level (see
// src/index.ts). What an entry does with the results after the loop is in
// helpers.

/** An element read as a host value (03 3.4.4). */
function readElement(c: CallContext, x: unknown): unknown {
  return typeof x === "object" ? x : c.run.read(x, c.span);
}

/** Truthiness (01 6.5). */
function isTrue(c: CallContext, v: unknown): boolean {
  return typeof v === "boolean" ? v : truthy(v, c.run.cls);
}

/**
 * The number of distinct keys a callback gave, which sets the size of a result
 * that is built after its units are charged (02 6.2 item 3). The set is scratch
 * space over keys that exist (6.2 item 6).
 */
function distinct(keys: unknown[]): number {
  return new Set(keys).size;
}

/** A callback's result of the wrong kind. */
function resultFail(c: CallContext, r: unknown): RunFailure {
  return new RunFailure("wrong-kind", c.span, "result", { name: c.name, kind: c.run.kind(r) });
}

/**
 * An entry that calls its lambda for each element and keeps each element and
 * its result, into `keys` and `xs`; then `finish`. (The results go to arrays
 * that `finish` is given: returned in a new object, they made the optimizer
 * deoptimize code compiled during the loop at each return.)
 */
function keyedEntry(finish: (c: CallContext, keys: unknown[], xs: unknown[]) => unknown): (c: CallContext) => unknown {
  return (c) => {
    const f = c.args[0] as Callback;
    const recv = c.receiver as unknown[];
    const keys: unknown[] = [];
    const xs: unknown[] = [];
    for (let i = 0; i < recv.length; i++) {
      c.run.charge(2);
      const e = readElement(c, recv[i]);
      xs.push(e);
      keys.push(f.call2(e, i));
    }
    return finish(c, keys, xs);
  };
}

/** keyedEntry for a callback that must give a string, tested when its call returns; the keys' size is charged after the last (f07 6.2). */
function stringKeyedEntry(finish: (c: CallContext, keys: string[], xs: unknown[]) => unknown): (c: CallContext) => unknown {
  return (c) => {
    const f = c.args[0] as Callback;
    const recv = c.receiver as unknown[];
    const keys: string[] = [];
    const xs: unknown[] = [];
    for (let i = 0; i < recv.length; i++) {
      c.run.charge(2);
      const e = readElement(c, recv[i]);
      const r = f.call2(e, i);
      if (typeof r !== "string") throw resultFail(c, r);
      keys.push(r);
      xs.push(e);
    }
    c.run.charge(sizeTogether(keys));
    return finish(c, keys, xs);
  };
}

const text = (c: CallContext, i: number) => c.args[i] as string;
const num = (c: CallContext, i: number) => c.args[i] as number;

// ---- 5.1 conversion, inspection and errors ---------------------------------

implement(null, "len", (c) => {
  const x = c.args[0];
  switch (c.run.kind(x)) {
    case "string":
      c.run.charge(size(x as string));
      return cpLength(x as string);
    case "list":
      return (x as unknown[]).length;
    default:
      return mapSize(x as object);
  }
});

implement(null, "string", (c) => {
  const x = c.args[0];
  if (c.run.kind(x) === "string") return x;
  const out = new Rendering();
  render(c.run, x, c.span, out);
  return renderedText(c.run, out, (full) => render(c.run, x, c.span, full));
});

const JSON_GRAMMAR = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$/;

/** Read text as a number, for `int` and `float` (04 3.10 rule 1). */
function readNumber(c: CallContext, s: string): number {
  const t = trimWhiteSpace(s);
  if (!JSON_GRAMMAR.test(t)) throw new RunFailure("invalid-argument", c.span, "not-a-number", { text: s });
  const x = parseDecimal(t)!;
  if (!Number.isFinite(x)) throw new RunFailure("invalid-argument", c.span, "not-finite", { text: s });
  return norm(x);
}

implement(null, "int", (c) => {
  const x = c.args[0];
  const isText = typeof x === "string";
  c.run.charge(isText ? size(x) : 0);
  const v = isText ? readNumber(c, x) : (x as number);
  const t = norm(Math.trunc(v));
  if (!isWhole(t)) rangeFail(c);
  return t;
});

implement(null, "float", (c) => {
  const x = c.args[0];
  if (typeof x !== "string") return x;
  c.run.charge(size(x));
  return readNumber(c, x);
});

implement(null, "bool", (c) => truthy(c.args[0], c.run.cls));

implement(null, "type", (c) => c.run.kind(c.args[0]));

implement(null, "fail", (c) => {
  throw new RunFailure("failed", c.span, null, {}, { text: text(c, 0), data: c.args.length > 1 ? c.args[1] : null });
});

// ---- 5.2 numbers --------------------------------------------------------------

implement(null, "abs", (c) => norm(Math.abs(num(c, 0))));

for (const [name, f] of [["floor", Math.floor], ["ceil", Math.ceil]] as const) {
  implement(null, name, (c) => {
    const r = norm(f(num(c, 0)));
    if (!isWhole(r)) rangeFail(c);
    return r;
  });
}

implement(null, "round", (c) => {
  const x = num(c, 0);
  // Half away from zero, of the exact value: Math.round rounds the exact
  // value, with ties toward +Infinity.
  const r = norm(x >= 0 ? Math.round(x) : -Math.round(-x));
  if (!isWhole(r)) rangeFail(c);
  return r;
});

for (const [name, sign] of [["min", -1], ["max", 1]] as const) {
  implement(null, name, (c) => {
    const args = c.args;
    c.run.charge(args.length - 1 + stringsSize(args));
    let best = args[0];
    for (let i = 1; i < args.length; i++) if (compareOrdered(args[i], best) * sign > 0) best = args[i];
    return best;
  });
}

implement(null, "range", (c) => {
  const a = c.args.length > 1 ? num(c, 0) : 0;
  const b = c.args.length > 1 ? num(c, 1) : num(c, 0);
  const k = Math.max(0, b - a);
  c.run.charge(k);
  c.run.alloc(1 + k);
  const out = new Array<number>(k);
  for (let i = 0; i < k; i++) out[i] = a + i;
  return out;
});

implement(null, "fixed", (c) => {
  const x = num(c, 0);
  const digits = num(c, 1);
  if (digits < 0 || digits > 20) throw new RunFailure("invalid-argument", c.span, "bad-digits", { digits });
  if (Math.abs(x) >= 1e21) throw new RunFailure("invalid-argument", c.span, "too-large", { x });
  c.run.alloc(1);
  // Number.prototype.toFixed is exact in ECMAScript for |x| < 1e21 and up to
  // 100 digits: the exact value of x, a tie toward the larger magnitude.
  let s = Math.abs(x).toFixed(digits);
  if (x < 0 && /[1-9]/.test(s)) s = "-" + s;
  return s;
});

// ---- 5.3 strings -------------------------------------------------------------

for (const [name, upper] of [["toUpperCase", true], ["toLowerCase", false]] as const) {
  implement("string", name, (c) => {
    const s = c.receiver as string;
    c.run.charge(size(s));
    c.run.alloc(1 + size(s));
    return mapCase(s, upper);
  });
}

implement("string", "trim", (c) => {
  const s = c.receiver as string;
  const r = trimWhiteSpace(s);
  c.run.charge(size(s));
  c.run.alloc(1 + size(r));
  return r;
});

function startsWith(s: string, p: string): boolean {
  return p.length <= s.length && s.startsWith(p);
}
function endsWith(s: string, p: string): boolean {
  return p.length <= s.length && s.endsWith(p);
}

implement("string", "removePrefix", (c) => {
  const s = c.receiver as string;
  const p = text(c, 0);
  const r = startsWith(s, p) ? s.slice(p.length) : s;
  c.run.charge(size(s));
  c.run.alloc(1 + size(r));
  return r;
});

implement("string", "removeSuffix", (c) => {
  const s = c.receiver as string;
  const p = text(c, 0);
  const r = endsWith(s, p) ? s.slice(0, s.length - p.length) : s;
  c.run.charge(size(s));
  c.run.alloc(1 + size(r));
  return r;
});

implement("string", "startsWith", (c) => {
  const p = text(c, 0);
  c.run.charge(size(p));
  return startsWith(c.receiver as string, p);
});

implement("string", "endsWith", (c) => {
  const p = text(c, 0);
  c.run.charge(size(p));
  return endsWith(c.receiver as string, p);
});

implement("string", "split", (c) => {
  const s = c.receiver as string;
  const sep = text(c, 0);
  const sz = size(s);
  c.run.charge(sz);
  let p: number;
  if (sep === "") {
    p = cpLength(s);
  } else {
    p = 1;
    let i = 0;
    for (;;) {
      const k = s.indexOf(sep, i);
      if (k < 0) break;
      p++;
      i = k + sep.length;
    }
  }
  c.run.charge(p);
  c.run.alloc(1 + 2 * p + sz);
  if (sep === "") return codePoints(s);
  const parts: string[] = [];
  let from = 0;
  for (;;) {
    const k = s.indexOf(sep, from);
    if (k < 0) break;
    parts.push(s.slice(from, k));
    from = k + sep.length;
  }
  parts.push(s.slice(from));
  return parts;
});

implement("string", "replaceAll", (c) => {
  const s = c.receiver as string;
  const old = text(c, 0);
  const rep = text(c, 1);
  if (old === "") throw new RunFailure("invalid-argument", c.span, "empty-search", {});
  const szS = size(s);
  c.run.charge(szS);
  let count = 0;
  let idx = 0;
  for (;;) {
    const k = s.indexOf(old, idx);
    if (k < 0) break;
    count++;
    idx = k + old.length;
  }
  const weight = s.length + count * (rep.length - old.length);
  const sz = div64(weight);
  c.run.charge(sz);
  c.run.alloc(1 + sz);
  if (count === 0) return s;
  const out: string[] = [];
  let from = 0;
  for (;;) {
    const k = s.indexOf(old, from);
    if (k < 0) break;
    out.push(s.slice(from, k), rep);
    from = k + old.length;
  }
  out.push(s.slice(from));
  return out.join("");
});

implement("string", "indexOf", (c) => {
  const s = c.receiver as string;
  c.run.charge(size(s));
  const i = textIndex(s, text(c, 0));
  return i < 0 ? null : cpLength(s.slice(0, i));
});

implement("string", "slice", (c) => {
  const s = c.receiver as string;
  c.run.charge(size(s));
  const n = cpLength(s);
  const a = resolve(num(c, 0), n);
  const b = c.args.length > 1 ? resolve(num(c, 1), n) : n;
  let r = "";
  if (a < b) {
    let startUnit = 0;
    let endUnit = s.length;
    let cp = 0;
    for (let u = 0; u < s.length; ) {
      if (cp === a) startUnit = u;
      if (cp === b) {
        endUnit = u;
        break;
      }
      const code = s.charCodeAt(u);
      u += code >= 0xd800 && code <= 0xdbff && u + 1 < s.length ? 2 : 1;
      cp++;
    }
    r = s.slice(startUnit, endUnit);
  }
  c.run.alloc(1 + size(r));
  return r;
});

implement("string", "repeat", (c) => {
  const s = c.receiver as string;
  const count = num(c, 0);
  if (count < 0) throw new RunFailure("invalid-argument", c.span, "negative-count", { count });
  const sz = div64(count * s.length);
  c.run.charge(sz);
  c.run.alloc(1 + sz);
  return s.repeat(count);
});

function pad(c: CallContext, atStart: boolean): string {
  const s = c.receiver as string;
  const length = num(c, 0);
  const fill = c.args.length > 1 ? text(c, 1) : " ";
  const n = cpLength(s);
  if (n >= length || fill === "") {
    c.run.charge(size(s));
    c.run.alloc(1 + size(s));
    return s;
  }
  const m = length - n;
  // Read at most m code points of `fill` without allocating (02 6.2 item 3).
  let fillCps = 0;
  let prefixUnits = fill.length;
  for (let u = 0; u < fill.length; ) {
    if (fillCps === m) {
      prefixUnits = u;
      break;
    }
    const code = fill.charCodeAt(u);
    u += code >= 0xd800 && code <= 0xdbff && u + 1 < fill.length ? 2 : 1;
    fillCps++;
  }
  let full = 0;
  let restUnits = prefixUnits;
  if (fillCps < m) {
    full = Math.floor(m / fillCps);
    const remCps = m % fillCps;
    restUnits = 0;
    let cp = 0;
    for (let u = 0; u < fill.length && cp < remCps; ) {
      const code = fill.charCodeAt(u);
      u += code >= 0xd800 && code <= 0xdbff && u + 1 < fill.length ? 2 : 1;
      cp++;
      restUnits = u;
    }
  }
  const weight = s.length + full * fill.length + restUnits;
  const sz = div64(weight);
  c.run.charge(sz);
  c.run.alloc(1 + sz);
  const padding = (full > 0 ? fill.repeat(full) : "") + fill.slice(0, restUnits);
  return atStart ? padding + s : s + padding;
}

implement("string", "padStart", (c) => pad(c, true));
implement("string", "padEnd", (c) => pad(c, false));

// ---- 5.4 lists ----------------------------------------------------------------

implement("list", "filter", (c) => {
  const f = c.args[0] as Callback;
  const xs = c.receiver as unknown[];
  const out: unknown[] = [];
  for (let i = 0; i < xs.length; i++) {
    c.run.charge(2);
    const e = readElement(c, xs[i]);
    if (isTrue(c, f.call2(e, i))) out.push(e);
  }
  c.run.alloc(1 + out.length);
  return out;
});

implement("list", "map", (c) => {
  const f = c.args[0] as Callback;
  const xs = c.receiver as unknown[];
  const out: unknown[] = [];
  for (let i = 0; i < xs.length; i++) {
    c.run.charge(2);
    const e = readElement(c, xs[i]);
    out.push(f.call2(e, i));
  }
  c.run.alloc(1 + out.length);
  return out;
});

implement("list", "flatMap", (c) => {
  const f = c.args[0] as Callback;
  const xs = c.receiver as unknown[];
  const lists: unknown[][] = [];
  for (let i = 0; i < xs.length; i++) {
    c.run.charge(2);
    const e = readElement(c, xs[i]);
    const r = f.call2(e, i);
    if (c.run.kind(r) !== "list") throw resultFail(c, r);
    lists.push(r as unknown[]);
  }
  return concatLists(c, lists);
});

/**
 * Package F17 (f17 5): the nodes of a tree in pre-order. The place of the walk
 * is a stack of its own (5.3): for each level entered, its list of children
 * and the position of the next. The loop calls the lambda from this frame for
 * every node, so a walk costs no native stack for its levels. The call for a
 * node of depth d adds d to the depth count, beside the measure of the call
 * (5.4); its index in the trace is the node's position in pre-order, which is
 * the number of calls before it.
 */
implement(null, "walk", (c) => {
  const r = c.run;
  const f = c.args[1] as Callback;
  const out: unknown[] = [];
  const lists: unknown[][] = [];
  const next: number[] = [];
  // The root is the argument, read where it was evaluated (5.2 item 5).
  let node = c.args[0];
  let depth = 0;
  for (;;) {
    r.charge(2);
    if (depth > 0) node = readElement(c, node);
    out.push(node);
    r.depth += depth;
    const kids = f.call2(node, depth);
    r.depth -= depth;
    const k = r.kind(kids);
    if (k === "list") {
      if ((kids as unknown[]).length > 0) {
        lists.push(kids as unknown[]);
        next.push(0);
      }
    } else if (k !== "null") throw resultFail(c, kids);
    // The next node in pre-order: the next child of the deepest level that has one.
    let top = lists.length - 1;
    while (top >= 0 && next[top] === lists[top].length) {
      lists.pop();
      next.pop();
      top--;
    }
    if (top < 0) break;
    node = lists[top][next[top]++];
    depth = top + 1;
  }
  r.alloc(1 + out.length);
  return out;
});

/** flatMap's results, the lists, as one list: k steps and 1 + k units for k elements in all. */
function concatLists(c: CallContext, lists: unknown[][]): unknown[] {
  let k = 0;
  for (const l of lists) k += l.length;
  c.run.charge(k);
  c.run.alloc(1 + k);
  const out: unknown[] = [];
  for (const l of lists) for (const x of l) out.push(x);
  return out;
}

/**
 * An entry that seeks the first element whose callback result is truthy
 * (`want` true) or falsy (`want` false), and gives `finish` its position, or -1.
 */
function seekEntry(want: boolean, finish: (c: CallContext, i: number) => unknown): (c: CallContext) => unknown {
  return (c) => {
    const f = c.args[0] as Callback;
    const xs = c.receiver as unknown[];
    for (let i = 0; i < xs.length; i++) {
      c.run.charge(2);
      const e = readElement(c, xs[i]);
      if (isTrue(c, f.call2(e, i)) === want) return finish(c, i);
    }
    return finish(c, -1);
  };
}

implement("list", "some", seekEntry(true, (_, i) => i >= 0));

implement("list", "every", seekEntry(false, (_, i) => i < 0));

implement("list", "find", seekEntry(true, (c, i) => (i < 0 ? null : readElement(c, (c.receiver as unknown[])[i]))));

implement("list", "findIndex", seekEntry(true, (_, i) => (i < 0 ? null : i)));

implement("list", "indexOf", (c) => {
  const i = indexIn(c.run, c.args[0], c.receiver as unknown[], c.span);
  return i < 0 ? null : i;
});

implement("list", "count", (c) => {
  const f = c.args[0] as Callback;
  const xs = c.receiver as unknown[];
  let n = 0;
  for (let i = 0; i < xs.length; i++) {
    c.run.charge(2);
    const e = readElement(c, xs[i]);
    if (isTrue(c, f.call2(e, i))) n++;
  }
  return n;
});

implement("list", "reduce", (c) => {
  const f = c.args[0] as Callback;
  let acc = c.args[1];
  const xs = c.receiver as unknown[];
  for (let i = 0; i < xs.length; i++) {
    c.run.charge(2);
    const e = readElement(c, xs[i]);
    acc = f.call3(acc, e, i);
  }
  return acc;
});

implement("list", "sum", (c) => {
  const xs = elements(c);
  for (let i = 0; i < xs.length; i++) {
    const v = xs[i];
    if (typeof v !== "number" && c.run.kind(v) !== "number") {
      // The scan's charge (4.4 rule 1): the elements tested.
      c.run.charge(i + 1);
      throw elementFail(c, v);
    }
  }
  c.run.charge(xs.length);
  let acc = 0;
  for (const x of xs) {
    const y = x as number;
    const r = acc + y;
    if (isWhole(acc) && isWhole(y) ? !Number.isSafeInteger(r) : !Number.isFinite(r)) rangeFail(c);
    acc = norm(r);
  }
  return acc;
});

for (const [name, sign] of [["min", -1], ["max", 1]] as const) {
  implement("list", name, (c) => {
    const xs = elements(c);
    orderedScan(c, xs);
    c.run.charge(xs.length + stringsSize(xs));
    if (xs.length === 0) return null;
    let best = xs[0];
    for (let i = 1; i < xs.length; i++) if (compareOrdered(xs[i], best) * sign > 0) best = xs[i];
    return best;
  });
}

implement("list", "sort", (c) => {
  const xs = elements(c);
  orderedScan(c, xs);
  const n = xs.length;
  c.run.charge(n + L(n) * (n + stringsSize(xs)));
  c.run.alloc(1 + n);
  return stableSort(xs);
});

implement(
  "list",
  "sortBy",
  keyedEntry((c, keys, xs) => {
    // The ordered test of the keys: a failure charges nothing more (4.5 rule 4).
    const f = allNumbersOrAllStrings(keys) ? null : orderedFailure(keys.map((k) => c.run.kind(k)));
    if (f !== null) throw new RunFailure("wrong-kind", c.span, "unordered", { name: c.name, kind: f.kind });
    const n = xs.length;
    c.run.charge(L(n) * (n + stringsSize(keys)));
    c.run.alloc(1 + n);
    // f07 6.3: with "desc", descending, and still stable.
    const idx = sortedPositions(keys, c.args[1] === "desc");
    const out = new Array<unknown>(n);
    for (let i = 0; i < n; i++) out[i] = xs[idx[i]];
    return out;
  }),
);

implement(
  "list",
  "groupBy",
  stringKeyedEntry((c, keys, xs) => {
    // 5.4.14: the units, then the build (02 6.2 item 3).
    c.run.alloc(1 + 3 * distinct(keys) + keys.length);
    const groups = new Map<string, unknown[]>();
    for (let i = 0; i < keys.length; i++) {
      const g = groups.get(keys[i]);
      if (g === undefined) groups.set(keys[i], [xs[i]]);
      else g.push(xs[i]);
    }
    return new RMap(groups);
  }),
);

implement("list", "toMap", (c) => {
  const xs = elements(c);
  const keys: string[] = [];
  const values: unknown[] = [];
  scan(c, xs, (v) => {
    if (c.run.kind(v) !== "map") return elementFail(c, v);
    const k = mapGet(v as object, "key");
    if (k === ABSENT) return new RunFailure("missing-field", c.span, "field", { name: "key" });
    const kr = c.run.read(k, c.span);
    if (typeof kr !== "string") return new RunFailure("wrong-kind", c.span, "key", { found: kr });
    const val = mapGet(v as object, "value");
    if (val === ABSENT) return new RunFailure("missing-field", c.span, "field", { name: "value" });
    keys.push(kr);
    values.push(val);
    return null;
  });
  const n = xs.length;
  c.run.charge(n + sizeTogether(keys));
  c.run.alloc(1 + 2 * n);
  const m = new RMap();
  keys.forEach((k, i) => m.set(k, values[i]));
  return m;
});

implement("list", "unique", (c) => {
  const xs = elements(c);
  orderedScan(c, xs);
  c.run.charge(xs.length + stringsSize(xs));
  const seen = new Set<unknown>();
  const out: unknown[] = [];
  for (const x of xs) {
    if (!seen.has(x)) {
      seen.add(x);
      out.push(x);
    }
  }
  c.run.alloc(1 + out.length);
  return out;
});

implement("list", "flat", (c) => {
  const xs = elements(c);
  scan(c, xs, (v) => (c.run.kind(v) === "list" ? null : elementFail(c, v)));
  let k = 0;
  for (const x of xs) k += (x as unknown[]).length;
  c.run.charge(xs.length + k);
  c.run.alloc(1 + k);
  const out: unknown[] = [];
  for (const x of xs) for (const y of x as unknown[]) out.push(y);
  return out;
});

implement("list", "reverse", (c) => {
  const xs = c.receiver as unknown[];
  c.run.charge(xs.length);
  c.run.alloc(1 + xs.length);
  return xs.slice().reverse();
});

implement("list", "first", (c) => {
  const xs = c.receiver as unknown[];
  return xs.length === 0 ? null : c.run.read(xs[0], c.span);
});

implement("list", "last", (c) => {
  const xs = c.receiver as unknown[];
  return xs.length === 0 ? null : c.run.read(xs[xs.length - 1], c.span);
});

implement("list", "slice", (c) => {
  const xs = c.receiver as unknown[];
  const n = xs.length;
  const a = resolve(num(c, 0), n);
  const b = c.args.length > 1 ? resolve(num(c, 1), n) : n;
  const k = Math.max(0, b - a);
  c.run.charge(k);
  c.run.alloc(1 + k);
  return xs.slice(a, a + k);
});

implement("list", "join", (c) => {
  const xs = elements(c);
  const sep = text(c, 0);
  scan(c, xs, (v) => (typeof v === "string" ? null : elementFail(c, v)));
  let w = 0;
  for (const x of xs) w += (x as string).length;
  if (xs.length > 1) w += (xs.length - 1) * sep.length;
  const sz = div64(w);
  c.run.charge(xs.length + sz);
  c.run.alloc(1 + sz);
  return (xs as string[]).join(sep);
});

// ---- 5.5 maps ------------------------------------------------------------------

function sortedKeys(c: CallContext): string[] {
  const keys = sortKeys(mapKeys(c.receiver as object));
  const n = keys.length;
  c.run.charge(n + L(n) * (n + sizeTogether(keys)));
  return keys;
}

implement("map", "keys", (c) => {
  const keys = sortedKeys(c);
  c.run.alloc(1 + keys.length);
  return keys;
});

implement("map", "values", (c) => {
  const keys = sortedKeys(c);
  c.run.alloc(1 + keys.length);
  return keys.map((k) => mapGet(c.receiver as object, k));
});

implement("map", "entries", (c) => {
  const keys = sortedKeys(c);
  c.run.alloc(1 + 6 * keys.length);
  return keys.map((k) => new RMap([["key", k], ["value", mapGet(c.receiver as object, k)]]));
});

// ---- package F7: the methods of queries (f07 6) ----------------------------------

/** The ordered test of the keys a callback gave, after the last call, and the charge for their strings (f07 6.2). */
function orderedKeys(c: CallContext, keys: unknown[]): void {
  const f = allNumbersOrAllStrings(keys) ? null : orderedFailure(keys.map((k) => c.run.kind(k)));
  if (f !== null) throw new RunFailure("wrong-kind", c.span, "unordered", { name: c.name, kind: f.kind });
  c.run.charge(stringsSize(keys));
}

for (const [name, sign] of [["minBy", -1], ["maxBy", 1]] as const) {
  implement(
    "list",
    name,
    keyedEntry((c, keys, xs) => {
      orderedKeys(c, keys);
      if (xs.length === 0) return null;
      // Of equal keys, the first (f07 6.4 item 3, 6.5 item 1).
      let best = 0;
      for (let i = 1; i < xs.length; i++) if (compareOrdered(keys[i], keys[best]) * sign > 0) best = i;
      return xs[best];
    }),
  );
}

implement(
  "list",
  "uniqueBy",
  keyedEntry((c, keys, xs) => {
    orderedKeys(c, keys);
    // The units, then the build (02 6.2 item 3). `1` and `1.0` are one key: numbers are held as doubles.
    c.run.alloc(1 + distinct(keys));
    const seen = new Set<unknown>();
    const out: unknown[] = [];
    keys.forEach((k, i) => {
      if (!seen.has(k)) {
        seen.add(k);
        out.push(xs[i]);
      }
    });
    return out;
  }),
);

implement(
  "list",
  "indexBy",
  stringKeyedEntry((c, keys, xs) => {
    c.run.alloc(1 + 2 * xs.length);
    const m = new RMap();
    keys.forEach((k, i) => m.set(k, xs[i]));
    return m;
  }),
);

implement(
  "list",
  "countBy",
  stringKeyedEntry((c, keys) => {
    // The units, then the build (02 6.2 item 3).
    c.run.alloc(1 + 2 * distinct(keys));
    const m = new RMap();
    for (const k of keys) m.set(k, ((m.get(k) as number | undefined) ?? 0) + 1);
    return m;
  }),
);

implement("map", "mapValues", (c) => {
  // f07 6.9: the keys in key order, charged first; then each entry: 2 steps, then f(value, key).
  const keys = keysInOrder(c);
  const f = c.args[0] as Callback;
  const out: unknown[] = [];
  for (let i = 0; i < keys.length; i++) {
    c.run.charge(2);
    const v = valueAt(c, keys[i]);
    out.push(f.call2(v, keys[i]));
  }
  return mapOf(c, keys, out);
});

/** The value under a key of the receiver map, read as a host value (03 3.4.4). */
function valueAt(c: CallContext, k: string): unknown {
  return c.run.read(mapGet(c.receiver as object, k), c.span);
}

/** mapValues: the receiver's keys in key order, and their charge. */
function keysInOrder(c: CallContext): string[] {
  const keys = sortKeys(mapKeys(c.receiver as object));
  const n = keys.length;
  c.run.charge(L(n) * (n + sizeTogether(keys)));
  return keys;
}

/** mapValues: the map of each key to its result, after its units. */
function mapOf(c: CallContext, keys: string[], out: unknown[]): RMap {
  c.run.alloc(1 + 2 * keys.length);
  const m = new RMap();
  keys.forEach((k, i) => m.set(k, out[i]));
  return m;
}
