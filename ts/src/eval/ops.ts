// Operators and walks, with their charges (01 6, 12; 02 3, 5.3 to 5.5).

import type { Span } from "../syntax/source.ts";
import { isWhole, norm, numberToText } from "../value/number.ts";
import { ABSENT, compareText, jsonString, lcpWeight, mapGet, mapKeys, mapSize, size, sortKeys, truthy, div64 } from "../value/value.ts";
import { RunFailure } from "./failure.ts";
import type { Run } from "./run.ts";

/** L(n) of 02 1.2. */
export function L(n: number): number {
  if (n < 2) return 0;
  let l = 0;
  while (2 ** l < n) l++;
  return l;
}

/** The size of several strings taken together (02 4 item 3). */
export function sizeTogether(xs: string[]): number {
  let w = 0;
  for (const x of xs) w += x.length;
  return div64(w);
}

export type ArithOp = "+" | "-" | "*" | "/" | "%" | "**";

function rangeFail(op: string, span: Span): never {
  throw new RunFailure("number-range", span, "operator", { op });
}

/** Multiply, following the row of `a * b` for whole operands (01 6.2). */
function mulChecked(x: number, y: number, whole: boolean, op: string, span: Span): number {
  const p = x * y;
  if (whole && !Number.isSafeInteger(p)) rangeFail(op, span);
  return p;
}

/** `a ** b`, `b` whole (01 6.2). */
function power(a: number, b: number, span: Span): number {
  if (b >= 0) {
    const whole = isWhole(a);
    let r = 1;
    const bits = b.toString(2);
    for (const bit of bits) {
      r = mulChecked(r, r, whole, "**", span);
      if (bit === "1") r = mulChecked(r, a, whole, "**", span);
    }
    if (!Number.isFinite(r)) rangeFail("**", span);
    return norm(r);
  }
  if (a === 0) throw new RunFailure("divide-by-zero", span, "power", { op: "**" });
  let p = 1;
  for (const bit of (-b).toString(2)) {
    p = p * p;
    if (bit === "1") p = p * a;
  }
  if (!Number.isFinite(p)) return 0;
  if (p === 0) rangeFail("**", span);
  const r = 1 / p;
  if (!Number.isFinite(r)) rangeFail("**", span);
  return norm(r);
}

/** An arithmetic operator on two values (01 6.2, 6.4; 02 2.5, 3.2). */
export function arith(r: Run, op: ArithOp, a: unknown, b: unknown, span: Span): unknown {
  const ka = r.kind(a);
  const kb = r.kind(b);
  if (ka === "null" || kb === "null") throw new RunFailure("null-operand", span, "operand", { op });
  if (op === "+" && ka === "string" && kb === "string") {
    const s = a as string;
    const t = b as string;
    const sz = div64(s.length + t.length);
    r.charge(1 + sz);
    r.alloc(1 + sz);
    return s + t;
  }
  if (ka !== "number" || kb !== "number") {
    throw new RunFailure("wrong-kind", span, "operands", { op, left: ka, right: kb });
  }
  const x = a as number;
  const y = b as number;
  if (op === "**" && !isWhole(y)) throw new RunFailure("wrong-kind", span, "whole", { found: y });
  r.charge(1);
  const whole = isWhole(x) && isWhole(y);
  let res: number;
  switch (op) {
    case "+":
      res = x + y;
      break;
    case "-":
      res = x - y;
      break;
    case "*":
      res = x * y;
      break;
    case "/":
      if (y === 0) throw new RunFailure("divide-by-zero", span, "divisor", { op });
      res = x / y;
      if (!Number.isFinite(res)) rangeFail(op, span);
      return norm(res);
    case "%":
      if (y === 0) throw new RunFailure("divide-by-zero", span, "divisor", { op });
      return norm(x % y);
    case "**":
      return power(x, y, span);
  }
  if (whole ? !Number.isSafeInteger(res) : !Number.isFinite(res)) rangeFail(op, span);
  return norm(res);
}

export function negate(r: Run, a: unknown, span: Span): unknown {
  const k = r.kind(a);
  if (k === "null") throw new RunFailure("null-operand", span, "operand", { op: "-" });
  if (k !== "number") throw new RunFailure("wrong-kind", span, "operand", { op: "-", kind: k });
  r.charge(1);
  return norm(-(a as number));
}

export type OrderOp = "<" | "<=" | ">" | ">=";

