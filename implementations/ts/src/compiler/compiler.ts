/**
 * Two-pass bytecode compiler for Risor.
 *
 * Pass 1: Collect function declarations for forward references
 * Pass 2: Compile AST to bytecode
 */

import * as ast from "../ast/nodes.js";
import { Position } from "../token/token.js";
import { Op, BinaryOpType, CompareOpType } from "../bytecode/opcode.js";
import { Code, CodeBuilder, Constant, ConstantType, ExceptionHandler } from "../bytecode/code.js";
import {
  SymbolTable,
  createRootSymbolTable,
  Scope,
  Resolution,
} from "./symbol-table.js";

/**
 * Compilation error.
 */
export class CompilerError extends Error {
  constructor(
    message: string,
    public readonly position: Position
  ) {
    super(`${message} at line ${position.line + 1}, column ${position.column + 1}`);
    this.name = "CompilerError";
  }
}

/**
 * Compiler configuration.
 */
export interface CompilerConfig {
  /** Available global names (builtins). */
  globalNames?: string[];
  /** Source filename. */
  filename?: string;
  /** Source code. */
  source?: string;
}

/**
 * Placeholder value for forward jumps.
 */
const PLACEHOLDER = 0xffff;

/**
 * Bytecode compiler for Risor.
 */
export class Compiler {
  private main: CodeBuilder;
  private current: CodeBuilder;
  private symbols: SymbolTable;
  private globalNames: string[];
  private filename: string;
  private source: string;
  private funcIndex: number = 0;
  private error: CompilerError | null = null;

  constructor(config: CompilerConfig = {}) {
    this.globalNames = config.globalNames ?? [];
    this.filename = config.filename ?? "<input>";
    this.source = config.source ?? "";

    this.symbols = createRootSymbolTable();
    this.main = new CodeBuilder("main", "", false, null, this.source, this.filename);
    this.current = this.main;

    // Register global names
    for (const name of this.globalNames) {
      this.symbols.insertVariable(name);
    }
  }

  /**
   * Compile a program to bytecode.
   */
  compile(program: ast.Program): Code {
    // Pass 1: Collect function declarations
    this.collectFunctionDeclarations(program);

    // Pass 2: Compile statements
    const stmts = program.stmts;
    for (let i = 0; i < stmts.length; i++) {
      const stmt = stmts[i];
      const isLast = i === stmts.length - 1;

      // For the last statement, if it's an expression, don't pop the result
      // so it becomes the return value of the program
      if (isLast && this.isExpressionStatement(stmt)) {
        this.compileExpr(stmt as ast.Expr);
      } else {
        this.compileStatement(stmt);
      }

      if (this.error) {
        throw this.error;
      }
    }

    // Emit halt at end of main
    this.emit(Op.Halt);

    // Convert to immutable Code
    return this.main.toCode(
      this.symbols.localCount(),
      this.globalNames.length,
      this.symbols.localNames(),
      this.globalNames
    );
  }

  /**
   * Check if a statement is actually an expression (to be evaluated as the result).
   */
  private isExpressionStatement(stmt: ast.Stmt | ast.Expr): boolean {
    // These are pure expressions that can return a value
    return (
      stmt instanceof ast.IntLit ||
      stmt instanceof ast.FloatLit ||
      stmt instanceof ast.StringLit ||
      stmt instanceof ast.BoolLit ||
      stmt instanceof ast.NilLit ||
      stmt instanceof ast.Ident ||
      stmt instanceof ast.PrefixExpr ||
      stmt instanceof ast.InfixExpr ||
      stmt instanceof ast.ListLit ||
      stmt instanceof ast.MapLit ||
      stmt instanceof ast.CallExpr ||
      stmt instanceof ast.GetAttrExpr ||
      stmt instanceof ast.ObjectCallExpr ||
      stmt instanceof ast.IndexExpr ||
      stmt instanceof ast.SliceExpr ||
      stmt instanceof ast.IfExpr ||
      stmt instanceof ast.SwitchExpr ||
      stmt instanceof ast.MatchExpr ||
      stmt instanceof ast.InExpr ||
      stmt instanceof ast.NotInExpr ||
      stmt instanceof ast.PipeExpr ||
      stmt instanceof ast.TryExpr
    );
  }

  // ===========================================================================
  // Pass 1: Collect Function Declarations
  // ===========================================================================

  private collectFunctionDeclarations(program: ast.Program): void {
    for (const stmt of program.stmts) {
      if (stmt instanceof ast.FuncLit && stmt.name) {
        // Register named function as constant
        this.symbols.insertConstant(stmt.name.name);
      }
    }
  }

  // ===========================================================================
  // Statement Compilation
  // ===========================================================================

