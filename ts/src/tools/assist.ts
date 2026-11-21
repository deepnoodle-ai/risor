// The assist stage of package F12 (f12 3): completion and hover at a position
// of a text that need not parse. The text before the cut is lexed, a hole is
// put at the cut for completion, the tokens are repaired into a program by
// inserting tokens at the end, and the program is checked; the candidates
// and the hover record are read from its tree, its scopes and its types.

import type * as A from "../syntax/ast.ts";
import type { Env } from "../host/api.ts";
import { printSignature } from "../host/api.ts";
import { ParseError } from "../syntax/diagnostic.ts";
import { lexCut } from "../syntax/lexer.ts";
import { parse, parseTokens } from "../syntax/parser.ts";
import { Source } from "../syntax/source.ts";
import type { LineSpan, Position, Span } from "../syntax/source.ts";
import { HOLE } from "../syntax/token.ts";
import type { Token } from "../syntax/token.ts";
import { FUNCTIONS, GROUP_CONSTANTS, GROUP_FUNCTIONS, METHODS } from "../library/registry.ts";
import { fromNode, isAny, isNever, join, joinAll, nonnull, opaque, PairLimit, printType, printUnfolded, soleRef, unfold } from "../types/type.ts";
import type { TypeNode } from "../types/typetext.ts";
import type { Alts, Rec, Type } from "../types/type.ts";
import type { Resolution } from "../eval/static.ts";
import type { CheckResult } from "../types/check.ts";
import { printFnType } from "../types/check.ts";
import { compareText } from "../value/value.ts";
import { GROUP_LIBRARY } from "../generated/library-groups.ts";
import { MATH_LIBRARY } from "../generated/library-math.ts";

export interface Candidate {
  label: string;
  kind: string;
  detail: string | null;
  insert: string;
}

export interface CompletionResult {
  span: LineSpan;
  items: Candidate[];
}

export interface HoverResult {
  kind: string;
  span: LineSpan;
  type: string | null;
  signature: string | null;
  doc: string | null;
  declaration: LineSpan | null;
}

const WORD = /[A-Za-z0-9_]/;
const FAR = 1 << 30;

// ---- the cut and the repair (f12 3.2, 3.3) -------------------------------------

/** The offset of a position, or -1 when it is in no line of the text. */
function offsetOf(src: Source, pos: Position): number {
  let line = 1;
  let i = 0;
  while (line < pos.line) {
    if (i >= src.length) return -1;
    if (src.cps[i] === 0x0a) line++;
    i++;
  }
  let lineEnd = i;
  while (lineEnd < src.length && src.cps[lineEnd] !== 0x0a) lineEnd++;
  const off = i + pos.column - 1;
  if (pos.column < 1 || off > lineEnd) return -1;
  return off;
}

/** Every token list of a lexed text: the top list and those of the open interpolations. */
function innermost(tokens: Token[]): Token[] {
  let list = tokens;
  for (;;) {
    const real = list.filter((t) => t.kind !== "eof" && !(t.kind === "interpEnd" && t.cut));
    const last = real[real.length - 1];
    if (last === undefined || last.kind !== "template" || last.parts === undefined) return list;
    const p = last.parts[last.parts.length - 1];
    if (p === undefined || p.kind !== "interp") return list;
    const endTok = p.tokens[p.tokens.length - 1];
    if (endTok === undefined || !endTok.cut) return list;
    list = p.tokens;
  }
}

/** The lists that hold a cut end, with the end token. */
function allLists(tokens: Token[], out: Token[][] = []): Token[][] {
  out.push(tokens);
  for (const t of tokens) if (t.kind === "template" && t.parts !== undefined) for (const p of t.parts) if (p.kind === "interp") allLists(p.tokens, out);
  return out;
}

const INSERTS: [Token["kind"], string][] = [
  ["word", "null"], ["punct", ")"], ["punct", "]"], ["punct", "}"], ["punct", ";"], ["punct", ":"],
  ["punct", "=>"], ["punct", "="], ["punct", "{"], ["word", "else"], ["word", "catch"], ["punct", ">"],
];

/** The repair (f12 3.3): the program, or null when it fails. */
function repair(tokens: Token[], src: Source): A.Program | null {
  // The ends of the text: the end of the program and of each open interpolation.
  let k = 0;
  for (const list of allLists(tokens)) {
    for (const t of list) {
      if (t.kind === "eof" || (t.kind === "interpEnd" && t.cut)) {
        t.cut = true;
        t.start = t.end = FAR + 1000 * k++;
      }
    }
  }
  let next = FAR - 100000;
  for (let step = 0; step < 400; step++) {
    let err: ParseError;
    try {
      return parseTokens(tokens, src);
    } catch (e) {
      if (!(e instanceof ParseError)) throw e;
      err = e;
    }
    const at = err.diagnostic.span?.start ?? -1;
    if (at < FAR) return null; // a diagnostic before the end, the hole included
    // The list and the place of the end token.
    let list: Token[] | null = null;
    let idx = -1;
    for (const l of allLists(tokens)) {
      const i = l.findIndex((t) => t.cut === true && t.start === at && (t.kind === "eof" || t.kind === "interpEnd"));
      if (i >= 0) {
        list = l;
        idx = i;
        break;
      }
    }
    if (list === null) return null;
    // The closer first, then the list.
    const opens: string[] = [];
    for (let i = 0; i < idx; i++) {
      const t = list[i];
      if (t.kind !== "punct") continue;
      if (t.text === "(" || t.text === "[" || t.text === "{" || t.text === "?.[") opens.push(t.text);
      else if (t.text === ")" || t.text === "]" || t.text === "}") opens.pop();
    }
    const closer = opens.length > 0 ? { "(": ")", "[": "]", "{": "}", "?.[": "]" }[opens[opens.length - 1]]! : null;
    const tries: [Token["kind"], string][] = closer !== null ? [["punct", closer], ...INSERTS] : INSERTS;
    let accepted = false;
    for (const [kind, text] of tries) {
      const pos = next++;
      const tok: Token = { kind, start: pos, end: pos, text, gap: true, cut: true };
      list.splice(idx, 0, tok);
      try {
        parseTokens(tokens, src);
        accepted = true;
      } catch (e) {
        if (!(e instanceof ParseError)) throw e;
        const s = e.diagnostic.span?.start ?? -1;
        accepted = s >= FAR;
      }
      if (accepted) break;
      list.splice(idx, 1);
    }
    if (!accepted) return null;
  }
  return null;
}

