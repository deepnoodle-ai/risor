// The formatter of package F12 (f12 2): the canonical text of a program and of
// a template. A program is parsed; its tokens and comments are read again from
// the source; a document is built from the tree (2.8 to 2.13), with each
// comment beside the token it is attached to (2.12); and the document is laid
// out at width 80 (2.5).

import type * as A from "../syntax/ast.ts";
import type { Diagnostic } from "../syntax/diagnostic.ts";
import { lex } from "../syntax/lexer.ts";
import { deepestLevel, parse } from "../syntax/parser.ts";
import { Source, SOURCE_LIMIT, withinSourceLimit } from "../syntax/source.ts";
import { sourceTooLarge } from "../syntax/messages.ts";
import type { Span } from "../syntax/source.ts";
import type { Token } from "../syntax/token.ts";
import { parseTemplateText } from "../syntax/template.ts";
import { lexPart } from "../syntax/lexer.ts";
import { AFTER, BLANK, H, L, S, SP, brokenGroup, choice, forced, group, ib, indent, layout, mapDoc } from "./doc.ts";
import type { Doc, Part } from "./doc.ts";

/** The outcome of the formatter (f12 2.1). */
export type FormatResult = { ok: true; text: string } | { ok: false; diagnostics: Diagnostic[] };

/** Format a program (f12 2.1). A text longer than the source limit gives its one diagnostic (01 2.1 item 6). */
export function formatProgram(text: string, limit: number = SOURCE_LIMIT): FormatResult {
  if (!withinSourceLimit(text, limit)) return { ok: false, diagnostics: [sourceTooLarge(limit)] };
  const source = new Source(text);
  const p = parse(source);
  if (!p.ok) return { ok: false, diagnostics: p.diagnostics };
  const tokens = lex(source).filter((t) => t.kind !== "eof");
  const f = new Formatter(source, tokens, 0, source.length, tooDeep(deepestLevel(p.program)));
  const doc = f.build((b) => b.program(p.program));
  return { ok: true, text: finish(layout(doc)) };
}

/** Whether a text reaching level `deepest` gets no pairs for the reader, which add up to 2 levels (f12 2.6 items 3 and 4). */
function tooDeep(deepest: number): boolean {
  return deepest + 2 > 100;
}

/** Format a template (f12 2.14). */
export function formatTemplate(text: string, limit: number = SOURCE_LIMIT): FormatResult {
  const t = parseTemplateText(text, limit);
  if (!t.ok) return { ok: false, diagnostics: t.diagnostics };
  const src = t.template.source;
  let deepest = 0;
  for (const seg of t.template.segments) if (seg.kind !== "text") deepest = Math.max(deepest, deepestLevel(seg.expr));
  const plain = tooDeep(deepest);
  let out = "";
  for (const seg of t.template.segments) {
    const raw = src.slice(seg.span.start, seg.span.end);
    if (seg.kind === "text") {
      out += raw;
      continue;
    }
    // The part's tokens, without the closing delimiter.
    const toks = lexPart(src, seg.span.start + 3, seg.span.start).filter((x) => x.kind !== "interpEnd" && x.kind !== "eof");
    const closeStart = raw.endsWith("}}") ? seg.span.end - 2 : seg.span.end - 1;
    const f = new Formatter(src, toks, seg.span.start + 3, closeStart, plain);
    const body = layout(f.build((b) => b.expr(seg.expr)), Infinity);
    // `trim` takes off what `\s` matches: white space and line ends.
    const trimmed = body.trim();
    if (!trimmed.includes("\n") && !f.hasLineComment) out += "${{ " + trimmed + " }}";
    else out += raw;
  }
  return { ok: true, text: out };
}

/** The text ends with exactly one U+000A (f12 2.1 item 5). */
function finish(s: string): string {
  // By a loop from the end: a search for `[ \n]+$` tries each space of each indent, which
  // costs the square of the deepest indent for every line.
  let e = s.length;
  while (e > 0 && (s.charCodeAt(e - 1) === 0x20 || s.charCodeAt(e - 1) === 0x0a)) e--;
  return s.slice(0, e) + "\n";
}

/** A comment of the source. */
interface Comment {
  start: number;
  end: number;
  text: string;
  line: boolean;
  /** Line ends in the blank space directly before it and directly after it. */
  nlBefore: number;
  nlAfter: number;
  /** Whether it starts on the line where the token before it ends. */
  trailing: boolean;
}

const OPEN = new Set(["(", "[", "{", "?.["]);
const CLOSE: Record<string, string> = { "(": ")", "[": "]", "{": "}", "?.[": "]" };

/** Line ends in a run of white space: CR LF counts once. */
function lineEnds(s: string): number {
  let n = 0;
  for (let i = 0; i < s.length; i++) {
    if (s[i] === "\n") n++;
    else if (s[i] === "\r") {
      n++;
      if (s[i + 1] === "\n") i++;
    }
  }
  return n;
}

/** The count of code points of a text. */
function codePoints(s: string): number {
  let n = 0;
  for (let i = 0; i < s.length; n++) i += s.codePointAt(i)! > 0xffff ? 2 : 1;
  return n;
}

/** A build: pass 1 learns the kept tokens, pass 2 writes the comments. */
type Marker = { k: "lead"; i: number } | { k: "trail"; i: number } | { k: "end" };

class Formatter {
  readonly src: Source;
  readonly toks: Token[];
  readonly byStart = new Map<number, number>();
  readonly match = new Map<number, number>();
  readonly comments: Comment[] = [];
  /** Line ends in the blank space directly before each token. */
  readonly nlBefore: number[] = [];
  hasLineComment = false;
  /** The pairs for the reader written around the node being built (f12 2.6 items 3 and 4). */
  private rdepth = 0;

  // Pass state.
  private kept = new Set<number>();
  private pass = 1;
  private lead = new Map<number, Comment[]>();
  private trail = new Map<number, Comment[]>();
  private endComments: Comment[] = [];
  /** Tokens that begin an element other than the first of its sequence. */
  private elementStarts = new Set<number>();
  /** Closing brackets of a sequence with elements. */
  private closers = new Set<number>();
  /** Opening brackets of a sequence with elements. */
  private openWithElements = new Set<number>();

  /** No pairs for the reader in the whole text (f12 2.6 item 4). */
  private readonly plain: boolean;

