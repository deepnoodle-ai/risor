/**
 * Compiled bytecode container.
 */

import { Position } from "../token/token.js";

/**
 * Constant type enumeration.
 */
export const enum ConstantType {
  Nil = 0,
  Bool = 1,
  Int = 2,
  Float = 3,
  String = 4,
  Function = 5,
}

/**
 * Constant value in the constant pool.
 */
export interface Constant {
  type: ConstantType;
  value: boolean | number | string | Code;
}

/**
 * Source location for an instruction.
 */
export interface SourceLocation {
  line: number;
  column: number;
}

/**
 * Exception handler entry.
 */
export interface ExceptionHandler {
  /** Start offset of the try block. */
  start: number;
  /** End offset of the try block. */
  end: number;
  /** Offset of the catch block (or -1 if none). */
  catchOffset: number;
  /** Offset of the finally block (or -1 if none). */
  finallyOffset: number;
  /** Catch variable name (or null). */
  catchVar: string | null;
}

/**
 * Function metadata for bytecode.
 */
export interface FunctionInfo {
  /** Function ID for debugging. */
  id: string;
  /** Function name (empty for anonymous). */
  name: string;
  /** Number of parameters. */
  paramCount: number;
  /** Parameter names. */
  paramNames: string[];
  /** Whether function has rest parameter. */
  hasRestParam: boolean;
  /** Rest parameter name. */
  restParamName: string | null;
  /** Number of free variables (closures). */
  freeCount: number;
}

/**
 * Immutable compiled bytecode.
 */
export class Code {
  /** Unique identifier. */
  readonly id: string;

  /** Function name (empty for module/anonymous). */
  readonly name: string;

  /** Whether this is a named function. */
  readonly isNamed: boolean;

  /** Bytecode instructions (opcodes + operands). */
  readonly instructions: readonly number[];

  /** Constant pool. */
  readonly constants: readonly Constant[];

  /** Attribute name pool. */
  readonly names: readonly string[];

  /** Number of local variables. */
  readonly localCount: number;

  /** Number of global variables. */
  readonly globalCount: number;

  /** Local variable names. */
  readonly localNames: readonly string[];

  /** Global variable names. */
  readonly globalNames: readonly string[];

  /** Child code blocks (nested functions). */
  readonly children: readonly Code[];

  /** Source code (for error messages). */
  readonly source: string;

  /** Source filename. */
  readonly filename: string;

  /** Source locations for each instruction. */
  readonly locations: readonly SourceLocation[];

  /** Exception handlers. */
  readonly exceptionHandlers: readonly ExceptionHandler[];

  /** Maximum call arguments (for stack sizing). */
  readonly maxCallArgs: number;

  constructor(
    id: string,
    name: string,
    isNamed: boolean,
    instructions: number[],
    constants: Constant[],
    names: string[],
    localCount: number,
    globalCount: number,
    localNames: string[],
    globalNames: string[],
    children: Code[],
    source: string,
    filename: string,
    locations: SourceLocation[],
    exceptionHandlers: ExceptionHandler[],
    maxCallArgs: number
  ) {
    this.id = id;
    this.name = name;
    this.isNamed = isNamed;
    this.instructions = Object.freeze([...instructions]);
    this.constants = Object.freeze([...constants]);
    this.names = Object.freeze([...names]);
    this.localCount = localCount;
    this.globalCount = globalCount;
    this.localNames = Object.freeze([...localNames]);
    this.globalNames = Object.freeze([...globalNames]);
    this.children = Object.freeze([...children]);
    this.source = source;
    this.filename = filename;
    this.locations = Object.freeze([...locations]);
    this.exceptionHandlers = Object.freeze([...exceptionHandlers]);
    this.maxCallArgs = maxCallArgs;
    Object.freeze(this);
  }

  /**
   * Get the source location for an instruction index.
   */
  getLocation(instrIndex: number): SourceLocation | undefined {
    return this.locations[instrIndex];
  }

  /**
   * Get a child code block by index.
   */
  getChild(index: number): Code | undefined {
    return this.children[index];
  }
}

/**
 * Mutable code builder used during compilation.
 */
