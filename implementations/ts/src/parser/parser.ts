/**
 * Pratt parser for Risor.
 */

import { Lexer, LexerError } from "../lexer/lexer.js";
import { Token, TokenKind, Position, NoPos } from "../token/token.js";
import { Precedence, getPrecedence } from "./precedence.js";
import * as ast from "../ast/nodes.js";

/**
 * Parser error with position information.
 */
export class ParserError extends Error {
  constructor(
    message: string,
    public readonly position: Position
  ) {
    super(`${message} at line ${position.line + 1}, column ${position.column + 1}`);
    this.name = "ParserError";
  }
}

type PrefixParseFn = () => ast.Expr | null;
type InfixParseFn = (left: ast.Expr) => ast.Expr | null;

/**
 * Pratt parser for Risor source code.
 */
export class Parser {
  private lexer: Lexer;
  private curToken: Token;
  private peekToken: Token;
  private errors: ParserError[] = [];
  private maxDepth = 500;
  private depth = 0;

  private prefixParseFns: Map<TokenKind, PrefixParseFn> = new Map();
  private infixParseFns: Map<TokenKind, InfixParseFn> = new Map();

  constructor(lexer: Lexer) {
    this.lexer = lexer;
    // Initialize tokens
    this.curToken = this.lexer.nextToken();
    this.peekToken = this.lexer.nextToken();

    // Register prefix parse functions
    this.registerPrefix(TokenKind.IDENT, () => this.parseIdent());
    this.registerPrefix(TokenKind.INT, () => this.parseInt());
    this.registerPrefix(TokenKind.FLOAT, () => this.parseFloat());
    this.registerPrefix(TokenKind.STRING, () => this.parseString());
    this.registerPrefix(TokenKind.TEMPLATE, () => this.parseTemplate());
    this.registerPrefix(TokenKind.TRUE, () => this.parseBool());
    this.registerPrefix(TokenKind.FALSE, () => this.parseBool());
    this.registerPrefix(TokenKind.NIL, () => this.parseNil());
    this.registerPrefix(TokenKind.BANG, () => this.parsePrefix());
    this.registerPrefix(TokenKind.MINUS, () => this.parsePrefix());
    this.registerPrefix(TokenKind.NOT, () => this.parsePrefix());
    this.registerPrefix(TokenKind.LPAREN, () => this.parseGrouped());
    this.registerPrefix(TokenKind.LBRACKET, () => this.parseList());
    this.registerPrefix(TokenKind.LBRACE, () => this.parseMap());
    this.registerPrefix(TokenKind.IF, () => this.parseIf());
    this.registerPrefix(TokenKind.SWITCH, () => this.parseSwitch());
    this.registerPrefix(TokenKind.MATCH, () => this.parseMatch());
    this.registerPrefix(TokenKind.FUNCTION, () => this.parseFunc());
    this.registerPrefix(TokenKind.SPREAD, () => this.parseSpread());
    this.registerPrefix(TokenKind.TRY, () => this.parseTry());
    this.registerPrefix(TokenKind.PIPE, () => this.parsePipePrefix());

    // Register infix parse functions
    this.registerInfix(TokenKind.PLUS, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.MINUS, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.ASTERISK, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.SLASH, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.MOD, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.POW, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.EQ, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.NOT_EQ, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.LT, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.GT, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.LT_EQUALS, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.GT_EQUALS, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.AND, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.OR, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.AMPERSAND, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.PIPE, (left) => this.parsePipeInfix(left));
    this.registerInfix(TokenKind.CARET, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.LT_LT, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.GT_GT, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.NULLISH, (left) => this.parseInfix(left));
    this.registerInfix(TokenKind.LPAREN, (left) => this.parseCall(left));
    this.registerInfix(TokenKind.LBRACKET, (left) => this.parseIndex(left));
    this.registerInfix(TokenKind.PERIOD, (left) => this.parseGetAttr(left));
    this.registerInfix(TokenKind.QUESTION_DOT, (left) => this.parseOptionalChain(left));
    this.registerInfix(TokenKind.IN, (left) => this.parseIn(left));
    this.registerInfix(TokenKind.NOT, (left) => this.parseNotIn(left));
  }

  private registerPrefix(kind: TokenKind, fn: PrefixParseFn): void {
    this.prefixParseFns.set(kind, fn);
  }

  private registerInfix(kind: TokenKind, fn: InfixParseFn): void {
    this.infixParseFns.set(kind, fn);
  }

  private nextToken(): void {
    this.curToken = this.peekToken;
    this.peekToken = this.lexer.nextToken();
  }

  private curTokenIs(kind: TokenKind): boolean {
    return this.curToken.kind === kind;
  }

  private peekTokenIs(kind: TokenKind): boolean {
    return this.peekToken.kind === kind;
  }

  private expectPeek(kind: TokenKind): boolean {
    if (this.peekTokenIs(kind)) {
      this.nextToken();
      return true;
    }
    this.peekError(kind);
    return false;
  }

  private peekError(kind: TokenKind): void {
    this.errors.push(
      new ParserError(
        `expected ${kind}, got ${this.peekToken.kind}`,
        this.peekToken.start
      )
    );
  }

  private noPrefixParseFnError(kind: TokenKind): void {
    this.errors.push(
      new ParserError(`unexpected token ${kind}`, this.curToken.start)
    );
  }

  private curPrecedence(): Precedence {
    return getPrecedence(this.curToken.kind);
  }

  private peekPrecedence(): Precedence {
    return getPrecedence(this.peekToken.kind);
  }

  private eatNewlines(): void {
    while (this.curTokenIs(TokenKind.NEWLINE)) {
      this.nextToken();
    }
  }

  private skipPeekNewlines(): void {
    while (this.peekTokenIs(TokenKind.NEWLINE)) {
      this.nextToken();
    }
  }

  /**
   * Synchronize after an error by skipping to the next statement boundary.
   */
  private synchronize(): void {
    while (!this.curTokenIs(TokenKind.EOF)) {
      // Stop at newline (statement boundary)
      if (this.curTokenIs(TokenKind.NEWLINE)) {
        this.nextToken();
        return;
      }
      // Stop at statement-starting keywords
      switch (this.curToken.kind) {
        case TokenKind.LET:
        case TokenKind.CONST:
        case TokenKind.FUNCTION:
        case TokenKind.RETURN:
        case TokenKind.IF:
        case TokenKind.SWITCH:
        case TokenKind.MATCH:
        case TokenKind.TRY:
        case TokenKind.THROW:
          return;
      }
      this.nextToken();
    }
  }