  constructor(src: Source, toks: Token[], from: number, to: number, plain = false) {
    this.plain = plain;
    this.src = src;
    this.toks = toks;
    toks.forEach((t, i) => this.byStart.set(t.start, i));
    const st: number[] = [];
    toks.forEach((t, i) => {
      if (t.kind !== "punct") return;
      if (OPEN.has(t.text)) st.push(i);
      else if (t.text === ")" || t.text === "]" || t.text === "}") {
        const o = st.pop();
        if (o !== undefined) {
          this.match.set(o, i);
          this.match.set(i, o);
        }
      }
    });
    // The gaps between tokens: white space and comments. A gap is read once,
    // from its start: a gap of many comments costs its length, not the square of it.
    const blank = /[\s\uFEFF]*/uy;
    const lineComment = /\/\/[^\r\n]*/y;
    let prevEnd = from;
    let prevTokEnd = -1;
    for (let k = 0; k <= toks.length; k++) {
      const gapEnd = k < toks.length ? toks[k].start : to;
      let pos = prevEnd;
      let nl = 0;
      let lastComment: Comment | null = null;
      const gap = pos < gapEnd ? src.slice(pos, gapEnd) : "";
      /** The place of *pos* in *gap*, which is counted in UTF-16 units. */
      let at = 0;
      /** The line ends from the token before the gap to *pos*. */
      let since = 0;
      while (pos < gapEnd) {
        blank.lastIndex = at;
        const ws = blank.exec(gap)![0];
        const wsN = lineEnds(ws);
        nl += wsN;
        since += wsN;
        if (lastComment !== null) lastComment.nlAfter += wsN;
        pos += codePoints(ws);
        at += ws.length;
        if (pos >= gapEnd) break;
        let text: string;
        let line: boolean;
        if (gap.startsWith("//", at)) {
          lineComment.lastIndex = at;
          text = lineComment.exec(gap)![0];
          line = true;
          this.hasLineComment = true;
        } else {
          const e = gap.indexOf("*/", at);
          text = e < 0 ? gap.slice(at, at + 1) : gap.slice(at, e + 2);
          line = false;
        }
        const len = codePoints(text);
        const trailing = prevTokEnd >= 0 && since === 0;
        const c: Comment = { start: pos, end: pos + len, text, line, nlBefore: nl, nlAfter: 0, trailing };
        this.comments.push(c);
        lastComment = c;
        nl = 0;
        since += lineEnds(text);
        pos += len;
        at += text.length;
      }
      if (k < toks.length) {
        this.nlBefore[k] = nl;
        prevEnd = toks[k].end;
        prevTokEnd = toks[k].end;
      }
    }
  }

  /** Build the document twice: the first pass finds the kept tokens. */
  build(f: (b: Formatter) => Doc): Doc {
    this.pass = 1;
    this.kept = new Set();
    f(this);
    this.attach();
    this.pass = 2;
    const d = f(this);
    return this.expand(d);
  }

  private attach(): void {
    const kept = [...this.kept].sort((a, b) => a - b);
    // The tokens are in the order of the text, so the kept token before a comment
    // and the one after it are found by halving.
    /** The count of kept tokens for which *before* holds: it holds for a first run of them. */
    const count = (before: (t: Token) => boolean): number => {
      let lo = 0;
      let hi = kept.length;
      while (lo < hi) {
        const m = (lo + hi) >> 1;
        if (before(this.toks[kept[m]])) lo = m + 1;
        else hi = m;
      }
      return lo;
    };
    for (const c of this.comments) {
      let trailingTo = -1;
      if (c.trailing) {
        const n = count((t) => t.end <= c.start);
        if (n > 0) trailingTo = kept[n - 1];
      }
      if (trailingTo >= 0) {
        if (!this.trail.has(trailingTo)) this.trail.set(trailingTo, []);
        this.trail.get(trailingTo)!.push(c);
        continue;
      }
      const n = count((t) => t.start < c.end);
      const next = n < kept.length ? kept[n] : undefined;
      if (next === undefined) this.endComments.push(c);
      else {
        if (!this.lead.has(next)) this.lead.set(next, []);
        this.lead.get(next)!.push(c);
      }
    }
  }

  // ---- tokens and comments -----------------------------------------------

  /** The text of a token of the source, with its comments. */
  tok(i: number, opts: { noLead?: boolean; noTrail?: boolean } = {}): Doc {
    if (i < 0 || i >= this.toks.length) throw new Error("formatter: no token");
    this.kept.add(i);
    const t = this.toks[i];
    const text = this.tokenText(t);
    const out: Doc[] = [];
    if (!opts.noLead) out.push(this.leadMark(i));
    out.push(text);
    if (!opts.noTrail) out.push(this.trailMark(i));
    return out;
  }

  leadMark(i: number): Doc {
    return { k: "lead", i } as unknown as Doc;
  }

  trailMark(i: number): Doc {
    return { k: "trail", i } as unknown as Doc;
  }

  private tokenText(t: Token): Doc {
    const raw = this.src.slice(t.start, t.end);
    if (t.kind === "string" && (t.form === '"""' || t.form === 'r"""')) return this.textBlock(raw);
    if (t.kind === "template" && (t.form === '"""' || t.form === 'r"""')) return this.textBlock(raw);
    if (t.kind === "word" && t.text === "not") return "not";
    return raw.replace(/\r\n?/g, "\n");
  }

  /** A text block, re-indented (f12 2.7 item 2). */
  private textBlock(raw: string): Doc {
    const text = raw.replace(/\r\n?/g, "\n");
    const isRaw = text.startsWith("r");
    const open = isRaw ? 'r"""' : '"""';
    const nl = text.indexOf("\n");
    const body = text.slice(nl + 1, text.length - 3);
    // Split into content lines, keeping track of interpolations.
    const lines: { text: string; inInterp: boolean }[] = [];
    let cur = "";
    let curInInterp = false;
    let depth = 0;
    for (let i = 0; i < body.length; i++) {
      const c = body[i];
      if (!isRaw && depth === 0 && c === "\\") {
        cur += c + (body[i + 1] ?? "");
        i++;
        continue;
      }
      if (!isRaw && depth === 0 && c === "$" && body[i + 1] === "{") {
        depth = 1;
        cur += "${";
        i++;
        continue;
      }
      if (depth > 0 && c === "/" && body[i + 1] === "*") {
        // A comment inside an interpolation: copied whole, its line ends kept.
        const e = body.indexOf("*/", i + 2);
        const end = e < 0 ? body.length : e + 2;
        const piece = body.slice(i, end).split("\n");
        cur += piece[0];
        for (const rest of piece.slice(1)) {
          lines.push({ text: cur, inInterp: curInInterp });
          cur = rest;
          curInInterp = true;
        }
        i = end - 1;
        continue;
      }
      if (depth > 0 && c === "/" && body[i + 1] === "/") {
        const e = body.indexOf("\n", i);
        const end = e < 0 ? body.length : e;
        cur += body.slice(i, end);
        i = end - 1;
        continue;
      }
      if (depth > 0) {
        if (c === "{") depth++;
        else if (c === "}") depth--;
        else if (c === '"' || c === "'" || c === "`") {
          // A string inside an interpolation: copy it whole.
          let j = i + 1;
          while (j < body.length && body[j] !== c) j += body[j] === "\\" ? 2 : 1;
          cur += body.slice(i, j + 1);
          i = j;
          continue;
        }
      }
      if (c === "\n") {
        lines.push({ text: cur, inInterp: curInInterp });
        cur = "";
        curInInterp = depth > 0;
        continue;
      }
      cur += c;
    }
    const closing = cur;
    const I = [...closing].length;
    return {
      k: "tb",
      open,
      render: (J: string) => {
        let s = open + "\n";
        for (const ln of lines) {
          if (ln.inInterp) s += ln.text + "\n";
          else if (/^[ \t]*$/.test(ln.text)) s += "\n";
          else s += J + [...ln.text].slice(I).join("") + "\n";
        }
        return s + J + '"""';
      },
    };
  }

  /** The comments of the end of the text (f12 2.8, Program). */
  endMark(): Doc {
    return { k: "end" } as unknown as Doc;
  }

  private commentText(c: Comment): string {
    if (c.line) return c.text.replace(/[ \t]+$/, "");
    return c.text.replace(/\r\n?/g, "\n");
  }

