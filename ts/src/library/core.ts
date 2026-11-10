// The 53 entries of the Core library (04-library.md section 5).
//
// Each entry runs after its receiver and arguments are evaluated, after the
// tests of 2.6 and after the call's step (src/eval/compile.ts, callEntry).
// What follows is each entry's own order, as its record's "Order" gives it.
// Every failure an entry raises has the span of the whole call (ruling W4).

import { RunFailure } from "../eval/failure.ts";
import { L, render, sizeTogether } from "../eval/ops.ts";
import { isWhole, norm, parseDecimal } from "../value/number.ts";
import { ABSENT, RMap, compareText, cpLength, mapGet, mapKeys, mapSize, size, sortKeys, truthy } from "../value/value.ts";
import { orderedFailure } from "./args.ts";
import { implement } from "./registry.ts";
import type { Callback, CallContext } from "./registry.ts";
import { codePoints, cpOffsets, mapCase, occurrences, resolve, trimWhiteSpace } from "./text.ts";


// ---- shared ---------------------------------------------------------------

/** The elements of the receiver list, each read (ruling AC16). */
function elements(c: CallContext): unknown[] {
  const xs = c.receiver as unknown[];
  const out = new Array<unknown>(xs.length);
  for (let i = 0; i < xs.length; i++) out[i] = c.run.read(xs[i], c.span);
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
  const f = orderedFailure(xs.map((x) => c.run.kind(x)));
  if (f !== null) {
    c.run.charge(f.index + 1);
    throw new RunFailure("wrong-kind", c.span, "unordered", { name: c.name, kind: f.kind });
  }
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
  return Math.floor(w / 64);
}

function compareOrdered(a: unknown, b: unknown): number {
  if (typeof a === "number") return a - (b as number);
  return compareText(a as string, b as string);
}

/** A stable sort with O(n log n) comparisons: the platform's sort, which ECMAScript requires to be stable. */
function stableSort<T>(xs: T[], key: (x: T) => unknown): T[] {
  return xs
    .map((x, i) => ({ x, k: key(x), i }))
    .sort((p, q) => compareOrdered(p.k, q.k) || p.i - q.i)
    .map((p) => p.x);
}

/** Visit each element: 2 steps, then the callback (04 4.4, the rule for a callback). */
function each(c: CallContext, f: Callback, visit: (result: unknown, element: unknown, index: number) => boolean | void): number {
  const xs = c.receiver as unknown[];
  for (let i = 0; i < xs.length; i++) {
    c.run.charge(2);
    const e = c.run.read(xs[i], c.span);
    if (visit(f.call([e, i]), e, i) === false) return i + 1;
  }
  return xs.length;
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
  const s = render(c.run, x, c.span);
  c.run.charge(size(s));
  c.run.alloc(1 + size(s));
  return s;
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
  let parts: string[];
  if (sep === "") parts = codePoints(s);
  else {
    const at = occurrences(s, sep);
    parts = [];
    let from = 0;
    for (const i of at) {
      parts.push(s.slice(from, i));
      from = i + sep.length;
    }
    parts.push(s.slice(from));
  }
  const p = parts.length;
  c.run.charge(size(s) + p);
  c.run.alloc(1 + 2 * p + size(s));
  return parts;
});

implement("string", "replaceAll", (c) => {
  const s = c.receiver as string;
  const old = text(c, 0);
  const rep = text(c, 1);
  if (old === "") throw new RunFailure("invalid-argument", c.span, "empty-search", {});
  const at = occurrences(s, old);
  const weight = s.length + at.length * (rep.length - old.length);
  const sz = Math.floor(weight / 64);
  c.run.charge(size(s) + sz);
  c.run.alloc(1 + sz);
  const out: string[] = [];
  let from = 0;
  for (const i of at) {
    out.push(s.slice(from, i), rep);
    from = i + old.length;
  }
  out.push(s.slice(from));
  return out.join("");
});

implement("string", "slice", (c) => {
  const s = c.receiver as string;
  const offs = cpOffsets(s);
  const n = offs.length - 1;
  const a = resolve(num(c, 0), n);
  const b = c.args.length > 1 ? resolve(num(c, 1), n) : n;
  const r = a < b ? s.slice(offs[a], offs[b]) : "";
  c.run.charge(size(s));
  c.run.alloc(1 + size(r));
  return r;
});