// ---- walking the tree ---------------------------------------------------------

type Visit = (n: A.Node, parents: () => A.Node[]) => boolean | void;

/**
 * Every node of a tree, a parent before its children and the children in
 * order. `parents` gives the parents of the node, the outermost first. A visit
 * that gives false leaves the node's children out.
 *
 * The walk keeps its own stack: a chain of steps or of operators is as deep in
 * the tree as it is long, and it opens no level (01 13.1).
 */
function walk(root: A.Node, visit: Visit): void {
  const path: A.Node[] = [];
  const kids: (A.Node | null | undefined)[][] = [];
  const at: number[] = [];
  const parents = (): A.Node[] => path.slice();
  const enter = (n: A.Node | null | undefined): void => {
    if (n === null || n === undefined) return;
    if (visit(n, parents) === false) return;
    path.push(n);
    kids.push(children(n));
    at.push(0);
  };
  enter(root);
  while (path.length > 0) {
    const top = path.length - 1;
    const ks = kids[top];
    if (at[top] < ks.length) enter(ks[at[top]++]);
    else {
      path.pop();
      kids.pop();
      at.pop();
    }
  }
}

function children(n: A.Node): A.Node[] {
  switch (n.kind) {
    case "Program":
    case "Block":
      return [...n.consts, n.result];
    case "Const":
      return [n.pattern, n.value];
    case "List":
      return n.items;
    case "Map":
      return n.entries;
    case "Entry":
      return n.key.kind === "expr" ? [n.key.expr, n.value] : [n.value];
    case "Spread":
      return [n.value];
    case "Unary":
      return [n.x];
    case "Binary":
      return [n.x, n.y];
    case "Cond":
      return [n.test, n.then, n.else];
    case "Member":
      return [n.x];
    case "Index":
      return [n.x, n.index];
    case "Call":
      return n.args;
    case "MethodCall":
      return [n.x, ...n.args];
    case "OptChain":
      return [n.x];
    case "Lambda":
      return [...n.params, n.body];
    case "Param":
      return n.default !== null ? [n.pattern, n.default] : [n.pattern];
    case "Match":
      return [n.subject, ...n.arms];
    case "Arm":
      return n.guard !== null ? [...n.literals, n.guard, n.result] : [...n.literals, n.result];
    case "Try":
      return [n.body, n.handler];
    case "MapPat":
      return n.fields.flatMap((f) => (f.default !== null ? [f.default] : []));
    case "ListPat":
      return n.elems.flatMap((e) => [e.pat, ...(e.default !== null ? [e.default] : [])]);
    case "As":
      return [n.x];
    case "Each":
      return [...(n.x !== null ? [n.x] : []), ...n.reads.flatMap((r) => (r.kind === "Index" ? [r.index] : r.kind === "Each" ? [r] : []))];
    case "Template":
      return n.parts.filter((p): p is A.Expr => typeof p !== "string");
    default:
      return [];
  }
}

// ---- scopes (f12 3.6) ------------------------------------------------------------

interface Bound {
  name: string;
  /** The binding: a NamePat, a field, an element, a rest, or a Try for `catch`. */
  node: object;
  span: Span;
  lambda: boolean;
}

/** The names a pattern binds, in order, each with its binding node. */
function patternNames(p: A.Pattern): Bound[] {
  switch (p.kind) {
    case "NamePat":
      return p.name === null ? [] : [{ name: p.name, node: p, span: p.span, lambda: false }];
    case "MapPat": {
      const out: Bound[] = p.fields.map((f) => ({ name: f.name, node: f, span: f.nameSpan, lambda: false }));
      if (p.rest !== null) out.push({ name: p.rest.name, node: p.rest, span: p.rest.span, lambda: false });
      return out;
    }
    case "ListPat": {
      const out: Bound[] = p.elems.filter((e) => e.pat.name !== null).map((e) => ({ name: e.pat.name!, node: e.pat, span: e.pat.span, lambda: false }));
      if (p.rest !== null) out.push({ name: p.rest.name, node: p.rest, span: p.rest.span, lambda: false });
      return out;
    }
  }
}

/**
 * A scope while it is searched: the first `len` names of `scope`, then `own`.
 * The copy is made only for the scopes of the target, so a block of many
 * `const` lines costs one copy, not one for each line.
 */
interface Open {
  scope: Bound[];
  len: number;
  own: Bound[];
}

const NO_OWN: Bound[] = [];

