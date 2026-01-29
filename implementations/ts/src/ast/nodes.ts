/**
 * AST node types for the Risor parser.
 */

import type { Position } from "../token/token.js";

/**
 * Base interface for all AST nodes.
 */
export interface Node {
  /** Start position in source */
  pos(): Position;
  /** End position in source */
  end(): Position;
  /** String representation */
  toString(): string;
}

/**
 * Expression nodes produce a value.
 */
export interface Expr extends Node {
  _exprBrand: void;
}

/**
 * Statement nodes perform an action.
 */
export interface Stmt extends Node {
  _stmtBrand: void;
}

/**
 * Pattern nodes are used in match expressions.
 */
export interface Pattern extends Node {
  _patternBrand: void;
}

// ============================================================================
// Literal Expressions
// ============================================================================

/**
 * Integer literal.
 */
export class IntLit implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly position: Position,
    public readonly literal: string,
    public readonly value: bigint
  ) {}

  pos(): Position {
    return this.position;
  }
  end(): Position {
    return { ...this.position, char: this.position.char + this.literal.length, column: this.position.column + this.literal.length };
  }
  toString(): string {
    return this.literal;
  }
}

/**
 * Float literal.
 */
export class FloatLit implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly position: Position,
    public readonly literal: string,
    public readonly value: number
  ) {}

  pos(): Position {
    return this.position;
  }
  end(): Position {
    return { ...this.position, char: this.position.char + this.literal.length, column: this.position.column + this.literal.length };
  }
  toString(): string {
    return this.literal;
  }
}

/**
 * Boolean literal.
 */
export class BoolLit implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly position: Position,
    public readonly value: boolean
  ) {}

  pos(): Position {
    return this.position;
  }
  end(): Position {
    const len = this.value ? 4 : 5; // "true" or "false"
    return { ...this.position, char: this.position.char + len, column: this.position.column + len };
  }
  toString(): string {
    return this.value ? "true" : "false";
  }
}

/**
 * Nil literal.
 */
export class NilLit implements Expr {
  _exprBrand!: void;

  constructor(public readonly position: Position) {}

  pos(): Position {
    return this.position;
  }
  end(): Position {
    return { ...this.position, char: this.position.char + 3, column: this.position.column + 3 };
  }
  toString(): string {
    return "nil";
  }
}

/**
 * String literal.
 */
export class StringLit implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly position: Position,
    public readonly value: string
  ) {}

  pos(): Position {
    return this.position;
  }
  end(): Position {
    // Approximate - includes quotes
    const len = this.value.length + 2;
    return { ...this.position, char: this.position.char + len, column: this.position.column + len };
  }
  toString(): string {
    return JSON.stringify(this.value);
  }
}

// ============================================================================
// Identifier
// ============================================================================

/**
 * Identifier (variable reference).
 */
export class Ident implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly position: Position,
    public readonly name: string
  ) {}

  pos(): Position {
    return this.position;
  }
  end(): Position {
    return { ...this.position, char: this.position.char + this.name.length, column: this.position.column + this.name.length };
  }
  toString(): string {
    return this.name;
  }
}

// ============================================================================
// Operator Expressions
// ============================================================================

/**
 * Prefix operator expression (unary).
 */
export class PrefixExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly opPos: Position,
    public readonly op: string,
    public readonly right: Expr
  ) {}

  pos(): Position {
    return this.opPos;
  }
  end(): Position {
    return this.right.end();
  }
  toString(): string {
    return `(${this.op}${this.right.toString()})`;
  }
}

/**
 * Infix operator expression (binary).
 */
export class InfixExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly left: Expr,
    public readonly opPos: Position,
    public readonly op: string,
    public readonly right: Expr
  ) {}

  pos(): Position {
    return this.left.pos();
  }
  end(): Position {
    return this.right.end();
  }
  toString(): string {
    return `(${this.left.toString()} ${this.op} ${this.right.toString()})`;
  }
}

