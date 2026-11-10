// Package F2: analysis (f02-templates-analysis.md 3). It reads a parsed tree
// with no environment and reports the free names a program reads, the paths
// it reads through them, the calls it makes and whether it is a literal.
// Long flat forms are walked by loops (3.1 item 5).

import type * as A from "./ast.ts";
import type { Span } from "./source.ts";
import type { Segment } from "./template.ts";
import { RMap, compareText } from "../value/value.ts";

/** A step of a path: a key, or with package F7 a star (f07 9.1). */
export type PathStep = { key: string } | { star: true };

export interface Reference {
  name: string;
  path: PathStep[];
  span: Span;
}

export interface CallReport {
  name: string;
  namespace: string | null;
  span: Span;
  /** The argument nodes, in order. */
  args: A.Arg[];
  lambda: boolean;
}

export interface Report {
  names: string[];
  refs: Reference[];
  calls: CallReport[];
  /** null when the program is not a literal (3.7). */
  literal: { value: unknown } | null;
  segments: Segment[] | null;
}

/** Analyze a parsed program (3.1). Never fails. */
export function analyze(program: A.Program, segments: Segment[] | null = null): Report {
  const w = new Walker();
  w.program(program);
  const refs = w.refs.sort((a, b) => a.span.start - b.span.start);
  const calls = w.calls.sort((a, b) => a.span.start - b.span.start);
  const names = [...new Set(refs.map((r) => r.name))].sort(compareText);
  return { names, refs, calls, literal: program.consts.length === 0 ? literalOf(program.result) : null, segments };
}

class Walker {
  readonly refs: Reference[] = [];
  readonly calls: CallReport[] = [];
  private readonly scopes: Set<string>[] = [];
  private lambdas = 0;

  private bound(name: string): boolean {
    for (let i = this.scopes.length - 1; i >= 0; i--) if (this.scopes[i].has(name)) return true;
    return false;
  }

  private bind(name: string | null): void {
    if (name !== null && name !== "_") this.scopes[this.scopes.length - 1].add(name);
  }

  program(p: A.Program): void {
    this.scopes.push(new Set());
    for (const c of p.consts) this.constLine(c);
    this.expr(p.result);
    this.scopes.pop();
  }

  private constLine(c: A.Const): void {
    if (c.value.kind === "Lambda") {
      this.lambda(c.value);
      this.bind((c.pattern as A.NamePat).name);
      return;
    }
    this.expr(c.value);
    this.pattern(c.pattern);
  }

  private pattern(p: A.Pattern): void {
    if (p.kind === "NamePat") this.bind(p.name);
    else if (p.kind === "MapPat") {
      for (const f of p.fields) {
        if (f.default !== null) this.expr(f.default);
        this.bind(f.name);
      }
      if (p.rest !== null) this.bind(p.rest.name);
    } else {
      for (const e of p.elems) {
        if (e.default !== null) this.expr(e.default);
        this.bind(e.pat.name);
      }
      if (p.rest !== null) this.bind(p.rest.name);
    }
  }

  private lambda(l: A.Lambda): void {
    this.lambdas++;
    this.scopes.push(new Set());
    for (const p of l.params) {
      if (p.default !== null) this.expr(p.default);
      this.pattern(p.pattern);
    }
    this.expr(l.body);
    this.scopes.pop();
    this.lambdas--;
  }

  private scoped(consts: A.Const[], result: A.Expr, first?: string | null): void {
    this.scopes.push(new Set());
    if (first !== undefined) this.bind(first);
    for (const c of consts) this.constLine(c);
    this.expr(result);
    this.scopes.pop();
  }

  private args(args: A.Arg[]): void {
    for (const a of args) {
      if (a.kind === "Lambda") this.lambda(a);
      else this.expr(a);
    }
  }