  private compileStatement(stmt: ast.Stmt | ast.Expr): void {
    if (stmt instanceof ast.VarStmt) {
      this.compileVarStmt(stmt);
    } else if (stmt instanceof ast.MultiVarStmt) {
      this.compileMultiVarStmt(stmt);
    } else if (stmt instanceof ast.ObjectDestructureStmt) {
      this.compileObjectDestructureStmt(stmt);
    } else if (stmt instanceof ast.ArrayDestructureStmt) {
      this.compileArrayDestructureStmt(stmt);
    } else if (stmt instanceof ast.ConstStmt) {
      this.compileConstStmt(stmt);
    } else if (stmt instanceof ast.ReturnStmt) {
      this.compileReturnStmt(stmt);
    } else if (stmt instanceof ast.AssignStmt) {
      this.compileAssignStmt(stmt);
    } else if (stmt instanceof ast.SetAttrStmt) {
      this.compileSetAttrStmt(stmt);
    } else if (stmt instanceof ast.PostfixStmt) {
      this.compilePostfixStmt(stmt);
    } else if (stmt instanceof ast.ThrowStmt) {
      this.compileThrowStmt(stmt);
    } else if (stmt instanceof ast.FuncLit) {
      // Named function as statement
      if (stmt.name) {
        // Register the name before compiling (for recursion)
        this.symbols.insertVariable(stmt.name.name);
      }
      this.compileFuncLit(stmt);
      if (stmt.name) {
        const resolution = this.symbols.resolve(stmt.name.name);
        if (resolution) {
          this.storeResolution(resolution);
        }
      } else {
        this.emit(Op.PopTop);
      }
    } else if (stmt instanceof ast.TryExpr) {
      this.compileTryExpr(stmt);
      this.emit(Op.PopTop);
    } else {
      // Expression statement
      this.compileExpr(stmt as ast.Expr);
      this.emit(Op.PopTop);
    }
  }

  private compileVarStmt(stmt: ast.VarStmt): void {
    this.compileExpr(stmt.value);
    const symbol = this.symbols.insertVariable(stmt.name.name);
    this.storeSymbol(symbol);
  }

  private compileMultiVarStmt(stmt: ast.MultiVarStmt): void {
    this.compileExpr(stmt.value);
    // Unpack into multiple variables
    this.emit1(Op.Unpack, stmt.names.length);
    for (let i = stmt.names.length - 1; i >= 0; i--) {
      const symbol = this.symbols.insertVariable(stmt.names[i].name);
      this.storeSymbol(symbol);
    }
  }

  private compileObjectDestructureStmt(stmt: ast.ObjectDestructureStmt): void {
    this.compileExpr(stmt.value);

    for (const binding of stmt.bindings) {
      // Duplicate object on stack
      this.emit1(Op.Copy, 0);

      // Load property
      const nameIndex = this.current.addName(binding.key);
      this.emit1(Op.LoadAttrOrNil, nameIndex);

      // Check for default value
      if (binding.defaultValue) {
        const skipDefault = this.emitJumpForward(Op.PopJumpForwardIfNotNil);
        this.emit(Op.PopTop);
        this.compileExpr(binding.defaultValue);
        this.patchJump(skipDefault);
      }

      // Store to variable (alias or key name)
      const varName = binding.alias ?? binding.key;
      const symbol = this.symbols.insertVariable(varName);
      this.storeSymbol(symbol);
    }

    // Pop original object
    this.emit(Op.PopTop);
  }

  private compileArrayDestructureStmt(stmt: ast.ArrayDestructureStmt): void {
    this.compileExpr(stmt.value);

    for (let i = 0; i < stmt.elements.length; i++) {
      const elem = stmt.elements[i];

      // Duplicate array on stack
      this.emit1(Op.Copy, 0);

      // Load index
      const indexConst = this.current.addConstant({ type: ConstantType.Int, value: i });
      this.emit1(Op.LoadConst, indexConst);
      this.emit1(Op.BinarySubscr, 0);

      // Check for default value
      if (elem.defaultValue) {
        const skipDefault = this.emitJumpForward(Op.PopJumpForwardIfNotNil);
        this.emit(Op.PopTop);
        this.compileExpr(elem.defaultValue);
        this.patchJump(skipDefault);
      }

      // Store to variable
      const symbol = this.symbols.insertVariable(elem.name.name);
      this.storeSymbol(symbol);
    }

    // Pop original array
    this.emit(Op.PopTop);
  }

  private compileConstStmt(stmt: ast.ConstStmt): void {
    this.compileExpr(stmt.value);
    const symbol = this.symbols.insertConstant(stmt.name.name);
    this.storeSymbol(symbol);
  }

  private compileReturnStmt(stmt: ast.ReturnStmt): void {
    if (stmt.value) {
      this.compileExpr(stmt.value);
    } else {
      this.emit(Op.Nil);
    }
    this.emit(Op.ReturnValue);
  }