/**
 * Spread expression (...expr).
 */
export class SpreadExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly ellipsis: Position,
    public readonly expr: Expr | null
  ) {}

  pos(): Position {
    return this.ellipsis;
  }
  end(): Position {
    if (this.expr) return this.expr.end();
    return { ...this.ellipsis, char: this.ellipsis.char + 3, column: this.ellipsis.column + 3 };
  }
  toString(): string {
    return this.expr ? `...${this.expr.toString()}` : "...";
  }
}

// ============================================================================
// Collection Literals
// ============================================================================

/**
 * List literal [a, b, c].
 */
export class ListLit implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly lbrack: Position,
    public readonly items: Expr[],
    public readonly rbrack: Position
  ) {}

  pos(): Position {
    return this.lbrack;
  }
  end(): Position {
    return { ...this.rbrack, char: this.rbrack.char + 1, column: this.rbrack.column + 1 };
  }
  toString(): string {
    return `[${this.items.map((i) => i.toString()).join(", ")}]`;
  }
}

/**
 * Map item (key-value pair or spread).
 */
export interface MapItem {
  key: Expr | null; // null for spread
  value: Expr;
}

/**
 * Map literal {a: 1, b: 2}.
 */
export class MapLit implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly lbrace: Position,
    public readonly items: MapItem[],
    public readonly rbrace: Position
  ) {}

  pos(): Position {
    return this.lbrace;
  }
  end(): Position {
    return { ...this.rbrace, char: this.rbrace.char + 1, column: this.rbrace.column + 1 };
  }
  toString(): string {
    const pairs = this.items.map((item) =>
      item.key ? `${item.key.toString()}: ${item.value.toString()}` : `...${item.value.toString()}`
    );
    return `{${pairs.join(", ")}}`;
  }
}

// ============================================================================
// Function Expressions
// ============================================================================

/**
 * Function parameter (can be identifier or destructuring).
 */
export type FuncParam = Ident | ObjectDestructureParam | ArrayDestructureParam;

/**
 * Object destructuring parameter {a, b}.
 */
export class ObjectDestructureParam implements Node {
  constructor(
    public readonly lbrace: Position,
    public readonly bindings: DestructureBinding[],
    public readonly rbrace: Position
  ) {}

  pos(): Position {
    return this.lbrace;
  }
  end(): Position {
    return { ...this.rbrace, char: this.rbrace.char + 1, column: this.rbrace.column + 1 };
  }
  toString(): string {
    const parts = this.bindings.map((b) => {
      let s = b.key;
      if (b.alias && b.alias !== b.key) s += `: ${b.alias}`;
      if (b.defaultValue) s += ` = ${b.defaultValue.toString()}`;
      return s;
    });
    return `{${parts.join(", ")}}`;
  }
}

/**
 * Array destructuring parameter [a, b].
 */
export class ArrayDestructureParam implements Node {
  constructor(
    public readonly lbrack: Position,
    public readonly elements: ArrayDestructureElement[],
    public readonly rbrack: Position
  ) {}

  pos(): Position {
    return this.lbrack;
  }
  end(): Position {
    return { ...this.rbrack, char: this.rbrack.char + 1, column: this.rbrack.column + 1 };
  }
  toString(): string {
    const parts = this.elements.map((e) => {
      let s = e.name.toString();
      if (e.defaultValue) s += ` = ${e.defaultValue.toString()}`;
      return s;
    });
    return `[${parts.join(", ")}]`;
  }
}

/**
 * Destructure binding for object destructuring.
 */
export interface DestructureBinding {
  key: string;
  alias: string | null;
  defaultValue: Expr | null;
}

/**
 * Array destructure element.
 */
export interface ArrayDestructureElement {
  name: Ident;
  defaultValue: Expr | null;
}

/**
 * Function literal.
 */
export class FuncLit implements Expr, Stmt {
  _exprBrand!: void;
  _stmtBrand!: void;

