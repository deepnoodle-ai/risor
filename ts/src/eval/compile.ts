// The compiler: a syntax tree, resolved by the static rules, becomes a tree
// of closures (no bytecode, no virtual machine). The closures evaluate in the
// order of 02 section 2 and charge by sections 5 and 6.
//
// Names (f17 11.5). Each call binds its own names: a call of a lambda makes a
// frame, an array that holds the names its lambda binds (its parameters, and
// the names of the `const` lines and `catch` of its body) at index 1 and up,
// and at index 0 the frame of the call in which the lambda was evaluated. A
// name is compiled to the number of links to follow from the frame of the call
// in progress and an index. A lambda never outlives the call in which it was
// evaluated (02 11.1), so a frame is only reached while its call is in
// progress. The program's own names are bound once in a run, and are kept in
// the run's slots, which every call reaches with no link. The calls a library
// entry makes of one lambda never overlap, so they share one frame, made with
// the lambda's callback. There is one path for every program: with no
// self-reference, the frames hold what fixed slots would. A function
// parameter (f17 13.4) holds a BoundLambda, a lambda with the frame its calls
// link to, so a lambda given to a call reads the names of the call where it
// was evaluated, whichever call of whichever lambda calls it.
//
// Long flat forms (a chain of binary operators, a chain of steps, an `else`
// chain, `const` lines) are compiled and run by loops, so no host stack is
// spent on their length.
//
// Stack. A call costs the depth count 1 plus its nesting depth (02 8.4), and
// what opens no level costs nothing, so the host stack that one unit of the
// count may take is set by what runs between a lambda's body and a call in
// it at nesting depth 0. That is kept to a few frames (see src/index.ts,
// STACK): a chain of operands that open no level is evaluated in one
// closure, a spine (Compiler.spine), which runs the method calls in it in
// place (a short chain that calls nothing at nesting depth 0 is nested
// closures, NEST_MAX); a library entry calls a lambda from its own frame
// (LambdaCallback); code in an operand is called in place, not through `ev`;
// a call by name runs the body from its own closure.
//
// Speed. The closures of one kind share one function literal, so the engine
// sees every site of a kind at once: calls between closures are indirect.
// The code is shaped by that. Constants, names and variables are read in
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
import type { CallContext, Callback, Entry, Impl } from "../library/registry.ts";
import { paramAt, testArgs } from "../library/args.ts";
import { RisorError } from "../host/error.ts";

export type Code = (r: Run) => unknown;
type Binder = (r: Run, v: unknown) => void;
/** A step of a spine (Compiler.spine): applied to the value so far. */
type Step = (r: Run, v: unknown) => unknown;

/**
 * A step of a spine: `f`, or the method call `m`, which the spine runs in
 * place. `end`: the index after the last step of the step's chain, where a
 * SHORT from it goes on, with null.
 */
interface SpineStep {
  f: Step | null;
  m: MethodCode | null;
  end: number;
}

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
 * A parameter of a lambda. `slot`: the index in the lambda's frame of a
 * parameter that is a name (NAME_UNUSED when the name is never read), or
 * PATTERN when `bind` binds it.
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
  n: number;
  /**
   * At most three parameters, each a name with no default: the frame indexes
   * of the first three (NAME_UNUSED for a name never read or a parameter not
   * there).
   */
  simple: boolean;
  s0: number;
  s1: number;
  s2: number;
  /** The size of the lambda's frame, its link included (f17 11.5). */
  size: number;
  /** The number of lambdas around it: the depth of the frame its frame links to, 0 for the program. */
  depth: number;
}

/** A new frame of `size` entries for a call, linked to `link` (f17 11.5). */
function newFrame(size: number, link: unknown): unknown[] {
  switch (size) {
    case 1:
      return [link];
    case 2:
      return [link, null];
    case 3:
      return [link, null, null];
    case 4:
      return [link, null, null, null];
    case 5:
      return [link, null, null, null, null];
    default: {
      const f: unknown[] = [link, null, null, null, null];
      for (let i = 5; i < size; i++) f.push(null);
      return f;
    }
  }
}

/** The frame `hops` links out from `f`. */
function up(f: unknown[], hops: number): unknown[] {
  for (let i = 0; i < hops; i++) f = f[0] as unknown[];
  return f;
}

/**
 * Store a value at a place (f17 11.5): a slot of the program (place >= 0),
 * or an index of the frame of the call in progress (place < 0, negated).
 */
function store(r: Run, place: number, v: unknown): void {
  if (place >= 0) r.slots[place] = v;
  else r.frame[-place] = v;
}

const SHORT: unique symbol = Symbol("short");

/**
 * A path of field reads from a constant, a name or a variable up to this long,
 * then a chain of binary operators up to this long, is compiled into nested
 * closures, one for each read or operator, which is faster than a spine. A
 * longer one is a spine, which runs in a loop.
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
 * An operand: compiled code (k 0), a constant (1), a slot of the program (2),
 * a variable of the environment (3), a name of the frame of the call in
 * progress (4), of the frame one link out from it (5), or of a frame `h`
 * links out (6). A constant, a name or a variable is read in place, with no
 * call of a closure.
 */
interface Operand {
  k: 0 | 1 | 2 | 3 | 4 | 5 | 6;
  c: Code;
  v: unknown;
  /** The slot, the index in the frame, or the index of the variable. */
  s: number;
  /** The links to follow to the frame (k 5 and 6). */
  h: number;
  /** The variable's name and span. */
  name: string;
  span: Span | null;
}

function codeOperand(c: Code): Operand {
  return { k: 0, c, v: null, s: 0, h: 0, name: "", span: null };
}

/**
 * The value of an operand.
 *
 * STACK. Where an operand may be code, the code is called in place, as
 * `o.k === 0 ? o.c(r) : ev(r, o)`, so that no frame of `ev` sits under it
 * while it runs: a call it makes, and so a recursion, would spend that frame
 * at every level (src/index.ts).
 *
 * SPEED. `ev` is inlined into the closures of the operators, and its size
 * counts against what else the engine inlines there (the operator's own
 * function, `charge`): a name of an outer frame, which is rarer, is read by a
 * function of its own (measured with scripts/bench.ts: with it in `ev`, the
 * engine stopped inlining `charge` and the operators into the closures, and
 * `heavy/arith_10k` ran 10% to 25% slower).
 */
