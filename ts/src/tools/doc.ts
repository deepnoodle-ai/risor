// The document of the formatter and its layout (f12 2.4, 2.5).

/** A part of a document (f12 2.4). A string is `text(s)`. */
export type Doc = string | Part | Doc[];

export type Part =
  | { k: "space" }
  | { k: "line" }
  | { k: "soft" }
  | { k: "hard" }
  | { k: "blank" }
  | { k: "after" }
  | { k: "indent"; d: Doc; f?: boolean }
  | { k: "group"; d: Doc; broken: boolean; f?: boolean }
  | { k: "ib"; a: Doc; b: Doc; f?: boolean }
  | { k: "choice"; alts: Doc[]; must: boolean; f?: boolean }
  | { k: "tb"; open: string; render: (column: string) => string };

export const SP: Doc = { k: "space" };
export const L: Doc = { k: "line" };
export const S: Doc = { k: "soft" };
export const H: Doc = { k: "hard" };
export const BLANK: Doc = { k: "blank" };
export const AFTER: Doc = { k: "after" };
export const indent = (d: Doc): Doc => ({ k: "indent", d });
export const group = (d: Doc): Doc => ({ k: "group", d, broken: false });
export const brokenGroup = (d: Doc): Doc => ({ k: "group", d, broken: true });
export const ib = (a: Doc, b: Doc): Doc => ({ k: "ib", a, b });
export const choice = (alts: Doc[], must: boolean): Doc => ({ k: "choice", alts, must });

/** The width of a text: its number of code points (f12 1.1). */
export function width(s: string): number {
  let n = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c < 0xdc00 || c > 0xdfff) n++;
  }
  return n;
}

// `f` of a part that holds parts is whether it holds a forced break, once that
// is known: `forced` keeps its answer there. A part is not changed after it is
// made, so the answer stays true.

// The walks of a document below keep their own stacks. A document is as deep as
// its program is long where the program is flat (01 13.1): the left operand of
// an operator is inside the group of the operator, and an `else` branch of
// `? :` is inside the indent of the one before it. So no walk may call itself
// once for each part.

/** The parts of a part that can force it: an if-broken part by its flat form, a choice by its first. */
function forcing(d: Doc[] | Part, i: number): Doc | undefined {
  if (Array.isArray(d)) return d[i];
  if (i > 0) return undefined;
  switch (d.k) {
    case "indent":
    case "group":
      return d.d;
    case "choice":
      return d.alts[0];
    case "ib":
      // An if-broken part is laid out by its group's mode: only its flat form can force it.
      return d.b;
    default:
      return undefined;
  }
}

/** Whether a part holds a forced break (f12 2.4 item 4). */
export function forced(root: Doc): boolean {
  // The parts whose answer waits for their own parts, the outermost first, and the next part of each.
  const open: (Doc[] | Part)[] = [];
  const next: number[] = [];
  /** The answer for a part that needs no walk, or undefined when the part is opened. */
  const enter = (d: Doc): boolean | undefined => {
    if (typeof d === "string") return false;
    if (Array.isArray(d)) {
      if (d.length === 0) return false;
    } else {
      switch (d.k) {
        case "hard":
        case "blank":
        case "after":
          return true;
        case "group":
          if (d.broken) return true;
          if (d.f !== undefined) return d.f;
          break;
        case "indent":
        case "choice":
        case "ib":
          if (d.f !== undefined) return d.f;
          break;
        default:
          return false;
      }
    }
    open.push(d);
    next.push(0);
    return undefined;
  };
  const keep = (d: Doc[] | Part, r: boolean): void => {
    if (!Array.isArray(d) && (d.k === "group" || d.k === "indent" || d.k === "choice" || d.k === "ib")) d.f = r;
  };
  const first = enter(root);
  if (first !== undefined) return first;
  for (;;) {
    const top = open.length - 1;
    const c = forcing(open[top], next[top]);
    if (c === undefined) {
      // No part of it is forced.
      keep(open[top], false);
      open.pop();
      next.pop();
      if (top === 0) return false;
      next[top - 1]++;
      continue;
    }
    const r = enter(c);
    if (r === undefined) continue;
    if (r) {
      // A forced part forces every part that holds it.
      for (const o of open) keep(o, true);
      return true;
    }
    next[top]++;
  }
}