  private compileAssignStmt(stmt: ast.AssignStmt): void {
    if (stmt.target instanceof ast.Ident) {
      const resolution = this.symbols.resolve(stmt.target.name);
      if (!resolution) {
        this.reportError(`undefined variable: ${stmt.target.name}`, stmt.target.position);
        return;
      }

      if (resolution.symbol.isConstant) {
        this.reportError(`cannot assign to constant: ${stmt.target.name}`, stmt.target.position);
        return;
      }

      if (stmt.op === "=") {
        this.compileExpr(stmt.value);
      } else {
        // Compound assignment (+=, -=, etc.)
        this.loadResolution(resolution);
        this.compileExpr(stmt.value);
        this.compileCompoundOp(stmt.op);
      }

      this.storeResolution(resolution);
    } else if (stmt.target instanceof ast.IndexExpr) {
      // Index assignment
      this.compileExpr(stmt.target.object);
      this.compileExpr(stmt.target.index);

      if (stmt.op === "=") {
        this.compileExpr(stmt.value);
      } else {
        // Compound assignment to index
        this.emit1(Op.Copy, 1); // Duplicate object
        this.emit1(Op.Copy, 1); // Duplicate index
        this.emit1(Op.BinarySubscr, 0);
        this.compileExpr(stmt.value);
        this.compileCompoundOp(stmt.op);
      }

      this.emit1(Op.StoreSubscr, 0);
    }
  }

  private compileSetAttrStmt(stmt: ast.SetAttrStmt): void {
    this.compileExpr(stmt.object);
    const nameIndex = this.current.addName(stmt.attr.name);

    if (stmt.op === "=") {
      this.compileExpr(stmt.value);
    } else {
      // Compound assignment
      this.emit1(Op.Copy, 0); // Duplicate object
      this.emit1(Op.LoadAttr, nameIndex);
      this.compileExpr(stmt.value);
      this.compileCompoundOp(stmt.op);
    }

    this.emit1(Op.StoreAttr, nameIndex);
  }

  private compilePostfixStmt(stmt: ast.PostfixStmt): void {
    if (!(stmt.operand instanceof ast.Ident)) {
      this.reportError("postfix operator requires identifier", stmt.operand.pos());
      return;
    }

    const resolution = this.symbols.resolve(stmt.operand.name);
    if (!resolution) {
      this.reportError(`undefined variable: ${stmt.operand.name}`, stmt.operand.pos());
      return;
    }

    // Load current value
    this.loadResolution(resolution);

    // Add/subtract 1
    const oneConst = this.current.addConstant({ type: ConstantType.Int, value: 1 });
    this.emit1(Op.LoadConst, oneConst);

    if (stmt.op === "++") {
      this.emit1(Op.BinaryOp, BinaryOpType.Add);
    } else {
      this.emit1(Op.BinaryOp, BinaryOpType.Subtract);
    }

    // Store back
    this.storeResolution(resolution);
  }

  private compileThrowStmt(stmt: ast.ThrowStmt): void {
    this.compileExpr(stmt.value);
    this.emit(Op.Throw);
  }

  // ===========================================================================
  // Expression Compilation
  // ===========================================================================

  private compileExpr(expr: ast.Expr): void {
    this.current.setPosition(expr.pos());

    if (expr instanceof ast.IntLit) {
      const constIndex = this.current.addConstant({ type: ConstantType.Int, value: Number(expr.value) });
      this.emit1(Op.LoadConst, constIndex);
    } else if (expr instanceof ast.FloatLit) {
      const constIndex = this.current.addConstant({ type: ConstantType.Float, value: expr.value });
      this.emit1(Op.LoadConst, constIndex);
    } else if (expr instanceof ast.BoolLit) {
      this.emit(expr.value ? Op.True : Op.False);
    } else if (expr instanceof ast.NilLit) {
      this.emit(Op.Nil);
    } else if (expr instanceof ast.StringLit) {
      const constIndex = this.current.addConstant({ type: ConstantType.String, value: expr.value });
      this.emit1(Op.LoadConst, constIndex);
    } else if (expr instanceof ast.Ident) {
      this.compileIdent(expr);
    } else if (expr instanceof ast.PrefixExpr) {
      this.compilePrefixExpr(expr);
    } else if (expr instanceof ast.InfixExpr) {
      this.compileInfixExpr(expr);
    } else if (expr instanceof ast.SpreadExpr) {
      this.compileSpreadExpr(expr);
    } else if (expr instanceof ast.ListLit) {
      this.compileListLit(expr);
    } else if (expr instanceof ast.MapLit) {
      this.compileMapLit(expr);
    } else if (expr instanceof ast.FuncLit) {
      this.compileFuncLit(expr);
    } else if (expr instanceof ast.CallExpr) {
      this.compileCallExpr(expr);
    } else if (expr instanceof ast.GetAttrExpr) {
      this.compileGetAttrExpr(expr);
    } else if (expr instanceof ast.ObjectCallExpr) {
      this.compileObjectCallExpr(expr);
    } else if (expr instanceof ast.IndexExpr) {
      this.compileIndexExpr(expr);
    } else if (expr instanceof ast.SliceExpr) {
      this.compileSliceExpr(expr);
    } else if (expr instanceof ast.IfExpr) {
      this.compileIfExpr(expr);
    } else if (expr instanceof ast.SwitchExpr) {
      this.compileSwitchExpr(expr);
    } else if (expr instanceof ast.MatchExpr) {
      this.compileMatchExpr(expr);
    } else if (expr instanceof ast.InExpr) {
      this.compileInExpr(expr);
    } else if (expr instanceof ast.NotInExpr) {
      this.compileNotInExpr(expr);
    } else if (expr instanceof ast.PipeExpr) {
      this.compilePipeExpr(expr);
    } else if (expr instanceof ast.TryExpr) {
      this.compileTryExpr(expr);
    } else {
      this.reportError(`unknown expression type: ${expr.constructor.name}`, expr.pos());
    }
  }