/** The scopes in force at a target node: from the innermost out, each in binding order. */
function scopesAt(root: A.Program, target: object): Bound[][] | null {
  let found: Bound[][] | null = null;
  const find = (scopes: Open[]): void => {
    found = scopes.map((o) => (o.len === o.scope.length && o.own.length === 0 ? o.scope : [...o.scope.slice(0, o.len), ...o.own]));
  };
  /** The scope as it is now: a later name is not in it. */
  const now = (scope: Bound[], own: Bound[] = NO_OWN): Open => ({ scope, len: scope.length, own });
  const lines = (consts: A.Const[], result: A.Expr, scopes: Open[]): void => {
    const scope: Bound[] = [];
    for (const c of consts) {
      // A const name is not in scope in its own value; a pattern's name is in the defaults to its right.
      // With the Core amendment, a const-bound lambda's name is in scope in its body (f16 7.2 item 5).
      visitPattern(c.pattern, [now(scope), ...scopes]);
      const own = c.value.kind === "Lambda" && c.pattern.kind === "NamePat" ? patternNames(c.pattern).map((b) => ({ ...b, lambda: true })) : NO_OWN;
      visit(c.value as A.Node, [now(scope, own), ...scopes]);
      if (found !== null) return;
      for (const b of patternNames(c.pattern)) scope.push({ ...b, lambda: c.value.kind === "Lambda" && c.pattern.kind === "NamePat" });
    }
    visit(result, [now(scope), ...scopes]);
  };
  const visitPattern = (p: A.Pattern, scopes2: Open[]): void => {
    if (p.kind === "MapPat") {
      const left: Bound[] = [];
      for (const f of p.fields) {
        if (f.default !== null) visit(f.default, [now(left), ...scopes2]);
        if (found !== null) return;
        left.push({ name: f.name, node: f, span: f.nameSpan, lambda: false });
      }
    } else if (p.kind === "ListPat") {
      const left: Bound[] = [];
      for (const e of p.elems) {
        if (e.default !== null) visit(e.default, [now(left), ...scopes2]);
        if (found !== null) return;
        if (e.pat.name !== null) left.push({ name: e.pat.name, node: e.pat, span: e.pat.span, lambda: false });
      }
    } else if (p === target) find(scopes2);
  };
  // The nodes that bind no name are searched by a loop with its own stack: a
  // chain of steps or of operators is as deep in the tree as it is long, and
  // it opens no level (01 13.1). A program, a block, a lambda and a `try` call
  // `visit` again; each is inside a bracket, so they nest at most 100 deep.
  const visit = (start: A.Node | null, scopes: Open[]): void => {
    const stack: (A.Node | null)[] = [start];
    while (stack.length > 0) {
      const n = stack.pop()!;
      if (found !== null) return;
      if (n === null) continue;
      if (n === target) {
        find(scopes);
        return;
      }
      switch (n.kind) {
        case "Program":
        case "Block":
          lines(n.consts, n.result, scopes);
          break;
        case "Lambda": {
          const scope: Bound[] = [];
          let done = false;
          for (const p of n.params) {
            if (p.pattern === target) {
              find([now(scope), ...scopes]);
              return;
            }
            visitPattern(p.pattern, [now(scope), ...scopes]);
            if (p.default !== null) visit(p.default, [now(scope), ...scopes]);
            if (found !== null) {
              done = true;
              break;
            }
            for (const b of patternNames(p.pattern)) scope.push(b);
          }
          if (!done) visit(n.body, [now(scope), ...scopes]);
          break;
        }
        case "Try": {
          visit(n.body, scopes);
          const scope: Bound[] = n.name !== null && n.nameSpan !== null ? [{ name: n.name, node: n, span: n.nameSpan, lambda: false }] : [];
          visit(n.handler, [now(scope), ...scopes]);
          break;
        }
        default: {
          const cs = children(n);
          for (let i = cs.length - 1; i >= 0; i--) stack.push(cs[i]);
        }
      }
    }
  };
  visit(root, []);
  return found;
}

// ---- types -----------------------------------------------------------------------

/** The alternatives of a type's unfolding (f17 12.1): a recursive type is read through its definition. */
function alts(t: Type): Alts | null {
  if (isAny(t)) return null;
  const u = unfold(t);
  return isAny(u) ? null : u;
}

/** The record alternatives of a type: the variants of a union, or its record. */
function records(t: Type): Rec[] {
  const a = alts(t);
  if (a === null || a.map === undefined) return [];
  if (a.union !== undefined) return [...a.union.variants];
  return [a.map];
}

/** The keys the records list, each with the Join of its field types. */
function keysOf(recs: Rec[]): Map<string, Type> {
  const out = new Map<string, Type>();
  for (const r of recs) for (const [k, f] of r.fields) out.set(k, out.has(k) ? join(out.get(k)!, f.type) : f.type);
  return out;
}

function isWord(s: string): boolean {
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(s);
}

// ---- the library's records -----------------------------------------------------

const GROUP_DOCS = new Map<string, string>([...GROUP_LIBRARY, ...MATH_LIBRARY].map((r) => [r.name, String(r.doc ?? "")]));

function recordDoc(r: { [k: string]: unknown }): string | null {
  return typeof r.doc === "string" ? r.doc : null;
}

// ---- completion (f12 3.5 to 3.12) ------------------------------------------------

/** Completion at a position (f12 3.1). */
export function complete(env: Env, text: string, pos: Position, returns?: string): CompletionResult {
  const src = new Source(text);
  const cut = offsetOf(src, pos);
  const span = (a: number, b: number): LineSpan => src.lineSpan({ start: a, end: b });
  if (cut < 0) return { span: [pos.line, pos.column, pos.line, pos.column], items: [] };
  // The prefix by the code points, used when the text before the cut does not lex.
  let ws = cut;
  while (ws > 0 && WORD.test(String.fromCodePoint(src.cps[ws - 1]))) ws--;
  const scanPrefix = ws < cut && !/[0-9]/.test(String.fromCodePoint(src.cps[ws])) ? ws : cut;
  const empty = (s = scanPrefix): CompletionResult => ({ span: span(s, cut), items: [] });
  const before = new Source(src.slice(0, cut), { keepBom: true });
  let tokens: Token[];
  let holeString: { quote: string; value: string; start: number } | null = null;
  try {
    tokens = lexCut(before);
  } catch (e) {
    if (!(e instanceof ParseError)) throw e;
    const d = e.diagnostic;
    // A cut inside a one-line string: the string is the hole (f12 3.4 item 3).
    const s0 = d.span?.start ?? -1;
    const q = s0 >= 0 ? String.fromCodePoint(src.cps[s0]) : "";
    if (d.code === "unterminated-string" && d.span !== undefined && d.span.end === cut && (q === '"' || q === "'") && !(s0 + 2 < cut && src.slice(s0, s0 + 3) === q.repeat(3))) {
      try {
        tokens = lexCut(new Source(src.slice(0, s0), { keepBom: true }));
      } catch {
        return empty();
      }
      holeString = { quote: q, value: src.slice(s0 + 1, cut), start: s0 };
    } else return empty();
  }
  const list = innermost(tokens);
  const endIdx = list.findIndex((t) => t.kind === "eof" || (t.kind === "interpEnd" && t.cut));
  const lastReal = list[endIdx - 1];
  // A cut inside a comment, or at the end of a number.
  const tail = src.slice(lastReal === undefined ? 0 : lastReal.end, holeString !== null ? holeString.start : cut);
  if (holeString === null && inComment(tail)) return empty(cut);
  if (holeString === null && lastReal !== undefined && lastReal.end === cut && lastReal.kind === "number") return empty(cut);
  let prefixStart = cut;
  let hole: Token;
  if (holeString !== null) {
    prefixStart = holeString.start + 1;
    hole = { kind: "string", start: holeString.start, end: cut, text: HOLE, str: holeString.value, quote: holeString.quote as '"' | "'", gap: true };
    list.splice(endIdx, 0, hole);
  } else if (lastReal !== undefined && lastReal.end === cut && lastReal.kind === "word") {
    prefixStart = lastReal.start;
    hole = { kind: "word", start: lastReal.start, end: cut, text: HOLE, gap: lastReal.gap, holePrefix: lastReal.text };
    list.splice(endIdx - 1, 1, hole);
  } else {
    hole = { kind: "word", start: cut, end: cut, text: HOLE, gap: true };
    list.splice(endIdx, 0, hole);
  }
  const prefix = holeString !== null ? holeString.value : src.slice(prefixStart, cut);
  const result = (items: Candidate[]): CompletionResult => ({ span: span(prefixStart, cut), items: items.filter((c) => c.label.startsWith(prefix)) });
  if (holeString !== null && holeString.value.includes("\\")) {
    // f12 3.2 item 5: a `\` before the cut, unless the context is strings; checked below.
  }
  const prog = repair(tokens, before);
  if (prog === null) {
    // The fallback (f12 3.5 item 1).
    if (holeString !== null) return result([]);
    const idx = list.indexOf(hole);
    const prev = list[idx - 1];
    if (prev !== undefined && prev.kind === "punct" && (prev.text === "." || prev.text === "?.")) return result([]);
    return result(envNames(env, new Set()));
  }
  const { resolution, check } = env.assistCheck(prog, before, returns);
  const ctx = new Context(env, prog, resolution, check, returns);
  if (holeString !== null) ctx.holeQuote = holeString.quote;
  return result(ctx.candidates(hole, holeString !== null));
}

