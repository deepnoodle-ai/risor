// The static rules (01 14.4; 05 section 22), and name resolution.
//
// One walk over the tree reports the static diagnostics and records, for the
// compiler, what each name and each call resolves to. Every node is visited
// once, and nothing is skipped after a diagnostic (05 22.4). Long flat forms
// (a chain of binary operators, a chain of steps, an `else` chain) are walked
// by loops, so a program of 100,000 terms needs no deep stack.

import { resolveProgramType } from "../types/resolve.ts";
import type { TypeNames } from "../types/resolve.ts";
import type { Type } from "../types/type.ts";
import type * as A from "../syntax/ast.ts";
import type { Diagnostic } from "../syntax/diagnostic.ts";
import type { Source, Span } from "../syntax/source.ts";
import { catalogMessage } from "./failure.ts";

/** The counts of a signature (05 22.3). `fns[i]` is the parameter count of an `Fn` parameter i, or null. */
export interface Counts {
  min: number;
  max: number; // Infinity with a rest parameter
  fns: (number | null)[];
  /** Whether the last parameter is a rest parameter. */
  rest: boolean;
  /** The parameters' names, for a message (f08 3.5). */
  names?: string[];
}

export function fnAt(c: Counts, i: number): number | null {
  if (c.fns.length === 0) return null;
  if (i >= c.fns.length) return c.rest ? c.fns[c.fns.length - 1] : null;
  return c.fns[i];
}

const NO_TYPE_NAMES: TypeNames = { opaque: new Set(), named: new Map() };

export interface StaticEnv {
  variables: ReadonlySet<string>;
  /** Host functions by full name, `geo.distance` or `rate`. */
  hostFunctions: ReadonlyMap<string, Counts>;
  namespaces: ReadonlySet<string>;
  /** Library functions (entries with no receiver). */
  library: ReadonlyMap<string, Counts>;
  /** Package F4: the type names and the named types' types (f04 6.1 item 1). */
  typeNames?: TypeNames;
  /** Package F6: the constants of the groups that are on, by full name (f06 2.5). */
  constants?: ReadonlyMap<string, number>;
}

/**
 * Package F17, step E3: a function binding (f17 13.1), the binding a
 * parameter with a function type makes. `slot` holds, at run time, the lambda
 * bound to it; `param` is the parameter, with its function type.
 */
export type FnRef = { k: "fn"; slot: number; param: A.Param };

export type Ref =
  | { k: "slot"; slot: number }
  | { k: "var"; name: string }
  | { k: "lambda"; id: number }
  | FnRef;

export type Target =
  | { k: "lambda"; id: number }
  | { k: "library"; name: string }
  | { k: "host"; name: string }
  | FnRef
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
  /** The name each const-bound lambda is bound to, by id (f08 4.2). */
  lambdaNames?: Map<number, string>;
  /** Each Ident that names a const-bound lambda as an argument (a use, 05 13.9). */
  lambdaArgs: Set<A.Ident>;
  /** Package F4: each `As` node's resolved type; null after a diagnostic of f04 6.1. */
  asTypes: Map<A.As, Type | null>;
  /** Package F6: each `Member` that reads a constant of a group, with its value (f06 2.4 item 2). */
  constants: Map<A.Member, number>;
  /**
   * The last step of each coalesced read under `??` (01 6.6): a step that
   * gives null, not `missing-field`, when its key is absent from its
   * receiver. Fixed by the nodes alone (01 6.6 item 6).
   */
  coalesced: Set<A.Member | A.Index>;
  slotCount: number;
  /**
   * Each reference of a const-bound lambda to its own name, in its body or
   * defaults: a `Call` or an `Ident` lambda argument (05 22.5). No use (13.9).
   */
  recursive: Set<A.Call | A.Ident>;
  /** Package F16: each annotation's resolved type; null after a diagnostic of f16 3.1 (f16 3.3 item 2). */
  annTypes: Map<A.TypeAnn, Type | null>;
  /** Package F16: the type lines that declare their names (f16 3.1 item 5), with the types they declare. */
  typeLines: Map<A.TypeLine, Type>;
  /**
   * Each call binds its own names (f17 11.5). Where each slot lives: the
   * number of lambdas around the scope that binds it (0: the program, whose
   * one frame is the run's slots), and for a slot in a lambda, its index in
   * that lambda's frame (index 0 is the frame's link).
   */
  slotDepth: number[];
  slotIndex: number[];
  /** The size of each lambda's frame, its link included. */
  frameSize: Map<A.Lambda, number>;
  /** The number of lambdas around each lambda: the depth of the frame its link points to. */
  lambdaDepth: Map<A.Lambda, number>;
  /**
   * Package F17, step E3: the slot of each parameter written as a name with no
   * default and no function type, whose name a hint names (f17 13.2 item 4).
   */
  hintSlots: Set<number>;
  /** Package F17, step E3: the slot of each function parameter, with its parameter (f17 13.1). */
  fnSlots: Map<number, A.Param>;
}