  /** The documents of the leading comments of a token, or of the end (f12 2.12 items 6, 7). */
  private leadDoc(cs: Comment[], i: number | null): Doc {
    const out: Doc[] = [];
    const closerOrEnd = i === null || this.closers.has(i);
    cs.forEach((c, n) => {
      if (n === 0 && c.nlBefore >= 2 && (i === null || this.elementStarts.has(i) || this.closers.has(i))) out.push(BLANK);
      out.push(H, this.commentText(c));
      const next = n + 1 < cs.length ? cs[n + 1] : null;
      const lineEndAfter = c.nlAfter > 0;
      if (c.line || lineEndAfter) out.push(H);
      else out.push(SP);
      const lastOfCloser = closerOrEnd && next === null;
      if (c.nlAfter >= 2 && !lastOfCloser) out.push(BLANK);
    });
    return out;
  }

  /** The documents of the trailing comments of a token (f12 2.12 item 4). */
  private trailDoc(cs: Comment[], i: number): Doc {
    const out: Doc[] = [];
    const t = this.toks[i];
    const opening = t.kind === "punct" && OPEN.has(t.text);
    cs.forEach((c, n) => {
      if (opening && !c.line && n === 0) {
        out.push(this.commentText(c));
        if (this.openWithElements.has(i)) out.push(SP);
      } else {
        out.push(SP, this.commentText(c));
      }
      if (c.line) out.push(AFTER);
    });
    return out;
  }

  /** Replace the markers by the comments' documents. */
  private expand(d: Doc): Doc {
    return mapDoc(d, (p) => {
      const m = p as unknown as Marker;
      switch (m.k) {
        case "lead": {
          const cs = this.lead.get(m.i);
          return cs === undefined ? "" : this.leadDoc(cs, m.i);
        }
        case "trail": {
          const cs = this.trail.get(m.i);
          return cs === undefined ? "" : this.trailDoc(cs, m.i);
        }
        case "end":
          return this.endComments.length === 0 ? "" : this.leadDoc(this.endComments, null);
        default:
          return undefined;
      }
    });
  }

  // ---- finding tokens --------------------------------------------------------

  at(offset: number): number {
    const i = this.byStart.get(offset);
    if (i === undefined) throw new Error(`formatter: no token at ${offset}`);
    return i;
  }

  /** The first token at or after an offset with this text. */
  find(text: string, from: number): number {
    let i = this.firstFrom(from);
    while (i < this.toks.length && this.toks[i].text !== text) i++;
    if (i >= this.toks.length) throw new Error(`formatter: no ${text} after ${from}`);
    return i;
  }

  private firstFrom(offset: number): number {
    let lo = 0;
    let hi = this.toks.length;
    while (lo < hi) {
      const m = (lo + hi) >> 1;
      if (this.toks[m].start < offset) lo = m + 1;
      else hi = m;
    }
    return lo;
  }

  /** The span of a node with its parentheses. */
  outer(e: A.Expr | A.Lambda | A.Pattern): Span {
    return e.outer ?? e.span;
  }

  firstTok(e: A.Expr | A.Lambda | A.Pattern): number {
    return this.at(this.outer(e).start);
  }

  /** Line ends before a node's first token. */
  blankBefore(e: { span: Span; outer?: Span }): boolean {
    const i = this.at((e.outer ?? e.span).start);
    return this.nlBefore[i] >= 2 && !this.lead.has(i);
  }

  // ---- elements and sequences (f12 2.9, 2.12 item 5) ---------------------------

  /** The largest token index in a document. */
  private lastTokenIn(d: Doc): number {
    let best = -1;
    // The walk keeps its own stack: a document is as deep as a flat program is long.
    const todo: Doc[] = [d];
    while (todo.length > 0) {
      const x = todo.pop()!;
      if (typeof x === "string") continue;
      if (Array.isArray(x)) {
        for (const y of x) todo.push(y);
        continue;
      }
      const m = x as unknown as { k: string; i?: number; d?: Doc; a?: Doc; b?: Doc; alts?: Doc[] };
      if (m.k === "trail" || m.k === "lead") {
        if (m.i! > best) best = m.i!;
        continue;
      }
      if (m.d !== undefined) todo.push(m.d);
      if (m.a !== undefined) todo.push(m.a);
      if (m.b !== undefined) todo.push(m.b);
      if (m.alts !== undefined) for (const y of m.alts) todo.push(y);
    }
    return best;
  }

  /** Take the trailing comments of the element's last token out of it, to stand after its comma. */
  element(d: Doc): { d: Doc; trail: Doc } {
    const last = this.lastTokenIn(d);
    if (last < 0) return { d, trail: "" };
    let found = false;
    const out = mapDoc(d, (p: Part) => {
      const m = p as unknown as { k: string; i?: number };
      if (m.k === "trail" && m.i === last) {
        found = true;
        return "";
      }
      return undefined;
    });
    return { d: out, trail: found ? this.trailMark(last) : "" };
  }

  /** seq(o, xs, c) (f12 2.9): `open` and `close` are token indices. */
  seq(open: number, xs: { d: Doc; first: number }[], close: number): Doc {
    if (xs.length === 0) return [this.tok(open), this.tok(close, { noLead: false })];
    this.openWithElements.add(open);
    this.closers.add(close);
    const inner: Doc[] = [S];
    xs.forEach((x, n) => {
      if (n > 0) {
        this.elementStarts.add(x.first);
        inner.push(L);
        if (this.nlBefore[x.first] >= 2 && !this.lead.has(x.first)) inner.push(ib(BLANK, ""));
      }
      const el = this.element(x.d);
      inner.push(el.d);
      inner.push(n < xs.length - 1 ? "," : ib(",", ""));
      inner.push(el.trail);
    });
    inner.push(this.leadMark(close));
    return group([this.tok(open), indent(inner), S, this.tok(close, { noLead: true })]);
  }

  /** Elements separated by hard breaks: a program's, a block's or a branch's lines (f12 2.9 item 4). */
  lines(items: { d: Doc; first: number; semi: Doc }[]): Doc {
    const out: Doc[] = [];
    items.forEach((x, n) => {
      if (n > 0) {
        this.elementStarts.add(x.first);
        out.push(H);
        if (this.nlBefore[x.first] >= 2 && !this.lead.has(x.first)) out.push(BLANK);
      }
      const el = this.element([x.d, x.semi]);
      out.push(el.d, el.trail);
    });
    return out;
  }

  // ---- nodes (f12 2.8) -------------------------------------------------------

  program(p: A.Program): Doc {
    // f16 7.1: a type line is a line of the program, as a const line is, in the order of the text.
    const lines = [...p.consts.map((c) => ({ at: c.span.start, item: () => this.constItem(c) })), ...p.types.map((l) => ({ at: l.span.start, item: () => this.typeItem(l) }))];
    lines.sort((a, b) => a.at - b.at);
    const items = [...lines.map((l) => l.item()), { d: this.expr(p.result), first: this.firstTok(p.result), semi: "" as Doc }];
    return [this.lines(items), H, this.endMark(), H];
  }

  /** f16 7.1: `"type" sp name sp "=" sp type ";"`, the type on one line. */
  private typeItem(l: A.TypeLine): { d: Doc; first: number; semi: Doc } {
    const kw = this.at(l.span.start);
    const name = this.at(l.nameSpan.start);
    const semi = this.find(";", l.type.span.end);
    return { d: [this.tok(kw), SP, this.tok(name), SP, this.tok(name + 1), SP, this.typeDoc(l.type.span)], first: kw, semi: this.tok(semi) };
  }