  /**
   * Parse the entire program.
   */
  parse(): ast.Program {
    const stmts: (ast.Stmt | ast.Expr)[] = [];

    this.eatNewlines();

    while (!this.curTokenIs(TokenKind.EOF)) {
      const stmt = this.parseStatement();
      if (stmt) {
        stmts.push(stmt);
      } else {
        // Error recovery: skip to next statement
        this.synchronize();
      }
      this.eatNewlines();
    }

    if (this.errors.length > 0) {
      throw this.errors[0];
    }

    return new ast.Program(stmts);
  }

  /**
   * Get all parse errors.
   */
  getErrors(): ParserError[] {
    return this.errors;
  }

  // =========================================================================
  // Statement Parsing
  // =========================================================================

  private parseStatement(): ast.Stmt | ast.Expr | null {
    switch (this.curToken.kind) {
      case TokenKind.LET:
        return this.parseLet();
      case TokenKind.CONST:
        return this.parseConst();
      case TokenKind.RETURN:
        return this.parseReturn();
      case TokenKind.THROW:
        return this.parseThrow();
      default:
        return this.parseExpressionStatement();
    }
  }

  private parseLet(): ast.Stmt | null {
    const letPos = this.curToken.start;
    this.nextToken(); // consume 'let'

    // Check for destructuring
    if (this.curTokenIs(TokenKind.LBRACE)) {
      return this.parseObjectDestructure(letPos);
    }
    if (this.curTokenIs(TokenKind.LBRACKET)) {
      return this.parseArrayDestructure(letPos);
    }

    // Simple variable or multi-var
    if (!this.curTokenIs(TokenKind.IDENT)) {
      this.errors.push(new ParserError("expected identifier", this.curToken.start));
      return null;
    }

    const firstName = new ast.Ident(this.curToken.start, this.curToken.literal);
    this.nextToken();

    // Check for multi-var (let x, y = ...)
    if (this.curTokenIs(TokenKind.COMMA)) {
      const names: ast.Ident[] = [firstName];
      while (this.curTokenIs(TokenKind.COMMA)) {
        this.nextToken(); // consume ','
        if (!this.curTokenIs(TokenKind.IDENT)) {
          this.errors.push(new ParserError("expected identifier", this.curToken.start));
          return null;
        }
        names.push(new ast.Ident(this.curToken.start, this.curToken.literal));
        this.nextToken();
      }
      if (!this.curTokenIs(TokenKind.ASSIGN)) {
        this.errors.push(new ParserError("expected '='", this.curToken.start));
        return null;
      }
      this.nextToken(); // consume '='
      const value = this.parseExpression(Precedence.LOWEST);
      if (!value) return null;
      return new ast.MultiVarStmt(letPos, names, value);
    }

    // Simple variable
    if (!this.curTokenIs(TokenKind.ASSIGN)) {
      this.errors.push(new ParserError("expected '='", this.curToken.start));
      return null;
    }
    this.nextToken(); // consume '='
    const value = this.parseExpression(Precedence.LOWEST);
    if (!value) return null;
    return new ast.VarStmt(letPos, firstName, value);
  }

  private parseObjectDestructure(letPos: Position): ast.ObjectDestructureStmt | null {
    const lbrace = this.curToken.start;
    this.nextToken(); // consume '{'
    this.eatNewlines();

    const bindings: ast.DestructureBinding[] = [];
    while (!this.curTokenIs(TokenKind.RBRACE) && !this.curTokenIs(TokenKind.EOF)) {
      if (!this.curTokenIs(TokenKind.IDENT)) {
        this.errors.push(new ParserError("expected identifier", this.curToken.start));
        return null;
      }
      const key = this.curToken.literal;
      let alias: string | null = null;
      let defaultValue: ast.Expr | null = null;
      this.nextToken();

      // Check for alias (: alias)
      if (this.curTokenIs(TokenKind.COLON)) {
        this.nextToken(); // consume ':'
        if (!this.curTokenIs(TokenKind.IDENT)) {
          this.errors.push(new ParserError("expected identifier", this.curToken.start));
          return null;
        }
        alias = this.curToken.literal;
        this.nextToken();
      }

      // Check for default (= value)
      if (this.curTokenIs(TokenKind.ASSIGN)) {
        this.nextToken(); // consume '='
        defaultValue = this.parseExpression(Precedence.LOWEST);
        if (!defaultValue) return null;
      }

      bindings.push({ key, alias, defaultValue });

      if (!this.curTokenIs(TokenKind.COMMA)) break;
      this.nextToken(); // consume ','
      this.eatNewlines();
    }

    if (!this.curTokenIs(TokenKind.RBRACE)) {
      this.errors.push(new ParserError("expected '}'", this.curToken.start));
      return null;
    }
    const rbrace = this.curToken.start;
    this.nextToken(); // consume '}'

    if (!this.curTokenIs(TokenKind.ASSIGN)) {
      this.errors.push(new ParserError("expected '='", this.curToken.start));
      return null;
    }
    this.nextToken(); // consume '='
    const value = this.parseExpression(Precedence.LOWEST);
    if (!value) return null;

    return new ast.ObjectDestructureStmt(letPos, lbrace, bindings, rbrace, value);
  }