implement("string", "repeat", (c) => {
  const s = c.receiver as string;
  const count = num(c, 0);
  if (count < 0) throw new RunFailure("invalid-argument", c.span, "negative-count", { count });
  const sz = Math.floor((count * s.length) / 64);
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
  // Read at most m + 1 code points of `fill`: work bounded by the result.
  const fcp: string[] = [];
  for (const cp of fill) {
    fcp.push(cp);
    if (fcp.length > m) break;
  }
  const full = fcp.length > m ? 0 : Math.floor(m / fcp.length);
  const rest = fcp.slice(0, fcp.length > m ? m : m % fcp.length).join("");
  const weight = s.length + full * fill.length + rest.length;
  const sz = Math.floor(weight / 64);
  c.run.charge(sz);
  c.run.alloc(1 + sz);
  const padding = fill.repeat(full) + rest;
  return atStart ? padding + s : s + padding;
}

implement("string", "padStart", (c) => pad(c, true));
implement("string", "padEnd", (c) => pad(c, false));

// ---- 5.4 lists ----------------------------------------------------------------

implement("list", "filter", (c) => {
  const out: unknown[] = [];
  each(c, c.args[0] as Callback, (r, e) => {
    if (truthy(r, c.run.cls)) out.push(e);
  });
  c.run.alloc(1 + out.length);
  return out;
});

implement("list", "map", (c) => {
  const out: unknown[] = [];
  each(c, c.args[0] as Callback, (r) => {
    out.push(r);
  });
  c.run.alloc(1 + out.length);
  return out;
});

implement("list", "flatMap", (c) => {
  const lists: unknown[][] = [];
  let k = 0;
  each(c, c.args[0] as Callback, (r) => {
    if (c.run.kind(r) !== "list") throw new RunFailure("wrong-kind", c.span, "result", { name: c.name, kind: c.run.kind(r) });
    lists.push(r as unknown[]);
    k += (r as unknown[]).length;
  });
  c.run.charge(k);
  c.run.alloc(1 + k);
  const out: unknown[] = [];
  for (const l of lists) for (const x of l) out.push(x);
  return out;
});

implement("list", "some", (c) => {
  let found = false;
  each(c, c.args[0] as Callback, (r) => {
    if (truthy(r, c.run.cls)) {
      found = true;
      return false;
    }
  });
  return found;
});

implement("list", "every", (c) => {
  let all = true;
  each(c, c.args[0] as Callback, (r) => {
    if (!truthy(r, c.run.cls)) {
      all = false;
      return false;
    }
  });
  return all;
});

implement("list", "find", (c) => {
  let found: unknown = null;
  each(c, c.args[0] as Callback, (r, e) => {
    if (truthy(r, c.run.cls)) {
      found = e;
      return false;
    }
  });
  return found;
});

implement("list", "count", (c) => {
  let n = 0;
  each(c, c.args[0] as Callback, (r) => {
    if (truthy(r, c.run.cls)) n++;
  });
  return n;
});

implement("list", "reduce", (c) => {
  const f = c.args[0] as Callback;
  let acc = c.args[1];
  const xs = c.receiver as unknown[];
  for (let i = 0; i < xs.length; i++) {
    c.run.charge(2);
    const e = c.run.read(xs[i], c.span);
    acc = f.call([acc, e, i]);
  }
  return acc;
});

implement("list", "sum", (c) => {
  const xs = elements(c);
  scan(c, xs, (v) => (c.run.kind(v) === "number" ? null : elementFail(c, v)));
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
  return stableSort(xs, (x) => x);
});

implement("list", "sortBy", (c) => {
  const keys: unknown[] = [];
  const xs: unknown[] = [];
  each(c, c.args[0] as Callback, (r, e) => {
    keys.push(r);
    xs.push(e);
  });
  // The ordered test of the keys: a failure charges nothing more (4.5 rule 4).
  const f = orderedFailure(keys.map((k) => c.run.kind(k)));
  if (f !== null) throw new RunFailure("wrong-kind", c.span, "unordered", { name: c.name, kind: f.kind });
  const n = xs.length;
  c.run.charge(L(n) * (n + stringsSize(keys)));
  c.run.alloc(1 + n);
  const idx = xs.map((_, i) => i);
  return stableSort(idx, (i) => keys[i]).map((i) => xs[i]);
});

implement("list", "groupBy", (c) => {
  const groups = new Map<string, unknown[]>();
  const keys: string[] = [];
  each(c, c.args[0] as Callback, (r, e) => {
    if (typeof r !== "string") throw new RunFailure("wrong-kind", c.span, "result", { name: c.name, kind: c.run.kind(r) });
    keys.push(r);
    const g = groups.get(r);
    if (g === undefined) groups.set(r, [e]);
    else g.push(e);
  });
  c.run.charge(sizeTogether(keys));
  c.run.alloc(1 + 3 * groups.size + keys.length);
  return new RMap(groups);
});

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
  const sz = Math.floor(w / 64);
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
