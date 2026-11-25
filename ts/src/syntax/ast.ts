// The syntax tree (01 section 4). Core has 30 node kinds, numbered 1 to 30.
// Every node has a span (01 4.3) in code-point offsets.

import type { Span } from "./source.ts";
import type { TypeNode } from "../types/typetext.ts";

export const NODE_NUMBER = {
  Program: 1, Const: 2, Block: 3, Ident: 4, Number: 5, String: 6, Template: 7,
  Bool: 8, Null: 9, List: 10, Map: 11, Entry: 12, Spread: 13, Unary: 14,
  Binary: 15, Cond: 16, Member: 17, Index: 18, Call: 19, MethodCall: 20,
  OptChain: 21, Lambda: 22, Param: 23, Match: 24, Arm: 25, Try: 26,
  NamePat: 27, MapPat: 28, ListPat: 29, Bad: 30,
  // Package F4 (f04 2.7).
  As: 31,
  // Package F7 (f07 2.3).
  Each: 32,
} as const;

export type NodeKind = keyof typeof NODE_NUMBER;

interface Base {
  span: Span;
  /** Written in parentheses, which end a chain (01 7.4 rule 5). */
  parens?: true;
  /** The span with its outermost parentheses, when it has them. */
  outer?: Span;
}

export interface Program extends Base {
  kind: "Program";
  consts: Const[];
  /** Package F16: the type lines, in the order of the text (f16 2.3). Not nodes. */
  types: TypeLine[];
  /** Package F16 slice D2: absent (null) when the program has at least one `export const` and omits its result (f16 12.2). */
  result: Expr | null;
}

/** Package F16: a program type written in an annotation or a type line (f16 1.1). Not a node. */
export interface TypeAnn {
  type: TypeNode;
  /** From the type's first token to its last. */
  span: Span;
  /** The spans of the type's names and of each PType with alternatives (f04 6.1). */
  spans: Map<TypeNode, Span>;
}

/** Package F16: `type Name = T;` (f16 2.1, 2.3). Its span runs from `type` to its `;`. */
export interface TypeLine {
  name: string;
  nameSpan: Span;
  type: TypeAnn;
  span: Span;
  /** Package F12 with F16: the repair supplied the type, so the line declares nothing (f16 7.3 item 5). */
  supplied?: true;
  /** Package F16 slice D3: its description, when it has one (f16 11.3). */
  doc?: string;
}

/** Package F16: a function type on a parameter, `(a: T) => R` (f16 2.1). Its words bind nothing. */
export interface FnAnn {
  params: { word: string; span: Span; type: TypeAnn }[];
  /** The result type, or null for `unknown`. */
  result: TypeAnn | null;
  /** The span of the result, `unknown` included. */
  resultSpan: Span;
  /** From the `(` to the end of the result. */
  span: Span;
}

export interface Const extends Base {
  kind: "Const";
  /** Package F16 slice D2: true for an `export const` line, false for a plain `const` line (f16 12.2 item 3). */
  exported: boolean;
  pattern: Pattern;
  /** Package F16: the annotation, or null (f16 2.3). */
  type: TypeAnn | null;
  value: Expr | Lambda;
  /** Package F16 slice D3: its description, when it has one (f16 11.3). No walker reads it. */
  doc?: string;
}

export interface Block extends Base {
  kind: "Block";
  consts: Const[]; // one or more
  result: Expr;
}

export interface Ident extends Base {
  kind: "Ident";
  name: string;
}

export interface NumberLit extends Base {
  kind: "Number";
  value: number;
}

export interface StringLit extends Base {
  kind: "String";
  value: string;
  /** `"`, `'`, a backtick, and with package F3 `"""`, `r"`, `r'` and `r"""` (f03 5.2). */
  form: string;
}

export interface Template extends Base {
  kind: "Template";
  /** Text parts as strings, interpolations as expressions, in order. */
  parts: (string | Expr)[];
  form: string;
}

export interface Bool extends Base {
  kind: "Bool";
  value: boolean;
}

export interface Null extends Base {
  kind: "Null";
}

export interface List extends Base {
  kind: "List";
  items: (Expr | Spread)[];
}

export interface MapLit extends Base {
  kind: "Map";
  entries: (Entry | Spread)[];
}

export type EntryKey =
  | { kind: "word"; text: string; span: Span }
  | { kind: "string"; value: string; span: Span }
  | { kind: "expr"; expr: Expr };

export interface Entry extends Base {
  kind: "Entry";
  key: EntryKey;
  value: Expr;
  shorthand: boolean;
}

export interface Spread extends Base {
  kind: "Spread";
  value: Expr;
}

export interface Unary extends Base {
  kind: "Unary";
  op: "!" | "-";
  x: Expr;
}

export type BinaryOp =
  | "||" | "&&" | "==" | "!=" | "<" | "<=" | ">" | ">=" | "in" | "not-in"
  | "??" | "+" | "-" | "*" | "/" | "%" | "**";

