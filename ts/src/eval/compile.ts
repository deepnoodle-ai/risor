// The compiler: a syntax tree, resolved by the static rules, becomes a tree
// of closures (no bytecode, no virtual machine). The closures evaluate in the
// order of 02 section 2 and charge by sections 5 and 6.
//
// Every binding has one fixed slot for the whole run (02 11.1): no lambda is
// entered while an earlier call of it is in progress. Long flat forms (a chain
// of binary operators, a chain of steps, an `else` chain, `const` lines) are
// compiled and run by loops, so no host stack is spent on their length; a
// short chain (NEST_MAX) is nested closures.
//
// Speed. The closures of one kind share one function literal, so the engine
// sees every site of a kind at once: calls between closures are indirect.
// The code is shaped by that. Constants, slots and variables are read in
// place (Operand) rather than by a closure; each operator has its own
// closure with a fast path for the common kinds; a field of a map is read
// with no general classification (quickMap, quickGet); a lambda whose
// parameters are plain names is entered without a list of arguments.

import type * as A from "../syntax/ast.ts";
import type { Span } from "../syntax/source.ts";
import { ABSENT, RMap, mapGet, mapKeys, size, truthy, cpAt, cpLength, lcpWeight, div64 } from "../value/value.ts";
import { isWhole } from "../value/number.ts";
import { conformWalk } from "./conform.ts";
import { RunFailure, caught, messageOf } from "./failure.ts";
import { Rendering, arith, contains, equal, negate, order, render, sizeTogether } from "./ops.ts";
import type { Kind } from "../value/value.ts";
import type { ArithOp } from "./ops.ts";
import type { Run } from "./run.ts";
import type { Counts, Resolution, Target } from "./static.ts";
import { libraryFunction, method as libraryMethod } from "../library/registry.ts";
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
  /** Library entries the environment does not have: `len`, `string.repeat` (03 3.3.1). */
  removed?: ReadonlySet<string>;
}

/**
 * A parameter of a lambda. `slot`: the slot of a parameter that is a name
 * (NAME_UNUSED when the name is never read), or PATTERN when `bind` binds it.
 */
interface ParamCode {
  bind: Binder;
  dflt: Code | null;
  slot: number;
}

const PATTERN = -1;
const NAME_UNUSED = -2;

interface LambdaCode {
  params: ParamCode[];
  body: Code;
  bodyOp: Operand;
  n: number;
  /**
   * At most three parameters, each a name with no default: the slots of the
   * first three (NAME_UNUSED for a name never read or a parameter not there).
   */
  simple: boolean;
  s0: number;
  s1: number;
  s2: number;
}

const SHORT: unique symbol = Symbol("short");

/**
 * A chain of binary operators, or of steps, up to this long is compiled into
 * nested closures, one for each operator or step. A longer chain runs in a
 * loop, so the host stack a chain spends stays small (01 13.2 bounds the
 * nesting of the source).
 */
const NEST_MAX = 4;

export interface Compiled {
  run: Code;
  /** Library entries the program names that have no implementation yet. */
  missing: Set<string>;
  /** The number of variables of the environment the program reads. */
  varCount: number;
}

/** Run a list of `const` lines, then a result. */
function sequence(consts: Code[], result: Code): Code {
  switch (consts.length) {
    case 0:
      return result;
    case 1: {
      const [a] = consts;
      return (r) => {
        a(r);
        return result(r);
      };
    }
    case 2: {
      const [a, b] = consts;
      return (r) => {
        a(r);
        b(r);
        return result(r);
      };
    }
    default:
      return (r) => {
        for (let i = 0; i < consts.length; i++) consts[i](r);
        return result(r);
      };
  }
}

export function compileProgram(p: A.Program, res: Resolution, env: CompileEnv): Compiled {
  const c = new Compiler(res, env);
  const consts = p.consts.map((k) => c.constLine(k));
  const result = c.expr(p.result);
  return { run: sequence(consts, result), missing: c.missing, varCount: c.varCount };
}

// ---- fast paths ---------------------------------------------------------------
//
// Each fast path below gives the result, the charges and the failure of the
// general path it stands in front of, for the values it accepts; any other
// value goes to the general path.

const OBJECT_PROTO = Object.prototype;
const RMAP_PROTO = RMap.prototype;
const getProto = Object.getPrototypeOf;
const isEnum = Object.prototype.propertyIsEnumerable;
const isSafe = Number.isSafeInteger;
const isFiniteNumber = Number.isFinite;

/** 1: a host plain object, which is a map; 2: an RMap; 0: let the classifier decide. */
function quickMap(r: Run, v: unknown): number {
  if (typeof v !== "object" || v === null || Array.isArray(v)) return 0;
  const p = getProto(v);
  if (p === RMAP_PROTO) return 2;
  if ((p === OBJECT_PROTO || p === null) && r.cls.plain) return 1;
  return 0;
}

/** The raw value under a key of a map that quickMap classed as `q`, or ABSENT (as mapGet). */
function quickGet(v: object, q: number, key: string): unknown {
  if (q === 2) {
    const g = (v as RMap).get(key);
    return g === undefined ? ABSENT : g;
  }
  if (!isEnum.call(v, key)) return ABSENT;
  const g = (v as Record<string, unknown>)[key];
  return g === undefined ? ABSENT : g;
}

/** Read a value out of host data or a container (as Run.read). */
function readQ(r: Run, g: unknown, span: Span): unknown {
  return typeof g === "object" ? g : r.read(g, span);
}