/**
 * f17 11.1: a typed lambda has a result annotation, and every parameter has an
 * annotation, of a program type or of a function type. Read from the text
 * alone, whatever the types resolve to.
 */
export function typedLambda(l: A.Lambda): boolean {
  return l.result !== null && l.params.every((p) => p.type !== null || p.fn !== null);
}

/** f17 13.1: a higher-order lambda has a function parameter. */
export function higherOrder(l: A.Lambda): boolean {
  return l.params.some((p) => p.fn !== null);
}

/**
 * A binding of a scope. A value binding's `hint`: it is a parameter written
 * as a name with no default (f17 13.2 item 4). A function binding is package
 * F17's (f17 13.1).
 */
type Binding = { k: "value"; slot: number; hint: boolean } | { k: "var"; name: string } | { k: "lambda"; id: number; counts: Counts } | FnRef;

export function lambdaCounts(l: A.Lambda): Counts {
  // 05 22: the required count is the position of the last parameter with no default.
  let min = 0;
  l.params.forEach((p, i) => {
    if (p.default === null) min = i + 1;
  });
  return {
    min,
    max: l.params.length,
    // f17 13.2: a function parameter takes a lambda argument of at most its listed count.
    fns: l.params.map((p) => (p.fn === null ? null : p.fn.params.length)),
    rest: false,
    names: l.params.map((p) => (p.pattern.kind === "NamePat" ? (p.pattern.name ?? "") : "")),
  };
}

export interface StaticResult {
  diagnostics: Diagnostic[];
  resolution: Resolution;
}