  private expr(e: A.Expr): void {
    for (;;) {
      switch (e.kind) {
        case "Number":
        case "String":
        case "Bool":
        case "Null":
        case "Bad":
          return;
        case "Ident":
          if (!this.bound(e.name)) this.refs.push({ name: e.name, path: [], span: e.span });
          return;
        case "Template":
          for (const p of e.parts) if (typeof p !== "string") this.expr(p);
          return;
        case "List":
          for (const it of e.items) this.expr(it.kind === "Spread" ? it.value : it);
          return;
        case "Map":
          for (const en of e.entries) {
            if (en.kind === "Spread") this.expr(en.value);
            else {
              if (en.key.kind === "expr") this.expr(en.key.expr);
              this.expr(en.value);
            }
          }
          return;
        case "Unary":
          e = e.x;
          continue;
        case "As":
          // Package F4: the subject; the program type names no value.
          e = e.x;
          continue;
        case "Binary": {
          const spine: A.Binary[] = [];
          let x: A.Expr = e;
          while (x.kind === "Binary") {
            spine.push(x);
            x = x.x;
          }
          this.expr(x);
          for (let i = spine.length - 1; i >= 0; i--) this.expr(spine[i].y);
          return;
        }
        case "Cond": {
          let c: A.Expr = e;
          while (c.kind === "Cond") {
            this.expr(c.test);
            this.expr(c.then);
            c = c.else;
          }
          e = c;
          continue;
        }
        case "Block":
          this.scoped(e.consts, e.result);
          return;
        case "Try": {
          const b = e.body.kind === "Block" ? e.body : null;
          if (b !== null) this.scoped(b.consts, b.result);
          else this.expr(e.body);
          const h = e.handler;
          if (h.kind === "Block") this.scoped(h.consts, h.result, e.name);
          else this.scoped([], h, e.name);
          return;
        }
        case "Match":
          this.expr(e.subject);
          for (const arm of e.arms) this.expr(arm.result);
          return;
        case "Call":
        case "MethodCall":
        case "Member":
        case "Index":
        case "OptChain":
        case "Each":
          this.chain(e);
          return;
      }
    }
  }

  /** A chain: its primary, then each step. A parenthesis ends a chain. */
  private chain(e: A.Expr): void {
    type Step = A.Member | A.Index | A.MethodCall | { kind: "Star"; span: Span };
    const steps: Step[] = [];
    let x: A.Expr = e.kind === "OptChain" ? e.x : e;
    let cut = false;
    // A projection's steps in order: its star, then its reads, a later star with its own (f07 9.1).
    const projection = (p: A.Each): Step[] => {
      const out: Step[] = [{ kind: "Star", span: p.starSpan }];
      for (const r of p.reads) {
        if (r.kind === "Each") out.push(...projection(r));
        else out.push(r);
      }
      return out;
    };
    for (;;) {
      if (x.kind === "Member" || x.kind === "Index" || x.kind === "MethodCall") {
        steps.push(x);
        x = x.x;
      } else if (x.kind === "Each" && x.x !== null) {
        steps.push(...projection(x).reverse());
        x = x.x;
      } else break;
      if (x.parens) {
        cut = true; // the receiver is written in parentheses: a chain of its own
        break;
      }
    }
    steps.reverse();
    if (x.kind === "Call" && !cut) {
      if (!this.bound(x.name)) this.calls.push({ name: x.name, namespace: null, span: x.span, args: x.args, lambda: this.lambdas > 0 });
      this.args(x.args);
    } else if (x.kind === "Ident" && !this.bound(x.name)) {
      // A read, with its path of static steps (3.5). A parenthesis stops the path.
      const path: PathStep[] = [];
      let end = x.span.end;
      if (!cut) {
        for (const s of steps) {
          if (s.kind === "Member") path.push({ key: s.name });
          else if (s.kind === "Index" && s.index.kind === "String") path.push({ key: s.index.value });
          else if (s.kind === "Star") path.push({ star: true });
          else break;
          end = s.span.end;
        }
      }
      this.refs.push({ name: x.name, path, span: { start: x.span.start, end } });
      const first = steps[0];
      if (first !== undefined && first.kind === "MethodCall" && !first.optional) {
        this.calls.push({ name: `${x.name}.${first.name}`, namespace: x.name, span: first.span, args: first.args, lambda: this.lambdas > 0 });
      }
    } else {
      this.expr(x);
    }
    for (const s of steps) {
      if (s.kind === "Index") this.expr(s.index);
      else if (s.kind === "MethodCall") this.args(s.args);
    }
  }
}

/** The literal of a program's result (3.7), or null. */
export function literalOf(e: A.Expr): { value: unknown } | null {
  const v = lit(e);
  return v === NOT ? null : { value: v };
}

const NOT: unique symbol = Symbol("not a literal");

function lit(e: A.Expr): unknown {
  switch (e.kind) {
    case "Null":
      return null;
    case "Bool":
    case "Number":
    case "String":
      return e.value;
    case "Unary":
      return e.op === "-" && e.x.kind === "Number" ? -e.x.value + 0 : NOT;
    case "List": {
      const out: unknown[] = [];
      for (const it of e.items) {
        if (it.kind === "Spread") return NOT;
        const v = lit(it);
        if (v === NOT) return NOT;
        out.push(v);
      }
      return out;
    }
    case "Map": {
      const m = new RMap();
      for (const en of e.entries) {
        if (en.kind === "Spread" || en.shorthand || en.key.kind === "expr") return NOT;
        const v = lit(en.value);
        if (v === NOT) return NOT;
        m.set(en.key.kind === "word" ? en.key.text : en.key.value, v);
      }
      return m;
    }
    default:
      return NOT;
  }
}
