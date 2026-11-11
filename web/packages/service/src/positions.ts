// Risor counts code points; JavaScript strings, the DOM and CodeMirror count
// UTF-16 code units. This is the one place the packages convert.

/** The positions of one text, in both counts. */
export class TextIndex {
  readonly text: string;
  /** The UTF-16 offset of each code point, and of the end; null when the text has no astral code point. */
  private readonly units: number[] | null;
  /** The code-point offset at which each line starts. */
  private readonly lineStarts: number[];
  /** Risor's Source drops a U+FEFF at the start of a program. */
  private readonly bom: number;

  constructor(text: string) {
    this.text = text;
    this.bom = text.charCodeAt(0) === 0xfeff ? 1 : 0;
    let astral = false;
    for (let i = 0; i < text.length; i++) {
      const c = text.charCodeAt(i);
      if (c >= 0xd800 && c <= 0xdbff) {
        astral = true;
        break;
      }
    }
    let units: number[] | null = null;
    if (astral) {
      units = [];
      let u = 0;
      for (const ch of text) {
        units.push(u);
        u += ch.length;
      }
      units.push(u);
    }
    this.units = units;
    this.lineStarts = [0];
    let cp = 0;
    for (const ch of astral ? text : "") {
      cp++;
      if (ch === "\n") this.lineStarts.push(cp);
    }
    if (!astral) for (let i = 0; i < text.length; i++) if (text.charCodeAt(i) === 0x0a) this.lineStarts.push(i + 1);
  }

  /** The number of code points. */
  get length(): number {
    return this.units ? this.units.length - 1 : this.text.length;
  }

  /** The UTF-16 offset of a code-point offset of the text as given. */
  unitOf(cp: number): number {
    const n = Math.max(0, Math.min(cp, this.length));
    return this.units ? this.units[n] : n;
  }

  /** The code-point offset of a UTF-16 offset. An offset inside a pair counts as the pair's start. */
  pointOf(unit: number): number {
    if (!this.units) return Math.max(0, Math.min(unit, this.text.length));
    let lo = 0;
    let hi = this.units.length - 1;
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if (this.units[mid] <= unit) lo = mid;
      else hi = mid - 1;
    }
    return lo;
  }

  /** The UTF-16 offset of a code-point offset that Risor gave for this text as a program. */
  unitOfRisor(cp: number): number {
    return this.unitOf(cp + this.bom);
  }

  /** The UTF-16 offset of a line and column (from 1, code points), as Risor gives them. */
  unitOfPosition(line: number, column: number): number {
    const l = Math.max(1, Math.min(line, this.lineStarts.length));
    return this.unitOf(this.lineStarts[l - 1] + column - 1 + (l === 1 ? this.bom : 0));
  }

  /** The line and column (from 1, code points) of a UTF-16 offset, as Risor counts them. */
  positionOf(unit: number): { line: number; column: number } {
    const cp = this.pointOf(unit);
    let lo = 0;
    let hi = this.lineStarts.length - 1;
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if (this.lineStarts[mid] <= cp) lo = mid;
      else hi = mid - 1;
    }
    const column = cp - this.lineStarts[lo] + 1 - (lo === 0 ? this.bom : 0);
    return { line: lo + 1, column: Math.max(1, column) };
  }
}
