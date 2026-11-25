// The entries of the four opt-in groups of package F6 (f06 6 to 9): `json`,
// `time`, `math` and `base64`.
//
// Each entry runs after its arguments are evaluated, after the tests of
// 04 2.6 and after the call's step, as every library entry does. None of them
// is the platform's own function: JSON text, times, durations and base64 are
// read and written here by the chapter's rules.

import { RunFailure } from "../eval/failure.ts";
import { Rendering, jsonText, renderedText } from "../eval/ops.ts";
import { norm, numberToText, parseDecimal } from "../value/number.ts";
import { RMap, size, div64 } from "../value/value.ts";
import { implement } from "./registry.ts";
import type { CallContext } from "./registry.ts";

// ---- json (f06 6) ---------------------------------------------------------------

implement(null, "json.stringify", (c) => {
  const out = new Rendering();
  jsonText(c.run, c.args[0], c.span, out);
  return renderedText(c.run, out, (full) => jsonText(c.run, c.args[0], c.span, full));
});

implement(null, "json.parse", (c) => {
  const s = c.args[0] as string;
  c.run.charge(8 * size(s));
  // Pass 1: validate grammar, find bad numbers, depth, value count and units
  // before allocating the parsed value tree (02 6.2 item 3).
  const r = scanJson(s, false);
  if (r === null) throw new RunFailure("invalid-argument", c.span, "not-json", {});
  // f06 6.2.4: the first bad number in the text, then the depth.
  if (r.numberFailure === "integer-range") throw new RunFailure("invalid-argument", c.span, "integer-range", {});
  if (r.numberFailure === "number-range") throw new RunFailure("number-range", c.span, "entry", { name: "json.parse" });
  c.run.testDepth(c.run.depth + r.depth);
  c.run.charge(r.values);
  c.run.alloc(r.units);
  return scanJson(s, true)!.value;
});

interface Scanned {
  value: unknown;
  /** `c` of f06 3.4: the values of the text. */
  values: number;
  /** `u` of f06 6.2.6. */
  units: number;
  /** The greatest level of an array or object (f06 6.2.5). */
  depth: number;
  numberFailure: "integer-range" | "number-range" | null;
}

type Frame =
  | { list: unknown[]; len: number }
  | { map: RMap | null; keys: Set<string> | null; key: string };

const NUMBER = /-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/y;

/**
 * One pass over a JSON text (f06 6.2.1 to 6.2.6), with an explicit stack so
 * that the host's stack does not grow with the nesting. When `build` is false,
 * measures values, units, depth and number validity without allocating values.
 * null: not JSON.
 */