  private parseArrayDestructure(letPos: Position): ast.ArrayDestructureStmt | null {
    const lbrack = this.curToken.start;
    this.nextToken(); // consume '['
    this.eatNewlines();

    const elements: ast.ArrayDestructureElement[] = [];
    while (!this.curTokenIs(TokenKind.RBRACKET) && !this.curTokenIs(TokenKind.EOF)) {
      if (!this.curTokenIs(TokenKind.IDENT)) {
        this.errors.push(new ParserError("expected identifier", this.curToken.start));
        return null;
      }
      const name = new ast.Ident(this.curToken.start, this.curToken.literal);
      let defaultValue: ast.Expr | null = null;
      this.nextToken();

      // Check for default (= value)
      if (this.curTokenIs(TokenKind.ASSIGN)) {
        this.nextToken(); // consume '='
        defaultValue = this.parseExpression(Precedence.LOWEST);
        if (!defaultValue) return null;
      }

      elements.push({ name, defaultValue });

      if (!this.curTokenIs(TokenKind.COMMA)) break;
      this.nextToken(); // consume ','
      this.eatNewlines();
    }

    if (!this.curTokenIs(TokenKind.RBRACKET)) {
      this.errors.push(new ParserError("expected ']'", this.curToken.start));
      return null;
    }
    const rbrack = this.curToken.start;
    this.nextToken(); // consume ']'

    if (!this.curTokenIs(TokenKind.ASSIGN)) {
      this.errors.push(new ParserError("expected '='", this.curToken.start));
      return null;
    }
    this.nextToken(); // consume '='
    const value = this.parseExpression(Precedence.LOWEST);
    if (!value) return null;

    return new ast.ArrayDestructureStmt(letPos, lbrack, elements, rbrack, value);
  }

  private parseConst(): ast.ConstStmt | null {
    const constPos = this.curToken.start;
    this.nextToken(); // consume 'const'

    if (!this.curTokenIs(TokenKind.IDENT)) {
      this.errors.push(new ParserError("expected identifier", this.curToken.start));
      return null;
    }
    const name = new ast.Ident(this.curToken.start, this.curToken.literal);
    this.nextToken();

    if (!this.curTokenIs(TokenKind.ASSIGN)) {
      this.errors.push(new ParserError("expected '='", this.curToken.start));
      return null;
    }
    this.nextToken(); // consume '='

    const value = this.parseExpression(Precedence.LOWEST);
    if (!value) return null;

    return new ast.ConstStmt(constPos, name, value);
  }

  private parseReturn(): ast.ReturnStmt {
    const returnPos = this.curToken.start;
    this.nextToken(); // consume 'return'

    // Check for empty return
    if (
      this.curTokenIs(TokenKind.NEWLINE) ||
      this.curTokenIs(TokenKind.EOF) ||
      this.curTokenIs(TokenKind.RBRACE)
    ) {
      return new ast.ReturnStmt(returnPos, null);
    }

    const value = this.parseExpression(Precedence.LOWEST);
    return new ast.ReturnStmt(returnPos, value);
  }

  private parseThrow(): ast.ThrowStmt | null {
    const throwPos = this.curToken.start;
    this.nextToken(); // consume 'throw'

    const value = this.parseExpression(Precedence.LOWEST);
    if (!value) return null;

    return new ast.ThrowStmt(throwPos, value);
  }

  private parseExpressionStatement(): ast.Stmt | ast.Expr | null {
    const expr = this.parseExpression(Precedence.LOWEST);
    if (!expr) return null;

    // Check for assignment
    if (this.curTokenIs(TokenKind.ASSIGN) || this.isCompoundAssign()) {
      return this.parseAssignment(expr);
    }

    // Check for postfix (++ or --)
    if (this.curTokenIs(TokenKind.PLUS_PLUS) || this.curTokenIs(TokenKind.MINUS_MINUS)) {
      const opPos = this.curToken.start;
      const op = this.curToken.literal;
      this.nextToken();
      return new ast.PostfixStmt(expr, opPos, op);
    }

    return expr;
  }

  private isCompoundAssign(): boolean {
    return (
      this.curTokenIs(TokenKind.PLUS_EQUALS) ||
      this.curTokenIs(TokenKind.MINUS_EQUALS) ||
      this.curTokenIs(TokenKind.ASTERISK_EQUALS) ||
      this.curTokenIs(TokenKind.SLASH_EQUALS)
    );
  }

  private parseAssignment(target: ast.Expr): ast.Stmt | null {
    const opPos = this.curToken.start;
    const op = this.curToken.literal;
    this.nextToken(); // consume operator

    const value = this.parseExpression(Precedence.LOWEST);
    if (!value) return null;

    // Property assignment
    if (target instanceof ast.GetAttrExpr) {
      return new ast.SetAttrStmt(target.object, target.period, target.attr, opPos, op, value);
    }

    // Index assignment or simple assignment
    if (target instanceof ast.Ident || target instanceof ast.IndexExpr) {
      return new ast.AssignStmt(target, opPos, op, value);
    }

    this.errors.push(new ParserError("invalid assignment target", target.pos()));
    return null;
  }

  // =========================================================================
  // Expression Parsing
  // =========================================================================

  private parseExpression(precedence: Precedence): ast.Expr | null {
    this.depth++;
    if (this.depth > this.maxDepth) {
      this.errors.push(new ParserError("maximum expression depth exceeded", this.curToken.start));
      this.depth--;
      return null;
    }

    const prefixFn = this.prefixParseFns.get(this.curToken.kind);
    if (!prefixFn) {
      this.noPrefixParseFnError(this.curToken.kind);
      this.depth--;
      return null;
    }

    let left = prefixFn();
    if (!left) {
      this.depth--;
      return null;
    }

    // Handle chaining operators after newlines
    while (!this.curTokenIs(TokenKind.EOF)) {
      // Skip newlines for chaining operators
      if (this.curTokenIs(TokenKind.NEWLINE)) {
        if (this.peekTokenIs(TokenKind.PERIOD) || this.peekTokenIs(TokenKind.QUESTION_DOT)) {
          this.nextToken(); // consume newline
          continue;
        }
        break;
      }

      if (precedence >= this.curPrecedence()) {
        break;
      }

      const infixFn = this.infixParseFns.get(this.curToken.kind);
      if (!infixFn) {
        break;
      }

      left = infixFn(left);
      if (!left) {
        this.depth--;
        return null;
      }
    }

    this.depth--;
    return left;
  }

  // =========================================================================
  // Literal Parsing
  // =========================================================================

  private parseIdent(): ast.Expr | null {
    const ident = new ast.Ident(this.curToken.start, this.curToken.literal);
    this.nextToken();

    // Check for arrow function (x => ...)
    if (this.curTokenIs(TokenKind.ARROW)) {
      return this.parseArrowFunc([ident]);
    }

    return ident;
  }