/**
 * A copy of a document with some of its parts replaced. `replace` is asked for
 * each part that is not a text or a list: it gives the document that stands in
 * the part's place, or undefined, and then the part is kept and its own parts
 * are copied the same way.
 */
export function mapDoc(root: Doc, replace: (p: Part) => Doc | undefined): Doc {
  interface Frame {
    src: Doc[] | Part;
    kids: Doc[];
    out: Doc[];
  }
  const frames: Frame[] = [];
  let d: Doc = root;
  for (;;) {
    // Down: the copy of *d*, when it needs no copy of a part; or its first part.
    let res: Doc;
    for (;;) {
      if (typeof d === "string") {
        res = d;
        break;
      }
      let kids: Doc[] | null = null;
      if (Array.isArray(d)) kids = d;
      else {
        const r = replace(d);
        if (r !== undefined) {
          res = r;
          break;
        }
        if (d.k === "indent" || d.k === "group") kids = [d.d];
        else if (d.k === "ib") kids = [d.a, d.b];
        else if (d.k === "choice") kids = d.alts;
        if (kids === null) {
          res = d;
          break;
        }
      }
      frames.push({ src: d, kids, out: [] });
      if (kids.length === 0) {
        res = "";
        break;
      }
      d = kids[0];
    }
    // Up: give the copy to the part that holds it, and close each part that is complete.
    for (;;) {
      const f = frames[frames.length - 1];
      if (f === undefined) return res;
      // A part with no parts was opened with nothing to copy.
      if (f.kids.length > 0) f.out.push(res);
      if (f.out.length < f.kids.length) {
        d = f.kids[f.out.length];
        break;
      }
      frames.pop();
      const src = f.src;
      if (Array.isArray(src)) res = f.out;
      else if (src.k === "indent") res = { k: "indent", d: f.out[0] };
      else if (src.k === "group") res = { k: "group", d: f.out[0], broken: src.broken };
      else if (src.k === "ib") res = { k: "ib", a: f.out[0], b: f.out[1] };
      else if (src.k === "choice") res = { k: "choice", alts: f.out, must: src.must };
      else res = src;
    }
  }
}

type Mode = "flat" | "broken";
interface Cmd {
  ind: number;
  mode: Mode;
  d: Doc;
}

/**
 * What a part adds to the line it starts on, in a mode: the width it adds
 * before the line ends or the part ends, and whether the line ends in it. The
 * line ends at a break that is taken, at a line end in a text, and after the
 * opening line of a text block. *checks* is whether the walk of item 7 of f12
 * 2.5 meets a part on the way that compares the width left with 0: a text, a
 * space, a flat line or a text block. A break that is taken does not, so a
 * line that is already too long fits when a break comes before any text.
 */
interface Reach {
  w: number;
  ends: boolean;
  checks: boolean;
}

/** The reaches that one layout has found, for each mode. */
interface Reaches {
  flat: Map<object, Reach>;
  broken: Map<object, Reach>;
}

const NOTHING: Reach = { w: 0, ends: false, checks: false };
const ONE: Reach = { w: 1, ends: false, checks: true };
const ENDS: Reach = { w: 0, ends: true, checks: false };

/** A list whose reach took more parts than this to find is kept; a smaller one is found again. */
const KEPT_FROM = 16;

function reachOfText(x: string): Reach {
  const nl = x.indexOf("\n");
  return nl >= 0 ? { w: width(x.slice(0, nl)), ends: true, checks: true } : { w: width(x), ends: false, checks: true };
}

/**
 * The reach of a part in a mode. The reach of a part that holds parts is kept
 * in *known*, so the parts of a document are walked a fixed number of times,
 * however deep the document is.
 */