function scanJson(s: string, build: boolean): Scanned | null {
  const n = s.length;
  let i = 0;
  let values = 0;
  let units = 0;
  let depth = 0;
  let numberFailure: Scanned["numberFailure"] = null;
  const stack: Frame[] = [];
  const ws = () => {
    while (i < n) {
      const ch = s.charCodeAt(i);
      if (ch === 0x20 || ch === 0x09 || ch === 0x0a || ch === 0x0d) i++;
      else break;
    }
  };
  const hex4 = (at: number): number => {
    if (at + 4 > n) return -1;
    let v = 0;
    for (let k = at; k < at + 4; k++) {
      const ch = s.charCodeAt(k);
      const d = ch >= 0x30 && ch <= 0x39 ? ch - 0x30 : ch >= 0x61 && ch <= 0x66 ? ch - 0x57 : ch >= 0x41 && ch <= 0x46 ? ch - 0x37 : -1;
      if (d < 0) return -1;
      v = v * 16 + d;
    }
    return v;
  };
  /** Measure a string at i (on its quote) without building it; returns UTF-16 length or -1. */
  const strLen = (): number => {
    i++;
    let len = 0;
    for (;;) {
      if (i >= n) return -1;
      const ch = s.charCodeAt(i);
      if (ch === 0x22) {
        i++;
        return len;
      }
      if (ch < 0x20) return -1;
      if (ch !== 0x5c) {
        i++;
        len++;
        continue;
      }
      const e = s.charCodeAt(i + 1);
      switch (e) {
        case 0x22: case 0x5c: case 0x2f: case 0x62: case 0x66: case 0x6e: case 0x72: case 0x74:
          i += 2;
          len += 1;
          break;
        case 0x75: {
          const v = hex4(i + 2);
          if (v < 0) return -1;
          i += 6;
          if (v >= 0xd800 && v <= 0xdbff && s.charCodeAt(i) === 0x5c && s.charCodeAt(i + 1) === 0x75) {
            const w = hex4(i + 2);
            if (w >= 0xdc00 && w <= 0xdfff) {
              i += 6;
              len += 2;
              break;
            }
          }
          len += 1;
          break;
        }
        default:
          return -1;
      }
    }
  };
  /** A string at i (on its quote); null when it is not one. */
  const str = (): string | null => {
    i++;
    let out = "";
    let from = i;
    for (;;) {
      if (i >= n) return null;
      const ch = s.charCodeAt(i);
      if (ch === 0x22) {
        out += s.slice(from, i);
        i++;
        return out;
      }
      if (ch < 0x20) return null;
      if (ch !== 0x5c) {
        i++;
        continue;
      }
      out += s.slice(from, i);
      const e = s.charCodeAt(i + 1);
      switch (e) {
        case 0x22: out += '"'; i += 2; break;
        case 0x5c: out += "\\"; i += 2; break;
        case 0x2f: out += "/"; i += 2; break;
        case 0x62: out += "\b"; i += 2; break;
        case 0x66: out += "\f"; i += 2; break;
        case 0x6e: out += "\n"; i += 2; break;
        case 0x72: out += "\r"; i += 2; break;
        case 0x74: out += "\t"; i += 2; break;
        case 0x75: {
          const v = hex4(i + 2);
          if (v < 0) return null;
          i += 6;
          if (v >= 0xd800 && v <= 0xdbff && s.charCodeAt(i) === 0x5c && s.charCodeAt(i + 1) === 0x75) {
            const w = hex4(i + 2);
            if (w >= 0xdc00 && w <= 0xdfff) {
              out += String.fromCharCode(v, w);
              i += 6;
              break;
            }
          }
          // f06 6.2.3: a surrogate escape in any other place is U+FFFD.
          out += v >= 0xd800 && v <= 0xdfff ? "�" : String.fromCharCode(v);
          break;
        }
        default:
          return null;
      }
      from = i;
    }
  };
  /** A number at i; its value, with the first failure of 6.2.3 noted. null when it is not one. */
  const num = (): number | null => {
    NUMBER.lastIndex = i;
    const m = NUMBER.exec(s);
    if (m === null || m[0].length === 0 || m[0] === "-") return null;
    const text = m[0];
    i += text.length;
    if (!/[.eE]/.test(text)) {
      const digits = text.startsWith("-") ? text.slice(1) : text;
      if (digits.length > 16 || (digits.length === 16 && digits > "9007199254740991")) {
        numberFailure ??= "integer-range";
        return 0;
      }
      return build ? norm(Number(text)) : 0;
    }
    const x = parseDecimal(text)!;
    if (!Number.isFinite(x)) {
      numberFailure ??= "number-range";
      return 0;
    }
    return build ? norm(x) : 0;
  };
  const key = (): string | null => {
    if (s.charCodeAt(i) !== 0x22) return null;
    const k = str();
    if (k === null) return null;
    ws();
    if (s.charCodeAt(i) !== 0x3a) return null;
    i++;
    ws();
    return k;
  };
  ws();
  let top: unknown;
  outer: for (;;) {
    // A value at i.
    let v: unknown;
    const ch = s.charCodeAt(i);
    values++;
    if (ch === 0x5b) {
      i++;
      stack.push({ list: build ? [] : (null as unknown as unknown[]), len: 0 });
      if (stack.length > depth) depth = stack.length;
      ws();
      if (s.charCodeAt(i) !== 0x5d) continue;
      i++;
      stack.pop();
      units += 1;
      v = build ? [] : null;
    } else if (ch === 0x7b) {
      i++;
      ws();
      if (s.charCodeAt(i) === 0x7d) {
        i++;
        units += 1;
        if (stack.length + 1 > depth) depth = stack.length + 1;
        v = build ? new RMap() : null;
      } else {
        const k = key();
        if (k === null) return null;
        stack.push(build ? { map: new RMap(), keys: null, key: k } : { map: null, keys: new Set<string>(), key: k });
        if (stack.length > depth) depth = stack.length;
        continue;
      }
    } else if (ch === 0x22) {
      if (build) {
        const t = str();
        if (t === null) return null;
        units += 1 + size(t);
        v = t;
      } else {
        const len = strLen();
        if (len < 0) return null;
        units += 1 + div64(len);
        v = null;
      }
    } else if (s.startsWith("true", i)) {
      i += 4;
      v = true;
    } else if (s.startsWith("false", i)) {
      i += 5;
      v = false;
    } else if (s.startsWith("null", i)) {
      i += 4;
      v = null;
    } else {
      const x = num();
      if (x === null) return null;
      v = x;
    }
    // The value is complete: give it to the frame it stands in.
    for (;;) {
      const f = stack[stack.length - 1];
      if (f === undefined) {
        top = v;
        break outer;
      }
      ws();
      if ("len" in f) {
        if (build) f.list.push(v);
        f.len++;
        if (s.charCodeAt(i) === 0x2c) {
          i++;
          ws();
          continue outer;
        }
        if (s.charCodeAt(i) !== 0x5d) return null;
        i++;
        stack.pop();
        units += 1 + f.len;
        v = build ? f.list : null;
      } else {
        if (build) f.map!.set(f.key, v);
        else f.keys!.add(f.key);
        if (s.charCodeAt(i) === 0x2c) {
          i++;
          ws();
          const k = key();
          if (k === null) return null;
          f.key = k;
          continue outer;
        }
        if (s.charCodeAt(i) !== 0x7d) return null;
        i++;
        stack.pop();
        units += 1 + 2 * (build ? f.map!.size : f.keys!.size);
        v = build ? f.map : null;
      }
    }
  }
  ws();
  if (i !== n) return null;
  return { value: top, values, units, depth, numberFailure };
}