  private parseInt(): ast.IntLit {
    const literal = this.curToken.literal;
    let value: bigint;
    if (literal.startsWith("0x") || literal.startsWith("0X")) {
      value = BigInt(literal);
    } else if (literal.startsWith("0b") || literal.startsWith("0B")) {
      value = BigInt(literal);
    } else if (literal.startsWith("0") && literal.length > 1 && !literal.includes(".")) {
      // Octal
      value = BigInt("0o" + literal.slice(1));
    } else {
      value = BigInt(literal);
    }
    const node = new ast.IntLit(this.curToken.start, literal, value);
    this.nextToken();
    return node;
  }

  private parseFloat(): ast.FloatLit {
    const literal = this.curToken.literal;
    const value = parseFloat(literal);
    const node = new ast.FloatLit(this.curToken.start, literal, value);
    this.nextToken();
    return node;
  }

  private parseString(): ast.StringLit {
    const node = new ast.StringLit(this.curToken.start, this.curToken.literal);
    this.nextToken();
    return node;
  }

  private parseTemplate(): ast.StringLit {
    // For now, treat templates as raw strings (no interpolation)
    const node = new ast.StringLit(this.curToken.start, this.curToken.literal);
    this.nextToken();
    return node;
  }

  private parseBool(): ast.BoolLit {
    const value = this.curTokenIs(TokenKind.TRUE);
    const node = new ast.BoolLit(this.curToken.start, value);
    this.nextToken();
    return node;
  }

  private parseNil(): ast.NilLit {
    const node = new ast.NilLit(this.curToken.start);
    this.nextToken();
    return node;
  }

  // =========================================================================
  // Operator Parsing
  // =========================================================================

  private parsePrefix(): ast.PrefixExpr | null {
    const opPos = this.curToken.start;
    const op = this.curToken.literal;
    this.nextToken();

    // Special handling for - before ** (right-associative)
    let precedence = Precedence.PREFIX;
    if (op === "-" && this.curTokenIs(TokenKind.INT) && this.peekTokenIs(TokenKind.POW)) {
      precedence = Precedence.POWER;
    }

    const right = this.parseExpression(precedence);
    if (!right) return null;

    return new ast.PrefixExpr(opPos, op, right);
  }

  private parseInfix(left: ast.Expr): ast.InfixExpr | null {
    const opPos = this.curToken.start;
    const op = this.curToken.literal;
    const precedence = this.curPrecedence();
    this.nextToken();

    // Right-associative for **
    const nextPrecedence =
      op === "**" ? precedence - 1 : precedence;

    const right = this.parseExpression(nextPrecedence);
    if (!right) return null;

    return new ast.InfixExpr(left, opPos, op, right);
  }

  private parseSpread(): ast.SpreadExpr | null {
    const ellipsis = this.curToken.start;
    this.nextToken(); // consume '...'

    // Check if there's an expression following
    if (
      this.curTokenIs(TokenKind.COMMA) ||
      this.curTokenIs(TokenKind.RPAREN) ||
      this.curTokenIs(TokenKind.RBRACKET) ||
      this.curTokenIs(TokenKind.RBRACE)
    ) {
      return new ast.SpreadExpr(ellipsis, null);
    }

    const expr = this.parseExpression(Precedence.LOWEST);
    if (!expr) return null;

    return new ast.SpreadExpr(ellipsis, expr);
  }

  // =========================================================================
  // Collection Parsing
  // =========================================================================

  private parseList(): ast.ListLit | null {
    const lbrack = this.curToken.start;
    this.nextToken(); // consume '['
    this.eatNewlines();

    const items: ast.Expr[] = [];
    while (!this.curTokenIs(TokenKind.RBRACKET) && !this.curTokenIs(TokenKind.EOF)) {
      const item = this.parseExpression(Precedence.LOWEST);
      if (!item) return null;
      items.push(item);

      this.eatNewlines();
      if (!this.curTokenIs(TokenKind.COMMA)) break;
      this.nextToken(); // consume ','
      this.eatNewlines();
    }

    this.eatNewlines();
    if (!this.curTokenIs(TokenKind.RBRACKET)) {
      this.errors.push(new ParserError("expected ']'", this.curToken.start));
      return null;
    }
    const rbrack = this.curToken.start;
    this.nextToken(); // consume ']'

    return new ast.ListLit(lbrack, items, rbrack);
  }

  private parseMap(): ast.MapLit | null {
    const lbrace = this.curToken.start;
    this.nextToken(); // consume '{'
    this.eatNewlines();

    const items: ast.MapItem[] = [];
    while (!this.curTokenIs(TokenKind.RBRACE) && !this.curTokenIs(TokenKind.EOF)) {
      // Check for spread
      if (this.curTokenIs(TokenKind.SPREAD)) {
        this.nextToken(); // consume '...'
        const value = this.parseExpression(Precedence.LOWEST);
        if (!value) return null;
        items.push({ key: null, value });
      } else {
        // Key-value pair
        const key = this.parseExpression(Precedence.LOWEST);
        if (!key) return null;

        // Shorthand syntax { a } is equivalent to { a: a }
        if (this.curTokenIs(TokenKind.COMMA) || this.curTokenIs(TokenKind.RBRACE) || this.curTokenIs(TokenKind.NEWLINE)) {
          if (key instanceof ast.Ident) {
            items.push({ key, value: key });
          } else {
            this.errors.push(new ParserError("expected ':'", this.curToken.start));
            return null;
          }
        } else {
          if (!this.curTokenIs(TokenKind.COLON)) {
            this.errors.push(new ParserError("expected ':'", this.curToken.start));
            return null;
          }
          this.nextToken(); // consume ':'
          this.eatNewlines();
          const value = this.parseExpression(Precedence.LOWEST);
          if (!value) return null;
          items.push({ key, value });
        }
      }

      this.eatNewlines();
      if (!this.curTokenIs(TokenKind.COMMA)) break;
      this.nextToken(); // consume ','
      this.eatNewlines();
    }

    this.eatNewlines();
    if (!this.curTokenIs(TokenKind.RBRACE)) {
      this.errors.push(new ParserError("expected '}'", this.curToken.start));
      return null;
    }
    const rbrace = this.curToken.start;
    this.nextToken(); // consume '}'

    return new ast.MapLit(lbrace, items, rbrace);
  }

