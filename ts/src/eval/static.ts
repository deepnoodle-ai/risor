// The static rules (01 14.4; 05 section 22), and name resolution.
//
// One walk over the tree reports the static diagnostics and records, for the
// compiler, what each name and each call resolves to. Every node is visited
// once, and nothing is skipped after a diagnostic (05 22.4). Long flat forms
// (a chain of binary operators, a chain of steps, an `else` chain) are walked
// by loops, so a program of 100,000 terms needs no deep stack.

import type * as A from "../syntax/ast.ts";
import type { Diagnostic } from "../syntax/diagnostic.ts";
import type { Span } from "../syntax/source.ts";

/** The counts of a signature (05 22.3). `fns[i]` is the parameter count of an `Fn` parameter i, or null. */
export interface Counts {
  min: number;
  max: number; // Infinity with a rest parameter
  fns: (number | null)[];
  /** Whether the last parameter is a rest parameter. */
  rest: boolean;
}

export function fnAt(c: Counts, i: number): number | null {
  if (c.fns.length === 0) return null;
  if (i >= c.fns.length) return c.rest ? c.fns[c.fns.length - 1] : null;
  return c.fns[i];
}

export interface StaticEnv {
  variables: ReadonlySet<string>;
  /** Host functions by full name, `geo.distance` or `rate`. */
  hostFunctions: ReadonlyMap<string, Counts>;
  namespaces: ReadonlySet<string>;
  /** Library functions (entries with no receiver). */
  library: ReadonlyMap<string, Counts>;
}

export type Ref =
  | { k: "slot"; slot: number }
  | { k: "var"; name: string }
  | { k: "lambda"; id: number };

export type Target =
  | { k: "lambda"; id: number }
  | { k: "library"; name: string }
  | { k: "host"; name: string }
  | { k: "unknown" };

/** What the compiler needs from the walk. */
export interface Resolution {
  /** Each Ident that reads a name, and each Ident that is a lambda argument. */
  refs: Map<A.Ident, Ref>;
  /** Each Call, and each MethodCall that is a call of a namespace. */
  targets: Map<A.Call | A.MethodCall, Target>;
  /** The slot of each name a pattern binds: a NamePat, a field, an element, a rest. */
  slots: Map<object, number>;
  /** The slot of the name of each `catch (err)`. */
  catchSlots: Map<A.Try, number>;
  /** Each const-bound lambda, by id. */
  lambdas: A.Lambda[];
  slotCount: number;
}

type Binding = { k: "value"; slot: number } | { k: "var"; name: string } | { k: "lambda"; id: number; counts: Counts };

export function lambdaCounts(l: A.Lambda): Counts {
  // Ruling AC10: the required count is the position of the last parameter with no default.
  let min = 0;
  l.params.forEach((p, i) => {
    if (p.default === null) min = i + 1;
  });
  return {
    min,
    max: l.params.length,
    fns: l.params.map(() => null),
    rest: false,
  };
}

export interface StaticResult {
  diagnostics: Diagnostic[];
  resolution: Resolution;
}