  private compileIdent(expr: ast.Ident): void {
    const resolution = this.symbols.resolve(expr.name);
    if (!resolution) {
      this.reportError(`undefined variable: ${expr.name}`, expr.position);
      return;
    }
    this.loadResolution(resolution);
  }

  private compilePrefixExpr(expr: ast.PrefixExpr): void {
    this.compileExpr(expr.right);

    switch (expr.op) {
      case "-":
        this.emit(Op.UnaryNegative);
        break;
      case "!":
      case "not":
        this.emit(Op.UnaryNot);
        break;
      default:
        this.reportError(`unknown prefix operator: ${expr.op}`, expr.opPos);
    }
  }

  private compileInfixExpr(expr: ast.InfixExpr): void {
    // Short-circuit operators
    if (expr.op === "&&") {
      this.compileExpr(expr.left);
      const jumpFalse = this.emitJumpForward(Op.PopJumpForwardIfFalse);
      this.compileExpr(expr.right);
      const jumpEnd = this.emitJumpForward(Op.JumpForward);
      this.patchJump(jumpFalse);
      this.emit(Op.False);
      this.patchJump(jumpEnd);
      return;
    }

    if (expr.op === "||") {
      this.compileExpr(expr.left);
      const jumpTrue = this.emitJumpForward(Op.PopJumpForwardIfTrue);
      this.compileExpr(expr.right);
      const jumpEnd = this.emitJumpForward(Op.JumpForward);
      this.patchJump(jumpTrue);
      this.emit(Op.True);
      this.patchJump(jumpEnd);
      return;
    }

    if (expr.op === "??") {
      // Compile left value
      this.compileExpr(expr.left);
      // If not nil, jump forward (keeping the value on stack)
      // If nil, the value was popped, so compile right
      const jumpNotNil = this.emitJumpForward(Op.PopJumpForwardIfNotNil);
      // Value was nil and popped, compile right to replace it
      this.compileExpr(expr.right);
      this.patchJump(jumpNotNil);
      return;
    }

    // Regular operators
    this.compileExpr(expr.left);
    this.compileExpr(expr.right);

    // Binary operations
    const binOp = this.getBinaryOp(expr.op);
    if (binOp !== null) {
      this.emit1(Op.BinaryOp, binOp);
      return;
    }

    // Comparison operations
    const cmpOp = this.getCompareOp(expr.op);
    if (cmpOp !== null) {
      this.emit1(Op.CompareOp, cmpOp);
      return;
    }

    this.reportError(`unknown operator: ${expr.op}`, expr.opPos);
  }

  private compileSpreadExpr(expr: ast.SpreadExpr): void {
    if (expr.expr) {
      this.compileExpr(expr.expr);
    }
    // Spread is handled by container compilation
  }

  private compileListLit(expr: ast.ListLit): void {
    let hasSpread = false;
    let count = 0;

    for (const item of expr.items) {
      if (item instanceof ast.SpreadExpr) {
        if (!hasSpread && count > 0) {
          // Build list from items before spread
          this.emit1(Op.BuildList, count);
        }
        hasSpread = true;
        if (item.expr) {
          this.compileExpr(item.expr);
          if (count > 0 || expr.items.indexOf(item) > 0) {
            this.emit1(Op.ListExtend, 0);
          }
        }
        count = 0;
      } else {
        this.compileExpr(item);
        if (hasSpread) {
          this.emit1(Op.ListAppend, 0);
        }
        count++;
      }
    }

    if (!hasSpread) {
      this.emit1(Op.BuildList, count);
    } else if (count > 0) {
      // Build remaining items and extend
      this.emit1(Op.BuildList, count);
      this.emit1(Op.ListExtend, 0);
    }
  }

  private compileMapLit(expr: ast.MapLit): void {
    let hasSpread = false;
    let count = 0;

    for (const item of expr.items) {
      if (item.key === null) {
        // Spread
        if (!hasSpread && count > 0) {
          this.emit1(Op.BuildMap, count);
        }
        hasSpread = true;
        this.compileExpr(item.value);
        if (count > 0 || expr.items.indexOf(item) > 0) {
          this.emit1(Op.MapMerge, 0);
        }
        count = 0;
      } else {
        // For identifier keys, use the name as a string constant
        if (item.key instanceof ast.Ident) {
          const constIndex = this.current.addConstant({ type: ConstantType.String, value: item.key.name });
          this.emit1(Op.LoadConst, constIndex);
        } else {
          this.compileExpr(item.key);
        }
        this.compileExpr(item.value);
        if (hasSpread) {
          this.emit1(Op.MapSet, 0);
        }
        count++;
      }
    }

    if (!hasSpread) {
      this.emit1(Op.BuildMap, count);
    } else if (count > 0) {
      this.emit1(Op.BuildMap, count);
      this.emit1(Op.MapMerge, 0);
    }
  }