  private constItem(c: A.Const): { d: Doc; first: number; semi: Doc } {
    const kw = this.at(c.span.start);
    const semi = this.find(";", this.outer(c.value).end);
    if (c.type !== null) {
      // f16 7.1: `"const" sp ‹pattern› ":" sp type sp "=" assigned(value) ";"`.
      const colon = this.find(":", this.outer(c.pattern).end);
      const eq = this.find("=", c.type.span.end);
      return { d: [this.tok(kw), SP, this.pattern(c.pattern), this.tok(colon), SP, this.typeDoc(c.type.span), SP, this.tok(eq), this.assigned(c.value)], first: kw, semi: this.tok(semi) };
    }
    const eq = this.find("=", this.outer(c.pattern).end);
    return { d: [this.tok(kw), SP, this.pattern(c.pattern), SP, this.tok(eq), this.assigned(c.value)], first: kw, semi: this.tok(semi) };
  }

  /** assigned(v) (f12 2.8 item 2). */
  assigned(v: A.Expr | A.Lambda): Doc {
    if (v.kind === "Binary" || v.kind === "As" || (v.kind === "Cond" && v.form === "?:")) {
      return group(indent([L, this.expr(v as A.Expr, { unindented: true })]));
    }
    return [SP, v.kind === "Lambda" ? this.lambda(v) : this.expr(v)];
  }

  /** The body of a block or a branch: its const lines and its result. */
  private body(consts: A.Const[], result: A.Expr): Doc {
    return this.lines([...consts.map((c) => this.constItem(c)), { d: this.expr(result), first: this.firstTok(result), semi: "" as Doc }]);
  }

  /** A branch of `if` or `try`, at its `{` (f12 2.8). */
  private branch(lb: number, content: A.Expr): Doc {
    const rb = this.match.get(lb)!;
    let body: Doc;
    if (content.kind === "Block" && content.span.start === this.toks[lb].start) body = this.body(content.consts, content.result);
    else if (content.kind === "Block" && !content.parens) {
      // A Block that is the whole content of a branch: one tree with the branch.
      body = this.body(content.consts, content.result);
    } else body = this.body([], content);
    return [this.tok(lb), indent([L, body, this.leadMark(rb)]), L, this.tok(rb, { noLead: true })];
  }

  /** The level of an expression (f12 2.6). */
  private level(e: A.Expr | A.Lambda): number {
    switch (e.kind) {
      case "Cond":
      case "Try":
        return 1;
      case "Binary":
        return binLevel(e.op);
      case "As":
        return 6;
      case "Unary":
        return 9;
      case "Lambda":
        return 0;
      default:
        return 11;
    }
  }

  /** ‹x› with parentheses where 2.6 says. */
  private paren(e: A.Expr, need: boolean): Doc {
    const d = this.expr(e);
    return need ? ["(", d, ")"] : d;
  }

  expr(e: A.Expr | A.Lambda, opts: { unindented?: boolean } = {}): Doc {
    switch (e.kind) {
      case "Lambda":
        return this.lambda(e);
      case "Ident":
      case "Number":
      case "String":
      case "Template":
      case "Bool":
      case "Null":
        return this.tok(this.at(e.span.start));
      case "Unary": {
        const op = this.at(e.span.start);
        const x = e.x;
        const need = this.level(x) < 9 || (x.kind === "Binary" && x.op === "**");
        const space = e.op === "-" && x.kind === "Unary" && x.op === "-" && !need;
        return [this.tok(op), space ? " " : "", this.paren(x, need)];
      }
      case "Binary":
        return e.op === "**" ? this.power(e) : this.operators(e, opts.unindented === true);
      case "As":
        return this.operators(e, false);
      case "Cond":
        return e.form === "?:" ? this.ternary(e) : this.ifExpr(e);
      case "Try":
        return this.tryExpr(e);
      case "List": {
        const lb = this.at(e.span.start);
        const rb = this.match.get(lb)!;
        return this.seq(lb, e.items.map((it) => this.item(it)), rb);
      }
      case "Map": {
        const lb = this.at(e.span.start);
        const rb = this.match.get(lb)!;
        return this.seq(lb, e.entries.map((en) => this.entry(en)), rb);
      }
      case "Block": {
        const lb = this.at(e.span.start);
        const rb = this.match.get(lb)!;
        return [this.tok(lb), indent([H, this.body(e.consts, e.result), this.leadMark(rb)]), H, this.tok(rb, { noLead: true })];
      }
      case "Match":
        return this.matchExpr(e);
      case "Call":
      case "MethodCall":
      case "Member":
      case "Index":
      case "OptChain":
      case "Each":
        return this.chain(e);
      case "Bad":
        return "";
    }
  }

  private item(it: A.Expr | A.Spread): { d: Doc; first: number } {
    if (it.kind === "Spread") {
      const dots = this.at(it.span.start);
      return { d: [this.tok(dots), this.expr(it.value)], first: dots };
    }
    return { d: this.expr(it), first: this.firstTok(it) };
  }

  private entry(en: A.Entry | A.Spread): { d: Doc; first: number } {
    if (en.kind === "Spread") return this.item(en);
    if (en.shorthand) {
      const i = this.at(en.span.start);
      return { d: this.tok(i), first: i };
    }
    let key: Doc;
    let first: number;
    let keyEnd: number;
    if (en.key.kind === "expr") {
      const lb = this.at(en.span.start);
      const rb = this.match.get(lb)!;
      key = [this.tok(lb), this.expr(en.key.expr), this.tok(rb)];
      first = lb;
      keyEnd = this.toks[rb].end;
    } else {
      first = this.at(en.key.span.start);
      key = this.tok(first);
      keyEnd = en.key.span.end;
    }
    const colon = this.find(":", keyEnd);
    return { d: [key, this.tok(colon), this.assigned(en.value)], first };
  }

  /** `x ** y` (f12 2.8 item 3): its left operand is a primary, so it is no part of a left spine. */
  private power(e: A.Binary): Doc {
    const need = this.level(e.x) < 11;
    const needR = this.level(e.y) < 9;
    const opI = this.find("**", this.outer(e.x).end);
    return group([this.paren(e.x, need), SP, this.tok(opI), indent([L, this.paren(e.y, needR)])]);
  }

