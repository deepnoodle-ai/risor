// The compiler: a syntax tree, resolved by the static rules, becomes a tree
// of closures (no bytecode, no virtual machine). The closures evaluate in the
// order of 02 section 2 and charge by sections 5 and 6.
//
// Every binding has one fixed slot for the whole run (02 11.1): no lambda is
// entered while an earlier call of it is in progress. Long flat forms (a chain
// of binary operators, a chain of steps, an `else` chain, `const` lines) are
// compiled and run by loops, so no host stack is spent on them.

import type * as A from "../syntax/ast.ts";
import type { Span } from "../syntax/source.ts";
import { ABSENT, RMap, mapGet, mapKeys, size, truthy, cpAt, cpLength, lcpWeight } from "../value/value.ts";
import { isWhole } from "../value/number.ts";
import { RunFailure, caught, messageOf } from "./failure.ts";
import { arith, contains, equal, negate, order, render, sizeTogether } from "./ops.ts";
import type { ArithOp, OrderOp } from "./ops.ts";
import type { Run } from "./run.ts";
import type { Counts, Resolution, Target } from "./static.ts";
import { FUNCTIONS, method as libraryMethod } from "../library/registry.ts";
import type { Callback, Entry } from "../library/registry.ts";
import { paramAt, testArgs } from "../library/args.ts";

export type Code = (r: Run) => unknown;
type Binder = (r: Run, v: unknown) => void;

/** What a host function's implementation is given (03 3.5.2). */
export interface CallHandle {
  readonly signal?: AbortSignal;
  charge(steps: number): void;
  allocate(units: number): void;
}

/** Thrown by a host function to return an error (03 3.5.3): `function-error`. */
export class HostError extends Error {}

export interface HostImpl {
  counts: Counts;
  call(handle: CallHandle, args: unknown[]): unknown;
}

export interface CompileEnv {
  /** Host functions by full name. */
  functions: ReadonlyMap<string, HostImpl>;
  /** Methods of opaque types: type name, then method name. */
  methods: ReadonlyMap<string, ReadonlyMap<string, HostImpl>>;
}

interface LambdaCode {
  params: { bind: Binder; dflt: Code | null }[];
  body: Code;
  n: number;
}

const SHORT: unique symbol = Symbol("short");

export interface Compiled {
  run: Code;
  /** Library entries the program names that have no implementation yet. */
  missing: Set<string>;
}

export function compileProgram(p: A.Program, res: Resolution, env: CompileEnv): Compiled {
  const c = new Compiler(res, env);
  const consts = p.consts.map((k) => c.constLine(k));
  const result = c.expr(p.result);
  const run: Code = (r) => {
    for (const k of consts) k(r);
    return result(r);
  };
  return { run, missing: c.missing };
}

class Compiler {
  readonly missing = new Set<string>();
  private readonly res: Resolution;
  private readonly env: CompileEnv;
  /** Each lambda's code, by its node. */
  private readonly codes = new Map<A.Lambda, LambdaCode>();
  /** The levels open at the first token of the lambda whose body is being compiled. */
  private lambdaLevel = 0;

  constructor(res: Resolution, env: CompileEnv) {
    this.res = res;
    this.env = env;
  }

  // ---- bindings ----------------------------------------------------------

  constLine(k: A.Const): Code {
    if (k.value.kind === "Lambda") {
      this.lambda(k.value);
      return (r) => {
        r.charge(1); // a lambda evaluated (02 5.2)
        return null;
      };
    }
    const v = this.expr(k.value);
    const bind = this.pattern(k.pattern);
    return (r) => {
      bind(r, v(r));
      return null;
    };
  }

  private lambda(l: A.Lambda): LambdaCode {
    const saved = this.lambdaLevel;
    this.lambdaLevel = l.level;
    const params = l.params.map((p) => ({ dflt: p.default === null ? null : this.expr(p.default), bind: this.pattern(p.pattern) }));
    const body = this.expr(l.body);
    this.lambdaLevel = saved;
    const lc = { params, body, n: l.params.length };
    this.codes.set(l, lc);
    return lc;
  }

  private pattern(p: A.Pattern): Binder {
    switch (p.kind) {
      case "NamePat": {
        const s = this.res.slots.get(p);
        if (s === undefined) return () => {};
        return (r, v) => {
          r.slots[s] = v;
        };
      }
      case "MapPat":
        return this.mapPattern(p);
      case "ListPat":
        return this.listPattern(p);
    }
  }