  private compileFuncLit(expr: ast.FuncLit): void {
    // Save current state
    const parentCode = this.current;
    const parentSymbols = this.symbols;

    // Create new scope for function
    const funcId = `${this.funcIndex++}`;
    const funcName = expr.name?.name ?? "";
    this.symbols = parentSymbols.newChild();

    // Create code builder for function
    this.current = parentCode.createChild(funcId, funcName, expr.name !== null);

    // Register parameters
    for (const param of expr.params) {
      if (param instanceof ast.Ident) {
        this.symbols.insertVariable(param.name);
      } else if (param instanceof ast.ObjectDestructureParam) {
        // Object destructure param creates temp variable
        const tempIndex = this.symbols.claimSlot();
        // Will destructure at function entry
      } else if (param instanceof ast.ArrayDestructureParam) {
        // Array destructure param creates temp variable
        const tempIndex = this.symbols.claimSlot();
        // Will destructure at function entry
      }
    }

    // Register rest parameter
    if (expr.restParam) {
      this.symbols.insertVariable(expr.restParam.name);
    }

    // Compile function body
    for (const stmt of expr.body.stmts) {
      this.compileStatement(stmt);
    }

    // Ensure function returns
    if (
      expr.body.stmts.length === 0 ||
      !(expr.body.stmts[expr.body.stmts.length - 1] instanceof ast.ReturnStmt)
    ) {
      this.emit(Op.Nil);
      this.emit(Op.ReturnValue);
    }

    // Get free variables before restoring scope
    const freeVars = this.symbols.getFreeVars();
    const localCount = this.symbols.localCount();
    const localNames = this.symbols.localNames();

    // Convert child code to immutable
    const funcCode = this.current.toCode(
      localCount,
      0,
      localNames,
      []
    );

    // Restore parent state
    this.current = parentCode;
    this.symbols = parentSymbols;

    // Add function as constant
    const funcConstIndex = this.current.addConstant({ type: ConstantType.Function, value: funcCode });

    // Emit closure loading with free variables
    if (freeVars.length > 0) {
      // Emit MakeCell for each free variable
      for (const freeVar of freeVars) {
        this.emit2(Op.MakeCell, freeVar.symbol.index, freeVar.depth - 1);
      }
      this.emit2(Op.LoadClosure, funcConstIndex, freeVars.length);
    } else {
      this.emit1(Op.LoadConst, funcConstIndex);
    }
  }

  private compileCallExpr(expr: ast.CallExpr): void {
    this.compileExpr(expr.func);

    let hasSpread = false;
    let argCount = 0;

    for (const arg of expr.args) {
      if (arg instanceof ast.SpreadExpr) {
        hasSpread = true;
      }
      this.compileExpr(arg);
      argCount++;
    }

    this.current.updateMaxCallArgs(argCount);

    if (hasSpread) {
      this.emit1(Op.CallSpread, argCount);
    } else {
      this.emit1(Op.Call, argCount);
    }
  }

  private compileGetAttrExpr(expr: ast.GetAttrExpr): void {
    this.compileExpr(expr.object);
    const nameIndex = this.current.addName(expr.attr.name);

    if (expr.optional) {
      // Optional chaining: skip if nil
      const skipIfNil = this.emitJumpForward(Op.PopJumpForwardIfNil);
      this.emit1(Op.LoadAttr, nameIndex);
      const skipEnd = this.emitJumpForward(Op.JumpForward);
      this.patchJump(skipIfNil);
      this.emit(Op.Nil);
      this.patchJump(skipEnd);
    } else {
      this.emit1(Op.LoadAttr, nameIndex);
    }
  }

  private compileObjectCallExpr(expr: ast.ObjectCallExpr): void {
    this.compileExpr(expr.object);

    // Get method name
    const methodName = (expr.call.func as ast.Ident).name;
    const nameIndex = this.current.addName(methodName);

    if (expr.optional) {
      // Optional chaining
      const skipIfNil = this.emitJumpForward(Op.PopJumpForwardIfNil);

      // Duplicate object for method call (self)
      this.emit1(Op.Copy, 0);
      this.emit1(Op.LoadAttr, nameIndex);

      // Swap so method is below object
      this.emit1(Op.Swap, 1);

      // Compile args
      for (const arg of expr.call.args) {
        this.compileExpr(arg);
      }

      this.emit1(Op.Call, expr.call.args.length + 1);

      const skipEnd = this.emitJumpForward(Op.JumpForward);
      this.patchJump(skipIfNil);
      this.emit(Op.Nil);
      this.patchJump(skipEnd);
    } else {
      // Duplicate object for method call (self)
      this.emit1(Op.Copy, 0);
      this.emit1(Op.LoadAttr, nameIndex);

      // Swap so method is below object
      this.emit1(Op.Swap, 1);

      // Compile args
      for (const arg of expr.call.args) {
        this.compileExpr(arg);
      }

      this.current.updateMaxCallArgs(expr.call.args.length + 1);
      this.emit1(Op.Call, expr.call.args.length + 1);
    }
  }