/** The language's truthiness (as truthy). */
function truthyQ(r: Run, v: unknown): boolean {
  return typeof v === "boolean" ? v : truthy(v, r.cls);
}

/** The kind of a value (as Run.kind). */
function kindQ(r: Run, v: unknown): Kind {
  if (typeof v === "string") return "string";
  if (Array.isArray(v)) return "list";
  return r.kind(v);
}

function rangeFail(op: string, span: Span): never {
  throw new RunFailure("number-range", span, "operator", { op });
}

/**
 * An operand: compiled code (k 0), a constant (1), a slot (2) or a variable
 * of the environment (3). A constant, a slot or a variable is read in place,
 * with no call of a closure.
 */
interface Operand {
  k: 0 | 1 | 2 | 3;
  c: Code;
  v: unknown;
  /** The slot, or the index of the variable. */
  s: number;
  /** The variable's name and span. */
  name: string;
  span: Span | null;
}

const NO_CODE: Code = () => null;

function codeOperand(c: Code): Operand {
  return { k: 0, c, v: null, s: 0, name: "", span: null };
}

/** The value of an operand. */
function ev(r: Run, o: Operand): unknown {
  switch (o.k) {
    case 0:
      return o.c(r);
    case 1:
      return o.v;
    case 2:
      return r.slots[o.s];
    default:
      return r.variable(o.s, o.name, o.span!);
  }
}

// The operators that take both operands, each with its fast path (01 6; 02 5.3).

function opEq(r: Run, x: unknown, y: unknown, span: Span): boolean {
  const t = typeof x;
  if (t === typeof y) {
    if (t === "number" || t === "boolean") {
      r.charge(1);
      return x === y;
    }
    // 02 5.3: 1 + the size of the common prefix, which is 0 when either is shorter than 64.
    if (t === "string" && ((x as string).length < 64 || (y as string).length < 64)) {
      r.charge(1);
      return x === y;
    }
  }
  return equal(r, x, y, span);
}

function opLt(r: Run, x: unknown, y: unknown, span: Span): boolean {
  if (typeof x === "number" && typeof y === "number") {
    r.charge(1);
    return x < y;
  }
  return order(r, "<", x, y, span);
}

function opLe(r: Run, x: unknown, y: unknown, span: Span): boolean {
  if (typeof x === "number" && typeof y === "number") {
    r.charge(1);
    return x <= y;
  }
  return order(r, "<=", x, y, span);
}

function opGt(r: Run, x: unknown, y: unknown, span: Span): boolean {
  if (typeof x === "number" && typeof y === "number") {
    r.charge(1);
    return x > y;
  }
  return order(r, ">", x, y, span);
}

function opGe(r: Run, x: unknown, y: unknown, span: Span): boolean {
  if (typeof x === "number" && typeof y === "number") {
    r.charge(1);
    return x >= y;
  }
  return order(r, ">=", x, y, span);
}

function opAdd(r: Run, x: unknown, y: unknown, span: Span): unknown {
  if (typeof x === "number" && typeof y === "number") {
    r.charge(1);
    const res = x + y;
    if (isSafe(x) && isSafe(y) ? !isSafe(res) : !isFiniteNumber(res)) rangeFail("+", span);
    return res + 0;
  }
  return arith(r, "+", x, y, span);
}

function opSub(r: Run, x: unknown, y: unknown, span: Span): unknown {
  if (typeof x === "number" && typeof y === "number") {
    r.charge(1);
    const res = x - y;
    if (isSafe(x) && isSafe(y) ? !isSafe(res) : !isFiniteNumber(res)) rangeFail("-", span);
    return res + 0;
  }
  return arith(r, "-", x, y, span);
}

function opMul(r: Run, x: unknown, y: unknown, span: Span): unknown {
  if (typeof x === "number" && typeof y === "number") {
    r.charge(1);
    const res = x * y;
    if (isSafe(x) && isSafe(y) ? !isSafe(res) : !isFiniteNumber(res)) rangeFail("*", span);
    return res + 0;
  }
  return arith(r, "*", x, y, span);
}

function opDiv(r: Run, x: unknown, y: unknown, span: Span): unknown {
  if (typeof x === "number" && typeof y === "number" && y !== 0) {
    r.charge(1);
    const res = x / y;
    if (!isFiniteNumber(res)) rangeFail("/", span);
    return res + 0;
  }
  return arith(r, "/", x, y, span);
}

function opRem(r: Run, x: unknown, y: unknown, span: Span): unknown {
  if (typeof x === "number" && typeof y === "number" && y !== 0) {
    r.charge(1);
    return (x % y) + 0;
  }
  return arith(r, "%", x, y, span);
}