// ---- time (f06 7) ---------------------------------------------------------------

const DAY = 86400;
const MIN_TIME = -62167219200; // 0000-01-01T00:00:00Z
const END_TIME = 253402300800; // 10000-01-01T00:00:00Z

function leap(y: number): boolean {
  return (y % 4 === 0 && y % 100 !== 0) || y % 400 === 0;
}

function monthLength(y: number, m: number): number {
  return m === 2 ? (leap(y) ? 29 : 28) : m === 4 || m === 6 || m === 9 || m === 11 ? 30 : 31;
}

/** The days from 1970-01-01 to a date of the proleptic Gregorian calendar. */
function daysFromCivil(y: number, m: number, d: number): number {
  const yy = m <= 2 ? y - 1 : y;
  const era = Math.floor(yy / 400);
  const yoe = yy - era * 400;
  const doy = Math.floor((153 * (m + (m > 2 ? -3 : 9)) + 2) / 5) + d - 1;
  const doe = yoe * 365 + Math.floor(yoe / 4) - Math.floor(yoe / 100) + doy;
  return era * 146097 + doe - 719468;
}

/** The date of a day counted from 1970-01-01. */
function civilFromDays(z: number): [number, number, number] {
  z += 719468;
  const era = Math.floor(z / 146097);
  const doe = z - era * 146097;
  const yoe = Math.floor((doe - Math.floor(doe / 1460) + Math.floor(doe / 36524) - Math.floor(doe / 146096)) / 365);
  const doy = doe - (365 * yoe + Math.floor(yoe / 4) - Math.floor(yoe / 100));
  const mp = Math.floor((5 * doy + 2) / 153);
  const d = doy - Math.floor((153 * mp + 2) / 5) + 1;
  const m = mp + (mp < 10 ? 3 : -9);
  return [yoe + era * 400 + (m <= 2 ? 1 : 0), m, d];
}