  /**
   * A binary operator other than `**`, `as` or `is`, with all that its left
   * operand holds (f12 2.8 item 3, f04 8).
   *
   * The left operand of each opens no level (01 13.1, f04 2.8), so the left
   * spine is as long as the program, and it is built by a loop: the innermost
   * left operand first, then each operator outward. The operators of one level
   * that stand next to one another on the spine, with no parentheses between
   * them, are one run: g(‹x1› sp o1 i(L ‹x2› sp o2 L ‹x3› ... L ‹xn›)).
   */
  private operators(e: A.Binary | A.As, unindented: boolean): Doc {
    const spine: (A.Binary | A.As)[] = [];
    let x: A.Expr = e;
    while (x.kind === "As" || (x.kind === "Binary" && x.op !== "**")) {
      spine.push(x);
      x = x.x;
    }
    // The parentheses around the left operand of each node of the spine, and
    // the pairs for the reader around each node, from the outermost in. A
    // run has pairs for the reader only when it has at most 3 operators and
    // no pair encloses its top (f12 2.6 items 3 and 4).
    const outerDepth = this.rdepth;
    const leftParen: boolean[] = new Array(spine.length);
    const depthAt: number[] = new Array(spine.length);
    const pairsOK: boolean[] = new Array(spine.length);
    let r = outerDepth;
    for (let k = 0; k < spine.length; k++) {
      depthAt[k] = r;
      const n = spine[k];
      if (n.kind === "As") continue;
      const prev = k > 0 ? spine[k - 1] : null;
      const top = prev === null || prev.kind === "As" || !sameRun(prev, n);
      pairsOK[k] = top ? !this.plain && r === 0 && runLen(n) <= 3 : pairsOK[k - 1];
      const need = this.needLeft(n.x, n);
      const rd = !need && pairsOK[k] && readerRow(n.x, n, "left");
      leftParen[k] = need || rd;
      if (rd) r++;
    }
    // The document of the left operand of spine[i], without its parentheses.
    this.rdepth = r;
    let d: Doc = this.expr(x);
    let i = spine.length - 1;
    while (i >= 0) {
      const n = spine[i];
      this.rdepth = depthAt[i];
      if (n.kind === "As") {
        const left = n.x;
        const need = this.level(left) < 6 || (left.kind === "Binary" && left.op === "??") || (left.kind === "Unary" && left.op === "!");
        const kw = this.find(n.form, this.outer(left).end);
        d = [need ? ["(", d, ")"] : d, SP, this.tok(kw), SP, this.typeDoc(n.typeSpan)];
        i--;
        continue;
      }
      // A run of one level: from *n* outward, while the next operator is of the level and takes this one bare.
      const p = binLevel(n.op);
      const head: Doc = [leftParen[i] ? ["(", d, ")"] : d, SP, this.opTok(n)];
      const tail: Doc[] = [L, this.right(n.y, n, pairsOK[i])];
      let j = i - 1;
      for (; j >= 0; j--) {
        const m = spine[j];
        if (m.kind !== "Binary" || binLevel(m.op) !== p || leftParen[j]) break;
        this.rdepth = depthAt[j];
        tail.push(SP, this.opTok(m), L, this.right(m.y, m, pairsOK[j]));
      }
      // The run of the whole expression is the one that `unindented` is about.
      d = group([head, unindented && j < 0 ? tail : indent(tail)]);
      i = j;
    }
    this.rdepth = outerDepth;
    return d;
  }

  /**
   * The right operand of a binary operator, in its parentheses (f12 2.6).
   * `pairsOK` says whether the parent's run may have pairs for the reader.
   */
  private right(y: A.Expr, parent: A.Binary, pairsOK: boolean): Doc {
    if (this.needRight(y, parent)) return this.paren(y, true);
    if (!pairsOK || !readerRow(y, parent, "right")) return this.expr(y);
    this.rdepth++;
    const d = this.expr(y);
    this.rdepth--;
    return ["(", d, ")"];
  }

  private opTok(n: A.Binary): Doc {
    if (n.op === "not-in") {
      const notI = this.find("not", this.outer(n.x).end);
      return [this.tok(notI), " ", this.tok(notI + 1)];
    }
    return this.tok(this.find(n.op, this.outer(n.x).end));
  }

  /** The left operand of a binary operator (f12 2.6). */
  private needLeft(x: A.Expr, parent: A.Binary): boolean {
    const p = binLevel(parent.op);
    const lx = this.level(x);
    if (parent.op === "??") return lx < 7 && !(x.kind === "Binary" && x.op === "??");
    if (parent.op === "**") return lx < 11;
    let need = lx < p || (x.kind === "Binary" && x.op === "??");
    if ((parent.op === "in" || parent.op === "not-in") && x.kind === "Unary" && x.op === "!") need = true;
    // A comparison inside a comparison (01 3.4 rule 13).
    if (isComparison(parent) && isComparison(x)) need = true;
    return need;
  }

  private needRight(y: A.Expr, parent: A.Binary): boolean {
    const p = binLevel(parent.op);
    const ly = this.level(y);
    if (parent.op === "??") return ly < 7 || (y.kind === "Binary" && y.op === "??");
    if (parent.op === "**") return ly < 9;
    let need = ly <= p || (y.kind === "Binary" && y.op === "??");
    // On the right, also one under `as` or `is` (01 3.4 rule 13).
    if (isComparison(parent) && this.comparisonUnder(y)) need = true;
    return need;
  }

  /**
   * Whether e is a comparison, or an `as` or `is` whose left operand, written
   * without parentheses, is one: its text holds a comparison operator outside
   * parentheses (01 3.4 rule 13).
   */
  private comparisonUnder(e: A.Expr): boolean {
    let x = e;
    while (x.kind === "As") {
      const left = x.x;
      if (this.level(left) < 6 || (left.kind === "Binary" && left.op === "??") || (left.kind === "Unary" && left.op === "!")) return false;
      x = left;
    }
    return isComparison(x);
  }

  /**
   * `test ? then : else` (f12 2.8). A conditional whose `else` branch is a
   * conditional is one chain with it (item 5). The chain is one group, and
   * each link, a test with its `?` and branch, is a group of its own: a broken
   * chain starts each link on a line at the chain's indentation, so its text
   * grows linearly with its length. The `else` branch opens no level (01 13.1),
   * so the chain is walked by a loop.
   */
  private ternary(e: A.Cond): Doc {
    const out: Doc[] = [];
    let n: A.Cond = e;
    for (;;) {
      const test = this.paren(n.test, this.level(n.test) === 1);
      const q = this.find("?", this.outer(n.test).end);
      const c = this.find(":", this.outer(n.then).end);
      const qd = this.tok(q);
      let then: Doc = this.expr(n.then);
      // A conditional in the branch writes its `:` lines one level inside the `?`.
      if (n.then.kind === "Cond" && n.then.form === "?:") then = indent([then]);
      out.push(group([test, indent([L, qd, SP, then])]), L, this.tok(c), SP);
      const x = n.else;
      if (x.kind === "Cond" && x.form === "?:") {
        n = x;
        continue;
      }
      out.push(this.expr(x));
      break;
    }
    return group(out);
  }

  private ifExpr(e: A.Cond): Doc {
    const out: Doc[] = [];
    let cur: A.Expr = e;
    let first = true;
    for (;;) {
      const c = cur as A.Cond;
      const ifI = this.at(c.span.start);
      const lp = ifI + 1;
      const rp = this.match.get(lp)!;
      if (!first) {
        out.push(SP);
      }
      out.push(this.tok(ifI), SP, this.tok(lp), group([indent([S, this.expr(c.test, { unindented: true })]), S]), this.tok(rp), SP);
      const lb = rp + 1;
      out.push(this.branch(lb, c.then));
      const thenEnd = this.match.get(lb)!;
      const elseI = thenEnd + 1;
      out.push(SP, this.tok(elseI));
      const els = c.else;
      if (els.kind === "Cond" && els.form === "if" && !els.parens) {
        const elseLb = elseI + 1;
        if (this.toks[elseLb].text === "if") {
          cur = els;
          first = false;
          out.push();
          continue;
        }
        // `else { if ... }`: the branch's braces are dropped, and it is written `else if`.
        cur = els;
        first = false;
        continue;
      }
      out.push(SP, this.branch(elseI + 1, els));
      break;
    }
    return group(out);
  }