/** A binary operator on two operands. */
function binaryOp(op: A.Binary["op"], a: Operand, b: Operand, span: Span): Code {
  switch (op) {
    case "&&":
      return (r) => {
        const v = ev(r, a);
        return truthyQ(r, v) ? ev(r, b) : v;
      };
    case "||":
      return (r) => {
        const v = ev(r, a);
        return truthyQ(r, v) ? v : ev(r, b);
      };
    case "??":
      return (r) => {
        const v = ev(r, a);
        return v === null || v === undefined ? ev(r, b) : v;
      };
    case "==":
      return (r) => opEq(r, ev(r, a), ev(r, b), span);
    case "!=":
      return (r) => !opEq(r, ev(r, a), ev(r, b), span);
    case "<":
      return (r) => opLt(r, ev(r, a), ev(r, b), span);
    case "<=":
      return (r) => opLe(r, ev(r, a), ev(r, b), span);
    case ">":
      return (r) => opGt(r, ev(r, a), ev(r, b), span);
    case ">=":
      return (r) => opGe(r, ev(r, a), ev(r, b), span);
    case "in":
      return (r) => contains(r, "in", ev(r, a), ev(r, b), span);
    case "not-in":
      return (r) => !contains(r, "not in", ev(r, a), ev(r, b), span);
    case "+":
      return (r) => opAdd(r, ev(r, a), ev(r, b), span);
    case "-":
      return (r) => opSub(r, ev(r, a), ev(r, b), span);
    case "*":
      return (r) => opMul(r, ev(r, a), ev(r, b), span);
    case "/":
      return (r) => opDiv(r, ev(r, a), ev(r, b), span);
    case "%":
      return (r) => opRem(r, ev(r, a), ev(r, b), span);
    default:
      return (r) => arith(r, op as ArithOp, ev(r, a), ev(r, b), span);
  }
}

/** A binary operator whose left operand is the value so far of a chain of operators. */
function binaryStep(op: A.Binary["op"], b: Operand, span: Span): (r: Run, x: unknown) => unknown {
  switch (op) {
    case "&&":
      return (r, x) => (truthyQ(r, x) ? ev(r, b) : x);
    case "||":
      return (r, x) => (truthyQ(r, x) ? x : ev(r, b));
    case "??":
      return (r, x) => (x === null || x === undefined ? ev(r, b) : x);
    case "==":
      return (r, x) => opEq(r, x, ev(r, b), span);
    case "!=":
      return (r, x) => !opEq(r, x, ev(r, b), span);
    case "<":
      return (r, x) => opLt(r, x, ev(r, b), span);
    case "<=":
      return (r, x) => opLe(r, x, ev(r, b), span);
    case ">":
      return (r, x) => opGt(r, x, ev(r, b), span);
    case ">=":
      return (r, x) => opGe(r, x, ev(r, b), span);
    case "in":
      return (r, x) => contains(r, "in", x, ev(r, b), span);
    case "not-in":
      return (r, x) => !contains(r, "not in", x, ev(r, b), span);
    case "+":
      return (r, x) => opAdd(r, x, ev(r, b), span);
    case "-":
      return (r, x) => opSub(r, x, ev(r, b), span);
    case "*":
      return (r, x) => opMul(r, x, ev(r, b), span);
    case "/":
      return (r, x) => opDiv(r, x, ev(r, b), span);
    case "%":
      return (r, x) => opRem(r, x, ev(r, b), span);
    default:
      return (r, x) => arith(r, op as ArithOp, x, ev(r, b), span);
  }
}

class Compiler {
  readonly missing = new Set<string>();
  private readonly res: Resolution;
  private readonly env: CompileEnv;
  /** Each lambda's code, by its node. */
  private readonly codes = new Map<A.Lambda, LambdaCode>();
  /** The levels open at the first token of the lambda whose body is being compiled. */
  private lambdaLevel = 0;
  /** The index of each variable of the environment the program reads. */
  private readonly varIndex = new Map<string, number>();

