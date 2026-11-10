// Text helpers of the library (04 3.5, 3.8, 3.9).

import { CASEMAP_UNICODE, LOWER, UPPER } from "../generated/casemap.ts";

export { CASEMAP_UNICODE };

const UPPER_MAP = new Map<number, number>(UPPER);
const LOWER_MAP = new Map<number, number>(LOWER);

/** Map each code point by the case table built from suite/casemap.json (04 3.8). */
export function mapCase(s: string, upper: boolean): string {
  const m = upper ? UPPER_MAP : LOWER_MAP;
  const out: string[] = [];
  let from = 0;
  for (let i = 0; i < s.length; i++) {
    let c = s.charCodeAt(i);
    let n = 1;
    if (c >= 0xd800 && c <= 0xdbff && i + 1 < s.length) {
      c = ((c - 0xd800) << 10) + (s.charCodeAt(i + 1) - 0xdc00) + 0x10000;
      n = 2;
    }
    const to = m.get(c);
    if (to !== undefined) {
      out.push(s.slice(from, i), String.fromCodePoint(to));
      from = i + n;
    }
    i += n - 1;
  }
  out.push(s.slice(from));
  return out.join("");
}

/** The 25 code points of white space (04 3.9). */
export function isWhiteSpace(c: number): boolean {
  return (
    (c >= 0x09 && c <= 0x0d) || c === 0x20 || c === 0xa0 || c === 0x1680 || (c >= 0x2000 && c <= 0x200a) ||
    c === 0x2028 || c === 0x2029 || c === 0x202f || c === 0x205f || c === 0x3000 || c === 0xfeff
  );
}

/** Remove white space from both ends. Every white space code point is in the BMP. */
export function trimWhiteSpace(s: string): string {
  let a = 0;
  let b = s.length;
  while (a < b && isWhiteSpace(s.charCodeAt(a))) a++;
  while (b > a && isWhiteSpace(s.charCodeAt(b - 1))) b--;
  return s.slice(a, b);
}

/**
 * The positions, in UTF-16 units, of the occurrences of a non-empty `t` in
 * `s`, left to right without overlap (04 3.5 rule 4). Work is bounded by a
 * constant times 1 + w(s) (rule 5): the platform's search for `t` under 64 in
 * weight, a search of its own (Knuth-Morris-Pratt) otherwise. A match of
 * UTF-16 units in well-formed text is a match of code points.
 */
export function occurrences(s: string, t: string): number[] {
  const out: number[] = [];
  if (t.length > s.length) return out;
  if (t.length < 64) {
    for (let i = s.indexOf(t); i !== -1; i = s.indexOf(t, i + t.length)) out.push(i);
    return out;
  }
  const fail = new Int32Array(t.length);
  for (let i = 1, k = 0; i < t.length; i++) {
    while (k > 0 && t.charCodeAt(i) !== t.charCodeAt(k)) k = fail[k - 1];
    if (t.charCodeAt(i) === t.charCodeAt(k)) k++;
    fail[i] = k;
  }
  for (let i = 0, k = 0; i < s.length; i++) {
    while (k > 0 && s.charCodeAt(i) !== t.charCodeAt(k)) k = fail[k - 1];
    if (s.charCodeAt(i) === t.charCodeAt(k)) k++;
    if (k === t.length) {
      out.push(i - t.length + 1);
      k = 0; // no overlap: start again after the occurrence
    }
  }
  return out;
}

/** The code points of a string, each as a string. */
export function codePoints(s: string): string[] {
  return Array.from(s);
}

/** UTF-16 offset of each code point position, and the end: offsets[i] for i in 0..len. */
export function cpOffsets(s: string): number[] {
  const out: number[] = [];
  for (let i = 0; i < s.length; i++) {
    out.push(i);
    const c = s.charCodeAt(i);
    if (c >= 0xd800 && c <= 0xdbff) i++;
  }
  out.push(s.length);
  return out;
}

/** Resolve a position against a length (04 3.10 rule 4). */
export function resolve(p: number, len: number): number {
  if (p < 0) p += len;
  return p < 0 ? 0 : p > len ? len : p;
}