function inComment(tail: string): boolean {
  let i = 0;
  while (i < tail.length) {
    if (tail.startsWith("//", i)) {
      const nl = tail.slice(i).search(/[\r\n]/);
      if (nl < 0) return true;
      i += nl;
      continue;
    }
    if (tail.startsWith("/*", i)) {
      const e = tail.indexOf("*/", i + 2);
      if (e < 0) return true;
      i = e + 2;
      continue;
    }
    i++;
  }
  return false;
}

/** The names of the environment alone (f12 3.6 items 4 to 6). */
function envNames(env: Env, taken: Set<string>): Candidate[] {
  const e = env.environment;
  const out: Candidate[] = [];
  const seen = new Set(taken);
  const add = (c: Candidate) => {
    if (seen.has(c.label)) return;
    seen.add(c.label);
    out.push(c);
  };
  for (const k of [...e.variableTypes.keys()].sort(compareText)) add({ label: k, kind: "variable", detail: printType(e.variableTypes.get(k)!), insert: k });
  const fns: Candidate[] = [];
  for (const [k, f] of FUNCTIONS) if (!e.removed.has(k)) fns.push({ label: k, kind: "function", detail: f.record.signature, insert: k });
  for (const [k, f] of e.functions) if (f.namespace === null) fns.push({ label: k, kind: "function", detail: printSignature(f.sig, e.opaque, e.named), insert: k });
  fns.sort((a, b) => compareText(a.label, b.label));
  for (const c of fns) add(c);
  const ns = new Set<string>([...e.groups, ...e.namespaces]);
  for (const k of [...ns].sort(compareText)) add({ label: k, kind: "namespace", detail: null, insert: k });
  return out;
}

class Context {
  private readonly env: Env;
  private readonly prog: A.Program;
  private readonly res: Resolution;
  private readonly chk: CheckResult;
  private readonly returns: string | undefined;

  constructor(env: Env, prog: A.Program, res: Resolution, chk: CheckResult, returns: string | undefined) {
    this.env = env;
    this.prog = prog;
    this.res = res;
    this.chk = chk;
    this.returns = returns;
  }

  private typeOf(e: object): Type | undefined {
    return this.chk.types.get(e);
  }

  /** The node that holds the hole, and its parents. */
  private locate(hole: Token): { node: A.Node; parents: A.Node[]; where: string } | null {
    let out: { node: A.Node; parents: A.Node[]; where: string } | null = null;
    // f16 7.2 item 1: a hole in the type of a type line, or of an annotation, is in the types context.
    for (const l of this.prog.types) if (typeHasHole(l.type.type)) return { node: this.prog, parents: [], where: "typeline" };
    walk(this.prog, (n, ps) => {
      if (out !== null) return false;
      const at = (s: Span) => s.start === hole.start && s.end === hole.end;
      switch (n.kind) {
        case "Ident":
          if (n.name === HOLE && at(n.span)) out = { node: n, parents: ps(), where: "ident" };
          break;
        case "Member":
        case "MethodCall":
          if (n.name === HOLE) out = { node: n, parents: ps(), where: "member" };
          break;
        case "Entry":
          if (n.shorthand && n.key.kind === "word" && n.key.text === HOLE) out = { node: n, parents: ps(), where: "shorthand" };
          break;
        case "String":
          if (at(n.span) && hole.kind === "string") out = { node: n, parents: ps(), where: "string" };
          break;
        case "As":
          if (typeHasHole(n.type)) out = { node: n, parents: ps(), where: "type" };
          break;
        case "Const":
        case "Param":
        case "Lambda":
          if (annotationsOf(n).some((a) => typeHasHole(a.type))) out = { node: n, parents: ps(), where: "type" };
          break;
      }
      return undefined;
    });
    return out;
  }

  candidates(hole: Token, isString: boolean): Candidate[] {
    try {
      return this.candidates0(hole, isString);
    } catch (e) {
      // A comparison of recursive types past the bound of f17 12.5 item 7: no candidate.
      if (e instanceof PairLimit) return [];
      throw e;
    }
  }

