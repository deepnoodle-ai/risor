/**
 * Call frame management for the Risor VM.
 */

import { Code } from "../bytecode/code.js";
import { RisorCell, RisorClosure, RisorObject } from "../object/object.js";

/**
 * A call frame representing a function invocation.
 */
export class Frame {
  /** Instruction pointer - current position in bytecode. */
  ip: number = 0;
  /** Base pointer - start of this frame's stack region. */
  readonly bp: number;
  /** The code being executed. */
  readonly code: Code;
  /** Local variables. */
  readonly locals: RisorObject[];
  /** Free variables (closures). */
  readonly freeVars: RisorCell[];

  constructor(
    code: Code,
    bp: number,
    locals: RisorObject[],
    freeVars: RisorCell[] = []
  ) {
    this.code = code;
    this.bp = bp;
    this.locals = locals;
    this.freeVars = freeVars;
  }

  /**
   * Create a frame from a closure.
   */
  static fromClosure(closure: RisorClosure, bp: number, args: RisorObject[]): Frame {
    // Pre-allocate locals array with nil values
    const locals: RisorObject[] = new Array(closure.code.localCount);

    // Copy arguments to locals
    for (let i = 0; i < args.length && i < locals.length; i++) {
      locals[i] = args[i];
    }

    return new Frame(closure.code, bp, locals, closure.freeVars);
  }

  /**
   * Read the current instruction and advance IP.
   */
  readOp(): number {
    return this.code.instructions[this.ip++];
  }

  /**
   * Read an operand without advancing IP.
   */
  peekOp(): number {
    return this.code.instructions[this.ip];
  }

  /**
   * Check if we've reached the end of the bytecode.
   */
  isAtEnd(): boolean {
    return this.ip >= this.code.instructions.length;
  }

  /**
   * Jump forward by offset.
   */
  jumpForward(offset: number): void {
    this.ip += offset;
  }

  /**
   * Jump backward by offset.
   */
  jumpBackward(offset: number): void {
    this.ip -= offset;
  }

  /**
   * Get local variable.
   */
  getLocal(index: number): RisorObject {
    return this.locals[index];
  }

  /**
   * Set local variable.
   */
  setLocal(index: number, value: RisorObject): void {
    this.locals[index] = value;
  }

  /**
   * Get free variable (from closure).
   */
  getFree(index: number): RisorObject {
    return this.freeVars[index].value;
  }

  /**
   * Set free variable (in closure cell).
   */
  setFree(index: number, value: RisorObject): void {
    this.freeVars[index].value = value;
  }

  /**
   * Get constant from code's constant pool.
   */
  getConstant(index: number): unknown {
    return this.code.constants[index];
  }

  /**
   * Get name from code's name pool.
   */
  getName(index: number): string {
    return this.code.names[index];
  }
}

/**
 * Exception handler entry for try-catch-finally.
 */
export interface ExceptionHandler {
  /** Start of try block (instruction index). */
  start: number;
  /** End of try block (instruction index). */
  end: number;
  /** Catch block offset (or -1 if none). */
  catchOffset: number;
  /** Finally block offset (or -1 if none). */
  finallyOffset: number;
  /** Stack depth at entry. */
  stackDepth: number;
  /** Frame depth at entry. */
  frameDepth: number;
}