function reach(root: Doc, rootMode: Mode, known: Reaches): Reach {
  // The lists whose parts are being added up, and the parts whose reach is that of one part of theirs.
  interface Frame {
    d: Doc[] | Part;
    mode: Mode;
    i: number;
    w: number;
    checks: boolean;
    /** The count of parts when the frame was opened. */
    from: number;
  }
  const frames: Frame[] = [];
  let parts = 0;
  let d: Doc = root;
  let mode: Mode = rootMode;
  for (;;) {
    // Down: the reach of *d*, when it is known or needs no walk; or its first part.
    let res: Reach;
    for (;;) {
      parts++;
      if (typeof d === "string") {
        res = reachOfText(d);
        break;
      }
      const memo = known[mode].get(d);
      if (memo !== undefined) {
        res = memo;
        break;
      }
      let inner: Doc | null = null;
      let innerMode: Mode = mode;
      let leaf: Reach = NOTHING;
      if (Array.isArray(d)) {
        if (d.length > 0) inner = d[0];
      } else {
        switch (d.k) {
          case "space":
            leaf = ONE;
            break;
          case "line":
            leaf = mode === "broken" ? ENDS : ONE;
            break;
          case "soft":
            leaf = mode === "broken" ? ENDS : NOTHING;
            break;
          case "hard":
          case "blank":
          case "after":
            leaf = ENDS;
            break;
          case "tb":
            leaf = { w: width(d.open), ends: true, checks: true };
            break;
          case "indent":
            inner = d.d;
            break;
          case "ib":
            inner = mode === "broken" ? d.a : d.b;
            break;
          case "group":
            inner = d.d;
            innerMode = forced(d) ? "broken" : mode;
            break;
          case "choice":
            inner = mode === "flat" ? d.alts[0] : d.alts[d.alts.length - 1];
            break;
        }
      }
      if (inner === null) {
        res = leaf;
        break;
      }
      frames.push({ d, mode, i: 0, w: 0, checks: false, from: parts });
      d = inner;
      mode = innerMode;
    }
    // Up: give the reach to the part that holds it.
    for (;;) {
      const f = frames[frames.length - 1];
      if (f === undefined) return res;
      if (Array.isArray(f.d)) {
        f.w += res.w;
        if (res.checks) f.checks = true;
        f.i++;
        if (!res.ends && f.i < f.d.length) {
          d = f.d[f.i];
          mode = f.mode;
          break;
        }
        res = { w: f.w, ends: res.ends, checks: f.checks };
        if (parts - f.from > KEPT_FROM) known[f.mode].set(f.d, res);
      } else known[f.mode].set(f.d, res);
      frames.pop();
    }
  }
}

function isSpaces(s: string): boolean {
  for (let i = 0; i < s.length; i++) if (s.charCodeAt(i) !== 0x20) return false;
  return true;
}