  private compileIndexExpr(expr: ast.IndexExpr): void {
    this.compileExpr(expr.object);
    this.compileExpr(expr.index);
    this.emit1(Op.BinarySubscr, 0);
  }

  private compileSliceExpr(expr: ast.SliceExpr): void {
    this.compileExpr(expr.object);

    if (expr.low) {
      this.compileExpr(expr.low);
    } else {
      this.emit(Op.Nil);
    }

    if (expr.high) {
      this.compileExpr(expr.high);
    } else {
      this.emit(Op.Nil);
    }

    this.emit2(Op.Slice, 0, 0);
  }

  private compileIfExpr(expr: ast.IfExpr): void {
    this.compileExpr(expr.condition);

    const jumpFalse = this.emitJumpForward(Op.PopJumpForwardIfFalse);

    // Compile consequence
    let hasValue = false;
    for (const stmt of expr.consequence.stmts) {
      this.compileStatement(stmt);
    }
    // If last statement was expression, it's the value
    if (
      expr.consequence.stmts.length > 0 &&
      !(expr.consequence.stmts[expr.consequence.stmts.length - 1] instanceof ast.ReturnStmt) &&
      isExprStmt(expr.consequence.stmts[expr.consequence.stmts.length - 1])
    ) {
      // The PopTop from statement compilation was wrong, re-compile last expr
      // Actually for simplicity, push nil
      hasValue = false;
    }
    if (!hasValue) {
      this.emit(Op.Nil);
    }

    if (expr.alternative) {
      const jumpEnd = this.emitJumpForward(Op.JumpForward);
      this.patchJump(jumpFalse);

      // Compile alternative
      for (const stmt of expr.alternative.stmts) {
        this.compileStatement(stmt);
      }
      if (!hasValue) {
        this.emit(Op.Nil);
      }

      this.patchJump(jumpEnd);
    } else {
      const jumpEnd = this.emitJumpForward(Op.JumpForward);
      this.patchJump(jumpFalse);
      this.emit(Op.Nil);
      this.patchJump(jumpEnd);
    }
  }

  private compileSwitchExpr(expr: ast.SwitchExpr): void {
    this.compileExpr(expr.value);

    const jumpEnds: number[] = [];

    for (const caseClause of expr.cases) {
      if (caseClause.isDefault) {
        // Default case
        this.emit(Op.PopTop); // Pop switch value
        for (const stmt of caseClause.body.stmts) {
          this.compileStatement(stmt);
        }
        this.emit(Op.Nil); // Result value
        break;
      }

      // Compare with each case expression
      const nextCaseJumps: number[] = [];
      for (const caseExpr of caseClause.exprs!) {
        this.emit1(Op.Copy, 0); // Duplicate switch value
        this.compileExpr(caseExpr);
        this.emit1(Op.CompareOp, CompareOpType.Eq);
        const jumpMatch = this.emitJumpForward(Op.PopJumpForwardIfTrue);
        nextCaseJumps.push(jumpMatch);
      }

      // No match, jump to next case
      const jumpNextCase = this.emitJumpForward(Op.JumpForward);

      // Patch all match jumps to here
      for (const jump of nextCaseJumps) {
        this.patchJump(jump);
      }

      // Execute case body
      this.emit(Op.PopTop); // Pop switch value
      for (const stmt of caseClause.body.stmts) {
        this.compileStatement(stmt);
      }
      this.emit(Op.Nil); // Result value
      jumpEnds.push(this.emitJumpForward(Op.JumpForward));

      this.patchJump(jumpNextCase);
    }

    // Patch all end jumps
    for (const jump of jumpEnds) {
      this.patchJump(jump);
    }
  }