function ev(r: Run, o: Operand): unknown {
  switch (o.k) {
    case 0:
      return o.c(r);
    case 1:
      return o.v;
    case 2:
      return r.slots[o.s];
    case 4:
      return r.frame[o.s];
    case 3:
      return r.variable(o.s, o.name, o.span!);
    default:
      return outerName(r, o);
  }
}

/** A name of a frame `o.h` links out from the frame of the call in progress (k 5 and 6). */
function outerName(r: Run, o: Operand): unknown {
  let f = r.frame[0] as unknown[];
  for (let i = 1; i < o.h; i++) f = f[0] as unknown[];
  return f[o.s];
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
        const v = a.k === 0 ? a.c(r) : ev(r, a);
        return truthyQ(r, v) ? (b.k === 0 ? b.c(r) : ev(r, b)) : v;
      };
    case "||":
      return (r) => {
        const v = a.k === 0 ? a.c(r) : ev(r, a);
        return truthyQ(r, v) ? v : b.k === 0 ? b.c(r) : ev(r, b);
      };
    case "??":
      return (r) => {
        const v = a.k === 0 ? a.c(r) : ev(r, a);
        return v === null || v === undefined ? (b.k === 0 ? b.c(r) : ev(r, b)) : v;
      };
    case "==":
      return (r) => opEq(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "!=":
      return (r) => !opEq(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "<":
      return (r) => opLt(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "<=":
      return (r) => opLe(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case ">":
      return (r) => opGt(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case ">=":
      return (r) => opGe(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "in":
      return (r) => contains(r, "in", a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "not-in":
      return (r) => !contains(r, "not in", a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "+":
      return (r) => opAdd(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "-":
      return (r) => opSub(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "*":
      return (r) => opMul(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "/":
      return (r) => opDiv(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    case "%":
      return (r) => opRem(r, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
    default:
      return (r) => arith(r, op as ArithOp, a.k === 0 ? a.c(r) : ev(r, a), b.k === 0 ? b.c(r) : ev(r, b), span);
  }
}

/** A binary operator whose left operand is the value so far of a chain of operators. */
function binaryStep(op: A.Binary["op"], b: Operand, span: Span): (r: Run, x: unknown) => unknown {
  switch (op) {
    case "&&":
      return (r, x) => (truthyQ(r, x) ? (b.k === 0 ? b.c(r) : ev(r, b)) : x);
    case "||":
      return (r, x) => (truthyQ(r, x) ? x : b.k === 0 ? b.c(r) : ev(r, b));
    case "??":
      return (r, x) => (x === null || x === undefined ? (b.k === 0 ? b.c(r) : ev(r, b)) : x);
    case "==":
      return (r, x) => opEq(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "!=":
      return (r, x) => !opEq(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "<":
      return (r, x) => opLt(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "<=":
      return (r, x) => opLe(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case ">":
      return (r, x) => opGt(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case ">=":
      return (r, x) => opGe(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "in":
      return (r, x) => contains(r, "in", x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "not-in":
      return (r, x) => !contains(r, "not in", x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "+":
      return (r, x) => opAdd(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "-":
      return (r, x) => opSub(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "*":
      return (r, x) => opMul(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "/":
      return (r, x) => opDiv(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    case "%":
      return (r, x) => opRem(r, x, b.k === 0 ? b.c(r) : ev(r, b), span);
    default:
      return (r, x) => arith(r, op as ArithOp, x, b.k === 0 ? b.c(r) : ev(r, b), span);
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
  /** The number of lambdas around the code being compiled: the depth of its frame (f17 11.5). */
  private frameDepth = 0;
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

  /**
   * Where the name of slot `s` is stored, seen from the code being compiled
   * (f17 11.5): a slot of the program (>= 0), or an index of the frame of the
   * call in progress (< 0, negated). A name is bound only by code of its own
   * frame.
   */
  private place(s: number): number {
    const depth = this.res.slotDepth[s];
    if (depth === 0) return this.res.slotIndex[s];
    if (depth !== this.frameDepth) throw new Error("a name is bound only in its own frame");
    return -this.res.slotIndex[s];
  }

  /** The index in the frame of the call in progress of slot `s`, a name its lambda binds. */
  private frameIndex(s: number): number {
    const p = this.place(s);
    if (p >= 0) throw new Error("a parameter is a name of its lambda's frame");
    return -p;
  }

  /** How code at this frame depth reads the name of slot `s`: as an operand (k 2, 4, 5 or 6). */
  private nameOperand(s: number, o: Operand): void {
    const depth = this.res.slotDepth[s];
    if (depth === 0) {
      o.k = 2;
      o.s = this.res.slotIndex[s];
      return;
    }
    o.s = this.res.slotIndex[s];
    o.h = this.frameDepth - depth;
    o.k = o.h === 0 ? 4 : o.h === 1 ? 5 : 6;
  }

  /** The frame a lambda `l` links to, from the code being compiled: null for a lambda of the program. */
  private linkTo(l: A.Lambda): (r: Run) => unknown {
    const depth = this.res.lambdaDepth.get(l)!;
    if (depth === 0) return () => null;
    const hops = this.frameDepth - depth;
    if (hops === 0) return (r) => r.frame;
    if (hops === 1) return (r) => r.frame[0];
    return (r) => up(r.frame, hops);
  }

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
      const w = this.place(s);
      if (w < 0) {
        const i = -w;
        return (r) => {
          const x = v(r);
          r.frame[i] = x;
          return null;
        };
      }
      return (r) => {
        r.slots[w] = v(r);
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
    const savedDepth = this.frameDepth;
    this.lambdaLevel = l.level;
    // f17 11.5: its parameters, defaults and body run in the frame of its call.
    const depth = this.res.lambdaDepth.get(l)!;
    this.frameDepth = depth + 1;
    const params = l.params.map((p): ParamCode => {
      const dflt = p.default === null ? null : this.expr(p.default);
      const bind = this.pattern(p.pattern);
      const s = p.pattern.kind === "NamePat" ? this.res.slots.get(p.pattern) : undefined;
      const slot = p.pattern.kind === "NamePat" ? (s === undefined ? NAME_UNUSED : this.frameIndex(s)) : PATTERN;
      return { dflt, bind, slot };
    });
    const body = this.expr(l.body);
    this.lambdaLevel = saved;
    this.frameDepth = savedDepth;
    const simple = params.length <= 3 && params.every((p) => p.slot !== PATTERN && p.dflt === null);
    const slotAt = (i: number) => (simple && i < params.length ? params[i].slot : NAME_UNUSED);
    const size = this.res.frameSize.get(l)!;
    const lc: LambdaCode = { params, body, n: l.params.length, simple, s0: slotAt(0), s1: slotAt(1), s2: slotAt(2), size, depth };
    this.codes.set(l, lc);
    return lc;
  }

  private pattern(p: A.Pattern): Binder {
    switch (p.kind) {
      case "NamePat": {
        const s = this.res.slots.get(p);
        if (s === undefined) return () => {};
        const w = this.place(s);
        return (r, v) => {
          store(r, w, v);
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
    const placeOf = (s: number | undefined) => (s === undefined ? undefined : this.place(s));
    const fields = p.fields.map((f) => ({
      key: f.key,
      keySize: size(f.key),
      slot: placeOf(this.res.slots.get(f)),
      dflt: f.default === null ? null : this.expr(f.default),
      span: f.span,
    }));
    const named = new Set(p.fields.map((f) => f.key));
    const restSlot = p.rest === null ? undefined : placeOf(this.res.slots.get(p.rest));
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
        if (f.slot !== undefined) store(r, f.slot, val);
      }
      if (hasRest) {
        const keys = mapKeys(v as object).filter((key) => !named.has(key));
        r.charge(keys.length + sizeTogether(keys));
        r.alloc(1 + 2 * keys.length);
        const m = new RMap();
        for (const key of keys) m.set(key, mapGet(v as object, key));
        if (restSlot !== undefined) store(r, restSlot, m);
      }
    };
  }

  private listPattern(p: A.ListPat): Binder {
    const span = p.span;
    const placeOf = (s: number | undefined) => (s === undefined ? undefined : this.place(s));
    const elems = p.elems.map((e) => ({
      slot: placeOf(this.res.slots.get(e.pat)),
      dflt: e.default === null ? null : this.expr(e.default),
      span: e.span,
    }));
    const restSlot = p.rest === null ? undefined : placeOf(this.res.slots.get(p.rest));
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
        if (e.slot !== undefined) store(r, e.slot, val);
      }
      if (hasRest) {
        const m = Math.max(0, xs.length - elems.length);
        r.charge(m);
        r.alloc(1 + m);
        const out = xs.slice(elems.length);
        if (restSlot !== undefined) store(r, restSlot, out);
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
      case "As":
      case "OptChain":
      case "Member":
      case "Index":
      case "MethodCall":
      case "Call":
      case "Each":
        return this.spine(e);
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
      case "Bad":
        throw new Error("a Bad node is never compiled");
    }
  }

  /**
   * A projection (package F7, f07 3, 4), as a step of a spine applied to its
   * receiver: level by level, element by element, into one list whose units
   * are charged once at the end. SHORT: a first star `?.[*]` found null, which
   * makes the chain null (f07 3.1 item 2).
   */
  private project(e: A.Each): Step {
    type Level = { optional: boolean; starSpan: Span; reads: Step[]; next: Level | null };
    const level = (p: A.Each): Level => {
      const reads: Step[] = [];
      let next: Level | null = null;
      for (const s of p.reads) {
        if (s.kind === "Each") next = level(s);
        else reads.push(this.step(s));
      }
      return { optional: p.optional, starSpan: p.starSpan, reads, next };
    };
    const first = level(e);
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
    return (r, recv) => {
      const out: unknown[] = [];
      if (!visit(r, recv, first, out, true)) return SHORT;
      r.alloc(1 + out.length);
      return out;
    };
  }

  /** `as T`, `is T` (package F4, f04 5.1, 5.2), as a step of a spine applied to the value tested. */
  private asStep(e: A.As): Step {
    const t = this.res.asTypes.get(e);
    if (t === undefined || t === null) throw new Error("a program type that did not resolve is never compiled");
    const span = e.span;
    if (e.form === "is") {
      return (r, v) => {
        const base = r.depth;
        const f = conformWalk(v, t, r.walkCtx());
        r.depth = base;
        return f === null;
      };
    }
    return (r, v) => {
      const base = r.depth;
      const f = conformWalk(v, t, r.walkCtx());
      r.depth = base;
      if (f === null) return v;
      const failure = new RunFailure("not-conforming", span, f.template, f.params);
      failure.path = f.path;
      throw failure;
    };
  }

  /** The read of the name of slot `s`: of the program, of the call in progress, or of a call a few links out (f17 11.5). */
  private slotRead(slot: number): Code {
    const o = codeOperand(() => null);
    this.nameOperand(slot, o);
    const s = o.s;
    switch (o.k) {
      case 2:
        return (r) => r.slots[s];
      case 4:
        return (r) => r.frame[s];
      default: {
        const h = o.h;
        if (h === 1) return (r) => (r.frame[0] as unknown[])[s];
        return (r) => up(r.frame, h)[s];
      }
    }
  }

  private ident(e: A.Ident): Code {
    const ref = this.res.refs.get(e);
    if (ref === undefined) throw new Error(`unresolved name ${e.name}`);
    const span = e.span;
    if (ref.k === "slot") return this.slotRead(ref.slot);
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
          this.nameOperand(ref.slot, o);
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
          const v = p.k === 0 ? p.c(r) : ev(r, p);
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
      return (r) =>
        truthyQ(r, test.k === 0 ? test.c(r) : ev(r, test)) ? (then.k === 0 ? then.c(r) : ev(r, then)) : other.k === 0 ? other.c(r) : ev(r, other);
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
      guard: a.guard === null ? null : this.expr(a.guard),
      result: this.expr(a.result),
    }));
    const span = e.span;
    return (r) => {
      const s = subject(r);
      const sk = kindQ(r, s);
      for (let i = 0; i < arms.length; i++) {
        const a = arms[i];
        r.charge(1);
        let hit = a.isDefault;
        for (let j = 0; !hit && j < a.lits.length; j++) {
          const l = a.lits[j];
          if (typeof l === "string" && sk === "string") {
            // 02 5.2, `match`: a string literal against a string subject compares text.
            if (l.length >= 64 && (s as string).length >= 64) r.charge(div64(lcpWeight(l, s as string)));
            hit = l === s;
          } else hit = l === null ? sk === "null" : l === s;
        }
        // Package F17 (f17 3): the guard runs once its arm matched; a falsy guard tries the next arm.
        if (hit && (a.guard === null || truthy(a.guard(r), r.cls))) return a.result(r);
      }
      throw new RunFailure("no-match", span, "default", { found: s });
    };
  }

  private tryExpr(e: A.Try): Code {
    const body = this.expr(e.body);
    const catchSlot = this.res.catchSlots.get(e);
    const slot = catchSlot === undefined ? undefined : this.place(catchSlot);
    const named = e.name !== null;
    const handler = this.expr(e.handler);
    return (r) => {
      const depth = r.depth;
      const frame = r.frame;
      let failure: RunFailure;
      try {
        return body(r);
      } catch (x) {
        if (!(x instanceof RunFailure) || !caught(x.code)) throw x;
        failure = x;
      }
      // 02 8.4 item 8, f17 11.5 item 3: the calls the failure passed through are over.
      r.depth = depth;
      r.frame = frame;
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
        if (slot !== undefined) store(r, slot, ev);
      }
      return handler(r);
    };
  }

  // ---- chains and calls ---------------------------------------------------

  private measure(level: number): number {
    return 1 + (level - this.lambdaLevel);
  }

  /**
   * A spine: an expression whose evaluation starts with an operand that
   * opens no level (01 13.1, f04 2.8, f07 2.5): the left operand of a binary
   * operator, `**`, `as` or `is`, the receiver of a step or a method call, the
   * receiver of a projection. Its core is the innermost such operand. The
   * spine evaluates the core, then applies each step to the value so far, in
   * the order of 02 2.2 and 2.3.
   *
   * A chain is the steps of one chain node (an OptChain, or a step that has
   * none): a step that gives SHORT makes its chain null, and the steps after
   * the chain go on from there (01 7.4 rule 5, f07 3.1 item 2).
   *
   * STACK. The whole spine runs in one closure, and a method call in it is
   * run in place, not by a closure of its own: a call that the core or a step
   * makes runs with this one frame under it, however long the spine.
   */
  private spine(e: A.Expr): Code {
    // The nodes of the spine, outermost first, each with its chain (-1: none).
    const nodes: { n: A.Expr; chain: number }[] = [];
    let chains = 0;
    let x: A.Expr = e;
    peel: for (;;) {
      switch (x.kind) {
        case "Binary":
        case "As":
          nodes.push({ n: x, chain: -1 });
          x = x.x;
          continue;
        case "OptChain":
        case "Member":
        case "Index":
        case "MethodCall":
        case "Each": {
          if (this.isPrimary(x)) break peel;
          // Only the outermost node of a chain is an OptChain: an inner one is a
          // chain of its own, written in parentheses (01 7.4 rule 5).
          const chain = chains++;
          let y: A.Expr = x.kind === "OptChain" ? x.x : x;
          while ((y.kind === "Member" || y.kind === "Index" || y.kind === "MethodCall" || (y.kind === "Each" && y.x !== null)) && !this.isPrimary(y)) {
            nodes.push({ n: y, chain });
            y = y.x!;
          }
          // A projection with no receiver is a later star, which the parser puts
          // only among the reads of another projection (f07 3); reached here, it
          // would make the peel loop for ever.
          if (y.kind === "Each" && y.x === null) throw new Error("a projection with no receiver is never an operand");
          x = y;
          continue;
        }
        default:
          break peel;
      }
    }
    // The core, then the steps in the order they apply.
    let core: Code;
    if (x.kind === "Member" && this.res.constants.has(x)) {
      // f06 5.3: the read of a constant charges nothing.
      const value = this.res.constants.get(x)!;
      core = () => value;
    } else core = x.kind === "Call" || x.kind === "MethodCall" ? this.call(x) : this.expr(x);
    if (nodes.length === 0) return core;
    const first = this.operand(x, core);
    nodes.reverse();
    const n = nodes.length;
    // A short path of field reads from a constant, a slot or a variable
    // (`a.b.c`), then a short chain of binary operators (`a.b * 2 + 1`), is a
    // closure for each, around the one before, which is faster than a loop.
    // With such a core, nothing at nesting depth 0 calls: the reads call
    // nothing, and a call in a right operand is one level deeper, which pays
    // for the frames of the operators under it. A single binary operator is
    // one closure whatever its left operand: one frame, as a spine is.
    let reads = 0;
    let ops = 0;
    if (first.k !== 0) {
      while (reads < n && reads < NEST_MAX && nodes[reads].n.kind === "Member" && !(nodes[reads].n as A.Member).optional) reads++;
      while (reads + ops < n && ops < NEST_MAX && nodes[reads + ops].n.kind === "Binary") ops++;
    }
    if (reads + ops === n || (n === 1 && nodes[0].n.kind === "Binary")) {
      let left = first;
      for (let i = 0; i < reads; i++) left = codeOperand(this.memberOn(left, nodes[i].n as A.Member));
      for (let i = reads; i < n; i++) {
        const b = nodes[i].n as A.Binary;
        left = codeOperand(binaryOp(b.op, left, this.operandOf(b.y), b.span));
      }
      return left.c;
    }
    // The end of each step's chain, in one pass from the last step: the steps
    // of a chain are next to one another, so a step's end is the next step's
    // when both are of one chain, and its own index plus 1 otherwise.
    const ends = new Array<number>(n);
    for (let i = n - 1; i >= 0; i--) {
      const chain = nodes[i].chain;
      ends[i] = chain >= 0 && i + 1 < n && nodes[i + 1].chain === chain ? ends[i + 1] : i + 1;
    }
    const steps: SpineStep[] = nodes.map(({ n: node }, i) => ({
      f: node.kind === "MethodCall" ? null : this.spineStep(node),
      m: node.kind === "MethodCall" ? this.methodCode(node) : null,
      end: ends[i],
    }));
    return (r) => {
      let v = first.k === 0 ? first.c(r) : ev(r, first);
      for (let i = 0; i < n; i++) {
        const s = steps[i];
        const m = s.m;
        if (m === null) v = s.f!(r, v);
        else {
          // A method call (02 2.3, 11.3): the entry runs from this frame.
          const c = m.enter(r, v);
          v = c === null ? SHORT : c.impl(c);
        }
        if (v === SHORT) {
          v = null;
          i = s.end - 1;
        }
      }
      return v;
    };
  }

  /** A node of a chain's kind that is a primary: the read of a constant, a call of a namespace. */
  private isPrimary(x: A.Expr): boolean {
    return (x.kind === "Member" && this.res.constants.has(x)) || (x.kind === "MethodCall" && this.res.targets.has(x));
  }

  /** A step of a spine that is not a method call. */
  private spineStep(n: A.Expr): Step {
    switch (n.kind) {
      case "Member":
      case "Index":
        return this.step(n);
      case "Each":
        return this.project(n);
      case "As":
        return this.asStep(n);
      case "Binary":
        return binaryStep(n.op, this.operandOf(n.y), n.span);
      default:
        throw new Error(`a ${n.kind} is not a step`);
    }
  }

  /** A field step on a value quickMap does not class. */
  private memberSlow(s: A.Member): (r: Run, v: unknown) => unknown {
    const { name, optional, span } = s;
    const nameSize = size(name);
    const absentNull = optional || this.res.coalesced.has(s);
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
        if (absentNull) return null;
        throw new RunFailure("missing-field", span, "field", { name });
      }
      return r.read(g, span);
    };
  }

  /** A field step `.name` after an operand that never gives SHORT, with the fast path for a map. */
  private memberOn(prev: Operand, s: A.Member): Code {
    const { name, span } = s;
    const cost = 1 + size(name);
    const slow = this.memberSlow(s);
    const absentNull = s.optional || this.res.coalesced.has(s);
    if (prev.k === 5 && prev.h === 1) {
      // A field of a name of the frame one link out, `x.id` in `xs.map(x => ys.filter(y => y.id == x.id))`:
      // read in place (SPEED at ev).
      const at = prev.s;
      return (r) => {
        const v = (r.frame[0] as unknown[])[at];
        const q = quickMap(r, v);
        if (q === 0) return slow(r, v);
        r.charge(cost);
        const g = quickGet(v as object, q, name);
        if (g === ABSENT) {
          if (absentNull) return null;
          throw new RunFailure("missing-field", span, "field", { name });
        }
        return readQ(r, g, span);
      };
    }
    return (r) => {
      const v = prev.k === 0 ? prev.c(r) : ev(r, prev);
      const q = quickMap(r, v);
      if (q === 0) return slow(r, v);
      r.charge(cost);
      const g = quickGet(v as object, q, name);
      if (g === ABSENT) {
        if (absentNull) return null;
        throw new RunFailure("missing-field", span, "field", { name });
      }
      return readQ(r, g, span);
    };
  }

  /** A read: a field step or an index step, applied to the value before it. */
  private step(s: A.Expr): Step {
    if (s.kind === "Member") {
      const { name, span } = s;
      const cost = 1 + size(name);
      const slow = this.memberSlow(s);
      // 01 6.6: the last step of a coalesced read gives null for an absent key, as `?.` does.
      const absentNull = s.optional || this.res.coalesced.has(s);
      return (r, v) => {
        const q = quickMap(r, v);
        if (q === 0) return slow(r, v);
        r.charge(cost);
        const g = quickGet(v as object, q, name);
        if (g === ABSENT) {
          if (absentNull) return null;
          throw new RunFailure("missing-field", span, "field", { name });
        }
        return readQ(r, g, span);
      };
    }
    if (s.kind === "Index") {
      const { optional, span } = s;
      const index = this.expr(s.index);
      const absentNull = optional || this.res.coalesced.has(s);
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
            if (absentNull) return null;
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
    throw new Error(`a ${s.kind} is not a read`);
  }

  /**
   * An argument that is a lambda: a Lambda node, or the name of a const-bound
   * lambda; with package F17, the name of a function parameter, `fn` its slot.
   * `params`: its count of parameters, for a function parameter the listed
   * count (f17 13.4 item 1).
   */
  private lambdaArg(a: A.Arg): { lambda: A.Lambda | null; id: number | null; fn: number | null; params: number } | null {
    if (a.kind === "Lambda") return { lambda: a, id: null, fn: null, params: a.params.length };
    if (a.kind === "Ident") {
      const ref = this.res.refs.get(a);
      if (ref !== undefined && ref.k === "lambda") return { lambda: null, id: ref.id, fn: null, params: this.res.lambdas[ref.id].params.length };
      if (ref !== undefined && ref.k === "fn") return { lambda: null, id: null, fn: ref.slot, params: ref.param.fn!.params.length };
    }
    return null;
  }

  /**
   * Compile the arguments of a call. A lambda argument becomes a Callback, or
   * for a call by name (`byName`) the BoundLambda a function parameter takes
   * (f17 13.4); one written in place charges its 1 step where it stands, and
   * a name charges nothing. A function parameter's name passes on the lambda
   * bound to it. `lambdas`: the positions of the lambda arguments.
   */
  private args(args: A.Arg[], measure: number, byName = false): { eval: (r: Run) => unknown[]; lambdas: number[] } {
    const codes = args.map((a) => {
      const la = this.lambdaArg(a);
      if (la === null) return { kind: "value" as const, op: this.operandOf(a as A.Expr), lc: null, id: null, inPlace: false, params: 0, link: null, read: null };
      if (la.fn !== null) return { kind: "lambda" as const, op: null, lc: null, id: null, inPlace: false, params: la.params, link: null, read: this.slotRead(la.fn) };
      const lc = la.lambda !== null ? this.lambda(la.lambda) : null;
      // f17 11.5 item 2: its calls' frames link to the frame of the call in which it was evaluated.
      const link = this.linkTo(la.lambda ?? this.res.lambdas[la.id!]);
      return { kind: "lambda" as const, op: null, lc, id: la.id, inPlace: la.lambda !== null, params: la.params, link, read: null };
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
          return { eval: (r) => [a.k === 0 ? a.c(r) : ev(r, a)], lambdas };
        }
        case 2: {
          const [a, b] = vs;
          return {
            eval: (r) => {
              const x = a.k === 0 ? a.c(r) : ev(r, a);
              return [x, b.k === 0 ? b.c(r) : ev(r, b)];
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
          const op = c.op!;
          out[i] = op.k === 0 ? op.c(r) : ev(r, op);
          continue;
        }
        if (c.read !== null) {
          // f17 13.4: a function parameter given on, which charges nothing where it stands.
          const b = c.read(r) as BoundLambda;
          out[i] = byName ? b : new LambdaCallback(r, b.lc, c.params, measure, b.link);
          continue;
        }
        if (c.inPlace) r.charge(1);
        const lc = c.lc ?? codesOf.get(named[c.id!])!;
        out[i] = byName ? new BoundLambda(lc, c.link!(r)) : new LambdaCallback(r, lc, c.params, measure, c.link!(r));
      }
      return out;
    };
    return { eval: evalArgs, lambdas };
  }

  /** A method call, as a step of a spine, which runs it in place (spine()). */
  private methodCode(s: A.MethodCall): MethodCode {
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
    return new MethodCode(span, name, lambdas, optional, nargs, shapes, args, env.methods, plans);
  }

  /** A call by name, or a call of a namespace (`MethodCall` resolved as a `Call`). */
  private call(node: A.Call | A.MethodCall): Code {
    const target: Target = this.res.targets.get(node) ?? { k: "unknown" };
    const span = node.span;
    const measure = this.measure(node.level);
    // f17 13.4: a lambda argument of a call by name is a lambda bound to a function parameter.
    const { eval: args, lambdas } = this.args(node.args, measure, target.k === "lambda" || target.k === "fn");
    switch (target.k) {
      case "lambda": {
        const node = this.res.lambdas[target.id];
        const codesOf = this.codes;
        const name = this.res.lambdaNames?.get(target.id) ?? "";
        const link = this.linkTo(node);
        // 02 11.2. The body runs from this closure's frame, with no other
        // under it (STACK in src/index.ts). f17 11.5: the call binds its
        // names in a frame of its own, which links to the frame of the call
        // in which the lambda was evaluated; a self-call is such a call.
        return (r) => {
          const vals = args(r);
          r.charge(1);
          const lc = codesOf.get(node)!;
          // f08 4.1: a call by name is in progress from its depth test to its end.
          r.depth += measure;
          try {
            r.testDepth(r.depth);
            const caller = r.frame;
            r.frame = newFrame(lc.size, link(r));
            const ps = lc.params;
            for (let i = 0; i < ps.length; i++) bindParam(r, ps[i], i < vals.length ? vals[i] : null);
            const out = lc.body(r);
            r.frame = caller;
            r.depth -= measure;
            return out;
          } catch (x) {
            // f08 4.1: the trace is built as the failure unwinds, from the outermost in.
            if (x instanceof RunFailure) x.trace.unshift({ span, name, index: null });
            throw x;
          }
        };
      }
      case "fn": {
        // f17 13.4: a call of a function parameter is a call by name of its bound lambda (02 11.2): the
        // arguments, 1 step, the depth test, then the bound lambda's parameters (an extra argument dropped,
        // a missing or null one taking the default) and its body, in a frame linked to the frame of the
        // call in which the lambda was evaluated. Its trace entry is named by the parameter.
        const read = this.slotRead(target.slot);
        const name = target.param.pattern.kind === "NamePat" ? (target.param.pattern.name ?? "") : "";
        return (r) => {
          const vals = args(r);
          r.charge(1);
          const b = read(r) as BoundLambda;
          r.depth += measure;
          try {
            r.testDepth(r.depth);
            const caller = r.frame;
            const lc = b.lc;
            r.frame = newFrame(lc.size, b.link);
            const ps = lc.params;
            for (let i = 0; i < ps.length; i++) bindParam(r, ps[i], i < vals.length ? vals[i] : null);
            const out = lc.body(r);
            r.frame = caller;
            r.depth -= measure;
            return out;
          } catch (x) {
            if (x instanceof RunFailure) x.trace.unshift({ span, name, index: null });
            throw x;
          }
        };
      }
      case "library": {
        const entry = libraryFunction(target.name)!;
        if (entry.impl === null) this.missing.add(target.name);
        const name = target.name;
        const site: EntrySite = { span, name, lambdas };
        return (r) => {
          const c = enterEntry(r, entry, undefined, args(r), site);
          return c.impl(c);
        };
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

/**
 * Package F17, step E3: the lambda bound to a function parameter (f17 13.1,
 * 13.4), held in the parameter's slot: its code, and the frame of the call in
 * which it was evaluated, which the frame of each call of it links to. It is
 * never a value: the static rules let a function parameter's name only be
 * called or given on (f17 13.2).
 */
class BoundLambda {
  readonly lc: LambdaCode;
  readonly link: unknown;

  constructor(lc: LambdaCode, link: unknown) {
    this.lc = lc;
    this.link = link;
  }
}

/** Bind one parameter of a lambda: its default first when the argument is null (02 11.2). */
function bindParam(r: Run, p: ParamCode, v: unknown): void {
  if (v === null && p.dflt !== null) v = p.dflt(r);
  if (p.slot >= 0) r.frame[p.slot] = v;
  else if (p.slot === PATTERN) p.bind(r, v);
}

/** Bind the parameters of a lambda that is not simple to a, b and c, and any more to null (02 11.2). */
function bindArgs(r: Run, lc: LambdaCode, a: unknown, b: unknown, c: unknown): void {
  const ps = lc.params;
  const n = ps.length;
  if (n > 0) bindParam(r, ps[0], a);
  if (n > 1) bindParam(r, ps[1], b);
  if (n > 2) bindParam(r, ps[2], c);
  for (let i = 3; i < n; i++) bindParam(r, ps[i], null);
}

/**
 * A lambda passed to the library (02 5.6, 11.3).
 *
 * STACK. A library entry calls a lambda through call2 or call3, and each runs
 * the body from its own frame: between the entry's frame and the body's there
 * is this one frame and nothing else. So the two are written out in full
 * rather than one calling the other, and binding a parameter that is not a
 * plain name (bindArgs) returns before the body runs. test/deep.test.ts
 * measures what this saves.
 *
 * FRAME (f17 11.5). The entry's calls of the lambda never overlap, so they
 * share one frame, made here, which links to the frame of the call in which
 * the lambda was evaluated. Each call binds the names again. A call makes
 * that frame the one in progress, and when it ends puts back the frame of the
 * code that called the entry, which was in progress when the callback was
 * made: kept here, not on the stack, as STACK asks. A failure that `catch`
 * stops puts back the frame of the `try`.
 */
class LambdaCallback implements Callback {
  readonly params: number;
  trace: { span: Span; name: string; count: number } | undefined = undefined;
  private readonly r: Run;
  private readonly lc: LambdaCode;
  private readonly measure: number;
  private readonly frame: unknown[];
  private readonly caller: unknown[];

  constructor(r: Run, lc: LambdaCode, params: number, measure: number, link: unknown) {
    this.r = r;
    this.lc = lc;
    this.params = params;
    this.measure = measure;
    this.frame = newFrame(lc.size, link);
    this.caller = r.frame;
  }

  /** The call with the arguments (a, b): the depth test, the parameters, the body (02 8.4 item 7, 11.2). */
  call2(a: unknown, b: unknown): unknown {
    const r = this.r;
    const lc = this.lc;
    // f08 4.1, 4.2: the library call is in progress while this call is; its index is the call's position.
    const tr = this.trace;
    const index = tr === undefined ? 0 : tr.count++;
    r.depth += this.measure;
    try {
      if (r.depth > r.depthLimit) r.testDepth(r.depth);
      const fr = this.frame;
      r.frame = fr;
      if (lc.simple) {
        if (lc.s0 >= 0) fr[lc.s0] = a;
        if (lc.s1 >= 0) fr[lc.s1] = b;
        if (lc.s2 >= 0) fr[lc.s2] = null;
      } else this.bind2(a, b);
      const out = lc.body(r);
      r.frame = this.caller;
      r.depth -= this.measure;
      return out;
    } catch (x) {
      if (tr !== undefined) this.traced(x, index);
      throw x;
    }
  }

  /** The call with the arguments (a, b, c), as call2. */
  call3(a: unknown, b: unknown, c: unknown): unknown {
    const r = this.r;
    const lc = this.lc;
    const tr = this.trace;
    const index = tr === undefined ? 0 : tr.count++;
    r.depth += this.measure;
    try {
      if (r.depth > r.depthLimit) r.testDepth(r.depth);
      const fr = this.frame;
      r.frame = fr;
      if (lc.simple) {
        if (lc.s0 >= 0) fr[lc.s0] = a;
        if (lc.s1 >= 0) fr[lc.s1] = b;
        if (lc.s2 >= 0) fr[lc.s2] = c;
      } else bindArgs(r, lc, a, b, c);
      const out = lc.body(r);
      r.frame = this.caller;
      r.depth -= this.measure;
      return out;
    } catch (x) {
      if (tr !== undefined) this.traced(x, index);
      throw x;
    }
  }

  // Each of these takes at most two arguments, so that a call of it from
  // call2 needs no list of registers in call2's frame.

  /** bindArgs with (a, b, null). */
  private bind2(a: unknown, b: unknown): void {
    bindArgs(this.r, this.lc, a, b, null);
  }

  /** f08 4.1: a failure leaving call `index` of the lambda gets the library call's entry in its trace. */
  private traced(x: unknown, index: number): void {
    const tr = this.trace!;
    if (x instanceof RunFailure) x.trace.unshift({ span: tr.span, name: tr.name, index });
  }
}

/** What does not change between the runs of a call of a library entry. `lambdas`: the positions of its lambda arguments. */
interface EntrySite {
  span: Span;
  name: string;
  lambdas: number[];
}

/** A call of a library entry, ready to make: `c.impl(c)`. */
interface EntryCall extends CallContext {
  impl: Impl;
}

/**
 * A call of a library entry, after its receiver and arguments, up to the
 * entry's own work (04 2.2, 2.6, 4.4): the tests of the arguments, the call's
 * step. It returns the call, which the caller makes, so that this frame is
 * not under the entry's while the entry calls a lambda.
 */
function enterEntry(r: Run, entry: Entry, receiver: unknown, args: unknown[], site: EntrySite): EntryCall {
  const { span, name, lambdas } = site;
  testArgs(r, entry, args, span, name, lambdas);
  r.charge(1);
  const impl = entry.impl;
  if (impl === null) throw new RunFailure("internal-error", span, "default");
  // f08 4.1, 4.2: the entry is in progress while a call of its lambda is; the index is the element's position, in visit order.
  for (let i = 0; i < lambdas.length; i++) (args[lambdas[i]] as LambdaCallback).trace = { span, name, count: 0 };
  return { impl, run: r, span, name, receiver, args };
}

/**
 * The shape test of a method call (02 2.3, 04 2.2) for a receiver of one of
 * the library's kinds, which the call fixes: the entry, or the failure.
 */
type Plan = { entry: Entry; fail: null } | { entry: null; fail: () => RunFailure };

/** The "entry" of a call of a method of an opaque type, made already: its receiver is the result. */
const hostResult: Impl = (c) => c.receiver;

/** A method call, a step of a spine (Compiler.spine runs it). */
class MethodCode implements EntrySite {
  readonly span: Span;
  readonly name: string;
  readonly lambdas: number[];
  readonly optional: boolean;
  readonly nargs: number;
  /** For each argument, its shape when it is a lambda, or null. */
  readonly shapes: ({ params: number } | null)[];
  readonly args: (r: Run) => unknown[];
  readonly methods: CompileEnv["methods"];
  readonly plans: Map<Kind, Plan>;
  private readonly list: Plan;
  private readonly string: Plan;

  constructor(
    span: Span,
    name: string,
    lambdas: number[],
    optional: boolean,
    nargs: number,
    shapes: ({ params: number } | null)[],
    args: (r: Run) => unknown[],
    methods: CompileEnv["methods"],
    plans: Map<Kind, Plan>,
  ) {
    this.span = span;
    this.name = name;
    this.lambdas = lambdas;
    this.optional = optional;
    this.nargs = nargs;
    this.shapes = shapes;
    this.args = args;
    this.methods = methods;
    this.plans = plans;
    this.list = plans.get("list")!;
    this.string = plans.get("string")!;
  }

  /**
   * The call after its receiver `v` (02 2.3, 11.3), up to the entry's own
   * work: the call to make, or null when the call is optional and `v` is null
   * (the chain's SHORT). A method of an opaque type is called here, as it
   * calls no lambda; the call returned gives its result.
   */
  enter(r: Run, v: unknown): EntryCall | null {
    const k = kindQ(r, v);
    if (k === "null") {
      if (this.optional) return null;
      throw new RunFailure("null-operand", this.span, "method", { name: this.name });
    }
    if (k === "opaque") return { impl: hostResult, run: r, span: this.span, name: this.name, receiver: opaqueMethod(r, v, this), args: [] };
    const p = k === "list" ? this.list : k === "string" ? this.string : this.plans.get(k)!;
    if (p.entry === null) throw p.fail();
    return enterEntry(r, p.entry, v, this.args(r), this);
  }
}

/** A method call on a value of an opaque type, after its receiver (02 2.3, 11.4). */
function opaqueMethod(r: Run, v: unknown, m: MethodCode): unknown {
  const { span, name, nargs, shapes, args } = m;
  const t = r.cls.opaqueName(v);
  const host = t === null ? undefined : m.methods.get(t)?.get(name);
  if (host === undefined) throw new RunFailure("wrong-kind", span, "method", { kind: "opaque", name });
  const counts = host.counts;
  if (nargs < counts.min || nargs > counts.max) throw new RunFailure("wrong-kind", span, "count", { count: nargs, name });
  for (let i = 0; i < nargs; i++) {
    if (shapes[i] !== null) throw new RunFailure("wrong-kind", span, "not-callback", { count: i + 1, name });
  }
  const vals = args(r);
  return callHost(r, host, [v, ...vals], name, span);
}

/** A call of a host function or of a method of an opaque type (02 11.4, 03 3.5). */
function callHost(r: Run, host: HostImpl, args: unknown[], name: string, span: Span): unknown {
  r.charge(1);
  // The handle is valid until the function returns (03 3.5.2 item 6).
  // After that, charge and allocate throw a host-side Error and charge
  // nothing, and the signal reads as cancelled.
  // A charge the budget cannot cover ends the run (03 3.5.2 item 3), even
  // when the function catches the failure and goes on: `stop` holds it, a
  // later charge throws it again and charges nothing, and it is raised when
  // the function returns or throws.
  let open = true;
  let stop: RunFailure | null = null;
  const late = () => new Error("the call handle was used after its host function returned");
  const metered = (f: () => void) => {
    if (!open) throw late();
    if (stop !== null) throw stop;
    try {
      f();
    } catch (x) {
      if (x instanceof RunFailure) stop = x;
      throw x;
    }
  };
  const handle: CallHandle = {
    get signal() {
      return open ? r.signal : AbortSignal.abort();
    },
    charge: (n) => {
      metered(() => {
        if (n > 0) r.charge(n);
      });
    },
    allocate: (n) => {
      metered(() => {
        if (n > 0) r.alloc(n);
      });
    },
  };
  let out: unknown;
  try {
    out = host.call(handle, args);
  } catch (x) {
    open = false;
    if (stop !== null) throw stop;
    if (x instanceof RunFailure) throw x;
    // 03 3.5.3 items 5 to 7: a failure of this binding (a nested run's, say) is
    // `function-error` whose message is its first diagnostic's message, whole,
    // and whose cause is that diagnostic. Its code, class, span, path and data
    // do not reach the run.
    if (x instanceof RisorError && x.diagnostics.length > 0) {
      const inner = x.diagnostics[0];
      throw new RunFailure("function-error", span, null, {}, { text: inner.message, cause: inner });
    }
    if (x instanceof Error) throw new RunFailure("function-error", span, null, {}, { text: x.message, cause: x });
    throw new RunFailure("function-panic", span, "default", { name });
  }
  open = false;
  if (stop !== null) throw stop;
  r.checkTime();
  if (out instanceof Promise) throw new RunFailure("function-error", span, null, {}, { text: "a host function returned a Promise" });
  return r.read(out, span);
}