export function order(r: Run, op: OrderOp, a: unknown, b: unknown, span: Span): boolean {
  const ka = r.kind(a);
  const kb = r.kind(b);
  if (ka === "null" || kb === "null") throw new RunFailure("null-operand", span, "operand", { op });
  let c: number;
  if (ka === "number" && kb === "number") {
    r.charge(1);
    c = (a as number) - (b as number);
  } else if (ka === "string" && kb === "string") {
    r.charge(1 + size(String(a).slice(0, lcpWeight(a as string, b as string))));
    c = compareText(a as string, b as string);
  } else {
    throw new RunFailure("wrong-kind", span, "operands", { op, left: ka, right: kb });
  }
  switch (op) {
    case "<": return c < 0;
    case "<=": return c <= 0;
    case ">": return c > 0;
    case ">=": return c >= 0;
  }
}

/** `==` with its charges (02 5.3). `level` is the walk's levels entered so far. */
export function equal(r: Run, a: unknown, b: unknown, span: Span, level = 0): boolean {
  const ka = r.kind(a);
  const kb = r.kind(b);
  if (ka !== kb) {
    r.charge(1);
    return false;
  }
  switch (ka) {
    case "null":
      r.charge(1);
      return true;
    case "bool":
    case "number":
    case "opaque":
      r.charge(1);
      return a === b;
    case "string":
      r.charge(1 + div64(lcpWeight(a as string, b as string)));
      return a === b;
    case "list": {
      const x = a as unknown[];
      const y = b as unknown[];
      if (x.length !== y.length) {
        r.charge(1);
        return false;
      }
      // 02 8.4: an empty pair is entered too.
      r.testDepth(r.depth + level + 1);
      for (let i = 0; i < x.length; i++) {
        r.charge(1);
        if (!equal(r, r.read(x[i], span), r.read(y[i], span), span, level + 1)) return false;
      }
      return true;
    }
    case "map": {
      const n = mapSize(a as object);
      if (n !== mapSize(b as object)) {
        r.charge(1);
        return false;
      }
      r.testDepth(r.depth + level + 1);
      const keys = sortKeys(mapKeys(a as object));
      r.charge(n + sizeTogether(keys));
      let same = true;
      for (const k of keys) {
        const w = mapGet(b as object, k);
        if (w === ABSENT) {
          same = false;
          continue;
        }
        if (!equal(r, r.read(mapGet(a as object, k), span), r.read(w, span), span, level + 1)) same = false;
      }
      return same;
    }
  }
}

/** A search for x in s whose work is bounded by 1 + w(s) (02 8.5). */
export function textContains(s: string, x: string): boolean {
  if (x.length > s.length) return false;
  if (x.length < 64) return s.includes(x);
  // Knuth-Morris-Pratt over UTF-16 units: a match of well-formed text is a match of code points.
  const fail = new Int32Array(x.length);
  for (let i = 1, k = 0; i < x.length; i++) {
    while (k > 0 && x.charCodeAt(i) !== x.charCodeAt(k)) k = fail[k - 1];
    if (x.charCodeAt(i) === x.charCodeAt(k)) k++;
    fail[i] = k;
  }
  for (let i = 0, k = 0; i < s.length; i++) {
    while (k > 0 && s.charCodeAt(i) !== x.charCodeAt(k)) k = fail[k - 1];
    if (s.charCodeAt(i) === x.charCodeAt(k)) k++;
    if (k === x.length) return true;
  }
  return false;
}

/** `x in y` with its charges (01 6.8, 02 5.3). `op` is `in` or `not in`. */
export function contains(r: Run, op: string, x: unknown, y: unknown, span: Span): boolean {
  const ky = r.kind(y);
  if (ky === "null") throw new RunFailure("null-operand", span, "operand", { op });
  const kx = r.kind(x);
  if (ky === "list") {
    const ys = y as unknown[];
    for (let i = 0; i < ys.length; i++) {
      const e = r.read(ys[i], span);
      const ke = r.kind(e);
      r.charge(1);
      if (kx !== ke) continue;
      if (kx === "string") {
        r.charge(div64(lcpWeight(x as string, e as string)));
        if (x === e) return true;
      } else if (kx === "list") {
        if ((x as unknown[]).length === (e as unknown[]).length) {
          if (equal(r, x, e, span)) return true;
        }
      } else if (kx === "map") {
        if (mapSize(x as object) === mapSize(e as object)) {
          if (equal(r, x, e, span)) return true;
        }
      } else if (kx === "null" || x === e) {
        return true;
      }
    }
    return false;
  }
  if (ky === "string" || ky === "map") {
    if (kx === "null") throw new RunFailure("null-operand", span, "operand", { op });
    if (kx !== "string") throw new RunFailure("wrong-kind", span, "operands", { op, left: kx, right: ky });
    if (ky === "string") {
      r.charge(1 + size(y as string));
      return textContains(y as string, x as string);
    }
    r.charge(1 + size(x as string));
    return mapGet(y as object, x as string) !== ABSENT;
  }
  throw new RunFailure("wrong-kind", span, "operands", { op, left: kx, right: ky });
}