  private tryExpr(e: A.Try): Doc {
    const tryI = this.at(e.span.start);
    const lb = tryI + 1;
    const rb = this.match.get(lb)!;
    const catchI = rb + 1;
    const out: Doc[] = [this.tok(tryI), SP, this.branch(lb, e.body), SP, this.tok(catchI), SP];
    let hlb = catchI + 1;
    if (e.name !== null) {
      const lp = catchI + 1;
      const rp = this.match.get(lp)!;
      out.push(this.tok(lp), this.tok(lp + 1), this.tok(rp), SP);
      hlb = rp + 1;
    }
    out.push(this.branch(hlb, e.handler));
    return group(out);
  }

  private matchExpr(e: A.Match): Doc {
    const kw = this.at(e.span.start);
    const s = e.subject;
    const lb = this.find("{", this.outer(s).end);
    const rb = this.match.get(lb)!;
    const subjText = this.subjectNeedsParens(s);
    const inner: Doc[] = [];
    e.arms.forEach((arm, n) => {
      const first = this.at(arm.span.start);
      if (n > 0) this.elementStarts.add(first);
      inner.push(L);
      if (n > 0 && this.nlBefore[first] >= 2 && !this.lead.has(first)) inner.push(BLANK);
      const el = this.element(this.arm(arm));
      inner.push(el.d, n < e.arms.length - 1 ? "," : ib(",", ""), el.trail);
    });
    this.closers.add(rb);
    inner.push(this.leadMark(rb));
    return group([this.tok(kw), SP, subjText ? ["(", this.expr(s), ")"] : this.expr(s), SP, this.tok(lb), indent(inner), L, this.tok(rb, { noLead: true })]);
  }

  /** Whether the text of a match subject would begin with `{`, `if` or `try` (f12 2.6). */
  private subjectNeedsParens(s: A.Expr): boolean {
    let x: A.Expr = s;
    // Whether the run of the Binary being read may have pairs for the reader.
    let pairsOK = false;
    let prev: A.Expr | null = null;
    for (;;) {
      if (x.kind === "Map" || x.kind === "Block" || x.kind === "Try" || (x.kind === "Cond" && x.form === "if")) return true;
      if (x.kind === "Cond" && x.form === "?:") {
        if (this.level(x.test) === 1) return false;
        x = x.test;
        continue;
      }
      if (x.kind === "Binary") {
        if (prev === null || prev.kind !== "Binary" || !sameRun(prev, x)) pairsOK = !this.plain && this.rdepth === 0 && runLen(x) <= 3;
        if (this.needLeft(x.x, x) || (pairsOK && readerRow(x.x, x, "left"))) return false;
        prev = x;
        x = x.x;
        continue;
      }
      if (x.kind === "As") {
        x = x.x;
        continue;
      }
      if (x.kind === "Member" || x.kind === "Index" || x.kind === "MethodCall") {
        if (this.receiverNeedsParens(x.x)) return false;
        x = x.x;
        continue;
      }
      if (x.kind === "OptChain") {
        x = x.x;
        continue;
      }
      if (x.kind === "Each" && x.x !== null) {
        if (this.receiverNeedsParens(x.x)) return false;
        x = x.x;
        continue;
      }
      return false;
    }
  }

  private arm(arm: A.Arm): Doc {
    const parts: Doc[] = [];
    if (arm.isDefault) parts.push(this.tok(this.at(arm.span.start)));
    else {
      arm.literals.forEach((l, n) => {
        if (n > 0) parts.push(SP, this.tok(this.find("|", this.outer(arm.literals[n - 1]).end)), SP);
        const i = this.at(l.span.start);
        if (this.toks[i].text === "-") parts.push(this.tok(i), this.tok(i + 1));
        else parts.push(this.tok(i));
      });
    }
    const lastLitEnd = arm.isDefault ? this.toks[this.at(arm.span.start)].end : this.outer(arm.literals[arm.literals.length - 1]).end;
    // Package F17 (f17 7.1): sp "if" sp "(" g(i(S ‹guard›) S) ")", the guard a run that is unindented.
    let guard: Doc = "";
    let guardEnd = lastLitEnd;
    if (arm.guard !== null) {
      const ifI = this.find("if", lastLitEnd);
      const lp = ifI + 1;
      const rp = this.match.get(lp)!;
      guard = [SP, this.tok(ifI), SP, this.tok(lp), group([indent([S, this.expr(arm.guard, { unindented: true })]), S]), this.tok(rp)];
      guardEnd = this.toks[rp].end;
    }
    const arrow = this.find("=>", guardEnd);
    return [parts, guard, SP, this.tok(arrow), this.assigned(arm.result)];
  }

  private lambda(e: A.Lambda): Doc {
    const arrow = this.find("=>", e.result !== null ? e.result.span.end : e.params.length > 0 ? this.paramEnd(e.params[e.params.length - 1]) : e.span.start);
    // f16 7.1: with an annotation the rule of one bare parameter does not apply.
    const annotated = e.result !== null || e.params.some((p) => p.type !== null || p.fn !== null);
    const single = !annotated && e.params.length === 1 && e.params[0].pattern.kind === "NamePat" && e.params[0].default === null;
    let params: Doc;
    const firstI = this.at(e.span.start);
    if (single) {
      const nameI = this.at(e.params[0].pattern.span.start);
      params = this.tok(nameI);
    } else {
      const lp = firstI;
      const rp = this.match.get(lp)!;
      params = this.seq(lp, e.params.map((p) => ({ d: this.param(p), first: this.at(p.pattern.span.start) })), rp);
      // f16 7.1: params `":" sp type sp "=>" assigned(body)`.
      if (e.result !== null) params = [params, this.tok(rp + 1), SP, this.typeDoc(e.result.span)];
    }
    return [params, SP, this.tok(arrow), this.assigned(e.body)];
  }

  /** The end of a parameter: its default, its annotation or its pattern (f16 2.3 item 1). */
  private paramEnd(p: A.Param): number {
    return p.span.end;
  }

  private param(p: A.Param): Doc {
    let pat = this.pattern(p.pattern);
    // f16 7.1: `‹pattern› ":" sp type`, or a function type on one line.
    const ann = p.type !== null ? p.type.span : p.fn !== null ? p.fn.span : null;
    if (ann !== null) pat = [pat, this.tok(this.find(":", p.pattern.span.end)), SP, p.fn !== null ? this.fnDoc(p.fn) : this.typeDoc(ann)];
    if (p.default === null) return pat;
    const eq = this.find("=", ann !== null ? ann.end : p.pattern.span.end);
    return [pat, SP, this.tok(eq), this.assigned(p.default)];
  }

  /** f16 7.1: `"(" w ":" sp type { "," sp w ":" sp type } ")" sp "=>" sp result`, on one line. */
  private fnDoc(fn: A.FnAnn): Doc {
    const lp = this.at(fn.span.start);
    const rp = this.match.get(lp)!;
    const out: Doc[] = [this.tok(lp)];
    // f16 7.1 item 4: a parameter is as a field of a program type: its trailing comments go after its comma.
    let trail: Doc = "";
    fn.params.forEach((fp, n) => {
      const w = this.at(fp.span.start);
      if (n > 0) out.push(this.tok(w - 1, { noTrail: true }), trail, this.trailMark(w - 1), SP);
      const el = this.element([this.tok(w), this.tok(w + 1), SP, this.typeDoc(fp.type.span)]);
      if (n < fn.params.length - 1) {
        out.push(el.d);
        trail = el.trail;
      } else out.push(el.d, el.trail);
    });
    out.push(this.tok(rp), SP, this.tok(rp + 1), SP, this.typeDoc(fn.resultSpan));
    return out;
  }