  // =========================================================================
  // Grouping and Arrow Functions
  // =========================================================================

  private parseGrouped(): ast.Expr | null {
    const lparen = this.curToken.start;
    this.nextToken(); // consume '('
    this.eatNewlines();

    // Check for empty parens (arrow function with no params)
    if (this.curTokenIs(TokenKind.RPAREN)) {
      this.nextToken(); // consume ')'
      if (this.curTokenIs(TokenKind.ARROW)) {
        return this.parseArrowFunc([]);
      }
      this.errors.push(new ParserError("unexpected ')'", this.curToken.start));
      return null;
    }

    // Parse first expression
    const first = this.parseExpression(Precedence.LOWEST);
    if (!first) return null;

    // Check for arrow function with multiple params
    if (this.curTokenIs(TokenKind.COMMA)) {
      // Collect identifiers for arrow function params
      if (!(first instanceof ast.Ident)) {
        this.errors.push(new ParserError("expected identifier in parameter list", first.pos()));
        return null;
      }
      const params: ast.Ident[] = [first];
      while (this.curTokenIs(TokenKind.COMMA)) {
        this.nextToken(); // consume ','
        this.eatNewlines();
        if (!this.curTokenIs(TokenKind.IDENT)) {
          this.errors.push(new ParserError("expected identifier", this.curToken.start));
          return null;
        }
        params.push(new ast.Ident(this.curToken.start, this.curToken.literal));
        this.nextToken();
      }
      this.eatNewlines();
      if (!this.curTokenIs(TokenKind.RPAREN)) {
        this.errors.push(new ParserError("expected ')'", this.curToken.start));
        return null;
      }
      this.nextToken(); // consume ')'
      if (this.curTokenIs(TokenKind.ARROW)) {
        return this.parseArrowFunc(params);
      }
      this.errors.push(new ParserError("expected '=>'", this.curToken.start));
      return null;
    }

    this.eatNewlines();
    if (!this.curTokenIs(TokenKind.RPAREN)) {
      this.errors.push(new ParserError("expected ')'", this.curToken.start));
      return null;
    }
    this.nextToken(); // consume ')'

    // Check for arrow function with single param in parens
    if (this.curTokenIs(TokenKind.ARROW) && first instanceof ast.Ident) {
      return this.parseArrowFunc([first]);
    }

    return first;
  }

  private parseArrowFunc(params: ast.Ident[]): ast.FuncLit | null {
    const arrowPos = this.curToken.start;
    this.nextToken(); // consume '=>'
    this.eatNewlines();

    // Parse body (expression or block)
    if (this.curTokenIs(TokenKind.LBRACE)) {
      const body = this.parseBlock();
      if (!body) return null;
      return new ast.FuncLit(
        params[0]?.position ?? arrowPos,
        null,
        arrowPos,
        params,
        new Map(),
        null,
        arrowPos,
        body
      );
    }

    // Expression body - wrap in return
    const expr = this.parseExpression(Precedence.LOWEST);
    if (!expr) return null;

    const returnStmt = new ast.ReturnStmt(expr.pos(), expr);
    const body = new ast.Block(expr.pos(), [returnStmt], expr.end());

    return new ast.FuncLit(
      params[0]?.position ?? arrowPos,
      null,
      arrowPos,
      params,
      new Map(),
      null,
      arrowPos,
      body
    );
  }

  // =========================================================================
  // Function Parsing
  // =========================================================================

  private parseFunc(): ast.FuncLit | null {
    const funcPos = this.curToken.start;
    this.nextToken(); // consume 'function'

    // Optional function name
    let name: ast.Ident | null = null;
    if (this.curTokenIs(TokenKind.IDENT)) {
      name = new ast.Ident(this.curToken.start, this.curToken.literal);
      this.nextToken();
    }

    if (!this.curTokenIs(TokenKind.LPAREN)) {
      this.errors.push(new ParserError("expected '('", this.curToken.start));
      return null;
    }
    const lparen = this.curToken.start;
    this.nextToken(); // consume '('
    this.eatNewlines();

    // Parse parameters
    const params: ast.FuncParam[] = [];
    const defaults = new Map<string, ast.Expr>();
    let restParam: ast.Ident | null = null;

    while (!this.curTokenIs(TokenKind.RPAREN) && !this.curTokenIs(TokenKind.EOF)) {
      // Check for rest parameter
      if (this.curTokenIs(TokenKind.SPREAD)) {
        this.nextToken(); // consume '...'
        if (!this.curTokenIs(TokenKind.IDENT)) {
          this.errors.push(new ParserError("expected identifier", this.curToken.start));
          return null;
        }
        restParam = new ast.Ident(this.curToken.start, this.curToken.literal);
        this.nextToken();
        break; // rest param must be last
      }

      // Check for destructuring parameters
      if (this.curTokenIs(TokenKind.LBRACE)) {
        const param = this.parseObjectDestructureParam();
        if (!param) return null;
        params.push(param);
      } else if (this.curTokenIs(TokenKind.LBRACKET)) {
        const param = this.parseArrayDestructureParam();
        if (!param) return null;
        params.push(param);
      } else if (this.curTokenIs(TokenKind.IDENT)) {
        const param = new ast.Ident(this.curToken.start, this.curToken.literal);
        this.nextToken();

        // Check for default value
        if (this.curTokenIs(TokenKind.ASSIGN)) {
          this.nextToken(); // consume '='
          const defaultVal = this.parseExpression(Precedence.LOWEST);
          if (!defaultVal) return null;
          defaults.set(param.name, defaultVal);
        }

        params.push(param);
      } else {
        this.errors.push(new ParserError("expected parameter", this.curToken.start));
        return null;
      }

      if (!this.curTokenIs(TokenKind.COMMA)) break;
      this.nextToken(); // consume ','
      this.eatNewlines();
    }

    if (!this.curTokenIs(TokenKind.RPAREN)) {
      this.errors.push(new ParserError("expected ')'", this.curToken.start));
      return null;
    }
    const rparen = this.curToken.start;
    this.nextToken(); // consume ')'

    const body = this.parseBlock();
    if (!body) return null;

    return new ast.FuncLit(funcPos, name, lparen, params, defaults, restParam, rparen, body);
  }