export function staticRules(program: A.Program, env: StaticEnv, source?: Source): StaticResult {
  const w = new Walker(env, source);
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
  readonly res: Resolution = {
    refs: new Map(),
    targets: new Map(),
    slots: new Map(),
    catchSlots: new Map(),
    lambdas: [],
    lambdaArgs: new Set(),
    asTypes: new Map(),
    constants: new Map(),
    coalesced: new Set(),
    slotCount: 0,
    recursive: new Set(),
    annTypes: new Map(),
    typeLines: new Map(),
    slotDepth: [],
    slotIndex: [],
    frameSize: new Map(),
    lambdaDepth: new Map(),
    hintSlots: new Set(),
    fnSlots: new Map(),
  };
  /** The next free index of the frame of each lambda whose text the walk is in, innermost last (f17 11.5). */
  private readonly frames: number[] = [];
  /** The const-bound lambdas whose text the walk is in, by id (05 22.5). */
  private readonly walking = new Set<number>();
  /** For each const-bound lambda, its reference to its own name with the least start (05 22.5). */
  private readonly selfRefs = new Map<number, Span>();
  /** Package F16: the type names in scope, the environment's and the type lines' walked so far (f16 3.1 items 2, 7). */
  private typeNames: TypeNames;
  /** The `??` nodes of the chains already marked (coalesce). */
  private readonly chained = new Set<A.Binary>();
  private readonly scopes: Map<string, Binding>[] = [];
  private readonly env: StaticEnv;

  private readonly source: Source | undefined;

  constructor(env: StaticEnv, source?: Source) {
    this.env = env;
    this.source = source;
    this.typeNames = env.typeNames ?? NO_TYPE_NAMES;
    const outer = new Map<string, Binding>();
    for (const v of env.variables) outer.set(v, { k: "var", name: v });
    this.scopes.push(outer);
  }

  /** A diagnostic with its message from the catalog (f08 3.5); `text` is the source text at the span. */
  private diag(code: string, span: Span, template = "default", params: Record<string, unknown> = {}): void {
    const ps = { ...params };
    if (ps.text === true) ps.text = this.source === undefined ? "" : this.source.slice(span.start, span.end);
    this.diags.push({ phase: "check", code, span: { start: span.start, end: span.end }, message: catalogMessage(code, template, ps) });
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

  /**
   * Bind a name in the innermost scope (05 22.7). Returns the slot, or null.
   * With no binding `b`, the name gets a slot, and `make` gives its binding:
   * a value binding with no hint when it is not given.
   */
  private bind(name: string | null, span: Span, b?: Binding, make?: (slot: number) => Binding): number | null {
    if (name === null || name === "_") return null;
    if (this.env.namespaces.has(name)) {
      this.diag("shadows-namespace", span, "default", { name });
      return null;
    }
    const scope = this.scopes[this.scopes.length - 1];
    if (scope.has(name)) this.diag("duplicate-name", span, "default", { name });
    if (b === undefined) {
      const slot = this.res.slotCount++;
      // f17 11.5: a name of the program in the run's slots, a name in a lambda in that lambda's frame.
      const depth = this.frames.length;
      this.res.slotDepth[slot] = depth;
      this.res.slotIndex[slot] = depth === 0 ? slot : this.frames[depth - 1]++;
      scope.set(name, make === undefined ? { k: "value", slot, hint: false } : make(slot));
      return slot;
    }
    scope.set(name, b);
    return null;
  }

  program(p: A.Program): void {
    this.push();
    // f16 3.1 item 1: the type lines and the const lines in the order of the text, then the result.
    let t = 0;
    for (const c of p.consts) {
      while (t < p.types.length && p.types[t].span.start < c.span.start) this.typeLine(p.types[t++]);
      this.constLine(c);
    }
    while (t < p.types.length) this.typeLine(p.types[t++]);
    this.expr(p.result);
    this.pop();
  }

  /** The names of the type lines walked so far (f16 3.1 item 4). */
  private readonly typeLineNames = new Set<string>();

  /** Package F16: a type line (f16 3.1 items 2 to 6). */
  private typeLine(l: A.TypeLine): void {
    const env = this.env.typeNames ?? NO_TYPE_NAMES;
    const clash = env.named.has(l.name) || env.opaque.has(l.name);
    if (clash) this.diag("name-clash", l.nameSpan, "default", { name: l.name });
    else if (this.typeLineNames.has(l.name)) this.diag("duplicate-name", l.nameSpan, "default", { name: l.name });
    this.typeLineNames.add(l.name);
    const t = this.annotation(l.type);
    if (clash) return;
    // A line that stands replaces an earlier line of its name (f16 3.1 item 4, R240); one whose type has a
    // diagnostic declares nothing, nor one whose type the repair supplied (f16 7.3 item 5).
    // The map is changed in place: every annotation is resolved, eagerly, when the walk reaches it, so it
    // sees the type lines before it and no later one. The environment's own map is copied once, not changed.
    if (this.lineNamed === null) {
      this.lineNamed = new Map(this.typeNames.named);
      this.typeNames = { opaque: this.typeNames.opaque, named: this.lineNamed };
    }
    if (t === null || l.supplied === true) this.lineNamed.delete(l.name);
    else {
      this.lineNamed.set(l.name, t);
      this.res.typeLines.set(l, t);
    }
  }

  /** The named types in scope once a type line is walked: the environment's and the type lines' (f16 3.1 item 7). */
  private lineNamed: Map<string, Type> | null = null;

  /** Package F16: resolve an annotation's program type in the type names in scope (f16 3.1 item 7). */
  private annotation(a: A.TypeAnn): Type | null {
    const r = resolveProgramType(a.type, a.span, a.spans, this.typeNames);
    if (r.ok) {
      this.res.annTypes.set(a, r.type);
      return r.type;
    }
    this.res.annTypes.set(a, null);
    for (const d of r.diagnostics) this.diag(d.code, d.span, d.template, d.params);
    return null;
  }

  private constLine(c: A.Const): void {
    if (c.value.kind === "Lambda") {
      // 05 22.7: the name is bound first; the lambda is walked in the scope with it bound.
      const id = this.res.lambdas.length;
      this.res.lambdas.push(c.value);
      const p = c.pattern as A.NamePat;
      (this.res.lambdaNames ??= new Map()).set(id, p.name ?? "");
      this.bind(p.name, p.span, { k: "lambda", id, counts: lambdaCounts(c.value) });
      this.res.slots.set(p, -1 - id);
      this.walking.add(id);
      this.lambda(c.value);
      this.walking.delete(id);
      const r = this.selfRefs.get(id);
      if (r !== undefined) this.diag("recursion-needs-types", r, "default", { name: p.name ?? "" });
      return;
    }
    this.expr(c.value);
    if (c.type !== null) this.annotation(c.type);
    this.pattern(c.pattern);
  }

  /**
   * A reference of a const-bound lambda to its own name (05 22.5): kept when it has the least start.
   * Package F17 (f17 11.2): a typed lambda's is no error, and is a use as any other (11.3).
   */
  private selfRef(id: number, node: A.Call | A.Ident, span: Span): void {
    if (!this.walking.has(id) || typedLambda(this.res.lambdas[id])) return;
    this.res.recursive.add(node);
    const r = this.selfRefs.get(id);
    if (r === undefined || span.start < r.start) this.selfRefs.set(id, span);
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
    this.res.lambdaDepth.set(l, this.frames.length);
    this.frames.push(1);
    this.push();
    for (const p of l.params) {
      if (p.default !== null) this.expr(p.default);
      // Package F16: an annotation, or the types of a function type (f16 3.1 items 7, 9).
      if (p.type !== null) this.annotation(p.type);
      if (p.fn !== null) {
        for (const fp of p.fn.params) this.annotation(fp.type);
        if (p.fn.result !== null) this.annotation(p.fn.result);
      }
      const pat = p.pattern;
      if (pat.kind === "NamePat" && p.fn !== null) {
        // f17 13.1 item 1: a function binding, bound as a parameter is. Its slot holds the bound lambda.
        const s = this.bind(pat.name, pat.span, undefined, (slot) => ({ k: "fn", slot, param: p }));
        if (s !== null) {
          this.res.slots.set(pat, s);
          this.res.fnSlots.set(s, p);
        }
      } else if (pat.kind === "NamePat" && p.default === null) {
        // f17 13.2 item 4: a parameter written as a name with no default; its hints name the fix.
        const s = this.bind(pat.name, pat.span, undefined, (slot) => ({ k: "value", slot, hint: true }));
        if (s !== null) {
          this.res.slots.set(pat, s);
          this.res.hintSlots.add(s);
        }
      } else this.pattern(pat);
    }
    if (l.result !== null) this.annotation(l.result);
    this.expr(l.body);
    this.pop();
    this.res.frameSize.set(l, this.frames.pop()!);
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

  /**
   * An `Ident`. `asLambdaArg`: it is a lambda argument. A reference of a
   * lambda to its own name is tested also where rule 8 of f17 13.2 reports
   * the argument: that rule is the lambda's, not the argument's (13.2 item 1).
   */
  private ident(id: A.Ident, asLambdaArg: boolean): void {
    const b = this.lookup(id.name);
    if (b !== undefined) {
      if (b.k === "value") this.res.refs.set(id, { k: "slot", slot: b.slot });
      else if (b.k === "var") this.res.refs.set(id, { k: "var", name: b.name });
      else if (b.k === "fn") {
        // f17 13.2 rule 4: a function binding read as a value; as a lambda argument it is no read (13.2 item 3).
        this.res.refs.set(id, b);
        if (!asLambdaArg) this.diag("not-a-value", id.span, "function", { name: id.name });
      } else {
        this.res.refs.set(id, { k: "lambda", id: b.id });
        if (asLambdaArg) {
          this.res.lambdaArgs.add(id);
          this.selfRef(b.id, id, id.span);
        } else this.diag("not-a-value", id.span, "lambda", { name: id.name });
      }
      return;
    }
    if (this.env.namespaces.has(id.name)) this.diag("not-a-value", id.span, "namespace", { name: id.name });
    else if (this.env.hostFunctions.has(id.name) || this.env.library.has(id.name)) this.diag("not-a-value", id.span, "function", { name: id.name });
    else this.diag("undefined-name", id.span, "name", { name: id.name });
  }

  /** 05 22.4, with f17 13.1: a `Lambda`, or an `Ident` whose innermost binding is a `const`-bound lambda or a function binding. */
  private isLambdaArg(a: A.Arg): boolean {
    if (a.kind === "Lambda") return true;
    if (a.kind !== "Ident") return false;
    const k = this.lookup(a.name)?.k;
    return k === "lambda" || k === "fn";
  }

  /** f17 13.2 rule 8: a lambda argument that is a higher-order lambda, written there or named. */
  private isHigherOrder(a: A.Arg): boolean {
    if (a.kind === "Lambda") return higherOrder(a);
    if (a.kind !== "Ident") return false;
    const b = this.lookup(a.name);
    return b !== undefined && b.k === "lambda" && higherOrder(this.res.lambdas[b.id]);
  }

  /**
   * f17 13.2 rule 8, at every `Call` and `MethodCall`, whatever the callee:
   * each higher-order lambda argument is `lambda-to-lambda`, template
   * `higher-order`. Returns which arguments it reported.
   */
  private higherOrderArgs(args: A.Arg[], callName: string): boolean[] {
    return args.map((a) => {
      if (!this.isHigherOrder(a)) return false;
      this.diag("lambda-to-lambda", a.span, "higher-order", { name: callName });
      return true;
    });
  }

  private walkArgs(args: A.Arg[]): void {
    for (const a of args) {
      if (a.kind === "Lambda") this.lambda(a);
      else if (a.kind === "Ident" && this.isLambdaArg(a)) this.ident(a, true);
      else this.expr(a);
    }
  }

  /** The count of parameters of a lambda argument; a function parameter's is its listed count (f17 13.1). */
  private argParams(a: A.Arg): number {
    if (a.kind === "Lambda") return a.params.length;
    const b = this.lookup((a as A.Ident).name);
    if (b !== undefined && b.k === "fn") return b.param.fn!.params.length;
    return b !== undefined && b.k === "lambda" ? b.counts.max : 0;
  }

  /** f17 13.2 rule 3: the template of a value at a function parameter, and its parameters. */
  private valueAtFn(a: A.Arg, param: string, callName: string): void {
    if (a.kind === "Ident") {
      const b = this.lookup(a.name);
      if (b !== undefined && b.k === "value" && b.hint) {
        this.diag("arg-mismatch", a.span, "parameter-at-fn", { name: a.name });
        return;
      }
    }
    this.diag("arg-mismatch", a.span, "function", { param, name: callName });
  }

  /** 05 22.5. */
  private call(node: A.Call | A.MethodCall, namespace: string | null, name: string, nameSpan: Span, args: A.Arg[]): void {
    let target: Target = { k: "unknown" };
    let counts: Counts | null = null;
    let isLambda = false;
    if (namespace !== null) {
      const full = `${namespace}.${name}`;
      const c = this.env.hostFunctions.get(full);
      const l = this.env.library.get(full);
      if (c !== undefined) {
        target = { k: "host", name: full };
        counts = c;
      } else if (l !== undefined) {
        // f06 2.5 item 1: an entry of a group that is on.
        target = { k: "library", name: full };
        counts = l;
      } else this.diag("undefined-function", nameSpan, "default", { name: full });
    } else {
      const b = this.lookup(name);
      if (b !== undefined && b.k === "lambda") {
        target = { k: "lambda", id: b.id };
        counts = b.counts;
        isLambda = true;
        if (node.kind === "Call") this.selfRef(b.id, node, nameSpan);
      } else if (b !== undefined && b.k === "fn") {
        // f17 13.2 rule 1: a call of a function binding.
        target = b;
      } else if (b !== undefined) {
        // f17 13.2 rule 1 and item 4: the hint names the fix for a parameter written as a name with no default.
        this.diag("call-of-parameter", nameSpan, b.k === "value" && b.hint ? "fn-hint" : "default", { name });
      } else if (this.env.hostFunctions.has(name)) {
        target = { k: "host", name };
        counts = this.env.hostFunctions.get(name)!;
      } else if (this.env.library.has(name)) {
        target = { k: "library", name };
        counts = this.env.library.get(name)!;
      } else {
        this.diag("undefined-function", nameSpan, "default", { name });
      }
    }
    this.res.targets.set(node, target);
    const callName = namespace === null ? name : `${namespace}.${name}`;
    // f17 13.2 rule 8 decides its argument first and alone, at every call (13.2 item 1).
    const higher = this.higherOrderArgs(args, callName);
    if (target.k === "fn") {
      // f17 13.2 rule 7: exactly the listed count, and a value at each listed parameter.
      const listed = target.param.fn!.params;
      if (args.length !== listed.length) {
        this.diag("arity", node.span, "exact", { name, count: args.length, required: listed.length, most: listed.length });
      } else {
        args.forEach((a, i) => {
          if (!higher[i] && this.isLambdaArg(a)) this.diag("not-a-value", a.span, "argument", { name, param: listed[i].word });
        });
      }
    } else if (counts !== null) {
      if (args.length < counts.min || args.length > counts.max) {
        const c = counts;
        this.diag("arity", node.span, c.rest ? "at-least" : c.min === c.max ? "exact" : "range", { name: callName, count: args.length, required: c.min, most: c.max });
      } else {
        // f17 13.2 item 2: each argument that rule 8 does not report is tested by its callee, left to right.
        args.forEach((a, i) => {
          if (higher[i]) return;
          const fn = fnAt(counts!, i);
          const names = counts!.names ?? [];
          if (isLambda) {
            if (fn === null) {
              // f17 13.2 rule 2: a lambda argument at a parameter with no function type.
              if (this.isLambdaArg(a)) this.diag("lambda-to-lambda", a.span, "default", { name: callName });
              return;
            }
            // f17 13.2 rules 3 and 9: at a function parameter, a lambda argument of at most the listed count.
            if (!this.isLambdaArg(a)) this.valueAtFn(a, names[i] ?? "", callName);
            else if (this.argParams(a) > fn) this.diag("arity", a.span, "parameters", { name: callName, most: fn });
            return;
          }
          if (!this.isLambdaArg(a)) return;
          if (fn === null) this.diag("not-a-value", a.span, "argument", { name: callName, param: names[Math.min(i, names.length - 1)] ?? "" });
          else if (this.argParams(a) > fn) this.diag("arity", a.span, "parameters", { name: callName, most: fn });
        });
      }
    }
    this.walkArgs(args);
  }

  /**
   * 01 6.6: the coalesced reads of the `??` chain whose top node is `top`.
   * The chain's operands are the operands of its `??` nodes that are not `??`
   * nodes, in source order; each but the last whose node is a plain `Member`,
   * or a plain `Index` by a `String` node, with or without an `OptChain`
   * around it, is a coalesced read, and that node is its last step. A walk
   * reaches the top of a chain before any other of its `??` nodes, so each
   * chain is marked once, from its top.
   */
  private coalesce(top: A.Binary): void {
    if (this.chained.has(top)) return;
    const operands: A.Expr[] = [];
    const stack: A.Expr[] = [top];
    while (stack.length > 0) {
      const n = stack.pop()!;
      if (n.kind === "Binary" && n.op === "??") {
        this.chained.add(n);
        stack.push(n.y, n.x);
      } else operands.push(n);
    }
    for (let i = 0; i < operands.length - 1; i++) {
      const o = operands[i].kind === "OptChain" ? (operands[i] as A.OptChain).x : operands[i];
      if ((o.kind === "Member" && !o.optional) || (o.kind === "Index" && !o.optional && o.index.kind === "String")) this.res.coalesced.add(o);
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
        case "Binary":
        case "As":
        case "Each":
        case "OptChain":
        case "Member":
        case "Index":
        case "MethodCall":
          this.spine(e);
          return;
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
        case "Call":
          this.call(e, null, e.name, e.nameSpan, e.args);
          return;
      }
    }
  }

  /**
   * A spine: an expression whose first operand opens no level (01 13.1, f04
   * 2.8, f07 2.5): the left operand of a binary operator, of `as` or of `is`,
   * the receiver of a step, of a method call or of a projection. The spine is
   * walked by a loop, from its innermost such operand outward, so its length
   * costs no stack: that operand, then what each node holds beside its
   * operand, in the order of the text.
   */
  private spine(e: A.Expr): void {
    // The nodes whose first operand is being walked, outermost first.
    const nodes: A.Expr[] = [];
    let x: A.Expr = e;
    descend: for (;;) {
      switch (x.kind) {
        case "Binary":
          // A walk reaches the top of a `??` chain before any other of its nodes.
          if (x.op === "??") this.coalesce(x);
          nodes.push(x);
          x = x.x;
          continue;
        case "As":
        case "Index":
          nodes.push(x);
          x = x.x;
          continue;
        case "OptChain":
          // An OptChain holds its chain and nothing else.
          x = x.x;
          continue;
        case "Each":
          nodes.push(x);
          // A later star has no receiver: its reads are all it holds (f07 2.3).
          if (x.x === null) break descend;
          x = x.x;
          continue;
        case "Member": {
          const value = this.constantOf(x);
          if (value !== undefined) {
            // f06 2.4 item 2: the read of a constant is a primary; its receiver is not walked.
            this.res.constants.set(x, value);
            break descend;
          }
          x = x.x;
          continue;
        }
        case "MethodCall":
          if (!x.optional && x.x.kind === "Ident" && this.env.namespaces.has(x.x.name)) {
            // A call of a namespace is a primary: its receiver is the namespace, which is no value.
            const ns = x.x;
            this.call(x, ns.name, x.name, { start: ns.span.start, end: x.nameSpan.end }, x.args);
            break descend;
          }
          nodes.push(x);
          x = x.x;
          continue;
        default:
          // The innermost operand: a primary, or an expression that opens a level.
          this.expr(x);
          break descend;
      }
    }
    for (let i = nodes.length - 1; i >= 0; i--) {
      const n = nodes[i];
      switch (n.kind) {
        case "Binary":
          this.expr(n.y);
          break;
        case "As":
          // f04 6.1: the subject as any operand, then the program type.
          this.programType(n);
          break;
        case "Index":
          this.expr(n.index);
          break;
        case "MethodCall":
          // f17 13.2 rule 8 holds at a method too; the type rules and the shape test decide the rest.
          this.higherOrderArgs(n.args, n.name);
          this.walkArgs(n.args);
          break;
        case "Each":
          // f07 5.7: `x`, then each read in order, an index's expression, and a later `Each`.
          this.reads(n);
          break;
      }
    }
  }

  /** The reads of a projection: an index's expression, and the reads of a later star (f07 5.7). */
  private reads(e: A.Each): void {
    for (const r of e.reads) {
      if (r.kind === "Index") this.expr(r.index);
      else if (r.kind === "Each") this.reads(r);
    }
  }

  /** Resolve the program type of an `As` node (f04 3.5, 6.1). */
  private programType(e: A.As): void {
    // f16 3.1 item 7: in the type lines before the line that holds it.
    const r = resolveProgramType(e.type, e.typeSpan, e.typeSpans, this.typeNames);
    if (r.ok) this.res.asTypes.set(e, r.type);
    else {
      this.res.asTypes.set(e, null);
      for (const d of r.diagnostics) this.diag(d.code, d.span, d.template, d.params);
    }
  }

  /** The value of a constant that a plain `Member` on a group's name reads (f06 2.4 item 2). */
  private constantOf(m: A.Member): number | undefined {
    if (m.optional || m.x.kind !== "Ident" || !this.env.namespaces.has(m.x.name) || this.lookup(m.x.name) !== undefined) return undefined;
    return this.env.constants?.get(`${m.x.name}.${m.name}`);
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
        if (keys.has(k)) this.diag("duplicate-key", en.key.span, "default", { key: k });
        keys.add(k);
      }
      if (en.shorthand) this.ident(en.value as A.Ident, false);
      else this.expr(en.value);
    }
  }

  /**
   * 05 22.4, 22.8, with package F17 (f17 4.1): a literal is compared with the
   * earlier literals of its arm and those of earlier plain arms, and an arm is
   * unreachable after the first plain `_`. Then the arm's guard and its result.
   */
  private match(m: A.Match): void {
    this.expr(m.subject);
    // The literals of the arms before with no guard, and those of the arm itself. They are
    // kept as sets, so a `match` of many arms costs its length and not the square of it.
    const lits = new Literals();
    let defaultSeen = false;
    for (const arm of m.arms) {
      if (defaultSeen) this.diag("unreachable-arm", arm.span);
      const own = new Literals();
      for (const l of arm.literals) {
        if (lits.has(l) || own.has(l)) this.diag("duplicate-case", l.span, "default", { text: true });
        own.add(l);
      }
      if (arm.guard === null) {
        for (const l of arm.literals) lits.add(l);
        if (arm.isDefault) defaultSeen = true;
      } else this.expr(arm.guard);
      this.expr(arm.result);
    }
  }
}

/** A set of literals. Two literals are the same when they are of one kind and their values are `===`. */
class Literals {
  private isNull = false;
  // A Set holds 0 and -0 as one value, as `===` does, and no literal is NaN.
  private readonly bools = new Set<boolean>();
  private readonly numbers = new Set<number>();
  private readonly strings = new Set<string>();

  has(l: A.Literal): boolean {
    switch (l.kind) {
      case "Null":
        return this.isNull;
      case "Bool":
        return this.bools.has(l.value);
      case "Number":
        return this.numbers.has(l.value);
      case "String":
        return this.strings.has(l.value);
    }
  }

  add(l: A.Literal): void {
    switch (l.kind) {
      case "Null":
        this.isNull = true;
        break;
      case "Bool":
        this.bools.add(l.value);
        break;
      case "Number":
        this.numbers.add(l.value);
        break;
      case "String":
        this.strings.add(l.value);
        break;
    }
  }
}