  private candidates0(hole: Token, isString: boolean): Candidate[] {
    const loc = this.locate(hole);
    if (loc === null) return [];
    switch (loc.where) {
      case "ident":
        return this.names(loc.node);
      case "shorthand": {
        const map = loc.parents[loc.parents.length - 1] as A.MapLit;
        const exp = this.expectedOf(map);
        if (exp !== null && records(exp).length > 0) return this.keys(map, exp);
        return this.names(loc.node);
      }
      case "member":
        return this.members(loc.node as A.Member);
      case "type":
        return this.types(hole, false);
      case "typeline":
        return this.types(hole, true);
      case "string":
        return this.strings(loc.node as A.StringLit, loc.parents, isString);
    }
    return [];
  }

  private names(at: A.Node): Candidate[] {
    const scopes = scopesAt(this.prog, at) ?? [];
    const out: Candidate[] = [];
    const seen = new Set<string>();
    for (const scope of scopes) {
      for (const b of scope) {
        if (seen.has(b.name)) continue;
        seen.add(b.name);
        if (b.lambda) out.push({ label: b.name, kind: "lambda", detail: null, insert: b.name });
        else {
          const slot = b.node instanceof Object && "kind" in b.node && (b.node as A.Try).kind === "Try" ? this.res.catchSlots.get(b.node as A.Try) : this.res.slots.get(b.node);
          // f17 13.7 item 2: a function parameter is of kind lambda, its detail the printed function type.
          const fp = slot === undefined ? undefined : this.res.fnSlots.get(slot);
          if (fp !== undefined) {
            out.push({ label: b.name, kind: "lambda", detail: printFnType(fp, this.res), insert: b.name });
            continue;
          }
          const t = slot === undefined ? undefined : this.chk.slotTypes[slot];
          out.push({ label: b.name, kind: "name", detail: t === undefined ? null : printType(t), insert: b.name });
        }
      }
    }
    return [...out, ...envNames(this.env, seen)];
  }

  private members(m: A.Member): Candidate[] {
    const e = this.env.environment;
    const recv = m.x;
    // A namespace (f12 3.7 item 4).
    if (recv.kind === "Ident" && !recv.parens && this.res.refs.get(recv) === undefined) {
      const ns = recv.name;
      if (e.groups.has(ns)) {
        const out: Candidate[] = [];
        for (const [k, f] of GROUP_FUNCTIONS) if (f.group === ns && !e.removed.has(k)) out.push({ label: k.slice(ns.length + 1), kind: "function", detail: f.record.signature, insert: k.slice(ns.length + 1) });
        for (const [k, c] of GROUP_CONSTANTS) if (c.group === ns) out.push({ label: k.slice(ns.length + 1), kind: "constant", detail: c.type, insert: k.slice(ns.length + 1) });
        return out.sort((a, b) => compareText(a.label, b.label));
      }
      if (e.namespaces.has(ns)) {
        const out: Candidate[] = [];
        for (const [k, f] of e.functions) if (f.namespace === ns) out.push({ label: k.slice(ns.length + 1), kind: "function", detail: printSignature(f.sig, e.opaque, e.named), insert: k.slice(ns.length + 1) });
        return out.sort((a, b) => compareText(a.label, b.label));
      }
    }
    const t0 = this.typeOf(recv);
    if (t0 === undefined) return [];
    const t = alts(nonnull(t0));
    if (t === null || isNever(t)) return [];
    const a = t;
    const out: Candidate[] = [];
    const keys = keysOf(records(t));
    for (const k of [...keys.keys()].filter(isWord).sort(compareText)) out.push({ label: k, kind: "field", detail: printType(keys.get(k)!), insert: k });
    const methods = new Map<string, string>();
    const addKind = (kind: "string" | "list" | "map") => {
      for (const [k, m] of METHODS.get(kind)!) if (!e.removed.has(`${kind}.${k}`) && !methods.has(k)) methods.set(k, m.record.signature);
    };
    if (a.str !== undefined) addKind("string");
    if (a.list !== undefined) addKind("list");
    if (a.map !== undefined) addKind("map");
    for (const o of [...(a.opaque ?? [])].sort(compareText)) {
      for (const md of e.methods.values()) if (md.type === o && !methods.has(md.name)) methods.set(md.name, printSignature({ ...md.sig }, e.opaque, e.named));
    }
    for (const k of [...methods.keys()].sort(compareText)) out.push({ label: k, kind: "method", detail: methods.get(k)!, insert: k });
    return out;
  }

  private types(hole: Token, inLine: boolean): Candidate[] {
    const e = this.env.environment;
    // f16 7.2 item 1: with the names of the type lines that end before the hole. In a type
    // line's type, every line of the repaired program, its own included (f17 12.8 item 3).
    const lines = this.prog.types.filter((l) => (inLine ? l.span.start <= hole.start : l.span.end <= hole.start)).map((l) => l.name);
    const words = ["any", "bool", "float", "int", "map", "never", "null", "string", ...e.named.keys(), ...e.opaque, ...lines];
    return [...new Set(words)].sort(compareText).map((w) => ({ label: w, kind: "type", detail: null, insert: w }));
  }

  private keys(map: A.MapLit, exp: Type): Candidate[] {
    const written = new Set<string>();
    for (const en of map.entries) {
      if (en.kind !== "Entry") continue;
      if (en.key.kind === "word") written.add(en.key.text);
      else if (en.key.kind === "string") written.add(en.key.value);
    }
    const keys = keysOf(records(exp));
    return [...keys.keys()]
      .filter((k) => !written.has(k))
      .sort(compareText)
      .map((k) => ({ label: k, kind: "key", detail: printType(keys.get(k)!), insert: isWord(k) ? k : JSON.stringify(k) }));
  }

  private strings(lit: A.StringLit, parents: A.Node[], _isString: boolean): Candidate[] {
    const parent = parents[parents.length - 1];
    let t: Type | null = null;
    const exclude = new Set<string>();
    if (parent !== undefined && parent.kind === "Arm" && parent.literals.includes(lit)) {
      const m = parents[parents.length - 2] as A.Match;
      t = this.typeOf(m.subject) ?? null;
      // f12 3.11 item 1, with F17 (f17 7.2 item 3): a string that another arm with no guard lists is left out,
      // and one that an earlier literal of the hole's own arm lists, guard or not (R231).
      for (const arm of m.arms) if (arm !== parent && arm.guard === null) for (const l of arm.literals) if (l.kind === "String") exclude.add(l.value);
      for (const l of parent.literals) {
        if (l === lit) break;
        if (l.kind === "String") exclude.add(l.value);
      }
    } else if (parent !== undefined && parent.kind === "Binary" && (parent.op === "==" || parent.op === "!=")) {
      const other = parent.x === lit ? parent.y : parent.x;
      t = this.typeOf(other) ?? null;
    } else t = this.expectedOf(lit);
    if (t === null || isAny(t)) return [];
    const s = (t as Alts).str;
    if (s === undefined || s === "string") return [];
    const quote = this.holeQuote ?? '"';
    return [...s]
      .filter((v) => !exclude.has(v))
      .sort(compareText)
      .map((v) => ({ label: v, kind: "literal", detail: null, insert: escapeIn(v, quote) }));
  }