/**
 * Text being rendered (01 12), and its length in UTF-16 units. The string's
 * units come before it is built (02 6.2 item 3), so the JSON form of a long
 * string is put off: `text` makes it, after the charges.
 */
export class Rendering {
  /** The text before each long string put off, and that string. */
  private readonly before: [string, string][] = [];
  private last = "";
  length = 0;

  add(s: string): void {
    this.last += s;
    this.length += s.length;
  }

  /** The JSON form of `s`. */
  quote(s: string): void {
    if (s.length <= 256) return this.add(jsonString(s));
    this.before.push([this.last, s]);
    this.last = "";
    let n = s.length + 2;
    for (let i = 0; i < s.length; i++) {
      const c = s.charCodeAt(i);
      if (c === 0x22 || c === 0x5c || c === 0x08 || c === 0x0c || c === 0x0a || c === 0x0d || c === 0x09) n += 1;
      else if (c < 0x20) n += 5;
    }
    this.length += n;
  }

  /** The string, once its charges are made. */
  text(): string {
    let out = "";
    for (const [t, s] of this.before) out += t + jsonString(s);
    return out + this.last;
  }
}

/**
 * Renders a value into `out` (01 12), with the walk's charges (02 5.5). An
 * opaque value fails with `wrong-kind`, template `text`, at `span`.
 */
export function render(r: Run, v: unknown, span: Span, out: Rendering): void {
  switch (r.kind(v)) {
    case "string":
      return out.add(v as string);
    case "null":
      return;
    case "bool":
      return out.add(String(v));
    case "number":
      return out.add(numberToText(v as number));
    case "opaque":
      throw new RunFailure("wrong-kind", span, "text", {});
  }
  jsonForm(r, v, span, 0, out);
}

/** The JSON form of a value, made in a run: `json.stringify` (f06 6.1). An opaque value is `wrong-kind`, template `json`. */
export function jsonText(r: Run, v: unknown, span: Span, out: Rendering): void {
  jsonForm(r, v, span, 0, out, "json");
}

/** The string a library entry renders: the steps of its size, then its units, then the string (04 4.4). */
export function renderedText(r: Run, out: Rendering): string {
  const sz = div64(out.length);
  r.charge(sz);
  r.alloc(1 + sz);
  return out.text();
}

function jsonForm(r: Run, v: unknown, span: Span, level: number, out: Rendering, opaqueTemplate = "text"): void {
  switch (r.kind(v)) {
    case "string":
      out.quote(v as string);
      return;
    case "null":
      out.add("null");
      return;
    case "bool":
      out.add(String(v));
      return;
    case "number":
      out.add(numberToText(v as number));
      return;
    case "opaque":
      throw new RunFailure("wrong-kind", span, opaqueTemplate, {});
    case "list": {
      const xs = v as unknown[];
      r.testDepth(r.depth + level + 1);
      r.charge(xs.length);
      out.add("[");
      for (let i = 0; i < xs.length; i++) {
        if (i > 0) out.add(",");
        jsonForm(r, r.read(xs[i], span), span, level + 1, out, opaqueTemplate);
      }
      out.add("]");
      return;
    }
    case "map": {
      r.testDepth(r.depth + level + 1);
      const keys = sortKeys(mapKeys(v as object));
      const n = keys.length;
      r.charge(n + L(n) * (n + sizeTogether(keys)));
      out.add("{");
      keys.forEach((k, i) => {
        if (i > 0) out.add(",");
        out.quote(k);
        out.add(":");
        jsonForm(r, r.read(mapGet(v as object, k), span), span, level + 1, out, opaqueTemplate);
      });
      out.add("}");
      return;
    }
  }
}

export function isTruthy(r: Run, v: unknown): boolean {
  return truthy(v, r.cls);
}