  private parseObjectDestructureParam(): ast.ObjectDestructureParam | null {
    const lbrace = this.curToken.start;
    this.nextToken(); // consume '{'
    this.eatNewlines();

    const bindings: ast.DestructureBinding[] = [];
    while (!this.curTokenIs(TokenKind.RBRACE) && !this.curTokenIs(TokenKind.EOF)) {
      if (!this.curTokenIs(TokenKind.IDENT)) {
        this.errors.push(new ParserError("expected identifier", this.curToken.start));
        return null;
      }
      const key = this.curToken.literal;
      let alias: string | null = null;
      let defaultValue: ast.Expr | null = null;
      this.nextToken();

      if (this.curTokenIs(TokenKind.COLON)) {
        this.nextToken();
        if (!this.curTokenIs(TokenKind.IDENT)) {
          this.errors.push(new ParserError("expected identifier", this.curToken.start));
          return null;
        }
        alias = this.curToken.literal;
        this.nextToken();
      }

      if (this.curTokenIs(TokenKind.ASSIGN)) {
        this.nextToken();
        defaultValue = this.parseExpression(Precedence.LOWEST);
        if (!defaultValue) return null;
      }

      bindings.push({ key, alias, defaultValue });

      if (!this.curTokenIs(TokenKind.COMMA)) break;
      this.nextToken();
      this.eatNewlines();
    }

    if (!this.curTokenIs(TokenKind.RBRACE)) {
      this.errors.push(new ParserError("expected '}'", this.curToken.start));
      return null;
    }
    const rbrace = this.curToken.start;
    this.nextToken();

    return new ast.ObjectDestructureParam(lbrace, bindings, rbrace);
  }

  private parseArrayDestructureParam(): ast.ArrayDestructureParam | null {
    const lbrack = this.curToken.start;
    this.nextToken(); // consume '['
    this.eatNewlines();

    const elements: ast.ArrayDestructureElement[] = [];
    while (!this.curTokenIs(TokenKind.RBRACKET) && !this.curTokenIs(TokenKind.EOF)) {
      if (!this.curTokenIs(TokenKind.IDENT)) {
        this.errors.push(new ParserError("expected identifier", this.curToken.start));
        return null;
      }
      const name = new ast.Ident(this.curToken.start, this.curToken.literal);
      let defaultValue: ast.Expr | null = null;
      this.nextToken();

      if (this.curTokenIs(TokenKind.ASSIGN)) {
        this.nextToken();
        defaultValue = this.parseExpression(Precedence.LOWEST);
        if (!defaultValue) return null;
      }

      elements.push({ name, defaultValue });

      if (!this.curTokenIs(TokenKind.COMMA)) break;
      this.nextToken();
      this.eatNewlines();
    }

    if (!this.curTokenIs(TokenKind.RBRACKET)) {
      this.errors.push(new ParserError("expected ']'", this.curToken.start));
      return null;
    }
    const rbrack = this.curToken.start;
    this.nextToken();

    return new ast.ArrayDestructureParam(lbrack, elements, rbrack);
  }

  // =========================================================================
  // Block Parsing
  // =========================================================================

  private parseBlock(): ast.Block | null {
    if (!this.curTokenIs(TokenKind.LBRACE)) {
      this.errors.push(new ParserError("expected '{'", this.curToken.start));
      return null;
    }
    const lbrace = this.curToken.start;
    this.nextToken(); // consume '{'
    this.eatNewlines();

    const stmts: (ast.Stmt | ast.Expr)[] = [];
    while (!this.curTokenIs(TokenKind.RBRACE) && !this.curTokenIs(TokenKind.EOF)) {
      const stmt = this.parseStatement();
      if (stmt) {
        stmts.push(stmt);
      }
      this.eatNewlines();
    }

    if (!this.curTokenIs(TokenKind.RBRACE)) {
      this.errors.push(new ParserError("expected '}'", this.curToken.start));
      return null;
    }
    const rbrace = this.curToken.start;
    this.nextToken(); // consume '}'

    return new ast.Block(lbrace, stmts, rbrace);
  }

  // =========================================================================
  // Access Expressions
  // =========================================================================

  private parseCall(fn: ast.Expr): ast.CallExpr | null {
    const lparen = this.curToken.start;
    this.nextToken(); // consume '('
    this.eatNewlines();

    const args: (ast.Expr | ast.SpreadExpr)[] = [];
    while (!this.curTokenIs(TokenKind.RPAREN) && !this.curTokenIs(TokenKind.EOF)) {
      const arg = this.parseExpression(Precedence.LOWEST);
      if (!arg) return null;
      args.push(arg);

      if (!this.curTokenIs(TokenKind.COMMA)) break;
      this.nextToken(); // consume ','
      this.eatNewlines();
    }

    if (!this.curTokenIs(TokenKind.RPAREN)) {
      this.errors.push(new ParserError("expected ')'", this.curToken.start));
      return null;
    }
    const rparen = this.curToken.start;
    this.nextToken(); // consume ')'

    return new ast.CallExpr(fn, lparen, args, rparen);
  }

  private parseIndex(left: ast.Expr): ast.IndexExpr | ast.SliceExpr | null {
    const lbrack = this.curToken.start;
    this.nextToken(); // consume '['
    this.eatNewlines();

    // Check for slice [:high]
    if (this.curTokenIs(TokenKind.COLON)) {
      this.nextToken(); // consume ':'
      this.eatNewlines();
      let high: ast.Expr | null = null;
      if (!this.curTokenIs(TokenKind.RBRACKET)) {
        high = this.parseExpression(Precedence.LOWEST);
        if (!high) return null;
      }
      if (!this.curTokenIs(TokenKind.RBRACKET)) {
        this.errors.push(new ParserError("expected ']'", this.curToken.start));
        return null;
      }
      const rbrack = this.curToken.start;
      this.nextToken();
      return new ast.SliceExpr(left, lbrack, null, high, rbrack);
    }

    const index = this.parseExpression(Precedence.LOWEST);
    if (!index) return null;

    // Check for slice [low:high]
    if (this.curTokenIs(TokenKind.COLON)) {
      this.nextToken(); // consume ':'
      this.eatNewlines();
      let high: ast.Expr | null = null;
      if (!this.curTokenIs(TokenKind.RBRACKET)) {
        high = this.parseExpression(Precedence.LOWEST);
        if (!high) return null;
      }
      if (!this.curTokenIs(TokenKind.RBRACKET)) {
        this.errors.push(new ParserError("expected ']'", this.curToken.start));
        return null;
      }
      const rbrack = this.curToken.start;
      this.nextToken();
      return new ast.SliceExpr(left, lbrack, index, high, rbrack);
    }

    if (!this.curTokenIs(TokenKind.RBRACKET)) {
      this.errors.push(new ParserError("expected ']'", this.curToken.start));
      return null;
    }
    const rbrack = this.curToken.start;
    this.nextToken(); // consume ']'

    return new ast.IndexExpr(left, lbrack, index, rbrack);
  }