const DATE_TIME = /^([0-9]{4})-([0-9]{2})-([0-9]{2})T([0-9]{2}):([0-9]{2}):([0-9]{2})(?:\.([0-9]+))?(?:Z|([+-])([0-9]{2}):([0-9]{2}))$/;

/** The nearest double to W + 0.digits, W whole (f06 7.2 item 4), in O(digits.length) time. */
function wholePlusFraction(w: number, digits: string): number {
  let end = digits.length;
  while (end > 0 && digits.charCodeAt(end - 1) === 0x30) end--;
  if (end === 0) return norm(w);
  const frac = digits.slice(0, end);
  if (w >= 0) return norm(parseDecimal(`${w}.${frac}`)!);
  // W < 0 and 0 < 0.frac < 1: W + 0.frac = -((|W| - 1) + (1 - 0.frac)).
  const whole = -w - 1;
  const sub = new Array<string>(frac.length);
  let borrow = 0;
  for (let i = frac.length - 1; i >= 0; i--) {
    const d = frac.charCodeAt(i) - 0x30 + borrow;
    if (d === 0) {
      sub[i] = "0";
      borrow = 0;
    } else {
      sub[i] = String(10 - d);
      borrow = 1;
    }
  }
  return norm(-parseDecimal(`${whole}.${sub.join("")}`)!);
}

implement(null, "time.parse", (c) => {
  const s = c.args[0] as string;
  c.run.charge(size(s));
  const bad = () => new RunFailure("invalid-argument", c.span, "not-a-time", { text: s });
  const m = DATE_TIME.exec(s);
  if (m === null) throw bad();
  const [y, mo, d, h, mi, se] = [1, 2, 3, 4, 5, 6].map((k) => Number(m[k]));
  if (mo < 1 || mo > 12 || d < 1 || d > monthLength(y, mo) || h > 23 || mi > 59 || se > 59) throw bad();
  let offset = 0;
  if (m[8] !== undefined) {
    const oh = Number(m[9]);
    const om = Number(m[10]);
    if (oh > 23 || om > 59) throw bad();
    offset = (m[8] === "+" ? 1 : -1) * (3600 * oh + 60 * om);
  }
  const w = DAY * daysFromCivil(y, mo, d) + 3600 * h + 60 * mi + se - offset;
  return wholePlusFraction(w, m[7] ?? "");
});

const pad = (n: number, k: number) => String(n).padStart(k, "0");

/** The exact value of a number's text, as digits × 10^exp. */
function exactOf(text: string): { digits: bigint; exp: number } {
  const m = /^(-?)([0-9]+)(?:\.([0-9]+))?(?:e([+-]?[0-9]+))?$/.exec(text)!;
  const frac = m[3] ?? "";
  let digits = BigInt(m[2] + frac);
  if (m[1] === "-") digits = -digits;
  return { digits, exp: Number(m[4] ?? "0") - frac.length };
}

implement(null, "time.format", (c) => {
  const t = c.args[0] as number;
  const w = Math.floor(t);
  if (!(w >= MIN_TIME && w < END_TIME)) throw new RunFailure("invalid-argument", c.span, "year-range", { t });
  const day = Math.floor(w / DAY);
  const sec = w - day * DAY;
  const [y, mo, d] = civilFromDays(day);
  let out = `${pad(y, 4)}-${pad(mo, 2)}-${pad(d, 2)}T${pad(Math.floor(sec / 3600), 2)}:${pad(Math.floor((sec % 3600) / 60), 2)}:${pad(sec % 60, 2)}`;
  if (t !== w) {
    // f06 7.3 item 3: the digits of V - W, as many as V has after the point.
    const v = exactOf(numberToText(t));
    const p = Math.max(0, -v.exp);
    const f = v.digits * 10n ** BigInt(v.exp + p) - BigInt(w) * 10n ** BigInt(p);
    out += "." + f.toString().padStart(p, "0");
  }
  out += "Z";
  c.run.charge(size(out));
  c.run.alloc(1 + size(out));
  return out;
});