  pattern(p: A.Pattern): Doc {
    switch (p.kind) {
      case "NamePat":
        return this.tok(this.at(p.span.start));
      case "MapPat": {
        const lb = this.at(p.span.start);
        const rb = this.match.get(lb)!;
        const xs: { d: Doc; first: number }[] = p.fields.map((f) => {
          const keyI = this.at(f.keySpan.start);
          const nameI = this.at(f.nameSpan.start);
          let d: Doc;
          const keyTok = this.toks[keyI];
          if (keyI === nameI || (keyTok.kind === "word" && keyTok.text === f.name)) d = this.tok(nameI);
          else d = [this.tok(keyI), this.tok(this.find(":", keyTok.end)), SP, this.tok(nameI)];
          if (f.default !== null) d = [d, SP, this.tok(this.find("=", f.nameSpan.end)), this.assigned(f.default)];
          return { d, first: keyI };
        });
        if (p.rest !== null) {
          const dots = this.find("...", p.fields.length > 0 ? p.fields[p.fields.length - 1].span.end : p.span.start);
          xs.push({ d: [this.tok(dots), this.tok(dots + 1)], first: dots });
        }
        return this.seq(lb, xs, rb);
      }
      case "ListPat": {
        const lb = this.at(p.span.start);
        const rb = this.match.get(lb)!;
        const xs: { d: Doc; first: number }[] = p.elems.map((el) => {
          const nameI = this.at(el.pat.span.start);
          let d: Doc = this.tok(nameI);
          if (el.default !== null) d = [d, SP, this.tok(this.find("=", el.pat.span.end)), this.assigned(el.default)];
          return { d, first: nameI };
        });
        if (p.rest !== null) {
          const dots = this.find("...", p.elems.length > 0 ? p.elems[p.elems.length - 1].span.end : p.span.start);
          xs.push({ d: [this.tok(dots), this.tok(dots + 1)], first: dots });
        }
        return this.seq(lb, xs, rb);
      }
    }
  }

  /** A program type, on one line from its tokens (f12 2.13). */
  private typeDoc(span: Span): Doc {
    let i = this.at(span.start);
    const out: Doc[] = [];
    // A comma that is written: one before `}` is dropped.
    const keptComma = (j: number): boolean => {
      const c = this.toks[j];
      if (c === undefined || c.start >= span.end || c.kind !== "punct" || c.text !== ",") return false;
      const n = this.toks[j + 1];
      return !(n !== undefined && n.text === "}" && n.start < span.end + 1);
    };
    // F12 2.12 item 5: the trailing comments of a field's last token are written after its comma.
    let moved = -1;
    while (i < this.toks.length && this.toks[i].start < span.end) {
      const t = this.toks[i];
      if (t.kind === "punct" && t.text === ",") {
        if (!keptComma(i)) {
          i++;
          continue;
        }
        out.push(this.tok(i, { noTrail: true }), moved >= 0 ? this.trailMark(moved) : "", this.trailMark(i), SP);
        moved = -1;
      } else if (keptComma(i + 1)) {
        out.push(this.tok(i, { noTrail: true }));
        moved = i;
      } else if (t.kind === "punct" && t.text === "|") out.push(SP, this.tok(i), SP);
      else if (t.kind === "punct" && t.text === ":") out.push(this.tok(i), SP);
      else out.push(this.tok(i));
      i++;
    }
    return out;
  }

  // ---- chains (f12 2.11) -------------------------------------------------------

  /** The receiver of a step needs parentheses (f12 2.6). */
  private receiverNeedsParens(x: A.Expr, step?: A.Expr): boolean {
    if (this.level(x) < 11) return true;
    if (x.kind === "OptChain") return true;
    // A method call ends a projection, so a star before it needs none (f07 3.4).
    if (x.kind === "Each") return step === undefined || step.kind !== "MethodCall";
    return false;
  }

  private chain(e: A.Expr): Doc {
    // The chain's nodes from its primary out: a receiver written in
    // parentheses that 2.6 does not need is part of the same chain.
    const nodes: A.Expr[] = [];
    let x: A.Expr = e.kind === "OptChain" ? e.x : e;
    for (;;) {
      let next: A.Expr | null = null;
      if (x.kind === "Member" || x.kind === "Index" || x.kind === "MethodCall") next = x.x;
      else if (x.kind === "Each" && x.x !== null) next = x.x;
      else break;
      nodes.push(x);
      if (this.receiverNeedsParens(next, x)) {
        x = next;
        break;
      }
      x = next;
    }
    nodes.reverse();
    if (nodes.length === 0) return x.kind === "Call" ? this.call(x) : this.expr(x);
    const primary: Doc = x.kind === "Call" ? this.call(x) : this.paren(x, this.receiverNeedsParens(x));
    const steps: { kind: "plain" | "call"; d: Doc; firstTok: number }[] = [];
    for (const n of nodes) for (const st of this.stepsOf(n)) steps.push(st);
    const links: Doc[][] = [];
    const recvPart: Doc[] = [primary];
    const linkFirst: number[] = [];
    for (const st of steps) {
      if (st.kind === "call") {
        links.push([st.d]);
        linkFirst.push(st.firstTok);
      } else if (links.length === 0) recvPart.push(st.d);
      else links[links.length - 1].push(st.d);
    }
    const leadComment = linkFirst.some((i) => this.lead.has(i));
    if (links.length === 0 || (links.length === 1 && !leadComment)) return [recvPart, links];
    const c2: Doc = brokenGroup([recvPart, indent(links.map((l) => [S, l]))]);
    if (leadComment) return c2;
    const c1: Doc = [recvPart, links];
    const must = links.slice(0, -1).some((l) => forced(l));
    return choice([c1, c2], must);
  }

  /** The documents of the steps of one chain node. */
  private stepsOf(s: A.Expr): { kind: "plain" | "call"; d: Doc; firstTok: number }[] {
    switch (s.kind) {
      case "Member": {
        const nameI = this.at(s.nameSpan.start);
        return [{ kind: "plain", d: [this.tok(nameI - 1), this.tok(nameI)], firstTok: nameI - 1 }];
      }
      case "Index": {
        const lb = this.find(s.optional ? "?.[" : "[", this.outer(s.x).end);
        const rb = this.match.get(lb)!;
        return [{ kind: "plain", d: [this.tok(lb), this.expr(s.index), this.tok(rb)], firstTok: lb }];
      }
      case "MethodCall": {
        const nameI = this.at(s.nameSpan.start);
        const lp = nameI + 1;
        return [{ kind: "call", d: [this.tok(nameI - 1), this.tok(nameI), this.args(lp, s.args)], firstTok: nameI - 1 }];
      }
      case "Each": {
        const out: { kind: "plain" | "call"; d: Doc; firstTok: number }[] = [];
        const star = (from: number) => {
          let lb = this.firstFrom(from);
          while (!(this.toks[lb].text === "[" || this.toks[lb].text === "?.[") || this.toks[lb + 1]?.text !== "*") lb++;
          const rb = this.match.get(lb)!;
          return { lb, rb };
        };
        let pos = s.x !== null ? this.outer(s.x).end : s.starSpan.start;
        const visit = (each: A.Each, from: number) => {
          const st = star(from);
          out.push({ kind: "plain", d: [this.tok(st.lb), this.tok(st.lb + 1), this.tok(st.rb)], firstTok: st.lb });
          let p = this.toks[st.rb].end;
          for (const r of each.reads) {
            if (r.kind === "Each") {
              p = visit(r, p);
              continue;
            }
            if (r.kind === "Member") {
              const nameI = this.at(r.nameSpan.start);
              out.push({ kind: "plain", d: [this.tok(nameI - 1), this.tok(nameI)], firstTok: nameI - 1 });
              p = this.toks[nameI].end;
            } else {
              const lb = this.find(r.optional ? "?.[" : "[", p);
              const rb = this.match.get(lb)!;
              out.push({ kind: "plain", d: [this.tok(lb), this.expr(r.index), this.tok(rb)], firstTok: lb });
              p = this.toks[rb].end;
            }
          }
          return p;
        };
        pos = visit(s, pos);
        return out;
      }
      default:
        return [];
    }
  }