  private mapPattern(p: A.MapPat): Binder {
    const span = p.span;
    const fields = p.fields.map((f) => ({
      key: f.key,
      keySize: size(f.key),
      slot: this.res.slots.get(f),
      dflt: f.default === null ? null : this.expr(f.default),
      span: f.span,
    }));
    const named = new Set(p.fields.map((f) => f.key));
    const restSlot = p.rest === null ? undefined : this.res.slots.get(p.rest);
    const hasRest = p.rest !== null;
    return (r, v) => {
      const k = r.kind(v);
      if (k === "null") throw new RunFailure("null-operand", span, "pattern");
      if (k !== "map") throw new RunFailure("wrong-kind", span, "pattern-map", { kind: k });
      for (const f of fields) {
        r.charge(1 + f.keySize);
        const g = mapGet(v as object, f.key);
        let val: unknown = g === ABSENT ? ABSENT : r.read(g, f.span);
        if (val === ABSENT || val === null) {
          if (f.dflt !== null) val = f.dflt(r);
          else if (val === ABSENT) throw new RunFailure("missing-field", f.span, "pattern", { name: f.key, key: f.key });
        }
        if (f.slot !== undefined) r.slots[f.slot] = val;
      }
      if (hasRest) {
        const keys = mapKeys(v as object).filter((key) => !named.has(key));
        r.charge(keys.length + sizeTogether(keys));
        r.alloc(1 + 2 * keys.length);
        const m = new RMap();
        for (const key of keys) m.set(key, mapGet(v as object, key));
        if (restSlot !== undefined) r.slots[restSlot] = m;
      }
    };
  }

  private listPattern(p: A.ListPat): Binder {
    const span = p.span;
    const elems = p.elems.map((e) => ({
      slot: this.res.slots.get(e.pat),
      dflt: e.default === null ? null : this.expr(e.default),
      span: e.span,
    }));
    const restSlot = p.rest === null ? undefined : this.res.slots.get(p.rest);
    const hasRest = p.rest !== null;
    return (r, v) => {
      const k = r.kind(v);
      if (k === "null") throw new RunFailure("null-operand", span, "pattern");
      if (k !== "list") throw new RunFailure("wrong-kind", span, "pattern-list", { kind: k });
      const xs = v as unknown[];
      for (let i = 0; i < elems.length; i++) {
        const e = elems[i];
        r.charge(1);
        let val: unknown = i < xs.length ? r.read(xs[i], e.span) : ABSENT;
        if (val === ABSENT || val === null) {
          if (e.dflt !== null) val = e.dflt(r);
          else if (val === ABSENT) throw new RunFailure("index-range", e.span, "pattern", { index: i, length: xs.length });
        }
        if (e.slot !== undefined) r.slots[e.slot] = val;
      }
      if (hasRest) {
        const m = Math.max(0, xs.length - elems.length);
        r.charge(m);
        r.alloc(1 + m);
        const out = xs.slice(elems.length);
        if (restSlot !== undefined) r.slots[restSlot] = out;
      }
    };
  }

  // ---- expressions --------------------------------------------------------

  expr(e: A.Expr): Code {
    switch (e.kind) {
      case "Number":
      case "String":
      case "Bool": {
        const v = e.value;
        return () => v;
      }
      case "Null":
        return () => null;
      case "Ident":
        return this.ident(e);
      case "Template":
        return this.template(e);
      case "List":
        return this.list(e);
      case "Map":
        return this.map(e);
      case "Unary": {
        const x = this.expr(e.x);
        const span = e.span;
        if (e.op === "-") return (r) => negate(r, x(r), span);
        return (r) => {
          const v = x(r);
          r.charge(1);
          return !truthy(v, r.cls);
        };
      }
      case "Binary":
        return this.binary(e);
      case "Cond":
        return this.cond(e);
      case "Block": {
        const consts = e.consts.map((k) => this.constLine(k));
        const result = this.expr(e.result);
        return (r) => {
          for (const k of consts) k(r);
          return result(r);
        };
      }
      case "Match":
        return this.match(e);
      case "Try":
        return this.tryExpr(e);
      case "OptChain":
      case "Member":
      case "Index":
      case "MethodCall":
      case "Call":
        return this.chain(e);
      case "Bad":
        throw new Error("a Bad node is never compiled");
    }
  }

  private ident(e: A.Ident): Code {
    const ref = this.res.refs.get(e);
    if (ref === undefined) throw new Error(`unresolved name ${e.name}`);
    const span = e.span;
    if (ref.k === "slot") {
      const s = ref.slot;
      return (r) => r.slots[s];
    }
    if (ref.k === "var") {
      const name = ref.name;
      return (r) => r.variable(name, span);
    }
    throw new Error("a lambda name is not a value");
  }