// Fixed-width decimal accumulator for `time.duration` (f06 7.4): every binary64
// value and midpoint terminates by 10^-1075 (half of 2^-1074) and overflows
// below 10^309. Keeping 1,120 fractional columns (45 guard digits) and 320
// integer columns bounds scratch memory to O(1) while preserving exact
// round-to-nearest-even and handling carries of any magnitude up to overflow.
const DUR_FRAC_COLS = 1120;
const DUR_INT_COLS = 320;
const DUR_TOTAL_COLS = DUR_FRAC_COLS + DUR_INT_COLS;
const DUR_GUARD_COLS = DUR_FRAC_COLS - 1075;
const DUR_TAIL_WIN = 1024;

function isAsciiDigit(ch: number): boolean {
  return ch >= 0x30 && ch <= 0x39;
}

function colsToDouble(cols: Float64Array): number {
  let top = cols.length - 1;
  while (top > DUR_FRAC_COLS && cols[top] === 0) top--;
  let lo = 0;
  while (lo < DUR_FRAC_COLS && cols[lo] === 0) lo++;
  if (top === DUR_FRAC_COLS && cols[DUR_FRAC_COLS] === 0 && lo === DUR_FRAC_COLS) return 0;
  const chars: string[] = [];
  for (let i = top; i >= DUR_FRAC_COLS; i--) chars.push(String(cols[i]));
  if (lo < DUR_FRAC_COLS) {
    chars.push(".");
    for (let i = DUR_FRAC_COLS - 1; i >= lo; i--) chars.push(String(cols[i]));
  }
  return parseDecimal(chars.join(""))!;
}

/** Compare the sum of all fractional digits strictly below 10^-DUR_FRAC_COLS against deficit * 10^-DUR_FRAC_COLS. */
function compareDurationTail(s: string, deficit: number, eps: number): -1 | 0 | 1 {
  const n = s.length;
  let i = s.charCodeAt(0) === 0x2d ? 1 : 0;
  const longTerms: { fpStart: number; fpTrim: number; mul: number; shift: number }[] = [];
  while (i < n) {
    while (i < n && isAsciiDigit(s.charCodeAt(i))) i++;
    let fpStart = i;
    let fpEnd = i;
    if (i < n && s.charCodeAt(i) === 0x2e) {
      i++;
      fpStart = i;
      while (i < n && isAsciiDigit(s.charCodeAt(i))) i++;
      fpEnd = i;
    }
    let mul = 1;
    let shift = 0;
    const ch = s.charCodeAt(i);
    if (ch === 0x6d && i + 1 < n && s.charCodeAt(i + 1) === 0x73) {
      i += 2;
      mul = 1;
      shift = 3;
    } else if (ch === 0x73) {
      i += 1;
      mul = 1;
    } else if (ch === 0x6d) {
      i += 1;
      mul = 60;
    } else if (ch === 0x68) {
      i += 1;
      mul = 3600;
    } else if (ch === 0x64) {
      i += 1;
      mul = 86400;
    } else {
      break;
    }
    const cutoff = DUR_FRAC_COLS - shift;
    if (fpEnd - fpStart > cutoff) {
      let trim = fpEnd;
      while (trim > fpStart + cutoff && s.charCodeAt(trim - 1) === 0x30) trim--;
      if (trim > fpStart + cutoff) longTerms.push({ fpStart, fpTrim: trim, mul, shift });
    }
  }

  let lo = DUR_FRAC_COLS;
  let rem = deficit;
  const win = new Float64Array(DUR_TAIL_WIN);
  for (;;) {
    win.fill(0);
    const hi = lo + DUR_TAIL_WIN;
    let write = 0;
    for (let tIdx = 0; tIdx < longTerms.length; tIdx++) {
      const t = longTerms[tIdx];
      const fpLen = t.fpTrim - t.fpStart;
      const kStart = lo - t.shift;
      const kEnd = Math.min(hi - t.shift, fpLen);
      for (let k = kStart; k < kEnd; k++) {
        const d = s.charCodeAt(t.fpStart + k) - 0x30;
        if (d !== 0) win[DUR_TAIL_WIN - 1 - (k - kStart)] += d * t.mul;
      }
      if (fpLen > hi - t.shift) longTerms[write++] = t;
    }
    longTerms.length = write;
    const anyBelowHi = longTerms.length > 0;

    let carry = 0;
    for (let j = 0; j < DUR_TAIL_WIN; j++) {
      const sum = win[j] + carry;
      carry = Math.floor(sum / 10);
      win[j] = sum - carry * 10;
    }
    if (carry > rem) return 1;
    rem -= carry;
    for (let j = DUR_TAIL_WIN - 1; j >= 0; j--) {
      if (rem > eps) return -1;
      const r10 = rem * 10;
      if (win[j] > r10) return 1;
      rem = r10 - win[j];
    }
    if (rem === 0) return anyBelowHi ? 1 : 0;
    if (!anyBelowHi) return -1;
    lo = hi;
  }
}