  constructor(
    public readonly funcPos: Position,
    public readonly name: Ident | null,
    public readonly lparen: Position,
    public readonly params: FuncParam[],
    public readonly defaults: Map<string, Expr>,
    public readonly restParam: Ident | null,
    public readonly rparen: Position,
    public readonly body: Block
  ) {}

  pos(): Position {
    return this.funcPos;
  }
  end(): Position {
    return this.body.end();
  }
  toString(): string {
    const params = this.params.map((p) => p.toString()).join(", ");
    const name = this.name ? ` ${this.name.name}` : "";
    return `function${name}(${params}) { ${this.body.toString()} }`;
  }
}

// ============================================================================
// Access Expressions
// ============================================================================

/**
 * Function call expression.
 */
export class CallExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly func: Expr,
    public readonly lparen: Position,
    public readonly args: (Expr | SpreadExpr)[],
    public readonly rparen: Position
  ) {}

  pos(): Position {
    return this.func.pos();
  }
  end(): Position {
    return { ...this.rparen, char: this.rparen.char + 1, column: this.rparen.column + 1 };
  }
  toString(): string {
    return `${this.func.toString()}(${this.args.map((a) => a.toString()).join(", ")})`;
  }
}

/**
 * Property access expression (obj.prop).
 */
export class GetAttrExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly object: Expr,
    public readonly period: Position,
    public readonly attr: Ident,
    public readonly optional: boolean
  ) {}

  pos(): Position {
    return this.object.pos();
  }
  end(): Position {
    return this.attr.end();
  }
  toString(): string {
    const op = this.optional ? "?." : ".";
    return `${this.object.toString()}${op}${this.attr.name}`;
  }
}

/**
 * Method call expression (obj.method()).
 */
export class ObjectCallExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly object: Expr,
    public readonly period: Position,
    public readonly call: CallExpr,
    public readonly optional: boolean
  ) {}

  pos(): Position {
    return this.object.pos();
  }
  end(): Position {
    return this.call.end();
  }
  toString(): string {
    const op = this.optional ? "?." : ".";
    return `${this.object.toString()}${op}${this.call.toString()}`;
  }
}

/**
 * Index expression (arr[index]).
 */
export class IndexExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly object: Expr,
    public readonly lbrack: Position,
    public readonly index: Expr,
    public readonly rbrack: Position
  ) {}

  pos(): Position {
    return this.object.pos();
  }
  end(): Position {
    return { ...this.rbrack, char: this.rbrack.char + 1, column: this.rbrack.column + 1 };
  }
  toString(): string {
    return `${this.object.toString()}[${this.index.toString()}]`;
  }
}

/**
 * Slice expression (arr[low:high]).
 */
export class SliceExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly object: Expr,
    public readonly lbrack: Position,
    public readonly low: Expr | null,
    public readonly high: Expr | null,
    public readonly rbrack: Position
  ) {}

  pos(): Position {
    return this.object.pos();
  }
  end(): Position {
    return { ...this.rbrack, char: this.rbrack.char + 1, column: this.rbrack.column + 1 };
  }
  toString(): string {
    const low = this.low?.toString() ?? "";
    const high = this.high?.toString() ?? "";
    return `${this.object.toString()}[${low}:${high}]`;
  }
}

// ============================================================================
// Control Flow Expressions
// ============================================================================

/**
 * If expression.
 */
export class IfExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly ifPos: Position,
    public readonly condition: Expr,
    public readonly consequence: Block,
    public readonly alternative: Block | null
  ) {}

  pos(): Position {
    return this.ifPos;
  }
  end(): Position {
    return this.alternative?.end() ?? this.consequence.end();
  }
  toString(): string {
    let s = `if (${this.condition.toString()}) ${this.consequence.toString()}`;
    if (this.alternative) s += ` else ${this.alternative.toString()}`;
    return s;
  }
}

