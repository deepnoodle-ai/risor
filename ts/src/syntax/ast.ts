// The syntax tree (01 section 4). Core has 30 node kinds, numbered 1 to 30.
// Every node has a span (01 4.3) in code-point offsets.

import type { Span } from "./source.ts";

export const NODE_NUMBER = {
  Program: 1, Const: 2, Block: 3, Ident: 4, Number: 5, String: 6, Template: 7,
  Bool: 8, Null: 9, List: 10, Map: 11, Entry: 12, Spread: 13, Unary: 14,
  Binary: 15, Cond: 16, Member: 17, Index: 18, Call: 19, MethodCall: 20,
  OptChain: 21, Lambda: 22, Param: 23, Match: 24, Arm: 25, Try: 26,
  NamePat: 27, MapPat: 28, ListPat: 29, Bad: 30,
} as const;

export type NodeKind = keyof typeof NODE_NUMBER;

interface Base {
  span: Span;
}

export interface Program extends Base {
  kind: "Program";
  consts: Const[];
  result: Expr;
}

export interface Const extends Base {
  kind: "Const";
  pattern: Pattern;
  value: Expr | Lambda;
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
  body: Expr;
}

export interface Param extends Base {
  kind: "Param";
  pattern: Pattern;
  default: Expr | null;
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

export type Expr =
  | Block | Ident | NumberLit | StringLit | Template | Bool | Null | List | MapLit
  | Unary | Binary | Cond | Member | Index | Call | MethodCall | OptChain | Match
  | Try | Bad;

export type Node =
  | Program | Const | Expr | Entry | Spread | Lambda | Param | Arm | Pattern;
