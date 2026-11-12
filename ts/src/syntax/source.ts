// Source text as code points, and positions in it.
//
// The specification counts every position in code points (01 2.1, 03 3.7.7).
// A JavaScript string is UTF-16, so the lexer never indexes the string: it
// reads an array of code points, and every offset in this implementation (a
// token's start and end, a node's span) is an index into that array. Lines
// and columns are computed from offsets only when a diagnostic leaves the
// implementation.

/** A span as two code-point offsets into the source. `end` is exclusive. */
export interface Span {
  start: number;
  end: number;
}

/** A position as the specification writes it: line and column from 1. */
export interface Position {
  line: number;
  column: number;
}

/** The four numbers of 03 3.7.7 item 5. */
export type LineSpan = [number, number, number, number];

export class Source {
  /** The text's code points. A U+FEFF at the start is not included. */
  readonly cps: Uint32Array;
  /** Offsets at which each line starts, made on first use. Lines end at U+000A only. */
  private starts: number[] | null = null;

  constructor(text: string, options: { keepBom?: boolean } = {}) {
    const cps = toCodePoints(text);
    // 01 2.1 rule 2: a U+FEFF at the start of a program is ignored. It is
    // removed before positions are counted (questions file, ts Q1). A
    // template keeps it (f02 2.1 item 2).
    this.cps = !options.keepBom && cps.length > 0 && cps[0] === 0xfeff ? cps.subarray(1) : cps;
  }

  /**
   * A source over code points as they are, with no copy and no U+FEFF
   * removed. For tools that lex part of a text (src/tools/tokens.ts).
   */
  static ofCodePoints(cps: Uint32Array): Source {
    const s: Source = Object.create(Source.prototype);
    (s as { cps: Uint32Array }).cps = cps;
    s.starts = null;
    return s;
  }

  private get lineStarts(): number[] {
    if (this.starts === null) {
      this.starts = [0];
      for (let i = 0; i < this.cps.length; i++) {
        if (this.cps[i] === 0x0a) this.starts.push(i + 1);
      }
    }
    return this.starts;
  }

  get length(): number {
    return this.cps.length;
  }

  position(offset: number): Position {
    // Binary search for the last line start at or before offset.
    let lo = 0;
    let hi = this.lineStarts.length - 1;
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if (this.lineStarts[mid] <= offset) lo = mid;
      else hi = mid - 1;
    }
    return { line: lo + 1, column: offset - this.lineStarts[lo] + 1 };
  }

  lineSpan(span: Span): LineSpan {
    const a = this.position(span.start);
    const b = this.position(span.end);
    return [a.line, a.column, b.line, b.column];
  }

  /** The text of a span, as a JavaScript string. */
  slice(start: number, end: number): string {
    return fromCodePoints(this.cps, start, end);
  }
}

/**
 * The code points of a JavaScript string. A lone surrogate is one malformed
 * unit and reads as U+FFFD (03 3.4.5).
 */
export function toCodePoints(text: string): Uint32Array {
  const out = new Uint32Array(text.length);
  let n = 0;
  for (let i = 0; i < text.length; i++) {
    const c = text.charCodeAt(i);
    if (c >= 0xd800 && c <= 0xdbff) {
      const d = i + 1 < text.length ? text.charCodeAt(i + 1) : 0;
      if (d >= 0xdc00 && d <= 0xdfff) {
        out[n++] = ((c - 0xd800) << 10) + (d - 0xdc00) + 0x10000;
        i++;
      } else {
        out[n++] = 0xfffd;
      }
    } else if (c >= 0xdc00 && c <= 0xdfff) {
      out[n++] = 0xfffd;
    } else {
      out[n++] = c;
    }
  }
  return out.subarray(0, n);
}

/** A JavaScript string from a range of code points. */
export function fromCodePoints(cps: ArrayLike<number>, start = 0, end = cps.length): string {
  let s = "";
  const CHUNK = 4096;
  for (let i = start; i < end; i += CHUNK) {
    const j = Math.min(end, i + CHUNK);
    const part: number[] = [];
    for (let k = i; k < j; k++) part.push(cps[k]);
    s += String.fromCodePoint(...part);
  }
  return s;
}