implement(null, "time.duration", (c) => {
  const s = c.args[0] as string;
  c.run.charge(size(s));
  const bad = () => new RunFailure("invalid-argument", c.span, "not-a-duration", { text: s });
  const range = () => new RunFailure("number-range", c.span, "entry", { name: "time.duration" });
  const n = s.length;
  const neg = n > 0 && s.charCodeAt(0) === 0x2d;
  let i = neg ? 1 : 0;
  if (i === n) throw bad();

  const cols = new Float64Array(DUR_TOTAL_COLS);
  let overflow = false;
  let tailTerms = 0;

  while (i < n) {
    const ipStart = i;
    while (i < n && isAsciiDigit(s.charCodeAt(i))) i++;
    const ipEnd = i;
    if (ipEnd === ipStart) throw bad();
    let fpStart = i;
    let fpEnd = i;
    if (i < n && s.charCodeAt(i) === 0x2e) {
      i++;
      fpStart = i;
      while (i < n && isAsciiDigit(s.charCodeAt(i))) i++;
      fpEnd = i;
      if (fpEnd === fpStart) throw bad();
    }
    let mul = 1;
    let shift = 0;
    if (i >= n) throw bad();
    const ch = s.charCodeAt(i);
    if (ch === 0x6d && i + 1 < n && s.charCodeAt(i + 1) === 0x73) {
      i += 2;
      mul = 1;
      shift = 3;
    } else if (ch === 0x73) {
      i += 1;
      mul = 1;
    } else if (ch === 0x6d) {
      i += 1;
      mul = 60;
    } else if (ch === 0x68) {
      i += 1;
      mul = 3600;
    } else if (ch === 0x64) {
      i += 1;
      mul = 86400;
    } else {
      throw bad();
    }
    if (!overflow) {
      let ipNz = ipStart;
      while (ipNz < ipEnd && s.charCodeAt(ipNz) === 0x30) ipNz++;
      if (ipEnd - ipNz > DUR_INT_COLS + shift) {
        overflow = true;
      } else {
        for (let p = 0; p < ipEnd - ipNz; p++) {
          const d = s.charCodeAt(ipEnd - 1 - p) - 0x30;
          if (d !== 0) {
            const col = DUR_FRAC_COLS + p - shift;
            if (col >= DUR_TOTAL_COLS) {
              overflow = true;
              break;
            }
            cols[col] += d * mul;
          }
        }
      }
      if (!overflow) {
        const cutoff = DUR_FRAC_COLS - shift;
        const fpLen = fpEnd - fpStart;
        const keep = Math.min(fpLen, cutoff);
        for (let k = 0; k < keep; k++) {
          const d = s.charCodeAt(fpStart + k) - 0x30;
          if (d !== 0) cols[DUR_FRAC_COLS - 1 - shift - k] += d * mul;
        }
        for (let k = cutoff; k < fpLen; k++) {
          if (s.charCodeAt(fpStart + k) !== 0x30) {
            tailTerms++;
            break;
          }
        }
      }
    }
  }

  if (overflow) throw range();
  let carry = 0;
  for (let idx = 0; idx < DUR_TOTAL_COLS; idx++) {
    const sum = cols[idx] + carry;
    carry = Math.floor(sum / 10);
    cols[idx] = sum - carry * 10;
  }
  if (carry > 0) throw range();

  const finish = (v: number): number => {
    if (!Number.isFinite(v)) throw range();
    return norm(neg && v !== 0 ? -v : v);
  };
  const fA = colsToDouble(cols);
  if (!Number.isFinite(fA)) throw range();
  if (tailTerms === 0) return finish(fA);

  const eps = tailTerms * 86400;
  const colsB = new Float64Array(cols);
  let remCarry = eps;
  for (let idx = 0; idx < DUR_TOTAL_COLS && remCarry > 0; idx++) {
    const sum = colsB[idx] + remCarry;
    remCarry = Math.floor(sum / 10);
    colsB[idx] = sum - remCarry * 10;
  }
  const fB = colsToDouble(colsB);
  let guardZero = true;
  for (let idx = 0; idx < DUR_GUARD_COLS; idx++) {
    if (cols[idx] !== 0) {
      guardZero = false;
      break;
    }
  }
  if (guardZero) return finish(fB);
  if (Number.isFinite(fB) && fA === fB) return finish(fA);

  const colsM = new Float64Array(colsB);
  colsM.fill(0, 0, DUR_GUARD_COLS);
  const fM = colsToDouble(colsM);
  let deficit = 0;
  let place = 1;
  let borrow = 0;
  for (let idx = 0; idx < DUR_GUARD_COLS; idx++) {
    const d = cols[idx] + borrow;
    let out = 0;
    if (d === 0) {
      borrow = 0;
    } else {
      borrow = 1;
      out = 10 - d;
    }
    if (out !== 0) deficit += out * place;
    if (place <= eps) place *= 10;
  }
  const cmp = compareDurationTail(s, deficit, eps);
  return finish(cmp < 0 ? fA : cmp === 0 ? fM : fB);
});