  get varCount(): number {
    return this.varIndex.size;
  }

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
    if (k.pattern.kind === "NamePat") {
      const s = this.res.slots.get(k.pattern);
      if (s === undefined) {
        return (r) => {
          v(r);
          return null;
        };
      }
      return (r) => {
        r.slots[s] = v(r);
        return null;
      };
    }
    const bind = this.pattern(k.pattern);
    return (r) => {
      bind(r, v(r));
      return null;
    };
  }

  private lambda(l: A.Lambda): LambdaCode {
    const saved = this.lambdaLevel;
    this.lambdaLevel = l.level;
    const params = l.params.map((p): ParamCode => {
      const dflt = p.default === null ? null : this.expr(p.default);
      const bind = this.pattern(p.pattern);
      const slot = p.pattern.kind === "NamePat" ? (this.res.slots.get(p.pattern) ?? NAME_UNUSED) : PATTERN;
      return { dflt, bind, slot };
    });
    const body = this.expr(l.body);
    const bodyOp = this.operand(l.body, body);
    this.lambdaLevel = saved;
    const simple = params.length <= 3 && params.every((p) => p.slot !== PATTERN && p.dflt === null);
    const slotAt = (i: number) => (simple && i < params.length ? params[i].slot : NAME_UNUSED);
    const lc: LambdaCode = { params, body, bodyOp, n: l.params.length, simple, s0: slotAt(0), s1: slotAt(1), s2: slotAt(2) };
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
      const q = quickMap(r, v);
      if (q === 0) {
        const k = r.kind(v);
        if (k === "null") throw new RunFailure("null-operand", span, "pattern");
        if (k !== "map") throw new RunFailure("wrong-kind", span, "pattern-map", { kind: k });
      }
      for (let i = 0; i < fields.length; i++) {
        const f = fields[i];
        r.charge(1 + f.keySize);
        const g = q === 0 ? mapGet(v as object, f.key) : quickGet(v as object, q, f.key);
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
          return !truthyQ(r, v);
        };
      }
      case "Binary":
        return this.binary(e);
      case "Cond":
        return this.cond(e);
      case "Block": {
        const consts = e.consts.map((k) => this.constLine(k));
        return sequence(consts, this.expr(e.result));
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
      case "As":
        return this.asExpr(e);
      case "Each": {
        const code = this.each(e);
        // A first star `?.[*]` on null makes its chain null (f07 3.1 item 2).
        return (r) => {
          const v = code(r);
          return v === SHORT ? null : v;
        };
      }
      case "Bad":
        throw new Error("a Bad node is never compiled");
    }
  }

  /**
   * A projection (package F7, f07 3, 4): level by level, element by element,
   * into one list whose units are charged once at the end. SHORT: a first
   * star `?.[*]` found null.
   */
  private each(e: A.Each): Code {
    type Level = { optional: boolean; starSpan: Span; reads: ((r: Run, v: unknown) => unknown)[]; next: Level | null };
    const level = (p: A.Each): Level => {
      const reads: ((r: Run, v: unknown) => unknown)[] = [];
      let next: Level | null = null;
      for (const s of p.reads) {
        if (s.kind === "Each") next = level(s);
        else reads.push(this.step(s));
      }
      return { optional: p.optional, starSpan: p.starSpan, reads, next };
    };
    const first = level(e);
    const ex = e.x!;
    // The chain before the first star is one chain with it: an optional step there that finds null makes it null.
    const x = ex.kind === "Member" || ex.kind === "Index" || ex.kind === "MethodCall" ? this.chain(ex, true) : this.expr(ex);
    const visit = (r: Run, recv: unknown, lv: Level, out: unknown[], isFirst: boolean): boolean => {
      const k = r.kind(recv);
      if (k === "null") {
        if (!lv.optional) throw new RunFailure("null-operand", lv.starSpan, "star", {});
        if (isFirst) return false;
        out.push(null);
        return true;
      }
      if (k !== "list") throw new RunFailure("wrong-kind", lv.starSpan, "star", { kind: k });
      const xs = recv as unknown[];
      const stepped = lv.reads.length > 0 || lv.next !== null;
      outer: for (let i = 0; i < xs.length; i++) {
        r.charge(1);
        // An element is read when a step is applied to it, and copied when none is (f07 3.6).
        let v = stepped ? r.read(xs[i], lv.starSpan) : xs[i];
        for (const read of lv.reads) {
          v = read(r, v);
          if (v === SHORT) {
            out.push(null);
            continue outer;
          }
        }
        if (lv.next !== null) visit(r, v, lv.next, out, false);
        else out.push(v);
      }
      return true;
    };
    return (r) => {
      const recv = x(r);
      if (recv === SHORT) return SHORT;
      const out: unknown[] = [];
      if (!visit(r, recv, first, out, true)) return SHORT;
      r.alloc(1 + out.length);
      return out;
    };
  }

  /** `x as T`, `x is T` (package F4, f04 5.1, 5.2). */
  private asExpr(e: A.As): Code {
    const x = this.expr(e.x);
    const t = this.res.asTypes.get(e);
    if (t === undefined || t === null) throw new Error("a program type that did not resolve is never compiled");
    const span = e.span;
    if (e.form === "is") {
      return (r) => {
        const v = x(r);
        const base = r.depth;
        const f = conformWalk(v, t, r.walkCtx());
        r.depth = base;
        return f === null;
      };
    }
    return (r) => {
      const v = x(r);
      const base = r.depth;
      const f = conformWalk(v, t, r.walkCtx());
      r.depth = base;
      if (f === null) return v;
      const failure = new RunFailure("not-conforming", span, f.template, f.params);
      failure.path = f.path;
      throw failure;
    };
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
      let i = this.varIndex.get(name);
      if (i === undefined) {
        i = this.varIndex.size;
        this.varIndex.set(name, i);
      }
      const index = i;
      return (r) => r.variable(index, name, span);
    }
    throw new Error("a lambda name is not a value");
  }

  /** An operand for an expression whose code is `code`. */
  private operand(e: A.Expr, code: Code): Operand {
    const o = codeOperand(code);
    switch (e.kind) {
      case "Number":
      case "String":
      case "Bool":
        o.k = 1;
        o.v = e.value;
        return o;
      case "Null":
        o.k = 1;
        return o;
      case "Ident": {
        const ref = this.res.refs.get(e);
        if (ref !== undefined && ref.k === "slot") {
          o.k = 2;
          o.s = ref.slot;
        } else if (ref !== undefined && ref.k === "var") {
          o.k = 3;
          o.s = this.varSlot(ref.name);
          o.name = ref.name;
          o.span = e.span;
        }
        return o;
      }
      default:
        return o;
    }
  }

  /** An operand for an expression. */
  private operandOf(e: A.Expr): Operand {
    return this.operand(e, this.expr(e));
  }

  private varSlot(name: string): number {
    let i = this.varIndex.get(name);
    if (i === undefined) {
      i = this.varIndex.size;
      this.varIndex.set(name, i);
    }
    return i;
  }

  private template(e: A.Template): Code {
    const parts = e.parts.map((p) => (typeof p === "string" ? p : this.operandOf(p)));
    const span = e.span;
    return (r) => {
      const out = new Rendering();
      for (let i = 0; i < parts.length; i++) {
        const p = parts[i];
        if (typeof p === "string") out.add(p);
        else {
          const v = ev(r, p);
          if (typeof v === "string") out.add(v);
          else render(r, v, span, out);
        }
      }
      const sz = div64(out.length);
      r.charge(1 + sz);
      r.alloc(1 + sz);
      return out.text();
    };
  }

  private list(e: A.List): Code {
    const items = e.items.map((it) =>
      it.kind === "Spread" ? { spread: true, code: this.expr(it.value), span: it.span } : { spread: false, code: this.expr(it), span: it.span },
    );
    if (!items.some((it) => it.spread)) {
      return (r) => {
        const out: unknown[] = new Array(items.length);
        for (let i = 0; i < items.length; i++) out[i] = items[i].code(r);
        r.alloc(1 + out.length);
        return out;
      };
    }
    // With a spread, the list's length is found before the list is built (02 6.2 item 3).
    return (r) => {
      const vals: unknown[] = new Array(items.length); // a spread's value is its list
      let n = 0;
      for (let i = 0; i < items.length; i++) {
        const it = items[i];
        const v = (vals[i] = it.code(r));
        if (!it.spread) {
          n++;
          continue;
        }
        const k = r.kind(v);
        if (k === "null") throw new RunFailure("null-operand", it.span, "spread");
        if (k !== "list") throw new RunFailure("wrong-kind", it.span, "spread-list", { kind: k });
        r.charge((v as unknown[]).length);
        n += (v as unknown[]).length;
      }
      r.alloc(1 + n);
      const out: unknown[] = new Array(n);
      let j = 0;
      for (let i = 0; i < items.length; i++) {
        if (!items[i].spread) out[j++] = vals[i];
        else for (const x of vals[i] as unknown[]) out[j++] = x;
      }
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
    // An entry evaluated: a key and its value, or a spread's map and its keys.
    type Written = { key: string; v: unknown } | { keys: string[]; m: object };
    return (r) => {
      const ws: Written[] = [];
      let k = 0; // the keys written, at least the map's entries
      for (const it of items) {
        if (it.t === "key") {
          const v = it.code(r);
          r.charge(it.keySize);
          ws.push({ key: it.key, v });
          k++;
        } else if (it.t === "computed") {
          const key = it.key(r);
          const kk = r.kind(key);
          if (kk === "null") throw new RunFailure("null-operand", it.span, "key");
          if (kk !== "string") throw new RunFailure("wrong-kind", it.span, "key", { found: key });
          const v = it.code(r);
          r.charge(size(key as string));
          ws.push({ key: key as string, v });
          k++;
        } else {
          const v = it.code(r);
          const kind = r.kind(v);
          if (kind === "null") throw new RunFailure("null-operand", it.span, "spread");
          if (kind !== "map") throw new RunFailure("wrong-kind", it.span, "spread-map", { kind });
          const keys = mapKeys(v as object);
          r.charge(keys.length + sizeTogether(keys));
          ws.push({ keys, m: v as object });
          k += keys.length;
        }
      }
      // The map's units come before the map (02 6.2 item 3). When k keys could
      // pass the limit, the number of entries is found first, without
      // building: the keys written, each counted once. The pass is linear in
      // k, which the spreads have charged as steps.
      let charged = false;
      if (1 + 2 * k > r.unitLimit - r.units) {
        const seen = new Set<string>();
        for (const w of ws) {
          if ("m" in w) for (const key of w.keys) seen.add(key);
          else seen.add(w.key);
        }
        r.alloc(1 + 2 * seen.size);
        charged = true;
      }
      const m = new RMap();
      for (const w of ws) {
        if ("m" in w) for (const key of w.keys) m.set(key, mapGet(w.m, key));
        else m.set(w.key, w.v);
      }
      if (!charged) r.alloc(1 + 2 * m.size);
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
    const first = this.operand(x, this.expr(x));
    if (spine.length <= NEST_MAX) {
      let left = first;
      let code: Code = NO_CODE;
      for (const n of spine) {
        code = binaryOp(n.op, left, this.operandOf(n.y), n.span);
        left = codeOperand(code);
      }
      return code;
    }
    // A longer chain runs in a loop, so it spends no host stack on its length.
    const steps = spine.map((n) => binaryStep(n.op, this.operandOf(n.y), n.span));
    const n = steps.length;
    return (r) => {
      let acc = ev(r, first);
      for (let i = 0; i < n; i++) acc = steps[i](r, acc);
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
    if (tests.length === 1) {
      const test = this.operand(e.test, tests[0]);
      const then = this.operand(e.then, thens[0]);
      const other = this.operand(c, otherwise);
      return (r) => (truthyQ(r, ev(r, test)) ? ev(r, then) : ev(r, other));
    }
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
      const sk = kindQ(r, s);
      for (let i = 0; i < arms.length; i++) {
        const a = arms[i];
        r.charge(1);
        if (a.isDefault) return a.result(r);
        for (let j = 0; j < a.lits.length; j++) {
          const l = a.lits[j];
          if (typeof l === "string" && sk === "string") {
            // 02 5.2, `match`: a string literal against a string subject compares text.
            if (l.length >= 64 && (s as string).length >= 64) r.charge(div64(lcpWeight(l, s as string)));
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
        // f04 5.5: the path of a `not-conforming` failure, 1 + n units.
        const path = failure.path === undefined ? null : [...failure.path];
        r.alloc(9 + (built ? 1 + sz : 0) + (path === null ? 0 : 1 + path.length));
        const ev = new RMap([
          ["code", failure.code],
          ["data", failure.data],
          ["message", message],
          ["path", path],
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
  /** `raw`: an optional step that finds null gives SHORT, for the chain a projection goes on (f07 3.1). */
  private chain(e: A.Expr, raw = false): Code {
    const steps: A.Expr[] = [];
    // Only the outermost node of a chain is an OptChain: an inner one is a
    // chain of its own, written in parentheses (01 7.4 rule 5).
    let x: A.Expr = e.kind === "OptChain" ? e.x : e;
    for (;;) {
      if (x.kind === "Member" && this.res.constants.has(x)) break; // the read of a constant is a primary
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
    let primary: Code;
    if (x.kind === "Each") primary = this.each(x);
    else if (x.kind === "Member" && this.res.constants.has(x)) {
      // f06 5.3: the read of a constant charges nothing.
      const value = this.res.constants.get(x)!;
      primary = () => value;
    } else primary = x.kind === "Call" || x.kind === "MethodCall" ? this.call(x) : this.expr(x);
    if (steps.length === 0 && x.kind !== "Each") return primary;
    if (steps.length <= NEST_MAX) {
      // Each step is a closure around the code before it. `mayShort`: the code so far can give SHORT.
      let code = primary;
      let mayShort = x.kind === "Each";
      let first = true;
      for (const s of steps) {
        code = s.kind === "Member" && !mayShort ? this.memberOn(first ? this.operand(x, primary) : codeOperand(code), s) : this.stepOn(code, this.step(s), mayShort);
        first = false;
        if ((s.kind === "Member" || s.kind === "Index" || s.kind === "MethodCall") && s.optional) mayShort = true;
      }
      if (!mayShort || raw) return code;
      return (r) => {
        const v = code(r);
        return v === SHORT ? null : v;
      };
    }
    // A longer chain runs in a loop, so it spends no host stack on its length.
    const codes = steps.map((s) => this.step(s));
    const short = raw ? SHORT : null;
    const first = this.operand(x, primary);
    const n = codes.length;
    return (r) => {
      let v = ev(r, first);
      if (v === SHORT) return short;
      for (let i = 0; i < n; i++) {
        v = codes[i](r, v);
        if (v === SHORT) return short;
      }
      return v;
    };
  }

  /** A step after the code before it. */
  private stepOn(prev: Code, step: (r: Run, v: unknown) => unknown, mayShort: boolean): Code {
    if (!mayShort) return (r) => step(r, prev(r));
    return (r) => {
      const v = prev(r);
      return v === SHORT ? SHORT : step(r, v);
    };
  }

  /** A field step on a value quickMap does not class. */
  private memberSlow(s: A.Member): (r: Run, v: unknown) => unknown {
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

  /** A field step `.name` after an operand that never gives SHORT, with the fast path for a map. */
  private memberOn(prev: Operand, s: A.Member): Code {
    const { name, optional, span } = s;
    const cost = 1 + size(name);
    const slow = this.memberSlow(s);
    return (r) => {
      const v = ev(r, prev);
      const q = quickMap(r, v);
      if (q === 0) return slow(r, v);
      r.charge(cost);
      const g = quickGet(v as object, q, name);
      if (g === ABSENT) {
        if (optional) return null;
        throw new RunFailure("missing-field", span, "field", { name });
      }
      return readQ(r, g, span);
    };
  }

  private step(s: A.Expr): (r: Run, v: unknown) => unknown {
    if (s.kind === "Member") {
      const { name, optional, span } = s;
      const cost = 1 + size(name);
      const slow = this.memberSlow(s);
      return (r, v) => {
        const q = quickMap(r, v);
        if (q === 0) return slow(r, v);
        r.charge(cost);
        const g = quickGet(v as object, q, name);
        if (g === ABSENT) {
          if (optional) return null;
          throw new RunFailure("missing-field", span, "field", { name });
        }
        return readQ(r, g, span);
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
   * one written in place charges its 1 step where it stands. `lambdas`: the
   * positions of the lambda arguments.
   */
  private args(args: A.Arg[], measure: number): { eval: (r: Run) => unknown[]; lambdas: number[] } {
    const codes = args.map((a) => {
      const la = this.lambdaArg(a);
      if (la === null) return { kind: "value" as const, op: this.operandOf(a as A.Expr), lc: null, id: null, inPlace: false, params: 0 };
      const lc = la.lambda !== null ? this.lambda(la.lambda) : null;
      return { kind: "lambda" as const, op: null, lc, id: la.id, inPlace: la.lambda !== null, params: la.params };
    });
    const lambdas: number[] = [];
    codes.forEach((c, i) => {
      if (c.kind === "lambda") lambdas.push(i);
    });
    const codesOf = this.codes;
    const named = this.res.lambdas;
    const n = codes.length;
    if (lambdas.length === 0) {
      const vs = codes.map((c) => c.op!);
      switch (n) {
        case 0:
          return { eval: () => [], lambdas };
        case 1: {
          const [a] = vs;
          return { eval: (r) => [ev(r, a)], lambdas };
        }
        case 2: {
          const [a, b] = vs;
          return {
            eval: (r) => {
              const x = ev(r, a);
              return [x, ev(r, b)];
            },
            lambdas,
          };
        }
      }
    }
    const evalArgs = (r: Run): unknown[] => {
      const out = new Array<unknown>(n);
      for (let i = 0; i < n; i++) {
        const c = codes[i];
        if (c.kind === "value") {
          out[i] = ev(r, c.op!);
          continue;
        }
        if (c.inPlace) r.charge(1);
        out[i] = new LambdaCallback(r, c.lc ?? codesOf.get(named[c.id!])!, c.params, measure);
      }
      return out;
    };
    return { eval: evalArgs, lambdas };
  }

  private methodStep(s: A.MethodCall): (r: Run, v: unknown) => unknown {
    const { name, optional, span } = s;
    const nargs = s.args.length;
    const shapes = s.args.map((a) => this.lambdaArg(a));
    const { eval: args, lambdas } = this.args(s.args, this.measure(s.level));
    const env = this.env;
    // A removed method is absent at run time too (03 3.3.1 item 6).
    const removed = this.env.removed;
    const gone = (kind: string) => removed !== undefined && removed.has(`${kind}.${name}`);
    for (const kind of ["string", "list", "map"]) {
      const e = gone(kind) ? undefined : libraryMethod(kind as "string", name);
      if (e !== undefined && e.impl === null) this.missing.add(`${kind}.${name}`);
    }
    // The shape test (02 2.3, 04 2.2) of a receiver of the library's kinds is fixed by the call:
    // the entry, or the failure the test gives.
    type Plan = { entry: Entry; fail: null } | { entry: null; fail: () => RunFailure };
    const plan = (k: Kind): Plan => {
      const entry = k === "string" || k === "list" || k === "map" ? (gone(k) ? undefined : libraryMethod(k, name)) : undefined;
      if (entry === undefined) return { entry: null, fail: () => new RunFailure("wrong-kind", span, "method", { kind: k, name }) };
      const counts = entry.counts;
      if (nargs < counts.min || nargs > counts.max) return { entry: null, fail: () => new RunFailure("wrong-kind", span, "count", { count: nargs, name }) };
      for (let i = 0; i < nargs; i++) {
        const fn = paramAt(entry, i).fn;
        if (fn !== null && shapes[i] === null) return { entry: null, fail: () => new RunFailure("wrong-kind", span, "callback", { count: i + 1, name }) };
        if (fn === null && shapes[i] !== null) return { entry: null, fail: () => new RunFailure("wrong-kind", span, "not-callback", { count: i + 1, name }) };
      }
      for (let i = 0; i < nargs; i++) {
        const fn = paramAt(entry, i).fn;
        if (fn !== null && shapes[i]!.params > fn) return { entry: null, fail: () => new RunFailure("wrong-kind", span, "parameters", { name, count: fn }) };
      }
      return { entry, fail: null };
    };
    const plans = new Map<Kind, Plan>();
    for (const k of ["bool", "number", "string", "list", "map"] as const) plans.set(k, plan(k));
    const listPlan = plans.get("list")!;
    const stringPlan = plans.get("string")!;
    return (r, v) => {
      const k = kindQ(r, v);
      if (k === "null") {
        if (optional) return SHORT;
        throw new RunFailure("null-operand", span, "method", { name });
      }
      if (k === "opaque") {
        const t = r.cls.opaqueName(v);
        const host = t === null ? undefined : env.methods.get(t)?.get(name);
        if (host === undefined) throw new RunFailure("wrong-kind", span, "method", { kind: k, name });
        const counts = host.counts;
        if (nargs < counts.min || nargs > counts.max) throw new RunFailure("wrong-kind", span, "count", { count: nargs, name });
        for (let i = 0; i < nargs; i++) {
          if (shapes[i] !== null) throw new RunFailure("wrong-kind", span, "not-callback", { count: i + 1, name });
        }
        const vals = args(r);
        return callHost(r, host, [v, ...vals], name, span);
      }
      const p = k === "list" ? listPlan : k === "string" ? stringPlan : plans.get(k)!;
      if (p.fail !== null) throw p.fail();
      return callEntry(r, p.entry, v, args(r), span, name, lambdas);
    };
  }

  /** A call by name, or a call of a namespace (`MethodCall` resolved as a `Call`). */
  private call(node: A.Call | A.MethodCall): Code {
    const target: Target = this.res.targets.get(node) ?? { k: "unknown" };
    const span = node.span;
    const measure = this.measure(node.level);
    const { eval: args, lambdas } = this.args(node.args, measure);
    switch (target.k) {
      case "lambda": {
        const node = this.res.lambdas[target.id];
        const codesOf = this.codes;
        const name = this.res.lambdaNames?.get(target.id) ?? "";
        return (r) => {
          const vals = args(r);
          r.charge(1);
          // f08 4.1: a call by name is in progress from its depth test to its end.
          return invoke(r, codesOf.get(node)!, vals, measure, span, name, null);
        };
      }
      case "library": {
        const entry = libraryFunction(target.name)!;
        if (entry.impl === null) this.missing.add(target.name);
        const name = target.name;
        return (r) => callEntry(r, entry, undefined, args(r), span, name, lambdas);
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

/** Bind one parameter of a lambda: its default first when the argument is null (02 11.2). */
function bindParam(r: Run, p: ParamCode, v: unknown): void {
  if (v === null && p.dflt !== null) v = p.dflt(r);
  if (p.slot >= 0) r.slots[p.slot] = v;
  else if (p.slot === PATTERN) p.bind(r, v);
}

/** Call a const-bound lambda (02 11.2) or a lambda passed to the library (5.6). */
function invoke(r: Run, lc: LambdaCode, args: unknown[], measure: number, span?: Span, name?: string, index?: number | null): unknown {
  r.depth += measure;
  try {
    r.testDepth(r.depth);
    const ps = lc.params;
    for (let i = 0; i < ps.length; i++) bindParam(r, ps[i], i < args.length ? args[i] : null);
    const out = ev(r, lc.bodyOp);
    r.depth -= measure;
    return out;
  } catch (x) {
    // f08 4.1: the trace is built as the failure unwinds, from the outermost in.
    if (span !== undefined && x instanceof RunFailure) x.trace.unshift({ span, name: name!, index: index ?? null });
    throw x;
  }
}

/** `invoke` with the arguments (a, b, c): what `invoke(r, lc, [a, b, c], ...)` does. */
function invoke3(r: Run, lc: LambdaCode, a: unknown, b: unknown, c: unknown, measure: number, span?: Span, name?: string, index?: number | null): unknown {
  r.depth += measure;
  try {
    r.testDepth(r.depth);
    const ps = lc.params;
    const n = ps.length;
    if (n > 0) bindParam(r, ps[0], a);
    if (n > 1) bindParam(r, ps[1], b);
    if (n > 2) bindParam(r, ps[2], c);
    for (let i = 3; i < n; i++) bindParam(r, ps[i], null);
    const out = ev(r, lc.bodyOp);
    r.depth -= measure;
    return out;
  } catch (x) {
    if (span !== undefined && x instanceof RunFailure) x.trace.unshift({ span, name: name!, index: index ?? null });
    throw x;
  }
}

/** A lambda passed to the library or to a host function. */
class LambdaCallback implements Callback {
  readonly params: number;
  trace: { span: Span; name: string; count: number } | undefined = undefined;
  private readonly r: Run;
  private readonly lc: LambdaCode;
  private readonly measure: number;

  constructor(r: Run, lc: LambdaCode, params: number, measure: number) {
    this.r = r;
    this.lc = lc;
    this.params = params;
    this.measure = measure;
  }

  call(args: unknown[]): unknown {
    // f08 4.1: the library call is in progress while this call is; its index is the call's position.
    const tr = this.trace;
    return tr === undefined ? invoke(this.r, this.lc, args, this.measure) : invoke(this.r, this.lc, args, this.measure, tr.span, tr.name, tr.count++);
  }

  /** call([a, b]) */
  call2(a: unknown, b: unknown): unknown {
    return this.call3(a, b, null);
  }

  /** call([a, b, c]) */
  call3(a: unknown, b: unknown, c: unknown): unknown {
    const r = this.r;
    const lc = this.lc;
    if (!lc.simple) {
      const tr = this.trace;
      return tr === undefined ? invoke3(r, lc, a, b, c, this.measure) : invoke3(r, lc, a, b, c, this.measure, tr.span, tr.name, tr.count++);
    }
    // What invoke3 does, for parameters that are names with no default.
    const tr = this.trace;
    const index = tr === undefined ? 0 : tr.count++;
    const measure = this.measure;
    r.depth += measure;
    try {
      if (r.depth > r.depthLimit) r.testDepth(r.depth);
      const slots = r.slots;
      if (lc.s0 >= 0) slots[lc.s0] = a;
      if (lc.s1 >= 0) slots[lc.s1] = b;
      if (lc.s2 >= 0) slots[lc.s2] = c;
      const out = ev(r, lc.bodyOp);
      r.depth -= measure;
      return out;
    } catch (x) {
      if (tr !== undefined && x instanceof RunFailure) x.trace.unshift({ span: tr.span, name: tr.name, index });
      throw x;
    }
  }
}

/** A call of a library entry, after its receiver and arguments (04 2.2, 2.6, 4.4). `lambdas`: the positions of its lambda arguments. */
function callEntry(r: Run, entry: Entry, receiver: unknown, args: unknown[], span: Span, name: string, lambdas: number[]): unknown {
  testArgs(r, entry, args, span, name);
  r.charge(1);
  if (entry.impl === null) throw new RunFailure("internal-error", span, "default");
  // f08 4.1, 4.2: the entry is in progress while a call of its lambda is; the index is the element's position, in visit order.
  for (let i = 0; i < lambdas.length; i++) (args[lambdas[i]] as LambdaCallback).trace = { span, name, count: 0 };
  return entry.impl({ run: r, span, name, receiver, args });
}

/** A call of a host function or of a method of an opaque type (02 11.4, 03 3.5). */
function callHost(r: Run, host: HostImpl, args: unknown[], name: string, span: Span): unknown {
  r.charge(1);
  // The handle is valid until the function returns (03 3.5.2 item 6).
  // After that, charge and allocate throw a host-side Error and charge
  // nothing, and the signal reads as cancelled.
  let open = true;
  const late = () => new Error("the call handle was used after its host function returned");
  const handle: CallHandle = {
    get signal() {
      return open ? r.signal : AbortSignal.abort();
    },
    charge: (n) => {
      if (!open) throw late();
      r.charge(n);
    },
    allocate: (n) => {
      if (!open) throw late();
      r.alloc(n);
    },
  };
  let out: unknown;
  try {
    out = host.call(handle, args);
  } catch (x) {
    open = false;
    if (x instanceof RunFailure) throw x;
    if (x instanceof Error) throw new RunFailure("function-error", span, null, {}, { text: x.message, cause: x });
    throw new RunFailure("function-panic", span, "default", { name });
  }
  open = false;
  r.checkTime();
  if (out instanceof Promise) throw new RunFailure("function-error", span, null, {}, { text: "a host function returned a Promise" });
  return r.read(out, span);
}