  private template(e: A.Template): Code {
    const parts = e.parts.map((p) => (typeof p === "string" ? p : this.expr(p)));
    const span = e.span;
    return (r) => {
      let out = "";
      for (const p of parts) out += typeof p === "string" ? p : render(r, p(r), span);
      const sz = size(out);
      r.charge(1 + sz);
      r.alloc(1 + sz);
      return out;
    };
  }

  private list(e: A.List): Code {
    const items = e.items.map((it) =>
      it.kind === "Spread" ? { spread: true, code: this.expr(it.value), span: it.span } : { spread: false, code: this.expr(it), span: it.span },
    );
    return (r) => {
      const out: unknown[] = [];
      for (const it of items) {
        const v = it.code(r);
        if (!it.spread) {
          out.push(v);
          continue;
        }
        const k = r.kind(v);
        if (k === "null") throw new RunFailure("null-operand", it.span, "spread");
        if (k !== "list") throw new RunFailure("wrong-kind", it.span, "spread-list", { kind: k });
        const xs = v as unknown[];
        r.charge(xs.length);
        for (let i = 0; i < xs.length; i++) out.push(xs[i]);
      }
      r.alloc(1 + out.length);
      return out;
    };
  }

  private map(e: A.MapLit): Code {
    type Item =
      | { t: "key"; key: string; keySize: number; code: Code }
      | { t: "computed"; key: Code; code: Code; span: Span }
      | { t: "spread"; code: Code; span: Span };
    const items: Item[] = e.entries.map((en): Item => {
      if (en.kind === "Spread") return { t: "spread", code: this.expr(en.value), span: en.span };
      if (en.key.kind === "expr") return { t: "computed", key: this.expr(en.key.expr), code: this.expr(en.value), span: en.span };
      const key = en.key.kind === "word" ? en.key.text : en.key.value;
      return { t: "key", key, keySize: size(key), code: this.expr(en.value) };
    });
    return (r) => {
      const m = new RMap();
      for (const it of items) {
        if (it.t === "key") {
          const v = it.code(r);
          r.charge(it.keySize);
          m.set(it.key, v);
        } else if (it.t === "computed") {
          const k = it.key(r);
          const kk = r.kind(k);
          if (kk === "null") throw new RunFailure("null-operand", it.span, "key");
          if (kk !== "string") throw new RunFailure("wrong-kind", it.span, "key", { found: k });
          const v = it.code(r);
          r.charge(size(k as string));
          m.set(k as string, v);
        } else {
          const v = it.code(r);
          const k = r.kind(v);
          if (k === "null") throw new RunFailure("null-operand", it.span, "spread");
          if (k !== "map") throw new RunFailure("wrong-kind", it.span, "spread-map", { kind: k });
          const keys = mapKeys(v as object);
          r.charge(keys.length + sizeTogether(keys));
          for (const key of keys) m.set(key, mapGet(v as object, key));
        }
      }
      r.alloc(1 + 2 * m.size);
      return m;
    };
  }

  /** A left-associative chain of binary operators, compiled and run by a loop. */
  private binary(e: A.Binary): Code {
    const spine: A.Binary[] = [];
    let x: A.Expr = e;
    while (x.kind === "Binary" && x.op !== "**") {
      spine.push(x);
      x = x.x;
    }
    if (spine.length === 0) {
      // `**`: right-associative; its left operand is a chain or a primary.
      const a = this.expr(e.x);
      const b = this.expr(e.y);
      const span = e.span;
      return (r) => {
        const va = a(r);
        return arith(r, "**", va, b(r), span);
      };
    }
    spine.reverse();
    const first = this.expr(x);
    const ops = spine.map((n) => ({ op: n.op, y: this.expr(n.y), span: n.span }));
    return (r) => {
      let acc = first(r);
      for (const o of ops) {
        switch (o.op) {
          case "&&":
            if (truthy(acc, r.cls)) acc = o.y(r);
            break;
          case "||":
            if (!truthy(acc, r.cls)) acc = o.y(r);
            break;
          case "??":
            if (r.kind(acc) === "null") acc = o.y(r);
            break;
          case "==":
            acc = equal(r, acc, o.y(r), o.span);
            break;
          case "!=":
            acc = !equal(r, acc, o.y(r), o.span);
            break;
          case "<":
          case "<=":
          case ">":
          case ">=":
            acc = order(r, o.op as OrderOp, acc, o.y(r), o.span);
            break;
          case "in":
            acc = contains(r, "in", acc, o.y(r), o.span);
            break;
          case "not-in":
            acc = !contains(r, "not in", acc, o.y(r), o.span);
            break;
          default:
            acc = arith(r, o.op as ArithOp, acc, o.y(r), o.span);
        }
      }
      return acc;
    };
  }