/** Lay out a document (f12 2.5), with a line width (Infinity for none). */
export function layout(doc: Doc, lineWidth = 80): string {
  // The text so far is *done* and then *line*. *done* is the lines that are
  // complete: it is only added to, so a long text costs its length and not the
  // square of it. *line* is the text after the last line end.
  let done = "";
  /** The line ends at the end of *done*: 0, 1, or 2 for two or more. */
  let ends = 0;
  let line = "";
  /** The width of *line*. */
  let col = 0;
  /** Whether *line* is spaces only. */
  let atLineStart = true;
  let mark = false;
  const stack: Cmd[] = [{ ind: 0, mode: "broken", d: doc }];

  const add = (s: string) => {
    const nl = s.lastIndexOf("\n");
    if (nl < 0) {
      line += s;
      col += width(s);
      if (atLineStart && !isSpaces(s)) atLineStart = false;
      return;
    }
    if (nl >= 1 && s.charCodeAt(nl - 1) === 0x0a) ends = 2;
    else if (nl === 0 && line.length === 0) ends = Math.min(2, ends + 1);
    else ends = 1;
    done += line + s.slice(0, nl + 1);
    line = s.slice(nl + 1);
    col = width(line);
    atLineStart = isSpaces(line);
  };
  /** Take the spaces off the end of the text. */
  const trimEnd = () => {
    if (atLineStart) {
      line = "";
      col = 0;
      return;
    }
    let e = line.length;
    while (line.charCodeAt(e - 1) === 0x20) e--;
    if (e < line.length) {
      col -= line.length - e;
      line = line.slice(0, e);
    }
  };
  const brk = (ind: number) => {
    if (atLineStart) trimEnd();
    else {
      trimEnd();
      add("\n");
    }
    add(" ".repeat(ind));
    mark = false;
  };
  const emptyLine = (ind: number) => {
    trimEnd();
    if (done.length === 0 && line.length === 0) return;
    while (!(line.length === 0 && ends === 2)) add("\n");
    add(" ".repeat(ind));
    mark = false;
  };
  const clearMark = (ind: number) => {
    if (mark) brk(ind);
  };

  /** Whether a part, flat, and what follows it to the end of its line fit in the width. */
  const known: Reaches = { flat: new Map(), broken: new Map() };
  const fits = (d: Doc, column: number): boolean => {
    let r = lineWidth - column;
    let x = reach(d, "flat", known);
    let si = stack.length - 1;
    for (;;) {
      r -= x.w;
      // Only a width that a part adds is checked; r falls only there, so the last check decides.
      if (x.checks && r < 0) return false;
      if (x.ends || si < 0) return true;
      const c = stack[si--];
      x = reach(c.d, c.mode, known);
    }
  };

  for (;;) {
    const c = stack.pop();
    if (c === undefined) break;
    const { ind, mode, d } = c;
    if (typeof d === "string") {
      if (d.length === 0) continue;
      clearMark(ind);
      add(d);
      continue;
    }
    if (Array.isArray(d)) {
      for (let i = d.length - 1; i >= 0; i--) stack.push({ ind, mode, d: d[i] });
      continue;
    }
    switch (d.k) {
      case "space":
        if (mark) clearMark(ind);
        else if (!atLineStart) add(" ");
        break;
      case "line":
        if (mode === "broken") brk(ind);
        else if (mark) clearMark(ind);
        else if (!atLineStart) add(" ");
        break;
      case "soft":
        if (mode === "broken") brk(ind);
        break;
      case "hard":
        brk(ind);
        break;
      case "blank":
        emptyLine(ind);
        break;
      case "after":
        mark = true;
        break;
      case "indent":
        stack.push({ ind: ind + 2, mode, d: d.d });
        break;
      case "group": {
        let m: Mode;
        // The answer and the reach of a group are those of what it holds, and they are kept for the group.
        if (forced(d)) m = "broken";
        else if (mode === "flat") m = "flat";
        else m = fits(d, col) ? "flat" : "broken";
        stack.push({ ind, mode: m, d: d.d });
        break;
      }
      case "ib":
        stack.push({ ind, mode, d: mode === "broken" ? d.a : d.b });
        break;
      case "choice": {
        if (mode === "flat") {
          stack.push({ ind, mode: "flat", d: d.alts[0] });
          break;
        }
        const last = d.alts[d.alts.length - 1];
        if (d.must) {
          stack.push({ ind, mode: "broken", d: last });
          break;
        }
        let picked: Doc | null = null;
        for (let i = 0; i < d.alts.length - 1; i++) {
          if (fits(d.alts[i], col)) {
            picked = d.alts[i];
            break;
          }
        }
        if (picked !== null) stack.push({ ind, mode: "flat", d: picked });
        else stack.push({ ind, mode: "broken", d: last });
        break;
      }
      case "tb": {
        clearMark(ind);
        let lead = 0;
        while (line.charCodeAt(lead) === 0x20) lead++;
        add(d.render(" ".repeat(lead) + "  "));
        break;
      }
    }
  }
  return done + line;
}