export function staticRules(program: A.Program, env: StaticEnv): StaticResult {
  const w = new Walker(env);
  w.program(program);
  // 05 22.10: a set, ordered by start, end, then code.
  const seen = new Set<string>();
  const out: Diagnostic[] = [];
  for (const d of w.diags) {
    const key = `${d.code}:${d.span!.start}:${d.span!.end}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(d);
  }
  out.sort((a, b) => a.span!.start - b.span!.start || a.span!.end - b.span!.end || (a.code < b.code ? -1 : a.code > b.code ? 1 : 0));
  return { diagnostics: out, resolution: w.res };
}

class Walker {
  readonly diags: Diagnostic[] = [];
  readonly res: Resolution = { refs: new Map(), targets: new Map(), slots: new Map(), catchSlots: new Map(), lambdas: [], slotCount: 0 };
  private readonly scopes: Map<string, Binding>[] = [];
  private readonly env: StaticEnv;

  constructor(env: StaticEnv) {
    this.env = env;
    const outer = new Map<string, Binding>();
    for (const v of env.variables) outer.set(v, { k: "var", name: v });
    this.scopes.push(outer);
  }

  private diag(code: string, span: Span): void {
    this.diags.push({ phase: "check", code, span: { start: span.start, end: span.end }, message: messageFor(code) });
  }

  private lookup(name: string): Binding | undefined {
    for (let i = this.scopes.length - 1; i >= 0; i--) {
      const b = this.scopes[i].get(name);
      if (b !== undefined) return b;
    }
    return undefined;
  }

  private push(): void {
    this.scopes.push(new Map());
  }

  private pop(): void {
    this.scopes.pop();
  }

  /** Bind a name in the innermost scope (05 22.7). Returns the slot, or null. */
  private bind(name: string | null, span: Span, b?: Binding): number | null {
    if (name === null || name === "_") return null;
    if (this.env.namespaces.has(name)) {
      this.diag("shadows-namespace", span);
      return null;
    }
    const scope = this.scopes[this.scopes.length - 1];
    if (scope.has(name)) this.diag("duplicate-name", span);
    if (b === undefined) {
      const slot = this.res.slotCount++;
      scope.set(name, { k: "value", slot });
      return slot;
    }
    scope.set(name, b);
    return null;
  }

  program(p: A.Program): void {
    this.push();
    for (const c of p.consts) this.constLine(c);
    this.expr(p.result);
    this.pop();
  }

  private constLine(c: A.Const): void {
    if (c.value.kind === "Lambda") {
      this.lambda(c.value);
      const id = this.res.lambdas.length;
      this.res.lambdas.push(c.value);
      const p = c.pattern as A.NamePat;
      this.bind(p.name, p.span, { k: "lambda", id, counts: lambdaCounts(c.value) });
      this.res.slots.set(p, -1 - id);
      return;
    }
    this.expr(c.value);
    this.pattern(c.pattern);
  }

  private pattern(p: A.Pattern): void {
    switch (p.kind) {
      case "NamePat": {
        const s = this.bind(p.name, p.span);
        if (s !== null) this.res.slots.set(p, s);
        return;
      }
      case "MapPat":
        for (const f of p.fields) {
          if (f.default !== null) this.expr(f.default);
          const s = this.bind(f.name, f.nameSpan);
          if (s !== null) this.res.slots.set(f, s);
        }
        if (p.rest !== null) {
          const s = this.bind(p.rest.name, { start: p.rest.span.end - p.rest.name.length, end: p.rest.span.end });
          if (s !== null) this.res.slots.set(p.rest, s);
        }
        return;
      case "ListPat":
        for (const e of p.elems) {
          if (e.default !== null) this.expr(e.default);
          const s = this.bind(e.pat.name, e.pat.span);
          if (s !== null) this.res.slots.set(e.pat, s);
        }
        if (p.rest !== null) {
          const s = this.bind(p.rest.name, { start: p.rest.span.end - p.rest.name.length, end: p.rest.span.end });
          if (s !== null) this.res.slots.set(p.rest, s);
        }
        return;
    }
  }

  private lambda(l: A.Lambda): void {
    this.push();
    for (const p of l.params) {
      if (p.default !== null) this.expr(p.default);
      this.pattern(p.pattern);
    }
    this.expr(l.body);
    this.pop();
  }

  /** A block, a branch, a try body: a new scope. */
  private scoped(e: A.Expr): void {
    this.push();
    if (e.kind === "Block") {
      for (const c of e.consts) this.constLine(c);
      this.expr(e.result);
    } else {
      this.expr(e);
    }
    this.pop();
  }

  private ident(id: A.Ident, asLambdaArg: boolean): void {
    const b = this.lookup(id.name);
    if (b !== undefined) {
      if (b.k === "value") this.res.refs.set(id, { k: "slot", slot: b.slot });
      else if (b.k === "var") this.res.refs.set(id, { k: "var", name: b.name });
      else {
        this.res.refs.set(id, { k: "lambda", id: b.id });
        if (!asLambdaArg) this.diag("not-a-value", id.span);
      }
      return;
    }
    if (this.env.namespaces.has(id.name)) this.diag("not-a-value", id.span);
    else if (this.env.hostFunctions.has(id.name) || this.env.library.has(id.name)) this.diag("not-a-value", id.span);
    else this.diag("undefined-name", id.span);
  }

  private isLambdaArg(a: A.Arg): boolean {
    if (a.kind === "Lambda") return true;
    if (a.kind !== "Ident") return false;
    return this.lookup(a.name)?.k === "lambda";
  }

  private walkArgs(args: A.Arg[]): void {
    for (const a of args) {
      if (a.kind === "Lambda") this.lambda(a);
      else if (a.kind === "Ident" && this.isLambdaArg(a)) this.ident(a, true);
      else this.expr(a);
    }
  }

  private argParams(a: A.Arg): number {
    if (a.kind === "Lambda") return a.params.length;
    const b = this.lookup((a as A.Ident).name);
    return b !== undefined && b.k === "lambda" ? b.counts.max : 0;
  }

  /** 05 22.5. */
  private call(node: A.Call | A.MethodCall, namespace: string | null, name: string, nameSpan: Span, args: A.Arg[]): void {
    let target: Target = { k: "unknown" };
    let counts: Counts | null = null;
    let isLambda = false;
    if (namespace !== null) {
      const full = `${namespace}.${name}`;
      const c = this.env.hostFunctions.get(full);
      if (c !== undefined) {
        target = { k: "host", name: full };
        counts = c;
      } else this.diag("undefined-function", nameSpan);
    } else {
      const b = this.lookup(name);
      if (b !== undefined && b.k === "lambda") {
        target = { k: "lambda", id: b.id };
        counts = b.counts;
        isLambda = true;
      } else if (b !== undefined) {
        this.diag("call-of-parameter", nameSpan);
      } else if (this.env.hostFunctions.has(name)) {
        target = { k: "host", name };
        counts = this.env.hostFunctions.get(name)!;
      } else if (this.env.library.has(name)) {
        target = { k: "library", name };
        counts = this.env.library.get(name)!;
      } else {
        this.diag("undefined-function", nameSpan);
      }
    }
    this.res.targets.set(node, target);
    if (counts !== null) {
      if (args.length < counts.min || args.length > counts.max) {
        this.diag("arity", node.span);
      } else {
        args.forEach((a, i) => {
          if (!this.isLambdaArg(a)) return;
          if (isLambda) {
            this.diag("lambda-to-lambda", a.span);
            return;
          }
          const fn = fnAt(counts!, i);
          if (fn === null) this.diag("not-a-value", a.span);
          else if (this.argParams(a) > fn) this.diag("arity", a.span);
        });
      }
    }
    this.walkArgs(args);
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
          this.ident(e, false);
          return;
        case "Template":
          for (const p of e.parts) if (typeof p !== "string") this.expr(p);
          return;
        case "List":
          for (const it of e.items) this.expr(it.kind === "Spread" ? it.value : it);
          return;
        case "Map":
          this.map(e);
          return;
        case "Unary":
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
          // An `else` chain is walked by a loop.
          let c: A.Cond = e;
          for (;;) {
            this.expr(c.test);
            this.expr(c.then);
            const n: A.Expr = c.else;
            if (n.kind === "Cond" && n.form === c.form) {
              c = n;
              continue;
            }
            e = n;
            break;
          }
          continue;
        }
        case "Block":
          this.scoped(e);
          return;
        case "Try":
          this.scoped(e.body);
          this.push();
          if (e.name !== null) {
            const s = this.bind(e.name, e.nameSpan!);
            if (s !== null) this.res.catchSlots.set(e, s);
          }
          if (e.handler.kind === "Block") {
            for (const c of e.handler.consts) this.constLine(c);
            this.expr(e.handler.result);
          } else this.expr(e.handler);
          this.pop();
          return;
        case "Match":
          this.match(e);
          return;
        case "OptChain":
        case "Member":
        case "Index":
        case "MethodCall":
        case "Call":
          this.chain(e);
          return;
      }
    }
  }

  /** A chain, walked from its primary. */
  private chain(e: A.Expr): void {
    const steps: A.Expr[] = [];
    // Only the outermost node of a chain is an OptChain: an inner one is a
    // chain of its own, written in parentheses (01 7.4 rule 5).
    let x: A.Expr = e.kind === "OptChain" ? e.x : e;
    for (;;) {
      if (x.kind === "Member" || x.kind === "Index") {
        steps.push(x);
        x = x.x;
      } else if (x.kind === "MethodCall") {
        if (!x.optional && x.x.kind === "Ident" && this.env.namespaces.has(x.x.name)) {
          steps.push(x);
          x = x.x;
          break;
        }
        steps.push(x);
        x = x.x;
      } else break;
    }
    // x is the primary, unless the innermost step is a namespace call.
    const innermost = steps[steps.length - 1];
    const nsCall = innermost !== undefined && innermost.kind === "MethodCall" && !innermost.optional && innermost.x === x && x.kind === "Ident" && this.env.namespaces.has(x.name);
    if (!nsCall) {
      if (x.kind === "Call") this.call(x, null, x.name, x.nameSpan, x.args);
      else this.expr(x);
    }
    for (let i = steps.length - 1; i >= 0; i--) {
      const s = steps[i];
      if (s.kind === "Index") this.expr(s.index);
      else if (s.kind === "MethodCall") {
        if (i === steps.length - 1 && nsCall) {
          const ns = s.x as A.Ident;
          this.call(s, ns.name, s.name, { start: ns.span.start, end: s.nameSpan.end }, s.args);
        } else this.walkArgs(s.args);
      }
    }
  }

  private map(m: A.MapLit): void {
    const keys = new Set<string>();
    for (const en of m.entries) {
      if (en.kind === "Spread") {
        this.expr(en.value);
        continue;
      }
      if (en.key.kind === "expr") this.expr(en.key.expr);
      else {
        const k = en.key.kind === "word" ? en.key.text : en.key.value;
        if (keys.has(k)) this.diag("duplicate-key", en.key.span);
        keys.add(k);
      }
      if (en.shorthand) this.ident(en.value as A.Ident, false);
      else this.expr(en.value);
    }
  }

  private match(m: A.Match): void {
    this.expr(m.subject);
    const lits: A.Literal[] = [];
    let defaultSeen = false;
    for (const arm of m.arms) {
      if (defaultSeen) this.diag("unreachable-arm", arm.span);
      for (const l of arm.literals) {
        if (lits.some((p) => litEqual(p, l))) this.diag("duplicate-case", l.span);
        lits.push(l);
      }
      if (arm.isDefault) defaultSeen = true;
      this.expr(arm.result);
    }
  }
}

function litEqual(a: A.Literal, b: A.Literal): boolean {
  if (a.kind !== b.kind) return false;
  switch (a.kind) {
    case "Null":
      return true;
    case "Bool":
    case "Number":
    case "String":
      return a.value === (b as typeof a).value;
  }
}

function messageFor(code: string): string {
  switch (code) {
    case "undefined-name": return "this name is not bound or declared";
    case "undefined-function": return "no function of this name";
    case "not-a-value": return "a function is not a value";
    case "call-of-parameter": return "only a function can be called";
    case "lambda-to-lambda": return "a lambda cannot be passed to a lambda";
    case "duplicate-name": return "this name is already bound here";
    case "shadows-namespace": return "a binding cannot have the name of a namespace";
    case "duplicate-key": return "this key is written twice";
    case "arity": return "wrong count of arguments or parameters";
    case "duplicate-case": return "this literal is listed twice";
    case "unreachable-arm": return "no arm can follow _";
  }
  return code;
}