/**
 * Switch case.
 */
export class CaseClause implements Node {
  constructor(
    public readonly casePos: Position,
    public readonly exprs: Expr[] | null, // null for default
    public readonly colon: Position,
    public readonly body: Block,
    public readonly isDefault: boolean
  ) {}

  pos(): Position {
    return this.casePos;
  }
  end(): Position {
    return this.body.end();
  }
  toString(): string {
    if (this.isDefault) return `default: ${this.body.toString()}`;
    return `case ${this.exprs!.map((e) => e.toString()).join(", ")}: ${this.body.toString()}`;
  }
}

/**
 * Switch expression.
 */
export class SwitchExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly switchPos: Position,
    public readonly value: Expr,
    public readonly lbrace: Position,
    public readonly cases: CaseClause[],
    public readonly rbrace: Position
  ) {}

  pos(): Position {
    return this.switchPos;
  }
  end(): Position {
    return { ...this.rbrace, char: this.rbrace.char + 1, column: this.rbrace.column + 1 };
  }
  toString(): string {
    const cases = this.cases.map((c) => c.toString()).join("\n");
    return `switch (${this.value.toString()}) {\n${cases}\n}`;
  }
}

/**
 * Wildcard pattern (_).
 */
export class WildcardPattern implements Pattern {
  _patternBrand!: void;

  constructor(public readonly position: Position) {}

  pos(): Position {
    return this.position;
  }
  end(): Position {
    return { ...this.position, char: this.position.char + 1, column: this.position.column + 1 };
  }
  toString(): string {
    return "_";
  }
}

/**
 * Literal pattern (expression evaluated at runtime).
 */
export class LiteralPattern implements Pattern {
  _patternBrand!: void;

  constructor(public readonly value: Expr) {}

  pos(): Position {
    return this.value.pos();
  }
  end(): Position {
    return this.value.end();
  }
  toString(): string {
    return this.value.toString();
  }
}

/**
 * Match arm (pattern => result).
 */
export class MatchArm implements Node {
  constructor(
    public readonly pattern: Pattern,
    public readonly guard: Expr | null,
    public readonly arrow: Position,
    public readonly result: Expr
  ) {}

  pos(): Position {
    return this.pattern.pos();
  }
  end(): Position {
    return this.result.end();
  }
  toString(): string {
    let s = this.pattern.toString();
    if (this.guard) s += ` if ${this.guard.toString()}`;
    s += ` => ${this.result.toString()}`;
    return s;
  }
}

/**
 * Match expression.
 */
export class MatchExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly matchPos: Position,
    public readonly subject: Expr,
    public readonly lbrace: Position,
    public readonly arms: MatchArm[],
    public readonly defaultArm: MatchArm | null,
    public readonly rbrace: Position
  ) {}

  pos(): Position {
    return this.matchPos;
  }
  end(): Position {
    return { ...this.rbrace, char: this.rbrace.char + 1, column: this.rbrace.column + 1 };
  }
  toString(): string {
    const arms = [...this.arms];
    if (this.defaultArm) arms.push(this.defaultArm);
    return `match ${this.subject.toString()} { ${arms.map((a) => a.toString()).join(", ")} }`;
  }
}

/**
 * In expression (x in container).
 */
export class InExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly left: Expr,
    public readonly inPos: Position,
    public readonly right: Expr
  ) {}

  pos(): Position {
    return this.left.pos();
  }
  end(): Position {
    return this.right.end();
  }
  toString(): string {
    return `${this.left.toString()} in ${this.right.toString()}`;
  }
}

/**
 * Not in expression (x not in container).
 */
export class NotInExpr implements Expr {
  _exprBrand!: void;

  constructor(
    public readonly left: Expr,
    public readonly notPos: Position,
    public readonly right: Expr
  ) {}

  pos(): Position {
    return this.left.pos();
  }
  end(): Position {
    return this.right.end();
  }
  toString(): string {
    return `${this.left.toString()} not in ${this.right.toString()}`;
  }
}