  /** `? :` and `if`, with every `else` part in one loop. */
  private cond(e: A.Cond): Code {
    const tests: Code[] = [];
    const thens: Code[] = [];
    let c: A.Expr = e;
    while (c.kind === "Cond") {
      tests.push(this.expr(c.test));
      thens.push(this.expr(c.then));
      c = c.else;
    }
    const otherwise = this.expr(c);
    return (r) => {
      for (let i = 0; i < tests.length; i++) {
        if (truthy(tests[i](r), r.cls)) return thens[i](r);
      }
      return otherwise(r);
    };
  }

  private match(e: A.Match): Code {
    const subject = this.expr(e.subject);
    const arms = e.arms.map((a) => ({
      lits: a.literals.map((l) => (l.kind === "Null" ? null : l.value)),
      isDefault: a.isDefault,
      result: this.expr(a.result),
    }));
    const span = e.span;
    return (r) => {
      const s = subject(r);
      const sk = r.kind(s);
      for (const a of arms) {
        r.charge(1);
        if (a.isDefault) return a.result(r);
        for (const l of a.lits) {
          if (typeof l === "string" && sk === "string") {
            // Ruling W5: a string literal against a string subject reads text.
            r.charge(Math.floor(lcpWeight(l, s as string) / 64));
            if (l === s) return a.result(r);
          } else if (l === null ? sk === "null" : l === s) {
            return a.result(r);
          }
        }
      }
      throw new RunFailure("no-match", span, "default", { found: s });
    };
  }

  private tryExpr(e: A.Try): Code {
    const body = this.expr(e.body);
    const slot = this.res.catchSlots.get(e);
    const named = e.name !== null;
    const handler = this.expr(e.handler);
    return (r) => {
      const depth = r.depth;
      let failure: RunFailure;
      try {
        return body(r);
      } catch (x) {
        if (!(x instanceof RunFailure) || !caught(x.code)) throw x;
        failure = x;
      }
      r.depth = depth;
      if (named) {
        // The error value (02 12.4): all its steps, then all its units.
        const message = messageOf(failure, r.cls);
        const built = failure.text === null;
        const sz = size(message);
        if (built) r.charge(1 + sz);
        r.alloc(9 + (built ? 1 + sz : 0));
        const ev = new RMap([
          ["code", failure.code],
          ["data", failure.data],
          ["message", message],
          ["path", null],
        ]);
        if (slot !== undefined) r.slots[slot] = ev;
      }
      return handler(r);
    };
  }

  // ---- chains and calls ---------------------------------------------------

  private measure(level: number): number {
    return 1 + (level - this.lambdaLevel);
  }

  /** A chain: its primary, then each step left to right (02 2.3). */
  private chain(e: A.Expr): Code {
    const steps: A.Expr[] = [];
    // Only the outermost node of a chain is an OptChain: an inner one is a
    // chain of its own, written in parentheses (01 7.4 rule 5).
    let x: A.Expr = e.kind === "OptChain" ? e.x : e;
    for (;;) {
      if (x.kind === "Member" || x.kind === "Index") {
        steps.push(x);
        x = x.x;
      } else if (x.kind === "MethodCall") {
        if (this.res.targets.has(x)) break; // a call of a namespace is a primary
        steps.push(x);
        x = x.x;
      } else break;
    }
    steps.reverse();
    const primary = x.kind === "Call" || x.kind === "MethodCall" ? this.call(x) : this.expr(x);
    if (steps.length === 0) return primary;
    const codes = steps.map((s) => this.step(s));
    return (r) => {
      let v = primary(r);
      for (const s of codes) {
        v = s(r, v);
        if (v === SHORT) return null;
      }
      return v;
    };
  }