export interface Binary extends Base {
  kind: "Binary";
  op: BinaryOp;
  x: Expr;
  y: Expr;
}

export interface Cond extends Base {
  kind: "Cond";
  form: "?:" | "if";
  test: Expr;
  then: Expr;
  else: Expr;
}

export interface Member extends Base {
  kind: "Member";
  x: Expr;
  name: string;
  nameSpan: Span;
  optional: boolean;
}

export interface Index extends Base {
  kind: "Index";
  x: Expr;
  index: Expr;
  optional: boolean;
}

export type Arg = Expr | Lambda;

export interface Call extends Base {
  kind: "Call";
  /** The levels open at the node's first token (01 13.2 rule 5). */
  level: number;
  namespace: string | null;
  name: string;
  nameSpan: Span;
  args: Arg[];
}

export interface MethodCall extends Base {
  kind: "MethodCall";
  /** The levels open at the node's first token (01 13.2 rule 5). */
  level: number;
  x: Expr;
  name: string;
  nameSpan: Span;
  args: Arg[];
  optional: boolean;
}

export interface OptChain extends Base {
  kind: "OptChain";
  x: Expr;
}

export interface Lambda extends Base {
  kind: "Lambda";
  /** The most levels open at any token of the lambda (05 20.2). */
  maxLevel: number;
  /** The levels open at the node's first token (01 13.2 rule 5). */
  level: number;
  params: Param[];
  /** Package F16: the result annotation, or null (f16 2.3). */
  result: TypeAnn | null;
  body: Expr;
}

export interface Param extends Base {
  kind: "Param";
  pattern: Pattern;
  /** Package F16: the annotation, a program type, or null; at most one of `type` and `fn` is set (f16 2.3). */
  type: TypeAnn | null;
  /** Package F16: the function type, or null. */
  fn: FnAnn | null;
  default: Expr | null;
  /** Package F16 slice D3: its description, when it has one (f16 11.3). No walker reads it. */
  doc?: string;
}

export type Literal = NumberLit | StringLit | Bool | Null;

export interface Match extends Base {
  kind: "Match";
  subject: Expr;
  arms: Arm[];
}

export interface Arm extends Base {
  kind: "Arm";
  /** The literals of the arm; empty for the default arm `_`. */
  literals: Literal[];
  isDefault: boolean;
  /** Package F17: the guard `if (e)`, or null (f17 2.2). */
  guard: Expr | null;
  result: Expr;
}

export interface Try extends Base {
  kind: "Try";
  body: Expr;
  name: string | null;
  nameSpan: Span | null;
  handler: Expr;
}

export interface NamePat extends Base {
  kind: "NamePat";
  /** The name bound, or null for the discard `_`. */
  name: string | null;
}

export interface MapPatField {
  /** The key read from the map. */
  key: string;
  /** The name bound. */
  name: string;
  nameSpan: Span;
  /** The key as written: the word, the string, or the shorthand name. */
  keySpan: Span;
  default: Expr | null;
  span: Span;
}

export interface MapPat extends Base {
  kind: "MapPat";
  fields: MapPatField[];
  rest: { name: string; span: Span } | null;
}

export interface ListPatElem {
  pat: NamePat;
  default: Expr | null;
  span: Span;
}

export interface ListPat extends Base {
  kind: "ListPat";
  elems: ListPatElem[];
  rest: { name: string; span: Span } | null;
}

export interface Bad extends Base {
  kind: "Bad";
}

export type Pattern = NamePat | MapPat | ListPat;

/** `x as T`, `x is T` (package F4, f04 2.7). */
export interface As extends Base {
  kind: "As";
  form: "as" | "is";
  x: Expr;
  /** The program type, as parsed: not resolved, not a node. */
  type: TypeNode;
  /** The span of the program type, from its first token to its last. */
  typeSpan: Span;
  /** The spans of the type's names and of each PType with alternatives (f04 6.1). */
  typeSpans: Map<TypeNode, Span>;
}

/**
 * A projection, `xs[*].f` (package F7, f07 2.3). The first star has `x`; a
 * later star is an `Each` with `x` null, the last of the reads of the level
 * before it. A read is a `Member` or `Index` whose `x` is a `Bad` node: it is
 * applied to the element in hand.
 */
export interface Each extends Base {
  kind: "Each";
  x: Expr | null;
  reads: (Member | Index | Each)[];
  optional: boolean;
  /** The star span: from the first token of the chain to the star's `]` (f07 2.4). */
  starSpan: Span;
  /** The span of the receiver of the star (f07 2.4 item 2). */
  recvSpan: Span;
}

export type Expr =
  | Block | Ident | NumberLit | StringLit | Template | Bool | Null | List | MapLit
  | Unary | Binary | Cond | Member | Index | Call | MethodCall | OptChain | Match
  | Try | Bad | As | Each;

export type Node =
  | Program | Const | Expr | Entry | Spread | Lambda | Param | Arm | Pattern;