  holeQuote: string | undefined;

  /** The expected type of a node (f12 3.9), from the outside in. */
  private expectedOf(target: A.Node): Type | null {
    const ret = this.returns !== undefined && this.returns !== "condition" ? safeType(this.env, this.returns) : null;
    // The search keeps its own stack: an `else` branch, a receiver and a left
    // operand are as deep in the tree as the program is long (01 13.1).
    const nodes: A.Node[] = [this.prog];
    const exps: (Type | null)[] = [ret];
    while (nodes.length > 0) {
      const n = nodes.pop()!;
      const exp = exps.pop()!;
      if (n === target) return exp;
      // The children, each with its expected type, in order.
      const cs: A.Node[] = [];
      const es: (Type | null)[] = [];
      const visit = (c: A.Node, e: Type | null): void => {
        cs.push(c);
        es.push(e);
      };
      switch (n.kind) {
        case "Program":
        case "Block":
          for (const c of n.consts) visit(c, null);
          visit(n.result, exp);
          break;
        case "Cond":
          visit(n.test, null);
          visit(n.then, exp);
          visit(n.else, exp);
          break;
        case "Match":
          visit(n.subject, null);
          for (const arm of n.arms) visit(arm, exp);
          break;
        case "Arm":
          for (const l of n.literals) visit(l, null);
          // f17 7.2 item 2: a guard has no expected type.
          if (n.guard !== null) visit(n.guard, null);
          visit(n.result, exp);
          break;
        case "Try":
          visit(n.body, exp);
          visit(n.handler, exp);
          break;
        case "List": {
          const els = exp === null ? [] : listElems(exp);
          const e2 = els.length > 0 ? joinAll(els) : null;
          for (const it of n.items) visit(it, it.kind === "Spread" ? null : e2);
          break;
        }
        case "Map": {
          const recs = exp === null ? [] : records(exp);
          for (const en of n.entries) {
            if (en.kind === "Spread" || recs.length === 0) {
              visit(en, null);
              continue;
            }
            const key = en.key.kind === "word" ? en.key.text : en.key.kind === "string" ? en.key.value : null;
            let ft: Type | null = null;
            if (key !== null) {
              const ts: Type[] = [];
              for (const r of recs) {
                const f = r.fields.get(key);
                if (f !== undefined) ts.push(f.type);
                else if (!isNever(r.rest)) ts.push(r.rest);
              }
              ft = ts.length > 0 ? joinAll(ts) : null;
            }
            if (en.key.kind === "expr") visit(en.key.expr, null);
            visit(en.value, ft);
          }
          break;
        }
        case "Call": {
          const f = this.env.environment.functions.get(n.namespace === null ? n.name : `${n.namespace}.${n.name}`);
          n.args.forEach((a, i) => {
            let pt: Type | null = null;
            if (f !== undefined && f.sig.params[i] !== undefined) {
              // The parameter's type, when it holds no type variable: a name no environment declares.
              try {
                pt = fromNode(f.sig.params[i].type as import("../types/typetext.ts").TypeNode, { opaque: this.env.environment.opaque, named: this.env.environment.named });
              } catch {
                pt = null;
              }
            }
            visit(a, pt);
          });
          break;
        }
        default:
          for (const c of children(n)) visit(c, null);
      }
      for (let i = cs.length - 1; i >= 0; i--) {
        nodes.push(cs[i]);
        exps.push(es[i]);
      }
    }
    return null;

  }
}

function listElems(t: Type): Type[] {
  const a = alts(t);
  return a !== null && a.list !== undefined ? [a.list] : [];
}

function safeType(env: Env, text: string): Type | null {
  try {
    return env.typeOfText(text);
  } catch {
    return null;
  }
}

/** Package F16: the annotations a node holds itself: a const line's, a parameter's, a lambda's result (f16 2.3). */
function annotationsOf(n: A.Node): A.TypeAnn[] {
  switch (n.kind) {
    case "Const":
      return n.type !== null ? [n.type] : [];
    case "Param":
      if (n.type !== null) return [n.type];
      if (n.fn !== null) return [...n.fn.params.map((p) => p.type), ...(n.fn.result !== null ? [n.fn.result] : [])];
      return [];
    case "Lambda":
      return n.result !== null ? [n.result] : [];
    default:
      return [];
  }
}

function typeHasHole(n: unknown): boolean {
  if (n === null || typeof n !== "object") return false;
  const o = n as Record<string, unknown>;
  if (o.t === "name" && o.name === HOLE) return true;
  return Object.values(o).some((v) => (Array.isArray(v) ? v.some(typeHasHole) : typeHasHole(v)));
}

/** A string as the content of a one-line string in a quote (f12 3.11 item 3). */
function escapeIn(v: string, quote: string): string {
  let out = "";
  for (const ch of v) {
    const c = ch.codePointAt(0)!;
    if (ch === "\\") out += "\\\\";
    else if (ch === quote) out += "\\" + quote;
    else if (c === 8) out += "\\b";
    else if (c === 12) out += "\\f";
    else if (c === 10) out += "\\n";
    else if (c === 13) out += "\\r";
    else if (c === 9) out += "\\t";
    else if (c < 0x20) out += "\\u" + c.toString(16).padStart(4, "0");
    else out += ch;
  }
  return out;
}

// ---- hover (f12 3.13) ----------------------------------------------------------

