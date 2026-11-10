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
  | { k: "indent"; d: Doc }
  | { k: "group"; d: Doc; broken: boolean }
  | { k: "ib"; a: Doc; b: Doc }
  | { k: "choice"; alts: Doc[]; must: boolean }
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

const forcedMemo = new WeakMap<object, boolean>();

/** Whether a part holds a forced break (f12 2.4 item 4). */
export function forced(d: Doc): boolean {
  if (typeof d === "string") return false;
  const m = forcedMemo.get(d);
  if (m !== undefined) return m;
  let r: boolean;
  if (Array.isArray(d)) r = d.some(forced);
  else {
    switch (d.k) {
      case "hard":
      case "blank":
      case "after":
        r = true;
        break;
      case "indent":
        r = forced(d.d);
        break;
      case "group":
        r = d.broken || forced(d.d);
        break;
      case "choice":
        r = forced(d.alts[0]);
        break;
      case "ib":
        // An if-broken part is laid out by its group's mode: only its flat form can force it.
        r = forced(d.b);
        break;
      default:
        r = false;
    }
  }
  forcedMemo.set(d, r);
  return r;
}

type Mode = "flat" | "broken";
interface Cmd {
  ind: number;
  mode: Mode;
  d: Doc;
}

/** Lay out a document (f12 2.5), with a line width (Infinity for none). */
export function layout(doc: Doc, lineWidth = 80): string {
  let out = "";
  let mark = false;
  const stack: Cmd[] = [{ ind: 0, mode: "broken", d: doc }];

  const column = () => width(out.slice(out.lastIndexOf("\n") + 1));
  const atLineStart = () => /^ *$/.test(out.slice(out.lastIndexOf("\n") + 1));
  const trimEnd = () => {
    let e = out.length;
    while (e > 0 && out[e - 1] === " ") e--;
    out = out.slice(0, e);
  };
  const brk = (ind: number) => {
    if (atLineStart()) trimEnd();
    else {
      trimEnd();
      out += "\n";
    }
    out += " ".repeat(ind);
    mark = false;
  };
  const emptyLine = (ind: number) => {
    trimEnd();
    if (out.length === 0) return;
    while (!out.endsWith("\n\n")) out += "\n";
    out += " ".repeat(ind);
    mark = false;
  };
  const clearMark = (ind: number) => {
    if (mark) brk(ind);
  };

  const fits = (d: Doc, col: number): boolean => {
    let r = lineWidth - col;
    const walk: { d: Doc; mode: Mode }[] = [{ d, mode: "flat" }];
    let si = stack.length - 1;
    for (;;) {
      let item = walk.pop();
      if (item === undefined) {
        if (si < 0) return true;
        const c = stack[si--];
        item = { d: c.d, mode: c.mode };
      }
      const { d: x, mode } = item;
      if (typeof x === "string") {
        const nl = x.indexOf("\n");
        if (nl >= 0) {
          r -= width(x.slice(0, nl));
          return r >= 0;
        }
        r -= width(x);
        if (r < 0) return false;
        continue;
      }
      if (Array.isArray(x)) {
        for (let i = x.length - 1; i >= 0; i--) walk.push({ d: x[i], mode });
        continue;
      }
      switch (x.k) {
        case "space":
          r -= 1;
          if (r < 0) return false;
          break;
        case "line":
          if (mode === "broken") return true;
          r -= 1;
          if (r < 0) return false;
          break;
        case "soft":
          if (mode === "broken") return true;
          break;
        case "hard":
        case "blank":
        case "after":
          return true;
        case "indent":
          walk.push({ d: x.d, mode });
          break;
        case "ib":
          walk.push({ d: mode === "broken" ? x.a : x.b, mode });
          break;
        case "group":
          walk.push({ d: x.d, mode: x.broken || forced(x.d) ? "broken" : mode });
          break;
        case "choice":
          walk.push(mode === "flat" ? { d: x.alts[0], mode: "flat" } : { d: x.alts[x.alts.length - 1], mode: "broken" });
          break;
        case "tb":
          r -= width(x.open);
          return r >= 0;
      }
    }
  };

  for (;;) {
    const c = stack.pop();
    if (c === undefined) break;
    const { ind, mode, d } = c;
    if (typeof d === "string") {
      if (d.length === 0) continue;
      clearMark(ind);
      out += d;
      continue;
    }
    if (Array.isArray(d)) {
      for (let i = d.length - 1; i >= 0; i--) stack.push({ ind, mode, d: d[i] });
      continue;
    }
    switch (d.k) {
      case "space":
        if (mark) clearMark(ind);
        else if (!atLineStart()) out += " ";
        break;
      case "line":
        if (mode === "broken") brk(ind);
        else if (mark) clearMark(ind);
        else if (!atLineStart()) out += " ";
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
        if (d.broken) m = "broken";
        else if (mode === "flat") m = forced(d.d) ? "broken" : "flat";
        else if (forced(d.d)) m = "broken";
        else m = fits(d.d, column()) ? "flat" : "broken";
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
        const col = column();
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
        const line = out.slice(out.lastIndexOf("\n") + 1);
        const lead = /^ */.exec(line)![0];
        out += d.render(lead + "  ");
        break;
      }
    }
  }
  return out;
}