// ---- math (f06 8) ---------------------------------------------------------------

implement(null, "math.sqrt", (c) => {
  const x = c.args[0] as number;
  if (x < 0) throw new RunFailure("invalid-argument", c.span, "not-real", { name: "math.sqrt" });
  // IEEE 754 square root, correctly rounded on every platform (f06 8.3 item 1).
  return norm(Math.sqrt(x));
});

// ---- base64 (f06 9) -------------------------------------------------------------

const ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

/** The UTF-8 encoding of a well-formed string (f06 4.1). */
function utf8(s: string): number[] {
  const out: number[] = [];
  for (const ch of s) {
    const cp = ch.codePointAt(0)!;
    if (cp < 0x80) out.push(cp);
    else if (cp < 0x800) out.push(0xc0 | (cp >> 6), 0x80 | (cp & 63));
    else if (cp < 0x10000) out.push(0xe0 | (cp >> 12), 0x80 | ((cp >> 6) & 63), 0x80 | (cp & 63));
    else out.push(0xf0 | (cp >> 18), 0x80 | ((cp >> 12) & 63), 0x80 | ((cp >> 6) & 63), 0x80 | (cp & 63));
  }
  return out;
}

/** The length of the UTF-8 encoding of a well-formed string, with nothing built. */
function utf8Length(s: string): number {
  let n = 0;
  for (let i = 0; i < s.length; i++) {
    const u = s.charCodeAt(i);
    if (u < 0x80) n += 1;
    else if (u < 0x800) n += 2;
    else if (u >= 0xd800 && u <= 0xdbff) {
      n += 4; // a surrogate pair: one code point above U+FFFF
      i++;
    } else n += 3;
  }
  return n;
}

implement(null, "base64.encode", (c: CallContext) => {
  const s = c.args[0] as string;
  // f06 9.1 item 2: the weight is found from the code points of s before
  // anything is charged or built.
  const weight = 4 * Math.ceil(utf8Length(s) / 3);
  c.run.charge(size(s) + div64(weight));
  c.run.alloc(1 + div64(weight));
  const b = utf8(s);
  let out = "";
  for (let i = 0; i < b.length; i += 3) {
    const n = (b[i] << 16) | ((b[i + 1] ?? 0) << 8) | (b[i + 2] ?? 0);
    out += ALPHABET[(n >> 18) & 63] + ALPHABET[(n >> 12) & 63];
    out += i + 1 < b.length ? ALPHABET[(n >> 6) & 63] : "=";
    out += i + 2 < b.length ? ALPHABET[n & 63] : "=";
  }
  return out;
});