export function hover(env: Env, text: string, pos: Position, returns?: string): HoverResult | null {
  const src = new Source(text);
  const at = offsetOf(src, pos);
  if (at < 0 || at >= src.length) return null;
  const isW = (i: number) => i >= 0 && i < src.length && WORD.test(String.fromCodePoint(src.cps[i]));
  if (!isW(at)) return null;
  let ws = at;
  while (isW(ws - 1)) ws--;
  let we = at;
  while (isW(we)) we++;
  // The text up to the end of the word must lex with the word as its last token.
  let toks: Token[];
  try {
    toks = lexCut(new Source(src.slice(0, we), { keepBom: true }));
  } catch {
    return null;
  }
  const list = innermost(toks);
  const real = list.filter((t) => t.kind !== "eof" && !(t.kind === "interpEnd" && t.cut));
  const last = real[real.length - 1];
  if (last === undefined || last.kind !== "word" || last.start !== ws || last.end !== we) return null;
  let cut = we;
  if (we < src.length && src.cps[we] === 0x28) cut = we + 1;
  let prog: A.Program | null = null;
  let progSrc: Source = src;
  const whole = parse(src);
  if (whole.ok) prog = whole.program;
  else {
    progSrc = new Source(src.slice(0, cut), { keepBom: true });
    let ts: Token[];
    try {
      ts = lexCut(progSrc);
    } catch {
      return null;
    }
    prog = repair(ts, progSrc);
  }
  if (prog === null) return null;
  const { resolution, check } = env.assistCheck(prog, progSrc, returns);
  return hoverRecord(env, prog, resolution, check, { start: ws, end: we }, src);
}

/** Package F16: a hover on a type name, or on the name of a type line (f16 7.3). */
function typeHover(env: Env, prog: A.Program, res: Resolution, word: Span, src: Source): HoverResult | null {
  const e = env.environment;
  const same = (s: Span) => s.start === word.start && s.end === word.end;
  // f17 12.8 item 2: a recursive type is printed by its unfolding.
  const rec = (type: Type, decl: Span | null, name: string): HoverResult => ({
    kind: "type",
    span: src.lineSpan(word),
    type: soleRef(type)?.name === name ? printUnfolded(type) : printType(type),
    signature: null,
    doc: null,
    declaration: decl === null ? null : src.lineSpan(decl),
  });
  const fromEnv = (name: string): HoverResult | null => {
    const n = e.named.get(name);
    if (n !== undefined) return rec(n, null, name);
    if (e.opaque.has(name)) return rec(opaque(name), null, name);
    return null;
  };
  // f16 7.3 items 1 and 2: the name of a type line stands for what the line declares.
  for (const l of prog.types) {
    if (!same(l.nameSpan)) continue;
    const t = res.typeLines.get(l);
    return t !== undefined ? rec(t, l.nameSpan, l.name) : fromEnv(l.name);
  }
  // f17 12.8 item 2: a name in the type of type line i is resolved by 12.2 item 1: the last
  // line of the name at or before i, else the first after it.
  for (let i = 0; i < prog.types.length; i++) {
    for (const [node, span] of prog.types[i].type.spans) {
      if (node.t !== "name" || !same(span)) continue;
      const name = node.name;
      if (e.named.has(name) || e.opaque.has(name)) return fromEnv(name);
      let line: A.TypeLine | null = null;
      for (let j = 0; j <= i; j++) if (prog.types[j].name === name) line = prog.types[j];
      if (line === null) for (let j = i + 1; j < prog.types.length && line === null; j++) if (prog.types[j].name === name) line = prog.types[j];
      const t = line === null ? undefined : res.typeLines.get(line);
      return line !== null && t !== undefined ? rec(t, line.nameSpan, name) : null;
    }
  }
  // A type name in an annotation, or in `as` and `is`.
  const maps: ReadonlyMap<TypeNode, Span>[] = [];
  walk(prog, (n) => {
    if (n.kind === "As") maps.push(n.typeSpans);
    for (const a of annotationsOf(n)) maps.push(a.spans);
  });
  for (const m of maps) {
    for (const [node, span] of m) {
      if (node.t !== "name" || !same(span)) continue;
      const name = node.name;
      // The last type line before the word with that name, when that line declares it; else the environment's type.
      let last: A.TypeLine | null = null;
      for (const l of prog.types) if (l.name === name && l.span.end <= word.start) last = l;
      const t = last === null ? undefined : res.typeLines.get(last);
      if (last !== null && t !== undefined) return rec(t, last.nameSpan, name);
      return fromEnv(name);
    }
  }
  return null;
}