  private compileMatchExpr(expr: ast.MatchExpr): void {
    this.compileExpr(expr.subject);

    const jumpEnds: number[] = [];

    for (const arm of expr.arms) {
      // Compare pattern
      this.emit1(Op.Copy, 0); // Duplicate subject

      if (arm.pattern instanceof ast.LiteralPattern) {
        this.compileExpr(arm.pattern.value);
      }

      this.emit1(Op.CompareOp, CompareOpType.Eq);

      // Check guard if present
      if (arm.guard) {
        const jumpNoMatch = this.emitJumpForward(Op.PopJumpForwardIfFalse);
        this.compileExpr(arm.guard);
        const jumpNoGuard = this.emitJumpForward(Op.PopJumpForwardIfFalse);

        // Match successful
        this.emit(Op.PopTop); // Pop subject
        this.compileExpr(arm.result);
        jumpEnds.push(this.emitJumpForward(Op.JumpForward));

        this.patchJump(jumpNoMatch);
        this.patchJump(jumpNoGuard);
      } else {
        const jumpNoMatch = this.emitJumpForward(Op.PopJumpForwardIfFalse);

        // Match successful
        this.emit(Op.PopTop); // Pop subject
        this.compileExpr(arm.result);
        jumpEnds.push(this.emitJumpForward(Op.JumpForward));

        this.patchJump(jumpNoMatch);
      }
    }

    // Default arm (wildcard)
    if (expr.defaultArm) {
      this.emit(Op.PopTop); // Pop subject
      this.compileExpr(expr.defaultArm.result);
    } else {
      this.emit(Op.PopTop);
      this.emit(Op.Nil);
    }

    // Patch all end jumps
    for (const jump of jumpEnds) {
      this.patchJump(jump);
    }
  }

  private compileInExpr(expr: ast.InExpr): void {
    this.compileExpr(expr.right);
    this.compileExpr(expr.left);
    this.emit1(Op.ContainsOp, 0);
  }

  private compileNotInExpr(expr: ast.NotInExpr): void {
    this.compileExpr(expr.right);
    this.compileExpr(expr.left);
    this.emit1(Op.ContainsOp, 0);
    this.emit(Op.UnaryNot);
  }

  private compilePipeExpr(expr: ast.PipeExpr): void {
    // Compile first expression
    this.compileExpr(expr.exprs[0]);

    // For each subsequent expression, pass result as argument
    for (let i = 1; i < expr.exprs.length; i++) {
      const pipeArg = expr.exprs[i];

      if (pipeArg instanceof ast.CallExpr) {
        // Function call: add pipe result as first argument
        this.compileExpr(pipeArg.func);
        this.emit1(Op.Swap, 1); // Move function below pipe result

        for (const arg of pipeArg.args) {
          this.compileExpr(arg);
        }

        this.emit1(Op.Call, pipeArg.args.length + 1);
      } else if (pipeArg instanceof ast.Ident) {
        // Simple function: call with pipe result
        this.compileExpr(pipeArg);
        this.emit1(Op.Swap, 1);
        this.emit1(Op.Call, 1);
      } else {
        // Create partial application
        this.compileExpr(pipeArg);
        this.emit1(Op.Partial, 0);
      }
    }
  }

  private compileTryExpr(expr: ast.TryExpr): void {
    const tryStart = this.current.offset();

    // Calculate offsets (will patch later)
    const catchOffset = expr.catchBlock ? PLACEHOLDER : -1;
    const finallyOffset = expr.finallyBlock ? PLACEHOLDER : -1;

    this.emit2(Op.PushExcept, PLACEHOLDER, PLACEHOLDER);
    const pushExceptOffset = this.current.offset() - 2;

    // Compile try body
    for (const stmt of expr.body.stmts) {
      this.compileStatement(stmt);
    }
    this.emit(Op.Nil); // Result if no exception
    this.emit(Op.PopExcept);

    const jumpAfterTry = this.emitJumpForward(Op.JumpForward);

    // Catch block
    let catchStartOffset = -1;
    if (expr.catchBlock) {
      catchStartOffset = this.current.offset();

      // Store exception in variable if named
      if (expr.catchIdent) {
        const symbol = this.symbols.insertVariable(expr.catchIdent.name);
        this.storeSymbol(symbol);
      } else {
        this.emit(Op.PopTop);
      }

      // Compile catch body
      for (const stmt of expr.catchBlock.stmts) {
        this.compileStatement(stmt);
      }
      this.emit(Op.Nil); // Result
    }

    const jumpAfterCatch = this.emitJumpForward(Op.JumpForward);

    // Finally block
    let finallyStartOffset = -1;
    if (expr.finallyBlock) {
      finallyStartOffset = this.current.offset();

      // Compile finally body
      for (const stmt of expr.finallyBlock.stmts) {
        this.compileStatement(stmt);
      }
      this.emit(Op.EndFinally);
    }

    // Patch jumps
    this.patchJump(jumpAfterTry);
    this.patchJump(jumpAfterCatch);

    // Patch PushExcept operands
    if (catchStartOffset !== -1) {
      this.current.patch(pushExceptOffset, catchStartOffset - pushExceptOffset);
    }
    if (finallyStartOffset !== -1) {
      this.current.patch(pushExceptOffset + 1, finallyStartOffset - pushExceptOffset);
    }

    // Add exception handler entry
    this.current.addExceptionHandler({
      start: tryStart,
      end: catchStartOffset !== -1 ? catchStartOffset : finallyStartOffset,
      catchOffset: catchStartOffset,
      finallyOffset: finallyStartOffset,
      catchVar: expr.catchIdent?.name ?? null,
    });
  }