/**
 * Pipe expression (x | f | g).
 */
export class PipeExpr implements Expr {
  _exprBrand!: void;

  constructor(public readonly exprs: Expr[]) {}

  pos(): Position {
    return this.exprs[0].pos();
  }
  end(): Position {
    return this.exprs[this.exprs.length - 1].end();
  }
  toString(): string {
    return `(${this.exprs.map((e) => e.toString()).join(" | ")})`;
  }
}

// ============================================================================
// Statements
// ============================================================================

/**
 * Block statement.
 */
export class Block implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly lbrace: Position,
    public readonly stmts: (Stmt | Expr)[],
    public readonly rbrace: Position
  ) {}

  pos(): Position {
    return this.lbrace;
  }
  end(): Position {
    return { ...this.rbrace, char: this.rbrace.char + 1, column: this.rbrace.column + 1 };
  }
  toString(): string {
    return this.stmts.map((s) => s.toString()).join("\n");
  }
}

/**
 * Variable declaration (let x = value).
 */
export class VarStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly letPos: Position,
    public readonly name: Ident,
    public readonly value: Expr
  ) {}

  pos(): Position {
    return this.letPos;
  }
  end(): Position {
    return this.value.end();
  }
  toString(): string {
    return `let ${this.name.name} = ${this.value.toString()}`;
  }
}

/**
 * Multiple variable declaration (let x, y = [1, 2]).
 */
export class MultiVarStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly letPos: Position,
    public readonly names: Ident[],
    public readonly value: Expr
  ) {}

  pos(): Position {
    return this.letPos;
  }
  end(): Position {
    return this.value.end();
  }
  toString(): string {
    return `let ${this.names.map((n) => n.name).join(", ")} = ${this.value.toString()}`;
  }
}

/**
 * Object destructuring statement (let { a, b } = obj).
 */
export class ObjectDestructureStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly letPos: Position,
    public readonly lbrace: Position,
    public readonly bindings: DestructureBinding[],
    public readonly rbrace: Position,
    public readonly value: Expr
  ) {}

  pos(): Position {
    return this.letPos;
  }
  end(): Position {
    return this.value.end();
  }
  toString(): string {
    const parts = this.bindings.map((b) => {
      let s = b.key;
      if (b.alias && b.alias !== b.key) s += `: ${b.alias}`;
      if (b.defaultValue) s += ` = ${b.defaultValue.toString()}`;
      return s;
    });
    return `let { ${parts.join(", ")} } = ${this.value.toString()}`;
  }
}

/**
 * Array destructuring statement (let [a, b] = arr).
 */
export class ArrayDestructureStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly letPos: Position,
    public readonly lbrack: Position,
    public readonly elements: ArrayDestructureElement[],
    public readonly rbrack: Position,
    public readonly value: Expr
  ) {}

  pos(): Position {
    return this.letPos;
  }
  end(): Position {
    return this.value.end();
  }
  toString(): string {
    const parts = this.elements.map((e) => {
      let s = e.name.toString();
      if (e.defaultValue) s += ` = ${e.defaultValue.toString()}`;
      return s;
    });
    return `let [${parts.join(", ")}] = ${this.value.toString()}`;
  }
}

/**
 * Constant declaration (const X = value).
 */
export class ConstStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly constPos: Position,
    public readonly name: Ident,
    public readonly value: Expr
  ) {}

  pos(): Position {
    return this.constPos;
  }
  end(): Position {
    return this.value.end();
  }
  toString(): string {
    return `const ${this.name.name} = ${this.value.toString()}`;
  }
}

/**
 * Return statement.
 */
export class ReturnStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly returnPos: Position,
    public readonly value: Expr | null
  ) {}

  pos(): Position {
    return this.returnPos;
  }
  end(): Position {
    return this.value?.end() ?? { ...this.returnPos, char: this.returnPos.char + 6, column: this.returnPos.column + 6 };
  }
  toString(): string {
    return this.value ? `return ${this.value.toString()}` : "return";
  }
}