function hoverRecord(env: Env, prog: A.Program, res: Resolution, chk: CheckResult, word: Span, src: Source): HoverResult | null {
  const th = typeHover(env, prog, res, word, src);
  if (th !== null) return th;
  const e = env.environment;
  const ls = (s: Span) => src.lineSpan(s);
  const same = (s: Span) => s.start === word.start && s.end === word.end;
  const tp = (x: object) => {
    const t = chk.types.get(x);
    return t === undefined ? null : printType(t);
  };
  const rec = (kind: string, type: string | null, signature: string | null = null, doc: string | null = null, declaration: LineSpan | null = null): HoverResult => ({ kind, span: ls(word), type, signature, doc, declaration });
  // The binding of each slot, for declarations.
  const slotSpan = new Map<number, Span>();
  for (const [node, slot] of res.slots) {
    const n = node as { kind?: string; span: Span; nameSpan?: Span };
    slotSpan.set(slot, n.nameSpan ?? n.span);
  }
  for (const [t, slot] of res.catchSlots) if (t.nameSpan !== null) slotSpan.set(slot, t.nameSpan);
  let out: HoverResult | null = null;
  const lambdaDecl = (l: A.Lambda): Span | null => {
    let s: Span | null = null;
    walk(prog, (n) => {
      if (n.kind === "Const" && n.value === l && n.pattern.kind === "NamePat") s = n.pattern.span;
    });
    return s;
  };
  walk(prog, (n) => {
    if (out !== null) return false;
    switch (n.kind) {
      case "Ident": {
        if (!same(n.span)) break;
        const r = res.refs.get(n);
        if (r === undefined) {
          if (e.groups.has(n.name) || e.namespaces.has(n.name)) out = rec("namespace", null);
          break;
        }
        if (r.k === "slot") out = rec("name", tp(n), null, null, slotSpan.has(r.slot) ? ls(slotSpan.get(r.slot)!) : null);
        else if (r.k === "var") out = rec("variable", tp(n));
        // f17 13.7 item 3: the name of a function parameter given on, or where a value is needed.
        else if (r.k === "fn") out = rec("lambda", null, printFnType(r.param, res), null, ls(r.param.pattern.span));
        else if (r.k === "lambda") {
          const d = lambdaDecl(res.lambdas[r.id]);
          out = rec("lambda", null, null, null, d === null ? null : ls(d));
        }
        break;
      }
      case "Call": {
        const full = n.namespace === null ? n.name : `${n.namespace}.${n.name}`;
        if (n.namespace !== null && n.span.start === word.start && n.span.start !== n.nameSpan.start) {
          if (word.end - word.start === n.namespace.length) {
            out = rec("namespace", null);
            break;
          }
        }
        if (!same(n.nameSpan)) break;
        const t = res.targets.get(n);
        if (t !== undefined && t.k === "lambda") {
          const d = lambdaDecl(res.lambdas[t.id]);
          out = { ...rec("lambda", tp(n), null, null, d === null ? null : ls(d)) };
          break;
        }
        // f17 13.7 item 3: a call of a function parameter gives the call's type.
        if (t !== undefined && t.k === "fn") {
          out = rec("lambda", tp(n), printFnType(t.param, res), null, ls(t.param.pattern.span));
          break;
        }
        const host = e.functions.get(full);
        if (host !== undefined) {
          out = rec("function", tp(n), printSignature(host.sig, e.opaque, e.named));
          break;
        }
        const lib = FUNCTIONS.get(full) ?? GROUP_FUNCTIONS.get(full);
        if (lib !== undefined) out = rec("function", tp(n), lib.record.signature, recordDoc(lib.record as unknown as { [k: string]: unknown }) ?? GROUP_DOCS.get(full) ?? null);
        break;
      }
      case "MethodCall": {
        // A call of a namespace is a Call in the targets.
        if (n.x.kind === "Ident" && same(n.x.span) && res.refs.get(n.x) === undefined && (e.groups.has(n.x.name) || e.namespaces.has(n.x.name))) {
          out = rec("namespace", null);
          break;
        }
        if (!same(n.nameSpan)) break;
        const target = res.targets.get(n);
        if (target !== undefined) {
          const full = `${(n.x as A.Ident).name}.${n.name}`;
          const host = e.functions.get(full);
          if (host !== undefined) out = rec("function", tp(n), printSignature(host.sig, e.opaque, e.named));
          else {
            const lib = GROUP_FUNCTIONS.get(full);
            out = rec("function", tp(n), lib?.record.signature ?? null, lib === undefined ? null : (GROUP_DOCS.get(full) ?? null));
          }
          break;
        }
        const rt = chk.types.get(n.x);
        let signature: string | null = null;
        let doc: string | null = null;
        const a = rt === undefined ? null : alts(nonnull(rt));
        if (a !== null) {
          const kinds: ("string" | "list" | "map")[] = [];
          if (a.str !== undefined) kinds.push("string");
          if (a.list !== undefined) kinds.push("list");
          if (a.map !== undefined) kinds.push("map");
          for (const k of kinds) {
            const m = METHODS.get(k)!.get(n.name);
            if (m !== undefined && !e.removed.has(`${k}.${n.name}`)) {
              signature = m.record.signature;
              doc = recordDoc(m.record as unknown as { [k: string]: unknown });
              break;
            }
          }
          if (signature === null) {
            for (const o of [...(a.opaque ?? [])].sort(compareText)) {
              const md = e.methods.get(`${o}.${n.name}`);
              if (md !== undefined) {
                signature = printSignature(md.sig, e.opaque, e.named);
                break;
              }
            }
          }
        }
        out = rec("method", tp(n), signature, doc);
        break;
      }
      case "Member": {
        const c = res.constants.get(n);
        if (n.x.kind === "Ident" && same(n.x.span) && res.refs.get(n.x) === undefined && (e.groups.has(n.x.name) || e.namespaces.has(n.x.name))) {
          out = rec("namespace", null);
          break;
        }
        if (!same(n.nameSpan)) break;
        if (c !== undefined) {
          const full = `${(n.x as A.Ident).name}.${n.name}`;
          const k = GROUP_CONSTANTS.get(full);
          out = rec("constant", k?.type ?? tp(n), null, GROUP_DOCS.get(full) ?? null);
          break;
        }
        out = rec("field", tp(n));
        break;
      }
      case "NamePat": {
        if (!same(n.span) || n.name === null) break;
        const slot = res.slots.get(n);
        // f17 13.7 item 3: a function parameter where it is declared.
        const fp = slot === undefined ? undefined : res.fnSlots.get(slot);
        if (fp !== undefined) {
          out = rec("lambda", null, printFnType(fp, res), null, ls(n.span));
          break;
        }
        const t = slot === undefined ? undefined : chk.slotTypes[slot];
        out = rec("name", t === undefined ? null : printType(t), null, null, ls(n.span));
        break;
      }
      case "MapPat":
      case "ListPat": {
        const binds: { node: object; span: Span }[] =
          n.kind === "MapPat" ? n.fields.map((f) => ({ node: f as object, span: f.nameSpan })) : n.elems.map((el) => ({ node: el.pat as object, span: el.pat.span }));
        if (n.rest !== null) binds.push({ node: n.rest, span: n.rest.span });
        for (const b of binds) {
          if (!same(b.span)) continue;
          const slot = res.slots.get(b.node);
          const t = slot === undefined ? undefined : chk.slotTypes[slot];
          out = rec("name", t === undefined ? null : printType(t), null, null, ls(b.span));
        }
        break;
      }
      case "Try": {
        if (n.nameSpan !== null && same(n.nameSpan)) {
          const slot = res.catchSlots.get(n);
          const t = slot === undefined ? undefined : chk.slotTypes[slot];
          out = rec("name", t === undefined ? null : printType(t), null, null, ls(n.nameSpan));
        }
        break;
      }
    }
    return undefined;
  });
  return out;
}