  private parseGetAttr(left: ast.Expr): ast.GetAttrExpr | ast.ObjectCallExpr | null {
    const period = this.curToken.start;
    this.nextToken(); // consume '.'

    if (!this.curTokenIs(TokenKind.IDENT)) {
      this.errors.push(new ParserError("expected identifier", this.curToken.start));
      return null;
    }
    const attr = new ast.Ident(this.curToken.start, this.curToken.literal);
    this.nextToken();

    // Check for method call
    if (this.curTokenIs(TokenKind.LPAREN)) {
      const call = this.parseCall(attr);
      if (!call) return null;
      return new ast.ObjectCallExpr(left, period, call, false);
    }

    return new ast.GetAttrExpr(left, period, attr, false);
  }

  private parseOptionalChain(left: ast.Expr): ast.GetAttrExpr | ast.ObjectCallExpr | null {
    const period = this.curToken.start;
    this.nextToken(); // consume '?.'

    if (!this.curTokenIs(TokenKind.IDENT)) {
      this.errors.push(new ParserError("expected identifier", this.curToken.start));
      return null;
    }
    const attr = new ast.Ident(this.curToken.start, this.curToken.literal);
    this.nextToken();

    // Check for method call
    if (this.curTokenIs(TokenKind.LPAREN)) {
      const call = this.parseCall(attr);
      if (!call) return null;
      return new ast.ObjectCallExpr(left, period, call, true);
    }

    return new ast.GetAttrExpr(left, period, attr, true);
  }

  // =========================================================================
  // Control Flow
  // =========================================================================

  private parseIf(): ast.IfExpr | null {
    const ifPos = this.curToken.start;
    this.nextToken(); // consume 'if'

    // Optional parentheses around condition
    const hasParen = this.curTokenIs(TokenKind.LPAREN);
    if (hasParen) this.nextToken();

    const condition = this.parseExpression(Precedence.LOWEST);
    if (!condition) return null;

    if (hasParen) {
      if (!this.curTokenIs(TokenKind.RPAREN)) {
        this.errors.push(new ParserError("expected ')'", this.curToken.start));
        return null;
      }
      this.nextToken();
    }

    const consequence = this.parseBlock();
    if (!consequence) return null;

    let alternative: ast.Block | null = null;
    if (this.curTokenIs(TokenKind.ELSE)) {
      this.nextToken(); // consume 'else'

      // else if
      if (this.curTokenIs(TokenKind.IF)) {
        const elseIf = this.parseIf();
        if (!elseIf) return null;
        alternative = new ast.Block(elseIf.pos(), [elseIf], elseIf.end());
      } else {
        alternative = this.parseBlock();
        if (!alternative) return null;
      }
    }

    return new ast.IfExpr(ifPos, condition, consequence, alternative);
  }

  private parseSwitch(): ast.SwitchExpr | null {
    const switchPos = this.curToken.start;
    this.nextToken(); // consume 'switch'

    // Parentheses around value
    if (!this.curTokenIs(TokenKind.LPAREN)) {
      this.errors.push(new ParserError("expected '('", this.curToken.start));
      return null;
    }
    this.nextToken();

    const value = this.parseExpression(Precedence.LOWEST);
    if (!value) return null;

    if (!this.curTokenIs(TokenKind.RPAREN)) {
      this.errors.push(new ParserError("expected ')'", this.curToken.start));
      return null;
    }
    this.nextToken();

    if (!this.curTokenIs(TokenKind.LBRACE)) {
      this.errors.push(new ParserError("expected '{'", this.curToken.start));
      return null;
    }
    const lbrace = this.curToken.start;
    this.nextToken();
    this.eatNewlines();

    const cases: ast.CaseClause[] = [];
    while (!this.curTokenIs(TokenKind.RBRACE) && !this.curTokenIs(TokenKind.EOF)) {
      const clause = this.parseCaseClause();
      if (!clause) return null;
      cases.push(clause);
      this.eatNewlines();
    }

    if (!this.curTokenIs(TokenKind.RBRACE)) {
      this.errors.push(new ParserError("expected '}'", this.curToken.start));
      return null;
    }
    const rbrace = this.curToken.start;
    this.nextToken();

    return new ast.SwitchExpr(switchPos, value, lbrace, cases, rbrace);
  }

  private parseCaseClause(): ast.CaseClause | null {
    const casePos = this.curToken.start;
    const isDefault = this.curTokenIs(TokenKind.DEFAULT);

    if (!this.curTokenIs(TokenKind.CASE) && !this.curTokenIs(TokenKind.DEFAULT)) {
      this.errors.push(new ParserError("expected 'case' or 'default'", this.curToken.start));
      return null;
    }
    this.nextToken();

    let exprs: ast.Expr[] | null = null;
    if (!isDefault) {
      exprs = [];
      const expr = this.parseExpression(Precedence.LOWEST);
      if (!expr) return null;
      exprs.push(expr);

      while (this.curTokenIs(TokenKind.COMMA)) {
        this.nextToken();
        const e = this.parseExpression(Precedence.LOWEST);
        if (!e) return null;
        exprs.push(e);
      }
    }

    if (!this.curTokenIs(TokenKind.COLON)) {
      this.errors.push(new ParserError("expected ':'", this.curToken.start));
      return null;
    }
    const colon = this.curToken.start;
    this.nextToken();
    this.eatNewlines();

    // Parse case body statements until next case/default/rbrace
    const stmts: (ast.Stmt | ast.Expr)[] = [];
    while (
      !this.curTokenIs(TokenKind.CASE) &&
      !this.curTokenIs(TokenKind.DEFAULT) &&
      !this.curTokenIs(TokenKind.RBRACE) &&
      !this.curTokenIs(TokenKind.EOF)
    ) {
      const stmt = this.parseStatement();
      if (stmt) stmts.push(stmt);
      this.eatNewlines();
    }

    const body = new ast.Block(colon, stmts, this.curToken.start);
    return new ast.CaseClause(casePos, exprs, colon, body, isDefault);
  }