export class CodeBuilder {
  id: string;
  name: string;
  isNamed: boolean;
  parent: CodeBuilder | null;
  instructions: number[] = [];
  constants: Constant[] = [];
  names: string[] = [];
  nameMap: Map<string, number> = new Map();
  children: CodeBuilder[] = [];
  source: string;
  filename: string;
  locations: SourceLocation[] = [];
  exceptionHandlers: ExceptionHandler[] = [];
  maxCallArgs: number = 0;

  // For tracking current position
  currentLine: number = 0;
  currentColumn: number = 0;

  constructor(
    id: string,
    name: string,
    isNamed: boolean,
    parent: CodeBuilder | null,
    source: string,
    filename: string
  ) {
    this.id = id;
    this.name = name;
    this.isNamed = isNamed;
    this.parent = parent;
    this.source = source;
    this.filename = filename;
  }

  /**
   * Set current source position for subsequent instructions.
   */
  setPosition(pos: Position): void {
    this.currentLine = pos.line;
    this.currentColumn = pos.column;
  }

  /**
   * Emit a single instruction with no operands.
   */
  emit(opcode: number): number {
    const offset = this.instructions.length;
    this.instructions.push(opcode);
    this.locations.push({ line: this.currentLine, column: this.currentColumn });
    return offset;
  }

  /**
   * Emit an instruction with one operand.
   */
  emit1(opcode: number, operand: number): number {
    const offset = this.instructions.length;
    this.instructions.push(opcode, operand);
    this.locations.push({ line: this.currentLine, column: this.currentColumn });
    this.locations.push({ line: this.currentLine, column: this.currentColumn });
    return offset;
  }

  /**
   * Emit an instruction with two operands.
   */
  emit2(opcode: number, operand1: number, operand2: number): number {
    const offset = this.instructions.length;
    this.instructions.push(opcode, operand1, operand2);
    this.locations.push({ line: this.currentLine, column: this.currentColumn });
    this.locations.push({ line: this.currentLine, column: this.currentColumn });
    this.locations.push({ line: this.currentLine, column: this.currentColumn });
    return offset;
  }

  /**
   * Patch an operand at a specific offset.
   */
  patch(offset: number, value: number): void {
    this.instructions[offset] = value;
  }

  /**
   * Get current instruction offset.
   */
  offset(): number {
    return this.instructions.length;
  }

  /**
   * Add a constant and return its index.
   */
  addConstant(constant: Constant): number {
    // Check for existing constant (for deduplication)
    const existing = this.constants.findIndex(
      (c) => c.type === constant.type && c.value === constant.value
    );
    if (existing !== -1) {
      return existing;
    }
    const index = this.constants.length;
    this.constants.push(constant);
    return index;
  }

  /**
   * Add a name and return its index.
   */
  addName(name: string): number {
    const existing = this.nameMap.get(name);
    if (existing !== undefined) {
      return existing;
    }
    const index = this.names.length;
    this.names.push(name);
    this.nameMap.set(name, index);
    return index;
  }

  /**
   * Create a child code builder.
   */
  createChild(id: string, name: string, isNamed: boolean): CodeBuilder {
    const child = new CodeBuilder(
      id,
      name,
      isNamed,
      this,
      this.source,
      this.filename
    );
    this.children.push(child);
    return child;
  }

  /**
   * Add an exception handler.
   */
  addExceptionHandler(handler: ExceptionHandler): void {
    this.exceptionHandlers.push(handler);
  }

  /**
   * Update max call args if needed.
   */
  updateMaxCallArgs(args: number): void {
    if (args > this.maxCallArgs) {
      this.maxCallArgs = args;
    }
  }

  /**
   * Convert to immutable Code.
   */
  toCode(localCount: number, globalCount: number, localNames: string[], globalNames: string[]): Code {
    // First convert all children
    const children = this.children.map((child) =>
      child.toCode(
        0, // Children get their own counts during compilation
        0,
        [],
        []
      )
    );

    return new Code(
      this.id,
      this.name,
      this.isNamed,
      this.instructions,
      this.constants,
      this.names,
      localCount,
      globalCount,
      localNames,
      globalNames,
      children,
      this.source,
      this.filename,
      this.locations,
      this.exceptionHandlers,
      this.maxCallArgs
    );
  }
}