  // ===========================================================================
  // Helper Methods
  // ===========================================================================

  private emit(opcode: Op): number {
    return this.current.emit(opcode);
  }

  private emit1(opcode: Op, operand: number): number {
    return this.current.emit1(opcode, operand);
  }

  private emit2(opcode: Op, operand1: number, operand2: number): number {
    return this.current.emit2(opcode, operand1, operand2);
  }

  private emitJumpForward(opcode: Op): number {
    return this.emit1(opcode, PLACEHOLDER);
  }

  private patchJump(offset: number): void {
    const currentOffset = this.current.offset();
    const jumpDistance = currentOffset - offset - 2; // -2 for opcode + operand
    this.current.patch(offset + 1, jumpDistance);
  }

  private loadResolution(resolution: Resolution): void {
    switch (resolution.scope) {
      case Scope.Local:
        this.emit1(Op.LoadFast, resolution.symbol.index);
        break;
      case Scope.Global:
        this.emit1(Op.LoadGlobal, resolution.symbol.index);
        break;
      case Scope.Free:
        this.emit1(Op.LoadFree, resolution.freeIndex);
        break;
    }
  }

  private storeResolution(resolution: Resolution): void {
    switch (resolution.scope) {
      case Scope.Local:
        this.emit1(Op.StoreFast, resolution.symbol.index);
        break;
      case Scope.Global:
        this.emit1(Op.StoreGlobal, resolution.symbol.index);
        break;
      case Scope.Free:
        this.emit1(Op.StoreFree, resolution.freeIndex);
        break;
    }
  }

  private storeVariable(name: string, pos: Position): void {
    const resolution = this.symbols.resolve(name);
    if (!resolution) {
      this.reportError(`undefined variable: ${name}`, pos);
      return;
    }
    this.storeResolution(resolution);
  }

  private storeSymbol(symbol: { index: number }): void {
    if (this.symbols.isRootScope()) {
      this.emit1(Op.StoreGlobal, symbol.index);
    } else {
      this.emit1(Op.StoreFast, symbol.index);
    }
  }

  private getBinaryOp(op: string): BinaryOpType | null {
    switch (op) {
      case "+":
        return BinaryOpType.Add;
      case "-":
        return BinaryOpType.Subtract;
      case "*":
        return BinaryOpType.Multiply;
      case "/":
        return BinaryOpType.Divide;
      case "%":
        return BinaryOpType.Modulo;
      case "**":
        return BinaryOpType.Power;
      case "^":
        return BinaryOpType.Xor;
      case "<<":
        return BinaryOpType.LShift;
      case ">>":
        return BinaryOpType.RShift;
      case "&":
        return BinaryOpType.BitwiseAnd;
      case "|":
        return BinaryOpType.BitwiseOr;
      default:
        return null;
    }
  }

  private getCompareOp(op: string): CompareOpType | null {
    switch (op) {
      case "<":
        return CompareOpType.Lt;
      case "<=":
        return CompareOpType.LtEquals;
      case "==":
        return CompareOpType.Eq;
      case "!=":
        return CompareOpType.NotEq;
      case ">":
        return CompareOpType.Gt;
      case ">=":
        return CompareOpType.GtEquals;
      default:
        return null;
    }
  }

  private compileCompoundOp(op: string): void {
    switch (op) {
      case "+=":
        this.emit1(Op.BinaryOp, BinaryOpType.Add);
        break;
      case "-=":
        this.emit1(Op.BinaryOp, BinaryOpType.Subtract);
        break;
      case "*=":
        this.emit1(Op.BinaryOp, BinaryOpType.Multiply);
        break;
      case "/=":
        this.emit1(Op.BinaryOp, BinaryOpType.Divide);
        break;
      default:
        this.reportError(`unknown compound operator: ${op}`, { line: 0, column: 0, char: 0, lineStart: 0, file: this.filename });
    }
  }

  private reportError(message: string, position: Position): void {
    if (!this.error) {
      this.error = new CompilerError(message, position);
    }
  }
}

/**
 * Check if a statement is an expression statement.
 */
function isExprStmt(stmt: ast.Stmt | ast.Expr): boolean {
  return !(
    stmt instanceof ast.VarStmt ||
    stmt instanceof ast.ConstStmt ||
    stmt instanceof ast.ReturnStmt ||
    stmt instanceof ast.AssignStmt ||
    stmt instanceof ast.SetAttrStmt ||
    stmt instanceof ast.PostfixStmt ||
    stmt instanceof ast.ThrowStmt ||
    stmt instanceof ast.MultiVarStmt ||
    stmt instanceof ast.ObjectDestructureStmt ||
    stmt instanceof ast.ArrayDestructureStmt
  );
}

/**
 * Compile source code to bytecode.
 */
export function compile(source: string, config?: CompilerConfig): Code {
  const { parse } = require("../parser/parser.js");
  const program = parse(source, config?.filename);
  const compiler = new Compiler({ ...config, source });
  return compiler.compile(program);
}