  private parseMatch(): ast.MatchExpr | null {
    const matchPos = this.curToken.start;
    this.nextToken(); // consume 'match'

    const subject = this.parseExpression(Precedence.LOWEST);
    if (!subject) return null;

    if (!this.curTokenIs(TokenKind.LBRACE)) {
      this.errors.push(new ParserError("expected '{'", this.curToken.start));
      return null;
    }
    const lbrace = this.curToken.start;
    this.nextToken();
    this.eatNewlines();

    const arms: ast.MatchArm[] = [];
    let defaultArm: ast.MatchArm | null = null;

    while (!this.curTokenIs(TokenKind.RBRACE) && !this.curTokenIs(TokenKind.EOF)) {
      const arm = this.parseMatchArm();
      if (!arm) return null;

      if (arm.pattern instanceof ast.WildcardPattern) {
        defaultArm = arm;
      } else {
        arms.push(arm);
      }

      if (this.curTokenIs(TokenKind.COMMA)) {
        this.nextToken();
      }
      this.eatNewlines();
    }

    if (!this.curTokenIs(TokenKind.RBRACE)) {
      this.errors.push(new ParserError("expected '}'", this.curToken.start));
      return null;
    }
    const rbrace = this.curToken.start;
    this.nextToken();

    return new ast.MatchExpr(matchPos, subject, lbrace, arms, defaultArm, rbrace);
  }

  private parseMatchArm(): ast.MatchArm | null {
    // Parse pattern
    let pattern: ast.Pattern;
    if (this.curTokenIs(TokenKind.IDENT) && this.curToken.literal === "_") {
      pattern = new ast.WildcardPattern(this.curToken.start);
      this.nextToken();
    } else {
      const expr = this.parseExpression(Precedence.LOWEST);
      if (!expr) return null;
      pattern = new ast.LiteralPattern(expr);
    }

    // Optional guard (if condition)
    let guard: ast.Expr | null = null;
    if (this.curTokenIs(TokenKind.IF)) {
      this.nextToken();
      guard = this.parseExpression(Precedence.LOWEST);
      if (!guard) return null;
    }

    if (!this.curTokenIs(TokenKind.ARROW)) {
      this.errors.push(new ParserError("expected '=>'", this.curToken.start));
      return null;
    }
    const arrow = this.curToken.start;
    this.nextToken();
    this.eatNewlines();

    const result = this.parseExpression(Precedence.LOWEST);
    if (!result) return null;

    return new ast.MatchArm(pattern, guard, arrow, result);
  }

  // =========================================================================
  // Membership and Pipe
  // =========================================================================

  private parseIn(left: ast.Expr): ast.InExpr | null {
    const inPos = this.curToken.start;
    this.nextToken(); // consume 'in'

    const right = this.parseExpression(Precedence.LESSGREATER);
    if (!right) return null;

    return new ast.InExpr(left, inPos, right);
  }

  private parseNotIn(left: ast.Expr): ast.NotInExpr | null {
    const notPos = this.curToken.start;
    this.nextToken(); // consume 'not'

    if (!this.curTokenIs(TokenKind.IN)) {
      this.errors.push(new ParserError("expected 'in'", this.curToken.start));
      return null;
    }
    this.nextToken(); // consume 'in'

    const right = this.parseExpression(Precedence.LESSGREATER);
    if (!right) return null;

    return new ast.NotInExpr(left, notPos, right);
  }

  private parsePipePrefix(): ast.Expr | null {
    // |x| closure syntax not implemented yet
    this.errors.push(new ParserError("pipe prefix not supported", this.curToken.start));
    return null;
  }

  private parsePipeInfix(left: ast.Expr): ast.PipeExpr | null {
    const exprs: ast.Expr[] = [left];

    while (this.curTokenIs(TokenKind.PIPE)) {
      this.nextToken(); // consume '|'
      this.eatNewlines();
      const right = this.parseExpression(Precedence.PIPE + 1);
      if (!right) return null;
      exprs.push(right);
    }

    return new ast.PipeExpr(exprs);
  }

  // =========================================================================
  // Try/Catch/Finally
  // =========================================================================

  private parseTry(): ast.TryExpr | null {
    const tryPos = this.curToken.start;
    this.nextToken(); // consume 'try'

    const body = this.parseBlock();
    if (!body) return null;

    let catchIdent: ast.Ident | null = null;
    let catchBlock: ast.Block | null = null;
    let finallyBlock: ast.Block | null = null;

    if (this.curTokenIs(TokenKind.CATCH)) {
      this.nextToken(); // consume 'catch'

      // Optional catch variable
      if (this.curTokenIs(TokenKind.IDENT)) {
        catchIdent = new ast.Ident(this.curToken.start, this.curToken.literal);
        this.nextToken();
      }

      catchBlock = this.parseBlock();
      if (!catchBlock) return null;
    }

    if (this.curTokenIs(TokenKind.FINALLY)) {
      this.nextToken(); // consume 'finally'
      finallyBlock = this.parseBlock();
      if (!finallyBlock) return null;
    }

    if (!catchBlock && !finallyBlock) {
      this.errors.push(new ParserError("try requires catch or finally", tryPos));
      return null;
    }

    return new ast.TryExpr(tryPos, body, catchIdent, catchBlock, finallyBlock);
  }
}

/**
 * Parse source code into an AST.
 */
export function parse(source: string, filename?: string): ast.Program {
  const lexer = new Lexer(source, filename);
  const parser = new Parser(lexer);
  return parser.parse();
}