  private call(e: A.Call): Doc {
    const first = this.at(e.span.start);
    const nameI = this.at(e.nameSpan.start);
    const name: Doc = e.namespace !== null && first !== nameI ? [this.tok(first), this.tok(first + 1), this.tok(nameI)] : this.tok(nameI);
    return [name, this.args(nameI + 1, e.args)];
  }

  /** The arguments of a call (f12 2.10). */
  private args(lp: number, args: A.Arg[]): Doc {
    const rp = this.match.get(lp)!;
    const xs = args.map((a) => ({ d: this.expr(a), first: this.firstTok(a) }));
    const last = args[args.length - 1];
    if (last === undefined || !huggable(last)) return this.seq(lp, xs, rp);
    // a1: on one line; a2: the last argument's outer group broken; a3: seq.
    const parts = xs.map((x) => this.element(x.d));
    const lineOf = (lastDoc: Doc): Doc => {
      const out: Doc[] = [this.tok(lp)];
      parts.forEach((p, n) => {
        out.push(n === parts.length - 1 ? lastDoc : p.d);
        if (n < parts.length - 1) out.push(",", p.trail, SP);
        else out.push(p.trail);
      });
      out.push(this.leadMark(rp), this.tok(rp, { noLead: true }));
      return out;
    };
    const a1 = lineOf(parts[parts.length - 1].d);
    const a2 = lineOf(this.hugged(last, parts[parts.length - 1].d));
    const a3 = this.seq(lp, xs, rp);
    const must = xs.slice(0, -1).some((x) => forced(x.d));
    return choice([a1, a2, a3], must);
  }

  /** The last argument with its outer group broken (f12 2.10 item 2). */
  private hugged(a: A.Arg, d: Doc): Doc {
    const breakOuter = (x: Doc): Doc => {
      // The outer group of the document; a Block has none, and always breaks.
      if (typeof x === "string" || Array.isArray(x)) return x;
      const p = x as Exclude<Doc, string | Doc[]>;
      if (p.k === "group") return { k: "group", d: p.d, broken: true };
      return x;
    };
    if (a.kind === "Lambda") {
      // The outer group of a lambda is that of its body: the last group of its document.
      const arr = d as Doc[];
      const assignedBody = arr[arr.length - 1] as Doc[];
      return [...arr.slice(0, -1), [assignedBody[0], breakOuter(assignedBody[1])]];
    }
    return breakOuter(d);
  }
}

function huggable(a: A.Arg): boolean {
  if (a.kind === "Map" || a.kind === "List" || a.kind === "Block") return true;
  if (a.kind === "Lambda") {
    const b = a.body;
    return b.kind === "Map" || b.kind === "List" || b.kind === "Block" || b.kind === "Match" || (b.kind === "Cond" && b.form === "if") || b.kind === "Try";
  }
  return false;
}

/** The level of a binary operator (01 3.3). */
/** Whether x has a place for a pair for the reader as an operand of parent (f12 2.6, the second table). */
function readerRow(x: A.Expr, parent: A.Binary, side: "left" | "right"): boolean {
  if (x.kind !== "Binary") return false;
  if (x.op === "&&" && parent.op === "||") return true;
  return side === "left" && isMul(x) && isMul(parent) && (x.op !== parent.op || x.op === "%");
}

function isMul(e: A.Expr | A.As): boolean {
  return e.kind === "Binary" && (e.op === "*" || e.op === "/" || e.op === "%");
}

/** Whether child, the left operand of parent, is in parent's run: both of `*`, `/` and `%`, or both `||` (f12 2.6 item 3). */
function sameRun(parent: A.Expr | A.As, child: A.Expr | A.As): boolean {
  if (parent.kind !== "Binary" || child.kind !== "Binary") return false;
  return (isMul(parent) && isMul(child)) || (parent.op === "||" && child.op === "||");
}

/** The number of operators of the run whose top is n: n and its left operands of the same run. */
function runLen(n: A.Binary): number {
  let k = 1;
  let x: A.Expr = n;
  while (x.kind === "Binary" && sameRun(x, x.x)) {
    k++;
    x = x.x;
  }
  return k;
}

/** Whether e is a comparison: a Binary of level 5 or 6 (01 3.3). */
function isComparison(e: A.Expr): boolean {
  return e.kind === "Binary" && (binLevel(e.op) === 5 || binLevel(e.op) === 6);
}

function binLevel(op: A.BinaryOp): number {
  switch (op) {
    case "??":
      return 2;
    case "||":
      return 3;
    case "&&":
      return 4;
    case "==":
    case "!=":
      return 5;
    case "<":
    case "<=":
    case ">":
    case ">=":
    case "in":
    case "not-in":
      return 6;
    case "+":
    case "-":
      return 7;
    case "*":
    case "/":
    case "%":
      return 8;
    case "**":
      return 10;
  }
}

/** The keys a tree's equality ignores: spans, and the levels that parentheses change (f12 2.2 item 1). */
const IGNORED = new Set(["span", "outer", "parens", "nameSpan", "keySpan", "typeSpan", "typeSpans", "starSpan", "recvSpan", "level", "maxLevel", "resultSpan", "spans"]);

/** Two trees are equal, spans aside (f12 2.2 item 1). */
export function treeEqual(a0: unknown, b0: unknown): boolean {
  // The pairs still to compare. A tree is as deep as a flat program is long (01 13.1), so the walk keeps its own stack.
  const as: unknown[] = [a0];
  const bs: unknown[] = [b0];
  while (as.length > 0) {
    const a = as.pop();
    const b = bs.pop();
    if (a === b) continue;
    if (typeof a === "number" && typeof b === "number") {
      if (Number.isNaN(a) && Number.isNaN(b)) continue;
      return false;
    }
    if (typeof a !== "object" || typeof b !== "object" || a === null || b === null) return false;
    if (Array.isArray(a) !== Array.isArray(b)) return false;
    if (Array.isArray(a)) {
      const bb = b as unknown[];
      if (a.length !== bb.length) return false;
      for (let i = 0; i < a.length; i++) {
        as.push(a[i]);
        bs.push(bb[i]);
      }
      continue;
    }
    if (a instanceof Map || b instanceof Map) continue;
    const ra = a as Record<string, unknown>;
    const rb = b as Record<string, unknown>;
    const ka = Object.keys(ra).filter((k) => !IGNORED.has(k) && ra[k] !== undefined);
    const kb = Object.keys(rb).filter((k) => !IGNORED.has(k) && rb[k] !== undefined);
    if (ka.length !== kb.length) return false;
    for (const k of ka) {
      as.push(ra[k]);
      bs.push(rb[k]);
    }
  }
  return true;
}