/**
 * Assignment statement (x = value).
 */
export class AssignStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly target: Ident | IndexExpr,
    public readonly opPos: Position,
    public readonly op: string,
    public readonly value: Expr
  ) {}

  pos(): Position {
    return this.target.pos();
  }
  end(): Position {
    return this.value.end();
  }
  toString(): string {
    return `${this.target.toString()} ${this.op} ${this.value.toString()}`;
  }
}

/**
 * Property assignment statement (obj.prop = value).
 */
export class SetAttrStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly object: Expr,
    public readonly period: Position,
    public readonly attr: Ident,
    public readonly opPos: Position,
    public readonly op: string,
    public readonly value: Expr
  ) {}

  pos(): Position {
    return this.object.pos();
  }
  end(): Position {
    return this.value.end();
  }
  toString(): string {
    return `${this.object.toString()}.${this.attr.name} ${this.op} ${this.value.toString()}`;
  }
}

/**
 * Postfix statement (x++ or x--).
 */
export class PostfixStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly operand: Expr,
    public readonly opPos: Position,
    public readonly op: string
  ) {}

  pos(): Position {
    return this.operand.pos();
  }
  end(): Position {
    return { ...this.opPos, char: this.opPos.char + 2, column: this.opPos.column + 2 };
  }
  toString(): string {
    return `(${this.operand.toString()}${this.op})`;
  }
}

/**
 * Throw statement.
 */
export class ThrowStmt implements Stmt {
  _stmtBrand!: void;

  constructor(
    public readonly throwPos: Position,
    public readonly value: Expr
  ) {}

  pos(): Position {
    return this.throwPos;
  }
  end(): Position {
    return this.value.end();
  }
  toString(): string {
    return `throw ${this.value.toString()}`;
  }
}

/**
 * Try/catch/finally expression/statement.
 */
export class TryExpr implements Expr, Stmt {
  _exprBrand!: void;
  _stmtBrand!: void;

  constructor(
    public readonly tryPos: Position,
    public readonly body: Block,
    public readonly catchIdent: Ident | null,
    public readonly catchBlock: Block | null,
    public readonly finallyBlock: Block | null
  ) {}

  pos(): Position {
    return this.tryPos;
  }
  end(): Position {
    return this.finallyBlock?.end() ?? this.catchBlock?.end() ?? this.body.end();
  }
  toString(): string {
    let s = `try ${this.body.toString()}`;
    if (this.catchBlock) {
      s += ` catch`;
      if (this.catchIdent) s += ` ${this.catchIdent.name}`;
      s += ` ${this.catchBlock.toString()}`;
    }
    if (this.finallyBlock) {
      s += ` finally ${this.finallyBlock.toString()}`;
    }
    return s;
  }
}

/**
 * Expression statement (expression used as statement).
 */
export class ExprStmt implements Stmt {
  _stmtBrand!: void;

  constructor(public readonly expr: Expr) {}

  pos(): Position {
    return this.expr.pos();
  }
  end(): Position {
    return this.expr.end();
  }
  toString(): string {
    return this.expr.toString();
  }
}

// ============================================================================
// Program
// ============================================================================

/**
 * Program is the root AST node.
 */
export class Program implements Node {
  constructor(public readonly stmts: (Stmt | Expr)[]) {}

  pos(): Position {
    if (this.stmts.length > 0) return this.stmts[0].pos();
    return { char: 0, lineStart: 0, line: 0, column: 0, file: "" };
  }
  end(): Position {
    if (this.stmts.length > 0) return this.stmts[this.stmts.length - 1].end();
    return { char: 0, lineStart: 0, line: 0, column: 0, file: "" };
  }
  toString(): string {
    return this.stmts.map((s) => s.toString()).join("\n");
  }
}