  private step(s: A.Expr): (r: Run, v: unknown) => unknown {
    if (s.kind === "Member") {
      const { name, optional, span } = s;
      const nameSize = size(name);
      return (r, v) => {
        const k = r.kind(v);
        if (k === "null") {
          if (optional) return SHORT;
          throw new RunFailure("null-operand", span, "field", { name });
        }
        if (k !== "map") throw new RunFailure("wrong-kind", span, "field", { name, kind: k });
        r.charge(1 + nameSize);
        const g = mapGet(v as object, name);
        if (g === ABSENT) {
          if (optional) return null;
          throw new RunFailure("missing-field", span, "field", { name });
        }
        return r.read(g, span);
      };
    }
    if (s.kind === "Index") {
      const { optional, span } = s;
      const index = this.expr(s.index);
      return (r, v) => {
        const k = r.kind(v);
        if (k === "null") {
          if (optional) return SHORT;
          throw new RunFailure("null-operand", span, "index");
        }
        if (k !== "list" && k !== "string" && k !== "map") throw new RunFailure("wrong-kind", span, "index", { kind: k });
        const i = index(r);
        const ki = r.kind(i);
        if (ki === "null") throw new RunFailure("null-operand", span, "key");
        if (k === "map") {
          if (ki !== "string") throw new RunFailure("wrong-kind", span, "key", { found: i });
          r.charge(1 + size(i as string));
          const g = mapGet(v as object, i as string);
          if (g === ABSENT) {
            if (optional) return null;
            throw new RunFailure("missing-field", span, "key", { key: i });
          }
          return r.read(g, span);
        }
        if (ki !== "number" || !isWhole(i as number)) throw new RunFailure("wrong-kind", span, "whole", { found: i });
        const n = i as number;
        if (k === "list") {
          r.charge(1);
          const xs = v as unknown[];
          if (n < 0 || n >= xs.length) {
            if (optional) return null;
            throw new RunFailure("index-range", span, "index", { index: n, length: xs.length, kind: "list" });
          }
          return r.read(xs[n], span);
        }
        const str = v as string;
        r.charge(1 + size(str));
        const c = cpAt(str, n);
        if (c === null) {
          if (optional) return null;
          throw new RunFailure("index-range", span, "index", { index: n, length: cpLength(str), kind: "string" });
        }
        r.alloc(1);
        return c;
      };
    }
    return this.methodStep(s as A.MethodCall);
  }

  /** An argument that is a lambda: a Lambda node, or the name of a const-bound lambda. */
  private lambdaArg(a: A.Arg): { lambda: A.Lambda | null; id: number | null; params: number } | null {
    if (a.kind === "Lambda") return { lambda: a, id: null, params: a.params.length };
    if (a.kind === "Ident") {
      const ref = this.res.refs.get(a);
      if (ref !== undefined && ref.k === "lambda") return { lambda: null, id: ref.id, params: this.res.lambdas[ref.id].params.length };
    }
    return null;
  }

  /**
   * Compile the arguments of a call. A lambda argument becomes a Callback;
   * one written in place charges its 1 step where it stands.
   */
  private args(args: A.Arg[], measure: number): (r: Run) => unknown[] {
    const codes = args.map((a) => {
      const la = this.lambdaArg(a);
      if (la === null) return { kind: "value" as const, code: this.expr(a as A.Expr) };
      const lc = la.lambda !== null ? this.lambda(la.lambda) : null;
      return { kind: "lambda" as const, lc, id: la.id, inPlace: la.lambda !== null, params: la.params };
    });
    const codesOf = this.codes;
    const named = this.res.lambdas;
    return (r) => {
      const out: unknown[] = [];
      for (const c of codes) {
        if (c.kind === "value") {
          out.push(c.code(r));
          continue;
        }
        if (c.inPlace) r.charge(1);
        const lc = c.lc ?? codesOf.get(named[c.id!])!;
        const cb: Callback = {
          params: c.params,
          call: (args) => invoke(r, lc, args, measure),
        };
        out.push(cb);
      }
      return out;
    };
  }