/** The value of a base64 character, and its alphabet: 1 for `+ /`, 2 for `- _`, 0 for both. -1: none. */
function b64(ch: number): [number, number] {
  if (ch >= 0x41 && ch <= 0x5a) return [ch - 0x41, 0];
  if (ch >= 0x61 && ch <= 0x7a) return [ch - 0x61 + 26, 0];
  if (ch >= 0x30 && ch <= 0x39) return [ch - 0x30 + 52, 0];
  if (ch === 0x2b) return [62, 1];
  if (ch === 0x2f) return [63, 1];
  if (ch === 0x2d) return [62, 2];
  if (ch === 0x5f) return [63, 2];
  return [-1, 0];
}

/** Decode well-formed UTF-8 (Unicode table 3-7), or null. */
function fromUtf8(b: number[]): string | null {
  let out = "";
  let i = 0;
  while (i < b.length) {
    const x = b[i];
    let cp: number;
    let len: number;
    let lo = 0x80;
    let hi = 0xbf;
    if (x < 0x80) {
      out += String.fromCharCode(x);
      i++;
      continue;
    } else if (x >= 0xc2 && x <= 0xdf) {
      len = 2;
      cp = x & 0x1f;
    } else if (x >= 0xe0 && x <= 0xef) {
      len = 3;
      cp = x & 0x0f;
      if (x === 0xe0) lo = 0xa0;
      if (x === 0xed) hi = 0x9f;
    } else if (x >= 0xf0 && x <= 0xf4) {
      len = 4;
      cp = x & 0x07;
      if (x === 0xf0) lo = 0x90;
      if (x === 0xf4) hi = 0x8f;
    } else return null;
    if (i + len > b.length) return null;
    for (let k = 1; k < len; k++) {
      const y = b[i + k];
      if (y < (k === 1 ? lo : 0x80) || y > (k === 1 ? hi : 0xbf)) return null;
      cp = (cp << 6) | (y & 0x3f);
    }
    out += String.fromCodePoint(cp);
    i += len;
  }
  return out;
}

implement(null, "base64.decode", (c) => {
  const s = c.args[0] as string;
  c.run.charge(size(s));
  const bad = () => new RunFailure("invalid-argument", c.span, "not-base64", {});
  let end = s.length;
  while (end > 0 && s.charCodeAt(end - 1) === 0x3d) end--;
  const p = s.length - end;
  const n = end;
  const vals: number[] = new Array(n);
  let alphabet = 0;
  for (let i = 0; i < n; i++) {
    const [v, a] = b64(s.charCodeAt(i));
    if (v < 0) throw bad();
    if (a !== 0) {
      if (alphabet !== 0 && alphabet !== a) throw bad();
      alphabet = a;
    }
    vals[i] = v;
  }
  const r = n % 4;
  if (r === 1) throw bad();
  if (p !== 0 && !(r !== 0 && p === 4 - r)) throw bad();
  if (r === 2 && (vals[n - 1] & 15) !== 0) throw bad();
  if (r === 3 && (vals[n - 1] & 3) !== 0) throw bad();
  const bytes: number[] = [];
  for (let i = 0; i < n; i += 4) {
    const a = vals[i];
    const b = vals[i + 1];
    bytes.push(((a << 2) | (b >> 4)) & 255);
    if (i + 2 < n) bytes.push(((b << 4) | (vals[i + 2] >> 2)) & 255);
    if (i + 3 < n) bytes.push(((vals[i + 2] << 6) | vals[i + 3]) & 255);
  }
  const out = fromUtf8(bytes);
  if (out === null) throw new RunFailure("invalid-argument", c.span, "not-utf8", {});
  c.run.alloc(1 + size(out));
  return out;
});