  private methodStep(s: A.MethodCall): (r: Run, v: unknown) => unknown {
    const { name, optional, span } = s;
    const nargs = s.args.length;
    const shapes = s.args.map((a) => this.lambdaArg(a));
    const args = this.args(s.args, this.measure(s.level));
    const env = this.env;
    for (const kind of ["string", "list", "map"]) {
      const e = libraryMethod(kind as "string", name);
      if (e !== undefined && e.impl === null) this.missing.add(`${kind}.${name}`);
    }
    return (r, v) => {
      const k = r.kind(v);
      if (k === "null") {
        if (optional) return SHORT;
        throw new RunFailure("null-operand", span, "method", { name });
      }
      // The shape test (02 2.3, rulings M1, AC1).
      let entry: Entry | undefined;
      let host: HostImpl | undefined;
      let counts: Counts;
      if (k === "opaque") {
        const t = r.cls.opaqueName(v);
        host = t === null ? undefined : env.methods.get(t)?.get(name);
        if (host === undefined) throw new RunFailure("wrong-kind", span, "method", { kind: k, name });
        counts = host.counts;
      } else {
        entry = libraryMethod(k, name);
        if (entry === undefined) throw new RunFailure("wrong-kind", span, "method", { kind: k, name });
        counts = entry.counts;
      }
      if (nargs < counts.min || nargs > counts.max) throw new RunFailure("wrong-kind", span, "count", { count: nargs, name });
      for (let i = 0; i < nargs; i++) {
        const fn = entry !== undefined ? paramAt(entry, i).fn : null;
        if (fn !== null && shapes[i] === null) throw new RunFailure("wrong-kind", span, "callback", { count: i + 1, name });
        if (fn === null && shapes[i] !== null) throw new RunFailure("wrong-kind", span, "not-callback", { count: i + 1, name });
      }
      if (entry !== undefined) {
        for (let i = 0; i < nargs; i++) {
          const fn = paramAt(entry, i).fn;
          if (fn !== null && shapes[i]!.params > fn) throw new RunFailure("wrong-kind", span, "parameters", { name, count: fn });
        }
      }
      const vals = args(r);
      if (host !== undefined) return callHost(r, host, [v, ...vals], name, span);
      return callEntry(r, entry!, v, vals, span, name);
    };
  }

  /** A call by name, or a call of a namespace (`MethodCall` resolved as a `Call`). */
  private call(node: A.Call | A.MethodCall): Code {
    const target: Target = this.res.targets.get(node) ?? { k: "unknown" };
    const span = node.span;
    const measure = this.measure(node.level);
    const args = this.args(node.args, measure);
    switch (target.k) {
      case "lambda": {
        const node = this.res.lambdas[target.id];
        const codesOf = this.codes;
        return (r) => {
          const vals = args(r);
          r.charge(1);
          return invoke(r, codesOf.get(node)!, vals, measure);
        };
      }
      case "library": {
        const entry = FUNCTIONS.get(target.name)!;
        if (entry.impl === null) this.missing.add(target.name);
        const name = target.name;
        return (r) => callEntry(r, entry, undefined, args(r), span, name);
      }
      case "host": {
        const host = this.env.functions.get(target.name);
        if (host === undefined) throw new Error(`no host function ${target.name}`);
        const name = target.name;
        return (r) => callHost(r, host, args(r), name, span);
      }
      case "unknown":
        throw new Error("a call with no callee is never compiled");
    }
  }
}

/** Call a const-bound lambda (02 11.2) or a lambda passed to the library (5.6). */
function invoke(r: Run, lc: LambdaCode, args: unknown[], measure: number): unknown {
  r.depth += measure;
  r.testDepth(r.depth);
  for (let i = 0; i < lc.params.length; i++) {
    const p = lc.params[i];
    let v = i < args.length ? args[i] : null;
    if (v === null && p.dflt !== null) v = p.dflt(r);
    p.bind(r, v);
  }
  const out = lc.body(r);
  r.depth -= measure;
  return out;
}

/** A call of a library entry, after its receiver and arguments (04 2.2, 2.6, 4.4). */
function callEntry(r: Run, entry: Entry, receiver: unknown, args: unknown[], span: Span, name: string): unknown {
  testArgs(r, entry, args, span, name);
  r.charge(1);
  if (entry.impl === null) throw new RunFailure("internal-error", span, "default");
  return entry.impl({ run: r, span, name, receiver, args });
}

/** A call of a host function or of a method of an opaque type (02 11.4, 03 3.5). */
function callHost(r: Run, host: HostImpl, args: unknown[], name: string, span: Span): unknown {
  r.charge(1);
  const handle: CallHandle = {
    signal: r.signal,
    charge: (n) => r.charge(n),
    allocate: (n) => r.alloc(n),
  };
  let out: unknown;
  try {
    out = host.call(handle, args);
  } catch (x) {
    if (x instanceof RunFailure) throw x;
    if (x instanceof Error) throw new RunFailure("function-error", span, null, {}, { text: x.message, cause: x });
    throw new RunFailure("function-panic", span, "default", { name });
  }
  r.checkTime();
  if (out instanceof Promise) throw new RunFailure("function-error", span, null, {}, { text: "a host function returned a Promise" });
  return r.read(out, span);
}

